package gops

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/rand/v2"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/AvengeMedia/dgop/models"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

func ownProcess(t *testing.T) *process.Process {
	t.Helper()
	self, err := process.NewProcess(int32(os.Getpid()))
	require.NoError(t, err)
	return self
}

// createdMillis is the earliest baseline time the PID reuse guard accepts.
func createdMillis(t *testing.T, p *process.Process) int64 {
	t.Helper()
	created, err := p.CreateTime()
	require.NoError(t, err)
	return created
}

func cursorJSON(t *testing.T, cursor string) map[string]json.RawMessage {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	require.NoError(t, err)
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	require.NoError(t, err)
	raw, err = io.ReadAll(gz)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fields))
	return fields
}

func TestProcessExtras_DiskRatesFromCursor(t *testing.T) {
	self := ownProcess(t)
	read, write, ok := processDiskBytes(self)
	if !ok {
		t.Skip("own process disk counters unreadable on this platform")
	}

	cursor := encodeProcessCursor([]processSample{{
		ProcessCursorData: models.ProcessCursorData{PID: self.Pid, Timestamp: createdMillis(t, self)},
		ioOK:              true,
		io:                [2]uint64{read, write},
	}})

	res, err := newProcessTestUtil(t, self).GetProcessesWithOptions(ProcessOptions{IO: true, Cursor: cursor})
	require.NoError(t, err)
	require.Len(t, res.Processes, 1)
	row := res.Processes[0]

	// Smoke test: the rate math is covered by TestSetProcessExtras_DiskRates.
	require.NotNil(t, row.DiskRead)
	require.NotNil(t, row.DiskWrite)
	assert.GreaterOrEqual(t, *row.DiskRead, 0.0)
	assert.GreaterOrEqual(t, *row.DiskWrite, 0.0)

	next := decodeProcessCursor(res.Cursor)[self.Pid]
	require.NotNil(t, next)
	assert.True(t, next.ioOK, "the response cursor must carry IO counters for the next rate")
}

func TestProcessExtras_DiskRatesAbsentWithoutPreviousSample(t *testing.T) {
	self := ownProcess(t)
	cursor := encodeProcessCursor([]processSample{{
		ProcessCursorData: models.ProcessCursorData{PID: self.Pid, Timestamp: time.Now().UnixMilli() - 10_000},
	}})

	res, err := newProcessTestUtil(t, self).GetProcessesWithOptions(ProcessOptions{IO: true, Cursor: cursor})
	require.NoError(t, err)

	assert.Nil(t, res.Processes[0].DiskRead, "no previous counters means unknown, never 0")
	assert.Nil(t, res.Processes[0].DiskWrite)
}

func TestProcessExtras_BaselinePassGivesFirstResponseRates(t *testing.T) {
	self := ownProcess(t)
	if _, _, ok := processDiskBytes(self); !ok {
		t.Skip("own process disk counters unreadable on this platform")
	}

	res, err := newProcessTestUtil(t, self).GetProcessesWithOptions(ProcessOptions{EnableCPU: true, IO: true, GPU: true})
	require.NoError(t, err)

	assert.NotNil(t, res.Processes[0].DiskRead)
	if runtime.GOOS == "linux" {
		assert.NotNil(t, res.Processes[0].GPU)
	}
}

func TestProcessExtras_GPUFromCursor(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("per-process GPU is Linux-only")
	}
	self := ownProcess(t)
	entry := models.ProcessCursorData{PID: self.Pid, Timestamp: createdMillis(t, self)}

	withGPU := encodeProcessCursor([]processSample{{ProcessCursorData: entry, gpuOK: true}})
	res, err := newProcessTestUtil(t, self).GetProcessesWithOptions(ProcessOptions{GPU: true, Cursor: withGPU})
	require.NoError(t, err)
	row := res.Processes[0]
	require.NotNil(t, row.GPU)
	require.NotNil(t, row.VRAMKB)
	assert.GreaterOrEqual(t, *row.GPU, 0.0)
	assert.True(t, decodeProcessCursor(res.Cursor)[self.Pid].gpuOK)

	withoutGPU := encodeProcessCursor([]processSample{{ProcessCursorData: entry}})
	res, err = newProcessTestUtil(t, self).GetProcessesWithOptions(ProcessOptions{GPU: true, Cursor: withoutGPU})
	require.NoError(t, err)
	assert.Nil(t, res.Processes[0].GPU, "no previous GPU sample means unknown")
	assert.NotNil(t, res.Processes[0].VRAMKB, "VRAM needs no previous sample")
}

