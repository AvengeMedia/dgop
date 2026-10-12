//go:build !linux

package gops

import "github.com/AvengeMedia/dgop/models"

// GetProcessDetail only knows the PID off Linux; the fields come from /proc.
func (self *GopsUtil) GetProcessDetail(pid int32) (*models.ProcessDetail, error) {
	return &models.ProcessDetail{PID: pid}, nil
}
