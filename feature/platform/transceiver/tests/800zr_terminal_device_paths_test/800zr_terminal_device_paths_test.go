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
	allocation             float64
	tribProtocol           oc.E_TransportTypes_TRIBUTARY_PROTOCOL_TYPE
	rateClass              oc.E_TransportTypes_TRIBUTARY_RATE_CLASS_TYPE
	ethIndexes             map[string]uint32
	otnIndexes             map[string]uint32
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
				batch := &gnmi.SetBatch{}
				cfgplugins.NewInterfaceConfigAll(t, dut, batch, frequency, targetOpticalPower, operationalMode)
				batch.Set(t, dut)

				populateValidationVariables(t, dut, operationalMode)

				// Create sample steams for each port.
				ethStreams := make(map[string]*samplestream.SampleStream[*oc.TerminalDevice_Channel])
				otnStreams := make(map[string]*samplestream.SampleStream[*oc.TerminalDevice_Channel])
				interfaceStreams := make(map[string]*samplestream.SampleStream[*oc.Interface])
				for _, p := range dut.Ports() {
					ethStreams[p.Name()] = samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(ethIndexes[p.Name()]).State(), samplingInterval)
					otnStreams[p.Name()] = samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndexes[p.Name()]).State(), samplingInterval)
					interfaceStreams[p.Name()] = samplestream.New(t, dut, gnmi.OC().Interface(p.Name()).State(), samplingInterval)
					defer ethStreams[p.Name()].Close()
					defer otnStreams[p.Name()].Close()
					defer interfaceStreams[p.Name()].Close()
				}

				// Wait for streaming telemetry to report the channels as up.
				for _, p := range dut.Ports() {
					gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
					awaitQValueStats(t, dut, p, oc.Interface_OperStatus_UP)
				}
				time.Sleep(3 * samplingInterval) // Wait extra time for telemetry to be updated.
				for _, p := range dut.Ports() {
					validateNextSample(t, dut, p, oc.Interface_OperStatus_UP, interfaceStreams[p.Name()].Next(), otnStreams[p.Name()].Next(), ethStreams[p.Name()].Next(), operationalMode)
				}

				t.Logf("\n*** Bringing DOWN all interfaces\n\n\n")
				for _, p := range dut.Ports() {
					cfgplugins.ToggleInterfaceState(t, p, false, operationalMode)
				}

				// Wait for streaming telemetry to report the channels as down and validate stats updated.
				for _, p := range dut.Ports() {
					gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_DOWN)
					awaitQValueStats(t, dut, p, oc.Interface_OperStatus_DOWN)
				}
				time.Sleep(3 * samplingInterval) // Wait extra time for telemetry to be updated.
				for _, p := range dut.Ports() {
					validateNextSample(t, dut, p, oc.Interface_OperStatus_DOWN, interfaceStreams[p.Name()].Next(), otnStreams[p.Name()].Next(), ethStreams[p.Name()].Next(), operationalMode)
				}

				t.Logf("\n*** Bringing UP all interfaces\n\n\n")
				for _, p := range dut.Ports() {
					cfgplugins.ToggleInterfaceState(t, p, true, operationalMode)
				}

				// Wait for streaming telemetry to report the channels as up and validate stats updated.
				for _, p := range dut.Ports() {
					gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
					awaitQValueStats(t, dut, p, oc.Interface_OperStatus_UP)
				}
				time.Sleep(3 * samplingInterval) // Wait extra time for telemetry to be updated.
				for _, p := range dut.Ports() {
					validateNextSample(t, dut, p, oc.Interface_OperStatus_UP, interfaceStreams[p.Name()].Next(), otnStreams[p.Name()].Next(), ethStreams[p.Name()].Next(), operationalMode)
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
	ethIndexes = cfgplugins.AssignETHIndexes(t, dut)
	otnIndexes = cfgplugins.AssignOTNIndexes(t, dut)
	pmd := dut.Ports()[0].PMD()
	switch pmd {
	case ondatra.PMD400GBASEZR, ondatra.PMD400GBASEZRP:
		allocation = 400
		tribProtocol = oc.TransportTypes_TRIBUTARY_PROTOCOL_TYPE_PROT_400GE
		rateClass = oc.TransportTypes_TRIBUTARY_RATE_CLASS_TYPE_TRIB_RATE_400G
	case ondatra.PMD800GBASEZR, ondatra.PMD800GBASEZRP:
		switch operationalMode {
		case 1, 2:
			allocation = 800
			tribProtocol = oc.TransportTypes_TRIBUTARY_PROTOCOL_TYPE_PROT_800GE
			rateClass = oc.TransportTypes_TRIBUTARY_RATE_CLASS_TYPE_TRIB_RATE_800G
		case 3, 4:
			allocation = 400
			tribProtocol = oc.TransportTypes_TRIBUTARY_PROTOCOL_TYPE_PROT_400GE
			rateClass = oc.TransportTypes_TRIBUTARY_RATE_CLASS_TYPE_TRIB_RATE_400G
		default:
			t.Fatalf("Unsupported operational mode for %v: %v", pmd, operationalMode)
		}
	default:
		t.Fatalf("Unsupported PMD type for %v", pmd)
	}
}

