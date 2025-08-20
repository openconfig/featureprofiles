package zr_terminal_device_paths_test

import (
	"flag"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	samplingInterval    = 1 * time.Second
	minAllowedQValue    = 7.0
	maxAllowedQValue    = 14.0
	minAllowedPreFECBER = 1e-9
	maxAllowedPreFECBER = 1e-2
	minAllowedESNR      = 10.0
	maxAllowedESNR      = 25.0
	inactiveQValue      = 0.0
	inactiveESNR        = 0.0
	errorTolerance      = 0.05
	timeout             = 10 * time.Minute
)

var (
	frequencyList          cfgplugins.FrequencyList
	targetOpticalPowerList cfgplugins.TargetOpticalPowerList
	operationalModeList    cfgplugins.OperationalModeList
	logicalChannelPath     = "openconfig/terminal-device/logical-channels/channel[index=%v]"
	// Different acceptable values for inactive pre-FEC BER.
	// Cisco returns 0.5 for inactive pre-FEC BER.
	// Arista MVC800 returns 1.0 for inactive pre-FEC BER.
	// All other vendors/platforms return 0.0 for inactive pre-FEC BER.
	inactivePreFECBER = []float64{0.0, 0.5, 1.0}
)

type testcase struct {
	desc       string
	path       string
	got        any
	want       any
	oneOf      []float64
	operStatus oc.E_Interface_OperStatus
	minAllowed float64
	maxAllowed float64
}

