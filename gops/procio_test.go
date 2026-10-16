package gops

import (
	"os"
	"runtime"
	"testing"

	"github.com/shirou/gopsutil/v4/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessDiskBytes_OwnProcessKnown(t *testing.T) {
	if runtime.GOOS == "freebsd" {
		t.Skip("freebsd reports no disk bytes")
	}
	p, err := process.NewProcess(int32(os.Getpid()))
	require.NoError(t, err)

	_, _, ok := newProcReader().diskBytes(p)
	assert.True(t, ok)
}

func TestProcessDiskBytes_NonexistentPIDUnknown(t *testing.T) {
	p := &process.Process{Pid: 1 << 30}

	r, w, ok := newProcReader().diskBytes(p)
	assert.False(t, ok)
	assert.Zero(t, r)
	assert.Zero(t, w)
}

func TestProcessDiskBytes_UnreadableIsUnknownNotZero(t *testing.T) {
	if runtime.GOOS != "linux" || os.Geteuid() == 0 {
		t.Skip("needs a non-root user on linux")
	}
	if _, err := os.ReadFile("/proc/1/io"); err == nil {
		t.Skip("/proc/1/io is readable")
	}

	_, _, ok := newProcReader().diskBytes(&process.Process{Pid: 1})
	assert.False(t, ok)
}
