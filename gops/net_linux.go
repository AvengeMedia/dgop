//go:build linux

package gops

import "strings"

func matchesNetworkInterface(name string) bool {
	prefixes := []string{"wlan", "wlo", "wlp", "wlx", "wl", "eth", "eno", "enp", "ens", "lxc", "wld", "enx", "usb", "rndis", "wwan", "rmnet"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
