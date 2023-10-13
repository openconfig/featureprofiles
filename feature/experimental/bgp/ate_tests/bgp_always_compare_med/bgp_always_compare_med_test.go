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

package bgp_always_compare_med_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	trafficDuration        = 1 * time.Minute
	ipv4SrcTraffic         = "192.0.2.2"
	advertisedRoutesv4CIDR = "203.0.113.1/32"
	ipv4DstTrafficStart    = "203.0.113.1"
	ipv4DstTrafficEnd      = "203.0.113.254"
	peerGrpName1           = "BGP-PEER-GROUP1"
	peerGrpName2           = "BGP-PEER-GROUP2"
	peerGrpName3           = "BGP-PEER-GROUP3"
	tolerancePct           = 2
	tolerance              = 50
	routeCount             = 254
	dutAS                  = 65501
	ateAS1                 = 65501
	ateAS2                 = 65502
	ateAS3                 = 65503
	plenIPv4               = 30
	plenIPv6               = 126
	setMEDPolicy100        = "SET-MED-100"
	setMEDPolicy50         = "SET-MED-50"
	rplAllowPolicy         = "ALLOW"
	aclStatement20         = "20"
	aclStatement30         = "30"
	bgpMED100              = 100
	bgpMED50               = 50
	wantLoss               = true
	flow1                  = "flowPort1toPort2"
	flow2                  = "flowPort1toPort3"
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
	dutDst1 = attrs.Attributes{
		Desc:    "DUT to ATE destination 1",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::192:0:2:5",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	ateDst1 = attrs.Attributes{
		Name:    "atedst1",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::192:0:2:6",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutDst2 = attrs.Attributes{
		Desc:    "DUT to ATE destination 2",
		IPv4:    "192.0.2.9",
		IPv6:    "2001:db8::192:0:2:9",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	ateDst2 = attrs.Attributes{
		Name:    "atedst2",
		IPv4:    "192.0.2.10",
		IPv6:    "2001:db8::192:0:2:10",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
)

// configureDUT configures all the interfaces on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dc := gnmi.OC()
	i1 := dutSrc.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	i2 := dutDst1.NewOCInterface(dut.Port(t, "port2").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i2.GetName()).Config(), i2)

	i3 := dutDst2.NewOCInterface(dut.Port(t, "port3").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i3.GetName()).Config(), i3)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
		fptest.SetPortSpeed(t, dut.Port(t, "port2"))
		fptest.SetPortSpeed(t, dut.Port(t, "port3"))
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, i1.GetName(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, i2.GetName(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, i3.GetName(), deviations.DefaultNetworkInstance(dut), 0)
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

// bgpCreateNbr creates a BGP object with neighbors pointing to ateSrc and ateDst.
func bgpCreateNbr(localAs, peerAs uint32, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	nbr1v4 := &bgpNeighbor{as: ateAS1, neighborip: ateSrc.IPv4, isV4: true, peerGrp: peerGrpName1}
	nbr2v4 := &bgpNeighbor{as: ateAS2, neighborip: ateDst1.IPv4, isV4: true, peerGrp: peerGrpName2}
	nbr3v4 := &bgpNeighbor{as: ateAS3, neighborip: ateDst2.IPv4, isV4: true, peerGrp: peerGrpName3}
	nbrs := []*bgpNeighbor{nbr1v4, nbr2v4, nbr3v4}

	dutOcRoot := &oc.Root{}
	ni1 := dutOcRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.RouterId = ygot.String(dutDst2.IPv4)
	global.As = ygot.Uint32(localAs)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)

	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	pg1 := bgp.GetOrCreatePeerGroup(peerGrpName1)
	pg1.PeerAs = ygot.Uint32(ateAS1)
	pg1.PeerGroupName = ygot.String(peerGrpName1)

	pg2 := bgp.GetOrCreatePeerGroup(peerGrpName2)
	pg2.PeerAs = ygot.Uint32(ateAS2)
	pg2.PeerGroupName = ygot.String(peerGrpName2)

	pg3 := bgp.GetOrCreatePeerGroup(peerGrpName3)
	pg3.PeerAs = ygot.Uint32(ateAS3)
	pg3.PeerGroupName = ygot.String(peerGrpName3)

	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		rp2 := pg2.GetOrCreateApplyPolicy()
		rp2.SetImportPolicy([]string{rplAllowPolicy})

		rp3 := pg3.GetOrCreateApplyPolicy()
		rp3.SetImportPolicy([]string{rplAllowPolicy})

	} else {

		pg2af4 := pg2.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		pg2af4.Enabled = ygot.Bool(true)

		pg2rpl4 := pg2af4.GetOrCreateApplyPolicy()
		pg2rpl4.SetImportPolicy([]string{rplAllowPolicy})

		pg3af4 := pg3.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		pg3af4.Enabled = ygot.Bool(true)

		pg3rpl4 := pg3af4.GetOrCreateApplyPolicy()
		pg3rpl4.SetImportPolicy([]string{rplAllowPolicy})
	}

	for _, nbr := range nbrs {
		if nbr.isV4 {
			nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv4.PeerGroup = ygot.String(nbr.peerGrp)
			nv4.PeerAs = ygot.Uint32(nbr.as)
			nv4.Enabled = ygot.Bool(true)
			af4 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(true)
			af6 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(false)
		}
	}
	return niProto
}

