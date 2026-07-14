package gops_handler

import (
	"context"

	"github.com/AvengeMedia/dankgo/httpapi"
	"github.com/AvengeMedia/dankgo/log"
	"github.com/AvengeMedia/dgop/models"
	"github.com/danielgtaylor/huma/v2"
)

type NetworkResponse struct {
	Body struct {
		Data []*models.NetworkInfo `json:"data"`
	}
}

// GET /network
func (self *HandlerGroup) Network(ctx context.Context, _ *httpapi.EmptyInput) (*NetworkResponse, error) {

	networkInfo, err := self.srv.Gops.GetNetworkInfo()
	if err != nil {
		log.Error("Error getting Network info")
		return nil, huma.Error500InternalServerError("Unable to retrieve Network info")
	}

	resp := &NetworkResponse{}
	resp.Body.Data = networkInfo
	return resp, nil
}
