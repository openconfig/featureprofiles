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
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	gnps "github.com/openconfig/gnoi/system"
	"github.com/openconfig/gnoigo/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/acl"
	"github.com/openconfig/ondatra/gnoi"
	"github.com/openconfig/ondatra/ixnet"
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
	ipv4DstTrafficEnd        = "203.0.113.254"
	ipv4Src                  = "192.0.2.2"
	aclNullPrefix            = "0.0.0.0/0"
	aclName                  = "BGP-DENY-ACL"
	aclv6Name                = "ipv6-policy-acl"
	routeCount               = 254
	dutAS                    = 64500
	ateAS                    = 64501
	plenIPv4                 = 30
	plenIPv6                 = 126
	bgpPort                  = 179
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
	setALLOWPolicy           = "ALLOW"
	bgpMED                   = 25
	aclStatement3            = "30"
)

var (
	bgpPeer          *ixnet.BGPPeer
	dutPort1SubIntf1 = attrs.Attributes{
		Desc:    "DUT to ATE sub interface 1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort1SubIntf1 = attrs.Attributes{
		Name:    "ateSrc",
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
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::192:0:2:6",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	routingDaemon = map[ondatra.Vendor]string{
		ondatra.JUNIPER: "rpd",
		ondatra.ARISTA:  "Bgp-main",
		ondatra.CISCO:   "emsd",
		ondatra.NOKIA:   "sr_bgp_mgr",
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
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
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
	dutConfNIPath := dc.NetworkInstance(deviations.DefaultNetworkInstance(dut))
	gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

	if deviations.InterfaceConfigVRFBeforeAddress(dut) {
		gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)
		gnmi.Replace(t, dut, dc.Interface(i2.GetName()).Config(), i2)
	}

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
		fptest.SetPortSpeed(t, dut.Port(t, "port2"))
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, i1.GetName(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, i2.GetName(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

func verifyPortsUp(t *testing.T, dev *ondatra.Device) {
	t.Helper()
	for _, p := range dev.Ports() {
		status := gnmi.Get(t, dev, gnmi.OC().Interface(p.Name()).OperStatus().State())
		if want := oc.Interface_OperStatus_UP; status != want {
			t.Errorf("%s Status: got %v, want %v", p, status, want)
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

func configureATE(t *testing.T, ate *ondatra.ATEDevice) (*ondatra.ATETopology, []*ondatra.Flow, []*ondatra.Interface) {
	t.Helper()
	topo := ate.Topology().New()
	port1 := ate.Port(t, "port1")

	iDut1SubIntf1 := topo.AddInterface(atePort1SubIntf1.Name).WithPort(port1)
	iDut1SubIntf1.Ethernet().WithVLANID(vlan10)
	iDut1SubIntf1.IPv4().WithAddress(atePort1SubIntf1.IPv4CIDR()).WithDefaultGateway(dutPort1SubIntf1.IPv4)
	iDut1SubIntf1.IPv6().WithAddress(atePort1SubIntf1.IPv6CIDR()).WithDefaultGateway(dutPort1SubIntf1.IPv6)

	iDut1SubIntf2 := topo.AddInterface(atePort1SubIntf2.Name).WithPort(port1)
	iDut1SubIntf2.Ethernet().WithVLANID(vlan20)
	iDut1SubIntf2.IPv4().WithAddress(atePort1SubIntf2.IPv4CIDR()).WithDefaultGateway(dutPort1SubIntf2.IPv4)
	iDut1SubIntf2.IPv6().WithAddress(atePort1SubIntf2.IPv6CIDR()).WithDefaultGateway(dutPort1SubIntf2.IPv6)

	iDut1SubIntf3 := topo.AddInterface(atePort1SubIntf3.Name).WithPort(port1)
	iDut1SubIntf3.Ethernet().WithVLANID(vlan30)
	iDut1SubIntf3.IPv4().WithAddress(atePort1SubIntf3.IPv4CIDR()).WithDefaultGateway(dutPort1SubIntf3.IPv4)
	iDut1SubIntf3.IPv6().WithAddress(atePort1SubIntf3.IPv6CIDR()).WithDefaultGateway(dutPort1SubIntf3.IPv6)

	iDut1SubIntf4 := topo.AddInterface(atePort1SubIntf4.Name).WithPort(port1)
	iDut1SubIntf4.Ethernet().WithVLANID(vlan40)
	iDut1SubIntf4.IPv4().WithAddress(atePort1SubIntf4.IPv4CIDR()).WithDefaultGateway(dutPort1SubIntf4.IPv4)
	iDut1SubIntf4.IPv6().WithAddress(atePort1SubIntf4.IPv6CIDR()).WithDefaultGateway(dutPort1SubIntf4.IPv6)

	iDut1SubIntf5 := topo.AddInterface(atePort1SubIntf5.Name).WithPort(port1)
	iDut1SubIntf5.Ethernet().WithVLANID(vlan50)
	iDut1SubIntf5.IPv4().WithAddress(atePort1SubIntf5.IPv4CIDR()).WithDefaultGateway(dutPort1SubIntf5.IPv4)
	iDut1SubIntf5.IPv6().WithAddress(atePort1SubIntf5.IPv6CIDR()).WithDefaultGateway(dutPort1SubIntf5.IPv6)

	iDut1SubIntf6 := topo.AddInterface(atePort1SubIntf6.Name).WithPort(port1)
	iDut1SubIntf6.Ethernet().WithVLANID(vlan60)
	iDut1SubIntf6.IPv4().WithAddress(atePort1SubIntf6.IPv4CIDR()).WithDefaultGateway(dutPort1SubIntf6.IPv4)
	iDut1SubIntf6.IPv6().WithAddress(atePort1SubIntf6.IPv6CIDR()).WithDefaultGateway(dutPort1SubIntf6.IPv6)

	port2 := ate.Port(t, "port2")
	iDut2 := topo.AddInterface(ateDst.Name).WithPort(port2)
	iDut2.IPv4().WithAddress(ateDst.IPv4CIDR()).WithDefaultGateway(dutDst.IPv4)
	iDut2.IPv6().WithAddress(ateDst.IPv6CIDR()).WithDefaultGateway(dutDst.IPv6)

	// Setup ATE BGP route v4 advertisement.
	bgpDut1 := iDut1SubIntf1.BGP()
	bgpDut1.AddPeer().WithPeerAddress(dutPort1SubIntf1.IPv4).WithLocalASN(ateAS).
		WithTypeExternal().Capabilities().WithGracefulRestart(true)
	bgpDut1.AddPeer().WithPeerAddress(dutPort1SubIntf1.IPv6).WithLocalASN(ateAS).
		WithTypeExternal().Capabilities().WithGracefulRestart(true)

	bgpDut2 := iDut2.BGP()
	bgpPeer = bgpDut2.AddPeer().WithPeerAddress(dutDst.IPv4).WithLocalASN(ateAS).
		WithTypeExternal()
	bgpPeer.Capabilities().WithGracefulRestart(true)

	bgpDut2.AddPeer().WithPeerAddress(dutDst.IPv6).WithLocalASN(ateAS).
		WithTypeExternal().Capabilities().WithGracefulRestart(true)

	bgpNeti1 := iDut1SubIntf1.AddNetwork("bgpNeti1")
	bgpNeti1.IPv4().WithAddress(advertisedRoutesv4CIDRp2).WithCount(routeCount)
	bgpNeti1.BGP().WithNextHopAddress(atePort1SubIntf1.IPv4)

	bgpNeti1v6 := iDut1SubIntf1.AddNetwork("bgpNeti1v6")
	bgpNeti1v6.IPv6().WithAddress(advertisedRoutesv6CIDRp2).WithCount(routeCount)
	bgpNeti1v6.BGP().WithActive(true).WithNextHopAddress(atePort1SubIntf1.IPv6)

	bgpNeti2 := iDut2.AddNetwork("bgpNeti2")
	bgpNeti2.IPv4().WithAddress(advertisedRoutesv4CIDR).WithCount(routeCount)
	bgpNeti2.BGP().WithNextHopAddress(ateDst.IPv4)

	bgpNeti2v6 := iDut2.AddNetwork("bgpNeti2v6")
	bgpNeti2v6.IPv6().WithAddress(advertisedRoutesv6CIDR).WithCount(routeCount)
	bgpNeti2v6.BGP().WithActive(true).WithNextHopAddress(ateDst.IPv6)

	t.Log("Pushing config to ATE and starting protocols...")
	topo.Push(t).StartProtocols(t)

	// ATE Traffic Configuration.
	t.Log("TestBGP:start ate Traffic config")
	ethHeader := ondatra.NewEthernetHeader()
	ethHeader.WithVLANID(vlan10)
	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.WithSrcAddress(ipv4SrcTraffic).DstAddressRange().
		WithMin(ipv4DstTrafficStart).WithMax(ipv4DstTrafficEnd).
		WithCount(routeCount)
	flowipv4 := ate.Traffic().NewFlow(flow1).
		WithSrcEndpoints(iDut1SubIntf1).
		WithDstEndpoints(iDut2).
		WithHeaders(ethHeader, ipv4Header).
		WithFrameSize(512)

	return topo, []*ondatra.Flow{flowipv4}, []*ondatra.Interface{iDut1SubIntf2, iDut1SubIntf3, iDut1SubIntf4, iDut1SubIntf5, iDut1SubIntf6}
}

func verifyNoPacketLoss(t *testing.T, ate *ondatra.ATEDevice, allFlows []*ondatra.Flow) {
	t.Helper()
	captureTrafficStats(t, ate)
	for _, flow := range allFlows {
		if lossPct := gnmi.Get(t, ate, gnmi.OC().Flow(flow.Name()).LossPct().State()); lossPct < 5.0 {
			t.Logf("Traffic Test Passed! Got %v loss", lossPct)
		} else {
			t.Errorf("Traffic Loss Pct for Flow %s: got %v", flow.Name(), lossPct)
		}
	}
}

func confirmPacketLoss(t *testing.T, ate *ondatra.ATEDevice, allFlows []*ondatra.Flow) {
	t.Helper()
	for _, flow := range allFlows {
		if lossPct := gnmi.Get(t, ate, gnmi.OC().Flow(flow.Name()).LossPct().State()); lossPct > 99.0 {
			t.Logf("Traffic Test Passed! Loss seen as expected: got %v, want 100%% ", lossPct)
		} else {
			t.Errorf("Traffic %s is expected to fail: got %v, want 100%% failure", flow.Name(), lossPct)
		}
	}
}

func captureTrafficStats(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	ap := ate.Port(t, "port1")
	aic1 := gnmi.OC().Interface(ap.Name()).Counters()
	sentPkts := gnmi.Get(t, ate, aic1.OutPkts().State())
	fptest.LogQuery(t, "ate:port1 counters", aic1.State(), gnmi.Get(t, ate, aic1.State()))

	op := ate.Port(t, "port2")
	aic2 := gnmi.OC().Interface(op.Name()).Counters()
	rxPkts := gnmi.Get(t, ate, aic2.InPkts().State())
	fptest.LogQuery(t, "ate:port2 counters", aic2.State(), gnmi.Get(t, ate, aic2.State()))
	var lostPkts uint64
	// Account for control plane packets in rxPkts.
	if rxPkts > sentPkts {
		lostPkts = rxPkts - sentPkts
	} else {
		lostPkts = sentPkts - rxPkts
	}
	t.Logf("Packets: %d sent, %d received, %d lost", sentPkts, rxPkts, lostPkts)
}

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice, allFlows []*ondatra.Flow, duration time.Duration) {
	t.Helper()
	t.Log("Starting traffic")
	ate.Traffic().Start(t, allFlows...)
	time.Sleep(duration)
	ate.Traffic().Stop(t)
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

func disableLLGRConf(dut *ondatra.DUTDevice, as int) string {
	switch dut.Vendor() {
	case ondatra.ARISTA:
		return fmt.Sprintf(`
		router bgp %d
		no graceful-restart-helper long-lived`, as)
	case ondatra.JUNIPER:
		return `
		protocols {
			bgp {
				graceful-restart {
					long-lived {
						receiver {
							disable;
						}
					}
				}
			}
		}`
	default:
		return ""
	}
}

func removeATENewPeers(t *testing.T, topo *ondatra.ATETopology, bgpPeers []*ixnet.BGP) {
	t.Helper()
	for _, peer := range bgpPeers {
		peer.ClearPeers()
	}
	topo.Update(t)
}

func removeNewPeers(t *testing.T, dut *ondatra.DUTDevice, nbrs []*bgpNeighbor) {
	t.Helper()
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	for _, nbr := range nbrs {
		gnmi.Delete(t, dut, dutConfPath.Neighbor(nbr.neighborip).Config())
	}
	fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))
}

func restartRoutingProcess(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	if _, ok := routingDaemon[dut.Vendor()]; !ok {
		t.Fatalf("Please add support for vendor %v in var routingDaemon", dut.Vendor())
	}
	t.Run("KillGRIBIDaemon", func(t *testing.T) {
		// Find the PID of routing Daemon.
		var pId uint64
		pName := routingDaemon[dut.Vendor()]
		t.Run("FindroutingDaemonPid", func(t *testing.T) {
			pId = findProcessByName(t, dut, pName)
			if pId == 0 {
				t.Fatalf("Couldn't find pid of routing daemon '%s'", pName)
			} else {
				t.Logf("Pid of routing daemon '%s' is '%d'", pName, pId)
			}
		})

		// Kill routing daemon through gNOI Kill Request.
		t.Run("ExecuteGnoiKill", func(t *testing.T) {
			// TODO - pid type is uint64 in oc-system model, but uint32 in gNOI Kill Request proto.
			// Until the models are brought in line, typecasting the uint64 to uint32.
			gNOIKillProcess(t, dut, pName, uint32(pId))
			// Wait for a bit for routing daemon on the DUT to restart.
			time.Sleep(30 * time.Second)
		})
	})
}

// findProcessByName uses telemetry to find out the PID of a process
func findProcessByName(t *testing.T, dut *ondatra.DUTDevice, pName string) uint64 {
	t.Helper()
	pList := gnmi.GetAll(t, dut, gnmi.OC().System().ProcessAny().State())
	var pID uint64
	for _, proc := range pList {
		if proc.GetName() == pName {
			pID = proc.GetPid()
			t.Logf("Pid of daemon '%s' is '%d'", pName, pID)
		}
	}
	return pID
}

// gNOIKillProcess kills a daemon on the DUT, given its name and pid.
func gNOIKillProcess(t *testing.T, dut *ondatra.DUTDevice, pName string, pID uint32) {
	t.Helper()
	killResponse := gnoi.Execute(t, dut, system.NewKillProcessOperation().Name(pName).PID(pID).Signal(gnps.KillProcessRequest_SIGNAL_TERM).Restart(true))
	t.Logf("Got kill process response: %v\n\n", killResponse)
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
	actions5.GetOrCreateBgpActions().SetLocalPref = ygot.Uint32(100)
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		gnmi.Update(t, dut, dutConfPath.PeerGroup(peerv4GrpName).ApplyPolicy().ExportPolicy().Config(), []string{"ALLOW", setMEDPolicy})
		gnmi.Update(t, dut, dutConfPath.PeerGroup(peerv4GrpName).ApplyPolicy().ImportPolicy().Config(), []string{"ALLOW", setMEDPolicy})
		gnmi.Update(t, dut, dutConfPath.PeerGroup(peerv6GrpName).ApplyPolicy().ExportPolicy().Config(), []string{"ALLOW", setMEDPolicy})
		gnmi.Update(t, dut, dutConfPath.PeerGroup(peerv6GrpName).ApplyPolicy().ImportPolicy().Config(), []string{"ALLOW", setMEDPolicy})
	} else {
		gnmi.Update(t, dut, dutConfPath.PeerGroup(peerv4GrpName).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy().ImportPolicy().Config(), []string{"ALLOW", setMEDPolicy})
		gnmi.Update(t, dut, dutConfPath.PeerGroup(peerv4GrpName).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy().ExportPolicy().Config(), []string{"ALLOW", setMEDPolicy})
		gnmi.Update(t, dut, dutConfPath.PeerGroup(peerv6GrpName).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy().ImportPolicy().Config(), []string{"ALLOW", setMEDPolicy})
		gnmi.Update(t, dut, dutConfPath.PeerGroup(peerv6GrpName).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy().ExportPolicy().Config(), []string{"ALLOW", setMEDPolicy})
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
	gnmi.Update(t, dut, dutConfPath.Config(), niProto)
	fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))
}

// configureATENewPeers configures five more new BGP peers on ATE.
func configureATENewPeers(t *testing.T, topo *ondatra.ATETopology, intfList []*ondatra.Interface) []*ixnet.BGP {
	t.Helper()
	bgpDut1 := intfList[0].BGP()
	bgpDut1.AddPeer().WithPeerAddress(dutPort1SubIntf2.IPv4).WithLocalASN(ateAS).
		WithTypeExternal().Capabilities().WithGracefulRestart(true)

	bgpDut2 := intfList[1].BGP()
	bgpDut2.AddPeer().WithPeerAddress(dutPort1SubIntf3.IPv4).WithLocalASN(ateAS).
		WithTypeExternal().Capabilities().WithGracefulRestart(true)

	bgpDut3 := intfList[2].BGP()
	bgpDut3.AddPeer().WithPeerAddress(dutPort1SubIntf4.IPv4).WithLocalASN(ateAS).
		WithTypeExternal().Capabilities().WithGracefulRestart(true)

	bgpDut4 := intfList[3].BGP()
	bgpDut4.AddPeer().WithPeerAddress(dutPort1SubIntf5.IPv4).WithLocalASN(ateAS).
		WithTypeExternal().Capabilities().WithGracefulRestart(true)

	bgpDut5 := intfList[4].BGP()
	bgpDut5.AddPeer().WithPeerAddress(dutPort1SubIntf6.IPv4).WithLocalASN(ateAS).
		WithTypeExternal().Capabilities().WithGracefulRestart(true)

	t.Logf("Pushing config to ATE and starting protocols...")
	topo.Update(t)

	return []*ixnet.BGP{bgpDut1, bgpDut2, bgpDut3, bgpDut4, bgpDut5}
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

	if llgrTimer := gnmi.Get(t, dut, nbrPath.GracefulRestart().StaleRoutesTime().State()); llgrTimer != grStaleRouteTime {
		t.Errorf("LLGR timer is incorrect, want %v, got %v", grStaleRouteTime, llgrTimer)
	}
	if grState := gnmi.Get(t, dut, nbrPath.GracefulRestart().Enabled().State()); grState != true {
		t.Errorf("Graceful restart enabled state is incorrect, want true, got %v", grState)
	}
	if peerRestartTime := gnmi.Get(t, dut, nbrPath.GracefulRestart().PeerRestartTime().State()); peerRestartTime != 0 {
		t.Errorf("Peer restart time is incorrect, want 0, got %v", peerRestartTime)
	}
	if peerRestartState := gnmi.Get(t, dut, nbrPath.GracefulRestart().PeerRestarting().State()); peerRestartState != true {
		t.Errorf("Peer restart state is incorrect, want true , got %v", peerRestartState)
	}
	if localRestartState := gnmi.Get(t, dut, nbrPath.GracefulRestart().LocalRestarting().State()); localRestartState != false {
		t.Errorf("Local restart state is incorrect, want false, got %v", localRestartState)
	}
	if grMode := gnmi.Get(t, dut, nbrPath.GracefulRestart().Mode().State()); grMode != oc.GracefulRestart_Mode_HELPER_ONLY {
		t.Errorf("Graceful restart mode is incorrect, want oc.GracefulRestart_Mode_HELPER_ONLY, got %v", grMode)
	}
	if nbrAfiSafiGrState := gnmi.Get(t, dut, nbrPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GracefulRestart().Enabled().State()); nbrAfiSafiGrState != true {
		t.Errorf("Neighbor AFI-SAFI graceful restart state is incorrect, want true, got %v", nbrAfiSafiGrState)
	}
}

func buildCliConfigRequest(config string) *gpb.SetRequest {
	// Build config with Origin set to cli and Ascii encoded config.
	gpbSetRequest := &gpb.SetRequest{
		Update: []*gpb.Update{{
			Path: &gpb.Path{
				Origin: "cli",
			},
			Val: &gpb.TypedValue{
				Value: &gpb.TypedValue_AsciiVal{
					AsciiVal: config,
				},
			},
		}},
	}
	return gpbSetRequest
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

	var allFlows []*ondatra.Flow
	var topo *ondatra.ATETopology
	var ateIntfList []*ondatra.Interface
	t.Run("configureATE", func(t *testing.T) {
		topo, allFlows, ateIntfList = configureATE(t, ate)
	})

	t.Run("verifyDUTPorts", func(t *testing.T) {
		verifyPortsUp(t, dut.Device)
	})

	t.Run("VerifyBGPParameters", func(t *testing.T) {
		checkBgpStatus(t, dut, nbrList)
	})

	t.Run("VerifyTrafficPassBeforeAcLBlock", func(t *testing.T) {
		t.Log("Send traffic with GR timer enabled. Traffic should pass.")
		sendTraffic(t, ate, allFlows, trafficDuration)
		verifyNoPacketLoss(t, ate, allFlows)
	})

	d := &oc.Root{}
	ifName := dut.Port(t, "port2").Name()
	iFace := d.GetOrCreateAcl().GetOrCreateInterface(ifName)
	t.Run("VerifyTrafficPasswithGRTimerWithAclApplied", func(t *testing.T) {
		t.Log("Configure Acl to block BGP on port 179")
		const stopDuration = 45 * time.Second
		t.Log("Starting traffic")
		ate.Traffic().Start(t, allFlows...)
		startTime := time.Now()
		t.Log("Trigger graceful restart on ATE")
		ate.Actions().NewBGPGracefulRestart().WithRestartTime(grRestartTime * time.Second).WithPeers(bgpPeer).Send(t)
		gnmi.Replace(t, dut, gnmi.OC().Acl().AclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4).Config(), configACL(d, aclName))
		aclConf := configACLInterface(iFace, ifName)
		gnmi.Replace(t, dut, aclConf.Config(), iFace)

		t.Run("Verify graceful restart telemetry", func(t *testing.T) {
			verifyGracefulRestart(t, dut)
		})

		replaceDuration := time.Since(startTime)
		time.Sleep(grTimer - stopDuration - replaceDuration)
		t.Log("Send traffic while GR timer is counting down. Traffic should pass as BGP GR is enabled!")
		ate.Traffic().Stop(t)
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
			restartRoutingProcess(t, dut)
		})

		var bgpIxPeer []*ixnet.BGP
		t.Run("configure 5 more new BGP peers", func(t *testing.T) {
			configureDUTNewPeers(t, dut, dutNbrs)
			bgpIxPeer = configureATENewPeers(t, topo, ateIntfList)
		})

		t.Run("Remove newly added 5 BGP peers", func(t *testing.T) {
			removeNewPeers(t, dut, dutNbrs)
			removeATENewPeers(t, topo, bgpIxPeer)
		})

		t.Run("Remove policy configured", func(t *testing.T) {
			dutBgpV4PeerGroupPath := dutConfPath.Bgp().PeerGroup(peerv4GrpName)
			dutBgpV6PeerGroupPath := dutConfPath.Bgp().PeerGroup(peerv6GrpName)
			if deviations.RoutePolicyUnderAFIUnsupported(dut) {
				gnmi.Replace(t, dut, dutBgpV4PeerGroupPath.ApplyPolicy().ExportPolicy().Config(), []string{"ALLOW"})
				gnmi.Replace(t, dut, dutBgpV4PeerGroupPath.ApplyPolicy().ImportPolicy().Config(), []string{"ALLOW"})
				gnmi.Replace(t, dut, dutBgpV6PeerGroupPath.ApplyPolicy().ExportPolicy().Config(), []string{"ALLOW"})
				gnmi.Replace(t, dut, dutBgpV6PeerGroupPath.ApplyPolicy().ImportPolicy().Config(), []string{"ALLOW"})
			} else {
				gnmi.Replace(t, dut, dutBgpV4PeerGroupPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy().ImportPolicy().Config(), []string{"ALLOW"})
				gnmi.Replace(t, dut, dutBgpV4PeerGroupPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy().ExportPolicy().Config(), []string{"ALLOW"})
				gnmi.Replace(t, dut, dutBgpV6PeerGroupPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy().ImportPolicy().Config(), []string{"ALLOW"})
				gnmi.Replace(t, dut, dutBgpV6PeerGroupPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy().ExportPolicy().Config(), []string{"ALLOW"})
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
		sendTraffic(t, ate, allFlows, trafficDuration)
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
		sendTraffic(t, ate, allFlows, trafficDuration)
		verifyNoPacketLoss(t, ate, allFlows)
	})
}

func TestTrafficWithGracefulRestart(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	t.Run("configureDut", func(t *testing.T) {
		configureDUT(t, dut)
		configureRoutePolicy(t, dut, "ALLOW", oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	})

	nbrList := buildNbrList(ateAS)
	t.Run("configureBGP", func(t *testing.T) {
		dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
		dutConf := bgpWithNbr(dutAS, nbrList, dut)
		gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
		fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))
	})

	var allFlows []*ondatra.Flow
	t.Run("configureATE", func(t *testing.T) {
		_, allFlows, _ = configureATE(t, ate)
	})

	t.Run("verifyDUTPorts", func(t *testing.T) {
		verifyPortsUp(t, dut.Device)
	})

	t.Run("VerifyBGPParameters", func(t *testing.T) {
		checkBgpStatus(t, dut, nbrList)
	})

	t.Run("VerifyTrafficPassBeforeAcLBlock", func(t *testing.T) {
		t.Log("Send traffic with GR timer enabled. Traffic should pass.")
		sendTraffic(t, ate, allFlows, trafficDuration)
		verifyNoPacketLoss(t, ate, allFlows)
	})

	if deviations.BgpLlgrOcUndefined(dut) {
		gnmiClient := dut.RawAPIs().GNMI(t)
		config := disableLLGRConf(dut, dutAS)
		t.Logf("Push the CLI config:%s", dut.Vendor())
		gpbSetRequest := buildCliConfigRequest(config)
		if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
			t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
		}
	}

	d := &oc.Root{}
	ifName := dut.Port(t, "port2").Name()
	iFace := d.GetOrCreateAcl().GetOrCreateInterface(ifName)
	t.Run("VerifyTrafficPasswithGRTimerWithAclApplied", func(t *testing.T) {
		t.Log("Configure ACL to block BGP on port 179")
		const stopDuration = 45 * time.Second
		t.Log("Starting traffic")
		ate.Traffic().Start(t, allFlows...)
		startTime := time.Now()
		t.Log("Trigger graceful restart on ATE")
		ate.Actions().NewBGPGracefulRestart().WithRestartTime(grRestartTime * time.Second).WithPeers(bgpPeer).Send(t)
		gnmi.Replace(t, dut, gnmi.OC().Acl().AclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4).Config(), configACL(d, aclName))
		aclConf := configACLInterface(iFace, ifName)
		gnmi.Replace(t, dut, aclConf.Config(), iFace)

		t.Run("Verify graceful restart telemetry", func(t *testing.T) {
			verifyGracefulRestart(t, dut)
		})

		replaceDuration := time.Since(startTime)
		time.Sleep(grTimer - stopDuration - replaceDuration)

		t.Log("Send traffic while GR timer is counting down. Traffic should pass as BGP GR is enabled!")
		ate.Traffic().Stop(t)
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
			t.Errorf("BGP session did not go Down as expected.")
		}
	})

	t.Run("VerifyTrafficFailureAfterGRexpired", func(t *testing.T) {
		t.Log("Send Traffic Again after GR timer has expired. This traffic should fail!")
		sendTraffic(t, ate, allFlows, trafficDuration)
		confirmPacketLoss(t, ate, allFlows)
	})

	t.Run("RemoveAclInterface", func(t *testing.T) {
		t.Log("Removing Acl on the interface to restore BGP GR. Traffic should now pass!")
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
		sendTraffic(t, ate, allFlows, trafficDuration)
		verifyNoPacketLoss(t, ate, allFlows)
	})
}
