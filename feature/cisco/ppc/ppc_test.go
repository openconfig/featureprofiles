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

package ppc_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/schemaless"
	"github.com/openconfig/ygnmi/ygnmi"
	"golang.org/x/exp/slices"
)

type testArgs struct {
	ate *ondatra.ATEDevice
	ctx context.Context
	dut *ondatra.DUTDevice
	top *ondatra.ATETopology
}

const (
	mask             = "32"
	policyID         = "match-ipip"
	ipOverIPProtocol = 4
	vrf1             = "TE"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func sortedInterfaces(ports []*ondatra.Port) []string {
	var interfaces []string
	for _, port := range ports {
		interfaces = append(interfaces, port.Name())
	}
	slices.Sort(interfaces)
	return interfaces
}

func (args *testArgs) interfaceToNPU(t testing.TB, dst *ondatra.Port) string {
	// tmp := make(map[string]bool)
	// var result []string
	// for _, p := range args.dut.Ports() {
	// 	hwport := gnmi.Get(t, args.dut, gnmi.OC().Interface(p.Name()).HardwarePort().State())
	// 	if !tmp[hwport] {
	// 		tmp[hwport] = true
	// 		result = append(result, hwport)
	// 	}
	// }
	// result = nil
	// for _, comp := range result {
	// 	int_npu := gnmi.Get(t, args.dut, gnmi.OC().Component(comp).Parent().State())
	// 	result = append(result, int_npu)
	// }
	// return result

	hwport := gnmi.Get(t, args.dut, gnmi.OC().Interface(dst.Name()).HardwarePort().State())
	intf_npu := gnmi.Get(t, args.dut, gnmi.OC().Component(hwport).Parent().State())
	return intf_npu

}

// func (args *testArgs) runBackgroundMonitor(t *testing.T) {
// 	go func() {
// 		ticker := time.NewTicker(5 * time.Second) // Adjust the interval as needed

// 		for {
// 			gnmi.Collect(t, args.dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(proto_gnmi.SubscriptionMode_SAMPLE), ygnmi.WithSampleInterval(5*time.Minute)), gnmi.OC().NetworkInstance("*").Afts().State(), subscription_timout*time.Minute)
// 			gnmi.Collect(t, args.dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(proto_gnmi.SubscriptionMode_SAMPLE), ygnmi.WithSampleInterval(5*time.Minute)), gnmi.OC().Interface("*").State(), subscription_timout*time.Minute)
// 			select {
// 			case <-ticker.C:
// 				// Make gRPC call to get system information
// 				response, err := client.GetSystemInfo(context.Background(), &pb.SystemInfoRequest{})
// 				if err != nil {
// 					log.Printf("Error getting system info: %v", err)
// 					return
// 				}

// 				log.Printf("Memory Usage: %v, CPU Usage: %v", response.MemoryUsage, response.CPUUsage)
// 				// Add more logging or processing based on the system information

// 			case <-time.After(30 * time.Second): // Adjust the duration for the overall background run
// 				log.Println("Background monitor finished.")
// 				return
// 			}
// 		}
// 	}()
// }

// to do triggers
// var (
// 	triggers = []Testcase{
// 		{
// 			name: "Interface flap",
// 			desc: "",
// 			// fn:   test,
// 		},
// 		{
// 			name: "Process restart",
// 			desc: "restart the process (emsd, ifmgr, dbwriter, fib_mgr, ipv4/ipv6 rib) and validate pipeline counters",
// 			// fn:   test,
// 		},
// 		{
// 			name: "HA triggers",
// 			desc: "perform HA triggers like RPFO, LC reload and validate pipeline counters",
// 			// fn:   test,
// 		},
// 	}
// )

// to do subscriptions
// var (
//
//	subscriptions = []Testcase{
//		//subcription mode covers for all leaf, container and root level
//		{
//			name: "once",
//			desc: "validates subscription mode once at the root, container and leaf level",
//			// fn:   test,
//		},
//		{
//			name: "on-change",
//			desc: "validates subscription on-change at the root, container and leaf level",
//			// fn:   test,
//		},
//		{
//			name: "sample",
//			desc: "validates subscription mode sampling at the root, container and leaf level",
//			// fn:   test,
//		},
//		{
//			name: "multiple_subcriptions",
//			desc: "mix various subscription modes and levels",
//			// fn:   test,
//		},
//	}
//
// )

func (a *testArgs) testOC_PPC_interface_subsystem(t *testing.T) {

	// Testcase defines testcase structure
	type Testcase struct {
		name     string
		flow     *ondatra.Flow
		dstPorts *ondatra.Port
	}

	test := []Testcase{
		{
			name:     "packet/interface-block/in-packets/state",
			flow:     a.createFlow("valid_stream", []ondatra.Endpoint{a.top.Interfaces()["atePort2"]}, &TGNoptions{fps: 1000}),
			dstPorts: a.dut.Port(t, "port2"),
		},
		{
			name: "packet/interface-block/state/out-packets/state",
			// flow: "",
		},
		{
			name: "packet/interface-block/state/in-bytes/state",
			//flow: "",
		},
		{
			name: "packet/interface-block/state/out-bytes/state",
			//flow: "",
		},
		{
			name: "drop/interface-block/state/oversubscription/state",
			//flow: "",
		},
		{
			name: "drop/interface-block/state/in-drops/state",
			//flow: "",
		},
		{
			name: "drop/interface-block/state/out-drops/state",
			//flow: "",
		},
		{
			name: "errors/interface-block/state/error-action/state",
			//flow: "",
		},
		{
			name: "errors/interface-block/state/error-count/state",
			//flow: "",
		},
		{
			name: "errors/interface-block/state/error-level/state",
			//flow: "",
		},
		{
			name: "errors/interface-block/state/error-name/state",
			//flow: "",
		},
		{
			name: "errors/interface-block/state/error-threshold/state",
			//flow: "",
		},
	}

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			tgn_data := a.validateTrafficFlows(t, tt.flow, false, &TGNoptions{traffic_timer: 120})
			npu := a.interfaceToNPU(t, tt.dstPorts)
			query, _ := schemaless.NewWildcard[uint64](fmt.Sprintf("/components/component[name=%s]/integrated-circuit/%s", npu, tt.name), "openconfig")
			ygnmi_client, _ := ygnmi.NewClient(a.dut.RawAPIs().GNMI(t), ygnmi.WithTarget(a.dut.ID()))

			ctx, _ := context.WithDeadline(context.Background(), time.Now().Add(2*time.Minute))
			watcher := ygnmi.WatchAll(ctx, ygnmi_client, query, func(v *ygnmi.Value[uint64]) error {
				if v.IsPresent() {
					return nil
				}
				vl, _ := v.Val()
				if vl == tgn_data {
					return nil
				}
				return ygnmi.Continue
			})
			watcher.Await()
		})
	}
}

