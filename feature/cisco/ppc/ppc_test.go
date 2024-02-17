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
	"math"
	"sort"
	"strings"
	"testing"
	"time"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/gribi"
	"github.com/openconfig/featureprofiles/internal/cisco/ha/runner"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	gnps "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/schemaless"
	"github.com/openconfig/ygnmi/ygnmi"
)

var (
	chassis_type                  string // check if its distributed or fixed chassis
	tolerance                     float64
	rpfo_count                    = 0 // used to track rpfo_count if its more than 10 then reset to 0 and reload the HW
	multiple_subscription         = 5 // total 5 parallel subscriptions will be tested
	multiple_subscription_runtime = 5 // for 5 mins multiple subscriptions will keep on running
)

const (
	with_RPFO      = true
	with_lc_reload = true
	active_rp      = "0/RP0/CPU0"
	standby_rp     = "0/RP1/CPU0"
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

// sortPorts sorts the ports by the testbed port ID.
func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})
	return ports
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func (args *testArgs) interfaceToNPU(t testing.TB) []string {
	dut := ondatra.DUT(t, "dut")
	var temp, npus []string
	uniqueMap := make(map[string]bool)

	for _, port := range sortPorts(dut.Ports())[1:] {
		hwport := gnmi.Get(t, args.dut, gnmi.OC().Interface(port.Name()).HardwarePort().State())
		temp = append(temp, gnmi.Get(t, args.dut, gnmi.OC().Component(hwport).Parent().State()))
	}

	for _, str := range temp {
		// Check if the string is not already in the map
		if _, ok := uniqueMap[str]; !ok {
			uniqueMap[str] = true
			npus = append(npus, str)
		}
	}
	return npus
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
				if *result.CpuUtilization > 80 {
					t.Logf("%s %s CPU Process utilization high for process %-10s, utilization: %3d%%", timestamp, deviceName, processName, result.GetCpuUsageSystem())
				} else {
					t.Logf("%s %s INFO: CPU process %-10s utilization: %3d%%", timestamp, deviceName, processName, result.GetCpuUsageSystem())
				}
				if result.MemoryUtilization != nil {
					t.Logf("%s %s Memory high for process: %-10s - Utilization: %3d%%", timestamp, deviceName, processName, result.GetMemoryUsage())
				} else {
					t.Logf("%s %s INFO:  Memory Process %-10s utilization: %3d%%", timestamp, deviceName, processName, result.GetMemoryUsage())
				}
			}
			// sleep for 30 seconds before checking cpu/memory again
			time.Sleep(30 * time.Second)
		}
	}()
}

// extend it to run p4rt in background as well
func runMultipleClientBackground(t *testing.T) {
	t.Logf("running multiple client like gribi in the background")
	dut := ondatra.DUT(t, "dut")
	client := gribi.Client{
		DUT:                   dut,
		FibACK:                true,
		Persistence:           true,
		InitialElectionIDLow:  1,
		InitialElectionIDHigh: 0,
	}
	defer client.Close(t)
	if err := client.Start(t); err != nil {
		t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
		if err = client.Start(t); err != nil {
			t.Fatalf("gRIBI Connection could not be established: %v", err)
		}
	}
	go func() {
		for {
			client.BecomeLeader(t)
			client.FlushServer(t)
			time.Sleep(10 * time.Second)
			ciscoFlags.GRIBIChecks.AFTCheck = false
			client.AddNH(t, 3, ateDst.IPv4, "DEFAULT", "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
			client.AddNHG(t, 3, 0, map[uint64]uint64{3: 30}, "DEFAULT", false, ciscoFlags.GRIBIChecks)
			client.AddIPv4(t, "10.1.0.1/32", 3, vrf1, "DEFAULT", false, ciscoFlags.GRIBIChecks)

			client.AddNH(t, 2, "DecapEncap", "DEFAULT", "TE", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: "222.222.222.222", Dest: []string{"10.1.0.1"}})
			client.AddNHG(t, 2, 0, map[uint64]uint64{2: 100}, "DEFAULT", false, ciscoFlags.GRIBIChecks)
			client.AddIPv4(t, "192.0.2.40/32", 2, "DEFAULT", "", false, ciscoFlags.GRIBIChecks)

			client.AddNH(t, 1, "192.0.2.40", "DEFAULT", "", "", false, ciscoFlags.GRIBIChecks)
			client.AddNHG(t, 1, 0, map[uint64]uint64{1: 20}, "DEFAULT", false, ciscoFlags.GRIBIChecks)
			client.AddIPv4(t, "198.51.100.1/32", 1, vrf1, "DEFAULT", false, ciscoFlags.GRIBIChecks)

			//reprogram every 30 seconds to add churn
			time.Sleep(30 * time.Second)
		}
	}()
	time.Sleep(30 * time.Second)
}

