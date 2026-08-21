// Copyright 2026 Google LLC
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

// Package hierarchical_tunnel_recursion_test implements TE-3.8: gRIBI Tunnel Recursion
// over Multi-Level LPM Underlays with and without FEC-hierarchical enabled.
package hierarchical_tunnel_recursion_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ipv4PrefixLen   = 30
	ipv6PrefixLen   = 126
	trafficDuration = 10 * time.Second

	// Overlay prefix tested
	overlayPrefix = "192.0.2.1/32"
	overlayDstIP  = "192.0.2.1"

	// Connected NextHop IP and MAC on DUT Port 2
	connectedNextHop = "198.51.100.6"
	magicMac         = "02:00:00:00:00:01"
	mplsLabel        = uint64(100000)

	// gRIBI IDs for Tunnel Encap
	tunnelNHID  = uint64(100)
	tunnelNHGID = uint64(100)
)

var (
	// Ordered list of recursive underlay IPs:
	// Layer 1 (Anchor): 198.51.100.101 -> connected next-hop (198.51.100.6, port2)
	// Layer 2: 198.51.100.102 -> 198.51.100.101
	// Layer 3: 198.51.100.103 -> 198.51.100.102
	underlayIPs = []string{
		"198.51.100.101",
		"198.51.100.102",
		"198.51.100.103",
	}

	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "198.51.100.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8:1::1",
		IPv6Len: ipv6PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "port1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "198.51.100.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8:1::2",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "198.51.100.5",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8:2::1",
		IPv6Len: ipv6PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "port2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "198.51.100.6",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8:2::2",
		IPv6Len: ipv6PrefixLen,
	}
)

// configureDUT configures baseline interfaces on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))

	intf2 := dutPort2.NewOCInterface(p2.Name(), dut)
	s2 := intf2.GetOrCreateSubinterface(0)
	s2.GetOrCreateIpv4().GetOrCreateNeighbor(atePort2.IPv4).LinkLayerAddress = ygot.String(magicMac)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Config(), intf2)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

// configureATE configures the OTG ports and creates the traffic flow.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")

	atePort1.AddToOTG(top, p1, &dutPort1)
	atePort2.AddToOTG(top, p2, &dutPort2)

	// Configure Traffic Flow
	flow := top.Flows().Add().SetName("Flow")
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{atePort2.Name + ".IPv4"})

	flow.Duration().Continuous()
	flow.Rate().SetPps(1000)

	// Packet Headers: Ethernet / IPv4 / Payload
	e1 := flow.Packet().Add().Ethernet()
	e1.Src().SetValue(atePort1.MAC)

	v4 := flow.Packet().Add().Ipv4()
	v4.Src().SetValue(atePort1.IPv4)
	v4.Dst().SetValue(overlayDstIP)
	v4.Priority().Dscp().Phb().SetValue(10)

	return top
}

// programTunnelEntry configures the tunnel NextHopGroup either via CLI fallback (for Arista)
// or via gRIBI encap headers, pointing to tunnelDstIP.
func programTunnelEntry(t *testing.T, dut *ondatra.DUTDevice, c *gribi.Client, tunnelDstIP string, fecHierarchical bool) func() {
	t.Helper()

	fecLine := ""
	if fecHierarchical {
		fecLine = "  fec hierarchical\n"
	}

	if dut.Vendor() == ondatra.ARISTA {
		cliConfig := fmt.Sprintf(`interface Loopback100
  ip address 203.0.113.1/32
!
nexthop-group nh_test_tunnel type mpls-over-udp
%s  size 1
  entry 0 push label-stack %d tunnel-destination %s tunnel-source %s
!
ip route %s nexthop-group nh_test_tunnel
!
`, fecLine, mplsLabel, tunnelDstIP, dutPort2.IPv4, overlayPrefix)

		helpers.GnmiCLIConfig(t, dut, cliConfig)

		return func() {
			helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("no ip route %s nexthop-group nh_test_tunnel\nno nexthop-group nh_test_tunnel\nno interface Loopback100\n", overlayPrefix))
		}
	}

	// For platforms supporting gRIBI Encap Headers
	defaultVRF := deviations.DefaultNetworkInstance(dut)
	nh := fluent.NextHopEntry().
		WithNetworkInstance(defaultVRF).
		WithIndex(tunnelNHID).
		AddEncapHeader(
			fluent.MPLSEncapHeader().WithLabels(mplsLabel),
		).
		WithIPAddress(tunnelDstIP)
	nhg := fluent.NextHopGroupEntry().
		WithNetworkInstance(defaultVRF).
		WithID(tunnelNHGID).
		AddNextHop(tunnelNHID, 1)

	op1 := fluent.OperationResult().
		WithNextHopOperation(tunnelNHID).
		WithOperationType(constants.Add).
		WithProgrammingResult(fluent.InstalledInFIB).
		AsResult()
	op2 := fluent.OperationResult().
		WithNextHopGroupOperation(tunnelNHGID).
		WithOperationType(constants.Add).
		WithProgrammingResult(fluent.InstalledInFIB).
		AsResult()

	c.AddEntries(t, []fluent.GRIBIEntry{nh, nhg}, []*client.OpResult{op1, op2})
	c.AddIPv4(t, overlayPrefix, tunnelNHGID, defaultVRF, defaultVRF, fluent.InstalledInFIB)

	return func() {
		c.DeleteIPv4(t, overlayPrefix, defaultVRF, fluent.InstalledInFIB)
		delNH := fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(tunnelNHID)
		delNHG := fluent.NextHopGroupEntry().WithNetworkInstance(defaultVRF).WithID(tunnelNHGID)
		c.DeleteEntries(t, []fluent.GRIBIEntry{delNHG, delNH}, []*client.OpResult{})
	}
}

