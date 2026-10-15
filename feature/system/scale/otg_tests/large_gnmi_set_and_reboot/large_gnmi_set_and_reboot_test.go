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

// Package large_gnmi_set_and_reboot_test implements SYS-5.1: Configuration
// Commit Validation after Large gNMI-Set and reboot in parallel.
//
// The test sends a large gNMI Set to the DUT and, in parallel, a gNOI cold
// reboot. Once the DUT is back in service, it verifies that the configuration
// of the Set is in place and that the configuration database is unlocked. The
// comments reference the test IDs and steps of the README, for example
// "SYS-5.1.1 Step 2".
package large_gnmi_set_and_reboot_test

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/iputil"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const (
	// Test environment setup: 200 LAGs with 2 member interfaces each and 300 LAGs
	// with 1 member interface each, that is 500 LAGs and 700 member (physical)
	// interfaces.
	numTwoMemberLAGs = 200
	numOneMemberLAGs = 300
	numLAGs          = numTwoMemberLAGs + numOneMemberLAGs
	numMemberIntfs   = 2*numTwoMemberLAGs + numOneMemberLAGs

	// The LAG with zero-based index i gets the IPv4 address lagIPv4Start +
	// i*lagIPv4Step and the IPv6 address lagIPv6Start + i*lagIPv6Step, so that
	// every LAG is in its own /30 and /126 subnets.
	lagIPv4Start = "198.18.0.1"
	lagIPv4Step  = "0.0.0.4"
	lagIPv4Len   = 30
	lagIPv6Start = "2001:db8:5:1::1"
	lagIPv6Step  = "::4"
	lagIPv6Len   = 126

	// lagSetupTimeout is the time allowed for the DUT to report the LAGs of the
	// test environment in its state.
	lagSetupTimeout = 10 * time.Minute
	// largeSetTimeout bounds the large gNMI Set of SYS-5.1.1 Step 2, so that the
	// test cannot hang if the reboot leaves the Set without a response.
	largeSetTimeout = 15 * time.Minute
	// rebootRPCTimeout bounds the gNOI System.Reboot RPC, so that a DUT that
	// goes down without answering cannot block the caller indefinitely.
	rebootRPCTimeout = 5 * time.Minute
	// rebootTimeout is the time allowed for the DUT to boot up and return to
	// service after the reboot of SYS-5.1.1 Step 3. MFF devices boot slowly.
	rebootTimeout = 30 * time.Minute
	// rebootWatchAttemptTimeout bounds a single boot-time subscription made by
	// awaitRebootComplete.
	rebootWatchAttemptTimeout = 5 * time.Minute
	// rebootResubscribeBackoff is the time awaitRebootComplete waits before
	// re-establishing the boot-time subscription after it failed because the
	// DUT is unreachable.
	rebootResubscribeBackoff = 15 * time.Second
	// stateTimeout is the time allowed for the DUT to report the description of
	// SYS-5.1.2 Step 3 in its state.
	stateTimeout = 2 * time.Minute
	// maxReportedMismatches is the maximum number of configuration mismatches
	// that SYS-5.1.2 Step 2 reports individually.
	maxReportedMismatches = 20
	// maxLoggedExcludedIntfs is the maximum number of interface names that
	// fetchUnusedPhysicalInterfaces logs for each reason to exclude interfaces.
	maxLoggedExcludedIntfs = 5

	// rebootMessage is the message of the reboot request of SYS-5.1.1 Step 3.
	rebootMessage = "SYS-5.1.1: cold reboot in parallel with a large gNMI Set"
	// testDescription is the description configured by SYS-5.1.2 Step 3.
	testDescription = "SYS-5.1.2 configuration database unlock check"

	// aristaGNMISetPersistenceCLI makes Arista devices save the configuration
	// applied with gNMI Set to the startup configuration, so that it is retained
	// across reboots.
	aristaGNMISetPersistenceCLI = "management api gnmi\ntransport grpc default\noperation set persistence"
	// aristaNoGNMISetPersistenceCLI reverts aristaGNMISetPersistenceCLI.
	aristaNoGNMISetPersistenceCLI = "management api gnmi\ntransport grpc default\nno operation set persistence"
)

