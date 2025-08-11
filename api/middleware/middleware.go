package middleware

import (
	"github.com/AvengeMedia/dgop/config"
	"github.com/danielgtaylor/huma/v2"
)

type Middleware struct {
	api huma.API
	cfg *config.Config
}

func NewMiddleware(cfg *config.Config, api huma.API) *Middleware {
	return &Middleware{
		api: api,
		cfg: cfg,
	}
}
