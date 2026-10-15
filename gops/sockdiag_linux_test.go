//go:build linux

package gops

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// Captured SOCK_DIAG_BY_FAMILY reply (seq 7) for the client end of a [::1]
// TCP connection that sent 5 bytes; tcp_info counts the SYN in bytes_acked.
const capturedTCPv6Msg = "" +
	"900100001400020007000000759e06000a0102008deca9f90000000000000000" +
	"0000000000000001000000000000000000000000000000010000000002000000" +
	"00000000983a00000000000000000000e8030000f7de1a000500080000000000" +
	"0c001500a63800000000000006001600120000001c010200010000000007aa01" +
	"2811030000000000008000001802000000000000000000000000000000000000" +
	"000000000000000000000000000000000000000000000100c4ff000028000000" +
	"16000000ffffff7f0a000000b8ff00000300000000000000c4ff000000000000" +
	"0d2f85cd03000000ffffffffffffffff06000000000000000000000000000000" +
	"0300000002000000000000000d0000000000000001000000d8893d9600000000" +
	"0000000000000000000000000000000000000000000000000200000000000000" +
	"0500000000000000000000000000000000000000000000000000000000000100" +
	"0000010000000000000000000000000000000000000000000000000000000000" +
	"00000000000000000000000000000000"

func nlMsg(typ uint16, seq uint32, payload []byte) []byte {
	b := make([]byte, unix.SizeofNlMsghdr, unix.SizeofNlMsghdr+len(payload)+3)
	binary.NativeEndian.PutUint32(b[0:], uint32(unix.SizeofNlMsghdr+len(payload)))
	binary.NativeEndian.PutUint16(b[4:], typ)
	binary.NativeEndian.PutUint32(b[8:], seq)
	b = append(b, payload...)
	for len(b)%4 != 0 {
		b = append(b, 0)
	}
	return b
}

func nlInt(v int32) []byte {
	return binary.NativeEndian.AppendUint32(nil, uint32(v))
}

func inetDiagV4(state uint8, local, remote string, inode uint32, attrs ...[]byte) []byte {
	b := make([]byte, inetDiagMsgLen)
	b[0] = unix.AF_INET
	b[1] = state
	l, r := netip.MustParseAddrPort(local), netip.MustParseAddrPort(remote)
	binary.BigEndian.PutUint16(b[4:], l.Port())
	binary.BigEndian.PutUint16(b[6:], r.Port())
	copy(b[8:], l.Addr().AsSlice())
	copy(b[24:], r.Addr().AsSlice())
	binary.NativeEndian.PutUint32(b[68:], inode)
	for _, a := range attrs {
		b = append(b, a...)
	}
	return b
}

func rtAttr(typ uint16, data []byte) []byte {
	b := make([]byte, unix.SizeofRtAttr, unix.SizeofRtAttr+len(data)+3)
	binary.NativeEndian.PutUint16(b[0:], uint16(unix.SizeofRtAttr+len(data)))
	binary.NativeEndian.PutUint16(b[2:], typ)
	b = append(b, data...)
	for len(b)%4 != 0 {
		b = append(b, 0)
	}
	return b
}

func TestEncodeSockDiagRequestTCPv4(t *testing.T) {
	got := encodeSockDiagRequest(1, unix.AF_INET, unix.IPPROTO_TCP, allTCPStates&^(1<<tcpListen), 1<<(inetDiagInfo-1))

	want := make([]byte, 72)
	binary.NativeEndian.PutUint32(want[0:], 72)
	binary.NativeEndian.PutUint16(want[4:], 20)     // SOCK_DIAG_BY_FAMILY
	binary.NativeEndian.PutUint16(want[6:], 0x0301) // NLM_F_REQUEST|NLM_F_DUMP
	binary.NativeEndian.PutUint32(want[8:], 1)
	copy(want[16:], []byte{2, 6, 2, 0}) // AF_INET, IPPROTO_TCP, INET_DIAG_INFO bit
	binary.NativeEndian.PutUint32(want[20:], 0xbff)
	assert.Equal(t, want, got)
}

func TestParseSockDiagCapturedTCPv6(t *testing.T) {
	msg, err := hex.DecodeString(capturedTCPv6Msg)
	require.NoError(t, err)
	want := sockDiagEntry{
		Inode:      0x1adef7,
		Protocol:   unix.IPPROTO_TCP,
		Remote:     netip.MustParseAddrPort("[::1]:43513"),
		TxBytes:    6,
		RxBytes:    0,
		HasTCPInfo: true,
	}

	entries, done, err := parseSockDiagResponse(append(msg, nlMsg(unix.NLMSG_DONE, 7, nlInt(0))...), 7, unix.IPPROTO_TCP)
	require.NoError(t, err)
	assert.True(t, done)
	assert.Equal(t, []sockDiagEntry{want}, entries)

	// Multipart: the same reply split across two datagrams.
	entries, done, err = parseSockDiagResponse(msg, 7, unix.IPPROTO_TCP)
	require.NoError(t, err)
	assert.False(t, done)
	assert.Equal(t, []sockDiagEntry{want}, entries)
	entries, done, err = parseSockDiagResponse(nlMsg(unix.NLMSG_DONE, 7, nlInt(0)), 7, unix.IPPROTO_TCP)
	require.NoError(t, err)
	assert.True(t, done)
	assert.Empty(t, entries)
}

