package probe

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"testing"
	"time"
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

		if result.CheckedAt.IsZero() {
			t.Error("HTTP() CheckedAt is zero, want check time")
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

		if result.CheckedAt.IsZero() {
			t.Error("HTTP() CheckedAt is zero, want check time")
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

		if result.CheckedAt.IsZero() {
			t.Error("HTTP() CheckedAt is zero, want check time")
		}
	})
}

func TestHTTPUnsafeRedirect(t *testing.T) {
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

	result, err := HTTP(parsedURL, ips)
	if !errors.Is(err, ErrUnsafeHost) {
		t.Fatalf("HTTP() error = %v, want %v", err, ErrUnsafeHost)
	}

	if result.Available {
		t.Error("HTTP() Available = true, want false")
	}
}

func TestHTTPNoIPs(t *testing.T) {
	parsedURL, err := url.Parse("https://example.com/")
	if err != nil {
		t.Fatalf("url.Parse() unexpected error = %v", err)
	}

	result, err := HTTP(parsedURL, nil)

	if !errors.Is(err, ErrUnsafeHost) {
		t.Fatalf("HTTP() error = %v, want %v", err, ErrUnsafeHost)
	}

	if result != (Result{}) {
		t.Errorf("HTTP() result = %+v, want empty Result", result)
	}
}

func TestTLSHTTP(t *testing.T) {
	parsedURL, err := url.Parse("http://example.com/")
	if err != nil {
		t.Fatalf("url.Parse() unexpected error = %v", err)
	}

	result, err := TLS(parsedURL, nil)
	if err != nil {
		t.Fatalf("TLS() unexpected error = %v", err)
	}

	if result != (TLSResult{}) {
		t.Errorf("TLS() result = %+v, want empty TLSResult", result)
	}
}

func TestTLSNoIPs(t *testing.T) {
	parsedURL, err := url.Parse("https://example.com/")
	if err != nil {
		t.Fatalf("url.Parse() unexpected error = %v", err)
	}

	result, err := TLS(parsedURL, nil)

	if !errors.Is(err, ErrUnsafeHost) {
		t.Fatalf("TLS() error = %v, want %v", err, ErrUnsafeHost)
	}

	if result != (TLSResult{}) {
		t.Errorf("TLS() result = %+v, want empty TLSResult", result)
	}
}

func TestTLSSelfSignedCertificate(t *testing.T) {
	server := httptest.NewTLSServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
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

	result, err := TLS(parsedURL, ips)

	if err == nil {
		t.Fatal("TLS() error = nil, want certificate validation error")
	}

	if !result.Enabled {
		t.Error("TLS() Enabled = false, want true")
	}
}

func newTestCertificate(t *testing.T) (tls.Certificate, *x509.CertPool, time.Time) {
	t.Helper()

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() CA error = %v", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "SignalDock Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}

	caDER, err := x509.CreateCertificate(
		rand.Reader,
		caTemplate,
		caTemplate,
		&caKey.PublicKey,
		caKey,
	)
	if err != nil {
		t.Fatalf("x509.CreateCertificate() CA error = %v", err)
	}

	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("x509.ParseCertificate() CA error = %v", err)
	}

	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey() server error = %v", err)
	}

	expiresAt := time.Now().Add(48 * time.Hour).Truncate(time.Second)

	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     expiresAt,

		IPAddresses: []net.IP{
			net.ParseIP("127.0.0.1"),
		},

		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
	}

	serverDER, err := x509.CreateCertificate(
		rand.Reader,
		serverTemplate,
		caCert,
		&serverKey.PublicKey,
		caKey,
	)
	if err != nil {
		t.Fatalf("x509.CreateCertificate() server error = %v", err)
	}

	cert := tls.Certificate{
		Certificate: [][]byte{
			serverDER,
			caDER,
		},
		PrivateKey: serverKey,
	}

	rootCAs := x509.NewCertPool()
	rootCAs.AddCert(caCert)

	return cert, rootCAs, expiresAt
}

func TestTLSValidCertificate(t *testing.T) {
	cert, rootCAs, wantExpiresAt := newTestCertificate(t)

	server := httptest.NewUnstartedServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	server.StartTLS()
	defer server.Close()

	parsedURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse() unexpected error = %v", err)
	}

	ips := []netip.Addr{
		netip.MustParseAddr(parsedURL.Hostname()),
	}

	result, err := tlsProbe(parsedURL, ips, rootCAs)
	if err != nil {
		t.Fatalf("tlsProbe() unexpected error = %v", err)
	}

	if !result.Enabled {
		t.Error("tlsProbe() Enabled = false, want true")
	}

	if result.ExpiresAt.IsZero() {
		t.Fatal("tlsProbe() ExpiresAt is zero")
	}

	if !result.ExpiresAt.Equal(wantExpiresAt) {
		t.Errorf("tlsProbe() ExpiresAt = %v, want %v", result.ExpiresAt, wantExpiresAt)
	}

	if result.DaysRemaining < 1 || result.DaysRemaining > 2 {
		t.Errorf("tlsProbe() DaysRemaining = %d, want between 1 and 2", result.DaysRemaining)
	}
}
