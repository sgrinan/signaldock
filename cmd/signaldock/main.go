package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"uuid"

	"github.com/sgrinan/signaldock/internal/auth"
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

func main() {
	showVersion := flag.Bool("version", false, "print version information")
	flag.Parse()

	if *showVersion {
		printVersion()
		return
	}

	port := os.Getenv("SIGNALDOCK_PORT")
	if port == "" {
		port = defaultPort
	}

	logger := slog.New(
		slog.NewTextHandler(os.Stdout, nil),
	)

	databaseURL := os.Getenv("SIGNALDOCK_DATABASE_URL")
	if databaseURL == "" {
		logger.Error("SIGNALDOCK_DATABASE_URL is required")
		os.Exit(1)
	}

	ctx := context.Background()

	pool, err := database.NewPostgresPool(ctx, databaseURL)
	if err != nil {
		logger.Error("failed to connect to PostgreSQL", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := database.EnsureSchema(ctx, pool); err != nil {
		logger.Error("failed to apply database migrations", "error", err)
		os.Exit(1)
	}

	endpointRepository := endpoint.NewRepository(pool)
	endpointChecks := endpoint.NewCheckStore()
	endpointService := endpoint.NewService(endpointRepository, endpointChecks)

	userRepository := user.NewRepository(pool)

	users, err := userRepository.List()
	if err != nil {
		logger.Error("failed to load users", "error", err)
		os.Exit(1)
	}

	if len(users) == 0 {
		username := os.Getenv("SIGNALDOCK_ADMIN_USERNAME")
		password := os.Getenv("SIGNALDOCK_ADMIN_PASSWORD")

		if username == "" || password == "" {
			logger.Error("initial admin credentials are required")
			os.Exit(1)
		}

		admin := user.User{
			ID:           uuid.NewV7(),
			Username:     username,
			PasswordHash: auth.HashPassword(password),
			Role:         user.RoleAdmin,
		}

		if err := userRepository.Insert(admin); err != nil {
			logger.Error("failed to create initial admin", "error", err)
			os.Exit(1)
		}

		logger.Info("initial admin created", "username", username)
	}

	sessionStore := session.NewStore(pool)
	sessionService := session.NewService(sessionStore)

	endpoints, err := endpointService.List()
	if err != nil {
		logger.Error("failed to load endpoints", "error", err)
		os.Exit(1)
	}

	for _, ep := range endpoints {
		if _, err := endpointService.Refresh(ep.ID); err != nil {
			logger.Warn("failed to refresh endpoint on startup", "endpoint_id", ep.ID, "url", ep.URL, "error", err)
		}
	}

	handler, err := web.NewHandler(endpointService, userRepository, sessionService, logger)
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
