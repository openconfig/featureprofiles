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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/ha/runner"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	gnps "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/schemaless"
	"github.com/openconfig/ygnmi/ygnmi"
)

var (
	chassisType                 string // check if its distributed or fixed chassis
	tolerance                   uint64
	rpfoCount                   = 0               // if more than 10 then reset to 0 and reload the HW
	subscriptionCount           = 5               // number of parallel subscriptions to be tested
	multipleSubscriptionRuntime = 5 * time.Minute // duration for which parallel subscriptions will run
)

const (
	withRpfo         = true
	withLcReload     = true
	activeRp         = "0/RP0/CPU0"
	standbyRp        = "0/RP1/CPU0"
	mask             = "32"
	policyID         = "match-ipip"
	ipOverIPProtocol = 4
	vrf1             = "TE"
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

func (args *testArgs) interfaceToNPU(t testing.TB, dst *ondatra.Port) string {
	hwPort := gnmi.Get(t, args.dut, gnmi.OC().Interface(dst.Name()).HardwarePort().State())
	intfNpu := gnmi.Get(t, args.dut, gnmi.OC().Component(hwPort).Parent().State())
	return intfNpu
}

func runBackgroundMonitor(t *testing.T) {
	t.Logf("check CPU/memory in the background")
	dut := ondatra.DUT(t, "dut")
	deviceName := dut.Name()
	processes := []string{"emsd", "dbwriter", "dblistener"}
	pList := gnmi.GetAll(t, dut, gnmi.OC().System().ProcessAny().State())
	var pID []uint64
	for _, process := range processes {
		for _, proc := range pList {
			if proc.GetName() == process {
				pID = append(pID, proc.GetPid())
				t.Logf("Pid of daemon '%s' is '%d'", process, pID)
			}
		}
	}

	go func() {
		for {
			for _, process := range pID {
				query := gnmi.OC().System().Process(process).State()
				timestamp := time.Now().Round(time.Second)
				result := gnmi.Get(t, dut, query)
				processName := result.GetName()
				t.Run(processName, func(t *testing.T) {
					if *result.CpuUtilization > 80 {
						t.Logf("%s %s CPU Process utilization high for process %-10s, utilization: %3d%%", timestamp, deviceName, processName, result.GetCpuUtilization())
					} else {
						t.Logf("%s %s INFO: CPU process %-10s utilization: %3d%%", timestamp, deviceName, processName, result.GetCpuUtilization())
					}
					if result.MemoryUtilization != nil {
						t.Logf("%s %s Memory high for process: %-10s - Utilization: %3d%%", timestamp, deviceName, processName, result.GetMemoryUtilization())
					} else {
						t.Logf("%s %s INFO:  Memory Process %-10s utilization: %3d%%", timestamp, deviceName, processName, result.GetMemoryUtilization())
					}
				})
			}
			// sleep for 30 seconds before checking cpu/memory again
			time.Sleep(30 * time.Second)
		}
	}()
}

type triggerType interface {
}

type triggerProcessRestart struct {
	processes []string
}

func (tt triggerProcessRestart) restartProcessBackground(t *testing.T, ctx context.Context) {
	dut := ondatra.DUT(t, "dut")
	for _, process := range tt.processes {

		//patch for CLIviaSSH failing, else pattern to use is #
		var acp string
		if withRpfo {
			acp = ".*Last switch-over.*ago"
		} else {
			acp = ".*"
		}

		ticker1 := time.NewTicker(3 * time.Second)
		runner.RunCLIInBackground(ctx, t, dut, fmt.Sprintf("process restart %s", process), []string{acp}, []string{".*Incomplete.*", ".*Unable.*"}, ticker1, 4*time.Second)
		time.Sleep(4 * time.Second)
		ticker1.Stop()
	}
}

type triggerRpfo struct {
}

func (tt triggerRpfo) rpfo(t *testing.T, ctx context.Context) {
	dut := ondatra.DUT(t, "dut")

	// reload the HW if rfpo count is 10 or more
	if rpfoCount == 10 {
		gnoiClient := dut.RawAPIs().GNOI(t)
		rebootRequest := &gnps.RebootRequest{
			Method: gnps.RebootMethod_COLD,
			Force:  true,
		}
		rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootRequest)
		t.Logf("Got reboot response: %v, err: %v", rebootResponse, err)
		if err != nil {
			t.Fatalf("Failed to reboot chassis with unexpected err: %v", err)
		}
		rpfoCount = 0
		time.Sleep(time.Minute * 20) // TODO - why 20 minutes?
	}
	// supervisor info
	var supervisors []string
	activeState := gnmi.OC().Component(activeRp).Name().State()
	active := gnmi.Get(t, dut, activeState)
	standbyState := gnmi.OC().Component(standbyRp).Name().State()
	standby := gnmi.Get(t, dut, standbyState)
	supervisors = append(supervisors, active, standby)

	// find active and standby RP
	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyRP(t, dut, supervisors)
	t.Logf("Detected activeRP: %v, standbyRP: %v", rpActiveBeforeSwitch, rpStandbyBeforeSwitch)

	// make sure standby RP is reachable
	switchoverReady := gnmi.OC().Component(rpActiveBeforeSwitch).SwitchoverReady()
	gnmi.Await(t, dut, switchoverReady.State(), 30*time.Minute, true)
	t.Logf("SwitchoverReady().Get(t): %v", gnmi.Get(t, dut, switchoverReady.State()))
	if got := gnmi.Get(t, dut, switchoverReady.State()); got != true {
		t.Errorf("switchoverReady.Get(t): got %v, want %v", got, true)
	}
	gnoiClient, _ := dut.RawAPIs().BindingDUT().DialGNOI(ctx)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	switchoverRequest := &gnps.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(rpStandbyBeforeSwitch, useNameOnly),
	}
	t.Logf("switchoverRequest: %v", switchoverRequest)
	var switchoverResponse *gnps.SwitchControlProcessorResponse
	err := retryUntilTimeout(func() error {
		switchoverResponse, _ = gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
		return nil
	}, 5, 1*time.Minute)

	if err != nil {
		fmt.Printf("RPFO failed: %v\n", err)
	} else {
		fmt.Println("RPFO succeeded!")
	}
	// t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v, err: %v", switchoverResponse, err)

	want := rpStandbyBeforeSwitch
	got := ""
	if useNameOnly {
		got = switchoverResponse.GetControlProcessor().GetElem()[0].GetName()
	} else {
		got = switchoverResponse.GetControlProcessor().GetElem()[1].GetKey()["name"]
	}
	if got != want {
		t.Fatalf("switchoverResponse.GetControlProcessor().GetElem()[0].GetName(): got %v, want %v", got, want)
	}

	startSwitchover := time.Now()
	t.Logf("Wait for new active RP to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f seconds since switchover started.", time.Since(startSwitchover).Seconds())
		time.Sleep(30 * time.Second)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("RP switchover has completed successfully with received time: %v", currentTime)
			break
		}
		if got, want := uint64(time.Since(startSwitchover).Seconds()), uint64(900); got >= want {
			t.Fatalf("time.Since(startSwitchover): got %v, want < %v", got, want)
		}
	}
	t.Logf("RP switchover time: %.2f seconds", time.Since(startSwitchover).Seconds())

	rpStandbyAfterSwitch, rpActiveAfterSwitch := components.FindStandbyRP(t, dut, supervisors)
	t.Logf("Found standbyRP after switchover: %v, activeRP: %v", rpStandbyAfterSwitch, rpActiveAfterSwitch)

	if got, want := rpActiveAfterSwitch, rpStandbyBeforeSwitch; got != want {
		t.Errorf("Get rpActiveAfterSwitch: got %v, want %v", got, want)
	}
	if got, want := rpStandbyAfterSwitch, rpActiveBeforeSwitch; got != want {
		t.Errorf("Get rpStandbyAfterSwitch: got %v, want %v", got, want)
	}

	t.Log("Validate OC Switchover time/reason.")
	activeRP := gnmi.OC().Component(rpActiveAfterSwitch)
	if got := gnmi.Lookup(t, dut, activeRP.LastSwitchoverTime().State()).IsPresent(); got != true {
		t.Errorf("activeRP.LastSwitchoverTime().Lookup(t).IsPresent(): got %v, want %v", got, true)
	} else {
		t.Logf("Found activeRP.LastSwitchoverTime(): %v", gnmi.Get(t, dut, activeRP.LastSwitchoverTime().State()))
	}

	if got, want := gnmi.Lookup(t, dut, activeRP.LastSwitchoverReason().State()).IsPresent(), true; got != want {
		t.Errorf("activeRP.LastSwitchoverReason().Lookup(t).IsPresent(): got %v, want %v", got, want)
	} else {
		lastSwitchoverReason := gnmi.Get(t, dut, activeRP.LastSwitchoverReason().State())
		t.Logf("Found lastSwitchoverReason.GetDetails(): %v", lastSwitchoverReason.GetDetails())
		t.Logf("Found lastSwitchoverReason.GetTrigger().String(): %v", lastSwitchoverReason.GetTrigger().String())
	}
}

