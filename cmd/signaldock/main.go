package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/sgrinan/signaldock/internal/endpoint"
	"github.com/sgrinan/signaldock/internal/web"
)

func main() {
	port := os.Getenv("SIGNALDOCK_PORT")
	if port == "" {
		port = "8080"
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	store := endpoint.NewStore()

	handler, err := web.NewHandler(store, logger)
	if err != nil {
		logger.Error("failed to create HTTP handler", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	logger.Info("starting SignalDock", "port", port)

	if err := server.ListenAndServe(); err != nil {
		logger.Error("HTTP server stopped", "error", err)
		os.Exit(1)
	}
}
