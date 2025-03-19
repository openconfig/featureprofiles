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
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	plqpb "github.com/openconfig/gnoi/packet_link_qualification"
	"github.com/openconfig/gnoigo"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Topology:
//   dut1:port1 <--> port2:dut1 - 400g links (as singleton and memberlink)
//   dut1:port3 <--> port4:dut1 - 100g links(as singleton and memberlink)
//
// Test notes:
//
//  - gnoi operation commands can be sent and tested using CLI command grpcurl.
//    https://github.com/fullstorydev/grpcurl
//

type aggPortData struct {
	port1IPV4     string
	port2IPV4     string
	dutPort1IPv4  string
	dutPort2IPv4  string
	dutPort3IPv4  string
	dutPort4IPv4  string
	dutAgg1Name   string
	dutAgg2Name   string
	aggPortIDDUT1 uint32
	aggPortIDDUT2 uint32
	aggPortIDDUT3 uint32
	aggPortIDDUT4 uint32
	aggPortID1    uint32
	aggPortID2    uint32
}

type LinkQualificationDuration struct {
	generatorsetupDuration    time.Duration
	reflectorsetupDuration    time.Duration
	generatorpreSyncDuration  time.Duration
	reflectorpreSyncDuration  time.Duration
	testDuration              time.Duration
	generatorPostSyncDuration time.Duration
	reflectorPostSyncDuration time.Duration
	generatorTeardownDuration time.Duration
	reflectorTeardownDuration time.Duration
}

const (
	ipv4PLen = 30
)

var (
	minRequiredGeneratorMTU = uint64(8184)
	minRequiredGeneratorPPS = uint64(1e8)
	agg1                    = &aggPortData{
		port1IPV4:     "",
		port2IPV4:     "",
		aggPortID1:    0,
		aggPortID2:    0,
		dutPort1IPv4:  "192.0.2.1",
		dutPort2IPv4:  "192.0.2.5",
		dutPort3IPv4:  "192.0.2.9",
		dutPort4IPv4:  "192.0.2.13",
		dutAgg1Name:   "lag3",
		dutAgg2Name:   "lag4",
		aggPortIDDUT1: 10,
		aggPortIDDUT2: 11,
		aggPortIDDUT3: 12,
		aggPortIDDUT4: 13,
	}
)

func TestCapabilitiesResponse(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut")
	gnoiClient1 := dut1.RawAPIs().GNOI(t)
	plqResp, err := gnoiClient1.LinkQualification().Capabilities(context.Background(), &plqpb.CapabilitiesRequest{})

	t.Logf("LinkQualification().Capabilities(): %v, err: %v", plqResp, err)
	if err != nil {
		t.Fatalf("Failed to handle gnoi LinkQualification().Capabilities(): %v", err)
	}

	if deviations.PLQGeneratorCapabilitiesMaxMTU(dut1) != 0 {
		minRequiredGeneratorMTU = uint64(deviations.PLQGeneratorCapabilitiesMaxMTU(dut1))
	}

	if deviations.PLQGeneratorCapabilitiesMaxPPS(dut1) != 0 {
		minRequiredGeneratorPPS = deviations.PLQGeneratorCapabilitiesMaxPPS(dut1)
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
		min:  minRequiredGeneratorMTU,
	}, {
		desc: "Generator MaxBps",
		got:  uint64(plqResp.GetGenerator().GetPacketGenerator().GetMaxBps()),
		min:  uint64(1e11),
	}, {
		desc: "Generator MaxPps",
		got:  uint64(plqResp.GetGenerator().GetPacketGenerator().GetMaxPps()),
		min:  minRequiredGeneratorPPS,
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
		if pmdLB := ref.GetPmdLoopback(); pmdLB.GetMinSetupDuration().GetSeconds() >= 1 && pmdLB.GetMinTeardownDuration().GetSeconds() >= 1 {
			t.Logf("Device supports PMD loopback reflector mode")

		} else if asicLB := ref.GetAsicLoopback(); asicLB.GetMinSetupDuration().GetSeconds() >= 1 && asicLB.GetMinTeardownDuration().GetSeconds() >= 1 {
			t.Logf("Device supports ASIC loopback reflector mode")
		} else {
			t.Errorf("Reflector MinSetupDuration or MinTeardownDuration is not >=1 for supported mode. Device reflector capabilities: %v", plqResp.GetReflector())
		}
	})
}

