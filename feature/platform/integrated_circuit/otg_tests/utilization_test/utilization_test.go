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

package utilization_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/functional-translators/registrar"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	dutAS                   = 64500
	ateAS                   = 64501
	bgpV6PeerGroup          = "BGP-PEER-GROUP-V6"
	bgpRoutePolicyName      = "BGP-ROUTE-POLICY-ALLOW"
	numBGPRoutes            = 250000
	bgpAdvertisedRouteStart = "2001:DB8:2::1"
	usedThresholdUpper      = uint8(60)
	usedThresholdUpperClear = uint8(50)
)

var (
	fibResource = map[ondatra.Vendor]string{
		ondatra.ARISTA: "Routing/Resource6",
		ondatra.NOKIA:  "ip-lpm-routes",
		ondatra.CISCO:  "central_em_0",
	}
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: 30,
		IPv6Len: 126,
	}
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "192.0.2.2",
		MAC:     "02:00:01:01:01:01",
		IPv6:    "2001:db8::192:0:2:2",
		IPv4Len: 30,
		IPv6Len: 126,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::192:0:2:5",
		IPv4Len: 30,
		IPv6Len: 126,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "192.0.2.6",
		MAC:     "02:00:02:01:01:01",
		IPv6:    "2001:db8::192:0:2:6",
		IPv4Len: 30,
		IPv6Len: 126,
	}
)

type utilization struct {
	name                   string
	maxLimit               uint64
	used                   uint64
	free                   uint64
	upperThreshold         uint8
	upperThresholdClear    uint8
	upperThresholdExceeded bool
}

func (u *utilization) percent() uint8 {
	if u.used == 0 && u.free == 0 {
		return 0
	}
	return uint8(u.used * 100 / (u.used + u.free))
}

func getOptsForFunctionalTranslator(t *testing.T, dut *ondatra.DUTDevice, functionalTranslatorName string) []ygnmi.Option {
	if functionalTranslatorName == "" {
		return nil
	}
	ft, ok := registrar.FunctionalTranslatorRegistry[functionalTranslatorName]
	if !ok {
		t.Fatalf("Functional translator %s is not registered", functionalTranslatorName)
	}
	return []ygnmi.Option{ygnmi.WithFT(ft)}
}

// Helper function to get translator friendly resource name and component name for Ciscoxr8000IntegratedCircuitResourceFt
func getFTCompatibleResourceNameAndComponentName(resourceName string, componentName string) (string, string) {
	ftResourceName := strings.ReplaceAll(resourceName, "_", "-")
	re := regexp.MustCompile(`(NPU)(\d+)$`)
	ftComponent := re.ReplaceAllString(componentName, "$1-$2")
	return ftResourceName, ftComponent
}

