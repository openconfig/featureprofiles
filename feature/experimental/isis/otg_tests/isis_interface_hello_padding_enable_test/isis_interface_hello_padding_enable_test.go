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

package isis_interface_hello_padding_enable_test

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
	"github.com/openconfig/featureprofiles/internal/otgutils"
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
	isisInstance   = "DEFAULT"
	dutAreaAddress = "49.0001"
	ateAreaAddress = "49.0002"
	dutSysID       = "1920.0000.2001"
	ateSystemID    = "640000000001"
	password       = "google"
	v4Route1       = "203.0.113.0"
	v6Route1       = "2001:db8::203:0:113:0"
	v4Route        = "203.0.113.0/30"
	v6Route        = "2001:db8::203:0:113:0/126"
	v4IP           = "203.0.113.1"
	v6IP           = "2001:db8::203:0:113:1"
	v4Metric       = 100
	v6Metric       = 100
	v4NetName      = "isisv4Net"
	v6NetName      = "isisv6Net"
	v4FlowName     = "v4Flow"
	v6FlowName     = "v6Flow"
)

var (
	dutPort1Attr = attrs.Attributes{
		Desc:    "DUT to ATE port1 ",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
		MTU:     1500,
	}
	atePort1attr = attrs.Attributes{
		Name:    "ATE to DUT port1 ",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		MAC:     "02:00:01:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
		MTU:     1500,
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
		MAC:     "02:00:02:01:01:01",
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
func configureISIS(t *testing.T, dut *ondatra.DUTDevice, intfName string) {
	t.Helper()
	d := &oc.Root{}
	configPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	prot.Enabled = ygot.Bool(true)
	isis := prot.GetOrCreateIsis()
	globalISIS := isis.GetOrCreateGlobal()

	// Global configs.
	globalISIS.Net = []string{fmt.Sprintf("%v.%v.00", dutAreaAddress, dutSysID)}
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalISIS.LevelCapability = oc.Isis_LevelType_LEVEL_2
	globalISIS.AuthenticationCheck = ygot.Bool(true)
	globalISIS.HelloPadding = oc.Isis_HelloPaddingType_ADAPTIVE

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
	intf.HelloPadding = oc.Isis_HelloPaddingType_ADAPTIVE

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

// configureOTG configures the interfaces and isis on OTG.
func configureOTG(t *testing.T, otg *otg.OTG) gosnappi.Config {
	t.Helper()
	config := otg.NewConfig(t)
	port1 := config.Ports().Add().SetName("port1")
	port2 := config.Ports().Add().SetName("port2")

	iDut1Dev := config.Devices().Add().SetName(atePort1attr.Name)
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(atePort1attr.Name + ".Eth").SetMac(atePort1attr.MAC).SetMtu(uint32(atePort1attr.MTU))
	iDut1Eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName(atePort1attr.Name + ".IPv4")
	iDut1Ipv4.SetAddress(atePort1attr.IPv4).SetGateway(dutPort1Attr.IPv4).SetPrefix(uint32(atePort1attr.IPv4Len))
	iDut1Ipv6 := iDut1Eth.Ipv6Addresses().Add().SetName(atePort1attr.Name + ".IPv6")
	iDut1Ipv6.SetAddress(atePort1attr.IPv6).SetGateway(dutPort1Attr.IPv6).SetPrefix(uint32(atePort1attr.IPv6Len))

	iDut2Dev := config.Devices().Add().SetName(atePort2attr.Name)
	iDut2Eth := iDut2Dev.Ethernets().Add().SetName(atePort2attr.Name + ".Eth").SetMac(atePort2attr.MAC)
	iDut2Eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(port2.Name())
	iDut2Ipv4 := iDut2Eth.Ipv4Addresses().Add().SetName(atePort2attr.Name + ".IPv4")
	iDut2Ipv4.SetAddress(atePort2attr.IPv4).SetGateway(dutPort2Attr.IPv4).SetPrefix(uint32(atePort2attr.IPv4Len))
	iDut2Ipv6 := iDut2Eth.Ipv6Addresses().Add().SetName(atePort2attr.Name + ".IPv6")
	iDut2Ipv6.SetAddress(atePort2attr.IPv6).SetGateway(dutPort2Attr.IPv6).SetPrefix(uint32(atePort2attr.IPv6Len))

	iDut1Dev.Isis().SetSystemId(ateSystemID).SetName("devIsis")
	iDut1Dev.Isis().RouterAuth().AreaAuth().SetAuthType("md5").SetMd5(password)
	iDut1Dev.Isis().RouterAuth().DomainAuth().SetAuthType("md5").SetMd5(password)
	iDut1Dev.Isis().Basic().SetIpv4TeRouterId(atePort1attr.IPv4).SetEnableWideMetric(false)
	iDut1Dev.Isis().Advanced().SetAreaAddresses([]string{strings.Replace(ateAreaAddress, ".", "", -1)})

	iDut1Dev.Isis().Interfaces().
		Add().
		SetEthName(iDut1Dev.Ethernets().Items()[0].Name()).
		SetName("devIsisInt").
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetMetric(10).Authentication().SetAuthType("md5").SetMd5(password)

	// netv4 is a simulated network containing the ipv4 addresses specified by targetNetwork
	netv4 := iDut1Dev.Isis().V4Routes().Add().SetName(v4NetName).SetLinkMetric(10)
	netv4.Addresses().Add().SetAddress(v4Route1).SetPrefix(plenIPv4)

	// netv6 is a simulated network containing the ipv6 addresses specified by targetNetwork
	netv6 := iDut1Dev.Isis().V6Routes().Add().SetName(v6NetName).SetLinkMetric(10)
	netv6.Addresses().Add().SetAddress(v6Route1).SetPrefix(plenIPv6)

	t.Log("Configuring v4 traffic flow ")
	v4Flow := config.Flows().Add().SetName(v4FlowName)
	v4Flow.Metrics().SetEnable(true)
	v4Flow.TxRx().Device().
		SetTxNames([]string{iDut2Ipv4.Name()}).
		SetRxNames([]string{v4NetName})
	v4Flow.Size().SetFixed(512)
	v4Flow.Rate().SetPps(100)
	v4Flow.Duration().SetChoice("continuous")
	e1 := v4Flow.Packet().Add().Ethernet()
	e1.Src().SetValue(atePort2attr.MAC)
	v4 := v4Flow.Packet().Add().Ipv4()
	v4.Src().SetValue(atePort2attr.IPv4)
	v4.Dst().Increment().SetStart(v4IP).SetCount(1)

	t.Log("Configuring v6 traffic flow ")
	v6Flow := config.Flows().Add().SetName(v6FlowName)
	v6Flow.Metrics().SetEnable(true)
	v6Flow.TxRx().Device().
		SetTxNames([]string{iDut2Ipv6.Name()}).
		SetRxNames([]string{v6NetName})
	v6Flow.Size().SetFixed(512)
	v6Flow.Rate().SetPps(100)
	v6Flow.Duration().SetChoice("continuous")
	e2 := v6Flow.Packet().Add().Ethernet()
	e2.Src().SetValue(atePort2attr.MAC)
	v6 := v6Flow.Packet().Add().Ipv6()
	v6.Src().SetValue(atePort2attr.IPv6)
	v6.Dst().Increment().SetStart(v6IP).SetCount(1)

	t.Logf("Pushing config to OTG and starting protocols...")
	otg.PushConfig(t, config)
	time.Sleep(30 * time.Second)
	otg.StartProtocols(t)

	otgutils.WaitForARP(t, otg, config, "IPv4")
	otgutils.WaitForARP(t, otg, config, "IPv6")

	return config
}

// TestIsisInterfaceHelloPaddingEnable verifies adjacency with hello padding enabled.
func TestIsisInterfaceHelloPaddingEnable(t *testing.T) {
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
	configureISIS(t, dut, intfName)

	// Configure interface,isis and traffic on ATE.
	otg := ate.OTG()
	otgConfig := configureOTG(t, otg)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		intfName = intfName + ".0"
	}
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Isis()

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
		ateLspID := ateSysID + ".00-00"

		t.Run("HelloPadding checks", func(t *testing.T) {
			if got := gnmi.Get(t, dut, statePath.Global().HelloPadding().State()); got != oc.Isis_HelloPaddingType_ADAPTIVE {
				t.Errorf("FAIL- Expected global hello padding state not found, got %d, want %d", got, oc.Isis_HelloPaddingType_ADAPTIVE)
			}
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).HelloPadding().State()); got != oc.Isis_HelloPaddingType_ADAPTIVE {
				t.Errorf("FAIL- Expected interface hello padding state not found, got %d, want %d", got, oc.Isis_HelloPaddingType_ADAPTIVE)
			}
			// Changing MTU at ATE side
			for _, d := range otgConfig.Devices().Items() {
				Eth := d.Ethernets().Items()[0]
				if Eth.Name() == atePort1attr.Name+".Eth" {
					Eth.SetMtu(uint32(2000))
				}
			}
			otg.PushConfig(t, otgConfig)
			otg.StartProtocols(t)

			// Adjacency check
			_, found := gnmi.Watch(t, dut, statePath.Interface(intfName).Level(2).Adjacency(ateSysID).AdjacencyState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
				state, present := val.Val()
				return present && state == oc.Isis_IsisInterfaceAdjState_DOWN
			}).Await(t)
			if !found {
				t.Errorf("Isis adjacency is not down on interface %v when MTU is changed", intfName)
			}

			// Reverting MTU at ATE side
			for _, d := range otgConfig.Devices().Items() {
				Eth := d.Ethernets().Items()[0]
				if Eth.Name() == atePort1attr.Name+".Eth" {
					Eth.SetMtu(uint32(atePort1attr.MTU))
				}
			}
			otg.PushConfig(t, otgConfig)
			otg.StartProtocols(t)

			// Adjacency check
			_, ok := gnmi.Watch(t, dut, statePath.Interface(intfName).Level(2).Adjacency(ateSysID).AdjacencyState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
				state, present := val.Val()
				return present && state == oc.Isis_IsisInterfaceAdjState_UP
			}).Await(t)
			if !ok {
				t.Fatalf("Interface %v has no level 2 isis adjacency", intfName)
			}
		})
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
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric().State()); got != v4Metric {
				t.Errorf("FAIL- Expected v4 unicast metric value not found, got %d, want %d", got, v4Metric)
			}
			if got := gnmi.Get(t, dut, statePath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Metric().State()); got != v6Metric {
				t.Errorf("FAIL- Expected v6 unicast metric value not found, got %d, want %d", got, v6Metric)
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
		t.Run("Route checks", func(t *testing.T) {
			if got := gnmi.Get(t, dut, statePath.Level(2).Lsp(ateLspID).Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_EXTENDED_IPV4_REACHABILITY).ExtendedIpv4Reachability().Prefix(v4Route).Prefix().State()); got != v4Route {
				t.Errorf("FAIL- Expected v4 route not found in isis, got %v, want %v", got, v4Route)
			}
			if got := gnmi.Get(t, dut, statePath.Level(2).Lsp(ateLspID).Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_IPV6_REACHABILITY).Ipv6Reachability().Prefix(v6Route).Prefix().State()); got != v6Route {
				t.Errorf("FAIL- Expected v6 route not found in isis, got %v, want %v", got, v6Route)
			}
			if got := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().Ipv4Entry(v4Route).State()).GetPrefix(); got != v4Route {
				t.Errorf("FAIL- Expected v4 route not found in aft, got %v, want %v", got, v4Route)
			}
			if got := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().Ipv6Entry(v6Route).State()).GetPrefix(); got != v6Route {
				t.Errorf("FAIL- Expected v6 route not found in aft, got %v, want %v", got, v6Route)
			}
		})
		t.Run("Traffic checks", func(t *testing.T) {
			t.Logf("Starting traffic")
			otg.StartTraffic(t)
			time.Sleep(time.Second * 15)
			t.Logf("Stop traffic")
			otg.StopTraffic(t)

			otgutils.LogFlowMetrics(t, otg, otgConfig)
			otgutils.LogPortMetrics(t, otg, otgConfig)

			for _, flow := range []string{v4FlowName, v6FlowName} {
				t.Log("Checking flow telemetry...")
				recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(flow).State())
				txPackets := recvMetric.GetCounters().GetOutPkts()
				rxPackets := recvMetric.GetCounters().GetInPkts()
				lostPackets := txPackets - rxPackets
				lossPct := lostPackets * 100 / txPackets

				if lossPct > 1 {
					t.Errorf("FAIL- Got %v%% packet loss for %s ; expected < 1%%", lossPct, flow)
				}
			}
		})
	})
}
