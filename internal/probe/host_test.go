package probe

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"testing"
)

func TestIsUnsafeAddr(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want bool
	}{
		{
			name: "public_ipv4",
			addr: "8.8.8.8",
			want: false,
		},
		{
			name: "public_ipv6",
			addr: "2606:4700:4700::1111",
			want: false,
		},
		{
			name: "private_ipv4",
			addr: "10.0.0.1",
			want: true,
		},
		{
			name: "private_ipv6",
			addr: "fc00::1",
			want: true,
		},
		{
			name: "loopback_ipv4",
			addr: "127.0.0.1",
			want: true,
		},
		{
			name: "loopback_ipv6",
			addr: "::1",
			want: true,
		},
		{
			name: "link_local",
			addr: "169.254.169.254",
			want: true,
		},
		{
			name: "cgnat",
			addr: "100.64.0.1",
			want: true,
		},
		{
			name: "mapped_private_ipv4",
			addr: "::ffff:127.0.0.1",
			want: true,
		},
		{
			name: "mapped_public_ipv4",
			addr: "::ffff:8.8.8.8",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := netip.MustParseAddr(tt.addr)

			if got := isUnsafeAddr(addr); got != tt.want {
				t.Errorf("isUnsafeAddr(%v) = %t, want %t", addr, got, tt.want)
			}
		})
	}
}

func TestValidateHost(t *testing.T) {
	t.Run("public_ip", func(t *testing.T) {
		got, err := ValidateHost("8.8.8.8")
		if err != nil {
			t.Fatalf("ValidateHost(%q) returned unexpected error: %v", "8.8.8.8", err)
		}

		want := netip.MustParseAddr("8.8.8.8")

		if len(got) != 1 {
			t.Fatalf("len(ValidateHost()) = %d, want 1", len(got))
		}

		if got[0] != want {
			t.Errorf("ValidateHost(%q)[0] = %v, want %v", "8.8.8.8", got[0], want)
		}
	})

	t.Run("unsafe_ip", func(t *testing.T) {
		_, err := ValidateHost("127.0.0.1")

		if !errors.Is(err, ErrUnsafeHost) {
			t.Errorf("ValidateHost(%q) error = %v, want %v", "127.0.0.1", err, ErrUnsafeHost)
		}
	})

	t.Run("unmaps_ipv4", func(t *testing.T) {
		got, err := ValidateHost("::ffff:8.8.8.8")
		if err != nil {
			t.Fatalf("ValidateHost(%q) returned unexpected error: %v", "::ffff:8.8.8.8", err)
		}

		want := netip.MustParseAddr("8.8.8.8")

		if got[0] != want {
			t.Errorf("ValidateHost(%q)[0] = %v, want %v", "::ffff:8.8.8.8", got[0], want)
		}
	})
}

func TestValidateHost_DNS(t *testing.T) {
	t.Run("public_addresses", func(t *testing.T) {
		want := []netip.Addr{
			netip.MustParseAddr("8.8.8.8"),
			netip.MustParseAddr("2606:4700:4700::1111"),
		}

		lookup := func(context.Context, string, string) ([]netip.Addr, error) {
			return append([]netip.Addr(nil), want...), nil
		}

		got, err := validateHost("example.com", lookup)
		if err != nil {
			t.Fatalf("validateHost() returned unexpected error: %v", err)
		}

		if !slices.Equal(got, want) {
			t.Errorf("validateHost() = %v, want %v", got, want)
		}
	})

	t.Run("unmaps_resolved_ipv4", func(t *testing.T) {
		lookup := func(context.Context, string, string) ([]netip.Addr, error) {
			return []netip.Addr{
				netip.MustParseAddr("::ffff:8.8.8.8"),
			}, nil
		}

		got, err := validateHost("example.com", lookup)
		if err != nil {
			t.Fatalf("validateHost() returned unexpected error: %v", err)
		}

		want := netip.MustParseAddr("8.8.8.8")

		if len(got) != 1 {
			t.Fatalf("len(validateHost()) = %d, want 1", len(got))
		}

		if got[0] != want {
			t.Errorf("validateHost()[0] = %v, want %v", got[0], want)
		}
	})
	t.Run("contains_unsafe_address", func(t *testing.T) {
		lookup := func(context.Context, string, string) ([]netip.Addr, error) {
			return []netip.Addr{
				netip.MustParseAddr("8.8.8.8"),
				netip.MustParseAddr("127.0.0.1"),
			}, nil
		}

		_, err := validateHost("example.com", lookup)

		if !errors.Is(err, ErrUnsafeHost) {
			t.Errorf("validateHost() error = %v, want %v", err, ErrUnsafeHost)
		}
	})

	t.Run("no_addresses", func(t *testing.T) {
		lookup := func(context.Context, string, string) ([]netip.Addr, error) {
			return nil, nil
		}

		_, err := validateHost("example.com", lookup)

		if !errors.Is(err, ErrNoResolvedAddresses) {
			t.Errorf("validateHost() error = %v, want %v", err, ErrNoResolvedAddresses)
		}
	})

	t.Run("resolver_error", func(t *testing.T) {
		wantErr := errors.New("lookup failed")

		lookup := func(context.Context, string, string) ([]netip.Addr, error) {
			return nil, wantErr
		}

		_, err := validateHost("example.com", lookup)

		if !errors.Is(err, wantErr) {
			t.Errorf("validateHost() error = %v, want error wrapping %v", err, wantErr)
		}
	})
}
