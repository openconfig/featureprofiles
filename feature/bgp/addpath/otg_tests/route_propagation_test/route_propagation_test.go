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

// Package route_propagation_test implements RT-1.3: BGP Route Propagation.
package route_propagation_test

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	otg "github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

// This test topology connects the DUT and ATE over two ports.
//
// We are testing BGP route propagation across the DUT, so we are using the ATE to simulate two
// neighbors: AS 65536 on ATE port1, and AS 65538 on ATE port2, which both neighbor the DUT AS
// 65537 (both port1 and port2).
//
// TestAddPath instead runs ATE port1 as an iBGP peer in the DUT AS (route reflector to the
// DUT), so the DUT receives multiple paths per prefix over iBGP.
var (
	// TODO: The DUT IP should be table driven as well, but assigning only an IPv6
	// address seems to prevent Ixia from bringing up the connection.
	dutPort1 = attrs.Attributes{
		Name:    "port1",
		Desc:    "To ATE",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
		IPv6:    "2001:db8::1",
		IPv6Len: 126,
	}
	dutPort2 = attrs.Attributes{
		Name:    "port2",
		Desc:    "To ATE",
		IPv4:    "192.0.2.5",
		IPv4Len: 30,
		IPv6:    "2001:db8::5",
		IPv6Len: 126,
	}
	dutAS = uint32(65537)

	ateAS1 = uint32(65536)
	ateAS2 = uint32(65538)
)

const (
	rplPermitAll = "PERMIT-ALL"
)

type otgPortDetails struct {
	mac, routerId string
	pathId        int32
}

var (
	otgPort1Details = otgPortDetails{
		mac:      "02:00:01:01:01:01",
		routerId: "192.0.2.2",
		pathId:   1,
	}
	otgPort2Details = otgPortDetails{
		mac:      "02:00:02:01:01:01",
		routerId: "192.0.2.6",
		pathId:   1,
	}
)

type ip struct {
	v4, v6 string
}

type ateData struct {
	Port1         ip
	Port2         ip
	Port1Neighbor string
	Port2Neighbor string
	prefixesStart ip
	prefixesCount uint32
	// Path IDs that ATE port-1 uses for every prefix; nil means a single path ID 1.
	pathIDs []uint32
	// port1IBGP makes ATE port-1 an iBGP peer in the DUT AS (route reflector to
	// client) instead of an eBGP peer in ateAS1.
	port1IBGP bool
}

// port1AS returns the AS number of the ATE port-1 BGP peer.
func (ad *ateData) port1AS() uint32 {
	if ad.port1IBGP {
		return dutAS
	}
	return ateAS1
}

// routePathIDs returns the path IDs ATE port-1 advertises for every prefix.
func (ad *ateData) routePathIDs() []uint32 {
	if len(ad.pathIDs) == 0 {
		return []uint32{uint32(otgPort1Details.pathId)}
	}
	return ad.pathIDs
}

