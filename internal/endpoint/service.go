package endpoint

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"time"

	"uuid"

	"github.com/sgrinan/signaldock/internal/probe"
)

// Service coordinates endpoint validation, checks, and storage.
type Service struct {
	store        endpointStore
	validateHost func(string) ([]netip.Addr, error)
	probeHTTP    func(*url.URL, []netip.Addr) (probe.HTTPResult, error)
	probeTLS     func(*url.URL, []netip.Addr) (probe.TLSResult, error)
}

type endpointStore interface {
	Insert(endpoint Endpoint) error
	List() ([]Endpoint, error)
	ByID(id uuid.UUID) (Endpoint, error)
	RemoveByID(id uuid.UUID) error
	UpdateLastCheck(id uuid.UUID, result CheckResult) error
}

// NewService returns a Service backed by the provided endpoint store.
func NewService(store endpointStore) *Service {
	return &Service{
		store:        store,
		validateHost: probe.ValidateHost,
		probeHTTP:    probe.HTTP,
		probeTLS:     probe.TLS,
	}
}

// Add validates and checks rawURL, then stores the resulting endpoint.
// Probe failures do not prevent the endpoint or its observed results from being stored.
func (s *Service) Add(rawURL string) (Endpoint, error) {
	parsedURL, err := ParseURL(rawURL)
	if err != nil {
		return Endpoint{}, err
	}

	ep := Endpoint{
		ID:  uuid.NewV7(),
		URL: parsedURL.String(),
	}

	ips, err := s.validateEndpointHost(parsedURL)
	if err != nil {
		if errors.Is(err, ErrUnsafeHost) {
			return Endpoint{}, err
		}

		ep.LastCheck = hostFailureResult(parsedURL)

		if insertErr := s.store.Insert(ep); insertErr != nil {
			return Endpoint{}, insertErr
		}

		return ep, CheckError{
			Host: err,
		}
	}

	// Reserve the normalized endpoint before performing slow network I/O.
	// Store.Insert performs the duplicate check atomically, so concurrent
	// Add calls for the same URL cannot all execute the probes.
	if err := s.store.Insert(ep); err != nil {
		return Endpoint{}, err
	}

	lastCheck, checkErr := s.check(parsedURL, ips)
	ep.LastCheck = lastCheck

	if err := s.store.UpdateLastCheck(ep.ID, lastCheck); err != nil {
		return ep, err
	}

	return ep, checkErr
}

// Refresh checks an existing endpoint and stores its latest observed result.
// Probe failures are returned after the result has been stored.
func (s *Service) Refresh(id uuid.UUID) (CheckResult, error) {
	ep, err := s.store.ByID(id)
	if err != nil {
		return CheckResult{}, err
	}

	parsedURL, err := ParseURL(ep.URL)
	if err != nil {
		return CheckResult{}, err
	}

	ips, err := s.validateEndpointHost(parsedURL)
	if err != nil {
		lastCheck := hostFailureResult(parsedURL)

		if updateErr := s.store.UpdateLastCheck(id, lastCheck); updateErr != nil {
			return lastCheck, updateErr
		}

		return lastCheck, CheckError{
			Host: err,
		}
	}

	lastCheck, checkErr := s.check(parsedURL, ips)

	if err := s.store.UpdateLastCheck(id, lastCheck); err != nil {
		return lastCheck, err
	}

	return lastCheck, checkErr
}

// List returns a snapshot of the stored endpoints.
func (s *Service) List() ([]Endpoint, error) {
	return s.store.List()
}

// ByID returns the endpoint with the given ID.
func (s *Service) ByID(id uuid.UUID) (Endpoint, error) {
	return s.store.ByID(id)
}

// RemoveByID removes the endpoint with the given ID.
func (s *Service) RemoveByID(id uuid.UUID) error {
	return s.store.RemoveByID(id)
}

// validateEndpointHost validates the endpoint destination and translates
// probe-level host safety errors into endpoint-domain errors.
func (s *Service) validateEndpointHost(parsedURL *url.URL) ([]netip.Addr, error) {
	ips, err := s.validateHost(parsedURL.Hostname())
	if err == nil {
		return ips, nil
	}

	if errors.Is(err, probe.ErrUnsafeHost) {
		return nil, ErrUnsafeHost
	}

	return nil, fmt.Errorf("validate endpoint host: %w", err)
}

// check runs the HTTP and TLS probes independently so both results and
// failures are preserved.
func (s *Service) check(parsedURL *url.URL, ips []netip.Addr) (CheckResult, error) {
	httpResult, httpErr := s.probeHTTP(parsedURL, ips)
	tlsResult, tlsErr := s.probeTLS(parsedURL, ips)

	result := CheckResult{
		HTTP: httpResult,
		TLS:  tlsResult,
	}

	if httpErr != nil || tlsErr != nil {
		return result, CheckError{
			HTTP: httpErr,
			TLS:  tlsErr,
		}
	}

	return result, nil
}

func hostFailureResult(parsedURL *url.URL) CheckResult {
	return CheckResult{
		HTTP: probe.HTTPResult{
			CheckedAt: time.Now(),
		},
		TLS: probe.TLSResult{
			Enabled: parsedURL.Scheme == "https",
		},
	}
}
