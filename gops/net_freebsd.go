//go:build freebsd

package gops

import "strings"

func matchesNetworkInterface(name string) bool {
	prefixes := []string{"em", "igb", "igc", "ix", "re", "bge", "bce",
		"alc", "fxp", "msk", "vtnet", "wlan", "ue", "genet", "lagg"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
