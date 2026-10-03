//go:build linux

package gops

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type procFDScan struct {
	DRM drmClientSet
	// NvidiaGPU is set for an fd on /dev/nvidia<N>, which CUDA and other
	// proprietary-driver clients use without any /dev/dri fd.
	NvidiaGPU bool
	// ExcludedGPU is set for an fd on a drmNodes node mapped to false: a GPU
	// whose fdinfo must not be read. Only the fd link is looked at.
	ExcludedGPU bool
	// SocketInodes is filled only when sockets are requested.
	SocketInodes []uint32
}

// scanProcFDs returns an error only when the fd directory itself is unreadable,
// so callers can tell an unknown process from one without matching fds.
// Only fdinfo of /dev/dri nodes mapped to true in drmNodes is read: reading it
// runs the driver, which can resume a runtime-suspended GPU.
func scanProcFDs(procRoot string, pid int32, drmNodes map[string]bool) (procFDScan, error) {
	return scanProcFDsOpt(procRoot, pid, drmNodes, false)
}

// scanProcFDsOpt additionally collects socket inodes when sockets is set.
func scanProcFDsOpt(procRoot string, pid int32, drmNodes map[string]bool, sockets bool) (procFDScan, error) {
	pidDir := filepath.Join(procRoot, strconv.Itoa(int(pid)))
	entries, err := os.ReadDir(filepath.Join(pidDir, "fd"))
	if err != nil {
		return procFDScan{}, err
	}

	scan := procFDScan{DRM: drmClientSet{}}
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join(pidDir, "fd", entry.Name()))
		if err == nil && isNvidiaGPUDevice(target) {
			scan.NvidiaGPU = true
			continue
		}
		if sockets && err == nil {
			if inode, ok := parseSocketLink(target); ok {
				scan.SocketInodes = append(scan.SocketInodes, inode)
				continue
			}
		}
		node, isDRM := strings.CutPrefix(target, "/dev/dri/")
		if err != nil || !isDRM {
			continue
		}
		if read, known := drmNodes[node]; !read {
			scan.ExcludedGPU = scan.ExcludedGPU || known
			continue
		}
		data, err := os.ReadFile(filepath.Join(pidDir, "fdinfo", entry.Name()))
		if err != nil {
			continue
		}
		if client, ok := parseDRMFdinfo(string(data)); ok {
			scan.DRM[client.key()] = client
		}
	}
	return scan, nil
}

func parseSocketLink(target string) (uint32, bool) {
	s, ok := strings.CutPrefix(target, "socket:[")
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseUint(strings.TrimSuffix(s, "]"), 10, 32)
	return uint32(n), err == nil
}

func isNvidiaGPUDevice(path string) bool {
	n, ok := strings.CutPrefix(path, "/dev/nvidia")
	if !ok || n == "" {
		return false
	}
	_, err := strconv.ParseUint(n, 10, 32)
	return err == nil
}

// drmNodes returns the /dev/dri node names of the given PCI devices, read from
// sysfs only.
func drmNodes(pciRoot string, bdfs ...string) map[string]bool {
	return drmNodesAs(map[string]bool{}, true, pciRoot, bdfs...)
}

// drmNodesAs adds the nodes of the given PCI devices to nodes with value v.
func drmNodesAs(nodes map[string]bool, v bool, pciRoot string, bdfs ...string) map[string]bool {
	for _, bdf := range bdfs {
		entries, _ := os.ReadDir(filepath.Join(pciRoot, bdf, "drm"))
		for _, entry := range entries {
			nodes[entry.Name()] = v
		}
	}
	return nodes
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
