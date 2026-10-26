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

// Package lacp_member_reboot_test implements RT-5.16: LACP Member Linecard
// Reboot.
//
// The test validates that a LACP bundle whose members span multiple linecards
// keeps forwarding traffic on the remaining member(s) while the linecard that
// hosts one member is rebooted with gnoi.system.Reboot, that the rebooted
// member rejoins the bundle after the linecard recovers, and that the bundle's
// cumulative traffic counters never decrement or reset to zero.
//
// Topology (featureprofiles/topologies/atedut_4.testbed):
//
//	ATE port1 ─┐               ┌─ DUT port1 (linecard A) ─┐
//	           ├── ATE LAG ════┤                          ├── DUT LAG (LACP)
//	ATE port2 ─┘               └─ DUT port2 (linecard B) ─┘
//	ATE port3 ──────────────────── DUT port3 (routed traffic source / sink)
//
// DUT port1 and DUT port2 must be on different linecards. The rebooted
// linecard is the one hosting a LAG member but not DUT port3, so that the
// non-LAG traffic port stays up for the whole test.
package lacp_member_reboot_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126

	// ATE LAG / LACP parameters.
	ateLagName          = "ateLag"
	ateLagSystemID      = "02:00:01:01:01:01"
	ateLagActorKey      = 1
	ateLagActorSysPrio  = 1
	ateLagActorPortPrio = 1

	// Traffic profile. UDP source and destination ports are incremented so
	// that the DUT (egress towards the ATE LAG) hashes the flow across all LAG
	// members. The OTG LAG transmits each flow on a single member port, so the
	// ATE -> DUT traffic is split into numLagIngressFlows flows (with distinct
	// UDP port ranges) that together send flowPPS, so that it is distributed
	// across all LAG members as well (README RT-5.16.1 Step 1).
	flowLagToPort3     = "flow-ateLag-to-atePort3"
	flowPort3ToLag     = "flow-atePort3-to-ateLag"
	numLagIngressFlows = 8
	flowPPS            = 1000
	flowPacketSize     = 256
	flowUDPSrcStart    = 10000
	flowUDPDstStart    = 20000
	flowUDPPortCount   = 1000

	// minPktDelta is the minimum number of packets a counter must grow by to
	// be considered "incrementing" with data traffic. It must be far above
	// the control-plane packets (LACP fast = 1 pps, ARP/ND) an interface can
	// send or receive within counterTimeout (~120 s => a few hundred), so
	// that LACP PDUs alone can never satisfy a check. At flowPPS spread over
	// two members (~500 pps per member per direction) it is reached in ~10 s.
	minPktDelta = 5000

	// Timeouts. All waits are event driven via gnmi.Watch / gnmi.Await; these
	// only bound how long we wait for the expected state.
	portUpTimeout       = 2 * time.Minute
	lacpTimeout         = 2 * time.Minute
	counterTimeout      = 2 * time.Minute
	trafficStopTimeout  = 1 * time.Minute
	memberDownTimeout   = 5 * time.Minute
	linecardBootTimeout = 20 * time.Minute
	// monitorMaxDuration bounds the bundle counter monitor which runs from
	// just before the linecard reboot (Step 3) until Step 5.
	monitorMaxDuration = memberDownTimeout + 2*linecardBootTimeout + 6*counterTimeout + 2*lacpTimeout
)

var (
	// DUT and ATE LAG addressing (RFC 5737 / RFC 3849).
	dutLag = attrs.Attributes{
		Desc:    "DUT LAG to ATE LAG",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::1",
		IPv6Len: ipv6PrefixLen,
	}
	ateLag = attrs.Attributes{
		Name:    ateLagName,
		MAC:     "02:00:01:01:01:02",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::2",
		IPv6Len: ipv6PrefixLen,
	}
	// MAC addresses for the ATE LAG member ports (port1, port2).
	ateLagMemberMACs = []string{"02:00:01:01:01:03", "02:00:01:01:01:04"}

	// DUT port3 / ATE port3 addressing (routed non-LAG port).
	dutPort3 = attrs.Attributes{
		Desc:    "DUT port3 to ATE port3",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::5",
		IPv6Len: ipv6PrefixLen,
	}
	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		MAC:     "02:00:03:01:01:01",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::6",
		IPv6Len: ipv6PrefixLen,
	}

	// flows lists the names of all traffic flows configured in Step 1: the
	// numLagIngressFlows ATE LAG -> ATE port3 flows and the ATE port3 -> ATE LAG
	// flow.
	flows = func() []string {
		var names []string
		for i := 0; i < numLagIngressFlows; i++ {
			names = append(names, lagIngressFlowName(i))
		}
		return append(names, flowPort3ToLag)
	}()
)

// lagIngressFlowName returns the name of the i-th ATE LAG -> ATE port3 flow.
func lagIngressFlowName(i int) string {
	return fmt.Sprintf("%s-%d", flowLagToPort3, i)
}

