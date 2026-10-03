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
	"slices"
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
	maxProcRemotes        = 32
	// A socket first seen with more bytes than this rate over the interval
	// predates the previous sample, so its counters are not new traffic.
	maxNewSocketBytesPerSec = 100e9 / 8
)

// A fresh gzip writer allocates ~850KB of flate hash tables, which is wasteful
// for the ~4KB cursor emitted on every poll. Reuse them instead.
var cursorWriters = sync.Pool{
	New: func() any {
		gz, _ := gzip.NewWriterLevel(io.Discard, gzip.BestCompression)
		return gz
	},
}

type processCursorBase struct {
	BaseMillis   int64   `json:"t"`
	PIDs         []int32 `json:"pid"`
	CPUMillis    []int64 `json:"cpu"`
	OffsetMillis []int64 `json:"dt"`
}

type processCursorWire struct {
	processCursorBase
	// Disk counters, parallel to IOPIDs.
	IOPIDs  []int32  `json:"iop,omitempty"`
	IORead  []uint64 `json:"ior,omitempty"`
	IOWrite []uint64 `json:"iow,omitempty"`
	// Every PID whose GPU usage was sampled, with its engine clients ([] for none).
	GPU map[int32][]gpuCursorClient `json:"g,omitzero"`
	// Every PID whose sockets were sampled, with its internet TCP sockets as
	// [inode, received, acked] ([] for none).
	Net map[int32][][3]uint64 `json:"n,omitzero"`
	// NetAt is when the socket dump behind Net was taken (Unix ms), shared by
	// every PID in it, so rates use the dump interval rather than per-PID clocks.
	NetAt int64 `json:"nt,omitempty"`
}

// processSample is one process's cursor entry. ioOK and gpuOK say whether the
// sample holds disk counters and GPU clients, netOK its TCP sockets; an
// unsampled extra has no rate.
type processSample struct {
	models.ProcessCursorData
	ioOK  bool
	io    [2]uint64
	gpuOK bool
	gpu   []gpuCursorClient
	netOK bool
	net   [][3]uint64
	netAt int64
}

