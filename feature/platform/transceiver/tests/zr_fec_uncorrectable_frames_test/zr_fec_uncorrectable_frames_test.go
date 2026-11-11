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
	"reflect"
	"testing"
	"time"

	"flag"

	"github.com/google/go-cmp/cmp"
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

	port1IPv4      = "192.0.2.1"
	port2IPv4      = "192.0.2.2"
	ipv4PrefixLen  = uint8(30)
	destIPv4Prefix = "198.51.100.0/24"
	port1IPv6      = "2001:db8::1"
	port2IPv6      = "2001:db8::2"
	ipv6PrefixLen  = uint8(126)
	destIPv6Prefix = "2001:db8:1::/64"

	nearEndQualID    = "qual-400zr-near"
	farEndQualID     = "qual-400zr-far"
	plqMinBps        = uint64(100_000_000_000)
	plqPacketRate    = uint64(138888)
	plqPacketSize    = uint32(8184)
	plqSetupDuration = 30 * time.Second
	plqPreSync       = 10 * time.Second
	plqDuration      = 30 * time.Second
	plqPostSync      = 30 * time.Second
	plqTeardown      = 30 * time.Second
)

var (
	operationalModeFlag = flag.Int("operational_mode", 0, "vendor-specific operational-mode for the channel.")
	operationalMode     uint16
)

type portFECStreams struct {
	otnUncorrectable  *samplestream.SampleStream[uint64]
	ethUncorrectable  *samplestream.SampleStream[uint64]
	ochUncorrectable  *samplestream.SampleStream[uint64]
	intfUncorrectable *samplestream.SampleStream[uint64]
	intfCorrected     *samplestream.SampleStream[uint64]
	inFcsErrors       *samplestream.SampleStream[uint64]
}

func (s *portFECStreams) closeAll() {
	for _, st := range []*samplestream.SampleStream[uint64]{
		s.otnUncorrectable, s.ethUncorrectable, s.ochUncorrectable,
		s.intfUncorrectable, s.intfCorrected, s.inFcsErrors,
	} {
		if st != nil {
			st.Close()
		}
	}
}

func (s *portFECStreams) primaryUncorrectableStream(t *testing.T, dut *ondatra.DUTDevice, dp *ondatra.Port, otnIdx uint32) *samplestream.SampleStream[uint64] {
	t.Helper()
	if dp.PMD() != ondatra.PMD400GBASEZR && dp.PMD() != ondatra.PMD400GBASEZRP {
		if _, ok := gnmi.Lookup(t, dut, gnmi.OC().TerminalDevice().Channel(otnIdx).Otn().FecUncorrectableBlocks().State()).Val(); !ok && s.inFcsErrors != nil {
			return s.inFcsErrors
		}
	}
	if s.otnUncorrectable != nil && len(s.otnUncorrectable.All()) > 0 {
		return s.otnUncorrectable
	}
	if _, ok := gnmi.Lookup(t, dut, gnmi.OC().TerminalDevice().Channel(otnIdx).Otn().FecUncorrectableBlocks().State()).Val(); ok {
		return s.otnUncorrectable
	}
	for _, candidate := range []*samplestream.SampleStream[uint64]{s.ochUncorrectable, s.ethUncorrectable, s.intfUncorrectable, s.inFcsErrors} {
		if candidate != nil && len(candidate.All()) > 0 {
			return candidate
		}
	}
	return s.otnUncorrectable
}

func (s *portFECStreams) getPrimaryUncorrectableBlocks(t *testing.T, dut *ondatra.DUTDevice, dp *ondatra.Port, otnIdx uint32) uint64 {
	t.Helper()
	if val, ok := gnmi.Lookup(t, dut, gnmi.OC().TerminalDevice().Channel(otnIdx).Otn().FecUncorrectableBlocks().State()).Val(); ok {
		return val
	}
	if s.otnUncorrectable != nil {
		if all := s.otnUncorrectable.All(); len(all) > 0 {
			if val, ok := all[len(all)-1].Val(); ok {
				return val
			}
		}
	}
	if val, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(dp.Name()).Counters().InFcsErrors().State()).Val(); ok {
		return val
	}
	return 0
}

func isTelemetryPathPresent[T any](t *testing.T, dut *ondatra.DUTDevice, q ygnmi.SingletonQuery[T]) bool {
	c, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.ID()))
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	val, err := ygnmi.Lookup(ctx, c, q)
	return err == nil && val != nil && val.IsPresent()
}

