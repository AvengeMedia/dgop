//go:build linux

package gops

import (
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	mk("1", 1, 2, 3, 4) // tcp internet, udp internet (v4-mapped), tcp LAN, udp link-local
	mk("2", 3, 4, 5)    // local only; 5 is loopback, absent from the reader
	mk("3", 1000, 1001) // many internet
	require.NoError(t, os.MkdirAll(filepath.Join(root, "4", "fd"), 0o755))

	ap := netip.MustParseAddrPort
	socks := map[uint32]sockDiagEntry{
		1: {Inode: 1, Cookie: 11, Protocol: unix.IPPROTO_TCP, Remote: ap("[2001:4860::8888]:443"), RxBytes: 5, TxBytes: 6, HasTCPInfo: true},
		2: {Inode: 2, Cookie: 12, Protocol: unix.IPPROTO_UDP, Remote: ap("[::ffff:8.8.8.8]:53")},
		3: {Inode: 3, Cookie: 13, Protocol: unix.IPPROTO_TCP, Remote: ap("192.168.2.1:22"), RxBytes: 7, TxBytes: 8, HasTCPInfo: true},
		4: {Inode: 4, Cookie: 14, Protocol: unix.IPPROTO_UDP, Remote: ap("[fe80::1%eth0]:5353")},
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
	assert.Equal(t, 2, got.LocalCount)
	assert.ElementsMatch(t, []procRemote{{"tcp", "[2001:4860::8888]:443", false}, {"udp", "8.8.8.8:53", false}, {"tcp", "192.168.2.1:22", true}, {"udp", "[fe80::1%eth0]:5353", true}}, got.Remotes)
	assert.ElementsMatch(t, []tcpCounters{{11, 5, 6}, {13, 7, 8}}, got.TCP, "LAN TCP counts toward rates; UDP carries no counters")
	assert.True(t, got.UDP)
	assert.False(t, got.RatesUnknown)

	got = readNet(r, root, 2)
	assert.ElementsMatch(t, []procRemote{{"tcp", "192.168.2.1:22", true}, {"udp", "[fe80::1%eth0]:5353", true}}, got.Remotes)
	got.Remotes = nil
	assert.Equal(t, procNet{Known: true, LocalCount: 2, TCP: []tcpCounters{{13, 7, 8}}, UDP: true}, got, "loopback is left out")

	got = readNet(r, root, 3)
	assert.Equal(t, 40, got.Count, "count is exact")
	assert.True(t, got.RatesUnknown, "TCP sockets without tcp_info make the rate unknown, not zero")
	assert.Empty(t, got.TCP)
	assert.Len(t, got.Remotes, maxProcRemotes)

	assert.True(t, readNet(r, root, 4).Known)
	assert.False(t, readNet(r, root, 99).Known, "unreadable fd dir is unknown")
	assert.False(t, readNet(&procNetReader{}, root, 1).Known, "failed dump is unknown")
}

func TestRemoteClassification(t *testing.T) {
	for addr, want := range map[string][2]bool{ // internet, local
		"8.8.8.8": {true, false}, "2001:4860::8888": {true, false}, "::ffff:1.1.1.1": {true, false},
		"192.168.2.1": {false, true}, "10.0.0.1": {false, true}, "172.16.0.1": {false, true}, "100.84.12.7": {false, true},
		"fd7a:115c:a1e0::1": {false, true}, "fe80::1": {false, true}, "169.254.1.1": {false, true}, "::ffff:10.0.0.1": {false, true},
		"127.0.0.1": {false, false}, "::1": {false, false}, "0.0.0.0": {false, false}, "224.0.0.1": {false, false}, "255.255.255.255": {false, false},
	} {
		a := netip.MustParseAddr(addr)
		assert.Equal(t, want, [2]bool{isInternetAddr(a), isLocalAddr(a)}, addr)
	}
}

func TestProcNetReaderOtherNamespace(t *testing.T) {
	root := t.TempDir()
	for pid, ns := range map[int]string{1: "net:[4026531840]", 2: "net:[4026532999]", 3: ""} {
		writeFDLink(t, root, pid, 3, "socket:[1]")
		if ns != "" {
			d := filepath.Join(root, itoa(pid), "ns")
			require.NoError(t, os.MkdirAll(d, 0o755))
			require.NoError(t, os.Symlink(ns, filepath.Join(d, "net")))
		}
	}
	socks := map[uint32]sockDiagEntry{1: {Inode: 1, Protocol: unix.IPPROTO_TCP, Remote: netip.MustParseAddrPort("8.8.8.8:443"), HasTCPInfo: true}}
	r := &procNetReader{socks: socks, ok: true, netNS: "net:[4026531840]"}
	assert.Equal(t, 1, readNet(r, root, 1).Count)
	assert.Equal(t, procNet{}, readNet(r, root, 2), "another namespace's sockets aren't in our dump")
	assert.Equal(t, 1, readNet(r, root, 3).Count, "an unreadable ns link counts as ours")

	devs := func(pid int, ifaces ...string) {
		d := filepath.Join(root, itoa(pid), "net")
		require.NoError(t, os.MkdirAll(d, 0o755))
		data := "Inter-|   Receive\n face |bytes\n"
		for _, i := range ifaces {
			data += "  " + i + ": 0 0 0\n"
		}
		require.NoError(t, os.WriteFile(filepath.Join(d, "dev"), []byte(data), 0o644))
	}
	devs(2, "lo", "eth0")
	r = &procNetReader{socks: socks, ok: true, netNS: "net:[4026531840]"}
	assert.Equal(t, procNet{}, readNet(r, root, 2), "a container with its own interfaces stays unknown")
	devs(2, "lo")
	assert.Equal(t, procNet{}, readNet(r, root, 2), "cached per namespace for the sample")
	r = &procNetReader{socks: socks, ok: true, netNS: "net:[4026531840]"}
	assert.Equal(t, 1, readNet(r, root, 2).Count, "a loopback-only sandbox only holds counted sockets passed in from our namespace")
}

func TestOwnNetNSMatchesScan(t *testing.T) {
	require.NotEmpty(t, ownNetNS())
	scan, err := scanProcFDs("/proc", int32(os.Getpid()), nil, true)
	require.NoError(t, err)
	assert.Equal(t, ownNetNS(), scan.NetNS)
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
	socks := map[uint32]sockDiagEntry{1: {Inode: 1, Cookie: 1, Protocol: unix.IPPROTO_TCP, Remote: netip.MustParseAddrPort("8.8.8.8:443"), HasTCPInfo: true}}
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

func TestSockTails(t *testing.T) {
	var tails sockTails
	var emit func(sockDiagEntry)
	calls, stops := 0, 0
	listen := func(e func(sockDiagEntry)) (func(), error) {
		calls++
		emit = e
		return func() { stops++ }, nil
	}
	tails.start(listen)
	tails.start(listen)
	assert.Equal(t, 1, calls, "one listener per GopsUtil")

	internet := netip.MustParseAddrPort("8.8.8.8:443")
	emit(sockDiagEntry{Cookie: 1, Remote: internet, RxBytes: 10, TxBytes: 20, HasTCPInfo: true})
	emit(sockDiagEntry{Cookie: 2, Remote: netip.MustParseAddrPort("127.0.0.1:80"), HasTCPInfo: true})
	emit(sockDiagEntry{Cookie: 3, Remote: internet})
	rx, tx, ok := tails.get(1)
	assert.Equal(t, []any{uint64(10), uint64(20), true}, []any{rx, tx, ok})
	_, _, ok = tails.get(1)
	assert.True(t, ok, "reading doesn't consume")
	_, _, ok = tails.get(2)
	assert.False(t, ok, "loopback is never counted")
	_, _, ok = tails.get(3)
	assert.False(t, ok, "no counters, nothing to credit")

	tails.record(sockDiagEntry{Cookie: 4, Remote: internet, HasTCPInfo: true}, time.Now().Add(sockTailMaxAge+time.Minute))
	_, _, ok = tails.get(1)
	assert.False(t, ok, "pruned by age")
	_, _, ok = tails.get(4)
	assert.True(t, ok)

	tails.close()
	tails.close()
	assert.Equal(t, 1, stops, "Close is idempotent")
	tails.start(listen)
	assert.Equal(t, 1, calls, "a closed listener stays closed")
}

func TestSockTailsWithoutListener(t *testing.T) {
	var tails sockTails
	tails.start(func(func(sockDiagEntry)) (func(), error) { return nil, unix.EPERM })
	_, _, ok := tails.get(1)
	assert.False(t, ok)
	tails.close()

	g := NewGopsUtil()
	g.Close()
	g.Close()
}
