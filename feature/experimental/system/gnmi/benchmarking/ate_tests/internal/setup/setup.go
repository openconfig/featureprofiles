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

// Package setup is scoped only to be used for scripts in path
// feature/experimental/system/gnmi/benchmarking/ate_tests/
// Do not use elsewhere.
package setup

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	// ISISInstance is ISIS instance name.
	ISISInstance = "DEFAULT"
	// PeerGrpName is BGP peer group name.
	PeerGrpName = "BGP-PEER-GROUP"

	// DUTAs is DUT AS.
	DUTAs = 64500
	// ATEAs is ATE AS.
	ATEAs = 64501
	// ATEAs2 is ATE source port AS
	ATEAs2 = 64502
	// ISISMetric is Metric for ISIS
	ISISMetric = 100
	// RouteCount for both BGP and ISIS
	RouteCount = 200
	// AdvertiseBGPRoutesv4 is the starting IPv4 address advertised by ATE Port 1.
	AdvertiseBGPRoutesv4 = "203.0.113.1"

	dutAreaAddress        = "49.0001"
	dutSysID              = "1920.0000.2001"
	dutStartIPAddr        = "192.0.2.1"
	ateStartIPAddr        = "192.0.2.2"
	plenIPv4              = 30
	authPassword          = "ISISAuthPassword"
	advertiseISISRoutesv4 = "198.18.0.0"
	setALLOWPolicy        = "ALLOW"
)

// DUTIPList, ATEIPList are lists of DUT and ATE interface ip addresses.
// ISISMetricList, ISISSetBitList are ISIS metric and setbit lists.
var (
	DUTIPList      = make(map[string]net.IP)
	ATEIPList      = make(map[string]net.IP)
	ISISMetricList []uint32
	ISISSetBitList []bool
)

// buildPortIPs generates ip addresses for the ports in binding file.
// (Both DUT and ATE ports).
func buildPortIPs(dut *ondatra.DUTDevice) {
	var dutIPIndex, ipSubnet, ateIPIndex int = 1, 2, 2
	var endSubnetIndex = 253
	for _, dp := range dut.Ports() {
		dutNextIP := nextIP(net.ParseIP(dutStartIPAddr), dutIPIndex, ipSubnet)
		ateNextIP := nextIP(net.ParseIP(ateStartIPAddr), ateIPIndex, ipSubnet)
		DUTIPList[dp.ID()] = dutNextIP
		ATEIPList[dp.ID()] = ateNextIP

		// Increment DUT and ATE host ip index by 4.
		dutIPIndex = dutIPIndex + 4
		ateIPIndex = ateIPIndex + 4

		// Reset DUT and ATE ip indexes when it is greater than endSubnetIndex.
		if dutIPIndex > int(endSubnetIndex) {
			ipSubnet = ipSubnet + 1
			dutIPIndex = 1
			ateIPIndex = 2
		}
	}
}

// nextIP returns ip address based on hostIndex and subnetIndex provided.
func nextIP(ip net.IP, hostIndex int, subnetIndex int) net.IP {
	s := ip.String()
	sa := strings.Split(s, ".")
	sa[2] = strconv.Itoa(subnetIndex)
	sa[3] = strconv.Itoa(hostIndex)
	s = strings.Join(sa, ".")
	return net.ParseIP(s)
}

