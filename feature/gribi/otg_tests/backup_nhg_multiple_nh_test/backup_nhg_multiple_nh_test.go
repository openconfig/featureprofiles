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

package backup_nhg_multiple_nh_test

import (
	"context"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygot/ygot"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	ipv4PrefixLen  = 30
	ipv6PrefixLen  = 126
	dstPfx         = "203.0.113.1/32"
	dstPfxMin      = "203.0.113.1"
	dstPfxMax      = "203.0.113.254"
	routeCount     = 1
	vrf1           = "vrfA"
	vrf2           = "vrfB"
	fps            = 1000000 // traffic frames per second
	switchovertime = 250.0   // switchovertime during interface shut in milliseconds
)

// testArgs holds the objects needed by a test case.
type testArgs struct {
	dut    *ondatra.DUTDevice
	ate    *ondatra.ATEDevice
	top    gosnappi.Config
	ctx    context.Context
	client *gribi.Client
}

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:1",
		IPv6Len: ipv6PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:2",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:5",
		IPv6Len: ipv6PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:6",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort3 = attrs.Attributes{
		Desc:    "dutPort3",
		IPv4:    "192.0.2.9",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:9",
		IPv6Len: ipv6PrefixLen,
	}

	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		MAC:     "02:00:03:01:01:01",
		IPv4:    "192.0.2.10",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:a",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort4 = attrs.Attributes{
		Desc:    "dutPort4",
		IPv4:    "192.0.2.13",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:D",
		IPv6Len: ipv6PrefixLen,
	}

	atePort4 = attrs.Attributes{
		Name:    "atePort4",
		MAC:     "02:00:04:01:01:01",
		IPv4:    "192.0.2.14",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:E",
		IPv6Len: ipv6PrefixLen,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configureATE configures ports on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	top := ate.OTG().NewConfig(t)

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	p3 := ate.Port(t, "port3")
	p4 := ate.Port(t, "port4")

	atePort1.AddToOTG(top, p1, &dutPort1)
	atePort2.AddToOTG(top, p2, &dutPort2)
	atePort3.AddToOTG(top, p3, &dutPort3)
	atePort4.AddToOTG(top, p4, &dutPort4)

	return top
}

// Configure Network instance
func configNetworkInstance(t *testing.T, dut *ondatra.DUTDevice, vrfname string) {
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(vrfname)
	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrfname).Config(), ni)
}

// configNetworkInstanceInterface creates VRFs and subinterfaces and then applies VRFs
func configNetworkInstanceInterface(t *testing.T, dut *ondatra.DUTDevice, vrfname string, intfname string, subint uint32) {
	// create empty subinterface
	si := &oc.Interface_Subinterface{}
	si.Index = ygot.Uint32(subint)
	gnmi.Replace(t, dut, gnmi.OC().Interface(intfname).Subinterface(subint).Config(), si)

	// create vrf and apply on subinterface
	v := &oc.NetworkInstance{
		Name: ygot.String(vrfname),
		Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
	}
	vi := v.GetOrCreateInterface(intfname)
	vi.Interface = ygot.String(intfname)
	vi.Subinterface = ygot.Uint32(subint)
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrfname).Config(), v)
}

