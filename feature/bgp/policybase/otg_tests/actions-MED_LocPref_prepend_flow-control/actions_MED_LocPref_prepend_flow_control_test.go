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

package actions_MED_LocPref_prepend_flow_control

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"google3/third_party/golang/ygot/ygot/ygot"
	"google3/third_party/open_traffic_generator/gosnappi/gosnappi"
	"google3/third_party/openconfig/featureprofiles/internal/attrs/attrs"
	"google3/third_party/openconfig/featureprofiles/internal/deviations/deviations"
	"google3/third_party/openconfig/featureprofiles/internal/fptest/fptest"
	"google3/third_party/openconfig/featureprofiles/internal/otgutils/otgutils"
	"google3/third_party/openconfig/ondatra/gnmi/gnmi"
	"google3/third_party/openconfig/ondatra/gnmi/oc/oc"
	otgtelemetry "google3/third_party/openconfig/ondatra/gnmi/otg/otg"
	"google3/third_party/openconfig/ondatra/ondatra"
	otg "google3/third_party/openconfig/ondatra/otg/otg"
	"google3/third_party/openconfig/ygnmi/ygnmi/ygnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// The testbed consists of ate:port1 -> dut:port1 and
// dut:port2 -> ate:port2.  The first pair is called the "source"
// pair, and the second the "destination" pair.
//
// Source: ate:port1 -> dut:port1 subnet 192.0.2.0/30 2001:db8::0/126
// Destination: dut:port2 -> ate:port2 subnet 192.0.2.4/30 2001:db8::4/126
//
// Note that the first (.0, .3) and last (.4, .7) IPv4 addresses are
// reserved from the subnet for broadcast, so a /30 leaves exactly 2
// usable addresses. This does not apply to IPv6 which allows /127
// for point to point links, but we use /126 so the numbering is
// consistent with IPv4.
//
// A traffic flow is configured from ate:port1 as the source interface
// and ate:port2 as the destination interface. Then 255 BGP routes 203.0.113.[1-254]/32
// are adverstised from port2 and traffic is sent originating from port1 to all
// these advertised routes. The traffic will pass only if the DUT installs the
// prefixes successfully in the routing table via BGP.Successful transmission of
// traffic will ensure BGP routes are properly installed on the DUT and programmed.
// Similarly, Traffic is sent for IPv6 destinations.

