package gops

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/AvengeMedia/dgop/models"
	"github.com/danielgtaylor/huma/v2"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/process"
)

const (
	cpuBaselineInterval   = 200 * time.Millisecond
	maxCursorDecodedBytes = 4 << 20
)

// A fresh gzip writer allocates ~850KB of flate hash tables, which is wasteful
// for the ~4KB cursor emitted on every poll. Reuse them instead.
var cursorWriters = sync.Pool{
	New: func() any {
		gz, _ := gzip.NewWriterLevel(io.Discard, gzip.BestCompression)
		return gz
	},
}

type processCursorWire struct {
	BaseMillis   int64   `json:"t"`
	PIDs         []int32 `json:"pid"`
	CPUMillis    []int64 `json:"cpu"`
	OffsetMillis []int64 `json:"dt"`
}

func encodeProcessCursor(entries []models.ProcessCursorData) string {
	wire := processCursorWire{
		PIDs:         make([]int32, 0, len(entries)),
		CPUMillis:    make([]int64, 0, len(entries)),
		OffsetMillis: make([]int64, 0, len(entries)),
	}
	for i, e := range entries {
		if i == 0 || e.Timestamp < wire.BaseMillis {
			wire.BaseMillis = e.Timestamp
		}
	}
	for _, e := range entries {
		wire.PIDs = append(wire.PIDs, e.PID)
		wire.CPUMillis = append(wire.CPUMillis, int64(math.Round(e.Ticks*1000)))
		wire.OffsetMillis = append(wire.OffsetMillis, e.Timestamp-wire.BaseMillis)
	}

	raw, _ := json.Marshal(wire)
	var buf bytes.Buffer
	gz := cursorWriters.Get().(*gzip.Writer)
	defer cursorWriters.Put(gz)
	gz.Reset(&buf)
	_, _ = gz.Write(raw)
	_ = gz.Close()
	return base64.RawURLEncoding.EncodeToString(buf.Bytes())
}

func decodeProcessCursor(cursor string) map[int32]*models.ProcessCursorData {
	out := make(map[int32]*models.ProcessCursorData)
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil || len(raw) == 0 {
		return out
	}

	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return out
	}
	defer func() { _ = gz.Close() }()
	if raw, err = io.ReadAll(io.LimitReader(gz, maxCursorDecodedBytes)); err != nil {
		return out
	}

	var wire processCursorWire
	if json.Unmarshal(raw, &wire) != nil {
		return out
	}
	if len(wire.CPUMillis) != len(wire.PIDs) || len(wire.OffsetMillis) != len(wire.PIDs) {
		return out
	}
	entries := make([]models.ProcessCursorData, len(wire.PIDs))
	for i, pid := range wire.PIDs {
		entries[i] = models.ProcessCursorData{
			PID:       pid,
			Ticks:     float64(wire.CPUMillis[i]) / 1000,
			Timestamp: wire.BaseMillis + wire.OffsetMillis[i],
		}
		out[pid] = &entries[i]
	}
	return out
}

// gopsutil can panic on macOS when reading process info for system processes
// or processes that exit mid-read, and this runs on the request goroutine
// where a panic would take down the whole call.
func readProcessTimes(p *process.Process) (times *cpu.TimesStat) {
	defer func() {
		if recover() != nil {
			times = nil
		}
	}()
	times, _ = p.Times()
	return times
}

func (self *GopsUtil) GetProcesses(sortBy ProcSortBy, limit int, enableCPU bool, mergeChildren bool) (*models.ProcessListResponse, error) {
	return self.GetProcessesWithCursor(sortBy, limit, enableCPU, "", mergeChildren)
}

func (self *GopsUtil) GetProcessesWithCursor(sortBy ProcSortBy, limit int, enableCPU bool, cursor string, mergeChildren bool) (*models.ProcessListResponse, error) {
	procs, err := self.procProvider.Processes()
	if err != nil {
		return nil, err
	}

	totalMem, _ := self.memProvider.VirtualMemory()

	cursorMap := decodeProcessCursor(cursor)

	if enableCPU && len(cursorMap) == 0 {
		for _, p := range procs {
			times := readProcessTimes(p)
			if times == nil {
				continue
			}
			cursorMap[p.Pid] = &models.ProcessCursorData{
				PID:       p.Pid,
				Ticks:     times.User + times.System,
				Timestamp: time.Now().UnixMilli(),
			}
		}
		time.Sleep(cpuBaselineInterval)
	}

	type procResult struct {
		index     int
		info      *models.ProcessInfo
		sampledAt int64
		sampled   bool
	}

	numCPU := float64(runtime.NumCPU())

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
					sampledAt := time.Now().UnixMilli()
					username, _ := p.Username()
					exePath, _ := p.Exe()

					currentCPUTime := float64(0)
					if times != nil {
						currentCPUTime = times.User + times.System
					}

					cpuPercent := 0.0
					if enableCPU {
						if cursorData, hasCursor := cursorMap[p.Pid]; hasCursor {
							coreRate := calculateProcessCPUPercentageWithCursor(cursorData, currentCPUTime, sampledAt)
							cpuPercent = coreRate / numCPU
							if cpuPercent > 100 {
								cpuPercent = 100
							}
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
						index:     idx,
						sampledAt: sampledAt,
						sampled:   true,
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
	cursorList := make([]models.ProcessCursorData, 0, len(procs))
	for i := 0; i < len(procs); i++ {
		r := <-results
		procList[r.index] = r.info
		if r.sampled {
			cursorList = append(cursorList, models.ProcessCursorData{
				PID:       r.info.PID,
				Ticks:     r.info.PTicks,
				Timestamp: r.sampledAt,
			})
		}
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

	return &models.ProcessListResponse{
		Processes: procList,
		Cursor:    encodeProcessCursor(cursorList),
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

	wallTimeDiff := float64(currentTime-cursor.Timestamp) / 1000.0
	if wallTimeDiff <= 0 {
		return 0
	}

	cpuTimeDiff := currentCPUTime - cursor.Ticks
	return (cpuTimeDiff / wallTimeDiff) * 100.0
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
