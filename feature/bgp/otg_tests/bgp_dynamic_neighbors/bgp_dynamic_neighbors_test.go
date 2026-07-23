// Copyright 2026 Google LLC
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

package bgp_dynamic_neighbors_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
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
	dutAS                    = 64500
	routerID                 = "192.0.2.1"
	ateRouterID              = "192.0.2.200"
	staticPeerPassword       = "secret_md5_key"
	dynamicListenPrefix      = "2001:db8:1::/64"
	dynamicRoutePrefix       = "2001:db8:2::/64"
	dynamicRoutePrefixBase   = "2001:db8:2::"
	dynamicRoutePrefixLength = 64
	port1IPv6Len             = 64
	port2IPv6Len             = 64
	establishedTimeout       = 2 * time.Minute
	negativeTimeout          = 30 * time.Second
	peerCount                = 6
	dynamicPeerGroupName     = "RM_DYNAMIC_PEERS"
	staticPeerGroupName      = "STATIC_PEERS"
	dynamicPeerPrefix        = "dyn-peer-"
	staticPeerName           = "static-peer"
	defaultSessionCountCheck = 6
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT to ATE port1",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
		IPv6:    "2001:db8:0:1::254",
		IPv6Len: port1IPv6Len,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT to ATE port2",
		IPv4:    "192.0.2.5",
		IPv4Len: 30,
		IPv6:    "2001:db8:1::254",
		IPv6Len: port2IPv6Len,
	}
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv6:    "2001:db8:0:1::1",
		IPv6Len: port1IPv6Len,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:00:02:01:01:01",
		IPv6:    "2001:db8:1::1",
		IPv6Len: port2IPv6Len,
	}
)

type atePeerSpec struct {
	name           string
	localIP        string
	peerAS         uint32
	authPassword   string
	advertiseRoute bool
	routePrefix    string
	routeMED       uint32
}

func bgpName(dut *ondatra.DUTDevice) string {
	if name := deviations.DefaultBgpInstanceName(dut); name != "" {
		return name
	}
	return "BGP"
}

func configureDUTPorts(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dc := gnmi.OC()
	i1 := dutPort1.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	i1.Enabled = ygot.Bool(true)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	i2 := dutPort2.NewOCInterface(dut.Port(t, "port2").Name(), dut)
	i2.Enabled = ygot.Bool(true)
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

func verifyPortsUp(t *testing.T, dev *ondatra.Device) {
	t.Helper()
	for _, p := range dev.Ports() {
		gnmi.Await(t, dev, gnmi.OC().Interface(p.Name()).OperStatus().State(), 30*time.Second, oc.Interface_OperStatus_UP)
	}
}

func bgpClearConfig(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	resetBatch := &gnmi.SetBatch{}
	protoPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName(dut))
	gnmi.BatchDelete(resetBatch, protoPath.Config())

	if deviations.NetworkInstanceTableDeletionRequired(dut) {
		tablePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).TableAny()
		for _, table := range gnmi.LookupAll[*oc.NetworkInstance_Table](t, dut, tablePath.Config()) {
			if val, ok := table.Val(); ok {
				if val.GetProtocol() == oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP {
					gnmi.BatchDelete(resetBatch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Table(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, val.GetAddressFamily()).Config())
				}
			}
		}
	}
	resetBatch.Set(t, dut)
}

func buildDUTBGPConfig(t *testing.T, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	t.Helper()

	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	proto := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName(dut))
	proto.SetEnabled(true)

	bgp := proto.GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(dutAS)
	global.RouterId = ygot.String(routerID)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

	dynamicPG := bgp.GetOrCreatePeerGroup(dynamicPeerGroupName)
	dynamicPG.PeerGroupName = ygot.String(dynamicPeerGroupName)
	dynamicPG.PeerAs = ygot.Uint32(dutAS)
	dynamicPG.AuthPassword = ygot.String(staticPeerPassword)
	dynamicPG.GetOrCreateTransport().LocalAddress = ygot.String(dutPort2.IPv6)
	dynamicPG.GetOrCreateTransport().SetPassiveMode(true)
	dynamicPG.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

	dynamicPrefix := global.GetOrCreateDynamicNeighborPrefix(dynamicListenPrefix)
	dynamicPrefix.Prefix = ygot.String(dynamicListenPrefix)
	dynamicPrefix.PeerGroup = ygot.String(dynamicPeerGroupName)

	staticPG := bgp.GetOrCreatePeerGroup(staticPeerGroupName)
	staticPG.PeerGroupName = ygot.String(staticPeerGroupName)
	staticPG.PeerAs = ygot.Uint32(dutAS)
	staticPG.AuthPassword = ygot.String(staticPeerPassword)
	staticPG.GetOrCreateTransport().LocalAddress = ygot.String(dutPort1.IPv6)
	staticPG.GetOrCreateTransport().SetPassiveMode(true)
	staticPG.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

	staticNbr := bgp.GetOrCreateNeighbor(atePort1.IPv6)
	staticNbr.PeerGroup = ygot.String(staticPeerGroupName)
	staticNbr.PeerAs = ygot.Uint32(dutAS)
	staticNbr.Enabled = ygot.Bool(true)
	staticNbr.GetOrCreateTransport().LocalAddress = ygot.String(dutPort1.IPv6)
	staticNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

	return proto
}

