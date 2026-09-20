package gops

import (
	"fmt"
	"time"

	"github.com/AvengeMedia/dgop/models"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
)

func (self *GopsUtil) GetSystemInfo() (*models.SystemInfo, error) {
	loadAvg, _ := load.Avg()
	bootTime, _ := host.BootTime()
	processes, threads := taskCounts()

	return &models.SystemInfo{
		LoadAvg:   fmt.Sprintf("%.2f %.2f %.2f", loadAvg.Load1, loadAvg.Load5, loadAvg.Load15),
		Processes: processes,
		Threads:   threads,
		BootTime:  time.Unix(int64(bootTime), 0).Format("2006-01-02 15:04:05"),
	}, nil
}
