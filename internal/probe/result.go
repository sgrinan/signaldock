package probe

import "time"

// HTTPResult contains the observable result of an HTTP probe.
type HTTPResult struct {
	StatusCode int
	Latency    time.Duration
	Responded  bool
	CheckedAt  time.Time
}

// TLSResult contains the observable TLS state of an endpoint.
type TLSResult struct {
	Enabled       bool
	Valid         bool
	ExpiresAt     time.Time
	DaysRemaining int
}
