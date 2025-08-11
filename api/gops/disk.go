package gops_handler

import (
	"context"

	"github.com/AvengeMedia/dgop/api/server"
	"github.com/AvengeMedia/dgop/internal/log"
	"github.com/AvengeMedia/dgop/models"
	"github.com/danielgtaylor/huma/v2"
)

type DiskResponse struct {
	Body struct {
		Data []*models.DiskInfo `json:"data"`
	}
}

// GET /disk
func (self *HandlerGroup) Disk(ctx context.Context, _ *server.EmptyInput) (*DiskResponse, error) {

	diskInfo, err := self.srv.Gops.GetDiskInfo()
	if err != nil {
		log.Error("Error getting Disk info")
		return nil, huma.Error500InternalServerError("Unable to retrieve Disk info")
	}

	resp := &DiskResponse{}
	resp.Body.Data = diskInfo
	return resp, nil
}

// GET /disk/mounts
type DiskMountsResponse struct {
	Body struct {
		Data []*models.DiskMountInfo `json:"data"`
	}
}

func (self *HandlerGroup) DiskMounts(ctx context.Context, _ *server.EmptyInput) (*DiskMountsResponse, error) {

	diskMountsInfo, err := self.srv.Gops.GetDiskMounts()
	if err != nil {
		log.Error("Error getting Disk Mounts info")
		return nil, huma.Error500InternalServerError("Unable to retrieve Disk Mounts info")
	}

	resp := &DiskMountsResponse{}
	resp.Body.Data = diskMountsInfo
	return resp, nil
}
