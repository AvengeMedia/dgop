//go:build linux

package gops

import (
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/AvengeMedia/dgop/models"
)

const netSysRoot = "/sys/class/net"

// ARPHRD types of IP tunnels (tun/WireGuard report none, 65534).
var tunnelARPTypes = map[string]bool{"65534": true, "768": true, "769": true, "776": true, "778": true, "823": true}

func fillLinkInfo(info *models.NetworkRateInfo) {
	readLinkSysfs(netSysRoot, info)
	iface, err := net.InterfaceByName(info.Interface)
	if err != nil {
		return
	}
	addrs, _ := iface.Addrs()
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		if ipnet.IP.To4() != nil {
			info.IPv4 = append(info.IPv4, ipnet.String())
		} else {
			info.IPv6 = append(info.IPv6, ipnet.String())
		}
	}
}

func readLinkSysfs(root string, info *models.NetworkRateInfo) {
	dir := filepath.Join(root, info.Interface)
	info.MTU, _ = strconv.Atoi(readTrimmed(filepath.Join(dir, "mtu")))
	if speed, err := strconv.Atoi(readTrimmed(filepath.Join(dir, "speed"))); err == nil && speed > 0 {
		info.Speed = speed
	}
	if duplex := readTrimmed(filepath.Join(dir, "duplex")); duplex == "full" || duplex == "half" {
		info.Duplex = duplex
	}
	if driver, err := os.Readlink(filepath.Join(dir, "device", "driver")); err == nil {
		info.Driver = filepath.Base(driver)
	}

	arpType := readTrimmed(filepath.Join(dir, "type"))
	switch {
	case arpType == "772":
		info.LinkType = "loopback"
	case exists(filepath.Join(dir, "wireless")) || exists(filepath.Join(dir, "phy80211")):
		info.LinkType = "wireless"
	case tunnelARPTypes[arpType] || exists(filepath.Join(dir, "tun_flags")):
		info.LinkType = "tunnel"
	case arpType == "1" && exists(filepath.Join(dir, "device")):
		info.LinkType = "ethernet"
	case arpType != "":
		info.LinkType = "other"
	}
	// Virtual links report a nominal speed and duplex.
	if info.LinkType != "ethernet" && info.LinkType != "wireless" {
		info.Speed, info.Duplex = 0, ""
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
