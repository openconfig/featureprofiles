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

package packet_link_qualification_test

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	plqpb "github.com/openconfig/gnoi/packet_link_qualification"
	"github.com/openconfig/ondatra"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test cases:
//  0) Connect two vendor devices back to back on 100G bandwidth ports.
//  1) Validate the link qualification Capabilities response.
//     - MaxHistoricalResultsPerInterface is >= 2.
//     - Time
//     - Generator:
//       - MinMtu > = 64,
//       - MaxMtu >= 9000,
//       - MaxBps >= 4e11,
//       - MaxPps >= 5e8,
//       - MinSetupDuration > 0
//       - MinTeardownDuration > 0,
//       - MinSampleInterval > 0,
//     - Generator:
//       - MinSetupDuration > 0
//       - MinTeardownDuration > 0,
//  2) Validate the error code is returned for Get and Delete requests with non-existing ID.
//       - Error code is 5 NOT_FOUND (HTTP Mapping: 404 Not Found).
//  3) Set a device as the NEAR_END (generator) device for Packet Based Link Qual.
//     - Issue gnoi.PacketLinkQual StartPacketQualification RPC to the device.
//       Provide following parameters:
//       - Id: A unique identifier for this run of the test
//       - InterfaceName: interface as the interface to be used as generator end.
//         This interface must be connected to the interface chosen on the reflector device using
//         100G connection.
//       - EndpointType: Qualification_end set as NEAR_END with PacketGeneratorConfiguration.
//     - Set the following parameters for link qualification service usage:
//       - PacketRate: Packet per second rate to use for this test
//       - PacketSize: Size of packets to inject. If unspecified, the default value is 1500 bytes.
//     - RPCSyncedTiming:
//       - PreSyncDuration: Minimum_wait_before_preparation_seconds. The default value for this is
//         70 seconds. Within this period, the device should:
//         - Initialize the link qualification state machine.
//         - Set port’s state to TESTING. This state is only relevant inside the linkQual service.
//           A port with the TESTING state set, will reject any further linkQualification requests.
//         - Set the port in loopback mode.
//       - SetupDuration: The requested setup time for the endpoint.
//       - Duration:The length of the qualification.
//       - PostSyncDuration: The amount time a side should wait before starting its teardown.
//  4) Set another device as the FAR_END (reflector) device for Packet Based Link Qual.
//     - Issue gnoi.PacketLinkQual StartPacketQualification RPC to the device.
//       Provide following parameters:
//     - Id: A unique identifier for this run of the test
//     - InterfaceName: Interface as the interface to be used as a reflector to turn the packet back.
//     - EndpointType: Qualification_end set as FAR_END.
//     - RPCSyncedTiming:
//       - Reflector timers should be same as the ones on the generator.
//  5) Get the result by issuing gnoi.PacketLinkQual GetPacketQualificationResult RPC to gather
//     the result of link qualification.Provide the following parameter.
//      - Id: The identifier used above on the NEAR_END side.
//      - Compare that the test_duration_in_secs, packet_size, bandwidth_utilization match the request.
//      - Ensure that the current_state is QUALIFICATION_STATE_COMPLETED
//      - Ensure that the num_corrupt_packets and num_packets_dropped_by_mmu are 0, and error_message
//        is not set.
//      - Ensure that the ports under test on the FAR_END and the NEAR_END have no test data being
//        sent on the ports.
//
// Topology:
//   dut1:port1 <--> port1:dut2
//
// Test notes:
//
//  - gnoi operation commands can be sent and tested using CLI command grpcurl.
//    https://github.com/fullstorydev/grpcurl
//

