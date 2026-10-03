package gops

import (
	"runtime"

	"github.com/shirou/gopsutil/v4/process"
)

// processDiskBytes returns cumulative disk bytes read and written by p.
// ok is false when the counters are unavailable (unreadable /proc/<pid>/io,
// exited process, FreeBSD), so callers must not treat that as zero.
func processDiskBytes(p *process.Process) (read, write uint64, ok bool) {
	if runtime.GOOS == "freebsd" {
		return 0, 0, false
	}
	io, err := p.IOCounters()
	if err != nil || io == nil {
		return 0, 0, false
	}
	return io.DiskReadBytes, io.DiskWriteBytes, true
}
