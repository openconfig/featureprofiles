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
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otg "github.com/openconfig/ondatra/otg"
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
	ateSystemID    = "640000000001"
	v4Metric       = 100
	v6Metric       = 100
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
		MAC:     "02:00:01:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
)

// configureDUT configures all the interfaces on the DUT.
func configureDUT(t *testing.T) {
	t.Helper()
	dc := gnmi.OC()
	dut := ondatra.DUT(t, "dut")
	i1 := dutPort1Attr.NewOCInterface(dut.Port(t, "port1").Name(), dut)

	t.Log("Pushing interface config on DUT")
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
	}
}

// configureISIS configures isis configs on DUT.
func configureISIS(t *testing.T, dut *ondatra.DUTDevice, intfName string, dutAreaAddress, dutSysID string) {
	t.Helper()
	d := &oc.Root{}
	configPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, ISISInstance)
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, ISISInstance)
	prot.Enabled = ygot.Bool(true)

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
	isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric = ygot.Uint32(v4Metric)
	isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric = ygot.Uint32(v6Metric)

	// Interface afi-safi configs.
	intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)

	t.Log("Pushing isis config on DUT")
	gnmi.Replace(t, dut, configPath.Config(), prot)
}

// configureOTG configures the interfaces and isis protocol on ATE.
func configureOTG(t *testing.T, otg *otg.OTG) {
	t.Helper()
	config := otg.NewConfig(t)
	port1 := config.Ports().Add().SetName("port1")

	iDut1Dev := config.Devices().Add().SetName(atePort1attr.Name)
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(atePort1attr.Name + ".Eth").SetMac(atePort1attr.MAC)
	iDut1Eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName(atePort1attr.Name + ".IPv4")
	iDut1Ipv4.SetAddress(atePort1attr.IPv4).SetGateway(dutPort1Attr.IPv4).SetPrefix(int32(atePort1attr.IPv4Len))
	iDut1Ipv6 := iDut1Eth.Ipv6Addresses().Add().SetName(atePort1attr.Name + ".IPv6")
	iDut1Ipv6.SetAddress(atePort1attr.IPv6).SetGateway(dutPort1Attr.IPv6).SetPrefix(int32(atePort1attr.IPv6Len))

	iDut1Dev.Isis().SetSystemId(ateSystemID).SetName("devIsis").RouterAuth().AreaAuth().SetAuthType("md5").SetMd5(password)
	iDut1Dev.Isis().RouterAuth().DomainAuth().SetAuthType("md5").SetMd5(password)
	iDut1Dev.Isis().Basic().SetIpv4TeRouterId(atePort1attr.IPv4).SetEnableWideMetric(false)
	iDut1Dev.Isis().Advanced().SetAreaAddresses([]string{strings.Replace(ateAreaAddress, ".", "", -1)}).SetEnableHelloPadding(true)

	iDut1Dev.Isis().Interfaces().
		Add().
		SetEthName(iDut1Dev.Ethernets().Items()[0].Name()).
		SetName("devIsisInt").
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetMetric(10).Authentication().SetAuthType("md5").SetMd5(password)

	t.Logf("Pushing config to OTG and starting protocols...")
	otg.PushConfig(t, config)
	time.Sleep(30 * time.Second)
	otg.StartProtocols(t)
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
	configureISIS(t, dut, intfName, dutAreaAddress, dutSysID)

	// Configure interface,isis and traffic on ATE.
	otg := ate.OTG()
	configureOTG(t, otg)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		intfName = intfName + ".0"
	}
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, ISISInstance).Isis()

	t.Run("Isis telemetry", func(t *testing.T) {

		adjacencyPath := statePath.Interface(intfName).Level(2).AdjacencyAny().AdjacencyState().State()

		_, ok := gnmi.WatchAll(t, dut, adjacencyPath, time.Minute, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
			state, present := val.Val()
			return present && state == oc.Isis_IsisInterfaceAdjState_UP
		}).Await(t)
		if !ok {
			t.Fatalf("No isis adjacency reported on interface %v", intfName)
		}
		// Getting neighbors sysid.
		sysid := gnmi.GetAll(t, dut, statePath.Interface(intfName).Level(2).AdjacencyAny().SystemId().State())
		ateSysID := sysid[0]

		t.Run("Afi-Safi checks", func(t *testing.T) {
			// Checking v4 afi name.
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).AfiName().State()); got != oc.IsisTypes_AFI_TYPE_IPV4 {
				t.Errorf("FAIL- Expected afi name not found, got %d, want %d", got, oc.IsisTypes_AFI_TYPE_IPV4)
			}
			// Checking v4 safi name.
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).SafiName().State()); got != oc.IsisTypes_SAFI_TYPE_UNICAST {
				t.Errorf("FAIL- Expected safi name not found, got %d, want %d", got, oc.IsisTypes_SAFI_TYPE_UNICAST)
			}
			// Checking v6 afi name.
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).AfiName().State()); got != oc.IsisTypes_AFI_TYPE_IPV6 {
				t.Errorf("FAIL- Expected afi name not found, got %d, want %d", got, oc.IsisTypes_AFI_TYPE_IPV6)
			}
			// Checking v6 safi name.
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).SafiName().State()); got != oc.IsisTypes_SAFI_TYPE_UNICAST {
				t.Errorf("FAIL- Expected safi name not found, got %d, want %d", got, oc.IsisTypes_SAFI_TYPE_UNICAST)
			}
			// Checking v4 unicast metric.
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric().State()); got != v4Metric {
				t.Errorf("FAIL- Expected v4 unicast metric value not found, got %d, want %d", got, v4Metric)
			}
			// Checking v6 unicast metric.
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric().State()); got != v6Metric {
				t.Errorf("FAIL- Expected v6 unicast metric value not found, got %d, want %d", got, v6Metric)
			}
		})
		t.Run("Adjacency state checks", func(t *testing.T) {
			adjPath := statePath.Interface(intfName).Level(2).Adjacency(ateSysID)

			// Checking neighbor sysid.
			if got := gnmi.Get(t, dut, adjPath.SystemId().State()); got != ateSysID {
				t.Errorf("FAIL- Expected neighbor system id not found, got %s, want %s", got, ateSysID)
			}
			// Checking isis area address.
			want := []string{ateAreaAddress, dutAreaAddress}
			if got := gnmi.Get(t, dut, adjPath.AreaAddress().State()); !cmp.Equal(got, want, cmpopts.SortSlices(func(a, b string) bool { return a < b })) {
				t.Errorf("FAIL- Expected area address not found, got %s, want %s", got, want)
			}
			// Checking dis system id.
			if got := gnmi.Get(t, dut, adjPath.DisSystemId().State()); got != "0000.0000.0000" {
				t.Errorf("FAIL- Expected dis system id not found, got %s, want %s", got, "0000.0000.0000")
			}
			// Checking isis local extended circuit id.
			if got := gnmi.Get(t, dut, adjPath.LocalExtendedCircuitId().State()); got == 0 {
				t.Errorf("FAIL- Expected local extended circuit id not found,expected non-zero value, got %d", got)
			}
			// Checking multitopology.
			if got := gnmi.Get(t, dut, adjPath.MultiTopology().State()); got != false {
				t.Errorf("FAIL- Expected value for multi topology not found, got %t, want %t", got, false)
			}
			// Checking neighbor circuit type.
			if got := gnmi.Get(t, dut, adjPath.NeighborCircuitType().State()); got != oc.Isis_LevelType_LEVEL_2 {
				t.Errorf("FAIL- Expected value for circuit type not found, got %s, want %s", got, oc.Isis_LevelType_LEVEL_2)
			}
			// Checking neighbor ipv4 address.
			if got := gnmi.Get(t, dut, adjPath.NeighborIpv4Address().State()); got != atePort1attr.IPv4 {
				t.Errorf("FAIL- Expected value for ipv4 address not found, got %s, want %s", got, atePort1attr.IPv4)
			}
			// Checking isis neighbor extended circuit id.
			if got := gnmi.Get(t, dut, adjPath.NeighborExtendedCircuitId().State()); got == 0 {
				t.Errorf("FAIL- Expected neighbor extended circuit id not found,expected non-zero value, got %d", got)
			}
			// Checking neighbor snpa.
			snpaAddress := gnmi.Get(t, dut, adjPath.NeighborSnpa().State())
			mac, err := net.ParseMAC(snpaAddress)
			if !(mac != nil && err == nil) {
				t.Errorf("FAIL- Expected value for snpa address not found, got %s", snpaAddress)
			}
			// Checking isis adjacency address families.
			if got := gnmi.Get(t, dut, adjPath.Nlpid().State()); !cmp.Equal(got, []oc.E_Adjacency_Nlpid{oc.Adjacency_Nlpid_IPV4, oc.Adjacency_Nlpid_IPV6}) {
				t.Errorf("FAIL- Expected address families not found, got %s, want %s", got, []oc.E_Adjacency_Nlpid{oc.Adjacency_Nlpid_IPV4, oc.Adjacency_Nlpid_IPV6})
			}
			// Checking neighbor ipv6 address.
			ipv6Address := gnmi.Get(t, dut, adjPath.NeighborIpv6Address().State())
			ip := net.ParseIP(ipv6Address)
			if !(ip != nil && ip.To16() != nil) {
				t.Errorf("FAIL- Expected ipv6 address not found, got %s", ipv6Address)
			}
			// Checking isis priority.
			if _, ok := gnmi.Lookup(t, dut, adjPath.Priority().State()).Val(); !ok {
				t.Errorf("FAIL- Priority is not present")
			}
			// Checking isis restart status.
			if _, ok := gnmi.Lookup(t, dut, adjPath.RestartStatus().State()).Val(); !ok {
				t.Errorf("FAIL- Restart status not present")
			}
			// Checking isis restart support.
			if _, ok := gnmi.Lookup(t, dut, adjPath.RestartSupport().State()).Val(); !ok {
				t.Errorf("FAIL- Restart support not present")
			}
			// Checking isis restart suppress.
			if _, ok := gnmi.Lookup(t, dut, adjPath.RestartStatus().State()).Val(); !ok {
				t.Errorf("FAIL- Restart suppress not present")
			}
		})
		t.Run("System level counter checks", func(t *testing.T) {
			// Checking authFail counters.
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().AuthFails().State()); got != 0 {
				t.Errorf("FAIL- Not expecting any authentication key failure, got %d, want %d", got, 0)
			}
			// Checking authTypeFail counters.
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().AuthTypeFails().State()); got != 0 {
				t.Errorf("FAIL- Not expecting any authentication type mismatches, got %d, want %d", got, 0)
			}
			// Checking corrupted lsps counters.
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().CorruptedLsps().State()); got != 0 {
				t.Errorf("FAIL- Not expecting any corrupted lsps, got %d, want %d", got, 0)
			}
			// Checking database_overloads counters.
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().DatabaseOverloads().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero database_overloads, got %d, want %d", got, 0)
			}
			// Checking execeeded maximum seq number counters").
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().ExceedMaxSeqNums().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero max_seqnum counter, got %d, want %d", got, 0)
			}
			// Checking IdLenMismatch counters.
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().IdLenMismatch().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero IdLen_Mismatch counter, got %d, want %d", got, 0)
			}
			// Checking LspErrors counters.
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().LspErrors().State()); got != 0 {
				t.Errorf("FAIL- Not expecting any lsp errors, got %d, want %d", got, 0)
			}
			// Checking MaxAreaAddressMismatches counters.
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().MaxAreaAddressMismatches().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero MaxAreaAddressMismatches counter, got %d, want %d", got, 0)
			}
			// Checking OwnLspPurges counters.
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().OwnLspPurges().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero OwnLspPurges counter, got %d, want %d", got, 0)
			}
			// Checking SeqNumSkips counters.
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().SeqNumSkips().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero SeqNumber skips, got %d, want %d", got, 0)
			}
			// Checking ManualAddressDropFromAreas counters.
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().ManualAddressDropFromAreas().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero ManualAddressDropFromAreas counter, got %d, want %d", got, 0)
			}
			// Checking PartChanges counters.
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().PartChanges().State()); got != 0 {
				t.Errorf("FAIL- Not expecting partition changes, got %d, want %d", got, 0)
			}
			// Checking SpfRuns counters.
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().SpfRuns().State()); got == 0 {
				t.Errorf("FAIL- Not expecting spf runs counter to be 0, got %d, want non zero", got)
			}
		})
		t.Run("Passive interface checks", func(t *testing.T) {
			// Pushing passive config to DUT.
			gnmi.Update(t, dut, statePath.Interface(intfName).Passive().Config(), true)
			gnmi.Update(t, dut, statePath.Interface(intfName).Level(2).Passive().Config(), false)

			// Checking passive telemetry.
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).Passive().State()); got != true {
				t.Errorf("FAIL- Expected value for passive not found on isis interface, got %t, want %t", got, true)
			}
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).Level(2).Passive().State()); got != true {
				t.Errorf("FAIL- Expected value for passive not found on isis interface level, got %t, want %t", got, true)
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