func (ad *ateData) ConfigureOTG(t *testing.T, otg *otg.OTG, ateList []string) gosnappi.Config {
	config := gosnappi.NewConfig()
	bgp4ObjectMap := make(map[string]gosnappi.BgpV4Peer)
	bgp6ObjectMap := make(map[string]gosnappi.BgpV6Peer)
	ipv4ObjectMap := make(map[string]gosnappi.DeviceIpv4)
	ipv6ObjectMap := make(map[string]gosnappi.DeviceIpv6)
	ateIndex := 0
	for _, v := range []struct {
		iface    otgPortDetails
		ip       ip
		neighbor string
		as       uint32
		ibgp     bool
	}{
		{otgPort1Details, ad.Port1, ad.Port1Neighbor, ad.port1AS(), ad.port1IBGP},
		{otgPort2Details, ad.Port2, ad.Port2Neighbor, ateAS2, false},
	} {

		devName := ateList[ateIndex] + ".dev"
		port := config.Ports().Add().SetName(ateList[ateIndex])
		dev := config.Devices().Add().SetName(devName)
		ateIndex++

		eth := dev.Ethernets().Add().SetName(devName + ".Eth").SetMac(v.iface.mac)
		eth.Connection().SetPortName(port.Name())
		bgp := dev.Bgp().SetRouterId(v.iface.routerId)
		if v.ip.v4 != "" {
			address := strings.Split(v.ip.v4, "/")[0]
			prefixInt4, _ := strconv.Atoi(strings.Split(v.ip.v4, "/")[1])
			ipv4 := eth.Ipv4Addresses().Add().SetName(devName + ".IPv4").SetAddress(address).SetGateway(v.neighbor).SetPrefix(uint32(prefixInt4))
			bgp4Name := devName + ".BGP4.peer"
			bgp4Peer := bgp.Ipv4Interfaces().Add().SetIpv4Name(ipv4.Name()).Peers().Add().SetName(bgp4Name).SetPeerAddress(ipv4.Gateway()).SetAsNumber(uint32(v.as)).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
			if v.ibgp {
				bgp4Peer.SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
			}

			bgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
			bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

			bgp4ObjectMap[bgp4Name] = bgp4Peer
			ipv4ObjectMap[devName+".IPv4"] = ipv4
		}
		if v.ip.v6 != "" {
			address := strings.Split(v.ip.v6, "/")[0]
			prefixInt6, _ := strconv.Atoi(strings.Split(v.ip.v6, "/")[1])
			ipv6 := eth.Ipv6Addresses().Add().SetName(devName + ".IPv6").SetAddress(address).SetGateway(v.neighbor).SetPrefix(uint32(prefixInt6))
			bgp6Name := devName + ".BGP6.peer"
			bgp6Peer := bgp.Ipv6Interfaces().Add().SetIpv6Name(ipv6.Name()).Peers().Add().SetName(bgp6Name).SetPeerAddress(ipv6.Gateway()).SetAsNumber(uint32(v.as)).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
			if v.ibgp {
				bgp6Peer.SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
			}

			bgp6Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true).SetExtendedNextHopEncoding(true)
			bgp6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

			bgp6ObjectMap[bgp6Name] = bgp6Peer
			ipv6ObjectMap[devName+".IPv6"] = ipv6
		}
	}
	if ad.prefixesStart.v4 != "" {
		if ad.Port1.v4 != "" {
			bgpName := ateList[0] + ".dev.BGP4.peer"
			bgpPeer := bgp4ObjectMap[bgpName]
			ip := ipv4ObjectMap[ateList[0]+".dev.IPv4"]
			firstAdvAddr := strings.Split(ad.prefixesStart.v4, "/")[0]
			firstAdvPrefix, _ := strconv.Atoi(strings.Split(ad.prefixesStart.v4, "/")[1])
			for _, id := range ad.routePathIDs() {
				bgp4PeerRoutes := bgpPeer.V4Routes().Add().SetName(bgpName + ".rr4." + strconv.Itoa(int(id))).SetNextHopIpv4Address(ip.Address()).SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
				bgp4PeerRoutes.Addresses().Add().SetAddress(firstAdvAddr).SetPrefix(uint32(firstAdvPrefix)).SetCount(uint32(ad.prefixesCount))
				bgp4PeerRoutes.AddPath().SetPathId(id)
			}
		} else {
			bgpName := ateList[0] + ".dev.BGP6.peer"
			bgpPeer := bgp6ObjectMap[bgpName]
			firstAdvAddr := strings.Split(ad.prefixesStart.v4, "/")[0]
			firstAdvPrefix, _ := strconv.Atoi(strings.Split(ad.prefixesStart.v4, "/")[1])
			for _, id := range ad.routePathIDs() {
				bgp4PeerRoutes := bgpPeer.V4Routes().Add().SetName(bgpName + ".rr4." + strconv.Itoa(int(id)))
				bgp4PeerRoutes.Addresses().Add().SetAddress(firstAdvAddr).SetPrefix(uint32(firstAdvPrefix)).SetCount(uint32(ad.prefixesCount))
				bgp4PeerRoutes.AddPath().SetPathId(id)
			}
		}
	}
	if ad.prefixesStart.v6 != "" {
		bgp6Name := ateList[0] + ".dev.BGP6.peer"
		bgp6Peer := bgp6ObjectMap[bgp6Name]
		firstAdvAddr := strings.Split(ad.prefixesStart.v6, "/")[0]
		firstAdvPrefix, _ := strconv.Atoi(strings.Split(ad.prefixesStart.v6, "/")[1])
		for _, id := range ad.routePathIDs() {
			bgp6PeerRoutes := bgp6Peer.V6Routes().Add().SetName(bgp6Name + ".rr6." + strconv.Itoa(int(id)))
			bgp6PeerRoutes.Addresses().Add().SetAddress(firstAdvAddr).SetPrefix(uint32(firstAdvPrefix)).SetCount(uint32(ad.prefixesCount))
			bgp6PeerRoutes.AddPath().SetPathId(id)
		}
	}

	t.Logf("Pushing config to ATE and starting protocols...")
	otg.PushConfig(t, config)
	otg.StartProtocols(t)
	return config

}

type dutData struct {
	bgpOC                         *oc.NetworkInstance_Protocol_Bgp
	ni                            string
	explicitPortSpeed             *bool
	explicitInterfaceInDefaultVRF *bool
}

func (d *dutData) networkInstance(dut *ondatra.DUTDevice) string {
	if d.ni != "" {
		return d.ni
	}
	return deviations.DefaultNetworkInstance(dut)
}

func (d *dutData) hasExplicitPortSpeed(dut *ondatra.DUTDevice) bool {
	if d.explicitPortSpeed != nil {
		return *d.explicitPortSpeed
	}
	return deviations.ExplicitPortSpeed(dut)
}

func (d *dutData) hasExplicitInterfaceInDefaultVRF(dut *ondatra.DUTDevice) bool {
	if d.explicitInterfaceInDefaultVRF != nil {
		return *d.explicitInterfaceInDefaultVRF
	}
	return deviations.ExplicitInterfaceInDefaultVRF(dut)
}

func configureRoutingPolicy(d *oc.Root) (*oc.RoutingPolicy, error) {
	rp := d.GetOrCreateRoutingPolicy()
	pdef := rp.GetOrCreatePolicyDefinition(rplPermitAll)
	stmt, err := pdef.AppendNewStatement("20")
	if err != nil {
		return nil, err
	}
	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	return rp, nil
}

func (d *dutData) Configure(t *testing.T, dut *ondatra.DUTDevice) {
	for _, a := range []attrs.Attributes{dutPort1, dutPort2} {
		ocName := dut.Port(t, a.Name).Name()
		gnmi.Replace(t, dut, gnmi.OC().Interface(ocName).Config(), a.NewOCInterface(ocName, dut))
	}

	if d.ni == "" {
		t.Log("Configure Network Instance")
	}
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if d.hasExplicitPortSpeed(dut) {
		for _, a := range []attrs.Attributes{dutPort1, dutPort2} {
			fptest.SetPortSpeed(t, dut.Port(t, a.Name))
		}
	}
	ni := d.networkInstance(dut)
	if d.hasExplicitInterfaceInDefaultVRF(dut) {
		for _, a := range []attrs.Attributes{dutPort1, dutPort2} {
			ocName := dut.Port(t, a.Name).Name()
			fptest.AssignToNetworkInstance(t, dut, ocName, ni, 0)
		}
	}

	dutProto := gnmi.OC().NetworkInstance(ni).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	key := oc.NetworkInstance_Protocol_Key{
		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
		Name:       "BGP",
	}

	niOC := &oc.NetworkInstance{
		Name: ygot.String(ni),
		Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE,
		Protocol: map[oc.NetworkInstance_Protocol_Key]*oc.NetworkInstance_Protocol{
			key: {
				Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
				Name:       ygot.String("BGP"),
				Bgp:        d.bgpOC,
			},
		},
	}
	rpl, err := configureRoutingPolicy(&oc.Root{})
	if err != nil {
		t.Fatalf("Failed to configure routing policy: %v", err)
	}
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rpl)
	gnmi.Replace(t, dut, dutProto.Config(), niOC.Protocol[key])
}

