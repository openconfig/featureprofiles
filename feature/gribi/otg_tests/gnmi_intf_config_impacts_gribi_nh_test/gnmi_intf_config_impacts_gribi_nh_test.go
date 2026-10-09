// Copyright 2025 Google LLC
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

// Package gnmiintfconfigimpactsgribinhtest implements TE-1.7 of the gRIBI
// test plan: verifying that gNMI interface configuration changes (admin
// state, MTU) to an interface used by an active gRIBI NextHop are handled
// gracefully.
package gnmiintfconfigimpactsgribinhtest

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/iputil"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	gribiclient "github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126

	nh2ID  = 10
	nh3ID  = 11
	nh4ID  = 12
	nhg1ID = 100

	nhDownID  = 20
	nhgDownID = 20

	// Port2-only NHG for TE-1.7.4, kept separate so it doesn't depend on TE-1.7.3's outcome.
	nhgPort2OnlyID = 30

	numV4Routes = 1000
	numV6Routes = 1000

	ipv4BaseRoute = "203.0.113.1"
	ipv6BaseRoute = "2001:db8:a::1"
	ipv4NegRoute  = "203.0.113.255"
	ipv4MTURoute  = "203.0.113.254"

	flowECMPv4Name  = "flowECMPv4"
	flowECMPv6Name  = "flowECMPv6"
	flowNegDownName = "flowNegDown"
	flowMTUName     = "flowMTU"

	trafficPPS    = 1000
	monitorWindow = 30 * time.Second
	awaitTimeout  = time.Minute
	// ASIC-level FIB reconvergence lag not reflected by oper-status.
	convergeSettle = 10 * time.Second

	mtuDefault   = 1500
	mtuJumbo     = 9000
	mtuTooSmall  = 500
	pktSizeLarge = 1500

	lossTolerancePct = 1.0

	// Loose bound: only catches gross ECMP imbalance, since real hashing is never even.
	ecmpHashTolerancePct = 40.0

	// Used only with the GRIBIMACOverrideStaticARPStaticRoute deviation.
	magicMac = "02:00:00:00:00:01"
	magicIP  = "192.168.1.1"
)

type portPair struct {
	name string
	dut  attrs.Attributes
	ate  attrs.Attributes
}

