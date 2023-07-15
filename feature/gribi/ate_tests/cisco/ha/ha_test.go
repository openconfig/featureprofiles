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

package ha_test

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/gribi"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"

	"github.com/openconfig/featureprofiles/internal/cisco/ha/monitor"
	"github.com/openconfig/featureprofiles/internal/cisco/ha/runner"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	proto_gnmi "github.com/openconfig/gnmi/proto/gnmi"
	gnps "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/testt"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// user needed inputs
const (
	with_scale            = false                       // run entire script with or without scale (Support not yet coded)
	with_RPFO             = false                       // run entire script with or without RFPO
	base_config           = "case4_decap_encap_recycle" // Will run all the tcs with set base programming case, options : case1_backup_decap, case2_decap_encap_exit, case3_decap_encap, case4_decap_encap_recycle
	active_rp             = "0/RP0/CPU0"
	standby_rp            = "0/RP1/CPU0"
	lc                    = "0/0/CPU0" // set value for lc_oir tc, if empty it means no lc, example: 0/0/CPU0
	process_restart_count = 1
	microdropsRepeat      = 1
)

// vrf and prefixes
const (
	dst       = "198.51.100.0"
	mask      = "32"
	dstPfxMin = "198.51.100.0"
	vrf1      = "TE"
	vrf2      = "REPAIRED"
	vrf3      = "REPAIR"
	vrf4      = "DECAP"
)

// gribi programming variables
const (
	gribi_Scale        = 2
	nhg_Scale_TE       = 1000  // NHG scale used for TE vrf
	nh_prefix_TE       = 2     // same nh will be used across all the nhgs
	nh_scale_TE        = 10000 // create NHs with different index and repeat prefix set under nh_prefix_TE flag, set an even number
	nhg_Scale_REPAIRED = 1000  // NHG scale used for REPAIR vrf
	nhg_Scale_REPAIR   = 500   // NHG scale used for DECAP_ENCAP case
	nh_scale_REPAIR    = 500   // create NHs used by NHGs for DECAP_ENCAP case
	programming_RFPO   = 1     // Perform RFPO and programming followed by it
	grpc_repeat        = 1
)

// traffic constant
const (
	bgpPfx                = 100000 //set value for scale bgp setup 100000
	isisPfx               = 25000  //set value for scale isis setup 10000
	innerdstPfxCount_bgp  = 1      //set value for number of inner prefix for bgp flow
	innerdstPfxCount_isis = 1      //set value for number of inner prefix for isis flow
)

// global variables
var (
	prefixes      = []string{}
	repair_prefix = []string{}
	flows         = []*ondatra.Flow{}
	te_flow       = []*ondatra.Flow{}
	src_ip_flow   = []*ondatra.Flow{}
	p4rtNodeName  = flag.String("p4rt_node_name", "0/0/CPU0-NPU0", "component name for P4RT Node")
	rpfo_count    = 0 // used to track rpfo_count if its more than 10 then reset to 0 and reload the HW
)

// NHScaleOptions
type NHScaleOptions struct {
	action string
	src    string
	dest   string
}

// NHGScaleOptions
type NHGScaleOptions struct {
	mode string
}

// IPv4ScaleOptions
type IPv4ScaleOptions struct {
	max int
}

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

// sortPorts sorts the ports by the testbed port ID.
func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})
	return ports
}

func (args *testArgs) processrestart(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, pName string) {
	pList := gnmi.GetAll(t, dut, gnmi.OC().System().ProcessAny().State())
	var pID uint64
	for _, proc := range pList {
		if proc.GetName() == pName {
			pID = proc.GetPid()
			t.Logf("Pid of daemon '%s' is '%d'", pName, pID)
		}
	}

	gnoiClient := dut.RawAPIs().GNOI().Default(t)
	for i := 0; i < process_restart_count; i++ {
		killRequest := &gnps.KillProcessRequest{Name: pName, Pid: uint32(pID), Signal: gnps.KillProcessRequest_SIGNAL_TERM, Restart: true}
		killResponse, err := gnoiClient.System().KillProcess(context.Background(), killRequest)
		t.Logf("Got kill process response: %v\n\n", killResponse)
		// bypassing the check as emsd restart causes timing issue
		if err != nil && pName != "emsd" {
			t.Fatalf("Failed to execute gNOI Kill Process, error received: %v", err)
		}
	}

	// reestablishing gribi connection
	if pName == "emsd" {
		// client := gribi.Client{
		// 	DUT:                   dut,
		// 	FibACK:                *ciscoFlags.GRIBIFIBCheck,
		// 	Persistence:           true,
		// 	InitialElectionIDLow:  1,
		// 	InitialElectionIDHigh: 0,
		// }
		// if err := client.Start(t); err != nil {
		// 	t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
		// 	if err = client.Start(t); err != nil {
		// 		t.Fatalf("gRIBI Connection could not be established: %v", err)
		// 	}
		// }
		// args.client = &client
		args.client.Start(t)
	}
}

func (args *testArgs) rpfo(ctx context.Context, t *testing.T, gribi_reconnect bool) {

	// reload the HW is rfpo count is 10 or more
	if rpfo_count == 10 {
		gnoiClient := args.dut.RawAPIs().GNOI().New(t)
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
		time.Sleep(time.Minute * 20)
	}
	// supervisor info
	var supervisors []string
	active_state := gnmi.OC().Component(active_rp).Name().State()
	active := gnmi.Get(t, args.dut, active_state)
	standby_state := gnmi.OC().Component(standby_rp).Name().State()
	standby := gnmi.Get(t, args.dut, standby_state)
	supervisors = append(supervisors, active, standby)

	// find active and standby RP
	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyRP(t, args.dut, supervisors)
	t.Logf("Detected activeRP: %v, standbyRP: %v", rpActiveBeforeSwitch, rpStandbyBeforeSwitch)

	// make sure standby RP is reach
	switchoverReady := gnmi.OC().Component(rpActiveBeforeSwitch).SwitchoverReady()
	gnmi.Await(t, args.dut, switchoverReady.State(), 30*time.Minute, true)
	t.Logf("SwitchoverReady().Get(t): %v", gnmi.Get(t, args.dut, switchoverReady.State()))
	if got, want := gnmi.Get(t, args.dut, switchoverReady.State()), true; got != want {
		t.Errorf("switchoverReady.Get(t): got %v, want %v", got, want)
	}
	gnoiClient := args.dut.RawAPIs().GNOI().New(t)
	useNameOnly := deviations.GNOISubcomponentPath(args.dut)
	switchoverRequest := &gnps.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(rpStandbyBeforeSwitch, useNameOnly),
	}
	t.Logf("switchoverRequest: %v", switchoverRequest)
	switchoverResponse, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
	if err != nil {
		t.Fatalf("Failed to perform control processor switchover with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v, err: %v", switchoverResponse, err)

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
			currentTime = gnmi.Get(t, args.dut, gnmi.OC().System().CurrentDatetime().State())
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

	rpStandbyAfterSwitch, rpActiveAfterSwitch := components.FindStandbyRP(t, args.dut, supervisors)
	t.Logf("Found standbyRP after switchover: %v, activeRP: %v", rpStandbyAfterSwitch, rpActiveAfterSwitch)

	if got, want := rpActiveAfterSwitch, rpStandbyBeforeSwitch; got != want {
		t.Errorf("Get rpActiveAfterSwitch: got %v, want %v", got, want)
	}
	if got, want := rpStandbyAfterSwitch, rpActiveBeforeSwitch; got != want {
		t.Errorf("Get rpStandbyAfterSwitch: got %v, want %v", got, want)
	}

	t.Log("Validate OC Switchover time/reason.")
	activeRP := gnmi.OC().Component(rpActiveAfterSwitch)
	if got, want := gnmi.Lookup(t, args.dut, activeRP.LastSwitchoverTime().State()).IsPresent(), true; got != want {
		t.Errorf("activeRP.LastSwitchoverTime().Lookup(t).IsPresent(): got %v, want %v", got, want)
	} else {
		t.Logf("Found activeRP.LastSwitchoverTime(): %v", gnmi.Get(t, args.dut, activeRP.LastSwitchoverTime().State()))
	}

	if got, want := gnmi.Lookup(t, args.dut, activeRP.LastSwitchoverReason().State()).IsPresent(), true; got != want {
		t.Errorf("activeRP.LastSwitchoverReason().Lookup(t).IsPresent(): got %v, want %v", got, want)
	} else {
		lastSwitchoverReason := gnmi.Get(t, args.dut, activeRP.LastSwitchoverReason().State())
		t.Logf("Found lastSwitchoverReason.GetDetails(): %v", lastSwitchoverReason.GetDetails())
		t.Logf("Found lastSwitchoverReason.GetTrigger().String(): %v", lastSwitchoverReason.GetTrigger().String())
	}

	// reestablishing gribi connection
	if gribi_reconnect {
		// client := gribi.Client{
		// 	DUT:                   args.dut,
		// 	FibACK:                *ciscoFlags.GRIBIFIBCheck,
		// 	Persistence:           true,
		// 	InitialElectionIDLow:  1,
		// 	InitialElectionIDHigh: 0,
		// }
		// if err := client.Start(t); err != nil {
		// 	t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
		// 	if err = client.Start(t); err != nil {
		// 		t.Fatalf("gRIBI Connection could not be established: %v", err)
		// 	}
		// }
		// args.client = &client
		args.client.Start(t)
	}
}

