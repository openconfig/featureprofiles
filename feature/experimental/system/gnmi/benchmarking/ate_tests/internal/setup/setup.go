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
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

// Some of the variables defined below like DutAS, AteAS, PeerGrpName
// RouteCount and IsisInstance are used in other files which import
// setup.go
const (
	// DutAS can be exported
	DutAS = 64500
	// AteAs can be exported
	AteAS  = 64501
	ateAS2 = 64502
	// PeerGrpName can be exported
	PeerGrpName    = "BGP-PEER-GROUP"
	plenIPv4       = 30
	dutStartIPAddr = "192.0.2.1"
	ateStartIPAddr = "192.0.2.2"
	// RouteCount can be exported
	RouteCount            = 200
	advertiseBGPRoutesv4  = "203.0.113.1"
	authPassword          = "ISISAuthPassword"
	advertiseISISRoutesv4 = "198.18.0.0"
	IsisInstance          = "DEFAULT"
)

var (
	// DutIPPool can be exported
	DutIPPool = make(map[string]net.IP)
	// AteIPPool can be exported
	AteIPPool = make(map[string]net.IP)
)

// BuildIPPool is to Build pool of ip addresses for both DUT and ATE interfaces.
// It reads ports given in binding file to calculate ip addresses needed.
func BuildIPPool(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	var dutIPIndex, ipSubnet, ateIPIndex int = 1, 2, 2
	var endSubnetIndex = 253
	for _, dp := range dut.Ports() {
		dutNextIP := nextIP(net.ParseIP(dutStartIPAddr), dutIPIndex, ipSubnet)
		ateNextIP := nextIP(net.ParseIP(ateStartIPAddr), ateIPIndex, ipSubnet)
		DutIPPool[dp.ID()] = dutNextIP
		AteIPPool[dp.ID()] = ateNextIP

		// Increment dut and ate host ip index by 4
		dutIPIndex = dutIPIndex + 4
		ateIPIndex = ateIPIndex + 4

		// Reset dut and ate IP indexes when it is greater endSubnetIndex
		if dutIPIndex > int(endSubnetIndex) {
			ipSubnet = ipSubnet + 1
			dutIPIndex = 1
			ateIPIndex = 2
		}
	}

}

// nextIP returns ip address based on hostindex and subnetindex provided.
func nextIP(ip net.IP, hostIndex int, subnetIndex int) net.IP {
	s := ip.String()
	sa := strings.Split(s, ".")
	sa[2] = strconv.Itoa(subnetIndex)
	sa[3] = strconv.Itoa(hostIndex)
	s = strings.Join(sa, ".")
	return net.ParseIP(s)
}

