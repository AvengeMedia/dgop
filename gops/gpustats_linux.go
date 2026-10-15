//go:build linux

package gops

import (
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AvengeMedia/dgop/models"
)

const pciDevicesRoot = "/sys/bus/pci/devices"

type gpuStatsCursor struct {
	Time    int64             `json:"t"`
	Clients []gpuCursorClient `json:"c"`
}

func (self *GopsUtil) fillGPUStats(gpus []models.GPUStats, cursor string) string {
	return self.collectGPUStats(pciDevicesRoot, "/proc", gpus, cursor, time.Now(), queryNvidiaSMI)
}

// collectGPUStats fills gpus in place and returns the next cursor. Nothing
// under a runtime-suspended device is read, so a sleeping dGPU stays asleep.
func (self *GopsUtil) collectGPUStats(pciRoot, procRoot string, gpus []models.GPUStats, cursor string, now time.Time, nvidia func(bdfs []string) map[string]models.GPUStats) string {
	fdinfoGPUs := map[string]*models.GPUStats{}
	var nvidiaBDFs []string

	for i := range gpus {
		g := &gpus[i]
		dev := filepath.Join(pciRoot, g.PciSlot)
		if gpuSuspended(pciRoot, g.PciSlot) {
			g.Suspended = true
			continue
		}
		if g.Driver == "nvidia" {
			nvidiaBDFs = append(nvidiaBDFs, g.PciSlot)
			continue
		}
		self.readGPUSysfs(g, dev)
		if g.Usage == nil && drmDriverReportsEngines(g.Driver) {
			fdinfoGPUs[g.PciSlot] = g
		}
	}

	if len(nvidiaBDFs) > 0 {
		stats := nvidia(nvidiaBDFs)
		for i := range gpus {
			g := &gpus[i]
			s, ok := stats[g.PciSlot]
			if g.Driver != "nvidia" || g.Suspended || !ok {
				continue
			}
			s.PciSlot, s.PciId, s.Driver, s.Vendor, s.Name = g.PciSlot, g.PciId, g.Driver, g.Vendor, g.Name
			*g = s
		}
	}

	if len(fdinfoGPUs) == 0 {
		return ""
	}
	return fillFdinfoUsage(pciRoot, procRoot, fdinfoGPUs, cursor, now)
}

func gpuSuspended(pciRoot, bdf string) bool {
	return readTrimmed(filepath.Join(pciRoot, bdf, "power", "runtime_status")) == "suspended"
}

func (self *GopsUtil) readGPUSysfs(g *models.GPUStats, dev string) {
	g.Usage = readFloat(filepath.Join(dev, "gpu_busy_percent"))
	g.VRAMUsed = readUint(filepath.Join(dev, "mem_info_vram_used"))
	g.VRAMTotal = readUint(filepath.Join(dev, "mem_info_vram_total"))
	g.GTTUsed = readUint(filepath.Join(dev, "mem_info_gtt_used"))
	g.GTTTotal = readUint(filepath.Join(dev, "mem_info_gtt_total"))
	g.MemoryBusy = readFloat(filepath.Join(dev, "mem_busy_percent"))

	hwmons, _ := filepath.Glob(filepath.Join(dev, "hwmon", "hwmon*"))
	var firstTemp *float64
	for _, dir := range hwmons {
		chip, ok := self.readHwmonChip(dir)
		if !ok {
			continue
		}
		for _, r := range chip.Readings {
			v := r.Value
			if r.Kind == sensorTemperature && firstTemp == nil {
				firstTemp = &v
			}
			switch {
			case r.Kind == sensorTemperature && r.Label == "edge":
				g.Temperature = &v
			case r.Kind == sensorTemperature && r.Label == "junction":
				g.Hotspot = &v
			case r.Kind == sensorTemperature && r.Label == "mem":
				g.MemTemp = &v
			case r.Kind == sensorPower && g.Power == nil:
				g.Power, g.PowerLimit = &v, r.Max
			case r.Kind == sensorFan && g.FanRpm == nil:
				rpm := int(v)
				g.FanRpm = &rpm
				if r.Max != nil {
					maxRpm := int(*r.Max)
					g.FanMaxRpm = &maxRpm
					if *r.Max > 0 {
						pct := min(v / *r.Max * 100, 100)
						g.FanPercent = &pct
					}
				}
			case r.Kind == sensorFrequency && r.Label == "sclk":
				g.CoreClock = &v
			case r.Kind == sensorFrequency && r.Label == "mclk":
				g.MemClock = &v
			}
		}
	}
	if g.Temperature == nil {
		g.Temperature = firstTemp
	}
	// PWM duty is what nvidia-smi reports as fan speed, so prefer it over rpm.
	for _, dir := range hwmons {
		pwm, pwmMax := readFloat(filepath.Join(dir, "pwm1")), readFloat(filepath.Join(dir, "pwm1_max"))
		if pwm != nil && pwmMax != nil && *pwmMax > 0 {
			pct := min(*pwm / *pwmMax * 100, 100)
			g.FanPercent = &pct
			break
		}
	}

	cur, maxClock := readDPMLevels(filepath.Join(dev, "pp_dpm_sclk"))
	g.CoreClock, g.CoreClockMax = firstSet(g.CoreClock, cur), maxClock
	cur, maxClock = readDPMLevels(filepath.Join(dev, "pp_dpm_mclk"))
	g.MemClock, g.MemClockMax = firstSet(g.MemClock, cur), maxClock

	cards, _ := filepath.Glob(filepath.Join(dev, "drm", "card[0-9]*"))
	for _, card := range cards {
		g.CoreClock = firstSet(g.CoreClock, readFloat(filepath.Join(card, "gt_act_freq_mhz")), readScaled(filepath.Join(card, "gt_cur_freq_mhz"), 1))
		g.CoreClockMax = firstSet(g.CoreClockMax, readScaled(filepath.Join(card, "gt_RP0_freq_mhz"), 1), readScaled(filepath.Join(card, "gt_max_freq_mhz"), 1))
	}
	xe := filepath.Join(dev, "tile0", "gt0", "freq0")
	g.CoreClock = firstSet(g.CoreClock, readFloat(filepath.Join(xe, "act_freq")))
	g.CoreClockMax = firstSet(g.CoreClockMax, readScaled(filepath.Join(xe, "rp0_freq"), 1))

	g.PcieLinkSpeed = readLinkSpeed(filepath.Join(dev, "current_link_speed"))
	g.PcieLinkSpeedMax = readLinkSpeed(filepath.Join(dev, "max_link_speed"))
	g.PcieLinkWidth, _ = strconv.Atoi(readTrimmed(filepath.Join(dev, "current_link_width")))
	g.PcieLinkWidthMax, _ = strconv.Atoi(readTrimmed(filepath.Join(dev, "max_link_width")))
}

