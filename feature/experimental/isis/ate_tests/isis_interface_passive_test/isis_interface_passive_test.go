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
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	plenIPv4       = 30
	plenIPv6       = 126
	ISISInstance   = "DEFAULT"
	dutAreaAddress = "49.0001"
	ateAreaAddress = "49.0002"
	dutSysID       = "1920.0000.2001"
	v4_metric      = 100
	v6_metric      = 100
	password       = "google"
)

var (
	dutPort1Attr = attrs.Attributes{
		Desc:    "DUT to ATE link1 ",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort1attr = attrs.Attributes{
		Name:    "ATE to DUT port1 ",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
)

// configureDUT configures all the interfaces on the DUT.
func configureDUT(t *testing.T) {

	dc := gnmi.OC()
	dut := ondatra.DUT(t, "dut")
	i1 := dutPort1Attr.NewOCInterface(dut.Port(t, "port1").Name(), dut)

	t.Log("Pushing interface config on DUT")
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
	}
}

// configureIsisDut configures isis configs on DUT.
func configureIsisDut(t *testing.T, dut *ondatra.DUTDevice, intfName string, dutAreaAddress, dutSysID string) {

	d := &oc.Root{}
	configPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, ISISInstance)
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, ISISInstance)

	if !deviations.ISISprotocolEnabledNotRequired(dut) {
		prot.Enabled = ygot.Bool(true)
	}
	isis := prot.GetOrCreateIsis()
	globalIsis := isis.GetOrCreateGlobal()

	// Global configs.
	globalIsis.Net = []string{fmt.Sprintf("%v.%v.00", dutAreaAddress, dutSysID)}
	globalIsis.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalIsis.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalIsis.LevelCapability = oc.Isis_LevelType_LEVEL_2
	globalIsis.AuthenticationCheck = ygot.Bool(true)
	globalIsis.HelloPadding = oc.Isis_HelloPaddingType_LOOSE

	// Level configs.
	level := isis.GetOrCreateLevel(2)
	level.Enabled = ygot.Bool(true)
	level.LevelNumber = ygot.Uint8(2)

	// Authentication configs.
	auth := level.GetOrCreateAuthentication()
	auth.Enabled = ygot.Bool(true)
	auth.AuthMode = oc.IsisTypes_AUTH_MODE_MD5
	auth.AuthType = oc.KeychainTypes_AUTH_TYPE_SIMPLE_KEY
	auth.AuthPassword = ygot.String(password)

	// Interface configs.
	intf := isis.GetOrCreateInterface(intfName)
	intf.Enabled = ygot.Bool(true)
	intf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
	intf.InterfaceId = &intfName

	// Interface timers.
	isisIntfTimers := intf.GetOrCreateTimers()
	isisIntfTimers.CsnpInterval = ygot.Uint16(5)
	isisIntfTimers.LspPacingInterval = ygot.Uint64(150)

	// Interface level configs.
	isisIntfLevel := intf.GetOrCreateLevel(2)
	isisIntfLevel.LevelNumber = ygot.Uint8(2)
	isisIntfLevel.GetOrCreateHelloAuthentication().Enabled = ygot.Bool(true)
	isisIntfLevel.GetHelloAuthentication().AuthPassword = ygot.String(password)
	isisIntfLevel.GetHelloAuthentication().AuthType = oc.KeychainTypes_AUTH_TYPE_SIMPLE_KEY
	isisIntfLevel.GetHelloAuthentication().AuthMode = oc.IsisTypes_AUTH_MODE_MD5

	isisIntfLevelTimers := isisIntfLevel.GetOrCreateTimers()
	isisIntfLevelTimers.HelloInterval = ygot.Uint32(5)
	isisIntfLevelTimers.HelloMultiplier = ygot.Uint8(3)

	isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric = ygot.Uint32(v4_metric)
	isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric = ygot.Uint32(v6_metric)

	// Interface afi-safi configs.
	intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)

	t.Log("Pushing isis config on DUT")
	gnmi.Replace(t, dut, configPath.Config(), prot)
}