func TestNonexistingID(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut")
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
	dut1 := ondatra.DUT(t, "dut")
	dut2 := ondatra.DUT(t, "dut")
	gnoiClient1 := dut1.RawAPIs().GNOI(t)
	gnoiClient2 := dut2.RawAPIs().GNOI(t)

	clients := []gnoigo.Clients{gnoiClient1, gnoiClient2}
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

func calculatePLQDurations(t *testing.T, generatorPlqResp *plqpb.CapabilitiesResponse, reflectorPlqResp *plqpb.CapabilitiesResponse, dut *ondatra.DUTDevice) *LinkQualificationDuration {

	genPblqMinSetup := float64(generatorPlqResp.GetGenerator().GetPacketGenerator().GetMinSetupDuration().GetSeconds())
	refPblqMinSetup := float64(reflectorPlqResp.GetGenerator().GetPacketGenerator().GetMinSetupDuration().GetSeconds())
	genPblqMinTearDown := float64(generatorPlqResp.GetGenerator().GetPacketGenerator().GetMinTeardownDuration().GetSeconds())
	refPblqMinTearDown := float64(reflectorPlqResp.GetGenerator().GetPacketGenerator().GetMinTeardownDuration().GetSeconds())

	return &LinkQualificationDuration{
		generatorpreSyncDuration:  30 * time.Second,
		reflectorpreSyncDuration:  0 * time.Second,
		generatorsetupDuration:    time.Duration(math.Max(30, genPblqMinSetup)) * time.Second,
		reflectorsetupDuration:    time.Duration(math.Max(60, refPblqMinSetup)) * time.Second,
		testDuration:              120 * time.Second,
		generatorPostSyncDuration: 5 * time.Second,
		reflectorPostSyncDuration: 10 * time.Second,
		generatorTeardownDuration: time.Duration(math.Max(30, genPblqMinTearDown)) * time.Second,
		reflectorTeardownDuration: time.Duration(math.Max(30, refPblqMinTearDown)) * time.Second,
	}
}

// configures DUT port1 lag1ID <-----> lagID DUT port 2 with 1 member link.
func configureDUTAggregate(t *testing.T, dut *ondatra.DUTDevice, dp1 *ondatra.Port, dp2 *ondatra.Port, speed string) {
	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	var aggID1 string
	var aggID2 string

	if speed == "400g" {
		agg1.port1IPV4 = agg1.dutPort1IPv4
		agg1.port2IPV4 = agg1.dutPort2IPv4
		agg1.aggPortID1 = agg1.aggPortIDDUT1
		agg1.aggPortID2 = agg1.aggPortIDDUT2
	} else {
		agg1.port1IPV4 = agg1.dutPort3IPv4
		agg1.port2IPV4 = agg1.dutPort4IPv4
		agg1.aggPortID1 = agg1.aggPortIDDUT3
		agg1.aggPortID2 = agg1.aggPortIDDUT4
	}

	for _, dp := range []*ondatra.Port{dp1, dp2} {
		b := &gnmi.SetBatch{}
		d := &oc.Root{}
		aggID := netutil.NextAggregateInterface(t, dut)

		agg := d.GetOrCreateInterface(aggID)
		agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC
		agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
		agg.Description = ygot.String(agg1.dutAgg1Name)

		if !deviations.OmitL2MTU(dut) {
			agg.Mtu = ygot.Uint16(9000)
		}
		if deviations.InterfaceEnabled(dut) {
			agg.Enabled = ygot.Bool(true)
		}

		s := agg.GetOrCreateSubinterface(0)
		s4 := s.GetOrCreateIpv4()
		if deviations.InterfaceEnabled(dut) {
			s4.Enabled = ygot.Bool(true)
		}
		var a4 *oc.Interface_Subinterface_Ipv4_Address
		if dp == dp1 {
			a4 = s4.GetOrCreateAddress(agg1.port1IPV4)
			aggID1 = aggID
		} else {
			a4 = s4.GetOrCreateAddress(agg1.port2IPV4)
			aggID2 = aggID
		}
		a4.PrefixLength = ygot.Uint8(ipv4PLen)

		gnmi.BatchDelete(b, gnmi.OC().Interface(aggID).Aggregation().MinLinks().Config())
		gnmi.BatchReplace(b, gnmi.OC().Interface(aggID).Config(), agg)

		gnmi.BatchDelete(b, gnmi.OC().Interface(dp.Name()).Ethernet().AggregateId().Config())
		i := d.GetOrCreateInterface(dp.Name())
		if dp == dp1 {
			i.Id = ygot.Uint32(agg1.aggPortID1)
		} else {
			i.Id = ygot.Uint32(agg1.aggPortID2)
		}
		i.Description = ygot.String(fmt.Sprintf("LAG - Member -%s", dp.Name()))
		e := i.GetOrCreateEthernet()
		e.AggregateId = ygot.String(aggID)
		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
		gnmi.BatchReplace(b, gnmi.OC().Interface(dp.Name()).Config(), i)

		b.Set(t, dut)

		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, aggID, deviations.DefaultNetworkInstance(dut), 0)
		}
	}

	// Wait for LAG interfaces to be UP
	gnmi.Await(t, dut, gnmi.OC().Interface(aggID1).OperStatus().State(), 60*time.Second, oc.Interface_OperStatus_UP)
	gnmi.Await(t, dut, gnmi.OC().Interface(aggID2).OperStatus().State(), 60*time.Second, oc.Interface_OperStatus_UP)
}

