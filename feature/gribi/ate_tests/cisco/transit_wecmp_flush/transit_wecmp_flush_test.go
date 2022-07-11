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
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/gribigo/server"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
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
func testChangeFlowSpecToPBR(t *testing.T, args *testArgs) {
	t.Log("Remove flow spec config and apply pbr config")
	configToChange := "no flowspec \nhw-module profile pbr vrf-redirect\n"
	config.Reload(args.ctx, t, args.dut, configToChange, "", 6*time.Minute)
	configbasePBR(t, args.dut)
	args.dut.Config().NetworkInstance("default").PolicyForwarding().Interface("Bundle-Ether120").ApplyVrfSelectionPolicy().Update(t, pbrName)
}

func testCD2ConnectedNHIP(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t) // let test to clear the entries at the begining for itself.
	//Using defer is problamtic since it may cause failuer when the gribi connection drops
	weights := map[uint64]uint64{3: 15}
	args.c1.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
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
	args.c1.AddNH(t, 41, "100.129.1.2", *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks) // Not connected
	args.c1.AddNHG(t, 100, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32 Self-Site
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.11", i, "32"))
	}
	weights = map[uint64]uint64{20: 99}
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
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
	args.c1.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "11.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Start Traffic
	portMaps := args.topology.Interfaces()
	args.topology.StartProtocols(t)
	defer args.topology.StopProtocols(t)
	baseflow := getBaseFlow(t, portMaps, args.ate, "IPinIP")
	args.ate.Traffic().Start(t, baseflow)

	// Add same ipv4 entry
	args.c2.BecomeLeader(t)
	args.c2.AddIPv4(t, "11.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// traffic verification
	defer args.ate.Traffic().Stop(t)
	time.Sleep(60 * time.Second)

	stats := args.ate.Telemetry().FlowAny().Get(t)
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
	args.c1.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
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

	stats := args.ate.Telemetry().FlowAny().Get(t)
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
	args.c1.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "11.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Start Traffic
	portMaps := args.topology.Interfaces()
	args.topology.StartProtocols(t)
	defer args.topology.StopProtocols(t)
	baseflow := getBaseFlow(t, portMaps, args.ate, "IPinIP")
	args.ate.Traffic().Start(t, baseflow)

	// Add same NHG entry
	args.c2.BecomeLeader(t)
	args.c2.AddNHG(t, 11, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// traffic verification
	defer args.ate.Traffic().Stop(t)
	time.Sleep(60 * time.Second)
	stats := args.ate.Telemetry().FlowAny().Get(t)
	lossStream := util.CheckTrafficPassViaRate(stats)
	if len(lossStream) > 0 {
		t.Error("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}

}

// Transit-49 REPLACE same NHG verify no traffic impact
func testReplaceNHGTrafficCheck(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	weights := map[uint64]uint64{3: 15}
	args.c1.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "11.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	// Start Traffic
	portMaps := args.topology.Interfaces()
	args.topology.StartProtocols(t)
	defer args.topology.StopProtocols(t)
	baseflow := getBaseFlow(t, portMaps, args.ate, "IPinIP")
	args.ate.Traffic().Start(t, baseflow)
	// Replace same NHG entry
	args.c2.BecomeLeader(t)
	args.c2.ReplaceNHG(t, 11, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	// traffic verification

	time.Sleep(60 * time.Second)
	stats := args.ate.Telemetry().FlowAny().Get(t)
	lossStream := util.CheckTrafficPassViaRate(stats)
	if len(lossStream) > 0 {
		t.Error("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}

}

// Transit-50 ADD same NH verify no traffic impact
func testAddNHTrafficCheck(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	weights := map[uint64]uint64{3: 15}
	args.c1.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "11.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Start Traffic
	portMaps := args.topology.Interfaces()
	args.topology.StartProtocols(t)
	defer args.topology.StopProtocols(t)
	baseflow := getBaseFlow(t, portMaps, args.ate, "IPinIP")
	args.ate.Traffic().Start(t, baseflow)

	// Add same NH entry
	args.c2.BecomeLeader(t)
	args.c2.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// traffic verification
	defer args.ate.Traffic().Stop(t)
	time.Sleep(60 * time.Second)
	stats := args.ate.Telemetry().FlowAny().Get(t)
	lossStream := util.CheckTrafficPassViaRate(stats)
	if len(lossStream) > 0 {
		t.Error("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}

}

// Transit-51 REPLACE same NH verify no traffic impact
func testReplaceNHTrafficCheck(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	weights := map[uint64]uint64{3: 15}
	args.c1.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "11.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	// Start Traffic
	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()
	topology.StartProtocols(t)
	defer topology.StopProtocols(t)
	baseflow := getBaseFlow(t, portMaps, ate, "IPinIP")
	ate.Traffic().Start(t, baseflow)

	// Replace same NH entry
	args.c2.BecomeLeader(t)
	args.c2.ReplaceNHG(t, 11, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// traffic verification
	defer ate.Traffic().Stop(t)
	time.Sleep(60 * time.Second)
	stats := ate.Telemetry().FlowAny().Get(t)
	lossStream := util.CheckTrafficPassViaRate(stats)
	if len(lossStream) > 0 {
		t.Error("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}

}

func testCD2SingleRecursion(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	weights := map[uint64]uint64{3: 15}
	args.c1.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
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
	args.c1.AddNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 40, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Next-Site
	weights = map[uint64]uint64{
		41: 40,
		42: 30,
		43: 20,
		44: 10,
	}
	args.c1.AddNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
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
	args.c1.AddNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	performATEAction(t, "ate", int(*ciscoFlags.GRIBIScale), true)
}

// Transit-34 REPLACE: default VRF IPv4 Entry with single path NHG+NH in default vrf
func testReplaceDefaultIPv4EntrySinglePath(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	weights := map[uint64]uint64{3: 15}
	args.c1.AddNH(t, 3, "100.121.1.3", *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "11.11.11.11/32", 11, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Add New NHG
	args.c2.BecomeLeader(t)
	weights = map[uint64]uint64{4: 15}
	args.c2.AddNH(t, 4, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.c2.AddNHG(t, 12, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

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
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)
	ops := []func(){
		func() {
			fluentC1.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("11.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 15),
			)
		},
	}
	res := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := uint64(1); i < 4; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()

	topology.StartProtocols(t)
	defer topology.StopProtocols(t)

	baseflow := getBaseFlow(t, portMaps, ate, "IPinIP")
	ate.Traffic().Start(t, baseflow)
	defer ate.Traffic().Stop(t)

	time.Sleep(60 * time.Second)

	stats := ate.Telemetry().FlowAny().Get(t)
	lossStream := util.CheckTrafficPassViaRate(stats)

	if len(lossStream) > 0 {
		t.Error("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}

	// Delete Entry
	args.c2.BecomeLeader(t)
	fluentC2 := args.c2.Fluent(t)
	defer util.FlushServer(fluentC2, t)
	elecLow2, _ := args.c2.LearnElectionID(t)
	ops2 := []func(){
		func() {
			fluentC2.Modify().DeleteEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("11.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
			)
		},
	}
	res2 := util.DoModifyOps(fluentC2, t, ops2, fluent.InstalledInRIB, false, elecLow2+1)

	for i := uint64(1); i < 4; i++ {
		chk.HasResult(t, res2, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

}

// Transit-42 DELETE: default VRF IPv4 Entry with single path NHG+NH in default vrf
func testDeleteDefaultIPv4EntrySinglePath(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)
	ops := []func(){
		func() {
			fluentC1.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(6).WithIPAddress("100.121.1.2"),
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("11.11.11.11/32").WithNextHopGroup(16).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(16).AddNextHop(6, 15),
			)
		},
	}
	res := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := uint64(1); i < 4; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()

	topology.StartProtocols(t)
	defer topology.StopProtocols(t)

	baseflow := getBaseFlow(t, portMaps, ate, "IPinIP", "default")
	ate.Traffic().Start(t, baseflow)
	defer ate.Traffic().Stop(t)

	time.Sleep(60 * time.Second)

	stats := ate.Telemetry().FlowAny().Get(t)
	lossStream := util.CheckTrafficPassViaRate(stats)

	if len(lossStream) > 0 {
		t.Error("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}

	// Delete Entry
	args.c2.BecomeLeader(t)
	fluentC2 := args.c2.Fluent(t)
	defer util.FlushServer(fluentC2, t)
	elecLow2, _ := args.c2.LearnElectionID(t)
	ops2 := []func(){
		func() {
			fluentC2.Modify().DeleteEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(6),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(16),
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("11.11.11.11/32").WithNextHopGroup(16).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
			)
		},
	}
	res2 := util.DoModifyOps(fluentC2, t, ops2, fluent.InstalledInRIB, false, elecLow2+1)

	for i := uint64(1); i < 3; i++ {
		chk.HasResult(t, res2, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

}

//Transit TC 066 - Two prefixes with NHGs with backup pointing to the each other's NHG
func testTwoPrefixesWithSameSetOfPrimaryAndBackup(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)
	ops := []func(){
		func() {
			fluentC1.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 15),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("12.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
			)
		},
		func() {
			fluentC1.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(4).WithIPAddress("100.122.1.2"),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(14).WithBackupNHG(11).AddNextHop(4, 15),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("12.11.11.12/32").WithNextHopGroup(14).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
			)
		},
		func() {
			fluentC1.Modify().AddEntry(t, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).WithBackupNHG(14).AddNextHop(3, 15))
		},
	}
	results := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := uint64(1); i < uint64(len(results)-2); i++ {
		chk.HasResult(t, results, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()

	topology.StartProtocols(t)
	defer topology.StopProtocols(t)

	dscp16Flow := getDSCPFlow(t, portMaps, ate, "DSCP16", 1, 16, "12.11.11.12", "1/3")
	dscp10Flow := getDSCPFlow(t, portMaps, ate, "DSCP10", 1, 10, "12.11.11.11", "1/2")

	checkTrafficFlows(t, ate, 60, dscp16Flow, dscp10Flow)
}

//Transit TC 067 - Same forwarding entries across multiple vrfs
func testSameForwardingEntriesAcrossMultipleVrfs(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)

	ops := []func(){
		func() {
			fluentC1.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 15),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(4).WithIPAddress("100.122.1.2"),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(14).WithBackupNHG(11).AddNextHop(4, 15),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("12.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("12.11.11.12/32").WithNextHopGroup(14).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
			)
		},
		func() {
			//Add previously used prefixes in a different vrf
			fluentC1.Modify().AddEntry(t, fluent.IPv4Entry().WithNetworkInstance("VRF1").WithPrefix("12.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.IPv4Entry().WithNetworkInstance("VRF1").WithPrefix("12.11.11.12/32").WithNextHopGroup(14).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
			)
		},
	}
	results := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := uint64(1); i < uint64(len(results)-2); i++ {
		chk.HasResult(t, results, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()

	topology.StartProtocols(t)
	defer topology.StopProtocols(t)

	dscp16FlowVrfTE := getDSCPFlow(t, portMaps, ate, "DSCP16_vrf_TE", 1, 16, "12.11.11.12", "1/3")
	dscp10FlowVrfTE := getDSCPFlow(t, portMaps, ate, "DSCP10_vrf_TE", 1, 10, "12.11.11.11", "1/2")
	dscp18Flow1VrfVRF1 := getDSCPFlow(t, portMaps, ate, "DSCP16_vrf_VRF1", 1, 18, "12.11.11.12", "1/3")
	dscp18Flow2VrfVRF1 := getDSCPFlow(t, portMaps, ate, "DSCP10_vrf_VRF1", 1, 18, "12.11.11.11", "1/2")

	checkTrafficFlows(t, ate, 60, dscp16FlowVrfTE, dscp10FlowVrfTE, dscp18Flow1VrfVRF1, dscp18Flow2VrfVRF1)
}

// Transit-11: Next Hop resoultion with interface in different VRF of NH_network_instance
func testNHInterfaceInDifferentVRF(t *testing.T, args *testArgs) {
	t.Log("Testcase: Next Hop resoultion with interface in different VRF of NH_network_instance")
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)

	ops := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121").WithNextHopNetworkInstance("TE"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 10).
					AddNextHop(42, 20).
					AddNextHop(43, 30).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := uint64(1); i < 13+1000; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Correct the related NH and verify traffic
	ops = []func(){
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
			)
		},
	}

	res = util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+2)

	chk.HasResult(t, res, fluent.OperationResult().
		WithOperationID(1015).
		WithProgrammingResult(fluent.InstalledInRIB).
		AsResult(),
	)

	performATEAction(t, "ate", 1000, true)
}

// Transit-13: Next Hop resolution with interface+IP out of that interface subnet
func testNHIPOutOfInterfaceSubnet(t *testing.T, args *testArgs) {
	t.Log("Testcase: Next Hop resolution with interface+IP out of that interface subnet")
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)

	ops := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 30).
					AddNextHop(32, 30).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.2.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 10).
					AddNextHop(42, 20).
					AddNextHop(43, 30).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := uint64(1); i < 13+1000; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Correct the related NH and verify traffic
	ops = []func(){
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
			)
		},
	}

	res = util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+2)

	chk.HasResult(t, res, fluent.OperationResult().
		WithOperationID(1015).
		WithProgrammingResult(fluent.InstalledInRIB).
		AsResult(),
	)

	performATEAction(t, "ate", 1000, true)
}

// Transit-16:Changing IP address on I/F making NHOP unreachable and changing it back
func testChangeNHToUnreachableAndChangeBack(t *testing.T, args *testArgs) {
	t.Log("Testcase: Changing IP address on I/F making NHOP unreachable and changing it back")
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)
	ops := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 15).
					AddNextHop(32, 25).
					AddNextHop(33, 35),
				// Setting Index 31 IP out of the related subnet
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 10).
					AddNextHop(42, 20).
					AddNextHop(43, 30).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := uint64(1); i < 13+1000; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	// Correct the related NH and verify traffic
	ops = []func(){
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("1.2.3.4"),
			)
		},
	}

	res = util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+2)

	chk.HasResult(t, res, fluent.OperationResult().
		WithOperationID(1015).
		WithProgrammingResult(fluent.InstalledInRIB).
		AsResult(),
	)

	// Correct the related NH and verify traffic
	ops = []func(){
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
			)
		},
	}

	res = util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+3)

	chk.HasResult(t, res, fluent.OperationResult().
		WithOperationID(1016).
		WithProgrammingResult(fluent.InstalledInRIB).
		AsResult(),
	)

	performATEAction(t, "ate", 1000, true)

}

