//go:build linux

package gops_handler_test

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestProcessesE2E_NetLoopbackIsZero(t *testing.T) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW, unix.NETLINK_SOCK_DIAG)
	if err != nil {
		t.Skipf("sock_diag netlink unavailable: %v", err)
	}
	_ = unix.Close(fd)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	pid := float64(os.Getpid())
	own := func(m map[string]any) map[string]any {
		list, _ := m["data"].([]any)
		for _, r := range list {
			if row := r.(map[string]any); row["pid"] == pid {
				return row
			}
		}
		t.Fatalf("own pid %v not in %d rows", pid, len(list))
		return nil
	}
	const path = "/gops/processes?net=true&limit=0"
	start := time.Now()
	first := getJSON(t, path, http.StatusOK)
	cursor, _ := first["cursor"].(string)
	if cursor == "" {
		t.Fatal("no process cursor")
	}
	second := getJSON(t, path+"&cursor="+url.QueryEscape(cursor), http.StatusOK)
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("two net calls took %v, want < 10s", d)
	}
	for i, m := range []map[string]any{first, second} {
		row := own(m)
		if v, ok := row["connections"].(float64); !ok || v != 0 {
			t.Errorf("call %d: connections = %v, want 0", i+1, row["connections"])
		}
		if r, ok := row["remotes"]; ok {
			t.Errorf("call %d: remotes = %v for loopback only", i+1, r)
		}
	}
}