func validateUtilizationExceeded(t *testing.T, utzs map[string]*utilization) {
	for c, u := range utzs {
		if u.upperThresholdExceeded && u.percent() < u.upperThresholdClear {
			t.Errorf("upperThresholdExceeded is true for component: %s when it should be false", c)
		}
		if !u.upperThresholdExceeded && u.percent() > u.upperThreshold {
			t.Errorf("upperThresholdExceeded is false for component: %s when it should be true", c)
		}
	}
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestResourceUtilization(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	otgV6Peer, otgPort1, otgConfig := configureOTG(t, otg)

	verifyBgpTelemetry(t, dut)
	gnmi.Replace(t, dut, gnmi.OC().System().Utilization().Resource(fibResource[dut.Vendor()]).Config(), &oc.System_Utilization_Resource{
		Name:                    ygot.String(fibResource[dut.Vendor()]),
		UsedThresholdUpper:      ygot.Uint8(usedThresholdUpper),
		UsedThresholdUpperClear: ygot.Uint8(usedThresholdUpperClear),
	})
	comps := components.FindActiveComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_INTEGRATED_CIRCUIT)
	beforeUtzs := componentUtilizations(t, dut, comps)
	if len(beforeUtzs) != len(comps) {
		t.Fatalf("Couldn't retrieve Utilization information for all Active Components")
	}
	if deviations.Ciscoxr8000IntegratedCircuitResourceFt(dut) != "" {
		validateUtilizationExceeded(t, beforeUtzs)
	}

	injectBGPRoutes(t, otg, otgV6Peer, otgPort1, otgConfig)

	// Use map to store utilization after BGP route installation to compare with cleared state later.
	afterUtzs := make(map[string]*utilization)
	if deviations.Ciscoxr8000IntegratedCircuitResourceFt(dut) != "" {
		validateUtilizationExceeded(t, afterUtzs)
	}

	t.Run("Utilization after BGP route installation", func(t *testing.T) {
		for _, c := range comps {
			t.Run(c, func(t *testing.T) {
				if deviations.Ciscoxr8000IntegratedCircuitResourceFt(dut) != "" {
					if got, want := beforeUtzs[c].name, fibResource[dut.Vendor()]; got != want {
						t.Errorf("Resource name mismatch! got: %s, want: %s", got, want)
					}
				}
				beforePct := beforeUtzs[c].percent()
				t.Logf("Waiting for utilization to increase above %d%%...", beforePct)
				u := awaitUtilization(t, dut, c, func(pct uint8) bool {
					return pct > beforePct
				})
				if u == nil {
					t.Errorf("Utilization Percent didn't increase for component: %s (Started at %d%%)", c, beforePct)
					// Fallback to get current value for map consistency, though test failed.
					afterUtzs[c] = beforeUtzs[c]
				} else if deviations.Ciscoxr8000IntegratedCircuitResourceFt(dut) != "" && u.used+u.free > u.maxLimit {
					t.Errorf("Max limit %d is less than used %d + free %d", u.maxLimit, u.used, u.free)
				} else {
					t.Logf("Before Utilization: %d, After Utilization: %d", beforePct, u.percent())
					afterUtzs[c] = u
				}
			})
		}
	})

	clearBGPRoutes(t, otg, otgV6Peer, otgConfig)

	t.Run("Utilization after BGP route clear", func(t *testing.T) {
		for _, c := range comps {
			t.Run(c, func(t *testing.T) {
				prev, ok := afterUtzs[c]
				if !ok {
					t.Fatalf("No previous utilization data for component %s", c)
				}
				prevPct := prev.percent()
				t.Logf("Waiting for utilization to decrease below %d%%...", prevPct)
				u := awaitUtilization(t, dut, c, func(pct uint8) bool {
					return pct < prevPct
				})

				if u == nil {
					t.Errorf("Utilization Percent didn't decrease for component: %s (Was %d%%)", c, prevPct)
				} else {
					t.Logf("Before Utilization: %d, After Utilization: %d", prevPct, u.percent())
				}
			})
		}
	})
}

// awaitUtilization polls the utilization resource until the predicate function returns true
// or the timeout expires. Returns the final utilization snapshot, or nil if timed out.
func awaitUtilization(t *testing.T, dut *ondatra.DUTDevice, c string, predicate func(uint8) bool) *utilization {
	resName := fibResource[dut.Vendor()]
	if deviations.MismatchedHardwareResourceNameInComponent(dut) {
		resName += "/-"
	}
	path := gnmi.OC().Component(c).IntegratedCircuit().Utilization().Resource(resName)

	var lastVal *utilization

	_, ok := gnmi.Watch(t, dut, path.State(), 2*time.Minute, func(val *ygnmi.Value[*oc.Component_IntegratedCircuit_Utilization_Resource]) bool {
		res, ok := val.Val()
		if !ok {
			return false
		}
		u := &utilization{
			name:                   res.GetName(),
			maxLimit:               res.GetMaxLimit(),
			used:                   res.GetUsed(),
			free:                   res.GetFree(),
			upperThreshold:         res.GetUsedThresholdUpper(),
			upperThresholdClear:    res.GetUsedThresholdUpperClear(),
			upperThresholdExceeded: res.GetUsedThresholdUpperExceeded(),
		}
		opts := getOptsForFunctionalTranslator(t, dut, deviations.Ciscoxr8000IntegratedCircuitResourceFt(dut))
		if len(opts) > 0 {
			ftResourceName, ftComponent := getFTCompatibleResourceNameAndComponentName(resName, c)
			resourcePath := gnmi.OC().Component(ftComponent).IntegratedCircuit().Utilization().Resource(ftResourceName)

			u.used = gnmi.Get(t, dut.GNMIOpts().WithYGNMIOpts(opts...), resourcePath.Used().State())
			u.upperThresholdClear = gnmi.Get(t, dut.GNMIOpts().WithYGNMIOpts(opts...), resourcePath.UsedThresholdUpperClear().State())
			u.upperThreshold = gnmi.Get(t, dut.GNMIOpts().WithYGNMIOpts(opts...), resourcePath.UsedThresholdUpper().State())
			u.maxLimit = gnmi.Get(t, dut.GNMIOpts().WithYGNMIOpts(opts...), resourcePath.MaxLimit().State())
			u.name = strings.ReplaceAll(gnmi.Get(t, dut.GNMIOpts().WithYGNMIOpts(opts...), resourcePath.Name().State()), "-", "_")
		}
		lastVal = u
		return predicate(u.percent())
	}).Await(t)

	if !ok {
		return lastVal
	}
	return &utilization{
		name:                   lastVal.name,
		maxLimit:               lastVal.maxLimit,
		used:                   lastVal.used,
		free:                   lastVal.free,
		upperThreshold:         lastVal.upperThreshold,
		upperThresholdClear:    lastVal.upperThresholdClear,
		upperThresholdExceeded: lastVal.upperThresholdExceeded,
	}
}