func TestCapabilitiesResponse(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	dp1 := dut1.Port(t, "port1")
	dp2 := dut2.Port(t, "port1")
	t.Logf("dut1: %v, dut2: %v", dut1, dut2)
	t.Logf("dut1 dp1 name: %v, dut2 dp2 name : %v", dp1.Name(), dp2.Name())

	gnoiClient1 := dut1.RawAPIs().GNOI().New(t)
	plqResp, err := gnoiClient1.LinkQualification().Capabilities(context.Background(), &plqpb.CapabilitiesRequest{})

	// TODO: Remove fakePlqResp and uncomment err checking if PlqResp is received from DUT.
	// if err != nil {
	// 	t.Fatalf("Failed to handle gnoi LinkQualification().Capabilities(): %v", err)
	// }
	t.Logf("LinkQualification().Capabilities(): %v, err: %v", plqResp, err)

	fakePlqResp := &plqpb.CapabilitiesResponse{
		MaxHistoricalResultsPerInterface: uint64(2),
		Time:                             &timestamppb.Timestamp{Seconds: int64(1666312526)},
		NtpSynced:                        true,
		Generator: &plqpb.GeneratorCapabilities{
			PacketGenerator: &plqpb.PacketGeneratorCapabilities{
				MinMtu:              uint32(64),
				MaxMtu:              uint32(9000),
				MaxBps:              uint64(4e11),
				MaxPps:              uint64(5e8),
				MinSetupDuration:    &durationpb.Duration{Seconds: int64(30)},
				MinTeardownDuration: &durationpb.Duration{Seconds: int64(30)},
				MinSampleInterval:   &durationpb.Duration{Seconds: int64(10)},
			},
		},
		Reflector: &plqpb.ReflectorCapabilities{
			PmdLoopback: &plqpb.PmdLoopbackCapabilities{
				MinSetupDuration:    &durationpb.Duration{Seconds: int64(30)},
				MinTeardownDuration: &durationpb.Duration{Seconds: int64(30)},
			},
		},
	}
	t.Logf("LinkQualification().Capabilities() fakePlqResp: %v", fakePlqResp)
	plqResp = fakePlqResp

	cases := []struct {
		desc string
		got  uint64
		min  uint64
	}{{
		desc: "Time",
		got:  uint64(plqResp.GetTime().GetSeconds()),
		min:  uint64(1),
	}, {
		desc: "MaxHistoricalResultsPerInterface",
		got:  uint64(plqResp.GetMaxHistoricalResultsPerInterface()),
		min:  uint64(2),
	}, {
		desc: "Reflector MinSetupDuration",
		got:  uint64(plqResp.GetReflector().GetPmdLoopback().GetMinSetupDuration().GetSeconds()),
		min:  uint64(1),
	}, {
		desc: "Reflector MinTeardownDuration",
		got:  uint64(plqResp.GetReflector().GetPmdLoopback().GetMinTeardownDuration().GetSeconds()),
		min:  uint64(1),
	}, {
		desc: "Generator MinSetupDuration",
		got:  uint64(plqResp.GetGenerator().GetPacketGenerator().GetMinSetupDuration().GetSeconds()),
		min:  uint64(1),
	}, {
		desc: "Generator MinTeardownDuration",
		got:  uint64(plqResp.GetGenerator().GetPacketGenerator().GetMinTeardownDuration().GetSeconds()),
		min:  uint64(1),
	}, {
		desc: "Generator MinSampleInterval",
		got:  uint64(plqResp.GetGenerator().GetPacketGenerator().GetMinSampleInterval().GetSeconds()),
		min:  uint64(10),
	}, {
		desc: "Generator MinMtu",
		got:  uint64(plqResp.GetGenerator().GetPacketGenerator().GetMinMtu()),
		min:  uint64(64),
	}, {
		desc: "Generator MaxMtu",
		got:  uint64(plqResp.GetGenerator().GetPacketGenerator().GetMaxMtu()),
		min:  uint64(9000),
	}, {
		desc: "Generator MaxBps",
		got:  uint64(plqResp.GetGenerator().GetPacketGenerator().GetMaxBps()),
		min:  uint64(1e11),
	}, {
		desc: "Generator MaxPps",
		got:  uint64(plqResp.GetGenerator().GetPacketGenerator().GetMaxPps()),
		min:  uint64(1e8),
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			if got, want := tc.got, tc.min; got < want {
				t.Errorf("%s: got %v, want >= %v", tc.desc, got, want)
			}
		})
	}
}

