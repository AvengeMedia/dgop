//go:build !linux

package gops

import "github.com/AvengeMedia/dgop/models"

func fillGPUStats(_ []models.GPUStats, _ string) string {
	return ""
}