const (
	advertisedRoutesv4Net1      = "192.168.10.0"
	advertisedRoutesv6Net1      = "2024:db8:128:128::"
	advertisedRoutesv4Net2      = "192.168.20.0"
	advertisedRoutesv6Net2      = "2024:db8:64:64::"
	advertisedRoutesv4PrefixLen = 24
	advertisedRoutesv6PrefixLen = 64
	peerGrpNamev4               = "BGP-PEER-GROUP-V4"
	peerGrpNamev6               = "BGP-PEER-GROUP-V6"
	dutAS                       = 64500
	ateAS                       = 64501
	plenIPv4                    = 30
	plenIPv6                    = 126
	setLocalPrefPolicy          = "lp-policy"
	initialLocalPrefValue       = 50
	initialMEDValue             = 50
	setMEDPolicy                = "med-policy"
	matchStatement1             = "match-statement-1"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort1 = attrs.Attributes{
		Name:    "ateSrc",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "DUT to ATE destination",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::192:0:2:5",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	atePort2 = attrs.Attributes{
		Name:    "atedst",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::192:0:2:6",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
)

// configureDUT configures all the interfaces on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dc := gnmi.OC()
	i1 := dutPort1.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	i2 := dutPort2.NewOCInterface(dut.Port(t, "port2").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i2.GetName()).Config(), i2)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
		fptest.SetPortSpeed(t, dut.Port(t, "port2"))
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, i1.GetName(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, i2.GetName(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

// verifyPortsUp asserts that each port on the device is operating.
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
	peerGrp    string
}

// bgpCreateNbr creates a BGP object with neighbors pointing to ateSrc and ateDst, optionally with
// a peer group policy.
func bgpCreateNbr(localAs, peerAs uint32, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	nbr1v4 := &bgpNeighbor{as: dutAS, neighborip: atePort1.IPv4, isV4: true, peerGrp: peerGrpName1}
	nbr2v4 := &bgpNeighbor{as: ateAS, neighborip: atePort2.IPv4, isV4: true, peerGrp: peerGrpName2}
	nbr1v6 := &bgpNeighbor{as: dutAS, neighborip: atePort1.IPv6, isV4: false, peerGrp: peerGrpName3}
	nbr2v6 := &bgpNeighbor{as: ateAS, neighborip: atePort2.IPv6, isV4: false, peerGrp: peerGrpName4}
	nbrs := []*bgpNeighbor{nbr1v4, nbr2v4, nbr1v6, nbr2v6}

	dutOcRoot := &oc.Root{}
	ni1 := dutOcRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.RouterId = ygot.String(dutlo0Attrs.IPv4)
	global.As = ygot.Uint32(localAs)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	pg1 := bgp.GetOrCreatePeerGroup(peerGrpName1)
	pg1.PeerAs = ygot.Uint32(ateAS)
	pg1.PeerGroupName = ygot.String(peerGrpName1)

	pg2 := bgp.GetOrCreatePeerGroup(peerGrpName2)
	pg2.PeerAs = ygot.Uint32(dutAS)
	pg2.PeerGroupName = ygot.String(peerGrpName2)

	pg3 := bgp.GetOrCreatePeerGroup(peerGrpName3)
	pg3.PeerAs = ygot.Uint32(ateAS)
	pg3.PeerGroupName = ygot.String(peerGrpName3)

	pg4 := bgp.GetOrCreatePeerGroup(peerGrpName4)
	pg4.PeerAs = ygot.Uint32(dutAS)
	pg4.PeerGroupName = ygot.String(peerGrpName4)

	for _, nbr := range nbrs {
		bgpNbr := bgp.GetOrCreateNeighbor(nbr.neighborip)
		bgpNbr.PeerGroup = ygot.String(nbr.peerGrp)
		bgpNbr.PeerAs = ygot.Uint32(nbr.as)
		bgpNbr.Enabled = ygot.Bool(true)
		if nbr.localAddress != "" {
			bgpNbrT := bgpNbr.GetOrCreateTransport()
			bgpNbrT.LocalAddress = ygot.String(nbr.localAddress)
		}
		if nbr.isV4 == true {
			af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(true)
			af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(false)
		} else {
			af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(false)
			af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(true)
		}
	}
	return niProto
}

func verifyBgpTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	var nbrIP = []string{atePort1.IPv4, atePort2.IPv4}
	t.Logf("Verifying BGP state.")
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	for _, nbr := range nbrIP {
		nbrPath := bgpPath.Neighbor(nbr)
		// Get BGP adjacency state.
		t.Logf("Waiting for BGP neighbor to establish...")
		var status *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]
		status, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			state, ok := val.Val()
			return ok && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
			t.Fatal("No BGP neighbor formed")
		}
		state, _ := status.Val()
		t.Logf("BGP adjacency for %s: %v", nbr, state)
		if want := oc.Bgp_Neighbor_SessionState_ESTABLISHED; state != want {
			t.Errorf("BGP peer %s status got %d, want %d", nbr, state, want)
		}
	}
}

func configureMEDLocalPrefPolicy(t *testing.T, dut *ondatra.DUTDevice, policyType, policyValue, statement string) {
	t.Helper()
	batchConfig := &gnmi.SetBatch{}
	rp := d.GetOrCreateRoutingPolicy()
	pdef := rp.GetOrCreatePolicyDefinition(policyType)
	stmt, err = pdef.AppendNewStatement(statement)
	if err != nil {
		return nil, err
	}
	actions := stmt.GetOrCreateActions()
	switch policyType {
	case setLocalPrefPolicy:
		actions.GetOrCreateBgpActions().SetLocalPref = ygot.Uint32(policyValue)
		actions.PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	case setMEDPolicy:
		actions.GetOrCreateBgpActions().SetMed = oc.UnionUint32(policyValue)
		actions.PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	default:
		rp := nil
	}
	gnmi.BatchReplace(batchConfig, gnmi.OC().RoutingPolicy().Config(), rp)
}

