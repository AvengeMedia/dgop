package gops_handler

import (
	"context"

	"github.com/AvengeMedia/dgop/models"
	"github.com/danielgtaylor/huma/v2"
)

type ProcessDetailInput struct {
	PID int32 `path:"pid" minimum:"1" example:"1234"`
}

type ProcessDetailResponse struct {
	Body *models.ProcessDetail
}

// GET /process/{pid}
func (self *HandlerGroup) ProcessDetail(ctx context.Context, input *ProcessDetailInput) (*ProcessDetailResponse, error) {
	detail, err := self.srv.Gops.GetProcessDetail(input.PID)
	if err != nil {
		return nil, huma.Error404NotFound(err.Error())
	}
	return &ProcessDetailResponse{Body: detail}, nil
}
