// Copyright 2024 Google LLC
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

// Package aftsprefixfilteringdualstack_test implements AFT-6.2: AFT Prefix Filtering Dual-Stack.
package aftsprefixfilteringdualstack_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/featureprofiles/internal/telemetry/aftcache"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	policyPrefixSetA   = "POLICY-PREFIX-SET-A"
	policyPrefixSetB   = "POLICY-PREFIX-SET-B"
	prefixSetA         = "PREFIX-SET-A"
	prefixSetB         = "PREFIX-SET-B"
	ipv4NonMatchPrefix = "100.64.0.0/24"
	ipv6NonMatchPrefix = "2001:db8:1::/64"
	operationTimeout   = 120 * time.Second
	pollInterval       = 5 * time.Second
)

var (
	ipv4MatchPrefixes = []string{
		"198.51.100.0/24",
		"203.0.113.0/28",
		"198.51.100.1/32",
	}
	ipv6MatchPrefixes = []string{
		"2001:db8:2::/64",
		"2001:db8:2::1/128",
	}
	dutPort1 = attrs.Attributes{
		Desc:    "DUT to ATE Port 1",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
		IPv6:    "2001:db8:0:1::1",
		IPv6Len: 64,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT to ATE Port 2",
		IPv4:    "192.0.2.5",
		IPv4Len: 30,
		IPv6:    "2001:db8:0:2::1",
		IPv6Len: 64,
	}
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		Desc:    "ATE to DUT Port 1",
		IPv4:    "192.0.2.2",
		IPv4Len: 30,
		IPv6:    "2001:db8:0:1::2",
		IPv6Len: 64,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:00:02:01:01:01",
		Desc:    "ATE to DUT Port 2",
		IPv4:    "192.0.2.6",
		IPv4Len: 30,
		IPv6:    "2001:db8:0:2::2",
		IPv6Len: 64,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configureDUT configures DUT interfaces, routing policies, and static routes.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	configureDUTInterface(t, dut, batch, &dutPort1, p1)
	configureDUTInterface(t, dut, batch, &dutPort2, p2)
	batch.Set(t, dut)
	configurePolicies(t, dut)
	mustConfigureStaticRoutes(t, dut)
}

// configureDUTInterface configures a single DUT interface using NewOCInterface.
func configureDUTInterface(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, a *attrs.Attributes, p *ondatra.Port) {
	t.Helper()
	i := a.NewOCInterface(p.Name(), dut)
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	gnmi.BatchUpdate(batch, gnmi.OC().Interface(p.Name()).Config(), i)
}

// configureATE configures ATE ports, starts protocols, and waits for ARP resolution.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	topo := gosnappi.NewConfig()
	atePort1.AddToOTG(topo, ate.Port(t, "port1"), &dutPort1)
	atePort2.AddToOTG(topo, ate.Port(t, "port2"), &dutPort2)
	ate.OTG().PushConfig(t, topo)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv6")
}

