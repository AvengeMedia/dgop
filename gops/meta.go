package gops

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/AvengeMedia/dgop/models"
	"golang.org/x/sync/errgroup"
)

var availableModules = []string{
	"cpu",
	"memory",
	"network",
	"net-rate",
	"disk",
	"disk-rate",
	"diskmounts",
	"processes",
	"system",
	"hardware",
	"gpu",
	"gpu-temp",
}

func (self *GopsUtil) GetModules() (*models.ModulesInfo, error) {
	return &models.ModulesInfo{
		Available: availableModules,
	}, nil
}

type MetaParams struct {
	SortBy         ProcSortBy
	ProcLimit      int
	EnableCPU      bool
	GPUPciIds      []string
	CPUCursor      string
	ProcCursor     string
	NetRateCursor  string
	DiskRateCursor string
}

func (self *GopsUtil) GetMeta(ctx context.Context, modules []string, params MetaParams) (*models.MetaInfo, error) {
	meta := &models.MetaInfo{}

	for _, module := range modules {
		switch strings.ToLower(module) {
		case "all":
			// Load all modules
			return self.loadAllModules(ctx, params)
		case "cpu":
			if cpu, err := self.GetCPUInfoWithCursor(params.CPUCursor); err == nil {
				meta.CPU = cpu
			}
		case "memory":
			if mem, err := self.GetMemoryInfo(); err == nil {
				meta.Memory = mem
			}
		case "network":
			if net, err := self.GetNetworkInfo(); err == nil {
				meta.Network = net
			}
		case "net-rate":
			if netRate, err := self.GetNetworkRates(params.NetRateCursor); err == nil {
				meta.NetRate = netRate
			}
		case "disk":
			if disk, err := self.GetDiskInfo(); err == nil {
				meta.Disk = disk
			}
		case "disk-rate":
			if diskRate, err := self.GetDiskRates(params.DiskRateCursor); err == nil {
				meta.DiskRate = diskRate
			}
		case "diskmounts":
			if mounts, err := self.GetDiskMounts(); err == nil {
				meta.DiskMounts = mounts
			}
		case "processes":
			if result, err := self.GetProcessesWithCursor(params.SortBy, params.ProcLimit, params.EnableCPU, params.ProcCursor); err == nil {
				meta.Processes = result.Processes
				meta.Cursor = result.Cursor
			}
		case "system":
			if sys, err := self.GetSystemInfo(); err == nil {
				meta.System = sys
			}
		case "hardware":
			if hw, err := self.GetSystemHardware(); err == nil {
				meta.Hardware = hw
			}
		case "gpu":
			// GPU module with optional temperature
			if gpu, err := self.GetGPUInfoWithTemp(params.GPUPciIds); err == nil {
				meta.GPU = gpu
			}
		case "gpu-temp":
			// GPU temperature only module
			if gpu, err := self.GetGPUInfoWithTemp(params.GPUPciIds); err == nil {
				meta.GPU = gpu
			}
		default:
			return nil, fmt.Errorf("unknown module: %s", module)
		}
	}

	return meta, nil
}

func (self *GopsUtil) loadAllModules(ctx context.Context, params MetaParams) (*models.MetaInfo, error) {
	meta := &models.MetaInfo{}
	var mu sync.Mutex

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		cpu, err := self.GetCPUInfoWithCursor(params.CPUCursor)
		if err != nil {
			return nil
		}
		mu.Lock()
		meta.CPU = cpu
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		mem, err := self.GetMemoryInfo()
		if err != nil {
			return nil
		}
		mu.Lock()
		meta.Memory = mem
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		net, err := self.GetNetworkInfo()
		if err != nil {
			return nil
		}
		mu.Lock()
		meta.Network = net
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		netRate, err := self.GetNetworkRates(params.NetRateCursor)
		if err != nil {
			return nil
		}
		mu.Lock()
		meta.NetRate = netRate
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		disk, err := self.GetDiskInfo()
		if err != nil {
			return nil
		}
		mu.Lock()
		meta.Disk = disk
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		diskRate, err := self.GetDiskRates(params.DiskRateCursor)
		if err != nil {
			return nil
		}
		mu.Lock()
		meta.DiskRate = diskRate
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		mounts, err := self.GetDiskMounts()
		if err != nil {
			return nil
		}
		mu.Lock()
		meta.DiskMounts = mounts
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		procs, err := self.GetProcessesWithCursor(params.SortBy, params.ProcLimit, params.EnableCPU, params.ProcCursor)
		if err != nil {
			return nil
		}
		mu.Lock()
		meta.Processes = procs.Processes
		meta.Cursor = procs.Cursor
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		sys, err := self.GetSystemInfo()
		if err != nil {
			return nil
		}
		mu.Lock()
		meta.System = sys
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		hw, err := self.GetSystemHardware()
		if err != nil {
			return nil
		}
		mu.Lock()
		meta.Hardware = hw
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		gpu, err := self.GetGPUInfoWithTemp(params.GPUPciIds)
		if err != nil {
			return nil
		}
		mu.Lock()
		meta.GPU = gpu
		mu.Unlock()
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return meta, nil
}
