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

package aggregate_admin_down_drain_test

import (
	"errors"
	"fmt"
	"math"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/netinstisis"
	"github.com/openconfig/ondatra/gnmi/oc/networkinstance"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	dutID   = "dut"
	ateID   = "ate"
	port1ID = "port1"
	port2ID = "port2"
	port3ID = "port3"
	port4ID = "port4"

	ipv4PrefixLen = 30
	ipv6PrefixLen = 126

	// Prefix lengths of the routes advertised by the ATE.
	routeV4PrefixLen = 24
	routeV6PrefixLen = 64

	defaultSubIdx       = 0
	lagMinLinks         = 1
	ateLagID            = 1
	disabledMemberCount = 2

	// OTG object name suffixes set by otgconfighelpers.ConfigureNetworkInterface.
	otgDevSuffix  = ".Dev"
	otgEthSuffix  = ".Eth"
	otgIPv4Suffix = ".IPv4"
	otgIPv6Suffix = ".IPv6"

	b2CoreNI          = "B2_CORE"
	vlanID            = 100
	bgpName           = "BGP"
	policyName        = "ALLOW-ALL"
	policyStatementID = "accept-all"
	// Peer-group name prefix used by cfgplugins.AppendBGPNeighbor for the LAG neighbors.
	lagPeerGroupPrefix = "lag"

	bgpHoldTime  = 90
	bgpKeepalive = 30

	dutAS      = 64500
	ateLagAS   = 64501
	atePort4AS = 64502

	dutRouterID       = "192.0.2.1"
	dutB2CoreRouterID = "192.0.2.9"

	isisAreaDUT     = "49.0001"
	isisSysIDDUT    = "1920.0000.2001"
	isisAreaATE     = "490001"
	isisSysIDATELag = "192000002002"
	isisSysIDATEP4  = "192000002003"
	isisMetric      = 10
	isisLevel       = 2

	isisHelloInterval   = 10
	isisDeadInterval    = 90
	isisHelloMultiplier = 9

	isisUpTimestampTolerance = time.Second
	// Up-timestamps above this value are nanoseconds, below it seconds.
	nsTimestampThreshold = 1e15

	ateLagName = "ateLag"
	ateLagMAC  = "02:00:01:01:01:01"

	bgpRouteCount     = 10000
	isisRouteCount    = 1
	transitRouteCount = 1
	b2CoreRouteCount  = 1
	bgpV4RouteStart   = "100.64.0.0"
	bgpV6RouteStart   = "2001:db8:1000::"
	bgpV4FlowStart    = "100.64.0.1"
	bgpV6FlowStart    = "2001:db8:1000::1"
	bgpV4FlowStep     = "0.0.1.0"
	bgpV6FlowStep     = "0:0:0:1::"
	isisV4Route       = "198.18.0.0"
	isisV6Route       = "2001:db8:2000::"
	isisV4FlowDst     = "198.18.0.1"
	isisV6FlowDst     = "2001:db8:2000::1"
	transitV4Route    = "198.51.100.0"
	transitV6Route    = "2001:db8:3000::"
	transitV4Host     = "198.51.100.1"
	transitV6Host     = "2001:db8:3000::1"
	b2CoreV4Route     = "203.0.113.0"
	b2CoreV6Route     = "2001:db8:4000::"
	b2CoreV4Host      = "203.0.113.1"
	b2CoreV6Host      = "2001:db8:4000::1"

	frameSize        = 512
	udpSrcPortStart  = 49152
	udpSrcPortCount  = 64
	udpDstPort       = 50000
	lossTolerancePct = 0.5
	fullLossPct      = 100.0
	noLossPct        = 0.0

	bgpFwdFlowRatePct    = 4
	isisFwdFlowRatePct   = 1
	revFlowRatePct       = 5
	b2CoreFlowRatePct    = 5
	trafficSettleUpdates = 2
	trafficWindowUpdates = 10
	trafficWindowTimeout = 2 * time.Minute

	flowStartTimeout  = 30 * time.Second
	flowStartAttempts = 2
	flowStopTimeout   = 30 * time.Second

	trafficConvergenceTimeout = 4 * time.Minute

	minMemberSharePct = 10.0

	intfTimeout          = 2 * time.Minute
	lacpTimeout          = 3 * time.Minute
	bgpTimeout           = 3 * time.Minute
	isisTimeout          = 3 * time.Minute
	prefixTimeout        = 5 * time.Minute
	counterStableTimeout = 2 * time.Minute
	counterStableDelta   = 100
	counterExportTimeout = 3 * time.Minute

	// Share of the ATE-received packets that the member counters must show before they are trusted.
	minEgressCounterFraction = 0.98

	flowV4Fwd     = "ipv4-port4-to-lag-bgp"
	flowV4IsisFwd = "ipv4-port4-to-lag-isis"
	flowV6Fwd     = "ipv6-port4-to-lag-bgp"
	flowV6IsisFwd = "ipv6-port4-to-lag-isis"
	flowV4Rev     = "ipv4-lag-to-port4"
	flowV6Rev     = "ipv6-lag-to-port4"

	flowB2V4Fwd = "b2core-ipv4-port4-to-lag"
	flowB2V6Fwd = "b2core-ipv6-port4-to-lag"
	flowB2V4Rev = "b2core-ipv4-lag-to-port4"
	flowB2V6Rev = "b2core-ipv6-lag-to-port4"
)

var (
	dutLag = attrs.Attributes{
		Desc:    "RT-5.18 LAG under test",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::1",
		IPv6Len: ipv6PrefixLen,
	}
	dutPort4 = attrs.Attributes{
		Desc:    "RT-5.18 standalone transit link",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::5",
		IPv6Len: ipv6PrefixLen,
	}
	dutLagVlan = cfgplugins.DUTSubInterfaceData{
		VlanID:        vlanID,
		IPv4Address:   net.ParseIP("192.0.2.9"),
		IPv4PrefixLen: ipv4PrefixLen,
		IPv6Address:   net.ParseIP("2001:db8::9"),
		IPv6PrefixLen: ipv6PrefixLen,
	}
	dutPort4Vlan = cfgplugins.DUTSubInterfaceData{
		VlanID:        vlanID,
		IPv4Address:   net.ParseIP("192.0.2.13"),
		IPv4PrefixLen: ipv4PrefixLen,
		IPv6Address:   net.ParseIP("2001:db8::d"),
		IPv6PrefixLen: ipv6PrefixLen,
	}

	ateLagIntf = &otgconfighelpers.InterfaceProperties{
		Name:        "ateLagIntf",
		MAC:         "02:00:01:01:01:02",
		IPv4:        "192.0.2.2",
		IPv4Gateway: "192.0.2.1",
		IPv4Len:     ipv4PrefixLen,
		IPv6:        "2001:db8::2",
		IPv6Gateway: "2001:db8::1",
		IPv6Len:     ipv6PrefixLen,
	}
	ateLagVlanIntf = &otgconfighelpers.InterfaceProperties{
		Name:        "ateLagVlan100",
		MAC:         "02:00:01:01:01:03",
		Vlan:        vlanID,
		IPv4:        "192.0.2.10",
		IPv4Gateway: "192.0.2.9",
		IPv4Len:     ipv4PrefixLen,
		IPv6:        "2001:db8::a",
		IPv6Gateway: "2001:db8::9",
		IPv6Len:     ipv6PrefixLen,
	}
	atePort4Intf = &otgconfighelpers.InterfaceProperties{
		Name:        "atePort4",
		MAC:         "02:00:04:01:01:01",
		IPv4:        "192.0.2.6",
		IPv4Gateway: "192.0.2.5",
		IPv4Len:     ipv4PrefixLen,
		IPv6:        "2001:db8::6",
		IPv6Gateway: "2001:db8::5",
		IPv6Len:     ipv6PrefixLen,
	}
	atePort4VlanIntf = &otgconfighelpers.InterfaceProperties{
		Name:        "atePort4Vlan100",
		MAC:         "02:00:04:01:01:02",
		Vlan:        vlanID,
		IPv4:        "192.0.2.14",
		IPv4Gateway: "192.0.2.13",
		IPv4Len:     ipv4PrefixLen,
		IPv6:        "2001:db8::e",
		IPv6Gateway: "2001:db8::d",
		IPv6Len:     ipv6PrefixLen,
	}

	defaultLagPeers   = []string{ateLagIntf.IPv4, ateLagIntf.IPv6}
	defaultPort4Peers = []string{atePort4Intf.IPv4, atePort4Intf.IPv6}
	b2CoreLagPeers    = []string{ateLagVlanIntf.IPv4, ateLagVlanIntf.IPv6}
	b2CorePort4Peers  = []string{atePort4VlanIntf.IPv4, atePort4VlanIntf.IPv6}

	defaultFlows = []string{flowV4Fwd, flowV4IsisFwd, flowV6Fwd, flowV6IsisFwd, flowV4Rev, flowV6Rev}
	b2CoreFlows  = []string{flowB2V4Fwd, flowB2V6Fwd, flowB2V4Rev, flowB2V6Rev}

	flowSpecs = []flowSpec{
		{name: flowV4Fwd, tx: atePort4Intf, rx: ateLagIntf, src: transitV4Host, dst: bgpV4FlowStart, dstStep: bgpV4FlowStep, dstCount: bgpRouteCount, ratePct: bgpFwdFlowRatePct},
		{name: flowV4IsisFwd, tx: atePort4Intf, rx: ateLagIntf, src: transitV4Host, dst: isisV4FlowDst, ratePct: isisFwdFlowRatePct},
		{name: flowV6Fwd, ipv6: true, tx: atePort4Intf, rx: ateLagIntf, src: transitV6Host, dst: bgpV6FlowStart, dstStep: bgpV6FlowStep, dstCount: bgpRouteCount, ratePct: bgpFwdFlowRatePct},
		{name: flowV6IsisFwd, ipv6: true, tx: atePort4Intf, rx: ateLagIntf, src: transitV6Host, dst: isisV6FlowDst, ratePct: isisFwdFlowRatePct},
		{name: flowV4Rev, tx: ateLagIntf, rx: atePort4Intf, src: bgpV4FlowStart, srcStep: bgpV4FlowStep, srcCount: bgpRouteCount, dst: transitV4Host, ratePct: revFlowRatePct},
		{name: flowV6Rev, ipv6: true, tx: ateLagIntf, rx: atePort4Intf, src: bgpV6FlowStart, srcStep: bgpV6FlowStep, srcCount: bgpRouteCount, dst: transitV6Host, ratePct: revFlowRatePct},
		{name: flowB2V4Fwd, tx: atePort4VlanIntf, rx: ateLagVlanIntf, src: transitV4Host, dst: b2CoreV4Host, ratePct: b2CoreFlowRatePct},
		{name: flowB2V6Fwd, ipv6: true, tx: atePort4VlanIntf, rx: ateLagVlanIntf, src: transitV6Host, dst: b2CoreV6Host, ratePct: b2CoreFlowRatePct},
		{name: flowB2V4Rev, tx: ateLagVlanIntf, rx: atePort4VlanIntf, src: b2CoreV4Host, dst: transitV4Host, ratePct: b2CoreFlowRatePct},
		{name: flowB2V6Rev, ipv6: true, tx: ateLagVlanIntf, rx: atePort4VlanIntf, src: b2CoreV6Host, dst: transitV6Host, ratePct: b2CoreFlowRatePct},
	}
)

