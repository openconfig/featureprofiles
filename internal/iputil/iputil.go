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

// NextIPMultiSteps returns the next IPv4 or IPv6 address after incrementing the last octet by count times.
func NextIPMultiSteps(ip net.IP, count int) net.IP {
	nextIPAddress := ip
	for i := 0; i < count; i++ {
		nextIPAddress = func(ip net.IP) net.IP {
			next := make(net.IP, len(ip))
			copy(next, ip)
			for i := len(next) - 1; i >= 0; i-- {
				next[i]++
				if next[i] > 0 {
					break
				}
			}
			return next
		}(nextIPAddress)
	}
	return nextIPAddress
}

// GenerateIPv6s generates a list of consecutive IPv6 addresses starting from a given base IP.
func GenerateIPv6s(baseIP net.IP, n int) ([]string, error) {
	entries := make([]string, 0, n)

	if baseIP == nil {
		return nil, fmt.Errorf("invalid IPv6 address")
	}
	ip := baseIP.To16()
	if ip == nil || baseIP.To4() != nil {
		return nil, fmt.Errorf("not a valid IPv6 address")
	}

	baseInt := new(big.Int).SetBytes(ip)
	pmax := new(big.Int).Lsh(big.NewInt(1), 128) // 2^128

	for i := 0; i < n; i++ {
		nextInt := new(big.Int).Add(baseInt, big.NewInt(int64(i)))
		nextInt.Mod(nextInt, pmax) // wrap around if overflow
		ipBytes := nextInt.FillBytes(make([]byte, 16))
		entries = append(entries, net.IP(ipBytes).String())
	}

	return entries, nil
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