func (d *dutData) AwaitBGPEstablished(t *testing.T, dut *ondatra.DUTDevice) {
	ni := d.networkInstance(dut)
	for neighbor := range d.bgpOC.Neighbor {
		gnmi.Await(t, dut, gnmi.OC().NetworkInstance(ni).
			Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").
			Bgp().
			Neighbor(neighbor).
			SessionState().State(), time.Second*120, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
	}
	if d.ni == "" {
		t.Log("BGP sessions established")
	}
}

func getPeerGroupWithPolicy(pgn string, aftype oc.E_BgpTypes_AFI_SAFI_TYPE, policyUnderPeerGroup bool) *oc.NetworkInstance_Protocol_Bgp_PeerGroup {
	bgp := &oc.NetworkInstance_Protocol_Bgp{}
	pg := bgp.GetOrCreatePeerGroup(pgn)
	pg.GetOrCreateTimers().SetMinimumAdvertisementInterval(10)
	if policyUnderPeerGroup {
		// policy under peer group
		rpl := pg.GetOrCreateApplyPolicy()
		rpl.SetExportPolicy([]string{rplPermitAll})
		rpl.SetImportPolicy([]string{rplPermitAll})
		return pg
	}

	// policy under peer group AFI
	afisafi := pg.GetOrCreateAfiSafi(aftype)
	afisafi.Enabled = ygot.Bool(true)
	rpl := afisafi.GetOrCreateApplyPolicy()
	rpl.SetExportPolicy([]string{rplPermitAll})
	rpl.SetImportPolicy([]string{rplPermitAll})
	return pg
}

func getPeerGroup(pgn string, aftype oc.E_BgpTypes_AFI_SAFI_TYPE, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol_Bgp_PeerGroup {
	return getPeerGroupWithPolicy(pgn, aftype, deviations.RoutePolicyUnderAFIUnsupported(dut))
}

// addPathCfg is the ADD-Path configuration that TestAddPath applies to the DUT.
type addPathCfg struct {
	nbrReceive *bool // add-paths/config/receive on the ATE port-1 neighbor; nil leaves it unset.
	pgReceive  *bool // add-paths/config/receive on BGP-PEER-GROUP1; nil leaves it unset.
}

// addPathBGP builds the BGP OpenConfig structure for a single AFI/SAFI with ADD-Path options applied.
func addPathBGP(afi oc.E_BgpTypes_AFI_SAFI_TYPE, rxNbr, txNbr string, cfg addPathCfg, policyUnderPeerGroup bool) *oc.NetworkInstance_Protocol_Bgp {
	bgp := &oc.NetworkInstance_Protocol_Bgp{
		PeerGroup: map[string]*oc.NetworkInstance_Protocol_Bgp_PeerGroup{
			"BGP-PEER-GROUP1": getPeerGroupWithPolicy("BGP-PEER-GROUP1", afi, policyUnderPeerGroup),
			"BGP-PEER-GROUP2": getPeerGroupWithPolicy("BGP-PEER-GROUP2", afi, policyUnderPeerGroup),
		},
	}
	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(dutAS)
	global.RouterId = ygot.String(dutPort2.IPv4)
	global.GetOrCreateAfiSafi(afi).Enabled = ygot.Bool(true)

	rx := bgp.GetOrCreateNeighbor(rxNbr)
	rx.PeerAs = ygot.Uint32(dutAS)
	rx.PeerGroup = ygot.String("BGP-PEER-GROUP1")
	rx.NeighborAddress = ygot.String(rxNbr)
	// When ADD-Path receive is configured only on the peer-group, leave the
	// AFI-SAFI unset on the neighbor so that it inherits the whole AFI-SAFI
	// configuration from BGP-PEER-GROUP1.
	if cfg.nbrReceive != nil || cfg.pgReceive == nil {
		rx.GetOrCreateAfiSafi(afi).Enabled = ygot.Bool(true)
	}

	tx := bgp.GetOrCreateNeighbor(txNbr)
	tx.PeerAs = ygot.Uint32(ateAS2)
	tx.PeerGroup = ygot.String("BGP-PEER-GROUP2")
	tx.NeighborAddress = ygot.String(txNbr)
	tx.GetOrCreateAfiSafi(afi).Enabled = ygot.Bool(true)

	if cfg.nbrReceive != nil {
		rx.GetOrCreateAfiSafi(afi).GetOrCreateAddPaths().Receive = cfg.nbrReceive
	}
	if cfg.pgReceive != nil {
		pgAfi := bgp.GetOrCreatePeerGroup("BGP-PEER-GROUP1").GetOrCreateAfiSafi(afi)
		pgAfi.Enabled = ygot.Bool(true)
		pgAfi.GetOrCreateAddPaths().Receive = cfg.pgReceive
	}
	return bgp
}

func verifyOTGBGPTelemetry(t *testing.T, otg *otg.OTG, c gosnappi.Config, state string) {
	for _, d := range c.Devices().Items() {
		for _, ip := range d.Bgp().Ipv4Interfaces().Items() {
			for _, configPeer := range ip.Peers().Items() {
				nbrPath := gnmi.OTG().BgpPeer(configPeer.Name())
				_, ok := gnmi.Watch(t, otg, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[otgtelemetry.E_BgpPeer_SessionState]) bool {
					currState, ok := val.Val()
					return ok && currState.String() == state
				}).Await(t)
				if !ok {
					fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, otg, nbrPath.State()))
					t.Errorf("No BGP neighbor formed for peer %s", configPeer.Name())
				}
			}
		}
		for _, ip := range d.Bgp().Ipv6Interfaces().Items() {
			for _, configPeer := range ip.Peers().Items() {
				nbrPath := gnmi.OTG().BgpPeer(configPeer.Name())
				_, ok := gnmi.Watch(t, otg, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[otgtelemetry.E_BgpPeer_SessionState]) bool {
					currState, ok := val.Val()
					return ok && currState.String() == state
				}).Await(t)
				if !ok {
					fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, otg, nbrPath.State()))
					t.Errorf("No BGP neighbor formed for peer %s", configPeer.Name())
				}
			}
		}
	}
}

