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

package dut_daemon_failure_test

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	gnps "github.com/openconfig/gnoi/system"
	grps "github.com/openconfig/gribi/v1/proto/service"
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

	gRIBIDaemons = map[ondatra.Vendor]string{
		ondatra.ARISTA:  "Gribi",
		ondatra.CISCO:   "emsd",
		ondatra.JUNIPER: "rpd",
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

// startTraffic generates traffic flow from source network to
// destination network via srcEndPoint to dstEndPoint.
// Returns the flow object that it creates.
func startTraffic(t *testing.T, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoint, dstEndPoint *ondatra.Interface) *ondatra.Flow {
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

	return flow
}

// stopAndVerifyTraffic stops traffic on the ATE
// and checks for packet loss for the given flow.
func stopAndVerifyTraffic(t *testing.T, ate *ondatra.ATEDevice, flow *ondatra.Flow) {

	ate.Traffic().Stop(t)

	time.Sleep(time.Minute)

	flowPath := gnmi.OC().Flow(flow.Name())
	if got := gnmi.Get(t, ate, flowPath.LossPct().State()); got != 0 {
		t.Errorf("FAIL: LossPct for flow named %s got %g, want 0", flow.Name(), got)
	} else {
		t.Logf("LossPct for flow named %s got %g, want 0", flow.Name(), got)
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

// verifyTraffic verifies that traffic flows through the DUT without any loss.
func verifyTraffic(ctx context.Context, t *testing.T, args *testArgs) {
	t.Logf("Verify by running traffic that %s is active", ateDstNetCIDR)
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()[atePort2.Name]

	flow := startTraffic(t, args.ate, args.top, srcEndPoint, dstEndPoint)
	t.Logf("Wait for 15 seconds")
	time.Sleep(15 * time.Second)
	stopAndVerifyTraffic(t, args.ate, flow)

}

// verifyGRIBIGet verifies through gRIBI Get RPC if a route is present on the DUT.
func verifyGRIBIGet(ctx context.Context, t *testing.T, clientA *gribi.Client) {
	t.Logf("Verify through gRIBI Get RPC that %s is present", ateDstNetCIDR)
	getResponse, err := clientA.Fluent(t).Get().WithNetworkInstance(*deviations.DefaultNetworkInstance).WithAFT(fluent.IPv4).Send()
	if err != nil {
		t.Errorf("Cannot Get: %v", err)
	}
	entries := getResponse.GetEntry()
	var found bool
	for _, entry := range entries {
		v := entry.Entry.(*grps.AFTEntry_Ipv4)
		if prefix := v.Ipv4.GetPrefix(); prefix != "" {
			if prefix == ateDstNetCIDR {
				found = true
				t.Logf("Found route to %s in gRIBI Get response", ateDstNetCIDR)
				break
			}
		}
	}
	if !found {
		t.Errorf("Route to %s NOT found in gRIBI Get response", ateDstNetCIDR)
	}
}

// gNOIKillProcess kills a daemon on the DUT, given its name and pid.
func gNOIKillProcess(ctx context.Context, t *testing.T, args *testArgs, pName string, pID uint32) {
	gnoiClient := args.dut.RawAPIs().GNOI().Default(t)
	killRequest := &gnps.KillProcessRequest{Name: pName, Pid: pID, Signal: gnps.KillProcessRequest_SIGNAL_TERM, Restart: true}
	killResponse, err := gnoiClient.System().KillProcess(context.Background(), killRequest)
	t.Logf("Got kill process response: %v\n\n", killResponse)
	if err != nil {
		t.Fatalf("Failed to execute gNOI Kill Process, error received: %v", err)
	}
}

// findProcessByName uses telemetry to find out the PID of a process
func findProcessByName(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, pName string) uint64 {
	pList := gnmi.GetAll(t, dut, gnmi.OC().System().ProcessAny().State())
	var pID uint64
	for _, proc := range pList {
		if proc.GetName() == pName {
			pID = proc.GetPid()
			t.Logf("Pid of daemon '%s' is '%d'", pName, pID)
		}
	}
	return pID
}

func TestDUTDaemonFailure(t *testing.T) {

	start := time.Now()
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	var flow *ondatra.Flow

	// Check if vendor specific gRIBI daemon name has been added to gRIBIDaemons var
	if _, ok := gRIBIDaemons[dut.Vendor()]; !ok {
		t.Fatalf("Please add support for vendor %v in var gRIBIDaemons", dut.Vendor())
	}

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

	t.Run("SetupGRIBIConnection", func(t *testing.T) {

		// Set parameters for gRIBI client clientA.
		// Set Persistence to true.
		clientA := &gribi.Client{
			DUT:         dut,
			FIBACK:      false,
			Persistence: true,
		}

		t.Log("Establish gRIBI client connection")
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

	})

	t.Run("RestartTraffic", func(t *testing.T) {

		srcEndPoint := top.Interfaces()[atePort1.Name]
		dstEndPoint := top.Interfaces()[atePort2.Name]

		flow = startTraffic(t, args.ate, args.top, srcEndPoint, dstEndPoint)

		t.Logf("Wait for 15 seconds")
		time.Sleep(15 * time.Second)

	})

	t.Run("KillGRIBIDaemon", func(t *testing.T) {

		// Find the PID of gRIBI Daemon.
		var pId uint64
		pName := gRIBIDaemons[dut.Vendor()]
		t.Run("FindGRIBIDaemonPid", func(t *testing.T) {

			pId = findProcessByName(ctx, t, dut, pName)
			if pId == 0 {
				t.Fatalf("Couldn't find pid of gRIBI daemon '%s'", pName)
			} else {
				t.Logf("Pid of gRIBI daemon '%s' is '%d'", pName, pId)
			}
		})

		// Kill gRIBI daemon through gNOI Kill Request.
		t.Run("ExecuteGnoiKill", func(t *testing.T) {
			// TODO - pid type is uint64 in oc-system model, but uint32 in gNOI Kill Request proto.
			// Until the models are brought in line, typecasting the uint64 to uint32.
			gNOIKillProcess(ctx, t, args, pName, uint32(pId))

			// Wait for a bit for gRIBI daemon on the DUT to restart.
			time.Sleep(30 * time.Second)

		})

		t.Logf("Time check: %s", time.Since(start))

		t.Run("VerifyAFT", func(t *testing.T) {
			verifyAFT(ctx, t, args)
		})

		t.Run("VerifyTrafficContinuesToFlow", func(t *testing.T) {
			stopAndVerifyTraffic(t, args.ate, flow)
		})

	})

	t.Run("Re-establishGribiConnection", func(t *testing.T) {

		// Set parameters for gRIBI client clientA.
		// Set Persistence to true.
		clientA := &gribi.Client{
			DUT:         dut,
			FIBACK:      false,
			Persistence: true,
		}

		// Flush all entries after test.
		defer clientA.FlushAll(t)

		t.Log("Re-establish gRIBI client connection")
		if err := clientA.Start(t); err != nil {
			t.Fatalf("gRIBI Connection for clientA could not be re-established")
		}

		t.Run("VerifyGRIBIGet", func(t *testing.T) {
			verifyGRIBIGet(ctx, t, clientA)
		})
	})

	t.Logf("Test run time: %s", time.Since(start))
}
