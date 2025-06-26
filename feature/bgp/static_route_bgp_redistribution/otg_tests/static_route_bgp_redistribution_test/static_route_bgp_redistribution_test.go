// Copyright 2023 Nokia, Google LLC
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
//
// This code is a Contribution to OpenConfig Feature Profiles project ("Work")
// made under the Google Software Grant and Corporate Contributor License
// Agreement ("CLA") and governed by the Apache License 2.0. No other rights
// or licenses in or to any of Nokia's intellectual property are granted for
// any other purpose. This code is provided on an "as is" basis without
// any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package static_route_bgp_redistribution_test

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ipv4PrefixLen                  = 30
	ipv6PrefixLen                  = 126
	subInterfaceIndex              = 0
	mtu                            = 1500
	peerGroupName                  = "PEER-GROUP"
	dutAsn                         = 64512
	atePeer1Asn                    = 64511
	atePeer2Asn                    = 64512
	acceptRoute                    = true
	metricPropagate                = true
	policyResultNext               = true
	isV4                           = true
	shouldBePresent                = true
	replace                        = true
	redistributeStaticPolicyNameV4 = "route-policy-v4"
	redistributeStaticPolicyNameV6 = "route-policy-v6"
	policyStatementNameV4          = "statement-v4"
	policyStatementNameV6          = "statement-v6"
	trafficDuration                = 30 * time.Second
	tolerancePct                   = 2
	medZero                        = 0
	medNonZero                     = 1000
	medIPv4                        = 104
	medIPv6                        = 106
	localPreference                = 100
)

var (
	dutPort1 = &attrs.Attributes{
		Name:    "dutPort1",
		MAC:     "00:12:01:01:01:01",
		IPv4:    "192.168.1.1",
		IPv6:    "2001:db8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     mtu,
	}

	dutPort2 = &attrs.Attributes{
		Name:    "dutPort2",
		MAC:     "00:12:02:01:01:01",
		IPv4:    "192.168.1.5",
		IPv6:    "2001:db8::5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     mtu,
	}

	dutPort3 = &attrs.Attributes{
		Name:    "dutPort3",
		MAC:     "00:12:03:01:01:01",
		IPv4:    "192.168.1.9",
		IPv6:    "2001:db8::9",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     mtu,
	}

	atePort1 = &attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.168.1.2",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     mtu,
	}

	atePort2 = &attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.168.1.6",
		IPv6:    "2001:db8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     mtu,
	}

	atePort3 = &attrs.Attributes{
		Name:    "atePort3",
		MAC:     "02:00:03:01:01:01",
		IPv4:    "192.168.1.10",
		IPv6:    "2001:db8::a",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     mtu,
	}

	dutPorts = map[string]*attrs.Attributes{
		"port1": dutPort1,
		"port2": dutPort2,
		"port3": dutPort3,
	}

	atePorts = map[string]*attrs.Attributes{
		"port1": atePort1,
		"port2": atePort2,
		"port3": atePort3,
	}
)

func configureDUTPort(t *testing.T, dut *ondatra.DUTDevice, port *ondatra.Port, portAttrs *attrs.Attributes) {
	t.Helper()

	gnmi.Replace(
		t,
		dut,
		gnmi.OC().Interface(port.Name()).Config(),
		portAttrs.NewOCInterface(port.Name(), dut),
	)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, port)
	}

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, port.Name(), deviations.DefaultNetworkInstance(dut), subInterfaceIndex)
	}
}

func configureDUTStatic(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	staticPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))

	dutOcRoot := &oc.Root{}
	networkInstance := dutOcRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	networkInstanceProtocolStatic := networkInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	networkInstanceProtocolStatic.SetEnabled(true)

	ipv4StaticRoute := networkInstanceProtocolStatic.GetOrCreateStatic("192.168.10.0/24")
	// TODO - we dont support, guessing table connection related?
	ipv4StaticRoute.SetSetTag(oc.UnionString("40"))

	ipv4StaticRouteNextHop := ipv4StaticRoute.GetOrCreateNextHop("0")
	if deviations.SetMetricAsPreference(dut) {
		ipv4StaticRouteNextHop.Preference = ygot.Uint32(medIPv4)
	} else {
		ipv4StaticRouteNextHop.Metric = ygot.Uint32(medIPv4)
	}
	ipv4StaticRouteNextHop.SetNextHop(oc.UnionString(atePort2.IPv4))

	ipv6StaticRoute := networkInstanceProtocolStatic.GetOrCreateStatic("2024:db8:128:128::/64")
	ipv6StaticRoute.SetSetTag(oc.UnionString("60"))

	ipv6StaticRouteNextHop := ipv6StaticRoute.GetOrCreateNextHop("0")
	if deviations.SetMetricAsPreference(dut) {
		ipv6StaticRouteNextHop.Preference = ygot.Uint32(medIPv6)
	} else {
		ipv6StaticRouteNextHop.Metric = ygot.Uint32(medIPv6)
	}
	ipv6StaticRouteNextHop.SetNextHop(oc.UnionString(atePort2.IPv6))

	gnmi.Update(t, dut, staticPath.Config(), networkInstanceProtocolStatic)
}

func configureDUTBGP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dutOcRoot := &oc.Root{}

	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	if deviations.DefaultBgpInstanceName(dut) != "" {
		bgpPath = gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut))
	}

	// permit all policy
	rp := dutOcRoot.GetOrCreateRoutingPolicy()
	pdef := rp.GetOrCreatePolicyDefinition("permit-all")
	stmt, err := pdef.AppendNewStatement("accept")
	if err != nil {
		t.Fatalf("failed creating new policy statement, err: %s", err)
	}
	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)

	// setup BGP
	networkInstance := dutOcRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	networkInstanceProtocolBgp := networkInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	networkInstanceProtocolBgp.SetEnabled(true)

	if deviations.DefaultBgpInstanceName(dut) != "" {
		networkInstanceProtocolBgp = networkInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut))
		networkInstanceProtocolBgp.SetEnabled(true)
	}
	bgp := networkInstanceProtocolBgp.GetOrCreateBgp()

	bgpGlobal := bgp.GetOrCreateGlobal()
	bgpGlobal.RouterId = ygot.String(dutPort1.IPv4)
	bgpGlobal.As = ygot.Uint32(dutAsn)

	bgpGlobalIPv4AF := bgpGlobal.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	bgpGlobalIPv4AF.SetEnabled(true)

	bgpGlobalIPv6AF := bgpGlobal.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	bgpGlobalIPv6AF.SetEnabled(true)

	if !deviations.SkipBgpSendCommunityType(dut) {
		bgpGlobalIPv6AF.SetSendCommunityType([]oc.E_Bgp_CommunityType{oc.Bgp_CommunityType_STANDARD})
		bgpGlobalIPv4AF.SetSendCommunityType([]oc.E_Bgp_CommunityType{oc.Bgp_CommunityType_STANDARD})
	}

	bgpPeerGroup := bgp.GetOrCreatePeerGroup(peerGroupName)
	bgpPeerGroup.SetPeerAs(dutAsn)

	// dutPort1 -> atePort1 peer (ebgp session)
	ateEBGPNeighborOne := bgp.GetOrCreateNeighbor(atePort1.IPv4)
	ateEBGPNeighborOne.PeerGroup = ygot.String(peerGroupName)
	ateEBGPNeighborOne.PeerAs = ygot.Uint32(atePeer1Asn)
	ateEBGPNeighborOne.Enabled = ygot.Bool(true)

	ateEBGPNeighborIPv4AF := ateEBGPNeighborOne.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	ateEBGPNeighborIPv4AF.SetEnabled(true)
	ateEBGPNeighborIPv4AFPolicy := ateEBGPNeighborIPv4AF.GetOrCreateApplyPolicy()
	ateEBGPNeighborIPv4AFPolicy.SetImportPolicy([]string{"permit-all"})
	ateEBGPNeighborIPv4AFPolicy.SetExportPolicy([]string{"permit-all"})

	ateEBGPNeighborTwo := bgp.GetOrCreateNeighbor(atePort1.IPv6)
	ateEBGPNeighborTwo.PeerGroup = ygot.String(peerGroupName)
	ateEBGPNeighborTwo.PeerAs = ygot.Uint32(atePeer1Asn)
	ateEBGPNeighborTwo.Enabled = ygot.Bool(true)

	ateEBGPNeighborIPv6AF := ateEBGPNeighborTwo.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	ateEBGPNeighborIPv6AF.SetEnabled(true)
	ateEBGPNeighborIPv6AFPolicy := ateEBGPNeighborIPv6AF.GetOrCreateApplyPolicy()
	ateEBGPNeighborIPv6AFPolicy.SetImportPolicy([]string{"permit-all"})
	ateEBGPNeighborIPv6AFPolicy.SetExportPolicy([]string{"permit-all"})

	// dutPort3 -> atePort3 peer (ibgp session)
	ateIBGPNeighborThree := bgp.GetOrCreateNeighbor(atePort3.IPv4)
	ateIBGPNeighborThree.PeerGroup = ygot.String(peerGroupName)
	ateIBGPNeighborThree.PeerAs = ygot.Uint32(atePeer2Asn)
	ateIBGPNeighborThree.Enabled = ygot.Bool(true)

	ateIBGPNeighborThreeIPv4AF := ateIBGPNeighborThree.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	ateIBGPNeighborThreeIPv4AF.SetEnabled(true)
	ateIBGPNeighborThreeIPv4AFPolicy := ateIBGPNeighborThreeIPv4AF.GetOrCreateApplyPolicy()
	ateIBGPNeighborThreeIPv4AFPolicy.SetImportPolicy([]string{"permit-all"})
	ateIBGPNeighborThreeIPv4AFPolicy.SetExportPolicy([]string{"permit-all"})

	ateIBGPNeighborFour := bgp.GetOrCreateNeighbor(atePort3.IPv6)
	ateIBGPNeighborFour.PeerGroup = ygot.String(peerGroupName)
	ateIBGPNeighborFour.PeerAs = ygot.Uint32(atePeer2Asn)
	ateIBGPNeighborFour.Enabled = ygot.Bool(true)

	ateIBGPNeighborFourIPv6AF := ateIBGPNeighborFour.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	ateIBGPNeighborFourIPv6AF.SetEnabled(true)
	ateIBGPNeighborFourIPv6AFPolicy := ateIBGPNeighborFourIPv6AF.GetOrCreateApplyPolicy()
	ateIBGPNeighborFourIPv6AFPolicy.SetImportPolicy([]string{"permit-all"})
	ateIBGPNeighborFourIPv6AFPolicy.SetExportPolicy([]string{"permit-all"})

	gnmi.Replace(t, dut, bgpPath.Config(), networkInstanceProtocolBgp)
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	for portName, portAttrs := range dutPorts {
		port := dut.Port(t, portName)
		configureDUTPort(t, dut, port, portAttrs)
	}
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	configureDUTStatic(t, dut)
	configureDUTBGP(t, dut)
}

