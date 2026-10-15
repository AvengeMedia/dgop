package gops_handler_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// Cursor written before the process extras existed: pids 1 and 2 only.
const legacyProcCursor = "H4sIAAAAAAAC_6tWKlGyMjQ3QAAdpYLMFCWraEMdo1gdpeSCUhAbJG4EJIAiKUAN0QY6prG1APp9R3w8AAAA"

func getJSON(t *testing.T, path string, want int) map[string]any {
	t.Helper()
	resp := newAPI(t).Get(path)
	if resp.Code != want {
		t.Fatalf("GET %s: status %d, want %d: %s", path, resp.Code, want, resp.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return body
}

func sub(t *testing.T, m map[string]any, key string) map[string]any {
	t.Helper()
	v, ok := m[key].(map[string]any)
	if !ok {
		t.Fatalf("missing object %q in %v", key, m)
	}
	return v
}

func cursorOf(t *testing.T, m map[string]any, key string) string {
	t.Helper()
	s, _ := sub(t, m, key)["cursor"].(string)
	if s == "" {
		t.Fatalf("%s has no cursor", key)
	}
	return s
}

func asMap(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("want object, got %T: %v", v, v)
	}
	return m
}

func asList(t *testing.T, v any) []any {
	t.Helper()
	l, ok := v.([]any)
	if !ok {
		t.Fatalf("want array, got %T: %v", v, v)
	}
	return l
}

func requireSource(t *testing.T, path string) {
	t.Helper()
	if b, err := os.ReadFile(path); err != nil || len(b) == 0 {
		t.Skipf("%s unavailable: %v", path, err)
	}
}

func TestMetaE2E_PerformanceRail(t *testing.T) {
	requireSource(t, "/proc/diskstats")
	const mods = "modules=cpu,memory,disk-rate,net-rate,gpu-stats,sensors,memory-modules"
	_, dmiErr := os.Stat("/run/udev/data/+dmi:id")

	first := getJSON(t, "/gops/meta?"+mods, http.StatusOK)
	if _, ok := sub(t, first, "cpu")["usage"]; !ok {
		t.Error("cpu.usage missing")
	}
	if sub(t, first, "memory")["total"] == nil {
		t.Error("memory.total missing")
	}
	if _, ok := sub(t, first, "sensors")["sensors"]; !ok {
		t.Error("sensors module returned no sensors key")
	}
	if dmiErr == nil {
		if _, ok := first["memorymodules"]; !ok {
			t.Error("memorymodules missing")
		}
	}
	disks, _ := sub(t, first, "diskrate")["disks"].([]any)
	if len(disks) == 0 {
		t.Skip("no disks reported by /proc/diskstats")
	}
	for _, d := range disks {
		if asMap(t, d)["busy"] != nil {
			t.Fatal("busy present without a prior sample")
		}
	}

	q := url.Values{}
	q.Set("cpu_cursor", cursorOf(t, first, "cpu"))
	q.Set("disk_rate_cursor", cursorOf(t, first, "diskrate"))
	gpuCursor := ""
	if g, ok := first["gpustats"].(map[string]any); ok {
		gpuCursor, _ = g["cursor"].(string)
	}
	q.Set("gpu_stats_cursor", gpuCursor)
	time.Sleep(100 * time.Millisecond)
	second := getJSON(t, "/gops/meta?"+mods+"&"+q.Encode(), http.StatusOK)

	if _, ok := sub(t, second, "cpu")["usage"]; !ok {
		t.Error("cpu.usage missing on second call")
	}
	if cursorOf(t, second, "cpu") == q.Get("cpu_cursor") {
		t.Error("cpu cursor did not advance")
	}

	if sub(t, second, "memory")["total"] == nil {
		t.Error("memory.total missing on second call")
	}
	var withBusy int
	for _, d := range asList(t, sub(t, second, "diskrate")["disks"]) {
		row := asMap(t, d)
		if row["busy"] != nil && row["responseTime"] != nil {
			withBusy++
		}
	}
	if withBusy == 0 {
		t.Error("no disk row gained busy/responseTime from the disk_rate_cursor")
	}
	if gpuCursor != "" {
		// The cursor came from a real fdinfo-capable GPU: it must round-trip.
		if g := sub(t, second, "gpustats"); g["cursor"] == nil || g["cursor"] == "" {
			t.Error("gpu stats cursor not re-emitted")
		}
	}
}

func TestMetaE2E_CPUCursorReachesCollector(t *testing.T) {
	cpu := sub(t, getJSON(t, "/gops/meta?modules=cpu", http.StatusOK), "cpu")
	// A hand-made cursor reporting 0.5s less user time over the last second
	// must yield usage, which only happens if cpu_cursor reaches the collector.
	total := asList(t, cpu["total"])
	user, ok := total[0].(float64)
	if !ok {
		t.Fatalf("cpu.total[0] is %T", total[0])
	}
	total[0] = user - 0.5
	raw, _ := json.Marshal(map[string]any{"total": total, "cores": cpu["cores"], "timestamp": time.Now().Add(-time.Second).UnixMilli()})
	crafted := getJSON(t, "/gops/meta?modules=cpu&cpu_cursor="+base64.RawURLEncoding.EncodeToString(raw), http.StatusOK)
	if u, _ := sub(t, crafted, "cpu")["usage"].(float64); u <= 0 {
		t.Errorf("cpu usage %v from a crafted cursor, want > 0", u)
	}
}

func TestMetaE2E_GPUStatsCursorParamAccepted(t *testing.T) {
	body := getJSON(t, "/gops/meta?modules=gpu-stats,sensors&gpu_stats_cursor=not-a-cursor", http.StatusOK)
	if _, ok := body["sensors"]; !ok {
		t.Error("other modules dropped when the gpu cursor is garbage")
	}
}

func TestMetaE2E_ProcessJourney(t *testing.T) {
	requireSource(t, "/proc/self/io")
	const path = "/gops/meta?modules=processes,process-gpu,process-io&limit=0&disable_proc_cpu=true"
	pid := float64(os.Getpid())

	rows := func(m map[string]any) map[string]any {
		list, _ := m["processes"].([]any)
		for _, r := range list {
			if row := asMap(t, r); row["pid"] == pid {
				return row
			}
		}
		t.Fatalf("own pid %v not in %d rows", pid, len(list))
		return nil
	}

	first := getJSON(t, path, http.StatusOK)
	for _, k := range []string{"diskRead", "diskWrite", "gpu"} {
		if rows(first)[k] == nil {
			t.Errorf("first call lacks %q: the baseline pass runs with process CPU off too", k)
		}
	}
	cursor, _ := first["cursor"].(string)
	if cursor == "" {
		t.Fatal("no process cursor")
	}
	time.Sleep(100 * time.Millisecond)
	second := getJSON(t, path+"&proc_cursor="+url.QueryEscape(cursor), http.StatusOK)
	own := rows(second)
	for _, k := range []string{"diskRead", "diskWrite", "gpu"} {
		if own[k] == nil {
			t.Errorf("own process lacks %q: %v", k, own)
		}
	}
}

func TestMetaE2E_LegacyProcessCursor(t *testing.T) {
	body := getJSON(t, "/gops/meta?modules=processes&limit=0&proc_cursor="+legacyProcCursor, http.StatusOK)
	list, _ := body["processes"].([]any)
	if len(list) == 0 {
		t.Fatal("no process rows")
	}
	for _, r := range list {
		row := asMap(t, r)
		for _, k := range []string{"diskRead", "diskWrite", "gpu", "vramKB"} {
			if _, ok := row[k]; ok {
				t.Fatalf("row %v has %q without its module", row["pid"], k)
			}
		}
	}
	if body["cursor"] == nil {
		t.Error("no fresh cursor returned")
	}
}

func TestMetaE2E_UnknownModule(t *testing.T) {
	getJSON(t, "/gops/meta?modules=processes,bogus", http.StatusBadRequest)
}

func TestMetaE2E_ModulesListsNewOnes(t *testing.T) {
	body := getJSON(t, "/gops/modules", http.StatusOK)
	got := map[string]bool{}
	for _, m := range asList(t, body["available"]) {
		name, ok := m.(string)
		if !ok {
			t.Fatalf("module name %v is %T", m, m)
		}
		got[name] = true
	}
	for _, m := range strings.Fields("gpu-stats sensors memory-modules process-gpu process-io") {
		if !got[m] {
			t.Errorf("module %q not listed", m)
		}
	}
}