func configureInterfaceAndOptics(t *testing.T, dut *ondatra.DUTDevice, dp *ondatra.Port, och, tr string, otnIdx, ethIdx uint32) {
	t.Helper()
	d := &oc.Root{}
	i := d.GetOrCreateInterface(dp.Name())
	i.Enabled = ygot.Bool(true)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if dut.Vendor() == ondatra.JUNIPER && !deviations.OmitL2MTU(dut) {
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
	gnmi.Replace(t, dut, gnmi.OC().Interface(dp.Name()).Config(), i)
	if deviations.ExplicitDcoConfig(dut) {
		gnmi.Replace(t, dut, gnmi.OC().Component(tr).Config(), &oc.Component{
			Name: ygot.String(tr),
			Transceiver: &oc.Component_Transceiver{
				ModuleFunctionalType: oc.TransportTypes_TRANSCEIVER_MODULE_FUNCTIONAL_TYPE_TYPE_DIGITAL_COHERENT_OPTIC,
			},
		})
	}
	if dp.PMD() != ondatra.PMD400GBASEZR && dp.PMD() != ondatra.PMD400GBASEZRP && dut.Vendor() == ondatra.CISCO {
		return
	}
	if dut.Vendor() == ondatra.CISCO {
		curFreq, okFreq := gnmi.Lookup(t, dut, gnmi.OC().Component(och).OpticalChannel().Frequency().State()).Val()
		curMode, okMode := gnmi.Lookup(t, dut, gnmi.OC().Component(och).OpticalChannel().OperationalMode().State()).Val()
		curPwr, okPwr := gnmi.Lookup(t, dut, gnmi.OC().Component(och).OpticalChannel().TargetOutputPower().State()).Val()
		if okFreq && okMode && okPwr && curFreq == uint64(targetFrequencyMHz) && curMode == operationalMode && math.Abs(curPwr-targetOutputPowerdBm) <= 0.5 {
			return
		}
	}
	cfgplugins.ConfigOpticalChannel(t, dut, och, targetFrequencyMHz, targetOutputPowerdBm, operationalMode)
	cfgplugins.ConfigOTNChannel(t, dut, och, otnIdx, ethIdx)
	var ingress *oc.TerminalDevice_Channel_Ingress
	if !deviations.EthChannelIngressParametersUnsupported(dut) {
		ingress = &oc.TerminalDevice_Channel_Ingress{
			Interface:   ygot.String(dp.Name()),
			Transceiver: ygot.String(tr),
		}
	}
	assignIdx := uint32(0)
	if deviations.EthChannelAssignmentCiscoNumbering(dut) {
		assignIdx = 1
	}
	ch := &oc.TerminalDevice_Channel{
		Description:        ygot.String("ETH Logical Channel"),
		Index:              ygot.Uint32(ethIdx),
		LogicalChannelType: oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_ETHERNET,
		TribProtocol:       oc.TransportTypes_TRIBUTARY_PROTOCOL_TYPE_PROT_400GE,
		Ingress:            ingress,
		Assignment: map[uint32]*oc.TerminalDevice_Channel_Assignment{
			0: {
				Index:          ygot.Uint32(assignIdx),
				LogicalChannel: ygot.Uint32(otnIdx),
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
	gnmi.Replace(t, dut, gnmi.OC().TerminalDevice().Channel(ethIdx).Config(), ch)
}

func toggleInterface(t *testing.T, dut *ondatra.DUTDevice, intf string, isEnabled bool) {
	t.Helper()
	d := &oc.Root{}
	i := d.GetOrCreateInterface(intf)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	i.Enabled = ygot.Bool(isEnabled)
	if !deviations.PortSpeedDuplexModeUnsupportedForInterfaceConfig(dut) && (dut.Vendor() == ondatra.ARISTA || deviations.ExplicitPortSpeed(dut)) {
		eth := i.GetOrCreateEthernet()
		if !deviations.PortSpeedUnsupported(dut) {
			eth.PortSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_400GB
		}
		if !deviations.DuplexModeUnsupported(dut) {
			eth.DuplexMode = oc.Ethernet_DuplexMode_FULL
		}
	}
	gnmi.Replace(t, dut, gnmi.OC().Interface(intf).Config(), i)
}

func toggleTransceiver(t *testing.T, dut *ondatra.DUTDevice, trName string, isEnabled bool) {
	t.Helper()
	if deviations.TransceiverConfigEnableUnsupported(dut) {
		return
	}
	gnmi.Update(t, dut, gnmi.OC().Component(trName).Name().Config(), trName)
	gnmi.Update(t, dut, gnmi.OC().Component(trName).Transceiver().Enabled().Config(), isEnabled)
}

func configureL3Subinterface(t *testing.T, dut *ondatra.DUTDevice, portName, ipv4Addr, ipv6Addr string) {
	t.Helper()
	intf := &oc.Interface{
		Name:    ygot.String(portName),
		Type:    oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		Enabled: ygot.Bool(true),
	}
	if dut.Vendor() == ondatra.JUNIPER && !deviations.OmitL2MTU(dut) {
		intf.Mtu = ygot.Uint16(9000)
	}
	if !deviations.PortSpeedDuplexModeUnsupportedForInterfaceConfig(dut) && (dut.Vendor() == ondatra.ARISTA || deviations.ExplicitPortSpeed(dut)) {
		eth := intf.GetOrCreateEthernet()
		if !deviations.PortSpeedUnsupported(dut) {
			eth.PortSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_400GB
		}
		if !deviations.DuplexModeUnsupported(dut) {
			eth.DuplexMode = oc.Ethernet_DuplexMode_FULL
		}
		if dut.Vendor() == ondatra.ARISTA {
			if hwMac, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(portName).Ethernet().HwMacAddress().State()).Val(); ok && hwMac != "" {
				eth.MacAddress = ygot.String(hwMac)
			}
		}
	}
	sub := intf.GetOrCreateSubinterface(0)
	if deviations.InterfaceEnabled(dut) {
		sub.Enabled = ygot.Bool(true)
	}
	cfgplugins.ConfigureSubinterfaceIPs(sub, dut, ipv4Addr, ipv4PrefixLen, ipv6Addr, ipv6PrefixLen)
	if deviations.InterfaceEnabled(dut) {
		if ipv4Addr != "" && !deviations.IPv4MissingEnabled(dut) {
			sub.GetOrCreateIpv4().Enabled = ygot.Bool(true)
		}
		if ipv6Addr != "" {
			sub.GetOrCreateIpv6().Enabled = ygot.Bool(true)
		}
	}
	gnmi.Update(t, dut, gnmi.OC().Interface(portName).Config(), intf)
}

func await400ZROpticalLinkUp(t *testing.T, dut *ondatra.DUTDevice, portName, ochName string, timeout time.Duration) {
	t.Helper()
	if st, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(portName).OperStatus().State()).Val(); !ok || st != oc.Interface_OperStatus_UP {
		gnmi.Await(t, dut, gnmi.OC().Interface(portName).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
	}
	ochPath := gnmi.OC().Component(ochName).OpticalChannel()
	if _, ok := gnmi.Lookup(t, dut, ochPath.LaserBiasCurrent().Instant().State()).Val(); !ok && dut.Vendor() == ondatra.CISCO {
		return
	}
	if cur, ok := gnmi.Lookup(t, dut, ochPath.LaserBiasCurrent().Instant().State()).Val(); !ok || cur <= 0.0 {
		if val, ok := gnmi.Watch(t, dut, ochPath.LaserBiasCurrent().Instant().State(), timeout, func(v *ygnmi.Value[float64]) bool {
			cur, present := v.Val()
			return present && cur > 0.0
		}).Await(t); !ok {
			t.Fatalf("Optical channel %s laser-bias-current/instant did not exceed 0.0 mA within %v (last: %v)", ochName, timeout, val)
		}
	}
	if pwr, ok := gnmi.Lookup(t, dut, ochPath.OutputPower().Instant().State()).Val(); !ok || pwr <= -20.0 {
		if val, ok := gnmi.Watch(t, dut, ochPath.OutputPower().Instant().State(), timeout, func(v *ygnmi.Value[float64]) bool {
			pwr, present := v.Val()
			return present && pwr > -20.0
		}).Await(t); !ok {
			t.Fatalf("Optical channel %s output-power/instant did not exceed -20.0 dBm within %v (last: %v)", ochName, timeout, val)
		}
	}
	if inPwr, ok := gnmi.Lookup(t, dut, ochPath.InputPower().Instant().State()).Val(); ok && inPwr > -20.0 {
		return
	}
	if _, ok := gnmi.Lookup(t, dut, ochPath.InputPower().Instant().State()).Val(); !ok {
		if trName, trOK := gnmi.Lookup(t, dut, gnmi.OC().Interface(portName).Transceiver().State()).Val(); trOK {
			chPath := gnmi.OC().Component(trName).Transceiver().Channel(0).InputPower().Instant().State()
			if chPwr, chOK := gnmi.Lookup(t, dut, chPath).Val(); chOK {
				if chPwr <= -20.0 {
					gnmi.Watch(t, dut, chPath, timeout, func(v *ygnmi.Value[float64]) bool {
						pwr, present := v.Val()
						return present && pwr > -20.0
					}).Await(t)
				}
				return
			}
		}
	}
	if val, ok := gnmi.Watch(t, dut, ochPath.InputPower().Instant().State(), timeout, func(v *ygnmi.Value[float64]) bool {
		pwr, present := v.Val()
		return present && pwr > -20.0
	}).Await(t); !ok {
		t.Fatalf("Optical channel %s input-power/instant did not exceed -20.0 dBm within %v (last: %v)", ochName, timeout, val)
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
		if currentFec > baselineValue {
			t.Fatalf("FecUncorrectableBlocks increased after flap: baseline=%d, current=%d, diff=%d",
				baselineValue, currentFec, currentFec-baselineValue)
		}
		t.Logf("FecUncorrectableBlocks not increasing: baseline=%d, current=%d", baselineValue, currentFec)
	} else if currentFec != 0 {
		t.Fatalf("Got FecUncorrectableBlocks got %d, want 0", currentFec)
	}
}

// TestZrUncorrectableFrames validates the baseline TRANSCEIVER-10.1 Procedure.
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
		trs[dp.Name()] = gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).Transceiver().State())
		ochs[dp.Name()] = components.OpticalChannelComponentFromPort(t, dut, dp)
		otnIndexes[dp.Name()] = otnIndexBase + uint32(i)
		ethIndexes[dp.Name()] = ethIndexBase + uint32(i)
		configureInterfaceAndOptics(t, dut, dp, ochs[dp.Name()], trs[dp.Name()], otnIndexes[dp.Name()], ethIndexes[dp.Name()])
	}

	for _, port := range ports {
		t.Run(fmt.Sprintf("Port:%s", port), func(t *testing.T) {
			dp := dut.Port(t, port)
			gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_UP)
			s := &portFECStreams{
				otnUncorrectable: samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndexes[dp.Name()]).Otn().FecUncorrectableBlocks().State(), sampleInterval),
			}
			if dp.PMD() != ondatra.PMD400GBASEZR && dp.PMD() != ondatra.PMD400GBASEZRP {
				s.inFcsErrors = samplestream.New(t, dut, gnmi.OC().Interface(dp.Name()).Counters().InFcsErrors().State(), sampleInterval)
			}
			t.Cleanup(s.closeAll)
			streamFecOtn := s.primaryUncorrectableStream(t, dut, dp, otnIndexes[dp.Name()])

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

			validateFecUncorrectableBlocks(t, streamFecOtn, baselineFec)

			toggleInterface(t, dut, dp.Name(), false)
			gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), intUpdateTime, oc.Interface_OperStatus_DOWN)

			if dut.Vendor() == ondatra.NOKIA || dut.Vendor() == ondatra.JUNIPER {
				configureInterfaceAndOptics(t, dut, dp, ochs[dp.Name()], trs[dp.Name()], otnIndexes[dp.Name()], ethIndexes[dp.Name()])
			} else {
				toggleInterface(t, dut, dp.Name(), true)
			}
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

			validateFecUncorrectableBlocks(t, streamFecOtn, baselineFec1)
		})
	}
}

