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

package qos_recycle

import (
	"context"
	"fmt"
	"net"

	//"sort"

	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/cisco/config"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/gribi"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	spb "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the test
// topology.
//
// The testbed consists of dut:port1 -> ate:port1,
// dut:port2 -> ate:port2, dut:port3 -> ate:port3, dut:port4 -> ate:port4, dut:port5 -> ate:port5,
// dut:port6 -> ate:port6, dut:port7 -> ate:port7 ,dut:port8 -> ate:port8
//
//   * ate:port1 -> dut:port1 subnet 192.0.2.0/30
//   * ate:port2 -> dut:port2 subnet 192.0.2.4/30
//   * ate:port3 -> dut:port3 subnet 192.0.2.8/30
//   * ate:port4 -> dut:port3 subnet 192.0.2.12/30
//   * ate:port5 -> dut:port3 subnet 192.0.2.16/30
//   * ate:port6 -> dut:port3 subnet 192.0.2.20/30
//   * ate:port7 -> dut:port3 subnet 192.0.2.24/30
//   * ate:port8 -> dut:port3 subnet 192.0.2.28/30
//
//   * Destination network: 198.51.100.0/24

const (
	ipv4PrefixLen         = 30
	ipv6PrefixLen         = 126
	dstPfx                = "198.51.100.1"
	mask                  = "32"
	dstPfxMin             = "198.51.100.1"
	dstPfx1               = "11.1.1.1"
	dstPfxCount1          = 10
	innersrcPfx           = "200.1.0.1"
	innerdstPfxMin_bgp    = "202.1.0.1"
	innerdstPfxCount_bgp  = 10
	innerdstPfxMin_isis   = "201.1.0.1"
	innerdstPfxCount_isis = 10
	bundleEther121        = "Bundle-Ether121"
	bundleEther122        = "Bundle-Ether122"
	bundleEther123        = "Bundle-Ether123"
	bundleEther124        = "Bundle-Ether124"
	lc                    = "0/2/CPU0"
	vrf1                  = "TE"
	vrf2                  = "REPAIRED"
	vrf3                  = "REPAIR"
	vrf4                  = "DECAP"
)

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx    context.Context
	client *gribi.Client
	dut    *ondatra.DUTDevice
	ate    *ondatra.ATEDevice
	top    *ondatra.ATETopology
}

func testBackupToDecapQos(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)
	t.Logf("Configure Qos Entries")
	ConfigureWrr(t, args.dut)
	defer teardownQos(t, args.dut)

	// adding default route pointing to Valid Path
	t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	// LEVEL 1
	// VIP1: NHG ID 1000
	//		- PATH1 NH ID 1000, weight 50, outgoing Port2
	//		- PATH2 NH ID 1100, weight 30, outgoing Port3
	//		- PATH3 NH ID 1200, weight 15, outgoing Port4
	//		- PATH4 NH ID 1300, weight  5, outgoing Port5
	// VIP2: NHG ID 2000
	//		- PATH1 NH ID 2000, weight 60, outgoing Port6
	//		- PATH2 NH ID 2100, weight 35, outgoing Port7
	//		- PATH3 NH ID 2200, weight  5, outgoing Port8
	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// LEVEL 2
	// Creating a backup NHG with ID 101 and NH ID 10 pointing to decap
	args.client.AddNH(t, 10, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Creating NHG ID 100 using backup NHG ID 101
	// PATH 1 NH ID 100, weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200, weight 15, VIP2 : 192.0.2.42
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 200, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlowsQos(), false, []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126", "Bundle-Ether127"})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}
	args.ValidateQosStats(t, []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126", "Bundle-Ether127"})
	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	args.interfaceaction(t, "port7", false)
	args.interfaceaction(t, "port6", false)
	defer args.interfaceaction(t, "port7", true)
	defer args.interfaceaction(t, "port6", true)
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlowsQos(), false, []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether127"})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		args.client.AftPushConfig(t)
		args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort7.IPv4)
		args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort6.IPv4)
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}

	args.interfaceaction(t, "port5", false)
	args.interfaceaction(t, "port4", false)
	args.interfaceaction(t, "port3", false)
	args.interfaceaction(t, "port2", false)
	defer args.interfaceaction(t, "port5", true)
	defer args.interfaceaction(t, "port4", true)
	defer args.interfaceaction(t, "port3", true)
	defer args.interfaceaction(t, "port2", true)
	// validate traffic decap over backup path

	//resp, err = CMDViaGNMI(context.Background(), "clear qos counters interface all")
	cliHandle := args.dut.RawAPIs().CLI(t)
	resp, err := cliHandle.SendCommand(context.Background(), "clear qos counters interface all")
	t.Logf(resp, err)
	time.Sleep(3 * time.Minute)
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlowsQos(), false, []string{"Bundle-Ether127"})
	}
	//time.Sleep(12 * time.Hour)
	args.ValidateQosStats(t, []string{"Bundle-Ether127"})
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort5.IPv4)
		args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort4.IPv4)
		args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort3.IPv4)
		args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort2.IPv4)
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}
}

func testFlushForwarding(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	// LEVEL 2
	// Creating a backup NHG with ID 101 and NH ID 10 pointing to a decap
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 10, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 101, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	//shutdown primary path
	args.interfaceaction(t, "port7", false)
	args.interfaceaction(t, "port6", false)
	args.interfaceaction(t, "port5", false)
	args.interfaceaction(t, "port4", false)
	args.interfaceaction(t, "port3", false)
	args.interfaceaction(t, "port2", false)
	defer args.interfaceaction(t, "port7", true)
	defer args.interfaceaction(t, "port6", true)
	defer args.interfaceaction(t, "port5", true)
	defer args.interfaceaction(t, "port4", true)
	defer args.interfaceaction(t, "port3", true)
	defer args.interfaceaction(t, "port2", true)

	//Validate traffic over backup is passing
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether127"})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}

	//flush all the entries
	args.client.FlushServer(t)

	//Validate traffic dropping after deleting forwarding
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), true, []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126"})
	}

	//unshut links
	args.interfaceaction(t, "port2", true)
	args.interfaceaction(t, "port3", true)
	args.interfaceaction(t, "port4", true)
	args.interfaceaction(t, "port5", true)
	args.interfaceaction(t, "port6", true)
	args.interfaceaction(t, "port7", true)

	// LEVEL 1
	// VIP1: NHG ID 1000
	//		- PATH1 NH ID 1000, weight 50, outgoing Port2
	//		- PATH2 NH ID 1100, weight 30, outgoing Port3
	//		- PATH3 NH ID 1200, weight 15, outgoing Port4
	//		- PATH4 NH ID 1300, weight  5, outgoing Port5
	// VIP2: NHG ID 2000
	//		- PATH1 NH ID 2000, weight 60, outgoing Port6
	//		- PATH2 NH ID 2100, weight 35, outgoing Port7
	//		- PATH3 NH ID 2200, weight  5, outgoing Port8
	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// LEVEL 2
	// Creating a backup NHG with ID 101 and NH ID 10 pointing to a decap
	args.client.AddNH(t, 10, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Creating NHG ID 100 using backup NHG ID 101
	// PATH 1 NH ID 100, weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200, weight 15, VIP2 : 192.0.2.42
	prefixes = []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 200, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// validate traffic passing over primary path
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126"})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		args.client.AftPopConfig(t)
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}

	//shutdown primary path
	args.interfaceaction(t, "port7", false)
	args.interfaceaction(t, "port6", false)
	args.interfaceaction(t, "port5", false)
	args.interfaceaction(t, "port4", false)
	args.interfaceaction(t, "port3", false)
	args.interfaceaction(t, "port2", false)

	//Validate traffic over backup
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether127"})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		args.client.AftPushConfig(t)
		args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort7.IPv4)
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

	//flush the entries
	args.client.FlushServer(t)
	//Validate traffic failing
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), true, []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126", "Bundle-Ether127"})
	}
}

