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
	"net"

	"strings"
	"time"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/google/gopacket/layers"
	"github.com/openconfig/featureprofiles/internal/cisco/config"
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
	Loopback12            = "44.44.44.44"
	Loopback126           = "4444::44"
	dstPfx                = "198.51.100.1"
	mask                  = "32"
	innerdstPfxMin_bgp    = "202.1.0.1"
	innerdstPfxCount_bgp  = 10
	innerdstPfxMin_isis   = "201.1.0.1"
	innerdstPfxCount_isis = 10
	dsip                  = "10.1.0.1"
	vrf1                  = "TE"
	vrf2                  = "TE2"
	vrf3                  = "TE3"
	pref6                 = "2555::2/128"
	prefi6                = "2556::2/128"
)

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

var args *testArgs

func baseconfig(t *testing.T) {
	t.Helper()

	if !baseconfigdone {
		args = &testArgs{}
		t.Log("in baseconfig")

		//Configure the DUT
		dut := ondatra.DUT(t, "dut")
		configureDUT(t, dut)
		//configure route-policy
		configRP(t, dut)
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

func addStaticRoute(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()
	s := &oc.Root{}
	static := s.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "DEFAULT")
	ipv4Nh := static.GetOrCreateStatic("0.0.0.0/0").GetOrCreateNextHop("0")
	ipv4Nh.NextHop, _ = ipv4Nh.To_NetworkInstance_Protocol_Static_NextHop_NextHop_Union(atePort6.IPv4)
	gnmi.Update(t, dut, d.NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "DEFAULT").Config(), static)

	ipv6nh := static.GetOrCreateStatic("::/0").GetOrCreateNextHop("0")
	ipv6nh.NextHop, _ = ipv4Nh.To_NetworkInstance_Protocol_Static_NextHop_NextHop_Union(atePort6.IPv6)
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

func testWithDCUnoptimized(ctx context.Context, t *testing.T, args *testArgs, isIPv4, encap bool, flap, te string, deviceSet bool, srcport string, opts ...*TOptions) {

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

	configPBR(t, dut, "PBR", true)
	configureIntfPBR(t, dut, "PBR", "Bundle-Ether120")
	configvrfInt(t, dut, vrfEncapA, "Loopback22")

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
	args.client.AddIPv6(t, "2555::/16", uint64(3000), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

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
	t.Logf("Delete re-add entries")
	args.client.DeleteIPv4Batch(t, prefixesd, 4000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	for _, prefix := range prefixesd {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance(vrfDecap).
			WithPrefix(prefix).
			WithNextHopGroup(uint64(4000)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	}

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
		} else {
			inDst = net.ParseIP("2555::")
			inDst[k] = uint8(rand.Intn(256))

			inSrc = net.ParseIP("6666::")

			inSrc[k] = uint8(rand.Intn(256))
			c = 2
		}

		var tcpd, tcps, udpd, udps uint16
		tcpd = uint16(rand.Intn(60000-1) + 1)
		tcps = uint16(rand.Intn(65000-1) + 1)
		udpd = uint16(rand.Intn(7000-1) + 1)
		udps = uint16(rand.Intn(8000-1) + 1)

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

		if c == 5 || c == 6 {
			c1 = 6
		} else if c == 2 || c == 22 {
			c1 = 22
		}

		var sourceport bool
		var sport string
		if srcport == "port1" {
			sourceport = true
			sport = srcport
		} else {
			sourceport = false
			sport = srcport
			stream = stream2
		}
		t.Logf("Source port %v", sport)
		if len(opts) != 0 {
			for _, opt := range opts {
				if opt.ptcp == 4 {

					if flap == "flap" {
						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcpd: tcpd, tcps: tcps})
						args.interfaceaction(t, "port2", false)
						t.Logf("Egress & Ingress port ids before flap %v %v", portid, portidin)
						args.interfaceaction(t, "port4", false)
						args.interfaceaction(t, "port6", false)
						args.interfaceaction(t, "port8", false)
						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcp: true, tcpd: tcpd, tcps: tcps})

					} else if flap == "flap1" {
						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcp: true, tcpd: tcpd, tcps: tcps})
						t.Logf("Egress & Ingress port ids before flap %v %v", portid, portidin)

						args.interfaceaction(t, "port2", false)

						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcp: true, tcpd: tcpd, tcps: tcps})

					} else {

						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcp: true, tcpd: tcpd, tcps: tcps})

					}
				} else if opt.pudp == 8 {
					if flap == "flap" {
						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true, udpd: udpd, udps: udps})
						t.Logf("Egress & Ingress port ids before flap %v %v", portid, portidin)

						args.interfaceaction(t, "port2", false)

						args.interfaceaction(t, "port4", false)
						args.interfaceaction(t, "port6", false)
						args.interfaceaction(t, "port8", false)
						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true, udpd: udpd, udps: udps})

					} else if flap == "flap1" {
						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true, udpd: udpd, udps: udps})
						t.Logf("Egress & Ingress port ids before flap %v %v", portid, portidin)

						args.interfaceaction(t, "port2", false)

						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true, udpd: udpd, udps: udps})

					} else {
						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true, udpd: udpd, udps: udps})
					}
				}
			}
		} else {
			if flap == "flap" {
				portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})
				t.Logf("Egress & Ingress port ids before flap %v %v", portid, portidin)

				args.interfaceaction(t, "port2", false)

				args.interfaceaction(t, "port4", false)
				args.interfaceaction(t, "port6", false)
				args.interfaceaction(t, "port8", false)
				portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})

			} else if flap == "flap1" {
				portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})
				t.Logf("Egress & Ingress port ids before flap %v %v", portid, portidin)

				args.interfaceaction(t, "port2", false)
				portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})

			} else {

				portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name()}, portinp: []string{p1.Name(), p3.Name()}})
			}
		}
		portIDe = IDMap[portid]
		portIDin = IDMap[portidin]
		var pkTOut int
		if len(opts) != 0 {
			for _, opt := range opts {
				if opt.ptcp == 4 {
					_, _, pkTOut = testTraffic(t, args.ate, args.top, c1, true, false, sourceport, 1, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{tcp: true, tcpd: tcpd, tcps: tcps})
				} else if opt.pudp == 8 {
					_, _, pkTOut = testTraffic(t, args.ate, args.top, c1, true, false, sourceport, 1, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{udp: true, udpd: udpd, udps: udps})
				}
			}
		} else {
			_, _, pkTOut = testTraffic(t, args.ate, args.top, c1, true, false, sourceport, 1, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String())
		}

		pktOut := pkTOut
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
				for i := 1; i <= 1; i++ {
					_, packets, err := test.client.StreamChannelGetPackets(&stream, uint64(test.wantPkts), 90*time.Second)
					s := len(packets)
					if s == 0 {
						if portIDe == "14" && (flap == "flap" || flap == "flap1") {
							break
						}
					}
					if s != pktOut {
						t.Log("Count mismatch")
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
					t.Logf("Want Packeet %v", wantPacket)

					for _, packet := range packets {
						if packet != nil {

							srcMAC, _, etherType := decodePacket(t, packet.Pkt.GetPayload())
							if etherType != layers.EthernetTypeIPv4 && etherType != layers.EthernetTypeIPv6 {
								continue
							}

							if !strings.EqualFold(srcMAC, tracerouteSrcMAC) {
								continue
							}
							if isIPv4 {
								if wantPacket.TTL != nil {
									//TTL/HopLimit comparison for IPV4 & IPV6
									if isIPv4 {
										captureTTL := decodePacket4(t, packet.Pkt.GetPayload())
										if captureTTL != TTL1 {
											t.Fatalf("Packet in PacketIn message is not matching wanted packet=IPV4 TTL1")
										}

									} else {
										captureHopLimit := decodePacket6(t, packet.Pkt.GetPayload())
										if captureHopLimit != HopLimit1 {
											t.Fatalf("Packet in PacketIn message is not matching wanted packet=IPV6 HopLimit1")
										}
									}
								}
							}

							//Metadata comparision
							if metaData := packet.Pkt.GetMetadata(); metaData != nil {
								if got := metaData[0].GetMetadataId(); got == METADATA_INGRESS_PORT {
									t.Logf("Metadata ingress port: want metadatingress %d, got %d", METADATA_INGRESS_PORT, got)
									if gotPortID := string(metaData[0].GetValue()); gotPortID != portIDin {
										t.Errorf("Ingress Port Id mismatch: want %s, got %s", portIDin, gotPortID)
										t.Errorf("Mismatch for out src %s dst %s, inner src %s dst %s", outSrc, outDst, inSrc, inDst)

									} else {
										t.Logf("Match for out src %s dst %s, inner src %s dst %s", outSrc, outDst, inSrc, inDst)
									}

								} else {
									t.Errorf("Metadata ingress port mismatch: want %d, got %d", METADATA_INGRESS_PORT, got)
								}

								if got := metaData[1].GetMetadataId(); got == METADATA_EGRESS_PORT {
									if gotPortID := string(metaData[1].GetValue()); gotPortID != portIDe {
										t.Errorf("Mismatch for out src %s dst %s, inner src %s dst %s", outSrc, outDst, inSrc, inDst)

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
						if portIDe == "14" && (flap == "flap" || flap == "flap1") {
							t.Errorf("Number of PacketIn, got: %d, want: %d", got, want)
							t.Logf(" Count Mismatch for out src %s dst %s, inner src %s dst %s", outSrc, outDst, inSrc, inDst)
						}
					} else {
						t.Logf("Count match for out src %s dst %s, inner src %s dst %s", outSrc, outDst, inSrc, inDst)
					}
				}
			})

		}
		k++

	}
}