func validateFECSampleStream(t *testing.T, dut *ondatra.DUTDevice, stream *samplestream.SampleStream[uint64], pathDesc string, count int, baseline uint64) uint64 {
	t.Helper()
	wantVal := uint64(0)
	if deviations.NonIntervalFecErrorCounter(dut) {
		wantVal = baseline
	}
	var lastGot uint64
	for idx := 0; idx < count; idx++ {
		sample, ok := stream.AwaitNext(sampleTimeout, func(v *ygnmi.Value[uint64]) bool {
			_, present := v.Val()
			return present
		})
		if !ok || sample == nil {
			t.Fatalf("%s: failed to receive PM interval sample %d/%d", pathDesc, idx+1, count)
		}
		val, ok := sample.Val()
		if !ok {
			t.Fatalf("%s [sample %d/%d]: telemetry value is empty", pathDesc, idx+1, count)
		}
		lastGot = val
		t.Logf("%s [PM interval %d/%d]: got=%d, want=%d (NonIntervalFecErrorCounter=%v)", pathDesc, idx+1, count, val, wantVal, deviations.NonIntervalFecErrorCounter(dut))
		if val != wantVal {
			t.Errorf("%s [PM interval %d/%d]: got %d, want %d", pathDesc, idx+1, count, val, wantVal)
		}
	}
	return lastGot
}

func verifySecondaryFECStreams(t *testing.T, dut *ondatra.DUTDevice, portName string, streams *portFECStreams) {
	t.Helper()
	aux := []struct {
		desc          string
		stream        *samplestream.SampleStream[uint64]
		uncorrectable bool
		cumulative    bool
	}{
		{desc: portName + " /terminal-device/logical-channels/channel/ethernet/state/fec-uncorrectable-blocks", stream: streams.ethUncorrectable, uncorrectable: true},
		{desc: portName + " /components/component/optical-channel/state/fec-uncorrectable-blocks", stream: streams.ochUncorrectable, uncorrectable: true},
		{desc: portName + " /interfaces/interface/ethernet/state/counters/fec-uncorrectable-blocks", stream: streams.intfUncorrectable, uncorrectable: true, cumulative: true},
		{desc: portName + " /interfaces/interface/ethernet/state/counters/fec-corrected-blocks", stream: streams.intfCorrected, uncorrectable: false, cumulative: true},
	}
	for _, item := range aux {
		if item.stream == nil {
			continue
		}
		if all := item.stream.All(); len(all) > 0 {
			if val, ok := all[len(all)-1].Val(); ok {
				t.Logf("%s: latest streamed value = %d", item.desc, val)
				if item.uncorrectable {
					if item.cumulative || deviations.NonIntervalFecErrorCounter(dut) {
						if len(all) >= 2 {
							if prev, ok := all[len(all)-2].Val(); ok && val > prev {
								t.Errorf("%s: counter increased over steady-state interval: prev=%d, latest=%d", item.desc, prev, val)
							}
						}
					} else if val != 0 {
						t.Errorf("%s: got %d, want 0", item.desc, val)
					}
				}
			}
		}
	}
}

