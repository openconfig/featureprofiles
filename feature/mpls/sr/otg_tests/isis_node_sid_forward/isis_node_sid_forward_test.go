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
	"net"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/isissession"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

// TestMain initializes the testbed and runs the tests
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Constants
const (
	srgbMplsLabelBlockName = "400000 465001"
	srgbLowerBound         = 400000
	srgbUpperBound         = 465001
	srgbLocalID            = "100.1.1.1"
	srlbLocalID            = "200.1.1.1"
	plenIPv4               = 30
	plenIPv6               = 126
	ateV4Route             = "203.0.113.0/30"
	ateV6Route             = "2001:db8::203:0:113:0/126"
	v4IP                   = "203.0.113.1"
	v6IP                   = "2001:db8::203:0:113:1"
	v4Route                = "203.0.113.0"
	v6Route                = "2001:db8::203:0:113:0"
	ateV4Metric            = 200
	ateV6Metric            = 200
	v4NetName              = "isisv4Net"
	v6NetName              = "isisv6Net"
	v4FlowName             = "v4Flow"
	v6FlowName             = "v6Flow"
)

// Configure ISIS, MPLS and ISIS-SR on DUT
func configureISISSegmentRouting(t *testing.T, ts *isissession.TestSession) {
	t.Helper()
	d := ts.DUTConf
	networkInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(ts.DUT))
	prot := networkInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isissession.ISISName)
	// prot.Enabled = ygot.Bool(true)

	// Configure MPLS
	mplsCfg := networkInstance.GetOrCreateMpls().GetOrCreateGlobal()
	mplsCfg.GetOrCreateInterface(ts.DUTPort1.Name())
	mplsCfg.GetOrCreateInterface(ts.DUTPort2.Name())
	mplsCfg.GetOrCreateReservedLabelBlock(srgbMplsLabelBlockName).LowerBound = oc.UnionUint32(srgbLowerBound)
	mplsCfg.GetOrCreateReservedLabelBlock(srgbMplsLabelBlockName).UpperBound = oc.UnionUint32(srgbUpperBound)

	// Configure SR
	srCfg := networkInstance.GetOrCreateSegmentRouting()
	srgb := srCfg.GetOrCreateSrgb("99.99.99.99")
	srgb.LocalId = ygot.String("99.99.99.99")
	srgb.SetMplsLabelBlocks([]string{srgbMplsLabelBlockName})

	// Configure ISIS-SR
	isisSR := prot.GetOrCreateIsis().GetOrCreateGlobal().GetOrCreateSegmentRouting()
	isisSR.SetSrgb("99.99.99.99")
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

	v4Flow.Duration().FixedPackets().SetPackets(1000)
	v4Flow.Size().SetFixed(512)
	v4Flow.Rate().SetPps(100)

	e1 := v4Flow.Packet().Add().Ethernet()
	e1.Src().SetValue(isissession.ATEISISAttrs.MAC)

	v4 := v4Flow.Packet().Add().Ipv4()
	v4.Src().SetValue(isissession.ATEISISAttrs.IPv4)
	v4.Dst().SetValues([]string{srcIpv4.Address(), srcIpv4.Address()})

	t.Log("Configuring v6 traffic flow ")

	v6Flow := ts.ATETop.Flows().Add().SetName(v6FlowName)
	v6Flow.Metrics().SetEnable(true)

	v6Flow.TxRx().Device().
		SetTxNames([]string{srcIpv6.Name()}).
		SetRxNames([]string{v6NetName})

	v4Flow.Duration().FixedPackets().SetPackets(1000)
	v6Flow.Size().SetFixed(512)
	v6Flow.Rate().SetPps(100)

	e2 := v6Flow.Packet().Add().Ethernet()
	e2.Src().SetValue(isissession.ATEISISAttrs.MAC)
	v6 := v6Flow.Packet().Add().Ipv6()
	v6.Src().SetValue(isissession.ATEISISAttrs.IPv6)
	v6.Dst().SetValues([]string{srcIpv6.Address(), srcIpv6.Address()})
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

		srgbValue := routing.GetIsis().GetGlobal().GetSegmentRouting().GetSrgb()
		if srgbValue == "nil" || srgbValue == "" {
			t.Errorf("FAIL- SRGB is not present on DUT")
		} else {
			t.Logf("SRGB is present on DUT value: %s", srgbValue)
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

func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice) {
	recvMetricV4 := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(v4FlowName).State())
	recvMetricV6 := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(v6FlowName).State())

	framesTxV4 := recvMetricV4.GetCounters().GetOutPkts()
	framesRxV4 := recvMetricV4.GetCounters().GetInPkts()
	framesTxV6 := recvMetricV6.GetCounters().GetOutPkts()
	framesRxV6 := recvMetricV6.GetCounters().GetInPkts()

	t.Logf("Starting V4 traffic validation")
	if framesTxV4 == 0 {
		t.Error("No traffic was generated and frames transmitted were 0")
	} else if framesRxV4 == framesTxV4 {
		t.Logf("Traffic validation successful for [%s] FramesTx: %d FramesRx: %d", v4FlowName, framesTxV4, framesRxV4)
	} else {
		t.Errorf("Traffic validation failed for [%s] FramesTx: %d FramesRx: %d", v4FlowName, framesTxV4, framesRxV4)
	}
	t.Logf("Starting V6 traffic validation")
	if framesTxV6 == 0 {
		t.Error("No traffic was generated and frames transmitted were 0")
	} else if framesRxV6 == framesTxV6 {
		t.Logf("Traffic validation successful for [%s] FramesTx: %d FramesRx: %d", v6FlowName, framesTxV6, framesRxV6)
	} else {
		t.Errorf("Traffic validation failed for [%s] FramesTx: %d FramesRx: %d", v6FlowName, framesTxV6, framesRxV6)
	}
}

// TestMPLSLabelBlockWithISIS verifies MPLS label block SRGB on DUT.
func TestMPLSLabelBlockWithISIS(t *testing.T) {
	ts := isissession.MustNew(t).WithISIS()
	configureISISSegmentRouting(t, ts)
	ts.ATETop.Flows().Clear()
	configureOTG(t, ts)
	ts.PushAndStart(t)
	ts.MustAdjacency(t)

	verifyMPLSSR(t, ts)

	// Traffic checks
	otg := ts.ATE.OTG()
	t.Run("Traffic checks", func(t *testing.T) {
		t.Logf("Starting traffic")
		otg.StartTraffic(t)
		time.Sleep(time.Second * 15)
		t.Logf("Stop traffic")
		otg.StopTraffic(t)

		otgutils.LogFlowMetrics(t, otg, ts.ATETop)
		otgutils.LogPortMetrics(t, otg, ts.ATETop)
		verifyTraffic(t, ts.ATE)
	})
}