// readDPMLevels parses amdgpu pp_dpm_* files ("1: 500Mhz *"). The "S:" sleep
// level is not a clock and is skipped even when it is current.
func readDPMLevels(path string) (cur, maxClock *float64) {
	for line := range strings.SplitSeq(readTrimmed(path), "\n") {
		level, rest, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(level) == "S" {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		mhz, err := strconv.ParseFloat(strings.TrimSuffix(strings.ToLower(fields[0]), "mhz"), 64)
		if err != nil {
			continue
		}
		if maxClock == nil || mhz > *maxClock {
			maxClock = &mhz
		}
		if len(fields) > 1 && fields[1] == "*" {
			cur = &mhz
		}
	}
	return cur, maxClock
}

func readLinkSpeed(path string) string {
	s := readTrimmed(path)
	if s == "Unknown" {
		return ""
	}
	return s
}

func firstSet[T any](values ...*T) *T {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

// fillFdinfoUsage scans every process's DRM fds, keeps the clients of the
// given GPUs, and rates them against the previous sample in cursor.
func fillFdinfoUsage(pciRoot, procRoot string, gpus map[string]*models.GPUStats, cursor string, now time.Time) string {
	read := make([]models.GPU, 0, len(gpus))
	for slot := range gpus {
		read = append(read, models.GPU{PciSlot: slot})
	}
	nodes := gpuNodes(pciRoot, read, nil).nodes
	cur := drmClientSet{}
	pids, _ := listProcPIDs(procRoot)
	for _, pid := range pids {
		scan, err := scanProcFDs(procRoot, pid, nodes, false)
		if err != nil {
			continue
		}
		for key, client := range scan.DRM {
			if gpus[key.PDev] != nil {
				cur[key] = client
			}
		}
	}

	if prev, prevTime, ok := decodeGPUStatsCursor(cursor); ok && now.After(prevTime) {
		byPDev := map[string]drmClientSet{}
		for key, client := range cur {
			if byPDev[key.PDev] == nil {
				byPDev[key.PDev] = drmClientSet{}
			}
			byPDev[key.PDev][key] = client
		}
		for pdev, g := range gpus {
			usage := maxEngineUtilization(drmEngineUtilization(prev, byPDev[pdev], now.Sub(prevTime)))
			g.Usage = &usage
		}
	}

	return encodeGPUStatsCursor(cur, now)
}

func encodeGPUStatsCursor(clients drmClientSet, now time.Time) string {
	b, _ := json.Marshal(gpuStatsCursor{Time: now.UnixNano(), Clients: cursorClients(clients)})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeGPUStatsCursor(cursor string) (drmClientSet, time.Time, bool) {
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, time.Time{}, false
	}
	var data gpuStatsCursor
	if err := json.Unmarshal(b, &data); err != nil || data.Time <= 0 {
		return nil, time.Time{}, false
	}
	return cursorClientSet(data.Clients), time.Unix(0, data.Time), true
}

// cursorClients returns the clients with at least one nonzero engine counter;
// an absent engine rates the same as one at zero.
func cursorClients(clients drmClientSet) []gpuCursorClient {
	result := make([]gpuCursorClient, 0, len(clients))
	for _, client := range clients {
		if c := toCursorClient(client); len(c.Engines) > 0 {
			result = append(result, c)
		}
	}
	return result
}

func toCursorClient(client drmClient) gpuCursorClient {
	c := gpuCursorClient{PDev: client.PDev, ID: client.ID, Engines: map[string][3]uint64{}}
	for name, e := range client.Engines {
		if e.BusyNs|e.Cycles|e.TotalCycles != 0 {
			c.Engines[name] = [3]uint64{e.BusyNs, e.Cycles, e.TotalCycles}
		}
	}
	return c
}

// cursorClientSet restores a previous sample. Only the current sample's
// HasBusy and Capacity are used for rating, so these are left at defaults.
func cursorClientSet(clients []gpuCursorClient) drmClientSet {
	set := drmClientSet{}
	for _, c := range clients {
		client := drmClient{PDev: c.PDev, ID: c.ID, Engines: map[string]drmEngine{}}
		for name, v := range c.Engines {
			client.Engines[name] = drmEngine{BusyNs: v[0], HasBusy: true, Cycles: v[1], TotalCycles: v[2], Capacity: 1}
		}
		set[client.key()] = client
	}
	return set
}
