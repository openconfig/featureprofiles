package zr_input_output_power_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	dp16QAM                    = uint16(1)
	samplingInterval           = 10 * time.Second
	inactiveOCHRxPower         = -30.0
	inactiveOCHTxPower         = -30.0
	inactiveTransceiverRxPower = -20.0
	rxPowerReadingError        = 2
	txPowerReadingError        = 0.5
	timeout                    = 10 * time.Minute
)

var (
	frequencies         = []uint64{191400000, 196100000}
	targetOpticalPowers = []float64{-9, -13}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestOpticalPower(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	fptest.ConfigureDefaultNetworkInstance(t, dut)

	var (
		trs  = make(map[string]string)
		ochs = make(map[string]string)
	)

	for _, p := range dut.Ports() {
		// Check the port PMD is 400ZR.
		if p.PMD() != ondatra.PMD400GBASEZR {
			t.Fatalf("%s PMD is %v, not 400ZR", p.Name(), p.PMD())
		}

		// Get transceiver and optical channel.
		trs[p.Name()] = gnmi.Get(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State())
		ochs[p.Name()] = gnmi.Get(t, dut, gnmi.OC().Component(trs[p.Name()]).Transceiver().Channel(0).AssociatedOpticalChannel().State())
	}

	for _, frequency := range frequencies {
		for _, targetOpticalPower := range targetOpticalPowers {
			// Configure OCH component and OTN and ETH logical channels.
			for _, p := range dut.Ports() {
				cfgplugins.ConfigOpticalChannel(t, dut, ochs[p.Name()], frequency, targetOpticalPower, dp16QAM)
			}

			// Create sample steams for each port.
			ochStreams := make(map[string]*samplestream.SampleStream[*oc.Component_OpticalChannel])
			trStreams := make(map[string]*samplestream.SampleStream[*oc.Component_Transceiver_Channel])
			interfaceStreams := make(map[string]*samplestream.SampleStream[*oc.Interface])
			for portName, och := range ochs {
				ochStreams[portName] = samplestream.New(t, dut, gnmi.OC().Component(och).OpticalChannel().State(), samplingInterval)
				trStreams[portName] = samplestream.New(t, dut, gnmi.OC().Component(trs[portName]).Transceiver().Channel(0).State(), samplingInterval)
				interfaceStreams[portName] = samplestream.New(t, dut, gnmi.OC().Interface(portName).State(), samplingInterval)
				defer ochStreams[portName].Close()
				defer trStreams[portName].Close()
				defer interfaceStreams[portName].Close()
			}

			// Enable interface.
			for _, p := range dut.Ports() {
				cfgplugins.ToggleInterface(t, dut, p.Name(), true)
			}

			// Wait for streaming telemetry to report the channels as up.
			for _, p := range dut.Ports() {
				gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
			}

			time.Sleep(3 * samplingInterval) // Wait an extra sample interval to ensure the device has time to process the change.

			validateAllSampleStreams(t, dut, true, interfaceStreams, ochStreams, trStreams, targetOpticalPower)

			// Disable interface.
			for _, p := range dut.Ports() {
				cfgplugins.ToggleInterface(t, dut, p.Name(), false)
			}

			// Wait for streaming telemetry to report the channels as down.
			for _, p := range dut.Ports() {
				gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_DOWN)
			}
			time.Sleep(3 * samplingInterval) // Wait an extra sample interval to ensure the device has time to process the change.

			validateAllSampleStreams(t, dut, false, interfaceStreams, ochStreams, trStreams, targetOpticalPower)

			// Re-enable transceivers.
			for _, p := range dut.Ports() {
				cfgplugins.ToggleInterface(t, dut, p.Name(), true)
			}

			// Wait for streaming telemetry to report the channels as up.
			for _, p := range dut.Ports() {
				gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
			}
			time.Sleep(3 * samplingInterval) // Wait an extra sample interval to ensure the device has time to process the change.

			validateAllSampleStreams(t, dut, true, interfaceStreams, ochStreams, trStreams, targetOpticalPower)
		}
	}
}

