package probe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

const (
	connectTimeout = 5 * time.Second
	requestTimeout = 5 * time.Second
	maxRedirects   = 10
)

// HTTP probes an endpoint and reports whether an HTTP response was received.
func HTTP(ctx context.Context, parsedURL *url.URL, ips []netip.Addr) (HTTPResult, error) {
	if len(ips) == 0 {
		return HTTPResult{}, ErrUnsafeHost
	}

	resp, latency, err := doRequest(ctx, parsedURL, ips)
	checkedAt := time.Now()

	if err != nil {
		result := HTTPResult{
			Latency:   latency,
			CheckedAt: checkedAt,
		}

		if resp != nil {
			result.StatusCode = resp.StatusCode
			result.Responded = true

			if resp.Body != nil {
				resp.Body.Close()
			}
		}

		return result, err
	}

	defer resp.Body.Close()

	return HTTPResult{
		StatusCode: resp.StatusCode,
		Latency:    latency,
		Responded:  true,
		CheckedAt:  checkedAt,
	}, nil
}

func newHTTPClient(parsedURL *url.URL, ips []netip.Addr) (*http.Client, error) {
	if len(ips) == 0 {
		return nil, ErrUnsafeHost
	}

	hostname := strings.ToLower(parsedURL.Hostname())

	validatedIPs := map[string][]netip.Addr{
		hostname: ips,
	}

	dialer := net.Dialer{
		Timeout: connectTimeout,
	}

	transport := &http.Transport{
		DisableKeepAlives: true,

		// Dial only previously validated IPs so DNS cannot change between
		// host validation and the outbound connection.
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}

			host = strings.ToLower(host)

			hostIPs, ok := validatedIPs[host]
			if !ok || len(hostIPs) == 0 {
				return nil, ErrUnsafeHost
			}

			var lastErr error

			for _, ip := range hostIPs {
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
		Timeout:   requestTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			req.Header.Del("Referer")

			if len(via) >= maxRedirects {
				return ErrTooManyRedirects
			}

			scheme := strings.ToLower(req.URL.Scheme)

			if scheme != "http" && scheme != "https" {
				return ErrUnsupportedRedirectScheme
			}

			if req.URL.User != nil {
				return ErrRedirectCredentials
			}

			host := strings.ToLower(req.URL.Hostname())

			ips, err := ValidateHost(req.Context(), host)
			if err != nil {
				return err
			}

			// Pin every redirect destination to the addresses that passed
			// validation before the redirected request is sent.
			validatedIPs[host] = ips

			return nil
		},
	}

	return &client, nil
}

func doRequest(ctx context.Context, parsedURL *url.URL, ips []netip.Addr) (*http.Response, time.Duration, error) {
	client, err := newHTTPClient(parsedURL, ips)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, 0, fmt.Errorf("create HTTP request: %w", err)
	}

	start := time.Now()

	resp, err := client.Do(req)

	latency := time.Since(start)

	if err != nil {
		return resp, latency, fmt.Errorf("HTTP request: %w", sanitizeRequestError(err))
	}

	return resp, latency, nil
}

func sanitizeRequestError(err error) error {

	if urlErr, ok := errors.AsType[*url.Error](err); ok {
		return urlErr.Err
	}

	return err
}
