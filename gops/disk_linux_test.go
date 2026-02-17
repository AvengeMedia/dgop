//go:build linux

package gops

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchesDiskDevice(t *testing.T) {
	tests := []struct {
		name     string
		device   string
		expected bool
	}{
		{"sda device", "sda", true},
		{"sdb1 partition", "sdb1", true},
		{"nvme device", "nvme0n1", true},
		{"nvme partition", "nvme0n1p1", true},
		{"virtual device", "vda", true},
		{"device mapper", "dm-0", true},
		{"mmc device", "mmcblk0", true},
		{"loop device", "loop0", false},
		{"ram device", "ram0", false},
		{"zram device", "zram0", false},
		{"empty string", "", false},
		{"random text", "foobar", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesDiskDevice(tt.device)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsVirtualFS(t *testing.T) {
	tests := []struct {
		name     string
		fstype   string
		expected bool
	}{
		{"tmpfs", "tmpfs", true},
		{"devtmpfs", "devtmpfs", true},
		{"sysfs", "sysfs", true},
		{"proc", "proc", true},
		{"cgroup2", "cgroup2", true},
		{"debugfs", "debugfs", true},
		{"binfmt_misc", "binfmt_misc", true},
		{"fuse.gvfsd-fuse", "fuse.gvfsd-fuse", true},
		{"fuse.portal", "fuse.portal", true},
		{"ext4", "ext4", false},
		{"fuse.sshfs", "fuse.sshfs", false},
		{"btrfs", "btrfs", false},
		{"xfs", "xfs", false},
		{"zfs", "zfs", false},
		{"ntfs", "ntfs", false},
		{"vfat", "vfat", false},
		{"overlay", "overlay", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isVirtualFS(tt.fstype)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsVirtualMount(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"proc submount", "/proc/sys/fs/binfmt_misc", true},
		{"sys submount", "/sys/firmware/efi/efivars", true},
		{"dev submount", "/dev/hugepages", true},
		{"root", "/", false},
		{"boot", "/boot", false},
		{"home", "/home", false},
		{"zfs mount", "/mnt/data", false},
		{"run media", "/run/media/user/usb", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isVirtualMount(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}
