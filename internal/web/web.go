package web

import (
	"embed"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"uuid"

	"github.com/sgrinan/signaldock/internal/endpoint"
)

//go:embed templates/*.html
var templates embed.FS

type PageData struct {
	Endpoints []endpoint.Endpoint
	Error     string
}

type EndpointStore interface {
	Add(string) (endpoint.Endpoint, int, error)
	List() []endpoint.Endpoint
	GetByID(uuid.UUID) (endpoint.Endpoint, bool)
	RemoveByID(uuid.UUID) bool
}

func NewHandler(store EndpointStore, logger *slog.Logger) (*http.ServeMux, error) {
	mux := http.NewServeMux()

	tmpl, err := template.ParseFS(templates, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		result := r.URL.Query().Get("result")

		pageData := PageData{
			Endpoints: store.List(),
		}

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
	})

	mux.HandleFunc("POST /endpoints", func(w http.ResponseWriter, r *http.Request) {
		rawURL := r.FormValue("url")

		added, _, err := store.Add(rawURL)
		switch {
		case err == nil:
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return

		case errors.Is(err, endpoint.ErrEndpointExists):
			http.Redirect(w, r, "/?result=duplicate", http.StatusSeeOther)
			return

		case errors.Is(err, endpoint.ErrUnsupportedScheme),
			errors.Is(err, endpoint.ErrHostRequired):
			http.Redirect(w, r, "/?result=invalid", http.StatusSeeOther)
			return

		case errors.Is(err, endpoint.ErrUnsafeHost):
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
	})

	mux.HandleFunc("GET /endpoints/{id}", func(w http.ResponseWriter, r *http.Request) {
		rawID := r.PathValue("id")

		id, err := uuid.Parse(rawID)
		if err != nil {
			http.Error(w, "invalid endpoint ID", http.StatusBadRequest)
			return
		}

		endpoint, found := store.GetByID(id)
		if !found {
			http.Error(w, "endpoint not found", http.StatusNotFound)
			return
		}

		if err := tmpl.ExecuteTemplate(w, "endpoint.html", endpoint); err != nil {
			logger.Error("failed to render template", "template", "endpoint.html", "error", err)
			http.Error(w, "failed to render page", http.StatusInternalServerError)
			return
		}
	})

	mux.HandleFunc("POST /endpoints/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
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

		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	return mux, nil
}
