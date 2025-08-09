package models

type NetworkInfo struct {
	Name string `json:"name"`
	Rx   uint64 `json:"rx"`
	Tx   uint64 `json:"tx"`
}
