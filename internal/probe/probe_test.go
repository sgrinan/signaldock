package probe

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"testing"
)

func TestValidateHost(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		wantErr error
	}{
		{
			name: "public IPv4 Cloudflare",
			host: "1.1.1.1",
		},
		{
			name: "public IPv4 Google",
			host: "8.8.8.8",
		},
		{
			name:    "loopback IPv4",
			host:    "127.0.0.1",
			wantErr: ErrUnsafeHost,
		},
		{
			name:    "localhost hostname",
			host:    "localhost",
			wantErr: ErrUnsafeHost,
		},
		{
			name:    "private IPv4 10 range",
			host:    "10.0.0.1",
			wantErr: ErrUnsafeHost,
		},
		{
			name:    "private IPv4 192 range",
			host:    "192.168.1.1",
			wantErr: ErrUnsafeHost,
		},
		{
			name:    "link-local metadata IPv4",
			host:    "169.254.169.254",
			wantErr: ErrUnsafeHost,
		},
		{
			name:    "unspecified IPv4",
			host:    "0.0.0.0",
			wantErr: ErrUnsafeHost,
		},
		{
			name:    "loopback IPv6",
			host:    "::1",
			wantErr: ErrUnsafeHost,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ips, err := ValidateHost(tt.host)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ValidateHost() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("ValidateHost() unexpected error = %v", err)
			}

			if len(ips) == 0 {
				t.Fatal("ValidateHost() returned no IPs, want at least one")
			}
		})
	}
}

func TestGetStatusCode(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{
			name:       "returns status 200",
			statusCode: http.StatusOK,
		},
		{
			name:       "returns status 503",
			statusCode: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tt.statusCode)
				}),
			)
			defer server.Close()

			parsedURL, err := url.Parse(server.URL)
			if err != nil {
				t.Fatalf("ParseURL() unexpected error = %v", err)
			}

			ips := []netip.Addr{
				netip.MustParseAddr(parsedURL.Hostname()),
			}

			statusCode, err := GetStatusCode(parsedURL, ips)
			if err != nil {
				t.Fatalf("GetStatusCode() unexpected error = %v", err)
			}

			if statusCode != tt.statusCode {
				t.Errorf("GetStatusCode() = %d, want %d", statusCode, tt.statusCode)
			}
		})
	}
}

func TestGetStatusCodeEndpointUnreachable(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)

	parsedURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("ParseURL() unexpected error = %v", err)
	}

	server.Close()

	ips := []netip.Addr{
		netip.MustParseAddr(parsedURL.Hostname()),
	}

	statusCode, err := GetStatusCode(parsedURL, ips)
	if err == nil {
		t.Fatal("GetStatusCode() error = nil, want error")
	}

	if statusCode != 0 {
		t.Errorf("GetStatusCode() = %d, want 0", statusCode)
	}
}

func TestGetStatusCodeUnsafeRedirect(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://127.0.0.1/", http.StatusFound)
		}),
	)
	defer server.Close()

	parsedURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse() unexpected error = %v", err)
	}

	ips := []netip.Addr{
		netip.MustParseAddr(parsedURL.Hostname()),
	}

	statusCode, err := GetStatusCode(parsedURL, ips)
	if !errors.Is(err, ErrUnsafeHost) {
		t.Fatalf("GetStatusCode() error = %v, want %v", err, ErrUnsafeHost)
	}

	if statusCode != 0 {
		t.Errorf("GetStatusCode() = %d, want 0", statusCode)
	}
}

func TestHTTP(t *testing.T) {
	t.Run("HTTP 200", func(t *testing.T) {
		server := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		)
		defer server.Close()

		parsedURL, err := url.Parse(server.URL)
		if err != nil {
			t.Fatalf("ParseURL() unexpected error = %v", err)
		}

		ips := []netip.Addr{
			netip.MustParseAddr(parsedURL.Hostname()),
		}

		result, err := HTTP(parsedURL, ips)
		if err != nil {
			t.Fatalf("HTTP() unexpected error = %v", err)
		}

		if result.StatusCode != http.StatusOK {
			t.Errorf("HTTP() StatusCode = %d, want %d", result.StatusCode, http.StatusOK)
		}

		if !result.Available {
			t.Error("HTTP() Available = false, want true")
		}

		if result.Latency <= 0 {
			t.Errorf("HTTP() Latency = %v, want > 0", result.Latency)
		}
	})

	t.Run("HTTP 503", func(t *testing.T) {
		server := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
			}),
		)
		defer server.Close()

		parsedURL, err := url.Parse(server.URL)
		if err != nil {
			t.Fatalf("ParseURL() unexpected error = %v", err)
		}

		ips := []netip.Addr{
			netip.MustParseAddr(parsedURL.Hostname()),
		}

		result, err := HTTP(parsedURL, ips)
		if err != nil {
			t.Fatalf("HTTP() unexpected error = %v", err)
		}

		if result.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("HTTP() StatusCode = %d, want %d", result.StatusCode, http.StatusServiceUnavailable)
		}

		if !result.Available {
			t.Error("HTTP() Available = false, want true")
		}

		if result.Latency <= 0 {
			t.Errorf("HTTP() Latency = %v, want > 0", result.Latency)
		}
	})

	t.Run("HTTP Endpoint unreachable", func(t *testing.T) {
		server := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		)

		parsedURL, err := url.Parse(server.URL)
		if err != nil {
			t.Fatalf("ParseURL() unexpected error = %v", err)
		}

		ips := []netip.Addr{
			netip.MustParseAddr(parsedURL.Hostname()),
		}

		server.Close()

		result, err := HTTP(parsedURL, ips)
		if err == nil {
			t.Fatal("HTTP() error = nil, want error")
		}

		if result.StatusCode != 0 {
			t.Errorf("HTTP() StatusCode = %d, want 0", result.StatusCode)
		}

		if result.Available {
			t.Error("HTTP() Available = true, want false")
		}

		if result.Latency <= 0 {
			t.Errorf("HTTP() Latency = %v, want > 0", result.Latency)
		}
	})
}