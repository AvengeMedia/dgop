//go:build linux

package gops

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCgroupUnit(t *testing.T) {
	assert.Equal(t, "app-zen@a1b2.scope", cgroupUnit("0::/user.slice/user-1000.slice/user@1000.service/app.slice/app-zen@a1b2.scope\n"))
	assert.Equal(t, "pipewire.service", cgroupUnit("0::/user.slice/user-1000.slice/user@1000.service/session.slice/pipewire.service\n"))
	assert.Equal(t, "", cgroupUnit("0::/\n"))
	assert.Equal(t, "", cgroupUnit("1:name=systemd:/foo\n"))
}

func TestProcField(t *testing.T) {
	io := "rchar: 10\nread_bytes: 4096\nwrite_bytes: 8192\n"
	assert.Equal(t, uint64(4096), *procField(io, "read_bytes:"))
	assert.Equal(t, uint64(8192), *procField(io, "write_bytes:"))
	assert.Nil(t, procField(io, "cancelled_write_bytes:"))
	assert.Equal(t, uint64(12), *procField("VmSwap:\t      12 kB\n", "VmSwap:"))
}

func TestGetProcessDetailSelf(t *testing.T) {
	d, err := NewGopsUtil().GetProcessDetail(int32(os.Getpid()))
	require.NoError(t, err)
	require.NotNil(t, d.OpenFiles)
	assert.Positive(t, *d.OpenFiles)
	assert.NotNil(t, d.SwapKB)

	_, err = NewGopsUtil().GetProcessDetail(-5)
	assert.ErrorIs(t, err, ErrProcessNotFound)
}