// configureATE configures the interfaces and isis protocol on ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {

	topo := ate.Topology().New()
	port1 := ate.Port(t, "port1")
	i1Dut := topo.AddInterface(atePort1attr.Name).WithPort(port1)
	i1Dut.IPv4().WithAddress(atePort1attr.IPv4CIDR()).WithDefaultGateway(dutPort1Attr.IPv4)
	i1Dut.IPv6().WithAddress(atePort1attr.IPv6CIDR()).WithDefaultGateway(dutPort1Attr.IPv6)

	isisDut := i1Dut.ISIS()
	isisDut.
		WithAreaID(ateAreaAddress).
		WithTERouterID(atePort1attr.IPv4).
		WithNetworkTypePointToPoint().
		WithLevelL2().
		WithAuthMD5(password).
		WithAreaAuthMD5(password).
		WithDomainAuthMD5(password).
		WithHelloPaddingEnabled(true)

	t.Log("Pushing config to ATE and starting protocols...")
	topo.Push(t)
	topo.StartProtocols(t)
	return topo
}

// TestIsisInterfacePassive verifies passive isis interface.
func TestIsisInterfacePassive(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	intfName := dut.Port(t, "port1").Name()

	// Configure interface on the DUT.
	configureDUT(t)

	// Configure network Instance type on DUT.
	dutConfNIPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	t.Log("Pushing network Instance type config on DUT")
	gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

	// Configure isis on DUT.
	configureIsisDut(t, dut, intfName, dutAreaAddress, dutSysID)

	// Configure interface,isis and traffic on ATE.
	configureATE(t, ate)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		intfName = intfName + ".0"
	}
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, ISISInstance).Isis()

	t.Run("Isis telemetry", func(t *testing.T) {
		t.Run("Verifying adjacency", func(t *testing.T) {
			adjacencyPath := statePath.Interface(intfName).Level(2).AdjacencyAny().AdjacencyState().State()

			_, ok := gnmi.WatchAll(t, dut, adjacencyPath, time.Minute, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
				state, present := val.Val()
				return present && state == oc.Isis_IsisInterfaceAdjState_UP
			}).Await(t)
			if !ok {
				t.Logf("Interface %v has no level 2 isis adjacency", intfName)
				t.Fatal("No isis adjacency reported.")
			}
		})
		// Getting neighbors sysid.
		sysid := gnmi.GetAll(t, dut, statePath.Interface(intfName).Level(2).AdjacencyAny().SystemId().State())
		ateSysID := sysid[0]

		t.Run("Afi-Safi checks", func(t *testing.T) {
			// Checking v4 afi name.
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).AfiName().State()); got != oc.IsisTypes_AFI_TYPE_IPV4 {
				t.Errorf("FAIL- Expected afi name not found,got %d,want %d", got, oc.IsisTypes_AFI_TYPE_IPV4)
			}
			// Checking v4 safi name.
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).SafiName().State()); got != oc.IsisTypes_SAFI_TYPE_UNICAST {
				t.Errorf("FAIL- Expected safi name not found,got %d,want %d", got, oc.IsisTypes_SAFI_TYPE_UNICAST)
			}
			// Checking v6 afi name.
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).AfiName().State()); got != oc.IsisTypes_AFI_TYPE_IPV6 {
				t.Errorf("FAIL- Expected afi name not found,got %d,want %d", got, oc.IsisTypes_AFI_TYPE_IPV6)
			}
			// Checking v6 safi name.
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).SafiName().State()); got != oc.IsisTypes_SAFI_TYPE_UNICAST {
				t.Errorf("FAIL- Expected safi name not found,got %d,want %d", got, oc.IsisTypes_SAFI_TYPE_UNICAST)
			}
			// Checking v4 unicast metric.
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric().State()); got != v4_metric {
				t.Errorf("FAIL- Expected v4 unicast metric value not found,got %d,want %d", got, v4_metric)
			}
			// Checking v6 unicast metric.
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric().State()); got != v6_metric {
				t.Errorf("FAIL- Expected v6 unicast metric value not found,got %d,want %d", got, v6_metric)
			}
		})
		t.Run("Adjacency state checks", func(t *testing.T) {
			adjPath := statePath.Interface(intfName).Level(2).Adjacency(ateSysID)

			// Checking neighbor sysid.
			if got := gnmi.Get(t, dut, adjPath.SystemId().State()); got != ateSysID {
				t.Errorf("FAIL- Expected neighbor system id not found,got %s,want %s", got, ateSysID)
			}
			// Checking isis area address.
			want := []string{ateAreaAddress, dutAreaAddress}
			if got := gnmi.Get(t, dut, adjPath.AreaAddress().State()); !cmp.Equal(got, want, cmpopts.SortSlices(func(a, b string) bool { return a < b })) {
				t.Errorf("FAIL- Expected area address not found,got %s,want %s", got, want)
			}
			// Checking dis system id.
			if got := gnmi.Get(t, dut, adjPath.DisSystemId().State()); got != "0000.0000.0000" {
				t.Errorf("FAIL- Expected dis system id not found,got %s,want %s", got, "0000.0000.0000")
			}
			// Checking isis local extended circuit id.
			if got := gnmi.Get(t, dut, adjPath.LocalExtendedCircuitId().State()); got == 0 {
				t.Errorf("FAIL- Expected local extended circuit id not found,expected non-zero value,got %d", got)
			}
			// Checking multitopology.
			if got := gnmi.Get(t, dut, adjPath.MultiTopology().State()); got != false {
				t.Errorf("FAIL- Expected value for multi topology not found,got %t,want %t", got, false)
			}
			// Checking neighbor circuit type.
			if got := gnmi.Get(t, dut, adjPath.NeighborCircuitType().State()); got != oc.Isis_LevelType_LEVEL_2 {
				t.Errorf("FAIL- Expected value for circuit type not found,got %s,want %s", got, oc.Isis_LevelType_LEVEL_2)
			}
			// Checking neighbor ipv4 address.
			if got := gnmi.Get(t, dut, adjPath.NeighborIpv4Address().State()); got != atePort1attr.IPv4 {
				t.Errorf("FAIL- Expected value for ipv4 address not found,got %s,want %s", got, atePort1attr.IPv4)
			}
			// Checking isis neighbor extended circuit id.
			if got := gnmi.Get(t, dut, adjPath.NeighborExtendedCircuitId().State()); got == 0 {
				t.Errorf("FAIL- Expected neighbor extended circuit id not found,expected non-zero value,got %d", got)
			}
			// Checking neighbor snpa.
			snpa_address := gnmi.Get(t, dut, adjPath.NeighborSnpa().State())
			mac, err := net.ParseMAC(snpa_address)
			if !(mac != nil && err == nil) {
				t.Errorf("FAIL- Expected value for snpa address not found,got %s", snpa_address)
			}
			// Checking isis adjacency address families.
			if got := gnmi.Get(t, dut, adjPath.Nlpid().State()); !cmp.Equal(got, []oc.E_Adjacency_Nlpid{oc.Adjacency_Nlpid_IPV4, oc.Adjacency_Nlpid_IPV6}) {
				t.Errorf("FAIL- Expected address families not found,got %s,want %s", got, []oc.E_Adjacency_Nlpid{oc.Adjacency_Nlpid_IPV4, oc.Adjacency_Nlpid_IPV6})
			}
			// Checking neighbor ipv6 address.
			ipv6_address := gnmi.Get(t, dut, adjPath.NeighborIpv6Address().State())
			ip := net.ParseIP(ipv6_address)
			if !(ip != nil && ip.To16() != nil) {
				t.Errorf("FAIL- Expected ipv6 address not found,got %s", ipv6_address)
			}
			// Checking isis priority.
			_, present := gnmi.Lookup(t, dut, adjPath.Priority().State()).Val()
			if !present {
				t.Errorf("FAIL- Priority is not present")
			}
			// Checking isis restart status.
			_, chk_present := gnmi.Lookup(t, dut, adjPath.RestartStatus().State()).Val()
			if !chk_present {
				t.Errorf("FAIL- Restart status not present")
			}
			// Checking isis restart support.
			_, check_present := gnmi.Lookup(t, dut, adjPath.RestartSupport().State()).Val()
			if !check_present {
				t.Errorf("FAIL- Restart support not present")
			}
			// Checking isis restart suppress.
			_, checks_present := gnmi.Lookup(t, dut, adjPath.RestartStatus().State()).Val()
			if !checks_present {
				t.Errorf("FAIL- Restart suppress not present")
			}
		})
		t.Run("System level counter checks", func(t *testing.T) {
			// Checking authFail counters.
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().AuthFails().State()); got != 0 {
				t.Errorf("FAIL- Not expecting any authentication key failure,got %d,want %d", got, 0)
			}
			// Checking authTypeFail counters.
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().AuthTypeFails().State()); got != 0 {
				t.Errorf("FAIL- Not expecting any authentication type mismatches,got %d,want %d", got, 0)
			}
			// Checking corrupted lsps counters.
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().CorruptedLsps().State()); got != 0 {
				t.Errorf("FAIL- Not expecting any corrupted lsps,got %d,want %d", got, 0)
			}
			// Checking database_overloads counters.
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().DatabaseOverloads().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero database_overloads,got %d,want %d", got, 0)
			}
			// Checking execeeded maximum seq number counters").
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().ExceedMaxSeqNums().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero max_seqnum counter,got %d,want %d", got, 0)
			}
			// Checking IdLenMismatch counters.
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().IdLenMismatch().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero IdLen_Mismatch counter,got %d,want %d", got, 0)
			}
			// Checking LspErrors counters.
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().LspErrors().State()); got != 0 {
				t.Errorf("FAIL- Not expecting any lsp errors,got %d,want %d", got, 0)
			}
			// Checking MaxAreaAddressMismatches counters.
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().MaxAreaAddressMismatches().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero MaxAreaAddressMismatches counter,got %d,want %d", got, 0)
			}
			// Checking OwnLspPurges counters.
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().OwnLspPurges().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero OwnLspPurges counter,got %d,want %d", got, 0)
			}
			// Checking SeqNumSkips counters.
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().SeqNumSkips().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero SeqNumber skips,got %d,want %d", got, 0)
			}
			// Checking ManualAddressDropFromAreas counters.
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().ManualAddressDropFromAreas().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero ManualAddressDropFromAreas counter,got %d,want %d", got, 0)
			}
			// Checking PartChanges counters.
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().PartChanges().State()); got != 0 {
				t.Errorf("FAIL- Not expecting partition changes,got %d,want %d", got, 0)
			}
			// Checking SpfRuns counters.
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().SpfRuns().State()); got == 0 {
				t.Errorf("FAIL- Not expecting spf runs counter to be 0,got %d,want non zero", got)
			}
		})
		t.Run("Passive interface checks", func(t *testing.T) {
			// Pushing passive config to DUT.
			gnmi.Update(t, dut, statePath.Interface(intfName).Passive().Config(), true)
			gnmi.Update(t, dut, statePath.Interface(intfName).Level(2).Passive().Config(), false)

			// Checking passive telemetry.
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).Passive().State()); got != true {
				t.Errorf("FAIL- Expected value for passive not found on isis interface,got %t,want %t", got, true)
			}
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).Level(2).Passive().State()); got != true {
				t.Errorf("FAIL- Expected value for passive not found on isis interface level,got %t,want %t", got, true)
			}
			// Checking adjacency after configuring interface as passive.
			adjacencyPath := statePath.Interface(intfName).Level(2).AdjacencyAny().AdjacencyState().State()

			_, ok := gnmi.WatchAll(t, dut, adjacencyPath, time.Minute, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
				state, present := val.Val()
				return present && state == oc.Isis_IsisInterfaceAdjState_DOWN
			}).Await(t)
			if !ok {
				t.Errorf("FAIL-isis adjacency on %s with level 2 is not down", intfName)
			}
		})
	})
}