type testEnv struct {
	dut              *ondatra.DUTDevice
	ate              *ondatra.ATEDevice
	otg              *otg.OTG
	otgConfig        gosnappi.Config
	lagName          string
	dutMembers       []*ondatra.Port
	dutPort4         *ondatra.Port
	b2CoreConfigured bool
}

type testCase struct {
	name string
	desc string
	fn   func(t *testing.T, env *testEnv) error
}

type bgpNeighbor struct {
	address      string
	localAddress string
	peerAS       uint32
	isV4         bool
}

type flowSpec struct {
	name     string
	ipv6     bool
	tx, rx   *otgconfighelpers.InterfaceProperties
	src      string
	srcStep  string
	srcCount uint32
	dst      string
	dstStep  string
	dstCount uint32
	ratePct  float32
}

type flowCounters struct {
	tx, rx uint64
}

type portCounters struct {
	inPkts, outPkts, inOctets, outOctets uint64
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestAggregateAdminDownDrain(t *testing.T) {
	dut := ondatra.DUT(t, dutID)
	ate := ondatra.ATE(t, ateID)
	env := &testEnv{
		dut:        dut,
		ate:        ate,
		otg:        ate.OTG(),
		lagName:    netutil.NextAggregateInterface(t, dut),
		dutMembers: []*ondatra.Port{dut.Port(t, port1ID), dut.Port(t, port2ID), dut.Port(t, port3ID)},
		dutPort4:   dut.Port(t, port4ID),
	}
	t.Logf("Using LAG %s with members %v", env.lagName, portNames(env.dutMembers))

	t.Cleanup(func() { cleanupTestbed(t, env) })

	configureDUT(t, env)
	env.otgConfig = configureATE(t, env, false)
	env.otg.PushConfig(t, env.otgConfig)
	env.otg.StartProtocols(t)
	defer env.otg.StopProtocols(t)
	if err := awaitLACPMembersUp(t, env.dut, env.lagName, env.dutMembers); err != nil {
		t.Fatalf("%s did not come up after configuration: %v", env.lagName, err)
	}

	testCases := []testCase{
		{
			name: "RT-5.18.1_Admin_Down_Drain_Disable",
			desc: "Disable the LAG and verify eBGP/IS-IS over the LAG go down, Port-4 protocols stay up and LAG traffic loss is 100%",
			fn:   testAdminDownDrainDisable,
		},
		{
			name: "RT-5.18.2_Admin_Down_Drain_Reenable",
			desc: "Re-enable the LAG and verify protocols, installed prefixes and 0% traffic loss are restored",
			fn:   testAdminDownDrainReenable,
		},
		{
			name: "RT-5.18.3_Negative_Test_Admin_Down_On_Individual_Member_Links",
			desc: "Disable 2 of 3 member links and verify the LAG, protocols and traffic remain up over the last member",
			fn:   testMemberLinksAdminDown,
		},
		{
			name: "RT-5.18.4_Negative_Test_Admin_Down_On_Already_Disabled_LAG",
			desc: "Send a redundant admin-down to a disabled LAG and verify it is accepted without changing state",
			fn:   testRedundantAdminDown,
		},
		{
			name: "RT-5.18.5_Uplink_LAG_Member_Retention_And_Inband_BGP_Reachability_In_B2Core",
			desc: "Verify LAG members retain aggregate-id/forwarding-viable and B2_CORE VLAN eBGP and traffic are up across all members",
			fn:   testUplinkMemberRetentionB2Core,
		},
		{
			name: "RT-5.18.6_Differential_Member_Drain_Behavior",
			desc: "Drain Port-1 with forwarding-viable=false and verify B2_CORE eBGP stays up and egress traffic avoids Port-1",
			fn:   testDifferentialMemberDrain,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Description: %s", tc.desc)
			defer env.otg.StopTraffic(t)
			if err := tc.fn(t, env); err != nil {
				t.Error(err)
			}
		})
	}
}

// testAdminDownDrainDisable implements RT-5.18.1.
func testAdminDownDrainDisable(t *testing.T, env *testEnv) error {
	if err := ensureLagEnabled(t, env); err != nil {
		return err
	}
	var errs []error

	t.Log("Step 1 - Verify eBGP sessions, IS-IS adjacencies and installed prefixes")
	errs = append(errs, verifyDefaultControlPlaneUp(t, env))

	t.Log("Step 2 - Resolve ARP/ND and start bidirectional IPv4/IPv6 traffic")
	if err := startFlowsAndAwaitTx(t, env, defaultFlows); err != nil {
		return errors.Join(append(errs, err)...)
	}

	t.Log("Step 3 - Verify 0% steady-state traffic loss")
	errs = append(errs, verifySteadyStateLoss(t, env, defaultFlows, noLossPct))

	port4Transitions := bgpEstablishedTransitions(t, env.dut, deviations.DefaultNetworkInstance(env.dut), defaultPort4Peers)
	port4Adjacencies := isisAdjacencyUpTimestamps(t, env.dut, env.dutPort4.Name())

	t.Logf("Step 4 - Administratively disable %s", env.lagName)
	setInterfaceEnabled(t, env.dut, env.lagName, false)

	t.Log("Steps 5-7 - Verify LAG down, LAG protocols down and Port-4 protocols stable")
	errs = append(errs, verifyLagDrained(t, env, port4Transitions, port4Adjacencies))

	t.Log("Step 8 - Verify 100% steady-state traffic loss across the LAG")
	errs = append(errs, verifySteadyStateLoss(t, env, defaultFlows, fullLossPct))
	return errors.Join(errs...)
}

// testAdminDownDrainReenable implements RT-5.18.2.
func testAdminDownDrainReenable(t *testing.T, env *testEnv) error {
	if err := ensureLagDrained(t, env); err != nil {
		return err
	}

	t.Logf("Step 1 - Re-enable %s", env.lagName)
	setInterfaceEnabled(t, env.dut, env.lagName, true)

	t.Log("Steps 2-5 - Verify LAG up, protocols re-established and installed prefixes restored")
	if err := verifyLagRestored(t, env); err != nil {
		return err
	}

	t.Log("Step 6 - Verify 0% steady-state traffic loss")
	if err := startFlowsAndAwaitTx(t, env, defaultFlows); err != nil {
		return err
	}
	return verifySteadyStateLoss(t, env, defaultFlows, noLossPct)
}

// testMemberLinksAdminDown implements RT-5.18.3.
func testMemberLinksAdminDown(t *testing.T, env *testEnv) error {
	if err := ensureLagEnabled(t, env); err != nil {
		return err
	}
	dni := deviations.DefaultNetworkInstance(env.dut)
	disabled := env.dutMembers[:disabledMemberCount]
	remaining := env.dutMembers[disabledMemberCount]
	var errs []error

	t.Log("Step 1 - Verify installed prefixes over the LAG")
	errs = append(errs, verifyBGPPrefixes(t, env.dut, dni, ateLagIntf.IPv4, ateLagIntf.IPv6, bgpRouteCount))

	t.Log("Step 2 - Start bidirectional IPv4/IPv6 traffic")
	if err := startFlowsAndAwaitTx(t, env, defaultFlows); err != nil {
		return errors.Join(append(errs, err)...)
	}
	errs = append(errs, verifySteadyStateLoss(t, env, defaultFlows, noLossPct))

	lagTransitions := bgpEstablishedTransitions(t, env.dut, dni, defaultLagPeers)
	lagAdjacencies := isisAdjacencyUpTimestamps(t, env.dut, env.lagName)

	t.Logf("Step 3 - Administratively disable member links %v", portNames(disabled))
	t.Cleanup(func() { setPortsEnabled(t, env.dut, env.dutMembers, true) })
	setPortsEnabled(t, env.dut, disabled, false)

	t.Logf("Step 4 - Verify %v are DOWN while %s stays UP via %s", portNames(disabled), env.lagName, remaining.Name())
	for _, p := range disabled {
		errs = append(errs, awaitValue(t, env.dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), intfTimeout, oc.Interface_OperStatus_DOWN, p.Name()+" oper-status", false))
	}
	errs = append(errs,
		awaitLACPMembersUp(t, env.dut, env.lagName, []*ondatra.Port{remaining}),
		checkValue(t, env.dut, gnmi.OC().Interface(env.lagName).OperStatus().State(), oc.Interface_OperStatus_UP, env.lagName+" oper-status"),
	)

	t.Log("Step 5 - Verify eBGP and IS-IS over the LAG remain up without flapping")
	errs = append(errs,
		verifyBGPStable(t, env.dut, dni, defaultLagPeers, lagTransitions),
		verifyISISStable(t, env.dut, env.lagName, lagAdjacencies),
	)

	t.Logf("Step 6 - Verify 0%% steady-state traffic loss over %s", remaining.Name())
	if len(idleFlowsAfterStart(t, env.otg, defaultFlows)) > 0 {
		t.Log("Flows stalled after member links disabled; restarting traffic...")
		if err := stopFlowsAndAwaitStopped(t, env, defaultFlows); err != nil {
			return err
		}
		if err := startFlowsAndAwaitTx(t, env, defaultFlows); err != nil {
			return err
		}
	}
	errs = append(errs, verifySteadyStateLoss(t, env, defaultFlows, noLossPct))

	t.Logf("Step 7 - Re-enable %v and verify traffic across all member links", portNames(disabled))
	setPortsEnabled(t, env.dut, disabled, true)
	for _, p := range disabled {
		errs = append(errs, awaitValue(t, env.dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), intfTimeout, oc.Interface_OperStatus_UP, p.Name()+" oper-status", false))
	}
	errs = append(errs,
		awaitLACPMembersUp(t, env.dut, env.lagName, env.dutMembers),
		runFlowsAndVerifyDistribution(t, env, defaultFlows, nil),
	)
	return errors.Join(errs...)
}

