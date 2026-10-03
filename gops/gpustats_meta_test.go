package gops

import (
	"context"
	"testing"

	"github.com/AvengeMedia/dgop/models"
	"github.com/stretchr/testify/assert"
)

func TestGetMeta_GPUStatsHonorsAllowList(t *testing.T) {
	origDetect, origFill := detectGPUsFn, fillGPUStatsFn
	t.Cleanup(func() { detectGPUsFn, fillGPUStatsFn = origDetect, origFill })
	detectGPUsFn = func() ([]models.GPU, error) {
		return []models.GPU{{PciId: "10de:2684"}, {PciId: "1002:164e"}}, nil
	}
	fillGPUStatsFn = func(s []models.GPUStats, _ string) string {
		for i := range s {
			assert.Equal(t, "10de:2684", s[i].PciId, "disabled GPU reached fill")
			s[i].FanRpm = new(1)
		}
		return "next"
	}
	params := MetaParams{GPUPciIds: []string{"10de:2684"}}

	for name, modules := range map[string][]string{"gpu-stats": {"gpu-stats"}, "all": {"all"}} {
		meta, err := NewGopsUtil().GetMeta(context.Background(), modules, params)
		assert.NoError(t, err, name)
		if !assert.NotNil(t, meta.GPUStats, name) || !assert.Len(t, meta.GPUStats.GPUs, 2, name) {
			continue
		}
		assert.NotNil(t, meta.GPUStats.GPUs[0].FanRpm, name)
		assert.Nil(t, meta.GPUStats.GPUs[1].FanRpm, name)
	}
}

func TestGetGPUInfoWithTemp_ReadsTempsOnlyForEnabled(t *testing.T) {
	origDetect, origTemp := detectGPUsFn, gpuTempFn
	t.Cleanup(func() { detectGPUsFn, gpuTempFn = origDetect, origTemp })
	detectGPUsFn = func() ([]models.GPU, error) {
		return []models.GPU{{PciId: "10de:2684"}, {PciId: "1002:164e"}}, nil
	}
	var read []string
	gpuTempFn = func(_ *GopsUtil, id string) (*models.GPUTempInfo, error) {
		read = append(read, id)
		return &models.GPUTempInfo{Temperature: 50}, nil
	}

	for name, ids := range map[string][]string{"nil": nil, "empty": {}} {
		read = nil
		info, err := (&GopsUtil{}).GetGPUInfoWithTemp(ids)
		assert.NoError(t, err, name)
		assert.Empty(t, read, name)
		assert.Len(t, info.GPUs, 2, name)
	}

	info, err := (&GopsUtil{}).GetGPUInfoWithTemp([]string{"1002:164e"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"1002:164e"}, read)
	assert.Equal(t, float64(50), info.GPUs[1].Temperature)
}
