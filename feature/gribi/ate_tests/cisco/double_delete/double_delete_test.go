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

package double_delete_test

import (
	"context"

	//"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/gribi"
	"github.com/openconfig/featureprofiles/internal/cisco/ha/confgen"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gribigo/fluent"
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
)

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx    context.Context
	client *gribi.Client
	dut    *ondatra.DUTDevice
	ate    *ondatra.ATEDevice
	top    *ondatra.ATETopology
	rpfo   bool
}

func TestDeleteIpv4NHGNH(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries on default vrf, verify traffic, delete entries")

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	var top *ondatra.ATETopology
	ctx := context.Background()
	ip := "203.0.2.1/32"

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
	args := &testArgs{
		ctx:    ctx,
		client: &client,
		dut:    dut,
		ate:    ate,
		top:    top,
		rpfo:   false,
	}
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)
	unconfigbasePBR(t, dut, "PBR", bundleEther120)
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 10, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther127, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 100, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
	}

	if args.rpfo {
		confgen.Dorpfo(args.ctx, t, true)
	}

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
	}

	for s := 0; s < 4; s++ {

		args.client.DeleteIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 100, 101, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 10, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther127, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 100, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)

		if *ciscoFlags.GRIBITrafficCheck {
			args.validateTrafficFlows(t, args.allFlows(t), true, []string{bundleEther121, bundleEther127})
		}

		//Non-existing
		args.client.DeleteIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 100, 101, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 10, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther127, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 100, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)

		//Random non-existing
		args.client.DeleteIPv4(t, ip, 111, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 111, 121, map[uint64]uint64{111: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 111, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther127, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 121, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)

		args.client.DeleteIPv4(t, ip, 111, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 111, 121, map[uint64]uint64{111: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 111, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther127, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 121, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
	}
}

func TestDeleteIpv4(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries on default vrf, verify traffic, reprogram & delete entries")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	var top *ondatra.ATETopology
	ctx := context.Background()
	ip := "203.1.2.1/32"

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
	args := &testArgs{
		ctx:    ctx,
		client: &client,
		dut:    dut,
		ate:    ate,
		top:    top,
		rpfo:   false,
	}

	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 10, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther127, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 100, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}

	if args.rpfo {
		confgen.Dorpfo(args.ctx, t, true)
	}
	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
	}

	for s := 0; s < 4; s++ {

		args.client.DeleteIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		if *ciscoFlags.GRIBITrafficCheck {
			args.validateTrafficFlows(t, args.allFlows(t), true, []string{bundleEther121, bundleEther127})
		}

		args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

		args.client.DeleteIPv4(t, ip, 121, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, ip, 121, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.DeleteIPv4(t, ip, 121, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, ip, 121, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	}
	args.client.FlushServer(t)
	args.client.FlushServer(t)
}

func TestDeleteNHG(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries on default vrf, verify traffic, delete ipv4/NHG")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	var top *ondatra.ATETopology
	ctx := context.Background()

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
	args := &testArgs{
		ctx:    ctx,
		client: &client,
		dut:    dut,
		ate:    ate,
		top:    top,
		rpfo:   false,
	}

	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 10, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther127, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 100, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
	}

	if args.rpfo {
		confgen.Dorpfo(args.ctx, t, true)
	}

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
	}

	for s := 0; s < 4; s++ {

		args.client.DeleteIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 100, 101, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

		if *ciscoFlags.GRIBITrafficCheck {
			args.validateTrafficFlows(t, args.allFlows(t), true, []string{bundleEther121, bundleEther127})
		}

		//Non-existing
		args.client.DeleteNHG(t, 100, 101, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

		//Random non-existing
		args.client.DeleteNHG(t, 111, 121, map[uint64]uint64{111: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 111, 121, map[uint64]uint64{111: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	}
}

func TestDeleteNH(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries on default vrf, verify traffic, delete ipv4/NHG/NH")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	var top *ondatra.ATETopology
	ctx := context.Background()

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
	args := &testArgs{
		ctx:    ctx,
		client: &client,
		dut:    dut,
		ate:    ate,
		top:    top,
		rpfo:   false,
	}

	args.client.BecomeLeader(t)
	args.client.FlushServer(t)
	dut = ondatra.DUT(t, "dut")

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 10, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther127, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 100, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
	}

	if args.rpfo {
		confgen.Dorpfo(args.ctx, t, true)

		if *ciscoFlags.GRIBITrafficCheck {
			args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
		}
	}

	for s := 0; s < 4; s++ {

		args.client.DeleteIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 100, 101, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNH(t, 10, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther127, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 100, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 10, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther127, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 100, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)

		if *ciscoFlags.GRIBITrafficCheck {
			args.validateTrafficFlows(t, args.allFlows(t), true, []string{bundleEther121, bundleEther127})
		}

		//Random non-existing

		args.client.DeleteNH(t, 111, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther127, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 121, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 111, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther127, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 121, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
	}
	defer configbasePBR(t, dut, "TE", "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{}, "PBR", bundleEther120)
}

func TestWithBackup(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with backup path, verify traffic, delete ipv4/NHG/NH")

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	var top *ondatra.ATETopology
	ctx := context.Background()

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
	args := &testArgs{
		ctx:    ctx,
		client: &client,
		dut:    dut,
		ate:    ate,
		top:    top,
		rpfo:   false,
	}
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	configbasePBR(t, dut, "TE", "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{}, "PBR", bundleEther120)

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther122, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 60, 1100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, vip1, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther123, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, vip2, 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 1, vip1ip, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2, vip2ip, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2, 0, map[uint64]uint64{2: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1, 2, map[uint64]uint64{1: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121, bundleEther122})
	}

	if args.rpfo {
		confgen.Dorpfo(args.ctx, t, true)

		if *ciscoFlags.GRIBITrafficCheck {
			args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121, bundleEther122})
		}
	}
	//Delete  twice
	for s := 0; s < 4; s++ {

		args.client.DeleteIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip1, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip2, 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1, 2, map[uint64]uint64{1: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2, 0, map[uint64]uint64{2: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1, vip1, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2, vip2, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

		args.client.DeleteIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip1, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip2, 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 1, 2, map[uint64]uint64{1: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2, 0, map[uint64]uint64{2: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1, vip1, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2, vip2, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther122, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther123, false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther122, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther123, false, ciscoFlags.GRIBIChecks)
	}
}

func TestWithBackupDelete(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with backup, verify traffic, reprogram & delete ipv4/NHG/NH")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	var top *ondatra.ATETopology
	ctx := context.Background()

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
	args := &testArgs{
		ctx:    ctx,
		client: &client,
		dut:    dut,
		ate:    ate,
		top:    top,
		rpfo:   false,
	}

	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther122, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 60, 1100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, vip1, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther123, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, vip2, 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 1, vip1ip, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2, vip2ip, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2, 0, map[uint64]uint64{2: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1, 2, map[uint64]uint64{1: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121, bundleEther122})
	}

	if args.rpfo {
		confgen.Dorpfo(args.ctx, t, true)

		if *ciscoFlags.GRIBITrafficCheck {
			args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121, bundleEther122})
		}
	}

	//Delete  twice

	args.client.DeleteIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, vip1, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, vip2, 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, vip1, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, vip2, 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	//Reprogram
	args.client.AddIPv4(t, vip1, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, vip2, 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	for s := 0; s < 4; s++ {

		args.client.DeleteIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip1, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip2, 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		//Delete NHG, NH

		args.client.DeleteNHG(t, 1, 2, map[uint64]uint64{1: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2, 0, map[uint64]uint64{2: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1, vip1, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2, vip2, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 1, 2, map[uint64]uint64{1: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2, 0, map[uint64]uint64{2: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNH(t, 1, vip1, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2, vip2, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther122, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther123, false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther122, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther123, false, ciscoFlags.GRIBIChecks)

	}
}

func TestWithDecapEncap(t *testing.T) {
	ctx := context.Background()

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with decapencap/decap, verify traffic, delete ipv4/NHG/NH")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	var top *ondatra.ATETopology

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
	args := &testArgs{
		ctx:    ctx,
		client: &client,
		dut:    dut,
		ate:    ate,
		top:    top,
		rpfo:   false,
	}

	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	args.client.AddNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 3000, 0, map[uint64]uint64{3000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, dip, 3000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: "222.222.222.222", Dest: []string{dsip}})
	args.client.AddNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
	}

	if args.rpfo {
		confgen.Dorpfo(args.ctx, t, true)

		if *ciscoFlags.GRIBITrafficCheck {
			args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
		}
	}

	for s := 0; s < 4; s++ {

		//Delete  twice
		args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, dip, 3000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 3000, 0, map[uint64]uint64{3000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)

		args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, dip, 3000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 3000, 0, map[uint64]uint64{3000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)

	}
}

func TestWithDecapEncapDelete(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with decapencap/decap, verify traffic, reprogram & delete ipv4/NHG/NH")

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	var top *ondatra.ATETopology
	ctx := context.Background()

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
	args := &testArgs{
		ctx:    ctx,
		client: &client,
		dut:    dut,
		ate:    ate,
		top:    top,
		rpfo:   false,
	}
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	args.client.AddNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 3000, 0, map[uint64]uint64{3000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, dip, 3000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: "222.222.222.222", Dest: []string{dsip}})
	args.client.AddNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
	}

	if args.rpfo {
		confgen.Dorpfo(args.ctx, t, true)

		if *ciscoFlags.GRIBITrafficCheck {
			args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
		}
	}

	//Delete  twice

	args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, dip, 3000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, dip, 3000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	//Reprogram

	args.client.AddIPv4(t, dip, 3000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	for s := 0; s < 4; s++ {

		args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, dip, 3000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 3000, 0, map[uint64]uint64{3000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 3000, 0, map[uint64]uint64{3000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)

	}
}

func TestWithDecapEncapvrf(t *testing.T) {

	// reference case details
	// https://cisco.sharepoint.com/:p:/r/Sites/Spitfire-Test/_layouts/15/Doc.aspx?sourcedoc=%7BBBBAE62E-F6C2-41B2-A3F6-305268149284%7D&file=VRF_Fallback_scenario.pptx&action=edit&mobileredirect=true

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with decapencap/decap with nh on vrf, verify traffic, delete ipv4/NHG/NH")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	var top *ondatra.ATETopology
	ctx := context.Background()

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
	args := &testArgs{
		ctx:    ctx,
		client: &client,
		dut:    dut,
		ate:    ate,
		top:    top,
		rpfo:   false,
	}

	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	args.client.AddNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 3000, 0, map[uint64]uint64{3000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, dip, 3000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 1000, "DecapEncapvrf", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: "222.222.222.222", Dest: []string{dsip}, VrfName: *ciscoFlags.NonDefaultNetworkInstance})
	args.client.AddNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
	}

	if args.rpfo {
		confgen.Dorpfo(args.ctx, t, true)

		if *ciscoFlags.GRIBITrafficCheck {
			args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
		}
	}

	//Delete  twice
	for s := 0; s < 4; s++ {

		args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, dip, 3000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 3000, 0, map[uint64]uint64{3000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)

		args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, dip, 3000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 3000, 0, map[uint64]uint64{3000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)

	}
}

func TestWithDecapEncapvrfDelete(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with decapencap/decap with nh on vrf, verify traffic, reprogram & delete ipv4/NHG/NH")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	var top *ondatra.ATETopology
	ctx := context.Background()

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
	args := &testArgs{
		ctx:    ctx,
		client: &client,
		dut:    dut,
		ate:    ate,
		top:    top,
		rpfo:   false,
	}

	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	args.client.AddNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 3000, 0, map[uint64]uint64{3000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, dip, 3000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 1000, "DecapEncapvrf", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: "222.222.222.222", Dest: []string{dsip}, VrfName: *ciscoFlags.NonDefaultNetworkInstance})
	args.client.AddNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
	}

	if args.rpfo {
		confgen.Dorpfo(args.ctx, t, true)

		if *ciscoFlags.GRIBITrafficCheck {
			args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
		}
	}
	//Delete  twice

	args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, dip, 3000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, dip, 3000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	//Reprogram
	args.client.AddIPv4(t, dip, 3000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	for s := 0; s < 4; s++ {

		args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, dip, 3000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 3000, 0, map[uint64]uint64{3000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 3000, 0, map[uint64]uint64{3000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)

	}
}

func TestWithBackupDecap(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with backup decap with nh on vrf, verify traffic, delete ipv4/NHG/NH")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	var top *ondatra.ATETopology
	ctx := context.Background()

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
	args := &testArgs{
		ctx:    ctx,
		client: &client,
		dut:    dut,
		ate:    ate,
		top:    top,
		rpfo:   false,
	}

	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 2000, map[uint64]uint64{1000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
	}

	if args.rpfo {
		confgen.Dorpfo(args.ctx, t, true)

		if *ciscoFlags.GRIBITrafficCheck {
			args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
		}
	}

	//Delete  twice
	for s := 0; s < 4; s++ {

		args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

		args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

	}
}

func TestWithBackupDecapDelete(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	var top *ondatra.ATETopology
	ctx := context.Background()

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
	args := &testArgs{
		ctx:    ctx,
		client: &client,
		dut:    dut,
		ate:    ate,
		top:    top,
		rpfo:   false,
	}

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with backup decap with nh on vrf, verify traffic, reprogram & delete ipv4/NHG/NH")
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 2000, map[uint64]uint64{1000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
	}

	if args.rpfo {
		confgen.Dorpfo(args.ctx, t, true)

		if *ciscoFlags.GRIBITrafficCheck {
			args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
		}
	}

	//Delete  twice

	args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	for s := 0; s < 4; s++ {

		//Reprogram
		args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

	}
}

func TestWithScale(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program scale gribi entries ~17K NH, 500 NHG/NH decapencap, 1K NHG, 1K default prefixes, 60K vrf prefixes verify traffic and delete")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	var top *ondatra.ATETopology
	ctx := context.Background()

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
	args := &testArgs{
		ctx:    ctx,
		client: &client,
		dut:    dut,
		ate:    ate,
		top:    top,
		rpfo:   false,
	}

	args.client.BecomeLeader(t)
	args.client.FlushServer(t)
	time.Sleep(10 * time.Second)
	dstPfxx := "198.51.100.1"

	configbasePBR(t, dut, "TE2", "ipv4", 2, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{}, "PBR2", bundleEther121)
	configbasePBR(t, dut, "TE3", "ipv4", 3, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{}, "PBR3", bundleEther122)
	defer unconfigbasePBR(t, dut, "PBR2", bundleEther121)
	defer unconfigbasePBR(t, dut, "PBR3", bundleEther122)
	var nh1, nh2 uint64 = 1, 33
	var i, j uint64
	for i = 1; i <= 32; i++ {
		args.client.AddNH(t, nh1, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther123, false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, nh2, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther124, false, ciscoFlags.GRIBIChecks)
		nh1 = nh1 + 1
		nh2 = nh2 + 1
	}
	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(1))

	for j = 1; j < 64; j++ {
		nhg.AddNextHop(j, uint64(64))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
	}

	var nh uint64 = 0
	NHEntry := fluent.NextHopEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance)

	for i = 2; i <= 499; i++ {
		nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID((i))
		for j = 0; j < 34; j++ {
			NHEntry = NHEntry.WithIPAddress(atePort4.IPv4).WithIndex(uint64(200 + nh))
			args.client.Fluent(t).Modify().AddEntry(t, NHEntry)
			nhg.AddNextHop(uint64(200+nh), uint64(10))
			args.client.Fluent(t).Modify().AddEntry(t, nhg)
			nh++
		}
	}

	dstPfx3 := "198.101.1.1"
	prefixes := []string{}
	for s := 0; s < 499; s++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx3, s, mask))
	}

	for _, prefix := range prefixes {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance("DEFAULT").
			WithPrefix(prefix).
			WithNextHopGroup(uint64(1)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	}

	var prefix string
	k, l := 1, 1
	for j = 1; j < 31; j++ {
		count := 0
		for i = 1; i < 499; i++ {
			prefix = util.GetIPPrefix(dstPfxx, k, "32")
			ipv4Entry := fluent.IPv4Entry().
				WithNetworkInstance("TE").
				WithPrefix(prefix).
				WithNextHopGroup(uint64(2 + count)).
				WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)

			args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
			ipv4Entry1 := fluent.IPv4Entry().
				WithNetworkInstance("TE2").
				WithPrefix(prefix).
				WithNextHopGroup(uint64(2 + count)).
				WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)

			args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry1)
			ipv4Entry2 := fluent.IPv4Entry().
				WithNetworkInstance("TE3").
				WithPrefix(prefix).
				WithNextHopGroup(uint64(2 + count)).
				WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
			args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry2)
			ipv4Entry3 := fluent.IPv4Entry().
				WithNetworkInstance("DEFAULT").
				WithPrefix(prefix).
				WithNextHopGroup(uint64(2 + count)).
				WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
			args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry3)
			count++
			k++

		}
		b := strings.Split(dstPfx, ".")
		Var1, _ := strconv.Atoi(b[1])
		Var2, _ := strconv.Atoi(b[2])
		Var3, _ := strconv.Atoi(b[3])
		Var0, _ := strconv.Atoi(b[0])
		Var1 = Var1 + 1
		var1 := byte(Var1)
		var0 := byte(Var0)
		var3 := byte(Var3)
		var2 := byte(Var2)
		dst := net.IPv4(var0, var1, var2, var3)
		dstPfxx = dst.String()
		l++
	}
	NHEntry = fluent.NextHopEntry()

	for i = 20000; i < 20499; i++ {
		nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID((i))
		NHEntry = NHEntry.WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithIndex(i)
		NHEntry = NHEntry.WithDecapsulateHeader(fluent.IPinIP)
		NHEntry = NHEntry.WithEncapsulateHeader(fluent.IPinIP)
		NHEntry = NHEntry.WithIPinIP("222.222.222.222", dsip)
		args.client.Fluent(t).Modify().AddEntry(t, NHEntry)
		nhg.AddNextHop(i, uint64(10))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
	}
	prefixess := []string{}
	dstPfx2 := "198.100.1.1"
	for i := 0; i < 499; i++ {
		prefixess = append(prefixess, util.GetIPPrefix(dstPfx2, i, mask))
	}
	count := 0
	for _, prefix := range prefixess {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance(*ciscoFlags.NonDefaultNetworkInstance).
			WithPrefix(prefix).
			WithNextHopGroup(uint64(20000 + count)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
		count++

	}

	args.client.AddNH(t, 30000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther125, false, ciscoFlags.GRIBIChecks)

	args.client.AddNHG(t, 30000, 0, map[uint64]uint64{30000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, dip, 30000, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort7.Name, SrcIP: atePort7.IPv4, DstIP: dstPfx3, Scalenum: 255}), false, []string{bundleEther123, bundleEther124}, &TGNoptions{Ifname: bundleEther126})
		args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort7.Name, SrcIP: atePort7.IPv4, DstIP: "198.101.2.1", Scalenum: 243}), false, []string{bundleEther123, bundleEther124}, &TGNoptions{Ifname: bundleEther126})
		args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort7.Name, SrcIP: atePort7.IPv4, DstIP: "198.51.100.2", Scalenum: 243}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther126})
		args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort7.Name, SrcIP: atePort7.IPv4, DstIP: "198.52.101.244", Scalenum: 14442}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther126})

		args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort1.Name, SrcIP: atePort1.IPv4, DstIP: "198.51.100.2", Scalenum: 243}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther120})
		args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort1.Name, SrcIP: atePort1.IPv4, DstIP: "198.52.101.244", Scalenum: 14442}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther120})
		args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort1.Name, SrcIP: atePort1.IPv4, DstIP: dstPfx2, Scalenum: 255}), false, []string{bundleEther125}, &TGNoptions{Ifname: bundleEther120})
		args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort1.Name, SrcIP: atePort1.IPv4, DstIP: "198.100.2.1", Scalenum: 243}), false, []string{bundleEther125}, &TGNoptions{Ifname: bundleEther120})

		args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort2.Name, SrcIP: atePort2.IPv4, DstIP: "198.51.100.2", Scalenum: 243}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther121})
		args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort2.Name, SrcIP: atePort2.IPv4, DstIP: "198.52.101.244", Scalenum: 14442}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther121})

		args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort3.Name, SrcIP: atePort3.IPv4, DstIP: "198.51.100.2", Scalenum: 243}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther122})
		args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort3.Name, SrcIP: atePort3.IPv4, DstIP: "198.52.101.244", Scalenum: 14442}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther122})

	}

	if args.rpfo {
		confgen.Dorpfo(args.ctx, t, true)

		if *ciscoFlags.GRIBITrafficCheck {
			args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort7.Name, SrcIP: atePort7.IPv4, DstIP: dstPfx3, Scalenum: 255}), false, []string{bundleEther123, bundleEther124}, &TGNoptions{Ifname: bundleEther126})
			args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort7.Name, SrcIP: atePort7.IPv4, DstIP: "198.101.2.1", Scalenum: 243}), false, []string{bundleEther123, bundleEther124}, &TGNoptions{Ifname: bundleEther126})
			args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort7.Name, SrcIP: atePort7.IPv4, DstIP: "198.51.100.2", Scalenum: 243}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther126})
			args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort7.Name, SrcIP: atePort7.IPv4, DstIP: "198.52.101.244", Scalenum: 14442}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther126})

			args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort1.Name, SrcIP: atePort1.IPv4, DstIP: "198.51.100.2", Scalenum: 243}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther120})
			args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort1.Name, SrcIP: atePort1.IPv4, DstIP: "198.52.101.244", Scalenum: 14442}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther120})
			args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort1.Name, SrcIP: atePort1.IPv4, DstIP: dstPfx2, Scalenum: 255}), false, []string{bundleEther125}, &TGNoptions{Ifname: bundleEther120})
			args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort1.Name, SrcIP: atePort1.IPv4, DstIP: "198.100.2.1", Scalenum: 243}), false, []string{bundleEther125}, &TGNoptions{Ifname: bundleEther120})

			args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort2.Name, SrcIP: atePort2.IPv4, DstIP: "198.51.100.2", Scalenum: 243}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther121})
			args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort2.Name, SrcIP: atePort2.IPv4, DstIP: "198.52.101.244", Scalenum: 14442}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther121})

			args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort3.Name, SrcIP: atePort3.IPv4, DstIP: "198.51.100.2", Scalenum: 243}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther122})
			args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort3.Name, SrcIP: atePort3.IPv4, DstIP: "198.52.101.244", Scalenum: 14442}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther122})

		}
	}

	prefixes1 := []string{}

	for i := 0; i < 1000; i++ {
		prefixes1 = append(prefixes1, util.GetIPPrefix("198.51.100.0", i, mask))
	}

	prefixes3 := []string{}

	for i := 0; i < 15000; i++ {
		prefixes3 = append(prefixes3, util.GetIPPrefix("198.52.101.0", i, mask))
	}

	for s := 0; s < 4; s++ {
		for _, prefix := range prefixes1 {
			ipv4Entry := fluent.IPv4Entry().WithPrefix(prefix).WithNetworkInstance("DEFAULT")
			args.client.Fluent(t).Modify().DeleteEntry(t, ipv4Entry)
			ipv4Entry1 := fluent.IPv4Entry().WithPrefix(prefix).WithNetworkInstance("TE")
			args.client.Fluent(t).Modify().DeleteEntry(t, ipv4Entry1)
			ipv4Entry2 := fluent.IPv4Entry().WithPrefix(prefix).WithNetworkInstance("TE2")
			args.client.Fluent(t).Modify().DeleteEntry(t, ipv4Entry2)
			ipv4Entry3 := fluent.IPv4Entry().WithPrefix(prefix).WithNetworkInstance("TE3")
			args.client.Fluent(t).Modify().DeleteEntry(t, ipv4Entry3)
		}

		for _, prefix := range prefixes3 {
			ipv4Entry := fluent.IPv4Entry().WithPrefix(prefix).WithNetworkInstance("DEFAULT")
			args.client.Fluent(t).Modify().DeleteEntry(t, ipv4Entry)
			ipv4Entry1 := fluent.IPv4Entry().WithPrefix(prefix).WithNetworkInstance("TE")
			args.client.Fluent(t).Modify().DeleteEntry(t, ipv4Entry1)
			ipv4Entry2 := fluent.IPv4Entry().WithPrefix(prefix).WithNetworkInstance("TE2")
			args.client.Fluent(t).Modify().DeleteEntry(t, ipv4Entry2)
			ipv4Entry3 := fluent.IPv4Entry().WithPrefix(prefix).WithNetworkInstance("TE3")
			args.client.Fluent(t).Modify().DeleteEntry(t, ipv4Entry3)
		}

		for _, prefix := range prefixess {
			ipv4Entry1 := fluent.IPv4Entry().WithPrefix(prefix).WithNetworkInstance("TE")
			args.client.Fluent(t).Modify().DeleteEntry(t, ipv4Entry1)
		}
		for _, prefix := range prefixes {
			ipv4Entry1 := fluent.IPv4Entry().WithPrefix(prefix).WithNetworkInstance("DEFAULT")
			args.client.Fluent(t).Modify().DeleteEntry(t, ipv4Entry1)
		}

		args.client.DeleteIPv4(t, dip, 30000, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

		for j = 1; j < 40000; j++ {
			nhg := fluent.NextHopGroupEntry().WithNetworkInstance("DEFAULT").WithID(j)
			args.client.Fluent(t).Modify().DeleteEntry(t, nhg)
		}

		for j = 1; j < 40000; j++ {
			NH := fluent.NextHopEntry().
				WithNetworkInstance("DEFAULT").
				WithIndex(j)
			args.client.Fluent(t).Modify().DeleteEntry(t, NH)
		}
	}
}

