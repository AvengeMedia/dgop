//go:build linux

package gops

import (
	"sync"
	"time"

	"github.com/AvengeMedia/dgop/models"
)

type platformSources struct {
	sockDump     func() ([]sockDiagEntry, error)
	slowChipRead func(dir string) []hwmonReading
	slowChipWait time.Duration

	memModules struct {
		sync.Mutex
		info *models.MemoryModulesInfo
	}
}

func defaultPlatformSources() platformSources {
	return platformSources{sockDump: dumpSockets, slowChipRead: readHwmonReadings, slowChipWait: 100 * time.Millisecond}
}
