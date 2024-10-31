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

package twofiftysix_ucmp_test

import (
	"context"
	"strings"
	"time"

	"testing"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/gribi"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/gribigo/fluent"

	//"github.com/openconfig/featureprofiles/internal/gribi"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
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

const (
	ipv4PrefixLen         = 30
	ipv6PrefixLen         = 126
	dstPfx                = "198.51.100.1"
	mask                  = "32"
	dstPfxMin             = "198.51.100.1"
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
	bundleEther120        = "Bundle-Ether120"
	bundleEther125        = "Bundle-Ether125"
	bundleEther126        = "Bundle-Ether126"
	bundleEther127        = "Bundle-Ether127"
	lc                    = "0/0/CPU0"
	active_rp             = "0/RP0/CPU0"
	standby_rp            = "0/RP1/CPU0"
	vip1                  = "192.0.2.40/32"
	vip2                  = "192.0.2.42/32"
	vip1ip                = "192.0.2.40"
	vip2ip                = "192.0.2.42"
	dip                   = "10.1.0.1/32"
	dsip                  = "10.1.0.1"
	vrf1                  = "TE"
	vrf2                  = "TE2"
	vrf3                  = "TE3"
)

var baseconfigdone bool

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx    context.Context
	client *gribi.Client
	dut    *ondatra.DUTDevice
	ate    *ondatra.ATEDevice
	top    *ondatra.ATETopology
}

var args *testArgs

func baseconfig(t *testing.T) {
	t.Helper()

	if !baseconfigdone {
		args = &testArgs{}

		//Configure the DUT
		dut := ondatra.DUT(t, "dut")
		configureDUT(t, dut)
		configbasePBR(t, dut, "TE", "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{}, "PBR", dut.Port(t, "port1").Name(), false)
		//configure route-policy
		configRP(t, dut)
		//configure ISIS on DUT
		util.AddISISOC(t, dut, bundleEther126)
		//configure BGP on DUT
		util.AddBGPOC(t, dut, "100.100.100.100")
		configureNetworkInstance(t, dut)
		// Configure the ATE
		args.ate = ondatra.ATE(t, "ate")
		args.top = configureATE(t, args.ate)
		if *ciscoFlags.GRIBITrafficCheck {
			addPrototoAte(t, args.top)
		}
		baseconfigdone = true
	}
}

func addStaticRoute(t *testing.T, dut *ondatra.DUTDevice, ip string, val bool) {
	t.Helper()
	d := gnmi.OC()
	s := &oc.Root{}
	static := s.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "DEFAULT")
	ipv4Nh := static.GetOrCreateStatic(ip).GetOrCreateNextHop("0")
	ipv4Nh.NextHop, _ = ipv4Nh.To_NetworkInstance_Protocol_Static_NextHop_NextHop_Union(atePort3.IPv4)
	if val {
		gnmi.Update(t, dut, d.NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "DEFAULT").Config(), static)

	} else {
		gnmi.Delete(t, dut, d.NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "DEFAULT").Config())
	}
	ipv6nh := static.GetOrCreateStatic(ipv6EntryPrefix + "/32").GetOrCreateNextHop("0")
	ipv6nh.NextHop, _ = ipv4Nh.To_NetworkInstance_Protocol_Static_NextHop_NextHop_Union(atePort7.IPv6)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
}

func configureNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	c := &oc.Root{}
	vrfs := []string{vrfDecap, vrfRepair, vrfRepaired, vrfEncapA, vrfEncapB, vrfDecapPostRepaired, vrf1, vrf2, vrf3}
	for _, vrf := range vrfs {
		ni := c.GetOrCreateNetworkInstance(vrf)
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrf).Config(), ni)
	}

}