// BuildBenchmarkingConfig builds required configuration for DUT interfaces, ISIS and BGP.
func BuildBenchmarkingConfig(t *testing.T) *oc.Root {
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}

	// Generate ip addresses to configure DUT and ATE ports.
	buildPortIPs(dut)

	// Network instance and BGP configs.
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))

	bgp := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(DUTAs)
	global.RouterId = ygot.String(dutStartIPAddr)

	afi := global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	afi.Enabled = ygot.Bool(true)

	pg := bgp.GetOrCreatePeerGroup(PeerGrpName)
	pg.PeerAs = ygot.Uint32(ATEAs)
	pg.PeerGroupName = ygot.String(PeerGrpName)
	afipg := pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	afipg.Enabled = ygot.Bool(true)
	rp := d.GetOrCreateRoutingPolicy()
	pdef := rp.GetOrCreatePolicyDefinition(setALLOWPolicy)
	stmt, err := pdef.AppendNewStatement("id-1")
	if err != nil {
		t.Fatal(err)
	}
	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		rpl := pg.GetOrCreateApplyPolicy()
		rpl.SetExportPolicy([]string{setALLOWPolicy})
		rpl.SetImportPolicy([]string{setALLOWPolicy})
	} else {
		rpl := afipg.GetOrCreateApplyPolicy()
		rpl.SetExportPolicy([]string{setALLOWPolicy})
		rpl.SetImportPolicy([]string{setALLOWPolicy})
	}

	// ISIS configs.
	prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, ISISInstance)
	prot.Enabled = ygot.Bool(true)
	isis := prot.GetOrCreateIsis()

	globalISIS := isis.GetOrCreateGlobal()
	if deviations.ISISInstanceEnabledRequired(dut) {
		globalISIS.Instance = ygot.String(ISISInstance)
	}
	globalISIS.LevelCapability = oc.Isis_LevelType_LEVEL_2
	globalISIS.AuthenticationCheck = ygot.Bool(true)
	if deviations.ISISGlobalAuthenticationNotRequired(dut) {
		globalISIS.AuthenticationCheck = nil
	}
	globalISIS.Net = []string{fmt.Sprintf("%v.%v.00", dutAreaAddress, dutSysID)}
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	lspBit := globalISIS.GetOrCreateLspBit().GetOrCreateOverloadBit()
	lspBit.SetBit = ygot.Bool(false)
	isisTimers := globalISIS.GetOrCreateTimers()
	isisTimers.LspLifetimeInterval = ygot.Uint16(600)
	isisTimers.LspRefreshInterval = ygot.Uint16(250)
	spfTimers := isisTimers.GetOrCreateSpf()
	spfTimers.SpfHoldInterval = ygot.Uint64(5000)
	spfTimers.SpfFirstInterval = ygot.Uint64(600)

	isisLevel2 := isis.GetOrCreateLevel(2)
	isisLevel2.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC

	isisLevel2Auth := isisLevel2.GetOrCreateAuthentication()
	isisLevel2Auth.Enabled = ygot.Bool(true)
	if deviations.ISISExplicitLevelAuthenticationConfig(dut) {
		isisLevel2Auth.DisableCsnp = ygot.Bool(false)
		isisLevel2Auth.DisableLsp = ygot.Bool(false)
		isisLevel2Auth.DisablePsnp = ygot.Bool(false)
	}
	isisLevel2Auth.AuthPassword = ygot.String(authPassword)
	isisLevel2Auth.AuthMode = oc.IsisTypes_AUTH_MODE_MD5
	isisLevel2Auth.AuthType = oc.KeychainTypes_AUTH_TYPE_SIMPLE_KEY

	for _, dp := range dut.Ports() {
		// Interfaces config.
		i := d.GetOrCreateInterface(dp.Name())
		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
		i.Description = ygot.String("from oc")
		i.Name = ygot.String(dp.Name())

		s := i.GetOrCreateSubinterface(0)
		s4 := s.GetOrCreateIpv4()
		if deviations.InterfaceEnabled(dut) {
			s4.Enabled = ygot.Bool(true)
		}
		a4 := s4.GetOrCreateAddress(DUTIPList[dp.ID()].String())
		a4.PrefixLength = ygot.Uint8(plenIPv4)

		if deviations.ExplicitPortSpeed(dut) {
			i.GetOrCreateEthernet().PortSpeed = fptest.GetIfSpeed(t, dp)
		}

		// BGP neighbor configs.
		nv4 := bgp.GetOrCreateNeighbor(ATEIPList[dp.ID()].String())
		nv4.PeerGroup = ygot.String(PeerGrpName)
		if dp.ID() == "port1" {
			nv4.PeerAs = ygot.Uint32(ATEAs2)
		} else {
			nv4.PeerAs = ygot.Uint32(ATEAs)
		}
		nv4.Enabled = ygot.Bool(true)

		// ISIS configs.
		intfName := dp.Name()
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			intfName = dp.Name() + ".0"
		}
		isisIntf := isis.GetOrCreateInterface(intfName)
		isisIntf.Enabled = ygot.Bool(true)
		isisIntf.HelloPadding = oc.Isis_HelloPaddingType_ADAPTIVE
		isisIntf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
		isisIntfAfi := isisIntf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
		isisIntfAfi.Enabled = ygot.Bool(true)
		if deviations.ISISInterfaceAfiUnsupported(dut) {
			isisIntf.Af = nil
		}

		isisIntfLevel := isisIntf.GetOrCreateLevel(2)
		isisIntfLevel.Enabled = ygot.Bool(true)
		isisIntfLevelAuth := isisIntfLevel.GetOrCreateHelloAuthentication()
		isisIntfLevelAuth.Enabled = ygot.Bool(true)
		isisIntfLevelAuth.AuthPassword = ygot.String(authPassword)
		isisIntfLevelAuth.AuthMode = oc.IsisTypes_AUTH_MODE_MD5
		isisIntfLevelAuth.AuthType = oc.KeychainTypes_AUTH_TYPE_SIMPLE_KEY

		isisIntfLevelTimers := isisIntfLevel.GetOrCreateTimers()
		isisIntfLevelTimers.HelloInterval = ygot.Uint32(1)
		isisIntfLevelTimers.HelloMultiplier = ygot.Uint8(5)

		isisIntfLevelAfi := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
		isisIntfLevelAfi.Metric = ygot.Uint32(200)
		isisIntfLevelAfi.Enabled = ygot.Bool(true)

		// Configure ISIS AfiSafi enable flag at the global level
		if deviations.MissingIsisInterfaceAfiSafiEnable(dut) {
			isisIntfLevelAfi.Enabled = nil
		}
	}
	p := gnmi.OC()
	fptest.LogQuery(t, "DUT", p.Config(), d)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		for _, dp := range dut.Ports() {
			ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
			niIntf, _ := ni.NewInterface(dp.Name())
			niIntf.Interface = ygot.String(dp.Name())
			niIntf.Subinterface = ygot.Uint32(0)
			niIntf.Id = ygot.String(dp.Name() + ".0")
		}
	}
	return d
}

