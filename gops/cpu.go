package gops

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AvengeMedia/dgop/models"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/sensors"
)

type CPUTracker struct {
	lastTotal  []float64
	lastCores  [][]float64
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
	return self.GetCPUInfoWithCursor("")
}

func (self *GopsUtil) GetCPUInfoWithCursor(cursor string) (*models.CPUInfo, error) {
	cpuInfo := models.CPUInfo{}

	cpuTracker.mu.Lock()
	defer cpuTracker.mu.Unlock()

	if !cpuTracker.modelCached {
		cpuTracker.cpuCount, _ = cpu.Counts(true)
		info, err := cpu.Info()
		if err == nil && len(info) > 0 {
			cpuTracker.cpuModel = info[0].ModelName
			cpuTracker.cpuFreq = info[0].Mhz
		}
		cpuTracker.modelCached = true
	}

	// Get current CPU frequency (more accurate than cached base frequency)
	currentFreq := getCurrentCPUFreq()
	if currentFreq > 0 {
		cpuInfo.Frequency = currentFreq
	} else {
		cpuInfo.Frequency = cpuTracker.cpuFreq
	}

	cpuInfo.Count = cpuTracker.cpuCount
	cpuInfo.Model = cpuTracker.cpuModel

	now := time.Now()
	if now.Sub(cpuTracker.tempLastRead) > 5*time.Second {
		cpuTracker.tempValue = getCPUTemperatureCached()
		cpuTracker.tempLastRead = now
	}
	cpuInfo.Temperature = cpuTracker.tempValue

	times, err := cpu.Times(false)
	if err == nil && len(times) > 0 {
		t := times[0]
		cpuInfo.Total = []float64{
			t.User, t.Nice, t.System,
			t.Idle, t.Iowait, t.Irq,
			t.Softirq, t.Steal,
		}
	}

	perCore, err := cpu.Times(true)
	if err == nil {
		cpuInfo.Cores = make([][]float64, len(perCore))
		for i, c := range perCore {
			cpuInfo.Cores[i] = []float64{
				c.User, c.Nice, c.System,
				c.Idle, c.Iowait, c.Irq,
				c.Softirq, c.Steal,
			}
		}
	}

	currentTime := now.UnixMilli()

	var cursorData models.CPUCursorData
	if cursor != "" {
		jsonBytes, err := base64.RawURLEncoding.DecodeString(cursor)
		if err == nil {
			json.Unmarshal(jsonBytes, &cursorData)
		}
	}

	if len(cursorData.Total) > 0 && len(cpuInfo.Total) > 0 && cursorData.Timestamp > 0 {
		timeDiff := float64(currentTime-cursorData.Timestamp) / 1000.0
		if timeDiff > 0 {
			cpuInfo.Usage = calculateCPUPercentage(cursorData.Total, cpuInfo.Total)

			if len(cursorData.Cores) > 0 && len(cpuInfo.Cores) > 0 {
				cpuInfo.CoreUsage = make([]float64, len(cpuInfo.Cores))
				for i := 0; i < len(cpuInfo.Cores) && i < len(cursorData.Cores); i++ {
					cpuInfo.CoreUsage[i] = calculateCPUPercentage(cursorData.Cores[i], cpuInfo.Cores[i])
				}
			}
		}
	} else {
		cpuPercent, err := cpu.Percent(100*time.Millisecond, false)
		if err == nil && len(cpuPercent) > 0 {
			cpuInfo.Usage = cpuPercent[0]
		}

		corePercent, err := cpu.Percent(100*time.Millisecond, true)
		if err == nil {
			cpuInfo.CoreUsage = corePercent
		}
	}

	newCursor := models.CPUCursorData{
		Total:     cpuInfo.Total,
		Cores:     cpuInfo.Cores,
		Timestamp: currentTime,
	}
	cursorBytes, _ := json.Marshal(newCursor)
	cpuInfo.Cursor = base64.RawURLEncoding.EncodeToString(cursorBytes)

	return &cpuInfo, nil
}

func getCPUTemperatureCached() float64 {
	// Try gopsutil sensors first (preferred method)
	temps, err := sensors.SensorsTemperatures()
	if err == nil {
		for _, temp := range temps {
			// Look for CPU temperature sensors
			if strings.Contains(temp.SensorKey, "coretemp_core_0") ||
				strings.Contains(temp.SensorKey, "k10temp_tdie") ||
				strings.Contains(temp.SensorKey, "cpu_thermal") ||
				strings.Contains(temp.SensorKey, "package_id_0") {
				return temp.Temperature
			}
		}
	}

	// Fallback to hwmon if gopsutil doesn't work
	if cpuTracker.tempPath != "" {
		tempBytes, err := os.ReadFile(cpuTracker.tempPath)
		if err == nil {
			temp, err := strconv.Atoi(strings.TrimSpace(string(tempBytes)))
			if err == nil {
				return float64(temp) / 1000.0
			}
		}
	}

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
			strings.Contains(name, "k8temp") || strings.Contains(name, "cpu_thermal") || strings.Contains(name, "zenpower") {
			tempPath := filepath.Join(hwmonPath, entry.Name(), "temp1_input")
			tempBytes, err := os.ReadFile(tempPath)
			if err == nil {
				temp, err := strconv.Atoi(strings.TrimSpace(string(tempBytes)))
				if err == nil {
					cpuTracker.tempPath = tempPath
					return float64(temp) / 1000.0
				}
			}
		}
	}

	return 0
}

func getCurrentCPUFreq() float64 {
	// Try to read current frequency from /proc/cpuinfo
	cpuinfoBytes, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return 0
	}

	lines := strings.Split(string(cpuinfoBytes), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "cpu MHz") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				freq, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
				if err == nil {
					return freq
				}
			}
		}
	}

	// Try scaling_cur_freq as fallback
	freqBytes, err := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq")
	if err == nil {
		freq, err := strconv.Atoi(strings.TrimSpace(string(freqBytes)))
		if err == nil {
			return float64(freq) / 1000.0 // Convert from kHz to MHz
		}
	}

	return 0
}

func calculateCPUPercentage(prev, curr []float64) float64 {
	if len(prev) < 8 || len(curr) < 8 {
		return 0
	}

	prevUser, prevNice, prevSystem := prev[0], prev[1], prev[2]
	prevIdle, prevIowait := prev[3], prev[4]
	prevIrq, prevSoftirq, prevSteal := prev[5], prev[6], prev[7]

	currUser, currNice, currSystem := curr[0], curr[1], curr[2]
	currIdle, currIowait := curr[3], curr[4]
	currIrq, currSoftirq, currSteal := curr[5], curr[6], curr[7]

	prevTotal := prevUser + prevNice + prevSystem + prevIdle + prevIowait + prevIrq + prevSoftirq + prevSteal
	currTotal := currUser + currNice + currSystem + currIdle + currIowait + currIrq + currSoftirq + currSteal

	prevBusy := prevTotal - prevIdle - prevIowait
	currBusy := currTotal - currIdle - currIowait

	if currBusy <= prevBusy {
		return 0
	}
	if currTotal <= prevTotal {
		return 100
	}

	usage := (currBusy - prevBusy) / (currTotal - prevTotal) * 100.0

	if usage < 0 {
		return 0
	}
	if usage > 100 {
		return 100
	}

	return usage
}
