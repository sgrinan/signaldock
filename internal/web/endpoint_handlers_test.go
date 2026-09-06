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
)

type fakeEndpointService struct {
	addFunc        func(string) (endpoint.Endpoint, error)
	listFunc       func() []endpoint.Endpoint
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

func (f *fakeEndpointService) List() []endpoint.Endpoint {
	if f.listFunc != nil {
		return f.listFunc()
	}

	return nil
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
				listFunc: func() []endpoint.Endpoint {
					return []endpoint.Endpoint{
						{ID: uuid.NewV7()},
						{ID: uuid.NewV7()},
					}
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

			if recorder.Code != http.StatusOK {
				t.Errorf("handleGetIndex() status = %d, want %d", recorder.Code, http.StatusOK)
			}

			want := tt.want + "|2|csrf"

			if got := recorder.Body.String(); got != want {
				t.Errorf("handleGetIndex() body = %q, want %q", got, want)
			}
		})
	}
}

func TestHandler_HandlePostEndpoint(t *testing.T) {
	httpErr := errors.New("http probe failed")
	tlsErr := errors.New("tls probe failed")

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
				"url": []string{"https://example.com"},
			},
			)

			recorder := httptest.NewRecorder()

			h.handlePostEndpoint(recorder, req)

			if gotRawURL != "https://example.com" {
				t.Errorf("Add() rawURL = %q, want %q", gotRawURL, "https://example.com")
			}

			if recorder.Code != tt.wantStatus {
				t.Errorf("handlePostEndpoint() status = %d, want %d", recorder.Code, tt.wantStatus)
			}

			if got := recorder.Header().Get("Location"); got != tt.wantLocation {
				t.Errorf("handlePostEndpoint() Location = %q, want %q", got, tt.wantLocation)
			}
		})
	}
}

func TestHandler_HandlePostEndpoint_InvalidCSRF(t *testing.T) {
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

	if recorder.Code != http.StatusForbidden {
		t.Errorf("handlePostEndpoint() status = %d, want %d", recorder.Code, http.StatusForbidden)
	}

	if called {
		t.Error("handlePostEndpoint() called Add() with invalid CSRF token")
	}
}

func TestHandler_HandleGetEndpoint(t *testing.T) {
	id := uuid.NewV7()

	ep := endpoint.Endpoint{
		ID:  id,
		URL: "https://example.com/",
	}

	ep.LastCheck.HTTP.Latency = 123 * time.Millisecond
	ep.LastCheck.HTTP.CheckedAt = time.Date(2026, time.September, 6, 15, 4, 5, 0, time.UTC)
	ep.LastCheck.TLS.ExpiresAt = time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)

	service := &fakeEndpointService{
		byIDFunc: func(gotID uuid.UUID) (endpoint.Endpoint, error) {
			if gotID != id {
				t.Errorf("ByID() id = %v, want %v", gotID, id)
			}

			return ep, nil
		},
	}

	h := newTestHandler(service)

	req := httptest.NewRequest(http.MethodGet, "/endpoints/"+id.String(), nil)
	req.SetPathValue("id", id.String())

	recorder := httptest.NewRecorder()

	h.handleGetEndpoint(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("handleGetEndpoint() status = %d, want %d", recorder.Code, http.StatusOK)
	}

	want := "https://example.com/|123|15:04:05|01 Oct 2026|csrf"

	if got := recorder.Body.String(); got != want {
		t.Errorf("handleGetEndpoint() body = %q, want %q", got, want)
	}
}

