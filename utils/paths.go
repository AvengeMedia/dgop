package utils

import (
	"os"
	"path/filepath"
	"strings"
)

func XDGConfigHome() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

func XDGCacheHome() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache")
}

func XDGDataHome() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share")
}

func XDGStateHome() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state")
}

func ConfigDir() string {
	return filepath.Join(XDGConfigHome(), "dgop")
}

func CacheDir() string {
	return filepath.Join(XDGCacheHome(), "dgop")
}

func DataDir() string {
	return filepath.Join(XDGDataHome(), "dgop")
}

func StateDir() string {
	return filepath.Join(XDGStateHome(), "dgop")
}

func ExpandPath(path string) (string, error) {
	expanded := os.ExpandEnv(path)

	if strings.HasPrefix(expanded, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		expanded = filepath.Join(home, expanded[1:])
	}

	return filepath.Clean(expanded), nil
}
