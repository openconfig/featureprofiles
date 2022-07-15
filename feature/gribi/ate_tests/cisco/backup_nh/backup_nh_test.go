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

package backup_nh_test

import (
	"context"
	"fmt"
	"net"
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
	"github.com/openconfig/ondatra/telemetry"
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
	instance              = "DEFAULT"
	dstPfx                = "198.51.100.1"
	mask                  = "32"
	dstPfxMin             = "198.51.100.1"
	dstPfxCount           = 100
	dstPfx1               = "11.1.1.1"
	dstPfxCount1          = 10
	innersrcPfx           = "200.1.0.1"
	innerdstPfxMin_bgp    = "202.1.0.1"
	innerdstPfxCount_bgp  = 100
	innerdstPfxMin_isis   = "201.1.0.1"
	innerdstPfxCount_isis = 100
)

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx    context.Context
	client *gribi.Client
	dut    *ondatra.DUTDevice
	ate    *ondatra.ATEDevice
	top    *ondatra.ATETopology
}

func testBackupToDrop(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	// LEVEL 2
	// Creating a backup NHG with ID 101 and NH ID 10 pointing to a drop address
	args.client.AddNH(t, 10, "192.0.2.100", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
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
	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// adding default route pointing to Valid Path
	t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if intf != "atePort1" {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	args.interfaceaction(t, "port7", false)
	args.interfaceaction(t, "port6", false)
	defer args.interfaceaction(t, "port7", true)
	defer args.interfaceaction(t, "port6", true)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port8"})

	args.interfaceaction(t, "port5", false)
	args.interfaceaction(t, "port4", false)
	args.interfaceaction(t, "port3", false)
	args.interfaceaction(t, "port2", false)
	defer args.interfaceaction(t, "port5", true)
	defer args.interfaceaction(t, "port4", true)
	defer args.interfaceaction(t, "port3", true)
	defer args.interfaceaction(t, "port2", true)

	// validate traffic dropping on backup
	args.validateTrafficFlows(t, flows, true, "port1", []string{"port8"})
}

func testDeleteAddBackupToDrop(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	// LEVEL 2
	// Creating a backup NHG with ID 101 and NH ID 10 pointing to a drop address
	args.client.AddNH(t, 10, "192.0.2.100", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
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
	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// adding default route pointing to Valid Path
	t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if intf != "atePort1" {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})

	//delete backup path and validate no traffic loss
	args.client.ReplaceNHG(t, 100, 0, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 10, "192.0.2.100", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})

	//add back backup path and validate no traffic loss
	args.client.AddNH(t, 10, "192.0.2.100", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})
}

