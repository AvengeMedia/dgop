//go:build linux

package gops

import (
	"net/netip"
	"time"

	"golang.org/x/sys/unix"
)

type procRemote struct {
	Protocol string
	Remote   string
}

type tcpCounters struct {
	Inode   uint32
	RxBytes uint64
	TxBytes uint64
}

type procNet struct {
	// Known is false when the process's fds or the socket dump were unreadable.
	Known bool
	// Count is the exact number of sockets with an internet remote.
	Count   int
	Remotes []procRemote
	// TCP lists the internet TCP sockets only.
	TCP []tcpCounters
	// RatesUnknown is set when a TCP socket came without byte counters, so
	// summing the rest would under-report.
	RatesUnknown bool
}

var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// isInternetAddr reports whether a is a global unicast address outside
// private (RFC 1918, ULA) and CGNAT ranges. Documentation, benchmark, 6to4,
// Teredo and NAT64 ranges count as internet on purpose: they are never LAN peers.
func isInternetAddr(a netip.Addr) bool {
	a = a.Unmap()
	return a.IsValid() && a.IsGlobalUnicast() && !a.IsPrivate() && !cgnat.Contains(a)
}

type procNetReader struct {
	procRoot string
	socks    map[uint32]sockDiagEntry
	ok       bool
	// dumpedAt is when the socket dump was taken, in Unix milliseconds.
	dumpedAt int64
}

// sockDump is the socket source of newProcNetReader; tests replace it.
var sockDump = dumpSockets

// newProcNetReader takes one socket dump to serve a whole sampling pass.
func newProcNetReader() *procNetReader {
	r := &procNetReader{procRoot: "/proc"}
	r.dumpedAt = time.Now().UnixMilli()
	entries, err := sockDump()
	if err != nil {
		return r
	}
	r.socks = make(map[uint32]sockDiagEntry, len(entries))
	for _, e := range entries {
		r.socks[e.Inode] = e
	}
	r.ok = true
	return r
}

func (self *procNetReader) read(pid int32) procNet {
	if !self.ok {
		return procNet{}
	}
	scan, err := scanProcFDsOpt(self.procRoot, pid, nil, true)
	if err != nil {
		return procNet{}
	}
	return self.fromScan(scan)
}

// fromScan derives a process's readings from an fd scan taken with sockets on.
// A socket held by several fds counts once.
func (self *procNetReader) fromScan(scan procFDScan) procNet {
	if !self.ok {
		return procNet{}
	}
	out := procNet{Known: true}
	seen := make(map[uint32]bool, len(scan.SocketInodes))
	for _, inode := range scan.SocketInodes {
		if seen[inode] {
			continue
		}
		seen[inode] = true
		e, found := self.socks[inode]
		if !found || !isInternetAddr(e.Remote.Addr()) {
			continue
		}
		out.Count++
		proto := "udp"
		if e.Protocol == unix.IPPROTO_TCP {
			proto = "tcp"
			if e.HasTCPInfo {
				out.TCP = append(out.TCP, tcpCounters{inode, e.RxBytes, e.TxBytes})
			} else {
				out.RatesUnknown = true
			}
		}
		if len(out.Remotes) < maxProcRemotes {
			out.Remotes = append(out.Remotes, procRemote{proto, netip.AddrPortFrom(e.Remote.Addr().Unmap(), e.Remote.Port()).String()})
		}
	}
	return out
}

// readGPUAndNet reads both from a single fd walk.
func readGPUAndNet(g *procGPUReader, n *procNetReader, pid int32) (procGPU, procNet) {
	if !n.ok {
		return g.read(pid), procNet{}
	}
	scan, err := scanProcFDsOpt(n.procRoot, pid, g.nodes, true)
	if err != nil {
		return procGPU{}, procNet{}
	}
	return g.fromScan(pid, scan), n.fromScan(scan)
}
