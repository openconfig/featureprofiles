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

package bgp_remove_private_as_test

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

// The testbed consists of ate:port1 -> dut:port1 and
// dut:port2 -> ate:port2.  The first pair is called the "source"
// pair, and the second the "destination" pair.
//
// Source: ate:port1 -> dut:port1 subnet 192.0.2.0/30
// Destination: dut:port2 -> ate:port2 subnet 192.0.2.4/30
//
// Note that the first (.0, .3) and last (.4, .7) IPv4 addresses are
// reserved from the subnet for broadcast, so a /30 leaves exactly 2
// usable addresses. This does not apply to IPv6 which allows /127
// for point to point links, but we use /126 so the numbering is
// consistent with IPv4.

const (
	trafficDuration        = 1 * time.Minute
	ipv4SrcTraffic         = "192.0.2.2"
	advertisedRoutesv4CIDR = "203.0.113.1/32"
	peerGrpName1           = "BGP-PEER-GROUP1"
	peerGrpName2           = "BGP-PEER-GROUP2"
	policyName             = "ALLOW"
	routeCount             = 254
	dutAS                  = 500
	ateAS1                 = 100
	ateAS2                 = 200
	plenIPv4               = 30
	plenIPv6               = 126
	removeASPath           = true
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

// configureDUT configures all the interfaces on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dc := gnmi.OC()
	i1 := dutSrc.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	i2 := dutDst.NewOCInterface(dut.Port(t, "port2").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i2.GetName()).Config(), i2)
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
	nbr2v4 := &bgpNeighbor{as: ateAS2, neighborip: ateDst.IPv4, isV4: true, peerGrp: peerGrpName2}
	nbrs := []*bgpNeighbor{nbr1v4, nbr2v4}

	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.RouterId = ygot.String(dutDst.IPv4)
	global.As = ygot.Uint32(localAs)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	pg1 := bgp.GetOrCreatePeerGroup(peerGrpName1)
	pg1.PeerAs = ygot.Uint32(ateAS1)
	pg1.PeerGroupName = ygot.String(peerGrpName1)

	pg2 := bgp.GetOrCreatePeerGroup(peerGrpName2)
	pg2.PeerAs = ygot.Uint32(ateAS2)
	pg2.PeerGroupName = ygot.String(peerGrpName2)

	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		rpl := pg1.GetOrCreateApplyPolicy()
		rpl.ImportPolicy = []string{policyName}
		rpl.ExportPolicy = []string{policyName}

		rp2 := pg2.GetOrCreateApplyPolicy()
		rp2.ImportPolicy = []string{policyName}
		rp2.ExportPolicy = []string{policyName}
	} else {
		pgaf := pg1.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		pgaf.Enabled = ygot.Bool(true)
		rpl := pgaf.GetOrCreateApplyPolicy()
		rpl.ImportPolicy = []string{policyName}
		rpl.ExportPolicy = []string{policyName}

		pgaf2 := pg2.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		pgaf2.Enabled = ygot.Bool(true)
		rp2 := pgaf2.GetOrCreateApplyPolicy()
		rp2.ImportPolicy = []string{policyName}
		rp2.ExportPolicy = []string{policyName}
	}

	for _, nbr := range nbrs {
		nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
		nv4.PeerGroup = ygot.String(nbr.peerGrp)
		nv4.PeerAs = ygot.Uint32(nbr.as)
		nv4.Enabled = ygot.Bool(true)
		af4 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		af4.Enabled = ygot.Bool(true)
		af6 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		af6.Enabled = ygot.Bool(false)
	}
	return niProto
}

// verifyBGPTelemetry checks that the dut has an established BGP session with reasonable settings.
func verifyBGPTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	var nbrIP = []string{ateSrc.IPv4, ateDst.IPv4}
	t.Logf("Verifying BGP state.")
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	for _, nbr := range nbrIP {
		var status *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]
		nbrPath := statePath.Neighbor(nbr)
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
		t.Logf("BGP adjacency for %s: %s", nbr, state)
		if want := oc.Bgp_Neighbor_SessionState_ESTABLISHED; state != want {
			t.Errorf("BGP peer %s status got %v, want %d", nbr, status, want)
		}
	}
}