// BuildOCUpdate function is to build  OC config for interfaces.
// It reads ports from binding file and returns gpb update message
// which will have configurations for all the ports.
func BuildOCUpdate(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &telemetry.Device{}

	// Default Network instance and Global BGP configs
	ni1 := d.GetOrCreateNetworkInstance(*deviations.DefaultNetworkInstance)

	bgp := ni1.GetOrCreateProtocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(DutAS)
	global.RouterId = ygot.String(dutStartIPAddr)

	afi := global.GetOrCreateAfiSafi(telemetry.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	afi.Enabled = ygot.Bool(true)

	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	pg := bgp.GetOrCreatePeerGroup(PeerGrpName)
	pg.PeerAs = ygot.Uint32(AteAS)
	pg.PeerGroupName = ygot.String(PeerGrpName)

	// ISIS Configs
	isis := ni1.GetOrCreateProtocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, IsisInstance).GetOrCreateIsis()

	globalISIS := isis.GetOrCreateGlobal()
	globalISIS.AuthenticationCheck = ygot.Bool(true)
	lspBit := globalISIS.GetOrCreateLspBit().GetOrCreateOverloadBit()
	lspBit.SetBit = ygot.Bool(false)
	isisTimers := globalISIS.GetOrCreateTimers()
	isisTimers.LspLifetimeInterval = ygot.Uint16(600)
	spfTimers := isisTimers.GetOrCreateSpf()
	spfTimers.SpfHoldInterval = ygot.Uint64(5000)
	spfTimers.SpfFirstInterval = ygot.Uint64(600)

	isisLevel1 := isis.GetOrCreateLevel(1)
	isisLevel1.Enabled = ygot.Bool(false)

	isisLevel2 := isis.GetOrCreateLevel(2)
	isisLevel2.Enabled = ygot.Bool(true)
	isisLevel2.MetricStyle = telemetry.IsisTypes_MetricStyle_WIDE_METRIC

	isisLevel2Auth := isisLevel2.GetOrCreateAuthentication()
	isisLevel2Auth.Enabled = ygot.Bool(true)
	isisLevel2Auth.AuthPassword = ygot.String(authPassword)
	isisLevel2Auth.AuthMode = telemetry.IsisTypes_AUTH_MODE_MD5
	isisLevel2Auth.AuthType = telemetry.KeychainTypes_AUTH_TYPE_SIMPLE_KEY

	for _, dp := range dut.Ports() {
		// Interface config
		i := d.GetOrCreateInterface(dp.Name())
		i.Type = telemetry.IETFInterfaces_InterfaceType_ethernetCsmacd
		if *deviations.InterfaceEnabled {
			i.Enabled = ygot.Bool(true)
		}
		i.Description = ygot.String("from oc")
		i.Name = ygot.String(dp.Name())

		s := i.GetOrCreateSubinterface(0)
		s4 := s.GetOrCreateIpv4()
		if *deviations.InterfaceEnabled {
			s4.Enabled = ygot.Bool(true)
		}
		a4 := s4.GetOrCreateAddress(DutIPPool[dp.ID()].String())
		a4.PrefixLength = ygot.Uint8(plenIPv4)

		// BGP Neighbor related configs.
		nv4 := bgp.GetOrCreateNeighbor(AteIPPool[dp.ID()].String())
		nv4.PeerGroup = ygot.String(PeerGrpName)
		if dp.ID() == "port1" {
			nv4.PeerAs = ygot.Uint32(ateAS2)
		} else {
			nv4.PeerAs = ygot.Uint32(AteAS)
		}
		nv4.Enabled = ygot.Bool(true)

		// ISIS configs
		isisIntf := isis.GetOrCreateInterface(dp.Name())
		isisIntf.Enabled = ygot.Bool(true)
		isisIntf.HelloPadding = telemetry.IsisTypes_HelloPaddingType_ADAPTIVE
		isisIntf.CircuitType = telemetry.IsisTypes_CircuitType_POINT_TO_POINT

		isisIntfAuth := isisIntf.GetOrCreateAuthentication()
		isisIntfAuth.Enabled = ygot.Bool(true)
		isisIntfAuth.AuthPassword = ygot.String(authPassword)
		isisIntfAuth.AuthMode = telemetry.IsisTypes_AUTH_MODE_MD5
		isisIntfAuth.AuthType = telemetry.KeychainTypes_AUTH_TYPE_SIMPLE_KEY

		isisIntfLevel := isisIntf.GetOrCreateLevel(2)
		isisIntfLevel.Enabled = ygot.Bool(true)

		isisIntfLevelTimers := isisIntfLevel.GetOrCreateTimers()
		isisIntfLevelTimers.HelloInterval = ygot.Uint32(1)
		isisIntfLevelTimers.HelloMultiplier = ygot.Uint8(5)

		isisIntfLevelAfi := isisIntfLevel.GetOrCreateAf(telemetry.IsisTypes_AFI_TYPE_IPV4, telemetry.IsisTypes_SAFI_TYPE_UNICAST)
		isisIntfLevelAfi.Metric = ygot.Uint32(200)
		isisIntfLevelAfi.Enabled = ygot.Bool(true)
	}
	p := dut.Config()
	fptest.LogYgot(t, fmt.Sprintf("%s to Update()", dut), p, d)
	p.Update(t, d)
}

