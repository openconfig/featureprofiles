// Copyright 2023 Google LLC
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

package bgp_long_lived_graceful_restart_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gnoi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/acl"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// The testbed consists of ate:port1 -> dut:port1 and
// dut:port2 -> ate:port2.  The first pair is called the "source"
// pair, and the second the "destination" pair.
//
//   * Source: ate:port1 -> dut:port1 subnet 192.0.2.0/30 2001:db8::192:0:2:0/126
//   * Destination: dut:port2 -> ate:port2 subnet 192.0.2.4/30 2001:db8::192:0:2:4/126
//
// Note that the first (.0, .3) and last (.4, .7) IPv4 addresses are
// reserved from the subnet for broadcast, so a /30 leaves exactly 2
// usable addresses. This does not apply to IPv6 which allows /127
// for point to point links, but we use /126 so the numbering is
// consistent with IPv4.
//

const (
	trafficDuration          = 1 * time.Minute
	grTimer                  = 2 * time.Minute
	grRestartTime            = 120
	grStaleRouteTime         = 600
	ipv4SrcTraffic           = "192.0.2.2"
	advertisedRoutesv4CIDR   = "203.0.113.1/32"
	advertisedRoutesv6CIDR   = "2001:db8::203:0:113:1/128"
	advertisedRoutesv4CIDRp2 = "198.18.1.1/32"
	advertisedRoutesv6CIDRp2 = "2001:db8::198:18:1:1/128"
	ipv4DstTrafficStart      = "203.0.113.1"
	aclNullPrefix            = "0.0.0.0/0"
	aclName                  = "BGP-DENY-ACL"
	routeCount               = 254
	dutAS                    = 64500
	ateAS                    = 64501
	plenIPv4                 = 30
	plenIPv6                 = 126
	flow1                    = "v4FlowPort1toPort2"
	peerv4GrpName            = "BGP-PEER-GROUP-V4"
	peerv6GrpName            = "BGP-PEER-GROUP-V6"
	ateDstCIDR               = "192.0.2.6/32"
	vlan10                   = 10
	vlan20                   = 20
	vlan30                   = 30
	vlan40                   = 40
	vlan50                   = 50
	vlan60                   = 60
	setMEDPolicy             = "SET-MED"
	bgpMED                   = 25
	aclStatement3            = "30"
	gnmiRetryCount           = 3
	gnmiDeleteRetryCount     = 3
	gnmiSleepDuration        = 30 * time.Second
)

var (
	bgpPeerName      string
	dutPort1SubIntf1 = attrs.Attributes{
		Desc:    "DUT to ATE sub interface 1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort1SubIntf1 = attrs.Attributes{
		Name:    "ateSrc",
		MAC:     "00:00:00:00:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort1SubIntf2 = attrs.Attributes{
		Desc:    "DUT to ATE sub interface 2",
		IPv4:    "192.0.2.9",
		IPv6:    "2001:db8::192:0:3:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort1SubIntf2 = attrs.Attributes{
		Name:    "ateSrcSubIntf2",
		IPv4:    "192.0.2.10",
		IPv6:    "2001:db8::192:0:3:2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort1SubIntf3 = attrs.Attributes{
		Desc:    "DUT to ATE sub interface 3",
		IPv4:    "192.0.2.13",
		IPv6:    "2001:db8::192:0:4:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort1SubIntf3 = attrs.Attributes{
		Name:    "ateSrcSubIntf3",
		IPv4:    "192.0.2.14",
		IPv6:    "2001:db8::192:0:4:2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort1SubIntf4 = attrs.Attributes{
		Desc:    "DUT to ATE sub interface 4",
		IPv4:    "192.0.2.17",
		IPv6:    "2001:db8::192:0:5:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort1SubIntf4 = attrs.Attributes{
		Name:    "ateSrcSubIntf4",
		IPv4:    "192.0.2.18",
		IPv6:    "2001:db8::192:0:5:2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort1SubIntf5 = attrs.Attributes{
		Desc:    "DUT to ATE sub interface 5",
		IPv4:    "192.0.2.21",
		IPv6:    "2001:db8::192:0:6:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort1SubIntf5 = attrs.Attributes{
		Name:    "ateSrcSubIntf5",
		IPv4:    "192.0.2.22",
		IPv6:    "2001:db8::192:0:6:2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort1SubIntf6 = attrs.Attributes{
		Desc:    "DUT to ATE sub interface 6",
		IPv4:    "192.0.2.25",
		IPv6:    "2001:db8::192:0:7:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort1SubIntf6 = attrs.Attributes{
		Name:    "ateSrcSubIntf6",
		IPv4:    "192.0.2.26",
		IPv6:    "2001:db8::192:0:7:2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutDst = attrs.Attributes{
		Desc:    "DUT to ATE destination",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::192:0:2:5",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	ateDst = attrs.Attributes{
		Name:    "atedst",
		MAC:     "00:00:00:00:01:02",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::192:0:2:6",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
)

func configureRoutePolicy(t *testing.T, dut *ondatra.DUTDevice, name string, pr oc.E_RoutingPolicy_PolicyResultType) {
	t.Helper()
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pd := rp.GetOrCreatePolicyDefinition(name)
	st, err := pd.AppendNewStatement("id-1")
	if err != nil {
		t.Fatal(err)
	}
	st.GetOrCreateActions().PolicyResult = pr
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

func configInterfaceDUT(t *testing.T, i *oc.Interface, me *attrs.Attributes, subIntfIndex uint32, vlan uint16, dut *ondatra.DUTDevice) {
	t.Helper()
	i.Description = ygot.String(me.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	// Create subinterface.
	s := i.GetOrCreateSubinterface(subIntfIndex)

	if vlan != 0 {
		// Add VLANs.
		if deviations.DeprecatedVlanID(dut) {
			s.GetOrCreateVlan().VlanId = oc.UnionUint16(vlan)
		} else {
			singletag := s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged()
			singletag.VlanId = ygot.Uint16(vlan)
		}
	}
	// Add IPv4 stack.
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	a := s4.GetOrCreateAddress(me.IPv4)
	a.PrefixLength = ygot.Uint8(plenIPv4)

	// Add IPv6 stack.
	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(me.IPv6).PrefixLength = ygot.Uint8(plenIPv6)
}

// configureDUT configures all the interfaces and network instance on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dc := gnmi.OC()
	if deviations.InterfaceConfigVRFBeforeAddress(dut) {
		t.Log("Configure/update Network Instance")
		dutConfNIPath := dc.NetworkInstance(deviations.DefaultNetworkInstance(dut))
		gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
	}
	i1 := &oc.Interface{Name: ygot.String(dut.Port(t, "port1").Name())}
	configInterfaceDUT(t, i1, &dutPort1SubIntf1, 10, vlan10, dut)
	configInterfaceDUT(t, i1, &dutPort1SubIntf2, 20, vlan20, dut)
	configInterfaceDUT(t, i1, &dutPort1SubIntf3, 30, vlan30, dut)
	configInterfaceDUT(t, i1, &dutPort1SubIntf4, 40, vlan40, dut)
	configInterfaceDUT(t, i1, &dutPort1SubIntf5, 50, vlan50, dut)
	configInterfaceDUT(t, i1, &dutPort1SubIntf6, 60, vlan60, dut)

	if deviations.RequireRoutedSubinterface0(dut) {
		s := i1.GetOrCreateSubinterface(0).GetOrCreateIpv4()
		s.Enabled = ygot.Bool(true)
	}
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	i2 := dutDst.NewOCInterface(dut.Port(t, "port2").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i2.GetName()).Config(), i2)

	t.Log("Configure/update Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if deviations.InterfaceConfigVRFBeforeAddress(dut) {
		gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)
		gnmi.Replace(t, dut, dc.Interface(i2.GetName()).Config(), i2)
	}

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
		fptest.SetPortSpeed(t, dut.Port(t, "port2"))
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, i2.GetName(), deviations.DefaultNetworkInstance(dut), 0)
		if deviations.RequireRoutedSubinterface0(dut) {
			fptest.AssignToNetworkInstance(t, dut, i1.GetName(), deviations.DefaultNetworkInstance(dut), 0)
		}
		for _, subIntf := range []uint32{10, 20, 30, 40, 50, 60} {
			fptest.AssignToNetworkInstance(t, dut, i1.GetName(), deviations.DefaultNetworkInstance(dut), subIntf)
		}
	}
}

