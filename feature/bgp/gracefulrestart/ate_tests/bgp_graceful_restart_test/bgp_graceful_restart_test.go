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

package bgp_graceful_restart_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/acl"
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
	grStaleRouteTime         = 300
	ipv4SrcTraffic           = "192.0.2.2"
	ipv6SrcTraffic           = "2001:db8::192:0:2:2"
	ipv4DstTrafficStart      = "203.0.113.1"
	ipv4DstTrafficEnd        = "203.0.113.2"
	ipv6DstTrafficStart      = "2001:db8::203:0:113:1"
	ipv6DstTrafficEnd        = "2001:db8::203:0:113:2"
	advertisedRoutesv4CIDR   = "203.0.113.1/32"
	advertisedRoutesv6CIDR   = "2001:db8::203:0:113:1/128"
	advertisedRoutesv4CIDRp2 = "203.0.113.3/32"
	advertisedRoutesv6CIDRp2 = "2001:db8::203:0:113:3/128"
	aclNullPrefix            = "0.0.0.0/0"
	aclv6NullPrefix          = "::/0"
	aclName                  = "BGP-DENY-ACL"
	aclv6Name                = "ipv6-policy-acl"
	routeCount               = 2
	dutAS                    = 64500
	ateAS                    = 64501
	plenIPv4                 = 30
	plenIPv6                 = 126
	bgpPort                  = 179
	peerv4GrpName            = "BGP-PEER-GROUP-V4"
	peerv6GrpName            = "BGP-PEER-GROUP-V6"
	ateDstCIDR               = "192.0.2.6/32"
)

var (
	bgpPeer *ixnet.BGPPeer

	dutSrc = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
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
)

func configureRoutePolicy(t *testing.T, dut *ondatra.DUTDevice, name string, pr oc.E_RoutingPolicy_PolicyResultType) {
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

// configureDUT configures all the interfaces and network instance on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dc := gnmi.OC()
	i1 := dutSrc.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	i2 := dutDst.NewOCInterface(dut.Port(t, "port2").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i2.GetName()).Config(), i2)

	t.Log("Configure/update Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

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
	nbr1v4 := &bgpNeighbor{as: asN, neighborip: ateSrc.IPv4, isV4: true}
	nbr1v6 := &bgpNeighbor{as: asN, neighborip: ateSrc.IPv6, isV4: false}
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
		if nbr.isV4 {
			nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv4.PeerGroup = ygot.String(peerv4GrpName)
			nv4.GetOrCreateTimers().HoldTime = ygot.Uint16(180)
			nv4.GetOrCreateTimers().KeepaliveInterval = ygot.Uint16(60)
			nv4.PeerAs = ygot.Uint32(nbr.as)
			nv4.Enabled = ygot.Bool(true)
			af4 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(true)
			af6 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(false)
		} else {
			nv6 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv6.PeerGroup = ygot.String(peerv6GrpName)
			nv6.GetOrCreateTimers().HoldTime = ygot.Uint16(180)
			nv6.GetOrCreateTimers().KeepaliveInterval = ygot.Uint16(60)
			nv6.PeerAs = ygot.Uint32(nbr.as)
			nv6.Enabled = ygot.Bool(true)
			nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6 := nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(true)
			af4 := nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(false)
		}
	}
	return niProto
}

