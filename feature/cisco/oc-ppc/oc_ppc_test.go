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

package oc_ppc_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/ha/monitor"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/ondatra"
)

// constant variables
const (
	vrf1 = "TE"
	traffic = true
)

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx     context.Context
	client  *gribi.Client
	dut     *ondatra.DUTDevice
	ate     *ondatra.ATEDevice
	top     *ondatra.ATETopology
	events  *monitor.CachedConsumer
	ATELock sync.Mutex
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func (a *testArgs) testOC_PPC_interfae_subsystem(t *testing.T){
	t.Skip("skipping interface subsystem")
} 

func (a *testArgs)testOC_PPC_queuing_subsystem(t *testing.T){
	type testCase struct {
		desc     string
		path     string
		counters []*ygnmi.Value[uint64]
	}
	interfaces := sortedInterfaces(a.dut.Ports())
	t.Logf("Interfaces: %s", interfaces)
	for _, intf := range interfaces {
		t.Run(intf, func(t *testing.T) {
			qosInterface := gnmi.OC().Qos().Interface(intf)
			cases := []testCase{
				{
					desc:     "Queue Input Dropped packets",
					path:     "/qos/interfaces/interface/input/queues/queue/state/dropped-pkts",
					counters: gnmi.LookupAll(t, a.dut, qosInterface.Input().QueueAny().DroppedPkts().State()),
				},
				{
					desc:     "Queue Output Dropped packets",
					path:     "/qos/interfaces/interface/output/queues/queue/state/dropped-pkts",
					counters: gnmi.LookupAll(t, a.dut, qosInterface.Output().QueueAny().DroppedPkts().State()),
				},
				{
					desc:     "Queue input voq-output-interface dropped packets",
					path:     "/qos/interfaces/interface/input/virtual-output-queues/voq-interface/queues/queue/state/dropped-pkts",
					counters: gnmi.LookupAll(t, a.dut, qosInterface.Input().VoqInterfaceAny().QueueAny().DroppedPkts().State()),
				},
			}
			for _, c := range cases {
				t.Run(c.desc, func(t *testing.T) {
					if len(c.counters) == 0 {
						t.Errorf("%s Interface %s Telemetry Value is not present", c.desc, intf)
					}
					for queueID, dropPkt := range c.counters {
						dropCount, present := dropPkt.Val()
						if !present {
							t.Errorf("%s Interface %s %s Telemetry Value is not present", c.desc, intf, dropPkt.Path)
						} else {
							t.Logf("%s Interface %s, Queue %d has %d drop(s)", dropPkt.Path.GetOrigin(), intf, queueID, dropCount)
						}
					}
				})
			}
		})
	}
} 

func (a *testArgs) testOC_PPC_lookup_subsystem(t *testing.T){
	t.Skip("skipping lookup subsystem")
} 

func (a *testArgs) testOC_PPC_host_subsystem(t *testing.T){
	t.Skip("skipping host subsystem")
}	

func (a *testArgs) testOC_PPC_fabric_subsystem(t *testing.T){
	t.Skip("skipping fabric subsystem")
}

func TestOC_PPC(t *testing.T) {
	t.Log("Name: OC PPC")

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()

	// Configure the DUT
	var vrfs = []string{vrf1}
	configVRF(t, dut, vrfs)
	configureDUT(t, dut)
	// PBR config
	// configbasePBR(t, dut, "REPAIRED", "ipv4", 1, "pbr", oc.PacketMatchTypes_IP_PROTOCOL_UNSET, []uint8{}, &PBROptions{SrcIP: "222.222.222.222/32"})
	// configbasePBR(t, dut, "TE", "ipv4", 2, "pbr", oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})
	// configbasePBRInt(t, dut, "Bundle-Ether120", "pbr")
	// RoutePolicy config
	configRP(t, dut)
	// configure ISIS on DUT
	addISISOC(t, dut, "Bundle-Ether127")
	// configure BGP on DUT
	addBGPOC(t, dut, "100.100.100.100")

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	if traffic {
		addPrototoAte(t, top)
		time.Sleep(120 * time.Second)
	}

	//Creating gribi client for run across 1 subsystem
	clientA := gribi.Client{
		DUT:         ondatra.DUT(t, "dut"),
		FIBACK:      true,
		Persistence: true,
		InitialElectionIDLow:  1,
		InitialElectionIDHigh: 0,
	}}
	defer clientA.Close(t)
	if err := clientA.Start(t); err != nil {
		t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
		if err = clientA.Start(t); err != nil {
			t.Fatalf("gRIBI Connection could not be established: %v", err)
		}
	}
	clientA.BecomeLeader(t)

	args := &testArgs{
		ctx:        ctx,
		clientA:    &clientA,
		dut:        dut,
		ate:        ate,
		top:        top,
		},
	}

	t.Run("Test interface subsystem", func(t *testing.T) {
		args.testOC_PPC_interfae_subsystem(t)
	})
	t.Run("Test queuing subsystem", func(t *testing.T) {
		args.testOC_PPC_queuing_subsystem(t)
	})
	t.Run("Test lookup subsystem", func(t *testing.T) {
		args.testOC_PPC_lookup_subsystem(t)
	})
	t.Run("Test host subsystem", func(t *testing.T) {
		args.testOC_PPC_host_subsystem(t)
	})
	t.Run("Test fabrc subsystem", func(t *testing.T) {
		args.testOC_PPC_fabric_subsystem(t)
	})
}