func testWithPoPUnoptimized(ctx context.Context, t *testing.T, args *testArgs, isIPv4 bool, prog int, flap, te string, deviceSet bool, srcport string, opts ...*TOptions) {

	leader := args.leader
	follower := args.follower

	if prog == 5 {
		if isIPv4 {
			// Insert p4rtutils acl entry on the DUT
			if err := programmTableEntry(leader, args.packetIO, false, isIPv4, deviceId); err != nil {
				t.Fatalf("There is error when programming entry")
			}
			if deviceSet {

				if err := programmTableEntry(leader, args.packetIO, false, isIPv4, deviceId2); err != nil {
					t.Fatalf("There is error when programming entry")
				}

			}

		} else {
			// Insert p4rtutils acl entry on the DUT
			if err := programmTableEntry(leader, args.packetIO, false, false, deviceId); err != nil {
				t.Fatalf("There is error when programming entry")
			}

			if deviceSet {

				if err := programmTableEntry(leader, args.packetIO, false, false, deviceId2); err != nil {
					t.Fatalf("There is error when programming entry")
				}
			}
		}
	} else if prog == 6 {
		if deviceSet {
			defer programmTableEntry(leader, args.packetIO, true, true, deviceId2)
			defer programmTableEntry(leader, args.packetIO, true, false, deviceId2)

		}
		// 	// Delete p4rtutils acl entry on the device
		defer programmTableEntry(leader, args.packetIO, true, true, deviceId)
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
	configPBR(t, dut, "PBR", true)
	configureIntfPBR(t, dut, "PBR", "Bundle-Ether120")
	unconfigbasePBR(t, dut, "PBR", []string{"Bundle-Ether120"})
	configPBR(t, dut, "TE", false)
	configureIntfPBR(t, dut, "PBR", "Bundle-Ether120")

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

	t.Log("Delete readd entries")
	args.client.DeleteIPv4Batch(t, prefixese, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	nhgi = 1000
	for _, prefix := range prefixese {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance("TE").
			WithPrefix(prefix).
			WithNextHopGroup(uint64(nhgi)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	}

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
		} else {
			inDst = net.ParseIP("2555::")
			inDst[k] = uint8(rand.Intn(256))
			inSrc = net.ParseIP("6666::")
			inSrc[k] = uint8(rand.Intn(256))
			c = 2
		}

		var tcpd, tcps, udpd, udps uint16
		tcpd = uint16(rand.Intn(60000-1) + 1)
		tcps = uint16(rand.Intn(65000-1) + 1)
		udpd = uint16(rand.Intn(7000-1) + 1)
		udps = uint16(rand.Intn(8000-1) + 1)

		var portid, portidin, portIDe, portIDin string
		p1 := dut.Port(t, "port1")
		p2 := dut.Port(t, "port2")
		p3 := dut.Port(t, "port3")
		p4 := dut.Port(t, "port4")
		p5 := dut.Port(t, "port5")
		p6 := dut.Port(t, "port6")
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

		if c == 5 || c == 6 {
			c1 = 6
		} else if c == 2 || c == 22 {
			c1 = 22
		}

		var sourceport bool
		var sport string
		if srcport == "port1" {
			sourceport = true
			sport = srcport
		} else {
			sourceport = false
			sport = srcport
			stream = stream2
		}
		t.Logf("Source port %v", sport)

		if len(opts) != 0 {
			for _, opt := range opts {
				if opt.ptcp == 4 {

					if flap == "flap" {
						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcpd: tcpd, tcps: tcps})
						t.Logf("Egress & Ingress port ids before flap %v %v", portid, portidin)

						args.interfaceaction(t, "port2", false)

						args.interfaceaction(t, "port4", false)
						args.interfaceaction(t, "port6", false)
						args.interfaceaction(t, "port8", false)
						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcp: true, tcpd: tcpd, tcps: tcps})

					} else if flap == "flap1" {
						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcp: true, tcpd: tcpd, tcps: tcps})
						t.Logf("Egress & Ingress port ids before flap %v %v", portid, portidin)

						args.interfaceaction(t, "port2", false)
						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcp: true, tcpd: tcpd, tcps: tcps})

					} else {
						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcp: true, tcpd: tcpd, tcps: tcps})

					}
				} else if opt.pudp == 8 {
					if flap == "flap" {
						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true, udpd: udpd, udps: udps})
						t.Logf("Egress & Ingress port ids before flap %v %v", portid, portidin)

						args.interfaceaction(t, "port2", false)

						args.interfaceaction(t, "port4", false)
						args.interfaceaction(t, "port6", false)
						args.interfaceaction(t, "port8", false)
						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true, udpd: udpd, udps: udps})

					} else if flap == "flap1" {
						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true, udpd: udpd, udps: udps})
						t.Logf("Egress & Ingress port ids before flap %v %v", portid, portidin)

						args.interfaceaction(t, "port2", false)
						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true, udpd: udpd, udps: udps})

					} else {

						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true, udpd: udpd, udps: udps})

					}
				}
			}
		} else {
			if flap == "flap" {
				portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})
				t.Logf("Egress & Ingress port ids before flap %v %v", portid, portidin)

				args.interfaceaction(t, "port2", false)

				args.interfaceaction(t, "port4", false)
				args.interfaceaction(t, "port6", false)
				args.interfaceaction(t, "port8", false)
				portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})

			} else if flap == "flap1" {

				portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})
				t.Logf("Egress & Ingress port ids before flap %v %v", portid, portidin)

				args.interfaceaction(t, "port2", false)

				portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})

			} else {
				portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name()}, portinp: []string{p1.Name(), p3.Name()}})
			}
		}
		portIDe = IDMap[portid]
		portIDin = IDMap[portidin]
		var pkTOut int
		if len(opts) != 0 {
			for _, opt := range opts {
				if opt.ptcp == 4 {
					_, _, pkTOut = testTraffic(t, args.ate, args.top, c1, true, false, sourceport, 1, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{tcp: true, tcpd: tcpd, tcps: tcps})

				} else if opt.pudp == 8 {
					_, _, pkTOut = testTraffic(t, args.ate, args.top, c1, true, false, sourceport, 1, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{udp: true, udpd: udpd, udps: udps})
				}
			}
		} else {
			_, _, pkTOut = testTraffic(t, args.ate, args.top, c1, true, false, sourceport, 1, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String())
		}

		pktOut := pkTOut
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
				for i := 1; i <= 1; i++ {

					// Extract packets from PacketIn message sent to p4rt client

					_, packets, err := test.client.StreamChannelGetPackets(&stream, uint64(test.wantPkts), 90*time.Second)
					s := len(packets)

					if s == 0 {
						if portIDe == "14" && (flap == "flap" || flap == "flap1") {
							break
						}
					}

					if s != pktOut {
						t.Log("Count mismatch")
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
					t.Logf("Want Packet %v", wantPacket)
					for _, packet := range packets {
						if packet != nil {

							srcMAC, _, etherType := decodePacket(t, packet.Pkt.GetPayload())
							if etherType != layers.EthernetTypeIPv4 && etherType != layers.EthernetTypeIPv6 {
								continue
							}
							if !strings.EqualFold(srcMAC, tracerouteSrcMAC) {
								continue
							}

							if wantPacket.TTL != nil {
								//TTL/HopLimit comparison for IPV4 & IPV6
								//if isIPv4 {
								captureTTL := decodePacket4(t, packet.Pkt.GetPayload())
								if captureTTL != TTL1 {
									t.Fatalf("Packet in PacketIn message is not matching wanted packet=IPV4 TTL1")
								}

								// } else {
								// 	captureHopLimit := decodePacket6(t, packet.Pkt.GetPayload())
								// 	if captureHopLimit != HopLimit1 {
								// 		t.Fatalf("Packet in PacketIn message is not matching wanted packet=IPV6 HopLimit1")
								// 	}
								// }
							}

							//Metadata comparision
							if metaData := packet.Pkt.GetMetadata(); metaData != nil {
								if got := metaData[0].GetMetadataId(); got == METADATA_INGRESS_PORT {
									t.Logf("Metadata ingress port: want metadatingress %d, got %d", METADATA_INGRESS_PORT, got)
									if gotPortID := string(metaData[0].GetValue()); gotPortID != portIDin {
										t.Errorf("Ingress Port Id mismatch: want %s, got %s", portIDin, gotPortID)
										t.Logf("Mismatch for out src %s dst %s, inner src %s dst %s", outSrc, outDst, inSrc, inDst)

									} else {
										t.Logf("Match for out src %s dst %s, inner src %s dst %s", outSrc, outDst, inSrc, inDst)
									}

								} else {
									t.Errorf("Metadata ingress port mismatch: want %d, got %d", METADATA_INGRESS_PORT, got)
								}
								if got := metaData[1].GetMetadataId(); got == METADATA_EGRESS_PORT {
									if gotPortID := string(metaData[1].GetValue()); gotPortID != portIDe {
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
						if portIDe == "14" && (flap == "flap" || flap == "flap1") {
							t.Errorf("Number of PacketIn, got: %d, want: %d", got, want)
							t.Logf(" Count Mismatch for out src %s dst %s, inner src %s dst %s", outSrc, outDst, inSrc, inDst)
						}
					} else {
						t.Logf("Count match for out src %s dst %s, inner src %s dst %s", outSrc, outDst, inSrc, inDst)
					}
				}
			})

		}
		k++

	}

}

