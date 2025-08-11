package gops

import (
	"github.com/AvengeMedia/dgop/internal/log"
	"github.com/AvengeMedia/dgop/models"
	"github.com/shirou/gopsutil/v4/sensors"
)

type GopsUtil struct{}

func NewGopsUtil() *GopsUtil {
	return &GopsUtil{}
}

func (self *GopsUtil) GetAllMetrics(procSortBy ProcSortBy, procLimit int, enableProcessCPU bool) (*models.SystemMetrics, error) {
	return self.GetAllMetricsWithCursors(procSortBy, procLimit, enableProcessCPU, "", "")
}

func (self *GopsUtil) GetAllMetricsWithCursors(procSortBy ProcSortBy, procLimit int, enableProcessCPU bool, cpuCursor string, procCursor string) (*models.SystemMetrics, error) {
	cpuInfo, err := self.GetCPUInfoWithCursor(cpuCursor)
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

	processResult, err := self.GetProcessesWithCursor(procSortBy, procLimit, enableProcessCPU, procCursor)
	if err != nil {
		log.Errorf("Failed to get processes: %v", err)
	}

	systemInfo, err := self.GetSystemInfo()
	if err != nil {
		log.Errorf("Failed to get system info: %v", err)
	}

	var processes []*models.ProcessInfo
	if processResult != nil {
		processes = processResult.Processes
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

// GetSystemTemperatures returns system temperature sensors
func (self *GopsUtil) GetSystemTemperatures() ([]models.TemperatureSensor, error) {
	temps, err := sensors.SensorsTemperatures()
	if err != nil {
		return nil, err
	}

	var sensors []models.TemperatureSensor
	for _, temp := range temps {
		sensor := models.TemperatureSensor{
			Name:        temp.SensorKey,
			Temperature: temp.Temperature,
			High:        temp.High,
			Critical:    temp.Critical,
		}
		sensors = append(sensors, sensor)
	}

	return sensors, nil
}