// Transit-19: Next Hop Group resolution change NH from recursive and non-recursive
func testChangeNHFromRecursiveToNonRecursive(t *testing.T, args *testArgs) {
	t.Log("Testcase: Next Hop Group resolution change NH from recursive and non-recursive")
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)

	ops := []func(){
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 15),
			)
		},
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				// Setting Index 31 IP out of the related subnet
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 15).
					AddNextHop(42, 25).
					AddNextHop(43, 35).
					AddNextHop(44, 45),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := uint64(1); i < 13+1000; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	// Correct the related NH and verify traffic
	ops = []func(){
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			fluentC1.Modify().AddEntry(t, entries...)
		},
	}

	res = util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+2)

	for i := uint64(1017); i < 1015+1000; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	performATEAction(t, "ate", 1000, true)

}

// Transit TC 073 - ADD/REPLACE/DELETE during related interface flap
func testAddReplaceDeleteWithRelatedInterfaceFLap(t *testing.T, args *testArgs) {
	t.Log("Testcase: Add, Replace, Delete operations with related interface flap")
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)
	scale := 100

	ops := []func(){
		//Add all entries
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 15).
					AddNextHop(42, 25).
					AddNextHop(43, 35).
					AddNextHop(44, 45),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().AddEntry(t, entries...)
		},
		//Replace all entries
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().ReplaceEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().ReplaceEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 10).
					AddNextHop(42, 20).
					AddNextHop(43, 30).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().ReplaceEntry(t, entries...)
		},
		//Delete all entries
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().DeleteEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().DeleteEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 10).
					AddNextHop(42, 20).
					AddNextHop(43, 30).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().DeleteEntry(t, entries...)
		},
	}
	results := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := 1; i <= 3*(scale+14); i++ {
		chk.HasResult(t, results, fluent.OperationResult().
			WithOperationID(uint64(i)).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	//Flap interfaces
	interfaceNames := []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126", "Bundle-Ether127"}
	for _, interfaceName := range interfaceNames {
		util.SetInterfaceState(t, args.dut, interfaceName, false)
	}
	time.Sleep(30 * time.Second)
	for _, interfaceName := range interfaceNames {
		util.SetInterfaceState(t, args.dut, interfaceName, true)
	}

	ops = []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 15).
					AddNextHop(32, 25).
					AddNextHop(33, 35),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 10).
					AddNextHop(42, 20).
					AddNextHop(43, 30).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().AddEntry(t, entries...)
		},
	}

	results = util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := 3*(scale+14) + 1; i <= 4*(scale+14); i++ {
		chk.HasResult(t, results, fluent.OperationResult().
			WithOperationID(uint64(i)).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Configure ATE and Verify traffic
	performATEAction(t, "ate", scale, true)
}

//Transit-40	DELETE: VRF IPv4 Entry with ECMP path NHG+NH in default vrf
func testDeleteVRFIPv4EntryECMPPath(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)

	ops := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 15).
					AddNextHop(42, 25).
					AddNextHop(43, 35).
					AddNextHop(44, 45),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := uint64(1); i < 13+1000; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	performATEAction(t, "ate", 1000, true)

	// Delete
	ops2 := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().DeleteEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().DeleteEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 30).
					AddNextHop(43, 20).
					AddNextHop(44, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().DeleteEntry(t, entries...)
		},
	}

	res2 := util.DoModifyOps(fluentC1, t, ops2, fluent.InstalledInRIB, false, elecLow1+2)

	for i := uint64(1015); i < 1015+1000; i++ {
		chk.HasResult(t, res2, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	// Expect traffic to fail
	performATEAction(t, "ate", 1000, false)

}

//Transit-45	DELETE: default VRF IPv4 Entry with ECMP+backup path NHG+NH in default vrf
func testDeleteDefaultIPv4EntryECMPPath(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)

	ops := []func(){
		func() {
			fluentC1.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 15),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(4).WithIPAddress("100.122.1.2"),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(14).WithBackupNHG(11).AddNextHop(4, 15),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("12.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("11.11.11.0/32").WithNextHopGroup(14).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
			)
		},
	}

	res := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := uint64(1); i < 7; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	performATEAction(t, "ate", 1, true)

	// Delete
	ops2 := []func(){
		func() {
			fluentC1.Modify().DeleteEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 15),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(4).WithIPAddress("100.122.1.2"),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(14).WithBackupNHG(11).AddNextHop(4, 15),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("12.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("11.11.11.0/32").WithNextHopGroup(14).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
			)
		},
	}
	res2 := util.DoModifyOps(fluentC1, t, ops2, fluent.InstalledInRIB, false, elecLow1+2)

	for i := uint64(7); i < 13; i++ {
		chk.HasResult(t, res2, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Expect traffic to fail
	performATEAction(t, "ate", 1, false)

}

//Transit-32 REPLACE: VRF IPv4 Entry with ECMP path NHG+NH in default vrf
func testReplaceVRFIPv4EntryECMPPath(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)

	ops := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 30).
					AddNextHop(43, 20).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := uint64(1); i < 13+1000; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Traffic start
	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()
	topology.StartProtocols(t)
	defer topology.StopProtocols(t)
	scaleflow := getScaleFlow(t, portMaps, ate, "IPinIPWithScale", 1000)
	ate.Traffic().Start(t, scaleflow)

	// Replace same ipv4 entry
	ops2 := []func(){
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}

			fluentC1.Modify().ReplaceEntry(t, entries...)
		},
	}

	res2 := util.DoModifyOps(fluentC1, t, ops2, fluent.InstalledInRIB, false, elecLow1+2)

	for i := uint64(1015); i < 1015+1000; i++ {
		chk.HasResult(t, res2, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// traffic verification
	defer ate.Traffic().Stop(t)
	time.Sleep(60 * time.Second)
	stats := ate.Telemetry().InterfaceAny().Counters().Get(t)
	trafficPass := util.CheckTrafficPassViaPortPktCounter(stats)
	if trafficPass == true {
		t.Log("Traffic works as expected")
	} else {
		t.Error("Traffic doesn't work as expected")
	}

}

//Transit-36 REPLACE: default VRF IPv4 Entry with ECMP path NHG+NH in default vrf
func testReplaceDefaultIPv4EntryECMPPath(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)

	ops := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 30).
					AddNextHop(43, 20).
					AddNextHop(44, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := uint64(1); i < 13+1000; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)

	}

	// Replace same ipv4 entry
	ops2 := []func(){
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}

			fluentC1.Modify().ReplaceEntry(t, entries...)
		},
	}

	res2 := util.DoModifyOps(fluentC1, t, ops2, fluent.InstalledInRIB, false, elecLow1+2)

	for i := uint64(1015); i < 1015+1000; i++ {
		chk.HasResult(t, res2, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	// Traffic start
	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()
	topology.StartProtocols(t)
	defer topology.StopProtocols(t)
	scaleflow := getScaleFlow(t, portMaps, ate, "IPinIPWithScale", 1000, "default")
	ate.Traffic().Start(t, scaleflow)

	// traffic verification
	defer ate.Traffic().Stop(t)
	time.Sleep(60 * time.Second)
	stats := ate.Telemetry().InterfaceAny().Counters().Get(t)
	trafficPass := util.CheckTrafficPassViaPortPktCounter(stats)
	if trafficPass == true {
		t.Log("Traffic works as expected")
	} else {
		t.Error("Traffic doesn't work as expected")
	}

}

// Transit-52	ADD/REPLACE change NH from single path to ECMP
func testReplaceSinglePathtoECMP(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)

	ops := []func(){
		func() {
			fluentC1.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("11.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 30),
			)
		},
	}
	res := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := uint64(1); i < 4; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Start Traffic
	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()
	topology.StartProtocols(t)
	defer topology.StopProtocols(t)
	baseflow := getBaseFlow(t, portMaps, ate, "IPinIP")
	ate.Traffic().Start(t, baseflow)

	// Add New NHG
	ops2 := []func(){
		func() {
			fluentC1.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(4).WithIPAddress("100.122.1.2"),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(4, 30).AddNextHop(3, 30),
			)
		},
	}
	res2 := util.DoModifyOps(fluentC1, t, ops2, fluent.InstalledInRIB, false, elecLow1+2)

	for i := uint64(4); i < 6; i++ {
		chk.HasResult(t, res2, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// traffic verification
	defer ate.Traffic().Stop(t)
	time.Sleep(60 * time.Second)
	stats := ate.Telemetry().InterfaceAny().Counters().Get(t)
	trafficPass := util.CheckTrafficPassViaPortPktCounter(stats)
	if trafficPass == true {
		t.Log("Traffic works as expected")
	} else {
		t.Error("Traffic doesn't work as expected")
	}

}

// Transit TC 068 - Verify ISIS/BGP control plane doesn’t  affect gRIBI related traffic with connected NHOP
func testIsisBgpControlPlaneInteractionWithGribi(t *testing.T, args *testArgs) {
	t.Log("Testcase: Verify ISIS/BGP control plane doesn’t  affect gRIBI related traffic with connected NHOP")
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)
	scale := 100

	ops := []func(){
		//Add all entries
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 30).
					AddNextHop(32, 20).
					AddNextHop(33, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 10).
					AddNextHop(42, 20).
					AddNextHop(43, 30).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().AddEntry(t, entries...)
		},
	}
	results := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := 1; i <= scale+14; i++ {
		chk.HasResult(t, results, fluent.OperationResult().
			WithOperationID(uint64(i)).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	//Generate flows over ISIS and BGP sessions.
	ate := ondatra.ATE(t, "ate")
	topo := getIXIATopology(t, "ate")
	isisFlow := util.GetBoundedFlow(t, ate, topo, "1/1", "1/2", "isis_network1", "isis_network2", "isis", 16)
	bgpFlow := util.GetBoundedFlow(t, ate, topo, "1/1", "1/2", "bgp_network", "bgp_network", "bgp", 16)
	scaleFlow := getScaleFlow(t, topo.Interfaces(), ate, "IPinIPWithScale", scale)
	// Configure ATE and Verify traffic
	performATEActionForMultipleFlows(t, "ate", true, 0.90, isisFlow, bgpFlow, scaleFlow)
}

// Transit TC 071 - Verify protocol (BGP) over gribi transit fwding entry
func testBgpProtocolOverGribiTransitEntry(t *testing.T, args *testArgs) {
	t.Log("Testcase: Verify protocol (BGP) over gribi transit fwding entry")
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)

	ops := []func(){
		// 192.0.2.40/32  for east-to-west flow
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).AddNextHop(31, 100),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.120.1.2").WithInterfaceRef("Bundle-Ether120"),
			)
		},
		// 192.0.2.140/32  for west-to-east flow
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.140/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).AddNextHop(41, 100),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
			)
		},
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("11.11.11.1/32").WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 100),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("12.12.12.1/32").WithNextHopGroup(2).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(2).AddNextHop(20, 100),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.140"),
			)
		},
	}
	results := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := 1; i <= 12; i++ {
		chk.HasResult(t, results, fluent.OperationResult().
			WithOperationID(uint64(i)).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	//Configure BGP on TGN
	ate := ondatra.ATE(t, "ate")
	topo := getIXIATopology(t, "ate")
	//Generate DSCP48 flow
	bgpFlow := util.GetBoundedFlow(t, ate, topo, "1/1", "1/2", "bgp_transit_network", "bgp_transit_network", "bgp", 48)

	// Configure ATE and Verify traffic
	performATEActionForMultipleFlows(t, "ate", true, 0.99, bgpFlow)

}

// Transit TC 075 - ADD/REPLACE/DELETE with same Prefix with varying prefix lengths
func testAddReplaceDeleteWithSamePrefixWithVaryingPrefixLength(t *testing.T, args *testArgs) {
	t.Log("Testcase: Add, Replace, Delete operations with same prefix with varying prefix lengths and traffic verification")
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)

	//create ipv4Entry for subnet 11.0.0.0/8 through 11.11.11.1/32
	start := 8
	end := 32
	prefix := "11.11.11.1"

	ops := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 10).
					AddNextHop(42, 20).
					AddNextHop(43, 30).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := start; i <= end; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIpv4Net(prefix, i)).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().AddEntry(t, entries...)
		},
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := start; i <= end; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIpv4Net(prefix, i)).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().ReplaceEntry(t, entries...)
		},
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().ReplaceEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().ReplaceEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 10).
					AddNextHop(42, 20).
					AddNextHop(43, 30).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		func() {
			entries := []fluent.GRIBIEntry{}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))
			for i := start; i <= end; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIpv4Net(prefix, i)).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			fluentC1.Modify().DeleteEntry(t, entries...)
		},
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().DeleteEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().DeleteEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 10).
					AddNextHop(42, 20).
					AddNextHop(43, 30).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
	}

	results := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := 1; i <= (end-start+15)*3; i++ {
		chk.HasResult(t, results, fluent.OperationResult().
			WithOperationID(uint64(i)).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Add back all entries

	ops = []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 10).
					AddNextHop(42, 20).
					AddNextHop(43, 30).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := start; i <= end; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIpv4Net(prefix, i)).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().AddEntry(t, entries...)
		},
	}

	results = util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+2)

	for i := (end-start+15)*3 + 1; i <= (end-start+15)*4; i++ {
		chk.HasResult(t, results, fluent.OperationResult().
			WithOperationID(uint64(i)).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	ate := ondatra.ATE(t, "ate")
	topo := getIXIATopology(t, "ate")
	scaleFlow := getScaleFlow(t, topo.Interfaces(), ate, "IPinIPWithScale", 1000)
	performATEActionForMultipleFlows(t, "ate", true, 0.99, scaleFlow)

}

// Transit-18: Next Hop Group resolution change NH from non-recursive and recursive
func testChangeNHFromNonRecursiveToRecursive(t *testing.T, args *testArgs) {
	t.Log("Testcase: Next Hop Group resolution change NH from recursive and non-recursive")
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)

	ops := []func(){
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 15),
			)
		},
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 30).
					AddNextHop(32, 20).
					AddNextHop(33, 10),
				// Setting Index 31 IP out of the related subnet
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 30).
					AddNextHop(43, 20).
					AddNextHop(44, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := uint64(1); i < 13+1000; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	// Correct the related NH and verify traffic
	ops = []func(){
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			fluentC1.Modify().AddEntry(t, entries...)
		},
	}

	res = util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+2)

	for i := uint64(1017); i < 1015+1000; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	performATEAction(t, "ate", 1000, true)

}

