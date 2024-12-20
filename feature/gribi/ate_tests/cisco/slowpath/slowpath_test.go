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

package slowpath_test

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/openconfig/featureprofiles/internal/cisco/ha/utils"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/gribigo/fluent"

	"context"

	"testing"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/gribi"
	"github.com/openconfig/featureprofiles/internal/deviations"

	"github.com/openconfig/featureprofiles/internal/fptest"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	IpEncap = "197.51.100.11"
	IpTE    = "198.51.100.1"

	Loopback22            = "88.88.88.88"
	Loopback226           = "8888::88"
	Loopback12            = "44.44.44.44"
	Loopback126           = "4444::44"
	Loopback0             = "30.30.30.30"
	Loopback06            = "30::30"
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
	bundleEther126        = "Bundle-Ether126"
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

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the test
// topology.
//
// The testbed consists of dut:port1 -> ate:port1,
// dut:port2 -> ate:port2, dut:port3 -> ate:port3, dut:port4 -> ate:port4, dut:port5 -> ate:port5,
// dut:port6 -> ate:port6, dut:port7 -> ate:port7 ,dut:port8 -> ate:port8

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
		t.Log("in baseconfig")

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

func TestWithDCBackUp(t *testing.T) {

	// // Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with decapencap/decap, verify traffic, reprogram & delete ipv4/NHG/NH")
	dut2 := ondatra.DUT(t, "dut2")

	configDUT(t, dut2)

	ctx2 := context.Background()
	args = &testArgs{}

	// Configure the gRIBI client
	client := gribi.Client{
		DUT:                   dut2,
		FibACK:                *ciscoFlags.GRIBIFIBCheck,
		Persistence:           true,
		InitialElectionIDLow:  10,
		InitialElectionIDHigh: 0,
	}
	defer client.Close(t)
	if err := client.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}

	args.ctx = ctx2
	args.client = &client
	args.dut = dut2
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)
	nh := 1
	for i := 1; i <= 2; i++ {
		args.client.AddNH(t, uint64(nh), "192.0.9.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	}
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{1: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "206.206.206.22/32", 101, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	nh = 1000
	for i := 1; i <= 2; i++ {
		args.client.AddNH(t, uint64(nh), "206.206.206.22", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	}
	args.client.AddNHG(t, 102, 0, map[uint64]uint64{1000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	ipv4Entry := fluent.IPv4Entry().
		WithNetworkInstance("TE").
		WithPrefix("198.51.100.11/32").
		WithNextHopGroup(uint64(102)).
		WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	args.client.AddNH(t, uint64(3000), "Encap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: []string{"198.51.100.11"}, VrfName: "TE"})
	args.client.AddNHG(t, 103, 0, map[uint64]uint64{3000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	ipv4Entry = fluent.IPv4Entry().
		WithNetworkInstance(vrfEncapA).
		WithPrefix("197.51.100.11/32").
		WithNextHopGroup(uint64(103)).
		WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)

	args.client.AddIPv6(t, ipv6EntryPrefix+"/128", uint64(103), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	ipv4Entry = fluent.IPv4Entry().
		WithNetworkInstance("TE").
		WithPrefix("196.51.100.2/32").
		WithNextHopGroup(uint64(102)).
		WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	args.client.AddNH(t, uint64(3001), "Encap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: []string{"196.51.100.2"}, VrfName: "TE"})
	args.client.AddNHG(t, 104, 0, map[uint64]uint64{3001: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	ipv4Entry = fluent.IPv4Entry().
		WithNetworkInstance(vrfEncapA).
		WithPrefix("197.51.100.2/32").
		WithNextHopGroup(uint64(104)).
		WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)

	args.client.AddIPv6(t, "2556::2/128", uint64(104), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	args = &testArgs{}

	baseconfig(t)
	configurePort(t, dut, "Loopback22", Loopback12, Loopback126, 32, 128)

	p1 := dut.Port(t, "port9")
	configurePort(t, dut, p1.Name(), "192.0.9.2", "7777::2", 30, 126)

	p2 := dut.Port(t, "port10")
	configurePort(t, dut, p2.Name(), "192.0.10.2", "192:0:2::1E", 30, 126)
	p3 := dut.Port(t, "port1")

	unconfigbasePBR(t, dut, "PBR", []string{p1.Name(), p2.Name(), p3.Name()})

	configPBR(t, dut, "PBR", true)
	configureIntfPBR(t, dut, "PBR", p3.Name())
	configureIntfPBR(t, dut, "PBR", p2.Name())
	configureIntfPBR(t, dut, "PBR", p1.Name())

	configvrfInt(t, dut, vrfEncapA, "Loopback22")

	configvrfInt(t, dut, vrfEncapA, p2.Name())
	staticvrf(t, dut, vrfEncapA, "192.0.10.1", "192:0:2::1d")
	staticvrf(t, dut, "DEFAULT", "192.0.9.1", "192:0:2::1a")

	// Configure the gRIBI client
	client = gribi.Client{
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

	dcchainconfig(t)
	args.client.AddNH(t, 22200, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 22200, 0, map[uint64]uint64{22200: 20}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	nhge := 22200
	nhgi := 2220
	nh = 1000
	var wt int

	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(2220))

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
			WithNetworkInstance("REPAIRED").
			WithPrefix(prefix).
			WithNextHopGroup(uint64(nhgi)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	}
	args.client.AddIPv6(t, "2555::2/128", uint64(nhgi), "REPAIRED", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	for k := 1; k <= 2; k++ {
		t.Run("Ping, Traceroute test", func(t *testing.T) {
			p1 = dut.Port(t, "port2")
			p2 = dut.Port(t, "port4")

			testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping", IpEncap, Loopback12, vrfEncapA)
			testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute", IpEncap, Loopback12, vrfEncapA)
		})

		t.Run("Ping, Traceroute test Ip", func(t *testing.T) {
			p1 := dut.Port(t, "port2")
			p2 := dut.Port(t, "port4")

			testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ip-ping", IpEncap, Loopback0, vrfEncapA)
			testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ip-traceroute", IpEncap, Loopback0, vrfEncapA, &Traceptions{Ip: "ip"})
		})
		t.Run("Ping, Traceroute test IpinIp", func(t *testing.T) {
			p1 = dut.Port(t, "port2")
			p2 = dut.Port(t, "port4")

			testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping-ipinip", IpEncap, Loopback22, vrfEncapA)
			testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute-ipinip", IpEncap, Loopback22, vrfEncapA)
		})
		t.Run("Ping, Traceroute test FRRs", func(t *testing.T) {
			args.interfaceaction(t, "port2", false)
			args.interfaceaction(t, "port4", false)

			p1 = dut.Port(t, "port5")

			testStats(t, dut, dut2, []string{p1.Name()}, "ping", IpEncap, Loopback12, vrfEncapA)
			testStats(t, dut, dut2, []string{p1.Name()}, "traceroute", IpEncap, Loopback12, vrfEncapA)

			args.interfaceaction(t, "port5", false)
			time.Sleep(5 * time.Second)
			p2 = dut.Port(t, "port3")
			testStats(t, dut, dut2, []string{p2.Name()}, "ping", IpEncap, Loopback12, vrfEncapA)
			testStats(t, dut, dut2, []string{p2.Name()}, "traceroute", IpEncap, Loopback12, vrfEncapA)

			args.interfaceaction(t, "port2", true)
			args.interfaceaction(t, "port4", true)
			args.interfaceaction(t, "port5", true)

		})

		t.Run("Ping, Traceroute test Ip Frrs", func(t *testing.T) {
			args.interfaceaction(t, "port2", false)
			args.interfaceaction(t, "port4", false)
			p1 = dut.Port(t, "port5")
			p2 = dut.Port(t, "port3")

			testStats(t, dut, dut2, []string{p1.Name()}, "ip-ping", IpEncap, Loopback0, vrfEncapA)
			testStats(t, dut, dut2, []string{p1.Name()}, "ip-traceroute", IpEncap, Loopback0, vrfEncapA, &Traceptions{Ip: "ip"})
			args.interfaceaction(t, "port5", false)
			time.Sleep(5 * time.Second)

			testStats(t, dut, dut2, []string{p2.Name()}, "ip-ping", IpEncap, Loopback0, vrfEncapA)
			testStats(t, dut, dut2, []string{p2.Name()}, "ip-traceroute", IpEncap, Loopback0, vrfEncapA, &Traceptions{Ip: "ip"})
			args.interfaceaction(t, "port2", true)
			args.interfaceaction(t, "port4", true)
			args.interfaceaction(t, "port5", true)

		})

		t.Run("Ping, Traceroute test IpinIp FRRs", func(t *testing.T) {
			p1 = dut.Port(t, "port5")
			p2 = dut.Port(t, "port3")
			args.interfaceaction(t, "port2", false)
			args.interfaceaction(t, "port4", false)
			testStats(t, dut, dut2, []string{p1.Name()}, "ping-ipinip", IpEncap, Loopback22, vrfEncapA)
			testStats(t, dut, dut2, []string{p1.Name()}, "traceroute-ipinip", IpEncap, Loopback22, vrfEncapA)
			args.interfaceaction(t, "port5", false)
			time.Sleep(5 * time.Second)

			testStats(t, dut, dut2, []string{p2.Name()}, "ping-ipinip", IpEncap, Loopback22, vrfEncapA)
			testStats(t, dut, dut2, []string{p2.Name()}, "traceroute-ipinip", IpEncap, Loopback22, vrfEncapA)

			args.interfaceaction(t, "port2", true)
			args.interfaceaction(t, "port4", true)
			args.interfaceaction(t, "port5", true)

		})

		t.Run("Ping, Traceroute test ipv6", func(t *testing.T) {
			p1 := dut.Port(t, "port2")
			p2 := dut.Port(t, "port4")
			testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping", ipv6EntryPrefix, Loopback126, vrfEncapA)
			testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute", ipv6EntryPrefix, Loopback126, vrfEncapA)
		})

		t.Run("Ping, Traceroute test Ipv6", func(t *testing.T) {
			p1 = dut.Port(t, "port2")
			p2 = dut.Port(t, "port4")
			testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ip-ping", ipv6EntryPrefix, Loopback06, vrfEncapA)
			testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ip-traceroute", ipv6EntryPrefix, Loopback06, vrfEncapA, &Traceptions{Ip: "ipv6"})
		})
		t.Run("Ping, Traceroute test Ipv6inIp", func(t *testing.T) {
			p1 = dut.Port(t, "port2")
			p2 = dut.Port(t, "port4")
			testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping-ipinip", ipv6EntryPrefix, Loopback226, vrfEncapA)
			testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute-ipinip", ipv6EntryPrefix, Loopback226, vrfEncapA)
		})
		t.Run("Ping, Traceroute tests  ipv6 FRRs", func(t *testing.T) {
			args.interfaceaction(t, "port2", false)
			args.interfaceaction(t, "port4", false)
			p1 = dut.Port(t, "port5")

			testStats(t, dut, dut2, []string{p1.Name()}, "ping", ipv6EntryPrefix, Loopback126, vrfEncapA)
			testStats(t, dut, dut2, []string{p1.Name()}, "traceroute", ipv6EntryPrefix, Loopback126, vrfEncapA)

			args.interfaceaction(t, "port5", false)
			time.Sleep(5 * time.Second)

			testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "ping", ipv6EntryPrefix, Loopback126, vrfEncapA)
			testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "traceroute", ipv6EntryPrefix, Loopback126, vrfEncapA)
			args.interfaceaction(t, "port2", true)
			args.interfaceaction(t, "port4", true)
			args.interfaceaction(t, "port5", true)

		})

		t.Run("Ping, Traceroute test Ipv6 Frrs", func(t *testing.T) {
			args.interfaceaction(t, "port2", false)
			args.interfaceaction(t, "port4", false)
			p1 = dut.Port(t, "port5")
			testStats(t, dut, dut2, []string{p1.Name()}, "ip-ping", ipv6EntryPrefix, Loopback06, vrfEncapA)
			testStats(t, dut, dut2, []string{p1.Name()}, "ip-traceroute", ipv6EntryPrefix, Loopback06, vrfEncapA, &Traceptions{Ip: "ipv6"})
			args.interfaceaction(t, "port5", false)
			time.Sleep(5 * time.Second)

			testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "ip-ping", ipv6EntryPrefix, Loopback06, vrfEncapA)
			testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "ip-traceroute", ipv6EntryPrefix, Loopback06, vrfEncapA, &Traceptions{Ip: "ipv6"})
			args.interfaceaction(t, "port2", true)
			args.interfaceaction(t, "port4", true)
			args.interfaceaction(t, "port5", true)
		})

		t.Run("Ping, Traceroute tests Ipv6inIp FRRs", func(t *testing.T) {
			args.interfaceaction(t, "port2", false)
			args.interfaceaction(t, "port4", false)
			p1 = dut.Port(t, "port5")

			testStats(t, dut, dut2, []string{p1.Name()}, "ping-ipinip", ipv6EntryPrefix, Loopback226, vrfEncapA)
			testStats(t, dut, dut2, []string{p1.Name()}, "traceroute-ipinip", ipv6EntryPrefix, Loopback226, vrfEncapA)
			args.interfaceaction(t, "port5", false)
			time.Sleep(5 * time.Second)

			testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "ping-ipinip", ipv6EntryPrefix, Loopback226, vrfEncapA)
			testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "traceroute-ipinip", ipv6EntryPrefix, Loopback226, vrfEncapA)

			args.interfaceaction(t, "port2", true)
			args.interfaceaction(t, "port4", true)
			args.interfaceaction(t, "port5", true)

		})

		t.Run("test with ttl 1 & different ttl", func(t *testing.T) {

			testTrafficmin(t, args.ate, args.top, 5, true, false, false)

			testTrafficmin(t, args.ate, args.top, 4, true, false, false)
		})
		t.Run("test with ttl 1 frrs", func(t *testing.T) {

			testTrafficmin(t, args.ate, args.top, 5, true, true, false)

			testTrafficmin(t, args.ate, args.top, 5, true, false, true)
			args.interfaceaction(t, "port2", true)
			args.interfaceaction(t, "port4", true)
			args.interfaceaction(t, "port5", true)
		})

		t.Run("test with ttl different - frrs", func(t *testing.T) {

			testTrafficmin(t, args.ate, args.top, 4, true, true, false)

			testTrafficmin(t, args.ate, args.top, 4, true, false, true)
			args.interfaceaction(t, "port2", true)
			args.interfaceaction(t, "port4", true)
			args.interfaceaction(t, "port5", true)

		})

		t.Run("test with ttl 1 & different ttl ipv6", func(t *testing.T) {

			testTrafficmin(t, args.ate, args.top, 6, true, false, false)

			testTrafficmin(t, args.ate, args.top, 7, true, false, false)
			args.interfaceaction(t, "port2", true)
			args.interfaceaction(t, "port4", true)
			args.interfaceaction(t, "port5", true)
		})
		t.Run("test with ttl 1 frrs ipv6", func(t *testing.T) {

			testTrafficmin(t, args.ate, args.top, 6, true, true, false)

			testTrafficmin(t, args.ate, args.top, 6, true, false, true)
			args.interfaceaction(t, "port2", true)
			args.interfaceaction(t, "port4", true)
			args.interfaceaction(t, "port5", true)
		})

		t.Run("test with ttl different - frrs ipv6", func(t *testing.T) {

			testTrafficmin(t, args.ate, args.top, 7, true, true, false)

			testTrafficmin(t, args.ate, args.top, 7, true, false, true)
			args.interfaceaction(t, "port2", true)
			args.interfaceaction(t, "port4", true)
			args.interfaceaction(t, "port5", true)

		})

		defer args.interfaceaction(t, "port2", true)
		defer args.interfaceaction(t, "port4", true)
		defer args.interfaceaction(t, "port5", true)

		if k == 1 {
			t.Run("tests after rpfo", func(t *testing.T) {

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
			})
			time.Sleep(6 * time.Minute)
		}
	}

	p3 = dut.Port(t, "port1")
	p4 := dut.Port(t, "port9")
	p5 := dut.Port(t, "port10")
	unconfigbasePBR(t, dut, "PBR", []string{p3.Name(), p4.Name(), p5.Name()})

	configPBR(t, dut, "REPAIRED", false)
	configureIntfPBR(t, dut, "PBR", p3.Name())
	configureIntfPBR(t, dut, "PBR", p4.Name())
	configureIntfPBR(t, dut, "PBR", p5.Name())
	configvrfInt(t, dut, "REPAIRED", "Loopback22")

	t.Run("Fallback Ping, Traceroute test IpinIp", func(t *testing.T) {
		p1 = dut.Port(t, "port2")
		p2 = dut.Port(t, "port4")

		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping-ipinip", "197.51.100.2", Loopback22, vrfEncapA)
		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute-ipinip", "197.51.100.2", Loopback22, vrfEncapA)
	})

	t.Run("Fallback Ping, Traceroute test IpinIp FRRs", func(t *testing.T) {
		p2 = dut.Port(t, "port3")
		args.interfaceaction(t, "port2", false)
		args.interfaceaction(t, "port4", false)

		testStats(t, dut, dut2, []string{p2.Name()}, "ping-ipinip", "197.51.100.2", Loopback22, vrfEncapA)
		testStats(t, dut, dut2, []string{p2.Name()}, "traceroute-ipinip", "197.51.100.2", Loopback22, vrfEncapA)

		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

	})

	t.Run("Fallback Ping, Traceroute test Ipv6inIp", func(t *testing.T) {
		p1 = dut.Port(t, "port2")
		p2 = dut.Port(t, "port4")
		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping-ipinip", "2556::2", Loopback226, vrfEncapA)
		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute-ipinip", "2556::2", Loopback226, vrfEncapA)
	})

	t.Run("Fallback Ping, Traceroutes tests Ipv6inIp FRRs", func(t *testing.T) {
		args.interfaceaction(t, "port2", false)
		args.interfaceaction(t, "port4", false)

		testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "ping-ipinip", "2556::2", Loopback226, vrfEncapA)
		testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "traceroute-ipinip", "2556::2", Loopback226, vrfEncapA)

		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

	})

	t.Run("Fallback test with ttl 1 & different ttl", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 5, false, false, false)

		testTrafficmin(t, args.ate, args.top, 4, false, false, false)
	})
	t.Run("Fallback test with ttl 1 frrs", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 5, false, true, true)
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

	})

	t.Run("Fallback test with ttl different - frrs", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 4, false, true, true)
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

	})

	t.Run("Fallback test with ttl 1 & different ttl ipv6", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 6, false, false, false)

		testTrafficmin(t, args.ate, args.top, 7, false, false, false)

	})
	t.Run("Fallback test with ttl 1 frrs ipv6", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 6, false, true, true)

		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)
	})

	t.Run("Fallback test with ttl different - frrs ipv6", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 7, false, true, true)
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

	})
}