func testWithregionalization(ctx context.Context, t *testing.T, args *testArgs, isIPv4, encap bool, flap, te string, deviceSet bool, srcport string, opts ...*TOptions) {

	leader := args.leader
	follower := args.follower

	if isIPv4 {
		programmTableEntry(leader, args.packetIO, true, isIPv4, deviceId)
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
	baseconfig(t)

	config.TextWithGNMI(args.ctx, t, args.dut, "vrf ENCAP_TE_VRF_A fallback-vrf default")
	addStaticRoute(t, dut)
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
	configPBR(t, dut, "PBR", true)

	unconfigbasePBR(t, dut, "PBR", []string{"Bundle-Ether120"})

	configPBR(t, dut, "PBR", true)
	configureIntfPBR(t, dut, "PBR", "Bundle-Ether120")
	configvrfInt(t, dut, vrfEncapA, "Loopback22")

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
	args.client.AddIPv6(t, "2555::/16", uint64(3000), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

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
	t.Log("Delete readd prefixes")
	args.client.DeleteIPv4Batch(t, prefixesd, 4000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	for _, prefix := range prefixesd {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance(vrfDecap).
			WithPrefix(prefix).
			WithNextHopGroup(uint64(4000)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	}

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
			inDst = net.IP{195, 51, uint8(rand.Intn(59-1) + 1), uint8(rand.Intn(255-1) + 1)}
			c = 5
		} else {
			inDst = net.ParseIP("2005::")
			inDst[k] = uint8(rand.Intn(256))
			inSrc = net.ParseIP("6666::")
			inSrc[k] = uint8(rand.Intn(256))
			c = 2
		}

		var tcpd, tcps, udpd, udps uint16
		tcpd = uint16(rand.Intn(60000-1) + 1)
		tcps = uint16(rand.Intn(65000-1) + 1)
		udpd = uint16(rand.Intn(7000-1) + 1)
		udps = uint16(rand.Intn(8000-1) + 1)

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

		if c == 5 || c == 6 {
			c1 = 6
		} else if c == 2 || c == 22 {
			c1 = 22
		}

		var sourceport bool
		var sport string
		if srcport == "port1" {
			sourceport = true
			sport = srcport
		} else {
			sourceport = false
			sport = srcport
			stream = stream2
		}
		t.Logf("Source port %v", sport)

		if len(opts) != 0 {
			for _, opt := range opts {
				if opt.ptcp == 4 {

					if flap == "flap" {
						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcpd: tcpd, tcps: tcps})
						t.Logf("Egress & Ingress port ids before flap %v %v", portid, portidin)

						args.interfaceaction(t, "port2", false)

						args.interfaceaction(t, "port4", false)
						args.interfaceaction(t, "port6", false)
						args.interfaceaction(t, "port8", false)
						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcp: true, tcpd: tcpd, tcps: tcps})

					} else if flap == "flap1" {
						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcp: true, tcpd: tcpd, tcps: tcps})
						t.Logf("Egress & Ingress port ids before flap %v %v", portid, portidin)

						args.interfaceaction(t, "port2", false)

						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcp: true, tcpd: tcpd, tcps: tcps})

					} else {
						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name()}, portinp: []string{p1.Name(), p3.Name()}, tcp: true, tcpd: tcpd, tcps: tcps})
					}
				} else if opt.pudp == 8 {
					if flap == "flap" {
						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true, udpd: udpd, udps: udps})
						t.Logf("Egress & Ingress port ids before flap %v %v", portid, portidin)

						args.interfaceaction(t, "port2", false)

						args.interfaceaction(t, "port4", false)
						args.interfaceaction(t, "port6", false)
						args.interfaceaction(t, "port8", false)
						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true, udpd: udpd, udps: udps})

					} else if flap == "flap1" {
						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true, udpd: udpd, udps: udps})
						t.Logf("Egress & Ingress port ids before flap %v %v", portid, portidin)

						args.interfaceaction(t, "port2", false)

						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true, udpd: udpd, udps: udps})

					} else {
						portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name()}, portinp: []string{p1.Name(), p3.Name()}, udp: true, udpd: udpd, udps: udps})
					}
				}
			}
		} else {
			if flap == "flap" {
				portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})
				t.Logf("Egress & Ingress port ids before flap %v %v", portid, portidin)

				args.interfaceaction(t, "port2", false)

				args.interfaceaction(t, "port4", false)
				args.interfaceaction(t, "port6", false)
				args.interfaceaction(t, "port8", false)
				portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})

			} else if flap == "flap1" {

				portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})
				t.Logf("Egress & Ingress port ids before flap %v %v", portid, portidin)

				args.interfaceaction(t, "port2", false)

				portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name(), p5.Name()}, portinp: []string{p1.Name(), p3.Name()}})

			} else {

				portid, portidin, _ = testTraffic(t, args.ate, args.top, c, true, true, sourceport, 10, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{portin: []string{p2.Name(), p4.Name(), p6.Name(), p8.Name()}, portinp: []string{p1.Name(), p3.Name()}})
			}
		}
		portIDe = IDMap[portid]
		portIDin = IDMap[portidin]
		var pkTOut int
		if len(opts) != 0 {
			for _, opt := range opts {
				if opt.ptcp == 4 {
					_, _, pkTOut = testTraffic(t, args.ate, args.top, c1, true, false, sourceport, 1, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{tcp: true, tcpd: tcpd, tcps: tcps})

				} else if opt.pudp == 8 {
					_, _, pkTOut = testTraffic(t, args.ate, args.top, c1, true, false, sourceport, 1, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String(), &Countoptions{udp: true, udpd: udpd, udps: udps})
				}
			}
		} else {
			_, _, pkTOut = testTraffic(t, args.ate, args.top, c1, true, false, sourceport, 1, 20, inDst.String(), inSrc.String(), outDst.String(), outSrc.String())

		}

		pktOut := pkTOut

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
				for i := 1; i <= 1; i++ {
					// Extract packets from PacketIn message sent to p4rt client
					_, packets, err := test.client.StreamChannelGetPackets(&stream, uint64(test.wantPkts), 90*time.Second)
					s := len(packets)
					if s == 0 && err != nil {
						if flap == "flap" {
							break
						}
					}

					if s != pktOut {
						t.Log("Count mismatch")
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
					t.Logf("Wantpacket %v", wantPacket)
					for _, packet := range packets {
						if packet != nil {

							srcMAC, _, etherType := decodePacket(t, packet.Pkt.GetPayload())
							if etherType != layers.EthernetTypeIPv4 && etherType != layers.EthernetTypeIPv6 {
								continue
							}

							if !strings.EqualFold(srcMAC, tracerouteSrcMAC) {
								continue
							}

							if isIPv4 {
								if wantPacket.TTL != nil {
									//TTL/HopLimit comparison for IPV4 & IPV6
									if isIPv4 {
										captureTTL := decodePacket4(t, packet.Pkt.GetPayload())
										if captureTTL != TTL1 {
											t.Fatalf("Packet in PacketIn message is not matching wanted packet=IPV4 TTL1")
										}

									} else {
										captureHopLimit := decodePacket6(t, packet.Pkt.GetPayload())
										if captureHopLimit != HopLimit1 {
											t.Fatalf("Packet in PacketIn message is not matching wanted packet=IPV6 HopLimit1")
										}
									}
								}
							}

							//Metadata comparision
							if metaData := packet.Pkt.GetMetadata(); metaData != nil {
								if got := metaData[0].GetMetadataId(); got == METADATA_INGRESS_PORT {
									t.Logf("Metadata ingress port: want metadatingress %d, got %d", METADATA_INGRESS_PORT, got)
									if gotPortID := string(metaData[0].GetValue()); gotPortID != portIDin {
										t.Errorf("Ingress Port Id mismatch: want %s, got %s", portIDin, gotPortID)
										t.Errorf("Mismatch for out src %s dst %s, inner src %s dst %s", outSrc, outDst, inSrc, inDst)

									} else {
										t.Logf("Match for out src %s dst %s, inner src %s dst %s", outSrc, outDst, inSrc, inDst)
									}

								} else {
									t.Errorf("Metadata ingress port mismatch: want %d, got %d", METADATA_INGRESS_PORT, got)
								}

								if got := metaData[1].GetMetadataId(); got == METADATA_EGRESS_PORT {
									if gotPortID := string(metaData[1].GetValue()); gotPortID != portIDe {
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

}
