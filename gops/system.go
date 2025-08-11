package gops

import (
	"fmt"
	"time"

	"github.com/AvengeMedia/dgop/models"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/process"
)

func (self *GopsUtil) GetSystemInfo() (*models.SystemInfo, error) {
	// System info
	loadAvg, _ := load.Avg()
	procs, _ := process.Pids()
	bootTime, _ := host.BootTime()

	// Count threads (approximation - gopsutil doesn't expose this directly)
	threadCount := 0
	for _, p := range procs {
		proc, err := process.NewProcess(p)
		if err == nil {
			threads, _ := proc.NumThreads()
			threadCount += int(threads)
		}
	}

	return &models.SystemInfo{
		LoadAvg:   fmt.Sprintf("%.2f %.2f %.2f", loadAvg.Load1, loadAvg.Load5, loadAvg.Load15),
		Processes: len(procs),
		Threads:   threadCount,
		BootTime:  time.Unix(int64(bootTime), 0).Format("2006-01-02 15:04:05"),
	}, nil
}
