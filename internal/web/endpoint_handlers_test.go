package web

import (
	"errors"
	"html/template"
	"io"
	"log/slog"
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

// handler.handleGetIndex

func TestHandler_HandleGetIndex(t *testing.T) {
	tests := []struct {
		name   string
		result string
		want   string
	}{
		{
			name: "success",
		},
		{
			name:   "unreachable",
			result: "unreachable",
			want:   "Endpoint was added, but it is currently unreachable",
		},
		{
			name:   "duplicate",
			result: "duplicate",
			want:   "Endpoint already exists",
		},
		{
			name:   "invalid",
			result: "invalid",
			want:   "Enter a valid HTTP or HTTPS URL",
		},
		{
			name:   "unsafe",
			result: "unsafe",
			want:   "Private or unsafe network destinations are not allowed",
		},
		{
			name:   "tls_invalid",
			result: "tls-invalid",
			want:   "Endpoint was added, but its TLS certificate is invalid",
		},
		{
			name:   "check_failed",
			result: "check-failed",
			want:   "Endpoint was added, but its HTTP and TLS checks failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeEndpointService{
				listFunc: func() ([]endpoint.Endpoint, error) {
					return []endpoint.Endpoint{
						{ID: uuid.NewV7()},
						{ID: uuid.NewV7()},
					}, nil
				},
			}

			h := newTestHandler(service)

			target := "/"

			if tt.result != "" {
				target = "/?result=" + url.QueryEscape(tt.result)
			}

			req := httptest.NewRequest(http.MethodGet, target, nil)
			recorder := httptest.NewRecorder()

			h.handleGetIndex(recorder, req)

			if got, want := recorder.Code, http.StatusOK; got != want {
				t.Errorf("handleGetIndex() status = %d, want %d", got, want)
			}

			want := tt.want + "|2|csrf"

			if got := recorder.Body.String(); got != want {
				t.Errorf("handleGetIndex() body = %q, want %q", got, want)
			}
		})
	}
}

