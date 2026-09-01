package endpoint

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantURL string
		wantErr string
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
			name:    "missing scheme",
			url:     "example.com",
			wantErr: "unsupported URL scheme",
		},
		{
			name:    "missing host",
			url:     "https://",
			wantErr: "URL host is required",
		},
		{
			name:    "unsupported scheme",
			url:     "ftp://example.com/",
			wantErr: "unsupported URL scheme",
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: "unsupported URL scheme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedURL, err := ParseURL(tt.url)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ParseURL() error = nil, want %q", tt.wantErr)
				}

				if err.Error() != tt.wantErr {
					t.Fatalf("ParseURL() error = %q, want %q", err.Error(), tt.wantErr)
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
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			})

			server := httptest.NewServer(handler)
			defer server.Close()

			parsedURL, err := ParseURL(server.URL)
			if err != nil {
				t.Fatalf("ParseURL() unexpected error = %v", err)
			}

			statusCode, err := GetStatusCode(parsedURL)
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

	statusCode, err := GetStatusCode(parsedURL)
	if err == nil {
		t.Fatal("GetStatusCode() error = nil, want error")
	}

	if statusCode != 0 {
		t.Errorf("GetStatusCode() = %d, want 0", statusCode)
	}
}

func TestAddValidEndpoint(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer server.Close()

	store := Store{}
	wantURL := server.URL + "/"

	endpoint, statusCode, err := store.Add(server.URL)
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
}

func TestAddInvalidEndpoint(t *testing.T) {
	store := Store{}

	endpoint, statusCode, err := store.Add("example.com")
	if err == nil {
		t.Fatal("Add() error = nil, want error")
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
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	)

	rawURL := server.URL
	server.Close()

	store := Store{}

	endpoint, statusCode, err := store.Add(rawURL)
	if err == nil {
		t.Fatal("Add() error = nil, want error")
	}

	wantURL := rawURL + "/"
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

func TestAddDuplicateEndpoint(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer server.Close()

	store := Store{}

	_, _, err := store.Add(server.URL)
	if err != nil {
		t.Fatalf("Add() unexpected error = %v", err)
	}

	endpoint, statusCode, err := store.Add(server.URL + "/")
	if err == nil {
		t.Fatal("Add() error = nil, want error")
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
