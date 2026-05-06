package zr_input_output_power_test

import (
	"flag"
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
	samplingInterval           = 10 * time.Second
	inactiveOCHRxPower         = 0.0
	inactiveOCHTxPower         = 0.0
	inactiveTransceiverRxPower = 0.0
	rxPowerReadingError        = 3
	txPowerReadingError        = 3
	timeout                    = 10 * time.Minute
)

var (
	frequencies         = []uint64{191400000, 196100000}
	targetOpticalPowers = []float64{-9, -13}
	operationalModeFlag = flag.Int("operational_mode", 0, "vendor-specific operational-mode for the channel.")
	operationalMode     uint16
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestOpticalPower(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	operationalMode = uint16(*operationalModeFlag)
	operationalMode = cfgplugins.InterfaceInitialize(t, dut, operationalMode)

	// Configure interfaces for all ports
	var (
		trs  = make(map[string]string)
		ochs = make(map[string]string)
	)
	for _, p := range dut.Ports() {
		// Check the port PMD is 400ZR.
		if p.PMD() != ondatra.PMD400GBASEZR {
			t.Fatalf("%s PMD is %v, not 400ZR", p.Name(), p.PMD())
		}
		cfgplugins.InterfaceConfig(t, dut, p)
		// Get transceiver and optical channel.
		trs[p.Name()] = gnmi.Get(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State())
		ochs[p.Name()] = gnmi.Get(t, dut, gnmi.OC().Component(trs[p.Name()]).Transceiver().Channel(0).AssociatedOpticalChannel().State())
	}

	for _, frequency := range frequencies {
		for _, targetOpticalPower := range targetOpticalPowers {
			// Configure OCH component and OTN and ETH logical channels.
			for _, p := range dut.Ports() {
				cfgplugins.ConfigOpticalChannel(t, dut, ochs[p.Name()], frequency, targetOpticalPower, operationalMode)
			}

			t.Logf(" Frequency: %v, targetOpticalPower: %v", frequency, targetOpticalPower)

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

			// Test interface lifecycle
			testInterfaceState := func(enabled bool, expectedStatus oc.E_Interface_OperStatus) {
				togglePortsState(t, dut, enabled)
				awaitPortsOperStatus(t, dut, expectedStatus)
				time.Sleep(8 * samplingInterval)
				validateAllSampleStreams(t, dut, enabled, interfaceStreams, ochStreams, trStreams, targetOpticalPower)
			}

			testInterfaceState(true, oc.Interface_OperStatus_UP)    // Enable
			testInterfaceState(false, oc.Interface_OperStatus_DOWN) // Disable
			testInterfaceState(true, oc.Interface_OperStatus_UP)    // Re-enable
		}
	}
}

// togglePortsState enables or disables all ports.
func togglePortsState(t *testing.T, dut *ondatra.DUTDevice, enabled bool) {
	for _, p := range dut.Ports() {
		cfgplugins.ToggleInterface(t, dut, p.Name(), enabled)
	}
}

// awaitPortsOperStatus waits for all ports to reach the target operational status.
func awaitPortsOperStatus(t *testing.T, dut *ondatra.DUTDevice, expectedStatus oc.E_Interface_OperStatus) {
	for _, p := range dut.Ports() {
		gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, expectedStatus)
	}
}

// validateAllSampleStreams validates all the sample streams.
func validateAllSampleStreams(t *testing.T, dut *ondatra.DUTDevice, isEnabled bool, interfaceStreams map[string]*samplestream.SampleStream[*oc.Interface], ochStreams map[string]*samplestream.SampleStream[*oc.Component_OpticalChannel], transceiverStreams map[string]*samplestream.SampleStream[*oc.Component_Transceiver_Channel], targetOpticalPower float64) {
	expectedStatus := oc.Interface_OperStatus_UP
	if !isEnabled {
		expectedStatus = oc.Interface_OperStatus_DOWN
	}
	for _, p := range dut.Ports() {
		for valIndex := range interfaceStreams[p.Name()].All() {
			if valIndex >= len(ochStreams[p.Name()].All()) || valIndex >= len(transceiverStreams[p.Name()].All()) {
				break
			}
			operStatus := validateSampleStream(t, interfaceStreams[p.Name()].All()[valIndex], ochStreams[p.Name()].All()[valIndex], transceiverStreams[p.Name()].All()[valIndex], p.Name(), targetOpticalPower)
			if operStatus != expectedStatus {
				t.Errorf("Invalid %v operStatus value: want %v, got %v", p.Name(), expectedStatus, operStatus)
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

	// Helper to validate power data with nil check
	validatePowerIfExists := func(power interface {
		GetInstant() float64
		GetMin() float64
		GetMax() float64
		GetAvg() float64
	}, label string, minAllowed, maxAllowed, inactiveValue float64) {
		if power == nil {
			t.Errorf("%s data is empty for port %v", label, portName)
			return
		}
		validatePowerValue(t, portName, label, power.GetInstant(), power.GetMin(), power.GetMax(), power.GetAvg(), minAllowed, maxAllowed, inactiveValue, operStatus)
	}

	validatePowerIfExists(ochValue.GetInputPower(), "OpticalChannelInputPower", targetOpticalPower-rxPowerReadingError, targetOpticalPower+rxPowerReadingError, inactiveOCHRxPower)
	validatePowerIfExists(ochValue.GetOutputPower(), "OpticalChannelOutputPower", targetOpticalPower-txPowerReadingError, targetOpticalPower+txPowerReadingError, inactiveOCHTxPower)

	transceiverValue, ok := transceiverData.Val()
	if !ok {
		t.Errorf("Transceiver data is empty for port %v.", portName)
		return oc.Interface_OperStatus_UNSET
	}

	validatePowerIfExists(transceiverValue.GetInputPower(), "TransceiverInputPower", targetOpticalPower-rxPowerReadingError, targetOpticalPower+rxPowerReadingError, inactiveTransceiverRxPower)
	return operStatus
}

// validatePowerValue validates the power value.
func validatePowerValue(t *testing.T, portName, pm string, instant, min, max, avg, minAllowed, maxAllowed, inactiveValue float64, operStatus oc.E_Interface_OperStatus) {
	isValid := false
	switch operStatus {
	case oc.Interface_OperStatus_UP:
		isValid = instant >= minAllowed && instant <= maxAllowed
	case oc.Interface_OperStatus_DOWN:
		isValid = instant <= inactiveValue
	}

	if !isValid {
		t.Errorf("Invalid %v sample when %v is %v --> min : %v, max : %v, avg : %v, instant : %v", pm, portName, operStatus, min, max, avg, instant)
	} else {
		t.Logf("Valid %v sample when %v is %v --> min : %v, max : %v, avg : %v, instant : %v", pm, portName, operStatus, min, max, avg, instant)
	}
}
