// Copyright 2024 Google LLC
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

package zr_fec_uncorrectable_frames_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"flag"

	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gnoi"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	sampleInterval = 10 * time.Second
	intUpdateTime  = 10 * time.Minute
	otnIndexBase   = uint32(4000)
	ethIndexBase   = uint32(40000)

	targetOutputPowerdBm          = -10.0
	targetOutputPowerTolerancedBm = 1.0
	targetFrequencyMHz            = 193100000
	targetFrequencyToleranceMHz   = 100000.0
	samplingInterval              = 10 * time.Second
	sampleTimeout                 = 60 * time.Second
	linkUpTimeout                 = 5 * time.Minute

	otnIndex1 = uint32(4000)
	otnIndex2 = uint32(4001)
	ethIndex1 = uint32(40000)
	ethIndex2 = uint32(40001)

	// Dual-stack subinterface addressing and destination prefixes defined in TRANSCEIVER-10.1 README.
	port1IPv4      = "192.0.2.1"
	port2IPv4      = "192.0.2.2"
	ipv4PrefixLen  = uint8(30)
	destIPv4Prefix = "198.51.100.0/24"
	port1IPv6      = "2001:db8::1"
	port2IPv6      = "2001:db8::2"
	ipv6PrefixLen  = uint8(126)
	destIPv6Prefix = "2001:db8:1::/64"

	// gNOI Packet Link Qualification session parameters defined in TRANSCEIVER-10.1.2.
	nearEndQualID    = "qual-400zr-near"
	farEndQualID     = "qual-400zr-far"
	plqMinBps        = uint64(400_000_000_000)
	plqPacketRate    = uint64(50_000_000)
	plqPacketSize    = uint32(8184)
	plqSetupDuration = 5 * time.Second
	plqPreSync       = 10 * time.Second
	plqDuration      = 30 * time.Second
	plqPostSync      = 5 * time.Second
	plqTeardown      = 5 * time.Second
)

var (
	operationalModeFlag = flag.Int("operational_mode", 0, "vendor-specific operational-mode for the channel.")
	operationalMode     uint16
)

// portFECStreams bundles the gNMI SAMPLE telemetry streams for a 400ZR port.
type portFECStreams struct {
	otnUncorrectable  *samplestream.SampleStream[uint64]
	ethUncorrectable  *samplestream.SampleStream[uint64]
	ochUncorrectable  *samplestream.SampleStream[uint64]
	intfUncorrectable *samplestream.SampleStream[uint64]
	intfCorrected     *samplestream.SampleStream[uint64]
}

