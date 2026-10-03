//go:build linux

package gops

import (
	"net"
	"net/netip"
	"testing"

	"github.com/AvengeMedia/dgop/models"
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
	cur := &processSample{ProcessCursorData: models.ProcessCursorData{PID: 42, Timestamp: now}, gpuOK: true, gpu: gpu.cursorClients()}

	var info models.ProcessInfo
	setProcessExtras(&info, prev, cur, gpu, 0)
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

	orig := sockDump
	defer func() { sockDump = orig }()
	var calls uint64
	sockDump = func() ([]sockDiagEntry, error) {
		calls++
		return []sockDiagEntry{{
			Inode: inode, Protocol: 6, Remote: netip.MustParseAddrPort("8.8.8.8:443"),
			RxBytes: calls * 1_000_000, TxBytes: calls * 1_000_000, HasTCPInfo: true,
		}}, nil
	}

	self := ownProcess(t)
	res, err := newProcessTestUtil(t, self).GetProcessesWithOptions(ProcessOptions{EnableCPU: true, Net: true})
	require.NoError(t, err)
	row := res.Processes[0]
	require.NotNil(t, row.NetRx)
	assert.Greater(t, *row.NetRx, 0.0, "the baseline dump to the second dump is the first rate")
	assert.Greater(t, *row.NetTx, 0.0)
	assert.EqualValues(t, 2, calls)
	require.NotNil(t, row.Connections)
	assert.Equal(t, 1, *row.Connections)
}
