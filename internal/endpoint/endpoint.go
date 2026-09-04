package endpoint

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"sync"
	"uuid"

	"github.com/sgrinan/signaldock/internal/probe"
)

var (
	ErrUnsupportedScheme = errors.New("unsupported URL scheme")
	ErrHostRequired      = errors.New("URL host is required")
	ErrEndpointExists    = errors.New("endpoint already exists")
	ErrEndpointNotFound  = errors.New("endpoint not found")
)

type Endpoint struct {
	ID         uuid.UUID
	URL        string
	LastResult probe.Result
}

type Store struct {
	mu           sync.RWMutex
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

	endpoint := Endpoint{
		ID:  uuid.NewV7(),
		URL: parsedURL.String(),
	}

	store.mu.Lock()

	if store.existsLocked(parsedURL.String()) {
		store.mu.Unlock()
		return Endpoint{}, probe.Result{}, ErrEndpointExists
	}

	store.endpoints = append(store.endpoints, endpoint)

	store.mu.Unlock()

	result, err := store.probeHTTP(parsedURL, ips)

	store.mu.Lock()

	for i := range store.endpoints {
		if store.endpoints[i].ID == endpoint.ID {
			store.endpoints[i].LastResult = result
			endpoint.LastResult = result
			break
		}
	}

	store.mu.Unlock()

	if err != nil {
		return endpoint, result, err
	}

	return endpoint, result, nil
}

func (store *Store) List() []Endpoint {
	store.mu.RLock()
	defer store.mu.RUnlock()

	endpoints := make([]Endpoint, len(store.endpoints))
	copy(endpoints, store.endpoints)

	return endpoints
}

func (store *Store) Exists(url string) bool {
	store.mu.RLock()
	defer store.mu.RUnlock()

	return store.existsLocked(url)
}

func (store *Store) existsLocked(url string) bool {
	for _, endpoint := range store.endpoints {
		if endpoint.URL == url {
			return true
		}
	}
	return false
}

func (store *Store) GetByID(id uuid.UUID) (Endpoint, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	for _, endpoint := range store.endpoints {
		if endpoint.ID == id {
			return endpoint, true
		}
	}

	return Endpoint{}, false
}

func (store *Store) RemoveByID(id uuid.UUID) bool {
	store.mu.Lock()
	defer store.mu.Unlock()

	for i, endpoint := range store.endpoints {
		if endpoint.ID == id {
			store.endpoints = slices.Delete(store.endpoints, i, i+1)
			return true
		}
	}
	return false
}

func (store *Store) Refresh(id uuid.UUID) (probe.Result, error) {
	endpoint, found := store.GetByID(id)
	if !found {
		return probe.Result{}, ErrEndpointNotFound
	}

	parsedURL, err := ParseURL(endpoint.URL)
	if err != nil {
		return probe.Result{}, err
	}

	ips, err := store.validateHost(parsedURL.Hostname())
	if err != nil {
		return probe.Result{}, err
	}

	result, err := store.probeHTTP(parsedURL, ips)

	store.mu.Lock()
	for i := range store.endpoints {
		if store.endpoints[i].ID == id {
			store.endpoints[i].LastResult = result
			break
		}
	}
	store.mu.Unlock()

	if err != nil {
		return result, err
	}

	return result, nil
}