// testRedundantAdminDown implements RT-5.18.4.
func testRedundantAdminDown(t *testing.T, env *testEnv) error {
	if err := ensureLagEnabled(t, env); err != nil {
		return err
	}
	lagEnabled := gnmi.OC().Interface(env.lagName).Enabled()
	if err := startFlowsAndAwaitTx(t, env, defaultFlows); err != nil {
		return err
	}
	var errs []error

	t.Logf("Step 1 - Administratively disable %s", env.lagName)
	t.Cleanup(func() { setInterfaceEnabled(t, env.dut, env.lagName, true) })
	setInterfaceEnabled(t, env.dut, env.lagName, false)

	t.Log("Step 2 - Verify LAG admin/oper DOWN and 100% traffic loss")
	errs = append(errs,
		awaitInterfaceStatus(t, env.dut, env.lagName, oc.Interface_AdminStatus_DOWN, oc.Interface_OperStatus_DOWN),
		verifySteadyStateLoss(t, env, defaultFlows, fullLossPct),
	)

	t.Log("Steps 3-4 - Send a redundant admin-down gNMI.Set and verify it is accepted")
	if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
		gnmi.Replace(t, env.dut, lagEnabled.Config(), false)
	}); errMsg != nil {
		errs = append(errs, fmt.Errorf("redundant gNMI.Set REPLACE of %s enabled=false returned an error: %s", env.lagName, *errMsg))
	}

	t.Log("Step 5 - Verify LAG stays admin/oper DOWN with 100% traffic loss")
	errs = append(errs,
		checkValue(t, env.dut, gnmi.OC().Interface(env.lagName).AdminStatus().State(), oc.Interface_AdminStatus_DOWN, env.lagName+" admin-status after redundant Set"),
		checkValue(t, env.dut, gnmi.OC().Interface(env.lagName).OperStatus().State(), oc.Interface_OperStatus_DOWN, env.lagName+" oper-status after redundant Set"),
		verifySteadyStateLoss(t, env, defaultFlows, fullLossPct),
	)

	t.Logf("Step 6 - Restore %s and verify protocols and 0%% traffic loss", env.lagName)
	setInterfaceEnabled(t, env.dut, env.lagName, true)
	if err := verifyLagRestored(t, env); err != nil {
		return errors.Join(append(errs, err)...)
	}
	waitForAllATENeighbors(t, env)
	errs = append(errs, verifySteadyStateLoss(t, env, defaultFlows, noLossPct))
	return errors.Join(errs...)
}

// testUplinkMemberRetentionB2Core implements RT-5.18.5.
func testUplinkMemberRetentionB2Core(t *testing.T, env *testEnv) error {
	if err := ensureLagEnabled(t, env); err != nil {
		return err
	}

	t.Log("Steps 1-4 - Configure B2_CORE VRF, VLAN 100 subinterfaces, LAG member retention and eBGP")
	if err := ensureB2Core(t, env); err != nil {
		return err
	}
	var errs []error

	t.Log("Step 5 - Verify member ports retain LAG membership and LACP participation")
	errs = append(errs, verifyLagMemberRetention(t, env))

	t.Log("Step 6 - Verify B2_CORE eBGP sessions and installed prefixes")
	errs = append(errs, verifyB2CoreControlPlane(t, env))

	t.Log("Steps 7-8 - Resolve ARP/ND, start B2_CORE traffic and verify 0% loss and egress distribution across all members")
	errs = append(errs, runFlowsAndVerifyDistribution(t, env, b2CoreFlows, nil))
	return errors.Join(errs...)
}

// testDifferentialMemberDrain implements RT-5.18.6.
func testDifferentialMemberDrain(t *testing.T, env *testEnv) error {
	if err := ensureLagEnabled(t, env); err != nil {
		return err
	}
	if err := ensureB2Core(t, env); err != nil {
		return err
	}
	if err := errors.Join(verifyLagMemberRetention(t, env), verifyB2CoreControlPlane(t, env)); err != nil {
		return err
	}

	drained := env.dutMembers[0]
	retained := env.dutMembers[1:]
	b2Transitions := bgpEstablishedTransitions(t, env.dut, b2CoreNI, b2CoreLagPeers)
	var errs []error

	t.Logf("Step 1 - Drain %s with forwarding-viable=false and retain %v", drained.Name(), portNames(retained))
	viable := map[string]bool{drained.Name(): false}
	t.Cleanup(func() { setForwardingViable(t, env, map[string]bool{drained.Name(): true}) })
	setForwardingViable(t, env, viable)

	t.Log("Step 2 - Verify differential member state")
	errs = append(errs, awaitValue(t, env.dut, gnmi.OC().Interface(drained.Name()).ForwardingViable().State(), intfTimeout, false, drained.Name()+" forwarding-viable", false))
	for _, p := range retained {
		errs = append(errs, awaitValue(t, env.dut, gnmi.OC().Interface(p.Name()).ForwardingViable().State(), intfTimeout, true, p.Name()+" forwarding-viable", deviations.MissingValueForDefaults(env.dut)))
	}
	for _, p := range env.dutMembers {
		errs = append(errs, awaitValue(t, env.dut, gnmi.OC().Interface(p.Name()).Ethernet().AggregateId().State(), intfTimeout, env.lagName, p.Name()+" aggregate-id", false))
	}

	t.Log("Step 3 - Verify B2_CORE eBGP over the LAG remains ESTABLISHED without flapping")
	errs = append(errs, verifyBGPStable(t, env.dut, b2CoreNI, b2CoreLagPeers, b2Transitions))

	t.Logf("Step 4 - Verify 0%% loss and no egress traffic on %s", drained.Name())
	errs = append(errs, runFlowsAndVerifyDistribution(t, env, b2CoreFlows, drained))

	t.Logf("Step 5 - Restore %s to forwarding-viable=true and verify recovery", drained.Name())
	setForwardingViable(t, env, map[string]bool{drained.Name(): true})
	errs = append(errs,
		awaitValue(t, env.dut, gnmi.OC().Interface(drained.Name()).ForwardingViable().State(), intfTimeout, true, drained.Name()+" forwarding-viable", deviations.MissingValueForDefaults(env.dut)),
		runFlowsAndVerifyDistribution(t, env, b2CoreFlows, nil),
	)
	return errors.Join(errs...)
}

func configureDUT(t *testing.T, env *testEnv) {
	t.Helper()
	dut := env.dut
	dni := deviations.DefaultNetworkInstance(dut)

	lacpActive := oc.Lacp_LacpActivityType_ACTIVE
	lacpFast := oc.Lacp_LacpPeriodType_FAST
	aggData := &cfgplugins.DUTAggData{
		Attributes:   dutLag,
		LagName:      env.lagName,
		OndatraPorts: env.dutMembers,
		AggType:      oc.IfAggregate_AggregationType_LACP,
		LacpParams:   &cfgplugins.LACPParams{Activity: &lacpActive, Period: &lacpFast},
	}

	intfBatch := &gnmi.SetBatch{}
	cfgplugins.NewAggregateInterface(t, dut, intfBatch, aggData)
	gnmi.BatchUpdate(intfBatch, gnmi.OC().Interface(env.lagName).Aggregation().MinLinks().Config(), uint16(lagMinLinks))

	p4 := dutPort4.NewOCInterface(env.dutPort4.Name(), dut)
	p4.Enabled = ygot.Bool(true)
	gnmi.BatchReplace(intfBatch, gnmi.OC().Interface(env.dutPort4.Name()).Config(), p4)
	if _, err := cfgplugins.ConfigureBGPRoutePolicy(t, intfBatch, cfgplugins.BGPPolicyConfig{PolicyName: policyName, StatementID: policyStatementID}); err != nil {
		t.Fatalf("Failed to build routing policy %s: %v", policyName, err)
	}
	intfBatch.Set(t, dut)

	defaultNI := &cfgplugins.NetworkInstanceParams{Default: true}
	niBatch := &gnmi.SetBatch{}
	cfgplugins.NewNetworkInstance(t, dut, niBatch, defaultNI)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		for _, intf := range []string{env.lagName, env.dutPort4.Name()} {
			cfgplugins.AssignInterfaceToNetworkInstance(t, niBatch, dut, intf, defaultNI, defaultSubIdx)
		}
	}
	niBatch.Set(t, dut)

	if deviations.ExplicitPortSpeed(dut) {
		for _, p := range append(append([]*ondatra.Port{}, env.dutMembers...), env.dutPort4) {
			fptest.SetPortSpeed(t, p)
		}
	}

	protoBatch := &gnmi.SetBatch{}
	cfgplugins.NewISIS(t, dut, &cfgplugins.ISISGlobalParams{
		DUTArea:             isisAreaDUT,
		DUTSysID:            isisSysIDDUT,
		NetworkInstanceName: dni,
		ISISInterfaceNames:  []string{env.lagName, env.dutPort4.Name()},
		HelloMultiplier:     isisHelloMultiplier,
		HelloInterval:       isisHelloInterval,
	}, protoBatch)
	bgp := cfgplugins.ConfigureDUTBGP(t, dut, protoBatch, cfgplugins.BGPConfig{DutAS: dutAS, RouterID: dutRouterID})
	if deviations.BgpAfiSafiInDefaultNiBeforeOtherNi(dut) {
		// Required before IPv4/IPv6 unicast can be enabled in the B2_CORE L3VRF.
		cfgplugins.ConfigureGlobal(bgp.GetBgp(), dut,
			cfgplugins.WithGlobalAfiSafiEnabled(oc.BgpTypes_AFI_SAFI_TYPE_L3VPN_IPV4_UNICAST, true),
			cfgplugins.WithGlobalAfiSafiEnabled(oc.BgpTypes_AFI_SAFI_TYPE_L3VPN_IPV6_UNICAST, true),
		)
	}
	policy := policyName
	for _, nbr := range []cfgplugins.BGPNeighborConfig{
		{AteAS: ateLagAS, PortName: lagPeerGroupPrefix, NeighborIPv4: ateLagIntf.IPv4, NeighborIPv6: ateLagIntf.IPv6, PolicyName: &policy},
		{AteAS: atePort4AS, PortName: port4ID, NeighborIPv4: atePort4Intf.IPv4, NeighborIPv6: atePort4Intf.IPv6, PolicyName: &policy},
	} {
		cfgplugins.AppendBGPNeighbor(t, dut, protoBatch, bgp.GetBgp(), nbr)
	}
	protoBatch.Set(t, dut)
}

