package zr_platform_paths_test

import (
	"flag"
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	samplingInterval                 = 1 * time.Second
	minAllowedCDValue                = -200
	maxAllowedCDValue                = 2400
	inactiveCDValue                  = 1.0
	inactivePower                    = -30.0
	powerLoss                        = 2.0
	powerReadingError                = 1.0
	minAllowedCarrierFrequencyOffset = -1000.0
	maxAllowedCarrierFrequencyOffset = 1000.0
	inactiveCarrierFrequencyOffset   = 1.0
	minAllowedLaserBiasCurrent       = 0.0
	maxAllowedLaserBiasCurrent       = 300.0
	inactiveLaserBiasCurrent         = 0.0
	minAllowedLaserTemperature       = 25.0
	maxAllowedLaserTemperature       = 75.0
	inactiveLaserTemperature         = 0.0
	minAllowedSupplyVoltage          = 3.0
	maxAllowedSupplyVoltage          = 4.0
	minAllowedTemperature            = 25.0
	maxAllowedTemperature            = 75.0
	errorTolerance                   = 0.05
	timeout                          = 10 * time.Minute
)

var (
	frequencyList          cfgplugins.FrequencyList
	targetOpticalPowerList cfgplugins.TargetOpticalPowerList
	operationalModeList    cfgplugins.OperationalModeList
	componentPath          = "openconfig/components/component[name=%v]"
)

type testcase struct {
	desc           string
	path           string
	got            any
	want           any
	operStatus     oc.E_Interface_OperStatus
	minAllowed     float64
	maxAllowed     float64
	patternToMatch string
}