func TestWithDCUnoptimized(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with decapencap/decap, verify traffic, reprogram & delete ipv4/NHG/NH")

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
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

	p1 := dut.Port(t, "port9")
	configurePort(t, dut, p1.Name(), "192.0.9.2", "7777::2", 30, 126)

	p2 := dut.Port(t, "port10")
	configurePort(t, dut, p2.Name(), "192.0.10.2", "192:0:2::1E", 30, 126)
	p3 := dut.Port(t, "port1")

	unconfigbasePBR(t, dut, "PBR", []string{p1.Name(), p2.Name(), p3.Name()})
	configPBR(t, dut, "PBR", true)
	configureIntfPBR(t, dut, "PBR", p3.Name())
	configureIntfPBR(t, dut, "PBR", p2.Name())
	configureIntfPBR(t, dut, "PBR", p1.Name())
	configvrfInt(t, dut, vrfEncapA, "Loopback22")

	configvrfInt(t, dut, vrfEncapA, p2.Name())

	staticvrf(t, dut, vrfEncapA, "192.0.10.1", "192:0:2::1d")
	staticvrf(t, dut, "DEFAULT", "192.0.9.1", "192:0:2::1a")

	dcunoptchain(t)
	dut2 := ondatra.DUT(t, "dut2")

	args.client.AddNH(t, 22200, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 22200, 0, map[uint64]uint64{22200: 20}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	nhge := 22200
	nhgi := 2220
	nh := 1000
	var wt int

	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(2220))

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
			WithNetworkInstance("REPAIRED").
			WithPrefix(prefix).
			WithNextHopGroup(uint64(nhgi)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	}
	args.client.AddIPv6(t, "2555::2/128", uint64(nhgi), "REPAIRED", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, "2556::2/128", uint64(nhgi), "REPAIRED", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Run("Ping, Traceroute test", func(t *testing.T) {
		p1 := dut.Port(t, "port2")
		p2 := dut.Port(t, "port4")
		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping", IpEncap, Loopback12, vrfEncapA)
		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute", IpEncap, Loopback12, vrfEncapA)
	})

	t.Run("Ping, Traceroute test Ip", func(t *testing.T) {
		p1 := dut.Port(t, "port2")
		p2 := dut.Port(t, "port4")

		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ip-ping", IpEncap, Loopback0, vrfEncapA)
		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ip-traceroute", IpEncap, Loopback0, vrfEncapA, &Traceptions{Ip: "ip"})
	})
	t.Run("Ping, Traceroute test IpinIp", func(t *testing.T) {
		p1 := dut.Port(t, "port2")
		p2 := dut.Port(t, "port4")

		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping-ipinip", IpEncap, Loopback22, vrfEncapA)
		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute-ipinip", IpEncap, Loopback22, vrfEncapA)
	})
	t.Run("Ping, Traceroute test FRRs", func(t *testing.T) {
		args.interfaceaction(t, "port2", false)
		args.interfaceaction(t, "port4", false)

		p1 := dut.Port(t, "port5")

		testStats(t, dut, dut2, []string{p1.Name()}, "ping", IpEncap, Loopback12, vrfEncapA)
		testStats(t, dut, dut2, []string{p1.Name()}, "traceroute", IpEncap, Loopback12, vrfEncapA)

		args.interfaceaction(t, "port5", false)
		time.Sleep(5 * time.Second)

		p2 := dut.Port(t, "port3")
		testStats(t, dut, dut2, []string{p2.Name()}, "ping", IpEncap, Loopback12, vrfEncapA)
		testStats(t, dut, dut2, []string{p2.Name()}, "traceroute", IpEncap, Loopback12, vrfEncapA)

		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

	})

	t.Run("Ping, Traceroute test Ip Frrs", func(t *testing.T) {
		args.interfaceaction(t, "port2", false)
		args.interfaceaction(t, "port4", false)
		p1 := dut.Port(t, "port5")
		p2 := dut.Port(t, "port3")
		time.Sleep(5 * time.Second)

		testStats(t, dut, dut2, []string{p1.Name()}, "ip-ping", IpEncap, Loopback0, vrfEncapA)
		testStats(t, dut, dut2, []string{p1.Name()}, "ip-traceroute", IpEncap, Loopback0, vrfEncapA, &Traceptions{Ip: "ip"})
		args.interfaceaction(t, "port5", false)
		time.Sleep(5 * time.Second)

		testStats(t, dut, dut2, []string{p2.Name()}, "ip-ping", IpEncap, Loopback0, vrfEncapA)
		testStats(t, dut, dut2, []string{p2.Name()}, "ip-traceroute", IpEncap, Loopback0, vrfEncapA, &Traceptions{Ip: "ip"})
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

	})

	t.Run("Ping, Traceroute test IpinIp FRRs", func(t *testing.T) {
		p1 := dut.Port(t, "port5")
		p2 := dut.Port(t, "port3")
		args.interfaceaction(t, "port2", false)
		args.interfaceaction(t, "port4", false)
		time.Sleep(5 * time.Second)

		testStats(t, dut, dut2, []string{p1.Name()}, "ping-ipinip", IpEncap, Loopback22, vrfEncapA)
		testStats(t, dut, dut2, []string{p1.Name()}, "traceroute-ipinip", IpEncap, Loopback22, vrfEncapA)
		args.interfaceaction(t, "port5", false)
		time.Sleep(5 * time.Second)

		testStats(t, dut, dut2, []string{p2.Name()}, "ping-ipinip", IpEncap, Loopback22, vrfEncapA)
		testStats(t, dut, dut2, []string{p2.Name()}, "traceroute-ipinip", IpEncap, Loopback22, vrfEncapA)

		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

	})

	t.Run("Ping, Traceroute test ipv6", func(t *testing.T) {
		p1 := dut.Port(t, "port2")
		p2 := dut.Port(t, "port4")
		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping", ipv6EntryPrefix, Loopback126, vrfEncapA)
		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute", ipv6EntryPrefix, Loopback126, vrfEncapA)
	})

	t.Run("Ping, Traceroute test Ipv6", func(t *testing.T) {
		p1 := dut.Port(t, "port2")
		p2 := dut.Port(t, "port4")
		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ip-ping", ipv6EntryPrefix, Loopback06, vrfEncapA)
		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ip-traceroute", ipv6EntryPrefix, Loopback06, vrfEncapA, &Traceptions{Ip: "ipv6"})
	})
	t.Run("Ping, Traceroute test Ipv6inIp", func(t *testing.T) {
		p1 := dut.Port(t, "port2")
		p2 := dut.Port(t, "port4")
		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping-ipinip", ipv6EntryPrefix, Loopback226, vrfEncapA)
		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute-ipinip", ipv6EntryPrefix, Loopback226, vrfEncapA)
	})
	t.Run("Ping, Traceroute tests  ipv6 FRRs", func(t *testing.T) {
		args.interfaceaction(t, "port2", false)
		args.interfaceaction(t, "port4", false)
		p1 := dut.Port(t, "port5")
		time.Sleep(5 * time.Second)

		testStats(t, dut, dut2, []string{p1.Name()}, "ping", ipv6EntryPrefix, Loopback126, vrfEncapA)
		testStats(t, dut, dut2, []string{p1.Name()}, "traceroute", ipv6EntryPrefix, Loopback126, vrfEncapA)

		args.interfaceaction(t, "port5", false)
		time.Sleep(5 * time.Second)

		testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "ping", ipv6EntryPrefix, Loopback126, vrfEncapA)
		testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "traceroute", ipv6EntryPrefix, Loopback126, vrfEncapA)
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

	})

	t.Run("Ping, Traceroute test Ipv6 Frrs", func(t *testing.T) {
		args.interfaceaction(t, "port2", false)
		args.interfaceaction(t, "port4", false)
		p1 := dut.Port(t, "port5")
		testStats(t, dut, dut2, []string{p1.Name()}, "ip-ping", ipv6EntryPrefix, Loopback06, vrfEncapA)
		testStats(t, dut, dut2, []string{p1.Name()}, "ip-traceroute", ipv6EntryPrefix, Loopback06, vrfEncapA, &Traceptions{Ip: "ipv6"})
		args.interfaceaction(t, "port5", false)
		time.Sleep(5 * time.Second)

		testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "ip-ping", ipv6EntryPrefix, Loopback06, vrfEncapA)
		testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "ip-traceroute", ipv6EntryPrefix, Loopback06, vrfEncapA, &Traceptions{Ip: "ipv6"})
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)
	})

	t.Run("Ping, Traceroutes tests Ipv6inIp FRRs", func(t *testing.T) {
		args.interfaceaction(t, "port2", false)
		args.interfaceaction(t, "port4", false)
		p1 := dut.Port(t, "port5")
		time.Sleep(5 * time.Second)

		testStats(t, dut, dut2, []string{p1.Name()}, "ping-ipinip", ipv6EntryPrefix, Loopback226, vrfEncapA)
		testStats(t, dut, dut2, []string{p1.Name()}, "traceroute-ipinip", ipv6EntryPrefix, Loopback226, vrfEncapA)
		args.interfaceaction(t, "port5", false)
		time.Sleep(5 * time.Second)

		testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "ping-ipinip", ipv6EntryPrefix, Loopback226, vrfEncapA)
		testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "traceroute-ipinip", ipv6EntryPrefix, Loopback226, vrfEncapA)

		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

	})

	t.Run("test with ttl 1 & different ttl", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 5, true, false, false)

		testTrafficmin(t, args.ate, args.top, 4, true, false, false)
	})
	t.Run("test with ttl 1 frrs", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 5, true, true, false)

		testTrafficmin(t, args.ate, args.top, 5, true, false, true)
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)
	})

	t.Run("test with ttl different - frrs", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 4, true, true, false)

		testTrafficmin(t, args.ate, args.top, 4, true, false, true)
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

	})

	t.Run("test with ttl 1 & different ttl ipv6", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 6, true, false, false)

		testTrafficmin(t, args.ate, args.top, 7, true, false, false)

	})
	t.Run("test with ttl 1 frrs ipv6", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 6, true, true, false)

		testTrafficmin(t, args.ate, args.top, 6, true, false, true)

		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)
	})

	t.Run("test with ttl different - frrs ipv6", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 7, true, true, false)

		testTrafficmin(t, args.ate, args.top, 7, true, false, true)
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

	})
	defer args.interfaceaction(t, "port2", true)
	defer args.interfaceaction(t, "port4", true)
	defer args.interfaceaction(t, "port5", true)
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
	// 	testTrafficWeight(t, args.ate, args.top, 65000, true, 5)

	// })
	configurePort(t, dut, "Loopback22", Loopback12, Loopback126, 32, 128)

	p3 = dut.Port(t, "port1")
	p4 := dut.Port(t, "port9")
	p5 := dut.Port(t, "port10")
	unconfigbasePBR(t, dut, "PBR", []string{p3.Name(), p4.Name(), p5.Name()})

	configPBR(t, dut, "REPAIRED", false)
	configureIntfPBR(t, dut, "PBR", p3.Name())
	configureIntfPBR(t, dut, "PBR", p4.Name())
	configureIntfPBR(t, dut, "PBR", p5.Name())
	configvrfInt(t, dut, "REPAIRED", "Loopback22")

	t.Run("Fallback Ping, traceroute test IpinIp", func(t *testing.T) {
		p1 := dut.Port(t, "port2")
		p2 := dut.Port(t, "port4")

		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping-ipinip", "197.51.100.2", Loopback22, vrfEncapA)
		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute-ipinip", "197.51.100.2", Loopback22, vrfEncapA)
	})

	t.Run("Fallback Ping, Traceroute test IpinIp FRRs", func(t *testing.T) {
		p2 := dut.Port(t, "port3")
		args.interfaceaction(t, "port2", false)
		args.interfaceaction(t, "port4", false)
		time.Sleep(5 * time.Second)

		testStats(t, dut, dut2, []string{p2.Name()}, "ping-ipinip", "197.51.100.2", Loopback22, vrfEncapA)
		testStats(t, dut, dut2, []string{p2.Name()}, "traceroute-ipinip", "197.51.100.2", Loopback22, vrfEncapA)

		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

	})

	t.Run("Fallback Ping, Traceroute test Ipv6inIp", func(t *testing.T) {
		p1 := dut.Port(t, "port2")
		p2 := dut.Port(t, "port4")
		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping-ipinip", "2556::2", Loopback226, vrfEncapA)
		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute-ipinip", "2556::2", Loopback226, vrfEncapA)
	})

	t.Run("Fallback Ping, Traceroutes tests Ipv6inIp FRRs", func(t *testing.T) {
		args.interfaceaction(t, "port2", false)
		args.interfaceaction(t, "port4", false)

		time.Sleep(5 * time.Second)

		testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "ping-ipinip", "2556::2", Loopback226, vrfEncapA)
		testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "traceroute-ipinip", "2556::2", Loopback226, vrfEncapA)

		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

	})

	t.Run("Fallback test with ttl 1 & different ttl", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 5, false, false, false)

		testTrafficmin(t, args.ate, args.top, 4, false, false, false)
	})
	t.Run("Fallback test with ttl 1 frr", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 5, false, true, true)
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)
	})

	t.Run("Fallback test with ttl different - frr", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 4, false, true, true)
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

	})

	t.Run("Fallback test with ttl 1 & different ttl ipv6", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 6, false, false, false)

		testTrafficmin(t, args.ate, args.top, 7, false, false, false)
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)
	})
	t.Run("test with ttl 1 frrs ipv6", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 6, true, true, true)
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)
	})

	t.Run("test with ttl different - frrs ipv6", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 7, true, true, false)

		testTrafficmin(t, args.ate, args.top, 7, true, false, true)
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

	})
}

