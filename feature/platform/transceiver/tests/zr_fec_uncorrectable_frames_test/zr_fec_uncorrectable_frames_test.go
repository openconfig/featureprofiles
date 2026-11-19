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
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"flag"

	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	plqpb "github.com/openconfig/gnoi/packet_link_qualification"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	// sampleInterval is the host Performance Monitoring (PM) interval used for gNMI SAMPLE
	// subscriptions to the FEC uncorrectable blocks leaves.
	sampleInterval = 10 * time.Second
	sampleTimeout  = 60 * time.Second
	intUpdateTime  = 10 * time.Minute
	linkUpTimeout  = 5 * time.Minute
	ipStateTimeout = 30 * time.Second

	targetOutputPowerdBm          = -10.0
	targetOutputPowerTolerancedBm = 1.0
	targetFrequencyMHz            = 193100000
	targetFrequencyToleranceMHz   = 100000.0

	// Both ports are on the same DUT in the default network-instance, so each port uses its own
	// /30 and /126 documentation subnet.
	port1IPv4     = "192.0.2.1"
	port2IPv4     = "192.0.2.5"
	ipv4PrefixLen = uint8(30)
	port1IPv6     = "2001:db8::1"
	port2IPv6     = "2001:db8::5"
	ipv6PrefixLen = uint8(126)

	// gNOI Packet Link Qualification parameters defined in TRANSCEIVER-10.1.2.
	nearEndQualID  = "qual-400zr-near"
	farEndQualID   = "qual-400zr-far"
	plqLineRateBps = uint64(400_000_000_000)
	// plqMinGeneratorBps matches the generator max_bps required by gNOI-2.1.
	plqMinGeneratorBps = uint64(100_000_000_000)
	plqPacketSize      = uint32(8184)
	plqFrameOverhead   = uint64(20) // Preamble, start-of-frame delimiter and inter-frame gap.
	// plqLineRatePercent leaves headroom for clock (ppm) offset and control-plane frames.
	plqLineRatePercent   = uint64(99)
	plqMinSetup          = 30 * time.Second
	plqMinTeardown       = 30 * time.Second
	plqGeneratorPreSync  = 10 * time.Second
	plqDuration          = 60 * time.Second
	plqReflectorPostSync = 30 * time.Second
	plqStatePollInterval = 2 * time.Second
	plqDeleteTimeout     = 30 * time.Second
)

var (
	operationalModeFlag = flag.Int("operational_mode", 0, "vendor-specific operational-mode for the channel.")
	operationalMode     uint16

	otnIndexes = map[string]uint32{"port1": 4000, "port2": 4001}
	ethIndexes = map[string]uint32{"port1": 40000, "port2": 40001}
	ipv4Addrs  = map[string]string{"port1": port1IPv4, "port2": port2IPv4}
	ipv6Addrs  = map[string]string{"port1": port1IPv6, "port2": port2IPv6}
)

// zrPort holds the DUT port and the 400ZR components and logical channels associated with it.
type zrPort struct {
	port   *ondatra.Port
	name   string
	tr     string
	och    string
	otnIdx uint32
	ethIdx uint32
	ipv4   string
	ipv6   string
}

// fecStreams holds the gNMI SAMPLE subscriptions to the FEC uncorrectable blocks leaves of a port.
// eth and och are nil only when the corresponding "unsupported" deviation is set.
type fecStreams struct {
	otn *samplestream.SampleStream[uint64]
	eth *samplestream.SampleStream[uint64]
	och *samplestream.SampleStream[uint64]
}

func (s *fecStreams) close() {
	for _, st := range []*samplestream.SampleStream[uint64]{s.otn, s.eth, s.och} {
		if st != nil {
			st.Close()
		}
	}
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func zrPorts(t *testing.T, dut *ondatra.DUTDevice) []*zrPort {
	t.Helper()
	var ports []*zrPort
	for _, id := range []string{"port1", "port2"} {
		dp := dut.Port(t, id)
		ports = append(ports, &zrPort{
			port:   dp,
			name:   dp.Name(),
			tr:     gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).Transceiver().State()),
			och:    components.OpticalChannelComponentFromPort(t, dut, dp),
			otnIdx: otnIndexes[id],
			ethIdx: ethIndexes[id],
			ipv4:   ipv4Addrs[id],
			ipv6:   ipv6Addrs[id],
		})
	}
	return ports
}