type OTGBGPPrefix struct {
	PeerName     string
	Address      string
	PrefixLength uint32
}

func checkOTGBGP4Prefix(t *testing.T, otg *otg.OTG, config gosnappi.Config, expectedOTGBGPPrefix OTGBGPPrefix) bool {
	t.Helper()
	_, ok := gnmi.WatchAll(t,
		otg,
		gnmi.OTG().BgpPeer(expectedOTGBGPPrefix.PeerName).UnicastIpv4PrefixAny().State(),
		2*time.Minute,
		func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv4Prefix]) bool {
			if prefix, present := v.Val(); present {
				return prefix.GetAddress() == expectedOTGBGPPrefix.Address &&
					prefix.GetPrefixLength() == expectedOTGBGPPrefix.PrefixLength
			}
			return false
		}).Await(t)
	return ok
}

func checkOTGBGP6Prefix(t *testing.T, otg *otg.OTG, config gosnappi.Config, expectedOTGBGPPrefix OTGBGPPrefix) bool {
	t.Helper()
	_, ok := gnmi.WatchAll(t,
		otg,
		gnmi.OTG().BgpPeer(expectedOTGBGPPrefix.PeerName).UnicastIpv6PrefixAny().State(),
		2*time.Minute,
		func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv6Prefix]) bool {
			if prefix, present := v.Val(); present {
				return prefix.GetAddress() == expectedOTGBGPPrefix.Address &&
					prefix.GetPrefixLength() == expectedOTGBGPPrefix.PrefixLength
			}
			return false
		}).Await(t)
	return ok
}

// verifyAddPathState verifies DUT telemetry for BGP ADD-Path receive states.
func verifyAddPathState(t *testing.T, dut *ondatra.DUTDevice, ni, tag string, afi oc.E_BgpTypes_AFI_SAFI_TYPE, rxNbr string, cfg addPathCfg) {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	if cfg.nbrReceive != nil {
		q := bgpPath.Neighbor(rxNbr).AfiSafi(afi).AddPaths().Receive().State()
		_, ok := gnmi.Watch(t, dut, q, 30*time.Second, func(v *ygnmi.Value[bool]) bool {
			got, present := v.Val()
			return present && got == *cfg.nbrReceive
		}).Await(t)
		if !ok {
			got, present := gnmi.Lookup(t, dut, q).Val()
			t.Errorf("%s STEP 2/4 FAIL: neighbor %s add-paths/state/receive: got %v (present: %v), want %t", tag, rxNbr, got, present, *cfg.nbrReceive)
		} else {
			t.Logf("%s STEP 2/4 PASS: neighbor %s add-paths/state/receive = %t", tag, rxNbr, *cfg.nbrReceive)
		}
	}

	if cfg.pgReceive != nil {
		q := bgpPath.PeerGroup("BGP-PEER-GROUP1").AfiSafi(afi).AddPaths().Receive().State()
		_, ok := gnmi.Watch(t, dut, q, 30*time.Second, func(v *ygnmi.Value[bool]) bool {
			got, present := v.Val()
			return present && got == *cfg.pgReceive
		}).Await(t)
		if !ok {
			got, present := gnmi.Lookup(t, dut, q).Val()
			t.Errorf("%s STEP 2/4 FAIL: peer-group BGP-PEER-GROUP1 add-paths/state/receive: got %v (present: %v), want %t", tag, got, present, *cfg.pgReceive)
			if cfg.nbrReceive != nil {
				nbrVal, nbrPresent := gnmi.Lookup(t, dut, bgpPath.Neighbor(rxNbr).AfiSafi(afi).AddPaths().Receive().State()).Val()
				t.Logf("%s INFO: override mismatch detail — configured pg=%t nbr=%t; observed state pg=%v(present:%v) nbr=%v(present:%v)",
					tag, *cfg.pgReceive, *cfg.nbrReceive, got, present, nbrVal, nbrPresent)
			}
		} else {
			t.Logf("%s STEP 2/4 PASS: peer-group BGP-PEER-GROUP1 add-paths/state/receive = %t", tag, *cfg.pgReceive)
		}
		if cfg.nbrReceive == nil {
			rxQ := bgpPath.Neighbor(rxNbr).AfiSafi(afi).AddPaths().Receive().State()
			rxVal, rxPresent := gnmi.Lookup(t, dut, rxQ).Val()
			if !rxPresent {
				t.Logf("%s INFO: neighbor %s inherited add-paths/state/receive: not reported by DUT (present: false)", tag, rxNbr)
			} else {
				t.Logf("%s INFO: neighbor %s inherited add-paths/state/receive = %t", tag, rxNbr, rxVal)
			}
		}
	}

	capQ := bgpPath.Neighbor(rxNbr).SupportedCapabilities().State()
	// When ADD-Path receive is in effect for the neighbor (set on the neighbor,
	// or inherited from the peer-group), ADD-Path must be negotiated on the
	// session and reported in the neighbor's supported-capabilities.
	receiveOn := (cfg.nbrReceive != nil && *cfg.nbrReceive) || (cfg.nbrReceive == nil && cfg.pgReceive != nil && *cfg.pgReceive)
	if receiveOn {
		_, ok := gnmi.Watch(t, dut, capQ, 30*time.Second, func(v *ygnmi.Value[[]oc.E_BgpTypes_BGP_CAPABILITY]) bool {
			caps, present := v.Val()
			return present && slices.Contains(caps, oc.BgpTypes_BGP_CAPABILITY_ADD_PATHS)
		}).Await(t)
		if !ok {
			got, present := gnmi.Lookup(t, dut, capQ).Val()
			t.Errorf("%s STEP 2/4 FAIL: neighbor %s supported-capabilities: got %v (present: %v), want ADD_PATHS included", tag, rxNbr, got, present)
		} else {
			gotCaps, _ := gnmi.Lookup(t, dut, capQ).Val()
			t.Logf("%s STEP 2/4 PASS: neighbor %s supported-capabilities includes ADD_PATHS (%v)", tag, rxNbr, gotCaps)
		}
	} else {
		capVal, capPresent := gnmi.Lookup(t, dut, capQ).Val()
		t.Logf("%s INFO: neighbor %s supported-capabilities = %v (present: %v)", tag, rxNbr, capVal, capPresent)
	}
}

