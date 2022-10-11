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
	"github.com/openconfig/ondatra/telemetry"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
)

const (
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126
	dstPfx        = "203.0.113.0/24"
	dstPfxMin     = "203.0.113.0"
	dstPfxMask    = "24"
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

// configureDUT configures port1, port2, port3 and port4 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := dut.Config()

	p1 := dut.Port(t, "port1")
	d.Interface(p1.Name()).Replace(t, dutPort1.NewInterface(p1.Name()))

	p2 := dut.Port(t, "port2")
	d.Interface(p2.Name()).Replace(t, dutPort2.NewInterface(p2.Name()))

	p3 := dut.Port(t, "port3")
	d.Interface(p3.Name()).Replace(t, dutPort3.NewInterface(p3.Name()))

	p4 := dut.Port(t, "port4")
	d.Interface(p4.Name()).Replace(t, dutPort4.NewInterface(p4.Name()))
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
			DUT:                  dut,
			FibACK:               false,
			Persistence:          true,
			InitialElectionIDLow: 10,
		}
		defer client.Close(t)
		if err := client.Start(t); err != nil {
			t.Fatalf("gRIBI Connection can not be established")
		}

		// Flush past entries before running the tc
		client.Flush(t)

		tcArgs := &testArgs{
			ctx:    ctx,
			dut:    dut,
			client: &client,
			ate:    ate,
			top:    top,
		}
		testIPv4BackUpSwitch(ctx, t, tcArgs)
	})
}

// testIPv4BackUpSwitch Ensure that backup NHGs are honoured with NextHopGroup entries containing >1 NH
//
// Setup Steps
//   - Connect ATE port-1 to DUT port-1.
//   - Connect ATE port-2 to DUT port-2.
//   - Connect ATE port-3 to DUT port-3.
//   - Connect ATE port-4 to DUT port-4.
//   - Connect a gRIBI client to the DUT and inject an IPv4Entry for 203.0.113.0/24 pointing to a NextHopGroup containing:
//   - Two primary next-hops:
//   - 2: to ATE port-2
//   - 3: to ATE port-3.
//   - A backup NHG containing a single next-hop:
//   - 4: to ATE port-4.
//   - Ensure that traffic forwarded to a destination in 203.0.113.0/24 is received at ATE port-2 and port-3.
//   - Disable ATE port-2. Ensure that traffic for a destination in 203.0.113.0/24 is received at ATE port-3.
//   - Disable ATE port-3. Ensure that traffic for a destination in 203.0.113.0/24 is received at ATE port-4.
//
// Validation Steps
//   - Verify AFT telemetry after shutting each port
//   - Verify traffic switches to the right ports
func testIPv4BackUpSwitch(ctx context.Context, t *testing.T, args *testArgs) {

	const (
		// Next hop group adjacency identifier.
		NHGID = 100
		// Backup next hop group ID that the dstPfx will forward to.
		BackupNHGID = 200

		NH1ID, NH2ID, NH3ID = 1001, 1002, 1003
	)
	t.Logf("Program a backup pointing to ATE port-4 via gRIBI")
	args.client.AddNH(t, NH3ID, atePort4.IPv4, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)
	args.client.AddNHG(t, BackupNHGID, map[uint64]uint64{NH3ID: 10}, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)

	t.Logf("an IPv4Entry for %s pointing to ATE port-2 and port-3 via gRIBI", dstPfx)
	args.client.AddNH(t, NH1ID, atePort2.IPv4, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)
	args.client.AddNH(t, NH2ID, atePort3.IPv4, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)
	args.client.AddNHG(t, NHGID, map[uint64]uint64{NH1ID: 80, NH2ID: 20}, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB, &gribi.NHGOptions{BackupNHG: BackupNHGID})
	args.client.AddIPv4(t, dstPfx, NHGID, *deviations.DefaultNetworkInstance, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)

	// create flow
	BaseFlow := createFlow(t, args.ate, args.top, "BaseFlow")

	// validate programming using AFT
	aftCheck(t, args.dut, dstPfx, []string{"192.0.2.6", "192.0.2.10"})
	// Validate traffic over primary path port2, port3
	validateTrafficFlows(t, args.ate, BaseFlow, false, []string{"port2", "port3"})

	//shutdown port2
	flapinterface(t, args.ate, "port2", false)
	defer flapinterface(t, args.ate, "port2", true)
	// validate programming using AFT
	aftCheck(t, args.dut, dstPfx, []string{"192.0.2.10"})
	// Validate traffic over primary path port3
	validateTrafficFlows(t, args.ate, BaseFlow, false, []string{"port3"})

	//shutdown port3
	flapinterface(t, args.ate, "port3", false)
	defer flapinterface(t, args.ate, "port3", true)
	// validate programming using AFT
	aftCheck(t, args.dut, dstPfx, []string{"192.0.2.14"})
	// validate traffic over backup
	validateTrafficFlows(t, args.ate, BaseFlow, false, []string{"port4"})
}