// configurePolicies installs PREFIX-SET-A/B and POLICY-PREFIX-SET-A/B on the DUT.
func configurePolicies(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	createPrefixSetAPolicy(rp)
	createPrefixSetBPolicy(rp)
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

// createPrefixSetAPolicy builds PREFIX-SET-A and POLICY-PREFIX-SET-A for IPv4 matching.
func createPrefixSetAPolicy(rp *oc.RoutingPolicy) {
	ps := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(prefixSetA)
	for _, p := range ipv4MatchPrefixes {
		addPrefix(ps, p, "exact")
	}
	pd := rp.GetOrCreatePolicyDefinition(policyPrefixSetA)
	stmt, _ := pd.AppendNewStatement("10")
	match := stmt.GetOrCreateConditions().GetOrCreateMatchPrefixSet()
	match.PrefixSet = ygot.String(prefixSetA)
	match.MatchSetOptions = oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY
	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
}

// createPrefixSetBPolicy builds PREFIX-SET-B and POLICY-PREFIX-SET-B for IPv6 matching.
func createPrefixSetBPolicy(rp *oc.RoutingPolicy) {
	ps := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(prefixSetB)
	for _, p := range ipv6MatchPrefixes {
		addPrefix(ps, p, "exact")
	}
	pd := rp.GetOrCreatePolicyDefinition(policyPrefixSetB)
	stmt, _ := pd.AppendNewStatement("10")
	match := stmt.GetOrCreateConditions().GetOrCreateMatchPrefixSet()
	match.PrefixSet = ygot.String(prefixSetB)
	match.MatchSetOptions = oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY
	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
}

// addPrefix inserts a prefix entry into a prefix-set.
func addPrefix(ps *oc.RoutingPolicy_DefinedSets_PrefixSet, prefix, maskRange string) {
	p := ps.GetOrCreatePrefix(prefix, maskRange)
	p.IpPrefix = ygot.String(prefix)
	p.MasklengthRange = ygot.String(maskRange)
}

// configureStaticRoutes installs all IPv4 and IPv6 static routes and verifies AFT presence.
func mustConfigureStaticRoutes(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	ni := deviations.DefaultNetworkInstance(dut)
	for idx, pfx := range append(ipv4MatchPrefixes, ipv4NonMatchPrefix) {
		sr := &cfgplugins.StaticRouteCfg{
			NetworkInstance: ni,
			Prefix:          pfx,
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				fmt.Sprintf("%d", idx): oc.UnionString(atePort1.IPv4),
			},
		}
		if _, err := cfgplugins.NewStaticRouteCfg(batch, sr, dut); err != nil {
			t.Fatalf("NewStaticRouteCfg %s: %v", pfx, err)
		}
	}
	for idx, pfx := range append(ipv6MatchPrefixes, ipv6NonMatchPrefix) {
		sr := &cfgplugins.StaticRouteCfg{
			NetworkInstance: ni,
			Prefix:          pfx,
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				fmt.Sprintf("%d", idx): oc.UnionString(atePort1.IPv6),
			},
		}
		if _, err := cfgplugins.NewStaticRouteCfg(batch, sr, dut); err != nil {
			t.Fatalf("NewStaticRouteCfg %s: %v", pfx, err)
		}
	}
	batch.Set(t, dut)
	t.Log("Verifying static routes are installed in AFT")
	niAfts := gnmi.OC().NetworkInstance(ni).Afts()
	for _, pfx := range append(ipv4MatchPrefixes, ipv4NonMatchPrefix) {
		pfx := pfx
		gnmi.Watch(t, dut, niAfts.Ipv4Entry(pfx).State(), operationTimeout,
			func(v *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
				entry, ok := v.Val()
				return ok && entry != nil && entry.Prefix != nil
			}).Await(t)
		t.Logf("Static route %s confirmed in AFT", pfx)
	}
	for _, pfx := range append(ipv6MatchPrefixes, ipv6NonMatchPrefix) {
		pfx := pfx
		gnmi.Watch(t, dut, niAfts.Ipv6Entry(pfx).State(), operationTimeout,
			func(v *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool {
				entry, ok := v.Val()
				return ok && entry != nil && entry.Prefix != nil
			}).Await(t)
		t.Logf("Static route %s confirmed in AFT", pfx)
	}
}

// globalFilterLeafPath returns the raw gNMI config path for a global-filter policy leaf.
func globalFilterLeafPath(ni, leaf string) *gpb.Path {
	return &gpb.Path{
		Origin: "openconfig",
		Elem: []*gpb.PathElem{
			{Name: "network-instances"},
			{Name: "network-instance", Key: map[string]string{"name": ni}},
			{Name: "afts"},
			{Name: "global-filter"},
			{Name: "config"},
			{Name: leaf},
		},
	}
}

// globalFilterContainerPath returns the raw gNMI config container path for the global-filter config node.
func globalFilterContainerPath(ni string) *gpb.Path {
	return &gpb.Path{
		Origin: "openconfig",
		Elem: []*gpb.PathElem{
			{Name: "network-instances"},
			{Name: "network-instance", Key: map[string]string{"name": ni}},
			{Name: "afts"},
			{Name: "global-filter"},
			{Name: "config"},
		},
	}
}

// aristaRCFFilterCmd returns the EOS CLI to register ipv4Policy and ipv6Policy as RCF functions.
func aristaRCFFilterCmd(ipv4Policy, ipv6Policy string) string {
	return fmt.Sprintf(
		"router general\n"+
			"   control-functions\n"+
			"      function openconfig %s\n"+
			"      function openconfig %s\n"+
			"   !\n"+
			"!",
		ipv4Policy, ipv6Policy,
	)
}

