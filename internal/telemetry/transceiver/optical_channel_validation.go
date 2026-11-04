package transceiver

import (
	"fmt"
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	minAllowedCDValue                = -200
	maxAllowedCDValue                = 2400
	inactiveCDValue                  = 1.0
	inactivePower                    = -25.0
	powerLoss                        = 2.0
	powerReadingError                = 2.0
	minAllowedCarrierFrequencyOffset = -3600.0
	maxAllowedCarrierFrequencyOffset = 3600.0
	inactiveCarrierFrequencyOffset   = 1.0
	minAllowedLaserBiasCurrent       = 0.0
	maxAllowedLaserBiasCurrent       = 1000.0
	inactiveLaserBiasCurrent         = 0.0
	minAllowedLaserTemperature       = 25.0
	maxAllowedLaserTemperature       = 75.0
	inactiveLaserTemperature         = 0.0
	minAllowedSupplyVoltage          = 3.0
	maxAllowedSupplyVoltage          = 4.0
	minAllowedPMDValue               = 0.0
	maxAllowedPMDValue               = 200.0
	inactivePMDValue                 = 1.0
	minAllowedSecondOrderPMDValue    = 0.0
	maxAllowedSecondOrderPMDValue    = 2000.0
	inactiveSecondOrderPMDValue      = 1.0
	minAllowedSOPROCValue            = 0.0
	maxAllowedSOPROCValue            = 5000.0
	inactiveSOPROCValue              = 1.0
)

// validateOpticalChannelTelemetry validates the optical channel telemetry.
func validateOpticalChannelTelemetry(t *testing.T, p *ondatra.Port, params *cfgplugins.ConfigParameters, wantOperStatus oc.E_Interface_OperStatus, opticalChannelStream *samplestream.SampleStream[*oc.Component]) {
	nextOpticalChannelSample, ok := opticalChannelStream.AwaitNext(timeout, func(v *ygnmi.Value[*oc.Component]) bool {
		val, present := v.Val()
		return present && isOpticalChannelStreamReady(val, wantOperStatus, params)
	})
	if !ok {
		t.Fatalf("Optical Channel %v is not ready after %v minutes.", params.OpticalChannelNames[p.Name()], timeout.Minutes())
	}
	opticalChannelValue, ok := nextOpticalChannelSample.Val()
	if !ok {
		t.Fatalf("Optical Channel value is empty for port %v.", p.Name())
	}
	validateOpticalChannelComponent(t, p.Name(), opticalChannelValue, params, wantOperStatus)
}