func awaitBGPEstablished(t *testing.T, dut *ondatra.DUTDevice, neighbors []string) {
	for _, neighbor := range neighbors {
		if deviations.DefaultBgpInstanceName(dut) == "" {
			gnmi.Await(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
				Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").
				Bgp().
				Neighbor(neighbor).
				SessionState().State(), time.Second*240, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
		} else {
			gnmi.Await(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
				Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)).
				Bgp().
				Neighbor(neighbor).
				SessionState().State(), time.Second*240, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
		}
	}
}

func sortDevicesByName(devices []gosnappi.Device) {
	sort.Slice(devices, func(i, j int) bool {
		return devices[i].Name() < devices[j].Name()
	})
}

func configureOTG(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()

	otgConfig := gosnappi.NewConfig()

	for portName, portAttrs := range atePorts {
		port := ate.Port(t, portName)
		portAttrs.AddToOTG(otgConfig, port, dutPorts[portName])
	}

	devices := otgConfig.Devices().Items()
	sortDevicesByName(devices)

	// eBGP v4 session on Port1.
	bgp := devices[0].Bgp().SetRouterId(atePort1.IPv4)
	iDut1Ipv4 := devices[0].Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	iDut1Bgp := bgp.SetRouterId(iDut1Ipv4.Address())
	iDut1Bgp4Peer := iDut1Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut1Ipv4.Name()).Peers().Add().SetName(atePort1.Name + ".BGP4.peer")
	iDut1Bgp4Peer.SetPeerAddress(iDut1Ipv4.Gateway()).SetAsNumber(atePeer1Asn).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	iDut1Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true)
	// eBGP v6 session on Port1.
	iDut1Ipv6 := devices[0].Ethernets().Items()[0].Ipv6Addresses().Items()[0]
	iDut1Bgp6Peer := iDut1Bgp.Ipv6Interfaces().Add().SetIpv6Name(iDut1Ipv6.Name()).Peers().Add().SetName(atePort1.Name + ".BGP6.peer")
	iDut1Bgp6Peer.SetPeerAddress(iDut1Ipv6.Gateway()).SetAsNumber(atePeer1Asn).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	iDut1Bgp6Peer.LearnedInformationFilter().SetUnicastIpv6Prefix(true)

	// iBGP v4 session on Port3.
	bgp = devices[2].Bgp().SetRouterId(atePort3.IPv4)
	iDut3Ipv4 := devices[2].Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	iDut3Bgp := bgp.SetRouterId(iDut3Ipv4.Address())
	iDut3Bgp4Peer := iDut3Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut3Ipv4.Name()).Peers().Add().SetName(atePort3.Name + ".BGP4.peer")
	iDut3Bgp4Peer.SetPeerAddress(iDut3Ipv4.Gateway()).SetAsNumber(atePeer2Asn).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	iDut3Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true)
	// iBGP v6 session on Port3.
	iDut3Ipv6 := devices[2].Ethernets().Items()[0].Ipv6Addresses().Items()[0]
	iDut3Bgp6Peer := iDut3Bgp.Ipv6Interfaces().Add().SetIpv6Name(iDut3Ipv6.Name()).Peers().Add().SetName(atePort3.Name + ".BGP6.peer")
	iDut3Bgp6Peer.SetPeerAddress(iDut3Ipv6.Gateway()).SetAsNumber(atePeer2Asn).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
	iDut3Bgp6Peer.LearnedInformationFilter().SetUnicastIpv6Prefix(true)

	return otgConfig
}

// Configure OTG traffic-flow
func configureTrafficFlow(t *testing.T, otgConfig gosnappi.Config, isV4 bool, name, flowSrcEndPoint, flowDstEndPoint, srcMac, srcIP, dstIP string) gosnappi.Config {
	t.Helper()

	// ATE Traffic Configuration.
	t.Logf("TestBGP:start ate Traffic config: %v", name)

	otgConfig.Flows().Clear()

	flow := otgConfig.Flows().Add().SetName(name)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().
		SetTxNames([]string{flowSrcEndPoint}).
		SetRxNames([]string{flowDstEndPoint})
	flow.Size().SetFixed(1500)
	flow.Duration().FixedPackets().SetPackets(1000)
	e := flow.Packet().Add().Ethernet()
	e.Src().SetValue(srcMac)
	if isV4 {
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(srcIP)
		v4.Dst().SetValue(dstIP)
	} else {
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(srcIP)
		v6.Dst().SetValue(dstIP)
	}

	return otgConfig
}

// Sending traffic over configured flow for fixed duration
func sendTraffic(t *testing.T, otg *otg.OTG) {
	t.Logf("Starting traffic")
	otg.StartTraffic(t)
	time.Sleep(trafficDuration)
	t.Logf("Stop traffic")
	otg.StopTraffic(t)
}

// Validate traffic flow
func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, conf gosnappi.Config) {
	otg := ate.OTG()
	otgutils.LogFlowMetrics(t, otg, conf)
	for _, flow := range conf.Flows().Items() {
		recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).State())
		txPackets := float32(recvMetric.GetCounters().GetOutPkts())
		rxPackets := float32(recvMetric.GetCounters().GetInPkts())
		if txPackets == 0 {
			t.Fatalf("TxPkts = 0, want > 0")
		}
		lostPackets := txPackets - rxPackets
		lossPct := lostPackets * 100 / txPackets
		if lossPct > tolerancePct {
			t.Fatalf("Traffic Loss Pct for Flow %s: got %v, want max %v pct failure", flow.Name(), lossPct, tolerancePct)
		} else {
			t.Logf("Traffic Test Passed! for flow %s", flow.Name())
		}
	}
}

// Configure table-connection with source as static-route and destination as bgp
func configureTableConnection(t *testing.T, dut *ondatra.DUTDevice, isV4, mPropagation bool, importPolicy string, defaultImport oc.E_RoutingPolicy_DefaultPolicyType) {
	t.Helper()

	niPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	dutOcRoot := &oc.Root{}
	networkInstance := dutOcRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	addressFamily := oc.Types_ADDRESS_FAMILY_IPV4
	if !isV4 {
		addressFamily = oc.Types_ADDRESS_FAMILY_IPV6
	}

	batchSet := &gnmi.SetBatch{}
	tc := networkInstance.GetOrCreateTableConnection(
		oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
		oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
		addressFamily,
	)

	if importPolicy != "" {
		tc.SetImportPolicy([]string{importPolicy})
	}
	if !deviations.SkipSettingDisableMetricPropagation(dut) {
		tc.SetDisableMetricPropagation(!mPropagation)
	}

	if deviations.EnableTableConnections(dut) {
		fptest.ConfigEnableTbNative(t, dut)
	}
	gnmi.BatchUpdate(batchSet, niPath.TableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, addressFamily).Config(), tc)

	if deviations.SamePolicyAttachedToAllAfis(dut) {
		if addressFamily == oc.Types_ADDRESS_FAMILY_IPV4 {
			addressFamily = oc.Types_ADDRESS_FAMILY_IPV6
		} else {
			addressFamily = oc.Types_ADDRESS_FAMILY_IPV4
		}

		tc1 := networkInstance.GetOrCreateTableConnection(
			oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
			oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
			addressFamily,
		)

		if importPolicy != "" {
			tc1.SetImportPolicy([]string{importPolicy})
		}
		if !deviations.SkipSettingDisableMetricPropagation(dut) {
			tc1.SetDisableMetricPropagation(!mPropagation)
		}
		gnmi.BatchUpdate(batchSet, niPath.TableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, addressFamily).Config(), tc1)
	}

	batchSet.Set(t, dut)
	if deviations.TcMetricPropagationUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.CISCO:
			med := medZero
			if mPropagation {
				if isV4 {
					med = medIPv4
				} else if !isV4 {
					med = medIPv6
				}
			}
			cfgplugins.DeviationCiscoTableConnectionsStatictoBGPMetricPropagation(t, dut, isV4, med, importPolicy)
		}
	}
}

