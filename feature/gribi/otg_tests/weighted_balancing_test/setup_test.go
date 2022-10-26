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

package weighted_balancing_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"regexp"
	"sort"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	otgtelemetry "github.com/openconfig/ondatra/telemetry/otg"
	"github.com/openconfig/ygot/ygot"
)

var (
	randomSrcIP = flag.Bool("random_src_ip", false,
		"Randomize source IP addresses of the generated traffic.")
	randomDstIP = flag.Bool("random_dst_ip", false,
		"Randomize destination IP address of the generated traffic.")
	randomSrcPort = flag.Bool("random_src_port", true,
		"Randomize source ports of the generated traffic.")
	randomDstPort = flag.Bool("random_dst_port", true,
		"Randomize destination ports of the generated traffic.")

	trafficPause = flag.Duration("traffic_pause", 0,
		"Amount of time to pause before sending traffic.")
	trafficDuration = flag.Duration("traffic_duration", 5*time.Second,
		"Duration for sending traffic.")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring next hop group consisting of multiple next
// hops.  IxNetwork flow requires both source and destination networks
// be configured on the ATE.  It is not possible to send packets to
// the ether.
//
// The testbed consists of ate:port1 -> dut:port1 and dut:port{2-9} ->
// ate:port{2-9}.  The first pair is called the "source" pair, and the
// second the "destination" pairs.  The destination pairs form the
// "next hop group."  Each pair is assigned a /30 subnet assigned
// consecutively.
//
//   - Source: ate:port1 192.0.2.2/30 -> dut:port1 192.0.2.1/30
//   - Destination ports{2-9}:
//     dut:port{i+1} 192.0.2.{4*i+1}/30 -> ate:port{i+1} 192.0.2.{4*i+2}/30
//     (e.g. dut:port2 192.0.2.5/30 -> ate:port2 192.0.2.6/30 where i=1)
//
// A traffic flow from a source network is configured to be sent from
// ate:port1, with a destination network expected to be received at
// ate:port{2-9}.
//
//   - Source network: 198.51.100.0/24 (TEST-NET-2)
//   - Destination network: 203.0.113.0/24 (TEST-NET-3)
//
// The DUT is configured via gRIBI to route TEST-NET-2 to TEST-NET-3
// via the next hop group configured in the topology.
const (
	plen = 30

	ateSrcPort       = "ate:port1"
	ateSrcPortMac    = "02:00:01:01:01:01"
	ateSrcNetName    = "srcnet"
	ateSrcNet        = "198.51.100.0"
	ateSrcNetCIDR    = "198.51.100.0/24"
	ateSrcNetFirstIP = "198.51.100.1"
	ateSrcNetCount   = 250

	ateDstFirstPort  = "ate:port2"
	ateDstNetName    = "dstnet"
	ateDstNet        = "203.0.113.0"
	ateDstNetCIDR    = "203.0.113.0/24"
	ateDstNetFirstIP = "203.0.113.1"
	ateDstNetCount   = 250

	discardCIDR = "192.0.2.0/24"
	nhgIndex    = 42
)

var (
	portsIPv4 = map[string]string{
		"dut:port1": "192.0.2.1",
		"ate:port1": "192.0.2.2",

		"dut:port2": "192.0.2.5",
		"ate:port2": "192.0.2.6",

		"dut:port3": "192.0.2.9",
		"ate:port3": "192.0.2.10",

		"dut:port4": "192.0.2.13",
		"ate:port4": "192.0.2.14",

		"dut:port5": "192.0.2.17",
		"ate:port5": "192.0.2.18",

		"dut:port6": "192.0.2.21",
		"ate:port6": "192.0.2.22",

		"dut:port7": "192.0.2.25",
		"ate:port7": "192.0.2.26",

		"dut:port8": "192.0.2.29",
		"ate:port8": "192.0.2.30",

		"dut:port9": "192.0.2.33",
		"ate:port9": "192.0.2.34",
	}
)

// nextHop describes the next hop configuration of a given test case.
type nextHop struct {
	Port string // port name in the portsIPv4 map.

	// gRIBI weight set for the next hop.  If 0, defaults to 1. See:
	// https://github.com/openconfig/autobahn/issues/10
	Weight uint64

	Want float64 // expected traffic distribution (approximate).
}

// dutInterface builds a DUT interface ygot struct for a given port
// according to portsIPv4.  Returns nil if the port has no IP address
// mapping.
func dutInterface(p *ondatra.Port) *telemetry.Interface {
	id := fmt.Sprintf("%s:%s", p.Device().ID(), p.ID())
	i := &telemetry.Interface{
		Name:        ygot.String(p.Name()),
		Description: ygot.String(p.String()),
		Type:        telemetry.IETFInterfaces_InterfaceType_ethernetCsmacd,
	}
	if *deviations.InterfaceEnabled {
		i.Enabled = ygot.Bool(true)
	}

	ipv4, ok := portsIPv4[id]
	if !ok {
		return nil
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if *deviations.InterfaceEnabled {
		s4.Enabled = ygot.Bool(true)
	}

	a := s4.GetOrCreateAddress(ipv4)
	a.PrefixLength = ygot.Uint8(plen)
	return i
}

// configureDUT configures all the interfaces on the DUT.
func configureDUT(t testing.TB, dut *ondatra.DUTDevice) {
	dc := dut.Config()

	// We add a discard route so that when the nexthop interface goes
	// down, the device does not attempt to route packets through the
	// default gateway 0.0.0.0/0.  Packets destined to the more specific
	// next hop CIDRs will be routed.
	static := &telemetry.NetworkInstance_Protocol_Static{
		Prefix: ygot.String(discardCIDR),
	}
	static.GetOrCreateNextHop("AUTO_drop_2").
		NextHop = telemetry.LocalRouting_LOCAL_DEFINED_NEXT_HOP_DROP
	staticp := dc.NetworkInstance(*deviations.DefaultNetworkInstance).
		Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *deviations.StaticProtocolName).
		Static(discardCIDR)
	fptest.LogYgot(t, "discard route", staticp, static)
	staticp.Replace(t, static)

	for _, dp := range dut.Ports() {
		if i := dutInterface(dp); i != nil {
			dc.Interface(dp.Name()).Replace(t, i)
		} else {
			t.Fatalf("No address found for port %v", dp)
		}
	}
}

// setDUTInterfaceState sets the admin state on the dut interface
func setDUTInterfaceState(t testing.TB, dut *ondatra.DUTDevice, p *ondatra.Port, state bool) {
	t.Helper()
	dc := dut.Config()
	i := &telemetry.Interface{Name: ygot.String(p.Name())}
	i.Enabled = ygot.Bool(state)
	dc.Interface(p.Name()).Update(t, i)
}

// configureATE configures the topology of the ATE.
func configureATE(t testing.TB, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	otg := ate.OTG()
	config := otg.NewConfig(t)
	for i, ap := range ate.Ports() {
		// DUT and ATE ports are connected by the same names.
		dutid := fmt.Sprintf("dut:%s", ap.ID())
		ateid := fmt.Sprintf("ate:%s", ap.ID())

		config.Ports().Add().SetName(ap.ID())
		dev := config.Devices().Add().SetName(ateid)
		macAddress, _ := incrementMAC(ateSrcPortMac, i)
		eth := dev.Ethernets().Add().SetName(ateid + ".Eth").
			SetPortName(ap.ID()).SetMac(macAddress)
		eth.Ipv4Addresses().Add().SetName(dev.Name() + ".IPv4").
			SetAddress(portsIPv4[ateid]).SetGateway(portsIPv4[dutid]).
			SetPrefix(plen)
	}
	otg.PushConfig(t, config)
	return config
}

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, c *fluent.GRIBIClient, t testing.TB, timeout time.Duration) error {
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}