type triggerLcReload struct {
	tolerance uint64
}

// func (tt trigger_lc_reload) lc_reload(t *testing.T) {
// 	dut := ondatra.DUT(t, "dut")
// 	ls := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD)

// 	for _, l := range ls {
// 		t.Run(l, func(t *testing.T) {
// 			empty, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(l).Empty().State()).Val()
// 			if ok && empty {
// 				t.Skipf("Linecard Component %s is empty, hence skipping", l)
// 			}
// 			if !gnmi.Get(t, dut, gnmi.OC().Component(l).Removable().State()) {
// 				t.Skipf("Skip the test on non-removable linecard.")
// 			}

// 			oper := gnmi.Get(t, dut, gnmi.OC().Component(l).OperStatus().State())

// 			if got, want := oper, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE; got != want {
// 				t.Skipf("Linecard Component %s is already INACTIVE, hence skipping", l)
// 			}

// 			gnoiClient := dut.RawAPIs().GNOI(t)
// 			useNameOnly := deviations.GNOISubcomponentPath(dut)
// 			lineCardPath := components.GetSubcomponentPath(l, useNameOnly)
// 			rebootSubComponentRequest := &gnps.RebootRequest{
// 				Method: gnps.RebootMethod_COLD,
// 				Subcomponents: []*tpb.Path{
// 					// {
// 					//  Elem: []*tpb.PathElem{{Name: lc}},
// 					// },
// 					lineCardPath,
// 				},
// 			}
// 			t.Logf("rebootSubComponentRequest: %v", rebootSubComponentRequest)
// 			rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootSubComponentRequest)
// 			if err != nil {
// 				t.Fatalf("Failed to perform line card reboot with unexpected err: %v", err)
// 			}
// 			t.Logf("gnoiClient.System().Reboot() response: %v, err: %v", rebootResponse, err)

