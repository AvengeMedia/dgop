//go:build linux

package gops

import (
	"net"
	"net/netip"
	"os"
	"testing"

	"github.com/AvengeMedia/dgop/models"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetProcessExtras_GPUFromCursorClients(t *testing.T) {
	const now = int64(1_755_000_010_000)
	// A nonzero baseline: an unmatched client would rate its whole 6 s as new, giving 60.
	prevCursor := encodeProcessCursor([]processSample{{
		ProcessCursorData: models.ProcessCursorData{PID: 42, Timestamp: now - 10_000},
		gpuOK:             true,
		gpu:               []gpuCursorClient{{PDev: "0000:0c:00.0", ID: 7, Engines: map[string][3]uint64{"gfx": {1_000_000_000, 0, 0}}}},
	}})
	prev := decodeProcessCursor(prevCursor)[42]
	require.NotNil(t, prev)

	key := drmClientKey{PDev: "0000:0c:00.0", ID: 7}
	gpu := procGPU{Readable: true, UsageKnown: true, Clients: drmClientSet{key: {
		Driver: "amdgpu", PDev: key.PDev, ID: key.ID,
		Engines: map[string]drmEngine{"gfx": {BusyNs: 6_000_000_000, HasBusy: true, Capacity: 1}},
	}}}
	cur := &processSample{ProcessCursorData: models.ProcessCursorData{PID: 42, Timestamp: now}, gpuOK: true, gpu: cursorClients(gpu.Clients)}

	var info models.ProcessInfo
	setProcessExtras(&info, prev, cur, gpu)
	require.NotNil(t, info.GPU)
	assert.InDelta(t, 50.0, *info.GPU, 1e-9)
}

func TestProcessExtras_NetLoopbackIsZeroNotUnknown(t *testing.T) {
	if _, err := dumpSockets(); err != nil {
		t.Skip("sock_diag unavailable")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	c, err := net.Dial("tcp", ln.Addr().String())
	require.NoError(t, err)
	defer c.Close()

	self := ownProcess(t)
	res, err := newProcessTestUtil(t, self).GetProcessesWithOptions(ProcessOptions{EnableCPU: true, Net: true, GPU: true})
	require.NoError(t, err)
	row := res.Processes[0]
	require.NotNil(t, row.Connections)
	assert.Zero(t, *row.Connections)
	assert.Empty(t, row.Remotes)
	require.NotNil(t, row.NetRx, "the baseline pass gives the first response rates")
	require.NotNil(t, row.NetTx)
	assert.Zero(t, *row.NetRx)
	assert.True(t, decodeProcessCursor(res.Cursor)[self.Pid].netOK)
	assert.NotNil(t, row.GPU, "GPU still works on the shared fd walk")
}

func TestProcessExtras_FirstPollNetRateFromBaselineDump(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	c, err := net.Dial("tcp", ln.Addr().String())
	require.NoError(t, err)
	defer c.Close()
	inode := socketInode(t, c.(*net.TCPConn))

	self := ownProcess(t)
	g := newProcessTestUtil(t, self)
	var calls uint64
	g.sockDump = func() ([]sockDiagEntry, error) {
		calls++
		return []sockDiagEntry{{
			Inode: inode, Protocol: 6, Remote: netip.MustParseAddrPort("8.8.8.8:443"),
			RxBytes: calls * 1_000_000, TxBytes: calls * 1_000_000, HasTCPInfo: true,
		}}, nil
	}

	res, err := g.GetProcessesWithOptions(ProcessOptions{EnableCPU: true, Net: true})
	require.NoError(t, err)
	row := res.Processes[0]
	require.NotNil(t, row.NetRx)
	assert.Greater(t, *row.NetRx, 0.0, "the baseline dump to the second dump is the first rate")
	assert.Greater(t, *row.NetTx, 0.0)
	assert.EqualValues(t, 2, calls)
	require.NotNil(t, row.Connections)
	assert.Equal(t, 1, *row.Connections)
}

func TestParseProcStat(t *testing.T) {
	st, ok := parseProcStat("1234 (Web Content (x)) S 1 1234 1234 34817 -1 4194560 100 0 0 0 5 3 0 0 20 5 27 0 100 0 0\n")
	require.True(t, ok)
	assert.Equal(t, procStat{PPID: 1, State: "S", Nice: 5, Threads: 27, TTY: 34817, CPUTime: 0.08, Start: 100}, st)

	st, ok = parseProcStat("7 (kworker/0:1) I 2 0 0 0 -1 69238880 0 0 0 0 0 0 0 0 0 -20 1 0 50\n")
	require.True(t, ok)
	assert.Equal(t, procStat{PPID: 2, State: "I", Nice: -20, Threads: 1, Start: 50}, st)

	_, ok = parseProcStat("1 (init) S 0 1 1 0 -1 4194560 0 0 0 0 0 0 0 0 20 0 1 0")
	assert.False(t, ok, "starttime missing")
	_, ok = parseProcStat("")
	assert.False(t, ok)
}

func TestReadProcStatSelf(t *testing.T) {
	p, err := process.NewProcess(int32(os.Getpid()))
	require.NoError(t, err)
	st, ok := readProcStat(p)
	require.True(t, ok)
	created, err := p.CreateTime()
	require.NoError(t, err)
	assert.Equal(t, created, st.CreatedMillis, "matches gopsutil")
	ppid, _ := p.Ppid()
	assert.Equal(t, ppid, st.PPID)
	assert.Positive(t, st.Threads)
}