// verifyBgpTelemetry checks that the dut has an established BGP session with reasonable settings.
func verifyBgpTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	var nbrIP = []string{ateSrc.IPv4, ateDst1.IPv4, ateDst2.IPv4}
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

// configureATE configures the interfaces and BGP protocols on an ATE, including
// advertising some(faked) networks over BGP.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) []*ondatra.Flow {
	topo := ate.Topology().New()

	port1 := ate.Port(t, "port1")
	iDut1 := topo.AddInterface(ateSrc.Name).WithPort(port1)
	iDut1.IPv4().WithAddress(ateSrc.IPv4CIDR()).WithDefaultGateway(dutSrc.IPv4)

	port2 := ate.Port(t, "port2")
	iDut2 := topo.AddInterface(ateDst1.Name).WithPort(port2)
	iDut2.IPv4().WithAddress(ateDst1.IPv4CIDR()).WithDefaultGateway(dutDst1.IPv4)

	port3 := ate.Port(t, "port3")
	iDut3 := topo.AddInterface(ateDst2.Name).WithPort(port3)
	iDut3.IPv4().WithAddress(ateDst2.IPv4CIDR()).WithDefaultGateway(dutDst2.IPv4)

	// Setup ATE BGP route v4 advertisement.
	bgpDut1 := iDut1.BGP()
	bgpDut1.AddPeer().WithPeerAddress(dutSrc.IPv4).WithLocalASN(ateAS1).
		WithTypeInternal()

	bgpDut2 := iDut2.BGP()
	bgpDut2.AddPeer().WithPeerAddress(dutDst1.IPv4).WithLocalASN(ateAS2).
		WithTypeExternal()

	bgpDut3 := iDut3.BGP()
	bgpDut3.AddPeer().WithPeerAddress(dutDst2.IPv4).WithLocalASN(ateAS3).
		WithTypeExternal()

	bgpNeti1 := iDut2.AddNetwork("bgpNeti1") // Advertise same prefixes from both eBGP Peers.
	bgpNeti1.IPv4().WithAddress(advertisedRoutesv4CIDR).WithCount(routeCount)
	bgpNeti1.BGP().WithNextHopAddress(ateDst1.IPv4)

	bgpNeti2 := iDut3.AddNetwork("bgpNeti2") // Advertise same prefixes from both eBGP Peers.
	bgpNeti2.IPv4().WithAddress(advertisedRoutesv4CIDR).WithCount(routeCount)
	bgpNeti2.BGP().WithNextHopAddress(ateDst2.IPv4)

	t.Logf("Pushing config to ATE and starting protocols...")
	topo.Push(t)
	topo.StartProtocols(t)

	// ATE Traffic Configuration.
	ethHeader := ondatra.NewEthernetHeader()
	// BGP V4 Traffic.
	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.WithSrcAddress(ipv4SrcTraffic).DstAddressRange().
		WithMin(ipv4DstTrafficStart).WithMax(ipv4DstTrafficEnd).
		WithCount(routeCount)
	flowipv41 := ate.Traffic().NewFlow(flow1).
		WithSrcEndpoints(iDut1).
		WithDstEndpoints(iDut2).
		WithHeaders(ethHeader, ipv4Header).
		WithFrameSize(512)
	flowipv42 := ate.Traffic().NewFlow(flow2).
		WithSrcEndpoints(iDut1).
		WithDstEndpoints(iDut3).
		WithHeaders(ethHeader, ipv4Header).
		WithFrameSize(512)
	return []*ondatra.Flow{flowipv41, flowipv42}
}

