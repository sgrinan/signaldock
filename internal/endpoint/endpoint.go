package endpoint

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"uuid"
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
	endpoints []Endpoint
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

func GetStatusCode(parsedURL *url.URL) (int, error) {
	client := http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(parsedURL.String())
	if err != nil {
		return 0, fmt.Errorf("GetStatusCode: %w", err)
	}

	defer resp.Body.Close()

	return resp.StatusCode, nil
}

func (store *Store) Add(rawURL string) (Endpoint, int, error) {
	parsedURL, err := ParseURL(rawURL)
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

	statusCode, err := GetStatusCode(parsedURL)
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