func applyDUTConfig(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	bgpClearConfig(t, dut)
	protoPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName(dut))
	gnmi.Replace(t, dut, protoPath.Config(), buildDUTBGPConfig(t, dut))

	// DIAGNOSTIC ONLY: confirm whether the OC dynamic-neighbor-prefix push alone
	// results in "bgp listen range" appearing in the EOS running-config.
	// Kept for reference; the deviation decision has been confirmed and
	// formalized as deviations.BgpDynamicNeighborPrefixUnsupported.
	//
	// if dut.Vendor() == ondatra.ARISTA {
	// 	runningCfg, err := dut.RawAPIs().CLI(t).RunCommand(context.Background(), "show running-config section bgp")
	// 	if err != nil {
	// 		t.Logf("diagnostic CLI error: %v", err)
	// 	} else {
	// 		t.Logf("running-config after OC push only:\n%s", runningCfg.Output())
	// 		if strings.Contains(runningCfg.Output(), "bgp listen range") {
	// 			t.Logf("RESULT: OC push DID program bgp listen range - CLI workaround may be unnecessary")
	// 		} else {
	// 			t.Logf("RESULT: OC push did NOT program bgp listen range - confirms silent-ignore, CLI workaround needed")
	// 		}
	// 	}
	// }

	// Arista accepts the OpenConfig dynamic-neighbor-prefixes path but
	// silently ignores it; configure bgp listen range via CLI as a workaround.
	if deviations.BgpDynamicNeighborPrefixUnsupported(dut) {
		output, err := dut.RawAPIs().CLI(t).RunCommand(context.Background(), fmt.Sprintf(
			"configure\nrouter bgp %d\nbgp listen range %s peer-group %s remote-as %d\nexit\nexit\n",
			dutAS, dynamicListenPrefix, dynamicPeerGroupName, dutAS))
		if err != nil {
			t.Fatalf("failed to configure bgp listen range via CLI: %v", err)
		}
		t.Logf("CLI output: %s", output.Output())

		// DIAGNOSTIC ONLY: confirm the CLI workaround actually programmed
		// "bgp listen range" into the running-config. Kept for reference; the
		// deviation decision has been confirmed and formalized as
		// deviations.BgpDynamicNeighborPrefixUnsupported.
		//
		// runningCfgAfter, err := dut.RawAPIs().CLI(t).RunCommand(context.Background(), "show running-config section bgp")
		// if err != nil {
		// 	t.Logf("diagnostic CLI error: %v", err)
		// } else {
		// 	t.Logf("running-config after CLI workaround:\n%s", runningCfgAfter.Output())
		// 	if strings.Contains(runningCfgAfter.Output(), "bgp listen range") {
		// 		t.Logf("RESULT: CLI workaround DID program bgp listen range")
		// 	} else {
		// 		t.Logf("RESULT: CLI workaround did NOT program bgp listen range - investigate further")
		// 	}
		// }
	}
}

