package gops_handler

import (
	"context"

	"github.com/AvengeMedia/dgop/gops"
	"github.com/AvengeMedia/dgop/internal/log"
	"github.com/AvengeMedia/dgop/models"
	"github.com/danielgtaylor/huma/v2"
)

type ProcessInput struct {
	SortBy         gops.ProcSortBy `query:"sort_by" required:"true" default:"cpu"`
	Limit          int             `query:"limit"`
	DisableProcCPU bool            `query:"disable_proc_cpu" default:"false"`
	Cursor         string          `query:"cursor" required:"false"`
}

type ProcessResponse struct {
	Body struct {
		Data   []*models.ProcessInfo `json:"data"`
		Cursor string                `json:"cursor,omitempty"`
	}
}

// GET /processes
func (self *HandlerGroup) Processes(ctx context.Context, input *ProcessInput) (*ProcessResponse, error) {
	enableCPU := !input.DisableProcCPU

	result, err := self.srv.Gops.GetProcessesWithCursor(input.SortBy, input.Limit, enableCPU, input.Cursor)
	if err != nil {
		log.Error("Error getting process info")
		return nil, huma.Error500InternalServerError("Unable to retrieve process info")
	}

	resp := &ProcessResponse{}
	resp.Body.Data = result.Processes
	resp.Body.Cursor = result.Cursor
	return resp, nil
}
