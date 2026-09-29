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
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126

	// gRIBI NextHop/NextHopGroup indices for the baseline ECMP group (port2/3/4).
	nh2ID  = 10
	nh3ID  = 11
	nh4ID  = 12
	nhg1ID = 100

	// gRIBI NextHop/NextHopGroup indices used only by TE-1.7.3 (NH programmed on a down port).
	nhDownID  = 20
	nhgDownID = 20

	// NHG containing only NH2 (port2), used by TE-1.7.4 so that prefix routes solely via
	// port2 instead of being spread across port2/3/4 by the shared ECMP NHG.
	nhgPort2OnlyID = 30

	// Number of gRIBI IPv4/IPv6 entries programmed into the ECMP NHG.
	numV4Routes = 1000
	numV6Routes = 1000

	ipv4BaseRoute = "203.0.113.1"   // first of numV4Routes consecutive /32s in the ECMP NHG.
	ipv6BaseRoute = "2001:db8:a::1" // first of numV6Routes consecutive /128s in the ECMP NHG.
	ipv4NegRoute  = "203.0.113.255" // dedicated prefix for TE-1.7.3.
	ipv4MTURoute  = "203.0.113.254" // dedicated prefix for TE-1.7.4.

	flowECMPv4Name  = "flowECMPv4"
	flowECMPv6Name  = "flowECMPv6"
	flowNegDownName = "flowNegDown"
	flowMTUName     = "flowMTU"

	trafficPPS    = 1000
	monitorWindow = 30 * time.Second
	awaitTimeout  = time.Minute

	mtuDefault   = 1500
	mtuJumbo     = 9000
	mtuTooSmall  = 500
	pktSizeLarge = 1500

	lossTolerancePct = 1.0 // percent
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

	// The dst MAC for the src-facing flows is the DUT port1 MAC as resolved by ARP.
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

	client := &gribi.Client{DUT: dut, FIBACK: true, Persistence: true}
	if err := client.Start(t); err != nil {
		t.Fatalf("gRIBI client could not start: %v", err)
	}
	defer client.Close(t)
	defer client.FlushAll(t)
	client.BecomeLeader(t)
	client.FlushAll(t)

	programECMPBaseline(t, dut, client, ni)

	ate.OTG().StartTraffic(t)
	defer ate.OTG().StopTraffic(t)
	defer restorePort2State(t, dut)

	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")

	t.Run("TE-1.7.1: Port Admin State Bounce Impact on gRIBI NextHop", func(t *testing.T) {
		testPortAdminStateBounce(t, dut, p2, p3, p4)
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

// configureDUT configures IPv4/IPv6 addressing on all 4 DUT ports.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()
	// Diagnostic: force a hardware TCAM/system profile (re)initialization, mirroring
	// TE-18.3's setup, to test whether the DUT's default profile is what's blocking
	// gRIBI FIB programming (not because this test needs VRF selection itself).
	if dut.Vendor() == ondatra.ARISTA {
		if hwCfg := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeatureVrfSelectionExtended); hwCfg != "" {
			cfgplugins.PushDUTHardwareInitConfig(t, dut, hwCfg)
		}
	}
	// Diagnostic: explicitly register the default NI's type via OC, mirroring TE-18.3,
	// in case the FIB agent needs this before it will install entries in that NI.
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	for _, pp := range portPairs {
		p := dut.Port(t, pp.name)
		gnmi.Replace(t, dut, d.Interface(p.Name()).Config(), pp.dut.NewOCInterface(p.Name(), dut))
		// gnmi.Replace resets unmodeled attributes to platform default, re-enabling switchport; clear it after.
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

// configureATE adds the 4 ATE ports/devices with IPv4/IPv6 addressing (no flows yet).
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

// configureFlows adds the 4 traffic flows (baseline ECMP v4/v6, plus the 2 dedicated
// single-destination flows used by the TE-1.7.3/TE-1.7.4 negative subtests), all
// sourced from ATE port1 and left running continuously.
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

// programECMPBaseline programs NH10/11/12 -> port2/3/4 (using MACwithInterface, since
// Arista rejects a MAC without an accompanying interface reference), an ECMP NHG100
// over them, and numV4Routes/numV6Routes IPv4/IPv6 entries pointing at NHG100. It also
// programs the TE-1.7.3 dedicated prefix onto NHG100 (until that subtest reprograms it)
// and the TE-1.7.4 dedicated prefix onto a port2-only NHG, since that subtest requires
// traffic solely destined via port2.
func programECMPBaseline(t *testing.T, dut *ondatra.DUTDevice, client *gribi.Client, ni string) {
	t.Helper()
	// nhOpts builds the NH options for portPairs[idx], adding Dest (the ATE's already
	// ARP-resolved IP) when the DUT can't install a MAC-only next-hop-entry.
	nhOpts := func(idx int) *gribi.NHOptions {
		opt := &gribi.NHOptions{Interface: dut.Port(t, portPairs[idx].name).Name(), Mac: portPairs[idx].ate.MAC}
		if deviations.GRIBIMACOverrideWithStaticARP(dut) {
			opt.Dest = portPairs[idx].ate.IPv4
		}
		return opt
	}
	client.AddNH(t, nh2ID, "MACwithInterface", ni, fluent.InstalledInFIB, nhOpts(1))
	client.AddNH(t, nh3ID, "MACwithInterface", ni, fluent.InstalledInFIB, nhOpts(2))
	client.AddNH(t, nh4ID, "MACwithInterface", ni, fluent.InstalledInFIB, nhOpts(3))
	client.AddNHG(t, nhg1ID, map[uint64]uint64{nh2ID: 1, nh3ID: 1, nh4ID: 1}, ni, fluent.InstalledInFIB)
	client.AddNHG(t, nhgPort2OnlyID, map[uint64]uint64{nh2ID: 1}, ni, fluent.InstalledInFIB)

	v4Prefixes, err := iputil.GenerateIPsWithStep(ipv4BaseRoute, numV4Routes, "0.0.0.1")
	if err != nil {
		t.Fatalf("Could not generate IPv4 routes: %v", err)
	}
	for _, ip := range v4Prefixes {
		client.AddIPv4(t, ip+"/32", nhg1ID, ni, ni, fluent.InstalledInFIB)
	}

	client.AddIPv4(t, ipv4NegRoute+"/32", nhg1ID, ni, ni, fluent.InstalledInFIB)
	client.AddIPv4(t, ipv4MTURoute+"/32", nhgPort2OnlyID, ni, ni, fluent.InstalledInFIB)

	v6Prefixes, err := iputil.GenerateIPv6s(net.ParseIP(ipv6BaseRoute), numV6Routes)
	if err != nil {
		t.Fatalf("Could not generate IPv6 routes: %v", err)
	}
	for _, ip := range v6Prefixes {
		client.AddIPv6(t, ip+"/128", nhg1ID, ni, ni, fluent.InstalledInFIB)
	}
}

// setPortEnabled sets the interface admin state and awaits the corresponding oper-status.
func setPortEnabled(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, enabled bool) {
	t.Helper()
	gnmi.Replace(t, dut, gnmi.OC().Interface(p.Name()).Enabled().Config(), enabled)
	want := oc.Interface_OperStatus_DOWN
	if enabled {
		want = oc.Interface_OperStatus_UP
	}
	gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), awaitTimeout, want)
}