func testBackupToTrafficLoss(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	// LEVEL 2
	// Creating a backup NHG with ID 101 and NH ID 10 pointing to a drop address
	args.client.AddNH(t, 10, "192.0.2.100", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
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
	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// adding default route pointing to Valid Path
	t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if intf != "atePort1" {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})

	//delete backup path and shut primary interfaces and validate traffic drops
	args.client.ReplaceNHG(t, 100, 0, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 10, "192.0.2.100", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
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
	args.validateTrafficFlows(t, flows, true, "port1", []string{"port8"})

	//add back backup path and validate traffic drops
	args.client.AddNH(t, 10, "192.0.2.100", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.validateTrafficFlows(t, flows, true, "port1", []string{"port8"})
}

func testUpdateBackUpToDropID(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	// LEVEL 2
	// Creating a backup NHG with ID 101 and NH ID 10 pointing to a drop address
	args.client.AddNH(t, 10, "192.0.2.100", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
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
	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// adding default route pointing to Valid Path
	t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if intf != "atePort1" {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})

	//Modify Backup pointing to Different ID which is pointing to a different static rooute pointitng to DROP
	args.client.AddNH(t, 999, "220.220.220.220", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNHG(t, 101, 0, map[uint64]uint64{999: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	//shut down primary path and validate traffic dropping
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
	args.validateTrafficFlows(t, flows, true, "port1", []string{"port8"})
}

func testBackupToDecap(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

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
	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// adding default route pointing to Valid Path
	t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if intf != "atePort1" {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	args.interfaceaction(t, "port7", false)
	args.interfaceaction(t, "port6", false)
	defer args.interfaceaction(t, "port7", true)
	defer args.interfaceaction(t, "port6", true)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port8"})

	args.interfaceaction(t, "port5", false)
	args.interfaceaction(t, "port4", false)
	args.interfaceaction(t, "port3", false)
	args.interfaceaction(t, "port2", false)
	defer args.interfaceaction(t, "port5", true)
	defer args.interfaceaction(t, "port4", true)
	defer args.interfaceaction(t, "port3", true)
	defer args.interfaceaction(t, "port2", true)
	// validate traffic decap over backup path
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})
}

func testFlushForwarding(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if intf != "atePort1" {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)

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
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})

	//flush all the entries
	args.client.FlushServer(t)

	//Validate traffic dropping after deleting forwarding
	args.validateTrafficFlows(t, flows, true, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7"})

	//unshut links
	args.interfaceaction(t, "port2", true)
	args.interfaceaction(t, "port3", true)
	args.interfaceaction(t, "port4", true)
	args.interfaceaction(t, "port5", true)
	args.interfaceaction(t, "port6", true)
	args.interfaceaction(t, "port7", true)

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
	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// validate traffic passing over primary path
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7"})

	//shutdown primary path
	args.interfaceaction(t, "port7", false)
	args.interfaceaction(t, "port6", false)
	args.interfaceaction(t, "port5", false)
	args.interfaceaction(t, "port4", false)
	args.interfaceaction(t, "port3", false)
	args.interfaceaction(t, "port2", false)

	//Validate traffic over backup
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})

	//flush the entries
	args.client.FlushServer(t)

	//Validate traffic failing
	args.validateTrafficFlows(t, flows, true, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})
}

func testBackupSwitchFromDropToDecap(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	// LEVEL 2
	// Creating a backup NHG with ID 101 and NH ID 10 pointing to a drop address
	args.client.AddNH(t, 10, "192.0.2.100", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
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
	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// adding default route pointing to Valid Path
	t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if intf != "atePort1" {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})

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
	args.validateTrafficFlows(t, flows, true, "port1", []string{"port8"})

	// Modify backup from pointing to a static route to a DECAP chain
	args.client.AddNH(t, 999, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNHG(t, 101, 0, map[uint64]uint64{999: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	// validate traffic decap over backup path
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})
}

func testUpdateBackupToDifferentNHG(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

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
	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// adding default route pointing to Valid Path
	t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if intf != "atePort1" {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}
	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})
}