// Validate configurations for table-connections and routing-policy
func validateRedistributeStatic(t *testing.T, dut *ondatra.DUTDevice, acceptRoute, isV4, mPropagation bool) {

	af := oc.Types_ADDRESS_FAMILY_IPV4
	if !isV4 {
		af = oc.Types_ADDRESS_FAMILY_IPV6
	}

	if deviations.TcSubscriptionUnsupported(dut) {
		// wait for routes to propagate to otg
		time.Sleep(20 * time.Second)
		return
	}

	tcState := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).TableConnection(
		oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
		oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
		af).State())

	if tcState.GetSrcProtocol() != oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC {
		t.Fatal("source protocol not static for table connection but should be")
	}

	if tcState.GetDstProtocol() != oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP {
		t.Fatal("destination protocol not bgp for table connection but should be")
	}

	if tcState.GetAddressFamily() != af {
		t.Fatal("address family not as expected or table connection but should be")
	}

	if !deviations.SkipSettingDisableMetricPropagation(dut) {
		if mPropagation {
			if tcState.GetDisableMetricPropagation() {
				t.Fatal("Metric propagation disabled for table connection, expected enabled")
			}
		} else {
			if !tcState.GetDisableMetricPropagation() {
				t.Fatal("Metric propagation is enabled for table connection, expected disabled")
			}
		}
	}

	if !deviations.SkipSettingDisableMetricPropagation(dut) {
		if mPropagation {
			if tcState.GetDisableMetricPropagation() {
				t.Fatal("Metric propagation disabled for table connection, expected enabled")
			}
		} else {
			if !tcState.GetDisableMetricPropagation() {
				t.Fatal("Metric propagation is enabled for table connection, expected disabled")
			}
		}
	}
}

// Validate prefix-set routing-policy configurations
func validatePrefixSetRoutingPolicy(t *testing.T, dut *ondatra.DUTDevice, isV4 bool) {

	redistributeStaticPolicyName := redistributeStaticPolicyNameV4
	policyStatementName := policyStatementNameV4
	prefixSetName := "prefix-set-v4"
	prefixSetMode := oc.PrefixSet_Mode_IPV4
	prefixAddress := "192.168.10.0/24"
	prefixMaskLen := "exact"
	if !isV4 {
		redistributeStaticPolicyName = redistributeStaticPolicyNameV6
		policyStatementName = policyStatementNameV6
		prefixSetName = "prefix-set-v6"
		prefixSetMode = oc.PrefixSet_Mode_IPV6
		prefixAddress = "2024:db8:128:128::/64"
	}

	var foundPDef oc.RoutingPolicy_PolicyDefinition
	policyDef := gnmi.GetAll(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinitionAny().State())
	for _, pDef := range policyDef {
		if pDef.GetName() == redistributeStaticPolicyName {
			foundPDef = *pDef
		}
	}

	if foundPDef.GetName() != redistributeStaticPolicyName {
		t.Fatal("Expected import policy is not configured")
	}

	if foundPDef.GetStatement(policyStatementName).GetActions().GetPolicyResult() != oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE {
		t.Fatalf("Routing-policy result unexpectd for statement %v. It is not set to ACCEPT_ROUTE.", policyStatementName)
	}

	if foundPDef.GetStatement(policyStatementName).GetConditions().GetMatchPrefixSet().GetPrefixSet() != prefixSetName {
		t.Fatal("Routing-policy not associated with expected prefix-set")
	}

	if !deviations.SkipSetRpMatchSetOptions(dut) {
		if foundPDef.GetStatement(policyStatementName).GetConditions().GetMatchPrefixSet().GetMatchSetOptions() != oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY {
			t.Fatal("Routing-policy prefix-set match-set-option not set to ANY")
		}
	}

	var foundPSet oc.RoutingPolicy_DefinedSets_PrefixSet
	prefixSet := gnmi.GetAll(t, dut, gnmi.OC().RoutingPolicy().DefinedSets().PrefixSetAny().State())
	for _, pSet := range prefixSet {
		if pSet.GetName() == prefixSetName {
			foundPSet = *pSet
		}
	}

	if foundPSet.GetName() != prefixSetName {
		t.Fatal("Expected prefix-set is not configured")
	}

	if !deviations.SkipPrefixSetMode(dut) {
		if foundPSet.GetMode() != prefixSetMode {
			t.Fatal("Expected prefix-set mode is not configured")
		}
	}

	if foundPSet.GetPrefix(prefixAddress, prefixMaskLen).GetIpPrefix() != prefixAddress {
		t.Fatal("Expected prefix not configured in prefix-set")
	}
}

// 1.27.3 setup function
func redistributeIPv4Static(t *testing.T, dut *ondatra.DUTDevice) {
	configureTableConnection(t, dut, isV4, !metricPropagate, "", oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE)
}

// 1.27.3 validation function
func validateRedistributeIPv4Static(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	validateRedistributeStatic(t, dut, acceptRoute, isV4, !metricPropagate)
	if deviations.SkipSettingDisableMetricPropagation(dut) {
		validateLearnedIPv4Prefix(t, ate, atePort1.Name+".BGP4.peer", "192.168.10.0", medIPv4, shouldBePresent)
	} else {
		validateLearnedIPv4Prefix(t, ate, atePort1.Name+".BGP4.peer", "192.168.10.0", medZero, shouldBePresent)
	}
}

// 1.27.4 setup function
func redistributeIPv4StaticWithMetricPropagation(t *testing.T, dut *ondatra.DUTDevice) {
	configureTableConnection(t, dut, isV4, metricPropagate, "", oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE)
}

// 1.27.4 validation function
func validateRedistributeIPv4StaticWithMetricPropagation(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	validateRedistributeStatic(t, dut, acceptRoute, isV4, metricPropagate)
	validateLearnedIPv4Prefix(t, ate, atePort1.Name+".BGP4.peer", "192.168.10.0", medIPv4, shouldBePresent)
}

// 1.27.14 setup function
func redistributeIPv6Static(t *testing.T, dut *ondatra.DUTDevice) {
	configureTableConnection(t, dut, !isV4, !metricPropagate, "", oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE)
}

// 1.27.14 validation function
func validateRedistributeIPv6Static(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	validateRedistributeStatic(t, dut, acceptRoute, !isV4, !metricPropagate)
	if deviations.SkipSettingDisableMetricPropagation(dut) {
		validateLearnedIPv6Prefix(t, ate, atePort1.Name+".BGP6.peer", "2024:db8:128:128::", medIPv6, shouldBePresent)
	} else {
		validateLearnedIPv6Prefix(t, ate, atePort1.Name+".BGP6.peer", "2024:db8:128:128::", medZero, shouldBePresent)
	}
}

// 1.27.15 setup function
func redistributeIPv6StaticWithMetricPropagation(t *testing.T, dut *ondatra.DUTDevice) {
	configureTableConnection(t, dut, !isV4, metricPropagate, "", oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE)
}

// 1.27.15 validation function
func validateRedistributeIPv6StaticWithMetricPropagation(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	validateRedistributeStatic(t, dut, acceptRoute, !isV4, metricPropagate)
	validateLearnedIPv6Prefix(t, ate, atePort1.Name+".BGP6.peer", "2024:db8:128:128::", medIPv6, shouldBePresent)
}

// 1.27.1 setup function
func redistributeIPv4StaticDefaultRejectPolicy(t *testing.T, dut *ondatra.DUTDevice) {
	defaultRejectPolicyName := ""
	if deviations.TcDefaultImportPolicyUnsupported(dut) {
		configureRejectRedistributionPolicy(t, dut, isV4)
		defaultRejectPolicyName = redistributeStaticPolicyNameV4
	}
	configureTableConnection(t, dut, isV4, !metricPropagate, defaultRejectPolicyName, oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE)
}

// 1.27.12 setup function
func redistributeIPv6StaticDefaultRejectPolicy(t *testing.T, dut *ondatra.DUTDevice) {
	defaultRejectPolicyName := ""
	if deviations.TcDefaultImportPolicyUnsupported(dut) {
		configureRejectRedistributionPolicy(t, dut, !isV4)
		defaultRejectPolicyName = redistributeStaticPolicyNameV6
	}
	configureTableConnection(t, dut, !isV4, !metricPropagate, defaultRejectPolicyName, oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE)
}

// 1.27.1 validation function
func validateRedistributeIPv4DefaultRejectPolicy(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	validateRedistributeStatic(t, dut, !acceptRoute, isV4, !metricPropagate)
	validateLearnedIPv4Prefix(t, ate, atePort1.Name+".BGP4.peer", "192.168.10.0", medZero, !shouldBePresent)
}