func TestRepairedDecapmin(t *testing.T) {

	t.Logf("Program gribi entries with decapencap/decap, verify traffic, reprogram & delete ipv4/NHG/NH")

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
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
	addStaticRoute(t, dut, "202.1.0.0/16", true)
	configurePort(t, dut, "Loopback22", Loopback12, Loopback126, 32, 128)

	p3 := dut.Port(t, "port1")
	p4 := dut.Port(t, "port9")
	p5 := dut.Port(t, "port10")
	unconfigbasePBR(t, dut, "PBR", []string{p3.Name(), p4.Name(), p5.Name()})

	configPBR(t, dut, "REPAIRED", false)
	configureIntfPBR(t, dut, "PBR", p3.Name())
	configureIntfPBR(t, dut, "PBR", p4.Name())
	configureIntfPBR(t, dut, "PBR", p5.Name())
	configvrfInt(t, dut, "REPAIRED", "Loopback22")

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
	dut2 := ondatra.DUT(t, "dut2")
	t.Run("Ping, Traceroute test IpinIp", func(t *testing.T) {
		p1 := dut.Port(t, "port2")
		p2 := dut.Port(t, "port4")

		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping-ipinip", IpEncap, Loopback22, "REPAIRED")
		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute-ipinip", IpEncap, Loopback22, "REPAIRED")
	})

	t.Run("Ping, Traceroute test IpinIp FRRs", func(t *testing.T) {
		p1 := dut.Port(t, "port3")
		args.interfaceaction(t, "port2", false)
		args.interfaceaction(t, "port4", false)
		time.Sleep(5 * time.Second)

		testStats(t, dut, dut2, []string{p1.Name()}, "ping-ipinip", IpEncap, Loopback22, "REPAIRED")
		testStats(t, dut, dut2, []string{p1.Name()}, "traceroute-ipinip", IpEncap, Loopback22, "REPAIRED")

		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

	})

	t.Run("Ping, Traceroute test Ipv6inIp", func(t *testing.T) {
		p1 := dut.Port(t, "port2")
		p2 := dut.Port(t, "port4")
		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping-ipinip", ipv6EntryPrefix, Loopback226, "REPAIRED")
		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute-ipinip", ipv6EntryPrefix, Loopback226, "REPAIRED")
	})

	t.Run("Ping, Traceroutes tests Ipv6inIp FRRs", func(t *testing.T) {
		args.interfaceaction(t, "port2", false)
		args.interfaceaction(t, "port4", false)
		time.Sleep(5 * time.Second)

		testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "ping-ipinip", ipv6EntryPrefix, Loopback226, "REPAIRED")
		testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "traceroute-ipinip", ipv6EntryPrefix, Loopback226, "REPAIRED")

		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

	})

	t.Run("test with ttl 1 & different ttl", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 5, true, false, false)

		testTrafficmin(t, args.ate, args.top, 4, true, false, false)
	})
	t.Run("test with ttl 1 frrs", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 5, true, true, false)

		testTrafficmin(t, args.ate, args.top, 5, true, false, true)
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)
	})

	t.Run("test with ttl different - frrs", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 4, true, true, false)

		testTrafficmin(t, args.ate, args.top, 4, true, false, true)

		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

	})

	t.Run("test with ttl 1 & different ttl ipv6", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 6, true, false, false)

		testTrafficmin(t, args.ate, args.top, 7, true, false, false)
	})
	t.Run("test with ttl 1 frrs ipv6", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 6, true, true, false)

		testTrafficmin(t, args.ate, args.top, 6, true, false, true)
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)
	})

	t.Run("test with ttl different - frrs ipv6", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 7, true, true, false)

		testTrafficmin(t, args.ate, args.top, 7, true, false, true)
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

	})
	defer args.interfaceaction(t, "port2", true)
	defer args.interfaceaction(t, "port4", true)
	defer args.interfaceaction(t, "port5", true)

}