func configureDUTB2Core(t *testing.T, env *testEnv) {
	t.Helper()
	dut := env.dut

	niBatch := &gnmi.SetBatch{}
	b2Core := &cfgplugins.NetworkInstanceParams{Name: b2CoreNI}
	cfgplugins.NewNetworkInstance(t, dut, niBatch, b2Core)
	if deviations.BgpAfiSafiInDefaultNiBeforeOtherNi(dut) {
		gnmi.BatchUpdate(niBatch, gnmi.OC().NetworkInstance(b2CoreNI).RouteDistinguisher().Config(), fmt.Sprintf("%d:%d", dutAS, vlanID))
	}
	for _, intf := range []string{env.lagName, env.dutPort4.Name()} {
		id := fmt.Sprintf("%s.%d", intf, vlanID)
		gnmi.BatchUpdate(niBatch, gnmi.OC().NetworkInstance(b2CoreNI).Interface(id).Config(), &oc.NetworkInstance_Interface{
			Id:           ygot.String(id),
			Interface:    ygot.String(intf),
			Subinterface: ygot.Uint32(vlanID),
		})
	}

	setSubinterfaces := func() {
		subifBatch := &gnmi.SetBatch{}
		addVLANSubinterface(dut, subifBatch, env.lagName, &dutLagVlan)
		addVLANSubinterface(dut, subifBatch, env.dutPort4.Name(), &dutPort4Vlan)
		subifBatch.Set(t, dut)
	}
	if deviations.InterfaceConfigVRFBeforeAddress(dut) {
		niBatch.Set(t, dut)
		setSubinterfaces()
	} else {
		setSubinterfaces()
		niBatch.Set(t, dut)
	}

	lagBatch := &gnmi.SetBatch{}
	gnmi.BatchReplace(lagBatch, gnmi.OC().Interface(env.lagName).Aggregation().LagType().Config(), oc.IfAggregate_AggregationType_LACP)
	gnmi.BatchReplace(lagBatch, gnmi.OC().Lacp().Interface(env.lagName).LacpMode().Config(), oc.Lacp_LacpActivityType_ACTIVE)
	gnmi.BatchReplace(lagBatch, gnmi.OC().Lacp().Interface(env.lagName).Interval().Config(), oc.Lacp_LacpPeriodType_FAST)
	for _, p := range env.dutMembers {
		gnmi.BatchReplace(lagBatch, gnmi.OC().Interface(p.Name()).Enabled().Config(), true)
		gnmi.BatchReplace(lagBatch, gnmi.OC().Interface(p.Name()).Ethernet().AggregateId().Config(), env.lagName)
		gnmi.BatchReplace(lagBatch, gnmi.OC().Interface(p.Name()).ForwardingViable().Config(), true)
	}
	bgp := buildBGPProtocol(dut, dutB2CoreRouterID, []bgpNeighbor{
		{address: ateLagVlanIntf.IPv4, localAddress: dutLagVlan.IPv4Address.String(), peerAS: ateLagAS, isV4: true},
		{address: ateLagVlanIntf.IPv6, localAddress: dutLagVlan.IPv6Address.String(), peerAS: ateLagAS},
		{address: atePort4VlanIntf.IPv4, localAddress: dutPort4Vlan.IPv4Address.String(), peerAS: atePort4AS, isV4: true},
		{address: atePort4VlanIntf.IPv6, localAddress: dutPort4Vlan.IPv6Address.String(), peerAS: atePort4AS},
	})
	gnmi.BatchReplace(lagBatch, bgpProtocolPath(b2CoreNI).Config(), bgp)
	lagBatch.Set(t, dut)
}

func addVLANSubinterface(dut *ondatra.DUTDevice, b *gnmi.SetBatch, intf string, s *cfgplugins.DUTSubInterfaceData) {
	sub := &oc.Interface_Subinterface{Index: ygot.Uint32(uint32(s.VlanID))}
	sub.Enabled = ygot.Bool(true)
	cfgplugins.ConfigureVLAN(sub, dut, uint16(s.VlanID))

	v4 := sub.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		v4.Enabled = ygot.Bool(true)
	}
	v4.GetOrCreateAddress(s.IPv4Address.String()).PrefixLength = ygot.Uint8(uint8(s.IPv4PrefixLen))

	v6 := sub.GetOrCreateIpv6()
	v6.Enabled = ygot.Bool(true)
	v6.GetOrCreateAddress(s.IPv6Address.String()).PrefixLength = ygot.Uint8(uint8(s.IPv6PrefixLen))

	gnmi.BatchReplace(b, gnmi.OC().Interface(intf).Subinterface(uint32(s.VlanID)).Config(), sub)
}

func buildBGPProtocol(dut *ondatra.DUTDevice, routerID string, nbrs []bgpNeighbor) *oc.NetworkInstance_Protocol {
	proto := &oc.NetworkInstance_Protocol{
		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
		Name:       ygot.String(bgpName),
	}
	bgp := proto.GetOrCreateBgp()
	cfgplugins.ConfigureGlobal(bgp, dut,
		cfgplugins.WithAS(dutAS),
		cfgplugins.WithRouterID(routerID),
		cfgplugins.WithGlobalAfiSafiEnabled(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, true),
		cfgplugins.WithGlobalAfiSafiEnabled(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, true),
	)

	for _, n := range nbrs {
		nbr := bgp.GetOrCreateNeighbor(n.address)
		nbr.PeerAs = ygot.Uint32(n.peerAS)
		nbr.Enabled = ygot.Bool(true)
		nbr.GetOrCreateTransport().LocalAddress = ygot.String(n.localAddress)
		cfgplugins.ConfigurePeer(nbr, dut,
			cfgplugins.WithPeerAfiSafiEnabled(n.isV4, policyName, policyName, false),
			cfgplugins.ApplyPeerPerAfiSafiRoutingPolicy(n.isV4, policyName, policyName, false),
		)
	}
	return proto
}

func bgpProtocolPath(ni string) *networkinstance.NetworkInstance_ProtocolPath {
	return gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName)
}

func isisProtocolPath(dut *ondatra.DUTDevice) *networkinstance.NetworkInstance_ProtocolPath {
	dni := deviations.DefaultNetworkInstance(dut)
	return gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, dni)
}

func setInterfaceEnabled(t *testing.T, dut *ondatra.DUTDevice, intf string, enabled bool) {
	t.Helper()
	gnmi.Replace(t, dut, gnmi.OC().Interface(intf).Enabled().Config(), enabled)
}

func setPortsEnabled(t *testing.T, dut *ondatra.DUTDevice, ports []*ondatra.Port, enabled bool) {
	t.Helper()
	b := &gnmi.SetBatch{}
	for _, p := range ports {
		gnmi.BatchReplace(b, gnmi.OC().Interface(p.Name()).Enabled().Config(), enabled)
	}
	b.Set(t, dut)
}

func setForwardingViable(t *testing.T, env *testEnv, viable map[string]bool) {
	t.Helper()
	b := &gnmi.SetBatch{}
	for portName, isViable := range viable {
		gnmi.BatchReplace(b, gnmi.OC().Interface(portName).ForwardingViable().Config(), isViable)
	}
	b.Set(t, env.dut)
}

func cleanupTestbed(t *testing.T, env *testEnv) {
	t.Helper()

	dni := deviations.DefaultNetworkInstance(env.dut)
	p4 := gnmi.OC().Interface(env.dutPort4.Name())
	groups := []struct {
		desc    string
		cleanup func(t testing.TB, b *gnmi.SetBatch) bool
	}{
		{"restoring admin state on DUT ports", func(_ testing.TB, b *gnmi.SetBatch) bool {
			for _, p := range append(append([]*ondatra.Port{}, env.dutMembers...), env.dutPort4) {
				gnmi.BatchReplace(b, gnmi.OC().Interface(p.Name()).Enabled().Config(), true)
			}
			return true
		}},
		{"B2_CORE network instance", func(_ testing.TB, b *gnmi.SetBatch) bool {
			gnmi.BatchDelete(b, gnmi.OC().NetworkInstance(b2CoreNI).Config())
			return true
		}},
		{"BGP and IS-IS protocols", func(_ testing.TB, b *gnmi.SetBatch) bool {
			gnmi.BatchDelete(b, bgpProtocolPath(dni).Config())
			gnmi.BatchDelete(b, isisProtocolPath(env.dut).Config())
			return true
		}},
		{"default network instance interface bindings", func(t testing.TB, b *gnmi.SetBatch) bool {
			if !deviations.ExplicitInterfaceInDefaultVRF(env.dut) {
				return false
			}
			queued := false
			for _, v := range gnmi.LookupAll(t, env.dut, gnmi.OC().NetworkInstance(dni).InterfaceAny().Config()) {
				if intf, ok := v.Val(); ok && (intf.GetInterface() == env.lagName || intf.GetInterface() == env.dutPort4.Name()) {
					gnmi.BatchDelete(b, gnmi.OC().NetworkInstance(dni).Interface(intf.GetId()).Config())
					queued = true
				}
			}
			return queued
		}},
		{"routing policy", func(_ testing.TB, b *gnmi.SetBatch) bool {
			gnmi.BatchDelete(b, gnmi.OC().RoutingPolicy().PolicyDefinition(policyName).Config())
			return true
		}},
		{"Port-4 configuration", func(_ testing.TB, b *gnmi.SetBatch) bool {
			gnmi.BatchDelete(b, p4.Description().Config())
			gnmi.BatchDelete(b, p4.Subinterface(vlanID).Config())
			gnmi.BatchDelete(b, p4.Subinterface(defaultSubIdx).Config())
			return true
		}},
		{"LAG membership", func(_ testing.TB, b *gnmi.SetBatch) bool {
			for _, p := range env.dutMembers {
				member := gnmi.OC().Interface(p.Name())
				gnmi.BatchDelete(b, member.Ethernet().AggregateId().Config())
				gnmi.BatchDelete(b, member.ForwardingViable().Config())
				gnmi.BatchDelete(b, member.Description().Config())
			}
			return true
		}},
		{"LAG and LACP", func(_ testing.TB, b *gnmi.SetBatch) bool {
			gnmi.BatchDelete(b, gnmi.OC().Lacp().Interface(env.lagName).Config())
			gnmi.BatchDelete(b, gnmi.OC().Interface(env.lagName).Config())
			return true
		}},
	}

	errMsg := testt.CaptureFatal(t, func(t testing.TB) {
		cleanupBatch := &gnmi.SetBatch{}
		for _, group := range groups {
			group.cleanup(t, cleanupBatch)
		}
		cleanupBatch.Set(t, env.dut)
	})
	if errMsg == nil {
		t.Log("Cleanup - removed test configuration from the DUT")
		return
	}
	// Retry per group so that one rejected change does not leave the rest of the configuration behind.
	var failures []string
	runCleanupStep := func(desc string, fn func(t testing.TB)) {
		t.Logf("Cleanup - %s", desc)
		if errMsg := testt.CaptureFatal(t, fn); errMsg != nil {
			failures = append(failures, fmt.Sprintf("%s: %s", desc, *errMsg))
		}
	}
	t.Logf("Cleanup - single request failed, retrying per group: %s", *errMsg)
	for _, group := range groups {
		runCleanupStep(group.desc, func(t testing.TB) {
			cleanupBatch := &gnmi.SetBatch{}
			if group.cleanup(t, cleanupBatch) {
				cleanupBatch.Set(t, env.dut)
			}
		})
	}
	if len(failures) > 0 {
		t.Fatalf("Cleanup failed:\n%s", strings.Join(failures, "\n"))
	}
}

