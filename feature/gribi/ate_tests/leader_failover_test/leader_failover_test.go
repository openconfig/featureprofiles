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

package leader_failover_test

import (
	"context"
	"testing"
	"time"

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
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
	}
)

// configInterfaceDUT configures the DUT interfaces.
func configInterfaceDUT(i *oc.Interface, a *attrs.Attributes) *oc.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if *deviations.InterfaceEnabled {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if *deviations.InterfaceEnabled {
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
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutPort1))

	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutPort2))
}

// configureATE configures port1 and port2 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	top := ate.Topology().New()

	p1 := ate.Port(t, "port1")
	i1 := top.AddInterface(atePort1.Name).WithPort(p1)
	i1.IPv4().
		WithAddress(atePort1.IPv4CIDR()).
		WithDefaultGateway(dutPort1.IPv4)

	p2 := ate.Port(t, "port2")
	i2 := top.AddInterface(atePort2.Name).WithPort(p2)
	i2.IPv4().
		WithAddress(atePort2.IPv4CIDR()).
		WithDefaultGateway(dutPort2.IPv4)

	return top
}

// testTraffic generates traffic flow from source network to
// destination network via srcEndPoint to dstEndPoint and checks for
// packet loss. The boolean flag wantLoss could be used to check
// either for 100% loss (when set to true) or 0% loss (when set to false).
func testTraffic(t *testing.T, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoint, dstEndPoint *ondatra.Interface, wantLoss bool) {
	ethHeader := ondatra.NewEthernetHeader()
	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.DstAddressRange().
		WithMin("203.0.113.1").
		WithMax("203.0.113.250").
		WithCount(250)

	flow := ate.Traffic().NewFlow("Flow").
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint).
		WithHeaders(ethHeader, ipv4Header)

	ate.Traffic().Start(t, flow)
	time.Sleep(15 * time.Second)
	ate.Traffic().Stop(t)

	time.Sleep(time.Minute)

	flowPath := gnmi.OC().Flow(flow.Name())

	if !wantLoss {
		if got := gnmi.Get(t, ate, flowPath.LossPct().State()); got != 0 {
			t.Errorf("FAIL: LossPct for flow named %s got %g, want 0", flow.Name(), got)
		} else {
			t.Logf("LossPct for flow named %s got %g, want 0", flow.Name(), got)
		}
	} else {
		if got := gnmi.Get(t, ate, flowPath.LossPct().State()); got != 100 {
			t.Errorf("FAIL: LossPct for flow named %s got %g, want 100", flow.Name(), got)
		} else {
			t.Logf("LossPct for flow named %s got %g, want 100", flow.Name(), got)
		}
	}
}

// testArgs holds the objects needed by the test case.
type testArgs struct {
	ctx context.Context
	dut *ondatra.DUTDevice
	ate *ondatra.ATEDevice
	top *ondatra.ATETopology
}

// addRoute programs an IPv4 route entry through the given gRIBI client
// after programming the necessary nexthop and nexthop-group.
func addRoute(ctx context.Context, t *testing.T, args *testArgs, clientA *gribi.Client) {
	t.Logf("Add an IPv4Entry for %s pointing to ATE port-2 via clientA", ateDstNetCIDR)
	clientA.AddNH(t, nhIndex, atePort2.IPv4, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)
	clientA.AddNHG(t, nhgIndex, map[uint64]uint64{nhIndex: 1}, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)
	clientA.AddIPv4(t, ateDstNetCIDR, nhgIndex, *deviations.DefaultNetworkInstance, "", fluent.InstalledInRIB)
}

// verifyAFT verifies through AFT Telemetry if a route is present on the DUT.
func verifyAFT(ctx context.Context, t *testing.T, args *testArgs) {
	t.Logf("Verify through AFT Telemetry that %s is active", ateDstNetCIDR)
	ipv4Path := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().Ipv4Entry(ateDstNetCIDR)
	if got, ok := gnmi.Watch(t, args.dut, ipv4Path.Prefix().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
		prefix, present := val.Val()
		return present && prefix == ateDstNetCIDR
	}).Await(t); !ok {
		t.Errorf("ipv4-entry/state/prefix got %v, want %s", got, ateDstNetCIDR)
	}
}

// verifyNoAFT verifies through AFT Telemetry that a route is NOT present on the DUT.
func verifyNoAFT(ctx context.Context, t *testing.T, args *testArgs) {
	t.Logf("Verify through Telemetry that the route to %s is not present", ateDstNetCIDR)
	ipv4Path := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().Ipv4Entry(ateDstNetCIDR)
	if got, ok := gnmi.Watch(t, args.dut, ipv4Path.Prefix().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
		prefix, present := val.Val()
		return !present || (present && prefix == "")
	}).Await(t); !ok {
		t.Errorf("ipv4-entry/state/prefix got %s, want nil", got)
	}
}

// verifyTraffic verifies that traffic flows through the DUT without any loss.
func verifyTraffic(ctx context.Context, t *testing.T, args *testArgs) {
	t.Logf("Verify by running traffic that %s is active", ateDstNetCIDR)
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()[atePort2.Name]
	testTraffic(t, args.ate, args.top, srcEndPoint, dstEndPoint, false)
}

// verifyNoTraffic verifies that traffic is completely dropped at the DUT and incurs a 100% loss.
func verifyNoTraffic(ctx context.Context, t *testing.T, args *testArgs) {
	t.Logf("Verify by running traffic that the route to %s is not present", ateDstNetCIDR)
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()[atePort2.Name]
	testTraffic(t, args.ate, args.top, srcEndPoint, dstEndPoint, true)
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
	top.Push(t).StartProtocols(t)

	t.Logf("Time check: %s", time.Since(start))

	args := &testArgs{
		ctx: ctx,
		dut: dut,
		ate: ate,
		top: top,
	}

	t.Run("SINGLE_PRIMARY/PERSISTENCE=DELETE", func(t *testing.T) {
		// This is an indicator test for gRIBI persistence DELETE, so we
		// do not skip based on *deviations.GRIBIPreserveOnly.

		// Set parameters for gRIBI client clientA.
		// Set Persistence to false.
		clientA := &gribi.Client{
			DUT:         dut,
			FIBACK:      false,
			Persistence: false,
		}

		defer clientA.Close(t)

		t.Log("Establish gRIBI client connection with PERSISTENCE set to FALSE/DELETE")
		if err := clientA.Start(t); err != nil {
			t.Fatalf("gRIBI Connection for clientA could not be established")
		}
		clientA.BecomeLeader(t)

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
		t.Log("Close gRIBI client connection")
	})

	t.Run("ShouldDelete", func(t *testing.T) {
		// This is an indicator test for gRIBI persistence DELETE, so we
		// do not skip based on *deviations.GRIBIPreserveOnly.

		t.Logf("Verify through Telemetry and Traffic that the route to %s has been deleted after gRIBI client disconnected", ateDstNetCIDR)

		t.Run("VerifyNoAFT", func(t *testing.T) {
			verifyNoAFT(ctx, t, args)
		})

		t.Run("VerifyNoTraffic", func(t *testing.T) {
			verifyNoTraffic(ctx, t, args)
		})

		t.Logf("Time check: %s", time.Since(start))

	})

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

		// Flush all gRIBI routes.
		defer clientA.FlushAll(t)

		t.Run("DeleteRoute", func(t *testing.T) {
			t.Logf("Delete route to %s and verify through Telemetry and Traffic", ateDstNetCIDR)
			clientA.DeleteIPv4(t, ateDstNetCIDR, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)

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
