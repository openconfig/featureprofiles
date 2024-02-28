// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
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
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
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
	"github.com/openconfig/ygot/ygot"
)

const (
	dst                   = "202.1.0.1"
	v4mask                = "32"
	dstCount              = 1
	innersrcPfx           = "200.1.0.1"
	totalBgpPfx           = 1           //set value for scale bgp setup ex: 100000
	innerdstpfxminBgp     = "202.1.0.1" // innerdstpfxminBgp
	innerdstPfxCount_bgp  = 1           //set value for number of inner prefix for bgp flow
	totalisisPfx          = 1           //set value for scale isis setup ex: 10000
	innerdstPfxMin_isis   = "201.1.0.1"
	innerdstPfxCount_isis = 1 //set value for number of inner prefix for isis flow
	ipv4PrefixLen         = 30
	ipv6PrefixLen         = 126
	policyTypeIsis        = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS
	dutAreaAddress        = "47.0001"
	dutSysId              = "0000.0000.0001"
	isisName              = "osiris"
	policyTypeBgp         = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP
	bgpAs                 = 65000
)

// Testcase defines testcase structure
type Testcase struct {
	name string
	desc string
	flow *ondatra.Flow
	// sub_type   SubscriptionType
	eventType   eventType   // events for creating the scenario
	triggerType triggerType // triggers
}

// Extend triggers
var triggers = []Testcase{
	{
		name: "Process restart",
		// restart npu_drvr from linux prompt, ofa_npd on LC since they'll cause router to reload and that is covered in RPFO tc
		// fib_mgr restart will reload the fixed chassis
		desc:        "restart the process emsd, ifmgr, dbwriter, dblistener, fib_mgr, ipv4/ipv6 rib, isis  and validate pipeline counters",
		triggerType: &triggerProcessRestart{processes: []string{"ifmgr", "db_writer", "db_listener", "emsd", "ipv4_rib", "ipv6_rib", "isis"}},
	},
	{
		name:        "RPFO",
		desc:        "perform RPFO and validate pipeline counters",
		triggerType: &triggerRpfo{tolerance: 40}, // for fix chassis rfpo is reload and hence tolerance is needed
	},
	{
		name:        "LC reload",
		desc:        "perform LC reload and validate pipeline counters",
		triggerType: &triggerLcReload{tolerance: 40}, //when LC is reloading, component is missing and indeed no data will be collected hence tolerance is needed
	},
}

// Extend triggers
var futureTriggers = []Testcase{
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

type triggerType interface {
}

type SubscriptionType interface {
	isSubscriptionType()
}

type subscriptionArgs struct {
	streamMode     gpb.SubscriptionMode
	sampleInterval time.Duration
}

// subMode represents type of STREAMING subscription mode
// TODO - support levels and sub modes
func (sa subscriptionArgs) multipleSubscriptions(t *testing.T, query ygnmi.WildcardQuery[uint64]) {
	dut := ondatra.DUT(t, "dut")
	// once, poll, stream
	// sample, on-change, target-defined
	for i := 1; i <= subscriptionCount; i++ {
		gnmi.CollectAll(t, dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(sa.streamMode), ygnmi.WithSampleInterval(sa.sampleInterval)), query, multipleSubscriptionRuntime*time.Minute)
	}
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

type eventType interface {
}

type eventAclConfig struct {
	aclName string
	config  bool
}

func (eventArgs eventAclConfig) aclConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	cliPath, err := schemaless.NewConfig[string]("", "cli")
	if err != nil {
		t.Fatalf("Failed to create CLI ygnmi query: %v", err)
	}
	var aclConfig string
	if eventArgs.config {
		aclConfig = fmt.Sprintf("ipv4 access-list %v 1 deny any", eventArgs.aclName)
	} else {
		aclConfig = fmt.Sprintf("no ipv4 access-list %v 1 deny any", eventArgs.aclName)
	}
	gnmi.Update(t, dut, cliPath, aclConfig)
}

type eventInterfaceConfig struct {
	config bool
	shut   bool
	mtu    int
	port   []*ondatra.Port
}

