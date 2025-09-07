package gops

import (
	"strings"

	"github.com/AvengeMedia/dgop/models"
	"github.com/shirou/gopsutil/v4/net"
)

func (self *GopsUtil) GetNetworkInfo() ([]*models.NetworkInfo, error) {
	netIO, err := net.IOCounters(true)
	res := make([]*models.NetworkInfo, 0)
	if err == nil {
		for _, n := range netIO {
			// Filter to match bash script (wlan, wlo, wlp, eth, eno, enp, ens, lxc)
			if matchesNetworkInterface(n.Name) {
				res = append(res, &models.NetworkInfo{
					Name: n.Name,
					Rx:   n.BytesRecv,
					Tx:   n.BytesSent,
				})
			}
		}
	}
	return res, nil
}

func matchesNetworkInterface(name string) bool {
	prefixes := []string{"wlan", "wlo", "wlp", "eth", "eno", "enp", "ens", "lxc"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
