package endpoint

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr string
	}{
		{
			name: "valid http URL",
			url:  "http://example.com/",
		},
		{
			name: "valid https URL",
			url:  "https://example.com/",
		},
		{
			name: "valid URL with path",
			url:  "https://example.com/health",
		},
		{
			name:    "missing scheme",
			url:     "example.com",
			wantErr: "unsupported URL scheme",
		},
		{
			name:    "missing host",
			url:     "https://",
			wantErr: "URL host is required",
		},
		{
			name:    "unsupported scheme",
			url:     "ftp://example.com/",
			wantErr: "unsupported URL scheme",
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: "unsupported URL scheme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseURL(tt.url)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ParseURL() unexpected error = %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("ParseURL() error = nil, want %q", tt.wantErr)
			}

			if err.Error() != tt.wantErr {
				t.Fatalf("ParseURL() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestGetStatusCode(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{
			name:       "status code 200",
			statusCode: http.StatusOK,
		},
		{
			name:       "status code 503",
			statusCode: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			})

			server := httptest.NewServer(handler)
			defer server.Close()

			parsedURL, err := ParseURL(server.URL)
			if err != nil {
				t.Fatalf("ParseURL() unexpected error = %v", err)
			}

			statusCode, err := GetStatusCode(parsedURL)
			if err != nil {
				t.Fatalf("GetStatusCode() unexpected error = %v", err)
			}

			if statusCode != tt.statusCode {
				t.Errorf("GetStatusCode() = %d, want %d", statusCode, tt.statusCode)
			}
		})
	}
}
