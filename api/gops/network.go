package gops_handler

import (
	"context"

	"github.com/AvengeMedia/dgop/api/server"
	"github.com/AvengeMedia/dgop/internal/log"
	"github.com/AvengeMedia/dgop/models"
	"github.com/danielgtaylor/huma/v2"
)

type NetworkResponse struct {
	Body struct {
		Data []*models.NetworkInfo `json:"data"`
	}
}

// GET /network
func (self *HandlerGroup) Network(ctx context.Context, _ *server.EmptyInput) (*NetworkResponse, error) {

	networkInfo, err := self.srv.Gops.GetNetworkInfo()
	if err != nil {
		log.Error("Error getting Network info")
		return nil, huma.Error500InternalServerError("Unable to retrieve Network info")
	}

	resp := &NetworkResponse{}
	resp.Body.Data = networkInfo
	return resp, nil
}