// newZRInterface returns the interface configuration shared by every helper in this test.
func newZRInterface(t *testing.T, dut *ondatra.DUTDevice, name string) *oc.Interface {
	t.Helper()
	i := &oc.Interface{
		Name:    ygot.String(name),
		Type:    oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		Enabled: ygot.Bool(true),
	}
	if !deviations.OmitL2MTU(dut) {
		// PLQ uses 8184-byte frames.
		i.Mtu = ygot.Uint16(9000)
	}
	if !deviations.PortSpeedDuplexModeUnsupportedForInterfaceConfig(dut) && (dut.Vendor() == ondatra.ARISTA || deviations.ExplicitPortSpeed(dut)) {
		eth := i.GetOrCreateEthernet()
		if !deviations.PortSpeedUnsupported(dut) {
			eth.PortSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_400GB
		}
		if !deviations.DuplexModeUnsupported(dut) {
			eth.DuplexMode = oc.Ethernet_DuplexMode_FULL
		}
	}
	return i
}

// opticalChannelAtTarget reports whether the optical channel state already matches the target
// configuration, so that an unnecessary REPLACE (which re-initializes the coherent DSP) is avoided.
func opticalChannelAtTarget(t *testing.T, dut *ondatra.DUTDevice, och string) bool {
	t.Helper()
	path := gnmi.OC().Component(och).OpticalChannel()
	freq, okFreq := gnmi.Lookup(t, dut, path.Frequency().State()).Val()
	pwr, okPwr := gnmi.Lookup(t, dut, path.TargetOutputPower().State()).Val()
	if !okFreq || !okPwr || freq != targetFrequencyMHz || math.Abs(pwr-targetOutputPowerdBm) > 0.01 {
		return false
	}
	if deviations.OperationalModeUnsupported(dut) {
		return true
	}
	mode, okMode := gnmi.Lookup(t, dut, path.OperationalMode().State()).Val()
	return okMode && mode == operationalMode
}

// logicalChannelAtTarget reports whether a logical channel already exists in state with the given
// type and an assignment to the given optical channel or logical channel.
func logicalChannelAtTarget(t *testing.T, dut *ondatra.DUTDevice, index uint32, lcType oc.E_TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE, och string, lc uint32) bool {
	t.Helper()
	ch, ok := gnmi.Lookup(t, dut, gnmi.OC().TerminalDevice().Channel(index).State()).Val()
	if !ok || ch.GetLogicalChannelType() != lcType {
		return false
	}
	for _, a := range ch.Assignment {
		if (och != "" && a.GetOpticalChannel() == och) || (lc != 0 && a.GetLogicalChannel() == lc) {
			return true
		}
	}
	return false
}

func configureETHChannel(t *testing.T, dut *ondatra.DUTDevice, p *zrPort) {
	t.Helper()
	var ingress *oc.TerminalDevice_Channel_Ingress
	if !deviations.EthChannelIngressParametersUnsupported(dut) {
		ingress = &oc.TerminalDevice_Channel_Ingress{
			Interface:   ygot.String(p.name),
			Transceiver: ygot.String(p.tr),
		}
	}
	assignIdx := uint32(0)
	if deviations.EthChannelAssignmentCiscoNumbering(dut) {
		assignIdx = 1
	}
	ch := &oc.TerminalDevice_Channel{
		Description:        ygot.String("ETH Logical Channel"),
		Index:              ygot.Uint32(p.ethIdx),
		LogicalChannelType: oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_ETHERNET,
		TribProtocol:       oc.TransportTypes_TRIBUTARY_PROTOCOL_TYPE_PROT_400GE,
		Ingress:            ingress,
		Assignment: map[uint32]*oc.TerminalDevice_Channel_Assignment{
			assignIdx: {
				Index:          ygot.Uint32(assignIdx),
				LogicalChannel: ygot.Uint32(p.otnIdx),
				Description:    ygot.String("ETH to OTN"),
				Allocation:     ygot.Float64(400),
				AssignmentType: oc.Assignment_AssignmentType_LOGICAL_CHANNEL,
			},
		},
	}
	if !deviations.TerminalDeviceChannelAdminStateUnsupported(dut) {
		ch.AdminState = oc.TerminalDevice_AdminStateType_ENABLED
	}
	if !deviations.ChannelRateClassParametersUnsupported(dut) {
		ch.RateClass = oc.TransportTypes_TRIBUTARY_RATE_CLASS_TYPE_TRIB_RATE_400G
	}
	gnmi.Replace(t, dut, gnmi.OC().TerminalDevice().Channel(p.ethIdx).Config(), ch)
}