func init() {
	flag.Var(&operationalModeList, "operational_mode", "operational-mode for the channel.")
	flag.Var(&frequencyList, "frequency", "frequency for the channel.")
	flag.Var(&targetOpticalPowerList, "target_optical_power", "target-optical-power for the channel.")
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestComponentPaths(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	fptest.ConfigureDefaultNetworkInstance(t, dut)

	setToDefaults(t, dut)

	for _, operationalMode := range operationalModeList {
		for _, frequency := range frequencyList {
			for _, targetOpticalPower := range targetOpticalPowerList {

				t.Logf("\n*** Configure interfaces with Operational Mode: %v, Optical Frequency: %v, Target Power: %v\n\n\n", operationalMode, frequency, targetOpticalPower)
				params := &cfgplugins.ConfigParameters{
					Enabled:            true,
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
				hwPortStreams := make(map[string]*samplestream.SampleStream[*oc.Component])
				interfaceStreams := make(map[string]*samplestream.SampleStream[*oc.Interface])
				for _, p := range dut.Ports() {
					ochStreams[p.Name()] = samplestream.New(t, dut, gnmi.OC().Component(params.OpticalChannelNames[p.Name()]).State(), samplingInterval)
					trStreams[p.Name()] = samplestream.New(t, dut, gnmi.OC().Component(params.TransceiverNames[p.Name()]).State(), samplingInterval)
					hwPortStreams[p.Name()] = samplestream.New(t, dut, gnmi.OC().Component(params.HWPortNames[p.Name()]).State(), samplingInterval)
					interfaceStreams[p.Name()] = samplestream.New(t, dut, gnmi.OC().Interface(p.Name()).State(), samplingInterval)
					defer ochStreams[p.Name()].Close()
					defer trStreams[p.Name()].Close()
					defer hwPortStreams[p.Name()].Close()
					defer interfaceStreams[p.Name()].Close()
				}

				for _, p := range dut.Ports() {
					gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
					awaitRxPowerStats(t, p, params, oc.Interface_OperStatus_UP)
				}
				time.Sleep(3 * samplingInterval) // Wait extra time for telemetry to be updated.
				for _, p := range dut.Ports() {
					validateNextSample(t, p, params, oc.Interface_OperStatus_UP, interfaceStreams[p.Name()].Next(), hwPortStreams[p.Name()].Next(), trStreams[p.Name()].Next(), ochStreams[p.Name()].Next())
				}

				t.Log("\n*** Bringing DOWN all interfaces\n\n\n")
				for _, p := range dut.Ports() {
					params.Enabled = false
					cfgplugins.ToggleInterfaceState(t, p, params)
				}

				// Wait for streaming telemetry to report the channels as down and validate stats updated.
				for _, p := range dut.Ports() {
					gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_DOWN)
					awaitRxPowerStats(t, p, params, oc.Interface_OperStatus_DOWN)
				}
				time.Sleep(3 * samplingInterval) // Wait extra time for telemetry to be updated.
				for _, p := range dut.Ports() {
					validateNextSample(t, p, params, oc.Interface_OperStatus_DOWN, interfaceStreams[p.Name()].Next(), hwPortStreams[p.Name()].Next(), trStreams[p.Name()].Next(), ochStreams[p.Name()].Next())
				}

				t.Logf("\n*** Bringing UP all interfaces\n\n\n")
				for _, p := range dut.Ports() {
					params.Enabled = true
					cfgplugins.ToggleInterfaceState(t, p, params)
				}

				// Wait for streaming telemetry to report the channels as up and validate stats updated.
				for _, p := range dut.Ports() {
					gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
					awaitRxPowerStats(t, p, params, oc.Interface_OperStatus_UP)
				}
				time.Sleep(3 * samplingInterval) // Wait extra time for telemetry to be updated.
				for _, p := range dut.Ports() {
					validateNextSample(t, p, params, oc.Interface_OperStatus_UP, interfaceStreams[p.Name()].Next(), hwPortStreams[p.Name()].Next(), trStreams[p.Name()].Next(), ochStreams[p.Name()].Next())
				}
			}
		}
	}
}

// setToDefaults sets the flags to their default values if the flags are not set.
func setToDefaults(t *testing.T, dut *ondatra.DUTDevice) {
	if len(operationalModeList) == 0 {
		operationalModeList = operationalModeList.Default(t, dut)
	}
	if len(frequencyList) == 0 {
		frequencyList = frequencyList.Default(t, dut)
	}
	if len(targetOpticalPowerList) == 0 {
		targetOpticalPowerList = targetOpticalPowerList.Default(t, dut)
	}
}

// awaitRxPowerStats waits for the Rx power stats to be within the expected range.
func awaitRxPowerStats(t *testing.T, p *ondatra.Port, params *cfgplugins.ConfigParameters, operStatus oc.E_Interface_OperStatus) {
	switch operStatus {
	case oc.Interface_OperStatus_UP:
		_, ok := gnmi.Watch(t, p.Device(), gnmi.OC().Component(params.TransceiverNames[p.Name()]).Transceiver().Channel(0).InputPower().State(), timeout, func(rxP *ygnmi.Value[*oc.Component_Transceiver_Channel_InputPower]) bool {
			rxPValue, present := rxP.Val()
			return present &&
				rxPValue.GetMax() <= (params.TargetOpticalPower+powerLoss+powerReadingError) && rxPValue.GetMax() >= (params.TargetOpticalPower-powerLoss) &&
				rxPValue.GetMin() <= (params.TargetOpticalPower+powerLoss+powerReadingError) && rxPValue.GetMin() >= (params.TargetOpticalPower-powerLoss) &&
				rxPValue.GetAvg() <= (params.TargetOpticalPower+powerLoss+powerReadingError) && rxPValue.GetAvg() >= (params.TargetOpticalPower-powerLoss) &&
				rxPValue.GetInstant() <= (params.TargetOpticalPower+powerLoss+powerReadingError) && rxPValue.GetInstant() >= (params.TargetOpticalPower-powerLoss)
		}).Await(t)
		if !ok {
			t.Fatalf("Rx power stats are not as expected for %v after %v minutes.", p.Name(), timeout.Minutes())
		}
	case oc.Interface_OperStatus_DOWN:
		_, ok := gnmi.Watch(t, p.Device(), gnmi.OC().Component(params.TransceiverNames[p.Name()]).Transceiver().Channel(0).InputPower().State(), timeout, func(rxP *ygnmi.Value[*oc.Component_Transceiver_Channel_InputPower]) bool {
			rxPValue, ok := rxP.Val()
			return ok && rxPValue.GetMax() <= inactivePower && rxPValue.GetMin() <= inactivePower && rxPValue.GetAvg() <= inactivePower && rxPValue.GetInstant() <= inactivePower
		}).Await(t)
		if !ok {
			t.Fatalf("Rx power stats are not as expected for %v after %v minutes.", p.Name(), timeout.Minutes())
		}
	default:
		t.Fatalf("Unsupported oper status for %v: %v", p.PMD(), operStatus)
	}
}

// validateNextSample validates the stream data.
func validateNextSample(t *testing.T, p *ondatra.Port, params *cfgplugins.ConfigParameters, wantOperStatus oc.E_Interface_OperStatus, interfaceData *ygnmi.Value[*oc.Interface], hwPortData, transceiverData, opticalChannelData *ygnmi.Value[*oc.Component]) {
	if interfaceData == nil {
		t.Errorf("Data not received for port %v.", p.Name())
		return
	}
	interfaceValue, ok := interfaceData.Val()
	if !ok {
		t.Errorf("Channel data is empty for port %v.", p.Name())
		return
	}
	gotOperStatus := interfaceValue.GetOperStatus()
	if gotOperStatus == oc.Interface_OperStatus_UNSET {
		t.Errorf("Link state data is empty for port %v", p.Name())
		return
	}
	t.Run("Interface operStatus Validation", func(t *testing.T) {
		t.Logf("\nInterface operStatus of %v is %v\n\n", p.Name(), gotOperStatus)
		if diff := cmp.Diff(gotOperStatus, wantOperStatus); diff != "" {
			t.Errorf("Interface operStatus is not as expected, diff (-got +want):\n%s", diff)
		}
	})
	if hwPortData == nil {
		t.Errorf("HW Port data is empty for port %v.", p.Name())
		return
	}
	hwPortValue, ok := hwPortData.Val()
	if !ok {
		t.Errorf("HW Port Value is empty for port %v.", p.Name())
		return
	}
	validateHWPortTelemetry(t, p, params, hwPortValue)
	if transceiverData == nil {
		t.Errorf("Transceiver data is empty for port %v.", p.Name())
		return
	}
	transceiverValue, ok := transceiverData.Val()
	if !ok {
		t.Errorf("Transceiver Value is empty for port %v.", p.Name())
		return
	}
	validateTranscieverTelemetry(t, p, params, transceiverValue, gotOperStatus)
	if opticalChannelData == nil {
		t.Errorf("Optical Channel data is empty for port %v.", p.Name())
		return
	}
	opticalChannelValue, ok := opticalChannelData.Val()
	if !ok {
		t.Errorf("Optical Channel Value is empty for port %v.", p.Name())
		return
	}
	validateOpticalChannelTelemetry(t, p, params, opticalChannelValue, gotOperStatus)
}

// validateHWPortTelemetry validates the hw port telemetry.
func validateHWPortTelemetry(t *testing.T, p *ondatra.Port, params *cfgplugins.ConfigParameters, hwPort *oc.Component) {
	if p.PMD() == ondatra.PMD400GBASEZR || p.PMD() == ondatra.PMD400GBASEZRP {
		// Skip HW Port validation for PMD400GBASEZR/PMD400GBASEZRP as it is not supported.
		return
	}
	tcs := []testcase{
		{
			desc: "HW Port Name Validation",
			path: fmt.Sprintf(componentPath+"/state/name", params.HWPortNames[p.Name()]),
			got:  hwPort.GetName(),
			want: params.HWPortNames[p.Name()],
		},
		{
			desc: "HW Port Location Validation",
			path: fmt.Sprintf(componentPath+"/state/location", params.HWPortNames[p.Name()]),
			got:  hwPort.GetLocation(),
			want: strings.Replace(params.TransceiverNames[p.Name()], "Ethernet", "", 1),
		},
		{
			desc: "HW Port Type Validation",
			path: fmt.Sprintf(componentPath+"/state/type", params.HWPortNames[p.Name()]),
			got:  hwPort.GetType().(oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT),
			want: oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_PORT,
		},
		{
			desc: "HW Port Breakout Index Validation",
			path: fmt.Sprintf(componentPath+"/port/breakout-mode/groups/group[index=1]/state/index", params.HWPortNames[p.Name()]),
			got:  hwPort.GetPort().GetBreakoutMode().GetGroup(1).GetIndex(),
			want: uint8(1),
		},
		{
			desc: "HW Port Breakout Speed Validation",
			path: fmt.Sprintf(componentPath+"/port/breakout-mode/groups/group[index=1]/state/breakout-speed", params.HWPortNames[p.Name()]),
			got:  hwPort.GetPort().GetBreakoutMode().GetGroup(1).GetBreakoutSpeed(),
			want: params.PortSpeed,
		},
		{
			desc: "HW Port Number of Breakouts Validation",
			path: fmt.Sprintf(componentPath+"/port/breakout-mode/groups/group[index=1]/state/num-breakouts", params.HWPortNames[p.Name()]),
			got:  hwPort.GetPort().GetBreakoutMode().GetGroup(1).GetNumBreakouts(),
			want: uint8(1),
		},
		{
			desc: "HW Port Number of Physical Channels Validation",
			path: fmt.Sprintf(componentPath+"/port/breakout-mode/groups/group[index=1]/state/num-physical-channels", params.HWPortNames[p.Name()]),
			got:  hwPort.GetPort().GetBreakoutMode().GetGroup(1).GetNumPhysicalChannels(),
			want: params.NumPhysicalChannels,
		},
	}
	for _, tc := range tcs {
		t.Run(fmt.Sprintf("%s of %v", tc.desc, p.Name()), func(t *testing.T) {
			t.Logf("\n%s: %s = %v\n\n", p.Name(), tc.path, tc.got)
			if diff := cmp.Diff(tc.got, tc.want); diff != "" {
				t.Errorf("\n%s: %s, diff (-got +want):\n%s\n\n", p.Name(), tc.path, diff)
			}
		})
	}
}

// validateTranscieverTelemetry validates the transceiver telemetry.
func validateTranscieverTelemetry(t *testing.T, p *ondatra.Port, params *cfgplugins.ConfigParameters, transceiver *oc.Component, operStatus oc.E_Interface_OperStatus) {
	tcs := []testcase{
		{
			desc: "Transceiver Name Validation",
			path: fmt.Sprintf(componentPath+"/state/name", params.TransceiverNames[p.Name()]),
			got:  transceiver.GetName(),
			want: params.TransceiverNames[p.Name()],
		},
		{
			desc: "Transceiver Parent Validation",
			path: fmt.Sprintf(componentPath+"/state/parent", params.TransceiverNames[p.Name()]),
			got:  transceiver.GetParent(),
			want: params.HWPortNames[p.Name()],
		},
		{
			desc: "Transceiver Location Validation",
			path: fmt.Sprintf(componentPath+"/state/location", params.TransceiverNames[p.Name()]),
			got:  transceiver.GetLocation(),
			want: strings.Replace(params.TransceiverNames[p.Name()], "Ethernet", "", 1),
		},
		{
			desc: "Transceiver removable Validation",
			path: fmt.Sprintf(componentPath+"/state/removable", params.TransceiverNames[p.Name()]),
			got:  transceiver.GetRemovable(),
			want: true,
		},
		{
			desc: "Transceiver Type Validation",
			path: fmt.Sprintf(componentPath+"/state/type", params.TransceiverNames[p.Name()]),
			got:  transceiver.GetType().(oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT),
			want: oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER,
		},
		{
			desc: "Transceiver Oper-Status Validation",
			path: fmt.Sprintf(componentPath+"/state/oper-status", params.TransceiverNames[p.Name()]),
			got:  transceiver.GetOperStatus(),
			want: oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE,
		},
		{
			desc:       "Transceiver Temperature Validation",
			path:       fmt.Sprintf(componentPath+"/state/temperature/instant", params.TransceiverNames[p.Name()]),
			got:        transceiver.GetTemperature().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedTemperature,
			maxAllowed: maxAllowedTemperature,
		},
		{
			desc:           "Transceiver Firmware Version Validation",
			path:           fmt.Sprintf(componentPath+"/state/firmware-version", params.TransceiverNames[p.Name()]),
			got:            transceiver.GetFirmwareVersion(),
			patternToMatch: `.+`,
		},
		{
			desc:           "Transceiver Hardware Version Validation",
			path:           fmt.Sprintf(componentPath+"/state/hardware-version", params.TransceiverNames[p.Name()]),
			got:            transceiver.GetHardwareVersion(),
			patternToMatch: `.+`,
		},
		{
			desc:           "Transceiver Serial Number Validation",
			path:           fmt.Sprintf(componentPath+"/state/serial-no", params.TransceiverNames[p.Name()]),
			got:            transceiver.GetSerialNo(),
			patternToMatch: `.+`,
		},
		{
			desc:           "Transceiver Part Number Validation",
			path:           fmt.Sprintf(componentPath+"/state/part-no", params.TransceiverNames[p.Name()]),
			got:            transceiver.GetPartNo(),
			patternToMatch: `.+`,
		},
		{
			desc:           "Transceiver mfg-name Validation",
			path:           fmt.Sprintf(componentPath+"/state/mfg-name", params.TransceiverNames[p.Name()]),
			got:            transceiver.GetMfgName(),
			patternToMatch: `(CIENA|CISCO|LUMENTUM|NOKIA|INFINERA|ACACIA|MARVEL)`,
		},
		{
			desc:           "Transceiver mfg-date Validation",
			path:           fmt.Sprintf(componentPath+"/state/mfg-date", params.TransceiverNames[p.Name()]),
			got:            transceiver.GetMfgDate(),
			patternToMatch: `\d{4}-\d{2}-\d{2}`,
		},
		{
			desc: "Transceiver Form Factor Validation",
			path: fmt.Sprintf(componentPath+"/transceiver/state/form-factor", params.TransceiverNames[p.Name()]),
			got:  transceiver.GetTransceiver().GetFormFactor(),
			want: params.FormFactor,
		},
		{
			desc: "Transceiver Present Validation",
			path: fmt.Sprintf(componentPath+"/transceiver/state/present", params.TransceiverNames[p.Name()]),
			got:  transceiver.GetTransceiver().GetPresent(),
			want: oc.Transceiver_Present_PRESENT,
		},
		{
			desc: "Transceiver Connector Type Validation",
			path: fmt.Sprintf(componentPath+"/transceiver/state/connector-type", params.TransceiverNames[p.Name()]),
			got:  transceiver.GetTransceiver().GetConnectorType(),
			want: oc.TransportTypes_FIBER_CONNECTOR_TYPE_LC_CONNECTOR,
		},
		{
			desc:       "Transceiver Supply Voltage Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/supply-voltage/instant", params.TransceiverNames[p.Name()]),
			got:        transceiver.GetTransceiver().GetSupplyVoltage().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedSupplyVoltage,
			maxAllowed: maxAllowedSupplyVoltage,
		},
		{
			desc: "Transceiver Physical Channel Index Validation",
			path: fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/index", params.TransceiverNames[p.Name()]),
			got:  transceiver.GetTransceiver().GetChannel(0).GetIndex(),
			want: uint16(0),
		},
		{
			desc:       "Transceiver Physical Channel Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/output-power/instant", params.TransceiverNames[p.Name()]),
			got:        transceiver.GetTransceiver().GetChannel(0).GetOutputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: params.TargetOpticalPower - powerReadingError,
			maxAllowed: params.TargetOpticalPower + powerReadingError,
		},
		{
			desc:       "Transceiver Physical Channel Instant Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/output-power/instant", params.TransceiverNames[p.Name()]),
			got:        transceiver.GetTransceiver().GetChannel(0).GetOutputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Transceiver Physical Channel Instant Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/instant", params.TransceiverNames[p.Name()]),
			got:        transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetMin() - math.Abs(transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetMin())*errorTolerance,
			maxAllowed: transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetMax() + math.Abs(transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetMax())*errorTolerance,
		},
		{
			desc:       "Transceiver Physical Channel Instant Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/instant", params.TransceiverNames[p.Name()]),
			got:        transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Transceiver Physical Channel Average Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/avg", params.TransceiverNames[p.Name()]),
			got:        transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetMin() - math.Abs(transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetMin())*errorTolerance,
			maxAllowed: transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetMax() + math.Abs(transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetMax())*errorTolerance,
		},
		{
			desc:       "Transceiver Physical Channel Average Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/avg", params.TransceiverNames[p.Name()]),
			got:        transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Transceiver Physical Channel Minimum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/min", params.TransceiverNames[p.Name()]),
			got:        transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: params.TargetOpticalPower - powerLoss - powerReadingError,
			maxAllowed: params.TargetOpticalPower + powerLoss + powerReadingError,
		},
		{
			desc:       "Transceiver Physical Channel Minimum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/min", params.TransceiverNames[p.Name()]),
			got:        transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Transceiver Physical Channel Maximum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/max", params.TransceiverNames[p.Name()]),
			got:        transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: params.TargetOpticalPower - powerLoss - powerReadingError,
			maxAllowed: params.TargetOpticalPower + powerLoss + powerReadingError,
		},
		{
			desc:       "Transceiver Physical Channel Maximum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/max", params.TransceiverNames[p.Name()]),
			got:        transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetMax(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
	}
	for _, tc := range tcs {
		if tc.operStatus != oc.Interface_OperStatus_UNSET && tc.operStatus != operStatus {
			// Skip the validation if the operStatus is not the same as the expected operStatus.
			continue
		}
		t.Run(fmt.Sprintf("%s of %v", tc.desc, p.Name()), func(t *testing.T) {
			t.Logf("\n%s: %s = %v\n\n", p.Name(), tc.path, tc.got)
			switch {
			case tc.patternToMatch != "":
				val, ok := tc.got.(string)
				if !ok {
					t.Errorf("\n%s: %s, invalid type: \n got %v want string\n\n", p.Name(), tc.path, tc.got)
				}
				if !regexp.MustCompile(tc.patternToMatch).MatchString(val) {
					t.Errorf("\n%s: %s, invalid:\n got %v, want pattern %v\n\n", p.Name(), tc.path, tc.got, tc.patternToMatch)
				}
			case tc.operStatus == oc.Interface_OperStatus_UP:
				val, ok := tc.got.(float64)
				if !ok {
					t.Errorf("\n%s: %s, invalid type: \n got %v want float64\n\n", p.Name(), tc.path, tc.got)
				}
				if val < tc.minAllowed || val > tc.maxAllowed {
					t.Errorf("\n%s: %s, out of range:\n got %v want >= %v, <= %v\n\n", p.Name(), tc.path, tc.got, tc.minAllowed, tc.maxAllowed)
				}
			case tc.operStatus == oc.Interface_OperStatus_DOWN:
				val, ok := tc.got.(float64)
				if !ok {
					t.Errorf("\n%s: %s, invalid type: \n got %v want float64\n\n", p.Name(), tc.path, tc.got)
				}
				if val > tc.maxAllowed {
					t.Errorf("\n%s: %s, out of range:\n got %v want <= %v\n\n", p.Name(), tc.path, tc.got, tc.maxAllowed)
				}
			default:
				if diff := cmp.Diff(tc.got, tc.want); diff != "" {
					t.Errorf("\n%s: %s, diff (-got +want):\n%s\n\n", p.Name(), tc.path, diff)
				}
			}
		})
	}
}