func verifyPortsUp(t *testing.T, dev *ondatra.Device) {
	t.Helper()
	for _, p := range dev.Ports() {
		status := gnmi.Get(t, dev, gnmi.OC().Interface(p.Name()).OperStatus().State())
		if want := oc.Interface_OperStatus_UP; status != want {
			t.Fatalf("%s Status: got %v, want %v", p, status, want)
		}
	}
}

type bgpNeighbor struct {
	as         uint32
	neighborip string
	isV4       bool
}

func buildNbrList(asN uint32) []*bgpNeighbor {
	nbr1v4 := &bgpNeighbor{as: asN, neighborip: atePort1SubIntf1.IPv4, isV4: true}
	nbr1v6 := &bgpNeighbor{as: asN, neighborip: atePort1SubIntf1.IPv6, isV4: false}
	nbr2v4 := &bgpNeighbor{as: asN, neighborip: ateDst.IPv4, isV4: true}
	nbr2v6 := &bgpNeighbor{as: asN, neighborip: ateDst.IPv6, isV4: false}
	return []*bgpNeighbor{nbr1v4, nbr2v4, nbr1v6, nbr2v6}
}

func bgpWithNbr(as uint32, nbrs []*bgpNeighbor, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	g := bgp.GetOrCreateGlobal()
	g.As = ygot.Uint32(as)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	g.RouterId = ygot.String(dutDst.IPv4)
	bgpgr := g.GetOrCreateGracefulRestart()
	bgpgr.Enabled = ygot.Bool(true)
	bgpgr.RestartTime = ygot.Uint16(grRestartTime)
	bgpgr.StaleRoutesTime = ygot.Uint16(grStaleRouteTime)

	pg := bgp.GetOrCreatePeerGroup(peerv4GrpName)
	pg.PeerAs = ygot.Uint32(ateAS)
	pg.PeerGroupName = ygot.String(peerv4GrpName)

	pgv6 := bgp.GetOrCreatePeerGroup(peerv6GrpName)
	pgv6.PeerAs = ygot.Uint32(ateAS)
	pgv6.PeerGroupName = ygot.String(peerv6GrpName)

	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		rpl := pg.GetOrCreateApplyPolicy()
		rpl.SetExportPolicy([]string{"ALLOW"})
		rpl.SetImportPolicy([]string{"ALLOW"})
		rplv6 := pgv6.GetOrCreateApplyPolicy()
		rplv6.SetExportPolicy([]string{"ALLOW"})
		rplv6.SetImportPolicy([]string{"ALLOW"})
	} else {
		pg1af4 := pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		pg1af4.Enabled = ygot.Bool(true)
		pg1rpl4 := pg1af4.GetOrCreateApplyPolicy()
		pg1rpl4.SetExportPolicy([]string{"ALLOW"})
		pg1rpl4.SetImportPolicy([]string{"ALLOW"})
		pg1af6 := pgv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		pg1af6.Enabled = ygot.Bool(true)
		pg1rpl6 := pg1af6.GetOrCreateApplyPolicy()
		pg1rpl6.SetExportPolicy([]string{"ALLOW"})
		pg1rpl6.SetImportPolicy([]string{"ALLOW"})
	}

	for _, nbr := range nbrs {
		bgpNbr := bgp.GetOrCreateNeighbor(nbr.neighborip)
		bgpNbr.GetOrCreateTimers().HoldTime = ygot.Uint16(180)
		bgpNbr.GetOrCreateTimers().KeepaliveInterval = ygot.Uint16(60)
		bgpNbr.PeerAs = ygot.Uint32(nbr.as)
		bgpNbr.Enabled = ygot.Bool(true)
		if nbr.isV4 {
			bgpNbr.PeerGroup = ygot.String(peerv4GrpName)
			af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(true)
			af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(false)
		} else {
			bgpNbr.PeerGroup = ygot.String(peerv6GrpName)
			bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(true)
			af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(false)
		}
	}
	return niProto
}

