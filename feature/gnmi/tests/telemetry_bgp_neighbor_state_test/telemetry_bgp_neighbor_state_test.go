// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package telemetry_bgp_neighbor_state_test implements HA-1.0
package telemetry_bgp_neighbor_state_test

import (
	"fmt"
	"net"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

// TestMain sets up the test environment.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestTelemetryBGPNeighborState tests the presence and compliance of BGP neighbor state oc paths.
func TestTelemetryBGPNeighborState(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	niNames := gnmi.GetAll(t, dut, gnmi.OC().NetworkInstanceAny().Name().State())
	if len(niNames) == 0 {
		t.Fatal("No network instances found on DUT")
	}

	for _, niName := range niNames {
		// Discover all protocol names in this network instance
		protocolNames := gnmi.GetAll(t, dut, gnmi.OC().NetworkInstance(niName).ProtocolAny().Name().State())
		for _, protoName := range protocolNames {
			// Check if this protocol is of type BGP by looking up its BGP state
			bgpPath := gnmi.OC().NetworkInstance(niName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, protoName).Bgp()
			if !gnmi.Lookup(t, dut, bgpPath.State()).IsPresent() {
				continue
			}

			// Discover neighbors for this BGP protocol
			neighbors := gnmi.GetAll(t, dut, bgpPath.NeighborAny().NeighborAddress().State())
			for _, nbrAddr := range neighbors {
				t.Run(fmt.Sprintf("NI:%s/Proto:%s/Neighbor:%s", niName, protoName, nbrAddr), func(t *testing.T) {
					neighborPath := bgpPath.Neighbor(nbrAddr)

					t.Run("session-state", func(t *testing.T) {
						val, present := gnmi.Lookup(t, dut, neighborPath.SessionState().State()).Val()
						if !present {
							t.Errorf("Path session-state is not present")
						} else {
							switch val {
							case oc.Bgp_Neighbor_SessionState_IDLE, oc.Bgp_Neighbor_SessionState_CONNECT:
								t.Logf("Session state %v is compliant", val)
							case oc.Bgp_Neighbor_SessionState_ACTIVE, oc.Bgp_Neighbor_SessionState_OPENSENT:
								t.Logf("Session state %v is compliant", val)
							case oc.Bgp_Neighbor_SessionState_OPENCONFIRM, oc.Bgp_Neighbor_SessionState_ESTABLISHED:
								t.Logf("Session state %v is compliant", val)
							default:
								t.Errorf("Session state %v is not compliant with OpenConfig standards", val)
							}
						}
					})

					t.Run("admin-status", func(t *testing.T) {
						if val, present := gnmi.Lookup(t, dut, neighborPath.Enabled().State()).Val(); !present {
							t.Errorf("Path admin-status (enabled) is not present")
						} else {
							t.Logf("Admin status (enabled): %v is compliant", val)
						}
					})

					t.Run("peer-as", func(t *testing.T) {
						val, present := gnmi.Lookup(t, dut, neighborPath.PeerAs().State()).Val()
						if !present {
							t.Errorf("Path peer-as is not present")
						} else if val == 0 {
							t.Errorf("Peer AS %v is not compliant (should be > 0)", val)
						} else {
							t.Logf("Peer AS: %v is compliant", val)
						}
					})

					t.Run("neighbor-address", func(t *testing.T) {
						val, present := gnmi.Lookup(t, dut, neighborPath.NeighborAddress().State()).Val()
						if !present {
							t.Errorf("Path neighbor-address is not present")
						} else {
							if net.ParseIP(val) == nil {
								t.Errorf("Neighbor address %v is not a valid IP", val)
							}
							if val != nbrAddr {
								t.Errorf("Neighbor address mismatch: got %v, want %v", val, nbrAddr)
							}
							t.Logf("Neighbor address: %v is compliant", val)
						}
					})

					t.Run("local-address", func(t *testing.T) {
						val, present := gnmi.Lookup(t, dut, neighborPath.Transport().LocalAddress().State()).Val()
						if !present {
							t.Errorf("Path transport/local-address is not present")
						} else if net.ParseIP(val) == nil {
							t.Errorf("Local address %v is not a valid IP", val)
						} else {
							t.Logf("Local address: %v is compliant", val)
						}
					})

					t.Run("last-established", func(t *testing.T) {
						if val, present := gnmi.Lookup(t, dut, neighborPath.LastEstablished().State()).Val(); !present {
							t.Errorf("Path last-established is not present")
						} else {
							t.Logf("Last established timestamp: %v is compliant", val)
						}
					})

					t.Run("established-transitions", func(t *testing.T) {
						if val, present := gnmi.Lookup(t, dut, neighborPath.EstablishedTransitions().State()).Val(); !present {
							t.Errorf("Path established-transitions is not present")
						} else {
							t.Logf("Established transitions: %v is compliant", val)
						}
					})

					t.Run("messages/received/UPDATE", func(t *testing.T) {
						if val, present := gnmi.Lookup(t, dut, neighborPath.Messages().Received().UPDATE().State()).Val(); !present {
							t.Errorf("Path messages/received/UPDATE is not present")
						} else {
							t.Logf("Received UPDATEs: %v is compliant", val)
						}
					})

					t.Run("messages/received/last-notification-error-code", func(t *testing.T) {
						if val, present := gnmi.Lookup(t, dut, neighborPath.Messages().Received().LastNotificationErrorCode().State()).Val(); !present {
							t.Errorf("Path messages/received/last-notification-error-code is not present")
						} else {
							t.Logf("Last notification error code: %v is compliant", val)
						}
					})

					t.Run("messages/sent/UPDATE", func(t *testing.T) {
						if val, present := gnmi.Lookup(t, dut, neighborPath.Messages().Sent().UPDATE().State()).Val(); !present {
							t.Errorf("Path messages/sent/UPDATE is not present")
						} else {
							t.Logf("Sent UPDATEs: %v is compliant", val)
						}
					})

					t.Run("ipv4-unicast/prefixes/received", func(t *testing.T) {
						if val, present := gnmi.Lookup(t, dut, neighborPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes().Received().State()).Val(); !present {
							t.Errorf("Path ipv4-unicast/prefixes/received is not present")
						} else {
							t.Logf("IPv4 Received Prefixes: %v is compliant", val)
						}
					})

					t.Run("ipv4-unicast/prefixes/sent", func(t *testing.T) {
						if val, present := gnmi.Lookup(t, dut, neighborPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes().Sent().State()).Val(); !present {
							t.Errorf("Path ipv4-unicast/prefixes/sent is not present")
						} else {
							t.Logf("IPv4 Sent Prefixes: %v is compliant", val)
						}
					})

					t.Run("ipv6-unicast/prefixes/received", func(t *testing.T) {
						if val, present := gnmi.Lookup(t, dut, neighborPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Prefixes().Received().State()).Val(); !present {
							t.Errorf("Path ipv6-unicast/prefixes/received is not present")
						} else {
							t.Logf("IPv6 Received Prefixes: %v is compliant", val)
						}
					})

					t.Run("ipv6-unicast/prefixes/sent", func(t *testing.T) {
						if val, present := gnmi.Lookup(t, dut, neighborPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Prefixes().Sent().State()).Val(); !present {
							t.Errorf("Path ipv6-unicast/prefixes/sent is not present")
						} else {
							t.Logf("IPv6 Sent Prefixes: %v is compliant", val)
						}
					})

					// Handle prefix-limit paths with deviations
					t.Run("ipv6-unicast/prefix-limit/max-prefixes", func(t *testing.T) {
						var val uint32
						var present bool
						if deviations.BGPExplicitPrefixLimitReceived(dut) {
							val, present = gnmi.Lookup(t, dut, neighborPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Ipv6Unicast().PrefixLimitReceived().MaxPrefixes().State()).Val()
						} else {
							val, present = gnmi.Lookup(t, dut, neighborPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Ipv6Unicast().PrefixLimit().MaxPrefixes().State()).Val()
						}
						if !present {
							t.Errorf("IPv6 max-prefixes path is not present")
						} else {
							t.Logf("IPv6 Max Prefixes: %v is compliant", val)
						}
					})

					t.Run("ipv4-unicast/prefix-limit/warning-threshold-pct", func(t *testing.T) {
						var val uint8
						var present bool
						if deviations.BGPExplicitPrefixLimitReceived(dut) {
							val, present = gnmi.Lookup(t, dut, neighborPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().PrefixLimitReceived().WarningThresholdPct().State()).Val()
						} else {
							val, present = gnmi.Lookup(t, dut, neighborPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().PrefixLimit().WarningThresholdPct().State()).Val()
						}
						if !present {
							t.Errorf("IPv4 warning-threshold-pct path is not present")
						} else if val > 100 {
							t.Errorf("Warning threshold percentage %v is invalid (> 100)", val)
						} else {
							t.Logf("IPv4 Warning Threshold Pct: %v is compliant", val)
						}
					})
				})
			}
		}
	}
}