func (eventArgs eventInterfaceConfig) interfaceConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	cliPath, err := schemaless.NewConfig[string]("", "cli")
	if err != nil {
		t.Fatalf("Failed to create CLI ygnmi query: %v", err)
	}
	for _, port := range eventArgs.port {
		if eventArgs.config {
			if eventArgs.shut {
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Enabled().Config(), false)
				}); errMsg != nil {
					gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Enabled().Config(), false)
				}
			}
			if eventArgs.mtu != 0 {
				mtu := fmt.Sprintf("interface bundle-Ether 121 mtu %d", eventArgs.mtu)
				gnmi.Update(t, dut, cliPath, mtu)
			}
		} else {
			//following reload need to try twice
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Enabled().Config(), true)
			}); errMsg != nil {
				gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Enabled().Config(), true)
			}
			if eventArgs.mtu != 0 {
				mtu := fmt.Sprintf("no interface bundle-Ether 121 mtu %d", eventArgs.mtu)
				gnmi.Update(t, dut, cliPath, mtu)
			}
		}

	}
}

type eventStaticRouteToNull struct {
	prefix string
	config bool
}

func (eventArgs eventStaticRouteToNull) staticRouteToNull(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	cliPath, err := schemaless.NewConfig[string]("", "cli")
	if err != nil {
		t.Fatalf("Failed to create CLI ygnmi query: %v", err)
	}
	var static_route string
	if eventArgs.config {
		static_route = fmt.Sprintf("router static address-family ipv4 unicast %s null 0", eventArgs.prefix)
	} else {
		static_route = fmt.Sprintf("no router static address-family ipv4 unicast %s null 0", eventArgs.prefix)
	}
	gnmi.Update(t, dut, cliPath, static_route)

}

type eventEnableMplsLdp struct {
	config bool
}

func (eventArgs eventEnableMplsLdp) enableMplsLdp(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	cliPath, err := schemaless.NewConfig[string]("", "cli")
	if err != nil {
		t.Fatalf("Failed to create CLI ygnmi query: %v", err)
	}
	var mpls_ldp string
	if eventArgs.config {
		mpls_ldp = "mpls ldp interface bundle-Ether 120"
	} else {
		mpls_ldp = "no mpls ldp"
	}
	gnmi.Update(t, dut, cliPath, mpls_ldp)

}

type triggerProcessRestart struct {
	processes []string
}

func (triggerArgs triggerProcessRestart) restartProcessBackground(t *testing.T, ctx context.Context) {
	dut := ondatra.DUT(t, "dut")
	for _, process := range triggerArgs.processes {

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
	tolerance float64
}

func (triggerArgs triggerRpfo) rpfo(t *testing.T, ctx context.Context, reload bool) {
	dut := ondatra.DUT(t, "dut")
	// reload the HW is rfpo count is 10 or more
	if rpfoCount == 10 || reload {
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
		if chassisType == "distributed" {
			time.Sleep(time.Minute * 20) // TODO - why 20 minutes?
		} else {
			time.Sleep(time.Minute * 10) // TODO - why 20 minutes?
		}
	}
	// supervisor info
	if chassisType == "distributed" {
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
			t.Errorf("activeRP.LastSwitchoverTime().Lookup(t).IsPresent(): got %v, want %v", got, want)
		} else {
			t.Logf("Found activeRP.LastSwitchoverTime(): %v", gnmi.Get(t, dut, activeRP.LastSwitchoverTime().State()))
		}

		if got := gnmi.Lookup(t, dut, activeRP.LastSwitchoverReason().State()).IsPresent(); got != true {
			t.Errorf("activeRP.LastSwitchoverReason().Lookup(t).IsPresent(): got %v, want %v", got, want)
		} else {
			lastSwitchoverReason := gnmi.Get(t, dut, activeRP.LastSwitchoverReason().State())
			t.Logf("Found lastSwitchoverReason.GetDetails(): %v", lastSwitchoverReason.GetDetails())
			t.Logf("Found lastSwitchoverReason.GetTrigger().String(): %v", lastSwitchoverReason.GetTrigger().String())
		}
	}
}

type triggerLcReload struct {
	tolerance float64
}

func (triggerArgs triggerLcReload) lcReload(t *testing.T) {
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
			time.Sleep(10 * time.Minute) // TODO - handle via polling
		})
	}
}

// sortPorts sorts the given slice of ports by the testbed port ID in ascending order.
func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})
	return ports
}

func (args *testArgs) checkChassisType(t *testing.T, dut *ondatra.DUTDevice) string {
	cs := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD)
	if len(cs) < 2 {
		return "fixed"
	} else {
		return "distributed"
	}
}