var portPairs = []portPair{
	{
		name: "port1",
		dut:  attrs.Attributes{Desc: "DUT Port 1", IPv4: "192.0.2.1", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8:1::1", IPv6Len: ipv6PrefixLen},
		ate:  attrs.Attributes{Name: "port1", MAC: "02:00:01:01:01:01", IPv4: "192.0.2.2", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8:1::2", IPv6Len: ipv6PrefixLen},
	},
	{
		name: "port2",
		dut:  attrs.Attributes{Desc: "DUT Port 2", IPv4: "198.51.100.1", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8:2::1", IPv6Len: ipv6PrefixLen},
		ate:  attrs.Attributes{Name: "port2", MAC: "02:00:02:01:01:01", IPv4: "198.51.100.2", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8:2::2", IPv6Len: ipv6PrefixLen},
	},
	{
		name: "port3",
		dut:  attrs.Attributes{Desc: "DUT Port 3", IPv4: "198.51.100.5", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8:3::1", IPv6Len: ipv6PrefixLen},
		ate:  attrs.Attributes{Name: "port3", MAC: "02:00:03:01:01:01", IPv4: "198.51.100.6", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8:3::2", IPv6Len: ipv6PrefixLen},
	},
	{
		name: "port4",
		dut:  attrs.Attributes{Desc: "DUT Port 4", IPv4: "198.51.100.9", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8:4::1", IPv6Len: ipv6PrefixLen},
		ate:  attrs.Attributes{Name: "port4", MAC: "02:00:04:01:01:01", IPv4: "198.51.100.10", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8:4::2", IPv6Len: ipv6PrefixLen},
	},
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestGNMIIntfConfigImpactsGRIBINH(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	ni := deviations.DefaultNetworkInstance(dut)

	configureDUT(t, dut)
	top := configureATE(t, ate)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")

	dstMac := gnmi.Get(t, ate.OTG(), gnmi.OTG().Interface(portPairs[0].ate.Name+".Eth").Ipv4Neighbor(portPairs[0].dut.IPv4).LinkLayerAddress().State())
	configureFlows(t, ate, top, dstMac)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")

	// See: https://partnerissuetracker.corp.google.com/issues/422275961
	if deviations.DisableHardwareNexthopProxy(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			helpers.GnmiCLIConfig(t, dut, "ip hardware fib next-hop proxy disabled")
		default:
			t.Errorf("Deviation DisableHardwareNexthopProxy is not handled for the dut: %v", dut.Vendor())
		}
	}

	client := &gribi.Client{DUT: dut, FIBACK: !deviations.GRIBIRIBAckOnly(dut), Persistence: true}
	if err := client.Start(t); err != nil {
		t.Fatalf("gRIBI client could not start: %v", err)
	}
	defer client.Close(t)
	defer client.FlushAll(t)
	client.BecomeLeader(t)
	client.FlushAll(t)

	programECMPBaseline(t, dut, client, ni)
	verifyAFTCoverage(t, dut, ni)

	// flowNegDown/flowMTU start in their own subtests; flowMTU would otherwise skew ECMP checks.
	setFlowTransmit(t, ate, gosnappi.StateTrafficFlowTransmitState.START, flowECMPv4Name, flowECMPv6Name)
	defer ate.OTG().StopTraffic(t)
	defer restorePort2State(t, dut)

	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")

	t.Run("TE-1.7.1: Port Admin State Bounce Impact on gRIBI NextHop", func(t *testing.T) {
		testPortAdminStateBounce(t, dut, ate, ni, p2, p3, p4)
	})

	t.Run("TE-1.7.2: MTU Change Impact on gRIBI NextHop", func(t *testing.T) {
		testMTUChange(t, dut, p2, p3, p4)
	})

	t.Run("TE-1.7.3: Negative Test: Program gRIBI Route on Admin DOWN Interface", func(t *testing.T) {
		testNHOnDownInterface(t, dut, ate, client, ni, p2)
	})

	t.Run("TE-1.7.4: Negative Test: MTU Smaller Than Packet Size", func(t *testing.T) {
		testMTUSmallerThanPacket(t, dut, ate, p2)
	})
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	for _, pp := range portPairs {
		p := dut.Port(t, pp.name)
		gnmi.Replace(t, dut, d.Interface(p.Name()).Config(), pp.dut.NewOCInterface(p.Name(), dut))
		// gnmi.Replace re-enables switchport on Arista.
		if dut.Vendor() == ondatra.ARISTA {
			helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("interface %s\n no switchport\n", p.Name()))
		}
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, p.Name(), deviations.DefaultNetworkInstance(dut), 0)
		}
		if deviations.ExplicitPortSpeed(dut) {
			fptest.SetPortSpeed(t, p)
		}
	}
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()
	for _, pp := range portPairs {
		ateAttr := pp.ate
		dutAttr := pp.dut
		ateAttr.AddToOTG(top, ate.Port(t, pp.name), &dutAttr)
	}
	return top
}

func setFlowTransmit(t *testing.T, ate *ondatra.ATEDevice, state gosnappi.StateTrafficFlowTransmitStateEnum, flowNames ...string) {
	t.Helper()
	cs := gosnappi.NewControlState()
	cs.Traffic().FlowTransmit().SetState(state).SetFlowNames(flowNames)
	ate.OTG().SetControlState(t, cs)
}

func configureFlows(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, dstMac string) {
	t.Helper()
	srcPort := ate.Port(t, "port1")
	src := portPairs[0].ate

	newFlow := func(name string) gosnappi.Flow {
		f := top.Flows().Add().SetName(name)
		f.Metrics().SetEnable(true)
		f.Duration().Continuous()
		f.Rate().SetPps(trafficPPS)
		f.TxRx().Port().SetTxName(srcPort.ID())
		eth := f.Packet().Add().Ethernet()
		eth.Src().SetValue(src.MAC)
		eth.Dst().SetValue(dstMac)
		return f
	}

	v4 := newFlow(flowECMPv4Name)
	ip4 := v4.Packet().Add().Ipv4()
	ip4.Src().SetValue(src.IPv4)
	ip4.Dst().Increment().SetStart(ipv4BaseRoute).SetStep("0.0.0.1").SetCount(numV4Routes)

	v6 := newFlow(flowECMPv6Name)
	ip6 := v6.Packet().Add().Ipv6()
	ip6.Src().SetValue(src.IPv6)
	ip6.Dst().Increment().SetStart(ipv6BaseRoute).SetStep("::1").SetCount(numV6Routes)

	neg := newFlow(flowNegDownName)
	ipNeg := neg.Packet().Add().Ipv4()
	ipNeg.Src().SetValue(src.IPv4)
	ipNeg.Dst().SetValue(ipv4NegRoute)

	mtu := newFlow(flowMTUName)
	mtu.Size().SetFixed(pktSizeLarge)
	ipMTU := mtu.Packet().Add().Ipv4()
	ipMTU.Src().SetValue(src.IPv4)
	ipMTU.Dst().SetValue(ipv4MTURoute)
}

func configStaticArp(p string, ipv4addr string, macAddr string) *oc.Interface {
	i := &oc.Interface{Name: ygot.String(p)}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	n4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4().GetOrCreateNeighbor(ipv4addr)
	n4.LinkLayerAddress = ygot.String(macAddr)
	return i
}

func configStaticRouteAndARPForMagicIP(t *testing.T, dut *ondatra.DUTDevice, ni string) {
	t.Helper()
	sb := &gnmi.SetBatch{}
	nexthops := map[string]*oc.NetworkInstance_Protocol_Static_NextHop{}
	for idx, pp := range portPairs[1:] {
		p := dut.Port(t, pp.name)
		nexthops[strconv.Itoa(idx)] = &oc.NetworkInstance_Protocol_Static_NextHop{
			Index:        ygot.String(strconv.Itoa(idx)),
			InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{Interface: ygot.String(p.Name())},
		}
		gnmi.BatchUpdate(sb, gnmi.OC().Interface(p.Name()).Config(), configStaticArp(p.Name(), magicIP, magicMac))
	}
	sp := gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	gnmi.BatchUpdate(sb, sp.Static(magicIP+"/32").Config(), &oc.NetworkInstance_Protocol_Static{
		Prefix:  ygot.String(magicIP + "/32"),
		NextHop: nexthops,
	})
	sb.Set(t, dut)

	for _, pp := range portPairs[1:] {
		p := dut.Port(t, pp.name)
		if got := gnmi.Get(t, dut, gnmi.OC().Interface(p.Name()).Subinterface(0).Ipv4().Neighbor(magicIP).LinkLayerAddress().State()); got != magicMac {
			t.Errorf("Static ARP on %s for %s: got %q, want %q", p.Name(), magicIP, got, magicMac)
		}
	}
}

func programECMPBaseline(t *testing.T, dut *ondatra.DUTDevice, client *gribi.Client, ni string) {
	t.Helper()
	if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		configStaticRouteAndARPForMagicIP(t, dut, ni)
	}

	nh2Opts := nhOpts(t, dut, 1)
	wantResult := fluent.InstalledInFIB
	if deviations.GRIBIRIBAckOnly(dut) {
		wantResult = fluent.InstalledInRIB
	}
	client.AddNH(t, nh2ID, "MACwithInterface", ni, wantResult, nh2Opts)
	client.AddNH(t, nh3ID, "MACwithInterface", ni, wantResult, nhOpts(t, dut, 2))
	client.AddNH(t, nh4ID, "MACwithInterface", ni, wantResult, nhOpts(t, dut, 3))
	client.AddNHG(t, nhg1ID, map[uint64]uint64{nh2ID: 1, nh3ID: 1, nh4ID: 1}, ni, wantResult)
	client.AddNHG(t, nhgPort2OnlyID, map[uint64]uint64{nh2ID: 1}, ni, wantResult)

	v4Prefixes, err := iputil.GenerateIPsWithStep(ipv4BaseRoute, numV4Routes, "0.0.0.1")
	if err != nil {
		t.Fatalf("Could not generate IPv4 routes: %v", err)
	}
	v4Entries, v4Results := make([]fluent.GRIBIEntry, 0, len(v4Prefixes)), make([]*gribiclient.OpResult, 0, len(v4Prefixes))
	for _, ip := range v4Prefixes {
		v4Entries = append(v4Entries, fluent.IPv4Entry().WithPrefix(ip+"/32").WithNetworkInstance(ni).WithNextHopGroup(nhg1ID))
		v4Results = append(v4Results, fluent.OperationResult().WithIPv4Operation(ip+"/32").WithOperationType(constants.Add).WithProgrammingResult(wantResult).AsResult())
	}
	addEntriesBatched(t, client, v4Entries, v4Results)

	// ipv4NegRoute is left unprogrammed: TE-1.7.3 introduces it as new.
	client.AddIPv4(t, ipv4MTURoute+"/32", nhgPort2OnlyID, ni, ni, wantResult)

	v6Prefixes, err := iputil.GenerateIPv6s(net.ParseIP(ipv6BaseRoute), numV6Routes)
	if err != nil {
		t.Fatalf("Could not generate IPv6 routes: %v", err)
	}
	v6Entries, v6Results := make([]fluent.GRIBIEntry, 0, len(v6Prefixes)), make([]*gribiclient.OpResult, 0, len(v6Prefixes))
	for _, ip := range v6Prefixes {
		v6Entries = append(v6Entries, fluent.IPv6Entry().WithPrefix(ip+"/128").WithNetworkInstance(ni).WithNextHopGroup(nhg1ID))
		v6Results = append(v6Results, fluent.OperationResult().WithIPv6Operation(ip+"/128").WithOperationType(constants.Add).WithProgrammingResult(wantResult).AsResult())
	}
	addEntriesBatched(t, client, v6Entries, v6Results)
}

// nhOpts uses real/ARP-bound MACs, not README placeholders, so forwarded frames reach the ATE.
func nhOpts(t *testing.T, dut *ondatra.DUTDevice, idx int) *gribi.NHOptions {
	t.Helper()
	intf := dut.Port(t, portPairs[idx].name).Name()
	switch {
	case deviations.GRIBIMACOverrideStaticARPStaticRoute(dut):
		return &gribi.NHOptions{Interface: intf, Mac: magicMac, Dest: magicIP}
	case deviations.GRIBIMACOverrideWithStaticARP(dut):
		return &gribi.NHOptions{Interface: intf, Mac: portPairs[idx].ate.MAC, Dest: portPairs[idx].ate.IPv4}
	default:
		return &gribi.NHOptions{Interface: intf, Mac: portPairs[idx].ate.MAC}
	}
}

// Smaller batches avoid gRPC deadlines on slow DUTs.
const ipRouteBatchSize = 100

func verifyAFTCoverage(t *testing.T, dut *ondatra.DUTDevice, ni string) {
	t.Helper()
	v4Prefix := ipv4BaseRoute + "/32"
	if got := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(ni).Afts().Ipv4Entry(v4Prefix).State()).GetNextHopGroup(); got != nhg1ID {
		t.Errorf("AFT ipv4-entry %s next-hop-group: got %d, want %d", v4Prefix, got, nhg1ID)
	}
	v6Prefix := ipv6BaseRoute + "/128"
	if got := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(ni).Afts().Ipv6Entry(v6Prefix).State()).GetNextHopGroup(); got != nhg1ID {
		t.Errorf("AFT ipv6-entry %s next-hop-group: got %d, want %d", v6Prefix, got, nhg1ID)
	}
	nhg := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(ni).Afts().NextHopGroup(nhg1ID).State())
	for _, want := range []uint64{nh2ID, nh3ID, nh4ID} {
		if _, ok := nhg.NextHop[want]; !ok {
			t.Errorf("AFT next-hop-group %d: missing next-hop index %d, want present", nhg1ID, want)
		}
	}
}

func addEntriesBatched(t *testing.T, client *gribi.Client, entries []fluent.GRIBIEntry, results []*gribiclient.OpResult) {
	t.Helper()
	for i := 0; i < len(entries); i += ipRouteBatchSize {
		end := i + ipRouteBatchSize
		if end > len(entries) {
			end = len(entries)
		}
		client.AddEntries(t, entries[i:end], results[i:end])
	}
}

func setPortEnabled(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, enabled bool) {
	t.Helper()
	gnmi.Replace(t, dut, gnmi.OC().Interface(p.Name()).Enabled().Config(), enabled)
	want := oc.Interface_OperStatus_DOWN
	if enabled {
		want = oc.Interface_OperStatus_UP
	}
	gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), awaitTimeout, want)
}