var (
	// swComponentTypes are the types of the software components whose
	// software-version SYS-5.1.2 Step 1 verifies.
	swComponentTypes = []oc.E_PlatformTypes_OPENCONFIG_SOFTWARE_COMPONENT{
		oc.PlatformTypes_OPENCONFIG_SOFTWARE_COMPONENT_BIOS,
		oc.PlatformTypes_OPENCONFIG_SOFTWARE_COMPONENT_BOOT_LOADER,
		oc.PlatformTypes_OPENCONFIG_SOFTWARE_COMPONENT_OPERATING_SYSTEM,
	}

	// connectionErrors are substrings of the gNMI connection errors that the NOTE
	// of SYS-5.1.1 accepts for the gNMI Set of Step 2, because the reboot can
	// close the gNMI connection before the Set response is sent. A
	// codes.DeadlineExceeded error means that the reboot left the Set without a
	// response until largeSetTimeout expired.
	connectionErrors = []string{
		"code = " + codes.Unavailable.String(),
		"code = " + codes.DeadlineExceeded.String(),
		"transport is closing",
		"connection reset",
		"RST_STREAM",
		"EOF",
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// lagSpec is a LAG of the test environment, with the description and the IP
// addresses that SYS-5.1.1 Step 1 configures on it and on its members.
type lagSpec struct {
	name    string
	desc    string
	ipv4    string
	ipv6    string
	members []memberSpec
}

// memberSpec is a member interface of a LAG of the test environment, with the
// description that SYS-5.1.1 Step 1 configures on it.
type memberSpec struct {
	name string
	desc string
}

// testEnv is the test environment and the state that the subtests share.
type testEnv struct {
	lags []lagSpec
	// preRebootBootTime and preRebootSWVersions are recorded by SYS-5.1.1 before
	// the reboot, and checked by SYS-5.1.2 Step 1.
	preRebootBootTime   uint64
	preRebootSWVersions map[oc.E_PlatformTypes_OPENCONFIG_SOFTWARE_COMPONENT]map[string]string
	// postRebootBootTime is the boot-time that the DUT reported when it returned
	// to service in SYS-5.1.1 Step 4.
	postRebootBootTime uint64
}

// memberSnapshot is the original configuration of a member interface, limited
// to the leaves that the test changes, so that the cleanup can restore it.
type memberSnapshot struct {
	name string
	// configured is false if the interface had no configuration at all.
	configured bool
	// desc and enabled are the original config/description and config/enabled
	// leaves, nil if they were not configured.
	desc    *string
	enabled *bool
}

// addRestoreOps adds to sb the operations that restore the original
// configuration of the member interface.
func (s memberSnapshot) addRestoreOps(sb *gnmi.SetBatch) {
	path := gnmi.OC().Interface(s.name)
	if !s.configured {
		gnmi.BatchDelete(sb, path.Config())
		return
	}
	if s.desc != nil {
		gnmi.BatchReplace(sb, path.Description().Config(), *s.desc)
	} else {
		gnmi.BatchDelete(sb, path.Description().Config())
	}
	if s.enabled != nil {
		gnmi.BatchReplace(sb, path.Enabled().Config(), *s.enabled)
	} else {
		gnmi.BatchDelete(sb, path.Enabled().Config())
	}
}

// setDispatchNotifier is a gNMI client that closes dispatched when it sends a
// Set RPC, so that SYS-5.1.1 Step 3 reboots the DUT as soon as the large gNMI
// Set of Step 2 is on its way to the DUT, instead of waiting for a fixed time.
type setDispatchNotifier struct {
	gpb.GNMIClient
	once       sync.Once
	dispatched chan struct{}
	// sent reports whether a Set RPC was sent. It is set before dispatched is
	// closed, so it can be read once dispatched is closed.
	sent atomic.Bool
}

// newSetDispatchNotifier returns a setDispatchNotifier that sends RPCs with c.
func newSetDispatchNotifier(c gpb.GNMIClient) *setDispatchNotifier {
	return &setDispatchNotifier{GNMIClient: c, dispatched: make(chan struct{})}
}

// Set closes dispatched and sends the Set RPC, bounded by largeSetTimeout.
func (n *setDispatchNotifier) Set(ctx context.Context, req *gpb.SetRequest, opts ...grpc.CallOption) (*gpb.SetResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, largeSetTimeout)
	defer cancel()
	n.sent.Store(true)
	n.release()
	return n.GNMIClient.Set(ctx, req, opts...)
}

// release closes dispatched if it is not closed yet.
func (n *setDispatchNotifier) release() {
	n.once.Do(func() { close(n.dispatched) })
}

// TestLargeGNMISetAndReboot implements SYS-5.1. SYS-5.1.2 validates the outcome
// of SYS-5.1.1, so the subtests run in order and stop at the first failure.
func TestLargeGNMISetAndReboot(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	env := setupEnvironment(t, dut)

	tests := []struct {
		name string
		fn   func(t *testing.T, dut *ondatra.DUTDevice, env *testEnv)
	}{{
		name: "SYS-5.1.1: gNMI Batch Set and reboot immediately",
		fn:   testBatchSetAndReboot,
	}, {
		name: "SYS-5.1.2: Post-reboot configuration validation",
		fn:   testPostRebootValidation,
	}}
	for _, tc := range tests {
		if !t.Run(tc.name, func(t *testing.T) { tc.fn(t, dut, env) }) {
			// The next subtests depend on the outcome of this one.
			break
		}
	}
}

// setupEnvironment implements the test environment setup of the README: it
// configures 200 LAGs with 2 member interfaces each and 300 LAGs with 1 member
// interface each, that is 500 LAGs and 700 member interfaces, on physical
// interfaces that are unused on the DUT. It registers cleanups that restore the
// original configuration of the DUT at the end of the test.
//
// It runs in the test rather than in TestMain, because the testbed is only
// reserved, and ondatra.DUT only usable, while fptest.RunTests runs the tests.
func setupEnvironment(t *testing.T, dut *ondatra.DUTDevice) *testEnv {
	t.Helper()
	lags := buildLAGSpecs(t, dut)
	snapshots := snapshotMembers(t, dut, lags)

	// The configuration applied by SYS-5.1.1 must be retained across the reboot.
	// Cleanups run in reverse order, so the persistence is only reverted after
	// the original configuration is restored (and persisted).
	configureGNMISetPersistence(t, dut)
	t.Cleanup(func() { restoreConfig(t, dut, lags, snapshots) })

	t.Logf("Configuring %d LAGs with %d member interfaces on %s", len(lags), numMemberIntfs, dut.Name())
	newScaleLAGs(t, dut, &gnmi.SetBatch{}, lags).Set(t, dut)
	awaitLAGsInState(t, dut, lags)
	return &testEnv{lags: lags}
}

// buildLAGSpecs selects the names of the LAGs and their 700 member interfaces,
// and the descriptions and IP addresses that SYS-5.1.1 Step 1 configures on
// them. The first numTwoMemberLAGs LAGs have two member interfaces, the others
// have one.
func buildLAGSpecs(t *testing.T, dut *ondatra.DUTDevice) []lagSpec {
	t.Helper()
	members := fetchUnusedPhysicalInterfaces(t, dut)
	if len(members) < numMemberIntfs {
		t.Fatalf("DUT %s has %d unused physical interfaces, the test needs %d LAG member interfaces", dut.Name(), len(members), numMemberIntfs)
	}
	ipv4s, err := iputil.GenerateIPsWithStep(lagIPv4Start, numLAGs, lagIPv4Step)
	if err != nil {
		t.Fatalf("Failed to generate the IPv4 addresses of the LAGs: %v", err)
	}
	ipv6s, err := iputil.GenerateIPv6sWithStep(lagIPv6Start, numLAGs, lagIPv6Step)
	if err != nil {
		t.Fatalf("Failed to generate the IPv6 addresses of the LAGs: %v", err)
	}

	lags := make([]lagSpec, 0, numLAGs)
	next := 0
	for i, name := range unusedLAGNames(t, dut, numLAGs) {
		numMembers := 1
		if i < numTwoMemberLAGs {
			numMembers = 2
		}
		lag := lagSpec{
			name: name,
			desc: fmt.Sprintf("SYS-5.1 LAG %s", name),
			ipv4: ipv4s[i],
			ipv6: ipv6s[i],
		}
		for _, member := range members[next : next+numMembers] {
			lag.members = append(lag.members, memberSpec{
				name: member,
				desc: fmt.Sprintf("SYS-5.1 member %s of %s", member, name),
			})
		}
		next += numMembers
		lags = append(lags, lag)
	}
	return lags
}

// unusedLAGNames returns the names of n LAG interfaces that do not exist on the
// DUT, so that the test does not modify existing LAGs.
func unusedLAGNames(t *testing.T, dut *ondatra.DUTDevice, n int) []string {
	t.Helper()
	existing := make(map[string]bool)
	for _, name := range gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().Name().State()) {
		existing[name] = true
	}
	var names []string
	for idx := 0; len(names) < n; idx++ {
		if name := netutil.AggregateInterface(t, dut, idx); !existing[name] {
			names = append(names, name)
		}
	}
	return names
}

