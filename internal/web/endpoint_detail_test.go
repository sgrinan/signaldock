package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"uuid"

	"github.com/sgrinan/signaldock/internal/endpoint"
	"github.com/sgrinan/signaldock/internal/probe"
)

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
				byIDFunc: func(ctx context.Context, gotID uuid.UUID) (endpoint.Endpoint, error) {
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