func TestProcessExtras_OffLeavesRowsAndCursorUnchanged(t *testing.T) {
	self := ownProcess(t)
	res, err := newProcessTestUtil(t, self).GetProcessesWithCursor(SortByCPU, 0, true, "", false)
	require.NoError(t, err)

	raw, err := json.Marshal(res.Processes[0])
	require.NoError(t, err)
	var row map[string]any
	require.NoError(t, json.Unmarshal(raw, &row))
	for _, key := range []string{"gpu", "vramKB", "diskRead", "diskWrite"} {
		assert.NotContains(t, row, key)
	}

	fields := cursorJSON(t, res.Cursor)
	assert.ElementsMatch(t, []string{"t", "pid", "cpu", "dt"}, keys(fields), "an extras-off cursor keeps the old wire format")
}

func keys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestProcessExtras_StartTime(t *testing.T) {
	self := ownProcess(t)
	created, err := self.CreateTime()
	require.NoError(t, err)

	res, err := newProcessTestUtil(t, self).GetProcessesWithOptions(ProcessOptions{})
	require.NoError(t, err)
	assert.InDelta(t, created/1000, res.Processes[0].StartTime, 1)
	assert.InDelta(t, time.Now().Unix(), res.Processes[0].StartTime, 600, "the test binary started moments ago")

	raw, err := json.Marshal(models.ProcessInfo{PID: 1})
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "startTime")
}

func TestProcessCursor_ExtrasRoundTrip(t *testing.T) {
	clients := []gpuCursorClient{{PDev: "0000:0c:00.0", ID: 7, Engines: map[string][3]uint64{"gfx": {123, 0, 0}}}}
	samples := []processSample{
		{ProcessCursorData: models.ProcessCursorData{PID: 1, Ticks: 1, Timestamp: 1000}, ioOK: true, io: [2]uint64{10, 20}, gpuOK: true, gpu: clients},
		{ProcessCursorData: models.ProcessCursorData{PID: 2, Ticks: 2, Timestamp: 1001}, gpuOK: true},
		{ProcessCursorData: models.ProcessCursorData{PID: 3, Ticks: 3, Timestamp: 1002}},
	}

	got := decodeProcessCursor(encodeProcessCursor(samples))

	require.Len(t, got, 3)
	assert.Equal(t, samples[0], *got[1])
	assert.True(t, got[2].gpuOK, "sampled with no clients")
	assert.Empty(t, got[2].gpu)
	assert.False(t, got[2].ioOK)
	assert.False(t, got[3].ioOK)
	assert.False(t, got[3].gpuOK, "not sampled")
}

func TestSetProcessExtras_DiskRates(t *testing.T) {
	const now = int64(1_755_000_010_000)
	prev := &processSample{ProcessCursorData: models.ProcessCursorData{Timestamp: now - 10_000}, ioOK: true}
	cur := &processSample{ProcessCursorData: models.ProcessCursorData{Timestamp: now}, ioOK: true, io: [2]uint64{1 << 20, 2 << 20}}

	var info models.ProcessInfo
	setProcessExtras(&info, prev, cur, procGPU{}, 0)
	require.NotNil(t, info.DiskRead)
	require.NotNil(t, info.DiskWrite)
	assert.InDelta(t, 104857.6, *info.DiskRead, 1e-6)
	assert.InDelta(t, 209715.2, *info.DiskWrite, 1e-6)

	info = models.ProcessInfo{}
	setProcessExtras(&info, cur, &processSample{ProcessCursorData: models.ProcessCursorData{Timestamp: now + 10_000}, ioOK: true}, procGPU{}, 0)
	assert.Nil(t, info.DiskRead, "counters going backwards are not a rate")
	assert.Nil(t, info.DiskWrite)
}

