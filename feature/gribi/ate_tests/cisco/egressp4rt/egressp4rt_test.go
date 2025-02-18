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

package egressp4rt_test

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/google/gopacket/layers"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/gribigo/fluent"
	"golang.org/x/exp/rand"

	"context"

	"testing"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/gribi"
	"github.com/openconfig/featureprofiles/internal/deviations"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	IpEncap = "197.51.100.11"
	IpTE    = "198.51.100.1"

	Loopback22  = "88.88.88.88"
	Loopback226 = "8888::88"
	Loopback12  = "44.44.44.44"
	Loopback126 = "4444::44"
	Loopback0   = "30.30.30.30"
	Loopback06  = "30::30"
	// ipv4PrefixLen         = 30
	// ipv6PrefixLen         = 126
	dstPfx                = "198.51.100.1"
	mask                  = "32"
	dstPfxMin             = "198.51.1.1"
	dstPfxCount1          = 10
	innersrcPfx           = "200.1.0.1"
	innerdstPfxMin_bgp    = "202.1.0.1"
	innerdstPfxCount_bgp  = 10
	innerdstPfxMin_isis   = "201.1.0.1"
	innerdstPfxCount_isis = 10
	bundleEther126        = "Bundle-Ether126"
	lc                    = "0/0/CPU0"
	active_rp             = "0/RP0/CPU0"
	standby_rp            = "0/RP1/CPU0"
	dsip                  = "10.1.0.1"
	vrf1                  = "TE"
	vrf2                  = "TE2"
	vrf3                  = "TE3"
	pref6                 = "2555::2/128"
	prefi6                = "2556::2/128"
	enc                   = "197.51.100.11/32"
	encp                  = "197.51.100.11"
)

// func TestMain(m *testing.M) {
// 	fptest.RunTests(m)
// }

// Settings for configuring the baseline testbed with the test
// topology.
//
// The testbed consists of dut:port1 -> ate:port1,
// dut:port2 -> ate:port2, dut:port3 -> ate:port3, dut:port4 -> ate:port4, dut:port5 -> ate:port5,
// dut:port6 -> ate:port6, dut:port7 -> ate:port7 ,dut:port8 -> ate:port8

type TOptions struct {
	ptcp int
	pudp int
}

var baseconfigdone bool

// testArgs holds the objects needed by a test case.
// type testArgs struct {
// 	ctx    context.Context
// 	client *gribi.Client
// 	dut    *ondatra.DUTDevice
// 	ate    *ondatra.ATEDevice
// 	top    *ondatra.ATETopology
// }

var args *testArgs

// var IDMap = map[string]string{
// 	"10": atePort1.Name,
// 	"11": atePort2.Name,
// 	"12": atePort3.Name,
// 	"13": atePort4.Name,
// 	"14": atePort5.Name,
// 	"15": atePort6.Name,
// 	"16": atePort7.Name,
// 	"17": atePort8.Name,
// }

