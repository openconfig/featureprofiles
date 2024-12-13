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

package transitwecmpflush_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/gribi"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gribigo/server"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

type testArgs struct {
	ctx      context.Context
	c1       *gribi.Client
	c2       *gribi.Client
	dut      *ondatra.DUTDevice
	ate      *ondatra.ATEDevice
	topology *ondatra.ATETopology
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Transit-83 DELETE FlowSPEC and ADD PBR config
// func testChangeFlowSpecToPBR(t *testing.T, args *testArgs) {
// 	t.Log("Remove flow spec config and apply pbr config")
// 	configToChange := "no flowspec \nhw-module profile pbr vrf-redirect\n"
// 	config.Reload(args.ctx, t, args.dut, configToChange, "", 15*time.Minute)
//  configbasePBR(t, args.dut)
//  args.dut.Config().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Interface("Bundle-Ether120").ApplyVrfSelectionPolicy().Update(t, pbrName)
// }

func testCD2ConnectedNHIP(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t) // let test to clear the entries at the begining for itself.
	//Using defer is problamtic since it may cause failuer when the gribi connection drops
	weights := map[uint64]uint64{3: 15}
	args.c1.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "11.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, server.DefaultNetworkInstanceName, false, ciscoFlags.GRIBIChecks)
	if *ciscoFlags.GRIBITrafficCheck {
		checkTraffic(t, "IPinIPUnConnected", args.ate, false)
	}
}

// testCD2RecursiveNonConnectedNHOP ?
func testCD2RecursiveNonConnectedNHOP(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)
	// 192.0.2.42/32  Next-Site
	weights := map[uint64]uint64{41: 40}

	// adding static route since nh is not connected
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 100.129.1.2/32 null 0")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 100.129.1.2/32 null 0")
	args.c1.AddNH(t, 41, "100.129.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks) // Not connected
	args.c1.AddNHG(t, 100, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32 Self-Site
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.11", i, "32"))
	}
	weights = map[uint64]uint64{20: 99}
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	if *ciscoFlags.GRIBITrafficCheck {
		checkTraffic(t, "IPinIPUnConnected", args.ate, true)
	}
}

