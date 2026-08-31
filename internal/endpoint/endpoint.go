package endpoint

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
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

func GetStatusCode(parsedURL *url.URL) (int, error) {
	client := http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(parsedURL.String())
	if err != nil {
		return 0, fmt.Errorf("GetStatusCode: %w", err)
	}

	defer resp.Body.Close()

	return resp.StatusCode, nil
}