// snapshotMembers reads, with a single gNMI Get, the original configuration of
// the member interfaces of lags that the test changes.
func snapshotMembers(t *testing.T, dut *ondatra.DUTDevice, lags []lagSpec) []memberSnapshot {
	t.Helper()
	intfs := gnmi.OC().InterfaceAny()
	batch := gnmi.OCBatch()
	batch.AddPaths(intfs.Name(), intfs.Description(), intfs.Enabled())
	var configured map[string]*oc.Interface
	if root, present := gnmi.Lookup(t, getOpts(dut), batch.Config()).Val(); present {
		configured = root.Interface
	}
	var snapshots []memberSnapshot
	for _, lag := range lags {
		for _, member := range lag.members {
			s := memberSnapshot{name: member.name}
			if intf, ok := configured[member.name]; ok {
				s.configured = true
				s.desc = intf.Description
				s.enabled = intf.Enabled
			}
			snapshots = append(snapshots, s)
		}
	}
	return snapshots
}

// newScaleLAGs adds to sb the configuration of the LAGs in lags and of their
// member interfaces, and returns sb. It configures:
//   - every LAG interface: type ieee8023adLag, enabled and lag-type LACP,
//   - the LACP interface of every LAG with LACP mode ACTIVE,
//   - every member interface: type ethernetCsmacd, enabled and aggregate-id.
func newScaleLAGs(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, lags []lagSpec) *gnmi.SetBatch {
	t.Helper()
	root := &oc.Root{}
	for _, lag := range lags {
		agg := root.GetOrCreateInterface(lag.name)
		agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
		agg.Enabled = ygot.Bool(true)
		agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_LACP
		root.GetOrCreateLacp().GetOrCreateInterface(lag.name).LacpMode = oc.Lacp_LacpActivityType_ACTIVE
		for _, m := range lag.members {
			member := root.GetOrCreateInterface(m.name)
			member.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
			member.Enabled = ygot.Bool(true)
			member.GetOrCreateEthernet().AggregateId = ygot.String(lag.name)
		}
	}
	if deviations.AggregateAtomicUpdate(dut) {
		gnmi.BatchUpdate(sb, gnmi.OC().Config(), root)
		return sb
	}
	for _, lag := range lags {
		if lacp := root.GetLacp().GetInterface(lag.name); lacp != nil {
			gnmi.BatchReplace(sb, gnmi.OC().Lacp().Interface(lag.name).Config(), lacp)
		}
		gnmi.BatchReplace(sb, gnmi.OC().Interface(lag.name).Config(), root.GetInterface(lag.name))
		for _, m := range lag.members {
			gnmi.BatchUpdate(sb, gnmi.OC().Interface(m.name).Config(), root.GetInterface(m.name))
		}
	}
	return sb
}

// deleteScaleLAGs adds to sb the operations that remove the configuration
// created by newScaleLAGs for lags, and returns sb: first the aggregate-id of
// every member interface, then the LACP interface and the LAG interface of
// every LAG.
func deleteScaleLAGs(sb *gnmi.SetBatch, lags []lagSpec) *gnmi.SetBatch {
	for _, lag := range lags {
		for _, m := range lag.members {
			gnmi.BatchDelete(sb, gnmi.OC().Interface(m.name).Ethernet().AggregateId().Config())
		}
	}
	for _, lag := range lags {
		gnmi.BatchDelete(sb, gnmi.OC().Lacp().Interface(lag.name).Config())
		gnmi.BatchDelete(sb, gnmi.OC().Interface(lag.name).Config())
	}
	return sb
}

