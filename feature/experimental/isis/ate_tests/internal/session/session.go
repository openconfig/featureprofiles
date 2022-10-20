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
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/networkinstance"
	"github.com/openconfig/ondatra/gnmi/oc/ocpath"
	"github.com/openconfig/ondatra/ixnet"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

// PTISIS is shorthand for the long oc protocol type constant
const PTISIS = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS

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
	ISISName       = "DEFAULT"
	pLen4          = 30
	pLen6          = 126
)

var (
	// Network entity title for the DUT
	DUTNET = fmt.Sprintf("%v.%v.00", DUTAreaAddress, DUTSysID)
	// DUTISISAttrs has attributes for the DUT ISIS connection on port1
	DUTISISAttrs = &Attributes{
		Desc:    "DUT to ATE with IS-IS",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: pLen4,
		IPv6Len: pLen6,
	}
	// ATEISISAttrs has attributes for the ATE ISIS connection on port1
	ATEISISAttrs = &Attributes{
		Name:    "port1",
		Desc:    "ATE to DUT with IS-IS",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: pLen4,
		IPv6Len: pLen6,
	}
	// DUTTrafficAttrs has attributes for the DUT end of the traffic connection (port2)
	DUTTrafficAttrs = &Attributes{
		Desc:    "DUT to ATE secondary link",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: pLen4,
		IPv6Len: pLen6,
	}
	// ATETrafficAttrs has attributes for the ATE end of the traffic connection (port2)
	ATETrafficAttrs = &Attributes{
		Name:    "port2",
		Desc:    "ATE to DUT secondary link",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: pLen4,
		IPv6Len: pLen6,
	}
)

// ISISPath is shorthand for ProtocolPath().Isis().
func ISISPath() *networkinstance.NetworkInstance_Protocol_IsisPath {
	return ProtocolPath().Isis()
}

// ProtocolPath returns the path to the IS-IS protocol named ISISName on the
// default network instance.
func ProtocolPath() *networkinstance.NetworkInstance_ProtocolPath {
	return ocpath.Root().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(PTISIS, ISISName)
}

// addISISOC configures basic IS-IS on a device.
func addISISOC(dev *oc.Root, areaAddress, sysID, ifaceName string) {
	inst := dev.GetOrCreateNetworkInstance(*deviations.DefaultNetworkInstance)
	prot := inst.GetOrCreateProtocol(PTISIS, ISISName)
	isis := prot.GetOrCreateIsis()
	glob := isis.GetOrCreateGlobal()
	glob.Instance = ygot.String(ISISName)
	glob.Net = []string{fmt.Sprintf("%v.%v.00", areaAddress, sysID)}
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	intf := isis.GetOrCreateInterface(ifaceName)
	intf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
	intf.Enabled = ygot.Bool(true)
	intf.GetOrCreateLevel(2).Enabled = ygot.Bool(true)
}

// addISISTopo configures basic IS-IS on an ATETopology interface.
func addISISTopo(iface *ondatra.Interface, areaAddress, sysID string) {
	isis := iface.ISIS()
	isis.
		WithAreaID(areaAddress).
		WithTERouterID(sysID).
		WithNetworkTypePointToPoint().
		WithWideMetricEnabled(true).
		WithLevelL2()
}

// TestSession is a convenience wrapper around the dut, ate, ports, and
// topology we're using.
type TestSession struct {
	DUT       *ondatra.DUTDevice
	DUTClient *ygnmi.Client
	ATE       *ondatra.ATEDevice
	// Rather than looking these up all the time, we fetch all the relevant ports
	// and interfaces at setup time.
	DUTPort1, DUTPort2, ATEPort1, ATEPort2 *ondatra.Port
	ATEIntf1, ATEIntf2                     *ondatra.Interface
	// DUTConf and ATETop can be modified by tests; calling .Push() will apply
	// them to the dut and ate.
	DUTConf *oc.Root
	ATETop  *ondatra.ATETopology
}

// New creates a new TestSession using the default global config, and
// configures the interfaces on the dut and the ate.
func New(t testing.TB) (*TestSession, error) {
	t.Helper()
	s := &TestSession{}
	s.DUT = ondatra.DUT(t, "dut")
	var err error
	s.DUTClient, err = ygnmi.NewClient(s.DUT.RawAPIs().GNMI().Default(t), ygnmi.WithTarget(s.DUT.ID()))
	if err != nil {
		return nil, fmt.Errorf("unable to connect to gNMI on %v: %w", s.DUT, err)
	}
	s.DUTPort1 = s.DUT.Port(t, "port1")
	s.DUTPort2 = s.DUT.Port(t, "port2")
	s.DUTConf = &oc.Root{}
	// configure dut ports
	DUTISISAttrs.ConfigInterface(s.DUTConf.GetOrCreateInterface(s.DUTPort1.Name()))
	DUTTrafficAttrs.ConfigInterface(s.DUTConf.GetOrCreateInterface(s.DUTPort2.Name()))

	// If there is no ate, any operation that requires the ATE will call
	// t.Fatal() instead. This is helpful for debugging the parts of the test
	// that don't use an ATE.
	if ate, ok := ondatra.ATEs(t)["ate"]; ok {
		s.ATE = ate
		s.ATEPort1 = s.ATE.Port(t, "port1")
		s.ATEPort2 = s.ATE.Port(t, "port2")
		s.ATETop = s.ATE.Topology().New()
		s.ATEIntf1 = ATEISISAttrs.AddToATE(s.ATETop, s.ATEPort1, DUTISISAttrs)
		s.ATEIntf2 = ATETrafficAttrs.AddToATE(s.ATETop, s.ATEPort2, DUTTrafficAttrs)
	}
	return s, nil
}

