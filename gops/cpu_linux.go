//go:build linux

package gops

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

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

	freqBytes, err := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq")
	if err == nil {
		freq, err := strconv.Atoi(strings.TrimSpace(string(freqBytes)))
		if err == nil {
			return float64(freq) / 1000.0
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
