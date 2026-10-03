package models

type ProcessInfo struct {
	PID               int32   `json:"pid"`
	PPID              int32   `json:"ppid"`
	CPU               float64 `json:"cpu" doc:"Percentage of total machine CPU capacity used since the cursor, in the range 0-100."`
	PTicks            float64 `json:"pticks" doc:"Cumulative CPU seconds (user + system) consumed by this process."`
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
	ExecutablePath    string  `json:"executablePath,omitempty"`
	ChildCount        int     `json:"childCount,omitempty"`

	StartTime int64    `json:"startTime,omitempty" doc:"Process start, unix seconds."`
	GPU       *float64 `json:"gpu,omitempty" doc:"Percentage of the busiest GPU engine used since the cursor, in the range 0-100."`
	VRAMKB    *uint64  `json:"vramKB,omitempty" doc:"GPU memory in KB."`
	// GPUUnmonitored distinguishes unknown GPU values (gpu and vramKB absent) caused by a GPU that is excluded or runtime-suspended from an unreadable process.
	GPUUnmonitored bool     `json:"gpuUnmonitored,omitempty" doc:"True when the process holds a GPU that is excluded or suspended, so gpu and vramKB are deliberately not measured."`
	DiskRead       *float64 `json:"diskRead,omitempty" doc:"Disk bytes read per second since the cursor."`
	DiskWrite      *float64 `json:"diskWrite,omitempty" doc:"Disk bytes written per second since the cursor."`
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
