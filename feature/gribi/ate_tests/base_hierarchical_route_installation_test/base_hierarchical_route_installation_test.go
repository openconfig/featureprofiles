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

package base_hierarchical_route_installation_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the test
// topology.
//
// The testbed consists of ate:port1 -> dut:port1
// and dut:port2 -> ate:port2.
//
//   - ate:port1 -> dut:port1 subnet 192.0.2.0/30
//   - ate:port2 -> dut:port2 subnet 192.0.2.4/30
const (
	ipv4PrefixLen     = 30
	ateDstNetCIDR     = "198.51.100.0/24"
	ateIndirectNH     = "203.0.113.1"
	ateIndirectNHCIDR = ateIndirectNH + "/32"
	nhIndex           = 1
	nhgIndex          = 42
	nhIndex2          = 2
	nhgIndex2         = 52
	nonDefaultVRF     = "VRF-1"
	nhMAC             = "00:1A:11:00:00:01"
	macFilter         = "1"                 // Decimal equalent of last 7 bits in nhMAC
	ipNHMAC           = "00:1A:11:11:11:11" //static ARP MAC for ip NH
	ipNHFilter        = "4369"              //Decimal Value of last 15 bits on ipNHMAC
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
	}
	atePorts = map[string]attrs.Attributes{
		"port1": atePort1,
		"port2": atePort2,
	}
	dutPorts = map[string]attrs.Attributes{
		"port1": dutPort1,
		"port2": dutPort2,
	}
)

// configInterfaceDUT configures the interface with the Addrs.
func configInterfaceDUT(i *oc.Interface, a *attrs.Attributes) *oc.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if *deviations.InterfaceEnabled {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if *deviations.InterfaceEnabled && !*deviations.IPv4MissingEnabled {
		s4.Enabled = ygot.Bool(true)
	}
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(ipv4PrefixLen)

	return i
}

// configureNetworkInstance creates nonDefaultVRF and dutPort1 to VRF
func configureNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(nonDefaultVRF)
	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	p := dut.Port(t, "port1")
	i := ni.GetOrCreateInterface(p.Name())
	i.Interface = ygot.String(p.Name())
	i.Subinterface = ygot.Uint32(0)
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(nonDefaultVRF).Config(), ni)
}

// configureDUT configures port1 and port2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	for p, dp := range dutPorts {
		p1 := dut.Port(t, p)
		i1 := &oc.Interface{Name: ygot.String(p1.Name())}
		gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dp))
		if *deviations.ExplicitPortSpeed {
			fptest.SetPortSpeed(t, p1)
		}
		if *deviations.ExplicitIPv6EnableForGRIBI {
			gnmi.Update(t, dut, d.Interface(p1.Name()).Subinterface(0).Ipv6().Enabled().Config(), bool(true))
		}
	}

	configureNetworkInstance(t, dut)

	if *deviations.ExplicitInterfaceInDefaultVRF {
		fptest.AssignToNetworkInstance(t, dut, dut.Port(t, "port2").Name(), *deviations.DefaultNetworkInstance, 0)
	}
	if *deviations.ExplicitGRIBIUnderNetworkInstance {
		fptest.EnableGRIBIUnderNetworkInstance(t, dut, nonDefaultVRF)
		fptest.EnableGRIBIUnderNetworkInstance(t, dut, *deviations.DefaultNetworkInstance)
	}
	//Configure static ARP for IP NH to validate flows based on MAC filter
	p := dut.Port(t, "port2")
	i := &oc.Interface{Name: ygot.String(p.Name())}
	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	n4 := s4.GetOrCreateNeighbor(atePort2.IPv4)
	n4.LinkLayerAddress = ygot.String(ipNHMAC)
	gnmi.Update(t, dut, gnmi.OC().Interface(p.Name()).Config(), i)
}

// configureATE configures port1 and port2 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	top := ate.Topology().New()
	for p, ap := range atePorts {
		p1 := ate.Port(t, p)
		i1 := top.AddInterface(ap.Name).WithPort(p1)
		i1.IPv4().
			WithAddress(ap.IPv4CIDR()).
			WithDefaultGateway(dutPorts[p].IPv4)
	}
	return top
}

// createFlow returns a flow from atePort1 to the dstPfx.
func createFlow(t *testing.T, ate *ondatra.ATEDevice, top *ondatra.ATETopology, name string) *ondatra.Flow {
	srcEndPoint := top.Interfaces()[atePort1.Name]
	dstEndPoint := top.Interfaces()[atePort2.Name]
	ethHeader := ondatra.NewEthernetHeader()
	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.DstAddressRange().
		WithMin("198.51.100.0").
		WithMax("198.51.100.254").
		WithCount(250)

	flow := ate.Traffic().NewFlow("Flow").
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint).
		WithHeaders(ethHeader, ipv4Header)
	flow.EgressTracking().WithOffset(33).WithWidth(15)
	return flow
}

