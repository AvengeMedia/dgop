//go:build linux

package gops

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
)

// queryNvidiaPIDs returns VRAM bytes per PID from `nvidia-smi -q -d PIDS`,
// which unlike --query-compute-apps also lists graphics processes. It
// returns nil when nvidia-smi is missing or fails.
func queryNvidiaPIDs() map[int32]uint64 {
	path, err := exec.LookPath("nvidia-smi")
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), nvidiaSmiTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "-q", "-d", "PIDS").Output()
	if err != nil {
		return nil
	}
	return parseNvidiaPIDs(string(out))
}

// parseNvidiaPIDs sums "Used GPU Memory" per "Process ID", across GPUs and
// process types. A listed process whose memory is not a number (N/A, Not
// available) is kept with 0.
func parseNvidiaPIDs(text string) map[int32]uint64 {
	result := map[int32]uint64{}
	var pid int32
	for line := range strings.SplitSeq(text, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "Process ID":
			n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 32)
			if err != nil || n <= 0 {
				n = 0
			}
			pid = int32(n)
		case "Used GPU Memory":
			if pid == 0 {
				continue
			}
			n, _ := parseDRMValue(value)
			result[pid] += n
		}
	}
	return result
}