// interfaceToNPU returns a slice of unique NPU (Network Processing Unit) names
// associated with the hardware ports of a DUT (Device Under Test).
func (args *testArgs) interfaceToNPU(t testing.TB) []string {
	var npus []string
	uniqueMap := make(map[string]bool)

	// Get hardware ports and corresponding components
	ports := sortPorts(args.dut.Ports())[1:]
	for _, port := range ports {
		hwPort := gnmi.Get(t, args.dut, gnmi.OC().Interface(port.Name()).HardwarePort().State())
		component := gnmi.Get(t, args.dut, gnmi.OC().Component(hwPort).Parent().State())
		// Check if the component is not already in the map
		if _, ok := uniqueMap[component]; !ok {
			uniqueMap[component] = true
			npus = append(npus, component)
		}
	}
	return npus
}

// TODO - raise TZs for the leaves and source of truth
func runBackgroundMonitor(t *testing.T, stop <-chan struct{}, done chan<- struct{}) {
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
		defer func() {
			if r := recover(); r != nil {
				t.Logf("Recovered from panic in runBackgroundMonitor: %v", r)
			}
			done <- struct{}{}
		}()
	Loop:
		for {
			select {
			case <-stop:
				break Loop
			default:
				for _, process := range pID {
					query := gnmi.OC().System().Process(process).State()
					timestamp := time.Now().Round(time.Second)
					result := gnmi.Get(t, dut, query)
					processName := result.GetName()
					if *result.CpuUtilization > 80 {
						// TODO - add a tracking flag for breach; with timer, keep polling the status; check with Takenaga what is the expected behavior
						t.Logf("%s %s CPU Process utilization high for process %-10s, utilization: %3d%%", timestamp, deviceName, processName, result.GetCpuUsageSystem())
					} else {
						t.Logf("%s %s INFO: CPU process %-10s utilization: %3d%%", timestamp, deviceName, processName, result.GetCpuUsageSystem())
					}
					if result.MemoryUtilization != nil {
						// TODO - check with Maya DE
						// TODO - check both leaves if they are returning valid values
						// TODO - check what Memory Utilization and CPU util are mapped to? even mem and cpu usage - where are they mapped to?
						t.Logf("%s %s Memory high for process: %-10s - Utilization: %3d%%", timestamp, deviceName, processName, result.GetMemoryUsage())
					} else {
						t.Logf("%s %s INFO:  Memory Process %-10s utilization: %3d%%", timestamp, deviceName, processName, result.GetMemoryUsage())
					}
				}
				// sleep for 30 seconds before checking cpu/memory again
				time.Sleep(30 * time.Second)
			}
		}
		done <- struct{}{}
	}()
}

// extend it to run p4rt in background as well
func runMultipleClientBackground(t *testing.T, stop <-chan struct{}, done chan<- struct{}) {
	t.Logf("running multiple client like gribi in the background")
	dut := ondatra.DUT(t, "dut")
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("Recovered from panic in runMultipleClientBackground: %v", r)
			}
			done <- struct{}{}
		}()
	Loop:
		for {
			select {
			case <-stop:
				break Loop
			default:
				client := gribi.Client{
					DUT:                   dut,
					FibACK:                true,
					Persistence:           true,
					InitialElectionIDLow:  1,
					InitialElectionIDHigh: 0,
				}
				if err := client.Start(t); err != nil {
					t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
					if err = client.Start(t); err != nil {
						t.Errorf("gRIBI Connection could not be established: %v", err)
					}
				}
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

				client.Close(t)
				//reprogram every 30 seconds to add churn
				time.Sleep(30 * time.Second)
			}
		}
		done <- struct{}{}
	}()
}

// TGNoptions are optional parameters to a validate traffic function.
type TGNoptions struct {
	drop, mpls, ipv4, ttl bool
	traffic_timer         int
	fps                   uint64
	fpercent              float64
	frame_size            uint32
	event                 eventType
}