// ValidateTraffic generates traffic flow from source network to
// destination network via srcEndPoint to dstEndPoint and checks for
// packet loss and returns loss percentage as float.
// Also validate the flows are with expected EgressTracking filter
func ValidateTraffic(t *testing.T, ate *ondatra.ATEDevice, top *ondatra.ATETopology, flow *ondatra.Flow, flowFilter string) float32 {
	ate.Traffic().Start(t, flow)
	time.Sleep(15 * time.Second)
	ate.Traffic().Stop(t)

	time.Sleep(time.Minute)
	flowPath := gnmi.OC().Flow(flow.Name())
	val, _ := gnmi.Watch(t, ate, flowPath.LossPct().State(), time.Minute, func(val *ygnmi.Value[float32]) bool {
		return val.IsPresent()
	}).Await(t)
	lossPct, present := val.Val()
	if !present {
		t.Fatalf("Could not read loss percentage for flow %q from ATE.", flow.Name())
	}
	if lossPct != 100 {
		inPkts := gnmi.Get(t, ate, flowPath.State()).GetCounters().GetInPkts()
		etPath := flowPath.EgressTrackingAny()
		ets := gnmi.GetAll(t, ate, etPath.State())
		if got := len(ets); got != 1 {
			t.Errorf("EgressTracking got %d items, want %d", got, 1)
		}
		if got := ets[0].GetFilter(); got != flowFilter {
			t.Errorf("EgressTracking filter got %q, want %q", got, macFilter)
		}
		if got := ets[0].GetCounters().GetInPkts(); got != inPkts {
			t.Errorf("EgressTracking counter in-pkts got %d, want %d", got, inPkts)
		}
	}
	return lossPct
}

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx    context.Context
	dut    *ondatra.DUTDevice
	ate    *ondatra.ATEDevice
	top    *ondatra.ATETopology
	client *gribi.Client
}

func verifyTelemetry(t *testing.T, args *testArgs, nhtype string) {

	// Verify that the entry for 198.51.100.0/24 (a) is installed through AFT Telemetry. a->c or a->b are the expected results.
	ipv4Entry := gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(nonDefaultVRF).Afts().Ipv4Entry(ateDstNetCIDR).State())
	if got, want := ipv4Entry.GetPrefix(), ateDstNetCIDR; got != want {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/prefix = %v, want %v", got, want)
	}
	if got, want := ipv4Entry.GetOriginProtocol(), oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_GRIBI; got != want {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/origin-protocol = %v, want %v", got, want)
	}
	if got, want := ipv4Entry.GetNextHopGroupNetworkInstance(), *deviations.DefaultNetworkInstance; got != want {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/next-hop-group-network-instance = %v, want %v", got, want)
	}
	nhgIndexInst := ipv4Entry.GetNextHopGroup()
	if nhgIndexInst == 0 {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/next-hop-group is not present")
	}
	nhg := gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().NextHopGroup(nhgIndexInst).State())
	if got, want := nhg.GetProgrammedId(), uint64(nhgIndex); got != want {
		t.Errorf("TestRecursiveIPv4Entry: next-hop-group/state/programmed-id = %v, want %v", got, want)
	}

	for nhIndexInst, nhgNH := range nhg.NextHop {
		if got, want := nhgNH.GetIndex(), uint64(nhIndexInst); got != want {
			t.Errorf("next-hop index is incorrect: got %v, want %v", got, want)
		}
		nh := gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().NextHop(nhIndexInst).State())
		// for devices that return  the nexthop with resolving it recursively. For a->b->c the device returns c
		if got := nh.GetIpAddress(); got != ateIndirectNH {
			if nhtype == "MAC" {
				if gotMac := nh.GetMacAddress(); !strings.EqualFold(gotMac, nhMAC) {
					t.Errorf("next-hop MAC is incorrect:  gotMac %v, wantMac %v", gotMac, nhMAC)
				}
			} else {
				if got := nh.GetIpAddress(); got != atePort2.IPv4 {
					t.Errorf("next-hop is incorrect: got %v, want %v ", got, atePort2.IPv4)
				}
			}
		}
	}

	// Verify that the entry for 203.0.113.1/32 (b) is installed through AFT Telemetry.
	ipv4Entry = gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().Ipv4Entry(ateIndirectNHCIDR).State())
	if got, want := ipv4Entry.GetPrefix(), ateIndirectNHCIDR; got != want {
		t.Errorf("TestRecursiveIPv4Entry = %v: ipv4-entry/state/prefix, want %v", got, want)
	}
	if got, want := ipv4Entry.GetOriginProtocol(), oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_GRIBI; got != want {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/origin-protocol = %v, want %v", got, want)
	}
	if got, want := ipv4Entry.GetNextHopGroupNetworkInstance(), *deviations.DefaultNetworkInstance; got != want {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/next-hop-group-network-instance = %v, want %v", got, want)
	}
	nhgIndexInst = ipv4Entry.GetNextHopGroup()
	if nhgIndexInst == 0 {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/next-hop-group is not present")
	}
	nhg = gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().NextHopGroup(nhgIndexInst).State())
	if got, want := nhg.GetProgrammedId(), uint64(nhgIndex2); got != want {
		t.Errorf("TestRecursiveIPv4Entry: next-hop-group/state/programmed-id = %v, want %v", got, want)
	}

	for nhIndexInst, nhgNH := range nhg.NextHop {
		if got, want := nhgNH.GetIndex(), uint64(nhIndexInst); got != want {
			t.Errorf("next-hop index is incorrect: got %v, want %v", got, want)
		}
		nh := gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().NextHop(nhIndexInst).State())
		if nhtype == "MAC" {
			if got, want := nh.GetMacAddress(), nhMAC; !strings.EqualFold(got, want) {
				t.Errorf("next-hop MAC is incorrect: got %v, want %v", got, want)
			}

		} else {
			if got, want := nh.GetIpAddress(), atePort2.IPv4; got != want {
				t.Errorf("next-hop address is incorrect: got %v, want %v", got, want)
			}
		}
	}
}

