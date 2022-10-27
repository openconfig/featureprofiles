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

package balancing_test

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
	"github.com/openconfig/ygot/ygot"

	"github.com/openconfig/ondatra/netutil"
	telemetry "github.com/openconfig/ondatra/telemetry"
	otgtelemetry "github.com/openconfig/ondatra/telemetry/otg"
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
	opUp           = telemetry.Interface_OperStatus_UP
	ethernetCsmacd = telemetry.IETFInterfaces_InterfaceType_ethernetCsmacd
	ieee8023adLag  = telemetry.IETFInterfaces_InterfaceType_ieee8023adLag
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

const (
	lagTypeLACP = telemetry.IfAggregate_AggregationType_LACP
)

type testCase struct {
	desc    string
	dut     *ondatra.DUTDevice
	ate     *ondatra.ATEDevice
	top     gosnappi.Config
	lagType telemetry.E_IfAggregate_AggregationType

	dutPorts []*ondatra.Port
	atePorts []*ondatra.Port
	aggID    string
	l3header string
}

func (*testCase) configSrcDUT(i *telemetry.Interface, a *attrs.Attributes) {
	i.Description = ygot.String(a.Desc)
	if *deviations.InterfaceEnabled {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if *deviations.InterfaceEnabled {
		s4.Enabled = ygot.Bool(true)
	}
	a4 := s4.GetOrCreateAddress(a.IPv4)
	a4.PrefixLength = ygot.Uint8(plen4)

	s6 := s.GetOrCreateIpv6()
	if *deviations.InterfaceEnabled {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(plen6)
}

func (tc *testCase) configDstAggregateDUT(i *telemetry.Interface, a *attrs.Attributes) {
	tc.configSrcDUT(i, a)
	i.Type = ieee8023adLag
	g := i.GetOrCreateAggregation()
	g.LagType = tc.lagType
}

func (tc *testCase) configDstMemberDUT(i *telemetry.Interface, p *ondatra.Port) {
	i.Description = ygot.String(p.String())
	i.Type = ethernetCsmacd
	if *deviations.InterfaceEnabled {
		i.Enabled = ygot.Bool(true)
	}

	e := i.GetOrCreateEthernet()
	e.AggregateId = ygot.String(tc.aggID)
}

func (tc *testCase) setupAggregateAtomically(t *testing.T) {
	d := &telemetry.Device{}

	if tc.lagType == lagTypeLACP {
		d.GetOrCreateLacp().GetOrCreateInterface(tc.aggID)
	}

	agg := d.GetOrCreateInterface(tc.aggID)
	agg.GetOrCreateAggregation().LagType = tc.lagType
	agg.Type = ieee8023adLag

	for n, port := range tc.dutPorts {
		if n < 1 {
			// We designate port 0 as the source link, not part of LAG.
			continue
		}
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(tc.aggID)
		i.Type = ethernetCsmacd

		if *deviations.InterfaceEnabled {
			i.Enabled = ygot.Bool(true)
		}
	}

	p := tc.dut.Config()
	fptest.LogYgot(t, fmt.Sprintf("%s to Update()", tc.dut), p, d)
	p.Update(t, d)
}

func (tc *testCase) clearAggregateMembers(t *testing.T) {
	for n, port := range tc.dutPorts {
		if n < 1 {
			// We designate port 0 as the source link, not part of LAG.
			continue
		}
		tc.dut.Config().Interface(port.Name()).Ethernet().AggregateId().Delete(t)
	}
}

func (tc *testCase) verifyDUT(t *testing.T) {
	// Wait for LAG negotiation and verify LAG type for the aggregate interface.
	tc.dut.Telemetry().Interface(tc.aggID).Type().Await(t, time.Minute, ieee8023adLag)
	for _, port := range tc.dutPorts {
		path := tc.dut.Telemetry().Interface(port.Name())
		path.OperStatus().Await(t, time.Minute, telemetry.Interface_OperStatus_UP)
	}
}

// Wait for LAG ports to be collecting and distributing and the LAG groups to be up on DUT and OTG
func (tc *testCase) verifyLAG(t *testing.T) {
	t.Logf("Waiting for LAG group on DUT to be up")
	_, ok := tc.dut.Telemetry().Interface(tc.aggID).OperStatus().Watch(
		t, time.Minute, func(val *telemetry.QualifiedE_Interface_OperStatus) bool {
			return val.Val(t) == opUp
		}).Await(t)
	if !ok {
		t.Fatalf("DUT LAG is not ready. Expected %s got %s", opUp.String(), tc.dut.Telemetry().Interface(tc.aggID).OperStatus().Get(t).String())
	}
	t.Logf("Waiting for LAG group on OTG to be up")
	_, ok = tc.ate.OTG().Telemetry().Lag("LAG").OperStatus().Watch(t, time.Minute,
		func(val *otgtelemetry.QualifiedE_Lag_OperStatus) bool {
			return val.IsPresent() && val.Val(t).String() == "UP"
		}).Await(t)
	if !ok {
		otgutils.LogLAGMetrics(t, tc.ate.OTG(), tc.top)
		t.Fatalf("OTG LAG is not ready. Expected UP got %s", tc.ate.OTG().Telemetry().Lag("LAG").OperStatus().Get(t).String())
	}

	if tc.lagType == telemetry.IfAggregate_AggregationType_LACP {
		t.Logf("Waiting LAG DUT ports to start collecting and distributing")
		for _, dp := range tc.dutPorts[1:] {
			_, ok := tc.dut.Telemetry().Lacp().InterfaceAny().Member(dp.Name()).Collecting().Watch(t, time.Minute,
				func(val *telemetry.QualifiedBool) bool {
					return val.Val(t)
				}).Await(t)
			if !ok {
				t.Fatalf("DUT LAG port %v is not collecting", dp)
			}
			_, ok = tc.dut.Telemetry().Lacp().InterfaceAny().Member(dp.Name()).Distributing().Watch(t, time.Minute,
				func(val *telemetry.QualifiedBool) bool {
					return val.Val(t)
				}).Await(t)
			if !ok {
				t.Fatalf("DUT LAG port %v is not distributing", dp)
			}
		}
		t.Logf("Waiting LAG OTG ports to start collecting and distributing")
		for _, p := range tc.atePorts[1:] {
			_, ok := tc.ate.OTG().Telemetry().Lacp().LagMember(p.ID()).Collecting().Watch(t, time.Minute,
				func(val *otgtelemetry.QualifiedBool) bool {
					return val.IsPresent() && val.Val(t)
				}).Await(t)
			if !ok {
				t.Fatalf("OTG LAG port %v is not collecting", p)
			}
			_, ok = tc.ate.OTG().Telemetry().Lacp().LagMember(p.ID()).Distributing().Watch(t, time.Minute,
				func(val *otgtelemetry.QualifiedBool) bool {
					return val.IsPresent() && val.Val(t)
				}).Await(t)
			if !ok {
				t.Fatalf("OTG LAG port %v is not distributing", p)
			}
		}
		otgutils.LogLACPMetrics(t, tc.ate.OTG(), tc.top)
	}
	otgutils.LogLAGMetrics(t, tc.ate.OTG(), tc.top)

}

func (tc *testCase) configureDUT(t *testing.T) {
	t.Logf("dut ports = %v", tc.dutPorts)
	if len(tc.dutPorts) < 2 {
		t.Fatalf("Testbed requires at least 2 ports, got %d", len(tc.dutPorts))
	}

	d := tc.dut.Config()

	if *deviations.AggregateAtomicUpdate {
		tc.clearAggregateMembers(t)
		tc.setupAggregateAtomically(t)
	}

	if tc.lagType == lagTypeLACP {
		lacp := &telemetry.Lacp_Interface{Name: ygot.String(tc.aggID)}
		lacp.LacpMode = telemetry.Lacp_LacpActivityType_ACTIVE

		lacpPath := d.Lacp().Interface(tc.aggID)
		fptest.LogYgot(t, "LACP", lacpPath, lacp)
		lacpPath.Replace(t, lacp)
	}

	// TODO - to remove this sleep later
	time.Sleep(5 * time.Second)

	agg := &telemetry.Interface{Name: ygot.String(tc.aggID)}
	tc.configDstAggregateDUT(agg, &dutDst)
	aggPath := d.Interface(tc.aggID)
	fptest.LogYgot(t, tc.aggID, aggPath, agg)
	aggPath.Replace(t, agg)

	srcp := tc.dutPorts[0]
	srci := &telemetry.Interface{Name: ygot.String(srcp.Name())}
	tc.configSrcDUT(srci, &dutSrc)
	srci.Type = ethernetCsmacd
	srciPath := d.Interface(srcp.Name())
	fptest.LogYgot(t, srcp.String(), srciPath, srci)
	srciPath.Replace(t, srci)

	for _, port := range tc.dutPorts[1:] {
		i := &telemetry.Interface{Name: ygot.String(port.Name())}
		tc.configDstMemberDUT(i, port)
		iPath := d.Interface(port.Name())
		fptest.LogYgot(t, port.String(), iPath, i)
		iPath.Replace(t, i)
	}

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
func generateRandomPortList(count int) []int32 {
	a := make([]int32, count)
	for index := range a {
		a[index] = int32(rand.Intn(65536-1) + 1)
	}
	return a
}

func (tc *testCase) configureATE(t *testing.T) {
	if len(tc.atePorts) < 2 {
		t.Fatalf("Testbed requires at least 2 ports, got: %v", tc.atePorts)
	}

	p0 := tc.atePorts[0]
	tc.top.Ports().Add().SetName(p0.ID())
	d0 := tc.top.Devices().Add().SetName(ateSrc.Name)
	srcEth := d0.Ethernets().Add().SetName(ateSrc.Name + ".Eth").SetPortName(p0.ID()).SetMac(ateSrc.MAC)
	srcEth.Connection().SetChoice("port_name").SetPortName(p0.ID())
	srcEth.Ipv4Addresses().Add().SetName(ateSrc.Name + ".IPv4").SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).SetPrefix(int32(ateSrc.IPv4Len))
	srcEth.Ipv6Addresses().Add().SetName(ateSrc.Name + ".IPv6").SetAddress(ateSrc.IPv6).SetGateway(dutSrc.IPv6).SetPrefix(int32(ateSrc.IPv6Len))

	agg := tc.top.Lags().Add().SetName("LAG")
	for i, p := range tc.atePorts[1:] {
		port := tc.top.Ports().Add().SetName(p.ID())
		lagPort := agg.Ports().Add()
		newMac, err := incrementMAC(ateDst.MAC, i+1)
		if err != nil {
			t.Fatal(err)
		}
		lagPort.SetPortName(port.Name()).
			Ethernet().SetMac(newMac).
			SetName("LAGRx-" + strconv.Itoa(i))
		lagPort.Lacp().SetActorPortNumber(int32(i + 1)).SetActorPortPriority(1).SetActorActivity("active")
	}
	agg.Protocol().Lacp().SetActorKey(1).SetActorSystemPriority(1).SetActorSystemId("01:01:01:01:01:01")

	dstDev := tc.top.Devices().Add().SetName(agg.Name())
	dstEth := dstDev.Ethernets().Add().SetName(ateDst.Name + ".Eth").SetPortName(agg.Name()).SetMac(ateDst.MAC)
	dstEth.Connection().SetChoice("lag_name").SetLagName(agg.Name())
	dstEth.Ipv4Addresses().Add().SetName(ateDst.Name + ".IPv4").SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).SetPrefix(int32(ateDst.IPv4Len))
	dstEth.Ipv6Addresses().Add().SetName(ateDst.Name + ".IPv6").SetAddress(ateDst.IPv6).SetGateway(dutDst.IPv6).SetPrefix(int32(ateDst.IPv6Len))

	tc.ate.OTG().PushConfig(t, tc.top)
	tc.ate.OTG().StartProtocols(t)
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

var approxOpt = cmpopts.EquateApprox(0 /* frac */, 0.01 /* absolute */)

// portWants converts the nextHop wanted weights to per-port wanted
// weights listed in the same order as atePorts.
func (tc *testCase) portWants() []float64 {
	numPorts := len(tc.dutPorts[1:])
	weights := []float64{}
	for i := 0; i < numPorts; i++ {
		weights = append(weights, 1/float64(numPorts))
	}
	return weights
}

func (tc *testCase) verifyCounterDiff(t *testing.T, before, after map[string]*telemetry.Interface_Counters) {
	b := &strings.Builder{}
	w := tabwriter.NewWriter(b, 0, 0, 1, ' ', 0)

	fmt.Fprint(w, "Interface Counter Deltas\n\n")
	fmt.Fprint(w, "Name\tInPkts\tInOctets\tOutPkts\tOutOctets\n")
	allInPkts := []uint64{}
	allOutPkts := []uint64{}

	for port := range before {
		inPkts := after[port].GetInUnicastPkts() - before[port].GetInUnicastPkts()
		allInPkts = append(allInPkts, inPkts)
		inOctets := after[port].GetInOctets() - before[port].GetInOctets()
		outPkts := after[port].GetOutUnicastPkts() - before[port].GetOutUnicastPkts()
		allOutPkts = append(allOutPkts, outPkts)
		outOctets := after[port].GetOutOctets() - before[port].GetOutOctets()

		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\n",
			port,
			inPkts, inOctets,
			outPkts, outOctets)
	}
	got, outSum := normalize(allOutPkts)
	want := tc.portWants()
	t.Logf("outPkts normalized got: %v", got)
	t.Logf("want: %v", want)
	t.Run("Ratio", func(t *testing.T) {
		if diff := cmp.Diff(want, got, approxOpt); diff != "" {
			t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
		}
	})
	t.Run("Loss", func(t *testing.T) {
		if allInPkts[0] > outSum {
			t.Errorf("Traffic flow received %d packets, sent only %d",
				allOutPkts[0], outSum)
		}
	})
	w.Flush()

	t.Log(b)
}

func (tc *testCase) testFlow(t *testing.T, l3header string) {
	i1 := ateSrc.Name
	i2 := ateDst.Name

	tc.top.Flows().Clear().Items()
	flow := tc.top.Flows().Add().SetName(l3header)
	flow.Metrics().SetEnable(true)
	flow.Size().SetFixed(128)
	flow.Packet().Add().Ethernet().Src().SetValue(ateSrc.MAC)

	if l3header == "ipv4" {
		flow.TxRx().Device().SetTxNames([]string{i1 + ".IPv4"}).SetRxNames([]string{i2 + ".IPv4"})
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(ateSrc.IPv4)
		v4.Dst().SetValue(ateDst.IPv4)
	}
	if l3header == "ipv4inipv4" {
		flow.TxRx().Device().SetTxNames([]string{i1 + ".IPv4"}).SetRxNames([]string{i2 + ".IPv4"})
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(ateSrc.IPv4)
		v4.Dst().SetValue(ateDst.IPv4)
		flow.Packet().Add().Ipv4()
	}
	if l3header == "ipv6" {
		flow.TxRx().Device().SetTxNames([]string{i1 + ".IPv6"}).SetRxNames([]string{i2 + ".IPv6"})
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(ateSrc.IPv6)
		v6.Dst().SetValue(ateDst.IPv6)
	}
	if l3header == "ipv6inipv4" {
		flow.TxRx().Device().SetTxNames([]string{i1 + ".IPv4"}).SetRxNames([]string{i2 + ".IPv4"})
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(ateSrc.IPv4)
		v4.Dst().SetValue(ateDst.IPv4)
		flow.Packet().Add().Ipv6()
	}

	tcp := flow.Packet().Add().Tcp()
	tcp.SrcPort().SetValues(generateRandomPortList(65534))
	tcp.DstPort().SetValues(generateRandomPortList(65534))
	tc.ate.OTG().PushConfig(t, tc.top)
	tc.ate.OTG().StartProtocols(t)

	tc.verifyLAG(t)

	beforeTrafficCounters := tc.getCounters(t, "before")

	tc.ate.OTG().StartTraffic(t)
	time.Sleep(15 * time.Second)
	tc.ate.OTG().StopTraffic(t)

	otgutils.LogPortMetrics(t, tc.ate.OTG(), tc.top)
	otgutils.LogFlowMetrics(t, tc.ate.OTG(), tc.top)
	otgutils.LogLAGMetrics(t, tc.ate.OTG(), tc.top)
	recvMetric := tc.ate.OTG().Telemetry().Flow(flow.Name()).Get(t)
	pkts := recvMetric.GetCounters().GetOutPkts()

	if pkts == 0 {
		t.Errorf("Flow sent packets: got %v, want non zero", pkts)
	}
	afterTrafficCounters := tc.getCounters(t, "after")
	tc.verifyCounterDiff(t, beforeTrafficCounters, afterTrafficCounters)

}

func (tc *testCase) getCounters(t *testing.T, when string) map[string]*telemetry.Interface_Counters {
	results := make(map[string]*telemetry.Interface_Counters)
	b := &strings.Builder{}
	w := tabwriter.NewWriter(b, 0, 0, 1, ' ', 0)

	fmt.Fprint(w, "Raw Interface Counters\n\n")
	fmt.Fprint(w, "Name\tInUnicastPkts\tInOctets\tOutUnicastPkts\tOutOctets\n")
	for _, port := range tc.dutPorts[1:] {
		counters := tc.dut.Telemetry().Interface(port.Name()).Counters().Get(t)
		results[port.Name()] = counters
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\n",
			port.Name(),
			counters.GetInUnicastPkts(), counters.GetInOctets(),
			counters.GetOutUnicastPkts(), counters.GetOutOctets())
	}
	w.Flush()

	t.Log(b)

	return results
}

