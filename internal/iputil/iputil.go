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

func ipv4ToInt(ip net.IP) uint32 {
	return uint32(ip[0])<<24 + uint32(ip[1])<<16 + uint32(ip[2])<<8 + uint32(ip[3])
}

func intToIPv4(n uint32) net.IP {
	return net.IPv4(
		byte(n>>24),
		byte((n>>16)&0xFF),
		byte((n>>8)&0xFF),
		byte(n&0xFF),
	)
}

// GenerateIPv4sWithStep returns a list of IPv4 addresses starting from startIP,
// generating count addresses by incrementing each step by stepIP. The function
// validates that startIP and stepIP are valid IPv4 addresses, ensures count is
// non-negative, and checks that the generated sequence does not exceed the
// IPv4 address space.
func GenerateIPv4sWithStep(startIP string, count int, stepIP string) ([]string, error) {
	if count < 0 {
		return nil, fmt.Errorf("count cannot be negative")
	}
	if count == 0 {
		return []string{}, nil
	}
	ip := net.ParseIP(startIP).To4()
	if ip == nil {
		return nil, fmt.Errorf("invalid start IPv4 address")
	}

	step := net.ParseIP(stepIP).To4()
	if step == nil {
		return nil, fmt.Errorf("invalid step IPv4 address")
	}

	ipInt := ipv4ToInt(ip)
	stepInt := ipv4ToInt(step)
	// Validate overflow before generating
	lastIP := uint64(ipInt) + uint64(count-1)*uint64(stepInt)
	if lastIP > math.MaxUint32 {
		return nil, fmt.Errorf("count and step exceed IPv4 address space")
	}

	ips := make([]string, count)
	for i := int(0); i < count; i++ {
		next := ipInt + uint32(i)*stepInt
		if next > math.MaxUint32 {
			return nil, fmt.Errorf("step caused IPv4 overflow at index %d", i)
		}
		ips[i] = intToIPv4(next).String()
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

// GenerateIPv6sWithStep returns a list of IPv6 addresses starting from startIP,
// generating count addresses by incrementing each step by stepIP. The function
// validates that startIP and stepIP are valid IPv6 addresses, ensures count is
// non-negative, and checks that the generated sequence does not exceed the
// IPv6 address space.
func GenerateIPv6sWithStep(startIP string, count int, stepIP string) ([]string, error) {
	if count < 0 {
		return nil, fmt.Errorf("count cannot be negative")
	}
	if count == 0 {
		return []string{}, nil
	}
	ip := net.ParseIP(startIP).To16()
	if ip == nil || ip.To4() != nil {
		return nil, fmt.Errorf("invalid start IPv6 address:  %q", startIP)
	}

	step := net.ParseIP(stepIP).To16()
	if step == nil || step.To4() != nil {
		return nil, fmt.Errorf("invalid step IPv6 address: %q", stepIP)
	}

	ipInt := ipToBigInt(ip)
	stepInt := ipToBigInt(step)
	// Max IPv6 value = 2^128 - 1
	maxIPv6 := big.NewInt(0)
	maxIPv6.Exp(big.NewInt(2), big.NewInt(128), nil)
	maxIPv6.Sub(maxIPv6, big.NewInt(1))

	ips := make([]string, count)
	for i := int(0); i < count; i++ {
		offset := big.NewInt(0).Mul(stepInt, big.NewInt(int64(i)))
		newIPInt := big.NewInt(0).Add(ipInt, offset)

		// Overflow check
		if newIPInt.Cmp(maxIPv6) > 0 {
			return nil, fmt.Errorf("IPv6 address overflow at index %d", i)
		}

		ips[i] = bigIntToIP(newIPInt).String()
	}

	return ips, nil
}

func macToInt(mac net.HardwareAddr) uint64 {
	result := uint64(0)
	for _, b := range mac {
		result = result<<8 + uint64(b)
	}
	return result
}

// GenerateMACs returns a list of MAC addresses starting from mac, generating
// count addresses by incrementing each step by stepMACStr. The function
// validates that both the base and step MAC addresses are valid 48-bit MACs,
// ensures count is non-negative, and checks that the generated sequence does
// not exceed the MAC address space.
func GenerateMACs(mac string, count int, stepMACStr string) ([]string, error) {
	if count < 0 {
		return nil, fmt.Errorf("count cannot be negative")
	}
	if count == 0 {
		return []string{}, nil
	}
	baseMAC, err := net.ParseMAC(mac)
	if err != nil || len(baseMAC) != 6 {
		return nil, fmt.Errorf("invalid base MAC address: %q", mac)
	}
	stepMAC, err := net.ParseMAC(stepMACStr)
	if err != nil || len(stepMAC) != 6 {
		return nil, fmt.Errorf("invalid step MAC address: %q", stepMACStr)
	}
	step := macToInt(stepMAC)

	baseInt := macToInt(baseMAC)

	// Max MAC value = FF:FF:FF:FF:FF:FF (48 bits)
	maxMAC := uint64(1<<48) - 1

	macs := make([]string, count)
	for i := int(0); i < count; i++ {
		newMACInt := baseInt + uint64(i)*step
		if newMACInt > maxMAC {
			return nil, fmt.Errorf("MAC address overflow at index %d", i)
		}
		macs[i] = intToMAC(newMACInt).String()
	}
	return macs, nil

}

func intToMAC(val uint64) net.HardwareAddr {
	mac := make(net.HardwareAddr, 6)
	for i := 5; i >= 0; i-- {
		mac[i] = byte(val & 0xFF)
		val >>= 8
	}
	return mac
}
