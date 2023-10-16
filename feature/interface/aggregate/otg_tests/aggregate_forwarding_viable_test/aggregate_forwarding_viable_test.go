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

// For both Static LAG and LACP. Make the following forwarding-viable transitions on a port within the LAG on the DUT.
// 1. Tag the forwarding-viable=true to allow all the ports to pass traffic in
//    port-channel.
// 2. Transition from forwarding-viable=true to forwarding-viable=false.
// For each condition above, ensure following two things:
// -  traffic is load-balanced across the remaining interfaces in the LAG.
// -  there is no packet loss source from ATE source to ATE destination port.

// What is forwarding viable ?
// If set to false, the interface is not used for forwarding traffic,
// but as long as it is up, the interface still maintains its layer-2 adjacencies and runs its configured layer-2 functions (e.g. LLDP, etc.).
package aggregate_forwarding_viable_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"sort"
	"strconv"
	"strings"
	"testing"
	"text/tabwriter"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the aggregate testbed with the test
// topology.  IxNetwork flow requires both source and destination
// networks be configured on the ATE.  It is not possible to send
// packets to the ether.
//
// The testbed consists of ate:port1 -> dut:port1 and dut:port{2-9} ->
// ate:port{2-9}.  The first pair is called the "source" pair, and the
// second aggregate link the "destination" pair.
//
//   * Source: ate:port1 -> dut:port1 subnet 192.0.2.0/30 2001:db8::0/126
//   * Destination: dut:port{2-9} -> ate:port{2-9}
//     subnet 192.0.2.4/30 2001:db8::4/126
//
// Note that the first (.0, .4) and last (.3, .7) IPv4 addresses are
// reserved from the subnet for broadcast, so a /30 leaves exactly 2
// usable addresses.  This does not apply to IPv6 which allows /127
// for point to point links, but we use /126 so the numbering is
// consistent with IPv4.
//
// A traffic flow is configured from ate:port1 as source and ate:port{2-9}
// as destination.

const (
	plen4          = 30
	plen6          = 126
	ethernetCsmacd = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	ieee8023adLag  = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	lagTypeLACP    = oc.IfAggregate_AggregationType_LACP
	lagTypeSTATIC  = oc.IfAggregate_AggregationType_STATIC
)

var (
	dutSrc = attrs.Attributes{
		Desc:    "dutsrc",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	ateSrc = attrs.Attributes{
		Name:    "atesrc",
		MAC:     "02:11:01:00:00:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	dutDst = attrs.Attributes{
		Desc:    "dutdst",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	ateDst = attrs.Attributes{
		Name:    "atedst",
		MAC:     "02:12:01:00:00:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
)

type testArgs struct {
	dut     *ondatra.DUTDevice
	ate     *ondatra.ATEDevice
	top     gosnappi.Config
	lagType oc.E_IfAggregate_AggregationType

	dutPorts []*ondatra.Port
	atePorts []*ondatra.Port
	aggID    string
}

type linkPairs []linkPair

type linkPair struct {
	ATEPort *ondatra.Port
	DUTPort *ondatra.Port
}

func newLinkPairs(dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) linkPairs {
	var lp linkPairs
	atePorts := sortPorts(ate.Ports())
	dutPorts := sortPorts(dut.Ports())
	for i := 0; i < len(dutPorts); i++ {
		newLink := linkPair{
			ATEPort: atePorts[i],
			DUTPort: dutPorts[i],
		}
		lp = append(lp, newLink)
	}
	return lp
}

// sortPorts sorts the ports by the testbed port ID.
func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})
	return ports
}

