//go:build linux

package gops

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/process"
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

// Linux reports stat times in USER_HZ, which is 100 on every architecture.
const clockTicks = 100

var bootMillis = sync.OnceValue(func() int64 {
	t, err := host.BootTime()
	if err != nil {
		return 0
	}
	return int64(t) * 1000
})

func readProcStat(p *process.Process) (procStat, bool) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", p.Pid))
	if err != nil {
		return procStat{}, false
	}
	st, ok := parseProcStat(string(data))
	if ok && bootMillis() > 0 {
		st.CreatedMillis = bootMillis() + int64(st.Start)*1000/clockTicks
	}
	return st, ok
}

// parseProcStat counts fields from the last ')', since the command name may
// contain spaces and parentheses.
func parseProcStat(text string) (procStat, bool) {
	i := strings.LastIndexByte(text, ')')
	if i < 0 {
		return procStat{}, false
	}
	f := strings.Fields(text[i+1:])
	if len(f) < 20 {
		return procStat{}, false
	}
	ppid, err1 := strconv.ParseInt(f[1], 10, 32)
	tty, err2 := strconv.Atoi(f[4])
	utime, err3 := strconv.ParseUint(f[11], 10, 64)
	stime, err4 := strconv.ParseUint(f[12], 10, 64)
	nice, err5 := strconv.Atoi(f[16])
	threads, err6 := strconv.Atoi(f[17])
	start, err7 := strconv.ParseUint(f[19], 10, 64)
	if err := errors.Join(err1, err2, err3, err4, err5, err6, err7); err != nil {
		return procStat{}, false
	}
	return procStat{
		PPID:    int32(ppid),
		State:   f[0],
		Nice:    nice,
		Threads: threads,
		TTY:     tty,
		CPUTime: float64(utime+stime) / clockTicks,
		Start:   start,
	}, true
}
