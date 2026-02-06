// Copyright 2024 Google LLC
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

package bgp_import_export_policy_test

import (
	"strings"
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
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

// The testbed consists of ate:port1 -> dut:port1.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// List of variables.
var (
	dutAttrs = attrs.Attributes{
		Desc:    "DUT port towards OTG",
		IPv4:    "10.1.1.0",
		IPv4Len: 31,
		IPv6:    "2001:db8:204:114::0",
		IPv6Len: 127,
	}
	ateAttrs = attrs.Attributes{
		Desc:    "OTG port towards DUT",
		Name:    "ateSrc",
		MAC:     "00:00:01:02:03:04",
		IPv4:    "10.1.1.1",
		IPv4Len: 31,
		IPv6:    "2001:db8:204:114::1",
		IPv6Len: 127,
	}
	dutloAttrs = attrs.Attributes{
		Desc:    "DUT Loopback port for static route injection",
		IPv4:    "1.2.3.4",
		IPv4Len: 24,
		IPv6:    "2001:db8::2:3:4:5",
		IPv6Len: 64,
	}
	loopbackIntfName            string
	dutAdvertisedRoutesv4Net    = []string{"172.16.1.0", "172.16.2.0", "192.168.10.0"}
	dutAdvertisedRoutesv6Net    = []string{"2001:db8:250:110::0", "2001:db8:251:110::0", "2001:db8:299:110::0"}
	otgAdvertisedRoutesv4Net    = []string{"192.0.2.1", "192.0.2.2", "198.51.100.1", "198.51.100.2"}
	otgDeniedRoutesv4Net        = []string{"198.51.100.1", "198.51.100.2"}
	otgAdvertisedRoutesv6Net    = []string{"2001:db8:300:100::0", "2001:db8:300:101::0", "2001:db8:400:100::1", "2001:db8:400:101::1"}
	otgDeniedRoutesv6Net        = []string{"2001:db8:400:100::1", "2001:db8:400:101::1"}
	otgAdvertisedRoutesv4Prefix = []uint32{32, 32, 32, 32}
	otgAdvertisedRoutesv6Prefix = []uint32{127, 127, 128, 128}
	routeCount                  = 1
)

// Constants.
const (
	dutAS          = 65001
	ateAS          = 65002
	peerGrpName    = "eBGP-PEER-GROUP"
	peerLvlPassive = "PeerGrpLevelPassive"
	peerLvlActive  = "PeerGrpLevelActive"
	nbrLvlPassive  = "nbrLevelPassive"
	nbrLvlActive   = "nbrLevelActive"
	rplPermitAll   = "PERMIT-ALL"
	rejectAspath   = "REJECT-AS-PATH"
	regexAsSet     = "REGEX-AS-SET"
)

// configureDUT is used to configure interfaces on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dc := gnmi.OC()
	i1 := dutAttrs.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, i1.GetName(), deviations.DefaultNetworkInstance(dut), 0)
	}

	// Configure loopback addresses to be able to advertise dutAdvertisedRoutesv4Net towards OTG

	for i := range dutAdvertisedRoutesv4Net {
		loopbackIntfName = netutil.LoopbackInterface(t, dut, i)
		loIntf := gnmi.OC().Interface(loopbackIntfName).Subinterface(uint32(i))
		ipv4Addrs := gnmi.LookupAll(t, dut, loIntf.Ipv4().AddressAny().State())
		ipv6Addrs := gnmi.LookupAll(t, dut, loIntf.Ipv6().AddressAny().State())
		if len(ipv4Addrs) == 0 && len(ipv6Addrs) == 0 {
			hostsv4 := hostify(dutAdvertisedRoutesv4Net)
			hostsv6 := hostify(dutAdvertisedRoutesv6Net)
			dutloAttrs.IPv4 = hostsv4[i]
			dutloAttrs.IPv6 = hostsv6[i]
			loop1 := dutloAttrs.NewOCInterface(loopbackIntfName, dut)
			loop1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
			gnmi.Update(t, dut, dc.Interface(loopbackIntfName).Config(), loop1)
		} else {
			v4, ok := ipv4Addrs[0].Val()
			if ok {
				dutloAttrs.IPv4 = v4.GetIp()
			}
			v6, ok := ipv6Addrs[0].Val()
			if ok {
				dutloAttrs.IPv6 = v6.GetIp()
			}
			t.Logf("Got DUT IPv4 loopback address: %v", dutloAttrs.IPv4)
			t.Logf("Got DUT IPv6 loopback address: %v", dutloAttrs.IPv6)
		}
	}

}