// handler.handlePostEndpoint

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
				addFunc: func(rawURL string) (endpoint.Endpoint, error) {
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
			addFunc: func(string) (endpoint.Endpoint, error) {
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
			addFunc: func(string) (endpoint.Endpoint, error) {
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

// handler.handleGetEndpoint

func TestHandler_HandleGetEndpoint(t *testing.T) {
	id := uuid.NewV7()

	checkedEndpoint := endpoint.Endpoint{
		ID:  id,
		URL: "https://example.com/",
		LastCheck: endpoint.CheckResult{
			HTTP: probe.HTTPResult{
				StatusCode: http.StatusOK,
				Latency:    123 * time.Millisecond,
				Responded:  true,
				CheckedAt:  time.Date(2026, time.September, 6, 15, 4, 5, 0, time.UTC),
			},
			TLS: probe.TLSResult{
				Enabled:       true,
				Valid:         true,
				ExpiresAt:     time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC),
				DaysRemaining: 25,
			},
		},
	}

	checkingEndpoint := endpoint.Endpoint{
		ID:  id,
		URL: "https://example.com/",
	}

	tests := []struct {
		name       string
		pathID     string
		ep         endpoint.Endpoint
		serviceErr error
		wantStatus int
		wantBody   string
		wantByID   bool
	}{
		{
			name:       "success",
			pathID:     id.String(),
			ep:         checkedEndpoint,
			wantStatus: http.StatusOK,
			wantBody:   "https://example.com/|123|15:04:05|01 Oct 2026|Responding|status-ok|status-warning|false|csrf",
			wantByID:   true,
		},
		{
			name:       "checking",
			pathID:     id.String(),
			ep:         checkingEndpoint,
			wantStatus: http.StatusOK,
			wantBody:   "https://example.com/|0|||Checking|status-pending||true|csrf",
			wantByID:   true,
		},
		{
			name:       "invalid_id",
			pathID:     "invalid",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "not_found",
			pathID:     id.String(),
			serviceErr: endpoint.ErrEndpointNotFound,
			wantStatus: http.StatusNotFound,
			wantByID:   true,
		},
		{
			name:       "unexpected_error",
			pathID:     id.String(),
			serviceErr: errors.New("unexpected error"),
			wantStatus: http.StatusInternalServerError,
			wantByID:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false

			service := &fakeEndpointService{
				byIDFunc: func(gotID uuid.UUID) (endpoint.Endpoint, error) {
					called = true

					if gotID != id {
						t.Errorf("ByID() id = %v, want %v", gotID, id)
					}

					return tt.ep, tt.serviceErr
				},
			}

			h := newTestHandler(service)

			req := httptest.NewRequest(http.MethodGet, "/endpoints/"+tt.pathID, nil)
			req.SetPathValue("id", tt.pathID)

			recorder := httptest.NewRecorder()

			h.handleGetEndpoint(recorder, req)

			if got := recorder.Code; got != tt.wantStatus {
				t.Errorf("handleGetEndpoint() status = %d, want %d", got, tt.wantStatus)
			}

			if called != tt.wantByID {
				t.Errorf("handleGetEndpoint() called ByID() = %t, want %t", called, tt.wantByID)
			}

			if tt.wantBody != "" {
				if got := recorder.Body.String(); got != tt.wantBody {
					t.Errorf("handleGetEndpoint() body = %q, want %q", got, tt.wantBody)
				}
			}
		})
	}
}

// handler.handleDeleteEndpoint

func TestHandler_HandleDeleteEndpoint(t *testing.T) {
	id := uuid.NewV7()

	tests := []struct {
		name         string
		id           string
		removeErr    error
		wantStatus   int
		wantLocation string
	}{
		{
			name:         "success",
			id:           id.String(),
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/",
		},
		{
			name:       "invalid_id",
			id:         "invalid",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "not_found",
			id:         id.String(),
			removeErr:  endpoint.ErrEndpointNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "unexpected_error",
			id:         id.String(),
			removeErr:  errors.New("unexpected error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeEndpointService{
				removeByIDFunc: func(uuid.UUID) error {
					return tt.removeErr
				},
			}

			h := newTestHandler(service)

			req := newCSRFPostRequest(t, "/endpoints/"+tt.id+"/delete", nil)
			req.SetPathValue("id", tt.id)

			recorder := httptest.NewRecorder()

			h.handleDeleteEndpoint(recorder, req)

			if got := recorder.Code; got != tt.wantStatus {
				t.Errorf("handleDeleteEndpoint() status = %d, want %d", got, tt.wantStatus)
			}

			if got := recorder.Header().Get("Location"); got != tt.wantLocation {
				t.Errorf("handleDeleteEndpoint() Location = %q, want %q", got, tt.wantLocation)
			}
		})
	}

	t.Run("invalid_csrf", func(t *testing.T) {
		called := false

		service := &fakeEndpointService{
			removeByIDFunc: func(uuid.UUID) error {
				called = true

				return nil
			},
		}

		h := newTestHandler(service)

		req := httptest.NewRequest(http.MethodPost, "/endpoints/id/delete", nil)
		req.SetPathValue("id", uuid.NewV7().String())

		recorder := httptest.NewRecorder()

		h.handleDeleteEndpoint(recorder, req)

		if got, want := recorder.Code, http.StatusForbidden; got != want {
			t.Errorf("handleDeleteEndpoint() status = %d, want %d", got, want)
		}

		if called {
			t.Error("handleDeleteEndpoint() called RemoveByID() with invalid CSRF token")
		}
	})
}

// handler.handleRefreshEndpoint

func TestHandler_HandleRefreshEndpoint(t *testing.T) {
	id := uuid.NewV7()

	checkErr := endpoint.CheckError{
		HTTP: errors.New("http failed"),
	}

	tests := []struct {
		name         string
		id           string
		refreshErr   error
		wantStatus   int
		wantLocation string
	}{
		{
			name:         "success",
			id:           id.String(),
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/endpoints/" + id.String(),
		},
		{
			name:         "check_error",
			id:           id.String(),
			refreshErr:   checkErr,
			wantStatus:   http.StatusSeeOther,
			wantLocation: "/endpoints/" + id.String(),
		},
		{
			name:       "invalid_id",
			id:         "invalid",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "not_found",
			id:         id.String(),
			refreshErr: endpoint.ErrEndpointNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "unexpected_error",
			id:         id.String(),
			refreshErr: errors.New("unexpected error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeEndpointService{
				refreshFunc: func(uuid.UUID) (endpoint.CheckResult, error) {
					return endpoint.CheckResult{}, tt.refreshErr
				},
			}

			h := newTestHandler(service)

			req := newCSRFPostRequest(t, "/endpoints/"+tt.id+"/refresh", nil)
			req.SetPathValue("id", tt.id)

			recorder := httptest.NewRecorder()

			h.handleRefreshEndpoint(recorder, req)

			if got := recorder.Code; got != tt.wantStatus {
				t.Errorf("handleRefreshEndpoint() status = %d, want %d", got, tt.wantStatus)
			}

			if got := recorder.Header().Get("Location"); got != tt.wantLocation {
				t.Errorf("handleRefreshEndpoint() Location = %q, want %q", got, tt.wantLocation)
			}
		})
	}

	t.Run("invalid_csrf", func(t *testing.T) {
		called := false

		service := &fakeEndpointService{
			refreshFunc: func(uuid.UUID) (endpoint.CheckResult, error) {
				called = true

				return endpoint.CheckResult{}, nil
			},
		}

		h := newTestHandler(service)

		req := httptest.NewRequest(http.MethodPost, "/endpoints/id/refresh", nil)
		req.SetPathValue("id", uuid.NewV7().String())

		recorder := httptest.NewRecorder()

		h.handleRefreshEndpoint(recorder, req)

		if got, want := recorder.Code, http.StatusForbidden; got != want {
			t.Errorf("handleRefreshEndpoint() status = %d, want %d", got, want)
		}

		if called {
			t.Error("handleRefreshEndpoint() called Refresh() with invalid CSRF token")
		}
	})
}

