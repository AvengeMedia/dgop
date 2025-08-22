package gops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/AvengeMedia/dgop/models"
	"github.com/shirou/gopsutil/v4/host"
)

func (self *GopsUtil) GetSystemHardware() (*models.SystemHardware, error) {
	info := &models.SystemHardware{}

	// Get CPU info from existing CPU API
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

	// Get BIOS info
	biosInfo := getBIOSInfo()
	info.BIOS = biosInfo

	// Get system info using gopsutil
	hostInfo, err := host.Info()
	if err != nil {
		return nil, err
	}

	info.Kernel = hostInfo.KernelVersion
	info.Hostname = hostInfo.Hostname
	info.Arch = hostInfo.KernelArch
	// Use manual distro detection for better results
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

	// If PCI IDs are specified, get temperatures for those GPUs
	if len(pciIds) > 0 {
		for i, gpu := range gpus {
			for _, pciId := range pciIds {
				if gpu.PciId == pciId {
					// Get temperature for this specific GPU
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

	// Find the GPU by PCI ID
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

	// Auto-detect temperature method based on driver
	var temperature float64
	var hwmon string

	if targetGPU.Driver == "nvidia" {
		temperature, hwmon = getNvidiaTemperature()
	} else {
		temperature, hwmon = getHwmonTemperature(pciId)
	}

	return &models.GPUTempInfo{
		Driver:      targetGPU.Driver,
		Hwmon:       hwmon,
		Temperature: temperature,
	}, nil
}

func getBIOSInfo() models.BIOSInfo {
	dmip := "/sys/class/dmi/id"
	if _, err := os.Stat(dmip); os.IsNotExist(err) {
		dmip = "/sys/devices/virtual/dmi/id"
	}

	biosInfo := models.BIOSInfo{}

	// Read motherboard vendor
	if vendor, err := readFile(filepath.Join(dmip, "board_vendor")); err == nil {
		biosInfo.Vendor = strings.TrimSpace(vendor)
	} else {
		biosInfo.Vendor = "Unknown"
	}

	// Read motherboard name
	var boardName string
	if name, err := readFile(filepath.Join(dmip, "board_name")); err == nil {
		boardName = strings.TrimSpace(name)
	}

	// Combine vendor and board name
	if biosInfo.Vendor != "Unknown" && boardName != "" {
		biosInfo.Motherboard = biosInfo.Vendor + " " + boardName
	} else if boardName != "" {
		biosInfo.Motherboard = boardName
	} else {
		biosInfo.Motherboard = "Unknown"
	}

	// Read BIOS version
	if version, err := readFile(filepath.Join(dmip, "bios_version")); err == nil {
		biosInfo.Version = strings.TrimSpace(version)
	} else {
		biosInfo.Version = "Unknown"
	}

	// Read BIOS date
	if date, err := readFile(filepath.Join(dmip, "bios_date")); err == nil {
		biosInfo.Date = strings.TrimSpace(date)
	}

	return biosInfo
}

func getDistroName() string {
	content, err := readFile("/etc/os-release")
	if err != nil {
		return "Unknown"
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			// Remove PRETTY_NAME= and quotes
			distro := strings.TrimPrefix(line, "PRETTY_NAME=")
			distro = strings.Trim(distro, "\"")
			return distro
		}
	}

	return "Unknown"
}

type gpuEntry struct {
	Priority int
	Driver   string
	Vendor   string
	RawLine  string
}

func detectGPUEntries() ([]gpuEntry, error) {
	cmd := exec.Command("lspci", "-nnD")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var gpuEntries []gpuEntry
	vgaRegex := regexp.MustCompile(`(?i) VGA| 3D| 2D| Display`)

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if vgaRegex.MatchString(line) {
			parts := strings.Fields(line)
			if len(parts) == 0 {
				continue
			}

			bdf := parts[0]
			driver := getGPUDriver(bdf)
			vendor := inferVendor(driver, line)
			priority := getPriority(driver, bdf)

			gpuEntries = append(gpuEntries, gpuEntry{
				Priority: priority,
				Driver:   driver,
				Vendor:   vendor,
				RawLine:  line,
			})
		}
	}

	// Sort by priority (descending), then by driver name
	sort.Slice(gpuEntries, func(i, j int) bool {
		if gpuEntries[i].Priority != gpuEntries[j].Priority {
			return gpuEntries[i].Priority > gpuEntries[j].Priority
		}
		return gpuEntries[i].Driver < gpuEntries[j].Driver
	})

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
			Temperature: 0,         // TODO: Add GPU temperature detection
			Hwmon:       "unknown", // TODO: Add hwmon path detection
		})
	}

	return gpuEntries, nil
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
			Temperature: 0,         // TODO: Add GPU temperature detection
			Hwmon:       "unknown", // TODO: Add hwmon path detection
		})
	}

	return gpus, nil
}

func getGPUDriver(bdf string) string {
	driverPath := filepath.Join("/sys/bus/pci/devices", bdf, "driver")
	if link, err := os.Readlink(driverPath); err == nil {
		return filepath.Base(link)
	}
	return ""
}

