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
	p := MetaParams{SortBy: SortByCPU, ProcLimit: 100, EnableCPU: true, MergeChildren: true, NetRateAllInterfaces: true, GPUPciIds: allGPUBusIds(b, u)}
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
		if m.GPU != nil {
			p.GPUCursor = m.GPU.Cursor
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

// allGPUBusIds is what DMS passes to read every GPU.
func allGPUBusIds(b *testing.B, u *GopsUtil) []string {
	info, err := u.GetGPUInfo()
	if err != nil {
		b.Fatal(err)
	}
	ids := []string{}
	for _, g := range info.GPUs {
		ids = append(ids, g.BusId)
	}
	return ids
}

func cpuTime() int64 {
	var r syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &r)
	return r.Utime.Nano() + r.Stime.Nano()
}

func BenchmarkMetaBar(b *testing.B) { benchView(b, "cpu", "memory", "system") }

func BenchmarkMetaProcessesTab(b *testing.B) {
	benchView(b, "processes", "cpu", "memory", "system", "gpu", "process-gpu", "process-io", "disk", "process-net", "network")
}

func BenchmarkMetaPerformanceTab(b *testing.B) {
	benchView(b, "cpu", "memory", "system", "disk-rate", "net-rate", "diskmounts", "gpu", "memory-modules")
}

func BenchmarkMetaGPU(b *testing.B) { benchView(b, "gpu") }

func BenchmarkMetaSystemTab(b *testing.B) { benchView(b, "system", "cpu", "memory-modules", "sensors") }

func BenchmarkMetaNetRate(b *testing.B) { benchView(b, "net-rate") }

func BenchmarkMetaSensors(b *testing.B) { benchView(b, "sensors") }
