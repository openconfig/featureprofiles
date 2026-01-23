// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package afts_reboot_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	"github.com/openconfig/featureprofiles/internal/telemetry/aftcache"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"

	spb "github.com/openconfig/gnoi/system"
)

const (
	aftConvergenceTime = 30 * time.Minute
	applyPolicyType    = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	gnmiTimeout        = 5 * time.Minute
	isisDUTArea        = "49.0001"
	isisDUTSystemID    = "1920.0000.2001"
	mtu                = 1500
	v4PrefixLen        = 30
	v6PrefixLen        = 126
	peerGrpNameV4P1    = "BGP-PEER-GROUP-V4-P1"
	peerGrpNameV6P1    = "BGP-PEER-GROUP-V6-P1"
	peerGrpNameV4P2    = "BGP-PEER-GROUP-V4-P2"
	peerGrpNameV6P2    = "BGP-PEER-GROUP-V6-P2"
	// contextTimeout is the overall timeout for the gNOI reboot operation.
	contextTimeout = 20 * time.Minute
	// maxRebootTime is the maximum time allowed for the DUT to complete the reboot.
	maxRebootTime = 20 * time.Minute
	// rebootPollInterval is the interval at which the DUT's reachability is polled during reboot.
	rebootPollInterval    = 30 * time.Second
	aristaPersistConfig   = "management api gnmi\ntransport grpc default\noperation set persistence"
	aristaNoPersistConfig = "management api gnmi\ntransport grpc default\nno operation set persistence"
)

var (
	ateP1 = attrs.Attributes{
		IPv4:    "192.0.2.2",
		IPv4Len: v4PrefixLen,
		IPv6:    "2001:db8::2",
		IPv6Len: v6PrefixLen,
		MAC:     "00:00:02:02:02:02",
		MTU:     mtu,
		Name:    "port1",
	}
	ateP2 = attrs.Attributes{
		IPv4:    "192.0.2.6",
		IPv4Len: v4PrefixLen,
		IPv6:    "2001:db8::6",
		IPv6Len: v6PrefixLen,
		MAC:     "00:00:03:03:03:03",
		MTU:     mtu,
		Name:    "port2",
	}
	dutP1 = attrs.Attributes{
		IPv4:    "192.0.2.1",
		IPv4Len: v4PrefixLen,
		IPv6:    "2001:db8::1",
		IPv6Len: v6PrefixLen,
	}
	dutP2 = attrs.Attributes{
		IPv4:    "192.0.2.5",
		IPv4Len: v4PrefixLen,
		IPv6:    "2001:db8::5",
		IPv6Len: v6PrefixLen,
	}
	ateAttrs       = []attrs.Attributes{ateP1, ateP2}
	dutAttrs       = []attrs.Attributes{dutP1, dutP2}
	v4PeerGrpNames = []string{peerGrpNameV4P1, peerGrpNameV4P2}
	v6PeerGrpNames = []string{peerGrpNameV6P1, peerGrpNameV6P2}
	port1Name      = "port1"
	port2Name      = "port2"
	ipv4TwoNHs     = map[string]bool{ateP1.IPv4: true, ateP2.IPv4: true}
	ipv6TwoNHs     = map[string]bool{ateP1.IPv6: true, ateP2.IPv6: true}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configureAllowPolicy(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()
	d := &oc.Root{}
	routePolicy := d.GetOrCreateRoutingPolicy()
	policyDefinition := routePolicy.GetOrCreatePolicyDefinition(cfgplugins.ALLOW)
	statement, err := policyDefinition.AppendNewStatement("id-1")
	if err != nil {
		return fmt.Errorf("failed to append new statement to policy definition %s: %v", cfgplugins.ALLOW, err)
	}
	statement.GetOrCreateActions().PolicyResult = applyPolicyType
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), routePolicy)
	return nil
}

