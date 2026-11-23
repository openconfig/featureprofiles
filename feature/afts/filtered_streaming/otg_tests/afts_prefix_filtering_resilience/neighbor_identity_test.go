// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0

// Package aftprefixfilteringresilience implements AFT-6.3:
// AFT Prefix Filtering Resilience.
package afts_prefix_filtering_resilience_test

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"testing"
)

// ciscoIPv6NeighborMatches requires the address, interface and MAC in the same
// native neighbor record. Wrapped rows are supported; an entry on another port
// cannot verify this peer, and conflicting MACs on the expected port fail.
func ciscoIPv6NeighborMatches(t *testing.T, output, intf string, address netip.Addr, wantMAC net.HardwareAddr) (bool, error) {
	t.Helper()
	interfaceNames := map[string]bool{intf: true, intf + ".0": true}
	// XR's native ND table abbreviates these interface types. Preserve the
	// complete port suffix so another port (for example /10 versus /100)
	// cannot verify this peer.
	for longName, shortPrefix := range map[string]string{"FourHundredGigE": "FH", "HundredGigE": "Hu", "TenGigE": "Te"} {
		if strings.HasPrefix(intf, longName) {
			shortName := shortPrefix + strings.TrimPrefix(intf, longName)
			interfaceNames[shortName] = true
			interfaceNames[shortName+".0"] = true
		}
	}
	var records [][]string
	// Only a partially printed neighbor may consume a continuation line.
	// In particular, [Mcast adjacency] is a separate record, not a neighbor's
	// continuation: its zero MAC must never be attributed to the peer above it.
	pending := -1
	complete := func(fields []string) bool {
		hasMAC, hasInterface := false, false
		for _, field := range fields {
			if mac, err := net.ParseMAC(field); err == nil && len(mac) == 6 {
				hasMAC = true
			}
			if strings.Contains(field, "/") && !strings.Contains(field, ":") {
				// Interface names have a port suffix; locations such as
				// 0/0/CPU0 do not start with a letter.
				hasInterface = hasInterface || (field[0] >= 'A' && field[0] <= 'Z') || (field[0] >= 'a' && field[0] <= 'z')
			}
		}
		return hasMAC && hasInterface
	}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			pending = -1
			continue
		}
		if ip, err := netip.ParseAddr(fields[0]); err == nil && ip.Is6() {
			records = append(records, fields)
			pending = len(records) - 1
		} else {
			// Wrapped XR rows begin with an age or a MAC. Reject headers,
			// multicast rows and unrelated CLI text as record boundaries.
			_, ageErr := strconv.ParseUint(fields[0], 10, 64)
			mac, macErr := net.ParseMAC(fields[0])
			continuation := ageErr == nil || fields[0] == "-" || (macErr == nil && len(mac) == 6) || interfaceNames[fields[0]]
			if pending < 0 || !continuation {
				pending = -1
				continue
			}
			records[pending] = append(records[pending], fields...)
		}
		if pending >= 0 && complete(records[pending]) {
			pending = -1
		}
	}
	matched := false
	for _, record := range records {
		ip, err := netip.ParseAddr(record[0])
		if err != nil || ip.WithZone("") != address {
			continue
		}
		correctInterface := false
		var macs []net.HardwareAddr
		for _, field := range record[1:] {
			if interfaceNames[field] {
				correctInterface = true
			}
			if mac, err := net.ParseMAC(field); err == nil && len(mac) == 6 {
				macs = append(macs, mac)
			}
		}
		if !correctInterface {
			continue
		}
		t.Logf("IPv6 neighbor validation selected record: address=%s interface=%s fields=%v", address, intf, record)
		if len(macs) == 0 {
			return false, fmt.Errorf("neighbor %s on %s has no resolved MAC: %v", address, intf, record)
		}
		for _, mac := range macs {
			if !strings.EqualFold(mac.String(), wantMAC.String()) {
				return false, fmt.Errorf("neighbor %s on %s has MAC %s, want %s", address, intf, mac, wantMAC)
			}
		}
		matched = true
	}
	if !matched {
		return false, fmt.Errorf("no native neighbor record for %s with MAC %s on %s", address, wantMAC, intf)
	}
	return true, nil
}

