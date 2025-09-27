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

// GenerateIPv6WithOffset increments the given IPv6 address by the specified offset. Returns nil if input is not a valid IPv6 address.
func GenerateIPv6WithOffset(baseIP net.IP, offset int64) net.IP {
	if baseIP == nil {
		return nil
	}
	// Reject IPv4 (including IPv4-mapped IPv6)
	if baseIP.To4() != nil {
		return nil
	}
	ip16 := baseIP.To16()
	if ip16 == nil {
		return nil
	}
	// 2^128 modulus
	pmax := new(big.Int).Lsh(big.NewInt(1), 128)

	baseInt := new(big.Int).SetBytes(baseIP.To16())
	ipInt := new(big.Int).Add(baseInt, big.NewInt(offset))

	// This will wrap around the IPv6 space if the offset pushes beyond the maximum address.
	ipInt.Mod(ipInt, pmax)

	ipBytes := ipInt.FillBytes(make([]byte, 16))
	return net.IP(ipBytes)
}

func ipToInt(ip net.IP) uint32 {
	return uint32(ip[0])<<24 + uint32(ip[1])<<16 + uint32(ip[2])<<8 + uint32(ip[3])
}

func intToIP(n uint32) net.IP {
	return net.IPv4(
		byte(n>>24),
		byte((n>>16)&0xFF),
		byte((n>>8)&0xFF),
		byte(n&0xFF),
	)
}

// GenerateIPsWithStep creates a list of IPv4 addresses.
// Returns a slice of IPv4 address strings or an error if inputs are invalid.
func GenerateIPsWithStep(startIP string, count int, stepIP string) ([]string, error) {
	ip := net.ParseIP(startIP).To4()
	if ip == nil {
		return nil, fmt.Errorf("invalid start IPv4 address")
	}

	step := net.ParseIP(stepIP).To4()
	if step == nil {
		return nil, fmt.Errorf("invalid step IPv4 address")
	}

	var ips []string
	ipInt := ipToInt(ip)
	stepInt := ipToInt(step)

	for i := range count {
		newIP := intToIP(ipInt + uint32(i)*stepInt)
		ips = append(ips, newIP.String())
	}

	return ips, nil
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
	ip := net.ParseIP(startIP).To16()
	if ip == nil || ip.To4() != nil {
		return nil, fmt.Errorf("invalid start IPv6 address")
	}

	step := net.ParseIP(stepIP).To16()
	if step == nil || step.To4() != nil {
		return nil, fmt.Errorf("invalid step IPv6 address")
	}

	ipInt := ipToBigInt(ip)
	stepInt := ipToBigInt(step)

	var ips []string
	for i := 0; i < count; i++ {
		newIPInt := big.NewInt(0).Add(ipInt, big.NewInt(0).Mul(stepInt, big.NewInt(int64(i))))
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

// GenerateIPv6s generates a list of consecutive IPv6 addresses starting from a given base IP.
func GenerateIPv6s(baseIP net.IP, n int) []string {
	var entries []string

	ip := baseIP.To16()
	if ip == nil || baseIP.To4() != nil {
		return entries // not a valid IPv6 address
	}

	baseInt := new(big.Int).SetBytes(ip)

	for i := 0; i < n; i++ {
		nextInt := new(big.Int).Add(baseInt, big.NewInt(int64(i)))
		ipBytes := nextInt.FillBytes(make([]byte, 16))
		entries = append(entries, net.IP(ipBytes).String())
	}

	return entries
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
func IncrementIPv4(ip net.IP, n int) (net.IP, error) {
	if ip == nil {
		return nil, fmt.Errorf("IP is nil")
	}

	ip4 := ip.To4()
	if ip4 == nil {
		return nil, fmt.Errorf("invalid IPv4 address: %v", ip)
	}

	if n < 0 {
		return nil, fmt.Errorf("negative increment not supported: %d", n)
	}

	carry := n
	newIP := make(net.IP, len(ip4))
	copy(newIP, ip4)

	for j := len(newIP) - 1; j >= 0 && carry > 0; j-- {
		sum := int(newIP[j]) + carry
		newIP[j] = byte(sum & 0xFF) // lower 8 bits
		carry = sum >> 8            // carry-over to next byte
	}

	if carry > 0 {
		return nil, fmt.Errorf("increment overflowed IPv4 space")
	}

	return newIP, nil
}

// IncrementIPv6 increments the given IPv6 address by n steps. Returns an error if the input IP is not a valid IPv6 address, if it is IPv4, or if the increment cannot be performed.
func IncrementIPv6(ip net.IP, n int) (net.IP, error) {
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

	if n < 0 {
		return nil, fmt.Errorf("negative increment not supported: %d", n)
	}

	// Work on a copy so the caller’s IP isn’t modified
	res := make(net.IP, len(ip6))
	copy(res, ip6)

	carry := n
	for j := len(res) - 1; j >= 0 && carry > 0; j-- {
		sum := int(res[j]) + carry
		res[j] = byte(sum & 0xFF)
		carry = sum >> 8
	}

	// If carry > 0 after the most significant byte, wrap around to zero (no error, just wrap in 128-bit space).
	if carry > 0 {
		for j := range res {
			res[j] = 0
		}
	}

	return res, nil
}
