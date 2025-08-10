package models

type ProcessInfo struct {
	PID           int32   `json:"pid"`
	PPID          int32   `json:"ppid"`
	CPU           float64 `json:"cpu"`
	PTicks        float64 `json:"pticks"`
	MemoryPercent float32 `json:"memoryPercent"`
	MemoryKB      uint64  `json:"memoryKB"`
	PSSKB         uint64  `json:"pssKB"`
	PSSPercent    float32 `json:"pssPercent"`
	Command       string  `json:"command"`
	FullCommand   string  `json:"fullCommand"`
}

type ProcessSampleData struct {
	PID           int32   `json:"pid"`
	PreviousTicks float64 `json:"previousTicks,omitempty"`
	Timestamp     int64   `json:"timestamp,omitempty"`
}

type ProcessCursor struct {
	PID       int32   `json:"pid"`
	Ticks     float64 `json:"ticks"`
	Timestamp int64   `json:"timestamp"`
}
