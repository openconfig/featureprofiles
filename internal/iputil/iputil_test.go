package iputil

import (
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/gnmi/errdiff"
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
		wantErr string
	}{
		{
			name:    "valid case",
			startIP: "192.168.0.1",
			count:   3,
			stepIP:  "0.0.0.1",
			want:    []string{"192.168.0.1", "192.168.0.2", "192.168.0.3"},
			wantErr: "",
		},
		{
			name:    "invalid startIP",
			startIP: "999.168.0.1",
			count:   3,
			stepIP:  "0.0.0.1",
			wantErr: "invalid startIP",
		},
		{
			name:    "invalid stepIP",
			startIP: "192.168.0.1",
			count:   3,
			stepIP:  "0.0.999.1",
			wantErr: "invalid stepIP",
		},
		{
			name:    "negative count",
			startIP: "192.168.0.1",
			count:   -5,
			stepIP:  "0.0.0.1",
			wantErr: "negative count",
		},
		{
			name:    "zero count",
			startIP: "192.168.0.1",
			count:   0,
			stepIP:  "0.0.0.1",
			want:    []string{},
			wantErr: "",
		},
		{
			name:    "count causes overflow",
			startIP: "255.255.255.250",
			count:   10,
			stepIP:  "0.0.0.1",
			wantErr: "count causes overflow",
		},
		{
			name:    "step causes overflow",
			startIP: "255.255.255.250",
			count:   2,
			stepIP:  "0.0.0.10",
			wantErr: "step causes overflow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateIPsWithStep(tt.startIP, tt.count, tt.stepIP)
			if diff := errdiff.Substring(err, tt.wantErr); diff != "" {
				t.Errorf("generateIPv4sWithStep() unexpected error (-want,+got): %s", diff)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("generateIPv4sWithStep() mismatch (-want +got):\n%s", diff)
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
		wantErr string
	}{
		{
			name:    "valid IPv6 sequence",
			startIP: "2001:db8::1",
			count:   3,
			stepIP:  "::1",
			want:    []string{"2001:db8::1", "2001:db8::2", "2001:db8::3"},
			wantErr: "",
		},
		{
			name:    "invalid start IPv6",
			startIP: "invalid",
			count:   3,
			stepIP:  "::1",
			wantErr: "invalid start IPv6",
		},
		{
			name:    "invalid step IPv6",
			startIP: "2001:db8::1",
			count:   3,
			stepIP:  "invalid",
			wantErr: "invalid step IPv6",
		},
		{
			name:    "negative count",
			startIP: "2001:db8::1",
			count:   -1,
			stepIP:  "::1",
			wantErr: "negative count",
		},
		{
			name:    "zero count",
			startIP: "2001:db8::1",
			count:   0,
			stepIP:  "::1",
			want:    []string{},
			wantErr: "",
		},
		{
			name:    "overflow IPv6",
			startIP: "ffff:ffff:ffff:ffff:ffff:ffff:ffff:fffe",
			count:   3,
			stepIP:  "::1",
			wantErr: "overflow IPv6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateIPv6sWithStep(tt.startIP, tt.count, tt.stepIP)
			if diff := errdiff.Substring(err, tt.wantErr); diff != "" {
				t.Errorf("generateIPv6sWithStep() unexpected error (-want,+got): %s", diff)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("generateIPv6sWithStep() mismatch (-want +got):\n%s", diff)
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

func Test_IPEqual(t *testing.T) {
	tests := []struct {
		name     string
		got      string
		want     string
		wantBool bool
	}{
		// IPv4 Cases
		{"Valid IPv4 Match", "192.168.1.1", "192.168.1.1", true},
		{"Valid IPv4 Mismatch", "192.168.1.1", "192.168.1.2", false},

		// IPv6 Cases (Semantic Equality)
		{"IPv6 Shorthand Match", "2001:db8::1", "2001:db8:0:0:0:0:0:1", true},
		{"IPv6 Case Insensitivity", "2001:DB8::1", "2001:db8::1", true},
		{"IPv6 Mismatch", "2001:db8::1", "2001:db8::2", false},

		// Fallback String Comparison (Parsing Fails)
		{"Hostname Fallback Match", "localhost", "localhost", true},
		{"Hostname Fallback Mismatch", "example.com", "google.com", false},
		{"Mixed IP and String", "127.0.0.1", "localhost", false},
		{"Empty Strings", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := IPEqual(tt.got, tt.want); result != tt.wantBool {
				t.Errorf("IPEqual(%q, %q) = %v, want %v", tt.got, tt.want, result, tt.wantBool)
			}
		})
	}
}

// generateIPv6Entries creates IPv6 Entries given the totalCount and starting prefix
func GenerateIPv6(startIP string, count uint64) ([]string, error) {
	if startIP == "" {
		return nil, fmt.Errorf("invalid IPv6 address")
	}

	_, netCIDR, _ := net.ParseCIDR(startIP)
	fmt.Println(netCIDR)

	if netCIDR == nil {
		return nil, fmt.Errorf("parsed CIDR is nil for input: %s", startIP)
	}

	// Ensure it's IPv6
	ipBytes := netCIDR.IP.To16()
	if ipBytes == nil || netCIDR.IP.To4() != nil {
		return nil, fmt.Errorf("IPv4 address given, expected IPv6: %s", startIP)
	}

	maskSize, bits := netCIDR.Mask.Size()
	if bits != 128 {
		return nil, fmt.Errorf("expected IPv6 mask, got %d bits", bits)
	}

	ip := new(big.Int).SetBytes(ipBytes)
	mask := new(big.Int).SetBytes(netCIDR.Mask)
	networkIP := new(big.Int).And(ip, mask)

	step := new(big.Int).Lsh(big.NewInt(1), uint(128-maskSize))
	hostOffset := big.NewInt(1)
	pmax := new(big.Int).Lsh(big.NewInt(1), 128)

	if count == 0 {
		count = 1
	}

	entries := []string{}
	for i := uint64(0); i < count; i++ {
		nextInt := new(big.Int).Mul(new(big.Int).SetUint64(i), step)
		nextInt.Add(nextInt, networkIP)
		nextInt.Add(nextInt, hostOffset)
		nextInt.Mod(nextInt, pmax)
		ipv6 := nextInt.FillBytes(make([]byte, 16))
		p, _ := netip.ParsePrefix(fmt.Sprintf("%v/%d", net.IP(ipv6), maskSize))
		entries = append(entries, p.Addr().String())
	}
	return entries, nil
}
