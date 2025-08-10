package gops

import (
	"reflect"
	"runtime"
	"sort"
	"time"

	"github.com/bbedward/DankMaterialShell/dankgop/models"
	"github.com/danielgtaylor/huma/v2"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
)

func (self *GopsUtil) GetProcesses(sortBy ProcSortBy, limit int, enableCPU bool) ([]*models.ProcessInfo, error) {
	return self.GetProcessesWithSample(sortBy, limit, enableCPU, nil)
}

func (self *GopsUtil) GetProcessesWithSample(sortBy ProcSortBy, limit int, enableCPU bool, sampleData []models.ProcessSampleData) ([]*models.ProcessInfo, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, err
	}

	procList := make([]*models.ProcessInfo, 0)
	totalMem, _ := mem.VirtualMemory()
	currentTime := time.Now().UnixMilli()
	
	// Create sample data map for quick lookup
	sampleMap := make(map[int32]*models.ProcessSampleData)
	if sampleData != nil {
		for i := range sampleData {
			sampleMap[sampleData[i].PID] = &sampleData[i]
		}
	}

	// CPU measurement setup - only if enabled and no sample data provided
	if enableCPU && len(sampleMap) == 0 {
		// First pass: Initialize CPU measurement for all processes
		for _, p := range procs {
			p.CPUPercent() // Initialize
		}

		// Wait for measurement period (1 second for more accurate readings)
		time.Sleep(1000 * time.Millisecond)
	}

	for _, p := range procs {
		// Get process info
		name, _ := p.Name()
		cmdline, _ := p.Cmdline()
		ppid, _ := p.Ppid()
		memInfo, _ := p.MemoryInfo()
		times, _ := p.Times()

		// Calculate current CPU time in seconds (gopsutil already converts to seconds)
		currentCPUTime := float64(0)
		if times != nil {
			currentCPUTime = times.User + times.System
		}

		// Get CPU percentage only if enabled
		cpuPercent := 0.0
		if enableCPU {
			if sample, hasSample := sampleMap[p.Pid]; hasSample {
				// Use sample data to calculate CPU percentage per core
				cpuPercent = calculateProcessCPUPercentageWithSample(sample, currentCPUTime, currentTime)
			} else {
				// Fallback to gopsutil measurement (normalize to per-CPU like htop)
				rawCpuPercent, _ := p.CPUPercent()
				cpuPercent = rawCpuPercent / float64(runtime.NumCPU())
			}
		}

		// Calculate memory percentage and KB
		memKB := uint64(0)
		memPercent := float32(0)
		if memInfo != nil {
			memKB = memInfo.RSS / 1024
			memPercent = float32(memInfo.RSS) / float32(totalMem.Total) * 100
		}

		procList = append(procList, &models.ProcessInfo{
			PID:           p.Pid,
			PPID:          ppid,
			CPU:           cpuPercent,
			PTicks:        uint64(currentCPUTime * 100), // Convert seconds back to ticks for compatibility
			MemoryPercent: memPercent,
			MemoryKB:      memKB,
			PSSKB:         memKB,
			PSSPercent:    memPercent,
			Command:       name,
			FullCommand:   cmdline,
		})
	}

	// Sort processes
	switch sortBy {
	case SortByCPU:
		sort.Slice(procList, func(i, j int) bool {
			return procList[i].CPU > procList[j].CPU
		})
	case SortByMemory:
		sort.Slice(procList, func(i, j int) bool {
			return procList[i].MemoryPercent > procList[j].MemoryPercent
		})
	case SortByName:
		sort.Slice(procList, func(i, j int) bool {
			return procList[i].Command < procList[j].Command
		})
	case SortByPID:
		sort.Slice(procList, func(i, j int) bool {
			return procList[i].PID < procList[j].PID
		})
	default:
		sort.Slice(procList, func(i, j int) bool {
			return procList[i].CPU > procList[j].CPU
		})
	}

	// Limit to MaxProcs
	if limit > 0 && len(procList) > limit {
		procList = procList[:limit]
	}

	return procList, nil
}

type ProcSortBy string

const (
	SortByCPU    ProcSortBy = "cpu"
	SortByMemory ProcSortBy = "memory"
	SortByName   ProcSortBy = "name"
	SortByPID    ProcSortBy = "pid"
)

// Register enum in OpenAPI specification
// https://github.com/danielgtaylor/huma/issues/621
func (u ProcSortBy) Schema(r huma.Registry) *huma.Schema {
	if r.Map()["ProcSortBy"] == nil {
		schemaRef := r.Schema(reflect.TypeOf(""), true, "ProcSortBy")
		schemaRef.Title = "ProcSortBy"
		schemaRef.Enum = append(schemaRef.Enum, []any{
			string(SortByCPU),
			string(SortByMemory),
			string(SortByName),
			string(SortByPID),
		}...)
		r.Map()["ProcSortBy"] = schemaRef
	}
	return &huma.Schema{Ref: "#/components/schemas/ProcSortBy"}
}

func calculateProcessCPUPercentageWithSample(sample *models.ProcessSampleData, currentCPUTime float64, currentTime int64) float64 {
	// Convert previous ticks back to seconds (since gopsutil uses seconds)
	previousCPUTime := float64(sample.PreviousTicks) / 100.0
	
	if sample.Timestamp == 0 || currentCPUTime <= previousCPUTime {
		return 0
	}
	
	cpuTimeDiff := currentCPUTime - previousCPUTime
	wallTimeDiff := float64(currentTime - sample.Timestamp) / 1000.0
	
	if wallTimeDiff <= 0 {
		return 0
	}
	
	// Calculate percentage: (cpu_time_used / wall_time_elapsed) * 100
	// This gives us the percentage of one CPU core used
	cpuPercent := (cpuTimeDiff / wallTimeDiff) * 100.0
	
	if cpuPercent > 100.0 {
		cpuPercent = 100.0
	}
	if cpuPercent < 0 {
		cpuPercent = 0
	}
	
	return cpuPercent
}
