package gops

import (
	"github.com/bbedward/DankMaterialShell/dankgop/internal/log"
	"github.com/bbedward/DankMaterialShell/dankgop/models"
)

type GopsUtil struct{}

func NewGopsUtil() *GopsUtil {
	return &GopsUtil{}
}

func (self *GopsUtil) GetAllMetrics(procSortBy ProcSortBy, procLimit int, enableProcessCPU bool) (*models.SystemMetrics, error) {
	return self.GetAllMetricsWithSample(procSortBy, procLimit, enableProcessCPU, nil, nil)
}

func (self *GopsUtil) GetAllMetricsWithSample(procSortBy ProcSortBy, procLimit int, enableProcessCPU bool, cpuSample *models.CPUSampleData, procSample []models.ProcessSampleData) (*models.SystemMetrics, error) {
	cpuInfo, err := self.GetCPUInfoWithSample(cpuSample)
	if err != nil {
		log.Errorf("Failed to get CPU info: %v", err)
	}

	memInfo, err := self.GetMemoryInfo()
	if err != nil {
		log.Errorf("Failed to get memory info: %v", err)
	}

	networkInfo, err := self.GetNetworkInfo()
	if err != nil {
		log.Errorf("Failed to get network info: %v", err)
	}

	diskInfo, err := self.GetDiskInfo()
	if err != nil {
		log.Errorf("Failed to get disk info: %v", err)
	}

	diskMounts, err := self.GetDiskMounts()
	if err != nil {
		log.Errorf("Failed to get disk mounts: %v", err)
	}

	processes, err := self.GetProcessesWithSample(procSortBy, procLimit, enableProcessCPU, procSample)
	if err != nil {
		log.Errorf("Failed to get processes: %v", err)
	}

	systemInfo, err := self.GetSystemInfo()
	if err != nil {
		log.Errorf("Failed to get system info: %v", err)
	}

	return &models.SystemMetrics{
		Memory:     memInfo,
		CPU:        cpuInfo,
		Network:    networkInfo,
		Disk:       diskInfo,
		Processes:  processes,
		System:     systemInfo,
		DiskMounts: diskMounts,
	}, nil
}
