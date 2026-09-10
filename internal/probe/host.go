package probe

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"time"
)

const resolverTimeout = 2 * time.Second

// sharedAddressPrefix is the RFC 6598 shared address space used by CGNAT.
var sharedAddressPrefix = netip.MustParsePrefix("100.64.0.0/10")

type lookupNetIPFunc func(
	context.Context,
	string,
	string,
) ([]netip.Addr, error)

// ValidateHost resolves host and rejects local, private, and other
// disallowed destinations for outbound probes.
func ValidateHost(host string) ([]netip.Addr, error) {
	return validateHost(host, net.DefaultResolver.LookupNetIP)
}

func isUnsafeAddr(addr netip.Addr) bool {
	addr = addr.Unmap()

	return !addr.IsGlobalUnicast() ||
		addr.IsPrivate() ||
		sharedAddressPrefix.Contains(addr)
}

func validateHost(host string, lookup lookupNetIPFunc) ([]netip.Addr, error) {
	addr, err := netip.ParseAddr(host)
	if err == nil {
		addr = addr.Unmap()

		if isUnsafeAddr(addr) {
			return nil, ErrUnsafeHost
		}

		return []netip.Addr{addr}, nil
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		resolverTimeout,
	)
	defer cancel()

	ips, err := lookup(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("resolve host %q: %w", host, err)
	}

	if len(ips) == 0 {
		return nil, ErrNoResolvedAddresses
	}

	for i, addr := range ips {
		addr = addr.Unmap()

		if isUnsafeAddr(addr) {
			return nil, ErrUnsafeHost
		}

		ips[i] = addr
	}

	return ips, nil
}
