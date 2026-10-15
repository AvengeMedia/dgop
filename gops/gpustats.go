package gops

import (
	"strings"

	"github.com/AvengeMedia/dgop/models"
)

// gpuEnabled reports whether a GPU may be monitored. A nil list means no
// filter; a non-nil list (even empty) enables only the GPUs it names, by
// vendor:device ID or PCI slot.
func gpuEnabled(pciIds []string, pciId, pciSlot string) bool {
	if pciIds == nil {
		return true
	}
	for _, id := range pciIds {
		id = strings.TrimSpace(id)
		if id != "" && (strings.EqualFold(id, pciId) || strings.EqualFold(id, pciSlot)) {
			return true
		}
	}
	return false
}

// GetGPUStats returns live stats for the detected GPUs enabled by pciIds (nil
// enables all); the others are listed with identity only and never read. The
// cursor carries DRM client counters for GPUs whose usage comes from fdinfo.
func (self *GopsUtil) GetGPUStats(cursor string, pciIds []string) (*models.GPUStatsInfo, error) {
	detected, err := self.detectGPUs()
	if err != nil {
		return nil, err
	}
	return statsFor(detected, cursor, pciIds, self.fillGPUStats), nil
}

// statsFor lists every detected GPU and fills stats for the enabled ones only.
func statsFor(detected []models.GPU, cursor string, pciIds []string, fill func([]models.GPUStats, string) string) *models.GPUStatsInfo {
	gpus := make([]models.GPUStats, 0, len(detected))
	enabled := make([]bool, 0, len(detected))
	for _, gpu := range detected {
		gpus = append(gpus, models.GPUStats{
			PciSlot: gpu.PciSlot,
			PciId:   gpu.PciId,
			Driver:  gpu.Driver,
			Vendor:  gpu.Vendor,
			Name:    gpu.FullName,
		})
		enabled = append(enabled, gpuEnabled(pciIds, gpu.PciId, gpu.PciSlot))
	}
	return &models.GPUStatsInfo{GPUs: gpus, Cursor: fillEnabled(gpus, enabled, cursor, fill)}
}

// fillEnabled runs fill over the enabled GPUs only and copies the results back.
func fillEnabled(gpus []models.GPUStats, enabled []bool, cursor string, fill func([]models.GPUStats, string) string) string {
	var subset []models.GPUStats
	for i, g := range gpus {
		if enabled[i] {
			subset = append(subset, g)
		}
	}
	if len(subset) == 0 {
		return ""
	}
	next := fill(subset, cursor)
	j := 0
	for i := range gpus {
		if enabled[i] {
			gpus[i] = subset[j]
			j++
		}
	}
	return next
}
