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

package actions_med_localpref_prepend_flow_control_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/netinstbgp"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	otg "github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	advertisedRoutesv4Net1      = "192.168.10.0"
	advertisedRoutesv6Net1      = "2024:db8:128:128::"
	advertisedRoutesv4Net2      = "192.168.20.0"
	advertisedRoutesv6Net2      = "2024:db8:64:64::"
	advertisedRoutesv4PrefixLen = 24
	advertisedRoutesv6PrefixLen = 64
	dutAS                       = 64500
	ateAS                       = 64501
	plenIPv4                    = 30
	plenIPv6                    = 126
	setLocalPrefPolicy          = "lp-policy"
	initialLocalPrefValue       = 50
	initialMEDValue             = 50
	setMEDPolicy                = "med-policy"
	matchStatement1             = "match-statement-1"
	setPrependPolicy            = "prepend-policy"
	testASN                     = 23456
	defRejectRoute              = oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE
	defAcceptRoute              = oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE
	peerGrpName1v4              = "iBGP-PEER-GROUP1-V4"
	peerGrpName1v6              = "iBGP-PEER-GROUP1-V6"
	peerGrpName2v4              = "eBGP-PEER-GROUP2-V4"
	peerGrpName2v6              = "eBGP-PEER-GROUP2-V6"
	nxtMED                      = 70
	nxtLocalPref                = 70
	setNxtPolicy                = "flow-control-policy"
	matchStatement2             = "match-statement-2"
	asnRepeatN                  = 10
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

type bgpNghbrs struct {
	localAs, peerAs     uint32
	localIP             string
	peerIP, peerGrpName []string
}

// createNewBgpSession configures BGP on DUT with neighbors pointing to ateSrc and ateDst and
// a peer group policy.
func createNewBgpSession(dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	nb1 := &bgpNghbrs{localAs: dutAS, peerAs: dutAS, localIP: dutPort1.IPv4, peerIP: []string{atePort1.IPv4, atePort1.IPv6}, peerGrpName: []string{peerGrpName1v4, peerGrpName1v6}}
	nb2 := &bgpNghbrs{localAs: dutAS, peerAs: ateAS, localIP: dutPort2.IPv4, peerIP: []string{atePort2.IPv4, atePort2.IPv6}, peerGrpName: []string{peerGrpName2v4, peerGrpName2v6}}
	nbs := []*bgpNghbrs{nb1, nb2}
	dutOcRoot := &oc.Root{}
	ni1 := dutOcRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	for _, nb := range nbs {
		routerID := nb.localIP
		peerV4 := nb.peerIP[0]
		peerV6 := nb.peerIP[1]
		peerGrpNameV4 := nb.peerGrpName[0]
		peerGrpNameV6 := nb.peerGrpName[1]
		global.RouterId = ygot.String(routerID)
		global.As = ygot.Uint32(nb.localAs)
		global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
		global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
		// Note: we have to define the peer group even if we aren't setting any policy because it's
		// invalid OC for the neighbor to be part of a peer group that doesn't exist.
		pg := bgp.GetOrCreatePeerGroup(peerGrpNameV4)
		pg.PeerAs = ygot.Uint32(nb.peerAs)
		pg.PeerGroupName = ygot.String(peerGrpNameV4)

		bgpNbr := bgp.GetOrCreateNeighbor(peerV4)
		bgpNbr.PeerGroup = ygot.String(peerGrpNameV4)
		bgpNbr.PeerAs = ygot.Uint32(nb.peerAs)
		bgpNbr.Enabled = ygot.Bool(true)
		af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		af4.Enabled = ygot.Bool(true)

		pg1 := bgp.GetOrCreatePeerGroup(peerGrpNameV6)
		pg1.PeerAs = ygot.Uint32(nb.peerAs)
		pg1.PeerGroupName = ygot.String(peerGrpNameV6)

		bgpNbr1 := bgp.GetOrCreateNeighbor(peerV6)
		bgpNbr1.PeerGroup = ygot.String(peerGrpNameV6)
		bgpNbr1.PeerAs = ygot.Uint32(nb.peerAs)
		bgpNbr1.Enabled = ygot.Bool(true)
		af6 := bgpNbr1.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		af6.Enabled = ygot.Bool(true)
	}
	return niProto
}

// VerifyBgpState verifies that BGP is established
func VerifyBgpState(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	var nbrIP = []string{atePort1.IPv4, atePort1.IPv6, atePort2.IPv4, atePort2.IPv6}
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	watch := gnmi.Watch(t, dut, bgpPath.State(), 2*time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Protocol_Bgp]) bool {
		path, _ := val.Val()
		for _, nbr := range nbrIP {
			if path.GetNeighbor(nbr).GetSessionState() != oc.Bgp_Neighbor_SessionState_ESTABLISHED {
				return false
			}
		}
		return true
	})
	if val, ok := watch.Await(t); !ok {
		t.Fatalf("BGP sessions not established: got %v", val)
	}
	t.Log("BGP sessions Established")
}

