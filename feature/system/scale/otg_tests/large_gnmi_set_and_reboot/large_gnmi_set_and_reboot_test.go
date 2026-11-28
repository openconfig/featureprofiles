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
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openconfig/entity-naming/entname"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/iputil"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const (
	// Test environment setup: 200 LAGs with 2 member interfaces each and 300 LAGs
	// with 1 member interface each, that is 500 LAGs and 700 member (physical)
	// interfaces. A DUT with fewer unused physical interfaces or LAG interfaces
	// gets fewer LAGs, see buildLAGSpecs.
	numTwoMemberLAGs = 200
	numOneMemberLAGs = 300
	numLAGIntfs      = numTwoMemberLAGs + numOneMemberLAGs
	numMemberIntfs   = 2*numTwoMemberLAGs + numOneMemberLAGs
	// numVLANs is the number of 802.1Q VLAN subinterfaces (VLAN IDs 1..numVLANs)
	// distributed across the LAGs in SYS-5.1.1 Step 1 so that the large gNMI Set
	// configures 700 VLAN subinterfaces with IPv4 and IPv6 addresses even on a
	// DUT with fewer physical or LAG interfaces.
	numVLANs = 700

	// The VLAN subinterface with zero-based index i gets the IPv4 address
	// vlanIPv4Start + i*vlanIPv4Step and the IPv6 address vlanIPv6Start +
	// i*vlanIPv6Step, so that every VLAN subinterface is in its own /30 and /126
	// subnets.
	vlanIPv4Start = "198.18.0.1"
	vlanIPv4Step  = "0.0.0.4"
	vlanIPv4Len   = 30
	vlanIPv6Start = "2001:db8:5:1::1"
	vlanIPv6Step  = "::4"
	vlanIPv6Len   = 126

	// lagSetupTimeout is the time allowed for the DUT to report the interfaces
	// of the test environment in its state once they are configured.
	lagSetupTimeout = 10 * time.Minute
	// largeSetTimeout bounds the large gNMI Set of SYS-5.1.1 Step 2, so that the
	// test cannot hang if the reboot leaves the Set without a response.
	largeSetTimeout = 15 * time.Minute
	// rebootRPCTimeout bounds the gNOI System.Reboot RPC, so that a DUT that
	// goes down without answering cannot block the caller indefinitely.
	rebootRPCTimeout = 5 * time.Minute
	// rebootTimeout is the time allowed for the DUT to boot up after the reboot
	// of SYS-5.1.1 Step 3, that is to report a new boot-time. MFF devices boot
	// slowly.
	rebootTimeout = 30 * time.Minute
	// rebootWatchAttemptTimeout bounds a single boot-time subscription made by
	// awaitRebootComplete.
	rebootWatchAttemptTimeout = 5 * time.Minute
	// rebootResubscribeBackoff is the time awaitRebootComplete waits before
	// re-establishing the boot-time subscription after it failed because the
	// DUT is unreachable.
	rebootResubscribeBackoff = 15 * time.Second
	// serviceTimeout is the time allowed, once the DUT has booted up, for it to
	// report the interfaces of the test environment in its state again, which
	// completes its return to service in SYS-5.1.1 Step 4. The linecards of a
	// modular device can come up after its control plane.
	serviceTimeout = 20 * time.Minute
	// swVersionTimeout is the time allowed for the DUT to report, after the
	// reboot, the software versions that it reported before the reboot.
	swVersionTimeout = 10 * time.Minute
	// bootTimeTolerance is the maximum difference between two boot-times that
	// the DUT reports for the same boot, so that small variations of the
	// reported boot-time are not taken for a reboot: a reboot changes the
	// boot-time by more than the time that the DUT was up before it, which
	// includes the setup of the test environment.
	bootTimeTolerance = time.Minute
	// stateTimeout is the time allowed for the DUT to report the description of
	// SYS-5.1.2 Step 3 in its state.
	stateTimeout = 5 * time.Minute
	// onChangeWatchTimeout bounds the initial ON_CHANGE subscription in
	// SYS-5.1.2 Step 5 before falling back to polling the state.
	onChangeWatchTimeout = time.Minute
	// statePollInterval is the interval between two reads of the state of the
	// DUT while the test waits for an expected state.
	statePollInterval = 15 * time.Second
	// stateReadTimeout bounds each of these reads, so that a read that the DUT
	// leaves without a response, for example while it boots up, is retried
	// instead of blocking the test.
	stateReadTimeout = 2 * time.Minute
	// gnmiGetAttempts is the maximum number of attempts for a gNMI Get RPC, so
	// that a transient authentication or transport error during a sequence of
	// many gNMI Gets does not abort the test.
	gnmiGetAttempts = 3
	// gnmiGetRetryDelay is the delay between two attempts of a gNMI Get RPC.
	gnmiGetRetryDelay = 5 * time.Second
	// maxReportedMismatches is the maximum number of differences with the
	// expected configuration or state that the test reports individually.
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

	// namingVendors maps the vendors of the DUTs to the vendors of the entity
	// naming library, which names the LAG interfaces, see unusedLAGNames.
	namingVendors = map[ondatra.Vendor]entname.Vendor{
		ondatra.ARISTA:  entname.VendorArista,
		ondatra.CISCO:   entname.VendorCisco,
		ondatra.JUNIPER: entname.VendorJuniper,
		ondatra.NOKIA:   entname.VendorNokia,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// lagSpec is a LAG of the test environment, with the description and the
// configuration that SYS-5.1.1 Step 1 configures on it, on its members, and on
// its VLAN subinterfaces.
type lagSpec struct {
	name    string
	desc    string
	members []memberSpec
	vlans   []vlanSpec
}

// memberSpec is a member interface of a LAG of the test environment, with the
// description that SYS-5.1.1 Step 1 configures on it.
type memberSpec struct {
	name string
	desc string
}

// vlanSpec is an 802.1Q VLAN subinterface of a LAG of the test environment,
// with the VLAN ID and IPv4/IPv6 addresses that SYS-5.1.1 Step 1 configures on
// it.
type vlanSpec struct {
	id   uint16
	ipv4 string
	ipv6 string
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

// Set sends the Set RPC, bounded by largeSetTimeout, and closes dispatched so
// that SYS-5.1.1 Step 3 triggers the cold reboot immediately.
func (n *setDispatchNotifier) Set(ctx context.Context, req *gpb.SetRequest, opts ...grpc.CallOption) (*gpb.SetResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, largeSetTimeout)
	defer cancel()
	n.sent.Store(true)
	defer n.release()
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
// configures LAGs with 2 member interfaces and LAGs with 1 member interface on
// physical interfaces that are unused on the DUT, 200 and 300 LAGs, that is 500
// LAGs and 700 member interfaces, at the scale of the README, or fewer on a DUT
// without enough unused physical interfaces (see buildLAGSpecs). It registers
// cleanups that restore the original configuration of the DUT at the end of the
// test.
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

	t.Logf("Configuring %d LAGs with %d member interfaces on %s", len(lags), numMembers(lags), dut.Name())
	sb := &gnmi.SetBatch{}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		// On these devices, SYS-5.1.1 Step 1 attaches the LAG interfaces to the
		// default network instance (see newLargeBatch), which a device can reject
		// if the type of the default network instance is not configured. The type
		// is configured with the test environment, so that the large gNMI Set only
		// has the configuration of the README, and the cleanup leaves it in place,
		// as the default network instance always exists.
		defNI := deviations.DefaultNetworkInstance(dut)
		gnmi.BatchUpdate(sb, gnmi.OC().NetworkInstance(defNI).Config(), &oc.NetworkInstance{
			Name: ygot.String(defNI),
			Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE,
		})
	}
	newScaleLAGs(t, dut, sb, lags).Set(t, dut)
	awaitEnvInState(t, dut, lags, lagSetupTimeout)
	return &testEnv{lags: lags}
}

// buildLAGSpecs selects the names of the LAGs of the test environment and of
// their member interfaces, and the descriptions and IP addresses that SYS-5.1.1
// Step 1 configures on them. The LAGs are not reported by the DUT, and their
// members are unused physical interfaces of the DUT. The two members of a LAG
// have the same speed, see pairBySpeedGroup. The first LAGs have two member
// interfaces, the others have one.
//
// The test environment has the scale of the README, 200 LAGs with 2 member
// interfaces and 300 LAGs with 1 member interface, if the DUT has enough unused
// physical interfaces and LAG interfaces. Otherwise, it has as many LAGs with 2
// member interfaces as the proportions of the README (200 such LAGs for 700
// member interfaces and for 500 LAGs) and the pairs of interfaces with the same
// speed allow, and up to 300 LAGs with 1 member interface on the other unused
// physical interfaces, within the LAG interfaces of the DUT. The reduced scale
// is logged.
func buildLAGSpecs(t *testing.T, dut *ondatra.DUTDevice) []lagSpec {
	t.Helper()
	unused, root := fetchUnusedPhysicalInterfaces(t, dut)
	states := interfaceStates(t, dut, root)
	pairs, singles := pairBySpeedGroup(t, unused, states)
	names := unusedLAGNames(t, dut, numLAGIntfs, states, interfaceIDs(t, dut))
	numTwo := min(numTwoMemberLAGs, len(pairs), len(unused)*numTwoMemberLAGs/numMemberIntfs, len(names)*numTwoMemberLAGs/numLAGIntfs)
	numOne := min(numOneMemberLAGs, len(unused)-2*numTwo, len(names)-numTwo)
	if numTwo == 0 || numOne == 0 {
		t.Fatalf("DUT %s has %d unused physical interfaces, which form %d pairs of interfaces with the same speed, and %d LAG interfaces that the test can configure: the test needs at least one LAG with 2 member interfaces and one LAG with 1 member interface", dut.Name(), len(unused), len(pairs), len(names))
	}
	if numTwo < numTwoMemberLAGs || numOne < numOneMemberLAGs {
		t.Logf("DUT %s has %d unused physical interfaces, which form %d pairs of interfaces with the same speed, and %d LAG interfaces that the test can configure, so the test environment is reduced from the scale of the README: %d LAGs with 2 member interfaces instead of %d, and %d LAGs with 1 member interface instead of %d", dut.Name(), len(unused), len(pairs), len(names), numTwo, numTwoMemberLAGs, numOne, numOneMemberLAGs)
	}

	// The LAGs with two member interfaces get the first pairs, the LAGs with one
	// member interface get the first of the other interfaces.
	members := make([][]string, 0, numTwo+numOne)
	members = append(members, pairs[:numTwo]...)
	var others []string
	for _, pair := range pairs[numTwo:] {
		others = append(others, pair...)
	}
	others = append(others, singles...)
	sort.Strings(others)
	for _, intf := range others[:numOne] {
		members = append(members, []string{intf})
	}

	numLAGs := len(members)
	ipv4s, err := iputil.GenerateIPsWithStep(vlanIPv4Start, numVLANs, vlanIPv4Step)
	if err != nil {
		t.Fatalf("Failed to generate the IPv4 addresses of the VLAN subinterfaces: %v", err)
	}
	ipv6s, err := iputil.GenerateIPv6sWithStep(vlanIPv6Start, numVLANs, vlanIPv6Step)
	if err != nil {
		t.Fatalf("Failed to generate the IPv6 addresses of the VLAN subinterfaces: %v", err)
	}

	lags := make([]lagSpec, 0, numLAGs)
	for i, name := range names[:numLAGs] {
		lag := lagSpec{
			name: name,
			desc: fmt.Sprintf("SYS-5.1 LAG %s", name),
		}
		for _, member := range members[i] {
			lag.members = append(lag.members, memberSpec{
				name: member,
				desc: fmt.Sprintf("SYS-5.1 member %s of %s", member, name),
			})
		}
		lags = append(lags, lag)
	}
	for i := range numVLANs {
		vlanID := uint16(i + 1)
		lag := &lags[i%numLAGs]
		lag.vlans = append(lag.vlans, vlanSpec{
			id:   vlanID,
			ipv4: ipv4s[i],
			ipv6: ipv6s[i],
		})
	}
	return lags
}

// interfaceStates merges the oper-status, admin-status, enabled state, and port
// speed of the interfaces of the DUT into root, which already holds their
// hardware-port and LAG (aggregate-id), and returns the interfaces keyed by
// name. Not all devices allow reading these leaves on all interfaces, so those
// paths are read separately and omitted if the DUT rejects them.
func interfaceStates(t *testing.T, dut *ondatra.DUTDevice, root *oc.Root) map[string]*oc.Interface {
	t.Helper()
	if root == nil || root.Interface == nil {
		return nil
	}
	intfs := gnmi.OC().InterfaceAny()
	client := newStateClient(t, dut)
	mergeOptionalInterfaceState(t, dut, client, root, intfs.OperStatus().State())
	mergeOptionalInterfaceState(t, dut, client, root, intfs.AdminStatus().State())
	mergeOptionalInterfaceState(t, dut, client, root, intfs.Enabled().State())
	if !deviations.PortSpeedUnsupported(dut) {
		mergeOptionalInterfaceState(t, dut, client, root, intfs.Ethernet().PortSpeed().State())
	}
	return root.Interface
}

// mergeOptionalInterfaceState reads q from the DUT and merges the result into
// dst, or logs the error if the DUT rejects q.
func mergeOptionalInterfaceState(t *testing.T, dut *ondatra.DUTDevice, client *ygnmi.Client, dst *oc.Root, q ygnmi.UntypedQuery) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), stateReadTimeout)
	defer cancel()
	batch := gnmi.OCBatch()
	batch.AddPaths(queryPaths(q)...)
	v, err := ygnmi.Lookup(ctx, client, batch.State())
	if err != nil {
		t.Logf("DUT %s does not report %v on its interfaces: %v", dut.Name(), q, err)
		return
	}
	if src, present := v.Val(); present {
		if err := ygot.MergeStructInto(dst, src); err != nil {
			t.Fatalf("Failed to merge the interface state read from %s: %v", dut.Name(), err)
		}
	}
}