func TestWithPoPBackUp(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with decapencap/decap, verify traffic, reprogram & delete ipv4/NHG/NH")

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
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

	popgateconfig(t)
	p3 := dut.Port(t, "port1")
	p4 := dut.Port(t, "port9")
	p5 := dut.Port(t, "port10")

	unconfigbasePBR(t, dut, "PBR", []string{p3.Name(), p4.Name(), p5.Name()})

	configPBR(t, dut, "TE", false)
	configureIntfPBR(t, dut, "PBR", p3.Name())
	configureIntfPBR(t, dut, "PBR", p4.Name())
	configureIntfPBR(t, dut, "PBR", p5.Name())
	configvrfInt(t, dut, "TE", "Loopback22")

	dut2 := ondatra.DUT(t, "dut2")
	t.Run("Ping, Traceroute test", func(t *testing.T) {
		p1 := dut.Port(t, "port2")
		p2 := dut.Port(t, "port4")
		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping", dstPfx, Loopback12, "TE")
		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute", dstPfx, Loopback12, "TE")
	})

	t.Run("Ping, Traceroute test IpinIp", func(t *testing.T) {
		p1 := dut.Port(t, "port2")
		p2 := dut.Port(t, "port4")

		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping-ipinip", IpEncap, Loopback22, "TE")
		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute-ipinip", IpEncap, Loopback22, "TE")
	})

	t.Run("Ping, Traceroute test IpinIp FRRs", func(t *testing.T) {
		p1 := dut.Port(t, "port5")
		p2 := dut.Port(t, "port3")
		args.interfaceaction(t, "port2", false)
		args.interfaceaction(t, "port4", false)
		time.Sleep(5 * time.Second)

		testStats(t, dut, dut2, []string{p1.Name()}, "ping-ipinip", IpEncap, Loopback22, "TE")
		testStats(t, dut, dut2, []string{p1.Name()}, "traceroute-ipinip", IpEncap, Loopback22, "TE")
		args.interfaceaction(t, "port5", false)
		time.Sleep(5 * time.Second)

		testStats(t, dut, dut2, []string{p2.Name()}, "ping-ipinip", IpEncap, Loopback22, "TE")
		testStats(t, dut, dut2, []string{p2.Name()}, "traceroute-ipinip", IpEncap, Loopback22, "TE")

		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

	})

	t.Run("Ping, Traceroute test Ipv6inIp", func(t *testing.T) {
		p1 := dut.Port(t, "port2")
		p2 := dut.Port(t, "port4")
		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping-ipinip", ipv6EntryPrefix, Loopback226, "TE")
		testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute-ipinip", ipv6EntryPrefix, Loopback226, "TE")
	})

	t.Run("Ping, Traceroutes tests Ipv6inIp FRRs", func(t *testing.T) {
		args.interfaceaction(t, "port2", false)
		args.interfaceaction(t, "port4", false)
		p1 := dut.Port(t, "port5")
		time.Sleep(5 * time.Second)

		testStats(t, dut, dut2, []string{p1.Name()}, "ping-ipinip", ipv6EntryPrefix, Loopback226, "TE")
		testStats(t, dut, dut2, []string{p1.Name()}, "traceroute-ipinip", ipv6EntryPrefix, Loopback226, "TE")
		args.interfaceaction(t, "port5", false)
		time.Sleep(5 * time.Second)

		testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "ping-ipinip", ipv6EntryPrefix, Loopback226, "TE")
		testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "traceroute-ipinip", ipv6EntryPrefix, Loopback226, "TE")

		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

	})

	t.Run("test with ttl 1 & different ttl", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 5, true, false, false)

		testTrafficmin(t, args.ate, args.top, 4, true, false, false)
	})
	t.Run("test with ttl 1 frrs", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 5, true, true, false)

		testTrafficmin(t, args.ate, args.top, 5, true, false, true)
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)
	})

	t.Run("test with ttl different - frrs", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 4, true, true, false)

		testTrafficmin(t, args.ate, args.top, 4, true, false, true)
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

	})

	t.Run("test with ttl 1 & different ttl ipv6", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 6, true, false, false)

		testTrafficmin(t, args.ate, args.top, 7, true, false, false)

	})
	t.Run("test with ttl 1 frrs ipv6", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 6, true, true, false)

		testTrafficmin(t, args.ate, args.top, 6, true, false, true)
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)
	})

	t.Run("test with ttl different - frrs ipv6", func(t *testing.T) {

		testTrafficmin(t, args.ate, args.top, 7, true, true, false)

		testTrafficmin(t, args.ate, args.top, 7, true, false, true)
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

	})
	defer args.interfaceaction(t, "port2", true)
	defer args.interfaceaction(t, "port4", true)
	defer args.interfaceaction(t, "port5", true)

}

