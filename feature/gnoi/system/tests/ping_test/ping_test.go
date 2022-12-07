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

package ping_test

import (
	"context"
	"io"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/netutil"
)

const (
	minimumPingRequestSize        = 56
	maximumDefaultPingRequestSize = 1512
	maximumPingRequestSize        = 9202
	minimumPingReplySize          = 56
	icmpHeaderSize                = 20
	// Set Minimum value to 1 to verify the field is not empty.
	minimumPingReplyTTL      = 1
	minimumPingReplySequence = 1
	minimumPingTime          = 1
	minimumSent              = 1
	minimumReceived          = 1
	minimumMinTime           = 1
	minimumAvgTime           = 1
	minimumMaxTime           = 1
	//StdDeviation would be 0 if we only send 1 ping.
	minimumStdDev = 1
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test cases:
//  - Send gNOI ping request with all options defined in system.proto.
//     - destination: Destination address to ping. It is required for each ping request.
//     - source: Source address to ping from.
//     - count: Number of ping request packets.
//     - interval: Nanoseconds between ping requests.
//     - wait = 5: Nanoseconds to wait for a response.
//     - size: Size of request packet with excluding ICMP header)
//     - do_not_fragment: Set the do not fragment bit. It only applied to IPv4 destinations.
//     - do_not_resolve: Do not try resolve the address returned.
//     - l3protocol: Layer3 protocol IPv4 or IPv6 for the ping.
//  - Verify echo replies are correct.
//     - Echo reply contains echo reply source, RTT time, bytes received, packet sequence and ttl.
//  - Verify ping summary stats in the response.
//     - source: Source of received bytes. It is the source address of ping request.
//     - sent: Total packets sent.
//     - received: Total packets received.
//     - min_time: Minimum round trip time in nanoseconds.
//     - avg_time: Average round trip time in nanoseconds.
//     - max_time: Maximum round trip time in nanoseconds.
//     - std_dev: Standard deviation in round trip time.
//
// Topology:
//   dut:port1 <--> ate:port1
//
// Test notes:
//  - Only the destination fields is required.
//  - Any field not specified is set to a reasonable server specified value.
//  - Not all fields are supported by all vendors.
//  - A count of 0 defaults to a vendor specified value, typically 5.
//  - A count of -1 means continue until the RPC times out or is canceled.
//  - If the interval is -1 then a flood ping is issued.
//  - If the size is 0, the vendor default size will be used (typically 56 bytes).
//
//  - A PingResponse represents either the response to a single ping packet
//    (the bytes field is non-zero) or the summary statistics (sent is non-zero).
//  - For a single ping packet, time is the round trip time, in nanoseconds.
//  - For summary statistics, it is the time spent by the ping operation.  The time is
//    not always present in summary statistics.
//  - The std_dev is not always present in summary statistics.
//
//  - gnoi operation commands can be sent and tested using CLI command grpcurl.
//    https://github.com/fullstorydev/grpcurl
//

func TestGNOIPing(t *testing.T) {
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

	commonExpectedIPv4Reply := &spb.PingResponse{
		Source:   ipv4Addrs[0].GetIp(),
		Time:     minimumPingTime,
		Bytes:    minimumPingReplySize,
		Sequence: minimumPingReplySequence,
		Ttl:      minimumPingReplyTTL,
	}
	commonExpectedIPv6Reply := &spb.PingResponse{
		Source:   ipv6Addrs[0].GetIp(),
		Time:     minimumPingTime,
		Bytes:    minimumPingReplySize,
		Sequence: minimumPingReplySequence,
		Ttl:      minimumPingReplyTTL,
	}
	commonExpectedReplyStats := &spb.PingResponse{
		Sent:     minimumSent,
		Received: minimumReceived,
		MinTime:  minimumMinTime,
		AvgTime:  minimumAvgTime,
		MaxTime:  minimumMaxTime,
		StdDev:   minimumStdDev,
	}

	cases := []struct {
		desc          string
		pingRequest   *spb.PingRequest
		expectedReply *spb.PingResponse
		expectedStats *spb.PingResponse
	}{{
		desc: "Check ping with IPv4 destination",
		pingRequest: &spb.PingRequest{
			Destination: ipv4Addrs[0].GetIp(),
		},
		expectedReply: commonExpectedIPv4Reply,
		expectedStats: commonExpectedReplyStats,
	}, {
		desc: "Check ping with IPv6 destination",
		pingRequest: &spb.PingRequest{
			Destination: ipv6Addrs[0].GetIp(),
		},
		expectedReply: commonExpectedIPv6Reply,
		expectedStats: commonExpectedReplyStats,
	}, {
		desc: "Check ping with IPv4 source",
		pingRequest: &spb.PingRequest{
			Destination: ipv4Addrs[0].GetIp(),
			Source:      ipv4Addrs[0].GetIp(),
		},
		expectedReply: commonExpectedIPv4Reply,
		expectedStats: commonExpectedReplyStats,
	}, {
		desc: "Check ping with IPv6 source",
		pingRequest: &spb.PingRequest{
			Destination: ipv6Addrs[0].GetIp(),
			Source:      ipv6Addrs[0].GetIp(),
		},
		expectedReply: commonExpectedIPv6Reply,
		expectedStats: commonExpectedReplyStats,
	}, {
		desc: "Check ping with IPv4 l3protocol",
		pingRequest: &spb.PingRequest{
			Destination: ipv4Addrs[0].GetIp(),
			Source:      ipv4Addrs[0].GetIp(),
			L3Protocol:  tpb.L3Protocol_IPV4,
		},
		expectedReply: commonExpectedIPv4Reply,
		expectedStats: commonExpectedReplyStats,
	}, {
		desc: "Check ping with IPv6 l3protocol",
		pingRequest: &spb.PingRequest{
			Destination: ipv6Addrs[0].GetIp(),
			Source:      ipv6Addrs[0].GetIp(),
			L3Protocol:  tpb.L3Protocol_IPV6,
		},
		expectedReply: commonExpectedIPv6Reply,
		expectedStats: commonExpectedReplyStats,
	}, {
		desc: "Check ping with IPv4 interval and wait",
		pingRequest: &spb.PingRequest{
			Destination: ipv4Addrs[0].GetIp(),
			Source:      ipv4Addrs[0].GetIp(),
			L3Protocol:  tpb.L3Protocol_IPV4,
			Interval:    123456,
			Wait:        12345678,
		},
		expectedReply: commonExpectedIPv4Reply,
		expectedStats: commonExpectedReplyStats,
	}, {
		desc: "Check ping with IPv6 interval and wait",
		pingRequest: &spb.PingRequest{
			Destination: ipv6Addrs[0].GetIp(),
			Source:      ipv6Addrs[0].GetIp(),
			L3Protocol:  tpb.L3Protocol_IPV6,
			Interval:    1234567,
			Wait:        123456789,
		},
		expectedReply: commonExpectedIPv6Reply,
		expectedStats: commonExpectedReplyStats,
	}, {
		desc: "Check ping with IPv4 do_not_resolve",
		pingRequest: &spb.PingRequest{
			Destination:  ipv4Addrs[0].GetIp(),
			Source:       ipv4Addrs[0].GetIp(),
			L3Protocol:   tpb.L3Protocol_IPV4,
			Interval:     123456,
			Wait:         12345678,
			DoNotResolve: true,
		},
		expectedReply: commonExpectedIPv4Reply,
		expectedStats: commonExpectedReplyStats,
	}, {
		desc: "Check ping with IPv6 do_not_resolve",
		pingRequest: &spb.PingRequest{
			Destination:  ipv6Addrs[0].GetIp(),
			Source:       ipv6Addrs[0].GetIp(),
			L3Protocol:   tpb.L3Protocol_IPV6,
			Interval:     1234567,
			Wait:         123456789,
			DoNotResolve: true,
		},
		expectedReply: commonExpectedIPv6Reply,
		expectedStats: commonExpectedReplyStats,
	}, {
		desc: "Check ping with IPv4 DF bit",
		pingRequest: &spb.PingRequest{
			Destination:   ipv4Addrs[0].GetIp(),
			Source:        ipv4Addrs[0].GetIp(),
			L3Protocol:    tpb.L3Protocol_IPV4,
			Interval:      123456,
			Wait:          12345678,
			DoNotFragment: true,
		},
		expectedReply: commonExpectedIPv4Reply,
		expectedStats: commonExpectedReplyStats,
	}, {
		desc: "Check ping with IPv4 count",
		pingRequest: &spb.PingRequest{
			Destination: ipv4Addrs[0].GetIp(),
			Source:      ipv4Addrs[0].GetIp(),
			L3Protocol:  tpb.L3Protocol_IPV4,
			Interval:    123456,
			Wait:        12345678,
			Count:       8,
		},
		expectedReply: commonExpectedIPv4Reply,
		expectedStats: &spb.PingResponse{
			Sent:     8,
			Received: 8,
			MinTime:  minimumMinTime,
			AvgTime:  minimumAvgTime,
			MaxTime:  minimumMaxTime,
			StdDev:   minimumStdDev,
		},
	}, {
		desc: "Check ping with IPv6 count",
		pingRequest: &spb.PingRequest{
			Destination: ipv6Addrs[0].GetIp(),
			Source:      ipv6Addrs[0].GetIp(),
			L3Protocol:  tpb.L3Protocol_IPV6,
			Interval:    1234567,
			Wait:        123456789,
			Count:       18,
		},
		expectedReply: commonExpectedIPv6Reply,
		expectedStats: &spb.PingResponse{
			Sent:     18,
			Received: 18,
			MinTime:  minimumMinTime,
			AvgTime:  minimumAvgTime,
			MaxTime:  minimumMaxTime,
			StdDev:   minimumStdDev,
		},
	}, {
		desc: "Check ping with IPv4 minimum packet size",
		pingRequest: &spb.PingRequest{
			Destination: ipv4Addrs[0].GetIp(),
			Source:      ipv4Addrs[0].GetIp(),
			L3Protocol:  tpb.L3Protocol_IPV4,
			Interval:    123456,
			Wait:        12345678,
			Count:       5,
			Size:        minimumPingRequestSize,
		},
		expectedReply: &spb.PingResponse{
			Source:   ipv4Addrs[0].GetIp(),
			Time:     minimumPingTime,
			Bytes:    minimumPingRequestSize - icmpHeaderSize,
			Sequence: minimumPingReplySequence,
			Ttl:      minimumPingReplyTTL,
		},
		expectedStats: commonExpectedReplyStats,
	}, {
		desc: "Check ping with IPv4 maximum default packet size",
		pingRequest: &spb.PingRequest{
			Destination: ipv4Addrs[0].GetIp(),
			Source:      ipv4Addrs[0].GetIp(),
			L3Protocol:  tpb.L3Protocol_IPV4,
			Interval:    123456,
			Wait:        12345678,
			Count:       6,
			Size:        maximumDefaultPingRequestSize,
		},
		expectedReply: &spb.PingResponse{
			Source:   ipv4Addrs[0].GetIp(),
			Time:     minimumPingTime,
			Bytes:    maximumDefaultPingRequestSize - icmpHeaderSize,
			Sequence: minimumPingReplySequence,
			Ttl:      minimumPingReplyTTL,
		},
		expectedStats: commonExpectedReplyStats,
	}, {
		desc: "Check ping with IPv4 maximum packet size",
		pingRequest: &spb.PingRequest{
			Destination: ipv4Addrs[0].GetIp(),
			Source:      ipv4Addrs[0].GetIp(),
			L3Protocol:  tpb.L3Protocol_IPV4,
			Interval:    123456,
			Wait:        12345678,
			Count:       7,
			Size:        maximumPingRequestSize,
		},
		expectedReply: &spb.PingResponse{
			Source:   ipv4Addrs[0].GetIp(),
			Time:     minimumPingTime,
			Bytes:    maximumPingRequestSize - icmpHeaderSize,
			Sequence: minimumPingReplySequence,
			Ttl:      minimumPingReplyTTL,
		},
		expectedStats: commonExpectedReplyStats,
	}, {
		desc: "Check ping with IPv6 minimum packet size",
		pingRequest: &spb.PingRequest{
			Destination: ipv6Addrs[0].GetIp(),
			Source:      ipv6Addrs[0].GetIp(),
			L3Protocol:  tpb.L3Protocol_IPV6,
			Interval:    123456,
			Wait:        12345678,
			Count:       8,
			Size:        minimumPingRequestSize,
		},
		expectedReply: &spb.PingResponse{
			Source:   ipv6Addrs[0].GetIp(),
			Time:     minimumPingTime,
			Bytes:    minimumPingRequestSize - icmpHeaderSize,
			Sequence: minimumPingReplySequence,
			Ttl:      minimumPingReplyTTL,
		},
		expectedStats: commonExpectedReplyStats,
	}, {
		desc: "Check ping with IPv6 maximum default packet size",
		pingRequest: &spb.PingRequest{
			Destination: ipv6Addrs[0].GetIp(),
			Source:      ipv6Addrs[0].GetIp(),
			L3Protocol:  tpb.L3Protocol_IPV6,
			Interval:    123456,
			Wait:        12345678,
			Count:       9,
			Size:        maximumDefaultPingRequestSize,
		},
		expectedReply: &spb.PingResponse{
			Source:   ipv6Addrs[0].GetIp(),
			Time:     minimumPingTime,
			Bytes:    maximumDefaultPingRequestSize - icmpHeaderSize,
			Sequence: minimumPingReplySequence,
			Ttl:      minimumPingReplyTTL,
		},
		expectedStats: commonExpectedReplyStats,
	}, {
		desc: "Check ping with IPv6 maximum packet size",
		pingRequest: &spb.PingRequest{
			Destination: ipv6Addrs[0].GetIp(),
			Source:      ipv6Addrs[0].GetIp(),
			L3Protocol:  tpb.L3Protocol_IPV6,
			Interval:    123456,
			Wait:        12345678,
			Count:       10,
			Size:        maximumPingRequestSize,
		},
		expectedReply: &spb.PingResponse{
			Source:   ipv6Addrs[0].GetIp(),
			Time:     minimumPingTime,
			Bytes:    maximumPingRequestSize - icmpHeaderSize,
			Sequence: minimumPingReplySequence,
			Ttl:      minimumPingReplyTTL,
		},
		expectedStats: commonExpectedReplyStats,
	}}

	gnoiClient := dut.RawAPIs().GNOI().Default(t)
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Logf("Sent ping request: %v\n\n", tc.pingRequest)

			pingClient, err := gnoiClient.System().Ping(context.Background(), tc.pingRequest)
			if err != nil {
				t.Fatalf("Failed to query gnoi endpoint: %v", err)
			}

			responses, err := fetchResponses(pingClient)
			if err != nil {
				t.Fatalf("Failed to handle gnoi ping client stream: %v", err)
			}
			t.Logf("Got ping responses: Items: %v\n, Content: %v\n\n", len(responses), responses)
			if len(responses) == 0 {
				t.Errorf("Number of responses to %v: got 0, want > 0", tc.pingRequest.Destination)
			}

			StdDevZero := true
			pingTime := responses[len(responses)-1].Time

			for i := 0; i < len(responses)-1; i++ {
				t.Logf("Check each ping reply %v out of %v.\n  %v\n", i+1, len(responses), responses[i])

				// Check StdDev if ping time is different
				if pingTime != responses[i].Time {
					StdDevZero = false
				}

				if responses[i].Source != tc.expectedReply.Source {
					t.Errorf("Ping reply source: got %v, want %v", responses[i].Source, tc.expectedReply.Source)
				}

				t.Logf("Check the following fields in echo response and skip them in summary stats.\n")
				if responses[i].Time < tc.expectedReply.Time {
					t.Errorf("Ping time: got %v, want >= %v", responses[i].Time, tc.expectedReply.Time)
				}
				if responses[i].Bytes < tc.expectedReply.Bytes {
					t.Errorf("Ping Bytes: got %v, want >= %v", responses[i].Bytes, tc.expectedReply.Bytes)
				}
				if responses[i].Sequence < tc.expectedReply.Sequence {
					t.Errorf("Ping time: got %v, want >= %v", responses[i].Sequence, tc.expectedReply.Sequence)
				}
				if responses[i].Ttl < tc.expectedReply.Ttl {
					t.Errorf("Ping TTL: got %v, want >= %v", responses[i].Ttl, tc.expectedReply.Ttl)
				}
			}

			summary := responses[len(responses)-1]
			t.Logf("Check ping reply summary stats.\n")
			if summary.Sent < tc.expectedStats.Sent {
				t.Errorf("Ping Sent: got %v, want >= %v", summary.Sent, tc.expectedStats.Sent)
			}
			if summary.Received < tc.expectedStats.Received {
				t.Errorf("Ping Sent: got %v, want >= %v", summary.Received, tc.expectedStats.Received)
			}
			if summary.MinTime < tc.expectedStats.MinTime {
				t.Errorf("Ping Received: got %v, want >= %v", summary.MinTime, tc.expectedStats.MinTime)
			}
			if summary.AvgTime < tc.expectedStats.AvgTime {
				t.Errorf("Ping AvgTime: got %v, want >= %v", summary.AvgTime, tc.expectedStats.AvgTime)
			}
			if summary.MaxTime < tc.expectedStats.MaxTime {
				t.Errorf("Ping MaxTime: got %v, want >= %v", summary.MaxTime, tc.expectedStats.MaxTime)
			}
			if summary.StdDev < tc.expectedStats.StdDev && !StdDevZero {
				t.Errorf("Ping MaxTime: got %v, want >= %v", summary.StdDev, tc.expectedStats.StdDev)
			}
		})
	}
}

func fetchResponses(c spb.System_PingClient) ([]*spb.PingResponse, error) {
	pingResp := []*spb.PingResponse{}
	for {
		resp, err := c.Recv()
		switch {
		case err == io.EOF:
			return pingResp, nil
		case err != nil:
			return nil, err
		default:
			pingResp = append(pingResp, resp)
		}
	}
}