// speedGroup returns the group of the interfaces that the interface named name,
// with state intf, is known to share its speed with. Two interfaces of the same
// group can be the members of a LAG, while devices can reject a LAG whose
// members have different speeds. The interfaces that are up and report the
// same known port speed are in the same group, and so are the channels of a
// breakout port, which report the same hardware-port. The port speed that an
// interface that is down reports does not necessarily match the speed that the
// device checks for the members of a LAG, so such an interface is only grouped
// with the other channels of its port, unless no interfaces are known to share
// their speed, see pairBySpeedGroup.
func speedGroup(name string, intf *oc.Interface) string {
	speed := intf.GetEthernet().GetPortSpeed()
	if intf.GetOperStatus() == oc.Interface_OperStatus_UP && knownSpeed(speed) {
		return fmt.Sprintf("up at %v", speed)
	}
	if hwPort := intf.GetHardwarePort(); hwPort != "" {
		return "hardware-port " + hwPort
	}
	// Without a hardware-port, the interface is not grouped with others.
	return "interface " + name
}

// reportedSpeedGroup returns the group of the interfaces that report the same
// known port speed as the interface named name, with state intf, whether they
// are up or down. An interface that does not report a known port speed is not
// grouped with others.
func reportedSpeedGroup(name string, intf *oc.Interface) string {
	if speed := intf.GetEthernet().GetPortSpeed(); knownSpeed(speed) {
		return fmt.Sprintf("reported port speed %v", speed)
	}
	return "interface " + name
}

