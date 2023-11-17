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
	"github.com/openconfig/gribigo/client"
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
	pktDropTolerance = 10
	ipv4PrefixLen    = 30
	ipv6PrefixLen    = 126
	dstPfx           = "203.0.113.1/32"
	dstPfxMin        = "203.0.113.1"
	dstPfxMax        = "203.0.113.254"
	ipOverIPProtocol = 4
	routeCount       = 1
	vrf1             = "vrfA"
	vrf2             = "vrfB"
	fps              = 1000000 // traffic frames per second
	switchovertime   = 250.0   // switchovertime during interface shut in milliseconds
	ethernetCsmacd   = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
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
	top := gosnappi.NewConfig()

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
func configInterfaceDUT(i *oc.Interface, dutPort *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {
	if deviations.InterfaceEnabled(dut) {
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
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutPort1, dut))
	configNetworkInstanceInterface(t, dut, vrf1, p1.Name(), uint32(0))
	// create VRF "vrfB"
	configNetworkInstance(t, dut, vrf2)

	if deviations.BackupNHGRequiresVrfWithDecap(dut) {
		d := &oc.Root{}
		ni := d.GetOrCreateNetworkInstance(vrf1)
		pf := ni.GetOrCreatePolicyForwarding()
		fp1 := pf.GetOrCreatePolicy("match-ipip")
		fp1.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)
		fp1.GetOrCreateRule(1).GetOrCreateIpv4().Protocol = oc.UnionUint8(ipOverIPProtocol)
		fp1.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String(vrf1)
		p1 := dut.Port(t, "port1")
		intf := pf.GetOrCreateInterface(p1.Name())
		intf.ApplyVrfSelectionPolicy = ygot.String("match-ipip")
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrf1).PolicyForwarding().Config(), pf)
	}

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
}