// sortPorts sorts the ports by the testbed port ID.
func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})
	return ports
}

// define different flows for traffic
func TestBalancing(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	aggID := netutil.NextBundleInterface(t, dut)

	tests := []testCase{
		{
			desc:     "IPV4",
			l3header: "ipv4",
		},
		{
			desc:     "IPV4inIPV4",
			l3header: "ipv4inipv4",
		},
		{
			desc:     "IPV6",
			l3header: "ipv6",
		},
		{
			desc:     "IPV6inIPV4",
			l3header: "ipv6inipv4",
		},
		// TODO: flowHeader support is not available on OTG
		// {
		// 	desc:     "IPV6 FlowLabel",
		// 	l3header: []ondatra.Header{flowHeader},
		// },
	}
	tc := &testCase{
		dut:     dut,
		ate:     ate,
		lagType: lagTypeLACP,
		top:     ate.OTG().NewConfig(t),

		dutPorts: sortPorts(dut.Ports()),
		atePorts: sortPorts(ate.Ports()),
		aggID:    aggID,
	}
	tc.configureDUT(t)
	t.Run("verifyDUT", tc.verifyDUT)
	tc.configureATE(t)

	for _, tf := range tests {
		t.Run(tf.desc, func(t *testing.T) {
			tc.l3header = tf.l3header
			tc.testFlow(t, tc.l3header)
		})
	}
}
