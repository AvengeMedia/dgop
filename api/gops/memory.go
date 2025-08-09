package gops_handler

import (
	"context"

	"github.com/bbedward/DankMaterialShell/dankgop/api/server"
	"github.com/bbedward/DankMaterialShell/dankgop/internal/log"
	"github.com/bbedward/DankMaterialShell/dankgop/models"
	"github.com/danielgtaylor/huma/v2"
)

type MemoryResponse struct {
	Body struct {
		Data *models.MemoryInfo `json:"data"`
	}
}

// GET /memory
func (self *HandlerGroup) Memory(ctx context.Context, _ *server.EmptyInput) (*MemoryResponse, error) {

	memoryInfo, err := self.srv.Gops.GetMemoryInfo()
	if err != nil {
		log.Error("Error getting memory info")
		return nil, huma.Error500InternalServerError("Unable to retrieve memory info")
	}

	resp := &MemoryResponse{}
	resp.Body.Data = memoryInfo
	return resp, nil
}