func TestCiscoIPv6NeighborMatches(t *testing.T) {
	const peer = "fe80::2ff:fe01:101"
	const row = peer + " 41 0200.0201.0101 REACH Te0/0/0/0/0 0/0/CPU0"
	const huRow = peer + " 32 0200.0201.0101 REACH Hu0/0/0/4/0 0/0/CPU0"
	mac, err := net.ParseMAC("02:00:02:01:01:01")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, output, intf string
		want               bool
	}{
		{name: "TenGigE native abbreviation", output: row, intf: "TenGigE0/0/0/0/0", want: true},
		{name: "full interface name", output: strings.ReplaceAll(row, "Te0/0/0/0/0", "TenGigE0/0/0/0/0"), intf: "TenGigE0/0/0/0/0", want: true},
		{name: "subinterface zero", output: strings.ReplaceAll(row, "Te0/0/0/0/0", "Te0/0/0/0/0.0"), intf: "TenGigE0/0/0/0/0", want: true},
		{name: "FourHundredGigE native abbreviation", output: strings.ReplaceAll(row, "Te0/0/0/0/0", "FH0/0/0/0"), intf: "FourHundredGigE0/0/0/0", want: true},
		{name: "HundredGigE native abbreviation", output: huRow, intf: "HundredGigE0/0/0/4/0", want: true},
		{name: "HundredGigE full interface name", output: strings.ReplaceAll(huRow, "Hu0/0/0/4/0", "HundredGigE0/0/0/4/0"), intf: "HundredGigE0/0/0/4/0", want: true},
		{name: "HundredGigE subinterface zero", output: strings.ReplaceAll(huRow, "Hu0/0/0/4/0", "Hu0/0/0/4/0.0"), intf: "HundredGigE0/0/0/4/0", want: true},
		{name: "HundredGigE wrapped neighbor", output: peer + "\n 32 0200.0201.0101 REACH Hu0/0/0/4/0 0/0/CPU0", intf: "HundredGigE0/0/0/4/0", want: true},
		{name: "HundredGigE wrong MAC", output: strings.ReplaceAll(huRow, "0200.0201.0101", "0200.0401.0101"), intf: "HundredGigE0/0/0/4/0"},
		{name: "HundredGigE wrong interface", output: huRow, intf: "HundredGigE0/0/0/4/1"},
		{name: "HundredGigE interface prefix collision", output: strings.ReplaceAll(huRow, "Hu0/0/0/4/0", "Hu0/0/0/4/00"), intf: "HundredGigE0/0/0/4/0"},
		{name: "HundredGigE wrong interface family", output: strings.ReplaceAll(huRow, "Hu0/0/0/4/0", "Te0/0/0/4/0"), intf: "HundredGigE0/0/0/4/0"},
		{name: "wrapped neighbor", output: peer + "\n 41 0200.0201.0101 REACH Te0/0/0/0/0 0/0/CPU0", intf: "TenGigE0/0/0/0/0", want: true},
		{name: "multicast follows resolved peer", output: row + "\n[Mcast adjacency] - 0000.0000.0000 REACH Te0/0/0/0/0 0/0/CPU0", intf: "TenGigE0/0/0/0/0", want: true},
		{name: "wrong MAC", output: strings.ReplaceAll(row, "0200.0201.0101", "0200.0401.0101"), intf: "TenGigE0/0/0/0/0"},
		{name: "wrong interface", output: row, intf: "TenGigE0/0/0/20/3"},
		{name: "interface prefix collision", output: strings.ReplaceAll(row, "Te0/0/0/0/0", "Te0/0/0/0/00"), intf: "TenGigE0/0/0/0/0"},
		{name: "wrong address", output: strings.ReplaceAll(row, peer, "fe80::4ff:fe01:101"), intf: "TenGigE0/0/0/0/0"},
		{name: "missing record", output: "IPv6 Address Age Link-layer Add State Interface Location", intf: "TenGigE0/0/0/0/0"},
		{name: "multicast cannot complete unresolved peer", output: peer + " 41\n[Mcast adjacency] - 0200.0201.0101 REACH Te0/0/0/0/0 0/0/CPU0", intf: "TenGigE0/0/0/0/0"},
		{name: "conflicting duplicate MAC", output: row + "\n" + strings.ReplaceAll(row, "0200.0201.0101", "0200.0401.0101"), intf: "TenGigE0/0/0/0/0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ciscoIPv6NeighborMatches(t, tc.output, tc.intf, netip.MustParseAddr(peer), mac)
			if got != tc.want || (err != nil) == tc.want {
				t.Errorf("ciscoIPv6NeighborMatches() = %v, %v; want match=%v", got, err, tc.want)
			}
		})
	}
}