func TestWithStatic(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program static route entries, and then through gribi, verify traffic, delete gribi entries")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	var top *ondatra.ATETopology
	ctx := context.Background()

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
	args := &testArgs{
		ctx:    ctx,
		client: &client,
		dut:    dut,
		ate:    ate,
		top:    top,
		rpfo:   false,
	}

	args.client.BecomeLeader(t)
	args.client.FlushServer(t)
	time.Sleep(10 * time.Second)

	addStaticRoute(t, dut, vip1, true)
	addStaticRoute(t, dut, vip2, true)

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther122, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 60, 1100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther123, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, vip2, 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 1, vip1ip, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2, vip2ip, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2, 0, map[uint64]uint64{2: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1, 2, map[uint64]uint64{1: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}

	args.client.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther126})
	}

	if args.rpfo {
		confgen.Dorpfo(args.ctx, t, true)

		if *ciscoFlags.GRIBITrafficCheck {
			args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther126})
		}
	}

	for s := 0; s < 4; s++ {

		args.client.DeleteIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip1, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip2, 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1, 2, map[uint64]uint64{1: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2, 0, map[uint64]uint64{2: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1, vip1, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2, vip2, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

		args.client.DeleteIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip1, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip2, 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 1, 2, map[uint64]uint64{1: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2, 0, map[uint64]uint64{2: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1, vip1, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2, vip2, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther122, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther123, false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther122, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther123, false, ciscoFlags.GRIBIChecks)
	}
}

func TestWithStaticremove(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Remove static routes, program gribi verify traffic, and delete gribi entries")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	var top *ondatra.ATETopology
	ctx := context.Background()

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
	args := &testArgs{
		ctx:    ctx,
		client: &client,
		dut:    dut,
		ate:    ate,
		top:    top,
		rpfo:   false,
	}
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)
	addStaticRoute(t, dut, vip1, false)
	addStaticRoute(t, dut, vip2, false)

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther122, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 60, 1100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, vip1, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther123, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, vip2, 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 1, vip1ip, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2, vip2ip, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2, 0, map[uint64]uint64{2: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1, 2, map[uint64]uint64{1: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121, bundleEther122})
	}

	if args.rpfo {
		confgen.Dorpfo(args.ctx, t, true)

		if *ciscoFlags.GRIBITrafficCheck {
			args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121, bundleEther122})
		}
	}
	time.Sleep(20 * time.Minute)

	for s := 0; s < 4; s++ {

		args.client.DeleteIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip1, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip2, 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1, 2, map[uint64]uint64{1: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2, 0, map[uint64]uint64{2: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1, vip1, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2, vip2, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

		args.client.DeleteIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip1, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip2, 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 1, 2, map[uint64]uint64{1: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2, 0, map[uint64]uint64{2: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1, vip1, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2, vip2, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther122, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther123, false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther122, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther123, false, ciscoFlags.GRIBIChecks)
	}
}

func baseconfig(t *testing.T) {
	var baseconfigdone bool
	if !baseconfigdone {
		//Configure the DUT
		dut := ondatra.DUT(t, "dut")

		configureDUT(t, dut)
		configbasePBR(t, dut, "TE", "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{}, "PBR", bundleEther120)
		defer unconfigbasePBR(t, dut, "PBR", bundleEther120)
		//configure route-policy
		configRP(t, dut)
		//configure ISIS on DUT
		util.AddISISOC(t, dut, bundleEther127)
		//configure BGP on DUT
		util.AddBGPOC(t, dut, "100.100.100.100")

		// Configure the ATE
		ate := ondatra.ATE(t, "ate")
		top := configureATE(t, ate)
		if *ciscoFlags.GRIBITrafficCheck {
			addPrototoAte(t, top)
		}
		baseconfigdone = true
	}
}

func addStaticRoute(t *testing.T, dut *ondatra.DUTDevice, ip string, val bool) {
	d := gnmi.OC()
	s := &oc.Root{}
	static := s.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "DEFAULT")
	ipv4Nh := static.GetOrCreateStatic(ip).GetOrCreateNextHop("0")
	ipv4Nh.NextHop, _ = ipv4Nh.To_NetworkInstance_Protocol_Static_NextHop_NextHop_Union(atePort7.IPv4)
	if val {
		gnmi.Update(t, dut, d.NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "DEFAULT").Config(), static)
	} else {
		gnmi.Delete(t, dut, d.NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "DEFAULT").Config())
	}
}