// testEnv holds the state shared by the RT-5.16.1 steps.
type testEnv struct {
	dut *ondatra.DUTDevice
	ate *ondatra.ATEDevice
	top gosnappi.Config

	aggID      string
	dutMembers []*ondatra.Port // DUT LAG member ports (port1, port2).
	ateMembers []*ondatra.Port // ATE LAG member ports (port1, port2).
	dutPort3   *ondatra.Port
	atePort3   *ondatra.Port

	// rebootLinecard is the linecard rebooted in Step 3.
	rebootLinecard string
	// rebootedMembers are the LAG members hosted on rebootLinecard.
	rebootedMembers []*ondatra.Port
	// remainingMembers are the LAG members on other linecards.
	remainingMembers []*ondatra.Port

	// preReboot holds the bundle and member counter snapshots taken right
	// before the linecard reboot in Step 3, keyed by interface name.
	preReboot map[string]*oc.Interface_Counters
	// bundleMonitor streams the bundle counters from Step 3 until Step 5.
	bundleMonitor *helpers.InterfaceCountersMonitor
	// rebootIssued records whether the gNOI reboot was sent, so cleanup can
	// wait for the linecard to recover if the test aborts.
	rebootIssued   bool
	trafficRunning bool
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestLACPMemberLinecardReboot implements RT-5.16.1: Traffic flow and stats
// validation during linecard reboot.
func TestLACPMemberLinecardReboot(t *testing.T) {
	env := setupEnvironment(t)

	// Cleanup functions run in LIFO order: this one (traffic and linecard
	// recovery) runs before the ATE protocol stop and the DUT config
	// reversion registered in setupEnvironment.
	t.Cleanup(func() {
		if env.trafficRunning {
			env.ate.OTG().StopTraffic(t)
		}
		// Leave the testbed usable for following tests: if the test
		// aborted mid-reboot, wait for the linecard to come back.
		if env.rebootIssued {
			if s, ok := components.AwaitOperStatus(t, env.dut, env.rebootLinecard, linecardBootTimeout, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE); !ok {
				t.Errorf("Cleanup: linecard %s did not return to ACTIVE within %v (last oper-status %v)", env.rebootLinecard, linecardBootTimeout, s)
			}
		}
	})

	steps := []struct {
		name string
		fn   func(t *testing.T, env *testEnv)
	}{
		{name: "RT-5.16.1 Step 1: Configure Traffic", fn: testConfigureTraffic},
		{name: "RT-5.16.1 Step 2: Verify Initial State", fn: testVerifyInitialState},
		{name: "RT-5.16.1 Step 3: Reboot Linecard", fn: testRebootLinecard},
		{name: "RT-5.16.1 Step 4: Linecard Recovery", fn: testLinecardRecovery},
		{name: "RT-5.16.1 Step 5: Verify Statistics Post-Reboot", fn: testStatisticsPostReboot},
	}
	// The steps are sequential and depend on each other; they must not run
	// in parallel, and a failed step stops the sequence so that, for
	// example, the linecard is not rebooted when the initial state is wrong.
	for _, s := range steps {
		if !t.Run(s.name, func(t *testing.T) {
			s.fn(t, env)
		}) {
			t.Fatalf("%s failed; skipping the remaining dependent steps", s.name)
		}
	}
}

// setupEnvironment implements the README "Test environment setup":
//   - Ensure the bundle member ports are on different linecards.
//   - Configure a LACP bundle on the DUT with at least two member ports.
//   - Configure the corresponding bundle on the ATE.
//   - Establish the LACP session between DUT and ATE.
//   - Configure IP addresses on the bundle interfaces in the default network
//     instance.
//
// It is called at the start of the test rather than from TestMain because
// Ondatra devices are only reserved once fptest.RunTests is running. The DUT
// configuration changes are reverted by a t.Cleanup registered here.
func setupEnvironment(t *testing.T) *testEnv {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	env := &testEnv{
		dut:        dut,
		ate:        ate,
		dutMembers: []*ondatra.Port{dut.Port(t, "port1"), dut.Port(t, "port2")},
		ateMembers: []*ondatra.Port{ate.Port(t, "port1"), ate.Port(t, "port2")},
		dutPort3:   dut.Port(t, "port3"),
		atePort3:   ate.Port(t, "port3"),
	}

	t.Log("Setup: ensure the bundle member ports are on different linecards and pick the linecard to reboot")
	selectRebootLinecard(t, env)

	// A run that is killed (e.g. by a timeout) never reaches its t.Cleanup;
	// remove what it left behind before taking the snapshot, so the snapshot
	// reflects the real baseline and the new LAG does not reuse the same IP.
	t.Log("Setup: remove configuration left behind by an earlier aborted run of this test")
	removeStaleTestConfig(t, env)

	t.Log("Setup: snapshot the DUT port configuration so it can be restored on cleanup")
	snapshot := cfgplugins.SnapshotInterfacesConfig(t, dut, allDUTPortNames(env))
	t.Cleanup(func() { revertDUTConfig(t, env, snapshot) })

	t.Log("Setup: configure the LACP bundle and IP addresses on the DUT")
	configureDUT(t, env)

	t.Log("Setup: configure the corresponding LACP bundle on the ATE")
	env.top = configureATE(t, env)
	env.ate.OTG().PushConfig(t, env.top)
	env.ate.OTG().StartProtocols(t)
	t.Cleanup(func() { env.ate.OTG().StopProtocols(t) })

	t.Log("Setup: verify the LACP session is established between DUT and ATE")
	if err := awaitLACPMembers(t, env, env.dutMembers, true, lacpTimeout); err != nil {
		t.Fatalf("Setup: LACP not established: %v", err)
	}
	gnmi.Await(t, dut, gnmi.OC().Interface(env.aggID).OperStatus().State(), lacpTimeout, oc.Interface_OperStatus_UP)
	return env
}

// selectRebootLinecard resolves the linecard hosting each DUT port and selects
// the linecard to reboot. The test is skipped on devices without linecards
// and fails if the LAG members are not on different linecards, since the
// README requires them to be.
func selectRebootLinecard(t *testing.T, env *testEnv) {
	t.Helper()
	lcOf, err := components.LinecardsFromPorts(t, env.dut, append([]*ondatra.Port{env.dutPort3}, env.dutMembers...))
	switch {
	case errors.Is(err, components.ErrNoLinecardComponents):
		t.Skipf("RT-5.16 requires a modular form factor (MFF) DUT: %v", err)
	case err != nil:
		t.Fatalf("Could not map the DUT ports to their linecards: %v", err)
	}
	for name, lc := range lcOf {
		t.Logf("DUT interface %s is hosted on linecard %s", name, lc)
	}

	m0, m1 := env.dutMembers[0], env.dutMembers[1]
	if lcOf[m0.Name()] == lcOf[m1.Name()] {
		t.Fatalf("LAG members %s and %s are both on linecard %s; RT-5.16 requires the bundle members to be on different linecards", m0.Name(), m1.Name(), lcOf[m0.Name()])
	}

	// Reboot the linecard of a LAG member that does not also host the
	// non-LAG traffic port (DUT port3), so traffic keeps a valid path.
	for _, m := range env.dutMembers {
		if lcOf[m.Name()] != lcOf[env.dutPort3.Name()] {
			env.rebootLinecard = lcOf[m.Name()]
			break
		}
	}
	for _, m := range env.dutMembers {
		if lcOf[m.Name()] == env.rebootLinecard {
			env.rebootedMembers = append(env.rebootedMembers, m)
		} else {
			env.remainingMembers = append(env.remainingMembers, m)
		}
	}
	t.Logf("Linecard to reboot: %s; rebooted members: %v; remaining members: %v", env.rebootLinecard, portNames(env.rebootedMembers), portNames(env.remainingMembers))
}

// configureDUT configures DUT port3 and the LACP bundle (with DUT port1 and
// DUT port2 as members) with IPv4/IPv6 addresses in the default network
// instance, using a single gNMI SetRequest. Deviations affecting the
// configuration are handled inside the cfgplugins functions.
func configureDUT(t *testing.T, env *testEnv) {
	t.Helper()
	dut := env.dut
	env.aggID = netutil.NextAggregateInterface(t, dut)
	t.Logf("Using aggregate interface %s", env.aggID)

	b := &gnmi.SetBatch{}
	// /interfaces/interface/config/{name,type} and IPs for DUT port3.
	gnmi.BatchReplace(b, gnmi.OC().Interface(env.dutPort3.Name()).Config(), dutPort3.NewOCInterface(env.dutPort3.Name(), dut))

	// Aggregate interface with lag-type LACP, LACP interface config and
	// member ports (/interfaces/interface/ethernet/config/aggregate-id).
	lacpActive := oc.Lacp_LacpActivityType_ACTIVE
	lacpFast := oc.Lacp_LacpPeriodType_FAST
	cfgplugins.NewAggregateInterface(t, dut, b, &cfgplugins.DUTAggData{
		Attributes:   dutLag,
		LagName:      env.aggID,
		OndatraPorts: env.dutMembers,
		AggType:      oc.IfAggregate_AggregationType_LACP,
		LacpParams:   &cfgplugins.LACPParams{Activity: &lacpActive, Period: &lacpFast},
	})
	// Default network instance membership (ExplicitInterfaceInDefaultVRF).
	cfgplugins.AssignInterfacesToDefaultNetworkInstance(t, dut, b, cfgplugins.DefaultNetworkInstanceInterfacesConfig{
		Interfaces: []string{env.aggID, env.dutPort3.Name()},
	})
	b.Set(t, dut)

	// Wait for the configuration to be reflected in state before moving on.
	gnmi.Await(t, dut, gnmi.OC().Interface(env.aggID).Aggregation().LagType().State(), portUpTimeout, oc.IfAggregate_AggregationType_LACP)
	for _, m := range env.dutMembers {
		gnmi.Await(t, dut, gnmi.OC().Interface(m.Name()).Ethernet().AggregateId().State(), portUpTimeout, env.aggID)
	}
	gnmi.Await(t, dut, gnmi.OC().Interface(env.dutPort3.Name()).OperStatus().State(), portUpTimeout, oc.Interface_OperStatus_UP)
}

// revertDUTConfig restores the DUT to the state captured before the test, in
// three SetRequests: (1) the LAG members are released and any default network
// instance assignment created by the test is removed, (2) the aggregate
// interface and its LACP config are deleted, (3) the member ports and DUT
// port3 get their original configuration back. Each step reports a failure
// without stopping the remaining steps, so the aggregate interface is always
// removed even if restoring the original port configuration fails.
func revertDUTConfig(t *testing.T, env *testEnv, snapshot cfgplugins.InterfaceConfigSnapshot) {
	t.Helper()
	if env.aggID != "" {
		t.Logf("Cleanup: release the members of %s and remove its default network instance assignment", env.aggID)
		b := &gnmi.SetBatch{}
		for _, m := range env.dutMembers {
			gnmi.BatchDelete(b, gnmi.OC().Interface(m.Name()).Ethernet().AggregateId().Config())
		}
		cfgplugins.AssignInterfacesToDefaultNetworkInstance(t, env.dut, b, cfgplugins.DefaultNetworkInstanceInterfacesConfig{
			Interfaces: []string{env.aggID, env.dutPort3.Name()},
			Delete:     true,
		})
		setForCleanup(t, env.dut, b, "release LAG members")

		t.Logf("Cleanup: delete aggregate interface %s and its LACP config", env.aggID)
		b = &gnmi.SetBatch{}
		gnmi.BatchDelete(b, gnmi.OC().Lacp().Interface(env.aggID).Config())
		gnmi.BatchDelete(b, gnmi.OC().Interface(env.aggID).Config())
		setForCleanup(t, env.dut, b, "delete aggregate interface")
	}

	t.Log("Cleanup: restore the original DUT port configuration")
	b := &gnmi.SetBatch{}
	cfgplugins.RestoreInterfacesConfig(t, env.dut, b, snapshot)
	setForCleanup(t, env.dut, b, "restore port configuration")
}

// setForCleanup applies b and reports a failure with t.Errorf instead of
// t.Fatalf, so that the remaining cleanup steps still run.
func setForCleanup(t *testing.T, dut *ondatra.DUTDevice, b *gnmi.SetBatch, step string) {
	t.Helper()
	if msg := testt.CaptureFatal(t, func(t testing.TB) { b.Set(t, dut) }); msg != nil {
		t.Errorf("Cleanup: %s failed: %s", step, *msg)
	}
}

// removeStaleTestConfig deletes any aggregate interface left behind by an
// earlier run of this test that was killed before its cleanup ran. Such an
// interface is identified by carrying dutLag's IPv4 address. Its member ports
// are released in a first SetRequest, then the aggregate interface, its LACP
// config and any default network instance assignment are deleted.
func removeStaleTestConfig(t *testing.T, env *testEnv) {
	t.Helper()
	dut := env.dut
	stale := staleTestAggregates(t, dut)
	if len(stale) == 0 {
		return
	}
	t.Logf("Removing aggregate interface(s) %v left behind by an earlier aborted run (they carry %s)", stale, dutLag.IPv4)

	b := &gnmi.SetBatch{}
	for _, m := range aggregateMembers(t, dut, stale) {
		gnmi.BatchDelete(b, gnmi.OC().Interface(m).Ethernet().AggregateId().Config())
	}
	cfgplugins.AssignInterfacesToDefaultNetworkInstance(t, dut, b, cfgplugins.DefaultNetworkInstanceInterfacesConfig{
		Interfaces: stale,
		Delete:     true,
	})
	b.Set(t, dut)

	b = &gnmi.SetBatch{}
	for _, agg := range stale {
		gnmi.BatchDelete(b, gnmi.OC().Lacp().Interface(agg).Config())
		gnmi.BatchDelete(b, gnmi.OC().Interface(agg).Config())
	}
	b.Set(t, dut)
}

// staleTestAggregates returns the aggregate interfaces whose subinterface 0
// carries dutLag's IPv4 address.
func staleTestAggregates(t *testing.T, dut *ondatra.DUTDevice) []string {
	t.Helper()
	var candidates []string
	for _, v := range gnmi.LookupAll(t, dut, gnmi.OC().InterfaceAny().Subinterface(0).Ipv4().Address(dutLag.IPv4).Ip().State()) {
		if name := interfaceNameFromPath(v.Path); name != "" {
			candidates = append(candidates, name)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	// Read the type of every candidate in one batched request.
	batch := gnmi.OCBatch()
	for _, name := range candidates {
		batch.AddPaths(gnmi.OC().Interface(name).Type())
	}
	root, _ := gnmi.Lookup(t, dut, batch.State()).Val()
	var stale []string
	for _, name := range candidates {
		if root.GetInterface(name).GetType() == oc.IETFInterfaces_InterfaceType_ieee8023adLag {
			stale = append(stale, name)
		}
	}
	return stale
}

// aggregateMembers returns the interfaces whose aggregate-id is one of aggs.
func aggregateMembers(t *testing.T, dut *ondatra.DUTDevice, aggs []string) []string {
	t.Helper()
	var members []string
	for _, v := range gnmi.LookupAll(t, dut, gnmi.OC().InterfaceAny().Ethernet().AggregateId().State()) {
		if id, ok := v.Val(); ok && slices.Contains(aggs, id) {
			members = append(members, interfaceNameFromPath(v.Path))
		}
	}
	return members
}

// interfaceNameFromPath returns the name key of the
// /interfaces/interface[name=...] element of p, or "" if absent.
func interfaceNameFromPath(p *gpb.Path) string {
	for _, e := range p.GetElem() {
		if e.GetName() == "interface" {
			return e.GetKey()["name"]
		}
	}
	return ""
}

// configureATE builds the OTG configuration: a LACP LAG over ATE port1 and
// ATE port2 with an IPv4/IPv6 device on top, and a routed device on ATE port3.
func configureATE(t *testing.T, env *testEnv) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()

	atePort3.AddToOTG(top, env.atePort3, &dutPort3)

	// OTG object names must be unique across the config; the LAG object gets
	// a ".lag" suffix so it does not clash with the "ateLag" device below.
	lag := top.Lags().Add().SetName(ateLagName + ".lag")
	lag.Protocol().Lacp().
		SetActorKey(ateLagActorKey).
		SetActorSystemPriority(ateLagActorSysPrio).
		SetActorSystemId(ateLagSystemID)
	for i, p := range env.ateMembers {
		top.Ports().Add().SetName(p.ID())
		lp := lag.Ports().Add().SetPortName(p.ID())
		lp.Ethernet().SetMac(ateLagMemberMACs[i]).SetName(fmt.Sprintf("%s.%s.Eth", ateLagName, p.ID()))
		lp.Lacp().
			SetActorActivity(gosnappi.LagPortLacpActorActivity.ACTIVE).
			SetActorPortNumber(uint32(i + 1)).
			SetActorPortPriority(ateLagActorPortPrio).
			SetLacpduTimeout(0)
	}

	dev := top.Devices().Add().SetName(ateLag.Name)
	eth := dev.Ethernets().Add().SetName(ateLag.Name + ".Eth").SetMac(ateLag.MAC)
	eth.Connection().SetLagName(lag.Name())
	eth.Ipv4Addresses().Add().SetName(ateLag.Name + ".IPv4").
		SetAddress(ateLag.IPv4).SetGateway(dutLag.IPv4).SetPrefix(uint32(ateLag.IPv4Len))
	eth.Ipv6Addresses().Add().SetName(ateLag.Name + ".IPv6").
		SetAddress(ateLag.IPv6).SetGateway(dutLag.IPv6).SetPrefix(uint32(ateLag.IPv6Len))
	return top
}

// addFlow adds a continuous IPv4/UDP flow from src to dst at pps packets per
// second. UDP source and destination ports are incremented over
// flowUDPPortCount values starting at udpOffset above flowUDPSrcStart and
// flowUDPDstStart, to give the flow enough entropy to be load-balanced across
// all LAG members by the DUT.
func addFlow(top gosnappi.Config, name string, src, dst *attrs.Attributes, pps uint64, udpOffset uint32) {
	f := top.Flows().Add().SetName(name)
	f.Metrics().SetEnable(true)
	f.TxRx().Device().SetTxNames([]string{src.Name + ".IPv4"}).SetRxNames([]string{dst.Name + ".IPv4"})
	f.Size().SetFixed(flowPacketSize)
	f.Rate().SetPps(pps)
	f.Duration().Continuous()
	f.Packet().Add().Ethernet().Src().SetValue(src.MAC)
	ip := f.Packet().Add().Ipv4()
	ip.Src().SetValue(src.IPv4)
	ip.Dst().SetValue(dst.IPv4)
	udp := f.Packet().Add().Udp()
	udp.SrcPort().Increment().SetStart(flowUDPSrcStart + udpOffset).SetStep(1).SetCount(flowUDPPortCount)
	udp.DstPort().Increment().SetStart(flowUDPDstStart + udpOffset).SetStep(1).SetCount(flowUDPPortCount)
}

// testConfigureTraffic implements RT-5.16.1 Step 1: Configure Traffic.
//   - Set up a traffic profile from the ATE to the DUT bundle interface.
//   - The traffic is distributed across all member links and is sent
//     continuously during the test.
func testConfigureTraffic(t *testing.T, env *testEnv) {
	otg := env.ate.OTG()
	t.Log("Step 1: configure continuous flows in both directions across the bundle")
	env.top.Flows().Clear()
	// ATE LAG -> DUT LAG -> DUT port3 -> ATE port3: the OTG LAG sends each flow
	// on one member port, so numLagIngressFlows flows (flowPPS in total) are
	// used to distribute the traffic across all DUT LAG members (DUT member
	// in-pkts).
	for i := 0; i < numLagIngressFlows; i++ {
		addFlow(env.top, lagIngressFlowName(i), &ateLag, &atePort3, flowPPS/numLagIngressFlows, uint32(i)*flowUDPPortCount)
	}
	// ATE port3 -> DUT port3 -> DUT LAG -> ATE LAG: the DUT hashes the flow
	// across the DUT LAG members (DUT member out-pkts).
	addFlow(env.top, flowPort3ToLag, &atePort3, &ateLag, flowPPS, 0)

	otg.PushConfig(t, env.top)
	otg.StartProtocols(t)
	if err := awaitLACPMembers(t, env, env.dutMembers, true, lacpTimeout); err != nil {
		t.Fatalf("Step 1: LACP not re-established after pushing traffic config: %v", err)
	}
	// Neighbor resolution must complete before traffic is started.
	otgutils.WaitForARP(t, otg, env.top, cfgplugins.IPv4)
	otgutils.WaitForARP(t, otg, env.top, cfgplugins.IPv6)

	t.Log("Step 1: start continuous traffic")
	otg.StartTraffic(t)
	env.trafficRunning = true
}

// testVerifyInitialState implements RT-5.16.1 Step 2: Verify Initial State.
//   - Verify LACP is ESTABLISHED.
//   - Verify traffic is flowing and distributed across all member links.
//   - Verify interface traffic counters are incrementing on all member ports
//     and on the bundle interface.
func testVerifyInitialState(t *testing.T, env *testEnv) {
	t.Log("Step 2: verify LACP is ESTABLISHED on all members")
	errs := []error{awaitLACPMembers(t, env, env.dutMembers, true, lacpTimeout)}

	t.Log("Step 2: verify traffic is flowing end to end")
	errs = append(errs, awaitFlowsRxIncrease(t, env, flows))

	t.Log("Step 2: verify counters are incrementing on every member port (traffic distributed across all members) and on the bundle")
	intfs := append(portNames(env.dutMembers), env.aggID)
	errs = append(errs, awaitCountersIncrease(t, env.dut, getCounters(t, env.dut, intfs), counterTimeout))
	otgutils.LogLAGMetrics(t, env.ate.OTG(), env.top)
	if err := errors.Join(errs...); err != nil {
		t.Errorf("Step 2: %v", err)
	}
}

// testRebootLinecard implements RT-5.16.1 Step 3: Reboot Linecard.
//   - Issue a gnoi.system.Reboot RPC targeting the linecard that hosts one of
//     the bundle member ports.
//   - During the reboot verify that:
//     (3a) the rebooted member port status goes DOWN,
//     (3b) the LACP session for that member goes DOWN,
//     (3c) traffic continues to flow through the remaining member(s),
//     (3d) the aggregate bundle counters continue to increment and never
//     decrement (the latter is monitored until Step 5).
func testRebootLinecard(t *testing.T, env *testEnv) {
	dut := env.dut
	env.preReboot = getCounters(t, dut, append(portNames(env.dutMembers), env.aggID))
	logCounters(t, "Step 3: pre-reboot bundle", env.aggID, env.preReboot[env.aggID])

	t.Log("Step 3: start streaming the bundle counters to verify they never decrement during the reboot and recovery")
	env.bundleMonitor = helpers.StartInterfaceCountersMonitor(t, dut, env.aggID, monitorMaxDuration)

	t.Logf("Step 3: issue gnoi.system.Reboot for linecard %s", env.rebootLinecard)
	rebootLinecard(t, env)

	t.Log("Step 3a: verify the rebooted member port(s) go DOWN")
	if err := awaitPortsNotUp(t, dut, portNames(env.rebootedMembers), memberDownTimeout); err != nil {
		t.Fatalf("Step 3a: %v", err)
	}

	t.Log("Step 3b: verify the LACP session for the rebooted member(s) goes DOWN")
	errs := []error{awaitLACPMembers(t, env, env.rebootedMembers, false, lacpTimeout)}

	t.Log("Step 3c: verify traffic continues to flow through the remaining member(s)")
	if status := gnmi.Get(t, dut, gnmi.OC().Interface(env.aggID).OperStatus().State()); status != oc.Interface_OperStatus_UP {
		errs = append(errs, fmt.Errorf("bundle %s oper-status is %v during the linecard reboot, want UP", env.aggID, status))
	}
	errs = append(errs, awaitLACPMembers(t, env, env.remainingMembers, true, lacpTimeout))
	errs = append(errs, awaitCountersIncrease(t, dut, getCounters(t, dut, portNames(env.remainingMembers)), counterTimeout))
	errs = append(errs, awaitFlowsRxIncrease(t, env, flows))

	t.Log("Step 3d: verify the aggregate bundle counters continue to increment past their pre-reboot values")
	errs = append(errs, awaitCountersIncrease(t, dut, map[string]*oc.Interface_Counters{env.aggID: env.preReboot[env.aggID]}, counterTimeout))

	// The README requires 3b-3d to hold "during the reboot". If the
	// linecard came back before the checks above completed, they would have
	// been satisfied by the recovered bundle instead, so confirm the
	// rebooted member(s) are still not UP.
	t.Log("Step 3: confirm the rebooted member(s) were still down when the traffic and counter checks completed")
	batch := gnmi.OCBatch()
	for _, m := range env.rebootedMembers {
		batch.AddPaths(gnmi.OC().Interface(m.Name()).OperStatus())
	}
	root, _ := gnmi.Lookup(t, dut, batch.State()).Val()
	for _, m := range env.rebootedMembers {
		if root.GetInterface(m.Name()).GetOperStatus() == oc.Interface_OperStatus_UP {
			errs = append(errs, fmt.Errorf("rebooted member %s was already UP again when the Step 3 checks completed; traffic was not verified during the linecard reboot", m.Name()))
		}
	}

	if err := errors.Join(errs...); err != nil {
		t.Errorf("Step 3: %v", err)
	}
}

// rebootLinecard sends gnoi.system.Reboot (method COLD) for env.rebootLinecard.
// The subcomponent path honours the GNOISubcomponentPath deviation (component
// name only vs. full /components/component[name] path). The RPC may return
// before the linecard has restarted; the callers track the reboot with
// gnmi.Watch / gnmi.Await on interface and component state.
func rebootLinecard(t *testing.T, env *testEnv) {
	t.Helper()
	req := &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
		Message: "RT-5.16 LACP member linecard reboot",
		Subcomponents: []*tpb.Path{
			components.GetSubcomponentPath(env.rebootLinecard, deviations.GNOISubcomponentPath(env.dut)),
		},
	}
	t.Logf("gnoi.system.Reboot request: %v", req)
	resp, err := env.dut.RawAPIs().GNOI(t).System().Reboot(t.Context(), req)
	if err != nil {
		t.Fatalf("gnoi.system.Reboot for linecard %s failed: %v", env.rebootLinecard, err)
	}
	env.rebootIssued = true
	t.Logf("gnoi.system.Reboot response: %v", resp)
}

// testLinecardRecovery implements RT-5.16.1 Step 4: Linecard Recovery.
//   - Wait for the rebooted linecard to fully recover and come back online.
//   - Verify that the member port status goes UP.
//   - Verify LACP session is re-established for that member port.
//   - Verify traffic distribution resumes across all member links.
func testLinecardRecovery(t *testing.T, env *testEnv) {
	dut := env.dut
	t.Logf("Step 4: wait for linecard %s to come back online (oper-status ACTIVE)", env.rebootLinecard)
	start := time.Now()
	if s, ok := components.AwaitOperStatus(t, dut, env.rebootLinecard, linecardBootTimeout, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE); !ok {
		t.Fatalf("Step 4: linecard %s did not return to ACTIVE within %v (last oper-status %v)", env.rebootLinecard, linecardBootTimeout, s)
	}

	t.Log("Step 4: verify the rebooted member port(s) go UP")
	for _, m := range env.rebootedMembers {
		// The linecard can report ACTIVE before its ports are ready, so the
		// port wait is bounded by the linecard boot timeout as well.
		gnmi.Await(t, onChange(dut), gnmi.OC().Interface(m.Name()).OperStatus().State(), linecardBootTimeout, oc.Interface_OperStatus_UP)
	}
	t.Logf("Step 4: linecard %s and its member port(s) recovered %v after the reboot RPC", env.rebootLinecard, time.Since(start).Round(time.Second))

	t.Log("Step 4: verify the LACP session is re-established for the rebooted member(s)")
	errs := []error{awaitLACPMembers(t, env, env.dutMembers, true, lacpTimeout)}

	t.Log("Step 4: verify traffic distribution resumes across all member links")
	errs = append(errs, awaitCountersIncrease(t, dut, getCounters(t, dut, portNames(env.dutMembers)), counterTimeout))
	if err := errors.Join(errs...); err != nil {
		t.Errorf("Step 4: %v", err)
	}
}

// testStatisticsPostReboot implements RT-5.16.1 Step 5: Verify Statistics
// Post-Reboot.
//   - Verify the counters of the rebooted port resume incrementing.
//   - Verify the cumulative bundle counters reported via telemetry show no
//     unexpected drops, decrements or reset to zero.
//   - Individual counters of the rebooted port may reset to zero, but must
//     resume incrementing without causing the bundle counters to decrement.
func testStatisticsPostReboot(t *testing.T, env *testEnv) {
	dut := env.dut
	var errs []error

	t.Log("Step 5: verify the rebooted port counters resume incrementing (a reset to zero is allowed)")
	cur := getCounters(t, dut, portNames(env.rebootedMembers))
	for name, c := range cur {
		if pre := env.preReboot[name]; pre != nil && c.GetInPkts() < pre.GetInPkts() {
			t.Logf("Step 5: member %s counters were reset by the linecard reboot (in-pkts %d -> %d); allowed by the README", name, pre.GetInPkts(), c.GetInPkts())
		}
	}
	errs = append(errs, awaitCountersIncrease(t, dut, cur, counterTimeout))

	t.Log("Step 5: verify the bundle counters never decremented or reset during the reboot and recovery")
	if env.bundleMonitor == nil {
		errs = append(errs, fmt.Errorf("bundle counters monitor was not started (Step 3 did not run)"))
	} else {
		samples, err := env.bundleMonitor.Stop(t)
		t.Logf("Step 5: bundle %s counters monitor received %d samples", env.aggID, samples)
		errs = append(errs, err)
	}

	t.Log("Step 5: verify the final bundle counters are not below the pre-reboot snapshot")
	if pre := env.preReboot[env.aggID]; pre != nil {
		errs = append(errs, awaitCountersNotBelow(t, dut, env.aggID, pre, counterTimeout))
	}

	t.Log("Step 5: stop traffic and log traffic statistics")
	otg := env.ate.OTG()
	otg.StopTraffic(t)
	env.trafficRunning = false
	errs = append(errs, awaitFlowsStopped(t, env, flows))
	otgutils.LogFlowMetrics(t, otg, env.top)
	otgutils.LogLAGMetrics(t, otg, env.top)
	for _, f := range gnmi.GetAll(t, otg, gnmi.OTG().FlowAny().State()) {
		tx, rx := f.GetCounters().GetOutPkts(), f.GetCounters().GetInPkts()
		if tx == 0 {
			errs = append(errs, fmt.Errorf("flow %s transmitted no packets", f.GetName()))
			continue
		}
		// Loss during the reboot is expected (README: "possibly at a reduced
		// rate if traffic is dropped"); it is logged for reference only.
		t.Logf("Step 5: flow %s tx=%d rx=%d loss=%.4f%%", f.GetName(), tx, rx, float64(tx-min(tx, rx))*100/float64(tx))
	}

	if err := errors.Join(errs...); err != nil {
		t.Errorf("Step 5: %v", err)
	}
}

// onChange returns gNMI options for dut that subscribe with ON_CHANGE mode
// (README RPC coverage: gNMI.Subscribe on_change). It is used for event-driven
// state leaves (interface oper-status, LACP member state); counters keep the
// default target-defined (sampled) mode.
func onChange(dut *ondatra.DUTDevice) *gnmi.Opts {
	return dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE))
}