// configureATE configures ports on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	atePorts := sortPorts(ate.Ports())
	top := ate.Topology().New()

	ateSource := atePorts[0]
	i1 := top.AddInterface(ateSrc.Name).WithPort(ateSource)
	i1.IPv4().
		WithAddress(ateSrc.IPv4CIDR()).
		WithDefaultGateway(dutSrc.IPv4)
	i1.IPv6().
		WithAddress(ateSrc.IPv6CIDR()).
		WithDefaultGateway(dutSrc.IPv6)

	i2 := top.AddInterface(ateDst.Name)
	lag := top.AddLAG("lag").WithPorts(atePorts[1:]...)
	lag.LACP().WithEnabled(true)
	i2.WithLAG(lag)

	// Disable FEC for 100G-FR ports because Novus does not support it.
	if ateSource.PMD() == ondatra.PMD100GBASEFR {
		i1.Ethernet().FEC().WithEnabled(false)
	}
	is100gfr := false
	for _, p := range atePorts[1:] {
		if p.PMD() == ondatra.PMD100GBASEFR {
			is100gfr = true
		}
	}
	if is100gfr {
		i2.Ethernet().FEC().WithEnabled(false)
	}
	top.Push(t).StartProtocols(t)

	i2.IPv4().
		WithAddress(ateDst.IPv4CIDR()).
		WithDefaultGateway(dutDst.IPv4)
	i2.IPv6().
		WithAddress(ateDst.IPv6CIDR()).
		WithDefaultGateway(dutDst.IPv6)
	top.Update(t)
	top.StartProtocols(t)
	return top
}

// configAteIsisL2 configures ISIS L2 ATE config
func configAteIsisL2(t *testing.T, topo *ondatra.ATETopology, atePort, areaId, networkName string, metric uint32, v4prefix string, count uint32) {
	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	network := intfs[atePort].AddNetwork(networkName)
	network.ISIS().WithIPReachabilityMetric(metric + 1)
	network.IPv4().WithAddress(v4prefix).WithCount(count)
	rNetwork := intfs[atePort].AddNetwork("recursive")
	rNetwork.ISIS().WithIPReachabilityMetric(metric + 1)
	rNetwork.IPv4().WithAddress("100.100.100.100/32")
	intfs[atePort].ISIS().WithAreaID(areaId).WithLevelL2().WithNetworkTypePointToPoint().WithMetric(metric).WithWideMetricEnabled(true)
}

// configAteEbgpPeer configures EBGP ATE config
func configAteEbgpPeer(t *testing.T, topo *ondatra.ATETopology, atePort, peerAddress string, localAsn uint32, networkName, nextHop, prefix string, count uint32, useLoopback bool) {

	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}

	network := intfs[atePort].AddNetwork(networkName)
	bgpAttribute := network.BGP()
	bgpAttribute.WithActive(true).WithNextHopAddress(nextHop)

	// Add prefixes, Add network instance
	if prefix != "" {
		network.IPv4().WithAddress(prefix).WithCount(count)
	}
	// Create BGP instance
	bgp := intfs[atePort].BGP()
	bgpPeer := bgp.AddPeer().WithPeerAddress(peerAddress).WithLocalASN(localAsn).WithTypeExternal()
	bgpPeer.WithOnLoopback(useLoopback)

	// Update BGP Capabilities
	bgpPeer.Capabilities().WithIPv4UnicastEnabled(true).WithIPv6UnicastEnabled(true).WithGracefulRestart(true)
}

// configAteRoutingProtocols calls ISIS/BGP api
func configAteRoutingProtocols(t *testing.T, top *ondatra.ATETopology) {
	//advertising 100.100.100.100/32 for bgp resolve over IGP prefix
	intfs := top.Interfaces()
	intfs["ateDst"].WithIPv4Loopback("100.100.100.100/32")
	configAteIsisL2(t, top, "ateDst", "B4", "isis_network", 20, innerdstPfxMin_isis+"/"+v4mask, totalisisPfx)
	configAteEbgpPeer(t, top, "ateDst", dutDst.IPv4, 64001, "bgp_recursive", ateDst.IPv4, innerdstpfxminBgp+"/"+v4mask, totalBgpPfx, true)
	top.Push(t).StartProtocols(t)
}

