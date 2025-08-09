package gops

import (
	"fmt"
	"strings"

	"github.com/bbedward/DankMaterialShell/dankgop/models"
)

var availableModules = []string{
	"cpu",
	"memory",
	"network",
	"disk",
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
	CPUSampleData  *models.CPUSampleData
	ProcSampleData []models.ProcessSampleData
}

func (self *GopsUtil) GetMeta(modules []string, params MetaParams) (*models.MetaInfo, error) {
	meta := &models.MetaInfo{}

	for _, module := range modules {
		switch strings.ToLower(module) {
		case "all":
			// Load all modules
			return self.loadAllModules(params)
		case "cpu":
			if cpu, err := self.GetCPUInfoWithSample(params.CPUSampleData); err == nil {
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
		case "disk":
			if disk, err := self.GetDiskInfo(); err == nil {
				meta.Disk = disk
			}
		case "diskmounts":
			if mounts, err := self.GetDiskMounts(); err == nil {
				meta.DiskMounts = mounts
			}
		case "processes":
			if procs, err := self.GetProcessesWithSample(params.SortBy, params.ProcLimit, params.EnableCPU, params.ProcSampleData); err == nil {
				meta.Processes = procs
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
	if cpu, err := self.GetCPUInfoWithSample(params.CPUSampleData); err == nil {
		meta.CPU = cpu
	}

	if mem, err := self.GetMemoryInfo(); err == nil {
		meta.Memory = mem
	}

	if net, err := self.GetNetworkInfo(); err == nil {
		meta.Network = net
	}

	if disk, err := self.GetDiskInfo(); err == nil {
		meta.Disk = disk
	}

	if mounts, err := self.GetDiskMounts(); err == nil {
		meta.DiskMounts = mounts
	}

	if procs, err := self.GetProcessesWithSample(params.SortBy, params.ProcLimit, params.EnableCPU, params.ProcSampleData); err == nil {
		meta.Processes = procs
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