// Transit-46 ADD same IPv4 Entry verify no traffic impact
func testAddIPv4EntryTrafficCheck(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	weights := map[uint64]uint64{3: 15}
	args.c1.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "11.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Start Traffic
	portMaps := args.topology.Interfaces()
	args.topology.StartProtocols(t)
	defer args.topology.StopProtocols(t)
	baseflow := getBaseFlow(t, portMaps, args.ate, "IPinIP")
	args.ate.Traffic().Start(t, baseflow)
	defer args.ate.Traffic().Stop(t)

	// Add same ipv4 entry
	args.c2.BecomeLeader(t)
	args.c2.AddIPv4(t, "11.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// traffic verification
	time.Sleep(60 * time.Second)

	stats := gnmi.GetAll(t, args.ate, gnmi.OC().FlowAny().State())
	lossStream := util.CheckTrafficPassViaRate(stats)
	if len(lossStream) > 0 {
		t.Fatal("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}
}

// Transit-47 REPLACE same IPv4 Entry verify no traffic impact
func testReplaceIPv4EntryTrafficCheck(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	weights := map[uint64]uint64{3: 15}
	args.c1.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "11.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Start Traffic
	portMaps := args.topology.Interfaces()
	args.topology.StartProtocols(t)
	defer args.topology.StopProtocols(t)
	baseflow := getBaseFlow(t, portMaps, args.ate, "IPinIP")
	args.ate.Traffic().Start(t, baseflow)
	defer args.ate.Traffic().Stop(t)

	// Replace same ipv4 entry
	args.c2.BecomeLeader(t)
	args.c2.ReplaceIPv4(t, "11.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// traffic verification
	time.Sleep(60 * time.Second)

	stats := gnmi.GetAll(t, args.ate, gnmi.OC().FlowAny().State())
	lossStream := util.CheckTrafficPassViaRate(stats)
	if len(lossStream) > 0 {
		t.Fatal("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}

}

// Transit-48 ADD same NHG verify no traffic impact
func testAddNHGTrafficCheck(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	weights := map[uint64]uint64{3: 15}
	args.c1.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "11.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Start Traffic
	portMaps := args.topology.Interfaces()
	args.topology.StartProtocols(t)
	defer args.topology.StopProtocols(t)
	baseflow := getBaseFlow(t, portMaps, args.ate, "IPinIP")
	args.ate.Traffic().Start(t, baseflow)
	defer args.ate.Traffic().Stop(t)

	// Add same NHG entry
	args.c2.BecomeLeader(t)
	args.c2.AddNHG(t, 11, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// traffic verification
	time.Sleep(60 * time.Second)
	stats := gnmi.GetAll(t, args.ate, gnmi.OC().FlowAny().State())
	lossStream := util.CheckTrafficPassViaRate(stats)
	if len(lossStream) > 0 {
		t.Fatal("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}

}

// Transit-49 REPLACE same NHG verify no traffic impact
func testReplaceNHGTrafficCheck(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	weights := map[uint64]uint64{3: 15}
	args.c1.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "11.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	// Start Traffic
	portMaps := args.topology.Interfaces()
	args.topology.StartProtocols(t)
	defer args.topology.StopProtocols(t)
	baseflow := getBaseFlow(t, portMaps, args.ate, "IPinIP")
	args.ate.Traffic().Start(t, baseflow)
	defer args.ate.Traffic().Stop(t)
	// Replace same NHG entry
	args.c2.BecomeLeader(t)
	args.c2.ReplaceNHG(t, 11, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	// traffic verification
	time.Sleep(60 * time.Second)
	stats := gnmi.GetAll(t, args.ate, gnmi.OC().FlowAny().State())
	lossStream := util.CheckTrafficPassViaRate(stats)
	if len(lossStream) > 0 {
		t.Fatal("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}

}

// Transit-50 ADD same NH verify no traffic impact
func testAddNHTrafficCheck(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	weights := map[uint64]uint64{3: 15}
	args.c1.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "11.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Start Traffic
	portMaps := args.topology.Interfaces()
	args.topology.StartProtocols(t)
	defer args.topology.StopProtocols(t)
	baseflow := getBaseFlow(t, portMaps, args.ate, "IPinIP")
	args.ate.Traffic().Start(t, baseflow)
	defer args.ate.Traffic().Stop(t)

	// Add same NH entry
	args.c2.BecomeLeader(t)
	args.c2.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

	// traffic verification
	time.Sleep(60 * time.Second)
	stats := gnmi.GetAll(t, args.ate, gnmi.OC().FlowAny().State())
	lossStream := util.CheckTrafficPassViaRate(stats)
	if len(lossStream) > 0 {
		t.Fatal("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}

}

// Transit-51 REPLACE same NH verify no traffic impact
func testReplaceNHTrafficCheck(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	weights := map[uint64]uint64{3: 15}
	args.c1.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "11.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	// Start Traffic
	portMaps := args.topology.Interfaces()
	args.topology.StartProtocols(t)
	defer args.topology.StopProtocols(t)
	baseflow := getBaseFlow(t, portMaps, args.ate, "IPinIP")
	args.ate.Traffic().Start(t, baseflow)
	defer args.ate.Traffic().Stop(t)

	// Replace same NH entry
	args.c2.BecomeLeader(t)
	args.c2.ReplaceNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

	// traffic verification
	time.Sleep(60 * time.Second)
	stats := gnmi.GetAll(t, args.ate, gnmi.OC().FlowAny().State())
	lossStream := util.CheckTrafficPassViaRate(stats)
	if len(lossStream) > 0 {
		t.Fatal("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}

}

func testCD2SingleRecursion(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	weights := map[uint64]uint64{3: 15}
	args.c1.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "11.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		checkTraffic(t, "IPinIP", args.ate, false)
	}
}

func testCD2DoubleRecursion(t *testing.T, args *testArgs) {
	// this test requires a setup for ips and bundle ethernet //TODO: the setup should be implemented via oc calls
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	// 192.0.2.40/32  Self-Site
	weights := map[uint64]uint64{
		31: 10,
		32: 20,
		33: 30,
	}
	// to do add nh with interfaceref
	args.c1.AddNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 40, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Next-Site
	weights = map[uint64]uint64{
		41: 40,
		42: 30,
		43: 20,
		44: 10,
	}
	args.c1.AddNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.0", i, "32"))
	}
	weights = map[uint64]uint64{
		10: 85,
		20: 15,
	}
	args.c1.AddNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	performATEAction(t, "ate", int(*ciscoFlags.GRIBIScale), true)
}

// Transit-34 REPLACE: default VRF IPv4 Entry with single path NHG+NH in default vrf
func testReplaceDefaultIPv4EntrySinglePath(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	weights := map[uint64]uint64{3: 15}
	args.c1.AddNH(t, 3, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "11.11.11.11/32", 11, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Add New NHG
	args.c2.BecomeLeader(t)
	weights = map[uint64]uint64{4: 15}
	args.c2.AddNH(t, 4, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c2.AddNHG(t, 12, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Replace VRF IPv4 Entry Pointing to different NHG
	// Todo: why we are using the third client
	c3 := gribi.Client{
		DUT:                  args.dut,
		FibACK:               *ciscoFlags.GRIBIFIBCheck,
		Persistence:          true,
		InitialElectionIDLow: 10,
	}
	defer c3.Close(t)
	if err := c3.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}
	c3.BecomeLeader(t)
	c3.AddIPv4(t, "11.11.11.11/32", 12, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		checkTraffic(t, "IPinIP", args.ate, false, "default") // to do: make the traffic check to work with array as protocls
	}
}

// Transit-38 DELETE: VRF IPv4 Entry with single path NHG+NH in default vrf
func testDeleteVRFIPv4EntrySinglePath(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	weights := map[uint64]uint64{3: 15}
	args.c1.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "11.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		checkTraffic(t, "IPinIP", args.ate, false)
	}

	// Delete Entry
	args.c2.BecomeLeader(t)
	defer args.c2.FlushServer(t)
	args.c2.DeleteIPv4(t, "11.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c2.DeleteNHG(t, 11, 0, map[uint64]uint64{}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c2.DeleteNH(t, 3, "", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
}

// Transit-42 DELETE: default VRF IPv4 Entry with single path NHG+NH in default vrf
func testDeleteDefaultIPv4EntrySinglePath(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	weights := map[uint64]uint64{6: 15}
	args.c1.AddNH(t, 6, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 16, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "11.11.11.11/32", 16, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		checkTraffic(t, "IPinIP", args.ate, false, "default")
	}

	// Delete Entry
	args.c2.BecomeLeader(t)
	defer args.c2.FlushServer(t)
	args.c2.DeleteIPv4(t, "11.11.11.11/32", 16, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c2.DeleteNHG(t, 16, 0, map[uint64]uint64{}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c2.DeleteNH(t, 6, "", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
}

// Transit TC 066 - Two prefixes with NHGs with backup pointing to the each other's NHG
func testTwoPrefixesWithSameSetOfPrimaryAndBackup(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	weights1 := map[uint64]uint64{3: 15}
	args.c1.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "12.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	weights2 := map[uint64]uint64{4: 15}
	args.c1.AddNH(t, 4, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 14, 11, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "12.11.11.12/32", 14, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// not suported as confirmed by DE, in CD2 we had addnhg call, which was over writing the existing one which failed
	// args.c1.ReplaceNHG(t, 11, 14, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	portMaps := args.topology.Interfaces()

	args.topology.StartProtocols(t)
	defer args.topology.StopProtocols(t)

	dscp16Flow := getDSCPFlow(t, portMaps, args.ate, "DSCP16", 1, 16, "12.11.11.12", sortedAtePorts[2])
	dscp10Flow := getDSCPFlow(t, portMaps, args.ate, "DSCP10", 1, 10, "12.11.11.11", sortedAtePorts[1])

	checkTrafficFlows(t, args.ate, 60, dscp16Flow, dscp10Flow)
}

// Transit TC 067 - Same forwarding entries across multiple vrfs
func testSameForwardingEntriesAcrossMultipleVrfs(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	weights1 := map[uint64]uint64{3: 15}
	weights2 := map[uint64]uint64{4: 15}
	args.c1.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 4, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 14, 11, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "12.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "12.11.11.12/32", 14, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.c1.AddIPv4(t, "12.11.11.11/32", 11, "VRF1", *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "12.11.11.12/32", 14, "VRF1", *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	portMaps := args.topology.Interfaces()

	args.topology.StartProtocols(t)
	defer args.topology.StopProtocols(t)

	dscp16FlowVrfTE := getDSCPFlow(t, portMaps, args.ate, "DSCP16_vrf_TE", 1, 16, "12.11.11.12", sortedAtePorts[2])
	dscp10FlowVrfTE := getDSCPFlow(t, portMaps, args.ate, "DSCP10_vrf_TE", 1, 10, "12.11.11.11", sortedAtePorts[1])
	dscp18Flow1VrfVRF1 := getDSCPFlow(t, portMaps, args.ate, "DSCP16_vrf_VRF1", 1, 18, "12.11.11.12", sortedAtePorts[2])
	dscp18Flow2VrfVRF1 := getDSCPFlow(t, portMaps, args.ate, "DSCP10_vrf_VRF1", 1, 18, "12.11.11.11", sortedAtePorts[1])

	checkTrafficFlows(t, args.ate, 60, dscp16FlowVrfTE, dscp10FlowVrfTE, dscp18Flow1VrfVRF1, dscp18Flow2VrfVRF1)
}

// Transit-11: Next Hop resoultion with interface in different VRF of NH_network_instance
func testNHInterfaceInDifferentVRF(t *testing.T, args *testArgs) {
	t.Log("Testcase: Next Hop resoultion with interface in different VRF of NH_network_instance")
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	// 192.0.2.40/32  Self-Site
	weights1 := map[uint64]uint64{
		31: 10,
		32: 20,
		33: 30,
	}
	ciscoFlags.GRIBIChecks.AFTCheck = false
	args.c1.AddNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.NonDefaultNetworkInstance, "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	ciscoFlags.GRIBIChecks.AFTCheck = true

	// 192.0.2.42/32  Next-Site
	weights2 := map[uint64]uint64{
		41: 10,
		42: 20,
		43: 30,
		44: 40,
	}
	args.c1.AddNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.0", i, "32"))
	}
	weights3 := map[uint64]uint64{
		10: 85,
		20: 15,
	}
	args.c1.AddNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Correct the related NH and verify traffic
	args.c1.ReplaceNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)

	performATEAction(t, "ate", int(*ciscoFlags.GRIBIScale), true)
}

// Transit-13: Next Hop resolution with interface+IP out of that interface subnet
func testNHIPOutOfInterfaceSubnet(t *testing.T, args *testArgs) {
	t.Log("Testcase: Next Hop resolution with interface+IP out of that interface subnet")
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	// 192.0.2.40/32  Self-Site
	weights1 := map[uint64]uint64{
		31: 30,
		32: 30,
		33: 30,
	}
	// adding static route since nh is not connected
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 100.121.2.2/32 Bundle-Ether121")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 100.121.2.2/32 Bundle-Ether121")

	args.c1.AddNH(t, 31, "100.121.2.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Next-Site
	weights2 := map[uint64]uint64{
		41: 10,
		42: 20,
		43: 30,
		44: 40,
	}
	args.c1.AddNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.0", i, "32"))
	}
	weights3 := map[uint64]uint64{
		10: 85,
		20: 15,
	}
	args.c1.AddNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Correct the related NH and verify traffic
	args.c1.AddNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)

	performATEAction(t, "ate", int(*ciscoFlags.GRIBIScale), true)
}

// Transit-16:Changing IP address on I/F making NHOP unreachable and changing it back
func testChangeNHToUnreachableAndChangeBack(t *testing.T, args *testArgs) {
	t.Log("Testcase: Changing IP address on I/F making NHOP unreachable and changing it back")
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	// 192.0.2.40/32  Self-Site
	weights1 := map[uint64]uint64{
		31: 15,
		32: 25,
		33: 35,
	}
	args.c1.AddNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Next-Site
	weights2 := map[uint64]uint64{
		41: 10,
		42: 20,
		43: 30,
		44: 40,
	}
	args.c1.AddNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.0", i, "32"))
	}
	weights3 := map[uint64]uint64{
		10: 85,
		20: 15,
	}
	args.c1.AddNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Set InCorrect related NH
	// adding static route since nh is not connected
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 1.2.3.4/32 null 0")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 1.2.3.4/32 null 0")

	args.c1.AddNH(t, 31, "1.2.3.4", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

	// Correct the related NH and verify traffic
	args.c1.AddNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)

	performATEAction(t, "ate", int(*ciscoFlags.GRIBIScale), true)

}

// Transit-19: Next Hop Group resolution change NH from recursive and non-recursive
func testChangeNHFromRecursiveToNonRecursive(t *testing.T, args *testArgs) {
	t.Log("Testcase: Next Hop Group resolution change NH from recursive and non-recursive")
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	weights1 := map[uint64]uint64{
		3: 15,
	}
	args.c1.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// 192.0.2.40/32  Self-Site
	weights2 := map[uint64]uint64{
		31: 10,
		32: 20,
		33: 30,
	}
	args.c1.AddNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 40, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Next-Site
	weights3 := map[uint64]uint64{
		41: 15,
		42: 25,
		43: 35,
		44: 45,
	}
	args.c1.AddNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.0", i, "32"))
	}
	weights4 := map[uint64]uint64{
		10: 85,
		20: 15,
	}
	args.c1.AddNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights4, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Correct the related NH and verify traffic
	args.c1.AddIPv4Batch(t, prefixes, 11, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	performATEAction(t, "ate", int(*ciscoFlags.GRIBIScale), true)

}

// Transit TC 073 - ADD/REPLACE/DELETE during related interface flap
func testAddReplaceDeleteWithRelatedInterfaceFLap(t *testing.T, args *testArgs) {
	t.Log("Testcase: Add, Replace, Delete operations with related interface flap")
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	// 192.0.2.40/32  Self-Site
	weights1 := map[uint64]uint64{
		31: 10,
		32: 20,
		33: 30,
	}
	args.c1.AddNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Next-Site
	weights2 := map[uint64]uint64{
		41: 15,
		42: 25,
		43: 35,
		44: 45,
	}
	args.c1.AddNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.0", i, "32"))
	}
	weights3 := map[uint64]uint64{
		10: 85,
		20: 15,
	}
	args.c1.AddNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	weights1 = map[uint64]uint64{
		31: 10,
		32: 20,
		33: 30,
	}
	weights2 = map[uint64]uint64{
		41: 15,
		42: 25,
		43: 35,
		44: 45,
	}
	weights3 = map[uint64]uint64{
		10: 85,
		20: 15,
	}
	//Replace all entries
	// 192.0.2.40/32  Self-Site
	args.c1.ReplaceNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	// ciscoFlags.GRIBIChecks.AFTCheck = true
	args.c1.ReplaceNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	// ciscoFlags.GRIBIChecks.AFTCheck = false

	// 192.0.2.42/32  Next-Site
	args.c1.ReplaceNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	args.c1.ReplaceNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	//Delete all entries
	// 192.0.2.40/32  Self-Site
	args.c1.DeleteIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Next-Site
	args.c1.DeleteIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	args.c1.DeleteIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

	//Flap interfaces
	interfaceNames := []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126", "Bundle-Ether127"}
	for _, interfaceName := range interfaceNames {
		util.SetInterfaceState(t, args.dut, interfaceName, false, oc.IETFInterfaces_InterfaceType_ieee8023adLag)
	}
	time.Sleep(30 * time.Second)
	for _, interfaceName := range interfaceNames {
		util.SetInterfaceState(t, args.dut, interfaceName, true, oc.IETFInterfaces_InterfaceType_ieee8023adLag)
	}

	// 192.0.2.40/32  Self-Site
	time.Sleep(30 * time.Second)
	weights1 = map[uint64]uint64{
		31: 10,
		32: 20,
		33: 30,
	}
	weights2 = map[uint64]uint64{
		41: 15,
		42: 25,
		43: 35,
		44: 45,
	}
	weights3 = map[uint64]uint64{
		10: 85,
		20: 15,
	}
	args.c1.AddNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Next-Site
	args.c1.AddNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	args.c1.AddNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Configure ATE and Verify traffic
	performATEAction(t, "ate", int(*ciscoFlags.GRIBIScale), true)
}

// Transit-40  DELETE: VRF IPv4 Entry with ECMP path NHG+NH in default vrf
func testDeleteVRFIPv4EntryECMPPath(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	// 192.0.2.40/32  Self-Site
	weights1 := map[uint64]uint64{
		31: 10,
		32: 20,
		33: 30,
	}
	args.c1.AddNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Next-Site
	weights2 := map[uint64]uint64{
		41: 15,
		42: 25,
		43: 35,
		44: 45,
	}
	args.c1.AddNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.0", i, "32"))
	}
	weights3 := map[uint64]uint64{
		10: 85,
		20: 15,
	}
	args.c1.AddNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	performATEAction(t, "ate", int(*ciscoFlags.GRIBIScale), true)

	// Delete
	// 192.0.2.40/32  Self-Site
	args.c1.DeleteIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Next-Site
	weights2 = map[uint64]uint64{
		41: 40,
		42: 30,
		43: 20,
		44: 10,
	}
	args.c1.DeleteIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	args.c1.DeleteIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

	// Expect traffic to fail
	performATEAction(t, "ate", int(*ciscoFlags.GRIBIScale), false)
}

// Transit-45  DELETE: default VRF IPv4 Entry with ECMP+backup path NHG+NH in default vrf
func testDeleteDefaultIPv4EntryECMPPath(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	weights1 := map[uint64]uint64{
		3: 15,
	}
	weights2 := map[uint64]uint64{
		4: 15,
	}
	args.c1.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 4, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 14, 11, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "12.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "11.11.11.0/32", 14, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	performATEAction(t, "ate", 1, true)

	// Delete
	args.c1.DeleteIPv4(t, "12.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteIPv4(t, "11.11.11.0/32", 14, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNHG(t, 14, 11, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNHG(t, 11, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 4, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

	performATEAction(t, "ate", 1, false)
}

// Transit-32 REPLACE: VRF IPv4 Entry with ECMP path NHG+NH in default vrf
func testReplaceVRFIPv4EntryECMPPath(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	// 192.0.2.40/32  Self-Site
	weights1 := map[uint64]uint64{
		31: 10,
		32: 20,
		33: 30,
	}
	args.c1.AddNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Next-Site
	weights2 := map[uint64]uint64{
		41: 40,
		42: 30,
		43: 20,
		44: 40,
	}
	args.c1.AddNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.0", i, "32"))
	}
	weights3 := map[uint64]uint64{
		10: 85,
		20: 15,
	}
	args.c1.AddNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Traffic start
	portMaps := args.topology.Interfaces()
	args.topology.StartProtocols(t)
	defer args.topology.StopProtocols(t)
	scaleflow := getScaleFlow(t, portMaps, args.ate, "IPinIPWithScale", int(*ciscoFlags.GRIBIScale))
	args.ate.Traffic().Start(t, scaleflow)
	defer args.ate.Traffic().Stop(t)
	// Replace same ipv4 entry
	args.c1.ReplaceIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// traffic verification
	time.Sleep(60 * time.Second)
	stats := gnmi.GetAll(t, args.ate, gnmi.OC().FlowAny().State())
	lossStream := util.CheckTrafficPassViaRate(stats)
	if len(lossStream) > 0 {
		t.Fatal("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}
}

// Transit-36 REPLACE: default VRF IPv4 Entry with ECMP path NHG+NH in default vrf
func testReplaceDefaultIPv4EntryECMPPath(t *testing.T, args *testArgs) {

	// Removing policy for the tc
	gnmi.Delete(t, args.dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Config())

	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	// 192.0.2.40/32  Self-Site
	weights1 := map[uint64]uint64{
		31: 10,
		32: 20,
		33: 30,
	}
	args.c1.AddNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Next-Site
	weights2 := map[uint64]uint64{
		41: 40,
		42: 30,
		43: 20,
		44: 40,
	}
	args.c1.AddNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.0", i, "32"))
	}
	weights3 := map[uint64]uint64{
		10: 85,
		20: 15,
	}
	args.c1.AddNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Traffic start
	portMaps := args.topology.Interfaces()
	args.topology.StartProtocols(t)
	defer args.topology.StopProtocols(t)
	scaleflow := getScaleFlow(t, portMaps, args.ate, "IPinIPWithScale", int(*ciscoFlags.GRIBIScale))
	args.ate.Traffic().Start(t, scaleflow)
	defer args.ate.Traffic().Stop(t)

	// Replace same ipv4 entry
	args.c1.ReplaceIPv4Batch(t, prefixes, 1, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// traffic verification
	time.Sleep(60 * time.Second)
	stats := gnmi.GetAll(t, args.ate, gnmi.OC().FlowAny().State())
	lossStream := util.CheckTrafficPassViaRate(stats)
	if len(lossStream) > 0 {
		t.Fatal("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}
}

// Transit-52 ADD/REPLACE change NH from single path to ECMP
func testReplaceSinglePathtoECMP(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	weights1 := map[uint64]uint64{
		3: 30,
	}
	args.c1.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "11.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Start Traffic
	portMaps := args.topology.Interfaces()
	args.topology.StartProtocols(t)
	defer args.topology.StopProtocols(t)
	baseflow := getBaseFlow(t, portMaps, args.ate, "IPinIP")
	args.ate.Traffic().Start(t, baseflow)
	defer args.ate.Traffic().Stop(t)
	// Add New NHG
	weights1 = map[uint64]uint64{
		3: 30,
		4: 30,
	}
	args.c1.AddNH(t, 4, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	// traffic verification
	time.Sleep(60 * time.Second)
	stats := gnmi.GetAll(t, args.ate, gnmi.OC().InterfaceAny().Counters().State())
	trafficPass := util.CheckTrafficPassViaPortPktCounter(stats)
	if trafficPass == true {
		t.Log("Traffic works as expected")
	} else {
		t.Fatal("Traffic doesn't work as expected")
	}
}

// Transit TC 068 - Verify ISIS/BGP control plane doesn’t  affect gRIBI related traffic with connected NHOP
func testIsisBgpControlPlaneInteractionWithGribi(t *testing.T, args *testArgs) {
	t.Log("Testcase: Verify ISIS/BGP control plane doesn’t  affect gRIBI related traffic with connected NHOP")
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	// 192.0.2.40/32  Self-Site
	weights1 := map[uint64]uint64{
		31: 30,
		32: 20,
		33: 10,
	}
	args.c1.AddNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Next-Site
	weights2 := map[uint64]uint64{
		41: 10,
		42: 20,
		43: 30,
		44: 40,
	}
	args.c1.AddNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.0", i, "32"))
		prefixes = append(prefixes, util.GetIPPrefix("121.1.1.1", i, "32"))
		prefixes = append(prefixes, util.GetIPPrefix("131.1.1.1", i, "32"))
	}
	weights3 := map[uint64]uint64{
		10: 85,
		20: 15,
	}
	args.c1.AddNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	//Generate flows over ISIS and BGP sessions.
	isisFlow := util.GetBoundedFlow(t, args.ate, args.topology, sortedAtePorts[0], sortedAtePorts[1], "transit_wecmp_isis_1", "transit_wecmp_isis_2", "isis", 0)
	bgpFlow := util.GetBoundedFlow(t, args.ate, args.topology, sortedAtePorts[0], sortedAtePorts[1], "network101", "bgp_network_2", "bgp", 0)
	scaleFlow := getScaleFlow(t, args.topology.Interfaces(), args.ate, "IPinIPWithScale", int(*ciscoFlags.GRIBIScale))
	// Configure ATE and Verify traffic
	performATEActionForMultipleFlows(t, "ate", true, 0.90, isisFlow, bgpFlow, scaleFlow)
}

// Transit TC 071 - Verify protocol (BGP) over gribi transit fwding entry
func testBgpProtocolOverGribiTransitEntry(t *testing.T, args *testArgs) {
	t.Log("Testcase: Verify protocol (BGP) over gribi transit fwding entry")
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	// 192.0.2.40/32  for east-to-west flow
	weights1 := map[uint64]uint64{
		31: 100,
	}
	args.c1.AddNH(t, 31, "100.120.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether120", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.140/32  for west-to-east flow
	weights2 := map[uint64]uint64{
		41: 100,
	}
	args.c1.AddNH(t, 41, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.140/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	weights3 := map[uint64]uint64{
		10: 100,
	}
	weights4 := map[uint64]uint64{
		20: 100,
	}
	args.c1.AddNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "11.11.11.1/32", 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 20, "192.0.2.140", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 2, 0, weights4, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "12.12.12.1/32", 2, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	//Configure BGP on TGN
	//Generate DSCP48 flow
	bgpFlow := util.GetBoundedFlow(t, args.ate, args.topology, sortedAtePorts[0], sortedAtePorts[1], "network101", "bgp_network_2", "bgp", 0)

	// Configure ATE and Verify traffic
	performATEActionForMultipleFlows(t, "ate", true, 0.99, bgpFlow)
}

// Transit TC 075 - ADD/REPLACE/DELETE with same Prefix with varying prefix lengths
func testAddReplaceDeleteWithSamePrefixWithVaryingPrefixLength(t *testing.T, args *testArgs) {
	t.Log("Testcase: Add, Replace, Delete operations with same prefix with varying prefix lengths and traffic verification")
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	// 192.0.2.40/32  Self-Site
	weights1 := map[uint64]uint64{
		31: 10,
		32: 20,
		33: 30,
	}
	args.c1.AddNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Next-Site
	weights2 := map[uint64]uint64{
		41: 10,
		42: 20,
		43: 30,
		44: 40,
	}
	args.c1.AddNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	prefixes := []string{}
	for i := 0; i <= int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.0", i, "32"))
	}
	weights3 := map[uint64]uint64{
		10: 85,
		20: 15,
	}
	args.c1.AddNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	weights1 = map[uint64]uint64{
		31: 10,
		32: 20,
		33: 30,
	}
	weights2 = map[uint64]uint64{
		41: 10,
		42: 20,
		43: 30,
		44: 40,
	}
	weights3 = map[uint64]uint64{
		10: 85,
		20: 15,
	}
	args.c1.ReplaceNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.c1.ReplaceNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.c1.ReplaceNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.c1.DeleteIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

	args.c1.DeleteIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)

	args.c1.DeleteIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)

	// Add back all entries
	weights1 = map[uint64]uint64{
		31: 10,
		32: 20,
		33: 30,
	}
	weights2 = map[uint64]uint64{
		41: 10,
		42: 20,
		43: 30,
		44: 40,
	}
	weights3 = map[uint64]uint64{
		10: 85,
		20: 15,
	}
	args.c1.AddNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.c1.AddNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.c1.AddNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	scaleFlow := getScaleFlow(t, args.topology.Interfaces(), args.ate, "IPinIPWithScale", int(*ciscoFlags.GRIBIScale))
	performATEActionForMultipleFlows(t, "ate", true, 0.99, scaleFlow)
}

// Transit-18: Next Hop Group resolution change NH from non-recursive and recursive
func testChangeNHFromNonRecursiveToRecursive(t *testing.T, args *testArgs) {
	t.Log("Testcase: Next Hop Group resolution change NH from recursive and non-recursive")
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	weights1 := map[uint64]uint64{
		3: 15,
	}
	args.c1.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// 192.0.2.40/32  Self-Site
	weights2 := map[uint64]uint64{
		31: 30,
		32: 20,
		33: 10,
	}
	args.c1.AddNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 40, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Next-Site
	weights3 := map[uint64]uint64{
		41: 40,
		42: 30,
		43: 20,
		44: 10,
	}
	args.c1.AddNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.0", i, "32"))
	}
	weights4 := map[uint64]uint64{
		10: 85,
		20: 15,
	}
	args.c1.AddNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights4, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 11, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Correct the related NH and verify traffic
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	performATEAction(t, "ate", int(*ciscoFlags.GRIBIScale), true)
}

// Transit- Set ISIS overload bit and then verify traffic
func testSetISISOverloadBit(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	// 192.0.2.40/32  Self-Site
	weights1 := map[uint64]uint64{
		31: 30,
		32: 20,
		33: 10,
	}
	args.c1.AddNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Next-Site
	weights2 := map[uint64]uint64{
		41: 40,
		42: 30,
		43: 20,
		44: 10,
	}
	args.c1.AddNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	prefixes := []string{}
	for i := 0; i < 100; i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.0", i, "32"))
	}
	weights3 := map[uint64]uint64{
		10: 85,
		20: 15,
	}
	args.c1.AddNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Configure ISIS overload bit
	gnmi.Update(t, args.dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Config(), &oc.NetworkInstance{
		Name: ygot.String("DEFAULT"),
	})
	gnmi.Update(t, args.dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "B4").Config(), &oc.NetworkInstance_Protocol{
		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS,
		Name:       ygot.String("B4"),
	})
	config := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "B4").Isis().Global().LspBit().OverloadBit().SetBit()
	gnmi.Update(t, args.dut, config.Config(), true)
	defer gnmi.Delete(t, args.dut, config.Config())

	performATEAction(t, "ate", 100, true)
}

// Transit- Change peer ip/mac address and then verify traffic
func testChangePeerAddress(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	// 192.0.2.40/32  Self-Site
	weights1 := map[uint64]uint64{
		31: 30,
		32: 20,
		33: 10,
	}
	args.c1.AddNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Next-Site
	weights2 := map[uint64]uint64{
		41: 40,
		42: 30,
		43: 20,
		44: 10,
	}
	args.c1.AddNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.0", i, "32"))
	}
	weights3 := map[uint64]uint64{
		10: 85,
		20: 15,
	}
	args.c1.AddNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Try to change peer mac or fallback to peer address
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()

	i := 1
	// portMaps["1/2"].IPv4().WithAddress(fmt.Sprintf("100.%d.1.3/24", 120+i)).WithDefaultGateway(fmt.Sprintf("100.%d.1.1", 120+i))
	// topology.Update(t)

	// Undo
	defer portMaps[sortedAtePorts[1]].IPv4().WithAddress(fmt.Sprintf("100.%d.1.2/24", 120+i)).WithDefaultGateway(fmt.Sprintf("100.%d.1.1", 120+i))
	defer topology.Update(t)

	performATEAction(t, "ate", int(*ciscoFlags.GRIBIScale), true)
}

// Transit- LC OIR
func testLCOIR(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	// 192.0.2.40/32  Self-Site
	weights1 := map[uint64]uint64{
		31: 30,
		32: 20,
		33: 10,
	}
	args.c1.AddNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Next-Site
	weights2 := map[uint64]uint64{
		41: 40,
		42: 30,
		43: 20,
		44: 10,
	}
	args.c1.AddNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.0", i, "32"))
	}
	weights3 := map[uint64]uint64{
		10: 85,
		20: 15,
	}
	args.c1.AddNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// LC OIR
	t.Log(" Reload the LC")
	//config.Reload(args.ctx, t, args.dut, "", "", 6*time.Minute)
	config.CMDViaGNMI(args.ctx, t, args.dut, "reload location 0/0/CPU0 noprompt \n")

	performATEAction(t, "ate", int(*ciscoFlags.GRIBIScale), true)
}

// Transit TC 072 - Verify dataplane fields(TTL, DSCP) with gribi transit fwding entry
func testDataPlaneFieldsOverGribiTransitFwdingEntry(t *testing.T, args *testArgs) {
	t.Log("Testcase:  Verify dataplane fields(TTL, DSCP) with gribi transit fwding entry")
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	// 192.0.2.40/32  for east-to-west flow
	weights1 := map[uint64]uint64{
		31: 100,
	}
	args.c1.AddNH(t, 31, "100.120.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether120", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.140/32  for west-to-east flow
	weights2 := map[uint64]uint64{
		41: 100,
	}
	args.c1.AddNH(t, 41, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.140/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	weights3 := map[uint64]uint64{
		10: 100,
	}
	prefixes1 := []string{}
	for i := 0; i < 10; i++ {
		prefixes1 = append(prefixes1, util.GetIPPrefix("101.1.1.1", i, "32"))
	}
	weights4 := map[uint64]uint64{
		20: 100,
	}
	prefixes2 := []string{}
	for i := 0; i < 10; i++ {
		prefixes2 = append(prefixes2, util.GetIPPrefix("102.1.1.1", i, "32"))
	}

	args.c1.AddNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes1, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.c1.AddNH(t, 20, "192.0.2.140", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 2, 0, weights4, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes2, 2, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	//Outer header TTL decrements by 1, DSCP stays same over gRIBI forwarding entry.
	//flow with dscp=48, ttl=100
	dscpTTLFlow := util.GetBoundedFlow(t, args.ate, args.topology, sortedAtePorts[0], sortedAtePorts[1], "network101", "network102", "dscpTtlFlow", 48, 100)
	//add acl with dscp=48, ttl=99. Transit traffic will have ttl decremented by 1
	aclName := "ttl_dscp"
	aclConfig := util.GetIpv4Acl(aclName, 10, 48, 99, oc.Acl_FORWARDING_ACTION_ACCEPT)
	gnmi.Update(t, args.dut, gnmi.OC().Acl().Config(), aclConfig)

	//delete acl
	defer gnmi.Delete(t, args.dut, gnmi.OC().Acl().AclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4).Config())
	//apply egress acl on all interfaces of interest
	interfaceNames := []string{"Bundle-Ether120", "Bundle-Ether121"}
	for _, interfaceName := range interfaceNames {
		gnmi.Update(t, args.dut, gnmi.OC().Acl().Interface(interfaceName).Config(), &oc.Acl_Interface{
			Id: ygot.String(interfaceName),
		})
		gnmi.Update(t, args.dut, gnmi.OC().Acl().Interface(interfaceName).EgressAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4).Config(), &oc.Acl_Interface_EgressAclSet{
			Type:    oc.Acl_ACL_TYPE_ACL_IPV4,
			SetName: &aclName,
		})
		gnmi.Update(t, args.dut, gnmi.OC().Acl().Interface(interfaceName).EgressAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4).SetName().Config(), aclName)
	}

	// Verify traffic passes through ACL - allowing same DSCP and TTL decremented by 1
	performATEActionForMultipleFlows(t, "ate", true, 0.99, dscpTTLFlow)

	//remove acl from interfaces
	for _, interfaceName := range interfaceNames {
		gnmi.Delete(t, args.dut, gnmi.OC().Acl().Interface(interfaceName).EgressAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4).Config())
	}
}

// Transit TC 074 - ADD/REPLACE/DELETE during related configuration change
func testAddReplaceDeleteWithRelatedConfigChange(t *testing.T, args *testArgs) {
	t.Log("Testcase: Add, Replace, Delete operations with related configuration change")
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	// 192.0.2.40/32  Self-Site
	weights1 := map[uint64]uint64{
		31: 10,
		32: 20,
		33: 30,
	}
	args.c1.AddNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Next-Site
	weights2 := map[uint64]uint64{
		41: 40,
		42: 30,
		43: 20,
		44: 10,
	}
	args.c1.AddNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.0", i, "32"))
	}
	weights3 := map[uint64]uint64{
		10: 85,
		20: 15,
	}
	args.c1.AddNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	//Replace all entries
	// 192.0.2.40/32  Self-Site
	weights1 = map[uint64]uint64{
		31: 30,
		32: 30,
		33: 30,
	}
	args.c1.ReplaceNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Self-Site
	weights2 = map[uint64]uint64{
		41: 40,
		42: 40,
		43: 40,
		44: 40,
	}
	args.c1.ReplaceNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	weights3 = map[uint64]uint64{
		10: 85,
		20: 15,
	}
	args.c1.ReplaceNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	//Delete all entries
	// 192.0.2.40/32  Self-Site
	args.c1.DeleteIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Self-Site
	args.c1.DeleteIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)

	args.c1.DeleteIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

	//Change interface configurations and revert back
	interfaceNames := []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126", "Bundle-Ether127"}
	//Store current config
	originalInterfaces := util.GetCopyOfIpv4Interfaces(t, args.dut, interfaceNames, 0)
	//Change IP addresses for the interfaces in the slice
	initialIP := "123.123.123.123"
	counter := 1
	for _, interfaceName := range interfaceNames {
		ipPrefix := util.GetIPPrefix(initialIP, counter, "24")
		initialIP = strings.Split(ipPrefix, "/")[0]
		gnmi.Replace(t, args.dut, gnmi.OC().Interface(interfaceName).Config(), util.GetInterface(interfaceName, initialIP, 24, 0))
		t.Logf("Changed configuration of interface %s", interfaceName)
		counter = counter + 256

	}
	//Revert original config
	for _, interfaceName := range interfaceNames {
		osi := originalInterfaces[interfaceName]
		// osi.Index = ygot.Uint32(0)
		gnmi.Replace(t, args.dut, gnmi.OC().Interface(interfaceName).Config(), osi)
		t.Logf("Restored configuration of interface %s", interfaceName)
	}
	//Config change end
	time.Sleep(30 * time.Second)
	weights1 = map[uint64]uint64{
		31: 10,
		32: 20,
		33: 30,
	}
	weights2 = map[uint64]uint64{
		41: 40,
		42: 30,
		43: 20,
		44: 10,
	}
	weights3 = map[uint64]uint64{
		10: 85,
		20: 15,
	}
	args.c1.AddNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.c1.AddNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.c1.AddNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Configure ATE and Verify traffic
	performATEAction(t, "ate", int(*ciscoFlags.GRIBIScale), true, 0.99)
}

// Static Arp Resolution
func testCD2StaticMacChangeNHOP(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	// 192.0.2.42/32  Next-Site
	weights1 := map[uint64]uint64{
		41: 40,
	}
	// adding static route since nh is not connected
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 100.121.1.9/32 Bundle-Ether121")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 100.121.1.9/32 Bundle-Ether121")

	args.c1.AddNH(t, 41, "100.121.1.9", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.11", i, "32"))
	}
	weights2 := map[uint64]uint64{
		20: 99,
	}
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	t.Log("going to program Static ARP different from Ixia ")
	config.TextWithGNMI(args.ctx, t, args.dut, "arp 100.121.1.9  0000.0012.0011 arpa")

	time.Sleep(10 * time.Second)

	portMaps := args.topology.Interfaces()

	args.topology.StartProtocols(t)
	defer args.topology.StopProtocols(t)
	baseflow := getBaseFlow(t, portMaps, args.ate, "IPinIP")
	t.Log("Traffic starting from Ixia should go with Next hop and Static ARP  ")
	args.ate.Traffic().Start(t, baseflow)
	defer args.ate.Traffic().Stop(t)

	time.Sleep(60 * time.Second)
	stats := gnmi.GetAll(t, args.ate, gnmi.OC().FlowAny().State())
	t.Log("slept and now need to collect stats")
	lossStream := util.CheckTrafficPassViaRate(stats)

	if len(lossStream) > 0 {
		t.Fatal("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}
	t.Log("going to change Static ARP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "arp 100.121.1.9  0000.0012.0011 arpa")

	time.Sleep(10 * time.Second)

	defer config.TextWithGNMI(args.ctx, t, args.dut, "no arp 100.121.1.9  0000.0012.0011 arpa")

	statsb := gnmi.GetAll(t, args.ate, gnmi.OC().FlowAny().State())
	lossStreamb := util.CheckTrafficPassViaRate(statsb)

	if len(lossStreamb) > 0 {
		t.Fatal("There is stream failing after configuring static arp :", strings.Join(lossStreamb, ","))
	} else {
		t.Log("There is no traffic loss even after adding static arp ")
	}

	// dut1.Telemetry().NetworkInstance().Afts().Ipv4Entry().Get(t)
	time.Sleep(10 * time.Second)
}

// Initially Dynamic arp and then static arp to be resolved
func testCD2StaticDynamicMacNHOP(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	// 192.0.2.42/32  Next-Site
	weights1 := map[uint64]uint64{
		41: 40,
	}
	args.c1.AddNH(t, 41, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.11", i, "32"))
	}
	weights2 := map[uint64]uint64{
		20: 99,
	}
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	t.Log("going to start Ixia protocols to bring up dynamic arp entry and start traffic  ")

	portMaps := args.topology.Interfaces()

	args.topology.StartProtocols(t)

	baseflow := getBaseFlow(t, portMaps, args.ate, "IPinIPDynamic")
	args.ate.Traffic().Start(t, baseflow)
	defer args.ate.Traffic().Stop(t)

	time.Sleep(60 * time.Second)
	stats := gnmi.GetAll(t, args.ate, gnmi.OC().FlowAny().State())
	lossStream := util.CheckTrafficPassViaRate(stats)

	if len(lossStream) > 0 {
		t.Fatal("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}
	t.Log("going to stop protocols to make sure static arp works ")
	args.topology.StopProtocols(t)

	t.Log("going to clear dynamic arp entry ")
	args.dut.RawAPIs().CLI(t).RunCommand(args.ctx, "clear arp-cache bundle-Ether 121 location all")

	time.Sleep(10 * time.Second)

	t.Log("going to configure static arp entry to make sure traffic is not failing after static arp is configured   ")
	util.GNMIWithText(args.ctx, t, args.dut, "arp 100.121.1.2  0000.0012.0011 arpa ")

	time.Sleep(10 * time.Second)

	defer util.GNMIWithText(args.ctx, t, args.dut, "no arp 100.121.1.2  0000.0012.0011 arpa")

	statsb := gnmi.GetAll(t, args.ate, gnmi.OC().FlowAny().State())
	lossStreamb := util.CheckTrafficPassViaRate(statsb)

	if len(lossStreamb) > 0 {
		t.Fatal("There is stream failing after configuring static arp :", strings.Join(lossStreamb, ","))
	} else {
		t.Log("There is no traffic loss even after adding static arp ")
	}
	// dut1.Telemetry().NetworkInstance().Afts().Ipv4Entry().Get(t)
	time.Sleep(10 * time.Second)
}

// Transit- Clearing ARP and then verify traffic
func testClearingARP(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	// 192.0.2.40/32  Self-Site
	weights1 := map[uint64]uint64{
		31: 10,
		32: 20,
		33: 30,
	}
	args.c1.AddNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Next-Site
	weights2 := map[uint64]uint64{
		41: 40,
		42: 30,
		43: 20,
		44: 10,
	}
	args.c1.AddNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.0", i, "32"))
	}
	weights3 := map[uint64]uint64{
		10: 85,
		20: 15,
	}
	args.c1.AddNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Clear ARP
	args.dut.RawAPIs().CLI(t).RunCommand(args.ctx, "clear arp-cache location all")
	time.Sleep(10 * time.Second)
	t.Log("Cleared ARP")
	time.Sleep(1 * time.Second)
	performATEAction(t, "ate", int(*ciscoFlags.GRIBIScale), true)
}

// Static Arp Resolution
func testCD2StaticMacNHOP(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	// 192.0.2.42/32  Next-Site
	weights1 := map[uint64]uint64{
		41: 40,
	}
	// adding static route since nh is not connected
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 100.121.1.9/32 Bundle-Ether121")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 100.121.1.9/32 Bundle-Ether121")

	args.c1.AddNH(t, 41, "100.121.1.9", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.11", i, "32"))
	}
	weights2 := map[uint64]uint64{
		20: 99,
	}
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	t.Log("going to program Static ARP different from Ixia ")
	config.TextWithGNMI(args.ctx, t, args.dut, "arp 100.121.1.9  0000.0012.0011 arpa")
	defer config.TextWithGNMI(args.ctx, t, args.dut, "no arp 100.121.1.9  0000.0012.0011 arpa")
	time.Sleep(10 * time.Second)

	if *ciscoFlags.GRIBITrafficCheck {
		checkTraffic(t, "IPinIP", args.ate, false)
	}

	// dut1.Telemetry().NetworkInstance().Afts().Ipv4Entry().Get(t)
	time.Sleep(10 * time.Second)
}

func TestTransitWECMPFlush(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")
	var vrfs = []string{vrf1, vrf2}
	configVRF(t, dut, vrfs)
	configureDUT(t, dut)
	configbasePBR(t, dut, "TE", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})
	configbasePBR(t, dut, "TE", 2, oc.PacketMatchTypes_IP_PROTOCOL_UNSET, []uint8{16})
	configbasePBR(t, dut, "VRF1", 3, oc.PacketMatchTypes_IP_PROTOCOL_UNSET, []uint8{18})
	configbasePBR(t, dut, "TE", 4, oc.PacketMatchTypes_IP_PROTOCOL_UNSET, []uint8{48})
	configRP(t, dut)
	addISISOC(t, dut, []string{"Bundle-Ether120", "Bundle-Ether121"})
	addBGPOC(t, dut, []string{"100.120.1.2", "100.121.1.2"})
	// convertFlowspecToPBR(ctx, t, dut)

	ate := ondatra.ATE(t, "ate")
	test := []struct {
		name string
		desc string
		fn   func(t *testing.T, args *testArgs)
	}{
		// Deactivated tc as PBR config is flawed and there is missing ondatra support, handling the PBR config in base config
		// {
		// 	name: "TestAddPBR",
		// 	desc: "ADD PBR", // make sure that PBR is added and no flowspec config is not in router
		// 	fn:   testChangeFlowSpecToPBR,
		// },
		{
			name: "CD2ConnectedNHIP",
			desc: "Transit Connected nexthop",
			fn:   testCD2ConnectedNHIP,
		},
		{
			name: "CD2RecursiveNonConnectedNHOP",
			desc: "Transit Recursive Non Connected nexthop",
			fn:   testCD2RecursiveNonConnectedNHOP,
		},
		{
			name: "AddIPv4EntryTrafficCheck",
			desc: "Transit-46 ADD same IPv4 Entry verify no traffic impact",
			fn:   testAddIPv4EntryTrafficCheck,
		},
		{
			name: "ReplaceIPv4EntryTrafficCheck",
			desc: "Transit-47 REPLACE same IPv4 Entry verify no traffic impact",
			fn:   testReplaceIPv4EntryTrafficCheck,
		},
		{
			name: "AddNHGTrafficCheck",
			desc: "Transit-48 ADD same NHG verify no traffic impact",
			fn:   testAddNHGTrafficCheck,
		},
		{
			name: "ReplaceNHGTrafficCheck",
			desc: "Transit-49 REPLACE same NHG verify no traffic impact",
			fn:   testReplaceNHGTrafficCheck,
		},
		{
			name: "AddNHTrafficCheck",
			desc: "Transit-50 ADD same NH verify no traffic impact",
			fn:   testAddNHTrafficCheck,
		},
		{
			name: "ReplaceNHTrafficCheck",
			desc: "Transit-51 REPLACE same NH verify no traffic impact",
			fn:   testReplaceNHTrafficCheck,
		},
		{
			name: "CD2SingleRecursion",
			desc: "Transit single recursion",
			fn:   testCD2SingleRecursion,
		},
		{
			name: "CD2DoubleRecursion",
			desc: "Transit double recursion",
			fn:   testCD2DoubleRecursion,
		},
		{
			name: "ReplaceDefaultIPv4EntrySinglePath",
			desc: "Transit-34 REPLACE: default VRF IPv4 Entry with single path NHG+NH in default vrf",
			fn:   testReplaceDefaultIPv4EntrySinglePath,
		},
		{
			name: "DeleteVRFIPv4EntrySinglePath",
			desc: "Transit-38 DELETE: VRF IPv4 Entry with single path NHG+NH in default vrf",
			fn:   testDeleteVRFIPv4EntrySinglePath,
		},
		{
			name: "DeleteDefaultIPv4EntrySinglePath",
			desc: "Transit-42 DELETE: default VRF IPv4 Entry with single path NHG+NH in default vrf",
			fn:   testDeleteDefaultIPv4EntrySinglePath,
		},
		{
			name: "TwoPrefixesWithSameSetOfPrimaryAndBackup",
			desc: "Transit TC 066 - Two prefixes with NHGs with backup pointing to the each other's NHG",
			fn:   testTwoPrefixesWithSameSetOfPrimaryAndBackup,
		},
		{
			name: "SameForwardingEntriesAcrossMultipleVrfs",
			desc: "Transit TC 067 - Same forwarding entries across multiple vrfs",
			fn:   testSameForwardingEntriesAcrossMultipleVrfs,
		},
		{
			name: "NHInterfaceInDifferentVRF",
			desc: "Transit-11: Next Hop resoultion with interface in different VRF of NH_network_instance",
			fn:   testNHInterfaceInDifferentVRF,
		},
		{
			name: "NHIPOutOfInterfaceSubnet",
			desc: "Transit-13: Next Hop resolution with interface+IP out of that interface subnet",
			fn:   testNHIPOutOfInterfaceSubnet,
		},
		{
			name: "ChangeNHToUnreachableAndChangeBack",
			desc: "Transit-16:Changing IP address on I/F making NHOP unreachable and changing it back",
			fn:   testChangeNHToUnreachableAndChangeBack,
		},
		{
			name: "ChangeNHFromRecursiveToNonRecursive",
			desc: "Transit-19: Next Hop Group resolution change NH from recursive and non-recursive",
			fn:   testChangeNHFromRecursiveToNonRecursive,
		},
		{
			name: "AddReplaceDeleteWithRelatedInterfaceFLap",
			desc: "Transit TC 073 - ADD/REPLACE/DELETE during related interface flap",
			fn:   testAddReplaceDeleteWithRelatedInterfaceFLap,
		},
		{
			name: "DeleteVRFIPv4EntryECMPPath",
			desc: "Transit-40 DELETE: VRF IPv4 Entry with ECMP path NHG+NH in default vrf",
			fn:   testDeleteVRFIPv4EntryECMPPath,
		},
		{
			name: "DeleteDefaultIPv4EntryECMPPath",
			desc: "Transit-45 DELETE: default VRF IPv4 Entry with ECMP+backup path NHG+NH in default vrf",
			fn:   testDeleteDefaultIPv4EntryECMPPath,
		},
		{
			name: "ReplaceVRFIPv4EntryECMPPath",
			desc: "Transit-32 REPLACE: VRF IPv4 Entry with ECMP path NHG+NH in default vrf",
			fn:   testReplaceVRFIPv4EntryECMPPath,
		},
		{
			name: "ReplaceSinglePathtoECMP",
			desc: "Transit-52 ADD/REPLACE change NH from single path to ECMP",
			fn:   testReplaceSinglePathtoECMP,
		},
		{
			name: "IsisBgpControlPlaneInteractionWithGribi",
			desc: "Transit TC 068 - Verify ISIS/BGP control plane doesn’t  affect gRIBI related traffic with connected NHOP",
			fn:   testIsisBgpControlPlaneInteractionWithGribi,
		},
		{
			name: "BgpProtocolOverGribiTransitEntry",
			desc: "Transit TC 071 - Verify protocol (BGP) over gribi transit fwding entry",
			fn:   testBgpProtocolOverGribiTransitEntry,
		},
		{
			name: "AddReplaceDeleteWithSamePrefixWithVaryingPrefixLength",
			desc: "Transit TC 075 - ADD/REPLACE/DELETE with same Prefix with varying prefix lengths",
			fn:   testAddReplaceDeleteWithSamePrefixWithVaryingPrefixLength,
		},
		{
			name: "ChangeNHFromNonRecursiveToRecursive",
			desc: "Transit-18: Next Hop Group resolution change NH from non-recursive and recursive",
			fn:   testChangeNHFromNonRecursiveToRecursive,
		},
		{
			name: "SetISISOverloadBit",
			desc: "Transit- Set ISIS overload bit and then verify traffici",
			fn:   testSetISISOverloadBit,
		},
		{
			name: "changePeerAddress",
			desc: "Transit- Change peer ip/mac address and then verify traffic",
			fn:   testChangePeerAddress,
		},
		{
			name: "LC_OIR",
			desc: "Transit- LC OIR",
			fn:   testLCOIR,
		},
		{
			name: "DataPlaneFieldsOverGribiTransitFwdingEntry",
			desc: "Transit TC 072 - Verify dataplane fields(TTL, DSCP) with gribi transit fwding entry",
			fn:   testDataPlaneFieldsOverGribiTransitFwdingEntry,
		},
		{
			name: "AddReplaceDeleteWithRelatedConfigChange",
			desc: "Transit TC 074 - ADD/REPLACE/DELETE during related configuration change",
			fn:   testAddReplaceDeleteWithRelatedConfigChange,
		},
		{
			name: "CD2StaticMacChangeNHOP",
			desc: "Static Arp Resolution",
			fn:   testCD2StaticMacChangeNHOP,
		},
		{
			name: "CD2StaticDynamicMacNHOP",
			desc: "Initially Dynamic arp and then static arp to be resolved",
			fn:   testCD2StaticDynamicMacNHOP,
		},
		{
			name: "ClearingARP",
			desc: "Transit- Clearing ARP and then verify traffic",
			fn:   testClearingARP,
		},
		{
			name: "CD2StaticMacNHOP",
			desc: "Static Arp Resolution",
			fn:   testCD2StaticMacNHOP,
		},
		{
			name: "ReplaceDefaultIPv4EntryECMPPath",
			desc: "Transit-36 REPLACE: default VRF IPv4 Entry with ECMP path NHG+NH in default vrf",
			fn:   testReplaceDefaultIPv4EntryECMPPath,
		},
	}
	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			t.Logf("Description: %s", tt.desc)
			topology := getIXIATopology(t, "ate")
			client1 := gribi.Client{
				DUT:                  dut,
				FibACK:               *ciscoFlags.GRIBIFIBCheck,
				Persistence:          true,
				InitialElectionIDLow: 100,
			}
			client2 := gribi.Client{
				DUT:                  dut,
				FibACK:               *ciscoFlags.GRIBIFIBCheck,
				Persistence:          true,
				InitialElectionIDLow: 10,
			}

			defer client1.Close(t)

			if err := client1.Start(t); err != nil {
				t.Fatalf("gRIBI Connection can not be established")
			}

			defer client2.Close(t)

			if err := client2.Start(t); err != nil {
				t.Fatalf("gRIBI Connection can not be established")
			}
			args := &testArgs{
				ctx:      ctx,
				c1:       &client1,
				c2:       &client2,
				dut:      dut,
				ate:      ate,
				topology: topology,
			}
			tt.fn(t, args)
		})
	}
}