func setMTU(t *testing.T, dut *ondatra.DUTDevice, intfName string, mtu uint16) {
	t.Helper()
	b := &gnmi.SetBatch{}
	cfgplugins.AddInterfaceMTUOps(b, dut, intfName, mtu, false)
	b.Set(t, dut)
}

func awaitMTU(t *testing.T, dut *ondatra.DUTDevice, intfName string, want uint16, timeout time.Duration) {
	t.Helper()
	path := gnmi.OC().Interface(intfName).Mtu().State()
	if deviations.OmitL2MTU(dut) {
		path = gnmi.OC().Interface(intfName).Subinterface(0).Ipv4().Mtu().State()
	}
	val, ok := gnmi.Watch(t, dut, path, timeout, func(v *ygnmi.Value[uint16]) bool {
		got, present := v.Val()
		return present && got == want
	}).Await(t)
	// Some DUTs never populate this leaf; fall back to trusting the Set().
	if !ok {
		got, _ := val.Val()
		t.Logf("Interface %s MTU state leaf did not confirm %d within %v (got %d; schema may not expose it here); trusting the earlier Set()", intfName, want, timeout, got)
		time.Sleep(convergeSettle)
	}
}

func restorePort2State(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p2 := dut.Port(t, "port2")
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Enabled().Config(), true)
	setMTU(t, dut, p2.Name(), mtuDefault)
}