func configurePortL3AndOptical(t *testing.T, dut *ondatra.DUTDevice, dp *ondatra.Port, trName, ochName string, otnIdx, ethIdx uint32, ipv4Addr, ipv6Addr string) {
	t.Helper()
	configureInterfaceAndOptics(t, dut, dp, ochName, trName, otnIdx, ethIdx)
	toggleTransceiver(t, dut, trName, true)
	configureL3Subinterface(t, dut, dp.Name(), ipv4Addr, ipv6Addr)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, dp.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

// TestZrFecPMIntervalSemantics validates TRANSCEIVER-10.1.1: Validate 400ZR FEC PM Interval Semantics.
func TestZrFecPMIntervalSemantics(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	ports := []*ondatra.Port{dp1, dp2}

	otnIndexes := map[string]uint32{dp1.Name(): otnIndex1, dp2.Name(): otnIndex2}
	ethIndexes := map[string]uint32{dp1.Name(): ethIndex1, dp2.Name(): ethIndex2}
	ipv4Addrs := map[string]string{dp1.Name(): port1IPv4, dp2.Name(): port2IPv4}
	ipv6Addrs := map[string]string{dp1.Name(): port1IPv6, dp2.Name(): port2IPv6}
	if dut.Vendor() == ondatra.ARISTA || dut.Vendor() == ondatra.JUNIPER || dut.Vendor() == ondatra.CISCO {
		ipv4Addrs[dp2.Name()] = "192.0.2.5"
		ipv6Addrs[dp2.Name()] = "2001:db8::5"
	}

	trs := make(map[string]string, len(ports))
	ochs := make(map[string]string, len(ports))
	for _, dp := range ports {
		trs[dp.Name()] = gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).Transceiver().State())
		ochs[dp.Name()] = components.OpticalChannelComponentFromPort(t, dut, dp)
	}

	operationalMode = uint16(*operationalModeFlag)
	operationalMode = cfgplugins.InterfaceInitialize(t, dut, operationalMode)

	t.Cleanup(func() {
		for _, dp := range ports {
			if st, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State()).Val(); !ok || st != oc.Interface_OperStatus_UP {
				configurePortL3AndOptical(t, dut, dp, trs[dp.Name()], ochs[dp.Name()], otnIndexes[dp.Name()], ethIndexes[dp.Name()], ipv4Addrs[dp.Name()], ipv6Addrs[dp.Name()])
				gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), linkUpTimeout, oc.Interface_OperStatus_UP)
			}
		}
	})

	fptest.ConfigureDefaultNetworkInstance(t, dut)

	// Step 1: Configuration
	for _, dp := range ports {
		configurePortL3AndOptical(t, dut, dp, trs[dp.Name()], ochs[dp.Name()], otnIndexes[dp.Name()], ethIndexes[dp.Name()], ipv4Addrs[dp.Name()], ipv6Addrs[dp.Name()])
	}

	// Step 2: Baseline Operational State and PM Interval Telemetry
	for _, dp := range ports {
		await400ZROpticalLinkUp(t, dut, dp.Name(), ochs[dp.Name()], linkUpTimeout)
		if dp.PMD() == ondatra.PMD400GBASEZR || dp.PMD() == ondatra.PMD400GBASEZRP {
			cfgplugins.ValidateInterfaceConfig(t, dut, dp, targetOutputPowerdBm, targetFrequencyMHz, targetOutputPowerTolerancedBm, targetFrequencyToleranceMHz)
		}
		if !deviations.OperationalModeUnsupported(dut) {
			if gotOpMode, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(ochs[dp.Name()]).OpticalChannel().OperationalMode().State()).Val(); ok && gotOpMode != operationalMode {
				t.Errorf("Port %s optical-channel operational-mode: got %d, want %d", dp.Name(), gotOpMode, operationalMode)
			}
		}
	}

	fecStreams := make(map[string]*portFECStreams, len(ports))
	for _, dp := range ports {
		s := &portFECStreams{
			otnUncorrectable: samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndexes[dp.Name()]).Otn().FecUncorrectableBlocks().State(), samplingInterval),
		}
		if isTelemetryPathPresent(t, dut, gnmi.OC().TerminalDevice().Channel(ethIndexes[dp.Name()]).Ethernet().FecUncorrectableBlocks().State()) {
			s.ethUncorrectable = samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(ethIndexes[dp.Name()]).Ethernet().FecUncorrectableBlocks().State(), samplingInterval)
		}
		if isTelemetryPathPresent(t, dut, gnmi.OC().Component(ochs[dp.Name()]).OpticalChannel().FecUncorrectableBlocks().State()) {
			s.ochUncorrectable = samplestream.New(t, dut, gnmi.OC().Component(ochs[dp.Name()]).OpticalChannel().FecUncorrectableBlocks().State(), samplingInterval)
		}
		if isTelemetryPathPresent(t, dut, gnmi.OC().Interface(dp.Name()).Ethernet().Counters().FecUncorrectableBlocks().State()) {
			s.intfUncorrectable = samplestream.New(t, dut, gnmi.OC().Interface(dp.Name()).Ethernet().Counters().FecUncorrectableBlocks().State(), samplingInterval)
		}
		if isTelemetryPathPresent(t, dut, gnmi.OC().Interface(dp.Name()).Ethernet().Counters().FecCorrectedBlocks().State()) {
			s.intfCorrected = samplestream.New(t, dut, gnmi.OC().Interface(dp.Name()).Ethernet().Counters().FecCorrectedBlocks().State(), samplingInterval)
		}
		if dp.PMD() != ondatra.PMD400GBASEZR && dp.PMD() != ondatra.PMD400GBASEZRP {
			s.inFcsErrors = samplestream.New(t, dut, gnmi.OC().Interface(dp.Name()).Counters().InFcsErrors().State(), samplingInterval)
		}
		fecStreams[dp.Name()] = s
		t.Cleanup(s.closeAll)
	}

	baselineOTNUncorrectable := make(map[string]uint64, len(ports))
	for _, dp := range ports {
		stream := fecStreams[dp.Name()].primaryUncorrectableStream(t, dut, dp, otnIndexes[dp.Name()])
		if deviations.NonIntervalFecErrorCounter(dut) {
			initSample, ok := stream.AwaitNext(sampleTimeout, func(v *ygnmi.Value[uint64]) bool {
				_, present := v.Val()
				return present
			})
			if !ok || initSample == nil {
				t.Fatalf("Port %s: failed to read initial OTN fec-uncorrectable-blocks sample", dp.Name())
			}
			baselineOTNUncorrectable[dp.Name()], _ = initSample.Val()
		}
		pathDesc := dp.Name() + " /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks (Step 2 Baseline)"
		baselineOTNUncorrectable[dp.Name()] = validateFECSampleStream(t, dut, stream, pathDesc, 3, baselineOTNUncorrectable[dp.Name()])
		verifySecondaryFECStreams(t, dut, dp.Name(), fecStreams[dp.Name()])
	}

	// Step 3a: Transient Link Disturbance via Interface Flap
	toggleInterface(t, dut, dp1.Name(), false)
	gnmi.Await(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), linkUpTimeout, oc.Interface_OperStatus_DOWN)
	fecStreams[dp1.Name()].primaryUncorrectableStream(t, dut, dp1, otnIndexes[dp1.Name()]).AwaitNext(sampleTimeout, func(v *ygnmi.Value[uint64]) bool {
		_, present := v.Val()
		return present
	})
	if dut.Vendor() == ondatra.CISCO {
		configureL3Subinterface(t, dut, dp1.Name(), ipv4Addrs[dp1.Name()], ipv6Addrs[dp1.Name()])
	} else {
		configurePortL3AndOptical(t, dut, dp1, trs[dp1.Name()], ochs[dp1.Name()], otnIndexes[dp1.Name()], ethIndexes[dp1.Name()], ipv4Addrs[dp1.Name()], ipv6Addrs[dp1.Name()])
	}
	for _, dp := range ports {
		await400ZROpticalLinkUp(t, dut, dp.Name(), ochs[dp.Name()], linkUpTimeout)
	}
	for _, dp := range ports {
		stream := fecStreams[dp.Name()].primaryUncorrectableStream(t, dut, dp, otnIndexes[dp.Name()])
		if deviations.NonIntervalFecErrorCounter(dut) {
			if postFlapSample, ok := stream.AwaitNext(sampleTimeout, func(v *ygnmi.Value[uint64]) bool {
				_, present := v.Val()
				return present
			}); ok && postFlapSample != nil {
				baselineOTNUncorrectable[dp.Name()], _ = postFlapSample.Val()
			}
		}
		pathDesc := dp.Name() + " /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks (Step 3a Post-Interface-Flap)"
		baselineOTNUncorrectable[dp.Name()] = validateFECSampleStream(t, dut, stream, pathDesc, 2, baselineOTNUncorrectable[dp.Name()])
		verifySecondaryFECStreams(t, dut, dp.Name(), fecStreams[dp.Name()])
	}

	// Step 3b: Transient Link Disturbance via Transceiver Toggle
	if !deviations.TransceiverConfigEnableUnsupported(dut) {
		toggleTransceiver(t, dut, trs[dp1.Name()], false)
		gnmi.Await(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), linkUpTimeout, oc.Interface_OperStatus_DOWN)
		toggleTransceiver(t, dut, trs[dp1.Name()], true)
		for _, dp := range ports {
			await400ZROpticalLinkUp(t, dut, dp.Name(), ochs[dp.Name()], linkUpTimeout)
		}
		for _, dp := range ports {
			stream := fecStreams[dp.Name()].primaryUncorrectableStream(t, dut, dp, otnIndexes[dp.Name()])
			if deviations.NonIntervalFecErrorCounter(dut) {
				if postToggleSample, ok := stream.AwaitNext(sampleTimeout, func(v *ygnmi.Value[uint64]) bool {
					_, present := v.Val()
					return present
				}); ok && postToggleSample != nil {
					baselineOTNUncorrectable[dp.Name()], _ = postToggleSample.Val()
				}
			}
			pathDesc := dp.Name() + " /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks (Step 3b Post-Transceiver-Toggle)"
			baselineOTNUncorrectable[dp.Name()] = validateFECSampleStream(t, dut, stream, pathDesc, 2, baselineOTNUncorrectable[dp.Name()])
			verifySecondaryFECStreams(t, dut, dp.Name(), fecStreams[dp.Name()])
		}
	}
}

