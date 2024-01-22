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

package persistence_mode_test

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
// The testbed consists of:
//   ate:port1 -> dut:port1 and
//   dut:port2 -> ate:port2
//
//   * ate:port1 -> dut:port1 subnet 192.0.2.0/30
//   * ate:port2 -> dut:port2 subnet 192.0.2.4/30
//
//   * Destination network: -> 203.0.113.0/24

const (
	ipv4PrefixLen = 30
	ateDstNetCIDR = "203.0.113.0/24"
	nhIndex       = 1
	nhgIndex      = 42
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
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
		Name:    "atePort2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
	}
)

// configInterfaceDUT configures the DUT interfaces.
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

// configureDUT configures port1 and port2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutPort1, dut))

	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutPort2, dut))

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

// configureATE configures port1 and port2 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	top := gosnappi.NewConfig()

	top.Ports().Add().SetName(ate.Port(t, "port1").ID())
	i1 := top.Devices().Add().SetName(ate.Port(t, "port1").ID())
	eth1 := i1.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	eth1.Connection().SetPortName(i1.Name())
	eth1.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4").
		SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).
		SetPrefix(uint32(atePort1.IPv4Len))

	top.Ports().Add().SetName(ate.Port(t, "port2").ID())
	i2 := top.Devices().Add().SetName(ate.Port(t, "port2").ID())
	eth2 := i2.Ethernets().Add().SetName(atePort2.Name + ".Eth").SetMac(atePort2.MAC)
	eth2.Connection().SetPortName(i2.Name())
	eth2.Ipv4Addresses().Add().SetName(atePort2.Name + ".IPv4").
		SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).
		SetPrefix(uint32(atePort2.IPv4Len))

	return top
}

// testTraffic generates traffic flow from source network to
// destination network via srcEndPoint to dstEndPoint and checks for
// packet loss. The boolean flag wantLoss could be used to check
// either for 100% loss (when set to true) or 0% loss (when set to false).
func testTraffic(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, srcEndPoint, dstEndPoint attrs.Attributes, wantLoss bool) {

	otg := ate.OTG()
	otg.StartProtocols(t)
	otgutils.WaitForARP(t, otg, top, "IPv4")
	dstMac := gnmi.Get(t, otg, gnmi.OTG().Interface(atePort1.Name+".Eth").Ipv4Neighbor(dutPort1.IPv4).LinkLayerAddress().State())
	top.Flows().Clear().Items()
	flowipv4 := top.Flows().Add().SetName("Flow")
	flowipv4.Metrics().SetEnable(true)
	flowipv4.TxRx().Port().
		SetTxName(ate.Port(t, "port1").ID()).
		SetRxName(ate.Port(t, "port2").ID())
	flowipv4.Duration().Continuous()
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(atePort1.MAC)
	e1.Dst().SetValue(dstMac)
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(atePort1.IPv4)
	v4.Dst().Increment().SetStart("203.0.113.1").SetCount(250)
	otg.PushConfig(t, top)
	otg.StartProtocols(t)

	otg.StartTraffic(t)
	time.Sleep(15 * time.Second)
	t.Logf("Stop traffic")
	otg.StopTraffic(t)
	otgutils.LogFlowMetrics(t, otg, top)

	time.Sleep(time.Minute)
	txPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(flowipv4.Name()).Counters().OutPkts().State()))
	rxPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(flowipv4.Name()).Counters().InPkts().State()))

	if txPkts == 0 {
		t.Fatalf("TxPkts == 0, want > 0")
	}

	if !wantLoss {
		if got := (txPkts - rxPkts) * 100 / txPkts; got != 0 {
			t.Errorf("FAIL: LossPct for flow named %s got %v, want 0", flowipv4.Name(), got)
		} else {
			t.Logf("LossPct for flow named %s got %v, want 0", flowipv4.Name(), got)
		}
	} else {
		if got := (txPkts - rxPkts) * 100 / txPkts; got != 100 {
			t.Errorf("FAIL: LossPct for flow named %s got %v, want 100", flowipv4.Name(), got)
		} else {
			t.Logf("LossPct for flow named %s got %v, want 100", flowipv4.Name(), got)
		}
	}
}

// testArgs holds the objects needed by the test case.
type testArgs struct {
	ctx context.Context
	dut *ondatra.DUTDevice
	ate *ondatra.ATEDevice
	top gosnappi.Config
}

// addRoute programs an IPv4 route entry through the given gRIBI client
// after programming the necessary nexthop and nexthop-group.
func addRoute(ctx context.Context, t *testing.T, args *testArgs, clientA *gribi.Client) {
	t.Logf("Add an IPv4Entry for %s pointing to ATE port-2 via clientA", ateDstNetCIDR)
	clientA.AddNH(t, nhIndex, atePort2.IPv4, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInRIB)
	clientA.AddNHG(t, nhgIndex, map[uint64]uint64{nhIndex: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInRIB)
	clientA.AddIPv4(t, ateDstNetCIDR, nhgIndex, deviations.DefaultNetworkInstance(args.dut), "", fluent.InstalledInRIB)
}

// verifyAFT verifies through AFT Telemetry if a route is present on the DUT.
func verifyAFT(ctx context.Context, t *testing.T, args *testArgs) {
	t.Logf("Verify through AFT Telemetry that %s is active", ateDstNetCIDR)
	ipv4Path := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Afts().Ipv4Entry(ateDstNetCIDR)
	if got, ok := gnmi.Watch(t, args.dut, ipv4Path.State(), time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		ipv4Entry, present := val.Val()
		return present && ipv4Entry.GetPrefix() == ateDstNetCIDR
	}).Await(t); !ok {
		t.Errorf("ipv4-entry/state/prefix got %v, want %s", got, ateDstNetCIDR)
	}
}

