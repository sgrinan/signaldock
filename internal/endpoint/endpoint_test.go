package endpoint

import (
	"errors"
	"net/http"
	"net/netip"
	"net/url"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/sgrinan/signaldock/internal/probe"
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

func TestAddValidEndpoint(t *testing.T) {
	store := NewStore()

	store.validateHost = func(string) ([]netip.Addr, error) {
		return []netip.Addr{
			netip.MustParseAddr("93.184.216.34"),
		}, nil
	}

	wantHTTPResult := probe.Result{
		StatusCode: http.StatusOK,
		Latency:    50 * time.Millisecond,
		Available:  true,
	}

	store.probeHTTP = func(*url.URL, []netip.Addr) (probe.Result, error) {
		return wantHTTPResult, nil
	}

	wantTLSResult := probe.TLSResult{
		Enabled: true,
		Valid:   true,
	}

	store.probeTLS = func(*url.URL, []netip.Addr) (probe.TLSResult, error) {
		return wantTLSResult, nil
	}

	wantURL := "https://example.com/"

	endpoint, err := store.Add(wantURL)
	if err != nil {
		t.Fatalf("Add() unexpected error = %v", err)
	}

	if endpoint.URL != wantURL {
		t.Errorf("Add() endpoint.URL = %q, want %q", endpoint.URL, wantURL)
	}

	if endpoint.LastCheck.HTTP != wantHTTPResult {
		t.Errorf("Add() endpoint.LastCheck.HTTP = %+v, want %+v", endpoint.LastCheck.HTTP, wantHTTPResult)
	}

	if store.endpoints[0].LastCheck.HTTP != wantHTTPResult {
		t.Errorf("Add() stored LastCheck.HTTP = %+v, want %+v", store.endpoints[0].LastCheck.HTTP, wantHTTPResult)
	}

	if endpoint.LastCheck.TLS != wantTLSResult {
		t.Errorf("Add() endpoint.LastCheck.TLS = %+v, want %+v", endpoint.LastCheck.TLS, wantTLSResult)
	}

	if store.endpoints[0].LastCheck.TLS != wantTLSResult {
		t.Errorf("Add() stored LastCheck.TLS = %+v, want %+v", store.endpoints[0].LastCheck.TLS, wantTLSResult)
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

	endpoint, err := store.Add("example.com")

	if !errors.Is(err, ErrUnsupportedScheme) {
		t.Fatalf("Add() error = %v, want %v", err, ErrUnsupportedScheme)
	}

	if endpoint != (Endpoint{}) {
		t.Errorf("Add() endpoint = %+v, want empty Endpoint", endpoint)
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

	wantHTTPResult := probe.Result{
		StatusCode: 0,
		Latency:    50 * time.Millisecond,
		Available:  false,
	}

	store.probeHTTP = func(*url.URL, []netip.Addr) (probe.Result, error) {
		return wantHTTPResult, unreachableErr
	}

	wantTLSResult := probe.TLSResult{
		Enabled: true,
		Valid:   true,
	}

	store.probeTLS = func(*url.URL, []netip.Addr) (probe.TLSResult, error) {
		return wantTLSResult, nil
	}

	wantURL := "https://example.com/"

	endpoint, err := store.Add(wantURL)

	if !errors.Is(err, unreachableErr) {
		t.Fatalf("Add() error = %v, want %v", err, unreachableErr)
	}

	if endpoint.URL != wantURL {
		t.Errorf("Add() endpoint.URL = %q, want %q", endpoint.URL, wantURL)
	}

	if endpoint.LastCheck.HTTP != wantHTTPResult {
		t.Errorf("Add() endpoint.LastCheck.HTTP = %+v, want %+v", endpoint.LastCheck.HTTP, wantHTTPResult)
	}

	if endpoint.LastCheck.TLS != wantTLSResult {
		t.Errorf("Add() endpoint.LastCheck.TLS = %+v, want %+v", endpoint.LastCheck.TLS, wantTLSResult)
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

	store.probeHTTP = func(*url.URL, []netip.Addr) (probe.Result, error) {
		return probe.Result{
			StatusCode: http.StatusOK,
			Latency:    50 * time.Millisecond,
			Available:  true,
		}, nil
	}

	store.probeTLS = func(*url.URL, []netip.Addr) (probe.TLSResult, error) {
		return probe.TLSResult{
			Enabled: true,
			Valid:   true,
		}, nil
	}

	_, err := store.Add("https://example.com")
	if err != nil {
		t.Fatalf("Add() unexpected error = %v", err)
	}

	endpoint, err := store.Add("https://EXAMPLE.COM/")

	if !errors.Is(err, ErrEndpointExists) {
		t.Fatalf("Add() error = %v, want %v", err, ErrEndpointExists)
	}

	if endpoint != (Endpoint{}) {
		t.Errorf("Add() endpoint = %+v, want empty Endpoint", endpoint)
	}

	if len(store.endpoints) != 1 {
		t.Errorf("Add() store size = %d, want 1", len(store.endpoints))
	}
}

func TestAddUnsafeEndpoint(t *testing.T) {
	store := NewStore()

	store.validateHost = func(string) ([]netip.Addr, error) {
		return nil, probe.ErrUnsafeHost
	}

	endpoint, err := store.Add("https://example.com/")

	if !errors.Is(err, probe.ErrUnsafeHost) {
		t.Fatalf("Add() error = %v, want %v", err, probe.ErrUnsafeHost)
	}

	if endpoint != (Endpoint{}) {
		t.Errorf("Add() endpoint = %+v, want empty Endpoint", endpoint)
	}

	if len(store.endpoints) != 0 {
		t.Errorf("Add() store size = %d, want 0", len(store.endpoints))
	}
}

func TestAddConcurrentDuplicateEndpoint(t *testing.T) {
	store := NewStore()

	store.validateHost = func(string) ([]netip.Addr, error) {
		return []netip.Addr{
			netip.MustParseAddr("93.184.216.34"),
		}, nil
	}

	store.probeHTTP = func(*url.URL, []netip.Addr) (probe.Result, error) {
		time.Sleep(50 * time.Millisecond)

		return probe.Result{
			StatusCode: http.StatusOK,
			Available:  true,
		}, nil
	}

	store.probeTLS = func(*url.URL, []netip.Addr) (probe.TLSResult, error) {
		return probe.TLSResult{
			Enabled: true,
			Valid:   true,
		}, nil
	}

	const attempts = 10

	var wg sync.WaitGroup
	wg.Add(attempts)

	errorsCh := make(chan error, attempts)

	for range attempts {
		go func() {
			defer wg.Done()

			_, err := store.Add("https://example.com/")
			errorsCh <- err
		}()
	}

	wg.Wait()
	close(errorsCh)

	var successCount int
	var duplicateCount int

	for err := range errorsCh {
		switch {
		case err == nil:
			successCount++

		case errors.Is(err, ErrEndpointExists):
			duplicateCount++

		default:
			t.Fatalf("Add() unexpected error = %v", err)
		}
	}

	if successCount != 1 {
		t.Errorf("Add() success count = %d, want 1", successCount)
	}

	if duplicateCount != attempts-1 {
		t.Errorf("Add() duplicate count = %d, want %d", duplicateCount, attempts-1)
	}

	if len(store.List()) != 1 {
		t.Errorf("Add() store size = %d, want 1", len(store.List()))
	}
}

func TestAddInvalidTLS(t *testing.T) {
	store := NewStore()

	store.validateHost = func(string) ([]netip.Addr, error) {
		return []netip.Addr{
			netip.MustParseAddr("93.184.216.34"),
		}, nil
	}

	wantHTTPResult := probe.Result{
		StatusCode: http.StatusOK,
		Latency:    50 * time.Millisecond,
		Available:  true,
	}

	store.probeHTTP = func(*url.URL, []netip.Addr) (probe.Result, error) {
		return wantHTTPResult, nil
	}

	tlsErr := errors.New("invalid TLS certificate")

	wantTLSResult := probe.TLSResult{
		Enabled: true,
		Valid:   false,
	}

	store.probeTLS = func(*url.URL, []netip.Addr) (probe.TLSResult, error) {
		return wantTLSResult, tlsErr
	}

	endpoint, err := store.Add("https://example.com/")

	if !errors.Is(err, tlsErr) {
		t.Fatalf("Add() error = %v, want %v", err, tlsErr)
	}

	if endpoint.LastCheck.HTTP != wantHTTPResult {
		t.Errorf("Add() endpoint.LastCheck.HTTP = %+v, want %+v", endpoint.LastCheck.HTTP, wantHTTPResult)
	}

	if endpoint.LastCheck.TLS != wantTLSResult {
		t.Errorf("Add() endpoint.LastCheck.TLS = %+v, want %+v", endpoint.LastCheck.TLS, wantTLSResult)
	}

	if len(store.List()) != 1 {
		t.Errorf("Add() store size = %d, want 1", len(store.List()))
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

func TestRefreshEndpoint(t *testing.T) {
	id := uuid.NewV7()

	store := NewStore()

	store.endpoints = []Endpoint{
		{
			ID:  id,
			URL: "https://example.com/",
			LastCheck: CheckResult{
				HTTP: probe.Result{
					StatusCode: http.StatusOK,
					Latency:    20 * time.Millisecond,
					Available:  true,
				},
			},
		},
	}

	store.validateHost = func(string) ([]netip.Addr, error) {
		return []netip.Addr{
			netip.MustParseAddr("93.184.216.34"),
		}, nil
	}

	wantHTTPResult := probe.Result{
		StatusCode: http.StatusServiceUnavailable,
		Latency:    80 * time.Millisecond,
		Available:  true,
	}

	wantTLSResult := probe.TLSResult{
		Enabled: true,
		Valid:   true,
	}

	store.probeHTTP = func(*url.URL, []netip.Addr) (probe.Result, error) {
		return wantHTTPResult, nil
	}

	store.probeTLS = func(*url.URL, []netip.Addr) (probe.TLSResult, error) {
		return wantTLSResult, nil
	}

	wantCheck := CheckResult{
		HTTP: wantHTTPResult,
		TLS:  wantTLSResult,
	}

	check, err := store.Refresh(id)
	if err != nil {
		t.Fatalf("Refresh() unexpected error = %v", err)
	}

	if check != wantCheck {
		t.Errorf("Refresh() check = %+v, want %+v", check, wantCheck)
	}

	if store.endpoints[0].LastCheck != wantCheck {
		t.Errorf("Refresh() stored LastCheck = %+v, want %+v", store.endpoints[0].LastCheck, wantCheck)
	}
}

func TestRefreshEndpointNotFound(t *testing.T) {
	store := NewStore()

	check, err := store.Refresh(uuid.NewV7())

	if !errors.Is(err, ErrEndpointNotFound) {
		t.Fatalf("Refresh() error = %v, want %v", err, ErrEndpointNotFound)
	}

	if check != (CheckResult{}) {
		t.Errorf("Refresh() check = %+v, want empty CheckResult", check)
	}
}

func TestRefreshUnreachableEndpoint(t *testing.T) {
	id := uuid.NewV7()

	store := NewStore()

	store.endpoints = []Endpoint{
		{
			ID:  id,
			URL: "https://example.com/",
			LastCheck: CheckResult{
				HTTP: probe.Result{
					StatusCode: http.StatusOK,
					Latency:    20 * time.Millisecond,
					Available:  true,
				},
			},
		},
	}

	store.validateHost = func(string) ([]netip.Addr, error) {
		return []netip.Addr{
			netip.MustParseAddr("93.184.216.34"),
		}, nil
	}

	unreachableErr := errors.New("endpoint unreachable")

	wantHTTPResult := probe.Result{
		StatusCode: 0,
		Latency:    80 * time.Millisecond,
		Available:  false,
	}

	wantTLSResult := probe.TLSResult{
		Enabled: true,
		Valid:   true,
	}

	store.probeHTTP = func(*url.URL, []netip.Addr) (probe.Result, error) {
		return wantHTTPResult, unreachableErr
	}

	store.probeTLS = func(*url.URL, []netip.Addr) (probe.TLSResult, error) {
		return wantTLSResult, nil
	}

	wantCheck := CheckResult{
		HTTP: wantHTTPResult,
		TLS:  wantTLSResult,
	}

	check, err := store.Refresh(id)

	if !errors.Is(err, unreachableErr) {
		t.Fatalf("Refresh() error = %v, want %v", err, unreachableErr)
	}

	if check != wantCheck {
		t.Errorf("Refresh() check = %+v, want %+v", check, wantCheck)
	}

	if store.endpoints[0].LastCheck != wantCheck {
		t.Errorf("Refresh() stored LastCheck = %+v, want %+v", store.endpoints[0].LastCheck, wantCheck)
	}
}