func testBackupSwitchFromDropToDecap(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)
	ConfigureWrr(t, args.dut)
	defer teardownQos(t, args.dut)
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	if *ciscoFlags.GRIBITrafficCheck {
		addPrototoAte(t, top)
	}

	t.Log("Adding a drop route to Null")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 192.0.2.29/32 Null0")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 192.0.2.29/32 Null0")

	// adding default route pointing to Valid Path
	t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	// LEVEL 1
	// VIP1: NHG ID 1000
	//		- PATH1 NH ID 1000, weight 50, outgoing Port2
	//		- PATH2 NH ID 1100, weight 30, outgoing Port3
	//		- PATH3 NH ID 1200, weight 15, outgoing Port4
	//		- PATH4 NH ID 1300, weight  5, outgoing Port5
	// VIP2: NHG ID 2000
	//		- PATH1 NH ID 2000, weight 60, outgoing Port6
	//		- PATH2 NH ID 2100, weight 35, outgoing Port7
	//		- PATH3 NH ID 2200, weight  5, outgoing Port8
	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// LEVEL 2
	// Creating a backup NHG with ID 101 and NH ID 10 pointing to a drop address
	args.client.AddNH(t, 10, "192.0.2.29", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Creating NHG ID 100 using backup NHG ID 101
	// PATH 1 NH ID 100, weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200, weight 15, VIP2 : 192.0.2.42
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 200, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlowsQos(), false, []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126"})
	}
	//time.Sleep(5 * time.Hour)
	args.ValidateQosStats(t, []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126"})
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}

	//shutdown primary path
	args.interfaceaction(t, "port7", false)
	args.interfaceaction(t, "port6", false)
	args.interfaceaction(t, "port5", false)
	args.interfaceaction(t, "port4", false)
	args.interfaceaction(t, "port3", false)
	args.interfaceaction(t, "port2", false)
	defer args.interfaceaction(t, "port7", true)
	defer args.interfaceaction(t, "port6", true)
	defer args.interfaceaction(t, "port5", true)
	defer args.interfaceaction(t, "port4", true)
	defer args.interfaceaction(t, "port3", true)
	defer args.interfaceaction(t, "port2", true)

	//Validate traffic over backup is failing
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlowsQos(), true, []string{"Bundle-Ether127"})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		args.client.AftPushConfig(t)
		args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort7.IPv4)
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

	config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 192.0.2.29/32 Null0")

	// Modify backup from pointing to a static route to a DECAP chain
	args.client.AddNH(t, 999, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNHG(t, 101, 0, map[uint64]uint64{999: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	// validate traffic decap over backup path
	cliHandle := args.dut.RawAPIs().CLI(t)
	resp, err := cliHandle.SendCommand(context.Background(), "clear qos counters interface all")
	t.Logf(resp, err)
	time.Sleep(3 * time.Minute)
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlowsQos(), false, []string{"Bundle-Ether127"})
	}
	//time.Sleep(12 * time.Hour)
	args.ValidateQosStats(t, []string{"Bundle-Ether127"})

	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}
}

func testUpdateBackupToDifferentNHG(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	// adding default route pointing to Valid Path
	t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	// LEVEL 1
	// VIP1: NHG ID 1000
	//		- PATH1 NH ID 1000, weight 50, outgoing Port2
	//		- PATH2 NH ID 1100, weight 30, outgoing Port3
	//		- PATH3 NH ID 1200, weight 15, outgoing Port4
	//		- PATH4 NH ID 1300, weight  5, outgoing Port5
	// VIP2: NHG ID 2000
	//		- PATH1 NH ID 2000, weight 60, outgoing Port6
	//		- PATH2 NH ID 2100, weight 35, outgoing Port7
	//		- PATH3 NH ID 2200, weight  5, outgoing Port8
	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// LEVEL 2
	// Creating a backup NHG with ID 101 and NH ID 10 pointing to a decap
	args.client.AddNH(t, 10, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 201, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Create REPAIRED INSTANCE POINTING TO THE SAME LEVEL1 VIPS
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 200, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 200, 201, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 200, "REPAIRED", *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 35, 200: 65}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126", "Bundle-Ether127"})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}
}