// verifyNoAFT verifies through AFT Telemetry that a route is NOT present on the DUT.
func verifyNoAFT(ctx context.Context, t *testing.T, args *testArgs) {
	t.Logf("Verify through Telemetry that the route to %s is not present", ateDstNetCIDR)
	ipv4Path := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Afts().Ipv4Entry(ateDstNetCIDR)
	if got, ok := gnmi.Watch(t, args.dut, ipv4Path.State(), time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		ipv4Entry, present := val.Val()
		return !present || (present && ipv4Entry.Prefix == nil)
	}).Await(t); !ok {
		t.Errorf("ipv4-entry/state/prefix got %v, want nil", got)
	}
}

// verifyTraffic verifies that traffic flows through the DUT without any loss.
func verifyTraffic(ctx context.Context, t *testing.T, args *testArgs) {
	t.Logf("Verify by running traffic that %s is active", ateDstNetCIDR)
	testTraffic(t, args.ate, args.top, atePort1, atePort2, false)
}

// verifyNoTraffic verifies that traffic is completely dropped at the DUT and incurs a 100% loss.
func verifyNoTraffic(ctx context.Context, t *testing.T, args *testArgs) {
	t.Logf("Verify by running traffic that the route to %s is not present", ateDstNetCIDR)
	testTraffic(t, args.ate, args.top, atePort1, atePort2, true)
}

func TestLeaderFailover(t *testing.T) {

	start := time.Now()
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()

	// Configure the DUT.
	t.Logf("Configure DUT")
	configureDUT(t, dut)

	// Configure the ATE.
	t.Logf("Configure ATE")
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	t.Logf("Time check: %s", time.Since(start))

	args := &testArgs{
		ctx: ctx,
		dut: dut,
		ate: ate,
		top: top,
	}

	t.Run("SINGLE_PRIMARY/PERSISTENCE=PRESERVE", func(t *testing.T) {
		// Set parameters for gRIBI client clientA.
		// Set Persistence to true.
		clientA := &gribi.Client{
			DUT:         args.dut,
			FIBACK:      false,
			Persistence: true,
		}

		t.Log("Reconnect clientA, with PERSISTENCE set to TRUE/PRESERVE")
		if err := clientA.Start(t); err != nil {
			t.Fatalf("gRIBI Connection for clientA could not be re-established")
		}
		clientA.BecomeLeader(t)

		defer clientA.Close(t)

		t.Run("AddRoute", func(t *testing.T) {

			t.Logf("Add gRIBI route to %s and verify through Telemetry and Traffic", ateDstNetCIDR)
			addRoute(ctx, t, args, clientA)

			t.Run("VerifyAFT", func(t *testing.T) {
				verifyAFT(ctx, t, args)
			})

			t.Run("VerifyTraffic", func(t *testing.T) {
				verifyTraffic(ctx, t, args)
			})
		})

		t.Logf("Time check: %s", time.Since(start))

		// Close below is done through defer.
		t.Log("Close gRIBI client connection again")
	})

	t.Run("ShouldPreserve", func(t *testing.T) {
		t.Logf("Verify through Telemetry and Traffic that the route to %s is preserved", ateDstNetCIDR)

		t.Run("VerifyAFT", func(t *testing.T) {
			verifyAFT(ctx, t, args)
		})

		t.Run("VerifyTraffic", func(t *testing.T) {
			verifyTraffic(ctx, t, args)
		})
	})

	t.Run("ReconnectAndDelete", func(t *testing.T) {
		// Set parameters for gRIBI client clientA.
		// Set Persistence to true.
		clientA := &gribi.Client{
			DUT:         args.dut,
			FIBACK:      false,
			Persistence: true,
		}

		t.Log("Reconnect clientA")
		if err := clientA.Start(t); err != nil {
			t.Fatalf("gRIBI Connection for clientA could not be re-established")
		}
		clientA.BecomeLeader(t)

		defer clientA.Close(t)

		// Flush all entries after test.
		defer clientA.FlushAll(t)

		t.Run("DeleteRoute", func(t *testing.T) {
			t.Logf("Delete route to %s and verify through Telemetry and Traffic", ateDstNetCIDR)
			clientA.DeleteIPv4(t, ateDstNetCIDR, deviations.DefaultNetworkInstance(dut), fluent.InstalledInRIB)

			t.Run("VerifyNoAFT", func(t *testing.T) {
				verifyNoAFT(ctx, t, args)
			})

			t.Run("VerifyNoTraffic", func(t *testing.T) {
				verifyNoTraffic(ctx, t, args)
			})
		})
	})

	t.Logf("Test run time: %s", time.Since(start))
}
