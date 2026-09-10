package web

import (
	"fmt"
	"strconv"
	"time"

	"uuid"

	"github.com/sgrinan/signaldock/internal/endpoint"
	"github.com/sgrinan/signaldock/internal/user"
)

type endpointListItem struct {
	ID               uuid.UUID
	URL              string
	State            string
	StateClass       string
	HTTPStatus       string
	HTTPState        string
	Latency          string
	LastCheckedAt    time.Time
	TLSState         string
	TLSDaysRemaining string
}

type pageData struct {
	Endpoints   []endpointListItem
	Error       string
	CSRFToken   string
	CurrentUser user.User
}

type endpointPageData struct {
	Endpoint      endpoint.Endpoint
	CSRFToken     string
	CurrentUser   user.User
	LatencyMS     int64
	LastCheckedAt time.Time
	TLSExpiresAt  string
	State         string
	StateClass    string
	TLSDaysClass  string
	Checking      bool
}

func newEndpointListItem(ep endpoint.Endpoint) endpointListItem {
	item := endpointListItem{
		ID:               ep.ID,
		URL:              ep.URL,
		State:            "Checking",
		StateClass:       "status-pending",
		HTTPStatus:       "—",
		HTTPState:        "Checking",
		Latency:          "—",
		TLSState:         "Checking",
		TLSDaysRemaining: "",
	}

	// An endpoint can briefly exist before its initial network check completes.
	if ep.LastCheck.HTTP.CheckedAt.IsZero() {
		return item
	}

	item.LastCheckedAt = ep.LastCheck.HTTP.CheckedAt
	item.Latency = fmt.Sprintf("%d ms", ep.LastCheck.HTTP.Latency.Milliseconds())

	if ep.LastCheck.HTTP.Responded {
		item.HTTPStatus = strconv.Itoa(ep.LastCheck.HTTP.StatusCode)
		item.HTTPState = "Responded"
	} else {
		item.HTTPState = "No response"
	}

	switch {
	case !ep.LastCheck.TLS.Enabled:
		item.TLSState = "Not enabled"

	case ep.LastCheck.TLS.Valid:
		item.TLSState = "Valid"

	case !ep.LastCheck.TLS.ExpiresAt.IsZero():
		item.TLSState = "Invalid"

	default:
		item.TLSState = "Unavailable"
	}

	if !ep.LastCheck.TLS.ExpiresAt.IsZero() {
		item.TLSDaysRemaining = fmt.Sprintf("%d days remaining", ep.LastCheck.TLS.DaysRemaining)
	}

	switch {
	case !ep.LastCheck.HTTP.Responded:
		item.State = "No response"
		item.StateClass = "status-error"

	case ep.LastCheck.TLS.Enabled && !ep.LastCheck.TLS.Valid:
		item.State = "TLS issue"
		item.StateClass = "status-warning"

	default:
		item.State = "Responding"
		item.StateClass = "status-ok"
	}

	return item
}

func newEndpointListItems(endpoints []endpoint.Endpoint) []endpointListItem {
	items := make([]endpointListItem, 0, len(endpoints))

	for _, ep := range endpoints {
		items = append(items, newEndpointListItem(ep))
	}

	return items
}

func tlsExpiryClass(days int) string {
	switch {
	case days <= 7:
		return "status-error"

	case days <= 30:
		return "status-warning"

	default:
		return "status-ok"
	}
}