type prefixKey struct {
	addr string
	plen uint32
}

func (k prefixKey) String() string {
	return fmt.Sprintf("%s/%d", k.addr, k.plen)
}

type learnedRoute struct {
	addr   string
	plen   uint32
	pathID uint32
}

func parsePrefix(t *testing.T, prefix string) prefixKey {
	t.Helper()
	addr, plenStr, ok := strings.Cut(prefix, "/")
	if !ok {
		t.Fatalf("Invalid prefix format: %s", prefix)
	}
	plen, err := strconv.Atoi(plenStr)
	if err != nil {
		t.Fatalf("Invalid prefix length in %s: %v", prefix, err)
	}
	return prefixKey{addr: addr, plen: uint32(plen)}
}

func watchAndLookupPort2RoutesV4(t *testing.T, otg *otg.OTG, peerName string, wantSet map[prefixKey]bool) ([]learnedRoute, bool) {
	t.Helper()
	seen := make(map[prefixKey]bool)
	q := gnmi.OTG().BgpPeer(peerName).UnicastIpv4PrefixAny().State()
	_, ok := gnmi.WatchAll(t, otg, q, 2*time.Minute, func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv4Prefix]) bool {
		prefix, present := v.Val()
		if !present {
			return false
		}
		k := prefixKey{addr: prefix.GetAddress(), plen: prefix.GetPrefixLength()}
		if wantSet[k] {
			seen[k] = true
		}
		return len(seen) == len(wantSet)
	}).Await(t)
	if !ok {
		return nil, false
	}
	snaps := gnmi.LookupAll(t, otg, q)
	var routes []learnedRoute
	for _, s := range snaps {
		if val, present := s.Val(); present {
			routes = append(routes, learnedRoute{
				addr:   val.GetAddress(),
				plen:   val.GetPrefixLength(),
				pathID: val.GetPathId(),
			})
		}
	}
	return routes, true
}

func watchAndLookupPort2RoutesV6(t *testing.T, otg *otg.OTG, peerName string, wantSet map[prefixKey]bool) ([]learnedRoute, bool) {
	t.Helper()
	seen := make(map[prefixKey]bool)
	q := gnmi.OTG().BgpPeer(peerName).UnicastIpv6PrefixAny().State()
	_, ok := gnmi.WatchAll(t, otg, q, 2*time.Minute, func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv6Prefix]) bool {
		prefix, present := v.Val()
		if !present {
			return false
		}
		k := prefixKey{addr: prefix.GetAddress(), plen: prefix.GetPrefixLength()}
		if wantSet[k] {
			seen[k] = true
		}
		return len(seen) == len(wantSet)
	}).Await(t)
	if !ok {
		return nil, false
	}
	snaps := gnmi.LookupAll(t, otg, q)
	var routes []learnedRoute
	for _, s := range snaps {
		if val, present := s.Val(); present {
			routes = append(routes, learnedRoute{
				addr:   val.GetAddress(),
				plen:   val.GetPrefixLength(),
				pathID: val.GetPathId(),
			})
		}
	}
	return routes, true
}

func verifyOTGPort2Routes(t *testing.T, otg *otg.OTG, tag, peerName string, isV4 bool, prefixes []string, wantPathsPerPrefix int) {
	t.Helper()
	wantSet := make(map[prefixKey]bool, len(prefixes))
	var wantList []prefixKey
	for _, p := range prefixes {
		k := parsePrefix(t, p)
		wantSet[k] = true
		wantList = append(wantList, k)
	}

	var routes []learnedRoute
	var ok bool
	if isV4 {
		routes, ok = watchAndLookupPort2RoutesV4(t, otg, peerName, wantSet)
	} else {
		routes, ok = watchAndLookupPort2RoutesV6(t, otg, peerName, wantSet)
	}
	if !ok {
		t.Errorf("%s STEP 4/4 FAIL: timed out waiting for all expected prefixes on ATE port-2 peer %s", tag, peerName)
		return
	}

	counts := make(map[prefixKey]map[uint32]bool)
	for _, r := range routes {
		k := prefixKey{addr: r.addr, plen: r.plen}
		if counts[k] == nil {
			counts[k] = make(map[uint32]bool)
		}
		counts[k][r.pathID] = true
	}

	failed := false
	for _, k := range wantList {
		pathMap := counts[k]
		if len(pathMap) != wantPathsPerPrefix {
			var pathIDs []uint32
			for pid := range pathMap {
				pathIDs = append(pathIDs, pid)
			}
			slices.Sort(pathIDs)
			t.Errorf("%s STEP 4/4 FAIL: ATE port-2 peer %s prefix %s has %d distinct path(s) %v, want %d",
				tag, peerName, k, len(pathMap), pathIDs, wantPathsPerPrefix)
			failed = true
		}
	}

	var extraPrefixes []string
	for k := range counts {
		if !wantSet[k] {
			extraPrefixes = append(extraPrefixes, k.String())
		}
	}
	if len(extraPrefixes) > 0 {
		slices.Sort(extraPrefixes)
		t.Logf("%s INFO: ATE port-2 peer %s ignored %d non-test DUT/link prefixes: %v",
			tag, peerName, len(extraPrefixes), extraPrefixes)
	}

	if !failed {
		totalExpected := len(wantList) * wantPathsPerPrefix
		t.Logf("%s STEP 4/4 PASS: ATE port-2 peer %s received %d paths (%d prefixes x %d distinct path each)",
			tag, peerName, totalExpected, len(wantList), wantPathsPerPrefix)
	}
}