func inferVendor(driver, line string) string {
	// Check driver first
	switch driver {
	case "nvidia", "nouveau":
		return "NVIDIA"
	case "amdgpu", "radeon":
		return "AMD"
	case "i915", "xe":
		return "Intel"
	}

	// Check line content
	lineLower := strings.ToLower(line)
	if strings.Contains(lineLower, "nvidia") {
		return "NVIDIA"
	}
	if strings.Contains(lineLower, "amd") || strings.Contains(lineLower, "ati") {
		return "AMD"
	}
	if strings.Contains(lineLower, "intel") {
		return "Intel"
	}

	return "Unknown"
}

func getPriority(driver, bdf string) int {
	switch driver {
	case "nvidia":
		return 3
	case "amdgpu", "radeon":
		// Check if it's the primary GPU (device 00)
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

	// Extract PCI ID [vvvv:dddd]
	pciRegex := regexp.MustCompile(`\[([0-9a-f]{4}:[0-9a-f]{4})\]`)
	if match := pciRegex.FindStringSubmatch(rawLine); len(match) > 1 {
		pciId = match[1]
	}

	// Remove BDF and class prefix
	s := regexp.MustCompile(`^[^:]+: `).ReplaceAllString(rawLine, "")
	// Remove PCI ID [vvvv:dddd] and everything after
	s = regexp.MustCompile(`\[[0-9a-f]{4}:[0-9a-f]{4}\].*$`).ReplaceAllString(s, "")

	// Try to extract text after last ']'
	afterBracketRegex := regexp.MustCompile(`\]\s*([^\[]+)$`)
	if match := afterBracketRegex.FindStringSubmatch(s); len(match) > 1 && strings.TrimSpace(match[1]) != "" {
		displayName = strings.TrimSpace(match[1])
	} else {
		// Try to get last bracketed text
		lastBracketRegex := regexp.MustCompile(`\[([^\]]+)\]([^\[]*$)`)
		if match := lastBracketRegex.FindStringSubmatch(s); len(match) > 1 {
			displayName = match[1]
		} else {
			displayName = s
		}
	}

	// Remove vendor prefixes
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
	default:
		return displayName
	}
}

func getNvidiaTemperature() (float64, string) {
	// Use nvidia-smi to get GPU temperature
	cmd := exec.Command("nvidia-smi", "--query-gpu=temperature.gpu", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		return 0, "unknown"
	}

	tempStr := strings.TrimSpace(string(output))
	lines := strings.Split(tempStr, "\n")
	if len(lines) > 0 && lines[0] != "" {
		if temp, err := strconv.ParseFloat(lines[0], 64); err == nil {
			return temp, "nvidia"
		}
	}

	return 0, "unknown"
}

func getHwmonTemperature(pciId string) (float64, string) {
	// Convert PCI ID format to search for DRM cards
	// Look for /sys/class/drm/card* that match our PCI device
	drmCards, err := filepath.Glob("/sys/class/drm/card*")
	if err != nil {
		return 0, "unknown"
	}

	for _, card := range drmCards {
		// Check if this card's device driver matches our target
		devicePath := filepath.Join(card, "device")
		
		// Check if this device matches our PCI ID
		vendorFile := filepath.Join(devicePath, "vendor")
		deviceFile := filepath.Join(devicePath, "device")
		
		vendorBytes, err1 := os.ReadFile(vendorFile)
		deviceBytes, err2 := os.ReadFile(deviceFile)
		
		if err1 != nil || err2 != nil {
			continue
		}
		
		vendorId := strings.TrimSpace(string(vendorBytes))
		deviceId := strings.TrimSpace(string(deviceBytes))
		
		// Remove 0x prefix if present
		vendorId = strings.TrimPrefix(vendorId, "0x")
		deviceId = strings.TrimPrefix(deviceId, "0x")
		
		// Construct the PCI ID in the format vvvv:dddd
		cardPciId := fmt.Sprintf("%s:%s", vendorId, deviceId)
		
		// Only proceed if this card matches our target PCI ID
		if cardPciId != pciId {
			continue
		}

		driverPath := filepath.Join(devicePath, "driver")
		if _, err := os.Stat(driverPath); os.IsNotExist(err) {
			continue
		}

		// Look for hwmon directory under this device
		hwmonGlob := filepath.Join(devicePath, "hwmon", "hwmon*")
		hwmonDirs, err := filepath.Glob(hwmonGlob)
		if err != nil {
			continue
		}

		for _, hwmonDir := range hwmonDirs {
			tempFile := filepath.Join(hwmonDir, "temp1_input")
			if _, err := os.Stat(tempFile); os.IsNotExist(err) {
				continue
			}

			tempBytes, err := os.ReadFile(tempFile)
			if err != nil {
				continue
			}

			tempStr := strings.TrimSpace(string(tempBytes))
			if tempInt, err := strconv.Atoi(tempStr); err == nil {
				hwmonName := filepath.Base(hwmonDir)
				return float64(tempInt) / 1000.0, hwmonName
			}
		}
	}

	return 0, "unknown"
}

func readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
