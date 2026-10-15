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

func TestProcessExtras_BaselinePassWithoutCPU(t *testing.T) {
	self := ownProcess(t)
	if _, _, ok := processDiskBytes(self); !ok {
		t.Skip("own process disk counters unreadable on this platform")
	}
	res, err := newProcessTestUtil(t, self).GetProcessesWithOptions(ProcessOptions{IO: true})
	require.NoError(t, err)
	assert.NotNil(t, res.Processes[0].DiskRead, "extras get first-response rates with process CPU off")
	assert.Zero(t, res.Processes[0].CPU)
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
	for _, key := range []string{"gpu", "gpuVideo", "vramKB", "gpuGtt", "diskRead", "diskWrite", "connections", "remotes", "netRx", "netTx"} {
		assert.NotContains(t, row, key)
	}

	fields := cursorJSON(t, res.Cursor)
	assert.ElementsMatch(t, []string{"t", "pid", "cpu", "dt", "st"}, keys(fields), "an extras-off cursor carries no extras")
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
	setProcessExtras(&info, prev, cur, procGPU{})
	require.NotNil(t, info.DiskRead)
	require.NotNil(t, info.DiskWrite)
	assert.InDelta(t, 104857.6, *info.DiskRead, 1e-6)
	assert.InDelta(t, 209715.2, *info.DiskWrite, 1e-6)

	info = models.ProcessInfo{}
	setProcessExtras(&info, cur, &processSample{ProcessCursorData: models.ProcessCursorData{Timestamp: now + 10_000}, ioOK: true}, procGPU{})
	assert.Nil(t, info.DiskRead, "counters going backwards are not a rate")
	assert.Nil(t, info.DiskWrite)
}

func TestProcessExtras_ReusedPIDIgnoresPreviousSample(t *testing.T) {
	self := ownProcess(t)
	if _, _, ok := processDiskBytes(self); !ok {
		t.Skip("own process disk counters unreadable on this platform")
	}
	st, ok := readProcStat(self)
	require.True(t, ok)
	cursor := func(start uint64) string {
		return encodeProcessCursor([]processSample{{
			ProcessCursorData: models.ProcessCursorData{PID: self.Pid, Ticks: st.CPUTime - 1, Timestamp: time.Now().UnixMilli() - 10},
			start:             start,
			ioOK:              true,
		}})
	}

	res, err := newProcessTestUtil(t, self).GetProcessesWithOptions(ProcessOptions{EnableCPU: true, IO: true, Cursor: cursor(st.Start + 1)})
	require.NoError(t, err)
	assert.Nil(t, res.Processes[0].DiskRead, "the previous sample belongs to an older process with this PID")
	assert.Zero(t, res.Processes[0].CPU)
	assert.Equal(t, st.Start, decodeProcessCursor(res.Cursor)[self.Pid].start)

	res, err = newProcessTestUtil(t, self).GetProcessesWithOptions(ProcessOptions{EnableCPU: true, IO: true, Cursor: cursor(st.Start)})
	require.NoError(t, err)
	assert.NotNil(t, res.Processes[0].DiskRead)
	assert.Positive(t, res.Processes[0].CPU)
}

