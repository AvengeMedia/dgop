package models

type MemoryInfo struct {
	Total        uint64  `json:"total"`
	Used         uint64  `json:"used"`
	UsedPercent  float64 `json:"usedPercent"`
	Free         uint64  `json:"free"`
	Available    uint64  `json:"available"`
	Buffers      uint64  `json:"buffers"`
	Cached       uint64  `json:"cached"`
	SReclaimable uint64  `json:"sreclaimable"`
	Shared       uint64  `json:"shared"`
	SwapTotal    uint64  `json:"swaptotal"`
	SwapFree     uint64  `json:"swapfree"`
}