func configureATE(t *testing.T, env *testEnv, withB2Core bool) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()

	lagIntfs := []*otgconfighelpers.InterfaceProperties{ateLagIntf}
	port4Intfs := []*otgconfighelpers.InterfaceProperties{atePort4Intf}
	if withB2Core {
		lagIntfs = append(lagIntfs, ateLagVlanIntf)
		port4Intfs = append(port4Intfs, atePort4VlanIntf)
	}

	otgconfighelpers.ConfigureNetworkInterface(t, top, env.ate, &otgconfighelpers.Port{
		Name:        ateLagName,
		AggMAC:      ateLagMAC,
		Interfaces:  lagIntfs,
		MemberPorts: []string{port1ID, port2ID, port3ID},
		LagID:       ateLagID,
		IsLag:       true,
	})
	otgconfighelpers.ConfigureNetworkInterface(t, top, env.ate, &otgconfighelpers.Port{
		Name:       port4ID,
		Interfaces: port4Intfs,
	})

	lagIsis := addOTGISIS(t, otgDevice(t, top, ateLagIntf), ateLagIntf, isisSysIDATELag)
	otgconfighelpers.AddISISRoutesV4(lagIsis, ateLagIntf.Name+".isis.v4", isisMetric, isisV4Route, routeV4PrefixLen, isisRouteCount)
	otgconfighelpers.AddISISRoutesV6(lagIsis, ateLagIntf.Name+".isis.v6", isisMetric, isisV6Route, routeV6PrefixLen, isisRouteCount)
	addOTGISIS(t, otgDevice(t, top, atePort4Intf), atePort4Intf, isisSysIDATEP4)

	addOTGBGP(t, top, ateLagIntf, ateLagAS, bgpV4RouteStart, bgpV6RouteStart, bgpRouteCount)
	addOTGBGP(t, top, atePort4Intf, atePort4AS, transitV4Route, transitV6Route, transitRouteCount)
	if withB2Core {
		addOTGBGP(t, top, ateLagVlanIntf, ateLagAS, b2CoreV4Route, b2CoreV6Route, b2CoreRouteCount)
		addOTGBGP(t, top, atePort4VlanIntf, atePort4AS, transitV4Route, transitV6Route, transitRouteCount)
	}

	for _, fs := range flowSpecs {
		if (fs.tx.Vlan != 0) != withB2Core {
			continue
		}
		addFlow(top, fs)
	}
	return top
}

func otgDevice(t *testing.T, top gosnappi.Config, intf *otgconfighelpers.InterfaceProperties) gosnappi.Device {
	t.Helper()
	name := intf.Name + otgDevSuffix
	for _, d := range top.Devices().Items() {
		if d.Name() == name {
			return d
		}
	}
	t.Fatalf("OTG device %s not found", name)
	return nil
}

func addOTGISIS(t *testing.T, dev gosnappi.Device, intf *otgconfighelpers.InterfaceProperties, sysID string) gosnappi.DeviceIsisRouter {
	t.Helper()
	isisRouter := otgconfighelpers.ConfigureISIS(t, dev, &otgconfighelpers.ISISAttrs{
		Name:          intf.Name + ".isis",
		SystemID:      sysID,
		Hostname:      intf.Name,
		AreaAddresses: []string{isisAreaATE},
		Interfaces: []*otgconfighelpers.ISISInterfaceAttrs{{
			Name:        intf.Name + ".isis.intf",
			EthName:     intf.Name + otgEthSuffix,
			NetworkType: gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT,
			LevelType:   gosnappi.IsisInterfaceLevelType.LEVEL_2,
			Metric:      isisMetric,
		}},
	})
	for _, isisIntf := range isisRouter.Interfaces().Items() {
		isisIntf.L2Settings().SetHelloInterval(isisHelloInterval).SetDeadInterval(isisDeadInterval)
	}
	return isisRouter
}

func addOTGBGP(t *testing.T, top gosnappi.Config, intf *otgconfighelpers.InterfaceProperties, as uint32, v4Route, v6Route string, routeCount uint32) {
	t.Helper()
	dev := otgDevice(t, top, intf)
	commonOpts := []otgconfighelpers.BGPPeerOption{
		otgconfighelpers.WithBGPASNumber(as),
		otgconfighelpers.WithBGPEBGP(),
		otgconfighelpers.WithBGPRouterID(intf.IPv4),
		otgconfighelpers.WithoutBGPGR(),
		otgconfighelpers.WithBGPTimers(bgpHoldTime, bgpKeepalive),
	}

	v4Peer := otgconfighelpers.AddBGPV4Peer(dev, intf.Name+otgIPv4Suffix, append(commonOpts,
		otgconfighelpers.WithBGPName(intf.Name+".bgp4.peer"),
		otgconfighelpers.WithBGPPeerAddress(intf.IPv4Gateway))...)
	otgconfighelpers.AddBGPV4Routes(v4Peer, intf.Name+".bgp4.routes", []string{fmt.Sprintf("%s/%d", v4Route, routeV4PrefixLen)},
		otgconfighelpers.WithBGPRouteAddressCount(routeCount),
		otgconfighelpers.WithBGPRouteNextHopIPv4(intf.IPv4))

	v6Peer := otgconfighelpers.AddBGPV6Peer(dev, intf.Name+otgIPv6Suffix, append(commonOpts,
		otgconfighelpers.WithBGPName(intf.Name+".bgp6.peer"),
		otgconfighelpers.WithBGPPeerAddress(intf.IPv6Gateway))...)
	otgconfighelpers.AddBGPV6Routes(v6Peer, intf.Name+".bgp6.routes", []string{fmt.Sprintf("%s/%d", v6Route, routeV6PrefixLen)},
		otgconfighelpers.WithBGPRouteAddressCount(routeCount),
		otgconfighelpers.WithBGPRouteNextHopIPv6(intf.IPv6))
}

func addFlow(top gosnappi.Config, fs flowSpec) {
	ipSuffix := otgIPv4Suffix
	if fs.ipv6 {
		ipSuffix = otgIPv6Suffix
	}
	flow := top.Flows().Add().SetName(fs.name)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{fs.tx.Name + ipSuffix}).SetRxNames([]string{fs.rx.Name + ipSuffix})
	flow.Size().SetFixed(frameSize)
	flow.Rate().SetPercentage(fs.ratePct)

	flow.Packet().Add().Ethernet().Src().SetValue(fs.tx.MAC)
	if fs.tx.Vlan != 0 {
		flow.Packet().Add().Vlan().Id().SetValue(fs.tx.Vlan)
	}
	if fs.ipv6 {
		ip := flow.Packet().Add().Ipv6()
		if fs.srcCount > 0 {
			ip.Src().Increment().SetStart(fs.src).SetStep(fs.srcStep).SetCount(fs.srcCount)
		} else {
			ip.Src().SetValue(fs.src)
		}
		if fs.dstCount > 0 {
			ip.Dst().Increment().SetStart(fs.dst).SetStep(fs.dstStep).SetCount(fs.dstCount)
		} else {
			ip.Dst().SetValue(fs.dst)
		}
	} else {
		ip := flow.Packet().Add().Ipv4()
		if fs.srcCount > 0 {
			ip.Src().Increment().SetStart(fs.src).SetStep(fs.srcStep).SetCount(fs.srcCount)
		} else {
			ip.Src().SetValue(fs.src)
		}
		if fs.dstCount > 0 {
			ip.Dst().Increment().SetStart(fs.dst).SetStep(fs.dstStep).SetCount(fs.dstCount)
		} else {
			ip.Dst().SetValue(fs.dst)
		}
	}
	udp := flow.Packet().Add().Udp()
	if fs.srcCount == 0 && fs.dstCount == 0 {
		udp.SrcPort().Increment().SetStart(udpSrcPortStart).SetCount(udpSrcPortCount)
	} else {
		udp.SrcPort().SetValue(udpSrcPortStart)
	}
	udp.DstPort().SetValue(udpDstPort)
}

func verifyDefaultControlPlaneUp(t *testing.T, env *testEnv) error {
	t.Helper()
	dni := deviations.DefaultNetworkInstance(env.dut)
	return errors.Join(
		verifyLagProperties(t, env),
		awaitBGPEstablished(t, env.dut, dni, append(append([]string{}, defaultLagPeers...), defaultPort4Peers...)),
		awaitISISAdjacencyUp(t, env.dut, env.lagName),
		awaitISISAdjacencyUp(t, env.dut, env.dutPort4.Name()),
		verifyBGPPrefixes(t, env.dut, dni, ateLagIntf.IPv4, ateLagIntf.IPv6, bgpRouteCount),
	)
}

func verifyLagDrained(t *testing.T, env *testEnv, port4Transitions map[string]uint64, port4Adjacencies map[string]time.Time) error {
	t.Helper()
	return errors.Join(
		awaitLagDown(t, env),
		verifyBGPStable(t, env.dut, deviations.DefaultNetworkInstance(env.dut), defaultPort4Peers, port4Transitions),
		verifyISISStable(t, env.dut, env.dutPort4.Name(), port4Adjacencies),
	)
}

func awaitLagDown(t *testing.T, env *testEnv) error {
	t.Helper()
	return errors.Join(
		awaitInterfaceStatus(t, env.dut, env.lagName, oc.Interface_AdminStatus_DOWN, oc.Interface_OperStatus_DOWN),
		awaitBGPNotEstablished(t, env.dut, deviations.DefaultNetworkInstance(env.dut), defaultLagPeers),
		awaitISISAdjacencyDown(t, env.dut, env.lagName),
	)
}

