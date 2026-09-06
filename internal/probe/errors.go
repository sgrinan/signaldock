package probe

import "errors"

var (
	ErrUnsafeHost                = errors.New("unsafe host")
	ErrTooManyRedirects          = errors.New("too many redirects")
	ErrUnsupportedRedirectScheme = errors.New("unsupported redirect scheme")
	ErrRedirectCredentials       = errors.New("credentials in redirect URL are not allowed")
	ErrNoResolvedAddresses       = errors.New("host resolved to no IP addresses")

	ErrNoPeerCertificate = errors.New("tls connection returned no peer certificate")
)