// addDefaultNetworkInstanceInterfaceOps adds to b, for every interface in
// intfNames, the operation that attaches subinterface si of the interface to
// the default network instance or, if isDelete is true, detaches it.
func addDefaultNetworkInstanceInterfaceOps(b *gnmi.SetBatch, dut *ondatra.DUTDevice, intfNames []string, si uint32, isDelete bool) {
	if !deviations.ExplicitInterfaceInDefaultVRF(dut) {
		return
	}
	ni := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	for _, name := range intfNames {
		id := name
		if deviations.InterfaceRefInterfaceIDFormat(dut) {
			id = fmt.Sprintf("%s.%d", name, si)
		}
		if isDelete {
			gnmi.BatchDelete(b, ni.Interface(id).Config())
			continue
		}
		gnmi.BatchUpdate(b, ni.Interface(id).Config(), &oc.NetworkInstance_Interface{
			Id:           ygot.String(id),
			Interface:    ygot.String(name),
			Subinterface: ygot.Uint32(si),
		})
	}
}

// configureGNMISetPersistence configures the DUT to retain the configuration
// applied with gNMI Set across reboots, on devices that do not do it by default
// (deviation retain_gnmi_cfg_after_reboot), and registers a t.Cleanup function
// that reverts it at the end of the test.
func configureGNMISetPersistence(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	if !deviations.GetRetainGnmiCfgAfterReboot(dut) {
		return
	}
	t.Cleanup(func() {
		t.Logf("Reverting gNMI Set configuration persistence on %s: %q", dut.Name(), aristaNoGNMISetPersistenceCLI)
		dut.Config().New().WithAristaText(aristaNoGNMISetPersistenceCLI).Append(t)
	})
	t.Logf("Configuring %s to retain gNMI Set configuration across reboots: %q", dut.Name(), aristaGNMISetPersistenceCLI)
	dut.Config().New().WithAristaText(aristaGNMISetPersistenceCLI).Append(t)
}

// lagNames returns the names of the LAG interfaces of lags.
func lagNames(lags []lagSpec) []string {
	names := make([]string, 0, len(lags))
	for _, lag := range lags {
		names = append(names, lag.name)
	}
	return names
}

// getOpts returns gNMI options that read the configuration with the gNMI Get
// RPC on all devices, as SYS-5.1.2 Steps 2 and 5 require.
func getOpts(dut *ondatra.DUTDevice) *gnmi.Opts {
	return dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithUseGet())
}

// awaitLAGsInState waits until the DUT reports all the LAG interfaces of lags
// in its state, which shows that the test environment setup is applied.
func awaitLAGsInState(t *testing.T, dut *ondatra.DUTDevice, lags []lagSpec) {
	t.Helper()
	batch := gnmi.OCBatch()
	batch.AddPaths(gnmi.OC().InterfaceAny().Type())
	_, ok := gnmi.Watch(t, dut, batch.State(), lagSetupTimeout, func(v *ygnmi.Value[*oc.Root]) bool {
		root, present := v.Val()
		if !present {
			return false
		}
		for _, lag := range lags {
			if root.GetInterface(lag.name).GetType() != oc.IETFInterfaces_InterfaceType_ieee8023adLag {
				return false
			}
		}
		return true
	}).Await(t)
	if !ok {
		t.Fatalf("DUT %s did not report the %d LAG interfaces of the test environment within %v", dut.Name(), len(lags), lagSetupTimeout)
	}
}

// restoreConfig removes the configuration that the test added, and restores the
// original configuration of the member interfaces, with a single gNMI Set.
func restoreConfig(t *testing.T, dut *ondatra.DUTDevice, lags []lagSpec, snapshots []memberSnapshot) {
	t.Helper()
	t.Logf("Restoring the original configuration of %s", dut.Name())
	sb := &gnmi.SetBatch{}
	addDefaultNetworkInstanceInterfaceOps(sb, dut, lagNames(lags), 0, true)
	deleteScaleLAGs(sb, lags)
	for _, s := range snapshots {
		s.addRestoreOps(sb)
	}
	sb.Set(t, dut)
}

// newLargeBatch returns the gNMI batch of SYS-5.1.1 Step 1: a description on
// all the 700 member (physical) and 500 LAG interfaces, and an IPv4 and an IPv6
// address on all the 500 LAG interfaces.
func newLargeBatch(dut *ondatra.DUTDevice, lags []lagSpec) *gnmi.SetBatch {
	sb := &gnmi.SetBatch{}
	for _, lag := range lags {
		for _, member := range lag.members {
			gnmi.BatchUpdate(sb, gnmi.OC().Interface(member.name).Description().Config(), member.desc)
		}
		a := &attrs.Attributes{
			Desc:    lag.desc,
			IPv4:    lag.ipv4,
			IPv4Len: lagIPv4Len,
			IPv6:    lag.ipv6,
			IPv6Len: lagIPv6Len,
		}
		intf := a.NewOCInterface(lag.name, dut)
		// NewOCInterface builds an Ethernet interface, turn it into a LAG.
		intf.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
		intf.Ethernet = nil
		gnmi.BatchUpdate(sb, gnmi.OC().Interface(lag.name).Config(), intf)
	}
	// On devices that require it, attach subinterface 0 of every LAG, which
	// holds the IP addresses, to the default network instance.
	addDefaultNetworkInstanceInterfaceOps(sb, dut, lagNames(lags), 0, false)
	return sb
}

