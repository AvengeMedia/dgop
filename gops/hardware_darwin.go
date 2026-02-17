//go:build darwin

package gops

import (
	"os/exec"
	"strings"

	"github.com/AvengeMedia/dgop/models"
)

func getBIOSInfo() models.BIOSInfo {
	model := "Unknown"
	out, err := exec.Command("sysctl", "-n", "hw.model").Output()
	if err == nil {
		model = strings.TrimSpace(string(out))
	}

	return models.BIOSInfo{
		Vendor:      "Apple",
		Motherboard: model,
		Version:     "N/A",
	}
}

func getDistroName() string {
	name, err := exec.Command("sw_vers", "-productName").Output()
	if err != nil {
		return "macOS"
	}

	version, err := exec.Command("sw_vers", "-productVersion").Output()
	if err != nil {
		return strings.TrimSpace(string(name))
	}

	return strings.TrimSpace(string(name)) + " " + strings.TrimSpace(string(version))
}

func detectGPUEntries() ([]gpuEntry, error) {
	out, err := exec.Command("system_profiler", "SPDisplaysDataType").Output()
	if err != nil {
		return nil, err
	}

	var entries []gpuEntry
	for _, line := range strings.Split(string(out), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "Chipset Model:") {
			continue
		}
		gpuName := strings.TrimSpace(strings.TrimPrefix(trimmed, "Chipset Model:"))
		vendor := inferVendor("", gpuName)
		entries = append(entries, gpuEntry{
			Priority: 1,
			Vendor:   vendor,
			RawLine:  gpuName,
		})
	}
	return entries, nil
}

func getNvidiaTemperature() (float64, string) {
	return 0, "unknown"
}

func getHwmonTemperature(_ string) (float64, string) {
	return 0, "unknown"
}
