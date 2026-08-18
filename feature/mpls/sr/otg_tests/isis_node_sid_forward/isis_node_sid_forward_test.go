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

// Package isis_node_sid_forward is a test to verify ISIS Node-SID forwarding
package isis_node_sid_forward_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/isissession"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/ygnmi/ygnmi"
)

// TestMain initializes the testbed and runs the tests
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Constants
const (
	srgbLowerBound           = 400000
	srgbUpperBound           = 465001
	srgbLocalID              = "100.1.1.1"
	srlbLocalID              = "200.1.1.1"
	plenIPv4                 = 30
	plenIPv6                 = 126
	ateV4Route               = "203.0.113.0"
	ateV6Route               = "2001:db8::203:0:113:0"
	v4IP                     = "203.0.113.1"
	v6IP                     = "2001:db8::203:0:113:1"
	v4Route                  = "203.0.113.0"
	v6Route                  = "2001:db8::203:0:113:0"
	ateV4Metric              = 200
	ateV6Metric              = 200
	v4NetName                = "isisv4Net"
	v6NetName                = "isisv6Net"
	v4FlowName               = "v4Flow"
	v6FlowName               = "v6Flow"
	SRReservedLabelblockName = "default-srgb" // supported name for Cisco SRGB
	fixedPackets             = 1000
)

var (
	srgbMplsLabelBlockName = "400000 465001"
	customLabelBlockName   = "99.99.99.99"
)

// Configure ISIS, MPLS and ISIS-SR on DUT
func configureISISSegmentRouting(t *testing.T, ts *isissession.TestSession, dut *ondatra.DUTDevice) {
	t.Helper()
	d := ts.DUTConf
	networkInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(ts.DUT))
	prot := networkInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isissession.ISISName)
	prot.Enabled = ygot.Bool(true)

	// Configure MPLS
	mplsCfg := networkInstance.GetOrCreateMpls().GetOrCreateGlobal()
	mplsCfg.GetOrCreateInterface(ts.DUTPort1.Name())
	mplsCfg.GetOrCreateInterface(ts.DUTPort2.Name())
	mplsCfg.GetOrCreateReservedLabelBlock(srgbMplsLabelBlockName).LowerBound = oc.UnionUint32(srgbLowerBound)
	mplsCfg.GetOrCreateReservedLabelBlock(srgbMplsLabelBlockName).UpperBound = oc.UnionUint32(srgbUpperBound)
	// Cisco requires the reserved label block name to be "default-srgb"
	switch dut.Vendor() {
	case ondatra.CISCO:
		mplsCfg.GetOrCreateReservedLabelBlock(SRReservedLabelblockName).LocalId = ygot.String(SRReservedLabelblockName)
	default:
	}

	mplsCfgIntf := mplsCfg.GetOrCreateInterface(ts.DUTPort1.Name())
	mplsCfgIntf.InterfaceId = ygot.String(ts.DUTPort1.Name())

	// Configure SR
	srCfg := networkInstance.GetOrCreateSegmentRouting()
	srgb := srCfg.GetOrCreateSrgb(customLabelBlockName)
	srgb.LocalId = ygot.String(customLabelBlockName)
	srgb.SetMplsLabelBlocks([]string{srgbMplsLabelBlockName})

	// ISIS Segment Routing configurations
	isisSR := prot.GetOrCreateIsis().GetOrCreateGlobal().GetOrCreateSegmentRouting()
	isisSR.SetSrgb(customLabelBlockName)
	isisSR.Enabled = ygot.Bool(true)
}

func configureOTG(t *testing.T, ts *isissession.TestSession) {
	t.Helper()

	// netv4 is a simulated network containing the ipv4 addresses specified by targetNetwork
	netv4 := ts.ATEIntf1.Isis().V4Routes().Add().SetName(v4NetName).SetLinkMetric(ateV4Metric)
	netv4.Addresses().Add().SetAddress(v4Route).SetPrefix(uint32(isissession.ATEISISAttrs.IPv4Len))

	// netv6 is a simulated network containing the ipv6 addresses specified by targetNetwork
	netv6 := ts.ATEIntf1.Isis().V6Routes().Add().SetName(v6NetName).SetLinkMetric(ateV6Metric)
	netv6.Addresses().Add().SetAddress(v6Route).SetPrefix(uint32(isissession.ATEISISAttrs.IPv6Len))

	// We generate traffic entering along port2 and destined for port1
	srcIpv4 := ts.ATEIntf2.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	srcIpv6 := ts.ATEIntf2.Ethernets().Items()[0].Ipv6Addresses().Items()[0]

	t.Log("Configuring v4 traffic flow ")

	v4Flow := ts.ATETop.Flows().Add().SetName(v4FlowName)
	v4Flow.Metrics().SetEnable(true)

	v4Flow.TxRx().Device().
		SetTxNames([]string{srcIpv4.Name()}).
		SetRxNames([]string{v4NetName})

	v4Flow.Duration().FixedPackets().SetPackets(fixedPackets)
	v4Flow.Size().SetFixed(512)
	v4Flow.Rate().SetPps(100)

	e1 := v4Flow.Packet().Add().Ethernet()
	e1.Src().SetValue(isissession.ATEISISAttrs.MAC)

	v4 := v4Flow.Packet().Add().Ipv4()
	v4.Src().SetValue(isissession.ATEISISAttrs.IPv4)
	v4.Dst().SetValue(ateV4Route)

	t.Log("Configuring v6 traffic flow ")

	v6Flow := ts.ATETop.Flows().Add().SetName(v6FlowName)
	v6Flow.Metrics().SetEnable(true)

	v6Flow.TxRx().Device().
		SetTxNames([]string{srcIpv6.Name()}).
		SetRxNames([]string{v6NetName})

	v6Flow.Duration().FixedPackets().SetPackets(fixedPackets)
	v6Flow.Size().SetFixed(512)
	v6Flow.Rate().SetPps(100)

	e2 := v6Flow.Packet().Add().Ethernet()
	e2.Src().SetValue(isissession.ATEISISAttrs.MAC)
	v6 := v6Flow.Packet().Add().Ipv6()
	v6.Src().SetValue(isissession.ATEISISAttrs.IPv6)
	v6.Dst().SetValue(ateV6Route)
}