// configSrcDUT configures source port of DUT
func (tc *testArgs) configSrcDUT(i *oc.Interface, a *attrs.Attributes) {
	i.Description = ygot.String(a.Desc)
	if deviations.InterfaceEnabled(tc.dut) {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(tc.dut) {
		s4.Enabled = ygot.Bool(true)
	}
	a4 := s4.GetOrCreateAddress(a.IPv4)
	a4.PrefixLength = ygot.Uint8(plen4)

	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(tc.dut) {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(plen6)
}

// configDstAggregateDUT configures port-channel destination ports
func (tc *testArgs) configDstAggregateDUT(i *oc.Interface, a *attrs.Attributes) {
	tc.configSrcDUT(i, a)
	i.Type = ieee8023adLag
	g := i.GetOrCreateAggregation()
	g.LagType = tc.lagType
}

// configDstMemberDUT enables destination ports, add other details like description, port and aggregate ID.
func (tc *testArgs) configDstMemberDUT(i *oc.Interface, p *ondatra.Port) {
	i.Description = ygot.String(p.String())
	i.Type = ethernetCsmacd

	if deviations.InterfaceEnabled(tc.dut) {
		i.Enabled = ygot.Bool(true)
	}

	e := i.GetOrCreateEthernet()
	e.AggregateId = ygot.String(tc.aggID)
}

// setupAggregateAtomically setup port-channel based on LAG type.
func (tc *testArgs) setupAggregateAtomically(t *testing.T) {
	d := &oc.Root{}

	if tc.lagType == lagTypeLACP {
		d.GetOrCreateLacp().GetOrCreateInterface(tc.aggID)
	}

	agg := d.GetOrCreateInterface(tc.aggID)
	agg.GetOrCreateAggregation().LagType = tc.lagType
	agg.Type = ieee8023adLag

	for _, port := range tc.dutPorts[1:] {
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(tc.aggID)

		i.Type = ethernetCsmacd

		if deviations.InterfaceEnabled(tc.dut) {
			i.Enabled = ygot.Bool(true)
		}
	}

	p := gnmi.OC()
	fptest.LogQuery(t, fmt.Sprintf("%s to Update()", tc.dut), p.Config(), d)
	gnmi.Update(t, tc.dut, p.Config(), d)
}

// clearAggregate delete any previously existing members of aggregate.
func (tc *testArgs) clearAggregate(t *testing.T) {
	// Clear the aggregate minlink.
	gnmi.Delete(t, tc.dut, gnmi.OC().Interface(tc.aggID).Aggregation().MinLinks().Config())

	// Clear the members of the aggregate.
	for _, port := range tc.dutPorts[1:] {
		resetBatch := &gnmi.SetBatch{}
		gnmi.BatchDelete(resetBatch, gnmi.OC().Interface(port.Name()).Ethernet().AggregateId().Config())
		gnmi.BatchDelete(resetBatch, gnmi.OC().Interface(port.Name()).ForwardingViable().Config())
		resetBatch.Set(t, tc.dut)
	}
}

// verifyDUT confirms if all the DUT ports are in operational enabled status.
func (tc *testArgs) verifyDUT(t *testing.T) {
	for _, port := range tc.dutPorts {
		path := gnmi.OC().Interface(port.Name())
		gnmi.Await(t, tc.dut, path.OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)
	}
}

// configureDUT configures source and destination ports of DUT and creates port-channel as well.
func (tc *testArgs) configureDUT(t *testing.T) {
	t.Helper()
	t.Logf("dut ports = %v", tc.dutPorts)
	if len(tc.dutPorts) < 2 {
		t.Fatalf("Testbed requires at least 2 ports, got %d", len(tc.dutPorts))
	}

	d := gnmi.OC()

	if deviations.AggregateAtomicUpdate(tc.dut) {
		tc.clearAggregate(t)
		tc.setupAggregateAtomically(t)
	}

	lacp := &oc.Lacp_Interface{Name: ygot.String(tc.aggID)}
	if tc.lagType == lagTypeLACP {
		lacp.LacpMode = oc.Lacp_LacpActivityType_ACTIVE
	} else {
		lacp.LacpMode = oc.Lacp_LacpActivityType_UNSET
	}
	lacpPath := d.Lacp().Interface(tc.aggID)
	fptest.LogQuery(t, "LACP", lacpPath.Config(), lacp)
	gnmi.Replace(t, tc.dut, lacpPath.Config(), lacp)

	agg := &oc.Interface{Name: ygot.String(tc.aggID)}
	tc.configDstAggregateDUT(agg, &dutDst)
	aggPath := d.Interface(tc.aggID)
	fptest.LogQuery(t, tc.aggID, aggPath.Config(), agg)
	gnmi.Replace(t, tc.dut, aggPath.Config(), agg)
	if deviations.ExplicitInterfaceInDefaultVRF(tc.dut) {
		fptest.AssignToNetworkInstance(t, tc.dut, tc.aggID, deviations.DefaultNetworkInstance(tc.dut), 0)
	}

	srcp := tc.dutPorts[0]
	srci := &oc.Interface{Name: ygot.String(srcp.Name())}
	tc.configSrcDUT(srci, &dutSrc)
	srci.Type = ethernetCsmacd
	srciPath := d.Interface(srcp.Name())
	if deviations.ExplicitPortSpeed(tc.dut) {
		srci.GetOrCreateEthernet().PortSpeed = fptest.GetIfSpeed(t, srcp)
	}
	fptest.LogQuery(t, srcp.String(), srciPath.Config(), srci)
	gnmi.Replace(t, tc.dut, srciPath.Config(), srci)
	if deviations.ExplicitInterfaceInDefaultVRF(tc.dut) {
		fptest.AssignToNetworkInstance(t, tc.dut, srcp.Name(), deviations.DefaultNetworkInstance(tc.dut), 0)
	}

	for _, port := range tc.dutPorts[1:] {
		i := &oc.Interface{Name: ygot.String(port.Name())}
		i.Type = ethernetCsmacd

		if deviations.InterfaceEnabled(tc.dut) {
			i.Enabled = ygot.Bool(true)
		}

		tc.configDstMemberDUT(i, port)
		iPath := d.Interface(port.Name())
		if deviations.ExplicitPortSpeed(tc.dut) {
			i.GetOrCreateEthernet().PortSpeed = fptest.GetIfSpeed(t, port)
		}
		fptest.LogQuery(t, port.String(), iPath.Config(), i)
		gnmi.Replace(t, tc.dut, iPath.Config(), i)
	}
}

// configureATE configures source and destination port of ATE and add creates port-channel as well.
func (tc *testArgs) configureATE(t *testing.T) {
	t.Helper()
	if len(tc.atePorts) < 2 {
		t.Fatalf("Testbed requires at least 2 ports, got: %v", tc.atePorts)
	}

	p0 := tc.atePorts[0]
	tc.top.Ports().Add().SetName(p0.ID())
	d0 := tc.top.Devices().Add().SetName(ateSrc.Name)
	srcEth := d0.Ethernets().Add().SetName(ateSrc.Name + ".Eth").SetMac(ateSrc.MAC)
	srcEth.Connection().SetPortName(p0.ID())
	srcEth.Ipv4Addresses().Add().SetName(ateSrc.Name + ".IPv4").SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).SetPrefix(uint32(ateSrc.IPv4Len))
	srcEth.Ipv6Addresses().Add().SetName(ateSrc.Name + ".IPv6").SetAddress(ateSrc.IPv6).SetGateway(dutSrc.IPv6).SetPrefix(uint32(ateSrc.IPv6Len))

	// Adding the rest of the ports to the configuration and to the LAG
	agg := tc.top.Lags().Add().SetName(ateDst.Name)
	if tc.lagType == lagTypeSTATIC {
		lagId, _ := strconv.Atoi(tc.aggID)
		agg.Protocol().SetChoice("static").Static().SetLagId(uint32(lagId))
		for i, p := range tc.atePorts[1:] {
			port := tc.top.Ports().Add().SetName(p.ID())
			newMac, err := incrementMAC(ateDst.MAC, i+1)
			if err != nil {
				t.Fatal(err)
			}
			agg.Ports().Add().SetPortName(port.Name()).Ethernet().SetMac(newMac).SetName("LAGRx-" + strconv.Itoa(i))
		}
	} else {
		agg.Protocol().SetChoice("lacp")
		agg.Protocol().Lacp().SetActorKey(1).SetActorSystemPriority(1).SetActorSystemId(ateDst.MAC)
		for i, p := range tc.atePorts[1:] {
			port := tc.top.Ports().Add().SetName(p.ID())
			newMac, err := incrementMAC(ateDst.MAC, i+1)
			if err != nil {
				t.Fatal(err)
			}
			lagPort := agg.Ports().Add().SetPortName(port.Name())
			lagPort.Ethernet().SetMac(newMac).SetName("LAGRx-" + strconv.Itoa(i))
			lagPort.Lacp().SetActorActivity("active").SetActorPortNumber(uint32(i) + 1).SetActorPortPriority(1).SetLacpduTimeout(0)
		}
	}

	// Disable FEC for 100G-FR ports because Novus does not support it.
	p100gbasefr := []string{}
	for _, p := range tc.atePorts {
		if p.PMD() == ondatra.PMD100GBASEFR {
			p100gbasefr = append(p100gbasefr, p.ID())
		}
	}

	if len(p100gbasefr) > 0 {
		l1Settings := tc.top.Layer1().Add().SetName("L1").SetPortNames(p100gbasefr)
		l1Settings.SetAutoNegotiate(true).SetIeeeMediaDefaults(false).SetSpeed("speed_100_gbps")
		autoNegotiate := l1Settings.AutoNegotiation()
		autoNegotiate.SetRsFec(false)
	}

	dstDev := tc.top.Devices().Add().SetName(agg.Name() + ".dev")
	dstEth := dstDev.Ethernets().Add().SetName(ateDst.Name + ".Eth").SetMac(ateDst.MAC)
	dstEth.Connection().SetLagName(agg.Name())
	dstEth.Ipv4Addresses().Add().SetName(ateDst.Name + ".IPv4").SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).SetPrefix(uint32(ateDst.IPv4Len))
	dstEth.Ipv6Addresses().Add().SetName(ateDst.Name + ".IPv6").SetAddress(ateDst.IPv6).SetGateway(dutDst.IPv6).SetPrefix(uint32(ateDst.IPv6Len))

	tc.ate.OTG().PushConfig(t, tc.top)
	tc.ate.OTG().StartProtocols(t)
}