// interfaceNamePrefix returns the group of the interfaces that have the same
// name prefix before the first digit as name, such as "et-" for "et-0/0/0".
func interfaceNamePrefix(name string) string {
	for i := 0; i < len(name); i++ {
		if name[i] >= '0' && name[i] <= '9' {
			if i > 0 {
				return "name prefix " + name[:i]
			}
			break
		}
	}
	return "interface " + name
}

// disabledInterfaceNamePrefix returns the group of the interfaces that are
// administratively down and have the same name prefix before the first digit as
// name, see interfaceNamePrefix. Interfaces that are not administratively down
// can have custom speeds configured in the base configuration of the testbed,
// whereas the unused interfaces that the base configuration leaves
// administratively down keep the default speed of their interface type.
func disabledInterfaceNamePrefix(name string, intf *oc.Interface) string {
	if intf != nil && (intf.GetAdminStatus() == oc.Interface_AdminStatus_DOWN || (intf.GetAdminStatus() == oc.Interface_AdminStatus_UNSET && intf.Enabled != nil && !intf.GetEnabled())) {
		return "administratively down with " + interfaceNamePrefix(name)
	}
	return "interface " + name
}

// knownSpeed reports whether speed is a known port speed.
func knownSpeed(speed oc.E_IfEthernet_ETHERNET_SPEED) bool {
	return speed != oc.IfEthernet_ETHERNET_SPEED_UNSET && speed != oc.IfEthernet_ETHERNET_SPEED_SPEED_UNKNOWN
}

// pairBySpeedGroup splits intfs, sorted by name, into pairs of interfaces of
// the same speed group (see speedGroup) and the interfaces left without a pair,
// sorted by name. states are the states of the interfaces of the DUT. If no
// interfaces are known to have the same speed, for example because the unused
// interfaces of the DUT are down and are not channels of a breakout port, the
// interfaces that report the same port speed are paired instead (see
// reportedSpeedGroup), or if no interfaces report the same port speed, the
// administratively down interfaces with the same name prefix (see
// disabledInterfaceNamePrefix) or the interfaces with the same name prefix (see
// interfaceNamePrefix), as the test needs LAGs with 2 member interfaces.
func pairBySpeedGroup(t *testing.T, intfs []string, states map[string]*oc.Interface) ([][]string, []string) {
	t.Helper()
	pairs, singles := pairByGroup(t, intfs, func(name string) string { return speedGroup(name, states[name]) })
	if len(pairs) == 0 {
		t.Logf("No unused physical interfaces are known to have the same speed, pairing the interfaces that report the same port speed")
		pairs, singles = pairByGroup(t, intfs, func(name string) string { return reportedSpeedGroup(name, states[name]) })
	}
	if len(pairs) == 0 {
		t.Logf("No unused physical interfaces report the same port speed, pairing the administratively down interfaces with the same name prefix")
		pairs, singles = pairByGroup(t, intfs, func(name string) string { return disabledInterfaceNamePrefix(name, states[name]) })
	}
	if len(pairs) == 0 {
		t.Logf("No administratively down interfaces share a name prefix, pairing the interfaces with the same name prefix")
		pairs, singles = pairByGroup(t, intfs, interfaceNamePrefix)
	}
	return pairs, singles
}