func verifyMPLSSR(t *testing.T, ts *isissession.TestSession) {
	t.Helper()
	d := ts.DUTConf
	networkInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(ts.DUT))
	routing := networkInstance.GetProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isissession.ISISName)

	t.Run("Segment Routing state checks - SR, SRGB and SRLB", func(t *testing.T) {
		SREnabled := routing.GetIsis().GetGlobal().GetSegmentRouting().GetEnabled()
		if !SREnabled {
			t.Errorf("FAIL - Segment Routing is not enabled on DUT")
		}

		if deviations.SrIgpConfigUnsupported(ts.DUT) {
			t.Log("Skipping Protocol Checks as SR IGP Configuration is not required or supported")
		} else {
			srgbValue := routing.GetIsis().GetGlobal().GetSegmentRouting().GetSrgb()
			if srgbValue == "nil" || srgbValue == "" {
				t.Errorf("FAIL- SRGB is not present on DUT")
			} else {
				t.Logf("SRGB is present on DUT value: %s", srgbValue)
			}
		}

		mplsprot := networkInstance.GetOrCreateMpls().GetOrCreateGlobal()

		if got := mplsprot.GetReservedLabelBlock(srgbMplsLabelBlockName).GetLowerBound(); got != oc.UnionUint32(srgbLowerBound) {
			t.Errorf("FAIL- SR Reserved Block is not present on DUT, got %d, want %d", got, srgbLowerBound)
		} else {
			t.Logf("SR Reserved Block is present on DUT value: %d, want %d", got, srgbLowerBound)
		}
		if got := mplsprot.GetReservedLabelBlock(srgbMplsLabelBlockName).GetUpperBound(); got != oc.UnionUint32(srgbUpperBound) {
			t.Errorf("FAIL- SR Reserved Block is not present on DUT, got %d, want %d", got, srgbUpperBound)
		} else {
			t.Logf("SR Reserved Block is present on DUT value: %d, want %d", got, srgbUpperBound)
		}

	})
}

func verifySRCounters(t *testing.T, ts *isissession.TestSession, ate *ondatra.ATEDevice) {
	d := ts.DUTConf
	networkInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(ts.DUT))

	otgutils.ExpectedTrafficLoss(t, ate.OTG(), v4FlowName, 0, 0.99)

	recvMetricV4 := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(v4FlowName).State())
	txPkts := recvMetricV4.GetCounters().GetOutPkts()
	rxPkts := recvMetricV4.GetCounters().GetInPkts()
	v4InPkts := rxPkts
	v4OutPkts := txPkts
	// Get SR Counters
	srSgProto := networkInstance.GetOrCreateMpls().GetOrCreateSignalingProtocols().GetSegmentRouting()
	srIntf := srSgProto.GetOrCreateInterface(ts.DUTPort1.Name())
	t.Logf("SR InPkts: %d, SR OutPkts: %d", srIntf.InPkts, srIntf.OutPkts)
	t.Logf("InPkts: %d, OutPkts: %d", v4InPkts, v4OutPkts)

	if got := srIntf.InPkts; got != ygot.Uint64(0) {
		t.Errorf("FAIL- SR InPkts is not zero, got %d, want %d", got, v4InPkts)
	}
	if got := srIntf.OutPkts; got != ygot.Uint64(0) {
		t.Errorf("FAIL- SR OutPkts is not zero, got %d, want %d", got, v4OutPkts)
	}
}

