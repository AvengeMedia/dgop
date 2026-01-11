package gops

import (
	"github.com/AvengeMedia/dgop/models"
)

func (self *GopsUtil) GetMemoryInfo() (*models.MemoryInfo, error) {
	v, err := self.memProvider.VirtualMemory()
	if err != nil {
		return nil, err
	}

	total := v.Total / 1024
	free := v.Free / 1024
	buffers := v.Buffers / 1024
	cached := v.Cached / 1024
	sreclaimable := v.Sreclaimable / 1024
	shared := v.Shared / 1024

	// Used = Total - Free - Cached - Buffers + Shared
	usedDiff := free + cached + buffers
	var used uint64
	switch {
	case total >= usedDiff:
		used = total - usedDiff + shared
	default:
		used = total - free
	}

	var usedPercent float64
	if total > 0 {
		usedPercent = float64(used) / float64(total) * 100
	}

	return &models.MemoryInfo{
		Total:        total,
		Used:         used,
		UsedPercent:  usedPercent,
		Free:         free,
		Available:    v.Available / 1024,
		Buffers:      buffers,
		Cached:       cached,
		SReclaimable: sreclaimable,
		Shared:       shared,
		SwapTotal:    v.SwapTotal / 1024,
		SwapFree:     v.SwapFree / 1024,
	}, nil
}