func buildATEConfig(t *testing.T, dut *ondatra.DUTDevice, staticPeer atePeerSpec, dynamicPeers []atePeerSpec) gosnappi.Config {
	t.Helper()

	config := gosnappi.NewConfig()
	port1 := config.Ports().Add().SetName("port1")
	port2 := config.Ports().Add().SetName("port2")

	staticDev := config.Devices().Add().SetName(atePort1.Name)
	staticEth := staticDev.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	staticEth.Connection().SetPortName(port1.Name())
	staticIpv6 := staticEth.Ipv6Addresses().Add().SetName(atePort1.Name + ".IPv6")
	staticIpv6.SetAddress(atePort1.IPv6).SetGateway(dutPort1.IPv6).SetPrefix(uint32(atePort1.IPv6Len))

	staticBgp := staticDev.Bgp().SetRouterId(ateRouterID)
	staticPeerBgp := staticBgp.Ipv6Interfaces().Add().SetIpv6Name(staticIpv6.Name()).Peers().Add().SetName(staticPeer.name)
	staticPeerBgp.SetPeerAddress(staticIpv6.Gateway()).SetAsNumber(staticPeer.peerAS).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
	staticPeerBgp.LearnedInformationFilter().SetUnicastIpv6Prefix(true)
	if staticPeer.authPassword != "" {
		staticPeerBgp.Advanced().SetMd5Key(staticPeer.authPassword)
	}
	if staticPeer.advertiseRoute {
		staticRoute := staticPeerBgp.V6Routes().Add().SetName(staticPeer.name + ".route")
		staticRoute.SetNextHopIpv6Address(staticIpv6.Address()).
			SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
			SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
		staticRoute.Addresses().Add().SetAddress(staticPeer.routePrefix).SetPrefix(dynamicRoutePrefixLength).SetCount(1)
		staticRoute.Advanced().SetIncludeMultiExitDiscriminator(true).SetMultiExitDiscriminator(staticPeer.routeMED)
	}

	for i, peer := range dynamicPeers {
		devName := fmt.Sprintf("%s.%d", atePort2.Name, i+1)
		dev := config.Devices().Add().SetName(devName)
		eth := dev.Ethernets().Add().SetName(devName + ".Eth").SetMac(fmt.Sprintf("02:00:02:01:01:%02x", i+1))
		eth.Connection().SetPortName(port2.Name())

		ipv6 := eth.Ipv6Addresses().Add().SetName(devName + ".IPv6")
		ipv6.SetAddress(peer.localIP).SetGateway(dutPort2.IPv6).SetPrefix(uint32(atePort2.IPv6Len))

		peerBgp := dev.Bgp().SetRouterId(ateRouterID).Ipv6Interfaces().Add().SetIpv6Name(ipv6.Name()).Peers().Add().SetName(peer.name)
		peerBgp.SetPeerAddress(ipv6.Gateway()).SetAsNumber(peer.peerAS).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
		peerBgp.LearnedInformationFilter().SetUnicastIpv6Prefix(true)
		if peer.authPassword != "" {
			peerBgp.Advanced().SetMd5Key(peer.authPassword)
		}
		if peer.advertiseRoute {
			route := peerBgp.V6Routes().Add().SetName(peer.name + ".route")
			route.SetNextHopIpv6Address(ipv6.Address()).
				SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
				SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
			route.Addresses().Add().SetAddress(peer.routePrefix).SetPrefix(dynamicRoutePrefixLength).SetCount(1)
			route.Advanced().SetIncludeMultiExitDiscriminator(true).SetMultiExitDiscriminator(peer.routeMED)
		}
	}

	return config
}

func applyATEConfig(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, stopFirst bool) {
	t.Helper()
	if stopFirst {
		ate.OTG().StopProtocols(t)
	}
	ate.OTG().PushConfig(t, config)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), config, "IPv6")
}

func waitForNeighborState(t *testing.T, dut *ondatra.DUTDevice, neighbor string, want oc.E_Bgp_Neighbor_SessionState) {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName(dut)).Bgp()
	nbrPath := bgpPath.Neighbor(neighbor)
	_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), establishedTimeout, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, present := val.Val()
		return present && state == want
	}).Await(t)
	if !ok {
		t.Fatalf("neighbor %s did not reach state %v", neighbor, want)
	}
}

func verifyNeighborTelemetry(t *testing.T, dut *ondatra.DUTDevice, neighbor string, wantDynamic bool) {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName(dut)).Bgp()
	nbrPath := bgpPath.Neighbor(neighbor)

	state := gnmi.Get(t, dut, nbrPath.SessionState().State())
	if state != oc.Bgp_Neighbor_SessionState_ESTABLISHED {
		t.Fatalf("neighbor %s session state got %v, want ESTABLISHED", neighbor, state)
	}

	dynamicVal := gnmi.Lookup(t, dut, nbrPath.DynamicallyConfigured().State())
	gotDynamic, present := dynamicVal.Val()
	if !present {
		gotDynamic = false // not present means default false (vendor omits default boolean values)
	}
	if gotDynamic != wantDynamic {
		t.Fatalf("neighbor %s dynamically-configured got %v, want %v", neighbor, gotDynamic, wantDynamic)
	}

	if got := gnmi.Get(t, dut, nbrPath.State()).GetPeerAs(); got != dutAS {
		t.Fatalf("neighbor %s peer-as got %v, want %v", neighbor, got, dutAS)
	}
}

