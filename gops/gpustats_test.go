package gops

import (
	"testing"

	"github.com/AvengeMedia/dgop/models"
	"github.com/stretchr/testify/assert"
)

func TestGPUEnabled(t *testing.T) {
	assert.True(t, gpuEnabled(nil, "10de:2684", "0000:01:00.0"), "not provided enables all")
	assert.False(t, gpuEnabled([]string{}, "10de:2684", "0000:01:00.0"), "empty enables none")
	assert.False(t, gpuEnabled([]string{""}, "10de:2684", "0000:01:00.0"))
	assert.True(t, gpuEnabled([]string{"1002:164e", "10DE:2684"}, "10de:2684", "0000:01:00.0"))
	assert.True(t, gpuEnabled([]string{"0000:01:00.0"}, "10de:2684", "0000:01:00.0"))
	assert.False(t, gpuEnabled([]string{"1002:164e"}, "10de:2684", "0000:01:00.0"))
}

func TestFillEnabled_OnlyEnabledAreFilled(t *testing.T) {
	gpus := []models.GPUStats{{PciSlot: "a"}, {PciSlot: "b"}, {PciSlot: "c"}}
	var seen []string
	next := fillEnabled(gpus, []bool{true, false, true}, "cur", func(s []models.GPUStats, cursor string) string {
		for i := range s {
			seen = append(seen, s[i].PciSlot)
			s[i].FanRpm = new(1)
		}
		assert.Equal(t, "cur", cursor)
		return "next"
	})
	assert.Equal(t, "next", next)
	assert.Equal(t, []string{"a", "c"}, seen)
	assert.NotNil(t, gpus[0].FanRpm)
	assert.Nil(t, gpus[1].FanRpm)
	assert.NotNil(t, gpus[2].FanRpm)

	called := false
	assert.Equal(t, "", fillEnabled(gpus, []bool{false, false, false}, "", func([]models.GPUStats, string) string { called = true; return "x" }))
	assert.False(t, called)
}

func TestStatsFor_DisabledGPUNeverReachesFill(t *testing.T) {
	detected := []models.GPU{
		{PciId: "10de:2684", PciSlot: "0000:01:00.0", FullName: "A"},
		{PciId: "1002:164e", PciSlot: "0000:6c:00.0", FullName: "B"},
	}
	fill := func(s []models.GPUStats, _ string) string {
		for i := range s {
			assert.Equal(t, "10de:2684", s[i].PciId, "disabled GPU reached fill")
			s[i].FanRpm = new(1)
		}
		return "next"
	}
	for name, ids := range map[string][]string{"id": {"10de:2684"}, "slot": {"0000:01:00.0"}} {
		got := statsFor(detected, "", ids, fill)
		assert.Len(t, got.GPUs, 2, name)
		assert.NotNil(t, got.GPUs[0].FanRpm, name)
		assert.Nil(t, got.GPUs[1].FanRpm, name)
		assert.Equal(t, "B", got.GPUs[1].Name, name)
	}

	none := statsFor(detected, "", []string{}, func([]models.GPUStats, string) string {
		t.Fatal("fill called with empty allow-list")
		return ""
	})
	assert.Len(t, none.GPUs, 2)

	all := statsFor(detected, "", nil, func(s []models.GPUStats, _ string) string {
		assert.Len(t, s, 2)
		return ""
	})
	assert.Len(t, all.GPUs, 2)
}

func TestGetGPUStats_AppliesAllowList(t *testing.T) {
	origDetect, origFill := detectGPUsFn, fillGPUStatsFn
	t.Cleanup(func() { detectGPUsFn, fillGPUStatsFn = origDetect, origFill })
	detectGPUsFn = func() ([]models.GPU, error) {
		return []models.GPU{{PciId: "10de:2684"}, {PciId: "1002:164e"}}, nil
	}
	fillGPUStatsFn = func(s []models.GPUStats, _ string) string {
		for i := range s {
			s[i].FanRpm = new(1)
		}
		return "next"
	}

	got, err := (&GopsUtil{}).GetGPUStats("", []string{"1002:164e"})
	assert.NoError(t, err)
	assert.Nil(t, got.GPUs[0].FanRpm)
	assert.NotNil(t, got.GPUs[1].FanRpm)

	got, _ = (&GopsUtil{}).GetGPUStats("", nil)
	assert.NotNil(t, got.GPUs[0].FanRpm)
}
