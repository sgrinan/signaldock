package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sgrinan/signaldock/internal/database"
	"github.com/sgrinan/signaldock/internal/endpoint"
	"github.com/sgrinan/signaldock/internal/session"
	"github.com/sgrinan/signaldock/internal/user"
	"github.com/sgrinan/signaldock/internal/web"
)

const (
	defaultPort     = "8080"
	shutdownTimeout = 10 * time.Second
)

func run(logger *slog.Logger) error {
	showVersion := flag.Bool("version", false, "print version information")
	flag.Parse()

	if *showVersion {
		printVersion()
		return nil
	}

	port := os.Getenv("SIGNALDOCK_PORT")
	if port == "" {
		port = defaultPort
	}

	databaseURL := os.Getenv("SIGNALDOCK_DATABASE_URL")
	if databaseURL == "" {
		return errors.New("SIGNALDOCK_DATABASE_URL is required")
	}

	ctx := context.Background()

	pool, err := database.NewPostgresPool(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	defer pool.Close()

	if err := database.EnsureSchema(ctx, pool); err != nil {
		return fmt.Errorf("apply database migrations: %w", err)
	}

	endpointRepository := endpoint.NewRepository(pool)
	endpointChecks := endpoint.NewCheckStore()
	endpointService := endpoint.NewService(endpointRepository, endpointChecks)

	userRepository := user.NewRepository(pool)

	if err := ensureInitialAdmin(userRepository, logger); err != nil {
		return err
	}

	sessionRepository := session.NewRepository(pool)
	sessionService := session.NewService(sessionRepository)

	if err := refreshEndpointsOnStartup(endpointService, logger); err != nil {
		return err
	}

	handler, err := web.NewHandler(endpointService, userRepository, sessionService, logger)
	if err != nil {
		return fmt.Errorf("create HTTP handler: %w", err)
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
			return fmt.Errorf("start HTTP server: %w", err)
		}

		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	logger.Info("SignalDock stopped")

	return nil
}
