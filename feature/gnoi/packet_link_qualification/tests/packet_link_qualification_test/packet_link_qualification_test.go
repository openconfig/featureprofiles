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
//   dut1:port1 <--> port1:dut2 (port1 as singleton and memberlink)
//
// Test notes:
//
//  - gnoi operation commands can be sent and tested using CLI command grpcurl.
//    https://github.com/fullstorydev/grpcurl
//

type aggPortData struct {
	dut1IPv4      string
	dut2IPv4      string
	ateAggName    string
	aggPortIDDUT1 uint32
	aggPortIDDUT2 uint32
}

const (
	ipv4PLen = 30
)

var (
	minRequiredGeneratorMTU = uint64(8184)
	minRequiredGeneratorPPS = uint64(1e8)
	agg1                    = &aggPortData{
		dut1IPv4:      "192.0.2.1",
		dut2IPv4:      "192.0.2.2",
		ateAggName:    "lag3",
		aggPortIDDUT1: 10,
		aggPortIDDUT2: 11,
	}
)

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

// configures DUT1 lagID <-----> lagID DUT2 with 1 member link.
func configureDUTAggregate(t *testing.T, dut1 *ondatra.DUTDevice, dut2 *ondatra.DUTDevice) {
	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut1)
	fptest.ConfigureDefaultNetworkInstance(t, dut2)
	var aggIdDut1 string
	var aggIdDut2 string

	for _, dut := range []*ondatra.DUTDevice{dut1, dut2} {
		b := &gnmi.SetBatch{}
		d := &oc.Root{}
		aggID := netutil.NextAggregateInterface(t, dut)

		agg := d.GetOrCreateInterface(aggID)
		agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC
		agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
		agg.Description = ygot.String(agg1.ateAggName)
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
		if dut == dut1 {
			a4 = s4.GetOrCreateAddress(agg1.dut1IPv4)
			aggIdDut1 = aggID
		} else {
			a4 = s4.GetOrCreateAddress(agg1.dut2IPv4)
			aggIdDut2 = aggID
		}
		a4.PrefixLength = ygot.Uint8(ipv4PLen)

		gnmi.BatchDelete(b, gnmi.OC().Interface(aggID).Aggregation().MinLinks().Config())
		gnmi.BatchReplace(b, gnmi.OC().Interface(aggID).Config(), agg)

		p1 := dut.Port(t, "port1")
		for _, port := range []*ondatra.Port{p1} {
			gnmi.BatchDelete(b, gnmi.OC().Interface(port.Name()).Ethernet().AggregateId().Config())
			i := d.GetOrCreateInterface(port.Name())
			if dut == dut1 {
				i.Id = ygot.Uint32(agg1.aggPortIDDUT1)
			} else {
				i.Id = ygot.Uint32(agg1.aggPortIDDUT2)
			}
			i.Description = ygot.String(fmt.Sprintf("LAG - Member -%s", port.Name()))
			e := i.GetOrCreateEthernet()
			e.AggregateId = ygot.String(aggID)
			i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

			if deviations.InterfaceEnabled(dut) {
				i.Enabled = ygot.Bool(true)
			}
			gnmi.BatchReplace(b, gnmi.OC().Interface(port.Name()).Config(), i)
		}

		b.Set(t, dut)

		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, aggID, deviations.DefaultNetworkInstance(dut), 0)
		}
	}
	// Wait for LAG interfaces to be UP
	gnmi.Await(t, dut1, gnmi.OC().Interface(aggIdDut1).OperStatus().State(), 60*time.Second, oc.Interface_OperStatus_UP)

	gnmi.Await(t, dut2, gnmi.OC().Interface(aggIdDut2).OperStatus().State(), 60*time.Second, oc.Interface_OperStatus_UP)
}

