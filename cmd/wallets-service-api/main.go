package main

import (
	"database/sql"

	"github.com/gersastas/wallets-service-api/internal/config"
	"github.com/gersastas/wallets-service-api/internal/database"
	httpserver "github.com/gersastas/wallets-service-api/internal/transport/http/server"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

func main() {
	cfg := config.New()

	db, err := sql.Open("postgres", cfg.GetDatabaseURL())
	if err != nil {
		logrus.Panicf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		logrus.Panicf("failed to ping database: %v", err)
	}

	logrus.Info("connected to database")

	if err := database.RunMigrations(db); err != nil {
		logrus.Panicf("failed to run migrations: %v", err)
	}

	walletRepo := database.NewWalletRepository(db)

	server := httpserver.New(cfg.GetHTTPBindAddr(), walletRepo)

	if err := server.Run(); err != nil {
		logrus.Panic("HTTP server failed", err)
	}
}
