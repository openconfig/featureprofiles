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

// Package setup is scoped only to be used for scripts in path
// feature/experimental/system/gnmi/benchmarking/otg_tests/
// Do not use elsewhere.
package gribi_scale_profile

import (
	// "context"
	// "slices"
	// "strconv"
	// "context"
	// "strings"
	"testing"
	// "time"

	// "github.com/openconfig/featureprofiles/internal/deviations"
	// "github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	// "github.com/openconfig/featureprofiles/internal/gribi"
	// "github.com/openconfig/gribigo/fluent"
	// "github.com/openconfig/ondatra"
	// "github.com/openconfig/featureprofiles/internal/gribi"
	// "github.com/openconfig/gribigo/fluent"
	// "github.com/openconfig/ondatra"
	// "github.com/openconfig/ondatra/gnmi"
)

const (
	nh1ID                     = 120
	nhg1ID                    = 20
	ipv4OuterDest             = "192.51.100.65"
	innerV4DstIP              = "198.18.1.1"
	innerV4SrcIP              = "198.18.0.255"
	innerV6SrcIP              = "2001:DB8::198:1"
	innerV6DstIP              = "2001:DB8:2:0:192::10"
	transitVrfIP              = "203.0.113.1"
	repairedVrfIP             = "203.0.113.100"
	noMatchSrcIP              = "198.100.200.123"
	decapMixPrefix1           = "192.51.128.0/22"
	decapMixPrefix2           = "192.55.200.3/32"
	src111TeDstFlowFilter     = "4043" // Egress tracking flow filter decimal value for first 4 bits of last octet of SA 198.51.100.111 + First 8 bits of first octet of TE DA 203.0.113.1
	src222TeDstFlowFilter     = "3787" // Egress tracking flow filter decimal value for first 4 bits of last octet of SA 198.51.100.222 + First 8 bits of first octet of TE DA 203.0.113.100
	noMatchSrcEncapDstFilter  = "2954" // Egress tracking flow filter decimal value for first 4 bits of last octet of SA 198.100.200.123 + First 8 bits of first octet of TE DA 138.0.11.8
	IPinIPProtocolFieldOffset = 184
	IPinIPProtocolFieldWidth  = 8
	IPinIPpSrcDstIPOffset     = 236
	IPinIPpSrcDstIPWidth      = 12
	IPinIPpDscpOffset         = 120
	IPinIPpDscpWidth          = 8
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestGribiScaleProfile(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with decapencap/decap, verify traffic, reprogram & delete ipv4/NHG/NH")
	// dut := ondatra.DUT(t, "dut")
	// ctx := context.Background()
	configureBaseProfile(t)
	// Configure the gRIBI client
	// client := gribi.Client{
	// 	DUT:                   dut,
	// 	FibACK:                *ciscoFlags.GRIBIFIBCheck,
	// 	Persistence:           true,
	// 	InitialElectionIDLow:  10,
	// 	InitialElectionIDHigh: 0,
	// }
	// defer client.Close(t)
	// if err := client.Start(t); err != nil {
	// 	t.Fatalf("gRIBI Connection can not be established")
	// }

	// args.ctx = ctx
	// args.client = &client
	// args.dut = dut
	// args.client.BecomeLeader(t)
	// args.client.FlushServer(t)
	// time.Sleep(10 * time.Second)

	// nh := 1
	// for i := 1; i <= 62; i++ {
	// 	args.client.AddNH(t, uint64(nh), atePort2.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	// 	nh++
	// }
	// for i := 1; i <= 60; i++ {
	// 	args.client.AddNH(t, uint64(nh), atePort4.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	// 	nh++
	// }
	// for i := 1; i <= 60; i++ {
	// 	args.client.AddNH(t, uint64(nh), atePort6.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	// 	nh++
	// }
	// for i := 1; i <= 40; i++ {
	// 	args.client.AddNH(t, uint64(nh), atePort8.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	// 	nh++
	// }
	// for i := 1; i <= 8; i++ {
	// 	args.client.AddNH(t, uint64(nh), atePort5.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	// 	nh++
	// }
	// for i := 1; i <= 26; i++ {
	// 	args.client.AddNH(t, uint64(nh), atePort2.ip(uint8(i)), *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	// 	nh++
	// }

	// wt := 1
	// var i, j, k uint64

	// k = 1
	// for i = 1; i <= 2; i++ {
	// 	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(i))

	// 	for j = 1; j <= 128; j++ {
	// 		nhg.AddNextHop(k, uint64(wt))
	// 		args.client.Fluent(t).Modify().AddEntry(t, nhg)
	// 		if j%2 == 0 && wt <= 63 {
	// 			wt = 10
	// 		} else {
	// 			wt = 31
	// 		}
	// 		k++
	// 	}
	// }
	// wt = 1

	// dstPfx2 := "205.205.205.1"
	// prefixes := []string{}
	// for i := 0; i < 256; i++ {
	// 	prefixes = append(prefixes, util.GetIPPrefix(dstPfx2, i, mask))
	// }
	// nhgID := 1
	// nhgIDs := 2
	// i = 1
	// for _, prefix := range prefixes {
	// 	if i%2 == 0 {
	// 		args.client.AddIPv4(t, prefix, uint64(nhgID), *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	// 	} else {
	// 		args.client.AddIPv4(t, prefix, uint64(nhgIDs), *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 	}
	// 	i++
	// }
	// nh = 1000
	// wt = 1
	// for _, prefix := range prefixes {
	// 	if nh >= 1128 {
	// 		nh = 1000
	// 	}
	// 	b := strings.Split(prefix, "/")
	// 	prefix = b[0]
	// 	args.client.AddNH(t, uint64(nh), prefix, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	// 	nh++
	// }
	// nhgi := 1000

	// dstPfxt := "200.200.200.1"
	// prefixest := []string{}
	// for i := 0; i < 128; i++ {
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
	// 	NHEntry = NHEntry.WithNextHopNetworkInstance("REPAIRED")
	// 	args.client.Fluent(t).Modify().AddEntry(t, NHEntry)
	// 	nhg.AddNextHop(uint64(nhge), uint64(100))
	// 	args.client.Fluent(t).Modify().AddEntry(t, nhg)
	// 	nhge++
	// }
	// nh = 1000
	// nhge = 20000
	// nhgi = 1000

	// for j = 1; j <= 128; j++ {
	// 	if j%2 == 0 && wt <= 255 {
	// 		wt = 10
	// 	} else {
	// 		wt = 255
	// 	}
	// 	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(nhgi))
	// 	nhg.AddNextHop(uint64(nh), uint64(wt))
	// 	nhg.WithBackupNHG(uint64(nhge))
	// 	args.client.Fluent(t).Modify().AddEntry(t, nhg)
	// 	nh++
	// 	nhgi++
	// 	nhge++
	// }
	// nhgi = 1000

	// prefixese := []string{}
	// dstPfxe := "170.170.170.1"
	// for i := 0; i < 128; i++ {
	// 	prefixese = append(prefixese, util.GetIPPrefix(dstPfxe, i, mask))
	// }

	// nhgi = 1000
	// for _, prefix := range prefixese {
	// 	ipv4Entry := fluent.IPv4Entry().
	// 		WithNetworkInstance("TE").
	// 		WithPrefix(prefix).
	// 		WithNextHopGroup(uint64(nhgi)).
	// 		WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	// 	args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	// 	nhgi++
	// }

	// args.client.AddNH(t, 30000, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	// args.client.AddNHG(t, 30000, 0, map[uint64]uint64{30000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// args.client.AddNHG(t, 31000, 30000, map[uint64]uint64{uint64(nhgi - 1): 1, uint64(nhgi - 2): 255}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	// args.client.AddIPv4Batch(t, prefixest, 31000, "REPAIRED", *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// //L2
	// nh = 3001
	// for _, prefix := range prefixese {
	// 	// if nh >= 3128 {
	// 	// 	nh = 3001
	// 	// }
	// 	b := strings.Split(prefix, "/")
	// 	prefix = b[0]
	// 	//args.client.AddNH(t, uint64(nh), prefix, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	// 	args.client.AddNH(t, uint64(nh), "Encap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: []string{prefix}, VrfName: "TE"})
	// 	nh++
	// }

	// nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(3000))

	// for j = 3001; j < 3128; j++ {
	// 	if j%2 == 0 && wt <= 255 {
	// 		wt = 10
	// 	} else {
	// 		wt = 255
	// 	}
	// 	nhg.AddNextHop(j, uint64(wt))
	// 	args.client.Fluent(t).Modify().AddEntry(t, nhg)
	// }
	// //
	// dstPfxd := "105.105.105.1"
	// prefixess := []string{}
	// for i := 0; i < 10; i++ {
	// 	prefixess = append(prefixess, util.GetIPPrefix(dstPfxd, i, mask))
	// }

	// for _, prefix := range prefixess {
	// 	ipv4Entry := fluent.IPv4Entry().
	// 		WithNetworkInstance(vrfEncapA).
	// 		WithPrefix(prefix).
	// 		WithNextHopGroup(uint64(3000)).
	// 		WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	// 	args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	// }

	// nh = 4000
	// args.client.AddNH(t, uint64(nh), "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{VrfName: vrfEncapA})

	// args.client.AddNHG(t, 4000, 0, map[uint64]uint64{4000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// prefixesd := []string{}
	// for i := 0; i < 100; i++ {
	// 	prefixesd = append(prefixesd, util.GetIPPrefix(dstPfx, i, mask))
	// }

	// for _, prefix := range prefixesd {
	// 	ipv4Entry := fluent.IPv4Entry().
	// 		WithNetworkInstance(vrfDecap).
	// 		WithPrefix(prefix).
	// 		WithNextHopGroup(uint64(4000)).
	// 		WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	// 	args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	// }
	// t.Log("case 4 prog done")
	// //time.Sleep(5 * time.Minute)
	// if *ciscoFlags.GRIBITrafficCheck {
	// 	//args.validateTrafficFlows(t, args.allFlows(t), false, []string{atePort2.Name, atePort4.Name, atePort6.Name, atePort8.Name})
	// 	//args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort1.Name, DstIP: dstPfx, Scalenum: 100}), false, []string{atePort2.Name, atePort4.Name, atePort6.Name, atePort8.Name}, &TGNoptions{Ifname: atePort1.Name})
	// 	testTraffic(t, args.ate, args.top, 100)

	// 	//args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort1.Name, SrcIP: atePort1.IPv4, DstIP: dstPfx, Scalenum: 255}), false, []string{bundleEther125}, &TGNoptions{Ifname: atePort1.Name})

	// }
}