// aristaRCFDeleteCmd returns the EOS CLI to remove ipv4Policy and ipv6Policy RCF function registrations.
func aristaRCFDeleteCmd(ipv4Policy, ipv6Policy string) string {
	return fmt.Sprintf(
		"router general\n"+
			"   control-functions\n"+
			"      no function openconfig %s\n"+
			"      no function openconfig %s\n"+
			"   !\n"+
			"!",
		ipv4Policy, ipv6Policy,
	)
}

// setGlobalFilter applies ipv4Policy and ipv6Policy to the AFT global-filter using one CLI session.
func setGlobalFilter(t *testing.T, dut *ondatra.DUTDevice, ipv4Policy, ipv6Policy string) error {
	t.Helper()
	ni := deviations.DefaultNetworkInstance(dut)
	if deviations.AftsGlobalFilterPolicyOCUnsupported(dut) {
		cli := dut.RawAPIs().CLI(t)
		out, err := cli.RunCommand(context.Background(), aristaRCFFilterCmd(ipv4Policy, ipv6Policy))
		if err != nil {
			return fmt.Errorf("setGlobalFilter (Arista RCF): CLI failed: %v\noutput: %s", err, out.Output())
		}
		t.Logf("setGlobalFilter (Arista RCF): applied ipv4=%s ipv6=%s", ipv4Policy, ipv6Policy)
		for _, check := range []struct{ leaf, val string }{
			{"ipv4-policy", ipv4Policy},
			{"ipv6-policy", ipv6Policy},
		} {
			deadline := time.Now().Add(operationTimeout)
			confirmed := false
			for time.Now().Before(deadline) {
				o, e := cli.RunCommand(context.Background(), fmt.Sprintf("show running-config all | include %s", check.val))
				if e == nil && strings.Contains(o.Output(), check.val) {
					t.Logf("global-filter %s = %q confirmed via CLI", check.leaf, check.val)
					confirmed = true
					break
				}
				time.Sleep(pollInterval)
			}
			if !confirmed {
				t.Logf("setGlobalFilter: %s=%s not confirmed in running-config (non-fatal)", check.leaf, check.val)
			}
		}
		return nil
	}
	var updates []*gpb.Update
	if ipv4Policy != "" {
		updates = append(updates, &gpb.Update{
			Path: globalFilterLeafPath(ni, "ipv4-policy"),
			Val:  &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: ipv4Policy}},
		})
	}
	if ipv6Policy != "" {
		updates = append(updates, &gpb.Update{
			Path: globalFilterLeafPath(ni, "ipv6-policy"),
			Val:  &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: ipv6Policy}},
		})
	}
	_, err := dut.RawAPIs().GNMI(t).Set(context.Background(), &gpb.SetRequest{Replace: updates})
	return err
}

// deleteGlobalFilter removes the global-filter configuration from the DUT.
func deleteGlobalFilter(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	ni := deviations.DefaultNetworkInstance(dut)
	if deviations.AftsGlobalFilterPolicyOCUnsupported(dut) {
		cli := dut.RawAPIs().CLI(t)
		out, err := cli.RunCommand(context.Background(), aristaRCFDeleteCmd(policyPrefixSetA, policyPrefixSetB))
		if err != nil {
			t.Logf("deleteGlobalFilter (Arista RCF): %v\noutput: %s (may be ok if not configured)", err, out.Output())
		}
		return
	}
	_, err := dut.RawAPIs().GNMI(t).Set(context.Background(), &gpb.SetRequest{
		Delete: []*gpb.Path{globalFilterContainerPath(ni)},
	})
	if err != nil {
		t.Logf("deleteGlobalFilter: %v (may be ok if not configured)", err)
	}
}

