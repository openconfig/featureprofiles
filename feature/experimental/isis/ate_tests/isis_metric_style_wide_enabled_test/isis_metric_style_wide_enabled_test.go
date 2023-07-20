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

package isis_metric_style_wide_enabled_test

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
	isisInstance   = "DEFAULT"
	dutAreaAddress = "49.0001"
	ateAreaAddress = "49.0002"
	dutSysID       = "1920.0000.2001"
	password       = "google"
	ateV4Route     = "203.0.113.0/30"
	ateV6Route     = "2001:db8::203:0:113:0/126"
	v4IP           = "203.0.113.1"
	v6IP           = "2001:db8::203:0:113:1"
	dutV4Metric    = 100
	dutV6Metric    = 100
	ateV4Metric    = 200
	ateV6Metric    = 200
	dutV4Route     = "192.0.2.0/30"
	dutV6Route     = "2001:db8::192:0:2:0/126"
)

var (
	dutPort1Attr = attrs.Attributes{
		Desc:    "DUT to ATE port1 ",
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
	dutPort2Attr = attrs.Attributes{
		Desc:    "DUT to ATE port2 ",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::192:0:2:5",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort2attr = attrs.Attributes{
		Name:    "ATE to DUT port2",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::192:0:2:6",
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

	t.Log("Pushing interface config on DUT port1")
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	i2 := dutPort2Attr.NewOCInterface(dut.Port(t, "port2").Name(), dut)
	t.Log("Pushing interface config on DUT port2")
	gnmi.Replace(t, dut, dc.Interface(i2.GetName()).Config(), i2)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
		fptest.SetPortSpeed(t, dut.Port(t, "port2"))
	}
}

// configureISIS configures isis on DUT.
func configureISIS(t *testing.T, dut *ondatra.DUTDevice, intfName string, dutAreaAddress, dutSysID string) {
	t.Helper()
	d := &oc.Root{}
	configPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)

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
	globalIsis.HelloPadding = oc.Isis_HelloPaddingType_ADAPTIVE

	// Level configs.
	level := isis.GetOrCreateLevel(2)
	level.Enabled = ygot.Bool(true)
	level.LevelNumber = ygot.Uint8(2)
	level.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC

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
	intf.Passive = ygot.Bool(false)

	// Interface timers.
	isisIntfTimers := intf.GetOrCreateTimers()
	isisIntfTimers.CsnpInterval = ygot.Uint16(5)
	isisIntfTimers.LspPacingInterval = ygot.Uint64(150)

	// Interface level configs.
	isisIntfLevel := intf.GetOrCreateLevel(2)
	isisIntfLevel.LevelNumber = ygot.Uint8(2)
	isisIntfLevel.Enabled = ygot.Bool(true)
	isisIntfLevel.Passive = ygot.Bool(false)
	isisIntfLevel.GetOrCreateHelloAuthentication().Enabled = ygot.Bool(true)
	isisIntfLevel.GetHelloAuthentication().AuthPassword = ygot.String(password)
	isisIntfLevel.GetHelloAuthentication().AuthType = oc.KeychainTypes_AUTH_TYPE_SIMPLE_KEY
	isisIntfLevel.GetHelloAuthentication().AuthMode = oc.IsisTypes_AUTH_MODE_MD5

	isisIntfLevelTimers := isisIntfLevel.GetOrCreateTimers()
	isisIntfLevelTimers.HelloInterval = ygot.Uint32(5)
	isisIntfLevelTimers.HelloMultiplier = ygot.Uint8(3)

	isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric = ygot.Uint32(dutV4Metric)
	isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric = ygot.Uint32(dutV6Metric)

	// Interface afi-safi configs.
	intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)

	t.Log("Pushing isis config on DUT")
	gnmi.Replace(t, dut, configPath.Config(), prot)
}