// Transit- Set ISIS overload bit and then verify traffic
func testSetISISOverloadBit(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)
	scale := 100

	ops := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 30).
					AddNextHop(32, 20).
					AddNextHop(33, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 30).
					AddNextHop(43, 20).
					AddNextHop(44, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := uint64(1); i < 13+100; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Configure ISIS overload bit
	config := args.dut.Config().NetworkInstance("default").Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "B4").Isis().Global().LspBit().OverloadBit().SetBit()
	config.Update(t, true)
	defer config.Delete(t)

	performATEAction(t, "ate", 100, true)
}

// Transit- Change peer ip/mac address and then verify traffic
func testChangePeerAddress(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)

	scale := 1000

	ops := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 30).
					AddNextHop(32, 20).
					AddNextHop(33, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 30).
					AddNextHop(43, 20).
					AddNextHop(44, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := uint64(1); i < 13+100; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Try to change peer mac or fallback to peer address
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()

	i := 1
	// portMaps["1/2"].IPv4().WithAddress(fmt.Sprintf("100.%d.1.3/24", 120+i)).WithDefaultGateway(fmt.Sprintf("100.%d.1.1", 120+i))
	// topology.Update(t)

	performATEAction(t, "ate", scale, true)

	// Undo
	portMaps["1/2"].IPv4().WithAddress(fmt.Sprintf("100.%d.1.2/24", 120+i)).WithDefaultGateway(fmt.Sprintf("100.%d.1.1", 120+i))
	topology.Update(t)
}

