package endpoint

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"
)

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

	if endpoint.ID == uuid.Nil() {
		t.Error("Add() endpoint.ID = uuid.Nil, want generated UUID")
	}
}

func TestAddInvalidEndpoint(t *testing.T) {
	store := Store{}

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

func TestGetByID(t *testing.T) {
	id := uuid.NewV7()
	missingID := uuid.NewV7()

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