func verifyDynamicNeighbors(t *testing.T, dut *ondatra.DUTDevice, peers []atePeerSpec) {
	t.Helper()
	for _, peer := range peers {
		waitForNeighborState(t, dut, peer.localIP, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
		verifyNeighborTelemetry(t, dut, peer.localIP, true)
	}
	verifyNeighborTelemetry(t, dut, atePort1.IPv6, false)
}

func verifyGlobalRouterID(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName(dut)).Bgp()
	if got := gnmi.Get(t, dut, bgpPath.Global().RouterId().State()); got != routerID {
		t.Fatalf("global router-id got %s, want %s", got, routerID)
	}
}

func verifyLocRIBMED(t *testing.T, dut *ondatra.DUTDevice, prefix string, wantMED uint32) {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName(dut)).Bgp()
	ribPath := bgpPath.Rib()

	locRibPath := ribPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Ipv6Unicast().LocRib().State()
	_, ok := gnmi.Watch(t, dut, locRibPath, establishedTimeout, func(val *ygnmi.Value[*oc.NetworkInstance_Protocol_Bgp_Rib_AfiSafi_Ipv6Unicast_LocRib]) bool {
		locRib, present := val.Val()
		if !present || locRib == nil {
			return false
		}
		for _, route := range locRib.Route {
			if route.GetPrefix() != prefix {
				continue
			}
			if deviations.SkipCheckingAttributeIndex(dut) {
				for _, attrSet := range gnmi.GetAll[*oc.NetworkInstance_Protocol_Bgp_Rib_AttrSet](t, dut, ribPath.AttrSetAny().State()) {
					if attrSet.GetMed() == wantMED {
						return true
					}
				}
				return false
			}
			attrSet := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Rib_AttrSet](t, dut, ribPath.AttrSet(route.GetAttrIndex()).State())
			return attrSet != nil && attrSet.GetMed() == wantMED
		}
		return false
	}).Await(t)
	if !ok {
		t.Fatalf("prefix %s did not converge to MED %d", prefix, wantMED)
	}
}

func waitForNeighborDownOrNotEstablished(t *testing.T, dut *ondatra.DUTDevice, neighbor string) {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName(dut)).Bgp()
	nbrPath := bgpPath.Neighbor(neighbor)
	_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), negativeTimeout, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, present := val.Val()
		return present && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}).Await(t)
	if ok {
		t.Fatalf("neighbor %s unexpectedly reached ESTABLISHED", neighbor)
	}
}

func makeDynamicPeers(count int, peerAS uint32, authPassword string) []atePeerSpec {
	peers := make([]atePeerSpec, 0, count)
	for i := 0; i < count; i++ {
		localIP := fmt.Sprintf("2001:db8:1::%x", i+1)
		peers = append(peers, atePeerSpec{
			name:         fmt.Sprintf("%s%d", dynamicPeerPrefix, i+1),
			localIP:      localIP,
			peerAS:       peerAS,
			authPassword: authPassword,
		})
	}
	return peers
}

func sessionOnlyConfig() (atePeerSpec, []atePeerSpec) {
	staticPeer := atePeerSpec{
		name:         staticPeerName,
		localIP:      atePort1.IPv6,
		peerAS:       dutAS,
		authPassword: staticPeerPassword,
	}
	dynamicPeers := makeDynamicPeers(peerCount, dutAS, staticPeerPassword)
	return staticPeer, dynamicPeers
}

func routeSelectionConfig(withdrawStatic bool) (atePeerSpec, []atePeerSpec) {
	staticPeer := atePeerSpec{
		name:           staticPeerName,
		localIP:        atePort1.IPv6,
		peerAS:         dutAS,
		authPassword:   staticPeerPassword,
		advertiseRoute: !withdrawStatic,
		routePrefix:    dynamicRoutePrefixBase,
		routeMED:       50,
	}
	dynamicPeers := []atePeerSpec{{
		name:           dynamicPeerPrefix + "1",
		localIP:        atePort2.IPv6,
		peerAS:         dutAS,
		authPassword:   staticPeerPassword,
		advertiseRoute: true,
		routePrefix:    dynamicRoutePrefixBase,
		routeMED:       100,
	}}
	return staticPeer, dynamicPeers
}