// incrementMAC uses a mac string and increments it by the given i
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

// generates a list of random tcp ports values
func generateRandomPortList(count uint) []uint32 {
	a := make([]uint32, count)
	for index := range a {
		a[index] = uint32(rand.Intn(65536-1) + 1)
	}
	return a
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
	return sort.Float64Slice(ys), sum
}

// portWants converts the nextHop wanted weights to per-port wanted
// weights listed in the same order as atePorts.
// portWantsViable -> [1/n, 1/n, ...]
func (tc *testArgs) portWantsViable(t *testing.T) []float64 {
	weights := []float64{}
	numPorts := len(tc.dutPorts[1:])
	for range tc.dutPorts[1:] {
		weights = append(weights, 1/float64(numPorts))
	}
	return sort.Float64Slice(weights)
}

// portWants converts the nextHop wanted weights to per-port wanted
// weights listed in the same order as atePorts.
// portWantsNotViable -> [0, 1/(n-1), 1/(n-1), ...]
func (tc *testArgs) portWantsNotViable(t *testing.T) []float64 {
	weights := []float64{}
	// Forwarding viable flag is set as false for one of the port in the port channel
	numPorts := len(tc.dutPorts[2:])
	weights = append(weights, 0)
	for range tc.dutPorts[2:] {
		weights = append(weights, 1/float64(numPorts))
	}
	return sort.Float64Slice(weights)
}