// setMTU replaces the interface MTU config, picking the L2 or L3 MTU leaf per deviation.
func setMTU(t *testing.T, dut *ondatra.DUTDevice, intfName string, mtu uint16) {
	t.Helper()
	b := &gnmi.SetBatch{}
	cfgplugins.AddInterfaceMTUOps(b, dut, intfName, mtu, false)
	b.Set(t, dut)
}

// getMTUState reads back the interface MTU state, matching the leaf setMTU wrote to.
func getMTUState(t *testing.T, dut *ondatra.DUTDevice, intfName string) uint16 {
	t.Helper()
	if deviations.OmitL2MTU(dut) {
		return gnmi.Get(t, dut, gnmi.OC().Interface(intfName).Subinterface(0).Ipv4().Mtu().State())
	}
	return gnmi.Get(t, dut, gnmi.OC().Interface(intfName).Mtu().State())
}

// awaitMTU polls getMTUState until it matches want or timeout elapses.
func awaitMTU(t *testing.T, dut *ondatra.DUTDevice, intfName string, want uint16, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if got := getMTUState(t, dut, intfName); got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("Interface %s MTU state did not converge to %d within %v", intfName, want, timeout)
		}
		time.Sleep(time.Second)
	}
}

// restorePort2State makes a best-effort attempt to leave port2 enabled with the
// default MTU, regardless of which subtests ran or failed.
func restorePort2State(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p2 := dut.Port(t, "port2")
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Enabled().Config(), true)
	setMTU(t, dut, p2.Name(), mtuDefault)
}

// portOutPkts returns the DUT's cumulative egress packet count for the given port.
func portOutPkts(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) uint64 {
	t.Helper()
	return gnmi.Get(t, dut, gnmi.OC().Interface(p.Name()).Counters().OutPkts().State())
}

// verifyPortTraffic checks, over window, whether each port in want received new egress
// packets (true) or none (false). ECMP hashing itself is not asserted, only presence.
func verifyPortTraffic(t *testing.T, dut *ondatra.DUTDevice, window time.Duration, want map[*ondatra.Port]bool) {
	t.Helper()
	before := make(map[*ondatra.Port]uint64, len(want))
	for p := range want {
		before[p] = portOutPkts(t, dut, p)
	}
	time.Sleep(window)
	for p, wantTraffic := range want {
		delta := portOutPkts(t, dut, p) - before[p]
		if wantTraffic && delta == 0 {
			t.Errorf("Port %s: got 0 new egress packets over %v, want > 0 (traffic flowing)", p.Name(), window)
		}
		if !wantTraffic && delta != 0 {
			t.Errorf("Port %s: got %d new egress packets over %v, want 0 (port should not carry traffic)", p.Name(), delta, window)
		}
	}
}