// createFlow returns a flow from atePort1 to the dstPfx, expected to arrive on ATE interface dst.
func (args *testArgs) createFlow(name string, dstEndPoint []ondatra.Endpoint, opts ...*TGNoptions) *ondatra.Flow {
	srcEndPoint := args.top.Interfaces()[ateSrc.Name]
	var flow *ondatra.Flow
	var header []ondatra.Header

	for _, opt := range opts {
		if opt.mpls {
			hdr_mpls := ondatra.NewMPLSHeader()
			header = []ondatra.Header{ondatra.NewEthernetHeader(), hdr_mpls}
		}
		if opt.ipv4 {
			var hdr_ipv4 *ondatra.IPv4Header
			// explicity set ttl 0 if zero
			if opt.ttl {
				hdr_ipv4 = ondatra.NewIPv4Header().WithTTL(0)
			} else {
				hdr_ipv4 = ondatra.NewIPv4Header()
			}
			hdr_ipv4.WithSrcAddress(dutSrc.IPv4).DstAddressRange().WithMin(dst).WithCount(dstCount).WithStep("0.0.0.1")
			header = []ondatra.Header{ondatra.NewEthernetHeader(), hdr_ipv4}
		}
	}
	flow = args.ate.Traffic().NewFlow(name).
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint...).
		WithHeaders(header...)

	if opts[0].fps != 0 {
		flow.WithFrameRateFPS(opts[0].fps)
	} else {
		flow.WithFrameRateFPS(1000)
	}

	flow.WithFrameRatePct(100)
	if opts[0].frame_size != 0 {
		flow.WithFrameSize(opts[0].frame_size)
	} else if opts[0].fpercent != 0 {
		flow.WithFrameRatePct((opts[0].fpercent))
	} else {
		flow.WithFrameSize(300)
	}

	return flow
}

// validateTrafficFlows validates traffic loss on tgn side and DUT incoming and outgoing counters
func (args *testArgs) validateTrafficFlows(t *testing.T, flow *ondatra.Flow, opts ...*TGNoptions) uint64 {
	args.ate.Traffic().Start(t, flow)
	// run traffic for 30 seconds, before introducing fault
	time.Sleep(time.Duration(30) * time.Second)

	// Set configs if needed for scenario
	for _, op := range opts {
		if eventAction, ok := op.event.(*eventInterfaceConfig); ok {
			eventAction.interfaceConfig(t)
		} else if eventAction, ok := op.event.(*eventStaticRouteToNull); ok {
			eventAction.staticRouteToNull(t)
		} else if eventAction, ok := op.event.(*eventEnableMplsLdp); ok {
			eventAction.enableMplsLdp(t)
		}
	}
	// TODO - uncomment
	//// close all the existing goroutine for the trigger
	//close(stopMonitor)
	//close(stopClients)
	//<-doneMonitor
	//<-doneClients

	// Space to add trigger code
	for _, tt := range triggers {
		t.Logf("Name: %s", tt.name)
		t.Logf("Description: %s", tt.desc)
		if triggerAction, ok := tt.triggerType.(*triggerProcessRestart); ok {
			triggerAction.restartProcessBackground(t, args.ctx)
		}
		if chassisType == "distributed" && withRpfo {
			if triggerAction, ok := tt.triggerType.(*triggerRpfo); ok {
				// false is for not reloading the box, since there is standby RP on distributed tb, we don't do a reload
				triggerAction.rpfo(t, args.ctx, false)
			}
		} else if chassisType == "fixed" && withRpfo {
			if triggerAction, ok := tt.triggerType.(*triggerRpfo); ok {
				// true is for reloading the box, since there is no RPFO on fixed tb, we do a reload
				triggerAction.rpfo(t, args.ctx, true)
				tolerance = uint64(triggerAction.tolerance)
			}
		}
		if chassisType == "distributed" && withLcReload {
			if triggerAction, ok := tt.triggerType.(*triggerLcReload); ok {
				triggerAction.lcReload(t)
				tolerance = uint64(triggerAction.tolerance)
			}
		}
	}
	// TODO - uncomment
	//// restart goroutines
	//if chassisType == "distributed" {
	//	doneMonitorTrigger = make(chan struct{})
	//	stopMonitorTrigger = make(chan struct{})
	//	runBackgroundMonitor(t, stopMonitorTrigger, doneMonitorTrigger)
	//}
	////starting other clients running in the background
	//doneClientsTrigger = make(chan struct{})
	//stopClientsTrigger = make(chan struct{})
	//runMultipleClientBackground(t, stopClientsTrigger, doneClientsTrigger)

	time.Sleep(time.Duration(opts[0].traffic_timer) * time.Second)
	args.ate.Traffic().Stop(t)

	// remove set configs before further check
	for _, op := range opts {
		if _, ok := op.event.(*eventInterfaceConfig); ok {
			eventAction := eventInterfaceConfig{config: false, mtu: 1514, port: sortPorts(args.dut.Ports())[1:]}
			eventAction.interfaceConfig(t)
		} else if _, ok := op.event.(*eventStaticRouteToNull); ok {
			eventAction := eventStaticRouteToNull{prefix: "202.1.0.1/32", config: false}
			eventAction.staticRouteToNull(t)
		} else if _, ok := op.event.(*eventEnableMplsLdp); ok {
			eventAction := eventEnableMplsLdp{config: false}
			eventAction.enableMplsLdp(t)
		}
	}

	for _, op := range opts {
		if op.drop {
			in := gnmi.Get(t, args.ate, gnmi.OC().Flow(flow.Name()).Counters().InPkts().State())
			out := gnmi.Get(t, args.ate, gnmi.OC().Flow(flow.Name()).Counters().OutPkts().State())
			return uint64(out - in)
		}
	}
	return 0
}