// debugATEFlows logs detailed tracking information on traffic flows and find if there is any loss pct in the flow.
func debugATEFlows(t *testing.T, ate *ondatra.ATEDevice, flow gosnappi.Flow, lp linkPairs) {

	recvMetric := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).State())
	txPackets := float32(recvMetric.GetCounters().GetOutPkts())
	rxPackets := float32(recvMetric.GetCounters().GetInPkts())
	if txPackets == 0 {
		t.Fatalf("Tx packets should be higher than 0")
	}
	lostPackets := txPackets - rxPackets
	if got := lostPackets * 100 / txPackets; got > 0 {
		t.Fatalf("LossPct for flow %s: got %f, want 0", flow.Name(), got)
	}
}

// verifyCounterDiff finds the difference between counter values before and after sending traffic. It also calculates if there is any packet loss.
func (tc *testArgs) verifyCounterDiff(t *testing.T, before, after []*oc.Interface_Counters, want []float64) {
	b := &strings.Builder{}
	w := tabwriter.NewWriter(b, 0, 0, 1, ' ', 0)
	approxOpt := cmpopts.EquateApprox(0 /* frac */, 0.01 /* absolute */)
	fmt.Fprint(w, "Interface Counter Deltas\n\n")
	fmt.Fprint(w, "Name\tInPkts\tInOctets\tOutPkts\tOutOctets\n")
	allOutPkts := []uint64{}

	for i, port := range tc.dutPorts[1:] {
		inPkts := after[i].GetInUnicastPkts() - before[i].GetInUnicastPkts()
		inOctets := after[i].GetInOctets() - before[i].GetInOctets()
		outPkts := after[i].GetOutUnicastPkts() - before[i].GetOutUnicastPkts()
		allOutPkts = append(allOutPkts, outPkts)
		outOctets := after[i].GetOutOctets() - before[i].GetOutOctets()

		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\n",
			port,
			inPkts, inOctets,
			outPkts, outOctets)
	}
	got, _ := normalize(allOutPkts)

	t.Logf("outPkts normalized got: %v", got)
	t.Logf("want: %v", want)
	t.Run("Ratio", func(t *testing.T) {
		if diff := cmp.Diff(want, got, approxOpt); diff != "" {
			t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
		}
	})

	w.Flush()
	t.Log(b)
}