// testBatchSetAndReboot implements SYS-5.1.1 - gNMI Batch Set and reboot
// immediately.
func testBatchSetAndReboot(t *testing.T, dut *ondatra.DUTDevice, env *testEnv) {
	// Record the boot-time and the software versions before the reboot, for
	// SYS-5.1.2 Step 1.
	env.preRebootBootTime = gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())
	if !deviations.SwVersionUnsupported(dut) {
		env.preRebootSWVersions = swVersionsByType(t, dut, swComponentTypes...)
	}
	t.Logf("Before the reboot: boot-time %d, software versions %v", env.preRebootBootTime, env.preRebootSWVersions)

	// SYS-5.1.1 Step 1: Create a gNMI batch configuration to configure a
	// description on all the 700 physical and 500 LAG interfaces, and IPv4 and
	// IPv6 addresses on all the 500 LAG interfaces.
	//
	// The batch merges (gNMI update) this configuration into the configuration
	// of the DUT. The union_replace listed in the RPC coverage of the README is
	// not used: it replaces the whole configuration of the DUT and needs its
	// complete native CLI configuration as a baseline (see gNMI-3,
	// union_replace_test), which is outside the scope of this test.
	batch := newLargeBatch(dut, env.lags)

	// Connect the gNMI and gNOI clients now, so that connecting does not delay
	// Steps 2 and 3. rebootDUT reuses the gNOI client that ondatra caches.
	notifier := newSetDispatchNotifier(dut.RawAPIs().GNMI(t))
	dut.RawAPIs().GNOI(t)

	var (
		setResult         *ygnmi.Result
		setErr, rebootErr *string
	)
	testt.ParallelFatal(t,
		// SYS-5.1.1 Step 2: In a goroutine, use gNMI Batch Set to set the
		// configuration on the DUT.
		func(t testing.TB) {
			// Unblock Step 3 even if the Set RPC is never sent.
			defer notifier.release()
			setErr = testt.CaptureFatal(t, func(t testing.TB) {
				setResult = batch.Set(t, dut.GNMIOpts().WithClient(notifier))
			})
		},
		// SYS-5.1.1 Step 3: In a separate goroutine running in parallel, trigger a
		// complete chassis cold reboot using the gnoi.system.System.Reboot RPC with
		// the method set to COLD, as soon as the gNMI Set of Step 2 is sent.
		func(t testing.TB) {
			<-notifier.dispatched
			rebootErr = testt.CaptureFatal(t, func(t testing.TB) {
				if !notifier.sent.Load() {
					t.Fatalf("the gNMI Set of Step 2 was not sent, so the DUT is not rebooted")
				}
				rebootDUT(t, dut, spb.RebootMethod_COLD, rebootMessage)
			})
		},
	)
	if rebootErr != nil {
		t.Fatalf("SYS-5.1.1 Step 3: cold reboot failed: %s (Step 2 gNMI Set error: %s)", *rebootErr, errString(setErr))
	}

	// NOTE of SYS-5.1.1: the reboot can close the gNMI connection before the Set
	// response is sent, so a connection error such as "rpc error: code =
	// Unavailable desc = transport is closing" is acceptable for the Set.
	switch {
	case setErr == nil:
		t.Logf("SYS-5.1.1 Step 2: the gNMI Set succeeded, response timestamp %v", setResult.Timestamp)
	case isConnectionError(*setErr):
		t.Logf("SYS-5.1.1 Step 2: the reboot interrupted the gNMI Set connection, which is acceptable: %s", *setErr)
	default:
		// Not fatal, so that Step 4 still waits for the DUT to return to service
		// before the cleanup restores its configuration.
		t.Errorf("SYS-5.1.1 Step 2: the gNMI Set failed with an error that is not a gNMI connection error: %s", *setErr)
	}

	// SYS-5.1.1 Step 4: Wait for the device to boot up and return to service.
	env.postRebootBootTime = awaitRebootComplete(t, dut, env.preRebootBootTime, rebootTimeout)

	// A successful Set response sent after the reboot means that the transport
	// re-sent the Set once the DUT was back in service, so the test could not
	// verify the fate of the configuration committed during the reboot.
	if setErr == nil && setResult.Timestamp.UnixNano() > int64(env.postRebootBootTime) {
		t.Errorf("SYS-5.1.1 Step 2: the gNMI Set response timestamp %v is after the boot-time %d: the Set was applied after the reboot, likely re-sent by a transport-level retry, so the test is inconclusive", setResult.Timestamp, env.postRebootBootTime)
	}
}

// rebootDUT sends a gNOI System.Reboot request with the given method and
// message to the DUT, with no delay and force set to true.
func rebootDUT(t testing.TB, dut *ondatra.DUTDevice, method spb.RebootMethod, message string) *spb.RebootResponse {
	t.Helper()
	req := &spb.RebootRequest{
		Method:  method,
		Delay:   0,
		Message: message,
		Force:   true,
	}
	ctx, cancel := context.WithTimeout(context.Background(), rebootRPCTimeout)
	defer cancel()
	t.Logf("Sending gNOI System.Reboot request to %s: %v", dut.Name(), req)
	resp, err := dut.RawAPIs().GNOI(t).System().Reboot(ctx, req)
	if err != nil {
		t.Fatalf("gNOI System.Reboot(%v) on %s failed: %v", req, dut.Name(), err)
	}
	t.Logf("gNOI System.Reboot response from %s: %v", dut.Name(), resp)
	return resp
}