// configureLink applies the 400ZR interface, transceiver, optical channel and logical channel
// configuration described in the Procedure. Optical and logical channels are only replaced when
// their state does not already match the target configuration.
func configureLink(t *testing.T, dut *ondatra.DUTDevice, p *zrPort) {
	t.Helper()
	gnmi.Replace(t, dut, gnmi.OC().Interface(p.name).Config(), newZRInterface(t, dut, p.name))
	if deviations.ExplicitDcoConfig(dut) {
		gnmi.Replace(t, dut, gnmi.OC().Component(p.tr).Config(), &oc.Component{
			Name: ygot.String(p.tr),
			Transceiver: &oc.Component_Transceiver{
				ModuleFunctionalType: oc.TransportTypes_TRANSCEIVER_MODULE_FUNCTIONAL_TYPE_TYPE_DIGITAL_COHERENT_OPTIC,
			},
		})
	}
	setTransceiverEnabled(t, dut, p.tr, true)
	if opticalChannelAtTarget(t, dut, p.och) {
		t.Logf("Optical channel %s already matches the target configuration", p.och)
	} else {
		cfgplugins.ConfigOpticalChannel(t, dut, p.och, targetFrequencyMHz, targetOutputPowerdBm, operationalMode)
	}
	if logicalChannelAtTarget(t, dut, p.otnIdx, oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_OTN, p.och, 0) {
		t.Logf("OTN logical channel %d already matches the target configuration", p.otnIdx)
	} else {
		cfgplugins.ConfigOTNChannel(t, dut, p.och, p.otnIdx, p.ethIdx)
	}
	if logicalChannelAtTarget(t, dut, p.ethIdx, oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_ETHERNET, "", p.otnIdx) {
		t.Logf("Ethernet logical channel %d already matches the target configuration", p.ethIdx)
	} else {
		configureETHChannel(t, dut, p)
	}
}

// configureL3 configures the dual-stack subinterface 0 addresses on the port.
func configureL3(t *testing.T, dut *ondatra.DUTDevice, p *zrPort) {
	t.Helper()
	intf := newZRInterface(t, dut, p.name)
	if deviations.InterfaceMacAddressFromHardwareRequired(dut) {
		intf.GetOrCreateEthernet().MacAddress = ygot.String(gnmi.Get(t, dut, gnmi.OC().Interface(p.name).Ethernet().HwMacAddress().State()))
	}
	sub := intf.GetOrCreateSubinterface(0)
	if deviations.InterfaceEnabled(dut) {
		sub.Enabled = ygot.Bool(true)
	}
	cfgplugins.ConfigureSubinterfaceIPs(sub, dut, p.ipv4, ipv4PrefixLen, p.ipv6, ipv6PrefixLen)
	if deviations.InterfaceEnabled(dut) {
		if !deviations.IPv4MissingEnabled(dut) {
			sub.GetOrCreateIpv4().Enabled = ygot.Bool(true)
		}
		sub.GetOrCreateIpv6().Enabled = ygot.Bool(true)
	}
	gnmi.Update(t, dut, gnmi.OC().Interface(p.name).Config(), intf)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p.name, deviations.DefaultNetworkInstance(dut), 0)
	}
}

// setInterfaceEnabled performs an administrative flap using only
// /interfaces/interface/config/enabled, leaving the rest of the interface configuration intact.
func setInterfaceEnabled(t *testing.T, dut *ondatra.DUTDevice, name string, enabled bool) {
	t.Helper()
	gnmi.Update(t, dut, gnmi.OC().Interface(name).Enabled().Config(), enabled)
}

func setTransceiverEnabled(t *testing.T, dut *ondatra.DUTDevice, tr string, enabled bool) {
	t.Helper()
	if deviations.TransceiverConfigEnableUnsupported(dut) {
		return
	}
	gnmi.Update(t, dut, gnmi.OC().Component(tr).Name().Config(), tr)
	gnmi.Update(t, dut, gnmi.OC().Component(tr).Transceiver().Enabled().Config(), enabled)
}

func awaitOperStatus(t *testing.T, dut *ondatra.DUTDevice, name string, want oc.E_Interface_OperStatus, timeout time.Duration) {
	t.Helper()
	gnmi.Await(t, dut, gnmi.OC().Interface(name).OperStatus().State(), timeout, want)
}

// restorePorts brings any port that is not UP back to the baseline configuration.
func restorePorts(t *testing.T, dut *ondatra.DUTDevice, ports []*zrPort, withL3 bool) {
	t.Helper()
	for _, p := range ports {
		if st, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(p.name).OperStatus().State()).Val(); ok && st == oc.Interface_OperStatus_UP {
			continue
		}
		configureLink(t, dut, p)
		if withL3 {
			configureL3(t, dut, p)
		}
		awaitOperStatus(t, dut, p.name, oc.Interface_OperStatus_UP, linkUpTimeout)
	}
}