func (s *portFECStreams) closeAll() {
	if s.otnUncorrectable != nil {
		s.otnUncorrectable.Close()
	}
	if s.ethUncorrectable != nil {
		s.ethUncorrectable.Close()
	}
	if s.ochUncorrectable != nil {
		s.ochUncorrectable.Close()
	}
	if s.intfUncorrectable != nil {
		s.intfUncorrectable.Close()
	}
	if s.intfCorrected != nil {
		s.intfCorrected.Close()
	}
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func validateFecUncorrectableBlocks(t *testing.T, stream *samplestream.SampleStream[uint64], baselineValue uint64) {
	dut := ondatra.DUT(t, "dut")
	fecStream, ok := stream.AwaitNext(sampleTimeout, func(v *ygnmi.Value[uint64]) bool {
		_, present := v.Val()
		return present
	})
	if !ok || fecStream == nil {
		t.Fatalf("Fec Uncorrectable Blocks was not streamed in the most recent subscription interval")
	}
	currentFec, ok := fecStream.Val()
	if !ok {
		t.Fatalf("Error capturing streaming Fec value")
	}
	if reflect.TypeOf(currentFec).Kind() != reflect.Uint64 {
		t.Fatalf("fec value is not type uint64")
	}
	if deviations.NonIntervalFecErrorCounter(dut) {
		// Check if the counter is incrementing
		if currentFec > baselineValue {
			t.Fatalf("FecUncorrectableBlocks increased after flap: baseline=%d, current=%d, diff=%d",
				baselineValue, currentFec, currentFec-baselineValue)
		}
		t.Logf("FecUncorrectableBlocks not increasing: baseline=%d, current=%d", baselineValue, currentFec)
	} else if currentFec != 0 {
		t.Fatalf("Got FecUncorrectableBlocks got %d, want 0", currentFec)
	}
}

// TestZrUncorrectableFrames validates the baseline TRANSCEIVER-10.1 Procedure (existing testcase).
func TestZrUncorrectableFrames(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var (
		trs        = make(map[string]string)
		ochs       = make(map[string]string)
		otnIndexes = make(map[string]uint32)
		ethIndexes = make(map[string]uint32)
	)

	ports := []string{"port1", "port2"}
	operationalMode = uint16(*operationalModeFlag)
	operationalMode = cfgplugins.InterfaceInitialize(t, dut, operationalMode)

	for i, port := range ports {
		dp := dut.Port(t, port)
		cfgplugins.InterfaceConfig(t, dut, dp)
		trs[dp.Name()] = gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).Transceiver().State())
		ochs[dp.Name()] = gnmi.Get(t, dut, gnmi.OC().Component(trs[dp.Name()]).Transceiver().Channel(0).AssociatedOpticalChannel().State())
		otnIndexes[dp.Name()] = otnIndexBase + uint32(i)
		ethIndexes[dp.Name()] = ethIndexBase + uint32(i)
		cfgplugins.ConfigOTNChannel(t, dut, ochs[dp.Name()], otnIndexes[dp.Name()], ethIndexes[dp.Name()])
		cfgplugins.ConfigETHChannel(t, dut, dp.Name(), trs[dp.Name()], otnIndexes[dp.Name()], ethIndexes[dp.Name()])
	}

	for _, port := range ports {
		t.Run(fmt.Sprintf("Port:%s", port), func(t *testing.T) {
			dp := dut.Port(t, port)
			gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)
			streamFecOtn := samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndexes[dp.Name()]).Otn().FecUncorrectableBlocks().State(), sampleInterval)
			t.Cleanup(streamFecOtn.Close)

			baselineFecStream, ok := streamFecOtn.AwaitNext(sampleTimeout, func(v *ygnmi.Value[uint64]) bool {
				_, present := v.Val()
				return present
			})
			if !ok || baselineFecStream == nil {
				t.Fatalf("Baseline Fec Uncorrectable Blocks was not streamed")
			}
			baselineFec, ok := baselineFecStream.Val()
			if !ok {
				t.Fatalf("Error capturing baseline Fec value")
			}

			// Validate that FEC errors haven't increased in the next sample interval.
			validateFecUncorrectableBlocks(t, streamFecOtn, baselineFec)

			// Disable interface.
			cfgplugins.ToggleInterface(t, dut, dp.Name(), false)
			gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_DOWN)

			// Enable interface.
			cfgplugins.ToggleInterface(t, dut, dp.Name(), true)
			gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)

			baselineFecStream1, ok := streamFecOtn.AwaitNext(sampleTimeout, func(v *ygnmi.Value[uint64]) bool {
				_, present := v.Val()
				return present
			})
			if !ok || baselineFecStream1 == nil {
				t.Fatalf("Baseline Fec Uncorrectable Blocks was not streamed")
			}
			baselineFec1, ok := baselineFecStream1.Val()
			if !ok {
				t.Fatalf("Error capturing baseline Fec value")
			}

			// Validate that FEC errors haven't increased in the next sample interval after flap.
			validateFecUncorrectableBlocks(t, streamFecOtn, baselineFec1)
		})
	}
}

// validateFECSampleStream verifies that count consecutive 10-second PM interval samples from the
// given uint64 stream report valid uint64 values equal to want (0 for per-interval PM counters, or
// unchanged baseline when deviations.NonIntervalFecErrorCounter(dut) is set).
func validateFECSampleStream(t *testing.T, dut *ondatra.DUTDevice, stream *samplestream.SampleStream[uint64], pathDesc string, count int, baseline uint64) uint64 {
	t.Helper()
	wantVal := uint64(0)
	if deviations.NonIntervalFecErrorCounter(dut) {
		wantVal = baseline
	}

	samples, err := samplestream.AwaitNextSamples(t, stream, count, sampleTimeout, func(v *ygnmi.Value[uint64]) bool {
		_, present := v.Val()
		return present
	})
	if err != nil {
		t.Fatalf("%s: failed to receive %d consecutive PM interval samples: %v", pathDesc, count, err)
	}

	var lastGot uint64
	for idx, sample := range samples {
		val, ok := sample.Val()
		if !ok {
			t.Fatalf("%s [sample %d/%d]: telemetry value is empty", pathDesc, idx+1, count)
		}
		if reflect.TypeOf(val).Kind() != reflect.Uint64 {
			t.Fatalf("%s [sample %d/%d]: value %v is not a valid uint64", pathDesc, idx+1, count, val)
		}
		lastGot = val
		t.Logf("%s [PM interval %d/%d]: got=%d, want=%d (NonIntervalFecErrorCounter=%v)", pathDesc, idx+1, count, val, wantVal, deviations.NonIntervalFecErrorCounter(dut))
		if val != wantVal {
			t.Errorf("%s [PM interval %d/%d]: got %d, want %d", pathDesc, idx+1, count, val, wantVal)
		}
	}
	return lastGot
}