// verifyTraffic confirms that every traffic flow has the expected amount of loss (0% or 100%
// depending on wantLoss, +- 2%).
func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, flowName string, wantLoss bool) {
	// Compare traffic loss based on wantLoss.
	lossPct := gnmi.Get(t, ate, gnmi.OC().Flow(flowName).LossPct().State())
	if wantLoss {
		if lossPct < 100-tolerancePct {
			t.Errorf("Traffic is expected to fail %s\n got %v, want 100%% failure", flowName, lossPct)
		} else {
			t.Logf("Traffic Loss Test Passed!")
		}
	} else {
		if lossPct > tolerancePct {
			t.Errorf("Traffic Loss Pct for Flow: %s\n got %v, want 0", flowName, lossPct)
		} else {
			t.Logf("Traffic Test Passed!")
		}
	}
}

// sendTraffic is used to send traffic.
func sendTraffic(t *testing.T, ate *ondatra.ATEDevice, allFlows []*ondatra.Flow) {
	t.Logf("Starting traffic.")
	ate.Traffic().Start(t, allFlows...)
	time.Sleep(trafficDuration)
	ate.Traffic().Stop(t)
	t.Logf("Stop traffic.")
}

// setMED is used to configure routing policy to set BGP MED on DUT.
func setMED(t *testing.T, dut *ondatra.DUTDevice, d *oc.Root) {

	dutPolicyConfPath2 := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().
		PeerGroup(peerGrpName2)

	dutPolicyConfPath3 := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().
		PeerGroup(peerGrpName3)

	// Apply setMed import policy on eBGP Peer1 - ATE Port2 - with MED 100.
	// Apply setMed Import policy on eBGP Peer2 - ATE Port3 -  with MED 50.
	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		gnmi.Replace(t, dut, dutPolicyConfPath2.ApplyPolicy().ImportPolicy().Config(), []string{setMEDPolicy100})
		gnmi.Replace(t, dut, dutPolicyConfPath3.ApplyPolicy().ImportPolicy().Config(), []string{setMEDPolicy50})
	} else {
		gnmi.Replace(t, dut, dutPolicyConfPath2.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).
			ApplyPolicy().ImportPolicy().Config(), []string{setMEDPolicy100})
		gnmi.Replace(t, dut, dutPolicyConfPath3.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).
			ApplyPolicy().ImportPolicy().Config(), []string{setMEDPolicy50})
	}
}

