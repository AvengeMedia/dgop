//go:build linux

package gops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/AvengeMedia/dgop/models"
)

func getBIOSInfo() models.BIOSInfo {
	dmip := "/sys/class/dmi/id"
	if _, err := os.Stat(dmip); os.IsNotExist(err) {
		dmip = "/sys/devices/virtual/dmi/id"
	}

	biosInfo := models.BIOSInfo{}

	if vendor, err := readFile(filepath.Join(dmip, "board_vendor")); err == nil {
		biosInfo.Vendor = strings.TrimSpace(vendor)
	} else {
		biosInfo.Vendor = "Unknown"
	}

	var boardName string
	if name, err := readFile(filepath.Join(dmip, "board_name")); err == nil {
		boardName = strings.TrimSpace(name)
	}

	switch {
	case biosInfo.Vendor != "Unknown" && boardName != "":
		biosInfo.Motherboard = biosInfo.Vendor + " " + boardName
	case boardName != "":
		biosInfo.Motherboard = boardName
	default:
		biosInfo.Motherboard = "Unknown"
	}

	if version, err := readFile(filepath.Join(dmip, "bios_version")); err == nil {
		biosInfo.Version = strings.TrimSpace(version)
	} else {
		biosInfo.Version = "Unknown"
	}

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

	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			distro := strings.TrimPrefix(line, "PRETTY_NAME=")
			distro = strings.Trim(distro, "\"")
			return distro
		}
	}

	return "Unknown"
}

func detectGPUEntries() ([]gpuEntry, error) {
	devices, err := filepath.Glob("/sys/bus/pci/devices/*")
	if err != nil {
		return nil, err
	}

	var gpuEntries []gpuEntry
	for _, devicePath := range devices {
		classBytes, err := os.ReadFile(filepath.Join(devicePath, "class"))
		if err != nil {
			continue
		}

		class := strings.TrimSpace(string(classBytes))
		if !strings.HasPrefix(class, "0x03") {
			continue
		}

		vendorBytes, err := os.ReadFile(filepath.Join(devicePath, "vendor"))
		if err != nil {
			continue
		}

		deviceBytes, err := os.ReadFile(filepath.Join(devicePath, "device"))
		if err != nil {
			continue
		}

		vendorId := strings.TrimSpace(strings.TrimPrefix(string(vendorBytes), "0x"))
		deviceId := strings.TrimSpace(strings.TrimPrefix(string(deviceBytes), "0x"))
		bdf := filepath.Base(devicePath)
		driver := getGPUDriver(bdf)
		displayName := lookupPCIDevice(vendorId, deviceId)
		vendor := inferVendorFromId(vendorId, driver)
		priority := getPriority(driver, bdf)
		pciId := fmt.Sprintf("%s:%s", vendorId, deviceId)
		rawLine := fmt.Sprintf("%s Display controller: %s [%s]", bdf, displayName, pciId)

		gpuEntries = append(gpuEntries, gpuEntry{
			Priority: priority,
			Driver:   driver,
			Vendor:   vendor,
			RawLine:  rawLine,
		})
	}

	sort.Slice(gpuEntries, func(i, j int) bool {
		if gpuEntries[i].Priority != gpuEntries[j].Priority {
			return gpuEntries[i].Priority > gpuEntries[j].Priority
		}
		return gpuEntries[i].Driver < gpuEntries[j].Driver
	})

	//fmt.Println(gpuEntries[0].Driver)

	return gpuEntries, nil
}

func lookupPCIDevice(vendorId, deviceId string) string {
	pciIdsPaths := []string{
		"/usr/share/hwdata/pci.ids",
		"/usr/share/misc/pci.ids",
		"/var/lib/pciutils/pci.ids",
	}

	var pciIdsPath string
	for _, path := range pciIdsPaths {
		if _, err := os.Stat(path); err == nil {
			pciIdsPath = path
			break
		}
	}

	if pciIdsPath == "" {
		return fmt.Sprintf("GPU %s:%s", vendorId, deviceId)
	}

	content, err := os.ReadFile(pciIdsPath)
	if err != nil {
		return fmt.Sprintf("GPU %s:%s", vendorId, deviceId)
	}

	inVendor := false
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "#") || len(strings.TrimSpace(line)) == 0 {
			continue
		}

		if !strings.HasPrefix(line, "\t") {
			inVendor = strings.HasPrefix(line, vendorId)
			continue
		}

		if !inVendor || strings.HasPrefix(line, "\t\t") {
			continue
		}

		fields := strings.SplitN(strings.TrimPrefix(line, "\t"), " ", 2)
		if len(fields) >= 2 && fields[0] == deviceId {
			return strings.TrimSpace(fields[1])
		}
	}

	return fmt.Sprintf("GPU %s:%s", vendorId, deviceId)
}

