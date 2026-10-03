//go:build linux

package gops

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/AvengeMedia/dgop/models"
)

const nvidiaSMIFields = "pci.bus_id,utilization.gpu,memory.used,memory.total,clocks.gr,clocks.max.gr,clocks.mem,clocks.max.mem,temperature.gpu,power.draw,power.limit,fan.speed,pcie.link.gen.current,pcie.link.gen.max,pcie.link.width.current,pcie.link.width.max"

var pcieGenSpeeds = map[int]string{
	1: "2.5 GT/s PCIe",
	2: "5.0 GT/s PCIe",
	3: "8.0 GT/s PCIe",
	4: "16.0 GT/s PCIe",
	5: "32.0 GT/s PCIe",
	6: "64.0 GT/s PCIe",
}

// queryNvidiaSMI returns device stats keyed by BDF, or nil when nvidia-smi is
// missing or every query fails. Each given BDF is queried in its own exec so
// suspended GPUs stay asleep and one failure doesn't drop the others.
func queryNvidiaSMI(bdfs []string) map[string]models.GPUStats {
	path, err := exec.LookPath("nvidia-smi")
	if err != nil {
		return nil
	}
	var result map[string]models.GPUStats
	failed := 0
	for _, bdf := range bdfs {
		out, err := runNvidiaSMI(path, nvidiaSMIID(bdf))
		if err != nil {
			failed++
			continue
		}
		for k, v := range parseNvidiaSMI(string(out)) {
			if result == nil {
				result = make(map[string]models.GPUStats)
			}
			result[k] = v
		}
	}
	if len(bdfs) > 0 && failed == len(bdfs) {
		return queryNvidiaSMIAll(path, bdfs)
	}
	return result
}

// queryNvidiaSMIAll is the fallback for nvidia-smi builds that reject --id: it
// queries every device in one exec and keeps only the requested BDFs.
func queryNvidiaSMIAll(path string, bdfs []string) map[string]models.GPUStats {
	out, err := runNvidiaSMI(path, "")
	if err != nil {
		return nil
	}
	all := parseNvidiaSMI(string(out))
	var result map[string]models.GPUStats
	for _, bdf := range bdfs {
		key := normalizeNvidiaBDF(nvidiaSMIID(bdf))
		if v, ok := all[key]; ok {
			if result == nil {
				result = make(map[string]models.GPUStats)
			}
			result[key] = v
		}
	}
	return result
}

func runNvidiaSMI(path, id string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	args := []string{"--query-gpu=" + nvidiaSMIFields, "--format=csv,noheader,nounits"}
	if id != "" {
		args = append([]string{"--id=" + id}, args...)
	}
	return exec.CommandContext(ctx, path, args...).Output()
}

// nvidiaSMIID turns a sysfs BDF (0000:01:00.0) into the form nvidia-smi prints
// in pci.bus_id: 8-digit domain, uppercase (00000000:01:00.0).
func nvidiaSMIID(bdf string) string {
	bdf = strings.ToUpper(strings.TrimSpace(bdf))
	if i := strings.Index(bdf, ":"); i >= 0 && i < 8 {
		bdf = strings.Repeat("0", 8-i) + bdf
	}
	return bdf
}

func parseNvidiaSMI(text string) map[string]models.GPUStats {
	result := make(map[string]models.GPUStats)
	for _, line := range strings.Split(text, "\n") {
		cols := strings.Split(line, ",")
		if len(cols) != 16 {
			continue
		}
		for i := range cols {
			cols[i] = strings.TrimSpace(cols[i])
		}
		bdf := normalizeNvidiaBDF(cols[0])
		if bdf == "" {
			continue
		}
		result[bdf] = models.GPUStats{
			PciSlot:          bdf,
			Usage:            nvidiaFloat(cols[1]),
			VRAMUsed:         nvidiaBytes(cols[2]),
			VRAMTotal:        nvidiaBytes(cols[3]),
			CoreClock:        nvidiaFloat(cols[4]),
			CoreClockMax:     nvidiaFloat(cols[5]),
			MemClock:         nvidiaFloat(cols[6]),
			MemClockMax:      nvidiaFloat(cols[7]),
			Temperature:      nvidiaFloat(cols[8]),
			Power:            nvidiaFloat(cols[9]),
			PowerLimit:       nvidiaFloat(cols[10]),
			FanPercent:       nvidiaFloat(cols[11]),
			PcieLinkSpeed:    pcieGenSpeeds[nvidiaInt(cols[12])],
			PcieLinkSpeedMax: pcieGenSpeeds[nvidiaInt(cols[13])],
			PcieLinkWidth:    nvidiaInt(cols[14]),
			PcieLinkWidthMax: nvidiaInt(cols[15]),
		}
	}
	return result
}

// normalizeNvidiaBDF turns 00000000:01:00.0 into 0000:01:00.0.
func normalizeNvidiaBDF(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if i := strings.Index(id, ":"); i > 4 {
		id = id[i-4:]
	}
	if strings.Count(id, ":") != 2 {
		return ""
	}
	return id
}

func nvidiaFloat(s string) *float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

func nvidiaInt(s string) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return v
}

func nvidiaBytes(s string) *uint64 {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return nil
	}
	v <<= 20
	return &v
}