func testValidateForwardingChain(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	// LEVEL 1
	// VIP1: NHG ID 1000
	//		- PATH1 NH ID 1000, weight 50, outgoing Port2
	//		- PATH2 NH ID 1100, weight 30, outgoing Port3
	//		- PATH3 NH ID 1200, weight 15, outgoing Port4
	//		- PATH4 NH ID 1300, weight  5, outgoing Port5
	// VIP2: NHG ID 2000
	//		- PATH1 NH ID 2000, weight 60, outgoing Port6
	//		- PATH2 NH ID 2100, weight 35, outgoing Port7
	//		- PATH3 NH ID 2200, weight  5, outgoing Port8
	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// LEVEL 2
	// Creating a backup NHG with ID 101 and NH ID 10 pointing to a decap
	args.client.AddNH(t, 10, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Creating NHG ID 100 using backup NHG ID 101
	// PATH 1 NH ID 100, weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200, weight 15, VIP2 : 192.0.2.42
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 200, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// get gribi contents
	getResponse1, err1 := args.client.Fluent(t).Get().AllNetworkInstances().WithAFT(fluent.IPv4).Send()
	getResponse2, err2 := args.client.Fluent(t).Get().AllNetworkInstances().WithAFT(fluent.AllAFTs).Send()
	getResponse3, err3 := args.client.Fluent(t).Get().AllNetworkInstances().WithAFT(fluent.NextHopGroup).Send()
	getResponse4, err4 := args.client.Fluent(t).Get().AllNetworkInstances().WithAFT(fluent.NextHop).Send()

	var pref []string
	if err1 != nil && err2 != nil && err3 != nil && err4 != nil {
		t.Errorf("Cannot Get")
	}

	entries1 := getResponse1.GetEntry()
	entries2 := getResponse2.GetEntry()
	entries3 := getResponse3.GetEntry()
	entries4 := getResponse4.GetEntry()

	fmt.Print(entries2, entries3, entries4)

	for _, entry := range entries1 {
		v := entry.Entry.(*spb.AFTEntry_Ipv4)
		if prefix := v.Ipv4.GetPrefix(); prefix != "" {
			pref = append(pref, prefix)
		}
	}
	var data []string
	data = append(data, "192.0.2.40/32", "192.0.2.42/32")
	ip := net.ParseIP(dstPfx)
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		ip_v4 := ip.To4()
		data = append(data, ip_v4.String()+"/32")
		ip_v4[3]++
	}

	if diff := cmp.Diff(data, pref); diff != "" {
		t.Errorf("Prefixes differed (-want +got):\n%v", diff)
	}
}

func testBackupSingleNH(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	// Verify the entry for 198.51.100.1/32 is active through Traffic.
	t.Log("going to program Static ARP different from Ixia ")
	config.TextWithGNMI(args.ctx, t, args.dut, "arp 198.51.100.1  0012.0100.0001 arpa")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no arp 198.51.100.1 0012.0100.0001 arpa")

	// LEVEL 2
	// Creating NHG ID 100 using backup NHG ID 101
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 10, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 100, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether121"})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}

	//shutdown primary path port2 and switch to backup port8
	args.interfaceaction(t, "port2", false)
	defer args.interfaceaction(t, "port2", true)
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether127"})
	}

	t.Log("going to remove Static ARP different from Ixia ")
	config.TextWithGNMI(args.ctx, t, args.dut, "no arp 198.51.100.1 0012.0100.0001 arpa")
	config.TextWithGNMI(args.ctx, t, args.dut, "interface Bundle-Ether121 arp learning disable")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no interface Bundle-Ether121 arp learning disable")

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether127"})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		args.client.AftPushConfig(t)
		args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort2.IPv4)
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}

	//adding back port2 configurations
	args.interfaceaction(t, "port2", true)

	t.Logf("deleting all the IPV4 entries added")
	args.client.FlushServer(t)

	args.client.AddNH(t, 3000, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 300, 0, map[uint64]uint64{3000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 200, 0, map[uint64]uint64{2000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "193.0.2.1/32", 200, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 100, "193.0.2.1", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 300, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.DeleteIPv4(t, "193.0.2.1/32", 200, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 200, 0, map[uint64]uint64{2000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 2000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether127"})
	}
}

func testBackupMultiNH(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	// Verify the entry for 198.51.100.1/32 is active through Traffic.

	t.Log("going to program Static ARP different from Ixia ")
	config.TextWithGNMI(args.ctx, t, args.dut, "arp 198.51.100.1  0012.0100.0001 arpa")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no arp 198.51.100.1  0012.0100.0001 arpa")

	// LEVEL 2
	// Creating NHG ID 100 using backup NHG ID 101 (bkhgIndex_2)
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 10, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 100, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 200, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 50, 200: 50}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether121", "Bundle-Ether122"})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	args.interfaceaction(t, "port2", false)
	defer args.interfaceaction(t, "port2", true)
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether122"})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		args.client.AftPushConfig(t)
		args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort2.IPv4)
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}

	args.interfaceaction(t, "port3", false)
	defer args.interfaceaction(t, "port3", true)
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether127"})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort3.IPv4)
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}
}

/*
 * Removing the Backup Path for a prefix when primary
 * links are Up. The traffic shouldnt be impacted
 */
func testIPv4BackUpRemoveBackup(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	// LEVEL 1

	// VIP1: NHG ID 1000 (nhgIndex_1_1)
	//		- PATH1 NH ID 1000 (nhIndex_1_11), weight 50, outgoing Port2
	//		- PATH2 NH ID 1100 (nhIndex_1_12), weight 30, outgoing Port3
	//		- PATH3 NH ID 1200 (nhIndex_1_13), weight 15, outgoing Port4
	//		- PATH4 NH ID 1300 (nhIndex_1_14), weight  5, outgoing Port5
	// VIP2: NHG ID 2000 (nhgIndex_1_2)
	//		- PATH1 NH ID 2000 (nhIndex_1_21), weight 60, outgoing Port6
	//		- PATH2 NH ID 2100 (nhIndex_1_22), weight 35, outgoing Port7
	//		- PATH3 NH ID 2200 (nhIndex_1_23), weight  5, outgoing Port8

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// LEVEL 2
	// Creating a backup NHG with ID 101 (bkhgIndex_2)
	// NH ID 10 (nhbIndex_2_1)

	args.client.AddNH(t, 10, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 200, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	args.client.ReplaceNHG(t, 100, 0, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// validate traffic passing via primary links
	time.Sleep(time.Minute)
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126"})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}
}

/* Add a backup path when primary links are
 * down. Traffic should start taking the backup path
 */
func testIPv4BackUpAddBkNHG(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)
	ConfigureWrr(t, args.dut)
	defer teardownQos(t, args.dut)
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	if *ciscoFlags.GRIBITrafficCheck {
		addPrototoAte(t, top)
	}

	// LEVEL 1
	// VIP1: NHG ID 1000 (nhgIndex_1_1)
	//		- PATH1 NH ID 1000 (nhIndex_1_11), weight 50, outgoing Port2
	//		- PATH2 NH ID 1100 (nhIndex_1_12), weight 30, outgoing Port3
	//		- PATH3 NH ID 1200 (nhIndex_1_13), weight 15, outgoing Port4
	//		- PATH4 NH ID 1300 (nhIndex_1_14), weight  5, outgoing Port5
	// VIP2: NHG ID 2000 (nhgIndex_1_2)
	//		- PATH1 NH ID 2000 (nhIndex_1_21), weight 60, outgoing Port6
	//		- PATH2 NH ID 2100 (nhIndex_1_22), weight 35, outgoing Port7
	//		- PATH3 NH ID 2200 (nhIndex_1_23), weight  5, outgoing Port8

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// LEVEL 2
	// Creating a backup NHG with ID 101 (bkhgIndex_2)
	// NH ID 10 (nhbIndex_2_1)

	args.client.AddNH(t, 10, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 200, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 0, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, false)
		defer args.interfaceaction(t, intf, true)
	}
	// validate traffic passing successfulling after decap via backup link
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks, &gribi.NHGOptions{FRR: true})

	time.Sleep(time.Minute)
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlowsQos(), false, []string{"Bundle-Ether127"})
	}
	args.ValidateQosStats(t, []string{"Bundle-Ether127"})
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		args.client.AftPushConfig(t)
		args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort7.IPv4)
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
}

