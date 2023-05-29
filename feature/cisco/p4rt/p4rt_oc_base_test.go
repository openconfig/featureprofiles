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

package p4rt_oc_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/internal/attrs"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen = 24
)

var (
	npu0            = *ciscoFlags.P4RTOcNPU // "0/RP0/CPU0-NPU0 for fixed platforms, provide Linecard NPU information otherwise"
	npu0NodeID      = uint64(1)
	deviceID        = uint64(1)
	nonExistingPort = "FourHundredGigE0/5/0/100"
	observer        = fptest.NewObserver("P4RT OC").AddCsvRecorder("ocreport").AddCsvRecorder("P4RT OC")
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "100.120.1.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "100.120.1.2",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "100.121.1.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "100.121.1.2",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort3 = attrs.Attributes{
		Desc:    "dutPort3",
		IPv4:    "100.122.1.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		IPv4:    "100.122.1.2",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort4 = attrs.Attributes{
		Desc:    "dutPort4",
		IPv4:    "100.123.1.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort4 = attrs.Attributes{
		Name:    "atePort4",
		IPv4:    "100.123.1.2",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort5 = attrs.Attributes{
		Desc:    "dutPort5",
		IPv4:    "100.124.1.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort5 = attrs.Attributes{
		Name:    "atePort5",
		IPv4:    "100.124.1.2",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort6 = attrs.Attributes{
		Desc:    "dutPort6",
		IPv4:    "100.125.1.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort6 = attrs.Attributes{
		Name:    "atePort6",
		IPv4:    "100.125.1.2",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort7 = attrs.Attributes{
		Desc:    "dutPort7",
		IPv4:    "100.126.1.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort7 = attrs.Attributes{
		Name:    "atePort7",
		IPv4:    "100.126.1.2",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort8 = attrs.Attributes{
		Desc:    "dutPort8",
		IPv4:    "100.127.1.1",
		IPv4Len: ipv4PrefixLen,
	}
	atePort8 = attrs.Attributes{
		Name:    "atePort8",
		IPv4:    "100.127.1.2",
		IPv4Len: ipv4PrefixLen,
	}
)

// testArgs holds the objects needed by a test case.
type testArgs struct {
	dut *ondatra.DUTDevice
	ate *ondatra.ATEDevice
	top *ondatra.ATETopology
}

// Testcase defines testcase structure
type Testcase struct {
	name string
	desc string
	fn   func(t *testing.T, args *testArgs)
}

var (
	P4RTOCTestcases = []Testcase{
		{
			name: "Test Configuration Of Non Existing Port",
			desc: "Verify P4RT configuration with device-id and port-id with non-existing port is allowed",
			fn:   testNonExistingPortConfig,
		},
		{
			name: "Test Reconfiguration Of P4RT",
			desc: "Configure P4RT with existing device-id and existing port-id and verify no impact on existing sessions",
			fn:   testReconfigureP4RTWithPacketIOSessionOn,
		},
		{
			name: "Test Configuration Of DeviceID PortID With Interface Down",
			desc: "Configure P4RT with device-id and port-id with interface in down state",
			fn:   testConfigDeviceIDPortIDWithInterfaceDown,
		},
		{
			name: "Test Configuration Of DeviceID PortID Using Bundle Interfaces",
			desc: "Configure P4RT with device-id and port-id with Bundle Interface",
			fn:   testP4RTConfigurationWithBundleInterface,
		},
		{
			name: "Test Configuration Of DeviceID PortID Using GNMI Update",
			desc: "Configure device-id and interface-id via GNMI update",
			fn:   testP4RTConfigurationUsingGNMIUpdate,
		},
		{
			name: "Test Deletion Of Configuration Of DeviceID PortID Using GNMI Delete",
			desc: "Delete device-id and interface-id via GNMI delete",
			fn:   testP4RTConfigurationDelete,
		},
		{
			name: "Test Configuration Of DeviceID PortID Using GNMI Get On Config",
			desc: "Get device-id and interface-id config via GNMI GET config",
			fn:   testP4RTConfigurationUsingGetConfig,
		},
		{
			name: "Test DeviceID PortID State Using Telemetry",
			desc: "Get device-id and interface-id state via Telemetry",
			fn:   testP4RTTelemetry,
		},
		{
			name: "Test Max Port ID Range Using GNMI Get On Config",
			desc: "Configure Max Port Id and get Pord-id via GNMI Get",
			fn:   testP4RTUprev,
		},
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestP4RTOC(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	initializeNPU(t, dut)
	configureDUT(t, dut)

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	top.Push(t).StartProtocols(t)

	args := &testArgs{
		dut: dut,
		ate: ate,
		top: top,
	}

	for _, tt := range P4RTOCTestcases {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			t.Logf("Description: %s", tt.desc)

			tt.fn(t, args)
		})
	}
}

// configureATE configures port1 through port8 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	top := ate.Topology().New()

	p1 := ate.Port(t, "port1")
	i1 := top.AddInterface(atePort1.Name).WithPort(p1)
	i1.IPv4().
		WithAddress(atePort1.IPv4CIDR()).
		WithDefaultGateway(dutPort1.IPv4)

	p2 := ate.Port(t, "port2")
	i2 := top.AddInterface(atePort2.Name).WithPort(p2)
	i2.IPv4().
		WithAddress(atePort2.IPv4CIDR()).
		WithDefaultGateway(dutPort2.IPv4)

	p3 := ate.Port(t, "port3")
	i3 := top.AddInterface(atePort3.Name).WithPort(p3)
	i3.IPv4().
		WithAddress(atePort3.IPv4CIDR()).
		WithDefaultGateway(dutPort3.IPv4)

	p4 := ate.Port(t, "port4")
	i4 := top.AddInterface(atePort4.Name).WithPort(p4)
	i4.IPv4().
		WithAddress(atePort4.IPv4CIDR()).
		WithDefaultGateway(dutPort4.IPv4)

	p5 := ate.Port(t, "port5")
	i5 := top.AddInterface(atePort5.Name).WithPort(p5)
	i5.IPv4().
		WithAddress(atePort5.IPv4CIDR()).
		WithDefaultGateway(dutPort5.IPv4)

	p6 := ate.Port(t, "port6")
	i6 := top.AddInterface(atePort6.Name).WithPort(p6)
	i6.IPv4().
		WithAddress(atePort6.IPv4CIDR()).
		WithDefaultGateway(dutPort6.IPv4)

	p7 := ate.Port(t, "port7")
	i7 := top.AddInterface(atePort7.Name).WithPort(p7)
	i7.IPv4().
		WithAddress(atePort7.IPv4CIDR()).
		WithDefaultGateway(dutPort7.IPv4)

	p8 := ate.Port(t, "port8")
	i8 := top.AddInterface(atePort8.Name).WithPort(p8)
	i8.IPv4().
		WithAddress(atePort8.IPv4CIDR()).
		WithDefaultGateway(dutPort8.IPv4)

	return top
}

// getNPUs returns all the available NPUs on the router, ignoring fabric npus.
func getNPUs(t *testing.T, dut *ondatra.DUTDevice) []string {
	var npus []string
	l := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())
	pattern, _ := regexp.Compile(`.*-NPU\d+$`)
	for _, each := range l {
		if each.Type == oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_INTEGRATED_CIRCUIT {
			if name := each.GetName(); pattern.MatchString(name) && !strings.Contains(name, "FC") {
				npus = append(npus, name)
			}
		}
	}
	t.Log("Available NPUs on the system: ", npus)
	return npus
}

// initializeNPU inialises base npu used in the tests. If the npu specified via flags is not available, first available npu is used.
func initializeNPU(t *testing.T, dut *ondatra.DUTDevice) {
	if npus := getNPUs(t, dut); len(npus) != 0 && !contains(npus, npu0) {
		//initialize with first npu
		npu0 = npus[0]
	}
	t.Log("Using device :", npu0)
}

// contains finds if a string exists in an array of strings
func contains(s []string, e string) bool {
	for _, item := range s {
		if item == e {
			return true
		}
	}
	return false
}

// configureP4RTIntf enables port-id on a specified interface
func configureP4RTIntf(t *testing.T, dut *ondatra.DUTDevice, intf string, id uint32, intfType oc.E_IETFInterfaces_InterfaceType) {
	i := &oc.Interface{
		Type: intfType,
		Id:   ygot.Uint32(id),
		Name: ygot.String(intf),
	}
	gnmi.Replace(t, dut, gnmi.OC().Interface(intf).Config(), i)
}

// configureP4RTDevice configures device-id for a specified npu instance
func configureP4RTDevice(t *testing.T, dut *ondatra.DUTDevice, npu string, nodeid uint64) {
	ic := &oc.Component_IntegratedCircuit{}
	ic.NodeId = ygot.Uint64(nodeid)

	config := gnmi.OC().Component(npu).IntegratedCircuit()
	defer observer.RecordYgot(t, "REPLACE", config)
	gnmi.Replace(t, dut, config.Config(), ic)
}

// getSubInterface returns a subinterface configuration populated with IP addresses and VLAN ID.
func getSubInterface(dutPort *attrs.Attributes, index uint32, vlanID uint16) *oc.Interface_Subinterface {
	s := &oc.Interface_Subinterface{}
	//unshut sub/interface
	if *deviations.InterfaceEnabled {
		s.Enabled = ygot.Bool(true)
	}
	s.Index = ygot.Uint32(index)
	s4 := s.GetOrCreateIpv4()
	a := s4.GetOrCreateAddress(dutPort.IPv4)
	a.PrefixLength = ygot.Uint8(dutPort.IPv4Len)
	v := s.GetOrCreateVlan()
	m := v.GetOrCreateMatch()
	if index != 0 {
		m.GetOrCreateSingleTagged().VlanId = ygot.Uint16(vlanID)
	}
	return s
}

// configInterfaceDUT configures bundle interface with the Addrs.
func configInterfaceDUT(i *oc.Interface, dutPort *attrs.Attributes) *oc.Interface {
	i.Description = ygot.String(dutPort.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	i.AppendSubinterface(getSubInterface(dutPort, 0, 0))
	return i
}

// configureDUT configures DUT interfaces and enables p4rt feature
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {

	d := gnmi.OC()
	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String("Bundle-Ether120")}
	gnmi.Replace(t, dut, d.Interface(*i1.Name).Config(), configInterfaceDUT(i1, &dutPort1))
	BE120 := generateBundleMemberInterfaceConfig(t, p1.Name(), *i1.Name)
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), BE120)

	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String("Bundle-Ether121")}
	gnmi.Replace(t, dut, d.Interface(*i2.Name).Config(), configInterfaceDUT(i2, &dutPort2))
	BE121 := generateBundleMemberInterfaceConfig(t, p2.Name(), *i2.Name)
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), BE121)

	p3 := dut.Port(t, "port3")
	i3 := &oc.Interface{Name: ygot.String("Bundle-Ether122")}
	gnmi.Replace(t, dut, d.Interface(*i3.Name).Config(), configInterfaceDUT(i3, &dutPort3))
	BE122 := generateBundleMemberInterfaceConfig(t, p3.Name(), *i3.Name)
	gnmi.Replace(t, dut, d.Interface(p3.Name()).Config(), BE122)

	p4 := dut.Port(t, "port4")
	i4 := &oc.Interface{Name: ygot.String("Bundle-Ether123")}
	gnmi.Replace(t, dut, d.Interface(*i4.Name).Config(), configInterfaceDUT(i4, &dutPort4))
	BE123 := generateBundleMemberInterfaceConfig(t, p4.Name(), *i4.Name)
	gnmi.Replace(t, dut, d.Interface(p4.Name()).Config(), BE123)

	p5 := dut.Port(t, "port5")
	i5 := &oc.Interface{Name: ygot.String("Bundle-Ether124")}
	gnmi.Replace(t, dut, d.Interface(*i5.Name).Config(), configInterfaceDUT(i5, &dutPort5))
	BE124 := generateBundleMemberInterfaceConfig(t, p5.Name(), *i5.Name)
	gnmi.Replace(t, dut, d.Interface(p5.Name()).Config(), BE124)

	p6 := dut.Port(t, "port6")
	i6 := &oc.Interface{Name: ygot.String("Bundle-Ether125")}
	gnmi.Replace(t, dut, d.Interface(*i6.Name).Config(), configInterfaceDUT(i6, &dutPort6))
	BE125 := generateBundleMemberInterfaceConfig(t, p6.Name(), *i6.Name)
	gnmi.Replace(t, dut, d.Interface(p6.Name()).Config(), BE125)

	p7 := dut.Port(t, "port7")
	i7 := &oc.Interface{Name: ygot.String("Bundle-Ether126")}
	gnmi.Replace(t, dut, d.Interface(*i7.Name).Config(), configInterfaceDUT(i7, &dutPort7))
	BE126 := generateBundleMemberInterfaceConfig(t, p7.Name(), *i7.Name)
	gnmi.Replace(t, dut, d.Interface(p7.Name()).Config(), BE126)

	p8 := dut.Port(t, "port8")
	i8 := &oc.Interface{Name: ygot.String("Bundle-Ether127")}
	gnmi.Replace(t, dut, d.Interface(*i8.Name).Config(), configInterfaceDUT(i8, &dutPort8))
	BE127 := generateBundleMemberInterfaceConfig(t, p8.Name(), *i8.Name)
	gnmi.Replace(t, dut, d.Interface(p8.Name()).Config(), BE127)

	//P4RT config
	configureP4RTDevice(t, dut, npu0, npu0NodeID)
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Id().Config(), 1)
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Id().Config(), 2)
}

// generateBundleMemberInterfaceConfig returns interface configuration populated with bundle info
func generateBundleMemberInterfaceConfig(t *testing.T, name, bundleID string) *oc.Interface {
	i := &oc.Interface{Name: ygot.String(name)}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	e := i.GetOrCreateEthernet()
	e.AutoNegotiate = ygot.Bool(false)
	e.AggregateId = ygot.String(bundleID)
	return i
}