// 1.27.12 validation function
func validateRedistributeIPv6DefaultRejectPolicy(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	validateRedistributeStatic(t, dut, !acceptRoute, !isV4, !metricPropagate)
	validateLearnedIPv6Prefix(t, ate, atePort1.Name+".BGP6.peer", "2024:db8:128:128::", medZero, !shouldBePresent)
}

// 1.27.2 setup function
func redistributeIPv4PrefixRoutePolicy(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	policyPath := gnmi.OC().RoutingPolicy().PolicyDefinition(redistributeStaticPolicyNameV4)

	otgConfig := configureOTG(t, ate)
	otgConfig = configureTrafficFlow(t, otgConfig, isV4, "StaticRoutesV4Flow", atePort1.Name+".IPv4", atePort2.Name+".IPv4", atePort1.MAC, atePort1.IPv4, "192.168.10.0")
	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)

	dutOcRoot := &oc.Root{}
	redistributePolicy := dutOcRoot.GetOrCreateRoutingPolicy()

	redistributePolicyDefinition := redistributePolicy.GetOrCreatePolicyDefinition(redistributeStaticPolicyNameV4)

	v4PrefixSet := redistributePolicy.GetOrCreateDefinedSets().GetOrCreatePrefixSet("prefix-set-v4")
	v4PrefixSet.GetOrCreatePrefix("192.168.10.0/24", "exact")
	if !deviations.SkipPrefixSetMode(dut) {
		v4PrefixSet.SetMode(oc.PrefixSet_Mode_IPV4)
	}

	v4PrefixSet.GetOrCreatePrefix("192.168.20.0/24", "exact")
	if !deviations.SkipPrefixSetMode(dut) {
		v4PrefixSet.SetMode(oc.PrefixSet_Mode_IPV4)
	}

	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet("prefix-set-v4").Config(), v4PrefixSet)

	ipv4PrefixPolicyStatement, err := redistributePolicyDefinition.AppendNewStatement(policyStatementNameV4)
	if err != nil {
		t.Fatalf("failed creating new policy statement, err: %s", err)
	}

	ipv4PrefixPolicyStatementAction := ipv4PrefixPolicyStatement.GetOrCreateActions()
	ipv4PrefixPolicyStatementAction.SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

	ipv4PrefixPolicyStatementConditionsPrefixes := ipv4PrefixPolicyStatement.GetOrCreateConditions().GetOrCreateMatchPrefixSet()
	ipv4PrefixPolicyStatementConditionsPrefixes.SetPrefixSet("prefix-set-v4")
	if !deviations.SkipSetRpMatchSetOptions(dut) {
		ipv4PrefixPolicyStatementConditionsPrefixes.SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY)
	}

	gnmi.Replace(t, dut, policyPath.Config(), redistributePolicyDefinition)
	if deviations.TcAttributePropagationUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.CISCO:
			cfgplugins.DeviationCiscoRoutingPolicyBGPActionSetMed(t, dut, redistributeStaticPolicyNameV4, "statement-v4", "prefix-set-v4", medIPv4, "igp")
		}
	}
	configureTableConnection(t, dut, isV4, metricPropagate, redistributeStaticPolicyNameV4, oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE)

	sendTraffic(t, ate.OTG())
	verifyTraffic(t, ate, otgConfig)
}

// 1.27.2 validation function
func validateRedistributeIPv4PrefixRoutePolicy(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	validateRedistributeStatic(t, dut, acceptRoute, isV4, metricPropagate)
	validatePrefixSetRoutingPolicy(t, dut, isV4)
	validateLearnedIPv4Prefix(t, ate, atePort1.Name+".BGP4.peer", "192.168.10.0", medIPv4, shouldBePresent)
}

// 1.27.5 and 1.27.16 setup function
func redistributeStaticRoutePolicyWithASN(t *testing.T, dut *ondatra.DUTDevice, isV4 bool) {

	redistributeStaticPolicyName := redistributeStaticPolicyNameV4
	policyStatementName := policyStatementNameV4

	if !isV4 {
		redistributeStaticPolicyName = redistributeStaticPolicyNameV6
		policyStatementName = policyStatementNameV6
	}

	policyPath := gnmi.OC().RoutingPolicy().PolicyDefinition(redistributeStaticPolicyName)

	dutOcRoot := &oc.Root{}
	redistributePolicy := dutOcRoot.GetOrCreateRoutingPolicy()

	redistributePolicyDefinition := redistributePolicy.GetOrCreatePolicyDefinition(redistributeStaticPolicyName)
	policyStatement, err := redistributePolicyDefinition.AppendNewStatement(policyStatementName)
	if err != nil {
		t.Fatalf("failed creating new policy statement, err: %s", err)
	}

	policyStatementAction := policyStatement.GetOrCreateActions()
	policyStatementAction.SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	policyStatementAction.GetOrCreateBgpActions().GetOrCreateSetAsPathPrepend().Asn = ygot.Uint32(64512)
	if isV4 {
		policyStatementAction.GetOrCreateBgpActions().GetOrCreateSetAsPathPrepend().Asn = ygot.Uint32(65499)
		policyStatementAction.GetOrCreateBgpActions().GetOrCreateSetAsPathPrepend().SetRepeatN(3)
	}

	gnmi.Replace(t, dut, policyPath.Config(), redistributePolicyDefinition)
	configureTableConnection(t, dut, isV4, metricPropagate, redistributeStaticPolicyName, oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE)
}

// 1.27.6 and 1.27.17 setup function
func redistributeStaticRoutePolicyWithMED(t *testing.T, dut *ondatra.DUTDevice, isV4 bool, medValue uint32) {
	redistributeStaticPolicyName := redistributeStaticPolicyNameV4
	policyStatementName := policyStatementNameV4

	if !isV4 {
		redistributeStaticPolicyName = redistributeStaticPolicyNameV6
		policyStatementName = policyStatementNameV6
	}

	policyPath := gnmi.OC().RoutingPolicy().PolicyDefinition(redistributeStaticPolicyName)

	dutOcRoot := &oc.Root{}
	redistributePolicy := dutOcRoot.GetOrCreateRoutingPolicy()

	redistributePolicyDefinition := redistributePolicy.GetOrCreatePolicyDefinition(redistributeStaticPolicyName)
	policyStatement, err := redistributePolicyDefinition.AppendNewStatement(policyStatementName)
	if err != nil {
		t.Fatalf("failed creating new policy statement, err: %s", err)
	}

	policyStatementAction := policyStatement.GetOrCreateActions()
	policyStatementAction.SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	policyStatement.GetOrCreateActions().GetOrCreateBgpActions().SetSetMed(oc.UnionUint32(medValue))
	if !deviations.BGPSetMedActionUnsupported(dut) {
		policyStatement.GetOrCreateActions().GetOrCreateBgpActions().SetSetMedAction(oc.BgpPolicy_BgpSetMedAction_SET)
	}
	gnmi.Replace(t, dut, policyPath.Config(), redistributePolicyDefinition)

	configureTableConnection(t, dut, isV4, metricPropagate, redistributeStaticPolicyName, oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE)
	if deviations.TcMetricPropagationUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.CISCO:
			cfgplugins.DeviationCiscoTableConnectionsStatictoBGPMetricPropagation(t, dut, isV4, int(medValue), redistributeStaticPolicyName)
		}
	}
}

// 1.27.7 and 1.27.18 setup function
func redistributeStaticRoutePolicyWithLocalPreference(t *testing.T, dut *ondatra.DUTDevice, isV4 bool, localPreferenceValue uint32) {

	redistributeStaticPolicyName := redistributeStaticPolicyNameV4
	policyStatementName := policyStatementNameV4

	if !isV4 {
		redistributeStaticPolicyName = redistributeStaticPolicyNameV6
		policyStatementName = policyStatementNameV6
	}

	policyPath := gnmi.OC().RoutingPolicy().PolicyDefinition(redistributeStaticPolicyName)

	dutOcRoot := &oc.Root{}
	redistributePolicy := dutOcRoot.GetOrCreateRoutingPolicy()

	redistributePolicyDefinition := redistributePolicy.GetOrCreatePolicyDefinition(redistributeStaticPolicyName)
	policyStatement, err := redistributePolicyDefinition.AppendNewStatement(policyStatementName)
	if err != nil {
		t.Fatalf("failed creating new policy statement, err: %s", err)
	}

	policyStatementAction := policyStatement.GetOrCreateActions()
	policyStatementAction.SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	policyStatement.GetOrCreateActions().GetOrCreateBgpActions().SetLocalPref = ygot.Uint32(localPreferenceValue)

	gnmi.Replace(t, dut, policyPath.Config(), redistributePolicyDefinition)
	configureTableConnection(t, dut, isV4, metricPropagate, redistributeStaticPolicyName, oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE)
}