func (tc *testCase) configureToStoreRunningGNMIConfig(t *testing.T) error {
	if tc.dut.Vendor() == ondatra.ARISTA {
		tc.dut.Config().New().WithAristaText(aristaPersistConfig).Append(t)
		t.Logf("Applied Arista config to persist gNMI running config: %q", aristaPersistConfig)
	}
	return nil
}

func (tc *testCase) unconfigureToStoreRunningGNMIConfig(t *testing.T) error {
	if tc.dut.Vendor() == ondatra.ARISTA {
		tc.dut.Config().New().WithAristaText(aristaNoPersistConfig).Append(t)
		t.Logf("Applied Arista config to remove gNMI running config persistence: %q", aristaNoPersistConfig)
	}
	return nil
}

// configureDUT configures all the interfaces, BGP, and ISIS on the DUT.
func (tc *testCase) configureDUT(t *testing.T) error {
	t.Helper()
	dut := tc.dut
	dutPort1 := dut.Port(t, port1Name).Name()
	dutIntf1 := dutP1.NewOCInterface(dutPort1, dut)
	gnmi.Replace(t, dut, gnmi.OC().Interface(dutPort1).Config(), dutIntf1)
	dutPort2 := dut.Port(t, port2Name).Name()
	dutIntf2 := dutP2.NewOCInterface(dutPort2, dut)
	gnmi.Replace(t, dut, gnmi.OC().Interface(dutPort2).Config(), dutIntf2)
	// Configure default network instance.
	t.Log("Configure Default Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, dutPort1, deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, dutPort2, deviations.DefaultNetworkInstance(dut), 0)
	}
	if err := configureAllowPolicy(t, dut); err != nil {
		return err
	}
	t.Log("Configure BGP")
	sb := &gnmi.SetBatch{}
	for ix, ateAttr := range ateAttrs {
		nbrs := []*cfgplugins.BgpNeighbor{
			{LocalAS: cfgplugins.DutAS, PeerAS: cfgplugins.AteAS1, Neighborip: ateAttr.IPv4, IsV4: true},
			{LocalAS: cfgplugins.DutAS, PeerAS: cfgplugins.AteAS1, Neighborip: ateAttr.IPv6, IsV4: false},
		}
		nbrsConfig := cfgplugins.BGPNeighborsConfig{
			RouterID:      dutP1.IPv4,
			PeerGrpNameV4: v4PeerGrpNames[ix],
			PeerGrpNameV6: v6PeerGrpNames[ix],
			Nbrs:          nbrs,
		}
		if err := cfgplugins.CreateBGPNeighbors(t, dut, sb, nbrsConfig); err != nil {
			return err
		}
	}
	sb.Set(t, dut)
	t.Log("Configure ISIS")
	b := &gnmi.SetBatch{}
	isisData := &cfgplugins.ISISGlobalParams{
		DUTArea:             isisDUTArea,
		DUTSysID:            isisDUTSystemID,
		ISISInterfaceNames:  []string{dutPort1}, // Only configure ISIS on one port.
		NetworkInstanceName: deviations.DefaultNetworkInstance(dut),
	}
	cfgplugins.NewISIS(t, dut, isisData, b)
	b.Set(t, dut)
	return nil
}

func (tc *testCase) configureATE(t *testing.T) {
	ate := tc.ate
	config := otgconfighelpers.ConfigureATEWithISISAndBGPRoutes(t, &otgconfighelpers.ATEAdvertiseRoutes{
		ATE:      ate,
		ATEAttrs: ateAttrs,
		DUTAttrs: dutAttrs,
		BGPV4Routes: &otgconfighelpers.AdvertisedRoutes{
			StartingAddress: otgconfighelpers.StartingBGPRouteIPv4,
			PrefixLength:    otgconfighelpers.V4PrefixLen,
			Count:           otgconfighelpers.DefaultBGPRouteCount,
			ATEAS:           cfgplugins.AteAS1,
		},
		BGPV6Routes: &otgconfighelpers.AdvertisedRoutes{
			StartingAddress: otgconfighelpers.StartingBGPRouteIPv6,
			PrefixLength:    otgconfighelpers.V6PrefixLen,
			Count:           otgconfighelpers.DefaultBGPRouteCount,
			ATEAS:           cfgplugins.AteAS1,
		},
	})
	otg := ate.OTG()
	otg.PushConfig(t, config)
	otg.StartProtocols(t)
}

