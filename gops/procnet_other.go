//go:build !linux

package gops

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
	Known        bool
	Count        int
	Remotes      []procRemote
	TCP          []tcpCounters
	RatesUnknown bool
}

type procNetReader struct{ dumpedAt int64 }

func newProcNetReader() *procNetReader { return &procNetReader{} }

func (self *procNetReader) read(_ int32) procNet { return procNet{} }

func readGPUAndNet(g *procGPUReader, n *procNetReader, pid int32) (procGPU, procNet) {
	return g.read(pid), n.read(pid)
}