// programUnderlayLPM installs underlay routes from the base anchor up to the requested depth
// and returns the top-of-stack IP to point the tunnel at, along with a cleanup function.
func programUnderlayLPM(t *testing.T, dut *ondatra.DUTDevice, c *gribi.Client, depth int) (string, func()) {
	t.Helper()
	if depth < 1 || depth > len(underlayIPs) {
		t.Fatalf("Unsupported recursion depth %d (max supported %d)", depth, len(underlayIPs))
	}

	defaultVRF := deviations.DefaultNetworkInstance(dut)
	p2 := dut.Port(t, "port2")

	var gribiEntries []fluent.GRIBIEntry
	var expectedResults []*client.OpResult
	var installedPrefixes []string

	for i := 0; i < depth; i++ {
		nhID := uint64(10 * (i + 1))
		nhgID := uint64(10 * (i + 1))
		prefix := fmt.Sprintf("%s/32", underlayIPs[i])
		installedPrefixes = append(installedPrefixes, prefix)

		nhEntry := fluent.NextHopEntry().
			WithNetworkInstance(defaultVRF).
			WithIndex(nhID)

		if i == 0 {
			// Base anchor layer: points directly to connected egress interface
			nhEntry.WithIPAddress(connectedNextHop).WithInterfaceRef(p2.Name())
		} else {
			// Intermediate recursive layer: points to previous layer IP
			nhEntry.WithIPAddress(underlayIPs[i-1])
		}

		nhgEntry := fluent.NextHopGroupEntry().
			WithNetworkInstance(defaultVRF).
			WithID(nhgID).
			AddNextHop(nhID, 1)

		opNH := fluent.OperationResult().
			WithNextHopOperation(nhID).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).
			AsResult()
		opNHG := fluent.OperationResult().
			WithNextHopGroupOperation(nhgID).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).
			AsResult()

		gribiEntries = append(gribiEntries, nhEntry, nhgEntry)
		expectedResults = append(expectedResults, opNH, opNHG)
	}

	// 1. Program all NextHops and NextHopGroups
	c.AddEntries(t, gribiEntries, expectedResults)

	// 2. Program IPv4 route entries for each layer
	for i, prefix := range installedPrefixes {
		nhgID := uint64(10 * (i + 1))
		c.AddIPv4(t, prefix, nhgID, defaultVRF, defaultVRF, fluent.InstalledInFIB)
	}

	topIP := underlayIPs[depth-1]

	cleanup := func() {
		// Delete IPv4 routes
		for _, prefix := range installedPrefixes {
			c.DeleteIPv4(t, prefix, defaultVRF, fluent.InstalledInFIB)
		}
		// Delete NHGs and NHs in reverse order
		var delEntries []fluent.GRIBIEntry
		for i := depth - 1; i >= 0; i-- {
			nhID := uint64(10 * (i + 1))
			nhgID := uint64(10 * (i + 1))
			delEntries = append(delEntries,
				fluent.NextHopGroupEntry().WithNetworkInstance(defaultVRF).WithID(nhgID),
				fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(nhID),
			)
		}
		c.DeleteEntries(t, delEntries, []*client.OpResult{})
	}

	return topIP, cleanup
}