// Transit- LC OIR
func testLCOIR(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)

	scale := 1000

	ops := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 30).
					AddNextHop(32, 20).
					AddNextHop(33, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 30).
					AddNextHop(43, 20).
					AddNextHop(44, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := uint64(1); i < 13+100; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// LC OIR
	//dut1.Config().New().WithCiscoText(" do reload location 0/0/CPU0 noprompt \n").Append(t)
	//t.Log(" Reload the LC")

	performATEAction(t, "ate", scale, true)
}

// Transit TC 072 - Verify dataplane fields(TTL, DSCP) with gribi transit fwding entry
func testDataPlaneFieldsOverGribiTransitFwdingEntry(t *testing.T, args *testArgs) {
	t.Log("Testcase:  Verify dataplane fields(TTL, DSCP) with gribi transit fwding entry")
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)

	scale := 100

	ops := []func(){
		// 192.0.2.40/32  for east-to-west flow
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).AddNextHop(31, 100),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.120.1.2").WithInterfaceRef("Bundle-Ether120"),
			)
		},
		// 192.0.2.140/32  for west-to-east flow
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.140/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).AddNextHop(41, 100),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
			)
		},
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("101.1.1.1", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 100))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			fluentC1.Modify().AddEntry(t, entries...)
		},
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("102.1.1.1", i, "32")).WithNextHopGroup(2).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(2).AddNextHop(20, 100))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.140"))
			fluentC1.Modify().AddEntry(t, entries...)
		},
	}
	results := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := 1; i <= 2*scale+10; i++ {
		chk.HasResult(t, results, fluent.OperationResult().
			WithOperationID(uint64(i)).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	//Outer header TTL decrements by 1, DSCP stays same over gRIBI forwarding entry.
	ate := ondatra.ATE(t, "ate")
	topo := getIXIATopology(t, "ate")

	//flow with dscp=48, ttl=100
	dscpTTLFlow := util.GetBoundedFlow(t, ate, topo, "1/1", "1/2", "network101", "network102", "dscpTtlFlow", 48, 100)
	//add acl with dscp=48, ttl=99. Transit traffic will have ttl decremented by 1
	aclName := "ttl_dscp"
	aclConfig := util.GetIpv4Acl(aclName, 10, 48, 99, telemetry.Acl_FORWARDING_ACTION_ACCEPT)
	args.dut.Config().Acl().Update(t, aclConfig)
	//apply egress acl on all interfaces of interest
	interfaceNames := []string{"Bundle-Ether120", "Bundle-Ether121"}
	for _, interfaceName := range interfaceNames {
		args.dut.Config().Acl().Interface(interfaceName).EgressAclSet(aclName, telemetry.Acl_ACL_TYPE_ACL_IPV4).SetName().Update(t, aclName)
	}

	// Verify traffic passes through ACL - allowing same DSCP and TTL decremented by 1
	performATEActionForMultipleFlows(t, "ate", true, 0.99, dscpTTLFlow)

	//remove acl from interfaces
	for _, interfaceName := range interfaceNames {
		args.dut.Config().Acl().Interface(interfaceName).EgressAclSet(aclName, telemetry.Acl_ACL_TYPE_ACL_IPV4).Delete(t)
	}
	//delete acl
	args.dut.Config().Acl().AclSet(aclName, telemetry.Acl_ACL_TYPE_ACL_IPV4).Delete(t)

}

// Transit TC 074 - ADD/REPLACE/DELETE during related configuration change
func testAddReplaceDeleteWithRelatedConfigChange(t *testing.T, args *testArgs) {
	t.Log("Testcase: Add, Replace, Delete operations with related configuration change")
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)

	scale := 100

	ops := []func(){
		//Add all entries
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 20).
					AddNextHop(43, 30).
					AddNextHop(44, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().AddEntry(t, entries...)
		},
		//Replace all entries
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().ReplaceEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 30).
					AddNextHop(32, 30).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().ReplaceEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 40).
					AddNextHop(43, 40).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().ReplaceEntry(t, entries...)
		},
		//Delete all entries
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().DeleteEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 30).
					AddNextHop(32, 30).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().DeleteEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 40).
					AddNextHop(43, 40).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().DeleteEntry(t, entries...)
		},
	}
	results := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := 1; i <= 3*(scale+14); i++ {
		chk.HasResult(t, results, fluent.OperationResult().
			WithOperationID(uint64(i)).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	//Change interface configurations and revert back
	interfaceNames := []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126", "Bundle-Ether127"}
	//Store current config
	originalInterfaces := util.GetCopyOfIpv4SubInterfaces(t, args.dut, interfaceNames, 0)
	//Change IP addresses for the interfaces in the slice
	initialIP := "123.123.123.123"
	counter := 1
	for _, interfaceName := range interfaceNames {
		ipPrefix := util.GetIPPrefix(initialIP, counter, "24")
		initialIP = strings.Split(ipPrefix, "/")[0]
		args.dut.Config().Interface(interfaceName).Subinterface(0).Replace(t, util.GetSubInterface(initialIP, 24, 0))
		t.Logf("Changed configuration of interface %s", interfaceName)
		counter = counter + 256

	}
	//Revert original config
	for _, interfaceName := range interfaceNames {
		osi := originalInterfaces[interfaceName]
		osi.Index = ygot.Uint32(0)
		args.dut.Config().Interface(interfaceName).Subinterface(0).Replace(t, osi)
		t.Logf("Restored configuration of interface %s", interfaceName)
	}
	//Config change end
	ops = []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 30).
					AddNextHop(32, 30).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 40).
					AddNextHop(43, 40).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().AddEntry(t, entries...)
		},
	}

	results = util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := 3*(scale+14) + 1; i <= 4*(scale+14); i++ {
		chk.HasResult(t, results, fluent.OperationResult().
			WithOperationID(uint64(i)).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Configure ATE and Verify traffic
	performATEAction(t, "ate", scale, true, 0.99)
}

//Static Arp Resolution
func testCD2StaticMacChangeNHOP(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)

	ops := []func(){

		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.121.1.3").WithInterfaceRef("Bundle-Ether121"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.11", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(20, 99))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := uint64(1); i < 7; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	t.Log("going to program Static ARP different from Ixia ")
	config.TextWithGNMI(args.ctx, t, args.dut, "arp 100.121.1.3  0000.0012.0011 arpa")

	time.Sleep(10 * time.Second)

	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()

	topology.StartProtocols(t)
	defer topology.StopProtocols(t)
	baseflow := getBaseFlow(t, portMaps, ate, "IPinIP")
	t.Log("Traffic starting from Ixia should go with Next hop and Static ARP  ")
	ate.Traffic().Start(t, baseflow)
	defer ate.Traffic().Stop(t)

	time.Sleep(60 * time.Second)
	stats := ate.Telemetry().FlowAny().Get(t)
	t.Log("slept and now need to collect stats")
	lossStream := util.CheckTrafficPassViaRate(stats)

	if len(lossStream) > 0 {
		t.Error("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}
	t.Log("going to change Static ARP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "arp 100.121.1.3  0000.0012.0011 arpa")

	time.Sleep(10 * time.Second)

	defer config.TextWithGNMI(args.ctx, t, args.dut, "no arp 100.121.1.3  0000.0012.0011 arpa")

	statsb := ate.Telemetry().FlowAny().Get(t)
	lossStreamb := util.CheckTrafficPassViaRate(statsb)

	if len(lossStreamb) > 0 {
		t.Error("There is stream failing after configuring static arp :", strings.Join(lossStreamb, ","))
	} else {
		t.Log("There is no traffic loss even after adding static arp ")
	}

	// dut1.Telemetry().NetworkInstance().Afts().Ipv4Entry().Get(t)
	time.Sleep(10 * time.Second)
}

//Initially Dynamic arp and then static arp to be resolved
func testCD2StaticDynamicMacNHOP(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)

	ops := []func(){

		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.11", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(20, 99))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := uint64(1); i < 7; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	t.Log("going to start Ixia protocols to bring up dynamic arp entry and start traffic  ")

	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()

	topology.StartProtocols(t)

	baseflow := getBaseFlow(t, portMaps, ate, "IPinIPDynamic")
	ate.Traffic().Start(t, baseflow)
	defer ate.Traffic().Stop(t)

	time.Sleep(60 * time.Second)
	stats := ate.Telemetry().FlowAny().Get(t)
	lossStream := util.CheckTrafficPassViaRate(stats)

	if len(lossStream) > 0 {
		t.Error("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}
	t.Log("going to stop protocols to make sure static arp works ")
	topology.StopProtocols(t)

	t.Log("going to clear dynamic arp entry ")
	config.TextWithGNMI(args.ctx, t, args.dut, "do clear arp-cache bundle-Ether 121 location 0/RP0/CPU0")

	time.Sleep(10 * time.Second)

	t.Log("going to configure static arp entry to make sure traffic is not failing after static arp is configured   ")
	config.TextWithGNMI(args.ctx, t, args.dut, "arp 100.121.1.2  0000.0012.0011 arpa ")

	time.Sleep(10 * time.Second)

	defer config.TextWithGNMI(args.ctx, t, args.dut, "no arp 100.121.1.2  0000.0012.0011 arpa")

	statsb := ate.Telemetry().FlowAny().Get(t)
	lossStreamb := util.CheckTrafficPassViaRate(statsb)

	if len(lossStreamb) > 0 {
		t.Error("There is stream failing after configuring static arp :", strings.Join(lossStreamb, ","))
	} else {
		t.Log("There is no traffic loss even after adding static arp ")
	}
	// dut1.Telemetry().NetworkInstance().Afts().Ipv4Entry().Get(t)
	time.Sleep(10 * time.Second)
}

// Transit- Clearing ARP and then verify traffic
func testClearingARP(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)

	ops := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 30).
					AddNextHop(43, 20).
					AddNextHop(44, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := uint64(1); i < 13+1000; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Clear ARP
	config.TextWithGNMI(args.ctx, t, args.dut, "do clear arp-cache location all")

	time.Sleep(10 * time.Second)

	t.Log("Cleared ARP")

	time.Sleep(1 * time.Second)

	performATEAction(t, "ate", 1000, true)

}

//Static Arp Resolution
func testCD2StaticMacNHOP(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	fluentC1 := args.c1.Fluent(t)
	defer util.FlushServer(fluentC1, t)
	elecLow1, _ := args.c1.LearnElectionID(t)

	ops := []func(){

		// 192.0.2.42/32  Next-Site
		func() {
			fluentC1.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.121.1.3").WithInterfaceRef("Bundle-Ether121"),
			)
		},
		// 11.11.11.1132
		func() {
			scale := 1
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIPPrefix("11.11.11.11", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(20, 99))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			fluentC1.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(fluentC1, t, ops, fluent.InstalledInRIB, false, elecLow1+1)

	for i := uint64(1); i < 7; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	t.Log("going to program Static ARP different from Ixia ")
	config.TextWithGNMI(args.ctx, t, args.dut, "arp 100.121.1.3  0000.0012.0011 arpa")

	time.Sleep(10 * time.Second)

	defer config.TextWithGNMI(args.ctx, t, args.dut, "no arp 100.121.1.3  0000.0012.0011 arpa")

	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()

	topology.StartProtocols(t)
	defer topology.StopProtocols(t)
	baseflow := getBaseFlow(t, portMaps, ate, "IPinIP")
	t.Log("Traffic starting from Ixia should go with Next hop and Static ARP  ")
	ate.Traffic().Start(t, baseflow)
	defer ate.Traffic().Stop(t)
	time.Sleep(60 * time.Second)

	stats := ate.Telemetry().FlowAny().Get(t)
	lossStream := util.CheckTrafficPassViaRate(stats)

	if len(lossStream) > 0 {
		t.Error("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}

	// dut1.Telemetry().NetworkInstance().Afts().Ipv4Entry().Get(t)
	time.Sleep(10 * time.Second)
}

func TestTransitWECMPFlush(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")
	// convertFlowspecToPBR(ctx, t, dut)
	ate := ondatra.ATE(t, "ate")
	test := []struct {
		name string
		desc string
		fn   func(t *testing.T, args *testArgs)
	}{
		/*{
			name: "TestAddPBR",
			desc: "ADD PBR",  // make sure that PBR is added and no flowspec config is not in router
			fn:   testChangeFlowSpecToPBR,
		},*/
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
		}, /*
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
				desc: "Transit-40	DELETE: VRF IPv4 Entry with ECMP path NHG+NH in default vrf",
				fn: testDeleteVRFIPv4EntryECMPPath,
			},
			{
				name: "DeleteDefaultIPv4EntryECMPPath",
				desc: "Transit-45	DELETE: default VRF IPv4 Entry with ECMP+backup path NHG+NH in default vrf",
				fn: testDeleteDefaultIPv4EntryECMPPath,
			},
			{
				name: "ReplaceVRFIPv4EntryECMPPath",
				desc: "Transit-32 REPLACE: VRF IPv4 Entry with ECMP path NHG+NH in default vrf",
				fn:   testReplaceVRFIPv4EntryECMPPath,
			},
			{
				name: "ReplaceDefaultIPv4EntryECMPPath",
				desc: "Transit-36 REPLACE: default VRF IPv4 Entry with ECMP path NHG+NH in default vrf",
				fn:   testReplaceDefaultIPv4EntryECMPPath,
			},
			{
				name: "ReplaceSinglePathtoECMP",
				desc: "Transit-52	ADD/REPLACE change NH from single path to ECMP",
				fn: testReplaceSinglePathtoECMP,
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
			},*/
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