func checkBgpStatus(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Verifying BGP state")
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateSrc.IPv4)
	nbrPathv6 := statePath.Neighbor(ateSrc.IPv6)

	// Get BGP adjacency state
	t.Log("Waiting for BGP neighbor to establish...")
	_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		currState, ok := val.Val()
		return ok && currState == oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}).Await(t)
	if !ok {
		fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
		t.Fatal("No BGP neighbor formed...")
	}

	// Get BGPv6 adjacency state
	t.Log("Waiting for BGPv6 neighbor to establish...")
	_, ok = gnmi.Watch(t, dut, nbrPathv6.SessionState().State(), 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		currState, ok := val.Val()
		return ok && currState == oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}).Await(t)
	if !ok {
		fptest.LogQuery(t, "BGPv6 reported state", nbrPathv6.State(), gnmi.Get(t, dut, nbrPathv6.State()))
		t.Fatal("No BGPv6 neighbor formed...")
	}

	isGrEnabled := gnmi.Get(t, dut, statePath.Global().GracefulRestart().Enabled().State())
	t.Logf("isGrEnabled %v", isGrEnabled)
	if isGrEnabled {
		t.Logf("Graceful restart on neighbor %v enabled as Expected", ateDst.IPv4)
	} else {
		t.Errorf("Expected Graceful restart status on neighbor: got %v, want Enabled", isGrEnabled)
	}

	grTimerVal := gnmi.Get(t, dut, statePath.Global().GracefulRestart().RestartTime().State())
	t.Logf("grTimerVal %v", grTimerVal)
	if grTimerVal == uint16(grRestartTime) {
		t.Logf("Graceful restart timer enabled as expected to be %v", grRestartTime)
	} else {
		t.Errorf("Expected Graceful restart timer: got %v, want %v", grTimerVal, grRestartTime)
	}

	t.Log("Waiting for BGP v4 prefix to be installed")
	got, found := gnmi.Watch(t, dut, nbrPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes().Installed().State(), 180*time.Second, func(val *ygnmi.Value[uint32]) bool {
		prefixCount, ok := val.Val()
		return ok && prefixCount == routeCount
	}).Await(t)
	if !found {
		t.Errorf("Installed prefixes v4 mismatch: got %v, want %v", got, routeCount)
	}
	t.Log("Waiting for BGP v6 prefix to be installed")
	got, found = gnmi.Watch(t, dut, nbrPathv6.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Prefixes().Installed().State(), 180*time.Second, func(val *ygnmi.Value[uint32]) bool {
		prefixCount, ok := val.Val()
		return ok && prefixCount == routeCount
	}).Await(t)
	if !found {
		t.Errorf("Installed prefixes v6 mismatch: got %v, want %v", got, routeCount)
	}
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) []*ondatra.Flow {
	topo := ate.Topology().New()
	port1 := ate.Port(t, "port1")
	ifDut1 := topo.AddInterface(ateSrc.Name).WithPort(port1)
	ifDut1.IPv4().WithAddress(ateSrc.IPv4CIDR()).WithDefaultGateway(dutSrc.IPv4)
	ifDut1.IPv6().WithAddress(ateSrc.IPv6CIDR()).WithDefaultGateway(dutSrc.IPv6)

	port2 := ate.Port(t, "port2")
	ifDut2 := topo.AddInterface(ateDst.Name).WithPort(port2)
	ifDut2.IPv4().WithAddress(ateDst.IPv4CIDR()).WithDefaultGateway(dutDst.IPv4)
	ifDut2.IPv6().WithAddress(ateDst.IPv6CIDR()).WithDefaultGateway(dutDst.IPv6)

	// Setup ATE BGP route v4 advertisement
	bgpDut1 := ifDut1.BGP()
	bgpDut1.AddPeer().WithPeerAddress(dutSrc.IPv4).WithLocalASN(ateAS).
		WithTypeExternal().Capabilities().WithGracefulRestart(true)
	bgpDut1.AddPeer().WithPeerAddress(dutSrc.IPv6).WithLocalASN(ateAS).
		WithTypeExternal().Capabilities().WithGracefulRestart(true)

	bgpDut2 := ifDut2.BGP()
	bgpPeer = bgpDut2.AddPeer().WithPeerAddress(dutDst.IPv4).WithLocalASN(ateAS).
		WithTypeExternal()
	bgpPeer.Capabilities().WithGracefulRestart(true)

	bgpDut2.AddPeer().WithPeerAddress(dutDst.IPv6).WithLocalASN(ateAS).
		WithTypeExternal().Capabilities().WithGracefulRestart(true)

	bgpNeti1 := ifDut1.AddNetwork("bgpNeti1") // ate port1
	bgpNeti1.IPv4().WithAddress(advertisedRoutesv4CIDRp2).WithCount(routeCount)
	bgpNeti1.BGP().WithNextHopAddress(ateSrc.IPv4)

	bgpNeti1v6 := ifDut1.AddNetwork("bgpNeti1v6") // ate port1 v6
	bgpNeti1v6.IPv6().WithAddress(advertisedRoutesv6CIDRp2).WithCount(routeCount)
	bgpNeti1v6.BGP().WithActive(true).WithNextHopAddress(ateSrc.IPv6)

	bgpNeti2 := ifDut2.AddNetwork("bgpNeti2") // ate port2
	bgpNeti2.IPv4().WithAddress(advertisedRoutesv4CIDR).WithCount(routeCount)
	bgpNeti2.BGP().WithNextHopAddress(ateDst.IPv4)

	bgpNeti2v6 := ifDut2.AddNetwork("bgpNeti2v6") // ate port2 v6
	bgpNeti2v6.IPv6().WithAddress(advertisedRoutesv6CIDR).WithCount(routeCount)
	bgpNeti2v6.BGP().WithActive(true).WithNextHopAddress(ateDst.IPv6)

	t.Log("Pushing config to ATE and starting protocols...")
	topo.Push(t).StartProtocols(t)

	// ATE Traffic Configuration
	t.Log("TestBGP:start ate Traffic config")
	ethHeader := ondatra.NewEthernetHeader()
	// BGP V4 Traffic
	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.WithSrcAddress(ipv4SrcTraffic).DstAddressRange().
		WithMin(ipv4DstTrafficStart).WithMax(ipv4DstTrafficEnd).
		WithCount(routeCount)
	flowipv4 := ate.Traffic().NewFlow("Ipv4").
		WithSrcEndpoints(ifDut1).
		WithDstEndpoints(ifDut2).
		WithHeaders(ethHeader, ipv4Header).
		WithFrameSize(512)
	return []*ondatra.Flow{flowipv4}
}

