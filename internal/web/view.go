package web

import "github.com/sgrinan/signaldock/internal/endpoint"

// PageData contains the data rendered on the endpoint list page.
type pageData struct {
	Endpoints []endpoint.Endpoint
	Error     string
	CSRFToken string
}

// EndpointPageData contains the data rendered on an endpoint detail page.
type endpointPageData struct {
	Endpoint      endpoint.Endpoint
	CSRFToken     string
	LatencyMS     int64
	LastCheckedAt string
	TLSExpiresAt  string
}
