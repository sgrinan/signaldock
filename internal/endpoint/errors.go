package endpoint

import "errors"

var (
	ErrUnsupportedScheme = errors.New("unsupported URL scheme")
	ErrHostRequired      = errors.New("URL host is required")
	ErrEndpointExists    = errors.New("endpoint already exists")
	ErrEndpointNotFound  = errors.New("endpoint not found")
)

type CheckError struct {
	HTTP error
	TLS  error
}

func (err CheckError) Error() string {
	switch {
	case err.HTTP != nil && err.TLS != nil:
		return "HTTP and TLS checks failed"
	case err.HTTP != nil:
		return "HTTP check failed"
	case err.TLS != nil:
		return "TLS check failed"
	default:
		return ""
	}
}

func (err CheckError) Unwrap() []error {
	var errs []error

	if err.HTTP != nil {
		errs = append(errs, err.HTTP)
	}

	if err.TLS != nil {
		errs = append(errs, err.TLS)
	}

	return errs
}
