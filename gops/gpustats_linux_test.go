//go:build linux

package gops

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/AvengeMedia/dgop/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func noNvidia(t *testing.T) func([]string) map[string]models.GPUStats {
	return func([]string) map[string]models.GPUStats {
		t.Error("nvidia-smi must not be queried")
		return nil
	}
}

func amdgpuFixture(t *testing.T, pciRoot string) string {
	dev := filepath.Join(pciRoot, "0000:0c:00.0")
	writeHwmonFiles(t, dev, map[string]string{
		"power/runtime_status": "active\n",
		"gpu_busy_percent":     "37\n",
		"mem_info_vram_used":   "2147483648\n",
		"mem_info_vram_total":  "25753026560\n",
		"mem_info_gtt_used":    "104857600\n",
		"mem_info_gtt_total":   "16777216000\n",
		"mem_busy_percent":     "8\n",
		"pp_dpm_sclk":          "S: 0Mhz\n1: 500Mhz *\n2: 2526Mhz \n",
		"pp_dpm_mclk":          "0: 96Mhz \n1: 456Mhz \n2: 772Mhz \n3: 1249Mhz *\n",
		"current_link_speed":   "16.0 GT/s PCIe\n",
		"max_link_speed":       "32.0 GT/s PCIe\n",
		"current_link_width":   "16\n",
		"max_link_width":       "16\n",
		"drm/card1/dev":        "226:1\n",
	})
	writeHwmonFiles(t, filepath.Join(dev, "hwmon", "hwmon3"), map[string]string{
		"name":           "amdgpu\n",
		"temp1_input":    "45000\n",
		"temp1_label":    "edge\n",
		"temp2_input":    "52000\n",
		"temp2_label":    "junction\n",
		"temp3_input":    "60000\n",
		"temp3_label":    "mem\n",
		"fan1_input":     "1200\n",
		"fan1_max":       "3600\n",
		"power1_average": "85000000\n",
		"power1_cap":     "327000000\n",
		"freq1_input":    "1800000000\n",
		"freq1_label":    "sclk\n",
		"freq2_input":    "1249000000\n",
		"freq2_label":    "mclk\n",
	})
	return dev
}

func amdIdentity() models.GPUStats {
	return models.GPUStats{PciSlot: "0000:0c:00.0", PciId: "1002:744c", Driver: "amdgpu", Vendor: "AMD", Name: "AMD Radeon RX 7900 XTX"}
}

func TestCollectGPUStats_Amdgpu(t *testing.T) {
	pciRoot := t.TempDir()
	amdgpuFixture(t, pciRoot)

	gpus := []models.GPUStats{amdIdentity()}
	cursor := NewGopsUtil().collectGPUStats(pciRoot, t.TempDir(), gpus, "", time.Now(), noNvidia(t), nil)
	assert.Empty(t, cursor)

	g := gpus[0]
	assert.Equal(t, "AMD Radeon RX 7900 XTX", g.Name)
	assert.False(t, g.Suspended)
	require.NotNil(t, g.Usage)
	assert.Equal(t, 37.0, *g.Usage)
	require.NotNil(t, g.VRAMUsed)
	assert.Equal(t, uint64(2147483648), *g.VRAMUsed)
	require.NotNil(t, g.VRAMTotal)
	assert.Equal(t, uint64(25753026560), *g.VRAMTotal)
	require.NotNil(t, g.GTTUsed)
	assert.Equal(t, uint64(104857600), *g.GTTUsed)
	require.NotNil(t, g.GTTTotal)
	assert.Equal(t, uint64(16777216000), *g.GTTTotal)
	require.NotNil(t, g.MemoryBusy)
	assert.Equal(t, 8.0, *g.MemoryBusy)
	require.NotNil(t, g.CoreClock)
	assert.Equal(t, 1800.0, *g.CoreClock)
	require.NotNil(t, g.CoreClockMax)
	assert.Equal(t, 2526.0, *g.CoreClockMax)
	require.NotNil(t, g.MemClock)
	assert.Equal(t, 1249.0, *g.MemClock)
	require.NotNil(t, g.MemClockMax)
	assert.Equal(t, 1249.0, *g.MemClockMax)
	require.NotNil(t, g.Temperature)
	assert.Equal(t, 45.0, *g.Temperature)
	require.NotNil(t, g.Hotspot)
	assert.Equal(t, 52.0, *g.Hotspot)
	require.NotNil(t, g.MemTemp)
	assert.Equal(t, 60.0, *g.MemTemp)
	require.NotNil(t, g.Power)
	assert.Equal(t, 85.0, *g.Power)
	require.NotNil(t, g.PowerLimit)
	assert.Equal(t, 327.0, *g.PowerLimit)
	require.NotNil(t, g.FanRpm)
	assert.Equal(t, 1200, *g.FanRpm)
	require.NotNil(t, g.FanMaxRpm)
	assert.Equal(t, 3600, *g.FanMaxRpm)
	require.NotNil(t, g.FanPercent)
	assert.InDelta(t, 33.33, *g.FanPercent, 0.01)
	assert.Equal(t, "16.0 GT/s PCIe", g.PcieLinkSpeed)
	assert.Equal(t, "32.0 GT/s PCIe", g.PcieLinkSpeedMax)
	assert.Equal(t, 16, g.PcieLinkWidth)
	assert.Equal(t, 16, g.PcieLinkWidthMax)
}