func configureBGPDefaultImportExportPolicy(t *testing.T, dut *ondatra.DUTDevice, ipv4, ipv6 string, polType oc.E_RoutingPolicy_DefaultPolicyType) {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	batchConfig := &gnmi.SetBatch{}
	nbrPolPathv4 := bgpPath.Neighbor(ipv4).AfiSafi(oc.E_BgpTypes_AFI_SAFI_TYPE).ApplyPolicy()
	nbrPolPathv6 := bgpPath.Neighbor(ipv6).AfiSafi(oc.E_BgpTypes_AFI_SAFI_TYPE).ApplyPolicy()
	gnmi.BatchReplace(batchConfig, nbrPolPathv4.DefaultImportPolicy().Config(), polType)
	gnmi.BatchReplace(batchConfig, nbrPolPathv4.DefaultExportPolicy().Config(), polType)
	gnmi.BatchReplace(batchConfig, nbrPolPathv6.DefaultImportPolicy().Config(), polType)
	gnmi.BatchReplace(batchConfig, nbrPolPathv6.DefaultExportPolicy().Config(), polType)
	batchConfig.Set(t, dut)
}

func configureBGPImportExportPolicy(t *testing.T, dut *ondatra.DUTDevice, ipv4, ipv6, policyDef string) {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	batchConfig := &gnmi.SetBatch{}
	nbrPolPathv4 := bgpPath.Neighbor(ipv4).AfiSafi(oc.E_BgpTypes_AFI_SAFI_TYPE).ApplyPolicy()
	nbrPolPathv6 := bgpPath.Neighbor(ipv6).AfiSafi(oc.E_BgpTypes_AFI_SAFI_TYPE).ApplyPolicy()
	gnmi.BatchReplace(batchConfig, nbrPolPathv4.ImportPolicy().Config(), []string{policyDef})
	gnmi.BatchReplace(batchConfig, nbrPolPathv4.ExportPolicy().Config(), []string{policyDef})
	gnmi.BatchReplace(batchConfig, nbrPolPathv6.ImportPolicy().Config(), []string{policyDef})
	gnmi.BatchReplace(batchConfig, nbrPolPathv6.ExportPolicy().Config(), []string{policyDef})
	batchConfig.Set(t, dut)
}

func verifyBgpPolicyTelemetry(t *testing.T, dut *ondatra.DUTDevice, ipAddr string, defPol, appliedPol []string, isV4 bool) {
	t.Helper()

	t.Logf("BGP Policy telemetry verification for the neighbor %v", ipAddr)

	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	var afiSafiPath *netinstbgp.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafiPath
	if isV4 {
		afiSafiPath = statePath.Neighbor(ipAddr).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	} else {
		afiSafiPath = statePath.Neighbor(ipAddr).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	}

	peerTel := gnmi.Get(t, dut, afiSafiPath.State())

	if gotDefExPolicy := peerTel.GetApplyPolicy().GetDefaultExportPolicy(); gotDefExPolicy != defPol {
		t.Errorf("Default export policy type mismatch: got %v, want %v", gotDefExPolicy, defPol)
	}

	if gotDefImPolicy := peerTel.GetApplyPolicy().GetDefaultImportPolicy(); gotDefImPolicy != defPol {
		t.Errorf("Default import policy type mismatch: got %v, want %v", gotDefImPolicy, defPol)
	}

	if gotExportPol := peerTel.GetApplyPolicy().GetExportPolicy(); cmp.Diff(gotExportPol, exportPol) != "" {
		t.Errorf("Export policy type mismatch: got %v, want %v", gotExportPol, appliedPol)
	}

	if gotImportPol := peerTel.GetApplyPolicy().GetImportPolicy(); cmp.Diff(gotImportPol, importPol) != "" {
		t.Errorf("Import policy type mismatch: got %v, want %v", gotImportPol, appliedPol)
	}
}