func TestParseInetDiagMsgTCPInfo(t *testing.T) {
	full := make([]byte, tcpInfoBytesReceivedOff+8)
	binary.NativeEndian.PutUint64(full[tcpInfoBytesAckedOff:], 1000)
	binary.NativeEndian.PutUint64(full[tcpInfoBytesReceivedOff:], 2000)

	tests := []struct {
		name     string
		protocol uint8
		attrs    [][]byte
		info     bool
		rx, tx   uint64
	}{
		{"full", unix.IPPROTO_TCP, [][]byte{rtAttr(1, make([]byte, 5)), rtAttr(inetDiagInfo, full)}, true, 2000, 1000},
		{"truncated tcp_info", unix.IPPROTO_TCP, [][]byte{rtAttr(inetDiagInfo, full[:tcpInfoBytesReceivedOff+7])}, false, 0, 0},
		{"attribute longer than message", unix.IPPROTO_TCP, [][]byte{rtAttr(inetDiagInfo, full)[:64]}, false, 0, 0},
		{"no attributes", unix.IPPROTO_TCP, nil, false, 0, 0},
		{"udp ignores info", unix.IPPROTO_UDP, [][]byte{rtAttr(inetDiagInfo, full)}, false, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, ok := parseInetDiagMsg(inetDiagV4(1, "192.0.2.10:40000", "198.51.100.7:443", 42, tt.attrs...), tt.protocol)
			require.True(t, ok)
			assert.Equal(t, uint32(42), e.Inode)
			assert.Equal(t, tt.protocol, e.Protocol)
			assert.Equal(t, netip.MustParseAddrPort("198.51.100.7:443"), e.Remote)
			assert.Equal(t, tt.info, e.HasTCPInfo)
			assert.Equal(t, tt.rx, e.RxBytes)
			assert.Equal(t, tt.tx, e.TxBytes)
		})
	}
}

func TestParseSockDiagResponseEdgeCases(t *testing.T) {
	sock := inetDiagV4(1, "192.0.2.10:40000", "198.51.100.7:443", 42)

	t.Run("skips inode 0, short messages and other sequences", func(t *testing.T) {
		buf := append(nlMsg(unix.SOCK_DIAG_BY_FAMILY, 3, inetDiagV4(6, "192.0.2.10:1", "198.51.100.7:2", 0)),
			nlMsg(unix.SOCK_DIAG_BY_FAMILY, 3, sock[:40])...)
		buf = append(buf, nlMsg(unix.SOCK_DIAG_BY_FAMILY, 9, sock)...)
		buf = append(buf, nlMsg(unix.SOCK_DIAG_BY_FAMILY, 3, sock)...)
		entries, done, err := parseSockDiagResponse(buf, 3, unix.IPPROTO_UDP)
		require.NoError(t, err)
		assert.False(t, done)
		require.Len(t, entries, 1)
		assert.Equal(t, uint32(42), entries[0].Inode)
	})

	t.Run("NLMSG_ERROR", func(t *testing.T) {
		_, done, err := parseSockDiagResponse(nlMsg(unix.NLMSG_ERROR, 3, nlInt(-int32(unix.EPERM))), 3, unix.IPPROTO_TCP)
		assert.True(t, done)
		assert.True(t, errors.Is(err, unix.EPERM))
	})

	t.Run("NLMSG_DONE with error code", func(t *testing.T) {
		_, done, err := parseSockDiagResponse(nlMsg(unix.NLMSG_DONE, 3, nlInt(-int32(unix.ENOBUFS))), 3, unix.IPPROTO_TCP)
		assert.True(t, done)
		assert.True(t, errors.Is(err, unix.ENOBUFS))
	})

	t.Run("truncated netlink header", func(t *testing.T) {
		buf := nlMsg(unix.SOCK_DIAG_BY_FAMILY, 3, sock)
		_, _, err := parseSockDiagResponse(buf[:len(buf)-8], 3, unix.IPPROTO_TCP)
		assert.Error(t, err)
	})
}