func (tc *testCase) waitForBGPSession(t *testing.T) error {
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(tc.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	verifySessionState := func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, ok := val.Val()
		if !ok {
			return false
		}
		t.Logf("BGP session state: %s", state.String())
		return state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}
	for i, ateAttr := range ateAttrs {
		nbrPath := bgpPath.Neighbor(ateAttr.IPv4)
		if _, ok := gnmi.Watch(t, tc.dut, nbrPath.SessionState().State(), gnmiTimeout, verifySessionState).Await(t); !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, tc.dut, nbrPath.State()))
			return fmt.Errorf("no BGP neighbor formed for port%d IPv4 (%s)", i+1, ateAttr.IPv4)
		}
		nbrPathv6 := bgpPath.Neighbor(ateAttr.IPv6)
		if _, ok := gnmi.Watch(t, tc.dut, nbrPathv6.SessionState().State(), gnmiTimeout, verifySessionState).Await(t); !ok {
			fptest.LogQuery(t, "BGPv6 reported state", nbrPathv6.State(), gnmi.Get(t, tc.dut, nbrPathv6.State()))
			return fmt.Errorf("no BGPv6 neighbor formed for port%d IPv6 (%s)", i+1, ateAttr.IPv6)
		}
	}
	return nil
}

func (tc *testCase) waitForISISAdjacency(t *testing.T) error {
	isisPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(tc.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, deviations.DefaultNetworkInstance(tc.dut)).Isis()
	verifyAdjacencyState := func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
		state, ok := val.Val()
		if !ok {
			return false
		}
		t.Logf("ISIS adjacency state: %s", state.String())
		return state == oc.Isis_IsisInterfaceAdjState_UP
	}
	dutPort := tc.dut.Port(t, port1Name).Name()
	adjPath := isisPath.Interface(dutPort).Level(2).AdjacencyAny()
	if _, ok := gnmi.WatchAll(t, tc.dut, adjPath.AdjacencyState().State(), gnmiTimeout, verifyAdjacencyState).Await(t); !ok {
		return fmt.Errorf("no ISIS adjacency formed for port1 (%s)", dutPort)
	}
	return nil
}

func generateBGPPrefixes(t *testing.T) map[string]bool {
	wantPrefixes := make(map[string]bool)
	cidrV4 := otgconfighelpers.StartingBGPRouteIPv4 + "/" + fmt.Sprintf("%d", otgconfighelpers.V4PrefixLen)
	for pfix := range netutil.GenCIDRs(t, cidrV4, int(otgconfighelpers.DefaultBGPRouteCount)) {
		wantPrefixes[pfix] = true
	}
	cidrV6 := otgconfighelpers.StartingBGPRouteIPv6 + "/" + fmt.Sprintf("%d", otgconfighelpers.V6PrefixLen)
	for pfix6 := range netutil.GenCIDRs(t, cidrV6, int(otgconfighelpers.DefaultBGPRouteCount)) {
		wantPrefixes[pfix6] = true
	}
	return wantPrefixes
}

func generateISISPrefixes(t *testing.T) map[string]bool {
	wantPrefixes := make(map[string]bool)
	v4Cidr := otgconfighelpers.StartingISISRouteV4 + "/" + fmt.Sprintf("%d", otgconfighelpers.V4PrefixLen)
	for pfix := range netutil.GenCIDRs(t, v4Cidr, int(otgconfighelpers.DefaultISISRouteCount)) {
		wantPrefixes[pfix] = true
	}
	v6Cidr := otgconfighelpers.StartingISISRouteV6 + "/" + fmt.Sprintf("%d", otgconfighelpers.V6PrefixLen)
	for pfix6 := range netutil.GenCIDRs(t, v6Cidr, int(otgconfighelpers.DefaultISISRouteCount)) {
		wantPrefixes[pfix6] = true
	}
	return wantPrefixes
}

