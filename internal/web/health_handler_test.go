package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	recorder := httptest.NewRecorder()

	handleHealth(recorder, req)

	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Errorf("handleHealth() status = %d, want %d", got, want)
	}
}

func TestHandleReady(t *testing.T) {
	tests := []struct {
		name       string
		pingErr    error
		wantStatus int
	}{
		{
			name:       "ready",
			pingErr:    nil,
			wantStatus: http.StatusOK,
		},
		{
			name:       "database unavailable",
			pingErr:    errors.New("database unavailable"),
			wantStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readiness := &fakeReadinessChecker{
				pingFunc: func(context.Context) error {
					return tt.pingErr
				},
			}

			h := &handler{
				readiness: readiness,
			}

			req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			recorder := httptest.NewRecorder()

			h.handleReady(recorder, req)

			if got := recorder.Code; got != tt.wantStatus {
				t.Errorf("handleReady() status = %d, want %d", got, tt.wantStatus)
			}
		})
	}
}
