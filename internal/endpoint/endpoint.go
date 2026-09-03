package endpoint

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"time"
	"uuid"
)

var (
	ErrUnsupportedScheme = errors.New("unsupported URL scheme")
	ErrHostRequired      = errors.New("URL host is required")
	ErrEndpointExists    = errors.New("endpoint already exists")
	ErrUnsafeHost        = errors.New("unsafe host")
)

type Endpoint struct {
	ID  uuid.UUID
	URL string
}

type Store struct {
	endpoints     []Endpoint
	validateHost  func(string) ([]netip.Addr, error)
	getStatusCode func(*url.URL, []netip.Addr) (int, error)
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

func ParseURL(rawURL string) (*url.URL, error) {
	rawURL = strings.TrimSpace(rawURL)

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("ParseURL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, ErrUnsupportedScheme
	}

	if parsedURL.Host == "" {
		return nil, ErrHostRequired
	}

	if parsedURL.Path == "" {
		parsedURL.Path = "/"
	}

	return parsedURL, nil
}

func GetStatusCode(parsedURL *url.URL, ips []netip.Addr) (int, error) {
	if len(ips) == 0 {
		return 0, ErrUnsafeHost
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

	resp, err := client.Get(parsedURL.String())
	if err != nil {
		return 0, fmt.Errorf("GetStatusCode: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}

func NewStore() *Store {
	return &Store{
		validateHost:  ValidateHost,
		getStatusCode: GetStatusCode,
	}
}

func (store *Store) Add(rawURL string) (Endpoint, int, error) {
	parsedURL, err := ParseURL(rawURL)
	if err != nil {
		return Endpoint{}, 0, err
	}

	ips, err := store.validateHost(parsedURL.Hostname())
	if err != nil {
		return Endpoint{}, 0, err
	}

	if store.Exists(parsedURL.String()) {
		return Endpoint{}, 0, ErrEndpointExists
	}

	id := uuid.NewV7()

	endpoint := Endpoint{
		ID:  id,
		URL: parsedURL.String(),
	}

	statusCode, err := store.getStatusCode(parsedURL, ips)
	if err != nil {
		store.endpoints = append(store.endpoints, endpoint)
		return endpoint, 0, err
	}

	store.endpoints = append(store.endpoints, endpoint)

	return endpoint, statusCode, nil
}

func (store *Store) List() []Endpoint {
	return store.endpoints
}

func (store *Store) Exists(url string) bool {
	for _, endpoint := range store.endpoints {
		if endpoint.URL == url {
			return true
		}
	}
	return false
}

func (store *Store) GetByID(id uuid.UUID) (Endpoint, bool) {
	for _, endpoint := range store.endpoints {
		if endpoint.ID == id {
			return endpoint, true
		}
	}
	return Endpoint{}, false
}

func (store *Store) RemoveByID(id uuid.UUID) bool {
	for i, endpoint := range store.endpoints {
		if endpoint.ID == id {
			store.endpoints = slices.Delete(store.endpoints, i, i+1)
			return true
		}
	}
	return false
}