type triggerType interface {
}

type trigger_process_restart struct {
	processes []string
}

func (tt trigger_process_restart) restartProcessBackground(t *testing.T, ctx context.Context) {
	dut := ondatra.DUT(t, "dut")
	for _, process := range tt.processes {

		//patch for CLIviaSSH failing, else pattern to use is #
		var acp string
		if with_RPFO {
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

type trigger_rpfo struct {
	tolerance float64
}

func (tt trigger_rpfo) rpfo(t *testing.T, ctx context.Context, reload bool) {
	dut := ondatra.DUT(t, "dut")
	// reload the HW is rfpo count is 10 or more
	if rpfo_count == 10 || reload {
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
		rpfo_count = 0
		if chassis_type == "distributed" {
			time.Sleep(time.Minute * 20)
		} else {
			time.Sleep(time.Minute * 10)
		}
	}
	// supervisor info
	if chassis_type == "distributed" {
		var supervisors []string
		active_state := gnmi.OC().Component(active_rp).Name().State()
		active := gnmi.Get(t, dut, active_state)
		standby_state := gnmi.OC().Component(standby_rp).Name().State()
		standby := gnmi.Get(t, dut, standby_state)
		supervisors = append(supervisors, active, standby)

		// find active and standby RP
		rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyRP(t, dut, supervisors)
		t.Logf("Detected activeRP: %v, standbyRP: %v", rpActiveBeforeSwitch, rpStandbyBeforeSwitch)

		// make sure standby RP is reachable
		switchoverReady := gnmi.OC().Component(rpActiveBeforeSwitch).SwitchoverReady()
		gnmi.Await(t, dut, switchoverReady.State(), 30*time.Minute, true)
		t.Logf("SwitchoverReady().Get(t): %v", gnmi.Get(t, dut, switchoverReady.State()))
		if got, want := gnmi.Get(t, dut, switchoverReady.State()), true; got != want {
			t.Errorf("switchoverReady.Get(t): got %v, want %v", got, want)
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
		if got, want := gnmi.Lookup(t, dut, activeRP.LastSwitchoverTime().State()).IsPresent(), true; got != want {
			t.Errorf("activeRP.LastSwitchoverTime().Lookup(t).IsPresent(): got %v, want %v", got, want)
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
}

type trigger_lc_reload struct {
	tolerance float64
}

func (tt trigger_lc_reload) lc_reload(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ls := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD)

	for _, l := range ls {
		t.Run(l, func(t *testing.T) {
			empty, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(l).Empty().State()).Val()
			if ok && empty {
				t.Skipf("Linecard Component %s is empty, hence skipping", l)
			}
			if !gnmi.Get(t, dut, gnmi.OC().Component(l).Removable().State()) {
				t.Skipf("Skip the test on non-removable linecard.")
			}

			oper := gnmi.Get(t, dut, gnmi.OC().Component(l).OperStatus().State())

			if got, want := oper, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE; got != want {
				t.Skipf("Linecard Component %s is already INACTIVE, hence skipping", l)
			}

			gnoiClient := dut.RawAPIs().GNOI(t)
			useNameOnly := deviations.GNOISubcomponentPath(dut)
			lineCardPath := components.GetSubcomponentPath(l, useNameOnly)
			rebootSubComponentRequest := &gnps.RebootRequest{
				Method: gnps.RebootMethod_COLD,
				Subcomponents: []*tpb.Path{
					// {
					//  Elem: []*tpb.PathElem{{Name: lc}},
					// },
					lineCardPath,
				},
			}
			t.Logf("rebootSubComponentRequest: %v", rebootSubComponentRequest)
			rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootSubComponentRequest)
			if err != nil {
				t.Fatalf("Failed to perform line card reboot with unexpected err: %v", err)
			}
			t.Logf("gnoiClient.System().Reboot() response: %v, err: %v", rebootResponse, err)

			// sleep while lc reloads
			time.Sleep(10 * time.Minute)
		})
	}
}

// Extend triggers
var (
	triggers = []Testcase{
		{
			name: "Process restart",
			// restart npu_drvr from linux prompt, ofa_npd on LC since they'll cause router to reload and that is covered in RPFO tc
			// fib_mgr restart will reload the fixed chassis
			desc:         "restart the process emsd, ifmgr, dbwriter, dblistener, fib_mgr, ipv4/ipv6 rib, isis  and validate pipeline counters",
			trigger_type: &trigger_process_restart{processes: []string{"ifmgr", "db_writer", "db_listener", "emsd", "ipv4_rib", "ipv6_rib", "isis"}},
		},
		{
			name:         "RPFO",
			desc:         "perform RPFO and validate pipeline counters",
			trigger_type: &trigger_rpfo{tolerance: 40}, // for fix chassis rfpo is reload and hence tolerance is needed
		},
		{
			name:         "LC reload",
			desc:         "perform LC reload and validate pipeline counters",
			trigger_type: &trigger_lc_reload{tolerance: 40}, //when LC is reloading, component is missing and indeed no data will be collected hence tolerance is needed
		},
	}
)

func checkChassisType(t *testing.T, dut *ondatra.DUTDevice) {
	cs := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD)
	if len(cs) < 2 {
		chassis_type = "fixed"
	} else {
		chassis_type = "distributed"
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

type event_interface_config struct {
	config bool
	shut   bool
	mtu    int
	port   []*ondatra.Port
}

func (ia event_interface_config) interface_config(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	cliPath, err := schemaless.NewConfig[string]("", "cli")
	if err != nil {
		t.Fatalf("Failed to create CLI ygnmi query: %v", err)
	}
	for _, port := range ia.port {
		if ia.config {
			if ia.shut {
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Enabled().Config(), false)
				}); errMsg != nil {
					gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Enabled().Config(), false)
				}
			}
			if ia.mtu != 0 {
				mtu := fmt.Sprintf("interface bundle-Ether 121 mtu %d", ia.mtu)
				gnmi.Update(t, dut, cliPath, mtu)
			}
		} else {
			//following reload need to try twice
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Enabled().Config(), true)
			}); errMsg != nil {
				gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Enabled().Config(), true)
			}
			if ia.mtu != 0 {
				mtu := fmt.Sprintf("no interface bundle-Ether 121 mtu %d", ia.mtu)
				gnmi.Update(t, dut, cliPath, mtu)
			}
		}

	}
}

