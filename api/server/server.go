package server

import (
	"github.com/AvengeMedia/dgop/config"
	"github.com/AvengeMedia/dgop/gops"
)

// EmptyInput can be used when no input is needed.
type EmptyInput struct{}

// DeletedResponse is used to return a deleted response.
type DeletedResponse struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// Server implements generated.ServerInterface
type Server struct {
	Cfg  *config.Config
	Gops *gops.GopsUtil
}
