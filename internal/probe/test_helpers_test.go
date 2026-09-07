package probe

import (
	"net/url"
	"testing"
)

// Test helpers

func mustParseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("url.Parse(%q) returned unexpected error: %v", rawURL, err)
	}

	return parsedURL
}