// getCounters returns /interfaces/interface/state/counters for every interface
// in intfs, fetched with a single batched gNMI Get.
func getCounters(t *testing.T, dut *ondatra.DUTDevice, intfs []string) map[string]*oc.Interface_Counters {
	t.Helper()
	batch := gnmi.OCBatch()
	for _, i := range intfs {
		batch.AddPaths(gnmi.OC().Interface(i).Counters())
	}
	root := gnmi.Get(t, dut, batch.State())
	counters := make(map[string]*oc.Interface_Counters)
	for _, i := range intfs {
		counters[i] = root.GetInterface(i).GetCounters()
	}
	return counters
}

// awaitLACPMembers waits until every given DUT LAG member reaches the wanted
// LACP state using gnmi.Watch on
// /lacp/interfaces/interface/members/member/state. All watches are started
// before any is awaited so the members are checked concurrently.
//   - wantUp: activity ACTIVE, synchronization IN_SYNC, collecting and
//     distributing true.
//   - !wantUp: the member is no longer IN_SYNC, collecting or distributing, or
//     is no longer reported at all (e.g. while its linecard is offline).
func awaitLACPMembers(t *testing.T, env *testEnv, members []*ondatra.Port, wantUp bool, timeout time.Duration) error {
	t.Helper()
	watchers := make(map[string]*gnmi.Watcher[*oc.Lacp_Interface_Member])
	for _, m := range members {
		q := gnmi.OC().Lacp().Interface(env.aggID).Member(m.Name()).State()
		watchers[m.Name()] = gnmi.Watch(t, onChange(env.dut), q, timeout, func(v *ygnmi.Value[*oc.Lacp_Interface_Member]) bool {
			mem, present := v.Val()
			if wantUp {
				return present &&
					mem.GetActivity() == oc.Lacp_LacpActivityType_ACTIVE &&
					mem.GetSynchronization() == oc.Lacp_LacpSynchronizationType_IN_SYNC &&
					mem.GetCollecting() && mem.GetDistributing()
			}
			return !present ||
				mem.GetSynchronization() != oc.Lacp_LacpSynchronizationType_IN_SYNC ||
				!mem.GetCollecting() || !mem.GetDistributing()
		})
	}
	var errs []error
	for name, w := range watchers {
		val, ok := w.Await(t)
		mem, present := val.Val()
		switch {
		case !ok && !present:
			errs = append(errs, fmt.Errorf("LACP member %s of %s: no state reported within %v (wantUp=%t)", name, env.aggID, timeout, wantUp))
		case !ok:
			errs = append(errs, fmt.Errorf("LACP member %s of %s: activity=%v sync=%v collecting=%t distributing=%t after %v (wantUp=%t)",
				name, env.aggID, mem.GetActivity(), mem.GetSynchronization(), mem.GetCollecting(), mem.GetDistributing(), timeout, wantUp))
		case !present:
			t.Logf("LACP member %s of %s is no longer reported (treated as DOWN)", name, env.aggID)
		default:
			t.Logf("LACP member %s of %s: activity=%v sync=%v collecting=%t distributing=%t",
				name, env.aggID, mem.GetActivity(), mem.GetSynchronization(), mem.GetCollecting(), mem.GetDistributing())
		}
	}
	return errors.Join(errs...)
}

