package endpoint

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"testing"
	"uuid"
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

func TestParseURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantURL string
		wantErr error
	}{
		{
			name:    "valid URL without root path",
			url:     "https://example.com",
			wantURL: "https://example.com/",
		},
		{
			name:    "valid HTTPS URL with root path",
			url:     "https://example.com/",
			wantURL: "https://example.com/",
		},
		{
			name:    "valid HTTP URL with root path",
			url:     "http://example.com/",
			wantURL: "http://example.com/",
		},
		{
			name:    "valid URL with path",
			url:     "https://example.com/health",
			wantURL: "https://example.com/health",
		},
		{
			name:    "valid URL with www hostname",
			url:     "https://www.example.com",
			wantURL: "https://www.example.com/",
		},
		{
			name:    "normalizes uppercase www hostname",
			url:     "https://WWW.EXAMPLE.COM",
			wantURL: "https://www.example.com/",
		},
		{
			name:    "normalizes uppercase scheme and hostname",
			url:     "HTTPS://EXAMPLE.COM",
			wantURL: "https://example.com/",
		},
		{
			name:    "normalizes hostname with port",
			url:     "https://EXAMPLE.COM:8443/",
			wantURL: "https://example.com:8443/",
		},
		{
			name:    "preserves path case",
			url:     "https://EXAMPLE.COM/MyAPI/Health",
			wantURL: "https://example.com/MyAPI/Health",
		},
		{
			name:    "valid IPv6 URL",
			url:     "https://[2001:DB8::1]/",
			wantURL: "https://[2001:db8::1]/",
		},
		{
			name:    "missing scheme",
			url:     "example.com",
			wantErr: ErrUnsupportedScheme,
		},
		{
			name:    "missing host",
			url:     "https://",
			wantErr: ErrHostRequired,
		},
		{
			name:    "unsupported scheme",
			url:     "ftp://example.com/",
			wantErr: ErrUnsupportedScheme,
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: ErrUnsupportedScheme,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedURL, err := ParseURL(tt.url)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ParseURL() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseURL() unexpected error = %v", err)
			}

			if parsedURL.String() != tt.wantURL {
				t.Errorf("ParseURL() = %q, want %q", parsedURL.String(), tt.wantURL)
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

			parsedURL, err := ParseURL(server.URL)
			if err != nil {
				t.Fatalf("ParseURL() unexpected error = %v", err)
			}

			addr := []netip.Addr{
				netip.MustParseAddr(parsedURL.Hostname()),
			}

			statusCode, err := GetStatusCode(parsedURL, addr)
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

	parsedURL, err := ParseURL(server.URL)
	if err != nil {
		t.Fatalf("ParseURL() unexpected error = %v", err)
	}

	server.Close()

	addr := []netip.Addr{
		netip.MustParseAddr(parsedURL.Hostname()),
	}

	statusCode, err := GetStatusCode(parsedURL, addr)
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

	parsedURL, err := ParseURL(server.URL)
	if err != nil {
		t.Fatalf("ParseURL() unexpected error = %v", err)
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

func TestAddValidEndpoint(t *testing.T) {
	store := NewStore()

	store.validateHost = func(string) ([]netip.Addr, error) {
		return []netip.Addr{
			netip.MustParseAddr("93.184.216.34"),
		}, nil
	}

	store.getStatusCode = func(*url.URL, []netip.Addr) (int, error) {
		return http.StatusOK, nil
	}

	wantURL := "https://example.com/"

	endpoint, statusCode, err := store.Add(wantURL)
	if err != nil {
		t.Fatalf("Add() unexpected error = %v", err)
	}

	if endpoint.URL != wantURL {
		t.Errorf("Add() endpoint.URL = %q, want %q", endpoint.URL, wantURL)
	}

	if statusCode != http.StatusOK {
		t.Errorf("Add() statusCode = %d, want %d", statusCode, http.StatusOK)
	}

	if len(store.endpoints) != 1 {
		t.Errorf("Add() store size = %d, want 1", len(store.endpoints))
	}

	if endpoint.ID == uuid.Nil() {
		t.Error("Add() endpoint.ID = uuid.Nil, want generated UUID")
	}
}

func TestAddInvalidEndpoint(t *testing.T) {
	store := NewStore()

	endpoint, statusCode, err := store.Add("example.com")

	if !errors.Is(err, ErrUnsupportedScheme) {
		t.Fatalf("Add() error = %v, want %v", err, ErrUnsupportedScheme)
	}

	if endpoint != (Endpoint{}) {
		t.Errorf("Add() endpoint = %+v, want empty Endpoint", endpoint)
	}

	if statusCode != 0 {
		t.Errorf("Add() statusCode = %d, want 0", statusCode)
	}

	if len(store.endpoints) != 0 {
		t.Errorf("Add() store size = %d, want 0", len(store.endpoints))
	}
}

func TestAddUnreachableEndpoint(t *testing.T) {
	store := NewStore()

	store.validateHost = func(string) ([]netip.Addr, error) {
		return []netip.Addr{
			netip.MustParseAddr("93.184.216.34"),
		}, nil
	}

	unreachableErr := errors.New("endpoint unreachable")

	store.getStatusCode = func(*url.URL, []netip.Addr) (int, error) {
		return 0, unreachableErr
	}

	wantURL := "https://example.com/"

	endpoint, statusCode, err := store.Add(wantURL)

	if !errors.Is(err, unreachableErr) {
		t.Fatalf("Add() error = %v, want %v", err, unreachableErr)
	}

	if endpoint.URL != wantURL {
		t.Errorf("Add() endpoint.URL = %q, want %q", endpoint.URL, wantURL)
	}

	if statusCode != 0 {
		t.Errorf("Add() statusCode = %d, want 0", statusCode)
	}

	if len(store.endpoints) != 1 {
		t.Errorf("Add() store size = %d, want 1", len(store.endpoints))
	}
}

func TestAddDuplicateEndpoint(t *testing.T) {
	store := NewStore()

	store.validateHost = func(string) ([]netip.Addr, error) {
		return []netip.Addr{
			netip.MustParseAddr("93.184.216.34"),
		}, nil
	}

	store.getStatusCode = func(*url.URL, []netip.Addr) (int, error) {
		return http.StatusOK, nil
	}

	_, _, err := store.Add("https://example.com")
	if err != nil {
		t.Fatalf("Add() unexpected error = %v", err)
	}

	endpoint, statusCode, err := store.Add("https://EXAMPLE.COM/")

	if !errors.Is(err, ErrEndpointExists) {
		t.Fatalf("Add() error = %v, want %v", err, ErrEndpointExists)
	}

	if endpoint != (Endpoint{}) {
		t.Errorf("Add() endpoint = %+v, want empty Endpoint", endpoint)
	}

	if statusCode != 0 {
		t.Errorf("Add() statusCode = %d, want 0", statusCode)
	}

	if len(store.endpoints) != 1 {
		t.Errorf("Add() store size = %d, want 1", len(store.endpoints))
	}
}

func TestAddUnsafeEndpoint(t *testing.T) {
	store := NewStore()

	store.validateHost = func(string) ([]netip.Addr, error) {
		return nil, ErrUnsafeHost
	}

	endpoint, statusCode, err := store.Add("https://example.com/")

	if !errors.Is(err, ErrUnsafeHost) {
		t.Fatalf("Add() error = %v, want %v", err, ErrUnsafeHost)
	}

	if endpoint != (Endpoint{}) {
		t.Errorf("Add() endpoint = %+v, want empty Endpoint", endpoint)
	}

	if statusCode != 0 {
		t.Errorf("Add() statusCode = %d, want 0", statusCode)
	}

	if len(store.endpoints) != 0 {
		t.Errorf("Add() store size = %d, want 0", len(store.endpoints))
	}
}

func TestExists(t *testing.T) {
	store := Store{
		endpoints: []Endpoint{
			{URL: "https://first.com/"},
			{URL: "https://second.com/"},
		},
	}

	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "existing endpoint",
			url:  "https://second.com/",
			want: true,
		},
		{
			name: "missing endpoint",
			url:  "https://third.com/",
			want: false,
		},
		{
			name: "missing alternative endpoint",
			url:  "https://www.second.com/",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists := store.Exists(tt.url)
			if exists != tt.want {
				t.Errorf("Exists(%q) = %v, want %v", tt.url, exists, tt.want)
			}
		})
	}
}

