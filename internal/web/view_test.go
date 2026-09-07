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

		wantState            string
		wantStateClass       string
		wantHTTPStatus       string
		wantHTTPState        string
		wantLatency          string
		wantLastCheckedAt    string
		wantTLSState         string
		wantTLSDaysRemaining string
	}{
		{
			name: "checking",
			ep: endpoint.Endpoint{
				ID:  uuid.NewV7(),
				URL: "https://example.com/",
			},
			wantState:         "Checking",
			wantStateClass:    "status-pending",
			wantHTTPStatus:    "—",
			wantHTTPState:     "Checking",
			wantLatency:       "—",
			wantLastCheckedAt: "Not checked yet",
			wantTLSState:      "Checking",
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
			wantState:            "Responding",
			wantStateClass:       "status-ok",
			wantHTTPStatus:       "200",
			wantHTTPState:        "Responded",
			wantLatency:          "125 ms",
			wantLastCheckedAt:    "18:30:00",
			wantTLSState:         "Valid",
			wantTLSDaysRemaining: "30 days remaining",
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
			wantState:         "No response",
			wantStateClass:    "status-error",
			wantHTTPStatus:    "—",
			wantHTTPState:     "No response",
			wantLatency:       "0 ms",
			wantLastCheckedAt: "18:30:00",
			wantTLSState:      "Valid",
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
			wantState:            "TLS issue",
			wantStateClass:       "status-warning",
			wantHTTPStatus:       "200",
			wantHTTPState:        "Responded",
			wantLatency:          "0 ms",
			wantLastCheckedAt:    "18:30:00",
			wantTLSState:         "Invalid",
			wantTLSDaysRemaining: "10 days remaining",
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
			wantState:         "Responding",
			wantStateClass:    "status-ok",
			wantHTTPStatus:    "204",
			wantHTTPState:     "Responded",
			wantLatency:       "0 ms",
			wantLastCheckedAt: "18:30:00",
			wantTLSState:      "Not enabled",
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
			wantState:         "TLS issue",
			wantStateClass:    "status-warning",
			wantHTTPStatus:    "200",
			wantHTTPState:     "Responded",
			wantLatency:       "0 ms",
			wantLastCheckedAt: "18:30:00",
			wantTLSState:      "Unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newEndpointListItem(tt.ep)

			if got.State != tt.wantState {
				t.Errorf("State = %q, want %q", got.State, tt.wantState)
			}

			if got.StateClass != tt.wantStateClass {
				t.Errorf("StateClass = %q, want %q", got.StateClass, tt.wantStateClass)
			}

			if got.HTTPStatus != tt.wantHTTPStatus {
				t.Errorf("HTTPStatus = %q, want %q", got.HTTPStatus, tt.wantHTTPStatus)
			}

			if got.HTTPState != tt.wantHTTPState {
				t.Errorf("HTTPState = %q, want %q", got.HTTPState, tt.wantHTTPState)
			}

			if got.Latency != tt.wantLatency {
				t.Errorf("Latency = %q, want %q", got.Latency, tt.wantLatency)
			}

			if got.LastCheckedAt != tt.wantLastCheckedAt {
				t.Errorf("LastCheckedAt = %q, want %q", got.LastCheckedAt, tt.wantLastCheckedAt)
			}

			if got.TLSState != tt.wantTLSState {
				t.Errorf("TLSState = %q, want %q", got.TLSState, tt.wantTLSState)
			}

			if got.TLSDaysRemaining != tt.wantTLSDaysRemaining {
				t.Errorf("TLSDaysRemaining = %q, want %q", got.TLSDaysRemaining, tt.wantTLSDaysRemaining)
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

	got := newEndpointListItems([]endpoint.Endpoint{first, second})

	if len(got) != 2 {
		t.Fatalf("len(newEndpointListItems()) = %d, want 2", len(got))
	}

	if got[0].ID != first.ID {
		t.Errorf("first ID = %v, want %v", got[0].ID, first.ID)
	}

	if got[1].ID != second.ID {
		t.Errorf("second ID = %v, want %v", got[1].ID, second.ID)
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
			got := tlsExpiryClass(tt.days)

			if got != tt.want {
				t.Errorf("tlsExpiryClass(%d) = %q, want %q", tt.days, got, tt.want)
			}
		})
	}
}
