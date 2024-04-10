// Copyright 2024 Google LLC
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
	"math"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/schemaless"
	"github.com/openconfig/ygnmi/ygnmi"
)

var (
	chassisType                                                                                                                        string // check if its distributed or fixed chassis
	tolerance                                                                                                                          uint64
	rpfoCount                                                                                                                          = 0               // if more than 10 then reset to 0 and reload the HW
	subscriptionCount                                                                                                                  = 5               // number of parallel subscriptions to be tested
	multipleSubscriptionRuntime                                                                                                        = 5 * time.Minute // duration for which parallel subscriptions will run
	doneMonitor, stopMonitor, doneClients, stopClients, doneMonitorTrigger, stopMonitorTrigger, doneClientsTrigger, stopClientsTrigger chan struct{}     // channel for go routine
)

const (
	withRpfo     = true
	withLcReload = true
	activeRp     = "0/RP0/CPU0"
	standbyRp    = "0/RP1/CPU0"
	vrf1         = "TE"
)

type testArgs struct {
	ate *ondatra.ATEDevice
	ctx context.Context
	dut *ondatra.DUTDevice
	top *ondatra.ATETopology
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func (args *testArgs) OcPpcDropBlock(t *testing.T) {
	testcases := []Testcase{
		{
			name:      "drop/lookup-block/state/no-nexthop",
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &TgnOptions{ipv4: true}),
			eventType: &eventStaticRouteToNull{prefix: "202.1.0.1/32", config: true},
		},
		{
			name:      "drop/lookup-block/state/no-label",
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &TgnOptions{mpls: true}),
			eventType: &eventEnableMplsLdp{config: true},
		},
		{
			name:      "drop/lookup-block/state/fragment-total-drops",
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &TgnOptions{ipv4: true, frame_size: 1400}),
			eventType: &eventInterfaceConfig{config: true, mtu: 500, port: sortPorts(args.dut.Ports())[1:]},
		},
		{
			name:      "drop/lookup-block/state/no-route",
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &TgnOptions{ipv4: true}),
			eventType: &eventInterfaceConfig{config: true, shut: true, port: sortPorts(args.dut.Ports())[1:]},
		},
		/*
			{
				name:      "drop/lookup-block/state/acl-drops",
				flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &TgnOptions{ipv4: true}),
				eventType: &eventAclConfig{aclName: "deny_all_ipv4", config: true}, // todo - how to cleanup while exiting the function?
			},


			{
				name: "drop/lookup-block/state/incorrect-software-state",
				flow: args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &TgnOptions{mpls: true}),
			},
			{
				name: "drop/lookup-block/state/invalid-packet",
				flow: args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &TgnOptions{ipv4: true, ttl: true}),
			},


		*/

		/*
			{ rate-limit need to check how to automate
				name: "drop/lookup-block/state/lookup-aggregate", // waiting for Muthu to advise - https://miggbo.atlassian.net/browse/XR-56749
				flow: args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &TgnOptions{ipv4: true, fps: 1000000000}),*/
	}

	npus := args.interfaceToNPU(t)                       // collecting all the destination NPUs
	data := make(map[string]ygnmi.WildcardQuery[uint64]) // holds a path and its query information
	//sampleInterval := 30 * time.Second
	for _, tt := range testcases {
		// loop over different streaming modes
		for _, subMode := range []gpb.SubscriptionMode{gpb.SubscriptionMode_SAMPLE} {
			t.Run(fmt.Sprintf("Test path %v in subscription mode %v", tt.name, subMode), func(t *testing.T) {
				t.Logf("Path name: %s", tt.name)
				var preCounters, postCounters = uint64(0), uint64(0)
				tolerance = 2.0 // 2% change tolerance is allowed between want and got value

				// TODO - uncomment after processmgr team confirms leaf mapping
				//chassisType = args.checkChassisType(t, args.dut)
				//// start go routine to track cpu/memory and running multiple clients
				//if chassisType == "distributed" {
				//	doneMonitor = make(chan struct{})
				//	stopMonitor = make(chan struct{})
				//	runBackgroundMonitor(t, stopMonitor, doneMonitor)
				//}
				//doneClients = make(chan struct{})
				//stopClients = make(chan struct{})
				//runMultipleClientBackground(t, stopClients, doneClients) // TODO - why?

				// collecting each path, query per destination NPU
				for _, npu := range npus {
					path := fmt.Sprintf("/components/component[name=%s]/integrated-circuit/pipeline-counters/%s", npu, tt.name)
					query, _ := schemaless.NewWildcard[uint64](path, "openconfig")
					data[path] = query
				}

				// running multiple subscriptions on all the queries while tc is executed
				for _, query := range data {
					sa := &subscriptionArgs{
						streamMode:     subMode,
						sampleInterval: 30 * time.Second,
					}
					sa.multipleSubscriptions(t, query)
				}

				// aggregate pre counters for a path across all the destination NPUs
				for path, query := range data {
					pre, _ := getData(t, path, query) // improve error handling
					preCounters = preCounters + pre
				}

				tgnData := float64(args.validateTrafficFlows(t, tt.flow, &TgnOptions{traffic_timer: 120, drop: true, event: tt.eventType}))

				// aggregate post counters for a path across all the destination NPUs
				for path, query := range data {
					post, _ := getData(t, path, query)
					postCounters = postCounters + post
				}

				//// Wait for both goroutines to finish using the channel
				//close(stopMonitorTrigger)
				//close(stopClientsTrigger)
				//<-doneMonitorTrigger
				//<-doneClientsTrigger

				// following reload, we can have pre data bigger than post data. So using absolute value
				want := math.Abs(float64(postCounters - preCounters)) // from DUT

				t.Logf("Initial counters for path %s : %d", tt.name, preCounters)
				t.Logf("Final counters for path %s: %d", tt.name, postCounters)
				t.Logf("Expected counters for path %s: %d", tt.name, uint64(want))

				if (math.Abs(tgnData-want)/(tgnData))*100 > float64(tolerance) {
					t.Errorf("Data doesn't match for path %s, got: %f, want: %f", tt.name, tgnData, want)
				} else {
					t.Logf("Data for path %s, got: %f, want: %f", tt.name, tgnData, want)
				}
			})
		}
	}
}

func TestOcPpc(t *testing.T) {
	t.Log("Name: OC PPC")

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	var vrfs = []string{vrf1}
	configVRF(t, dut, vrfs)
	configureDUT(t, dut)
	configBasePBR(t, dut, "TE", "ipv4", 1, "pbr", oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})
	configRoutePolicy(t, dut)
	configIsis(t, dut, "Bundle-Ether121")
	configBgp(t, dut, "100.100.100.100")

	// Configure the ATE
	// port 1 is source port
	// port 2 is destination port running isis
	// port 3 and port 4 are additional destination ports
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	configAteRoutingProtocols(t, top)
	//time.Sleep(120 * time.Second) --> remove this

	args := &testArgs{
		dut: dut,
		ate: ate,
		top: top,
		ctx: ctx,
	}
	t.Run("Test drop block", func(t *testing.T) {
		args.OcPpcDropBlock(t)
	})
}