// configureASLocalPrefMEDPolicy configures MED, Local Pref, AS prepend etc
func configureASLocalPrefMEDPolicy(t *testing.T, dut *ondatra.DUTDevice, policyType, policyValue, statement string, ASN uint32) {
	t.Helper()
	dutOcRoot := &oc.Root{}
	batchConfig := &gnmi.SetBatch{}
	rp := dutOcRoot.GetOrCreateRoutingPolicy()
	pdef := rp.GetOrCreatePolicyDefinition(policyType)
	stmt, err := pdef.AppendNewStatement(statement)
	if err != nil {
		t.Fatal(err)
	}
	actions := stmt.GetOrCreateActions()
	switch policyType {
	case setLocalPrefPolicy:
		metric, _ := strconv.Atoi(policyValue)
		actions.GetOrCreateBgpActions().SetLocalPref = ygot.Uint32(uint32(metric))
		actions.PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	case setMEDPolicy:
		if strings.Contains(policyValue, "+") {
			actions.GetOrCreateBgpActions().SetMed = oc.UnionString(policyValue)
		} else {
			metric, _ := strconv.Atoi(policyValue)
			actions.GetOrCreateBgpActions().SetMed = oc.UnionUint32(uint32(metric))
		}
		actions.PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	case setPrependPolicy:
		metric, _ := strconv.Atoi(policyValue)
		asPrepend := actions.GetOrCreateBgpActions().GetOrCreateSetAsPathPrepend()
		asPrepend.Asn = ygot.Uint32(ASN)
		asPrepend.RepeatN = ygot.Uint8(uint8(metric))
		actions.PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	case setNxtPolicy:
		if !deviations.SkipSettingStatementForPolicy(dut) {
			actions.PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
		}
		metric, _ := strconv.Atoi(policyValue)
		actions.GetOrCreateBgpActions().SetMed = oc.UnionUint32(uint32(metric))

		stmt2, err := pdef.AppendNewStatement(matchStatement2)
		if err != nil {
			t.Fatal(err)
		}
		actions2 := stmt2.GetOrCreateActions()
		asPrepend := actions2.GetOrCreateBgpActions().GetOrCreateSetAsPathPrepend()
		asPrepend.Asn = ygot.Uint32(ASN)
		asPrepend.RepeatN = ygot.Uint8(asnRepeatN)
		actions2.PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	default:
		rp = nil
	}
	gnmi.BatchReplace(batchConfig, gnmi.OC().RoutingPolicy().Config(), rp)
	batchConfig.Set(t, dut)
}

