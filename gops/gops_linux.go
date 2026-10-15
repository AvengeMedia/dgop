//go:build linux

package gops

import "time"

// platformSources holds the Linux-only data sources tests replace.
type platformSources struct {
	sockDump func() ([]sockDiagEntry, error)
	i2cRead  func(dir string) []hwmonReading
	i2cWait  time.Duration
}

func defaultPlatformSources() platformSources {
	return platformSources{sockDump: dumpSockets, i2cRead: readHwmonReadings, i2cWait: 100 * time.Millisecond}
}