func TestNonexistingID(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	id := "non-extsing-ID"
	gnoiClient1 := dut1.RawAPIs().GNOI().Default(t)
	getResp, err := gnoiClient1.LinkQualification().Get(context.Background(), &plqpb.GetRequest{Ids: []string{id}})

	// TODO: Remove fakeResp and uncomment err checking if getResp is received from DUT.
	// if err != nil {
	// 	t.Fatalf("Failed to handle gnoi LinkQualification().Get(): %v", err)
	// }
	t.Logf("LinkQualification().Get(): %v, err: %v", getResp, err)

	fakeGetResp := &plqpb.GetResponse{
		Results: map[string]*plqpb.QualificationResult{
			id: {
				Status: &status.Status{
					Code:    int32(5),
					Message: "ID not found for result",
				},
			},
		},
	}
	t.Logf("LinkQualification().Get() fakePlqResp: %v", fakeGetResp)
	getResp = fakeGetResp

	t.Run("GetResponse", func(t *testing.T) {
		if got, want := getResp.GetResults()[id].GetStatus().GetCode(), int32(5); got != want {
			t.Errorf("GetResponse: got %v, want %v", got, want)
		}
	})

	deleteResp, err := gnoiClient1.LinkQualification().Delete(context.Background(), &plqpb.DeleteRequest{Ids: []string{id}})

	// TODO: Remove fakeResp and uncomment err checking if deleteResp is received from DUT.
	// if err != nil {
	// 	t.Fatalf("Failed to handle gnoi LinkQualification().Get(): %v", err)
	// }
	t.Logf("LinkQualification().Get(): %v, err: %v", getResp, err)

	fakeDeleteResp := &plqpb.DeleteResponse{
		Results: map[string]*status.Status{
			id: {
				Code:    int32(5),
				Message: "ID not found for deletion",
			},
		},
	}
	t.Logf("LinkQualification().Get() fakePlqResp: %v", fakeDeleteResp)
	deleteResp = fakeDeleteResp

	t.Run("DeleteResp", func(t *testing.T) {
		if got, want := deleteResp.GetResults()[id].GetCode(), int32(5); got != want {
			t.Errorf("DeleteResp: got %v, want %v", got, want)
		}
	})
}