// awaitPortsNotUp waits, using concurrent gnmi.Watch subscriptions, until the
// oper-status of every interface in intfs is no longer UP (DOWN,
// LOWER_LAYER_DOWN, NOT_PRESENT, ...) or the leaf is deleted because the
// hosting linecard went offline.
func awaitPortsNotUp(t *testing.T, dut *ondatra.DUTDevice, intfs []string, timeout time.Duration) error {
	t.Helper()
	watchers := make(map[string]*gnmi.Watcher[oc.E_Interface_OperStatus])
	for _, i := range intfs {
		watchers[i] = gnmi.Watch(t, onChange(dut), gnmi.OC().Interface(i).OperStatus().State(), timeout, func(v *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
			s, present := v.Val()
			return !present || s != oc.Interface_OperStatus_UP
		})
	}
	var errs []error
	for i, w := range watchers {
		val, ok := w.Await(t)
		s, present := val.Val()
		switch {
		case !ok:
			errs = append(errs, fmt.Errorf("interface %s still UP %v after the linecard reboot", i, timeout))
		case present:
			t.Logf("Interface %s oper-status is %v", i, s)
		default:
			t.Logf("Interface %s oper-status is no longer reported (treated as DOWN)", i)
		}
	}
	return errors.Join(errs...)
}