func TestWithTEmin(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with decapencap/decap, verify traffic, reprogram & delete ipv4/NHG/NH")

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()

	baseconfig(t)
	unconfigbasePBR(t, dut, "PBR", dut.Port(t, "port1").Name())
	configbasePBR(t, dut, "TE", "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{}, "PBR", dut.Port(t, "port1").Name(), false)
	defer unconfigbasePBR(t, dut, "PBR", dut.Port(t, "port1").Name())

	// Configure the gRIBI client
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

	args.ctx = ctx
	args.client = &client
	args.dut = dut
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	nh := 1
	for i := 1; i <= 15; i++ {
		args.client.AddNH(t, uint64(nh), atePort2.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}
	for i := 1; i <= 15; i++ {
		args.client.AddNH(t, uint64(nh), atePort4.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}
	wt := 1
	nh = 1
	var i, j uint64
	for i = 1; i <= 7; i++ {
		nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(i))

		for j = 1; j <= 3; j++ {
			if j == 1 {
				wt = 3
			} else if j == 2 {
				wt = 25
			} else {
				wt = 31
			}
			nhg.AddNextHop(uint64(nh), uint64(wt))
			args.client.Fluent(t).Modify().AddEntry(t, nhg)
			nh++
		}
	}
	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(8))
	nh = 22
	for j = 1; j <= 9; j++ {
		if j == 1 {
			wt = 8
		} else {
			wt = 31
		}
		nhg.AddNextHop(uint64(nh), uint64(wt))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nh++
	}
	dstPfx2 := "205.205.205.1"
	prefixes := []string{}
	for i := 0; i < 8; i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx2, i, mask))
	}
	nhgID := 1
	for _, prefix := range prefixes {
		args.client.AddIPv4(t, prefix, uint64(nhgID), *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		nhgID++
	}
	nh = 1001

	for _, prefix := range prefixes {
		b := strings.Split(prefix, "/")
		prefix = b[0]
		args.client.AddNH(t, uint64(nh), prefix, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}

	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(1000))

	for j = 1001; j <= 1008; j++ {
		if j == 1001 {
			wt = 7
		} else if j == 1002 {
			wt = 9
		} else {
			wt = 40
		}
		nhg.AddNextHop(j, uint64(wt))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
	}

	prefixess := []string{}
	for i := 0; i < 5000; i++ {
		prefixess = append(prefixess, util.GetIPPrefix(dstPfx, i, mask))
	}

	for _, prefix := range prefixess {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance("TE").
			WithPrefix(prefix).
			WithNextHopGroup(uint64(1000)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	}

	testTrafficWeight(t, args.ate, args.top, 65000, false, 1)
	args.interfaceaction(t, "port2", false)
	args.interfaceaction(t, "port4", false)
	t.Run("testTrafficaftr shut, no shut", func(t *testing.T) {
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		testTrafficmin(t, args.ate, args.top, 100, false, false, false)
		testTrafficWeight(t, args.ate, args.top, 65000, false, 1)
	})
	t.Run("testTraffic aftr rpfo", func(t *testing.T) {

		utils.Dorpfo(args.ctx, t, true)
		client = gribi.Client{
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
		testTrafficmin(t, args.ate, args.top, 100, false, false, false)
		testTrafficWeight(t, args.ate, args.top, 65000, false, 1)

	})
	t.Run("testTrafficaftr delete", func(t *testing.T) {

		args.client.DeleteIPv4Batch(t, prefixess, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		for _, prefix := range prefixess {
			ipv4Entry := fluent.IPv4Entry().
				WithNetworkInstance("TE").
				WithPrefix(prefix).
				WithNextHopGroup(uint64(1000)).
				WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
			args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
		}
		testTrafficmin(t, args.ate, args.top, 100, false, false, false)
		testTrafficWeight(t, args.ate, args.top, 65000, false, 1)
	})
}

func TestWithEncapvrfmin(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with decapencap/decap, verify traffic, reprogram & delete ipv4/NHG/NH")

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	baseconfig(t)
	unconfigbasePBR(t, dut, "PBR", dut.Port(t, "port1").Name())
	configbasePBR(t, dut, vrfEncapA, "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{}, "PBR", dut.Port(t, "port1").Name(), false)
	defer unconfigbasePBR(t, dut, "PBR", dut.Port(t, "port1").Name())

	// Configure the gRIBI client
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

	args.ctx = ctx
	args.client = &client
	args.dut = dut
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	nh := 1
	for i := 1; i <= 15; i++ {
		args.client.AddNH(t, uint64(nh), atePort2.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}
	for i := 1; i <= 15; i++ {
		args.client.AddNH(t, uint64(nh), atePort4.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}

	wt := 1
	nh = 1
	var i, j uint64
	for i = 1; i <= 7; i++ {
		nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(i))

		for j = 1; j <= 3; j++ {
			if j == 1 {
				wt = 3
			} else if j == 2 {
				wt = 25
			} else {
				wt = 31
			}
			nhg.AddNextHop(uint64(nh), uint64(wt))
			args.client.Fluent(t).Modify().AddEntry(t, nhg)
			nh++
		}
	}
	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(8))
	nh = 22
	for j = 1; j <= 9; j++ {
		if j < 2 {
			wt = 8
		} else {
			wt = 31
		}
		nhg.AddNextHop(uint64(nh), uint64(wt))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nh++
	}

	dstPfx2 := "205.205.205.1"
	prefixes := []string{}
	for i := 0; i < 8; i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx2, i, mask))
	}
	nhgID := 1
	i = 1
	for _, prefix := range prefixes {
		args.client.AddIPv4(t, prefix, uint64(nhgID), *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		nhgID++
	}
	nh = 1000
	wt = 1

	for _, prefix := range prefixes {
		b := strings.Split(prefix, "/")
		prefix = b[0]
		args.client.AddNH(t, uint64(nh), prefix, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}
	nhgi := 1000

	nh = 1000
	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(nhgi))

	for j = 1; j <= 3; j++ {
		if j <= 2 {
			wt = 1
		} else {
			wt = 254
		}
		nhg.AddNextHop(uint64(nh), uint64(wt))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nh++
	}
	nhgi = 1001
	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(nhgi))

	for j = 1; j <= 5; j++ {
		if j == 1 {
			wt = 4
		} else {
			wt = 63
		}
		nhg.AddNextHop(uint64(nh), uint64(wt))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nh++
	}

	prefixese := []string{}
	dstPfxe := "170.170.170.1"
	for i := 0; i < 2; i++ {
		prefixese = append(prefixese, util.GetIPPrefix(dstPfxe, i, mask))
	}

	nhgi = 1000
	i = 1
	for _, prefix := range prefixese {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance("TE").
			WithPrefix(prefix).
			WithNextHopGroup(uint64(nhgi)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
		nhgi++
	}

	nh = 3001
	for _, prefix := range prefixese {
		b := strings.Split(prefix, "/")
		prefix = b[0]
		args.client.AddNH(t, uint64(nh), "Encap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: []string{prefix}, VrfName: "TE"})
		nh++
	}

	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(3000))

	for j = 3001; j < 3003; j++ {
		if j == 3001 {
			wt = 15
		} else {
			wt = 241
		}
		nhg.AddNextHop(j, uint64(wt))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
	}

	prefixess := []string{}
	for i := 0; i < 15000; i++ {
		prefixess = append(prefixess, util.GetIPPrefix(dstPfx, i, mask))
	}

	for _, prefix := range prefixess {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance(vrfEncapA).
			WithPrefix(prefix).
			WithNextHopGroup(uint64(3000)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	}
	testTrafficWeight(t, args.ate, args.top, 65000, false, 2)
	args.interfaceaction(t, "port2", false)
	args.interfaceaction(t, "port4", false)
	t.Run("testTrafficaftr shut, no shut", func(t *testing.T) {
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		testTrafficmin(t, args.ate, args.top, 100, false, false, false)
		testTrafficWeight(t, args.ate, args.top, 65000, false, 2)
	})
	// t.Run("testTraffic aftr rpfo", func(t *testing.T) {

	// 	utils.Dorpfo(args.ctx, t, true)
	// 	client = gribi.Client{
	// 		DUT:                   args.dut,
	// 		FibACK:                *ciscoFlags.GRIBIFIBCheck,
	// 		Persistence:           true,
	// 		InitialElectionIDLow:  1,
	// 		InitialElectionIDHigh: 0,
	// 	}
	// 	if err := client.Start(t); err != nil {
	// 		t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
	// 		if err = client.Start(t); err != nil {
	// 			t.Fatalf("gRIBI Connection could not be established: %v", err)
	// 		}
	// 	}
	// 	args.client = &client
	// 	testTrafficmin(t, args.ate, args.top, 100, false, false, false)
	// 	testTrafficWeight(t, args.ate, args.top, 65000, false, 2)

	// })
	t.Run("testTrafficaftr delete", func(t *testing.T) {

		args.client.DeleteIPv4Batch(t, prefixess, 3000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		for _, prefix := range prefixess {
			ipv4Entry := fluent.IPv4Entry().
				WithNetworkInstance(vrfEncapA).
				WithPrefix(prefix).
				WithNextHopGroup(uint64(3000)).
				WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
			args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
		}
		testTrafficmin(t, args.ate, args.top, 100, false, false, false)
		testTrafficWeight(t, args.ate, args.top, 65000, false, 2)
	})

}

func TestWithDecapEncapTEmin(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with decapencap/decap, verify traffic, reprogram & delete ipv4/NHG/NH")

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	baseconfig(t)
	unconfigbasePBR(t, dut, "PBR", dut.Port(t, "port1").Name())
	configbasePBR(t, dut, vrfDecap, "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{}, "PBR", dut.Port(t, "port1").Name(), true)

	// Configure the gRIBI client
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

	args.ctx = ctx
	args.client = &client
	args.dut = dut
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	nh := 1
	for i := 1; i <= 15; i++ {
		args.client.AddNH(t, uint64(nh), atePort2.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}
	for i := 1; i <= 15; i++ {
		args.client.AddNH(t, uint64(nh), atePort4.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}

	wt := 1
	nh = 1
	var i, j uint64
	for i = 1; i <= 7; i++ {
		nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(i))

		for j = 1; j <= 3; j++ {
			if j == 1 {
				wt = 3
			} else if j == 2 {
				wt = 25
			} else {
				wt = 31
			}
			nhg.AddNextHop(uint64(nh), uint64(wt))
			args.client.Fluent(t).Modify().AddEntry(t, nhg)
			nh++
		}
	}
	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(8))
	nh = 22
	for j = 1; j <= 9; j++ {
		if j < 2 {
			wt = 8
		} else {
			wt = 31
		}
		nhg.AddNextHop(uint64(nh), uint64(wt))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nh++
	}

	dstPfx2 := "205.205.205.1"
	prefixes := []string{}
	for i := 0; i < 8; i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx2, i, mask))
	}
	nhgID := 1
	i = 1
	for _, prefix := range prefixes {
		args.client.AddIPv4(t, prefix, uint64(nhgID), *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		nhgID++
	}
	nh = 1000
	wt = 1

	for _, prefix := range prefixes {
		b := strings.Split(prefix, "/")
		prefix = b[0]
		args.client.AddNH(t, uint64(nh), prefix, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}
	nhgi := 1000

	nh = 1000
	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(nhgi))

	for j = 1; j <= 3; j++ {
		if j < 2 {
			wt = 52
		} else if j == 2 {
			wt = 51
		} else {
			wt = 153
		}
		nhg.AddNextHop(uint64(nh), uint64(wt))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nh++
	}
	nhgi = 1001
	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(nhgi))

	for j = 1; j <= 5; j++ {
		if j == 1 {
			wt = 132
		} else {
			wt = 31
		}
		nhg.AddNextHop(uint64(nh), uint64(wt))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nh++
	}

	prefixese := []string{}
	dstPfxe := "170.170.170.1"
	for i := 0; i < 2; i++ {
		prefixese = append(prefixese, util.GetIPPrefix(dstPfxe, i, mask))
	}

	nhgi = 1000
	i = 1
	for _, prefix := range prefixese {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance("TE").
			WithPrefix(prefix).
			WithNextHopGroup(uint64(nhgi)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
		nhgi++
	}

	nh = 3001
	for _, prefix := range prefixese {
		b := strings.Split(prefix, "/")
		prefix = b[0]
		args.client.AddNH(t, uint64(nh), "Encap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: []string{prefix}, VrfName: "TE"})
		nh++
	}

	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(3000))

	for j = 3001; j < 3003; j++ {
		if j == 3001 {
			wt = 15
		} else {
			wt = 241
		}
		nhg.AddNextHop(j, uint64(wt))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
	}
	dstPfxx := "197.51.100.1"
	prefixess := []string{}
	for i := 0; i < 15000; i++ {
		prefixess = append(prefixess, util.GetIPPrefix(dstPfxx, i, mask))
	}

	for _, prefix := range prefixess {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance(vrfEncapA).
			WithPrefix(prefix).
			WithNextHopGroup(uint64(3000)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	}

	nh = 4000
	args.client.AddNH(t, uint64(nh), "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{VrfName: vrfEncapA})
	args.client.AddNHG(t, 4000, 0, map[uint64]uint64{4000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixesd := []string{}
	for i := 0; i < 150; i++ {
		prefixesd = append(prefixesd, util.GetIPPrefix(dstPfx, i, mask))
	}

	for _, prefix := range prefixesd {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance(vrfDecap).
			WithPrefix(prefix).
			WithNextHopGroup(uint64(4000)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	}

	testTrafficWeight(t, args.ate, args.top, 65000, true, 3)
	t.Run("testTrafficaftr shut", func(t *testing.T) {
		testTrafficWeight(t, args.ate, args.top, 65000, true, 7)
	})

	t.Run("testTrafficaftr no shut", func(t *testing.T) {
		args.interfaceaction(t, "port2", true)
		testTrafficmin(t, args.ate, args.top, 100, true, false, false)
		testTrafficWeight(t, args.ate, args.top, 65000, true, 3)
	})
	t.Run("testTraffic aftr rpfo", func(t *testing.T) {

		utils.Dorpfo(args.ctx, t, true)
		client = gribi.Client{
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
		testTrafficmin(t, args.ate, args.top, 100, true, false, false)
		testTrafficWeight(t, args.ate, args.top, 65000, true, 3)

	})
	t.Run("testTrafficaftr delete", func(t *testing.T) {

		args.client.DeleteIPv4Batch(t, prefixesd, 4000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		for _, prefix := range prefixesd {
			ipv4Entry := fluent.IPv4Entry().
				WithNetworkInstance(vrfDecap).
				WithPrefix(prefix).
				WithNextHopGroup(uint64(4000)).
				WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
			args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
		}
		testTrafficmin(t, args.ate, args.top, 100, true, false, false)
		testTrafficWeight(t, args.ate, args.top, 65000, true, 3)
	})

}

func TestWithDecapEncapTEBackUpmin(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with decapencap/decap, verify traffic, reprogram & delete ipv4/NHG/NH")

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	baseconfig(t)
	addStaticRoute(t, dut, "197.51.0.0/16", true)
	unconfigbasePBR(t, dut, "PBR", dut.Port(t, "port1").Name())
	configbasePBR(t, dut, vrfDecap, "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{}, "PBR", dut.Port(t, "port1").Name(), true)

	args.top.StopProtocols(t)
	time.Sleep(30 * time.Second)
	args.top.StartProtocols(t)

	// Configure the gRIBI client
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

	args.ctx = ctx
	args.client = &client
	args.dut = dut
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	nh := 1
	for i := 1; i <= 15; i++ {
		args.client.AddNH(t, uint64(nh), atePort2.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}
	for i := 1; i <= 15; i++ {
		args.client.AddNH(t, uint64(nh), atePort4.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}

	wt := 1
	nh = 1
	var i, j uint64
	for i = 1; i <= 7; i++ {
		nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(i))

		for j = 1; j <= 3; j++ {
			if j == 1 {
				wt = 3
			} else if j == 2 {
				wt = 25
			} else {
				wt = 31
			}
			nhg.AddNextHop(uint64(nh), uint64(wt))
			args.client.Fluent(t).Modify().AddEntry(t, nhg)
			nh++
		}
	}
	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(8))
	nh = 22
	for j = 1; j <= 9; j++ {
		if j < 2 {
			wt = 8
		} else {
			wt = 31
		}
		nhg.AddNextHop(uint64(nh), uint64(wt))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nh++
	}

	dstPfx2 := "205.205.205.1"
	prefixes := []string{}
	for i := 0; i < 8; i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx2, i, mask))
	}
	nhgID := 1
	i = 1
	for _, prefix := range prefixes {
		args.client.AddIPv4(t, prefix, uint64(nhgID), *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		nhgID++
	}
	nh = 1000
	wt = 1

	for _, prefix := range prefixes {
		b := strings.Split(prefix, "/")
		prefix = b[0]
		args.client.AddNH(t, uint64(nh), prefix, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}
	nhgi := 1000

	nh = 1000
	dstPfxt := "200.200.200.1"
	prefixest := []string{}
	for i := 0; i < 2; i++ {
		prefixest = append(prefixest, util.GetIPPrefix(dstPfxt, i, mask))
	}
	NHEntry := fluent.NextHopEntry()
	nhge := 20000
	for _, prefix := range prefixest {
		b := strings.Split(prefix, "/")
		prefix = b[0]
		nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(nhge))
		NHEntry = NHEntry.WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithIndex(uint64(nhge))
		NHEntry = NHEntry.WithDecapsulateHeader(fluent.IPinIP)
		NHEntry = NHEntry.WithEncapsulateHeader(fluent.IPinIP)
		NHEntry = NHEntry.WithIPinIP("222.222.222.222", prefix)
		NHEntry = NHEntry.WithNextHopNetworkInstance("REPAIRED")
		args.client.Fluent(t).Modify().AddEntry(t, NHEntry)
		nhg.AddNextHop(uint64(nhge), uint64(256))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nhge++
	}
	nh = 1000
	nhge = 20000
	nhgi = 1000
	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(nhgi))

	for j = 1; j <= 3; j++ {
		if j == 1 {
			wt = 52
		} else if j == 2 {
			wt = 51
		} else {
			wt = 153
		}
		nhg.AddNextHop(uint64(nh), uint64(wt))
		nhg.WithBackupNHG(uint64(nhge))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nh++
	}
	nhgi = 1001
	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(nhgi))

	for j = 1; j <= 5; j++ {
		if j == 1 {
			wt = 132
		} else {
			wt = 31
		}
		nhg.AddNextHop(uint64(nh), uint64(wt))
		nhg.WithBackupNHG(uint64(nhge + 1))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nh++
	}

	prefixese := []string{}
	dstPfxe := "170.170.170.1"
	for i := 0; i < 2; i++ {
		prefixese = append(prefixese, util.GetIPPrefix(dstPfxe, i, mask))
	}

	nhgi = 1000
	for _, prefix := range prefixese {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance("TE").
			WithPrefix(prefix).
			WithNextHopGroup(uint64(nhgi)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
		nhgi++
	}

	args.client.AddNH(t, 30000, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 30000, 0, map[uint64]uint64{30000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, uint64(30001), atePort5.ip(uint8(1)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, uint64(30002), atePort5.ip(uint8(2)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 31000, 30000, map[uint64]uint64{uint64(30001): 1, uint64(30002): 255}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixest, 31000, "REPAIRED", *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	nh = 3001
	for _, prefix := range prefixese {
		b := strings.Split(prefix, "/")
		prefix = b[0]
		args.client.AddNH(t, uint64(nh), "Encap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: []string{prefix}, VrfName: "TE"})
		nh++
	}

	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(3000))

	for j = 3001; j < 3003; j++ {
		if j == 3001 {
			wt = 15
		} else {
			wt = 241
		}
		nhg.AddNextHop(j, uint64(wt))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
	}

	dstPfxe = "197.51.100.1"

	prefixess := []string{}
	for i := 0; i < 15000; i++ {
		prefixess = append(prefixess, util.GetIPPrefix(dstPfxe, i, mask))
	}

	for _, prefix := range prefixess {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance(vrfEncapA).
			WithPrefix(prefix).
			WithNextHopGroup(uint64(3000)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	}
	args.client.AddIPv6(t, ipv6EntryPrefix+"/32", uint64(3000), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	nh = 4000

	args.client.AddNH(t, uint64(nh), "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{VrfName: vrfEncapA})
	args.client.AddNHG(t, 4000, 0, map[uint64]uint64{4000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	prefixesd := []string{}
	for i := 0; i < 100; i++ {
		prefixesd = append(prefixesd, util.GetIPPrefix(dstPfx, i, mask))
	}

	for _, prefix := range prefixesd {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance(vrfDecap).
			WithPrefix(prefix).
			WithNextHopGroup(uint64(4000)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	}

	testTrafficWeight(t, args.ate, args.top, 65000, true, 3)

	t.Run("testTrafficaftr frrs", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 100, true, true, false)
		testTrafficWeight(t, args.ate, args.top, 65000, true, 8)

		testTrafficmin(t, args.ate, args.top, 100, true, false, true)
	})

	t.Run("testTrafficaftr no shut", func(t *testing.T) {
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

		testTrafficmin(t, args.ate, args.top, 100, true, false, false)
		testTrafficWeight(t, args.ate, args.top, 65000, true, 3)
	})
	// t.Run("testTraffic aftr rpfo", func(t *testing.T) {

	// 	utils.Dorpfo(args.ctx, t, true)
	// 	client = gribi.Client{
	// 		DUT:                   args.dut,
	// 		FibACK:                *ciscoFlags.GRIBIFIBCheck,
	// 		Persistence:           true,
	// 		InitialElectionIDLow:  1,
	// 		InitialElectionIDHigh: 0,
	// 	}
	// 	if err := client.Start(t); err != nil {
	// 		t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
	// 		if err = client.Start(t); err != nil {
	// 			t.Fatalf("gRIBI Connection could not be established: %v", err)
	// 		}
	// 	}
	// 	args.client = &client
	// 	testTrafficmin(t, args.ate, args.top, 100, true, false, false)
	// 	testTrafficWeight(t, args.ate, args.top, 65000, true, 3)

	// })
	t.Run("testTrafficaftr delete", func(t *testing.T) {

		args.client.DeleteIPv4Batch(t, prefixesd, 4000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		for _, prefix := range prefixesd {
			ipv4Entry := fluent.IPv4Entry().
				WithNetworkInstance(vrfDecap).
				WithPrefix(prefix).
				WithNextHopGroup(uint64(4000)).
				WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
			args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
		}
		testTrafficmin(t, args.ate, args.top, 100, true, false, false)
		testTrafficWeight(t, args.ate, args.top, 65000, true, 3)
	})
}

func TestWithDecapEncapTEUnoptimizedmin(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with decapencap/decap, verify traffic, reprogram & delete ipv4/NHG/NH")

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	baseconfig(t)
	args.top.StopProtocols(t)
	time.Sleep(30 * time.Second)
	args.top.StartProtocols(t)

	// Configure the gRIBI client
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

	args.ctx = ctx
	args.client = &client
	args.dut = dut
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	nh := 1
	for i := 1; i <= 15; i++ {
		args.client.AddNH(t, uint64(nh), atePort2.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}
	for i := 1; i <= 15; i++ {
		args.client.AddNH(t, uint64(nh), atePort4.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}

	wt := 1
	nh = 1
	var i, j uint64
	for i = 1; i <= 7; i++ {
		nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(i))

		for j = 1; j <= 3; j++ {
			if j == 1 {
				wt = 3
			} else if j == 2 {
				wt = 25
			} else {
				wt = 31
			}
			nhg.AddNextHop(uint64(nh), uint64(wt))
			args.client.Fluent(t).Modify().AddEntry(t, nhg)
			nh++
		}
	}
	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(8))
	nh = 22
	for j = 1; j <= 9; j++ {
		if j < 2 {
			wt = 8
		} else {
			wt = 31
		}
		nhg.AddNextHop(uint64(nh), uint64(wt))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nh++
	}

	dstPfx2 := "205.205.205.1"
	prefixes := []string{}
	for i := 0; i < 8; i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx2, i, mask))
	}
	nhgID := 1
	i = 1
	for _, prefix := range prefixes {
		args.client.AddIPv4(t, prefix, uint64(nhgID), *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		nhgID++
	}
	nh = 1000
	wt = 1

	for _, prefix := range prefixes {
		b := strings.Split(prefix, "/")
		prefix = b[0]
		args.client.AddNH(t, uint64(nh), prefix, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}
	nhgi := 1000

	nh = 1000
	NHEntry := fluent.NextHopEntry()
	nhge := 20000
	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID((uint64(nhge)))
	NHEntry = NHEntry.WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithIndex(uint64(nhge))
	NHEntry = NHEntry.WithNextHopNetworkInstance("REPAIR")
	args.client.Fluent(t).Modify().AddEntry(t, NHEntry)
	nhg.AddNextHop(uint64(nhge), uint64(256))
	args.client.Fluent(t).Modify().AddEntry(t, nhg)
	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(nhgi))

	for j = 1; j <= 3; j++ {
		if j == 1 {
			wt = 101
		} else if j == 2 {
			wt = 52
		} else {
			wt = 83
		}
		nhg.AddNextHop(uint64(nh), uint64(wt))
		nhg.WithBackupNHG(uint64(nhge))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nh++
	}
	nhgi = 1001
	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(nhgi))

	for j = 1; j <= 5; j++ {
		if j == 1 {
			wt = 132
		} else {
			wt = 31
		}
		nhg.AddNextHop(uint64(nh), uint64(wt))
		nhg.WithBackupNHG(uint64(nhge))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nh++
	}
	nhgi = 1000

	prefixese := []string{}
	dstPfxe := "170.170.170.1"
	for i := 0; i < 2; i++ {
		prefixese = append(prefixese, util.GetIPPrefix(dstPfxe, i, mask))
	}

	nhgi = 1000
	for _, prefix := range prefixese {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance("TE").
			WithPrefix(prefix).
			WithNextHopGroup(uint64(nhgi)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
		nhgi++
	}

	args.client.AddNH(t, 30000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 30000, 0, map[uint64]uint64{30000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, uint64(30001), atePort5.ip(uint8(1)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, uint64(30002), atePort5.ip(uint8(2)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 31000, 0, map[uint64]uint64{uint64(30001): 1, uint64(30002): 255}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, dsip+"/32", 31000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 31000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: "222.222.222.222", Dest: []string{dsip}})
	args.client.AddNHG(t, 32000, 30000, map[uint64]uint64{31000: 256}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixese, 32000, "REPAIR", *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	nh = 3001
	for _, prefix := range prefixese {
		b := strings.Split(prefix, "/")
		prefix = b[0]
		args.client.AddNH(t, uint64(nh), "Encap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: []string{prefix}, VrfName: "TE"})
		nh++
	}

	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(3000))

	for j = 3001; j < 3003; j++ {
		if j == 3001 {
			wt = 15
		} else {
			wt = 241
		}
		nhg.AddNextHop(j, uint64(wt))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
	}

	dstPfxe = "197.51.100.1"
	prefixess := []string{}
	for i := 0; i < 15000; i++ {
		prefixess = append(prefixess, util.GetIPPrefix(dstPfxe, i, mask))
	}

	for _, prefix := range prefixess {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance(vrfEncapA).
			WithPrefix(prefix).
			WithNextHopGroup(uint64(3000)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	}
	args.client.AddIPv6(t, ipv6EntryPrefix+"/32", uint64(3000), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	nh = 4000
	args.client.AddNH(t, uint64(nh), "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{VrfName: vrfEncapA})
	args.client.AddNHG(t, 4000, 0, map[uint64]uint64{4000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixesd := []string{}
	for i := 0; i < 150; i++ {
		prefixesd = append(prefixesd, util.GetIPPrefix(dstPfx, i, mask))
	}

	for _, prefix := range prefixesd {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance(vrfDecap).
			WithPrefix(prefix).
			WithNextHopGroup(uint64(4000)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	}

	testTrafficWeight(t, args.ate, args.top, 65000, true, 5)
	t.Run("test traffic after frrs", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 100, true, true, false)
		testTrafficWeight(t, args.ate, args.top, 65000, true, 8)

		testTrafficmin(t, args.ate, args.top, 100, true, false, true)
	})
	t.Run("testTrafficaftr no shut", func(t *testing.T) {
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

		testTrafficmin(t, args.ate, args.top, 100, true, false, false)
		testTrafficWeight(t, args.ate, args.top, 65000, true, 5)
	})
	t.Run("testTraffic aftr rpfo", func(t *testing.T) {

		utils.Dorpfo(args.ctx, t, true)
		client = gribi.Client{
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
		testTrafficmin(t, args.ate, args.top, 100, true, false, false)
		testTrafficWeight(t, args.ate, args.top, 65000, true, 5)

	})
	t.Run("testTrafficaftr delete", func(t *testing.T) {

		args.client.DeleteIPv4Batch(t, prefixesd, 4000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		for _, prefix := range prefixesd {
			ipv4Entry := fluent.IPv4Entry().
				WithNetworkInstance(vrfDecap).
				WithPrefix(prefix).
				WithNextHopGroup(uint64(4000)).
				WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
			args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
		}
		testTrafficmin(t, args.ate, args.top, 100, true, false, false)
		testTrafficWeight(t, args.ate, args.top, 65000, true, 5)
	})
}

func TestRepairedDecapmin(t *testing.T) {

	t.Logf("Program gribi entries with decapencap/decap, verify traffic, reprogram & delete ipv4/NHG/NH")

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	baseconfig(t)
	unconfigbasePBR(t, dut, "PBR", dut.Port(t, "port1").Name())

	configbasePBR(t, dut, "REPAIRED", "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{}, "PBR", dut.Port(t, "port1").Name(), false)
	defer unconfigbasePBR(t, dut, "PBR", dut.Port(t, "port1").Name())

	// Configure the gRIBI client
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

	args.ctx = ctx
	args.client = &client
	args.dut = dut
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)
	addStaticRoute(t, dut, "202.1.0.0/16", true)

	nh := 1
	for i := 1; i <= 15; i++ {
		args.client.AddNH(t, uint64(nh), atePort2.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}
	for i := 1; i <= 15; i++ {
		args.client.AddNH(t, uint64(nh), atePort4.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}
	wt := 1
	nh = 1
	var i, j uint64
	for i = 1; i <= 7; i++ {
		nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(i))

		for j = 1; j <= 3; j++ {
			if j == 1 {
				wt = 3
			} else if j == 2 {
				wt = 25
			} else {
				wt = 31
			}
			nhg.AddNextHop(uint64(nh), uint64(wt))
			args.client.Fluent(t).Modify().AddEntry(t, nhg)
			nh++
		}
	}
	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(8))
	nh = 22
	for j = 1; j <= 9; j++ {
		if j == 1 {
			wt = 8
		} else {
			wt = 31
		}
		nhg.AddNextHop(uint64(nh), uint64(wt))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nh++
	}
	dstPfx2 := "205.205.205.1"
	prefixes := []string{}
	for i := 0; i < 8; i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx2, i, mask))
	}
	nhgID := 1
	for _, prefix := range prefixes {
		args.client.AddIPv4(t, prefix, uint64(nhgID), *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		nhgID++
	}
	nh = 1001

	for _, prefix := range prefixes {
		b := strings.Split(prefix, "/")
		prefix = b[0]
		args.client.AddNH(t, uint64(nh), prefix, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}

	args.client.AddNH(t, 20000, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 20000, 0, map[uint64]uint64{20000: 20}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	nhge := 20000
	nhgi := 1000
	nh = 1001

	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(nhgi))

	for j = 1; j <= 8; j++ {
		if j < 2 {
			wt = 9
		} else if j == 2 {
			wt = 7
		} else {
			wt = 40
		}
		nhg.AddNextHop(uint64(nh), uint64(wt))
		nhg.WithBackupNHG(uint64(nhge))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nh++
	}

	nhgi = 1000

	prefixesr := []string{}
	for i := 0; i < 5000; i++ {
		prefixesr = append(prefixesr, util.GetIPPrefix(dstPfx, i, mask))
	}

	for _, prefix := range prefixesr {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance("REPAIRED").
			WithPrefix(prefix).
			WithNextHopGroup(uint64(nhgi)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	}

	testTrafficWeight(t, args.ate, args.top, 65000, false, 6)
	t.Run("testTrafficaftr frr", func(t *testing.T) {
		testTrafficmin(t, args.ate, args.top, 100, false, true, true)
	})

	t.Run("testTrafficaftr no shut", func(t *testing.T) {
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)

		testTrafficmin(t, args.ate, args.top, 100, false, false, false)
		testTrafficWeight(t, args.ate, args.top, 65000, false, 6)
	})
	t.Run("testTraffic aftr rpfo", func(t *testing.T) {

		utils.Dorpfo(args.ctx, t, true)
		client = gribi.Client{
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
		testTrafficmin(t, args.ate, args.top, 100, false, false, false)
		testTrafficWeight(t, args.ate, args.top, 65000, false, 6)

	})
	t.Run("testTrafficaftr delete", func(t *testing.T) {

		args.client.DeleteIPv4Batch(t, prefixesr, 4000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		for _, prefix := range prefixesr {
			ipv4Entry := fluent.IPv4Entry().
				WithNetworkInstance("REPAIRED").
				WithPrefix(prefix).
				WithNextHopGroup(uint64(4000)).
				WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
			args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
		}
		testTrafficmin(t, args.ate, args.top, 100, true, false, false)
		testTrafficWeight(t, args.ate, args.top, 65000, true, 6)
	})

}

func TestWithDecapEncapTEBackUpminipv6(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with decapencap/decap, verify traffic, reprogram & delete ipv4/NHG/NH")

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	baseconfig(t)
	unconfigbasePBR(t, dut, "PBR", dut.Port(t, "port1").Name())
	configbasePBR(t, dut, vrfDecap, "ipv6", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{}, "PBR", dut.Port(t, "port1").Name(), true)
	args.top.StopProtocols(t)
	time.Sleep(30 * time.Second)
	args.top.StartProtocols(t)

	// Configure the gRIBI client
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

	args.ctx = ctx
	args.client = &client
	args.dut = dut
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	nh := 1
	for i := 1; i <= 15; i++ {
		args.client.AddNH(t, uint64(nh), atePort2.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}
	for i := 1; i <= 15; i++ {
		args.client.AddNH(t, uint64(nh), atePort4.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}

	wt := 1
	nh = 1
	var i, j uint64
	for i = 1; i <= 7; i++ {
		nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(i))

		for j = 1; j <= 3; j++ {
			if j == 1 {
				wt = 3
			} else if j == 2 {
				wt = 25
			} else {
				wt = 31
			}
			nhg.AddNextHop(uint64(nh), uint64(wt))
			args.client.Fluent(t).Modify().AddEntry(t, nhg)
			nh++
		}
	}
	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(8))
	nh = 22
	for j = 1; j <= 9; j++ {
		if j < 2 {
			wt = 8
		} else {
			wt = 31
		}
		nhg.AddNextHop(uint64(nh), uint64(wt))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nh++
	}

	dstPfx2 := "205.205.205.1"
	prefixes := []string{}
	for i := 0; i < 8; i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx2, i, mask))
	}
	nhgID := 1
	i = 1
	for _, prefix := range prefixes {
		args.client.AddIPv4(t, prefix, uint64(nhgID), *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		nhgID++
	}
	nh = 1000
	wt = 1

	for _, prefix := range prefixes {
		b := strings.Split(prefix, "/")
		prefix = b[0]
		args.client.AddNH(t, uint64(nh), prefix, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}
	nhgi := 1000

	nh = 1000
	dstPfxt := "200.200.200.1"
	prefixest := []string{}
	for i := 0; i < 2; i++ {
		prefixest = append(prefixest, util.GetIPPrefix(dstPfxt, i, mask))
	}
	NHEntry := fluent.NextHopEntry()
	nhge := 20000
	for _, prefix := range prefixest {
		b := strings.Split(prefix, "/")
		prefix = b[0]
		nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(nhge))
		NHEntry = NHEntry.WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithIndex(uint64(nhge))
		NHEntry = NHEntry.WithDecapsulateHeader(fluent.IPinIP)
		NHEntry = NHEntry.WithEncapsulateHeader(fluent.IPinIP)
		NHEntry = NHEntry.WithIPinIP("222.222.222.222", prefix)
		NHEntry = NHEntry.WithNextHopNetworkInstance("REPAIRED")
		args.client.Fluent(t).Modify().AddEntry(t, NHEntry)
		nhg.AddNextHop(uint64(nhge), uint64(256))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nhge++
	}
	nh = 1000
	nhge = 20000
	nhgi = 1000
	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(nhgi))

	for j = 1; j <= 3; j++ {
		if j == 1 {
			wt = 52
		} else if j == 2 {
			wt = 51
		} else {
			wt = 153
		}
		nhg.AddNextHop(uint64(nh), uint64(wt))
		nhg.WithBackupNHG(uint64(nhge))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nh++
	}
	nhgi = 1001
	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(nhgi))

	for j = 1; j <= 5; j++ {
		if j == 1 {
			wt = 132
		} else {
			wt = 31
		}
		nhg.AddNextHop(uint64(nh), uint64(wt))
		nhg.WithBackupNHG(uint64(nhge + 1))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nh++
	}
	prefixese := []string{}
	dstPfxe := "170.170.170.1"
	for i := 0; i < 2; i++ {
		prefixese = append(prefixese, util.GetIPPrefix(dstPfxe, i, mask))
	}

	nhgi = 1000
	for _, prefix := range prefixese {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance("TE").
			WithPrefix(prefix).
			WithNextHopGroup(uint64(nhgi)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
		nhgi++
	}

	args.client.AddNH(t, 30000, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 30000, 0, map[uint64]uint64{30000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, uint64(30001), atePort5.ip(uint8(1)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, uint64(30002), atePort5.ip(uint8(2)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 31000, 30000, map[uint64]uint64{uint64(30001): 1, uint64(30002): 255}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixest, 31000, "REPAIRED", *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	nh = 3001
	for _, prefix := range prefixese {
		b := strings.Split(prefix, "/")
		prefix = b[0]
		args.client.AddNH(t, uint64(nh), "Encap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: []string{prefix}, VrfName: "TE"})
		nh++
	}

	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(3000))

	for j = 3001; j < 3003; j++ {
		if j == 3001 {
			wt = 15
		} else {
			wt = 241
		}
		nhg.AddNextHop(j, uint64(wt))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
	}

	dstPfxe = "197.51.100.1"

	prefixess := []string{}
	for i := 0; i < 15000; i++ {
		prefixess = append(prefixess, util.GetIPPrefix(dstPfxe, i, mask))
	}

	for _, prefix := range prefixess {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance(vrfEncapA).
			WithPrefix(prefix).
			WithNextHopGroup(uint64(3000)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	}
	args.client.AddIPv6(t, ipv6EntryPrefix+"/32", uint64(3000), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	nh = 4000
	args.client.AddNH(t, uint64(nh), "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{VrfName: vrfEncapA})
	args.client.AddNHG(t, 4000, 0, map[uint64]uint64{4000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixesd := []string{}
	for i := 0; i < 150; i++ {
		prefixesd = append(prefixesd, util.GetIPPrefix(dstPfx, i, mask))
	}

	for _, prefix := range prefixesd {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance(vrfDecap).
			WithPrefix(prefix).
			WithNextHopGroup(uint64(4000)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	}

	testTrafficWeight(t, args.ate, args.top, 2, true, 3)
	t.Run("testTrafficaftr frrs", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 2, true, true, false)
		testTrafficWeight(t, args.ate, args.top, 2, true, 8)

		testTrafficmin(t, args.ate, args.top, 2, true, false, true)
	})

	t.Run("testTrafficaftr no shut", func(t *testing.T) {
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

		testTrafficmin(t, args.ate, args.top, 2, true, false, false)
		testTrafficWeight(t, args.ate, args.top, 2, true, 3)
	})
	// t.Run("testTraffic aftr rpfo", func(t *testing.T) {

	// 	utils.Dorpfo(args.ctx, t, true)
	// 	client = gribi.Client{
	// 		DUT:                   args.dut,
	// 		FibACK:                *ciscoFlags.GRIBIFIBCheck,
	// 		Persistence:           true,
	// 		InitialElectionIDLow:  1,
	// 		InitialElectionIDHigh: 0,
	// 	}
	// 	if err := client.Start(t); err != nil {
	// 		t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
	// 		if err = client.Start(t); err != nil {
	// 			t.Fatalf("gRIBI Connection could not be established: %v", err)
	// 		}
	// 	}
	// 	args.client = &client
	// 	testTrafficmin(t, args.ate, args.top, 2, true, false, false)
	// 	testTrafficWeight(t, args.ate, args.top, 2, true, 3)

	// })
}

func TestWithDecapEncapTEUnoptimizedminipv6(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with decapencap/decap, verify traffic, reprogram & delete ipv4/NHG/NH")

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	baseconfig(t)
	args.top.StopProtocols(t)
	time.Sleep(30 * time.Second)
	args.top.StartProtocols(t)
	// Configure the gRIBI client
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

	args.ctx = ctx
	args.client = &client
	args.dut = dut
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	nh := 1
	for i := 1; i <= 15; i++ {
		args.client.AddNH(t, uint64(nh), atePort2.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}
	for i := 1; i <= 15; i++ {
		args.client.AddNH(t, uint64(nh), atePort4.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}

	wt := 1
	nh = 1
	var i, j uint64
	for i = 1; i <= 7; i++ {
		nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(i))

		for j = 1; j <= 3; j++ {
			if j == 1 {
				wt = 3
			} else if j == 2 {
				wt = 25
			} else {
				wt = 31
			}
			nhg.AddNextHop(uint64(nh), uint64(wt))
			args.client.Fluent(t).Modify().AddEntry(t, nhg)
			nh++
		}
	}
	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(8))
	nh = 22
	for j = 1; j <= 9; j++ {
		if j < 2 {
			wt = 8
		} else {
			wt = 31
		}
		nhg.AddNextHop(uint64(nh), uint64(wt))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nh++
	}

	dstPfx2 := "205.205.205.1"
	prefixes := []string{}
	for i := 0; i < 8; i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx2, i, mask))
	}
	nhgID := 1
	i = 1
	for _, prefix := range prefixes {
		args.client.AddIPv4(t, prefix, uint64(nhgID), *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		nhgID++
	}
	nh = 1000
	wt = 1
	//L1

	for _, prefix := range prefixes {
		b := strings.Split(prefix, "/")
		prefix = b[0]
		args.client.AddNH(t, uint64(nh), prefix, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}
	nhgi := 1000

	nh = 1000

	NHEntry := fluent.NextHopEntry()
	nhge := 20000
	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID((uint64(nhge)))
	NHEntry = NHEntry.WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithIndex(uint64(nhge))
	NHEntry = NHEntry.WithNextHopNetworkInstance("REPAIR")
	args.client.Fluent(t).Modify().AddEntry(t, NHEntry)
	nhg.AddNextHop(uint64(nhge), uint64(256))
	args.client.Fluent(t).Modify().AddEntry(t, nhg)
	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(nhgi))

	for j = 1; j <= 3; j++ {
		if j == 1 {
			wt = 101
		} else if j == 2 {
			wt = 52
		} else {
			wt = 83
		}
		nhg.AddNextHop(uint64(nh), uint64(wt))
		nhg.WithBackupNHG(uint64(nhge))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nh++
	}
	nhgi = 1001
	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(nhgi))

	for j = 1; j <= 5; j++ {
		if j == 1 {
			wt = 132
		} else {
			wt = 31
		}
		nhg.AddNextHop(uint64(nh), uint64(wt))
		nhg.WithBackupNHG(uint64(nhge))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nh++
	}
	nhgi = 1000

	prefixese := []string{}
	dstPfxe := "170.170.170.1"
	for i := 0; i < 2; i++ {
		prefixese = append(prefixese, util.GetIPPrefix(dstPfxe, i, mask))
	}

	nhgi = 1000
	for _, prefix := range prefixese {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance("TE").
			WithPrefix(prefix).
			WithNextHopGroup(uint64(nhgi)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
		nhgi++
	}

	args.client.AddNH(t, 30000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 30000, 0, map[uint64]uint64{30000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, uint64(30001), atePort5.ip(uint8(1)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, uint64(30002), atePort5.ip(uint8(2)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 31000, 0, map[uint64]uint64{uint64(30001): 1, uint64(30002): 255}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, dsip+"/32", 31000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 31000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: "222.222.222.222", Dest: []string{dsip}})
	args.client.AddNHG(t, 32000, 30000, map[uint64]uint64{31000: 256}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixese, 32000, "REPAIR", *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	nh = 3001
	for _, prefix := range prefixese {
		b := strings.Split(prefix, "/")
		prefix = b[0]
		args.client.AddNH(t, uint64(nh), "Encap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: []string{prefix}, VrfName: "TE"})
		nh++
	}

	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(3000))

	for j = 3001; j < 3003; j++ {
		if j == 3001 {
			wt = 15
		} else {
			wt = 241
		}
		nhg.AddNextHop(j, uint64(wt))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
	}

	dstPfxe = "197.51.100.1"
	prefixess := []string{}
	for i := 0; i < 15000; i++ {
		prefixess = append(prefixess, util.GetIPPrefix(dstPfxe, i, mask))
	}

	for _, prefix := range prefixess {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance(vrfEncapA).
			WithPrefix(prefix).
			WithNextHopGroup(uint64(3000)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	}
	args.client.AddIPv6(t, ipv6EntryPrefix+"/32", uint64(3000), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	nh = 4000
	args.client.AddNH(t, uint64(nh), "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{VrfName: vrfEncapA})
	args.client.AddNHG(t, 4000, 0, map[uint64]uint64{4000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixesd := []string{}
	for i := 0; i < 150; i++ {
		prefixesd = append(prefixesd, util.GetIPPrefix(dstPfx, i, mask))
	}

	for _, prefix := range prefixesd {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance(vrfDecap).
			WithPrefix(prefix).
			WithNextHopGroup(uint64(4000)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	}
	testTrafficWeight(t, args.ate, args.top, 2, true, 5)
	t.Run("testTrafficaftr frrs", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 2, true, true, false)
		testTrafficWeight(t, args.ate, args.top, 2, true, 8)

		testTrafficmin(t, args.ate, args.top, 2, true, false, true)
	})
	t.Run("testTrafficaftr no shut", func(t *testing.T) {
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

		testTrafficmin(t, args.ate, args.top, 2, true, false, false)
		testTrafficWeight(t, args.ate, args.top, 2, true, 5)
	})
	// t.Run("testTraffic aftr rpfo", func(t *testing.T) {

	// 	utils.Dorpfo(args.ctx, t, true)
	// 	client = gribi.Client{
	// 		DUT:                   args.dut,
	// 		FibACK:                *ciscoFlags.GRIBIFIBCheck,
	// 		Persistence:           true,
	// 		InitialElectionIDLow:  1,
	// 		InitialElectionIDHigh: 0,
	// 	}
	// 	if err := client.Start(t); err != nil {
	// 		t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
	// 		if err = client.Start(t); err != nil {
	// 			t.Fatalf("gRIBI Connection could not be established: %v", err)
	// 		}
	// 	}
	// 	args.client = &client
	// 	testTrafficmin(t, args.ate, args.top, 2, true, false, false)
	// 	testTrafficWeight(t, args.ate, args.top, 2, true, 5)

	// })
}

func TestWithDecapEncapTEBackUpminpop(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with decapencap/decap, verify traffic, reprogram & delete ipv4/NHG/NH")

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	baseconfig(t)
	addStaticRoute(t, dut, "202.1.0.0/16", true)
	unconfigbasePBR(t, dut, "PBR", dut.Port(t, "port1").Name())
	configbasePBR(t, dut, "TE", "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{}, "PBR", dut.Port(t, "port1").Name(), false)

	// Configure the gRIBI client
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

	args.ctx = ctx
	args.client = &client
	args.dut = dut
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	nh := 1
	for i := 1; i <= 15; i++ {
		args.client.AddNH(t, uint64(nh), atePort2.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}
	for i := 1; i <= 15; i++ {
		args.client.AddNH(t, uint64(nh), atePort4.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}

	wt := 1
	nh = 1
	var i, j uint64
	for i = 1; i <= 7; i++ {
		nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(i))

		for j = 1; j <= 3; j++ {
			if j == 1 {
				wt = 3
			} else if j == 2 {
				wt = 25
			} else {
				wt = 31
			}
			nhg.AddNextHop(uint64(nh), uint64(wt))
			args.client.Fluent(t).Modify().AddEntry(t, nhg)
			nh++
		}
	}
	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(8))
	nh = 22
	for j = 1; j <= 9; j++ {
		if j < 2 {
			wt = 8
		} else {
			wt = 31
		}
		nhg.AddNextHop(uint64(nh), uint64(wt))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nh++
	}

	dstPfx2 := "205.205.205.1"
	prefixes := []string{}
	for i := 0; i < 8; i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx2, i, mask))
	}
	nhgID := 1
	i = 1
	for _, prefix := range prefixes {
		args.client.AddIPv4(t, prefix, uint64(nhgID), *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		nhgID++
	}
	nh = 1000
	wt = 1

	for _, prefix := range prefixes {
		b := strings.Split(prefix, "/")
		prefix = b[0]
		args.client.AddNH(t, uint64(nh), prefix, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}
	nhgi := 1000
	nh = 1000
	dstPfxt := "200.200.200.1"
	prefixest := []string{}
	for i := 0; i < 1; i++ {
		prefixest = append(prefixest, util.GetIPPrefix(dstPfxt, i, mask))
	}
	NHEntry := fluent.NextHopEntry()
	nhge := 20000
	for _, prefix := range prefixest {
		b := strings.Split(prefix, "/")
		prefix = b[0]
		nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(nhge))
		NHEntry = NHEntry.WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithIndex(uint64(nhge))
		NHEntry = NHEntry.WithDecapsulateHeader(fluent.IPinIP)
		NHEntry = NHEntry.WithEncapsulateHeader(fluent.IPinIP)
		NHEntry = NHEntry.WithIPinIP("222.222.222.222", prefix)
		NHEntry = NHEntry.WithNextHopNetworkInstance("REPAIRED")
		args.client.Fluent(t).Modify().AddEntry(t, NHEntry)
		nhg.AddNextHop(uint64(nhge), uint64(256))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nhge++
	}

	nh = 1000
	nhge = 20000
	nhgi = 1000
	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(nhgi))
	for j = 1000; j <= 1007; j++ {
		if j == 1000 {
			wt = 7
		} else if j == 1001 {
			wt = 9
		} else {
			wt = 40
		}
		nhg.AddNextHop(j, uint64(wt))
		nhg.WithBackupNHG(uint64(nhge))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
	}

	prefixese := []string{}
	for i := 0; i < 15000; i++ {
		prefixese = append(prefixese, util.GetIPPrefix(dstPfx, i, mask))
	}

	nhgi = 1000
	for _, prefix := range prefixese {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance("TE").
			WithPrefix(prefix).
			WithNextHopGroup(uint64(nhgi)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	}

	args.client.AddNH(t, 30000, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 30000, 0, map[uint64]uint64{30000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, uint64(30001), atePort5.ip(uint8(1)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, uint64(30002), atePort5.ip(uint8(2)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNHG(t, 31000, 30000, map[uint64]uint64{uint64(30001): 1, uint64(30002): 255}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddIPv4Batch(t, prefixest, 31000, "REPAIRED", *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	testTrafficWeight(t, args.ate, args.top, 65000, true, 1)

	t.Run("testTrafficaftr frr", func(t *testing.T) {
		args.interfaceaction(t, "port2", false)
		args.interfaceaction(t, "port4", false)
		testTrafficWeight(t, args.ate, args.top, 65000, false, 8)
	})
	t.Run("testTrafficaftr frr", func(t *testing.T) {
		testTrafficmin(t, args.ate, args.top, 100, false, false, true)
	})

	t.Run("testTrafficaftr no shut", func(t *testing.T) {
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)
		testTrafficWeight(t, args.ate, args.top, 65000, false, 1)
	})
	// t.Run("testTraffic aftr rpfo", func(t *testing.T) {

	// 	utils.Dorpfo(args.ctx, t, true)
	// 	client = gribi.Client{
	// 		DUT:                   args.dut,
	// 		FibACK:                *ciscoFlags.GRIBIFIBCheck,
	// 		Persistence:           true,
	// 		InitialElectionIDLow:  1,
	// 		InitialElectionIDHigh: 0,
	// 	}
	// 	if err := client.Start(t); err != nil {
	// 		t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
	// 		if err = client.Start(t); err != nil {
	// 			t.Fatalf("gRIBI Connection could not be established: %v", err)
	// 		}
	// 	}
	// 	args.client = &client
	// 	testTrafficWeight(t, args.ate, args.top, 65000, false, 1)

	// })
	// t.Run("testTrafficaftr delete", func(t *testing.T) {

	// 	args.client.DeleteIPv4Batch(t, prefixese, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	// 	for _, prefix := range prefixese {
	// 		ipv4Entry := fluent.IPv4Entry().
	// 			WithNetworkInstance("TE").
	// 			WithPrefix(prefix).
	// 			WithNextHopGroup(uint64(1000)).
	// 			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	// 		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	// 	}
	// 	testTrafficWeight(t, args.ate, args.top, 65000, false, 1)
	// })

}

func TestWithDecapEncapTEUnoptimizedminpop(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with decapencap/decap, verify traffic, reprogram & delete ipv4/NHG/NH")

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	baseconfig(t)
	args.top.StopProtocols(t)
	time.Sleep(30 * time.Second)
	args.top.StartProtocols(t)

	// Configure the gRIBI client
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

	args.ctx = ctx
	args.client = &client
	args.dut = dut
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	nh := 1
	for i := 1; i <= 15; i++ {
		args.client.AddNH(t, uint64(nh), atePort2.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}
	for i := 1; i <= 15; i++ {
		args.client.AddNH(t, uint64(nh), atePort4.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}

	wt := 1
	nh = 1
	var i, j uint64
	for i = 1; i <= 7; i++ {
		nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(i))

		for j = 1; j <= 3; j++ {
			if j == 1 {
				wt = 3
			} else if j == 2 {
				wt = 25
			} else {
				wt = 31
			}
			nhg.AddNextHop(uint64(nh), uint64(wt))
			args.client.Fluent(t).Modify().AddEntry(t, nhg)
			nh++
		}
	}
	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(8))
	nh = 22
	for j = 1; j <= 9; j++ {
		if j < 2 {
			wt = 8
		} else {
			wt = 31
		}
		nhg.AddNextHop(uint64(nh), uint64(wt))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
		nh++
	}

	dstPfx2 := "205.205.205.1"
	prefixes := []string{}
	for i := 0; i < 8; i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx2, i, mask))
	}
	nhgID := 1
	i = 1
	for _, prefix := range prefixes {
		args.client.AddIPv4(t, prefix, uint64(nhgID), *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		nhgID++
	}
	nh = 1000
	wt = 1

	for _, prefix := range prefixes {
		b := strings.Split(prefix, "/")
		prefix = b[0]
		args.client.AddNH(t, uint64(nh), prefix, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		nh++
	}
	nhgi := 1000
	nh = 1000

	NHEntry := fluent.NextHopEntry()
	nhge := 20000
	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID((uint64(nhge)))
	NHEntry = NHEntry.WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithIndex(uint64(nhge))
	NHEntry = NHEntry.WithNextHopNetworkInstance("REPAIR")
	args.client.Fluent(t).Modify().AddEntry(t, NHEntry)
	nhg.AddNextHop(uint64(nhge), uint64(256))
	args.client.Fluent(t).Modify().AddEntry(t, nhg)

	nhgi = 1000
	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(nhgi))
	for j = 1000; j <= 1007; j++ {
		if j == 1000 {
			wt = 7
		} else if j == 1001 {
			wt = 9
		} else {
			wt = 40
		}
		nhg.AddNextHop(j, uint64(wt))
		nhg.WithBackupNHG(uint64(nhge))

		args.client.Fluent(t).Modify().AddEntry(t, nhg)
	}

	prefixese := []string{}
	for i := 0; i < 15000; i++ {
		prefixese = append(prefixese, util.GetIPPrefix(dstPfx, i, mask))
	}

	nhgi = 1000
	for _, prefix := range prefixese {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance("TE").
			WithPrefix(prefix).
			WithNextHopGroup(uint64(nhgi)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	}

	args.client.AddNH(t, 30000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 30000, 0, map[uint64]uint64{30000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, uint64(30001), atePort5.ip(uint8(1)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, uint64(30002), atePort5.ip(uint8(2)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 31000, 0, map[uint64]uint64{uint64(30001): 1, uint64(30002): 255}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, dsip+"/32", 31000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 31000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: "222.222.222.222", Dest: []string{dsip}})
	args.client.AddNHG(t, 32000, 30000, map[uint64]uint64{31000: 256}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixese, 32000, "REPAIR", *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	testTrafficWeight(t, args.ate, args.top, 65000, false, 1)
	t.Run("test traffic after frrs", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 100, false, true, false)
		testTrafficWeight(t, args.ate, args.top, 65000, false, 8)
		testTrafficmin(t, args.ate, args.top, 100, false, false, true)
	})
	t.Run("testTrafficaftr no shut", func(t *testing.T) {
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

		testTrafficWeight(t, args.ate, args.top, 65000, false, 1)
	})
	// t.Run("testTraffic aftr rpfo", func(t *testing.T) {

	// 	utils.Dorpfo(args.ctx, t, true)
	// 	client = gribi.Client{
	// 		DUT:                   args.dut,
	// 		FibACK:                *ciscoFlags.GRIBIFIBCheck,
	// 		Persistence:           true,
	// 		InitialElectionIDLow:  1,
	// 		InitialElectionIDHigh: 0,
	// 	}
	// 	if err := client.Start(t); err != nil {
	// 		t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
	// 		if err = client.Start(t); err != nil {
	// 			t.Fatalf("gRIBI Connection could not be established: %v", err)
	// 		}
	// 	}
	// 	args.client = &client
	// 	testTrafficWeight(t, args.ate, args.top, 65000, false, 1)

	// })
	// t.Run("testTrafficaftr delete", func(t *testing.T) {

	// 	args.client.DeleteIPv4Batch(t, prefixese, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	// 	for _, prefix := range prefixese {
	// 		ipv4Entry := fluent.IPv4Entry().
	// 			WithNetworkInstance("TE").
	// 			WithPrefix(prefix).
	// 			WithNextHopGroup(uint64(1000)).
	// 			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	// 		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	// 	}
	// 	testTrafficWeight(t, args.ate, args.top, 65000, true, 1)
	// })
}
