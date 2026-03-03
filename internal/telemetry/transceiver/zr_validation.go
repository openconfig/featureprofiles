// Package transceiver provides functions to validate the transceiver telemetry.
package transceiver

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	samplingInterval = 10 * time.Second
	errorTolerance   = 0.05
	timeout          = 10 * time.Minute
)

var (
	logicalChannelPath = "openconfig/terminal-device/logical-channels/channel[index=%v]"
	componentPath      = "openconfig/components/component[name=%v]"
	interfacePath      = "openconfig/interfaces/interface[name=%v]"
)

// TunableParamters contains the tunable parameters for the test.
// This includes the operational modes, optical frequencies, and target optical powers.
type TunableParamters struct {
	OperationalModeList    cfgplugins.OperationalModeList
	FrequencyList          cfgplugins.FrequencyList
	TargetOpticalPowerList cfgplugins.TargetOpticalPowerList
}

type testcase struct {
	desc           string
	path           string
	got            any
	want           any
	oneOf          []float64
	operStatus     oc.E_Interface_OperStatus
	minAllowed     float64
	maxAllowed     float64
	patternToMatch string
}

// TerminalDevicePathsTest tests the terminal device paths for the given operational modes, optical
// frequencies, and target optical powers.
func TerminalDevicePathsTest(t *testing.T, tp *TunableParamters) {
	t.Helper()

	dut := ondatra.DUT(t, "dut")

	fptest.ConfigureDefaultNetworkInstance(t, dut)

	setToDefaults(t, dut, tp)

	for _, operationalMode := range tp.OperationalModeList {
		for _, frequency := range tp.FrequencyList {
			for _, targetOpticalPower := range tp.TargetOpticalPowerList {

				t.Logf("\n*** Configure interfaces with Operational Mode: %v, Optical Frequency: %v, Target Power: %v\n\n\n", operationalMode, frequency, targetOpticalPower)
				params := &cfgplugins.ConfigParameters{
					Enabled:            false, // Set to false to avoid the interface being enabled at startup.
					Frequency:          frequency,
					TargetOpticalPower: targetOpticalPower,
					OperationalMode:    operationalMode,
				}
				batch := &gnmi.SetBatch{}
				cfgplugins.NewInterfaceConfigAll(t, dut, batch, params)
				batch.Set(t, dut)

				// Create sample steams for each port.
				ethStreams := make(map[string]*samplestream.SampleStream[*oc.TerminalDevice_Channel])
				otnStreams := make(map[string]*samplestream.SampleStream[*oc.TerminalDevice_Channel])
				interfaceStreams := make(map[string]*samplestream.SampleStream[*oc.Interface])
				for _, p := range dut.Ports() {
					ethStreams[p.Name()] = samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(params.ETHIndexes[p.Name()]).State(), samplingInterval)
					otnStreams[p.Name()] = samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(params.OTNIndexes[p.Name()]).State(), samplingInterval)
					interfaceStreams[p.Name()] = samplestream.New(t, dut, gnmi.OC().Interface(p.Name()).State(), samplingInterval)
					defer ethStreams[p.Name()].Close()
					defer otnStreams[p.Name()].Close()
					defer interfaceStreams[p.Name()].Close()
				}

				// Ensure all interfaces are DOWN after the config push.
				for _, p := range dut.Ports() {
					validateInterfaceTelemetry(t, dut, p, params, oc.Interface_OperStatus_DOWN, interfaceStreams[p.Name()])
					validateOTNChannelTelemetry(t, dut, p, params, oc.Interface_OperStatus_DOWN, otnStreams[p.Name()])
					validateEthernetChannelTelemetry(t, dut, p, params, ethStreams[p.Name()])
				}

				t.Logf("\n*** Bringing UP all interfaces\n\n\n")
				for _, p := range dut.Ports() {
					params.Enabled = true
					cfgplugins.ToggleInterfaceState(t, dut, p, params)
				}
				for _, p := range dut.Ports() {
					validateInterfaceTelemetry(t, dut, p, params, oc.Interface_OperStatus_UP, interfaceStreams[p.Name()])
					validateOTNChannelTelemetry(t, dut, p, params, oc.Interface_OperStatus_UP, otnStreams[p.Name()])
					validateEthernetChannelTelemetry(t, dut, p, params, ethStreams[p.Name()])
				}

				t.Logf("\n*** Bringing DOWN all interfaces\n\n\n")
				for _, p := range dut.Ports() {
					params.Enabled = false
					cfgplugins.ToggleInterfaceState(t, dut, p, params)
				}
				for _, p := range dut.Ports() {
					validateInterfaceTelemetry(t, dut, p, params, oc.Interface_OperStatus_DOWN, interfaceStreams[p.Name()])
					validateOTNChannelTelemetry(t, dut, p, params, oc.Interface_OperStatus_DOWN, otnStreams[p.Name()])
					validateEthernetChannelTelemetry(t, dut, p, params, ethStreams[p.Name()])
				}

				t.Logf("\n*** Bringing UP all interfaces\n\n\n")
				for _, p := range dut.Ports() {
					params.Enabled = true
					cfgplugins.ToggleInterfaceState(t, dut, p, params)
				}
				for _, p := range dut.Ports() {
					validateInterfaceTelemetry(t, dut, p, params, oc.Interface_OperStatus_UP, interfaceStreams[p.Name()])
					validateOTNChannelTelemetry(t, dut, p, params, oc.Interface_OperStatus_UP, otnStreams[p.Name()])
					validateEthernetChannelTelemetry(t, dut, p, params, ethStreams[p.Name()])
				}

				// Close all streams.
				for _, p := range dut.Ports() {
					ethStreams[p.Name()].Close()
					otnStreams[p.Name()].Close()
					interfaceStreams[p.Name()].Close()
				}
			}
		}
	}
}

