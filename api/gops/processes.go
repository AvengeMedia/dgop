package gops_handler

import (
	"context"

	"github.com/bbedward/DankMaterialShell/dankgop/gops"
	"github.com/bbedward/DankMaterialShell/dankgop/internal/log"
	"github.com/bbedward/DankMaterialShell/dankgop/models"
	"github.com/danielgtaylor/huma/v2"
)

type ProcessInput struct {
	SortBy         gops.ProcSortBy `json:"sort_by" required:"true" default:"cpu"`
	Limit          int             `json:"limit"`
	DisableProcCPU bool            `json:"disable_proc_cpu" default:"false"`
}

type ProcessResponse struct {
	Body struct {
		Data []*models.ProcessInfo `json:"data"`
	}
}

// GET /processes
func (self *HandlerGroup) Processes(ctx context.Context, input *ProcessInput) (*ProcessResponse, error) {
	enableCPU := !input.DisableProcCPU
	processInfo, err := self.srv.Gops.GetProcesses(input.SortBy, input.Limit, enableCPU)
	if err != nil {
		log.Error("Error getting process info")
		return nil, huma.Error500InternalServerError("Unable to retrieve process info")
	}

	resp := &ProcessResponse{}
	resp.Body.Data = processInfo
	return resp, nil
}
