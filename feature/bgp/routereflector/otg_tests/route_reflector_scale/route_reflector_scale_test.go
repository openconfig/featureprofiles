// Copyright 2025 Google LLC
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

package route_reflector_scale_test

import (
	"fmt"
	"net/netip"
	"sort"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/netinstbgp"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	dutAreaAddress     = "49.0001"
	dutSysID           = "1920.0000.2001"
	otgIsisPort2LoopV4 = "203.0.113.10"
	otgIsisPort2LoopV6 = "2001:db8::203:0:113:10"
	otgIsisPort3LoopV4 = "203.0.113.14"
	otgIsisPort3LoopV6 = "2001:db8::203:0:113:14"
	isisInstance       = "DEFAULT"
	otgSysID2          = "640000000002"
	otgSysID3          = "640000000003"
)

var (
	dutLoopback = attrs.Attributes{
		Desc:    "Loopback ip",
		IPv4:    "203.0.113.1",
		IPv6:    "2001:db8::203:0:113:1",
		IPv4Len: 32,
		IPv6Len: 128,
	}

	lb string
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestRouteReflector(t *testing.T) {
	// Configure eBGP session on DUT & ATE port1
	bs := cfgplugins.NewBGPSession(t, cfgplugins.PortCount4, nil)
	configureDUTLoopback(t, bs.DUT)
	bs.WithEBGP(
		t,
		[]oc.E_BgpTypes_AFI_SAFI_TYPE{oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST},
		[]string{"port1"},
		true,
		true,
	)
	dni := deviations.DefaultNetworkInstance(bs.DUT)
	bgp := bs.DUTConf.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(cfgplugins.PTBGP, "BGP").GetOrCreateBgp().GetOrCreateGlobal()
	bgp.SetRouterId(dutLoopback.IPv4)

	configureDUT(t, bs)
	configureOTG(t, bs)

	bs.PushAndStart(t)

	t.Run("Verify port status on DUT", func(t *testing.T) {
		verifyPortsUp(t, bs.DUT.Device)
	})

	dutIsisIntfNames := []string{bs.OndatraDUTPorts[1].Name(), bs.OndatraDUTPorts[2].Name(), lb}
	if deviations.ExplicitInterfaceInDefaultVRF(bs.DUT) {
		dutIsisIntfNames = []string{bs.OndatraDUTPorts[1].Name() + ".0", bs.OndatraDUTPorts[2].Name() + ".0", lb + ".0"}
	}
	configureISIS(t, bs, dutIsisIntfNames)

	t.Run("Verify ISIS session status on DUT", func(t *testing.T) {
		dutIsisPeerIntf := []string{bs.OndatraDUTPorts[1].Name(), bs.OndatraDUTPorts[2].Name()}
		if deviations.ExplicitInterfaceInDefaultVRF(bs.DUT) {
			dutIsisPeerIntf = []string{bs.OndatraDUTPorts[1].Name() + ".0", bs.OndatraDUTPorts[2].Name() + ".0"}
		}
		verifyISISTelemetry(t, bs.DUT, dutIsisPeerIntf)
	})

	t.Run("Verify BGP session telemetry", func(t *testing.T) {
		verifyBgpTelemetry(t, bs)
	})

	t.Run("Verify BGP capabilities", func(t *testing.T) {
		verifyBGPCapabilities(t, bs)
	})

	t.Run("Verify BGP route telemetry", func(t *testing.T) {
		verifyPrefixesTelemetry(t, bs.DUT, bs.ATEPorts[0].IPv4, 200000, true)
		verifyPrefixesTelemetry(t, bs.DUT, bs.ATEPorts[0].IPv6, 200000, false)
	})
}

func configureDUTLoopback(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	lb = netutil.LoopbackInterface(t, dut, 0)
	lo0 := gnmi.OC().Interface(lb).Subinterface(0)
	ipv4Addrs := gnmi.LookupAll(t, dut, lo0.Ipv4().AddressAny().State())
	ipv6Addrs := gnmi.LookupAll(t, dut, lo0.Ipv6().AddressAny().State())
	foundV4 := false
	for _, ip := range ipv4Addrs {
		if v, ok := ip.Val(); ok {
			foundV4 = true
			dutLoopback.IPv4 = v.GetIp()
			break
		}
	}
	foundV6 := false
	for _, ip := range ipv6Addrs {
		if v, ok := ip.Val(); ok {
			foundV6 = true
			dutLoopback.IPv6 = v.GetIp()
			break
		}
	}
	if !foundV4 || !foundV6 {
		lo1 := dutLoopback.NewOCInterface(lb, dut)
		lo1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
		gnmi.Update(t, dut, gnmi.OC().Interface(lb).Config(), lo1)
	}

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, lb, deviations.DefaultNetworkInstance(dut), 0)
	}
}

