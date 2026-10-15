//go:build linux

package gops

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/AvengeMedia/dgop/models"
)

const nvidiaSmiTimeout = 2 * time.Second

var nvidiaSmiFields = []string{
	"pci.bus_id",
	"temperature.gpu",
	"utilization.gpu",
	"memory.used",
	"memory.total",
	"power.draw",
	"clocks.gr",
	"clocks.mem",
}

type gpuSampler struct {
	pciDevices string
	thermal    string
	nvidiaSmi  func(busIds []string) ([]byte, error)
}

var defaultGPUSampler = gpuSampler{
	pciDevices: "/sys/bus/pci/devices",
	thermal:    "/sys/class/thermal",
	nvidiaSmi:  runNvidiaSmi,
}

func sampleGPUs(entries []gpuEntry, prev gpuCursor, now time.Time) ([]gpuSample, gpuCursor) {
	return defaultGPUSampler.sample(entries, prev, now)
}

func (s gpuSampler) sample(entries []gpuEntry, prev gpuCursor, now time.Time) ([]gpuSample, gpuCursor) {
	samples := make([]gpuSample, len(entries))
	next := newGPUCursor(now)
	var nvidia []int

	for i, entry := range entries {
		sample := &samples[i]
		sample.Hwmon = "unknown"
		dev := filepath.Join(s.pciDevices, entry.BusId)

		// Most driver queries resume a runtime-suspended card, and a sleeping GPU
		// is idle by definition.
		if entry.BusId != "" && isRuntimeSuspended(dev) {
			sample.State = models.GPUStateSuspended
			sample.Usage = float64Ptr(0)
			continue
		}
		sample.State = models.GPUStateActive

		switch entry.Driver {
		case "nvidia":
			nvidia = append(nvidia, i)
			continue
		case "amdgpu":
			readAMDGPU(dev, sample)
		case "i915", "xe":
			readIntelGPU(dev, entry.Driver, entry.BusId, sample, prev, next)
		}

		readGPUHwmon(dev, entry.BusId, sample, prev, next)
		if sample.Temperature == 0 {
			if temp := s.acpiTemperature(); temp > 0 {
				sample.Temperature, sample.Hwmon = temp, "acpitz"
			}
		}
	}

	if len(nvidia) > 0 {
		s.sampleNvidia(entries, nvidia, samples)
	}

	return samples, next
}

func readAMDGPU(dev string, sample *gpuSample) {
	if busy, ok := readSysUint(filepath.Join(dev, "gpu_busy_percent")); ok {
		sample.Usage = float64Ptr(float64(busy))
	}
	used, okUsed := readSysUint(filepath.Join(dev, "mem_info_vram_used"))
	total, okTotal := readSysUint(filepath.Join(dev, "mem_info_vram_total"))
	if okUsed && okTotal && total > 0 {
		sample.MemoryUsed = uint64Ptr(used / 1024)
		sample.MemoryTotal = uint64Ptr(total / 1024)
	}
}

func isRuntimeSuspended(dev string) bool {
	switch readSysString(filepath.Join(dev, "power", "runtime_status")) {
	case "suspended", "suspending":
		return true
	}
	return false
}

// Intel exposes no busy counter without perf privileges, so usage is time the
// render GT spent out of RC6 between two samples. That overstates engine load
// somewhat, but it is what an unprivileged reader can get.
func readIntelGPU(dev, driver, busId string, sample *gpuSample, prev, next gpuCursor) {
	var idlePath, freqPath string
	switch driver {
	case "xe":
		gt := xeRenderGT(dev)
		idlePath = filepath.Join(gt, "gtidle", "idle_residency_ms")
		freqPath = filepath.Join(gt, "freq0", "act_freq")
		readXeVRAM(dev, sample)
	case "i915":
		card := drmCardDir(dev)
		if card == "" {
			return
		}
		gt := filepath.Join(card, "gt", "gt0")
		idlePath = filepath.Join(gt, "rc6_residency_ms")
		freqPath = filepath.Join(gt, "rps_act_freq_mhz")
		if _, err := os.Stat(gt); err != nil {
			idlePath = filepath.Join(card, "power", "rc6_residency_ms")
			freqPath = filepath.Join(card, "gt_act_freq_mhz")
		}
	}

	if mhz, ok := readSysUint(freqPath); ok {
		sample.CoreClock = float64Ptr(float64(mhz))
	}

	idleMs, ok := readSysUint(idlePath)
	if !ok {
		return
	}
	next.set(busId, "idle", idleMs)
	diff, seconds, ok := next.delta(prev, busId, "idle")
	if !ok || seconds <= 0 {
		return
	}
	busy := 100 * (1 - float64(diff)/1000/seconds)
	sample.Usage = float64Ptr(min(100, max(0, busy)))
}

