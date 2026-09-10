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

// Service coordinates endpoint validation, checks, and persistence.
type Service struct {
	repository endpointRepository
	checks     checkStore

	validateHost func(string) ([]netip.Addr, error)
	probeHTTP    func(*url.URL, []netip.Addr) (probe.HTTPResult, error)
	probeTLS     func(*url.URL, []netip.Addr) (probe.TLSResult, error)
}

type endpointRepository interface {
	Insert(endpoint Endpoint) error
	List() ([]Endpoint, error)
	ByID(id uuid.UUID) (Endpoint, error)
	RemoveByID(id uuid.UUID) error
}

type checkStore interface {
	Set(id uuid.UUID, result CheckResult)
	Get(id uuid.UUID) CheckResult
	Delete(id uuid.UUID)
}

// NewService returns a Service backed by the provided repository and check store.
func NewService(repository endpointRepository, checks checkStore) *Service {
	return &Service{
		repository:   repository,
		checks:       checks,
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

		lastCheck := hostFailureResult(parsedURL)

		if err := s.repository.Insert(ep); err != nil {
			return Endpoint{}, err
		}

		s.checks.Set(ep.ID, lastCheck)
		ep.LastCheck = lastCheck

		return ep, CheckError{
			Host: err,
		}
	}

	// Reserve the normalized endpoint before performing slow network I/O.
	// Repository.Insert performs the duplicate check atomically, so concurrent
	// Add calls for the same URL cannot all execute the probes.
	if err := s.repository.Insert(ep); err != nil {
		return Endpoint{}, err
	}

	lastCheck, checkErr := s.check(parsedURL, ips)

	s.checks.Set(ep.ID, lastCheck)
	ep.LastCheck = lastCheck

	return ep, checkErr
}

// Refresh checks an existing endpoint and stores its latest observed result.
// Probe failures are returned after the result has been stored.
func (s *Service) Refresh(id uuid.UUID) (CheckResult, error) {
	ep, err := s.repository.ByID(id)
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

		s.checks.Set(id, lastCheck)

		return lastCheck, CheckError{
			Host: err,
		}
	}

	lastCheck, checkErr := s.check(parsedURL, ips)

	s.checks.Set(id, lastCheck)

	return lastCheck, checkErr
}

// List returns a snapshot of the stored endpoints with their latest checks.
func (s *Service) List() ([]Endpoint, error) {
	endpoints, err := s.repository.List()
	if err != nil {
		return nil, err
	}

	for i := range endpoints {
		endpoints[i].LastCheck = s.checks.Get(endpoints[i].ID)
	}

	return endpoints, nil
}

// ByID returns the endpoint with the given ID and its latest check.
func (s *Service) ByID(id uuid.UUID) (Endpoint, error) {
	ep, err := s.repository.ByID(id)
	if err != nil {
		return Endpoint{}, err
	}

	ep.LastCheck = s.checks.Get(id)

	return ep, nil
}

// RemoveByID removes the endpoint with the given ID.
func (s *Service) RemoveByID(id uuid.UUID) error {
	if err := s.repository.RemoveByID(id); err != nil {
		return err
	}

	s.checks.Delete(id)

	return nil
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