// validateAllSampleStreams validates all the sample streams.
func validateAllSampleStreams(t *testing.T, dut *ondatra.DUTDevice, isEnabled bool, interfaceStreams map[string]*samplestream.SampleStream[*oc.Interface], ochStreams map[string]*samplestream.SampleStream[*oc.Component_OpticalChannel], transceiverStreams map[string]*samplestream.SampleStream[*oc.Component_Transceiver_Channel], targetOpticalPower float64) {
	for _, p := range dut.Ports() {
		for valIndex := range interfaceStreams[p.Name()].All() {
			if valIndex >= len(ochStreams[p.Name()].All()) || valIndex >= len(transceiverStreams[p.Name()].All()) {
				break
			}
			operStatus := validateSampleStream(t, interfaceStreams[p.Name()].All()[valIndex], ochStreams[p.Name()].All()[valIndex], transceiverStreams[p.Name()].All()[valIndex], p.Name(), targetOpticalPower)
			switch operStatus {
			case oc.Interface_OperStatus_UP:
				if !isEnabled {
					t.Errorf("Invalid %v operStatus value: want DOWN, got %v", p.Name(), operStatus)
				}
			case oc.Interface_OperStatus_DOWN:
				if isEnabled {
					t.Errorf("Invalid %v operStatus value: want UP, got %v", p.Name(), operStatus)
				}
			}
		}
	}
}

// validateSampleStream validates the stream data.
func validateSampleStream(t *testing.T, interfaceData *ygnmi.Value[*oc.Interface], ochData *ygnmi.Value[*oc.Component_OpticalChannel], transceiverData *ygnmi.Value[*oc.Component_Transceiver_Channel], portName string, targetOpticalPower float64) oc.E_Interface_OperStatus {
	if interfaceData == nil {
		t.Errorf("Data not received for port %v.", portName)
		return oc.Interface_OperStatus_UNSET
	}
	interfaceValue, ok := interfaceData.Val()
	if !ok {
		t.Errorf("Channel data is empty for port %v.", portName)
		return oc.Interface_OperStatus_UNSET
	}
	operStatus := interfaceValue.GetOperStatus()
	if operStatus == oc.Interface_OperStatus_UNSET {
		t.Errorf("Link state data is empty for port %v", portName)
		return oc.Interface_OperStatus_UNSET
	}
	ochValue, ok := ochData.Val()
	if !ok {
		t.Errorf("Terminal Device data is empty for port %v.", portName)
		return oc.Interface_OperStatus_UNSET
	}
	if inPow := ochValue.GetInputPower(); inPow == nil {
		t.Errorf("InputPower data is empty for port %v", portName)
	} else {
		validatePowerValue(t, portName, "OpticalChannelInputPower", inPow.GetInstant(), inPow.GetMin(), inPow.GetMax(), inPow.GetAvg(), targetOpticalPower-rxPowerReadingError, targetOpticalPower+rxPowerReadingError, inactiveOCHRxPower, operStatus)
	}
	if outPow := ochValue.GetOutputPower(); outPow == nil {
		t.Errorf("OutputPower data is empty for port %v", portName)
	} else {
		validatePowerValue(t, portName, "OpticalChannelOutputPower", outPow.GetInstant(), outPow.GetMin(), outPow.GetMax(), outPow.GetAvg(), targetOpticalPower-txPowerReadingError, targetOpticalPower+txPowerReadingError, inactiveOCHTxPower, operStatus)
	}
	transceiverValue, ok := transceiverData.Val()
	if !ok {
		t.Errorf("Transceiver data is empty for port %v.", portName)
		return oc.Interface_OperStatus_UNSET
	}
	if inPow := transceiverValue.GetInputPower(); inPow == nil {
		t.Errorf("InputPower data is empty for port %v", portName)
	} else {
		validatePowerValue(t, portName, "TransceiverInputPower", inPow.GetInstant(), inPow.GetMin(), inPow.GetMax(), inPow.GetAvg(), targetOpticalPower-rxPowerReadingError, targetOpticalPower+rxPowerReadingError, inactiveTransceiverRxPower, operStatus)
	}
	return operStatus
}

// validatePowerValue validates the power value.
func validatePowerValue(t *testing.T, portName, pm string, instant, min, max, avg, minAllowed, maxAllowed, inactiveValue float64, operStatus oc.E_Interface_OperStatus) {
	switch operStatus {
	case oc.Interface_OperStatus_UP:
		if instant < minAllowed || instant > maxAllowed {
			t.Errorf("Invalid %v sample when %v is UP --> min : %v, max : %v, avg : %v, instant : %v", pm, portName, min, max, avg, instant)
			return
		}
	case oc.Interface_OperStatus_DOWN:
		if instant > inactiveValue {
			t.Errorf("Invalid %v sample when %v is DOWN --> min : %v, max : %v, avg : %v, instant : %v", pm, portName, min, max, avg, instant)
			return
		}
	}
	t.Logf("Valid %v sample when %v is %v --> min : %v, max : %v, avg : %v, instant : %v", pm, portName, operStatus, min, max, avg, instant)
}