func TestDeleteIpv4NHGNHrpfo(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries on default vrf, verify traffic, delete entries")

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	var top *ondatra.ATETopology
	ctx := context.Background()
	ip := "203.0.2.1/32"

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
	args := &testArgs{
		ctx:    ctx,
		client: &client,
		dut:    dut,
		ate:    ate,
		top:    top,
		rpfo:   true,
	}
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)
	//dut := ondatra.DUT(t, "dut")
	unconfigbasePBR(t, dut, "PBR", bundleEther120)
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 10, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther127, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 100, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
	}

	if args.rpfo {
		confgen.Dorpfo(args.ctx, t, true)
	}

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
	}

	for s := 0; s < 4; s++ {

		args.client.DeleteIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 100, 101, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 10, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther127, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 100, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)

		if *ciscoFlags.GRIBITrafficCheck {
			args.validateTrafficFlows(t, args.allFlows(t), true, []string{bundleEther121, bundleEther127})
		}

		//Non-existing
		args.client.DeleteIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 100, 101, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 10, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther127, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 100, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)

		//Random non-existing
		args.client.DeleteIPv4(t, ip, 111, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 111, 121, map[uint64]uint64{111: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 111, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther127, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 121, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)

		args.client.DeleteIPv4(t, ip, 111, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 111, 121, map[uint64]uint64{111: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 111, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther127, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 121, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
	}
}

func TestWithBackuprpfo(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with backup path, verify traffic, delete ipv4/NHG/NH")

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	var top *ondatra.ATETopology
	ctx := context.Background()

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
	args := &testArgs{
		ctx:    ctx,
		client: &client,
		dut:    dut,
		ate:    ate,
		top:    top,
		rpfo:   true,
	}
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)
	//dut := ondatra.DUT(t, "dut")

	configbasePBR(t, dut, "TE", "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{}, "PBR", bundleEther120)

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther122, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 60, 1100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, vip1, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther123, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, vip2, 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 1, vip1ip, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2, vip2ip, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2, 0, map[uint64]uint64{2: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1, 2, map[uint64]uint64{1: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121, bundleEther122})
	}

	if args.rpfo {
		confgen.Dorpfo(args.ctx, t, true)

		if *ciscoFlags.GRIBITrafficCheck {
			args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121, bundleEther122})
		}
	}
	//Delete  twice
	for s := 0; s < 4; s++ {

		args.client.DeleteIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip1, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip2, 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1, 2, map[uint64]uint64{1: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2, 0, map[uint64]uint64{2: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1, vip1, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2, vip2, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

		args.client.DeleteIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip1, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip2, 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 1, 2, map[uint64]uint64{1: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2, 0, map[uint64]uint64{2: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1, vip1, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2, vip2, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther122, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther123, false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther122, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther123, false, ciscoFlags.GRIBIChecks)
	}
}

func TestWithDecapEncaprpfo(t *testing.T) {
	ctx := context.Background()

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with decapencap/decap, verify traffic, delete ipv4/NHG/NH")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	var top *ondatra.ATETopology

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
	args := &testArgs{
		ctx:    ctx,
		client: &client,
		dut:    dut,
		ate:    ate,
		top:    top,
		rpfo:   true,
	}

	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	args.client.AddNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 3000, 0, map[uint64]uint64{3000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, dip, 3000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: "222.222.222.222", Dest: []string{dsip}})
	args.client.AddNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
	}

	if args.rpfo {
		confgen.Dorpfo(args.ctx, t, true)

		if *ciscoFlags.GRIBITrafficCheck {
			args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
		}
	}

	for s := 0; s < 4; s++ {

		//Delete  twice
		args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, dip, 3000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 3000, 0, map[uint64]uint64{3000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)

		args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, dip, 3000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 3000, 0, map[uint64]uint64{3000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)

	}
}

