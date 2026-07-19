//go:build freebsd

package gops

import "golang.org/x/sys/unix"

// "IK" sysctls store decikelvin: val = celsius*10 + TZ_ZEROC (2731),
// per sys/dev/coretemp/coretemp.c and sys/dev/acpica/acpi_thermal.c.
const deciKelvinZeroC = 2731

func deciKelvinToCelsius(raw uint32) float64 {
	return float64(int32(raw)-deciKelvinZeroC) / 10.0
}

func getCPUTemperatureCached() float64 {
	// dev.cpu.%d.temperature per coretemp(4)/amdtemp(4).
	if raw, err := unix.SysctlUint32("dev.cpu.0.temperature"); err == nil {
		return deciKelvinToCelsius(raw)
	}

	// hw.acpi.thermal.tz%d.temperature per acpi_thermal(4).
	raw, err := unix.SysctlUint32("hw.acpi.thermal.tz0.temperature")
	if err != nil {
		return 0
	}

	temp := deciKelvinToCelsius(raw)
	if temp < 0 {
		return 0
	}
	return temp
}

func getCurrentCPUFreq() float64 {
	// dev.cpu.%d.freq per cpufreq(4), in MHz.
	freq, err := unix.SysctlUint32("dev.cpu.0.freq")
	if err != nil {
		return 0
	}
	return float64(freq)
}

// cpuUsageFromProvider on FreeBSD uses the tick-ratio approach like Linux;
// gopsutil reads kern.cp_times, which always includes all cores.
func cpuUsageFromProvider(_ CPUInfoProvider, cursorTotal, currentTotal []float64, _ float64, _ int) (float64, []float64) {
	return calculateCPUPercentage(cursorTotal, currentTotal), nil
}

func primeCPUPercent() {}