// PlatformPathsTest tests the platform paths for the given operational modes, optical frequencies,
// and target optical powers.
func PlatformPathsTest(t *testing.T, tp *TunableParamters) {
	t.Helper()

	dut := ondatra.DUT(t, "dut")

	fptest.ConfigureDefaultNetworkInstance(t, dut)

	setToDefaults(t, dut, tp)

	for _, operationalMode := range tp.OperationalModeList {
		for _, frequency := range tp.FrequencyList {
			for _, targetOpticalPower := range tp.TargetOpticalPowerList {

				t.Logf("\n*** Configure interfaces with Operational Mode: %v, Optical Frequency: %v, Target Power: %v\n\n\n", operationalMode, frequency, targetOpticalPower)
				params := &cfgplugins.ConfigParameters{
					Enabled:            false, // Set to false to avoid the interface being enabled at startup.
					Frequency:          frequency,
					TargetOpticalPower: targetOpticalPower,
					OperationalMode:    operationalMode,
				}
				batch := &gnmi.SetBatch{}
				cfgplugins.NewInterfaceConfigAll(t, dut, batch, params)
				batch.Set(t, dut)

				// Create sample steams for each port.
				ochStreams := make(map[string]*samplestream.SampleStream[*oc.Component])
				trStreams := make(map[string]*samplestream.SampleStream[*oc.Component])
				tempSensorStreams := make(map[string]*samplestream.SampleStream[*oc.Component])
				hwPortStreams := make(map[string]*samplestream.SampleStream[*oc.Component])
				interfaceStreams := make(map[string]*samplestream.SampleStream[*oc.Interface])
				for _, p := range dut.Ports() {
					ochStreams[p.Name()] = samplestream.New(t, dut, gnmi.OC().Component(params.OpticalChannelNames[p.Name()]).State(), samplingInterval)
					trStreams[p.Name()] = samplestream.New(t, dut, gnmi.OC().Component(params.TransceiverNames[p.Name()]).State(), samplingInterval)
					hwPortStreams[p.Name()] = samplestream.New(t, dut, gnmi.OC().Component(params.HWPortNames[p.Name()]).State(), samplingInterval)
					interfaceStreams[p.Name()] = samplestream.New(t, dut, gnmi.OC().Interface(p.Name()).State(), samplingInterval)
					tempSensorStreams[p.Name()] = samplestream.New(t, dut, gnmi.OC().Component(params.TempSensorNames[p.Name()]).State(), samplingInterval)
					defer ochStreams[p.Name()].Close()
					defer trStreams[p.Name()].Close()
					defer hwPortStreams[p.Name()].Close()
					defer interfaceStreams[p.Name()].Close()
					defer tempSensorStreams[p.Name()].Close()
				}

				// Ensure all interfaces are DOWN after the config push.
				for _, p := range dut.Ports() {
					validateInterfaceTelemetry(t, dut, p, params, oc.Interface_OperStatus_DOWN, interfaceStreams[p.Name()])
					validateTranscieverTelemetry(t, dut, p, params, oc.Interface_OperStatus_DOWN, trStreams[p.Name()])
					validateTempSensorTelemetry(t, dut, p, params, oc.Interface_OperStatus_DOWN, tempSensorStreams[p.Name()])
					validateOpticalChannelTelemetry(t, p, params, oc.Interface_OperStatus_DOWN, ochStreams[p.Name()])
					validateHWPortTelemetry(t, dut, p, params, hwPortStreams[p.Name()])
				}

				t.Logf("\n*** Bringing UP all interfaces\n\n\n")
				for _, p := range dut.Ports() {
					params.Enabled = true
					cfgplugins.ToggleInterfaceState(t, dut, p, params)
				}
				for _, p := range dut.Ports() {
					validateInterfaceTelemetry(t, dut, p, params, oc.Interface_OperStatus_UP, interfaceStreams[p.Name()])
					validateTranscieverTelemetry(t, dut, p, params, oc.Interface_OperStatus_UP, trStreams[p.Name()])
					validateTempSensorTelemetry(t, dut, p, params, oc.Interface_OperStatus_DOWN, tempSensorStreams[p.Name()])
					validateOpticalChannelTelemetry(t, p, params, oc.Interface_OperStatus_UP, ochStreams[p.Name()])
					validateHWPortTelemetry(t, dut, p, params, hwPortStreams[p.Name()])
				}

				t.Logf("\n*** Bringing DOWN all interfaces\n\n\n")
				for _, p := range dut.Ports() {
					params.Enabled = false
					cfgplugins.ToggleInterfaceState(t, dut, p, params)
				}
				for _, p := range dut.Ports() {
					validateInterfaceTelemetry(t, dut, p, params, oc.Interface_OperStatus_DOWN, interfaceStreams[p.Name()])
					validateTranscieverTelemetry(t, dut, p, params, oc.Interface_OperStatus_DOWN, trStreams[p.Name()])
					validateTempSensorTelemetry(t, dut, p, params, oc.Interface_OperStatus_DOWN, tempSensorStreams[p.Name()])
					validateOpticalChannelTelemetry(t, p, params, oc.Interface_OperStatus_DOWN, ochStreams[p.Name()])
					validateHWPortTelemetry(t, dut, p, params, hwPortStreams[p.Name()])
				}

				t.Logf("\n*** Bringing UP all interfaces\n\n\n")
				for _, p := range dut.Ports() {
					params.Enabled = true
					cfgplugins.ToggleInterfaceState(t, dut, p, params)
				}
				for _, p := range dut.Ports() {
					validateInterfaceTelemetry(t, dut, p, params, oc.Interface_OperStatus_UP, interfaceStreams[p.Name()])
					validateTranscieverTelemetry(t, dut, p, params, oc.Interface_OperStatus_UP, trStreams[p.Name()])
					validateTempSensorTelemetry(t, dut, p, params, oc.Interface_OperStatus_DOWN, tempSensorStreams[p.Name()])
					validateOpticalChannelTelemetry(t, p, params, oc.Interface_OperStatus_UP, ochStreams[p.Name()])
					validateHWPortTelemetry(t, dut, p, params, hwPortStreams[p.Name()])
				}

				// Close all streams.
				for _, p := range dut.Ports() {
					ochStreams[p.Name()].Close()
					trStreams[p.Name()].Close()
					hwPortStreams[p.Name()].Close()
					interfaceStreams[p.Name()].Close()
					tempSensorStreams[p.Name()].Close()
				}
			}
		}
	}
}

// setToDefaults sets the flags to their default values if the flags are not set.
func setToDefaults(t *testing.T, dut *ondatra.DUTDevice, tp *TunableParamters) {
	if len(tp.OperationalModeList) == 0 {
		tp.OperationalModeList = tp.OperationalModeList.Default(t, dut)
	}
	if len(tp.FrequencyList) == 0 {
		tp.FrequencyList = tp.FrequencyList.Default(t, dut)
	}
	if len(tp.TargetOpticalPowerList) == 0 {
		tp.TargetOpticalPowerList = tp.TargetOpticalPowerList.Default(t, dut)
	}
}