func TestWithDecapEncapvrfrpfo(t *testing.T) {

	// reference case details
	// https://cisco.sharepoint.com/:p:/r/Sites/Spitfire-Test/_layouts/15/Doc.aspx?sourcedoc=%7BBBBAE62E-F6C2-41B2-A3F6-305268149284%7D&file=VRF_Fallback_scenario.pptx&action=edit&mobileredirect=true

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with decapencap/decap with nh on vrf, verify traffic, delete ipv4/NHG/NH")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	var top *ondatra.ATETopology
	ctx := context.Background()

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
	args := &testArgs{
		ctx:    ctx,
		client: &client,
		dut:    dut,
		ate:    ate,
		top:    top,
		rpfo:   true,
	}

	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	args.client.AddNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 3000, 0, map[uint64]uint64{3000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, dip, 3000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 1000, "DecapEncapvrf", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks, &gribi.NHOptions{Src: "222.222.222.222", Dest: []string{dsip}, VrfName: *ciscoFlags.NonDefaultNetworkInstance})
	args.client.AddNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
	}

	if args.rpfo {
		confgen.Dorpfo(args.ctx, t, true)

		if *ciscoFlags.GRIBITrafficCheck {
			args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
		}
	}

	//Delete  twice
	for s := 0; s < 4; s++ {

		args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, dip, 3000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 3000, 0, map[uint64]uint64{3000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)

		args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, dip, 3000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 3000, 0, map[uint64]uint64{3000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)

	}
}

