//go:build linux

package gops

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/AvengeMedia/dgop/models"
)

// GetProcessDetail reads the on-demand fields of one process. Fields that are
// unreadable (another user's process, an exited PID) stay unset.
func (self *GopsUtil) GetProcessDetail(pid int32) (*models.ProcessDetail, error) {
	dir := fmt.Sprintf("/proc/%d", pid)
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("process %d not found", pid)
	}
	d := &models.ProcessDetail{PID: pid}
	if data, err := os.ReadFile(dir + "/io"); err == nil {
		d.ReadTotal = procField(string(data), "read_bytes:")
		d.WriteTotal = procField(string(data), "write_bytes:")
	}
	if data, err := os.ReadFile(dir + "/status"); err == nil {
		d.SwapKB = procField(string(data), "VmSwap:")
	}
	if fds, err := os.ReadDir(dir + "/fd"); err == nil {
		d.OpenFiles = new(len(fds))
	}
	if data, err := os.ReadFile(dir + "/stat"); err == nil {
		d.Terminal = statTTY(string(data)) != 0
	}
	if data, err := os.ReadFile(dir + "/cgroup"); err == nil {
		d.Unit = cgroupUnit(string(data))
	}
	return d, nil
}

// procField parses the numeric value after key in a "key value" /proc file.
func procField(text, key string) *uint64 {
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, key) {
			continue
		}
		fields := strings.Fields(line[len(key):])
		if len(fields) == 0 {
			return nil
		}
		v, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return nil
		}
		return new(v)
	}
	return nil
}

// cgroupUnit returns the innermost .service or .scope of the cgroup v2 path
// ("0::/user.slice/.../app.slice/app-foo@1.scope" -> "app-foo@1.scope").
func cgroupUnit(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "0::") {
			continue
		}
		parts := strings.Split(strings.TrimPrefix(line, "0::"), "/")
		for i := len(parts) - 1; i >= 0; i-- {
			if strings.HasSuffix(parts[i], ".service") || strings.HasSuffix(parts[i], ".scope") {
				return parts[i]
			}
		}
	}
	return ""
}

// statTTY returns tty_nr (field 7) of a /proc/<pid>/stat line; 0 means no controlling terminal.
func statTTY(text string) int {
	i := strings.LastIndexByte(text, ')')
	if i < 0 {
		return 0
	}
	fields := strings.Fields(text[i+1:])
	if len(fields) < 5 {
		return 0
	}
	n, _ := strconv.Atoi(fields[4])
	return n
}