// func (a *testArgs) testOC_PPC_queuing_subsystem(t *testing.T) {
// 	type testCase struct {
// 		desc     string
// 		path     string
// 		counters []*ygnmi.Value[uint64]
// 	}
// 	interfaces := sortedInterfaces(a.dut.Ports())
// 	t.Logf("Interfaces: %s", interfaces)
// 	for _, intf := range interfaces {
// 		t.Run(intf, func(t *testing.T) {
// 			qosInterface := gnmi.OC().Qos().Interface(intf)
// 			cases := []testCase{
// 				{
// 					desc:     "Queue Input Dropped packets",
// 					path:     "/qos/interfaces/interface/input/queues/queue/state/dropped-pkts",
// 					counters: gnmi.LookupAll(t, a.dut, qosInterface.Input().QueueAny().DroppedPkts().State()),
// 				},
// 				{
// 					desc:     "Queue Output Dropped packets",
// 					path:     "/qos/interfaces/interface/output/queues/queue/state/dropped-pkts",
// 					counters: gnmi.LookupAll(t, a.dut, qosInterface.Output().QueueAny().DroppedPkts().State()),
// 				},
// 				{
// 					desc:     "Queue input voq-output-interface dropped packets",
// 					path:     "/qos/interfaces/interface/input/virtual-output-queues/voq-interface/queues/queue/state/dropped-pkts",
// 					counters: gnmi.LookupAll(t, a.dut, qosInterface.Input().VoqInterfaceAny().QueueAny().DroppedPkts().State()),
// 				},
// 			}
// 			for _, c := range cases {
// 				t.Run(c.desc, func(t *testing.T) {
// 					if len(c.counters) == 0 {
// 						t.Errorf("%s Interface %s Telemetry Value is not present", c.desc, intf)
// 					}
// 					for queueID, dropPkt := range c.counters {
// 						dropCount, present := dropPkt.Val()
// 						if !present {
// 							t.Errorf("%s Interface %s %s Telemetry Value is not present", c.desc, intf, dropPkt.Path)
// 						} else {
// 							t.Logf("%s Interface %s, Queue %d has %d drop(s)", dropPkt.Path.GetOrigin(), intf, queueID, dropCount)
// 						}
// 					}
// 				})
// 			}
// 		})
// 	}
// }

// func (a *testArgs) testOC_PPC_lookup_subsystem(t *testing.T) {
// 	query := gnmi.OC().ComponentAny().IntegratedCircuit().PipelineCounters().Drop().State()
// 	asicDrops := gnmi.LookupAll(t, a.dut, query)
// 	if len(asicDrops) == 0 {
// 		t.Fatalf("ERROR: Could not perform lookup as no asic drop paths exist")
// 	}

