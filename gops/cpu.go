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
	return self.GetCPUInfoWithSample(nil)
}

func (self *GopsUtil) GetCPUInfoWithSample(sampleData *models.CPUSampleData) (*models.CPUInfo, error) {
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

	// Calculate CPU usage - use sample data if provided, otherwise use gopsutil
	if sampleData != nil && len(sampleData.PreviousTotal) > 0 && len(cpuInfo.Total) > 0 {
		cpuInfo.Usage = calculateCPUPercentage(sampleData.PreviousTotal, cpuInfo.Total)
		
		// Calculate per-core usage if we have previous core data
		if len(sampleData.PreviousCores) > 0 && len(cpuInfo.Cores) > 0 {
			cpuInfo.CoreUsage = make([]float64, len(cpuInfo.Cores))
			for i := 0; i < len(cpuInfo.Cores) && i < len(sampleData.PreviousCores); i++ {
				cpuInfo.CoreUsage[i] = calculateCPUPercentage(sampleData.PreviousCores[i], cpuInfo.Cores[i])
			}
		}
	} else {
		// Fallback to gopsutil for real-time measurement
		cpuPercent, err := cpu.Percent(100*time.Millisecond, false)
		if err == nil && len(cpuPercent) > 0 {
			cpuInfo.Usage = cpuPercent[0]
		}

		// Get per-core CPU usage percentages
		corePercent, err := cpu.Percent(100*time.Millisecond, true)
		if err == nil {
			cpuInfo.CoreUsage = corePercent
		}
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

func calculateCPUPercentage(prev, curr []uint64) float64 {
	if len(prev) < 4 || len(curr) < 4 {
		return 0
	}
	
	// CPU times: user, nice, system, idle, iowait, irq, softirq, steal
	prevIdle := prev[3]
	prevTotal := uint64(0)
	for _, v := range prev {
		prevTotal += v
	}
	
	currIdle := curr[3]
	currTotal := uint64(0)
	for _, v := range curr {
		currTotal += v
	}
	
	totalDiff := currTotal - prevTotal
	idleDiff := currIdle - prevIdle
	
	if totalDiff == 0 {
		return 0
	}
	
	usage := float64(totalDiff-idleDiff) / float64(totalDiff) * 100.0
	if usage < 0 {
		return 0
	}
	return usage
}
