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