// Worst case (seeded, host-independent): 500 readable processes with
// incompressible IO counters, start ticks, GPU clients and TCP sockets;
// measured 21.2 KB.
func TestProcessCursorWithExtrasSize(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	counter := func() uint64 { return 1_000_000_000 + rng.Uint64N(99_000_000_000) } // 10-11 digits
	entries := make([]processSample, 0, 500)
	for i := range 500 {
		s := processSample{
			ProcessCursorData: models.ProcessCursorData{PID: int32(1000 + i*7 + int(rng.Uint64N(7))), Ticks: float64(rng.Uint64N(10_000_000)) / 100, Timestamp: 1_755_000_000_000 + int64(rng.Uint64N(200))},
			start:             rng.Uint64N(100_000_000),
			ioOK:              true,
			io:                [2]uint64{counter(), counter()},
			gpuOK:             i%2 == 0,
			netOK:             true,
			netAt:             1_755_000_000_000,
		}
		if i%10 == 0 {
			for range 3 {
				s.net = append(s.net, [3]uint64{rng.Uint64N(1 << 32), counter(), counter()})
			}
		}
		if i%25 == 0 {
			s.gpu = []gpuCursorClient{{PDev: "0000:0c:00.0", ID: rng.Uint64N(1 << 20), Engines: map[string][3]uint64{
				"gfx": {counter(), 0, 0}, "compute": {counter(), 0, 0},
			}}}
		}
		entries = append(entries, s)
	}

	encoded := encodeProcessCursor(entries)
	assert.Less(t, len(encoded), 24<<10, "cursor with extras is %d bytes", len(encoded))
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

func TestMergeProcesses_GPUSumIsClamped(t *testing.T) {
	merged := mergeProcessesByExecutable([]*models.ProcessInfo{
		{PID: 1, ExecutablePath: "/bin/a", GPU: ptr(30.0), GPUVideo: ptr(70.0), VRAMKB: ptr(uint64(100))},
		{PID: 2, PPID: 1, ExecutablePath: "/bin/a", GPU: ptr(90.0), GPUVideo: ptr(40.0), VRAMKB: ptr(uint64(50))},
	})
	require.Len(t, merged, 1)
	assert.Equal(t, 100.0, *merged[0].GPU, "summed busiest engines are clamped")
	assert.Equal(t, 100.0, *merged[0].GPUVideo)
	assert.Equal(t, uint64(150), *merged[0].VRAMKB)
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
	assert.Nil(t, root.GPU, "a member without GPU makes the sum unknown")
	assert.Nil(t, root.VRAMKB)
	assert.Nil(t, root.DiskRead, "known parent plus unknown child is unknown, not the parent's value")
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

func TestSetProcessExtras_NetRatesPerSocket(t *testing.T) {
	const now = int64(1_755_000_010_000)
	const mb = 1_000_000
	prev := &processSample{ProcessCursorData: models.ProcessCursorData{Timestamp: now - 1_000}, netOK: true, netAt: now - 1_000, net: [][3]uint64{
		{1, 10 * mb, 5 * mb}, // kept
		{2, 7 * mb, 7 * mb},  // closed
		{3, 4 * mb, 4 * mb},  // counters went backwards
	}}
	cur := &processSample{ProcessCursorData: models.ProcessCursorData{Timestamp: now}, netOK: true, netAt: now, net: [][3]uint64{
		{1, 11 * mb, 5 * mb},
		{3, 1, 1},
		{4, mb / 2, mb/2 + 1}, // new: bytes_acked counts the SYN
		{5, 1 << 50, 1},       // new but far beyond any link for 1 s: not a new socket
	}}

	var info models.ProcessInfo
	setProcessExtras(&info, prev, cur, procGPU{})
	require.NotNil(t, info.NetRx)
	require.NotNil(t, info.NetTx)
	assert.InDelta(t, 1.5*mb, *info.NetRx, 1e-6)
	assert.InDelta(t, 0.5*mb, *info.NetTx, 1e-6)

	info = models.ProcessInfo{}
	setProcessExtras(&info, &processSample{ProcessCursorData: prev.ProcessCursorData}, cur, procGPU{})
	assert.Nil(t, info.NetRx, "no previous socket sample means unknown")
	assert.Nil(t, info.NetTx)

	// The rate spans the dump interval (2 s), not the per-process sample times (1 s).
	skewedPrev := *prev
	skewedPrev.netAt = now - 2_000
	info = models.ProcessInfo{}
	setProcessExtras(&info, &skewedPrev, cur, procGPU{})
	require.NotNil(t, info.NetRx)
	assert.InDelta(t, 0.75*mb, *info.NetRx, 1e-6)

	skewedPrev.netAt = 0
	info = models.ProcessInfo{}
	setProcessExtras(&info, &skewedPrev, cur, procGPU{})
	assert.Nil(t, info.NetRx, "a previous sample without a dump time has no rate")

	info = models.ProcessInfo{}
	setProcessExtras(&info, prev, &processSample{ProcessCursorData: cur.ProcessCursorData, netOK: true, netAt: now}, procGPU{})
	require.NotNil(t, info.NetRx)
	assert.Zero(t, *info.NetRx, "all sockets closed is 0, not unknown")
}

func TestProcessCursor_NetRoundTrip(t *testing.T) {
	prev := []processSample{
		{ProcessCursorData: models.ProcessCursorData{PID: 1, Timestamp: 1000}, netOK: true, netAt: 1000, net: [][3]uint64{{9, 10, 20}}},
		{ProcessCursorData: models.ProcessCursorData{PID: 2, Timestamp: 1000}, netOK: true, netAt: 1000},
		{ProcessCursorData: models.ProcessCursorData{PID: 3, Timestamp: 1000}},
	}
	got := decodeProcessCursor(encodeProcessCursor(prev))
	require.Len(t, got, 3)

	rate := func(pid int32, net ...[3]uint64) *float64 {
		var info models.ProcessInfo
		cur := &processSample{ProcessCursorData: models.ProcessCursorData{PID: pid, Timestamp: 2000}, netOK: true, netAt: 2000, net: net}
		setProcessExtras(&info, got[pid], cur, procGPU{})
		return info.NetRx
	}
	assert.Equal(t, ptr(1000.0), rate(1, [3]uint64{9, 1010, 20}), "socket counters survive the round trip")
	assert.Equal(t, ptr(0.0), rate(2), "sampled with no sockets is 0")
	assert.Nil(t, rate(3), "not sampled has no rate")

	got = decodeProcessCursor(gzipB64(t, `{"t":5,"pid":[1],"cpu":[1000],"dt":[0],"n":{"1":"nope"}}`))
	require.Len(t, got, 1)
	assert.False(t, got[1].netOK, "malformed socket data drops only the extras")
}

func TestMergeProcesses_NetSumsAndRecaps(t *testing.T) {
	remotes := func(n int, host string) []models.NetConnection {
		out := make([]models.NetConnection, n, n+10)
		for i := range out {
			out[i] = models.NetConnection{Protocol: "tcp", Remote: host}
		}
		return out
	}
	parent := &models.ProcessInfo{PID: 1, ExecutablePath: "/bin/a", Connections: ptr(30), Remotes: remotes(30, "1.1.1.1:443"), NetRx: ptr(100.0), NetTx: ptr(10.0)}
	child := &models.ProcessInfo{PID: 2, PPID: 1, ExecutablePath: "/bin/a", Connections: ptr(5), Remotes: remotes(5, "8.8.8.8:443"), NetRx: ptr(50.0)}
	unknown := &models.ProcessInfo{PID: 3, PPID: 1, ExecutablePath: "/bin/a"}
	other := &models.ProcessInfo{PID: 4, ExecutablePath: "/bin/b"}

	merged := mergeProcessesByExecutable([]*models.ProcessInfo{parent, child, unknown, other})

	byPID := map[int32]*models.ProcessInfo{}
	for _, p := range merged {
		byPID[p.PID] = p
	}
	root := byPID[1]
	assert.Nil(t, root.Connections, "a member with unknown sockets makes the count unknown")
	assert.Len(t, root.Remotes, maxProcRemotes)
	assert.Equal(t, "8.8.8.8:443", root.Remotes[31].Remote)
	assert.Nil(t, root.NetRx)
	assert.Nil(t, byPID[4].Connections)

	known := mergeProcessesByExecutable([]*models.ProcessInfo{parent, child})
	require.Len(t, known, 1)
	assert.Equal(t, 35, *known[0].Connections, "the count stays exact")
	assert.Equal(t, 150.0, *known[0].NetRx)
	assert.Nil(t, known[0].NetTx, "the child's unknown tx makes the sum unknown")

	assert.Equal(t, 30, *parent.Connections, "the original rows must not be written through")
	assert.Equal(t, models.NetConnection{}, parent.Remotes[:31][30], "the parent's spare capacity must not be appended into")
	assert.Equal(t, 100.0, *parent.NetRx)
}

func TestSortProcesses_TiesBreakOnPID(t *testing.T) {
	for _, sortBy := range []ProcSortBy{SortByCPU, SortByMemory, SortByName, SortByGPU, SortByNetwork} {
		list := []*models.ProcessInfo{{PID: 5}, {PID: 2}, {PID: 9}, {PID: 1}}
		sortProcesses(list, sortBy)
		assert.Equal(t, []int32{1, 2, 5, 9}, []int32{list[0].PID, list[1].PID, list[2].PID, list[3].PID}, string(sortBy))
	}
}

func TestProcessOptions_ExtraSortWithoutExtraFallsBackToCPU(t *testing.T) {
	for sortBy, opts := range map[ProcSortBy]ProcessOptions{
		SortByGPU: {GPU: true}, SortByVRAM: {GPU: true}, SortByDisk: {IO: true}, SortByNetwork: {Net: true},
	} {
		assert.Equal(t, SortByCPU, ProcessOptions{SortBy: sortBy}.effectiveSort(), string(sortBy))
		opts.SortBy = sortBy
		assert.Equal(t, sortBy, opts.effectiveSort(), string(sortBy))
	}
	assert.Equal(t, SortByName, ProcessOptions{SortBy: SortByName}.effectiveSort())
}

func TestSortProcesses_Network(t *testing.T) {
	list := []*models.ProcessInfo{
		{PID: 1},
		{PID: 2, Connections: ptr(2), NetRx: ptr(1.0), NetTx: ptr(0.0)},
		{PID: 3, Connections: ptr(3)},
		{PID: 4, Connections: ptr(2), NetRx: ptr(5.0), NetTx: ptr(0.0)},
		{PID: 5, Connections: ptr(0), NetRx: ptr(0.0), NetTx: ptr(0.0)},
	}
	sortProcesses(list, SortByNetwork)
	pids := make([]int32, len(list))
	for i, p := range list {
		pids[i] = p.PID
	}
	assert.Equal(t, []int32{3, 4, 2, 5, 1}, pids)
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