func baseconfig(t *testing.T) {
	t.Helper()

	if !baseconfigdone {
		args = &testArgs{}
		t.Log("in baseconfig")

		//Configure the DUT
		dut := ondatra.DUT(t, "dut")
		configureDUT(t, dut)
		//configbasePBR(t, dut, vrf1, "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{}, "PBR", dut.Port(t, "port1").Name(), false)
		//configure route-policy
		configRP(t, dut)
		//configure ISIS on DUT
		//util.AddISISOC(t, dut, bundleEther126)
		//configure BGP on DUT
		//util.AddBGPOC(t, dut, "100.100.100.100")
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
	ipv6nh := static.GetOrCreateStatic(ipv6EntryPrefix + "/128").GetOrCreateNextHop("0")
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

type Traceptions struct {
	Ip string
}

// func testWithDCBackUp(ctx context.Context, t *testing.T, args *testArgs, isIPv4 bool) {
// 	leader := args.leader
// 	follower := args.follower

// 	if isIPv4 {
// 		// Insert p4rtutils acl entry on the DUT
// 		if err := programmTableEntry(leader, args.packetIO, false, isIPv4); err != nil {
// 			t.Fatalf("There is error when programming entry")
// 		}
// 		// Delete p4rtutils acl entry on the device
// 		defer programmTableEntry(leader, args.packetIO, true, isIPv4)
// 	} else {
// 		// Insert p4rtutils acl entry on the DUT
// 		if err := programmTableEntry(leader, args.packetIO, false, false); err != nil {
// 			t.Fatalf("There is error when programming entry")
// 		}
// 		// Delete p4rtutils acl entry on the device
// 		defer programmTableEntry(leader, args.packetIO, true, false)
// 	}
// 	//time.Sleep(20000 * time.Minute)

// 	// Send Traceroute traffic from ATE

// 	// // Elect client as leader and flush all the past entries
// 	t.Logf("Program gribi entries, verify ping, traceroute triggers & ttl")
// 	dut2 := ondatra.DUT(t, "dut2")

// 	configDUT(t, dut2)

// 	ctx2 := context.Background()
// 	args = &testArgs{}

// 	// Configure the gRIBI client
// 	clientg := gribi.Client{
// 		DUT:                   dut2,
// 		FibACK:                *ciscoFlags.GRIBIFIBCheck,
// 		Persistence:           true,
// 		InitialElectionIDLow:  10,
// 		InitialElectionIDHigh: 0,
// 	}
// 	defer clientg.Close(t)
// 	if err := clientg.Start(t); err != nil {
// 		t.Fatalf("gRIBI Connection can not be established")
// 	}

// 	args.ctx = ctx2
// 	args.client = &clientg
// 	args.dut = dut2
// 	args.client.BecomeLeader(t)
// 	args.client.FlushServer(t)
// 	nh := 1
// 	nhip := "192.0.9.2"
// 	p4ip := "206.206.206.22"
// 	pre := "198.51.100.11"
// 	for i := 1; i <= 2; i++ {
// 		args.client.AddNH(t, uint64(nh), nhip, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
// 	}
// 	args.client.AddNHG(t, 101, 0, map[uint64]uint64{1: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
// 	args.client.AddIPv4(t, p4ip+"/32", 101, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
// 	nh = 1000
// 	for i := 1; i <= 2; i++ {
// 		args.client.AddNH(t, uint64(nh), p4ip, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
// 	}
// 	args.client.AddNHG(t, 102, 0, map[uint64]uint64{1000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
// 	ipv4Entry := fluent.IPv4Entry().
// 		WithNetworkInstance(vrf1).
// 		WithPrefix(pre + "/32").
// 		WithNextHopGroup(uint64(102)).
// 		WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
// 	args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
// 	args.client.AddNH(t, uint64(3000), "Encap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: []string{pre}, VrfName: vrf1})
// 	args.client.AddNHG(t, 103, 0, map[uint64]uint64{3000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
// 	ipv4Entry = fluent.IPv4Entry().
// 		WithNetworkInstance(vrfEncapA).
// 		WithPrefix(enc).
// 		WithNextHopGroup(uint64(103)).
// 		WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
// 	args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)

// 	args.client.AddIPv6(t, ipv6EntryPrefix+"/128", uint64(103), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

// 	flip := "196.51.100.2/32"
// 	flip2 := "197.51.100.2/32"
// 	ipv4Entry = fluent.IPv4Entry().
// 		WithNetworkInstance(vrf1).
// 		WithPrefix(flip).
// 		WithNextHopGroup(uint64(102)).
// 		WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
// 	args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
// 	args.client.AddNH(t, uint64(3001), "Encap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: []string{"196.51.100.2"}, VrfName: "TE"})
// 	args.client.AddNHG(t, 104, 0, map[uint64]uint64{3001: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
// 	ipv4Entry = fluent.IPv4Entry().
// 		WithNetworkInstance(vrfEncapA).
// 		WithPrefix(flip2).
// 		WithNextHopGroup(uint64(104)).
// 		WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
// 	args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)

// 	args.client.AddIPv6(t, prefi6, uint64(104), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

// 	dut := ondatra.DUT(t, "dut")
// 	ctx = context.Background()
// 	args = &testArgs{}
// 	destip := "197.51.0.0/16"

// 	baseconfig(t)
// 	addStaticRoute(t, dut, destip, true)
// 	configurePort(t, dut, "Loopback22", Loopback12, Loopback126, 32, 128)
// 	nhip1 := "192.0.9.2"
// 	nhip6 := "7777::2"
// 	nhip2 := "192.0.10.2"
// 	nhip26 := "192:0:2::1E"

// 	p1 := dut.Port(t, "port9")
// 	configurePort(t, dut, p1.Name(), nhip1, nhip6, 30, 126)

// 	p2 := dut.Port(t, "port10")
// 	configurePort(t, dut, p2.Name(), nhip2, nhip26, 30, 126)
// 	p3 := dut.Port(t, "port1")

// 	unconfigbasePBR(t, dut, "PBR", []string{p1.Name(), p2.Name(), p3.Name()})

// 	configPBR(t, dut, "PBR", true)
// 	configureIntfPBR(t, dut, "PBR", p3.Name())
// 	configureIntfPBR(t, dut, "PBR", p2.Name())
// 	configureIntfPBR(t, dut, "PBR", p1.Name())

// 	configvrfInt(t, dut, vrfEncapA, "Loopback22")

// 	configvrfInt(t, dut, vrfEncapA, p2.Name())
// 	staticvrf(t, dut, vrfEncapA, "192.0.10.1", "192:0:2::1d")
// 	staticvrf(t, dut, "DEFAULT", "192.0.9.1", "192:0:2::1a")

// 	// Configure the gRIBI client
// 	client := gribi.Client{
// 		DUT:                   dut,
// 		FibACK:                *ciscoFlags.GRIBIFIBCheck,
// 		Persistence:           true,
// 		InitialElectionIDLow:  10,
// 		InitialElectionIDHigh: 0,
// 	}
// 	defer client.Close(t)
// 	if err := client.Start(t); err != nil {
// 		t.Fatalf("gRIBI Connection can not be established")
// 	}

// 	args.ctx = ctx
// 	args.client = &client
// 	args.dut = dut
// 	args.client.BecomeLeader(t)
// 	args.client.FlushServer(t)

// 	dcchainconfig(t)
// 	args.client.AddNH(t, 22200, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
// 	args.client.AddNHG(t, 22200, 0, map[uint64]uint64{22200: 20}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

// 	nhge := 22200
// 	nhgi := 2220
// 	nh = 1000
// 	var wt int

// 	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(2220))

// 	for j := 1; j <= 8; j++ {
// 		if j < 2 {
// 			wt = 9
// 		} else if j == 2 {
// 			wt = 7
// 		} else {
// 			wt = 40
// 		}
// 		nhg.AddNextHop(uint64(nh), uint64(wt))
// 		nhg.WithBackupNHG(uint64(nhge))
// 		args.client.Fluent(t).Modify().AddEntry(t, nhg)
// 		nh++
// 	}

// 	nhgi = 2220
// 	dstPfxx := "196.51.100.1"
// 	prefixesr := []string{}
// 	for i := 0; i < 5000; i++ {
// 		prefixesr = append(prefixesr, util.GetIPPrefix(dstPfxx, i, mask))
// 	}

// 	for _, prefix := range prefixesr {
// 		ipv4Entry := fluent.IPv4Entry().
// 			WithNetworkInstance(vrfRepaired).
// 			WithPrefix(prefix).
// 			WithNextHopGroup(uint64(nhgi)).
// 			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
// 		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
// 	}
// 	args.client.AddIPv6(t, pref6, uint64(nhgi), vrfRepaired, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

// 	// for k := 1; k <= 2; k++ {
// 	// 	t.Run("Ping, Traceroute test", func(t *testing.T) {
// 	// 		p1 = dut.Port(t, "port2")
// 	// 		p2 = dut.Port(t, "port4")

// 	// 		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping", IpEncap, Loopback12, vrfEncapA)
// 	// 		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute", IpEncap, Loopback12, vrfEncapA)
// 	// 	})

// 	// 	t.Run("Ping, Traceroute test Ip", func(t *testing.T) {
// 	// 		p1 = dut.Port(t, "port2")
// 	// 		p2 = dut.Port(t, "port4")

// 	// 		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ip-ping", IpEncap, Loopback0, vrfEncapA)
// 	// 		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ip-traceroute", IpEncap, Loopback0, vrfEncapA, &Traceptions{Ip: "ip"})
// 	// 	})
// 	// 	t.Run("Ping, Traceroute test IpinIp", func(t *testing.T) {
// 	// 		p1 = dut.Port(t, "port2")
// 	// 		p2 = dut.Port(t, "port4")

// 	// 		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping-ipinip", IpEncap, Loopback22, vrfEncapA)
// 	// 		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute-ipinip", IpEncap, Loopback22, vrfEncapA)
// 	// 	})
// 	// 	t.Run("Ping, Traceroute test FRRs", func(t *testing.T) {
// 	// 		args.interfaceaction(t, "port2", false)
// 	// 		args.interfaceaction(t, "port4", false)

// 	// 		p1 = dut.Port(t, "port5")

// 	// 		testStats(t, dut, dut2, []string{p1.Name()}, "ping", IpEncap, Loopback12, vrfEncapA)
// 	// 		testStats(t, dut, dut2, []string{p1.Name()}, "traceroute", IpEncap, Loopback12, vrfEncapA)

// 	// 		args.interfaceaction(t, "port5", false)
// 	// 		time.Sleep(5 * time.Second)
// 	// 		p2 = dut.Port(t, "port3")
// 	// 		testStats(t, dut, dut2, []string{p2.Name()}, "ping", IpEncap, Loopback12, vrfEncapA)
// 	// 		testStats(t, dut, dut2, []string{p2.Name()}, "traceroute", IpEncap, Loopback12, vrfEncapA)

// 	// 		args.interfaceaction(t, "port2", true)
// 	// 		args.interfaceaction(t, "port4", true)
// 	// 		args.interfaceaction(t, "port5", true)

// 	// 	})

// 	// 	t.Run("Ping, Traceroute test Ip Frrs", func(t *testing.T) {
// 	// 		args.interfaceaction(t, "port2", false)
// 	// 		args.interfaceaction(t, "port4", false)
// 	// 		p1 = dut.Port(t, "port5")
// 	// 		p2 = dut.Port(t, "port3")

// 	// 		testStats(t, dut, dut2, []string{p1.Name()}, "ip-ping", IpEncap, Loopback0, vrfEncapA)
// 	// 		testStats(t, dut, dut2, []string{p1.Name()}, "ip-traceroute", IpEncap, Loopback0, vrfEncapA, &Traceptions{Ip: "ip"})
// 	// 		args.interfaceaction(t, "port5", false)
// 	// 		time.Sleep(5 * time.Second)

// 	// 		testStats(t, dut, dut2, []string{p2.Name()}, "ip-ping", IpEncap, Loopback0, vrfEncapA)
// 	// 		testStats(t, dut, dut2, []string{p2.Name()}, "ip-traceroute", IpEncap, Loopback0, vrfEncapA, &Traceptions{Ip: "ip"})
// 	// 		args.interfaceaction(t, "port2", true)
// 	// 		args.interfaceaction(t, "port4", true)
// 	// 		args.interfaceaction(t, "port5", true)

// 	// 	})

// 	// 	t.Run("Ping, Traceroute test IpinIp FRRs", func(t *testing.T) {
// 	// 		p1 = dut.Port(t, "port5")
// 	// 		p2 = dut.Port(t, "port3")
// 	// 		args.interfaceaction(t, "port2", false)
// 	// 		args.interfaceaction(t, "port4", false)
// 	// 		testStats(t, dut, dut2, []string{p1.Name()}, "ping-ipinip", IpEncap, Loopback22, vrfEncapA)
// 	// 		testStats(t, dut, dut2, []string{p1.Name()}, "traceroute-ipinip", IpEncap, Loopback22, vrfEncapA)
// 	// 		args.interfaceaction(t, "port5", false)
// 	// 		time.Sleep(5 * time.Second)

// 	// 		testStats(t, dut, dut2, []string{p2.Name()}, "ping-ipinip", IpEncap, Loopback22, vrfEncapA)
// 	// 		testStats(t, dut, dut2, []string{p2.Name()}, "traceroute-ipinip", IpEncap, Loopback22, vrfEncapA)

// 	// 		args.interfaceaction(t, "port2", true)
// 	// 		args.interfaceaction(t, "port4", true)
// 	// 		args.interfaceaction(t, "port5", true)

// 	// 	})

// 	// 	t.Run("Ping, Traceroute test ipv6", func(t *testing.T) {
// 	// 		p1 := dut.Port(t, "port2")
// 	// 		p2 := dut.Port(t, "port4")
// 	// 		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping", ipv6EntryPrefix, Loopback126, vrfEncapA)
// 	// 		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute", ipv6EntryPrefix, Loopback126, vrfEncapA)
// 	// 	})

// 	// 	t.Run("Ping, Traceroute test Ipv6", func(t *testing.T) {
// 	// 		p1 = dut.Port(t, "port2")
// 	// 		p2 = dut.Port(t, "port4")
// 	// 		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ip-ping", ipv6EntryPrefix, Loopback06, vrfEncapA)
// 	// 		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ip-traceroute", ipv6EntryPrefix, Loopback06, vrfEncapA, &Traceptions{Ip: "ipv6"})
// 	// 	})
// 	// 	t.Run("Ping, Traceroute test Ipv6inIp", func(t *testing.T) {
// 	// 		p1 = dut.Port(t, "port2")
// 	// 		p2 = dut.Port(t, "port4")
// 	// 		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping-ipinip", ipv6EntryPrefix, Loopback226, vrfEncapA)
// 	// 		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute-ipinip", ipv6EntryPrefix, Loopback226, vrfEncapA)
// 	// 	})
// 	// 	t.Run("Ping, Traceroute tests  ipv6 FRRs", func(t *testing.T) {
// 	// 		args.interfaceaction(t, "port2", false)
// 	// 		args.interfaceaction(t, "port4", false)
// 	// 		p1 = dut.Port(t, "port5")

// 	// 		testStats(t, dut, dut2, []string{p1.Name()}, "ping", ipv6EntryPrefix, Loopback126, vrfEncapA)
// 	// 		testStats(t, dut, dut2, []string{p1.Name()}, "traceroute", ipv6EntryPrefix, Loopback126, vrfEncapA)

// 	// 		args.interfaceaction(t, "port5", false)
// 	// 		time.Sleep(5 * time.Second)

// 	// 		testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "ping", ipv6EntryPrefix, Loopback126, vrfEncapA)
// 	// 		testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "traceroute", ipv6EntryPrefix, Loopback126, vrfEncapA)
// 	// 		args.interfaceaction(t, "port2", true)
// 	// 		args.interfaceaction(t, "port4", true)
// 	// 		args.interfaceaction(t, "port5", true)

// 	// 	})

// 	// 	t.Run("Ping, Traceroute test Ipv6 Frrs", func(t *testing.T) {
// 	// 		args.interfaceaction(t, "port2", false)
// 	// 		args.interfaceaction(t, "port4", false)
// 	// 		p1 = dut.Port(t, "port5")
// 	// 		testStats(t, dut, dut2, []string{p1.Name()}, "ip-ping", ipv6EntryPrefix, Loopback06, vrfEncapA)
// 	// 		testStats(t, dut, dut2, []string{p1.Name()}, "ip-traceroute", ipv6EntryPrefix, Loopback06, vrfEncapA, &Traceptions{Ip: "ipv6"})
// 	// 		args.interfaceaction(t, "port5", false)
// 	// 		time.Sleep(5 * time.Second)

// 	// 		testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "ip-ping", ipv6EntryPrefix, Loopback06, vrfEncapA)
// 	// 		testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "ip-traceroute", ipv6EntryPrefix, Loopback06, vrfEncapA, &Traceptions{Ip: "ipv6"})
// 	// 		args.interfaceaction(t, "port2", true)
// 	// 		args.interfaceaction(t, "port4", true)
// 	// 		args.interfaceaction(t, "port5", true)
// 	// 	})

// 	// 	t.Run("Ping, Traceroute tests Ipv6inIp FRRs", func(t *testing.T) {
// 	// 		args.interfaceaction(t, "port2", false)
// 	// 		args.interfaceaction(t, "port4", false)
// 	// 		p1 = dut.Port(t, "port5")

// 	// 		testStats(t, dut, dut2, []string{p1.Name()}, "ping-ipinip", ipv6EntryPrefix, Loopback226, vrfEncapA)
// 	// 		testStats(t, dut, dut2, []string{p1.Name()}, "traceroute-ipinip", ipv6EntryPrefix, Loopback226, vrfEncapA)
// 	// 		args.interfaceaction(t, "port5", false)
// 	// 		time.Sleep(5 * time.Second)

// 	// 		testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "ping-ipinip", ipv6EntryPrefix, Loopback226, vrfEncapA)
// 	// 		testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "traceroute-ipinip", ipv6EntryPrefix, Loopback226, vrfEncapA)

// 	// 		args.interfaceaction(t, "port2", true)
// 	// 		args.interfaceaction(t, "port4", true)
// 	// 		args.interfaceaction(t, "port5", true)

// 	// 	})

// 	// 	t.Run("test with ttl 1 & different ttl", func(t *testing.T) {

// 	// 		testTraffic(t, args.ate, args.top, 5, true, false, false)

// 	// 		testTraffic(t, args.ate, args.top, 4, true, false, false)
// 	// 	})
// 	// 	t.Run("test with ttl 1 frrs", func(t *testing.T) {

// 	// 		testTraffic(t, args.ate, args.top, 5, true, true, false)

// 	// 		testTraffic(t, args.ate, args.top, 5, true, false, true)
// 	// 		args.interfaceaction(t, "port2", true)
// 	// 		args.interfaceaction(t, "port4", true)
// 	// 		args.interfaceaction(t, "port5", true)
// 	// 	})

// 	// 	t.Run("test with ttl different - frrs", func(t *testing.T) {

// 	// 		testTraffic(t, args.ate, args.top, 4, true, true, false)

// 	// 		testTraffic(t, args.ate, args.top, 4, true, false, true)
// 	// 		args.interfaceaction(t, "port2", true)
// 	// 		args.interfaceaction(t, "port4", true)
// 	// 		args.interfaceaction(t, "port5", true)

// 	// 	})

// 	// 	t.Run("test with ttl 1 & different ttl ipv6", func(t *testing.T) {

// 	// 		testTraffic(t, args.ate, args.top, 6, true, false, false)

// 	// 		testTraffic(t, args.ate, args.top, 7, true, false, false)
// 	// 		args.interfaceaction(t, "port2", true)
// 	// 		args.interfaceaction(t, "port4", true)
// 	// 		args.interfaceaction(t, "port5", true)
// 	// 	})
// 	// 	t.Run("test with ttl 1 frrs ipv6", func(t *testing.T) {

// 	// 		testTraffic(t, args.ate, args.top, 6, true, true, false)

// 	// 		testTraffic(t, args.ate, args.top, 6, true, false, true)
// 	// 		args.interfaceaction(t, "port2", true)
// 	// 		args.interfaceaction(t, "port4", true)
// 	// 		args.interfaceaction(t, "port5", true)
// 	// 	})

// 	// 	t.Run("test with ttl different - frrs ipv6", func(t *testing.T) {

// 	// 		testTraffic(t, args.ate, args.top, 7, true, true, false)

// 	// 		testTraffic(t, args.ate, args.top, 7, true, false, true)
// 	// 		args.interfaceaction(t, "port2", true)
// 	// 		args.interfaceaction(t, "port4", true)
// 	// 		args.interfaceaction(t, "port5", true)

// 	// 	})

// 	// 	defer args.interfaceaction(t, "port2", true)
// 	// 	defer args.interfaceaction(t, "port4", true)
// 	// 	defer args.interfaceaction(t, "port5", true)

// 	// 	if k == 1 {
// 	// 		t.Run("tests after rpfo", func(t *testing.T) {

// 	// 			utils.Dorpfo(args.ctx, t, true)
// 	// 			client = gribi.Client{
// 	// 				DUT:                   args.dut,
// 	// 				FibACK:                *ciscoFlags.GRIBIFIBCheck,
// 	// 				Persistence:           true,
// 	// 				InitialElectionIDLow:  1,
// 	// 				InitialElectionIDHigh: 0,
// 	// 			}
// 	// 			if err := client.Start(t); err != nil {
// 	// 				t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
// 	// 				if err = client.Start(t); err != nil {
// 	// 					t.Fatalf("gRIBI Connection could not be established: %v", err)
// 	// 				}
// 	// 			}
// 	// 			args.client = &client
// 	// 		})
// 	// 		time.Sleep(6 * time.Minute)
// 	// 	}
// 	// }

// 	// p3 = dut.Port(t, "port1")
// 	// p4 := dut.Port(t, "port9")
// 	// p5 := dut.Port(t, "port10")
// 	// unconfigbasePBR(t, dut, "PBR", []string{p3.Name(), p4.Name(), p5.Name()})

// 	// configPBR(t, dut, vrfRepaired, false)
// 	// configureIntfPBR(t, dut, "PBR", p3.Name())
// 	// configureIntfPBR(t, dut, "PBR", p4.Name())
// 	// configureIntfPBR(t, dut, "PBR", p5.Name())
// 	// configvrfInt(t, dut, vrfRepaired, "Loopback22")

// 	// ip4 := "197.51.100.2"
// 	// ip6 := "2556::2"

// 	// t.Run("Fallback Ping, Traceroute test IpinIp", func(t *testing.T) {
// 	// 	p1 = dut.Port(t, "port2")
// 	// 	p2 = dut.Port(t, "port4")

// 	// 	testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping-ipinip", ip4, Loopback22, vrfEncapA)
// 	// 	testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute-ipinip", ip4, Loopback22, vrfEncapA)
// 	// })

// 	// t.Run("Fallback Ping, Traceroute test IpinIp FRRs", func(t *testing.T) {
// 	// 	p2 = dut.Port(t, "port3")
// 	// 	args.interfaceaction(t, "port2", false)
// 	// 	args.interfaceaction(t, "port4", false)

// 	// 	testStats(t, dut, dut2, []string{p2.Name()}, "ping-ipinip", ip4, Loopback22, vrfEncapA)
// 	// 	testStats(t, dut, dut2, []string{p2.Name()}, "traceroute-ipinip", ip4, Loopback22, vrfEncapA)

// 	// 	args.interfaceaction(t, "port2", true)
// 	// 	args.interfaceaction(t, "port4", true)
// 	// 	args.interfaceaction(t, "port5", true)

// 	// })

// 	// t.Run("Fallback Ping, Traceroute test Ipv6inIp", func(t *testing.T) {
// 	// 	p1 = dut.Port(t, "port2")
// 	// 	p2 = dut.Port(t, "port4")
// 	// 	testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping-ipinip", ip6, Loopback226, vrfEncapA)
// 	// 	testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute-ipinip", ip6, Loopback226, vrfEncapA)
// 	// })

// 	// t.Run("Fallback Ping, Traceroutes tests Ipv6inIp FRRs", func(t *testing.T) {
// 	// 	args.interfaceaction(t, "port2", false)
// 	// 	args.interfaceaction(t, "port4", false)

// 	// 	testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "ping-ipinip", ip6, Loopback226, vrfEncapA)
// 	// 	testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "traceroute-ipinip", ip6, Loopback226, vrfEncapA)

// 	// 	args.interfaceaction(t, "port2", true)
// 	// 	args.interfaceaction(t, "port4", true)
// 	// 	args.interfaceaction(t, "port5", true)

// 	// })

// 	// t.Run("Fallback test with ttl 1 & different ttl", func(t *testing.T) {

// 	// 	testTraffic(t, args.ate, args.top, 5, false, false, false)

// 	// 	testTraffic(t, args.ate, args.top, 4, false, false, false)
// 	// })
// 	// t.Run("Fallback test with ttl 1 frrs", func(t *testing.T) {

// 	// 	testTraffic(t, args.ate, args.top, 5, false, true, true)
// 	// 	args.interfaceaction(t, "port2", true)
// 	// 	args.interfaceaction(t, "port4", true)
// 	// 	args.interfaceaction(t, "port5", true)

// 	// })

// 	// t.Run("Fallback test with ttl different - frrs", func(t *testing.T) {

// 	// 	testTraffic(t, args.ate, args.top, 4, false, true, true)
// 	// 	args.interfaceaction(t, "port2", true)
// 	// 	args.interfaceaction(t, "port4", true)
// 	// 	args.interfaceaction(t, "port5", true)

// 	// })

// 	// t.Run("Fallback test with ttl 1 & different ttl ipv6", func(t *testing.T) {

// 	// 	testTraffic(t, args.ate, args.top, 6, false, false, false)

// 	// 	testTraffic(t, args.ate, args.top, 7, false, false, false)

// 	// })
// 	// t.Run("Fallback test with ttl 1 frrs ipv6", func(t *testing.T) {

// 	// 	testTraffic(t, args.ate, args.top, 6, false, true, true)

// 	// 	args.interfaceaction(t, "port2", true)
// 	// 	args.interfaceaction(t, "port4", true)
// 	// 	args.interfaceaction(t, "port5", true)
// 	// })

// 	// t.Run("Fallback test with ttl different - frrs ipv6", func(t *testing.T) {

// 	// 	testTraffic(t, args.ate, args.top, 7, false, true, true)
// 	// 	args.interfaceaction(t, "port2", true)
// 	// 	args.interfaceaction(t, "port4", true)
// 	// 	args.interfaceaction(t, "port5", true)

// 	// })

// 	//srcEndPoint := args.top.Interfaces()[atePort1.Name]
// 	// pktOut := testTraffic(t, args.ate, args.packetIO.GetTrafficFlow(args.ate, isIPv4, 1, 300, 2), srcEndPoint, 10)
// 	testTraffic(t, args.ate, args.top, 100, true, false, false)

// 	fmt.Println("oooo2222")
// 	// fmt.Println(pktOut)
// 	packetInTests := []struct {
// 		desc     string
// 		client   *p4rt_client.P4RTClient
// 		wantPkts int
// 	}{{
// 		desc:   "PacketIn to Primary Controller",
// 		client: leader,
// 		//wantPkts: pktOut,
// 	}, {
// 		desc:   "PacketIn to Secondary Controller",
// 		client: follower,
// 		//wantPkts: 0,
// 	}}

// 	t.Log("TTL/HopLimit 1")
// 	for _, test := range packetInTests {
// 		t.Run(test.desc, func(t *testing.T) {
// 			// Extract packets from PacketIn message sent to p4rt client
// 			_, packets, err := test.client.StreamChannelGetPackets(&streamName, uint64(test.wantPkts), 30*time.Second)
// 			if err != nil {
// 				t.Errorf("Unexpected error on fetchPackets: %v", err)
// 			}

// 			if test.wantPkts == 0 {
// 				return
// 			}

// 			gotPkts := 0
// 			t.Logf("Start to decode packet and compare with expected packets.")
// 			wantPacket := args.packetIO.GetPacketTemplate()
// 			fmt.Println("oooo7777")
// 			fmt.Println(wantPacket)
// 			time.Sleep(20000 * time.Minute)
// 			for _, packet := range packets {
// 				if packet != nil {

// 					srcMAC, _, etherType := decodePacket(t, packet.Pkt.GetPayload())
// 					if etherType != layers.EthernetTypeIPv4 && etherType != layers.EthernetTypeIPv6 {
// 						continue
// 					}
// 					if !strings.EqualFold(srcMAC, tracerouteSrcMAC) {
// 						continue
// 					}
// 					if wantPacket.TTL != nil {
// 						//TTL/HopLimit comparison for IPV4 & IPV6
// 						if isIPv4 {
// 							captureTTL := decodePacket4(t, packet.Pkt.GetPayload())
// 							if captureTTL != TTL1 {
// 								t.Fatalf("Packet in PacketIn message is not matching wanted packet=IPV4 TTL1")
// 							}

// 						} else {
// 							captureHopLimit := decodePacket6(t, packet.Pkt.GetPayload())
// 							if captureHopLimit != HopLimit1 {
// 								t.Fatalf("Packet in PacketIn message is not matching wanted packet=IPV6 HopLimit1")
// 							}
// 						}
// 					}

// 					//Metadata comparision
// 					if metaData := packet.Pkt.GetMetadata(); metaData != nil {
// 						if got := metaData[0].GetMetadataId(); got == METADATA_INGRESS_PORT {
// 							fmt.Println("xxxxx")
// 							t.Logf("Metadata ingress port: want metadatingress %d, got %d", METADATA_INGRESS_PORT, got)
// 							if gotPortID := string(metaData[0].GetValue()); gotPortID != args.packetIO.GetIngressPort() {
// 								t.Fatalf("Ingress Port Id mismatch: want %s, got %s", args.packetIO.GetIngressPort(), gotPortID)
// 							}
// 						} else {
// 							t.Fatalf("Metadata ingress port mismatch: want %d, got %d", METADATA_INGRESS_PORT, got)
// 						}
// 						fmt.Println("2222222")
// 						t.Log(metaData)

// 						if got := metaData[1].GetMetadataId(); got == METADATA_EGRESS_PORT {
// 							if gotPortID := string(metaData[1].GetValue()); gotPortID != args.packetIO.GetEgressPort() {
// 								t.Fatalf("Egress Port Id mismatch: want %s, got %s", args.packetIO.GetEgressPort(), gotPortID)
// 							}
// 						} else {
// 							t.Fatalf("Metadata egress port mismatch: want %d, got %d", METADATA_EGRESS_PORT, got)
// 						}
// 					} else {
// 						t.Fatalf("Packet missing metadata information.")
// 					}
// 					gotPkts++
// 				}
// 			}
// 			if got, want := gotPkts, test.wantPkts; got != want {
// 				t.Errorf("Number of PacketIn, got: %d, want: %d", got, want)
// 			}
// 		})
// 	}
// }

func testWithDCUnoptimized(ctx context.Context, t *testing.T, args *testArgs, isIPv4, encap bool, flap, te string, deviceSet bool, opts ...*TOptions) {

	leader := args.leader
	follower := args.follower

	if isIPv4 {
		// Insert p4rtutils acl entry on the DUT
		if err := programmTableEntry(leader, args.packetIO, false, isIPv4, deviceId); err != nil {
			t.Fatalf("There is error when programming entry")
		}
		if deviceSet {

			if err := programmTableEntry(leader, args.packetIO, false, isIPv4, deviceId2); err != nil {
				t.Fatalf("There is error when programming entry")
			}
			defer programmTableEntry(leader, args.packetIO, true, isIPv4, deviceId2)

		}
		// Delete p4rtutils acl entry on the device
		defer programmTableEntry(leader, args.packetIO, true, isIPv4, deviceId)

	} else {
		// Insert p4rtutils acl entry on the DUT
		if err := programmTableEntry(leader, args.packetIO, false, false, deviceId); err != nil {
			t.Fatalf("There is error when programming entry")
		}

		if deviceSet {

			if err := programmTableEntry(leader, args.packetIO, false, false, deviceId2); err != nil {
				t.Fatalf("There is error when programming entry")
			}

			defer programmTableEntry(leader, args.packetIO, true, false, deviceId2)

		}
		// Delete p4rtutils acl entry on the device
		defer programmTableEntry(leader, args.packetIO, true, false, deviceId)

	}

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries, verify ping, traceroute triggers & ttl")

	dut := ondatra.DUT(t, "dut")
	//ctx = context.Background()
	baseconfig(t)

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
	configurePort(t, dut, "Loopback22", Loopback12, Loopback126, 32, 128)

	// p1 := dut.Port(t, "port9")
	// p2 := dut.Port(t, "port10")
	//p3 := dut.Port(t, "port1")

	//unconfigbasePBR(t, dut, "PBR", []string{p1.Name(), p2.Name(), p3.Name()})
	//unconfigbasePBR(t, dut, "PBR", []string{"Bundle-Ether120"})

	configPBR(t, dut, "PBR", true)
	configureIntfPBR(t, dut, "PBR", "Bundle-Ether120")
	//configureIntfPBR(t, dut, "PBR", p2.Name())
	//configureIntfPBR(t, dut, "PBR", p1.Name())
	configvrfInt(t, dut, vrfEncapA, "Loopback22")

	//dcunoptchain(t)
	//dut2 := ondatra.DUT(t, "dut2")
	nh := 1
	// for i := 1; i <= 15; i++ {
	// 	args.client.AddNH(t, uint64(nh), atePort2.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	// 	nh++
	// }
	for i := 1; i <= 15; i++ {
		//args.client.AddNH(t, uint64(nh), atePort4.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, uint64(nh), atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)

		nh++
	}
	//nh = 1
	for i := 1; i <= 15; i++ {
		args.client.AddNH(t, uint64(nh), atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
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
	NHEntry = NHEntry.WithNextHopNetworkInstance(vrfRepair)
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
	//args.client.AddNH(t, uint64(30001), atePort5.ip(uint8(1)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	// args.client.AddNH(t, uint64(30002), atePort5.ip(uint8(2)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	//args.client.AddNH(t, uint64(30001), "Bundle-Ether120", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	//args.client.AddNH(t, uint64(30002), "Bundle-Ether120", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, uint64(30001), atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, uint64(30002), atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)

	args.client.AddNHG(t, 31000, 0, map[uint64]uint64{uint64(30001): 1, uint64(30002): 255}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, dsip+"/32", 31000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 31000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: "222.222.222.222", Dest: []string{dsip}})
	args.client.AddNHG(t, 32000, 30000, map[uint64]uint64{31000: 256}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixese, 32000, vrfRepair, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

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

	dstPfxe = "197.51.1.1"
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
	args.client.AddIPv6(t, ipv6EntryPrefix+"/128", uint64(3000), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	//args.client.AddIPv6(t, "2555::/16", uint64(3000), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, "2555:1::/32", uint64(3000), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

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

	args.client.AddNH(t, 22200, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 22200, 0, map[uint64]uint64{22200: 20}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	nhge = 22200
	nhgi = 2220
	nh = 1000
	//	var wt int

	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(2220))

	for j := 1; j <= 8; j++ {
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

	nhgi = 2220
	dstPfxx := "196.51.100.1"
	prefixesr := []string{}
	for i := 0; i < 5000; i++ {
		prefixesr = append(prefixesr, util.GetIPPrefix(dstPfxx, i, mask))
	}

	for _, prefix := range prefixesr {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance(vrfRepaired).
			WithPrefix(prefix).
			WithNextHopGroup(uint64(nhgi)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	}
	args.client.AddIPv6(t, pref6, uint64(nhgi), vrfRepaired, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, prefi6, uint64(nhgi), vrfRepaired, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	
	args.client.DeleteIPv4Batch(t, prefixesd, 4000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	// for _, prefix := range prefixesd {
	// 	ipv4Entry := fluent.IPv4Entry().
	// 		WithNetworkInstance(vrfDecap).
	// 		WithPrefix(prefix).
	// 		WithNextHopGroup(uint64(4000)).
	// 		WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	// 	args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	// }
	
	for _, prefix := range prefixesd {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance(vrfDecap).
			WithPrefix(prefix).
			WithNextHopGroup(uint64(4000)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	}
	// t.Run("Ping, Traceroute test", func(t *testing.T) {
	// 	p1 := dut.Port(t, "port2")
	// 	p2 := dut.Port(t, "port4")
	// 	testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping", IpEncap, Loopback12, vrfEncapA)
	// 	testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute", IpEncap, Loopback12, vrfEncapA)
	// })

	// t.Run("Ping, Traceroute test Ip", func(t *testing.T) {
	// 	p1 := dut.Port(t, "port2")
	// 	p2 := dut.Port(t, "port4")

	// 	testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ip-ping", IpEncap, Loopback0, vrfEncapA)
	// 	testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ip-traceroute", IpEncap, Loopback0, vrfEncapA, &Traceptions{Ip: "ip"})
	// })
	// t.Run("Ping, Traceroute test IpinIp", func(t *testing.T) {
	// 	p1 := dut.Port(t, "port2")
	// 	p2 := dut.Port(t, "port4")

	// 	testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping-ipinip", IpEncap, Loopback22, vrfEncapA)
	// 	testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute-ipinip", IpEncap, Loopback22, vrfEncapA)
	// })
	// t.Run("Ping, Traceroute test FRRs", func(t *testing.T) {
	// 	args.interfaceaction(t, "port2", false)
	// 	args.interfaceaction(t, "port4", false)

	// 	p1 := dut.Port(t, "port5")

	// 	testStats(t, dut, dut2, []string{p1.Name()}, "ping", IpEncap, Loopback12, vrfEncapA)
	// 	testStats(t, dut, dut2, []string{p1.Name()}, "traceroute", IpEncap, Loopback12, vrfEncapA)

	// 	args.interfaceaction(t, "port5", false)
	// 	time.Sleep(5 * time.Second)

	// 	p2 := dut.Port(t, "port3")
	// 	testStats(t, dut, dut2, []string{p2.Name()}, "ping", IpEncap, Loopback12, vrfEncapA)
	// 	testStats(t, dut, dut2, []string{p2.Name()}, "traceroute", IpEncap, Loopback12, vrfEncapA)

	// 	args.interfaceaction(t, "port2", true)
	// 	args.interfaceaction(t, "port4", true)
	// 	args.interfaceaction(t, "port5", true)

	// })

	// t.Run("Ping, Traceroute test Ip Frrs", func(t *testing.T) {
	// 	args.interfaceaction(t, "port2", false)
	// 	args.interfaceaction(t, "port4", false)
	// 	p1 := dut.Port(t, "port5")
	// 	p2 := dut.Port(t, "port3")
	// 	time.Sleep(5 * time.Second)

	// 	testStats(t, dut, dut2, []string{p1.Name()}, "ip-ping", IpEncap, Loopback0, vrfEncapA)
	// 	testStats(t, dut, dut2, []string{p1.Name()}, "ip-traceroute", IpEncap, Loopback0, vrfEncapA, &Traceptions{Ip: "ip"})
	// 	args.interfaceaction(t, "port5", false)
	// 	time.Sleep(5 * time.Second)

	// 	testStats(t, dut, dut2, []string{p2.Name()}, "ip-ping", IpEncap, Loopback0, vrfEncapA)
	// 	testStats(t, dut, dut2, []string{p2.Name()}, "ip-traceroute", IpEncap, Loopback0, vrfEncapA, &Traceptions{Ip: "ip"})
	// 	args.interfaceaction(t, "port2", true)
	// 	args.interfaceaction(t, "port4", true)
	// 	args.interfaceaction(t, "port5", true)

	// })

	// t.Run("Ping, Traceroute test IpinIp FRRs", func(t *testing.T) {
	// 	p1 := dut.Port(t, "port5")
	// 	p2 := dut.Port(t, "port3")
	// 	args.interfaceaction(t, "port2", false)
	// 	args.interfaceaction(t, "port4", false)
	// 	time.Sleep(5 * time.Second)

	// 	testStats(t, dut, dut2, []string{p1.Name()}, "ping-ipinip", IpEncap, Loopback22, vrfEncapA)
	// 	testStats(t, dut, dut2, []string{p1.Name()}, "traceroute-ipinip", IpEncap, Loopback22, vrfEncapA)
	// 	args.interfaceaction(t, "port5", false)
	// 	time.Sleep(5 * time.Second)

	// 	testStats(t, dut, dut2, []string{p2.Name()}, "ping-ipinip", IpEncap, Loopback22, vrfEncapA)
	// 	testStats(t, dut, dut2, []string{p2.Name()}, "traceroute-ipinip", IpEncap, Loopback22, vrfEncapA)

	// 	args.interfaceaction(t, "port2", true)
	// 	args.interfaceaction(t, "port4", true)
	// 	args.interfaceaction(t, "port5", true)

	// })

	// t.Run("Ping, Traceroute test ipv6", func(t *testing.T) {
	// 	p1 := dut.Port(t, "port2")
	// 	p2 := dut.Port(t, "port4")
	// 	testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping", ipv6EntryPrefix, Loopback126, vrfEncapA)
	// 	testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute", ipv6EntryPrefix, Loopback126, vrfEncapA)
	// })

	// t.Run("Ping, Traceroute test Ipv6", func(t *testing.T) {
	// 	p1 := dut.Port(t, "port2")
	// 	p2 := dut.Port(t, "port4")
	// 	testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ip-ping", ipv6EntryPrefix, Loopback06, vrfEncapA)
	// 	testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ip-traceroute", ipv6EntryPrefix, Loopback06, vrfEncapA, &Traceptions{Ip: "ipv6"})
	// })
	// t.Run("Ping, Traceroute test Ipv6inIp", func(t *testing.T) {
	// 	p1 := dut.Port(t, "port2")
	// 	p2 := dut.Port(t, "port4")
	// 	testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping-ipinip", ipv6EntryPrefix, Loopback226, vrfEncapA)
	// 	testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute-ipinip", ipv6EntryPrefix, Loopback226, vrfEncapA)
	// })
	// t.Run("Ping, Traceroute tests  ipv6 FRRs", func(t *testing.T) {
	// 	args.interfaceaction(t, "port2", false)
	// 	args.interfaceaction(t, "port4", false)
	// 	p1 := dut.Port(t, "port5")
	// 	time.Sleep(5 * time.Second)

	// 	testStats(t, dut, dut2, []string{p1.Name()}, "ping", ipv6EntryPrefix, Loopback126, vrfEncapA)
	// 	testStats(t, dut, dut2, []string{p1.Name()}, "traceroute", ipv6EntryPrefix, Loopback126, vrfEncapA)

	// 	args.interfaceaction(t, "port5", false)
	// 	time.Sleep(5 * time.Second)

	// 	testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "ping", ipv6EntryPrefix, Loopback126, vrfEncapA)
	// 	testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "traceroute", ipv6EntryPrefix, Loopback126, vrfEncapA)
	// 	args.interfaceaction(t, "port2", true)
	// 	args.interfaceaction(t, "port4", true)
	// 	args.interfaceaction(t, "port5", true)

	// })

	// t.Run("Ping, Traceroute test Ipv6 Frrs", func(t *testing.T) {
	// 	args.interfaceaction(t, "port2", false)
	// 	args.interfaceaction(t, "port4", false)
	// 	p1 := dut.Port(t, "port5")
	// 	testStats(t, dut, dut2, []string{p1.Name()}, "ip-ping", ipv6EntryPrefix, Loopback06, vrfEncapA)
	// 	testStats(t, dut, dut2, []string{p1.Name()}, "ip-traceroute", ipv6EntryPrefix, Loopback06, vrfEncapA, &Traceptions{Ip: "ipv6"})
	// 	args.interfaceaction(t, "port5", false)
	// 	time.Sleep(5 * time.Second)

	// 	testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "ip-ping", ipv6EntryPrefix, Loopback06, vrfEncapA)
	// 	testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "ip-traceroute", ipv6EntryPrefix, Loopback06, vrfEncapA, &Traceptions{Ip: "ipv6"})
	// 	args.interfaceaction(t, "port2", true)
	// 	args.interfaceaction(t, "port4", true)
	// 	args.interfaceaction(t, "port5", true)
	// })

	// t.Run("Ping, Traceroutes tests Ipv6inIp FRRs", func(t *testing.T) {
	// 	args.interfaceaction(t, "port2", false)
	// 	args.interfaceaction(t, "port4", false)
	// 	p1 := dut.Port(t, "port5")
	// 	time.Sleep(5 * time.Second)

	// 	testStats(t, dut, dut2, []string{p1.Name()}, "ping-ipinip", ipv6EntryPrefix, Loopback226, vrfEncapA)
	// 	testStats(t, dut, dut2, []string{p1.Name()}, "traceroute-ipinip", ipv6EntryPrefix, Loopback226, vrfEncapA)
	// 	args.interfaceaction(t, "port5", false)
	// 	time.Sleep(5 * time.Second)

	// 	testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "ping-ipinip", ipv6EntryPrefix, Loopback226, vrfEncapA)
	// 	testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "traceroute-ipinip", ipv6EntryPrefix, Loopback226, vrfEncapA)

	// 	args.interfaceaction(t, "port2", true)
	// 	args.interfaceaction(t, "port4", true)
	// 	args.interfaceaction(t, "port5", true)

	// })

	// t.Run("test with ttl 1 & different ttl", func(t *testing.T) {

	// 	testTraffic(t, args.ate, args.top, 5, true, false, false)

	// 	testTraffic(t, args.ate, args.top, 4, true, false, false)
	// })
	// t.Run("test with ttl 1 frrs", func(t *testing.T) {

	// 	testTraffic(t, args.ate, args.top, 5, true, true, false)

	// 	testTraffic(t, args.ate, args.top, 5, true, false, true)
	// 	args.interfaceaction(t, "port2", true)
	// 	args.interfaceaction(t, "port4", true)
	// 	args.interfaceaction(t, "port5", true)
	// })

	// t.Run("test with ttl different - frrs", func(t *testing.T) {

	// 	testTraffic(t, args.ate, args.top, 4, true, true, false)

	// 	testTraffic(t, args.ate, args.top, 4, true, false, true)
	// 	args.interfaceaction(t, "port2", true)
	// 	args.interfaceaction(t, "port4", true)
	// 	args.interfaceaction(t, "port5", true)

	// })

	// t.Run("test with ttl 1 & different ttl ipv6", func(t *testing.T) {

	// 	testTraffic(t, args.ate, args.top, 6, true, false, false)

	// 	testTraffic(t, args.ate, args.top, 7, true, false, false)

	// })
	// t.Run("test with ttl 1 frrs ipv6", func(t *testing.T) {

	// 	testTraffic(t, args.ate, args.top, 6, true, true, false)

	// 	testTraffic(t, args.ate, args.top, 6, true, false, true)

	// 	args.interfaceaction(t, "port2", true)
	// 	args.interfaceaction(t, "port4", true)
	// 	args.interfaceaction(t, "port5", true)
	// })

	// t.Run("test with ttl different - frrs ipv6", func(t *testing.T) {

	// 	testTraffic(t, args.ate, args.top, 7, true, true, false)

	// 	testTraffic(t, args.ate, args.top, 7, true, false, true)
	// 	args.interfaceaction(t, "port2", true)
	// 	args.interfaceaction(t, "port4", true)
	// 	args.interfaceaction(t, "port5", true)

	// })
	// defer args.interfaceaction(t, "port2", true)
	// defer args.interfaceaction(t, "port4", true)
	// defer args.interfaceaction(t, "port5", true)
	// // t.Run("testTraffic aftr rpfo", func(t *testing.T) {

	// // 	utils.Dorpfo(args.ctx, t, true)
	// // 	client = gribi.Client{
	// // 		DUT:                   args.dut,
	// // 		FibACK:                *ciscoFlags.GRIBIFIBCheck,
	// // 		Persistence:           true,
	// // 		InitialElectionIDLow:  1,
	// // 		InitialElectionIDHigh: 0,
	// // 	}
	// // 	if err := client.Start(t); err != nil {
	// // 		t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
	// // 		if err = client.Start(t); err != nil {
	// // 			t.Fatalf("gRIBI Connection could not be established: %v", err)
	// // 		}
	// // 	}
	// // 	args.client = &client
	// // 	testTraffic(t, args.ate, args.top, 100, true, false, false)
	// // 	testTrafficWeight(t, args.ate, args.top, 65000, true, 5)

	// // })
	// configurePort(t, dut, "Loopback22", Loopback12, Loopback126, 32, 128)

	// p3 = dut.Port(t, "port1")
	// p4 := dut.Port(t, "port9")
	// p5 := dut.Port(t, "port10")
	// unconfigbasePBR(t, dut, "PBR", []string{p3.Name(), p4.Name(), p5.Name()})

	// configPBR(t, dut, vrfRepaired, false)
	// configureIntfPBR(t, dut, "PBR", p3.Name())
	// configureIntfPBR(t, dut, "PBR", p4.Name())
	// configureIntfPBR(t, dut, "PBR", p5.Name())
	// configvrfInt(t, dut, vrfRepaired, "Loopback22")
	// ip4 := "197.51.100.2"
	// ip6 := "2556::2"

	// t.Run("Fallback Ping, traceroute test IpinIp", func(t *testing.T) {
	// 	p1 := dut.Port(t, "port2")
	// 	p2 := dut.Port(t, "port4")

	// 	testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping-ipinip", ip4, Loopback22, vrfEncapA)
	// 	testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute-ipinip", ip4, Loopback22, vrfEncapA)
	// })

	// t.Run("Fallback Ping, Traceroute test IpinIp FRRs", func(t *testing.T) {
	// 	p2 := dut.Port(t, "port3")
	// 	args.interfaceaction(t, "port2", false)
	// 	args.interfaceaction(t, "port4", false)
	// 	time.Sleep(5 * time.Second)

	// 	testStats(t, dut, dut2, []string{p2.Name()}, "ping-ipinip", ip4, Loopback22, vrfEncapA)
	// 	testStats(t, dut, dut2, []string{p2.Name()}, "traceroute-ipinip", ip4, Loopback22, vrfEncapA)

	// 	args.interfaceaction(t, "port2", true)
	// 	args.interfaceaction(t, "port4", true)
	// 	args.interfaceaction(t, "port5", true)

	// })

	// t.Run("Fallback Ping, Traceroute test Ipv6inIp", func(t *testing.T) {
	// 	p1 := dut.Port(t, "port2")
	// 	p2 := dut.Port(t, "port4")
	// 	testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping-ipinip", ip6, Loopback226, vrfEncapA)
	// 	testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute-ipinip", ip6, Loopback226, vrfEncapA)
	// })

	// t.Run("Fallback Ping, Traceroutes tests Ipv6inIp FRRs", func(t *testing.T) {
	// 	args.interfaceaction(t, "port2", false)
	// 	args.interfaceaction(t, "port4", false)

	// 	time.Sleep(5 * time.Second)

	// 	testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "ping-ipinip", ip6, Loopback226, vrfEncapA)
	// 	testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "traceroute-ipinip", ip6, Loopback226, vrfEncapA)

	// 	args.interfaceaction(t, "port2", true)
	// 	args.interfaceaction(t, "port4", true)
	// 	args.interfaceaction(t, "port5", true)

	// })

	// t.Run("Fallback test with ttl 1 & different ttl", func(t *testing.T) {

	// 	testTraffic(t, args.ate, args.top, 5, false, false, false)

	// 	testTraffic(t, args.ate, args.top, 4, false, false, false)
	// })
	// t.Run("Fallback test with ttl 1 frr", func(t *testing.T) {

	// 	testTraffic(t, args.ate, args.top, 5, false, true, true)
	// 	args.interfaceaction(t, "port2", true)
	// 	args.interfaceaction(t, "port4", true)
	// 	args.interfaceaction(t, "port5", true)
	// })

	// t.Run("Fallback test with ttl different - frr", func(t *testing.T) {

	// 	testTraffic(t, args.ate, args.top, 4, false, true, true)
	// 	args.interfaceaction(t, "port2", true)
	// 	args.interfaceaction(t, "port4", true)
	// 	args.interfaceaction(t, "port5", true)

	// })

	// t.Run("Fallback test with ttl 1 & different ttl ipv6", func(t *testing.T) {

	// 	testTraffic(t, args.ate, args.top, 6, false, false, false)

	// 	testTraffic(t, args.ate, args.top, 7, false, false, false)
	// 	args.interfaceaction(t, "port2", true)
	// 	args.interfaceaction(t, "port4", true)
	// 	args.interfaceaction(t, "port5", true)
	// })
	// t.Run("test with ttl 1 frrs ipv6", func(t *testing.T) {

	// 	testTraffic(t, args.ate, args.top, 6, true, true, true)
	// 	args.interfaceaction(t, "port2", true)
	// 	args.interfaceaction(t, "port4", true)
	// 	args.interfaceaction(t, "port5", true)
	// })

	// t.Run("test with ttl different - frrs ipv6", func(t *testing.T) {

	// 	testTraffic(t, args.ate, args.top, 7, true, true, false)

	// 	testTraffic(t, args.ate, args.top, 7, true, false, true)
	// 	args.interfaceaction(t, "port2", true)
	// 	args.interfaceaction(t, "port4", true)
	// 	args.interfaceaction(t, "port5", true)

	// })
	// p4 := dut.Port(t, "port2")
	// p5 := dut.Port(t, "port4")
	// p6 := dut.Port(t, "port6")
	// p7 := dut.Port(t, "port8")

	fmt.Println("traffic1")

	//for 5000 flows
	var outSrc, outDst, inSrc, inDst net.IP
	var c, c1 int
	k := 2
	for i := 1; i <= 1; i++ {
		if k >= 16 {
			k = 2
		}
		outSrc = net.IP{150, 150, uint8(rand.Intn(255-1) + 1), uint8(rand.Intn(255-1) + 1)}
		outDst = net.IP{198, 51, 100, uint8(rand.Intn(150-1) + 1)}
		if isIPv4 {

			inSrc = net.IP{153, 153, uint8(rand.Intn(255-1) + 1), uint8(rand.Intn(255-1) + 1)}
			inDst = net.IP{197, 51, uint8(rand.Intn(59-1) + 1), uint8(rand.Intn(255-1) + 1)}
			c = 5
			if flap == "flap" {
				//c = 5
				args.interfaceaction(t, "port4", false)
				args.interfaceaction(t, "port6", false)
				args.interfaceaction(t, "port8", false)
			} else if flap == "flap1" {
				c = 6
				args.interfaceaction(t, "port2", false)
			}
		} else {
			inDst = net.ParseIP("2555::")
			//for i := 8; i < 16; i++ {
			// "random" is within 2555::/32/16
			inDst[k] = uint8(rand.Intn(256))
			//}
			inSrc = net.ParseIP("6666::")
			inSrc[k] = uint8(rand.Intn(256))
			c = 2
			if flap == "flap" {
				//c = 22
				args.interfaceaction(t, "port4", false)
				args.interfaceaction(t, "port6", false)
				args.interfaceaction(t, "port8", false)
			} else if flap == "flap1" {
				c = 22
				args.interfaceaction(t, "port2", false)
			}
		}

		fmt.Println("dstaddresssout")
		fmt.Println(outDst)

		fmt.Println("outaddressssrc")
		fmt.Println(outSrc)

		fmt.Println("dstaddresss")
		fmt.Println(inDst)

		fmt.Println("dstaddresss")
		fmt.Println(inSrc)

		var tcpd, tcps, udpd, udps uint16
		tcpd = uint16(rand.Intn(60000-1) + 1)
		tcps = uint16(rand.Intn(65000-1) + 1)
		udpd = uint16(rand.Intn(7000-1) + 1)
		udps = uint16(rand.Intn(8000-1) + 1)

		fmt.Println("tcpppddd")
		fmt.Println(tcpd)

		fmt.Println("tcpppsss")
		fmt.Println(tcps)

		fmt.Println("udpdddd")
		fmt.Println(udpd)

		fmt.Println("udpsss")
		fmt.Println(udps)

		if encap {
			outDst = inDst
		}
		var portid, portidin, portIDe, portIDin string
		p1 := dut.Port(t, "port1")
		p2 := dut.Port(t, "port2")
		p3 := dut.Port(t, "port3")
		p4 := dut.Port(t, "port4")
		p5 := dut.Port(t, "port5")
		p6 := dut.Port(t, "port6")
		//p7 := dut.Port(t, "port7")
		p8 := dut.Port(t, "port8")

		var IDMap = map[string]string{
			p1.Name(): "10",
			p2.Name(): "11",
			p3.Name(): "12",
			p4.Name(): "13",
			p5.Name(): "14",
			p6.Name(): "15",
			p8.Name(): "16",
		}

		//
		fmt.Println("traffic2")
		if c == 5 || c == 6 {
			c1 = 6
		} else if c == 2 || c == 22 {
			c1 = 22
		}

		if len(opts) != 0 {
			for _, opt := range opts {
				if opt.ptcp == 4 {

					if flap == "flap" {
						portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcpd: tcpd, tcps: tcps})
						fmt.Println("portidbeforeshutegressfirstnextingress")
						fmt.Println(portid)
						fmt.Println(portidin)
						args.interfaceaction(t, "port4", false)
						args.interfaceaction(t, "port6", false)
						args.interfaceaction(t, "port8", false)
						portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcp: true, tcpd: tcpd, tcps: tcps})

					} else if flap == "flap1" {
						portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcp: true, tcpd: tcpd, tcps: tcps})
						fmt.Println("portidbeforeshutegressfirstnextingress")
						fmt.Println(portid)
						fmt.Println(portidin)
						args.interfaceaction(t, "port4", false)
						args.interfaceaction(t, "port6", false)
						args.interfaceaction(t, "port8", false)
						args.interfaceaction(t, "port2", false)

						//portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})
						portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcp: true, tcpd: tcpd, tcps: tcps})

					} else {

						//portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcp: true, tcpd: tcpd, tcps: tcps})

					}
				} else if opt.pudp == 8 {
					if flap == "flap" {
						portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true, udpd: udpd, udps: udps})
						fmt.Println("portidbeforeshutegressfirstnextingress")
						fmt.Println(portid)
						fmt.Println(portidin)
						args.interfaceaction(t, "port4", false)
						args.interfaceaction(t, "port6", false)
						args.interfaceaction(t, "port8", false)
						portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true, udpd: udpd, udps: udps})

					} else if flap == "flap1" {
						portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true, udpd: udpd, udps: udps})
						fmt.Println("portidbeforeshutegressfirstnextingress")
						fmt.Println(portid)
						fmt.Println(portidin)
						args.interfaceaction(t, "port4", false)
						args.interfaceaction(t, "port6", false)
						args.interfaceaction(t, "port8", false)
						//portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})
						portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true, udpd: udpd, udps: udps})

					} else {

						portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true, udpd: udpd, udps: udps})

					}
				}
			}
		} else {
			if flap == "flap" {
				portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})
				fmt.Println("portidbeforeshutegressfirstnextingress")
				fmt.Println(portid)
				fmt.Println(portidin)
				args.interfaceaction(t, "port4", false)
				args.interfaceaction(t, "port6", false)
				args.interfaceaction(t, "port8", false)
				portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})

			} else if flap == "flap1" {

				//portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})
				portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})

				fmt.Println("portidbeforeshutegressfirstnextingress")
				fmt.Println(portid)
				fmt.Println(portidin)
				args.interfaceaction(t, "port4", false)
				args.interfaceaction(t, "port6", false)
				args.interfaceaction(t, "port8", false)
				portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})

			} else {

				portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name()}, portinp: []string{p1.Name(), p3.Name()}})

				portIDe = IDMap[portid]
				portIDin = IDMap[portidin]
				fmt.Println("eggresswanttt")
				fmt.Println(portid)
				fmt.Println(portIDe)

				fmt.Println("ingresswanttt")
				fmt.Println(portidin)
				fmt.Println(portIDin)
			}
		}
		portIDe = IDMap[portid]
		portIDin = IDMap[portidin]

		// var portID, portIDin string
		// if portid != "" {
		// 	portID = IDMap[portid]
		// } else {
		// 	portID = "14"

		// }
		// if portidin != "" {
		// 	portIDin = IDMap[portidin]
		// }

		fmt.Println("eggresswant")
		fmt.Println(portid)
		fmt.Println(portIDe)

		fmt.Println("ingresswant")
		fmt.Println(portidin)
		fmt.Println(portIDin)

		// fmt.Println("devicepooorttttid")

		// fmt.Println(portid)
		var pkTOut int
		if len(opts) != 0 {
			for _, opt := range opts {
				if opt.ptcp == 4 {

					_, _, pkTOut = testTrafficc(t, args.ate, args.top, c1, true, false, 1, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{tcp: true, tcpd: tcpd, tcps: tcps})

				} else if opt.pudp == 8 {

					_, _, pkTOut = testTrafficc(t, args.ate, args.top, c1, true, false, 1, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{udp: true, udpd: udpd, udps: udps})

				}
			}
		} else {
			_, _, pkTOut = testTrafficc(t, args.ate, args.top, c1, true, false, 1, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String())

		}

		pktOut := pkTOut
		// if len(opts) != 0 {
		// 	for _, opt := range opts {
		// 		if opt.ptcp == 4 {
		// 			if flap == "flap" || flap == "flap1" {

		// 				//portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})
		// 				portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcp: true})

		// 			} else {

		// 				portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcp: true})

		// 			}
		// 		} else if opt.pudp == 8 {
		// 			if flap == "flap" || flap == "flap1" {

		// 				//portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})
		// 				portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true})

		// 			} else {

		// 				portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true})

		// 			}
		// 		}
		// 	}
		// } else {
		// 	if flap == "flap" || flap == "flap1" {

		// 		//portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})
		// 		portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})

		// 	} else {

		// 		portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name()}, portinp: []string{p1.Name(), p3.Name()}})

		// 		portIDe = IDMap[portid]
		// 		portIDin = IDMap[portidin]
		// 		fmt.Println("eggresswanttt")
		// 		fmt.Println(portid)
		// 		fmt.Println(portIDe)

		// 		fmt.Println("ingresswanttt")
		// 		fmt.Println(portidin)
		// 		fmt.Println(portIDin)
		// 	}
		// }
		// portIDe = IDMap[portid]
		// portIDin = IDMap[portidin]

		// // var portID, portIDin string
		// // if portid != "" {
		// // 	portID = IDMap[portid]
		// // } else {
		// // 	portID = "14"

		// // }
		// // if portidin != "" {
		// // 	portIDin = IDMap[portidin]
		// // }

		// fmt.Println("eggresswant")
		// fmt.Println(portid)
		// fmt.Println(portIDe)

		// fmt.Println("ingresswant")
		// fmt.Println(portidin)
		// fmt.Println(portIDin)

		// // fmt.Println("devicepooorttttid")

		// // fmt.Println(portid)
		// var pkTOut int
		// if len(opts) != 0 {
		// 	for _, opt := range opts {
		// 		if opt.ptcp == 4 {

		// 			_, _, pkTOut = testTrafficc(t, args.ate, args.top, c1, true, false, 1, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{tcp: true})

		// 		} else if opt.pudp == 8 {

		// 			_, _, pkTOut = testTrafficc(t, args.ate, args.top, c1, true, false, 1, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{udp: true})

		// 		}
		// 	}
		// } else {
		// 	_, _, pkTOut = testTrafficc(t, args.ate, args.top, c1, true, false, 1, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String())

		// }

		// pktOut := pkTOut
		fmt.Println("oooo2222pktcount")
		fmt.Println(pktOut)
		packetInTests := []struct {
			desc     string
			client   *p4rt_client.P4RTClient
			wantPkts int
		}{{
			desc:     "PacketIn to Primary Controller",
			client:   leader,
			wantPkts: pktOut,
		}, {
			desc:     "PacketIn to Secondary Controller",
			client:   follower,
			wantPkts: 0,
		}}

		t.Log("TTL/HopLimit 1")
		for _, test := range packetInTests {

			t.Run(te+test.desc, func(t *testing.T) {
				for i := 1; i <= 2; i++ {

					if i == 2 {
						if deviceSet {
							fmt.Println("2nd deviceid getpkts")
							stream = stream2
						} else {
							continue
						}
					}
					// Extract packets from PacketIn message sent to p4rt client
					//time.Sleep(20000 * time.Minute)
					_, packets, err := test.client.StreamChannelGetPackets(&stream, uint64(test.wantPkts), 90*time.Second)
					s := len(packets)
					fmt.Println("lengggggth")
					fmt.Println(s)

					if s != pktOut {
						fmt.Println("count mismatch")
					}
					if err != nil {
						t.Errorf("Unexpected error on fetchPackets: %v", err)
					}

					if test.wantPkts == 0 {
						return
					}

					gotPkts := 0
					t.Logf("Start to decode packet and compare with expected packets.")
					wantPacket := args.packetIO.GetPacketTemplate()
					fmt.Println("oooo7777")
					fmt.Println(wantPacket)
					//time.Sleep(20000 * time.Minute)
					for _, packet := range packets {
						if packet != nil {

							srcMAC, _, etherType := decodePacket(t, packet.Pkt.GetPayload())
							if etherType != layers.EthernetTypeIPv4 && etherType != layers.EthernetTypeIPv6 {
								continue
							}

							fmt.Println("sourcemac")
							fmt.Println(srcMAC)
							if !strings.EqualFold(srcMAC, tracerouteSrcMAC) {
								fmt.Println("sourcemac")
								fmt.Println(srcMAC)
								continue
							}
							// if wantPacket.TTL != nil {
							// 	//TTL/HopLimit comparison for IPV4 & IPV6
							// 	if isIPv4 {
							// 		captureTTL := decodePacket4(t, packet.Pkt.GetPayload())
							// 		if captureTTL != TTL1 {
							// 			t.Fatalf("Packet in PacketIn message is not matching wanted packet=IPV4 TTL1")
							// 		}

							// 	} else {
							// 		captureHopLimit := decodePacket6(t, packet.Pkt.GetPayload())
							// 		fmt.Println("hoppplimit")
							// 		fmt.Println(captureHopLimit)
							// 		if captureHopLimit != HopLimit1 {
							// 			t.Fatalf("Packet in PacketIn message is not matching wanted packet=IPV6 HopLimit1")
							// 		}
							// 	}
							// }

							//Metadata comparision
							if metaData := packet.Pkt.GetMetadata(); metaData != nil {
								if got := metaData[0].GetMetadataId(); got == METADATA_INGRESS_PORT {
									fmt.Println("xxxxx")
									t.Logf("Metadata ingress port: want metadatingress %d, got %d", METADATA_INGRESS_PORT, got)
									if gotPortID := string(metaData[0].GetValue()); gotPortID != portIDin {
										//t.Errorf("Ingress Port Id mismatch: want %s, got %s", args.packetIO.GetIngressPort(), gotPortID)
										t.Errorf("Ingress Port Id mismatch: want %s, got %s", portIDin, gotPortID)
										t.Logf("Mismatch for out src %s dst %s, inner src %s dst %s", outSrc, outDst, inSrc, inDst)

									} else {
										t.Logf("Match for out src %s dst %s, inner src %s dst %s", outSrc, outDst, inSrc, inDst)
									}

								} else {
									t.Errorf("Metadata ingress port mismatch: want %d, got %d", METADATA_INGRESS_PORT, got)
								}
								fmt.Println("2222222")
								t.Log(metaData)

								if got := metaData[1].GetMetadataId(); got == METADATA_EGRESS_PORT {
									if gotPortID := string(metaData[1].GetValue()); gotPortID != portIDe {
										//t.Errorf("Egress Port Id mismatch: want %s, got %s", args.packetIO.GetEgressPort(), gotPortID)
										t.Logf("Mismatch for out src %s dst %s, inner src %s dst %s", outSrc, outDst, inSrc, inDst)

										t.Errorf("Egress Port Id mismatch: want %s, got %s", portIDe, gotPortID)

									} else {
										t.Logf("Match for out src %s dst %s, inner src %s dst %s", outSrc, outDst, inSrc, inDst)
									}
								} else {
									t.Errorf("Metadata egress port mismatch: want %d, got %d", METADATA_EGRESS_PORT, got)
								}
							} else {
								t.Fatalf("Packet missing metadata information.")
							}
							gotPkts++
							i = 3
						}
					}
					if got, want := gotPkts, test.wantPkts; got != want {
						t.Errorf("Number of PacketIn, got: %d, want: %d", got, want)
						t.Logf(" Count Mismatch for out src %s dst %s, inner src %s dst %s", outSrc, outDst, inSrc, inDst)
					} else {
						t.Logf("Count match for out src %s dst %s, inner src %s dst %s", outSrc, outDst, inSrc, inDst)
					}
				}
			})

		}
		k++

	}
	// incPrefix(dstPfx)
	// incPrefix(ipv4FlowIP)
	// incPrefix(innerdst)
	// incPrefix(innersrc)

}

