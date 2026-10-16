//go:build !linux

package gops

import "github.com/AvengeMedia/dgop/models"

func fillLinkInfos(_ []*models.NetworkRateInfo) {}
