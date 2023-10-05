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

package p4rt_daemon_failure_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/feature/experimental/p4rt/internal/p4rtutils"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"

	syspb "github.com/openconfig/gnoi/system"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ipv4PrefixLen    = 30
	ateDstNetCIDR    = "203.0.113.0/24"
	ipv4TrafficStart = "203.0.113.1"
	nhIndex          = 1
	nhgIndex         = 42
	deviceID1        = uint64(1)
	deviceID2        = uint64(2)
	lossTolerance    = float32(0.02)
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:11:01:00:00:01",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:12:01:00:00:01",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
	}

	p4rtDaemons = map[ondatra.Vendor]string{
		ondatra.ARISTA:  "P4Runtime",
		ondatra.CISCO:   "emsd",
		ondatra.JUNIPER: "p4-switch",
		ondatra.NOKIA:   "sr_p4rt_server",
	}
)

// configInterfaceDUT returns the OC Interface config for a given port.
func configInterfaceDUT(i *oc.Interface, a *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(ipv4PrefixLen)

	return i
}

// configP4RTNode returns the OC P4RT Node Component for a given port.
func configP4RTNode(nodeID string, deviceID uint64) *oc.Component {
	c := &oc.Component{}
	c.Name = ygot.String(nodeID)
	c.IntegratedCircuit = &oc.Component_IntegratedCircuit{}
	c.IntegratedCircuit.NodeId = ygot.Uint64(deviceID)
	return c
}

// configureDUT uses gNMI to configure port1 and port2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()
	nodes := p4rtutils.P4RTNodesByPort(t, dut)

	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutPort1, dut))

	n1, ok := nodes[p1.ID()]
	if !ok {
		t.Fatal("P4RT node name for port1 not found.")
	}
	gnmi.Replace(t, dut, gnmi.OC().Component(n1).Config(), configP4RTNode(n1, deviceID1))

	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutPort2, dut))
	n2, ok := nodes[p2.ID()]
	if !ok {
		t.Fatal("P4RT node name for port2 not found.")
	}
	gnmi.Replace(t, dut, gnmi.OC().Component(n2).Config(), configP4RTNode(n2, deviceID2))

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	top := gosnappi.NewConfig()

	p1 := ate.Port(t, "port1")
	atePort1.AddToOTG(top, p1, &dutPort1)

	p2 := ate.Port(t, "port2")
	atePort2.AddToOTG(top, p2, &dutPort2)

	return top
}

// startTraffic generates traffic flow from source network to
// destination network via atePort1 to atePort2.
// Returns the flow object that it creates.
func startTraffic(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) gosnappi.Flow {
	t.Helper()

	otg := ate.OTG()
	flow := top.Flows().Add().SetName("Flow")
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{atePort2.Name + ".IPv4"})
	e1 := flow.Packet().Add().Ethernet()
	e1.Src().SetValue(atePort1.MAC)
	v4 := flow.Packet().Add().Ipv4()
	v4.Src().SetValue(atePort1.IPv4)
	v4.Dst().Increment().SetStart(ipv4TrafficStart).SetCount(250)

	otg.PushConfig(t, top)
	otg.StartProtocols(t)

	otg.StartTraffic(t)

	return flow
}

// pidByName uses telemetry to find out the PID of a process
func pidByName(t *testing.T, dut *ondatra.DUTDevice, process string) (uint64, error) {
	t.Helper()
	ps := gnmi.GetAll(t, dut, gnmi.OC().System().ProcessAny().State())
	for _, p := range ps {
		if p.GetName() == process {
			return p.GetPid(), nil
		}
	}
	return 0, fmt.Errorf("could not find PID for process: %s", process)
}

func installRoutes(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()

	c := &gribi.Client{
		DUT:         dut,
		FIBACK:      false,
		Persistence: true,
	}

	t.Log("Establish gRIBI client connection")
	if err := c.Start(t); err != nil {
		return err
	}
	c.BecomeLeader(t)
	defer c.Close(t)

	t.Logf("Add an IPv4Entry for %s pointing to ATE port-2 via clientA", ateDstNetCIDR)
	c.AddNH(t, nhIndex, atePort2.IPv4, deviations.DefaultNetworkInstance(dut), fluent.InstalledInRIB)
	c.AddNHG(t, nhgIndex, map[uint64]uint64{nhIndex: 1}, deviations.DefaultNetworkInstance(dut), fluent.InstalledInRIB)
	c.AddIPv4(t, ateDstNetCIDR, nhgIndex, deviations.DefaultNetworkInstance(dut), "", fluent.InstalledInRIB)

	t.Logf("Verify through AFT Telemetry that %s is active", ateDstNetCIDR)
	ipv4Path := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().Ipv4Entry(ateDstNetCIDR)
	if got, ok := gnmi.Watch(t, dut, ipv4Path.State(), time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		ipv4Entry, present := val.Val()
		return present && ipv4Entry.GetPrefix() == ateDstNetCIDR
	}).Await(t); !ok {
		return fmt.Errorf("ipv4-entry/state/prefix got %v, want %s", got, ateDstNetCIDR)
	}
	return nil
}

func flushRoutes(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()

	c := &gribi.Client{
		DUT:         dut,
		FIBACK:      false,
		Persistence: true,
	}

	t.Log("Establish gRIBI client connection")
	if err := c.Start(t); err != nil {
		return err
	}
	defer c.Close(t)

	c.FlushAll(t)
	return nil
}

func TestP4RTDaemonFailure(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	p4rtD, ok := p4rtDaemons[dut.Vendor()]
	if !ok {
		t.Fatalf("Please add support for vendor %v in var p4rtDaemons", dut.Vendor())
	}

	t.Logf("Configure DUT")
	configureDUT(t, dut)

	t.Logf("Configure ATE")
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	if err := installRoutes(t, dut); err != nil {
		t.Fatalf("Could not install routes to DUT: %v", err)
	}
	defer func() {
		if err := flushRoutes(t, dut); err != nil {
			t.Errorf("Could not flush routes to DUT: %v", err)
		}
	}()

	flow := startTraffic(t, ate, top)

	pID, err := pidByName(t, dut, p4rtD)
	if err != nil {
		t.Fatal(err)
	}

	c := dut.RawAPIs().GNOI(t)
	req := &syspb.KillProcessRequest{
		Name:    p4rtD,
		Pid:     uint32(pID),
		Signal:  syspb.KillProcessRequest_SIGNAL_TERM,
		Restart: true,
	}
	resp, err := c.System().KillProcess(context.Background(), req)
	t.Logf("Got kill process response: %v", resp)
	if err != nil {
		t.Fatalf("Failed to execute gNOI.KillProcess, error received: %v", err)
	}

	// let traffic keep running for another 10 seconds.
	time.Sleep(10 * time.Second)

	t.Logf("Stop traffic")
	ate.OTG().StopTraffic(t)

	recvMetric := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).State())
	txPackets := float32(recvMetric.GetCounters().GetOutPkts())
	rxPackets := float32(recvMetric.GetCounters().GetInPkts())
	lostPackets := txPackets - rxPackets
	lossPct := lostPackets * 100 / txPackets

	if lossPct > lossTolerance {
		t.Errorf("FAIL: LossPct for %s got: %f, want: 0", flow.Name(), lossPct)
	}
}
