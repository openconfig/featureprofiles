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

package isis_interface_passive_test

import (
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/openconfig/featureprofiles/feature/experimental/isis/otg_tests/internal/session"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	plenIPv4 = 30
	plenIPv6 = 126
	v4Metric = 100
	v6Metric = 100
	password = "google"
)

// configureISIS configures isis configs on ts.DUT.
func configureISIS(t *testing.T, ts *session.TestSession) {
	t.Helper()
	d := ts.DUTConf
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(ts.DUT))
	prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, session.ISISName)
	prot.Enabled = ygot.Bool(true)

	isis := prot.GetOrCreateIsis()
	globalIsis := isis.GetOrCreateGlobal()

	// Global configs.
	globalIsis.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalIsis.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalIsis.LevelCapability = oc.Isis_LevelType_LEVEL_2
	globalIsis.AuthenticationCheck = ygot.Bool(true)
	globalIsis.HelloPadding = oc.Isis_HelloPaddingType_ADAPTIVE

	// Level configs.
	level := isis.GetOrCreateLevel(2)

	// Authentication configs.
	auth := level.GetOrCreateAuthentication()
	auth.Enabled = ygot.Bool(true)
	auth.AuthMode = oc.IsisTypes_AUTH_MODE_MD5
	auth.AuthType = oc.KeychainTypes_AUTH_TYPE_SIMPLE_KEY
	auth.AuthPassword = ygot.String(password)

	// Interface configs.
	intfName := ts.DUTPort1.Name()
	if deviations.ExplicitInterfaceInDefaultVRF(ts.DUT) {
		intfName += ".0"
	}
	intf := isis.GetOrCreateInterface(intfName)

	// Interface timers.
	isisIntfTimers := intf.GetOrCreateTimers()
	isisIntfTimers.CsnpInterval = ygot.Uint16(5)
	if deviations.ISISTimersCsnpIntervalUnsupported(ts.DUT) {
		isisIntfTimers.CsnpInterval = nil
	}
	isisIntfTimers.LspPacingInterval = ygot.Uint64(150)

	// Interface level configs.
	isisIntfLevel := intf.GetOrCreateLevel(2)
	isisIntfLevel.GetOrCreateHelloAuthentication().Enabled = ygot.Bool(true)
	isisIntfLevel.GetHelloAuthentication().AuthPassword = ygot.String(password)
	isisIntfLevel.GetHelloAuthentication().AuthType = oc.KeychainTypes_AUTH_TYPE_SIMPLE_KEY
	isisIntfLevel.GetHelloAuthentication().AuthMode = oc.IsisTypes_AUTH_MODE_MD5

	isisIntfLevelTimers := isisIntfLevel.GetOrCreateTimers()
	isisIntfLevelTimers.HelloInterval = ygot.Uint32(5)
	isisIntfLevelTimers.HelloMultiplier = ygot.Uint8(3)

	isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric = ygot.Uint32(v4Metric)
	isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric = ygot.Uint32(v6Metric)
}

// configureOTG configures the interfaces and isis protocol on ATE.
func configureOTG(t *testing.T, ts *session.TestSession) {
	t.Helper()
	ts.ATEIntf1.Isis().RouterAuth().AreaAuth().SetAuthType("md5").SetMd5(password)
	ts.ATEIntf1.Isis().RouterAuth().DomainAuth().SetAuthType("md5").SetMd5(password)
	ts.ATEIntf1.Isis().Basic().SetEnableWideMetric(true).SetLearnedLspFilter(false)
	ts.ATEIntf1.Isis().Advanced().SetEnableHelloPadding(true)

	ts.ATEIntf1.Isis().Interfaces().Items()[0].Authentication().SetAuthType("md5").SetMd5(password)
}

