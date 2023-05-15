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

	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
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
	top    *ondatra.ATETopology
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
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	top := ate.Topology().New()

	p1 := ate.Port(t, "port1")
	i1 := top.AddInterface(atePort1.Name).WithPort(p1)
	i1.IPv4().
		WithAddress(atePort1.IPv4CIDR()).
		WithDefaultGateway(dutPort1.IPv4)
	i1.IPv6().
		WithAddress(atePort1.IPv6CIDR()).
		WithDefaultGateway(dutPort1.IPv6)

	p2 := ate.Port(t, "port2")
	i2 := top.AddInterface(atePort2.Name).WithPort(p2)
	i2.IPv4().
		WithAddress(atePort2.IPv4CIDR()).
		WithDefaultGateway(dutPort2.IPv4)
	i2.IPv6().
		WithAddress(atePort2.IPv6CIDR()).
		WithDefaultGateway(dutPort2.IPv6)

	p3 := ate.Port(t, "port3")
	i3 := top.AddInterface(atePort3.Name).WithPort(p3)
	i3.IPv4().
		WithAddress(atePort3.IPv4CIDR()).
		WithDefaultGateway(dutPort3.IPv4)
	i3.IPv6().
		WithAddress(atePort3.IPv6CIDR()).
		WithDefaultGateway(dutPort3.IPv6)

	p4 := ate.Port(t, "port4")
	i4 := top.AddInterface(atePort4.Name).WithPort(p4)
	i4.IPv4().
		WithAddress(atePort4.IPv4CIDR()).
		WithDefaultGateway(dutPort4.IPv4)
	i4.IPv6().
		WithAddress(atePort4.IPv6CIDR()).
		WithDefaultGateway(dutPort4.IPv6)

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

	gnmi.Update(t, dut, d.Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))

	p2 := dut.Port(t, "port2")
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))

	p3 := dut.Port(t, "port3")
	gnmi.Replace(t, dut, d.Interface(p3.Name()).Config(), dutPort3.NewOCInterface(p3.Name(), dut))

	p4 := dut.Port(t, "port4")
	gnmi.Replace(t, dut, d.Interface(p4.Name()).Config(), dutPort4.NewOCInterface(p4.Name(), dut))

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
		fptest.SetPortSpeed(t, p3)
		fptest.SetPortSpeed(t, p4)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p3.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p4.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
	if deviations.ExplicitGRIBIUnderNetworkInstance(dut) {
		fptest.EnableGRIBIUnderNetworkInstance(t, dut, deviations.DefaultNetworkInstance(dut))
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
	top.Push(t).StartProtocols(t)

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
	a.client.AddNH(t, nhid3, "VRFOnly", deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: vrf2})
	a.client.AddNHG(t, backupnhgid, map[uint64]uint64{nhid3: 10}, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)

	t.Logf("an IPv4Entry for %s in %s pointing to ATE port-2 and port-3 via gRIBI", dstPfx, vrf1)
	a.client.AddNH(t, nhid1, atePort2.IPv4, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
	a.client.AddNH(t, nhid2, atePort3.IPv4, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
	a.client.AddNHG(t, nhgid1, map[uint64]uint64{nhid1: 80, nhid2: 20}, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: backupnhgid})
	a.client.AddIPv4(t, dstPfx, nhgid1, vrf1, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)

	t.Logf("an IPv4Entry for %s in %s pointing to ATE port-4 via gRIBI", dstPfx, vrf2)
	a.client.AddNH(t, nhid4, atePort4.IPv4, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
	a.client.AddNHG(t, nhgid2, map[uint64]uint64{nhid4: 100}, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
	a.client.AddIPv4(t, dstPfx, nhgid2, vrf2, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)

	// validate programming using AFT
	// TODO: add checks for NHs when AFT OC schema concludes how viability should be indicated.
	a.aftCheck(t, dstPfx, vrf2)
	// Create flow
	flow := a.createFlow("Baseline Flow")

	// Validate traffic over primary path port2, port3
	t.Logf("Validate traffic over primary path port2, port3")
	a.validateTrafficFlows(t, flow, []*ondatra.Port{a.ate.Port(t, "port2"), a.ate.Port(t, "port3")})

	//shutdown port2
	t.Logf("Shutdown port 2 and validate traffic switching over port3 primary path")
	a.validateTrafficFlows(t, flow, []*ondatra.Port{a.ate.Port(t, "port3")}, "port2")
	defer a.flapinterface(t, "port2", true)
	// TODO: add checks for NHs when AFT OC schema concludes how viability should be indicated.

	//shutdown port3
	t.Logf("Shutdown port 3 and validate traffic switching over port4 backup path")
	a.validateTrafficFlows(t, flow, []*ondatra.Port{a.ate.Port(t, "port4")}, "port3")
	defer a.flapinterface(t, "port3", true)
	// TODO: add checks for NHs when AFT OC schema concludes how viability should be indicated.
}

// createFlow returns a flow from atePort1 to the dstPfx
func (a *testArgs) createFlow(name string) *ondatra.Flow {
	srcEndPoint := a.top.Interfaces()[atePort1.Name]
	dstEndPoint := []ondatra.Endpoint{}
	for intf, intfData := range a.top.Interfaces() {
		if intf != "atePort1" {
			dstEndPoint = append(dstEndPoint, intfData)
		}
	}
	hdr := ondatra.NewIPv4Header()
	hdr.WithSrcAddress(dutPort1.IPv4).DstAddressRange().WithMin(dstPfxMin).WithMax(dstPfxMax).WithCount(routeCount)

	flow := a.ate.Traffic().NewFlow(name).
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint...).
		WithHeaders(ondatra.NewEthernetHeader(), hdr).WithFrameSize(300).WithFrameRateFPS(fps)

	return flow
}

// validateTrafficFlows verifies that the flow on ATE and check interface counters on DUT
func (a *testArgs) validateTrafficFlows(t *testing.T, flow *ondatra.Flow, expected_outgoing_port []*ondatra.Port, shut_ports ...string) {
	a.ate.Traffic().Start(t, flow)
	//Shutdown interface if provided while traffic is flowing and validate traffic
	time.Sleep(30 * time.Second)
	for _, port := range shut_ports {
		a.flapinterface(t, port, false)
		gnmi.Await(t, a.dut, gnmi.OC().Interface(a.dut.Port(t, port).Name()).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_DOWN)
	}
	time.Sleep(30 * time.Second)
	a.ate.Traffic().Stop(t)

	// Get send traffic
	incoming_traffic_counters := gnmi.OC().Interface(a.ate.Port(t, "port1").Name()).Counters()
	sentPkts := gnmi.Get(t, a.ate, incoming_traffic_counters.OutPkts().State())

	var receivedPkts uint64

	// Get traffic received on primary outgoing interface before interface shutdown
	for _, port := range shut_ports {
		outgoing_traffic_counters := gnmi.OC().Interface(a.ate.Port(t, port).Name()).Counters()
		outPkts := gnmi.Get(t, a.ate, outgoing_traffic_counters.InPkts().State())
		receivedPkts = receivedPkts + outPkts
	}

	// Get traffic received on expected port after interface shut
	for _, outPort := range expected_outgoing_port {
		outgoing_traffic_counters := gnmi.OC().Interface(outPort.Name()).Counters()
		outPkts := gnmi.Get(t, a.ate, outgoing_traffic_counters.InPkts().State())
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
	ateP := a.ate.Port(t, port)
	a.ate.Actions().NewSetPortState().WithPort(ateP).WithEnabled(action).Send(t)
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
	aftNHG := gnmi.Get(t, a.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(a.dut)).Afts().NextHopGroup(nhg).State())
	if len(aftNHG.NextHop) == 0 && aftNHG.BackupNextHopGroup == nil {
		t.Fatalf("Prefix %s references a NHG that has neither NH or backup NHG", prefix)
	}
}