func testValidateForwardingChain(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

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
	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

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
	for i := 0; i < dstPfxCount; i++ {
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
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()

	t.Log("going to program Static ARP different from Ixia ")
	config.TextWithGNMI(args.ctx, t, args.dut, "arp 198.51.100.1  0012.0100.0001 arpa")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no arp 198.51.100.1 0012.0100.0001 arpa")

	// LEVEL 2
	// Creating NHG ID 100 using backup NHG ID 101
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 10, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 100, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if intf != "atePort1" {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2"})

	//shutdown primary path port2 and switch to backup port8
	args.interfaceaction(t, "port2", false)
	defer args.interfaceaction(t, "port2", true)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})

	t.Log("going to remove Static ARP different from Ixia ")
	config.TextWithGNMI(args.ctx, t, args.dut, "no arp 198.51.100.1 0012.0100.0001 arpa")
	config.TextWithGNMI(args.ctx, t, args.dut, "interface HundredGigE0/0/0/1 arp learning disable")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no interface HundredGigE0/0/0/1 arp learning disable")

	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})

	//adding back port2 configurations
	args.interfaceaction(t, "port2", true)

	t.Logf("deleting all the IPV4 entries added")
	args.client.FlushServer(t)

	args.client.AddNH(t, 3000, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 300, 0, map[uint64]uint64{3000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 100, "193.0.2.1", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 300, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 200, 0, map[uint64]uint64{2000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "193.0.2.1/32", 200, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.DeleteIPv4(t, "193.0.2.1/32", 200, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 200, 0, map[uint64]uint64{2000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 2000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})
}

func testBackupMultiNH(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	// Verify the entry for 198.51.100.1/32 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()

	t.Log("going to program Static ARP different from Ixia ")
	config.TextWithGNMI(args.ctx, t, args.dut, "arp 198.51.100.1  0012.0100.0001 arpa")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no arp 198.51.100.1  0012.0100.0001 arpa")

	// LEVEL 2
	// Creating NHG ID 100 using backup NHG ID 101 (bkhgIndex_2)
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 10, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 100, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 200, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 50, 200: 50}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if intf != "atePort1" {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}
	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3"})

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	args.interfaceaction(t, "port2", false)
	defer args.interfaceaction(t, "port2", true)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port3"})

	args.interfaceaction(t, "port3", false)
	defer args.interfaceaction(t, "port3", true)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})
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

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if intf != "atePort1" {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}
	args.client.AddNHG(t, 100, 0, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// validate traffic passing via primary links
	time.Sleep(time.Minute)
	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7"})
}

/* Add a backup path when primary links are
 * down. Traffic should start taking the backup path
 */
func testIPv4BackUpAddBkNHG(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

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

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if intf != "atePort1" {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, false)
		defer args.interfaceaction(t, intf, true)
	}
	// validate traffic passing successfulling after decap via backup link
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	time.Sleep(time.Minute)
	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})
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

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if intf != "atePort1" {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, false)
		defer args.interfaceaction(t, intf, true)
	}
	// validate traffic passing successfulling after decap via ISIS route
	args.client.AddNHG(t, 100, 0, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	time.Sleep(time.Minute)
	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, true, "port1", []string{"port8"})

	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	time.Sleep(time.Minute)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})
}

func testIPv4BackUpShutSite1(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

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

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if intf != "atePort1" {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, false)
		defer args.interfaceaction(t, intf, true)
	}
	// validate traffic passing successfulling via primary Site 2
	time.Sleep(time.Minute)
	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port6", "port7"})
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

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if intf != "atePort1" {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	//shutdown primary path one by one
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, false)
		defer args.interfaceaction(t, intf, true)
	}
	// validate traffic passing successfulling after decap via backup path
	time.Sleep(time.Minute)
	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})

	args.client.AddNH(t, 11, "192.0.2.100", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 102, 0, map[uint64]uint64{11: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 102, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// validate traffic dropping completely on the backup path
	time.Sleep(time.Minute)
	args.validateTrafficFlows(t, flows, true, "port1", []string{"port8"})
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

	// LEVEL 2

	// Creating a backup NHG with ID 101 (bkhgIndex_2)
	// NH ID 10 (nhbIndex_2_1)

	args.client.AddNH(t, 11, "192.0.2.100", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
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

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if intf != "atePort1" {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	//shutdown primary path one by one
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, false)
		defer args.interfaceaction(t, intf, true)
	}
	// validate traffic dropping on the backup path
	time.Sleep(time.Minute)
	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, true, "port1", []string{"port8"})

	args.client.AddNH(t, 10, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	// validate traffic passing successfulling after decap via ISIS route
	time.Sleep(time.Minute)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})
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

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if intf != "atePort1" {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, false)
		defer args.interfaceaction(t, intf, true)
	}

	args.client.AddNH(t, 11, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 102, 0, map[uint64]uint64{11: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 102, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// validate traffic passing successfulling after decap via ISIS route
	time.Sleep(time.Minute)
	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})
}