// configPolicy is used to configure routing policies on the DUT.
func configPolicy(t *testing.T, dut *ondatra.DUTDevice, d *oc.Root) {

	rp := d.GetOrCreateRoutingPolicy()

	pdef1 := rp.GetOrCreatePolicyDefinition(setMEDPolicy100)
	st, err := pdef1.AppendNewStatement(aclStatement20)
	if err != nil {
		t.Fatal(err)
	}
	actions1 := st.GetOrCreateActions()
	actions1.GetOrCreateBgpActions().SetMed = oc.UnionUint32(bgpMED100)
	if deviations.BGPSetMedRequiresEqualOspfSetMetric(dut) {
		actions1.GetOrCreateOspfActions().GetOrCreateSetMetric().SetMetric(bgpMED100)
	}
	actions1.PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	pdef2 := rp.GetOrCreatePolicyDefinition(setMEDPolicy50)
	st, err = pdef2.AppendNewStatement(aclStatement20)
	if err != nil {
		t.Fatal(err)
	}
	actions2 := st.GetOrCreateActions()
	actions2.GetOrCreateBgpActions().SetMed = oc.UnionUint32(bgpMED50)
	if deviations.BGPSetMedRequiresEqualOspfSetMetric(dut) {
		actions2.GetOrCreateOspfActions().GetOrCreateSetMetric().SetMetric(bgpMED50)
	}
	actions2.PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	pdef3 := rp.GetOrCreatePolicyDefinition(rplAllowPolicy)
	st, err = pdef3.AppendNewStatement("id-1")
	if err != nil {
		t.Fatal(err)
	}
	action3 := st.GetOrCreateActions()
	action3.PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

// verifySetMed is used to validate MED on received prefixes at ATE Port1.
func verifySetMed(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, wantMEDValue uint32) {
	at := gnmi.OC()

	rib := at.NetworkInstance(ateSrc.Name).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "0").Bgp().Rib()
	prefixPath := rib.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().
		NeighborAny().AdjRibInPre().RouteAny().WithPathId(0).Prefix()

	gnmi.WatchAll(t, ate, prefixPath.State(), time.Minute, func(v *ygnmi.Value[string]) bool {
		_, present := v.Val()
		return present
	}).Await(t)

	wantMED := []uint32{}
	// Build wantMED to compare the diff.
	for i := 0; i < routeCount; i++ {
		wantMED = append(wantMED, uint32(wantMEDValue))
	}

	gotMED := gnmi.GetAll(t, ate, rib.AttrSetAny().Med().State())
	if diff := cmp.Diff(wantMED, gotMED); diff != "" {
		t.Errorf("Obtained MED on ATE is not as expected, got %v, want %v", gotMED, wantMED)
	}
}