// awaitRebootComplete waits until the DUT reports a /system/state/boot-time
// newer than prevBootTime, which shows that the DUT has rebooted and serves
// gNMI again, and returns the new boot-time.
func awaitRebootComplete(t testing.TB, dut *ondatra.DUTDevice, prevBootTime uint64, timeout time.Duration) uint64 {
	t.Helper()
	bootTimeQuery := gnmi.OC().System().BootTime().State()
	start := time.Now()
	deadline := start.Add(timeout)
	for attempt := 1; time.Until(deadline) > 0; attempt++ {
		var (
			bootTime uint64
			rebooted bool
		)
		errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			w := gnmi.Watch(t, dut, bootTimeQuery, min(time.Until(deadline), rebootWatchAttemptTimeout), func(v *ygnmi.Value[uint64]) bool {
				bt, present := v.Val()
				return present && bt > prevBootTime
			})
			defer w.Cancel()
			if v, ok := w.Await(t); ok {
				bootTime, _ = v.Val()
				rebooted = true
			}
		})
		if rebooted {
			t.Logf("DUT %s is back in service %v after the wait started, boot-time changed from %d to %d.", dut.Name(), time.Since(start).Round(time.Second), prevBootTime, bootTime)
			return bootTime
		}
		if errMsg == nil {
			t.Logf("Attempt %d: DUT %s did not report a boot-time newer than %d yet (%v elapsed).", attempt, dut.Name(), prevBootTime, time.Since(start).Round(time.Second))
			continue
		}
		t.Logf("Attempt %d: DUT %s is not reachable yet (%v elapsed): %s", attempt, dut.Name(), time.Since(start).Round(time.Second), *errMsg)
		time.Sleep(min(rebootResubscribeBackoff, time.Until(deadline)))
	}
	t.Fatalf("DUT %s did not report a boot-time newer than %d within %v", dut.Name(), prevBootTime, timeout)
	return 0
}

// swVersionsByType returns the /components/component/state/software-version
// of every software component of the given types, keyed by software component
// type and then by component name.
func swVersionsByType(t testing.TB, dut *ondatra.DUTDevice, swTypes ...oc.E_PlatformTypes_OPENCONFIG_SOFTWARE_COMPONENT) map[oc.E_PlatformTypes_OPENCONFIG_SOFTWARE_COMPONENT]map[string]string {
	t.Helper()
	wantTypes := make(map[oc.E_PlatformTypes_OPENCONFIG_SOFTWARE_COMPONENT]bool, len(swTypes))
	for _, swType := range swTypes {
		wantTypes[swType] = true
	}
	batch := gnmi.OCBatch()
	batch.AddPaths(
		gnmi.OC().ComponentAny().Type().State().PathStruct(),
		gnmi.OC().ComponentAny().SoftwareVersion().State().PathStruct(),
	)
	versions := make(map[oc.E_PlatformTypes_OPENCONFIG_SOFTWARE_COMPONENT]map[string]string)
	root, present := gnmi.Lookup(t, dut, batch.State()).Val()
	if !present {
		return versions
	}
	for name, c := range root.Component {
		swType, ok := c.GetType().(oc.E_PlatformTypes_OPENCONFIG_SOFTWARE_COMPONENT)
		if !ok || !wantTypes[swType] {
			continue
		}
		if versions[swType] == nil {
			versions[swType] = make(map[string]string)
		}
		versions[swType][name] = c.GetSoftwareVersion()
	}
	return versions
}

// isConnectionError reports whether the captured error message msg is a gNMI
// connection error, see connectionErrors.
func isConnectionError(msg string) bool {
	for _, connErr := range connectionErrors {
		if strings.Contains(msg, connErr) {
			return true
		}
	}
	return false
}

// errString returns the captured error message msg, or "none" if msg is nil.
func errString(msg *string) string {
	if msg == nil {
		return "none"
	}
	return *msg
}