// openFECStreams subscribes to every FEC uncorrectable blocks leaf that TRANSCEIVER-10.1.1
// requires. A leaf is skipped only when the matching "unsupported" deviation is set.
func openFECStreams(t *testing.T, dut *ondatra.DUTDevice, p *zrPort) *fecStreams {
	t.Helper()
	s := &fecStreams{
		otn: samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(p.otnIdx).Otn().FecUncorrectableBlocks().State(), sampleInterval),
	}
	if !deviations.TerminalDeviceEthFecUncorrectableBlocksUnsupported(dut) {
		s.eth = samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(p.ethIdx).Ethernet().FecUncorrectableBlocks().State(), sampleInterval)
	}
	if !deviations.OpticalChannelFecUncorrectableBlocksUnsupported(dut) {
		s.och = samplestream.New(t, dut, gnmi.OC().Component(p.och).OpticalChannel().FecUncorrectableBlocks().State(), sampleInterval)
	}
	t.Cleanup(s.close)
	return s
}

// awaitFECSample waits for the next PM interval sample. Only a sample received after the call
// starts counts, so a leaf that stops streaming cannot pass on a stale value. It reports an error
// and returns false if no new sample arrives.
func awaitFECSample(t *testing.T, s *samplestream.SampleStream[uint64], desc string) (uint64, bool) {
	t.Helper()
	received := len(s.All())
	v, ok := s.AwaitNext(sampleTimeout, func(v *ygnmi.Value[uint64]) bool {
		return v.IsPresent() && len(s.All()) > received
	})
	if !ok || v == nil {
		t.Errorf("%s: no new sample was streamed within %v", desc, sampleTimeout)
		return 0, false
	}
	val, _ := v.Val()
	return val, true
}

// settleAndValidateFEC discards one full PM interval sample, which may still cover an optical
// lock transition, and then verifies count consecutive PM interval samples. Each sample must be 0,
// or must not exceed the settling sample when deviations.NonIntervalFecErrorCounter is set.
func settleAndValidateFEC(t *testing.T, dut *ondatra.DUTDevice, s *samplestream.SampleStream[uint64], desc string, count int) {
	t.Helper()
	settled, ok := awaitFECSample(t, s, desc+" (settling interval)")
	if !ok {
		return
	}
	t.Logf("%s: settling interval sample = %d (discarded)", desc, settled)
	nonInterval := deviations.NonIntervalFecErrorCounter(dut)
	for i := 1; i <= count; i++ {
		got, ok := awaitFECSample(t, s, desc)
		if !ok {
			return
		}
		t.Logf("%s [PM interval %d/%d]: got %d", desc, i, count, got)
		switch {
		case nonInterval && got > settled:
			t.Errorf("%s [PM interval %d/%d]: cumulative counter increased on a stable link: got %d, want <= %d", desc, i, count, got, settled)
		case !nonInterval && got != 0:
			t.Errorf("%s [PM interval %d/%d]: got %d, want 0", desc, i, count, got)
		}
	}
}

// validateFECStreamed verifies that count consecutive PM interval samples are streamed for the
// leaf. Any value, including non-zero, is allowed; a missing (nil) sample fails.
func validateFECStreamed(t *testing.T, s *samplestream.SampleStream[uint64], desc string, count int) {
	t.Helper()
	for i := 1; i <= count; i++ {
		got, ok := awaitFECSample(t, s, desc)
		if !ok {
			return
		}
		t.Logf("%s [PM interval %d/%d]: got %d", desc, i, count, got)
	}
}

// validateAllFEC runs validateFECStreamed on every subscribed FEC leaf of every port.
func validateAllFEC(t *testing.T, ports []*zrPort, streams map[string]*fecStreams, step string, count int) {
	t.Helper()
	for _, p := range ports {
		s := streams[p.name]
		validateFECStreamed(t, s.otn, fmt.Sprintf("%s %s /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks", step, p.name), count)
		if s.eth != nil {
			validateFECStreamed(t, s.eth, fmt.Sprintf("%s %s /terminal-device/logical-channels/channel/ethernet/state/fec-uncorrectable-blocks", step, p.name), count)
		}
		if s.och != nil {
			validateFECStreamed(t, s.och, fmt.Sprintf("%s %s /components/component/optical-channel/state/fec-uncorrectable-blocks", step, p.name), count)
		}
	}
}

// TestZrUncorrectableFrames validates the baseline TRANSCEIVER-10.1 Procedure.
func TestZrUncorrectableFrames(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	operationalMode = cfgplugins.InterfaceInitialize(t, dut, uint16(*operationalModeFlag))
	ports := zrPorts(t, dut)
	t.Cleanup(func() { restorePorts(t, dut, ports, false) })
	for _, p := range ports {
		configureLink(t, dut, p)
	}

	for _, p := range ports {
		t.Run(fmt.Sprintf("Port:%s", p.port.ID()), func(t *testing.T) {
			awaitOperStatus(t, dut, p.name, oc.Interface_OperStatus_UP, intUpdateTime)
			stream := samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(p.otnIdx).Otn().FecUncorrectableBlocks().State(), sampleInterval)
			t.Cleanup(stream.Close)
			desc := p.name + " /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks"
			settleAndValidateFEC(t, dut, stream, desc+" (before flap)", 1)

			setInterfaceEnabled(t, dut, p.name, false)
			awaitOperStatus(t, dut, p.name, oc.Interface_OperStatus_DOWN, intUpdateTime)
			setInterfaceEnabled(t, dut, p.name, true)
			awaitOperStatus(t, dut, p.name, oc.Interface_OperStatus_UP, intUpdateTime)

			settleAndValidateFEC(t, dut, stream, desc+" (after flap)", 1)
		})
	}
}

