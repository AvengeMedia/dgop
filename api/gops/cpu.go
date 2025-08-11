package gops_handler

import (
	"context"

	"github.com/AvengeMedia/dgop/internal/log"
	"github.com/AvengeMedia/dgop/models"
	"github.com/danielgtaylor/huma/v2"
)

type CpuInput struct {
	Cursor string `query:"cursor" required:"false"`
}

type CpuResponse struct {
	Body struct {
		Data *models.CPUInfo `json:"data"`
	}
}

// GET /cpu
func (self *HandlerGroup) Cpu(ctx context.Context, input *CpuInput) (*CpuResponse, error) {
	cpuInfo, err := self.srv.Gops.GetCPUInfoWithCursor(input.Cursor)
	if err != nil {
		log.Error("Error getting CPU info")
		return nil, huma.Error500InternalServerError("Unable to retrieve CPU info")
	}

	resp := &CpuResponse{}
	resp.Body.Data = cpuInfo
	return resp, nil
}