// verifySecondaryFECStreams checks the latest sample on the auxiliary FEC telemetry paths when
// supported by the platform, and asserts 0 for uncorrectable paths when interval semantics apply.
func verifySecondaryFECStreams(t *testing.T, dut *ondatra.DUTDevice, portName string, streams *portFECStreams) {
	t.Helper()
	aux := []struct {
		desc          string
		stream        *samplestream.SampleStream[uint64]
		uncorrectable bool
	}{
		{desc: portName + " /terminal-device/logical-channels/channel/ethernet/state/fec-uncorrectable-blocks", stream: streams.ethUncorrectable, uncorrectable: true},
		{desc: portName + " /components/component/optical-channel/state/fec-uncorrectable-blocks", stream: streams.ochUncorrectable, uncorrectable: true},
		{desc: portName + " /interfaces/interface/ethernet/state/counters/fec-uncorrectable-blocks", stream: streams.intfUncorrectable, uncorrectable: true},
		{desc: portName + " /interfaces/interface/ethernet/state/counters/fec-corrected-blocks", stream: streams.intfCorrected, uncorrectable: false},
	}
	for _, item := range aux {
		if all := item.stream.All(); len(all) > 0 {
			if val, ok := all[len(all)-1].Val(); ok {
				t.Logf("%s: latest streamed value = %d", item.desc, val)
				if item.uncorrectable && !deviations.NonIntervalFecErrorCounter(dut) && val != 0 {
					t.Errorf("%s: got %d, want 0", item.desc, val)
				}
			}
		}
	}
}