// validateOpticalChannelComponent validates the optical channel component telemetry leaves against expected ranges.
func validateOpticalChannelComponent(t *testing.T, portName string, opticalChannelValue *oc.Component, params *cfgplugins.ConfigParameters, wantOperStatus oc.E_Interface_OperStatus) {
	tcs := []testcase{
		{
			desc: "Optical Channel Name Validation",
			path: fmt.Sprintf(componentPath+"/state/name", params.OpticalChannelNames[portName]),
			got:  opticalChannelValue.GetName(),
			want: params.OpticalChannelNames[portName],
		},
		{
			desc: "Optical Channel Parent Validation",
			path: fmt.Sprintf(componentPath+"/state/parent", params.OpticalChannelNames[portName]),
			got:  opticalChannelValue.GetParent(),
			want: params.TransceiverNames[portName],
		},
		{
			desc: "Optical Channel Operational Mode Validation",
			path: fmt.Sprintf(componentPath+"/optical-channel/state/operational-mode", params.OpticalChannelNames[portName]),
			got:  opticalChannelValue.GetOpticalChannel().GetOperationalMode(),
			want: params.OperationalMode,
		},
		{
			desc:       "Optical Channel Frequency Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/frequency", params.OpticalChannelNames[portName]),
			got:        float64(opticalChannelValue.GetOpticalChannel().GetFrequency()),
			operStatus: oc.Interface_OperStatus_UP,
			maxAllowed: float64(params.Frequency) + maxAllowedCarrierFrequencyOffset,
			minAllowed: float64(params.Frequency) + minAllowedCarrierFrequencyOffset,
		},
		{
			desc: "Optical Channel Target Output Power Validation",
			path: fmt.Sprintf(componentPath+"/optical-channel/state/target-output-power", params.OpticalChannelNames[portName]),
			got:  opticalChannelValue.GetOpticalChannel().GetTargetOutputPower(),
			want: params.TargetOpticalPower,
		},
		{
			desc: "Optical Channel Line Port Validation",
			path: fmt.Sprintf(componentPath+"/optical-channel/state/line-port", params.OpticalChannelNames[portName]),
			got:  opticalChannelValue.GetOpticalChannel().GetLinePort(),
			want: params.LinePortNames[portName],
		},
		{
			desc:       "Optical Channel Instant Laser Bias Current Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/laser-bias-current/instant", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetLaserBiasCurrent().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedLaserBiasCurrent,
			maxAllowed: maxAllowedLaserBiasCurrent,
		},
		{
			desc:       "Optical Channel Instant Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/input-power/instant", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetInputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: opticalChannelValue.GetOpticalChannel().GetInputPower().GetMin() - math.Abs(opticalChannelValue.GetOpticalChannel().GetInputPower().GetMin())*errorTolerance,
			maxAllowed: opticalChannelValue.GetOpticalChannel().GetInputPower().GetMax() + math.Abs(opticalChannelValue.GetOpticalChannel().GetInputPower().GetMax())*errorTolerance,
		},
		{
			desc:       "Optical Channel Instant Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/input-power/instant", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetInputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Optical Channel Average Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/input-power/avg", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetInputPower().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: opticalChannelValue.GetOpticalChannel().GetInputPower().GetMin() - math.Abs(opticalChannelValue.GetOpticalChannel().GetInputPower().GetMin())*errorTolerance,
			maxAllowed: opticalChannelValue.GetOpticalChannel().GetInputPower().GetMax() + math.Abs(opticalChannelValue.GetOpticalChannel().GetInputPower().GetMax())*errorTolerance,
		},
		{
			desc:       "Optical Channel Average Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/input-power/avg", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetInputPower().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Optical Channel Minimum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/input-power/min", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetInputPower().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: params.TargetOpticalPower - powerLoss - powerReadingError,
			maxAllowed: params.TargetOpticalPower + powerReadingError,
		},
		{
			desc:       "Optical Channel Minimum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/input-power/min", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetInputPower().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Optical Channel Maximum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/input-power/max", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetInputPower().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: params.TargetOpticalPower - powerLoss - powerReadingError,
			maxAllowed: params.TargetOpticalPower + powerReadingError,
		},
		{
			desc:       "Optical Channel Maximum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/input-power/max", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetInputPower().GetMax(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Optical Channel Instant Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/output-power/instant", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetOutputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: opticalChannelValue.GetOpticalChannel().GetOutputPower().GetMin() - math.Abs(opticalChannelValue.GetOpticalChannel().GetOutputPower().GetMin())*errorTolerance,
			maxAllowed: opticalChannelValue.GetOpticalChannel().GetOutputPower().GetMax() + math.Abs(opticalChannelValue.GetOpticalChannel().GetOutputPower().GetMax())*errorTolerance,
		},
		{
			desc:       "Optical Channel Instant Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/output-power/instant", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetOutputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Optical Channel Average Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/output-power/avg", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetOutputPower().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: opticalChannelValue.GetOpticalChannel().GetOutputPower().GetMin() - math.Abs(opticalChannelValue.GetOpticalChannel().GetOutputPower().GetMin())*errorTolerance,
			maxAllowed: opticalChannelValue.GetOpticalChannel().GetOutputPower().GetMax() + math.Abs(opticalChannelValue.GetOpticalChannel().GetOutputPower().GetMax())*errorTolerance,
		},
		{
			desc:       "Optical Channel Average Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/output-power/avg", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetOutputPower().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Optical Channel Minimum Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/output-power/min", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetOutputPower().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: params.TargetOpticalPower - powerReadingError,
			maxAllowed: params.TargetOpticalPower + powerReadingError,
		},
		{
			desc:       "Optical Channel Minimum Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/output-power/min", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetOutputPower().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Optical Channel Maximum Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/output-power/max", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetOutputPower().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: params.TargetOpticalPower - powerReadingError,
			maxAllowed: params.TargetOpticalPower + powerReadingError,
		},
		{
			desc:       "Optical Channel Maximum Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/output-power/max", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetOutputPower().GetMax(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Optical Channel Instant Chromatic Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/chromatic-dispersion/instant", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetChromaticDispersion().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedCDValue,
			maxAllowed: maxAllowedCDValue,
		},
		{
			desc:       "Optical Channel Instant Chromatic Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/chromatic-dispersion/instant", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetChromaticDispersion().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveCDValue,
		},
		{
			desc:       "Optical Channel Average Chromatic Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/chromatic-dispersion/avg", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetChromaticDispersion().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedCDValue,
			maxAllowed: maxAllowedCDValue,
		},
		{
			desc:       "Optical Channel Average Chromatic Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/chromatic-dispersion/avg", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetChromaticDispersion().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveCDValue,
		},
		{
			desc:       "Optical Channel Minimum Chromatic Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/chromatic-dispersion/min", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetChromaticDispersion().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedCDValue,
			maxAllowed: maxAllowedCDValue,
		},
		{
			desc:       "Optical Channel Minimum Chromatic Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/chromatic-dispersion/min", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetChromaticDispersion().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveCDValue,
		},
		{
			desc:       "Optical Channel Maximum Chromatic Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/chromatic-dispersion/max", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetChromaticDispersion().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedCDValue,
			maxAllowed: maxAllowedCDValue,
		},
		{
			desc:       "Optical Channel Maximum Chromatic Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/chromatic-dispersion/max", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetChromaticDispersion().GetMax(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveCDValue,
		},
		{
			desc:       "Optical Channel Instant Carrier Frequency Offset Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/carrier-frequency-offset/instant", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetCarrierFrequencyOffset().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedCarrierFrequencyOffset,
			maxAllowed: maxAllowedCarrierFrequencyOffset,
		},
		{
			desc:       "Optical Channel Average Carrier Frequency Offset Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/carrier-frequency-offset/avg", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetCarrierFrequencyOffset().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedCarrierFrequencyOffset,
			maxAllowed: maxAllowedCarrierFrequencyOffset,
		},
		{
			desc:       "Optical Channel Minimum Carrier Frequency Offset Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/carrier-frequency-offset/min", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetCarrierFrequencyOffset().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedCarrierFrequencyOffset,
			maxAllowed: maxAllowedCarrierFrequencyOffset,
		},
		{
			desc:       "Optical Channel Maximum Carrier Frequency Offset Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/carrier-frequency-offset/max", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetCarrierFrequencyOffset().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedCarrierFrequencyOffset,
			maxAllowed: maxAllowedCarrierFrequencyOffset,
		},
		{
			desc:       "Optical Channel Instant Polarization Mode Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/polarization-mode-dispersion/instant", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetPolarizationModeDispersion().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedPMDValue,
			maxAllowed: maxAllowedPMDValue,
		},
		{
			desc:       "Optical Channel Instant Polarization Mode Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/polarization-mode-dispersion/instant", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetPolarizationModeDispersion().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePMDValue,
		},
		{
			desc:       "Optical Channel Average Polarization Mode Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/polarization-mode-dispersion/avg", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetPolarizationModeDispersion().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedPMDValue,
			maxAllowed: maxAllowedPMDValue,
		},
		{
			desc:       "Optical Channel Average Polarization Mode Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/polarization-mode-dispersion/avg", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetPolarizationModeDispersion().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePMDValue,
		},
		{
			desc:       "Optical Channel Minimum Polarization Mode Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/polarization-mode-dispersion/min", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetPolarizationModeDispersion().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedPMDValue,
			maxAllowed: maxAllowedPMDValue,
		},
		{
			desc:       "Optical Channel Minimum Polarization Mode Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/polarization-mode-dispersion/min", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetPolarizationModeDispersion().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePMDValue,
		},
		{
			desc:       "Optical Channel Maximum Polarization Mode Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/polarization-mode-dispersion/max", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetPolarizationModeDispersion().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedPMDValue,
			maxAllowed: maxAllowedPMDValue,
		},
		{
			desc:       "Optical Channel Maximum Polarization Mode Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/polarization-mode-dispersion/max", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetPolarizationModeDispersion().GetMax(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePMDValue,
		},
		{
			desc:       "Optical Channel Instant Second Order Polarization Mode Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/second-order-polarization-mode-dispersion/instant", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetSecondOrderPolarizationModeDispersion().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedSecondOrderPMDValue,
			maxAllowed: maxAllowedSecondOrderPMDValue,
		},
		{
			desc:       "Optical Channel Instant Second Order Polarization Mode Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/second-order-polarization-mode-dispersion/instant", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetSecondOrderPolarizationModeDispersion().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveSecondOrderPMDValue,
		},
		{
			desc:       "Optical Channel Average Second Order Polarization Mode Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/second-order-polarization-mode-dispersion/avg", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetSecondOrderPolarizationModeDispersion().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedSecondOrderPMDValue,
			maxAllowed: maxAllowedSecondOrderPMDValue,
		},
		{
			desc:       "Optical Channel Average Second Order Polarization Mode Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/second-order-polarization-mode-dispersion/avg", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetSecondOrderPolarizationModeDispersion().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveSecondOrderPMDValue,
		},
		{
			desc:       "Optical Channel Minimum Second Order Polarization Mode Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/second-order-polarization-mode-dispersion/min", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetSecondOrderPolarizationModeDispersion().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedSecondOrderPMDValue,
			maxAllowed: maxAllowedSecondOrderPMDValue,
		},
		{
			desc:       "Optical Channel Minimum Second Order Polarization Mode Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/second-order-polarization-mode-dispersion/min", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetSecondOrderPolarizationModeDispersion().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveSecondOrderPMDValue,
		},
		{
			desc:       "Optical Channel Maximum Second Order Polarization Mode Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/second-order-polarization-mode-dispersion/max", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetSecondOrderPolarizationModeDispersion().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedSecondOrderPMDValue,
			maxAllowed: maxAllowedSecondOrderPMDValue,
		},
		{
			desc:       "Optical Channel Maximum Second Order Polarization Mode Dispersion Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/second-order-polarization-mode-dispersion/max", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetSecondOrderPolarizationModeDispersion().GetMax(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveSecondOrderPMDValue,
		},
		{
			desc:       "Optical Channel Instant SOP-ROC Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/sop-roc/instant", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetSopRoc().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedSOPROCValue,
			maxAllowed: maxAllowedSOPROCValue,
		},
		{
			desc:       "Optical Channel Instant SOP-ROC Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/sop-roc/instant", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetSopRoc().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveSOPROCValue,
		},
		{
			desc:       "Optical Channel Average SOP-ROC Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/sop-roc/avg", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetSopRoc().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedSOPROCValue,
			maxAllowed: maxAllowedSOPROCValue,
		},
		{
			desc:       "Optical Channel Average SOP-ROC Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/sop-roc/avg", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetSopRoc().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveSOPROCValue,
		},
		{
			desc:       "Optical Channel Minimum SOP-ROC Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/sop-roc/min", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetSopRoc().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedSOPROCValue,
			maxAllowed: maxAllowedSOPROCValue,
		},
		{
			desc:       "Optical Channel Minimum SOP-ROC Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/sop-roc/min", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetSopRoc().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveSOPROCValue,
		},
		{
			desc:       "Optical Channel Maximum SOP-ROC Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/sop-roc/max", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetSopRoc().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedSOPROCValue,
			maxAllowed: maxAllowedSOPROCValue,
		},
		{
			desc:       "Optical Channel Maximum SOP-ROC Validation",
			path:       fmt.Sprintf(componentPath+"/optical-channel/state/sop-roc/max", params.OpticalChannelNames[portName]),
			got:        opticalChannelValue.GetOpticalChannel().GetSopRoc().GetMax(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactiveSOPROCValue,
		},
	}
	for _, tc := range tcs {
		if tc.operStatus != oc.Interface_OperStatus_UNSET && tc.operStatus != wantOperStatus {
			// Skip the validation if the operStatus is not the same as the expected operStatus.
			continue
		}
		t.Run(fmt.Sprintf("%s of %v", tc.desc, portName), func(t *testing.T) {
			t.Logf("\n%s: %s = %v\n\n", portName, tc.path, tc.got)
			switch tc.operStatus {
			case oc.Interface_OperStatus_UP:
				val, ok := tc.got.(float64)
				if !ok {
					t.Errorf("\n%s: %s, invalid type: \n got %v want float64\n\n", portName, tc.path, tc.got)
				}
				if val < tc.minAllowed || val > tc.maxAllowed {
					t.Errorf("\n%s: %s, out of range:\n got %v want >= %v, <= %v\n\n", portName, tc.path, tc.got, tc.minAllowed, tc.maxAllowed)
				}
			case oc.Interface_OperStatus_DOWN:
				val, ok := tc.got.(float64)
				if !ok {
					t.Errorf("\n%s: %s, invalid type: \n got %v want float64\n\n", portName, tc.path, tc.got)
				}
				if val > tc.maxAllowed {
					t.Errorf("\n%s: %s, out of range:\n got %v want <= %v\n\n", portName, tc.path, tc.got, tc.maxAllowed)
				}
			default:
				if diff := cmp.Diff(tc.got, tc.want); diff != "" {
					t.Errorf("\nOptical Channel Component: %s, diff (-got +want):\n%s\n\n", tc.desc, diff)
				}
			}
		})
	}
}

// isOpticalChannelStreamReady returns true if the optical channel stream is ready for the given oper status and params.
func isOpticalChannelStreamReady(val *oc.Component, operStatus oc.E_Interface_OperStatus, params *cfgplugins.ConfigParameters) bool {
	if val.GetOpticalChannel().GetOperationalMode() != params.OperationalMode {
		return false
	}
	switch operStatus {
	case oc.Interface_OperStatus_UP:
		return val.GetOpticalChannel().GetInputPower().GetMax() <= (params.TargetOpticalPower+powerReadingError) &&
			val.GetOpticalChannel().GetInputPower().GetMax() >= (params.TargetOpticalPower-powerLoss-powerReadingError) &&
			val.GetOpticalChannel().GetInputPower().GetMin() <= (params.TargetOpticalPower+powerReadingError) &&
			val.GetOpticalChannel().GetInputPower().GetMin() >= (params.TargetOpticalPower-powerLoss-powerReadingError) &&
			val.GetOpticalChannel().GetInputPower().GetAvg() <= (val.GetOpticalChannel().GetInputPower().GetMax()+math.Abs(val.GetOpticalChannel().GetInputPower().GetMax())*errorTolerance) &&
			val.GetOpticalChannel().GetInputPower().GetAvg() >= (val.GetOpticalChannel().GetInputPower().GetMin()-math.Abs(val.GetOpticalChannel().GetInputPower().GetMin())*errorTolerance) &&
			val.GetOpticalChannel().GetInputPower().GetInstant() <= (val.GetOpticalChannel().GetInputPower().GetMax()+math.Abs(val.GetOpticalChannel().GetInputPower().GetMax())*errorTolerance) &&
			val.GetOpticalChannel().GetInputPower().GetInstant() >= (val.GetOpticalChannel().GetInputPower().GetMin()-math.Abs(val.GetOpticalChannel().GetInputPower().GetMin())*errorTolerance) &&
			val.GetOpticalChannel().GetOutputPower().GetMax() <= (params.TargetOpticalPower+powerReadingError) &&
			val.GetOpticalChannel().GetOutputPower().GetMax() >= (params.TargetOpticalPower-powerReadingError) &&
			val.GetOpticalChannel().GetOutputPower().GetMin() <= (params.TargetOpticalPower+powerReadingError) &&
			val.GetOpticalChannel().GetOutputPower().GetMin() >= (params.TargetOpticalPower-powerReadingError) &&
			val.GetOpticalChannel().GetOutputPower().GetAvg() <= (val.GetOpticalChannel().GetOutputPower().GetMax()+math.Abs(val.GetOpticalChannel().GetOutputPower().GetMax())*errorTolerance) &&
			val.GetOpticalChannel().GetOutputPower().GetAvg() >= (val.GetOpticalChannel().GetOutputPower().GetMin()-math.Abs(val.GetOpticalChannel().GetOutputPower().GetMin())*errorTolerance) &&
			val.GetOpticalChannel().GetOutputPower().GetInstant() <= (val.GetOpticalChannel().GetOutputPower().GetMax()+math.Abs(val.GetOpticalChannel().GetOutputPower().GetMax())*errorTolerance) &&
			val.GetOpticalChannel().GetOutputPower().GetInstant() >= (val.GetOpticalChannel().GetOutputPower().GetMin()-math.Abs(val.GetOpticalChannel().GetOutputPower().GetMin())*errorTolerance)
	default:
		return val.GetOpticalChannel().GetInputPower().GetMax() <= inactivePower &&
			val.GetOpticalChannel().GetInputPower().GetMin() <= inactivePower &&
			val.GetOpticalChannel().GetInputPower().GetAvg() <= inactivePower &&
			val.GetOpticalChannel().GetInputPower().GetInstant() <= inactivePower &&
			val.GetOpticalChannel().GetOutputPower().GetMax() <= inactivePower &&
			val.GetOpticalChannel().GetOutputPower().GetMin() <= inactivePower &&
			val.GetOpticalChannel().GetOutputPower().GetAvg() <= inactivePower &&
			val.GetOpticalChannel().GetOutputPower().GetInstant() <= inactivePower
	}
}
