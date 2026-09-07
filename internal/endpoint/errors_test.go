package endpoint

import (
	"errors"
	"testing"
)

// CheckError.Error

func TestCheckError_Error(t *testing.T) {
	hostErr := errors.New("host failed")
	httpErr := errors.New("http failed")
	tlsErr := errors.New("tls failed")

	tests := []struct {
		name string
		err  CheckError
		want string
	}{
		{
			name: "host_only",
			err: CheckError{
				Host: hostErr,
			},
			want: "host check failed",
		},
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

// CheckError.Unwrap

func TestCheckError_Unwrap(t *testing.T) {
	hostErr := errors.New("host failed")
	httpErr := errors.New("http failed")
	tlsErr := errors.New("tls failed")
	otherErr := errors.New("other")

	err := CheckError{
		Host: hostErr,
		HTTP: httpErr,
		TLS:  tlsErr,
	}

	if !errors.Is(err, hostErr) {
		t.Error("errors.Is(CheckError, hostErr) = false, want true")
	}

	if !errors.Is(err, httpErr) {
		t.Error("errors.Is(CheckError, httpErr) = false, want true")
	}

	if !errors.Is(err, tlsErr) {
		t.Error("errors.Is(CheckError, tlsErr) = false, want true")
	}

	if errors.Is(err, otherErr) {
		t.Error("errors.Is(CheckError, otherErr) = true, want false")
	}
}