// pairByGroup splits intfs, sorted by name, into pairs of interfaces of the
// same group, as returned by group for the name of an interface, and the
// interfaces left without a pair, sorted by name. The groups that provide pairs
// are logged.
func pairByGroup(t *testing.T, intfs []string, group func(string) string) ([][]string, []string) {
	t.Helper()
	var groupNames []string
	groups := make(map[string][]string)
	for _, intf := range intfs {
		g := group(intf)
		if _, ok := groups[g]; !ok {
			groupNames = append(groupNames, g)
		}
		groups[g] = append(groups[g], intf)
	}
	var (
		pairs   [][]string
		singles []string
	)
	for _, g := range groupNames {
		groupIntfs := groups[g]
		if len(groupIntfs) >= 2 {
			t.Logf("Interfaces with the same speed (%s): %v", g, groupIntfs)
		}
		for ; len(groupIntfs) >= 2; groupIntfs = groupIntfs[2:] {
			pairs = append(pairs, []string{groupIntfs[0], groupIntfs[1]})
		}
		singles = append(singles, groupIntfs...)
	}
	sort.Strings(singles)
	return pairs, singles
}

// unusedLAGNames returns the names of the n LAG interfaces with the lowest
// indices that are neither interfaces in states, the state of the interfaces of
// the DUT, nor the LAG of an interface in states, so that the test does not
// modify existing LAGs. A device can derive the id of a LAG interface
// (/interfaces/interface/state/id) from the number in its name, and reject the
// LAG interface if another interface already has this id, so the names whose
// number is in ids, the ids of the interfaces of the DUT, are skipped too. The
// names are those of the entity naming library, which also knows the highest
// index of the LAG interfaces of a device: fewer than n names are returned if
// the device does not have n such LAG interfaces.
func unusedLAGNames(t *testing.T, dut *ondatra.DUTDevice, n int, states map[string]*oc.Interface, ids map[uint32]bool) []string {
	t.Helper()
	vendor, ok := namingVendors[dut.Vendor()]
	if !ok {
		t.Fatalf("The entity naming library does not support the vendor %v of DUT %s", dut.Vendor(), dut.Name())
	}
	params := &entname.DeviceParams{Vendor: vendor, HardwareModel: dut.Model()}
	used := make(map[string]bool)
	for name, intf := range states {
		used[name] = true
		if lag := intf.GetEthernet().GetAggregateId(); lag != "" {
			used[lag] = true
		}
	}
	var (
		names      []string
		idConflict []string
	)
	for idx := 0; len(names) < n; idx++ {
		name, err := entname.AggregateInterface(params, idx)
		if err != nil {
			t.Logf("DUT %s has %d LAG interfaces that the test can configure, there is no LAG interface with index %d: %v", dut.Name(), len(names), idx, err)
			break
		}
		switch {
		case used[name]:
		case ids[trailingNumber(name)]:
			idConflict = append(idConflict, name)
		default:
			names = append(names, name)
		}
	}
	if len(idConflict) > 0 {
		t.Logf("DUT %s: skipping the LAG interfaces %v, whose number is the id of another interface", dut.Name(), idConflict)
	}
	return names
}

// interfaceIDs returns the ids (/interfaces/interface/state/id) of the
// interfaces of the DUT. Not all devices support the id leaf: if the DUT
// rejects it, no id is returned.
func interfaceIDs(t *testing.T, dut *ondatra.DUTDevice) map[uint32]bool {
	t.Helper()
	ids := make(map[uint32]bool)
	ctx, cancel := context.WithTimeout(context.Background(), stateReadTimeout)
	defer cancel()
	vals, err := ygnmi.LookupAll(ctx, newStateClient(t, dut), gnmi.OC().InterfaceAny().Id().State())
	if err != nil {
		t.Logf("DUT %s does not report the ids of its interfaces: %v", dut.Name(), err)
		return ids
	}
	for _, v := range vals {
		if id, present := v.Val(); present {
			ids[id] = true
		}
	}
	return ids
}

// trailingNumber returns the number at the end of name, for example 31 for
// "lag31", or 0 if name does not end with a number.
func trailingNumber(name string) uint32 {
	start := len(name)
	for start > 0 && name[start-1] >= '0' && name[start-1] <= '9' {
		start--
	}
	number, err := strconv.ParseUint(name[start:], 10, 32)
	if err != nil {
		return 0
	}
	return uint32(number)
}

// interfaceConfigs reads qs, config queries of leaves of all the interfaces of
// the DUT, with the gNMI Get RPC, and returns the configuration read, keyed by
// interface name, in a map that is never nil. Each query is read with its own
// Get, so that a path without data does not hide the data of the others: a Get
// of several paths can fail with NOT_FOUND if one of them has no data, and
// ygnmi then reports no data for any of them.
func interfaceConfigs(t *testing.T, dut *ondatra.DUTDevice, qs ...ygnmi.UntypedQuery) map[string]*oc.Interface {
	t.Helper()
	merged := &oc.Root{}
	for _, q := range qs {
		batch := gnmi.OCBatch()
		batch.AddPaths(queryPaths(q)...)
		root, present := lookupWithRetry(t, getOpts(dut), batch.Config()).Val()
		if !present {
			continue
		}
		if err := ygot.MergeStructInto(merged, root); err != nil {
			t.Fatalf("Failed to merge the interface configuration read from %s: %v", dut.Name(), err)
		}
	}
	if merged.Interface == nil {
		return make(map[string]*oc.Interface)
	}
	return merged.Interface
}