// xeRenderGT picks the render GT; media GTs report MC6 and are named gtN-mc.
func xeRenderGT(dev string) string {
	names, _ := filepath.Glob(filepath.Join(dev, "tile*", "gt*", "gtidle", "name"))
	sort.Strings(names)
	for _, name := range names {
		if strings.HasSuffix(readSysString(name), "-rc") {
			return filepath.Dir(filepath.Dir(name))
		}
	}
	return filepath.Join(dev, "tile0", "gt0")
}

func drmCardDir(dev string) string {
	cards, _ := filepath.Glob(filepath.Join(dev, "drm", "card[0-9]*"))
	for _, card := range cards {
		if !strings.Contains(filepath.Base(card), "-") {
			return card
		}
	}
	return ""
}

// readGPUHwmon fills whatever the driver's hwmon node exposes and the driver
// specific readers left empty.
func readGPUHwmon(dev, busId string, sample *gpuSample, prev, next gpuCursor) {
	dirs, _ := filepath.Glob(filepath.Join(dev, "hwmon", "hwmon*"))
	sort.Strings(dirs)
	for _, dir := range dirs {
		// i915 adds per-GT energy-only nodes next to the card one.
		if strings.HasPrefix(readSysString(filepath.Join(dir, "name")), "i915_gt") {
			continue
		}

		if sample.Temperature == 0 {
			if temp, ok := readHwmonTemperature(dir); ok {
				sample.Temperature = temp
				sample.Hwmon = filepath.Base(dir)
			}
		}

		if sample.Power == nil {
			sample.Power = readHwmonPower(dir, busId, prev, next)
		}

		for _, n := range []string{"1", "2"} {
			hz, ok := readSysUint(filepath.Join(dir, "freq"+n+"_input"))
			if !ok {
				continue
			}
			mhz := float64Ptr(float64(hz) / 1e6)
			switch readSysString(filepath.Join(dir, "freq"+n+"_label")) {
			case "sclk":
				if sample.CoreClock == nil {
					sample.CoreClock = mhz
				}
			case "mclk":
				if sample.MemoryClock == nil {
					sample.MemoryClock = mhz
				}
			}
		}
	}
}

// readHwmonTemperature prefers temp1 (amdgpu edge, i915, nouveau) and falls
// back to the package sensor, which is where xe starts numbering.
func readHwmonTemperature(dir string) (float64, bool) {
	if milli, ok := readSysUint(filepath.Join(dir, "temp1_input")); ok {
		return float64(milli) / 1000, true
	}
	labels, _ := filepath.Glob(filepath.Join(dir, "temp*_label"))
	for _, label := range labels {
		if readSysString(label) != "pkg" {
			continue
		}
		input := strings.TrimSuffix(label, "_label") + "_input"
		if milli, ok := readSysUint(input); ok {
			return float64(milli) / 1000, true
		}
	}
	return 0, false
}

func readHwmonPower(dir, busId string, prev, next gpuCursor) *float64 {
	for _, name := range []string{"power1_average", "power1_input"} {
		if micro, ok := readSysUint(filepath.Join(dir, name)); ok {
			return float64Ptr(float64(micro) / 1e6)
		}
	}

	// Some drivers only publish a cumulative energy counter in microjoules.
	microJ, ok := readSysUint(filepath.Join(dir, "energy1_input"))
	if !ok {
		return nil
	}
	key := "energy:" + filepath.Base(dir)
	next.set(busId, key, microJ)
	diff, seconds, ok := next.delta(prev, busId, key)
	if !ok || seconds <= 0 {
		return nil
	}
	return float64Ptr(float64(diff) / 1e6 / seconds)
}