// 1.27.8 and 1.27.19 setup function
func redistributeStaticRoutePolicyWithCommunitySet(t *testing.T, dut *ondatra.DUTDevice, isV4 bool) {

	redistributeStaticPolicyName := redistributeStaticPolicyNameV4
	policyStatementName := policyStatementNameV4
	communitySetName := "community-set-v4"

	if !isV4 {
		redistributeStaticPolicyName = redistributeStaticPolicyNameV6
		policyStatementName = policyStatementNameV6
		communitySetName = "community-set-v6"
	}

	policyPath := gnmi.OC().RoutingPolicy().PolicyDefinition(redistributeStaticPolicyName)
	communityPath := gnmi.OC().RoutingPolicy().DefinedSets().BgpDefinedSets().CommunitySet(communitySetName)

	dutOcRoot := &oc.Root{}
	redistributePolicy := dutOcRoot.GetOrCreateRoutingPolicy()
	redistributePolicyDefinition := redistributePolicy.GetOrCreatePolicyDefinition(redistributeStaticPolicyName)

	communitySet := dutOcRoot.GetOrCreateRoutingPolicy()
	communitySetPolicyDefinition := communitySet.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(communitySetName)
	communitySetPolicyDefinition.SetCommunityMember([]oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{oc.UnionString("64512:100")})

	gnmi.Replace(t, dut, policyPath.Config(), redistributePolicyDefinition)
	gnmi.Replace(t, dut, communityPath.Config(), communitySetPolicyDefinition)

	policyStatement, err := redistributePolicyDefinition.AppendNewStatement(policyStatementName)
	if err != nil {
		t.Fatalf("failed creating new policy statement, err: %s", err)
	}

	policyStatementAction := policyStatement.GetOrCreateActions()
	policyStatementAction.SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

	if !deviations.BgpCommunitySetRefsUnsupported(dut) {
		policyStatementAction.GetOrCreateBgpActions().GetOrCreateSetCommunity().SetOptions(oc.BgpPolicy_BgpSetCommunityOptionType_ADD)
		policyStatementAction.GetOrCreateBgpActions().GetOrCreateSetCommunity().GetOrCreateReference().SetCommunitySetRefs([]string{communitySetName})
	}
	gnmi.Replace(t, dut, policyPath.Config(), redistributePolicyDefinition)
	if deviations.BgpCommunitySetRefsUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.CISCO:
			cfgplugins.DeviationCiscoRoutingPolicyBGPActionSetCommunity(t, dut, redistributeStaticPolicyName, policyStatementName, communitySetName)
		case ondatra.JUNIPER:
			cfgplugins.DeviationJuniperRoutingPolicyBGPActionSetCommunity(t, dut, redistributeStaticPolicyName, policyStatementName, communitySetName)
		default:
			t.Fatalf("BgpCommunitySetRefsUnsupported deviation needs cli configuration for vendor %s which is not defined", dut.Vendor())
		}

	}

	configureTableConnection(t, dut, isV4, metricPropagate, redistributeStaticPolicyName, oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE)
}

// 1.27.9, 1.27.10, 1.27.20 and 1.27.21 setup function
func redistributeStaticRoutePolicyWithTagSet(t *testing.T, dut *ondatra.DUTDevice, isV4 bool, tagSetValue uint32) {

	redistributeStaticPolicyName := redistributeStaticPolicyNameV4
	tagSetName := "tag-set-v4"
	policyStatementName := policyStatementNameV4
	if !isV4 {
		redistributeStaticPolicyName = redistributeStaticPolicyNameV6
		tagSetName = "tag-set-v6"
		policyStatementName = policyStatementNameV6
	}

	policyPath := gnmi.OC().RoutingPolicy().PolicyDefinition(redistributeStaticPolicyName)
	tagSetPath := gnmi.OC().RoutingPolicy().DefinedSets().TagSet(tagSetName)

	dutOcRoot := &oc.Root{}
	redistributePolicy := dutOcRoot.GetOrCreateRoutingPolicy()
	redistributePolicyDefinition := redistributePolicy.GetOrCreatePolicyDefinition(redistributeStaticPolicyName)
	tagSet := dutOcRoot.GetOrCreateRoutingPolicy()
	tagSetPolicyDefinition := tagSet.GetOrCreateDefinedSets().GetOrCreateTagSet(tagSetName)
	tagSetPolicyDefinition.SetTagValue([]oc.RoutingPolicy_DefinedSets_TagSet_TagValue_Union{oc.UnionString(fmt.Sprintf("%v", tagSetValue))})
	gnmi.Replace(t, dut, tagSetPath.Config(), tagSetPolicyDefinition)

	policyStatement, err := redistributePolicyDefinition.AppendNewStatement(policyStatementName)
	if err != nil {
		t.Fatalf("failed creating new policy statement, err: %s", err)
	}

	policyStatementCondition := policyStatement.GetOrCreateConditions()
	if !deviations.SkipSetRpMatchSetOptions(dut) {
		policyStatementCondition.GetOrCreateMatchTagSet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY)
	}
	policyStatementCondition.GetOrCreateMatchTagSet().SetTagSet(tagSetName)
	policyStatementAction := policyStatement.GetOrCreateActions()
	policyStatementAction.SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	gnmi.Replace(t, dut, policyPath.Config(), redistributePolicyDefinition)

	configureTableConnection(t, dut, isV4, !metricPropagate, redistributeStaticPolicyName, oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE)
}

// 1.27.11 and 1.27.22 setup function
func redistributeNullNextHopStaticRoute(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, isV4 bool) {

	redistributeStaticPolicyName := redistributeStaticPolicyNameV4
	tagValue := "40"
	policyStatementName := policyStatementNameV4
	ipRoute := "192.168.20.0/24"
	routeNextHop := "192.168.1.9"
	if !isV4 {
		redistributeStaticPolicyName = redistributeStaticPolicyNameV6
		tagValue = "60"
		policyStatementName = policyStatementNameV6
		ipRoute = "2024:db8:64:64::/64"
		routeNextHop = "2001:DB8::9"
	}

	policyPath := gnmi.OC().RoutingPolicy().PolicyDefinition(redistributeStaticPolicyName)
	staticPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))

	otgConfig := configureOTG(t, ate)
	if isV4 {
		otgConfig = configureTrafficFlow(t, otgConfig, isV4, "StaticDropRoutesV4Flow", atePort3.Name+".IPv4", atePort2.Name+".IPv4", atePort3.MAC, atePort3.IPv4, "192.168.20.0")
	} else {
		otgConfig = configureTrafficFlow(t, otgConfig, isV4, "StaticDropRoutesV6Flow", atePort3.Name+".IPv6", atePort2.Name+".IPv6", atePort3.MAC, atePort3.IPv6, "2024:db8:64:64::")
	}
	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)

	dutOcRoot := &oc.Root{}
	networkInstance := dutOcRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	networkInstanceProtocolStatic := networkInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	networkInstanceProtocolStatic.SetEnabled(true)
	ipStaticRoute := networkInstanceProtocolStatic.GetOrCreateStatic(ipRoute)
	ipStaticRoute.SetSetTag(oc.UnionString(tagValue))

	ipStaticRouteNextHop := ipStaticRoute.GetOrCreateNextHop("0")
	ipStaticRouteNextHop.SetNextHop(oc.UnionString("DROP"))
	gnmi.Update(t, dut, staticPath.Config(), networkInstanceProtocolStatic)

	redistributePolicy := dutOcRoot.GetOrCreateRoutingPolicy()
	redistributePolicyDefinition := redistributePolicy.GetOrCreatePolicyDefinition(redistributeStaticPolicyName)

	ipPrefixPolicyStatement, err := redistributePolicyDefinition.AppendNewStatement(policyStatementName)
	if err != nil {
		t.Fatalf("failed creating new policy statement, err: %s", err)
	}

	ipPrefixPolicyStatementAction := ipPrefixPolicyStatement.GetOrCreateActions()
	ipPrefixPolicyStatementAction.GetOrCreateBgpActions().SetSetNextHop(oc.UnionString(routeNextHop))
	ipPrefixPolicyStatementAction.SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	gnmi.Replace(t, dut, policyPath.Config(), redistributePolicyDefinition)

	configureTableConnection(t, dut, isV4, !metricPropagate, redistributeStaticPolicyName, oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE)

	// Sending traffic to network via dut having static-route to drop it.
	// Traffic must be dropped by the dut irrespective of the bgp advertised-route
	// having updated next-hop, considering existing static-route is preferred over bgp.
	// Commenting traffic validation for now
	/*
		sendTraffic(t, ate.OTG())
		verifyTraffic(t, ate, otgConfig)
	*/
}

