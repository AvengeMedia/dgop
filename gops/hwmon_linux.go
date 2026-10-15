//go:build linux

package gops

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const hwmonRoot = "/sys/class/hwmon"

const (
	sensorTemperature = "temperature"
	sensorFan         = "fan"
	sensorPower       = "power"
	sensorFrequency   = "frequency"
)

// hwmonReading values: temperature °C, fan rpm, power W, frequency MHz.
type hwmonReading struct {
	Kind     string
	Label    string
	Value    float64
	Max      *float64
	Critical *float64
}

type hwmonChip struct {
	Name     string
	Device   string
	Readings []hwmonReading
}

var hwmonInputRe = regexp.MustCompile(`^(temp|fan|power|freq)(\d+)_(input|average)$`)

// i2c reads run in the background and keep their last result, so a held
// bus delays a caller by at most i2cWait.
var (
	i2cMu    sync.Mutex
	i2cCache = map[string]*i2cState{}
)

type i2cState struct {
	readings []hwmonReading
	inflight bool
	done     chan struct{}
}

// readHwmonChips reads every usable chip under an hwmon class root.
func (self *GopsUtil) readHwmonChips(root string) []hwmonChip {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var chips []hwmonChip
	seen := map[string]bool{}
	for _, entry := range entries {
		dir := filepath.Join(root, entry.Name())
		seen[dir] = true
		if chip, ok := self.readHwmonChip(dir); ok {
			chips = append(chips, chip)
		}
	}
	i2cMu.Lock()
	for dir := range i2cCache {
		if !seen[dir] {
			delete(i2cCache, dir)
		}
	}
	i2cMu.Unlock()
	return chips
}

// readHwmonChip is not ok for chips without a name or that must not be read:
// drivetemp, and any chip of a runtime-suspended device.
func (self *GopsUtil) readHwmonChip(dir string) (hwmonChip, bool) {
	name := readTrimmed(filepath.Join(dir, "name"))
	if name == "" || name == "drivetemp" {
		return hwmonChip{}, false
	}
	if readTrimmed(filepath.Join(dir, "device", "power", "runtime_status")) == "suspended" {
		return hwmonChip{}, false
	}
	chip := hwmonChip{Name: name}
	if link, err := os.Readlink(filepath.Join(dir, "device")); err == nil {
		chip.Device = filepath.Base(link)
	}
	if resolved, err := filepath.EvalSymlinks(filepath.Join(dir, "device")); err == nil && strings.Contains(resolved, "/i2c-") {
		chip.Readings = self.readI2CReadings(dir)
	} else {
		chip.Readings = readHwmonReadings(dir)
	}
	return chip, true
}

func (self *GopsUtil) readI2CReadings(dir string) []hwmonReading {
	i2cMu.Lock()
	st := i2cCache[dir]
	if st == nil {
		st = &i2cState{}
		i2cCache[dir] = st
	}
	if !st.inflight {
		st.inflight = true
		done := make(chan struct{})
		st.done = done
		go func() {
			r := self.i2cRead(dir)
			i2cMu.Lock()
			st.readings, st.inflight = r, false
			i2cMu.Unlock()
			close(done)
		}()
	}
	done := st.done
	i2cMu.Unlock()

	select {
	case <-done:
	case <-time.After(self.i2cWait):
	}

	i2cMu.Lock()
	defer i2cMu.Unlock()
	return st.readings
}

func readHwmonReadings(dir string) []hwmonReading {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	type key struct {
		typ string
		n   int
	}
	files := map[key]string{}
	for _, entry := range entries {
		m := hwmonInputRe.FindStringSubmatch(entry.Name())
		if m == nil || (m[3] == "average" && m[1] != "power") {
			continue
		}
		n, _ := strconv.Atoi(m[2])
		k := key{m[1], n}
		// power*_input wins over power*_average
		if _, seen := files[k]; !seen || m[3] == "input" {
			files[k] = entry.Name()
		}
	}

	keys := make([]key, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].typ != keys[j].typ {
			return keys[i].typ < keys[j].typ
		}
		return keys[i].n < keys[j].n
	})

	var out []hwmonReading
	for _, k := range keys {
		prefix := fmt.Sprintf("%s%d", k.typ, k.n)
		raw := readFloat(filepath.Join(dir, files[k]))
		if raw == nil {
			continue
		}
		label := readTrimmed(filepath.Join(dir, prefix+"_label"))
		if label == "" {
			label = prefix
		}
		r := hwmonReading{Label: label}
		switch k.typ {
		case "temp":
			r.Kind, r.Value = sensorTemperature, *raw/1000
			r.Max = readTempLimit(filepath.Join(dir, prefix+"_max"))
			r.Critical = readTempLimit(filepath.Join(dir, prefix+"_crit"))
		case "fan":
			r.Kind, r.Value = sensorFan, *raw
			r.Max = readScaled(filepath.Join(dir, prefix+"_max"), 1)
		case "power":
			r.Kind, r.Value = sensorPower, *raw/1e6
			r.Max = readScaled(filepath.Join(dir, prefix+"_cap"), 1e6)
			if r.Max == nil {
				r.Max = readScaled(filepath.Join(dir, prefix+"_max"), 1e6)
			}
		case "freq":
			r.Kind, r.Value = sensorFrequency, *raw/1e6
		}
		out = append(out, r)
	}
	return out
}

// readTempLimit drops "unset" zeros and sentinel values no real limit has.
func readTempLimit(path string) *float64 {
	v := readScaled(path, 1000)
	if v != nil && *v > 200 {
		return nil
	}
	return v
}
