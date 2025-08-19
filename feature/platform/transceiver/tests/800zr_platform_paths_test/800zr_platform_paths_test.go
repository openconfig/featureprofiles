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
	"github.com/openconfig/featureprofiles/internal/components"
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
	trName                 map[string]string
	ochName                map[string]string
	hwPortName             map[string]string
	formFactor             map[string]oc.E_TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE
	numPhysicalChannels    map[string]uint8
	portSpeed              map[string]oc.E_IfEthernet_ETHERNET_SPEED
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
				batch := &gnmi.SetBatch{}
				cfgplugins.NewInterfaceConfigAll(t, dut, batch, frequency, targetOpticalPower, operationalMode)
				batch.Set(t, dut)

				populateValidationVariables(t, dut, operationalMode)

				// Create sample steams for each port.
				ochStreams := make(map[string]*samplestream.SampleStream[*oc.Component])
				trStreams := make(map[string]*samplestream.SampleStream[*oc.Component])
				hwPortStreams := make(map[string]*samplestream.SampleStream[*oc.Component])
				interfaceStreams := make(map[string]*samplestream.SampleStream[*oc.Interface])
				for _, p := range dut.Ports() {
					ochStreams[p.Name()] = samplestream.New(t, dut, gnmi.OC().Component(ochName[p.Name()]).State(), samplingInterval)
					trStreams[p.Name()] = samplestream.New(t, dut, gnmi.OC().Component(trName[p.Name()]).State(), samplingInterval)
					hwPortStreams[p.Name()] = samplestream.New(t, dut, gnmi.OC().Component(hwPortName[p.Name()]).State(), samplingInterval)
					interfaceStreams[p.Name()] = samplestream.New(t, dut, gnmi.OC().Interface(p.Name()).State(), samplingInterval)
					defer ochStreams[p.Name()].Close()
					defer trStreams[p.Name()].Close()
					defer hwPortStreams[p.Name()].Close()
					defer interfaceStreams[p.Name()].Close()
				}

				for _, p := range dut.Ports() {
					gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
					awaitRxPowerStats(t, p, oc.Interface_OperStatus_UP, targetOpticalPower)
				}
				time.Sleep(3 * samplingInterval) // Wait extra time for telemetry to be updated.
				for _, p := range dut.Ports() {
					validateNextSample(t, p, oc.Interface_OperStatus_UP, interfaceStreams[p.Name()].Next(), hwPortStreams[p.Name()].Next(), trStreams[p.Name()].Next(), ochStreams[p.Name()].Next(), operationalMode, frequency, targetOpticalPower)
				}

				t.Log("\n*** Bringing DOWN all interfaces\n\n\n")
				for _, p := range dut.Ports() {
					cfgplugins.ToggleInterfaceState(t, p, false, operationalMode)
				}

				// Wait for streaming telemetry to report the channels as down and validate stats updated.
				for _, p := range dut.Ports() {
					gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_DOWN)
					awaitRxPowerStats(t, p, oc.Interface_OperStatus_DOWN, targetOpticalPower)
				}
				time.Sleep(3 * samplingInterval) // Wait extra time for telemetry to be updated.
				for _, p := range dut.Ports() {
					validateNextSample(t, p, oc.Interface_OperStatus_DOWN, interfaceStreams[p.Name()].Next(), hwPortStreams[p.Name()].Next(), trStreams[p.Name()].Next(), ochStreams[p.Name()].Next(), operationalMode, frequency, targetOpticalPower)
				}

				t.Logf("\n*** Bringing UP all interfaces\n\n\n")
				for _, p := range dut.Ports() {
					cfgplugins.ToggleInterfaceState(t, p, true, operationalMode)
				}

				// Wait for streaming telemetry to report the channels as up and validate stats updated.
				for _, p := range dut.Ports() {
					gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
					awaitRxPowerStats(t, p, oc.Interface_OperStatus_UP, targetOpticalPower)
				}
				time.Sleep(3 * samplingInterval) // Wait extra time for telemetry to be updated.
				for _, p := range dut.Ports() {
					validateNextSample(t, p, oc.Interface_OperStatus_UP, interfaceStreams[p.Name()].Next(), hwPortStreams[p.Name()].Next(), trStreams[p.Name()].Next(), ochStreams[p.Name()].Next(), operationalMode, frequency, targetOpticalPower)
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

// populateValidationVariables populates the validation parameters.
func populateValidationVariables(t *testing.T, dut *ondatra.DUTDevice, operationalMode uint16) {
	trName = make(map[string]string)
	ochName = make(map[string]string)
	hwPortName = make(map[string]string)
	formFactor = make(map[string]oc.E_TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE)
	numPhysicalChannels = make(map[string]uint8)
	portSpeed = make(map[string]oc.E_IfEthernet_ETHERNET_SPEED)
	for _, p := range dut.Ports() {
		if tr, present := gnmi.Lookup(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State()).Val(); !present {
			t.Fatalf("Transceiver not found for port %v", p.Name())
		} else {
			trName[p.Name()] = tr
		}
		if hwPort, present := gnmi.Lookup(t, dut, gnmi.OC().Interface(p.Name()).HardwarePort().State()).Val(); !present {
			t.Fatalf("Hardware Port not found for port %v", p.Name())
		} else {
			hwPortName[p.Name()] = hwPort
		}
		ochName[p.Name()] = components.OpticalChannelComponentFromPort(t, dut, p)
		switch p.PMD() {
		case ondatra.PMD400GBASEZR, ondatra.PMD400GBASEZRP:
			formFactor[p.Name()] = oc.TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE_QSFP56_DD
			portSpeed[p.Name()] = oc.IfEthernet_ETHERNET_SPEED_SPEED_400GB
			numPhysicalChannels[p.Name()] = 8
		case ondatra.PMD800GBASEZR, ondatra.PMD800GBASEZRP:
			formFactor[p.Name()] = oc.TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE_OSFP
			switch operationalMode {
			case 1, 2:
				portSpeed[p.Name()] = oc.IfEthernet_ETHERNET_SPEED_SPEED_800GB
				numPhysicalChannels[p.Name()] = 8
			case 3, 4:
				portSpeed[p.Name()] = oc.IfEthernet_ETHERNET_SPEED_SPEED_400GB
				numPhysicalChannels[p.Name()] = 4
			default:
				t.Fatalf("Unsupported operational mode for %v: %v", p.PMD(), operationalMode)
			}
		default:
			t.Fatalf("Unsupported PMD type for %v", p.PMD())
		}
	}
}

// awaitRxPowerStats waits for the Rx power stats to be within the expected range.
func awaitRxPowerStats(t *testing.T, p *ondatra.Port, operStatus oc.E_Interface_OperStatus, targetOpticalPower float64) {
	switch operStatus {
	case oc.Interface_OperStatus_UP:
		_, ok := gnmi.Watch(t, p.Device(), gnmi.OC().Component(trName[p.Name()]).Transceiver().Channel(0).InputPower().State(), timeout, func(rxP *ygnmi.Value[*oc.Component_Transceiver_Channel_InputPower]) bool {
			rxPValue, present := rxP.Val()
			return present &&
				rxPValue.GetMax() <= (targetOpticalPower+powerLoss+powerReadingError) && rxPValue.GetMax() >= (targetOpticalPower-powerLoss-powerReadingError) &&
				rxPValue.GetMin() <= (targetOpticalPower+powerLoss+powerReadingError) && rxPValue.GetMin() >= (targetOpticalPower-powerLoss-powerReadingError) &&
				rxPValue.GetAvg() <= (targetOpticalPower+powerLoss+powerReadingError) && rxPValue.GetAvg() >= (targetOpticalPower-powerLoss-powerReadingError) &&
				rxPValue.GetInstant() <= (targetOpticalPower+powerLoss+powerReadingError) && rxPValue.GetInstant() >= (targetOpticalPower-powerLoss-powerReadingError)
		}).Await(t)
		if !ok {
			t.Fatalf("Rx power stats are not as expected for %v after %v minutes.", p.Name(), timeout.Minutes())
		}
	case oc.Interface_OperStatus_DOWN:
		_, ok := gnmi.Watch(t, p.Device(), gnmi.OC().Component(trName[p.Name()]).Transceiver().Channel(0).InputPower().State(), timeout, func(rxP *ygnmi.Value[*oc.Component_Transceiver_Channel_InputPower]) bool {
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
func validateNextSample(t *testing.T, p *ondatra.Port, wantOperStatus oc.E_Interface_OperStatus, interfaceData *ygnmi.Value[*oc.Interface], hwPortData, transceiverData, opticalChannelData *ygnmi.Value[*oc.Component], operationalMode uint16, frequency uint64, targetOpticalPower float64) {
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
	validateHWPortTelemetry(t, p, hwPortValue)
	if transceiverData == nil {
		t.Errorf("Transceiver data is empty for port %v.", p.Name())
		return
	}
	transceiverValue, ok := transceiverData.Val()
	if !ok {
		t.Errorf("Transceiver Value is empty for port %v.", p.Name())
		return
	}
	validateTranscieverTelemetry(t, p, transceiverValue, gotOperStatus, targetOpticalPower)
	if opticalChannelData == nil {
		t.Errorf("Optical Channel data is empty for port %v.", p.Name())
		return
	}
	opticalChannelValue, ok := opticalChannelData.Val()
	if !ok {
		t.Errorf("Optical Channel Value is empty for port %v.", p.Name())
		return
	}
	validateOpticalChannelTelemetry(t, p, opticalChannelValue, gotOperStatus, operationalMode, frequency, targetOpticalPower)
}

// validateHWPortTelemetry validates the hw port telemetry.
func validateHWPortTelemetry(t *testing.T, p *ondatra.Port, hwPort *oc.Component) {
	if p.PMD() == ondatra.PMD400GBASEZR || p.PMD() == ondatra.PMD400GBASEZRP {
		// Skip HW Port validation for PMD400GBASEZR/PMD400GBASEZRP as it is not supported.
		return
	}
	tcs := []testcase{
		{
			desc: "HW Port Name Validation",
			path: fmt.Sprintf(componentPath+"/state/name", hwPortName[p.Name()]),
			got:  hwPort.GetName(),
			want: hwPortName[p.Name()],
		},
		{
			desc: "HW Port Location Validation",
			path: fmt.Sprintf(componentPath+"/state/location", hwPortName[p.Name()]),
			got:  hwPort.GetLocation(),
			want: strings.Replace(trName[p.Name()], "Ethernet", "", 1),
		},
		{
			desc: "HW Port Type Validation",
			path: fmt.Sprintf(componentPath+"/state/type", hwPortName[p.Name()]),
			got:  hwPort.GetType().(oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT),
			want: oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_PORT,
		},
		{
			desc: "HW Port Breakout Index Validation",
			path: fmt.Sprintf(componentPath+"/port/breakout-mode/groups/group[index=1]/state/index", hwPortName[p.Name()]),
			got:  hwPort.GetPort().GetBreakoutMode().GetGroup(1).GetIndex(),
			want: uint8(1),
		},
		{
			desc: "HW Port Breakout Speed Validation",
			path: fmt.Sprintf(componentPath+"/port/breakout-mode/groups/group[index=1]/state/breakout-speed", hwPortName[p.Name()]),
			got:  hwPort.GetPort().GetBreakoutMode().GetGroup(1).GetBreakoutSpeed(),
			want: portSpeed[p.Name()],
		},
		{
			desc: "HW Port Number of Breakouts Validation",
			path: fmt.Sprintf(componentPath+"/port/breakout-mode/groups/group[index=1]/state/num-breakouts", hwPortName[p.Name()]),
			got:  hwPort.GetPort().GetBreakoutMode().GetGroup(1).GetNumBreakouts(),
			want: uint8(1),
		},
		{
			desc: "HW Port Number of Physical Channels Validation",
			path: fmt.Sprintf(componentPath+"/port/breakout-mode/groups/group[index=1]/state/num-physical-channels", hwPortName[p.Name()]),
			got:  hwPort.GetPort().GetBreakoutMode().GetGroup(1).GetNumPhysicalChannels(),
			want: numPhysicalChannels[p.Name()],
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
func validateTranscieverTelemetry(t *testing.T, p *ondatra.Port, transceiver *oc.Component, operStatus oc.E_Interface_OperStatus, targetOpticalPower float64) {
	tcs := []testcase{
		{
			desc: "Transceiver Name Validation",
			path: fmt.Sprintf(componentPath+"/state/name", trName[p.Name()]),
			got:  transceiver.GetName(),
			want: trName[p.Name()],
		},
		{
			desc: "Transceiver Parent Validation",
			path: fmt.Sprintf(componentPath+"/state/parent", trName[p.Name()]),
			got:  transceiver.GetParent(),
			want: hwPortName[p.Name()],
		},
		{
			desc: "Transceiver Location Validation",
			path: fmt.Sprintf(componentPath+"/state/location", trName[p.Name()]),
			got:  transceiver.GetLocation(),
			want: strings.Replace(trName[p.Name()], "Ethernet", "", 1),
		},
		{
			desc: "Transceiver removable Validation",
			path: fmt.Sprintf(componentPath+"/state/removable", trName[p.Name()]),
			got:  transceiver.GetRemovable(),
			want: true,
		},
		{
			desc: "Transceiver Type Validation",
			path: fmt.Sprintf(componentPath+"/state/type", trName[p.Name()]),
			got:  transceiver.GetType().(oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT),
			want: oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER,
		},
		{
			desc: "Transceiver Oper-Status Validation",
			path: fmt.Sprintf(componentPath+"/state/oper-status", trName[p.Name()]),
			got:  transceiver.GetOperStatus(),
			want: oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE,
		},
		{
			desc:       "Transceiver Temperature Validation",
			path:       fmt.Sprintf(componentPath+"/state/temperature/instant", trName[p.Name()]),
			got:        transceiver.GetTemperature().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedTemperature,
			maxAllowed: maxAllowedTemperature,
		},
		{
			desc:           "Transceiver Firmware Version Validation",
			path:           fmt.Sprintf(componentPath+"/state/firmware-version", trName[p.Name()]),
			got:            transceiver.GetFirmwareVersion(),
			patternToMatch: `.+`,
		},
		{
			desc:           "Transceiver Hardware Version Validation",
			path:           fmt.Sprintf(componentPath+"/state/hardware-version", trName[p.Name()]),
			got:            transceiver.GetHardwareVersion(),
			patternToMatch: `.+`,
		},
		{
			desc:           "Transceiver Serial Number Validation",
			path:           fmt.Sprintf(componentPath+"/state/serial-no", trName[p.Name()]),
			got:            transceiver.GetSerialNo(),
			patternToMatch: `.+`,
		},
		{
			desc:           "Transceiver Part Number Validation",
			path:           fmt.Sprintf(componentPath+"/state/part-no", trName[p.Name()]),
			got:            transceiver.GetPartNo(),
			patternToMatch: `.+`,
		},
		{
			desc:           "Transceiver mfg-name Validation",
			path:           fmt.Sprintf(componentPath+"/state/mfg-name", trName[p.Name()]),
			got:            transceiver.GetMfgName(),
			patternToMatch: `(CIENA|CISCO|LUMENTUM|NOKIA|INFINERA|ACACIA|MARVEL)`,
		},
		{
			desc:           "Transceiver mfg-date Validation",
			path:           fmt.Sprintf(componentPath+"/state/mfg-date", trName[p.Name()]),
			got:            transceiver.GetMfgDate(),
			patternToMatch: `\d{4}-\d{2}-\d{2}`,
		},
		{
			desc: "Transceiver Form Factor Validation",
			path: fmt.Sprintf(componentPath+"/transceiver/state/form-factor", trName[p.Name()]),
			got:  transceiver.GetTransceiver().GetFormFactor(),
			want: formFactor[p.Name()],
		},
		{
			desc: "Transceiver Present Validation",
			path: fmt.Sprintf(componentPath+"/transceiver/state/present", trName[p.Name()]),
			got:  transceiver.GetTransceiver().GetPresent(),
			want: oc.Transceiver_Present_PRESENT,
		},
		{
			desc: "Transceiver Connector Type Validation",
			path: fmt.Sprintf(componentPath+"/transceiver/state/connector-type", trName[p.Name()]),
			got:  transceiver.GetTransceiver().GetConnectorType(),
			want: oc.TransportTypes_FIBER_CONNECTOR_TYPE_LC_CONNECTOR,
		},
		{
			desc:       "Transceiver Supply Voltage Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/supply-voltage/instant", trName[p.Name()]),
			got:        transceiver.GetTransceiver().GetSupplyVoltage().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedSupplyVoltage,
			maxAllowed: maxAllowedSupplyVoltage,
		},
		{
			desc: "Transceiver Physical Channel Index Validation",
			path: fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/index", trName[p.Name()]),
			got:  transceiver.GetTransceiver().GetChannel(0).GetIndex(),
			want: uint16(0),
		},
		{
			desc:       "Transceiver Physical Channel Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/output-power/instant", trName[p.Name()]),
			got:        transceiver.GetTransceiver().GetChannel(0).GetOutputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: targetOpticalPower - powerReadingError,
			maxAllowed: targetOpticalPower + powerReadingError,
		},
		{
			desc:       "Transceiver Physical Channel Instant Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/output-power/instant", trName[p.Name()]),
			got:        transceiver.GetTransceiver().GetChannel(0).GetOutputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Transceiver Physical Channel Instant Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/instant", trName[p.Name()]),
			got:        transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetMin() - math.Abs(transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetMin())*errorTolerance,
			maxAllowed: transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetMax() + math.Abs(transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetMax())*errorTolerance,
		},
		{
			desc:       "Transceiver Physical Channel Instant Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/instant", trName[p.Name()]),
			got:        transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Transceiver Physical Channel Average Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/avg", trName[p.Name()]),
			got:        transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetMin() - math.Abs(transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetMin())*errorTolerance,
			maxAllowed: transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetMax() + math.Abs(transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetMax())*errorTolerance,
		},
		{
			desc:       "Transceiver Physical Channel Average Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/avg", trName[p.Name()]),
			got:        transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Transceiver Physical Channel Minimum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/min", trName[p.Name()]),
			got:        transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: targetOpticalPower - powerLoss - powerReadingError,
			maxAllowed: targetOpticalPower + powerLoss + powerReadingError,
		},
		{
			desc:       "Transceiver Physical Channel Minimum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/min", trName[p.Name()]),
			got:        transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Transceiver Physical Channel Maximum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/max", trName[p.Name()]),
			got:        transceiver.GetTransceiver().GetChannel(0).GetInputPower().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: targetOpticalPower - powerLoss - powerReadingError,
			maxAllowed: targetOpticalPower + powerLoss + powerReadingError,
		},
		{
			desc:       "Transceiver Physical Channel Maximum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/max", trName[p.Name()]),
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
func validateOpticalChannelTelemetry(t *testing.T, p *ondatra.Port, opticalChannel *oc.Component, operStatus oc.E_Interface_OperStatus, operationalMode uint16, frequency uint64, targetOpticalPower float64) {
	tcs := []testcase{
		{
			desc: "Optical Channel Name Validation",
			path: fmt.Sprintf(componentPath+"/state/name", ochName[p.Name()]),
			got:  opticalChannel.GetName(),
			want: ochName[p.Name()],
		},
		{
			desc: "Optical Channel Parent Validation",
			path: fmt.Sprintf(componentPath+"/state/parent", ochName[p.Name()]),
			got:  opticalChannel.GetParent(),
			want: trName[p.Name()],
		},
		{
			desc: "Optical Channel Operational Mode Validation",
			path: fmt.Sprintf(componentPath+"/optical-channel/state/operational-mode", ochName[p.Name()]),
			got:  opticalChannel.GetOpticalChannel().GetOperationalMode(),
			want: operationalMode,
		},
		{
			desc: "Optical Channel Frequency Validation",
			path: fmt.Sprintf(componentPath+"/optical-channel/state/frequency", ochName[p.Name()]),
			got:  opticalChannel.GetOpticalChannel().GetFrequency(),
			want: frequency,
		},
		{
			desc: "Optical Channel Target Output Power Validation",
			path: fmt.Sprintf(componentPath+"/optical-channel/state/target-output-power", ochName[p.Name()]),
			got:  opticalChannel.GetOpticalChannel().GetTargetOutputPower(),
			want: targetOpticalPower,
		},
		{
			desc:       "Optical Channel Instant Laser Bias Current Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/laser-bias-current/instant", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetLaserBiasCurrent().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedLaserBiasCurrent,
			maxAllowed: maxAllowedLaserBiasCurrent,
		},
		{
			desc:       "Optical Channel Instant Laser Bias Current Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/laser-bias-current/instant", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetLaserBiasCurrent().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveLaserBiasCurrent,
		},
		{
			desc:       "Optical Channel Instant Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/input-power/instant", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetInputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: opticalChannel.GetOpticalChannel().GetInputPower().GetMin() - math.Abs(opticalChannel.GetOpticalChannel().GetInputPower().GetMin())*errorTolerance,
			maxAllowed: opticalChannel.GetOpticalChannel().GetInputPower().GetMax() + math.Abs(opticalChannel.GetOpticalChannel().GetInputPower().GetMax())*errorTolerance,
		},
		{
			desc:       "Optical Channel Instant Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/input-power/instant", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetInputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Optical Channel Average Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/input-power/avg", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetInputPower().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: opticalChannel.GetOpticalChannel().GetInputPower().GetMin() - math.Abs(opticalChannel.GetOpticalChannel().GetInputPower().GetMin())*errorTolerance,
			maxAllowed: opticalChannel.GetOpticalChannel().GetInputPower().GetMax() + math.Abs(opticalChannel.GetOpticalChannel().GetInputPower().GetMax())*errorTolerance,
		},
		{
			desc:       "Optical Channel Average Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/input-power/avg", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetInputPower().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Optical Channel Minimum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/input-power/min", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetInputPower().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: targetOpticalPower - powerLoss - powerReadingError,
			maxAllowed: targetOpticalPower + powerLoss + powerReadingError,
		},
		{
			desc:       "Optical Channel Minimum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/input-power/min", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetInputPower().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Optical Channel Maximum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/input-power/max", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetInputPower().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: targetOpticalPower - powerLoss - powerReadingError,
			maxAllowed: targetOpticalPower + powerLoss + powerReadingError,
		},
		{
			desc:       "Optical Channel Maximum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/input-power/max", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetInputPower().GetMax(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Optical Channel Instant Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/output-power/instant", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetOutputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: opticalChannel.GetOpticalChannel().GetOutputPower().GetMin() - math.Abs(opticalChannel.GetOpticalChannel().GetOutputPower().GetMin())*errorTolerance,
			maxAllowed: opticalChannel.GetOpticalChannel().GetOutputPower().GetMax() + math.Abs(opticalChannel.GetOpticalChannel().GetOutputPower().GetMax())*errorTolerance,
		},
		{
			desc:       "Optical Channel Instant Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/output-power/instant", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetOutputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Optical Channel Average Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/output-power/avg", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetOutputPower().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: opticalChannel.GetOpticalChannel().GetOutputPower().GetMin() - math.Abs(opticalChannel.GetOpticalChannel().GetOutputPower().GetMin())*errorTolerance,
			maxAllowed: opticalChannel.GetOpticalChannel().GetOutputPower().GetMax() + math.Abs(opticalChannel.GetOpticalChannel().GetOutputPower().GetMax())*errorTolerance,
		},
		{
			desc:       "Optical Channel Average Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/output-power/avg", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetOutputPower().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Optical Channel Minimum Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/output-power/min", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetOutputPower().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: targetOpticalPower - powerReadingError,
			maxAllowed: targetOpticalPower + powerReadingError,
		},
		{
			desc:       "Optical Channel Minimum Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/output-power/min", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetOutputPower().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Optical Channel Maximum Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/output-power/max", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetOutputPower().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: targetOpticalPower - powerReadingError,
			maxAllowed: targetOpticalPower + powerReadingError,
		},
		{
			desc:       "Optical Channel Maximum Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/output-power/max", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetOutputPower().GetMax(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Optical Channel Instant Chromatic Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/chromatic-dispersion/instant", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetMin() - math.Abs(opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetMin())*errorTolerance,
			maxAllowed: opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetMax() + math.Abs(opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetMax())*errorTolerance,
		},
		{
			desc:       "Optical Channel Instant Chromatic Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/chromatic-dispersion/instant", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveCDValue,
		},
		{
			desc:       "Optical Channel Average Chromatic Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/chromatic-dispersion/avg", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetMin() - math.Abs(opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetMin())*errorTolerance,
			maxAllowed: opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetMax() + math.Abs(opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetMax())*errorTolerance,
		},
		{
			desc:       "Optical Channel Average Chromatic Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/chromatic-dispersion/avg", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveCDValue,
		},
		{
			desc:       "Optical Channel Minimum Chromatic Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/chromatic-dispersion/min", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedCDValue,
			maxAllowed: maxAllowedCDValue,
		},
		{
			desc:       "Optical Channel Minimum Chromatic Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/chromatic-dispersion/min", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveCDValue,
		},
		{
			desc:       "Optical Channel Maximum Chromatic Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/chromatic-dispersion/max", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedCDValue,
			maxAllowed: maxAllowedCDValue,
		},
		{
			desc:       "Optical Channel Maximum Chromatic Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/chromatic-dispersion/max", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetChromaticDispersion().GetMax(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveCDValue,
		},
		{
			desc:       "Optical Channel Instant Carrier Frequency Offset Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/carrier-frequency-offset/instant", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetMin() - math.Abs(opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetMin())*errorTolerance,
			maxAllowed: opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetMax() + math.Abs(opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetMax())*errorTolerance,
		},
		{
			desc:       "Optical Channel Instant Carrier Frequency Offset Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/carrier-frequency-offset/instant", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveCarrierFrequencyOffset,
		},
		{
			desc:       "Optical Channel Average Carrier Frequency Offset Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/carrier-frequency-offset/avg", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetMin() - math.Abs(opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetMin())*errorTolerance,
			maxAllowed: opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetMax() + math.Abs(opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetMax())*errorTolerance,
		},
		{
			desc:       "Optical Channel Average Carrier Frequency Offset Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/carrier-frequency-offset/avg", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveCarrierFrequencyOffset,
		},
		{
			desc:       "Optical Channel Minimum Carrier Frequency Offset Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/carrier-frequency-offset/min", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedCarrierFrequencyOffset,
			maxAllowed: maxAllowedCarrierFrequencyOffset,
		},
		{
			desc:       "Optical Channel Minimum Carrier Frequency Offset Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/carrier-frequency-offset/min", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveCarrierFrequencyOffset,
		},
		{
			desc:       "Optical Channel Maximum Carrier Frequency Offset Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/carrier-frequency-offset/max", ochName[p.Name()]),
			got:        opticalChannel.GetOpticalChannel().GetCarrierFrequencyOffset().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedCarrierFrequencyOffset,
			maxAllowed: maxAllowedCarrierFrequencyOffset,
		},
		{
			desc:       "Optical Channel Maximum Carrier Frequency Offset Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/carrier-frequency-offset/max", ochName[p.Name()]),
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