func componentUtilizations(t *testing.T, dut *ondatra.DUTDevice, comps []string) map[string]*utilization {
	t.Helper()
	resName := fibResource[dut.Vendor()]
	if deviations.MismatchedHardwareResourceNameInComponent(dut) {
		resName += "/-"
	}
	utzs := map[string]*utilization{}
	for _, c := range comps {
		comp := gnmi.Get(t, dut, gnmi.OC().Component(c).State())
		res := comp.GetIntegratedCircuit().GetUtilization().GetResource(resName)
		utzs[c] = &utilization{
			name:                   res.GetName(),
			maxLimit:               res.GetMaxLimit(),
			used:                   res.GetUsed(),
			free:                   res.GetFree(),
			upperThreshold:         res.GetUsedThresholdUpper(),
			upperThresholdClear:    res.GetUsedThresholdUpperClear(),
			upperThresholdExceeded: res.GetUsedThresholdUpperExceeded(),
		}
		opts := getOptsForFunctionalTranslator(t, dut, deviations.Ciscoxr8000IntegratedCircuitResourceFt(dut))
		if len(opts) > 0 {
			ftResourceName, ftComponent := getFTCompatibleResourceNameAndComponentName(resName, c)
			resourcePath := gnmi.OC().Component(ftComponent).IntegratedCircuit().Utilization().Resource(ftResourceName)

			utzs[c].used = gnmi.Get(t, dut.GNMIOpts().WithYGNMIOpts(opts...), resourcePath.Used().State())
			utzs[c].upperThresholdClear = gnmi.Get(t, dut.GNMIOpts().WithYGNMIOpts(opts...), resourcePath.UsedThresholdUpperClear().State())
			utzs[c].upperThreshold = gnmi.Get(t, dut.GNMIOpts().WithYGNMIOpts(opts...), resourcePath.UsedThresholdUpper().State())
			utzs[c].maxLimit = gnmi.Get(t, dut.GNMIOpts().WithYGNMIOpts(opts...), resourcePath.MaxLimit().State())
			utzs[c].name = strings.ReplaceAll(gnmi.Get(t, dut.GNMIOpts().WithYGNMIOpts(opts...), resourcePath.Name().State()), "-", "_")
		}
	}
	return utzs
}

// configureDUT configures port1-2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))

	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
		fptest.SetPortSpeed(t, dut.Port(t, "port2"))
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
	configureAcceptRoutePolicy(t, dut)
	configureBGPDUT(t, dut)
}

