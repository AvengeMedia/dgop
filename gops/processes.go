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
	procs, err := process.Processes()
	if err != nil {
		return nil, err
	}

	procList := make([]*models.ProcessInfo, 0)
	totalMem, _ := mem.VirtualMemory()
	numCPU := float64(runtime.NumCPU())

	// CPU measurement setup - only if enabled
	if enableCPU {
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

		// Get CPU percentage only if enabled
		cpuPercent := 0.0
		if enableCPU {
			rawCpuPercent, _ := p.CPUPercent()
			cpuPercent = rawCpuPercent / numCPU
		}

		// Calculate memory percentage and KB
		memKB := uint64(0)
		memPercent := float32(0)
		if memInfo != nil {
			memKB = memInfo.RSS / 1024
			memPercent = float32(memInfo.RSS) / float32(totalMem.Total) * 100
		}

		// Calculate ticks (user + system time)
		ticks := uint64(0)
		if times != nil {
			ticks = uint64(times.User + times.System)
		}

		procList = append(procList, &models.ProcessInfo{
			PID:           p.Pid,
			PPID:          ppid,
			CPU:           cpuPercent,
			PTicks:        ticks,
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
