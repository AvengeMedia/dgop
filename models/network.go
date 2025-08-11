package models

type NetworkInfo struct {
	Name string `json:"name"`
	Rx   uint64 `json:"rx"`
	Tx   uint64 `json:"tx"`
}

type NetworkRateInfo struct {
	Interface string  `json:"interface"`
	RxRate    float64 `json:"rxrate"`
	TxRate    float64 `json:"txrate"`
	RxTotal   uint64  `json:"rxtotal"`
	TxTotal   uint64  `json:"txtotal"`
}

type NetworkRateResponse struct {
	Interfaces []*NetworkRateInfo `json:"interfaces"`
	Cursor     string             `json:"cursor"`
}