func hostify(ipList []string) []string {
	var result []string

	for _, ip := range ipList {
		if strings.Contains(ip, ".") {
			parts := strings.Split(ip, ".")
			parts[len(parts)-1] = "1"
			result = append(result, strings.Join(parts, "."))
		} else if strings.Contains(ip, ":") {
			parts := strings.Split(ip, ":")
			parts[len(parts)-1] = "1"
			result = append(result, strings.Join(parts, ":"))
		} else {
			// No delimiter found, skip or return as-is
			result = append(result, ip)
		}
	}
	return result
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

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Struct is to pass bgp session parameters.
type bgpTestParams struct {
	localAS, peerAS, nbrLocalAS uint32
	peerIP                      string
	transportMode               string
	wantOTGPrefixesv4           []string
	wantOTGPrefixesv6           []string
	wantOTGDeniedPrefixesv4     []string
	wantOTGDeniedPrefixesv6     []string
}

// bgpCreateNbr creates a BGP object with neighbors pointing to ate and returns bgp object.
func bgpCreateNbr(t *testing.T, bgpParams *bgpTestParams, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	t.Helper()
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")

	bgp := niProto.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(bgpParams.localAS)
	global.RouterId = ygot.String(dutAttrs.IPv4)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)

	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	pg := bgp.GetOrCreatePeerGroup(peerGrpName)
	pg.PeerAs = ygot.Uint32(ateAS)
	pg.PeerGroupName = ygot.String(peerGrpName)
	pgT := pg.GetOrCreateTransport()
	pgT.LocalAddress = ygot.String(dutAttrs.IPv4)
	pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)

	nv4 := bgp.GetOrCreateNeighbor(ateAttrs.IPv4)
	nv4.PeerGroup = ygot.String(peerGrpName)
	nv4.PeerAs = ygot.Uint32(ateAS)
	nv4.Enabled = ygot.Bool(true)
	nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	nv4T := nv4.GetOrCreateTransport()
	nv4T.LocalAddress = ygot.String(dutAttrs.IPv4)
	nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)

	switch bgpParams.transportMode {
	case nbrLvlPassive:
		nv4.GetOrCreateTransport().SetPassiveMode(true)
	case nbrLvlActive:
		nv4.GetOrCreateTransport().SetPassiveMode(false)
	case peerLvlPassive:
		pg.GetOrCreateTransport().SetPassiveMode(true)
	case peerLvlActive:
		pg.GetOrCreateTransport().SetPassiveMode(false)
	}

	return niProto
}

// bgpClearConfig removes all BGP configuration from the DUT.
func bgpClearConfig(t *testing.T, dut *ondatra.DUTDevice) {
	resetBatch := &gnmi.SetBatch{}
	gnmi.BatchDelete(resetBatch, gnmi.OC().RoutingPolicy().Config())
	gnmi.BatchDelete(resetBatch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)).Config())

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

