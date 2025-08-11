package gops

import (
	"github.com/AvengeMedia/dgop/models"
	"github.com/shirou/gopsutil/v4/mem"
)

func (self *GopsUtil) GetMemoryInfo() (*models.MemoryInfo, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	return &models.MemoryInfo{
		Total:     v.Total / 1024,
		Free:      v.Free / 1024,
		Available: v.Available / 1024,
		Buffers:   v.Buffers / 1024,
		Cached:    v.Cached / 1024,
		Shared:    v.Shared / 1024,
		SwapTotal: v.SwapTotal / 1024,
		SwapFree:  v.SwapFree / 1024,
	}, nil
}
