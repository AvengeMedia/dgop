//go:build linux

package gops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AvengeMedia/dgop/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const amdgpuEngineClient = "drm-driver:\tamdgpu\ndrm-client-id:\t7\ndrm-pdev:\t0000:0c:00.0\ndrm-engine-gfx:\t500 ns\ndrm-resident-vram:\t2048 KiB\n"

func writeFDLink(t *testing.T, procRoot string, pid, fd int, target string) {
	t.Helper()
	dir := filepath.Join(procRoot, itoa(pid), "fd")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.Symlink(target, filepath.Join(dir, itoa(fd))))
}

func TestProcGPUReader_TriState(t *testing.T) {
	procRoot := t.TempDir()
	nodes := map[string]bool{"renderD128": true, "renderD129": true}

	writeDRMClient(t, procRoot, 1, 4, "renderD128", amdgpuEngineClient)
	// Same client through a second fd must count once.
	writeDRMClient(t, procRoot, 1, 5, "renderD128", amdgpuEngineClient)
	require.NoError(t, os.MkdirAll(filepath.Join(procRoot, "2", "fd"), 0o755))
	// pid 3: no fd dir.
	require.NoError(t, os.MkdirAll(filepath.Join(procRoot, "3"), 0o755))
	writeDRMClient(t, procRoot, 4, 3, "renderD129", "drm-driver:\tnvidia-drm\ndrm-client-id:\t1\ndrm-pdev:\t0000:01:00.0\n")
	// pid 5: CUDA process, only /dev/nvidia* fds.
	writeFDLink(t, procRoot, 5, 3, "/dev/nvidiactl")
	writeFDLink(t, procRoot, 5, 4, "/dev/nvidia0")
	// pid 6: listed by nvidia-smi, no GPU fds visible.
	require.NoError(t, os.MkdirAll(filepath.Join(procRoot, "6", "fd"), 0o755))
	// pid 7: nvidiactl alone is not a GPU.
	writeFDLink(t, procRoot, 7, 3, "/dev/nvidiactl")
	// pid 8: nvidia fd plus an engine-reporting client.
	writeDRMClient(t, procRoot, 8, 4, "renderD128", amdgpuEngineClient)
	writeFDLink(t, procRoot, 8, 5, "/dev/nvidia0")

	r := &procGPUReader{procRoot: procRoot, nodes: nodes, nvidia: true,
		nvidiaVRAM: map[int32]uint64{4: 100, 3: 5, 6: 0}}

	a := r.read(1)
	assert.True(t, a.Readable && a.UsageKnown)
	assert.Equal(t, uint64(2<<20), a.VRAMBytes)
	require.Len(t, a.Clients, 1)
	assert.Equal(t, []gpuCursorClient{{PDev: "0000:0c:00.0", ID: 7, Engines: map[string][3]uint64{"gfx": {500, 0, 0}}}}, a.cursorClients())

	assert.Equal(t, procGPU{Readable: true, UsageKnown: true}, r.read(2))
	assert.Equal(t, procGPU{}, r.read(3), "unreadable stays unknown even with nvidia-smi data")

	d := r.read(4)
	assert.Equal(t, procGPU{Readable: true, VRAMBytes: 100}, d)

	assert.Equal(t, procGPU{Readable: true}, r.read(5), "nvidia fd without engine clients")
	assert.Equal(t, procGPU{Readable: true}, r.read(6), "listed by nvidia-smi without engine clients")
	assert.Equal(t, procGPU{Readable: true, UsageKnown: true}, r.read(7))
	h := r.read(8)
	assert.False(t, h.UsageKnown, "hybrid: nvidia fd plus iGPU client is unknown")
	assert.Len(t, h.Clients, 1, "iGPU client is still kept")
}

