//go:build linux

package gops

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/AvengeMedia/dgop/models"
)

// procGPU is one process's GPU readings. Readable false means everything is
// unknown. UsageKnown false means the process uses a GPU through a driver that
// reports no engine data, so VRAM is as read but a percentage is unknown.
type procGPU struct {
	Readable bool
	// Unmonitored: unknown because the process holds a GPU that is not queried.
	Unmonitored bool
	UsageKnown  bool
	VRAMBytes   uint64
	GTTBytes    uint64
	// Clients that report engines, for rating.
	Clients drmClientSet
}

// cursorClients is the form of Clients stored as the previous sample.
func (self procGPU) cursorClients() []gpuCursorClient {
	return cursorClients(self.Clients)
}

type procGPUReader struct {
	procRoot string
	// nodes maps /dev/dri node names to whether fdinfo may be read: true for
	// enabled, awake GPUs; false for excluded or suspended ones.
	nodes map[string]bool
	// nvidia is set when an NVIDIA GPU is awake.
	nvidia bool
	// VRAM bytes per process listed by nvidia-smi.
	nvidiaVRAM map[int32]uint64
}

// newProcGPUReader prepares per-process reads against the awake GPUs enabled
// by pciIds (nil enables all). It runs nvidia-smi once, and only if an
// enabled, awake NVIDIA GPU exists.
func newProcGPUReader(pciIds []string) *procGPUReader {
	detected, err := detectGPUs()
	if err != nil {
		return &procGPUReader{procRoot: "/proc"}
	}
	return buildProcGPUReader(pciDevicesRoot, detected, pciIds, queryNvidiaPIDs)
}

func buildProcGPUReader(pciRoot string, detected []models.GPU, pciIds []string, nvidiaPIDs func() map[int32]uint64) *procGPUReader {
	r := &procGPUReader{procRoot: "/proc"}
	var awake, skipped []string
	for _, g := range detected {
		if !gpuEnabled(pciIds, g.PciId, g.PciSlot) || gpuSuspended(pciRoot, g.PciSlot) {
			skipped = append(skipped, g.PciSlot)
			continue
		}
		awake = append(awake, g.PciSlot)
		r.nvidia = r.nvidia || g.Driver == "nvidia"
	}
	r.nodes = drmNodesAs(drmNodesAs(map[string]bool{}, false, pciRoot, skipped...), true, pciRoot, awake...)
	if r.nvidia {
		r.nvidiaVRAM = nvidiaPIDs()
	}
	return r
}

func (self *procGPUReader) read(pid int32) procGPU {
	if len(self.nodes) == 0 && !self.nvidia {
		f, err := os.Open(filepath.Join(self.procRoot, strconv.Itoa(int(pid)), "fd"))
		if err != nil {
			return procGPU{}
		}
		f.Close()
		return procGPU{Readable: true, UsageKnown: true}
	}

	scan, err := scanProcFDs(self.procRoot, pid, self.nodes)
	if err != nil {
		return procGPU{}
	}
	return self.fromScan(pid, scan)
}

// fromScan derives a process's readings from an fd scan taken with the
// reader's DRM nodes; the scan may also carry socket inodes.
func (self *procGPUReader) fromScan(pid int32, scan procFDScan) procGPU {
	if scan.ExcludedGPU {
		// Usage on a GPU we must not query was not measured.
		return procGPU{Unmonitored: true}
	}
	vram, listed := self.nvidiaVRAM[pid]
	res := procGPU{Readable: true, VRAMBytes: vram}
	for key, client := range scan.DRM {
		res.VRAMBytes += client.VRAMBytes
		res.GTTBytes += client.GTTBytes
		if !drmDriverReportsEngines(client.Driver) {
			continue
		}
		if res.Clients == nil {
			res.Clients = drmClientSet{}
		}
		res.Clients[key] = client
	}
	// An NVIDIA process is never known: on a hybrid laptop its DRM clients
	// would show only the integrated GPU's share.
	nvidiaProc := scan.NvidiaGPU || listed
	res.UsageKnown = !nvidiaProc && (len(res.Clients) > 0 || len(scan.DRM) == 0)
	return res
}

// procGPUUsage is the busy percent of one process's clients since the
// previous sample: max over all engines, and max over the video engines.
func procGPUUsage(prev []gpuCursorClient, cur drmClientSet, elapsed time.Duration) (all, video float64) {
	engines := drmEngineUtilization(cursorClientSet(prev), cur, elapsed)
	return maxEngineUtilization(engines), maxVideoEngineUtilization(engines)
}
