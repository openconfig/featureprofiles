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

// package session is scoped only to be used with feature/experimental/isis/ate_tests/*
package session

import (
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/ixnet"
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
	ISISName       = "osiris"
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
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		Desc:    "ATE to DUT secondary link",
		IPv4Len: pLen4,
		IPv6Len: pLen6,
	}
)

// addISISOC configures basic IS-IS on a device.
func addISISOC(dev *telemetry.Device, areaAddress, sysID, ifaceName string) {
	inst := dev.GetOrCreateNetworkInstance("default")
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
func addISISTopo(iface *ondatra.Interface, areaAddress, sysID string) {
	// Find the interface (why doesn't ondatra have a function for this?)
	// configure IS-IS on it
	isis := iface.ISIS()
	isis.
		WithAreaID(areaAddress).
		WithTERouterID(sysID).
		WithNetworkTypePointToPoint().
		WithWideMetricEnabled(true).
		WithLevelL2()
}

// TestSession is a convenience wrapper around the dut, ate, ports, and topology we're using.
type TestSession struct {
	DUT *ondatra.DUTDevice
	ATE *ondatra.ATEDevice
	// DUTConf, ATEConf, and ATETop can be modified by tests; calling .Push() will apply them to the
	// dut and ate.
	DUTConf, ATEConf *telemetry.Device
	ATETop           *ondatra.ATETopology
}

// DUTISISTelemetry gets the telemetry PathStruct for /network-instance[default]/protocol[ISIS]/isis on the DUT
func (s *TestSession) DUTISISTelemetry(t testing.TB) *networkinstance.NetworkInstance_Protocol_IsisPath {
	return s.DUT.Telemetry().NetworkInstance("default").Protocol(PTISIS, ISISName).Isis()
}

// ATEISISTelemetry gets the telemetry PathStruct for /network-instance[default]/protocol[ISIS]/isis on the ATE
func (s *TestSession) ATEISISTelemetry(t testing.TB) *networkinstance.NetworkInstance_Protocol_IsisPath {
	return s.ATE.Telemetry().NetworkInstance("default").Protocol(PTISIS, ISISName).Isis()
}

// ATEInterface returns an ondatra.Interface for the port with the given name, or nil if our ATE is
// actually an ondatra.DUTDevice.
func (s *TestSession) ATEInterface(t testing.TB, portID string) *ondatra.Interface {
	iface, ok := s.ATETop.Interfaces()[portID]
	if !ok {
		t.Logf("Available ATE Interfaces:")
		for name := range s.ATETop.Interfaces() {
			t.Logf("  %v", name)
		}
		t.Fatalf("No ATE interface with ID %v", portID)
	}
	return iface
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
	s.ATETop = s.ATE.Topology().New()
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
	attrs.AddToATE(s.ATETop, s.ATE.Port(t, portID), peerAttrs)
}

// WithISIS adds ISIS to a test session.
func (s *TestSession) WithISIS(t testing.TB) *TestSession {
	t.Helper()
	addISISOC(s.DUTConf, DUTAreaAddress, DUTSysID, s.DUT.Port(t, "port1").Name())
	if s.ATEInterface(t, "port1") == nil {
		t.Fatalf("Nil interface:\n***\nATE: %v\n***\n***\nIfaces:\n%v\n***\nPortName: %v\n***\n", s.ATE, s.ATETop.Interfaces(), s.ATE.Port(t, "port1").Name())
	}
	addISISTopo(s.ATEInterface(t, "port1"), ATEAreaAddress, "*")
	return s
}

// ConfigISIS takes two functions, one that operates on an OC IS-IS block and one that operates on
// an ondatra ATE IS-IS block. The first will be applied to the IS-IS block of ts.DUTConfig; if the
// ATE is an ATEDevice, the second will be applied to s.ATETop, otherwise the first will be called
// again on s.ATEConf
func (s *TestSession) ConfigISIS(t testing.TB, ocFn func(*telemetry.NetworkInstance_Protocol_Isis), ateFn func(*ixnet.ISIS)) {
	ocFn(s.DUTConf.GetOrCreateNetworkInstance("default").GetOrCreateProtocol(PTISIS, ISISName).GetOrCreateIsis())
	ateFn(s.ATEInterface(t, "port1").ISIS())
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
	// Push the ISIS protocol
	dutNode := s.DUT.Config().NetworkInstance("default").Protocol(PTISIS, ISISName)
	dutConf := s.DUTConf.GetOrCreateNetworkInstance("default").GetOrCreateProtocol(PTISIS, ISISName)
	dutNode.Replace(t, dutConf)
}

// PushAndStartATE configures the ATE, using s.ATEConfig if the ATE is a DUTDevice and s.ATETop if the ATE
// is an ATEDevice. If it is an ATEDevice, this will also call StartProtocols() on it.
func (s *TestSession) PushAndStartATE(t testing.TB) {
	t.Helper()
	s.ATETop.Push(t).StartProtocols(t)
}

// AwaitAdjacency waits up to a minute for the dut to report that the ISISIntf link has formed an
// IS-IS adjaceny, logging the full state and Fataling out if it doesn't.
func (s *TestSession) AwaitAdjacency(t testing.TB) {
	t.Logf("Waiting for any adjacency to form on %v...", s.DUT.Port(t, "port1").Name())
	telem := s.DUT.Telemetry().NetworkInstance("default").Protocol(PTISIS, ISISName)
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