// 			// sleep while lc reloads
// 			time.Sleep(10 * time.Minute)
// 		})
// 	}
// }

func checkChassisType(t *testing.T, dut *ondatra.DUTDevice) string {
	cs := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD)
	if len(cs) < 2 {
		return "fixed"
	} else {
		return "distributed"
	}
}

type SubscriptionType interface {
	isSubscriptionType()
}

type SubscriptionModeWrapper struct {
	Mode gpb.SubscriptionMode
}

func (s *SubscriptionModeWrapper) isSubscriptionType() {}

// SubscriptionListWrapper is a struct that wraps gnmi.SubscriptionList.
type SubscriptionListWrapper struct {
	List *gpb.SubscriptionList
}

func (s *SubscriptionListWrapper) isSubscriptionType() {}

type eventType interface {
}

type eventInterfaceConfig struct {
	config bool
	shut   bool
	mtu    int
	port   []string
}

func (ia eventInterfaceConfig) interfaceConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	cliPath, err := schemaless.NewConfig[string]("", "cli")
	if err != nil {
		t.Fatalf("Failed to create CLI ygnmi query: %v", err)
	}
	for _, port := range ia.port {
		dutP := dut.Port(t, port)
		if ia.config {
			if ia.shut {
				gnmi.Replace(t, dut, gnmi.OC().Interface(dutP.Name()).Enabled().Config(), false)
			}
			if ia.mtu != 0 {
				mtu := fmt.Sprintf("interface bundle-Ether 121 mtu %d", ia.mtu)
				gnmi.Update(t, dut, cliPath, mtu)
			}
		} else {
			if ia.shut {
				gnmi.Replace(t, dut, gnmi.OC().Interface(dutP.Name()).Enabled().Config(), true)
			}
			if ia.mtu != 0 {
				mtu := fmt.Sprintf("no interface bundle-Ether 121 mtu %d", ia.mtu)
				gnmi.Update(t, dut, cliPath, mtu)
			}
		}

	}
}

type eventStaticRouteToNull struct {
	prefix string
	config bool
}