type PBROptions struct {
	// BackupNHG specifies the backup next-hop-group to be used when all next-hops are unavailable.
	SrcIP string
}

var (
	dutSrc = attrs.Attributes{
		Desc:    "dutSrc",
		IPv4:    "100.121.1.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:1:1",
		IPv6Len: ipv6PrefixLen,
	}
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		IPv4:    "100.121.1.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:1:2",
		IPv6Len: ipv6PrefixLen,
	}
	dutDst = attrs.Attributes{
		Desc:    "dutDst",
		IPv4:    "100.122.1.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:122:1:1",
		IPv6Len: ipv6PrefixLen,
	}
	ateDst = attrs.Attributes{
		Name:    "ateDst",
		IPv4:    "100.122.1.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:122:1:2",
		IPv6Len: ipv6PrefixLen,
	}
	// dutPort2Vlan10 = attrs.Attributes{
	// 	Desc:    "dutPort2Vlan10",
	// 	IPv4:    "100.128.10.1",
	// 	IPv4Len: ipv4PrefixLen,
	// 	IPv6:    "2000::100:128:10:1",
	// 	IPv6Len: ipv6PrefixLen,
	// 	MTU:     vlanMTU,
	// }
	// atePort2Vlan10 = attrs.Attributes{
	// 	Name:    "atePort2Vlan10",
	// 	IPv4:    "100.128.10.2",
	// 	IPv4Len: ipv4PrefixLen,
	// 	IPv6:    "2000::100:128:10:2",
	// 	IPv6Len: ipv6PrefixLen,
	// 	MTU:     vlanMTU,
	// }
)

// configInterfaceDUT configures the interface with the Addrs.
func configInterfaceDUT(i *oc.Interface, a *attrs.Attributes) *oc.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(ipv4PrefixLen)

	s6 := s.GetOrCreateIpv6()
	s6a := s6.GetOrCreateAddress(a.IPv6)
	s6a.PrefixLength = ygot.Uint8(ipv6PrefixLen)

	return i
}

// configureDUT configures port1-port8 on DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dutPorts := sortPorts(dut.Ports())
	d := gnmi.OC()

	// incoming interface is Bundle-Ether120 with only 1 member (port1)
	incoming := &oc.Interface{Name: ygot.String("Bundle-Ether120")}
	gnmi.Replace(t, dut, d.Interface(*incoming.Name).Config(), configInterfaceDUT(incoming, &dutSrc))
	srcPort := dutPorts[0]
	dutSource := generateBundleMemberInterfaceConfig(t, srcPort.Name(), *incoming.Name)
	gnmi.Replace(t, dut, gnmi.OC().Interface(srcPort.Name()).Config(), dutSource)

	// outgoing interface is bundle-121 with 7 members (port2, port 3, port4, port5, port6, port7, port8)
	// lacp := &oc.Lacp_Interface{Name: ygot.String("Bundle-Ether121")}
	// lacp.LacpMode = oc.Lacp_LacpActivityType_ACTIVE
	// lacpPath := d.Lacp().Interface("Bundle-Ether121")
	// gnmi.Replace(t, dut, lacpPath.Config(), lacp)

	outgoing := &oc.Interface{Name: ygot.String("Bundle-Ether121")}
	outgoingData := configInterfaceDUT(outgoing, &dutDst)
	g := outgoingData.GetOrCreateAggregation()
	g.LagType = oc.IfAggregate_AggregationType_LACP
	gnmi.Replace(t, dut, d.Interface(*outgoing.Name).Config(), configInterfaceDUT(outgoing, &dutDst))
	for _, port := range dutPorts[1:] {
		dutDest := generateBundleMemberInterfaceConfig(t, port.Name(), *outgoing.Name)
		gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Config(), dutDest)
	}
}