func checkBgpStatus(t *testing.T, dut *ondatra.DUTDevice, nbrIP []*bgpNeighbor) {
	t.Helper()
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	for _, nbr := range nbrIP {
		nbrPath := statePath.Neighbor(nbr.neighborip)
		t.Logf("Waiting for BGP neighbor to establish...")
		status, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			state, ok := val.Val()
			return ok && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
			t.Fatal("No BGP neighbor formed")
		}
		state, _ := status.Val()
		t.Logf("BGP adjacency for %s: %s", nbr.neighborip, state)
		if want := oc.Bgp_Neighbor_SessionState_ESTABLISHED; state != want {
			t.Errorf("BGP peer %s status got %d, want %d", nbr.neighborip, state, want)
		}

		t.Log("Verifying BGP capabilities.")
		capabilities := map[oc.E_BgpTypes_BGP_CAPABILITY]bool{
			oc.BgpTypes_BGP_CAPABILITY_ROUTE_REFRESH: false,
			oc.BgpTypes_BGP_CAPABILITY_MPBGP:         false,
		}
		for _, cap := range gnmi.Get(t, dut, nbrPath.SupportedCapabilities().State()) {
			capabilities[cap] = true
		}
		for cap, present := range capabilities {
			if !present {
				t.Errorf("Capability not reported: %v", cap)
			}
		}
	}
}

func addOTGSubInterface(t *testing.T, topo gosnappi.Config, portName string, a, gw *attrs.Attributes, vlan uint16) (gosnappi.Device, gosnappi.DeviceIpv4, gosnappi.DeviceIpv6) {
	t.Helper()
	dev := topo.Devices().Add().SetName(a.Name)
	eth := dev.Ethernets().Add().SetName(a.Name + ".Eth")
	if a.MAC != "" {
		eth.SetMac(a.MAC)
	}
	eth.Connection().SetPortName(portName)
	if vlan != 0 {
		eth.Vlans().Add().SetName(a.Name + ".VLAN").SetId(uint32(vlan))
	}
	ipv4 := eth.Ipv4Addresses().Add().SetName(a.Name + ".IPv4")
	ipv4.SetAddress(a.IPv4).SetGateway(gw.IPv4).SetPrefix(uint32(a.IPv4Len))
	ipv6 := eth.Ipv6Addresses().Add().SetName(a.Name + ".IPv6")
	ipv6.SetAddress(a.IPv6).SetGateway(gw.IPv6).SetPrefix(uint32(a.IPv6Len))
	return dev, ipv4, ipv6
}

