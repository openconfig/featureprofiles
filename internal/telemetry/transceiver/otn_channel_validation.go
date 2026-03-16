package transceiver

import (
	"fmt"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	minAllowedQValue    = 7.0
	maxAllowedQValue    = 14.0
	minAllowedPreFECBER = 1e-9
	maxAllowedPreFECBER = 1e-2
	minAllowedESNR      = 10.0
	maxAllowedESNR      = 25.0
	inactiveQValue      = 0.0
	inactiveESNR        = 0.0
)

var (
	// Different acceptable values for inactive pre-FEC BER.
	// Cisco returns 0.5 for inactive pre-FEC BER.
	// Arista MVC800 returns 1.0 for inactive pre-FEC BER.
	// All other vendors/platforms return 0.0 for inactive pre-FEC BER.
	inactivePreFECBER = []float64{0.0, 0.5, 1.0}
)

// validateOTNChannelTelemetry validates the OTN channel telemetry.
func validateOTNChannelTelemetry(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, params *cfgplugins.ConfigParameters, wantOperStatus oc.E_Interface_OperStatus, otnChannelStream *samplestream.SampleStream[*oc.TerminalDevice_Channel]) {
	nextOTNChannelSample, ok := otnChannelStream.AwaitNext(timeout, func(v *ygnmi.Value[*oc.TerminalDevice_Channel]) bool {
		val, present := v.Val()
		return present && isOTNChannelReady(val, wantOperStatus)
	})
	if !ok {
		t.Fatalf("OTN Channel %v is not ready after %v minutes.", params.OTNIndexes[p.Name()], timeout.Minutes())
	}
	otnChannelValue, ok := nextOTNChannelSample.Val()
	if !ok {
		t.Fatalf("OTN Channel value is empty for port %v.", p.Name())
	}
	var firstAssignmentIndex uint32
	if deviations.OTNChannelAssignmentCiscoNumbering(dut) {
		firstAssignmentIndex = 1
	} else {
		firstAssignmentIndex = 0
	}
	tcs := []testcase{
		{
			desc: "OTN Index Validation",
			path: fmt.Sprintf(logicalChannelPath+"/state/index", params.OTNIndexes[p.Name()]),
			got:  otnChannelValue.GetIndex(),
			want: params.OTNIndexes[p.Name()],
		},
		{
			desc: "OTN Description Validation",
			path: fmt.Sprintf(logicalChannelPath+"/state/description", params.OTNIndexes[p.Name()]),
			got:  otnChannelValue.GetDescription(),
			want: "OTN Logical Channel",
		},
		{
			desc: "OTN Logical Channel Type Validation",
			path: fmt.Sprintf(logicalChannelPath+"/state/logical-channel-type", params.OTNIndexes[p.Name()]),
			got:  otnChannelValue.GetLogicalChannelType(),
			want: oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_OTN,
		},
		{
			desc: "OTN Loopback Mode Validation",
			path: fmt.Sprintf(logicalChannelPath+"/state/loopback-mode", params.OTNIndexes[p.Name()]),
			got:  otnChannelValue.GetLoopbackMode(),
			want: oc.TerminalDevice_LoopbackModeType_NONE,
		},
		{
			desc: "OTN to Optical Channel Assignment Index Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/index", params.OTNIndexes[p.Name()], firstAssignmentIndex),
			got:  otnChannelValue.GetAssignment(firstAssignmentIndex).GetIndex(),
			want: uint32(firstAssignmentIndex),
		},
		{
			desc: "OTN to Optical Channel Assignment Optical Channel Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/optical-channel", params.OTNIndexes[p.Name()], firstAssignmentIndex),
			got:  otnChannelValue.GetAssignment(firstAssignmentIndex).GetOpticalChannel(),
			want: params.OpticalChannelNames[p.Name()],
		},
		{
			desc: "OTN to Optical Channel Assignment Description Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/description", params.OTNIndexes[p.Name()], firstAssignmentIndex),
			got:  otnChannelValue.GetAssignment(firstAssignmentIndex).GetDescription(),
			want: "OTN to Optical Channel",
		},
		{
			desc: "OTN to Optical Channel Assignment Allocation Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/allocation", params.OTNIndexes[p.Name()], firstAssignmentIndex),
			got:  otnChannelValue.GetAssignment(firstAssignmentIndex).GetAllocation(),
			want: params.Allocation,
		},
		{
			desc: "OTN to Optical Channel Assignment Type Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/assignment-type", params.OTNIndexes[p.Name()], firstAssignmentIndex),
			got:  otnChannelValue.GetAssignment(firstAssignmentIndex).GetAssignmentType(),
			want: oc.Assignment_AssignmentType_OPTICAL_CHANNEL,
		},
		{
			desc:       "Instant QValue Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/q-value/instant", params.OTNIndexes[p.Name()]),
			got:        otnChannelValue.GetOtn().GetQValue().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: otnChannelValue.GetOtn().GetQValue().GetMin() * (1 - errorTolerance),
			maxAllowed: otnChannelValue.GetOtn().GetQValue().GetMax() * (1 + errorTolerance),
		},
		{
			desc:       "Instant QValue Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/q-value/instant", params.OTNIndexes[p.Name()]),
			got:        otnChannelValue.GetOtn().GetQValue().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       inactiveQValue,
		},
		{
			desc:       "Average QValue Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/q-value/avg", params.OTNIndexes[p.Name()]),
			got:        otnChannelValue.GetOtn().GetQValue().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: otnChannelValue.GetOtn().GetQValue().GetMin() * (1 - errorTolerance),
			maxAllowed: otnChannelValue.GetOtn().GetQValue().GetMax() * (1 + errorTolerance),
		},
		{
			desc:       "Average QValue Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/q-value/avg", params.OTNIndexes[p.Name()]),
			got:        otnChannelValue.GetOtn().GetQValue().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       inactiveQValue,
		},
		{
			desc:       "Minimum QValue Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/q-value/min", params.OTNIndexes[p.Name()]),
			got:        otnChannelValue.GetOtn().GetQValue().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedQValue,
			maxAllowed: maxAllowedQValue,
		},
		{
			desc:       "Minimum QValue Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/q-value/min", params.OTNIndexes[p.Name()]),
			got:        otnChannelValue.GetOtn().GetQValue().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       inactiveQValue,
		},
		{
			desc:       "Maximum QValue Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/q-value/max", params.OTNIndexes[p.Name()]),
			got:        otnChannelValue.GetOtn().GetQValue().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedQValue,
			maxAllowed: maxAllowedQValue,
		},
		{
			desc:       "Maximum QValue Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/q-value/max", params.OTNIndexes[p.Name()]),
			got:        otnChannelValue.GetOtn().GetQValue().GetMax(),
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       inactiveQValue,
		},
		{
			desc:       "Instant ESNR Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/esnr/instant", params.OTNIndexes[p.Name()]),
			got:        otnChannelValue.GetOtn().GetEsnr().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: otnChannelValue.GetOtn().GetEsnr().GetMin() * (1 - errorTolerance),
			maxAllowed: otnChannelValue.GetOtn().GetEsnr().GetMax() * (1 + errorTolerance),
		},
		{
			desc:       "Instant ESNR Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/esnr/instant", params.OTNIndexes[p.Name()]),
			got:        otnChannelValue.GetOtn().GetEsnr().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       inactiveESNR,
		},
		{
			desc:       "Average ESNR Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/esnr/avg", params.OTNIndexes[p.Name()]),
			got:        otnChannelValue.GetOtn().GetEsnr().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: otnChannelValue.GetOtn().GetEsnr().GetMin() * (1 - errorTolerance),
			maxAllowed: otnChannelValue.GetOtn().GetEsnr().GetMax() * (1 + errorTolerance),
		},
		{
			desc:       "Average ESNR Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/esnr/avg", params.OTNIndexes[p.Name()]),
			got:        otnChannelValue.GetOtn().GetEsnr().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       inactiveESNR,
		},
		{
			desc:       "Minimum ESNR Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/esnr/min", params.OTNIndexes[p.Name()]),
			got:        otnChannelValue.GetOtn().GetEsnr().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedESNR,
			maxAllowed: maxAllowedESNR,
		},
		{
			desc:       "Minimum ESNR Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/esnr/min", params.OTNIndexes[p.Name()]),
			got:        otnChannelValue.GetOtn().GetEsnr().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       inactiveESNR,
		},
		{
			desc:       "Maximum ESNR Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/esnr/max", params.OTNIndexes[p.Name()]),
			got:        otnChannelValue.GetOtn().GetEsnr().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedESNR,
			maxAllowed: maxAllowedESNR,
		},
		{
			desc:       "Maximum ESNR Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/esnr/max", params.OTNIndexes[p.Name()]),
			got:        otnChannelValue.GetOtn().GetEsnr().GetMax(),
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       inactiveESNR,
		},
		{
			desc:       "Instant PreFECBER Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/pre-fec-ber/instant", params.OTNIndexes[p.Name()]),
			got:        otnChannelValue.GetOtn().GetPreFecBer().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: otnChannelValue.GetOtn().GetPreFecBer().GetMin() * (1 - errorTolerance),
			maxAllowed: otnChannelValue.GetOtn().GetPreFecBer().GetMax() * (1 + errorTolerance),
		},
		{
			desc:       "Instant PreFECBER Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/pre-fec-ber/instant", params.OTNIndexes[p.Name()]),
			got:        otnChannelValue.GetOtn().GetPreFecBer().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			oneOf:      inactivePreFECBER,
		},
		{
			desc:       "Average PreFECBER Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/pre-fec-ber/avg", params.OTNIndexes[p.Name()]),
			got:        otnChannelValue.GetOtn().GetPreFecBer().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: otnChannelValue.GetOtn().GetPreFecBer().GetMin() * (1 - errorTolerance),
			maxAllowed: otnChannelValue.GetOtn().GetPreFecBer().GetMax() * (1 + errorTolerance),
		},
		{
			desc:       "Average PreFECBER Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/pre-fec-ber/avg", params.OTNIndexes[p.Name()]),
			got:        otnChannelValue.GetOtn().GetPreFecBer().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			oneOf:      inactivePreFECBER,
		},
		{
			desc:       "Minimum PreFECBER Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/pre-fec-ber/min", params.OTNIndexes[p.Name()]),
			got:        otnChannelValue.GetOtn().GetPreFecBer().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedPreFECBER,
			maxAllowed: maxAllowedPreFECBER,
		},
		{
			desc:       "Minimum PreFECBER Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/pre-fec-ber/min", params.OTNIndexes[p.Name()]),
			got:        otnChannelValue.GetOtn().GetPreFecBer().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			oneOf:      inactivePreFECBER,
		},
		{
			desc:       "Maximum PreFECBER Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/pre-fec-ber/max", params.OTNIndexes[p.Name()]),
			got:        otnChannelValue.GetOtn().GetPreFecBer().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedPreFECBER,
			maxAllowed: maxAllowedPreFECBER,
		},
		{
			desc:       "Maximum PreFECBER Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/pre-fec-ber/max", params.OTNIndexes[p.Name()]),
			got:        otnChannelValue.GetOtn().GetPreFecBer().GetMax(),
			operStatus: oc.Interface_OperStatus_DOWN,
			oneOf:      inactivePreFECBER,
		},
	}
	if dut.Vendor() != ondatra.JUNIPER {
		tcs = append(tcs, testcase{
			desc:       "FEC Uncorrectable Block Count Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/fec-uncorrectable-blocks", params.OTNIndexes[p.Name()]),
			got:        otnChannelValue.GetOtn().GetFecUncorrectableBlocks(),
			operStatus: oc.Interface_OperStatus_UP,
			want:       uint64(0),
		})
	}
	if deviations.OTNToETHAssignment(dut) {
		tcs = append(tcs, []testcase{
			{
				desc: "OTN to Logical Channel Assignment Index Validation",
				path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/index", params.OTNIndexes[p.Name()], firstAssignmentIndex+1),
				got:  otnChannelValue.GetAssignment(firstAssignmentIndex + 1).GetIndex(),
				want: uint32(firstAssignmentIndex + 1),
			},
			{
				desc: "OTN to Logical Channel Assignment Optical Channel Validation",
				path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/logical-channel", params.OTNIndexes[p.Name()], firstAssignmentIndex+1),
				got:  otnChannelValue.GetAssignment(firstAssignmentIndex + 1).GetLogicalChannel(),
				want: params.ETHIndexes[p.Name()],
			},
			{
				desc: "OTN to Logical Channel Assignment Description Validation",
				path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/description", params.OTNIndexes[p.Name()], firstAssignmentIndex+1),
				got:  otnChannelValue.GetAssignment(firstAssignmentIndex + 1).GetDescription(),
				want: "OTN to ETH",
			},
			{
				desc: "OTN to Logical Channel Assignment Allocation Validation",
				path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/allocation", params.OTNIndexes[p.Name()], firstAssignmentIndex+1),
				got:  otnChannelValue.GetAssignment(firstAssignmentIndex + 1).GetAllocation(),
				want: params.Allocation,
			},
			{
				desc: "OTN to Logical Channel Assignment Type Validation",
				path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/assignment-type", params.OTNIndexes[p.Name()], firstAssignmentIndex+1),
				got:  otnChannelValue.GetAssignment(firstAssignmentIndex + 1).GetAssignmentType(),
				want: oc.Assignment_AssignmentType_LOGICAL_CHANNEL,
			},
		}...)
	}
	if !deviations.OTNChannelTribUnsupported(dut) {
		tcs = append(tcs, []testcase{
			{
				desc: "OTN Trib Protocol Validation",
				path: fmt.Sprintf(logicalChannelPath+"/state/trib-protocol", params.OTNIndexes[p.Name()]),
				got:  otnChannelValue.GetTribProtocol(),
				want: params.TribProtocol,
			},
			{
				desc: "OTN Admin State Validation",
				path: fmt.Sprintf(logicalChannelPath+"/state/admin-state", params.OTNIndexes[p.Name()]),
				got:  otnChannelValue.GetAdminState(),
				want: oc.TerminalDevice_AdminStateType_ENABLED,
			},
		}...)
	}
	for _, tc := range tcs {
		if tc.operStatus != oc.Interface_OperStatus_UNSET && tc.operStatus != wantOperStatus {
			// Skip the validation if the operStatus is not the same as the expected operStatus.
			continue
		}
		t.Run(fmt.Sprintf("%s of %v", tc.desc, p.Name()), func(t *testing.T) {
			t.Logf("\n%s: %s = %v\n\n", p.Name(), tc.path, tc.got)
			switch {
			case len(tc.oneOf) > 0:
				val, ok := tc.got.(float64)
				if !ok {
					t.Errorf("\n%s: %s, invalid type: \n got %v want float64\n\n", p.Name(), tc.path, tc.got)
				}
				if !slices.Contains(tc.oneOf, val) {
					t.Errorf("\n%s: %s, none of the expected values: \n got %v want one of %v\n\n", p.Name(), tc.path, tc.got, tc.oneOf)
				}
			case tc.operStatus == oc.Interface_OperStatus_UP && tc.want != nil:
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
			default:
				if diff := cmp.Diff(tc.got, tc.want); diff != "" {
					t.Errorf("\n%s: %s, diff (-got +want):\n%s\n\n", p.Name(), tc.path, diff)
				}
			}
		})
	}
}

