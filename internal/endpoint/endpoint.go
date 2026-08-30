package endpoint

import (
	"errors"
	"fmt"
	"net/url"
)

func ParseURL(rawURL string) (*url.URL, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("ParseURL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, errors.New("unsupported URL scheme")
	}

	if parsedURL.Host == "" {
		return nil, errors.New("URL host is required")
	}

	return parsedURL, nil
}
