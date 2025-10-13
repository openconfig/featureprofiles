// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package iputil provides utilities for IPv4/IPv6 related utils
package iputil

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"net"
)

// GenerateIPs creates list of n IPs using ipBlock
func GenerateIPs(ipBlock string, n int) []string {
	var entries []string
	_, netCIDR, err := net.ParseCIDR(ipBlock)
	if err != nil {
		return entries
	}
	netMask := binary.BigEndian.Uint32(netCIDR.Mask)
	firstIP := binary.BigEndian.Uint32(netCIDR.IP)
	lastIP := (firstIP & netMask) | (netMask ^ 0xffffffff)

	for i := firstIP; i <= lastIP && n > 0; i++ {
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, i)
		entries = append(entries, fmt.Sprint(ip))
		n--
	}

	return entries
}

// GenerateIPsWithStep creates a list of IPv4 addresses.
// Returns a slice of IPv4 address strings or an error if inputs are invalid.
func GenerateIPsWithStep(startIP string, count int, stepIP string) ([]string, error) {
	if count < 0 {
		return nil, fmt.Errorf("negative count")
	}
	if count == 0 {
		return []string{}, nil
	}

	ip := net.ParseIP(startIP).To4()
	if ip == nil {
		return nil, fmt.Errorf("invalid startIP")
	}
	step := net.ParseIP(stepIP).To4()
	if step == nil {
		return nil, fmt.Errorf("invalid stepIP")
	}

	start := binary.BigEndian.Uint32(ip)
	stepVal := binary.BigEndian.Uint32(step)

	// --- New overflow checks ---
	if stepVal == 0 {
		return nil, fmt.Errorf("invalid stepIP: step is zero")
	}
	// Step overflow check (first increment already too large)
	if start+stepVal < start {
		return nil, fmt.Errorf("step causes overflow")
	}
	// Count overflow check (final increment too large)
	if uint64(start)+uint64(stepVal)*uint64(count-1) > math.MaxUint32 {
		return nil, fmt.Errorf("count causes overflow")
	}

	out := make([]string, 0, count)
	for i := 0; i < count; i++ {
		val := start + uint32(i)*stepVal
		buf := make(net.IP, 4)
		binary.BigEndian.PutUint32(buf, val)
		out = append(out, buf.String())
	}
	return out, nil
}

func ipToBigInt(ip net.IP) *big.Int {
	ip = ip.To16()
	return big.NewInt(0).SetBytes(ip)
}

func bigIntToIP(ipInt *big.Int) net.IP {
	ipBytes := ipInt.Bytes()
	// Ensure the slice is 16 bytes long
	if len(ipBytes) < 16 {
		padded := make([]byte, 16)
		copy(padded[16-len(ipBytes):], ipBytes)
		ipBytes = padded
	}
	return net.IP(ipBytes)
}

// GenerateIPv6sWithStep creates a list of IPv6 addresses.
// Returns a slice of IPv6 address strings or an error if inputs are invalid.
func GenerateIPv6sWithStep(startIP string, count int, stepIP string) ([]string, error) {
	if count < 0 {
		return nil, fmt.Errorf("negative count")
	}
	if count == 0 {
		return []string{}, nil
	}

	ip := net.ParseIP(startIP).To16()
	if ip == nil || ip.To4() != nil {
		return nil, fmt.Errorf("invalid start IPv6")
	}

	step := net.ParseIP(stepIP).To16()
	if step == nil || step.To4() != nil {
		return nil, fmt.Errorf("invalid step IPv6")
	}

	ipInt := ipToBigInt(ip)
	stepInt := ipToBigInt(step)

	if stepInt.Sign() == 0 {
		return nil, fmt.Errorf("invalid step IPv6: step is zero")
	}

	maxIPv6 := new(big.Int).Lsh(big.NewInt(1), 128) // 2^128

	// --- Overflow check ---
	lastIPInt := new(big.Int).Add(ipInt, new(big.Int).Mul(stepInt, big.NewInt(int64(count-1))))
	if lastIPInt.Cmp(maxIPv6) >= 0 {
		return nil, fmt.Errorf("overflow IPv6")
	}

	// Generate sequence
	ips := make([]string, 0, count)
	for i := 0; i < count; i++ {
		newIPInt := new(big.Int).Add(ipInt, new(big.Int).Mul(stepInt, big.NewInt(int64(i))))
		newIP := bigIntToIP(newIPInt)
		ips = append(ips, newIP.String())
	}

	return ips, nil
}