// verifyBGPCapabilities is used to Verify BGP capabilities like route refresh as32 and mpbgp.
func verifyBGPCapabilities(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Verifying BGP capabilities.")
	var nbrIP = []string{ateSrc.IPv4, ateDst1.IPv4, ateDst2.IPv4}
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	for _, nbr := range nbrIP {
		nbrPath := statePath.Neighbor(nbr)
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

// verifyPrefixesTelemetry confirms that the dut shows the correct numbers of installed,
// sent and received IPv4 prefixes.
func verifyPrefixesTelemetry(t *testing.T, dut *ondatra.DUTDevice, wantInstalled, wantSent uint32) {
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	prefixesv4 := statePath.Neighbor(ateSrc.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
	if gotInstalled, ok := gnmi.Watch(t, dut, prefixesv4.Installed().State(), time.Minute, func(v *ygnmi.Value[uint32]) bool {
		got, ok := v.Val()
		return ok && got == wantInstalled
	}).Await(t); !ok {
		t.Errorf("Installed prefixes mismatch: got %v, want %v", gotInstalled, wantInstalled)
	}
	if gotSent, ok := gnmi.Watch(t, dut, prefixesv4.Sent().State(), time.Minute, func(v *ygnmi.Value[uint32]) bool {
		got, ok := v.Val()
		return ok && got == wantSent
	}).Await(t); !ok {
		t.Errorf("Sent prefixes mismatch: got %v, want %v", gotSent, wantSent)
	}
}

type bgpNeighbor struct {
	as         uint32
	neighborip string
	isV4       bool
	peerGrp    string
}

// TestRemovePrivateAS is to Validate that private AS numbers are stripped
// before advertisement to the eBGP neighbor.
func TestAlwaysCompareMED(t *testing.T) {
	t.Logf("Start DUT config load.")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	d := &oc.Root{}

	t.Run("Configure DUT interfaces", func(t *testing.T) {
		t.Logf("Start DUT interface Config.")
		configureDUT(t, dut)
	})

	t.Run("Configure DEFAULT network instance", func(t *testing.T) {
		t.Log("Configure Network Instance type.")
		fptest.ConfigureDefaultNetworkInstance(t, dut)
	})

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	t.Run("Configure BGP Neighbors", func(t *testing.T) {
		t.Logf("Start DUT BGP Config.")
		gnmi.Delete(t, dut, dutConfPath.Config())
		configPolicy(t, dut, d)
		dutConf := bgpCreateNbr(dutAS, ateAS1, dut)
		gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
		fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.GetConfig(t, dut, dutConfPath.Config()))
	})

	var allFlows []*ondatra.Flow
	t.Run("Configure ATE", func(t *testing.T) {
		t.Logf("Start ATE Config.")
		allFlows = configureATE(t, ate)
	})

	t.Run("Verify port status on DUT", func(t *testing.T) {
		t.Log("Verifying port status.")
		verifyPortsUp(t, dut.Device)
	})

	t.Run("Verify BGP telemetry", func(t *testing.T) {
		t.Log("Check BGP parameters.")
		verifyBgpTelemetry(t, dut)
		t.Log("Check BGP Capabilities")
		verifyBGPCapabilities(t, dut)
	})

	t.Run("Configure SET MED on DUT", func(t *testing.T) {
		setMED(t, dut, d)
	})

	t.Run("Configure always compare med on DUT", func(t *testing.T) {
		t.Log("Configure always compare med on DUT.")
		gnmi.Replace(t, dut, dutConfPath.Bgp().Global().RouteSelectionOptions().AlwaysCompareMed().Config(), true)
	})

	t.Run("Verify received BGP routes at ATE Port 1 have lowest MED", func(t *testing.T) {
		t.Log("Verify BGP prefix telemetry.")
		verifyPrefixesTelemetry(t, dut, 0, routeCount)
		t.Log("Verify best route advertised to atePort1 is Peer with lowest MED 50 - eBGP Peer2.")
		verifySetMed(t, dut, ate, bgpMED50)
	})

	t.Run("Send and validate traffic from ATE Port1", func(t *testing.T) {
		t.Log("Validate traffic flowing to the prefixes received from eBGP neighbor #2 from DUT (lowest MED-50).")
		sendTraffic(t, ate, allFlows)
		verifyTraffic(t, ate, flow1, wantLoss)
		verifyTraffic(t, ate, flow2, !wantLoss)
	})

	t.Run("Remove MED settings on DUT", func(t *testing.T) {
		t.Log("Disable MED settings on DUT.")
		dutPolicyConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
		if deviations.RoutePolicyUnderAFIUnsupported(dut) {
			gnmi.Replace(t, dut, dutPolicyConfPath.PeerGroup(peerGrpName2).ApplyPolicy().ImportPolicy().Config(), []string{rplAllowPolicy})
			gnmi.Replace(t, dut, dutPolicyConfPath.PeerGroup(peerGrpName3).ApplyPolicy().ImportPolicy().Config(), []string{rplAllowPolicy})
		} else {
			gnmi.Replace(t, dut, dutPolicyConfPath.PeerGroup(peerGrpName2).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy().ImportPolicy().Config(), []string{rplAllowPolicy})
			gnmi.Replace(t, dut, dutPolicyConfPath.PeerGroup(peerGrpName3).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy().ImportPolicy().Config(), []string{rplAllowPolicy})
		}

	})

	t.Run("Verify MED on received routes at ATE Port1 after removing MED settings", func(t *testing.T) {
		t.Log("Verify BGP prefix telemetry.")
		verifyPrefixesTelemetry(t, dut, 0, routeCount)
		t.Log("Verify best route advertised to atePort1.")
		verifySetMed(t, dut, ate, uint32(0))
	})

	t.Run("Send and verify traffic after removing MED settings on DUT", func(t *testing.T) {
		t.Log("Validate traffic change due to change in MED settings - Best route changes.")
		sendTraffic(t, ate, allFlows)
		verifyTraffic(t, ate, flow1, !wantLoss)
		verifyTraffic(t, ate, flow2, wantLoss)
	})
}