// lookupWithRetry runs gnmi.Lookup(t, opts, q) and retries up to
// gnmiGetAttempts times if it fails, so that a transient RPC error (such as a
// temporary TACACS+ authentication failure during a sequence of many gNMI Gets)
// does not abort the test.
func lookupWithRetry[T any](t *testing.T, opts *gnmi.Opts, q ygnmi.SingletonQuery[T]) *ygnmi.Value[T] {
	t.Helper()
	var (
		val    *ygnmi.Value[T]
		errMsg *string
	)
	for attempt := 1; attempt <= gnmiGetAttempts; attempt++ {
		errMsg = testt.CaptureFatal(t, func(t testing.TB) {
			val = gnmi.Lookup(t, opts, q)
		})
		if errMsg == nil {
			return val
		}
		if attempt < gnmiGetAttempts {
			t.Logf("gNMI Lookup attempt %d/%d failed, retrying in %v: %s", attempt, gnmiGetAttempts, gnmiGetRetryDelay, *errMsg)
			time.Sleep(gnmiGetRetryDelay)
		}
	}
	t.Fatalf("%s", *errMsg)
	return nil
}

// queryPaths returns the paths of the queries qs, to read them with a gNMI
// batch. The paths of the queries, rather than the paths of the leaves, are
// read, because the path of a leaf of the compressed schema has a wildcard in
// place of the config or state container, which devices can reject.
func queryPaths(qs ...ygnmi.UntypedQuery) []ygnmi.PathStruct {
	paths := make([]ygnmi.PathStruct, 0, len(qs))
	for _, q := range qs {
		paths = append(paths, q.PathStruct())
	}
	return paths
}

// snapshotMembers returns the original configuration of the member interfaces
// of lags that the test changes. The configuration of each member interface is
// read with its own gNMI Get, because a device can return the configuration of
// all the interfaces at the path of the interfaces container, which is
// compressed out of the schema of the test and cannot be unmarshalled.
func snapshotMembers(t *testing.T, dut *ondatra.DUTDevice, lags []lagSpec) []memberSnapshot {
	t.Helper()
	var (
		snapshots  []memberSnapshot
		configured int
	)
	for _, lag := range lags {
		for _, member := range lag.members {
			s := memberSnapshot{name: member.name}
			if intf, present := lookupWithRetry(t, getOpts(dut), gnmi.OC().Interface(member.name).Config()).Val(); present {
				s.configured = true
				s.desc = intf.Description
				s.enabled = intf.Enabled
				configured++
			}
			snapshots = append(snapshots, s)
		}
	}
	t.Logf("DUT %s: %d of the %d member interfaces of the test environment are configured", dut.Name(), configured, len(snapshots))
	return snapshots
}

// numMembers returns the number of member interfaces of lags.
func numMembers(lags []lagSpec) int {
	n := 0
	for _, lag := range lags {
		n += len(lag.members)
	}
	return n
}

// numVLANSubintfs returns the number of VLAN subinterfaces of lags.
func numVLANSubintfs(lags []lagSpec) int {
	n := 0
	for _, lag := range lags {
		n += len(lag.vlans)
	}
	return n
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

// addDefaultNetworkInstanceInterfaceOps adds to b, for every VLAN subinterface
// of lags, the operation that attaches the subinterface to the default network
// instance or, if isDelete is true, detaches it.
func addDefaultNetworkInstanceInterfaceOps(b *gnmi.SetBatch, dut *ondatra.DUTDevice, lags []lagSpec, isDelete bool) {
	if !deviations.ExplicitInterfaceInDefaultVRF(dut) {
		return
	}
	ni := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	for _, lag := range lags {
		for _, vlan := range lag.vlans {
			id := fmt.Sprintf("%s.%d", lag.name, vlan.id)
			if isDelete {
				gnmi.BatchDelete(b, ni.Interface(id).Config())
				continue
			}
			gnmi.BatchUpdate(b, ni.Interface(id).Config(), &oc.NetworkInstance_Interface{
				Id:           ygot.String(id),
				Interface:    ygot.String(lag.name),
				Subinterface: ygot.Uint32(uint32(vlan.id)),
			})
		}
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

// getOpts returns gNMI options that read the configuration with the gNMI Get
// RPC on all devices, as SYS-5.1.2 Steps 2 and 5 require.
func getOpts(dut *ondatra.DUTDevice) *gnmi.Opts {
	return dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithUseGet())
}

// newStateClient returns a ygnmi client of the DUT, for the reads of its state
// that need a deadline, which the reads of the gnmi package do not have.
func newStateClient(t testing.TB, dut *ondatra.DUTDevice) *ygnmi.Client {
	t.Helper()
	c, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.Name()))
	if err != nil {
		t.Fatalf("Failed to create a gNMI client of %s: %v", dut.Name(), err)
	}
	return c
}