func testLinkQualification(t *testing.T, dut *ondatra.DUTDevice, dp1 *ondatra.Port, dp2 *ondatra.Port, plqID string, aggregate bool) {
	if deviations.PLQGeneratorCapabilitiesMaxMTU(dut) != 0 {
		minRequiredGeneratorMTU = uint64(deviations.PLQGeneratorCapabilitiesMaxMTU(dut))
	}

	gnoiClient := dut.RawAPIs().GNOI(t)
	capabilities, err := gnoiClient.LinkQualification().Capabilities(context.Background(), &plqpb.CapabilitiesRequest{})
	if err != nil {
		t.Logf("Failed to handle gnoi LinkQualification().Capabilities(): %v", err)
	}

	generatorPlqResp, err := gnoiClient.LinkQualification().Capabilities(context.Background(), &plqpb.CapabilitiesRequest{})
	t.Logf("LinkQualification().Capabilities(): %v, err: %v", generatorPlqResp, err)
	if err != nil {
		t.Fatalf("Failed to handle gnoi LinkQualification().Capabilities(): %v", err)
	}

	reflectorPlqResp, err := gnoiClient.LinkQualification().Capabilities(context.Background(), &plqpb.CapabilitiesRequest{})
	t.Logf("LinkQualification().Capabilities(): %v, err: %v", reflectorPlqResp, err)
	if err != nil {
		t.Fatalf("Failed to handle gnoi LinkQualification().Capabilities(): %v", err)
	}

	// Create calculatePLQDuration function to get the duration of the PLQ test.
	plqDuration := calculatePLQDurations(t, generatorPlqResp, reflectorPlqResp, dut)

	// Create unique IDs for generator and reflector.
	generatorPLQID := plqID + "-generator"
	reflectorPLQID := plqID + "-reflector"

	generatorCreateRequest := &plqpb.CreateRequest{
		Interfaces: []*plqpb.QualificationConfiguration{
			{
				Id:            generatorPLQID,
				InterfaceName: dp1.Name(),
				EndpointType: &plqpb.QualificationConfiguration_PacketGenerator{
					PacketGenerator: &plqpb.PacketGeneratorConfiguration{
						PacketRate: uint64(138888),
						PacketSize: uint32(minRequiredGeneratorMTU),
					},
				},
				Timing: &plqpb.QualificationConfiguration_Rpc{
					Rpc: &plqpb.RPCSyncedTiming{
						Duration:         durationpb.New(plqDuration.testDuration),
						PreSyncDuration:  durationpb.New(plqDuration.generatorpreSyncDuration),
						SetupDuration:    durationpb.New(plqDuration.generatorsetupDuration),
						PostSyncDuration: durationpb.New(plqDuration.generatorPostSyncDuration),
						TeardownDuration: durationpb.New(plqDuration.generatorTeardownDuration),
					},
				},
			},
		},
	}
	t.Logf("generatorCreateRequest: %v", generatorCreateRequest)

	intf := &plqpb.QualificationConfiguration{
		Id:            reflectorPLQID,
		InterfaceName: dp2.Name(),

		Timing: &plqpb.QualificationConfiguration_Rpc{
			Rpc: &plqpb.RPCSyncedTiming{
				Duration:         durationpb.New(plqDuration.testDuration),
				PreSyncDuration:  durationpb.New(plqDuration.reflectorpreSyncDuration),
				SetupDuration:    durationpb.New(plqDuration.reflectorsetupDuration),
				PostSyncDuration: durationpb.New(plqDuration.reflectorPostSyncDuration),
				TeardownDuration: durationpb.New(plqDuration.reflectorTeardownDuration),
			},
		},
	}

	asicLoopbackSupported := capabilities.GetReflector().GetAsicLoopback().GetMinSetupDuration().AsDuration() > 0
	pmdLoopbackSupported := capabilities.GetReflector().GetPmdLoopback().GetMinSetupDuration().AsDuration() > 0

	if asicLoopbackSupported {
		intf.EndpointType = &plqpb.QualificationConfiguration_AsicLoopback{
			AsicLoopback: &plqpb.AsicLoopbackConfiguration{},
		}
	} else if pmdLoopbackSupported {
		intf.EndpointType = &plqpb.QualificationConfiguration_PmdLoopback{
			PmdLoopback: &plqpb.PmdLoopbackConfiguration{},
		}
	} else {
		// Handle case where neither loopback mode is supported
		t.Fatalf("Neither ASIC nor PMD loopback is supported by the DUT.")
	}

	generatorCreateRequest.Interfaces = append(generatorCreateRequest.Interfaces, intf)

	generatorCreateResp, err := gnoiClient.LinkQualification().Create(context.Background(), generatorCreateRequest)
	t.Logf("LinkQualification().Create() generatorCreateResp: %v, err: %v", generatorCreateResp, err)
	if err != nil {
		t.Fatalf("Failed to handle generator LinkQualification().Create(): %v", err)
	}

	// Check for errors on both created interfaces.
	if status, ok := generatorCreateResp.GetStatus()[generatorPLQID]; ok {
		if got, want := status.GetCode(), int32(0); got != want {
			t.Errorf("generatorCreateResp (generator): got %v, want %v", got, want)
		}
	} else {
		t.Fatalf("generatorCreateResp missing status for generator ID: %v", generatorPLQID)
	}

	if status, ok := generatorCreateResp.GetStatus()[reflectorPLQID]; ok {
		if got, want := status.GetCode(), int32(0); got != want {
			t.Errorf("reflectorCreateResp: got %v, want %v", got, want)
		}
	} else {
		t.Fatalf("reflectorCreateResp missing status for reflector ID: %v", reflectorPLQID)
	}

	listResp, err := gnoiClient.LinkQualification().List(context.Background(), &plqpb.ListRequest{})
	if err != nil {
		t.Fatalf("Failed to handle gnoi LinkQualification().List(): %v", err)
	}

	var discoveredIDs []string
	for _, result := range listResp.GetResults() {
		// Check if the result's interface name matches either dp1 or dp2.
		if result.GetInterfaceName() == dp1.Name() || result.GetInterfaceName() == dp2.Name() {
			discoveredIDs = append(discoveredIDs, result.GetId())
			t.Logf("Discovered Link Qualification ID: %v", result.GetId())
		}
	}

	if len(discoveredIDs) == 0 {
		t.Fatalf("Could not discover a Link Qualification ID after Create.  List response: %v", listResp)
	}
	sleepTime := 30 * time.Second
	minTestTime := plqDuration.testDuration + plqDuration.reflectorPostSyncDuration + plqDuration.generatorpreSyncDuration + plqDuration.generatorsetupDuration + plqDuration.generatorTeardownDuration
	counter := int(minTestTime.Seconds())/int(sleepTime.Seconds()) + 2
	for i := 0; i <= counter; i++ {
		t.Logf("Wait for %v seconds: %d/%d", sleepTime.Seconds(), i+1, counter)
		time.Sleep(sleepTime)
		testDone := true
		t.Logf("Check client")

		for j := 0; j < len(listResp.GetResults()); j++ {
			if listResp.GetResults()[j].GetState() != plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED {
				t.Logf("LinkQualification in progress, current state: %v", listResp.GetResults()[j].GetState())
				testDone = false
			} else {
				t.Logf("LinkQualification completed: %v", listResp.GetResults()[j])
			}

			if !deviations.SkipPlqInterfaceOperStatusCheck(dut) {
				if listResp.GetResults()[j].GetState() == plqpb.QualificationState_QUALIFICATION_STATE_RUNNING {
					t.Logf("Checking link under qualificaton (generator) interface oper-status (dut: %v, dp: %v)", dut.Name(), dp1.Name())
					if got, want := gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State()), oc.Interface_OperStatus_TESTING; got != want {
						t.Errorf("Interface(%v) oper-status: got %v, want %v", dp1.Name(), got, want)
					}

					t.Logf("Checking link under qualificaton (reflector) interface oper-status (dut: %v, dp: %v)", dut.Name(), dp2.Name())
					if got, want := gnmi.Get(t, dut, gnmi.OC().Interface(dp2.Name()).OperStatus().State()), oc.Interface_OperStatus_TESTING; got != want {
						t.Errorf("Interface(%v) oper-status: got %v, want %v", dp2.Name(), got, want)
					}

				}
			}
		}

		if testDone {
			t.Logf("Detected QualificationState_QUALIFICATION_STATE_COMPLETED.")
			break
		}
	}

	getRequest := &plqpb.GetRequest{
		Ids: discoveredIDs,
	}

	t.Logf("Check client")
	getResp, err := gnoiClient.LinkQualification().Get(context.Background(), getRequest)
	t.Logf("LinkQualification().Get(): %v, err: %v", getResp, err)
	if err != nil {
		t.Fatalf("Failed to handle LinkQualification().Get(): %v", err)
	}

	// The packet counters between Generator and Reflector mismatch tolerance level in percentage
	tolerance := 0.0001

	for _, result := range getResp.GetResults() {
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

		generatorPktsSent := result.GetPacketsSent()
		generatorPktsRxed := result.GetPacketsReceived()
		reflectorPktsSent := result.GetPacketsSent()
		reflectorPktsRxed := result.GetPacketsReceived()

		if deviations.PLQReflectorStatsUnsupported(dut) {
			if (math.Abs(float64(generatorPktsSent)-float64(generatorPktsRxed))/float64(generatorPktsSent))*100.00 > tolerance {
				t.Errorf("The difference between packets sent count and packets received count at Generator is greater than %0.4f percent: generatorPktsSent %v, generatorPktsRxed %v", tolerance, generatorPktsSent, generatorPktsRxed)
			}
		} else {
			if ((math.Abs(float64(generatorPktsSent)-float64(reflectorPktsRxed)))/(float64(generatorPktsSent)+float64(reflectorPktsRxed)+tolerance))*200.00 > tolerance {
				t.Errorf("The difference between packets received count at Reflector and packets sent count at Generator is greater than %0.4f percent: generatorPktsSent %v, reflectorPktsRxed %v", tolerance, generatorPktsSent, reflectorPktsRxed)
			}
			if ((math.Abs(float64(reflectorPktsSent)-float64(generatorPktsRxed)))/(float64(reflectorPktsSent)+float64(generatorPktsRxed)+tolerance))*200.00 > tolerance {
				t.Errorf("The difference between packets received count at Generator and packets sent count at Reflector is greater than %0.4f percent: reflectorPktsSent %v, generatorPktsRxed %v", tolerance, reflectorPktsSent, generatorPktsRxed)
			}
		}
	}
}

