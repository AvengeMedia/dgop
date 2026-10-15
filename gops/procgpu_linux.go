//go:build linux

package gops

import (
	"time"

	"github.com/AvengeMedia/dgop/models"
)

type procGPUReader struct {
	gpuNodeSet
	// VRAM bytes per process listed by nvidia-smi.
	nvidiaVRAM map[int32]uint64
}

// newProcGPUReader prepares per-process reads against the GPUs enabled by
// pciIds (nil enables all). It runs nvidia-smi once, and only if an enabled,
// awake NVIDIA GPU exists.
func (self *GopsUtil) newProcGPUReader(pciIds []string) *procGPUReader {
	detected, _ := self.detectGPUs()
	return buildProcGPUReader(pciDevicesRoot, detected, pciIds, queryNvidiaPIDs)
}

func buildProcGPUReader(pciRoot string, detected []models.GPU, pciIds []string, nvidiaPIDs func() map[int32]uint64) *procGPUReader {
	r := &procGPUReader{gpuNodeSet: gpuNodes(pciRoot, detected, pciIds)}
	if r.nvidia {
		r.nvidiaVRAM = nvidiaPIDs()
	}
	return r
}

func (self *procGPUReader) fromScan(pid int32, scan procFDScan) procGPU {
	if scan.ExcludedGPU || scan.NvidiaGPU && !self.nvidia {
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
	res.UsageKnown = !nvidiaProc && !scan.UnknownGPU && (len(res.Clients) > 0 || len(scan.DRM) == 0)
	return res
}

// procGPUUsage is the busy percent of one process's clients since the
// previous sample: max over all engines, and max over the video engines.
func procGPUUsage(prev []gpuCursorClient, cur drmClientSet, elapsed time.Duration) (all, video float64) {
	engines := drmEngineUtilization(cursorClientSet(prev), cur, elapsed)
	return maxEngineUtilization(engines), maxVideoEngineUtilization(engines)
}
