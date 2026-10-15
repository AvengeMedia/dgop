//go:build !linux

package gops

import "github.com/shirou/gopsutil/v4/process"

func getPssDirty(_ int32) (uint64, error) {
	return 0, nil
}

// readProcStat recovers because gopsutil can panic on macOS for system
// processes or ones that exit mid-read.
func readProcStat(p *process.Process) (st procStat, ok bool) {
	defer func() {
		if recover() != nil {
			st, ok = procStat{}, false
		}
	}()
	times, err := p.Times()
	if err != nil || times == nil {
		return procStat{}, false
	}
	st.CPUTime = times.User + times.System
	st.PPID, _ = p.Ppid()
	if c, err := p.CreateTime(); err == nil {
		st.CreatedMillis, st.Start = c, uint64(c)
	}
	return st, true
}
