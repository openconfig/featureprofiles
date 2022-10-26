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

// Package session is deprecated and scoped only to be used with
// feature/experimental/isis/ate_tests/*.  Do not use elsewhere.
package session

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry/networkinstance"
	"github.com/openconfig/ygot/ygot"

	telemetry "github.com/openconfig/ondatra/telemetry"
)

// PTISIS is shorthand for the long oc protocol type constant
const PTISIS = telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS

// The testbed consists of a dut and an ate with two connections, labeled ISISIntf and Intf2.
// ISISIntf links dut:port1 and ate:port1, which are assigned 192.0.2.1/30 and 192.0.2.2/30
// respectively. Intf2 connects dut:port2 to ate:port2, which are 192.0.2.5/30 and 192.0.2.6/30.
// We establish an IS-IS adjacency over ISISIntf. For traffic testing, we configure the ATE end
// of the IS-IS adjacency to advertise 198.51.100.0/24, then generate traffic through ate:port2 with
// IPv4 headers indicating that it should go to a random address in that range; the dut should
// route this traffic to the IS-IS link, where the ATE should log it arriving on ate:port1.
const (
	DUTAreaAddress = "49.0001"
	ATEAreaAddress = "49.0002"
	DUTSysID       = "1920.0000.2001"
	ATESysID       = "640000000001"
	ISISName       = "DEFAULT"
	pLen4          = 30
	pLen6          = 126
)

var (
	// DUTISISAttrs has attributes for the DUT ISIS connection on port1
	DUTISISAttrs = &attrs.Attributes{
		Desc:    "DUT to ATE with IS-IS",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: pLen4,
		IPv6Len: pLen6,
	}
	// ATEISISAttrs has attributes for the ATE ISIS connection on port1
	ATEISISAttrs = &attrs.Attributes{
		Name:    "port1",
		MAC:     "02:11:01:00:00:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		Desc:    "ATE to DUT with IS-IS",
		IPv4Len: pLen4,
		IPv6Len: pLen6,
	}
	// DUTTrafficAttrs has attributes for the DUT end of the traffic connection (port2)
	DUTTrafficAttrs = &attrs.Attributes{
		Desc:    "DUT to ATE secondary link",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: pLen4,
		IPv6Len: pLen6,
	}
	// ATETrafficAttrs has attributes for the ATE end of the traffic connection (port2)
	ATETrafficAttrs = &attrs.Attributes{
		Name:    "port2",
		MAC:     "02:12:01:00:00:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		Desc:    "ATE to DUT secondary link",
		IPv4Len: pLen4,
		IPv6Len: pLen6,
	}
)

// addISISOC configures basic IS-IS on a device.
func addISISOC(dev *telemetry.Device, areaAddress, sysID, ifaceName string) {
	inst := dev.GetOrCreateNetworkInstance(*deviations.DefaultNetworkInstance)
	prot := inst.GetOrCreateProtocol(PTISIS, ISISName)
	isis := prot.GetOrCreateIsis()
	glob := isis.GetOrCreateGlobal()
	glob.Instance = ygot.String(ISISName)
	glob.Net = []string{fmt.Sprintf("%v.%v.00", areaAddress, sysID)}
	glob.GetOrCreateAf(telemetry.IsisTypes_AFI_TYPE_IPV4, telemetry.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	glob.GetOrCreateAf(telemetry.IsisTypes_AFI_TYPE_IPV6, telemetry.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	intf := isis.GetOrCreateInterface(ifaceName)
	intf.CircuitType = telemetry.IsisTypes_CircuitType_POINT_TO_POINT
	intf.Enabled = ygot.Bool(true)
	intf.GetOrCreateLevel(2).Enabled = ygot.Bool(true)
}

// addISISTopo configures basic IS-IS on an ATETopology interface.
func addISISTopo(dev gosnappi.Device, areaAddress, sysID string) {

	devIsis := dev.Isis().
		SetSystemId(sysID).
		SetName("devIsis")

	devIsis.Basic().
		SetHostname(devIsis.Name()).SetLearnedLspFilter(true)

	devIsis.Advanced().
		SetAreaAddresses([]string{strings.Replace(areaAddress, ".", "", -1)})

	devIsisInt := devIsis.Interfaces().
		Add().
		SetEthName(dev.Ethernets().Items()[0].Name()).
		SetName("devIsisInt").
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2)

	devIsisInt.Advanced().
		SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)

}