func TestWithPopUnoptimized(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with decapencap/decap, verify traffic, reprogram & delete ipv4/NHG/NH")

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
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

	popunoptchain(t)
	dut2 := ondatra.DUT(t, "dut2")

	for k := 1; k <= 2; k++ {

		t.Run("Ping, Traceroute test IpinIp", func(t *testing.T) {
			p1 := dut.Port(t, "port2")
			p2 := dut.Port(t, "port4")

			testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping-ipinip", IpEncap, Loopback22, "TE")
			testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute-ipinip", IpEncap, Loopback22, "TE")
		})

		t.Run("Ping, Traceroute test IpinIp FRRs", func(t *testing.T) {
			p1 := dut.Port(t, "port5")
			p2 := dut.Port(t, "port3")
			args.interfaceaction(t, "port2", false)
			args.interfaceaction(t, "port4", false)
			time.Sleep(5 * time.Second)

			testStats(t, dut, dut2, []string{p1.Name()}, "ping-ipinip", IpEncap, Loopback22, "TE")
			testStats(t, dut, dut2, []string{p1.Name()}, "traceroute-ipinip", IpEncap, Loopback22, "TE")
			args.interfaceaction(t, "port5", false)
			time.Sleep(5 * time.Second)

			testStats(t, dut, dut2, []string{p2.Name()}, "ping-ipinip", IpEncap, Loopback22, "TE")
			testStats(t, dut, dut2, []string{p2.Name()}, "traceroute-ipinip", IpEncap, Loopback22, "TE")

			args.interfaceaction(t, "port2", true)
			args.interfaceaction(t, "port4", true)
			args.interfaceaction(t, "port5", true)

		})

		t.Run("Ping, Traceroute test Ipv6inIp", func(t *testing.T) {
			p1 := dut.Port(t, "port2")
			p2 := dut.Port(t, "port4")
			testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "ping-ipinip", ipv6EntryPrefix, Loopback226, "TE")
			testStats(t, dut, dut2, []string{p1.Name(), p2.Name()}, "traceroute-ipinip", ipv6EntryPrefix, Loopback226, "TE")
		})

		t.Run("Ping, Traceroutes tests Ipv6inIp FRRs", func(t *testing.T) {
			args.interfaceaction(t, "port2", false)
			args.interfaceaction(t, "port4", false)
			p1 := dut.Port(t, "port5")
			//p2 = dut.Port(t, "port7")
			time.Sleep(10 * time.Second)

			testStats(t, dut, dut2, []string{p1.Name()}, "ping-ipinip", ipv6EntryPrefix, Loopback226, "TE")
			testStats(t, dut, dut2, []string{p1.Name()}, "traceroute-ipinip", ipv6EntryPrefix, Loopback226, "TE")
			args.interfaceaction(t, "port5", false)
			time.Sleep(5 * time.Second)

			testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "ping-ipinip", ipv6EntryPrefix, Loopback226, "TE")
			testStats(t, dut, dut2, []string{"Bundle-Ether126"}, "traceroute-ipinip", ipv6EntryPrefix, Loopback226, "TE")

			args.interfaceaction(t, "port2", true)
			args.interfaceaction(t, "port4", true)
			args.interfaceaction(t, "port5", true)

		})
		args.interfaceaction(t, "port2", true)
		args.interfaceaction(t, "port4", true)
		args.interfaceaction(t, "port5", true)

		t.Run("test with ttl 1 & different ttl", func(t *testing.T) {

			testTrafficmin(t, args.ate, args.top, 5, true, false, false)

			testTrafficmin(t, args.ate, args.top, 4, true, false, false)
		})
		t.Run("test with ttl 1 frrs", func(t *testing.T) {

			testTrafficmin(t, args.ate, args.top, 5, true, true, false)

			testTrafficmin(t, args.ate, args.top, 5, true, false, true)
			args.interfaceaction(t, "port2", true)
			args.interfaceaction(t, "port4", true)
			args.interfaceaction(t, "port5", true)
		})

		t.Run("test with ttl different - frrs", func(t *testing.T) {

			testTrafficmin(t, args.ate, args.top, 4, true, true, false)

			testTrafficmin(t, args.ate, args.top, 4, true, false, true)

			args.interfaceaction(t, "port2", true)
			args.interfaceaction(t, "port4", true)
			args.interfaceaction(t, "port5", true)

		})

		t.Run("test with ttl 1 & different ttl ipv6", func(t *testing.T) {

			testTrafficmin(t, args.ate, args.top, 6, true, false, false)

			testTrafficmin(t, args.ate, args.top, 7, true, false, false)
		})
		t.Run("test with ttl 1 frrs ipv6", func(t *testing.T) {

			testTrafficmin(t, args.ate, args.top, 6, true, true, false)

			testTrafficmin(t, args.ate, args.top, 6, true, false, true)

			args.interfaceaction(t, "port2", true)
			args.interfaceaction(t, "port4", true)
			args.interfaceaction(t, "port5", true)
		})

		t.Run("test with ttl different - frrs ipv6", func(t *testing.T) {

			testTrafficmin(t, args.ate, args.top, 7, true, true, false)

			testTrafficmin(t, args.ate, args.top, 7, true, false, true)

			args.interfaceaction(t, "port2", true)
			args.interfaceaction(t, "port4", true)
			args.interfaceaction(t, "port5", true)

		})
		defer args.interfaceaction(t, "port2", true)
		defer args.interfaceaction(t, "port4", true)
		defer args.interfaceaction(t, "port5", true)

		if k == 1 {
			t.Run("tests after rpfo", func(t *testing.T) {

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
			})
			time.Sleep(5 * time.Minute)
		}
	}
}