/*
 * Remove and re-add Backup when Primary is down
 * Traffic should be impacted during remove and
 * should be recovered during Re-Add
 */
func testIPv4BackUpToggleBkNHG(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)
	ConfigureWrr(t, args.dut)
	defer teardownQos(t, args.dut)
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	if *ciscoFlags.GRIBITrafficCheck {
		addPrototoAte(t, top)
	}

	// LEVEL 1
	// VIP1: NHG ID 1000 (nhgIndex_1_1)
	//		- PATH1 NH ID 1000 (nhIndex_1_11), weight 50, outgoing Port2
	//		- PATH2 NH ID 1100 (nhIndex_1_12), weight 30, outgoing Port3
	//		- PATH3 NH ID 1200 (nhIndex_1_13), weight 15, outgoing Port4
	//		- PATH4 NH ID 1300 (nhIndex_1_14), weight  5, outgoing Port5
	// VIP2: NHG ID 2000 (nhgIndex_1_2)
	//		- PATH1 NH ID 2000 (nhIndex_1_21), weight 60, outgoing Port6
	//		- PATH2 NH ID 2100 (nhIndex_1_22), weight 35, outgoing Port7
	//		- PATH3 NH ID 2200 (nhIndex_1_23), weight  5, outgoing Port8

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// LEVEL 2
	// Creating a backup NHG with ID 101 (bkhgIndex_2)
	// NH ID 10 (nhbIndex_2_1)

	args.client.AddNH(t, 10, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 200, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, false)
		defer args.interfaceaction(t, intf, true)
	}
	// validate traffic passing successfulling after decap via ISIS route
	args.client.ReplaceNHG(t, 100, 0, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks, &gribi.NHGOptions{FRR: true})

	time.Sleep(time.Minute)
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlowsQos(), true, []string{"Bundle-Ether127"})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		args.client.AftPushConfig(t)
		args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort7.IPv4)
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

	args.client.ReplaceNHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks, &gribi.NHGOptions{FRR: true})

	time.Sleep(time.Minute)
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlowsQos(), false, []string{"Bundle-Ether127"})
	}
	args.ValidateQosStats(t, []string{"Bundle-Ether127"})
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}
}

func testIPv4BackUpShutSite1(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	// LEVEL 1
	// VIP1: NHG ID 1000 (nhgIndex_1_1)
	//		- PATH1 NH ID 1000 (nhIndex_1_11), weight 50, outgoing Port2
	//		- PATH2 NH ID 1100 (nhIndex_1_12), weight 30, outgoing Port3
	//		- PATH3 NH ID 1200 (nhIndex_1_13), weight 15, outgoing Port4
	//		- PATH4 NH ID 1300 (nhIndex_1_14), weight  5, outgoing Port5
	// VIP2: NHG ID 2000 (nhgIndex_1_2)
	//		- PATH1 NH ID 2000 (nhIndex_1_21), weight 60, outgoing Port6
	//		- PATH2 NH ID 2100 (nhIndex_1_22), weight 35, outgoing Port7
	//		- PATH3 NH ID 2200 (nhIndex_1_23), weight  5, outgoing Port8

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// LEVEL 2
	// Creating a backup NHG with ID 101 (bkhgIndex_2)
	// NH ID 10 (nhbIndex_2_1)

	args.client.AddNH(t, 10, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 200, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, false)
		defer args.interfaceaction(t, intf, true)
	}
	// validate traffic passing successfulling via primary Site 2
	time.Sleep(time.Minute)
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether125", "Bundle-Ether126"})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		args.client.AftPushConfig(t)
		args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort5.IPv4)
		args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort4.IPv4)
		args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort3.IPv4)
		args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort2.IPv4)
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}
}

/* Change the Backup NHG index from a Decap NHG to
 * Drop NHG. The primary links should be shut.
 * Packets should be dropped
 */
func testIPv4BackUpDecapToDrop(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	// LEVEL 1
	// VIP1: NHG ID 1000 (nhgIndex_1_1)
	//		- PATH1 NH ID 1000 (nhIndex_1_11), weight 50, outgoing Port2
	//		- PATH2 NH ID 1100 (nhIndex_1_12), weight 30, outgoing Port3
	//		- PATH3 NH ID 1200 (nhIndex_1_13), weight 15, outgoing Port4
	//		- PATH4 NH ID 1300 (nhIndex_1_14), weight  5, outgoing Port5
	// VIP2: NHG ID 2000 (nhgIndex_1_2)
	//		- PATH1 NH ID 2000 (nhIndex_1_21), weight 60, outgoing Port6
	//		- PATH2 NH ID 2100 (nhIndex_1_22), weight 35, outgoing Port7
	//		- PATH3 NH ID 2200 (nhIndex_1_23), weight  5, outgoing Port8

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// LEVEL 2
	// Creating a backup NHG with ID 101 (bkhgIndex_2)
	// NH ID 10 (nhbIndex_2_1)

	args.client.AddNH(t, 10, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 200, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	//shutdown primary path one by one
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, false)
		defer args.interfaceaction(t, intf, true)
	}
	// validate traffic passing successfulling after decap via backup path
	time.Sleep(time.Minute)
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether127"})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		args.client.AftPushConfig(t)
		args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort7.IPv4)
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

	t.Log("Adding a drop route to Null")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 192.0.2.29/32 Null0")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 192.0.2.29/32 Null0")

	args.client.AddNH(t, 11, "192.0.2.29", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 102, 0, map[uint64]uint64{11: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 102, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks, &gribi.NHGOptions{FRR: true})

	// validate traffic dropping completely on the backup path
	time.Sleep(time.Minute)
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), true, []string{"Bundle-Ether127"})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}
}

/* Change the Backup NHG index from a Drop NHG to
 * Decap NHG. The primary links should be shut.
 * Packets should take the backup path
 */