// 1.27.13 setup function
func redistributeIPv6StaticRoutePolicy(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	policyPath := gnmi.OC().RoutingPolicy().PolicyDefinition(redistributeStaticPolicyNameV6)

	otgConfig := configureOTG(t, ate)
	otgConfig = configureTrafficFlow(t, otgConfig, !isV4, "StaticRoutesV6Flow", atePort1.Name+".IPv6", atePort2.Name+".IPv6", atePort1.MAC, atePort1.IPv6, "2024:db8:128:128::")
	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)

	dutOcRoot := &oc.Root{}
	redistributePolicy := dutOcRoot.GetOrCreateRoutingPolicy()
	redistributePolicyDefinition := redistributePolicy.GetOrCreatePolicyDefinition(redistributeStaticPolicyNameV6)

	v6PrefixSet := redistributePolicy.GetOrCreateDefinedSets().GetOrCreatePrefixSet("prefix-set-v6")
	v6PrefixSet.GetOrCreatePrefix("2024:db8:128:128::/64", "exact")
	if !deviations.SkipPrefixSetMode(dut) {
		v6PrefixSet.SetMode(oc.PrefixSet_Mode_IPV6)
	}

	v6PrefixSet.GetOrCreatePrefix("2024:db8:64:64::/64", "exact")
	if !deviations.SkipPrefixSetMode(dut) {
		v6PrefixSet.SetMode(oc.PrefixSet_Mode_IPV6)
	}

	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet("prefix-set-v6").Config(), v6PrefixSet)

	ipv6PrefixPolicyStatement, err := redistributePolicyDefinition.AppendNewStatement(policyStatementNameV6)
	if err != nil {
		t.Fatalf("failed creating new policy statement, err: %s", err)
	}

	ipv6PrefixPolicyStatementAction := ipv6PrefixPolicyStatement.GetOrCreateActions()
	ipv6PrefixPolicyStatementAction.SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

	ipv6PrefixPolicyStatementConditionsPrefixes := ipv6PrefixPolicyStatement.GetOrCreateConditions().GetOrCreateMatchPrefixSet()
	ipv6PrefixPolicyStatementConditionsPrefixes.SetPrefixSet("prefix-set-v6")
	if !deviations.SkipSetRpMatchSetOptions(dut) {
		ipv6PrefixPolicyStatementConditionsPrefixes.SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY)
	}

	gnmi.Replace(t, dut, policyPath.Config(), redistributePolicyDefinition)

	configureTableConnection(t, dut, !isV4, metricPropagate, redistributeStaticPolicyNameV6, oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE)
	if deviations.TcAttributePropagationUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.CISCO:
			cfgplugins.DeviationCiscoRoutingPolicyBGPActionSetMed(t, dut, redistributeStaticPolicyNameV6, "statement-v6", "prefix-set-v6", medIPv6, "igp")
		}
	}

	sendTraffic(t, ate.OTG())
	verifyTraffic(t, ate, otgConfig)
}

// 1.27.13 validation function
func validateRedistributeIPv6RoutePolicy(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	validateRedistributeStatic(t, dut, acceptRoute, !isV4, metricPropagate)
	validatePrefixSetRoutingPolicy(t, dut, !isV4)
	validateLearnedIPv6Prefix(t, ate, atePort1.Name+".BGP6.peer", "2024:db8:128:128::", medIPv6, shouldBePresent)
}

// 1.27.5 and 1.27.16 validation function
func validatePrefixASN(t *testing.T, ate *ondatra.ATEDevice, isV4 bool, bgpPeerName, subnet string, wantASPath []uint32) {

	foundPrefix := false

	if isV4 {
		prefixPath := gnmi.OTG().BgpPeer(bgpPeerName).UnicastIpv4PrefixAny()
		prefix, ok := gnmi.WatchAll(t, ate.OTG(), prefixPath.State(), 20*time.Second, func(val *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv4Prefix]) bool {
			prefix, _ := val.Val()
			if prefix.GetAddress() == subnet {
				foundPrefix = true
				gotASPath := prefix.AsPath[len(prefix.AsPath)-1].GetAsNumbers()
				t.Logf("Prefix %v learned with ASN : %v", prefix.GetAddress(), gotASPath)
				return reflect.DeepEqual(gotASPath, wantASPath)
			}
			return false
		}).Await(t)
		if !ok {
			pfx, _ := prefix.Val()
			t.Fatalf("Prefix not updated with required as-path. Got %v, want %v", pfx.AsPath[len(pfx.AsPath)-1].GetAsNumbers(), wantASPath)
		}
	} else {
		prefixPath := gnmi.OTG().BgpPeer(bgpPeerName).UnicastIpv6PrefixAny()
		prefix, ok := gnmi.WatchAll(t, ate.OTG(), prefixPath.State(), 20*time.Second, func(val *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv6Prefix]) bool {
			prefix, _ := val.Val()
			if prefix.GetAddress() == subnet {
				foundPrefix = true
				gotASPath := prefix.AsPath[len(prefix.AsPath)-1].GetAsNumbers()
				t.Logf("Prefix %v learned with ASN : %v", prefix.GetAddress(), gotASPath)
				return reflect.DeepEqual(gotASPath, wantASPath)
			}
			return false
		}).Await(t)
		if !ok {
			pfx, _ := prefix.Val()
			t.Fatalf("Prefix not updated with required as-path. Got %v, want %v", pfx.AsPath[len(pfx.AsPath)-1].GetAsNumbers(), wantASPath)
		}
	}
	if !foundPrefix {
		t.Fatalf("Prefix %v not present in OTG", subnet)
	}
}

// 1.27.7 and 1.27.18 validation function
func validatePrefixLocalPreference(t *testing.T, ate *ondatra.ATEDevice, isV4 bool, bgpPeerName, subnet string, wantLocalPreference uint32) {

	foundPrefix := false
	if isV4 {
		prefixPath := gnmi.OTG().BgpPeer(bgpPeerName).UnicastIpv4PrefixAny()
		prefix, ok := gnmi.WatchAll(t, ate.OTG(), prefixPath.State(), 10*time.Second, func(val *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv4Prefix]) bool {
			prefix, _ := val.Val()
			if prefix.GetAddress() == subnet {
				foundPrefix = true
				gotLocalPreference := prefix.GetLocalPreference()
				t.Logf("Prefix %v learned with localPreference : %v", prefix.GetAddress(), gotLocalPreference)
				return gotLocalPreference == wantLocalPreference
			}
			return false
		}).Await(t)
		if !ok {
			pfx, _ := prefix.Val()
			t.Fatalf("Prefix not updated with the local-preference. Got %v, want %v", pfx.GetLocalPreference(), wantLocalPreference)
		}
	} else {
		prefixPath := gnmi.OTG().BgpPeer(bgpPeerName).UnicastIpv6PrefixAny()
		prefix, ok := gnmi.WatchAll(t, ate.OTG(), prefixPath.State(), 10*time.Second, func(val *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv6Prefix]) bool {
			prefix, _ := val.Val()
			if prefix.GetAddress() == subnet {
				foundPrefix = true
				gotLocalPreference := prefix.GetLocalPreference()
				t.Logf("Prefix %v learned with localPreference : %v", prefix.GetAddress(), gotLocalPreference)
				return gotLocalPreference == wantLocalPreference
			}
			return false
		}).Await(t)
		if !ok {
			pfx, _ := prefix.Val()
			t.Fatalf("Prefix not updated with the local-preference. Got %v, want %v", pfx.GetLocalPreference(), wantLocalPreference)
		}
	}
	if !foundPrefix {
		t.Fatalf("Prefix %v not present in OTG", subnet)
	}
}

// 1.27.8 and 1.27.19 validation function
func validatePrefixCommunitySet(t *testing.T, ate *ondatra.ATEDevice, isV4 bool, bgpPeerName, subnet, wantCommunitySet string) {

	foundPrefix := false
	if isV4 {
		prefixPath := gnmi.OTG().BgpPeer(bgpPeerName).UnicastIpv4PrefixAny()
		prefix, ok := gnmi.WatchAll(t, ate.OTG(), prefixPath.State(), 10*time.Second, func(val *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv4Prefix]) bool {
			prefix, _ := val.Val()
			if prefix.GetAddress() == subnet {
				foundPrefix = true
				var gotCommunitySet string
				for _, community := range prefix.Community {
					gotCommunityNumber := community.GetCustomAsNumber()
					gotCommunityValue := community.GetCustomAsValue()
					gotCommunitySet = fmt.Sprint(gotCommunityNumber) + ":" + fmt.Sprint(gotCommunityValue)
				}
				t.Logf("Prefix %v learned with CommunitySet : %v", prefix.GetAddress(), gotCommunitySet)
				return gotCommunitySet == wantCommunitySet
			}
			return false
		}).Await(t)
		if !ok {
			pfx, _ := prefix.Val()
			var gotCS string
			for _, community := range pfx.Community {
				gotCN := community.GetCustomAsNumber()
				gotCV := community.GetCustomAsValue()
				gotCS = fmt.Sprint(gotCN) + ":" + fmt.Sprint(gotCV)
			}
			t.Fatalf("Prefix not updated with the community-set. Got %v, want %v", gotCS, wantCommunitySet)
		}
	} else {
		prefixPath := gnmi.OTG().BgpPeer(bgpPeerName).UnicastIpv6PrefixAny()
		prefix, ok := gnmi.WatchAll(t, ate.OTG(), prefixPath.State(), 10*time.Second, func(val *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv6Prefix]) bool {
			prefix, _ := val.Val()
			if prefix.GetAddress() == subnet {
				foundPrefix = true
				var gotCommunitySet string
				for _, community := range prefix.Community {
					gotCommunityNumber := community.GetCustomAsNumber()
					gotCommunityValue := community.GetCustomAsValue()
					gotCommunitySet = fmt.Sprint(gotCommunityNumber) + ":" + fmt.Sprint(gotCommunityValue)
				}
				t.Logf("Prefix %v learned with CommunitySet : %v", prefix.GetAddress(), gotCommunitySet)
				return gotCommunitySet == wantCommunitySet
			}
			return false
		}).Await(t)
		if !ok {
			pfx, _ := prefix.Val()
			var gotCS string
			for _, community := range pfx.Community {
				gotCN := community.GetCustomAsNumber()
				gotCV := community.GetCustomAsValue()
				gotCS = fmt.Sprint(gotCN) + ":" + fmt.Sprint(gotCV)
			}
			t.Fatalf("Prefix not updated with the community-set. Got %v, want %v", gotCS, wantCommunitySet)
		}
	}

	if !foundPrefix {
		t.Fatalf("Prefix %v not present in OTG", subnet)
	}
}

