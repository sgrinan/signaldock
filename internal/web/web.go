package web

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"uuid"

	"github.com/sgrinan/signaldock/internal/endpoint"
	"github.com/sgrinan/signaldock/internal/probe"
)

//go:embed templates/*.html
var templates embed.FS

type PageData struct {
	Endpoints []endpoint.Endpoint
	Error     string
	CSRFToken string
}

type EndpointPageData struct {
	Endpoint      endpoint.Endpoint
	CSRFToken     string
	LatencyMS     int64
	LastCheckedAt string
	TLSExpiresAt  string
}

type EndpointStore interface {
	Add(string) (endpoint.Endpoint, error)
	List() []endpoint.Endpoint
	GetByID(uuid.UUID) (endpoint.Endpoint, bool)
	RemoveByID(uuid.UUID) bool
	Refresh(uuid.UUID) (endpoint.CheckResult, error)
}

func generateCSRFToken() string {
	key := make([]byte, 32)
	rand.Read(key)

	return hex.EncodeToString(key)
}

func validateCSRF(r *http.Request) bool {
	cookie, err := r.Cookie("csrf_token")
	if err != nil {
		return false
	}

	formToken := r.FormValue("csrf_token")

	return cookie.Value != "" && formToken != "" && cookie.Value == formToken
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")

		next.ServeHTTP(w, r)
	})
}

func NewHandler(store EndpointStore, logger *slog.Logger) (http.Handler, error) {
	tmpl, err := template.ParseFS(templates, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", handleGetIndex(store, tmpl, logger))
	mux.HandleFunc("POST /endpoints", handlePostEndpoint(store, logger))
	mux.HandleFunc("GET /endpoints/{id}", handleGetEndpoint(store, tmpl, logger))
	mux.HandleFunc("POST /endpoints/{id}/delete", handleDeleteEndpoint(store, logger))
	mux.HandleFunc("POST /endpoints/{id}/refresh", handleRefreshEndpoint(store, logger))

	return securityHeaders(mux), nil
}

func handleGetIndex(store EndpointStore, tmpl *template.Template, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result := r.URL.Query().Get("result")

		csrfToken := generateCSRFToken()

		pageData := PageData{
			Endpoints: store.List(),
			CSRFToken: csrfToken,
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "csrf_token",
			Value:    csrfToken,
			Path:     "/",
			SameSite: http.SameSiteStrictMode,
		})

		switch result {
		case "unreachable":
			pageData.Error = "Endpoint was added, but it is currently unreachable"
		case "duplicate":
			pageData.Error = "Endpoint already exists"
		case "invalid":
			pageData.Error = "Enter a valid HTTP or HTTPS URL"
		case "unsafe":
			pageData.Error = "Private or unsafe network destinations are not allowed"
		}

		if err := tmpl.ExecuteTemplate(w, "index.html", pageData); err != nil {
			logger.Error("failed to render template", "template", "index.html", "error", err)
			http.Error(w, "failed to render page", http.StatusInternalServerError)
			return
		}
	}
}

func handlePostEndpoint(store EndpointStore, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !validateCSRF(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		rawURL := r.FormValue("url")

		added, err := store.Add(rawURL)
		switch {
		case err == nil:
			logger.Info("endpoint added", "url", added.URL, "id", added.ID)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return

		case errors.Is(err, endpoint.ErrEndpointExists):
			http.Redirect(w, r, "/?result=duplicate", http.StatusSeeOther)
			return

		case errors.Is(err, endpoint.ErrUnsupportedScheme),
			errors.Is(err, endpoint.ErrHostRequired):
			http.Redirect(w, r, "/?result=invalid", http.StatusSeeOther)
			return

		case errors.Is(err, probe.ErrUnsafeHost):
			http.Redirect(w, r, "/?result=unsafe", http.StatusSeeOther)
			return

		case added != (endpoint.Endpoint{}):
			logger.Warn("endpoint initial check failed", "url", added.URL, "error", err)
			http.Redirect(w, r, "/?result=unreachable", http.StatusSeeOther)
			return

		default:
			logger.Error("failed to add endpoint", "url", rawURL, "error", err)
			http.Error(w, "failed to add endpoint", http.StatusInternalServerError)
			return
		}
	}
}

func handleGetEndpoint(store EndpointStore, tmpl *template.Template, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rawID := r.PathValue("id")

		id, err := uuid.Parse(rawID)
		if err != nil {
			http.Error(w, "invalid endpoint ID", http.StatusBadRequest)
			return
		}

		ep, found := store.GetByID(id)
		if !found {
			http.Error(w, "endpoint not found", http.StatusNotFound)
			return
		}

		csrfToken := generateCSRFToken()

		tlsExpiresAt := ""

		if !ep.LastCheck.TLS.ExpiresAt.IsZero() {
			tlsExpiresAt = ep.LastCheck.TLS.ExpiresAt.Format("02 Jan 2006")
		}

		pageData := EndpointPageData{
			Endpoint:      ep,
			CSRFToken:     csrfToken,
			LatencyMS:     ep.LastCheck.HTTP.Latency.Milliseconds(),
			LastCheckedAt: ep.LastCheck.HTTP.CheckedAt.Format("15:04:05"),
			TLSExpiresAt:  tlsExpiresAt,
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "csrf_token",
			Value:    csrfToken,
			Path:     "/",
			SameSite: http.SameSiteStrictMode,
		})

		if err := tmpl.ExecuteTemplate(w, "endpoint.html", pageData); err != nil {
			logger.Error("failed to render template", "template", "endpoint.html", "error", err)
			http.Error(w, "failed to render page", http.StatusInternalServerError)
			return
		}
	}
}

func handleDeleteEndpoint(store EndpointStore, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !validateCSRF(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		rawID := r.PathValue("id")

		id, err := uuid.Parse(rawID)
		if err != nil {
			http.Error(w, "invalid endpoint ID", http.StatusBadRequest)
			return
		}

		removed := store.RemoveByID(id)
		if !removed {
			http.Error(w, "endpoint not found", http.StatusNotFound)
			return
		}

		logger.Info("endpoint removed", "id", id)

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func handleRefreshEndpoint(store EndpointStore, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !validateCSRF(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		rawID := r.PathValue("id")

		id, err := uuid.Parse(rawID)
		if err != nil {
			http.Error(w, "invalid endpoint ID", http.StatusBadRequest)
			return
		}

		result, err := store.Refresh(id)

		switch {
		case err == nil:
			logger.Info(
				"endpoint refreshed",
				"id", id,
				"status_code", result.HTTP.StatusCode,
				"available", result.HTTP.Available,
				"latency", result.HTTP.Latency,
			)

		case errors.Is(err, endpoint.ErrEndpointNotFound):
			http.Error(w, "endpoint not found", http.StatusNotFound)
			return

		default:
			logger.Warn("endpoint refresh failed", "id", id, "error", err)
		}

		http.Redirect(w, r, "/endpoints/"+id.String(), http.StatusSeeOther)
	}
}