// testRecursiveIPv4Entry verifies recursive IPv4 Entry for 198.51.100.0/24 (a) -> 203.0.113.1/32 (b) -> 192.0.2.6 (c).
// The IPv4 Entry is verified through AFT Telemetry and Traffic.
func testRecursiveIPv4Entry(t *testing.T, args *testArgs) {

	t.Logf("Adding IP %v with NHG %d NH %d with IP %v as NH via gRIBI", ateIndirectNH, nhgIndex2, nhIndex2, atePort2.IPv4)
	args.client.AddNH(t, nhIndex2, atePort2.IPv4, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)
	args.client.AddNHG(t, nhgIndex2, map[uint64]uint64{nhIndex2: 1}, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)
	args.client.AddIPv4(t, ateIndirectNHCIDR, nhgIndex2, *deviations.DefaultNetworkInstance, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)

	t.Logf("Adding IP %v with NHG %d NH %d  with indirect IP %v via gRIBI", ateDstNetCIDR, nhgIndex, nhIndex, ateIndirectNHCIDR)
	args.client.AddNH(t, nhIndex, ateIndirectNH, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)
	args.client.AddNHG(t, nhgIndex, map[uint64]uint64{nhIndex: 1}, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)
	args.client.AddIPv4(t, ateDstNetCIDR, nhgIndex, nonDefaultVRF, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)
	baseFlow := createFlow(t, args.ate, args.top, "BaseFlow")
	t.Run("ValidateTelemtry", func(t *testing.T) {
		t.Log("Validate Telemetry to verify IPV4 entry is resolved through IP next-hop")
		verifyTelemetry(t, args, "IP")
	})

	t.Run("ValidateTraffic", func(t *testing.T) {
		t.Log("Validate Traffic is recieved on atePort2 with dst MAC as IP NH's static ARP")
		if got, want := ValidateTraffic(t, args.ate, args.top, baseFlow, ipNHFilter), 0; int(got) != want {
			t.Errorf("Loss: got %v, want %v", got, want)
		}
	})

	t.Logf("Deleting NH entry and verifing there is no traffic")
	args.client.DeleteIPv4(t, ateIndirectNHCIDR, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)

	ipv4Path := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().Ipv4Entry(ateIndirectNHCIDR)
	if gnmi.Lookup(t, args.dut, ipv4Path.State()).IsPresent() {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/prefix: Found route %s that should not exist", ateIndirectNHCIDR)
	}
	t.Run("ValidateNoTrafficAfterNHDelete", func(t *testing.T) {
		t.Log("Validate No traffic Traffic is recieved on atePort2 after NH delete")
		if got, want := ValidateTraffic(t, args.ate, args.top, baseFlow, ipNHFilter), 100; int(got) != want {
			t.Errorf("Loss: got %v, want %v", got, want)
		}
	})
}

