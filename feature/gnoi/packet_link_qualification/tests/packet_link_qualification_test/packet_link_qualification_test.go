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
	"math"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	plqpb "github.com/openconfig/gnoi/packet_link_qualification"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

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

	gnoiClient1 := dut1.RawAPIs().GNOI(t)
	plqResp, err := gnoiClient1.LinkQualification().Capabilities(context.Background(), &plqpb.CapabilitiesRequest{})

	t.Logf("LinkQualification().Capabilities(): %v, err: %v", plqResp, err)
	if err != nil {
		t.Fatalf("Failed to handle gnoi LinkQualification().Capabilities(): %v", err)
	}

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
		min:  uint64(8184),
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

	t.Run("Reflector", func(t *testing.T) {
		ref := plqResp.GetReflector()
		if asicLB := ref.GetAsicLoopback(); asicLB.GetMinSetupDuration().GetSeconds() >= 1 && asicLB.GetMinTeardownDuration().GetSeconds() >= 1 {
			t.Logf("Device supports ASIC loopback reflector mode")
		} else if pmdLB := ref.GetPmdLoopback(); pmdLB.GetMinSetupDuration().GetSeconds() >= 1 && pmdLB.GetMinTeardownDuration().GetSeconds() >= 1 {
			t.Logf("Device supports PMD loopback reflector mode")
		} else {
			t.Errorf("Reflector MinSetupDuration or MinTeardownDuration is not >=1 for supported mode. Device reflector capabilities: %v", plqResp.GetReflector())
		}
	})
}

func TestNonexistingID(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	id := "non-extsing-ID"
	gnoiClient1 := dut1.RawAPIs().GNOI(t)
	getResp, err := gnoiClient1.LinkQualification().Get(context.Background(), &plqpb.GetRequest{Ids: []string{id}})

	t.Logf("LinkQualification().Get(): %v, err: %v", getResp, err)
	if err != nil {
		t.Fatalf("Failed to handle gnoi LinkQualification().Get(): %v", err)
	}

	t.Run("GetResponse", func(t *testing.T) {
		if got, want := getResp.GetResults()[id].GetStatus().GetCode(), int32(5); got != want {
			t.Errorf("getResp.GetResults()[id].GetStatus().GetCode(): got %v, want %v", got, want)
		}
	})

	deleteResp, err := gnoiClient1.LinkQualification().Delete(context.Background(), &plqpb.DeleteRequest{Ids: []string{id}})

	t.Logf("LinkQualification().Get(): %v, err: %v", getResp, err)
	if err != nil {
		t.Fatalf("Failed to handle gnoi LinkQualification().Delete(): %v", err)
	}

	t.Run("DeleteResp", func(t *testing.T) {
		if got, want := deleteResp.GetResults()[id].GetCode(), int32(5); got != want {
			t.Errorf("deleteResp.GetResults()[id].GetCode(): got %v, want %v", got, want)
		}
	})
}

func TestListDelete(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	gnoiClient1 := dut1.RawAPIs().GNOI(t)
	gnoiClient2 := dut2.RawAPIs().GNOI(t)

	clients := []binding.GNOIClients{gnoiClient1, gnoiClient2}
	for i, client := range clients {
		t.Logf("Check client: %d", i+1)
		listResp, err := client.LinkQualification().List(context.Background(), &plqpb.ListRequest{})
		t.Logf("LinkQualification().List(): %v, err: %v", listResp, err)
		if err != nil {
			t.Fatalf("Failed to handle gnoi LinkQualification().List(): %v", err)
		}
		if len(listResp.GetResults()) != 0 {
			for j, result := range listResp.GetResults() {
				t.Logf("Delete result %d: Result: %v", j, result)
				id := result.GetId()
				deleteResp, err := client.LinkQualification().Delete(context.Background(), &plqpb.DeleteRequest{Ids: []string{id}})

				t.Logf("LinkQualification().Delete(): %v, err: %v", deleteResp, err)
				if err != nil {
					t.Fatalf("Failed to handle gnoi LinkQualification().Delete(): %v", err)
				}
			}
		} else {
			t.Logf("The LinkQualification request was not found on client %d", i+1)
			continue
		}

		t.Logf("Verify that the qualification has been deleted on client %d", i+1)
		listResp, err = client.LinkQualification().List(context.Background(), &plqpb.ListRequest{})
		t.Logf("LinkQualification().List(): %v, err: %v", listResp, err)
		if err != nil {
			t.Fatalf("Failed to handle gnoi LinkQualification().List(): %v", err)
		}
		if got, want := len(listResp.GetResults()), 0; got != want {
			t.Errorf("len(listResp.GetResults()): got %v, want %v", got, want)
		}
	}
	if deviations.LinkQualWaitAfterDeleteRequired(dut1) {
		time.Sleep(10 * time.Second)
	}
}

