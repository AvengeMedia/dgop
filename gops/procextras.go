package gops

// procGPU is one process's GPU readings. Readable false means everything is
// unknown. UsageKnown false means a percentage can't be derived (a driver
// without engine data, NVIDIA, or a GPU we don't classify), so only VRAM is as read.
type procGPU struct {
	Readable bool
	// Unmonitored: unknown because the process holds a GPU that is not queried.
	Unmonitored bool
	UsageKnown  bool
	VRAMBytes   uint64
	GTTBytes    uint64
	// Clients that report engines, for rating.
	Clients drmClientSet
}

type gpuCursorClient struct {
	PDev string `json:"p"`
	ID   uint64 `json:"i"`
	// Engine name -> busy ns, cycles, total cycles.
	Engines map[string][3]uint64 `json:"e"`
}

type procRemote struct {
	Protocol string
	Remote   string
}

type tcpCounters struct {
	Inode   uint32
	RxBytes uint64
	TxBytes uint64
}

type procNet struct {
	// Known is false when the process's fds or the socket dump were unreadable.
	Known bool
	// Count is the exact number of sockets with an internet remote.
	Count   int
	Remotes []procRemote
	// TCP lists the internet TCP sockets only.
	TCP []tcpCounters
	// RatesUnknown is set when a TCP socket came without byte counters, so
	// summing the rest would under-report.
	RatesUnknown bool
}
