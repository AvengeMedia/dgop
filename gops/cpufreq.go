package gops

import "github.com/AvengeMedia/dgop/models"

type cpuStatic struct {
	minMHz, maxMHz float64
	physicalCores  int
	cache          *models.CPUCache
}

type cpuDynamic struct {
	coreMHz  []float64
	governor string
	epp      string
}