// 	for _, asicDrop := range asicDrops {
// 		component := asicDrop.Path.GetElem()[1].GetKey()["name"]
// 		t.Run("Component "+component, func(t *testing.T) {
// 			drop, _ := asicDrop.Val()
// 			if drop.InterfaceBlock != nil {
// 				if drop.InterfaceBlock.InDrops == nil {
// 					t.Errorf("ERROR: InDrops counter is not present")
// 				} else {
// 					t.Logf("INFO: InDrops counter: %d", drop.InterfaceBlock.GetInDrops())
// 				}
// 				if drop.InterfaceBlock.OutDrops == nil {
// 					t.Errorf("ERROR: OutDrops counter is not present")
// 				} else {
// 					t.Logf("INFO: OutDrops counter: %d", drop.InterfaceBlock.GetInDrops())
// 				}
// 				if drop.InterfaceBlock.Oversubscription == nil {
// 					t.Errorf("ERROR: Oversubscription counter is not present")
// 				} else {
// 					t.Logf("INFO: Oversubscription counter: %d", drop.InterfaceBlock.GetInDrops())
// 				}
// 			}
// 			if drop.LookupBlock != nil {
// 				if drop.LookupBlock.AclDrops == nil {
// 					t.Errorf("ERROR: AclDrops counter is not present")
// 				} else {
// 					t.Logf("INFO: AclDrops counter: %d", drop.LookupBlock.GetAclDrops())
// 				}
// 				if drop.LookupBlock.ForwardingPolicy == nil {
// 					t.Errorf("ERROR: ForwardingPolicy is not present")
// 				} else {
// 					t.Logf("INFO: GetForwardingPolicy counter: %d", drop.LookupBlock.GetForwardingPolicy())
// 				}
// 				if drop.LookupBlock.FragmentTotalDrops == nil {
// 					t.Errorf("ERROR: FragmentTotalDrops is not present")
// 				} else {
// 					t.Logf("INFO: FragmentTotalDrops: %d", drop.LookupBlock.GetFragmentTotalDrops())
// 				}
// 				if drop.LookupBlock.IncorrectSoftwareState == nil {
// 					t.Errorf("ERROR: IncorrectSoftwareState counter is not present")
// 				} else {
// 					t.Logf("INFO: IncorrectSoftwareState counter: %d", drop.LookupBlock.GetIncorrectSoftwareState())
// 				}
// 				if drop.LookupBlock.InvalidPacket == nil {
// 					t.Errorf("ERROR: InvalidPacket counter is not present")
// 				} else {
// 					t.Logf("INFO: InvalidPacket counter: %d", drop.LookupBlock.GetInvalidPacket())
// 				}
// 				if drop.LookupBlock.LookupAggregate == nil {
// 					t.Errorf("ERROR: LookupAggregate counter is not present")
// 				} else {
// 					t.Logf("INFO: LookupAggregate counter: %d", drop.LookupBlock.GetLookupAggregate())
// 				}
// 				if drop.LookupBlock.NoLabel == nil {
// 					t.Errorf("ERROR: NoLabel counter is not present")
// 				} else {
// 					t.Logf("INFO: NoLabel counter: %d", drop.LookupBlock.GetNoLabel())
// 				}
// 				if drop.LookupBlock.NoNexthop == nil {
// 					t.Errorf("ERROR: NoNexthop counter is not present")
// 				} else {
// 					t.Logf("INFO: NoNexthop counter: %d", drop.LookupBlock.GetNoNexthop())
// 				}
// 				if drop.LookupBlock.NoRoute == nil {
// 					t.Errorf("ERROR: NoRoute counter is not present")
// 				} else {
// 					t.Logf("INFO: NoRoute counter: %d", drop.LookupBlock.GetNoNexthop())
// 				}
// 				if drop.LookupBlock.RateLimit == nil {
// 					t.Errorf("ERROR: RateLimit counter is not present")
// 				} else {
// 					t.Logf("INFO: RateLimit counter: %d", drop.LookupBlock.GetRateLimit())
// 				}
// 			}
// 		})
// 	}
// }

// func (a *testArgs) testOC_PPC_host_subsystem(t *testing.T) {
// 	query := gnmi.OC().System().AlarmAny().State()
// 	alarms := gnmi.LookupAll(t, a.dut, query)
// 	if len(alarms) > 0 {
// 		for _, alarm := range alarms {
// 			val, _ := alarm.Val()
// 			alarmSeverity := val.GetSeverity()
// 			// Checking for major system alarms.
// 			if alarmSeverity == oc.AlarmTypes_OPENCONFIG_ALARM_SEVERITY_MAJOR {
// 				t.Logf("INFO: System Alarm with severity %s seen: %s", alarmSeverity, val.GetText())
// 			}
// 		}
// 	}
// }

