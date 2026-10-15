package gops_handler

import (
	"context"

	"github.com/AvengeMedia/dankgo/log"
	"github.com/AvengeMedia/dgop/models"
	"github.com/danielgtaylor/huma/v2"
)

type SystemHardwareResponse struct {
	Body *models.SystemHardware
}

type GPUResponse struct {
	Body *models.GPUInfo
}

type GPUTempInput struct {
	PciId string `query:"pciId" required:"true" example:"10de:2684"`
}

type GPUTempResponse struct {
	Body *models.GPUTempInfo
}

// GET /hardware
func (self *HandlerGroup) SystemHardware(ctx context.Context, input *struct{}) (*SystemHardwareResponse, error) {
	systemInfo, err := self.srv.Gops.GetSystemHardware()
	if err != nil {
		log.Error("Error getting system hardware info")
		return nil, huma.Error500InternalServerError("Unable to retrieve system hardware info")
	}

	return &SystemHardwareResponse{Body: systemInfo}, nil
}

// GET /gpu
func (self *HandlerGroup) GPU(ctx context.Context, input *struct{}) (*GPUResponse, error) {
	gpuInfo, err := self.srv.Gops.GetGPUInfo()
	if err != nil {
		log.Error("Error getting GPU info")
		return nil, huma.Error500InternalServerError("Unable to retrieve GPU info")
	}

	return &GPUResponse{Body: gpuInfo}, nil
}

// GET /gpu/temp
func (self *HandlerGroup) GPUTemp(ctx context.Context, input *GPUTempInput) (*GPUTempResponse, error) {
	gpuTempInfo, err := self.srv.Gops.GetGPUTemp(input.PciId)
	if err != nil {
		log.Error("Error getting GPU temperature")
		return nil, huma.Error400BadRequest(err.Error())
	}

	return &GPUTempResponse{Body: gpuTempInfo}, nil
}

type GPUStatsInput struct {
	Cursor    string   `query:"cursor" doc:"Base64 cursor for rate calculation"`
	GPUPciIds []string `query:"gpu_pci_ids" example:"10de:2684,1002:164e" doc:"Only read these GPUs; others are listed without stats. Omit for all"`
}

func (in *GPUStatsInput) Resolve(ctx huma.Context) []error {
	in.GPUPciIds = pciIdsFromQuery(ctx)
	return nil
}

type GPUStatsResponse struct {
	Body *models.GPUStatsInfo
}

// GET /gpu/stats
func (self *HandlerGroup) GPUStats(ctx context.Context, input *GPUStatsInput) (*GPUStatsResponse, error) {
	gpuStats, err := self.srv.Gops.GetGPUStats(input.Cursor, input.GPUPciIds)
	if err != nil {
		log.Error("Error getting GPU stats")
		return nil, huma.Error500InternalServerError("Unable to retrieve GPU stats")
	}

	return &GPUStatsResponse{Body: gpuStats}, nil
}
