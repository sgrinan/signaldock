package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/sgrinan/signaldock/internal/endpoint"
	"github.com/sgrinan/signaldock/internal/web"
)

const port = "8080"

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	logger.Info("starting SignalDock", "port", port)

	store := endpoint.Store{}

	handler, err := web.NewHandler(&store, logger)
	if err != nil {
		logger.Error("failed to create HTTP handler", "error", err)
		os.Exit(1)
	}

	if err := http.ListenAndServe(":"+port, handler); err != nil {
		logger.Error("HTTP server stopped", "error", err)
		os.Exit(1)
	}
}