func portOutPkts(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) uint64 {
	t.Helper()
	return gnmi.Get(t, dut, gnmi.OC().Interface(p.Name()).Counters().OutPkts().State())
}

func verifyPortTraffic(t *testing.T, dut *ondatra.DUTDevice, window time.Duration, want map[*ondatra.Port]bool) {
	t.Helper()
	before := make(map[*ondatra.Port]uint64, len(want))
	for p := range want {
		before[p] = portOutPkts(t, dut, p)
	}
	time.Sleep(window)
	deltas := make(map[*ondatra.Port]uint64, len(want))
	for p, wantTraffic := range want {
		delta := portOutPkts(t, dut, p) - before[p]
		deltas[p] = delta
		if wantTraffic && delta == 0 {
			t.Errorf("Port %s: got 0 new egress packets over %v, want > 0 (traffic flowing)", p.Name(), window)
		}
		if !wantTraffic && delta != 0 {
			t.Errorf("Port %s: got %d new egress packets over %v, want 0 (port should not carry traffic)", p.Name(), delta, window)
		}
	}
	verifyECMPDistribution(t, deltas, want)
}

func verifyECMPDistribution(t *testing.T, deltas map[*ondatra.Port]uint64, want map[*ondatra.Port]bool) {
	t.Helper()
	var active []*ondatra.Port
	var total uint64
	for p, wantTraffic := range want {
		if !wantTraffic {
			continue
		}
		active = append(active, p)
		total += deltas[p]
	}
	if len(active) < 2 || total == 0 {
		return
	}
	mean := float64(total) / float64(len(active))
	for _, p := range active {
		if diffPct := math.Abs(float64(deltas[p])-mean) / mean * 100; diffPct > ecmpHashTolerancePct {
			t.Errorf("Port %s: got %d egress packets (mean %.0f across %d active ports), deviates %.1f%% from even ECMP hash, want within %.0f%%", p.Name(), deltas[p], mean, len(active), diffPct, ecmpHashTolerancePct)
		}
	}
}