// func createNameSpace(t *testing.T, dut *ondatra.DUTDevice, name, intfname string, subint uint32) {
// 	//create empty subinterface
// 	si := &oc.Interface_Subinterface{}
// 	si.Index = ygot.Uint32(subint)
// 	gnmi.Replace(t, dut, gnmi.OC().Interface(intfname).Subinterface(subint).Config(), si)

// 	//create vrf and apply on subinterface
// 	v := &oc.NetworkInstance{
// 		Name: ygot.String(name),
// 	}
// 	vi := v.GetOrCreateInterface(intfname + "." + strconv.Itoa(int(subint)))
// 	vi.Subinterface = ygot.Uint32(subint)
// 	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(name).Config(), v)
// }

// func getSubInterface(ipv4 string, prefixlen4 uint8, ipv6 string, prefixlen6 uint8, vlanID uint16, index uint32) *oc.Interface_Subinterface {
// 	s := &oc.Interface_Subinterface{}
// 	s.Index = ygot.Uint32(index)
// 	s4 := s.GetOrCreateIpv4()
// 	a := s4.GetOrCreateAddress(ipv4)
// 	a.PrefixLength = ygot.Uint8(prefixlen4)
// 	s6 := s.GetOrCreateIpv6()
// 	a6 := s6.GetOrCreateAddress(ipv6)
// 	a6.PrefixLength = ygot.Uint8(prefixlen6)
// 	v := s.GetOrCreateVlan()
// 	m := v.GetOrCreateMatch()
// 	if index != 0 {
// 		m.GetOrCreateSingleTagged().VlanId = ygot.Uint16(vlanID)
// 	}
// 	return s
// }

func generateBundleMemberInterfaceConfig(t *testing.T, name, bundleID string) *oc.Interface {
	t.Helper()
	i := &oc.Interface{Name: ygot.String(name)}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	e := i.GetOrCreateEthernet()
	e.AutoNegotiate = ygot.Bool(false)
	e.AggregateId = ygot.String(bundleID)
	return i
}

// configRoutePolicy configures route_policy for BGP
func configRoutePolicy(t *testing.T, dut *ondatra.DUTDevice) {
	dev := &oc.Root{}
	inst := dev.GetOrCreateRoutingPolicy()
	pdef := inst.GetOrCreatePolicyDefinition("ALLOW")
	stmt1, _ := pdef.AppendNewStatement("1")
	stmt1.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	dutNode := gnmi.OC().RoutingPolicy()
	dutConf := dev.GetOrCreateRoutingPolicy()
	gnmi.Update(t, dut, dutNode.Config(), dutConf)
}

// configIsis configures ISIS on DUT
func configIsis(t *testing.T, dut *ondatra.DUTDevice, intfName string) {
	dev := &oc.Root{}
	inst := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	prot := inst.GetOrCreateProtocol(policyTypeIsis, isisName)
	isis := prot.GetOrCreateIsis()
	glob := isis.GetOrCreateGlobal()
	glob.Net = []string{fmt.Sprintf("%v.%v.00", dutAreaAddress, dutSysId)}
	glob.LevelCapability = 2
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	intf := isis.GetOrCreateInterface(intfName)
	intf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
	intf.Enabled = ygot.Bool(true)
	intf.HelloPadding = 1
	intf.Passive = ygot.Bool(false)
	intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	level := isis.GetOrCreateLevel(2)
	level.MetricStyle = 2

	dutNode := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(policyTypeIsis, isisName)
	dutConf := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance).GetOrCreateProtocol(policyTypeIsis, isisName)
	gnmi.Update(t, dut, dutNode.Config(), dutConf)
}

