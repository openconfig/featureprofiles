package zr_pm_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	dp16QAM             = uint16(16)
	samplingInterval    = 10 * time.Second
	minAllowedQValue    = 7.0
	maxAllowedQValue    = 14.0
	minAllowedPreFECBER = 1e-9
	maxAllowedPreFECBER = 1e-2
	minAllowedESNR      = 10.0
	maxAllowedESNR      = 25.0
	inactiveQValue      = 0.0
	inactivePreFECBER   = 0.0
	inactiveESNR        = 0.0
	timeout             = 10 * time.Minute
	flapInterval        = 30 * time.Second
	otnIndexBase        = uint32(4000)
	ethernetIndexBase   = uint32(40000)
)

var (
	frequencies         = []uint64{191400000, 196100000}
	targetOpticalPowers = []float64{-6, -10}

	dutPorts = []attrs.Attributes{
		{
			Desc:    "dutPort1",
			IPv4:    "192.0.2.1",
			IPv4Len: 30,
		},
		{
			Desc:    "dutPort2",
			IPv4:    "192.0.2.5",
			IPv4Len: 30,
		},
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestPM(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	fptest.ConfigureDefaultNetworkInstance(t, dut)

	var (
		trs        = make(map[string]string)
		ochs       = make(map[string]string)
		otnIndexes = make(map[string]uint32)
		ethIndexes = make(map[string]uint32)
	)

	for i, p := range dut.Ports() {
		// Check the port PMD is 400ZR.
		if p.PMD() != ondatra.PMD400GBASEZR {
			t.Fatalf("%s PMD is %v, not 400ZR", p.Name(), p.PMD())
		}

		// Configure interfaces.
		gnmi.Replace(t, dut, gnmi.OC().Interface(p.Name()).Config(), dutPorts[i].NewOCInterface(p.Name(), dut))

		// Get transceiver and optical channel.
		trs[p.Name()] = gnmi.Get(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State())
		ochs[p.Name()] = gnmi.Get(t, dut, gnmi.OC().Component(trs[p.Name()]).Transceiver().Channel(0).AssociatedOpticalChannel().State())

		// Assign OTN and ethernet indexes.
		otnIndexes[p.Name()] = otnIndexBase + uint32(i)
		ethIndexes[p.Name()] = ethernetIndexBase + uint32(i)
	}

	for _, frequency := range frequencies {
		for _, targetOpticalPower := range targetOpticalPowers {
			// Configure OCH component and OTN and ETH logical channels.
			for _, p := range dut.Ports() {
				configOpticalChannel(t, dut, ochs[p.Name()], frequency, targetOpticalPower)
				configOTNChannel(t, dut, ochs[p.Name()], otnIndexes[p.Name()])
				configETHChannel(t, dut, otnIndexes[p.Name()], ethIndexes[p.Name()])
			}

			// Create OTN channel sample steam for each port.
			otnStreams := make(map[string]*samplestream.SampleStream[*oc.TerminalDevice_Channel])
			for portName, otnIndex := range otnIndexes {
				otnStreams[portName] = samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndex).State(), samplingInterval)
				defer otnStreams[portName].Close()
			}

			// Enable transceivers.
			for _, tr := range trs {
				gnmi.Replace(t, dut, gnmi.OC().Component(tr).Transceiver().Enabled().Config(), true)
			}

			// Wait for streaming telemetry to report the channels as up.
			for _, otnIndex := range otnIndexes {
				gnmi.Await(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndex).LinkState().State(), timeout, oc.Channel_LinkState_UP)
			}
			time.Sleep(samplingInterval) // Wait an extra sample interval to ensure the device has time to process the change.

			// Disable transceivers.
			for _, tr := range trs {
				gnmi.Replace(t, dut, gnmi.OC().Component(tr).Transceiver().Enabled().Config(), false)
			}

			// Wait for streaming telemetry to report the channels as down.
			for _, otnIndex := range otnIndexes {
				gnmi.Await(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndex).LinkState().State(), timeout, oc.Channel_LinkState_DOWN)
			}
			time.Sleep(samplingInterval) // Wait an extra sample interval to ensure the device has time to process the change.

			// Re-enable transceivers.
			for _, tr := range trs {
				gnmi.Replace(t, dut, gnmi.OC().Component(tr).Transceiver().Enabled().Config(), true)
			}

			// Wait for streaming telemetry to report the channels as up.
			for _, otnIndex := range otnIndexes {
				gnmi.Await(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndex).LinkState().State(), timeout, oc.Channel_LinkState_UP)
			}
			time.Sleep(samplingInterval) // Wait an extra sample interval to ensure the device has time to process the change.

			// Now validate OTN streams didn't return any invalid values.
			for portName, stream := range otnStreams {
				var linkStates []oc.E_Channel_LinkState
				for _, val := range stream.All() {
					linkStates = append(linkStates, validateStream(t, val, portName))
				}
				validateLinkStateTransitions(t, linkStates, portName)
			}
		}
	}
}