// countersSample holds the last /interfaces/interface/state/counters sample
// that contained counter values. Some targets also stream notifications under
// the counters container that carry only non-OpenConfig (vendor) leaves, which
// ygnmi reports as not present; tracking the last real sample keeps those from
// hiding the actual counter values in error messages. Each sample is written
// only by its own watch goroutine and read after the watch's Await returns.
type countersSample struct {
	c *oc.Interface_Counters
}

// awaitCountersIncrease waits, using concurrent gnmi.Watch subscriptions on
// /interfaces/interface/state/counters, until in-pkts and out-pkts of every
// interface in base have grown by at least minPktDelta and in-octets and
// out-octets have grown compared to the base value.
func awaitCountersIncrease(t *testing.T, dut *ondatra.DUTDevice, base map[string]*oc.Interface_Counters, timeout time.Duration) error {
	t.Helper()
	watchers := make(map[string]*gnmi.Watcher[*oc.Interface_Counters])
	lastSeen := make(map[string]*countersSample)
	for intf, b := range base {
		s := &countersSample{}
		lastSeen[intf] = s
		watchers[intf] = gnmi.Watch(t, dut, gnmi.OC().Interface(intf).Counters().State(), timeout, func(v *ygnmi.Value[*oc.Interface_Counters]) bool {
			c, present := v.Val()
			if !present {
				return false
			}
			s.c = c
			return c.GetInPkts() >= b.GetInPkts()+minPktDelta &&
				c.GetOutPkts() >= b.GetOutPkts()+minPktDelta &&
				c.GetInOctets() > b.GetInOctets() &&
				c.GetOutOctets() > b.GetOutOctets()
		})
	}
	var errs []error
	for intf, w := range watchers {
		b := base[intf]
		_, ok := w.Await(t)
		c := lastSeen[intf].c
		switch {
		case !ok && c == nil:
			errs = append(errs, fmt.Errorf("interface %s: counters not reported within %v", intf, timeout))
		case !ok:
			errs = append(errs, fmt.Errorf("interface %s: counters did not increment (pkts by >= %d, octets by > 0) within %v: in-pkts %d -> %d, out-pkts %d -> %d, in-octets %d -> %d, out-octets %d -> %d",
				intf, minPktDelta, timeout,
				b.GetInPkts(), c.GetInPkts(), b.GetOutPkts(), c.GetOutPkts(),
				b.GetInOctets(), c.GetInOctets(), b.GetOutOctets(), c.GetOutOctets()))
		default:
			t.Logf("Interface %s counters incrementing: in-pkts +%d, out-pkts +%d, in-octets +%d, out-octets +%d", intf,
				c.GetInPkts()-b.GetInPkts(), c.GetOutPkts()-b.GetOutPkts(), c.GetInOctets()-b.GetInOctets(), c.GetOutOctets()-b.GetOutOctets())
		}
	}
	return errors.Join(errs...)
}