func ipOnly(cidr string) string {
	ip, _, found := strings.Cut(cidr, "/")
	if found {
		return ip
	}
	return cidr
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) (gosnappi.Config, []string, []string) {
	t.Helper()
	otg := ate.OTG()
	topo := gosnappi.NewConfig()
	topo.Ports().Add().SetName("port1")
	topo.Ports().Add().SetName("port2")

	dev1, dev1v4, dev1v6 := addOTGSubInterface(t, topo, "port1", &atePort1SubIntf1, &dutPort1SubIntf1, vlan10)
	dev3, dev3v4, dev3v6 := addOTGSubInterface(t, topo, "port1", &atePort1SubIntf2, &dutPort1SubIntf2, vlan20)
	dev4, dev4v4, dev4v6 := addOTGSubInterface(t, topo, "port1", &atePort1SubIntf3, &dutPort1SubIntf3, vlan30)
	dev5, dev5v4, dev5v6 := addOTGSubInterface(t, topo, "port1", &atePort1SubIntf4, &dutPort1SubIntf4, vlan40)
	dev6, dev6v4, dev6v6 := addOTGSubInterface(t, topo, "port1", &atePort1SubIntf5, &dutPort1SubIntf5, vlan50)
	dev7, dev7v4, dev7v6 := addOTGSubInterface(t, topo, "port1", &atePort1SubIntf6, &dutPort1SubIntf6, vlan60)
	dev2, dev2v4, dev2v6 := addOTGSubInterface(t, topo, "port2", &ateDst, &dutDst, 0)

	// Setup OTG BGP sessions and route advertisement.
	b1 := dev1.Bgp().SetRouterId(dev1v4.Address())
	b1v4PeerName := atePort1SubIntf1.Name + ".BGP4.peer"
	b1v4 := b1.Ipv4Interfaces().Add().SetIpv4Name(dev1v4.Name()).Peers().Add().SetName(b1v4PeerName)
	b1v4.SetPeerAddress(dev1v4.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	b1v4.GracefulRestart().SetEnableGr(true).SetRestartTime(uint32(grRestartTime)).SetEnableLlgr(true)
	b1v6 := b1.Ipv6Interfaces().Add().SetIpv6Name(dev1v6.Name()).Peers().Add().SetName(atePort1SubIntf1.Name + ".BGP6.peer")
	b1v6.SetPeerAddress(dev1v6.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	b1v6.GracefulRestart().SetEnableGr(true).SetRestartTime(uint32(grRestartTime)).SetEnableLlgr(true)

	b1r4 := b1v4.V4Routes().Add().SetName("bgpNeti1")
	b1r4.SetNextHopIpv4Address(dev1v4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	b1r4.Addresses().Add().SetAddress(ipOnly(advertisedRoutesv4CIDRp2)).SetPrefix(32).SetCount(uint32(routeCount))

	b1r6 := b1v6.V6Routes().Add().SetName("bgpNeti1v6")
	b1r6.SetNextHopIpv6Address(dev1v6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	b1r6.Addresses().Add().SetAddress(ipOnly(advertisedRoutesv6CIDRp2)).SetPrefix(128).SetCount(uint32(routeCount))

	b2 := dev2.Bgp().SetRouterId(dev2v4.Address())
	bgpPeerName = ateDst.Name + ".BGP4.peer"
	b2v4 := b2.Ipv4Interfaces().Add().SetIpv4Name(dev2v4.Name()).Peers().Add().SetName(bgpPeerName)
	b2v4.SetPeerAddress(dev2v4.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	b2v4.GracefulRestart().SetEnableGr(true).SetRestartTime(uint32(grRestartTime))
	b2v6 := b2.Ipv6Interfaces().Add().SetIpv6Name(dev2v6.Name()).Peers().Add().SetName(ateDst.Name + ".BGP6.peer")
	b2v6.SetPeerAddress(dev2v6.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	b2v6.GracefulRestart().SetEnableGr(true).SetRestartTime(uint32(grRestartTime))

	b2r4 := b2v4.V4Routes().Add().SetName("bgpNeti2")
	b2r4.SetNextHopIpv4Address(dev2v4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	b2r4.Addresses().Add().SetAddress(ipOnly(advertisedRoutesv4CIDR)).SetPrefix(32).SetCount(uint32(routeCount))

	b2r6 := b2v6.V6Routes().Add().SetName("bgpNeti2v6")
	b2r6.SetNextHopIpv6Address(dev2v6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	b2r6.Addresses().Add().SetAddress(ipOnly(advertisedRoutesv6CIDR)).SetPrefix(128).SetCount(uint32(routeCount))

	flowipv4 := topo.Flows().Add().SetName(flow1)
	flowipv4.Metrics().SetEnable(true)
	flowipv4.TxRx().Device().
		SetTxNames([]string{dev1v4.Name()}).
		SetRxNames([]string{b2r4.Name()})
	flowipv4.Size().SetFixed(512)
	flowipv4.Rate().SetPps(100)
	eth := flowipv4.Packet().Add().Ethernet()
	eth.Src().SetValue("aa:aa:aa:aa:aa:aa")
	flowipv4.Packet().Add().Vlan().Id().SetValue(uint32(vlan10))
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(ipv4SrcTraffic)
	v4.Dst().Increment().SetStart(ipv4DstTrafficStart).SetCount(uint32(routeCount))

	// Configure the five additional BGP peers up front so the ATE config is
	// pushed only once. Their neighborship will only form once the matching
	// neighbors are configured on the DUT later in the test. Each peer mirrors
	// the structure of the working peers above (IPv4 + IPv6 BGP interface with
	// an advertised route range) to keep the OTG config valid.
	newPeerDevs := []gosnappi.Device{dev3, dev4, dev5, dev6, dev7}
	newPeerV4 := []gosnappi.DeviceIpv4{dev3v4, dev4v4, dev5v4, dev6v4, dev7v4}
	newPeerV6 := []gosnappi.DeviceIpv6{dev3v6, dev4v6, dev5v6, dev6v6, dev7v6}
	newPeerIntfNames := []string{atePort1SubIntf2.Name, atePort1SubIntf3.Name, atePort1SubIntf4.Name, atePort1SubIntf5.Name, atePort1SubIntf6.Name}
	newPeerNames := make([]string, 0, len(newPeerDevs))
	for i := range newPeerDevs {
		bgp := newPeerDevs[i].Bgp().SetRouterId(newPeerV4[i].Address())

		v4PeerName := newPeerIntfNames[i] + ".BGP4.peer"
		v4Peer := bgp.Ipv4Interfaces().Add().SetIpv4Name(newPeerV4[i].Name()).Peers().Add().SetName(v4PeerName)
		v4Peer.SetPeerAddress(newPeerV4[i].Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
		v4Peer.GracefulRestart().SetEnableGr(true).SetRestartTime(uint32(grRestartTime))
		v4Route := v4Peer.V4Routes().Add().SetName(newPeerIntfNames[i] + ".v4routes")
		v4Route.SetNextHopIpv4Address(newPeerV4[i].Address()).
			SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
			SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
		v4Route.Addresses().Add().SetAddress(fmt.Sprintf("198.51.%d.1", i+1)).SetPrefix(32).SetCount(1)

		v6PeerName := newPeerIntfNames[i] + ".BGP6.peer"
		v6Peer := bgp.Ipv6Interfaces().Add().SetIpv6Name(newPeerV6[i].Name()).Peers().Add().SetName(v6PeerName)
		v6Peer.SetPeerAddress(newPeerV6[i].Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
		v6Peer.GracefulRestart().SetEnableGr(true).SetRestartTime(uint32(grRestartTime))
		v6Route := v6Peer.V6Routes().Add().SetName(newPeerIntfNames[i] + ".v6routes")
		v6Route.SetNextHopIpv6Address(newPeerV6[i].Address()).
			SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
			SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
		v6Route.Addresses().Add().SetAddress(fmt.Sprintf("2001:db8::198:51:%d:1", i+1)).SetPrefix(128).SetCount(1)

		newPeerNames = append(newPeerNames, v4PeerName)
	}

	t.Log("Pushing config to ATE and starting protocols...")
	otg.PushConfig(t, topo)
	otg.StartProtocols(t)

	return topo, []string{flow1}, newPeerNames
}

func verifyNoPacketLoss(t *testing.T, ate *ondatra.ATEDevice, allFlows []string) {
	t.Helper()
	otg := ate.OTG()
	c := otg.FetchConfig(t)
	otgutils.LogFlowMetrics(t, otg, c)
	for _, flow := range allFlows {
		recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(flow).State())
		txPackets := float64(recvMetric.GetCounters().GetOutPkts())
		rxPackets := float64(recvMetric.GetCounters().GetInPkts())
		if txPackets == 0 {
			t.Fatalf("Tx packets should be higher than 0 for flow %s", flow)
		}
		lossPct := (txPackets - rxPackets) * 100 / txPackets
		if lossPct < 5.0 {
			t.Logf("Traffic Test Passed! Got %v loss", lossPct)
		} else {
			t.Errorf("Traffic Loss Pct for Flow %s: got %v", flow, lossPct)
		}
	}
}

func confirmPacketLoss(t *testing.T, ate *ondatra.ATEDevice, allFlows []string) {
	t.Helper()
	otg := ate.OTG()
	c := otg.FetchConfig(t)
	otgutils.LogFlowMetrics(t, otg, c)
	for _, flow := range allFlows {
		recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(flow).State())
		txPackets := float64(recvMetric.GetCounters().GetOutPkts())
		rxPackets := float64(recvMetric.GetCounters().GetInPkts())
		if txPackets == 0 {
			t.Fatalf("Tx packets should be higher than 0 for flow %s", flow)
		}
		lossPct := (txPackets - rxPackets) * 100 / txPackets
		if lossPct > 99.0 {
			t.Logf("Traffic Test Passed! Loss seen as expected: got %v, want 100%% ", lossPct)
		} else {
			t.Errorf("Traffic %s is expected to fail: got %v, want 100%% failure", flow, lossPct)
		}
	}
}

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice, duration time.Duration) {
	t.Helper()
	t.Log("Starting traffic")
	ate.OTG().StartTraffic(t)
	time.Sleep(duration)
	ate.OTG().StopTraffic(t)
	t.Log("Traffic stopped")
}

func configACL(d *oc.Root, name string) *oc.Acl_AclSet {
	acl := d.GetOrCreateAcl().GetOrCreateAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4)
	aclEntry10 := acl.GetOrCreateAclEntry(10)
	aclEntry10.SequenceId = ygot.Uint32(10)
	aclEntry10.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_DROP
	a := aclEntry10.GetOrCreateIpv4()
	a.SourceAddress = ygot.String(aclNullPrefix)
	a.DestinationAddress = ygot.String(ateDstCIDR)

	aclEntry20 := acl.GetOrCreateAclEntry(20)
	aclEntry20.SequenceId = ygot.Uint32(20)
	aclEntry20.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_DROP
	a2 := aclEntry20.GetOrCreateIpv4()
	a2.SourceAddress = ygot.String(ateDstCIDR)
	a2.DestinationAddress = ygot.String(aclNullPrefix)

	aclEntry30 := acl.GetOrCreateAclEntry(30)
	aclEntry30.SequenceId = ygot.Uint32(30)
	aclEntry30.GetOrCreateActions().ForwardingAction = oc.Acl_FORWARDING_ACTION_ACCEPT
	a3 := aclEntry30.GetOrCreateIpv4()
	a3.SourceAddress = ygot.String(aclNullPrefix)
	a3.DestinationAddress = ygot.String(aclNullPrefix)
	return acl
}

func configAdmitAllACL(d *oc.Root, name string) *oc.Acl_AclSet {
	acl := d.GetOrCreateAcl().GetOrCreateAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4)
	acl.DeleteAclEntry(10)
	acl.DeleteAclEntry(20)
	return acl
}

func configACLInterface(iFace *oc.Acl_Interface, ifName string) *acl.Acl_InterfacePath {
	aclConf := gnmi.OC().Acl().Interface(ifName)
	if ifName != "" {
		iFace.GetOrCreateIngressAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4)
		iFace.GetOrCreateInterfaceRef().Interface = ygot.String(ifName)
		iFace.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	} else {
		iFace.GetOrCreateIngressAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4)
		iFace.DeleteIngressAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4)
	}
	return aclConf
}

func stopATENewPeers(t *testing.T, ate *ondatra.ATEDevice, peerNames []string) {
	t.Helper()
	cs := gosnappi.NewControlState()
	cs.Protocol().Bgp().Peers().SetPeerNames(peerNames).SetState(gosnappi.StateProtocolBgpPeersState.DOWN)
	ate.OTG().SetControlState(t, cs)
}

func removeNewPeers(t *testing.T, dut *ondatra.DUTDevice, nbrs []*bgpNeighbor) {
	t.Helper()
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	for _, nbr := range nbrs {
		deleteWithRetry(t, dut, dutConfPath.Neighbor(nbr.neighborip).Config())
	}
	fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))
}

