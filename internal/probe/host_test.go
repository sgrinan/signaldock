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
		const host = "8.8.8.8"

		got, err := ValidateHost(t.Context(), host)
		if err != nil {
			t.Fatalf("ValidateHost(%q) returned unexpected error: %v", host, err)
		}

		want := netip.MustParseAddr(host)

		if gotLen, wantLen := len(got), 1; gotLen != wantLen {
			t.Fatalf("len(ValidateHost(%q)) = %d, want %d", host, gotLen, wantLen)
		}

		if got[0] != want {
			t.Errorf("ValidateHost(%q)[0] = %v, want %v", host, got[0], want)
		}
	})

	t.Run("unsafe_ip", func(t *testing.T) {
		const host = "127.0.0.1"

		_, err := ValidateHost(t.Context(), host)

		if !errors.Is(err, ErrUnsafeHost) {
			t.Errorf("ValidateHost(%q) error = %v, want %v", host, err, ErrUnsafeHost)
		}
	})

	t.Run("unmaps_ipv4", func(t *testing.T) {
		const host = "::ffff:8.8.8.8"

		got, err := ValidateHost(t.Context(), host)
		if err != nil {
			t.Fatalf("ValidateHost(%q) error = %v, want nil", host, err)
		}

		want := netip.MustParseAddr("8.8.8.8")

		if gotLen, wantLen := len(got), 1; gotLen != wantLen {
			t.Fatalf("len(ValidateHost(%q)) = %d, want %d", host, gotLen, wantLen)
		}

		if got[0] != want {
			t.Errorf("ValidateHost(%q)[0] = %v, want %v", host, got[0], want)
		}
	})
}

func TestValidateHostWithLookup(t *testing.T) {
	publicIPv4 := netip.MustParseAddr("8.8.8.8")
	publicIPv6 := netip.MustParseAddr("2606:4700:4700::1111")
	mappedIPv4 := netip.MustParseAddr("::ffff:8.8.8.8")
	unsafeIPv4 := netip.MustParseAddr("127.0.0.1")

	lookupErr := errors.New("lookup failed")

	tests := []struct {
		name      string
		resolved  []netip.Addr
		lookupErr error
		want      []netip.Addr
		wantErr   error
	}{
		{
			name: "public_addresses",
			resolved: []netip.Addr{
				publicIPv4,
				publicIPv6,
			},
			want: []netip.Addr{
				publicIPv4,
				publicIPv6,
			},
		},
		{
			name: "unmaps_resolved_ipv4",
			resolved: []netip.Addr{
				mappedIPv4,
			},
			want: []netip.Addr{
				publicIPv4,
			},
		},
		{
			name: "contains_unsafe_address",
			resolved: []netip.Addr{
				publicIPv4,
				unsafeIPv4,
			},
			wantErr: ErrUnsafeHost,
		},
		{
			name:    "no_addresses",
			wantErr: ErrNoResolvedAddresses,
		},
		{
			name:      "resolver_error",
			lookupErr: lookupErr,
			wantErr:   lookupErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const host = "example.com"

			lookup := func(context.Context, string, string) ([]netip.Addr, error) {
				return append([]netip.Addr(nil), tt.resolved...), tt.lookupErr
			}

			got, err := validateHost(t.Context(), host, lookup)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("validateHost(%q) error = %v, want %v", host, err, tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("validateHost(%q) error = %v, want nil", host, err)
			}

			if !slices.Equal(got, tt.want) {
				t.Errorf("validateHost(%q) = %v, want %v", host, got, tt.want)
			}
		})
	}
}