// awaitCountersNotBelow waits, using gnmi.Watch, until every cumulative
// counter of intf is at least the value in pre, and returns an error naming
// the counters that are still below it when timeout expires.
func awaitCountersNotBelow(t *testing.T, dut *ondatra.DUTDevice, intf string, pre *oc.Interface_Counters, timeout time.Duration) error {
	t.Helper()
	below := func(c *oc.Interface_Counters) []string {
		var names []string
		for _, x := range []struct {
			name     string
			pre, cur uint64
		}{
			{"in-pkts", pre.GetInPkts(), c.GetInPkts()},
			{"out-pkts", pre.GetOutPkts(), c.GetOutPkts()},
			{"in-octets", pre.GetInOctets(), c.GetInOctets()},
			{"out-octets", pre.GetOutOctets(), c.GetOutOctets()},
		} {
			if x.cur < x.pre {
				names = append(names, fmt.Sprintf("%s %d < %d", x.name, x.cur, x.pre))
			}
		}
		return names
	}
	s := &countersSample{}
	_, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(intf).Counters().State(), timeout, func(v *ygnmi.Value[*oc.Interface_Counters]) bool {
		c, present := v.Val()
		if !present {
			return false
		}
		s.c = c
		return len(below(c)) == 0
	}).Await(t)
	c := s.c
	if c == nil {
		return fmt.Errorf("interface %s: counters not reported within %v", intf, timeout)
	}
	logCounters(t, "Step 5: final bundle", intf, c)
	if !ok {
		return fmt.Errorf("interface %s: counters below their pre-reboot values: %v", intf, below(c))
	}
	return nil
}