// GenerateMACs returns a slice of MAC address strings.
// Returns generated MAC addresses or an empty slice on parse errors.
func GenerateMACs(startMAC string, count int, stepMACStr string) []string {
	if count < 0 {
		return []string{} // negative count → return empty
	}
	if count == 0 {
		return []string{}
	}

	baseMAC, err := net.ParseMAC(startMAC)
	if err != nil || len(baseMAC) != 6 {
		return []string{} // invalid base MAC
	}
	stepMAC, err := net.ParseMAC(stepMACStr)
	if err != nil || len(stepMAC) != 6 {
		return []string{} // invalid step MAC
	}

	baseInt := new(big.Int).SetBytes(baseMAC)
	stepInt := new(big.Int).SetBytes(stepMAC)

	// Maximum MAC value = 2^48 - 1
	maxMac := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 48), big.NewInt(1))

	// Check final value does not overflow: base + step*(count-1) <= maxMac
	mul := new(big.Int).Mul(stepInt, big.NewInt(int64(count-1)))
	final := new(big.Int).Add(baseInt, mul)
	if final.Cmp(maxMac) > 0 {
		return []string{} // overflow → return empty
	}

	// Generate sequence
	out := make([]string, 0, count)
	for i := 0; i < count; i++ {
		cur := new(big.Int).Add(baseInt, new(big.Int).Mul(stepInt, big.NewInt(int64(i))))
		buf := cur.FillBytes(make([]byte, 6)) // 6 bytes for MAC
		hw := net.HardwareAddr(buf)
		out = append(out, hw.String()) // canonical lower-case hex with colons
	}

	return out
}

// GenerateIPv6WithOffset increments the given IPv6 address by the specified offset. Returns nil if input is not a valid IPv6 address.
func GenerateIPv6WithOffset(baseIP net.IP, offset int64) net.IP {
	if baseIP == nil {
		return nil
	}

	// Handle negative offsets
	if offset < 0 {
		// Convert to equivalent positive offset modulo 2^128
		max := new(big.Int).Lsh(big.NewInt(1), 128)
		negOffset := new(big.Int).Mod(big.NewInt(offset), max)
		ipInt := ipToBigInt(baseIP.To16())
		result := new(big.Int).Add(ipInt, negOffset)
		result.Mod(result, max)
		return bigIntToIP(result)
	}

	// Use IncrementIPv6 for positive offsets
	ip, err := IncrementIPv6(baseIP, uint64(offset))
	if err != nil {
		return nil
	}
	return ip
}

// IncrementMAC increments the given MAC address by `i` and returns the result.
// This is just a convenience wrapper around GenerateMACs.
func IncrementMAC(startMAC string, i int) (string, error) {
	macs := GenerateMACs(startMAC, i, "00:00:00:00:00:01")
	if len(macs) == 0 {
		return "", fmt.Errorf("failed to generate MAC address")
	}
	return macs[0], nil
}

// IncrementIPv4 increments the given IPv4 address by n. Returns the incremented IP, or an error if the input is invalid.
func IncrementIPv4(ip net.IP, n uint32) (net.IP, error) {
	if ip == nil {
		return nil, fmt.Errorf("IP is nil")
	}

	ip4 := ip.To4()
	if ip4 == nil {
		return nil, fmt.Errorf("invalid IPv4 address: %v", ip)
	}

	ipInt := ipv4ToInt(ip4)
	if ipInt == 0 {
			return nil, fmt.Errorf("base IP %q is invalid", ip4.String())
	}
	offset := ipInt + n
	
	if offset < ipInt {
		return nil, fmt.Errorf("base IP %q plus increment %d overflowed IPv4 space", ip4.String(), n)
	}
	
	return intToIPv4(offset), nil
}

// ipv4ToInt converts a net.IP (expected to be IPv4) to a uint32.
func ipv4ToInt(ipv4 net.IP) uint32 {
	ipv4Bytes := ipv4.To4()
	if ipv4Bytes == nil {
		return 0
	}
	return binary.BigEndian.Uint32(ipv4Bytes)
}

// intToIPv4 converts a net.IP (expected to be IPv4) to a uint32.
func intToIPv4(n uint32) net.IP {
	// Create a 4-byte slice
	b := make([]byte, 4)

	// Use BigEndian to put the uint32 into the byte slice
	binary.BigEndian.PutUint32(b, n)

	// Convert the byte slice to net.IP
	return net.IP(b)
}

// IncrementIPv6 increments the given IPv6 address by n steps. Returns an error if the input IP is not a valid IPv6 address, if it is IPv4, or if the increment cannot be performed.
func IncrementIPv6(ip net.IP, n uint64) (net.IP, error) {
	if ip == nil {
		return nil, fmt.Errorf("IP is nil")
	}

	// Reject IPv4 or IPv4-mapped IPv6
	if ip.To4() != nil {
		return nil, fmt.Errorf("invalid IPv6 address (IPv4 detected): %s", ip.String())
	}

	ip6 := ip.To16()
	if ip6 == nil {
		return nil, fmt.Errorf("invalid IPv6 address: %s", ip.String())
	}

	ipInt := ipToBigInt(ip6)
	increment := new(big.Int).SetUint64(n)
	result := new(big.Int).Add(ipInt, increment)

	// 2^128 is the IPv6 address space limit
	max := new(big.Int).Lsh(big.NewInt(1), 128)
	result.Mod(result, max)

	return bigIntToIP(result), nil
}