// configInterfaceDUT configures the interface
func configInterfaceDUT(i *oc.Interface, dutPort *attrs.Attributes) *oc.Interface {
	if *deviations.InterfaceEnabled {
		i.Enabled = ygot.Bool(true)
	}
	i.Description = ygot.String(dutPort.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	return i
}

// configureDUT configures port1, port2, port3 and port4 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	// create VRF "vrfA" and assign incoming port under it
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutPort1))
	configNetworkInstanceInterface(t, dut, vrf1, p1.Name(), uint32(0))
	// create VRF "vrfB"
	configNetworkInstance(t, dut, vrf2)

	gnmi.Update(t, dut, d.Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name()))

	p2 := dut.Port(t, "port2")
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name()))

	p3 := dut.Port(t, "port3")
	gnmi.Replace(t, dut, d.Interface(p3.Name()).Config(), dutPort3.NewOCInterface(p3.Name()))

	p4 := dut.Port(t, "port4")
	gnmi.Replace(t, dut, d.Interface(p4.Name()).Config(), dutPort4.NewOCInterface(p4.Name()))

	if *deviations.ExplicitPortSpeed {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
		fptest.SetPortSpeed(t, p3)
		fptest.SetPortSpeed(t, p4)
	}
	if *deviations.ExplicitInterfaceInDefaultVRF {
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), *deviations.DefaultNetworkInstance, 0)
		fptest.AssignToNetworkInstance(t, dut, p3.Name(), *deviations.DefaultNetworkInstance, 0)
		fptest.AssignToNetworkInstance(t, dut, p4.Name(), *deviations.DefaultNetworkInstance, 0)
	}
	if *deviations.ExplicitGRIBIUnderNetworkInstance {
		fptest.EnableGRIBIUnderNetworkInstance(t, dut, *deviations.DefaultNetworkInstance)
		fptest.EnableGRIBIUnderNetworkInstance(t, dut, vrf1)
		fptest.EnableGRIBIUnderNetworkInstance(t, dut, vrf2)
	}
}

func TestBackup(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	//configure DUT
	configureDUT(t, dut)

	// Configure ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	waitOTGARPEntry(t)

	t.Run("IPv4BackUpSwitch", func(t *testing.T) {
		t.Logf("Name: IPv4BackUpSwitch")
		t.Logf("Description: Set primary and backup path with gribi and shutdown the primary path validating traffic switching over backup path")

		// Configure the gRIBI client clientA
		client := gribi.Client{
			DUT:         dut,
			FIBACK:      true,
			Persistence: true,
		}
		defer client.Close(t)

		// Flush all entries after the test
		defer client.FlushAll(t)

		if err := client.Start(t); err != nil {
			t.Fatalf("gRIBI Connection can not be established")
		}

		// Make client leader
		client.BecomeLeader(t)

		// Flush past entries before running the tc
		client.FlushAll(t)

		tcArgs := &testArgs{
			ctx:    ctx,
			dut:    dut,
			client: &client,
			ate:    ate,
			top:    top,
		}
		tcArgs.testIPv4BackUpSwitch(t)
	})
}

