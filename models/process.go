package models

type ProcessInfo struct {
	PID               int32   `json:"pid"`
	PPID              int32   `json:"ppid"`
	CPU               float64 `json:"cpu"`
	PTicks            float64 `json:"pticks"`
	MemoryPercent     float32 `json:"memoryPercent"`
	MemoryKB          uint64  `json:"memoryKB"`
	MemoryCalculation string  `json:"memoryCalculation"`
	RSSKB             uint64  `json:"rssKB"`
	RSSPercent        float32 `json:"rssPercent"`
	PSSKB             uint64  `json:"pssKB"`
	PSSPercent        float32 `json:"pssPercent"`
	Username          string  `json:"username"`
	Command           string  `json:"command"`
	FullCommand       string  `json:"fullCommand"`
}

type ProcessCursorData struct {
	PID       int32   `json:"pid"`
	Ticks     float64 `json:"ticks"`
	Timestamp int64   `json:"timestamp"`
}

type ProcessListResponse struct {
	Processes []*ProcessInfo `json:"processes"`
	Cursor    string         `json:"cursor,omitempty"`
}
