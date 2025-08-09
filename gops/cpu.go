package gops

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bbedward/DankMaterialShell/dankgop/models"
	"github.com/shirou/gopsutil/v4/cpu"
)

type CPUTracker struct {
	lastTotal  []uint64
	lastCores  [][]uint64
	lastUpdate time.Time

	// Cache for expensive operations
	cpuModel    string
	cpuFreq     float64
	cpuCount    int
	modelCached bool

	// Temperature cache
	tempPath     string
	tempLastRead time.Time
	tempValue    float64

	mu sync.RWMutex
}

var cpuTracker = &CPUTracker{}

func (self *GopsUtil) GetCPUInfo() (*models.CPUInfo, error) {
	cpuInfo := models.CPUInfo{}

	cpuTracker.mu.Lock()
	defer cpuTracker.mu.Unlock()

	// Get static CPU info (cache this - it never changes)
	if !cpuTracker.modelCached {
		cpuTracker.cpuCount, _ = cpu.Counts(true)
		info, err := cpu.Info()
		if err == nil && len(info) > 0 {
			cpuTracker.cpuModel = info[0].ModelName
			cpuTracker.cpuFreq = info[0].Mhz
		}
		cpuTracker.modelCached = true
	}

	cpuInfo.Count = cpuTracker.cpuCount
	cpuInfo.Model = cpuTracker.cpuModel
	cpuInfo.Frequency = cpuTracker.cpuFreq

	// Get CPU temperature (cache for 5 seconds)
	now := time.Now()
	if now.Sub(cpuTracker.tempLastRead) > 5*time.Second {
		cpuTracker.tempValue = getCPUTemperatureCached()
		cpuTracker.tempLastRead = now
	}
	cpuInfo.Temperature = cpuTracker.tempValue

	// Get CPU times - this is fast, keep as-is
	times, err := cpu.Times(false)
	if err == nil && len(times) > 0 {
		t := times[0]
		cpuInfo.Total = []uint64{
			uint64(t.User), uint64(t.Nice), uint64(t.System),
			uint64(t.Idle), uint64(t.Iowait), uint64(t.Irq),
			uint64(t.Softirq), uint64(t.Steal),
		}
	}

	// Per-core CPU times
	perCore, err := cpu.Times(true)
	if err == nil {
		cpuInfo.Cores = make([][]uint64, len(perCore))
		for i, c := range perCore {
			cpuInfo.Cores[i] = []uint64{
				uint64(c.User), uint64(c.Nice), uint64(c.System),
				uint64(c.Idle), uint64(c.Iowait), uint64(c.Irq),
				uint64(c.Softirq), uint64(c.Steal),
			}
		}
	}

	// Get CPU usage percentage (averaged across all cores)
	cpuPercent, err := cpu.Percent(100*time.Millisecond, false)
	if err == nil && len(cpuPercent) > 0 {
		cpuInfo.Usage = cpuPercent[0]
	}

	// Get per-core CPU usage percentages
	corePercent, err := cpu.Percent(100*time.Millisecond, true)
	if err == nil {
		cpuInfo.CoreUsage = corePercent
	}

	// Update tracker
	cpuTracker.lastTotal = cpuInfo.Total
	cpuTracker.lastCores = cpuInfo.Cores
	cpuTracker.lastUpdate = time.Now()

	return &cpuInfo, nil
}

func getCPUTemperatureCached() float64 {
	// If we already found the temp path, use it
	if cpuTracker.tempPath != "" {
		tempBytes, err := os.ReadFile(cpuTracker.tempPath)
		if err == nil {
			temp, err := strconv.Atoi(strings.TrimSpace(string(tempBytes)))
			if err == nil {
				return float64(temp) / 1000.0
			}
		}
	}

	// Otherwise, search for it (and cache the path)
	hwmonPath := "/sys/class/hwmon"
	entries, err := os.ReadDir(hwmonPath)
	if err != nil {
		return 0
	}

	for _, entry := range entries {
		namePath := filepath.Join(hwmonPath, entry.Name(), "name")
		nameBytes, err := os.ReadFile(namePath)
		if err != nil {
			continue
		}

		name := strings.TrimSpace(string(nameBytes))
		if strings.Contains(name, "coretemp") || strings.Contains(name, "k10temp") ||
			strings.Contains(name, "k8temp") || strings.Contains(name, "cpu_thermal") {
			tempPath := filepath.Join(hwmonPath, entry.Name(), "temp1_input")
			tempBytes, err := os.ReadFile(tempPath)
			if err == nil {
				temp, err := strconv.Atoi(strings.TrimSpace(string(tempBytes)))
				if err == nil {
					cpuTracker.tempPath = tempPath // Cache the path
					return float64(temp) / 1000.0
				}
			}
		}
	}

	return 0
}