func (ia eventStaticRouteToNull) staticRouteToNull(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	cliPath, err := schemaless.NewConfig[string]("", "cli")
	if err != nil {
		t.Fatalf("Failed to create CLI ygnmi query: %v", err)
	}
	var static_route string
	if ia.config {
		static_route = fmt.Sprintf("router static address-family ipv4 unicast %s null 0", ia.prefix)
	} else {
		static_route = fmt.Sprintf("no router static address-family ipv4 unicast %s null 0", ia.prefix)
	}
	gnmi.Update(t, dut, cliPath, static_route)

}

type eventEnableMplsLdp struct {
	config bool
}

func (ia eventEnableMplsLdp) enableMplsLdp(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	cliPath, err := schemaless.NewConfig[string]("", "cli")
	if err != nil {
		t.Fatalf("Failed to create CLI ygnmi query: %v", err)
	}
	var mpls_ldp string
	if ia.config {
		mpls_ldp = "mpls ldp interface bundle-Ether 120"
	} else {
		mpls_ldp = "no mpls ldp"
	}
	gnmi.Update(t, dut, cliPath, mpls_ldp)

}

// Testcase defines testcase structure
type Testcase struct {
	name string
	desc string
	npu  string
	flow *ondatra.Flow
	// sub_type   SubscriptionType
	eventType   eventType   // events for creating the scenario
	triggerType triggerType // triggers
}

// to do subscriptions
// var (
// 	subscriptions = []Testcase{
// 		//subcription mode covers for all leaf, container and root level
// 		// {
// 		// 	name:     "once",
// 		// 	desc:     "validates subscription mode once at the root, container and leaf level once using gNMI get",
// 		// 	sub_type: &SubscriptionListWrapper{List: &gpb.SubscriptionList_ONCE},
// 		// },
// 		{
// 			name:     "on-change",
// 			desc:     "validates subscription on-change at the root, container and leaf level",
// 			sub_type: &SubscriptionModeWrapper{Mode: gpb.SubscriptionMode_ON_CHANGE},
// 		},
// 		{
// 			name:     "sample",
// 			desc:     "validates subscription mode sampling at the root, container and leaf level",
// 			sub_type: &SubscriptionModeWrapper{Mode: gpb.SubscriptionMode_SAMPLE},
// 		},
// 		// {
// 		// 	name: "sample",
// 		// 	desc: "validates subscription mode sampling at the root, container and leaf level",
// 		// 	mode: gpb.SubscriptionMode_TARGET_DEFINED,
// 		// },
// 		// {
// 		// 	name: "multiple_subcriptions",
// 		// 	desc: "mix various subscription modes and levels",
// 		// },
// 	}
// )

// func create_gnmi_request(t *testing.T, path, npu string, sub SubscriptionType) *gpb.SubscribeRequest {

// 	var request *gpb.SubscribeRequest

// 	switch v := sub.(type) {
// 	case *SubscriptionModeWrapper:
// 		request = &gpb.SubscribeRequest{
// 			Request: &gpb.SubscribeRequest_Subscribe{
// 				Subscribe: &gpb.SubscriptionList{
// 					Subscription: []*gpb.Subscription{
// 						{
// 							Path: &gpb.Path{
// 								Elem: []*gpb.PathElem{
// 									{Name: fmt.Sprintf("/components/component[name=%s]/integrated-circuit/pipeline-counters/%s", npu, path)},
// 								},
// 							},
// 							Mode:           v.Mode,
// 							SampleInterval: 10000,
// 						},
// 					},
// 				},
// 			},
// 		}
// 		return request
// 	// case *SubscriptionListWrapper:
// 	// 	request = &gpb.SubscribeRequest{
// 	// 		Request: &gpb.SubscribeRequest_Subscribe{
// 	// 			Subscribe: &gpb.SubscriptionList{
// 	// 				Subscription: []*gpb.Subscription{
// 	// 					{
// 	// 						Path: &gpb.Path{
// 	// 							Elem: []*gpb.PathElem{
// 	// 								{Name: path},
// 	// 							},
// 	// 						},
// 	// 						List: v.List,
// 	// 					},
// 	// 				},
// 	// 			},
// 	// 		},
// 	// 	}
// 	default:
// 		return request
// 	}
// }

