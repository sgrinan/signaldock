package web

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"uuid"

	"github.com/sgrinan/signaldock/internal/endpoint"
	"github.com/sgrinan/signaldock/internal/probe"
)

type fakeStore struct {
	endpoints []endpoint.Endpoint
}

func (store *fakeStore) List() []endpoint.Endpoint {
	return store.endpoints
}

func (store *fakeStore) GetByID(id uuid.UUID) (endpoint.Endpoint, bool) {
	for _, endpoint := range store.endpoints {
		if endpoint.ID == id {
			return endpoint, true
		}
	}
	return endpoint.Endpoint{}, false
}

func (store *fakeStore) RemoveByID(id uuid.UUID) bool {
	for i, ep := range store.endpoints {
		if ep.ID == id {
			store.endpoints = slices.Delete(store.endpoints, i, i+1)
			return true
		}
	}
	return false
}

func (store *fakeStore) Add(rawURL string) (endpoint.Endpoint, probe.Result, error) {
	ep := endpoint.Endpoint{
		ID:  uuid.NewV7(),
		URL: rawURL,
	}

	store.endpoints = append(store.endpoints, ep)

	return ep, probe.Result{
		StatusCode: http.StatusOK,
		Available:  true,
	}, nil
}

func TestGetIndex(t *testing.T) {
	wantURL := "https://example.com/"

	store := &fakeStore{
		endpoints: []endpoint.Endpoint{
			{
				URL: wantURL,
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler, err := NewHandler(store, logger)
	if err != nil {
		t.Fatalf("NewHandler() unexpected error = %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("GET / status = %d, want %d", recorder.Code, http.StatusOK)
	}

	if !strings.Contains(recorder.Body.String(), wantURL) {
		t.Errorf("GET / body = %q, want endpoint URL %q", recorder.Body.String(), wantURL)
	}
}

func TestPostEndpoint(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("valid CSRF token", func(t *testing.T) {
		store := &fakeStore{}

		handler, err := NewHandler(store, logger)
		if err != nil {
			t.Fatalf("NewHandler() unexpected error = %v", err)
		}

		wantURL := "https://example.com/"
		csrfToken := "test-csrf-token"

		form := url.Values{}
		form.Set("url", wantURL)
		form.Set("csrf_token", csrfToken)

		request := httptest.NewRequest(http.MethodPost, "/endpoints", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		request.AddCookie(&http.Cookie{
			Name:  "csrf_token",
			Value: csrfToken,
		})

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

		if store.List()[0].URL != wantURL {
			t.Errorf("POST /endpoints URL = %q, want %q", store.List()[0].URL, wantURL)
		}
	})

	t.Run("missing CSRF token", func(t *testing.T) {
		store := &fakeStore{}

		handler, err := NewHandler(store, logger)
		if err != nil {
			t.Fatalf("NewHandler() unexpected error = %v", err)
		}

		form := url.Values{}
		form.Set("url", "https://example.com/")

		request := httptest.NewRequest(http.MethodPost, "/endpoints", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusForbidden {
			t.Errorf("POST /endpoints status = %d, want %d", recorder.Code, http.StatusForbidden)
		}
	})

	t.Run("mismatched CSRF token", func(t *testing.T) {
		store := &fakeStore{}

		handler, err := NewHandler(store, logger)
		if err != nil {
			t.Fatalf("NewHandler() unexpected error = %v", err)
		}

		form := url.Values{}
		form.Set("url", "https://example.com/")
		form.Set("csrf_token", "form-token")

		request := httptest.NewRequest(http.MethodPost, "/endpoints", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		request.AddCookie(&http.Cookie{
			Name:  "csrf_token",
			Value: "cookie-token",
		})

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusForbidden {
			t.Errorf("POST /endpoints status = %d, want %d", recorder.Code, http.StatusForbidden)
		}
	})
}

func TestGetEndpoint(t *testing.T) {
	id := uuid.NewV7()
	wantURL := "https://example.com/"

	store := &fakeStore{
		endpoints: []endpoint.Endpoint{
			{
				ID:  id,
				URL: wantURL,
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler, err := NewHandler(store, logger)
	if err != nil {
		t.Fatalf("NewHandler() unexpected error = %v", err)
	}

	t.Run("existing endpoint", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/endpoints/"+id.String(), nil)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("GET /endpoints/{id} status = %d, want %d", recorder.Code, http.StatusOK)
		}

		if !strings.Contains(recorder.Body.String(), wantURL) {
			t.Errorf("GET /endpoints/{id} body = %q, want endpoint URL %q", recorder.Body.String(), wantURL)
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

func TestPostDeleteEndpoint(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("existing endpoint", func(t *testing.T) {
		id := uuid.NewV7()
		otherID := uuid.NewV7()

		store := &fakeStore{
			endpoints: []endpoint.Endpoint{
				{
					ID:  id,
					URL: "https://example.com/",
				},
				{
					ID:  otherID,
					URL: "https://second.com/",
				},
			},
		}

		handler, err := NewHandler(store, logger)
		if err != nil {
			t.Fatalf("NewHandler() unexpected error = %v", err)
		}

		csrfToken := "test-csrf-token"

		form := url.Values{}
		form.Set("csrf_token", csrfToken)

		request := httptest.NewRequest(http.MethodPost, "/endpoints/"+id.String()+"/delete", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		request.AddCookie(&http.Cookie{
			Name:  "csrf_token",
			Value: csrfToken,
		})

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusSeeOther {
			t.Errorf("POST /endpoints/{id}/delete status = %d, want %d", recorder.Code, http.StatusSeeOther)
		}

		if location := recorder.Header().Get("Location"); location != "/" {
			t.Errorf("POST /endpoints/{id}/delete Location = %q, want %q", location, "/")
		}

		if len(store.List()) != 1 {
			t.Fatalf("POST /endpoints/{id}/delete store size = %d, want 1", len(store.List()))
		}

		if store.List()[0].ID != otherID {
			t.Errorf("POST /endpoints/{id}/delete remaining ID = %v, want %v", store.List()[0].ID, otherID)
		}
	})

	t.Run("invalid ID", func(t *testing.T) {
		store := &fakeStore{}

		handler, err := NewHandler(store, logger)
		if err != nil {
			t.Fatalf("NewHandler() unexpected error = %v", err)
		}

		csrfToken := "test-csrf-token"

		form := url.Values{}
		form.Set("csrf_token", csrfToken)

		request := httptest.NewRequest(http.MethodPost, "/endpoints/not-a-uuid/delete", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		request.AddCookie(&http.Cookie{
			Name:  "csrf_token",
			Value: csrfToken,
		})

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Errorf("POST /endpoints/not-a-uuid/delete status = %d, want %d", recorder.Code, http.StatusBadRequest)
		}
	})

	t.Run("endpoint not found", func(t *testing.T) {
		store := &fakeStore{}

		handler, err := NewHandler(store, logger)
		if err != nil {
			t.Fatalf("NewHandler() unexpected error = %v", err)
		}

		csrfToken := "test-csrf-token"
		missingID := uuid.NewV7()

		form := url.Values{}
		form.Set("csrf_token", csrfToken)

		request := httptest.NewRequest(http.MethodPost, "/endpoints/"+missingID.String()+"/delete", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		request.AddCookie(&http.Cookie{
			Name:  "csrf_token",
			Value: csrfToken,
		})

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusNotFound {
			t.Errorf("POST /endpoints/{id}/delete status = %d, want %d", recorder.Code, http.StatusNotFound)
		}
	})

	t.Run("missing CSRF token", func(t *testing.T) {
		id := uuid.NewV7()

		store := &fakeStore{
			endpoints: []endpoint.Endpoint{
				{
					ID:  id,
					URL: "https://example.com/",
				},
			},
		}

		handler, err := NewHandler(store, logger)
		if err != nil {
			t.Fatalf("NewHandler() unexpected error = %v", err)
		}

		request := httptest.NewRequest(http.MethodPost, "/endpoints/"+id.String()+"/delete", nil)

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusForbidden {
			t.Errorf("POST /endpoints/{id}/delete status = %d, want %d", recorder.Code, http.StatusForbidden)
		}
	})

	t.Run("mismatched CSRF token", func(t *testing.T) {
		id := uuid.NewV7()

		store := &fakeStore{
			endpoints: []endpoint.Endpoint{
				{
					ID:  id,
					URL: "https://example.com/",
				},
			},
		}

		handler, err := NewHandler(store, logger)
		if err != nil {
			t.Fatalf("NewHandler() unexpected error = %v", err)
		}

		form := url.Values{}
		form.Set("csrf_token", "form-token")

		request := httptest.NewRequest(http.MethodPost, "/endpoints/"+id.String()+"/delete", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		request.AddCookie(&http.Cookie{
			Name:  "csrf_token",
			Value: "cookie-token",
		})

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusForbidden {
			t.Errorf("POST /endpoints/{id}/delete status = %d, want %d", recorder.Code, http.StatusForbidden)
		}
	})
}

func TestSecurityHeaders(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := securityHeaders(next)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("securityHeaders status = %d, want %d", recorder.Code, http.StatusOK)
	}

	if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want %q", got, "nosniff")
	}

	if got := recorder.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("Referrer-Policy = %q, want %q", got, "no-referrer")
	}

	wantCSP := "default-src 'self'; frame-ancestors 'none'"

	if got := recorder.Header().Get("Content-Security-Policy"); got != wantCSP {
		t.Errorf("Content-Security-Policy = %q, want %q", got, wantCSP)
	}
}