// setBgpPolicy is used to configure routing policy on DUT.
func setBgpPolicy(t *testing.T, dut *ondatra.DUTDevice, d *oc.Root) {
	t.Helper()
	rp := d.GetOrCreateRoutingPolicy()
	pdef5 := rp.GetOrCreatePolicyDefinition(setMEDPolicy)
	stmt1, err := pdef5.AppendNewStatement(aclStatement3)
	if err != nil {
		t.Errorf("Error while creating new statement %v", err)
	}
	actions5 := stmt1.GetOrCreateActions()
	actions5.GetOrCreateBgpActions().SetMed = oc.UnionUint32(bgpMED)
	if !deviations.BGPSetMedActionUnsupported(dut) {
		actions5.GetOrCreateBgpActions().SetMedAction = oc.BgpPolicy_BgpSetMedAction_SET
	}
	actions5.GetOrCreateBgpActions().SetLocalPref = ygot.Uint32(100)
	updateWithRetry(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		updateWithRetry(t, dut, dutConfPath.PeerGroup(peerv4GrpName).ApplyPolicy().ExportPolicy().Config(), []string{"ALLOW", setMEDPolicy})
		updateWithRetry(t, dut, dutConfPath.PeerGroup(peerv4GrpName).ApplyPolicy().ImportPolicy().Config(), []string{"ALLOW", setMEDPolicy})
		updateWithRetry(t, dut, dutConfPath.PeerGroup(peerv6GrpName).ApplyPolicy().ExportPolicy().Config(), []string{"ALLOW", setMEDPolicy})
		updateWithRetry(t, dut, dutConfPath.PeerGroup(peerv6GrpName).ApplyPolicy().ImportPolicy().Config(), []string{"ALLOW", setMEDPolicy})
	} else {
		updateWithRetry(t, dut, dutConfPath.PeerGroup(peerv4GrpName).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy().ImportPolicy().Config(), []string{"ALLOW", setMEDPolicy})
		updateWithRetry(t, dut, dutConfPath.PeerGroup(peerv4GrpName).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy().ExportPolicy().Config(), []string{"ALLOW", setMEDPolicy})
		updateWithRetry(t, dut, dutConfPath.PeerGroup(peerv6GrpName).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy().ImportPolicy().Config(), []string{"ALLOW", setMEDPolicy})
		updateWithRetry(t, dut, dutConfPath.PeerGroup(peerv6GrpName).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy().ExportPolicy().Config(), []string{"ALLOW", setMEDPolicy})
	}
}

