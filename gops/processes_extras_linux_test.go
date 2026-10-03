//go:build linux

package gops

import (
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
