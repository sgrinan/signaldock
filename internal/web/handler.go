package web

import (
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
	Add(string) (endpoint.Endpoint, error)
	List() ([]endpoint.Endpoint, error)
	ByID(uuid.UUID) (endpoint.Endpoint, error)
	RemoveByID(uuid.UUID) error
	Refresh(uuid.UUID) (endpoint.CheckResult, error)
}

type handler struct {
	endpoints endpointService
	users     userStore
	sessions  sessionService
	templates *template.Template
	logger    *slog.Logger
}

// NewHandler returns the HTTP handler for the SignalDock web interface.
func NewHandler(endpoints endpointService, users userStore, sessions sessionService, logger *slog.Logger) (http.Handler, error) {
	tmpl, err := template.ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	staticFS, err := fs.Sub(assets, "static")
	if err != nil {
		return nil, fmt.Errorf("create static filesystem: %w", err)
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

	mux.HandleFunc("GET /{$}", h.handleGetIndex)
	mux.HandleFunc("GET /login", h.handleGetLogin)
	mux.HandleFunc("POST /login", h.handlePostLogin)
	mux.HandleFunc("POST /logout", h.handlePostLogout)
	mux.HandleFunc("POST /endpoints", h.handlePostEndpoint)
	mux.HandleFunc("GET /endpoints/{id}", h.handleGetEndpoint)
	mux.HandleFunc("POST /endpoints/{id}/delete", h.handleDeleteEndpoint)
	mux.HandleFunc("POST /endpoints/{id}/refresh", h.handleRefreshEndpoint)

	mux.HandleFunc("GET /api/prometheus/targets", h.handlePrometheusTargets)

	return securityHeaders(mux), nil
}