func (c gpuCursor) set(busId, name string, value uint64) {
	if c.Counters[busId] == nil {
		c.Counters[busId] = map[string]uint64{}
	}
	c.Counters[busId][name] = value
}

// delta returns how much a counter grew since prev and the elapsed time in
// seconds. ok is false on the first sample or when the counter went backwards.
func (c gpuCursor) delta(prev gpuCursor, busId, name string) (diff uint64, seconds float64, ok bool) {
	if prev.Timestamp == 0 || c.Timestamp <= prev.Timestamp {
		return 0, 0, false
	}
	cur, hasCur := c.Counters[busId][name]
	old, hasOld := prev.Counters[busId][name]
	if !hasCur || !hasOld || cur < old {
		return 0, 0, false
	}
	return cur - old, float64(c.Timestamp-prev.Timestamp) / float64(time.Second), true
}

func (s gpuSampler) acpiTemperature() float64 {
	entries, err := os.ReadDir(s.thermal)
	if err != nil {
		return 0
	}
	return getMaxACPITZTemperatureForGPU(s.thermal, entries, 20, 90)
}

func (s gpuSampler) sampleNvidia(entries []gpuEntry, indexes []int, samples []gpuSample) {
	busIds := make([]string, 0, len(indexes))
	for _, i := range indexes {
		if entries[i].BusId != "" {
			busIds = append(busIds, entries[i].BusId)
		}
	}
	if len(busIds) == 0 {
		return
	}

	out, err := s.nvidiaSmi(busIds)
	if err != nil {
		return
	}

	rows := parseNvidiaSmi(out)
	for _, i := range indexes {
		row, ok := rows[entries[i].BusId]
		if !ok {
			continue
		}
		sample := &samples[i]
		if row.Temperature != nil {
			sample.Temperature, sample.Hwmon = *row.Temperature, "nvidia"
		}
		sample.Usage = row.Usage
		sample.MemoryUsed = row.MemoryUsed
		sample.MemoryTotal = row.MemoryTotal
		sample.Power = row.Power
		sample.CoreClock = row.CoreClock
		sample.MemoryClock = row.MemoryClock
	}
}

func runNvidiaSmi(busIds []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), nvidiaSmiTimeout)
	defer cancel()
	return exec.CommandContext(ctx, "nvidia-smi",
		"--query-gpu="+strings.Join(nvidiaSmiFields, ","),
		"--format=csv,noheader,nounits",
		"--id="+strings.Join(busIds, ","),
	).Output()
}

type nvidiaSmiRow struct {
	Temperature *float64
	Usage       *float64
	MemoryUsed  *uint64
	MemoryTotal *uint64
	Power       *float64
	CoreClock   *float64
	MemoryClock *float64
}

// parseNvidiaSmi reads rows in nvidiaSmiFields order, keyed by sysfs bus id.
// Unsupported values come back as "[N/A]" or "[Not Supported]" and stay nil.
func parseNvidiaSmi(out []byte) map[string]nvidiaSmiRow {
	rows := map[string]nvidiaSmiRow{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Split(line, ",")
		if len(fields) != len(nvidiaSmiFields) {
			continue
		}
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}

		mib := func(s string) *uint64 {
			v := parseNvidiaFloat(s)
			if v == nil {
				return nil
			}
			return uint64Ptr(uint64(*v) * 1024)
		}

		rows[normalizeBusId(fields[0])] = nvidiaSmiRow{
			Temperature: parseNvidiaFloat(fields[1]),
			Usage:       parseNvidiaFloat(fields[2]),
			MemoryUsed:  mib(fields[3]),
			MemoryTotal: mib(fields[4]),
			Power:       parseNvidiaFloat(fields[5]),
			CoreClock:   parseNvidiaFloat(fields[6]),
			MemoryClock: parseNvidiaFloat(fields[7]),
		}
	}
	return rows
}

func parseNvidiaFloat(s string) *float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

func readSysString(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func readSysUint(path string) (uint64, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func float64Ptr(v float64) *float64 { return &v }

func uint64Ptr(v uint64) *uint64 { return &v }