func testIPv4BackUpDropToDecap(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	t.Log("Adding a drop route to Null")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 192.0.2.29/32 Null0")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 192.0.2.29/32 Null0")

	// LEVEL 1
	// VIP1: NHG ID 1000 (nhgIndex_1_1)
	//		- PATH1 NH ID 1000 (nhIndex_1_11), weight 50, outgoing Port2
	//		- PATH2 NH ID 1100 (nhIndex_1_12), weight 30, outgoing Port3
	//		- PATH3 NH ID 1200 (nhIndex_1_13), weight 15, outgoing Port4
	//		- PATH4 NH ID 1300 (nhIndex_1_14), weight  5, outgoing Port5
	// VIP2: NHG ID 2000 (nhgIndex_1_2)
	//		- PATH1 NH ID 2000 (nhIndex_1_21), weight 60, outgoing Port6
	//		- PATH2 NH ID 2100 (nhIndex_1_22), weight 35, outgoing Port7
	//		- PATH3 NH ID 2200 (nhIndex_1_23), weight  5, outgoing Port8

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// LEVEL 2
	// Creating a backup NHG with ID 101 (bkhgIndex_2)
	// NH ID 10 (nhbIndex_2_1)

	args.client.AddNH(t, 11, "192.0.2.29", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 102, 0, map[uint64]uint64{11: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 200, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 102, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	//shutdown primary path one by one
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, false)
		defer args.interfaceaction(t, intf, true)
	}
	// validate traffic dropping on the backup path
	time.Sleep(time.Minute)
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), true, []string{"Bundle-Ether127"})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		args.client.AftPushConfig(t)
		args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort7.IPv4)
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

	config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 192.0.2.29/32 Null0")

	args.client.AddNH(t, 10, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks, &gribi.NHGOptions{FRR: true})
	// validate traffic passing successfulling after decap via ISIS route
	time.Sleep(time.Minute)
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether127"})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}
}

/* Change the Backup NHG index from a Decap NHG to
 * another Decap NHG. The primary links should be shut.
 * Packets should be forwarded after decapped
 */
func testIPv4BackUpModifyDecapNHG(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)
	ConfigureWrr(t, args.dut)
	defer teardownQos(t, args.dut)
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	if *ciscoFlags.GRIBITrafficCheck {
		addPrototoAte(t, top)
	}

	// LEVEL 1
	// VIP1: NHG ID 1000 (nhgIndex_1_1)
	//		- PATH1 NH ID 1000 (nhIndex_1_11), weight 50, outgoing Port2
	//		- PATH2 NH ID 1100 (nhIndex_1_12), weight 30, outgoing Port3
	//		- PATH3 NH ID 1200 (nhIndex_1_13), weight 15, outgoing Port4
	//		- PATH4 NH ID 1300 (nhIndex_1_14), weight  5, outgoing Port5
	// VIP2: NHG ID 2000 (nhgIndex_1_2)
	//		- PATH1 NH ID 2000 (nhIndex_1_21), weight 60, outgoing Port6
	//		- PATH2 NH ID 2100 (nhIndex_1_22), weight 35, outgoing Port7
	//		- PATH3 NH ID 2200 (nhIndex_1_23), weight  5, outgoing Port8

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// LEVEL 2
	// Creating a backup NHG with ID 101 (bkhgIndex_2)
	// NH ID 10 (nhbIndex_2_1)

	args.client.AddNH(t, 10, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 200, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, false)
		defer args.interfaceaction(t, intf, true)
	}

	args.client.AddNH(t, 11, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 102, 0, map[uint64]uint64{11: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 102, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks, &gribi.NHGOptions{FRR: true})

	// validate traffic passing successfulling after decap via ISIS route
	time.Sleep(time.Minute)
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlowsQos(), false, []string{"Bundle-Ether127"})
	}
	args.ValidateQosStats(t, []string{"Bundle-Ether127"})
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		args.client.AftPushConfig(t)
		args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort7.IPv4)
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
}

func testIPv4BackUpMultipleVRF(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	// LEVEL 1
	// VIP1: NHG ID 1000 (nhgIndex_1_1)
	//		- PATH1 NH ID 1000 (nhIndex_1_11), weight 50, outgoing Port2
	//		- PATH2 NH ID 1100 (nhIndex_1_12), weight 30, outgoing Port3
	//		- PATH3 NH ID 1200 (nhIndex_1_13), weight 15, outgoing Port4
	//		- PATH4 NH ID 1300 (nhIndex_1_14), weight  5, outgoing Port5
	// VIP2: NHG ID 2000 (nhgIndex_1_2)
	//		- PATH1 NH ID 2000 (nhIndex_1_21), weight 60, outgoing Port6
	//		- PATH2 NH ID 2100 (nhIndex_1_22), weight 35, outgoing Port7
	//		- PATH3 NH ID 2200 (nhIndex_1_23), weight  5, outgoing Port8

	args.client.AddNH(t, 1001, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1002, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1003, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1004, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1001: 50, 1002: 30, 1003: 15, 1004: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2001, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2002, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2001: 60, 2002: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// LEVEL 2
	// Creating a backup NHG with ID 101 (bkhgIndex_2)
	// NH ID 10 (nhbIndex_2_1)

	args.client.AddNH(t, 10, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 110, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 111, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 112, 100, map[uint64]uint64{110: 85, 111: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 112, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx1)

	// LEVEL 1
	// VIP1: NHG ID 1000 (nhgIndex_1_1)
	//		- PATH1 NH ID 1000 (nhIndex_1_11), weight 50, outgoing Port2
	//		- PATH2 NH ID 1100 (nhIndex_1_12), weight 30, outgoing Port3
	//		- PATH3 NH ID 1200 (nhIndex_1_13), weight 15, outgoing Port4
	//		- PATH4 NH ID 1300 (nhIndex_1_14), weight  5, outgoing Port5
	// VIP2: NHG ID 2000 (nhgIndex_1_2)
	//		- PATH1 NH ID 2000 (nhIndex_1_21), weight 60, outgoing Port6
	//		- PATH2 NH ID 2100 (nhIndex_1_22), weight 35, outgoing Port7
	//		- PATH3 NH ID 2200 (nhIndex_1_23), weight  5, outgoing Port8

	args.client.AddNH(t, 1001, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1002, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1003, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1004, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1001: 50, 1002: 30, 1003: 15, 1004: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2001, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2002, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2001: 60, 2002: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// LEVEL 2
	// Creating a backup NHG with ID 101 (bkhgIndex_2)
	// NH ID 10 (nhbIndex_2_1)

	args.client.AddNH(t, 20, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 200, 0, map[uint64]uint64{20: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42
	args.client.AddNH(t, 110, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 111, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 212, 200, map[uint64]uint64{110: 85, 111: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 212, "VRF1", *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	ConfigureWrr(t, args.dut)
	defer teardownQos(t, args.dut)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, false)
		defer args.interfaceaction(t, intf, true)
	}
	// validate traffic passing successfulling after decap via ISIS route
	time.Sleep(time.Minute)
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlowsQos(), false, []string{"Bundle-Ether127"})
	}
	args.ValidateQosStats(t, []string{"Bundle-Ether127"})
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		args.client.AftPushConfig(t)
		args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort7.IPv4)
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
}

func testIPv4BackUpFlapBGPISIS(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	// LEVEL 1
	// VIP1: NHG ID 1000 (nhgIndex_1_1)
	//		- PATH1 NH ID 1000 (nhIndex_1_11), weight 50, outgoing Port2
	//		- PATH2 NH ID 1100 (nhIndex_1_12), weight 30, outgoing Port3
	//		- PATH3 NH ID 1200 (nhIndex_1_13), weight 15, outgoing Port4
	//		- PATH4 NH ID 1300 (nhIndex_1_14), weight  5, outgoing Port5
	// VIP2: NHG ID 2000 (nhgIndex_1_2)
	//		- PATH1 NH ID 2000 (nhIndex_1_21), weight 60, outgoing Port6
	//		- PATH2 NH ID 2100 (nhIndex_1_22), weight 35, outgoing Port7
	//		- PATH3 NH ID 2200 (nhIndex_1_23), weight  5, outgoing Port8

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// LEVEL 2
	// Creating a backup NHG with ID 101 (bkhgIndex_2)
	// NH ID 10 (nhbIndex_2_1)

	args.client.AddNH(t, 10, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 200, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	ConfigureWrr(t, args.dut)
	defer teardownQos(t, args.dut)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, false)
		defer args.interfaceaction(t, intf, true)
	}
	// BGP /ISIS peer is in port 8. So flap port 8
	args.interfaceaction(t, "port8", false)
	args.interfaceaction(t, "port8", true)

	// validate traffic passing successfulling via primary Site 2
	time.Sleep(time.Minute)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlowsQos(), false, []string{"Bundle-Ether127"})
	}
	args.ValidateQosStats(t, []string{"Bundle-Ether127"})
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		args.client.AftPushConfig(t)
		args.client.AftRemoveIPv4(t, *ciscoFlags.DefaultNetworkInstance, atePort7.IPv4)
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
}