// ConfigureATE function is to configure ate ports with ipv4 , bgp
// and isis peers.
func ConfigureATE(t *testing.T, ate *ondatra.ATEDevice) {
	topo := ate.Topology().New()

	for _, dp := range ate.Ports() {
		ISISMetricList = append(ISISMetricList, ISISMetric)
		ISISSetBitList = append(ISISSetBitList, true)
		atePortAttr := attrs.Attributes{
			Name:    "ate" + dp.ID(),
			IPv4:    ATEIPList[dp.ID()].String(),
			IPv4Len: plenIPv4,
		}
		iDut1 := topo.AddInterface(dp.Name()).WithPort(dp)
		iDut1.IPv4().WithAddress(atePortAttr.IPv4CIDR()).WithDefaultGateway(DUTIPList[dp.ID()].String())

		// Add BGP routes and ISIS routes , ate port1 is ingress port.
		if dp.ID() == "port1" {
			// Add BGP on ATE
			bgpDut1 := iDut1.BGP()
			bgpDut1.AddPeer().WithPeerAddress(DUTIPList[dp.ID()].String()).WithLocalASN(ATEAs2).
				WithTypeExternal()

			// Add ISIS on ATE
			isisDut1 := iDut1.ISIS()
			isisDut1.WithLevelL2().WithNetworkTypePointToPoint().WithTERouterID(DUTIPList[dp.ID()].String()).WithAuthMD5(authPassword)

			netCIDR := fmt.Sprintf("%s/%d", AdvertiseBGPRoutesv4, 32)
			bgpNeti1 := iDut1.AddNetwork("bgpNeti1")
			bgpNeti1.IPv4().WithAddress(netCIDR).WithCount(RouteCount)
			bgpNeti1.BGP().WithNextHopAddress(atePortAttr.IPv4)

			netCIDR = fmt.Sprintf("%s/%d", advertiseISISRoutesv4, 32)
			isisnet1 := iDut1.AddNetwork("isisnet1")
			isisnet1.IPv4().WithAddress(netCIDR).WithCount(RouteCount)
			isisnet1.ISIS().WithActive(true).WithIPReachabilityMetric(20)

			continue
		}

		// Add BGP on ATE
		bgpDut1 := iDut1.BGP()
		bgpDut1.AddPeer().WithPeerAddress(DUTIPList[dp.ID()].String()).WithLocalASN(ATEAs).
			WithTypeExternal()

		// Add BGP on ATE
		isisDut1 := iDut1.ISIS()
		isisDut1.WithLevelL2().WithNetworkTypePointToPoint().WithTERouterID(DUTIPList[dp.ID()].String()).WithAuthMD5(authPassword)
	}

	t.Log("Pushing config to ATE...")
	topo.Push(t)
	t.Log("Starting protocols to ATE...")
	topo.StartProtocols(t)
}

// VerifyISISTelemetry function to used verify ISIS telemetry on DUT
// using OC isis telemetry path.
func VerifyISISTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, ISISInstance).Isis()
	for _, dp := range dut.Ports() {
		intfName := dp.Name()
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			intfName = dp.Name() + ".0"
		}
		nbrPath := statePath.Interface(intfName)
		query := nbrPath.LevelAny().AdjacencyAny().AdjacencyState().State()
		_, ok := gnmi.WatchAll(t, dut, query, time.Minute, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
			state, present := val.Val()
			return present && state == oc.Isis_IsisInterfaceAdjState_UP
		}).Await(t)
		if !ok {
			t.Logf("IS-IS state on %v has no adjacencies", intfName)
			t.Fatal("No IS-IS adjacencies reported.")
		}
	}
}

// VerifyBgpTelemetry function is to verify BGP telemetry on DUT using
// BGP OC telemetry path.
func VerifyBgpTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	for _, peerAddr := range ATEIPList {
		nbrIP := peerAddr.String()
		nbrPath := statePath.Neighbor(nbrIP)
		gnmi.Await(t, dut, nbrPath.SessionState().State(), time.Second*120, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
	}

}
