package probe

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"testing"
	"time"
)

func TestTLS(t *testing.T) {
	t.Run("non_https", func(t *testing.T) {
		parsedURL := mustParseURL(t, "http://example.com/")

		got, err := TLS(parsedURL, nil)
		if err != nil {
			t.Fatalf("TLS(%q) returned unexpected error: %v", parsedURL, err)
		}

		if got != (TLSResult{}) {
			t.Errorf("TLS(%q) = %+v, want zero TLSResult", parsedURL, got)
		}
	})

	t.Run("no_addresses", func(t *testing.T) {
		parsedURL := mustParseURL(t, "https://example.com/")

		got, err := TLS(parsedURL, nil)

		if !errors.Is(err, ErrUnsafeHost) {
			t.Errorf("TLS(%q, nil) error = %v, want %v", parsedURL, err, ErrUnsafeHost)
		}

		if got != (TLSResult{}) {
			t.Errorf("TLS(%q, nil) = %+v, want zero TLSResult", parsedURL, got)
		}
	})

	t.Run("valid_certificate", func(t *testing.T) {
		now := time.Now().UTC()

		_, parsedURL, ips, roots, cert := newTLSTestServer(t, "localhost", now.Add(-time.Hour), now.Add(72*time.Hour))

		got, err := tlsProbe(parsedURL, ips, roots)
		if err != nil {
			t.Fatalf("tlsProbe(%q) returned unexpected error: %v", parsedURL, err)
		}

		if !got.Enabled {
			t.Errorf("tlsProbe(%q).Enabled = false, want true", parsedURL)
		}

		if !got.Valid {
			t.Errorf("tlsProbe(%q).Valid = false, want true", parsedURL)
		}

		if !got.ExpiresAt.Equal(cert.NotAfter) {
			t.Errorf("tlsProbe(%q).ExpiresAt = %v, want %v", parsedURL, got.ExpiresAt, cert.NotAfter)
		}

		if got.DaysRemaining < 2 {
			t.Errorf("tlsProbe(%q).DaysRemaining = %d, want >= 2", parsedURL, got.DaysRemaining)
		}
	})

	t.Run("expired_certificate", func(t *testing.T) {
		now := time.Now().UTC()

		_, parsedURL, ips, roots, cert := newTLSTestServer(t, "localhost", now.Add(-72*time.Hour), now.Add(-time.Hour))

		got, err := tlsProbe(parsedURL, ips, roots)
		if err == nil {
			t.Fatalf("tlsProbe(%q) error = nil, want certificate validation error", parsedURL)
		}

		if !got.Enabled {
			t.Errorf("tlsProbe(%q).Enabled = false, want true", parsedURL)
		}

		if got.Valid {
			t.Errorf("tlsProbe(%q).Valid = true, want false", parsedURL)
		}

		if !got.ExpiresAt.Equal(cert.NotAfter) {
			t.Errorf("tlsProbe(%q).ExpiresAt = %v, want %v", parsedURL, got.ExpiresAt, cert.NotAfter)
		}

		if got.DaysRemaining >= 0 {
			t.Errorf("tlsProbe(%q).DaysRemaining = %d, want < 0", parsedURL, got.DaysRemaining)
		}
	})

	t.Run("untrusted_certificate", func(t *testing.T) {
		now := time.Now().UTC()

		_, parsedURL, ips, _, cert := newTLSTestServer(t, "localhost", now.Add(-time.Hour), now.Add(72*time.Hour))

		untrustedRoots := x509.NewCertPool()

		got, err := tlsProbe(parsedURL, ips, untrustedRoots)
		if err == nil {
			t.Fatalf("tlsProbe(%q) error = nil, want certificate validation error", parsedURL)
		}

		if !got.Enabled {
			t.Errorf("tlsProbe(%q).Enabled = false, want true", parsedURL)
		}

		if got.Valid {
			t.Errorf("tlsProbe(%q).Valid = true, want false", parsedURL)
		}

		if !got.ExpiresAt.Equal(cert.NotAfter) {
			t.Errorf("tlsProbe(%q).ExpiresAt = %v, want %v", parsedURL, got.ExpiresAt, cert.NotAfter)
		}
	})

	t.Run("hostname_mismatch", func(t *testing.T) {
		now := time.Now().UTC()

		_, parsedURL, ips, roots, cert := newTLSTestServer(t, "localhost", now.Add(-time.Hour), now.Add(72*time.Hour))

		parsedURL = mustParseURL(t, "https://example.com:"+parsedURL.Port())

		got, err := tlsProbe(parsedURL, ips, roots)
		if err == nil {
			t.Fatalf("tlsProbe(%q) error = nil, want hostname validation error", parsedURL)
		}

		if !got.Enabled {
			t.Errorf("tlsProbe(%q).Enabled = false, want true", parsedURL)
		}

		if got.Valid {
			t.Errorf("tlsProbe(%q).Valid = true, want false", parsedURL)
		}

		if !got.ExpiresAt.Equal(cert.NotAfter) {
			t.Errorf("tlsProbe(%q).ExpiresAt = %v, want %v", parsedURL, got.ExpiresAt, cert.NotAfter)
		}
	})

	t.Run("connection_error", func(t *testing.T) {
		now := time.Now().UTC()

		server, parsedURL, ips, roots, _ := newTLSTestServer(t, "localhost", now.Add(-time.Hour), now.Add(72*time.Hour))

		server.Close()

		got, err := tlsProbe(parsedURL, ips, roots)
		if err == nil {
			t.Fatalf("tlsProbe(%q) error = nil, want connection error", parsedURL)
		}

		if !got.Enabled {
			t.Errorf("tlsProbe(%q).Enabled = false, want true", parsedURL)
		}

		if got.Valid {
			t.Errorf("tlsProbe(%q).Valid = true, want false", parsedURL)
		}

		if !got.ExpiresAt.IsZero() {
			t.Errorf("tlsProbe(%q).ExpiresAt = %v, want zero time", parsedURL, got.ExpiresAt)
		}
	})

	t.Run("falls_back_to_next_address", func(t *testing.T) {
		now := time.Now().UTC()

		_, parsedURL, ips, roots, _ := newTLSTestServer(t, "localhost", now.Add(-time.Hour), now.Add(72*time.Hour))

		ips = append([]netip.Addr{netip.MustParseAddr("127.0.0.2")}, ips...)

		got, err := tlsProbe(parsedURL, ips, roots)
		if err != nil {
			t.Fatalf("tlsProbe(%q) returned unexpected error: %v", parsedURL, err)
		}

		if !got.Valid {
			t.Errorf("tlsProbe(%q).Valid = false, want true", parsedURL)
		}
	})
}