func baseProgramming(ctx context.Context, t *testing.T, args *testArgs) {

	if with_scale {
		ciscoFlags.GRIBIChecks.AFTChainCheck = false
		ciscoFlags.GRIBIChecks.AFTCheck = false
	}

	if base_config == "case1_backup_decap" {
		for i := 0; i < gribi_Scale; i++ {
			prefixes = append(prefixes, util.GetIPPrefix(dst, i, mask))
		}
		case1_backup_decap(ctx, t, args)
	} else if base_config == "case2_decap_encap_exit" {
		for i := 0; i < gribi_Scale; i++ {
			if i < 500 {
				repair_prefix = append(repair_prefix, util.GetIPPrefix(dst, i, mask))
			}
			prefixes = append(prefixes, util.GetIPPrefix(dst, i, mask))
		}
		case2_decap_encap_exit(ctx, t, args)
	} else if base_config == "case3_decap_encap" {
		for i := 0; i < gribi_Scale; i++ {
			if i < 500 {
				repair_prefix = append(repair_prefix, util.GetIPPrefix(dst, i, mask))
			}
			prefixes = append(prefixes, util.GetIPPrefix(dst, i, mask))
		}
		case3_decap_encap(ctx, t, args)
	} else if base_config == "case4_decap_encap_recycle" {
		for i := 0; i < gribi_Scale; i++ {
			if i < 500 {
				repair_prefix = append(repair_prefix, util.GetIPPrefix(dst, i, mask))
			}
			prefixes = append(prefixes, util.GetIPPrefix(dst, i, mask))
		}
		case4_decap_encap_recycle(ctx, t, args)
	}
}
func case1_backup_decap(ctx context.Context, t *testing.T, args *testArgs) {

	// Programming
	// ======================================================
	// IPinIP (198.51.100.1/32)  --- NHG ---- NH1 VIP1 (192.0.2.40/32) --- NHG --- NH 1000 - BE 121, NH 1100 - BE 122, NH 1200 - BE 123
	// network instance TE            |  ---- NH2 VIP2 (192.0.2.41/32) --- NHG --- NH 2000 - BE 124, NH 2100 - BE 125
	//                                |
	//                               BNG ---- NH DECAP BE 127
	// ======================================================

	args.client.BecomeLeader(t)
	args.client.FlushServer(t)
	time.Sleep(10 * time.Second)

	// adding default route pointing to Valid Path
	// t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	// config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	// config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
	// defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	// defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	if with_scale {
		args.client.AddNH(t, 1000000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 1100000, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 1200000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 100000, 0, map[uint64]uint64{1000000: 50, 1100000: 30, 1200000: 20}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4(t, "192.0.2.40/32", 100000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 2000000, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 2100000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 2200000, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 200000, 0, map[uint64]uint64{2000000: 30, 2100000: 50, 2200000: 20}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4(t, "192.0.2.41/32", 200000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 10, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 99, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

		args.scaleNH(t, "192.0.2.40", 1000, nh_scale_TE, nh_prefix_TE)
		args.scaleNHG(t, 1000, nhg_Scale_TE, 99, 1000)
		args.scaleIPV4(t, vrf1, 1000)

	} else {
		args.client.AddNH(t, 1000000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 1100000, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 1200000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 100000, 0, map[uint64]uint64{1000000: 50, 1100000: 30, 1200000: 20}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4(t, "192.0.2.40/32", 100000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 2000000, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 2100000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 2200000, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 200000, 0, map[uint64]uint64{2000000: 30, 2100000: 50, 2200000: 20}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4(t, "192.0.2.42/32", 200000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 10, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 99, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 200, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 1000, 99, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	}
}

func case2_decap_encap_exit(ctx context.Context, t *testing.T, args *testArgs) {

	// Programming
	// ======================================================
	// IPinIP (198.51.100.1/32)  --- NHG ---- NH1 VIP1 (192.0.2.40/32) --- NHG --- NH 1000 - BE 121, NH 1100 - BE 122, NH 1200 - BE 123
	// network instance TE            |  ---- NH2 VIP2 (192.0.2.41/32) --- NHG --- NH 2000 - BE 124, NH 2100 - BE 125
	//                                |
	//                               BNG ---- NH REPAIR
	//
	// IPinIP (198.51.100.1/32)  --- NHG ---- NH1 VIP1 (192.0.2.40/32) --- NHG --- NH 1000 - BE 121, NH 1100 - BE 122, NH 1200 - BE 123
	// network instance REPAIRED      |  ---- NH2 VIP2 (192.0.2.41/32) --- NHG --- NH 2000 - BE 124, NH 2100 - BE 125
	// with sourceip                  |
	//                               BNG ---- NH DECAP VRF
	//
	// 198.51.100.1/32 REPAIR    --- NHG ---- NH DECAP/ENCAP           --- NHG --- NH 3000 - BE 126
	//                                |
	//                               BNG ---- NH DECAP BE 127
	//
	// 0.0.0.0/0 DECAP           --- NHG ---- NH DECAP BE 127
	//
	// ======================================================

	args.client.BecomeLeader(t)
	args.client.FlushServer(t)
	time.Sleep(10 * time.Second)

	// t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	// config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	// config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
	// defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	// defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	if with_scale {
		args.client.AddNH(t, 1000000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 1100000, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 1200000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 100000, 0, map[uint64]uint64{1000000: 50, 1100000: 30, 1200000: 20}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4(t, "192.0.2.40/32", 100000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 2000000, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 2100000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 200000, 0, map[uint64]uint64{2000000: 60, 2100000: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4(t, "192.0.2.41/32", 200000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 3000000, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 300000, 0, map[uint64]uint64{3000000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4(t, "10.1.0.1/32", 300000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 1111111, "", *ciscoFlags.DefaultNetworkInstance, vrf3, "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 111111, 0, map[uint64]uint64{1111111: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.scaleNH(t, "192.0.2.40", 1000, nh_scale_TE, nh_prefix_TE)
		args.scaleNHG(t, 1000, nhg_Scale_TE, 111111, 1000)
		args.scaleIPV4(t, vrf1, 1000)

		args.client.AddNH(t, 2222222, "", *ciscoFlags.DefaultNetworkInstance, vrf4, "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 222222, 0, map[uint64]uint64{2222222: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.scaleNHG(t, 21000, nhg_Scale_REPAIRED, 222222, 1000)
		args.scaleIPV4(t, vrf2, 21000)

		args.client.AddNH(t, 4444444, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 444444, 0, map[uint64]uint64{4444444: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

		args.scaleNH(t, "10.1.0.1", 30000, nh_scale_REPAIR, 0, &NHScaleOptions{action: "DECAP_ENCAP", src: "222.222.222.222", dest: "10.1.0.1"})
		args.scaleNHG(t, 30000, nhg_Scale_REPAIR, 444444, 30000, &NHGScaleOptions{mode: "DECAP_ENCAP"})
		args.scaleIPV4(t, vrf3, 30000, &IPv4ScaleOptions{max: nhg_Scale_REPAIR})

		args.client.AddIPv4(t, "0.0.0.0/0", 444444, vrf4, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	} else {
		args.client.AddNH(t, 1000000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 1100000, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 1200000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 100000, 0, map[uint64]uint64{1000000: 60, 1100000: 30, 1200000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4(t, "192.0.2.40/32", 100000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 2000000, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 2100000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 200000, 0, map[uint64]uint64{2000000: 50, 2100000: 50}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4(t, "192.0.2.42/32", 200000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 3000000, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 300000, 0, map[uint64]uint64{3000000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4(t, "10.1.0.1/32", 300000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 1111111, "", *ciscoFlags.DefaultNetworkInstance, vrf3, "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 111111, 0, map[uint64]uint64{1111111: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 200, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 100, 111111, map[uint64]uint64{100: 2, 200: 2}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4Batch(t, prefixes, 100, vrf1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 2222222, "", *ciscoFlags.DefaultNetworkInstance, vrf4, "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 222222, 0, map[uint64]uint64{2222222: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 200, 222222, map[uint64]uint64{100: 30, 200: 70}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4Batch(t, prefixes, 200, vrf2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 4444444, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 444444, 0, map[uint64]uint64{4444444: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 3333333, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: "222.222.222.222", Dest: []string{"10.1.0.1"}})
		args.client.AddNHG(t, 333333, 444444, map[uint64]uint64{3333333: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4Batch(t, repair_prefix, 333333, vrf3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

		args.client.AddIPv4(t, "0.0.0.0/0", 444444, vrf4, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	}
}

func case3_decap_encap(ctx context.Context, t *testing.T, args *testArgs) {

	// Programming
	// ======================================================
	// IPinIP (198.51.100.1/32)  --- NHG ---- NH1 VIP1 (192.0.2.40/32) --- NHG --- NH 1000 - BE 121, NH 1100 - BE 122, NH 1200 - BE 123
	// network instance TE            |  ---- NH2 VIP2 (192.0.2.41/32) --- NHG --- NH 2000 - BE 124, NH 2100 - BE 125
	//                                |
	//                               BNG ---- NH REPAIR
	//
	// IPinIP (10.1.0.1/32)      --- NHG ---- NH1 VIP1 (192.0.2.40/32) --- NHG --- NH 1000 - BE 121, NH 1100 - BE 122, NH 1200 - BE 123
	// network instance REPAIRED      |  ---- NH2 VIP2 (192.0.2.41/32) --- NHG --- NH 2000 - BE 124, NH 2100 - BE 125
	// with sourceip                  |
	//                               BNG ---- NH DECAP BE 127
	//
	// 198.51.100.1/32 REPAIR    --- NHG ---- NH DECAP/ENCAP Network Instance REPAIRED
	//                                |
	//                               BNG ---- NH DECAP BE 127
	//
	// 0.0.0.0/0 DECAP           --- NHG ---- NH DECAP BE 127
	//
	// ======================================================

	args.client.BecomeLeader(t)
	args.client.FlushServer(t)
	time.Sleep(10 * time.Second)

	// t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	// config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	// config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
	// defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	// defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	if with_scale {
		args.client.AddNH(t, 1000000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 1100000, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 1200000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 100000, 0, map[uint64]uint64{1000000: 50, 1100000: 30, 1200000: 20}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4(t, "192.0.2.40/32", 100000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 2000000, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 2100000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 200000, 0, map[uint64]uint64{2000000: 60, 2100000: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4(t, "192.0.2.41/32", 200000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 1111111, "", *ciscoFlags.DefaultNetworkInstance, vrf3, "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 111111, 0, map[uint64]uint64{1111111: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.scaleNH(t, "192.0.2.40", 1000, nh_scale_TE, nh_prefix_TE)
		args.scaleNHG(t, 1000, nhg_Scale_TE, 111111, 1000)
		args.scaleIPV4(t, vrf1, 1000)

		args.client.AddNH(t, 4444444, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 444444, 0, map[uint64]uint64{4444444: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

		args.scaleNHG(t, 21000, nhg_Scale_REPAIRED, 444444, 1000)
		args.scaleIPV4(t, vrf2, 21000)

		args.scaleNH(t, "10.1.0.1", 30000, nh_scale_REPAIR, 0, &NHScaleOptions{action: "DECAP_ENCAP", src: "222.222.222.222", dest: "10.1.0.1"})
		args.scaleNHG(t, 30000, nhg_Scale_REPAIR, 444444, 30000, &NHGScaleOptions{mode: "DECAP_ENCAP"})
		args.scaleIPV4(t, vrf3, 30000, &IPv4ScaleOptions{max: nhg_Scale_REPAIR})

		args.client.AddIPv4(t, "0.0.0.0/0", 444444, vrf4, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	} else {
		args.client.AddNH(t, 1000000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 1100000, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 1200000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 100000, 0, map[uint64]uint64{1000000: 60, 1100000: 30, 1200000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4(t, "192.0.2.40/32", 100000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 2000000, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 2100000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 200000, 0, map[uint64]uint64{2000000: 50, 2100000: 50}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4(t, "192.0.2.42/32", 200000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 1111111, "", *ciscoFlags.DefaultNetworkInstance, vrf3, "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 111111, 0, map[uint64]uint64{1111111: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 200, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 100, 111111, map[uint64]uint64{100: 2, 200: 2}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4Batch(t, prefixes, 100, vrf1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 2222222, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 222222, 0, map[uint64]uint64{2222222: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 3000000, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 200, 222222, map[uint64]uint64{3000000: 1}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4(t, "10.1.0.1/32", 200, vrf2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 3333333, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, vrf2, "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: "222.222.222.222", Dest: []string{"10.1.0.1"}})
		args.client.AddNHG(t, 333333, 222222, map[uint64]uint64{3333333: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4Batch(t, prefixes, 333333, vrf3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	}
}

func case4_decap_encap_recycle(ctx context.Context, t *testing.T, args *testArgs) {

	// Programming
	// ======================================================
	// IPinIP (198.51.100.1/32)  --- NHG ---- NH1 VIP1 (192.0.2.40/32) --- NHG --- NH 1000 - BE 121, NH 1100 - BE 122, NH 1200 - BE 123
	// network instance TE            |  ---- NH2 VIP2 (192.0.2.41/32) --- NHG --- NH 2000 - BE 124, NH 2100 - BE 125
	//                                |
	//                               BNG ---- NH DECAP/ENCAP Network Instance REPAIRED (filter on destination address in REPAIRED vrf)
	//
	// IPinIP (198.51.100.1/32)  --- NHG ---- NH1 VIP1 (192.0.2.40/32) --- NHG --- NH 1000 - BE 121, NH 1100 - BE 122, NH 1200 - BE 123
	// network instance REPAIRED      |  ---- NH2 VIP2 (192.0.2.41/32) --- NHG --- NH 2000 - BE 124, NH 2100 - BE 125
	// with sourceip                  |
	//                               BNG ---- NH DECAP BE 127
	//
	// 10.1.0.1/32               --- NHG ---- NH2 VIP3 (20.0.0.1/32) --- NHG --- NH 11 - DECAP BE 126
	// 				                  |
	//                               BNG ---- NH DECAP BE 127
	//
	// ======================================================

	args.client.BecomeLeader(t)
	args.client.FlushServer(t)
	time.Sleep(10 * time.Second)

	// t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	// config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	// config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
	// defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	// defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	if with_scale {
		args.client.AddNH(t, 1000000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 1100000, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 1200000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 100000, 0, map[uint64]uint64{1000000: 50, 1100000: 30, 1200000: 20}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4(t, "192.0.2.40/32", 100000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 2000000, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 2100000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 200000, 0, map[uint64]uint64{2000000: 60, 2100000: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4(t, "192.0.2.41/32", 200000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 3000000, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 300000, 0, map[uint64]uint64{3000000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4(t, "20.0.0.1/32", 300000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 1111111, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, vrf2, "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: "222.222.222.222", Dest: []string{"10.1.0.1"}})
		args.client.AddNHG(t, 111111, 0, map[uint64]uint64{1111111: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.scaleNH(t, "192.0.2.40", 1000, nh_scale_TE, nh_prefix_TE)
		args.scaleNHG(t, 1000, nhg_Scale_TE, 111111, 1000)
		args.scaleIPV4(t, vrf1, 1000)

		args.client.AddNH(t, 2222222, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 222222, 0, map[uint64]uint64{2222222: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.scaleNHG(t, 21000, nhg_Scale_REPAIRED, 222222, 1000)
		args.scaleIPV4(t, vrf2, 21000)

		args.client.AddNH(t, 3333333, "20.0.0.1", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 333333, 222222, map[uint64]uint64{3333333: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4(t, "10.1.0.1/32", 333333, vrf2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	} else {
		args.client.AddNH(t, 1000000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 1100000, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 1200000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 100000, 0, map[uint64]uint64{1000000: 60, 1100000: 30, 1200000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4(t, "192.0.2.40/32", 100000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 2000000, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 2100000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 200000, 0, map[uint64]uint64{2000000: 50, 2100000: 50}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4(t, "192.0.2.42/32", 200000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 3000000, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 300000, 0, map[uint64]uint64{3000000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4(t, "20.0.0.1/32", 300000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 1111111, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, vrf2, "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: "222.222.222.222", Dest: []string{"10.1.0.1"}})
		args.client.AddNHG(t, 111111, 0, map[uint64]uint64{1111111: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, 200, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 100, 111111, map[uint64]uint64{100: 2, 200: 2}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4Batch(t, prefixes, 100, vrf1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 2222222, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 222222, 0, map[uint64]uint64{2222222: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 200, 222222, map[uint64]uint64{100: 30, 200: 70}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4Batch(t, prefixes, 200, vrf2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

		args.client.AddNH(t, 3333333, "20.0.0.1", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNHG(t, 333333, 222222, map[uint64]uint64{3333333: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.AddIPv4(t, "10.1.0.1/32", 333333, vrf2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	}
}

func (a *testArgs) scaleNH(t *testing.T, nh_prefix string, start_index int, scale int, prefix_repeat int, opts ...*NHScaleOptions) {

	prefix := net.ParseIP(nh_prefix)
	prefix = prefix.To4()

	resultLenBefore := len(a.client.Fluent(t).Results(t))
	NHEntries := []fluent.GRIBIEntry{}

	for i := 0; i < scale; i++ {
		if len(opts) != 0 {
			NHEntry := fluent.NextHopEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
			NHEntry = NHEntry.WithIPAddress(prefix.String()).WithIndex(uint64(start_index + i))
			for _, opt := range opts {
				if opt.action == "DECAP" {
					NHEntry = NHEntry.WithDecapsulateHeader(fluent.IPinIP)
				} else if opt.action == "ENCAP" {
					NHEntry = NHEntry.WithEncapsulateHeader(fluent.IPinIP)
				} else if opt.action == "DECAP_ENCAP" {
					NHEntry = NHEntry.WithDecapsulateHeader(fluent.IPinIP)
					NHEntry = NHEntry.WithEncapsulateHeader(fluent.IPinIP)
					NHEntry = NHEntry.WithIPinIP(opt.src, opt.dest)
				}
			}
			NHEntries = append(NHEntries, NHEntry)
		} else {
			resultLenBefore = len(a.client.Fluent(t).Results(t))
			for j := i; j < prefix_repeat+i; j++ {
				// if j%256 == 0 && j != 0 {
				// 	prefix[2]++
				// }
				NHEntry := fluent.NextHopEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
				NHEntry = NHEntry.WithIPAddress(prefix.String()).WithIndex(uint64(start_index + i + j))
				NHEntries = append(NHEntries, NHEntry)
				prefix[3]++
			}
			prefix = net.ParseIP(nh_prefix)
			prefix = prefix.To4()
		}
	}
	a.client.Fluent(t).Modify().AddEntry(t, NHEntries...)
	if err := a.client.AwaitTimeout(context.Background(), t, 10*time.Minute); err != nil {
		t.Fatalf("Error waiting to add NH entries: %v", err)
	}
	resultLenAfter := len(a.client.Fluent(t).Results(t))
	newResultsCount := resultLenAfter - resultLenBefore
	expectResultCount := scale
	if len(opts) == 0 {
		// fib, rib, * prefix_repeat
		if newResultsCount != expectResultCount*2*prefix_repeat {
			t.Fatalf("Number of responses for programing NH results is not as expected, want: %d , got: %d ", expectResultCount, newResultsCount)
		}
	} else {
		if newResultsCount != expectResultCount*2 {
			t.Fatalf("Number of responses for programing NH results is not as expected, want: %d , got: %d ", expectResultCount, newResultsCount)
		}
	}
}

func (a *testArgs) scaleNHG(t *testing.T, nhg_start uint64, nhg_scale int, bkgNHG uint64, nhs_start uint64, opts ...*NHGScaleOptions) {
	resultLenBefore := len(a.client.Fluent(t).Results(t))
	NHGEntries := []fluent.GRIBIEntry{}
	for i := 0; i < nhg_scale; i++ {
		nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(nhg_start)
		if bkgNHG != 0 {
			nhg.WithBackupNHG(bkgNHG)
		}
		if len(opts) != 0 {
			rand.Seed(time.Now().UnixNano())
			min := 10
			max := 70
			value := rand.Intn(max-min+1) + min
			nhg.AddNextHop(nhs_start, uint64(value))
			nhs_start = nhs_start + 1
		} else {
			for j := 0; j < nh_prefix_TE; j++ {
				rand.Seed(time.Now().UnixNano())
				min := 10
				max := 70
				value := rand.Intn(max-min+1) + min
				nhg.AddNextHop(nhs_start, uint64(value))
				nhs_start = nhs_start + 1
			}
		}
		NHGEntries = append(NHGEntries, nhg)
		nhg_start = nhg_start + 1
	}
	a.client.Fluent(t).Modify().AddEntry(t, NHGEntries...)
	if err := a.client.AwaitTimeout(context.Background(), t, 10*time.Minute); err != nil {
		t.Fatalf("Error waiting to add NH entries: %v", err)
	}
	resultLenAfter := len(a.client.Fluent(t).Results(t))
	newResultsCount := resultLenAfter - resultLenBefore
	expectResultCount := nhg_scale
	if newResultsCount != expectResultCount*2 {
		t.Fatalf("Number of responses for programing NHG results is not as expected, want: %d , got: %d ", expectResultCount, newResultsCount)
	}
}

func (a *testArgs) scaleIPV4(t *testing.T, vrf_name string, nhg_start int, opts ...*IPv4ScaleOptions) {
	resultLenBefore := len(a.client.Fluent(t).Results(t))
	nhg_count := 0
	ipv4Entries := []fluent.GRIBIEntry{}
	for index, prefix := range prefixes {
		if len(opts) != 0 {
			if index == opts[0].max {
				break
			}
		}
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance(vrf_name).
			WithPrefix(prefix).
			WithNextHopGroup(uint64(nhg_start + nhg_count)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		ipv4Entries = append(ipv4Entries, ipv4Entry)
		nhg_count = nhg_count + 1
		if nhg_count == nhg_Scale_TE {
			nhg_count = 0
		}
	}
	a.client.Fluent(t).Modify().AddEntry(t, ipv4Entries...)
	if err := a.client.AwaitTimeout(context.Background(), t, 10*time.Minute); err != nil {
		t.Fatalf("Error waiting to add IPv4 entries: %v", err)
	}
	resultLenAfter := len(a.client.Fluent(t).Results(t))
	newResultsCount := resultLenAfter - resultLenBefore
	var expectResultCount int
	if len(opts) != 0 && len(prefixes) > opts[0].max {
		expectResultCount = opts[0].max
	} else {
		expectResultCount = len(prefixes)
	}

	if newResultsCount != expectResultCount*2 {
		t.Fatalf("Number of responses for programing IPV4 results is not as expected, want: %d , got: %d ", expectResultCount, newResultsCount)
	}
}

//lint:ignore U1000 Ignore unused function temporarily for debugging
func baseScaleProgramming(ctx context.Context, t *testing.T, args *testArgs) {

	// setting aft check false, as it can take forever to run
	ciscoFlags.GRIBIChecks.AFTChainCheck = false
	ciscoFlags.GRIBIChecks.AFTCheck = false

	args.client.BecomeLeader(t)
	args.client.FlushServer(t)
	time.Sleep(10 * time.Second)

	for i := 0; i < gribi_Scale; i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dst, i, mask))
	}

	// =========================================
	// LEVEL 1 (DEFAULT VRF)
	// VIP1
	//	- PATH1 NH ID 1000, weight 60, outgoing Port2
	//	- PATH2 NH ID 1100, weight 30, outgoing Port3
	//	- PATH3 NH ID 1200, weight 10, outgoing Port4
	// VIP2
	//	- PATH1 NH ID 2000, weight 50, outgoing Port5
	//	- PATH2 NH ID 2100, weight 50, outgoing Port6
	// Decap/Encap
	//	- PATH1 NH ID 3000, weight  5, outgoing Port7
	// Decap
	//	- PATH1 NH ID 4000, weight  100, outgoing Port8

	// -----------------------------------------
	// VIP1 NHs
	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	// VIP1 mapping to NHs
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 60, 1100: 30, 1200: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// -----------------------------------------
	// VIP2 NHs
	args.client.AddNH(t, 2000, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	//VIP2 mapping to NHs
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 50, 2100: 50}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.41/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// -----------------------------------------
	// DECAP/ENCAP NHs
	args.client.AddNH(t, 3000, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 3000, 0, map[uint64]uint64{3000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "10.1.0.1/32", 3000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// =========================================
	// LEVEL 2
	// -----------------------------------------
	// BACKUP Decap
	// -----------------------------------------
	args.client.AddNH(t, 4000, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 4000, 0, map[uint64]uint64{4000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// -----------------------------------------
	// create backup in vrf REPAIR
	args.client.AddNH(t, 5000, "", *ciscoFlags.DefaultNetworkInstance, "REPAIR", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 5000, 0, map[uint64]uint64{5000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	// -----------------------------------------
	// backup in vrf DECAP
	args.client.AddNH(t, 6000, "", *ciscoFlags.DefaultNetworkInstance, "DECAP", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 6000, 0, map[uint64]uint64{6000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// vrf TE
	args.scaleNH(t, "192.0.2.40", 10000, nh_scale_TE, nh_prefix_TE)
	args.scaleNHG(t, 10000, nhg_Scale_TE, 5000, 10000)
	args.scaleIPV4(t, vrf1, 10000)

	// // -----------------------------------------
	// // vrf REPAIRED using the same NH with different backup
	args.scaleNHG(t, 20000, nhg_Scale_REPAIRED, 6000, 10000)
	args.scaleIPV4(t, vrf2, 10000)

	// // -----------------------------------------
	// // vrf REPAIR
	args.scaleNH(t, "10.1.0.1", 30000, nh_scale_REPAIR, 0, &NHScaleOptions{action: "DECAP_ENCAP", src: "222.222.222.222", dest: "10.1.0.1"})
	args.scaleNHG(t, 30000, nhg_Scale_REPAIR, 4000, 30000, &NHGScaleOptions{mode: "DECAP_ENCAP"})
	args.scaleIPV4(t, vrf3, 30000, &IPv4ScaleOptions{max: nhg_Scale_REPAIR})

	// // -----------------------------------------
	// // vrf DECAP
	args.client.AddIPv4(t, "0.0.0.0/0", 4000, vrf4, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
}

//lint:ignore U1000 Ignore unused function temporarily for debugging
func (args *testArgs) gnmiConf(t *testing.T, conf string) {
	updateRequest := &proto_gnmi.Update{
		Path: &proto_gnmi.Path{
			Origin: "cli",
		},
		Val: &proto_gnmi.TypedValue{
			Value: &proto_gnmi.TypedValue_AsciiVal{
				AsciiVal: conf,
			},
		},
	}
	setRequest := &proto_gnmi.SetRequest{}
	setRequest.Update = []*proto_gnmi.Update{updateRequest}
	gnmiClient := args.dut.RawAPIs().GNMI().New(t)
	if _, err := gnmiClient.Set(args.ctx, setRequest); err != nil {
		t.Fatalf("gNMI set request failed: %v", err)
	}
}

func testRestart_single_process(t *testing.T, args *testArgs) {

	// base programming
	baseProgramming(args.ctx, t, args)

	// create new flows and start traffic
	if *ciscoFlags.GRIBITrafficCheck {
		te_flow = args.allFlows(t)
		if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
			src_ip_flow = args.allFlows(t, &TGNoptions{SrcIP: "222.222.222.222"})
		}
		flows = append(te_flow, src_ip_flow...)
	}

	outgoing_interface := make(map[string][]string)

	// verify traffic
	if *ciscoFlags.GRIBITrafficCheck {
		outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
		if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
			outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
		}
		args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{burst: true, start_after_verification: true})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck && !with_scale {
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}

	processes := []string{"isis", "bgp", "db_writer", "emsd", "ipv4_rib", "ipv6_rib", "fib_mgr", "ifmgr"}
	for i := 0; i < len(processes); i++ {
		t.Run(processes[i], func(t *testing.T) {
			// RPFO
			if with_RPFO {
				rpfo_count = rpfo_count + 1
				t.Logf("This is RPFO #%d", rpfo_count)
				args.rpfo(args.ctx, t, false)
				// verify traffic
				t.Logf("checking traffic after RPFO")
				if *ciscoFlags.GRIBITrafficCheck {
					outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
					if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
						outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
					}
					args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{start_after_verification: true})
				}
			}

			// Restart process
			t.Logf("Restarting process %s", processes[i])
			if processes[i] == "fib_mgr" {
				config.CMDViaGNMI(args.ctx, t, args.dut, "process restart fib_mgr location 0/RP0/CPU0")
			} else {
				args.processrestart(args.ctx, t, args.dut, processes[i])
			}
			time.Sleep(time.Second * 10)
			t.Logf("checking traffic after restarting process %s", processes[i])
			if *ciscoFlags.GRIBITrafficCheck {
				outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
				if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
					outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
				}
				args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{start_after_verification: true})
			}
			//aft check
			// if *ciscoFlags.GRIBIAFTChainCheck && !with_scale && processes[i] != "emsd" {
			// 	randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
			// 	for i := 0; i < len(randomItems); i++ {
			// 		args.client.CheckAftIPv4(t, "TE", randomItems[i])
			// 	}
			// }

			if processes[i] == "emsd" {
				if err := args.client.Start(t); err != nil {
					t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
					if err = args.client.Start(t); err != nil {
						t.Fatalf("gRIBI Connection could not be established: %v", err)
					}
				}
				//Base level 1 scenario
				// ======================================================
				// VIP1 (192.0.2.40/32) --- 1000 (NHG) --- NH 1000 - BE 121
				//                                     --- NH 1100 - BE 122
				//                                     --- NH 1200 - BE 123
				//
				// VIP2 (192.0.2.41/32) --- 2000 (NHG) --- NH 2000 - BE 124
				//                                     --- NH 2100 - BE 125
				// ======================================================

				// ======================================================
				// DELETE VIP1, traffic via BE 124, 125
				//                      --- 1000 (NHG) --- NH 1000 - BE 121
				//                                     --- NH 1100 - BE 122
				//                                     --- NH 1200 - BE 123
				//
				// VIP2 (192.0.2.41/32) --- 2000 (NHG) --- NH 2000 - BE 124
				//                                     --- NH 2100 - BE 125
				// ======================================================
				args.client.DeleteIPv4(t, "192.0.2.40/32", 100000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
				if *ciscoFlags.GRIBITrafficCheck {
					outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
					if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
						outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
					}
					args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{start_after_verification: true})
				}

				// ======================================================
				// ADD VIP1 to point to 2000 NHG, traffic via BE 124, 125
				// VIP1 (192.0.2.40/32) --- 2000 (NHG)
				//
				// VIP2 (192.0.2.41/32) --- 2000 (NHG) --- NH 2000 - BE 124
				//                                     --- NH 2100 - BE 125
				// ======================================================
				// Add back with different NHG i.e traffic over VIP2 NHG (NH BE 124, 125)
				args.client.AddIPv4(t, "192.0.2.40/32", 200000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
				if *ciscoFlags.GRIBITrafficCheck {
					outgoing_interface["te_flow"] = []string{"Bundle-Ether124", "Bundle-Ether125"}
					if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
						outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether124", "Bundle-Ether125"}
					}
					args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{start_after_verification: true})
				}

				// ======================================================
				// UPDATE VIP1 to default state, traffic via BE 121,122,123,124,125
				// VIP1 (192.0.2.40/32) --- 1000 (NHG) --- NH 1000 - BE 121
				//                                     --- NH 1100 - BE 122
				//                                     --- NH 1200 - BE 123
				//
				// VIP2 (192.0.2.41/32) --- 2000 (NHG) --- NH 2000 - BE 124
				//                                     --- NH 2100 - BE 125
				// ======================================================
				args.client.ReplaceIPv4(t, "192.0.2.40/32", 100000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
				if *ciscoFlags.GRIBITrafficCheck {
					outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
					if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
						outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
					}
					args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{start_after_verification: true})
				}

				// ======================================================
				// UPDATE VIP2 NHG to use VIP1 NH, traffic via BE 121,122,123
				// VIP1 (192.0.2.40/32) --- 1000 (NHG) --- NH 1000 - BE 121
				//                                     --- NH 1100 - BE 122
				//                                     --- NH 1200 - BE 123
				//
				// VIP2 (192.0.2.41/32) --- 1000 (NHG)
				// ======================================================
				args.client.ReplaceNHG(t, 200000, 0, map[uint64]uint64{1000000: 50, 1100000: 5, 1200000: 45}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
				if *ciscoFlags.GRIBITrafficCheck {
					outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
					if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
						outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
					}
					args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{start_after_verification: true})
				}

				// ======================================================
				// UPDATE VIP1 NH outgoing interfaces, traffic via BE 123,124,125
				// VIP1 (192.0.2.40/32) --- 1000 (NHG) --- NH 1000 - BE 123
				//                                     --- NH 1100 - BE 124
				//                                     --- NH 1200 - BE 125
				//
				// VIP2 (192.0.2.41/32) --- 1000 (NHG)
				// ======================================================
				args.client.ReplaceNH(t, 1000000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
				args.client.ReplaceNH(t, 1100000, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
				args.client.ReplaceNH(t, 1200000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
				if *ciscoFlags.GRIBITrafficCheck {
					outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
					if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
						outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
					}
					args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{start_after_verification: true})
				}

				// ======================================================
				// ADD and REPLACE original NH config, traffic via BE 121, 122, 123
				// VIP1 (192.0.2.40/32) --- 1000 (NHG) --- NH 1000 - BE 121
				//                                     --- NH 1100 - BE 122
				//                                     --- NH 1200 - BE 123
				//
				// VIP2 (192.0.2.41/32) --- 1000 (NHG)
				// ======================================================
				args.client.ReplaceNH(t, 1000000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
				args.client.ReplaceNH(t, 1100000, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
				args.client.ReplaceNH(t, 1200000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
				if *ciscoFlags.GRIBITrafficCheck {
					outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
					if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
						outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
					}
					args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{start_after_verification: true})
				}

				// ======================================================
				// REPLACE original NHG config, traffic via BE 121, 122, 123, 124, 125
				// VIP1 (192.0.2.40/32) --- 1000 (NHG) --- NH 1000 - BE 121
				//                                     --- NH 1100 - BE 122
				//                                     --- NH 1200 - BE 123
				//
				// VIP2 (192.0.2.41/32) --- 2000 (NHG) --- NH 2000 - BE 124
				//                                     --- NH 2100 - BE 125
				// ======================================================
				args.client.ReplaceNHG(t, 200000, 0, map[uint64]uint64{2000000: 50, 2100000: 50}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
				if *ciscoFlags.GRIBITrafficCheck {
					outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
					if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
						outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
					}
					args.validateTrafficFlows(t, flows, false, outgoing_interface)
				}
			}
		})
	}
}

func test_RFPO_with_programming(t *testing.T, args *testArgs) {

	if !with_RPFO {
		t.Skip("run is without RPFO, skipping the tc")
	}
	// base programming
	baseProgramming(args.ctx, t, args)

	// create new flows and start traffic
	if *ciscoFlags.GRIBITrafficCheck {
		te_flow = args.allFlows(t)
		if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
			src_ip_flow = args.allFlows(t, &TGNoptions{SrcIP: "222.222.222.222"})
		}
		flows = append(te_flow, src_ip_flow...)
	}
	outgoing_interface := make(map[string][]string)

	// verify traffic
	if *ciscoFlags.GRIBITrafficCheck {
		outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
		if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
			outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
		}
		args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{burst: true, start_after_verification: true})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck && !with_scale {
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}

	for i := 0; i < programming_RFPO; i++ {

		// RPFO
		if with_RPFO {
			rpfo_count = rpfo_count + 1
			t.Logf("This is RPFO #%d", rpfo_count)
			args.rpfo(args.ctx, t, true)
		}

		//Base level 1 scenario
		// ======================================================
		// VIP1 (192.0.2.40/32) --- 1000 (NHG) --- NH 1000 - BE 121
		//                                     --- NH 1100 - BE 122
		//                                     --- NH 1200 - BE 123
		//
		// VIP2 (192.0.2.41/32) --- 2000 (NHG) --- NH 2000 - BE 124
		//                                     --- NH 2100 - BE 125
		// ======================================================

		// ======================================================
		// DELETE VIP1, traffic via BE 124, 125
		//                      --- 1000 (NHG) --- NH 1000 - BE 121
		//                                     --- NH 1100 - BE 122
		//                                     --- NH 1200 - BE 123
		//
		// VIP2 (192.0.2.41/32) --- 2000 (NHG) --- NH 2000 - BE 124
		//                                     --- NH 2100 - BE 125
		// ======================================================
		args.client.DeleteIPv4(t, "192.0.2.40/32", 100000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		// verify traffic
		if *ciscoFlags.GRIBITrafficCheck {
			outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
			if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
				outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
			}
			args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{start_after_verification: true})
		}

		// RPFO
		if with_RPFO {
			rpfo_count = rpfo_count + 1
			t.Logf("This is RPFO #%d", rpfo_count)
			args.rpfo(args.ctx, t, true)
		}

		// ======================================================
		// ADD VIP1 to point to 2000 NHG, traffic via BE 124, 125
		// VIP1 (192.0.2.40/32) --- 2000 (NHG)
		//
		// VIP2 (192.0.2.41/32) --- 2000 (NHG) --- NH 2000 - BE 124
		//                                     --- NH 2100 - BE 125
		// ======================================================
		// Add back with different NHG i.e traffic over VIP2 NHG (NH BE 124, 125)
		args.client.AddIPv4(t, "192.0.2.40/32", 200000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		// verify traffic
		if *ciscoFlags.GRIBITrafficCheck {
			outgoing_interface["te_flow"] = []string{"Bundle-Ether124", "Bundle-Ether125"}
			if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
				outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether124", "Bundle-Ether125"}
			}
			args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{start_after_verification: true})
		}

		// RPFO
		if with_RPFO {
			rpfo_count = rpfo_count + 1
			t.Logf("This is RPFO #%d", rpfo_count)
			args.rpfo(args.ctx, t, true)
		}

		// ======================================================
		// UPDATE VIP1 to default state, traffic via BE 121,122,123,124,125
		// VIP1 (192.0.2.40/32) --- 1000 (NHG) --- NH 1000 - BE 121
		//                                     --- NH 1100 - BE 122
		//                                     --- NH 1200 - BE 123
		//
		// VIP2 (192.0.2.41/32) --- 2000 (NHG) --- NH 2000 - BE 124
		//                                     --- NH 2100 - BE 125
		// ======================================================
		args.client.ReplaceIPv4(t, "192.0.2.40/32", 100000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		// verify traffic
		if *ciscoFlags.GRIBITrafficCheck {
			outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
			if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
				outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
			}
			args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{start_after_verification: true})
		}

		// RPFO
		if with_RPFO {
			rpfo_count = rpfo_count + 1
			t.Logf("This is RPFO #%d", rpfo_count)
			args.rpfo(args.ctx, t, true)
		}

		// ======================================================
		// UPDATE VIP2 NHG to use VIP1 NH, traffic via BE 121,122,123
		// VIP1 (192.0.2.40/32) --- 1000 (NHG) --- NH 1000 - BE 121
		//                                     --- NH 1100 - BE 122
		//                                     --- NH 1200 - BE 123
		//
		// VIP2 (192.0.2.41/32) --- 2000 (NHG)
		// ======================================================
		args.client.ReplaceNHG(t, 200000, 0, map[uint64]uint64{1000000: 50, 1100000: 5, 1200000: 45}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		// verify traffic
		if *ciscoFlags.GRIBITrafficCheck {
			outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
			if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
				outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
			}
			args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{start_after_verification: true})
		}

		// RPFO
		if with_RPFO {
			rpfo_count = rpfo_count + 1
			t.Logf("This is RPFO #%d", rpfo_count)
			args.rpfo(args.ctx, t, true)
		}

		// ======================================================
		// UPDATE VIP1 NH outgoing interfaces, traffic via BE 123,124,125
		// VIP1 (192.0.2.40/32) --- 1000 (NHG) --- NH 1000 - BE 123
		//                                     --- NH 1100 - BE 124
		//                                     --- NH 1200 - BE 125
		//
		// VIP2 (192.0.2.41/32) --- 1000 (NHG)
		// ======================================================
		args.client.ReplaceNH(t, 1000000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.ReplaceNH(t, 1100000, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.ReplaceNH(t, 1200000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		// verify traffic
		if *ciscoFlags.GRIBITrafficCheck {
			outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
			if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
				outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
			}
			args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{start_after_verification: true})
		}

		// RPFO
		if with_RPFO {
			rpfo_count = rpfo_count + 1
			t.Logf("This is RPFO #%d", rpfo_count)
			args.rpfo(args.ctx, t, true)
		}

		// ======================================================
		// ADD and REPLACE original NH config, traffic via BE 121, 122, 123
		// VIP1 (192.0.2.40/32) --- 1000 (NHG) --- NH 1000 - BE 121
		//                                     --- NH 1100 - BE 122
		//                                     --- NH 1200 - BE 123
		//
		// VIP2 (192.0.2.41/32) --- 1000 (NHG)
		// ======================================================
		args.client.ReplaceNH(t, 1000000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.ReplaceNH(t, 1100000, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.ReplaceNH(t, 1200000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		// verify traffic
		if *ciscoFlags.GRIBITrafficCheck {
			outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
			if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
				outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
			}
			args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{start_after_verification: true})
		}

		// RPFO
		if with_RPFO {
			rpfo_count = rpfo_count + 1
			t.Logf("This is RPFO #%d", rpfo_count)
			args.rpfo(args.ctx, t, true)
		}

		// ======================================================
		// REPLACE original NHG config, traffic via BE 121, 122, 123, 124, 125
		// VIP1 (192.0.2.40/32) --- 1000 (NHG) --- NH 1000 - BE 121
		//                                     --- NH 1100 - BE 122
		//                                     --- NH 1200 - BE 123
		//
		// VIP2 (192.0.2.41/32) --- 2000 (NHG) --- NH 2000 - BE 124
		//                                     --- NH 2100 - BE 125
		// ======================================================
		args.client.ReplaceNHG(t, 200000, 0, map[uint64]uint64{2000000: 50, 2100000: 50}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		// verify traffic
		if *ciscoFlags.GRIBITrafficCheck {
			outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
			if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
				outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
			}
			args.validateTrafficFlows(t, flows, false, outgoing_interface)
		}
	}
}

func testRestart_multiple_process(t *testing.T, args *testArgs) {

	// base programming
	baseProgramming(args.ctx, t, args)

	// create new flows and start traffic
	if *ciscoFlags.GRIBITrafficCheck {
		te_flow = args.allFlows(t)
		if base_config != "case1_backup_decap" {
			src_ip_flow = args.allFlows(t, &TGNoptions{SrcIP: "222.222.222.222"})
		}
		flows = append(te_flow, src_ip_flow...)
	}
	outgoing_interface := make(map[string][]string)

	// verify traffic
	if *ciscoFlags.GRIBITrafficCheck {
		outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
		if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
			outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
		}
		args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{burst: true, start_after_verification: true})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck && !with_scale {
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}

	processes := []string{"ifmgr", "ipv4_rib", "ipv6_rib", "db_writer", "fib_mgr"}
	for i := 0; i < len(processes); i++ {
		t.Run(processes[i], func(t *testing.T) {
			// RPFO
			if with_RPFO {
				rpfo_count = rpfo_count + 1
				t.Logf("This is RPFO #%d", rpfo_count)
				args.rpfo(args.ctx, t, false)
			}
			// verify traffic
			t.Logf("checking traffic after RPFO")
			if *ciscoFlags.GRIBITrafficCheck {
				outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
				if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
					outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
				}
				args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{start_after_verification: true})
			}

			// Restart process
			t.Logf("Restarting process %s", processes[i])
			ticker1 := time.NewTicker(8 * time.Second)
			ticker2 := time.NewTicker(10 * time.Second)
			if processes[i] == "fib_mgr" {
				// Restart process after 10seconds
				runner.RunCLIInBackground(args.ctx, t, args.dut, "process restart emsd", []string{"#"}, []string{".*Incomplete.*", ".*Unable.*"}, ticker1, 10*time.Second)
				runner.RunCLIInBackground(args.ctx, t, args.dut, "process restart fib_mgr location 0/RP0/CPU0", []string{"#"}, []string{".*Incomplete.*", ".*Unable.*"}, ticker2, 10*time.Second)
			} else {
				// Restart process after 10seconds
				runner.RunCLIInBackground(args.ctx, t, args.dut, "process restart emsd", []string{"#"}, []string{".*Incomplete.*", ".*Unable.*"}, ticker1, 10*time.Second)
				runner.RunCLIInBackground(args.ctx, t, args.dut, fmt.Sprintf("process restart %s", processes[i]), []string{"#"}, []string{".*Incomplete.*", ".*Unable.*"}, ticker2, 10*time.Second)
			}
			time.Sleep(12 * time.Second)
			ticker1.Stop()
			ticker2.Stop()
			time.Sleep(20 * time.Second)

			if *ciscoFlags.GRIBITrafficCheck {
				outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
				if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
					outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
				}
				args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{start_after_verification: true})
			}
			//aft check
			if *ciscoFlags.GRIBIAFTChainCheck && !with_scale {
				randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
				for i := 0; i < len(randomItems); i++ {
					args.client.CheckAftIPv4(t, "TE", randomItems[i])
				}
			}
		})
	}
}

func test_microdrops(t *testing.T, args *testArgs) {

	// base programming
	baseProgramming(args.ctx, t, args)

	// RPFO
	if with_RPFO {
		rpfo_count = rpfo_count + 1
		t.Logf("This is RPFO #%d", rpfo_count)
		args.rpfo(args.ctx, t, true)
	}

	// create new flows and start traffic
	if *ciscoFlags.GRIBITrafficCheck {
		te_flow = args.allFlows(t)
		if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
			src_ip_flow = args.allFlows(t, &TGNoptions{SrcIP: "222.222.222.222"})
		}
		flows = append(te_flow, src_ip_flow...)
	}
	outgoing_interface := make(map[string][]string)

	// verify traffic
	if *ciscoFlags.GRIBITrafficCheck {
		outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
		if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
			outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
		}
		args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{burst: true, start_after_verification: true})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck && !with_scale {
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}

	//Base level 1 scenario
	// ======================================================
	// VIP1 (192.0.2.40/32) --- 1000 (NHG) --- NH 1000 - BE 121
	//                                     --- NH 1100 - BE 122
	//                                     --- NH 1200 - BE 123
	//
	// VIP2 (192.0.2.41/32) --- 2000 (NHG) --- NH 2000 - BE 124
	//                                     --- NH 2100 - BE 125
	// ======================================================
	for i := 0; i < microdropsRepeat; i++ {

		// RPFO
		if with_RPFO {
			rpfo_count = rpfo_count + 1
			t.Logf("This is RPFO #%d", rpfo_count)
			args.rpfo(args.ctx, t, true)
		}

		// ======================================================
		// DELETE VIP1, traffic via BE 124, 125
		//                      --- 1000 (NHG) --- NH 1000 - BE 121
		//                                     --- NH 1100 - BE 122
		//                                     --- NH 1200 - BE 123
		//
		// VIP2 (192.0.2.41/32) --- 2000 (NHG) --- NH 2000 - BE 124
		//                                     --- NH 2100 - BE 125
		// ======================================================
		args.client.DeleteIPv4(t, "192.0.2.40/32", 100000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		if *ciscoFlags.GRIBITrafficCheck {
			outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
			if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
				outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
			}
			args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{start_after_verification: true})
		}

		// ======================================================
		// ADD VIP1 to point to 2000 NHG, traffic via BE 124, 125
		// VIP1 (192.0.2.40/32)
		//
		// VIP2 (192.0.2.41/32) --- 2000 (NHG) --- NH 2000 - BE 124
		//                                     --- NH 2100 - BE 125
		// ======================================================
		// Add back with different NHG i.e traffic over VIP2 NHG (NH BE 124, 125)
		args.client.AddIPv4(t, "192.0.2.40/32", 200000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		if *ciscoFlags.GRIBITrafficCheck {
			outgoing_interface["te_flow"] = []string{"Bundle-Ether124", "Bundle-Ether125"}
			if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
				outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether124", "Bundle-Ether125"}
			}
			args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{start_after_verification: true})
		}

		// ======================================================
		// UPDATE VIP1 to default state, traffic via BE 121,122,123,124,125
		// VIP1 (192.0.2.40/32) --- 1000 (NHG) --- NH 1000 - BE 121
		//                                     --- NH 1100 - BE 122
		//                                     --- NH 1200 - BE 123
		//
		// VIP2 (192.0.2.41/32) --- 2000 (NHG) --- NH 2000 - BE 124
		//                                     --- NH 2100 - BE 125
		// ======================================================
		args.client.ReplaceIPv4(t, "192.0.2.40/32", 100000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		if *ciscoFlags.GRIBITrafficCheck {
			outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
			if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
				outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
			}
			args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{start_after_verification: true})
		}

		// ======================================================
		// UPDATE VIP2 NHG to use VIP1 NH, traffic via BE 121,122,123
		// VIP1 (192.0.2.40/32) --- 1000 (NHG) --- NH 1000 - BE 121
		//                                     --- NH 1100 - BE 122
		//                                     --- NH 1200 - BE 123
		//
		// VIP2 (192.0.2.41/32) --- 1000 (NHG)
		// ======================================================
		args.client.ReplaceNHG(t, 200000, 0, map[uint64]uint64{1000000: 50, 1100000: 5, 1200000: 45}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		if *ciscoFlags.GRIBITrafficCheck {
			outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
			if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
				outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
			}
			args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{start_after_verification: true})
		}

		// ======================================================
		// UPDATE VIP1 NH outgoing interfaces, traffic via BE 123,124,125
		// VIP1 (192.0.2.40/32) --- 1000 (NHG) --- NH 1000 - BE 123
		//                                     --- NH 1100 - BE 124
		//                                     --- NH 1200 - BE 125
		//
		// VIP2 (192.0.2.41/32) --- 1000 (NHG)
		// ======================================================
		args.client.ReplaceNH(t, 1000000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.ReplaceNH(t, 1100000, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.ReplaceNH(t, 1200000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		if *ciscoFlags.GRIBITrafficCheck {
			outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
			if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
				outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
			}
			args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{start_after_verification: true})
		}

		// ======================================================
		// ADD and REPLACE original NH config, traffic via BE 121, 122, 123
		// VIP1 (192.0.2.40/32) --- 1000 (NHG) --- NH 1000 - BE 121
		//                                     --- NH 1100 - BE 122
		//                                     --- NH 1200 - BE 123
		//
		// VIP2 (192.0.2.41/32) --- 1000 (NHG)
		// ======================================================
		args.client.ReplaceNH(t, 1000000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.ReplaceNH(t, 1100000, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.ReplaceNH(t, 1200000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		if *ciscoFlags.GRIBITrafficCheck {
			outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
			if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
				outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
			}
			args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{start_after_verification: true})
		}

		// ======================================================
		// REPLACE original NHG config, traffic via BE 121, 122, 123, 124, 125
		// VIP1 (192.0.2.40/32) --- 1000 (NHG) --- NH 1000 - BE 121
		//                                     --- NH 1100 - BE 122
		//                                     --- NH 1200 - BE 123
		//
		// VIP2 (192.0.2.41/32) --- 2000 (NHG) --- NH 2000 - BE 124
		//                                     --- NH 2100 - BE 125
		// ======================================================
		args.client.ReplaceNHG(t, 200000, 0, map[uint64]uint64{2000000: 50, 2100000: 50}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		if *ciscoFlags.GRIBITrafficCheck {
			outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
			if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
				outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
			}
			args.validateTrafficFlows(t, flows, false, outgoing_interface)
		}
	}
}

//lint:ignore U1000 Ignore unused function temporarily for debugging
func test_multiple_clients(t *testing.T, args *testArgs) {
	args.ATELock = sync.Mutex{}
	testGroup := &sync.WaitGroup{}

	configureDeviceId(args.ctx, t, args.dut)
	configurePortId(args.ctx, t, args.dut)

	// if *ciscoFlags.GRIBITrafficCheck {
	// 	te_flow = args.allFlows(t)
	// 	if base_config != "case1_backup_decap" && base_config != "case3_decap_encap"{
	//		src_ip_flow = args.allFlows(t, &TGNoptions{SrcIP: "222.222.222.222"})
	//	}
	// 	flows = append(te_flow, src_ip_flow...)
	// }
	// outgoing_interface := make(map[string][]string)

	// verify traffic
	// if *ciscoFlags.GRIBITrafficCheck {
	// 	outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125"}
	// 	if base_config != "case1_backup_decap" && base_config != "case3_decap_encap"{
	//		outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether126"}
	//	}
	// 	// args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{burst: true, start_after_verification: true})
	// 	args.ate.Traffic().Start(t, flows...)
	// 	time.Sleep(120 * time.Second)
	// 	args.ate.Traffic().Stop(t)
	// }

	// multi_process_gribi_programming(t, args.events, args)
	p4rtPacketOut(t, args.events, args)
	// runner.RunTestInBackground(args.ctx, t, time.NewTimer(1*time.Second), testGroup, args.events, multi_process_gribi_programming, args)
	// runner.RunTestInBackground(args.ctx, t, time.NewTimer(1*time.Second), testGroup, args.events, p4rtPacketOut, args)

	testGroup.Wait()
}

//lint:ignore U1000 Ignore unused function temporarily for debugging
func multi_process_gribi_programming(t *testing.T, events *monitor.CachedConsumer, args ...interface{}) {

	// base programming
	arg := args[0].(*testArgs)
	baseProgramming(arg.ctx, t, arg)
}

func test_triggers(t *testing.T, args *testArgs) {

	// base programming
	baseProgramming(args.ctx, t, args)

	// create new flows and start traffic
	if *ciscoFlags.GRIBITrafficCheck {
		te_flow = args.allFlows(t)
		if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
			src_ip_flow = args.allFlows(t, &TGNoptions{SrcIP: "222.222.222.222"})
		}
		flows = append(te_flow, src_ip_flow...)
	}

	outgoing_interface := make(map[string][]string)

	// verify traffic
	if *ciscoFlags.GRIBITrafficCheck {
		if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
			outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether127"}
		}
		outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126"}
		args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{burst: true, start_after_verification: true})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck && !with_scale {
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}

	processes := []string{"shutdown", "disconnect_gribi_reconnect", "delete_vrfs", "grpc_config_change", "grpc_AF_change", "LC_OIR", "viable"}
	for i := 0; i < len(processes); i++ {
		t.Run(processes[i], func(t *testing.T) {
			if processes[i] == "shutdown" {

				// Run with RPFO is flag is set
				if with_RPFO {
					rpfo_count = rpfo_count + 1
					t.Logf("This is RPFO #%d", rpfo_count)
					args.rpfo(args.ctx, t, false)
				}

				t.Logf("Shutting down primary interfaces BE121, BE122, BE123, BE124, BE125")
				args.interfaceaction(t, false, []string{"port2", "port3", "port4", "port5", "port6"})
				defer args.interfaceaction(t, true, []string{"port2", "port3", "port4", "port5", "port6"})

				//aft check TE
				if *ciscoFlags.GRIBIAFTChainCheck && !with_scale {
					args.client.AftPushConfig(t)
					args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort6.IPv4)
					args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort5.IPv4)
					args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort4.IPv4)
					args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort3.IPv4)
					args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort2.IPv4)
					randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
					for i := 0; i < len(randomItems); i++ {
						args.client.CheckAftIPv4(t, "TE", randomItems[i])
					}
				}
				//aft check REPAIRED
				if *ciscoFlags.GRIBIAFTChainCheck && !with_scale && base_config != "case1_backup_decap" {
					randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
					for i := 0; i < len(randomItems); i++ {
						args.client.CheckAftIPv4(t, "REPAIRED", randomItems[i])
					}
				}
				// verify traffic
				if *ciscoFlags.GRIBITrafficCheck {
					if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
						outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether127"}
					}
					outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126"}
					args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{tolerance: 15, start_after_verification: true})
				}

				t.Logf("Shutting down interfaces BE126 so traffic flows via DECAP path")
				args.interfaceaction(t, false, []string{"port7"})
				defer args.interfaceaction(t, true, []string{"port7"})

				//aft check TE
				if *ciscoFlags.GRIBIAFTChainCheck && !with_scale {
					args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort7.IPv4)
					randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
					for i := 0; i < len(randomItems); i++ {
						args.client.CheckAftIPv4(t, "TE", randomItems[i])
					}
				}
				//aft check REPAIRED
				if *ciscoFlags.GRIBIAFTChainCheck && !with_scale {
					randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
					for i := 0; i < len(randomItems); i++ {
						args.client.CheckAftIPv4(t, "REPAIRED", randomItems[i])
					}
				}
				// verify traffic
				if *ciscoFlags.GRIBITrafficCheck {
					if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
						outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether127"}
					}
					outgoing_interface["te_flow"] = []string{"Bundle-Ether126", "Bundle-Ether127"}
					args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{tolerance: 10, start_after_verification: true})
				}

				t.Logf("Unshut primary interfaces and verify traffic restored to original interfaces")
				args.interfaceaction(t, true, []string{"port2", "port3", "port4", "port5", "port6", "port7"})
				//aft check TE
				if *ciscoFlags.GRIBIAFTChainCheck && !with_scale {
					args.client.AftPopConfig(t)
					randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
					for i := 0; i < len(randomItems); i++ {
						args.client.CheckAftIPv4(t, "TE", randomItems[i])
					}
				}
				//aft check REPAIRED
				if *ciscoFlags.GRIBIAFTChainCheck && !with_scale {
					randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
					for i := 0; i < len(randomItems); i++ {
						args.client.CheckAftIPv4(t, "REPAIRED", randomItems[i])
					}
				}
				// verify traffic
				if *ciscoFlags.GRIBITrafficCheck {
					if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
						outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether127"}
					}
					outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126", "Bundle-Ether127"}
					args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{tolerance: 5, start_after_verification: true})
				}
			}

			if processes[i] == "viable" {

				// Run with RPFO is flag is set
				if with_RPFO {
					rpfo_count = rpfo_count + 1
					t.Logf("This is RPFO #%d", rpfo_count)
					args.rpfo(args.ctx, t, false)
				}

				t.Logf("Shutting down primary interfaces BE121, BE122, BE123, BE124, BE125")
				gnmi.Update(t, args.dut, gnmi.OC().Interface(sortPorts(args.dut.Ports())[1].Name()).ForwardingViable().Config(), false)
				gnmi.Update(t, args.dut, gnmi.OC().Interface(sortPorts(args.dut.Ports())[2].Name()).ForwardingViable().Config(), false)
				gnmi.Update(t, args.dut, gnmi.OC().Interface(sortPorts(args.dut.Ports())[3].Name()).ForwardingViable().Config(), false)
				gnmi.Update(t, args.dut, gnmi.OC().Interface(sortPorts(args.dut.Ports())[4].Name()).ForwardingViable().Config(), false)
				gnmi.Update(t, args.dut, gnmi.OC().Interface(sortPorts(args.dut.Ports())[5].Name()).ForwardingViable().Config(), false)

				//aft check TE
				if *ciscoFlags.GRIBIAFTChainCheck && !with_scale {
					args.client.AftPushConfig(t)
					args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort6.IPv4)
					args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort5.IPv4)
					args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort4.IPv4)
					args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort3.IPv4)
					args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort2.IPv4)
					randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
					for i := 0; i < len(randomItems); i++ {
						args.client.CheckAftIPv4(t, "TE", randomItems[i])
					}
				}
				//aft check REPAIRED
				if *ciscoFlags.GRIBIAFTChainCheck && !with_scale && base_config != "case1_backup_decap" {
					randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
					for i := 0; i < len(randomItems); i++ {
						args.client.CheckAftIPv4(t, "REPAIRED", randomItems[i])
					}
				}
				// verify traffic
				if *ciscoFlags.GRIBITrafficCheck {
					if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
						outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether127"}
					}
					outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126"}
					args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{tolerance: 15, start_after_verification: true})
				}

				t.Logf("Shutting down interfaces BE126 so traffic flows via DECAP path")
				gnmi.Update(t, args.dut, gnmi.OC().Interface(sortPorts(args.dut.Ports())[6].Name()).ForwardingViable().Config(), false)

				//aft check TE
				if *ciscoFlags.GRIBIAFTChainCheck && !with_scale {
					args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort7.IPv4)
					randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
					for i := 0; i < len(randomItems); i++ {
						args.client.CheckAftIPv4(t, "TE", randomItems[i])
					}
				}
				//aft check REPAIRED
				if *ciscoFlags.GRIBIAFTChainCheck && !with_scale {
					randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
					for i := 0; i < len(randomItems); i++ {
						args.client.CheckAftIPv4(t, "REPAIRED", randomItems[i])
					}
				}
				// verify traffic
				if *ciscoFlags.GRIBITrafficCheck {
					if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
						outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether127"}
					}
					outgoing_interface["te_flow"] = []string{"Bundle-Ether126", "Bundle-Ether127"}
					args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{tolerance: 10, start_after_verification: true})
				}

				t.Logf("Unshut primary interfaces and verify traffic restored to original interfaces")
				gnmi.Update(t, args.dut, gnmi.OC().Interface(sortPorts(args.dut.Ports())[1].Name()).ForwardingViable().Config(), true)
				gnmi.Update(t, args.dut, gnmi.OC().Interface(sortPorts(args.dut.Ports())[2].Name()).ForwardingViable().Config(), true)
				gnmi.Update(t, args.dut, gnmi.OC().Interface(sortPorts(args.dut.Ports())[3].Name()).ForwardingViable().Config(), true)
				gnmi.Update(t, args.dut, gnmi.OC().Interface(sortPorts(args.dut.Ports())[4].Name()).ForwardingViable().Config(), true)
				gnmi.Update(t, args.dut, gnmi.OC().Interface(sortPorts(args.dut.Ports())[5].Name()).ForwardingViable().Config(), true)
				gnmi.Update(t, args.dut, gnmi.OC().Interface(sortPorts(args.dut.Ports())[6].Name()).ForwardingViable().Config(), true)

				//aft check TE
				if *ciscoFlags.GRIBIAFTChainCheck && !with_scale {
					args.client.AftPopConfig(t)
					randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
					for i := 0; i < len(randomItems); i++ {
						args.client.CheckAftIPv4(t, "TE", randomItems[i])
					}
				}
				//aft check REPAIRED
				if *ciscoFlags.GRIBIAFTChainCheck && !with_scale {
					randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
					for i := 0; i < len(randomItems); i++ {
						args.client.CheckAftIPv4(t, "REPAIRED", randomItems[i])
					}
				}
				// verify traffic
				if *ciscoFlags.GRIBITrafficCheck {
					if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
						outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether127"}
					}
					outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126", "Bundle-Ether127"}
					args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{tolerance: 5, start_after_verification: true})
				}
			}

			if processes[i] == "disconnect_gribi_reconnect" {
				t.Logf("Disconnect client")
				args.client.Close(t)

				// Perform RPFO if flag is set
				if with_RPFO {
					rpfo_count = rpfo_count + 1
					t.Logf("This is RPFO #%d", rpfo_count)
					args.rpfo(args.ctx, t, true)
				} else {
					t.Logf("reconnect, flush entries, reprogram and validate traffic")
					if err := args.client.Start(t); err != nil {
						t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
						if err = args.client.Start(t); err != nil {
							t.Fatalf("gRIBI Connection could not be established: %v", err)
						}
					}
				}

				args.client.BecomeLeader(t)
				args.client.FlushServer(t)
				time.Sleep(10 * time.Second)

				// base programming
				baseProgramming(args.ctx, t, args)

				// verify traffic
				if *ciscoFlags.GRIBITrafficCheck {
					if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
						outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether127"}
					}
					outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126"}
					args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{tolerance: 100, start_after_verification: true})
				}

				// ======================================================
				// DELETE VIP1, traffic via BE 124, 125
				//                      --- 1000 (NHG) --- NH 1000 - BE 121
				//                                     --- NH 1100 - BE 122
				//                                     --- NH 1200 - BE 123
				//
				// VIP2 (192.0.2.41/32) --- 2000 (NHG) --- NH 2000 - BE 124
				//                                     --- NH 2100 - BE 125
				// ======================================================
				args.client.DeleteIPv4(t, "192.0.2.40/32", 100000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
				if *ciscoFlags.GRIBITrafficCheck {
					if base_config != "case1_backup_decap" {
						outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether127"}
					}
					outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126"}
					args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{start_after_verification: true})
				}

				// ======================================================
				// ADD VIP1 to point to 2000 NHG, traffic via BE 124, 125
				// VIP1 (192.0.2.40/32)
				//
				// VIP2 (192.0.2.41/32) --- 2000 (NHG) --- NH 2000 - BE 124
				//                                     --- NH 2100 - BE 125
				// ======================================================
				// Add back with different NHG i.e traffic over VIP2 NHG (NH BE 124, 125)
				args.client.AddIPv4(t, "192.0.2.40/32", 200000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
				if *ciscoFlags.GRIBITrafficCheck {
					if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
						outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether127"}
					}
					outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126"}
					args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{start_after_verification: true})
				}

				// ======================================================
				// UPDATE VIP1 to default state, traffic via BE 121,122,123,124,125
				// VIP1 (192.0.2.40/32) --- 1000 (NHG) --- NH 1000 - BE 121
				//                                     --- NH 1100 - BE 122
				//                                     --- NH 1200 - BE 123
				//
				// VIP2 (192.0.2.41/32) --- 2000 (NHG) --- NH 2000 - BE 124
				//                                     --- NH 2100 - BE 125
				// ======================================================
				args.client.ReplaceIPv4(t, "192.0.2.40/32", 100000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
				if *ciscoFlags.GRIBITrafficCheck {
					if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
						outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126"}
					}
					outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126"}
					args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{start_after_verification: true})
				}
			}

			if processes[i] == "delete_vrfs" {
				t.Skip("skipping vrf delete till we have the fix")
				t.Logf("Delete vrfs and validate traffic is failing as prefix are deleted")
				args.gnmiConf(t, "no vrf TE \n no vrf REPAIR \n no vrf REPAIRED")
				defer args.gnmiConf(t, "vrf TE \n vrf REPAIR \n vrf REPAIRED")

				if *ciscoFlags.GRIBITrafficCheck {
					if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
						outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126"}
					}
					outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126"}
					args.validateTrafficFlows(t, flows, true, outgoing_interface, &TGNoptions{start_after_verification: true})
				}

				if with_RPFO {
					rpfo_count = rpfo_count + 1
					t.Logf("This is RPFO #%d", rpfo_count)
					args.rpfo(args.ctx, t, true)
				}
				t.Logf("Reprogram prefixes to use the same forwarding chaing")
				args.gnmiConf(t, "vrf TE \n vrf REPAIR \n vrf REPAIRED")

				if base_config == "case1_backup_decap" {
					if with_scale {
						args.scaleIPV4(t, vrf1, 1000)
					} else {
						args.client.AddIPv4Batch(t, prefixes, 10, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
					}
				} else if base_config == "case2_decap_encap_exit" {
					if with_scale {
						args.scaleIPV4(t, vrf1, 10000)
						args.scaleIPV4(t, vrf2, 10000)
						args.scaleIPV4(t, vrf3, 30000, &IPv4ScaleOptions{max: nhg_Scale_REPAIR})
					} else {
						args.client.AddIPv4Batch(t, prefixes, 100, vrf1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
						args.client.AddIPv4Batch(t, prefixes, 200, vrf2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
						args.client.AddIPv4Batch(t, repair_prefix, 333333, vrf3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
					}
				}

				t.Logf("Verify traffic after adding back vrfs")
				if *ciscoFlags.GRIBITrafficCheck {
					if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
						outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126"}
					}
					outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126"}
					args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{burst: true, start_after_verification: true})
				}
			}

			if processes[i] == "LC_OIR" {
				// stop traffic and perform LC_OIR
				if *ciscoFlags.GRIBITrafficCheck {
					args.ate.Traffic().Stop(t)
				}

				gnoiClient := args.dut.RawAPIs().GNOI().Default(t)
				useNameOnly := deviations.GNOISubcomponentPath(args.dut)
				lineCardPath := components.GetSubcomponentPath(lc, useNameOnly)
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

				if with_RPFO {
					rpfo_count = rpfo_count + 1
					t.Logf("This is RPFO #%d", rpfo_count)
					args.rpfo(args.ctx, t, true)
				}
				// sleep while lc reloads
				time.Sleep(10 * time.Minute)

				// base programming
				baseProgramming(args.ctx, t, args)

				// verify traffic
				if *ciscoFlags.GRIBITrafficCheck {
					if base_config != "case1_backup_decap" && base_config != "case3_decap_encap" {
						outgoing_interface["src_ip_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126"}
					}
					outgoing_interface["te_flow"] = []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126"}
					args.validateTrafficFlows(t, flows, false, outgoing_interface, &TGNoptions{burst: true})
				}
			}

			if processes[i] == "grpc_AF_change" {
				// kill previous gribi client
				args.client.Close(t)

				for j := 0; j < grpc_repeat; j++ {

					if with_RPFO {
						rpfo_count = rpfo_count + 1
						t.Logf("This is RPFO #%d", rpfo_count)
						args.rpfo(args.ctx, t, true)
					}
					sshClient := args.dut.RawAPIs().CLI(t)
					defer sshClient.Close()
					time.Sleep(10 * time.Second)

					config.TextWithSSH(args.ctx, t, args.dut, "configure \n grpc \n address-family ipv6 \n commit \n", 30*time.Second)
					response, _ := sshClient.SendCommand(args.ctx, "show grpc")
					t.Logf("grpc value after configuring ipv6 af %s", response)

					config.TextWithSSH(args.ctx, t, args.dut, "configure \n grpc \n address-family ipv4 \n commit \n", 30*time.Second)
					response, _ = sshClient.SendCommand(args.ctx, "show grpc")
					t.Logf("grpc value after configuring only ipv4 af %s", response)

					config.TextWithSSH(args.ctx, t, args.dut, "configure \n grpc \n address-family dual \n commit \n", 30*time.Second)
					response, _ = sshClient.SendCommand(args.ctx, "show grpc")
					t.Logf("grpc value after configuring dual af %s", response)

					client := gribi.Client{
						DUT:                   args.dut,
						FibACK:                *ciscoFlags.GRIBIFIBCheck,
						Persistence:           true,
						InitialElectionIDLow:  1,
						InitialElectionIDHigh: 0,
					}
					if err := client.Start(t); err != nil {
						t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
						if err = client.Start(t); err != nil {
							t.Fatalf("gRIBI Connection could not be established: %v", err)
						}
					}
					args.client = &client
					baseProgramming(args.ctx, t, args)

					// keeping last gribi client up for the next tc's
					if j < grpc_repeat-1 {
						client.Close(t)
					}
				}
			}

			if processes[i] == "grpc_config_change" {
				// kill previous gribi client
				args.client.Close(t)

				rand.Seed(time.Now().UnixNano())
				min := 57344
				max := 57998
				for k := 0; k < grpc_repeat; k++ {
					if with_RPFO {
						rpfo_count = rpfo_count + 1
						t.Logf("This is RPFO #%d", rpfo_count)
						args.rpfo(args.ctx, t, true)
					}
					sshClient := args.dut.RawAPIs().CLI(t)
					defer sshClient.Close()
					time.Sleep(10 * time.Second)

					port := rand.Intn(max-min+1) + min

					response, _ := sshClient.SendCommand(args.ctx, "show grpc")
					t.Logf("initial grpc values: %s", response)

					config.TextWithSSH(args.ctx, t, args.dut, fmt.Sprintf("configure \n grpc \n no-tls \n port %s \n commit \n", strconv.Itoa(port)), 30*time.Second)

					response, _ = sshClient.SendCommand(args.ctx, "show grpc")
					t.Logf("grpc value after no-tls and random port %s", response)

					config.TextWithSSH(args.ctx, t, args.dut, fmt.Sprintf("configure \n grpc \n no no-tls \n no port %s \n commit \n", strconv.Itoa(port)), 30*time.Second)

					response, _ = sshClient.SendCommand(args.ctx, "show grpc")
					t.Logf("grpc value with no no-tls and no port %s configured", response)

					client := gribi.Client{
						DUT:                   args.dut,
						FibACK:                *ciscoFlags.GRIBIFIBCheck,
						Persistence:           true,
						InitialElectionIDLow:  1,
						InitialElectionIDHigh: 0,
					}
					if err := client.Start(t); err != nil {
						t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
						if err = client.Start(t); err != nil {
							t.Fatalf("gRIBI Connection could not be established: %v", err)
						}
					}
					args.client = &client
					baseProgramming(args.ctx, t, args)

					// keeping last gribi client up for the next tc's
					if k < grpc_repeat-1 {
						client.Close(t)
					}
				}
			}
		})
	}
}

func TestHA(t *testing.T) {
	t.Log("Name: HA")
	t.Log("Description: Connect gRIBI client to DUT using SINGLE_PRIMARY client redundancy with persistance, RibACK and FibACK")

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	// ctx, cancelMonitors := context.WithCancel(context.Background())

	// Configure the DUT
	var vrfs = []string{vrf1, vrf2, vrf3, vrf4}
	configVRF(t, dut, vrfs)
	configureDUT(t, dut)
	// PBR config
	configbasePBR(t, dut, "REPAIRED", "ipv4", 1, "pbr", oc.PacketMatchTypes_IP_PROTOCOL_UNSET, []uint8{}, &PBROptions{SrcIP: "222.222.222.222/32"})
	configbasePBR(t, dut, "TE", "ipv4", 2, "pbr", oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})
	configbasePBRInt(t, dut, "Bundle-Ether120", "pbr")
	// RoutePolicy config
	configRP(t, dut)
	// configure ISIS on DUT
	addISISOC(t, dut, "Bundle-Ether127")
	// configure BGP on DUT
	addBGPOC(t, dut, "100.100.100.100")
	// Configure P4RT device-id and port-id
	configureDeviceId(ctx, t, dut)
	configurePortId(ctx, t, dut)

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	if *ciscoFlags.GRIBITrafficCheck {
		addPrototoAte(t, top)
		time.Sleep(120 * time.Second)
	}

	test := []struct {
		name string
		desc string
		fn   func(t *testing.T, args *testArgs)
	}{
		// {
		// 	name: "check_microdrops",
		// 	desc: "With traffic running do delete/update/create programming and look for drops",
		// 	fn:   test_microdrops,
		// },
		// {
		// 	name: "Restart RFPO with programming",
		// 	desc: "After programming, perform RPFO try new programming and validate traffic",
		// 	fn:   test_RFPO_with_programming,
		// },
		// {
		// 	name: "Restart single process",
		// 	desc: "After programming, restart fib_mgr, isis, ifmgr, ipv4_rib, ipv6_rib, emsd, db_writer and valid programming exists",
		// 	fn:   testRestart_single_process,
		// },
		// {
		// 	name: "Restart multiple process",
		// 	desc: "After programming, restart multiple process fib_mgr, isis, ifmgr, ipv4_rib, ipv6_rib, emsd, db_writer and valid programming exists",
		// 	fn:   testRestart_multiple_process,
		// },
		{
			name: "Triggers",
			desc: "With traffic running, validate multiple triggers",
			fn:   test_triggers,
		},
		// {
		// 	name: "check multiple clients",
		// 	desc: "With traffic running, validate use of multiple clients",
		// 	fn:   test_multiple_clients,
		// },
	}
	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			t.Logf("Description: %s", tt.desc)

			// Configure the gRIBI client client
			client := gribi.Client{
				DUT:                   dut,
				FibACK:                *ciscoFlags.GRIBIFIBCheck,
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

			//Monitor and eventConsumer
			t.Log("creating event monitor")
			eventConsumer := monitor.NewCachedConsumer(2*time.Hour, /*expiration time for events in the cache*/
				1 /*number of events for keep for each leaf*/)
			// monitor := monitor.GNMIMonior{
			//  Paths: []ygnmi.PathStruct{
			//      gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Afts(),
			//      gnmi.OC().NetworkInstance(vrf1).Afts(),
			//      gnmi.OC().NetworkInstance(vrf2).Afts(),
			//      gnmi.OC().NetworkInstance(vrf3).Afts(),
			//      gnmi.OC().NetworkInstance(vrf4).Afts(),
			//  },
			//  Consumer: eventConsumer,
			//  DUT:      dut,
			// }
			// monitor.Start(ctx, t, true, gpb.SubscriptionList_STREAM)
			// defer cancelMonitors()

			args := &testArgs{
				ctx:    ctx,
				client: &client,
				dut:    dut,
				ate:    ate,
				top:    top,
				events: eventConsumer,
			}
			tt.fn(t, args)
		})
	}
}