// configBgp configures ISIS on DUT
func configBgp(t *testing.T, dut *ondatra.DUTDevice, neighbor string) {
	dev := &oc.Root{}
	inst := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	prot := inst.GetOrCreateProtocol(policyTypeBgp, *ciscoFlags.DefaultNetworkInstance)
	bgp := prot.GetOrCreateBgp()
	glob := bgp.GetOrCreateGlobal()
	glob.As = ygot.Uint32(bgpAs)
	glob.RouterId = ygot.String("1.1.1.1")
	glob.GetOrCreateGracefulRestart().Enabled = ygot.Bool(true)
	glob.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)

	pg := bgp.GetOrCreatePeerGroup("BGP-PEER-GROUP")
	pg.PeerAs = ygot.Uint32(64001)
	pg.LocalAs = ygot.Uint32(63001)
	pg.PeerGroupName = ygot.String("BGP-PEER-GROUP")

	peer := bgp.GetOrCreateNeighbor(neighbor)
	peer.PeerGroup = ygot.String("BGP-PEER-GROUP")
	peer.GetOrCreateEbgpMultihop().Enabled = ygot.Bool(true)
	peer.GetOrCreateEbgpMultihop().MultihopTtl = ygot.Uint8(255)
	peer.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	peer.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy().ImportPolicy = []string{"ALLOW"}
	peer.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy().ExportPolicy = []string{"ALLOW"}

	dutNode := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(policyTypeBgp, *ciscoFlags.DefaultNetworkInstance)
	dutConf := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance).GetOrCreateProtocol(policyTypeBgp, *ciscoFlags.DefaultNetworkInstance)
	gnmi.Update(t, dut, dutNode.Config(), dutConf)
}

func configVRF(t *testing.T, dut *ondatra.DUTDevice, vrfs []string) {
	for _, vrfName := range vrfs {
		vrf := &oc.NetworkInstance{
			Name: ygot.String(vrfName),
			Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
		}
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrfName).Config(), vrf)
	}
}

// configbasePBR creates class map, policy and configures them under source interface
func configbasePBR(t *testing.T, dut *ondatra.DUTDevice, networkInstance, ipType string, index uint32, pbrName string, protocol oc.E_PacketMatchTypes_IP_PROTOCOL, dscpSet []uint8, opts ...*PBROptions) {
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	r := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r.SequenceId = ygot.Uint32(index)
	r.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(networkInstance)}
	if ipType == "ipv4" {
		if len(opts) != 0 {
			for _, opt := range opts {
				if opt.SrcIP != "" {
					r.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
						SourceAddress: &opt.SrcIP,
						Protocol:      protocol,
					}
				}
			}
		} else {
			r.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
				Protocol: protocol,
			}
		}
		if len(dscpSet) > 0 {
			r.Ipv4.DscpSet = dscpSet
		}
	} else if ipType == "ipv6" {
		r.Ipv6 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv6{
			Protocol: protocol,
		}
		if len(dscpSet) > 0 {
			r.Ipv6.DscpSet = dscpSet
		}
	}
	pf := oc.NetworkInstance_PolicyForwarding{}
	p := pf.GetOrCreatePolicy(pbrName)
	p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
	err := p.AppendRule(&r)
	if err != nil {
		t.Error(err)
	}
	intf := pf.GetOrCreateInterface("Bundle-Ether120.0")
	intf.GetOrCreateInterfaceRef().Interface = ygot.String("Bundle-Ether120")
	intf.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	intf.ApplyVrfSelectionPolicy = ygot.String(pbrName)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).PolicyForwarding().Config(), &pf)
}

// getPathFromElements constructs a path string from a slice of PathElem elements.
// It iterates through each PathElem and concatenates the element names along with any key-value pairs.
// If a PathElem has key-value pairs, they are formatted as "[key=value]" and appended to the element name.
// The resulting path string is returned with "/" as the delimiter.
func getPathFromElements(input []*gpb.PathElem) string {
	var result []string
	for _, elem := range input {
		// If there are key-value pairs, add them to the element name
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

// TODO - support levels and sub modes
// getData retrieves data from a DUT using GNMI.
// It performs a one-time subscription to the specified path using a wildcard query.
func getData(t *testing.T, path string, query ygnmi.WildcardQuery[uint64]) (uint64, error) {
	dut := ondatra.DUT(t, "dut")

	data, _ := gnmi.WatchAll(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode(gpb.SubscriptionList_ONCE))),
		query,
		30*time.Second,
		func(val *ygnmi.Value[uint64]) bool {
			_, present := val.Val()
			element := val.Path.Elem
			if getPathFromElements(element) == path {
				return present
			}
			return !present
		},
	).Await(t)

	counter, ok := data.Val()
	if ok {
		return counter, nil
	} else {
		return 0, fmt.Errorf("failed to collect data for path %s", path)
	}
}
