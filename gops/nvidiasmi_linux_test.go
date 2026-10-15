//go:build linux

package gops

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const nvidiaSMIFixture = `00000000:01:00.0, 37, 1024, 8192, 1815, 2100, 7000, 7001, 64, 120.55, 200.00, 45, 4, 4, 16, 16, 12, 30
00000000:0C:00.0, 0, 2, 24576, 210, 2520, 405, 10501, 38, [N/A], 450.00, [Not Supported], 1, 4, 8, 16, [N/A], 0
`

func TestParseNvidiaSMI(t *testing.T) {
	got := parseNvidiaSMI(nvidiaSMIFixture)
	require.Len(t, got, 2)

	a := got["0000:01:00.0"]
	assert.Equal(t, "0000:01:00.0", a.PciSlot)
	assert.Equal(t, 37.0, *a.Usage)
	assert.Equal(t, uint64(1024<<20), *a.VRAMUsed)
	assert.Equal(t, uint64(8192<<20), *a.VRAMTotal)
	assert.Equal(t, 1815.0, *a.CoreClock)
	assert.Equal(t, 2100.0, *a.CoreClockMax)
	assert.Equal(t, 7000.0, *a.MemClock)
	assert.Equal(t, 64.0, *a.Temperature)
	assert.Equal(t, 120.55, *a.Power)
	assert.Equal(t, 200.0, *a.PowerLimit)
	assert.Equal(t, 45.0, *a.FanPercent)
	assert.Equal(t, "16.0 GT/s PCIe", a.PcieLinkSpeed)
	assert.Equal(t, "16.0 GT/s PCIe", a.PcieLinkSpeedMax)
	assert.Equal(t, 16, a.PcieLinkWidth)
	assert.Equal(t, 12.0, *a.EncoderUsage)
	assert.Equal(t, 30.0, *a.DecoderUsage)

	b := got["0000:0c:00.0"]
	assert.Nil(t, b.Power)
	assert.Nil(t, b.FanPercent)
	assert.Equal(t, 450.0, *b.PowerLimit)
	assert.Equal(t, 0.0, *b.Usage)
	assert.Equal(t, "2.5 GT/s PCIe", b.PcieLinkSpeed)
	assert.Equal(t, 8, b.PcieLinkWidth)
	assert.Nil(t, b.EncoderUsage)
	assert.Equal(t, 0.0, *b.DecoderUsage)
}

func TestParseNvidiaSMIMalformed(t *testing.T) {
	assert.Empty(t, parseNvidiaSMI(""))
	assert.Empty(t, parseNvidiaSMI("\n\n"))
	assert.Empty(t, parseNvidiaSMI("00000000:01:00.0, 37, 1024\n"))
	assert.Empty(t, parseNvidiaSMI("garbage, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17\n"))
	assert.Len(t, parseNvidiaSMI("bad\n"+nvidiaSMIFixture), 2)
}

func TestQueryNvidiaSMIAbsent(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	assert.Nil(t, queryNvidiaSMI([]string{"0000:01:00.0"}))
}

func TestNvidiaSMIID(t *testing.T) {
	assert.Equal(t, "00000000:01:00.0", nvidiaSMIID("0000:01:00.0"))
	assert.Equal(t, "00000000:0C:00.0", nvidiaSMIID("0000:0c:00.0"))
	assert.Equal(t, "00000000:0C:00.0", nvidiaSMIID("00000000:0c:00.0"))
	assert.Equal(t, "00000001:0C:00.0", nvidiaSMIID("0001:0c:00.0"))
}

func TestQueryNvidiaSMIPerDeviceExec(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"--id=00000000:01:00.0) echo '00000000:01:00.0, 37, 1024, 8192, 1815, 2100, 7000, 7001, 64, 120.55, 200.00, 45, 4, 4, 16, 16, 12, 30';;\n" +
		"--id=00000000:0C:00.0) echo '00000000:0C:00.0, 0, 2, 24576, 210, 2520, 405, 10501, 38, 10, 450.00, 30, 1, 4, 8, 16, 0, 0';;\n" +
		"*) exit 1;;\nesac\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "nvidia-smi"), []byte(script), 0o755))
	t.Setenv("PATH", dir)

	got := queryNvidiaSMI([]string{"0000:01:00.0", "0000:09:00.0", "0000:0c:00.0"})
	require.Len(t, got, 2)
	assert.Contains(t, got, "0000:01:00.0")
	assert.Contains(t, got, "0000:0c:00.0")

	assert.Nil(t, queryNvidiaSMI(nil))
	assert.Nil(t, queryNvidiaSMI([]string{"0000:09:00.0"}))
}

func TestQueryNvidiaSMIRetriesWithoutVideoFields(t *testing.T) {
	t.Cleanup(func() { nvidiaSMINoVideo.Store(false) })
	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		"case \"$*\" in *utilization.encoder*) echo 'Field \"utilization.encoder\" is not a valid field to query.'; exit 2;; esac\n" +
		"echo '00000000:01:00.0, 37, 1024, 8192, 1815, 2100, 7000, 7001, 64, 120.55, 200.00, 45, 4, 4, 16, 16'\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "nvidia-smi"), []byte(script), 0o755))
	t.Setenv("PATH", dir)

	got := queryNvidiaSMI([]string{"0000:01:00.0"})
	require.Contains(t, got, "0000:01:00.0")
	assert.Equal(t, 37.0, *got["0000:01:00.0"].Usage)
	assert.Nil(t, got["0000:01:00.0"].EncoderUsage)
	assert.True(t, nvidiaSMINoVideo.Load())
}

const nvidiaPIDsFixture = `
GPU 00000000:01:00.0
    Processes
        Process ID                        : 1234
            Type                          : G
            Name                          : /usr/lib/Xorg
            Used GPU Memory               : 150 MiB
        Process ID                        : 1234
            Type                          : C
            Name                          : /usr/lib/Xorg
            Used GPU Memory               : 50 MiB
        Process ID                        : 5678
            Type                          : C+G
            Name                          : game
            Used GPU Memory               : Not available
        Process ID                        : 9
            Type                          : G
            Name                          : other
            Used GPU Memory               : 1 GiB
GPU 00000000:02:00.0
    Processes                             : None
`

func TestParseNvidiaPIDs(t *testing.T) {
	got := parseNvidiaPIDs(nvidiaPIDsFixture)
	assert.Equal(t, map[int32]uint64{1234: 200 << 20, 5678: 0, 9: 1 << 30}, got)
	assert.Empty(t, parseNvidiaPIDs("GPU 0\n    Processes : None\n"))
}
