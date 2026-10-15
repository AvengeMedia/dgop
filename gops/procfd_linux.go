//go:build linux

package gops

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"

	"github.com/AvengeMedia/dgop/models"
	"golang.org/x/sys/unix"
)

type gpuNodeState uint8

const (
	gpuNodeUnknown gpuNodeState = iota // a node of no detected GPU
	gpuNodeRead
	// A runtime-suspended GPU has been idle for at least its autosuspend delay.
	gpuNodeSuspended
	gpuNodeExcluded
)

type gpuNodeSet struct {
	nodes  map[string]gpuNodeState
	nvidia bool // an enabled NVIDIA GPU is awake
}

// gpuNodes reads sysfs only. Only gpuNodeRead nodes may have their fdinfo
// read: that runs the driver, which can resume a suspended GPU.
func gpuNodes(pciRoot string, gpus []models.GPU, pciIds []string) gpuNodeSet {
	set := gpuNodeSet{nodes: map[string]gpuNodeState{}}
	for _, g := range gpus {
		state := gpuNodeRead
		switch {
		case !gpuEnabled(pciIds, g.PciId, g.PciSlot):
			state = gpuNodeExcluded
		case gpuSuspended(pciRoot, g.PciSlot):
			state = gpuNodeSuspended
		}
		set.nvidia = set.nvidia || g.Driver == "nvidia" && state == gpuNodeRead
		entries, _ := os.ReadDir(filepath.Join(pciRoot, g.PciSlot, "drm"))
		for _, e := range entries {
			set.nodes[e.Name()] = state
		}
	}
	return set
}

type procFDScan struct {
	DRM drmClientSet
	// NvidiaGPU is set for an fd on /dev/nvidia<N>, which CUDA and other
	// proprietary-driver clients use without any /dev/dri fd.
	NvidiaGPU   bool
	ExcludedGPU bool
	UnknownGPU  bool
	// SocketInodes is filled only when sockets are requested.
	SocketInodes []uint32
}

var (
	driPrefix    = []byte("/dev/dri/")
	nvidiaPrefix = []byte("/dev/nvidia")
	socketPrefix = []byte("socket:[")
)

// scanProcFDs returns an error only when the fd directory itself is
// unreadable, so callers can tell an unknown process from one without
// matching fds.
func scanProcFDs(procRoot string, pid int32, nodes map[string]gpuNodeState, sockets bool) (procFDScan, error) {
	pidDir := filepath.Join(procRoot, strconv.Itoa(int(pid)))
	dir, err := os.Open(filepath.Join(pidDir, "fd"))
	if err != nil {
		return procFDScan{}, err
	}
	defer dir.Close()
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return procFDScan{}, err
	}

	var scan procFDScan
	dirfd := int(dir.Fd())
	buf := make([]byte, 64)
	for _, name := range names {
		n, err := unix.Readlinkat(dirfd, name, buf)
		if err != nil {
			continue
		}
		target := buf[:n]
		switch {
		case sockets && bytes.HasPrefix(target, socketPrefix):
			if inode, ok := parseSocketLink(target); ok {
				scan.SocketInodes = append(scan.SocketInodes, inode)
			}
		case bytes.HasPrefix(target, nvidiaPrefix):
			scan.NvidiaGPU = scan.NvidiaGPU || isNvidiaGPUDevice(target)
		case bytes.HasPrefix(target, driPrefix):
			switch nodes[string(target[len(driPrefix):])] {
			case gpuNodeUnknown:
				scan.UnknownGPU = true
			case gpuNodeExcluded:
				scan.ExcludedGPU = true
			case gpuNodeRead:
				data, err := os.ReadFile(filepath.Join(pidDir, "fdinfo", name))
				if err != nil {
					continue
				}
				if client, ok := parseDRMFdinfo(string(data)); ok {
					if scan.DRM == nil {
						scan.DRM = drmClientSet{}
					}
					scan.DRM[client.key()] = client
				}
			}
		}
	}
	return scan, nil
}

func (self processExtras) readFDs(procRoot string, pid int32) (g procGPU, n procNet) {
	sockets := self.net != nil && self.net.ok
	if self.gpu == nil && !sockets {
		return g, n
	}
	var nodes map[string]gpuNodeState
	if self.gpu != nil {
		nodes = self.gpu.nodes
	}
	scan, err := scanProcFDs(procRoot, pid, nodes, sockets)
	if err != nil {
		return g, n
	}
	if self.gpu != nil {
		g = self.gpu.fromScan(pid, scan)
	}
	if sockets {
		n = self.net.fromScan(scan)
	}
	return g, n
}

func parseSocketLink(target []byte) (uint32, bool) {
	s, ok := bytes.CutPrefix(target, socketPrefix)
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseUint(string(bytes.TrimSuffix(s, []byte("]"))), 10, 32)
	return uint32(n), err == nil
}

func isNvidiaGPUDevice(target []byte) bool {
	n, ok := bytes.CutPrefix(target, nvidiaPrefix)
	if !ok || len(n) == 0 {
		return false
	}
	for _, c := range n {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func listProcPIDs(procRoot string) ([]int32, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil, err
	}
	pids := make([]int32, 0, len(entries))
	for _, entry := range entries {
		pid, err := strconv.ParseInt(entry.Name(), 10, 32)
		if err != nil || pid <= 0 {
			continue
		}
		pids = append(pids, int32(pid))
	}
	return pids, nil
}