func testLinkQualification(t *testing.T, dut1 *ondatra.DUTDevice, dut2 *ondatra.DUTDevice, dp1 *ondatra.Port, dp2 *ondatra.Port, plqID string) {

	if deviations.PLQGeneratorCapabilitiesMaxMTU(dut1) != 0 {
		minRequiredGeneratorMTU = uint64(deviations.PLQGeneratorCapabilitiesMaxMTU(dut1))
	}
	type LinkQualificationDuration struct {
		// time needed to complete preparation
		generatorsetupDuration time.Duration
		reflectorsetupDuration time.Duration
		// time duration to wait before starting link qual preparation
		generatorpreSyncDuration time.Duration
		reflectorpreSyncDuration time.Duration
		// packet linkqual duration
		testDuration time.Duration
		// time to wait post link-qual before starting teardown
		generatorPostSyncDuration time.Duration
		reflectorPostSyncDuration time.Duration
		// time required to bring the interface back to pre-test state
		tearDownDuration time.Duration
                generatorTearDownDuration time.Duration
                reflectorTearDownDuration time.Duration
	}

        gnoiClient1 := dut1.RawAPIs().GNOI(t)
        gnoiClient2 := dut2.RawAPIs().GNOI(t)
        generatorPlqResp, err := gnoiClient1.LinkQualification().Capabilities(context.Background(), &plqpb.CapabilitiesRequest{})
        reflectorPlqResp, err := gnoiClient1.LinkQualification().Capabilities(context.Background(), &plqpb.CapabilitiesRequest{})

        t.Logf("LinkQualification().Capabilities(): %v, err: %v", generatorPlqResp, err)
        if err != nil {
                t.Fatalf("Failed to handle gnoi LinkQualification().Capabilities(): %v", err)
        }
        t.Logf("LinkQualification().Capabilities(): %v, err: %v", reflectorPlqResp, err)
        if err != nil {
                t.Fatalf("Failed to handle gnoi LinkQualification().Capabilities(): %v", err)
        }

        genPblqMinSetup := float64(generatorPlqResp.GetGenerator().GetPacketGenerator().GetMinSetupDuration().GetSeconds())
        refPblqMinSetup := float64(reflectorPlqResp.GetGenerator().GetPacketGenerator().GetMinSetupDuration().GetSeconds())
        genPblqMinTearDown := float64(generatorPlqResp.GetGenerator().GetPacketGenerator().GetMinTeardownDuration().GetSeconds())
        refPblqMinTearDown := float64(reflectorPlqResp.GetGenerator().GetPacketGenerator().GetMinTeardownDuration().GetSeconds())

	plqDuration := &LinkQualificationDuration{
		generatorpreSyncDuration:  30 * time.Second,
		reflectorpreSyncDuration:  0 * time.Second,
                generatorsetupDuration:    time.Duration(math.Max(30, genPblqMinSetup)) * time.Second,
                reflectorsetupDuration:    time.Duration(math.Max(60, refPblqMinSetup)) * time.Second,
		testDuration:              120 * time.Second,
		generatorPostSyncDuration: 5 * time.Second,
		reflectorPostSyncDuration: 10 * time.Second,
                generatorTearDownDuration: time.Duration(math.Max(30, genPblqMinTearDown)) * time.Second,
                reflectorTearDownDuration: time.Duration(math.Max(30, refPblqMinTearDown)) * time.Second,
	}

	generatorCreateRequest := &plqpb.CreateRequest{
		Interfaces: []*plqpb.QualificationConfiguration{
			{
				Id:            plqID,
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
						TeardownDuration: durationpb.New(plqDuration.generatorTearDownDuration),
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
				PreSyncDuration:  durationpb.New(plqDuration.reflectorpreSyncDuration),
				SetupDuration:    durationpb.New(plqDuration.reflectorsetupDuration),
				PostSyncDuration: durationpb.New(plqDuration.reflectorPostSyncDuration),
				TeardownDuration: durationpb.New(plqDuration.reflectorTearDownDuration),
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

	gnoiClient1 = dut1.RawAPIs().GNOI(t)
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
        minTearDownDuration := time.Duration(math.Max(float64(plqDuration.generatorTearDownDuration), float64(plqDuration.reflectorTearDownDuration)))
        minTestTime := plqDuration.testDuration + plqDuration.reflectorPostSyncDuration + plqDuration.generatorpreSyncDuration + plqDuration.generatorsetupDuration +  minTearDownDuration
	counter := int(minTestTime.Seconds())/int(sleepTime.Seconds()) + 2
	for i := 0; i <= counter; i++ {
		t.Logf("Wait for %v seconds: %d/%d", sleepTime.Seconds(), i+1, counter)
		time.Sleep(sleepTime)
		testDone := true
		for i, client := range []gnoigo.Clients{gnoiClient1, gnoiClient2} {
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
				if !deviations.SkipPlqInterfaceOperStatusCheck(dut1) {
					if listResp.GetResults()[j].GetState() == plqpb.QualificationState_QUALIFICATION_STATE_RUNNING {
						if client == gnoiClient1 {
							t.Logf("Checking link under qualificaton (generator) interface oper-status (dut: %v, dp: %v)", dut1.Name(), dp1.Name())
							if got, want := gnmi.Get(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State()), oc.Interface_OperStatus_TESTING; got != want {
								t.Errorf("Interface(%v) oper-status: got %v, want %v", dp1.Name(), got, oc.Interface_OperStatus_TESTING)
							}
						} else if client == gnoiClient2 {
							t.Logf("Checking link under qualificaton (reflector) interface oper-status (dut: %v, dp: %v)", dut2.Name(), dp2.Name())
							if got, want := gnmi.Get(t, dut2, gnmi.OC().Interface(dp2.Name()).OperStatus().State()), oc.Interface_OperStatus_TESTING; got != want {
								t.Errorf("Interface(%v) oper-status: got %v, want %v", dp2.Name(), got, oc.Interface_OperStatus_TESTING)
							}
						}
					}
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

	for i, client := range []gnoigo.Clients{gnoiClient1, gnoiClient2} {
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

	if deviations.PLQReflectorStatsUnsupported(dut1) {
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

	cases := []struct {
		desc      string
		plqID     string
		testFunc  func(t *testing.T, dut1 *ondatra.DUTDevice, dut2 *ondatra.DUTDevice, dp1 *ondatra.Port, dp2 *ondatra.Port, plqID string)
		aggregate bool
	}{{
		desc:      "Singleton Interface LinkQualification",
		plqID:     dut1.Name() + ":" + dp1.Name() + "<->" + dut2.Name() + ":" + dp2.Name() + ":singleton",
		testFunc:  testLinkQualification,
		aggregate: false,
	}, {
		desc:      "Member Link LinkQualification",
		plqID:     dut1.Name() + ":" + dp1.Name() + "<->" + dut2.Name() + ":" + dp2.Name() + ":memberlink",
		testFunc:  testLinkQualification,
		aggregate: true,
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.aggregate {
				configureDUTAggregate(t, dut1, dut2)
			}
			tc.testFunc(t, dut1, dut2, dp1, dp2, tc.plqID)
		})
	}
}
