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

// Package isissession is deprecated and scoped only to be used with
// feature/experimental/isis/ate_tests/*.  Do not use elsewhere.
package isissession

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/netinstisis"
	"github.com/openconfig/ondatra/gnmi/oc/networkinstance"
	"github.com/openconfig/ondatra/gnmi/oc/ocpath"
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
		Desc:    "ATE to DUT with IS-IS",
		MAC:     "02:11:01:00:00:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
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
		Desc:    "ATE to DUT secondary link",
		MAC:     "02:12:01:00:00:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: pLen4,
		IPv6Len: pLen6,
	}
)

// ISISPath is shorthand for ProtocolPath().Isis().
func ISISPath(dut *ondatra.DUTDevice) *netinstisis.NetworkInstance_Protocol_IsisPath {
	return ProtocolPath(dut).Isis()
}

// ProtocolPath returns the path to the IS-IS protocol named ISISName on the
// default network instance.
func ProtocolPath(dut *ondatra.DUTDevice) *networkinstance.NetworkInstance_ProtocolPath {
	return ocpath.Root().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(PTISIS, ISISName)
}

// addISISOC configures basic IS-IS on a device.
func addISISOC(dev *oc.Root, areaAddress, sysID, ifaceName string, dut *ondatra.DUTDevice) {
	inst := dev.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	prot := inst.GetOrCreateProtocol(PTISIS, ISISName)
	prot.Enabled = ygot.Bool(true)
	isis := prot.GetOrCreateIsis()
	glob := isis.GetOrCreateGlobal()
	if deviations.ISISInstanceEnabledRequired(dut) {
		glob.Instance = ygot.String(ISISName)
	}
	glob.Net = []string{fmt.Sprintf("%v.%v.00", areaAddress, sysID)}
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	level := isis.GetOrCreateLevel(2)
	level.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC
	// Configure ISIS enabled flag at level
	if deviations.ISISLevelEnabled(dut) {
		level.Enabled = ygot.Bool(true)
	}
	intf := isis.GetOrCreateInterface(ifaceName)
	intf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
	intf.Enabled = ygot.Bool(true)
	// Configure ISIS level at global mode if true else at interface mode
	if deviations.ISISInterfaceLevel1DisableRequired(dut) {
		intf.GetOrCreateLevel(1).Enabled = ygot.Bool(false)
	} else {
		intf.GetOrCreateLevel(2).Enabled = ygot.Bool(true)
	}
	glob.LevelCapability = oc.Isis_LevelType_LEVEL_2
	// Configure ISIS enable flag at interface level
	intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	if deviations.ISISInterfaceAfiUnsupported(dut) {
		intf.Af = nil
	}
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
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetMetric(10)

	devIsisInt.Advanced().
		SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)

}

// TestSession is a convenience wrapper around the dut, ate, ports, and topology we're using.
type TestSession struct {
	DUT       *ondatra.DUTDevice
	DUTClient *ygnmi.Client
	ATE       *ondatra.ATEDevice
	// DUTConf, ATEConf, and ATETop can be modified by tests; calling .Push() will apply them to the
	// dut and ate.
	DUTPort1, DUTPort2, ATEPort1, ATEPort2 *ondatra.Port
	ATEIntf1, ATEIntf2                     gosnappi.Device
	// DUTConf and ATETop can be modified by tests; calling .Push() will apply
	// them to the dut and ate.
	DUTConf *oc.Root
	ATETop  gosnappi.Config
}

// New creates a new TestSession using the default global config, and
// configures the interfaces on the dut and the ate.
func New(t testing.TB) (*TestSession, error) {
	t.Helper()
	s := &TestSession{}
	s.DUT = ondatra.DUT(t, "dut")
	var err error
	s.DUTClient, err = ygnmi.NewClient(s.DUT.RawAPIs().GNMI(t), ygnmi.WithTarget(s.DUT.ID()))
	if err != nil {
		return nil, fmt.Errorf("unable to connect to gNMI on %v: %w", s.DUT, err)
	}
	s.DUTPort1 = s.DUT.Port(t, "port1")
	s.DUTPort2 = s.DUT.Port(t, "port2")
	s.DUTConf = &oc.Root{}
	// configure dut ports
	DUTISISAttrs.ConfigOCInterface(s.DUTConf.GetOrCreateInterface(s.DUTPort1.Name()), s.DUT)
	DUTTrafficAttrs.ConfigOCInterface(s.DUTConf.GetOrCreateInterface(s.DUTPort2.Name()), s.DUT)

	// If there is no ate, any operation that requires the ATE will call
	// t.Fatal() instead. This is helpful for debugging the parts of the test
	// that don't use an ATE.
	if ate, ok := ondatra.ATEs(t)["ate"]; ok {
		s.ATE = ate
		s.ATETop = gosnappi.NewConfig()
		s.ATEPort1 = s.ATE.Port(t, "port1")
		s.ATEPort2 = s.ATE.Port(t, "port2")
		s.ATEIntf1 = ATEISISAttrs.AddToOTG(s.ATETop, s.ATEPort1, DUTISISAttrs)
		s.ATEIntf2 = ATETrafficAttrs.AddToOTG(s.ATETop, s.ATEPort2, DUTTrafficAttrs)
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
	if deviations.ExplicitInterfaceInDefaultVRF(s.DUT) {
		addISISOC(s.DUTConf, DUTAreaAddress, DUTSysID, s.DUTPort1.Name()+".0", s.DUT)
	} else {
		addISISOC(s.DUTConf, DUTAreaAddress, DUTSysID, s.DUTPort1.Name(), s.DUT)
	}
	if s.ATE != nil {
		addISISTopo(s.ATEIntf1, ATEAreaAddress, ATESysID)
	}
	return s
}

// ConfigISIS takes two functions, one that operates on an OC IS-IS block and
// one that operates on an ondatra ATE IS-IS block. The first will be applied
// to the IS-IS block of ts.DUTConfig, and the second will be applied to the
// IS-IS configuration of ts.ATETop
func (s *TestSession) ConfigISIS(ocFn func(*oc.NetworkInstance_Protocol_Isis)) {
	ocFn(s.DUTConf.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(s.DUT)).GetOrCreateProtocol(PTISIS, ISISName).GetOrCreateIsis())
}