func TestDumpSocketsLive(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	client, err := net.Dial("tcp4", ln.Addr().String())
	require.NoError(t, err)
	defer client.Close()
	server, err := ln.Accept()
	require.NoError(t, err)
	defer server.Close()
	_, err = client.Write([]byte("hello"))
	require.NoError(t, err)
	_, err = server.Read(make([]byte, 5))
	require.NoError(t, err)

	udpSrv, err := net.ListenPacket("udp4", "127.0.0.1:0")
	require.NoError(t, err)
	defer udpSrv.Close()
	udpCli, err := net.Dial("udp4", udpSrv.LocalAddr().String())
	require.NoError(t, err)
	defer udpCli.Close()

	entries, err := dumpSockets()
	if err != nil {
		t.Skipf("sock_diag unavailable: %v", err)
	}

	serverInode := socketInode(t, server.(*net.TCPConn))
	clientInode := socketInode(t, client.(*net.TCPConn))
	udpInode := socketInode(t, udpCli.(*net.UDPConn))
	listenInode := socketInode(t, ln.(*net.TCPListener))
	var found int
	for _, e := range entries {
		assert.NotZero(t, e.Inode)
		assert.NotEqual(t, listenInode, e.Inode, "listening sockets are not dumped")
		switch e.Inode {
		case serverInode:
			found++
			assert.Equal(t, uint8(unix.IPPROTO_TCP), e.Protocol)
			assert.Equal(t, client.LocalAddr().String(), e.Remote.String())
			assert.True(t, e.HasTCPInfo)
			assert.Equal(t, uint64(5), e.RxBytes)
		case clientInode:
			found++
			assert.Equal(t, ln.Addr().String(), e.Remote.String())
			assert.True(t, e.HasTCPInfo)
		case udpInode:
			found++
			assert.Equal(t, uint8(unix.IPPROTO_UDP), e.Protocol)
			assert.Equal(t, udpSrv.LocalAddr().String(), e.Remote.String())
			assert.False(t, e.HasTCPInfo)
		}
	}
	if udpDiagMissing(t) {
		assert.Equal(t, 2, found, "TCP survives a kernel without udp_diag")
		return
	}
	assert.Equal(t, 3, found)
}

func socketInode(t *testing.T, c syscall.Conn) uint32 {
	raw, err := c.SyscallConn()
	require.NoError(t, err)
	var target string
	require.NoError(t, raw.Control(func(fd uintptr) {
		target, err = os.Readlink(fmt.Sprintf("/proc/self/fd/%d", fd))
	}))
	require.NoError(t, err)
	var inode uint32
	_, err = fmt.Sscanf(target, "socket:[%d]", &inode)
	require.NoError(t, err)
	return inode
}

func TestUDPUnsupported(t *testing.T) {
	enoent := fmt.Errorf("sock_diag: %w", unix.ENOENT)
	assert.True(t, udpUnsupported(unix.IPPROTO_UDP, enoent))
	assert.False(t, udpUnsupported(unix.IPPROTO_TCP, enoent), "a TCP failure is a real failure")
	assert.False(t, udpUnsupported(unix.IPPROTO_UDP, fmt.Errorf("sock_diag: %w", unix.EPERM)))
	assert.False(t, udpUnsupported(unix.IPPROTO_UDP, nil))

	_, _, err := parseSockDiagResponse(nlMsg(unix.NLMSG_ERROR, 3, nlInt(-int32(unix.ENOENT))), 3, unix.IPPROTO_UDP)
	assert.True(t, udpUnsupported(unix.IPPROTO_UDP, err), "the kernel's reply must match")
}

func TestOpenSockDiagSetsReceiveTimeout(t *testing.T) {
	fd, err := openSockDiag()
	if err != nil {
		t.Skipf("sock_diag unavailable: %v", err)
	}
	defer unix.Close(fd)
	tv, err := unix.GetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO)
	require.NoError(t, err)
	assert.Equal(t, int64(1), int64(tv.Sec))
	assert.Zero(t, tv.Usec)
}

// udpDiagMissing sends a UDP request and reports whether the kernel refuses it with ENOENT.
func udpDiagMissing(t *testing.T) bool {
	fd, err := openSockDiag()
	require.NoError(t, err)
	defer unix.Close(fd)
	req := encodeSockDiagRequest(1, unix.AF_INET, unix.IPPROTO_UDP, 1<<tcpEstablished, 0)
	require.NoError(t, unix.Sendto(fd, req, 0, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}))
	buf := make([]byte, 64*1024)
	n, _, err := unix.Recvfrom(fd, buf, 0)
	require.NoError(t, err)
	_, _, err = parseSockDiagResponse(buf[:n], 1, unix.IPPROTO_UDP)
	return udpUnsupported(unix.IPPROTO_UDP, err)
}
