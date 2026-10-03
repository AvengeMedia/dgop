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
}

// scanProcFDs returns an error only when the fd directory itself is unreadable,
// so callers can tell an unknown process from one without matching fds.
// Only fdinfo of /dev/dri nodes in drmNodes is read: reading it runs the
// driver, which can resume a runtime-suspended GPU.
func scanProcFDs(procRoot string, pid int32, drmNodes map[string]bool) (procFDScan, error) {
	pidDir := filepath.Join(procRoot, strconv.Itoa(int(pid)))
	entries, err := os.ReadDir(filepath.Join(pidDir, "fd"))
	if err != nil {
		return procFDScan{}, err
	}

	scan := procFDScan{DRM: drmClientSet{}}
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join(pidDir, "fd", entry.Name()))
		node, isDRM := strings.CutPrefix(target, "/dev/dri/")
		if err != nil || !isDRM || !drmNodes[node] {
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

// drmNodes returns the /dev/dri node names of the given PCI devices, read from
// sysfs only.
func drmNodes(pciRoot string, bdfs ...string) map[string]bool {
	nodes := map[string]bool{}
	for _, bdf := range bdfs {
		entries, _ := os.ReadDir(filepath.Join(pciRoot, bdf, "drm"))
		for _, entry := range entries {
			nodes[entry.Name()] = true
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