func verifyLagUp(t *testing.T, env *testEnv) error {
	t.Helper()
	dni := deviations.DefaultNetworkInstance(env.dut)
	return errors.Join(
		awaitInterfaceStatus(t, env.dut, env.lagName, oc.Interface_AdminStatus_UP, oc.Interface_OperStatus_UP),
		verifyLagProperties(t, env),
		awaitLACPMembersUp(t, env.dut, env.lagName, env.dutMembers),
		awaitBGPEstablished(t, env.dut, dni, defaultLagPeers),
		awaitISISAdjacencyUp(t, env.dut, env.lagName),
	)
}

func verifyLagRestored(t *testing.T, env *testEnv) error {
	t.Helper()
	if err := verifyLagUp(t, env); err != nil {
		return err
	}
	return verifyBGPPrefixes(t, env.dut, deviations.DefaultNetworkInstance(env.dut), ateLagIntf.IPv4, ateLagIntf.IPv6, bgpRouteCount)
}

func verifyLagProperties(t *testing.T, env *testEnv) error {
	t.Helper()
	return errors.Join(
		awaitValue(t, env.dut, gnmi.OC().Interface(env.lagName).Aggregation().LagType().State(), intfTimeout, oc.IfAggregate_AggregationType_LACP, env.lagName+" state lag-type", false),
		awaitValue(t, env.dut, gnmi.OC().Interface(env.lagName).Aggregation().MinLinks().State(), intfTimeout, uint16(lagMinLinks), env.lagName+" state min-links", false),
	)
}

func ensureLagEnabled(t *testing.T, env *testEnv) error {
	t.Helper()
	if admin, ok := gnmi.Lookup(t, env.dut, gnmi.OC().Interface(env.lagName).AdminStatus().State()).Val(); !ok || admin != oc.Interface_AdminStatus_UP {
		t.Logf("Precondition - enabling %s", env.lagName)
		setInterfaceEnabled(t, env.dut, env.lagName, true)
	}
	return verifyLagUp(t, env)
}

func ensureLagDrained(t *testing.T, env *testEnv) error {
	t.Helper()
	if admin, ok := gnmi.Lookup(t, env.dut, gnmi.OC().Interface(env.lagName).AdminStatus().State()).Val(); ok && admin == oc.Interface_AdminStatus_DOWN {
		return nil
	}
	t.Logf("Precondition - disabling %s", env.lagName)
	setInterfaceEnabled(t, env.dut, env.lagName, false)
	return awaitLagDown(t, env)
}

// ensureB2Core configures the B2_CORE topology once; the VLAN 100 ATE side is added only after the DUT side exists.
func ensureB2Core(t *testing.T, env *testEnv) error {
	t.Helper()
	if env.b2CoreConfigured {
		return nil
	}
	configureDUTB2Core(t, env)

	env.otgConfig = configureATE(t, env, true)
	env.otg.PushConfig(t, env.otgConfig)
	env.otg.StartProtocols(t)
	env.b2CoreConfigured = true
	return awaitLACPMembersUp(t, env.dut, env.lagName, env.dutMembers)
}

func verifyB2CoreControlPlane(t *testing.T, env *testEnv) error {
	t.Helper()
	return errors.Join(
		awaitBGPEstablished(t, env.dut, b2CoreNI, append(append([]string{}, b2CoreLagPeers...), b2CorePort4Peers...)),
		verifyBGPPrefixes(t, env.dut, b2CoreNI, ateLagVlanIntf.IPv4, ateLagVlanIntf.IPv6, b2CoreRouteCount),
	)
}

func verifyLagMemberRetention(t *testing.T, env *testEnv) error {
	t.Helper()
	errs := []error{awaitValue(t, env.dut, gnmi.OC().Interface(env.lagName).OperStatus().State(), intfTimeout, oc.Interface_OperStatus_UP, env.lagName+" oper-status", false)}
	for _, p := range env.dutMembers {
		intf := gnmi.OC().Interface(p.Name())
		member := gnmi.OC().Lacp().Interface(env.lagName).Member(p.Name())
		memberErr := errors.Join(
			awaitValue(t, env.dut, intf.Ethernet().AggregateId().State(), intfTimeout, env.lagName, p.Name()+" aggregate-id", false),
			awaitValue(t, env.dut, gnmi.OC().Interface(p.Name()).ForwardingViable().State(), intfTimeout, true, p.Name()+" forwarding-viable", deviations.MissingValueForDefaults(env.dut)),
			awaitValue(t, env.dut, intf.OperStatus().State(), intfTimeout, oc.Interface_OperStatus_UP, p.Name()+" oper-status", false),
			awaitValue(t, env.dut, member.Activity().State(), lacpTimeout, oc.Lacp_LacpActivityType_ACTIVE, p.Name()+" LACP activity", false),
			awaitValue(t, env.dut, member.Aggregatable().State(), lacpTimeout, true, p.Name()+" LACP aggregatable", false),
			awaitLACPMembersUp(t, env.dut, env.lagName, []*ondatra.Port{p}),
		)
		if memberErr == nil {
			t.Logf("%s retained in %s and LACP collecting/distributing/IN_SYNC", p.Name(), env.lagName)
		}
		errs = append(errs, memberErr)
	}
	return errors.Join(errs...)
}

func awaitValue[T comparable](t *testing.T, dut *ondatra.DUTDevice, q ygnmi.SingletonQuery[T], timeout time.Duration, want T, desc string, allowMissing bool) error {
	t.Helper()
	got, ok := gnmi.Watch(t, dut, q, timeout, func(v *ygnmi.Value[T]) bool {
		val, present := v.Val()
		return present && val == want
	}).Await(t)
	if ok {
		return nil
	}
	if allowMissing && (got == nil || !got.IsPresent()) {
		return nil
	}
	last := "not reported"
	if got != nil {
		if val, present := got.Val(); present {
			last = fmt.Sprint(val)
		}
	}
	return fmt.Errorf("%s: got %s, want %v within %v", desc, last, want, timeout)
}

func checkValue[T comparable](t *testing.T, dut *ondatra.DUTDevice, q ygnmi.SingletonQuery[T], want T, desc string) error {
	t.Helper()
	got, present := gnmi.Lookup(t, dut, q).Val()
	if !present || got != want {
		return fmt.Errorf("%s: got %v (present=%v), want %v", desc, got, present, want)
	}
	return nil
}

func awaitInterfaceStatus(t *testing.T, dut *ondatra.DUTDevice, intf string, admin oc.E_Interface_AdminStatus, oper oc.E_Interface_OperStatus) error {
	t.Helper()
	errs := []error{
		awaitValue(t, dut, gnmi.OC().Interface(intf).AdminStatus().State(), intfTimeout, admin, intf+" admin-status", false),
		awaitValue(t, dut, gnmi.OC().Interface(intf).OperStatus().State(), intfTimeout, oper, intf+" oper-status", false),
	}
	wantEnabled := admin == oc.Interface_AdminStatus_UP
	if got, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(intf).Enabled().State()).Val(); ok && got != wantEnabled {
		errs = append(errs, fmt.Errorf("%s enabled state: got %v, want %v", intf, got, wantEnabled))
	}
	err := errors.Join(errs...)
	if err == nil {
		t.Logf("%s admin-status=%v oper-status=%v (enabled=%v)", intf, admin, oper, wantEnabled)
	}
	return err
}

func awaitLACPMembersUp(t *testing.T, dut *ondatra.DUTDevice, lag string, ports []*ondatra.Port) error {
	t.Helper()
	var errs []error
	for _, p := range ports {
		member := gnmi.OC().Lacp().Interface(lag).Member(p.Name())
		errs = append(errs,
			awaitValue(t, dut, member.Collecting().State(), lacpTimeout, true, p.Name()+" LACP collecting", false),
			awaitValue(t, dut, member.Distributing().State(), lacpTimeout, true, p.Name()+" LACP distributing", false),
			awaitValue(t, dut, member.Synchronization().State(), lacpTimeout, oc.Lacp_LacpSynchronizationType_IN_SYNC, p.Name()+" LACP synchronization", false),
		)
	}
	return errors.Join(errs...)
}

func awaitBGPEstablished(t *testing.T, dut *ondatra.DUTDevice, ni string, peers []string) error {
	t.Helper()
	var errs []error
	for _, peer := range peers {
		if err := awaitValue(t, dut, bgpProtocolPath(ni).Bgp().Neighbor(peer).SessionState().State(), bgpTimeout, oc.Bgp_Neighbor_SessionState_ESTABLISHED, fmt.Sprintf("BGP neighbor %s in %s", peer, ni), false); err != nil {
			errs = append(errs, err)
			continue
		}
		t.Logf("BGP neighbor %s in %s is ESTABLISHED", peer, ni)
	}
	return errors.Join(errs...)
}

func awaitBGPNotEstablished(t *testing.T, dut *ondatra.DUTDevice, ni string, peers []string) error {
	t.Helper()
	var errs []error
	for _, peer := range peers {
		got, ok := gnmi.Watch(t, dut, bgpProtocolPath(ni).Bgp().Neighbor(peer).SessionState().State(), bgpTimeout, func(v *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			state, present := v.Val()
			return present && (state == oc.Bgp_Neighbor_SessionState_IDLE || state == oc.Bgp_Neighbor_SessionState_ACTIVE)
		}).Await(t)
		if !ok {
			errs = append(errs, fmt.Errorf("BGP neighbor %s in %s did not transition to IDLE/ACTIVE, last state: %v", peer, ni, got))
			continue
		}
		state, _ := got.Val()
		t.Logf("BGP neighbor %s in %s is %v", peer, ni, state)
	}
	return errors.Join(errs...)
}

func bgpEstablishedTransitions(t *testing.T, dut *ondatra.DUTDevice, ni string, peers []string) map[string]uint64 {
	t.Helper()
	transitions := make(map[string]uint64)
	for _, peer := range peers {
		if v, ok := gnmi.Lookup(t, dut, bgpProtocolPath(ni).Bgp().Neighbor(peer).EstablishedTransitions().State()).Val(); ok {
			transitions[peer] = v
		}
	}
	return transitions
}