// configureOTG configures the interfaces and BGP protocols on an OTG, including advertising some
// (faked) networks over BGP.
func configureOTG(t *testing.T, otg *otg.OTG) gosnappi.Config {
	config := gosnappi.NewConfig()
	port1 := config.Ports().Add().SetName("port1")
	port2 := config.Ports().Add().SetName("port2")

	iDut1Dev := config.Devices().Add().SetName(atePort1.Name)
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	iDut1Eth.Connection().SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4")
	iDut1Ipv4.SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))
	iDut1Ipv6 := iDut1Eth.Ipv6Addresses().Add().SetName(atePort1.Name + ".IPv6")
	iDut1Ipv6.SetAddress(atePort1.IPv6).SetGateway(dutPort1.IPv6).SetPrefix(uint32(atePort1.IPv6Len))

	iDut2Dev := config.Devices().Add().SetName(atePort2.Name)
	iDut2Eth := iDut2Dev.Ethernets().Add().SetName(atePort2.Name + ".Eth").SetMac(atePort2.MAC)
	iDut2Eth.Connection().SetPortName(port2.Name())
	iDut2Ipv4 := iDut2Eth.Ipv4Addresses().Add().SetName(atePort2.Name + ".IPv4")
	iDut2Ipv4.SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).SetPrefix(uint32(atePort2.IPv4Len))
	iDut2Ipv6 := iDut2Eth.Ipv6Addresses().Add().SetName(atePort2.Name + ".IPv6")
	iDut2Ipv6.SetAddress(atePort2.IPv6).SetGateway(dutPort2.IPv6).SetPrefix(uint32(atePort2.IPv6Len))

	// iBGP v4 and v6 sessions on port1
	iDut1Bgp := iDut1Dev.Bgp().SetRouterId(iDut1Ipv4.Address())
	iDut1Bgp4Peer := iDut1Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut1Ipv4.Name()).Peers().Add().SetName(atePort1.Name + ".BGP4.peer")
	iDut1Bgp4Peer.SetPeerAddress(iDut1Ipv4.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	iDut1Bgp6Peer := iDut1Bgp.Ipv6Interfaces().Add().SetIpv6Name(iDut1Ipv6.Name()).Peers().Add().SetName(atePort1.Name + ".BGP6.peer")
	iDut1Bgp6Peer.SetPeerAddress(iDut1Ipv6.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)

	// eBGP v4 and v6 sessions on port2
	iDut2Bgp := iDut2Dev.Bgp().SetRouterId(iDut2Ipv4.Address())
	iDut2Bgp4Peer := iDut2Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut2Ipv4.Name()).Peers().Add().SetName(atePort2.Name + ".BGP4.peer")
	iDut2Bgp4Peer.SetPeerAddress(dstIpv4.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	iDut2Bgp6Peer := dstBgp.Ipv6Interfaces().Add().SetIpv6Name(iDut2Ipv6.Name()).Peers().Add().SetName(atePort2.Name + ".BGP6.peer")
	iDut2Bgp6Peer.SetPeerAddress(dstIpv6.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

	// iBGP V4 routes from Port1 and set MED, Local Preference.
	bgpNeti1Bgp4PeerRoutes := iDut1Bgp4Peer.V4Routes().Add().SetName(atePort1.Name + ".BGP4.Route")
	bgpNeti1Bgp4PeerRoutes.SetNextHopIpv4Address(iDut1Ipv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	bgpNeti1Bgp4PeerRoutes.Addresses().Add().
		SetAddress(advertisedRoutesv4Net1).SetPrefix(advertisedRoutesv4PrefixLen)
	bgpNeti1Bgp4PeerRoutes.Advanced().SetMultiExitDiscriminator(50)
	bgpNeti1Bgp4PeerRoutes.Advanced().SetLocalPreference(50)

	// iBGP V6 routes from Port1 and set MED, Local Preference.
	bgpNeti1Bgp6PeerRoutes := iDut1Bgp6Peer.V6Routes().Add().SetName(atePort1.Name + ".BGP6.Route")
	bgpNeti1Bgp6PeerRoutes.SetNextHopIpv6Address(iDut1Ipv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	bgpNeti1Bgp6PeerRoutes.Addresses().Add().
		SetAddress(advertisedRoutesv6Net1).SetPrefix(advertisedRoutesv6PrefixLen)
	bgpNeti1Bgp6PeerRoutes.Advanced().SetMultiExitDiscriminator(50)
	bgpNeti1Bgp6PeerRoutes.Advanced().SetLocalPreference(50)

	// eBGP V4 routes from Port2 and set MED, Local Preference.
	bgpNeti2Bgp4PeerRoutes := iDut2Bgp4Peer.V4Routes().Add().SetName(atePort2.Name + ".BGP4.Route")
	bgpNeti2Bgp4PeerRoutes.SetNextHopIpv4Address(iDut2Ipv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	bgpNeti2Bgp4PeerRoutes.Addresses().Add().
		SetAddress(advertisedRoutesv4Net2).SetPrefix(advertisedRoutesv4PrefixLen)
	bgpNeti2Bgp4PeerRoutes.Advanced().SetMultiExitDiscriminator(50)
	bgpNeti2Bgp4PeerRoutes.Advanced().SetLocalPreference(50)

	// eBGP V6 routes from Port2 and set MED, Local Preference.
	bgpNeti2Bgp6PeerRoutes := iDut2Bgp6Peer.V6Routes().Add().SetName(atePort2.Name + ".BGP6.Route")
	bgpNeti2Bgp6PeerRoutes.SetNextHopIpv6Address(iDut2Ipv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	bgpNeti2Bgp6PeerRoutes.Addresses().Add().
		SetAddress(advertisedRoutesv6Net2).SetPrefix(advertisedRoutesv6PrefixLen)
	bgpNeti2Bgp6PeerRoutes.Advanced().SetMultiExitDiscriminator(50)
	bgpNeti2Bgp6PeerRoutes.Advanced().SetLocalPreference(50)

	t.Logf("Pushing config to ATE and starting protocols...")
	otg.PushConfig(t, config)
	otg.StartProtocols(t)
	return config
}

func validateOTGBgpPrefixV4AndMED(t *testing.T, otg *otg.OTG, config gosnappi.Config, peerName, ipAddr string, prefixLen uint32, isMEDCheck bool, med uint32) {
	t.Helper()
	_, ok := gnmi.WatchAll(t,
		otg,
		gnmi.OTG().BgpPeer(peerName).UnicastIpv4PrefixAny().State(),
		time.Minute,
		func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv4Prefix]) bool {
			_, present := v.Val()
			return present
		}).Await(t)

	if ok {
		bgpPrefixes := gnmi.GetAll(t, otg, gnmi.OTG().BgpPeer(peerName).UnicastIpv4PrefixAny().State())
		for _, bgpPrefix := range bgpPrefixes {
			if bgpPrefix.Address != nil && bgpPrefix.GetAddress() == ipAddr &&
				bgpPrefix.PrefixLength != nil && bgpPrefix.GetPrefixLength() == prefixLen {
				t.Logf("Prefix recevied on OTG is correct, got prefix %v, want prefix %v", bgpPrefix, ipAddr)
				if isMEDCheck {
					if bgpPrefix.GetMultiExitDiscriminator() != med {
						t.Errorf("For Prefix %v, got MED %d want MED %d", bgpPrefix.GetAddress(), bgpPrefix.GetMultiExitDiscriminator(), med)
					} else {
						t.Logf("For Prefix %v, got MED %d want MED %d", bgpPrefix.GetAddress(), bgpPrefix.GetMultiExitDiscriminator(), med)
					}
				}
				break
			}
		}
		t.Logf("Prefix %v not received on OTG", ipAddr)
	}
}

func validateOTGBgpPrefixV6AndMED(t *testing.T, otg *otg.OTG, config gosnappi.Config, peerName, ipAddr string, prefixLen uint32, isMEDCheck bool, med uint32) {
	t.Helper()
	_, ok := gnmi.WatchAll(t,
		otg,
		gnmi.OTG().BgpPeer(peerName).UnicastIpv6PrefixAny().State(),
		time.Minute,
		func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv6Prefix]) bool {
			_, present := v.Val()
			return present
		}).Await(t)

	if ok {
		bgpPrefixes := gnmi.GetAll(t, otg, gnmi.OTG().BgpPeer(peerName).UnicastIpv6PrefixAny().State())
		for _, bgpPrefix := range bgpPrefixes {
			if bgpPrefix.Address != nil && bgpPrefix.GetAddress() == ipAddr &&
				bgpPrefix.PrefixLength != nil && bgpPrefix.GetPrefixLength() == prefixLen {
				t.Logf("Prefix recevied on OTG is correct, got prefix %v, want prefix %v", bgpPrefix, ipAddr)
				if isMEDCheck {
					if bgpPrefix.GetMultiExitDiscriminator() != med {
						t.Errorf("For Prefix %v, got MED %d want MED %d", bgpPrefix.GetAddress(), bgpPrefix.GetMultiExitDiscriminator(), med)
					} else {
						t.Logf("For Prefix %v, got MED %d want MED %d", bgpPrefix.GetAddress(), bgpPrefix.GetMultiExitDiscriminator(), med)
					}
				}
				break
			}
		}
		t.Logf("Prefix %v not received on OTG", ipAddr)
	}
}

func TestBGPPolicy(t *testing.T) {
	// DUT configurations.
	t.Run("Configure DUT interfaces", func(t *testing.T) {
		configureDUT(t, dut)
	})

	t.Run("Configure DEFAULT network instance", func(t *testing.T) {
		fptest.ConfigureDefaultNetworkInstance(t, dut)
	})

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	t.Run("Configure BGP Neighbors", func(t *testing.T) {
		gnmi.Delete(t, dut, dutConfPath.Config())
		dutConf := bgpCreateNbr(dutAS, ateAS, dut)
		gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
		fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))
	})

	otg := ate.OTG()
	var otgConfig gosnappi.Config
	t.Run("Configure OTG", func(t *testing.T) {
		otgConfig = configureOTG(t, otg)
	})

	t.Run("Verify port status on DUT", func(t *testing.T) {
		verifyPortsUp(t, dut.Device)
	})

	t.Run("Verify BGP session telemetry", func(t *testing.T) {
		verifyBgpTelemetry(t, dut)
	})

	cases := []struct {
		desc                                                       string
		policyType, policyType2, policyStatement                   string
		defPolicyPort1, defPolicyPort2                             string
		policyValue                                                uint32
		port1v4Prefix, port1v6Prefix, port2v4Prefix, port2v6Prefix string
		isMEDCheck                                                 bool
		med                                                        uint32
	}{{
		desc:            "Configure iBGP MED Import Export Policy",
		policyType:      setMEDPolicy,
		policyValue:     100,
		policyStatement: matchStatement1,
		defPolicyPort1:  "REJECT_ROUTE",
		defPolicyPort2:  "ACCEPT_ROUTE",
		policyType2:     nil,
		port1v4Prefix:   advertisedRoutesv4Net2,
		port1v6Prefix:   advertisedRoutesv6Net2,
		port2v4Prefix:   advertisedRoutesv4Net1,
		port2v6Prefix:   advertisedRoutesv6Net1,
		isMEDCheck:      true,
		med:             100,
	}}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			dut := ondatra.DUT(t, "dut")
			ate := ondatra.ATE(t, "ate")

			// Configure Routing Policy on the DUT.
			configureMEDLocalPrefPolicy(tc.policyType, tc.policyValue, tc.policyStatement)
			// Configure BGP default import export policy
			configureBGPDefaultImportExportPolicy(atePort1.IPv4, atePort1.IPv6, tc.defPolicyPort1)
			configureBGPImportExportPolicy(atePort1.IPv4, atePort1.IPv6, tc.policyType)
			//Verify BGP policy
			verifyBgpPolicyTelemetry(t, dut, atePort1.IPv4, tc.defPolicyPort1, tc.policyType, true)
			verifyBgpPolicyTelemetry(t, dut, atePort1.IPv6, tc.defPolicyPort1, tc.policyType, false)
			verifyBgpPolicyTelemetry(t, dut, atePort2.IPv4, tc.defPolicyPort2, tc.policyType2, true)
			verifyBgpPolicyTelemetry(t, dut, atePort2.IPv6, tc.defPolicyPort2, tc.policyType2, false)
			//Validate Prefixes
			validateOTGBgpPrefixV4AndMED(t, otg, otgConfig, atePort1.Name+".BGP4.Route", tc.port1v4Prefix, advertisedRoutesv4PrefixLen, tc.isMEDCheck, tc.med)
			validateOTGBgpPrefixV6AndMED(t, otg, otgConfig, atePort1.Name+".BGP6.Route", tc.port1v6Prefix, advertisedRoutesv6PrefixLen, tc.isMEDCheck, tc.med)
			validateOTGBgpPrefixV4AndMED(t, otg, otgConfig, atePort2.Name+".BGP4.Route", tc.port2v4Prefix, advertisedRoutesv4PrefixLen, tc.isMEDCheck, tc.med)
			validateOTGBgpPrefixV6AndMED(t, otg, otgConfig, atePort2.Name+".BGP6.Route", tc.port2v6Prefix, advertisedRoutesv6PrefixLen, tc.isMEDCheck, tc.med)
		})
	}
}