func testIPv4MultipleNHG(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)
	ConfigureWrr(t, args.dut)
	defer teardownQos(t, args.dut)
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	if *ciscoFlags.GRIBITrafficCheck {
		addPrototoAte(t, top)
	}

	// LEVEL 1
	// VIP1: NHG ID 1000 (nhgIndex_1_1)
	//		- PATH1 NH ID 1000 (nhIndex_1_11), weight 50, outgoing Port2
	//		- PATH2 NH ID 1100 (nhIndex_1_12), weight 30, outgoing Port3
	//		- PATH3 NH ID 1200 (nhIndex_1_13), weight 15, outgoing Port4
	//		- PATH4 NH ID 1300 (nhIndex_1_14), weight  5, outgoing Port5
	// VIP2: NHG ID 2000 (nhgIndex_1_2)
	//		- PATH1 NH ID 2000 (nhIndex_1_21), weight 60, outgoing Port6
	//		- PATH2 NH ID 2100 (nhIndex_1_22), weight 35, outgoing Port7
	//		- PATH3 NH ID 2200 (nhIndex_1_23), weight  5, outgoing Port8

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// LEVEL 2
	// Creating a backup NHG with ID 101 (bkhgIndex_2)
	// NH ID 10 (nhbIndex_2_1)

	args.client.AddNH(t, 10, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42

	args.client.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 200, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

	ip := net.ParseIP(dstPfx)
	ip = ip.To4()

	for i := 501; i < 1001; i++ {
		args.client.AddNHG(t, uint64(i), 101, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		ipv4 := fluent.IPv4Entry().WithPrefix(ip.String() + "/" + mask).WithNetworkInstance(*ciscoFlags.NonDefaultNetworkInstance).WithNextHopGroup(uint64(i)).WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		ip[3]++
		args.client.Fluent(t).Modify().AddEntry(t, ipv4)
	}

	// adding default route pointing to Valid Path
	t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlowsQosScale(), false, []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126", "Bundle-Ether127"})
	}
	args.ValidateQosStats(t, []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126", "Bundle-Ether127"})

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	args.interfaceaction(t, "port7", false)
	args.interfaceaction(t, "port6", false)
	defer args.interfaceaction(t, "port7", true)
	defer args.interfaceaction(t, "port6", true)
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlowsQosScale(), false, []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether127"})
	}

	args.interfaceaction(t, "port5", false)
	args.interfaceaction(t, "port4", false)
	args.interfaceaction(t, "port3", false)
	args.interfaceaction(t, "port2", false)
	defer args.interfaceaction(t, "port5", true)
	defer args.interfaceaction(t, "port4", true)
	defer args.interfaceaction(t, "port3", true)
	defer args.interfaceaction(t, "port2", true)
	// validate traffic decap over backup path
	cliHandle := args.dut.RawAPIs().CLI(t)
	resp, err := cliHandle.SendCommand(context.Background(), "clear qos counters interface all")
	t.Logf(resp, err)
	time.Sleep(3 * time.Minute)
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlowsQos(), false, []string{"Bundle-Ether127"})
	}
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlowsQosScale(), false, []string{"Bundle-Ether127"})
	}
	args.ValidateQosStats(t, []string{"Bundle-Ether127"})
}
func testBackupUseCase3(ctx context.Context, t *testing.T, args *testArgs) {
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)
	t.Logf("Configure Qos Entries")
	ConfigureWrr(t, args.dut)
	defer teardownQos(t, args.dut)
	time.Sleep(10 * time.Second)
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	// VIP1 mapping to NHs
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 60, 1100: 30, 1200: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// VIP2 NHs
	args.client.AddNH(t, 2000, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	//VIP2 mapping to NHs
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 50, 2100: 50}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	// -----------------------------------------
	// DECAP/ENCAP NHs
	args.client.AddNH(t, 2222, "", *ciscoFlags.DefaultNetworkInstance, "DECAP", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2222, 0, map[uint64]uint64{2222: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 3000, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 3000, 2222, map[uint64]uint64{3000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "10.1.0.1/32", 3000, "REPAIRED", *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	// =========================================
	// BACKUP Decap
	// -----------------------------------------
	args.client.AddNH(t, 4000, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 4000, 0, map[uint64]uint64{4000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// =========================================
	// LEVEL 2
	// -----------------------------------------
	// create backup in vrf REPAIR
	args.client.AddNH(t, 1111, "", *ciscoFlags.DefaultNetworkInstance, "REPAIR", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1111, 0, map[uint64]uint64{1111: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	// vrf TE
	args.client.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 200, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 1111, map[uint64]uint64{100: 2, 200: 2}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, vrf1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// -----------------------------------------
	// backup in vrf DECAP
	// args.client.AddNH(t, 2222, "", *ciscoFlags.DefaultNetworkInstance, "DECAP", "", false, ciscoFlags.GRIBIChecks)
	// args.client.AddNHG(t, 2222, 0, map[uint64]uint64{2222: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	// vrf REPAIRED
	args.client.AddNHG(t, 200, 2222, map[uint64]uint64{100: 30, 200: 70}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 200, vrf2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// -----------------------------------------
	// vrf REPAIR
	args.client.AddNH(t, 5000, "DecapEncap", "REPAIRED", "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: "222.222.222.222", Dest: []string{"10.1.0.1"}})
	args.client.AddNHG(t, 300, 0, map[uint64]uint64{5000: 100}, "REPAIRED", false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 300, vrf3, "REPAIRED", false, ciscoFlags.GRIBIChecks)

	// -----------------------------------------
	// vrf DECAP
	args.client.AddIPv4(t, "0.0.0.0/0", 4000, vrf4, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlowsQos(), false, []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126", "Bundle-Ether127"})
	}

	args.ValidateQosStats(t, []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126", "Bundle-Ether127"})
	//time.Sleep(6 * time.Hour)

	args.interfaceaction(t, "port6", false)
	args.interfaceaction(t, "port5", false)
	args.interfaceaction(t, "port4", false)
	args.interfaceaction(t, "port3", false)
	args.interfaceaction(t, "port2", false)
	defer args.interfaceaction(t, "port6", true)
	defer args.interfaceaction(t, "port5", true)
	defer args.interfaceaction(t, "port4", true)
	defer args.interfaceaction(t, "port3", true)
	defer args.interfaceaction(t, "port2", true)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlowsQos(), false, []string{"Bundle-Ether126"})
	}

	args.ValidateQosStats(t, []string{"Bundle-Ether126"})

	args.interfaceaction(t, "port7", false)
	defer args.interfaceaction(t, "port7", true)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlowsQos(), false, []string{"Bundle-Ether127"})
	}

	args.ValidateQosStats(t, []string{"Bundle-Ether127"})

	//time.Sleep(12 * time.Hour)

}

func testDelPbr(ctx context.Context, t *testing.T, args *testArgs) {
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)
	ConfigureWrr(t, args.dut)

}

func ConfigureWrr(t *testing.T, dut *ondatra.DUTDevice) {
	d := &oc.Root{}
	qos := d.GetOrCreateQos()
	queues := []string{"tc7", "tc6", "tc5", "tc4", "tc3", "tc2", "tc1"}
	for _, queue := range queues {
		q1 := qos.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		gnmi.Update(t, dut, gnmi.OC().Qos().Queue(*q1.Name).Config(), q1)
	}
	priorqueues := []string{"tc7", "tc6"}
	schedulerpol := qos.GetOrCreateSchedulerPolicy("eg_policy1111")
	schedule := schedulerpol.GetOrCreateScheduler(1)
	schedule.Priority = oc.Scheduler_Priority_STRICT
	var ind uint64
	ind = 0
	for _, schedqueue := range priorqueues {
		input := schedule.GetOrCreateInput(schedqueue)
		input.Id = ygot.String(schedqueue)
		input.Weight = ygot.Uint64(7 - ind)
		input.Queue = ygot.String(schedqueue)
		ind += 1
	}
	configprior := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(1)
	gnmi.Replace(t, dut, configprior.Config(), schedule)
	configGotprior := gnmi.GetConfig(t, dut, configprior.Config())
	if diff := cmp.Diff(*configGotprior, *schedule); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	nonpriorqueues := []string{"tc5", "tc4", "tc3", "tc2", "tc1"}
	schedulenonprior := schedulerpol.GetOrCreateScheduler(2)
	schedulenonprior.Priority = oc.Scheduler_Priority_UNSET
	var weight uint64
	weight = 0
	for _, wrrqueue := range nonpriorqueues {
		inputwrr := schedulenonprior.GetOrCreateInput(wrrqueue)
		inputwrr.Id = ygot.String(wrrqueue)
		inputwrr.Queue = ygot.String(wrrqueue)
		inputwrr.Weight = ygot.Uint64(60 - weight)
		weight += 10
		configInputwrr := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2).Input(*inputwrr.Id)
		gnmi.Update(t, dut, configInputwrr.Config(), inputwrr)
		configGotwrr := gnmi.GetConfig(t, dut, configInputwrr.Config())
		if diff := cmp.Diff(*configGotwrr, *inputwrr); diff != "" {
			t.Errorf("Config Input fail: \n%v", diff)
		}

	}
	confignonprior := gnmi.OC().Qos().SchedulerPolicy(*schedulerpol.Name).Scheduler(2)
	// confignonprior.Update(t, schedulenonprior)
	configGotnonprior := gnmi.GetConfig(t, dut, confignonprior.Config())
	if diff := cmp.Diff(*configGotnonprior, *schedulenonprior); diff != "" {
		t.Errorf("Config Schedule fail: \n%v", diff)
	}
	interfaceList := []string{}
	for i := 121; i < 128; i++ {
		interfaceList = append(interfaceList, fmt.Sprintf("Bundle-Ether%d", i))
	}
	for _, inter := range interfaceList {

		schedinterface := qos.GetOrCreateInterface(inter)
		schedinterface.InterfaceId = ygot.String(inter)
		schedinterfaceout := schedinterface.GetOrCreateOutput()
		scheinterfaceschedpol := schedinterfaceout.GetOrCreateSchedulerPolicy()
		scheinterfaceschedpol.Name = ygot.String("eg_policy1111")

		ConfigIntf := gnmi.OC().Qos().Interface(*schedinterface.InterfaceId)
		gnmi.Update(t, dut, ConfigIntf.Config(), schedinterface)
		ConfigGotIntf := gnmi.GetConfig(t, dut, ConfigIntf.Config())
		if diff := cmp.Diff(*ConfigGotIntf, *schedinterface); diff != "" {
			t.Errorf("Config Schedule fail: \n%v", diff)
		}
	}

	qosi := d.GetOrCreateQos()
	classifiers := qosi.GetOrCreateClassifier("pmap9")
	classifiers.Name = ygot.String("pmap9")
	classifiers.Type = oc.Qos_Classifier_Type_IPV4
	classmaps := []string{"cmap1", "cmap2", "cmap3", "cmap4", "cmap5", "cmap6", "cmap7"}
	tclass := []string{"tc1", "tc2", "tc3", "tc4", "tc5", "tc6", "tc7"}
	dscps := []int{1, 9, 17, 25, 33, 41, 49}
	for index, classmap := range classmaps {
		terms := classifiers.GetOrCreateTerm(classmap)
		terms.Id = ygot.String(classmap)
		conditions := terms.GetOrCreateConditions()
		ipv4dscp := conditions.GetOrCreateIpv4()
		ipv4dscp.Dscp = ygot.Uint8(uint8(dscps[index]))

		actions := terms.GetOrCreateActions()
		actions.TargetGroup = ygot.String(tclass[index])
		fwdgroups := qosi.GetOrCreateForwardingGroup(tclass[index])
		fwdgroups.Name = ygot.String(tclass[index])
		fwdgroups.OutputQueue = ygot.String(tclass[index])

	}
	gnmi.Update(t, dut, gnmi.OC().Qos().Config(), qosi)
	classinterface := qosi.GetOrCreateInterface("Bundle-Ether120")
	classinterface.InterfaceId = ygot.String("Bundle-Ether120")
	Inputs := classinterface.GetOrCreateInput()
	Inputs.GetOrCreateClassifier(oc.Input_Classifier_Type_IPV4).Name = ygot.String("pmap9")
	Inputs.GetOrCreateClassifier(oc.Input_Classifier_Type_IPV6).Name = ygot.String("pmap9")
	Inputs.GetOrCreateClassifier(oc.Input_Classifier_Type_MPLS).Name = ygot.String("pmap9")
	//TODO: we use updtae due to the bug CSCwc76718, will change it to replace when the bug is fixed
	gnmi.Replace(t, dut, gnmi.OC().Qos().Interface(*classinterface.InterfaceId).Config(), classinterface)
}
func teardownQos(t *testing.T, dut *ondatra.DUTDevice) {

	gnmi.Delete(t, dut, gnmi.OC().Qos().Config())

}

func TestBackUp(t *testing.T) {
	t.Log("Name: BackUp")
	t.Log("Description: Connect gRIBI client and B to DUT using SINGLE_PRIMARY client redundancy with persistance and RibACK")

	dut := ondatra.DUT(t, "dut")

	// Dial gRIBI
	ctx := context.Background()

	// Configure the DUT
	configureDUT(t, dut)
	configbasePBR(t, dut, "TE", "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})
	defer unconfigbasePBR(t, dut)
	// configure route-policy
	configRP(t, dut)
	// configure ISIS on DUT
	addISISOC(t, dut, "Bundle-Ether127")
	// configure BGP on DUT
	addBGPOC(t, dut, "100.100.100.100")

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	if *ciscoFlags.GRIBITrafficCheck {
		addPrototoAte(t, top)
	}

	// Connfigure vty-pool
	config.TextWithGNMI(context.Background(), t, dut, "vty-pool default 0 99 line-template default")
	test := []struct {
		name string
		desc string
		fn   func(ctx context.Context, t *testing.T, args *testArgs)
	}{

		{
			name: "Backup pointing to decap",
			desc: "Base usecase with 2 NHOP Groups - - Backup Pointing to Decap",
			fn:   testBackupToDecapQos,
		},

		{
			name: "Backup change from static to decap",
			desc: "While Primary Paths are down Modify the Backup from poiniting to a static route to a DECAP chain - Traffic resumes after Decap",
			fn:   testBackupSwitchFromDropToDecap,
		},

		{
			name: "IPv4BackUpAddBkNHG",
			desc: "Set primary path with gribi and shutdown all the primary path. Now add the backup NHG and  validate traffic ",
			fn:   testIPv4BackUpAddBkNHG,
		},
		{
			name: "IPv4BackUpToggleBkNHG",
			desc: "Set primary and backup path with gribi and shutdown all the primary path. Now remove,readd the backup NHG and validate traffic ",
			fn:   testIPv4BackUpToggleBkNHG,
		},

		{
			name: "IPv4BackUpModifyDecapNHG",
			desc: "Shutdown all the primary path and modify Backup NHG from  Decap NHG 101 to Decap NHG 102 and validate traffic ",
			fn:   testIPv4BackUpModifyDecapNHG,
		},

		{
			name: "IPv4MultipleNHG",
			desc: "Have same primary and backup decap with multiple nhg",
			fn:   testIPv4MultipleNHG,
		},
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
				InitialElectionIDLow:  10,
				InitialElectionIDHigh: 0,
			}
			defer client.Close(t)
			if err := client.Start(t); err != nil {
				t.Fatalf("gRIBI Connection can not be established")
			}
			args := &testArgs{
				ctx:    ctx,
				client: &client,
				dut:    dut,
				ate:    ate,
				top:    top,
			}
			tt.fn(ctx, t, args)
		})
	}
}
func TestUseCase3(t *testing.T) {
	t.Log("Name: HA")
	t.Log("Description: Connect gRIBI client to DUT using SINGLE_PRIMARY client redundancy with persistance, RibACK and FibACK")

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	var vrfs = []string{vrf1, vrf2, vrf3, vrf4}
	configVRF(t, dut, vrfs)
	configureDUT(t, dut)
	configbasePBR(t, dut, "TE", "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})
	defer unconfigbasePBR(t, dut)
	// configure route-policy
	configRP(t, dut)
	// configure ISIS on DUT
	addISISOC(t, dut, "Bundle-Ether127")
	// configure BGP on DUT
	addBGPOC(t, dut, "100.100.100.100")

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	if *ciscoFlags.GRIBITrafficCheck {
		addPrototoAte(t, top)
	}

	// Connfigure vty-pool
	config.TextWithGNMI(context.Background(), t, dut, "vty-pool default 0 99 line-template default")
	test := []struct {
		name string
		desc string
		fn   func(ctx context.Context, t *testing.T, args *testArgs)
	}{

		{
			name: "Backup pointing to decap",
			desc: "Base usecase with 2 NHOP Groups - - Backup Pointing to Decap",
			fn:   testBackupUseCase3,
		},
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
				InitialElectionIDLow:  10,
				InitialElectionIDHigh: 0,
			}
			defer client.Close(t)
			if err := client.Start(t); err != nil {
				t.Fatalf("gRIBI Connection can not be established")
			}
			args := &testArgs{
				ctx:    ctx,
				client: &client,
				dut:    dut,
				ate:    ate,
				top:    top,
			}
			tt.fn(ctx, t, args)
		})
	}
}

func configVRF(t *testing.T, dut *ondatra.DUTDevice, vrfs []string) {
	for _, vrf_name := range vrfs {
		vrf := &oc.NetworkInstance{
			Name: ygot.String(vrf_name),
			Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
		}
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrf_name).Config(), vrf)
	}
}
