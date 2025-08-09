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
}

type GPUInfo struct {
	GPUs []GPU `json:"gpus"`
}

type GPUTempInfo struct {
	Driver      string  `json:"driver"`
	Hwmon       string  `json:"hwmon"`
	Temperature float64 `json:"temperature"`
}

type GPUTempsInfo struct {
	GPUTemps []GPUTempInfo `json:"gputemps"`
}
