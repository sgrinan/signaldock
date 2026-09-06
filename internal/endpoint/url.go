package endpoint

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// ParseURL parses and normalizes an HTTP or HTTPS endpoint URL.
func ParseURL(rawURL string) (*url.URL, error) {
	rawURL = strings.TrimSpace(rawURL)

	if rawURL == "" {
		return nil, ErrInvalidURL
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}

	parsedURL.Scheme = strings.ToLower(parsedURL.Scheme)

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, ErrUnsupportedScheme
	}

	if parsedURL.Hostname() == "" {
		return nil, ErrHostRequired
	}

	if parsedURL.User != nil {
		return nil, ErrCredentialsNotAllowed
	}

	if strings.HasSuffix(parsedURL.Host, ":") {
		return nil, ErrInvalidPort
	}

	hostname := strings.ToLower(parsedURL.Hostname())
	port := parsedURL.Port()

	if port != "" {
		portNumber, err := strconv.Atoi(port)
		if err != nil || portNumber < 1 || portNumber > 65535 {
			return nil, ErrInvalidPort
		}

		switch {
		case parsedURL.Scheme == "http" && portNumber == 80:
			port = ""
		case parsedURL.Scheme == "https" && portNumber == 443:
			port = ""
		default:
			port = strconv.Itoa(portNumber)
		}
	}

	// Rebuild Host from its normalized components so equivalent URLs
	// share the same representation while preserving IPv6 brackets.
	switch {
	case port != "":
		parsedURL.Host = net.JoinHostPort(hostname, port)

	case strings.Contains(hostname, ":"):
		parsedURL.Host = "[" + hostname + "]"

	default:
		parsedURL.Host = hostname
	}

	if parsedURL.Path == "" {
		parsedURL.Path = "/"
	}

	// Fragments are client-side only and are never sent in HTTP requests.
	parsedURL.Fragment = ""
	parsedURL.RawFragment = ""

	return parsedURL, nil
}