// 1.27.9, 1.27.10, 1.27.20 and 1.27.21 validation function
func validateRedistributeRouteWithTagSet(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, isV4, shouldBePresent bool) {

	redistributeStaticPolicyName := redistributeStaticPolicyNameV4
	af := oc.Types_ADDRESS_FAMILY_IPV4
	tagSet := "tag-set-v4"
	policyStatementName := policyStatementNameV4
	if !isV4 {
		redistributeStaticPolicyName = redistributeStaticPolicyNameV6
		af = oc.Types_ADDRESS_FAMILY_IPV6
		tagSet = "tag-set-v6"
		policyStatementName = policyStatementNameV6
	}

	if !deviations.TcSubscriptionUnsupported(dut) {
		tcState := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).TableConnection(
			oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
			oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
			af).State())

		importPolicies := tcState.GetImportPolicy()
		found := false
		for _, iPolicy := range importPolicies {
			if iPolicy == redistributeStaticPolicyName {
				found = true
			}
		}

		if !found {
			t.Fatal("expected import policy is not configured")
		}
	}

	var foundPDef oc.RoutingPolicy_PolicyDefinition
	policyDef := gnmi.GetAll(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinitionAny().State())
	for _, pDef := range policyDef {
		if pDef.GetName() == redistributeStaticPolicyName {
			foundPDef = *pDef
		}
	}

	if foundPDef.GetName() != redistributeStaticPolicyName {
		t.Fatal("Expected import policy is not configured")
	}
	if foundPDef.GetStatement(policyStatementName).GetConditions().GetOrCreateMatchTagSet().GetTagSet() != tagSet {
		if deviations.RoutingPolicyTagSetEmbedded(dut) {
			t.Log("Tag-set are embeded in the policy statement.")
		} else {
			t.Fatal("Expected tag-set is not configured")
		}
	}
	if foundPDef.GetStatement(policyStatementName).GetConditions().GetOrCreateMatchTagSet().GetMatchSetOptions() != oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY {
		if deviations.RoutingPolicyTagSetEmbedded(dut) {
			t.Log("Tag-set are embeded in the policy statement.")
		} else {
			t.Fatal("Expected match-set-option for tag-set is not configured")
		}
	}

	if isV4 {
		if deviations.SkipSettingDisableMetricPropagation(dut) {
			validateLearnedIPv4Prefix(t, ate, atePort1.Name+".BGP4.peer", "192.168.10.0", medIPv4, shouldBePresent)
		} else {
			validateLearnedIPv4Prefix(t, ate, atePort1.Name+".BGP4.peer", "192.168.10.0", medZero, shouldBePresent)
		}
	} else {
		if deviations.SkipSettingDisableMetricPropagation(dut) {
			validateLearnedIPv6Prefix(t, ate, atePort1.Name+".BGP6.peer", "2024:db8:128:128::", medIPv6, shouldBePresent)
		} else {
			validateLearnedIPv6Prefix(t, ate, atePort1.Name+".BGP6.peer", "2024:db8:128:128::", medZero, shouldBePresent)
		}
	}
}

// 1.27.11 and 1.27.22 validation function
func validateRedistributeNullNextHopStaticRoute(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, isV4 bool) {

	redistributeStaticPolicyName := redistributeStaticPolicyNameV4
	policyStatementName := policyStatementNameV4
	addressFamily := oc.Types_ADDRESS_FAMILY_IPV4
	nextHop := "192.168.1.9"
	if !isV4 {
		redistributeStaticPolicyName = redistributeStaticPolicyNameV6
		policyStatementName = policyStatementNameV6
		addressFamily = oc.Types_ADDRESS_FAMILY_IPV6
		nextHop = "2001:db8::9"
	}

	if !deviations.TcSubscriptionUnsupported(dut) {
		tcState := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).TableConnection(
			oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
			oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
			addressFamily).State())

		importPolicies := tcState.GetImportPolicy()
		found := false
		for _, iPolicy := range importPolicies {
			if iPolicy == redistributeStaticPolicyName {
				found = true
			}
		}

		if !found {
			t.Fatal("expected import policy is not configured")
		}
	}

	var foundPDef oc.RoutingPolicy_PolicyDefinition
	policyDef := gnmi.GetAll(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinitionAny().State())
	for _, pDef := range policyDef {
		if pDef.GetName() == redistributeStaticPolicyName {
			foundPDef = *pDef
		}
	}

	if foundPDef.GetName() != redistributeStaticPolicyName {
		t.Fatal("Expected import policy is not configured")
	}

	if foundPDef.GetStatement(policyStatementName).GetActions().GetBgpActions().GetSetNextHop() != oc.UnionString(nextHop) {
		t.Fatal("Expected next-hop is not configured")
	}

	if isV4 {
		validateLearnedIPv4Prefix(t, ate, atePort1.Name+".BGP4.peer", "192.168.20.0", medZero, shouldBePresent)
	} else {
		validateLearnedIPv6Prefix(t, ate, atePort1.Name+".BGP6.peer", "2024:db8:64:64::", medZero, shouldBePresent)
	}
}

// Used by multiple IPv4 test validations for route presence and MED value
func validateLearnedIPv4Prefix(t *testing.T, ate *ondatra.ATEDevice, bgpPeerName, subnet string, expectedMED uint32, shouldBePresent bool) {
	// Track if the expected prefix is found
	found := false
	medMatched := false
	var gotMed uint32
	gnmi.WatchAll(t, ate.OTG(), gnmi.OTG().BgpPeer(bgpPeerName).UnicastIpv4PrefixAny().State(),
		time.Minute, func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv4Prefix]) bool {
			if !v.IsPresent() {
				return false
			}

			bgpPrefix, _ := v.Val()
			if bgpPrefix.Address != nil && bgpPrefix.GetAddress() == subnet {
				found = true
				t.Logf("Prefix received on OTG is correct, got prefix %v, want prefix %v", bgpPrefix.GetAddress(), subnet)

				// When metric-propagation is disabled, MED is not included in the route update, thus that leaf stop showing up under
				// subscription and due to that WatchAll still holds old value. Here we re-query the MED and udpate it to zero if MED leaf
				// is not present.
				medVal := gnmi.Lookup(t, ate.OTG(), gnmi.OTG().BgpPeer(bgpPeerName).UnicastIpv4Prefix(bgpPrefix.GetAddress(), bgpPrefix.GetPrefixLength(), bgpPrefix.GetOrigin(), bgpPrefix.GetPathId()).MultiExitDiscriminator().State())

				if !medVal.IsPresent() {
					gotMed = 0
				} else {
					gotMed, _ = medVal.Val()
				}
				t.Logf("Prefix MED %d", gotMed)

				if gotMed == expectedMED {
					medMatched = true
					return true
				}
			}
			return false
		}).Await(t)

	if shouldBePresent {
		if !found {
			t.Errorf("No Route found for prefix %s", subnet)
		} else if !medMatched {
			t.Errorf("For Prefix %v, got MED %d, want MED %d", subnet, gotMed, expectedMED)
		}
	}
}

// Used by multiple IPv6 test validations for route presence and MED value
func validateLearnedIPv6Prefix(t *testing.T, ate *ondatra.ATEDevice, bgpPeerName, subnet string, expectedMED uint32, shouldBePresent bool) {
	// Track if the expected prefix is found
	found := false
	medMatched := false
	var gotMed uint32
	gnmi.WatchAll(t, ate.OTG(), gnmi.OTG().BgpPeer(bgpPeerName).UnicastIpv6PrefixAny().State(),
		time.Minute, func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv6Prefix]) bool {
			if !v.IsPresent() {
				return false
			}

			bgpPrefix, _ := v.Val()
			if bgpPrefix.Address != nil && bgpPrefix.GetAddress() == subnet {
				found = true
				t.Logf("Prefix received on OTG is correct, got prefix %v, want prefix %v", bgpPrefix.GetAddress(), subnet)

				// When metric-propagation is disabled, MED is not included in the route update, thus that leaf stop showing up under
				// subscription and due to that WatchAll still holds old value. Here we re-query the MED and udpate it to zero if MED leaf
				// is not present.
				medVal := gnmi.Lookup(t, ate.OTG(), gnmi.OTG().BgpPeer(bgpPeerName).UnicastIpv6Prefix(bgpPrefix.GetAddress(), bgpPrefix.GetPrefixLength(), bgpPrefix.GetOrigin(), bgpPrefix.GetPathId()).MultiExitDiscriminator().State())

				if !medVal.IsPresent() {
					gotMed = 0
				} else {
					gotMed, _ = medVal.Val()
				}
				t.Logf("Prefix MED %d", gotMed)

				if gotMed == expectedMED {
					medMatched = true
					return true
				}
			}
			return false
		}).Await(t)

	if shouldBePresent {
		if !found {
			t.Errorf("No Route found for prefix %s", subnet)
		} else if !medMatched {
			t.Errorf("For Prefix %v, got MED %d, want MED %d", subnet, gotMed, expectedMED)
		}
	}
}

