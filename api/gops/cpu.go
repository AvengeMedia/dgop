package gops_handler

import (
	"context"

	"github.com/bbedward/DankMaterialShell/dankgop/internal/log"
	"github.com/bbedward/DankMaterialShell/dankgop/models"
	"github.com/danielgtaylor/huma/v2"
)

type CpuInput struct {
	SampleData string `query:"sample_data" required:"false"`
}

type CpuResponse struct {
	Body struct {
		Data *models.CPUInfo `json:"data"`
	}
}

// GET /cpu
func (self *HandlerGroup) Cpu(ctx context.Context, input *CpuInput) (*CpuResponse, error) {
	var sampleData *models.CPUSampleData
	if input.SampleData != "" {
		// Client can encode sample data as JSON in query parameter
		// Implementation would decode it here
	}
	
	cpuInfo, err := self.srv.Gops.GetCPUInfoWithSample(sampleData)
	if err != nil {
		log.Error("Error getting CPU info")
		return nil, huma.Error500InternalServerError("Unable to retrieve CPU info")
	}

	resp := &CpuResponse{}
	resp.Body.Data = cpuInfo
	return resp, nil
}
