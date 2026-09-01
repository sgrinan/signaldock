package web

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"

	"github.com/sgrinan/signaldock/internal/endpoint"
)

//go:embed templates/index.html
var templates embed.FS

func NewHandler(store *endpoint.Store) (*http.ServeMux, error) {
	mux := http.NewServeMux()

	tmpl, err := template.ParseFS(templates, "templates/index.html")
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if err := tmpl.Execute(w, store.List()); err != nil {
			http.Error(w, "failed to render page", http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("POST /endpoints", func(w http.ResponseWriter, r *http.Request) {
		rawURL := r.FormValue("url")

		_, _, err := store.Add(rawURL)
		if err != nil {
			return
		}

		http.Redirect(w, r, "GET /", http.StatusSeeOther)
	})

	return mux, nil
}