// configureBGPDefaultImportExportPolicy configures default import/export policies
func configureBGPDefaultImportExportPolicy(t *testing.T, dut *ondatra.DUTDevice, ipv4, ipv6 string, polType oc.E_RoutingPolicy_DefaultPolicyType) {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	batchConfig := &gnmi.SetBatch{}
	nbrPolPathv4 := bgpPath.Neighbor(ipv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()
	nbrPolPathv6 := bgpPath.Neighbor(ipv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy()
	gnmi.BatchReplace(batchConfig, nbrPolPathv4.DefaultImportPolicy().Config(), polType)
	gnmi.BatchReplace(batchConfig, nbrPolPathv4.DefaultExportPolicy().Config(), polType)
	gnmi.BatchReplace(batchConfig, nbrPolPathv6.DefaultImportPolicy().Config(), polType)
	gnmi.BatchReplace(batchConfig, nbrPolPathv6.DefaultExportPolicy().Config(), polType)
	batchConfig.Set(t, dut)
}

// configureBGPImportExportPolicy configures import/export policies
func configureBGPImportExportPolicy(t *testing.T, dut *ondatra.DUTDevice, ipv4, ipv6, policyDef string) {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	batchConfig := &gnmi.SetBatch{}
	nbrPolPathv4 := bgpPath.Neighbor(ipv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()
	nbrPolPathv6 := bgpPath.Neighbor(ipv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy()
	gnmi.BatchReplace(batchConfig, nbrPolPathv4.ImportPolicy().Config(), []string{policyDef})
	gnmi.BatchReplace(batchConfig, nbrPolPathv4.ExportPolicy().Config(), []string{policyDef})
	gnmi.BatchReplace(batchConfig, nbrPolPathv6.ImportPolicy().Config(), []string{policyDef})
	gnmi.BatchReplace(batchConfig, nbrPolPathv6.ExportPolicy().Config(), []string{policyDef})
	batchConfig.Set(t, dut)
}

// deleteBGPImportExportPolicy configures import/export policies
func deleteBGPImportExportPolicy(t *testing.T, dut *ondatra.DUTDevice, ipv4, ipv6 string, ipv4_2 string, ipv6_2 string) {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	batchConfig := &gnmi.SetBatch{}
	nbrPolPathv4 := bgpPath.Neighbor(ipv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()
	nbrPolPathv6 := bgpPath.Neighbor(ipv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy()
	if deviations.DefaultRoutePolicyUnsupported(dut) {
		// deleteBGPImportExportPolicy on port2 needed when default policy is not supported
		nbrPolPathv4_2 := bgpPath.Neighbor(ipv4_2).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()
		nbrPolPathv6_2 := bgpPath.Neighbor(ipv6_2).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy()
		gnmi.BatchDelete(batchConfig, nbrPolPathv4_2.ImportPolicy().Config())
		gnmi.BatchDelete(batchConfig, nbrPolPathv4_2.ExportPolicy().Config())
		gnmi.BatchDelete(batchConfig, nbrPolPathv6_2.ImportPolicy().Config())
		gnmi.BatchDelete(batchConfig, nbrPolPathv6_2.ExportPolicy().Config())
	}
	gnmi.BatchDelete(batchConfig, nbrPolPathv4.ImportPolicy().Config())
	gnmi.BatchDelete(batchConfig, nbrPolPathv4.ExportPolicy().Config())
	gnmi.BatchDelete(batchConfig, nbrPolPathv6.ImportPolicy().Config())
	gnmi.BatchDelete(batchConfig, nbrPolPathv6.ExportPolicy().Config())
	batchConfig.Set(t, dut)
}

// verifyBgpPolicyTelemetry verifies that the BGP policy telemetry matches
func verifyBgpPolicyTelemetry(t *testing.T, dut *ondatra.DUTDevice, ipAddr string, defPol oc.E_RoutingPolicy_DefaultPolicyType, appliedPol string, isV4 bool) {
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

	if !deviations.DefaultRoutePolicyUnsupported(dut) {
		if gotDefExPolicy := peerTel.GetApplyPolicy().GetDefaultExportPolicy(); gotDefExPolicy != defPol {
			t.Errorf("Default export policy type mismatch: got %v, want %v", gotDefExPolicy, defPol)
		}

		if gotDefImPolicy := peerTel.GetApplyPolicy().GetDefaultImportPolicy(); gotDefImPolicy != defPol {
			t.Errorf("Default import policy type mismatch: got %v, want %v", gotDefImPolicy, defPol)
		}
	}

	if appliedPol != "" {
		if gotExportPol := peerTel.GetApplyPolicy().GetExportPolicy(); cmp.Diff(gotExportPol, []string{appliedPol}) != "" {
			t.Errorf("Export policy type mismatch: got %v, want %v", gotExportPol, []string{appliedPol})
		}
	} else {
		if gotExportPol := peerTel.GetApplyPolicy().GetExportPolicy(); gotExportPol != nil {
			t.Errorf("Export policy type mismatch: got %v, want %v", gotExportPol, "nil")
		}
	}

	if appliedPol != "" {
		if gotImportPol := peerTel.GetApplyPolicy().GetImportPolicy(); cmp.Diff(gotImportPol, []string{appliedPol}) != "" {
			t.Errorf("Import policy type mismatch: got %v, want %v", gotImportPol, []string{appliedPol})
		}
	} else {
		if gotImportPol := peerTel.GetApplyPolicy().GetImportPolicy(); gotImportPol != nil {
			t.Errorf("Import policy type mismatch: got %v, want %v", gotImportPol, "nil")
		}
	}
}

// configureOTG configures the interfaces and BGP protocols on an OTG, including advertising some
// networks over BGP.
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
	iDut1Bgp4Peer.SetPeerAddress(iDut1Ipv4.Gateway()).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	iDut1Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
	iDut1Bgp6Peer := iDut1Bgp.Ipv6Interfaces().Add().SetIpv6Name(iDut1Ipv6.Name()).Peers().Add().SetName(atePort1.Name + ".BGP6.peer")
	iDut1Bgp6Peer.SetPeerAddress(iDut1Ipv6.Gateway()).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
	iDut1Bgp6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	// eBGP v4 and v6 sessions on port2
	iDut2Bgp := iDut2Dev.Bgp().SetRouterId(iDut2Ipv4.Address())
	iDut2Bgp4Peer := iDut2Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut2Ipv4.Name()).Peers().Add().SetName(atePort2.Name + ".BGP4.peer")
	iDut2Bgp4Peer.SetPeerAddress(iDut2Ipv4.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	iDut2Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
	iDut2Bgp6Peer := iDut2Bgp.Ipv6Interfaces().Add().SetIpv6Name(iDut2Ipv6.Name()).Peers().Add().SetName(atePort2.Name + ".BGP6.peer")
	iDut2Bgp6Peer.SetPeerAddress(iDut2Ipv6.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	iDut2Bgp6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	// iBGP V4 routes from Port1 and set MED, Local Preference.
	bgpNeti1Bgp4PeerRoutes := iDut1Bgp4Peer.V4Routes().Add().SetName(atePort1.Name + ".BGP4.Route")
	bgpNeti1Bgp4PeerRoutes.SetNextHopIpv4Address(iDut1Ipv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	bgpNeti1Bgp4PeerRoutes.Addresses().Add().
		SetAddress(advertisedRoutesv4Net1).SetPrefix(advertisedRoutesv4PrefixLen)
	bgpNeti1Bgp4PeerRoutes.Advanced().SetIncludeMultiExitDiscriminator(true).SetMultiExitDiscriminator(50)
	bgpNeti1Bgp4PeerRoutes.Advanced().SetIncludeLocalPreference(true).SetLocalPreference(50)

	// iBGP V6 routes from Port1 and set MED, Local Preference.
	bgpNeti1Bgp6PeerRoutes := iDut1Bgp6Peer.V6Routes().Add().SetName(atePort1.Name + ".BGP6.Route")
	bgpNeti1Bgp6PeerRoutes.SetNextHopIpv6Address(iDut1Ipv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	bgpNeti1Bgp6PeerRoutes.Addresses().Add().
		SetAddress(advertisedRoutesv6Net1).SetPrefix(advertisedRoutesv6PrefixLen)
	bgpNeti1Bgp6PeerRoutes.Advanced().SetIncludeMultiExitDiscriminator(true).SetMultiExitDiscriminator(50)
	bgpNeti1Bgp6PeerRoutes.Advanced().SetIncludeLocalPreference(true).SetLocalPreference(50)

	// eBGP V4 routes from Port2 and set MED, Local Preference.
	bgpNeti2Bgp4PeerRoutes := iDut2Bgp4Peer.V4Routes().Add().SetName(atePort2.Name + ".BGP4.Route")
	bgpNeti2Bgp4PeerRoutes.SetNextHopIpv4Address(iDut2Ipv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	bgpNeti2Bgp4PeerRoutes.Addresses().Add().
		SetAddress(advertisedRoutesv4Net2).SetPrefix(advertisedRoutesv4PrefixLen)
	bgpNeti2Bgp4PeerRoutes.Advanced().SetIncludeMultiExitDiscriminator(true).SetMultiExitDiscriminator(50)
	bgpNeti2Bgp4PeerRoutes.Advanced().SetIncludeLocalPreference(true).SetLocalPreference(50)

	// eBGP V6 routes from Port2 and set MED, Local Preference.
	bgpNeti2Bgp6PeerRoutes := iDut2Bgp6Peer.V6Routes().Add().SetName(atePort2.Name + ".BGP6.Route")
	bgpNeti2Bgp6PeerRoutes.SetNextHopIpv6Address(iDut2Ipv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	bgpNeti2Bgp6PeerRoutes.Addresses().Add().
		SetAddress(advertisedRoutesv6Net2).SetPrefix(advertisedRoutesv6PrefixLen)
	bgpNeti2Bgp6PeerRoutes.Advanced().SetIncludeMultiExitDiscriminator(true).SetMultiExitDiscriminator(50)
	bgpNeti2Bgp6PeerRoutes.Advanced().SetIncludeLocalPreference(true).SetLocalPreference(50)

	t.Logf("Pushing config to ATE and starting protocols...")
	otg.PushConfig(t, config)
	otg.StartProtocols(t)
	return config
}

// validateOTGBgpPrefixV4AndASLocalPrefMED verifies that the IPv4 prefix is received on OTG.
func validateOTGBgpPrefixV4AndASLocalPrefMED(t *testing.T, otg *otg.OTG, dut *ondatra.DUTDevice, config gosnappi.Config, peerName, ipAddr string, prefixLen uint32, pathAttr string, metric uint32) {
	// t.Helper()
	_, ok := gnmi.WatchAll(t,
		otg,
		gnmi.OTG().BgpPeer(peerName).UnicastIpv4PrefixAny().State(),
		30*time.Second,
		func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv4Prefix]) bool {
			_, present := v.Val()
			return present
		}).Await(t)
	var foundPrefix = false
	if ok {
		bgpPrefixes := gnmi.GetAll[*otgtelemetry.BgpPeer_UnicastIpv4Prefix](t, otg, gnmi.OTG().BgpPeer(peerName).UnicastIpv4PrefixAny().State())
		for _, bgpPrefix := range bgpPrefixes {
			if bgpPrefix.Address != nil && bgpPrefix.GetAddress() == ipAddr &&
				bgpPrefix.PrefixLength != nil && bgpPrefix.GetPrefixLength() == prefixLen {
				foundPrefix = true
				t.Logf("Prefix recevied on OTG is correct, got prefix %v, want prefix %v", bgpPrefix, ipAddr)
				switch pathAttr {
				case setMEDPolicy:
					if bgpPrefix.GetMultiExitDiscriminator() != metric {
						t.Errorf("For Prefix %v, got MED %d want MED %d", bgpPrefix.GetAddress(), bgpPrefix.GetMultiExitDiscriminator(), metric)
					} else {
						t.Logf("For Prefix %v, got MED %d want MED %d", bgpPrefix.GetAddress(), bgpPrefix.GetMultiExitDiscriminator(), metric)
					}
				case setLocalPrefPolicy:
					validateImportRoutingPolicy(t, dut, ipAddr, metric)
				case setPrependPolicy:
					if len(bgpPrefix.AsPath[0].GetAsNumbers()) != int(metric) {
						t.Errorf("For Prefix %v, got AS Path Prepend %d want AS Path Prepend %d", bgpPrefix.GetAddress(), len(bgpPrefix.AsPath[0].GetAsNumbers()), int(metric))
					} else {
						t.Logf("For Prefix %v, got AS Path Prepend %d want AS Path Prepend %d", bgpPrefix.GetAddress(), len(bgpPrefix.AsPath), int(metric))
					}
				case setNxtPolicy:
					if bgpPrefix.GetMultiExitDiscriminator() != metric {
						t.Errorf("For Prefix %v, got MED %d want MED %d", bgpPrefix.GetAddress(), bgpPrefix.GetMultiExitDiscriminator(), metric)
					} else {
						t.Logf("For Prefix %v, got MED %d want MED %d", bgpPrefix.GetAddress(), bgpPrefix.GetMultiExitDiscriminator(), metric)
					}
					if len(bgpPrefix.AsPath[0].GetAsNumbers()) != asnRepeatN+1 {
						t.Errorf("For Prefix %v, got AS Path Prepend %d want AS Path Prepend %d", bgpPrefix.GetAddress(), len(bgpPrefix.AsPath[0].GetAsNumbers()), asnRepeatN+1)
					} else {
						t.Logf("For Prefix %v, got AS Path Prepend %d want AS Path Prepend %d", bgpPrefix.GetAddress(), len(bgpPrefix.AsPath), asnRepeatN+1)
					}
				default:
					t.Errorf("Incorrect BGP Path Attribute. Expected MED, Local Pref or AS Path Prepend!!!!")
				}
				break
			}
		}
	}
	if !foundPrefix {
		t.Errorf("Prefix %v not received on OTG", ipAddr)
	}
}

// validateOTGBgpPrefixV6AndASLocalPrefMED verifies that the IPv6 prefix is received on OTG.
func validateOTGBgpPrefixV6AndASLocalPrefMED(t *testing.T, otg *otg.OTG, dut *ondatra.DUTDevice, config gosnappi.Config, peerName, ipAddr string, prefixLen uint32, pathAttr string, metric uint32) {
	// t.Helper()
	_, ok := gnmi.WatchAll(t,
		otg,
		gnmi.OTG().BgpPeer(peerName).UnicastIpv6PrefixAny().State(),
		30*time.Second,
		func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv6Prefix]) bool {
			_, present := v.Val()
			return present
		}).Await(t)
	var foundPrefix = false
	if ok {
		bgpPrefixes := gnmi.GetAll[*otgtelemetry.BgpPeer_UnicastIpv6Prefix](t, otg, gnmi.OTG().BgpPeer(peerName).UnicastIpv6PrefixAny().State())
		for _, bgpPrefix := range bgpPrefixes {
			if bgpPrefix.Address != nil && bgpPrefix.GetAddress() == ipAddr &&
				bgpPrefix.PrefixLength != nil && bgpPrefix.GetPrefixLength() == prefixLen {
				foundPrefix = true
				t.Logf("Prefix recevied on OTG is correct, got prefix %v, want prefix %v", bgpPrefix, ipAddr)
				switch pathAttr {
				case setMEDPolicy:
					if bgpPrefix.GetMultiExitDiscriminator() != metric {
						t.Errorf("For Prefix %v, got MED %d want MED %d", bgpPrefix.GetAddress(), bgpPrefix.GetMultiExitDiscriminator(), metric)
					} else {
						t.Logf("For Prefix %v, got MED %d want MED %d", bgpPrefix.GetAddress(), bgpPrefix.GetMultiExitDiscriminator(), metric)
					}
				case setLocalPrefPolicy:
					validateImportRoutingPolicyV6(t, dut, ipAddr, metric)
				case setPrependPolicy:
					if len(bgpPrefix.AsPath[0].GetAsNumbers()) != int(metric) {
						t.Errorf("For Prefix %v, got AS Path Prepend %d want AS Path Prepend %d", bgpPrefix.GetAddress(), len(bgpPrefix.AsPath[0].GetAsNumbers()), int(metric))
					} else {
						t.Logf("For Prefix %v, got AS Path Prepend %d want AS Path Prepend %d", bgpPrefix.GetAddress(), len(bgpPrefix.AsPath), int(metric))
					}
				case setNxtPolicy:
					if bgpPrefix.GetMultiExitDiscriminator() != metric {
						t.Errorf("For Prefix %v, got MED %d want MED %d", bgpPrefix.GetAddress(), bgpPrefix.GetMultiExitDiscriminator(), metric)
					} else {
						t.Logf("For Prefix %v, got MED %d want MED %d", bgpPrefix.GetAddress(), bgpPrefix.GetMultiExitDiscriminator(), metric)
					}
					if len(bgpPrefix.AsPath[0].GetAsNumbers()) != asnRepeatN+1 {
						t.Errorf("For Prefix %v, got AS Path Prepend %d want AS Path Prepend %d", bgpPrefix.GetAddress(), len(bgpPrefix.AsPath[0].GetAsNumbers()), asnRepeatN+1)
					} else {
						t.Logf("For Prefix %v, got AS Path Prepend %d want AS Path Prepend %d", bgpPrefix.GetAddress(), len(bgpPrefix.AsPath), asnRepeatN+1)
					}
				default:
					t.Errorf("Incorrect Routing Policy. Expected MED, Local Pref or AS Path Prepend!!!!")
				}
				break
			}
		}
	}
	if !foundPrefix {
		t.Errorf("Prefix %v not received on OTG", ipAddr)
	}
}

func TestBGPPolicy(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	otg := ate.OTG()
	var otgConfig gosnappi.Config
	t.Run("Configure OTG", func(t *testing.T) {
		otgConfig = configureOTG(t, otg)
	})

	// DUT configurations.
	t.Run("Configure DUT interfaces", func(t *testing.T) {
		configureDUT(t, dut)
	})

	t.Run("Configure DEFAULT network instance", func(t *testing.T) {
		fptest.ConfigureDefaultNetworkInstance(t, dut)
	})

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")

	t.Run("Configure BGP v4 and v6 Neighbors", func(t *testing.T) {
		gnmi.Delete(t, dut, dutConfPath.Config())
		dutConf := createNewBgpSession(dut)
		gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
		fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))
	})

	t.Run("Verify port status on DUT", func(t *testing.T) {
		verifyPortsUp(t, dut.Device)
	})

	t.Run("Verify BGP session", func(t *testing.T) {
		VerifyBgpState(t, dut)
	})

	cases := []struct {
		desc                                                        string
		rpPolicy, policyTypePort1, policyTypePort2, policyStatement string
		defPolicyPort1, defPolicyPort2                              oc.E_RoutingPolicy_DefaultPolicyType
		policyValue                                                 string
		port1v4Prefix, port1v6Prefix, port2v4Prefix, port2v6Prefix  string
		isDeletePolicy                                              bool
		metricValue, asn                                            uint32
		deleteNbrv4, deleteNbrv6                                    string
		polNbrv4, polNbrv6                                          string
	}{{
		desc:            "Configure eBGP set MED Import Export Policy",
		rpPolicy:        setMEDPolicy,
		policyTypePort1: "",
		policyValue:     "100",
		policyStatement: matchStatement1,
		defPolicyPort1:  defAcceptRoute,
		defPolicyPort2:  defRejectRoute,
		policyTypePort2: setMEDPolicy,
		port1v4Prefix:   advertisedRoutesv4Net2,
		port1v6Prefix:   advertisedRoutesv6Net2,
		port2v4Prefix:   advertisedRoutesv4Net1,
		port2v6Prefix:   advertisedRoutesv6Net1,
		metricValue:     100,
		polNbrv4:        atePort2.IPv4,
		polNbrv6:        atePort2.IPv6,
		isDeletePolicy:  true,
		deleteNbrv4:     atePort1.IPv4,
		deleteNbrv6:     atePort1.IPv6,
		asn:             dutAS,
	}, {
		desc:            "Configure eBGP increase MED Import Export Policy",
		rpPolicy:        setMEDPolicy,
		policyTypePort1: "",
		policyValue:     "+100",
		policyStatement: matchStatement1,
		defPolicyPort1:  defAcceptRoute,
		defPolicyPort2:  defRejectRoute,
		policyTypePort2: setMEDPolicy,
		port1v4Prefix:   advertisedRoutesv4Net2,
		port1v6Prefix:   advertisedRoutesv6Net2,
		port2v4Prefix:   advertisedRoutesv4Net1,
		port2v6Prefix:   advertisedRoutesv6Net1,
		metricValue:     150,
		polNbrv4:        atePort2.IPv4,
		polNbrv6:        atePort2.IPv6,
		isDeletePolicy:  true,
		deleteNbrv4:     atePort1.IPv4,
		deleteNbrv6:     atePort1.IPv6,
		asn:             dutAS,
	}, {
		desc:            "Configure iBGP set Local Preference Import Export Policy",
		rpPolicy:        setLocalPrefPolicy,
		policyTypePort1: setLocalPrefPolicy,
		policyValue:     "100",
		policyStatement: matchStatement1,
		defPolicyPort1:  defRejectRoute,
		defPolicyPort2:  defAcceptRoute,
		policyTypePort2: "",
		port1v4Prefix:   advertisedRoutesv4Net2,
		port1v6Prefix:   advertisedRoutesv6Net2,
		port2v4Prefix:   advertisedRoutesv4Net1,
		port2v6Prefix:   advertisedRoutesv6Net1,
		metricValue:     100,
		polNbrv4:        atePort1.IPv4,
		polNbrv6:        atePort1.IPv6,
		isDeletePolicy:  true,
		deleteNbrv4:     atePort2.IPv4,
		deleteNbrv6:     atePort2.IPv6,
		asn:             dutAS,
	}, {
		desc:            "Configure eBGP set NEXT-STATEMENT Import Export Policy",
		rpPolicy:        setNxtPolicy,
		policyTypePort1: "",
		policyValue:     "70",
		policyStatement: matchStatement1,
		defPolicyPort1:  defAcceptRoute,
		defPolicyPort2:  defRejectRoute,
		policyTypePort2: setNxtPolicy,
		port1v4Prefix:   advertisedRoutesv4Net2,
		port1v6Prefix:   advertisedRoutesv6Net2,
		port2v4Prefix:   advertisedRoutesv4Net1,
		port2v6Prefix:   advertisedRoutesv6Net1,
		metricValue:     70,
		polNbrv4:        atePort2.IPv4,
		polNbrv6:        atePort2.IPv6,
		isDeletePolicy:  true,
		deleteNbrv4:     atePort1.IPv4,
		deleteNbrv6:     atePort1.IPv6,
		asn:             dutAS,
	}, {
		desc:            "Configure eBGP  prepend 10 x local ASN Import Export Policy",
		rpPolicy:        setPrependPolicy,
		policyTypePort1: "",
		policyValue:     "10",
		policyStatement: matchStatement1,
		defPolicyPort1:  defAcceptRoute,
		defPolicyPort2:  defRejectRoute,
		policyTypePort2: setPrependPolicy,
		port1v4Prefix:   advertisedRoutesv4Net2,
		port1v6Prefix:   advertisedRoutesv6Net2,
		port2v4Prefix:   advertisedRoutesv4Net1,
		port2v6Prefix:   advertisedRoutesv6Net1,
		metricValue:     asnRepeatN + 1,
		polNbrv4:        atePort2.IPv4,
		polNbrv6:        atePort2.IPv6,
		isDeletePolicy:  true,
		deleteNbrv4:     atePort1.IPv4,
		deleteNbrv6:     atePort1.IPv6,
		asn:             dutAS,
	}, {
		desc:            "Configure eBGP  prepend 10 x ASN Import Export Policy",
		rpPolicy:        setPrependPolicy,
		policyTypePort1: "",
		policyValue:     "10",
		policyStatement: matchStatement1,
		defPolicyPort1:  defAcceptRoute,
		defPolicyPort2:  defRejectRoute,
		policyTypePort2: setPrependPolicy,
		port1v4Prefix:   advertisedRoutesv4Net2,
		port1v6Prefix:   advertisedRoutesv6Net2,
		port2v4Prefix:   advertisedRoutesv4Net1,
		port2v6Prefix:   advertisedRoutesv6Net1,
		metricValue:     asnRepeatN + 1,
		polNbrv4:        atePort2.IPv4,
		polNbrv6:        atePort2.IPv6,
		isDeletePolicy:  true,
		deleteNbrv4:     atePort1.IPv4,
		deleteNbrv6:     atePort1.IPv6,
		asn:             23456,
	}}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			// Delete BGP import export policy
			if tc.isDeletePolicy {
				deleteBGPImportExportPolicy(t, dut, tc.deleteNbrv4, tc.deleteNbrv6, atePort2.IPv4, atePort2.IPv6)
			}
			// Configure Routing Policy on the DUT.
			configureASLocalPrefMEDPolicy(t, dut, tc.rpPolicy, tc.policyValue, tc.policyStatement, tc.asn)
			if !deviations.DefaultRoutePolicyUnsupported(dut) {
				// Configure BGP default import export policy on Port1
				configureBGPDefaultImportExportPolicy(t, dut, atePort1.IPv4, atePort1.IPv6, tc.defPolicyPort1)
				// Configure BGP default import export policy on Port2
				configureBGPDefaultImportExportPolicy(t, dut, atePort2.IPv4, atePort2.IPv6, tc.defPolicyPort2)
			} else {
				if tc.rpPolicy == setLocalPrefPolicy {
					tc.policyTypePort2 = setLocalPrefPolicy
					// when default policy is not configured on port2 ebgp configuration is needed for setLocalPrefPolicy
					t.Logf("Configuring BGP import export policy on Port2 when default policy is not configured for %v", tc.rpPolicy)
					configureBGPImportExportPolicy(t, dut, atePort2.IPv4, atePort2.IPv6, tc.rpPolicy)
				}
			}
			// Configure BGP import export policy
			configureBGPImportExportPolicy(t, dut, tc.polNbrv4, tc.polNbrv6, tc.rpPolicy)

			// Verify BGP policy
			verifyBgpPolicyTelemetry(t, dut, atePort1.IPv4, tc.defPolicyPort1, tc.policyTypePort1, true)
			verifyBgpPolicyTelemetry(t, dut, atePort1.IPv6, tc.defPolicyPort1, tc.policyTypePort1, false)
			verifyBgpPolicyTelemetry(t, dut, atePort2.IPv4, tc.defPolicyPort2, tc.policyTypePort2, true)
			verifyBgpPolicyTelemetry(t, dut, atePort2.IPv6, tc.defPolicyPort2, tc.policyTypePort2, false)

			// Validate Prefixes
			validateOTGBgpPrefixV4AndASLocalPrefMED(t, otg, dut, otgConfig, atePort1.Name+".BGP4.peer", tc.port1v4Prefix, advertisedRoutesv4PrefixLen, tc.rpPolicy, tc.metricValue)
			validateOTGBgpPrefixV6AndASLocalPrefMED(t, otg, dut, otgConfig, atePort1.Name+".BGP6.peer", tc.port1v6Prefix, advertisedRoutesv6PrefixLen, tc.rpPolicy, tc.metricValue)
			validateOTGBgpPrefixV4AndASLocalPrefMED(t, otg, dut, otgConfig, atePort2.Name+".BGP4.peer", tc.port2v4Prefix, advertisedRoutesv4PrefixLen, tc.rpPolicy, tc.metricValue)
			validateOTGBgpPrefixV6AndASLocalPrefMED(t, otg, dut, otgConfig, atePort2.Name+".BGP6.peer", tc.port2v6Prefix, advertisedRoutesv6PrefixLen, tc.rpPolicy, tc.metricValue)
		})
	}
}

func validateImportRoutingPolicy(t *testing.T, dut *ondatra.DUTDevice, prefix string, metricValue uint32) {
	dni := deviations.DefaultNetworkInstance(dut)
	bgpRIBPath := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Rib()
	locRib := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Rib_AfiSafi_Ipv4Unicast_LocRib](t, dut, bgpRIBPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().LocRib().State())
	found := false
	for k, lr := range locRib.Route {
		prefixAddr := strings.Split(lr.GetPrefix(), "/")
		if prefixAddr[0] == prefix {
			found = true
			t.Logf("Found Route(prefix %s, origin: %v, pathid: %d) => %s", k.Prefix, k.Origin, k.PathId, lr.GetPrefix())
			if !deviations.SkipCheckingAttributeIndex(dut) {
				attrSet := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Rib_AttrSet](t, dut, bgpRIBPath.AttrSet(lr.GetAttrIndex()).State())
				if attrSet == nil || attrSet.GetLocalPref() != metricValue {
					t.Errorf("No local pref found for prefix %s", prefix)
				}
				break
			} else {
				attrSetList := gnmi.GetAll[*oc.NetworkInstance_Protocol_Bgp_Rib_AttrSet](t, dut, bgpRIBPath.AttrSetAny().State())
				foundLP := false
				for _, attrSet := range attrSetList {
					if attrSet.GetLocalPref() == metricValue {
						foundLP = true
						t.Logf("Found local pref %d for prefix %s", attrSet.GetLocalPref(), prefix)
						break
					}
				}
				if !foundLP {
					t.Errorf("No local pref found for prefix %s", prefix)
				}
			}
		}
	}

	if !found {
		t.Errorf("No Route found for prefix %s", prefix)
	}
}