// func (a *testArgs) testOC_PPC_fabric_subsystem(t *testing.T) {
// 	t.Logf("INFO: Check no fabric drop")
// 	if deviations.FabricDropCounterUnsupported(a.dut) {
// 		t.Skipf("INFO: Skipping test due to deviation fabric_drop_counter_unsupported")
// 	}
// 	query := gnmi.OC().ComponentAny().IntegratedCircuit().PipelineCounters().Drop().FabricBlock().State()
// 	t.Logf("query %s", query)
// 	fabricBlocks := gnmi.GetAll(t, a.dut, query)
// 	if len(fabricBlocks) == 0 {
// 		t.Fatalf("ERROR: %s is not present", query)
// 	}
// 	for _, fabricBlock := range fabricBlocks {
// 		drop := fabricBlock.GetLostPackets()
// 		if fabricBlock.LostPackets == nil {
// 			t.Errorf("ERROR: Fabric drops is not present")
// 		} else {
// 			t.Logf("INFO: Fabric drops: %d", drop)
// 		}
// 	}
// }

// func (a *testArgs) testOC_PPC_ethernet_drop(t *testing.T) {
// 	interfaces := sortedInterfaces(a.dut.Ports())
// 	t.Logf("Interfaces: %s", interfaces)
// 	for _, intf := range interfaces {
// 		t.Run(intf, func(t *testing.T) {
// 			counters := gnmi.OC().Interface(intf).Ethernet().Counters()
// 			cases := []struct {
// 				desc    string
// 				counter ygnmi.SingletonQuery[uint64]
// 			}{
// 				{
// 					desc:    "InCrcErrors",
// 					counter: counters.InCrcErrors().State(),
// 				},
// 				{
// 					desc:    "InMacPauseFrames",
// 					counter: counters.InMacPauseFrames().State(),
// 				},
// 				{
// 					desc:    "OutMacPauseFrames",
// 					counter: counters.OutMacPauseFrames().State(),
// 				},
// 				{
// 					desc:    "InBlockErrors",
// 					counter: counters.InBlockErrors().State(),
// 				},
// 			}

// 			for _, c := range cases {
// 				t.Run(c.desc, func(t *testing.T) {
// 					if val, present := gnmi.Lookup(t, a.dut, c.counter).Val(); present {
// 						t.Logf("INFO: %s: %d", c.counter, val)
// 					} else {
// 						t.Errorf("ERROR: %s is not present", c.counter)
// 					}
// 				})
// 			}
// 		})
// 	}
// }

func TestOC_PPC(t *testing.T) {
	t.Log("Name: OC PPC")

	dut := ondatra.DUT(t, "dut")
	// ctx := context.Background()
	// Configure the DUT
	var vrfs = []string{vrf1}
	configVRF(t, dut, vrfs)
	configureDUT(t, dut)
	// PBR config
	// configbasePBR(t, dut, "REPAIRED", "ipv4", 1, "pbr", oc.PacketMatchTypes_IP_PROTOCOL_UNSET, []uint8{}, &PBROptions{SrcIP: "222.222.222.222/32"})
	configbasePBR(t, dut, "TE", "ipv4", 1, "pbr", oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})
	// RoutePolicy config
	configRP(t, dut)
	// configure ISIS on DUT
	addISISOC(t, dut, "Bundle-Ether121")
	// configure BGP on DUT
	addBGPOC(t, dut, "100.100.100.100")

	// Configure the ATE
	// port 1 is source port
	// port 2 is destination port running isis
	// port 3 and port 4 are additional destionation ports
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	addPrototoAte(t, top)
	time.Sleep(120 * time.Second)

	args := &testArgs{
		dut: dut,
		ate: ate,
		top: top,
	}

	// goroutine to check cpu/memory in the background, need to work on it
	// t.Logf("check CPU/memory in the background")
	// args.runBackgroundMonitor(t)

	t.Run("Test interface subsystem", func(t *testing.T) {
		args.testOC_PPC_interface_subsystem(t)
	})

	// t.Run("Test queuing subsystem", func(t *testing.T) {
	// 	args.testOC_PPC_queuing_subsystem(t)
	// })

	// t.Run("Test lookup subsystem", func(t *testing.T) {
	// 	args.testOC_PPC_lookup_subsystem(t)
	// })

	// t.Run("Test host subsystem", func(t *testing.T) {
	// 	args.testOC_PPC_host_subsystem(t)
	// })

	// t.Run("Test fabric subsystem", func(t *testing.T) {
	// 	args.testOC_PPC_fabric_subsystem(t)
	// })
}
