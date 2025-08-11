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

type DiskRateInfo struct {
	Device     string  `json:"device"`
	ReadRate   float64 `json:"readrate"`
	WriteRate  float64 `json:"writerate"`
	ReadTotal  uint64  `json:"readtotal"`
	WriteTotal uint64  `json:"writetotal"`
	ReadCount  uint64  `json:"readcount"`
	WriteCount uint64  `json:"writecount"`
}

type DiskRateResponse struct {
	Disks  []*DiskRateInfo `json:"disks"`
	Cursor string          `json:"cursor"`
}
