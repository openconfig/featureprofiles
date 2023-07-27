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
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gnps "github.com/openconfig/gnoi/system"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
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
	grStaleRouteTime         = 120
	ipv4SrcTraffic           = "192.0.2.2"
	ipv6SrcTraffic           = "2001:db8::192:0:2:2"
	ipv4DstTrafficStart      = "203.0.113.1"
	ipv4DstTrafficEnd        = "203.0.113.2"
	ipv6DstTrafficStart      = "2001:db8::203:0:113:1"
	ipv6DstTrafficEnd        = "2001:db8::203:0:113:2"
	advertisedRoutesv4Net    = "203.0.113.1"
	advertisedRoutesv6Net    = "2001:db8::203:0:113:1"
	advertisedRoutesv4Net2   = "203.0.113.3"
	advertisedRoutesv6Net2   = "2001:db8::203:0:113:3"
	advertisedRoutesv4Prefix = 32
	advertisedRoutesv6Prefix = 128
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
	dutSrc = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		MAC:     "02:00:01:01:01:01",
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
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::192:0:2:6",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	BGPDaemons = map[ondatra.Vendor]string{
		ondatra.ARISTA:  "Bgp-main",
		ondatra.CISCO:   "emsd",
		ondatra.JUNIPER: "rpd",
		ondatra.NOKIA:   "sr_bgp_mgr",
	}
)

// configureDUT configures all the interfaces and network instance on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dc := gnmi.OC()
	i1 := dutSrc.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	i2 := dutDst.NewOCInterface(dut.Port(t, "port2").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i2.GetName()).Config(), i2)

	t.Log("Configure/update Network Instance")
	dutConfNIPath := dc.NetworkInstance(deviations.DefaultNetworkInstance(dut))
	gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

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
	ni_proto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := ni_proto.GetOrCreateBgp()

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
	return ni_proto
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