func flowCounters(t *testing.T, ate *ondatra.ATEDevice, flowName string) (tx, rx uint64) {
	t.Helper()
	fc := gnmi.OTG().Flow(flowName).Counters()
	return gnmi.Get(t, ate.OTG(), fc.OutPkts().State()), gnmi.Get(t, ate.OTG(), fc.InPkts().State())
}

func verifyFlowLoss(t *testing.T, ate *ondatra.ATEDevice, flowName string, window time.Duration, wantLossPct float64) uint64 {
	t.Helper()
	txBefore, rxBefore := flowCounters(t, ate, flowName)
	time.Sleep(window)
	txAfter, rxAfter := flowCounters(t, ate, flowName)
	dtx := txAfter - txBefore
	drx := rxAfter - rxBefore
	if dtx == 0 {
		t.Fatalf("Flow %s: no packets transmitted during window, cannot compute loss", flowName)
	}
	gotLossPct := float64(dtx-drx) / float64(dtx) * 100
	if diff := math.Abs(gotLossPct - wantLossPct); diff > lossTolerancePct {
		t.Errorf("Flow %s: got %.2f%% loss over %v, want %.2f%% (+/- %.2f%%)", flowName, gotLossPct, window, wantLossPct, lossTolerancePct)
	}
	return dtx - drx
}

