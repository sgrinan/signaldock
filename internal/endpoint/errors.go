package endpoint

import "errors"

var (
	ErrUnsupportedScheme     = errors.New("unsupported URL scheme")
	ErrHostRequired          = errors.New("URL host is required")
	ErrCredentialsNotAllowed = errors.New("credentials in URL are not allowed")
	ErrInvalidPort           = errors.New("invalid URL port")
	ErrInvalidURL            = errors.New("invalid URL")
	ErrUnsafeHost            = errors.New("unsafe endpoint host")

	ErrEndpointExists   = errors.New("endpoint already exists")
	ErrEndpointNotFound = errors.New("endpoint not found")
)

// CheckError contains failures produced by the HTTP and TLS checks.
type CheckError struct {
	Host error
	HTTP error
	TLS  error
}

func (err CheckError) Error() string {
	switch {
	case err.Host != nil:
		return "host check failed"
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

	if err.Host != nil {
		errs = append(errs, err.Host)
	}

	if err.HTTP != nil {
		errs = append(errs, err.HTTP)
	}

	if err.TLS != nil {
		errs = append(errs, err.TLS)
	}

	return errs
}
