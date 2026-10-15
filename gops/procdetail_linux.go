//go:build linux

package gops

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"github.com/AvengeMedia/dgop/models"
)

// GetProcessDetail reads the on-demand fields of one process. Fields that are
// unreadable (another user's process, an exited PID) stay unset.
func (self *GopsUtil) GetProcessDetail(pid int32) (*models.ProcessDetail, error) {
	dir := fmt.Sprintf("/proc/%d", pid)
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("process %d: %w", pid, ErrProcessNotFound)
	} else if err != nil {
		return nil, err
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
		st, _ := parseProcStat(string(data))
		d.Terminal = st.TTY != 0
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
