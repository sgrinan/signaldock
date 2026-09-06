package probe

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestNewHTTPClient(t *testing.T) {
	parsedURL := mustParseURL(t, "https://example.com/")

	t.Run("no_addresses", func(t *testing.T) {
		_, err := newHTTPClient(parsedURL, nil)

		if !errors.Is(err, ErrUnsafeHost) {
			t.Errorf("newHTTPClient(%q, nil) error = %v, want %v", parsedURL, err, ErrUnsafeHost)
		}
	})

	t.Run("too_many_redirects", func(t *testing.T) {
		client, err := newHTTPClient(
			parsedURL,
			[]netip.Addr{netip.MustParseAddr("8.8.8.8")},
		)
		if err != nil {
			t.Fatalf("newHTTPClient() returned unexpected error: %v", err)
		}

		req := &http.Request{
			URL: parsedURL,
		}

		via := make([]*http.Request, maxRedirects)

		err = client.CheckRedirect(req, via)

		if !errors.Is(err, ErrTooManyRedirects) {
			t.Errorf("CheckRedirect() error = %v, want %v", err, ErrTooManyRedirects)
		}
	})

	t.Run("unsupported_scheme", func(t *testing.T) {
		client, err := newHTTPClient(parsedURL, []netip.Addr{netip.MustParseAddr("8.8.8.8")})
		if err != nil {
			t.Fatalf("newHTTPClient() returned unexpected error: %v", err)
		}

		redirectURL := mustParseURL(t, "ftp://example.com/")

		req := &http.Request{
			URL: redirectURL,
		}

		err = client.CheckRedirect(req, nil)

		if !errors.Is(err, ErrUnsupportedRedirectScheme) {
			t.Errorf("CheckRedirect() error = %v, want %v", err, ErrUnsupportedRedirectScheme)
		}
	})

	t.Run("credentials", func(t *testing.T) {
		client, err := newHTTPClient(parsedURL, []netip.Addr{netip.MustParseAddr("8.8.8.8")})
		if err != nil {
			t.Fatalf("newHTTPClient() returned unexpected error: %v", err)
		}

		redirectURL := mustParseURL(t, "https://user:password@example.com/")

		req := &http.Request{
			URL: redirectURL,
		}

		err = client.CheckRedirect(req, nil)

		if !errors.Is(err, ErrRedirectCredentials) {
			t.Errorf("CheckRedirect() error = %v, want %v", err, ErrRedirectCredentials)
		}
	})

	t.Run("unsafe_destination", func(t *testing.T) {
		client, err := newHTTPClient(parsedURL, []netip.Addr{netip.MustParseAddr("8.8.8.8")})
		if err != nil {
			t.Fatalf("newHTTPClient() returned unexpected error: %v", err)
		}

		redirectURL := mustParseURL(t, "http://127.0.0.1/")

		req := &http.Request{
			URL: redirectURL,
		}

		err = client.CheckRedirect(req, nil)

		if !errors.Is(err, ErrUnsafeHost) {
			t.Errorf("CheckRedirect() error = %v, want %v", err, ErrUnsafeHost)
		}
	})
}

func TestHTTP(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}),
	)
	t.Cleanup(server.Close)

	parsedURL := mustParseURL(t, server.URL)

	got, err := HTTP(parsedURL, []netip.Addr{netip.MustParseAddr("127.0.0.1")})
	if err != nil {
		t.Fatalf("HTTP(%q) returned unexpected error: %v", parsedURL, err)
	}

	if got.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("HTTP(%q).StatusCode = %d, want %d", parsedURL, got.StatusCode, http.StatusServiceUnavailable)
	}

	if !got.Responded {
		t.Errorf("HTTP(%q).Responded = false, want true", parsedURL)
	}

	if got.CheckedAt.IsZero() {
		t.Errorf("HTTP(%q).CheckedAt is zero, want non-zero", parsedURL)
	}

	if got.Latency < 0 {
		t.Errorf("HTTP(%q).Latency = %v, want >= 0", parsedURL, got.Latency)
	}
}

func TestHTTP_RequestError(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	)

	parsedURL := mustParseURL(t, server.URL)

	server.Close()

	got, err := HTTP(parsedURL, []netip.Addr{netip.MustParseAddr("127.0.0.1")})

	if err == nil {
		t.Fatalf("HTTP(%q) error = nil, want non-nil", parsedURL)
	}

	if got.Responded {
		t.Errorf("HTTP(%q).Responded = true, want false", parsedURL)
	}

	if got.CheckedAt.IsZero() {
		t.Errorf("HTTP(%q).CheckedAt is zero, want non-zero", parsedURL)
	}
}

func TestHTTP_NoAddresses(t *testing.T) {
	parsedURL := mustParseURL(t, "https://example.com/")

	got, err := HTTP(parsedURL, nil)

	if !errors.Is(err, ErrUnsafeHost) {
		t.Errorf("HTTP(%q, nil) error = %v, want %v", parsedURL, err, ErrUnsafeHost)
	}

	if got != (HTTPResult{}) {
		t.Errorf("HTTP(%q, nil) = %+v, want zero HTTPResult", parsedURL, got)
	}
}
