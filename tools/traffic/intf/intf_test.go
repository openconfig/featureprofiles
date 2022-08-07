package intf

import (
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func mustParseCIDR(t *testing.T, pfx string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(pfx)
	if err != nil {
		t.Fatalf("cannot parse CIDR %s, %v", pfx, err)
	}
	return n
}

func TestIPs(t *testing.T) {
	tests := []struct {
		name string
		in   *net.IPNet
		want []net.IP
	}{{
		name: "/31",
		in:   mustParseCIDR(t, "192.0.2.0/31"),
		want: []net.IP{
			net.ParseIP("192.0.2.0"),
			net.ParseIP("192.0.2.1"),
		},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ips(tt.in)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Fatalf("did not get expected, diff(-got,+want):\n%s", diff)
			}
		})
	}
}
