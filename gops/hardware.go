package gops

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/AvengeMedia/dgop/models"
	"github.com/shirou/gopsutil/v4/host"
)

func (self *GopsUtil) GetSystemHardware() (*models.SystemHardware, error) {
	info := &models.SystemHardware{}

	cpuInfo, err := self.GetCPUInfo()
	if err == nil {
		info.CPU = models.CPUBasic{
			Count: cpuInfo.Count,
			Model: cpuInfo.Model,
		}
	} else {
		info.CPU = models.CPUBasic{
			Count: 0,
			Model: "Unknown",
		}
	}

	biosInfo := getBIOSInfo()
	info.BIOS = biosInfo

	hostInfo, err := host.Info()
	if err != nil {
		return nil, err
	}

	info.Kernel = hostInfo.KernelVersion
	info.Hostname = hostInfo.Hostname
	info.Arch = hostInfo.KernelArch
	info.Distro = getDistroName()

	return info, nil
}

func (self *GopsUtil) GetGPUInfo() (*models.GPUInfo, error) {
	gpus, err := detectGPUs()
	if err != nil {
		return nil, err
	}

	return &models.GPUInfo{GPUs: gpus}, nil
}

func (self *GopsUtil) GetGPUInfoWithTemp(pciIds []string) (*models.GPUInfo, error) {
	gpus, err := detectGPUs()
	if err != nil {
		return nil, err
	}

	if len(pciIds) > 0 {
		for i, gpu := range gpus {
			for _, pciId := range pciIds {
				if gpu.PciId == pciId {
					if tempInfo, err := self.GetGPUTemp(pciId); err == nil {
						gpus[i].Temperature = tempInfo.Temperature
						gpus[i].Hwmon = tempInfo.Hwmon
					}
					break
				}
			}
		}
	}

	return &models.GPUInfo{GPUs: gpus}, nil
}

func (self *GopsUtil) GetGPUTemp(pciId string) (*models.GPUTempInfo, error) {
	if pciId == "" {
		return nil, fmt.Errorf("pciId is required")
	}

	gpuEntries, err := detectGPUEntries()
	if err != nil {
		return nil, err
	}

	var targetGPU *gpuEntry
	for _, gpu := range gpuEntries {
		_, gpuPciId := parseGPUInfo(gpu.RawLine)
		if gpuPciId == pciId {
			targetGPU = &gpu
			break
		}
	}

	if targetGPU == nil {
		return nil, fmt.Errorf("GPU with PCI ID %s not found", pciId)
	}

	var temperature float64
	var hwmon string

	switch targetGPU.Driver {
	case "nvidia":
		temperature, hwmon = getNvidiaTemperature()
	default:
		temperature, hwmon = getHwmonTemperature(pciId)
	}

	return &models.GPUTempInfo{
		Driver:      targetGPU.Driver,
		Hwmon:       hwmon,
		Temperature: temperature,
	}, nil
}

type gpuEntry struct {
	Priority int
	Driver   string
	Vendor   string
	RawLine  string
}

func inferVendorFromId(vendorId, driver string) string {
	switch vendorId {
	case "10de":
		return "NVIDIA"
	case "1002":
		return "AMD"
	case "8086":
		return "Intel"
	default:
		return inferVendor(driver, "")
	}
}

func detectGPUs() ([]models.GPU, error) {
	gpuEntries, err := detectGPUEntries()
	if err != nil {
		return nil, err
	}

	var gpus []models.GPU
	for _, entry := range gpuEntries {
		displayName, pciId := parseGPUInfo(entry.RawLine)
		fullName := buildFullName(entry.Vendor, displayName)

		gpus = append(gpus, models.GPU{
			Driver:      entry.Driver,
			Vendor:      entry.Vendor,
			DisplayName: displayName,
			FullName:    fullName,
			PciId:       pciId,
			RawLine:     entry.RawLine,
			Temperature: 0,
			Hwmon:       "unknown",
		})
	}

	return gpus, nil
}

func inferVendor(driver, line string) string {
	switch driver {
	case "nvidia", "nouveau":
		return "NVIDIA"
	case "amdgpu", "radeon":
		return "AMD"
	case "i915", "xe":
		return "Intel"
	}

	lineLower := strings.ToLower(line)
	switch {
	case strings.Contains(lineLower, "nvidia"):
		return "NVIDIA"
	case strings.Contains(lineLower, "amd"), strings.Contains(lineLower, "ati"):
		return "AMD"
	case strings.Contains(lineLower, "intel"):
		return "Intel"
	case strings.Contains(lineLower, "apple"):
		return "Apple"
	}

	return "Unknown"
}

func getPriority(driver, bdf string) int {
	switch driver {
	case "nvidia":
		return 3
	case "amdgpu", "radeon":
		parts := strings.Split(bdf, ":")
		if len(parts) >= 3 {
			deviceFunc := parts[2]
			if strings.HasPrefix(deviceFunc, "00.") {
				return 1
			}
		}
		return 2
	case "i915", "xe":
		return 0
	default:
		return 0
	}
}

func parseGPUInfo(rawLine string) (displayName, pciId string) {
	if rawLine == "" {
		return "Unknown", ""
	}

	pciRegex := regexp.MustCompile(`\[([0-9a-f]{4}:[0-9a-f]{4})\]`)
	if match := pciRegex.FindStringSubmatch(rawLine); len(match) > 1 {
		pciId = match[1]
	}

	s := regexp.MustCompile(`^[^:]+: `).ReplaceAllString(rawLine, "")
	s = regexp.MustCompile(`\[[0-9a-f]{4}:[0-9a-f]{4}\].*$`).ReplaceAllString(s, "")

	afterBracketRegex := regexp.MustCompile(`\]\s*([^\[]+)$`)
	if match := afterBracketRegex.FindStringSubmatch(s); len(match) > 1 && strings.TrimSpace(match[1]) != "" {
		displayName = strings.TrimSpace(match[1])
	} else {
		lastBracketRegex := regexp.MustCompile(`\[([^\]]+)\]([^\[]*$)`)
		if match := lastBracketRegex.FindStringSubmatch(s); len(match) > 1 {
			displayName = match[1]
		} else {
			displayName = s
		}
	}

	displayName = removeVendorPrefixes(displayName)

	if displayName == "" {
		displayName = "Unknown"
	}

	return displayName, pciId
}

func removeVendorPrefixes(name string) string {
	prefixes := []string{
		"NVIDIA Corporation ",
		"NVIDIA ",
		"Advanced Micro Devices, Inc. ",
		"AMD/ATI ",
		"AMD ",
		"ATI ",
		"Intel Corporation ",
		"Intel ",
		"Apple ",
	}

	result := name
	for _, prefix := range prefixes {
		if strings.HasPrefix(strings.ToLower(result), strings.ToLower(prefix)) {
			result = result[len(prefix):]
			break
		}
	}

	return strings.TrimSpace(result)
}

func buildFullName(vendor, displayName string) string {
	if displayName == "Unknown" {
		return displayName
	}

	switch vendor {
	case "NVIDIA":
		return "NVIDIA " + displayName
	case "AMD":
		return "AMD " + displayName
	case "Intel":
		return "Intel " + displayName
	case "Apple":
		return "Apple " + displayName
	default:
		return displayName
	}
}

func readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