func TestLinkQuality(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	dp1 := dut1.Port(t, "port1")
	dp2 := dut2.Port(t, "port1")
	t.Logf("dut1: %v, dut: %v", dut1.Name(), dut2.Name())
	t.Logf("dut1 dp1 name: %v, dut2 dp1 name : %v", dp1.Name(), dp2.Name())

	plqID := dut1.Name() + ":" + dp1.Name() + "<->" + dut2.Name() + ":" + dp2.Name()
	type packetLinkQualDuration struct {
		// time needed to complete preparation
		setupDuration time.Duration
		// time duration to wait before starting link qual preparation
		preSyncDuration time.Duration
		// packet linkqual duration
		testDuration time.Duration
		// time to wait post link-qual before starting teardown
		postSyncDuration time.Duration
		// time required to bring the interface back to pre-test state
		tearDownDuration time.Duration
	}
	plqDuration := &packetLinkQualDuration{
		preSyncDuration:  30 * time.Second,
		setupDuration:    30 * time.Second,
		testDuration:     600 * time.Second,
		postSyncDuration: 5 * time.Second,
		tearDownDuration: 30 * time.Second,
	}

	generatorCreateRequest := &plqpb.CreateRequest{
		Interfaces: []*plqpb.QualificationConfiguration{
			{
				Id:            plqID,
				InterfaceName: dp1.Name(),
				EndpointType: &plqpb.QualificationConfiguration_PacketGenerator{
					PacketGenerator: &plqpb.PacketGeneratorConfiguration{
						PacketRate: uint64(138888),
						PacketSize: uint32(9000),
					},
				},
				Timing: &plqpb.QualificationConfiguration_Rpc{
					Rpc: &plqpb.RPCSyncedTiming{
						Duration: &durationpb.Duration{
							Seconds: int64(plqDuration.testDuration.Seconds()),
						},
						PreSyncDuration: &durationpb.Duration{
							Seconds: int64(plqDuration.preSyncDuration.Seconds()),
						},
						SetupDuration: &durationpb.Duration{
							Seconds: int64(plqDuration.setupDuration.Seconds()),
						},
						PostSyncDuration: &durationpb.Duration{
							Seconds: int64(plqDuration.postSyncDuration.Seconds()),
						},
						TeardownDuration: &durationpb.Duration{
							Seconds: int64(plqDuration.tearDownDuration.Seconds()),
						},
					},
				},
			},
		},
	}
	t.Logf("generatorCreateRequest: %v", generatorCreateRequest)

	reflectorCreateRequest := &plqpb.CreateRequest{
		Interfaces: []*plqpb.QualificationConfiguration{
			{
				Id:            plqID,
				InterfaceName: dp2.Name(),
				EndpointType: &plqpb.QualificationConfiguration_PmdLoopback{
					PmdLoopback: &plqpb.PmdLoopbackConfiguration{},
				},
				Timing: &plqpb.QualificationConfiguration_Rpc{
					Rpc: &plqpb.RPCSyncedTiming{
						Duration: &durationpb.Duration{
							Seconds: int64(plqDuration.testDuration.Seconds()),
						},
						PreSyncDuration: &durationpb.Duration{
							Seconds: int64(plqDuration.preSyncDuration.Seconds()),
						},
						SetupDuration: &durationpb.Duration{
							Seconds: int64(plqDuration.setupDuration.Seconds()),
						},
						PostSyncDuration: &durationpb.Duration{
							Seconds: int64(plqDuration.postSyncDuration.Seconds()),
						},
						TeardownDuration: &durationpb.Duration{
							Seconds: int64(plqDuration.tearDownDuration.Seconds()),
						},
					},
				},
			},
		},
	}
	t.Logf("ReflectorCreateRequest: %v", reflectorCreateRequest)

	gnoiClient1 := dut1.RawAPIs().GNOI().Default(t)
	gnoiClient2 := dut1.RawAPIs().GNOI().Default(t)

	generatorCreateResp, err := gnoiClient1.LinkQualification().Create(context.Background(), generatorCreateRequest)
	// TODO: Remove fakeResp and uncomment err checking if generatorCreateResp is received from DUT.
	// if err != nil {
	// 	t.Fatalf("Failed to handle generator LinkQualification().Create(): %v", err)
	// }
	t.Logf("LinkQualification().Create(): %v, err: %v", generatorCreateResp, err)

	reflectorCreateResp, err := gnoiClient2.LinkQualification().Create(context.Background(), reflectorCreateRequest)
	// TODO: Remove fakeResp and uncomment err checking if reflectorCreateResp is received from DUT.
	// if err != nil {
	// 	t.Fatalf("Failed to handle reflector LinkQualification().Create(): %v", err)
	// }
	// time.Sleep(1200 * time.Second)
	t.Logf("LinkQualification().Create(): %v, err: %v", reflectorCreateResp, err)

	fakeCreateResp := &plqpb.CreateResponse{
		Status: map[string]*status.Status{
			plqID: {
				Code:    int32(0), //OK = 0 and HTTP Mapping: 200 OK.
				Message: "request id " + plqID,
			},
		},
	}
	t.Logf("fakeCreateResp: %v", fakeCreateResp)

	generatorCreateResp = fakeCreateResp
	if got, want := generatorCreateResp.GetStatus()[plqID].GetCode(), int32(0); got != want {
		t.Errorf("generatorCreateResp: got %v, want %v", got, want)
	}
	reflectorCreateResp = fakeCreateResp
	if got, want := reflectorCreateResp.GetStatus()[plqID].GetCode(), int32(0); got != want {
		t.Errorf("reflectorCreateResp: got %v, want %v", got, want)
	}

	getRequest := &plqpb.GetRequest{
		Ids: []string{plqID},
	}
	getResp, err := gnoiClient1.LinkQualification().Get(context.Background(), getRequest)
	// TODO: Remove fakeResp and uncomment err checking if getResp is received from DUT.
	// if err != nil {
	// 	t.Fatalf("Failed to handle generator LinkQualification().Get(): %v", err)
	// }
	t.Logf("LinkQualification().Create(): %v, err: %v", getResp, err)

	fakeGetResp := &plqpb.GetResponse{
		Results: map[string]*plqpb.QualificationResult{
			plqID: {
				Id:                              plqID,
				InterfaceName:                   dp1.Name(),
				State:                           plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED,
				PacketsSent:                     uint64(83316403),
				PacketsReceived:                 uint64(83316343),
				PacketsError:                    uint64(0),
				PacketsDropped:                  uint64(60),
				StartTime:                       &timestamppb.Timestamp{Seconds: int64(1666375740)},
				EndTime:                         &timestamppb.Timestamp{Seconds: int64(1666376341)},
				ExpectedRateBytesPerSecond:      uint64(1249745125),
				QualificationRateBytesPerSecond: uint64(1249745125),
				Status: &status.Status{
					Code:    int32(0), //OK = 0 and HTTP Mapping: 200 OK.
					Message: "request id " + plqID,
				},
			},
		},
	}
	t.Logf("fakeGetResp: %v", fakeGetResp)

	getResp = fakeGetResp
	result := getResp.GetResults()[plqID]
	if got, want := result.GetStatus().GetCode(), int32(0); got != want {
		t.Errorf("getResp: got %v, want %v", got, want)
	}

	if got, want := result.GetQualificationRateBytesPerSecond(), result.GetExpectedRateBytesPerSecond(); got != want {
		t.Errorf("Packet rate in Bps: got %v, want %v", got, want)
	}
}
