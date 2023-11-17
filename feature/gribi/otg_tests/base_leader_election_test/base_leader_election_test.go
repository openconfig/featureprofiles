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

package base_leader_election_test

import (
	"context"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the test
// topology.
//
// The testbed consists of ate:port1 -> dut:port1,
// dut:port2 -> ate:port2 and dut:port3 -> ate:port3.
//
//   * ate:port1 -> dut:port1 subnet 192.0.2.0/30
//   * ate:port2 -> dut:port2 subnet 192.0.2.4/30
//   * ate:port3 -> dut:port3 subnet 192.0.2.8/30
//
//   * Destination network: 198.51.100.0/24

const (
	ipv4PrefixLen   = 30
	ateDstNetCIDR   = "198.51.100.0/24"
	ateDstNetStart  = "198.51.100.0"
	nhIndex         = 1
	nhgIndex        = 42
	trafficDuration = 10 * time.Second
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "port1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "port2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort3 = attrs.Attributes{
		Desc:    "dutPort3",
		IPv4:    "192.0.2.9",
		IPv4Len: ipv4PrefixLen,
	}

	atePort3 = attrs.Attributes{
		Name:    "port3",
		MAC:     "02:00:03:01:01:01",
		IPv4:    "192.0.2.10",
		IPv4Len: ipv4PrefixLen,
	}
)

var gatewayMap = map[attrs.Attributes]attrs.Attributes{
	atePort1: dutPort1,
	atePort2: dutPort2,
	atePort3: dutPort3,
}

// configInterfaceDUT configures the interface with the Addrs.
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

// configureDUT configures port1, port2 and port3 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutPort1, dut))
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}

	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutPort2, dut))
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}

	p3 := dut.Port(t, "port3")
	i3 := &oc.Interface{Name: ygot.String(p3.Name())}
	gnmi.Replace(t, dut, d.Interface(p3.Name()).Config(), configInterfaceDUT(i3, &dutPort3, dut))
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p3.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
		fptest.SetPortSpeed(t, p3)
	}
}

// configureATE configures port1, port2 and port3 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	top := gosnappi.NewConfig()

	top.Ports().Add().SetName(atePort1.Name)
	dev := top.Devices().Add().SetName(atePort1.Name)
	eth := dev.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(dev.Name())
	ip := eth.Ipv4Addresses().Add().SetName(dev.Name() + ".IPv4")
	ip.SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))

	top.Ports().Add().SetName(atePort2.Name)
	dev = top.Devices().Add().SetName(atePort2.Name)
	eth = dev.Ethernets().Add().SetName(atePort2.Name + ".Eth").SetMac(atePort2.MAC)
	eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(dev.Name())
	ip = eth.Ipv4Addresses().Add().SetName(dev.Name() + ".IPv4")
	ip.SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).SetPrefix(uint32(atePort2.IPv4Len))

	top.Ports().Add().SetName(atePort3.Name)
	dev = top.Devices().Add().SetName(atePort3.Name)
	eth = dev.Ethernets().Add().SetName(atePort3.Name + ".Eth").SetMac(atePort3.MAC)
	eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(dev.Name())
	ip = eth.Ipv4Addresses().Add().SetName(dev.Name() + ".IPv4")
	ip.SetAddress(atePort3.IPv4).SetGateway(dutPort3.IPv4).SetPrefix(uint32(atePort3.IPv4Len))

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	return top
}

// testTraffic generates traffic flow from source network to
// destination network via srcEndPoint to dstEndPoint and checks for
// packet loss.
func testTraffic(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, srcEndPoint, dstEndPoint attrs.Attributes) {

	// srcEndPoint is atePort1
	otg := ate.OTG()
	gwIP := gatewayMap[srcEndPoint].IPv4
	otg.StartProtocols(t)
	otgutils.WaitForARP(t, otg, config, "IPv4")
	dstMac := gnmi.Get(t, otg, gnmi.OTG().Interface(srcEndPoint.Name+".Eth").Ipv4Neighbor(gwIP).LinkLayerAddress().State())
	config.Flows().Clear().Items()
	flowipv4 := config.Flows().Add().SetName("Flow")
	flowipv4.Metrics().SetEnable(true)
	flowipv4.TxRx().Port().SetTxName(srcEndPoint.Name).SetRxName(dstEndPoint.Name)
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(srcEndPoint.MAC)
	e1.Dst().SetChoice("value").SetValue(dstMac)
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(srcEndPoint.IPv4)
	v4.Dst().Increment().SetStart(ateDstNetStart).SetCount(250)
	otg.PushConfig(t, config)
	otg.StartProtocols(t)

	t.Logf("Starting traffic")
	otg.StartTraffic(t)
	time.Sleep(15 * time.Second)
	t.Logf("Stop traffic")
	otg.StopTraffic(t)

	// Print Port metrics
	otgutils.LogPortMetrics(t, otg, config)

	// Check the flow statistics
	otgutils.LogFlowMetrics(t, otg, config)
	for _, f := range config.Flows().Items() {
		recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(f.Name()).State())
		txPackets := float32(recvMetric.GetCounters().GetOutPkts())
		rxPackets := float32(recvMetric.GetCounters().GetInPkts())
		lostPackets := txPackets - rxPackets
		if txPackets == 0 {
			t.Fatalf("TxPkts == 0, want > 0")
		}
		lossPct := lostPackets * 100 / txPackets
		if lossPct > 0 && recvMetric.GetCounters().GetOutPkts() > 0 {
			t.Errorf("Loss Pct for %s got %v, want 0", f.Name(), lossPct)
		}
	}
}

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx     context.Context
	clientA *gribi.Client
	clientB *gribi.Client
	dut     *ondatra.DUTDevice
	ate     *ondatra.ATEDevice
	top     gosnappi.Config
}