// verifyBgpTelemetry checks that the dut has an established BGP session with reasonable settings.
func verifyBgpTelemetry(t *testing.T, dut *ondatra.DUTDevice, wantState oc.E_Bgp_Neighbor_SessionState, transMode string, transModeOnATE string) {
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateAttrs.IPv4)
	if deviations.BgpSessionStateIdleInPassiveMode(dut) {
		if transModeOnATE == nbrLvlPassive || transModeOnATE == peerLvlPassive {
			t.Logf("BGP session state idle is supported in passive mode, transMode: %s, transModeOnATE: %s", transMode, transModeOnATE)
			wantState = oc.Bgp_Neighbor_SessionState_IDLE
		}
	}
	// Get BGP adjacency state
	t.Log("Checking BGP neighbor to state...")
	_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, present := val.Val()
		return present && state == wantState
	}).Await(t)
	if !ok {
		fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
		t.Errorf("BGP Session state is not as expected.")
	}
	status := gnmi.Get(t, dut, nbrPath.SessionState().State())
	t.Logf("BGP adjacency for %s: %s", ateAttrs.IPv4, status)
	t.Logf("wantState: %s, status: %s", wantState, status)
	if status != wantState {
		t.Errorf("BGP peer %s status got %d, want %d", ateAttrs.IPv4, status, wantState)
	}

	nbrTransMode := gnmi.Get(t, dut, nbrPath.Transport().State())
	pgTransMode := gnmi.Get(t, dut, statePath.PeerGroup(peerGrpName).Transport().State())
	t.Logf("Neighbor level passive mode is set to %v on DUT", nbrTransMode.GetPassiveMode())
	t.Logf("Peer group level passive mode is set to %v on DUT", pgTransMode.GetPassiveMode())

	// Check transport mode telemetry.
	switch transMode {
	case nbrLvlPassive:
		if nbrTransMode.GetPassiveMode() != true {
			t.Errorf("Neighbor level passive mode is not set to true on DUT. want true, got %v", nbrTransMode.GetPassiveMode())
		}
		t.Logf("Neighbor level passive mode is set to %v on DUT", nbrTransMode.GetPassiveMode())
	case nbrLvlActive:
		if nbrTransMode.GetPassiveMode() != false {
			t.Errorf("Neighbor level passive mode is not set to false on DUT. want false, got %v", nbrTransMode.GetPassiveMode())
		}
		t.Logf("Neighbor level passive mode is set to %v on DUT", nbrTransMode.GetPassiveMode())
	case peerLvlPassive:
		if pgTransMode.GetPassiveMode() != true {
			t.Errorf("Peer group level passive mode is not set to true on DUT. want true, got %v", pgTransMode.GetPassiveMode())
		}
		t.Logf("Peer group level passive mode is set to %v on DUT", pgTransMode.GetPassiveMode())
	case peerLvlActive:
		if pgTransMode.GetPassiveMode() != false {
			t.Errorf("Peer group level passive mode is not set to false on DUT. want false, got %v", pgTransMode.GetPassiveMode())
		}
		t.Logf("Peer group level passive mode is set to %v on DUT", pgTransMode.GetPassiveMode())
	}
}

// Function to configure ATE configs based on args and returns ate topology handle.
func configureATE(t *testing.T, ateParams *bgpTestParams, optionalAsPath []uint32, optionalAsPathRouteTarget []string) gosnappi.Config {
	t.Helper()
	ate := ondatra.ATE(t, "ate")
	port1 := ate.Port(t, "port1")
	topo := gosnappi.NewConfig()

	topo.Ports().Add().SetName(port1.ID())
	dev := topo.Devices().Add().SetName(ateAttrs.Name)
	eth := dev.Ethernets().Add().SetName(ateAttrs.Name + ".Eth")
	eth.Connection().SetPortName(port1.ID())
	eth.SetMac(ateAttrs.MAC)

	ip := eth.Ipv4Addresses().Add().SetName(dev.Name() + ".IPv4")
	ip.SetAddress(ateAttrs.IPv4).SetGateway(dutAttrs.IPv4).SetPrefix(uint32(ateAttrs.IPv4Len))

	ip6 := eth.Ipv6Addresses().Add().SetName(dev.Name() + ".IPv6")
	ip6.SetAddress(ateAttrs.IPv6).SetGateway(dutAttrs.IPv6).SetPrefix(uint32(ateAttrs.IPv6Len))

	// Configure BGP peers
	bgp4 := dev.Bgp().SetRouterId(ateAttrs.IPv4)
	peerBGP4 := bgp4.Ipv4Interfaces().Add().SetIpv4Name(ip.Name()).Peers().Add()
	peerBGP4.SetName(ateAttrs.Name + ".BGP4.peer")
	peerBGP4.SetPeerAddress(ip.Gateway()).SetAsNumber(uint32(ateParams.localAS))
	peerBGP4.SetAsType(gosnappi.BgpV4PeerAsType.EBGP)

	//bgp6 := dev.Bgp().SetRouterId(ateAttrs.IPv6)
	peerBGP6 := bgp4.Ipv6Interfaces().Add().SetIpv6Name(ip6.Name()).Peers().Add()
	peerBGP6.SetName(ateAttrs.Name + ".BGP6.peer")
	peerBGP6.SetPeerAddress(ip6.Gateway()).SetAsNumber(uint32(ateParams.localAS))
	peerBGP6.SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

	switch ateParams.transportMode {
	case nbrLvlPassive:
		peerBGP4.Advanced().SetPassiveMode(true)
	case peerLvlPassive:
		peerBGP4.Advanced().SetPassiveMode(true)
	case peerLvlActive:
		peerBGP4.Advanced().SetPassiveMode(false)
	case nbrLvlActive:
		peerBGP4.Advanced().SetPassiveMode(false)
	}

	// Configure BGP routes to be pushed to DUT
	bgp4PeerRoutes := peerBGP4.V4Routes().Add().SetName(ateAttrs.Name + ".BGP4.peer" + ".RR4")
	bgp4PeerRoutes.SetNextHopIpv4Address(ip.Address()).SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	for i, rr := range ateParams.wantOTGPrefixesv4 {
		bgp4PeerRoutes.Addresses().Add().SetAddress(rr).SetPrefix(otgAdvertisedRoutesv4Prefix[i]).SetCount(uint32(routeCount))
		if len(optionalAsPath) > 0 && len(optionalAsPathRouteTarget) > 0 && contains(optionalAsPathRouteTarget, rr) {
			asp4 := bgp4PeerRoutes.AsPath().Segments().Add()
			asp4.SetAsNumbers(optionalAsPath)
		}

	}

	bgp6PeerRoutes := peerBGP6.V6Routes().Add().SetName(ateAttrs.Name + ".BGP6.peer" + ".RR6")
	bgp6PeerRoutes.SetNextHopIpv6Address(ip6.Address()).SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	for i, rr := range ateParams.wantOTGPrefixesv6 {
		bgp6PeerRoutes.Addresses().Add().SetAddress(rr).SetPrefix(otgAdvertisedRoutesv6Prefix[i]).SetCount(uint32(routeCount))
	}

	return topo
}