// validateStream validates the stream data.
func validateStream(t *testing.T, data *ygnmi.Value[*oc.TerminalDevice_Channel], portName string) oc.E_Channel_LinkState {
	if data == nil {
		t.Errorf("Data not received for port %v.", portName)
		return oc.Channel_LinkState_UNSET
	}
	v, ok := data.Val()
	if !ok {
		t.Errorf("Channel data is empty for port %v.", portName)
		return oc.Channel_LinkState_UNSET
	}
	linkState := v.GetLinkState()
	if linkState == oc.Channel_LinkState_UNSET {
		t.Errorf("Link state data is empty for port %v", portName)
		return oc.Channel_LinkState_UNSET
	}
	otn := v.GetOtn()
	if otn == nil {
		t.Errorf("OTN data is empty for port %v", portName)
		return linkState
	}
	if b := otn.GetPreFecBer(); b == nil {
		t.Errorf("PreFECBER data is empty for port %v", portName)
	} else {
		validatePMValue(t, portName, "PreFECBER", b.GetMin(), b.GetMax(), b.GetAvg(), b.GetInstant(), minAllowedPreFECBER, maxAllowedPreFECBER, inactivePreFECBER, linkState)
	}
	if e := otn.GetEsnr(); e == nil {
		t.Errorf("ESNR data is empty for port %v", portName)
	} else {
		validatePMValue(t, portName, "esnr", e.GetMin(), e.GetMax(), e.GetAvg(), e.GetInstant(), minAllowedESNR, maxAllowedESNR, inactiveESNR, linkState)
	}
	if q := otn.GetQValue(); q == nil {
		t.Errorf("QValue data is empty for port %v", portName)
	} else {
		validatePMValue(t, portName, "QValue", q.GetMin(), q.GetMax(), q.GetAvg(), q.GetInstant(), minAllowedQValue, maxAllowedQValue, inactiveQValue, linkState)
	}
	return linkState
}

// validatePMValue validates the pm value.
func validatePMValue(t *testing.T, portName, pm string, instant, min, max, avg, minAllowed, maxAllowed, inactiveValue float64, linkState oc.E_Channel_LinkState) {
	switch linkState {
	case oc.Channel_LinkState_UP:
		if instant < min || instant > max || avg < min || avg > max || min < minAllowed || max > maxAllowed {
			t.Errorf("Invalid %v sample when %v is UP --> min : %v, max : %v, avg : %v, instant : %v", pm, portName, min, max, avg, instant)
			return
		}
	case oc.Channel_LinkState_DOWN:
		if instant != inactiveValue || avg != inactiveValue || min != inactiveValue || max != inactiveValue {
			t.Errorf("Invalid %v sample when %v is DOWN --> min : %v, max : %v, avg : %v, instant : %v", pm, portName, min, max, avg, instant)
			return
		}
	}
	t.Logf("Valid %v sample when %v is %v --> min : %v, max : %v, avg : %v, instant : %v", pm, portName, linkState, min, max, avg, instant)
}