// checkAftIPv4Entry verifies that the prefix exists as an AFT IPv4 Entry.
func checkAftIPv4Entry(t *testing.T, dut *ondatra.DUTDevice, instance string, prefix string) {
	t.Helper()
	ipv4Path := gnmi.OC().NetworkInstance(instance).Afts().Ipv4Entry(prefix)
	_, found := gnmi.Watch(t, dut, ipv4Path.State(), time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		ipv4Entry, present := val.Val()
		return present && ipv4Entry.GetPrefix() == prefix
	}).Await(t)
	if !found {
		t.Fatalf("Could not find prefix %s in AFT telemetry", prefix)
	}
}

// testIPv4LeaderActiveChange first configures an IPV4 Entry through clientB
// and ensures that the entry is active by checking AFT Telemetry and traffic.
// It then configures an IPv4 entry through clientA without updating the election
// and ensures that the installation fails. Finally, it updated the ClientA election
// id to 12, configures an IPV4 through clinetA and ensures that the entry is active
// by checking AFT Telemetry and traffic.
func testIPv4LeaderActiveChange(ctx context.Context, t *testing.T, args *testArgs) {
	// Add an IPv4Entry for 198.51.100.0/24 pointing to ATE port-3 via gRIBI-B,
	// ensure that the entry is active through AFT telemetry and traffic.
	t.Logf("an IPv4Entry for %s pointing to ATE port-3 via gRIBI-B", ateDstNetCIDR)
	args.clientB.BecomeLeader(t)
	args.clientB.AddNH(t, nhIndex, atePort3.IPv4, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInRIB)
	args.clientB.AddNHG(t, nhgIndex, map[uint64]uint64{nhIndex: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInRIB)
	args.clientB.AddIPv4(t, ateDstNetCIDR, nhgIndex, deviations.DefaultNetworkInstance(args.dut), "", fluent.InstalledInRIB)

	// Verify the entry for 198.51.100.0/24 is active through AFT Telemetry.
	t.Logf("Verify the entry for %s is active through AFT Telemetry.", ateDstNetCIDR)
	checkAftIPv4Entry(t, args.dut, deviations.DefaultNetworkInstance(args.dut), ateDstNetCIDR)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	testTraffic(t, args.ate, args.top, atePort1, atePort3)

	// Add an IPv4Entry for 198.51.100.0/24 pointing to ATE port-2 via gRIBI-A,
	// ensure that the entry is ignored by the DUT.
	t.Logf("Adding an IPv4Entry for %s pointing to ATE port-2 via gRIBI-A", ateDstNetCIDR)
	args.clientA.AddNH(t, nhIndex+1, atePort2.IPv4, deviations.DefaultNetworkInstance(args.dut), fluent.ProgrammingFailed)
	args.clientA.AddNHG(t, nhgIndex+1, map[uint64]uint64{nhIndex + 1: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.ProgrammingFailed)
	args.clientA.AddIPv4(t, ateDstNetCIDR, nhgIndex+1, deviations.DefaultNetworkInstance(args.dut), "", fluent.ProgrammingFailed)

	// Send a ModifyRequest from gRIBI-A updating its election_id to make it leader,
	// followed by a ModifyRequest updating 198.51.100.0/24 pointing to ATE port-2,
	// ensure that routing is updated to receive packets for 198.51.100.0/24 at ATE port-2.
	args.clientA.BecomeLeader(t)
	t.Logf("Adding an IPv4Entry for %s pointing to ATE port-2 via client gRIBI-A", ateDstNetCIDR)
	args.clientA.AddNH(t, nhIndex+2, atePort2.IPv4, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInRIB)
	args.clientA.AddNHG(t, nhgIndex+2, map[uint64]uint64{nhIndex + 2: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInRIB)
	args.clientA.AddIPv4(t, ateDstNetCIDR, nhgIndex+2, deviations.DefaultNetworkInstance(args.dut), "", fluent.InstalledInRIB)

	// Verify the entry for 198.51.100.0/24 is active through AFT Telemetry.
	t.Logf("Verify the entry for %s is active through AFT Telemetry.", ateDstNetCIDR)
	checkAftIPv4Entry(t, args.dut, deviations.DefaultNetworkInstance(args.dut), ateDstNetCIDR)

	// Verify with traffic that the entry for 198.51.100.0/24 is installed through the ATE port-2.
	testTraffic(t, args.ate, args.top, atePort1, atePort2)
}

func TestElectionIDChange(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()

	// Configure the DUT
	configureDUT(t, dut)

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)

	// Configure the gRIBI client clientA
	clientA := gribi.Client{
		DUT:         dut,
		FIBACK:      false,
		Persistence: true,
	}
	defer clientA.Close(t)

	// Flush all entries after test. The client doesn't matter since we use Election Override.
	defer clientA.FlushAll(t)

	if err := clientA.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}

	// Configure the gRIBI client clientB
	clientB := gribi.Client{
		DUT:         dut,
		FIBACK:      false,
		Persistence: true,
	}
	defer clientB.Close(t)
	if err := clientB.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}

	args := &testArgs{
		ctx:     ctx,
		clientA: &clientA,
		clientB: &clientB,
		dut:     dut,
		ate:     ate,
		top:     top,
	}

	testIPv4LeaderActiveChange(ctx, t, args)
}