// flowCounters returns the cumulative Tx/Rx packet counts for an OTG flow.
func flowCounters(t *testing.T, ate *ondatra.ATEDevice, flowName string) (tx, rx uint64) {
	t.Helper()
	fc := gnmi.OTG().Flow(flowName).Counters()
	return gnmi.Get(t, ate.OTG(), fc.OutPkts().State()), gnmi.Get(t, ate.OTG(), fc.InPkts().State())
}

// verifyFlowLoss checks a flow's packet loss percentage over window (using deltas so
// state from before the window, e.g. an earlier down period, isn't counted).
func verifyFlowLoss(t *testing.T, ate *ondatra.ATEDevice, flowName string, window time.Duration, wantLossPct float64) {
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
}

func testPortAdminStateBounce(t *testing.T, dut *ondatra.DUTDevice, p2, p3, p4 *ondatra.Port) {
	setPortEnabled(t, dut, p2, false)
	verifyPortTraffic(t, dut, monitorWindow, map[*ondatra.Port]bool{p2: false, p3: true, p4: true})

	setPortEnabled(t, dut, p2, true)
	verifyPortTraffic(t, dut, monitorWindow, map[*ondatra.Port]bool{p2: true, p3: true, p4: true})
}

func testMTUChange(t *testing.T, dut *ondatra.DUTDevice, p2, p3, p4 *ondatra.Port) {
	setMTU(t, dut, p2.Name(), mtuJumbo)
	awaitMTU(t, dut, p2.Name(), mtuJumbo, awaitTimeout)
	verifyPortTraffic(t, dut, monitorWindow, map[*ondatra.Port]bool{p2: true, p3: true, p4: true})

	setMTU(t, dut, p2.Name(), mtuDefault)
	awaitMTU(t, dut, p2.Name(), mtuDefault, awaitTimeout)
	verifyPortTraffic(t, dut, monitorWindow, map[*ondatra.Port]bool{p2: true, p3: true, p4: true})
}

// testNHOnDownInterface programs a new NH/NHG/route pointing solely at the already-down
// port2 and checks the hard, vendor-independent pass/fail signal from the README (100%
// traffic loss), rather than asserting a specific gRIBI programming/telemetry outcome
// which the README itself says is vendor-variable ("rejected or ... unviable").
func testNHOnDownInterface(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, client *gribi.Client, ni string, p2 *ondatra.Port) {
	setPortEnabled(t, dut, p2, false)

	nh, _ := gribi.NHEntry(nhDownID, portPairs[1].ate.IPv4, ni, fluent.InstalledInFIB)
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

	verifyFlowLoss(t, ate, flowNegDownName, monitorWindow, 100)

	setPortEnabled(t, dut, p2, true)
	verifyFlowLoss(t, ate, flowNegDownName, monitorWindow, 0)
}

func testMTUSmallerThanPacket(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, p2 *ondatra.Port) {
	setMTU(t, dut, p2.Name(), mtuTooSmall)
	awaitMTU(t, dut, p2.Name(), mtuTooSmall, awaitTimeout)

	verifyFlowLoss(t, ate, flowMTUName, monitorWindow, 100)

	got := getOversizeFrameCounter(t, dut, p2.Name())
	if got == 0 {
		t.Errorf("Interface %s in-oversize-frames counter got 0, want > 0 after sending oversized frames", p2.Name())
	}
	t.Logf("Interface %s in-oversize-frames counter: %d", p2.Name(), got)

	setMTU(t, dut, p2.Name(), mtuDefault)
	awaitMTU(t, dut, p2.Name(), mtuDefault, awaitTimeout)
	verifyFlowLoss(t, ate, flowMTUName, monitorWindow, 0)
}

// getOversizeFrameCounter fetches the README's literal
// /interfaces/interface/ethernet/state/counters/in-oversize-frames leaf via a raw gNMI
// Get. This leaf is NOT modeled in this repo's generated OC schema (only InErrors,
// InFragmentFrames, InJabberFrames exist for related counters), so it cannot be read via
// the typed gnmi.OC() path helpers. Rather than silently substituting a different
// counter, this fails loudly if the DUT/schema doesn't support it, so the gap is
// immediately visible and reportable.
func getOversizeFrameCounter(t *testing.T, dut *ondatra.DUTDevice, intfName string) uint64 {
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
	resp, err := dut.RawAPIs().GNMI(t).Get(context.Background(), req)
	if err != nil {
		t.Fatalf("in-oversize-frames leaf unsupported by DUT/schema (raw gNMI Get failed) on %s: %v", intfName, err)
	}
	for _, notif := range resp.GetNotification() {
		for _, upd := range notif.GetUpdate() {
			if jsonVal := upd.GetVal().GetJsonIetfVal(); len(jsonVal) > 0 {
				var n uint64
				if err := json.Unmarshal(jsonVal, &n); err != nil {
					t.Fatalf("Could not parse in-oversize-frames value on %s: %v", intfName, err)
				}
				return n
			}
			if upd.GetVal().GetUintVal() != 0 {
				return upd.GetVal().GetUintVal()
			}
		}
	}
	t.Fatalf("in-oversize-frames leaf unsupported by DUT/schema: empty gNMI response for %s", intfName)
	return 0
}