func testIPv4BackUpMultiplePrefixes(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

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

	args.client.AddNH(t, 1001, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1002, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1003, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1004, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1001: 50, 1002: 30, 1003: 15, 1004: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2001, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2002, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2001: 60, 2002: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx1)

	// LEVEL 2

	// Creating a backup NHG with ID 101 (bkhgIndex_2)
	// NH ID 10 (nhbIndex_2_1)

	args.client.AddNH(t, 20, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 200, 0, map[uint64]uint64{20: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42
	args.client.AddNH(t, 210, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 211, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 212, 200, map[uint64]uint64{210: 85, 211: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 212, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

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

	args.client.AddNH(t, 3001, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 3002, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 3003, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 3004, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 3000, 0, map[uint64]uint64{3001: 50, 3002: 30, 3003: 15, 3004: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 3000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 4001, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 4002, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 4000, 0, map[uint64]uint64{4001: 60, 4002: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 4000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if intf != "atePort1" {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, false)
		defer args.interfaceaction(t, intf, true)
	}
	// validate traffic passing successfulling after decap via ISIS route
	time.Sleep(time.Minute)
	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})
}

func testIPv4BackUpMultipleVRF(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

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

	args.client.AddNH(t, 1001, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1002, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1003, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1004, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1001: 50, 1002: 30, 1003: 15, 1004: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2001, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2002, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2001: 60, 2002: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx1)

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

	args.client.AddNH(t, 1001, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1002, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1003, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1004, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1001: 50, 1002: 30, 1003: 15, 1004: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2001, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2002, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2001: 60, 2002: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if intf != "atePort1" {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, false)
		defer args.interfaceaction(t, intf, true)
	}
	// validate traffic passing successfulling after decap via ISIS route
	time.Sleep(time.Minute)
	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})
}

func testIPv4BackUpFlapBGPISIS(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

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

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if intf != "atePort1" {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

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

	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})
}

func testIPv4MultipleNHG(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

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

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// adding default route pointing to Valid Path
	t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if intf != "atePort1" {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	args.interfaceaction(t, "port7", false)
	args.interfaceaction(t, "port6", false)
	defer args.interfaceaction(t, "port7", true)
	defer args.interfaceaction(t, "port6", true)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port8"})

	args.interfaceaction(t, "port5", false)
	args.interfaceaction(t, "port4", false)
	args.interfaceaction(t, "port3", false)
	args.interfaceaction(t, "port2", false)
	defer args.interfaceaction(t, "port5", true)
	defer args.interfaceaction(t, "port4", true)
	defer args.interfaceaction(t, "port3", true)
	defer args.interfaceaction(t, "port2", true)
	// validate traffic decap over backup path
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})
}

func testIPv4BackUpLCOIR(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

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

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if intf != "atePort1" {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, false)
		defer args.interfaceaction(t, intf, true)
	}
	// BGP /ISIS peer is in port 8. So flap port 8
	config.CMDViaGNMI(args.ctx, t, args.dut, "reload location 0/0/CPU0 noprompt \n")
	// validate traffic passing successfulling via primary Site 2
	time.Sleep(time.Minute)

	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})
}

