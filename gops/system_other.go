//go:build !linux

package gops

import "github.com/shirou/gopsutil/v4/process"

func taskCounts() (processes, threads int) {
	pids, _ := process.Pids()
	for _, pid := range pids {
		proc, err := process.NewProcess(pid)
		if err != nil {
			continue
		}
		count, _ := proc.NumThreads()
		threads += int(count)
	}
	return len(pids), threads
}