// buildNextHops converts the nextHop specification to gRIBI entries
// and wanted OpResult.  The entries are part of the Modify request,
// and the Modify response is verified against the wants.
func buildNextHops(t testing.TB, nexthops []nextHop, scale uint64) (ents []fluent.GRIBIEntry, wants []*client.OpResult) {
	nhgent := fluent.NextHopGroupEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithID(nhgIndex)
	nhgwant := fluent.OperationResult().
		WithOperationID(uint64(len(nexthops) + 1)).
		WithOperationType(constants.Add).
		WithProgrammingResult(fluent.InstalledInRIB).
		WithNextHopGroupOperation(nhgIndex).
		AsResult()

	for i, nh := range nexthops {
		index := uint64(i + 1)
		nhip := portsIPv4[nh.Port]
		t.Logf("Installing gRIBI next hop entry %d to %s (%s) of weight %d",
			index, nhip, nh.Port, nh.Weight*scale)

		ent := fluent.NextHopEntry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithIndex(index).WithIPAddress(nhip)
		ents = append(ents, ent)

		nhgent.AddNextHop(index, nh.Weight*scale)

		want := fluent.OperationResult().
			WithOperationID(index).
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInRIB).
			WithNextHopOperation(index).
			AsResult()
		wants = append(wants, want)
	}

	ipv4ent := fluent.IPv4Entry().WithNetworkInstance(*deviations.DefaultNetworkInstance).
		WithPrefix(ateDstNetCIDR).WithNextHopGroup(42)
	ipv4want := fluent.OperationResult().
		WithOperationID(uint64(len(nexthops) + 2)).
		WithOperationType(constants.Add).
		WithProgrammingResult(fluent.InstalledInRIB).
		WithIPv4Operation(ateDstNetCIDR).
		AsResult()

	// OperationID must be i+1 where i is the position in the slice, but
	// gRIBI index doesn't have to be like that, only done so for the
	// sake of simplicity.
	ents = append(ents, nhgent, ipv4ent)
	wants = append(wants, nhgwant, ipv4want)
	return ents, wants
}

