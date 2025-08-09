package models

type MemoryInfo struct {
	Total     uint64 `json:"total"`
	Free      uint64 `json:"free"`
	Available uint64 `json:"available"`
	Buffers   uint64 `json:"buffers"`
	Cached    uint64 `json:"cached"`
	Shared    uint64 `json:"shared"`
	SwapTotal uint64 `json:"swaptotal"`
	SwapFree  uint64 `json:"swapfree"`
}