func verifyPLQCapabilities(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	plqClient := dut.RawAPIs().GNOI(t).LinkQualification()
	minBps, minMTU, minPPS := plqMinBps, plqPacketSize, plqPacketRate
	if devMTU := deviations.PLQGeneratorCapabilitiesMaxMTU(dut); devMTU != 0 && devMTU < minMTU {
		minBps = minBps * uint64(devMTU) / uint64(minMTU)
		minMTU = devMTU
	}
	if devPPS := deviations.PLQGeneratorCapabilitiesMaxPPS(dut); devPPS != 0 && devPPS < minPPS {
		minPPS = devPPS
	}

	resp, err := plqClient.Capabilities(context.Background(), &plqpb.CapabilitiesRequest{})
	if err != nil {
		t.Fatalf("LinkQualification.Capabilities() failed: %v", err)
	}
	genCap := resp.GetGenerator().GetPacketGenerator()
	if genCap == nil {
		t.Fatalf("DUT %s does not report PacketGenerator capabilities", dut.Name())
	}
	if got := genCap.GetMaxBps(); got < minBps {
		t.Errorf("PacketGenerator MaxBps: got %d, want >= %d", got, minBps)
	}
	if got := genCap.GetMaxMtu(); got < minMTU {
		t.Errorf("PacketGenerator MaxMtu: got %d, want >= %d", got, minMTU)
	}
	if got := genCap.GetMaxPps(); got < minPPS {
		t.Errorf("PacketGenerator MaxPps: got %d, want >= %d", got, minPPS)
	}
	refCap := resp.GetReflector()
	if refCap == nil || (refCap.GetAsicLoopback() == nil && refCap.GetPmdLoopback() == nil) {
		t.Fatalf("DUT %s does not report AsicLoopback or PmdLoopback reflector capabilities", dut.Name())
	}
}