// configurePortL3AndOptical applies the complete 400ZR optical, OTN/ETH logical channel,
// transceiver, and L3 dual-stack subinterface configuration for a port.
func configurePortL3AndOptical(t *testing.T, dut *ondatra.DUTDevice, dp *ondatra.Port, trName, ochName string, otnIdx, ethIdx uint32, ipv4Addr, ipv6Addr string) {
	t.Helper()
	cfgplugins.InterfaceConfig(t, dut, dp)
	cfgplugins.ToggleTransceiver(t, dut, trName, true)
	cfgplugins.ConfigOpticalChannel(t, dut, ochName, targetFrequencyMHz, targetOutputPowerdBm, operationalMode)
	cfgplugins.ConfigOTNChannel(t, dut, ochName, otnIdx, ethIdx)
	cfgplugins.ConfigETHChannel(t, dut, dp.Name(), trName, otnIdx, ethIdx)
	cfgplugins.Configure400ZRL3Interface(t, dut, nil, dp.Name(), ipv4Addr, ipv4PrefixLen, ipv6Addr, ipv6PrefixLen)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, dp.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

// TestZrFecPMIntervalSemantics independently validates TRANSCEIVER-10.1.1: Validate 400ZR FEC PM
// Interval Semantics without any dependency on prior or subsequent testcases.
func TestZrFecPMIntervalSemantics(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	ports := []*ondatra.Port{dp1, dp2}

	otnIndexes := map[string]uint32{
		dp1.Name(): otnIndex1,
		dp2.Name(): otnIndex2,
	}
	ethIndexes := map[string]uint32{
		dp1.Name(): ethIndex1,
		dp2.Name(): ethIndex2,
	}
	ipv4Addrs := map[string]string{
		dp1.Name(): port1IPv4,
		dp2.Name(): port2IPv4,
	}
	ipv6Addrs := map[string]string{
		dp1.Name(): port1IPv6,
		dp2.Name(): port2IPv6,
	}

	trs := make(map[string]string, len(ports))
	ochs := make(map[string]string, len(ports))
	for _, dp := range ports {
		trs[dp.Name()] = gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).Transceiver().State())
		ochs[dp.Name()] = components.OpticalChannelComponentFromPort(t, dut, dp)
	}

	// Register cleanup before pushing any configuration changes to the DUT.
	t.Cleanup(func() {
		for _, dp := range ports {
			configurePortL3AndOptical(t, dut, dp, trs[dp.Name()], ochs[dp.Name()], otnIndexes[dp.Name()], ethIndexes[dp.Name()], ipv4Addrs[dp.Name()], ipv6Addrs[dp.Name()])
		}
		for _, dp := range ports {
			gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), linkUpTimeout, oc.Interface_OperStatus_UP)
		}
	})

	// =========================================================================================
	// Topology and Test Environment Setup (TRANSCEIVER-10.1.1)
	// =========================================================================================
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	operationalMode = uint16(*operationalModeFlag)
	operationalMode = cfgplugins.InterfaceInitialize(t, dut, operationalMode)

	// =========================================================================================
	// TRANSCEIVER-10.1.1 - Step 1: Configuration
	// =========================================================================================
	// Independently configure the 400ZR optical interfaces, transceivers, optical channels,
	// terminal-device logical channels, and dual-stack Layer 3 subinterfaces on port1 and port2.
	for _, dp := range ports {
		configurePortL3AndOptical(t, dut, dp, trs[dp.Name()], ochs[dp.Name()], otnIndexes[dp.Name()], ethIndexes[dp.Name()], ipv4Addrs[dp.Name()], ipv6Addrs[dp.Name()])
	}

	fecStreams := make(map[string]*portFECStreams, len(ports))
	for _, dp := range ports {
		s := &portFECStreams{
			otnUncorrectable:  samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndexes[dp.Name()]).Otn().FecUncorrectableBlocks().State(), samplingInterval),
			ethUncorrectable:  samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(ethIndexes[dp.Name()]).Ethernet().FecUncorrectableBlocks().State(), samplingInterval),
			ochUncorrectable:  samplestream.New(t, dut, gnmi.OC().Component(ochs[dp.Name()]).OpticalChannel().FecUncorrectableBlocks().State(), samplingInterval),
			intfUncorrectable: samplestream.New(t, dut, gnmi.OC().Interface(dp.Name()).Ethernet().Counters().FecUncorrectableBlocks().State(), samplingInterval),
			intfCorrected:     samplestream.New(t, dut, gnmi.OC().Interface(dp.Name()).Ethernet().Counters().FecCorrectedBlocks().State(), samplingInterval),
		}
		fecStreams[dp.Name()] = s
		t.Cleanup(s.closeAll)
	}

	baselineOTNUncorrectable := make(map[string]uint64, len(ports))

	// ---------------------------------------------------------------------------------
	// TRANSCEIVER-10.1.1 - Step 2: Baseline Operational State and PM Interval Telemetry
	// ---------------------------------------------------------------------------------
	for _, dp := range ports {
		cfgplugins.Await400ZROpticalLinkUp(t, dut, dp.Name(), ochs[dp.Name()], linkUpTimeout)
		cfgplugins.ValidateInterfaceConfig(t, dut, dp, targetOutputPowerdBm, targetFrequencyMHz, targetOutputPowerTolerancedBm, targetFrequencyToleranceMHz)
		if !deviations.OperationalModeUnsupported(dut) {
			gotOpMode := gnmi.Get(t, dut, gnmi.OC().Component(ochs[dp.Name()]).OpticalChannel().OperationalMode().State())
			if gotOpMode != operationalMode {
				t.Errorf("Port %s optical-channel operational-mode: got %d, want %d", dp.Name(), gotOpMode, operationalMode)
			}
		}
	}

	for _, dp := range ports {
		if deviations.NonIntervalFecErrorCounter(dut) {
			initSample, ok := fecStreams[dp.Name()].otnUncorrectable.AwaitNext(sampleTimeout, func(v *ygnmi.Value[uint64]) bool {
				_, present := v.Val()
				return present
			})
			if !ok || initSample == nil {
				t.Fatalf("Port %s: failed to read initial OTN fec-uncorrectable-blocks sample", dp.Name())
			}
			baselineOTNUncorrectable[dp.Name()], _ = initSample.Val()
		}

		pathDesc := dp.Name() + " /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks (Step 2 Baseline)"
		baselineOTNUncorrectable[dp.Name()] = validateFECSampleStream(t, dut, fecStreams[dp.Name()].otnUncorrectable, pathDesc, 3, baselineOTNUncorrectable[dp.Name()])
		verifySecondaryFECStreams(t, dut, dp.Name(), fecStreams[dp.Name()])
	}

	// ---------------------------------------------------------------------------------
	// TRANSCEIVER-10.1.1 - Step 3a: Transient Link Disturbance via Interface Flap
	// ---------------------------------------------------------------------------------
	cfgplugins.ToggleInterface(t, dut, dp1.Name(), false)
	gnmi.Await(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), linkUpTimeout, oc.Interface_OperStatus_DOWN)
	// Wait 1 PM interval (10s) while port1 is DOWN by awaiting the next 10s PM sample on the active subscription.
	fecStreams[dp1.Name()].otnUncorrectable.AwaitNext(sampleTimeout, func(v *ygnmi.Value[uint64]) bool {
		_, present := v.Val()
		return present
	})
	configurePortL3AndOptical(t, dut, dp1, trs[dp1.Name()], ochs[dp1.Name()], otnIndexes[dp1.Name()], ethIndexes[dp1.Name()], ipv4Addrs[dp1.Name()], ipv6Addrs[dp1.Name()])
	for _, dp := range ports {
		cfgplugins.Await400ZROpticalLinkUp(t, dut, dp.Name(), ochs[dp.Name()], linkUpTimeout)
	}
	for _, dp := range ports {
		if deviations.NonIntervalFecErrorCounter(dut) {
			postFlapSample, ok := fecStreams[dp.Name()].otnUncorrectable.AwaitNext(sampleTimeout, func(v *ygnmi.Value[uint64]) bool {
				_, present := v.Val()
				return present
			})
			if ok && postFlapSample != nil {
				baselineOTNUncorrectable[dp.Name()], _ = postFlapSample.Val()
			}
		}
		pathDesc := dp.Name() + " /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks (Step 3a Post-Interface-Flap)"
		baselineOTNUncorrectable[dp.Name()] = validateFECSampleStream(t, dut, fecStreams[dp.Name()].otnUncorrectable, pathDesc, 2, baselineOTNUncorrectable[dp.Name()])
		verifySecondaryFECStreams(t, dut, dp.Name(), fecStreams[dp.Name()])
	}

	// ---------------------------------------------------------------------------------
	// TRANSCEIVER-10.1.1 - Step 3b: Transient Link Disturbance via Transceiver Toggle
	// ---------------------------------------------------------------------------------
	if !deviations.TransceiverConfigEnableUnsupported(dut) {
		cfgplugins.ToggleTransceiver(t, dut, trs[dp1.Name()], false)
		gnmi.Await(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), linkUpTimeout, oc.Interface_OperStatus_DOWN)
		cfgplugins.ToggleTransceiver(t, dut, trs[dp1.Name()], true)
		for _, dp := range ports {
			cfgplugins.Await400ZROpticalLinkUp(t, dut, dp.Name(), ochs[dp.Name()], linkUpTimeout)
		}
		for _, dp := range ports {
			if deviations.NonIntervalFecErrorCounter(dut) {
				postToggleSample, ok := fecStreams[dp.Name()].otnUncorrectable.AwaitNext(sampleTimeout, func(v *ygnmi.Value[uint64]) bool {
					_, present := v.Val()
					return present
				})
				if ok && postToggleSample != nil {
					baselineOTNUncorrectable[dp.Name()], _ = postToggleSample.Val()
				}
			}
			pathDesc := dp.Name() + " /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks (Step 3b Post-Transceiver-Toggle)"
			baselineOTNUncorrectable[dp.Name()] = validateFECSampleStream(t, dut, fecStreams[dp.Name()].otnUncorrectable, pathDesc, 2, baselineOTNUncorrectable[dp.Name()])
			verifySecondaryFECStreams(t, dut, dp.Name(), fecStreams[dp.Name()])
		}
	}
}

