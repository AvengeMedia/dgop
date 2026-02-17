//go:build darwin

package gops

import (
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
)

func getCPUTemperatureCached() float64 {
	return 0
}

func getCurrentCPUFreq() float64 {
	return 0
}

// cpuUsageFromProvider uses gopsutil's cpu.Percent on macOS.
// The custom tick-ratio calculation is unreliable on Apple Silicon because
// host_processor_info may not account for parked efficiency cores correctly,
// inflating the busy/total ratio. gopsutil's Percent(0) uses its own internal
// delta cache and is tested on macOS.
func cpuUsageFromProvider(cpuProvider CPUInfoProvider, cursorTotal, currentTotal []float64, timeDiff float64, numCPUs int) (float64, []float64) {
	totalPercent := 0.0
	var corePercents []float64

	// Percent(0) returns delta from last Percent() call.
	// First call after the initial Percent(100ms) seed will have a good baseline.
	total, err := cpu.Percent(0, false)
	if err == nil && len(total) > 0 {
		totalPercent = total[0]
	}

	perCore, err := cpu.Percent(0, true)
	if err == nil {
		corePercents = perCore
	}

	return totalPercent, corePercents
}

// Prime gopsutil's internal CPU cache so subsequent Percent(0) calls work.
func primeCPUPercent() {
	cpu.Percent(200*time.Millisecond, false)
	cpu.Percent(200*time.Millisecond, true)
}