// sortPorts sorts the ports by the testbed port ID.
func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})
	return ports
}

// generateTraffic generates traffic from ateSrcNetCIDR to
// ateDstNetCIDR, then returns the atePorts as well as the number of
// packets received (inPkts) and sent (outPkts) across the atePorts.
func generateTraffic(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config) (atePorts []*ondatra.Port, inPkts []uint64, outPkts []uint64) {

	re, _ := regexp.Compile(".+:([a-zA-Z0-9]+)")
	dutString := "dut:" + re.FindStringSubmatch(ateSrcPort)[1]
	gwIp := portsIPv4[dutString]
	waitOTGARPEntry(t, time.Minute)
	dstMac := ate.OTG().Telemetry().Interface(ateSrcPort + ".Eth").Ipv4Neighbor(gwIp).LinkLayerAddress().Get(t)
	config.Flows().Clear().Items()
	flow := config.Flows().Add().SetName("flow")
	flow.Metrics().SetEnable(true)
	flow.TxRx().Port().
		SetTxName(re.FindStringSubmatch(ateSrcPort)[1])
	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(ateSrcPortMac)
	eth.Dst().SetValue(dstMac)
	ipv4 := flow.Packet().Add().Ipv4()
	if *randomSrcIP {
		t.Errorf("Random source IP not yet supported")
	} else {
		ipv4.Src().SetChoice("increment").Increment().SetStart(ateSrcNetFirstIP).SetCount(int32(ateSrcNetCount))
	}
	if *randomDstIP {
		t.Errorf("Random destination IP not yet supported")
	} else {
		ipv4.Dst().SetChoice("increment").Increment().SetStart(ateDstNetFirstIP).SetCount(int32(ateDstNetCount))
	}
	tcp := flow.Packet().Add().Tcp()
	if *randomSrcPort {
		tcp.SrcPort().SetValues(generateRandomPortList(65534))
	} else {
		tcp.SrcPort().SetChoice("increment").Increment().SetStart(1).SetCount(65534)
	}
	if *randomDstPort {
		tcp.DstPort().SetValues(generateRandomPortList(65534))
	} else {
		tcp.DstPort().SetChoice("increment").Increment().SetStart(1).SetCount(65534)
	}

	flow.Size().SetFixed(200)
	ate.OTG().PushConfig(t, config)

	if *trafficPause != 0 {
		t.Logf("Pausing before traffic at %v for %v", time.Now(), *trafficPause)
		time.Sleep(*trafficPause)
		t.Logf("Pausing ended at %v", time.Now())
	}

	ate.OTG().StartTraffic(t)
	t.Logf("Traffic starting at %v for %v", time.Now(), *trafficDuration)
	time.Sleep(*trafficDuration)
	t.Logf("Traffic stopping at %v", time.Now())
	ate.OTG().StopTraffic(t)

	atePorts = sortPorts(ate.Ports())
	inPkts = make([]uint64, len(atePorts))
	outPkts = make([]uint64, len(atePorts))

	otgutils.LogPortMetrics(t, ate.OTG(), config)
	for i, ap := range atePorts {
		for _, p := range config.Ports().Items() {
			portMetrics := ate.OTG().Telemetry().Port(p.Name()).Get(t)
			if ap.ID() == p.Name() {
				inPkts[i] = portMetrics.GetCounters().GetInFrames()
				outPkts[i] = portMetrics.GetCounters().GetOutFrames()
				continue
			}
		}
	}

	return atePorts, inPkts, outPkts
}