// verifyFlowHealthy fails fast so carried-over failures are attributed to the earlier subtest.
func verifyFlowHealthy(t *testing.T, ate *ondatra.ATEDevice, flowName string, window time.Duration) {
	t.Helper()
	txBefore, rxBefore := flowCounters(t, ate, flowName)
	time.Sleep(window)
	txAfter, rxAfter := flowCounters(t, ate, flowName)
	dtx := txAfter - txBefore
	drx := rxAfter - rxBefore
	if dtx == 0 {
		t.Fatalf("Flow %s: no packets transmitted during precondition window, cannot verify traffic is flowing", flowName)
	}
	if gotLossPct := float64(dtx-drx) / float64(dtx) * 100; gotLossPct > lossTolerancePct {
		t.Fatalf("Flow %s: got %.2f%% loss before this subtest's fault injection, want ~0%%; traffic was not flowing steadily (likely carried over from an earlier subtest failure)", flowName, gotLossPct)
	}
}

func testPortAdminStateBounce(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, ni string, p2, p3, p4 *ondatra.Port) {
	ateP2 := ate.Port(t, "port2").ID()
	ateP3 := ate.Port(t, "port3").ID()
	ateP4 := ate.Port(t, "port4").ID()

	verifyPortAndATETraffic(t, dut, ate, monitorWindow,
		map[*ondatra.Port]bool{p2: true, p3: true, p4: true},
		map[string]bool{ateP2: true, ateP3: true, ateP4: true})

	setPortEnabled(t, dut, p2, false)
	t.Cleanup(func() { setPortEnabled(t, dut, p2, true) })
	verifyNHViaAFT(t, dut, ni, nh2ID)
	time.Sleep(convergeSettle)
	verifyPortAndATETraffic(t, dut, ate, monitorWindow,
		map[*ondatra.Port]bool{p2: false, p3: true, p4: true},
		map[string]bool{ateP2: false, ateP3: true, ateP4: true})

	// Threshold exceeds link-up control frames so only recovered data-plane traffic satisfies it.
	p2FramesDown := ateInFrames(t, ate, ateP2)
	setPortEnabled(t, dut, p2, true)
	verifyNHViaAFT(t, dut, ni, nh2ID)
	awaitCounterAbove(t, ate, gnmi.OTG().Port(ateP2).Counters().InFrames().State(), p2FramesDown+trafficPPS, "ATE port "+ateP2)
	verifyPortAndATETraffic(t, dut, ate, monitorWindow,
		map[*ondatra.Port]bool{p2: true, p3: true, p4: true},
		map[string]bool{ateP2: true, ateP3: true, ateP4: true})
}

func ateInFrames(t *testing.T, ate *ondatra.ATEDevice, portID string) uint64 {
	t.Helper()
	return gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(portID).Counters().InFrames().State())
}

func awaitCounterAbove(t *testing.T, ate *ondatra.ATEDevice, q ygnmi.SingletonQuery[uint64], threshold uint64, what string) {
	t.Helper()
	if _, ok := gnmi.Watch(t, ate.OTG(), q, awaitTimeout, func(v *ygnmi.Value[uint64]) bool {
		got, present := v.Val()
		return present && got > threshold
	}).Await(t); !ok {
		t.Errorf("%s: counter did not exceed %d within %v after re-enabling port2", what, threshold, awaitTimeout)
	}
}

func verifyPortAndATETraffic(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, window time.Duration, dutWant map[*ondatra.Port]bool, ateWant map[string]bool) {
	t.Helper()
	dutBefore := make(map[*ondatra.Port]uint64, len(dutWant))
	for p := range dutWant {
		dutBefore[p] = portOutPkts(t, dut, p)
	}
	ateBefore := make(map[string]uint64, len(ateWant))
	for portID := range ateWant {
		ateBefore[portID] = ateInFrames(t, ate, portID)
	}
	time.Sleep(window)
	dutDeltas := make(map[*ondatra.Port]uint64, len(dutWant))
	for p, wantTraffic := range dutWant {
		delta := portOutPkts(t, dut, p) - dutBefore[p]
		dutDeltas[p] = delta
		if wantTraffic && delta == 0 {
			t.Errorf("DUT port %s: got 0 new egress packets over %v, want > 0 (traffic flowing)", p.Name(), window)
		}
		if !wantTraffic && delta != 0 {
			t.Errorf("DUT port %s: got %d new egress packets over %v, want 0 (port should not carry traffic)", p.Name(), delta, window)
		}
	}
	verifyECMPDistribution(t, dutDeltas, dutWant)
	for portID, wantTraffic := range ateWant {
		delta := ateInFrames(t, ate, portID) - ateBefore[portID]
		if wantTraffic && delta == 0 {
			t.Errorf("ATE port %s: got 0 new ingress frames over %v, want > 0 (traffic received)", portID, window)
		}
		if !wantTraffic && delta != 0 {
			t.Errorf("ATE port %s: got %d new ingress frames over %v, want 0 (port should not receive traffic)", portID, delta, window)
		}
	}
}

