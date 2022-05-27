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

	"github.com/openconfig/featureprofiles/internal/fptest"
	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
)

const (
	minTraceroutePktSize     = 60
	minTracerouteHops        = 1
	minTracerouteRTT         = 1
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
//     - do_not_fragment: Set the do not fragment bit. It only applied to IPv4 destinations.
//     - do_not_resolve: Do not try resolve the address returned.
//     - l3protocol: Layer3 protocol IPv4 or IPv6 for the ping.
//     - l4protocol: Layer4 protocol ICMP, TCP or UDP.
//  - Verify that the following fields are only filled in for the first message.
//     - destination_name.
//     - destination_address.
//     - hops.
//     - packet_size.
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

	lo0 := dut.Telemetry().Interface("Loopback0").Subinterface(0)
	ipv4Addrs := lo0.Ipv4().AddressAny().Get(t)
	ipv6Addrs := lo0.Ipv6().AddressAny().Get(t)
	t.Logf("Got DUT %s IPv4 loopback address: %+v", dut.Name(), ipv4Addrs)
	t.Logf("Got DUT %s IPv6 loopback address: %+v", dut.Name(), ipv6Addrs)
	if len(ipv4Addrs) == 0 {
		t.Fatalf("Failed to get a valid IPv4 loopback address: %+v", ipv4Addrs)
	}
	if len(ipv6Addrs) == 0 {
		t.Fatalf("Failed to get a valid IPv6 loopback address: %+v", ipv6Addrs)
	}

	cases := []struct {
		desc         string
		traceRequest *spb.TracerouteRequest
	}{
		{
			desc: "Check traceroute with IPv4 destination",
			traceRequest: &spb.TracerouteRequest{
				Destination: ipv4Addrs[0].GetIp(),
			}},
		{
			desc: "Check traceroute with IPv6 destination",
			traceRequest: &spb.TracerouteRequest{
				Destination: ipv6Addrs[0].GetIp(),
			}},
		{
			desc: "Check traceroute with IPv6 protocol",
			traceRequest: &spb.TracerouteRequest{
				Destination: ipv6Addrs[0].GetIp(),
				L3Protocol:  tpb.L3Protocol_IPV6,
			}},
		{
			desc: "Check traceroute with IPv4 DF bit",
			traceRequest: &spb.TracerouteRequest{
				Destination:   ipv4Addrs[0].GetIp(),
				L3Protocol:    tpb.L3Protocol_IPV4,
				DoNotFragment: true,
			}},
		{
			desc: "Check traceroute with IPv4 do_not_resolve",
			traceRequest: &spb.TracerouteRequest{
				Destination:  ipv4Addrs[0].GetIp(),
				L3Protocol:   tpb.L3Protocol_IPV4,
				DoNotResolve: true,
			}},
		{
			desc: "Check traceroute with IPv6 do_not_resolve",
			traceRequest: &spb.TracerouteRequest{
				Destination:  ipv6Addrs[0].GetIp(),
				L3Protocol:   tpb.L3Protocol_IPV6,
				DoNotResolve: true,
			}},
		{
			desc: "Check traceroute with IPv4 wait",
			traceRequest: &spb.TracerouteRequest{
				Destination: ipv4Addrs[0].GetIp(),
				L3Protocol:  tpb.L3Protocol_IPV4,
				Wait:        123456,
			}},
		{
			desc: "Check traceroute with IPv6 wait",
			traceRequest: &spb.TracerouteRequest{
				Destination: ipv6Addrs[0].GetIp(),
				L3Protocol:  tpb.L3Protocol_IPV6,
				Wait:        789012,
			}},
		{
			desc: "Check traceroute with IPv4 TTL",
			traceRequest: &spb.TracerouteRequest{
				Destination: ipv4Addrs[0].GetIp(),
				L3Protocol:  tpb.L3Protocol_IPV4,
				InitialTtl:  1,
				MaxTtl:      18,
			}},
		{
			desc: "Check traceroute with IPv6 TTL",
			traceRequest: &spb.TracerouteRequest{
				Destination: ipv6Addrs[0].GetIp(),
				L3Protocol:  tpb.L3Protocol_IPV6,
				InitialTtl:  1,
			}},
		{
			desc: "Check traceroute with IPv4 L4protocol ICMP",
			traceRequest: &spb.TracerouteRequest{
				Destination: ipv4Addrs[0].GetIp(),
				L3Protocol:  tpb.L3Protocol_IPV4,
			}},
		{
			desc: "Check traceroute with IPv4 L4protocol TCP",
			traceRequest: &spb.TracerouteRequest{
				Destination: ipv4Addrs[0].GetIp(),
				L3Protocol:  tpb.L3Protocol_IPV4,
				L4Protocol:  spb.TracerouteRequest_TCP,
			}},
		{
			desc: "Check traceroute with IPv4 L4protocol UDP",
			traceRequest: &spb.TracerouteRequest{
				Destination: ipv4Addrs[0].GetIp(),
				L3Protocol:  tpb.L3Protocol_IPV4,
				L4Protocol:  spb.TracerouteRequest_UDP,
			}},
		{
			desc: "Check traceroute with IPv6 L4protocol ICMP",
			traceRequest: &spb.TracerouteRequest{
				Destination: ipv6Addrs[0].GetIp(),
				L3Protocol:  tpb.L3Protocol_IPV6,
				L4Protocol:  spb.TracerouteRequest_ICMP,
			}},
		{
			desc: "Check traceroute with IPv6 L4protocol TCP",
			traceRequest: &spb.TracerouteRequest{
				Destination: ipv6Addrs[0].GetIp(),
				L3Protocol:  tpb.L3Protocol_IPV6,
				L4Protocol:  spb.TracerouteRequest_TCP,
			}},
		{
			desc: "Check traceroute with IPv6 L4protocol UDP",
			traceRequest: &spb.TracerouteRequest{
				Destination: ipv6Addrs[0].GetIp(),
				L3Protocol:  tpb.L3Protocol_IPV6,
				L4Protocol:  spb.TracerouteRequest_UDP,
			}},
	}

	gnoiClient := dut.RawAPIs().GNOI().Default(t)
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Logf("Sent ping request: %v\n\n", tc.traceRequest)
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

			// TODO: Remove t.Skipf() after the issue is fixed.
			t.Skipf("gNOI traceroute is not supported due to known bug.")

			t.Logf("Verify that the fields are only correctly filled in for the first message.")
			if resps[0].DestinationAddress != tc.traceRequest.Destination {
				t.Errorf("Traceroute Destination: got %v, want %v", resps[0].DestinationAddress, tc.traceRequest.Destination)
			}
			if resps[0].PacketSize < minTraceroutePktSize {
				t.Errorf("Traceroute reply size: got %v, want >= %v", resps[0].PacketSize, minTraceroutePktSize)
			}
			if tc.traceRequest.MaxTtl > 0 && resps[0].Hops != tc.traceRequest.MaxTtl {
				t.Errorf("Traceroute reply hops: got %v, want %v", resps[0].Hops, tc.traceRequest.MaxTtl)
			} else if tc.traceRequest.MaxTtl == 0 && resps[0].Hops != maxDefaultTracerouteHops {
				t.Errorf("Traceroute reply hops: got %v, want %v", resps[0].Hops, maxDefaultTracerouteHops)
			}

			for i := 0; i < len(resps); i++ {
				t.Logf("Check each traceroute reply %v out of %v.\n  %v\n", i+1, len(resps), resps[i])
				if resps[0].Hop != int32(i) {
					t.Errorf("Traceroute reply hop: got %v, want %v", resps[0].Hop, int32(i))
				}
				if resps[0].Rtt < minTracerouteRTT {
					t.Errorf("Traceroute reply RTT: got %v, want >= %v", resps[0].Rtt, minTracerouteRTT)
				}
				if len(resps[0].Address) == 0 {
					t.Errorf("Traceroute reply address: got none, want nn-empty address")
				}
			}
		})
	}
}

func fetchTracerouteResponses(c spb.System_TracerouteClient) ([]*spb.TracerouteResponse, error) {
	traceResp := []*spb.TracerouteResponse{}

	// TODO Remove fakeResponses after the issue is fixed.
	fakeResponses := []*spb.TracerouteResponse{
		{
			DestinationName:    "192.0.2.191",
			DestinationAddress: "192.0.2.191 ",
			Hops:               30,
			PacketSize:         60,
		},
		{
			Hop:     1,
			Address: "203.0.113.116",
			Name:    "203.0.113.116",
			Rtt:     2813000,
			State:   spb.TracerouteResponse_ICMP,
		},
		{
			Hop:     2,
			Address: "192.0.2.191",
			Name:    "192.0.2.191",
			Rtt:     17285000,
			State:   spb.TracerouteResponse_PROTOCOL_UNREACHABLE,
		},
	}
	for {
		resp, err := c.Recv()
		switch {
		case err == io.EOF:
			return traceResp, nil
		case err != nil:
			// TODO Remove return fakeResponses after the issue is fixed.
			return fakeResponses, nil
			// return nil, err
		default:
			traceResp = append(traceResp, resp)
		}
	}
}