// testPostRebootValidation implements SYS-5.1.2 - Post-reboot configuration
// validation.
func testPostRebootValidation(t *testing.T, dut *ondatra.DUTDevice, env *testEnv) {
	// SYS-5.1.2 Step 1: Verify boot time using /system/state/boot-time and
	// software versions using /components/component/state/software-version.
	t.Run("Step 1: boot-time and software versions", func(t *testing.T) {
		bootTime := gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())
		if bootTime <= env.preRebootBootTime {
			t.Errorf("boot-time got %d, want greater than %d, the boot-time before the reboot", bootTime, env.preRebootBootTime)
		}
		if bootTime != env.postRebootBootTime {
			t.Errorf("boot-time got %d, want %d, the boot-time when the DUT returned to service in SYS-5.1.1 Step 4: the DUT rebooted again", bootTime, env.postRebootBootTime)
		}
		if deviations.SwVersionUnsupported(dut) {
			t.Log("Skipping the software version verification: deviation sw_version_unsupported")
			return
		}
		got := swVersionsByType(t, dut, swComponentTypes...)
		t.Logf("Software versions after the reboot: %v", got)
		for _, swType := range swComponentTypes {
			if len(got[swType]) == 0 {
				t.Logf("The DUT reports no %v software component", swType)
			}
			for name, version := range got[swType] {
				if version == "" {
					t.Errorf("software component %s of type %v reports no software-version", name, swType)
				}
			}
		}
		if len(got[oc.PlatformTypes_OPENCONFIG_SOFTWARE_COMPONENT_OPERATING_SYSTEM]) == 0 {
			t.Errorf("the DUT reports no OPERATING_SYSTEM software component")
		}
		if diff := cmp.Diff(env.preRebootSWVersions, got); diff != "" {
			t.Errorf("software versions changed across the reboot (-before +after):\n%s", diff)
		}
	})

	// SYS-5.1.2 Step 2: Using gNMI Get, verify that the interface descriptions and
	// IP addresses applied in SYS-5.1.1 are present in the configuration.
	t.Run("Step 2: descriptions and IP addresses in the configuration", func(t *testing.T) {
		intfs := gnmi.OC().InterfaceAny()
		batch := gnmi.OCBatch()
		batch.AddPaths(
			intfs.Description(),
			intfs.SubinterfaceAny().Ipv4().AddressAny().Ip(),
			intfs.SubinterfaceAny().Ipv4().AddressAny().PrefixLength(),
			intfs.SubinterfaceAny().Ipv6().AddressAny().Ip(),
			intfs.SubinterfaceAny().Ipv6().AddressAny().PrefixLength(),
		)
		root := gnmi.Get(t, getOpts(dut), batch.Config())
		var mismatches []string
		for _, lag := range env.lags {
			for _, member := range lag.members {
				if got := root.GetInterface(member.name).GetDescription(); got != member.desc {
					mismatches = append(mismatches, fmt.Sprintf("%s: description got %q, want %q", member.name, got, member.desc))
				}
			}
			if got := root.GetInterface(lag.name).GetDescription(); got != lag.desc {
				mismatches = append(mismatches, fmt.Sprintf("%s: description got %q, want %q", lag.name, got, lag.desc))
			}
			mismatches = append(mismatches, addressMismatches(root, lag)...)
		}
		if n := len(mismatches); n > 0 {
			shown := min(n, maxReportedMismatches)
			t.Errorf("%d interface descriptions or IP addresses applied in SYS-5.1.1 are missing or wrong in the configuration after the reboot, the first %d are:\n%s", n, shown, strings.Join(mismatches[:shown], "\n"))
			return
		}
		t.Logf("The %d interface descriptions and %d IP addresses applied in SYS-5.1.1 are in the configuration", numMemberIntfs+numLAGs, 2*numLAGs)
	})

	// SYS-5.1.2 Steps 3 to 5 verify that the configuration database is unlocked
	// and that commits are functional after the reboot.
	t.Run("Steps 3-5: configuration database is unlocked", func(t *testing.T) {
		intf := env.lags[0].members[0].name
		descPath := gnmi.OC().Interface(intf).Description()

		// SYS-5.1.2 Step 3: Issue a gNMI Set request to configure a test
		// description on any one DUT interface.
		errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Update(t, dut, descPath.Config(), testDescription)
		})
		// SYS-5.1.2 Step 4: Verify that the gNMI Set request succeeds, which shows
		// that the configuration database is unlocked and commits are functional.
		if errMsg != nil {
			t.Fatalf("SYS-5.1.2 Step 4: the gNMI Set of description %q on %s failed, the configuration database is locked or commits are not functional: %s", testDescription, intf, *errMsg)
		}

		// SYS-5.1.2 Step 5: Verify the new description is applied correctly using a
		// gNMI Get request.
		if got := gnmi.Get(t, getOpts(dut), descPath.Config()); got != testDescription {
			t.Errorf("SYS-5.1.2 Step 5: configured description of %s got %q, want %q", intf, got, testDescription)
		}
		// The applied description is also expected in the state, streamed with an
		// ON_CHANGE subscription.
		onChange := dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE))
		if got, ok := gnmi.Watch(t, onChange, descPath.State(), stateTimeout, func(v *ygnmi.Value[string]) bool {
			desc, present := v.Val()
			return present && desc == testDescription
		}).Await(t); !ok {
			t.Errorf("SYS-5.1.2 Step 5: state description of %s got %v, want %q within %v", intf, got, testDescription, stateTimeout)
		}
	})
}

// addressMismatches returns the differences between the IPv4 and IPv6 addresses
// of subinterface 0 of the LAG in root, the configuration read from the DUT, and
// the addresses that SYS-5.1.1 Step 1 configures on the LAG.
func addressMismatches(root *oc.Root, lag lagSpec) []string {
	var mismatches []string
	sub := root.GetInterface(lag.name).GetSubinterface(0)
	var ipv4Addrs map[string]*oc.Interface_Subinterface_Ipv4_Address
	if ipv4 := sub.GetIpv4(); ipv4 != nil {
		ipv4Addrs = ipv4.Address
	}
	if got, ok := prefixLength(ipv4Addrs, lag.ipv4); !ok || got != lagIPv4Len {
		mismatches = append(mismatches, fmt.Sprintf("%s: IPv4 address %s/%d got present %t, prefix-length %d", lag.name, lag.ipv4, lagIPv4Len, ok, got))
	}
	var ipv6Addrs map[string]*oc.Interface_Subinterface_Ipv6_Address
	if ipv6 := sub.GetIpv6(); ipv6 != nil {
		ipv6Addrs = ipv6.Address
	}
	if got, ok := prefixLength(ipv6Addrs, lag.ipv6); !ok || got != lagIPv6Len {
		mismatches = append(mismatches, fmt.Sprintf("%s: IPv6 address %s/%d got present %t, prefix-length %d", lag.name, lag.ipv6, lagIPv6Len, ok, got))
	}
	return mismatches
}

