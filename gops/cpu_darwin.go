//go:build darwin

package gops

func getCPUTemperatureCached() float64 {
	return 0
}

func getCurrentCPUFreq() float64 {
	return 0
}
