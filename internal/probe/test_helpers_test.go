package probe

import (
	"net/url"
	"testing"
)

func mustParseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v, want nil", rawURL, err)
	}

	return parsedURL
}