func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	config := ate.OTG().NewConfig(t)
	p1 := ate.Port(t, "port1")
	ateSrc.AddToOTG(config, p1, &dutSrc)
	p2 := ate.Port(t, "port2")
	ateDst.AddToOTG(config, p2, &dutDst)
	srcDev := config.Devices().Items()[0]
	srcEth := srcDev.Ethernets().Items()[0]
	srcIpv4 := srcEth.Ipv4Addresses().Items()[0]
	srcIpv6 := srcEth.Ipv6Addresses().Items()[0]
	dstDev := config.Devices().Items()[1]
	dstEth := dstDev.Ethernets().Items()[0]
	dstIpv4 := dstEth.Ipv4Addresses().Items()[0]
	dstIpv6 := dstEth.Ipv6Addresses().Items()[0]

	srcBgp := srcDev.Bgp().SetRouterId(ateSrc.IPv4)
	srcBgp4Peer := srcBgp.Ipv4Interfaces().Add().SetIpv4Name(srcIpv4.Name()).Peers().Add().SetName(ateSrc.Name + ".BGP4.peer")
	srcBgp4Peer.GracefulRestart().SetEnableGr(true).SetRestartTime(grRestartTime).SetStaleTime(grStaleRouteTime).SetEnableLlgr(true)
	srcBgp4Peer.SetPeerAddress(srcIpv4.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	srcBgp6Peer := srcBgp.Ipv6Interfaces().Add().SetIpv6Name(srcIpv6.Name()).Peers().Add().SetName(ateSrc.Name + ".BGP6.peer")
	srcBgp6Peer.GracefulRestart().SetEnableGr(true).SetRestartTime(grRestartTime).SetStaleTime(grStaleRouteTime).SetEnableLlgr(true)
	srcBgp6Peer.SetPeerAddress(srcIpv6.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

	dstBgp := dstDev.Bgp().SetRouterId(dstIpv4.Address())
	dstBgp4Peer := dstBgp.Ipv4Interfaces().Add().SetIpv4Name(dstIpv4.Name()).Peers().Add().SetName(ateDst.Name + ".BGP4.peer")
	dstBgp4Peer.GracefulRestart().SetEnableGr(true).SetRestartTime(grRestartTime).SetStaleTime(grStaleRouteTime).SetEnableLlgr(true)
	dstBgp4Peer.SetPeerAddress(dstIpv4.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	dstBgp6Peer := dstBgp.Ipv6Interfaces().Add().SetIpv6Name(dstIpv6.Name()).Peers().Add().SetName(ateDst.Name + ".BGP6.peer")
	dstBgp6Peer.GracefulRestart().SetEnableGr(true).SetRestartTime(grRestartTime).SetStaleTime(grStaleRouteTime).SetEnableLlgr(true)
	dstBgp6Peer.SetPeerAddress(dstIpv6.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

	srcBgp4PeerRoutes := srcBgp4Peer.V4Routes().Add().SetName("bgpNeti1")
	srcBgp4PeerRoutes.SetNextHopIpv4Address(srcIpv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	srcBgp4PeerRoutes.Addresses().Add().SetAddress(advertisedRoutesv4Net2).SetPrefix(advertisedRoutesv4Prefix).SetCount(routeCount)
	srcBgp6PeerRoutes := srcBgp6Peer.V6Routes().Add().SetName("bgpNeti1v6")
	srcBgp6PeerRoutes.SetNextHopIpv6Address(srcIpv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	srcBgp6PeerRoutes.Addresses().Add().SetAddress(advertisedRoutesv6Net2).SetPrefix(advertisedRoutesv6Prefix).SetCount(routeCount)

	dstBgp4PeerRoutes := dstBgp4Peer.V4Routes().Add().SetName("bgpNeti2")
	dstBgp4PeerRoutes.SetNextHopIpv4Address(dstIpv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	dstBgp4PeerRoutes.Addresses().Add().SetAddress(advertisedRoutesv4Net).SetPrefix(advertisedRoutesv4Prefix).SetCount(routeCount)
	dstBgp6PeerRoutes := dstBgp6Peer.V6Routes().Add().SetName("bgpNeti2v6")
	dstBgp6PeerRoutes.SetNextHopIpv6Address(dstIpv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	dstBgp6PeerRoutes.Addresses().Add().SetAddress(advertisedRoutesv6Net).SetPrefix(advertisedRoutesv6Prefix).SetCount(routeCount)

	// ATE Traffic Configuration
	t.Logf("start ate Traffic config")
	flowipv4 := config.Flows().Add().SetName("Ipv4")
	flowipv4.Metrics().SetEnable(true)
	flowipv4.TxRx().Device().
		SetTxNames([]string{srcIpv4.Name()}).
		SetRxNames([]string{dstIpv4.Name()})
	flowipv4.Size().SetFixed(512)
	flowipv4.Rate().SetPps(100)
	flowipv4.Duration().SetChoice("continuous")
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(srcEth.Mac())
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(ipv4SrcTraffic)
	v4.Dst().Increment().SetStart(ipv4DstTrafficStart).SetCount(routeCount)

	ate.OTG().PushConfig(t, config)
	ate.OTG().StartProtocols(t)
	t.Log("Pushing config to ATE and starting protocols...")

	return config
}

func verifyNoPacketLoss(t *testing.T, ate *ondatra.ATEDevice, c gosnappi.Config) {
	otg := ate.OTG()
	otgutils.LogFlowMetrics(t, otg, c)
	for _, f := range c.Flows().Items() {
		t.Logf("Verifying flow metrics for flow %s\n", f.Name())
		recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(f.Name()).State())
		txPackets := float32(recvMetric.GetCounters().GetOutPkts())
		rxPackets := float32(recvMetric.GetCounters().GetInPkts())
		lostPackets := txPackets - rxPackets
		if txPackets == 0 {
			t.Fatalf("Tx packets should be higher than 0 for flow %s", f.Name())
		}
		if lossPct := lostPackets * 100 / txPackets; lossPct < 5.0 {
			t.Logf("Traffic Test Passed! Got %v loss", lossPct)
		} else {
			t.Errorf("Traffic Loss Pct for Flow %s: got %f", f.Name(), lossPct)
		}
	}
}

func confirmPacketLoss(t *testing.T, ate *ondatra.ATEDevice, c gosnappi.Config) {
	otg := ate.OTG()
	otgutils.LogFlowMetrics(t, otg, c)
	for _, f := range c.Flows().Items() {
		t.Logf("Verifying flow metrics for flow %s\n", f.Name())
		recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(f.Name()).State())
		txPackets := float32(recvMetric.GetCounters().GetOutPkts())
		rxPackets := float32(recvMetric.GetCounters().GetInPkts())
		lostPackets := txPackets - rxPackets
		if txPackets == 0 {
			t.Fatalf("Tx packets should be higher than 0 for flow %s", f.Name())
		}
		if lossPct := lostPackets * 100 / txPackets; lossPct > 99.0 {
			t.Logf("Traffic Test Passed! Loss seen as expected: got %v, want 100%% ", lossPct)
		} else {
			t.Errorf("Traffic %s is expected to fail: got %f, want 100%% failure", f.Name(), lossPct)
		}
	}
}

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice, c gosnappi.Config) {
	t.Helper()
	t.Logf("Starting traffic")
	ate.OTG().StartTraffic(t)
	time.Sleep(trafficDuration)
	t.Logf("Stop traffic")
	ate.OTG().StopTraffic(t)
}

// createGracefulRestartAction create a bgp control action for initiating the graceful restart process
func createGracefulRestartAction(t *testing.T, peerNames []string, restartDelay uint32) gosnappi.ControlAction {
	t.Helper()
	grAction := gosnappi.NewControlAction().SetChoice(gosnappi.ControlActionChoice.PROTOCOL)
	grAction.Protocol().SetChoice(gosnappi.ActionProtocolChoice.BGP).Bgp().
		SetChoice(gosnappi.ActionProtocolBgpChoice.INITIATE_GRACEFUL_RESTART).InitiateGracefulRestart().
		SetPeerNames(peerNames).SetRestartDelay(restartDelay)
	return grAction
}

// findProcessByName uses telemetry to find out the PID of a process
func findProcessByName(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, pName string) uint64 {
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
func gNOIKillProcess(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, pName string, pID uint32) {
	gnoiClient := dut.RawAPIs().GNOI().Default(t)
	killRequest := &gnps.KillProcessRequest{Name: pName, Pid: pID, Signal: gnps.KillProcessRequest_SIGNAL_TERM, Restart: true}
	killResponse, err := gnoiClient.System().KillProcess(context.Background(), killRequest)
	t.Logf("Got kill process response: %v\n\n", killResponse)
	if err != nil {
		t.Fatalf("Failed to execute gNOI Kill Process, error received: %v", err)
	}
}

func TestTrafficWithGracefulRestartReceiver(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	// Configure interface on the DUT
	t.Log("Start DUT interface Config")
	configureDUT(t, dut)

	// Configure BGP+Neighbors on the DUT
	t.Log("Configure BGP with Graceful Restart option under Global Bgp")
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	nbrList := buildNbrList(ateAS)
	dutConf := bgpWithNbr(dutAS, nbrList, dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
	fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.GetConfig(t, dut, dutConfPath.Config()))

	// ATE Configuration.
	t.Log("Start ATE Config")
	config := configureATE(t, ate)

	// Verify Port Status
	t.Run("verifyDUTPorts", func(t *testing.T) {
		t.Log("Verifying port status")
		verifyPortsUp(t, dut.Device)
	})
	t.Run("VerifyBGPParameters", func(t *testing.T) {
		t.Log("Check BGP parameters")
		checkBgpStatus(t, dut)
	})

	var startTime time.Time
	// Send Graceful Restart Trigger from ATE to DUT within the GR timer configured on the DUT
	t.Run("InitiateGracefulRestartWithinDUTGRTimer", func(t *testing.T) {
		// starting traffic before graceful restart
		t.Log("Starting traffic before Graceful restrat Trigger from OTG")
		ate.OTG().StartTraffic(t)
		startTime = time.Now()
		t.Log("Send Traffic while GR timer counting down. Traffic should pass as BGP GR is enabled!")
		t.Log("Send Graceful Restart Trigger from OTG to DUT -- trigger GR Restart timer(60s) < configured GR restart Timer(120s)")
		ate.OTG().SetControlAction(t, createGracefulRestartAction(t, []string{"atedst.BGP4.peer"}, 60))
	})
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateDst.IPv4)

	t.Run("VerifyBGPNOTEstablished", func(t *testing.T) {
		t.Log("Waiting for BGP neighbor to go to ACTIVE state after GR Trigger to DUT...")
		bgpState := oc.Bgp_Neighbor_SessionState_ACTIVE
		_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			currState, ok := val.Val()
			return ok && currState == bgpState
		}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
			t.Errorf("BGP session did not go Down as expected")
		}
	})

	const stopDuration = 45 * time.Second
	replaceDuration := time.Since(startTime)
	time.Sleep(grTimer - stopDuration - replaceDuration)
	ate.OTG().StopTraffic(t)
	t.Log("Traffic stopped")
	verifyNoPacketLoss(t, ate, config)

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

	// Send Graceful Restart Trigger from ATE to DUT after the expiration of the GR timer configured on the DUT
	t.Run("InitiateGracefulRestartExceedingDUTGRTimer", func(t *testing.T) {
		t.Log("Send Graceful Restart Trigger from OTG to DUT -- Trigger GR Restart timer(140s) > configured GR Stale Time (120s) ")
		ate.OTG().SetControlAction(t, createGracefulRestartAction(t, []string{"atedst.BGP4.peer"}, grStaleRouteTime+20))
		time.Sleep(time.Second * grStaleRouteTime)
		ate.OTG().StartTraffic(t)
	})
	t.Run("VerifyBGPActiveState", func(t *testing.T) {
		t.Log("Waiting for BGP neighbor to go to ACTIVE state after GR Trigger to DUT...")
		bgpState := oc.Bgp_Neighbor_SessionState_ACTIVE
		_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			currState, ok := val.Val()
			return ok && currState == bgpState
		}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
			t.Errorf("BGP session did not go Down as expected")
		}
	})

	time.Sleep(10 * time.Second)
	t.Run("VerifyTrafficFailureAfterGRexpired", func(t *testing.T) {
		t.Log("Send Traffic Again after GR timer has expired. This traffic should fail!")
		ate.OTG().StopTraffic(t)
		confirmPacketLoss(t, ate, config)
	})
}