// awaitFlowsRxIncrease waits, using concurrent gnmi.Watch subscriptions on the
// OTG flow counters, until every flow has received at least minPktDelta more
// packets than when called.
func awaitFlowsRxIncrease(t *testing.T, env *testEnv, flowNames []string) error {
	t.Helper()
	otg := env.ate.OTG()
	base := make(map[string]uint64)
	for _, f := range gnmi.GetAll(t, otg, gnmi.OTG().FlowAny().State()) {
		base[f.GetName()] = f.GetCounters().GetInPkts()
	}
	watchers := make(map[string]*gnmi.Watcher[uint64])
	for _, name := range flowNames {
		b := base[name]
		watchers[name] = gnmi.Watch(t, otg, gnmi.OTG().Flow(name).Counters().InPkts().State(), counterTimeout, func(v *ygnmi.Value[uint64]) bool {
			rx, present := v.Val()
			return present && rx >= b+minPktDelta
		})
	}
	var errs []error
	for name, w := range watchers {
		val, ok := w.Await(t)
		rx, _ := val.Val()
		if !ok {
			errs = append(errs, fmt.Errorf("flow %s: rx packets did not increase by >= %d within %v (%d -> %d)", name, minPktDelta, counterTimeout, base[name], rx))
			continue
		}
		t.Logf("Flow %s is being received: rx %d -> %d", name, base[name], rx)
	}
	return errors.Join(errs...)
}

