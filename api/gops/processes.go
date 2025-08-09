package gops_handler

import (
	"context"

	"github.com/bbedward/DankMaterialShell/dankgop/gops"
	"github.com/bbedward/DankMaterialShell/dankgop/internal/log"
	"github.com/bbedward/DankMaterialShell/dankgop/models"
	"github.com/danielgtaylor/huma/v2"
)

type ProcessInput struct {
	SortBy         gops.ProcSortBy `query:"sort_by" required:"true" default:"cpu"`
	Limit          int             `query:"limit"`
	DisableProcCPU bool            `query:"disable_proc_cpu" default:"false"`
	SampleData     string          `query:"sample_data" required:"false"`
}

type ProcessResponse struct {
	Body struct {
		Data []*models.ProcessInfo `json:"data"`
	}
}

// GET /processes
func (self *HandlerGroup) Processes(ctx context.Context, input *ProcessInput) (*ProcessResponse, error) {
	enableCPU := !input.DisableProcCPU
	
	var sampleData []models.ProcessSampleData
	if input.SampleData != "" {
		// Client can encode sample data as JSON in query parameter
		// Implementation would decode it here
	}
	
	processInfo, err := self.srv.Gops.GetProcessesWithSample(input.SortBy, input.Limit, enableCPU, sampleData)
	if err != nil {
		log.Error("Error getting process info")
		return nil, huma.Error500InternalServerError("Unable to retrieve process info")
	}

	resp := &ProcessResponse{}
	resp.Body.Data = processInfo
	return resp, nil
}