// verifyPrefixesTelemetry confirms that the dut shows the correct numbers of installed,
// sent and received IPv4 prefixes.
func verifyPrefixesTelemetry(t *testing.T, dut *ondatra.DUTDevice, nbr string, wantInstalled, wantSent uint32) {
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	prefixesv4 := statePath.Neighbor(nbr).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
	if gotInstalled := gnmi.Get(t, dut, prefixesv4.Installed().State()); gotInstalled != wantInstalled {
		t.Errorf("Installed prefixes mismatch: got %v, want %v", gotInstalled, wantInstalled)
	}
	if gotSent := gnmi.Get(t, dut, prefixesv4.Sent().State()); gotSent != wantSent {
		t.Errorf("Sent prefixes mismatch: got %v, want %v", gotSent, wantSent)
	}
}

// configureATE configures the interfaces and BGP protocols on an ATE, including
// advertising some(faked) networks over BGP.
func configureATE(t *testing.T, ate *ondatra.ATEDevice, asSeg []uint32, asSEQMode bool) *ondatra.ATETopology {
	port1 := ate.Port(t, "port1")
	topo := ate.Topology().New()
	iDut1 := topo.AddInterface(ateSrc.Name).WithPort(port1)
	iDut1.IPv4().WithAddress(ateSrc.IPv4CIDR()).WithDefaultGateway(dutSrc.IPv4)

	port2 := ate.Port(t, "port2")
	iDut2 := topo.AddInterface(ateDst.Name).WithPort(port2)
	iDut2.IPv4().WithAddress(ateDst.IPv4CIDR()).WithDefaultGateway(dutDst.IPv4)

	// Setup ATE BGP route v4 advertisement.
	bgpDut1 := iDut1.BGP()
	bgpDut1.AddPeer().WithPeerAddress(dutSrc.IPv4).WithLocalASN(ateAS1).
		WithTypeExternal()

	bgpDut2 := iDut2.BGP()
	bgpDut2.AddPeer().WithPeerAddress(dutDst.IPv4).WithLocalASN(ateAS2).
		WithTypeExternal()

	bgpNeti1 := iDut1.AddNetwork("bgpNeti1")
	bgpNeti1.IPv4().WithAddress(advertisedRoutesv4CIDR).WithCount(routeCount)
	bgpNeti1.BGP().WithNextHopAddress(ateSrc.IPv4)

	if asSEQMode {
		bgpNeti1.BGP().AddASPathSegment(asSeg...).WithTypeSEQ()
	} else {
		// TODO : SET mode is not working
		// https://github.com/openconfig/featureprofiles/issues/1659
		bgpNeti1.BGP().AddASPathSegment(asSeg...).WithTypeSET()
	}

	t.Logf("Pushing config to ATE and starting protocols...")
	topo.Push(t)
	topo.StartProtocols(t)

	return topo
}

type bgpNeighbor struct {
	as         uint32
	neighborip string
	isV4       bool
	peerGrp    string
}

// verifyBGPAsPath is to Validate AS Path attribute using bgp rib telemetry on ATE.
func verifyBGPAsPath(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, asSeg []uint32, removeASPath bool) {
	at := gnmi.OC()
	rib := at.NetworkInstance(ateDst.Name).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "0").Bgp().Rib()
	prefixPath := rib.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().
		NeighborAny().AdjRibInPre().RouteAny().WithPathId(0).Prefix()

	gnmi.WatchAll(t, ate, prefixPath.State(), time.Minute, func(v *ygnmi.Value[string]) bool {
		_, present := v.Val()
		return present
	}).Await(t)

	var wantASSeg = []uint32{dutAS, ateAS1}

	if removeASPath {
		for _, as := range asSeg {
			if as < 64512 {
				wantASSeg = append(wantASSeg, as)
			}
		}
	} else {
		wantASSeg = append(wantASSeg, asSeg...)
	}

	gotASSeg, ok := gnmi.WatchAll(t, ate, rib.AttrSetAny().AsSegmentMap().State(), 1*time.Minute, func(v *ygnmi.Value[map[uint32]*oc.NetworkInstance_Protocol_Bgp_Rib_AttrSet_AsSegment]) bool {
		val, present := v.Val()
		if present {
			for _, as := range val {
				if cmp.Equal(as.Member, wantASSeg) {
					return true
				}
			}
		}
		return false
	}).Await(t)
	if !ok {
		t.Errorf("Obtained AS path on ATE is not as expected, gotASSeg %v, wantASSeg %v", gotASSeg, wantASSeg)
	}
}

