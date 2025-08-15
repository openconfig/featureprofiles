package iputil

import (
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

func TestGenerateIPv4sWithStep(t *testing.T) {
	tests := []struct {
		name    string
		startIP string
		count   int
		stepIP  string
		want    []string
		wantErr bool
	}{
		{
			name:    "valid case",
			startIP: "192.168.0.1",
			count:   3,
			stepIP:  "0.0.0.1",
			want:    []string{"192.168.0.1", "192.168.0.2", "192.168.0.3"},
			wantErr: false,
		},
		{
			name:    "invalid startIP",
			startIP: "999.168.0.1",
			count:   3,
			stepIP:  "0.0.0.1",
			wantErr: true,
		},
		{
			name:    "invalid stepIP",
			startIP: "192.168.0.1",
			count:   3,
			stepIP:  "0.0.999.1",
			wantErr: true,
		},
		{
			name:    "negative count",
			startIP: "192.168.0.1",
			count:   -5,
			stepIP:  "0.0.0.1",
			wantErr: true,
		},
		{
			name:    "zero count",
			startIP: "192.168.0.1",
			count:   0,
			stepIP:  "0.0.0.1",
			want:    []string{},
			wantErr: false,
		},
		{
			name:    "count causes overflow",
			startIP: "255.255.255.250",
			count:   10,
			stepIP:  "0.0.0.1",
			wantErr: true,
		},
		{
			name:    "step causes overflow",
			startIP: "255.255.255.250",
			count:   2,
			stepIP:  "0.0.0.10",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateIPv4sWithStep(tt.startIP, tt.count, tt.stepIP)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateIPv4sWithStep() error = %v, wantErr %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("GenerateIPv4sWithStep() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGenerateIPv6sWithStep(t *testing.T) {
	tests := []struct {
		name    string
		startIP string
		count   int
		stepIP  string
		want    []string
		wantErr bool
	}{
		{
			name:    "valid IPv6 sequence",
			startIP: "2001:db8::1",
			count:   3,
			stepIP:  "::1",
			want:    []string{"2001:db8::1", "2001:db8::2", "2001:db8::3"},
			wantErr: false,
		},
		{
			name:    "invalid start IPv6",
			startIP: "invalid",
			count:   3,
			stepIP:  "::1",
			wantErr: true,
		},
		{
			name:    "invalid step IPv6",
			startIP: "2001:db8::1",
			count:   3,
			stepIP:  "invalid",
			wantErr: true,
		},
		{
			name:    "negative count",
			startIP: "2001:db8::1",
			count:   -1,
			stepIP:  "::1",
			wantErr: true,
		},
		{
			name:    "zero count",
			startIP: "2001:db8::1",
			count:   0,
			stepIP:  "::1",
			want:    []string{},
			wantErr: false,
		},
		{
			name:    "overflow IPv6",
			startIP: "ffff:ffff:ffff:ffff:ffff:ffff:ffff:fffe",
			count:   3,
			stepIP:  "::1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateIPv6sWithStep(tt.startIP, tt.count, tt.stepIP)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateIPv6sWithStep() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !cmp.Equal(got, tt.want) {
				t.Errorf("GenerateIPv6sWithStep() mismatch (-want +got):\n%s", cmp.Diff(tt.want, got))
			}
		})
	}
}

func TestGenerateMACs(t *testing.T) {
	tests := []struct {
		name      string
		startMAC  string
		count     int
		stepMAC   string
		want      []string
		wantError bool
	}{
		{
			name:     "valid MAC sequence",
			startMAC: "00:00:00:00:00:AA",
			count:    3,
			stepMAC:  "00:00:00:00:00:01",
			want:     []string{"00:00:00:00:00:aa", "00:00:00:00:00:ab", "00:00:00:00:00:ac"},
		},
		{
			name:      "invalid base MAC",
			startMAC:  "invalid",
			count:     3,
			stepMAC:   "00:00:00:00:00:01",
			wantError: true,
		},
		{
			name:      "invalid step MAC",
			startMAC:  "00:00:00:00:00:AA",
			count:     3,
			stepMAC:   "invalid",
			wantError: true,
		},
		{
			name:      "negative count",
			startMAC:  "00:00:00:00:00:AA",
			count:     -1,
			stepMAC:   "00:00:00:00:00:01",
			wantError: true,
		},
		{
			name:     "zero count",
			startMAC: "00:00:00:00:00:AA",
			count:    0,
			stepMAC:  "00:00:00:00:00:01",
			want:     []string{},
		},
		{
			name:      "overflow MAC",
			startMAC:  "ff:ff:ff:ff:ff:fe",
			count:     3,
			stepMAC:   "00:00:00:00:00:01",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateMACs(tt.startMAC, tt.count, tt.stepMAC)
			if (err != nil) != tt.wantError {
				t.Errorf("GenerateMACs() error = %v, wantError %v", err, tt.wantError)
			}
			if !tt.wantError && !cmp.Equal(got, tt.want) {
				t.Errorf("GenerateMACs() mismatch (-want +got):\n%s", cmp.Diff(tt.want, got))
			}
		})
	}
}