func TestVerifyCertificate(t *testing.T) {
	err := verifyCertificate(nil, "example.com", nil)

	if !errors.Is(err, ErrNoPeerCertificate) {
		t.Errorf("verifyCertificate(nil, %q, nil) error = %v, want %v", "example.com", err, ErrNoPeerCertificate)
	}
}

func TestDaysBetween(t *testing.T) {
	now := time.Date(2026, time.September, 6, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		expiresAt time.Time
		want      int
	}{
		{
			name:      "two_days",
			expiresAt: now.Add(48 * time.Hour),
			want:      2,
		},
		{
			name:      "less_than_one_day",
			expiresAt: now.Add(23 * time.Hour),
			want:      0,
		},
		{
			name:      "expired_less_than_one_day",
			expiresAt: now.Add(-time.Hour),
			want:      -1,
		},
		{
			name:      "expired_more_than_one_day",
			expiresAt: now.Add(-25 * time.Hour),
			want:      -2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := daysBetween(now, tt.expiresAt); got != tt.want {
				t.Errorf("daysBetween(%v, %v) = %d, want %d", now, tt.expiresAt, got, tt.want)
			}
		})
	}
}

func mustParseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("url.Parse(%q) returned unexpected error: %v", rawURL, err)
	}

	return parsedURL
}

func newTLSTestServer(t *testing.T, serverName string, notBefore time.Time, notAfter time.Time) (*httptest.Server, *url.URL, []netip.Addr, *x509.CertPool, *x509.Certificate) {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey() returned unexpected error: %v", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "SignalDock Test CA",
		},
		NotBefore:             notBefore.Add(-24 * time.Hour),
		NotAfter:              notAfter.Add(365 * 24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}

	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("x509.CreateCertificate(CA) returned unexpected error: %v", err)
	}

	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("x509.ParseCertificate(CA) returned unexpected error: %v", err)
	}

	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey() returned unexpected error: %v", err)
	}

	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: serverName,
		},
		DNSNames:  []string{serverName},
		NotBefore: notBefore,
		NotAfter:  notAfter,
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
	}

	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("x509.CreateCertificate(server) returned unexpected error: %v", err)
	}

	serverCert, err := x509.ParseCertificate(serverDER)
	if err != nil {
		t.Fatalf("x509.ParseCertificate(server) returned unexpected error: %v", err)
	}

	serverKeyDER, err := x509.MarshalECPrivateKey(serverKey)
	if err != nil {
		t.Fatalf("x509.MarshalECPrivateKey() returned unexpected error: %v", err)
	}

	certificate, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: serverDER,
		}),
		pem.EncodeToMemory(&pem.Block{
			Type:  "EC PRIVATE KEY",
			Bytes: serverKeyDER,
		}),
	)
	if err != nil {
		t.Fatalf("tls.X509KeyPair() returned unexpected error: %v", err)
	}

	server := httptest.NewUnstartedServer(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	)

	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   tls.VersionTLS12,
	}

	server.StartTLS()
	t.Cleanup(server.Close)

	serverURL := mustParseURL(t, server.URL)

	parsedURL := mustParseURL(t, "https://"+serverName+":"+serverURL.Port())

	roots := x509.NewCertPool()
	roots.AddCert(caCert)

	ips := []netip.Addr{
		netip.MustParseAddr("127.0.0.1"),
	}

	return server, parsedURL, ips, roots, serverCert
}