func configureDUT(t *testing.T, bs *cfgplugins.BGPSession) *cfgplugins.BGPSession {
	t.Helper()

	dni := deviations.DefaultNetworkInstance(bs.DUT)
	bgp := bs.DUTConf.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(cfgplugins.PTBGP, "BGP").GetOrCreateBgp()

	// Increase the received and accepted prefix limits.
	afiSafiV4 := bgp.GetOrCreatePeerGroup(cfgplugins.BGPPeerGroup1).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	afiSafiV4.GetOrCreateIpv4Unicast().GetOrCreatePrefixLimitReceived().MaxPrefixes = ygot.Uint32(10000000)
	afiSafiV4.GetOrCreateIpv4Unicast().GetOrCreatePrefixLimit().MaxPrefixes = ygot.Uint32(2000000)
	afiSafiV6 := bgp.GetOrCreatePeerGroup(cfgplugins.BGPPeerGroup1).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	afiSafiV6.GetOrCreateIpv6Unicast().GetOrCreatePrefixLimitReceived().MaxPrefixes = ygot.Uint32(10000000)
	afiSafiV6.GetOrCreateIpv6Unicast().GetOrCreatePrefixLimit().MaxPrefixes = ygot.Uint32(2000000)

	// dutPort2 -> atePort2 peer (ibgp session)
	ateIBGPN2IPv4 := bgp.GetOrCreateNeighbor(otgIsisPort2LoopV4)
	ateIBGPN2IPv4.PeerGroup = ygot.String(cfgplugins.BGPPeerGroup1)
	ateIBGPN2IPv4.PeerAs = ygot.Uint32(cfgplugins.DutAS)
	ateIBGPN2IPv4.Enabled = ygot.Bool(true)
	bgpNbrT := ateIBGPN2IPv4.GetOrCreateTransport()
	bgpNbrT.LocalAddress = ygot.String(dutLoopback.IPv4)
	routeReflector := ateIBGPN2IPv4.GetOrCreateRouteReflector()
	routeReflector.RouteReflectorClient = ygot.Bool(true)
	// routeReflector.RouteReflectorClusterId = oc.UnionString(dutLoopback.IPv4)

	ateIBGPN2IPv4AF := ateIBGPN2IPv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	ateIBGPN2IPv4AF.SetEnabled(true)
	ateIBGPN2IPv4AFPolicy := ateIBGPN2IPv4AF.GetOrCreateApplyPolicy()
	ateIBGPN2IPv4AFPolicy.SetImportPolicy([]string{cfgplugins.RPLPermitAll})
	ateIBGPN2IPv4AFPolicy.SetExportPolicy([]string{cfgplugins.RPLPermitAll})

	ateIBGPN2IPv6 := bgp.GetOrCreateNeighbor(otgIsisPort2LoopV6)
	ateIBGPN2IPv6.PeerGroup = ygot.String(cfgplugins.BGPPeerGroup1)
	ateIBGPN2IPv6.PeerAs = ygot.Uint32(cfgplugins.DutAS)
	ateIBGPN2IPv6.Enabled = ygot.Bool(true)
	bgpNbrT = ateIBGPN2IPv6.GetOrCreateTransport()
	bgpNbrT.LocalAddress = ygot.String(dutLoopback.IPv6)
	routeReflector = ateIBGPN2IPv6.GetOrCreateRouteReflector()
	routeReflector.RouteReflectorClient = ygot.Bool(true)
	// routeReflector.RouteReflectorClusterId = oc.UnionString(dutLoopback.IPv4)

	ateIBGPN2IPv6AF := ateIBGPN2IPv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	ateIBGPN2IPv6AF.SetEnabled(true)
	ateIBGPN2IPv6AFPolicy := ateIBGPN2IPv6AF.GetOrCreateApplyPolicy()
	ateIBGPN2IPv6AFPolicy.SetImportPolicy([]string{cfgplugins.RPLPermitAll})
	ateIBGPN2IPv6AFPolicy.SetExportPolicy([]string{cfgplugins.RPLPermitAll})

	// dutPort3 -> atePort3 peer (ibgp session)
	ateIBGPN3IPv4 := bgp.GetOrCreateNeighbor(otgIsisPort3LoopV4)
	ateIBGPN3IPv4.PeerGroup = ygot.String(cfgplugins.BGPPeerGroup1)
	ateIBGPN3IPv4.PeerAs = ygot.Uint32(cfgplugins.DutAS)
	ateIBGPN3IPv4.Enabled = ygot.Bool(true)
	bgpNbrT = ateIBGPN3IPv4.GetOrCreateTransport()
	bgpNbrT.LocalAddress = ygot.String(dutLoopback.IPv4)
	routeReflector = ateIBGPN3IPv4.GetOrCreateRouteReflector()
	routeReflector.RouteReflectorClient = ygot.Bool(true)
	// routeReflector.RouteReflectorClusterId = oc.UnionString(dutLoopback.IPv4)

	ateIBGPN3IPv4AF := ateIBGPN3IPv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	ateIBGPN3IPv4AF.SetEnabled(true)
	ateIBGPN3IPv4AFPolicy := ateIBGPN3IPv4AF.GetOrCreateApplyPolicy()
	ateIBGPN3IPv4AFPolicy.SetImportPolicy([]string{cfgplugins.RPLPermitAll})
	ateIBGPN3IPv4AFPolicy.SetExportPolicy([]string{cfgplugins.RPLPermitAll})

	ateIBGPN3IPv6 := bgp.GetOrCreateNeighbor(otgIsisPort3LoopV6)
	ateIBGPN3IPv6.PeerGroup = ygot.String(cfgplugins.BGPPeerGroup1)
	ateIBGPN3IPv6.PeerAs = ygot.Uint32(cfgplugins.DutAS)
	ateIBGPN3IPv6.Enabled = ygot.Bool(true)
	bgpNbrT = ateIBGPN3IPv6.GetOrCreateTransport()
	bgpNbrT.LocalAddress = ygot.String(dutLoopback.IPv6)
	routeReflector = ateIBGPN3IPv6.GetOrCreateRouteReflector()
	routeReflector.RouteReflectorClient = ygot.Bool(true)
	// routeReflector.RouteReflectorClusterId = oc.UnionString(dutLoopback.IPv4)

	ateIBGPN3IPv6AF := ateIBGPN3IPv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	ateIBGPN3IPv6AF.SetEnabled(true)
	ateIBGPN3IPv6AFPolicy := ateIBGPN3IPv6AF.GetOrCreateApplyPolicy()
	ateIBGPN3IPv6AFPolicy.SetImportPolicy([]string{cfgplugins.RPLPermitAll})
	ateIBGPN3IPv6AFPolicy.SetExportPolicy([]string{cfgplugins.RPLPermitAll})

	return bs
}