func (tc *testCase) bootTime(t *testing.T) (uint64, error) {
	var bootTime uint64
	_, ok := gnmi.Watch(t, tc.dut, gnmi.OC().System().BootTime().State(), gnmiTimeout, func(val *ygnmi.Value[uint64]) bool {
		var ok bool
		bootTime, ok = val.Val()
		return ok
	}).Await(t)
	if !ok {
		return 0, fmt.Errorf("failed to get boot time")
	}
	return bootTime, nil
}

func (tc *testCase) waitForReboot(t *testing.T, lastBootTime uint64) {
	t.Helper()
	startReboot := time.Now()
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	{
		ticker := time.NewTicker(rebootPollInterval)
		defer ticker.Stop()
		timeout := time.After(maxRebootTime)
		var deviceWentDown bool
	rebootLoop:
		for {
			select {
			case <-timeout:
				t.Fatalf("Timeout exceeded: DUT did not reboot within maximum boot time(%v).", maxRebootTime)
			case <-ticker.C:
				var currentTime string
				errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					currentTime = gnmi.Get(t, tc.dut, gnmi.OC().System().CurrentDatetime().State())
				})
				if errMsg != nil {
					if !deviceWentDown {
						t.Logf("Device is now unreachable. Waiting for it to come back up.")
						deviceWentDown = true
					}
					t.Logf("Time elapsed %.2f seconds, DUT not reachable yet: %s.", time.Since(startReboot).Seconds(), *errMsg)
				} else {
					if deviceWentDown {
						t.Logf("Device rebooted successfully with received time: %v.", currentTime)
						break rebootLoop
					}
					t.Logf("Device is still reachable; reboot hasn't started yet.")
				}
			}
		}
	}
	t.Logf("Device boot time: %.2f seconds.", time.Since(startReboot).Seconds())
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	_, err := tc.dut.RawAPIs().BindingDUT().DialGNMI(t.Context())
	if err != nil {
		t.Fatalf("Failed to dial GNMI after reboot: %v", err)
	}
	// Wait for boot time to change.
	_, ok := gnmi.Watch(t, tc.dut, gnmi.OC().System().BootTime().State(), maxRebootTime, bootTimePredicate(lastBootTime)).Await(t)
	if !ok {
		currentBootTime, _ := tc.bootTime(t)
		t.Fatalf("Boot time did not update after reboot. Current: %d, Last: %d", currentBootTime, lastBootTime)
	}
	t.Logf("Device boot time: %.2f seconds.", time.Since(startReboot).Seconds())
	currentBootTime, _ := tc.bootTime(t)
	t.Logf("Boot time successfully changed from %d to %d", lastBootTime, currentBootTime)
}

func bootTimePredicate(lastBootTime uint64) func(val *ygnmi.Value[uint64]) bool {
	return func(val *ygnmi.Value[uint64]) bool {
		currentBootTime, ok := val.Val()
		return ok && currentBootTime > lastBootTime
	}
}

func (tc *testCase) rebootDUT(t *testing.T) {
	t.Helper()
	rebootRequest := &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
		Delay:   0,
		Message: "Reboot chassis without delay",
		Force:   true,
	}
	gnoiClient, err := tc.dut.RawAPIs().BindingDUT().DialGNOI(t.Context())
	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	bootTimeBeforeReboot := gnmi.Get(t, tc.dut, gnmi.OC().System().BootTime().State())
	t.Logf("DUT boot time before reboot: %v", bootTimeBeforeReboot)
	t.Log("Sending reboot request to DUT")
	ctxWithTimeout, cancel := context.WithTimeout(t.Context(), contextTimeout)
	defer cancel()
	if _, err = gnoiClient.System().Reboot(ctxWithTimeout, rebootRequest); err != nil {
		t.Fatalf("Failed to reboot chassis with unexpected err: %v", err)
	}
	t.Log("Reboot request sent to DUT, waiting for DUT to reboot")
}

