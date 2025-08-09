package gops_handler

import (
	"context"

	"github.com/bbedward/DankMaterialShell/dankgop/api/server"
	"github.com/bbedward/DankMaterialShell/dankgop/internal/log"
	"github.com/bbedward/DankMaterialShell/dankgop/models"
	"github.com/danielgtaylor/huma/v2"
)

type CpuResponse struct {
	Body struct {
		Data *models.CPUInfo `json:"data"`
	}
}

// GET /cpu
func (self *HandlerGroup) Cpu(ctx context.Context, _ *server.EmptyInput) (*CpuResponse, error) {

	cpuInfo, err := self.srv.Gops.GetCPUInfo()
	if err != nil {
		log.Error("Error getting CPU info")
		return nil, huma.Error500InternalServerError("Unable to retrieve CPU info")
	}

	resp := &CpuResponse{}
	resp.Body.Data = cpuInfo
	return resp, nil
}
