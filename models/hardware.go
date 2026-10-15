package models

type BIOSInfo struct {
	Vendor      string `json:"vendor"`
	Version     string `json:"version"`
	Date        string `json:"date"`
	Motherboard string `json:"motherboard"`
}

type SystemHardware struct {
	Kernel   string   `json:"kernel"`
	Distro   string   `json:"distro"`
	Hostname string   `json:"hostname"`
	Arch     string   `json:"arch"`
	CPU      CPUBasic `json:"cpu"`
	BIOS     BIOSInfo `json:"bios"`
}

type CPUBasic struct {
	Count int    `json:"count"`
	Model string `json:"model"`
}

type GPU struct {
	Driver      string  `json:"driver"`
	Vendor      string  `json:"vendor"`
	DisplayName string  `json:"displayName"`
	FullName    string  `json:"fullName"`
	PciId       string  `json:"pciId"`
	RawLine     string  `json:"rawLine"`
	Temperature float64 `json:"temperature"`
	Hwmon       string  `json:"hwmon"`
	BusId       string  `json:"busId,omitempty"`

	// Live metrics are only filled for requested GPUs; nil means the driver does
	// not expose the value. Units: percent, KB, watts, MHz.
	State       string   `json:"state,omitempty"`
	Usage       *float64 `json:"usage,omitempty"`
	MemoryUsed  *uint64  `json:"memoryUsed,omitempty"`
	MemoryTotal *uint64  `json:"memoryTotal,omitempty"`
	Power       *float64 `json:"power,omitempty"`
	CoreClock   *float64 `json:"coreClock,omitempty"`
	MemoryClock *float64 `json:"memoryClock,omitempty"`
}

const (
	GPUStateActive    = "active"
	GPUStateSuspended = "suspended"
)

type GPUInfo struct {
	GPUs   []GPU  `json:"gpus"`
	Cursor string `json:"cursor,omitempty"`
}

type GPUTempInfo struct {
	Driver      string  `json:"driver"`
	Hwmon       string  `json:"hwmon"`
	Temperature float64 `json:"temperature"`
}

type GPUTempsInfo struct {
	GPUTemps []GPUTempInfo `json:"gputemps"`
}