func getGPUDriver(bdf string) string {
	driverPath := filepath.Join("/sys/bus/pci/devices", bdf, "driver")
	if link, err := os.Readlink(driverPath); err == nil {
		return filepath.Base(link)
	}
	return ""
}

func getNvidiaTemperature() (float64, string) {
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
	drmCards, err := filepath.Glob("/sys/class/drm/card*")
	if err != nil {
		return 0, "unknown"
	}

	for _, card := range drmCards {
		devicePath := filepath.Join(card, "device")

		vendorFile := filepath.Join(devicePath, "vendor")
		deviceFile := filepath.Join(devicePath, "device")

		vendorBytes, err1 := os.ReadFile(vendorFile)
		deviceBytes, err2 := os.ReadFile(deviceFile)

		if err1 != nil || err2 != nil {
			continue
		}

		vendorId := strings.TrimSpace(string(vendorBytes))
		deviceId := strings.TrimSpace(string(deviceBytes))

		vendorId = strings.TrimPrefix(vendorId, "0x")
		deviceId = strings.TrimPrefix(deviceId, "0x")

		cardPciId := fmt.Sprintf("%s:%s", vendorId, deviceId)

		if cardPciId != pciId {
			continue
		}

		driverPath := filepath.Join(devicePath, "driver")
		if _, err := os.Stat(driverPath); os.IsNotExist(err) {
			continue
		}

		// Build the standard path used by AMD/Generic cards
		hwmonGlob := filepath.Join(devicePath, "hwmon", "hwmon*")
		hwmonDirs, err := filepath.Glob(hwmonGlob)

		// VIBE CODED BY GEMINI
		// INTEL INTEGRATED FALLBACK: Only trigger if the hardware belongs to Intel (8086)
		// AND the system is running integrated graphics architecture (i915 driver)
		if (err != nil || len(hwmonDirs) == 0) && vendorId == "8086" {
			driverLink, driverErr := os.Readlink(filepath.Join(devicePath, "driver"))
			isIntegratedIntel := driverErr == nil && strings.Contains(filepath.Base(driverLink), "i915")

			if isIntegratedIntel {
				globalHwmons, globalErr := filepath.Glob("/sys/class/hwmon/hwmon*")
				if globalErr == nil {
					for _, gHwmon := range globalHwmons {
						nameBytes, nameErr := os.ReadFile(filepath.Join(gHwmon, "name"))
						if nameErr == nil {
							nameStr := strings.TrimSpace(string(nameBytes))
							// Target Intel's CPU/iGPU package temperature layer safely
							if nameStr == "coretemp" {
								hwmonDirs = []string{gHwmon}
								break
							}
						}
					}
				}
			}
		}

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

	thermalPath := "/sys/class/thermal"
	thermalEntries, err := os.ReadDir(thermalPath)
	if err != nil {
		return 0, "unknown"
	}

	maxTemp := getMaxACPITZTemperatureForGPU(thermalPath, thermalEntries, 20, 90)
	if maxTemp > 0 {
		return maxTemp, "acpitz"
	}

	return 0, "unknown"
}

func getMaxACPITZTemperatureForGPU(thermalPath string, thermalEntries []os.DirEntry, minTemp, maxTemp float64) float64 {
	var highestTemp float64

	for _, entry := range thermalEntries {
		if !strings.HasPrefix(entry.Name(), "thermal_zone") {
			continue
		}

		thermalType, err := readThermalTypeForGPU(thermalPath, entry.Name())
		if err != nil {
			continue
		}

		if thermalType != "acpitz" {
			continue
		}

		temp, err := readThermalTempForGPU(thermalPath, entry.Name())
		if err != nil {
			continue
		}

		if temp < minTemp || temp > maxTemp {
			continue
		}

		if temp > highestTemp {
			highestTemp = temp
		}
	}

	return highestTemp
}

func readThermalTypeForGPU(thermalPath, entryName string) (string, error) {
	typePath := filepath.Join(thermalPath, entryName, "type")
	typeBytes, err := os.ReadFile(typePath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(typeBytes)), nil
}

func readThermalTempForGPU(thermalPath, entryName string) (float64, error) {
	tempPath := filepath.Join(thermalPath, entryName, "temp")
	tempBytes, err := os.ReadFile(tempPath)
	if err != nil {
		return 0, err
	}

	temp, err := strconv.Atoi(strings.TrimSpace(string(tempBytes)))
	if err != nil {
		return 0, err
	}

	return float64(temp) / 1000.0, nil
}

// VIBE CODED BY GEMINI
// Global state trackers for the PMU reader loop
var (
	intelPerfFd      int       = -1
	lastIntelTicks   uint64
	lastSampleTime   time.Time
)

// VIBE CODED BY GEMINI
// initIntelPMU opens the direct kernel performance event counter
func initIntelPMU() error {
	typeBytes, err := os.ReadFile("/sys/bus/event_source/devices/i915/type")
	if err != nil {
		typeBytes, err = os.ReadFile("/sys/bus/event_source/devices/xe/type")
		if err != nil {
			return err
		}
	}
	pmuType, err := strconv.ParseUint(strings.TrimSpace(string(typeBytes)), 10, 32)
	if err != nil {
		return err
	}

	attr := make([]uint64, 16)

	// Pack Type and Size. Modern kernels require 112 bytes for VER0/VER1 configurations
	attr[0] = uint64(pmuType) | (uint64(112) << 32)
	attr[1] = 0 // config=0x0 for rcs0-busy (Intel Render engine)

	// Bit 0 inside index 4 maps to 'disabled'. Setting it to 0 forces the counter to ENABLE immediately
	attr[4] = 0

	fd, _, sysErr := syscall.Syscall6(
		syscall.SYS_PERF_EVENT_OPEN,
		uintptr(unsafe.Pointer(&attr[0])),
		^uintptr(0), // PID -1 (System-wide context tracking)
		uintptr(0),  // CPU 0
		^uintptr(0), // Group FD -1
		uintptr(0),  // Flags 0
		0,
	)

	if sysErr != 0 {
		return sysErr
	}

	intelPerfFd = int(fd)
	return nil
}


// VIBE CODED BY GEMINI
func getIntelGpuUsage() float64 {
	// Initialize the connection once if it's closed
	if intelPerfFd == -1 {
		if err := initIntelPMU(); err != nil {
			return 0.0
		}
	}

	// 1. Perform ONE instantaneous system counter file-descriptor read
	var buf [8]byte
	n, err := syscall.Read(intelPerfFd, buf[:])
	if err != nil || n < 8 {
		return 0.0
	}
	currentTicks := *(*uint64)(unsafe.Pointer(&buf))
	currentTime := time.Now()

	// 2. Establish baseline memory state on the very first daemon iteration pass
	if lastSampleTime.IsZero() {
		lastIntelTicks = currentTicks
		lastSampleTime = currentTime
		return 0.0
	}

	// 3. Measure time differences utilizing the daemon's natural background window intervals
	measuredDuration := uint64(currentTime.Sub(lastSampleTime).Nanoseconds())
	busyDuration := currentTicks - lastIntelTicks

	// Prevent mathematical divide-by-zero crashes during layout glitches
	if measuredDuration == 0 {
		return 0.0
	}

	// 4. Calculate engine load percentage
	usagePercent := (float64(busyDuration) / float64(measuredDuration)) * 100.0

	// Handle standard clamping boundaries safely
	if usagePercent > 100.0 { usagePercent = 100.0 }
	if usagePercent < 0.0   { usagePercent = 0.0 }

	// 5. Explicitly cache current parameters to drive the NEXT background daemon loop
	lastIntelTicks = currentTicks
	lastSampleTime = currentTime

	return usagePercent
}