func TestLinkQualification(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	port1 := dut.Port(t, "port1")
	port2 := dut.Port(t, "port2")
	port3 := dut.Port(t, "port3")
	port4 := dut.Port(t, "port4")
	t.Logf("dut: %v", dut.Name())
	t.Logf("PLQ will run on 400g links on dut port1 name: %v, dut port2 name : %v", port1.Name(), port2.Name())
	t.Logf("PLQ will run on 100g links on dut port3 name: %v, dut port4 name : %v", port3.Name(), port4.Name())

	d := gnmi.OC()

	for _, p := range []string{"port1", "port2", "port3", "port4"} {
		port := dut.Port(t, p)
		i := &oc.Interface{Name: ygot.String(port.Name())}
		gnmi.Replace(t, dut, d.Interface(port.Name()).Config(), configInterfaceMTU(i, dut))
		if deviations.ExplicitPortSpeed(dut) {
			fptest.SetPortSpeed(t, port)
		}
	}

	cases := []struct {
		desc      string
		plqID     string
		testFunc  func(t *testing.T, dut *ondatra.DUTDevice, dp1 *ondatra.Port, dp2 *ondatra.Port, plqID string, aggregate bool)
		aggregate bool
		speed     string
	}{{
		desc:      "Singleton Interface LinkQualification on 400g links",
		plqID:     fmt.Sprintf("%s:%s-%s:%s", dut.Name(), port1.Name(), dut.Name(), port2.Name()),
		testFunc:  testLinkQualification,
		aggregate: false,
		speed:     "400g",
	}, {
		desc:      "Member Link LinkQualification on 400g links",
		plqID:     fmt.Sprintf("%s:%s-%s:%s-aggregate", dut.Name(), port1.Name(), dut.Name(), port2.Name()),
		testFunc:  testLinkQualification,
		aggregate: true,
		speed:     "400g",
	}, {
		desc:      "Singleton Interface LinkQualification on 100g links",
		plqID:     fmt.Sprintf("%s:%s-%s:%s", dut.Name(), port3.Name(), dut.Name(), port4.Name()),
		testFunc:  testLinkQualification,
		aggregate: false,
		speed:     "100g",
	}, {
		desc:      "Member Link LinkQualification on 100g links",
		plqID:     fmt.Sprintf("%s:%s-%s:%s-aggregate", dut.Name(), port3.Name(), dut.Name(), port4.Name()),
		testFunc:  testLinkQualification,
		aggregate: true,
		speed:     "100g",
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.speed == "400g" {
				if tc.aggregate {
					configureDUTAggregate(t, dut, port1, port2, tc.speed)
				}
				tc.testFunc(t, dut, port1, port2, tc.plqID, tc.aggregate)
			} else if tc.speed == "100g" {
				if tc.aggregate {
					configureDUTAggregate(t, dut, port3, port4, tc.speed)
				}
				tc.testFunc(t, dut, port3, port4, tc.plqID, tc.aggregate)
			}
		})
	}
}
