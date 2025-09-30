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

func TestGenerateMACs(t *testing.T) {
	tests := []struct {
		name     string
		startMAC string
		count    int
		stepMAC  string
		want     []string
	}{
		{
			name:     "valid MAC sequence",
			startMAC: "00:00:00:00:00:AA",
			count:    3,
			stepMAC:  "00:00:00:00:00:01",
			want:     []string{"00:00:00:00:00:aa", "00:00:00:00:00:ab", "00:00:00:00:00:ac"},
		},
		{
			name:     "invalid base MAC",
			startMAC: "invalid",
			count:    3,
			stepMAC:  "00:00:00:00:00:01",
			want:     []string{}, // invalid MAC → empty list
		},
		{
			name:     "invalid step MAC",
			startMAC: "00:00:00:00:00:AA",
			count:    3,
			stepMAC:  "invalid",
			want:     []string{}, // invalid MAC → empty list
		},
		{
			name:     "negative count",
			startMAC: "00:00:00:00:00:AA",
			count:    -1,
			stepMAC:  "00:00:00:00:00:01",
			want:     []string{}, // negative count → empty list
		},
		{
			name:     "zero count",
			startMAC: "00:00:00:00:00:AA",
			count:    0,
			stepMAC:  "00:00:00:00:00:01",
			want:     []string{}, // zero count → empty list
		},
		{
			name:     "overflow MAC",
			startMAC: "ff:ff:ff:ff:ff:fe",
			count:    3,
			stepMAC:  "00:00:00:00:00:01",
			want:     []string{}, // overflow → empty list
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateMACs(tt.startMAC, tt.count, tt.stepMAC)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("GenerateMACs() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGenerateIPv6s(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		n       int
		want    []string
		wantErr bool
	}{
		{
			name: "Generate single IPv6",
			ip:   "2001:db8::1",
			n:    1,
			want: []string{"2001:db8::1"},
		},
		{
			name: "Generate consecutive IPv6s",
			ip:   "2001:db8::1",
			n:    3,
			want: []string{
				"2001:db8::1",
				"2001:db8::2",
				"2001:db8::3",
			},
		},
		{
			name: "Increment across boundary",
			ip:   "2001:db8::ff",
			n:    2,
			want: []string{
				"2001:db8::ff",
				"2001:db8::100",
			},
		},
		{
			name: "Zero count",
			ip:   "2001:db8::abcd",
			n:    0,
			want: []string{},
		},
		{
			name:    "Invalid IPv6 address",
			ip:      "invalid",
			n:       5,
			want:    []string{},
			wantErr: true,
		},
		{
			name:    "IPv4 address given",
			ip:      "192.168.1.1",
			n:       1,
			want:    []string{},
			wantErr: true,
		},
		{
			name: "Overflow IPv6 space",
			ip:   "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
			n:    2,
			want: []string{
				"ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
				"::", // wrap-around
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			got, err := GenerateIPv6s(ip, tt.n)

			if (err != nil) != tt.wantErr {
				t.Fatalf("GenerateIPv6s() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if diff := cmp.Diff(tt.want, got); diff != "" {
					t.Errorf("GenerateIPv6s() mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}
