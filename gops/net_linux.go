//go:build linux

package gops

import "strings"

func matchesNetworkInterface(name string) bool {
	prefixes := []string{"wlan", "wlo", "wlp", "eth", "eno", "enp", "ens", "lxc"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
