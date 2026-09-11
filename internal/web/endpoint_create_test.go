package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"uuid"

	"github.com/sgrinan/signaldock/internal/endpoint"
	"github.com/sgrinan/signaldock/internal/probe"
)

func TestHandler_HandlePostEndpoint(t *testing.T) {
	const rawURL = "https://example.com"

	httpErr := errors.New("http probe failed")
	tlsErr := errors.New("tls probe failed")
	hostErr := errors.New("host check failed")

	added := endpoint.Endpoint{
		ID:  uuid.NewV7(),
		URL: "https://example.com/",
	}

	tests := []struct {
		name         string
		err          error
		wantStatus   int
		wantLocation string
	}{
		{
			name:         "success",
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/",
		},
		{
			name:         "duplicate",
			err:          endpoint.ErrEndpointExists,
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/?result=duplicate",
		},
		{
			name:         "invalid_url",
			err:          endpoint.ErrInvalidURL,
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/?result=invalid",
		},
		{
			name:         "unsupported_scheme",
			err:          endpoint.ErrUnsupportedScheme,
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/?result=invalid",
		},
		{
			name:         "missing_host",
			err:          endpoint.ErrHostRequired,
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/?result=invalid",
		},
		{
			name:         "credentials",
			err:          endpoint.ErrCredentialsNotAllowed,
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/?result=invalid",
		},
		{
			name:         "invalid_port",
			err:          endpoint.ErrInvalidPort,
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/?result=invalid",
		},
		{
			name:         "unsafe_host",
			err:          endpoint.ErrUnsafeHost,
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/?result=unsafe",
		},
		{
			name: "host_check_error",
			err: endpoint.CheckError{
				Host: hostErr,
			},
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/?result=unreachable",
		},
		{
			name: "http_check_error",
			err: endpoint.CheckError{
				HTTP: httpErr,
			},
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/?result=unreachable",
		},
		{
			name: "tls_check_error",
			err: endpoint.CheckError{
				TLS: tlsErr,
			},
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/?result=tls-invalid",
		},
		{
			name: "http_and_tls_check_error",
			err: endpoint.CheckError{
				HTTP: httpErr,
				TLS:  tlsErr,
			},
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/?result=check-failed",
		},
		{
			name:       "empty_check_error",
			err:        endpoint.CheckError{},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "unexpected_error",
			err:        errors.New("unexpected error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotRawURL string

			service := &fakeEndpointService{
				addFunc: func(ctx context.Context, rawURL string) (endpoint.Endpoint, error) {
					gotRawURL = rawURL

					return added, tt.err
				},
			}

			h := newTestHandler(service)

			req := newCSRFPostRequest(t, "/endpoints", url.Values{
				"url": []string{rawURL},
			},
			)

			recorder := httptest.NewRecorder()

			h.handlePostEndpoint(recorder, req)

			if gotRawURL != rawURL {
				t.Errorf("Add() rawURL = %q, want %q", gotRawURL, rawURL)
			}

			if got := recorder.Code; got != tt.wantStatus {
				t.Errorf("handlePostEndpoint() status = %d, want %d", got, tt.wantStatus)
			}

			if got := recorder.Header().Get("Location"); got != tt.wantLocation {
				t.Errorf("handlePostEndpoint() Location = %q, want %q", got, tt.wantLocation)
			}
		})
	}

	t.Run("invalid_tls_certificate", func(t *testing.T) {
		added := endpoint.Endpoint{
			ID:  uuid.NewV7(),
			URL: "https://example.com/",
			LastCheck: endpoint.CheckResult{
				TLS: probe.TLSResult{
					Enabled:       true,
					Valid:         false,
					ExpiresAt:     time.Now().Add(30 * 24 * time.Hour),
					DaysRemaining: 30,
				},
			},
		}

		service := &fakeEndpointService{
			addFunc: func(context.Context, string) (endpoint.Endpoint, error) {
				return added, endpoint.CheckError{
					HTTP: httpErr,
					TLS:  tlsErr,
				}
			},
		}

		h := newTestHandler(service)

		req := newCSRFPostRequest(t, "/endpoints", url.Values{
			"url": []string{rawURL},
		},
		)

		recorder := httptest.NewRecorder()

		h.handlePostEndpoint(recorder, req)

		if got, want := recorder.Code, http.StatusSeeOther; got != want {
			t.Errorf("handlePostEndpoint() status = %d, want %d", got, want)
		}

		if got, want := recorder.Header().Get("Location"), "/?result=tls-invalid"; got != want {
			t.Errorf("handlePostEndpoint() Location = %q, want %q", got, want)
		}
	})

	t.Run("invalid_csrf", func(t *testing.T) {
		called := false

		service := &fakeEndpointService{
			addFunc: func(context.Context, string) (endpoint.Endpoint, error) {
				called = true

				return endpoint.Endpoint{}, nil
			},
		}

		h := newTestHandler(service)

		req := httptest.NewRequest(http.MethodPost, "/endpoints", strings.NewReader("url=https%3A%2F%2Fexample.com"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		recorder := httptest.NewRecorder()

		h.handlePostEndpoint(recorder, req)

		if got, want := recorder.Code, http.StatusForbidden; got != want {
			t.Errorf("handlePostEndpoint() status = %d, want %d", got, want)
		}

		if called {
			t.Error("handlePostEndpoint() called Add() with invalid CSRF token")
		}
	})
}