// configureATE configures the interfaces and isis on ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	t.Helper()
	topo := ate.Topology().New()
	port1 := ate.Port(t, "port1")
	port2 := ate.Port(t, "port2")

	i1Dut := topo.AddInterface(atePort1attr.Name).WithPort(port1)
	i1Dut.IPv4().WithAddress(atePort1attr.IPv4CIDR()).WithDefaultGateway(dutPort1Attr.IPv4)
	i1Dut.IPv6().WithAddress(atePort1attr.IPv6CIDR()).WithDefaultGateway(dutPort1Attr.IPv6)

	i2Dut := topo.AddInterface(atePort2attr.Name).WithPort(port2)
	i2Dut.IPv4().WithAddress(atePort2attr.IPv4CIDR()).WithDefaultGateway(dutPort2Attr.IPv4)
	i2Dut.IPv6().WithAddress(atePort2attr.IPv6CIDR()).WithDefaultGateway(dutPort2Attr.IPv6)

	isisDut := i1Dut.ISIS()
	isisDut.
		WithAreaID(ateAreaAddress).
		WithTERouterID(atePort1attr.IPv4).
		WithNetworkTypePointToPoint().
		WithWideMetricEnabled(true).
		WithLevelL2().WithAuthMD5(password).
		WithAreaAuthMD5(password).
		WithDomainAuthMD5(password)

	netGrp := i1Dut.AddNetwork(fmt.Sprintf("isis-%d", 1))
	netGrp.IPv4().WithAddress(ateV4Route)
	netGrp.ISIS().WithActive(true).WithIPReachabilityMetric(ateV4Metric)
	netGrp.IPv6().WithAddress(ateV6Route)
	netGrp.ISIS().WithActive(true).WithIPReachabilityMetric(ateV6Metric)

	t.Log("Pushing config to ATE and starting protocols...")
	topo.Push(t)
	topo.StartProtocols(t)
	return topo
}

// createFlow returns v4 and v6 flow from atePort2 to atePort1
func createFlow(t *testing.T, ate *ondatra.ATEDevice, ateTopo *ondatra.ATETopology) []*ondatra.Flow {
	t.Helper()
	srcIntf := ateTopo.Interfaces()[atePort2attr.Name]
	dstIntf := ateTopo.Interfaces()[atePort1attr.Name]

	t.Log("Configuring v4 traffic flow ")
	v4Header := ondatra.NewIPv4Header()
	v4Header.DstAddressRange().WithMin(v4IP).WithCount(1)

	v4Flow := ate.Traffic().NewFlow("v4Flow").
		WithSrcEndpoints(srcIntf).WithDstEndpoints(dstIntf).
		WithHeaders(ondatra.NewEthernetHeader(), v4Header)

	t.Log("Configuring v6 traffic flow ")
	v6Header := ondatra.NewIPv6Header()
	v6Header.DstAddressRange().WithMin(v6IP).WithCount(1)

	v6Flow := ate.Traffic().NewFlow("v6Flow").
		WithSrcEndpoints(srcIntf).WithDstEndpoints(dstIntf).
		WithHeaders(ondatra.NewEthernetHeader(), v6Header)

	return []*ondatra.Flow{v4Flow, v6Flow}
}