// testAggregateForwardingFlow set the forwardingViable tag based on test case and returns test results of packet distribution and packet loss for each LAG type.
// Interfaces that are not viable for forwarding should still be allowed to receive traffic, but should not be used for sending packets.
func (tc *testArgs) testAggregateForwardingFlow(t *testing.T, forwardingViable bool) {
	var want []float64
	lp := newLinkPairs(tc.dut, tc.ate)
	// Update the interface config of one port in port-channel when the forwarding flag is set as false.
	if !forwardingViable {
		t.Log("First port does not forward traffic because it is marked as not viable.")
		gnmi.Update(t, tc.dut, gnmi.OC().Interface(tc.dutPorts[1].Name()).ForwardingViable().Config(), forwardingViable)
	}

	i1 := ateSrc.Name
	i2 := ateDst.Name

	tc.top.Flows().Clear().Items()
	flow := tc.top.Flows().Add().SetName("flow")
	flow.Metrics().SetEnable(true)
	flow.Size().SetFixed(128)
	flow.Packet().Add().Ethernet().Src().SetValue(ateSrc.MAC)

	flow.TxRx().Device().SetTxNames([]string{i1 + ".IPv4"}).SetRxNames([]string{i2 + ".IPv4"})
	v4 := flow.Packet().Add().Ipv4()
	v4.Src().SetValue(ateSrc.IPv4)
	v4.Dst().SetValue(ateDst.IPv4)
	tcp := flow.Packet().Add().Tcp()
	tcp.SrcPort().SetValues(generateRandomPortList(65534))
	tc.ate.OTG().PushConfig(t, tc.top)
	tc.ate.OTG().StartProtocols(t)

	beforeTrafficCounters := tc.getCounters(t, "before")

	tc.ate.OTG().StartTraffic(t)
	time.Sleep(15 * time.Second)
	tc.ate.OTG().StopTraffic(t)

	otgutils.LogFlowMetrics(t, tc.ate.OTG(), tc.top)
	otgutils.LogPortMetrics(t, tc.ate.OTG(), tc.top)

	debugATEFlows(t, tc.ate, flow, lp)

	pkts := gnmi.Get(t, tc.ate.OTG(), gnmi.OC().Flow("flow").Counters().OutPkts().State())
	if pkts == 0 {
		t.Errorf("Flow sent packets: got %v, want non zero", pkts)
	}
	afterTrafficCounters := tc.getCounters(t, "after")
	if forwardingViable == false {
		want = tc.portWantsNotViable(t)
	} else {
		want = tc.portWantsViable(t)
	}
	tc.verifyCounterDiff(t, beforeTrafficCounters, afterTrafficCounters, want)
	t.Log("Counters", beforeTrafficCounters, afterTrafficCounters)
}

// getCounters returns list of interface counters for each dut port part of LAG.
func (tc *testArgs) getCounters(t *testing.T, when string) []*oc.Interface_Counters {
	results := []*oc.Interface_Counters{}
	b := &strings.Builder{}
	w := tabwriter.NewWriter(b, 0, 0, 1, ' ', 0)

	t.Log("DUT counters entries")
	fmt.Fprint(w, "Raw Interface Counters\n\n")
	fmt.Fprint(w, "Name\tInUnicastPkts\tInOctets\tOutUnicastPkts\tOutOctets\n")
	for _, port := range tc.dutPorts[1:] {
		counters := gnmi.Get(t, tc.dut, gnmi.OC().Interface(port.Name()).Counters().State())
		results = append(results, counters)
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\n",
			port.Name(),
			counters.GetInUnicastPkts(), counters.GetInOctets(),
			counters.GetOutUnicastPkts(), counters.GetOutOctets())
	}
	w.Flush()
	t.Log(b)
	return results
}

func TestAggregateForwardingViable(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	aggID := netutil.NextAggregateInterface(t, dut)

	lagTypes := []oc.E_IfAggregate_AggregationType{lagTypeLACP, lagTypeSTATIC}
	for _, lagType := range lagTypes {
		args := &testArgs{
			dut:      dut,
			ate:      ate,
			top:      gosnappi.NewConfig(),
			lagType:  lagType,
			dutPorts: sortPorts(dut.Ports()),
			atePorts: sortPorts(ate.Ports()),
			aggID:    aggID,
		}
		t.Run(fmt.Sprintf("LagType=%s", lagType), func(t *testing.T) {
			args.configureATE(t)
			args.configureDUT(t)
			args.verifyDUT(t)

			for _, forwardingViable := range []bool{true, false} {
				t.Run(fmt.Sprintf("ForwardingViable=%t", forwardingViable), func(t *testing.T) {
					args.testAggregateForwardingFlow(t, forwardingViable)
				})
			}
		})
	}
}
