package main

import (
	"github.com/gersastas/wallets-service-api/internal/config"
	httpserver "github.com/gersastas/wallets-service-api/internal/transport/http/server"
	"github.com/sirupsen/logrus"
)

func main() {
	cfg := config.New()
	server := httpserver.New(cfg.GetHTTPBindAddr())

	if err := server.Run(); err != nil {
		logrus.Panic("HTTP server failed", err)
	}
}