// testIPv4BackUpSwitch Ensure that backup NHGs are honoured with NextHopGroup entries containing >1 NH
//
// Setup Steps
//   - Connect ATE port-1 to DUT port-1.
//   - Connect ATE port-2 to DUT port-2.
//   - Connect ATE port-3 to DUT port-3.
//   - Connect ATE port-4 to DUT port-4.
//   - Create a L3 routing instance (vrfA), and assign DUT port-1 to vrfA.
//   - Create a L3 routing instance (vrfB) that includes no interface.
//   - Connect a gRIBI client to the DUT, make it become leader and inject the following:
//   - An IPv4Entry in VRF-A, pointing to a NextHopGroup (in DEFAULT VRF) containing:
//   - Two primary next-hops:
//   - IP of ATE port-2
//   - IP of ATE port-3
//   - A backup NHG containing a single next-hop pointing to VRF-B.
//   - The same IPv4Entry but in VRF-B, pointing to a NextHopGroup (in DEFAULT VRF) containing a primary next-hop to the IP of ATE port-4.
//   - Ensure that traffic forwarded to a destination is received at ATE port-2 and port-3. Validate that AFT telemetry covers this case.
//   - Disable ATE port-2. Ensure that traffic for a destination is received at ATE port-3.
//   - Disable ATE port-3. Ensure that traffic for a destination is received at ATE port-4.
//
// Validation Steps
//   - Verify AFT telemetry after shutting each port
//   - Verify traffic switches to the right ports
func (a *testArgs) testIPv4BackUpSwitch(t *testing.T) {

	const (
		// Next hop group adjacency identifier.
		nhgid1, nhgid2 uint64 = 100, 200
		// Backup next hop group ID that the dstPfx will forward to.
		backupnhgid uint64 = 500

		nhid1, nhid2, nhid3, nhid4 uint64 = 1001, 1002, 1003, 1004
	)

	t.Logf("Program a backup pointing to vrfB via gRIBI")
	a.client.AddNH(t, nhid3, "VRFOnly", *deviations.DefaultNetworkInstance, fluent.InstalledInFIB, &gribi.NHOptions{VrfName: vrf2})
	a.client.AddNHG(t, backupnhgid, map[uint64]uint64{nhid3: 10}, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)

	t.Logf("an IPv4Entry for %s in %s pointing to ATE port-2 and port-3 via gRIBI", dstPfx, vrf1)
	a.client.AddNH(t, nhid1, atePort2.IPv4, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	a.client.AddNH(t, nhid2, atePort3.IPv4, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	a.client.AddNHG(t, nhgid1, map[uint64]uint64{nhid1: 80, nhid2: 20}, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: backupnhgid})
	a.client.AddIPv4(t, dstPfx, nhgid1, vrf1, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)

	t.Logf("an IPv4Entry for %s in %s pointing to ATE port-4 via gRIBI", dstPfx, vrf2)
	a.client.AddNH(t, nhid4, atePort4.IPv4, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	a.client.AddNHG(t, nhgid2, map[uint64]uint64{nhid4: 100}, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	a.client.AddIPv4(t, dstPfx, nhgid2, vrf2, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)

	// validate programming using AFT
	// TODO: add checks for NHs when AFT OC schema concludes how viability should be indicated.
	a.aftCheck(t, dstPfx, vrf2)

	// create flow
	dstMac := gnmi.Get(t, a.ate.OTG(), gnmi.OTG().Interface(atePort1.Name+".Eth").Ipv4Neighbor(dutPort1.IPv4).LinkLayerAddress().State())
	BaseFlow := a.createFlow(t, "BaseFlow", dstMac)

	// Validate traffic over primary path port2, port3
	t.Logf("Validate traffic over primary path port2, port3")
	a.validateTrafficFlows(t, BaseFlow, []*ondatra.Port{a.ate.Port(t, "port2"), a.ate.Port(t, "port3")})

	//shutdown port2
	t.Logf("Shutdown port 2 and validate traffic switching over port3 primary path")
	a.validateTrafficFlows(t, BaseFlow, []*ondatra.Port{a.ate.Port(t, "port3")}, "port2")
	defer a.flapinterface(t, "port2", true)
	// TODO: add checks for NHs when AFT OC schema concludes how viability should be indicated.

	//shutdown port3
	t.Logf("Shutdown port 3 and validate traffic switching over port4 backup path")
	a.validateTrafficFlows(t, BaseFlow, []*ondatra.Port{a.ate.Port(t, "port4")}, "port3")
	defer a.flapinterface(t, "port3", true)
	// TODO: add checks for NHs when AFT OC schema concludes how viability should be indicated.
}

// createFlow returns a flow from atePort1 to the dstPfx
func (a *testArgs) createFlow(t *testing.T, name, dstMac string) string {

	flow := a.top.Flows().Add().SetName(name)
	flow.Metrics().SetEnable(true)
	flow.Size().SetFixed(300)
	e1 := flow.Packet().Add().Ethernet()
	e1.Src().SetValue(atePort1.MAC)
	flow.TxRx().Port().SetTxName("port1")
	e1.Dst().SetChoice("value").SetValue(dstMac)
	v4 := flow.Packet().Add().Ipv4()
	v4.Src().SetValue(atePort1.IPv4)
	v4.Dst().SetValue(dstPfxMin)
	a.ate.OTG().PushConfig(t, a.top)
	// StartProtocols required for running on hardware
	a.ate.OTG().StartProtocols(t)
	return name

}

// validateTrafficFlows verifies that the flow on ATE and check interface counters on DUT
func (a *testArgs) validateTrafficFlows(t *testing.T, flow string, expected_outgoing_port []*ondatra.Port, shut_ports ...string) {
	a.ate.OTG().StartTraffic(t)
	//Shutdown interface if provided while traffic is flowing and validate traffic
	time.Sleep(30 * time.Second)
	for _, port := range shut_ports {
		a.flapinterface(t, port, false)
		gnmi.Await(t, a.dut, gnmi.OC().Interface(a.dut.Port(t, port).Name()).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_DOWN)
	}
	time.Sleep(30 * time.Second)
	a.ate.OTG().StopTraffic(t)
	otgutils.LogFlowMetrics(t, a.ate.OTG(), a.top)
	otgutils.LogPortMetrics(t, a.ate.OTG(), a.top)
	// Get send traffic
	incoming_traffic_state := gnmi.OTG().Port(a.ate.Port(t, "port1").ID()).State()
	sentPkts := gnmi.Get(t, a.ate.OTG(), incoming_traffic_state).GetCounters().GetOutFrames()
	if sentPkts == 0 {
		t.Fatalf("Tx packets should be higher than 0")
	}

	var receivedPkts uint64

	// Get traffic received on primary outgoing interface before interface shutdown
	for _, port := range shut_ports {
		outgoing_traffic_counters := gnmi.OTG().Port(a.ate.Port(t, port).ID()).State()
		outPkts := gnmi.Get(t, a.ate.OTG(), outgoing_traffic_counters).GetCounters().GetInFrames()
		receivedPkts = receivedPkts + outPkts
	}

	// Get traffic received on expected port after interface shut
	for _, outPort := range expected_outgoing_port {
		outgoing_traffic_counters := gnmi.OTG().Port(outPort.ID()).State()
		outPkts := gnmi.Get(t, a.ate.OTG(), outgoing_traffic_counters).GetCounters().GetInFrames()
		receivedPkts = receivedPkts + outPkts
	}

	// Check if traffic restores with in expected time in milliseconds during interface shut
	// else if there is no interface trigger, validate received packets (control+data) are more than send packets
	if len(shut_ports) > 0 {
		// Time took for traffic to restore in milliseconds after trigger
		fpm := ((sentPkts - receivedPkts) / (fps / 1000))
		if fpm > switchovertime {
			t.Fatalf("Traffic loss %v msecs more than expected %v msecs", fpm, switchovertime)
		}
		t.Logf("Traffic loss during path change : %v msecs", fpm)
	} else if sentPkts > receivedPkts {
		t.Fatalf("Traffic didn't switch to the expected outgoing port")
	}
}

// flapinterface shut/unshut interface, action true bringsup the interface and false brings it down
func (a *testArgs) flapinterface(t *testing.T, port string, action bool) {
	// Currently, setting the OTG port down has no effect on kne and thus the corresponding dut port will be used
	dutP := a.dut.Port(t, port)
	dc := gnmi.OC()
	i := &oc.Interface{}
	i.Enabled = ygot.Bool(action)
	gnmi.Update(t, a.dut, dc.Interface(dutP.Name()).Config(), i)
}

// aftCheck does ipv4, NHG and NH aft check
// TODO: add checks for NHs when AFT OC schema concludes how viability should be indicated.

func (a *testArgs) aftCheck(t testing.TB, prefix string, instance string) {
	// check prefix and get NHG ID
	aftPfxNHG := gnmi.OC().NetworkInstance(instance).Afts().Ipv4Entry(prefix).NextHopGroup()
	aftPfxNHGVal, found := gnmi.Watch(t, a.dut, aftPfxNHG.State(), 2*time.Minute, func(val *ygnmi.Value[uint64]) bool {
		return val.IsPresent()
	}).Await(t)
	if !found {
		t.Fatalf("Could not find prefix %s in telemetry AFT", dstPfx)
	}
	nhg, _ := aftPfxNHGVal.Val()

	// using NHG ID validate NH
	aftNHG := gnmi.Get(t, a.dut, gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().NextHopGroup(nhg).State())
	if len(aftNHG.NextHop) == 0 && aftNHG.BackupNextHopGroup == nil {
		t.Fatalf("Prefix %s references a NHG that has neither NH or backup NHG", prefix)
	}
}

// Waits for at least one ARP entry on the tx OTG interface
func waitOTGARPEntry(t *testing.T) {
	t.Helper()
	ate := ondatra.ATE(t, "ate")
	gnmi.WatchAll(t, ate.OTG(), gnmi.OTG().Interface(atePort1.Name+".Eth").Ipv4NeighborAny().LinkLayerAddress().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
		return val.IsPresent()
	}).Await(t)
}