func validateImportRoutingPolicyV6(t *testing.T, dut *ondatra.DUTDevice, prefix string, metricValue uint32) {
	dni := deviations.DefaultNetworkInstance(dut)
	bgpRIBPath := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Rib()
	locRib := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Rib_AfiSafi_Ipv6Unicast_LocRib](t, dut, bgpRIBPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Ipv6Unicast().LocRib().State())
	found := false
	for k, lr := range locRib.Route {
		prefixAddr := strings.Split(lr.GetPrefix(), "/")
		if prefixAddr[0] == prefix {
			found = true
			t.Logf("Found Route(prefix %s, origin: %v, pathid: %d) => %s", k.Prefix, k.Origin, k.PathId, lr.GetPrefix())
			if !deviations.SkipCheckingAttributeIndex(dut) {
				attrSet := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Rib_AttrSet](t, dut, bgpRIBPath.AttrSet(lr.GetAttrIndex()).State())
				if attrSet == nil || attrSet.GetLocalPref() != metricValue {
					t.Errorf("No local pref found for prefix %s", prefix)
				}
				break
			} else {
				attrSetList := gnmi.GetAll[*oc.NetworkInstance_Protocol_Bgp_Rib_AttrSet](t, dut, bgpRIBPath.AttrSetAny().State())
				foundLP := false
				for _, attrSet := range attrSetList {
					if attrSet.GetLocalPref() == metricValue {
						foundLP = true
						t.Logf("Found local pref %d for prefix %s", attrSet.GetLocalPref(), prefix)
						break
					}
				}
				if !foundLP {
					t.Errorf("No local pref found for prefix %s", prefix)
				}
			}
		}
	}

	if !found {
		t.Errorf("No Route found for prefix %s", prefix)
	}
}