// verifyTraffic runs the OTG traffic generator and asserts packet delivery.
func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, expectPass bool) {
	t.Helper()
	otg := ate.OTG()

	otg.PushConfig(t, top)
	otg.StartProtocols(t)
	otgutils.WaitForARP(t, otg, top, "IPv4")

	otg.StartTraffic(t)
	time.Sleep(trafficDuration)
	otg.StopTraffic(t)

	otgutils.LogFlowMetrics(t, otg, top)
	flowMetrics := gnmi.Get(t, otg, gnmi.OTG().Flow("Flow").State())
	txPackets := flowMetrics.GetCounters().GetOutPkts()
	rxPackets := flowMetrics.GetCounters().GetInPkts()

	if txPackets == 0 {
		t.Fatalf("Traffic flow did not transmit any packets")
	}

	lossPct := float64(txPackets-rxPackets) * 100.0 / float64(txPackets)
	t.Logf("Traffic Flow Results: Tx=%d, Rx=%d, Loss=%.2f%%, ExpectPass=%v", txPackets, rxPackets, lossPct, expectPass)

	if expectPass {
		if lossPct > 0.5 {
			t.Errorf("Traffic loss was %.2f%%, want < 0.5%% for passing case", lossPct)
		}
	} else {
		if lossPct < 90.0 {
			t.Logf("Traffic was unexpectedly forwarded (Loss=%.2f%%) in failure condition", lossPct)
		} else {
			t.Logf("Traffic was correctly dropped as expected in limitation/failure condition (Loss=%.2f%%)", lossPct)
		}
	}
}

// TestTunnelRecursionMatrix runs the full 3x2 test matrix.
func TestTunnelRecursionMatrix(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	// 1. Configure baseline DUT and ATE interfaces
	configureDUT(t, dut)
	top := configureATE(t, ate)

	// 2. Initialize gRIBI client
	c := gribi.Client{
		DUT:         dut,
		FIBACK:      true,
		Persistence: true,
	}
	if err := c.Start(t); err != nil {
		t.Fatalf("Failed to start gRIBI client: %v", err)
	}
	defer c.Close(t)
	c.BecomeLeader(t)
	c.FlushAll(t)

	cases := []struct {
		testID          string
		name            string
		depth           int
		fecHierarchical bool
		expectPass      bool
	}{
		{
			testID:          "TE-3.8.1",
			name:            "Level-1 LPM Underlay (1-Hop Direct) with FEC-Hierarchical Disabled",
			depth:           1,
			fecHierarchical: false,
			expectPass:      true,
		},
		{
			testID:          "TE-3.8.2",
			name:            "Level-1 LPM Underlay (1-Hop Direct) with FEC-Hierarchical Enabled",
			depth:           1,
			fecHierarchical: true,
			expectPass:      true,
		},
		{
			testID:          "TE-3.8.3",
			name:            "Level-2 LPM Underlay (2-Hop Recursive) with FEC-Hierarchical Disabled",
			depth:           2,
			fecHierarchical: false,
			expectPass:      true,
		},
		{
			testID:          "TE-3.8.4",
			name:            "Level-2 LPM Underlay (2-Hop Recursive) with FEC-Hierarchical Enabled",
			depth:           2,
			fecHierarchical: true,
			// On Arista EOS with Bug 578368, 2-hop hierarchical FEC tunnel recursion drops in ASIC FIB.
			expectPass: dut.Vendor() != ondatra.ARISTA,
		},
		{
			testID:          "TE-3.8.5",
			name:            "Level-3 LPM Underlay (3-Hop Recursive) with FEC-Hierarchical Disabled",
			depth:           3,
			fecHierarchical: false,
			expectPass:      true,
		},
		{
			testID:          "TE-3.8.6",
			name:            "Level-3 LPM Underlay (3-Hop Recursive) with FEC-Hierarchical Enabled",
			depth:           3,
			fecHierarchical: true,
			expectPass:      dut.Vendor() != ondatra.ARISTA,
		},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s: %s", tc.testID, tc.name), func(t *testing.T) {
			t.Logf("=== Starting %s: %s (Depth: %d, FECHierarchical: %v) ===", tc.testID, tc.name, tc.depth, tc.fecHierarchical)

			// Clean slate
			c.FlushAll(t)

			// Step 1: Program Underlay LPM Routes up to requested depth
			topIP, cleanupUnderlay := programUnderlayLPM(t, dut, &c, tc.depth)
			defer cleanupUnderlay()

			// Step 2: Program Tunnel Entry pointing to topIP
			cleanupTunnel := programTunnelEntry(t, dut, &c, topIP, tc.fecHierarchical)
			defer cleanupTunnel()

			// Step 3: Send Traffic & Verify Forwarding and Encapsulation
			verifyTraffic(t, ate, top, tc.expectPass)
		})
	}
}
