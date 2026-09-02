package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"uuid"

	"github.com/sgrinan/signaldock/internal/endpoint"
)

func TestGetIndex(t *testing.T) {
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

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("GET / status = %d, want %d", recorder.Code, http.StatusOK)
	}

	if !strings.Contains(recorder.Body.String(), server.URL) {
		t.Errorf("GET / body = %q, want endpoint URL %q", recorder.Body.String(), server.URL)
	}
}

func TestPostEndpoint(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer server.Close()

	store := endpoint.Store{}

	handler, err := NewHandler(&store)
	if err != nil {
		t.Fatalf("NewHandler() unexpected error = %v", err)
	}

	form := url.Values{}
	form.Set("url", server.URL)

	request := httptest.NewRequest(
		http.MethodPost,
		"/endpoints",
		strings.NewReader(form.Encode()),
	)

	request.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
	)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusSeeOther {
		t.Errorf("POST /endpoints status = %d, want %d", recorder.Code, http.StatusSeeOther)
	}

	if location := recorder.Header().Get("Location"); location != "/" {
		t.Errorf("POST /endpoints Location = %q, want %q", location, "/")
	}

	if len(store.List()) != 1 {
		t.Errorf("POST /endpoints store size = %d, want 1", len(store.List()))
	}
}

func TestGetEndpoint(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer server.Close()

	store := endpoint.Store{}

	added, _, err := store.Add(server.URL)
	if err != nil {
		t.Fatalf("Add() unexpected error = %v", err)
	}

	handler, err := NewHandler(&store)
	if err != nil {
		t.Fatalf("NewHandler() unexpected error = %v", err)
	}

	t.Run("existing endpoint", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/endpoints/"+added.ID.String(), nil)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("GET /endpoints/{id} status = %d, want %d", recorder.Code, http.StatusOK)
		}

		if !strings.Contains(recorder.Body.String(), added.URL) {
			t.Errorf("GET /endpoints/{id} body = %q, want endpoint URL %q", recorder.Body.String(), added.URL)
		}
	})

	t.Run("invalid ID", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/endpoints/not-a-uuid", nil)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Errorf("GET /endpoints/not-a-uuid status = %d, want %d", recorder.Code, http.StatusBadRequest)
		}
	})

	t.Run("endpoint not found", func(t *testing.T) {
		missingID := uuid.NewV7()
		request := httptest.NewRequest(http.MethodGet, "/endpoints/"+missingID.String(), nil)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusNotFound {
			t.Errorf("GET /endpoints/{id} status = %d, want %d", recorder.Code, http.StatusNotFound)
		}
	})
}