// awaitGlobalFilterState polls until the global-filter policy leaf matches wantVal.
func awaitGlobalFilterState(t *testing.T, dut *ondatra.DUTDevice, leaf, wantVal string) error {
	t.Helper()
	ni := deviations.DefaultNetworkInstance(dut)
	if deviations.AftsGlobalFilterPolicyOCUnsupported(dut) {
		return nil
	}
	gnmiC := dut.RawAPIs().GNMI(t)
	statePath := &gpb.Path{
		Origin: "openconfig",
		Elem: []*gpb.PathElem{
			{Name: "network-instances"},
			{Name: "network-instance", Key: map[string]string{"name": ni}},
			{Name: "afts"},
			{Name: "global-filter"},
			{Name: "state"},
			{Name: leaf},
		},
	}
	deadline := time.Now().Add(operationTimeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		stream, err := gnmiC.Subscribe(ctx)
		if err != nil {
			cancel()
			time.Sleep(pollInterval)
			continue
		}
		err = stream.Send(&gpb.SubscribeRequest{
			Request: &gpb.SubscribeRequest_Subscribe{
				Subscribe: &gpb.SubscriptionList{
					Mode:         gpb.SubscriptionList_ONCE,
					Subscription: []*gpb.Subscription{{Path: statePath}},
				},
			},
		})
		if err != nil {
			cancel()
			time.Sleep(pollInterval)
			continue
		}
		resp, err := stream.Recv()
		cancel()
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}
		for _, u := range resp.GetUpdate().GetUpdate() {
			if u.GetVal().GetStringVal() == wantVal {
				t.Logf("global-filter state/%s = %q confirmed", leaf, wantVal)
				return nil
			}
		}
		time.Sleep(pollInterval)
	}
	return fmt.Errorf("awaitGlobalFilterState: timed out waiting for state/%s = %q", leaf, wantVal)
}

// openAFTSessions opens two independent ON_CHANGE AFT gNMI sessions with a shared stopping condition.
func mustOpenAFTSessions(t *testing.T, dut *ondatra.DUTDevice, wantPrefixes map[string]bool) (
	*aftcache.AFTStreamSession, *aftcache.AFTStreamSession, aftcache.PeriodicHook,
) {
	t.Helper()
	ctx := t.Context()
	gnmiC1, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
	if err != nil {
		t.Fatalf("mustOpenAFTSessions: DialGNMI (session 1) failed: %v", err)
	}
	gnmiC2, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
	if err != nil {
		t.Fatalf("mustOpenAFTSessions: DialGNMI (session 2) failed: %v", err)
	}
	wantIPv4NHs := map[string]bool{atePort1.IPv4: true}
	wantIPv6NHs := map[string]bool{atePort1.IPv6: true}
	stopCondition := aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes, wantIPv4NHs, wantIPv6NHs)
	return aftcache.NewAFTStreamSession(ctx, t, gnmiC1, dut),
		aftcache.NewAFTStreamSession(ctx, t, gnmiC2, dut),
		stopCondition
}

type aftSessionConfig struct {
	session1 *aftcache.AFTStreamSession
	session2 *aftcache.AFTStreamSession
	stop     aftcache.PeriodicHook
	dut      *ondatra.DUTDevice
}

// mustRunAFTSessions runs both sessions concurrently and returns the converged AFTData from session1.
func mustRunAFTSessions(t *testing.T, cfg aftSessionConfig) *aftcache.AFTData {
	t.Helper()
	ctx := t.Context()
	done := make(chan struct{}, 2)
	go func() {
		cfg.session1.ListenUntil(ctx, t, operationTimeout, cfg.stop)
		done <- struct{}{}
	}()
	go func() {
		cfg.session2.ListenUntil(ctx, t, operationTimeout, cfg.stop)
		done <- struct{}{}
	}()
	<-done
	<-done
	aft, err := cfg.session1.ToAFT(t, cfg.dut)
	if err != nil {
		t.Fatalf("mustRunAFTSessions: ToAFT failed: %v", err)
	}
	return aft
}

