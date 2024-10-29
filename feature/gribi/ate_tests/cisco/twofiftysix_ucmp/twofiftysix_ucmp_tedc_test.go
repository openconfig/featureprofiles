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
	"github.com/openconfig/featureprofiles/internal/cisco/ha/utils"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/gribigo/fluent"

	//"github.com/openconfig/featureprofiles/internal/gribi"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
)

// Settings for configuring the baseline testbed with the test
// topology.
//
// The testbed consists of dut:port1 -> ate:port1,
// dut:port2 -> ate:port2, dut:port3 -> ate:port3, dut:port4 -> ate:port4, dut:port5 -> ate:port5,
// dut:port6 -> ate:port6, dut:port7 -> ate:port7 ,dut:port8 -> ate:port8

func TestWithDecapEncapTEBackUpmin(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with decapencap/decap, verify traffic, reprogram & delete ipv4/NHG/NH")

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	baseconfig(t)
	addStaticRoute(t, dut, "197.51.0.0/16", true)
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
