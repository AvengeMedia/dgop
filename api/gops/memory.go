package gops_handler

import (
	"context"

	"github.com/AvengeMedia/dgop/api/server"
	"github.com/AvengeMedia/dgop/internal/log"
	"github.com/AvengeMedia/dgop/models"
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