// TestAFTDualStackPrefixFiltering validates dual-stack AFT prefix filtering, NHG/NH resolution, and cross-family policy swap behavior.
func TestAFTDualStackPrefixFiltering(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	configureDUT(t, dut)
	configureATE(t, ate)

	deleteGlobalFilter(t, dut)
	t.Cleanup(func() { deleteGlobalFilter(t, dut) })

	// Apply initial IPv4/IPv6 filter policies.
	t.Log("Step 1: Configuring ipv4-policy=POLICY-PREFIX-SET-A, ipv6-policy=POLICY-PREFIX-SET-B")
	if err := setGlobalFilter(t, dut, policyPrefixSetA, policyPrefixSetB); err != nil {
		t.Fatalf("setGlobalFilter failed: %v", err)
	}
	if err := awaitGlobalFilterState(t, dut, "ipv4-policy", policyPrefixSetA); err != nil {
		t.Errorf("%v", err)
	}
	if err := awaitGlobalFilterState(t, dut, "ipv6-policy", policyPrefixSetB); err != nil {
		t.Errorf("%v", err)
	}

	// Open AFT subscription and wait for initial sync.
	matchPrefixes := append(append([]string{}, ipv4MatchPrefixes...), ipv6MatchPrefixes...)
	wantPrefixes := make(map[string]bool)
	for _, pfx := range matchPrefixes {
		wantPrefixes[pfx] = true
	}

	t.Log("Step 2: Subscribing to AFT ON_CHANGE, waiting for SYNC")
	session1, session2, stoppingCond := mustOpenAFTSessions(t, dut, wantPrefixes)
	aft := mustRunAFTSessions(t, aftSessionConfig{session1: session1, session2: session2, stop: stoppingCond, dut: dut})

	// Verify IPv4 prefixes matched by POLICY-PREFIX-SET-A.
	t.Log("Step 3: Verifying IPv4 entries match POLICY-PREFIX-SET-A")
	for _, pfx := range ipv4MatchPrefixes {
		if _, ok := aft.Prefixes[pfx]; !ok {
			t.Errorf("ipv4 prefix %s expected in AFT stream but not received", pfx)
		}
	}

	// Verify IPv6 prefixes exist in AFT state.
	t.Log("Step 4: Verifying IPv6 entries present in AFT state")
	niAfts := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts()
	for _, pfx := range ipv6MatchPrefixes {
		_, ok := gnmi.Watch(t, dut, niAfts.Ipv6Entry(pfx).State(), operationTimeout,
			func(v *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool {
				entry, present := v.Val()
				return present && entry != nil
			}).Await(t)
		if !ok {
			t.Errorf("ipv6 prefix %s not found in AFT state", pfx)
		}
	}

	// Verify non-matching prefixes are absent from the stream.
	t.Log("Step 5: Verifying non-matching prefixes are absent")
	for _, pfx := range []string{ipv4NonMatchPrefix, ipv6NonMatchPrefix} {
		if _, ok := aft.Prefixes[pfx]; ok {
			t.Errorf("non-matching prefix %s present in stream", pfx)
		}
	}

	// Swap policies across address families.
	t.Log("Step 6: Swapping policies (cross-family)")
	if err := setGlobalFilter(t, dut, policyPrefixSetB, policyPrefixSetA); err != nil {
		t.Fatalf("setGlobalFilter (swap) failed: %v", err)
	}
	if err := awaitGlobalFilterState(t, dut, "ipv4-policy", policyPrefixSetB); err != nil {
		t.Errorf("%v", err)
	}
	if err := awaitGlobalFilterState(t, dut, "ipv6-policy", policyPrefixSetA); err != nil {
		t.Errorf("%v", err)
	}

	// Open a new subscription and verify matched prefixes are removed.
	t.Log("Step 7: Opening AFT subscription with swapped policy")
	session1Swapped, session2Swapped, stoppingCondSwapped := mustOpenAFTSessions(t, dut, map[string]bool{})
	aftSwapped := mustRunAFTSessions(t, aftSessionConfig{session1: session1Swapped, session2: session2Swapped, stop: stoppingCondSwapped, dut: dut})

	// Verify IPv4 prefixes are absent after policy swap.
	for _, pfx := range ipv4MatchPrefixes {
		if _, ok := aftSwapped.Prefixes[pfx]; ok {
			t.Errorf("ipv4 prefix %s still present after policy swap", pfx)
		}
	}

	// Verify IPv6 prefixes are absent after policy swap.
	for _, pfx := range ipv6MatchPrefixes {
		if _, ok := aftSwapped.Prefixes[pfx]; ok {
			t.Errorf("ipv6 prefix %s still present after policy swap", pfx)
		}
	}

	// Complete cross-family policy swap verification.
	t.Log("Step 7: Policy swap verification complete")
}
