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

// next hop groups are honored for next hop groups containing a single next hop.
package backup_nhg_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
)

type testArgs struct {
	ate    *ondatra.ATEDevice
	ateTop gosnappi.Config
	ctx    context.Context
	dut    *ondatra.DUTDevice
	client *gribi.Client
}

const (
	// Destination prefix for DUT to ATE traffic.
	dstPfx           = "198.51.100.0"
	vrfB             = "VRF-B"
	nh1ID            = 1
	nh2ID            = 2
	nh100ID          = 100
	nh101ID          = 101
	nhg1ID           = 1
	nhg2ID           = 2
	nhg100ID         = 100
	nhg101ID         = 101
	nhip             = "192.0.2.254"
	mask             = "32"
	policyID         = "match-ipip"
	ipOverIPProtocol = 4
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT Port 1",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT Port 2",
		IPv4:    "192.0.2.5",
		IPv4Len: 30,
	}
	dutPort3 = attrs.Attributes{
		Desc:    "DUT Port 3",
		IPv4:    "192.0.2.9",
		IPv4Len: 30,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		Desc:    "ATE Port 1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: 30,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		Desc:    "ATE Port 2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv4Len: 30,
	}
	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		Desc:    "ATE Port 3",
		MAC:     "02:00:03:01:01:01",
		IPv4:    "192.0.2.10",
		IPv4Len: 30,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestDirectBackupNexthopGroup validates a prefix directly linked to a next hop