func TestCollectGPUStats_FanPercent(t *testing.T) {
	pciRoot := t.TempDir()
	dev := amdgpuFixture(t, pciRoot)
	hw := filepath.Join(dev, "hwmon", "hwmon3")
	writeHwmonFiles(t, hw, map[string]string{"pwm1": "40\n", "pwm1_max": "255\n"})

	gpus := []models.GPUStats{amdIdentity()}
	NewGopsUtil().collectGPUStats(pciRoot, t.TempDir(), gpus, "", time.Now(), noNvidia(t), nil)
	require.NotNil(t, gpus[0].FanPercent)
	assert.InDelta(t, 15.69, *gpus[0].FanPercent, 0.01, "PWM duty wins over rpm")

	require.NoError(t, os.Remove(filepath.Join(hw, "pwm1_max")))
	require.NoError(t, os.WriteFile(filepath.Join(hw, "fan1_input"), []byte("4000\n"), 0o644))
	gpus = []models.GPUStats{amdIdentity()}
	NewGopsUtil().collectGPUStats(pciRoot, t.TempDir(), gpus, "", time.Now(), noNvidia(t), nil)
	require.NotNil(t, gpus[0].FanPercent)
	assert.Equal(t, 100.0, *gpus[0].FanPercent, "rpm above max is clamped")
}

func TestCollectGPUStats_AmdgpuClockFallbackIgnoresSleepLevel(t *testing.T) {
	pciRoot := t.TempDir()
	dev := amdgpuFixture(t, pciRoot)
	hw := filepath.Join(dev, "hwmon", "hwmon3")
	for _, f := range []string{"freq1_input", "freq1_label", "freq2_input", "freq2_label"} {
		require.NoError(t, os.Remove(filepath.Join(hw, f)))
	}
	require.NoError(t, os.WriteFile(filepath.Join(dev, "pp_dpm_mclk"), []byte("S: 0Mhz *\n0: 96Mhz \n1: 1249Mhz \n"), 0o644))

	gpus := []models.GPUStats{amdIdentity()}
	NewGopsUtil().collectGPUStats(pciRoot, t.TempDir(), gpus, "", time.Now(), noNvidia(t), nil)

	g := gpus[0]
	require.NotNil(t, g.CoreClock)
	assert.Equal(t, 500.0, *g.CoreClock)
	assert.Equal(t, 2526.0, *g.CoreClockMax)
	assert.Nil(t, g.MemClock, "S: level is not a clock")
	require.NotNil(t, g.MemClockMax)
	assert.Equal(t, 1249.0, *g.MemClockMax)
}

func TestCollectGPUStats_SuspendedReadsNothing(t *testing.T) {
	pciRoot := t.TempDir()
	dev := amdgpuFixture(t, pciRoot)
	require.NoError(t, os.WriteFile(filepath.Join(dev, "power", "runtime_status"), []byte("suspended\n"), 0o644))

	nv := filepath.Join(pciRoot, "0000:01:00.0")
	writeHwmonFiles(t, nv, map[string]string{"power/runtime_status": "suspended\n"})

	gpus := []models.GPUStats{
		amdIdentity(),
		{PciSlot: "0000:01:00.0", Driver: "nvidia", Vendor: "NVIDIA", Name: "NVIDIA GeForce RTX 4070"},
	}
	cursor := NewGopsUtil().collectGPUStats(pciRoot, t.TempDir(), gpus, "", time.Now(), noNvidia(t), nil)
	assert.Empty(t, cursor)

	assert.Equal(t, models.GPUStats{
		PciSlot: "0000:0c:00.0", PciId: "1002:744c", Driver: "amdgpu", Vendor: "AMD",
		Name: "AMD Radeon RX 7900 XTX", Suspended: true,
	}, gpus[0])
	assert.Equal(t, models.GPUStats{
		PciSlot: "0000:01:00.0", Driver: "nvidia", Vendor: "NVIDIA",
		Name: "NVIDIA GeForce RTX 4070", Suspended: true,
	}, gpus[1])
}