func TestSetProcessExtras_ReusedPIDIgnoresPreviousSample(t *testing.T) {
	const now = int64(1_755_000_010_000)
	prev := &processSample{ProcessCursorData: models.ProcessCursorData{Timestamp: now - 10_000}, ioOK: true, gpuOK: true}
	cur := &processSample{ProcessCursorData: models.ProcessCursorData{Timestamp: now}, ioOK: true, io: [2]uint64{1 << 20, 0}, gpuOK: true}

	var info models.ProcessInfo
	setProcessExtras(&info, prev, cur, procGPU{Readable: true, UsageKnown: true}, now-5_000)
	assert.Nil(t, info.DiskRead, "the previous sample belongs to an older process with this PID")
	assert.Nil(t, info.GPU)
	assert.NotNil(t, info.VRAMKB, "VRAM needs no previous sample")

	setProcessExtras(&info, prev, cur, procGPU{Readable: true, UsageKnown: true}, now-20_000)
	assert.NotNil(t, info.DiskRead)
	assert.NotNil(t, info.GPU)
}

// Worst case (seeded, host-independent): 500 readable processes with
// incompressible 10-12 digit IO counters and GPU clients; measured 13.9 KB total.
func TestProcessCursorWithExtrasSize(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	counter := func() uint64 { return 1_000_000_000 + rng.Uint64N(99_000_000_000) } // 10-11 digits
	entries := make([]processSample, 0, 500)
	for i := range 500 {
		s := processSample{
			ProcessCursorData: models.ProcessCursorData{PID: int32(1000 + i*7 + int(rng.Uint64N(7))), Ticks: float64(rng.Uint64N(10_000_000)) / 100, Timestamp: 1_755_000_000_000 + int64(rng.Uint64N(200))},
			ioOK:              true,
			io:                [2]uint64{counter(), counter()},
			gpuOK:             i%2 == 0,
		}
		if i%25 == 0 {
			s.gpu = []gpuCursorClient{{PDev: "0000:0c:00.0", ID: rng.Uint64N(1 << 20), Engines: map[string][3]uint64{
				"gfx": {counter(), 0, 0}, "compute": {counter(), 0, 0},
			}}}
		}
		entries = append(entries, s)
	}

	encoded := encodeProcessCursor(entries)
	assert.Less(t, len(encoded), 18<<10, "cursor with extras is %d bytes", len(encoded))
	assert.Len(t, decodeProcessCursor(encoded), 500)
}

func TestProcessCursor_BadExtrasDropOnlyExtras(t *testing.T) {
	got := decodeProcessCursor(gzipB64(t, `{"t":5,"pid":[1,2],"cpu":[1000,2000],"dt":[0,1],"iop":[1,2],"ior":[1],"iow":[1,2],"g":"nope"}`))

	require.Len(t, got, 2)
	assert.Equal(t, 2.0, got[2].Ticks)
	assert.False(t, got[1].ioOK)
	assert.False(t, got[1].gpuOK)
}

func TestProcessCursor_LegacyCursorStillRatesCPU(t *testing.T) {
	self := ownProcess(t)
	times, err := self.Times()
	require.NoError(t, err)

	// The wire format written before extras existed.
	legacy, err := json.Marshal(map[string]any{
		"t": time.Now().UnixMilli() - 10_000, "pid": []int32{self.Pid},
		"cpu": []int64{int64((times.User + times.System - 5) * 1000)}, "dt": []int64{0},
	})
	require.NoError(t, err)

	res, err := newProcessTestUtil(t, self).GetProcessesWithOptions(ProcessOptions{EnableCPU: true, IO: true, GPU: true, Cursor: gzipB64(t, string(legacy))})
	require.NoError(t, err)

	expected := 50.0 / float64(runtime.NumCPU())
	assert.InDelta(t, expected, res.Processes[0].CPU, expected/10)
	assert.Nil(t, res.Processes[0].DiskRead, "a legacy cursor has no IO baseline")
	assert.Nil(t, res.Processes[0].GPU, "a legacy cursor has no GPU baseline")
}

