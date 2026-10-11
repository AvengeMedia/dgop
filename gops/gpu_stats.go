package gops

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/AvengeMedia/dgop/models"
)

type gpuSample struct {
	State       string
	Temperature float64
	Hwmon       string
	Usage       *float64
	MemoryUsed  *uint64
	MemoryTotal *uint64
	Power       *float64
	CoreClock   *float64
	MemoryClock *float64
}

func (s gpuSample) applyTo(gpu *models.GPU) {
	gpu.State = s.State
	gpu.Temperature = s.Temperature
	gpu.Hwmon = s.Hwmon
	gpu.Usage = s.Usage
	gpu.MemoryUsed = s.MemoryUsed
	gpu.MemoryTotal = s.MemoryTotal
	gpu.Power = s.Power
	gpu.CoreClock = s.CoreClock
	gpu.MemoryClock = s.MemoryClock
}

// gpuCursor carries monotonic counters (idle residency, energy) between samples,
// keyed by bus id and then by counter name.
type gpuCursor struct {
	Timestamp int64                        `json:"timestamp"`
	Counters  map[string]map[string]uint64 `json:"counters,omitempty"`
}

func newGPUCursor(now time.Time) gpuCursor {
	return gpuCursor{Timestamp: now.UnixNano(), Counters: map[string]map[string]uint64{}}
}

func encodeGPUCursor(c gpuCursor) string {
	if len(c.Counters) == 0 {
		return ""
	}
	data, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeGPUCursor(s string) gpuCursor {
	var c gpuCursor
	if s == "" {
		return c
	}
	data, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return gpuCursor{}
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return gpuCursor{}
	}
	return c
}

// gpuRequested matches an id given as vendor:device PCI ID or as bus address.
func gpuRequested(gpu models.GPU, ids []string) bool {
	for _, id := range ids {
		id = strings.ToLower(strings.TrimSpace(id))
		if id == "" {
			continue
		}
		if id == gpu.PciId || (gpu.BusId != "" && normalizeBusId(id) == gpu.BusId) {
			return true
		}
	}
	return false
}

// normalizeBusId turns nvidia-smi's 8 digit PCI domain ("00000000:01:00.0")
// into the sysfs form ("0000:01:00.0").
func normalizeBusId(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	domain, rest, ok := strings.Cut(id, ":")
	if !ok || !strings.Contains(rest, ":") {
		return id
	}
	if len(domain) > 4 {
		domain = domain[len(domain)-4:]
	}
	return domain + ":" + rest
}
