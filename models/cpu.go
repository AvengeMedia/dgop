package models

type CPUInfo struct {
	Count       int        `json:"count"`
	Model       string     `json:"model"`
	Frequency   float64    `json:"frequency"`
	Temperature float64    `json:"temperature"`
	Usage       float64    `json:"usage"`
	CoreUsage   []float64  `json:"coreUsage"`
	Total       []uint64   `json:"total"`
	Cores       [][]uint64 `json:"cores"`
}

type CPUSampleData struct {
	PreviousTotal []uint64   `json:"previousTotal,omitempty"`
	PreviousCores [][]uint64 `json:"previousCores,omitempty"`
	SampleTime    int64      `json:"sampleTime,omitempty"`
}
