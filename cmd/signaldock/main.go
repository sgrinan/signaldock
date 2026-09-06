package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sgrinan/signaldock/internal/endpoint"
	"github.com/sgrinan/signaldock/internal/web"
)

const (
	defaultPort     = "8080"
	shutdownTimeout = 10 * time.Second
)

func main() {
	port := os.Getenv("SIGNALDOCK_PORT")
	if port == "" {
		port = defaultPort
	}

	logger := slog.New(
		slog.NewTextHandler(os.Stdout, nil),
	)

	store := endpoint.NewStore()
	service := endpoint.NewService(store)

	handler, err := web.NewHandler(service, logger)
	if err != nil {
		logger.Error("failed to create HTTP handler", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)

	go func() {
		logger.Info("starting SignalDock", "port", port)

		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}

		serverErr <- nil
	}()

	shutdown := make(chan os.Signal, 1)

	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(shutdown)

	select {
	case sig := <-shutdown:
		logger.Info("shutting down SignalDock", "signal", sig.String())

	case err := <-serverErr:
		if err != nil {
			logger.Error("failed to start HTTP server", "error", err)
			os.Exit(1)
		}

		return
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		shutdownTimeout,
	)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("failed to shutdown HTTP server", "error", err)
		os.Exit(1)
	}

	logger.Info("SignalDock stopped")
}
