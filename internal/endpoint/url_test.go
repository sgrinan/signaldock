package endpoint

import (
	"errors"
	"testing"
)

func TestParseURL(t *testing.T) {
	tests := []struct {
		name   string
		rawURL string
		want   string
	}{
		{
			name:   "http",
			rawURL: "http://example.com",
			want:   "http://example.com/",
		},
		{
			name:   "https",
			rawURL: "https://example.com",
			want:   "https://example.com/",
		},
		{
			name:   "trims_whitespace",
			rawURL: "  https://example.com  ",
			want:   "https://example.com/",
		},
		{
			name:   "normalizes_scheme_and_hostname",
			rawURL: "HTTPS://EXAMPLE.COM",
			want:   "https://example.com/",
		},
		{
			name:   "preserves_path",
			rawURL: "https://example.com/api/health",
			want:   "https://example.com/api/health",
		},
		{
			name:   "preserves_query",
			rawURL: "https://example.com/health?full=true",
			want:   "https://example.com/health?full=true",
		},
		{
			name:   "removes_fragment",
			rawURL: "https://example.com/health?full=true#details",
			want:   "https://example.com/health?full=true",
		},
		{
			name:   "removes_default_http_port",
			rawURL: "http://example.com:80",
			want:   "http://example.com/",
		},
		{
			name:   "removes_default_https_port",
			rawURL: "https://example.com:443",
			want:   "https://example.com/",
		},
		{
			name:   "normalizes_default_port",
			rawURL: "https://example.com:0443",
			want:   "https://example.com/",
		},
		{
			name:   "preserves_custom_port",
			rawURL: "https://example.com:8443",
			want:   "https://example.com:8443/",
		},
		{
			name:   "normalizes_custom_port",
			rawURL: "https://example.com:08443",
			want:   "https://example.com:8443/",
		},
		{
			name:   "ipv6",
			rawURL: "https://[2001:4860:4860::8888]",
			want:   "https://[2001:4860:4860::8888]/",
		},
		{
			name:   "ipv6_with_custom_port",
			rawURL: "https://[2001:4860:4860::8888]:8443",
			want:   "https://[2001:4860:4860::8888]:8443/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedURL, err := ParseURL(tt.rawURL)
			if err != nil {
				t.Fatalf(
					"ParseURL(%q) returned unexpected error: %v",
					tt.rawURL,
					err,
				)
			}

			if got := parsedURL.String(); got != tt.want {
				t.Errorf(
					"ParseURL(%q) = %q, want %q",
					tt.rawURL,
					got,
					tt.want,
				)
			}
		})
	}
}

func TestParseURL_Error(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		wantErr error
	}{
		{
			name:    "empty",
			rawURL:  "",
			wantErr: ErrInvalidURL,
		},
		{
			name:    "missing_scheme",
			rawURL:  "example.com",
			wantErr: ErrUnsupportedScheme,
		},
		{
			name:    "unsupported_scheme",
			rawURL:  "ftp://example.com",
			wantErr: ErrUnsupportedScheme,
		},
		{
			name:    "missing_host",
			rawURL:  "https://",
			wantErr: ErrHostRequired,
		},
		{
			name:    "credentials",
			rawURL:  "https://user:password@example.com",
			wantErr: ErrCredentialsNotAllowed,
		},
		{
			name:    "empty_port",
			rawURL:  "https://example.com:",
			wantErr: ErrInvalidPort,
		},
		{
			name:    "zero_port",
			rawURL:  "https://example.com:0",
			wantErr: ErrInvalidPort,
		},
		{
			name:    "port_above_maximum",
			rawURL:  "https://example.com:65536",
			wantErr: ErrInvalidPort,
		},
		{
			name:    "malformed",
			rawURL:  "https://example.com/%zz",
			wantErr: ErrInvalidURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseURL(tt.rawURL)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf(
					"ParseURL(%q) error = %v, want %v",
					tt.rawURL,
					err,
					tt.wantErr,
				)
			}

			if got != nil {
				t.Errorf(
					"ParseURL(%q) = %v, want nil",
					tt.rawURL,
					got,
				)
			}
		})
	}
}
