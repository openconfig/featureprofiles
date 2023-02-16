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
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/cisco/config"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/gribi"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gnps "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	spb "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
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
	dstPfx1               = "11.1.1.1"
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
	lc                    = "0/0/CPU0"
)

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx    context.Context
	client *gribi.Client
	dut    *ondatra.DUTDevice
	ate    *ondatra.ATEDevice
	top    *ondatra.ATETopology
}

func testDeleteIpv4NHGNH(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	unconfigbasePBR(t, dut)
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 10, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 100, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether121"})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}

	args.client.DeleteIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 100, 101, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 10, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 100, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), true, []string{"Bundle-Ether121", "Bundle-Ether127"})
	}
	
	//Non-existing
	args.client.DeleteIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 100, 101, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 10, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 100, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)

	//Random non-existing
	args.client.DeleteIPv4(t, "203.0.2.1/32", 111, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 111, 121, map[uint64]uint64{111: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 111, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 121, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)

	args.client.DeleteIPv4(t, "203.0.2.1/32", 111, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 111, 121, map[uint64]uint64{111: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 111, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 121, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)

}

func testDeleteIpv4(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 10, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 100, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether121"})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}

	args.client.DeleteIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), true, []string{"Bundle-Ether121", "Bundle-Ether127"})
	}

	args.client.DeleteIPv4(t, "203.1.2.1/32", 121, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, "203.1.2.1/32", 121, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.DeleteIPv4(t, "203.1.2.1/32", 121, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, "203.1.2.1/32", 121, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

}

func testDeleteNHG(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 10, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 100, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether121"})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}
	
	args.client.DeleteNHG(t, 100, 101, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), true, []string{"Bundle-Ether121", "Bundle-Ether127"})
	}
	
	//Non-existing
	args.client.DeleteNHG(t, 100, 101, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	
    //Random non-existing
	args.client.DeleteNHG(t, 111, 121, map[uint64]uint64{111: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 111, 121, map[uint64]uint64{111: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
}

func testDeleteNH(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 10, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 100, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether121"})
	}
	//aft check
	if *ciscoFlags.GRIBIAFTChainCheck {
		randomItems := args.client.RandomEntries(t, *ciscoFlags.GRIBIConfidence, prefixes)
		for i := 0; i < len(randomItems); i++ {
			args.client.CheckAftIPv4(t, "TE", randomItems[i])
		}
	}

	args.client.DeleteNH(t, 10, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 100, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 10, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 100, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), true, []string{"Bundle-Ether121", "Bundle-Ether127"})
	}
	
	//Random non-existing
	
	args.client.DeleteNH(t, 111, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 121, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 111, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 121, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	defer configbasePBR(t, dut, "TE", "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})

}

func testwithBackup(ctx context.Context, t *testing.T, args *testArgs) {

	// reference case details
	// https://cisco.sharepoint.com/:p:/r/Sites/Spitfire-Test/_layouts/15/Doc.aspx?sourcedoc=%7BBBBAE62E-F6C2-41B2-A3F6-305268149284%7D&file=VRF_Fallback_scenario.pptx&action=edit&mobileredirect=true

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	// LEVEL 1
	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 60, 1100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 1, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1, 2, map[uint64]uint64{1: 85, 2: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether121", "Bundle-Ether122"})
	}

	//Delete  twice
	args.client.DeleteIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 1, 2, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 1, "192.0.2.40/32", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 2, "192.0.2.42/32", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

	args.client.DeleteIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 1, 2, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 1, "192.0.2.40/32", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 2, "192.0.2.42/32", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	
	args.client.DeleteNHG(t, 1000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)

	args.client.DeleteNHG(t, 1000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)

}

func testwithBackupDelete(ctx context.Context, t *testing.T, args *testArgs) {

	// reference case details
	// https://cisco.sharepoint.com/:p:/r/Sites/Spitfire-Test/_layouts/15/Doc.aspx?sourcedoc=%7BBBBAE62E-F6C2-41B2-A3F6-305268149284%7D&file=VRF_Fallback_scenario.pptx&action=edit&mobileredirect=true

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	// LEVEL 1
	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 60, 1100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 1, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1, 2, map[uint64]uint64{1: 85, 2: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether121", "Bundle-Ether122"})
	}

	//Delete  twice
	args.client.DeleteIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4Batch(t, prefixes, 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	//Reprogram
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	
	//Delete NHG, NH	
	
	args.client.DeleteNHG(t, 1, 2, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 1, "192.0.2.40/32", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 2, "192.0.2.42/32", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 1, 2, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 1, "192.0.2.40/32", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 2, "192.0.2.42/32", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	
	args.client.DeleteNHG(t, 1000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)

	args.client.DeleteNHG(t, 1000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 2000, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)

}

func testwithDecapEncap(ctx context.Context, t *testing.T, args *testArgs) {

	// reference case details
	// https://cisco.sharepoint.com/:p:/r/Sites/Spitfire-Test/_layouts/15/Doc.aspx?sourcedoc=%7BBBBAE62E-F6C2-41B2-A3F6-305268149284%7D&file=VRF_Fallback_scenario.pptx&action=edit&mobileredirect=true

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	args.client.AddNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 3000, 0, map[uint64]uint64{3000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "10.1.0.1/32", 3000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks,  &gribi.NHOptions{Src: "222.222.222.222", Dest: []string{"10.1.0.1"}})
	args.client.AddNHG(t, 1000, 2000, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether121"})
	}

	//Delete  twice
	args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, "10.1.0.1/32", 3000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 3000, 0, map[uint64]uint64{3000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)

	
	args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, "10.1.0.1/32", 3000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 3000, 0, map[uint64]uint64{3000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	
}

func testwithDecapEncapDelete(ctx context.Context, t *testing.T, args *testArgs) {

	// reference case details
	// https://cisco.sharepoint.com/:p:/r/Sites/Spitfire-Test/_layouts/15/Doc.aspx?sourcedoc=%7BBBBAE62E-F6C2-41B2-A3F6-305268149284%7D&file=VRF_Fallback_scenario.pptx&action=edit&mobileredirect=true

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	args.client.AddNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 3000, 0, map[uint64]uint64{3000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "10.1.0.1/32", 3000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks,  &gribi.NHOptions{Src: "222.222.222.222", Dest: []string{"10.1.0.1"}})
	args.client.AddNHG(t, 1000, 2000, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether121"})
	}

	//Delete  twice
	args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, "10.1.0.1/32", 3000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, "10.1.0.1/32", 3000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	
	//Reprogram

	args.client.AddIPv4(t, "10.1.0.1/32", 3000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)


	args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 3000, 0, map[uint64]uint64{3000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)

	args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 3000, 0, map[uint64]uint64{3000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	
}

func testwithDecapEncapvrf(ctx context.Context, t *testing.T, args *testArgs) {

	// reference case details
	// https://cisco.sharepoint.com/:p:/r/Sites/Spitfire-Test/_layouts/15/Doc.aspx?sourcedoc=%7BBBBAE62E-F6C2-41B2-A3F6-305268149284%7D&file=VRF_Fallback_scenario.pptx&action=edit&mobileredirect=true

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	args.client.AddNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 3000, 0, map[uint64]uint64{3000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "10.1.0.1/32", 3000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks,  &gribi.NHOptions{Src: "222.222.222.222", Dest: []string{"10.1.0.1"}})
	args.client.AddNHG(t, 1000, 2000, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether121"})
	}

	//Delete  twice
	args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, "10.1.0.1/32", 3000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 3000, 0, map[uint64]uint64{3000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)

	
	args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, "10.1.0.1/32", 3000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 3000, 0, map[uint64]uint64{3000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	
}

func testwithDecapEncapvrfDelete(ctx context.Context, t *testing.T, args *testArgs) {

	// reference case details
	// https://cisco.sharepoint.com/:p:/r/Sites/Spitfire-Test/_layouts/15/Doc.aspx?sourcedoc=%7BBBBAE62E-F6C2-41B2-A3F6-305268149284%7D&file=VRF_Fallback_scenario.pptx&action=edit&mobileredirect=true

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	args.client.AddNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 3000, 0, map[uint64]uint64{3000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "10.1.0.1/32", 3000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks,  &gribi.NHOptions{Src: "222.222.222.222", Dest: []string{"10.1.0.1"}})
	args.client.AddNHG(t, 1000, 2000, map[uint64]uint64{2: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether121"})
	}

	//Delete  twice
	args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, "10.1.0.1/32", 3000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4(t, "10.1.0.1/32", 3000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	
	//Reprogram
	args.client.AddIPv4(t, "10.1.0.1/32", 3000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)


	args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 3000, 0, map[uint64]uint64{3000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	
	
	args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 1000, "DecapEncap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 3000, 0, map[uint64]uint64{3000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 3000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	
}

func testwithBackupDecap(ctx context.Context, t *testing.T, args *testArgs) {

	// reference case details
	// https://cisco.sharepoint.com/:p:/r/Sites/Spitfire-Test/_layouts/15/Doc.aspx?sourcedoc=%7BBBBAE62E-F6C2-41B2-A3F6-305268149284%7D&file=VRF_Fallback_scenario.pptx&action=edit&mobileredirect=true

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 2000, map[uint64]uint64{1000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether121"})
	}

	//Delete  twice
	args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

	args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

}

func testwithBackupDecapDelete(ctx context.Context, t *testing.T, args *testArgs) {

	// reference case details
	// https://cisco.sharepoint.com/:p:/r/Sites/Spitfire-Test/_layouts/15/Doc.aspx?sourcedoc=%7BBBBAE62E-F6C2-41B2-A3F6-305268149284%7D&file=VRF_Fallback_scenario.pptx&action=edit&mobileredirect=true

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 2000, map[uint64]uint64{1000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	if *ciscoFlags.GRIBITrafficCheck {
		args.validateTrafficFlows(t, args.allFlows(), false, []string{"Bundle-Ether121"})
	}

	//Delete  twice
	args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	
	//Reprogram
	args.client.AddIPv4Batch(t, prefixes, 1000, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

	args.client.DeleteNHG(t, 1000, 2000, map[uint64]uint64{1000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNHG(t, 2000, 0, map[uint64]uint64{2000: 10}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.DeleteNH(t, 2000, "decap", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

}

func TestDoubleDelete(t *testing.T) {
	t.Log("Name: DoubleDelete")
	t.Log("Description: Connect gRIBI client and B to DUT using SINGLE_PRIMARY client redundancy with persistance and RibACK")

	dut := ondatra.DUT(t, "dut")

	// Dial gRIBI
	ctx := context.Background()

	// Configure the DUT
	configureDUT(t, dut)
	configbasePBR(t, dut, "TE", "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})
	defer unconfigbasePBR(t, dut)
	// configure route-policy
	configRP(t, dut)
	// configure ISIS on DUT
	addISISOC(t, dut, "Bundle-Ether127")
	// configure BGP on DUT
	addBGPOC(t, dut, "100.100.100.100")

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	if *ciscoFlags.GRIBITrafficCheck {
		addPrototoAte(t, top)
	}

	// Connfigure vty-pool
	config.TextWithGNMI(context.Background(), t, dut, "vty-pool default 0 99 line-template default")
	test := []struct {
		name string
		desc string
		fn   func(ctx context.Context, t *testing.T, args *testArgs)
	}{
		{
			name: "Double delete test1",
			desc: "Delete ipv4, nhg, nh",
			fn:   testDeleteIpv4NHGNH,
		},
		{
			name: "Double delete test2",
			desc: "Delete ipv4, nhg, nh",
			fn:   testDeleteIpv4,
		},
		{
			name: "Double delete test3",
			desc: "Delete ipv4, nhg, nh",
			fn:   testDeleteNHG,
		},
		{
			name: "Double delete test4",
			desc: "Delete ipv4, nhg, nh",
			fn:   testDeleteNH,
		},
		{
			name: "Double delete test5",
			desc: "Delete ipv4, nhg, nh",
			fn:   testwithBackup,
		},
		{
			name: "Double delete test6",
			desc: "Delete ipv4, nhg, nh",
			fn:   testwithBackupDelete,
		},
		{
			name: "Double delete test6",
			desc: "Delete ipv4, nhg, nh",
			fn:   testwithDecapEncap,
		},
		{
			name: "Double delete test6",
			desc: "Delete ipv4, nhg, nh",
			fn:   testwithDecapEncapDelete,
		},
		{
			name: "Double delete test6",
			desc: "Delete ipv4, nhg, nh",
			fn:   testwithDecapEncapvrf,
		},
		{
			name: "Double delete test6",
			desc: "Delete ipv4, nhg, nh",
			fn:   testwithDecapEncapvrfDelete,
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