// createFlow returns a flow from atePort1 to the dstPfx
func createFlow(t *testing.T, ate *ondatra.ATEDevice, top *ondatra.ATETopology, name string) *ondatra.Flow {
	srcEndPoint := top.Interfaces()[atePort1.Name]
	dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range top.Interfaces() {
		if intf != "atePort1" {
			dstEndPoint = append(dstEndPoint, intf_data)
		}
	}
	hdr := ondatra.NewIPv4Header()
	hdr.WithSrcAddress(dutPort1.IPv4).DstAddressRange().WithMin(dstPfxMin).WithCount(1)

	flow := ate.Traffic().NewFlow(name).
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint...).
		WithHeaders(ondatra.NewEthernetHeader(), hdr).WithFrameSize(300)

	return flow
}

// validateTrafficFlows verifies that the flow on ATE and check interface counters on DUT
func validateTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, flow *ondatra.Flow, drop bool, d_port []string) {
	ate.Traffic().Start(t, flow)
	time.Sleep(60 * time.Second)
	ate.Traffic().Stop(t)
	flowPath := ate.Telemetry().Flow(flow.Name())
	got := flowPath.LossPct().Get(t)
	if drop {
		if got != 100 {
			t.Fatalf("Traffic passing for flow %s got %f, want 100 percent loss", flow.Name(), got)
		}
	} else {
		if got > 0 {
			t.Fatalf("LossPct for flow %s got %f, want 0", flow.Name(), got)
		}
	}
}

// flapinterface shut/unshut interface, action true bringsup the interface and false brings it down
func flapinterface(t *testing.T, ate *ondatra.ATEDevice, port string, action bool) {
	ateP := ate.Port(t, port)
	ate.Actions().NewSetPortState().WithPort(ateP).WithEnabled(action).Send(t)
}

// aftCheck does ipv4, NHG and NH aft check
func aftCheck(t testing.TB, dut *ondatra.DUTDevice, prefix string, expectedNH []string) {
	// check prefix and get NHG ID
	aftPfxNHG := dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().Ipv4Entry(prefix).NextHopGroup()
	aftPfxNHGVal, found := aftPfxNHG.Watch(t, 10*time.Second, func(val *telemetry.QualifiedUint64) bool {
		return true
	}).Await(t)
	if !found {
		t.Fatalf("Could not find prefix %s in telemetry AFT", dstPfx)
	}

	// using NHG ID validate NH
	aftNHG := dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().NextHopGroup(aftPfxNHGVal.Val(t)).Get(t)
	if got := len(aftNHG.NextHop); got < 1 && aftNHG.BackupNextHopGroup == nil {
		t.Fatalf("Prefix %s reachability didn't switch to backup path", prefix)
	}
	if len(aftNHG.NextHop) != 0 {
		for k := range aftNHG.NextHop {
			aftnh := dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Afts().NextHop(k).Get(t)
			total_ips := len(expectedNH)
			for _, ip := range expectedNH {
				if ip == aftnh.GetIpAddress() {
					break
				}
				total_ips -= 1
			}
			if total_ips == 0 {
				t.Fatalf("No matching NH found")
			}
		}
	}
}
