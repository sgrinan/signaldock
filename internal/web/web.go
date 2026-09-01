package web

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/sgrinan/signaldock/internal/endpoint"
)

func NewHandler(store *endpoint.Store) (*http.ServeMux, error) {
	mux := http.NewServeMux()

	tmpl, err := template.ParseFiles("/home/rat/projects/signaldock/internal/web/templates/index.html")
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if err := tmpl.Execute(w, store.List()); err != nil {
			http.Error(w, "failed to render page", http.StatusInternalServerError)
		}
	})

	return mux, nil
}
