package gops

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"testing"
)

func TestGetModulesListsNewModules(t *testing.T) {
	mods, err := NewGopsUtil().GetModules()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range []string{"gpu-stats", "sensors", "memory-modules"} {
		if !slices.Contains(mods.Available, m) {
			t.Errorf("module %q not listed", m)
		}
	}
}

func TestGetMetaUnknownModule(t *testing.T) {
	if _, err := NewGopsUtil().GetMeta(context.Background(), []string{"nope"}, MetaParams{}); err == nil {
		t.Fatal("expected error for unknown module")
	}
}

func hasEntries(dir string) bool {
	e, err := os.ReadDir(dir)
	return err == nil && len(e) > 0
}

func TestGetMetaNewModules(t *testing.T) {
	check := func(t *testing.T, modules []string) {
		meta, err := NewGopsUtil().GetMeta(context.Background(), modules, MetaParams{NetRateAllInterfaces: true})
		if err != nil {
			t.Fatal(err)
		}
		if hasEntries("/sys/class/hwmon") && meta.Sensors == nil {
			t.Error("sensors missing")
		}
		if _, err := os.Stat("/run/udev/data/+dmi:id"); err == nil && meta.MemoryModules == nil {
			t.Error("memory-modules missing")
		}
		if hasEntries("/sys/class/drm") && meta.GPUStats == nil {
			t.Error("gpu-stats missing")
		}
	}
	t.Run("explicit", func(t *testing.T) { check(t, []string{"sensors", "memory-modules", "gpu-stats"}) })
	t.Run("all", func(t *testing.T) { check(t, []string{"all"}) })
}

func TestGetMetaKeysBackwardCompatible(t *testing.T) {
	meta, err := NewGopsUtil().GetMeta(context.Background(), []string{"cpu", "memory"}, MetaParams{})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(meta)
	var out map[string]map[string]json.RawMessage
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	golden := map[string][]string{
		"cpu":    {"count", "model", "frequency", "usage", "temperature", "cursor"},
		"memory": {"total", "used", "free", "available"},
	}
	for mod, keys := range golden {
		if out[mod] == nil {
			t.Fatalf("module %s missing", mod)
		}
		for _, k := range keys {
			if _, ok := out[mod][k]; !ok {
				t.Errorf("%s.%s missing", mod, k)
			}
		}
	}
}
