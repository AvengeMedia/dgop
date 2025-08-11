package gops

import (
	"fmt"
	"strings"

	"github.com/AvengeMedia/dgop/models"
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

func (self *GopsUtil) GetMeta(modules []string, params MetaParams) (*models.MetaInfo, error) {
	meta := &models.MetaInfo{}

	for _, module := range modules {
		switch strings.ToLower(module) {
		case "all":
			// Load all modules
			return self.loadAllModules(params)
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

func (self *GopsUtil) loadAllModules(params MetaParams) (*models.MetaInfo, error) {
	meta := &models.MetaInfo{}

	// Load all modules (ignore errors for individual modules)
	if cpu, err := self.GetCPUInfoWithCursor(params.CPUCursor); err == nil {
		meta.CPU = cpu
	}

	if mem, err := self.GetMemoryInfo(); err == nil {
		meta.Memory = mem
	}

	if net, err := self.GetNetworkInfo(); err == nil {
		meta.Network = net
	}

	if netRate, err := self.GetNetworkRates(params.NetRateCursor); err == nil {
		meta.NetRate = netRate
	}

	if disk, err := self.GetDiskInfo(); err == nil {
		meta.Disk = disk
	}

	if diskRate, err := self.GetDiskRates(params.DiskRateCursor); err == nil {
		meta.DiskRate = diskRate
	}

	if mounts, err := self.GetDiskMounts(); err == nil {
		meta.DiskMounts = mounts
	}

	if result, err := self.GetProcessesWithCursor(params.SortBy, params.ProcLimit, params.EnableCPU, params.ProcCursor); err == nil {
		meta.Processes = result.Processes
	}

	if sys, err := self.GetSystemInfo(); err == nil {
		meta.System = sys
	}

	if hw, err := self.GetSystemHardware(); err == nil {
		meta.Hardware = hw
	}

	if gpu, err := self.GetGPUInfoWithTemp(params.GPUPciIds); err == nil {
		meta.GPU = gpu
	}

	return meta, nil
}