// awaitEnvInState waits until the DUT reports in its state all the LAG and
// member interfaces of lags, which shows that the test environment is in
// place. The type of a LAG interface is only checked if the DUT reports it. The
// state is read every statePollInterval with a gNMI Subscribe ONCE request,
// rather than with a single STREAM subscription, so that the interfaces that
// the DUT adds to its state after the start of the wait are seen as well. Each
// read is bounded by stateReadTimeout and a failed read is retried, so that a
// read that the DUT leaves without a response, for example while it boots up,
// does not block the test. The test fails, with the interfaces that are
// missing, if the DUT does not report them within timeout.
func awaitEnvInState(t *testing.T, dut *ondatra.DUTDevice, lags []lagSpec, timeout time.Duration) {
	t.Helper()
	intfs := gnmi.OC().InterfaceAny()
	batch := gnmi.OCBatch()
	batch.AddPaths(queryPaths(intfs.Name().State(), intfs.Type().State())...)
	start := time.Now()
	for attempt := 1; ; attempt++ {
		var problems []string
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			client := newStateClient(t, dut)
			ctx, cancel := context.WithTimeout(context.Background(), stateReadTimeout)
			defer cancel()
			v, err := ygnmi.Lookup(ctx, client, batch.State())
			if err != nil {
				t.Fatalf("%v", err)
			}
			root, _ := v.Val()
			problems = envStateProblems(root, lags)
		}); errMsg != nil {
			problems = []string{"failed to read the state: " + *errMsg}
		}
		elapsed := time.Since(start).Round(time.Second)
		if len(problems) == 0 {
			t.Logf("DUT %s reports the %d LAG and %d member interfaces of the test environment in its state (%v elapsed)", dut.Name(), len(lags), numMembers(lags), elapsed)
			return
		}
		if elapsed >= timeout {
			shown := min(len(problems), maxReportedMismatches)
			t.Fatalf("DUT %s does not report %d interfaces of the test environment as expected in its state after %v, the first %d are:\n%s", dut.Name(), len(problems), elapsed, shown, strings.Join(problems[:shown], "\n"))
		}
		t.Logf("Attempt %d: DUT %s does not report %d interfaces of the test environment as expected in its state yet (%v elapsed), for example %s", attempt, dut.Name(), len(problems), elapsed, problems[0])
		time.Sleep(statePollInterval)
	}
}

// envStateProblems returns how root, the state read from the DUT, differs from
// the interfaces of lags, see awaitEnvInState.
func envStateProblems(root *oc.Root, lags []lagSpec) []string {
	var problems []string
	for _, lag := range lags {
		intf := root.GetInterface(lag.name)
		if intf == nil {
			problems = append(problems, lag.name+": not reported")
		} else if intfType := intf.GetType(); intfType != oc.IETFInterfaces_InterfaceType_UNSET && intfType != oc.IETFInterfaces_InterfaceType_ieee8023adLag {
			problems = append(problems, fmt.Sprintf("%s: type %v, want %v", lag.name, intfType, oc.IETFInterfaces_InterfaceType_ieee8023adLag))
		}
		for _, member := range lag.members {
			if root.GetInterface(member.name) == nil {
				problems = append(problems, member.name+": not reported")
			}
		}
	}
	return problems
}

// restoreConfig removes the configuration that the test added, except the type
// of the default network instance (see setupEnvironment), and restores the
// original configuration of the member interfaces, with a single gNMI Set.
func restoreConfig(t *testing.T, dut *ondatra.DUTDevice, lags []lagSpec, snapshots []memberSnapshot) {
	t.Helper()
	t.Logf("Restoring the original configuration of %s", dut.Name())
	sb := &gnmi.SetBatch{}
	addDefaultNetworkInstanceInterfaceOps(sb, dut, lags, true)
	deleteScaleLAGs(sb, lags)
	for _, s := range snapshots {
		s.addRestoreOps(sb)
	}
	sb.Set(t, dut)
}

