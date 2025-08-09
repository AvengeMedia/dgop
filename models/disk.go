package models

type DiskInfo struct {
	Name  string `json:"name"`
	Read  uint64 `json:"read"`
	Write uint64 `json:"write"`
}

type DiskMountInfo struct {
	Device  string `json:"device"`
	Mount   string `json:"mount"`
	FSType  string `json:"fstype"`
	Size    string `json:"size"`
	Used    string `json:"used"`
	Avail   string `json:"avail"`
	Percent string `json:"percent"`
}
