//go:build darwin

package gops

func getCPUTemperatureCached() float64 {
	return 0
}

func getCurrentCPUFreq() float64 {
	return 0
}

// cpuUsageFromTimes computes CPU usage using wall-time normalization.
// On macOS, host_processor_info may omit parked/sleeping cores from the
// aggregate, making the tick-ratio approach over-report usage. Dividing
// busy-time delta by (wall-clock seconds * core count) avoids this.
func cpuUsageFromTimes(prev, curr []float64, wallTimeSec float64, numCPUs int) float64 {
	if len(prev) < 3 || len(curr) < 3 || wallTimeSec <= 0 || numCPUs <= 0 {
		return 0
	}

	prevBusy := prev[0] + prev[1] + prev[2] // user + nice + system
	currBusy := curr[0] + curr[1] + curr[2]

	busyDelta := currBusy - prevBusy
	if busyDelta <= 0 {
		return 0
	}

	usage := busyDelta / (wallTimeSec * float64(numCPUs)) * 100.0
	if usage > 100 {
		return 100
	}
	return usage
}
