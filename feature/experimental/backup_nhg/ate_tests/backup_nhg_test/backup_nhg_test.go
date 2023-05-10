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

type testArgs struct {
	ate    *ondatra.ATEDevice
	ateTop *ondatra.ATETopology
	ctx    context.Context
	dut    *ondatra.DUTDevice
	client *gribi.Client
}

const (
	// Destination prefix for DUT to ATE traffic.
	dstPfx   = "198.51.100.0"
	vrfA     = "VRF-A"
	vrfB     = "VRF-B"
	nh1ID    = 1
	nh2ID    = 2
	nh100ID  = 100
	nh101ID  = 101
	nhg1ID   = 1
	nhg2ID   = 2
	nhg100ID = 100
	nhg101ID = 101
	nhip     = "192.0.2.254"
	mask     = "32"
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
		IPv4:    "192.0.2.2",
		IPv4Len: 30,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		Desc:    "ATE Port 2",
		IPv4:    "192.0.2.6",
		IPv4Len: 30,
	}
	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		Desc:    "ATE Port 3",
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
//   - Delete ipv4 entry 192.0.2.254/32.
func TestDirectBackupNexthopGroup(t *testing.T) {
	ctx := context.Background()

	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)
	configureNetworkInstance(t, dut)

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

	baselineFlow := tcArgs.createFlow("Baseline Path Flow", &atePort2)
	backupFlow := tcArgs.createFlow("Backup Path Flow", &atePort3)

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
				ate.Actions().NewSetPortState().WithPort(ateP2).WithEnabled(false).Send(t)
				gnmi.Await(t, dut, gnmi.OC().Interface(dutP2.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_DOWN)
			},
			removeImpairmentFn: func() {
				ateP2 := ate.Port(t, "port2")
				dutP2 := dut.Port(t, "port2")
				ate.Actions().NewSetPortState().WithPort(ateP2).WithEnabled(true).Send(t)
				gnmi.Await(t, dut, gnmi.OC().Interface(dutP2.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)
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
				client.DeleteIPv4(t, nhip+"/"+mask, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
			},
			removeImpairmentFn: func() {
				client.AddIPv4(t, nhip+"/"+mask, nhg1ID, *deviations.DefaultNetworkInstance, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)

			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Run("Validate Baseline AFT Telemetry", func(t *testing.T) {
				tcArgs.validateAftTelemetry(t, *deviations.DefaultNetworkInstance, nhip, atePort2.IPv4, atePort2.IPv4)
				tcArgs.validateAftTelemetry(t, vrfA, dstPfx, nhip, atePort2.IPv4)
				tcArgs.validateAftTelemetry(t, vrfB, dstPfx, atePort3.IPv4, atePort3.IPv4)
			})

			t.Run("Validate Baseline Traffic Delivery", func(t *testing.T) {
				tcArgs.validateTrafficFlows(t, baselineFlow, backupFlow)
			})

			tc.applyImpairmentFn()
			defer tc.removeImpairmentFn()

			t.Run("Validate Backup Path Traffic Delivery", func(t *testing.T) {
				tcArgs.validateTrafficFlows(t, backupFlow, baselineFlow)
			})
		})
	}
}

// configreATE configures port1-3 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	top := ate.Topology().New()

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	p3 := ate.Port(t, "port3")

	atePort1.AddToATE(top, p1, &dutPort1)
	atePort2.AddToATE(top, p2, &dutPort2)
	atePort3.AddToATE(top, p3, &dutPort3)

	top.Push(t).StartProtocols(t)

	return top
}

// configureDUT configures port1-3 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")

	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name()))
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name()))
	gnmi.Replace(t, dut, d.Interface(p3.Name()).Config(), dutPort3.NewOCInterface(p3.Name()))

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
		fptest.SetPortSpeed(t, p3)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), *deviations.DefaultNetworkInstance, 0)
		fptest.AssignToNetworkInstance(t, dut, p3.Name(), *deviations.DefaultNetworkInstance, 0)
	}

}