func TestProcGPUReader_NoAwakeGPUSkipsFDWalk(t *testing.T) {
	procRoot := t.TempDir()
	// Would be an nvidia fd, but no awake GPU means it is never looked at.
	writeFDLink(t, procRoot, 1, 3, "/dev/nvidia0")
	require.NoError(t, os.MkdirAll(filepath.Join(procRoot, "2"), 0o755))

	r := &procGPUReader{procRoot: procRoot}
	assert.Equal(t, procGPU{Readable: true, UsageKnown: true}, r.read(1))
	assert.Equal(t, procGPU{}, r.read(2))
}

func TestProcGPUUsage(t *testing.T) {
	prev := []gpuCursorClient{{PDev: "p", ID: 1, Engines: map[string][3]uint64{"gfx": {1_000_000_000, 0, 0}}}}
	cur := drmClientSet{{PDev: "p", ID: 1}: {PDev: "p", ID: 1, Engines: map[string]drmEngine{
		"gfx": {BusyNs: 1_250_000_000, HasBusy: true, Capacity: 1},
	}}}
	assert.InDelta(t, 25.0, procGPUUsage(prev, cur, time.Second), 0.001)
}

func TestProcGPUUsage_Capacity(t *testing.T) {
	text := "drm-driver:\ti915\ndrm-client-id:\t1\ndrm-pdev:\t0000:00:02.0\n" +
		"drm-engine-video:\t2000000000 ns\ndrm-engine-capacity-video:\t2\n"
	client, ok := parseDRMFdinfo(text)
	require.True(t, ok)
	prev := []gpuCursorClient{toCursorClient(client)}
	prev[0].Engines["video"] = [3]uint64{1_000_000_000, 0, 0}

	cur := drmClientSet{client.key(): client}
	assert.InDelta(t, 50.0, procGPUUsage(prev, cur, time.Second), 0.001)
}

func TestProcGPUUsage_Cycles(t *testing.T) {
	text := "drm-driver:\txe\ndrm-client-id:\t1\ndrm-pdev:\t0000:00:02.0\n" +
		"drm-cycles-rcs:\t300\ndrm-total-cycles-rcs:\t2000\n"
	client, ok := parseDRMFdinfo(text)
	require.True(t, ok)
	prev := []gpuCursorClient{{PDev: client.PDev, ID: 1, Engines: map[string][3]uint64{"rcs": {0, 200, 1600}}}}
	assert.InDelta(t, 25.0, procGPUUsage(prev, drmClientSet{client.key(): client}, time.Second), 0.001)
}

func TestIsNvidiaGPUDevice(t *testing.T) {
	assert.True(t, isNvidiaGPUDevice("/dev/nvidia0"))
	assert.True(t, isNvidiaGPUDevice("/dev/nvidia12"))
	for _, s := range []string{"/dev/nvidia", "/dev/nvidiactl", "/dev/nvidia-uvm", "/dev/nvidia0x", "/dev/dri/renderD128"} {
		assert.False(t, isNvidiaGPUDevice(s), s)
	}
}

func TestGPUCursorClient_DriverRoundTrip(t *testing.T) {
	client := drmClient{Driver: "msm", ID: 3, Engines: map[string]drmEngine{"gpu": {BusyNs: 5}}}
	c := toCursorClient(client)
	assert.Equal(t, "msm", c.Driver)
	assert.Contains(t, cursorClientSet([]gpuCursorClient{c}), client.key(), "pdev-less key keeps its driver")

	withPDev := toCursorClient(drmClient{Driver: "amdgpu", PDev: "0000:0c:00.0", ID: 1})
	assert.Empty(t, withPDev.Driver, "driver is only carried for pdev-less clients")
}

