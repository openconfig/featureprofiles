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
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/config/acl"
	"github.com/openconfig/ondatra/telemetry"
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
	peerGrpName              = "BGP-PEER-GROUP"
	ateDstCIDR               = "192.0.2.6/32"
)

var (
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

// configureDUT configures all the interfaces and network instance on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dc := dut.Config()
	i1 := dutSrc.NewInterface(dut.Port(t, "port1").Name())
	dc.Interface(i1.GetName()).Replace(t, i1)

	i2 := dutDst.NewInterface(dut.Port(t, "port2").Name())
	dc.Interface(i2.GetName()).Replace(t, i2)

	t.Log("Configure/update Network Instance")
	dutConfNIPath := dc.NetworkInstance(*deviations.DefaultNetworkInstance)
	dutConfNIPath.Type().Replace(t, telemetry.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
}

func verifyPortsUp(t *testing.T, dev *ondatra.Device) {
	t.Helper()
	for _, p := range dev.Ports() {
		status := dev.Telemetry().Interface(p.Name()).OperStatus().Get(t)
		if want := telemetry.Interface_OperStatus_UP; status != want {
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

func bgpWithNbr(as uint32, nbrs []*bgpNeighbor) *telemetry.NetworkInstance_Protocol_Bgp {
	bgp := &telemetry.NetworkInstance_Protocol_Bgp{}
	g := bgp.GetOrCreateGlobal()
	g.As = ygot.Uint32(as)
	g.RouterId = ygot.String(dutDst.IPv4)
	bgpgr := g.GetOrCreateGracefulRestart()
	bgpgr.Enabled = ygot.Bool(true)
	bgpgr.RestartTime = ygot.Uint16(grRestartTime)
	bgpgr.StaleRoutesTime = ygot.Uint16(grStaleRouteTime)

	pg := bgp.GetOrCreatePeerGroup(peerGrpName)
	pg.PeerAs = ygot.Uint32(ateAS)
	pg.PeerGroupName = ygot.String(peerGrpName)

	for _, nbr := range nbrs {
		if nbr.isV4 {
			nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv4.PeerGroup = ygot.String(peerGrpName)
			nv4.GetOrCreateTimers().HoldTime = ygot.Uint16(180)
			nv4.PeerAs = ygot.Uint32(nbr.as)
			nv4.Enabled = ygot.Bool(true)
			nv4.GetOrCreateAfiSafi(telemetry.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
		} else {
			nv6 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv6.PeerGroup = ygot.String(peerGrpName)
			nv6.PeerAs = ygot.Uint32(nbr.as)
			nv6.Enabled = ygot.Bool(true)
			nv6.GetOrCreateAfiSafi(telemetry.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
		}
	}
	return bgp
}

func checkBgpStatus(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Verifying BGP state")
	statePath := dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateSrc.IPv4)
	nbrPathv6 := statePath.Neighbor(ateSrc.IPv6)

	// Get BGP adjacency state
	t.Log("Waiting for BGP neighbor to establish...")
	_, ok := nbrPath.SessionState().Watch(t, time.Minute, func(val *telemetry.QualifiedE_Bgp_Neighbor_SessionState) bool {
		return val.IsPresent() && val.Val(t) == telemetry.Bgp_Neighbor_SessionState_ESTABLISHED
	}).Await(t)
	if !ok {
		fptest.LogYgot(t, "BGP reported state", nbrPath, nbrPath.Get(t))
		t.Fatal("No BGP neighbor formed...")
	}

	// Get BGPv6 adjacency state
	t.Log("Waiting for BGPv6 neighbor to establish...")
	_, ok = nbrPathv6.SessionState().Watch(t, time.Minute, func(val *telemetry.QualifiedE_Bgp_Neighbor_SessionState) bool {
		return val.IsPresent() && val.Val(t) == telemetry.Bgp_Neighbor_SessionState_ESTABLISHED
	}).Await(t)
	if !ok {
		fptest.LogYgot(t, "BGPv6 reported state", nbrPathv6, nbrPathv6.Get(t))
		t.Fatal("No BGPv6 neighbor formed...")
	}

	isGrEnabled := statePath.Global().GracefulRestart().Enabled().Get(t)
	t.Logf("isGrEnabled %v", isGrEnabled)
	if isGrEnabled {
		t.Logf("Graceful restart on neighbor %v enabled as Expected", ateDst.IPv4)
	} else {
		t.Errorf("Expected Graceful restart status on neighbor: got %v, want Enabled", isGrEnabled)
	}

	grTimerVal := statePath.Global().GracefulRestart().RestartTime().Get(t)
	t.Logf("grTimerVal %v", grTimerVal)
	if grTimerVal == uint16(grRestartTime) {
		t.Logf("Graceful restart timer enabled as expected to be %v", grRestartTime)
	} else {
		t.Errorf("Expected Graceful restart timer: got %v, want %v", grTimerVal, grRestartTime)
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
	bgpDut2.AddPeer().WithPeerAddress(dutDst.IPv4).WithLocalASN(ateAS).
		WithTypeExternal().Capabilities().WithGracefulRestart(true)
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
		if lossPct := ate.Telemetry().Flow(flow.Name()).LossPct().Get(t); lossPct < 5.0 {
			t.Logf("Traffic Test Passed! Got %v loss", lossPct)
		} else {
			t.Errorf("Traffic Loss Pct for Flow %s: got %v", flow.Name(), lossPct)
		}
	}
}

func confirmPacketLoss(t *testing.T, ate *ondatra.ATEDevice, allFlows []*ondatra.Flow) {
	for _, flow := range allFlows {
		if lossPct := ate.Telemetry().Flow(flow.Name()).LossPct().Get(t); lossPct > 99.0 {
			t.Logf("Traffic Test Passed! Loss seen as expected: got %v, want 100%% ", lossPct)
		} else {
			t.Errorf("Traffic %s is expected to fail: got %v, want 100%% failure", flow.Name(), lossPct)
		}
	}
}

func captureTrafficStats(t *testing.T, ate *ondatra.ATEDevice) {
	ap := ate.Port(t, "port1")
	aic1 := ate.Telemetry().Interface(ap.Name()).Counters()
	sentPkts := aic1.OutPkts().Get(t)
	fptest.LogYgot(t, "ate:port1 counters", aic1, aic1.Get(t))

	op := ate.Port(t, "port2")
	aic2 := ate.Telemetry().Interface(op.Name()).Counters()
	rxPkts := aic2.InPkts().Get(t)
	fptest.LogYgot(t, "ate:port2 counters", aic2, aic2.Get(t))
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

func configACL(d *telemetry.Device, name string) *telemetry.Acl_AclSet {
	acl := d.GetOrCreateAcl().GetOrCreateAclSet(aclName, telemetry.Acl_ACL_TYPE_ACL_IPV4)
	aclEntry10 := acl.GetOrCreateAclEntry(10)
	aclEntry10.SequenceId = ygot.Uint32(10)
	aclEntry10.GetOrCreateActions().ForwardingAction = telemetry.Acl_FORWARDING_ACTION_DROP
	a := aclEntry10.GetOrCreateIpv4()
	a.SourceAddress = ygot.String(aclNullPrefix)
	a.DestinationAddress = ygot.String(ateDstCIDR)

	aclEntry20 := acl.GetOrCreateAclEntry(20)
	aclEntry20.SequenceId = ygot.Uint32(20)
	aclEntry20.GetOrCreateActions().ForwardingAction = telemetry.Acl_FORWARDING_ACTION_DROP
	a2 := aclEntry20.GetOrCreateIpv4()
	a2.SourceAddress = ygot.String(ateDstCIDR)
	a2.DestinationAddress = ygot.String(aclNullPrefix)

	aclEntry30 := acl.GetOrCreateAclEntry(30)
	aclEntry30.SequenceId = ygot.Uint32(30)
	aclEntry30.GetOrCreateActions().ForwardingAction = telemetry.Acl_FORWARDING_ACTION_ACCEPT
	a3 := aclEntry30.GetOrCreateIpv4()
	a3.SourceAddress = ygot.String(aclNullPrefix)
	a3.DestinationAddress = ygot.String(aclNullPrefix)
	return acl
}

func configAdmitAllACL(d *telemetry.Device, name string) *telemetry.Acl_AclSet {
	acl := d.GetOrCreateAcl().GetOrCreateAclSet(aclName, telemetry.Acl_ACL_TYPE_ACL_IPV4)
	acl.DeleteAclEntry(10)
	acl.DeleteAclEntry(20)
	return acl
}

func configACLInterface(t *testing.T, dut *ondatra.DUTDevice, iFace *telemetry.Acl_Interface, ifName string) *acl.Acl_InterfacePath {
	aclConf := dut.Config().Acl().Interface(ifName)
	if ifName != "" {
		iFace.GetOrCreateIngressAclSet(aclName, telemetry.Acl_ACL_TYPE_ACL_IPV4)
		iFace.GetOrCreateInterfaceRef().Interface = ygot.String(ifName)
		iFace.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	} else {
		iFace.GetOrCreateIngressAclSet(aclName, telemetry.Acl_ACL_TYPE_ACL_IPV4)
		iFace.DeleteIngressAclSet(aclName, telemetry.Acl_ACL_TYPE_ACL_IPV4)
	}
	return aclConf
}

func TestTrafficWithGracefulRestartSpeaker(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	// Configure interface on the DUT
	t.Run("configureDut", func(t *testing.T) {
		t.Log("Start DUT interface Config")
		configureDUT(t, dut)
	})

	// Configure BGP+Neighbors on the DUT
	t.Run("configureBGP", func(t *testing.T) {
		t.Log("Configure BGP with Graceful Restart option under Global Bgp")
		dutConfPath := dut.Config().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
		dutConfPath.Delete(t)
		nbrList := buildNbrList(ateAS)
		dutConf := bgpWithNbr(dutAS, nbrList)
		dutConfPath.Replace(t, dutConf)
		fptest.LogYgot(t, "DUT BGP Config", dutConfPath, dutConfPath.Get(t))
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
	d := &telemetry.Device{}
	ifName := dut.Port(t, "port2").Name()
	iFace := d.GetOrCreateAcl().GetOrCreateInterface(ifName)
	t.Run("VerifyTrafficPasswithGRTimerWithAclApplied", func(t *testing.T) {
		t.Log("Configure Acl to block BGP on port 179")
		const stopDuration = 45 * time.Second
		dut.Config().Acl().AclSet(aclName, telemetry.Acl_ACL_TYPE_ACL_IPV4).Replace(t, configACL(d, aclName))
		aclConf := configACLInterface(t, dut, iFace, ifName)
		t.Log("Starting traffic")
		ate.Traffic().Start(t, allFlows...)
		startTime := time.Now()
		aclConf.Replace(t, iFace)
		replaceDuration := time.Since(startTime)
		time.Sleep(grTimer - stopDuration - replaceDuration)
		t.Log("Send Traffic while GR timer counting down. Traffic should pass as BGP GR is enabled!")
		ate.Traffic().Stop(t)
		t.Log("Traffic stopped")
		verifyNoPacketLoss(t, ate, allFlows)
	})

	statePath := dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateDst.IPv4)
	t.Run("VerifyBGPNOTEstablished", func(t *testing.T) {
		t.Logf("Waiting for BGP neighbor to establish...")
		_, ok := nbrPath.SessionState().Watch(t, 2*time.Minute, func(val *telemetry.QualifiedE_Bgp_Neighbor_SessionState) bool {
			return val.IsPresent() && val.Val(t) == telemetry.Bgp_Neighbor_SessionState_CONNECT
		}).Await(t)
		if !ok {
			fptest.LogYgot(t, "BGP reported state", nbrPath, nbrPath.Get(t))
			t.Errorf("BGP session did not go Down as expected")
		}
	})

	t.Run("VerifyTrafficFailureAfterGRexpired", func(t *testing.T) {
		t.Log("Send Traffic Again after GR timer has expired. This traffic should fail!")
		sendTraffic(t, ate, allFlows, trafficDuration)
		confirmPacketLoss(t, ate, allFlows)
	})

	t.Run("RemoveAclInterface", func(t *testing.T) {
		t.Log("Removing Acl on the interface to restore BGP GR. Traffic should now pass!")
		dut.Config().Acl().AclSet(aclName, telemetry.Acl_ACL_TYPE_ACL_IPV4).Replace(t, configAdmitAllACL(d, aclName))
		configACLInterface(t, dut, iFace, ifName).Replace(t, iFace)
	})

	t.Run("VerifyBGPEstablished", func(t *testing.T) {
		t.Logf("Waiting for BGP neighbor to establish...")
		_, ok := nbrPath.SessionState().Watch(t, time.Minute, func(val *telemetry.QualifiedE_Bgp_Neighbor_SessionState) bool {
			return val.IsPresent() && val.Val(t) == telemetry.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			fptest.LogYgot(t, "BGP reported state", nbrPath, nbrPath.Get(t))
			t.Errorf("BGP session not Established as expected")
		}
	})

	t.Run("VerifyTrafficPassBGPRestored", func(t *testing.T) {
		status := nbrPath.SessionState().Get(t)
		if want := telemetry.Bgp_Neighbor_SessionState_ESTABLISHED; status != want {
			t.Errorf("Get(BGP peer %s status): got %d, want %d", ateDst.IPv4, status, want)
		}
		sendTraffic(t, ate, allFlows, trafficDuration)
		verifyNoPacketLoss(t, ate, allFlows)
	})
}
