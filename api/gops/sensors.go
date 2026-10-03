package gops_handler

import (
	"context"

	"github.com/AvengeMedia/dankgo/httpapi"
	"github.com/AvengeMedia/dankgo/log"
	"github.com/AvengeMedia/dgop/models"
	"github.com/danielgtaylor/huma/v2"
)

type SensorsResponse struct {
	Body *models.SensorsInfo
}

// GET /sensors
func (self *HandlerGroup) Sensors(ctx context.Context, _ *httpapi.EmptyInput) (*SensorsResponse, error) {
	sensorsInfo, err := self.srv.Gops.GetSensors()
	if err != nil {
		log.Error("Error getting sensors")
		return nil, huma.Error500InternalServerError("Unable to retrieve sensors")
	}

	return &SensorsResponse{Body: sensorsInfo}, nil
}
