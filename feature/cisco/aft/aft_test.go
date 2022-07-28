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

package aft_test

import (
	"context"
	"testing"
	"time"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/gribi"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"

	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ipv4PrefixLen         = 30
	ipv6PrefixLen         = 126
	instance              = "default"
	dstPfx                = "198.51.100.1"
	mask                  = "32"
	dstPfxMin             = "198.51.100.1"
	dstPfxCount           = 100
	dstPfx1               = "11.1.1.1"
	dstPfxCount1          = 10
	innersrcPfx           = "200.1.0.1"
	innerdstPfxMin_bgp    = "202.1.0.1"
	innerdstPfxCount_bgp  = 100
	innerdstPfxMin_isis   = "201.1.0.1"
	innerdstPfxCount_isis = 100
)

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx    context.Context
	client *gribi.Client
	dut    *ondatra.DUTDevice
	ate    *ondatra.ATEDevice
	top    *ondatra.ATETopology
}

const (
	timeout = time.Minute
)

// Client provides access to GRIBI APIs of the DUT.
//
// Usage:
//
//   c := &Client{
//     DUT: ondatra.DUT(t, "dut"),
//     FibACK: true,
//     Persistence: true,
//   }
//   defer c.Close(t)
//   if err := c.Start(t); err != nil {
//     t.Fatalf("Could not initialize gRIBI: %v", err)
//   }
type Client struct {
	DUT                   *ondatra.DUTDevice
	FibACK                bool
	Persistence           bool
	InitialElectionIDLow  uint64
	InitialElectionIDHigh uint64

	// Unexport fields below.
	fluentC *fluent.GRIBIClient
}

// Fluent resturns the fluent client that can be used to directly call the gribi fluent APIs
func (c *Client) Fluent(t testing.TB) *fluent.GRIBIClient {
	return c.fluentC
}