// TestZrFecPMIntervalSemantics validates TRANSCEIVER-10.1.1: Validate 400ZR FEC PM Interval
// Semantics.
func TestZrFecPMIntervalSemantics(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	operationalMode = cfgplugins.InterfaceInitialize(t, dut, uint16(*operationalModeFlag))
	ports := zrPorts(t, dut)
	t.Cleanup(func() { restorePorts(t, dut, ports, true) })

	// Step 1: Configuration.
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	for _, p := range ports {
		configureLink(t, dut, p)
		configureL3(t, dut, p)
	}

	// Step 2: Baseline operational state and PM interval telemetry.
	streams := make(map[string]*fecStreams, len(ports))
	for _, p := range ports {
		awaitOperStatus(t, dut, p.name, oc.Interface_OperStatus_UP, linkUpTimeout)
		cfgplugins.ValidateInterfaceConfig(t, dut, p.port, targetOutputPowerdBm, targetFrequencyMHz, targetOutputPowerTolerancedBm, targetFrequencyToleranceMHz)
		if !deviations.OperationalModeUnsupported(dut) {
			if got := gnmi.Get(t, dut, gnmi.OC().Component(p.och).OpticalChannel().OperationalMode().State()); got != operationalMode {
				t.Errorf("Port %s optical-channel operational-mode: got %d, want %d", p.name, got, operationalMode)
			}
		}
		streams[p.name] = openFECStreams(t, dut, p)
	}
	validateAllFEC(t, ports, streams, "Step 2 (baseline)", 3)

	// Step 3a: Transient link disturbance via an administrative interface flap on port1.
	p1 := ports[0]
	setInterfaceEnabled(t, dut, p1.name, false)
	awaitOperStatus(t, dut, p1.name, oc.Interface_OperStatus_DOWN, linkUpTimeout)
	// The README requires port1 to stay DOWN for one PM interval before it is re-enabled.
	time.Sleep(sampleInterval)
	setInterfaceEnabled(t, dut, p1.name, true)
	for _, p := range ports {
		awaitOperStatus(t, dut, p.name, oc.Interface_OperStatus_UP, linkUpTimeout)
	}
	validateAllFEC(t, ports, streams, "Step 3a (after interface flap)", 2)

	// Step 3b: Transient link disturbance via a transceiver power toggle on port1.
	if deviations.TransceiverConfigEnableUnsupported(dut) {
		t.Logf("Skipping the transceiver toggle: deviation TransceiverConfigEnableUnsupported is set")
		return
	}
	setTransceiverEnabled(t, dut, p1.tr, false)
	awaitOperStatus(t, dut, p1.name, oc.Interface_OperStatus_DOWN, linkUpTimeout)
	setTransceiverEnabled(t, dut, p1.tr, true)
	for _, p := range ports {
		awaitOperStatus(t, dut, p.name, oc.Interface_OperStatus_UP, linkUpTimeout)
	}
	validateAllFEC(t, ports, streams, "Step 3b (after transceiver toggle)", 2)
}

// plqPlan holds the PLQ parameters derived from the DUT capabilities.
type plqPlan struct {
	packetSize  uint32
	packetRate  uint64
	usePMD      bool
	setup       time.Duration
	teardown    time.Duration
	completedBy time.Duration
}

func maxDuration(ds ...time.Duration) time.Duration {
	m := ds[0]
	for _, d := range ds[1:] {
		if d > m {
			m = d
		}
	}
	return m
}