func configInterfaceMTU(i *oc.Interface, dut *ondatra.DUTDevice) *oc.Interface {
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	if !deviations.OmitL2MTU(dut) {
		i.Mtu = ygot.Uint16(9000)
	}

	return i
}

func TestLinkQualification(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	dp1 := dut1.Port(t, "port1")
	dp2 := dut2.Port(t, "port1")
	t.Logf("dut1: %v, dut2: %v", dut1.Name(), dut2.Name())
	t.Logf("dut1 dp1 name: %v, dut2 dp2 name : %v", dp1.Name(), dp2.Name())

	for _, dut := range []*ondatra.DUTDevice{dut1, dut2} {
		d := gnmi.OC()
		p := dut.Port(t, "port1")
		i := &oc.Interface{Name: ygot.String(p.Name())}
		gnmi.Replace(t, dut, d.Interface(p.Name()).Config(), configInterfaceMTU(i, dut))
		if deviations.ExplicitPortSpeed(dut) {
			fptest.SetPortSpeed(t, p)
		}
	}

	plqID := dut1.Name() + ":" + dp1.Name() + "<->" + dut2.Name() + ":" + dp2.Name()
	type LinkQualificationDuration struct {
		// time needed to complete preparation
		setupDuration time.Duration
		// time duration to wait before starting link qual preparation
		preSyncDuration time.Duration
		// packet linkqual duration
		testDuration time.Duration
		// time to wait post link-qual before starting teardown
		generatorPostSyncDuration time.Duration
		reflectorPostSyncDuration time.Duration
		// time required to bring the interface back to pre-test state
		tearDownDuration time.Duration
	}
	plqDuration := &LinkQualificationDuration{
		preSyncDuration:           30 * time.Second,
		setupDuration:             30 * time.Second,
		testDuration:              120 * time.Second,
		generatorPostSyncDuration: 5 * time.Second,
		reflectorPostSyncDuration: 10 * time.Second,
		tearDownDuration:          30 * time.Second,
	}

	generatorCreateRequest := &plqpb.CreateRequest{
		Interfaces: []*plqpb.QualificationConfiguration{
			{
				Id:            plqID,
				InterfaceName: dp1.Name(),
				EndpointType: &plqpb.QualificationConfiguration_PacketGenerator{
					PacketGenerator: &plqpb.PacketGeneratorConfiguration{
						PacketRate: uint64(138888),
						PacketSize: uint32(8184),
					},
				},
				Timing: &plqpb.QualificationConfiguration_Rpc{
					Rpc: &plqpb.RPCSyncedTiming{
						Duration:         durationpb.New(plqDuration.testDuration),
						PreSyncDuration:  durationpb.New(plqDuration.preSyncDuration),
						SetupDuration:    durationpb.New(plqDuration.setupDuration),
						PostSyncDuration: durationpb.New(plqDuration.generatorPostSyncDuration),
						TeardownDuration: durationpb.New(plqDuration.tearDownDuration),
					},
				},
			},
		},
	}
	t.Logf("generatorCreateRequest: %v", generatorCreateRequest)

	intf := &plqpb.QualificationConfiguration{
		Id:            plqID,
		InterfaceName: dp2.Name(),

		Timing: &plqpb.QualificationConfiguration_Rpc{
			Rpc: &plqpb.RPCSyncedTiming{
				Duration:         durationpb.New(plqDuration.testDuration),
				PreSyncDuration:  durationpb.New(plqDuration.preSyncDuration),
				SetupDuration:    durationpb.New(plqDuration.setupDuration),
				PostSyncDuration: durationpb.New(plqDuration.reflectorPostSyncDuration),
				TeardownDuration: durationpb.New(plqDuration.tearDownDuration),
			},
		},
	}

	switch dut2.Vendor() {
	case ondatra.NOKIA, ondatra.JUNIPER:
		intf.EndpointType = &plqpb.QualificationConfiguration_AsicLoopback{
			AsicLoopback: &plqpb.AsicLoopbackConfiguration{},
		}
	default:
		intf.EndpointType = &plqpb.QualificationConfiguration_PmdLoopback{
			PmdLoopback: &plqpb.PmdLoopbackConfiguration{},
		}
	}

	reflectorCreateRequest := &plqpb.CreateRequest{
		Interfaces: []*plqpb.QualificationConfiguration{intf},
	}
	t.Logf("ReflectorCreateRequest: %v", reflectorCreateRequest)

	gnoiClient1 := dut1.RawAPIs().GNOI(t)
	gnoiClient2 := dut2.RawAPIs().GNOI(t)

	generatorCreateResp, err := gnoiClient1.LinkQualification().Create(context.Background(), generatorCreateRequest)
	t.Logf("LinkQualification().Create() generatorCreateResp: %v, err: %v", generatorCreateResp, err)
	if err != nil {
		t.Fatalf("Failed to handle generator LinkQualification().Create(): %v", err)
	}
	if got, want := generatorCreateResp.GetStatus()[plqID].GetCode(), int32(0); got != want {
		t.Errorf("generatorCreateResp: got %v, want %v", got, want)
	}

	reflectorCreateResp, err := gnoiClient2.LinkQualification().Create(context.Background(), reflectorCreateRequest)
	t.Logf("LinkQualification().Create() reflectorCreateResp: %v, err: %v", reflectorCreateResp, err)
	if err != nil {
		t.Fatalf("Failed to handle reflector LinkQualification().Create(): %v", err)
	}
	if got, want := reflectorCreateResp.GetStatus()[plqID].GetCode(), int32(0); got != want {
		t.Errorf("reflectorCreateResp: got %v, want %v", got, want)
	}

	sleepTime := 30 * time.Second
	minTestTime := plqDuration.testDuration + plqDuration.reflectorPostSyncDuration + plqDuration.preSyncDuration + plqDuration.setupDuration + plqDuration.tearDownDuration
	counter := int(minTestTime.Seconds())/int(sleepTime.Seconds()) + 2
	for i := 0; i <= counter; i++ {
		t.Logf("Wait for %v seconds: %d/%d", sleepTime.Seconds(), i+1, counter)
		time.Sleep(sleepTime)
		testDone := true
		for i, client := range []binding.GNOIClients{gnoiClient1, gnoiClient2} {
			t.Logf("Check client: %d", i+1)

			listResp, err := client.LinkQualification().List(context.Background(), &plqpb.ListRequest{})
			t.Logf("LinkQualification().List(): %v, err: %v", listResp, err)
			if err != nil {
				t.Fatalf("Failed to handle gnoi LinkQualification().List(): %v", err)
			}

			for j := 0; j < len(listResp.GetResults()); j++ {
				if listResp.GetResults()[j].GetState() != plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED {
					testDone = false
				}
			}
			if len(listResp.GetResults()) == 0 {
				testDone = false
			}
		}
		if testDone {
			t.Logf("Detected QualificationState_QUALIFICATION_STATE_COMPLETED.")
			break
		}
	}

	getRequest := &plqpb.GetRequest{
		Ids: []string{plqID},
	}

	var generatorPktsSent, generatorPktsRxed, reflectorPktsSent, reflectorPktsRxed uint64

	for i, client := range []binding.GNOIClients{gnoiClient1, gnoiClient2} {
		t.Logf("Check client: %d", i+1)
		getResp, err := client.LinkQualification().Get(context.Background(), getRequest)
		t.Logf("LinkQualification().Get(): %v, err: %v", getResp, err)
		if err != nil {
			t.Fatalf("Failed to handle LinkQualification().Get(): %v", err)
		}

		result := getResp.GetResults()[plqID]
		if got, want := result.GetStatus().GetCode(), int32(0); got != want {
			t.Errorf("result.GetStatus().GetCode(): got %v, want %v", got, want)
		}
		if got, want := result.GetState(), plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED; got != want {
			t.Errorf("result.GetState(): got %v, want %v", got, want)
		}
		if got, want := result.GetPacketsError(), uint64(0); got != want {
			t.Errorf("result.GetPacketsError(): got %v, want %v", got, want)
		}
		if got, want := result.GetPacketsDropped(), uint64(0); got != want {
			t.Errorf("result.GetPacketsDropped(): got %v, want %v", got, want)
		}

		if client == gnoiClient1 {
			generatorPktsSent = result.GetPacketsSent()
			generatorPktsRxed = result.GetPacketsReceived()
		}

		if client == gnoiClient2 {
			reflectorPktsSent = result.GetPacketsSent()
			reflectorPktsRxed = result.GetPacketsReceived()
		}
	}

	// The packet counters between Generator and Reflector mismatch tolerance level in percentage
	var tolerance float64 = 0.0001
	if !deviations.SkipPLQPacketsCountCheck(dut1) {
		if ((math.Abs(float64(generatorPktsSent)-float64(reflectorPktsRxed)))/(float64(generatorPktsSent)+float64(reflectorPktsRxed)+tolerance))*200.00 > tolerance {
			t.Errorf("The difference between packets received count at Reflector and packets sent count at Generator is greater than %0.4f percent: generatorPktsSent %v, reflectorPktsRxed %v", tolerance, generatorPktsSent, reflectorPktsRxed)
		}
		if ((math.Abs(float64(reflectorPktsSent)-float64(generatorPktsRxed)))/(float64(reflectorPktsSent)+float64(generatorPktsRxed)+tolerance))*200.00 > tolerance {
			t.Errorf("The difference between packets received count at Generator and packets sent count at Reflector is greater than %0.4f percent: reflectorPktsSent %v, generatorPktsRxed %v", tolerance, reflectorPktsSent, generatorPktsRxed)
		}
	}

}
