package web

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	"uuid"

	"github.com/sgrinan/signaldock/internal/endpoint"
)

//go:embed templates/*.html static/*
var assets embed.FS

type endpointService interface {
	Add(context.Context, string) (endpoint.Endpoint, error)
	List(context.Context) ([]endpoint.Endpoint, error)
	ByID(context.Context, uuid.UUID) (endpoint.Endpoint, error)
	RemoveByID(context.Context, uuid.UUID) error
	Refresh(context.Context, uuid.UUID) (endpoint.CheckResult, error)
}

type handler struct {
	endpoints endpointService
	users     userRepository
	sessions  sessionService
	templates *template.Template
	logger    *slog.Logger
}

// NewHandler returns the HTTP handler for the SignalDock web interface.
func NewHandler(endpoints endpointService, users userRepository, sessions sessionService, grafanaURL string, logger *slog.Logger) (http.Handler, error) {
	tmpl, err := template.ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	staticFS, err := fs.Sub(assets, "static")
	if err != nil {
		return nil, fmt.Errorf("create static filesystem: %w", err)
	}

	grafanaProxy, err := newGrafanaProxy(grafanaURL, logger)
	if err != nil {
		return nil, err
	}

	h := &handler{
		endpoints: endpoints,
		users:     users,
		sessions:  sessions,
		templates: tmpl,
		logger:    logger,
	}

	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.Handle("GET /{$}", h.requireAuth(http.HandlerFunc(h.handleGetIndex)))

	mux.HandleFunc("GET /login", h.handleGetLogin)
	mux.HandleFunc("POST /login", h.handlePostLogin)
	mux.Handle("POST /logout", h.requireAuth(http.HandlerFunc(h.handlePostLogout)))

	mux.Handle("GET /endpoints/{id}", h.requireAuth(http.HandlerFunc(h.handleGetEndpoint)))
	mux.Handle("POST /endpoints", h.requireAuth(h.requireAdmin(http.HandlerFunc(h.handlePostEndpoint))))
	mux.Handle("POST /endpoints/{id}/delete", h.requireAuth(h.requireAdmin(http.HandlerFunc(h.handleDeleteEndpoint))))
	mux.Handle("POST /endpoints/{id}/refresh", h.requireAuth(h.requireAdmin(http.HandlerFunc(h.handleRefreshEndpoint))))

	mux.Handle("GET /users", h.requireAuth(h.requireAdmin(http.HandlerFunc(h.handleGetUsers))))
	mux.Handle("POST /users", h.requireAuth(h.requireAdmin(http.HandlerFunc(h.handlePostUser))))
	mux.Handle("POST /users/{id}/delete", h.requireAuth(h.requireAdmin(http.HandlerFunc(h.handleDeleteUser))))
	mux.Handle("POST /users/{id}", h.requireAuth(h.requireAdmin(http.HandlerFunc(h.handleUpdateUser))))

	mux.HandleFunc("GET /api/prometheus/targets", h.handlePrometheusTargets)

	rootMux := http.NewServeMux()

	rootMux.Handle("/grafana/", h.requireAuth(grafanaProxy))

	rootMux.Handle("/", securityHeaders(mux))

	return rootMux, nil
}