func init() {
	flag.Var(&operationalModeList, "operational_mode", "operational-mode for the channel.")
	flag.Var(&frequencyList, "frequency", "frequency for the channel.")
	flag.Var(&targetOpticalPowerList, "target_optical_power", "target-optical-power for the channel.")
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestTerminalDevicePaths(t *testing.T) {
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

				// Wait for streaming telemetry to report the channels as up.
				for _, p := range dut.Ports() {
					gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
					awaitQValueStats(t, dut, p, params, oc.Interface_OperStatus_UP)
				}
				time.Sleep(3 * samplingInterval) // Wait extra time for telemetry to be updated.
				for _, p := range dut.Ports() {
					validateNextSample(t, dut, p, params, oc.Interface_OperStatus_UP, interfaceStreams[p.Name()].Next(), otnStreams[p.Name()].Next(), ethStreams[p.Name()].Next(), operationalMode)
				}

				t.Logf("\n*** Bringing DOWN all interfaces\n\n\n")
				for _, p := range dut.Ports() {
					params.Enabled = false
					cfgplugins.ToggleInterfaceState(t, p, params)
				}

				// Wait for streaming telemetry to report the channels as down and validate stats updated.
				for _, p := range dut.Ports() {
					gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_DOWN)
					awaitQValueStats(t, dut, p, params, oc.Interface_OperStatus_DOWN)
				}
				time.Sleep(3 * samplingInterval) // Wait extra time for telemetry to be updated.
				for _, p := range dut.Ports() {
					validateNextSample(t, dut, p, params, oc.Interface_OperStatus_DOWN, interfaceStreams[p.Name()].Next(), otnStreams[p.Name()].Next(), ethStreams[p.Name()].Next(), operationalMode)
				}

				t.Logf("\n*** Bringing UP all interfaces\n\n\n")
				for _, p := range dut.Ports() {
					params.Enabled = true
					cfgplugins.ToggleInterfaceState(t, p, params)
				}

				// Wait for streaming telemetry to report the channels as up and validate stats updated.
				for _, p := range dut.Ports() {
					gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
					awaitQValueStats(t, dut, p, params, oc.Interface_OperStatus_UP)
				}
				time.Sleep(3 * samplingInterval) // Wait extra time for telemetry to be updated.
				for _, p := range dut.Ports() {
					validateNextSample(t, dut, p, params, oc.Interface_OperStatus_UP, interfaceStreams[p.Name()].Next(), otnStreams[p.Name()].Next(), ethStreams[p.Name()].Next(), operationalMode)
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

// awaitQValueStats waits for the QValue stats (i.e., min/max/avg) to be within the expected range.
func awaitQValueStats(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, params *cfgplugins.ConfigParameters, operStatus oc.E_Interface_OperStatus) {
	switch operStatus {
	case oc.Interface_OperStatus_UP:
		_, ok := gnmi.Watch(t, dut, gnmi.OC().TerminalDevice().Channel(params.OTNIndexes[p.Name()]).Otn().QValue().State(), timeout, func(min *ygnmi.Value[*oc.TerminalDevice_Channel_Otn_QValue]) bool {
			qValue, present := min.Val()
			return present &&
				qValue.GetMin() <= maxAllowedQValue && qValue.GetMin() >= minAllowedQValue &&
				qValue.GetAvg() <= maxAllowedQValue && qValue.GetAvg() >= minAllowedQValue &&
				qValue.GetMax() <= maxAllowedQValue && qValue.GetMax() >= minAllowedQValue &&
				qValue.GetInstant() <= maxAllowedQValue && qValue.GetInstant() >= minAllowedQValue
		}).Await(t)
		if !ok {
			t.Fatalf("QValue stats are not as expected for %v after %v minutes.", p.Name(), timeout.Minutes())
		}
	case oc.Interface_OperStatus_DOWN:
		_, ok := gnmi.Watch(t, dut, gnmi.OC().TerminalDevice().Channel(params.OTNIndexes[p.Name()]).Otn().QValue().State(), timeout, func(max *ygnmi.Value[*oc.TerminalDevice_Channel_Otn_QValue]) bool {
			qValue, present := max.Val()
			return present && qValue.GetMin() == inactiveQValue && qValue.GetAvg() == inactiveQValue && qValue.GetMax() == inactiveQValue && qValue.GetInstant() == inactiveQValue
		}).Await(t)
		if !ok {
			t.Fatalf("QValue stats are not as expected for %v after %v minutes.", p.Name(), timeout.Minutes())
		}
	default:
		t.Fatalf("Unsupported oper status for %v: %v", p.Name(), operStatus)
	}
}

// validateNextSample validates the stream data.
func validateNextSample(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, params *cfgplugins.ConfigParameters, wantOperStatus oc.E_Interface_OperStatus, interfaceData *ygnmi.Value[*oc.Interface], otnChannelData, ethChannelData *ygnmi.Value[*oc.TerminalDevice_Channel], operationalMode uint16) {
	// Validate Interface OperStatus Telemetry.
	if interfaceData == nil {
		t.Errorf("Data not received for %v.", p.Name())
		return
	}
	interfaceValue, ok := interfaceData.Val()
	if !ok {
		t.Errorf("Channel data is empty for %v.", p.Name())
		return
	}
	gotOperStatus := interfaceValue.GetOperStatus()
	if gotOperStatus == oc.Interface_OperStatus_UNSET {
		t.Errorf("Link state data is empty for %v", p.Name())
		return
	}
	t.Run("Interface operStatus Validation", func(t *testing.T) {
		t.Logf("\nInterface operStatus of %v is %v\n\n", p.Name(), gotOperStatus)
		if diff := cmp.Diff(gotOperStatus, wantOperStatus); diff != "" {
			t.Errorf("Interface operStatus is not as expected, diff (-got +want):\n%s", diff)
		}
	})

	// Validate OTN Channel Telemetry.
	if otnChannelData == nil {
		t.Errorf("OTN Channel data is empty for %v.", p.Name())
		return
	}
	otnChannelValue, ok := otnChannelData.Val()
	if !ok {
		t.Errorf("OTN Channel value is empty for %v.", p.Name())
		return
	}
	validateOTNChannelTelemetry(t, dut, p, params, otnChannelValue, gotOperStatus)

	// Validate Ethernet Channel Telemetry.
	if ethChannelData == nil {
		t.Errorf("Ethernet Channel data is empty for %v.", p.Name())
		return
	}
	ethChannelValue, ok := ethChannelData.Val()
	if !ok {
		t.Errorf("Ethernet Channel value is empty for %v.", p.Name())
		return
	}
	validateEthernetChannelTelemetry(t, dut, p, params, ethChannelValue)
}

// validateOTNChannelTelemetry validates the OTN channel telemetry.
func validateOTNChannelTelemetry(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, params *cfgplugins.ConfigParameters, otnChannel *oc.TerminalDevice_Channel, operStatus oc.E_Interface_OperStatus) {
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
			got:  otnChannel.GetIndex(),
			want: params.OTNIndexes[p.Name()],
		},
		{
			desc: "OTN Description Validation",
			path: fmt.Sprintf(logicalChannelPath+"/state/description", params.OTNIndexes[p.Name()]),
			got:  otnChannel.GetDescription(),
			want: "OTN Logical Channel",
		},
		{
			desc: "OTN Logical Channel Type Validation",
			path: fmt.Sprintf(logicalChannelPath+"/state/logical-channel-type", params.OTNIndexes[p.Name()]),
			got:  otnChannel.GetLogicalChannelType(),
			want: oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_OTN,
		},
		{
			desc: "OTN Loopback Mode Validation",
			path: fmt.Sprintf(logicalChannelPath+"/state/loopback-mode", params.OTNIndexes[p.Name()]),
			got:  otnChannel.GetLoopbackMode(),
			want: oc.TerminalDevice_LoopbackModeType_NONE,
		},
		{
			desc: "OTN to Optical Channel Assignment Index Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/index", params.OTNIndexes[p.Name()], firstAssignmentIndex),
			got:  otnChannel.GetAssignment(firstAssignmentIndex).GetIndex(),
			want: uint32(firstAssignmentIndex),
		},
		{
			desc: "OTN to Optical Channel Assignment Optical Channel Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/optical-channel", params.OTNIndexes[p.Name()], firstAssignmentIndex),
			got:  otnChannel.GetAssignment(firstAssignmentIndex).GetOpticalChannel(),
			want: components.OpticalChannelComponentFromPort(t, dut, p),
		},
		{
			desc: "OTN to Optical Channel Assignment Description Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/description", params.OTNIndexes[p.Name()], firstAssignmentIndex),
			got:  otnChannel.GetAssignment(firstAssignmentIndex).GetDescription(),
			want: "OTN to Optical Channel",
		},
		{
			desc: "OTN to Optical Channel Assignment Allocation Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/allocation", params.OTNIndexes[p.Name()], firstAssignmentIndex),
			got:  otnChannel.GetAssignment(firstAssignmentIndex).GetAllocation(),
			want: params.Allocation,
		},
		{
			desc: "OTN to Optical Channel Assignment Type Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/assignment-type", params.OTNIndexes[p.Name()], firstAssignmentIndex),
			got:  otnChannel.GetAssignment(firstAssignmentIndex).GetAssignmentType(),
			want: oc.Assignment_AssignmentType_OPTICAL_CHANNEL,
		},
		{
			desc:       "Instant QValue Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/q-value/instant", params.OTNIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetQValue().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: otnChannel.GetOtn().GetQValue().GetMin() * (1 - errorTolerance),
			maxAllowed: otnChannel.GetOtn().GetQValue().GetMax() * (1 + errorTolerance),
		},
		{
			desc:       "Instant QValue Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/q-value/instant", params.OTNIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetQValue().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       inactiveQValue,
		},
		{
			desc:       "Average QValue Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/q-value/avg", params.OTNIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetQValue().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: otnChannel.GetOtn().GetQValue().GetMin() * (1 - errorTolerance),
			maxAllowed: otnChannel.GetOtn().GetQValue().GetMax() * (1 + errorTolerance),
		},
		{
			desc:       "Average QValue Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/q-value/avg", params.OTNIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetQValue().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       inactiveQValue,
		},
		{
			desc:       "Minimum QValue Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/q-value/min", params.OTNIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetQValue().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedQValue,
			maxAllowed: maxAllowedQValue,
		},
		{
			desc:       "Minimum QValue Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/q-value/min", params.OTNIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetQValue().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       inactiveQValue,
		},
		{
			desc:       "Maximum QValue Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/q-value/max", params.OTNIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetQValue().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedQValue,
			maxAllowed: maxAllowedQValue,
		},
		{
			desc:       "Maximum QValue Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/q-value/max", params.OTNIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetQValue().GetMax(),
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       inactiveQValue,
		},
		{
			desc:       "Instant ESNR Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/esnr/instant", params.OTNIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetEsnr().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: otnChannel.GetOtn().GetEsnr().GetMin() * (1 - errorTolerance),
			maxAllowed: otnChannel.GetOtn().GetEsnr().GetMax() * (1 + errorTolerance),
		},
		{
			desc:       "Instant ESNR Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/esnr/instant", params.OTNIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetEsnr().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       inactiveESNR,
		},
		{
			desc:       "Average ESNR Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/esnr/avg", params.OTNIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetEsnr().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: otnChannel.GetOtn().GetEsnr().GetMin() * (1 - errorTolerance),
			maxAllowed: otnChannel.GetOtn().GetEsnr().GetMax() * (1 + errorTolerance),
		},
		{
			desc:       "Average ESNR Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/esnr/avg", params.OTNIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetEsnr().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       inactiveESNR,
		},
		{
			desc:       "Minimum ESNR Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/esnr/min", params.OTNIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetEsnr().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedESNR,
			maxAllowed: maxAllowedESNR,
		},
		{
			desc:       "Minimum ESNR Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/esnr/min", params.OTNIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetEsnr().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       inactiveESNR,
		},
		{
			desc:       "Maximum ESNR Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/esnr/max", params.OTNIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetEsnr().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedESNR,
			maxAllowed: maxAllowedESNR,
		},
		{
			desc:       "Maximum ESNR Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/esnr/max", params.OTNIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetEsnr().GetMax(),
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       inactiveESNR,
		},
		{
			desc:       "Instant PreFECBER Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/pre-fec-ber/instant", params.OTNIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetPreFecBer().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: otnChannel.GetOtn().GetPreFecBer().GetMin() * (1 - errorTolerance),
			maxAllowed: otnChannel.GetOtn().GetPreFecBer().GetMax() * (1 + errorTolerance),
		},
		{
			desc:       "Instant PreFECBER Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/pre-fec-ber/instant", params.OTNIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetPreFecBer().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			oneOf:      inactivePreFECBER,
		},
		{
			desc:       "Average PreFECBER Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/pre-fec-ber/avg", params.OTNIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetPreFecBer().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: otnChannel.GetOtn().GetPreFecBer().GetMin() * (1 - errorTolerance),
			maxAllowed: otnChannel.GetOtn().GetPreFecBer().GetMax() * (1 + errorTolerance),
		},
		{
			desc:       "Average PreFECBER Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/pre-fec-ber/avg", params.OTNIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetPreFecBer().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			oneOf:      inactivePreFECBER,
		},
		{
			desc:       "Minimum PreFECBER Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/pre-fec-ber/min", params.OTNIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetPreFecBer().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedPreFECBER,
			maxAllowed: maxAllowedPreFECBER,
		},
		{
			desc:       "Minimum PreFECBER Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/pre-fec-ber/min", params.OTNIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetPreFecBer().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			oneOf:      inactivePreFECBER,
		},
		{
			desc:       "Maximum PreFECBER Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/pre-fec-ber/max", params.OTNIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetPreFecBer().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedPreFECBER,
			maxAllowed: maxAllowedPreFECBER,
		},
		{
			desc:       "Maximum PreFECBER Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/pre-fec-ber/max", params.OTNIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetPreFecBer().GetMax(),
			operStatus: oc.Interface_OperStatus_DOWN,
			oneOf:      inactivePreFECBER,
		},
		{
			desc:       "FEC Uncorrectable Block Count Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/fec-uncorrectable-blocks", params.OTNIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetFecUncorrectableBlocks(),
			operStatus: oc.Interface_OperStatus_UP,
			want:       uint64(0),
		},
	}
	if deviations.OTNToETHAssignment(dut) {
		tcs = append(tcs, []testcase{
			{
				desc: "OTN to Logical Channel Assignment Index Validation",
				path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/index", params.OTNIndexes[p.Name()], firstAssignmentIndex+1),
				got:  otnChannel.GetAssignment(firstAssignmentIndex + 1).GetIndex(),
				want: uint32(firstAssignmentIndex + 1),
			},
			{
				desc: "OTN to Logical Channel Assignment Optical Channel Validation",
				path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/logical-channel", params.OTNIndexes[p.Name()], firstAssignmentIndex+1),
				got:  otnChannel.GetAssignment(firstAssignmentIndex + 1).GetLogicalChannel(),
				want: params.ETHIndexes[p.Name()],
			},
			{
				desc: "OTN to Logical Channel Assignment Description Validation",
				path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/description", params.OTNIndexes[p.Name()], firstAssignmentIndex+1),
				got:  otnChannel.GetAssignment(firstAssignmentIndex + 1).GetDescription(),
				want: "OTN to ETH",
			},
			{
				desc: "OTN to Logical Channel Assignment Allocation Validation",
				path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/allocation", params.OTNIndexes[p.Name()], firstAssignmentIndex+1),
				got:  otnChannel.GetAssignment(firstAssignmentIndex + 1).GetAllocation(),
				want: params.Allocation,
			},
			{
				desc: "OTN to Logical Channel Assignment Type Validation",
				path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/assignment-type", params.OTNIndexes[p.Name()], firstAssignmentIndex+1),
				got:  otnChannel.GetAssignment(firstAssignmentIndex + 1).GetAssignmentType(),
				want: oc.Assignment_AssignmentType_LOGICAL_CHANNEL,
			},
		}...)
	}
	if !deviations.ChannelRateClassParametersUnsupported(dut) {
		tcs = append(tcs, testcase{
			desc: "Rate Class",
			path: fmt.Sprintf(logicalChannelPath+"/state/rate-class", params.OTNIndexes[p.Name()]),
			got:  otnChannel.GetRateClass(),
			want: params.RateClass,
		})
	}
	if !deviations.OTNChannelTribUnsupported(dut) {
		tcs = append(tcs, []testcase{
			{
				desc: "OTN Trib Protocol Validation",
				path: fmt.Sprintf(logicalChannelPath+"/state/trib-protocol", params.OTNIndexes[p.Name()]),
				got:  otnChannel.GetTribProtocol(),
				want: params.TribProtocol,
			},
			{
				desc: "OTN Admin State Validation",
				path: fmt.Sprintf(logicalChannelPath+"/state/admin-state", params.OTNIndexes[p.Name()]),
				got:  otnChannel.GetAdminState(),
				want: oc.TerminalDevice_AdminStateType_ENABLED,
			},
		}...)
	}
	for _, tc := range tcs {
		if tc.operStatus != oc.Interface_OperStatus_UNSET && tc.operStatus != operStatus {
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

// validateEthernetChannelTelemetry validates the ethernet channel telemetry.
func validateEthernetChannelTelemetry(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, params *cfgplugins.ConfigParameters, ethChannel *oc.TerminalDevice_Channel) {
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
			got:  ethChannel.GetIndex(),
			want: params.ETHIndexes[p.Name()],
		},
		{
			desc: "ETH Description Validation",
			path: fmt.Sprintf(logicalChannelPath+"/state/description", params.ETHIndexes[p.Name()]),
			got:  ethChannel.GetDescription(),
			want: "ETH Logical Channel",
		},
		{
			desc: "ETH Logical Channel Type Validation",
			path: fmt.Sprintf(logicalChannelPath+"/state/logical-channel-type", params.ETHIndexes[p.Name()]),
			got:  ethChannel.GetLogicalChannelType(),
			want: oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_ETHERNET,
		},
		{
			desc: "ETH Loopback Mode Validation",
			path: fmt.Sprintf(logicalChannelPath+"/state/loopback-mode", params.ETHIndexes[p.Name()]),
			got:  ethChannel.GetLoopbackMode(),
			want: oc.TerminalDevice_LoopbackModeType_NONE,
		},
		{
			desc: "ETH Assignment Index Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/index", params.ETHIndexes[p.Name()], assignmentIndex),
			got:  ethChannel.GetAssignment(assignmentIndex).GetIndex(),
			want: uint32(assignmentIndex),
		},
		{
			desc: "ETH Assignment Logical Channel Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/logical-channel", params.ETHIndexes[p.Name()], assignmentIndex),
			got:  ethChannel.GetAssignment(assignmentIndex).GetLogicalChannel(),
			want: params.OTNIndexes[p.Name()],
		},
		{
			desc: "ETH Assignment Allocation Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/allocation", params.ETHIndexes[p.Name()], assignmentIndex),
			got:  ethChannel.GetAssignment(assignmentIndex).GetAllocation(),
			want: params.Allocation,
		},
		{
			desc: "ETH Assignment Type Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/assignment-type", params.ETHIndexes[p.Name()], assignmentIndex),
			got:  ethChannel.GetAssignment(assignmentIndex).GetAssignmentType(),
			want: oc.Assignment_AssignmentType_LOGICAL_CHANNEL,
		},
	}
	if !deviations.ChannelRateClassParametersUnsupported(dut) {
		tcs = append(tcs, testcase{
			desc: "ETH Rate Class Validation",
			path: fmt.Sprintf(logicalChannelPath+"/state/rate-class", params.ETHIndexes[p.Name()]),
			got:  ethChannel.GetRateClass(),
			want: params.RateClass,
		})
	}
	if !deviations.OTNChannelTribUnsupported(dut) {
		tcs = append(tcs, []testcase{
			{
				desc: "ETH Trib Protocol Validation",
				path: fmt.Sprintf(logicalChannelPath+"/state/trib-protocol", params.ETHIndexes[p.Name()]),
				got:  ethChannel.GetTribProtocol(),
				want: params.TribProtocol,
			},
			{
				desc: "ETH Admin State Validation",
				path: fmt.Sprintf(logicalChannelPath+"/state/admin-state", params.ETHIndexes[p.Name()]),
				got:  ethChannel.GetAdminState(),
				want: oc.TerminalDevice_AdminStateType_ENABLED,
			},
		}...)
	}
	if !deviations.EthChannelIngressParametersUnsupported(dut) {
		tcs = append(tcs, []testcase{
			{
				desc: "ETH Ingress Interface Validation",
				path: fmt.Sprintf(logicalChannelPath+"/ingress/state/interface", params.ETHIndexes[p.Name()]),
				got:  ethChannel.GetIngress().GetInterface(),
				want: p.Name(),
			},
			{
				desc: "ETH Ingress Transceiver Validation",
				path: fmt.Sprintf(logicalChannelPath+"/ingress/state/transceiver", params.ETHIndexes[p.Name()]),
				got:  ethChannel.GetIngress().GetTransceiver(),
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
