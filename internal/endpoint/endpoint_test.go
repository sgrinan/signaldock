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
		wantErr string
	}{
		{
			name: "valid http URL",
			url:  "http://example.com/",
		},
		{
			name: "valid https URL",
			url:  "https://example.com/",
		},
		{
			name: "valid URL with path",
			url:  "https://example.com/health",
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
			_, err := ParseURL(tt.url)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ParseURL() unexpected error = %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("ParseURL() error = nil, want %q", tt.wantErr)
			}

			if err.Error() != tt.wantErr {
				t.Fatalf("ParseURL() error = %q, want %q", err.Error(), tt.wantErr)
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
			name:       "return status 200",
			statusCode: http.StatusOK,
		},
		{
			name:       "return status 503",
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
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	server := httptest.NewServer(handler)

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
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	store := Store{}

	endpoint, statusCode, err := store.Add(server.URL)
	if err != nil {
		t.Fatalf("Add() unexpected error = %v", err)
	}

	if endpoint.URL != server.URL {
		t.Errorf("Add() endpoint.URL = %q, want %q", endpoint.URL, server.URL)
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
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	server := httptest.NewServer(handler)

	rawURL := server.URL
	server.Close()

	store := Store{}

	endpoint, statusCode, err := store.Add(rawURL)
	if err == nil {
		t.Fatal("Add() error = nil, want error")
	}

	if endpoint.URL != rawURL {
		t.Errorf("Add() endpoint.URL = %q, want %q", endpoint.URL, rawURL)
	}

	if statusCode != 0 {
		t.Errorf("Add() statusCode = %d, want 0", statusCode)
	}

	if len(store.endpoints) != 1 {
		t.Errorf("Add() store size = %d, want 1", len(store.endpoints))
	}
}