func TestBuildProcGPUReader_AllowList(t *testing.T) {
	pciRoot := t.TempDir()
	for slot, node := range map[string]string{"0000:0c:00.0": "renderD128", "0000:01:00.0": "renderD129"} {
		require.NoError(t, os.MkdirAll(filepath.Join(pciRoot, slot, "drm", node), 0o755))
	}
	detected := []models.GPU{
		{PciSlot: "0000:0c:00.0", PciId: "1002:744c", Driver: "amdgpu"},
		{PciSlot: "0000:01:00.0", PciId: "10de:2786", Driver: "nvidia"},
	}
	smi := 0
	query := func() map[int32]uint64 { smi++; return map[int32]uint64{} }

	r := buildProcGPUReader(pciRoot, detected, nil, query)
	assert.Equal(t, map[string]bool{"renderD128": true, "renderD129": true}, r.nodes)
	assert.Equal(t, 1, smi)

	smi = 0
	r = buildProcGPUReader(pciRoot, detected, []string{"1002:744c"}, query)
	assert.Equal(t, map[string]bool{"renderD128": true, "renderD129": false}, r.nodes)
	assert.False(t, r.nvidia)
	assert.Equal(t, 0, smi, "nvidia-smi must not run when no NVIDIA GPU is enabled")

	r = buildProcGPUReader(pciRoot, detected, []string{}, query)
	assert.Equal(t, map[string]bool{"renderD128": false, "renderD129": false}, r.nodes)
	assert.Equal(t, 0, smi)

	r = buildProcGPUReader(pciRoot, detected, []string{"10de:2786"}, query)
	assert.Equal(t, map[string]bool{"renderD128": false, "renderD129": true}, r.nodes)
	assert.True(t, r.nvidia)
	assert.Equal(t, 1, smi)
}

func TestProcGPUReader_ExcludedGPUIsUnknown(t *testing.T) {
	pciRoot := t.TempDir()
	for slot, node := range map[string]string{"0000:0c:00.0": "renderD128", "0000:00:02.0": "renderD129"} {
		require.NoError(t, os.MkdirAll(filepath.Join(pciRoot, slot, "drm", node), 0o755))
	}
	detected := []models.GPU{
		{PciSlot: "0000:0c:00.0", PciId: "1002:744c", Driver: "amdgpu"},
		{PciSlot: "0000:00:02.0", PciId: "8086:a780", Driver: "i915"},
	}
	procRoot := t.TempDir()
	writeDRMClient(t, procRoot, 1, 4, "renderD129", i915Client(1, 10, 10)) // excluded iGPU
	writeDRMClient(t, procRoot, 2, 4, "renderD128", amdgpuEngineClient)    // enabled dGPU
	require.NoError(t, os.MkdirAll(filepath.Join(procRoot, "3", "fd"), 0o755))
	// pid 4: excluded GPU through a bare link; fdinfo must never be needed.
	writeFDLink(t, procRoot, 4, 3, "/dev/dri/card1")

	build := func(ids []string) *procGPUReader {
		r := buildProcGPUReader(pciRoot, detected, ids, func() map[int32]uint64 { return nil })
		r.procRoot = procRoot
		return r
	}

	r := build([]string{"1002:744c"})
	assert.Equal(t, procGPU{Unmonitored: true}, r.read(1), "excluded GPU is unknown, not 0")
	assert.True(t, r.read(2).UsageKnown, "enabled GPU is measured")
	assert.Equal(t, uint64(2<<20), r.read(2).VRAMBytes)
	assert.Equal(t, procGPU{Readable: true, UsageKnown: true}, r.read(3), "no GPU fd stays known 0")

	require.NoError(t, os.MkdirAll(filepath.Join(pciRoot, "0000:00:02.0", "drm", "card1"), 0o755))
	r = build([]string{"1002:744c"})
	assert.Equal(t, procGPU{Unmonitored: true}, r.read(4), "link alone marks the process unknown")

	// Suspended GPUs are skipped the same way.
	susp := filepath.Join(pciRoot, "0000:00:02.0", "power")
	require.NoError(t, os.MkdirAll(susp, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(susp, "runtime_status"), []byte("suspended\n"), 0o644))
	assert.Equal(t, procGPU{Unmonitored: true}, build(nil).read(1), "suspended GPU is unknown")
	require.NoError(t, os.Remove(filepath.Join(susp, "runtime_status")))

	r = build(nil)
	assert.True(t, r.read(1).UsageKnown, "nil allow-list measures every GPU")
	assert.True(t, r.read(2).UsageKnown)
	assert.Equal(t, procGPU{Readable: true, UsageKnown: true}, r.read(3))
}