func configureOTG(t *testing.T, bs *cfgplugins.BGPSession) {
	t.Helper()

	devices := bs.ATETop.Devices().Items()
	byName := func(i, j int) bool { return devices[i].Name() < devices[j].Name() }
	sort.Slice(devices, byName)

	// Port2 Loopback Configuration
	iDut2LoopV4 := devices[1].Ipv4Loopbacks().Add().SetName("Port2LoopV4").SetEthName(bs.ATEPorts[1].Name + ".Eth")
	iDut2LoopV4.SetAddress(otgIsisPort2LoopV4)
	iDut2LoopV6 := devices[1].Ipv6Loopbacks().Add().SetName("Port2LoopV6").SetEthName(bs.ATEPorts[1].Name + ".Eth")
	iDut2LoopV6.SetAddress(otgIsisPort2LoopV6)

	// Port3 Loopback Configuration
	iDut3LoopV4 := devices[2].Ipv4Loopbacks().Add().SetName("Port3LoopV4").SetEthName(bs.ATEPorts[2].Name + ".Eth")
	iDut3LoopV4.SetAddress(otgIsisPort3LoopV4)
	iDut3LoopV6 := devices[2].Ipv6Loopbacks().Add().SetName("Port3LoopV6").SetEthName(bs.ATEPorts[2].Name + ".Eth")
	iDut3LoopV6.SetAddress(otgIsisPort3LoopV6)

	// ISIS configuration on Port2 for iBGP session establishment.
	isisDut2 := devices[1].Isis().SetName("ISIS2").SetSystemId(otgSysID2)
	isisDut2.Basic().SetIpv4TeRouterId(bs.ATEPorts[1].IPv4).SetHostname(isisDut2.Name()).SetLearnedLspFilter(true)
	isisDut2.Interfaces().Add().SetEthName(devices[1].Ethernets().Items()[0].Name()).
		SetName("devIsisInt2").
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT)

	// Advertise OTG Port2 loopback address via ISIS.
	isisPort2V4 := devices[1].Isis().V4Routes().Add().SetName("ISISPort2V4").SetLinkMetric(10)
	isisPort2V4.Addresses().Add().SetAddress(otgIsisPort2LoopV4).SetPrefix(32)
	isisPort2V6 := devices[1].Isis().V6Routes().Add().SetName("ISISPort2V6").SetLinkMetric(10)
	isisPort2V6.Addresses().Add().SetAddress(otgIsisPort2LoopV6).SetPrefix(uint32(128))

	// ISIS configuration on Port3 for iBGP session establishment.
	isisDut3 := devices[2].Isis().SetName("ISIS3").SetSystemId(otgSysID3)
	isisDut3.Basic().SetIpv4TeRouterId(bs.ATEPorts[2].IPv4).SetHostname(isisDut3.Name()).SetLearnedLspFilter(true)
	isisDut3.Interfaces().Add().SetEthName(devices[2].Ethernets().Items()[0].Name()).
		SetName("devIsisInt3").
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT)

	// Advertise OTG Port3 loopback address via ISIS.
	isisPort3V4 := devices[2].Isis().V4Routes().Add().SetName("ISISPort3V4").SetLinkMetric(10)
	isisPort3V4.Addresses().Add().SetAddress(otgIsisPort3LoopV4).SetPrefix(32)
	isisPort3V6 := devices[2].Isis().V6Routes().Add().SetName("ISISPort3V6").SetLinkMetric(10)
	isisPort3V6.Addresses().Add().SetAddress(otgIsisPort3LoopV6).SetPrefix(uint32(128))

	// dutPort2 -> atePort2 peer (ibgp session)
	iDut2Bgp := devices[1].Bgp().SetRouterId(otgIsisPort2LoopV4)
	iDut2Bgp4Peer := iDut2Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut2LoopV4.Name()).Peers().Add().SetName(devices[1].Name() + ".BGP4.peer")
	iDut2Bgp4Peer.SetPeerAddress(dutLoopback.IPv4).SetAsNumber(cfgplugins.DutAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	iDut2Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true)
	// iBGP v6 session on Port2.
	iDut2Bgp6Peer := iDut2Bgp.Ipv6Interfaces().Add().SetIpv6Name(iDut2LoopV6.Name()).Peers().Add().SetName(devices[1].Name() + ".BGP6.peer")
	iDut2Bgp6Peer.SetPeerAddress(dutLoopback.IPv6).SetAsNumber(cfgplugins.DutAS).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
	iDut2Bgp6Peer.LearnedInformationFilter().SetUnicastIpv6Prefix(true)

	// dutPort3 -> atePort3 peer (ibgp session)
	iDut3Bgp := devices[2].Bgp().SetRouterId(otgIsisPort3LoopV4)
	iDut3Bgp4Peer := iDut3Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut3LoopV4.Name()).Peers().Add().SetName(devices[2].Name() + ".BGP4.peer")
	iDut3Bgp4Peer.SetPeerAddress(dutLoopback.IPv4).SetAsNumber(cfgplugins.DutAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	iDut3Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true)
	// iBGP v6 session on Port3.
	iDut3Bgp6Peer := iDut3Bgp.Ipv6Interfaces().Add().SetIpv6Name(iDut3LoopV6.Name()).Peers().Add().SetName(devices[2].Name() + ".BGP6.peer")
	iDut3Bgp6Peer.SetPeerAddress(dutLoopback.IPv6).SetAsNumber(cfgplugins.DutAS).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
	iDut3Bgp6Peer.LearnedInformationFilter().SetUnicastIpv6Prefix(true)

	advertiseRoutes(t, bs, 1)
	advertiseRoutes(t, bs, 2)
}

