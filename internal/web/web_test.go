package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sgrinan/signaldock/internal/endpoint"
)

func TestNewHandler(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer server.Close()

	store := endpoint.Store{}

	_, _, err := store.Add(server.URL)
	if err != nil {
		t.Fatalf("Add() unexpected error = %v", err)
	}

	handler, err := NewHandler(&store)
	if err != nil {
		t.Fatalf("NewHandler() unexpected error = %v", err)
	}

	request := httptest.NewRequest("GET", "/", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("NewHandler() = %d, want 200", recorder.Code)
	}

	if !strings.Contains(recorder.Body.String(), server.URL) {
		t.Errorf("NewHandler() body = %q, want endpoint URL", recorder.Body.String())
	}
}
