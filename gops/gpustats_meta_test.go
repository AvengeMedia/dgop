package gops

import (
	"context"
	"testing"

	"github.com/AvengeMedia/dgop/models"
	"github.com/stretchr/testify/assert"
)

func TestGetMeta_GPUStatsHonorsAllowList(t *testing.T) {
	g := NewGopsUtil()
	// A nonexistent i915 slot: no sysfs stats, but enabling it starts an fdinfo cursor.
	g.detectGPUs = func() ([]models.GPU, error) {
		return []models.GPU{{BusId: "0000:99:00.0", PciId: "8086:a780", Driver: "i915"}}, nil
	}

	for _, modules := range [][]string{{"gpu-stats"}, {"all"}} {
		meta, err := g.GetMeta(context.Background(), modules, MetaParams{GPUPciIds: []string{"8086:a780"}})
		assert.NoError(t, err)
		if assert.NotNil(t, meta.GPUStats, modules) {
			assert.NotEmpty(t, meta.GPUStats.Cursor, modules)
		}
		meta, err = g.GetMeta(context.Background(), modules, MetaParams{GPUPciIds: []string{"10de:2684"}})
		assert.NoError(t, err)
		if assert.NotNil(t, meta.GPUStats, modules) {
			assert.Empty(t, meta.GPUStats.Cursor, modules, "a disabled GPU is never read")
			assert.Len(t, meta.GPUStats.GPUs, 1, modules)
		}
	}
}