// configreRoutePolicy adds route-policy config
func configureRoutePolicy(t *testing.T, dut *ondatra.DUTDevice, name string, pr oc.E_RoutingPolicy_PolicyResultType) {
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pd := rp.GetOrCreatePolicyDefinition(name)
	st, err := pd.AppendNewStatement("id-1")
	if err != nil {
		t.Fatal(err)
	}
	stc := st.GetOrCreateConditions()
	stc.InstallProtocolEq = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP
	st.GetOrCreateActions().PolicyResult = pr
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

// TestRemovePrivateAS is to Validate that private AS numbers are stripped
// before advertisement to the eBGP neighbor.
func TestRemovePrivateAS(t *testing.T) {
	t.Logf("Start DUT config load.")
	dut := ondatra.DUT(t, "dut")

	t.Run("Configure DUT interfaces", func(t *testing.T) {
		t.Logf("Start DUT interface Config.")
		configureDUT(t, dut)
	})

	t.Run("Configure DEFAULT network instance.", func(t *testing.T) {
		t.Log("Configure Network Instance type to DEFAULT.")
		dutConfNIPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
		gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
	})

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	t.Run("Configure BGP Neighbors.", func(t *testing.T) {
		t.Logf("Start DUT BGP Config.")
		gnmi.Delete(t, dut, dutConfPath.Config())
		configureRoutePolicy(t, dut, policyName, oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
		dutConf := bgpCreateNbr(dutAS, ateAS1, dut)
		gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
		fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.GetConfig(t, dut, dutConfPath.Config()))
	})

	cases := []struct {
		desc      string
		asSeg     []uint32
		asSEQMode bool
	}{{
		desc:      "AS Path SEQ - 65501, 65507, 65534",
		asSeg:     []uint32{65501, 65507, 65534},
		asSEQMode: true,
	}, {
		desc:      "AS Path SEQ - 65501, 600",
		asSeg:     []uint32{65501, 600},
		asSEQMode: true,
	}, {
		desc:      "AS Path SEQ - 800, 65501, 600",
		asSeg:     []uint32{800, 65501, 600},
		asSEQMode: true,
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Logf("Start ATE Config.")
			ate := ondatra.ATE(t, "ate")
			topo := configureATE(t, ate, tc.asSeg, tc.asSEQMode)

			t.Log("Verifying port status.")
			verifyPortsUp(t, dut.Device)

			t.Log("Check BGP parameters.")
			verifyBGPTelemetry(t, dut)

			t.Log("Verify BGP prefix telemetry.")
			verifyPrefixesTelemetry(t, dut, ateSrc.IPv4, routeCount, 0)
			verifyPrefixesTelemetry(t, dut, ateDst.IPv4, 0, routeCount)

			t.Log("Verify AS Path list received at ate Port2 including private AS number.")
			verifyBGPAsPath(t, dut, ate, tc.asSeg, !removeASPath)

			t.Log("Configure remove private AS on DUT.")
			gnmi.Update(t, dut, dutConfPath.Bgp().PeerGroup(peerGrpName2).RemovePrivateAs().Config(), oc.Bgp_RemovePrivateAsOption_PRIVATE_AS_REMOVE_ALL)

			t.Log("Private AS numbers should be stripped off while advertising BGP routes into public AS.")
			verifyBGPAsPath(t, dut, ate, tc.asSeg, removeASPath)

			topo.StopProtocols(t)

			t.Log("Remove remove-private-AS on DUT.")
			gnmi.Delete(t, dut, dutConfPath.Bgp().PeerGroup(peerGrpName2).RemovePrivateAs().Config())
		})
	}
}
