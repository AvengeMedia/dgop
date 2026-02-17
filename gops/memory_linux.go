//go:build linux

package gops

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/AvengeMedia/dgop/models"
)

func (self *GopsUtil) GetMemoryInfo() (*models.MemoryInfo, error) {
	v, err := self.memProvider.VirtualMemory()
	if err != nil {
		return nil, err
	}

	total := v.Total / 1024
	free := v.Free / 1024
	buffers := v.Buffers / 1024
	cached := v.Cached / 1024
	sreclaimable := v.Sreclaimable / 1024
	shared := v.Shared / 1024
	available := v.Available / 1024

	rawCached := cached - sreclaimable

	arcSize, arcMin := readZfsArcStats()
	arcSizeKB := arcSize / 1024
	arcMinKB := arcMin / 1024

	var freeableArc uint64
	switch {
	case arcSizeKB > arcMinKB:
		freeableArc = arcSizeKB - arcMinKB
	}

	rawCached += arcSizeKB
	available += freeableArc

	usedDiff := free + rawCached + sreclaimable + buffers
	var used uint64
	switch {
	case total >= usedDiff:
		used = total - usedDiff + shared
	default:
		used = total - free
	}

	var usedPercent float64
	if total > 0 {
		usedPercent = float64(used) / float64(total) * 100
	}

	return &models.MemoryInfo{
		Total:        total,
		Used:         used,
		UsedPercent:  usedPercent,
		Free:         free,
		Available:    available,
		Buffers:      buffers,
		Cached:       rawCached,
		SReclaimable: sreclaimable,
		Shared:       shared,
		ZfsArcSize:   arcSizeKB,
		SwapTotal:    v.SwapTotal / 1024,
		SwapFree:     v.SwapFree / 1024,
	}, nil
}

func readZfsArcStats() (size uint64, cMin uint64) {
	f, err := os.Open("/proc/spl/kstat/zfs/arcstats")
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 3 {
			continue
		}
		switch fields[0] {
		case "size":
			size, _ = strconv.ParseUint(fields[2], 10, 64)
		case "c_min":
			cMin, _ = strconv.ParseUint(fields[2], 10, 64)
		}
	}
	return size, cMin
}
