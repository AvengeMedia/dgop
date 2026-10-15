//go:build !linux

package gops

import "github.com/AvengeMedia/dgop/models"

func (self *GopsUtil) fillGPUStats(_ []models.GPUStats, _ string) string {
	return ""
}