type testCase struct {
	name string
	dut  *ondatra.DUTDevice
	ate  *ondatra.ATEDevice
}

func TestReboot(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	bgpPrefixes := generateBGPPrefixes(t)
	isisPrefixes := generateISISPrefixes(t)
	verifyBGPPrefixes := aftcache.InitialSyncStoppingCondition(t, dut, bgpPrefixes, ipv4TwoNHs, ipv6TwoNHs)
	verifyISISPrefixes := aftcache.AssertNextHopCount(t, dut, isisPrefixes, 1)
	tc := testCase{
		name: "AFT-5.1.1: AFT DUT Reboot",
		dut:  dut,
		ate:  ate,
	}
	t.Run(tc.name, func(t *testing.T) {
		if deviations.GetRetainGnmiCfgAfterReboot(dut) {
			if err := tc.configureToStoreRunningGNMIConfig(t); err != nil {
				t.Fatalf("failed to configure DUT to store running gNMI config: %v", err)
			}
			defer tc.unconfigureToStoreRunningGNMIConfig(t)
		}
		gnmiClient, err := tc.dut.RawAPIs().BindingDUT().DialGNMI(t.Context())
		if err != nil {
			t.Fatalf("Failed to dial GNMI: %v", err)
		}
		if err := tc.configureDUT(t); err != nil {
			t.Fatalf("failed to configure DUT: %v", err)
		}
		tc.configureATE(t)
		t.Log("Waiting for BGP neighbor to establish...")
		if err := tc.waitForBGPSession(t); err != nil {
			t.Fatalf("Unable to establish BGP session: %v", err)
		}
		if err := tc.waitForISISAdjacency(t); err != nil {
			t.Fatalf("Unable to establish ISIS Adjacency: %v", err)
		}
		aftSession := aftcache.NewAFTStreamSession(t.Context(), t, gnmiClient, tc.dut)
		t.Logf("Initial verification of %d bgp prefixes and %d isis prefixes", len(bgpPrefixes), len(isisPrefixes))
		aftSession.ListenUntil(t.Context(), t, aftConvergenceTime, verifyBGPPrefixes)
		aftSession.ListenUntil(t.Context(), t, aftConvergenceTime, verifyISISPrefixes)
		t.Log("Done listening for initial verification.")
		// Get initial boot time.
		lastBootTime, err := tc.bootTime(t)
		if err != nil {
			t.Fatalf("Failed to get boot time: %v", err)
		}
		// Reboot
		tc.rebootDUT(t)
		// Verify boot time changed.
		tc.waitForReboot(t, lastBootTime)
		// Re-dial GNMI and create new AFT session because the connection was reset.
		gnmiClient, err = tc.dut.RawAPIs().BindingDUT().DialGNMI(t.Context())
		if err != nil {
			t.Fatalf("Failed to dial GNMI after reboot: %v", err)
		}
		aftSession = aftcache.NewAFTStreamSession(t.Context(), t, gnmiClient, tc.dut)
		t.Log("Waiting for BGP neighbor to establish after reboot...")
		if err := tc.waitForBGPSession(t); err != nil {
			t.Fatalf("Unable to establish BGP session: %v", err)
		}
		t.Logf("Re-verification of %d bgp prefixes and %d isis prefixes", len(bgpPrefixes), len(isisPrefixes))
		aftSession.ListenUntil(t.Context(), t, aftConvergenceTime, verifyBGPPrefixes)
		aftSession.ListenUntil(t.Context(), t, aftConvergenceTime, verifyISISPrefixes)
		t.Log("Done listening after reboot.")
	})
}