func verifyBGPStable(t *testing.T, dut *ondatra.DUTDevice, ni string, peers []string, before map[string]uint64) error {
	t.Helper()
	var errs []error
	for _, peer := range peers {
		nbr := bgpProtocolPath(ni).Bgp().Neighbor(peer)
		if err := checkValue(t, dut, nbr.SessionState().State(), oc.Bgp_Neighbor_SessionState_ESTABLISHED, fmt.Sprintf("BGP neighbor %s in %s session-state", peer, ni)); err != nil {
			errs = append(errs, err)
			continue
		}
		want, ok := before[peer]
		if !ok {
			t.Logf("BGP neighbor %s in %s is ESTABLISHED (established-transitions not reported)", peer, ni)
			continue
		}
		if got, _ := gnmi.Lookup(t, dut, nbr.EstablishedTransitions().State()).Val(); got != want {
			errs = append(errs, fmt.Errorf("BGP neighbor %s in %s flapped: established-transitions got %d, want %d", peer, ni, got, want))
			continue
		}
		t.Logf("BGP neighbor %s in %s remained ESTABLISHED without flapping", peer, ni)
	}
	return errors.Join(errs...)
}

func verifyBGPPrefixes(t *testing.T, dut *ondatra.DUTDevice, ni, v4Peer, v6Peer string, want uint32) error {
	t.Helper()
	bgp := bgpProtocolPath(ni).Bgp()
	var errs []error
	for _, p := range []struct {
		peer    string
		afiSafi oc.E_BgpTypes_AFI_SAFI_TYPE
	}{
		{v4Peer, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST},
		{v6Peer, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST},
	} {
		peer, afiSafi := p.peer, p.afiSafi
		prefixes := bgp.Neighbor(peer).AfiSafi(afiSafi).Prefixes()
		var got *ygnmi.Value[uint32]
		var ok bool
		if ni != deviations.DefaultNetworkInstance(dut) && deviations.BgpNonDefaultVrfInstalledPrefixesUnsupported(dut) {
			t.Logf("BGP neighbor %s in non-default vrf %s %v installed prefixes not reported, skipping check", peer, ni, afiSafi)
		} else {
			got, ok = gnmi.Watch(t, dut, prefixes.Installed().State(), prefixTimeout, func(v *ygnmi.Value[uint32]) bool {
				n, present := v.Val()
				return present && n == want
			}).Await(t)
			if !ok {
				var installed uint32
				if got != nil {
					installed, _ = got.Val()
				}
				received, _ := gnmi.Lookup(t, dut, prefixes.Received().State()).Val()
				errs = append(errs, fmt.Errorf("BGP neighbor %s in %s %v prefixes after %v: installed %d, received %d, want %d", peer, ni, afiSafi, prefixTimeout, installed, received, want))
				continue
			}
			t.Logf("BGP neighbor %s in %s %v installed prefixes: %d", peer, ni, afiSafi, want)
		}

		got, ok = gnmi.Watch(t, dut, prefixes.Received().State(), bgpTimeout, func(v *ygnmi.Value[uint32]) bool {
			n, present := v.Val()
			return present && n == want
		}).Await(t)
		switch {
		case ok:
			t.Logf("BGP neighbor %s in %s %v received prefixes: %d", peer, ni, afiSafi, want)
		case got == nil || !got.IsPresent():
			t.Logf("BGP neighbor %s in %s %v received prefixes not reported, skipping check", peer, ni, afiSafi)
		default:
			n, _ := got.Val()
			errs = append(errs, fmt.Errorf("BGP neighbor %s in %s %v received prefixes: got %d, want %d", peer, ni, afiSafi, n, want))
		}
	}
	return errors.Join(errs...)
}

func isisAdjacencyPath(dut *ondatra.DUTDevice, intf string) *netinstisis.NetworkInstance_Protocol_Isis_Interface_Level_AdjacencyPathAny {
	return isisProtocolPath(dut).Isis().Interface(cfgplugins.InterfaceRefID(dut, intf)).Level(isisLevel).AdjacencyAny()
}

func isisAdjacencyStatePath(dut *ondatra.DUTDevice, intf string) ygnmi.WildcardQuery[oc.E_Isis_IsisInterfaceAdjState] {
	return isisAdjacencyPath(dut, intf).AdjacencyState().State()
}

func awaitISISAdjacencyUp(t *testing.T, dut *ondatra.DUTDevice, intf string) error {
	t.Helper()
	_, ok := gnmi.WatchAll(t, dut, isisAdjacencyStatePath(dut, intf), isisTimeout, func(v *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
		state, present := v.Val()
		return present && state == oc.Isis_IsisInterfaceAdjState_UP
	}).Await(t)
	if !ok {
		return fmt.Errorf("IS-IS L2 adjacency on %s is not UP within %v", intf, isisTimeout)
	}
	t.Logf("IS-IS L2 adjacency on %s is UP", intf)
	return nil
}

func isisAdjacencyUpTimestamps(t *testing.T, dut *ondatra.DUTDevice, intf string) map[string]time.Time {
	t.Helper()
	timestamps := make(map[string]time.Time)
	for _, v := range gnmi.LookupAll(t, dut, isisAdjacencyPath(dut, intf).State()) {
		adj, present := v.Val()
		if !present || adj.GetAdjacencyState() != oc.Isis_IsisInterfaceAdjState_UP || adj.GetUpTimestamp() == 0 {
			continue
		}
		timestamps[adj.GetSystemId()] = isisUpTime(adj.GetUpTimestamp())
	}
	return timestamps
}

func isisUpTime(ts uint64) time.Time {
	if ts > nsTimestampThreshold {
		return time.Unix(0, int64(ts))
	}
	return time.Unix(int64(ts), 0)
}

func verifyISISStable(t *testing.T, dut *ondatra.DUTDevice, intf string, before map[string]time.Time) error {
	t.Helper()
	if err := awaitISISAdjacencyUp(t, dut, intf); err != nil {
		return err
	}
	if len(before) == 0 {
		t.Logf("IS-IS adjacency up-timestamp not reported on %s, flap detection skipped", intf)
		return nil
	}
	after := isisAdjacencyUpTimestamps(t, dut, intf)
	var errs []error
	for sysID, want := range before {
		got, ok := after[sysID]
		if !ok || got.Sub(want).Abs() > isisUpTimestampTolerance {
			errs = append(errs, fmt.Errorf("IS-IS L2 adjacency to %s on %s flapped: up-timestamp got %v (present=%v), want %v", sysID, intf, got, ok, want))
			continue
		}
		t.Logf("IS-IS L2 adjacency to %s on %s remained UP without flapping", sysID, intf)
	}
	return errors.Join(errs...)
}

// awaitISISAdjacencyDown covers adjacencies reported DOWN as well as removed ones.
func awaitISISAdjacencyDown(t *testing.T, dut *ondatra.DUTDevice, intf string) error {
	t.Helper()
	anyUp := false
	for _, v := range gnmi.LookupAll(t, dut, isisAdjacencyStatePath(dut, intf)) {
		if state, present := v.Val(); present && state == oc.Isis_IsisInterfaceAdjState_UP {
			anyUp = true
		}
	}
	if !anyUp {
		t.Logf("IS-IS L2 adjacency on %s is DOWN or removed", intf)
		return nil
	}
	up := make(map[string]bool)
	_, ok := gnmi.WatchAll(t, dut, isisAdjacencyStatePath(dut, intf), isisTimeout, func(v *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
		state, present := v.Val()
		up[v.Path.String()] = present && state == oc.Isis_IsisInterfaceAdjState_UP
		for _, isUp := range up {
			if isUp {
				return false
			}
		}
		return true
	}).Await(t)
	if !ok {
		return fmt.Errorf("IS-IS L2 adjacency on %s is still UP after %v", intf, isisTimeout)
	}
	t.Logf("IS-IS L2 adjacency on %s is DOWN or removed", intf)
	return nil
}

func waitForAllATENeighbors(t *testing.T, env *testEnv) {
	t.Helper()
	otgutils.WaitForARP(t, env.otg, env.otgConfig, cfgplugins.IPv4)
	otgutils.WaitForARP(t, env.otg, env.otgConfig, cfgplugins.IPv6)
}

func logOTGState(t *testing.T, env *testEnv) {
	t.Helper()
	otgutils.LogPortMetrics(t, env.otg, env.otgConfig)
	otgutils.LogLAGMetrics(t, env.otg, env.otgConfig)
	otgutils.LogLACPMetrics(t, env.otg, env.otgConfig)
	otgutils.LogFlowMetrics(t, env.otg, env.otgConfig)
}

func startFlowsAndAwaitTx(t *testing.T, env *testEnv, flows []string) error {
	t.Helper()
	waitForAllATENeighbors(t, env)
	otg := env.otg
	for attempt := 1; attempt <= flowStartAttempts; attempt++ {
		otg.StartTraffic(t)
		idle := idleFlowsAfterStart(t, otg, flows)
		if len(idle) == 0 {
			return nil
		}
		t.Logf("Flows %v did not transmit within %v after start (attempt %d/%d)", idle, flowStartTimeout, attempt, flowStartAttempts)
		logOTGState(t, env)
		if err := stopFlowsAndAwaitStopped(t, env, flows); err != nil {
			return err
		}
	}
	return fmt.Errorf("flows did not start transmitting after %d start attempts", flowStartAttempts)
}

// idleFlowsAfterStart returns the flows whose transmit counter is still not advancing after flowStartTimeout.
func idleFlowsAfterStart(t *testing.T, otg *otg.OTG, flows []string) []string {
	t.Helper()
	// OTG resets flow counters on start, so progress is measured between samples of the watch, not against an earlier read.
	watchers := make([]*gnmi.Watcher[uint64], len(flows))
	for i, f := range flows {
		var last uint64
		seen := false
		watchers[i] = gnmi.Watch(t, otg, gnmi.OTG().Flow(f).Counters().OutPkts().State(), flowStartTimeout, func(v *ygnmi.Value[uint64]) bool {
			pkts, present := v.Val()
			if !present {
				return false
			}
			advanced := seen && pkts > last
			last, seen = pkts, true
			return advanced
		})
	}
	var idle []string
	for i, f := range flows {
		if _, ok := watchers[i].Await(t); !ok {
			idle = append(idle, f)
		}
	}
	return idle
}

func stopFlowsAndAwaitStopped(t *testing.T, env *testEnv, flows []string) error {
	t.Helper()
	env.otg.StopTraffic(t)
	var errs []error
	for _, flow := range flows {
		_, ok := gnmi.Watch(t, env.otg, gnmi.OTG().Flow(flow).Transmit().State(), flowStopTimeout, func(value *ygnmi.Value[bool]) bool {
			transmitting, present := value.Val()
			return present && !transmitting
		}).Await(t)
		if !ok {
			errs = append(errs, fmt.Errorf("flow %s did not report stopped within %v", flow, flowStopTimeout))
		}
	}
	return errors.Join(errs...)
}

