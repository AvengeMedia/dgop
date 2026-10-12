//go:build !linux

package gops

import "time"

type gpuCursorClient struct {
	PDev    string               `json:"p"`
	Driver  string               `json:"d,omitempty"`
	ID      uint64               `json:"i"`
	Engines map[string][3]uint64 `json:"e"`
}

type drmClientSet map[string]struct{}

type procGPU struct {
	Readable    bool
	Unmonitored bool
	UsageKnown  bool
	VRAMBytes   uint64
	GTTBytes    uint64
	Clients     drmClientSet
}

func (self procGPU) cursorClients() []gpuCursorClient { return nil }

type procGPUReader struct{}

func newProcGPUReader(_ []string) *procGPUReader { return &procGPUReader{} }

func (self *procGPUReader) read(_ int32) procGPU { return procGPU{} }

func procGPUUsage(_ []gpuCursorClient, _ drmClientSet, _ time.Duration) (all, video float64) {
	return 0, 0
}
