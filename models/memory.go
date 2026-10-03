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
	ZfsArcSize   uint64  `json:"zfsArcSize"`
	GPUActive    uint64  `json:"gpuActive"`
	GPUReclaim   uint64  `json:"gpuReclaim"`
	SwapTotal    uint64  `json:"swaptotal"`
	SwapFree     uint64  `json:"swapfree"`
	ZramOrig     uint64  `json:"zramOrig,omitempty" doc:"Uncompressed data stored in zram, in KB"`
	ZramCompr    uint64  `json:"zramCompr,omitempty" doc:"Compressed size of zram data, in KB"`
}

type MemoryModule struct {
	Locator         string `json:"locator"`
	Size            uint64 `json:"size" doc:"Bytes"`
	Type            string `json:"type,omitempty"`
	Speed           uint32 `json:"speed,omitempty" doc:"Rated speed in MT/s"`
	ConfiguredSpeed uint32 `json:"configuredSpeed,omitempty" doc:"Configured speed in MT/s"`
	Manufacturer    string `json:"manufacturer,omitempty"`
	PartNumber      string `json:"partNumber,omitempty"`
	FormFactor      string `json:"formFactor,omitempty"`
}

type MemoryModulesInfo struct {
	Slots   int            `json:"slots"`
	Modules []MemoryModule `json:"modules"`
}