type event_static_route_to_null struct {
	prefix string
	config bool
}

func (ia event_static_route_to_null) static_route_to_null(t *testing.T) {
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

type event_enable_mpls_ldp struct {
	config bool
}

func (ia event_enable_mpls_ldp) enable_mpls_ldp(t *testing.T) {
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
	flow *ondatra.Flow
	// sub_type   SubscriptionType
	event_type   eventType   // events for creating the scenario
	trigger_type triggerType // triggers
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

func (a *testArgs) testOC_drop_block(t *testing.T) {

	test := []Testcase{
		{
			name:       "drop/lookup-block/state/no-route",
			flow:       a.createFlow("valid_stream", []ondatra.Endpoint{a.top.Interfaces()["ateDst"]}, &TGNoptions{ipv4: true}),
			event_type: &event_interface_config{config: true, shut: true, port: sortPorts(a.dut.Ports())[1:]},
		},
		{
			name:       "drop/lookup-block/state/no-nexthop",
			flow:       a.createFlow("valid_stream", []ondatra.Endpoint{a.top.Interfaces()["ateDst"]}, &TGNoptions{ipv4: true}),
			event_type: &event_static_route_to_null{prefix: "202.1.0.1/32", config: true},
		},
		{
			name:       "drop/lookup-block/state/no-label",
			flow:       a.createFlow("valid_stream", []ondatra.Endpoint{a.top.Interfaces()["ateDst"]}, &TGNoptions{mpls: true}),
			event_type: &event_enable_mpls_ldp{config: true},
		},
		{
			name: "drop/lookup-block/state/incorrect-software-state",
			flow: a.createFlow("valid_stream", []ondatra.Endpoint{a.top.Interfaces()["ateDst"]}, &TGNoptions{mpls: true}),
		},
		{
			name: "drop/lookup-block/state/invalid-packet",
			flow: a.createFlow("valid_stream", []ondatra.Endpoint{a.top.Interfaces()["ateDst"]}, &TGNoptions{ipv4: true, ttl: true}),
		},
		{
			name:       "drop/lookup-block/state/fragment-total-drops",
			flow:       a.createFlow("valid_stream", []ondatra.Endpoint{a.top.Interfaces()["ateDst"]}, &TGNoptions{ipv4: true, frame_size: 1400}),
			event_type: &event_interface_config{config: true, mtu: 500, port: sortPorts(a.dut.Ports())[1:]},
		},
		// {
		// 	name: "drop/lookup-block/state/acl-drops",
		// 	CSCwi94987,
		// },
		// {
		// 	name: "drop/lookup-block/state/lookup-aggregate",
		// 	flow: a.createFlow("valid_stream", []ondatra.Endpoint{a.top.Interfaces()["ateDst"]}, &TGNoptions{ipv4: true, fps: 1000000000}),
		// },
	}

	npus := a.interfaceToNPU(t)                          // collecting all the destination NPUs
	data := make(map[string]ygnmi.WildcardQuery[uint64]) //holds path and its query information

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			var pre_counters, post_counters uint64
			pre_counters, post_counters = 0, 0

			tolerance = 2.0 // 2% change tolerance is allowed between want and got value

			// collecting each path, query per destination NPU
			for _, npu := range npus {
				path := fmt.Sprintf("/components/component[name=%s]/integrated-circuit/pipeline-counters/%s", npu, tt.name)
				query, _ := schemaless.NewWildcard[uint64](path, "openconfig")
				data[path] = query
			}

			// running mulitple subscriptions on all the queries while tc is executed
			for _, query := range data {
				multiple_subscriptions(t, query)
			}

			//aggregrate pre counters for a path across all the destination NPUs
			for path, query := range data {
				pre, _ := get_data(t, path, query)
				pre_counters = pre_counters + pre
			}

			tgn_data := float64(a.validateTrafficFlows(t, tt.flow, &TGNoptions{traffic_timer: 120, drop: true, event: tt.event_type}))

			//aggregrate post counters for a path across all the destination NPUs
			for path, query := range data {
				post, _ := get_data(t, path, query)
				post_counters = post_counters + post
			}

			// following reload, we can have pre data bigger than post indeed using absolute value
			got := math.Abs(float64(post_counters - pre_counters))

			t.Logf("Initial counters for path %s : %d", tt.name, pre_counters)
			t.Logf("Final counters for path %s: %d", tt.name, post_counters)
			t.Logf("Expected counters for path %s: %f", tt.name, got)

			if (math.Abs(tgn_data-got)/(tgn_data))*100 > tolerance {
				t.Errorf("Data doesn't match for path %s, got: %f, want: %f", tt.name, got, tgn_data)
			} else {
				t.Logf("Data for path %s, got: %f, want: %f", tt.name, got, tgn_data)
			}
		})
	}
}

func get_data(t *testing.T, path string, query ygnmi.WildcardQuery[uint64]) (uint64, error) {
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

func multiple_subscriptions(t *testing.T, query ygnmi.WildcardQuery[uint64]) {
	dut := ondatra.DUT(t, "dut")
	for i := 1; i <= multiple_subscription; i++ {
		gnmi.CollectAll(t, dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_SAMPLE), ygnmi.WithSampleInterval(30*time.Second)), query, time.Duration(multiple_subscription_runtime)*time.Minute)
	}
	// sleeping while all the concurrent subscriptions are executed
	// time.Sleep(time.Duration(multiple_subscription_runtime) * time.Minute)
}

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
	// only running for distributed tb since rpfo event for fix is equal to reload and during reload
	// we need to kill go routine else cpu/mem call will fail with EOF
	if chassis_type == "distributed" {
		runBackgroundMonitor(t)
	}

	//starting other clients running in the backgroun
	// runMultipleClientBackground(t)

	dut := ondatra.DUT(t, "dut")

	//Determine if its fixed or distributed chassis
	checkChassisType(t, dut)

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
		args.testOC_drop_block(t)
	})
}
