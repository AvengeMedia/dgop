package models

type GPUStats struct {
	PciSlot   string `json:"pciSlot"`
	PciId     string `json:"pciId"`
	Driver    string `json:"driver"`
	Vendor    string `json:"vendor"`
	Name      string `json:"name"`
	Suspended bool   `json:"suspended,omitempty"`

	Usage        *float64 `json:"usage,omitempty" doc:"Busy percentage. Without a driver busy counter (e.g. Intel) it is measured from the DRM clients of processes dgop can read."`
	VRAMUsedKB   *uint64  `json:"vramUsedKB,omitempty"`
	VRAMTotalKB  *uint64  `json:"vramTotalKB,omitempty"`
	GTTUsedKB    *uint64  `json:"gttUsedKB,omitempty" doc:"System memory mapped for the GPU"`
	GTTTotalKB   *uint64  `json:"gttTotalKB,omitempty"`
	MemoryBusy   *float64 `json:"memoryBusy,omitempty" doc:"Memory controller busy percentage"`
	EncoderUsage *float64 `json:"encoderUsage,omitempty" doc:"Video encoder busy percentage"`
	DecoderUsage *float64 `json:"decoderUsage,omitempty" doc:"Video decoder busy percentage"`
	CoreClock    *float64 `json:"coreClock,omitempty" doc:"MHz"`
	CoreClockMax *float64 `json:"coreClockMax,omitempty" doc:"MHz"`
	MemClock     *float64 `json:"memClock,omitempty" doc:"MHz"`
	MemClockMax  *float64 `json:"memClockMax,omitempty" doc:"MHz"`
	Temperature  *float64 `json:"temperature,omitempty" doc:"Celsius"`
	Hotspot      *float64 `json:"hotspot,omitempty" doc:"Celsius"`
	MemTemp      *float64 `json:"memTemperature,omitempty" doc:"Celsius"`
	Power        *float64 `json:"power,omitempty" doc:"Watts"`
	PowerLimit   *float64 `json:"powerLimit,omitempty" doc:"Watts"`
	FanRpm       *int     `json:"fanRpm,omitempty"`
	FanMaxRpm    *int     `json:"fanMaxRpm,omitempty"`
	FanPercent   *float64 `json:"fanPercent,omitempty"`

	PcieLinkSpeed    string `json:"pcieLinkSpeed,omitempty" doc:"Kernel string, e.g. 16.0 GT/s PCIe"`
	PcieLinkSpeedMax string `json:"pcieLinkSpeedMax,omitempty"`
	PcieLinkWidth    int    `json:"pcieLinkWidth,omitempty"`
	PcieLinkWidthMax int    `json:"pcieLinkWidthMax,omitempty"`
}

type GPUStatsInfo struct {
	GPUs   []GPUStats `json:"gpus"`
	Cursor string     `json:"cursor,omitempty"`
}
