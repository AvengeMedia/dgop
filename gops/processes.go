package gops

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"time"

	"github.com/AvengeMedia/dgop/models"
	"github.com/danielgtaylor/huma/v2"
)

func (self *GopsUtil) GetProcesses(sortBy ProcSortBy, limit int, enableCPU bool, mergeChildren bool) (*models.ProcessListResponse, error) {
	return self.GetProcessesWithCursor(sortBy, limit, enableCPU, "", mergeChildren)
}

func (self *GopsUtil) GetProcessesWithCursor(sortBy ProcSortBy, limit int, enableCPU bool, cursor string, mergeChildren bool) (*models.ProcessListResponse, error) {
	procs, err := self.procProvider.Processes()
	if err != nil {
		return nil, err
	}

	totalMem, _ := self.memProvider.VirtualMemory()
	currentTime := time.Now().UnixMilli()

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

	if enableCPU && len(cursorMap) == 0 {
		maxSample := 100
		if len(procs) < maxSample {
			maxSample = len(procs)
		}
		for i := 0; i < maxSample; i++ {
			_, _ = procs[i].CPUPercent()
		}
		time.Sleep(200 * time.Millisecond)
	}

	type procResult struct {
		index int
		info  *models.ProcessInfo
	}

	numWorkers := runtime.NumCPU()
	if numWorkers > 8 {
		numWorkers = 8
	}

	jobs := make(chan int, len(procs))
	results := make(chan procResult, len(procs))

	for w := 0; w < numWorkers; w++ {
		go func() {
			for idx := range jobs {
				p := procs[idx]

				// gopsutil can panic on macOS when reading process info
				// for system processes or processes that exit mid-read.
				func() {
					defer func() {
						if r := recover(); r != nil {
							results <- procResult{
								index: idx,
								info: &models.ProcessInfo{
									PID:     p.Pid,
									Command: fmt.Sprintf("[pid %d]", p.Pid),
								},
							}
						}
					}()

					name, _ := p.Name()
					cmdline, _ := p.Cmdline()
					ppid, _ := p.Ppid()
					memInfo, _ := p.MemoryInfo()
					times, _ := p.Times()
					username, _ := p.Username()
					exePath, _ := p.Exe()

					currentCPUTime := float64(0)
					if times != nil {
						currentCPUTime = times.User + times.System
					}

					cpuPercent := 0.0
					if enableCPU {
						rawCpuPercent, _ := p.CPUPercent()
						cpuPercent = rawCpuPercent / float64(runtime.NumCPU())
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

						if rssKB > 102400 {
							pssDirty, err := getPssDirty(p.Pid)
							if err == nil && pssDirty > 0 {
								memKB = pssDirty
								memPercent = float32(memKB*1024) / float32(totalMem.Total) * 100
								memCalc = "pss_dirty"
							}
						}
					}

					results <- procResult{
						index: idx,
						info: &models.ProcessInfo{
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
							ExecutablePath:    exePath,
						},
					}
				}()
			}
		}()
	}

	for i := range procs {
		jobs <- i
	}
	close(jobs)

	procList := make([]*models.ProcessInfo, len(procs))
	for i := 0; i < len(procs); i++ {
		r := <-results
		procList[r.index] = r.info
	}

	if mergeChildren {
		procList = mergeProcessesByExecutable(procList)
	}

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

	if limit > 0 && len(procList) > limit {
		procList = procList[:limit]
	}

	cursorList := make([]models.ProcessCursorData, 0, len(procList))
	for _, proc := range procList {
		cursorList = append(cursorList, models.ProcessCursorData{
			PID:       proc.PID,
			Ticks:     proc.PTicks,
			Timestamp: currentTime,
		})
	}

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

func findMergeRoot(p *models.ProcessInfo, pidMap map[int32]*models.ProcessInfo) *models.ProcessInfo {
	parent, exists := pidMap[p.PPID]
	switch {
	case !exists:
		return p
	case parent.ExecutablePath == "" || p.ExecutablePath == "":
		return p
	case parent.ExecutablePath != p.ExecutablePath:
		return p
	default:
		return findMergeRoot(parent, pidMap)
	}
}

func mergeProcessesByExecutable(procList []*models.ProcessInfo) []*models.ProcessInfo {
	pidMap := make(map[int32]*models.ProcessInfo)
	for _, p := range procList {
		pidMap[p.PID] = p
	}

	mergeRoots := make(map[int32]int32)
	for _, p := range procList {
		root := findMergeRoot(p, pidMap)
		mergeRoots[p.PID] = root.PID
	}

	rootProcs := make(map[int32]*models.ProcessInfo)
	for _, p := range procList {
		rootPID := mergeRoots[p.PID]
		root := rootProcs[rootPID]
		if root == nil {
			clone := *pidMap[rootPID]
			clone.ChildCount = 0
			rootProcs[rootPID] = &clone
			root = rootProcs[rootPID]
		}

		if p.PID != rootPID {
			root.CPU += p.CPU
			root.MemoryKB += p.MemoryKB
			root.MemoryPercent += p.MemoryPercent
			root.RSSKB += p.RSSKB
			root.RSSPercent += p.RSSPercent
			root.PSSKB += p.PSSKB
			root.PSSPercent += p.PSSPercent
			root.ChildCount++
		}
	}

	result := make([]*models.ProcessInfo, 0, len(rootProcs))
	for _, p := range rootProcs {
		result = append(result, p)
	}
	return result
}