func getFlowCounters(t *testing.T, otg *otg.OTG, flows []string) map[string]flowCounters {
	t.Helper()
	counters := make(map[string]flowCounters)
	for _, f := range flows {
		c := gnmi.Get(t, otg, gnmi.OTG().Flow(f).Counters().State())
		counters[f] = flowCounters{tx: c.GetOutPkts(), rx: c.GetInPkts()}
	}
	return counters
}

func countTrafficUpdates(t *testing.T, otg *otg.OTG, refFlow string, start uint64, n int, timeout time.Duration) int {
	t.Helper()
	last, updates := start, 0
	gnmi.Watch(t, otg, gnmi.OTG().Flow(refFlow).Counters().OutPkts().State(), timeout, func(v *ygnmi.Value[uint64]) bool {
		if pkts, present := v.Val(); present {
			switch {
			case pkts < last:
				t.Logf("Flow %s transmit counter reset from %d to %d; restarting observation window", refFlow, last, pkts)
				updates = 0
			case pkts > last:
				updates++
			}
			last = pkts
		}
		return updates >= n
	}).Await(t)
	return updates
}

func awaitTrafficUpdates(t *testing.T, otg *otg.OTG, refFlow string, start uint64, n int) error {
	t.Helper()
	if updates := countTrafficUpdates(t, otg, refFlow, start, n, trafficWindowTimeout); updates < n {
		return fmt.Errorf("flow %s transmitted counter advanced %d of %d times within %v", refFlow, updates, n, trafficWindowTimeout)
	}
	return nil
}

func verifySteadyStateLoss(t *testing.T, env *testEnv, flows []string, wantLossPct float64) error {
	t.Helper()
	start := time.Now()
	deadline := start.Add(trafficConvergenceTimeout)
	for attempt := 1; ; attempt++ {
		results, failures, err := measureLossWindow(t, env, flows, wantLossPct)
		if err != nil {
			logFlowMetricsFor(t, env, flows)
			return err
		}
		if len(failures) == 0 {
			logFlowMetricsFor(t, env, flows)
			for _, r := range results {
				t.Log(r)
			}
			t.Logf("Traffic converged to %.0f%% loss after %d window(s), %v", wantLossPct, attempt, time.Since(start).Round(time.Second))
			return nil
		}
		if time.Now().After(deadline) {
			logFlowMetricsFor(t, env, flows)
			return errors.Join(failures...)
		}
		t.Logf("Window %d has not converged to %.0f%% loss: %v", attempt, wantLossPct, failures)
	}
}

// measureLossWindow separates loss mismatches from an inability to measure traffic.
func measureLossWindow(t *testing.T, env *testEnv, flows []string, wantLossPct float64) (results []string, failures []error, err error) {
	t.Helper()
	refFlow := flows[0]
	if err := awaitTrafficUpdates(t, env.otg, refFlow, gnmi.Get(t, env.otg, gnmi.OTG().Flow(refFlow).Counters().OutPkts().State()), trafficSettleUpdates); err != nil {
		return nil, nil, err
	}
	before := getFlowCounters(t, env.otg, flows)
	if err := awaitTrafficUpdates(t, env.otg, refFlow, before[refFlow].tx, trafficWindowUpdates); err != nil {
		return nil, nil, err
	}
	after := getFlowCounters(t, env.otg, flows)

	for _, f := range flows {
		if after[f].tx < before[f].tx || after[f].rx < before[f].rx {
			failures = append(failures, fmt.Errorf("flow %s counters decreased during the measurement window: before %+v, after %+v", f, before[f], after[f]))
			continue
		}
		txDelta := after[f].tx - before[f].tx
		rxDelta := after[f].rx - before[f].rx

		if txDelta == 0 {
			switch {
			case rxDelta != 0:
				failures = append(failures, fmt.Errorf("flow %s: got %d packets received without any transmitted during the window", f, rxDelta))
			case wantLossPct == fullLossPct:
				results = append(results, fmt.Sprintf("Flow %s: no packets transmitted or received during the window", f))
			default:
				failures = append(failures, fmt.Errorf("flow %s: no packets transmitted during the measurement window", f))
			}
			continue
		}

		lossPct := (float64(txDelta) - float64(rxDelta)) * 100 / float64(txDelta)
		if math.Abs(lossPct-wantLossPct) > lossTolerancePct {
			failures = append(failures, fmt.Errorf("flow %s steady-state loss: got %.2f%% (tx %d, rx %d), want %.0f%%", f, lossPct, txDelta, rxDelta, wantLossPct))
			continue
		}
		results = append(results, fmt.Sprintf("Flow %s steady-state loss: %.2f%% (tx %d, rx %d)", f, lossPct, txDelta, rxDelta))
	}
	return results, failures, nil
}

func logFlowMetricsFor(t *testing.T, env *testEnv, flows []string) {
	t.Helper()
	cfg := gosnappi.NewConfig()
	for _, f := range flows {
		cfg.Flows().Add().SetName(f)
	}
	otgutils.LogFlowMetrics(t, env.otg, cfg)
}

// runFlowsAndVerifyDistribution runs flows, verifies 0% loss and that DUT
// egress on the LAG is spread across all members except the drained one.
func runFlowsAndVerifyDistribution(t *testing.T, env *testEnv, flows []string, drained *ondatra.Port) error {
	t.Helper()
	if err := stopFlowsAndAwaitStopped(t, env, flows); err != nil {
		return err
	}
	before, err := awaitStableMemberCounters(t, env.dut, env.dutMembers)
	if err != nil {
		return err
	}
	if err := startFlowsAndAwaitTx(t, env, flows); err != nil {
		return err
	}
	errs := []error{verifySteadyStateLoss(t, env, flows, noLossPct)}
	if err := stopFlowsAndAwaitStopped(t, env, flows); err != nil {
		return errors.Join(append(errs, err)...)
	}
	after, err := awaitMemberEgress(t, env.dut, env.dutMembers, before, lagEgressRxPkts(t, env, flows))
	if err != nil {
		return errors.Join(append(errs, err)...)
	}

	var total uint64
	deltas := make(map[string]uint64)
	for _, p := range env.dutMembers {
		deltas[p.Name()] = after[p.Name()].outPkts - before[p.Name()].outPkts
		total += deltas[p.Name()]
	}
	if total == 0 {
		return errors.Join(append(errs, fmt.Errorf("no egress unicast packets counted on %s members %v", env.lagName, portNames(env.dutMembers)))...)
	}
	for _, p := range env.dutMembers {
		share := float64(deltas[p.Name()]) * 100 / float64(total)
		switch {
		case drained != nil && p.Name() == drained.Name():
			if deltas[p.Name()] > 0 {
				errs = append(errs, fmt.Errorf("drained member %s transmitted %d egress unicast packets, want 0", p.Name(), deltas[p.Name()]))
			}
		case share < minMemberSharePct:
			errs = append(errs, fmt.Errorf("member %s egress share: got %.2f%% (%d pkts), want >= %.0f%%", p.Name(), share, deltas[p.Name()], minMemberSharePct))
		}
		t.Logf("Member %s: out-unicast-pkts %d (%.2f%%), in-unicast-pkts %d, out-octets %d, in-octets %d", p.Name(), deltas[p.Name()], share,
			after[p.Name()].inPkts-before[p.Name()].inPkts,
			after[p.Name()].outOctets-before[p.Name()].outOctets,
			after[p.Name()].inOctets-before[p.Name()].inOctets)
	}
	return errors.Join(errs...)
}

// lagEgressRxPkts returns the packets the ATE received on flows that the DUT forwards out of the LAG.
func lagEgressRxPkts(t *testing.T, env *testEnv, flows []string) uint64 {
	t.Helper()
	var egress []string
	for _, f := range flows {
		for _, fs := range flowSpecs {
			if fs.name == f && (fs.tx == atePort4Intf || fs.tx == atePort4VlanIntf) {
				egress = append(egress, f)
			}
		}
	}
	var total uint64
	for _, c := range getFlowCounters(t, env.otg, egress) {
		total += c.rx
	}
	return total
}

func awaitMemberEgress(t *testing.T, dut *ondatra.DUTDevice, ports []*ondatra.Port, before map[string]portCounters, want uint64) (map[string]portCounters, error) {
	t.Helper()
	threshold := uint64(float64(want) * minEgressCounterFraction)
	deadline := time.Now().Add(counterExportTimeout)
	for {
		cur, err := awaitStableMemberCounters(t, dut, ports)
		if err != nil {
			return nil, err
		}
		var got uint64
		for name, c := range cur {
			got += c.outPkts - before[name].outPkts
		}
		if got >= threshold {
			return cur, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("member counters show %d egress packets after %v, want >= %d of the %d received by the ATE", got, counterExportTimeout, threshold, want)
		}
	}
}

// awaitStableMemberCounters returns each port's counters once two consecutive samples differ by at most counterStableDelta.
func awaitStableMemberCounters(t *testing.T, dut *ondatra.DUTDevice, ports []*ondatra.Port) (map[string]portCounters, error) {
	t.Helper()
	// Each watcher keeps its own state, so all ports are sampled in parallel.
	watchers := make([]*gnmi.Watcher[*oc.Interface_Counters], len(ports))
	for i, p := range ports {
		var prev *portCounters
		watchers[i] = gnmi.Watch(t, dut, gnmi.OC().Interface(p.Name()).Counters().State(), counterStableTimeout, func(v *ygnmi.Value[*oc.Interface_Counters]) bool {
			c, present := v.Val()
			if !present {
				return false
			}
			cur := portCounters{inPkts: c.GetInUnicastPkts(), outPkts: c.GetOutUnicastPkts(), inOctets: c.GetInOctets(), outOctets: c.GetOutOctets()}
			settled := prev != nil && cur.outPkts-prev.outPkts <= counterStableDelta && cur.inPkts-prev.inPkts <= counterStableDelta
			prev = &cur
			return settled
		})
	}
	counters := make(map[string]portCounters)
	for i, p := range ports {
		got, ok := watchers[i].Await(t)
		if !ok {
			t.Logf("%s counters still changing after %v, using latest values", p.Name(), counterStableTimeout)
		}
		if got == nil || !got.IsPresent() {
			return nil, fmt.Errorf("%s counters not reported within %v", p.Name(), counterStableTimeout)
		}
		c, _ := got.Val()
		counters[p.Name()] = portCounters{inPkts: c.GetInUnicastPkts(), outPkts: c.GetOutUnicastPkts(), inOctets: c.GetInOctets(), outOctets: c.GetOutOctets()}
	}
	return counters, nil
}

func portNames(ports []*ondatra.Port) []string {
	var names []string
	for _, p := range ports {
		names = append(names, p.Name())
	}
	return names
}
