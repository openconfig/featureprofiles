package zr_pm_test

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
	dp16QAM             = uint16(1)
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
	targetOpticalPowers = []float64{-9, -13}
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
				cfgplugins.ConfigOpticalChannel(t, dut, ochs[p.Name()], frequency, targetOpticalPower, dp16QAM)
				cfgplugins.ConfigOTNChannel(t, dut, ochs[p.Name()], otnIndexes[p.Name()], ethIndexes[p.Name()])
				cfgplugins.ConfigETHChannel(t, dut, p.Name(), trs[p.Name()], otnIndexes[p.Name()], ethIndexes[p.Name()])
			}

			// Create sample steams for each port.
			otnStreams := make(map[string]*samplestream.SampleStream[*oc.TerminalDevice_Channel])
			interfaceStreams := make(map[string]*samplestream.SampleStream[*oc.Interface])
			for portName, otnIndex := range otnIndexes {
				otnStreams[portName] = samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndex).State(), samplingInterval)
				interfaceStreams[portName] = samplestream.New(t, dut, gnmi.OC().Interface(portName).State(), samplingInterval)
				defer otnStreams[portName].Close()
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

			validateAllSamples(t, dut, true, interfaceStreams, otnStreams)

			// Disable interface.
			for _, p := range dut.Ports() {
				cfgplugins.ToggleInterface(t, dut, p.Name(), false)
			}

			// Wait for streaming telemetry to report the channels as down.
			for _, p := range dut.Ports() {
				gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_DOWN)
			}
			time.Sleep(3 * samplingInterval) // Wait an extra sample interval to ensure the device has time to process the change.

			validateAllSamples(t, dut, false, interfaceStreams, otnStreams)

			// Re-enable transceivers.
			for _, p := range dut.Ports() {
				cfgplugins.ToggleInterface(t, dut, p.Name(), true)
			}

			// Wait for streaming telemetry to report the channels as up.
			for _, p := range dut.Ports() {
				gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
			}
			time.Sleep(3 * samplingInterval) // Wait an extra sample interval to ensure the device has time to process the change.

			validateAllSamples(t, dut, true, interfaceStreams, otnStreams)
		}
	}
}

// validateAllSamples validates all the sample streams.
func validateAllSamples(t *testing.T, dut *ondatra.DUTDevice, isEnabled bool, interfaceStreams map[string]*samplestream.SampleStream[*oc.Interface], otnStreams map[string]*samplestream.SampleStream[*oc.TerminalDevice_Channel]) {
	for _, p := range dut.Ports() {
		for valIndex := range interfaceStreams[p.Name()].All() {
			if valIndex >= len(otnStreams[p.Name()].All()) {
				break
			}
			operStatus := validateSampleStream(t, interfaceStreams[p.Name()].All()[valIndex], otnStreams[p.Name()].All()[valIndex], p.Name())
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
func validateSampleStream(t *testing.T, interfaceData *ygnmi.Value[*oc.Interface], terminalDeviceData *ygnmi.Value[*oc.TerminalDevice_Channel], portName string) oc.E_Interface_OperStatus {
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
	terminalDeviceValue, ok := terminalDeviceData.Val()
	if !ok {
		t.Errorf("Terminal Device data is empty for port %v.", portName)
		return oc.Interface_OperStatus_UNSET
	}
	otn := terminalDeviceValue.GetOtn()
	if otn == nil {
		t.Errorf("OTN data is empty for port %v", portName)
		return operStatus
	}
	if b := otn.GetPreFecBer(); b == nil {
		t.Errorf("PreFECBER data is empty for port %v", portName)
	} else {
		validatePMValue(t, portName, "PreFECBER", b.GetInstant(), b.GetMin(), b.GetMax(), b.GetAvg(), minAllowedPreFECBER, maxAllowedPreFECBER, inactivePreFECBER, operStatus)
	}
	if e := otn.GetEsnr(); e == nil {
		t.Errorf("ESNR data is empty for port %v", portName)
	} else {
		validatePMValue(t, portName, "esnr", e.GetInstant(), e.GetMin(), e.GetMax(), e.GetAvg(), minAllowedESNR, maxAllowedESNR, inactiveESNR, operStatus)
	}
	if q := otn.GetQValue(); q == nil {
		t.Errorf("QValue data is empty for port %v", portName)
	} else {
		validatePMValue(t, portName, "QValue", q.GetInstant(), q.GetMin(), q.GetMax(), q.GetAvg(), minAllowedQValue, maxAllowedQValue, inactiveQValue, operStatus)
	}
	return operStatus
}

// validatePMValue validates the pm value.
func validatePMValue(t *testing.T, portName, pm string, instant, min, max, avg, minAllowed, maxAllowed, inactiveValue float64, operStatus oc.E_Interface_OperStatus) {
	switch operStatus {
	case oc.Interface_OperStatus_UP:
		if instant < minAllowed || instant > maxAllowed {
			t.Errorf("Invalid %v sample when %v is UP --> min : %v, max : %v, avg : %v, instant : %v", pm, portName, min, max, avg, instant)
			return
		}
	case oc.Interface_OperStatus_DOWN:
		if instant != inactiveValue {
			t.Errorf("Invalid %v sample when %v is DOWN --> min : %v, max : %v, avg : %v, instant : %v", pm, portName, min, max, avg, instant)
			return
		}
	}
	t.Logf("Valid %v sample when %v is %v --> min : %v, max : %v, avg : %v, instant : %v", pm, portName, operStatus, min, max, avg, instant)
}
