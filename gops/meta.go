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

func (self *GopsUtil) loadAllModules(params MetaParams) (*models.MetaInfo, error) {
	meta := &models.MetaInfo{}

	type result struct {
		name string
		data interface{}
		err  error
	}

	ch := make(chan result, 12)

	go func() {
		cpu, err := self.GetCPUInfoWithCursor(params.CPUCursor)
		ch <- result{"cpu", cpu, err}
	}()

	go func() {
		mem, err := self.GetMemoryInfo()
		ch <- result{"memory", mem, err}
	}()

	go func() {
		net, err := self.GetNetworkInfo()
		ch <- result{"network", net, err}
	}()

	go func() {
		netRate, err := self.GetNetworkRates(params.NetRateCursor)
		ch <- result{"netrate", netRate, err}
	}()

	go func() {
		disk, err := self.GetDiskInfo()
		ch <- result{"disk", disk, err}
	}()

	go func() {
		diskRate, err := self.GetDiskRates(params.DiskRateCursor)
		ch <- result{"diskrate", diskRate, err}
	}()

	go func() {
		mounts, err := self.GetDiskMounts()
		ch <- result{"mounts", mounts, err}
	}()

	go func() {
		procs, err := self.GetProcessesWithCursor(params.SortBy, params.ProcLimit, params.EnableCPU, params.ProcCursor)
		ch <- result{"processes", procs, err}
	}()

	go func() {
		sys, err := self.GetSystemInfo()
		ch <- result{"system", sys, err}
	}()

	go func() {
		hw, err := self.GetSystemHardware()
		ch <- result{"hardware", hw, err}
	}()

	go func() {
		gpu, err := self.GetGPUInfoWithTemp(params.GPUPciIds)
		ch <- result{"gpu", gpu, err}
	}()

	for i := 0; i < 11; i++ {
		r := <-ch
		if r.err != nil {
			continue
		}
		switch r.name {
		case "cpu":
			meta.CPU = r.data.(*models.CPUInfo)
		case "memory":
			meta.Memory = r.data.(*models.MemoryInfo)
		case "network":
			meta.Network = r.data.([]*models.NetworkInfo)
		case "netrate":
			meta.NetRate = r.data.(*models.NetworkRateResponse)
		case "disk":
			meta.Disk = r.data.([]*models.DiskInfo)
		case "diskrate":
			meta.DiskRate = r.data.(*models.DiskRateResponse)
		case "mounts":
			meta.DiskMounts = r.data.([]*models.DiskMountInfo)
		case "processes":
			procResult := r.data.(*models.ProcessListResponse)
			meta.Processes = procResult.Processes
			meta.Cursor = procResult.Cursor
		case "system":
			meta.System = r.data.(*models.SystemInfo)
		case "hardware":
			meta.Hardware = r.data.(*models.SystemHardware)
		case "gpu":
			meta.GPU = r.data.(*models.GPUInfo)
		}
	}

	return meta, nil
}