func verifyNoPacketLoss(t *testing.T, ate *ondatra.ATEDevice, allFlows []*ondatra.Flow) {
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
	for _, flow := range allFlows {
		if lossPct := gnmi.Get(t, ate, gnmi.OC().Flow(flow.Name()).LossPct().State()); lossPct > 99.0 {
			t.Logf("Traffic Test Passed! Loss seen as expected: got %v, want 100%% ", lossPct)
		} else {
			t.Errorf("Traffic %s is expected to fail: got %v, want 100%% failure", flow.Name(), lossPct)
		}
	}
}

func captureTrafficStats(t *testing.T, ate *ondatra.ATEDevice) {
	ap := ate.Port(t, "port1")
	aic1 := gnmi.OC().Interface(ap.Name()).Counters()
	sentPkts := gnmi.Get(t, ate, aic1.OutPkts().State())
	fptest.LogQuery(t, "ate:port1 counters", aic1.State(), gnmi.Get(t, ate, aic1.State()))

	op := ate.Port(t, "port2")
	aic2 := gnmi.OC().Interface(op.Name()).Counters()
	rxPkts := gnmi.Get(t, ate, aic2.InPkts().State())
	fptest.LogQuery(t, "ate:port2 counters", aic2.State(), gnmi.Get(t, ate, aic2.State()))
	var lostPkts uint64
	// Account for control plane packets in rxPkts
	if rxPkts > sentPkts {
		lostPkts = rxPkts - sentPkts
	} else {
		lostPkts = sentPkts - rxPkts
	}
	t.Logf("Packets: %d sent, %d received, %d lost", sentPkts, rxPkts, lostPkts)
}

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice, allFlows []*ondatra.Flow, duration time.Duration) {
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

func configACLInterface(t *testing.T, iFace *oc.Acl_Interface, ifName string) *acl.Acl_InterfacePath {
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

// Helper function to replicate configACL() configs in native model
// Define the values for each ACL entry and marshal for json encoding.
// Then craft a gNMI set Request to update the changes.
func configACLNative(t testing.TB, d *ondatra.DUTDevice, name string) {
	t.Helper()
	switch d.Vendor() {
	case ondatra.NOKIA:
		var aclEntry10Val = []any{
			map[string]any{
				"action": map[string]any{
					"drop": map[string]any{},
				},
				"match": map[string]any{
					"destination-ip": map[string]any{
						"prefix": ateDstCIDR,
					},
					"source-ip": map[string]any{
						"prefix": aclNullPrefix,
					},
				},
			},
		}
		entry10Update, err := json.Marshal(aclEntry10Val)
		if err != nil {
			t.Fatalf("Error with json Marshal: %v", err)
		}

		var aclEntry20Val = []any{
			map[string]any{
				"action": map[string]any{
					"drop": map[string]any{},
				},
				"match": map[string]any{
					"source-ip": map[string]any{
						"prefix": ateDstCIDR,
					},
					"destination-ip": map[string]any{
						"prefix": aclNullPrefix,
					},
				},
			},
		}
		entry20Update, err := json.Marshal(aclEntry20Val)
		if err != nil {
			t.Fatalf("Error with json Marshal: %v", err)
		}

		var aclEntry30Val = []any{
			map[string]any{
				"action": map[string]any{
					"accept": map[string]any{},
				},
				"match": map[string]any{
					"source-ip": map[string]any{
						"prefix": aclNullPrefix,
					},
					"destination-ip": map[string]any{
						"prefix": aclNullPrefix,
					},
				},
			},
		}
		entry30Update, err := json.Marshal(aclEntry30Val)
		if err != nil {
			t.Fatalf("Error with json Marshal: %v", err)
		}
		gpbSetRequest := &gpb.SetRequest{
			Prefix: &gpb.Path{
				Origin: "srl",
			},
			Update: []*gpb.Update{
				{
					Path: &gpb.Path{
						Elem: []*gpb.PathElem{
							{Name: "acl"},
							{Name: "ipv4-filter", Key: map[string]string{"name": name}},
							{Name: "entry", Key: map[string]string{"sequence-id": "10"}},
						},
					},
					Val: &gpb.TypedValue{
						Value: &gpb.TypedValue_JsonIetfVal{
							JsonIetfVal: entry10Update,
						},
					},
				},
				{
					Path: &gpb.Path{
						Elem: []*gpb.PathElem{
							{Name: "acl"},
							{Name: "ipv4-filter", Key: map[string]string{"name": name}},
							{Name: "entry", Key: map[string]string{"sequence-id": "20"}},
						},
					},
					Val: &gpb.TypedValue{
						Value: &gpb.TypedValue_JsonIetfVal{
							JsonIetfVal: entry20Update,
						},
					},
				},
				{
					Path: &gpb.Path{
						Elem: []*gpb.PathElem{
							{Name: "acl"},
							{Name: "ipv4-filter", Key: map[string]string{"name": name}},
							{Name: "entry", Key: map[string]string{"sequence-id": "30"}},
						},
					},
					Val: &gpb.TypedValue{
						Value: &gpb.TypedValue_JsonIetfVal{
							JsonIetfVal: entry30Update,
						},
					},
				},
			},
		}
		gnmiClient := d.RawAPIs().GNMI(t)
		if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
			t.Fatalf("Unexpected error configuring SRL ACL: %v", err)
		}
	default:
		t.Fatalf("Unsupported vendor %s for deviation 'UseVendorNativeACLConfiguration'", d.Vendor())
	}
}

// Helper function to replicate AdmitAllACL() configs in native model,
// then craft a gNMI set Request to update the changes.
func configAdmitAllACLNative(t testing.TB, d *ondatra.DUTDevice, name string) {
	t.Helper()
	switch d.Vendor() {
	case ondatra.NOKIA:
		gpbDelRequest := &gpb.SetRequest{
			Prefix: &gpb.Path{
				Origin: "srl",
			},
			Delete: []*gpb.Path{
				{
					Elem: []*gpb.PathElem{
						{Name: "acl"},
						{Name: "ipv4-filter", Key: map[string]string{"name": name}},
						{Name: "entry", Key: map[string]string{"sequence-id": "10"}},
					},
				},
				{
					Elem: []*gpb.PathElem{
						{Name: "acl"},
						{Name: "ipv4-filter", Key: map[string]string{"name": name}},
						{Name: "entry", Key: map[string]string{"sequence-id": "20"}},
					},
				},
			},
		}
		gnmiClient := d.RawAPIs().GNMI(t)
		if _, err := gnmiClient.Set(context.Background(), gpbDelRequest); err != nil {
			t.Fatalf("Unexpected error removing SRL ACL: %v", err)
		}
	default:
		t.Fatalf("Unsupported vendor %s for deviation 'UseVendorNativeACLConfiguration'", d.Vendor())
	}
}

// Helper function to replicate configACLInterface in native model.
// Set ACL at interface ingress,
// then craft a gNMI set Request to update the changes.
func configACLInterfaceNative(t *testing.T, d *ondatra.DUTDevice, ifName string) {
	t.Helper()
	switch d.Vendor() {
	case ondatra.NOKIA:
		var interfaceAclVal = []any{
			map[string]any{
				"ipv4-filter": []any{
					aclName,
				},
			},
		}
		interfaceAclUpdate, err := json.Marshal(interfaceAclVal)
		if err != nil {
			t.Fatalf("Error with json Marshal: %v", err)
		}
		gpbSetRequest := &gpb.SetRequest{
			Prefix: &gpb.Path{
				Origin: "srl",
			},
			Update: []*gpb.Update{
				{
					Path: &gpb.Path{
						Elem: []*gpb.PathElem{
							{Name: "interface", Key: map[string]string{"name": ifName}},
							{Name: "subinterface", Key: map[string]string{"index": "0"}},
							{Name: "acl"},
							{Name: "input"},
						},
					},
					Val: &gpb.TypedValue{
						Value: &gpb.TypedValue_JsonIetfVal{
							JsonIetfVal: interfaceAclUpdate,
						},
					},
				},
			},
		}
		gnmiClient := d.RawAPIs().GNMI(t)
		if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
			t.Fatalf("Unexpected error configuring interface ACL: %v", err)
		}
	default:
		t.Fatalf("Unsupported vendor %s for deviation 'UseVendorNativeACLConfiguration'", d.Vendor())
	}
}