func aftCheck(ctx context.Context, t *testing.T, args *testArgs) {

	ipv4prefix := "192.0.2.40/32"
	nhlist, nexthopgroup := getaftnh(t, args.dut, ipv4prefix, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance)
	t.Logf("First NH in list %d", nhlist[0])
	nexthop := nhlist[0]

	ipv4prefix_nondefault := "198.51.100.1/32"
	nhlist_nondefault, nexthopgroup_nondefault := getaftnh(t, args.dut, ipv4prefix_nondefault, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance)
	t.Logf("First NH in list %d", nhlist_nondefault[0])

	// Telemerty check
	t.Run("Telemetry on AFT TOP Container", func(t *testing.T) {
		args.dut.Telemetry().NetworkInstance(instance).Afts().Get(t)
	})
	t.Run("Telemetry on Ipv4Entry", func(t *testing.T) {
		args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ipv4prefix).Get(t)
	})
	t.Run("Telemetry on Ipv4Entry NextHopGroup", func(t *testing.T) {
		path := args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ipv4prefix).NextHopGroup()
		nhgvalue := path.Get(t)
		if nhgvalue != nexthopgroup {
			t.Errorf("Incorrect value for NextHopGroup , got:%v,want:%v", nhgvalue, nexthopgroup)
		}
	})
	t.Run("Telemetry on Ipv4Entry Prefix", func(t *testing.T) {
		path := args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ipv4prefix).Prefix()
		prefixvalue := path.Get(t)
		if prefixvalue != ipv4prefix {
			t.Errorf("Incorrect value for AFT Ipv4Entry Prefix got %s, want %s", prefixvalue, ipv4prefix)
		}
	})

	// t.Run("Telemetry on Ipv4Entry Prefix", func(t *testing.T) {
	// 	path := args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ipv4prefix).EntryMetadata()
	// 	path.Get(t)
	// })
	t.Run("Telemetry on Ipv4Entry NextHopGroupNetworkInstance", func(t *testing.T) {
		path := args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ipv4prefix).NextHopGroupNetworkInstance()
		path.Get(t)
	})
	// t.Run("Telemetry on Ipv4Entry DecapsulateHeader", func(t *testing.T) {
	// 	args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ipv4prefix).DecapsulateHeader().Get(t)
	// })
	// t.Run("Telemetry on Ipv4Entry OctetsForwarded", func(t *testing.T) {
	// 	args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ipv4prefix).Counters().OctetsForwarded().Get(t)
	// })
	// t.Run("Telemetry on Ipv4Entry PacketsForwarded", func(t *testing.T) {
	// 	args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ipv4prefix).Counters().PacketsForwarded().Get(t)
	// })
	// t.Run("Telemetry on Ipv4Entry OriginProtocol", func(t *testing.T) {
	// 	args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ipv4prefix).OriginProtocol().Get(t)
	// })
	// t.Run("Telemetry on Ipv4Entry OriginNetworkInstance", func(t *testing.T) {
	// 	args.dut.Telemetry().NetworkInstance(instance).Afts().Ipv4Entry(ipv4prefix).OriginNetworkInstance().Get(t)
	// })

	// /network-instances/network-instance/afts/next-hop-groups/next-hop-group/id
	// /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops
	// /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop
	// /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/index
	// /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state
	// /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/index
	// /network-instances/network-instance/afts/next-hop-groups/next-hop-group/next-hops/next-hop/state/weight
	// /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state
	// /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/backup-next-hop-group
	// /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/color
	// /network-instances/network-instance/afts/next-hop-groups/next-hop-group/state/id
	t.Run("Telemetry on NextHopGroup", func(t *testing.T) {
		aftNHG := args.dut.Telemetry().NetworkInstance(instance).Afts().NextHopGroup(nexthopgroup).Get(t)
		if got := len(aftNHG.NextHop); got != 4 {
			t.Fatalf("Prefix %s next-hop entry count: got %d, want 4", dstPfx, got)
		}
	})
	t.Run("Telemetry on NextHopGroup Id", func(t *testing.T) {
		path := args.dut.Telemetry().NetworkInstance(instance).Afts().NextHopGroup(nexthopgroup).Id()
		value := path.Get(t)
		t.Logf("Value %d", value)
	})
	t.Run("Telemetry on NextHopGroup NextHopAny", func(t *testing.T) {
		path := args.dut.Telemetry().NetworkInstance(instance).Afts().NextHopGroup(nexthopgroup).NextHopAny()
		path.Get(t)
	})

	t.Run("Telemetry on NextHopGroup NextHop", func(t *testing.T) {
		path := args.dut.Telemetry().NetworkInstance(instance).Afts().NextHopGroup(nexthopgroup).NextHop(nexthop)
		value := path.Get(t)
		t.Logf("Value %d", value)
	})
	t.Run("Telemetry on NextHopGroup NextHop Index", func(t *testing.T) {
		path := args.dut.Telemetry().NetworkInstance(instance).Afts().NextHopGroup(nexthopgroup).NextHop(nexthop).Index()
		value := path.Get(t)
		t.Logf("Value %d", value)
	})
	t.Run("Telemetry on NextHopGroup NextHop Weight", func(t *testing.T) {
		path := args.dut.Telemetry().NetworkInstance(instance).Afts().NextHopGroup(nexthopgroup).NextHop(nexthop).Weight()
		value := path.Get(t)
		t.Logf("Value %d", value)
	})
	t.Run("Telemetry on NextHopGroup BackupNextHopGroup", func(t *testing.T) {
		path := args.dut.Telemetry().NetworkInstance(instance).Afts().NextHopGroup(nexthopgroup_nondefault).BackupNextHopGroup()
		value := path.Get(t)
		t.Logf("Value %d", value)
		nhg := args.dut.Telemetry().NetworkInstance(instance).Afts().NextHopGroup(value).Get(t)
		t.Logf("BackupNextHopGroup ProgrammedId VALUE: ..............................: %d", nhg.GetProgrammedId())
	})
	// t.Run("Telemetry on NextHopGroup Color", func(t *testing.T) {
	// 	args.dut.Telemetry().NetworkInstance(instance).Afts().NextHopGroup(nexthopgroup).Color().Get(t)
	// })

	// /network-instances/network-instance/afts/next-hops/next-hop/index
	// /network-instances/network-instance/afts/next-hops/next-hop/interface-ref
	// /network-instances/network-instance/afts/next-hops/next-hop/interface-ref/state
	// /network-instances/network-instance/afts/next-hops/next-hop/interface-ref/state/interface
	// /network-instances/network-instance/afts/next-hops/next-hop/interface-ref/state/subinterface
	// /network-instances/network-instance/afts/next-hops/next-hop/state
	// /network-instances/network-instance/afts/next-hops/next-hop/state/encapsulate-header
	// /network-instances/network-instance/afts/next-hops/next-hop/state/index
	// /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address
	// /network-instances/network-instance/afts/next-hops/next-hop/state/mac-address
	// /network-instances/network-instance/afts/next-hops/next-hop/state/origin-protocol
	// /network-instances/network-instance/afts/next-hops/next-hop/state/pushed-mpls-label-stack
	t.Run("Telemetry on NextHop", func(t *testing.T) {
		path := args.dut.Telemetry().NetworkInstance(instance).Afts().NextHop(nexthop)
		value := path.Get(t)
		t.Logf("Value %v", value)
	})
	t.Run("Telemetry on NextHop Index", func(t *testing.T) {
		path := args.dut.Telemetry().NetworkInstance(instance).Afts().NextHop(nexthop).Index()
		value := path.Get(t)
		t.Logf("Value %d", value)
	})

	ipv4prefix = "192.0.2.50/32"
	nhlist, nexthopgroup = getaftnh(t, args.dut, ipv4prefix, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance)
	t.Logf("First NH in list %d", nhlist[0])
	nexthop_interfaceref := nhlist[0]

	ipv4prefix = "192.0.2.51/32"
	nhlist, nexthopgroup = getaftnh(t, args.dut, ipv4prefix, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance)
	t.Logf("First NH in list %d", nhlist[0])
	nexthop_ipinip := nhlist[0]

	t.Run("Telemetry on NextHop InterfaceRef(main interface)", func(t *testing.T) {
		path := args.dut.Telemetry().NetworkInstance(instance).Afts().NextHop(nexthop_interfaceref).InterfaceRef()
		value := path.Get(t)
		t.Logf("Value %d", value)
	})

	t.Run("Telemetry on NextHop InterfaceRef Interface", func(t *testing.T) {
		path := args.dut.Telemetry().NetworkInstance(instance).Afts().NextHop(nexthop_interfaceref).InterfaceRef().Interface()
		value := path.Get(t)
		t.Logf("Value %s", value)
	})

	ipv4prefix = "192.0.2.52/32"
	nhlist, nexthopgroup = getaftnh(t, args.dut, ipv4prefix, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance)
	t.Logf("First NH in list %d", nhlist[0])
	nexthop_subinterfaceref := nhlist[0]

	t.Run("Telemetry on NextHop InterfaceRef(subinterface)", func(t *testing.T) {
		path := args.dut.Telemetry().NetworkInstance(instance).Afts().NextHop(nexthop_subinterfaceref).InterfaceRef()
		value := path.Get(t)
		t.Logf("Value %d", value)
	})
	t.Run("Telemetry on NextHop InterfaceRef Subinterface", func(t *testing.T) {
		path := args.dut.Telemetry().NetworkInstance(instance).Afts().NextHop(nexthop_subinterfaceref).InterfaceRef().Subinterface()
		value := path.Get(t)
		t.Logf("Value %d", value)
	})
	t.Run("Telemetry on NextHop EncapsulateHeader", func(t *testing.T) {
		path := args.dut.Telemetry().NetworkInstance(instance).Afts().NextHop(nexthop).EncapsulateHeader()
		value := path.Get(t)
		t.Logf("Value %d", value)
	})
	t.Run("Telemetry on NextHop DecapsulateHeader", func(t *testing.T) {
		path := args.dut.Telemetry().NetworkInstance(instance).Afts().NextHop(nexthop).DecapsulateHeader()
		value := path.Get(t)
		t.Logf("Value %d", value)
	})
	// /network-instances/network-instance/afts/next-hops/next-hop/state/ip-address
	t.Run("Telemetry on NextHop IpAddress", func(t *testing.T) {
		path := args.dut.Telemetry().NetworkInstance(instance).Afts().NextHop(nexthop).IpAddress()
		value := path.Get(t)
		t.Logf("Value %s", value)
	})
	// t.Run("Telemetry on NextHop MacAddress", func(t *testing.T) {
	// 	args.dut.Telemetry().NetworkInstance(instance).Afts().NextHop(nexthop).MacAddress().Get(t)
	// })
	// t.Run("Telemetry on NextHop OriginProtocol", func(t *testing.T) {
	// 	args.dut.Telemetry().NetworkInstance(instance).Afts().NextHop(nexthop).OriginProtocol().Get(t)
	// })
	// t.Run("Telemetry on NextHop PushedMplsLabelStack", func(t *testing.T) {
	// 	args.dut.Telemetry().NetworkInstance(instance).Afts().NextHop(nexthop).PushedMplsLabelStack().Get(t)
	// })

	// /network-instances/network-instance[name]/afts/next-hops/next-hop[index]/state/programmed-index
	// /network-instances/network-instance[name]/afts/next-hop-groups/next-hop-group[id]/state/programmed-id
	// /network-instances/network-instance[name]/afts/next-hops/next-hop[index]/ip-in-ip/state/src-ip
	// /network-instances/network-instance[name]/afts/next-hops/next-hop[index]/ip-in-ip/state/dst-ip
	t.Run("Telemetry on NextHop ProgrammedIndex", func(t *testing.T) {
		path := args.dut.Telemetry().NetworkInstance(instance).Afts().NextHop(nexthop).ProgrammedIndex()
		value := path.Get(t)
		t.Logf("Value %d", value)
	})
	t.Run("Telemetry on NextHopGroup ProgrammedId", func(t *testing.T) {
		path := args.dut.Telemetry().NetworkInstance(instance).Afts().NextHopGroup(nexthopgroup).ProgrammedId()
		value := path.Get(t)
		t.Logf("Value %d", value)
	})
	t.Run("Telemetry on NextHop IpInIp", func(t *testing.T) {
		path := args.dut.Telemetry().NetworkInstance(instance).Afts().NextHop(nexthop_ipinip).IpInIp()
		value := path.Get(t)
		t.Logf("Value %d", value)
	})
	t.Run("Telemetry on NextHop IpInIp SrcIp", func(t *testing.T) {
		path := args.dut.Telemetry().NetworkInstance(instance).Afts().NextHop(nexthop_ipinip).IpInIp().SrcIp()
		value := path.Get(t)
		t.Logf("Value %s", value)
	})
	t.Run("Telemetry on NextHop IpInIp DstIp", func(t *testing.T) {
		path := args.dut.Telemetry().NetworkInstance(instance).Afts().NextHop(nexthop_ipinip).IpInIp().DstIp()
		value := path.Get(t)
		t.Logf("Value %s", value)
	})
}