// newLargeBatch returns the gNMI batch of SYS-5.1.1 Step 1: a description on
// all the member (physical) and LAG interfaces of lags, and 700 802.1Q VLAN
// subinterfaces (VLAN IDs 1..700) with IPv4 and IPv6 addresses distributed
// across the LAG interfaces.
func newLargeBatch(dut *ondatra.DUTDevice, lags []lagSpec) *gnmi.SetBatch {
	sb := &gnmi.SetBatch{}
	for _, lag := range lags {
		for _, member := range lag.members {
			gnmi.BatchUpdate(sb, gnmi.OC().Interface(member.name).Description().Config(), member.desc)
		}
		intf := &oc.Interface{
			Name:        ygot.String(lag.name),
			Description: ygot.String(lag.desc),
			Type:        oc.IETFInterfaces_InterfaceType_ieee8023adLag,
		}
		if deviations.InterfaceEnabled(dut) {
			intf.Enabled = ygot.Bool(true)
		}
		if deviations.RequireRoutedSubinterface0(dut) {
			s0 := intf.GetOrCreateSubinterface(0)
			if deviations.InterfaceEnabled(dut) {
				if !deviations.IPv4MissingEnabled(dut) {
					s0.GetOrCreateIpv4().Enabled = ygot.Bool(true)
				}
				s0.GetOrCreateIpv6().Enabled = ygot.Bool(true)
			}
		}
		for _, vlan := range lag.vlans {
			subif := intf.GetOrCreateSubinterface(uint32(vlan.id))
			if deviations.InterfaceEnabled(dut) {
				subif.Enabled = ygot.Bool(true)
			}
			if deviations.DeprecatedVlanID(dut) {
				subif.GetOrCreateVlan().VlanId = oc.UnionUint16(vlan.id)
			} else {
				subif.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(vlan.id)
			}
			ipv4 := subif.GetOrCreateIpv4()
			if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
				ipv4.Enabled = ygot.Bool(true)
			}
			ipv4.GetOrCreateAddress(vlan.ipv4).PrefixLength = ygot.Uint8(vlanIPv4Len)
			ipv6 := subif.GetOrCreateIpv6()
			if deviations.InterfaceEnabled(dut) {
				ipv6.Enabled = ygot.Bool(true)
			}
			ipv6.GetOrCreateAddress(vlan.ipv6).PrefixLength = ygot.Uint8(vlanIPv6Len)
		}
		gnmi.BatchUpdate(sb, gnmi.OC().Interface(lag.name).Config(), intf)
	}
	// On devices that require it, attach the VLAN subinterfaces of every LAG,
	// which hold the IP addresses, to the default network instance.
	addDefaultNetworkInstanceInterfaceOps(sb, dut, lags, false)
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
	// IPv6 addresses on all the 500 LAG interfaces. These are the numbers of the
	// README scale; the batch covers all the interfaces of the test environment,
	// which can be scaled down, see buildLAGSpecs.
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
	// The wait can start while the DUT is still shutting down, so a boot-time
	// within bootTimeTolerance of the boot-time before the reboot is not taken
	// for a new boot.
	env.postRebootBootTime = awaitRebootComplete(t, dut, env.preRebootBootTime+uint64(bootTimeTolerance), rebootTimeout)
	// The DUT is back in service once it also reports the interfaces of the test
	// environment in its state again.
	awaitEnvInState(t, dut, env.lags, serviceTimeout)

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
		if diff := max(bootTime, env.postRebootBootTime) - min(bootTime, env.postRebootBootTime); time.Duration(diff) > bootTimeTolerance {
			t.Errorf("boot-time got %d, want %d (within %v), the boot-time when the DUT booted up in SYS-5.1.1 Step 4: the DUT rebooted again", bootTime, env.postRebootBootTime, bootTimeTolerance)
		}
		if deviations.SwVersionUnsupported(dut) {
			t.Log("Skipping the software version verification: deviation sw_version_unsupported")
			return
		}
		// Software components can be reported some time after the DUT returns to
		// service, so the software versions are read again until the DUT reports
		// all the OPERATING_SYSTEM components of before the reboot, for up to
		// swVersionTimeout.
		osType := oc.PlatformTypes_OPENCONFIG_SOFTWARE_COMPONENT_OPERATING_SYSTEM
		got := swVersionsByType(t, dut, swComponentTypes...)
		for start := time.Now(); len(missingComponents(env.preRebootSWVersions[osType], got[osType])) > 0 && time.Since(start) < swVersionTimeout; {
			t.Logf("The DUT does not report the OPERATING_SYSTEM components %v after the reboot yet, reading the software versions again in %v", missingComponents(env.preRebootSWVersions[osType], got[osType]), statePollInterval)
			time.Sleep(statePollInterval)
			got = swVersionsByType(t, dut, swComponentTypes...)
		}
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
		if len(got[osType]) == 0 {
			t.Errorf("the DUT reports no OPERATING_SYSTEM software component")
		}
		// A cold reboot does not change the software, so a component that the DUT
		// reports both before and after the reboot must report the same version.
		// A component that the DUT does not report after the reboot, for example a
		// component of a standby control processor that has not reported all its
		// components yet, is logged.
		var changed []string
		for _, swType := range swComponentTypes {
			for name, before := range env.preRebootSWVersions[swType] {
				after, ok := got[swType][name]
				switch {
				case !ok:
					t.Logf("Software component %s of type %v, with version %q before the reboot, is not reported after the reboot", name, swType, before)
				case after != before:
					changed = append(changed, fmt.Sprintf("%s of type %v: %q before the reboot, %q after", name, swType, before, after))
				}
			}
		}
		if len(changed) > 0 {
			t.Errorf("software versions changed across the reboot:\n%s", strings.Join(changed, "\n"))
		}
	})

	// SYS-5.1.2 Step 2: Using gNMI Get, verify that the interface descriptions,
	// VLAN subinterfaces, and IP addresses applied in SYS-5.1.1 are present in
	// the configuration.
	t.Run("Step 2: descriptions and IP addresses in the configuration", func(t *testing.T) {
		intfs := gnmi.OC().InterfaceAny()
		vlanIDQuery := ygnmi.UntypedQuery(intfs.SubinterfaceAny().Vlan().Match().SingleTagged().VlanId().Config())
		if deviations.DeprecatedVlanID(dut) {
			vlanIDQuery = intfs.SubinterfaceAny().Vlan().VlanId().Config()
		}
		root := &oc.Root{Interface: interfaceConfigs(t, dut,
			intfs.Description().Config(),
			vlanIDQuery,
			intfs.SubinterfaceAny().Ipv4().AddressAny().Ip().Config(),
			intfs.SubinterfaceAny().Ipv4().AddressAny().PrefixLength().Config(),
			intfs.SubinterfaceAny().Ipv6().AddressAny().Ip().Config(),
			intfs.SubinterfaceAny().Ipv6().AddressAny().PrefixLength().Config(),
		)}
		var (
			mismatches []string
			reread     int
			// absent are the interfaces that have no configuration at all.
			absent []string
		)
		for _, lag := range env.lags {
			lagMismatches := lagConfigMismatches(root, lag)
			if len(lagMismatches) > 0 {
				// A device can return less configuration for a Get of all the
				// interfaces than for a Get of one interface, so the configuration
				// of the interfaces of the LAG is read again, one interface at a
				// time, before the differences are reported.
				for _, name := range lagInterfaceNames(lag) {
					if intf, present := lookupWithRetry(t, getOpts(dut), gnmi.OC().Interface(name).Config()).Val(); present {
						root.Interface[name] = intf
					} else {
						absent = append(absent, name)
					}
				}
				reread++
				lagMismatches = lagConfigMismatches(root, lag)
			}
			mismatches = append(mismatches, lagMismatches...)
		}
		if reread > 0 {
			t.Logf("The Gets of all the interfaces did not return the expected configuration of %d LAGs or of their member interfaces, which were read again one interface at a time: %d of these interfaces have no configuration, for example %v", reread, len(absent), absent[:min(len(absent), maxReportedMismatches)])
		}
		if n := len(mismatches); n > 0 {
			shown := min(n, maxReportedMismatches)
			t.Errorf("%d interface descriptions, VLAN subinterfaces, or IP addresses applied in SYS-5.1.1 are missing or wrong in the configuration after the reboot, the first %d are:\n%s", n, shown, strings.Join(mismatches[:shown], "\n"))
			logStateDescriptions(t, dut, env.lags)
			return
		}
		t.Logf("The %d interface descriptions, %d VLAN subinterfaces, and %d IP addresses applied in SYS-5.1.1 are in the configuration", numMembers(env.lags)+len(env.lags), numVLANSubintfs(env.lags), 2*numVLANSubintfs(env.lags))
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
		if got, present := lookupWithRetry(t, getOpts(dut), descPath.Config()).Val(); !present || got != testDescription {
			t.Errorf("SYS-5.1.2 Step 5: configured description of %s got %q (present=%v), want %q", intf, got, present, testDescription)
		}
		// The applied description is also expected in the state, streamed with an
		// ON_CHANGE subscription (or read with a ONCE subscription or gNMI Get if
		// the ON_CHANGE stream does not emit an initial sync after the update).
		start := time.Now()
		onChange := dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE))
		if got, ok := gnmi.Watch(t, onChange, descPath.State(), onChangeWatchTimeout, func(v *ygnmi.Value[string]) bool {
			desc, present := v.Val()
			return present && desc == testDescription
		}).Await(t); !ok {
			var (
				desc    string
				present bool
			)
			for time.Since(start) < stateTimeout {
				if desc, present = lookupWithRetry(t, dut.GNMIOpts(), descPath.State()).Val(); present && desc == testDescription {
					break
				}
				if desc, present = lookupWithRetry(t, getOpts(dut), descPath.State()).Val(); present && desc == testDescription {
					break
				}
				time.Sleep(statePollInterval)
			}
			if !present || desc != testDescription {
				t.Errorf("SYS-5.1.2 Step 5: state description of %s got %v (last polled: %q, present=%v), want %q within %v", intf, got, desc, present, testDescription, stateTimeout)
			}
		}
	})
}

