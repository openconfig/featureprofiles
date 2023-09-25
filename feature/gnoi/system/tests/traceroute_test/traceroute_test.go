// Copyright 2022 Google LLC
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

package traceroute_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/netutil"
)

const (
	minTracerouteHops        = 1
	minTracerouteRTT         = 0 // the device traceroute to its loopback, the RTT can be zero.
	maxDefaultTracerouteHops = 30
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test cases:
//  - Send gNOI traceroute request with all options defined in system.proto.
//     - destination: Destination address to traceroute. It is required for each traceroute request.
//     - source: Source address to traceroute from.
//     - initial_ttl: Initial TTL. (default=1).
//     - max_ttl: Maximum number of hops. (default=30).
//     - wait: Nanoseconds to wait for a response.
//     - do_not_resolve: Do not try resolve the address returned.
//     - l3protocol: Layer3 protocol IPv4 or IPv6 for the ping.
//     - l4protocol: Layer4 protocol ICMP, TCP or UDP.
//  - Verify that the following fields are only filled in for the first message.
//     - destination_name.
//     - destination_address.
//     - hops.
//  - Verify that traceroute response contains some of the following fields.
//     - hop: Hop number is required.
//     - address: Address of responding hop is required.
//     - name: Name of responding hop.
//     - rtt: Round trip time in nanoseconds.
//     - state" State of this hop.
//
// Topology:
//   dut:port1 <--> ate:port1
//
// Test notes:
//  - A TracerouteRequest describes the traceroute operation to perform.  Only the
//    destination field is required.  Any field not specified is set to a
//    reasonable server specified value.  Not all fields are supported by all
//    vendors.
//  - A TraceRouteResponse contains the result of a single traceroute packet.
//  - There is an initial response that provides information about the
//    traceroute request itself and contains at least one of the fields in the
//    initial block of fields and none of the fields following that block.  All
//    subsequent responses should not contain any of these fields.
//  - Typically multiple responses are received for each hop, as the packets are
//    received.
//
//  - gnoi operation commands can be sent and tested using CLI command grpcurl.
//    https://github.com/fullstorydev/grpcurl
//

func TestGNOITraceroute(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	lbIntf := netutil.LoopbackInterface(t, dut, 0)
	lo0 := gnmi.OC().Interface(lbIntf).Subinterface(0)
	ipv4Addrs := gnmi.GetAll(t, dut, lo0.Ipv4().AddressAny().State())
	ipv6Addrs := gnmi.GetAll(t, dut, lo0.Ipv6().AddressAny().State())
	t.Logf("Got DUT %s IPv4 loopback address: %+v", dut.Name(), ipv4Addrs)
	t.Logf("Got DUT %s IPv6 loopback address: %+v", dut.Name(), ipv6Addrs)
	if len(ipv4Addrs) == 0 {
		t.Fatalf("Failed to get a valid IPv4 loopback address: %+v", ipv4Addrs)
	}
	if len(ipv6Addrs) == 0 {
		t.Fatalf("Failed to get a valid IPv6 loopback address: %+v", ipv6Addrs)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, lbIntf, deviations.DefaultNetworkInstance(dut), 0)
	}
	cases := []struct {
		desc              string
		traceRequest      *spb.TracerouteRequest
		defaultL4Protocol bool
	}{
		{
			desc:              "Check traceroute with IPv4 destination",
			defaultL4Protocol: true,
			traceRequest: &spb.TracerouteRequest{
				Destination:    ipv4Addrs[0].GetIp(),
				DoNotLookupAsn: true,
			}},
		{
			desc:              "Check traceroute with IPv6 destination",
			defaultL4Protocol: true,
			traceRequest: &spb.TracerouteRequest{
				Destination:    ipv6Addrs[0].GetIp(),
				DoNotLookupAsn: true,
			}},
		{
			desc:              "Check traceroute with IPv6 protocol",
			defaultL4Protocol: true,
			traceRequest: &spb.TracerouteRequest{
				Destination:    ipv6Addrs[0].GetIp(),
				L3Protocol:     tpb.L3Protocol_IPV6,
				DoNotLookupAsn: true,
			}},
		{
			desc:              "Check traceroute with IPv4 do_not_resolve",
			defaultL4Protocol: true,
			traceRequest: &spb.TracerouteRequest{
				Destination:    ipv4Addrs[0].GetIp(),
				L3Protocol:     tpb.L3Protocol_IPV4,
				DoNotResolve:   true,
				DoNotLookupAsn: true,
			}},
		{
			desc:              "Check traceroute with IPv6 do_not_resolve",
			defaultL4Protocol: true,
			traceRequest: &spb.TracerouteRequest{
				Destination:    ipv6Addrs[0].GetIp(),
				L3Protocol:     tpb.L3Protocol_IPV6,
				DoNotResolve:   true,
				DoNotLookupAsn: true,
			}},
		{
			desc:              "Check traceroute with IPv4 wait",
			defaultL4Protocol: true,
			traceRequest: &spb.TracerouteRequest{
				Destination:    ipv4Addrs[0].GetIp(),
				L3Protocol:     tpb.L3Protocol_IPV4,
				Wait:           1234567890,
				DoNotLookupAsn: true,
			}},
		{
			desc:              "Check traceroute with IPv6 wait",
			defaultL4Protocol: true,
			traceRequest: &spb.TracerouteRequest{
				Destination:    ipv6Addrs[0].GetIp(),
				L3Protocol:     tpb.L3Protocol_IPV6,
				Wait:           1e9,
				DoNotLookupAsn: true,
			}},
		{
			desc:              "Check traceroute with IPv4 TTL",
			defaultL4Protocol: true,
			traceRequest: &spb.TracerouteRequest{
				Destination:    ipv4Addrs[0].GetIp(),
				L3Protocol:     tpb.L3Protocol_IPV4,
				InitialTtl:     1,
				MaxTtl:         18,
				DoNotLookupAsn: true,
			}},
		{
			desc:              "Check traceroute with IPv6 TTL",
			defaultL4Protocol: true,
			traceRequest: &spb.TracerouteRequest{
				Destination:    ipv6Addrs[0].GetIp(),
				L3Protocol:     tpb.L3Protocol_IPV6,
				InitialTtl:     1,
				DoNotLookupAsn: true,
			}},
		{
			desc:              "Check traceroute with IPv4 L4protocol ICMP",
			defaultL4Protocol: true,
			traceRequest: &spb.TracerouteRequest{
				Destination:    ipv4Addrs[0].GetIp(),
				L3Protocol:     tpb.L3Protocol_IPV4,
				DoNotLookupAsn: true,
			}},
		{
			desc: "Check traceroute with IPv4 L4protocol TCP",
			traceRequest: &spb.TracerouteRequest{
				Destination:    ipv4Addrs[0].GetIp(),
				L3Protocol:     tpb.L3Protocol_IPV4,
				L4Protocol:     spb.TracerouteRequest_TCP,
				DoNotLookupAsn: true,
			}},
		{
			desc: "Check traceroute with IPv4 L4protocol UDP",
			traceRequest: &spb.TracerouteRequest{
				Destination:    ipv4Addrs[0].GetIp(),
				L3Protocol:     tpb.L3Protocol_IPV4,
				L4Protocol:     spb.TracerouteRequest_UDP,
				DoNotLookupAsn: true,
			}},
		{
			desc: "Check traceroute with IPv6 L4protocol ICMP",
			traceRequest: &spb.TracerouteRequest{
				Destination:    ipv6Addrs[0].GetIp(),
				L3Protocol:     tpb.L3Protocol_IPV6,
				L4Protocol:     spb.TracerouteRequest_ICMP,
				DoNotLookupAsn: true,
			}},
		{
			desc: "Check traceroute with IPv6 L4protocol TCP",
			traceRequest: &spb.TracerouteRequest{
				Destination:    ipv6Addrs[0].GetIp(),
				L3Protocol:     tpb.L3Protocol_IPV6,
				L4Protocol:     spb.TracerouteRequest_TCP,
				DoNotLookupAsn: true,
			}},
		{
			desc: "Check traceroute with IPv6 L4protocol UDP",
			traceRequest: &spb.TracerouteRequest{
				Destination:    ipv6Addrs[0].GetIp(),
				L3Protocol:     tpb.L3Protocol_IPV6,
				L4Protocol:     spb.TracerouteRequest_UDP,
				DoNotLookupAsn: true,
			}},
	}

	gnoiClient := dut.RawAPIs().GNOI(t)
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			time.Sleep(1 * time.Second) // some devices do not allow back to back traceroute to prevent flooding
			if deviations.TraceRouteL4ProtocolUDP(dut) {
				if tc.defaultL4Protocol {
					tc.traceRequest.L4Protocol = spb.TracerouteRequest_UDP
				}
				if tc.traceRequest.L4Protocol != spb.TracerouteRequest_UDP {
					t.Skip("Test is skiped due to the TraceRouteL4ProtocolUDP deviation")
				}
			}
			t.Logf("Sent traceroute request: %v\n\n", tc.traceRequest)
			traceClient, err := gnoiClient.System().Traceroute(context.Background(), tc.traceRequest)
			if err != nil {
				t.Fatalf("Failed to query gnoi endpoint: %v", err)
			}
			resps, err := fetchTracerouteResponses(traceClient)
			if err != nil {
				t.Fatalf("Failed to handle gnoi ping client stream: %v", err)
			}
			t.Logf("Got traceroute responses: Items: %v\n, Content: %v\n\n", len(resps), resps)
			if len(resps) == 0 {
				t.Errorf("Number of responses to %v: got 0, want > 0", tc.traceRequest.Destination)
			}

			t.Logf("Verify that the fields are only correctly filled in for the first message.")
			if resps[0].DestinationAddress != tc.traceRequest.Destination {
				t.Errorf("Traceroute Destination: got %v, want %v", resps[0].DestinationAddress, tc.traceRequest.Destination)
			}
			if tc.traceRequest.MaxTtl > 0 && resps[0].Hops != tc.traceRequest.MaxTtl {
				t.Errorf("Traceroute reply hops: got %v, want %v", resps[0].Hops, tc.traceRequest.MaxTtl)
			} else if tc.traceRequest.MaxTtl == 0 && resps[0].Hops != maxDefaultTracerouteHops {
				t.Errorf("Traceroute reply hops: got %v, want %v", resps[0].Hops, maxDefaultTracerouteHops)
			}

			for i := 1; i < len(resps); i++ {
				t.Logf("Check each traceroute reply %v out of %v:\n  %v\n", i, len(resps)-1, resps[i])
				if resps[i].GetHop() == 0 {
					t.Errorf("Traceroute reply hop: got 0, want > 0")
				}
				if resps[i].GetRtt() < minTracerouteRTT {
					t.Errorf("Traceroute reply RTT: got %v, want >= %v", resps[i].GetRtt(), minTracerouteRTT)
				}
				if len(resps[i].GetAddress()) == 0 {
					t.Errorf("Traceroute reply address: got none, want non-empty address")
				}
			}
		})
	}
}

func fetchTracerouteResponses(c spb.System_TracerouteClient) ([]*spb.TracerouteResponse, error) {
	traceResp := []*spb.TracerouteResponse{}
	for {
		resp, err := c.Recv()
		switch {
		case err == io.EOF:
			return traceResp, nil
		case err != nil:
			return nil, err
		default:
			traceResp = append(traceResp, resp)
		}
	}
}
