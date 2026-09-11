package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"uuid"

	"github.com/sgrinan/signaldock/internal/endpoint"
)

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