func runAndVerifyPLQ(t *testing.T, dut *ondatra.DUTDevice, dp1, dp2 *ondatra.Port, ports []*ondatra.Port, fecStreams map[string]*portFECStreams, otnIndexes map[string]uint32, baselineOTN map[string]uint64) {
	t.Helper()
	plqClient := dut.RawAPIs().GNOI(t).LinkQualification()
	cleanupPLQ(t, dut, []string{nearEndQualID, farEndQualID})

	resp, err := plqClient.Capabilities(context.Background(), &plqpb.CapabilitiesRequest{})
	if err != nil {
		t.Fatalf("LinkQualification.Capabilities() failed: %v", err)
	}
	genCap := resp.GetGenerator().GetPacketGenerator()
	refCap := resp.GetReflector()

	reflectorCfg := &plqpb.QualificationConfiguration{
		Id:            farEndQualID,
		InterfaceName: dp2.Name(),
	}
	if refCap.GetPmdLoopback() != nil && (dut.Vendor() == ondatra.NOKIA || refCap.GetAsicLoopback() == nil) {
		reflectorCfg.EndpointType = &plqpb.QualificationConfiguration_PmdLoopback{PmdLoopback: &plqpb.PmdLoopbackConfiguration{}}
	} else {
		reflectorCfg.EndpointType = &plqpb.QualificationConfiguration_AsicLoopback{AsicLoopback: &plqpb.AsicLoopbackConfiguration{}}
	}

	packetSize := plqPacketSize
	if devMTU := deviations.PLQGeneratorCapabilitiesMaxMTU(dut); devMTU != 0 && devMTU < packetSize {
		packetSize = devMTU
	}
	if maxMTU := genCap.GetMaxMtu(); maxMTU > 0 && maxMTU < packetSize {
		packetSize = maxMTU
	}
	packetRate := plqPacketRate
	if devPPS := deviations.PLQGeneratorCapabilitiesMaxPPS(dut); devPPS != 0 && devPPS < packetRate {
		packetRate = devPPS
	}

	genSetup, genTeardown := plqSetupDuration, plqTeardown
	if minSetup := genCap.GetMinSetupDuration().AsDuration(); minSetup > genSetup {
		genSetup = minSetup
	}
	if minTd := genCap.GetMinTeardownDuration().AsDuration(); minTd > genTeardown {
		genTeardown = minTd
	}
	refSetup, refTeardown := plqSetupDuration, plqTeardown
	if lb := refCap.GetAsicLoopback(); lb != nil {
		if minSetup := lb.GetMinSetupDuration().AsDuration(); minSetup > refSetup {
			refSetup = minSetup
		}
		if minTd := lb.GetMinTeardownDuration().AsDuration(); minTd > refTeardown {
			refTeardown = minTd
		}
	}
	if genSetup != refSetup {
		if genSetup > refSetup {
			refSetup = genSetup
		} else {
			genSetup = refSetup
		}
	}
	if genTeardown != refTeardown {
		if genTeardown > refTeardown {
			refTeardown = genTeardown
		} else {
			genTeardown = refTeardown
		}
	}

	reflectorCfg.Timing = &plqpb.QualificationConfiguration_Rpc{
		Rpc: &plqpb.RPCSyncedTiming{
			PreSyncDuration:  durationpb.New(0),
			SetupDuration:    durationpb.New(refSetup),
			Duration:         durationpb.New(plqDuration),
			PostSyncDuration: durationpb.New(plqPostSync),
			TeardownDuration: durationpb.New(refTeardown),
		},
	}

	createResp, err := plqClient.Create(context.Background(), &plqpb.CreateRequest{
		Interfaces: []*plqpb.QualificationConfiguration{
			{
				Id:            nearEndQualID,
				InterfaceName: dp1.Name(),
				Timing: &plqpb.QualificationConfiguration_Rpc{
					Rpc: &plqpb.RPCSyncedTiming{
						PreSyncDuration:  durationpb.New(plqPreSync),
						SetupDuration:    durationpb.New(genSetup),
						Duration:         durationpb.New(plqDuration),
						PostSyncDuration: durationpb.New(0),
						TeardownDuration: durationpb.New(genTeardown),
					},
				},
				EndpointType: &plqpb.QualificationConfiguration_PacketGenerator{
					PacketGenerator: &plqpb.PacketGeneratorConfiguration{
						PacketRate: packetRate,
						PacketSize: packetSize,
					},
				},
			},
			reflectorCfg,
		},
	})
	if err != nil {
		t.Fatalf("LinkQualification.Create() failed: %v", err)
	}
	for id, st := range createResp.GetStatus() {
		if st.GetCode() != 0 {
			t.Fatalf("LinkQualification.Create() returned non-OK status for %s: %v", id, st)
		}
	}
	if !deviations.SkipPlqInterfaceOperStatusCheck(dut) {
		for _, p := range []*ondatra.Port{dp1, dp2} {
			gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_TESTING)
		}
	}

	// Verify FEC uncorrectable blocks remain 0 during the active qualification window.
	for _, dp := range ports {
		gotOTN := fecStreams[dp.Name()].getPrimaryUncorrectableBlocks(t, dut, dp, otnIndexes[dp.Name()])
		wantOTN := uint64(0)
		if deviations.NonIntervalFecErrorCounter(dut) {
			wantOTN = baselineOTN[dp.Name()]
		}
		if gotOTN != wantOTN {
			t.Errorf("Port %s during-PLQ gNMI.Get fec-uncorrectable-blocks: got %d, want %d", dp.Name(), gotOTN, wantOTN)
		}
		pathDesc := dp.Name() + " /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks (Step 2 During PLQ)"
		stream := fecStreams[dp.Name()].primaryUncorrectableStream(t, dut, dp, otnIndexes[dp.Name()])
		baselineOTN[dp.Name()] = validateFECSampleStream(t, dut, stream, pathDesc, 1, baselineOTN[dp.Name()])
	}

	deadline := time.Now().Add(5 * time.Minute)
	for {
		listResp, err := plqClient.List(context.Background(), &plqpb.ListRequest{})
		if err != nil {
			t.Fatalf("LinkQualification.List() failed: %v", err)
		}
		allCompleted := len(listResp.GetResults()) >= 2
		for _, res := range listResp.GetResults() {
			if res.GetState() != plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED {
				allCompleted = false
			}
			if res.GetState() == plqpb.QualificationState_QUALIFICATION_STATE_ERROR {
				errGetResp, _ := plqClient.Get(context.Background(), &plqpb.GetRequest{Ids: []string{nearEndQualID, farEndQualID}})
				t.Fatalf("PLQ session %s on %s entered QUALIFICATION_STATE_ERROR (details: %v)", res.GetId(), res.GetInterfaceName(), errGetResp)
			}
		}
		if allCompleted {
			break
		}
		if time.Until(deadline) <= 0 {
			t.Fatalf("PLQ sessions (%s, %s) did not reach QUALIFICATION_STATE_COMPLETED within 5m", nearEndQualID, farEndQualID)
		}
		time.Sleep(5 * time.Second)
	}

	getResp, err := plqClient.Get(context.Background(), &plqpb.GetRequest{Ids: []string{nearEndQualID, farEndQualID}})
	if err != nil {
		t.Fatalf("LinkQualification.Get() failed: %v", err)
	}
	for _, id := range []string{nearEndQualID, farEndQualID} {
		res, ok := getResp.GetResults()[id]
		if !ok {
			t.Errorf("LinkQualification.Get() missing result for ID %q", id)
			continue
		}
		if res.GetStatus().GetCode() != 0 {
			t.Errorf("Result[%s] status code: got %d, want 0", id, res.GetStatus().GetCode())
		}
		if res.GetState() != plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED {
			t.Errorf("Result[%s] state: got %v, want QUALIFICATION_STATE_COMPLETED", id, res.GetState())
		}
		if res.GetPacketsError() != 0 {
			t.Errorf("Result[%s] packets_error: got %d, want 0", id, res.GetPacketsError())
		}
		if res.GetPacketsDropped() != 0 {
			t.Errorf("Result[%s] packets_dropped: got %d, want 0", id, res.GetPacketsDropped())
		}
		if id == farEndQualID && deviations.PLQReflectorStatsUnsupported(dut) {
			continue
		}
		sent, rxed := res.GetPacketsSent(), res.GetPacketsReceived()
		if sent == 0 || rxed == 0 {
			t.Errorf("Result[%s] packets sent/received must be > 0: sent=%d, received=%d", id, sent, rxed)
		}
		const tolerancePct = 0.0001
		if sent > 0 && (math.Abs(float64(sent)-float64(rxed))/float64(sent))*100.0 > tolerancePct {
			if diff := cmp.Diff(sent, rxed); diff != "" {
				t.Errorf("Result[%s] packets_sent != packets_received (-sent +received):\n%s", id, diff)
			}
		}
	}
	cleanupPLQ(t, dut, []string{nearEndQualID, farEndQualID})
}