func configureISIS(t *testing.T, bs *cfgplugins.BGPSession, intfName []string) {
	t.Helper()

	dutConfIsisPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(bs.DUT)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	netInstance := bs.DUTConf.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(bs.DUT))
	prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	prot.Enabled = ygot.Bool(true)
	isis := prot.GetOrCreateIsis()
	globalISIS := isis.GetOrCreateGlobal()
	globalISIS.LevelCapability = oc.Isis_LevelType_LEVEL_2
	globalISIS.Net = []string{fmt.Sprintf("%v.%v.00", dutAreaAddress, dutSysID)}
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	if deviations.ISISInstanceEnabledRequired(bs.DUT) {
		globalISIS.Instance = ygot.String(isisInstance)
	}
	isisLevel2 := isis.GetOrCreateLevel(2)
	isisLevel2.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC
	if deviations.ISISLevelEnabled(bs.DUT) {
		isisLevel2.Enabled = ygot.Bool(true)
	}

	for _, intf := range intfName {
		if deviations.InterfaceRefInterfaceIDFormat(bs.DUT) {
			intf += ".0"
		}
		isisIntf := isis.GetOrCreateInterface(intf)
		isisIntf.Enabled = ygot.Bool(true)
		isisIntf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
		isisIntfLevel := isisIntf.GetOrCreateLevel(2)
		isisIntfLevel.Enabled = ygot.Bool(true)
		isisIntfLevelAfi := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
		isisIntfLevelAfi.Metric = ygot.Uint32(200)
		isisIntfLevelAfi.Enabled = ygot.Bool(true)
		isisIntfLevelAfi6 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)
		isisIntfLevelAfi6.Metric = ygot.Uint32(200)
		isisIntfLevelAfi6.Enabled = ygot.Bool(true)
		if deviations.ISISInterfaceAfiUnsupported(bs.DUT) {
			isisIntf.Af = nil
		}
		if deviations.MissingIsisInterfaceAfiSafiEnable(bs.DUT) {
			isisIntfLevelAfi.Enabled = nil
			isisIntfLevelAfi6.Enabled = nil
		}
	}

	gnmi.Replace(t, bs.DUT, dutConfIsisPath.Config(), prot)
}

