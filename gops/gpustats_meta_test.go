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
		return []models.GPU{{PciSlot: "0000:99:00.0", PciId: "8086:a780", Driver: "i915"}}, nil
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

func TestGetGPUInfoWithTemp_ReadsTempsOnlyForEnabled(t *testing.T) {
	var read []string
	g := &GopsUtil{
		detectGPUs: func() ([]models.GPU, error) {
			return []models.GPU{{PciId: "1002:164e", PciSlot: "0000:0c:00.0"}, {PciId: "1002:164e", PciSlot: "0000:0d:00.0"}}, nil
		},
		gpuTemp: func(gpu models.GPU) models.GPUTempInfo {
			read = append(read, gpu.PciSlot)
			return models.GPUTempInfo{Temperature: float64(len(read))}
		},
	}

	for name, ids := range map[string][]string{"nil": nil, "empty": {}} {
		read = nil
		info, err := g.GetGPUInfoWithTemp(ids)
		assert.NoError(t, err, name)
		assert.Empty(t, read, name)
		assert.Len(t, info.GPUs, 2, name)
	}

	read = nil
	info, err := g.GetGPUInfoWithTemp([]string{"0000:0d:00.0"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"0000:0d:00.0"}, read, "identical GPUs are told apart by slot")
	assert.Zero(t, info.GPUs[0].Temperature)
	assert.Equal(t, 1.0, info.GPUs[1].Temperature)

	read = nil
	info, _ = g.GetGPUInfoWithTemp([]string{"1002:164e"})
	assert.Equal(t, []string{"0000:0c:00.0", "0000:0d:00.0"}, read, "each GPU reads its own temperature")
	assert.Equal(t, []float64{1, 2}, []float64{info.GPUs[0].Temperature, info.GPUs[1].Temperature})
}