// TestSession is a convenience wrapper around the dut, ate, ports, and topology we're using.
type TestSession struct {
	DUT *ondatra.DUTDevice
	ATE *ondatra.ATEDevice
	// DUTConf, ATEConf, and ATETop can be modified by tests; calling .Push() will apply them to the
	// dut and ate.
	DUTConf, ATEConf *telemetry.Device
	ATETop           gosnappi.Config
}

// DUTISISTelemetry gets the telemetry PathStruct for /network-instance[default]/protocol[ISIS]/isis on the DUT
func (s *TestSession) DUTISISTelemetry(t testing.TB) *networkinstance.NetworkInstance_Protocol_IsisPath {
	return s.DUT.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(PTISIS, ISISName).Isis()
}

// ATEISISTelemetry gets the telemetry PathStruct for /network-instance[default]/protocol[ISIS]/isis on the ATE
func (s *TestSession) ATEISISTelemetry(t testing.TB) *networkinstance.NetworkInstance_Protocol_IsisPath {
	return s.ATE.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(PTISIS, ISISName).Isis()
}

// Interfaces returns a map of port names to devices
func (s *TestSession) ATEDevices() map[string]gosnappi.Device {
	devs := make(map[string]gosnappi.Device)
	for _, dev := range s.ATETop.Devices().Items() {
		devs[dev.Ethernets().Items()[0].PortName()] = dev
	}
	return devs
}

// ATEDevice returns an gosnappi.Device for the port with the given name, or nil if our ATE is
// actually an ondatra.DUTDevice.
func (s *TestSession) ATEDevice(t testing.TB, portID string) gosnappi.Device {
	dev, ok := s.ATEDevices()[portID]
	if !ok {
		t.Logf("Available ATE Devices:")
		for name := range s.ATEDevices() {
			t.Logf("  %v", name)
		}
		t.Fatalf("No ATE Device with ID %v", portID)
	}
	return dev
}

// New creates a new TestSession using the default global config, and configures
// the interfaces on the dut and the ate.
func New(t testing.TB) *TestSession {
	t.Helper()
	s := &TestSession{}
	s.DUT = ondatra.DUT(t, "dut")
	s.DUTConf = &telemetry.Device{}
	s.confDUTInterface(t, "port1", DUTISISAttrs)
	s.confDUTInterface(t, "port2", DUTTrafficAttrs)

	s.ATE = ondatra.ATE(t, "ate")
	otg := s.ATE.OTG()
	s.ATETop = otg.NewConfig(t)
	s.confATEInterface(t, "port1", ATEISISAttrs, DUTISISAttrs)
	s.confATEInterface(t, "port2", ATETrafficAttrs, DUTTrafficAttrs)
	return s
}

// NewWithISIS creates a new TestSession using the default global config, configuring the interfaces
// and a basic IS-IS adjacency between the two. This does NOT push any config to the devices.
func NewWithISIS(t testing.TB) *TestSession {
	t.Helper()
	s := New(t)
	s.WithISIS(t)
	return s
}

// confDUTInterface sets the given attributes on the specified interface on the dut. portID should
// be an ID from the testbed, e.g. "port1" or "port2".
func (s *TestSession) confDUTInterface(t testing.TB, portID string, attrs *attrs.Attributes) {
	t.Helper()
	intfName := s.DUT.Port(t, portID).Name()
	attrs.ConfigInterface(s.DUTConf.GetOrCreateInterface(intfName))
}

// confATEInterface configures the expected interface on the ate.
// If the ate is a DUTDevice, this calls Replace(); if it's an ATEDevice, it just updates s.top.
func (s *TestSession) confATEInterface(t testing.TB, portID string, attrs, peerAttrs *attrs.Attributes) {
	t.Helper()
	attrs.AddToOTG(s.ATETop, s.ATE.Port(t, portID), peerAttrs)
}

// WithISIS adds ISIS to a test session.
func (s *TestSession) WithISIS(t testing.TB) *TestSession {
	t.Helper()
	addISISOC(s.DUTConf, DUTAreaAddress, DUTSysID, s.DUT.Port(t, "port1").Name())
	if s.ATEDevice(t, "port1") == nil {
		t.Fatalf("Nil interface:\n***\nATE: %v\n***\n***\nDevices:\n%v\n***\nPortName: %v\n***\n", s.ATE, s.ATEDevices(), s.ATE.Port(t, "port1").Name())
	}

	addISISTopo(s.ATEDevice(t, "port1"), ATEAreaAddress, ATESysID)

	return s
}