// TestIsisInterfacePassive verifies passive isis interface.
func TestIsisInterfacePassive(t *testing.T) {
	ts := session.MustNew(t).WithISIS()

	// Configure isis on dut.
	configureISIS(t, ts)

	configureOTG(t, ts)
	pcl := ts.DUTConf.GetNetworkInstance(deviations.DefaultNetworkInstance(ts.DUT)).GetProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, session.ISISName)
	fptest.LogQuery(t, "Protocol ISIS", session.ProtocolPath(ts.DUT).Config(), pcl)

	ts.PushAndStart(t)

	_, err := ts.AwaitAdjacency()
	if err != nil {
		t.Fatalf("Adjacency state invalid: %v", err)
	}

	statePath := session.ISISPath(ts.DUT)
	intfName := ts.DUTPort1.Name()
	if deviations.ExplicitInterfaceInDefaultVRF(ts.DUT) {
		intfName += ".0"
	}
	t.Run("Isis telemetry", func(t *testing.T) {

		adjacencyPath := statePath.Interface(intfName).Level(2).AdjacencyAny().AdjacencyState().State()

		_, ok := gnmi.WatchAll(t, ts.DUT, adjacencyPath, time.Minute, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
			state, present := val.Val()
			return present && state == oc.Isis_IsisInterfaceAdjState_UP
		}).Await(t)
		if !ok {
			t.Fatalf("No isis adjacency reported on interface %v", intfName)
		}
		// Getting neighbors sysid.
		sysid := gnmi.GetAll(t, ts.DUT, statePath.Interface(intfName).Level(2).AdjacencyAny().SystemId().State())
		ateSysID := sysid[0]

		t.Run("Afi-Safi checks", func(t *testing.T) {
			// Checking v4 afi name.
			if got := gnmi.Get(t, ts.DUT, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).AfiName().State()); got != oc.IsisTypes_AFI_TYPE_IPV4 {
				t.Errorf("FAIL- Expected afi name not found, got %d, want %d", got, oc.IsisTypes_AFI_TYPE_IPV4)
			}
			// Checking v4 safi name.
			if got := gnmi.Get(t, ts.DUT, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).SafiName().State()); got != oc.IsisTypes_SAFI_TYPE_UNICAST {
				t.Errorf("FAIL- Expected safi name not found, got %d, want %d", got, oc.IsisTypes_SAFI_TYPE_UNICAST)
			}
			// Checking v6 afi name.
			if got := gnmi.Get(t, ts.DUT, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).AfiName().State()); got != oc.IsisTypes_AFI_TYPE_IPV6 {
				t.Errorf("FAIL- Expected afi name not found, got %d, want %d", got, oc.IsisTypes_AFI_TYPE_IPV6)
			}
			// Checking v6 safi name.
			if got := gnmi.Get(t, ts.DUT, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).SafiName().State()); got != oc.IsisTypes_SAFI_TYPE_UNICAST {
				t.Errorf("FAIL- Expected safi name not found, got %d, want %d", got, oc.IsisTypes_SAFI_TYPE_UNICAST)
			}
			// Checking v4 unicast metric.
			if got := gnmi.Get(t, ts.DUT, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric().State()); got != v4Metric {
				t.Errorf("FAIL- Expected v4 unicast metric value not found, got %d, want %d", got, v4Metric)
			}
			// Checking v6 unicast metric.
			if got := gnmi.Get(t, ts.DUT, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric().State()); got != v6Metric {
				t.Errorf("FAIL- Expected v6 unicast metric value not found, got %d, want %d", got, v6Metric)
			}
		})
		t.Run("Adjacency state checks", func(t *testing.T) {
			adjPath := statePath.Interface(intfName).Level(2).Adjacency(ateSysID)

			// Checking neighbor sysid.
			if got := gnmi.Get(t, ts.DUT, adjPath.SystemId().State()); got != ateSysID {
				t.Errorf("FAIL- Expected neighbor system id not found, got %s, want %s", got, ateSysID)
			}
			// Checking isis area address.
			want := []string{session.ATEAreaAddress, session.DUTAreaAddress}
			if got := gnmi.Get(t, ts.DUT, adjPath.AreaAddress().State()); !cmp.Equal(got, want, cmpopts.SortSlices(func(a, b string) bool { return a < b })) {
				t.Errorf("FAIL- Expected area address not found, got %s, want %s", got, want)
			}
			// Checking dis system id.
			if got := gnmi.Get(t, ts.DUT, adjPath.DisSystemId().State()); got != "0000.0000.0000" {
				t.Errorf("FAIL- Expected dis system id not found, got %s, want %s", got, "0000.0000.0000")
			}
			// Checking isis local extended circuit id.
			if got := gnmi.Get(t, ts.DUT, adjPath.LocalExtendedCircuitId().State()); got == 0 {
				t.Errorf("FAIL- Expected local extended circuit id not found,expected non-zero value, got %d", got)
			}
			// Checking multitopology.
			if got := gnmi.Get(t, ts.DUT, adjPath.MultiTopology().State()); got != false {
				t.Errorf("FAIL- Expected value for multi topology not found, got %t, want %t", got, false)
			}
			// Checking neighbor circuit type.
			if got := gnmi.Get(t, ts.DUT, adjPath.NeighborCircuitType().State()); got != oc.Isis_LevelType_LEVEL_2 {
				t.Errorf("FAIL- Expected value for circuit type not found, got %s, want %s", got, oc.Isis_LevelType_LEVEL_2)
			}
			// Checking neighbor ipv4 address.
			if got := gnmi.Get(t, ts.DUT, adjPath.NeighborIpv4Address().State()); got != session.ATEISISAttrs.IPv4 {
				t.Errorf("FAIL- Expected value for ipv4 address not found, got %s, want %s", got, session.ATEISISAttrs.IPv4)
			}
			// Checking isis neighbor extended circuit id.
			if got := gnmi.Get(t, ts.DUT, adjPath.NeighborExtendedCircuitId().State()); got == 0 {
				t.Errorf("FAIL- Expected neighbor extended circuit id not found,expected non-zero value, got %d", got)
			}
			// Checking neighbor snpa.
			snpaAddress := gnmi.Get(t, ts.DUT, adjPath.NeighborSnpa().State())
			mac, err := net.ParseMAC(snpaAddress)
			if !(mac != nil && err == nil) {
				t.Errorf("FAIL- Expected value for snpa address not found, got %s", snpaAddress)
			}
			// Checking isis adjacency address families.
			if got := gnmi.Get(t, ts.DUT, adjPath.Nlpid().State()); !cmp.Equal(got, []oc.E_Adjacency_Nlpid{oc.Adjacency_Nlpid_IPV4, oc.Adjacency_Nlpid_IPV6}) {
				t.Errorf("FAIL- Expected address families not found, got %s, want %s", got, []oc.E_Adjacency_Nlpid{oc.Adjacency_Nlpid_IPV4, oc.Adjacency_Nlpid_IPV6})
			}
			// Checking neighbor ipv6 address.
			ipv6Address := gnmi.Get(t, ts.DUT, adjPath.NeighborIpv6Address().State())
			ip := net.ParseIP(ipv6Address)
			if !(ip != nil && ip.To16() != nil) {
				t.Errorf("FAIL- Expected ipv6 address not found, got %s", ipv6Address)
			}
			// Checking isis priority.
			if _, ok := gnmi.Lookup(t, ts.DUT, adjPath.Priority().State()).Val(); !ok {
				t.Errorf("FAIL- Priority is not present")
			}
			// Checking isis restart status.
			if _, ok := gnmi.Lookup(t, ts.DUT, adjPath.RestartStatus().State()).Val(); !ok {
				t.Errorf("FAIL- Restart status not present")
			}
			// Checking isis restart support.
			if _, ok := gnmi.Lookup(t, ts.DUT, adjPath.RestartSupport().State()).Val(); !ok {
				t.Errorf("FAIL- Restart support not present")
			}
			// Checking isis restart suppress.
			if _, ok := gnmi.Lookup(t, ts.DUT, adjPath.RestartStatus().State()).Val(); !ok {
				t.Errorf("FAIL- Restart suppress not present")
			}
		})
		t.Run("System level counter checks", func(t *testing.T) {
			// Checking authFail counters.
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().AuthFails().State()); got != 0 {
				t.Errorf("FAIL- Not expecting any authentication key failure, got %d, want %d", got, 0)
			}
			// Checking authTypeFail counters.
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().AuthTypeFails().State()); got != 0 {
				t.Errorf("FAIL- Not expecting any authentication type mismatches, got %d, want %d", got, 0)
			}
			// Checking corrupted lsps counters.
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().CorruptedLsps().State()); got != 0 {
				t.Errorf("FAIL- Not expecting any corrupted lsps, got %d, want %d", got, 0)
			}
			// Checking database_overloads counters.
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().DatabaseOverloads().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero database_overloads, got %d, want %d", got, 0)
			}
			// Checking execeeded maximum seq number counters").
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().ExceedMaxSeqNums().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero max_seqnum counter, got %d, want %d", got, 0)
			}
			// Checking IdLenMismatch counters.
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().IdLenMismatch().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero IdLen_Mismatch counter, got %d, want %d", got, 0)
			}
			// Checking LspErrors counters.
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().LspErrors().State()); got != 0 {
				t.Errorf("FAIL- Not expecting any lsp errors, got %d, want %d", got, 0)
			}
			// Checking MaxAreaAddressMismatches counters.
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().MaxAreaAddressMismatches().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero MaxAreaAddressMismatches counter, got %d, want %d", got, 0)
			}
			// Checking OwnLspPurges counters.
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().OwnLspPurges().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero OwnLspPurges counter, got %d, want %d", got, 0)
			}
			// Checking SeqNumSkips counters.
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().SeqNumSkips().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero SeqNumber skips, got %d, want %d", got, 0)
			}
			// Checking ManualAddressDropFromAreas counters.
			if !deviations.ISISCounterManualAddressDropFromAreasUnsupported(ts.DUT) {
				if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().ManualAddressDropFromAreas().State()); got != 0 {
					t.Errorf("FAIL- Not expecting non zero ManualAddressDropFromAreas counter, got %d, want %d", got, 0)
				}
			}
			// Checking PartChanges counters.
			if !deviations.ISISCounterPartChangesUnsupported(ts.DUT) {
				if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().PartChanges().State()); got != 0 {
					t.Errorf("FAIL- Not expecting partition changes, got %d, want %d", got, 0)
				}
			}
			// Checking SpfRuns counters.
			if got := gnmi.Get(t, ts.DUT, statePath.Level(2).SystemLevelCounters().SpfRuns().State()); got == 0 {
				t.Errorf("FAIL- Not expecting spf runs counter to be 0, got %d, want non zero", got)
			}
		})
		t.Run("Passive interface checks", func(t *testing.T) {
			// Pushing passive config to ts.DUT.
			gnmi.Update(t, ts.DUT, statePath.Interface(intfName).Passive().Config(), true)

			// Checking passive telemetry.
			if got := gnmi.Get(t, ts.DUT, statePath.Interface(intfName).Passive().State()); got != true {
				t.Errorf("FAIL- Expected value for passive not found on isis interface, got %t, want %t", got, true)
			}
			l2 := statePath.Interface(intfName).Level(2).State()
			_, ok := gnmi.Watch(t, ts.DUT, l2, time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Protocol_Isis_Interface_Level]) bool {
				state, present := val.Val()
				if !present {
					return false
				}

				for _, adj := range state.Adjacency {
					if adj.GetAdjacencyState() != oc.Isis_IsisInterfaceAdjState_DOWN {
						return false
					}
				}
				return true
			}).Await(t)
			if !ok {
				t.Errorf("FAIL-isis adjacency on %s with level 2 is not down", intfName)
			}
		})
	})
}
