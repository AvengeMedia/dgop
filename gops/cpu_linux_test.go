//go:build linux

package gops

import (
	"os"
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