func TestBGP(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	tests := []struct {
		desc, fullDesc string
		skipReason     string
		dut            dutData
		ate            ateData
		wantPrefixes   []ip
	}{{
		desc:     "propagate IPv4 over IPv4",
		fullDesc: "Advertise prefixes from ATE port1, observe received prefixes at ATE port2",
		dut: dutData{bgpOC: &oc.NetworkInstance_Protocol_Bgp{
			PeerGroup: map[string]*oc.NetworkInstance_Protocol_Bgp_PeerGroup{
				"BGP-PEER-GROUP1": getPeerGroup("BGP-PEER-GROUP1", oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, dut),
				"BGP-PEER-GROUP2": getPeerGroup("BGP-PEER-GROUP2", oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, dut),
			},
			Global: &oc.NetworkInstance_Protocol_Bgp_Global{
				As:       ygot.Uint32(dutAS),
				RouterId: ygot.String(dutPort2.IPv4),
				AfiSafi: map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Global_AfiSafi{
					oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST: {
						AfiSafiName: oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST,
						Enabled:     ygot.Bool(true),
					},
				},
			},
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"192.0.2.2": {
					PeerAs:          ygot.Uint32(ateAS1),
					NeighborAddress: ygot.String("192.0.2.2"),
					PeerGroup:       ygot.String("BGP-PEER-GROUP1"),
					AfiSafi: map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
						oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST: {
							AfiSafiName: oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST,
							Enabled:     ygot.Bool(true),
						},
					},
				},
				"192.0.2.6": {
					PeerAs:          ygot.Uint32(ateAS2),
					NeighborAddress: ygot.String("192.0.2.6"),
					PeerGroup:       ygot.String("BGP-PEER-GROUP2"),
					AfiSafi: map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
						oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST: {
							AfiSafiName: oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST,
							Enabled:     ygot.Bool(true),
						},
					},
				},
			},
		}},
		ate: ateData{
			Port1:         ip{v4: "192.0.2.2/30"},
			Port1Neighbor: dutPort1.IPv4,
			Port2:         ip{v4: "192.0.2.6/30"},
			Port2Neighbor: dutPort2.IPv4,
			prefixesStart: ip{v4: "198.51.100.0/32"},
			prefixesCount: 4,
		},
		wantPrefixes: []ip{
			{v4: "198.51.100.0/32"},
			{v4: "198.51.100.1/32"},
			{v4: "198.51.100.2/32"},
			{v4: "198.51.100.3/32"},
		},
	}, {
		desc:     "propagate IPv6 over IPv6",
		fullDesc: "Advertise IPv6 prefixes from ATE port1, observe received prefixes at ATE port2",
		dut: dutData{bgpOC: &oc.NetworkInstance_Protocol_Bgp{
			PeerGroup: map[string]*oc.NetworkInstance_Protocol_Bgp_PeerGroup{
				"BGP-PEER-GROUP1": getPeerGroup("BGP-PEER-GROUP1", oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, dut),
				"BGP-PEER-GROUP2": getPeerGroup("BGP-PEER-GROUP2", oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, dut),
			},
			Global: &oc.NetworkInstance_Protocol_Bgp_Global{
				As:       ygot.Uint32(dutAS),
				RouterId: ygot.String(dutPort2.IPv4),
				AfiSafi: map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Global_AfiSafi{
					oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST: {
						AfiSafiName: oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST,
						Enabled:     ygot.Bool(true),
					},
				},
			},
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"2001:db8::2": {
					PeerAs:          ygot.Uint32(ateAS1),
					PeerGroup:       ygot.String("BGP-PEER-GROUP1"),
					NeighborAddress: ygot.String("2001:db8::2"),
					AfiSafi: map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
						oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST: {
							AfiSafiName: oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST,
							Enabled:     ygot.Bool(true),
						},
					},
				},
				"2001:db8::6": {
					PeerAs:          ygot.Uint32(ateAS2),
					PeerGroup:       ygot.String("BGP-PEER-GROUP2"),
					NeighborAddress: ygot.String("2001:db8::6"),
					AfiSafi: map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
						oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST: {
							AfiSafiName: oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST,
							Enabled:     ygot.Bool(true),
						},
					},
				},
			},
		}},
		ate: ateData{
			Port1:         ip{v6: "2001:db8::2/126"},
			Port1Neighbor: dutPort1.IPv6,
			Port2:         ip{v6: "2001:db8::6/126"},
			Port2Neighbor: dutPort2.IPv6,
			prefixesStart: ip{v6: "2001:db8:1::1/128"},
			prefixesCount: 4,
		},
		wantPrefixes: []ip{
			{v6: "2001:db8:1::1/128"},
			{v6: "2001:db8:1::2/128"},
			{v6: "2001:db8:1::3/128"},
			{v6: "2001:db8:1::4/128"},
		},
	}, {
		desc:       "propagate IPv4 over IPv6",
		skipReason: "TODO: RFC5549 needs to be enabled explicitly and OpenConfig does not currently provide a signal.",
		fullDesc:   "IPv4 routes with an IPv6 next-hop when negotiating RFC5549 - validating that routes are accepted and advertised with the specified values.",
		dut: dutData{bgpOC: &oc.NetworkInstance_Protocol_Bgp{
			PeerGroup: map[string]*oc.NetworkInstance_Protocol_Bgp_PeerGroup{
				"BGP-PEER-GROUP1": getPeerGroup("BGP-PEER-GROUP1", oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, dut),
				"BGP-PEER-GROUP2": getPeerGroup("BGP-PEER-GROUP2", oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, dut),
			},
			Global: &oc.NetworkInstance_Protocol_Bgp_Global{
				As:       ygot.Uint32(dutAS),
				RouterId: ygot.String(dutPort2.IPv4),
				AfiSafi: map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Global_AfiSafi{
					oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST: {
						AfiSafiName: oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST,
						Enabled:     ygot.Bool(true),
					},
				},
			},
			Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
				"2001:db8::2": {
					PeerAs:          ygot.Uint32(ateAS1),
					PeerGroup:       ygot.String("BGP-PEER-GROUP1"),
					NeighborAddress: ygot.String("2001:db8::2"),
					AfiSafi: map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
						oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST: {
							AfiSafiName: oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST,
							Enabled:     ygot.Bool(true),
						},
					},
				},
				"192.0.2.6": {
					PeerAs:          ygot.Uint32(ateAS2),
					PeerGroup:       ygot.String("BGP-PEER-GROUP2"),
					NeighborAddress: ygot.String("192.0.2.6"),
					AfiSafi: map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
						oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST: {
							AfiSafiName: oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST,
							Enabled:     ygot.Bool(true),
						},
					},
				},
			},
		}},
		ate: ateData{
			Port1:         ip{v6: "2001:db8::2/126"},
			Port1Neighbor: dutPort1.IPv6,
			Port2:         ip{v4: "192.0.2.6/30"},
			Port2Neighbor: dutPort2.IPv4,
			prefixesStart: ip{v4: "198.51.100.0/32"},
			prefixesCount: 4,
		},
		wantPrefixes: []ip{
			{v4: "198.51.100.0/32"},
			{v4: "198.51.100.1/32"},
			{v4: "198.51.100.2/32"},
			{v4: "198.51.100.3/32"},
		},
	}}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			t.Log(tc.fullDesc)

			if tc.skipReason != "" {
				t.Skip(tc.skipReason)
			}

			tc.dut.Configure(t, dut)

			ate := ondatra.ATE(t, "ate")

			otg := ate.OTG()
			ateList := []string{"port1", "port2"}
			otgConfig := tc.ate.ConfigureOTG(t, otg, ateList)

			t.Logf("Verify DUT BGP sessions up")
			tc.dut.AwaitBGPEstablished(t, dut)
			t.Logf("Verify OTG BGP sessions up")
			verifyOTGBGPTelemetry(t, otg, otgConfig, "ESTABLISHED")

			for _, prefix := range tc.wantPrefixes {
				var expectedOTGBGPPrefix OTGBGPPrefix
				if prefix.v4 != "" {
					t.Logf("Checking for BGP Prefix %v", prefix.v4)
					addr := strings.Split(prefix.v4, "/")[0]
					prefixLen, _ := strconv.Atoi(strings.Split(prefix.v4, "/")[1])
					if tc.ate.Port2.v4 != "" {
						expectedOTGBGPPrefix = OTGBGPPrefix{PeerName: "port2.dev.BGP4.peer", Address: addr, PrefixLength: uint32(prefixLen)}
					} else {
						expectedOTGBGPPrefix = OTGBGPPrefix{PeerName: "port2.dev.BGP6.peer", Address: addr, PrefixLength: uint32(prefixLen)}
					}
					if !checkOTGBGP4Prefix(t, otg, otgConfig, expectedOTGBGPPrefix) {
						t.Errorf("Prefix %v is not being learned", expectedOTGBGPPrefix.Address)
					}
				}
				if prefix.v6 != "" {
					t.Logf("Checking for BGP Prefix %v", prefix.v6)
					addr := strings.Split(prefix.v6, "/")[0]
					prefixLen, _ := strconv.Atoi(strings.Split(prefix.v6, "/")[1])
					expectedOTGBGPPrefix = OTGBGPPrefix{PeerName: "port2.dev.BGP6.peer", Address: addr, PrefixLength: uint32(prefixLen)}
					if !checkOTGBGP6Prefix(t, otg, otgConfig, expectedOTGBGPPrefix) {
						t.Errorf("Prefix %v is not being learned", expectedOTGBGPPrefix.Address)
					}
				}
			}
		})
	}
}

