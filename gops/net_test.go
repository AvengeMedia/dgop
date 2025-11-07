package gops

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchesNetworkInterface(t *testing.T) {
	tests := []struct {
		name     string
		iface    string
		expected bool
	}{
		{"wireless wlan", "wlan0", true},
		{"wireless wlo", "wlo1", true},
		{"wireless wlp", "wlp2s0", true},
		{"ethernet eth", "eth0", true},
		{"ethernet eno", "eno1", true},
		{"ethernet enp", "enp3s0", true},
		{"ethernet ens", "ens33", true},
		{"lxc bridge", "lxcbr0", true},
		{"loopback", "lo", false},
		{"docker", "docker0", false},
		{"virbr", "virbr0", false},
		{"veth", "veth123abc", false},
		{"tun", "tun0", false},
		{"tap", "tap0", false},
		{"empty", "", false},
		{"random", "foobar", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesNetworkInterface(tt.iface)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchesNetworkInterfaceRealWorld(t *testing.T) {
	// Test common real-world interface names
	validInterfaces := []string{
		"eth0", "eth1",
		"eno1", "eno2",
		"enp0s3", "enp0s25",
		"ens33", "ens192",
		"wlan0", "wlan1",
		"wlo1",
		"wlp3s0", "wlp4s0",
		"lxcbr0", "lxcbr1",
	}

	for _, iface := range validInterfaces {
		t.Run(iface, func(t *testing.T) {
			assert.True(t, matchesNetworkInterface(iface), "%s should match", iface)
		})
	}

	invalidInterfaces := []string{
		"lo",
		"docker0", "docker1",
		"virbr0", "virbr1",
		"veth0", "vethab123cd",
		"br-1234567890ab",
		"tun0", "tap0",
	}

	for _, iface := range invalidInterfaces {
		t.Run(iface, func(t *testing.T) {
			assert.False(t, matchesNetworkInterface(iface), "%s should not match", iface)
		})
	}
}

func BenchmarkMatchesNetworkInterface(b *testing.B) {
	testCases := []string{"eth0", "wlan0", "docker0", "lo", "enp3s0"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			matchesNetworkInterface(tc)
		}
	}
}