// validateOpticalChannelTelemetry validates the optical channel telemetry.
func validateOpticalChannelTelemetry(t *testing.T, p *ondatra.Port, params *cfgplugins.ConfigParameters, opticalChannel *oc.Component, operStatus oc.E_Interface_OperStatus) {
	tcs := []testcase{
		{
			desc: "Optical Channel Name Validation",
			path: fmt.Sprintf(componentPath+"/state/name", params.OpticalChannelNames[p.Name()]),
			got:  opticalChannel.GetName(),
			want: params.OpticalChannelNames[p.Name()],
		},
		{
			desc: "Optical Channel Parent Validation",
			path: fmt.Sprintf(componentPath+"/state/parent", params.OpticalChannelNames[p.Name()]),
			got:  opticalChannel.GetParent(),
			want: params.TransceiverNames[p.Name()],
		},
		{
			desc: "Optical Channel Operational Mode Validation",
			path: fmt.Sprintf(componentPath+"/optical-channel/state/operational-mode", params.OpticalChannelNames[p.Name()]),
			got:  opticalChannel.GetOpticalChannel().GetOperationalMode(),
			want: params.OperationalMode,
		},
		{
			desc: "Optical Channel Frequency Validation",
			path: fmt.Sprintf(componentPath+"/optical-channel/state/frequency", params.OpticalChannelNames[p.Name()]),
			got:  opticalChannel.GetOpticalChannel().GetFrequency(),
			want: params.Frequency,
		},
		{
			desc: "Optical Channel Target Output Power Validation",
			path: fmt.Sprintf(componentPath+"/optical-channel/state/target-output-power", params.OpticalChannelNames[p.Name()]),
			got:  opticalChannel.GetOpticalChannel().GetTargetOutputPower(),
			want: params.TargetOpticalPower,
		},
		{
			desc:       "Optical Channel Instant Laser Bias Current Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/laser-bias-current/instant", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetLaserBiasCurrent().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedLaserBiasCurrent,
			maxAllowed: maxAllowedLaserBiasCurrent,
		},
		{
			desc:       "Optical Channel Instant Laser Bias Current Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/laser-bias-current/instant", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetLaserBiasCurrent().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveLaserBiasCurrent,
		},
		{
			desc:       "Optical Channel Instant Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/input-power/instant", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetInputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: opticalChannel.GetOpticalChannel().GetInputPower().GetMin() - math.Abs(opticalChannel.GetOpticalChannel().GetInputPower().GetMin())*errorTolerance,
			maxAllowed: opticalChannel.GetOpticalChannel().GetInputPower().GetMax() + math.Abs(opticalChannel.GetOpticalChannel().GetInputPower().GetMax())*errorTolerance,
		},
		{
			desc:       "Optical Channel Instant Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/input-power/instant", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetInputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Optical Channel Average Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/input-power/avg", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetInputPower().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: opticalChannel.GetOpticalChannel().GetInputPower().GetMin() - math.Abs(opticalChannel.GetOpticalChannel().GetInputPower().GetMin())*errorTolerance,
			maxAllowed: opticalChannel.GetOpticalChannel().GetInputPower().GetMax() + math.Abs(opticalChannel.GetOpticalChannel().GetInputPower().GetMax())*errorTolerance,
		},
		{
			desc:       "Optical Channel Average Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/input-power/avg", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetInputPower().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Optical Channel Minimum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/input-power/min", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetInputPower().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: params.TargetOpticalPower - powerLoss - powerReadingError,
			maxAllowed: params.TargetOpticalPower + powerLoss + powerReadingError,
		},
		{
			desc:       "Optical Channel Minimum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/input-power/min", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetInputPower().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Optical Channel Maximum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/input-power/max", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetInputPower().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: params.TargetOpticalPower - powerLoss - powerReadingError,
			maxAllowed: params.TargetOpticalPower + powerLoss + powerReadingError,
		},
		{
			desc:       "Optical Channel Maximum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/input-power/max", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetInputPower().GetMax(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Optical Channel Instant Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/output-power/instant", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetOutputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: opticalChannel.GetOpticalChannel().GetOutputPower().GetMin() - math.Abs(opticalChannel.GetOpticalChannel().GetOutputPower().GetMin())*errorTolerance,
			maxAllowed: opticalChannel.GetOpticalChannel().GetOutputPower().GetMax() + math.Abs(opticalChannel.GetOpticalChannel().GetOutputPower().GetMax())*errorTolerance,
		},
		{
			desc:       "Optical Channel Instant Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/output-power/instant", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetOutputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Optical Channel Average Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/output-power/avg", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetOutputPower().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: opticalChannel.GetOpticalChannel().GetOutputPower().GetMin() - math.Abs(opticalChannel.GetOpticalChannel().GetOutputPower().GetMin())*errorTolerance,
			maxAllowed: opticalChannel.GetOpticalChannel().GetOutputPower().GetMax() + math.Abs(opticalChannel.GetOpticalChannel().GetOutputPower().GetMax())*errorTolerance,
		},
		{
			desc:       "Optical Channel Average Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/output-power/avg", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetOutputPower().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Optical Channel Minimum Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/output-power/min", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetOutputPower().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: params.TargetOpticalPower - powerReadingError,
			maxAllowed: params.TargetOpticalPower + powerReadingError,
		},
		{
			desc:       "Optical Channel Minimum Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/output-power/min", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetOutputPower().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Optical Channel Maximum Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/output-power/max", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetOutputPower().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: params.TargetOpticalPower - powerReadingError,
			maxAllowed: params.TargetOpticalPower + powerReadingError,
		},
		{
			desc:       "Optical Channel Maximum Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/output-power/max", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetOutputPower().GetMax(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Optical Channel Instant Chromatic Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/chromatic-dispersion/instant", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetMin() - math.Abs(opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetMin())*errorTolerance,
			maxAllowed: opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetMax() + math.Abs(opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetMax())*errorTolerance,
		},
		{
			desc:       "Optical Channel Instant Chromatic Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/chromatic-dispersion/instant", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveCDValue,
		},
		{
			desc:       "Optical Channel Average Chromatic Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/chromatic-dispersion/avg", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetMin() - math.Abs(opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetMin())*errorTolerance,
			maxAllowed: opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetMax() + math.Abs(opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetMax())*errorTolerance,
		},
		{
			desc:       "Optical Channel Average Chromatic Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/chromatic-dispersion/avg", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveCDValue,
		},
		{
			desc:       "Optical Channel Minimum Chromatic Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/chromatic-dispersion/min", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedCDValue,
			maxAllowed: maxAllowedCDValue,
		},
		{
			desc:       "Optical Channel Minimum Chromatic Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/chromatic-dispersion/min", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveCDValue,
		},
		{
			desc:       "Optical Channel Maximum Chromatic Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/chromatic-dispersion/max", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedCDValue,
			maxAllowed: maxAllowedCDValue,
		},
		{
			desc:       "Optical Channel Maximum Chromatic Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/chromatic-dispersion/max", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetMax(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveCDValue,
		},
		{
			desc:       "Optical Channel Instant Carrier Frequency Offset Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/carrier-frequency-offset/instant", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetMin() - math.Abs(opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetMin())*errorTolerance,
			maxAllowed: opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetMax() + math.Abs(opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetMax())*errorTolerance,
		},
		{
			desc:       "Optical Channel Instant Carrier Frequency Offset Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/carrier-frequency-offset/instant", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveCarrierFrequencyOffset,
		},
		{
			desc:       "Optical Channel Average Carrier Frequency Offset Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/carrier-frequency-offset/avg", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetMin() - math.Abs(opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetMin())*errorTolerance,
			maxAllowed: opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetMax() + math.Abs(opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetMax())*errorTolerance,
		},
		{
			desc:       "Optical Channel Average Carrier Frequency Offset Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/carrier-frequency-offset/avg", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveCarrierFrequencyOffset,
		},
		{
			desc:       "Optical Channel Minimum Carrier Frequency Offset Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/carrier-frequency-offset/min", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedCarrierFrequencyOffset,
			maxAllowed: maxAllowedCarrierFrequencyOffset,
		},
		{
			desc:       "Optical Channel Minimum Carrier Frequency Offset Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/carrier-frequency-offset/min", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveCarrierFrequencyOffset,
		},
		{
			desc:       "Optical Channel Maximum Carrier Frequency Offset Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/carrier-frequency-offset/max", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedCarrierFrequencyOffset,
			maxAllowed: maxAllowedCarrierFrequencyOffset,
		},
		{
			desc:       "Optical Channel Maximum Carrier Frequency Offset Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/carrier-frequency-offset/max", params.OpticalChannelNames[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetMax(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveCarrierFrequencyOffset,
		},
	}
	for _, tc := range tcs {
		if tc.operStatus != oc.Interface_OperStatus_UNSET && tc.operStatus != operStatus {
			// Skip the validation if the operStatus is not the same as the expected operStatus.
			continue
		}
		t.Run(fmt.Sprintf("%s of %v", tc.desc, p.Name()), func(t *testing.T) {
			t.Logf("\n%s: %s = %v\n\n", p.Name(), tc.path, tc.got)
			switch tc.operStatus {
			case oc.Interface_OperStatus_UP:
				val, ok := tc.got.(float64)
				if !ok {
					t.Errorf("\n%s: %s, invalid type: \n got %v want float64\n\n", p.Name(), tc.path, tc.got)
				}
				if val < tc.minAllowed || val > tc.maxAllowed {
					t.Errorf("\n%s: %s, out of range:\n got %v want >= %v, <= %v\n\n", p.Name(), tc.path, tc.got, tc.minAllowed, tc.maxAllowed)
				}
			case oc.Interface_OperStatus_DOWN:
				val, ok := tc.got.(float64)
				if !ok {
					t.Errorf("\n%s: %s, invalid type: \n got %v want float64\n\n", p.Name(), tc.path, tc.got)
				}
				if val > tc.maxAllowed {
					t.Errorf("\n%s: %s, out of range:\n got %v want <= %v\n\n", p.Name(), tc.path, tc.got, tc.maxAllowed)
				}
			default:
				if diff := cmp.Diff(tc.got, tc.want); diff != "" {
					t.Errorf("\nOptical Channel Component: %s, diff (-got +want):\n%s\n\n", tc.desc, diff)
				}
			}
		})
	}
}