// normalize normalizes the input values so that the output values sum
// to 1.0 but reflect the proportions of the input.  For example,
// input [1, 2, 3, 4] is normalized to [0.1, 0.2, 0.3, 0.4].
func normalize(xs []uint64) (ys []float64, sum uint64) {
	for _, x := range xs {
		sum += x
	}
	ys = make([]float64, len(xs))
	for i, x := range xs {
		ys[i] = float64(x) / float64(sum)
	}
	return ys, sum
}

// generates a list of random tcp ports values
func generateRandomPortList(count int) []int32 {
	a := make([]int32, count)
	for index := range a {
		a[index] = int32(rand.Intn(65536-1) + 1)
	}
	return a
}

// portWants converts the nextHop wanted weights to per-port wanted
// weights listed in the same order as atePorts.
func portWants(nexthops []nextHop, atePorts []*ondatra.Port) []float64 {
	indexOfPort := make(map[string]int)
	for i, ap := range atePorts {
		indexOfPort["ate:"+ap.ID()] = i
	}

	weights := make([]float64, len(atePorts))
	for _, nh := range nexthops {
		if i, ok := indexOfPort[nh.Port]; ok {
			weights[i] = nh.Want
		}
	}

	return weights
}

func debugGRIBI(t testing.TB, dut *ondatra.DUTDevice) {
	// Debugging through OpenConfig.
	aftsPath := dut.Telemetry().NetworkInstance(*deviations.DefaultNetworkInstance).Afts()
	if q := aftsPath.Lookup(t); q.IsPresent() {
		fptest.LogYgot(t, "Afts", aftsPath, q.Val(t))
	} else {
		t.Log("afts value not present")
	}
}

// incrementMAC increments the MAC by i. Returns error if the mac cannot be parsed or overflows the mac address space
func incrementMAC(mac string, i int) (string, error) {
	macAddr, err := net.ParseMAC(mac)
	if err != nil {
		return "", err
	}
	convMac := binary.BigEndian.Uint64(append([]byte{0, 0}, macAddr...))
	convMac = convMac + uint64(i)
	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, convMac)
	if err != nil {
		return "", err
	}
	newMac := net.HardwareAddr(buf.Bytes()[2:8])
	return newMac.String(), nil
}

// waitOTGArpEntry ensures that ARP entries are present on the tx otg interface and traffic could be started
func waitOTGARPEntry(t *testing.T, timeout time.Duration) {
	t.Helper()
	ate := ondatra.ATE(t, "ate")
	ate.OTG().Telemetry().Interface(ateSrcPort+".Eth").Ipv4NeighborAny().LinkLayerAddress().Watch(
		t, timeout, func(val *otgtelemetry.QualifiedString) bool {
			return val.IsPresent()
		}).Await(t)
}