func cleanupPLQ(t *testing.T, dut *ondatra.DUTDevice, ids []string) {
	t.Helper()
	plqClient := dut.RawAPIs().GNOI(t).LinkQualification()
	listResp, err := plqClient.List(context.Background(), &plqpb.ListRequest{})
	if err != nil || len(listResp.GetResults()) == 0 {
		return
	}
	if _, err := plqClient.Delete(context.Background(), &plqpb.DeleteRequest{Ids: ids}); err != nil {
		t.Logf("LinkQualification.Delete(%v) returned error: %v", ids, err)
	}
	for i := 0; i < 6; i++ {
		if lr, err := plqClient.List(context.Background(), &plqpb.ListRequest{}); err == nil && len(lr.GetResults()) == 0 {
			return
		}
		time.Sleep(5 * time.Second)
	}
}

// TestZrLinkQualificationAndDualStackTraffic validates TRANSCEIVER-10.1.2: Validate 400ZR Link
// Qualification and Dual-Stack Traffic.
func TestZrLinkQualificationAndDualStackTraffic(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	ports := []*ondatra.Port{dp1, dp2}

	otnIndexes := map[string]uint32{dp1.Name(): otnIndex1, dp2.Name(): otnIndex2}
	ethIndexes := map[string]uint32{dp1.Name(): ethIndex1, dp2.Name(): ethIndex2}
	ipv4Addrs := map[string]string{dp1.Name(): port1IPv4, dp2.Name(): port2IPv4}
	ipv6Addrs := map[string]string{dp1.Name(): port1IPv6, dp2.Name(): port2IPv6}
	if dut.Vendor() == ondatra.ARISTA || dut.Vendor() == ondatra.JUNIPER || dut.Vendor() == ondatra.CISCO {
		ipv4Addrs[dp2.Name()] = "192.0.2.5"
		ipv6Addrs[dp2.Name()] = "2001:db8::5"
	}

	trs := make(map[string]string, len(ports))
	ochs := make(map[string]string, len(ports))
	for _, dp := range ports {
		trs[dp.Name()] = gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).Transceiver().State())
		ochs[dp.Name()] = components.OpticalChannelComponentFromPort(t, dut, dp)
	}

	operationalMode = uint16(*operationalModeFlag)
	operationalMode = cfgplugins.InterfaceInitialize(t, dut, operationalMode)

	t.Cleanup(func() {
		cleanupPLQ(t, dut, []string{nearEndQualID, farEndQualID})
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
			if st, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State()).Val(); !ok || st != oc.Interface_OperStatus_UP {
				configurePortL3AndOptical(t, dut, dp, trs[dp.Name()], ochs[dp.Name()], otnIndexes[dp.Name()], ethIndexes[dp.Name()], ipv4Addrs[dp.Name()], ipv6Addrs[dp.Name()])
				gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), linkUpTimeout, oc.Interface_OperStatus_UP)
			}
		}
	})

	fptest.ConfigureDefaultNetworkInstance(t, dut)

	// Step 1: Independent Link Configuration & Pre-Qualification Capabilities Check
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

	for _, dp := range ports {
		await400ZROpticalLinkUp(t, dut, dp.Name(), ochs[dp.Name()], linkUpTimeout)
	}

	fecStreams := make(map[string]*portFECStreams, len(ports))
	for _, dp := range ports {
		s := &portFECStreams{
			otnUncorrectable: samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndexes[dp.Name()]).Otn().FecUncorrectableBlocks().State(), samplingInterval),
		}
		if dp.PMD() != ondatra.PMD400GBASEZR && dp.PMD() != ondatra.PMD400GBASEZRP {
			s.inFcsErrors = samplestream.New(t, dut, gnmi.OC().Interface(dp.Name()).Counters().InFcsErrors().State(), samplingInterval)
		}
		fecStreams[dp.Name()] = s
		t.Cleanup(s.closeAll)
	}

	baselineOTNUncorrectable := make(map[string]uint64, len(ports))
	for _, dp := range ports {
		if deviations.NonIntervalFecErrorCounter(dut) {
			stream := fecStreams[dp.Name()].primaryUncorrectableStream(t, dut, dp, otnIndexes[dp.Name()])
			initSample, ok := stream.AwaitNext(sampleTimeout, func(v *ygnmi.Value[uint64]) bool {
				_, present := v.Val()
				return present
			})
			if !ok || initSample == nil {
				t.Fatalf("Port %s: failed to read initial OTN fec-uncorrectable-blocks sample", dp.Name())
			}
			baselineOTNUncorrectable[dp.Name()], _ = initSample.Val()
		}
	}

	verifyPLQCapabilities(t, dut)

	// Step 2: Packet Link Qualification Execution
	runAndVerifyPLQ(t, dut, dp1, dp2, ports, fecStreams, otnIndexes, baselineOTNUncorrectable)

	for _, dp := range ports {
		gotOTN := fecStreams[dp.Name()].getPrimaryUncorrectableBlocks(t, dut, dp, otnIndexes[dp.Name()])
		wantOTN := uint64(0)
		if deviations.NonIntervalFecErrorCounter(dut) {
			wantOTN = baselineOTNUncorrectable[dp.Name()]
		}
		if gotOTN != wantOTN {
			t.Errorf("Port %s post-PLQ gNMI.Get fec-uncorrectable-blocks: got %d, want %d", dp.Name(), gotOTN, wantOTN)
		}
		pathDesc := dp.Name() + " /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks (Step 2 Post-PLQ)"
		stream := fecStreams[dp.Name()].primaryUncorrectableStream(t, dut, dp, otnIndexes[dp.Name()])
		baselineOTNUncorrectable[dp.Name()] = validateFECSampleStream(t, dut, stream, pathDesc, 1, baselineOTNUncorrectable[dp.Name()])
	}

	// Step 3: Post-Qualification Dual-Stack Traffic and Counter Verification
	for _, dp := range ports {
		gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), linkUpTimeout, oc.Interface_OperStatus_UP)

		var baselineInFcs uint64
		if !deviations.InterfaceCountersInFcsErrorsUnsupported(dut) {
			baselineInFcs, _ = gnmi.Lookup(t, dut, gnmi.OC().Interface(dp.Name()).Counters().InFcsErrors().State()).Val()
		}

		v4AddrPath := gnmi.OC().Interface(dp.Name()).Subinterface(0).Ipv4().Address(ipv4Addrs[dp.Name()])
		if _, ok := gnmi.Watch(t, dut, v4AddrPath.Ip().State(), 15*time.Second, func(v *ygnmi.Value[string]) bool {
			ip, present := v.Val()
			return present && ip == ipv4Addrs[dp.Name()]
		}).Await(t); !ok {
			configureL3Subinterface(t, dut, dp.Name(), ipv4Addrs[dp.Name()], ipv6Addrs[dp.Name()])
			if got, ok := gnmi.Lookup(t, dut, v4AddrPath.Ip().Config()).Val(); !ok || got != ipv4Addrs[dp.Name()] {
				t.Errorf("Port %s IPv4 address config/state mismatch: got (%q, %v), want %q", dp.Name(), got, ok, ipv4Addrs[dp.Name()])
			}
		}

		v6AddrPath := gnmi.OC().Interface(dp.Name()).Subinterface(0).Ipv6().Address(ipv6Addrs[dp.Name()])
		if _, ok := gnmi.Watch(t, dut, v6AddrPath.Ip().State(), 15*time.Second, func(v *ygnmi.Value[string]) bool {
			ip, present := v.Val()
			return present && ip == ipv6Addrs[dp.Name()]
		}).Await(t); !ok {
			configureL3Subinterface(t, dut, dp.Name(), ipv4Addrs[dp.Name()], ipv6Addrs[dp.Name()])
			if got, ok := gnmi.Lookup(t, dut, v6AddrPath.Ip().Config()).Val(); !ok || got != ipv6Addrs[dp.Name()] {
				t.Errorf("Port %s IPv6 address config/state mismatch: got (%q, %v), want %q", dp.Name(), got, ok, ipv6Addrs[dp.Name()])
			}
		}

		countersPath := gnmi.OC().Interface(dp.Name()).Counters()
		if pkts, ok := gnmi.Lookup(t, dut, countersPath.OutPkts().State()).Val(); !ok || pkts == 0 {
			if val, ok := gnmi.Watch(t, dut, countersPath.OutPkts().State(), 30*time.Second, func(v *ygnmi.Value[uint64]) bool {
				pkts, present := v.Val()
				return present && pkts > 0
			}).Await(t); !ok {
				t.Errorf("Port %s out-pkts did not exceed 0 (last: %v)", dp.Name(), val)
			}
		}
		if pkts, ok := gnmi.Lookup(t, dut, countersPath.InPkts().State()).Val(); !ok || pkts == 0 {
			if val, ok := gnmi.Watch(t, dut, countersPath.InPkts().State(), 30*time.Second, func(v *ygnmi.Value[uint64]) bool {
				pkts, present := v.Val()
				return present && pkts > 0
			}).Await(t); !ok {
				t.Errorf("Port %s in-pkts did not exceed 0 (last: %v)", dp.Name(), val)
			}
		}

		if !deviations.InterfaceCountersInFcsErrorsUnsupported(dut) {
			if got, ok := gnmi.Lookup(t, dut, countersPath.InFcsErrors().State()).Val(); ok && got > baselineInFcs {
				t.Errorf("Port %s in-fcs-errors increased post-qualification: baseline=%d, current=%d", dp.Name(), baselineInFcs, got)
			}
		}

		gotOTN := fecStreams[dp.Name()].getPrimaryUncorrectableBlocks(t, dut, dp, otnIndexes[dp.Name()])
		wantOTN := uint64(0)
		if deviations.NonIntervalFecErrorCounter(dut) {
			wantOTN = baselineOTNUncorrectable[dp.Name()]
		}
		if gotOTN != wantOTN {
			t.Errorf("Port %s post-traffic gNMI.Get fec-uncorrectable-blocks: got %d, want %d", dp.Name(), gotOTN, wantOTN)
		}

		pathDesc := dp.Name() + " /terminal-device/logical-channels/channel/otn/state/fec-uncorrectable-blocks (Step 3 Post-Traffic)"
		stream := fecStreams[dp.Name()].primaryUncorrectableStream(t, dut, dp, otnIndexes[dp.Name()])
		baselineOTNUncorrectable[dp.Name()] = validateFECSampleStream(t, dut, stream, pathDesc, 1, baselineOTNUncorrectable[dp.Name()])

		t.Logf("Port %s counters: out-pkts=%d, in-pkts=%d", dp.Name(), gnmi.Get(t, dut, countersPath.OutPkts().State()), gnmi.Get(t, dut, countersPath.InPkts().State()))
	}
}