func TestMergeProcesses_ExtrasUnknownPropagation(t *testing.T) {
	parent := &models.ProcessInfo{PID: 1, ExecutablePath: "/bin/a", GPU: ptr(30.0), VRAMKB: ptr(uint64(100)), DiskRead: ptr(5.0), StartTime: 111}
	child := &models.ProcessInfo{PID: 2, PPID: 1, ExecutablePath: "/bin/a", GPU: ptr(90.0), DiskRead: nil, StartTime: 222}
	grandchild := &models.ProcessInfo{PID: 3, PPID: 2, ExecutablePath: "/bin/a", VRAMKB: ptr(uint64(50))}
	other := &models.ProcessInfo{PID: 4, ExecutablePath: "/bin/b"}

	merged := mergeProcessesByExecutable([]*models.ProcessInfo{parent, child, grandchild, other})

	require.Len(t, merged, 2)
	byPID := map[int32]*models.ProcessInfo{}
	for _, p := range merged {
		byPID[p.PID] = p
	}
	root := byPID[1]
	assert.Equal(t, 100.0, *root.GPU, "summed GPU is clamped")
	assert.Equal(t, uint64(150), *root.VRAMKB)
	assert.Equal(t, 5.0, *root.DiskRead, "known parent plus unknown child is the parent's value")
	assert.Nil(t, root.DiskWrite, "all members unknown stays unknown")
	assert.Equal(t, int64(111), root.StartTime, "merged rows keep the root's start time")
	assert.Nil(t, byPID[4].GPU)

	assert.Equal(t, 30.0, *parent.GPU, "the original rows must not be written through")
	assert.Equal(t, uint64(100), *parent.VRAMKB)
	assert.Equal(t, 5.0, *parent.DiskRead)
}

func TestSortProcesses_ExtrasPutUnknownLast(t *testing.T) {
	rows := func() []*models.ProcessInfo {
		return []*models.ProcessInfo{
			{PID: 1},
			{PID: 2, GPU: ptr(10.0), VRAMKB: ptr(uint64(5)), DiskRead: ptr(1.0), DiskWrite: ptr(1.0)},
			{PID: 3},
			{PID: 4, GPU: ptr(0.0), VRAMKB: ptr(uint64(50)), DiskRead: ptr(0.0), DiskWrite: ptr(10.0)},
		}
	}
	pids := func(list []*models.ProcessInfo) []int32 {
		out := make([]int32, len(list))
		for i, p := range list {
			out[i] = p.PID
		}
		return out
	}

	for sortBy, want := range map[ProcSortBy][]int32{
		SortByGPU:  {2, 4},
		SortByVRAM: {4, 2},
		SortByDisk: {4, 2},
	} {
		list := rows()
		sortProcesses(list, sortBy)
		assert.Equal(t, want, pids(list)[:2], string(sortBy))
		assert.ElementsMatch(t, []int32{1, 3}, pids(list)[2:], string(sortBy))
	}
}

func TestMergeProcesses_UnmonitoredDropsGPUAndVRAM(t *testing.T) {
	row := func(pid, ppid int32, gpu float64, vram uint64, unmonitored bool) *models.ProcessInfo {
		p := &models.ProcessInfo{PID: pid, PPID: ppid, ExecutablePath: "/bin/a", GPUUnmonitored: unmonitored}
		if !unmonitored {
			p.GPU, p.VRAMKB = ptr(gpu), ptr(vram)
		}
		return p
	}
	merge := func(rows ...*models.ProcessInfo) *models.ProcessInfo {
		merged := mergeProcessesByExecutable(rows)
		require.Len(t, merged, 1)
		return merged[0]
	}

	root := merge(row(1, 0, 10, 100, false), row(2, 1, 0, 0, true))
	assert.True(t, root.GPUUnmonitored)
	assert.Nil(t, root.GPU)
	assert.Nil(t, root.VRAMKB)

	root = merge(row(1, 0, 0, 0, true), row(2, 1, 10, 100, false))
	assert.True(t, root.GPUUnmonitored)
	assert.Nil(t, root.GPU)
	assert.Nil(t, root.VRAMKB)

	root = merge(row(1, 0, 10, 100, false), row(2, 1, 0, 0, true), row(3, 1, 20, 200, false))
	assert.True(t, root.GPUUnmonitored)
	assert.Nil(t, root.GPU)
	assert.Nil(t, root.VRAMKB)

	root = merge(row(1, 0, 10, 100, false), row(2, 1, 20, 200, false))
	assert.False(t, root.GPUUnmonitored)
	assert.Equal(t, 30.0, *root.GPU)
	assert.Equal(t, uint64(300), *root.VRAMKB)
}
