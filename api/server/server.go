package server

import (
	"github.com/AvengeMedia/dgop/config"
	"github.com/AvengeMedia/dgop/gops"
)

type Server struct {
	Cfg  *config.Config
	Gops *gops.GopsUtil
}
