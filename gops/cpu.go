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
)

type CPUTracker struct {
	cpuModel    string
	cpuFreq     float64
	cpuCount    int
	modelCached bool

	tempPath     string
	tempLastRead time.Time
	tempValue    float64

	freqLastRead time.Time
	freqValue    float64

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
		cpuTracker.cpuCount, _ = self.cpuProvider.Counts(true)
		info, err := self.cpuProvider.Info()
		if err == nil && len(info) > 0 {
			cpuTracker.cpuModel = info[0].ModelName
			cpuTracker.cpuFreq = info[0].Mhz
		}
		cpuTracker.modelCached = true
	}

	now := time.Now()
	if now.Sub(cpuTracker.freqLastRead) > 2*time.Second {
		cpuTracker.freqValue = getCurrentCPUFreq()
		cpuTracker.freqLastRead = now
	}
	if cpuTracker.freqValue > 0 {
		cpuInfo.Frequency = cpuTracker.freqValue
	} else {
		cpuInfo.Frequency = cpuTracker.cpuFreq
	}

	cpuInfo.Count = cpuTracker.cpuCount
	cpuInfo.Model = cpuTracker.cpuModel

	if now.Sub(cpuTracker.tempLastRead) > 5*time.Second {
		cpuTracker.tempValue = getCPUTemperatureCached()
		cpuTracker.tempLastRead = now
	}
	cpuInfo.Temperature = cpuTracker.tempValue

	times, err := self.cpuProvider.Times(false)
	if err == nil && len(times) > 0 {
		t := times[0]
		cpuInfo.Total = []float64{
			t.User, t.Nice, t.System,
			t.Idle, t.Iowait, t.Irq,
			t.Softirq, t.Steal,
		}
	}

	perCore, err := self.cpuProvider.Times(true)
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
		if jsonBytes, err := base64.RawURLEncoding.DecodeString(cursor); err == nil {
			if err := json.Unmarshal(jsonBytes, &cursorData); err != nil {
				cursorData = models.CPUCursorData{}
			}
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
		cpuPercent, err := self.cpuProvider.Percent(50*time.Millisecond, false)
		if err == nil && len(cpuPercent) > 0 {
			cpuInfo.Usage = cpuPercent[0]
		}

		corePercent, err := self.cpuProvider.Percent(50*time.Millisecond, true)
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
	if cpuTracker.tempPath != "" {
		tempBytes, err := os.ReadFile(cpuTracker.tempPath)
		if err != nil {
			cpuTracker.tempPath = ""
			return getCPUTemperatureCached()
		}
		temp, err := strconv.Atoi(strings.TrimSpace(string(tempBytes)))
		if err != nil {
			cpuTracker.tempPath = ""
			return getCPUTemperatureCached()
		}
		return float64(temp) / 1000.0
	}

	// Read directly from CPU-specific hwmon devices only.
	// Avoid gopsutil sensors.SensorsTemperatures() as it enumerates ALL hwmon
	// devices including GPU, which can wake discrete GPUs from D3cold.
	hwmonPath := "/sys/class/hwmon"
	entries, err := os.ReadDir(hwmonPath)
	if err != nil {
		return getACPITZFallback()
	}

	cpuHwmonNames := []string{"coretemp", "k10temp", "k8temp", "cpu_thermal", "zenpower"}
	for _, entry := range entries {
		namePath := filepath.Join(hwmonPath, entry.Name(), "name")
		nameBytes, err := os.ReadFile(namePath)
		if err != nil {
			continue
		}

		name := strings.TrimSpace(string(nameBytes))
		found := false
		for _, cpuName := range cpuHwmonNames {
			if name == cpuName {
				found = true
				break
			}
		}
		if !found {
			continue
		}

		tempPath := filepath.Join(hwmonPath, entry.Name(), "temp1_input")
		tempBytes, err := os.ReadFile(tempPath)
		if err != nil {
			continue
		}
		temp, err := strconv.Atoi(strings.TrimSpace(string(tempBytes)))
		if err != nil {
			continue
		}
		cpuTracker.tempPath = tempPath
		return float64(temp) / 1000.0
	}

	return getACPITZFallback()
}

func getACPITZFallback() float64 {
	thermalPath := "/sys/class/thermal"
	thermalEntries, err := os.ReadDir(thermalPath)
	if err != nil {
		return 0
	}
	return getMaxACPITZTemperature(thermalPath, thermalEntries, 20, 100, true)
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

func getMaxACPITZTemperature(thermalPath string, thermalEntries []os.DirEntry, minTemp, maxTemp float64, isCPU bool) float64 {
	var highestTemp float64

	for _, entry := range thermalEntries {
		if !strings.HasPrefix(entry.Name(), "thermal_zone") {
			continue
		}

		thermalType, err := readThermalType(thermalPath, entry.Name())
		if err != nil {
			continue
		}

		if thermalType != "acpitz" {
			continue
		}

		temp, tempPath, err := readThermalTemp(thermalPath, entry.Name())
		if err != nil {
			continue
		}

		if temp < minTemp || temp > maxTemp {
			continue
		}

		if temp > highestTemp {
			highestTemp = temp
			if isCPU {
				cpuTracker.tempPath = tempPath
			}
		}
	}

	return highestTemp
}

func readThermalType(thermalPath, entryName string) (string, error) {
	typePath := filepath.Join(thermalPath, entryName, "type")
	typeBytes, err := os.ReadFile(typePath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(typeBytes)), nil
}

func readThermalTemp(thermalPath, entryName string) (float64, string, error) {
	tempPath := filepath.Join(thermalPath, entryName, "temp")
	tempBytes, err := os.ReadFile(tempPath)
	if err != nil {
		return 0, "", err
	}

	temp, err := strconv.Atoi(strings.TrimSpace(string(tempBytes)))
	if err != nil {
		return 0, "", err
	}

	tempC := float64(temp) / 1000.0
	return tempC, tempPath, nil
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
