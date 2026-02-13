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

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    uint64
		expected string
	}{
		{"zero bytes", 0, "0B"},
		{"single byte", 1, "1B"},
		{"bytes only", 512, "512B"},
		{"exact KB", 1024, "1.0K"},
		{"kilobytes", 1536, "1.5K"},
		{"exact MB", 1048576, "1.0M"},
		{"megabytes", 5242880, "5.0M"},
		{"fractional MB", 1572864, "1.5M"},
		{"exact GB", 1073741824, "1.0G"},
		{"gigabytes", 5368709120, "5.0G"},
		{"exact TB", 1099511627776, "1.0T"},
		{"terabytes", 2199023255552, "2.0T"},
		{"exact PB", 1125899906842624, "1.0P"},
		{"petabytes", 2251799813685248, "2.0P"},
		{"exact EB", 1152921504606846976, "1.0E"},
		{"large value", 18446744073709551615, "16.0E"}, // Max uint64
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatBytesRounding(t *testing.T) {
	tests := []struct {
		bytes    uint64
		contains string
	}{
		{1024 * 1024 * 100, "100.0M"},
		{1024 * 1024 * 1024 * 5, "5.0G"},
		{1024 * 1024 * 500, "500.0M"},
		{1024 * 1024 * 1024 * 10, "10.0G"},
	}

	for _, tt := range tests {
		t.Run(tt.contains, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			assert.Equal(t, tt.contains, result)
		})
	}
}
