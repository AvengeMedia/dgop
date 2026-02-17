package gops

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    uint64
		expected string
	}{
		{"zero bytes", 0, "0B"},
		{"single byte", 1, "1B"},
		{"bytes only", 512, "512B"},
		{"exact KB", 1024, "1.0K"},
		{"kilobytes", 1536, "1.5K"},
		{"exact MB", 1048576, "1.0M"},
		{"megabytes", 5242880, "5.0M"},
		{"fractional MB", 1572864, "1.5M"},
		{"exact GB", 1073741824, "1.0G"},
		{"gigabytes", 5368709120, "5.0G"},
		{"exact TB", 1099511627776, "1.0T"},
		{"terabytes", 2199023255552, "2.0T"},
		{"exact PB", 1125899906842624, "1.0P"},
		{"petabytes", 2251799813685248, "2.0P"},
		{"exact EB", 1152921504606846976, "1.0E"},
		{"large value", 18446744073709551615, "16.0E"}, // Max uint64
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatBytesRounding(t *testing.T) {
	tests := []struct {
		bytes    uint64
		contains string
	}{
		{1024 * 1024 * 100, "100.0M"},
		{1024 * 1024 * 1024 * 5, "5.0G"},
		{1024 * 1024 * 500, "500.0M"},
		{1024 * 1024 * 1024 * 10, "10.0G"},
	}

	for _, tt := range tests {
		t.Run(tt.contains, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			assert.Equal(t, tt.contains, result)
		})
	}
}