// TestAddPath implements RT-1.3.3 and RT-1.3.5: ADD-Path receive enabled and disabled on the neighbor and on the
// peer-group, and the neighbor value overriding the peer-group value in both directions.
func TestAddPath(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	defaultNI := deviations.DefaultNetworkInstance(dut)
	explicitPortSpeed := deviations.ExplicitPortSpeed(dut)
	explicitIfInDefaultVRF := deviations.ExplicitInterfaceInDefaultVRF(dut)
	routePolicyUnderAFIUnsupported := deviations.RoutePolicyUnderAFIUnsupported(dut)

	afs := []struct {
		name     string
		afi      oc.E_BgpTypes_AFI_SAFI_TYPE
		isV4     bool
		rxNbr    string
		txNbr    string
		ate      ateData
		prefixes []string
	}{
		{
			name:  "IPv4",
			afi:   oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST,
			isV4:  true,
			rxNbr: "192.0.2.2",
			txNbr: "192.0.2.6",
			ate: ateData{
				Port1:         ip{v4: "192.0.2.2/30"},
				Port1Neighbor: dutPort1.IPv4,
				Port2:         ip{v4: "192.0.2.6/30"},
				Port2Neighbor: dutPort2.IPv4,
				prefixesStart: ip{v4: "198.51.100.0/32"},
				prefixesCount: 4,
				pathIDs:       []uint32{1, 2, 3, 4},
			},
			prefixes: []string{
				"198.51.100.0/32",
				"198.51.100.1/32",
				"198.51.100.2/32",
				"198.51.100.3/32",
			},
		},
		{
			name:  "IPv6",
			afi:   oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST,
			isV4:  false,
			rxNbr: "2001:db8::2",
			txNbr: "2001:db8::6",
			ate: ateData{
				Port1:         ip{v6: "2001:db8::2/126"},
				Port1Neighbor: dutPort1.IPv6,
				Port2:         ip{v6: "2001:db8::6/126"},
				Port2Neighbor: dutPort2.IPv6,
				prefixesStart: ip{v6: "2001:db8:1::1/128"},
				prefixesCount: 4,
				pathIDs:       []uint32{1, 2, 3, 4},
			},
			prefixes: []string{
				"2001:db8:1::1/128",
				"2001:db8:1::2/128",
				"2001:db8:1::3/128",
				"2001:db8:1::4/128",
			},
		},
	}

	cases := []struct {
		id   string
		name string
		desc string
		cfg  addPathCfg
	}{
		{
			id:   "RT-1.3.3.1",
			name: "RT-1.3.3.1 receive on neighbor",
			desc: "neighbor add-paths receive=true (peer-group unset)",
			cfg: addPathCfg{
				nbrReceive: ygot.Bool(true),
			},
		},
		{
			id:   "RT-1.3.3.2",
			name: "RT-1.3.3.2 receive on peer-group",
			desc: "peer-group BGP-PEER-GROUP1 receive=true (neighbor AFI-SAFI unset; inherited)",
			cfg: addPathCfg{
				pgReceive: ygot.Bool(true),
			},
		},
		{
			id:   "RT-1.3.3.3",
			name: "RT-1.3.3.3 peer-group receive overridden off on neighbor",
			desc: "peer-group receive=true overridden OFF by neighbor receive=false",
			cfg: addPathCfg{
				pgReceive:  ygot.Bool(true),
				nbrReceive: ygot.Bool(false),
			},
		},
		{
			id:   "RT-1.3.3.4",
			name: "RT-1.3.3.4 peer-group receive overridden on by neighbor",
			desc: "peer-group receive=false overridden ON by neighbor receive=true",
			cfg: addPathCfg{
				pgReceive:  ygot.Bool(false),
				nbrReceive: ygot.Bool(true),
			},
		},
		{
			id:   "RT-1.3.5.1",
			name: "RT-1.3.5.1 receive disabled on neighbor",
			desc: "neighbor add-paths receive=false (peer-group unset; baseline disable)",
			cfg: addPathCfg{
				nbrReceive: ygot.Bool(false),
			},
		},
		{
			id:   "RT-1.3.5.2",
			name: "RT-1.3.5.2 receive disabled on peer-group",
			desc: "peer-group BGP-PEER-GROUP1 receive=false (neighbor AFI-SAFI unset; negative control)",
			cfg: addPathCfg{
				pgReceive: ygot.Bool(false),
			},
		},
	}

	for _, af := range afs {
		for _, tc := range cases {
			t.Run(af.name+" "+tc.name, func(t *testing.T) {
				tag := fmt.Sprintf("[%s %s]", tc.id, af.name)
				receiveOn := (tc.cfg.nbrReceive != nil && *tc.cfg.nbrReceive) || (tc.cfg.nbrReceive == nil && tc.cfg.pgReceive != nil && *tc.cfg.pgReceive)
				wantRcv := af.ate.prefixesCount
				if receiveOn {
					wantRcv = af.ate.prefixesCount * uint32(len(af.ate.routePathIDs()))
				}
				t.Logf("%s START | Config: %s | Expect: effective receive=%t, ADD_PATHS=%t, DUT(%s) rcv=%d, ATE port-2=4 paths (1/prefix)", tag, tc.desc, receiveOn, receiveOn, af.rxNbr, wantRcv)

				t.Cleanup(func() {
					gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(defaultNI).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config())
				})
				d := dutData{
					bgpOC:                         addPathBGP(af.afi, af.rxNbr, af.txNbr, tc.cfg, routePolicyUnderAFIUnsupported),
					ni:                            defaultNI,
					explicitPortSpeed:             &explicitPortSpeed,
					explicitInterfaceInDefaultVRF: &explicitIfInDefaultVRF,
				}
				d.Configure(t, dut)
				otg := ondatra.ATE(t, "ate").OTG()
				ate := af.ate
				ate.port1IBGP = true
				otgConfig := ate.ConfigureOTG(t, otg, []string{"port1", "port2"})
				d.AwaitBGPEstablished(t, dut)
				verifyOTGBGPTelemetry(t, otg, otgConfig, "ESTABLISHED")
				t.Logf("%s STEP 1/4 PASS: BGP sessions ESTABLISHED (rxNbr=%s, txNbr=%s)", tag, af.rxNbr, af.txNbr)
				verifyAddPathState(t, dut, defaultNI, tag, af.afi, af.rxNbr, tc.cfg)

				// Primary check: the number of paths the DUT holds from ATE port-1.
				// With ADD-Path receive enabled the DUT keeps every path ID for each
				// prefix; with it disabled each new path implicitly replaces the
				// previous one, so the DUT keeps one path per prefix.
				rcvQ := gnmi.OC().NetworkInstance(defaultNI).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Neighbor(af.rxNbr).AfiSafi(af.afi).Prefixes().Received().State()
				if _, ok := gnmi.Watch(t, dut, rcvQ, time.Minute, func(v *ygnmi.Value[uint32]) bool {
					got, present := v.Val()
					return present && got == wantRcv
				}).Await(t); !ok {
					got, present := gnmi.Lookup(t, dut, rcvQ).Val()
					t.Errorf("%s STEP 3/4 FAIL: DUT neighbor %s received prefixes: got %v (present: %v), want %d", tag, af.rxNbr, got, present, wantRcv)
				} else {
					t.Logf("%s STEP 3/4 PASS: DUT neighbor %s received prefixes = %d (want %d)", tag, af.rxNbr, wantRcv, wantRcv)
				}

				peer := "port2.dev.BGP6.peer"
				if af.isV4 {
					peer = "port2.dev.BGP4.peer"
				}
				verifyOTGPort2Routes(t, otg, tag, peer, af.isV4, af.prefixes, 1)
			})
		}
	}
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
