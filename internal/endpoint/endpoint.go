package endpoint

import (
	"uuid"

	"github.com/sgrinan/signaldock/internal/probe"
)

// Endpoint represents a monitored URL and its latest check result.
type Endpoint struct {
	ID        uuid.UUID
	URL       string
	LastCheck CheckResult
}

// CheckResult contains the HTTP and TLS results of an endpoint check.
type CheckResult struct {
	HTTP probe.HTTPResult
	TLS  probe.TLSResult
}
