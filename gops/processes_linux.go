//go:build linux

package gops

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func getPssDirty(pid int32) (uint64, error) {
	smapsRollupPath := fmt.Sprintf("/proc/%d/smaps_rollup", pid)
	contents, err := os.ReadFile(smapsRollupPath)
	if err != nil {
		return 0, err
	}

	for _, line := range strings.Split(string(contents), "\n") {
		if !strings.HasPrefix(line, "Pss_Dirty:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			val, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return 0, err
			}
			return val, nil
		}
	}
	return 0, fmt.Errorf("Pss_Dirty not found")
}

type procStat struct {
	Status  string
	Nice    int
	Threads int
}

func readProcStat(pid int32) (procStat, bool) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return procStat{}, false
	}
	return parseProcStat(string(data))
}

// parseProcStat reads state (field 3), nice (19) and num_threads (20). The
// command name may contain spaces and parentheses, so fields count from the
// last ')'.
func parseProcStat(text string) (procStat, bool) {
	i := strings.LastIndexByte(text, ')')
	if i < 0 {
		return procStat{}, false
	}
	fields := strings.Fields(text[i+1:])
	if len(fields) < 18 {
		return procStat{}, false
	}
	nice, err1 := strconv.Atoi(fields[16])
	threads, err2 := strconv.Atoi(fields[17])
	if err1 != nil || err2 != nil {
		return procStat{}, false
	}
	return procStat{Status: fields[0], Nice: nice, Threads: threads}, true
}
