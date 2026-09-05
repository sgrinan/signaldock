package endpoint

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

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
