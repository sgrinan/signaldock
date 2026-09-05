package endpoint

import (
	"uuid"

	"github.com/sgrinan/signaldock/internal/probe"
)

type Endpoint struct {
	ID        uuid.UUID
	URL       string
	LastCheck CheckResult
}

type CheckResult struct {
	HTTP probe.Result
	TLS  probe.TLSResult
}
