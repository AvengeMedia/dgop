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
	ChildCount        int     `json:"childCount,omitempty" doc:"Processes merged into this row. A merged row sums its members; a value unknown for any member is absent for the row."`
	Status            string  `json:"status,omitempty" doc:"State letter from /proc/pid/stat: R, S, D, Z, T, I..."`
	Threads           int     `json:"threads,omitempty" doc:"Thread count; merged rows sum their members."`
	Nice              *int    `json:"nice,omitempty"`

	StartTime      int64    `json:"startTime,omitempty" doc:"Process start, unix seconds."`
	GPU            *float64 `json:"gpu,omitempty" doc:"Percentage of the busiest GPU engine used since the cursor, in the range 0-100. Merged rows sum each member's busiest engine, capped at 100."`
	VRAMKB         *uint64  `json:"vramKB,omitempty" doc:"GPU memory in KB."`
	GPUVideo       *float64 `json:"gpuVideo,omitempty" doc:"Percentage of the busiest video engine (encode, decode, JPEG) used since the cursor. Merged rows sum each member's busiest video engine, capped at 100."`
	GPUGttKB       *uint64  `json:"gpuGttKB,omitempty" doc:"System memory mapped for the GPU (GTT), in KB."`
	GPUUnmonitored bool     `json:"gpuUnmonitored,omitempty" doc:"True when the process holds a GPU that is excluded, or an NVIDIA GPU that is excluded or suspended, so gpu and vramKB are deliberately not measured."`
	DiskRead       *float64 `json:"diskRead,omitempty" doc:"Disk bytes read per second since the cursor."`
	DiskWrite      *float64 `json:"diskWrite,omitempty" doc:"Disk bytes written per second since the cursor."`

	Connections      *int            `json:"connections,omitempty" doc:"Open sockets with an internet remote; merged rows sum their members."`
	LocalConnections *int            `json:"localConnections,omitempty" doc:"Open sockets with a LAN, VPN or link-local remote (private, CGNAT or link-local address); loopback is never counted. Merged rows sum their members."`
	Remotes          []NetConnection `json:"remotes,omitempty" doc:"Internet and local remotes, capped at 32."`
	NetRx            *float64        `json:"netRx,omitempty" doc:"TCP payload bytes per second received from internet and local remotes since the cursor. Absent when unknown: unreadable fds, a container's own network namespace, or only UDP sockets, which have no byte counters."`
	NetTx            *float64        `json:"netTx,omitempty" doc:"TCP payload bytes per second sent and acknowledged, like netRx."`
	NetPartial       bool            `json:"netPartial,omitempty" doc:"True when UDP sockets (e.g. QUIC) to internet or local remotes are open too; netRx and netTx leave them out, so they are a lower bound."`
}

type NetConnection struct {
	Protocol string `json:"protocol" doc:"tcp or udp."`
	Remote   string `json:"remote" doc:"Remote address as ip:port, IPv6 bracketed."`
	Local    bool   `json:"local,omitempty" doc:"True for a LAN, VPN or link-local remote rather than an internet one."`
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

// ProcessDetail holds per-process data that is too costly to poll for every row; it is read on demand for one PID.
type ProcessDetail struct {
	PID        int32   `json:"pid"`
	ReadTotal  *uint64 `json:"readTotal,omitempty" doc:"Bytes read from storage over the process lifetime."`
	WriteTotal *uint64 `json:"writeTotal,omitempty" doc:"Bytes written to storage over the process lifetime."`
	SwapKB     *uint64 `json:"swapKB,omitempty" doc:"Memory swapped out, in KB."`
	OpenFiles  *int    `json:"openFiles,omitempty" doc:"Open file descriptors."`
	Unit       string  `json:"unit,omitempty" doc:"Innermost systemd unit or scope from the cgroup path, e.g. app-firefox@1.scope."`
	Terminal   bool    `json:"terminal,omitempty" doc:"True when the process has a controlling terminal, so it cannot be relaunched detached."`
}