func testWithPoPUnoptimized(ctx context.Context, t *testing.T, args *testArgs, isIPv4, encap bool, flap, te string, deviceSet bool, opts ...*TOptions) {

	leader := args.leader
	follower := args.follower

	if isIPv4 {
		// Insert p4rtutils acl entry on the DUT
		if err := programmTableEntry(leader, args.packetIO, false, isIPv4, deviceId); err != nil {
			t.Fatalf("There is error when programming entry")
		}
		if deviceSet {

			if err := programmTableEntry(leader, args.packetIO, false, isIPv4, deviceId2); err != nil {
				t.Fatalf("There is error when programming entry")
			}
			defer programmTableEntry(leader, args.packetIO, true, isIPv4, deviceId2)

		}
		// Delete p4rtutils acl entry on the device
		defer programmTableEntry(leader, args.packetIO, true, isIPv4, deviceId)

	} else {
		// Insert p4rtutils acl entry on the DUT
		if err := programmTableEntry(leader, args.packetIO, false, false, deviceId); err != nil {
			t.Fatalf("There is error when programming entry")
		}

		if deviceSet {

			if err := programmTableEntry(leader, args.packetIO, false, false, deviceId2); err != nil {
				t.Fatalf("There is error when programming entry")
			}

			defer programmTableEntry(leader, args.packetIO, true, false, deviceId2)

		}
		// Delete p4rtutils acl entry on the device
		defer programmTableEntry(leader, args.packetIO, true, false, deviceId)

	}

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries, verify ping, traceroute triggers & ttl")

	dut := ondatra.DUT(t, "dut")
	//ctx = context.Background()
	baseconfig(t)

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
	configurePort(t, dut, "Loopback22", Loopback12, Loopback126, 32, 128)

	// p1 := dut.Port(t, "port9")
	// p2 := dut.Port(t, "port10")
	//p3 := dut.Port(t, "port1")

	//unconfigbasePBR(t, dut, "PBR", []string{p1.Name(), p2.Name(), p3.Name()})
	//unconfigbasePBR(t, dut, "PBR", []string{"Bundle-Ether120"})

	configPBR(t, dut, "PBR", true)
	configureIntfPBR(t, dut, "PBR", "Bundle-Ether120")
	//configureIntfPBR(t, dut, "PBR", p2.Name())
	//configureIntfPBR(t, dut, "PBR", p1.Name())
	configvrfInt(t, dut, vrfEncapA, "Loopback22")

	//dcunoptchain(t)
	//dut2 := ondatra.DUT(t, "dut2")
	// nh := 1
	// for i := 1; i <= 15; i++ {
	// 	//args.client.AddNH(t, uint64(nh), atePort4.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	// 	args.client.AddNH(t, uint64(nh), atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)

	// 	nh++
	// }
	// //nh = 1
	// for i := 1; i <= 15; i++ {
	// 	args.client.AddNH(t, uint64(nh), atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	// 	nh++
	// }

	// wt := 1
	// nh = 1
	// var i, j uint64
	// for i = 1; i <= 7; i++ {
	// 	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(i))

	// 	for j = 1; j <= 3; j++ {
	// 		if j == 1 {
	// 			wt = 3
	// 		} else if j == 2 {
	// 			wt = 25
	// 		} else {
	// 			wt = 31
	// 		}
	// 		nhg.AddNextHop(uint64(nh), uint64(wt))
	// 		args.client.Fluent(t).Modify().AddEntry(t, nhg)
	// 		nh++
	// 	}
	// }
	// nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(8))
	// nh = 22
	// for j = 1; j <= 9; j++ {
	// 	if j < 2 {
	// 		wt = 8
	// 	} else {
	// 		wt = 31
	// 	}
	// 	nhg.AddNextHop(uint64(nh), uint64(wt))
	// 	args.client.Fluent(t).Modify().AddEntry(t, nhg)
	// 	nh++
	// }

	// dstPfx2 := "205.205.205.1"
	// prefixes := []string{}
	// for i := 0; i < 8; i++ {
	// 	prefixes = append(prefixes, util.GetIPPrefix(dstPfx2, i, mask))
	// }
	// nhgID := 1
	// i = 1
	// for _, prefix := range prefixes {
	// 	args.client.AddIPv4(t, prefix, uint64(nhgID), *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	// 	nhgID++
	// }
	// nh = 1000
	// wt = 1

	// for _, prefix := range prefixes {
	// 	b := strings.Split(prefix, "/")
	// 	prefix = b[0]
	// 	args.client.AddNH(t, uint64(nh), prefix, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	// 	nh++
	// }
	// nhgi := 1000
	// nh = 1000
	// dstPfxt := "200.200.200.1"
	// prefixest := []string{}
	// for i := 0; i < 1; i++ {
	// 	prefixest = append(prefixest, util.GetIPPrefix(dstPfxt, i, mask))
	// }
	// NHEntry := fluent.NextHopEntry()
	// nhge := 20000
	// for _, prefix := range prefixest {
	// 	b := strings.Split(prefix, "/")
	// 	prefix = b[0]
	// 	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(nhge))
	// 	NHEntry = NHEntry.WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithIndex(uint64(nhge))
	// 	NHEntry = NHEntry.WithDecapsulateHeader(fluent.IPinIP)
	// 	NHEntry = NHEntry.WithEncapsulateHeader(fluent.IPinIP)
	// 	NHEntry = NHEntry.WithIPinIP("222.222.222.222", prefix)
	// 	NHEntry = NHEntry.WithNextHopNetworkInstance(vrfRepaired)
	// 	args.client.Fluent(t).Modify().AddEntry(t, NHEntry)
	// 	nhg.AddNextHop(uint64(nhge), uint64(256))
	// 	args.client.Fluent(t).Modify().AddEntry(t, nhg)
	// 	nhge++
	// }

	// nh = 1000
	// nhge = 20000
	// nhgi = 1000
	// nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(nhgi))
	// for j = 1000; j <= 1007; j++ {
	// 	if j == 1000 {
	// 		wt = 7
	// 	} else if j == 1001 {
	// 		wt = 9
	// 	} else {
	// 		wt = 40
	// 	}
	// 	nhg.AddNextHop(j, uint64(wt))
	// 	nhg.WithBackupNHG(uint64(nhge))
	// 	args.client.Fluent(t).Modify().AddEntry(t, nhg)
	// }

	// prefixese := []string{}
	// for i := 0; i < 15000; i++ {
	// 	prefixese = append(prefixese, util.GetIPPrefix(dstPfx, i, mask))
	// }

	// nhgi = 1000
	// for _, prefix := range prefixese {
	// 	ipv4Entry := fluent.IPv4Entry().
	// 		WithNetworkInstance("TE").
	// 		WithPrefix(prefix).
	// 		WithNextHopGroup(uint64(nhgi)).
	// 		WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	// 	args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	// }

	// args.client.AddNH(t, 30000, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	// args.client.AddNHG(t, 30000, 0, map[uint64]uint64{30000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// args.client.AddNH(t, uint64(30001), atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	// args.client.AddNH(t, uint64(30002), atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)

	// args.client.AddNHG(t, 31000, 30000, map[uint64]uint64{uint64(30001): 1, uint64(30002): 255}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// args.client.AddIPv4Batch(t, prefixest, 31000, vrfRepaired, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	nh := 1
	for i := 1; i <= 15; i++ {
		args.client.AddNH(t, uint64(nh), atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
		nh++
	}
	for i := 1; i <= 15; i++ {
		args.client.AddNH(t, uint64(nh), atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
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
	NHEntry = NHEntry.WithNextHopNetworkInstance(vrfRepair)
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
	args.client.AddNH(t, uint64(30001), atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, uint64(30002), atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 31000, 0, map[uint64]uint64{uint64(30001): 1, uint64(30002): 255}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, dsip+"/32", 31000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 31000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: "222.222.222.222", Dest: []string{dsip}})
	args.client.AddNHG(t, 32000, 30000, map[uint64]uint64{31000: 256}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixese, 32000, vrfRepair, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	fmt.Println("traffic1")

	args.client.DeleteIPv4Batch(t, prefixese, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	// for _, prefix := range prefixese {
	// 	ipv4Entry := fluent.IPv4Entry().
	// 		WithNetworkInstance("TE").
	// 		WithPrefix(prefix).
	// 		WithNextHopGroup(uint64(1000)).
	// 		WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	// 	args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	// }

	nhgi = 1000
	for _, prefix := range prefixese {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance("TE").
			WithPrefix(prefix).
			WithNextHopGroup(uint64(nhgi)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	}

	//for 5000 flows
	//var outSrc, outDst, inSrc, inDst net.IP
	var outSrc, outDst, inSrc, inDst net.IP
	var c, c1 int
	k := 2
	for i := 1; i <= 1; i++ {
		if k >= 16 {
			k = 2
		}
		outSrc = net.IP{150, 150, uint8(rand.Intn(255-1) + 1), uint8(rand.Intn(255-1) + 1)}
		outDst = net.IP{198, 51, 100, uint8(rand.Intn(150-1) + 1)}
		if isIPv4 {

			inSrc = net.IP{153, 153, uint8(rand.Intn(255-1) + 1), uint8(rand.Intn(255-1) + 1)}
			inDst = net.IP{197, 51, uint8(rand.Intn(59-1) + 1), uint8(rand.Intn(255-1) + 1)}
			c = 5
			if flap == "flap" {
				//c = 5
				args.interfaceaction(t, "port4", false)
				args.interfaceaction(t, "port6", false)
				args.interfaceaction(t, "port8", false)
			} else if flap == "flap1" {
				c = 6
				args.interfaceaction(t, "port2", false)
			}
		} else {
			inDst = net.ParseIP("2555::")
			//for i := 8; i < 16; i++ {
			// "random" is within 2555::/32/16
			inDst[k] = uint8(rand.Intn(256))
			//}
			inSrc = net.ParseIP("6666::")
			inSrc[k] = uint8(rand.Intn(256))
			c = 2
			if flap == "flap" {
				//c = 22
				args.interfaceaction(t, "port4", false)
				args.interfaceaction(t, "port6", false)
				args.interfaceaction(t, "port8", false)
			} else if flap == "flap1" {
				c = 22
				args.interfaceaction(t, "port2", false)
			}
		}

		fmt.Println("dstaddresssout")
		fmt.Println(outDst)

		fmt.Println("outaddressssrc")
		fmt.Println(outSrc)

		fmt.Println("dstaddresss")
		fmt.Println(inDst)

		fmt.Println("dstaddresss")
		fmt.Println(inSrc)

		var tcpd, tcps, udpd, udps uint16
		tcpd = uint16(rand.Intn(60000-1) + 1)
		tcps = uint16(rand.Intn(65000-1) + 1)
		udpd = uint16(rand.Intn(7000-1) + 1)
		udps = uint16(rand.Intn(8000-1) + 1)

		fmt.Println("tcpppddd")
		fmt.Println(tcpd)

		fmt.Println("tcpppsss")
		fmt.Println(tcps)

		fmt.Println("udpdddd")
		fmt.Println(udpd)

		fmt.Println("udpsss")
		fmt.Println(udps)

		if encap {
			outDst = inDst
		}
		var portid, portidin, portIDe, portIDin string
		p1 := dut.Port(t, "port1")
		p2 := dut.Port(t, "port2")
		p3 := dut.Port(t, "port3")
		p4 := dut.Port(t, "port4")
		p5 := dut.Port(t, "port5")
		p6 := dut.Port(t, "port6")
		//p7 := dut.Port(t, "port7")
		p8 := dut.Port(t, "port8")

		var IDMap = map[string]string{
			p1.Name(): "10",
			p2.Name(): "11",
			p3.Name(): "12",
			p4.Name(): "13",
			p5.Name(): "14",
			p6.Name(): "15",
			p8.Name(): "16",
		}

		//
		fmt.Println("traffic2")
		if c == 5 || c == 6 {
			c1 = 6
		} else if c == 2 || c == 22 {
			c1 = 22
		}

		if len(opts) != 0 {
			for _, opt := range opts {
				if opt.ptcp == 4 {

					if flap == "flap" {
						portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcpd: tcpd, tcps: tcps})
						fmt.Println("portidbeforeshutegressfirstnextingress")
						fmt.Println(portid)
						fmt.Println(portidin)
						args.interfaceaction(t, "port4", false)
						args.interfaceaction(t, "port6", false)
						args.interfaceaction(t, "port8", false)
						portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcp: true, tcpd: tcpd, tcps: tcps})

					} else if flap == "flap1" {
						portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcp: true, tcpd: tcpd, tcps: tcps})
						fmt.Println("portidbeforeshutegressfirstnextingress")
						fmt.Println(portid)
						fmt.Println(portidin)
						args.interfaceaction(t, "port4", false)
						args.interfaceaction(t, "port6", false)
						args.interfaceaction(t, "port8", false)
						args.interfaceaction(t, "port2", false)

						//portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})
						portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcp: true, tcpd: tcpd, tcps: tcps})

					} else {

						//portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcp: true, tcpd: tcpd, tcps: tcps})

					}
				} else if opt.pudp == 8 {
					if flap == "flap" {
						portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true, udpd: udpd, udps: udps})
						fmt.Println("portidbeforeshutegressfirstnextingress")
						fmt.Println(portid)
						fmt.Println(portidin)
						args.interfaceaction(t, "port4", false)
						args.interfaceaction(t, "port6", false)
						args.interfaceaction(t, "port8", false)
						portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true, udpd: udpd, udps: udps})

					} else if flap == "flap1" {
						portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true, udpd: udpd, udps: udps})
						fmt.Println("portidbeforeshutegressfirstnextingress")
						fmt.Println(portid)
						fmt.Println(portidin)
						args.interfaceaction(t, "port4", false)
						args.interfaceaction(t, "port6", false)
						args.interfaceaction(t, "port8", false)
						//portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})
						portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true, udpd: udpd, udps: udps})

					} else {

						portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true, udpd: udpd, udps: udps})

					}
				}
			}
		} else {
			if flap == "flap" {
				portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})
				fmt.Println("portidbeforeshutegressfirstnextingress")
				fmt.Println(portid)
				fmt.Println(portidin)
				args.interfaceaction(t, "port4", false)
				args.interfaceaction(t, "port6", false)
				args.interfaceaction(t, "port8", false)
				portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})

			} else if flap == "flap1" {

				//portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})
				portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})

				fmt.Println("portidbeforeshutegressfirstnextingress")
				fmt.Println(portid)
				fmt.Println(portidin)
				args.interfaceaction(t, "port4", false)
				args.interfaceaction(t, "port6", false)
				args.interfaceaction(t, "port8", false)
				portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})

			} else {

				//portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name()}, portinp: []string{p1.Name(), p3.Name()}})

				portIDe = IDMap[portid]
				portIDin = IDMap[portidin]
				fmt.Println("eggresswanttt")
				fmt.Println(portid)
				fmt.Println(portIDe)

				fmt.Println("ingresswanttt")
				fmt.Println(portidin)
				fmt.Println(portIDin)
			}
		}
		portIDe = IDMap[portid]
		portIDin = IDMap[portidin]

		// var portID, portIDin string
		// if portid != "" {
		// 	portID = IDMap[portid]
		// } else {
		// 	portID = "14"

		// }
		// if portidin != "" {
		// 	portIDin = IDMap[portidin]
		// }

		fmt.Println("eggresswant")
		fmt.Println(portid)
		fmt.Println(portIDe)

		fmt.Println("ingresswant")
		fmt.Println(portidin)
		fmt.Println(portIDin)

		// fmt.Println("devicepooorttttid")

		// fmt.Println(portid)
		var pkTOut int
		if len(opts) != 0 {
			for _, opt := range opts {
				if opt.ptcp == 4 {

					_, _, pkTOut = testTrafficc(t, args.ate, args.top, c1, true, false, 1, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{tcp: true, tcpd: tcpd, tcps: tcps})

				} else if opt.pudp == 8 {

					_, _, pkTOut = testTrafficc(t, args.ate, args.top, c1, true, false, 1, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{udp: true, udpd: udpd, udps: udps})

				}
			}
		} else {
			_, _, pkTOut = testTrafficc(t, args.ate, args.top, c1, true, false, 1, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String())

		}

		pktOut := pkTOut
		// if len(opts) != 0 {
		// 	for _, opt := range opts {
		// 		if opt.ptcp == 4 {
		// 			if flap == "flap" || flap == "flap1" {

		// 				//portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})
		// 				portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcp: true})

		// 			} else {

		// 				portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcp: true})

		// 			}
		// 		} else if opt.pudp == 8 {
		// 			if flap == "flap" || flap == "flap1" {

		// 				//portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})
		// 				portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true})

		// 			} else {

		// 				portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true})

		// 			}
		// 		}
		// 	}
		// } else {
		// 	if flap == "flap" || flap == "flap1" {

		// 		//portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})
		// 		portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})

		// 	} else {

		// 		portid, portidin, _ = testTrafficc(t, args.ate, args.top, c, true, true, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name()}, portinp: []string{p1.Name(), p3.Name()}})

		// 		portIDe = IDMap[portid]
		// 		portIDin = IDMap[portidin]
		// 		fmt.Println("eggresswanttt")
		// 		fmt.Println(portid)
		// 		fmt.Println(portIDe)

		// 		fmt.Println("ingresswanttt")
		// 		fmt.Println(portidin)
		// 		fmt.Println(portIDin)
		// 	}
		// }
		// portIDe = IDMap[portid]
		// portIDin = IDMap[portidin]

		// // var portID, portIDin string
		// // if portid != "" {
		// // 	portID = IDMap[portid]
		// // } else {
		// // 	portID = "14"

		// // }
		// // if portidin != "" {
		// // 	portIDin = IDMap[portidin]
		// // }

		// fmt.Println("eggresswant")
		// fmt.Println(portid)
		// fmt.Println(portIDe)

		// fmt.Println("ingresswant")
		// fmt.Println(portidin)
		// fmt.Println(portIDin)

		// // fmt.Println("devicepooorttttid")

		// // fmt.Println(portid)
		// var pkTOut int
		// if len(opts) != 0 {
		// 	for _, opt := range opts {
		// 		if opt.ptcp == 4 {

		// 			_, _, pkTOut = testTrafficc(t, args.ate, args.top, c1, true, false, 1, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{tcp: true})

		// 		} else if opt.pudp == 8 {

		// 			_, _, pkTOut = testTrafficc(t, args.ate, args.top, c1, true, false, 1, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{udp: true})

		// 		}
		// 	}
		// } else {
		// 	_, _, pkTOut = testTrafficc(t, args.ate, args.top, c1, true, false, 1, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String())

		// }

		// pktOut := pkTOut
		fmt.Println("oooo2222pktcount")
		fmt.Println(pktOut)
		packetInTests := []struct {
			desc     string
			client   *p4rt_client.P4RTClient
			wantPkts int
		}{{
			desc:     "PacketIn to Primary Controller",
			client:   leader,
			wantPkts: pktOut,
		}, {
			desc:     "PacketIn to Secondary Controller",
			client:   follower,
			wantPkts: 0,
		}}

		t.Log("TTL/HopLimit 1")
		for _, test := range packetInTests {

			t.Run(te+test.desc, func(t *testing.T) {
				for i := 1; i <= 2; i++ {

					if i == 2 {
						if deviceSet {
							fmt.Println("2nd deviceid getpkts")
							stream = stream2
						} else {
							continue
						}
					}
					// Extract packets from PacketIn message sent to p4rt client
					//time.Sleep(20000 * time.Minute)
					_, packets, err := test.client.StreamChannelGetPackets(&stream, uint64(test.wantPkts), 90*time.Second)
					s := len(packets)
					fmt.Println("lengggggth")
					fmt.Println(s)

					if s != pktOut {
						fmt.Println("count mismatch")
					}
					if err != nil {
						t.Errorf("Unexpected error on fetchPackets: %v", err)
					}

					if test.wantPkts == 0 {
						return
					}

					gotPkts := 0
					t.Logf("Start to decode packet and compare with expected packets.")
					wantPacket := args.packetIO.GetPacketTemplate()
					fmt.Println("oooo7777")
					fmt.Println(wantPacket)
					//time.Sleep(20000 * time.Minute)
					for _, packet := range packets {
						if packet != nil {

							srcMAC, _, etherType := decodePacket(t, packet.Pkt.GetPayload())
							if etherType != layers.EthernetTypeIPv4 && etherType != layers.EthernetTypeIPv6 {
								continue
							}

							fmt.Println("sourcemac")
							fmt.Println(srcMAC)
							if !strings.EqualFold(srcMAC, tracerouteSrcMAC) {
								fmt.Println("sourcemac")
								fmt.Println(srcMAC)
								continue
							}
							// if wantPacket.TTL != nil {
							// 	//TTL/HopLimit comparison for IPV4 & IPV6
							// 	if isIPv4 {
							// 		captureTTL := decodePacket4(t, packet.Pkt.GetPayload())
							// 		if captureTTL != TTL1 {
							// 			t.Fatalf("Packet in PacketIn message is not matching wanted packet=IPV4 TTL1")
							// 		}

							// 	} else {
							// 		captureHopLimit := decodePacket6(t, packet.Pkt.GetPayload())
							// 		fmt.Println("hoppplimit")
							// 		fmt.Println(captureHopLimit)
							// 		if captureHopLimit != HopLimit1 {
							// 			t.Fatalf("Packet in PacketIn message is not matching wanted packet=IPV6 HopLimit1")
							// 		}
							// 	}
							// }

							//Metadata comparision
							if metaData := packet.Pkt.GetMetadata(); metaData != nil {
								if got := metaData[0].GetMetadataId(); got == METADATA_INGRESS_PORT {
									fmt.Println("xxxxx")
									t.Logf("Metadata ingress port: want metadatingress %d, got %d", METADATA_INGRESS_PORT, got)
									if gotPortID := string(metaData[0].GetValue()); gotPortID != portIDin {
										//t.Errorf("Ingress Port Id mismatch: want %s, got %s", args.packetIO.GetIngressPort(), gotPortID)
										t.Errorf("Ingress Port Id mismatch: want %s, got %s", portIDin, gotPortID)
										t.Logf("Mismatch for out src %s dst %s, inner src %s dst %s", outSrc, outDst, inSrc, inDst)

									} else {
										t.Logf("Match for out src %s dst %s, inner src %s dst %s", outSrc, outDst, inSrc, inDst)
									}

								} else {
									t.Errorf("Metadata ingress port mismatch: want %d, got %d", METADATA_INGRESS_PORT, got)
								}
								fmt.Println("2222222")
								t.Log(metaData)

								if got := metaData[1].GetMetadataId(); got == METADATA_EGRESS_PORT {
									if gotPortID := string(metaData[1].GetValue()); gotPortID != portIDe {
										//t.Errorf("Egress Port Id mismatch: want %s, got %s", args.packetIO.GetEgressPort(), gotPortID)
										t.Logf("Mismatch for out src %s dst %s, inner src %s dst %s", outSrc, outDst, inSrc, inDst)

										t.Errorf("Egress Port Id mismatch: want %s, got %s", portIDe, gotPortID)

									} else {
										t.Logf("Match for out src %s dst %s, inner src %s dst %s", outSrc, outDst, inSrc, inDst)
									}
								} else {
									t.Errorf("Metadata egress port mismatch: want %d, got %d", METADATA_EGRESS_PORT, got)
								}
							} else {
								t.Fatalf("Packet missing metadata information.")
							}
							gotPkts++
							i = 3
						}
					}
					if got, want := gotPkts, test.wantPkts; got != want {
						t.Errorf("Number of PacketIn, got: %d, want: %d", got, want)
						t.Logf(" Count Mismatch for out src %s dst %s, inner src %s dst %s", outSrc, outDst, inSrc, inDst)
					} else {
						t.Logf("Count match for out src %s dst %s, inner src %s dst %s", outSrc, outDst, inSrc, inDst)
					}
				}
			})

		}
		k++

	}
	// incPrefix(dstPfx)
	// incPrefix(ipv4FlowIP)
	// incPrefix(innerdst)
	// incPrefix(innersrc)

}

