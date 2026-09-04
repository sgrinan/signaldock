package probe

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"time"
)

var ErrUnsafeHost = errors.New("unsafe host")

type Result struct {
	StatusCode int
	Latency    time.Duration
	Available  bool
	CheckedAt  time.Time
}

type TLSResult struct {
	Enabled       bool
	Valid         bool
	ExpiresAt     time.Time
	DaysRemaining int
}

func isUnsafeAddr(addr netip.Addr) bool {
	return addr.IsPrivate() ||
		addr.IsLoopback() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsMulticast() ||
		addr.IsUnspecified()
}

func ValidateHost(host string) ([]netip.Addr, error) {
	addr, err := netip.ParseAddr(host)
	if err == nil {
		if isUnsafeAddr(addr) {
			return nil, ErrUnsafeHost
		}
		return []netip.Addr{addr}, nil
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		2*time.Second,
	)
	defer cancel()

	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("ValidateHost: %w", err)
	}

	if len(ips) == 0 {
		return nil, ErrUnsafeHost
	}

	if slices.ContainsFunc(ips, isUnsafeAddr) {
		return nil, ErrUnsafeHost
	}

	return ips, nil
}

func newHTTPClient(parsedURL *url.URL, ips []netip.Addr) (*http.Client, error) {
	if len(ips) == 0 {
		return nil, ErrUnsafeHost
	}

	validatedIPs := map[string][]netip.Addr{
		parsedURL.Hostname(): ips,
	}

	dialer := net.Dialer{
		Timeout: 5 * time.Second,
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network string, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}

			ips, ok := validatedIPs[host]
			if !ok || len(ips) == 0 {
				return nil, ErrUnsafeHost
			}

			var lastErr error
			for _, ip := range ips {
				target := net.JoinHostPort(ip.String(), port)

				conn, err := dialer.DialContext(ctx, network, target)
				if err == nil {
					return conn, nil
				}

				lastErr = err
			}

			return nil, lastErr
		},
	}

	client := http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			host := req.URL.Hostname()

			ips, err := ValidateHost(host)
			if err != nil {
				return err
			}

			validatedIPs[host] = ips
			return nil
		},
	}

	return &client, nil
}

func doRequest(parsedURL *url.URL, ips []netip.Addr) (*http.Response, time.Duration, error) {
	client, err := newHTTPClient(parsedURL, ips)
	if err != nil {
		return nil, 0, err
	}

	start := time.Now()
	resp, err := client.Get(parsedURL.String())
	latency := time.Since(start)

	if err != nil {
		return nil, latency, fmt.Errorf("doRequest: %w", err)
	}

	return resp, latency, nil
}

func HTTP(parsedURL *url.URL, ips []netip.Addr) (Result, error) {
	if len(ips) == 0 {
		return Result{}, ErrUnsafeHost
	}

	resp, latency, err := doRequest(parsedURL, ips)
	checkedAt := time.Now()

	if err != nil {
		return Result{
			Latency:   latency,
			Available: false,
			CheckedAt: checkedAt,
		}, err
	}
	defer resp.Body.Close()

	return Result{
		StatusCode: resp.StatusCode,
		Latency:    latency,
		Available:  true,
		CheckedAt:  checkedAt,
	}, nil
}

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

	config := newTLSConfig(parsedURL.Hostname())
	config.RootCAs = rootCAs

	port := parsedURL.Port()
	if port == "" {
		port = "443"
	}

	var lastErr error
	for _, ip := range ips {
		target := net.JoinHostPort(ip.String(), port)

		dialer := &net.Dialer{
			Timeout: 5 * time.Second,
		}

		conn, err := tls.DialWithDialer(dialer, "tcp", target, config)
		if err != nil {
			lastErr = err
			continue
		}

		state := conn.ConnectionState()

		if len(state.PeerCertificates) == 0 {
			conn.Close()

			return TLSResult{
				Enabled: true,
			}, errors.New("TLS connection returned no peer certificate")
		}

		cert := state.PeerCertificates[0]

		expiresAt := cert.NotAfter
		daysRemaining := int(time.Until(expiresAt).Hours() / 24)

		conn.Close()

		return TLSResult{
			Enabled:       true,
			Valid:         true,
			ExpiresAt:     expiresAt,
			DaysRemaining: daysRemaining,
		}, nil
	}

	return TLSResult{
		Enabled: true,
	}, lastErr
}

func newTLSConfig(serverName string) *tls.Config {
	return &tls.Config{
		ServerName: serverName,
		MinVersion: tls.VersionTLS12,
	}
}