func TestTrafficWithGracefulRestartSpeaker(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	// Configure interface on the DUT
	t.Run("configureDut", func(t *testing.T) {
		t.Log("Start DUT interface Config")
		configureDUT(t, dut)
		configureRoutePolicy(t, dut, "ALLOW", oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	})

	// Configure BGP+Neighbors on the DUT
	t.Run("configureBGP", func(t *testing.T) {
		t.Log("Configure BGP with Graceful Restart option under Global Bgp")
		dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
		nbrList := buildNbrList(ateAS)
		dutConf := bgpWithNbr(dutAS, nbrList, dut)
		gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
		fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))
	})
	// ATE Configuration.
	var allFlows []*ondatra.Flow
	t.Run("configureATE", func(t *testing.T) {
		t.Log("Start ATE Config")
		allFlows = configureATE(t, ate)
	})
	// Verify Port Status
	t.Run("verifyDUTPorts", func(t *testing.T) {
		t.Log("Verifying port status")
		verifyPortsUp(t, dut.Device)
	})
	t.Run("VerifyBGPParameters", func(t *testing.T) {
		t.Log("Check BGP parameters")
		checkBgpStatus(t, dut)
	})
	// Starting ATE Traffic
	t.Run("VerifyTrafficPassBeforeAcLBlock", func(t *testing.T) {
		t.Log("Send Traffic with GR timer enabled. Traffic should pass")
		sendTraffic(t, ate, allFlows, trafficDuration)
		verifyNoPacketLoss(t, ate, allFlows)
	})
	// Configure an ACL to block BGP
	d := &oc.Root{}
	ifName := dut.Port(t, "port2").Name()
	iFace := d.GetOrCreateAcl().GetOrCreateInterface(ifName)
	t.Run("VerifyTrafficPasswithGRTimerWithAclApplied", func(t *testing.T) {
		t.Log("Configure Acl to block BGP on port 179")
		const stopDuration = 45 * time.Second
		t.Log("Starting traffic")
		ate.Traffic().Start(t, allFlows...)
		startTime := time.Now()
		t.Log("Trigger Graceful Restart on ATE")
		ate.Actions().NewBGPGracefulRestart().WithRestartTime(grRestartTime * time.Second).WithPeers(bgpPeer).Send(t)
		if deviations.UseVendorNativeACLConfig(dut) {
			configACLNative(t, dut, aclName)
			configACLInterfaceNative(t, dut, ifName)
		} else {
			gnmi.Replace(t, dut, gnmi.OC().Acl().AclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4).Config(), configACL(d, aclName))
			aclConf := configACLInterface(t, iFace, ifName)
			gnmi.Replace(t, dut, aclConf.Config(), iFace)
		}
		replaceDuration := time.Since(startTime)
		time.Sleep(grTimer - stopDuration - replaceDuration)
		t.Log("Send Traffic while GR timer counting down. Traffic should pass as BGP GR is enabled!")
		ate.Traffic().Stop(t)
		t.Log("Traffic stopped")
		verifyNoPacketLoss(t, ate, allFlows)
	})

	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateDst.IPv4)
	t.Run("VerifyBGPNOTEstablished", func(t *testing.T) {
		t.Log("Waiting for BGP neighbor to Not be in Established state after applying ACL DENY policy..")
		_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			currState, ok := val.Val()
			return ok && currState != oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
			t.Errorf("BGP session did not go Down as expected")
		}
	})

	t.Log("Wait till LLGR/Stale timer expires to delete long live routes.....")
	time.Sleep(time.Second * grRestartTime)
	time.Sleep(time.Second * grStaleRouteTime)

	t.Run("VerifyTrafficFailureAfterGRexpired", func(t *testing.T) {
		t.Log("Send Traffic Again after GR timer has expired. This traffic should fail!")
		sendTraffic(t, ate, allFlows, trafficDuration)
		confirmPacketLoss(t, ate, allFlows)
	})

	t.Run("RemoveAclInterface", func(t *testing.T) {
		t.Log("Removing Acl on the interface to restore BGP GR. Traffic should now pass!")
		if deviations.UseVendorNativeACLConfig(dut) {
			configAdmitAllACLNative(t, dut, aclName)
			configACLInterfaceNative(t, dut, ifName)
		} else {
			gnmi.Replace(t, dut, gnmi.OC().Acl().AclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4).Config(), configAdmitAllACL(d, aclName))
			aclPath := configACLInterface(t, iFace, ifName)
			gnmi.Replace(t, dut, aclPath.Config(), iFace)
		}
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
