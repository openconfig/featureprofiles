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

// GenerateIPv6Prefix takes a base IPv6 string and an integer offset, adds the offset to the base address, and returns the resulting IPv6 net.IP Caller can decide whether to append a CIDR mask or format it as needed.
func GenerateIPv6Prefix(baseIPv6 string, offset int64) net.IP {
	baseIP := net.ParseIP(baseIPv6).To16()
	if baseIP == nil {
		return nil
	}

	// 2^128 modulus
	pmax := new(big.Int).Lsh(big.NewInt(1), 128)

	baseInt := new(big.Int).SetBytes(baseIP)
	ipInt := new(big.Int).Add(baseInt, big.NewInt(offset))

	// wrap around
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

// incrementMAC increments the MAC address by the given step.
func incrementMAC(mac net.HardwareAddr, step int) {
	for i := len(mac) - 1; i >= 0 && step > 0; i-- {
		sum := int(mac[i]) + step
		mac[i] = byte(sum % 256)
		step = sum / 256
	}
}

func macToInt(mac net.HardwareAddr) int {
	result := 0
	for _, b := range mac {
		result = result<<8 + int(b)
	}
	return result
}

// GenerateMACs returns a slice of MAC address strings.
// Returns generated MAC addresses or an empty slice on parse errors.
func GenerateMACs(mac string, count int, stepMACStr string) []string {
	baseMAC, _ := net.ParseMAC(mac)
	stepMAC, _ := net.ParseMAC(stepMACStr)
	step := macToInt(stepMAC)

	macs := make([]string, count)
	current := make(net.HardwareAddr, len(baseMAC))
	copy(current, baseMAC)

	for i := range count {
		macs[i] = current.String()
		incrementMAC(current, step)
	}
	return macs

}