func TestWithBackupDecaprpfo(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program gribi entries with backup decap with nh on vrf, verify traffic, delete ipv4/NHG/NH")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	var top *ondatra.ATETopology
	ctx := context.Background()

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
	args := &testArgs{
		ctx:    ctx,
		client: &client,
		dut:    dut,
		ate:    ate,
		top:    top,
		rpfo:   true,
	}

	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 2000, map[uint64]uint64{1000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
	}

	if args.rpfo {
		confgen.Dorpfo(args.ctx, t, true)

		if *ciscoFlags.GRIBITrafficCheck {
			args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121})
		}
	}

	//Delete  twice
	for s := 0; s < 4; s++ {

		args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

		args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

	}
}

func TestWithScalerpfo(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program scale gribi entries ~17K NH, 500 NHG/NH decapencap, 1K NHG, 1K default prefixes, 60K vrf prefixes verify traffic and delete")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	var top *ondatra.ATETopology
	ctx := context.Background()

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
	args := &testArgs{
		ctx:    ctx,
		client: &client,
		dut:    dut,
		ate:    ate,
		top:    top,
		rpfo:   true,
	}

	args.client.BecomeLeader(t)
	args.client.FlushServer(t)
	time.Sleep(10 * time.Second)
	//dut := ondatra.DUT(t, "dut")

	dstPfxx := "198.51.100.1"

	configbasePBR(t, dut, "TE2", "ipv4", 2, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{}, "PBR2", bundleEther121)
	configbasePBR(t, dut, "TE3", "ipv4", 3, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{}, "PBR3", bundleEther122)
	defer unconfigbasePBR(t, dut, "PBR2", bundleEther121)
	defer unconfigbasePBR(t, dut, "PBR3", bundleEther122)
	var nh1, nh2 uint64 = 1, 33
	var i, j uint64
	for i = 1; i <= 32; i++ {
		args.client.AddNH(t, nh1, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther123, false, ciscoFlags.GRIBIChecks)
		args.client.AddNH(t, nh2, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther124, false, ciscoFlags.GRIBIChecks)
		nh1 = nh1 + 1
		nh2 = nh2 + 1
	}
	nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID(uint64(1))

	for j = 1; j < 64; j++ {
		nhg.AddNextHop(j, uint64(64))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
	}

	var nh uint64 = 0
	NHEntry := fluent.NextHopEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance)

	for i = 2; i <= 499; i++ {
		nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID((i))
		for j = 0; j < 34; j++ {
			NHEntry = NHEntry.WithIPAddress(atePort4.IPv4).WithIndex(uint64(200 + nh))
			args.client.Fluent(t).Modify().AddEntry(t, NHEntry)
			nhg.AddNextHop(uint64(200+nh), uint64(10))
			args.client.Fluent(t).Modify().AddEntry(t, nhg)
			nh++
		}
	}

	dstPfx3 := "198.101.1.1"
	prefixes := []string{}
	for s := 0; s < 499; s++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx3, s, mask))
	}

	for _, prefix := range prefixes {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance("DEFAULT").
			WithPrefix(prefix).
			WithNextHopGroup(uint64(1)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
	}

	var prefix string
	k, l := 1, 1
	for j = 1; j < 31; j++ {
		count := 0
		for i = 1; i < 499; i++ {
			prefix = util.GetIPPrefix(dstPfxx, k, "32")
			ipv4Entry := fluent.IPv4Entry().
				WithNetworkInstance("TE").
				WithPrefix(prefix).
				WithNextHopGroup(uint64(2 + count)).
				WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)

			args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
			ipv4Entry1 := fluent.IPv4Entry().
				WithNetworkInstance("TE2").
				WithPrefix(prefix).
				WithNextHopGroup(uint64(2 + count)).
				WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)

			args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry1)
			ipv4Entry2 := fluent.IPv4Entry().
				WithNetworkInstance("TE3").
				WithPrefix(prefix).
				WithNextHopGroup(uint64(2 + count)).
				WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
			args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry2)
			ipv4Entry3 := fluent.IPv4Entry().
				WithNetworkInstance("DEFAULT").
				WithPrefix(prefix).
				WithNextHopGroup(uint64(2 + count)).
				WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
			args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry3)
			count++
			k++

		}
		b := strings.Split(dstPfx, ".")
		Var1, _ := strconv.Atoi(b[1])
		Var2, _ := strconv.Atoi(b[2])
		Var3, _ := strconv.Atoi(b[3])
		Var0, _ := strconv.Atoi(b[0])
		Var1 = Var1 + 1
		var1 := byte(Var1)
		var0 := byte(Var0)
		var3 := byte(Var3)
		var2 := byte(Var2)
		dst := net.IPv4(var0, var1, var2, var3)
		dstPfxx = dst.String()
		l++
	}
	NHEntry = fluent.NextHopEntry()

	for i = 20000; i < 20499; i++ {
		nhg := fluent.NextHopGroupEntry().WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithID((i))
		NHEntry = NHEntry.WithNetworkInstance(*ciscoFlags.DefaultNetworkInstance).WithIndex(i)
		NHEntry = NHEntry.WithDecapsulateHeader(fluent.IPinIP)
		NHEntry = NHEntry.WithEncapsulateHeader(fluent.IPinIP)
		NHEntry = NHEntry.WithIPinIP("222.222.222.222", dsip)
		args.client.Fluent(t).Modify().AddEntry(t, NHEntry)
		nhg.AddNextHop(i, uint64(10))
		args.client.Fluent(t).Modify().AddEntry(t, nhg)
	}
	prefixess := []string{}
	dstPfx2 := "198.100.1.1"
	for i := 0; i < 499; i++ {
		prefixess = append(prefixess, util.GetIPPrefix(dstPfx2, i, mask))
	}
	count := 0
	for _, prefix := range prefixess {
		ipv4Entry := fluent.IPv4Entry().
			WithNetworkInstance(*ciscoFlags.NonDefaultNetworkInstance).
			WithPrefix(prefix).
			WithNextHopGroup(uint64(20000 + count)).
			WithNextHopGroupNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
		args.client.Fluent(t).Modify().AddEntry(t, ipv4Entry)
		count++

	}

	args.client.AddNH(t, 30000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther125, false, ciscoFlags.GRIBIChecks)

	args.client.AddNHG(t, 30000, 0, map[uint64]uint64{30000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, dip, 30000, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort7.Name, SrcIP: atePort7.IPv4, DstIP: dstPfx3, Scalenum: 255}), false, []string{bundleEther123, bundleEther124}, &TGNoptions{Ifname: bundleEther126})
		args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort7.Name, SrcIP: atePort7.IPv4, DstIP: "198.101.2.1", Scalenum: 243}), false, []string{bundleEther123, bundleEther124}, &TGNoptions{Ifname: bundleEther126})
		args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort7.Name, SrcIP: atePort7.IPv4, DstIP: "198.51.100.2", Scalenum: 243}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther126})
		args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort7.Name, SrcIP: atePort7.IPv4, DstIP: "198.52.101.244", Scalenum: 14442}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther126})

		args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort1.Name, SrcIP: atePort1.IPv4, DstIP: "198.51.100.2", Scalenum: 243}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther120})
		args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort1.Name, SrcIP: atePort1.IPv4, DstIP: "198.52.101.244", Scalenum: 14442}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther120})
		args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort1.Name, SrcIP: atePort1.IPv4, DstIP: dstPfx2, Scalenum: 255}), false, []string{bundleEther125}, &TGNoptions{Ifname: bundleEther120})
		args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort1.Name, SrcIP: atePort1.IPv4, DstIP: "198.100.2.1", Scalenum: 243}), false, []string{bundleEther125}, &TGNoptions{Ifname: bundleEther120})

		args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort2.Name, SrcIP: atePort2.IPv4, DstIP: "198.51.100.2", Scalenum: 243}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther121})
		args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort2.Name, SrcIP: atePort2.IPv4, DstIP: "198.52.101.244", Scalenum: 14442}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther121})

		args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort3.Name, SrcIP: atePort3.IPv4, DstIP: "198.51.100.2", Scalenum: 243}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther122})
		args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort3.Name, SrcIP: atePort3.IPv4, DstIP: "198.52.101.244", Scalenum: 14442}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther122})

	}

	if args.rpfo {
		confgen.Dorpfo(args.ctx, t, true)

		if *ciscoFlags.GRIBITrafficCheck {
			args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort7.Name, SrcIP: atePort7.IPv4, DstIP: dstPfx3, Scalenum: 255}), false, []string{bundleEther123, bundleEther124}, &TGNoptions{Ifname: bundleEther126})
			args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort7.Name, SrcIP: atePort7.IPv4, DstIP: "198.101.2.1", Scalenum: 243}), false, []string{bundleEther123, bundleEther124}, &TGNoptions{Ifname: bundleEther126})
			args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort7.Name, SrcIP: atePort7.IPv4, DstIP: "198.51.100.2", Scalenum: 243}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther126})
			args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort7.Name, SrcIP: atePort7.IPv4, DstIP: "198.52.101.244", Scalenum: 14442}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther126})

			args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort1.Name, SrcIP: atePort1.IPv4, DstIP: "198.51.100.2", Scalenum: 243}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther120})
			args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort1.Name, SrcIP: atePort1.IPv4, DstIP: "198.52.101.244", Scalenum: 14442}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther120})
			args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort1.Name, SrcIP: atePort1.IPv4, DstIP: dstPfx2, Scalenum: 255}), false, []string{bundleEther125}, &TGNoptions{Ifname: bundleEther120})
			args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort1.Name, SrcIP: atePort1.IPv4, DstIP: "198.100.2.1", Scalenum: 243}), false, []string{bundleEther125}, &TGNoptions{Ifname: bundleEther120})

			args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort2.Name, SrcIP: atePort2.IPv4, DstIP: "198.51.100.2", Scalenum: 243}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther121})
			args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort2.Name, SrcIP: atePort2.IPv4, DstIP: "198.52.101.244", Scalenum: 14442}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther121})

			args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort3.Name, SrcIP: atePort3.IPv4, DstIP: "198.51.100.2", Scalenum: 243}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther122})
			args.validateTrafficFlows(t, args.allFlows(t, &TGNoptions{SrcIf: atePort3.Name, SrcIP: atePort3.IPv4, DstIP: "198.52.101.244", Scalenum: 14442}), false, []string{bundleEther123}, &TGNoptions{Ifname: bundleEther122})

		}
	}

	prefixes1 := []string{}

	for i := 0; i < 1000; i++ {
		prefixes1 = append(prefixes1, util.GetIPPrefix("198.51.100.0", i, mask))
	}

	prefixes3 := []string{}

	for i := 0; i < 15000; i++ {
		prefixes3 = append(prefixes3, util.GetIPPrefix("198.52.101.0", i, mask))
	}

	for s := 0; s < 4; s++ {
		for _, prefix := range prefixes1 {
			ipv4Entry := fluent.IPv4Entry().WithPrefix(prefix).WithNetworkInstance("DEFAULT")
			args.client.Fluent(t).Modify().DeleteEntry(t, ipv4Entry)
			ipv4Entry1 := fluent.IPv4Entry().WithPrefix(prefix).WithNetworkInstance("TE")
			args.client.Fluent(t).Modify().DeleteEntry(t, ipv4Entry1)
			ipv4Entry2 := fluent.IPv4Entry().WithPrefix(prefix).WithNetworkInstance("TE2")
			args.client.Fluent(t).Modify().DeleteEntry(t, ipv4Entry2)
			ipv4Entry3 := fluent.IPv4Entry().WithPrefix(prefix).WithNetworkInstance("TE3")
			args.client.Fluent(t).Modify().DeleteEntry(t, ipv4Entry3)
		}

		for _, prefix := range prefixes3 {
			ipv4Entry := fluent.IPv4Entry().WithPrefix(prefix).WithNetworkInstance("DEFAULT")
			args.client.Fluent(t).Modify().DeleteEntry(t, ipv4Entry)
			ipv4Entry1 := fluent.IPv4Entry().WithPrefix(prefix).WithNetworkInstance("TE")
			args.client.Fluent(t).Modify().DeleteEntry(t, ipv4Entry1)
			ipv4Entry2 := fluent.IPv4Entry().WithPrefix(prefix).WithNetworkInstance("TE2")
			args.client.Fluent(t).Modify().DeleteEntry(t, ipv4Entry2)
			ipv4Entry3 := fluent.IPv4Entry().WithPrefix(prefix).WithNetworkInstance("TE3")
			args.client.Fluent(t).Modify().DeleteEntry(t, ipv4Entry3)
		}

		for _, prefix := range prefixess {
			ipv4Entry1 := fluent.IPv4Entry().WithPrefix(prefix).WithNetworkInstance("TE")
			args.client.Fluent(t).Modify().DeleteEntry(t, ipv4Entry1)
		}
		for _, prefix := range prefixes {
			ipv4Entry1 := fluent.IPv4Entry().WithPrefix(prefix).WithNetworkInstance("DEFAULT")
			args.client.Fluent(t).Modify().DeleteEntry(t, ipv4Entry1)
		}

		args.client.DeleteIPv4(t, dip, 30000, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

		for j = 1; j < 40000; j++ {
			nhg := fluent.NextHopGroupEntry().WithNetworkInstance("DEFAULT").WithID(j)
			args.client.Fluent(t).Modify().DeleteEntry(t, nhg)
		}

		for j = 1; j < 40000; j++ {
			NH := fluent.NextHopEntry().
				WithNetworkInstance("DEFAULT").
				WithIndex(j)
			args.client.Fluent(t).Modify().DeleteEntry(t, NH)
		}
	}
}

