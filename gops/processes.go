package gops

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/AvengeMedia/dgop/models"
	"github.com/danielgtaylor/huma/v2"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
)

func getPssDirty(pid int32) (uint64, error) {
	smapsRollupPath := fmt.Sprintf("/proc/%d/smaps_rollup", pid)
	contents, err := os.ReadFile(smapsRollupPath)
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(contents), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Pss_Dirty:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				val, err := strconv.ParseUint(fields[1], 10, 64)
				if err != nil {
					return 0, err
				}
				return val, nil
			}
		}
	}
	return 0, fmt.Errorf("Pss_Dirty not found")
}

func (self *GopsUtil) GetProcesses(sortBy ProcSortBy, limit int, enableCPU bool) (*models.ProcessListResponse, error) {
	return self.GetProcessesWithCursor(sortBy, limit, enableCPU, "")
}

func (self *GopsUtil) GetProcessesWithCursor(sortBy ProcSortBy, limit int, enableCPU bool, cursor string) (*models.ProcessListResponse, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, err
	}

	procList := make([]*models.ProcessInfo, 0)
	totalMem, _ := mem.VirtualMemory()
	currentTime := time.Now().UnixMilli()

	// Decode cursor string into cursor data map
	cursorMap := make(map[int32]*models.ProcessCursorData)
	if cursor != "" {
		jsonBytes, err := base64.RawURLEncoding.DecodeString(cursor)
		if err == nil {
			var cursors []models.ProcessCursorData
			if json.Unmarshal(jsonBytes, &cursors) == nil {
				for i := range cursors {
					cursorMap[cursors[i].PID] = &cursors[i]
				}
			}
		}
	}

	// CPU measurement setup - only if enabled and no cursor data provided
	if enableCPU && len(cursorMap) == 0 {
		// First pass: Initialize CPU measurement for all processes
		for _, p := range procs {
			p.CPUPercent() // Initialize
		}

		// Wait for measurement period (1 second for more accurate readings)
		time.Sleep(1000 * time.Millisecond)
	}

	for _, p := range procs {
		name, _ := p.Name()
		cmdline, _ := p.Cmdline()
		ppid, _ := p.Ppid()
		memInfo, _ := p.MemoryInfo()
		times, _ := p.Times()
		username, _ := p.Username()

		currentCPUTime := float64(0)
		if times != nil {
			currentCPUTime = times.User + times.System
		}

		cpuPercent := 0.0
		if enableCPU {
			if cursorData, hasCursor := cursorMap[p.Pid]; hasCursor {
				cpuPercent = calculateProcessCPUPercentageWithCursor(cursorData, currentCPUTime, currentTime)
			} else {
				rawCpuPercent, _ := p.CPUPercent()
				cpuPercent = rawCpuPercent / float64(runtime.NumCPU())
			}
		}

		rssKB := uint64(0)
		rssPercent := float32(0)
		pssKB := uint64(0)
		pssPercent := float32(0)
		memKB := uint64(0)
		memPercent := float32(0)
		memCalc := "rss"

		if memInfo != nil {
			rssKB = memInfo.RSS / 1024
			rssPercent = float32(memInfo.RSS) / float32(totalMem.Total) * 100

			memKB = rssKB
			memPercent = rssPercent

			if rssKB > 20480 {
				pssDirty, err := getPssDirty(p.Pid)
				if err == nil && pssDirty > 0 {
					memMaps, _ := p.MemoryMaps(true)
					if memMaps != nil && len(*memMaps) > 0 {
						pssKB = (*memMaps)[0].Pss
						pssPercent = float32(pssKB*1024) / float32(totalMem.Total) * 100
					}

					memKB = pssDirty
					memPercent = float32(memKB*1024) / float32(totalMem.Total) * 100
					memCalc = "pss_dirty"
				}
			}
		}

		procList = append(procList, &models.ProcessInfo{
			PID:               p.Pid,
			PPID:              ppid,
			CPU:               cpuPercent,
			PTicks:            currentCPUTime,
			MemoryPercent:     memPercent,
			MemoryKB:          memKB,
			MemoryCalculation: memCalc,
			RSSKB:             rssKB,
			RSSPercent:        rssPercent,
			PSSKB:             pssKB,
			PSSPercent:        pssPercent,
			Username:          username,
			Command:           name,
			FullCommand:       cmdline,
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

	// Create cursor data for all processes
	cursorList := make([]models.ProcessCursorData, 0, len(procList))
	for _, proc := range procList {
		cursorList = append(cursorList, models.ProcessCursorData{
			PID:       proc.PID,
			Ticks:     proc.PTicks,
			Timestamp: currentTime,
		})
	}

	// Encode cursor list as single base64 string
	cursorBytes, _ := json.Marshal(cursorList)
	cursorStr := base64.RawURLEncoding.EncodeToString(cursorBytes)

	return &models.ProcessListResponse{
		Processes: procList,
		Cursor:    cursorStr,
	}, nil
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

func calculateProcessCPUPercentageWithCursor(cursor *models.ProcessCursorData, currentCPUTime float64, currentTime int64) float64 {
	if cursor.Timestamp == 0 || currentCPUTime <= cursor.Ticks {
		return 0
	}

	cpuTimeDiff := currentCPUTime - cursor.Ticks
	wallTimeDiff := float64(currentTime-cursor.Timestamp) / 1000.0

	if wallTimeDiff <= 0 {
		return 0
	}

	cpuPercent := (cpuTimeDiff / wallTimeDiff) * 100.0

	if cpuPercent > 100.0 {
		cpuPercent = 100.0
	}
	if cpuPercent < 0 {
		cpuPercent = 0
	}

	return cpuPercent
}
