package models

type CPUInfo struct {
	Count       int         `json:"count"`
	Model       string      `json:"model"`
	Frequency   float64     `json:"frequency"`
	Temperature float64     `json:"temperature"`
	Usage       float64     `json:"usage"`
	CoreUsage   []float64   `json:"coreUsage"`
	Total       []float64   `json:"total"`
	Cores       [][]float64 `json:"cores"`
	Cursor      *CPUCursor  `json:"cursor,omitempty"`
}

type CPUSampleData struct {
	PreviousTotal []float64   `json:"previousTotal,omitempty"`
	PreviousCores [][]float64 `json:"previousCores,omitempty"`
	Timestamp     int64       `json:"timestamp,omitempty"`
}

type CPUCursor struct {
	Total     []float64   `json:"total"`
	Cores     [][]float64 `json:"cores"`
	Timestamp int64       `json:"timestamp"`
}