func TestWithStaticrpfo(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Program static route entries, and then through gribi, verify traffic, delete gribi entries")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	var top *ondatra.ATETopology
	ctx := context.Background()

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
	args := &testArgs{
		ctx:    ctx,
		client: &client,
		dut:    dut,
		ate:    ate,
		top:    top,
		rpfo:   true,
	}

	args.client.BecomeLeader(t)
	args.client.FlushServer(t)
	time.Sleep(10 * time.Second)

	// config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast vip1 Bundle-Ether126 192.0.2.26")
	// config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 192.0.2.42/32 Bundle-Ether126 192.0.2.26")

	addStaticRoute(t, dut, vip1, true)
	addStaticRoute(t, dut, vip2, true)

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther122, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 60, 1100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther123, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, vip2, 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 1, vip1ip, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2, vip2ip, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2, 0, map[uint64]uint64{2: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1, 2, map[uint64]uint64{1: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}

	args.client.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther126})
	}

	if args.rpfo {
		confgen.Dorpfo(args.ctx, t, true)

		if *ciscoFlags.GRIBITrafficCheck {
			args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther126})
		}
	}

	for s := 0; s < 4; s++ {

		args.client.DeleteIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip1, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip2, 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1, 2, map[uint64]uint64{1: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2, 0, map[uint64]uint64{2: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1, vip1, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2, vip2, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

		args.client.DeleteIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip1, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip2, 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 1, 2, map[uint64]uint64{1: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2, 0, map[uint64]uint64{2: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1, vip1, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2, vip2, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther122, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther123, false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther122, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther123, false, ciscoFlags.GRIBIChecks)
	}
}

func TestWithStaticremoverpfo(t *testing.T) {

	// Elect client as leader and flush all the past entries
	t.Logf("Remove static routes, program gribi verify traffic, and delete gribi entries")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	var top *ondatra.ATETopology
	ctx := context.Background()

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
	args := &testArgs{
		ctx:    ctx,
		client: &client,
		dut:    dut,
		ate:    ate,
		top:    top,
		rpfo:   true,
	}
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)
	// vip1ip := "192.0.2.40"
	// vip2ip := "192.0.2.42"

	// config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast vip1 Bundle-Ether126 192.0.2.26")
	// config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast vip2 Bundle-Ether126 192.0.2.26")
	addStaticRoute(t, dut, vip1, false)
	addStaticRoute(t, dut, vip2, false)

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther122, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 60, 1100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, vip1, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther123, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, vip2, 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 1, vip1ip, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2, vip2ip, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2, 0, map[uint64]uint64{2: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1, 2, map[uint64]uint64{1: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121, bundleEther122})
	}

	if args.rpfo {
		confgen.Dorpfo(args.ctx, t, true)

		if *ciscoFlags.GRIBITrafficCheck {
			args.validateTrafficFlows(t, args.allFlows(t), false, []string{bundleEther121, bundleEther122})
		}
	}
	time.Sleep(20 * time.Minute)

	for s := 0; s < 4; s++ {

		args.client.DeleteIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip1, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip2, 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1, 2, map[uint64]uint64{1: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2, 0, map[uint64]uint64{2: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1, vip1, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2, vip2, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

		args.client.DeleteIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip1, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteIPv4(t, vip2, 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 1, 2, map[uint64]uint64{1: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2, 0, map[uint64]uint64{2: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1, vip1, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2, vip2, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther122, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther123, false, ciscoFlags.GRIBIChecks)

		args.client.DeleteNHG(t, 1000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther121, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther122, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
		args.client.DeleteNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", bundleEther123, false, ciscoFlags.GRIBIChecks)
	}
}