// verifyNHViaAFT asserts state/index only; OC AFT has no vendor-neutral viability leaf, and absence is a valid rejection.
func verifyNHViaAFT(t *testing.T, dut *ondatra.DUTDevice, ni string, nhIndex uint64) {
	t.Helper()
	val, ok := gnmi.Watch(t, dut, gnmi.OC().NetworkInstance(ni).Afts().NextHop(nhIndex).State(), convergeSettle, func(v *ygnmi.Value[*oc.NetworkInstance_Afts_NextHop]) bool {
		_, present := v.Val()
		return present
	}).Await(t)
	nh, present := val.Val()
	if !present {
		t.Logf("AFT next-hop %d: not present within %v via ON_CHANGE (not installed or rejected)", nhIndex, convergeSettle)
		return
	}
	if got := nh.GetIndex(); got != nhIndex {
		t.Errorf("AFT next-hop %d state/index: got %d, want %d", nhIndex, got, nhIndex)
	}
	t.Logf("AFT next-hop %d telemetry via ON_CHANGE subscription (stabilized=%v): %+v", nhIndex, ok, nh)
}

func testMTUChange(t *testing.T, dut *ondatra.DUTDevice, p2, p3, p4 *ondatra.Port) {
	setMTU(t, dut, p2.Name(), mtuJumbo)
	t.Cleanup(func() {
		setMTU(t, dut, p2.Name(), mtuDefault)
		awaitMTU(t, dut, p2.Name(), mtuDefault, awaitTimeout)
	})
	awaitMTU(t, dut, p2.Name(), mtuJumbo, awaitTimeout)
	verifyPortTraffic(t, dut, monitorWindow, map[*ondatra.Port]bool{p2: true, p3: true, p4: true})

	setMTU(t, dut, p2.Name(), mtuDefault)
	awaitMTU(t, dut, p2.Name(), mtuDefault, awaitTimeout)
	verifyPortTraffic(t, dut, monitorWindow, map[*ondatra.Port]bool{p2: true, p3: true, p4: true})
}

func testNHOnDownInterface(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, client *gribi.Client, ni string, p2 *ondatra.Port) {
	setPortEnabled(t, dut, p2, false)
	t.Cleanup(func() { setPortEnabled(t, dut, p2, true) })

	nh, _ := gribi.NHEntry(nhDownID, "MACwithInterface", ni, fluent.InstalledInFIB, nhOpts(t, dut, 1))
	nhg, _ := gribi.NHGEntry(nhgDownID, map[uint64]uint64{nhDownID: 1}, ni, fluent.InstalledInFIB)
	ipEntry := fluent.IPv4Entry().WithPrefix(ipv4NegRoute + "/32").WithNetworkInstance(ni).WithNextHopGroup(nhgDownID)

	client.Fluent(t).Modify().AddEntry(t, nh, nhg, ipEntry)
	ctx, cancel := context.WithTimeout(context.Background(), awaitTimeout)
	defer cancel()
	if err := client.Fluent(t).Await(ctx, t); err != nil {
		t.Logf("gRIBI programming on down port2 did not fully converge (vendor-variable, expected): %v", err)
	}
	for _, res := range client.Fluent(t).Results(t) {
		t.Logf("gRIBI result for down-port2 NH/NHG/IPv4 programming: %+v", res)
	}

	verifyNHViaAFT(t, dut, ni, nhDownID)

	// Unviability is proven by the 100% loss below, since OC AFT has no vendor-neutral viability leaf.
	setFlowTransmit(t, ate, gosnappi.StateTrafficFlowTransmitState.START, flowNegDownName)
	defer setFlowTransmit(t, ate, gosnappi.StateTrafficFlowTransmitState.STOP, flowNegDownName)
	verifyFlowLoss(t, ate, flowNegDownName, monitorWindow, 100)

	_, rxDown := flowCounters(t, ate, flowNegDownName)
	setPortEnabled(t, dut, p2, true)
	awaitCounterAbove(t, ate, gnmi.OTG().Flow(flowNegDownName).Counters().InPkts().State(), rxDown, "Flow "+flowNegDownName)
	ateP2 := ate.Port(t, "port2").ID()
	ateP2Before := ateInFrames(t, ate, ateP2)
	verifyFlowLoss(t, ate, flowNegDownName, monitorWindow, 0)
	if got := ateInFrames(t, ate, ateP2) - ateP2Before; got == 0 {
		t.Errorf("ATE port %s: got 0 new ingress frames over %v after re-enabling port2, want > 0 (recovered traffic arriving on port-2)", ateP2, monitorWindow)
	}
}

