package probe

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestNewHTTPClient(t *testing.T) {
	parsedURL := mustParseURL(t, "https://example.com/")
	publicIP := netip.MustParseAddr("8.8.8.8")

	newClient := func(t *testing.T) *http.Client {
		t.Helper()

		client, err := newHTTPClient(parsedURL, []netip.Addr{publicIP})
		if err != nil {
			t.Fatalf("newHTTPClient() error = %v, want nil", err)
		}

		return client
	}

	t.Run("no_addresses", func(t *testing.T) {
		_, err := newHTTPClient(parsedURL, nil)

		if !errors.Is(err, ErrUnsafeHost) {
			t.Errorf("newHTTPClient(%q, nil) error = %v, want %v", parsedURL, err, ErrUnsafeHost)
		}
	})

	t.Run("redirect_error", func(t *testing.T) {
		tests := []struct {
			name        string
			redirectURL string
			via         []*http.Request
			wantErr     error
		}{
			{
				name:        "too_many_redirects",
				redirectURL: "https://example.com/",
				via:         make([]*http.Request, maxRedirects),
				wantErr:     ErrTooManyRedirects,
			},
			{
				name:        "unsupported_scheme",
				redirectURL: "ftp://example.com/",
				wantErr:     ErrUnsupportedRedirectScheme,
			},
			{
				name:        "credentials",
				redirectURL: "https://user:password@example.com/",
				wantErr:     ErrRedirectCredentials,
			},
			{
				name:        "unsafe_destination",
				redirectURL: "http://127.0.0.1/",
				wantErr:     ErrUnsafeHost,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				client := newClient(t)

				req := &http.Request{
					URL: mustParseURL(t, tt.redirectURL),
				}

				err := client.CheckRedirect(req, tt.via)

				if !errors.Is(err, tt.wantErr) {
					t.Errorf("CheckRedirect() error = %v, want %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("removes_referer", func(t *testing.T) {
		client := newClient(t)

		req := &http.Request{
			URL:    mustParseURL(t, "https://8.8.8.8/redirected"),
			Header: make(http.Header),
		}

		req.Header.Set("Referer", "https://example.com/health?token=super-secret-token")

		err := client.CheckRedirect(req, nil)
		if err != nil {
			t.Fatalf("CheckRedirect() error = %v, want nil", err)
		}

		if got := req.Header.Get("Referer"); got != "" {
			t.Errorf("CheckRedirect() Referer = %q, want empty", got)
		}
	})
}

func TestHTTP(t *testing.T) {
	t.Run("response", func(t *testing.T) {
		server := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
			}),
		)
		t.Cleanup(server.Close)

		parsedURL := mustParseURL(t, server.URL)

		got, err := HTTP(t.Context(), parsedURL, []netip.Addr{netip.MustParseAddr("127.0.0.1")})
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
	})

	t.Run("request_error_does_not_leak_query", func(t *testing.T) {
		const secret = "super-secret-token"

		server := httptest.NewServer(
			http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		)

		parsedURL := mustParseURL(t, server.URL)
		parsedURL.RawQuery = "token=" + secret

		server.Close()

		got, err := HTTP(t.Context(), parsedURL, []netip.Addr{netip.MustParseAddr("127.0.0.1")})

		if err == nil {
			t.Fatal("HTTP() error = nil, want non-nil")
		}

		if strings.Contains(err.Error(), secret) {
			t.Error("HTTP() error contains URL query secret, want secret omitted")
		}

		if got.Responded {
			t.Error("HTTP().Responded = true, want false")
		}

		if got.CheckedAt.IsZero() {
			t.Error("HTTP().CheckedAt is zero, want non-zero")
		}
	})

	t.Run("no_addresses", func(t *testing.T) {
		parsedURL := mustParseURL(t, "https://example.com/")

		got, err := HTTP(t.Context(), parsedURL, nil)

		if !errors.Is(err, ErrUnsafeHost) {
			t.Errorf("HTTP(%q, nil) error = %v, want %v", parsedURL, err, ErrUnsafeHost)
		}

		if got != (HTTPResult{}) {
			t.Errorf("HTTP(%q, nil) = %+v, want zero HTTPResult", parsedURL, got)
		}
	})

	t.Run("blocked_redirect_preserves_response", func(t *testing.T) {
		server := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "http://127.0.0.1/", http.StatusFound)
			}),
		)
		t.Cleanup(server.Close)

		parsedURL := mustParseURL(t, server.URL)

		got, err := HTTP(t.Context(), parsedURL, []netip.Addr{netip.MustParseAddr("127.0.0.1")})

		if !errors.Is(err, ErrUnsafeHost) {
			t.Fatalf("HTTP(%q) error = %v, want %v", parsedURL, err, ErrUnsafeHost)
		}

		if !got.Responded {
			t.Errorf("HTTP(%q).Responded = false, want true", parsedURL)
		}

		if got.StatusCode != http.StatusFound {
			t.Errorf("HTTP(%q).StatusCode = %d, want %d", parsedURL, got.StatusCode, http.StatusFound)
		}
	})

	t.Run("canceled_context", func(t *testing.T) {
		server := httptest.NewServer(
			http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		)
		t.Cleanup(server.Close)

		parsedURL := mustParseURL(t, server.URL)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := HTTP(ctx, parsedURL, []netip.Addr{netip.MustParseAddr("127.0.0.1")})

		if !errors.Is(err, context.Canceled) {
			t.Errorf("HTTP() error = %v, want %v", err, context.Canceled)
		}
	})
}