// func GetIPPrefix(IPAddr string, i int, prefixLen string) string {
// 	ip := net.ParseIP(IPAddr)
// 	ip = ip.To4()
// 	ip[3] = ip[3] + byte(i%256)
// 	ip[2] = ip[2] + byte(i/256)
// 	ip[1] = ip[1] + byte(i/(256*256))
// 	return ip.String() + "/" + prefixLen
// }

func dcchainconfig(t *testing.T) {
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
		NHEntry = NHEntry.WithNextHopNetworkInstance(vrfRepaired)
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

	args.client.AddIPv4(t, "209.209.209.1/32", uint64(31000), *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, uint64(30003), "209.209.209.1", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 33000, 0, map[uint64]uint64{uint64(30003): 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddIPv4Batch(t, prefixest, 33000, vrfRepaired, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	nh = 3001
	for _, prefix := range prefixese {
		b := strings.Split(prefix, "/")
		prefix = b[0]
		args.client.AddNH(t, uint64(nh), "Encap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: []string{prefix}, VrfName: vrf1})
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
	args.client.AddIPv6(t, ipv6EntryPrefix+"/128", uint64(3000), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

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
}

// func dcunoptchain(t *testing.T) {
// 	nh := 1
// 	for i := 1; i <= 15; i++ {
// 		args.client.AddNH(t, uint64(nh), atePort2.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
// 		nh++
// 	}
// 	for i := 1; i <= 15; i++ {
// 		args.client.AddNH(t, uint64(nh), atePort4.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
// 		nh++
// 	}
// 	nh = 1
// 	for i := 1; i <= 30; i++ {
// 		args.client.AddNH(t, uint64(nh), atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
// 		nh++
// 	}

// 	wt := 1
// 	nh = 1
// 	var i, j uint64
// 	for i = 1; i <= 7; i++ {
// 		nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(i))

// 		for j = 1; j <= 3; j++ {
// 			if j == 1 {
// 				wt = 3
// 			} else if j == 2 {
// 				wt = 25
// 			} else {
// 				wt = 31
// 			}
// 			nhg.AddNextHop(uint64(nh), uint64(wt))
// 			args.client.Fluent(t).Modify().AddEntry(t, nhg)
// 			nh++
// 		}
// 	}
// 	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(8))
// 	nh = 22
// 	for j = 1; j <= 9; j++ {
// 		if j < 2 {
// 			wt = 8
// 		} else {
// 			wt = 31
// 		}
// 		nhg.AddNextHop(uint64(nh), uint64(wt))
// 		args.client.Fluent(t).Modify().AddEntry(t, nhg)
// 		nh++
// 	}

// 	dstPfx2 := "205.205.205.1"
// 	prefixes := []string{}
// 	for i := 0; i < 8; i++ {
// 		prefixes = append(prefixes, util.GetIPPrefix(dstPfx2, i, mask))
// 	}
// 	nhgID := 1
// 	i = 1
// 	for _, prefix := range prefixes {
// 		args.client.AddIPv4(t, prefix, uint64(nhgID), *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
// 		nhgID++
// 	}
// 	nh = 1000
// 	wt = 1

// 	for _, prefix := range prefixes {
// 		b := strings.Split(prefix, "/")
// 		prefix = b[0]
// 		args.client.AddNH(t, uint64(nh), prefix, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
// 		nh++
// 	}
// 	nhgi := 1000

// 	nh = 1000
// 	NHEntry := fluent.NextHopEntry()
// 	nhge := 20000
// 	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID((uint64(nhge)))
// 	NHEntry = NHEntry.WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithIndex(uint64(nhge))
// 	NHEntry = NHEntry.WithNextHopNetworkInstance(vrfRepair)
// 	args.client.Fluent(t).Modify().AddEntry(t, NHEntry)
// 	nhg.AddNextHop(uint64(nhge), uint64(256))
// 	args.client.Fluent(t).Modify().AddEntry(t, nhg)
// 	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(nhgi))

// 	for j = 1; j <= 3; j++ {
// 		if j == 1 {
// 			wt = 101
// 		} else if j == 2 {
// 			wt = 52
// 		} else {
// 			wt = 83
// 		}
// 		nhg.AddNextHop(uint64(nh), uint64(wt))
// 		nhg.WithBackupNHG(uint64(nhge))
// 		args.client.Fluent(t).Modify().AddEntry(t, nhg)
// 		nh++
// 	}
// 	nhgi = 1001
// 	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(nhgi))

// 	for j = 1; j <= 5; j++ {
// 		if j == 1 {
// 			wt = 132
// 		} else {
// 			wt = 31
// 		}
// 		nhg.AddNextHop(uint64(nh), uint64(wt))
// 		nhg.WithBackupNHG(uint64(nhge))
// 		args.client.Fluent(t).Modify().AddEntry(t, nhg)
// 		nh++
// 	}
// 	nhgi = 1000

// 	prefixese := []string{}
// 	dstPfxe := "170.170.170.1"
// 	for i := 0; i < 2; i++ {
// 		prefixese = append(prefixese, util.GetIPPrefix(dstPfxe, i, mask))
// 	}

// 	nhgi = 1000
// 	for _, prefix := range prefixese {
// 		ipv4Entry := fluent.IPv4Entry().
// 			WithNetworkInstance("TE").
// 			WithPrefix(prefix).
// 			WithNextHopGroup(uint64(nhgi)).
// 			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
// 		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
// 		nhgi++
// 	}

// 	args.client.AddNH(t, 30000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
// 	args.client.AddNHG(t, 30000, 0, map[uint64]uint64{30000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
// 	args.client.AddNH(t, uint64(30001), atePort5.ip(uint8(1)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
// 	args.client.AddNH(t, uint64(30002), atePort5.ip(uint8(2)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
// 	args.client.AddNHG(t, 31000, 0, map[uint64]uint64{uint64(30001): 1, uint64(30002): 255}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
// 	args.client.AddIPv4(t, dsip+"/32", 31000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
// 	args.client.AddNH(t, 31000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: "222.222.222.222", Dest: []string{dsip}})
// 	args.client.AddNHG(t, 32000, 30000, map[uint64]uint64{31000: 256}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
// 	args.client.AddIPv4Batch(t, prefixese, 32000, vrfRepair, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

// 	nh = 3001
// 	for _, prefix := range prefixese {
// 		b := strings.Split(prefix, "/")
// 		prefix = b[0]
// 		args.client.AddNH(t, uint64(nh), "Encap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: []string{prefix}, VrfName: "TE"})
// 		nh++
// 	}

// 	nhg = fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(3000))

// 	for j = 3001; j < 3003; j++ {
// 		if j == 3001 {
// 			wt = 15
// 		} else {
// 			wt = 241
// 		}
// 		nhg.AddNextHop(j, uint64(wt))
// 		args.client.Fluent(t).Modify().AddEntry(t, nhg)
// 	}

// 	dstPfxe = "197.51.100.1"
// 	prefixess := []string{}
// 	for i := 0; i < 15000; i++ {
// 		prefixess = append(prefixess, util.GetIPPrefix(dstPfxe, i, mask))
// 	}

// 	for _, prefix := range prefixess {
// 		ipv4Entry := fluent.IPv4Entry().
// 			WithNetworkInstance(vrfEncapA).
// 			WithPrefix(prefix).
// 			WithNextHopGroup(uint64(3000)).
// 			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
// 		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
// 	}
// 	args.client.AddIPv6(t, ipv6EntryPrefix+"/128", uint64(3000), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

// 	nh = 4000
// 	args.client.AddNH(t, uint64(nh), "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{VrfName: vrfEncapA})
// 	args.client.AddNHG(t, 4000, 0, map[uint64]uint64{4000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

// 	prefixesd := []string{}
// 	for i := 0; i < 150; i++ {
// 		prefixesd = append(prefixesd, util.GetIPPrefix(dstPfx, i, mask))
// 	}

// 	for _, prefix := range prefixesd {
// 		ipv4Entry := fluent.IPv4Entry().
// 			WithNetworkInstance(vrfDecap).
// 			WithPrefix(prefix).
// 			WithNextHopGroup(uint64(4000)).
// 			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
// 		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
// 	}

// }

func popgateconfig(t *testing.T) {
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
		NHEntry = NHEntry.WithNextHopNetworkInstance(vrfRepaired)
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

	args.client.AddIPv4Batch(t, prefixest, 31000, vrfRepaired, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

}

func popunoptchain(t *testing.T) {
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
	NHEntry = NHEntry.WithNextHopNetworkInstance(vrfRepair)
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
	args.client.AddIPv4Batch(t, prefixese, 32000, vrfRepair, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

}