func verifyOTGBGPTelemetry(t *testing.T, otg *otg.OTG, c gosnappi.Config) {
	// nbrPath := gnmi.OTG().BgpPeer("ateSrc.BGP4.peer")
	t.Log("OTG telemetry does not support checking transport mode.")
}

func verifyPrefixesTelemetry(t *testing.T, dut *ondatra.DUTDevice, wantInstalled uint32, isV4 bool) {
	t.Helper()
	t.Logf("Verify BGP prefix count")
	if isV4 {
		verifyPrefixesTelemetryV4(t, dut, wantInstalled)
	} else {
		verifyPrefixesTelemetryV6(t, dut, wantInstalled)
	}
}

// verifyPrefixesTelemetry confirms that the dut shows the correct numbers of installed, sent and
// received IPv4 prefixes
func verifyPrefixesTelemetryV4(t *testing.T, dut *ondatra.DUTDevice, wantInstalled uint32) {
	t.Helper()
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	prefixesv4 := statePath.Neighbor(ateAttrs.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
	_, ok := gnmi.Watch(t, dut, prefixesv4.Installed().State(), 30*time.Second, func(val *ygnmi.Value[uint32]) bool {
		gotVal, present := val.Val()
		if !present {
			return false
		}
		return gotVal == wantInstalled
	}).Await(t)
	if !ok {
		gotInstalled := gnmi.Get(t, dut, prefixesv4.Installed().State())
		t.Fatalf("Installed prefixes mismatch: got %v, want %v", gotInstalled, wantInstalled)
	}
}

// verifyPrefixesTelemetryV6 confirms that the dut shows the correct numbers of installed, sent and
// received IPv6 prefixes
func verifyPrefixesTelemetryV6(t *testing.T, dut *ondatra.DUTDevice, wantInstalledv6 uint32) {
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	prefixesv6 := statePath.Neighbor(ateAttrs.IPv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Prefixes()

	if gotInstalledv6 := gnmi.Get(t, dut, prefixesv6.Installed().State()); gotInstalledv6 != wantInstalledv6 {
		t.Errorf("IPV6 Installed prefixes mismatch: got %v, want %v", gotInstalledv6, wantInstalledv6)
	}
}

func configurePrefixMatchPolicy(t *testing.T, dut *ondatra.DUTDevice, prefixSet, prefixSubnetRange, maskLen string, ipPrefixSet []string) *oc.RoutingPolicy {
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pset := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(prefixSet)
	for _, pref := range ipPrefixSet {
		pset.GetOrCreatePrefix(pref+"/"+maskLen, prefixSubnetRange)
	}
	mode := oc.PrefixSet_Mode_IPV4
	if maskLen == "128" {
		mode = oc.PrefixSet_Mode_IPV6
	}
	if !deviations.SkipPrefixSetMode(dut) {
		pset.SetMode(mode)
	}

	pdef := rp.GetOrCreatePolicyDefinition(prefixSet)
	stmt5, err := pdef.AppendNewStatement("10")
	if err != nil {
		t.Logf("Statement definition: %v", err)
	}
	stmt5.GetOrCreateConditions().GetOrCreateMatchPrefixSet().PrefixSet = ygot.String(prefixSet)
	stmt5.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	return rp
}

func configureRoutePolicies(t *testing.T, dut *ondatra.DUTDevice, policyType string) {
	t.Helper()
	if policyType == "allow-all" {
		d := &oc.Root{}
		rp := d.GetOrCreateRoutingPolicy()
		pdef := rp.GetOrCreatePolicyDefinition(rplPermitAll)
		stmt, err := pdef.AppendNewStatement("10")
		if err != nil {
			t.Fatalf("Failed to append statement: %v", err)
		}
		stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
		gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)

		bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)).Bgp()
		neighborPolicy := bgpPath.Neighbor(ateAttrs.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()
		gnmi.Replace(t, dut, neighborPolicy.ImportPolicy().Config(), []string{rplPermitAll})
		gnmi.Replace(t, dut, neighborPolicy.ExportPolicy().Config(), []string{rplPermitAll})
		return
	}
	if policyType == "pfx-based-allow" {
		bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)).Bgp()
		batchConfig := &gnmi.SetBatch{}

		gnmi.BatchUpdate(batchConfig, gnmi.OC().RoutingPolicy().Config(), configurePrefixMatchPolicy(t, dut, "EBGP-IMPORT-IPV4", "exact", "32", []string{otgAdvertisedRoutesv4Net[0], otgAdvertisedRoutesv4Net[1]}))

		// Apply the above policies to the respective peering at the repective AFI-SAFI levels
		gnmi.BatchReplace(batchConfig, bgpPath.Neighbor(ateAttrs.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy().ImportPolicy().Config(), []string{"EBGP-IMPORT-IPV4"})

		batchConfig.Set(t, dut)
	}
	if policyType == "as-path-deny" {
		if deviations.BgpAspathsetUnsupported(dut) {
			if dut.Vendor() == ondatra.ARISTA {
				cfgplugins.DeviationAristaRoutingPolicyBGPAsPathSetUnsupported(t, dut, "aclRegexAsPathAllowed", "FILTER-IN", "^65002$")
			}
		} else {
			d := &oc.Root{}
			rp := d.GetOrCreateRoutingPolicy()
			rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateAsPathSet(regexAsSet).SetAsPathSetMember([]string{"65002 .*"})
			pdefAs := rp.GetOrCreatePolicyDefinition(rejectAspath)

			stmt70, err := pdefAs.AppendNewStatement("20")
			if err != nil {
				t.Errorf("Error while creating new statement %v", err)
			}
			stmt70.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE
			stmt70.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchAsPathSet().SetAsPathSet(regexAsSet)
			stmt70.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchAsPathSet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)

			stmt80, err := pdefAs.AppendNewStatement("30")
			if err != nil {
				t.Errorf("Error while creating new statement %v", err)
			}
			stmt80.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
			gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)

			bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)).Bgp()
			neighborPolicy := bgpPath.Neighbor(ateAttrs.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()
			gnmi.Replace(t, dut, neighborPolicy.ImportPolicy().Config(), []string{rejectAspath})
		}

	}
}