func getPathFromElements(input []*gpb.PathElem) string {
	var result []string
	for _, elem := range input {
		// If there are key-value pairs, add them to the keyPart
		if elem.Key != nil {
			for key, value := range elem.Key {
				result = append(result, elem.Name+fmt.Sprintf("[%s=%s]", key, value))
			}
		} else {
			result = append(result, elem.Name)
		}
	}
	return "/" + strings.Join(result, "/")
}

// func gnmiOpts(t *testing.T, dut *ondatra.DUTDevice, mode gpb.SubscriptionMode, interval time.Duration) *gnmi.Opts {
// 	return dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(mode), ygnmi.WithSampleInterval(interval))
// }

// Extend triggers
var triggers = []Testcase{
	{
		name: "Process restart",
		// restart npu_drvr from linux prompt, ofa_npd on LC since they'll cause router to reload and that is covered in RPFO tc
		desc:        "restart the process emsd, ifmgr, dbwriter, dblistener, fib_mgr, ipv4/ipv6 rib, isis  and validate pipeline counters",
		triggerType: &triggerProcessRestart{processes: []string{"ifmgr", "db_writer", "db_listener", "emsd", "ipv4_rib", "ipv6_rib", "fib_mgr", "isis"}},
	},
	{
		name:        "RPFO",
		desc:        "perform RPFO and validate pipeline counters",
		triggerType: &triggerRpfo{},
	},
	{
		name:        "LC reload",
		desc:        "perform LC reload and validate pipeline counters",
		triggerType: &triggerLcReload{tolerance: 40}, //when LC is reloading, component is missing and indeed no data will be collected hence tolerance is needed
	},
}

func (args *testArgs) testocDropBlock(t *testing.T) {

	test := []Testcase{
		{
			name:      "drop/lookup-block/state/no-route",
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["atePort2"]}, &TGNoptions{ipv4: true}),
			npu:       args.interfaceToNPU(t, args.dut.Port(t, "port2")),
			eventType: &eventInterfaceConfig{config: true, shut: true, port: []string{"port2"}},
		},
		//{
		//	name:       "drop/lookup-block/state/no-nexthop",
		//	flow:       a.createFlow("valid_stream", []ondatra.Endpoint{a.top.Interfaces()["atePort2"]}, &TGNoptions{ipv4: true}),
		//	npu:        a.interfaceToNPU(t, a.dut.Port(t, "port2")),
		//	event_type: &event_static_route_to_null{prefix: "202.1.0.1/32", config: true},
		//},
		//{
		//	name:       "drop/lookup-block/state/no-label",
		//	flow:       a.createFlow("valid_stream", []ondatra.Endpoint{a.top.Interfaces()["atePort2"]}, &TGNoptions{mpls: true}),
		//	npu:        a.interfaceToNPU(t, a.dut.Port(t, "port2")),
		//	event_type: &event_enable_mpls_ldp{config: true},
		//},
		//{
		//	name: "drop/lookup-block/state/incorrect-software-state",
		//	flow: a.createFlow("valid_stream", []ondatra.Endpoint{a.top.Interfaces()["atePort2"]}, &TGNoptions{mpls: true}),
		//	npu:  a.interfaceToNPU(t, a.dut.Port(t, "port2")),
		//},
		//{
		//	name: "drop/lookup-block/state/invalid-packet",
		//	flow: a.createFlow("valid_stream", []ondatra.Endpoint{a.top.Interfaces()["atePort2"]}, &TGNoptions{ipv4: true, ttl: true}),
		//	npu:  a.interfaceToNPU(t, a.dut.Port(t, "port2")),
		//},
		//{
		//	name:       "drop/lookup-block/state/fragment-total-drops",
		//	flow:       a.createFlow("valid_stream", []ondatra.Endpoint{a.top.Interfaces()["atePort2"]}, &TGNoptions{ipv4: true, frame_size: 1400}),
		//	npu:        a.interfaceToNPU(t, a.dut.Port(t, "port2")),
		//	event_type: &
		//	{config: true, mtu: 500, port: []string{"port2"}},
		//},
		// {
		// 	name: "drop/lookup-block/state/acl-drops",
		// 	CSCwi94987,
		// },
	}

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			tolerance = 2.0 // 2% change tolerance is allowed between want and got value
			path := fmt.Sprintf("/components/component[name=%s]/integrated-circuit/pipeline-counters/%s", tt.npu, tt.name)
			query, _ := schemaless.NewWildcard[uint64](path, "openconfig")

			// running multiple subscriptions while tc is executed
			sa := &subscriptionArgs{streamMode: gpb.SubscriptionMode_SAMPLE}
			sa.multipleSubscriptions(t, query, subscriptionCount)

			preData, _ := getData(t, path, query)

			tgnData := args.validateTrafficFlows(t, tt.flow, &TGNoptions{traffic_timer: 120, drop: true, event: tt.eventType})

			postData, _ := getData(t, path, query)

			got := postData - preData
			if ((tgnData-got)/tgnData)*100 > tolerance {
				t.Errorf("Data doesn't match for path %s, got: %d, want: %d", path, got, tgnData)
			} else {
				t.Logf("Data for path %s, got: %d, want: %d", path, got, tgnData)
			}
		})
	}
}

