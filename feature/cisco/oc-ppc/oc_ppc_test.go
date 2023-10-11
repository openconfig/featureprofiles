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

func (a *testArgs) testOC_PPC_interface_subsystem(t *testing.T){
	interfaces := sortedInterfaces(a.dut.Ports())
	t.Logf("Interfaces: %s", interfaces)
	for _, intf := range interfaces {
		t.Run(intf, func(t *testing.T) {
			query := gnmi.OC().Interface(intf).State()
			root := gnmi.Get(t, a.dut, query)
			t.Logf("INFO: Interface %s: \n", intf)
			if root.OperStatus == oc.Interface_OperStatus_NOT_PRESENT {
				t.Errorf("ERROR: Oper status is not present")
			} else {
				t.Logf("INFO: Oper status: %s", root.GetOperStatus())
			}
			if root.AdminStatus == oc.Interface_AdminStatus_UNSET {
				t.Errorf("ERROR: Admin status is not set")
			} else {
				t.Logf("INFO: Admin status: %s", root.GetAdminStatus())
			}
			if root.Type == oc.IETFInterfaces_InterfaceType_UNSET {
				t.Errorf("ERROR: Type is not present")
			} else {
				t.Logf("INFO: Type: %s", root.GetType())
			}
			if root.Description == nil {
				t.Errorf("ERROR: Description is not present")
			} else {
				t.Logf("INFO: Description: %s", root.GetType())
			}
			if root.GetCounters().OutOctets == nil {
				t.Errorf("ERROR: Counter OutOctets is not present")
			} else {
				t.Logf("INFO: Counter OutOctets: %d", root.GetCounters().GetOutOctets())
			}
			if root.GetCounters().InMulticastPkts == nil {
				t.Errorf("ERROR: Counter InMulticastPkts is not present")
			} else {
				t.Logf("INFO: Counter InMulticastPkts: %d", root.GetCounters().GetInMulticastPkts())
			}
			if root.GetCounters().InDiscards == nil {
				t.Errorf("ERROR: Counter InDiscards is not present")
			} else {
				t.Logf("INFO: Counter InDiscards: %d", root.GetCounters().GetInDiscards())
			}
			if root.GetCounters().InErrors == nil {
				t.Errorf("ERROR: Counter InErrors is not present")
			} else {
				t.Logf("INFO: Counter InErrors: %d", root.GetCounters().GetInErrors())
			}
			if root.GetCounters().InUnknownProtos == nil {
				t.Errorf("ERROR: Counter InUnknownProtos is not present")
			} else {
				t.Logf("INFO: Counter InUnknownProtos: %d", root.GetCounters().GetInUnknownProtos())
			}
			if root.GetCounters().OutDiscards == nil {
				t.Errorf("ERROR: Counter OutDiscards is not present")
			} else {
				t.Logf("INFO: Counter OutDiscards: %d", root.GetCounters().GetOutDiscards())
			}
			if root.GetCounters().OutErrors == nil {
				t.Errorf("ERROR: Counter OutErrors is not present")
			} else {
				t.Logf("INFO: Counter OutErrors: %d", root.GetCounters().GetOutErrors())
			}
			if root.GetCounters().InFcsErrors == nil {
				t.Errorf("ERROR: Counter InFcsErrors is not present")
			} else {
				t.Logf("INFO: Counter InFcsErrors: %d", root.GetCounters().GetInFcsErrors())
			}
		})
	}
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
	query := gnmi.OC().ComponentAny().IntegratedCircuit().PipelineCounters().Drop().State()
	asicDrops := gnmi.LookupAll(t, a.dut, query)
	if len(asicDrops) == 0 {
		t.Fatalf("ERROR: Could not perform lookup as no asic drop paths exist")
	}

	for _, asicDrop := range asicDrops {
		component := asicDrop.Path.GetElem()[1].GetKey()["name"]
		t.Run("Component "+component, func(t *testing.T) {
			drop, _ := asicDrop.Val()
			if drop.InterfaceBlock != nil {
				if drop.InterfaceBlock.InDrops == nil {
					t.Errorf("ERROR: InDrops counter is not present")
				} else {
					t.Logf("INFO: InDrops counter: %d", drop.InterfaceBlock.GetInDrops())
				}
				if drop.InterfaceBlock.OutDrops == nil {
					t.Errorf("ERROR: OutDrops counter is not present")
				} else {
					t.Logf("INFO: OutDrops counter: %d", drop.InterfaceBlock.GetInDrops())
				}
				if drop.InterfaceBlock.Oversubscription == nil {
					t.Errorf("ERROR: Oversubscription counter is not present")
				} else {
					t.Logf("INFO: Oversubscription counter: %d", drop.InterfaceBlock.GetInDrops())
				}
			}
			if drop.LookupBlock != nil {
				if drop.LookupBlock.AclDrops == nil {
					t.Errorf("ERROR: AclDrops counter is not present")
				} else {
					t.Logf("INFO: AclDrops counter: %d", drop.LookupBlock.GetAclDrops())
				}
				if drop.LookupBlock.ForwardingPolicy == nil {
					t.Errorf("ERROR: ForwardingPolicy is not present")
				} else {
					t.Logf("INFO: GetForwardingPolicy counter: %d", drop.LookupBlock.GetForwardingPolicy())
				}
				if drop.LookupBlock.FragmentTotalDrops == nil {
					t.Errorf("ERROR: FragmentTotalDrops is not present")
				} else {
					t.Logf("INFO: FragmentTotalDrops: %d", drop.LookupBlock.GetFragmentTotalDrops())
				}
				if drop.LookupBlock.IncorrectSoftwareState == nil {
					t.Errorf("ERROR: IncorrectSoftwareState counter is not present")
				} else {
					t.Logf("INFO: IncorrectSoftwareState counter: %d", drop.LookupBlock.GetIncorrectSoftwareState())
				}
				if drop.LookupBlock.InvalidPacket == nil {
					t.Errorf("ERROR: InvalidPacket counter is not present")
				} else {
					t.Logf("INFO: InvalidPacket counter: %d", drop.LookupBlock.GetInvalidPacket())
				}
				if drop.LookupBlock.LookupAggregate == nil {
					t.Errorf("ERROR: LookupAggregate counter is not present")
				} else {
					t.Logf("INFO: LookupAggregate counter: %d", drop.LookupBlock.GetLookupAggregate())
				}
				if drop.LookupBlock.NoLabel == nil {
					t.Errorf("ERROR: NoLabel counter is not present")
				} else {
					t.Logf("INFO: NoLabel counter: %d", drop.LookupBlock.GetNoLabel())
				}
				if drop.LookupBlock.NoNexthop == nil {
					t.Errorf("ERROR: NoNexthop counter is not present")
				} else {
					t.Logf("INFO: NoNexthop counter: %d", drop.LookupBlock.GetNoNexthop())
				}
				if drop.LookupBlock.NoRoute == nil {
					t.Errorf("ERROR: NoRoute counter is not present")
				} else {
					t.Logf("INFO: NoRoute counter: %d", drop.LookupBlock.GetNoNexthop())
				}
				if drop.LookupBlock.RateLimit == nil {
					t.Errorf("ERROR: RateLimit counter is not present")
				} else {
					t.Logf("INFO: RateLimit counter: %d", drop.LookupBlock.GetRateLimit())
				}
			}
		})
	}
} 

func (a *testArgs) testOC_PPC_host_subsystem(t *testing.T){
	t.Skip("skipping host subsystem")
}	

func (a *testArgs) testOC_PPC_fabric_subsystem(t *testing.T){
	t.Logf("INFO: Check no fabric drop")
	if deviations.FabricDropCounterUnsupported(a.dut) {
		t.Skipf("INFO: Skipping test due to deviation fabric_drop_counter_unsupported")
	}
	query := gnmi.OC().ComponentAny().IntegratedCircuit().PipelineCounters().Drop().FabricBlock().State()
	t.Logf("query %s", query)
	fabricBlocks := gnmi.GetAll(t, a.dut, query)
	if len(fabricBlocks) == 0 {
		t.Fatalf("ERROR: %s is not present", query)
	}
	for _, fabricBlock := range fabricBlocks {
		drop := fabricBlock.GetLostPackets()
		if fabricBlock.LostPackets == nil {
			t.Errorf("ERROR: Fabric drops is not present")
		} else {
			t.Logf("INFO: Fabric drops: %d", drop)
		}
	}
}
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
		args.testOC_PPC_interface_subsystem(t)
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
