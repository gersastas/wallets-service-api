package main

import (
	"context"
	"database/sql"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	// ✅ ИСПРАВЛЕНО lint: Проверяем ошибку Close
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			logrus.Errorf("failed to close database connection: %v", closeErr)
		}
	}()

	if err := db.Ping(); err != nil {
		logrus.Panicf("failed to ping database: %v", err)
	}

	logrus.Info("connected to database")

	if err := database.RunMigrations(db); err != nil {
		logrus.Panicf("failed to run migrations: %v", err)
	}

	walletRepo := database.NewWalletRepository(db)
	transactionRepo := database.NewTransactionRepository(db)

	server := httpserver.New(cfg.GetHTTPBindAddr(), walletRepo, transactionRepo, db)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logrus.Infof("starting server on %s", cfg.GetHTTPBindAddr())
		if err := server.Run(); err != nil {
			logrus.Fatalf("server failed: %v", err)
		}
	}()

	<-quit
	logrus.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logrus.Fatalf("server forced to shutdown: %v", err)
	}

	logrus.Info("server stopped gracefully")
}