// configureDUTNewPeers configured five more BGP peers on subinterfaces.
func configureDUTNewPeers(t *testing.T, dut *ondatra.DUTDevice, nbrs []*bgpNeighbor) {
	t.Helper()
	d := &oc.Root{}
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	for _, nbr := range nbrs {
		pg1 := bgp.GetOrCreatePeerGroup(peerv4GrpName)
		pg1.PeerAs = ygot.Uint32(nbr.as)
		pg1.PeerGroupName = ygot.String(peerv4GrpName)
		nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
		nv4.PeerGroup = ygot.String(peerv4GrpName)
		nv4.PeerAs = ygot.Uint32(nbr.as)
		nv4.Enabled = ygot.Bool(true)
		af4 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		af4.Enabled = ygot.Bool(true)
		af6 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		af6.Enabled = ygot.Bool(false)
	}
	updateWithRetry(t, dut, dutConfPath.Config(), niProto)
	fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))
}

func createGracefulRestartAction(peerNames []string, restartDelay uint32) gosnappi.ControlAction {
	grAction := gosnappi.NewControlAction()
	grAction.Protocol().Bgp().InitiateGracefulRestart().SetPeerNames(peerNames).SetRestartDelay(restartDelay)
	return grAction
}

// verifyGracefulRestart validates graceful restart telemetry on DUT.
func verifyGracefulRestart(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateDst.IPv4)

	isGrEnabled := gnmi.Get(t, dut, statePath.Global().GracefulRestart().Enabled().State())
	if isGrEnabled {
		t.Logf("Graceful restart is enabled as Expected")
	} else {
		t.Errorf("Expected Graceful restart status: got %v, want Enabled", isGrEnabled)
	}
	grTimerVal := gnmi.Get(t, dut, statePath.Global().GracefulRestart().RestartTime().State())
	if grTimerVal == uint16(grRestartTime) {
		t.Logf("Graceful restart timer enabled as expected to be %v", grRestartTime)
	} else {
		t.Errorf("Expected Graceful restart timer: got %v, want %v", grTimerVal, grRestartTime)
	}

	if !deviations.BgpLlgrOcUndefined(dut) {
		if llgrTimer := gnmi.Get(t, dut, nbrPath.GracefulRestart().StaleRoutesTime().State()); llgrTimer != grStaleRouteTime {
			t.Errorf("LLGR timer is incorrect, want %v, got %v", grStaleRouteTime, llgrTimer)
		}
	}
	grState, present := gnmi.Lookup(t, dut, nbrPath.GracefulRestart().Enabled().State()).Val()
	if !present && deviations.MissingValueForDefaults(dut) {
		grState = true
	} else if !present {
		t.Errorf("Graceful restart enabled state is not present")
	}
	if grState != true {
		t.Errorf("Graceful restart enabled state is incorrect, want true, got %v", grState)
	}
	if !deviations.BgpLlgrOcUndefined(dut) {
		peerRestartState, present := gnmi.Lookup(t, dut, nbrPath.GracefulRestart().PeerRestarting().State()).Val()
		if !present && deviations.MissingValueForDefaults(dut) {
			peerRestartState = true
		} else if !present {
			t.Errorf("Graceful restart peer-restarting state is not present")
		}
		if peerRestartState != true {
			peerRestartTime, present := gnmi.Lookup(t, dut, nbrPath.GracefulRestart().PeerRestartTime().State()).Val()
			if !present && deviations.MissingValueForDefaults(dut) {
				peerRestartTime = 0
			} else if !present {
				t.Errorf("Peer restart time is not present")
			}
			if peerRestartTime != 0 {
				t.Errorf("Peer restart time is incorrect, want 0, got %v", peerRestartTime)
			}
			t.Errorf("Peer restart state is incorrect, want true, got %v", peerRestartState)
		}
		if localRestartState := gnmi.Get(t, dut, nbrPath.GracefulRestart().LocalRestarting().State()); localRestartState != false {
			t.Errorf("Local restart state is incorrect, want false, got %v", localRestartState)
		}
		if grMode := gnmi.Get(t, dut, nbrPath.GracefulRestart().Mode().State()); grMode != oc.GracefulRestart_Mode_HELPER_ONLY && grMode != oc.GracefulRestart_Mode_BILATERAL {
			t.Errorf("Graceful restart mode is incorrect, want %v or %v, got %v", oc.GracefulRestart_Mode_HELPER_ONLY, oc.GracefulRestart_Mode_BILATERAL, grMode)
		}
	}
	if !deviations.BgpGracefulRestartUnderAfiSafiUnsupported(dut) {
		nbrAfiSafiGrState, present := gnmi.Lookup(t, dut, nbrPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GracefulRestart().Enabled().State()).Val()
		if !present && deviations.MissingValueForDefaults(dut) {
			nbrAfiSafiGrState = true
		} else if !present {
			t.Errorf("Neighbor AFI-SAFI Graceful restart enabled state is not present")
		}
		if nbrAfiSafiGrState != true {
			t.Errorf("Neighbor AFI-SAFI Graceful restart status: got %v, want Enabled", nbrAfiSafiGrState)
		}
	}
}