// awaitQValueStats waits for the QValue stats (i.e., min/max/avg) to be within the expected range.
func awaitQValueStats(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, operStatus oc.E_Interface_OperStatus) {
	if p.PMD() == ondatra.PMD800GBASEZR || p.PMD() == ondatra.PMD800GBASEZRP {
		// Skip the QValue stats validation for 800ZR and 800ZR Plus due to OC stats bug for logical channels.
		t.Logf("Skipping the QValue stats validation for 800ZR and 800ZR Plus due to OC stats bug for logical channels.")
		time.Sleep(1 * time.Minute)
		return
	}
	switch operStatus {
	case oc.Interface_OperStatus_UP:
		_, ok := gnmi.Watch(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndexes[p.Name()]).Otn().QValue().State(), timeout, func(min *ygnmi.Value[*oc.TerminalDevice_Channel_Otn_QValue]) bool {
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
		_, ok := gnmi.Watch(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndexes[p.Name()]).Otn().QValue().State(), timeout, func(max *ygnmi.Value[*oc.TerminalDevice_Channel_Otn_QValue]) bool {
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
func validateNextSample(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, wantOperStatus oc.E_Interface_OperStatus, interfaceData *ygnmi.Value[*oc.Interface], otnChannelData, ethChannelData *ygnmi.Value[*oc.TerminalDevice_Channel], operationalMode uint16) {
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
	validateOTNChannelTelemetry(t, dut, p, otnChannelValue, gotOperStatus)

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
	validateEthernetChannelTelemetry(t, dut, p, ethChannelValue)
}

// validateOTNChannelTelemetry validates the OTN channel telemetry.
func validateOTNChannelTelemetry(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, otnChannel *oc.TerminalDevice_Channel, operStatus oc.E_Interface_OperStatus) {
	var firstAssignmentIndex uint32
	if deviations.OTNChannelAssignmentCiscoNumbering(dut) {
		firstAssignmentIndex = 1
	} else {
		firstAssignmentIndex = 0
	}
	tcs := []testcase{
		{
			desc: "OTN Index Validation",
			path: fmt.Sprintf(logicalChannelPath+"/state/index", otnIndexes[p.Name()]),
			got:  otnChannel.GetIndex(),
			want: otnIndexes[p.Name()],
		},
		{
			desc: "OTN Description Validation",
			path: fmt.Sprintf(logicalChannelPath+"/state/description", otnIndexes[p.Name()]),
			got:  otnChannel.GetDescription(),
			want: "OTN Logical Channel",
		},
		{
			desc: "OTN Logical Channel Type Validation",
			path: fmt.Sprintf(logicalChannelPath+"/state/logical-channel-type", otnIndexes[p.Name()]),
			got:  otnChannel.GetLogicalChannelType(),
			want: oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_OTN,
		},
		{
			desc: "OTN Loopback Mode Validation",
			path: fmt.Sprintf(logicalChannelPath+"/state/loopback-mode", otnIndexes[p.Name()]),
			got:  otnChannel.GetLoopbackMode(),
			want: oc.TerminalDevice_LoopbackModeType_NONE,
		},
		{
			desc: "OTN to Optical Channel Assignment Index Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/index", otnIndexes[p.Name()], firstAssignmentIndex),
			got:  otnChannel.GetAssignment(firstAssignmentIndex).GetIndex(),
			want: uint32(firstAssignmentIndex),
		},
		{
			desc: "OTN to Optical Channel Assignment Optical Channel Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/optical-channel", otnIndexes[p.Name()], firstAssignmentIndex),
			got:  otnChannel.GetAssignment(firstAssignmentIndex).GetOpticalChannel(),
			want: components.OpticalChannelComponentFromPort(t, dut, p),
		},
		{
			desc: "OTN to Optical Channel Assignment Description Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/description", otnIndexes[p.Name()], firstAssignmentIndex),
			got:  otnChannel.GetAssignment(firstAssignmentIndex).GetDescription(),
			want: "OTN to Optical Channel",
		},
		{
			desc: "OTN to Optical Channel Assignment Allocation Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/allocation", otnIndexes[p.Name()], firstAssignmentIndex),
			got:  otnChannel.GetAssignment(firstAssignmentIndex).GetAllocation(),
			want: allocation,
		},
		{
			desc: "OTN to Optical Channel Assignment Type Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/assignment-type", otnIndexes[p.Name()], firstAssignmentIndex),
			got:  otnChannel.GetAssignment(firstAssignmentIndex).GetAssignmentType(),
			want: oc.Assignment_AssignmentType_OPTICAL_CHANNEL,
		},
		{
			desc:       "Instant QValue Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/q-value/instant", otnIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetQValue().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: otnChannel.GetOtn().GetQValue().GetMin() * (1 - errorTolerance),
			maxAllowed: otnChannel.GetOtn().GetQValue().GetMax() * (1 + errorTolerance),
		},
		{
			desc:       "Instant QValue Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/q-value/instant", otnIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetQValue().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       inactiveQValue,
		},
		{
			desc:       "Average QValue Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/q-value/avg", otnIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetQValue().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: otnChannel.GetOtn().GetQValue().GetMin() * (1 - errorTolerance),
			maxAllowed: otnChannel.GetOtn().GetQValue().GetMax() * (1 + errorTolerance),
		},
		{
			desc:       "Average QValue Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/q-value/avg", otnIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetQValue().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       inactiveQValue,
		},
		{
			desc:       "Minimum QValue Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/q-value/min", otnIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetQValue().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedQValue,
			maxAllowed: maxAllowedQValue,
		},
		{
			desc:       "Minimum QValue Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/q-value/min", otnIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetQValue().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       inactiveQValue,
		},
		{
			desc:       "Maximum QValue Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/q-value/max", otnIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetQValue().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedQValue,
			maxAllowed: maxAllowedQValue,
		},
		{
			desc:       "Maximum QValue Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/q-value/max", otnIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetQValue().GetMax(),
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       inactiveQValue,
		},
		{
			desc:       "Instant ESNR Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/esnr/instant", otnIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetEsnr().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: otnChannel.GetOtn().GetEsnr().GetMin() * (1 - errorTolerance),
			maxAllowed: otnChannel.GetOtn().GetEsnr().GetMax() * (1 + errorTolerance),
		},
		{
			desc:       "Instant ESNR Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/esnr/instant", otnIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetEsnr().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       inactiveESNR,
		},
		{
			desc:       "Average ESNR Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/esnr/avg", otnIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetEsnr().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: otnChannel.GetOtn().GetEsnr().GetMin() * (1 - errorTolerance),
			maxAllowed: otnChannel.GetOtn().GetEsnr().GetMax() * (1 + errorTolerance),
		},
		{
			desc:       "Average ESNR Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/esnr/avg", otnIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetEsnr().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       inactiveESNR,
		},
		{
			desc:       "Minimum ESNR Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/esnr/min", otnIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetEsnr().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedESNR,
			maxAllowed: maxAllowedESNR,
		},
		{
			desc:       "Minimum ESNR Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/esnr/min", otnIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetEsnr().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       inactiveESNR,
		},
		{
			desc:       "Maximum ESNR Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/esnr/max", otnIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetEsnr().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedESNR,
			maxAllowed: maxAllowedESNR,
		},
		{
			desc:       "Maximum ESNR Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/esnr/max", otnIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetEsnr().GetMax(),
			operStatus: oc.Interface_OperStatus_DOWN,
			want:       inactiveESNR,
		},
		{
			desc:       "Instant PreFECBER Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/pre-fec-ber/instant", otnIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetPreFecBer().GetInstant(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: otnChannel.GetOtn().GetPreFecBer().GetMin() * (1 - errorTolerance),
			maxAllowed: otnChannel.GetOtn().GetPreFecBer().GetMax() * (1 + errorTolerance),
		},
		{
			desc:       "Instant PreFECBER Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/pre-fec-ber/instant", otnIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetPreFecBer().GetInstant(),
			operStatus: oc.Interface_OperStatus_DOWN,
			oneOf:      inactivePreFECBER,
		},
		{
			desc:       "Average PreFECBER Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/pre-fec-ber/avg", otnIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetPreFecBer().GetAvg(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: otnChannel.GetOtn().GetPreFecBer().GetMin() * (1 - errorTolerance),
			maxAllowed: otnChannel.GetOtn().GetPreFecBer().GetMax() * (1 + errorTolerance),
		},
		{
			desc:       "Average PreFECBER Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/pre-fec-ber/avg", otnIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetPreFecBer().GetAvg(),
			operStatus: oc.Interface_OperStatus_DOWN,
			oneOf:      inactivePreFECBER,
		},
		{
			desc:       "Minimum PreFECBER Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/pre-fec-ber/min", otnIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetPreFecBer().GetMin(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedPreFECBER,
			maxAllowed: maxAllowedPreFECBER,
		},
		{
			desc:       "Minimum PreFECBER Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/pre-fec-ber/min", otnIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetPreFecBer().GetMin(),
			operStatus: oc.Interface_OperStatus_DOWN,
			oneOf:      inactivePreFECBER,
		},
		{
			desc:       "Maximum PreFECBER Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/pre-fec-ber/max", otnIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetPreFecBer().GetMax(),
			operStatus: oc.Interface_OperStatus_UP,
			minAllowed: minAllowedPreFECBER,
			maxAllowed: maxAllowedPreFECBER,
		},
		{
			desc:       "Maximum PreFECBER Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/pre-fec-ber/max", otnIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetPreFecBer().GetMax(),
			operStatus: oc.Interface_OperStatus_DOWN,
			oneOf:      inactivePreFECBER,
		},
		{
			desc:       "FEC Uncorrectable Block Count Validation",
			path:       fmt.Sprintf(logicalChannelPath+"/otn/state/fec-uncorrectable-blocks", otnIndexes[p.Name()]),
			got:        otnChannel.GetOtn().GetFecUncorrectableBlocks(),
			operStatus: oc.Interface_OperStatus_UP,
			want:       uint64(0),
		},
	}
	if deviations.OTNToETHAssignment(dut) {
		tcs = append(tcs, []testcase{
			{
				desc: "OTN to Logical Channel Assignment Index Validation",
				path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/index", otnIndexes[p.Name()], firstAssignmentIndex+1),
				got:  otnChannel.GetAssignment(firstAssignmentIndex + 1).GetIndex(),
				want: uint32(firstAssignmentIndex + 1),
			},
			{
				desc: "OTN to Logical Channel Assignment Optical Channel Validation",
				path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/logical-channel", otnIndexes[p.Name()], firstAssignmentIndex+1),
				got:  otnChannel.GetAssignment(firstAssignmentIndex + 1).GetLogicalChannel(),
				want: ethIndexes[p.Name()],
			},
			{
				desc: "OTN to Logical Channel Assignment Description Validation",
				path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/description", otnIndexes[p.Name()], firstAssignmentIndex+1),
				got:  otnChannel.GetAssignment(firstAssignmentIndex + 1).GetDescription(),
				want: "OTN to ETH",
			},
			{
				desc: "OTN to Logical Channel Assignment Allocation Validation",
				path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/allocation", otnIndexes[p.Name()], firstAssignmentIndex+1),
				got:  otnChannel.GetAssignment(firstAssignmentIndex + 1).GetAllocation(),
				want: allocation,
			},
			{
				desc: "OTN to Logical Channel Assignment Type Validation",
				path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/assignment-type", otnIndexes[p.Name()], firstAssignmentIndex+1),
				got:  otnChannel.GetAssignment(firstAssignmentIndex + 1).GetAssignmentType(),
				want: oc.Assignment_AssignmentType_LOGICAL_CHANNEL,
			},
		}...)
	}
	if !deviations.ChannelRateClassParametersUnsupported(dut) {
		tcs = append(tcs, testcase{
			desc: "Rate Class",
			path: fmt.Sprintf(logicalChannelPath+"/state/rate-class", otnIndexes[p.Name()]),
			got:  otnChannel.GetRateClass(),
			want: rateClass,
		})
	}
	if !deviations.OTNChannelTribUnsupported(dut) {
		tcs = append(tcs, []testcase{
			{
				desc: "OTN Trib Protocol Validation",
				path: fmt.Sprintf(logicalChannelPath+"/state/trib-protocol", otnIndexes[p.Name()]),
				got:  otnChannel.GetTribProtocol(),
				want: tribProtocol,
			},
			{
				desc: "OTN Admin State Validation",
				path: fmt.Sprintf(logicalChannelPath+"/state/admin-state", otnIndexes[p.Name()]),
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
func validateEthernetChannelTelemetry(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, ethChannel *oc.TerminalDevice_Channel) {
	var assignmentIndex uint32
	if deviations.EthChannelAssignmentCiscoNumbering(dut) {
		assignmentIndex = 1
	} else {
		assignmentIndex = 0
	}
	tcs := []testcase{
		{
			desc: "ETH Index Validation",
			path: fmt.Sprintf(logicalChannelPath+"/state/index", ethIndexes[p.Name()]),
			got:  ethChannel.GetIndex(),
			want: ethIndexes[p.Name()],
		},
		{
			desc: "ETH Description Validation",
			path: fmt.Sprintf(logicalChannelPath+"/state/description", ethIndexes[p.Name()]),
			got:  ethChannel.GetDescription(),
			want: "ETH Logical Channel",
		},
		{
			desc: "ETH Logical Channel Type Validation",
			path: fmt.Sprintf(logicalChannelPath+"/state/logical-channel-type", ethIndexes[p.Name()]),
			got:  ethChannel.GetLogicalChannelType(),
			want: oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_ETHERNET,
		},
		{
			desc: "ETH Loopback Mode Validation",
			path: fmt.Sprintf(logicalChannelPath+"/state/loopback-mode", ethIndexes[p.Name()]),
			got:  ethChannel.GetLoopbackMode(),
			want: oc.TerminalDevice_LoopbackModeType_NONE,
		},
		{
			desc: "ETH Assignment Index Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/index", ethIndexes[p.Name()], assignmentIndex),
			got:  ethChannel.GetAssignment(assignmentIndex).GetIndex(),
			want: uint32(assignmentIndex),
		},
		{
			desc: "ETH Assignment Logical Channel Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/logical-channel", ethIndexes[p.Name()], assignmentIndex),
			got:  ethChannel.GetAssignment(assignmentIndex).GetLogicalChannel(),
			want: otnIndexes[p.Name()],
		},
		{
			desc: "ETH Assignment Allocation Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/allocation", ethIndexes[p.Name()], assignmentIndex),
			got:  ethChannel.GetAssignment(assignmentIndex).GetAllocation(),
			want: allocation,
		},
		{
			desc: "ETH Assignment Type Validation",
			path: fmt.Sprintf(logicalChannelPath+"/logical-channel-assignments/assignment[index=%v]/state/assignment-type", ethIndexes[p.Name()], assignmentIndex),
			got:  ethChannel.GetAssignment(assignmentIndex).GetAssignmentType(),
			want: oc.Assignment_AssignmentType_LOGICAL_CHANNEL,
		},
	}
	if !deviations.ChannelRateClassParametersUnsupported(dut) {
		tcs = append(tcs, testcase{
			desc: "ETH Rate Class Validation",
			path: fmt.Sprintf(logicalChannelPath+"/state/rate-class", ethIndexes[p.Name()]),
			got:  ethChannel.GetRateClass(),
			want: rateClass,
		})
	}
	if !deviations.OTNChannelTribUnsupported(dut) {
		tcs = append(tcs, []testcase{
			{
				desc: "ETH Trib Protocol Validation",
				path: fmt.Sprintf(logicalChannelPath+"/state/trib-protocol", ethIndexes[p.Name()]),
				got:  ethChannel.GetTribProtocol(),
				want: tribProtocol,
			},
			{
				desc: "ETH Admin State Validation",
				path: fmt.Sprintf(logicalChannelPath+"/state/admin-state", ethIndexes[p.Name()]),
				got:  ethChannel.GetAdminState(),
				want: oc.TerminalDevice_AdminStateType_ENABLED,
			},
		}...)
	}
	if !deviations.EthChannelIngressParametersUnsupported(dut) {
		tcs = append(tcs, []testcase{
			{
				desc: "ETH Ingress Interface Validation",
				path: fmt.Sprintf(logicalChannelPath+"/ingress/state/interface", ethIndexes[p.Name()]),
				got:  ethChannel.GetIngress().GetInterface(),
				want: p.Name(),
			},
			{
				desc: "ETH Ingress Transceiver Validation",
				path: fmt.Sprintf(logicalChannelPath+"/ingress/state/transceiver", ethIndexes[p.Name()]),
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
