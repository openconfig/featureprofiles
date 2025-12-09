package transceiver

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

// validateEthernetChannelTelemetry validates the ethernet channel telemetry.
func validateEthernetChannelTelemetry(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, params *cfgplugins.ConfigParameters, ethChannelStream *samplestream.SampleStream[*oc.TerminalDevice_Channel]) {
	nextETHChannelSample := ethChannelStream.Next()
	ethChannelValue, ok := nextETHChannelSample.Val()
	if !ok {
		t.Fatalf("Ethernet Channel value is empty for port %v.", p.Name())
	}
	var assignmentIndex uint32
	if deviations.EthChannelAssignmentCiscoNumbering(dut) {
		assignmentIndex = 1
	} else {
		assignmentIndex = 0
	}
	tcs := []testcase{
		{
			desc: "ETH Index Validation",
			path: fmt.Sprintf(logicalChannelPath+"/state/index", params.ETHIndexes[p.Name()]),
			got:  ethChannelValue.GetIndex(),
			want: params.ETHIndexes[p.Name()],
		},
		{
			desc: "ETH Description Validation",
			path: fmt.Sprintf(logicalChannelPath+"/state/description", params.ETHIndexes[p.Name()]),
			got:  ethChannelValue.GetDescription(),
			want: "ETH Logical Channel",
		},
		{
			desc: "ETH Logical Channel Type Validation",
			path: fmt.Sprintf(logicalChannelPath+"/state/logical-channel-type", params.ETHIndexes[p.Name()]),
			got:  ethChannelValue.GetLogicalChannelType(),
			want: oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_ETHERNET,
		},
		{
			desc: "ETH Loopback Mode Validation",
			path: fmt.Sprintf(logicalChannelPath+"/state/loopback-mode", params.ETHIndexes[p.Name()]),
			got:  ethChannelValue.GetLoopbackMode(),
			want: oc.TerminalDevice_LoopbackModeType_NONE,
		},
		{
			desc: "ETH Assignment Index Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/index", params.ETHIndexes[p.Name()], assignmentIndex),
			got:  ethChannelValue.GetAssignment(assignmentIndex).GetIndex(),
			want: uint32(assignmentIndex),
		},
		{
			desc: "ETH Assignment Logical Channel Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/logical-channel", params.ETHIndexes[p.Name()], assignmentIndex),
			got:  ethChannelValue.GetAssignment(assignmentIndex).GetLogicalChannel(),
			want: params.OTNIndexes[p.Name()],
		},
		{
			desc: "ETH Assignment Allocation Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/allocation", params.ETHIndexes[p.Name()], assignmentIndex),
			got:  ethChannelValue.GetAssignment(assignmentIndex).GetAllocation(),
			want: params.Allocation,
		},
		{
			desc: "ETH Assignment Type Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/assignment-type", params.ETHIndexes[p.Name()], assignmentIndex),
			got:  ethChannelValue.GetAssignment(assignmentIndex).GetAssignmentType(),
			want: oc.Assignment_AssignmentType_LOGICAL_CHANNEL,
		},
	}
	if !deviations.ChannelRateClassParametersUnsupported(dut) {
		tcs = append(tcs, testcase{
			desc: "ETH Rate Class Validation",
			path: fmt.Sprintf(logicalChannelPath+"/state/rate-class", params.ETHIndexes[p.Name()]),
			got:  ethChannelValue.GetRateClass(),
			want: params.RateClass,
		})
	}
	if !deviations.OTNChannelTribUnsupported(dut) {
		tcs = append(tcs, []testcase{
			{
				desc: "ETH Trib Protocol Validation",
				path: fmt.Sprintf(logicalChannelPath+"/state/trib-protocol", params.ETHIndexes[p.Name()]),
				got:  ethChannelValue.GetTribProtocol(),
				want: params.TribProtocol,
			},
			{
				desc: "ETH Admin State Validation",
				path: fmt.Sprintf(logicalChannelPath+"/state/admin-state", params.ETHIndexes[p.Name()]),
				got:  ethChannelValue.GetAdminState(),
				want: oc.TerminalDevice_AdminStateType_ENABLED,
			},
		}...)
	}
	if !deviations.EthChannelIngressParametersUnsupported(dut) {
		tcs = append(tcs, []testcase{
			{
				desc: "ETH Ingress Interface Validation",
				path: fmt.Sprintf(logicalChannelPath+"/ingress/state/interface", params.ETHIndexes[p.Name()]),
				got:  ethChannelValue.GetIngress().GetInterface(),
				want: p.Name(),
			},
			{
				desc: "ETH Ingress Transceiver Validation",
				path: fmt.Sprintf(logicalChannelPath+"/ingress/state/transceiver", params.ETHIndexes[p.Name()]),
				got:  ethChannelValue.GetIngress().GetTransceiver(),
				want: gnmi.Get(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State()),
			},
		}...)
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