// PushAndStart calls PushDUT and PushAndStartATE to send config to both
// devices.
func (s *TestSession) PushAndStart(t testing.TB) error {
	t.Helper()
	if err := s.PushDUT(context.Background(), t); err != nil {
		return err
	}
	s.PushAndStartATE(t)
	return nil
}

// PushDUT replaces DUT config with s.dutConf. Only interfaces and the ISIS
// protocol are written.
func (s *TestSession) PushDUT(ctx context.Context, t testing.TB) error {
	// Push the interfaces
	for name, conf := range s.DUTConf.Interface {
		_, err := ygnmi.Replace(ctx, s.DUTClient, ocpath.Root().Interface(name).Config(), conf)
		if err != nil {
			return fmt.Errorf("configuring interface %s: %w", name, err)
		}
	}
	if deviations.ExplicitInterfaceInDefaultVRF(s.DUT) {
		fptest.AssignToNetworkInstance(t, s.DUT, s.DUTPort1.Name(), deviations.DefaultNetworkInstance(s.DUT), 0)
		fptest.AssignToNetworkInstance(t, s.DUT, s.DUTPort2.Name(), deviations.DefaultNetworkInstance(s.DUT), 0)
	}
	if deviations.ExplicitPortSpeed(s.DUT) {
		fptest.SetPortSpeed(t, s.DUTPort1)
		fptest.SetPortSpeed(t, s.DUTPort2)
	}

	// Push the ISIS protocol
	if _, err := ygnmi.Update(ctx, s.DUTClient, ocpath.Root().NetworkInstance(deviations.DefaultNetworkInstance(s.DUT)).Config(), &oc.NetworkInstance{
		Name: ygot.String(deviations.DefaultNetworkInstance(s.DUT)),
		Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE,
	}); err != nil {
		return fmt.Errorf("configuring network instance: %w", err)
	}
	dutConf := s.DUTConf.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(s.DUT)).GetOrCreateProtocol(PTISIS, ISISName)
	_, err := ygnmi.Replace(ctx, s.DUTClient, ProtocolPath(s.DUT).Config(), dutConf)
	if err != nil {
		return fmt.Errorf("configuring ISIS: %w", err)
	}
	return nil
}

// PushAndStartATE pushes the ATETop to the ATE and starts protocols on it.
func (s *TestSession) PushAndStartATE(t testing.TB) {
	t.Helper()
	otg := s.ATE.OTG()
	otg.PushConfig(t, s.ATETop)
	otg.StartProtocols(t)
}

// AwaitAdjacency waits up to a minute for the dut to report that the ISISIntf
// link has formed any IS-IS adjacency, returning the adjacency ID or an error
// if one doesn't form.
func (s *TestSession) AwaitAdjacency() (string, error) {
	intf := ISISPath(s.DUT).Interface(s.DUTPort1.Name())
	if deviations.ExplicitInterfaceInDefaultVRF(s.DUT) {
		intf = ISISPath(s.DUT).Interface(s.DUTPort1.Name() + ".0")
	}
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
func (s *TestSession) MustATEInterface(t testing.TB, portID string) gosnappi.Device {
	if s.ATE == nil {
		t.Fatal("Cannot run test without ATE")
	}
	for _, d := range s.ATETop.Devices().Items() {
		Eth := d.Ethernets().Items()[0]
		if Eth.Connection().PortName() == portID {
			return d
		}
	}
	return nil
}

// GetPacketLoss returns the packet loss for a given flow
func (s *TestSession) GetPacketLoss(t testing.TB, flow gosnappi.Flow) float32 {
	t.Helper()
	flowMetric := gnmi.Get(t, s.ATE.OTG(), gnmi.OTG().Flow(flow.Name()).State())
	txPackets := float32(flowMetric.GetCounters().GetOutPkts())
	rxPackets := float32(flowMetric.GetCounters().GetInPkts())
	lossPct := (txPackets - rxPackets) * 100 / txPackets

	if txPackets == 0 {
		return -1
	}
	return lossPct
}
