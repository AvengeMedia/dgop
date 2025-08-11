package gops

import (
	"fmt"
	"strings"

	"github.com/AvengeMedia/dgop/models"
	"github.com/shirou/gopsutil/v4/disk"
)

func (self *GopsUtil) GetDiskInfo() ([]*models.DiskInfo, error) {
	diskIO, err := disk.IOCounters()
	res := make([]*models.DiskInfo, 0)
	if err == nil {
		for name, d := range diskIO {
			// Filter to match bash script patterns
			if matchesDiskDevice(name) {
				res = append(res, &models.DiskInfo{
					Name:  name,
					Read:  d.ReadBytes / 512,  // Convert to sectors
					Write: d.WriteBytes / 512, // Convert to sectors
				})
			}
		}
	}
	return res, nil
}

func (self *GopsUtil) GetDiskMounts() ([]*models.DiskMountInfo, error) {
	partitions, err := disk.Partitions(false)
	var metrics []*models.DiskMountInfo
	if err == nil {
		for _, p := range partitions {
			// Skip tmpfs and devtmpfs
			if p.Fstype == "tmpfs" || p.Fstype == "devtmpfs" {
				continue
			}

			usage, err := disk.Usage(p.Mountpoint)
			if err != nil {
				continue
			}

			metrics = append(metrics, &models.DiskMountInfo{
				Device:  p.Device,
				Mount:   p.Mountpoint,
				FSType:  p.Fstype,
				Size:    formatBytes(usage.Total),
				Used:    formatBytes(usage.Used),
				Avail:   formatBytes(usage.Free),
				Percent: fmt.Sprintf("%.0f%%", usage.UsedPercent),
			})
		}
	}

	return metrics, nil
}

func matchesDiskDevice(name string) bool {
	patterns := []string{"sd", "nvme", "vd", "dm-", "mmcblk"}
	for _, pattern := range patterns {
		if strings.HasPrefix(name, pattern) {
			return true
		}
	}
	return false
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%c", float64(bytes)/float64(div), "KMGTPE"[exp])
}
