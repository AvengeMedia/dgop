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
	Cursor      string      `json:"cursor,omitempty"`
}

type CPUCursorData struct {
	Total     []float64   `json:"total"`
	Cores     [][]float64 `json:"cores"`
	Timestamp int64       `json:"timestamp"`
}
