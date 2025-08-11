package gops_handler

import (
	"context"

	"github.com/AvengeMedia/dgop/api/server"
	"github.com/AvengeMedia/dgop/internal/log"
	"github.com/AvengeMedia/dgop/models"
	"github.com/danielgtaylor/huma/v2"
)

type SystemResponse struct {
	Body struct {
		Data *models.SystemInfo `json:"data"`
	}
}

// GET /system
func (self *HandlerGroup) System(ctx context.Context, _ *server.EmptyInput) (*SystemResponse, error) {

	systemInfo, err := self.srv.Gops.GetSystemInfo()
	if err != nil {
		log.Error("Error getting system info")
		return nil, huma.Error500InternalServerError("Unable to retrieve system info")
	}

	resp := &SystemResponse{}
	resp.Body.Data = systemInfo
	return resp, nil
}