func TestBackUp(t *testing.T) {
	t.Log("Name: BackUp")
	t.Log("Description: Connect gRIBI client and B to DUT using SINGLE_PRIMARY client redundancy with persistance and RibACK")

	dut := ondatra.DUT(t, "dut")

	// Dial gRIBI
	ctx := context.Background()

	// Configure the DUT
	configureDUT(t, dut)
	configbasePBR(t, dut, "TE", "ipv4", 1, telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	addPrototoAte(t, top)

	test := []struct {
		name string
		desc string
		fn   func(ctx context.Context, t *testing.T, args *testArgs)
	}{
		{
			name: "Backup pointing to route",
			desc: "Base usecase with 2 NHOP Groups - Backup Pointing to route (drop)",
			fn:   testBackupToDrop,
		},
		{
			name: "Delete add backup",
			desc: "Deleting and Adding Back the Backup has no impact on traffic",
			fn:   testDeleteAddBackupToDrop,
		},
		{
			name: "Add backup after primary down",
			desc: "Add the backup - After Primary links are down Traffic continues ot DROP",
			fn:   testBackupToTrafficLoss,
		},
		{
			name: "Update backup to another ID",
			desc: "Modify Backup pointing to Different ID which is pointing to a different static rooute pointitng to DROP",
			fn:   testUpdateBackUpToDropID,
		},
		{
			name: "Backup pointing to decap",
			desc: "Base usecase with 2 NHOP Groups - - Backup Pointing to Decap",
			fn:   testBackupToDecap,
		},
		{
			name: "flush forwarding chain with and without backup NH",
			desc: "add testcase to flush forwarding chain with backup NHG only and forwarding chain with backup NHG",
			fn:   testFlushForwarding,
		},
		{
			name: "Backup change from static to decap",
			desc: "While Primary Paths are down Modify the Backup from poiniting to a static route to a DECAP chain - Traffic resumes after Decap",
			fn:   testBackupSwitchFromDropToDecap,
		},
		{
			name: "Multiple NW Instance with different NHG, same NH and different NHG backup",
			desc: "Multiple NW Instances (VRF's ) pointing to different NHG but same NH Entry but different NHG Backup",
			fn:   testUpdateBackupToDifferentNHG,
		},
		{
			name: "Get function validation",
			desc: "add decap NH and related forwarding chain and validate them using GET function",
			fn:   testValidateForwardingChain,
		},
		{
			name: "IPv4BackUpSingleNH",
			desc: "Single NH Ensure that backup NextHopGroup entries are honoured in gRIBI for NHGs containing a single NH",
			fn:   testBackupSingleNH,
		},
		{
			name: "IPv4BackUpMultiNH",
			desc: "Multiple NHBackup NHG: Multiple NH Ensure that backup NHGs are honoured with NextHopGroup entries containing",
			fn:   testBackupMultiNH,
		},
		{
			name: "IPv4BackUpRemoveBackup",
			desc: "Set primary and backup path with gribi and send traffic. Delete the backup NHG and check if impacts traffic",
			fn:   testIPv4BackUpRemoveBackup,
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
			name: "IPv4BackUpDecapToDrop",
			desc: "Shutdown all the primary path and modify Backup NHG from Drop to Decap and validate traffic ",
			fn:   testIPv4BackUpDecapToDrop,
		},
		{
			name: "IPv4BackUpDropToDecap",
			desc: "Shutdown all the primary path and modify Backup NHG from Decap to Drop and validate traffic ",
			fn:   testIPv4BackUpDropToDecap,
		},
		{
			name: "IPv4BackUpShutSite1",
			desc: "Shutdown the primary path for 1 Site  and validate traffic is going through another primary and not backup ",
			fn:   testIPv4BackUpShutSite1,
		},
		{
			name: "IPv4BackUpModifyDecapNHG",
			desc: "Shutdown all the primary path and modify Backup NHG from  Decap NHG 101 to Decap NHG 102 and validate traffic ",
			fn:   testIPv4BackUpModifyDecapNHG,
		},
		{
			name: "IPv4BackUpMultiplePrefixes",
			desc: "Have same primary and backup links for 2 prefixes with different NHG IDs and validate backup traffic ",
			fn:   testIPv4BackUpMultiplePrefixes,
		},
		{
			name: "IPv4BackUpMultipleVRF",
			desc: "Have same primary and backup links for 2 prefixes with different NHG IDs in different VRFs and validate backup traffic ",
			fn:   testIPv4BackUpMultipleVRF,
		},
		{
			name: "IPv4BackUpFlapBGPISIS",
			desc: "Have same primary and backup links for 2 prefixes with different NHG IDs in different VRFs and validate backup traffic ",
			fn:   testIPv4BackUpFlapBGPISIS,
		},
		{
			name: "IPv4BackupLCOIR",
			desc: "Have Primary and backup configured on same LC and do a shut of primary. Followed by LC reload",
			fn:   testIPv4BackUpLCOIR,
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
