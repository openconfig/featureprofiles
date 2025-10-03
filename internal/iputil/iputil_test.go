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
		{
			name:    "IPv4 input should return nil",
			baseIP:  "192.168.1.1",
			offset:  5,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateIPv6WithOffset(net.ParseIP(tt.baseIP), tt.offset)

			if tt.wantNil {
				if got != nil {
					t.Errorf("GenerateIPv6WithOffset(%q, %d) = %v, want nil", tt.baseIP, tt.offset, got)
				}
				return
			}

			if diff := cmp.Diff(tt.want, got, cmp.Comparer(func(x, y net.IP) bool {
				return x.Equal(y)
			})); diff != "" {
				t.Errorf("GenerateIPv6WithOffset() mismatch (-want +got):\n%s", diff)
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

func TestIncrementIPv4(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		n       int
		want    string
		wantErr bool
	}{
		{
			name:    "Simple increment",
			ip:      "10.0.0.1",
			n:       1,
			want:    "10.0.0.2",
			wantErr: false,
		},
		{
			name:    "Increment across byte boundary",
			ip:      "10.0.0.255",
			n:       1,
			want:    "10.0.1.0",
			wantErr: false,
		},
		{
			name:    "Large increment within subnet",
			ip:      "192.168.1.0",
			n:       300,
			want:    "192.168.2.44",
			wantErr: false,
		},
		{
			name:    "Overflow IPv4 space",
			ip:      "255.255.255.255",
			n:       1,
			want:    "",
			wantErr: true,
		},
		{
			name:    "Zero increment",
			ip:      "10.0.0.1",
			n:       0,
			want:    "10.0.0.1",
			wantErr: false,
		},
		{
			name:    "Invalid IPv4 address",
			ip:      "invalid",
			n:       5,
			want:    "",
			wantErr: true,
		},
		{
			name:    "Negative increment",
			ip:      "10.0.0.1",
			n:       -1,
			want:    "",
			wantErr: true,
		},
		{
			name:    "Max valid increment",
			ip:      "0.0.0.0",
			n:       4294967295, // 2^32 - 1
			want:    "255.255.255.255",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			got, err := IncrementIPv4(ip, tt.n)
			if (err != nil) != tt.wantErr {
				t.Errorf("IncrementIPv4() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && got.String() != tt.want {
				t.Errorf("IncrementIPv4() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIncrementIPv6(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		n       int
		want    string
		wantErr bool
	}{
		{
			name:    "Simple increment",
			ip:      "2001:db8::1",
			n:       1,
			want:    "2001:db8::2",
			wantErr: false,
		},
		{
			name:    "Increment across segment boundary",
			ip:      "2001:db8::ff",
			n:       1,
			want:    "2001:db8::100",
			wantErr: false,
		},
		{
			name:    "Large increment",
			ip:      "2001:db8::",
			n:       65536,
			want:    "2001:db8::1:0",
			wantErr: false,
		},
		{
			name:    "Zero increment",
			ip:      "2001:db8::abcd",
			n:       0,
			want:    "2001:db8::abcd",
			wantErr: false,
		},
		{
			name:    "Invalid IPv6 address",
			ip:      "invalid",
			n:       5,
			want:    "",
			wantErr: true,
		},
		{
			name:    "IPv4 address given",
			ip:      "192.168.1.1",
			n:       1,
			want:    "",
			wantErr: true,
		},
		{
			name:    "Negative increment",
			ip:      "2001:db8::1",
			n:       -1,
			want:    "",
			wantErr: true,
		},
		{
			name:    "Max increment without overflow",
			ip:      "::",
			n:       1<<16 - 1, // just before big wrap
			want:    "::ffff",
			wantErr: false,
		},
		{
			name:    "Overflow IPv6 space",
			ip:      "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
			n:       1,
			want:    "::",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			got, err := IncrementIPv6(ip, tt.n)
			if (err != nil) != tt.wantErr {
				t.Errorf("IncrementIPv6() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && got.String() != tt.want {
				t.Errorf("IncrementIPv6() = %v, want %v", got, tt.want)
			}
		})
	}
}