// missingComponents returns the sorted names of the software components in
// before, the versions keyed by component name before the reboot, that are not
// in after, the versions after the reboot.
func missingComponents(before, after map[string]string) []string {
	var missing []string
	for name := range before {
		if _, ok := after[name]; !ok {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

// lagConfigMismatches returns the differences between the descriptions, VLAN
// subinterfaces, and IP addresses of the LAG and of its member interfaces in
// root, the configuration read from the DUT, and the ones that SYS-5.1.1 Step 1
// configures on them.
func lagConfigMismatches(root *oc.Root, lag lagSpec) []string {
	var mismatches []string
	for _, member := range lag.members {
		if got := root.GetInterface(member.name).GetDescription(); got != member.desc {
			mismatches = append(mismatches, fmt.Sprintf("%s: description got %q, want %q", member.name, got, member.desc))
		}
	}
	if got := root.GetInterface(lag.name).GetDescription(); got != lag.desc {
		mismatches = append(mismatches, fmt.Sprintf("%s: description got %q, want %q", lag.name, got, lag.desc))
	}
	return append(mismatches, vlanMismatches(root, lag)...)
}

// lagInterfaceNames returns the names of the LAG interface and of the member
// interfaces of lag.
func lagInterfaceNames(lag lagSpec) []string {
	names := []string{lag.name}
	for _, member := range lag.members {
		names = append(names, member.name)
	}
	return names
}

// logStateDescriptions logs how many of the LAG and member interfaces of lags
// report in their state the description that SYS-5.1.1 Step 1 configures on
// them, which shows whether the configuration that SYS-5.1.2 Step 2 does not
// find is in effect on the DUT.
func logStateDescriptions(t *testing.T, dut *ondatra.DUTDevice, lags []lagSpec) {
	t.Helper()
	batch := gnmi.OCBatch()
	batch.AddPaths(queryPaths(gnmi.OC().InterfaceAny().Description().State())...)
	root, _ := gnmi.Lookup(t, dut, batch.State()).Val()
	var got, want int
	for _, lag := range lags {
		want += 1 + len(lag.members)
		if root.GetInterface(lag.name).GetDescription() == lag.desc {
			got++
		}
		for _, member := range lag.members {
			if root.GetInterface(member.name).GetDescription() == member.desc {
				got++
			}
		}
	}
	t.Logf("DUT %s reports in its state the description applied in SYS-5.1.1 on %d of the %d LAG and member interfaces", dut.Name(), got, want)
}

// vlanMismatches returns the differences between the VLAN IDs and IPv4/IPv6
// addresses of the VLAN subinterfaces of the LAG in root, the configuration
// read from the DUT, and the ones that SYS-5.1.1 Step 1 configures on the LAG.
func vlanMismatches(root *oc.Root, lag lagSpec) []string {
	var mismatches []string
	intf := root.GetInterface(lag.name)
	for _, vlan := range lag.vlans {
		sub := intf.GetSubinterface(uint32(vlan.id))
		if got := configuredVLANID(sub); got != vlan.id {
			mismatches = append(mismatches, fmt.Sprintf("%s.%d: VLAN ID got %d, want %d", lag.name, vlan.id, got, vlan.id))
		}
		var ipv4Addrs map[string]*oc.Interface_Subinterface_Ipv4_Address
		if ipv4 := sub.GetIpv4(); ipv4 != nil {
			ipv4Addrs = ipv4.Address
		}
		if got, ok := prefixLength(ipv4Addrs, vlan.ipv4); !ok || got != vlanIPv4Len {
			mismatches = append(mismatches, fmt.Sprintf("%s.%d: IPv4 address %s/%d got present %t, prefix-length %d", lag.name, vlan.id, vlan.ipv4, vlanIPv4Len, ok, got))
		}
		var ipv6Addrs map[string]*oc.Interface_Subinterface_Ipv6_Address
		if ipv6 := sub.GetIpv6(); ipv6 != nil {
			ipv6Addrs = ipv6.Address
		}
		if got, ok := prefixLength(ipv6Addrs, vlan.ipv6); !ok || got != vlanIPv6Len {
			mismatches = append(mismatches, fmt.Sprintf("%s.%d: IPv6 address %s/%d got present %t, prefix-length %d", lag.name, vlan.id, vlan.ipv6, vlanIPv6Len, ok, got))
		}
	}
	return mismatches
}

// configuredVLANID returns the 802.1Q VLAN ID configured on sub, either at
// vlan/match/single-tagged/config/vlan-id or at the deprecated
// vlan/config/vlan-id leaf.
func configuredVLANID(sub *oc.Interface_Subinterface) uint16 {
	if id := sub.GetVlan().GetMatch().GetSingleTagged().GetVlanId(); id != 0 {
		return id
	}
	if id, ok := sub.GetVlan().GetVlanId().(oc.UnionUint16); ok {
		return uint16(id)
	}
	return 0
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
// interfaces of the DUT that are not in use, and the interface state read from
// the DUT.
func fetchUnusedPhysicalInterfaces(t testing.TB, dut *ondatra.DUTDevice) ([]string, *oc.Root) {
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
	return unused, root
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