// ConfigureATE function is to configure ate ports with ipv4 , bgp
// and isis peers.
func ConfigureATE(t *testing.T, ate *ondatra.ATEDevice) {
	topo := ate.Topology().New()

	for _, dp := range ate.Ports() {
		atePortAttr := attrs.Attributes{
			Name:    "ate" + dp.ID(),
			IPv4:    AteIPPool[dp.ID()].String(),
			IPv4Len: plenIPv4,
		}
		iDut1 := topo.AddInterface(dp.Name()).WithPort(dp)
		iDut1.IPv4().WithAddress(atePortAttr.IPv4CIDR()).WithDefaultGateway(DutIPPool[dp.ID()].String())

		// Add BGP routes and ISIS routes , ate port1 is ingress port.
		if dp.ID() == "port1" {
			//port1 is ingress port.
			// Add BGP on ATE
			bgpDut1 := iDut1.BGP()
			bgpDut1.AddPeer().WithPeerAddress(DutIPPool[dp.ID()].String()).WithLocalASN(ateAS2).
				WithTypeExternal()

			// Add BGP on ATE
			isisDut1 := iDut1.ISIS()
			isisDut1.WithLevelL2().WithNetworkTypePointToPoint().WithTERouterID(DutIPPool[dp.ID()].String()).WithAuthMD5(authPassword)

			netCIDR := fmt.Sprintf("%s/%d", advertiseBGPRoutesv4, 32)
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
		bgpDut1.AddPeer().WithPeerAddress(DutIPPool[dp.ID()].String()).WithLocalASN(AteAS).
			WithTypeExternal()

		// Add BGP on ATE
		isisDut1 := iDut1.ISIS()
		isisDut1.WithLevelL2().WithNetworkTypePointToPoint().WithTERouterID(DutIPPool[dp.ID()].String()).WithAuthMD5(authPassword)

	}

	t.Logf("Pushing config to ATE and starting protocols...")
	topo.Push(t)
	topo.StartProtocols(t)
}

// VerifyISISTelemetry function to used verify ISIS telemetry on DUT
// using OC isis telemetry path.
func VerifyISISTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	statePath := dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, IsisInstance).Isis()
	for _, dp := range dut.Ports() {
		nbrPath := statePath.Interface(dp.Name())
		_, ok := nbrPath.LevelAny().AdjacencyAny().AdjacencyState().Watch(t, time.Minute,
			func(val *telemetry.QualifiedE_IsisTypes_IsisInterfaceAdjState) bool {
				return val.IsPresent() && val.Val(t) == telemetry.IsisTypes_IsisInterfaceAdjState_UP
			}).Await(t)
		if !ok {
			fptest.LogYgot(t, fmt.Sprintf("IS-IS state on %v has no adjacencies", dp.Name()), nbrPath, nbrPath.Get(t))
			t.Fatal("No IS-IS adjacencies reported.")
		}
	}
}

// VerifyBgpTelemetry function to verify BGP telemetry on DUT using
// BGP OC telemetry path.
func VerifyBgpTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	statePath := dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	for _, peerAddr := range AteIPPool {
		nbrIP := peerAddr.String()
		nbrPath := statePath.Neighbor(nbrIP)

		// Get BGP adjacency state
		_, ok := nbrPath.SessionState().Watch(t, time.Minute, func(val *telemetry.QualifiedE_Bgp_Neighbor_SessionState) bool {
			return val.IsPresent() && val.Val(t) == telemetry.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			fptest.LogYgot(t, "BGP reported state", nbrPath, nbrPath.Get(t))
			t.Fatal("No BGP neighbor formed")
		}
		status := nbrPath.SessionState().Get(t)
		if want := telemetry.Bgp_Neighbor_SessionState_ESTABLISHED; status != want {
			t.Errorf("BGP peer %s status got %d, want %d", nbrIP, status, want)
		}
	}
}