// AGNEL AFT ADD
func testAFT(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect client as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.client.BecomeLeader(t)
	args.client.FlushServer(t)

	// LEVEL 2
	// Creating a backup NHG with ID 101 and NH ID 10 pointing to decap
	args.client.AddNH(t, 10, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// Creating NHG ID 100 using backup NHG ID 101
	// PATH 1 NH ID 100, weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200, weight 15, VIP2 : 192.0.2.42
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.AddNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 200, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4Batch(t, prefixes, 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	// LEVEL 1
	// VIP1: NHG ID 1000
	//		- PATH1 NH ID 1000, weight 50, outgoing Port2
	//		- PATH2 NH ID 1100, weight 30, outgoing Port3
	//		- PATH3 NH ID 1200, weight 15, outgoing Port4
	//		- PATH4 NH ID 1300, weight  5, outgoing Port5
	// VIP2: NHG ID 2000
	//		- PATH1 NH ID 2000, weight 60, outgoing Port6
	//		- PATH2 NH ID 2100, weight 35, outgoing Port7
	//		- PATH3 NH ID 2200, weight  5, outgoing Port8
	args.client.AddNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	args.client.AddNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// NH WithInterfaceRef
	args.client.AddNH(t, 5000, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "FourHundredGigE0/0/0/17", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 5000, 0, map[uint64]uint64{5000: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.50/32", 5000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// NH WithIPinIP
	//args.client.AddNHAGN(t, 5001, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHWithIPinIP(t, 5001, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", true, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 5001, 0, map[uint64]uint64{5001: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.51/32", 5001, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// NH WithSubinterfaceRef
	//args.client.AddNHSubIntf(t, 5002, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "Bundle-Ether1", false, ciscoFlags.GRIBIChecks)
	args.client.AddNHWithIPinIP(t, 5002, atePort8.IPv4, *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "Bundle-Ether1", false, false, ciscoFlags.GRIBIChecks)
	args.client.AddNHG(t, 5002, 0, map[uint64]uint64{5002: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.AddIPv4(t, "192.0.2.52/32", 5002, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// Telemerty check
	aftCheck(ctx, t, args)

	// END  FULL CHECK ########################################################################
	// REPLACE
	args.client.ReplaceNH(t, 10, "decap", *ciscoFlags.DefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNHG(t, 101, 0, map[uint64]uint64{10: 100}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix(dstPfx, i, mask))
	}
	args.client.ReplaceNH(t, 100, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNH(t, 200, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceIPv4Batch(t, prefixes, 100, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNH(t, 1000, atePort2.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNH(t, 1100, atePort3.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNH(t, 1200, atePort4.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNH(t, 1300, atePort5.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceIPv4(t, "192.0.2.40/32", 1000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNH(t, 2000, atePort6.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNH(t, 2100, atePort7.IPv4, *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.client.ReplaceIPv4(t, "192.0.2.42/32", 2000, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// REPLACE Telemerty check
	aftCheck(ctx, t, args)

}

// func (c *Client) AddNHWithIPinIP(t testing.TB, nhIndex uint64, address, instance string, nhInstance string, subinterfaceRef string, expecteFailure bool, check *flags.GRIBICheck) {
// 	NH := fluent.NextHopEntry().
// 		WithNetworkInstance(instance).
// 		WithIndex(nhIndex)

// 	if address == "encap" {
// 		NH = NH.WithEncapsulateHeader(fluent.IPinIP)
// 	} else if address != "" {
// 		NH = NH.WithIPAddress(address).WithEncapsulateHeader(fluent.IPinIP).WithIPinIP("20.20.20.1", "10.10.10.1")
// 	}
// 	if nhInstance != "" {
// 		NH = NH.WithNextHopNetworkInstance(nhInstance)
// 	}
// 	if subinterfaceRef != "" {
// 		NH = NH.WithSubinterfaceRef(subinterfaceRef, 1)
// 	}
// 	c.fluentC.Modify().AddEntry(t, NH)
// 	if err := c.AwaitTimeout(context.Background(), t, timeout); err != nil {
// 		t.Fatalf("Error waiting to add NH: %v", err)
// 	}
// 	if expecteFailure {
// 		c.checkNHResult(t, fluent.ProgrammingFailed, constants.Add, nhIndex)
// 	} else {
// 		c.checkNHResult(t, fluent.InstalledInRIB, constants.Add, nhIndex)
// 		if check.FIBACK {
// 			c.checkNHResult(t, fluent.InstalledInFIB, constants.Add, nhIndex)
// 		}
// 	}
// 	if check.AFTCheck {
// 		nh := c.DUT.Telemetry().NetworkInstance(instance).Afts().NextHop(nhIndex).Get(t)
// 		if (*nh.Index != nhIndex) || (*nh.IpAddress != address) {
// 			t.Fatalf("AFT Check failed for aft/nexthop-entry got ip %s, want ip %s; got index %d , want index %d", *nh.IpAddress, address, *nh.Index, nhIndex)
// 		}
// 	}
// }

// AwaitTimeout calls a fluent client Await by adding a timeout to the context.
func (c *Client) AwaitTimeout(ctx context.Context, t testing.TB, timeout time.Duration) error {
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.fluentC.Await(subctx, t)
}

func (c *Client) checkNHResult(t testing.TB, expectedResult fluent.ProgrammingResult, operation constants.OpType, nhIndex uint64) {
	chk.HasResult(t, c.fluentC.Results(t),
		fluent.OperationResult().
			WithNextHopOperation(nhIndex).
			WithOperationType(operation).
			WithProgrammingResult(expectedResult).
			AsResult(),
		chk.IgnoreOperationID(),
	)
}

func TestOCAFT(t *testing.T) {
	t.Log("Name: OCAFT")
	t.Log("Description: Verify OC AFT gNMI Subscribe")

	dut := ondatra.DUT(t, "dut")

	// Dial gRIBI
	ctx := context.Background()

	// Configure the DUT
	configureDUT(t, dut)
	//configbasePBR(t, dut, "TE", "ipv4", 1, telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})

	// Configure the ATE
	// ate := ondatra.ATE(t, "ate")
	// top := configureATE(t, ate)
	// addPrototoAte(t, top)

	test := []struct {
		name string
		desc string
		fn   func(ctx context.Context, t *testing.T, args *testArgs)
	}{
		{
			name: "AFT Verification",
			desc: "AFT Verification with base use case",
			fn:   testAFT,
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
				// ate:    ate,
				// top:    top,
			}
			tt.fn(ctx, t, args)
		})
	}
}