// testRecursiveMacEntry verifies recursive IPv4 Entry for 198.51.100.0/24 (a) -> 203.0.113.1/32 (b) -> Port1 + MAC
// The IPv4 Entry is verified through AFT Telemetry and Traffic.
func testRecursiveMacEntry(t *testing.T, args *testArgs) {

	p := args.dut.Port(t, "port2")
	t.Logf("Adding IP %v with NHG %d NH %d with interface %v and MAC %v as NH via gRIBI", ateIndirectNH, nhgIndex2, nhIndex2, p.Name(), nhMAC)
	args.client.AddNH(t, nhIndex2, "MACwithInterface", *deviations.DefaultNetworkInstance, fluent.InstalledInRIB, &gribi.NHOptions{Interface: p.Name(), SubInterface: 0, Mac: nhMAC})
	args.client.AddNHG(t, nhgIndex2, map[uint64]uint64{nhIndex2: 1}, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)
	args.client.AddIPv4(t, ateIndirectNHCIDR, nhgIndex2, *deviations.DefaultNetworkInstance, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)

	t.Logf("Adding IP %v with NHG %d NH %d  with indirect IP %v via gRIBI", ateDstNetCIDR, nhgIndex, nhIndex, ateIndirectNHCIDR)
	args.client.AddNH(t, nhIndex, ateIndirectNH, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)
	args.client.AddNHG(t, nhgIndex, map[uint64]uint64{nhIndex: 1}, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)
	args.client.AddIPv4(t, ateDstNetCIDR, nhgIndex, nonDefaultVRF, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)
	baseFlow := createFlow(t, args.ate, args.top, "BaseFlow")

	t.Run("ValidateTelemtry", func(t *testing.T) {
		t.Log("Validate Telemetry to verify IPV4 entry is resolved through MAC next-hop")
		verifyTelemetry(t, args, "MAC")
	})

	t.Run("ValidateTraffic", func(t *testing.T) {
		t.Log("Validate Traffic is recieved on atePort2 with dst MAC as gRIBI NH MAC")
		if got, want := ValidateTraffic(t, args.ate, args.top, baseFlow, macFilter), 0; int(got) != want {
			t.Errorf("Loss: got %v, want %v", got, want)
		}
	})

	t.Logf("Deleting NH entry and verifing there is no traffic")
	args.client.DeleteIPv4(t, ateIndirectNHCIDR, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)

	ipv4Path := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().Ipv4Entry(ateIndirectNHCIDR)
	if gnmi.Lookup(t, args.dut, ipv4Path.State()).IsPresent() {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/prefix: Found route %s that should not exist", ateIndirectNHCIDR)
	}
	t.Run("ValidateNoTrafficAfterNHDelete", func(t *testing.T) {
		t.Log("Validate No traffic Traffic is recieved on atePort2 after NH delete")
		if got, want := ValidateTraffic(t, args.ate, args.top, baseFlow, macFilter), 100; int(got) != want {
			t.Errorf("Loss: got %v, want %v", got, want)
		}
	})
}

func TestRecursiveIPv4Entries(t *testing.T) {

	ctx := context.Background()

	// Configure DUT
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	top.Push(t).StartProtocols(t)

	tests := []struct {
		name string
		desc string
		fn   func(t *testing.T, args *testArgs)
	}{
		{
			name: "TestRecursiveIPv4Entry",
			desc: "Program IPV4 entry recursively to IP next-hop and verify with Telemetry and Traffic.",
			fn:   testRecursiveIPv4Entry,
		},
		{
			name: "TestRecursiveMACEntry",
			desc: "Program IPV4 entry recursively to MAC next-hop and verify with Telemetry and Traffic",
			fn:   testRecursiveMacEntry,
		},
	}

	const (
		usePreserve = "PRESERVE"
		useDelete   = "DELETE"
	)

	// Each case will run with its own gRIBI fluent client.
	for _, persist := range []string{usePreserve, useDelete} {
		t.Run(fmt.Sprintf("Persistence=%s", persist), func(t *testing.T) {
			if *deviations.GRIBIPreserveOnly && persist == useDelete {
				t.Skip("Skipping due to --deviation_gribi_preserve_only")
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					t.Logf("Name: %s", tc.name)
					t.Logf("Description: %s", tc.desc)

					// Configure the gRIBI client
					client := gribi.Client{
						DUT: dut,
					}
					if persist == usePreserve {
						client.Persistence = true
					}

					defer client.Close(t)
					if err := client.Start(t); err != nil {
						t.Fatalf("gRIBI Connection can not be established")
					}
					client.BecomeLeader(t)
					if persist == usePreserve {
						defer client.FlushAll(t)
					}

					args := &testArgs{
						ctx:    ctx,
						dut:    dut,
						ate:    ate,
						top:    top,
						client: &client,
					}

					tc.fn(t, args)
				})
			}
		})

	}
}