func configureAcceptRoutePolicy(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pd := rp.GetOrCreatePolicyDefinition(bgpRoutePolicyName)
	st, err := pd.AppendNewStatement("id-1")
	if err != nil {
		t.Fatal(err)
	}
	st.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

func configureBGPDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	g := bgp.GetOrCreateGlobal()
	g.As = ygot.Uint32(dutAS)
	g.RouterId = ygot.String(dutPort1.IPv4)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

	pg := bgp.GetOrCreatePeerGroup(bgpV6PeerGroup)
	pg.PeerAs = ygot.Uint32(ateAS)
	pg.PeerGroupName = ygot.String(bgpV6PeerGroup)
	pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		rpl := pg.GetOrCreateApplyPolicy()
		rpl.SetExportPolicy([]string{bgpRoutePolicyName})
		rpl.SetImportPolicy([]string{bgpRoutePolicyName})
	} else {
		pg1af4 := pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		pg1af4.Enabled = ygot.Bool(true)
		pg1rpl4 := pg1af4.GetOrCreateApplyPolicy()
		pg1rpl4.SetExportPolicy([]string{bgpRoutePolicyName})
		pg1rpl4.SetImportPolicy([]string{bgpRoutePolicyName})
	}

	bgpNbr := bgp.GetOrCreateNeighbor(atePort1.IPv6)
	bgpNbr.PeerAs = ygot.Uint32(ateAS)
	bgpNbr.Enabled = ygot.Bool(true)
	bgpNbr.PeerGroup = ygot.String(bgpV6PeerGroup)
	af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	af6.Enabled = ygot.Bool(true)
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config(), niProto)
}

func configureOTG(t *testing.T, otg *otg.OTG) (gosnappi.BgpV6Peer, gosnappi.DeviceIpv6, gosnappi.Config) {
	t.Helper()
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

	iDut1Bgp := iDut1Dev.Bgp().SetRouterId(iDut1Ipv4.Address())
	iDut1Bgp6Peer := iDut1Bgp.Ipv6Interfaces().Add().SetIpv6Name(iDut1Ipv6.Name()).Peers().Add().SetName(atePort1.Name + ".BGP6.peer")
	iDut1Bgp6Peer.SetPeerAddress(iDut1Ipv6.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	iDut1Bgp6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	t.Logf("Pushing config to ATE and starting protocols...")
	otg.PushConfig(t, config)
	time.Sleep(30 * time.Second)
	otg.StartProtocols(t)
	time.Sleep(30 * time.Second)

	otgutils.WaitForARP(t, otg, config, "IPv4")

	return iDut1Bgp6Peer, iDut1Ipv6, config
}

func verifyBgpTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	var nbrIP = []string{atePort1.IPv6}
	t.Logf("Verifying BGP state.")
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	for _, nbr := range nbrIP {
		nbrPath := bgpPath.Neighbor(nbr)
		// Get BGP adjacency state.
		t.Logf("Waiting for BGP neighbor to establish...")
		status, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			state, ok := val.Val()
			return ok && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			t.Fatal("No BGP neighbor formed")
		}
		state, _ := status.Val()
		t.Logf("BGP adjacency for %s: %v", nbr, state)
		if want := oc.Bgp_Neighbor_SessionState_ESTABLISHED; state != want {
			t.Errorf("BGP peer %s status got %d, want %d", nbr, state, want)
		}
	}
}

func injectBGPRoutes(t *testing.T, otg *otg.OTG, bgpPeer gosnappi.BgpV6Peer, otgPort1 gosnappi.DeviceIpv6, otgConfig gosnappi.Config) {
	t.Helper()
	peerRoutes := bgpPeer.V6Routes().Add().SetName(atePort1.Name + ".BGP6.Route")
	peerRoutes.SetNextHopIpv6Address(otgPort1.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	peerRoutes.Addresses().Add().
		SetAddress(bgpAdvertisedRouteStart).
		SetPrefix(128).
		SetCount(numBGPRoutes).SetStep(2)
	peerRoutes.Advanced().SetIncludeLocalPreference(false)

	otg.PushConfig(t, otgConfig)
	time.Sleep(30 * time.Second)
	otg.StartProtocols(t)
	time.Sleep(time.Minute)
}

func clearBGPRoutes(t *testing.T, otg *otg.OTG, bgpPeer gosnappi.BgpV6Peer, otgConfig gosnappi.Config) {
	bgpPeer.V6Routes().Clear()
	otg.PushConfig(t, otgConfig)
	time.Sleep(30 * time.Second)
	otg.StopProtocols(t)
	time.Sleep(time.Minute)
}
