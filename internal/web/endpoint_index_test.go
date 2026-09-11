package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"uuid"

	"github.com/sgrinan/signaldock/internal/endpoint"
)

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
				listFunc: func(context.Context) ([]endpoint.Endpoint, error) {
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