func encodeProcessCursor(entries []processSample) string {
	wire := processCursorWire{processCursorBase: processCursorBase{
		PIDs:         make([]int32, 0, len(entries)),
		CPUMillis:    make([]int64, 0, len(entries)),
		OffsetMillis: make([]int64, 0, len(entries)),
	}}
	for i, e := range entries {
		if i == 0 || e.Timestamp < wire.BaseMillis {
			wire.BaseMillis = e.Timestamp
		}
	}
	for _, e := range entries {
		wire.PIDs = append(wire.PIDs, e.PID)
		wire.CPUMillis = append(wire.CPUMillis, int64(math.Round(e.Ticks*1000)))
		wire.OffsetMillis = append(wire.OffsetMillis, e.Timestamp-wire.BaseMillis)
		if e.ioOK {
			wire.IOPIDs = append(wire.IOPIDs, e.PID)
			wire.IORead = append(wire.IORead, e.io[0])
			wire.IOWrite = append(wire.IOWrite, e.io[1])
		}
		if e.gpuOK {
			if wire.GPU == nil {
				wire.GPU = map[int32][]gpuCursorClient{}
			}
			wire.GPU[e.PID] = append([]gpuCursorClient{}, e.gpu...)
		}
		if e.netOK {
			if wire.Net == nil {
				wire.Net = map[int32][][3]uint64{}
			}
			wire.Net[e.PID] = append([][3]uint64{}, e.net...)
			wire.NetAt = e.netAt
		}
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

func decodeProcessCursor(cursor string) map[int32]*processSample {
	out := make(map[int32]*processSample)
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
		// Malformed extras must not cost the CPU baseline.
		wire = processCursorWire{}
		if json.Unmarshal(raw, &wire.processCursorBase) != nil {
			return out
		}
	}
	if len(wire.CPUMillis) != len(wire.PIDs) || len(wire.OffsetMillis) != len(wire.PIDs) {
		return out
	}
	entries := make([]processSample, len(wire.PIDs))
	for i, pid := range wire.PIDs {
		entries[i] = processSample{
			ProcessCursorData: models.ProcessCursorData{
				PID:       pid,
				Ticks:     float64(wire.CPUMillis[i]) / 1000,
				Timestamp: wire.BaseMillis + wire.OffsetMillis[i],
			},
		}
		entries[i].gpu, entries[i].gpuOK = wire.GPU[pid]
		if entries[i].net, entries[i].netOK = wire.Net[pid]; entries[i].netOK {
			entries[i].netAt = wire.NetAt
		}
		out[pid] = &entries[i]
	}
	if len(wire.IORead) == len(wire.IOPIDs) && len(wire.IOWrite) == len(wire.IOPIDs) {
		for i, pid := range wire.IOPIDs {
			if e := out[pid]; e != nil {
				e.ioOK, e.io = true, [2]uint64{wire.IORead[i], wire.IOWrite[i]}
			}
		}
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
	return self.GetProcessesWithOptions(ProcessOptions{SortBy: sortBy, Limit: limit, EnableCPU: enableCPU, Cursor: cursor, MergeChildren: mergeChildren})
}

// ProcessOptions selects the process list. GPU adds GPU usage and VRAM, IO
// adds disk read/write rates, Net adds internet connections and TCP rates;
// each costs extra reads per process.
type ProcessOptions struct {
	SortBy        ProcSortBy
	Limit         int
	EnableCPU     bool
	Cursor        string
	MergeChildren bool
	GPU           bool
	// GPUPciIds limits GPU reads to the named GPUs; nil means all.
	GPUPciIds []string
	IO        bool
	Net       bool
}

// processExtras reads the optional per-process counters into a sample.
type processExtras struct {
	gpu *procGPUReader
	net *procNetReader
	io  bool
}

// sample recovers like readProcessTimes, since the baseline pass calls it on
// the request goroutine.
func (self processExtras) sample(p *process.Process, s *processSample) (g procGPU, n procNet) {
	defer func() {
		if recover() != nil {
			g, n = procGPU{}, procNet{}
			s.ioOK, s.gpuOK, s.netOK = false, false, false
		}
	}()
	if self.io {
		var read, write uint64
		read, write, s.ioOK = processDiskBytes(p)
		s.io = [2]uint64{read, write}
	}
	switch {
	case self.gpu != nil && self.net != nil:
		g, n = readGPUAndNet(self.gpu, self.net, p.Pid)
	case self.gpu != nil:
		g = self.gpu.read(p.Pid)
	case self.net != nil:
		n = self.net.read(p.Pid)
	}
	if g.Readable && g.UsageKnown {
		s.gpuOK, s.gpu = true, g.cursorClients()
	}
	if n.Known {
		s.netOK, s.netAt = true, self.net.dumpedAt
		for _, c := range n.TCP {
			s.net = append(s.net, [3]uint64{uint64(c.Inode), c.RxBytes, c.TxBytes})
		}
	}
	return g, n
}

func (self *GopsUtil) GetProcessesWithOptions(opts ProcessOptions) (*models.ProcessListResponse, error) {
	sortBy, limit, enableCPU, mergeChildren := opts.SortBy, opts.Limit, opts.EnableCPU, opts.MergeChildren
	procs, err := self.procProvider.Processes()
	if err != nil {
		return nil, err
	}

	totalMem, _ := self.memProvider.VirtualMemory()

	cursorMap := decodeProcessCursor(opts.Cursor)

	extras := processExtras{io: opts.IO}
	if opts.GPU {
		extras.gpu = newProcGPUReader(opts.GPUPciIds)
	}
	if opts.Net {
		extras.net = newProcNetReader()
	}

	if enableCPU && len(cursorMap) == 0 {
		for _, p := range procs {
			times := readProcessTimes(p)
			if times == nil {
				continue
			}
			s := &processSample{ProcessCursorData: models.ProcessCursorData{
				PID:       p.Pid,
				Ticks:     times.User + times.System,
				Timestamp: time.Now().UnixMilli(),
			}}
			extras.sample(p, s)
			cursorMap[p.Pid] = s
		}
		time.Sleep(cpuBaselineInterval)
		if opts.Net {
			// Socket counters come from the dump, so the baseline needs its own.
			extras.net = newProcNetReader()
		}
	}

	type procResult struct {
		index   int
		info    *models.ProcessInfo
		sample  processSample
		sampled bool
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
					sample := processSample{ProcessCursorData: models.ProcessCursorData{PID: p.Pid, Timestamp: time.Now().UnixMilli()}}
					gpu, netInfo := extras.sample(p, &sample)
					sampledAt := sample.Timestamp
					username, _ := p.Username()
					exePath, _ := p.Exe()
					var created, startTime int64
					if c, err := p.CreateTime(); err == nil {
						created, startTime = c, c/1000
					}

					currentCPUTime := float64(0)
					if times != nil {
						currentCPUTime = times.User + times.System
					}
					sample.Ticks = currentCPUTime
					prev := cursorMap[p.Pid]

					cpuPercent := 0.0
					if enableCPU {
						if prev != nil {
							coreRate := calculateProcessCPUPercentageWithCursor(&prev.ProcessCursorData, currentCPUTime, sampledAt)
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

					info := &models.ProcessInfo{
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
						StartTime:         startTime,
					}
					setProcessExtras(info, prev, &sample, gpu, created)
					if netInfo.Known {
						info.Connections = new(netInfo.Count)
						for _, r := range netInfo.Remotes {
							info.Remotes = append(info.Remotes, models.NetConnection{Protocol: r.Protocol, Remote: r.Remote})
						}
					}

					results <- procResult{
						index:   idx,
						sample:  sample,
						sampled: times != nil,
						info:    info,
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
	cursorList := make([]processSample, 0, len(procs))
	for i := 0; i < len(procs); i++ {
		r := <-results
		procList[r.index] = r.info
		if r.sampled {
			cursorList = append(cursorList, r.sample)
		}
	}

	if mergeChildren {
		procList = mergeProcessesByExecutable(procList)
	}

	sortProcesses(procList, sortBy)

	if limit > 0 && len(procList) > limit {
		procList = procList[:limit]
	}

	return &models.ProcessListResponse{
		Processes: procList,
		Cursor:    encodeProcessCursor(cursorList),
	}, nil
}

// setProcessExtras fills the requested extras of info from the current and
// previous samples. Anything without a previous sample stays unknown, as does
// a previous sample taken before the process was created (a reused PID).
func setProcessExtras(info *models.ProcessInfo, prev, cur *processSample, gpu procGPU, createdMillis int64) {
	info.GPUUnmonitored = gpu.Unmonitored
	if gpu.Readable {
		info.VRAMKB = new(gpu.VRAMBytes / 1024)
	}
	if prev == nil || cur.Timestamp <= prev.Timestamp || createdMillis > prev.Timestamp {
		return
	}
	elapsed := time.Duration(cur.Timestamp-prev.Timestamp) * time.Millisecond
	if cur.gpuOK && prev.gpuOK && !gpu.Unmonitored {
		usage := procGPUUsage(prev.gpu, gpu.Clients, elapsed)
		info.GPU = &usage
	}
	if cur.ioOK && prev.ioOK && cur.io[0] >= prev.io[0] && cur.io[1] >= prev.io[1] {
		read := float64(cur.io[0]-prev.io[0]) / elapsed.Seconds()
		write := float64(cur.io[1]-prev.io[1]) / elapsed.Seconds()
		info.DiskRead, info.DiskWrite = &read, &write
	}
	if cur.netOK && prev.netOK && prev.netAt > 0 && cur.netAt > prev.netAt {
		netElapsed := time.Duration(cur.netAt-prev.netAt) * time.Millisecond
		rx, tx := tcpSocketDeltas(prev.net, cur.net, netElapsed)
		rx, tx = rx/netElapsed.Seconds(), tx/netElapsed.Seconds()
		info.NetRx, info.NetTx = &rx, &tx
	}
}

// tcpSocketDeltas sums per-socket byte deltas. A socket missing from prev
// opened during the interval and counts from zero, less the SYN that
// bytes_acked includes; counters going backwards count as 0.
func tcpSocketDeltas(prev, cur [][3]uint64, elapsed time.Duration) (rx, tx float64) {
	before := make(map[uint64][3]uint64, len(prev))
	for _, s := range prev {
		before[s[0]] = s
	}
	limit := maxNewSocketBytesPerSec * elapsed.Seconds()
	for _, s := range cur {
		p, seen := before[s[0]]
		if !seen {
			if float64(s[1]) <= limit && float64(s[2]) <= limit {
				rx += float64(s[1])
				tx += float64(s[2] - min(s[2], 1))
			}
			continue
		}
		if s[1] >= p[1] {
			rx += float64(s[1] - p[1])
		}
		if s[2] >= p[2] {
			tx += float64(s[2] - p[2])
		}
	}
	return rx, tx
}

func sortProcesses(procList []*models.ProcessInfo, sortBy ProcSortBy) {
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
	case SortByGPU:
		sort.Slice(procList, func(i, j int) bool {
			return knownGreater(procList[i].GPU, procList[j].GPU)
		})
	case SortByVRAM:
		sort.Slice(procList, func(i, j int) bool {
			return knownGreater(procList[i].VRAMKB, procList[j].VRAMKB)
		})
	case SortByDisk:
		sort.Slice(procList, func(i, j int) bool {
			return knownGreater(diskTotal(procList[i]), diskTotal(procList[j]))
		})
	case SortByNetwork:
		sort.Slice(procList, func(i, j int) bool {
			a, b := procList[i].Connections, procList[j].Connections
			if (a == nil) != (b == nil) || (a != nil && *a != *b) {
				return knownGreater(a, b)
			}
			return knownGreater(netTotal(procList[i]), netTotal(procList[j]))
		})
	default:
		sort.Slice(procList, func(i, j int) bool {
			return procList[i].CPU > procList[j].CPU
		})
	}
}

// knownGreater orders descending with unknown values last.
func knownGreater[T int | float64 | uint64](a, b *T) bool {
	if a == nil || b == nil {
		return a != nil && b == nil
	}
	return *a > *b
}

func diskTotal(p *models.ProcessInfo) *float64 {
	if p.DiskRead == nil || p.DiskWrite == nil {
		return nil
	}
	return new(*p.DiskRead + *p.DiskWrite)
}

func netTotal(p *models.ProcessInfo) *float64 {
	if p.NetRx == nil || p.NetTx == nil {
		return nil
	}
	return new(*p.NetRx + *p.NetTx)
}

type ProcSortBy string

const (
	SortByCPU    ProcSortBy = "cpu"
	SortByMemory ProcSortBy = "memory"
	SortByName   ProcSortBy = "name"
	SortByPID    ProcSortBy = "pid"
	SortByGPU    ProcSortBy = "gpu"
	SortByVRAM   ProcSortBy = "vram"
	SortByDisk   ProcSortBy = "disk"
	// SortByNetwork orders by internet connections, then TCP rx+tx.
	SortByNetwork ProcSortBy = "network"
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
			string(SortByGPU),
			string(SortByVRAM),
			string(SortByDisk),
			string(SortByNetwork),
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
			root.GPU = addKnown(root.GPU, p.GPU)
			if root.GPU != nil && *root.GPU > 100 {
				root.GPU = new(100.0)
			}
			root.GPUUnmonitored = root.GPUUnmonitored || p.GPUUnmonitored
			root.VRAMKB = addKnown(root.VRAMKB, p.VRAMKB)
			if root.GPUUnmonitored {
				root.GPU, root.VRAMKB = nil, nil
			}
			root.DiskRead = addKnown(root.DiskRead, p.DiskRead)
			root.DiskWrite = addKnown(root.DiskWrite, p.DiskWrite)
			root.Connections = addKnown(root.Connections, p.Connections)
			root.NetRx = addKnown(root.NetRx, p.NetRx)
			root.NetTx = addKnown(root.NetTx, p.NetTx)
			if len(p.Remotes) > 0 {
				// Clipped so the append never writes into the cloned row's array.
				root.Remotes = append(slices.Clip(root.Remotes), p.Remotes...)
				root.Remotes = root.Remotes[:min(len(root.Remotes), maxProcRemotes)]
			}
			root.ChildCount++
		}
	}

	result := make([]*models.ProcessInfo, 0, len(rootProcs))
	for _, p := range rootProcs {
		result = append(result, p)
	}
	return result
}

// addKnown sums two possibly unknown values into a new allocation, since
// merged roots share pointers with the rows they were cloned from.
func addKnown[T int | float64 | uint64](sum, v *T) *T {
	switch {
	case v == nil:
		return sum
	case sum == nil:
		return new(*v)
	default:
		return new(*sum + *v)
	}
}
