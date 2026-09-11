package web

import (
	"net/http"
	"testing"
	"time"

	"uuid"

	"github.com/sgrinan/signaldock/internal/endpoint"
	"github.com/sgrinan/signaldock/internal/probe"
)

func TestNewEndpointListItem(t *testing.T) {
	checkedAt := time.Date(2026, time.September, 6, 18, 30, 0, 0, time.UTC)

	tests := []struct {
		name string
		ep   endpoint.Endpoint
		want endpointListItem
	}{
		{
			name: "checking",
			ep: endpoint.Endpoint{
				ID:  uuid.NewV7(),
				URL: "https://example.com/",
			},
			want: endpointListItem{
				State:      "Checking",
				StateClass: "status-pending",
				HTTPStatus: "—",
				HTTPState:  "Checking",
				Latency:    "—",
				TLSState:   "Checking",
			},
		},
		{
			name: "responding_https_valid",
			ep: endpoint.Endpoint{
				ID:  uuid.NewV7(),
				URL: "https://example.com/",
				LastCheck: endpoint.CheckResult{
					HTTP: probe.HTTPResult{
						StatusCode: http.StatusOK,
						Latency:    125 * time.Millisecond,
						Responded:  true,
						CheckedAt:  checkedAt,
					},
					TLS: probe.TLSResult{
						Enabled:       true,
						Valid:         true,
						DaysRemaining: 30,
						ExpiresAt:     checkedAt.Add(30 * 24 * time.Hour),
					},
				},
			},
			want: endpointListItem{
				State:            "Responding",
				StateClass:       "status-ok",
				HTTPStatus:       "200",
				HTTPState:        "Responded",
				Latency:          "125 ms",
				LastCheckedAt:    checkedAt,
				TLSState:         "Valid",
				TLSDaysRemaining: "30 days remaining",
			},
		},
		{
			name: "no_http_response",
			ep: endpoint.Endpoint{
				ID:  uuid.NewV7(),
				URL: "https://example.com/",
				LastCheck: endpoint.CheckResult{
					HTTP: probe.HTTPResult{
						Responded: false,
						CheckedAt: checkedAt,
					},
					TLS: probe.TLSResult{
						Enabled: true,
						Valid:   true,
					},
				},
			},
			want: endpointListItem{
				State:         "No response",
				StateClass:    "status-error",
				HTTPStatus:    "—",
				HTTPState:     "No response",
				Latency:       "0 ms",
				LastCheckedAt: checkedAt,
				TLSState:      "Valid",
			},
		},
		{
			name: "invalid_tls",
			ep: endpoint.Endpoint{
				ID:  uuid.NewV7(),
				URL: "https://example.com/",
				LastCheck: endpoint.CheckResult{
					HTTP: probe.HTTPResult{
						StatusCode: http.StatusOK,
						Responded:  true,
						CheckedAt:  checkedAt,
					},
					TLS: probe.TLSResult{
						Enabled:       true,
						Valid:         false,
						ExpiresAt:     checkedAt.Add(10 * 24 * time.Hour),
						DaysRemaining: 10,
					},
				},
			},
			want: endpointListItem{
				State:            "TLS issue",
				StateClass:       "status-warning",
				HTTPStatus:       "200",
				HTTPState:        "Responded",
				Latency:          "0 ms",
				LastCheckedAt:    checkedAt,
				TLSState:         "Invalid",
				TLSDaysRemaining: "10 days remaining",
			},
		},
		{
			name: "http_without_tls",
			ep: endpoint.Endpoint{
				ID:  uuid.NewV7(),
				URL: "http://example.com/",
				LastCheck: endpoint.CheckResult{
					HTTP: probe.HTTPResult{
						StatusCode: http.StatusNoContent,
						Responded:  true,
						CheckedAt:  checkedAt,
					},
				},
			},
			want: endpointListItem{
				State:         "Responding",
				StateClass:    "status-ok",
				HTTPStatus:    "204",
				HTTPState:     "Responded",
				Latency:       "0 ms",
				LastCheckedAt: checkedAt,
				TLSState:      "Not enabled",
			},
		},
		{
			name: "tls_information_unavailable",
			ep: endpoint.Endpoint{
				ID:  uuid.NewV7(),
				URL: "https://example.com/",
				LastCheck: endpoint.CheckResult{
					HTTP: probe.HTTPResult{
						StatusCode: http.StatusOK,
						Responded:  true,
						CheckedAt:  checkedAt,
					},
					TLS: probe.TLSResult{
						Enabled: true,
						Valid:   false,
					},
				},
			},
			want: endpointListItem{
				State:         "TLS issue",
				StateClass:    "status-warning",
				HTTPStatus:    "200",
				HTTPState:     "Responded",
				Latency:       "0 ms",
				LastCheckedAt: checkedAt,
				TLSState:      "Unavailable",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := tt.want
			want.ID = tt.ep.ID
			want.URL = tt.ep.URL

			got := newEndpointListItem(tt.ep)

			if got != want {
				t.Errorf("newEndpointListItem() = %+v, want %+v", got, want)
			}
		})
	}
}

func TestNewEndpointListItems(t *testing.T) {
	first := endpoint.Endpoint{
		ID:  uuid.NewV7(),
		URL: "https://first.example/",
	}

	second := endpoint.Endpoint{
		ID:  uuid.NewV7(),
		URL: "https://second.example/",
	}

	got := newEndpointListItems(
		[]endpoint.Endpoint{
			first,
			second,
		},
	)

	if gotLen, wantLen := len(got), 2; gotLen != wantLen {
		t.Fatalf("len(newEndpointListItems()) = %d, want %d", gotLen, wantLen)
	}

	if got[0].ID != first.ID {
		t.Errorf("newEndpointListItems()[0].ID = %v, want %v", got[0].ID, first.ID)
	}

	if got[1].ID != second.ID {
		t.Errorf("newEndpointListItems()[1].ID = %v, want %v", got[1].ID, second.ID)
	}
}

func TestTLSExpiryClass(t *testing.T) {
	tests := []struct {
		name string
		days int
		want string
	}{
		{
			name: "expired",
			days: -1,
			want: "status-error",
		},
		{
			name: "seven_days",
			days: 7,
			want: "status-error",
		},
		{
			name: "eight_days",
			days: 8,
			want: "status-warning",
		},
		{
			name: "thirty_days",
			days: 30,
			want: "status-warning",
		},
		{
			name: "more_than_thirty_days",
			days: 31,
			want: "status-ok",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tlsExpiryClass(tt.days); got != tt.want {
				t.Errorf("tlsExpiryClass(%d) = %q, want %q", tt.days, got, tt.want)
			}
		})
	}
}