// planPLQ verifies the gNOI LinkQualification capabilities required by TRANSCEIVER-10.1.2 and
// derives the generator rate, reflector type and timers.
func planPLQ(t *testing.T, dut *ondatra.DUTDevice) *plqPlan {
	t.Helper()
	resp, err := dut.RawAPIs().GNOI(t).LinkQualification().Capabilities(context.Background(), &plqpb.CapabilitiesRequest{})
	if err != nil {
		t.Fatalf("LinkQualification.Capabilities() failed: %v", err)
	}
	t.Logf("LinkQualification.Capabilities(): %v", resp)

	gen := resp.GetGenerator().GetPacketGenerator()
	if gen == nil {
		t.Fatalf("DUT %s does not report packet generator capabilities", dut.Name())
	}
	packetSize := plqPacketSize
	if dev := deviations.PLQGeneratorCapabilitiesMaxMTU(dut); dev != 0 && dev < packetSize {
		packetSize = dev
	}
	if got := gen.GetMaxMtu(); got < packetSize {
		t.Errorf("Generator max_mtu: got %d, want >= %d", got, packetSize)
	}
	minBps := plqMinGeneratorBps
	if dev := deviations.PLQGeneratorCapabilitiesMaxBPS(dut); dev != 0 && dev < minBps {
		minBps = dev
	}
	if got := gen.GetMaxBps(); got < minBps {
		t.Errorf("Generator max_bps: got %d, want >= %d", got, minBps)
	}

	// Run at 99% of the 400ZR line rate, capped by what the generator supports. A zero capability
	// value means the generator reports no limit.
	packetRate := plqLineRateBps * plqLineRatePercent / 100 / (8 * (uint64(packetSize) + plqFrameOverhead))
	if r := gen.GetMaxBps() / (8 * uint64(packetSize)); r != 0 && r < packetRate {
		packetRate = r
	}
	if r := gen.GetMaxPps(); r != 0 && r < packetRate {
		packetRate = r
	}
	if r := deviations.PLQGeneratorCapabilitiesMaxPPS(dut); r != 0 && r < packetRate {
		packetRate = r
	}
	t.Logf("PLQ generator: packet_size=%d bytes, packet_rate=%d pps (%.2f Gbps)", packetSize, packetRate, float64(packetRate*uint64(packetSize)*8)/1e9)

	ref := resp.GetReflector()
	var refSetup, refTeardown time.Duration
	usePMD := false
	switch {
	case ref.GetAsicLoopback().GetMinSetupDuration().AsDuration() > 0:
		refSetup = ref.GetAsicLoopback().GetMinSetupDuration().AsDuration()
		refTeardown = ref.GetAsicLoopback().GetMinTeardownDuration().AsDuration()
	case ref.GetPmdLoopback().GetMinSetupDuration().AsDuration() > 0:
		usePMD = true
		refSetup = ref.GetPmdLoopback().GetMinSetupDuration().AsDuration()
		refTeardown = ref.GetPmdLoopback().GetMinTeardownDuration().AsDuration()
	default:
		t.Fatalf("DUT %s does not report ASIC or PMD loopback reflector capabilities: %v", dut.Name(), ref)
	}
	setup := maxDuration(plqMinSetup, gen.GetMinSetupDuration().AsDuration(), refSetup)
	teardown := maxDuration(plqMinTeardown, gen.GetMinTeardownDuration().AsDuration(), refTeardown)
	return &plqPlan{
		packetSize:  packetSize,
		packetRate:  packetRate,
		usePMD:      usePMD,
		setup:       setup,
		teardown:    teardown,
		completedBy: plqGeneratorPreSync + setup + plqDuration + plqReflectorPostSync + teardown + 2*time.Minute,
	}
}

func createPLQ(t *testing.T, dut *ondatra.DUTDevice, gen, ref *zrPort, plan *plqPlan) {
	t.Helper()
	reflector := &plqpb.QualificationConfiguration{
		Id:            farEndQualID,
		InterfaceName: ref.name,
		Timing: &plqpb.QualificationConfiguration_Rpc{
			Rpc: &plqpb.RPCSyncedTiming{
				PreSyncDuration:  durationpb.New(0),
				SetupDuration:    durationpb.New(plan.setup),
				Duration:         durationpb.New(plqDuration),
				PostSyncDuration: durationpb.New(plqReflectorPostSync),
				TeardownDuration: durationpb.New(plan.teardown),
			},
		},
	}
	if plan.usePMD {
		reflector.EndpointType = &plqpb.QualificationConfiguration_PmdLoopback{PmdLoopback: &plqpb.PmdLoopbackConfiguration{}}
	} else {
		reflector.EndpointType = &plqpb.QualificationConfiguration_AsicLoopback{AsicLoopback: &plqpb.AsicLoopbackConfiguration{}}
	}
	generator := &plqpb.QualificationConfiguration{
		Id:            nearEndQualID,
		InterfaceName: gen.name,
		Timing: &plqpb.QualificationConfiguration_Rpc{
			Rpc: &plqpb.RPCSyncedTiming{
				PreSyncDuration:  durationpb.New(plqGeneratorPreSync),
				SetupDuration:    durationpb.New(plan.setup),
				Duration:         durationpb.New(plqDuration),
				PostSyncDuration: durationpb.New(0),
				TeardownDuration: durationpb.New(plan.teardown),
			},
		},
		EndpointType: &plqpb.QualificationConfiguration_PacketGenerator{
			PacketGenerator: &plqpb.PacketGeneratorConfiguration{
				PacketRate: plan.packetRate,
				PacketSize: plan.packetSize,
			},
		},
	}
	resp, err := dut.RawAPIs().GNOI(t).LinkQualification().Create(context.Background(), &plqpb.CreateRequest{
		Interfaces: []*plqpb.QualificationConfiguration{generator, reflector},
	})
	if err != nil {
		t.Fatalf("LinkQualification.Create() failed: %v", err)
	}
	for id, st := range resp.GetStatus() {
		if st.GetCode() != 0 {
			t.Fatalf("LinkQualification.Create() returned status %v for %s", st, id)
		}
	}
}

