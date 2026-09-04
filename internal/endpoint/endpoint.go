package endpoint

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"uuid"

	"github.com/sgrinan/signaldock/internal/probe"
)

var (
	ErrUnsupportedScheme = errors.New("unsupported URL scheme")
	ErrHostRequired      = errors.New("URL host is required")
	ErrEndpointExists    = errors.New("endpoint already exists")
)

type Endpoint struct {
	ID  uuid.UUID
	URL string
}

type Store struct {
	endpoints    []Endpoint
	validateHost func(string) ([]netip.Addr, error)
	probeHTTP    func(*url.URL, []netip.Addr) (probe.Result, error)
}

func ParseURL(rawURL string) (*url.URL, error) {
	rawURL = strings.TrimSpace(rawURL)

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("ParseURL: %w", err)
	}

	parsedURL.Scheme = strings.ToLower(parsedURL.Scheme)

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, ErrUnsupportedScheme
	}

	if parsedURL.Hostname() == "" {
		return nil, ErrHostRequired
	}

	hostname := strings.ToLower(parsedURL.Hostname())
	port := parsedURL.Port()

	if port != "" {
		parsedURL.Host = net.JoinHostPort(hostname, port)
	} else if strings.Contains(hostname, ":") {
		parsedURL.Host = "[" + hostname + "]"
	} else {
		parsedURL.Host = hostname
	}

	if parsedURL.Path == "" {
		parsedURL.Path = "/"
	}

	return parsedURL, nil
}

func NewStore() *Store {
	return &Store{
		validateHost: probe.ValidateHost,
		probeHTTP:    probe.HTTP,
	}
}

func (store *Store) Add(rawURL string) (Endpoint, probe.Result, error) {
	parsedURL, err := ParseURL(rawURL)
	if err != nil {
		return Endpoint{}, probe.Result{}, err
	}

	ips, err := store.validateHost(parsedURL.Hostname())
	if err != nil {
		return Endpoint{}, probe.Result{}, err
	}

	if store.Exists(parsedURL.String()) {
		return Endpoint{}, probe.Result{}, ErrEndpointExists
	}

	id := uuid.NewV7()

	endpoint := Endpoint{
		ID:  id,
		URL: parsedURL.String(),
	}

	result, err := store.probeHTTP(parsedURL, ips)
	if err != nil {
		store.endpoints = append(store.endpoints, endpoint)
		return endpoint, result, err
	}

	store.endpoints = append(store.endpoints, endpoint)

	return endpoint, result, nil
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