func TestProcGPUReader_ExcludedWinsOverPartialSum(t *testing.T) {
	pciRoot := t.TempDir()
	for slot, node := range map[string]string{"0000:0c:00.0": "renderD128", "0000:00:02.0": "renderD129"} {
		require.NoError(t, os.MkdirAll(filepath.Join(pciRoot, slot, "drm", node), 0o755))
	}
	detected := []models.GPU{
		{PciSlot: "0000:0c:00.0", PciId: "1002:744c", Driver: "amdgpu"},
		{PciSlot: "0000:00:02.0", PciId: "8086:a780", Driver: "i915"},
	}
	procRoot := t.TempDir()
	writeDRMClient(t, procRoot, 1, 4, "renderD128", amdgpuEngineClient)
	writeFDLink(t, procRoot, 1, 5, "/dev/dri/renderD129")
	r := buildProcGPUReader(pciRoot, detected, []string{"1002:744c"}, func() map[int32]uint64 { return nil })
	r.procRoot = procRoot
	assert.Equal(t, procGPU{Unmonitored: true}, r.read(1))
}

func TestProcGPUReader_SuspendedNeverOpensFdinfo(t *testing.T) {
	pciRoot := t.TempDir()
	slot := "0000:0c:00.0"
	require.NoError(t, os.MkdirAll(filepath.Join(pciRoot, slot, "drm", "renderD128"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(pciRoot, slot, "power"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pciRoot, slot, "power", "runtime_status"), []byte("suspended\n"), 0o644))
	procRoot := t.TempDir()
	writeDRMClient(t, procRoot, 1, 4, "renderD128", amdgpuEngineClient)
	// An fdinfo that cannot be opened: reading it would fail the scan or make the process readable.
	fdinfo := filepath.Join(procRoot, "1", "fdinfo", "4")
	require.NoError(t, os.Remove(fdinfo))
	require.NoError(t, os.Mkdir(fdinfo, 0o755))
	detected := []models.GPU{{PciSlot: slot, PciId: "1002:744c", Driver: "amdgpu"}}
	r := buildProcGPUReader(pciRoot, detected, nil, func() map[int32]uint64 { return nil })
	r.procRoot = procRoot
	assert.Equal(t, procGPU{Unmonitored: true}, r.read(1))
}

func TestSetProcessExtras_UnmonitoredReason(t *testing.T) {
	var info models.ProcessInfo
	setProcessExtras(&info, nil, &processSample{}, procGPU{Unmonitored: true}, 0)
	assert.True(t, info.GPUUnmonitored)
	assert.Nil(t, info.GPU)
	assert.Nil(t, info.VRAMKB)
	b, err := json.Marshal(info)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"gpuUnmonitored":true`)

	info = models.ProcessInfo{}
	setProcessExtras(&info, nil, &processSample{}, procGPU{}, 0)
	b, _ = json.Marshal(info)
	assert.NotContains(t, string(b), "gpuUnmonitored")

	info = models.ProcessInfo{}
	prev := &processSample{ProcessCursorData: models.ProcessCursorData{Timestamp: 1000}, gpuOK: true}
	cur := &processSample{ProcessCursorData: models.ProcessCursorData{Timestamp: 2000}, gpuOK: true}
	setProcessExtras(&info, prev, cur, procGPU{Unmonitored: true}, 0)
	assert.True(t, info.GPUUnmonitored)
	assert.Nil(t, info.GPU, "a previous sample must not turn unmonitored into 0%")
	assert.Nil(t, info.VRAMKB)
}