func getData(t *testing.T, path string, query ygnmi.WildcardQuery[uint64]) (uint64, error) {
	dut := ondatra.DUT(t, "dut")

	data, _ := gnmi.WatchAll(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode(gpb.SubscriptionList_ONCE))),
		query,
		30*time.Second,
		// Stop the gnmi.Watch() if value is invalid.
		func(val *ygnmi.Value[uint64]) bool {
			_, present := val.Val()
			element := val.Path.Elem
			if getPathFromElements(element) == path {
				return present
			}
			return !present
		}).Await(t)
	counter, ok := data.Val()
	if ok {
		return counter, nil
	} else {
		return 0, fmt.Errorf("Failed to collect data for path %s", path)
	}
}

// keep subscription args
type subscriptionArgs struct {
	sampleInterval time.Duration
	streamMode     gpb.SubscriptionMode
}

// subMode represents type of STREAMING subscription mode
func (sa subscriptionArgs) multipleSubscriptions(t *testing.T, query ygnmi.WildcardQuery[uint64], subCount int) {
	dut := ondatra.DUT(t, "dut")
	// once, poll, stream
	// sample, on-change, target-defined
	var wg sync.WaitGroup
	wg.Add(subCount)

	for i := 1; i <= subCount; i++ {
		go func() {
			switch sa.streamMode {
			case gpb.SubscriptionMode_SAMPLE:
				gnmi.CollectAll(t, dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(sa.streamMode), ygnmi.WithSampleInterval(sa.sampleInterval)), query, multipleSubscriptionRuntime)
			case gpb.SubscriptionMode_ON_CHANGE:
				gnmi.CollectAll(t, dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(sa.streamMode)), query, multipleSubscriptionRuntime)
			default: // TODO - target-defined is not supported yet till Sev6 is resolved
				gnmi.CollectAll(t, dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_SAMPLE)), query, multipleSubscriptionRuntime)
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

// sleeping while all the concurrent subscriptions are executed
// time.Sleep(time.Duration(multiple_subscription_runtime) * time.Minute)

//func multiple_subscriptions(t *testing.T, query ygnmi.WildcardQuery[uint64]) {
//	dut := ondatra.DUT(t, "dut")
//	for i := 1; i <= multiple_subscription; i++ {
//		gnmi.CollectAll(t, dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_SAMPLE), ygnmi.WithSampleInterval(30*time.Second)), query, time.Duration(multiple_subscription_runtime)*time.Minute)
//	}
//	// sleeping while all the concurrent subscriptions are executed
//	// time.Sleep(time.Duration(multiple_subscription_runtime) * time.Minute)
//}

func retryUntilTimeout(task func() error, maxAttempts int, timeout time.Duration) error {
	startTime := time.Now()
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := task(); err == nil {
			return nil
		}

		// Calculate how much time has passed
		elapsedTime := time.Since(startTime)

		// If the elapsed time exceeds the timeout, break out of the loop
		if elapsedTime >= timeout {
			break
		}

		// Wait for a short interval before the next attempt
		// You can adjust the sleep duration based on your needs
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("Task failed after %d attempts within a %s timeout", maxAttempts, timeout)
}

func TestOC_PPC(t *testing.T) {
	t.Log("Name: OC PPC")

	// starting cpu/memory check for all the processes in the background
	runBackgroundMonitor(t)

	dut := ondatra.DUT(t, "dut")

	//Determine if its fixed or distributed chassis
	//chassisType := checkChassisType(t, dut)

	ctx := context.Background()

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
		ctx: ctx,
	}

	t.Run("Test drop block", func(t *testing.T) {
		args.testocDropBlock(t)
	})
}