// waitForFIBProgramming watches the AFT to guarantee the route is in the data plane.
func waitForFIBProgramming(t *testing.T, dut *ondatra.DUTDevice, ipv4 bool, prefix string) {
	t.Helper()
	t.Logf("Waiting for prefixes to be programmed in the hardware FIB (AFT)...")

	netInst := deviations.DefaultNetworkInstance(dut)
	if ipv4 {
		// Watch IPv4 AFT Entry
		ipv4AftPath := gnmi.OC().NetworkInstance(netInst).Afts().Ipv4Entry(prefix).State()
		_, ok4 := gnmi.Watch(t, dut, ipv4AftPath, time.Minute, func(v *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
			return v.IsPresent()
		}).Await(t)
		if !ok4 {
			t.Fatalf("IPv4 Prefix %s was not programmed into the hardware FIB within the timeout", prefix)
		}
	} else {
		// Watch IPv6 AFT Entry
		ipv6AftPath := gnmi.OC().NetworkInstance(netInst).Afts().Ipv6Entry(prefix).State()
		_, ok6 := gnmi.Watch(t, dut, ipv6AftPath, time.Minute, func(v *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool {
			return v.IsPresent()
		}).Await(t)
		if !ok6 {
			t.Fatalf("IPv6 Prefix %s was not programmed into the hardware FIB within the timeout", prefix)
		}
	}
	t.Log("Hardware FIB programming confirmed.")
}

func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) {
	defer otgutils.LogFlowMetrics(t, ate.OTG(), top)
	defer otgutils.LogPortMetrics(t, ate.OTG(), top)
	for _, flowName := range []string{v4FlowName, v6FlowName} {
		otgutils.ExpectedTrafficLoss(t, ate.OTG(), flowName, 0, 0.99)
	}
}

// TestMPLSLabelBlockWithISIS verifies MPLS label block SRGB on DUT.
func TestMPLSLabelBlockWithISIS(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ts := isissession.MustNew(t).WithISIS()
	switch ts.DUT.Vendor() {
	case ondatra.CISCO:
		srgbMplsLabelBlockName = SRReservedLabelblockName
		customLabelBlockName = SRReservedLabelblockName
	default:
	}
	configureISISSegmentRouting(t, ts, ts.DUT)
	switch deviations.IsisSrgbSrlbUnsupported(ts.DUT) {
	case true:
		// Configures SRGB via network-instance/mpls/global/ OC Path as SR-IGP Not needed or supported
		t.Log("configure SR label block via network-instance/mpls/global/ OC Path")
		srgbGlobalConfig := configureSRGBViaMplsGlobalPath(srgbLowerBound, srgbUpperBound)
		gnmi.Update(t, dut, gnmi.OC().Config(), srgbGlobalConfig)
	case false:
		// Other vendors
		t.Log("SRGB configuration under only network-instance/MPLS")
	}
	ts.ATETop.Flows().Clear()
	configureOTG(t, ts)
	ts.PushAndStart(t)
	ts.MustAdjacency(t)

	verifyMPLSSR(t, ts)

	// Traffic checks
	otg := ts.ATE.OTG()
	t.Run("Traffic checks", func(t *testing.T) {
		otgutils.WaitForARP(t, otg, ts.ATETop, "IPv4")
		otgutils.WaitForARP(t, otg, ts.ATETop, "IPv6")

		waitForFIBProgramming(t, ts.DUT, true, fmt.Sprintf("%s/%d", ateV4Route, plenIPv4))
		waitForFIBProgramming(t, ts.DUT, false, fmt.Sprintf("%s/%d", ateV6Route, plenIPv6))
		
		t.Logf("Starting traffic")
		t.Log(otg.GetConfig(t))
		otg.StartTraffic(t)
		time.Sleep(time.Second * 15)
		t.Logf("Stop traffic")
		otg.StopTraffic(t)

		verifyTraffic(t, ts.ATE, ts.ATETop)
	})

	// SR counters checks
	t.Run("SR counters checks", func(t *testing.T) {
		t.Logf("Starting traffic")
		otg.StartTraffic(t)
		time.Sleep(time.Second * 15)
		t.Logf("Stop traffic")
		otg.StopTraffic(t)

		t.Logf("Starting SR counters checks")
		if !deviations.SIDPerInterfaceCounterUnsupported(ts.DUT) {
			verifySRCounters(t, ts, ts.ATE)
		}

	})
}

// configureSRGBGlobalPath
func configureSRGBViaMplsGlobalPath(LowerBoundLabel int, UpperBoundLabel int) *oc.Root {

	d := &oc.Root{}

	netInstance := d.GetOrCreateNetworkInstance("DEFAULT")
	netInstance.Name = ygot.String("DEFAULT")
	mplsGlobal := netInstance.GetOrCreateMpls().GetOrCreateGlobal()

	rlb := mplsGlobal.GetOrCreateReservedLabelBlock(SRReservedLabelblockName)
	rlb.LocalId = ygot.String(SRReservedLabelblockName)
	rlb.LowerBound = oc.UnionUint32(LowerBoundLabel)
	rlb.UpperBound = oc.UnionUint32(UpperBoundLabel)

	sr := netInstance.GetOrCreateSegmentRouting()
	srgb := sr.GetOrCreateSrgb(SRReservedLabelblockName)
	srgb.LocalId = ygot.String(SRReservedLabelblockName)
	srgb.SetMplsLabelBlocks([]string{SRReservedLabelblockName})

	return d
}
