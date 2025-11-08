package gops

import (
	"testing"
	"time"

	"github.com/AvengeMedia/dgop/models"
	"github.com/stretchr/testify/assert"
)

func TestCalculateProcessCPUPercentageWithCursor(t *testing.T) {
	baseTime := time.Now().UnixMilli()

	tests := []struct {
		name           string
		cursor         *models.ProcessCursorData
		currentCPUTime float64
		currentTime    int64
		expected       float64
	}{
		{
			name: "1 second elapsed, 1 second CPU time",
			cursor: &models.ProcessCursorData{
				PID:       1234,
				Ticks:     0.0,
				Timestamp: baseTime,
			},
			currentCPUTime: 1.0,
			currentTime:    baseTime + 1000,
			expected:       100.0,
		},
		{
			name: "1 second elapsed, 0.5 second CPU time",
			cursor: &models.ProcessCursorData{
				PID:       1234,
				Ticks:     0.0,
				Timestamp: baseTime,
			},
			currentCPUTime: 0.5,
			currentTime:    baseTime + 1000,
			expected:       50.0,
		},
		{
			name: "2 seconds elapsed, 0.5 second CPU time",
			cursor: &models.ProcessCursorData{
				PID:       1234,
				Ticks:     0.0,
				Timestamp: baseTime,
			},
			currentCPUTime: 0.5,
			currentTime:    baseTime + 2000,
			expected:       25.0,
		},
		{
			name: "incremental measurement",
			cursor: &models.ProcessCursorData{
				PID:       1234,
				Ticks:     5.0,
				Timestamp: baseTime,
			},
			currentCPUTime: 6.0,
			currentTime:    baseTime + 1000,
			expected:       100.0,
		},
		{
			name: "fractional CPU usage",
			cursor: &models.ProcessCursorData{
				PID:       1234,
				Ticks:     10.0,
				Timestamp: baseTime,
			},
			currentCPUTime: 10.25,
			currentTime:    baseTime + 1000,
			expected:       25.0,
		},
		{
			name: "zero timestamp",
			cursor: &models.ProcessCursorData{
				PID:       1234,
				Ticks:     0.0,
				Timestamp: 0,
			},
			currentCPUTime: 1.0,
			currentTime:    baseTime + 1000,
			expected:       0.0,
		},
		{
			name: "CPU time didn't increase",
			cursor: &models.ProcessCursorData{
				PID:       1234,
				Ticks:     5.0,
				Timestamp: baseTime,
			},
			currentCPUTime: 5.0,
			currentTime:    baseTime + 1000,
			expected:       0.0,
		},
		{
			name: "CPU time decreased (process restarted?)",
			cursor: &models.ProcessCursorData{
				PID:       1234,
				Ticks:     10.0,
				Timestamp: baseTime,
			},
			currentCPUTime: 5.0,
			currentTime:    baseTime + 1000,
			expected:       0.0,
		},
		{
			name: "wall time is zero",
			cursor: &models.ProcessCursorData{
				PID:       1234,
				Ticks:     0.0,
				Timestamp: baseTime,
			},
			currentCPUTime: 1.0,
			currentTime:    baseTime,
			expected:       0.0,
		},
		{
			name: "wall time is negative (clock skew)",
			cursor: &models.ProcessCursorData{
				PID:       1234,
				Ticks:     0.0,
				Timestamp: baseTime,
			},
			currentCPUTime: 1.0,
			currentTime:    baseTime - 1000,
			expected:       0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateProcessCPUPercentageWithCursor(tt.cursor, tt.currentCPUTime, tt.currentTime)
			assert.InDelta(t, tt.expected, result, 0.01)
		})
	}
}

func TestCalculateProcessCPUPercentageRealWorld(t *testing.T) {
	tests := []struct {
		name         string
		cursor       *models.ProcessCursorData
		currentTicks float64
		elapsedMs    int64
		expected     float64
		delta        float64
	}{
		{
			name: "busy process for 5 seconds",
			cursor: &models.ProcessCursorData{
				PID:       9999,
				Ticks:     100.0,
				Timestamp: 1000000,
			},
			currentTicks: 105.0,
			elapsedMs:    5000,
			expected:     100.0,
			delta:        0.1,
		},
		{
			name: "idle process",
			cursor: &models.ProcessCursorData{
				PID:       9999,
				Ticks:     50.0,
				Timestamp: 1000000,
			},
			currentTicks: 50.05,
			elapsedMs:    10000,
			expected:     0.5,
			delta:        0.1,
		},
		{
			name: "moderate activity",
			cursor: &models.ProcessCursorData{
				PID:       9999,
				Ticks:     200.0,
				Timestamp: 1000000,
			},
			currentTicks: 203.0,
			elapsedMs:    10000,
			expected:     30.0,
			delta:        0.1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			currentTime := tt.cursor.Timestamp + tt.elapsedMs
			result := calculateProcessCPUPercentageWithCursor(tt.cursor, tt.currentTicks, currentTime)
			assert.InDelta(t, tt.expected, result, tt.delta)
		})
	}
}

func TestGetPssDirty(t *testing.T) {
	_, err := getPssDirty(999999)
	assert.Error(t, err, "Should error for non-existent PID")
}

func BenchmarkCalculateProcessCPUPercentage(b *testing.B) {
	cursor := &models.ProcessCursorData{
		PID:       1234,
		Ticks:     100.0,
		Timestamp: 1000000,
	}
	currentCPUTime := 105.5
	currentTime := int64(1005000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calculateProcessCPUPercentageWithCursor(cursor, currentCPUTime, currentTime)
	}
}
