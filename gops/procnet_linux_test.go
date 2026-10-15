//go:build linux

package gops

import (
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestIsInternetAddr(t *testing.T) {
	yes := []string{"8.8.8.8", "::ffff:8.8.8.8", "2001:4860::8888"}
	no := []string{"192.168.2.1", "10.0.0.1", "100.84.12.7", "127.0.0.1", "::1", "fe80::1", "fd00::1", "0.0.0.0", "224.0.0.1", "::", "::ffff:10.0.0.1"}
	for _, s := range yes {
		assert.True(t, isInternetAddr(netip.MustParseAddr(s)), s)
	}
	for _, s := range no {
		assert.False(t, isInternetAddr(netip.MustParseAddr(s)), s)
	}
	assert.False(t, isInternetAddr(netip.Addr{}))
}

func TestScanProcFDsSocketsOptIn(t *testing.T) {
	root := t.TempDir()
	fdDir := filepath.Join(root, "7", "fd")
	require.NoError(t, os.MkdirAll(fdDir, 0o755))
	for fd, target := range map[string]string{"3": "socket:[100]", "4": "socket:[bad]", "5": "/dev/null"} {
		require.NoError(t, os.Symlink(target, filepath.Join(fdDir, fd)))
	}
	scan, err := scanProcFDs(root, 7, nil, false)
	require.NoError(t, err)
	assert.Empty(t, scan.SocketInodes)
	scan, err = scanProcFDs(root, 7, nil, true)
	require.NoError(t, err)
	assert.Equal(t, []uint32{100}, scan.SocketInodes)
}

func readNet(r *procNetReader, procRoot string, pid int32) procNet {
	_, n := processExtras{net: r}.readFDs(procRoot, pid)
	return n
}

func TestProcNetReaderRead(t *testing.T) {
	root := t.TempDir()
	mk := func(pid string, inodes ...int) {
		d := filepath.Join(root, pid, "fd")
		require.NoError(t, os.MkdirAll(d, 0o755))
		for i, n := range inodes {
			require.NoError(t, os.Symlink(fmt.Sprintf("socket:[%d]", n), filepath.Join(d, fmt.Sprint(i))))
		}
	}
	mk("1", 1, 2, 3, 4) // tcp internet, udp internet (v4-mapped), tcp LAN, tcp loopback
	mk("2", 3, 4)       // loopback/LAN only
	mk("3", 1000, 1001) // many internet
	require.NoError(t, os.MkdirAll(filepath.Join(root, "4", "fd"), 0o755))

	ap := netip.MustParseAddrPort
	socks := map[uint32]sockDiagEntry{
		1: {Inode: 1, Protocol: unix.IPPROTO_TCP, Remote: ap("[2001:4860::8888]:443"), RxBytes: 5, TxBytes: 6, HasTCPInfo: true},
		2: {Inode: 2, Protocol: unix.IPPROTO_UDP, Remote: ap("[::ffff:8.8.8.8]:53")},
		3: {Inode: 3, Protocol: unix.IPPROTO_TCP, Remote: ap("192.168.2.1:22")},
		4: {Inode: 4, Protocol: unix.IPPROTO_TCP, Remote: ap("127.0.0.1:80")},
	}
	for i := range 40 {
		socks[uint32(2000+i)] = sockDiagEntry{Protocol: unix.IPPROTO_TCP, Remote: ap("1.1.1.1:443")}
	}
	d := filepath.Join(root, "3", "fd")
	for i := range 40 {
		require.NoError(t, os.Symlink(fmt.Sprintf("socket:[%d]", 2000+i), filepath.Join(d, fmt.Sprint("x", i))))
	}
	r := &procNetReader{socks: socks, ok: true}

	got := readNet(r, root, 1)
	assert.True(t, got.Known)
	assert.Equal(t, 2, got.Count)
	assert.ElementsMatch(t, []procRemote{{"tcp", "[2001:4860::8888]:443"}, {"udp", "8.8.8.8:53"}}, got.Remotes)
	assert.Equal(t, []tcpCounters{{1, 5, 6}}, got.TCP, "UDP and non-internet sockets carry no counters")
	assert.False(t, got.RatesUnknown)

	got = readNet(r, root, 2)
	assert.True(t, got.Known, "loopback-only is zero, not unknown")
	assert.Zero(t, got.Count)

	got = readNet(r, root, 3)
	assert.Equal(t, 40, got.Count, "count is exact")
	assert.True(t, got.RatesUnknown, "TCP sockets without tcp_info make the rate unknown, not zero")
	assert.Empty(t, got.TCP)
	assert.Len(t, got.Remotes, maxProcRemotes)

	assert.True(t, readNet(r, root, 4).Known)
	assert.False(t, readNet(r, root, 99).Known, "unreadable fd dir is unknown")
	assert.False(t, readNet(&procNetReader{}, root, 1).Known, "failed dump is unknown")
}

func TestProcNetReaderLive(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	c, err := net.Dial("tcp", ln.Addr().String())
	require.NoError(t, err)
	defer c.Close()
	got := readNet(NewGopsUtil().newProcNetReader(), "/proc", int32(os.Getpid()))
	if !got.Known {
		t.Skip("sock_diag unavailable")
	}
	assert.Zero(t, got.Count)
}

func TestProcNetReaderDedupesSharedSocket(t *testing.T) {
	root := t.TempDir()
	d := filepath.Join(root, "1", "fd")
	require.NoError(t, os.MkdirAll(d, 0o755))
	for _, fd := range []string{"0", "1", "2"} {
		require.NoError(t, os.Symlink("socket:[1]", filepath.Join(d, fd)))
	}
	socks := map[uint32]sockDiagEntry{1: {Inode: 1, Protocol: unix.IPPROTO_TCP, Remote: netip.MustParseAddrPort("8.8.8.8:443"), HasTCPInfo: true}}
	got := readNet(&procNetReader{socks: socks, ok: true}, root, 1)
	assert.Equal(t, 1, got.Count)
	assert.Len(t, got.Remotes, 1)
	assert.Len(t, got.TCP, 1)
}

func TestFromScanSharedWalk(t *testing.T) {
	root := t.TempDir()
	writeDRMClient(t, root, 1, 4, "renderD128", amdgpuEngineClient)
	writeFDLink(t, root, 1, 5, "socket:[1]")
	nodes := map[string]gpuNodeState{"renderD128": gpuNodeRead}
	scan, err := scanProcFDs(root, 1, nodes, true)
	require.NoError(t, err)

	n := &procNetReader{socks: map[uint32]sockDiagEntry{1: {Inode: 1, Protocol: unix.IPPROTO_UDP, Remote: netip.MustParseAddrPort("8.8.8.8:53")}}, ok: true}
	assert.Equal(t, 1, n.fromScan(scan).Count)
	assert.Equal(t, uint64(2<<20), (&procGPUReader{gpuNodeSet: gpuNodeSet{nodes: nodes}}).fromScan(1, scan).VRAMBytes)
	assert.False(t, (&procNetReader{}).fromScan(scan).Known)
}