func verifyISISTelemetry(t *testing.T, dut *ondatra.DUTDevice, dutIntf []string) {
	t.Helper()

	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Isis()
	for _, intfName := range dutIntf {
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			intfName = intfName + ".0"
		}
		nbrPath := statePath.Interface(intfName)
		query := nbrPath.LevelAny().AdjacencyAny().AdjacencyState().State()
		_, ok := gnmi.WatchAll(t, dut, query, time.Minute, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
			state, present := val.Val()
			return present && state == oc.Isis_IsisInterfaceAdjState_UP
		}).Await(t)
		if !ok {
			t.Logf("IS-IS state on %v has no adjacencies", intfName)
			t.Fatal("No IS-IS adjacency reported.")
		}
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

func verifyBgpTelemetry(t *testing.T, bs *cfgplugins.BGPSession) {
	// t.Helper()

	var nbrIP = []string{bs.ATEPorts[0].IPv4, bs.ATEPorts[0].IPv6, otgIsisPort2LoopV4, otgIsisPort2LoopV6, otgIsisPort3LoopV4, otgIsisPort3LoopV6}
	t.Logf("Verifying BGP state.")
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(bs.DUT)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	for _, nbr := range nbrIP {
		nbrPath := bgpPath.Neighbor(nbr)
		t.Logf("Waiting for BGP neighbor to establish...")
		var status *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]
		status, ok := gnmi.Watch(t, bs.DUT, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			state, ok := val.Val()
			return ok && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, bs.DUT, nbrPath.State()))
			t.Fatal("No BGP neighbor formed")
		}
		state, _ := status.Val()
		t.Logf("BGP adjacency for %s: %v", nbr, state)
		if want := oc.Bgp_Neighbor_SessionState_ESTABLISHED; state != want {
			t.Errorf("BGP peer %s status got %d, want %d", nbr, state, want)
		}
	}
}

func verifyBGPCapabilities(t *testing.T, bs *cfgplugins.BGPSession) {
	t.Helper()

	t.Log("Verifying BGP capabilities.")
	var nbrIP = []string{bs.ATEPorts[0].IPv4, bs.ATEPorts[0].IPv6, otgIsisPort2LoopV4, otgIsisPort2LoopV6, otgIsisPort3LoopV4, otgIsisPort3LoopV6}
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(bs.DUT)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	for _, nbr := range nbrIP {
		nbrPath := statePath.Neighbor(nbr)
		capabilities := map[oc.E_BgpTypes_BGP_CAPABILITY]bool{
			oc.BgpTypes_BGP_CAPABILITY_ROUTE_REFRESH: false,
			oc.BgpTypes_BGP_CAPABILITY_MPBGP:         false,
			oc.BgpTypes_BGP_CAPABILITY_ASN32:         false,
		}
		for _, sCap := range gnmi.Get(t, bs.DUT, nbrPath.SupportedCapabilities().State()) {
			capabilities[sCap] = true
		}
		for sCap, present := range capabilities {
			if !present {
				t.Errorf("Capability not reported: %v", sCap)
			}
		}
	}
}