// isOTNChannelReady returns true if the OTN stream is ready for the given oper status.
func isOTNChannelReady(val *oc.TerminalDevice_Channel, operStatus oc.E_Interface_OperStatus) bool {
	switch operStatus {
	case oc.Interface_OperStatus_UP:
		return val.GetOtn().GetQValue().GetMin() <= maxAllowedQValue &&
			val.GetOtn().GetQValue().GetMin() >= minAllowedQValue &&
			val.GetOtn().GetQValue().GetAvg() <= val.GetOtn().GetQValue().GetMax()*(1+errorTolerance) &&
			val.GetOtn().GetQValue().GetAvg() >= val.GetOtn().GetQValue().GetMin()*(1-errorTolerance) &&
			val.GetOtn().GetQValue().GetMax() <= maxAllowedQValue &&
			val.GetOtn().GetQValue().GetMax() >= minAllowedQValue &&
			val.GetOtn().GetQValue().GetInstant() <= val.GetOtn().GetQValue().GetMax()*(1+errorTolerance) &&
			val.GetOtn().GetQValue().GetInstant() >= val.GetOtn().GetQValue().GetMin()*(1-errorTolerance) &&
			val.GetOtn().GetPreFecBer().GetMin() <= maxAllowedPreFECBER &&
			val.GetOtn().GetPreFecBer().GetMin() >= minAllowedPreFECBER &&
			val.GetOtn().GetPreFecBer().GetAvg() <= val.GetOtn().GetPreFecBer().GetMax()*(1+errorTolerance) &&
			val.GetOtn().GetPreFecBer().GetAvg() >= val.GetOtn().GetPreFecBer().GetMin()*(1-errorTolerance) &&
			val.GetOtn().GetPreFecBer().GetMax() <= maxAllowedPreFECBER &&
			val.GetOtn().GetPreFecBer().GetMax() >= minAllowedPreFECBER &&
			val.GetOtn().GetPreFecBer().GetInstant() <= val.GetOtn().GetPreFecBer().GetMax()*(1+errorTolerance) &&
			val.GetOtn().GetPreFecBer().GetInstant() >= val.GetOtn().GetPreFecBer().GetMin()*(1-errorTolerance) &&
			val.GetOtn().GetEsnr().GetMin() <= maxAllowedESNR &&
			val.GetOtn().GetEsnr().GetMin() >= minAllowedESNR &&
			val.GetOtn().GetEsnr().GetAvg() <= val.GetOtn().GetEsnr().GetMax()*(1+errorTolerance) &&
			val.GetOtn().GetEsnr().GetAvg() >= val.GetOtn().GetEsnr().GetMin()*(1-errorTolerance) &&
			val.GetOtn().GetEsnr().GetMax() <= maxAllowedESNR &&
			val.GetOtn().GetEsnr().GetMax() >= minAllowedESNR &&
			val.GetOtn().GetEsnr().GetInstant() <= val.GetOtn().GetEsnr().GetMax()*(1+errorTolerance) &&
			val.GetOtn().GetEsnr().GetInstant() >= val.GetOtn().GetEsnr().GetMin()*(1-errorTolerance)
	default:
		isPreFECBERReady := false
		for _, ipfb := range inactivePreFECBER {
			isPreFECBERReady = isPreFECBERReady ||
				(val.GetOtn().GetPreFecBer().GetMin() == ipfb &&
					val.GetOtn().GetPreFecBer().GetAvg() == ipfb &&
					val.GetOtn().GetPreFecBer().GetMax() == ipfb &&
					val.GetOtn().GetPreFecBer().GetInstant() == ipfb)
		}
		return isPreFECBERReady &&
			val.GetOtn().GetQValue().GetMin() == inactiveQValue &&
			val.GetOtn().GetQValue().GetAvg() == inactiveQValue &&
			val.GetOtn().GetQValue().GetMax() == inactiveQValue &&
			val.GetOtn().GetQValue().GetInstant() == inactiveQValue &&
			val.GetOtn().GetEsnr().GetMin() == inactiveESNR &&
			val.GetOtn().GetEsnr().GetAvg() == inactiveESNR &&
			val.GetOtn().GetEsnr().GetMax() == inactiveESNR &&
			val.GetOtn().GetEsnr().GetInstant() == inactiveESNR
	}
}
