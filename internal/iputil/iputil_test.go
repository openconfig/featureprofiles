package iputil

import (
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGenerateIPs(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		count  int
		want   []string
	}{{
		name:   "IPv4/24",
		prefix: "192.168.0.0/24",
		count:  3,
		want:   []string{"192.168.0.0", "192.168.0.1", "192.168.0.2"},
	}, {
		name:   "IPv4/31",
		prefix: "192.168.0.0/31",
		count:  3,
		want:   []string{"192.168.0.0", "192.168.0.1"},
	}, {
		name:   "Invalid prefix",
		prefix: "192.168.0.0/24/24",
		count:  3,
		want:   nil,
	}, {
		name:   "Invalid count",
		prefix: "192.168.0.0/24",
		count:  0,
		want:   nil,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateIPs(tt.prefix, tt.count)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("GenerateIPs() returned diff (-want +got):\n%s", diff)
			}
		})
	}
}

// TestGenerateIPv6Prefix covers valid and invalid cases for IPv6 prefix generation.
func TestGenerateIPv6Prefix(t *testing.T) {
	tests := []struct {
		name    string
		baseIP  string
		offset  int64
		want    net.IP
		wantNil bool
	}{
		{
			name:   "valid prefix with offset",
			baseIP: "2001:db8::",
			offset: 5,
			want:   net.ParseIP("2001:db8::5"),
		},
		{
			name:   "valid prefix with negative offset",
			baseIP: "2001:db8::5",
			offset: -5,
			want:   net.ParseIP("2001:db8::"),
		},
		{
			name:   "valid prefix with large offset",
			baseIP: "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
			offset: 1,
			// overflows, so wraps to zero
			want: net.ParseIP("::"),
		},
		{
			name:    "empty prefix",
			baseIP:  "",
			offset:  1,
			wantNil: true,
		},
		{
			name:    "invalid prefix",
			baseIP:  "invalid-ip",
			offset:  1,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateIPv6Prefix(tt.baseIP, tt.offset)

			if tt.wantNil {
				if got != nil {
					t.Errorf("GenerateIPv6Prefix(%q, %d) = %v, want nil", tt.baseIP, tt.offset, got)
				}
				return
			}

			if diff := cmp.Diff(tt.want, got, cmp.Comparer(func(x, y net.IP) bool {
				return x.Equal(y)
			})); diff != "" {
				t.Errorf("GenerateIPv6Prefix() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
