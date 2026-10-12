//go:build !linux

package gops

import "github.com/AvengeMedia/dgop/models"

func fillLinkInfo(_ *models.NetworkRateInfo) {}