func TestBGPStaticRouteRedistribution(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	configureDUT(t, dut)
	otgConfig := configureOTG(t, ate)
	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)

	awaitBGPEstablished(t, dut, []string{atePort1.IPv4, atePort3.IPv4, atePort1.IPv6, atePort3.IPv6})

	type testCase struct {
		name     string
		setup    func()
		validate func()
	}

	testCases := []testCase{
		// 1.27.1
		{
			name:     "1.27.1 redistribute-ipv4-ipv6-default-reject-policy",
			setup:    func() { redistributeIPv4StaticDefaultRejectPolicy(t, dut) },
			validate: func() { validateRedistributeIPv4DefaultRejectPolicy(t, dut, ate) },
		},
		// 1.27.2
		{
			name:     "1.27.2 redistribute-ipv4-prefix-route-policy",
			setup:    func() { redistributeIPv4PrefixRoutePolicy(t, dut, ate) },
			validate: func() { validateRedistributeIPv4PrefixRoutePolicy(t, dut, ate) },
		},
		// 1.27.3
		{
			name:     "1.27.3 redistribute-ipv4-static-routes-with-metric-propagation-disabled",
			setup:    func() { redistributeIPv4Static(t, dut) },
			validate: func() { validateRedistributeIPv4Static(t, dut, ate) },
		},
		// 1.27.4
		{
			name:     "1.27.4 redistribute-ipv4-static-routes-with-metric-propagation-enabled",
			setup:    func() { redistributeIPv4StaticWithMetricPropagation(t, dut) },
			validate: func() { validateRedistributeIPv4StaticWithMetricPropagation(t, dut, ate) },
		},
		// 1.27.5
		{
			name:  "1.27.5 redistribute-ipv4-route-policy-as-prepend",
			setup: func() { redistributeStaticRoutePolicyWithASN(t, dut, isV4) },
			validate: func() {
				validatePrefixASN(t, ate, isV4, atePort1.Name+".BGP4.peer", "192.168.10.0", []uint32{64512, 65499, 65499, 65499})
			},
		},
		// 1.27.6
		{
			name:  "1.27.6 redistribute-ipv4-route-policy-med",
			setup: func() { redistributeStaticRoutePolicyWithMED(t, dut, isV4, medNonZero) },
			validate: func() {
				validateLearnedIPv4Prefix(t, ate, atePort1.Name+".BGP4.peer", "192.168.10.0", medNonZero, shouldBePresent)
			},
		},
		// 1.27.7
		{
			name:  "1.27.7 redistribute-ipv4-route-policy-local-preference",
			setup: func() { redistributeStaticRoutePolicyWithLocalPreference(t, dut, isV4, localPreference) },
			validate: func() {
				validatePrefixLocalPreference(t, ate, isV4, atePort3.Name+".BGP4.peer", "192.168.10.0", localPreference)
			},
		},
		// 1.27.8
		{
			name:  "1.27.8 redistribute-ipv4-route-policy-community-set",
			setup: func() { redistributeStaticRoutePolicyWithCommunitySet(t, dut, isV4) },
			validate: func() {
				validatePrefixCommunitySet(t, ate, isV4, atePort3.Name+".BGP4.peer", "192.168.10.0", "64512:100")
			},
		},
		// 1.27.9
		{
			name:     "1.27.9 redistribute-ipv4-route-policy-unmatched-tag",
			setup:    func() { redistributeStaticRoutePolicyWithTagSet(t, dut, isV4, 100) },
			validate: func() { validateRedistributeRouteWithTagSet(t, dut, ate, isV4, !shouldBePresent) },
		},
		// 1.27.10
		{
			name:     "1.27.10 redistribute-ipv4-route-policy-matched-set",
			setup:    func() { redistributeStaticRoutePolicyWithTagSet(t, dut, isV4, 40) },
			validate: func() { validateRedistributeRouteWithTagSet(t, dut, ate, isV4, shouldBePresent) },
		},
		// 1.27.11
		{
			name:     "1.27.11 redistribute-ipv4-route-policy-nexthop",
			setup:    func() { redistributeNullNextHopStaticRoute(t, dut, ate, isV4) },
			validate: func() { validateRedistributeNullNextHopStaticRoute(t, dut, ate, isV4) },
		},
		// 1.27.12
		{
			name:     "1.27.12 redistribute-ipv6-default-reject-policy",
			setup:    func() { redistributeIPv6StaticDefaultRejectPolicy(t, dut) },
			validate: func() { validateRedistributeIPv6DefaultRejectPolicy(t, dut, ate) },
		},
		// 1.27.13
		{
			name:     "1.27.13 redistribute-ipv6-route-policy",
			setup:    func() { redistributeIPv6StaticRoutePolicy(t, dut, ate) },
			validate: func() { validateRedistributeIPv6RoutePolicy(t, dut, ate) },
		},
		// 1.27.14
		{
			name:     "1.27.14 redistribute-ipv6-static-routes-with-metric-propagation-disabled",
			setup:    func() { redistributeIPv6Static(t, dut) },
			validate: func() { validateRedistributeIPv6Static(t, dut, ate) },
		},
		// 1.27.15
		{
			name:     "1.27.15 redistribute-ipv6-static-routes-with-metric-propagation-enabled",
			setup:    func() { redistributeIPv6StaticWithMetricPropagation(t, dut) },
			validate: func() { validateRedistributeIPv6StaticWithMetricPropagation(t, dut, ate) },
		},
		// 1.27.16
		{
			name:  "1.27.16 redistribute-ipv6-route-policy-as-prepend",
			setup: func() { redistributeStaticRoutePolicyWithASN(t, dut, !isV4) },
			validate: func() {
				validatePrefixASN(t, ate, !isV4, atePort1.Name+".BGP6.peer", "2024:db8:128:128::", []uint32{64512, 64512})
			},
		},
		// 1.27.17
		{
			name:  "1.27.17 redistribute-ipv6-route-policy-med",
			setup: func() { redistributeStaticRoutePolicyWithMED(t, dut, !isV4, medNonZero) },
			validate: func() {
				validateLearnedIPv6Prefix(t, ate, atePort1.Name+".BGP6.peer", "2024:db8:128:128::", medNonZero, shouldBePresent)
			},
		},
		// 1.27.18
		{
			name:  "1.27.18 redistribute-ipv6-route-policy-local-preference",
			setup: func() { redistributeStaticRoutePolicyWithLocalPreference(t, dut, !isV4, localPreference) },
			validate: func() {
				validatePrefixLocalPreference(t, ate, !isV4, atePort3.Name+".BGP6.peer", "2024:db8:128:128::", localPreference)
			},
		},
		// 1.27.19
		{
			name:  "1.27.19 redistribute-ipv6-route-policy-community-set",
			setup: func() { redistributeStaticRoutePolicyWithCommunitySet(t, dut, !isV4) },
			validate: func() {
				validatePrefixCommunitySet(t, ate, !isV4, atePort3.Name+".BGP6.peer", "2024:db8:128:128::", "64512:100")
			},
		},
		// 1.27.20
		{
			name:     "1.27.20 redistribute-ipv6-route-policy-unmatched-tag",
			setup:    func() { redistributeStaticRoutePolicyWithTagSet(t, dut, !isV4, 100) },
			validate: func() { validateRedistributeRouteWithTagSet(t, dut, ate, !isV4, !shouldBePresent) },
		},
		// 1.27.21
		{
			name:     "1.27.21 redistribute-ipv6-route-policy-matched-set",
			setup:    func() { redistributeStaticRoutePolicyWithTagSet(t, dut, !isV4, 60) },
			validate: func() { validateRedistributeRouteWithTagSet(t, dut, ate, !isV4, shouldBePresent) },
		},
		// 1.27.22
		{
			name:     "1.27.22 redistribute-ipv6-route-policy-nexthop",
			setup:    func() { redistributeNullNextHopStaticRoute(t, dut, ate, !isV4) },
			validate: func() { validateRedistributeNullNextHopStaticRoute(t, dut, ate, !isV4) },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup()
			tc.validate()
		})
	}
}

// Configure default reject route-policy with no entries
func configureRejectRedistributionPolicy(t *testing.T, dut *ondatra.DUTDevice, isV4 bool) {
	t.Helper()

	dutOcRoot := &oc.Root{}
	rp := dutOcRoot.GetOrCreateRoutingPolicy()

	redistributeStaticPolicyName := redistributeStaticPolicyNameV4
	if !isV4 {
		redistributeStaticPolicyName = redistributeStaticPolicyNameV6
	}
	rpConfPath := gnmi.OC().RoutingPolicy()
	gnmi.Replace(t, dut, rpConfPath.PolicyDefinition(redistributeStaticPolicyName).Config(), rp.GetOrCreatePolicyDefinition(redistributeStaticPolicyName))
}