func replaceWithRetry[T any](t *testing.T, dut *ondatra.DUTDevice, q ygnmi.ConfigQuery[T], val T) {
	t.Helper()
	gnmiOperationWithRetry(t, "Replace", gnmiRetryCount, func() error {
		gnmiClient := dut.RawAPIs().GNMI(t)
		c, err := ygnmi.NewClient(gnmiClient, ygnmi.WithTarget(dut.Name()))
		if err != nil {
			return fmt.Errorf("failed to create ygnmi client: %w", err)
		}
		_, err = ygnmi.Replace(context.Background(), c, q, val)
		return err
	})
}

func updateWithRetry[T any](t *testing.T, dut *ondatra.DUTDevice, q ygnmi.ConfigQuery[T], val T) {
	t.Helper()
	gnmiOperationWithRetry(t, "Update", gnmiRetryCount, func() error {
		gnmiClient := dut.RawAPIs().GNMI(t)
		c, err := ygnmi.NewClient(gnmiClient, ygnmi.WithTarget(dut.Name()))
		if err != nil {
			return fmt.Errorf("failed to create ygnmi client: %w", err)
		}
		_, err = ygnmi.Update(context.Background(), c, q, val)
		return err
	})
}

func deleteWithRetry[T any](t *testing.T, dut *ondatra.DUTDevice, q ygnmi.ConfigQuery[T]) {
	t.Helper()
	gnmiOperationWithRetry(t, "Delete", gnmiDeleteRetryCount, func() error {
		gnmiClient := dut.RawAPIs().GNMI(t)
		c, err := ygnmi.NewClient(gnmiClient, ygnmi.WithTarget(dut.Name()))
		if err != nil {
			return fmt.Errorf("failed to create ygnmi client: %w", err)
		}
		_, err = ygnmi.Delete(context.Background(), c, q)
		return err
	})
}

func gnmiOperationWithRetry(t *testing.T, opName string, retryCount int, op func() error) {
	t.Helper()
	for i := 0; i < retryCount; i++ {
		err := op()
		if err == nil {
			return
		}
		t.Logf("%s failed, retrying... Attempt %d/%d. Error: %v", opName, i+1, retryCount, err)
		time.Sleep(gnmiSleepDuration)
	}
	t.Fatalf("%s failed after %d attempts", opName, retryCount)
}