// group with a backup uses the backup when the primary next hop becomes
// impaired.
//
// Setup Steps
//   - Connect DUT port 1 <> ATE port 1.
//   - Connect DUT port 2 <> ATE port 2.
//   - Connect DUT port 3 <> ATE port 3.
//   - Create prefix 198.51.100.0/32, next hop group, and next hop to forward to
//     ATE port 2 with GRIBI.
//   - Assign backup next hop group to forward to ATE port 3 with GRIBI, as in readme.
//
// Validation Steps
//   - Verify AFT telemetry shows ATE port 2 selected.
//   - Verify traffic flows to ATE port 2 and not ATE port 3.
//   - After each impairment, verify traffic flows to ATE port 3 and not ATE
//     port 2.
//
// Impairments
//   - Interface ATE port-2 is disabled.
//   - Interface DUT port-2 is disabled.
//   - Delete ipv4 entry 192.0.2.254/32
func TestDirectBackupNexthopGroup(t *testing.T) {
	ctx := context.Background()

	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)
	configureNetworkInstance(t, dut)
	// For interface configuration, prefer config Vrf first then the IP address
	if deviations.InterfaceConfigVRFBeforeAddress(dut) {
		configureDUT(t, dut)
	}

	if deviations.BackupNHGRequiresVrfWithDecap(dut) {
		d := &oc.Root{}
		ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
		pf := ni.GetOrCreatePolicyForwarding()
		fp1 := pf.GetOrCreatePolicy(policyID)
		fp1.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)
		fp1.GetOrCreateRule(1).GetOrCreateIpv4().Protocol = oc.UnionUint8(ipOverIPProtocol)
		fp1.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String(vrfB)
		p1 := dut.Port(t, "port1")
		intf := pf.GetOrCreateInterface(p1.Name())
		intf.ApplyVrfSelectionPolicy = ygot.String(policyID)
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Config(), pf)
	}

	ate := ondatra.ATE(t, "ate")
	ateTop := configureATE(t, ate)

	client := gribi.Client{
		DUT:         dut,
		FIBACK:      true,
		Persistence: true,
	}

	defer client.Close(t)
	defer client.FlushAll(t)
	if err := client.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}

	client.BecomeLeader(t)
	client.FlushAll(t)

	tcArgs := &testArgs{
		ate:    ate,
		ateTop: ateTop,
		ctx:    ctx,
		dut:    dut,
		client: &client,
	}

	tcArgs.configureBackupNextHopGroup(t, false)

	baselineFlow := tcArgs.createFlow("Baseline_Path_Flow", ateTop, &atePort2)
	backupFlow := tcArgs.createFlow("Backup_Path_Flow", ateTop, &atePort3)
	backupIPIPFlow := tcArgs.createIPIPFlow("Backup IP Over IP Path Flow", ateTop, &atePort3)
	tcArgs.ate.OTG().PushConfig(t, ateTop)
	tcArgs.ate.OTG().StartProtocols(t)

	cases := []struct {
		desc               string
		applyImpairmentFn  func()
		removeImpairmentFn func()
	}{
		{
			desc: "Disable ATE port-2",
			applyImpairmentFn: func() {
				ateP2 := ate.Port(t, "port2")
				dutP2 := dut.Port(t, "port2")
				if deviations.ATEPortLinkStateOperationsUnsupported(tcArgs.ate) {
					gnmi.Replace(t, dut, gnmi.OC().Interface(dutP2.Name()).Enabled().Config(), false)
					gnmi.Await(t, dut, gnmi.OC().Interface(dutP2.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_DOWN)
				} else {
					portStateAction := gosnappi.NewControlState()
					portStateAction.Port().Link().SetPortNames([]string{ateP2.ID()}).SetState(gosnappi.StatePortLinkState.DOWN)
					tcArgs.ate.OTG().SetControlState(t, portStateAction)
				}
			},
			removeImpairmentFn: func() {
				ateP2 := ate.Port(t, "port2")
				dutP2 := dut.Port(t, "port2")
				if deviations.ATEPortLinkStateOperationsUnsupported(tcArgs.ate) {
					gnmi.Replace(t, dut, gnmi.OC().Interface(dutP2.Name()).Enabled().Config(), true)
					gnmi.Await(t, dut, gnmi.OC().Interface(dutP2.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)
				} else {
					portStateAction := gosnappi.NewControlState()
					portStateAction.Port().Link().SetPortNames([]string{ateP2.ID()}).SetState(gosnappi.StatePortLinkState.UP)
					tcArgs.ate.OTG().SetControlState(t, portStateAction)
					otgutils.WaitForARP(t, ate.OTG(), tcArgs.ateTop, "IPv4")
				}
			},
		},
		{
			desc: "Disable DUT port-2",
			applyImpairmentFn: func() {
				dutP2 := dut.Port(t, "port2")
				gnmi.Replace(t, dut, gnmi.OC().Interface(dutP2.Name()).Enabled().Config(), false)
				gnmi.Await(t, dut, gnmi.OC().Interface(dutP2.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_DOWN)
			},
			removeImpairmentFn: func() {
				dutP2 := dut.Port(t, "port2")
				gnmi.Replace(t, dut, gnmi.OC().Interface(dutP2.Name()).Enabled().Config(), true)
				gnmi.Await(t, dut, gnmi.OC().Interface(dutP2.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)
			},
		},
		{
			desc: "Delete nh ipv4 entry",
			applyImpairmentFn: func() {
				client.DeleteIPv4(t, nhip+"/"+mask, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
			},
			removeImpairmentFn: func() {
				client.AddIPv4(t, nhip+"/"+mask, nhg1ID, deviations.DefaultNetworkInstance(dut), deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Run("Validate Baseline AFT Telemetry", func(t *testing.T) {
				tcArgs.validateAftTelemetry(t, deviations.DefaultNetworkInstance(dut), nhip, atePort2.IPv4, atePort2.IPv4)
				tcArgs.validateAftTelemetry(t, deviations.DefaultNetworkInstance(dut), dstPfx, nhip, atePort2.IPv4)
				tcArgs.validateAftTelemetry(t, vrfB, dstPfx, atePort3.IPv4, atePort3.IPv4)
			})

			t.Run("Validate Baseline Traffic Delivery", func(t *testing.T) {
				tcArgs.validateTrafficFlows(t, ate, ateTop, baselineFlow, backupFlow)
			})

			tc.applyImpairmentFn()
			defer tc.removeImpairmentFn()

			t.Run("Validate Backup Path Traffic Delivery", func(t *testing.T) {
				if deviations.BackupNHGRequiresVrfWithDecap(dut) {
					tcArgs.validateTrafficFlows(t, ate, ateTop, backupIPIPFlow, baselineFlow)
				} else {
					tcArgs.validateTrafficFlows(t, ate, ateTop, backupFlow, baselineFlow)
				}
			})
		})
	}
}

// configureATE configures port1-3 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	top := gosnappi.NewConfig()

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	p3 := ate.Port(t, "port3")

	atePort1.AddToOTG(top, p1, &dutPort1)
	atePort2.AddToOTG(top, p2, &dutPort2)
	atePort3.AddToOTG(top, p3, &dutPort3)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")

	return top
}

// configureDUT configures port1-3 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")

	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
	gnmi.Replace(t, dut, d.Interface(p3.Name()).Config(), dutPort3.NewOCInterface(p3.Name(), dut))

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
		fptest.SetPortSpeed(t, p3)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p3.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

// configureBackupNextHopGroup creates gribi nexthops, nexthop
// groups, and prefixes for evaluating backup next hop forwarding
// entry.
func (a *testArgs) configureBackupNextHopGroup(t *testing.T, del bool) {
	t.Logf("Adding NH %d with atePort2 via gRIBI", nh1ID)
	nh1, op1 := gribi.NHEntry(nh1ID, atePort2.IPv4, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
	t.Logf("Adding NH %d with atePort3 and NHGs %d, %d via gRIBI", nh2ID, nhg1ID, nhg2ID)
	nh2, op2 := gribi.NHEntry(nh2ID, atePort3.IPv4, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
	nhg1, op3 := gribi.NHGEntry(nhg1ID, map[uint64]uint64{nh1ID: 100}, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
	nhg2, op4 := gribi.NHGEntry(nhg2ID, map[uint64]uint64{nh2ID: 100}, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
	a.client.AddEntries(t, []fluent.GRIBIEntry{nh1, nh2, nhg1, nhg2}, []*client.OpResult{op1, op2, op3, op4})
	t.Logf("Adding an IPv4Entry for %s via gRIBI", nhip)
	a.client.AddIPv4(t, nhip+"/"+mask, nhg1ID, deviations.DefaultNetworkInstance(a.dut), deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
	t.Logf("Adding NH %d in VRF-B via gRIBI", nh100ID)
	nh100, op5 := gribi.NHEntry(nh100ID, "VRFOnly", deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: vrfB})
	if deviations.BackupNHGRequiresVrfWithDecap(a.dut) {
		nh100, op5 = gribi.NHEntry(nh100ID, "Decap", deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: vrfB})
	}
	t.Logf("Adding NH %d and NHGs %d, %d via gRIBI", nh101ID, nhg100ID, nhg101ID)
	nh101, op6 := gribi.NHEntry(nh101ID, nhip, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
	nhg100, op7 := gribi.NHGEntry(nhg100ID, map[uint64]uint64{nh100ID: 100}, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
	nhg101, op8 := gribi.NHGEntry(nhg101ID, map[uint64]uint64{nh101ID: 100}, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: nhg100ID})
	a.client.AddEntries(t, []fluent.GRIBIEntry{nh100, nh101, nhg100, nhg101}, []*client.OpResult{op5, op6, op7, op8})
	t.Logf("Adding IPv4Entries for %s for DEFAULT and VRF-B via gRIBI", dstPfx)
	a.client.AddIPv4(t, dstPfx+"/"+mask, nhg101ID, deviations.DefaultNetworkInstance(a.dut), deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
	a.client.AddIPv4(t, dstPfx+"/"+mask, nhg2ID, vrfB, deviations.DefaultNetworkInstance(a.dut), fluent.InstalledInFIB)
}

// configureNetworkInstance configures vrf-B.
func configureNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {
	c := &oc.Root{}
	ni := c.GetOrCreateNetworkInstance(vrfB)
	ni.Description = ygot.String("Non Default routing instance VRF-B created for testing")
	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrfB).Config(), ni)
}

// createFlow returns a flow from atePort1 to the dstPfx, expected to arrive on ATE interface dst.
func (a *testArgs) createFlow(name string, ateTop gosnappi.Config, dst *attrs.Attributes) string {

	flowipv4 := ateTop.Flows().Add().SetName(name)
	flowipv4.Metrics().SetEnable(true)
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(atePort1.MAC)
	flowipv4.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{dst.Name + ".IPv4"})
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(atePort1.IPv4)
	v4.Dst().SetValue(dstPfx)

	return flowipv4.Name()
}

// createIPIPFlow returns a flow from atePort1 to the dstPfx, expected to arrive on ATE interface dst.
func (a *testArgs) createIPIPFlow(name string, ateTop gosnappi.Config, dst *attrs.Attributes) string {

	modName := strings.Replace(name, " ", "_", -1)
	flowipv4 := ateTop.Flows().Add().SetName(modName)
	flowipv4.Metrics().SetEnable(true)
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(atePort1.MAC)
	flowipv4.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{dst.Name + ".IPv4"})
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(atePort1.IPv4)
	v4.Dst().SetValue(dstPfx)

	v4 = flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(atePort1.IPv4)
	v4.Dst().SetValue(dstPfx)

	return modName
}

// validateAftTelmetry verifies aft telemetry entries.
func (a *testArgs) validateAftTelemetry(t *testing.T, vrfName, prefix, ipAddress, resolvedNhIPAddress string) {
	aftPfxPath := gnmi.OC().NetworkInstance(vrfName).Afts().Ipv4Entry(prefix + "/" + mask)
	aftPfxVal, found := gnmi.Watch(t, a.dut, aftPfxPath.State(), 2*time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		value, present := val.Val()
		return present && value.GetNextHopGroup() != 0
	}).Await(t)
	if !found {
		t.Fatalf("Could not find prefix %s in telemetry AFT", prefix+"/"+mask)
	}
	aftPfx, _ := aftPfxVal.Val()

	aftNHG := gnmi.Get(t, a.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(a.dut)).Afts().NextHopGroup(aftPfx.GetNextHopGroup()).State())
	if got := len(aftNHG.NextHop); got != 1 {
		t.Fatalf("Prefix %s next-hop entry count: got %d, want 1", prefix+"/"+mask, got)
	}

	for k := range aftNHG.NextHop {
		aftnh := gnmi.Get(t, a.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(a.dut)).Afts().NextHop(k).State())
		// Handle the cases where the device returns the indirect NH or the recursively resolved NH.
		// For e.g. in case of a->b->c, device should return either b or c.
		if got := aftnh.GetIpAddress(); got != ipAddress && got != resolvedNhIPAddress {
			t.Fatalf("Prefix %s next-hop IP: got %s, want %s or %s", prefix+"/"+mask, got, ipAddress, resolvedNhIPAddress)
		}
	}
}

// validateTrafficFlows verifies that the good flow delivers traffic and the
// bad flow does not deliver traffic.
func (a *testArgs) validateTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, good, bad string) {

	ate.OTG().StartTraffic(t)
	time.Sleep(15 * time.Second)
	ate.OTG().StopTraffic(t)

	otgutils.LogFlowMetrics(t, ate.OTG(), config)
	otgutils.LogPortMetrics(t, ate.OTG(), config)
	if got := getLossPct(t, ate, good); got > 0 {
		t.Errorf("LossPct for flow %s: got %v, want 0", good, got)
	}
	if got := getLossPct(t, ate, bad); got < 100 {
		t.Errorf("LossPct for flow %s: got %v, want 100", bad, got)
	}

}

// getLossPct returns the loss percentage for a given flow
func getLossPct(t *testing.T, ate *ondatra.ATEDevice, flowName string) float32 {
	t.Helper()
	recvMetric := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).State())
	txPackets := float32(recvMetric.GetCounters().GetOutPkts())
	rxPackets := float32(recvMetric.GetCounters().GetInPkts())
	lostPackets := txPackets - rxPackets
	if txPackets == 0 {
		t.Fatalf("Tx packets should be higher than 0 for flow %s", flowName)
	}
	lossPct := lostPackets * 100 / txPackets
	return lossPct
}
