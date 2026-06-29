package utils

import (
	"path/filepath"
	"testing"
)

func TestXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")
	if got := XDGConfigHome(); got != "/custom/config" {
		t.Errorf("XDGConfigHome() = %q, want /custom/config", got)
	}

	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/home/tester")
	if got, want := XDGConfigHome(), "/home/tester/.config"; got != want {
		t.Errorf("XDGConfigHome() = %q, want %q", got, want)
	}
}

func TestXDGCacheHome(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/custom/cache")
	if got := XDGCacheHome(); got != "/custom/cache" {
		t.Errorf("XDGCacheHome() = %q, want /custom/cache", got)
	}

	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "/home/tester")
	if got, want := XDGCacheHome(), "/home/tester/.cache"; got != want {
		t.Errorf("XDGCacheHome() = %q, want %q", got, want)
	}
}

func TestXDGDataHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/custom/data")
	if got := XDGDataHome(); got != "/custom/data" {
		t.Errorf("XDGDataHome() = %q, want /custom/data", got)
	}

	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "/home/tester")
	if got, want := XDGDataHome(), "/home/tester/.local/share"; got != want {
		t.Errorf("XDGDataHome() = %q, want %q", got, want)
	}
}

func TestXDGStateHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/custom/state")
	if got := XDGStateHome(); got != "/custom/state" {
		t.Errorf("XDGStateHome() = %q, want /custom/state", got)
	}

	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "/home/tester")
	if got, want := XDGStateHome(), "/home/tester/.local/state"; got != want {
		t.Errorf("XDGStateHome() = %q, want %q", got, want)
	}
}

func TestDgopDirs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")
	t.Setenv("XDG_CACHE_HOME", "/custom/cache")
	t.Setenv("XDG_DATA_HOME", "/custom/data")
	t.Setenv("XDG_STATE_HOME", "/custom/state")

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"ConfigDir", ConfigDir(), "/custom/config/dgop"},
		{"CacheDir", CacheDir(), "/custom/cache/dgop"},
		{"DataDir", DataDir(), "/custom/data/dgop"},
		{"StateDir", StateDir(), "/custom/state/dgop"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

func TestExpandPath(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	t.Setenv("FOO", "bar")

	tests := []struct {
		in   string
		want string
	}{
		{"~/config", "/home/tester/config"},
		{"$FOO/baz", "bar/baz"},
		{"/abs/path", "/abs/path"},
		{"~/a/../b", "/home/tester/b"},
	}
	for _, tt := range tests {
		got, err := ExpandPath(tt.in)
		if err != nil {
			t.Errorf("ExpandPath(%q) returned error: %v", tt.in, err)
			continue
		}
		if got != filepath.Clean(tt.want) {
			t.Errorf("ExpandPath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
