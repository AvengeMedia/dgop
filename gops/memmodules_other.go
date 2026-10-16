//go:build !linux

package gops

import "github.com/AvengeMedia/dgop/models"

func (self *GopsUtil) GetMemoryModules() (*models.MemoryModulesInfo, error) {
	return &models.MemoryModulesInfo{Modules: []models.MemoryModule{}}, nil
}
