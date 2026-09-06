package probe

import (
	"crypto/tls"
	"crypto/x509"
	"math"
	"net"
	"net/netip"
	"net/url"
	"time"
)

// TLS probes the TLS configuration and certificate of an HTTPS endpoint.
func TLS(parsedURL *url.URL, ips []netip.Addr) (TLSResult, error) {
	return tlsProbe(parsedURL, ips, nil)
}

func tlsProbe(parsedURL *url.URL, ips []netip.Addr, rootCAs *x509.CertPool) (TLSResult, error) {
	if parsedURL.Scheme != "https" {
		return TLSResult{}, nil
	}

	if len(ips) == 0 {
		return TLSResult{}, ErrUnsafeHost
	}

	port := parsedURL.Port()
	if port == "" {
		port = "443"
	}

	config := newTLSConfig(parsedURL.Hostname())

	dialer := &net.Dialer{
		Timeout: connectTimeout,
	}

	var lastErr error

	for _, ip := range ips {
		target := net.JoinHostPort(ip.String(), port)

		conn, err := tls.DialWithDialer(dialer, "tcp", target, config)
		if err != nil {
			lastErr = err
			continue
		}

		state := conn.ConnectionState()
		_ = conn.Close()

		if len(state.PeerCertificates) == 0 {
			return TLSResult{
				Enabled: true,
			}, ErrNoPeerCertificate
		}

		cert := state.PeerCertificates[0]

		result := TLSResult{
			Enabled:       true,
			ExpiresAt:     cert.NotAfter,
			DaysRemaining: daysBetween(time.Now(), cert.NotAfter),
		}

		if err := verifyCertificate(state.PeerCertificates, parsedURL.Hostname(), rootCAs); err != nil {
			return result, err
		}

		result.Valid = true

		return result, nil
	}

	return TLSResult{
		Enabled: true,
	}, lastErr
}

func newTLSConfig(serverName string) *tls.Config {
	return &tls.Config{
		ServerName: serverName,
		MinVersion: tls.VersionTLS12,

		// Certificate verification is performed explicitly after the
		// handshake so invalid certificates can still be inspected.
		InsecureSkipVerify: true,
	}
}

func verifyCertificate(certificates []*x509.Certificate, serverName string, rootCAs *x509.CertPool) error {
	if len(certificates) == 0 {
		return ErrNoPeerCertificate
	}

	intermediates := x509.NewCertPool()

	for _, cert := range certificates[1:] {
		intermediates.AddCert(cert)
	}

	_, err := certificates[0].Verify(x509.VerifyOptions{
		DNSName:       serverName,
		Roots:         rootCAs,
		Intermediates: intermediates,
		KeyUsages: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
	})

	return err
}

func daysBetween(now, expiresAt time.Time) int {
	return int(math.Floor(expiresAt.Sub(now).Hours() / 24))
}