// configureBackupNextHopGroup creates gribi nexthops, nexthop
// groups, and prefixes for evaluating backup next hop forwarding
// entry.
func (a *testArgs) configureBackupNextHopGroup(t *testing.T, del bool) {
	t.Logf("Adding NH %d with atePort2 via gRIBI", nh1ID)
	a.client.AddNH(t, nh1ID, atePort2.IPv4, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	t.Logf("Adding NH %d with atePort3 and NHGs %d, %d via gRIBI", nh2ID, nhg1ID, nhg2ID)
	a.client.AddNH(t, nh2ID, atePort3.IPv4, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	a.client.AddNHG(t, nhg1ID, map[uint64]uint64{nh1ID: 100}, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	a.client.AddNHG(t, nhg2ID, map[uint64]uint64{nh2ID: 100}, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	t.Logf("Adding an IPv4Entry for %s via gRIBI", nhip)
	a.client.AddIPv4(t, nhip+"/"+mask, nhg1ID, *deviations.DefaultNetworkInstance, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	t.Logf("Adding NH %d in VRF-B via gRIBI", nh100ID)
	a.client.AddNH(t, nh100ID, "VRFOnly", *deviations.DefaultNetworkInstance, fluent.InstalledInFIB, &gribi.NHOptions{VrfName: vrfB})
	t.Logf("Adding NH %d and NHGs %d, %d via gRIBI", nh101ID, nhg100ID, nhg101ID)
	a.client.AddNH(t, nh101ID, nhip, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	a.client.AddNHG(t, nhg100ID, map[uint64]uint64{nh100ID: 100}, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	a.client.AddNHG(t, nhg101ID, map[uint64]uint64{nh101ID: 100}, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: nhg100ID})
	t.Logf("Adding IPv4Entries for %s for VRF-A and VRF-B via gRIBI", dstPfx)
	a.client.AddIPv4(t, dstPfx+"/"+mask, nhg101ID, vrfA, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	a.client.AddIPv4(t, dstPfx+"/"+mask, nhg2ID, vrfB, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)

}

// configureNetworkInstance configures vrf VRF-A and adds the vrf to port1, and configures vrf-B.
func configureNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {
	c := &oc.Root{}
	ni := c.GetOrCreateNetworkInstance(vrfA)
	ni.Description = ygot.String("Non Default routing instance VRF-A created for testing")
	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	p1 := dut.Port(t, "port1")
	niIntf := ni.GetOrCreateInterface(p1.Name())
	niIntf.Subinterface = ygot.Uint32(0)
	niIntf.Interface = ygot.String(p1.Name())
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrfA).Config(), ni)

	ni1 := c.GetOrCreateNetworkInstance(vrfB)
	ni1.Description = ygot.String("Non Default routing instance VRF-B created for testing")
	ni1.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrfB).Config(), ni1)

	if deviations.ExplicitGRIBIUnderNetworkInstance(dut) {
		fptest.EnableGRIBIUnderNetworkInstance(t, dut, *deviations.DefaultNetworkInstance)
		fptest.EnableGRIBIUnderNetworkInstance(t, dut, vrfA)
		fptest.EnableGRIBIUnderNetworkInstance(t, dut, vrfB)
	}
}

// createFlow returns a flow from atePort1 to the dstPfx, expected to arrive on ATE interface dst.
func (a *testArgs) createFlow(name string, dst *attrs.Attributes) *ondatra.Flow {
	hdr := ondatra.NewIPv4Header()
	hdr.WithSrcAddress(dutPort1.IPv4).DstAddressRange().WithMin(dstPfx).WithCount(1)

	flow := a.ate.Traffic().NewFlow(name).
		WithSrcEndpoints(a.ateTop.Interfaces()[atePort1.Name]).
		WithDstEndpoints(a.ateTop.Interfaces()[dst.Name]).
		WithHeaders(ondatra.NewEthernetHeader(), hdr)

	return flow
}

// validateAftTelmetry verifies aft telemetry entries.
func (a *testArgs) validateAftTelemetry(t *testing.T, vrfName, prefix, ipAddress, resolvedNhIpAddress string) {
	aftPfxNHG := gnmi.OC().NetworkInstance(vrfName).Afts().Ipv4Entry(prefix + "/" + mask).NextHopGroup()
	aftPfxNHGVal, found := gnmi.Watch(t, a.dut, aftPfxNHG.State(), 2*time.Minute, func(val *ygnmi.Value[uint64]) bool {
		if val.IsPresent() {
			value, _ := val.Val()
			if value != 0 {
				return true
			}
		}
		return false
	}).Await(t)
	if !found {
		t.Fatalf("Could not find prefix %s in telemetry AFT", prefix+"/"+mask)
	}
	nhg, _ := aftPfxNHGVal.Val()

	aftNHG := gnmi.Get(t, a.dut, gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().NextHopGroup(nhg).State())
	if got := len(aftNHG.NextHop); got != 1 {
		t.Fatalf("Prefix %s next-hop entry count: got %d, want 1", prefix+"/"+mask, got)
	}

	for k := range aftNHG.NextHop {
		aftnh := gnmi.Get(t, a.dut, gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().NextHop(k).State())
		// Handle the cases where the device returns the indirect NH or the recursively resolved NH.
		// For e.g. in case of a->b->c, device should return either b or c.
		if got := aftnh.GetIpAddress(); got != ipAddress && got != resolvedNhIpAddress {
			t.Fatalf("Prefix %s next-hop IP: got %s, want %s or %s", prefix+"/"+mask, got, ipAddress, resolvedNhIpAddress)
		}
	}
}

// validateTrafficFlows verifies that the good flow delivers traffic and the
// bad flow does not deliver traffic.
func (a *testArgs) validateTrafficFlows(t *testing.T, good *ondatra.Flow, bad *ondatra.Flow) {
	a.ate.Traffic().Start(t, good, bad)
	time.Sleep(15 * time.Second)
	a.ate.Traffic().Stop(t)

	val, _ := gnmi.Watch(t, a.ate, gnmi.OC().Flow(good.Name()).LossPct().State(), 5*time.Minute, func(val *ygnmi.Value[float32]) bool {
		return val.IsPresent()
	}).Await(t)
	lossPct, present := val.Val()
	if !present {
		t.Fatalf("Could not read loss percentage for flow %q from ATE.", good.Name())
	}
	if lossPct > 0 {
		t.Fatalf("LossPct for flow %s got %f, want 0", good.Name(), lossPct)
	}

	val, _ = gnmi.Watch(t, a.ate, gnmi.OC().Flow(bad.Name()).LossPct().State(), 5*time.Minute, func(val *ygnmi.Value[float32]) bool {
		return val.IsPresent()
	}).Await(t)
	lossPct, present = val.Val()
	if !present {
		t.Fatalf("Could not read loss percentage for flow %q from ATE.", bad.Name())
	}
	if lossPct < 100 {
		t.Fatalf("LossPct for flow %s got %f, want 100", bad.Name(), lossPct)
	}
}
