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

package packet_link_qualification_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/plq"
	plqpb "github.com/openconfig/gnoi/packet_link_qualification"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"google.golang.org/grpc/codes"
)

const (
	generatorPacketRate = uint64(138888)
	interfaceMTU        = uint16(9000)
	ipv4PrefixLen       = uint8(30)
	ipv6PrefixLen       = uint8(126)
	plqRPCTimeout       = 60 * time.Second
	intfConvergeTimeout = 2 * time.Minute
	lagConvergeTimeout  = 12 * time.Minute
	testingTimeout      = 30 * time.Second
	runningMargin       = 60 * time.Second
	completionMargin    = 2 * time.Minute
	plqTolerancePct     = 0.0001
	defaultMinBps       = uint64(1e11)
)

var (
	dutPortNames = []string{"port1", "port2", "port3", "port4"}

	// singletonInterfaces are the gNOI-2.1.2 singleton interfaces.
	singletonInterfaces = cfgplugins.SingletonInterfacesConfig{
		Interfaces: []cfgplugins.SingletonInterfaceParams{
			{
				PortName:    "port1",
				Description: "Singleton 400G Generator Port 1",
				MTU:         interfaceMTU,
				IPv4:        "192.0.2.1",
				IPv4Len:     ipv4PrefixLen,
				IPv6:        "2001:db8::1",
				IPv6Len:     ipv6PrefixLen,
			},
			{
				PortName:    "port2",
				Description: "Singleton 400G Reflector Port 2",
				MTU:         interfaceMTU,
				IPv4:        "192.0.2.5",
				IPv4Len:     ipv4PrefixLen,
				IPv6:        "2001:db8::5",
				IPv6Len:     ipv6PrefixLen,
			},
			{
				PortName:    "port3",
				Description: "Singleton 100G Generator Port 3",
				MTU:         interfaceMTU,
				IPv4:        "192.0.2.9",
				IPv4Len:     ipv4PrefixLen,
				IPv6:        "2001:db8::9",
				IPv6Len:     ipv6PrefixLen,
			},
			{
				PortName:    "port4",
				Description: "Singleton 100G Reflector Port 4",
				MTU:         interfaceMTU,
				IPv4:        "192.0.2.13",
				IPv4Len:     ipv4PrefixLen,
				IPv6:        "2001:db8::d",
				IPv6Len:     ipv6PrefixLen,
			},
		},
	}

	// aggregates are the gNOI-2.1.3 logical aggregates lag1 through lag4. Their
	// platform names are allocated when the subtest runs.
	aggregates = []cfgplugins.StaticAggregateParams{
		{
			Description: "400G Aggregate Interface 1",
			MTU:         interfaceMTU,
			IPv4:        "198.51.100.1",
			IPv4Len:     ipv4PrefixLen,
			IPv6:        "2001:db8:1::1",
			IPv6Len:     ipv6PrefixLen,
			Members:     []cfgplugins.StaticAggregateMemberParams{{PortName: "port1", ID: 10, Description: "LAG Member Port 1"}},
		},
		{
			Description: "400G Aggregate Interface 2",
			MTU:         interfaceMTU,
			IPv4:        "198.51.100.5",
			IPv4Len:     ipv4PrefixLen,
			IPv6:        "2001:db8:1::5",
			IPv6Len:     ipv6PrefixLen,
			Members:     []cfgplugins.StaticAggregateMemberParams{{PortName: "port2", ID: 11, Description: "LAG Member Port 2"}},
		},
		{
			Description: "100G Aggregate Interface 3",
			MTU:         interfaceMTU,
			IPv4:        "198.51.100.9",
			IPv4Len:     ipv4PrefixLen,
			IPv6:        "2001:db8:1::9",
			IPv6Len:     ipv6PrefixLen,
			Members:     []cfgplugins.StaticAggregateMemberParams{{PortName: "port3", ID: 12, Description: "LAG Member Port 3"}},
		},
		{
			Description: "100G Aggregate Interface 4",
			MTU:         interfaceMTU,
			IPv4:        "198.51.100.13",
			IPv4Len:     ipv4PrefixLen,
			IPv6:        "2001:db8:1::d",
			IPv6Len:     ipv6PrefixLen,
			Members:     []cfgplugins.StaticAggregateMemberParams{{PortName: "port4", ID: 13, Description: "LAG Member Port 4"}},
		},
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// requiredGeneratorBps returns the minimum generator max_bps capability: the
// speed of the fastest port under test (4e11 with 400G ports), or 1e11 when
// the port speeds are unknown. Capabilities describe the whole device, so the
// fastest link sets the threshold.
func requiredGeneratorBps(t *testing.T, dut *ondatra.DUTDevice) uint64 {
	t.Helper()
	var maxBps uint64
	for _, name := range dutPortNames {
		maxBps = max(maxBps, fptest.EthernetSpeedToUint64(fptest.GetIfSpeed(t, dut.Port(t, name))))
	}
	if maxBps == 0 {
		return defaultMinBps
	}
	return maxBps
}

// TestCapabilitiesResponse validates gNOI-2.1.1 Step 1: Capabilities RPC response fields.
func TestCapabilitiesResponse(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	plqResp := plq.Capabilities(t, dut, plqRPCTimeout)
	t.Logf("LinkQualification.Capabilities: %v", plqResp)

	genCap := plqResp.GetGenerator().GetPacketGenerator()

	cases := []struct {
		desc string
		got  uint64
		min  uint64
	}{
		{
			desc: "Time",
			got:  uint64(plqResp.GetTime().GetSeconds()),
			min:  1,
		},
		{
			desc: "MaxHistoricalResultsPerInterface",
			got:  plqResp.GetMaxHistoricalResultsPerInterface(),
			min:  2,
		},
		{
			desc: "Generator MinSetupDuration",
			got:  uint64(genCap.GetMinSetupDuration().GetSeconds()),
			min:  1,
		},
		{
			desc: "Generator MinTeardownDuration",
			got:  uint64(genCap.GetMinTeardownDuration().GetSeconds()),
			min:  1,
		},
		{
			desc: "Generator MinSampleInterval",
			got:  uint64(genCap.GetMinSampleInterval().GetSeconds()),
			min:  10,
		},
		{
			desc: "Generator MinMtu",
			got:  uint64(genCap.GetMinMtu()),
			min:  64,
		},
		{
			desc: "Generator MaxMtu",
			got:  uint64(genCap.GetMaxMtu()),
			min:  plq.MinGeneratorMTU(dut),
		},
		{
			desc: "Generator MaxBps",
			got:  genCap.GetMaxBps(),
			min:  requiredGeneratorBps(t, dut),
		},
		{
			desc: "Generator MaxPps",
			got:  genCap.GetMaxPps(),
			min:  plq.MinGeneratorPPS(dut),
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.got < tc.min {
				t.Errorf("%s: got %v, want >= %v", tc.desc, tc.got, tc.min)
			}
		})
	}

	t.Run("Reflector", func(t *testing.T) {
		ref := plqResp.GetReflector()
		asicLB := ref.GetAsicLoopback()
		pmdLB := ref.GetPmdLoopback()
		switch {
		case asicLB.GetMinSetupDuration().GetSeconds() >= 1 && asicLB.GetMinTeardownDuration().GetSeconds() >= 1:
			t.Logf("Device supports ASIC loopback reflector mode: %v", asicLB)
		case pmdLB.GetMinSetupDuration().GetSeconds() >= 1 && pmdLB.GetMinTeardownDuration().GetSeconds() >= 1:
			t.Logf("Device supports PMD loopback reflector mode: %v", pmdLB)
		default:
			t.Errorf("Reflector MinSetupDuration or MinTeardownDuration is not >= 1s for ASIC or PMD loopback: %v", ref)
		}
	})
}

// TestNonexistingID validates gNOI-2.1.1 Step 2: Get and Delete RPC error handling for a non-existent ID.
func TestNonexistingID(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	id := "non-existing-ID"

	getResp := plq.Get(t, dut, []string{id}, plqRPCTimeout)
	t.Logf("LinkQualification.Get(%q): %v", id, getResp)

	t.Run("GetResponse", func(t *testing.T) {
		res, ok := getResp.GetResults()[id]
		if !ok {
			t.Fatalf("GetResponse missing result entry for ID %q", id)
		}
		if got, want := res.GetStatus().GetCode(), int32(codes.NotFound); got != want {
			t.Errorf("getResp.GetResults()[%q].GetStatus().GetCode(): got %v, want %v", id, got, want)
		}
	})

	deleteResp := plq.Delete(t, dut, []string{id}, plqRPCTimeout)
	t.Logf("LinkQualification.Delete(%q): %v", id, deleteResp)

	t.Run("DeleteResp", func(t *testing.T) {
		res, ok := deleteResp.GetResults()[id]
		if !ok {
			t.Fatalf("DeleteResponse missing result entry for ID %q", id)
		}
		if got, want := res.GetCode(), int32(codes.NotFound); got != want {
			t.Errorf("deleteResp.GetResults()[%q].GetCode(): got %v, want %v", id, got, want)
		}
	})
}

// TestListDelete validates gNOI-2.1.1 Step 3: Baseline List and Delete validation on the DUT.
func TestListDelete(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	plq.ClearAll(t, dut, plqRPCTimeout)
}

// testEnv holds the DUT state captured by setupEnvironment and restored by its
// cleanup.
type testEnv struct {
	// physPorts are the platform names of port1 through port4.
	physPorts    []string
	portBaseline cfgplugins.InterfaceConfigSnapshot
	// lags are the aggregate interfaces created by the test.
	lags        []string
	lagBaseline cfgplugins.InterfaceConfigSnapshot
}

// setupEnvironment implements the README test environment setup: it
// configures the default network instance, captures the baseline
// configuration of port1 through port4, and registers the README Cleanup
// section with t.Cleanup.
func setupEnvironment(t *testing.T, dut *ondatra.DUTDevice) *testEnv {
	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	env := &testEnv{lagBaseline: cfgplugins.InterfaceConfigSnapshot{}}
	for _, name := range dutPortNames {
		env.physPorts = append(env.physPorts, dut.Port(t, name).Name())
	}
	env.portBaseline = cfgplugins.SnapshotInterfacesConfig(t, dut, env.physPorts)

	// Cleanups run in reverse order: the qualification records are deleted
	// first, and the configuration is restored even if that deletion fails.
	t.Cleanup(func() { env.restoreConfig(t, dut) })
	t.Cleanup(func() {
		t.Log("Cleanup: Deleting any remaining link qualification records")
		plq.ClearAll(t, dut, plqRPCTimeout)
	})
	return env
}

// trackLags records aggregate interfaces before they are created, so that the
// cleanup removes them even if the test fails part way through.
func (e *testEnv) trackLags(t *testing.T, dut *ondatra.DUTDevice, names []string) {
	t.Helper()
	for name, cfg := range cfgplugins.SnapshotInterfacesConfig(t, dut, names) {
		e.lagBaseline[name] = cfg
	}
	e.lags = append(e.lags, names...)
}

// restoreConfig removes the aggregate interfaces created by the test, restores
// the baseline configuration of port1 through port4 with a single gNMI Set,
// and verifies that they are admin-status and oper-status UP.
func (e *testEnv) restoreConfig(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	t.Log("Cleanup: Removing test aggregates and restoring the baseline configuration of port1..port4")
	sb := &gnmi.SetBatch{}
	cfgplugins.AssignInterfacesToDefaultNetworkInstance(t, dut, sb, cfgplugins.DefaultNetworkInstanceInterfacesConfig{
		Interfaces: append(append([]string{}, e.lags...), e.physPorts...),
		Delete:     true,
	})
	if len(e.lags) > 0 {
		for _, port := range e.physPorts {
			gnmi.BatchDelete(sb, gnmi.OC().Interface(port).Ethernet().AggregateId().Config())
		}
		cfgplugins.RestoreInterfacesConfig(t, dut, sb, e.lagBaseline)
	}
	cfgplugins.RestoreInterfacesConfig(t, dut, sb, e.portBaseline)
	sb.Set(t, dut)

	t.Log("Cleanup: Verifying admin-status and oper-status UP on port1..port4")
	helpers.ValidateAdminAndOperStatusUPIntfs(t, dut, e.physPorts, intfConvergeTimeout)
}

// qualification holds the state shared by the steps of one concurrent
// multi-link qualification subtest.
type qualification struct {
	dut      *ondatra.DUTDevice
	env      *testEnv
	idSuffix string
	// configured, created and running record whether the interface
	// configuration, the Create RPC and the RUNNING state check succeeded;
	// steps that depend on them are skipped otherwise.
	configured    bool
	created       bool
	running       bool
	recoveryIntfs []string
	pairs         []plq.LinkPair
	timing        *plq.Timing
	mode          plq.ReflectorMode
}

// configureSingletons implements gNOI-2.1.2 Step 1.
func (q *qualification) configureSingletons(t *testing.T) {
	t.Log("Configuring port1..port4 as dual-stack L3 singleton interfaces")
	sb := &gnmi.SetBatch{}
	cfgplugins.NewSingletonInterfaces(t, q.dut, sb, singletonInterfaces)
	sb.Set(t, q.dut)

	t.Log("Verifying admin-status and oper-status UP on port1..port4")
	helpers.ValidateAdminAndOperStatusUPIntfs(t, q.dut, q.env.physPorts, intfConvergeTimeout)
	q.recoveryIntfs = q.env.physPorts
	q.configured = true
}

// configureAggregates implements gNOI-2.1.3 Step 1.
func (q *qualification) configureAggregates(t *testing.T) {
	names := cfgplugins.NextAggregateInterfaces(t, q.dut, len(aggregates))
	q.env.trackLags(t, q.dut, names)

	cfg := cfgplugins.StaticAggregatesConfig{}
	memberToLag := make(map[string]string)
	for i, agg := range aggregates {
		agg.Name = names[i]
		cfg.Aggregates = append(cfg.Aggregates, agg)
		for _, m := range agg.Members {
			memberToLag[q.dut.Port(t, m.PortName).Name()] = agg.Name
		}
		t.Logf("Logical aggregate lag%d is %s", i+1, agg.Name)
	}

	t.Log("Removing the singleton L3 configuration from port1..port4, configuring static LAGs lag1..lag4 and binding their member ports")
	sb := &gnmi.SetBatch{}
	cfgplugins.NewStaticAggregates(t, q.dut, sb, cfg)
	sb.Set(t, q.dut)

	t.Log("Verifying oper-status UP on all LAG and member interfaces and ethernet/state/aggregate-id on the members")
	helpers.ValidateAggregateMembersState(t, q.dut, memberToLag, lagConvergeTimeout)
	q.recoveryIntfs = append(append([]string{}, q.env.physPorts...), names...)
	q.configured = true
}

// create starts concurrent qualification of the port1->port2 (400G) and
// port3->port4 (100G) links.
func (q *qualification) create(t *testing.T) {
	if !q.configured {
		t.Skip("Skipping: the interface configuration step did not complete")
	}
	plq.ClearAll(t, q.dut, plqRPCTimeout)

	caps := plq.Capabilities(t, q.dut, plqRPCTimeout)
	q.timing, q.mode = plq.ComputeTiming(t, caps)
	q.pairs = linkPairs(t, q.dut, q.idSuffix)
	t.Logf("Creating concurrent link qualification on %d link pairs with %v reflectors: %+v", len(q.pairs), q.mode, q.timing)
	plq.Create(t, q.dut, q.pairs, q.timing, q.mode, plqRPCTimeout)
	if t.Failed() {
		return
	}
	q.created = true
}

// verifyRunning verifies that all qualification IDs reach
// QUALIFICATION_STATE_RUNNING and that all participating ports report
// oper-status TESTING.
func (q *qualification) verifyRunning(t *testing.T) {
	q.requireCreated(t)
	t.Log("Verifying that all qualification IDs reach QUALIFICATION_STATE_RUNNING")
	plq.AwaitState(t, q.dut, plq.IDs(q.pairs), plqpb.QualificationState_QUALIFICATION_STATE_RUNNING, q.timing.SetupTime()+runningMargin, plqRPCTimeout)
	q.running = true

	if deviations.SkipPlqInterfaceOperStatusCheck(q.dut) {
		t.Log("Skipping the oper-status TESTING check (skip_plq_interface_oper_status_check deviation)")
		return
	}
	t.Log("Verifying oper-status TESTING on all generator and reflector ports")
	helpers.ValidateOperStatusIntfs(t, q.dut, plq.Ports(q.pairs), oc.Interface_OperStatus_TESTING, testingTimeout)
}

// verifyResults waits for all qualification IDs to complete and validates
// their results.
func (q *qualification) verifyResults(t *testing.T) {
	q.requireCreated(t)
	if !q.running {
		t.Skip("Skipping: the qualifications did not reach QUALIFICATION_STATE_RUNNING")
	}
	maxWait := q.timing.MaxCompletionTime() + completionMargin
	t.Logf("Waiting up to %v for all qualification IDs to reach QUALIFICATION_STATE_COMPLETED", maxWait)
	plq.AwaitState(t, q.dut, plq.IDs(q.pairs), plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED, maxWait, plqRPCTimeout)

	t.Log("Retrieving and validating all qualification results with a batched Get RPC")
	plq.VerifyResults(t, q.dut, q.pairs, q.mode, plqTolerancePct, plqRPCTimeout)
}

// deleteAndRecover deletes the qualification records and verifies that the
// interfaces recover to oper-status UP.
func (q *qualification) deleteAndRecover(t *testing.T) {
	q.requireCreated(t)
	t.Log("Deleting all qualification IDs and verifying that no qualification records remain")
	plq.DeleteAndVerify(t, q.dut, plq.IDs(q.pairs), plqRPCTimeout)

	t.Logf("Verifying oper-status UP on %v", q.recoveryIntfs)
	helpers.ValidateOperStatusUPIntfs(t, q.dut, q.recoveryIntfs, lagConvergeTimeout)
}

// requireCreated skips the calling step if the Create step did not complete.
func (q *qualification) requireCreated(t *testing.T) {
	t.Helper()
	if !q.created {
		t.Skip("Skipping: the Create step did not complete")
	}
}

// linkPairs returns the port1->port2 (400G) and port3->port4 (100G) generator
// and reflector pairs, with qualification IDs ending in suffix.
func linkPairs(t *testing.T, dut *ondatra.DUTDevice, suffix string) []plq.LinkPair {
	t.Helper()
	packetSize := uint32(plq.MinGeneratorMTU(dut))
	pair := func(generator, reflector string) plq.LinkPair {
		gen, ref := dut.Port(t, generator).Name(), dut.Port(t, reflector).Name()
		id := fmt.Sprintf("%s:%s-%s:%s%s", dut.Name(), gen, dut.Name(), ref, suffix)
		return plq.LinkPair{
			GeneratorPort: gen,
			ReflectorPort: ref,
			GeneratorID:   id + "-generator",
			ReflectorID:   id + "-reflector",
			PacketRate:    generatorPacketRate,
			PacketSize:    packetSize,
		}
	}
	return []plq.LinkPair{pair("port1", "port2"), pair("port3", "port4")}
}

// TestLinkQualification validates gNOI-2.1.2 (Concurrent Multi-Link
// Qualification on Singleton Interfaces), gNOI-2.1.3 (Concurrent Multi-Link
// Qualification on Aggregate Member Links), and Cleanup.
func TestLinkQualification(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	env := setupEnvironment(t, dut)

	type phase func(*qualification, *testing.T)
	type step struct {
		name   string
		phases []phase
	}
	cases := []struct {
		name     string
		idSuffix string
		steps    []step
	}{
		{
			name: "gNOI-2.1.2 Concurrent Multi-Link Qualification on Singleton Interfaces",
			steps: []step{
				{name: "Step 1 - Interface Configuration and Pre-Test Baseline", phases: []phase{(*qualification).configureSingletons}},
				{name: "Step 2 - Concurrent Multi-Link Qualification Start (Create RPC)", phases: []phase{(*qualification).create}},
				{name: "Step 3 - Concurrent Active State and Telemetry Verification (List RPC)", phases: []phase{(*qualification).verifyRunning}},
				{name: "Step 4 - Concurrent Result Retrieval and Counter Validation (Get RPC)", phases: []phase{(*qualification).verifyResults}},
				{name: "Step 5 - Cleanup and Interface State Restoration (Delete RPC)", phases: []phase{(*qualification).deleteAndRecover}},
			},
		},
		{
			name:     "gNOI-2.1.3 Concurrent Multi-Link Qualification on Aggregate Member Links",
			idSuffix: "-aggregate",
			steps: []step{
				{name: "Step 1 - Aggregate and Member-Link Configuration", phases: []phase{(*qualification).configureAggregates}},
				{name: "Step 2 - Concurrent Member-Link Qualification (Create, List, Get RPCs)", phases: []phase{(*qualification).create, (*qualification).verifyRunning, (*qualification).verifyResults}},
				{name: "Step 3 - Cleanup and LAG Recovery Verification (Delete RPC)", phases: []phase{(*qualification).deleteAndRecover}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := &qualification{dut: dut, env: env, idSuffix: tc.idSuffix}
			for _, s := range tc.steps {
				t.Run(s.name, func(t *testing.T) {
					for _, p := range s.phases {
						p(q, t)
					}
				})
			}
		})
	}
}
