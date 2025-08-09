package models

type ProcessInfo struct {
	PID           int32   `json:"pid"`
	PPID          int32   `json:"ppid"`
	CPU           float64 `json:"cpu"`
	PTicks        uint64  `json:"pticks"`
	MemoryPercent float32 `json:"memoryPercent"`
	MemoryKB      uint64  `json:"memoryKB"`
	PSSKB         uint64  `json:"pssKB"`
	PSSPercent    float32 `json:"pssPercent"`
	Command       string  `json:"command"`
	FullCommand   string  `json:"fullCommand"`
}

type ProcessSampleData struct {
	PID            int32   `json:"pid"`
	PreviousTicks  uint64  `json:"previousTicks,omitempty"`
	PreviousCPU    float64 `json:"previousCPU,omitempty"`
	SampleTime     int64   `json:"sampleTime,omitempty"`
}
