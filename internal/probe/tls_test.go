package probe

import (
	"crypto/x509"
	"errors"
	"net/netip"
	"testing"
	"time"
)

func TestTLS(t *testing.T) {
	t.Run("non_https", func(t *testing.T) {
		parsedURL := mustParseURL(t, "http://example.com/")

		got, err := TLS(parsedURL, nil)
		if err != nil {
			t.Fatalf("TLS(%q) error = %v, want nil", parsedURL, err)
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
}

func TestTLSProbe(t *testing.T) {
	t.Run("valid_certificate", func(t *testing.T) {
		now := time.Now().UTC()

		_, parsedURL, ips, roots, cert := newTLSTestServer(t, "localhost", now.Add(-time.Hour), now.Add(72*time.Hour))

		got, err := tlsProbe(parsedURL, ips, roots)
		if err != nil {
			t.Fatalf("tlsProbe(%q) error = %v, want nil", parsedURL, err)
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
			t.Fatalf("tlsProbe(%q) error = %v, want nil", parsedURL, err)
		}

		if !got.Valid {
			t.Errorf("tlsProbe(%q).Valid = false, want true", parsedURL)
		}
	})
}

func TestVerifyCertificate(t *testing.T) {
	const hostname = "example.com"

	err := verifyCertificate(nil, hostname, nil)

	if !errors.Is(err, ErrNoPeerCertificate) {
		t.Errorf("verifyCertificate(nil, %q, nil) error = %v, want %v", hostname, err, ErrNoPeerCertificate)
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