// TestZrLinkQualificationAndDualStackTraffic independently validates TRANSCEIVER-10.1.2: Validate
// 400ZR Link Qualification and Dual-Stack Traffic without any dependency on prior testcases.
func TestZrLinkQualificationAndDualStackTraffic(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	ports := []*ondatra.Port{dp1, dp2}

	otnIndexes := map[string]uint32{
		dp1.Name(): otnIndex1,
		dp2.Name(): otnIndex2,
	}
	ethIndexes := map[string]uint32{
		dp1.Name(): ethIndex1,
		dp2.Name(): ethIndex2,
	}
	ipv4Addrs := map[string]string{
		dp1.Name(): port1IPv4,
		dp2.Name(): port2IPv4,
	}
	ipv6Addrs := map[string]string{
		dp1.Name(): port1IPv6,
		dp2.Name(): port2IPv6,
	}

	trs := make(map[string]string, len(ports))
	ochs := make(map[string]string, len(ports))
	for _, dp := range ports {
		trs[dp.Name()] = gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).Transceiver().State())
		ochs[dp.Name()] = components.OpticalChannelComponentFromPort(t, dut, dp)
	}

	// Register cleanup before pushing any configuration changes or starting PLQ sessions on the DUT.
	t.Cleanup(func() {
		gnoi.CleanupPLQSessions(t, dut, []string{nearEndQualID, farEndQualID}, ports)
		cleanupBatch := &gnmi.SetBatch{}
		_, _ = cfgplugins.NewStaticRouteCfg(cleanupBatch, &cfgplugins.StaticRouteCfg{
			NetworkInstance:   deviations.DefaultNetworkInstance(dut),
			Prefix:            destIPv4Prefix,
			RemoveStaticRoute: true,
		}, dut)
		_, _ = cfgplugins.NewStaticRouteCfg(cleanupBatch, &cfgplugins.StaticRouteCfg{
			NetworkInstance:   deviations.DefaultNetworkInstance(dut),
			Prefix:            destIPv6Prefix,
			RemoveStaticRoute: true,
		}, dut)
		cleanupBatch.Set(t, dut)

		for _, dp := range ports {
			configurePortL3AndOptical(t, dut, dp, trs[dp.Name()], ochs[dp.Name()], otnIndexes[dp.Name()], ethIndexes[dp.Name()], ipv4Addrs[dp.Name()], ipv6Addrs[dp.Name()])
		}
		for _, dp := range ports {
			gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), linkUpTimeout, oc.Interface_OperStatus_UP)
		}
	})

	// =========================================================================================
	// Topology and Test Environment Setup (TRANSCEIVER-10.1.2)
	// =========================================================================================
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	operationalMode = uint16(*operationalModeFlag)
	operationalMode = cfgplugins.InterfaceInitialize(t, dut, operationalMode)

	// =========================================================================================
	// TRANSCEIVER-10.1.2 - Step 1: Independent Link Configuration & Pre-Qualification Capabilities Check
	// =========================================================================================
	for _, dp := range ports {
		configurePortL3AndOptical(t, dut, dp, trs[dp.Name()], ochs[dp.Name()], otnIndexes[dp.Name()], ethIndexes[dp.Name()], ipv4Addrs[dp.Name()], ipv6Addrs[dp.Name()])
	}

	routeBatch := &gnmi.SetBatch{}
	cfgplugins.ConfigureStaticRoute(t, dut, routeBatch, cfgplugins.ConfigureStaticRouteParams{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          destIPv4Prefix,
		Index:           "0",
		NextHop:         port2IPv4,
	})
	cfgplugins.ConfigureStaticRoute(t, dut, routeBatch, cfgplugins.ConfigureStaticRouteParams{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          destIPv6Prefix,
		Index:           "0",
		NextHop:         port2IPv6,
	})
	routeBatch.Set(t, dut)

	fecStreams := make(map[string]*portFECStreams, len(ports))
	for _, dp := range ports {
		s := &portFECStreams{
			otnUncorrectable:  samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndexes[dp.Name()]).Otn().FecUncorrectableBlocks().State(), samplingInterval),
			ethUncorrectable:  samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(ethIndexes[dp.Name()]).Ethernet().FecUncorrectableBlocks().State(), samplingInterval),
			ochUncorrectable:  samplestream.New(t, dut, gnmi.OC().Component(ochs[dp.Name()]).OpticalChannel().FecUncorrectableBlocks().State(), samplingInterval),
			intfUncorrectable: samplestream.New(t, dut, gnmi.OC().Interface(dp.Name()).Ethernet().Counters().FecUncorrectableBlocks().State(), samplingInterval),
			intfCorrected:     samplestream.New(t, dut, gnmi.OC().Interface(dp.Name()).Ethernet().Counters().FecCorrectedBlocks().State(), samplingInterval),
		}
		fecStreams[dp.Name()] = s
		t.Cleanup(s.closeAll)
	}

	baselineOTNUncorrectable := make(map[string]uint64, len(ports))
	for _, dp := range ports {
		cfgplugins.Await400ZROpticalLinkUp(t, dut, dp.Name(), ochs[dp.Name()], linkUpTimeout)
		if deviations.NonIntervalFecErrorCounter(dut) {
			initSample, ok := fecStreams[dp.Name()].otnUncorrectable.AwaitNext(sampleTimeout, func(v *ygnmi.Value[uint64]) bool {
				_, present := v.Val()
				return present
			})
			if !ok || initSample == nil {
				t.Fatalf("Port %s: failed to read initial OTN fec-uncorrectable-blocks sample", dp.Name())
			}
			baselineOTNUncorrectable[dp.Name()], _ = initSample.Val()
		}
	}

	gnoi.VerifyPLQCapabilities(t, dut, ports, plqMinBps, plqPacketSize, plqPacketRate)

	// ---------------------------------------------------------------------------------
	// TRANSCEIVER-10.1.2 - Step 2: Packet Link Qualification Execution
	// ---------------------------------------------------------------------------------
	plqParams := gnoi.PLQTestParams{
		GeneratorPort:    dp1,
		ReflectorPort:    dp2,
		GeneratorQualID:  nearEndQualID,
		ReflectorQualID:  farEndQualID,
		PacketRate:       plqPacketRate,
		PacketSize:       plqPacketSize,
		SetupDuration:    plqSetupDuration,
		PreSyncDuration:  plqPreSync,
		TestDuration:     plqDuration,
		PostSyncDuration: plqPostSync,
		TeardownDuration: plqTeardown,
	}

	gnoi.StartPLQ(t, dut, plqParams)

	// Verify FEC uncorrectable blocks remain 0 during the active qualification window via gNMI.Get and gNMI.Subscribe.
	for _, dp := range ports {
		gotOTN := gnmi.Get(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndexes[dp.Name()]).Otn().FecUncorrectableBlocks().State())
		wantOTN := uint64(0)
		if deviations.NonIntervalFecErrorCounter(dut) {
			wantOTN = baselineOTNUncorrectable[dp.Name()]
		}
		if gotOTN != wantOTN {
			t.Errorf("Port %s during-PLQ gNMI.Get /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks: got %d, want %d", dp.Name(), gotOTN, wantOTN)
		}
		pathDesc := dp.Name() + " /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks (Step 2 During PLQ)"
		baselineOTNUncorrectable[dp.Name()] = validateFECSampleStream(t, dut, fecStreams[dp.Name()].otnUncorrectable, pathDesc, 1, baselineOTNUncorrectable[dp.Name()])
	}

	gnoi.AwaitAndVerifyPLQCompleted(t, dut, plqParams, 3*time.Minute)
	gnoi.CleanupPLQSessions(t, dut, []string{nearEndQualID, farEndQualID}, ports)

	// Verify FEC uncorrectable blocks remain 0 after qualification completion via gNMI.Get and gNMI.Subscribe.
	for _, dp := range ports {
		gotOTN := gnmi.Get(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndexes[dp.Name()]).Otn().FecUncorrectableBlocks().State())
		wantOTN := uint64(0)
		if deviations.NonIntervalFecErrorCounter(dut) {
			wantOTN = baselineOTNUncorrectable[dp.Name()]
		}
		if gotOTN != wantOTN {
			t.Errorf("Port %s post-PLQ gNMI.Get /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks: got %d, want %d", dp.Name(), gotOTN, wantOTN)
		}
		pathDesc := dp.Name() + " /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks (Step 2 Post-PLQ)"
		baselineOTNUncorrectable[dp.Name()] = validateFECSampleStream(t, dut, fecStreams[dp.Name()].otnUncorrectable, pathDesc, 1, baselineOTNUncorrectable[dp.Name()])
	}

	// ---------------------------------------------------------------------------------
	// TRANSCEIVER-10.1.2 - Step 3: Post-Qualification Dual-Stack Traffic and Counter Verification
	// ---------------------------------------------------------------------------------
	inPkts := make(map[string]uint64, len(ports))
	outPkts := make(map[string]uint64, len(ports))
	for _, dp := range ports {
		gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), linkUpTimeout, oc.Interface_OperStatus_UP)

		v4AddrPath := gnmi.OC().Interface(dp.Name()).Subinterface(0).Ipv4().Address(ipv4Addrs[dp.Name()])
		gnmi.Await(t, dut, v4AddrPath.Ip().State(), 30*time.Second, ipv4Addrs[dp.Name()])

		v6AddrPath := gnmi.OC().Interface(dp.Name()).Subinterface(0).Ipv6().Address(ipv6Addrs[dp.Name()])
		gnmi.Await(t, dut, v6AddrPath.Ip().State(), 30*time.Second, ipv6Addrs[dp.Name()])

		countersPath := gnmi.OC().Interface(dp.Name()).Counters()
		if val, ok := gnmi.Watch(t, dut, countersPath.OutPkts().State(), 30*time.Second, func(v *ygnmi.Value[uint64]) bool {
			pkts, present := v.Val()
			return present && pkts > 0
		}).Await(t); !ok {
			t.Errorf("Port %s /interfaces/interface/state/counters/out-pkts did not exceed 0 (last value: %v)", dp.Name(), val)
		}
		if val, ok := gnmi.Watch(t, dut, countersPath.InPkts().State(), 30*time.Second, func(v *ygnmi.Value[uint64]) bool {
			pkts, present := v.Val()
			return present && pkts > 0
		}).Await(t); !ok {
			t.Errorf("Port %s /interfaces/interface/state/counters/in-pkts did not exceed 0 (last value: %v)", dp.Name(), val)
		}

		outPkts[dp.Name()] = gnmi.Get(t, dut, countersPath.OutPkts().State())
		inPkts[dp.Name()] = gnmi.Get(t, dut, countersPath.InPkts().State())
		t.Logf("Port %s counters via gNMI.Get: out-pkts=%d, in-pkts=%d", dp.Name(), outPkts[dp.Name()], inPkts[dp.Name()])

		if !deviations.InterfaceCountersInFcsErrorsUnsupported(dut) {
			if got := gnmi.Get(t, dut, countersPath.InFcsErrors().State()); got != 0 {
				t.Errorf("Port %s /interfaces/interface/state/counters/in-fcs-errors: got %d, want 0", dp.Name(), got)
			}
		}

		gotOTN := gnmi.Get(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndexes[dp.Name()]).Otn().FecUncorrectableBlocks().State())
		wantOTN := uint64(0)
		if deviations.NonIntervalFecErrorCounter(dut) {
			wantOTN = baselineOTNUncorrectable[dp.Name()]
		}
		if gotOTN != wantOTN {
			t.Errorf("Port %s post-traffic gNMI.Get /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks: got %d, want %d", dp.Name(), gotOTN, wantOTN)
		}

		pathDesc := dp.Name() + " /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks (Step 3 Post-Traffic)"
		baselineOTNUncorrectable[dp.Name()] = validateFECSampleStream(t, dut, fecStreams[dp.Name()].otnUncorrectable, pathDesc, 1, baselineOTNUncorrectable[dp.Name()])
	}
	if outPkts[dp1.Name()] != inPkts[dp2.Name()] {
		t.Errorf("Point-to-point packet counter mismatch (%s out-pkts=%d != %s in-pkts=%d)", dp1.Name(), outPkts[dp1.Name()], dp2.Name(), inPkts[dp2.Name()])
	}
	if outPkts[dp2.Name()] != inPkts[dp1.Name()] {
		t.Errorf("Point-to-point packet counter mismatch (%s out-pkts=%d != %s in-pkts=%d)", dp2.Name(), outPkts[dp2.Name()], dp1.Name(), inPkts[dp1.Name()])
	}
}