// TestISISWideMetricEnabled verifies route metric with wide metric enabled on DUT.
func TestISISWideMetricEnabled(t *testing.T) {
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
	ateTopo := configureATE(t, ate)
	flows := createFlow(t, ate, ateTopo)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		intfName = intfName + ".0"
	}
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Isis()

	t.Run("ISIS telemetry", func(t *testing.T) {
		t.Run("Verifying adjacency", func(t *testing.T) {
			adjacencyPath := statePath.Interface(intfName).Level(2).AdjacencyAny().AdjacencyState().State()

			_, ok := gnmi.WatchAll(t, dut, adjacencyPath, time.Minute, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
				state, present := val.Val()
				return present && state == oc.Isis_IsisInterfaceAdjState_UP
			}).Await(t)
			if !ok {
				t.Fatalf("No isis adjacency reported on interface %v", intfName)
			}
		})
		// Getting neighbors sysid.
		sysid := gnmi.GetAll(t, dut, statePath.Interface(intfName).Level(2).AdjacencyAny().SystemId().State())
		ateSysID := sysid[0]
		ateLspID := ateSysID + ".00-00"
		dutLspID := dutSysID + ".00-00"

		t.Run("Afi-Safi checks", func(t *testing.T) {
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).AfiName().State()); got != oc.IsisTypes_AFI_TYPE_IPV4 {
				t.Errorf("FAIL- Expected afi name not found, got %d, want %d", got, oc.IsisTypes_AFI_TYPE_IPV4)
			}
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).SafiName().State()); got != oc.IsisTypes_SAFI_TYPE_UNICAST {
				t.Errorf("FAIL- Expected safi name not found, got %d, want %d", got, oc.IsisTypes_SAFI_TYPE_UNICAST)
			}
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).AfiName().State()); got != oc.IsisTypes_AFI_TYPE_IPV6 {
				t.Errorf("FAIL- Expected afi name not found, got %d, want %d", got, oc.IsisTypes_AFI_TYPE_IPV6)
			}
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).SafiName().State()); got != oc.IsisTypes_SAFI_TYPE_UNICAST {
				t.Errorf("FAIL- Expected safi name not found, got %d, want %d", got, oc.IsisTypes_SAFI_TYPE_UNICAST)
			}
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric().State()); got != dutV4Metric {
				t.Errorf("FAIL- Expected v4 unicast metric value not found, got %d, want %d", got, dutV4Metric)
			}
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric().State()); got != dutV6Metric {
				t.Errorf("FAIL- Expected v6 unicast metric value not found, got %d, want %d", got, dutV6Metric)
			}
		})
		t.Run("Adjacency state checks", func(t *testing.T) {
			adjPath := statePath.Interface(intfName).Level(2).Adjacency(ateSysID)

			if got := gnmi.Get(t, dut, adjPath.SystemId().State()); got != ateSysID {
				t.Errorf("FAIL- Expected neighbor system id not found, got %s, want %s", got, ateSysID)
			}
			want := []string{ateAreaAddress, dutAreaAddress}
			if got := gnmi.Get(t, dut, adjPath.AreaAddress().State()); !cmp.Equal(got, want, cmpopts.SortSlices(func(a, b string) bool { return a < b })) {
				t.Errorf("FAIL- Expected area address not found, got %s, want %s", got, want)
			}
			if got := gnmi.Get(t, dut, adjPath.DisSystemId().State()); got != "0000.0000.0000" {
				t.Errorf("FAIL- Expected dis system id not found, got %s, want %s", got, "0000.0000.0000")
			}
			if got := gnmi.Get(t, dut, adjPath.LocalExtendedCircuitId().State()); got == 0 {
				t.Errorf("FAIL- Expected local extended circuit id not found,expected non-zero value, got %d", got)
			}
			if got := gnmi.Get(t, dut, adjPath.MultiTopology().State()); got != false {
				t.Errorf("FAIL- Expected value for multi topology not found, got %t, want %t", got, false)
			}
			if got := gnmi.Get(t, dut, adjPath.NeighborCircuitType().State()); got != oc.Isis_LevelType_LEVEL_2 {
				t.Errorf("FAIL- Expected value for circuit type not found, got %s, want %s", got, oc.Isis_LevelType_LEVEL_2)
			}
			if got := gnmi.Get(t, dut, adjPath.NeighborIpv4Address().State()); got != atePort1attr.IPv4 {
				t.Errorf("FAIL- Expected value for ipv4 address not found, got %s, want %s", got, atePort1attr.IPv4)
			}
			if got := gnmi.Get(t, dut, adjPath.NeighborExtendedCircuitId().State()); got == 0 {
				t.Errorf("FAIL- Expected neighbor extended circuit id not found,expected non-zero value, got %d", got)
			}
			snpaAddress := gnmi.Get(t, dut, adjPath.NeighborSnpa().State())
			mac, err := net.ParseMAC(snpaAddress)
			if !(mac != nil && err == nil) {
				t.Errorf("FAIL- Expected value for snpa address not found, got %s", snpaAddress)
			}
			if got := gnmi.Get(t, dut, adjPath.Nlpid().State()); !cmp.Equal(got, []oc.E_Adjacency_Nlpid{oc.Adjacency_Nlpid_IPV4, oc.Adjacency_Nlpid_IPV6}) {
				t.Errorf("FAIL- Expected address families not found, got %s, want %s", got, []oc.E_Adjacency_Nlpid{oc.Adjacency_Nlpid_IPV4, oc.Adjacency_Nlpid_IPV6})
			}
			ipv6Address := gnmi.Get(t, dut, adjPath.NeighborIpv6Address().State())
			ip := net.ParseIP(ipv6Address)
			if !(ip != nil && ip.To16() != nil) {
				t.Errorf("FAIL- Expected ipv6 address not found, got %s", ipv6Address)
			}
			if _, ok := gnmi.Lookup(t, dut, adjPath.Priority().State()).Val(); !ok {
				t.Errorf("FAIL- Priority is not present")
			}
			if _, ok := gnmi.Lookup(t, dut, adjPath.RestartStatus().State()).Val(); !ok {
				t.Errorf("FAIL- Restart status not present")
			}
			if _, ok := gnmi.Lookup(t, dut, adjPath.RestartSupport().State()).Val(); !ok {
				t.Errorf("FAIL- Restart support not present")
			}
			if _, ok := gnmi.Lookup(t, dut, adjPath.RestartStatus().State()).Val(); !ok {
				t.Errorf("FAIL- Restart suppress not present")
			}
		})
		t.Run("System level counter checks", func(t *testing.T) {
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().AuthFails().State()); got != 0 {
				t.Errorf("FAIL- Not expecting any authentication key failure, got %d, want %d", got, 0)
			}
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().AuthTypeFails().State()); got != 0 {
				t.Errorf("FAIL- Not expecting any authentication type mismatches, got %d, want %d", got, 0)
			}
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().CorruptedLsps().State()); got != 0 {
				t.Errorf("FAIL- Not expecting any corrupted lsps, got %d, want %d", got, 0)
			}
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().DatabaseOverloads().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero database_overloads, got %d, want %d", got, 0)
			}
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().ExceedMaxSeqNums().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero max_seqnum counter, got %d, want %d", got, 0)
			}
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().IdLenMismatch().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero IdLen_Mismatch counter, got %d, want %d", got, 0)
			}
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().LspErrors().State()); got != 0 {
				t.Errorf("FAIL- Not expecting any lsp errors, got %d, want %d", got, 0)
			}
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().MaxAreaAddressMismatches().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero MaxAreaAddressMismatches counter, got %d, want %d", got, 0)
			}
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().OwnLspPurges().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero OwnLspPurges counter, got %d, want %d", got, 0)
			}
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().SeqNumSkips().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero SeqNumber skips, got %d, want %d", got, 0)
			}
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().ManualAddressDropFromAreas().State()); got != 0 {
				t.Errorf("FAIL- Not expecting non zero ManualAddressDropFromAreas counter, got %d, want %d", got, 0)
			}
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().PartChanges().State()); got != 0 {
				t.Errorf("FAIL- Not expecting partition changes, got %d, want %d", got, 0)
			}
			if got := gnmi.Get(t, dut, statePath.Level(2).SystemLevelCounters().SpfRuns().State()); got == 0 {
				t.Errorf("FAIL- Not expecting spf runs counter to be 0, got %d, want non zero", got)
			}
		})
		t.Run("Wide metric checks", func(t *testing.T) {
			if got := gnmi.Get(t, dut, statePath.Level(2).MetricStyle().State()); got != oc.E_Isis_MetricStyle(2) {
				t.Errorf("FAIL- Expected metric style not found, got %s, want %s", got, oc.E_Isis_MetricStyle(2))
			}
			if got := gnmi.Get(t, dut, statePath.Level(2).Lsp(ateLspID).Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_EXTENDED_IPV4_REACHABILITY).ExtendedIpv4Reachability().Prefix(ateV4Route).Prefix().State()); got != ateV4Route {
				t.Errorf("FAIL- Expected ate v4 route not found, got %v, want %v", got, ateV4Route)
			}
			if got := gnmi.Get(t, dut, statePath.Level(2).Lsp(ateLspID).Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_EXTENDED_IPV4_REACHABILITY).ExtendedIpv4Reachability().Prefix(ateV4Route).Metric().State()); got != ateV4Metric {
				t.Errorf("FAIL- Expected metric for ate v4 route not found, got %v, want %v", got, ateV4Metric)
			}
			if got := gnmi.Get(t, dut, statePath.Level(2).Lsp(ateLspID).Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_IPV6_REACHABILITY).Ipv6Reachability().Prefix(ateV6Route).Prefix().State()); got != ateV6Route {
				t.Errorf("FAIL- Expected ate v6 route not found, got %v, want %v", got, ateV6Route)
			}
			if got := gnmi.Get(t, dut, statePath.Level(2).Lsp(ateLspID).Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_IPV6_REACHABILITY).Ipv6Reachability().Prefix(ateV6Route).Metric().State()); got != ateV6Metric {
				t.Errorf("FAIL- Expected metric for ate v6 route not found, got %v, want %v", got, ateV6Metric)
			}
			if got := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().Ipv4Entry(ateV4Route).State()).GetPrefix(); got != ateV4Route {
				t.Errorf("FAIL- Expected ate v4 route not found in aft, got %v, want %v", got, ateV4Route)
			}
			if got := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().Ipv6Entry(ateV6Route).State()).GetPrefix(); got != ateV6Route {
				t.Errorf("FAIL- Expected ate v6 route not found in aft, got %v, want %v", got, ateV6Route)
			}
			if got := gnmi.Get(t, dut, statePath.Level(2).Lsp(dutLspID).Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_EXTENDED_IPV4_REACHABILITY).ExtendedIpv4Reachability().Prefix(dutV4Route).Prefix().State()); got != dutV4Route {
				t.Errorf("FAIL- Expected dut v4 route not found, got %v, want %v", got, dutV4Route)
			}
			if got := gnmi.Get(t, dut, statePath.Level(2).Lsp(dutLspID).Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_EXTENDED_IPV4_REACHABILITY).ExtendedIpv4Reachability().Prefix(dutV4Route).Metric().State()); got != dutV4Metric {
				t.Errorf("FAIL- Expected metric for dut v4 route not found, got %v, want %v", got, dutV4Metric)
			}
			if got := gnmi.Get(t, dut, statePath.Level(2).Lsp(dutLspID).Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_IPV6_REACHABILITY).Ipv6Reachability().Prefix(dutV6Route).Prefix().State()); got != dutV6Route {
				t.Errorf("FAIL- Expected dut v6 route not found, got %v, want %v", got, dutV6Route)
			}
			if got := gnmi.Get(t, dut, statePath.Level(2).Lsp(dutLspID).Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_IPV6_REACHABILITY).Ipv6Reachability().Prefix(dutV6Route).Metric().State()); got != dutV6Metric {
				t.Errorf("FAIL- Expected metric for dut v6 route not found, got %v, want %v", got, dutV6Metric)
			}
		})
		t.Run("Traffic checks", func(t *testing.T) {
			ate.Traffic().Start(t, flows...)
			time.Sleep(time.Second * 15)
			ate.Traffic().Stop(t)

			for _, flow := range flows {
				t.Log("Checking flow telemetry...")
				telem := gnmi.OC()
				loss := gnmi.Get(t, ate, telem.Flow(flow.Name()).LossPct().State())
				if loss > 1 {
					t.Errorf("FAIL- Got %v%% packet loss for %s ; expected < 1%%", loss, flow.Name())
				}
			}
		})
	})
}