func TestBackup(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	// Configure ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	// configure DUT
	configureDUT(t, dut)

	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

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
	nh3, op1 := gribi.NHEntry(nhid3, "VRFOnly", deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: vrf2})
	if deviations.BackupNHGRequiresVrfWithDecap(a.dut) {
		nh3, op1 = gribi.NHEntry(nhid3, "Decap", deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: vrf2})
	}
	bkupNHG, op2 := gribi.NHGEntry(backupnhgid, map[uint64]uint64{nhid3: 10}, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
	a.client.AddEntries(t, []fluent.GRIBIEntry{nh3, bkupNHG}, []*client.OpResult{op1, op2})

	t.Logf("an IPv4Entry for %s in %s pointing to ATE port-2 and port-3 via gRIBI", dstPfx, vrf1)
	nh1, op3 := gribi.NHEntry(nhid1, atePort2.IPv4, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
	nh2, op4 := gribi.NHEntry(nhid2, atePort3.IPv4, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
	nhg1, op5 := gribi.NHGEntry(nhgid1, map[uint64]uint64{nhid1: 80, nhid2: 20}, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: backupnhgid})
	a.client.AddEntries(t, []fluent.GRIBIEntry{nh1, nh2, nhg1}, []*client.OpResult{op3, op4, op5})
	a.client.AddIPv4(t, dstPfx, nhgid1, vrf1, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)

	t.Logf("an IPv4Entry for %s in %s pointing to ATE port-4 via gRIBI", dstPfx, vrf2)
	nh4, op6 := gribi.NHEntry(nhid4, atePort4.IPv4, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
	nhg2, op7 := gribi.NHGEntry(nhgid2, map[uint64]uint64{nhid4: 100}, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
	a.client.AddEntries(t, []fluent.GRIBIEntry{nh4, nhg2}, []*client.OpResult{op6, op7})
	a.client.AddIPv4(t, dstPfx, nhgid2, vrf2, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)

	// validate programming using AFT
	// TODO: add checks for NHs when AFT OC schema concludes how viability should be indicated.
	a.aftCheck(t, dstPfx, vrf2)

	// create flow
	dstMac := gnmi.Get(t, a.ate.OTG(), gnmi.OTG().Interface(atePort1.Name+".Eth").Ipv4Neighbor(dutPort1.IPv4).LinkLayerAddress().State())
	baseFlow := a.createFlow(t, "baseFlow", dstMac)

	// Validate traffic over primary path port2, port3
	t.Run("Baseline (port2 + port3)", func(t *testing.T) {
		t.Logf("Validate traffic over primary path port2, port3")
		a.validateTrafficFlows(t, baseFlow, []*ondatra.Port{a.ate.Port(t, "port2"), a.ate.Port(t, "port3")})
	})

	// shutdown port2
	t.Run("Baseline (port3 only)", func(t *testing.T) {
		t.Logf("Shutdown port 2 and validate traffic switching over port3 primary path")
		a.validateTrafficFlows(t, baseFlow, []*ondatra.Port{a.ate.Port(t, "port3")}, "port2")
	})

	// TODO: add checks for NHs when AFT OC schema concludes how viability should be indicated.

	// shutdown port3
	t.Run("Backup (port4)", func(t *testing.T) {
		t.Logf("Shutdown port 3 and validate traffic switching over port4 backup path")
		a.validateTrafficFlows(t, baseFlow, []*ondatra.Port{a.ate.Port(t, "port4")}, "port3")
	})
	if deviations.ATEPortLinkStateOperationsUnsupported(a.ate) {
		defer a.flapinterface(t, "port2", true)
		defer a.flapinterface(t, "port3", true)
	} else {
		portStateAction := gosnappi.NewControlState()
		portStateAction.Port().Link().SetPortNames([]string{"port2", "port3"}).SetState(gosnappi.StatePortLinkState.UP)
		defer a.ate.OTG().SetControlState(t, portStateAction)
	}
	// TODO: add checks for NHs when AFT OC schema concludes how viability should be indicated.
}

// createFlow returns a flow from atePort1 to the dstPfx
func (a *testArgs) createFlow(t *testing.T, name, dstMac string) string {

	flow := a.top.Flows().Add().SetName(name)
	flow.Metrics().SetEnable(true)
	flow.Size().SetFixed(300)
	e1 := flow.Packet().Add().Ethernet()
	e1.Src().SetValue(atePort1.MAC)
	flow.TxRx().Port().SetTxName("port1").SetRxNames([]string{"port2", "port3", "port4"})
	flow.Rate().SetPps(fps)
	e1.Dst().SetChoice("value").SetValue(dstMac)
	v4 := flow.Packet().Add().Ipv4()
	v4.Src().Increment().SetStart(dutPort1.IPv4)
	v4.Dst().Increment().SetStart(dstPfxMin).SetCount(routeCount)

	// use ip over ip packets since some vendors only support decap for backup
	v4 = flow.Packet().Add().Ipv4()
	v4.Src().Increment().SetStart(dutPort1.IPv4)
	v4.Dst().Increment().SetStart(dstPfxMin).SetCount(routeCount)

	// StartProtocols required for running on hardware
	a.ate.OTG().PushConfig(t, a.top)
	a.ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, a.ate.OTG(), a.top, "IPv4")
	return name
}

// validateTrafficFlows verifies that the flow on ATE and check interface counters on DUT
func (a *testArgs) validateTrafficFlows(t *testing.T, flow string, outPorts []*ondatra.Port, shutPorts ...string) {
	a.ate.OTG().StartTraffic(t)
	// Shutdown interface if provided while traffic is flowing and validate traffic
	time.Sleep(30 * time.Second)
	for _, port := range shutPorts {
		if deviations.ATEPortLinkStateOperationsUnsupported(a.ate) {
			a.flapinterface(t, port, false)
		} else {
			portStateAction := gosnappi.NewControlState()
			portStateAction.Port().Link().SetPortNames([]string{port}).SetState(gosnappi.StatePortLinkState.DOWN)
			a.ate.OTG().SetControlState(t, portStateAction)
		}
		gnmi.Await(t, a.dut, gnmi.OC().Interface(a.dut.Port(t, port).Name()).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_DOWN)
	}
	time.Sleep(30 * time.Second)
	a.ate.OTG().StopTraffic(t)
	time.Sleep(10 * time.Second)
	otgutils.LogPortMetrics(t, a.ate.OTG(), a.top)
	otgutils.LogFlowMetrics(t, a.ate.OTG(), a.top)

	// Get send and receive traffic
	flowMetrics := gnmi.Get(t, a.ate.OTG(), gnmi.OTG().Flow(flow).State())
	sentPkts := uint64(flowMetrics.GetCounters().GetOutPkts())
	receivedPkts := uint64(flowMetrics.GetCounters().GetInPkts())

	if sentPkts == 0 {
		t.Fatalf("Tx packets should be higher than 0")
	}

	// Check if traffic restores with in expected time in milliseconds during interface shut
	// else if there is no interface trigger, validate received packets (control+data) are more than send packets
	t.Logf("Sent Packets: %v, Received packets: %v", sentPkts, receivedPkts)
	diff := pktDiff(sentPkts, receivedPkts)
	if len(shutPorts) > 0 {
		// Time took for traffic to restore in milliseconds after trigger
		fpm := (diff / (fps / 1000))
		if fpm > switchovertime {
			t.Errorf("Traffic loss %v msecs more than expected %v msecs", fpm, switchovertime)
		}
		t.Logf("Traffic loss during path change : %v msecs", fpm)
	} else if diff > pktDropTolerance {
		t.Error("Traffic didn't switch to the expected outgoing port")
	}
}

func pktDiff(sent, recveived uint64) uint64 {
	if sent > recveived {
		return sent - recveived
	}
	return recveived - sent
}

// flapinterface shut/unshut interface, action true bringsup the interface and false brings it down
func (a *testArgs) flapinterface(t *testing.T, port string, action bool) {
	// Currently, setting the OTG port down has no effect on kne and thus the corresponding dut port will be used
	dutP := a.dut.Port(t, port)
	dc := gnmi.OC()
	i := &oc.Interface{}
	i.Enabled = ygot.Bool(action)
	i.Name = ygot.String(dutP.Name())
	i.Type = ethernetCsmacd
	gnmi.Update(t, a.dut, dc.Interface(dutP.Name()).Config(), i)
}

// aftCheck does ipv4, NHG and NH aft check
// TODO: add checks for NHs when AFT OC schema concludes how viability should be indicated.

func (a *testArgs) aftCheck(t testing.TB, prefix string, instance string) {
	// check prefix and get NHG ID
	aftPfxPath := gnmi.OC().NetworkInstance(instance).Afts().Ipv4Entry(prefix)
	aftPfxVal, found := gnmi.Watch(t, a.dut, aftPfxPath.State(), 2*time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		ipv4Entry, present := val.Val()
		return present && ipv4Entry.NextHopGroup != nil
	}).Await(t)
	if !found {
		t.Fatalf("Could not find prefix %s in telemetry AFT", dstPfx)
	}
	aftPfx, _ := aftPfxVal.Val()

	// using NHG ID validate NH
	aftNHG := gnmi.Get(t, a.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(a.dut)).Afts().NextHopGroup(aftPfx.GetNextHopGroup()).State())
	if len(aftNHG.NextHop) == 0 && aftNHG.BackupNextHopGroup == nil {
		t.Fatalf("Prefix %s references a NHG that has neither NH or backup NHG", prefix)
	}
}
