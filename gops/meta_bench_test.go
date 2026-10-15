//go:build unix

package gops

import (
	"context"
	"syscall"
	"testing"
)

// benchView polls GetMeta like DMS does, passing every cursor back, and
// reports process CPU time per call next to wall time, since collection
// runs in parallel.
func benchView(b *testing.B, modules ...string) {
	u := NewGopsUtil()
	p := MetaParams{SortBy: SortByCPU, ProcLimit: 100, EnableCPU: true, MergeChildren: true, NetRateAllInterfaces: true}
	poll := func() {
		m, err := u.GetMeta(context.Background(), modules, p)
		if err != nil {
			b.Fatal(err)
		}
		p.ProcCursor = m.Cursor
		if m.CPU != nil {
			p.CPUCursor = m.CPU.Cursor
		}
		if m.NetRate != nil {
			p.NetRateCursor = m.NetRate.Cursor
		}
		if m.DiskRate != nil {
			p.DiskRateCursor = m.DiskRate.Cursor
		}
		if m.GPUStats != nil {
			p.GPUStatsCursor = m.GPUStats.Cursor
		}
	}
	poll()
	b.ReportAllocs()
	b.ResetTimer()
	start := cpuTime()
	for range b.N {
		poll()
	}
	b.ReportMetric(float64(cpuTime()-start)/float64(b.N), "cpu-ns/op")
}

func cpuTime() int64 {
	var r syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &r)
	return r.Utime.Nano() + r.Stime.Nano()
}

func BenchmarkMetaBar(b *testing.B) { benchView(b, "cpu", "memory", "system") }

func BenchmarkMetaProcessesTab(b *testing.B) {
	benchView(b, "processes", "cpu", "memory", "system", "gpu", "process-gpu", "gpu-stats", "process-io", "disk", "process-net", "network")
}

func BenchmarkMetaPerformanceTab(b *testing.B) {
	benchView(b, "cpu", "memory", "system", "disk-rate", "net-rate", "diskmounts", "gpu-stats", "memory-modules")
}

func BenchmarkMetaSystemTab(b *testing.B) { benchView(b, "system", "cpu", "memory-modules", "sensors") }

func BenchmarkMetaNetRate(b *testing.B) { benchView(b, "net-rate") }

func BenchmarkMetaSensors(b *testing.B) { benchView(b, "sensors") }