func TestBgpImportExportPolicy(t *testing.T) {
	t.Logf(" **************")
	t.Logf(" *** RT1.64 ***")
	t.Logf(" **************")
	dutIP := dutAttrs.IPv4
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	// Configure interface on the DUT
	t.Log("Start DUT interface Config")
	configureDUT(t, dut)

	// Configure Network instance type on DUT
	t.Log("Configure Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	// Verify Port Status
	t.Log("Verifying port status")
	verifyPortsUp(t, dut.Device)

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")

	cases := []struct {
		name             string
		dutConf          *oc.NetworkInstance_Protocol
		ateConf          gosnappi.Config
		wantBGPState     oc.E_Bgp_Neighbor_SessionState
		dutTransportMode string
		otgTransportMode string
		expectedPrefixes uint32
		policyType       string
	}{
		{
			name:             "### RT-1.64.1 Verify BGP Peering without policy",
			dutConf:          bgpCreateNbr(t, &bgpTestParams{localAS: dutAS, peerAS: ateAS, nbrLocalAS: ateAS, transportMode: nbrLvlPassive}, dut),
			ateConf:          configureATE(t, &bgpTestParams{localAS: ateAS, peerIP: dutIP, transportMode: nbrLvlActive, wantOTGPrefixesv4: otgAdvertisedRoutesv4Net, wantOTGPrefixesv6: otgAdvertisedRoutesv6Net, wantOTGDeniedPrefixesv4: otgDeniedRoutesv4Net, wantOTGDeniedPrefixesv6: otgDeniedRoutesv6Net}, []uint32{}, []string{}),
			wantBGPState:     oc.Bgp_Neighbor_SessionState_ESTABLISHED,
			dutTransportMode: nbrLvlPassive,
			otgTransportMode: nbrLvlActive,
			expectedPrefixes: 4,
			policyType:       "allow-all",
		},
		{
			name:             "### RT-1.64.2 Test Export Policy (Prefix-list based)",
			dutConf:          bgpCreateNbr(t, &bgpTestParams{localAS: dutAS, peerAS: ateAS, nbrLocalAS: ateAS, transportMode: nbrLvlPassive}, dut),
			ateConf:          configureATE(t, &bgpTestParams{localAS: ateAS, peerIP: dutIP, transportMode: nbrLvlActive, wantOTGPrefixesv4: otgAdvertisedRoutesv4Net, wantOTGPrefixesv6: otgAdvertisedRoutesv6Net, wantOTGDeniedPrefixesv4: otgDeniedRoutesv4Net, wantOTGDeniedPrefixesv6: otgDeniedRoutesv6Net}, []uint32{}, []string{}),
			wantBGPState:     oc.Bgp_Neighbor_SessionState_ESTABLISHED,
			dutTransportMode: nbrLvlPassive,
			otgTransportMode: nbrLvlActive,
			expectedPrefixes: 2,
			policyType:       "pfx-based-allow",
		},
		{
			name:             "### RT-1.64.3 Test Import Policy (AS-Path based)",
			dutConf:          bgpCreateNbr(t, &bgpTestParams{localAS: dutAS, peerAS: ateAS, nbrLocalAS: ateAS, transportMode: nbrLvlPassive}, dut),
			ateConf:          configureATE(t, &bgpTestParams{localAS: ateAS, peerIP: dutIP, transportMode: nbrLvlActive, wantOTGPrefixesv4: otgAdvertisedRoutesv4Net, wantOTGPrefixesv6: otgAdvertisedRoutesv6Net, wantOTGDeniedPrefixesv4: otgDeniedRoutesv4Net, wantOTGDeniedPrefixesv6: otgDeniedRoutesv6Net}, []uint32{}, []string{}),
			wantBGPState:     oc.Bgp_Neighbor_SessionState_ESTABLISHED,
			dutTransportMode: nbrLvlPassive,
			otgTransportMode: nbrLvlActive,
			expectedPrefixes: 4,
			policyType:       "as-path-deny",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {

			t.Logf("Clear BGP configuration")
			bgpClearConfig(t, dut)

			t.Logf("Configure BGP Configs on DUT")
			gnmi.Replace(t, dut, dutConfPath.Config(), tc.dutConf)
			fptest.LogQuery(t, "DUT BGP Config ", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))

			t.Logf("Configuring route policies")
			configureRoutePolicies(t, dut, tc.policyType)

			t.Log("Configure BGP on OTG")
			ate.OTG().PushConfig(t, tc.ateConf)
			ate.OTG().StartProtocols(t)
			otgutils.WaitForARP(t, ate.OTG(), tc.ateConf, "IPv4")
			otgutils.WaitForARP(t, ate.OTG(), tc.ateConf, "IPv6")

			t.Logf("Verify BGP telemetry")
			verifyBgpTelemetry(t, dut, tc.wantBGPState, tc.dutTransportMode, tc.otgTransportMode)

			t.Logf("Verify BGP telemetry on OTG")
			verifyOTGBGPTelemetry(t, ate.OTG(), tc.ateConf)

			t.Logf("Verify prefixes telemetry on DUT")
			verifyPrefixesTelemetry(t, dut, tc.expectedPrefixes, true)

			t.Log("Clear BGP Configs on ATE")
			ate.OTG().StopProtocols(t)

		})
	}
}