func TestTrafficWithGracefulRestartLLGR(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	t.Run("configureDut", func(t *testing.T) {
		configureDUT(t, dut)
		configureRoutePolicy(t, dut, "ALLOW", oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	})

	nbrList := buildNbrList(ateAS)
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	t.Run("configureBGP", func(t *testing.T) {
		dutConf := bgpWithNbr(dutAS, nbrList, dut)
		gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
		fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))
	})

	var allFlows []string
	var ateBgpPeers []string
	if ok := t.Run("configureATE", func(t *testing.T) {
		_, allFlows, ateBgpPeers = configureATE(t, ate)
	}); !ok {
		panic("configureATE failed")
	}

	t.Run("verifyDUTPorts", func(t *testing.T) {
		verifyPortsUp(t, dut.Device)
	})

	t.Run("VerifyBGPParameters", func(t *testing.T) {
		checkBgpStatus(t, dut, nbrList)
	})

	t.Run("VerifyTrafficPassBeforeAcLBlock", func(t *testing.T) {
		t.Log("Send traffic with GR timer enabled. Traffic should pass.")
		sendTraffic(t, ate, trafficDuration)
		verifyNoPacketLoss(t, ate, allFlows)
	})

	d := &oc.Root{}
	ifName := dut.Port(t, "port2").Name()
	iFace := d.GetOrCreateAcl().GetOrCreateInterface(ifName)
	t.Run("VerifyTrafficPasswithGRTimerWithAclApplied", func(t *testing.T) {
		t.Log("Configure Acl to block BGP on port 179")
		const stopDuration = 45 * time.Second
		t.Log("Starting traffic")
		ate.OTG().StartTraffic(t)
		startTime := time.Now()
		t.Log("Trigger graceful restart on ATE")
		ate.OTG().SetControlAction(t, createGracefulRestartAction([]string{bgpPeerName}, uint32(grRestartTime)))
		gnmi.Replace(t, dut, gnmi.OC().Acl().AclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4).Config(), configACL(d, aclName))
		aclConf := configACLInterface(iFace, ifName)
		gnmi.Replace(t, dut, aclConf.Config(), iFace)

		t.Run("Verify graceful restart telemetry", func(t *testing.T) {
			verifyGracefulRestart(t, dut)
		})

		replaceDuration := time.Since(startTime)
		time.Sleep(grTimer - stopDuration - replaceDuration)
		t.Log("Send traffic while GR timer is counting down. Traffic should pass as BGP GR is enabled!")
		ate.OTG().StopTraffic(t)
		t.Log("Traffic stopped")
		verifyNoPacketLoss(t, ate, allFlows)
	})

	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateDst.IPv4)
	t.Run("VerifyBGPNOTEstablished", func(t *testing.T) {
		t.Log("Waiting for BGP neighbor to be not in Established state after applying ACL DENY policy..")
		_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			currState, ok := val.Val()
			return ok && currState != oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
			t.Errorf("BGP session did not go Down as expected")
		}
	})

	startTime := time.Now()

	dutNbr1 := &bgpNeighbor{as: ateAS, neighborip: atePort1SubIntf2.IPv4, isV4: true}
	dutNbr2 := &bgpNeighbor{as: ateAS, neighborip: atePort1SubIntf3.IPv4, isV4: true}
	dutNbr3 := &bgpNeighbor{as: ateAS, neighborip: atePort1SubIntf4.IPv4, isV4: true}
	dutNbr4 := &bgpNeighbor{as: ateAS, neighborip: atePort1SubIntf5.IPv4, isV4: true}
	dutNbr5 := &bgpNeighbor{as: ateAS, neighborip: atePort1SubIntf6.IPv4, isV4: true}
	dutNbrs := []*bgpNeighbor{dutNbr1, dutNbr2, dutNbr3, dutNbr4, dutNbr5}

	t.Run("Verify different BGP Operations during graceful restart", func(t *testing.T) {

		t.Run("Configure MED routing policy", func(t *testing.T) {
			setBgpPolicy(t, dut, d)
			time.Sleep(2 * time.Second)
		})

		t.Run("Restart routing", func(t *testing.T) {
			if deviations.RoutingRestartViaGnoiUnsupported(dut) {
				t.Skip("Skipping routing restart via gNOI due to deviation")
			}
			gnoi.KillProcess(t, dut, gnoi.ROUTING, gnoi.SigTerm, true, true)
		})

		t.Run("configure 5 more new BGP peers", func(t *testing.T) {
			if deviations.BgpConfigDuringGracefulRestartUnsupported(dut) {
				t.Skip("Skipping BGP Peer configuration during graceful restart due to deviation")
			}
			configureDUTNewPeers(t, dut, dutNbrs)
		})

		t.Run("Remove newly added 5 BGP peers", func(t *testing.T) {
			if deviations.BgpConfigDuringGracefulRestartUnsupported(dut) {
				t.Skip("Skipping BGP Peer removal during graceful restart due to deviation")
			}
			removeNewPeers(t, dut, dutNbrs)
			stopATENewPeers(t, ate, ateBgpPeers)
		})

		t.Run("Remove policy configured", func(t *testing.T) {
			if deviations.BgpConfigDuringGracefulRestartUnsupported(dut) {
				t.Skip("Skipping BGP Policy removal during graceful restart due to deviation")
			}
			dutBgpV4PeerGroupPath := dutConfPath.Bgp().PeerGroup(peerv4GrpName)
			dutBgpV6PeerGroupPath := dutConfPath.Bgp().PeerGroup(peerv6GrpName)
			if deviations.RoutePolicyUnderAFIUnsupported(dut) {
				replaceWithRetry(t, dut, dutBgpV4PeerGroupPath.ApplyPolicy().ExportPolicy().Config(), []string{"ALLOW"})
				replaceWithRetry(t, dut, dutBgpV4PeerGroupPath.ApplyPolicy().ImportPolicy().Config(), []string{"ALLOW"})
				replaceWithRetry(t, dut, dutBgpV6PeerGroupPath.ApplyPolicy().ExportPolicy().Config(), []string{"ALLOW"})
				replaceWithRetry(t, dut, dutBgpV6PeerGroupPath.ApplyPolicy().ImportPolicy().Config(), []string{"ALLOW"})
			} else {
				replaceWithRetry(t, dut, dutBgpV4PeerGroupPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy().ImportPolicy().Config(), []string{"ALLOW"})
				replaceWithRetry(t, dut, dutBgpV4PeerGroupPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy().ExportPolicy().Config(), []string{"ALLOW"})
				replaceWithRetry(t, dut, dutBgpV6PeerGroupPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy().ImportPolicy().Config(), []string{"ALLOW"})
				replaceWithRetry(t, dut, dutBgpV6PeerGroupPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy().ExportPolicy().Config(), []string{"ALLOW"})
			}
		})
	})

	t.Run("Wait till LLGR/Stale timer expires to delete long live routes.....", func(t *testing.T) {
		replaceDuration := time.Since(startTime)
		staleTime := time.Duration(grRestartTime+grStaleRouteTime) * time.Second
		time.Sleep(staleTime - replaceDuration)
	})

	t.Run("VerifyTrafficFailureAfterGRexpired", func(t *testing.T) {
		t.Log("Send traffic again after GR timer has expired. This traffic should fail!")
		sendTraffic(t, ate, trafficDuration)
		confirmPacketLoss(t, ate, allFlows)
	})

	t.Run("RemoveAclInterface", func(t *testing.T) {
		t.Log("Removing ACL on the interface to restore BGP GR. Traffic should now pass!")
		gnmi.Replace(t, dut, gnmi.OC().Acl().AclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4).Config(), configAdmitAllACL(d, aclName))
		aclPath := configACLInterface(iFace, ifName)
		gnmi.Replace(t, dut, aclPath.Config(), iFace)
	})

	t.Run("VerifyBGPEstablished", func(t *testing.T) {
		t.Logf("Waiting for BGP neighbor to establish...")
		_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			currState, ok := val.Val()
			return ok && currState == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
			t.Errorf("BGP session not Established as expected")
		}
	})

	t.Run("VerifyTrafficPassBGPRestored", func(t *testing.T) {
		status := gnmi.Get(t, dut, nbrPath.SessionState().State())
		if want := oc.Bgp_Neighbor_SessionState_ESTABLISHED; status != want {
			t.Errorf("Get(BGP peer %s status): got %d, want %d", ateDst.IPv4, status, want)
		}
		sendTraffic(t, ate, trafficDuration)
		verifyNoPacketLoss(t, ate, allFlows)
	})
}
