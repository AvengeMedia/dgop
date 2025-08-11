package gops_handler

import (
	"context"

	"github.com/AvengeMedia/dgop/internal/log"
	"github.com/AvengeMedia/dgop/models"
	"github.com/danielgtaylor/huma/v2"
)

type NetRateInput struct {
	Cursor string `query:"cursor" doc:"Base64 cursor for rate calculation"`
}

type NetRateResponse struct {
	Body *models.NetworkRateResponse
}

// GET /net-rate
func (self *HandlerGroup) NetRate(ctx context.Context, input *NetRateInput) (*NetRateResponse, error) {
	netRateInfo, err := self.srv.Gops.GetNetworkRates(input.Cursor)
	if err != nil {
		log.Error("Error getting network rates")
		return nil, huma.Error500InternalServerError("Unable to retrieve network rates")
	}

	resp := &NetRateResponse{}
	resp.Body = netRateInfo
	return resp, nil
}