// validateLinkStateTransitions validates the link state transitions.
func validateLinkStateTransitions(t *testing.T, linkStates []oc.E_Channel_LinkState, portName string) {
	if len(linkStates) < 3 {
		t.Errorf("Invalid %v link state transitions: want at least 3 samples, got %v", portName, len(linkStates))
		return
	}
	if linkStates[0] != oc.Channel_LinkState_DOWN {
		t.Errorf("Invalid %v link state transitions: want DOWN for initial link state, got %v ", portName, linkStates[0])
		return
	}
	var transitionIndexes []int
	for i := range linkStates {
		if i == 0 {
			continue
		}
		if linkStates[i-1] != linkStates[i] {
			transitionIndexes = append(transitionIndexes, i)
		}
	}
	if len(transitionIndexes) != 2 {
		t.Errorf("Invalid %v link state transitions: want 2 transitions, got %v ", portName, len(transitionIndexes))
		return
	}
	if linkStates[transitionIndexes[0]] != oc.Channel_LinkState_UP {
		t.Errorf("Invalid %v link state transitions: want DOWN-->UP, got %v-->%v", portName, linkStates[transitionIndexes[0]-1], linkStates[transitionIndexes[0]])
		return
	}
	if linkStates[transitionIndexes[1]] != oc.Channel_LinkState_DOWN {
		t.Errorf("Invalid %v link state transitions: want UP-->DOWN, got %v-->%v", portName, linkStates[transitionIndexes[1]-1], linkStates[transitionIndexes[1]])
	}
}

// configOpticalChannel configures the optical channel.
func configOpticalChannel(t *testing.T, dut *ondatra.DUTDevice, och string, frequency uint64, targetOpticalPower float64) {
	gnmi.Replace(t, dut, gnmi.OC().Component(och).OpticalChannel().Config(), &oc.Component_OpticalChannel{
		Frequency:         ygot.Uint64(frequency),
		TargetOutputPower: ygot.Float64(targetOpticalPower),
	})
}

// configOTNChannel configures the OTN channel.
func configOTNChannel(t *testing.T, dut *ondatra.DUTDevice, och string, otnIndex uint32) {
	gnmi.Replace(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndex).Config(), &oc.TerminalDevice_Channel{
		LogicalChannelType: oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_OTN,
		AdminState:         oc.TerminalDevice_AdminStateType_ENABLED,
		Description:        ygot.String("OTN Logical Channel"),
		Index:              ygot.Uint32(otnIndex),
		Assignment: map[uint32]*oc.TerminalDevice_Channel_Assignment{
			1: {
				Index:          ygot.Uint32(1),
				OpticalChannel: ygot.String(och),
				Description:    ygot.String("OTN to Optical"),
				Allocation:     ygot.Float64(400),
				AssignmentType: oc.Assignment_AssignmentType_OPTICAL_CHANNEL,
			},
		},
	})
}

// configETHChannel configures the ETH channel.
func configETHChannel(t *testing.T, dut *ondatra.DUTDevice, otnIndex, ethIndex uint32) {
	gnmi.Replace(t, dut, gnmi.OC().TerminalDevice().Channel(ethIndex).Config(), &oc.TerminalDevice_Channel{
		LogicalChannelType: oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_ETHERNET,
		AdminState:         oc.TerminalDevice_AdminStateType_ENABLED,
		Description:        ygot.String("ETH Logical Channel"),
		Index:              ygot.Uint32(ethIndex),
		RateClass:          oc.TransportTypes_TRIBUTARY_RATE_CLASS_TYPE_TRIB_RATE_400G,
		TribProtocol:       oc.TransportTypes_TRIBUTARY_PROTOCOL_TYPE_PROT_400GE,
		Assignment: map[uint32]*oc.TerminalDevice_Channel_Assignment{
			1: {
				Index:          ygot.Uint32(1),
				LogicalChannel: ygot.Uint32(otnIndex),
				Description:    ygot.String("ETH to OTN"),
				Allocation:     ygot.Float64(400),
				AssignmentType: oc.Assignment_AssignmentType_LOGICAL_CHANNEL,
			},
		},
	})
}