// prefixLength returns the prefix length of the IP address ip in addrs, the
// addresses of a subinterface keyed by IP address, and whether ip is in addrs.
// The addresses are compared semantically, so that different textual forms of
// the same IPv6 address match.
func prefixLength[A interface{ GetPrefixLength() uint8 }](addrs map[string]A, ip string) (uint8, bool) {
	for addr, a := range addrs {
		if iputil.IPEqual(addr, ip) {
			return a.GetPrefixLength(), true
		}
	}
	return 0, false
}

// fetchUnusedPhysicalInterfaces returns the sorted names of the physical
// interfaces of the DUT that are not in use.
func fetchUnusedPhysicalInterfaces(t testing.TB, dut *ondatra.DUTDevice) []string {
	t.Helper()
	intfs := gnmi.OC().InterfaceAny()
	batch := gnmi.OCBatch()
	batch.AddPaths(
		intfs.Type().State().PathStruct(),
		intfs.HardwarePort().State().PathStruct(),
		intfs.Ethernet().AggregateId().State().PathStruct(),
		intfs.SubinterfaceAny().Ipv4().AddressAny().Ip().State().PathStruct(),
		intfs.SubinterfaceAny().Ipv6().AddressAny().Ip().State().PathStruct(),
	)
	root := gnmi.Get(t, dut, batch.State())
	client, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.Name()))
	if err != nil {
		t.Fatalf("Failed to create a gNMI client of %s: %v", dut.Name(), err)
	}
	for name := range optionalInterfaceFlags(t, dut, client, intfs.Management().State()) {
		if intf := root.GetInterface(name); intf != nil {
			intf.Management = ygot.Bool(true)
		}
	}
	for name := range optionalInterfaceFlags(t, dut, client, intfs.Cpu().State()) {
		if intf := root.GetInterface(name); intf != nil {
			intf.Cpu = ygot.Bool(true)
		}
	}
	parents := breakoutParents(root.Interface)
	var unused []string
	excluded := make(map[string][]string)
	for name, intf := range root.Interface {
		reason := exclusionReason(name, intf)
		if reason == "" && parents[name] {
			reason = "breakout port with channels"
		}
		if reason != "" {
			excluded[reason] = append(excluded[reason], name)
			continue
		}
		unused = append(unused, name)
	}
	sort.Strings(unused)

	t.Logf("DUT %s reports %d interfaces, of which %d are unused physical interfaces", dut.Name(), len(root.Interface), len(unused))
	reasons := make([]string, 0, len(excluded))
	for reason := range excluded {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	for _, reason := range reasons {
		names := excluded[reason]
		sort.Strings(names)
		t.Logf("DUT %s: %d interfaces excluded, %s, for example %v", dut.Name(), len(names), reason, names[:min(len(names), maxLoggedExcludedIntfs)])
	}
	return unused
}

// breakoutParents returns the names of the interfaces in intfs that are broken
// out into channels.
func breakoutParents(intfs map[string]*oc.Interface) map[string]bool {
	byPort := make(map[string][]string)
	for name, intf := range intfs {
		if hwPort := intf.GetHardwarePort(); hwPort != "" {
			byPort[hwPort] = append(byPort[hwPort], name)
		}
	}
	parents := make(map[string]bool)
	for _, names := range byPort {
		for _, parent := range names {
			for _, channel := range names {
				if len(channel) > len(parent) && strings.HasPrefix(channel, parent) && strings.ContainsRune("/:.", rune(channel[len(parent)])) {
					parents[parent] = true
				}
			}
		}
	}
	return parents
}

// optionalInterfaceFlags returns the names of the interfaces for which the DUT
// reports true for q.
func optionalInterfaceFlags(t testing.TB, dut *ondatra.DUTDevice, client *ygnmi.Client, q ygnmi.WildcardQuery[bool]) map[string]bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	flags := make(map[string]bool)
	vals, err := ygnmi.LookupAll(ctx, client, q)
	if err != nil {
		t.Logf("DUT %s does not report %v, it is considered false on all the interfaces: %v", dut.Name(), q, err)
		return flags
	}
	for _, v := range vals {
		set, present := v.Val()
		if elems := v.Path.GetElem(); present && set && len(elems) > 1 {
			flags[elems[1].GetKey()["name"]] = true
		}
	}
	return flags
}

// exclusionReason returns why intf is not an unused physical interface, or ""
// if it is an unused physical interface.
func exclusionReason(name string, intf *oc.Interface) string {
	intfType := intf.GetType()
	switch {
	case intf.GetHardwarePort() == "":
		return "no hardware-port"
	case intfType != oc.IETFInterfaces_InterfaceType_UNSET && intfType != oc.IETFInterfaces_InterfaceType_ethernetCsmacd:
		return fmt.Sprintf("type %v", intfType)
	case intf.GetManagement() || isManagementName(name) || isManagementName(intf.GetHardwarePort()):
		return "management interface"
	case intf.GetCpu():
		return "CPU interface"
	case intf.GetEthernet().GetAggregateId() != "":
		return "LAG member"
	case hasIPAddress(intf):
		return "has an IP address"
	}
	return ""
}

// isManagementName reports whether name designates a management interface.
func isManagementName(name string) bool {
	name = strings.ToLower(name)
	return strings.Contains(name, "mgmt") || strings.Contains(name, "management")
}

// hasIPAddress reports whether any subinterface of intf has an IPv4 or IPv6
// address.
func hasIPAddress(intf *oc.Interface) bool {
	for _, sub := range intf.Subinterface {
		if ipv4 := sub.GetIpv4(); ipv4 != nil && len(ipv4.Address) > 0 {
			return true
		}
		if ipv6 := sub.GetIpv6(); ipv6 != nil && len(ipv6.Address) > 0 {
			return true
		}
	}
	return false
}
