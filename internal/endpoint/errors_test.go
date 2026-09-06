package endpoint

import (
	"errors"
	"testing"
)

func TestCheckError_Error(t *testing.T) {
	httpErr := errors.New("http failed")
	tlsErr := errors.New("tls failed")

	tests := []struct {
		name string
		err  CheckError
		want string
	}{
		{
			name: "http_and_tls",
			err: CheckError{
				HTTP: httpErr,
				TLS:  tlsErr,
			},
			want: "HTTP and TLS checks failed",
		},
		{
			name: "http_only",
			err: CheckError{
				HTTP: httpErr,
			},
			want: "HTTP check failed",
		},
		{
			name: "tls_only",
			err: CheckError{
				TLS: tlsErr,
			},
			want: "TLS check failed",
		},
		{
			name: "no_errors",
			err:  CheckError{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("CheckError.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheckError_Unwrap(t *testing.T) {
	httpErr := errors.New("http failed")
	tlsErr := errors.New("tls failed")
	otherErr := errors.New("other")

	err := CheckError{
		HTTP: httpErr,
		TLS:  tlsErr,
	}

	if !errors.Is(err, httpErr) {
		t.Errorf("errors.Is(CheckError, httpErr) = false, want true")
	}

	if !errors.Is(err, tlsErr) {
		t.Errorf("errors.Is(CheckError, tlsErr) = false, want true")
	}

	if errors.Is(err, otherErr) {
		t.Errorf("errors.Is(CheckError, otherErr) = true, want false")
	}
}