func testStats(t *testing.T, dut, dut2 *ondatra.DUTDevice, d_port []string, val, dest, src, vrfName string, opts ...*Traceptions) {
	t.Helper()
	var initial, count, final uint64
	initial, final = 0, 0
	cliHandle := dut.RawAPIs().CLI(t)

	for _, dp := range d_port {
		cmd := fmt.Sprintf("show interface %s", dp)
		output, _ := cliHandle.RunCommand(context.Background(), cmd)
		re := regexp.MustCompile(`(\d+)\spackets output`)
		match := re.FindStringSubmatch(output.Output())
		val, _ := strconv.Atoi(match[1])
		initial = initial + uint64(val)
	}

	switch val {
	case "ping":
		cliHandle := dut.RawAPIs().CLI(t)

		cmd := fmt.Sprintf("ping vrf %s %s source %s count 64", vrfName, dest, src)
		cliHandle.RunCommand(context.Background(), cmd)
		time.Sleep(60 * time.Second)
		count = 64
		t.Log("Ping done")

	case "traceroute":

		cliHandle := dut.RawAPIs().CLI(t)
		cmd := fmt.Sprintf("traceroute vrf %s %s source %s timeout 0", vrfName, dest, src)
		cliHandle.RunCommand(context.Background(), cmd)
		time.Sleep(15 * time.Second)
		count = 90
		t.Log("traceroute done")

	case "ip-ping":
		cliHandle := dut2.RawAPIs().CLI(t)

		cmd := fmt.Sprintf("ping %s source %s count 64", dest, src)
		cliHandle.RunCommand(context.Background(), cmd)
		time.Sleep(60 * time.Second)
		count = 64
		t.Log("ping done")

	case "ip-traceroute":
		t.Log("traceroute")

		cliHandle := dut2.RawAPIs().CLI(t)
		cmd := fmt.Sprintf("traceroute %s source %s timeout 0", dest, src)
		out, _ := cliHandle.RunCommand(context.Background(), cmd)
		fmt.Println(out.Output())
		var re *regexp.Regexp
		if len(opts) != 0 {
			for _, opt := range opts {
				if opt.Ip == "ip" {
					re = regexp.MustCompile(`\d\s+(\d+.\d+.\d+.\d+)\s+\d+ msec`)
				} else if opt.Ip == "ipv6" {
					re = regexp.MustCompile(`\d\s+([a-fA-F0-9]+:[a-fA-F0-9]+:[a-fA-F0-9]+::[a-fA-F0-9]+)\s+\d+ msec`)
				}
			}
		}

		match := re.FindStringSubmatch(out.Output())

		t.Log("counttttttt")
		fmt.Println(match)
		if match[1] == "192.0.10.2" || match[1] == "192:0:2::1e" {
			t.Log("Response received")
		}
		time.Sleep(15 * time.Second)
		count = 80
		t.Log("traceroute done")

	case "ping-ipinip":
		cliHandle := dut2.RawAPIs().CLI(t)

		cmd := fmt.Sprintf("ping vrf ENCAP_TE_VRF_A %s source %s count 64", dest, src)
		cliHandle.RunCommand(context.Background(), cmd)
		time.Sleep(60 * time.Second)
		count = 64
		t.Log("ping done")

	case "traceroute-ipinip":
		t.Log("traceroute")

		cliHandle := dut2.RawAPIs().CLI(t)
		cmd := fmt.Sprintf("traceroute vrf ENCAP_TE_VRF_A %s source %s timeout 0", dest, src)
		cliHandle.RunCommand(context.Background(), cmd)

		time.Sleep(15 * time.Second)
		count = 80
		t.Log("traceroute done")
	}

	for _, dp := range d_port {
		cmd := fmt.Sprintf("show interface %s", dp)
		output, _ := cliHandle.RunCommand(context.Background(), cmd)
		re := regexp.MustCompile(`(\d+)\spackets output`)
		match := re.FindStringSubmatch(output.Output())
		val, _ := strconv.Atoi(match[1])
		final = final + uint64(val)
		fmt.Println("final count")
		fmt.Println(final)
	}

	if got, want := final-initial, uint64(count); got < want {
		t.Errorf("Get less inPkts from telemetry: got %v, want >= %v", got, want)
		if diff := cmp.Diff(want, got, cmpopts.EquateApprox(0, 20)); diff != "" {
			t.Errorf("Packet count -want,+got:\n%s", diff)
		}
	}

}

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

	args.client.AddIPv4(t, "209.209.209.1/32", uint64(31000), *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, uint64(30003), "209.209.209.1", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 33000, 0, map[uint64]uint64{uint64(30003): 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddIPv4Batch(t, prefixest, 33000, "REPAIRED", *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

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

func dcunoptchain(t *testing.T) {
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
	args.client.AddIPv6(t, ipv6EntryPrefix+"/128", uint64(3000), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

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

}

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

}