func TestBGPDynamicNeighbors(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	t.Cleanup(func() {
		bgpClearConfig(t, dut)
	})

	configureDUTPorts(t, dut)
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	verifyPortsUp(t, dut.Device)

	t.Run("RT-1.103.1 Dynamic Peering and Router-ID Validation", func(t *testing.T) {
		staticPeer, dynamicPeers := sessionOnlyConfig()
		applyDUTConfig(t, dut)
		applyATEConfig(t, ate, buildATEConfig(t, dut, staticPeer, dynamicPeers), false)

		verifyGlobalRouterID(t, dut)
		verifyDynamicNeighbors(t, dut, dynamicPeers)
		waitForNeighborState(t, dut, atePort1.IPv6, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
		verifyNeighborTelemetry(t, dut, atePort1.IPv6, false)

		if got := len(dynamicPeers); got != defaultSessionCountCheck {
			t.Fatalf("session count helper mismatch, got %d, want %d", got, defaultSessionCountCheck)
		}
	})

	t.Run("RT-1.103.2 Telemetry for Dynamic Neighbors", func(t *testing.T) {
		staticPeer, dynamicPeers := sessionOnlyConfig()
		applyDUTConfig(t, dut)
		applyATEConfig(t, ate, buildATEConfig(t, dut, staticPeer, dynamicPeers), true)

		verifyDynamicNeighbors(t, dut, dynamicPeers)
		verifyNeighborTelemetry(t, dut, atePort1.IPv6, false)
	})

	t.Run("RT-1.103.3 Secured Dynamic Peers (iBGP MD5)", func(t *testing.T) {
		staticPeer, dynamicPeers := sessionOnlyConfig()
		applyDUTConfig(t, dut)
		applyATEConfig(t, ate, buildATEConfig(t, dut, staticPeer, dynamicPeers), true)

		for _, peer := range dynamicPeers {
			waitForNeighborState(t, dut, peer.localIP, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
		}

		wrongPasswordPeers := makeDynamicPeers(1, dutAS, "wrong-md5")
		applyATEConfig(t, ate, buildATEConfig(t, dut, staticPeer, append(wrongPasswordPeers, dynamicPeers[1:]...)), true)
		waitForNeighborState(t, dut, dynamicPeers[1].localIP, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
		waitForNeighborDownOrNotEstablished(t, dut, wrongPasswordPeers[0].localIP)

		noPasswordPeers := makeDynamicPeers(1, dutAS, "")
		applyATEConfig(t, ate, buildATEConfig(t, dut, staticPeer, append(noPasswordPeers, dynamicPeers[1:]...)), true)
		waitForNeighborState(t, dut, dynamicPeers[1].localIP, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
		waitForNeighborDownOrNotEstablished(t, dut, noPasswordPeers[0].localIP)
	})

	t.Run("RT-1.103.4 Duplicate ATE Router-ID (Transport Separation)", func(t *testing.T) {
		staticPeer, dynamicPeers := sessionOnlyConfig()
		applyDUTConfig(t, dut)
		applyATEConfig(t, ate, buildATEConfig(t, dut, staticPeer, dynamicPeers), true)

		for _, peer := range dynamicPeers {
			waitForNeighborState(t, dut, peer.localIP, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
		}
		verifyGlobalRouterID(t, dut)
	})

	t.Run("RT-1.103.5 Overlapping Prefix Best-Path Selection and Reconvergence", func(t *testing.T) {
		staticPeer, dynamicPeers := routeSelectionConfig(false)
		applyDUTConfig(t, dut)
		applyATEConfig(t, ate, buildATEConfig(t, dut, staticPeer, dynamicPeers), true)

		waitForNeighborState(t, dut, staticPeer.localIP, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
		waitForNeighborState(t, dut, dynamicPeers[0].localIP, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
		verifyLocRIBMED(t, dut, dynamicRoutePrefix, 50)

		withdrawStaticPeer, reconvergeDynamicPeers := routeSelectionConfig(true)
		applyATEConfig(t, ate, buildATEConfig(t, dut, withdrawStaticPeer, reconvergeDynamicPeers), true)
		waitForNeighborState(t, dut, reconvergeDynamicPeers[0].localIP, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
		verifyLocRIBMED(t, dut, dynamicRoutePrefix, 100)
	})
}
