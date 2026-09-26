package transceiver

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	minAllowedTemperature = 25.0
	maxAllowedTemperature = 80.0
)

// validateTranscieverTelemetry validates the transceiver telemetry.
func validateTranscieverTelemetry(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, params *cfgplugins.ConfigParameters, wantOperStatus oc.E_Interface_OperStatus, transceiverStream *samplestream.SampleStream[*oc.Component]) {
	nextTransceiverSample, ok := transceiverStream.AwaitNext(timeout, func(v *ygnmi.Value[*oc.Component]) bool {
		val, present := v.Val()
		return present && isTransceiverStreamReady(val, wantOperStatus, params)
	})
	if !ok {
		t.Fatalf("Transceiver %v is not ready after %v minutes.", params.TransceiverNames[p.Name()], timeout.Minutes())
	}
	transceiverValue, ok := nextTransceiverSample.Val()
	if !ok {
		t.Fatalf("Transceiver value is empty for port %v.", p.Name())
	}
	tcs := []testcase{
		{
			desc: "Transceiver Name Validation",
			path: fmt.Sprintf(componentPath+"/state/name", params.TransceiverNames[p.Name()]),
			got:  transceiverValue.GetName(),
			want: params.TransceiverNames[p.Name()],
		},
		{
			desc: "Transceiver Parent Validation",
			path: fmt.Sprintf(componentPath+"/state/parent", params.TransceiverNames[p.Name()]),
			got:  transceiverValue.GetParent(),
			want: params.HWPortNames[p.Name()],
		},
		{
			desc:           "Transceiver Location Validation",
			path:           fmt.Sprintf(componentPath+"/state/location", params.TransceiverNames[p.Name()]),
			got:            transceiverValue.GetLocation(),
			patternToMatch: strings.Replace(strings.Replace(params.TransceiverNames[p.Name()], "Ethernet", "", 1), "transceiver", "", 1),
		},
		{
			desc: "Transceiver Type Validation",
			path: fmt.Sprintf(componentPath+"/state/type", params.TransceiverNames[p.Name()]),
			got:  transceiverValue.GetType().(oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT),
			want: oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER,
		},
		{
			desc:       "Transceiver Oper-Status Validation",
			path:       fmt.Sprintf(componentPath+"/state/oper-status", params.TransceiverNames[p.Name()]),
			operStatus: oc.Interface_OperStatus_UP,
			got:        transceiverValue.GetOperStatus(),
			want:       oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE,
		},
		{
			desc:           "Transceiver Firmware Version Validation",
			path:           fmt.Sprintf(componentPath+"/state/firmware-version", params.TransceiverNames[p.Name()]),
			got:            transceiverValue.GetFirmwareVersion(),
			patternToMatch: `.+`,
		},
		{
			desc:           "Transceiver Hardware Version Validation",
			path:           fmt.Sprintf(componentPath+"/state/hardware-version", params.TransceiverNames[p.Name()]),
			got:            transceiverValue.GetHardwareVersion(),
			patternToMatch: `.+`,
		},
		{
			desc:           "Transceiver Serial Number Validation",
			path:           fmt.Sprintf(componentPath+"/state/serial-no", params.TransceiverNames[p.Name()]),
			got:            transceiverValue.GetSerialNo(),
			patternToMatch: `.+`,
		},
		{
			desc:           "Transceiver Part Number Validation",
			path:           fmt.Sprintf(componentPath+"/state/part-no", params.TransceiverNames[p.Name()]),
			got:            transceiverValue.GetPartNo(),
			patternToMatch: `.+`,
		},
		{
			desc:           "Transceiver mfg-name Validation",
			path:           fmt.Sprintf(componentPath+"/state/mfg-name", params.TransceiverNames[p.Name()]),
			got:            transceiverValue.GetMfgName(),
			patternToMatch: `(CIENA|CISCO|LUMENTUM|NOKIA|INFINERA|ACACIA|MARVEL)`,
		},
		{
			desc:           "Transceiver mfg-date Validation",
			path:           fmt.Sprintf(componentPath+"/state/mfg-date", params.TransceiverNames[p.Name()]),
			got:            transceiverValue.GetMfgDate(),
			patternToMatch: `\d{4}-\d{2}-\d{2}`,
		},
		{
			desc:       "Transceiver Supply Voltage Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/supply-voltage/instant", params.TransceiverNames[p.Name()]),
			got:        transceiverValue.GetTransceiver().GetSupplyVoltage().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedSupplyVoltage,
			maxAllowed: maxAllowedSupplyVoltage,
		},
		{
			desc: "Transceiver Physical Channel Index Validation",
			path: fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/index", params.TransceiverNames[p.Name()]),
			got:  transceiverValue.GetTransceiver().GetChannel(0).GetIndex(),
			want: uint16(0),
		},
		{
			desc:       "Transceiver Physical Channel Instant Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/output-power/instant", params.TransceiverNames[p.Name()]),
			got:        transceiverValue.GetTransceiver().GetChannel(0).GetOutputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: params.TargetOpticalPower - powerReadingError,
			maxAllowed: params.TargetOpticalPower + powerReadingError,
		},
		{
			desc:       "Transceiver Physical Channel Instant Output Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/output-power/instant", params.TransceiverNames[p.Name()]),
			got:        transceiverValue.GetTransceiver().GetChannel(0).GetOutputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Transceiver Physical Channel Instant Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/instant", params.TransceiverNames[p.Name()]),
			got:        transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetMin() - math.Abs(transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetMin())*errorTolerance,
			maxAllowed: transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetMax() + math.Abs(transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetMax())*errorTolerance,
		},
		{
			desc:       "Transceiver Physical Channel Instant Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/instant", params.TransceiverNames[p.Name()]),
			got:        transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Transceiver Physical Channel Average Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/avg", params.TransceiverNames[p.Name()]),
			got:        transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetMin() - math.Abs(transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetMin())*errorTolerance,
			maxAllowed: transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetMax() + math.Abs(transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetMax())*errorTolerance,
		},
		{
			desc:       "Transceiver Physical Channel Average Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/avg", params.TransceiverNames[p.Name()]),
			got:        transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Transceiver Physical Channel Minimum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/min", params.TransceiverNames[p.Name()]),
			got:        transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: params.TargetOpticalPower - powerLoss - powerReadingError,
			maxAllowed: params.TargetOpticalPower + powerLoss + powerReadingError,
		},
		{
			desc:       "Transceiver Physical Channel Minimum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/min", params.TransceiverNames[p.Name()]),
			got:        transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
		{
			desc:       "Transceiver Physical Channel Maximum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/max", params.TransceiverNames[p.Name()]),
			got:        transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: params.TargetOpticalPower - powerLoss - powerReadingError,
			maxAllowed: params.TargetOpticalPower + powerLoss + powerReadingError,
		},
		{
			desc:       "Transceiver Physical Channel Maximum Input Power Validation",
			path:       fmt.Sprintf(componentPath+"/transceiver/state/physical-channels/channel[index=0]/input-power/max", params.TransceiverNames[p.Name()]),
			got:        transceiverValue.GetTransceiver().GetChannel(0).GetInputPower().GetMax(),
			operStatus: oc.Interface_OperStatus_DOWN,
			maxAllowed: inactivePower,
		},
	}
	if dut.Vendor() != ondatra.NOKIA {
		tcs = append(tcs, testcase{
			desc:       "Transceiver Temperature Validation",
			path:       fmt.Sprintf(componentPath+"/state/temperature/instant", params.TransceiverNames[p.Name()]),
			got:        transceiverValue.GetTemperature().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedTemperature,
			maxAllowed: maxAllowedTemperature,
		})
	}
	if dut.Vendor() != ondatra.JUNIPER {
		tcs = append(tcs, testcase{
			desc: "Transceiver Form Factor Validation",
			path: fmt.Sprintf(componentPath+"/transceiver/state/form-factor", params.TransceiverNames[p.Name()]),
			got:  transceiverValue.GetTransceiver().GetFormFactor(),
			want: params.FormFactor,
		})
	}
	for _, tc := range tcs {
		if tc.operStatus != oc.Interface_OperStatus_UNSET && tc.operStatus != wantOperStatus {
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
			case tc.operStatus != oc.Interface_OperStatus_UNSET && tc.want != nil:
				if diff := cmp.Diff(tc.got, tc.want); diff != "" {
					t.Errorf("\n%s: %s, diff (-got +want):\n%s\n\n", p.Name(), tc.path, diff)
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

// isTransceiverStreamReady returns true if the transceiver stream is ready for the given oper status and params.
func isTransceiverStreamReady(val *oc.Component, operStatus oc.E_Interface_OperStatus, params *cfgplugins.ConfigParameters) bool {
	switch operStatus {
	case oc.Interface_OperStatus_UP:
		return val.GetOperStatus() == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE &&
			val.GetTransceiver().GetChannel(0).GetInputPower().GetMax() <= (params.TargetOpticalPower+powerReadingError) &&
			val.GetTransceiver().GetChannel(0).GetInputPower().GetMax() >= (params.TargetOpticalPower-powerLoss-powerReadingError) &&
			val.GetTransceiver().GetChannel(0).GetInputPower().GetMin() <= (params.TargetOpticalPower+powerReadingError) &&
			val.GetTransceiver().GetChannel(0).GetInputPower().GetMin() >= (params.TargetOpticalPower-powerLoss-powerReadingError) &&
			val.GetTransceiver().GetChannel(0).GetInputPower().GetAvg() <= (val.GetTransceiver().GetChannel(0).GetInputPower().GetMax()+math.Abs(val.GetTransceiver().GetChannel(0).GetInputPower().GetMax())*errorTolerance) &&
			val.GetTransceiver().GetChannel(0).GetInputPower().GetAvg() >= (val.GetTransceiver().GetChannel(0).GetInputPower().GetMin()-math.Abs(val.GetTransceiver().GetChannel(0).GetInputPower().GetMin())*errorTolerance) &&
			val.GetTransceiver().GetChannel(0).GetInputPower().GetInstant() <= (val.GetTransceiver().GetChannel(0).GetInputPower().GetMax()+math.Abs(val.GetTransceiver().GetChannel(0).GetInputPower().GetMax())*errorTolerance) &&
			val.GetTransceiver().GetChannel(0).GetInputPower().GetInstant() >= (val.GetTransceiver().GetChannel(0).GetInputPower().GetMin()-math.Abs(val.GetTransceiver().GetChannel(0).GetInputPower().GetMin())*errorTolerance) &&
			val.GetTransceiver().GetChannel(0).GetOutputPower().GetInstant() <= (params.TargetOpticalPower+powerReadingError) &&
			val.GetTransceiver().GetChannel(0).GetOutputPower().GetInstant() >= (params.TargetOpticalPower-powerLoss-powerReadingError)
	default:
		return val.GetTransceiver().GetChannel(0).GetInputPower().GetMax() <= inactivePower &&
			val.GetTransceiver().GetChannel(0).GetInputPower().GetMin() <= inactivePower &&
			val.GetTransceiver().GetChannel(0).GetInputPower().GetAvg() <= inactivePower &&
			val.GetTransceiver().GetChannel(0).GetInputPower().GetInstant() <= inactivePower &&
			val.GetTransceiver().GetChannel(0).GetOutputPower().GetInstant() <= inactivePower
	}
}