func TestTrafficWithGracefulRestartSpeaker(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	ctx := context.Background()

	// Configure interface on the DUT
	t.Log("Start DUT interface Config")
	configureDUT(t, dut)

	// Configure BGP+Neighbors on the DUT
	t.Log("Configure BGP with Graceful Restart option under Global Bgp")
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	nbrList := buildNbrList(ateAS)
	dutConf := bgpWithNbr(dutAS, nbrList, dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
	fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.GetConfig(t, dut, dutConfPath.Config()))

	// ATE Configuration.
	t.Log("Start ATE Config")
	config := configureATE(t, ate)

	// Verify Port Status
	t.Run("verifyDUTPorts", func(t *testing.T) {
		t.Log("Verifying port status")
		verifyPortsUp(t, dut.Device)
	})
	t.Run("VerifyBGPParameters", func(t *testing.T) {
		t.Log("Check BGP parameters")
		checkBgpStatus(t, dut)
	})

	// Starting traffic before graceful restart from DUT
	t.Log("Starting traffic before Graceful restart trigger from DUT")
	ate.OTG().StartTraffic(t)
	startTime := time.Now()
	t.Log("Send Traffic while GR timer counting down. Traffic should pass as BGP GR is enabled!")

	// Send GNOI Graceful Restart Trigger from DUT to ATE
	// Find the PID of gRIBI Daemon.
	var pId uint64
	pName := BGPDaemons[dut.Vendor()]

	t.Run("FindGRIBIDaemonPid", func(t *testing.T) {
		pId = findProcessByName(ctx, t, dut, pName)
		if pId == 0 {
			t.Fatalf("Couldn't find pid of gRIBI daemon '%s'", pName)
		} else {
			t.Logf("Pid of gRIBI daemon '%s' is '%d'", pName, pId)
		}
	})

	// Kill gRIBI daemon through gNOI Kill Request.
	t.Run("ExecuteGnoiKill", func(t *testing.T) {
		// TODO - pid type is uint64 in oc-system model, but uint32 in gNOI Kill Request proto.
		// Until the models are brought in line, typecasting the uint64 to uint32.
		gNOIKillProcess(ctx, t, dut, pName, uint32(pId))
		// Wait for a bit for gRIBI daemon on the DUT to restart.
	})

	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateDst.IPv4)

	t.Run("VerifyBGPNOTEstablished", func(t *testing.T) {
		t.Log("Waiting for BGP neighbor to go to ACTIVE state after GR Trigger to DUT...")
		bgpState := oc.Bgp_Neighbor_SessionState_ACTIVE
		_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			currState, ok := val.Val()
			return ok && currState == bgpState
		}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
			t.Errorf("BGP session did not go Down as expected")
		}
	})

	const stopDuration = 45 * time.Second
	replaceDuration := time.Since(startTime)
	time.Sleep(grTimer - stopDuration - replaceDuration)
	ate.OTG().StopTraffic(t)
	t.Log("Traffic stopped")
	verifyNoPacketLoss(t, ate, config)

	t.Run("VerifyBGPNOTEstablished", func(t *testing.T) {
		t.Log("Waiting for BGP neighbor to go to ACTIVE state after initiating the restart")
		bgpState := oc.Bgp_Neighbor_SessionState_ACTIVE
		_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			currState, ok := val.Val()
			return ok && currState == bgpState
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
		sendTraffic(t, ate, config)
		confirmPacketLoss(t, ate, config)
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
		sendTraffic(t, ate, config)
		verifyNoPacketLoss(t, ate, config)
	})

}