func (s *TestSession) EnableHelloPadding(t testing.TB, flag bool) {
	for _, d := range s.ATETop.Devices().Items() {
		if d.Name() == s.ATEDevice(t, "port1").Name() {
			d.Isis().Advanced().SetEnableHelloPadding(flag)
		}
	}
}

// ConfigISIS takes two functions, one that operates on an OC IS-IS block and one that operates on
// an ondatra ATE IS-IS block. The first will be applied to the IS-IS block of ts.DUTConfig; if the
// ATE is an ATEDevice, the second will be applied to s.ATETop, otherwise the first will be called
// again on s.ATEConf
func (s *TestSession) ConfigISIS(t testing.TB, ocFn func(*telemetry.NetworkInstance_Protocol_Isis)) {
	ocFn(s.DUTConf.GetOrCreateNetworkInstance(*deviations.DefaultNetworkInstance).GetOrCreateProtocol(PTISIS, ISISName).GetOrCreateIsis())
	// ateFn(s.ATEInterface(t, "port1").ISIS())
}

// PushAndStart calss PushDUT and PushAndStartATE to send config to both devices.
func (s *TestSession) PushAndStart(t testing.TB) {
	t.Helper()
	s.PushDUT(t)
	s.PushAndStartATE(t)
}

// PushDUT replaces DUT config with s.dutConf. Only interfaces and the ISIS protocol are written.
func (s *TestSession) PushDUT(t testing.TB) {
	t.Helper()
	// Push the interfaces
	for name, conf := range s.DUTConf.Interface {
		node := s.DUT.Config().Interface(name)
		node.Replace(t, conf)
	}
	//configure Network Instance
	dutConfNIPath := s.DUT.Config().NetworkInstance(*deviations.DefaultNetworkInstance)
	dutConfNIPath.Type().Replace(t, telemetry.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

	// Push the ISIS protocol
	dutNode := s.DUT.Config().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(PTISIS, ISISName)
	dutConf := s.DUTConf.GetOrCreateNetworkInstance(*deviations.DefaultNetworkInstance).GetOrCreateProtocol(PTISIS, ISISName)
	dutNode.Replace(t, dutConf)
}

// PushDUT replaces DUT config with s.dutConf. Only interfaces and the ISIS protocol are written.
func (s *TestSession) PushISISDUTConfig(t testing.TB) {
	t.Helper()
	//configure Network Instance
	dutConfNIPath := s.DUT.Config().NetworkInstance(*deviations.DefaultNetworkInstance)
	dutConfNIPath.Type().Replace(t, telemetry.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

	// Push the ISIS protocol
	dutNode := s.DUT.Config().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(PTISIS, ISISName)
	dutConf := s.DUTConf.GetOrCreateNetworkInstance(*deviations.DefaultNetworkInstance).GetOrCreateProtocol(PTISIS, ISISName)
	dutNode.Replace(t, dutConf)
}

// PushAndStartATE configures the ATE, using s.ATEConfig if the ATE is a DUTDevice and s.ATETop if the ATE
// is an ATEDevice. If it is an ATEDevice, this will also call StartProtocols() on it.
func (s *TestSession) PushAndStartATE(t testing.TB) {
	t.Helper()
	otg := s.ATE.OTG()
	otg.PushConfig(t, s.ATETop)
	otg.StartProtocols(t)
}

// AwaitAdjacency waits up to a minute for the dut to report that the ISISIntf link has formed an
// IS-IS adjaceny, logging the full state and Fataling out if it doesn't.
func (s *TestSession) AwaitAdjacency(t testing.TB) {
	t.Logf("Waiting for any adjacency to form on %v...", s.DUT.Port(t, "port1").Name())
	telem := s.DUT.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(PTISIS, ISISName)
	intf := telem.Isis().Interface(s.DUT.Port(t, "port1").Name())

	_, ok := intf.LevelAny().AdjacencyAny().AdjacencyState().Watch(t, time.Minute,
		func(val *telemetry.QualifiedE_IsisTypes_IsisInterfaceAdjState) bool {
			return val.IsPresent() && val.Val(t) == telemetry.IsisTypes_IsisInterfaceAdjState_UP
		}).Await(t)
	if !ok {
		fptest.LogYgot(t, fmt.Sprintf("IS-IS state on %v has no adjacencies", s.DUT.Port(t, "port1").Name()), intf, intf.Get(t))
		t.Fatal("No IS-IS adjacencies reported.")
	}
}