func advertiseRoutes(t *testing.T, bs *cfgplugins.BGPSession, portIndex int) {
	t.Helper()
	t.Log("Advertising routes to OTG")
	// each IPv4 ibgp port // 2M unique - 1.5M internet, 500k internal
	// each IPv6 ibgp port // 1M unique - 600k internet, 400k internal

	devices := bs.ATETop.Devices().Items()
	ipV4 := devices[portIndex].Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	bgp4Peer := devices[portIndex].Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0]
	ipV6 := devices[portIndex].Ethernets().Items()[0].Ipv6Addresses().Items()[0]
	bgp6Peer := devices[portIndex].Bgp().Ipv6Interfaces().Items()[0].Peers().Items()[0]

	// configure emulated IPv4 and IPv6 networks
	netV4 := bgp4Peer.V4Routes().Add().SetName(fmt.Sprintf("v4-bgpNet-dev%d", portIndex))

	netV4.SetNextHopIpv4Address(ipV4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	v4Prefixes := createInternetPrefixesV4(t)
	for _, prefix := range v4Prefixes {
		netV4.Addresses().Add().SetAddress(prefix.Addr().String()).SetPrefix(uint32(prefix.Bits())).SetCount(1)
	}
	// netV4.Addresses().Add().SetAddress("172.24.0.0").SetPrefix(uint32(30)).SetCount(300000)

	netV6 := bgp6Peer.V6Routes().Add().SetName(fmt.Sprintf("v6-bgpNet-dev%d", portIndex))
	netV6.SetNextHopIpv6Address(ipV6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	v6Prefixes := createInternetPrefixesV6(t)
	for _, prefix := range v6Prefixes {
		netV6.Addresses().Add().SetAddress(prefix.Addr().String()).SetPrefix(uint32(prefix.Bits())).SetCount(1)
	}
	netV6.Addresses().Add().SetAddress("fc00:abce::").SetPrefix(uint32(126)).SetCount(100000)
}

func createInternetPrefixesV4(t *testing.T) []netip.Prefix {
	t.Helper()
	var ips []netip.Prefix
	// Create 1500000 /32s - internet prefixes
	for i := 50; i < 74; i++ {
		for j := 0; j < 250; j++ {
			for k := 0; k < 100; k += 4 {
				ips = append(ips, netip.MustParsePrefix(fmt.Sprintf("182.%d.%d.%d/30", i, j, k)))
			}
		}
	}

	// Create 500000 /32s - internal prefixes
	for i := 16; i < 24; i++ {
		for j := 0; j < 250; j++ {
			for k := 0; k < 100; k += 4 {
				ips = append(ips, netip.MustParsePrefix(fmt.Sprintf("172.%d.%d.%d/30", i, j, k)))
			}
		}
	}

	return ips
}

func createInternetPrefixesV6(t *testing.T) []netip.Prefix {
	t.Helper()
	var ips []netip.Prefix
	// Create 600000 /64s - internet prefixes
	for j := 10; j < 70; j++ {
		for i := 0; i < 4000; i += 4 {
			ip := netip.MustParsePrefix(fmt.Sprintf("2001:db8:%d:%d::/126", i, j))
			ips = append(ips, ip)
		}
	}

	// Create 400000 /64s - internal prefixes
	for j := 10; j < 50; j++ {
		for i := 0; i < 4000; i += 4 {
			ip := netip.MustParsePrefix(fmt.Sprintf("fc00:abcd:%d:%d::/126", i, j))
			ips = append(ips, ip)
		}
	}

	return ips
}

func verifyPrefixesTelemetry(t *testing.T, dut *ondatra.DUTDevice, nbr string, wantSent uint32, isV4 bool) {
	t.Helper()
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	t.Logf("Prefix telemetry on DUT for peer %v", nbr)

	var prefixPath *netinstbgp.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_PrefixesPath
	if isV4 {
		prefixPath = statePath.Neighbor(nbr).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
	} else {
		prefixPath = statePath.Neighbor(nbr).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Prefixes()
	}
	if gotSent, ok := gnmi.Watch(t, dut, prefixPath.Sent().State(), 30*time.Second, func(val *ygnmi.Value[uint32]) bool {
		gotSent, ok := val.Val()
		return ok && gotSent == wantSent
	}).Await(t); !ok {
		t.Logf("Installed prefixes mismatch: got %v, want %v", gotSent, wantSent)
	}
}