// awaitPLQStates polls LinkQualification.List until every qualification ID reaches want. The gNOI
// LinkQualification service has no streaming RPC, so polling is required.
func awaitPLQStates(t *testing.T, dut *ondatra.DUTDevice, want plqpb.QualificationState, timeout time.Duration, ids ...string) {
	t.Helper()
	client := dut.RawAPIs().GNOI(t).LinkQualification()
	deadline := time.Now().Add(timeout)
	for {
		resp, err := client.List(context.Background(), &plqpb.ListRequest{})
		if err != nil {
			t.Fatalf("LinkQualification.List() failed: %v", err)
		}
		reached := 0
		for _, res := range resp.GetResults() {
			for _, id := range ids {
				if res.GetId() != id {
					continue
				}
				if res.GetState() == plqpb.QualificationState_QUALIFICATION_STATE_ERROR {
					details, _ := client.Get(context.Background(), &plqpb.GetRequest{Ids: ids})
					t.Fatalf("PLQ session %s on %s entered QUALIFICATION_STATE_ERROR: %v", id, res.GetInterfaceName(), details)
				}
				if res.GetState() == want || (want == plqpb.QualificationState_QUALIFICATION_STATE_RUNNING && res.GetState() == plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED) {
					reached++
				}
			}
		}
		if reached == len(ids) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("PLQ sessions %v did not reach %v within %v: %v", ids, want, timeout, resp.GetResults())
		}
		time.Sleep(plqStatePollInterval)
	}
}

func verifyPLQResults(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	ids := []string{nearEndQualID, farEndQualID}
	resp, err := dut.RawAPIs().GNOI(t).LinkQualification().Get(context.Background(), &plqpb.GetRequest{Ids: ids})
	if err != nil {
		t.Fatalf("LinkQualification.Get() failed: %v", err)
	}
	for _, id := range ids {
		res, ok := resp.GetResults()[id]
		if !ok {
			t.Errorf("LinkQualification.Get() has no result for %q", id)
			continue
		}
		t.Logf("LinkQualification.Get() result for %s: %v", id, res)
		if got := res.GetStatus().GetCode(); got != 0 {
			t.Errorf("Result[%s] status code: got %d, want 0", id, got)
		}
		if got := res.GetState(); got != plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED {
			t.Errorf("Result[%s] state: got %v, want QUALIFICATION_STATE_COMPLETED", id, got)
		}
		if got := res.GetPacketsError(); got != 0 {
			t.Errorf("Result[%s] packets_error: got %d, want 0", id, got)
		}
		if got := res.GetPacketsDropped(); got != 0 {
			t.Errorf("Result[%s] packets_dropped: got %d, want 0", id, got)
		}
		if id == farEndQualID && deviations.PLQReflectorStatsUnsupported(dut) {
			continue
		}
		sent, received := res.GetPacketsSent(), res.GetPacketsReceived()
		if sent == 0 || sent != received {
			t.Errorf("Result[%s]: packets_sent=%d, packets_received=%d, want equal and non-zero", id, sent, received)
		}
	}
}

// cleanupPLQ deletes every qualification session on the given ports and waits until none remain.
func cleanupPLQ(t *testing.T, dut *ondatra.DUTDevice, ports []*zrPort) {
	t.Helper()
	client := dut.RawAPIs().GNOI(t).LinkQualification()
	onPorts := func() ([]string, error) {
		resp, err := client.List(context.Background(), &plqpb.ListRequest{})
		if err != nil {
			return nil, err
		}
		var ids []string
		for _, res := range resp.GetResults() {
			for _, p := range ports {
				if res.GetInterfaceName() == p.name {
					ids = append(ids, res.GetId())
				}
			}
		}
		return ids, nil
	}
	ids, err := onPorts()
	if err != nil {
		t.Errorf("LinkQualification.List() failed: %v", err)
		return
	}
	if len(ids) == 0 {
		return
	}
	if _, err := client.Delete(context.Background(), &plqpb.DeleteRequest{Ids: ids}); err != nil {
		t.Errorf("LinkQualification.Delete(%v) failed: %v", ids, err)
		return
	}
	deadline := time.Now().Add(plqDeleteTimeout)
	for {
		remaining, err := onPorts()
		if err == nil && len(remaining) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Errorf("LinkQualification.List() still has sessions %v (err: %v) after Delete", remaining, err)
			return
		}
		time.Sleep(plqStatePollInterval)
	}
}