// Test doubles

type fakeEndpointService struct {
	addFunc        func(string) (endpoint.Endpoint, error)
	listFunc       func() ([]endpoint.Endpoint, error)
	byIDFunc       func(uuid.UUID) (endpoint.Endpoint, error)
	removeByIDFunc func(uuid.UUID) error
	refreshFunc    func(uuid.UUID) (endpoint.CheckResult, error)
}

var _ endpointService = (*fakeEndpointService)(nil)

func (f *fakeEndpointService) Add(rawURL string) (endpoint.Endpoint, error) {
	if f.addFunc != nil {
		return f.addFunc(rawURL)
	}

	return endpoint.Endpoint{}, nil
}

func (f *fakeEndpointService) List() ([]endpoint.Endpoint, error) {
	if f.listFunc != nil {
		return f.listFunc()
	}

	return nil, nil
}

func (f *fakeEndpointService) ByID(id uuid.UUID) (endpoint.Endpoint, error) {
	if f.byIDFunc != nil {
		return f.byIDFunc(id)
	}

	return endpoint.Endpoint{}, endpoint.ErrEndpointNotFound
}

func (f *fakeEndpointService) RemoveByID(id uuid.UUID) error {
	if f.removeByIDFunc != nil {
		return f.removeByIDFunc(id)
	}

	return nil
}

func (f *fakeEndpointService) Refresh(id uuid.UUID) (endpoint.CheckResult, error) {
	if f.refreshFunc != nil {
		return f.refreshFunc(id)
	}

	return endpoint.CheckResult{}, nil
}

// Test helpers

func newTestHandler(service endpointService) *handler {
	tmpl := template.Must(
		template.New("index.html").Parse(
			`{{.Error}}|{{len .Endpoints}}|{{if .CSRFToken}}csrf{{end}}`,
		),
	)

	template.Must(
		tmpl.New("endpoint.html").Parse(
			`{{.Endpoint.URL}}|{{.LatencyMS}}|{{if not .LastCheckedAt.IsZero}}{{.LastCheckedAt.Format "15:04:05"}}{{end}}|{{.TLSExpiresAt}}|{{.State}}|{{.StateClass}}|{{.TLSDaysClass}}|{{.Checking}}|{{if .CSRFToken}}csrf{{end}}`,
		),
	)

	template.Must(
		tmpl.New("login.html").Parse(
			`{{.Error}}|{{if .CSRFToken}}csrf{{end}}`,
		),
	)

	return &handler{
		endpoints: service,
		users:     &fakeUserStore{},
		sessions:  &fakeSessionService{},
		templates: tmpl,
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func newCSRFPostRequest(t *testing.T, target string, form url.Values) *http.Request {
	t.Helper()

	tokenRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	tokenRecorder := httptest.NewRecorder()

	token, err := getCSRFToken(tokenRecorder, tokenRequest)
	if err != nil {
		t.Fatalf("getCSRFToken() returned unexpected error: %v", err)
	}

	if form == nil {
		form = url.Values{}
	} else {
		form = form.Clone()
	}

	form.Set(csrfFormField, token)

	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	cookies := tokenRecorder.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("getCSRFToken() set no cookie")
	}

	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	return req
}