// awaitFlowsStopped waits, using concurrent gnmi.Watch subscriptions on the
// OTG flow transmit state, until every flow has stopped transmitting so that
// the final flow counters are stable before they are read. All watches are
// started before any is awaited.
func awaitFlowsStopped(t *testing.T, env *testEnv, flowNames []string) error {
	t.Helper()
	otg := env.ate.OTG()
	watchers := make(map[string]*gnmi.Watcher[bool])
	for _, name := range flowNames {
		watchers[name] = gnmi.Watch(t, otg, gnmi.OTG().Flow(name).Transmit().State(), trafficStopTimeout, func(v *ygnmi.Value[bool]) bool {
			transmitting, present := v.Val()
			return present && !transmitting
		})
	}
	var errs []error
	for name, w := range watchers {
		if _, ok := w.Await(t); !ok {
			errs = append(errs, fmt.Errorf("flow %s did not stop transmitting within %v", name, trafficStopTimeout))
		}
	}
	return errors.Join(errs...)
}

// logCounters logs the cumulative counters of intf.
func logCounters(t *testing.T, desc, intf string, c *oc.Interface_Counters) {
	t.Helper()
	t.Logf("%s %s counters: in-pkts=%d out-pkts=%d in-octets=%d out-octets=%d", desc, intf, c.GetInPkts(), c.GetOutPkts(), c.GetInOctets(), c.GetOutOctets())
}

// allDUTPortNames returns the names of all DUT ports used by the test.
func allDUTPortNames(env *testEnv) []string {
	return append(portNames(env.dutMembers), env.dutPort3.Name())
}

// portNames returns the DUT interface names of ports.
func portNames(ports []*ondatra.Port) []string {
	var names []string
	for _, p := range ports {
		names = append(names, p.Name())
	}
	return names
}