// MustNew creates a new TestSession or Fatal()s if anything goes wrong.
func MustNew(t testing.TB) *TestSession {
	t.Helper()
	v, err := New(t)
	if err != nil {
		t.Fatalf("Unable to initialize topology: %v", err)
	}
	return v
}

// WithISIS adds ISIS to a test session.
func (s *TestSession) WithISIS() *TestSession {
	addISISOC(s.DUTConf, DUTAreaAddress, DUTSysID, s.DUTPort1.Name())
	if s.ATE != nil {
		addISISTopo(s.ATEIntf1, ATEAreaAddress, "*")
	}
	return s
}

// ConfigISIS takes two functions, one that operates on an OC IS-IS block and
// one that operates on an ondatra ATE IS-IS block. The first will be applied
// to the IS-IS block of ts.DUTConfig, and the second will be applied to the
// IS-IS configuration of ts.ATETop
func (s *TestSession) ConfigISIS(ocFn func(*oc.NetworkInstance_Protocol_Isis), ateFn func(*ixnet.ISIS)) {
	ocFn(s.DUTConf.GetOrCreateNetworkInstance(*deviations.DefaultNetworkInstance).GetOrCreateProtocol(PTISIS, ISISName).GetOrCreateIsis())
	if s.ATE != nil {
		ateFn(s.ATEIntf1.ISIS())
	}
}

// PushAndStart calls PushDUT and PushAndStartATE to send config to both
// devices.
func (s *TestSession) PushAndStart(t testing.TB) error {
	t.Helper()
	if err := s.PushDUT(context.Background()); err != nil {
		return err
	}
	s.PushAndStartATE(t)
	return nil
}

// PushDUT replaces DUT config with s.dutConf. Only interfaces and the ISIS
// protocol are written.
func (s *TestSession) PushDUT(ctx context.Context) error {
	// Push the interfaces
	for name, conf := range s.DUTConf.Interface {
		_, err := ygnmi.Replace(ctx, s.DUTClient, ocpath.Root().Interface(name).Config(), conf)
		if err != nil {
			return fmt.Errorf("configuring interface %s: %w", name, err)
		}
	}
	// Push the ISIS protocol
	if _, err := ygnmi.Replace(ctx, s.DUTClient, ocpath.Root().NetworkInstance(*deviations.DefaultNetworkInstance).Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE); err != nil {
		return fmt.Errorf("configuring network instance: %w", err)
	}
	dutConf := s.DUTConf.GetOrCreateNetworkInstance(*deviations.DefaultNetworkInstance).GetOrCreateProtocol(PTISIS, ISISName)
	_, err := ygnmi.Replace(ctx, s.DUTClient, ProtocolPath().Config(), dutConf)
	if err != nil {
		return fmt.Errorf("configuring ISIS: %w", err)
	}
	return nil
}

// PushAndStartATE pushes the ATETop to the ATE and starts protocols on it.
func (s *TestSession) PushAndStartATE(t testing.TB) {
	t.Helper()
	if s.ATE == nil {
		t.Fatal("Cannot run test without ATE")
	}
	s.ATETop.Push(t).StartProtocols(t)
}

// AwaitAdjacency waits up to a minute for the dut to report that the ISISIntf
// link has formed any IS-IS adjacency, returning the adjacency ID or an error
// if one doesn't form.
func (s *TestSession) AwaitAdjacency() (string, error) {
	intf := ISISPath().Interface(s.DUTPort1.Name())
	query := intf.LevelAny().AdjacencyAny().AdjacencyState().State()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	watcher := ygnmi.WatchAll(ctx, s.DUTClient, query, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) error {
		if val == nil || !val.IsPresent() {
			return ygnmi.Continue
		}
		v, _ := val.Val()
		if v == oc.Isis_IsisInterfaceAdjState_UP {
			return nil
		}
		return ygnmi.Continue
	})
	got, err := watcher.Await()
	if err != nil {
		return "", err
	}
	return got.Path.GetElem()[10].GetKey()["system-id"], nil
}

// MustAdjacency waits up to a minute for an IS-IS adjacency to form between
// the DUT and the ATE; it returns the adjacency ID or calls t.Fatal no
// adjacency forms.
func (s *TestSession) MustAdjacency(t testing.TB) string {
	adjID, err := s.AwaitAdjacency()
	if err != nil {
		t.Fatalf("Waiting for adjacency to form: %v", err)
	}
	return adjID
}

// MustATEInterface returns the ATE interface for the portID, or calls t.Fatal
// if this fails.
func (s *TestSession) MustATEInterface(t testing.TB, portID string) *ondatra.Interface {
	if s.ATE == nil {
		t.Fatal("Cannot run test without ATE")
	}
	iface, ok := s.ATETop.Interfaces()[portID]
	if !ok {
		t.Fatalf("No ATE interface with ID %v", portID)
	}
	return iface
}