// verifyIPState fails the test if the configured IPv4 or IPv6 address is missing from state.
func verifyIPState(t *testing.T, dut *ondatra.DUTDevice, p *zrPort, phase string) {
	t.Helper()
	sub := gnmi.OC().Interface(p.name).Subinterface(0)
	if _, ok := gnmi.Watch(t, dut, sub.Ipv4().Address(p.ipv4).Ip().State(), ipStateTimeout, func(v *ygnmi.Value[string]) bool {
		ip, present := v.Val()
		return present && ip == p.ipv4
	}).Await(t); !ok {
		t.Errorf("%s: port %s IPv4 address %s is not present in state", phase, p.name, p.ipv4)
	}
	if _, ok := gnmi.Watch(t, dut, sub.Ipv6().Address(p.ipv6).Ip().State(), ipStateTimeout, func(v *ygnmi.Value[string]) bool {
		ip, present := v.Val()
		return present && ip == p.ipv6
	}).Await(t); !ok {
		t.Errorf("%s: port %s IPv6 address %s is not present in state", phase, p.name, p.ipv6)
	}
}

// TestZrLinkQualification validates TRANSCEIVER-10.1.2: Validate 400ZR Link Qualification and
// Post-Qualification Link Health.
func TestZrLinkQualification(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	operationalMode = cfgplugins.InterfaceInitialize(t, dut, uint16(*operationalModeFlag))
	ports := zrPorts(t, dut)
	t.Cleanup(func() {
		cleanupPLQ(t, dut, ports)
		restorePorts(t, dut, ports, true)
	})

	// Step 1: Link configuration and capabilities check.
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	for _, p := range ports {
		configureLink(t, dut, p)
		configureL3(t, dut, p)
	}
	otnStreams := make(map[string]*samplestream.SampleStream[uint64], len(ports))
	inFcsBaseline := make(map[string]uint64, len(ports))
	for _, p := range ports {
		awaitOperStatus(t, dut, p.name, oc.Interface_OperStatus_UP, linkUpTimeout)
		verifyIPState(t, dut, p, "Before PLQ")
		otnStreams[p.name] = samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(p.otnIdx).Otn().FecUncorrectableBlocks().State(), sampleInterval)
		t.Cleanup(otnStreams[p.name].Close)
		if !deviations.InterfaceCountersInFcsErrorsUnsupported(dut) {
			inFcsBaseline[p.name] = gnmi.Get(t, dut, gnmi.OC().Interface(p.name).Counters().InFcsErrors().State())
		}
	}
	plan := planPLQ(t, dut)

	// Step 2: Packet Link Qualification.
	cleanupPLQ(t, dut, ports)
	createPLQ(t, dut, ports[0], ports[1], plan)
	if !deviations.SkipPlqInterfaceOperStatusCheck(dut) {
		for _, p := range ports {
			awaitOperStatus(t, dut, p.name, oc.Interface_OperStatus_TESTING, plqGeneratorPreSync+plan.setup+time.Minute)
		}
	}
	// Check FEC only once the generator is sending packets.
	awaitPLQStates(t, dut, plqpb.QualificationState_QUALIFICATION_STATE_RUNNING, plqGeneratorPreSync+plan.setup+time.Minute, nearEndQualID)
	for _, p := range ports {
		settleAndValidateFEC(t, dut, otnStreams[p.name], "Step 2 (during PLQ) "+p.name+" /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks", 1)
	}
	awaitPLQStates(t, dut, plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED, plan.completedBy, nearEndQualID, farEndQualID)
	verifyPLQResults(t, dut)
	cleanupPLQ(t, dut, ports)

	// Step 3: Post-qualification link health.
	for _, p := range ports {
		awaitOperStatus(t, dut, p.name, oc.Interface_OperStatus_UP, linkUpTimeout)
		verifyIPState(t, dut, p, "After PLQ")
		if !deviations.InterfaceCountersInFcsErrorsUnsupported(dut) {
			if got := gnmi.Get(t, dut, gnmi.OC().Interface(p.name).Counters().InFcsErrors().State()); got != inFcsBaseline[p.name] {
				t.Errorf("Port %s in-fcs-errors changed across PLQ: before=%d, after=%d", p.name, inFcsBaseline[p.name], got)
			}
		}
		settleAndValidateFEC(t, dut, otnStreams[p.name], "Step 3 (after PLQ) "+p.name+" /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks", 1)
	}
}