func testMTUSmallerThanPacket(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, p2 *ondatra.Port) {
	// Started before the MTU change: Step 1 checks all prefixes flow, and Step 4's "drops to 0" needs a live baseline.
	setFlowTransmit(t, ate, gosnappi.StateTrafficFlowTransmitState.START, flowMTUName)
	defer setFlowTransmit(t, ate, gosnappi.StateTrafficFlowTransmitState.STOP, flowMTUName)

	verifyFlowHealthy(t, ate, flowMTUName, monitorWindow)

	errBefore, counterName := errorFrameCounter(t, dut, p2.Name())

	setMTU(t, dut, p2.Name(), mtuTooSmall)
	t.Cleanup(func() {
		setMTU(t, dut, p2.Name(), mtuDefault)
		awaitMTU(t, dut, p2.Name(), mtuDefault, awaitTimeout)
	})
	awaitMTU(t, dut, p2.Name(), mtuTooSmall, awaitTimeout)

	dropped := verifyFlowLoss(t, ate, flowMTUName, monitorWindow, 100)

	errAfter, counterNameAfter := errorFrameCounter(t, dut, p2.Name())
	if counterNameAfter != counterName {
		t.Fatalf("Interface %s error counter source changed between reads (%s -> %s), cannot compute increment", p2.Name(), counterName, counterNameAfter)
	}
	errDelta := errAfter - errBefore
	if errDelta == 0 {
		t.Errorf("Interface %s %s: got 0 increment, want ~%d (dropped oversized packets)", p2.Name(), counterName, dropped)
	}
	// Not asserted equal: drops occur at port2 egress, while README's counters are ingress-side and vendor-variable.
	t.Logf("Interface %s %s increment: %d, ATE-measured dropped packets: %d", p2.Name(), counterName, errDelta, dropped)

	setMTU(t, dut, p2.Name(), mtuDefault)
	awaitMTU(t, dut, p2.Name(), mtuDefault, awaitTimeout)
	verifyFlowLoss(t, ate, flowMTUName, monitorWindow, 0)
}

// in-oversize-frames isn't in the generated OC schema, so read it raw; fall back to in-errors.
func errorFrameCounter(t *testing.T, dut *ondatra.DUTDevice, intfName string) (uint64, string) {
	t.Helper()
	req := &gpb.GetRequest{
		Path: []*gpb.Path{{
			Elem: []*gpb.PathElem{
				{Name: "interfaces"},
				{Name: "interface", Key: map[string]string{"name": intfName}},
				{Name: "ethernet"},
				{Name: "state"},
				{Name: "counters"},
				{Name: "in-oversize-frames"},
			},
		}},
		Type:     gpb.GetRequest_STATE,
		Encoding: gpb.Encoding_JSON_IETF,
	}
	if resp, err := dut.RawAPIs().GNMI(t).Get(context.Background(), req); err == nil {
		for _, notif := range resp.GetNotification() {
			for _, upd := range notif.GetUpdate() {
				if jsonVal := upd.GetVal().GetJsonIetfVal(); len(jsonVal) > 0 {
					var n uint64
					if err := json.Unmarshal(jsonVal, &n); err == nil {
						return n, "in-oversize-frames"
					}
				}
				if v := upd.GetVal().GetUintVal(); v != 0 {
					return v, "in-oversize-frames"
				}
			}
		}
	}
	t.Logf("in-oversize-frames leaf unsupported by DUT/schema on %s; falling back to in-errors per README", intfName)
	return gnmi.Get(t, dut, gnmi.OC().Interface(intfName).Counters().InErrors().State()), "in-errors"
}