func writeDRMClient(t *testing.T, procRoot string, pid, fd int, node, fdinfo string) {
	t.Helper()
	pidDir := filepath.Join(procRoot, itoa(pid))
	require.NoError(t, os.MkdirAll(filepath.Join(pidDir, "fd"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(pidDir, "fdinfo"), 0o755))
	link := filepath.Join(pidDir, "fd", itoa(fd))
	_ = os.Remove(link)
	require.NoError(t, os.Symlink("/dev/dri/"+node, link))
	require.NoError(t, os.WriteFile(filepath.Join(pidDir, "fdinfo", itoa(fd)), []byte(fdinfo), 0o644))
}

func itoa(n int) string { return strconv.Itoa(n) }

func i915Client(id int, renderNs, videoNs int64) string {
	return "drm-driver:\ti915\ndrm-client-id:\t" + itoa(id) + "\ndrm-pdev:\t0000:00:02.0\n" +
		"drm-engine-render:\t" + strconv.FormatInt(renderNs, 10) + " ns\ndrm-engine-video:\t" + strconv.FormatInt(videoNs, 10) + " ns\n"
}

func TestCollectGPUStats_IntelUsageFromFdinfo(t *testing.T) {
	pciRoot := t.TempDir()
	dev := filepath.Join(pciRoot, "0000:00:02.0")
	writeHwmonFiles(t, dev, map[string]string{
		"power/runtime_status":      "active\n",
		"drm/card0/gt_act_freq_mhz": "650\n",
		"drm/card0/gt_cur_freq_mhz": "700\n",
		"drm/card0/gt_RP0_freq_mhz": "1300\n",
		"drm/card0/gt_max_freq_mhz": "1200\n",
		"drm/renderD128/dev":        "226:128\n",
	})
	procRoot := t.TempDir()
	writeDRMClient(t, procRoot, 100, 5, "renderD128", i915Client(1, 1_000_000_000, 0))
	writeDRMClient(t, procRoot, 200, 7, "renderD128", i915Client(2, 0, 2_000_000_000))
	// A client of another GPU must not count.
	writeDRMClient(t, procRoot, 300, 3, "renderD129", "drm-driver:\tamdgpu\ndrm-client-id:\t9\ndrm-pdev:\t0000:0c:00.0\ndrm-engine-gfx:\t9000000000 ns\n")
	// Unreadable process: fd dir missing.
	require.NoError(t, os.MkdirAll(filepath.Join(procRoot, "400"), 0o755))

	identity := models.GPUStats{PciSlot: "0000:00:02.0", Driver: "i915", Vendor: "Intel", Name: "Intel UHD Graphics 770"}
	t0 := time.Unix(1000, 0)

	gpus := []models.GPUStats{identity}
	cursor := NewGopsUtil().collectGPUStats(pciRoot, procRoot, gpus, "", t0, noNvidia(t), nil)
	require.NotEmpty(t, cursor)
	assert.Nil(t, gpus[0].Usage, "no previous sample")
	require.NotNil(t, gpus[0].CoreClock)
	assert.Equal(t, 650.0, *gpus[0].CoreClock)
	require.NotNil(t, gpus[0].CoreClockMax)
	assert.Equal(t, 1300.0, *gpus[0].CoreClockMax)

	// One second later: render +250ms on client 1, video +600ms on client 2.
	writeDRMClient(t, procRoot, 100, 5, "renderD128", i915Client(1, 1_250_000_000, 0))
	writeDRMClient(t, procRoot, 200, 7, "renderD128", i915Client(2, 0, 2_600_000_000))
	writeDRMClient(t, procRoot, 300, 3, "renderD129", "drm-driver:\tamdgpu\ndrm-client-id:\t9\ndrm-pdev:\t0000:0c:00.0\ndrm-engine-gfx:\t9900000000 ns\n")

	gpus = []models.GPUStats{identity}
	next := NewGopsUtil().collectGPUStats(pciRoot, procRoot, gpus, cursor, t0.Add(time.Second), noNvidia(t), nil)
	require.NotEmpty(t, next)
	require.NotNil(t, gpus[0].Usage)
	assert.InDelta(t, 60.0, *gpus[0].Usage, 0.001, "max over engines")

	gpus = []models.GPUStats{identity}
	NewGopsUtil().collectGPUStats(pciRoot, procRoot, gpus, next, t0.Add(time.Second), noNvidia(t), nil)
	assert.Nil(t, gpus[0].Usage, "No time elapsed since the cursor")
}

func TestCollectGPUStats_CursorSkipsIdleEngines(t *testing.T) {
	pciRoot := t.TempDir()
	writeHwmonFiles(t, filepath.Join(pciRoot, "0000:00:02.0"), map[string]string{"drm/renderD128/dev": "226:128\n"})
	procRoot := t.TempDir()
	writeDRMClient(t, procRoot, 100, 5, "renderD128", i915Client(1, 500, 0))
	writeDRMClient(t, procRoot, 200, 5, "renderD128", i915Client(2, 0, 0))

	gpus := []models.GPUStats{{PciSlot: "0000:00:02.0", Driver: "i915"}}
	cursor := NewGopsUtil().collectGPUStats(pciRoot, procRoot, gpus, "", time.Now(), noNvidia(t), nil)
	prev, _, ok := decodeGPUStatsCursor(cursor)
	require.True(t, ok)
	require.Len(t, prev, 1, "A client without activity is not stored")
	client := prev[drmClientKey{PDev: "0000:00:02.0", ID: 1}]
	assert.Equal(t, []string{"render"}, slices.Collect(maps.Keys(client.Engines)))
}

func TestCollectGPUStats_CorruptCursorIsEmpty(t *testing.T) {
	pciRoot := t.TempDir()
	writeHwmonFiles(t, filepath.Join(pciRoot, "0000:00:02.0"), map[string]string{"power/runtime_status": "active\n"})
	procRoot := t.TempDir()
	writeDRMClient(t, procRoot, 100, 5, "renderD128", i915Client(1, 500, 0))

	for _, cursor := range []string{"!!!", "bm90IGpzb24"} {
		gpus := []models.GPUStats{{PciSlot: "0000:00:02.0", Driver: "xe"}}
		next := NewGopsUtil().collectGPUStats(pciRoot, procRoot, gpus, cursor, time.Now(), noNvidia(t), nil)
		assert.NotEmpty(t, next)
		assert.Nil(t, gpus[0].Usage)
	}
}

func TestCollectGPUStats_XeClocks(t *testing.T) {
	pciRoot := t.TempDir()
	writeHwmonFiles(t, filepath.Join(pciRoot, "0000:03:00.0"), map[string]string{
		"tile0/gt0/freq0/act_freq": "1100\n",
		"tile0/gt0/freq0/rp0_freq": "2400\n",
	})
	gpus := []models.GPUStats{{PciSlot: "0000:03:00.0", Driver: "xe"}}
	NewGopsUtil().collectGPUStats(pciRoot, t.TempDir(), gpus, "", time.Now(), noNvidia(t), nil)
	require.NotNil(t, gpus[0].CoreClock)
	assert.Equal(t, 1100.0, *gpus[0].CoreClock)
	require.NotNil(t, gpus[0].CoreClockMax)
	assert.Equal(t, 2400.0, *gpus[0].CoreClockMax)
}

func TestCollectGPUStats_NvidiaMergedByBDF(t *testing.T) {
	pciRoot := t.TempDir()
	writeHwmonFiles(t, filepath.Join(pciRoot, "0000:01:00.0"), map[string]string{"power/runtime_status": "active\n"})

	calls := 0
	nvidia := func(bdfs []string) map[string]models.GPUStats {
		assert.Equal(t, []string{"0000:01:00.0"}, bdfs)
		calls++
		return parseNvidiaSMI("00000000:01:00.0, 42, 1024, 12282, 2100, 2805, 10501, 10501, 55, 120.5, 200.00, 30, 4, 4, 16, 16\n")
	}
	gpus := []models.GPUStats{{PciSlot: "0000:01:00.0", PciId: "10de:2786", Driver: "nvidia", Vendor: "NVIDIA", Name: "NVIDIA GeForce RTX 4070"}}
	NewGopsUtil().collectGPUStats(pciRoot, t.TempDir(), gpus, "", time.Now(), nvidia, nil)

	assert.Equal(t, 1, calls)
	g := gpus[0]
	assert.Equal(t, "NVIDIA GeForce RTX 4070", g.Name)
	assert.Equal(t, "10de:2786", g.PciId)
	require.NotNil(t, g.Usage)
	assert.Equal(t, 42.0, *g.Usage)
	require.NotNil(t, g.VRAMTotal)
	assert.Equal(t, uint64(12282)<<20, *g.VRAMTotal)
	require.NotNil(t, g.Power)
	assert.Equal(t, 120.5, *g.Power)
	assert.Equal(t, "16.0 GT/s PCIe", g.PcieLinkSpeed)
}

func TestCollectGPUStats_NvidiaQueriesAwakeOnly(t *testing.T) {
	pciRoot := t.TempDir()
	writeHwmonFiles(t, filepath.Join(pciRoot, "0000:01:00.0"), map[string]string{"power/runtime_status": "suspended\n"})
	writeHwmonFiles(t, filepath.Join(pciRoot, "0000:02:00.0"), map[string]string{"power/runtime_status": "active\n"})

	var got []string
	nvidia := func(bdfs []string) map[string]models.GPUStats {
		got = bdfs
		return nil
	}
	gpus := []models.GPUStats{
		{PciSlot: "0000:01:00.0", Driver: "nvidia"},
		{PciSlot: "0000:02:00.0", Driver: "nvidia"},
	}
	NewGopsUtil().collectGPUStats(pciRoot, t.TempDir(), gpus, "", time.Now(), nvidia, nil)
	assert.Equal(t, []string{"0000:02:00.0"}, got)
}

func TestCollectGPUStats_SuspendedDGPUFdinfoNotOpened(t *testing.T) {
	pciRoot := t.TempDir()
	writeHwmonFiles(t, filepath.Join(pciRoot, "0000:00:02.0"), map[string]string{
		"power/runtime_status": "active\n",
		"drm/renderD128/dev":   "226:128\n",
	})
	writeHwmonFiles(t, filepath.Join(pciRoot, "0000:0c:00.0"), map[string]string{
		"power/runtime_status": "suspended\n",
		"drm/renderD129/dev":   "226:129\n",
	})
	procRoot := t.TempDir()
	writeDRMClient(t, procRoot, 100, 5, "renderD128", i915Client(1, 1_000_000_000, 0))
	// A FIFO with no writer blocks any open for read.
	writeDRMClient(t, procRoot, 100, 6, "renderD129", "")
	fifo := filepath.Join(procRoot, "100", "fdinfo", "6")
	require.NoError(t, os.Remove(fifo))
	require.NoError(t, syscall.Mkfifo(fifo, 0o644))

	gpus := []models.GPUStats{
		{PciSlot: "0000:00:02.0", Driver: "i915", Vendor: "Intel"},
		{PciSlot: "0000:0c:00.0", Driver: "amdgpu", Vendor: "AMD"},
	}
	done := make(chan string, 1)
	go func() {
		done <- NewGopsUtil().collectGPUStats(pciRoot, procRoot, gpus, "", time.Now(), noNvidia(t), nil)
	}()
	select {
	case cursor := <-done:
		assert.NotEmpty(t, cursor)
	case <-time.After(5 * time.Second):
		t.Fatal("fdinfo of a suspended GPU node was opened")
	}
	assert.True(t, gpus[1].Suspended)
}

func TestCollectGPUStats_SharedClientsFromProcessScan(t *testing.T) {
	pciRoot := t.TempDir()
	writeHwmonFiles(t, filepath.Join(pciRoot, "0000:00:02.0"), map[string]string{"drm/renderD128/dev": "226:128\n"})
	procRoot := t.TempDir()
	// What the process list collects with process-gpu.
	shared := func() drmClientSet {
		r := buildProcGPUReader(pciRoot, []models.GPU{{PciSlot: "0000:00:02.0", Driver: "i915"}}, nil, func() map[int32]uint64 { return nil })
		set := drmClientSet{}
		pids, err := listProcPIDs(procRoot)
		require.NoError(t, err)
		for _, pid := range pids {
			g, _ := processExtras{gpu: r}.readFDs(procRoot, pid)
			maps.Copy(set, g.drm)
		}
		return set
	}
	gpus := []models.GPUStats{{PciSlot: "0000:00:02.0", Driver: "i915"}}
	empty := t.TempDir()
	t0 := time.Unix(1000, 0)

	writeDRMClient(t, procRoot, 100, 5, "renderD128", i915Client(1, 1_000_000_000, 0))
	cursor := NewGopsUtil().collectGPUStats(pciRoot, empty, gpus, "", t0, noNvidia(t), shared())
	writeDRMClient(t, procRoot, 100, 5, "renderD128", i915Client(1, 1_500_000_000, 0))
	NewGopsUtil().collectGPUStats(pciRoot, empty, gpus, cursor, t0.Add(time.Second), noNvidia(t), shared())
	require.NotNil(t, gpus[0].Usage, "the shared clients are used instead of scanning procRoot")
	assert.InDelta(t, 50.0, *gpus[0].Usage, 0.001)
}