func TestHandler_HandleGetEndpoint_Error(t *testing.T) {
	t.Run("invalid_id", func(t *testing.T) {
		h := newTestHandler(&fakeEndpointService{})

		req := httptest.NewRequest(http.MethodGet, "/endpoints/invalid", nil)
		req.SetPathValue("id", "invalid")

		recorder := httptest.NewRecorder()

		h.handleGetEndpoint(recorder, req)

		if recorder.Code != http.StatusBadRequest {
			t.Errorf("handleGetEndpoint() status = %d, want %d", recorder.Code, http.StatusBadRequest)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		id := uuid.NewV7()

		service := &fakeEndpointService{
			byIDFunc: func(uuid.UUID) (endpoint.Endpoint, error) {
				return endpoint.Endpoint{}, endpoint.ErrEndpointNotFound
			},
		}

		h := newTestHandler(service)

		req := httptest.NewRequest(http.MethodGet, "/endpoints/"+id.String(), nil)
		req.SetPathValue("id", id.String())

		recorder := httptest.NewRecorder()

		h.handleGetEndpoint(recorder, req)

		if recorder.Code != http.StatusNotFound {
			t.Errorf("handleGetEndpoint() status = %d, want %d", recorder.Code, http.StatusNotFound)
		}
	})

	t.Run("unexpected_error", func(t *testing.T) {
		id := uuid.NewV7()

		service := &fakeEndpointService{
			byIDFunc: func(uuid.UUID) (endpoint.Endpoint, error) {
				return endpoint.Endpoint{}, errors.New("unexpected error")
			},
		}

		h := newTestHandler(service)

		req := httptest.NewRequest(http.MethodGet, "/endpoints/"+id.String(), nil)
		req.SetPathValue("id", id.String())

		recorder := httptest.NewRecorder()

		h.handleGetEndpoint(recorder, req)

		if recorder.Code != http.StatusInternalServerError {
			t.Errorf("handleGetEndpoint() status = %d, want %d", recorder.Code, http.StatusInternalServerError)
		}
	})
}

func TestHandler_HandleDeleteEndpoint(t *testing.T) {
	id := uuid.NewV7()

	tests := []struct {
		name       string
		id         string
		removeErr  error
		wantStatus int
		want       string
	}{
		{
			name:       "success",
			id:         id.String(),
			wantStatus: http.StatusSeeOther,
			want:       "/",
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

			if recorder.Code != tt.wantStatus {
				t.Errorf("handleDeleteEndpoint() status = %d, want %d", recorder.Code, tt.wantStatus)
			}

			if got := recorder.Header().Get("Location"); got != tt.want {
				t.Errorf("handleDeleteEndpoint() Location = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHandler_HandleDeleteEndpoint_InvalidCSRF(t *testing.T) {
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

	if recorder.Code != http.StatusForbidden {
		t.Errorf("handleDeleteEndpoint() status = %d, want %d", recorder.Code, http.StatusForbidden)
	}

	if called {
		t.Error("handleDeleteEndpoint() called RemoveByID() with invalid CSRF token")
	}
}

func TestHandler_HandleRefreshEndpoint(t *testing.T) {
	id := uuid.NewV7()

	checkErr := endpoint.CheckError{
		HTTP: errors.New("http failed"),
	}

	tests := []struct {
		name       string
		id         string
		refreshErr error
		wantStatus int
		want       string
	}{
		{
			name:       "success",
			id:         id.String(),
			wantStatus: http.StatusSeeOther,
			want:       "/endpoints/" + id.String(),
		},
		{
			name:       "check_error",
			id:         id.String(),
			refreshErr: checkErr,
			wantStatus: http.StatusSeeOther,
			want:       "/endpoints/" + id.String(),
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

			if recorder.Code != tt.wantStatus {
				t.Errorf("handleRefreshEndpoint() status = %d, want %d", recorder.Code, tt.wantStatus)
			}

			if got := recorder.Header().Get("Location"); got != tt.want {
				t.Errorf("handleRefreshEndpoint() Location = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHandler_HandleRefreshEndpoint_InvalidCSRF(t *testing.T) {
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

	if recorder.Code != http.StatusForbidden {
		t.Errorf("handleRefreshEndpoint() status = %d, want %d", recorder.Code, http.StatusForbidden)
	}

	if called {
		t.Error("handleRefreshEndpoint() called Refresh() with invalid CSRF token")
	}
}

func newTestHandler(service endpointService) *handler {
	tmpl := template.Must(
		template.New("index.html").Parse(
			`{{.Error}}|{{len .Endpoints}}|{{if .CSRFToken}}csrf{{end}}`,
		),
	)

	template.Must(
		tmpl.New("endpoint.html").Parse(
			`{{.Endpoint.URL}}|{{.LatencyMS}}|{{.LastCheckedAt}}|{{.TLSExpiresAt}}|{{if .CSRFToken}}csrf{{end}}`,
		),
	)

	return &handler{
		endpoints: service,
		templates: tmpl,
		logger: slog.New(
			slog.NewTextHandler(io.Discard, nil),
		),
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
