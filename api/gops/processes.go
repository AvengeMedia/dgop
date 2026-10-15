package gops_handler

import (
	"context"

	"github.com/AvengeMedia/dankgo/log"
	"github.com/AvengeMedia/dgop/gops"
	"github.com/AvengeMedia/dgop/models"
	"github.com/danielgtaylor/huma/v2"
)

type ProcessInput struct {
	SortBy         gops.ProcSortBy `query:"sort_by" required:"true" default:"cpu"`
	Limit          int             `query:"limit"`
	DisableProcCPU bool            `query:"disable_proc_cpu" default:"false"`
	Cursor         string          `query:"cursor" required:"false"`
	MergeChildren  bool            `query:"merge_children" default:"true"`
	GPU            bool            `query:"gpu" default:"false"`
	IO             bool            `query:"io" default:"false"`
	Net            bool            `query:"net" default:"false"`
	GPUPciIds      []string        `query:"gpu_pci_ids" example:"10de:2684" doc:"With gpu, only read these GPUs. Omit for all"`
}

func (in *ProcessInput) Resolve(ctx huma.Context) []error {
	in.GPUPciIds = pciIdsFromQuery(ctx)
	return nil
}

type ProcessResponse struct {
	Body struct {
		Data         []*models.ProcessInfo `json:"data"`
		Cursor       string                `json:"cursor,omitempty"`
		ProcessNetRx *float64              `json:"processNetRx,omitempty" doc:"Sum of netRx over every process with a known rate, before merging and the limit."`
		ProcessNetTx *float64              `json:"processNetTx,omitempty" doc:"Sum of netTx over every process with a known rate, before merging and the limit."`
	}
}

// GET /processes
func (self *HandlerGroup) Processes(ctx context.Context, input *ProcessInput) (*ProcessResponse, error) {
	enableCPU := !input.DisableProcCPU

	result, err := self.srv.Gops.GetProcessesWithOptions(gops.ProcessOptions{
		SortBy: input.SortBy, Limit: input.Limit, EnableCPU: enableCPU, Cursor: input.Cursor,
		MergeChildren: input.MergeChildren, GPU: input.GPU, GPUPciIds: input.GPUPciIds, IO: input.IO, Net: input.Net,
	})
	if err != nil {
		log.Error("Error getting process info")
		return nil, huma.Error500InternalServerError("Unable to retrieve process info")
	}

	resp := &ProcessResponse{}
	resp.Body.Data = result.Processes
	resp.Body.Cursor = result.Cursor
	resp.Body.ProcessNetRx, resp.Body.ProcessNetTx = result.ProcessNetRx, result.ProcessNetTx
	return resp, nil
}
