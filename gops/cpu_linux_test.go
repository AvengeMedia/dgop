//go:build linux

package gops

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadThermalTemp(t *testing.T) {
	_, _, err := readThermalTemp("/nonexistent/path", "thermal_zone0")
	assert.Error(t, err, "Should error on nonexistent path")
}

func TestReadThermalType(t *testing.T) {
	_, err := readThermalType("/nonexistent/path", "thermal_zone0")
	assert.Error(t, err, "Should error on nonexistent path")
}

func TestGetMaxACPITZTemperature(t *testing.T) {
	result := getMaxACPITZTemperature("/nonexistent", []os.DirEntry{}, 20, 100, true)
	assert.Equal(t, float64(0), result, "Should return 0 for empty entries")
}

func writeTemp(t *testing.T, dir, name, value string) {
	t.Helper()
	assert.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(value), 0o644))
}

func TestReadCoretempInputPackage(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "temp1_input", "45000\n")
	writeTemp(t, dir, "temp2_input", "73000\n")

	temp, path, ok := readCoretempInput(dir)
	assert.True(t, ok)
	assert.Equal(t, 45.0, temp, "Should prefer the package sensor (temp1_input)")
	assert.Equal(t, filepath.Join(dir, "temp1_input"), path)
}

func TestReadCoretempInputPerCoreFallback(t *testing.T) {
	// Older CPUs (e.g. Nehalem i7 920XM) expose no package sensor, so
	// temp1_input is absent and only per-core inputs exist.
	dir := t.TempDir()
	writeTemp(t, dir, "temp2_input", "72000\n")
	writeTemp(t, dir, "temp3_input", "76500\n")
	writeTemp(t, dir, "temp4_input", "74000\n")

	temp, path, ok := readCoretempInput(dir)
	assert.True(t, ok)
	assert.Equal(t, 76.5, temp, "Should use the hottest core when no package sensor exists")
	assert.Equal(t, filepath.Join(dir, "temp3_input"), path)
}

func TestReadCoretempInputMissing(t *testing.T) {
	_, _, ok := readCoretempInput(t.TempDir())
	assert.False(t, ok, "Should report no reading when no temp inputs exist")
}
