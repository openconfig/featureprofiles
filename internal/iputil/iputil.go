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
	"net"
	"net/netip"
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

// generateIPv6Entries creates IPv6 Entries given the totalCount and starting prefix
func GenerateIPv6(startIP string, count uint64) []string {

	_, netCIDR, _ := net.ParseCIDR(startIP)
	netMask := binary.BigEndian.Uint64(netCIDR.Mask)
	maskSize, _ := netCIDR.Mask.Size()
	firstIP := binary.BigEndian.Uint64(netCIDR.IP)
	lastIP := (firstIP & netMask) | (netMask ^ 0xffffffff)
	entries := []string{}

	for i := firstIP; i <= lastIP; i++ {
		ipv6 := make(net.IP, 16)
		binary.BigEndian.PutUint64(ipv6, i)
		// make last byte non-zero
		p, _ := netip.ParsePrefix(fmt.Sprintf("%v/%d", ipv6, maskSize))
		entries = append(entries, p.Addr().Next().String())
		if uint64(len(entries)) >= count {
			break
		}
	}
	return entries
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