func TestGetByID(t *testing.T) {
	id := uuid.NewV7()

	store := Store{
		endpoints: []Endpoint{
			{
				ID:  id,
				URL: "https://example.com/",
			},
		},
	}

	endpoint, found := store.GetByID(id)

	if !found {
		t.Error("GetByID() found = false, want true")
	}

	if endpoint.ID != id {
		t.Errorf("GetByID() endpoint.ID = %v, want %v", endpoint.ID, id)
	}

	if endpoint.URL != "https://example.com/" {
		t.Errorf("GetByID() endpoint.URL = %q, want %q", endpoint.URL, "https://example.com/")
	}

	missingID := uuid.NewV7()
	endpoint, found = store.GetByID(missingID)

	if found {
		t.Error("GetByID() found = true, want false")
	}

	if endpoint != (Endpoint{}) {
		t.Errorf("GetByID() endpoint = %+v, want empty Endpoint", endpoint)
	}
}

func TestRemoveByID(t *testing.T) {
	t.Run("existing endpoint", func(t *testing.T) {
		id := uuid.NewV7()

		store := Store{
			endpoints: []Endpoint{
				{URL: "https://first.com/"},
				{URL: "https://second.com/", ID: id},
			},
		}

		removed := store.RemoveByID(id)

		if !removed {
			t.Error("RemoveByID() = false, want true")
		}

		if len(store.endpoints) != 1 {
			t.Errorf("RemoveByID() store size = %d, want 1", len(store.endpoints))
		}

		if store.endpoints[0].URL != "https://first.com/" {
			t.Errorf("RemoveByID() remaining endpoint = %q, want %q", store.endpoints[0].URL, "https://first.com/")
		}
	})

	t.Run("missing endpoint", func(t *testing.T) {
		store := Store{
			endpoints: []Endpoint{
				{URL: "https://first.com/"},
			},
		}

		missingID := uuid.NewV7()
		removed := store.RemoveByID(missingID)

		if removed {
			t.Error("RemoveByID() = true, want false")
		}

		if len(store.endpoints) != 1 {
			t.Errorf("RemoveByID() store size = %d, want 1", len(store.endpoints))
		}
	})
}
