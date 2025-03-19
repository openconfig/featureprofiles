package transceiver_otn_test

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/openconfig/featureprofiles/feature/cisco/performance"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/cisco/ha/utils"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	targetOutputPower    = -9
	frequency            = 193100000
	operational_mode     = 5003
	samplingInterval     = 10 * time.Second
	timeout              = 10 * time.Minute
	minAllowedQValue     = 7.0
	maxAllowedQValue     = 14.0
	minAllowedPreFECBER  = 1e-9
	maxAllowedPreFECBER  = 1e-2
	minAllowedPostFECBER = 0.0
	maxAllowedPostFECBER = 0.0
	minAllowedESNR       = 10.0
	maxAllowedESNR       = 25.0
	inactiveQValue       = 0.0
	inactivePreFECBER    = 0.0
	inactivePostFECBER   = 0.0
	inactiveESNR         = 0.0
	flapInterval         = 30 * time.Second
	otnIndex1            = uint32(4000)
	otnIndex2            = uint32(5000)
	ethernetIndex1       = uint32(40001)
	ethernetIndex2       = uint32(50001)
	otnIndexBase         = uint32(4000)
	ethernetIndexBase    = uint32(40000)
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// appendToTableIfNotNil appends a row to the provided table if the given value is not nil.
// If the value is nil, it logs an error using the testing framework.
//
// Parameters:
//   - t: *testing.T - The testing object used for logging errors.
//   - table: *tablewriter.Table - The table to which the row will be appended.
//   - portName: string - The name of the port being processed.
//   - leaf: string - The leaf identifier to be added to the table.
//   - value: interface{} - The value to be added to the table (must not be nil).
//   - errMsg: string - The error message format string (expects portName as a formatting argument).
//
// Behavior:
//   - If `value` is nil, logs an error with `t.Errorf` and does not append to the table.
//   - If `value` is not nil, appends a row to `table` with `portName`, `leaf`, and the formatted `value`.
//
// Example Usage:
//
//	appendToTableIfNotNil(t, table, "eth0", "Index", device.GetAssignment(1).Index, "Index is empty for port %v")
func appendToTableIfNotNil(t *testing.T, table *tablewriter.Table, portName, leaf string, value interface{}, errMsg string) {
	if value == nil {
		t.Errorf(errMsg, portName)
		return
	}

	var formattedValue string
	switch v := value.(type) {
	case *float64:
		formattedValue = fmt.Sprintf("%f", *v) // Dereference pointer
	case *float32:
		formattedValue = fmt.Sprintf("%f", *v) // Dereference pointer
	case *int:
		formattedValue = fmt.Sprintf("%d", *v) // Dereference pointer
	case *int64:
		formattedValue = fmt.Sprintf("%d", *v) // Dereference pointer
	case *int32:
		formattedValue = fmt.Sprintf("%d", *v) // Dereference pointer
	case *uint32:
		formattedValue = fmt.Sprintf("%d", *v) // Dereference pointer
	case *string:
		formattedValue = *v // Directly use the dereferenced string
	case float64, float32:
		formattedValue = fmt.Sprintf("%f", v) // Handle non-pointer floats
	case int, int64, int32, uint32:
		formattedValue = fmt.Sprintf("%d", v) // Handle non-pointer integers
	case string:
		formattedValue = v // Handle strings
	default:
		formattedValue = fmt.Sprintf("%v", v) // Default fallback for any other type
	}

	table.Append([]string{portName, leaf, formattedValue})
}

// validateSampleStream validates the operational status of an interface and a terminal device channel on a DUT (Device Under Test).
// It checks for the presence of various attributes and logs errors if any expected data is missing. The function also appends
// relevant data to a table for display.
//
// Parameters:
//   - t: The testing context used for logging errors.
//   - interfaceData: A ygnmi.Value containing the interface state data to be validated.
//   - terminalDeviceData: A ygnmi.Value containing the terminal device channel state data to be validated.
//   - portName: The name of the port whose data is being validated.
//
// Returns:
//   - The operational status of the interface. If any critical data is missing, oc.Interface_OperStatus_UNSET is returned.
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

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Transceiver", "Leaf", "Value"})

	appendToTableIfNotNil(t, table, portName, "Index", terminalDeviceValue.GetAssignment(1).Index, "Index is empty for port %v")

	appendToTableIfNotNil(t, table, portName, "Allocation", terminalDeviceValue.GetAssignment(1).Allocation, "Allocation is empty for port %v")

	appendToTableIfNotNil(t, table, portName, "assignmentType", terminalDeviceValue.GetAssignment(1).AssignmentType, "AssignmentType is empty for port %v")

	appendToTableIfNotNil(t, table, portName, "Description", terminalDeviceValue.GetAssignment(1).Description, "Description is empty for port %v")

	appendToTableIfNotNil(t, table, portName, "LogicalChannel", terminalDeviceValue.GetAssignment(1).LogicalChannel, "LogicalChannel is empty for port %v")

	appendToTableIfNotNil(t, table, portName, "OpticalChannel", terminalDeviceValue.GetAssignment(1).OpticalChannel, "OpticalChannel is empty for port %v")

	appendToTableIfNotNil(t, table, portName, "AdminState", terminalDeviceValue.AdminState, "AdminState is empty for port %v")

	appendToTableIfNotNil(t, table, portName, "Description", terminalDeviceValue.Description, "Description is empty for port %v")

	appendToTableIfNotNil(t, table, portName, "Index", terminalDeviceValue.Index, "Index is empty for port %v")

	appendToTableIfNotNil(t, table, portName, "LinkState", terminalDeviceValue.LinkState, "LinkState is empty for port %v")

	appendToTableIfNotNil(t, table, portName, "LogicalChannelType", terminalDeviceValue.LogicalChannelType, "LogicalChannelType is empty for port %v")

	appendToTableIfNotNil(t, table, portName, "LoopbackMode", terminalDeviceValue.LoopbackMode, "LoopbackMode is empty for port %v")

	otn := terminalDeviceValue.GetOtn()
	if otn == nil {
		t.Errorf("OTN data is empty for port %v", portName)
		return operStatus
	}
	if b := otn.GetPreFecBer(); b == nil {
		t.Errorf("PreFECBER data is empty for port %v", portName)
	} else {
		appendToTableIfNotNil(t, table, portName, "PreFECBER_instant", otn.GetPreFecBer().GetInstant(), "PreFECBER_instant is empty for port %v")
		appendToTableIfNotNil(t, table, portName, "PreFECBER_min", otn.GetPreFecBer().GetMin(), "PreFECBER_min is empty for port %v")
		appendToTableIfNotNil(t, table, portName, "PreFECBER_max", otn.GetPreFecBer().GetMax(), "PreFECBER_max is empty for port %v")
		appendToTableIfNotNil(t, table, portName, "PreFECBER_avg", otn.GetPreFecBer().GetAvg(), "PreFECBER_avg is empty for port %v")
		validatePMValue(t, portName, "PreFECBER", b.GetInstant(), b.GetMin(), b.GetMax(), b.GetAvg(), minAllowedPreFECBER, maxAllowedPreFECBER, inactivePreFECBER, operStatus)
	}

	if e := otn.GetEsnr(); e == nil {
		t.Errorf("ESNR data is empty for port %v", portName)
	} else {
		appendToTableIfNotNil(t, table, portName, "ESNR_instant", otn.GetEsnr().GetInstant(), "ESNR_instant is empty for port %v")
		appendToTableIfNotNil(t, table, portName, "ESNR_min", otn.GetEsnr().GetMin(), "ESNR_min is empty for port %v")
		appendToTableIfNotNil(t, table, portName, "ESNR_max", otn.GetEsnr().GetMax(), "ESNR_max is empty for port %v")
		appendToTableIfNotNil(t, table, portName, "ESNR_avg", otn.GetEsnr().GetAvg(), "ESNR_avg is empty for port %v")
		validatePMValue(t, portName, "esnr", e.GetInstant(), e.GetMin(), e.GetMax(), e.GetAvg(), minAllowedESNR, maxAllowedESNR, inactiveESNR, operStatus)
	}
	if q := otn.GetQValue(); q == nil {
		t.Errorf("QValue data is empty for port %v", portName)
	} else {
		appendToTableIfNotNil(t, table, portName, "QValue_instant", otn.GetQValue().GetInstant(), "QValue_instant is empty for port %v")
		appendToTableIfNotNil(t, table, portName, "QValue_min", otn.GetQValue().GetMin(), "QValue_min is empty for port %v")
		appendToTableIfNotNil(t, table, portName, "QValue_max", otn.GetQValue().GetMax(), "QValue_max is empty for port %v")
		appendToTableIfNotNil(t, table, portName, "QValue_avg", otn.GetQValue().GetAvg(), "QValue_avg is empty for port %v")
		validatePMValue(t, portName, "QValue", q.GetInstant(), q.GetMin(), q.GetMax(), q.GetAvg(), minAllowedQValue, maxAllowedQValue, inactiveQValue, operStatus)
	}
	if b := otn.GetPostFecBer(); b == nil {
		t.Errorf("PostFECBER data is empty for port %v", portName)
	} else {
		appendToTableIfNotNil(t, table, portName, "PostFECBER_instant", otn.GetPostFecBer().GetInstant(), "PostFECBER_instant is empty for port %v")
		appendToTableIfNotNil(t, table, portName, "PostFECBER_min", otn.GetPostFecBer().GetMin(), "PostFECBER_min is empty for port %v")
		appendToTableIfNotNil(t, table, portName, "PostFECBER_max", otn.GetPostFecBer().GetMax(), "PostFECBER_max is empty for port %v")
		appendToTableIfNotNil(t, table, portName, "PostFECBER_avg", otn.GetPostFecBer().GetAvg(), "PostFECBER_avg is empty for port %v")
		validatePMValue(t, portName, "PostFECBER", b.GetInstant(), b.GetMin(), b.GetMax(), b.GetAvg(), minAllowedPostFECBER, maxAllowedPostFECBER, inactivePostFECBER, operStatus)
	}
	table.Render()
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

func configureETHandOptChannel(t *testing.T, dut *ondatra.DUTDevice, och string, otnIndex, ethIndex uint32) {
	gnmi.Replace(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndex).Config(), &oc.TerminalDevice_Channel{
		Index:              ygot.Uint32(otnIndex),
		AdminState:         oc.TerminalDevice_AdminStateType_ENABLED,
		Description:        ygot.String("Coherent Logical Channel"),
		LoopbackMode:       oc.TerminalDevice_LoopbackModeType_TERMINAL,
		LogicalChannelType: oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_OTN,
		Assignment: map[uint32]*oc.TerminalDevice_Channel_Assignment{
			1: {
				Index:          ygot.Uint32(1),
				OpticalChannel: ygot.String(och),
				Description:    ygot.String("OTN to Optical Channel"),
				Allocation:     ygot.Float64(400),
				AssignmentType: oc.Assignment_AssignmentType_OPTICAL_CHANNEL,
			},
		},
	})
	gnmi.Replace(t, dut, gnmi.OC().TerminalDevice().Channel(ethIndex).Config(), &oc.TerminalDevice_Channel{
		Index:              ygot.Uint32(ethIndex),
		RateClass:          oc.TransportTypes_TRIBUTARY_RATE_CLASS_TYPE_TRIB_RATE_400G,
		TribProtocol:       oc.TransportTypes_TRIBUTARY_PROTOCOL_TYPE_PROT_400GE,
		AdminState:         oc.TerminalDevice_AdminStateType_ENABLED,
		Description:        ygot.String("ETH Logical Channel"),
		LoopbackMode:       oc.TerminalDevice_LoopbackModeType_TERMINAL,
		LogicalChannelType: oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_ETHERNET,
		Assignment: map[uint32]*oc.TerminalDevice_Channel_Assignment{
			1: {
				Index:          ygot.Uint32(1),
				LogicalChannel: ygot.Uint32(otnIndex),
				Description:    ygot.String("ETH to Coherent assignment"),
				Allocation:     ygot.Float64(400),
				AssignmentType: oc.Assignment_AssignmentType_LOGICAL_CHANNEL,
			},
		},
	})
}

var (
	otnIndexes = make(map[string]uint32)
	ethIndexes = make(map[string]uint32)
)

func configureOTN(t *testing.T, dut *ondatra.DUTDevice) {
	for i, p := range dut.Ports() {
		oc := components.OpticalChannelComponentFromPort(t, dut, p)
		cfgplugins.ConfigOpticalChannel(t, dut, oc, frequency, targetOutputPower, operational_mode)
		configureETHandOptChannel(t, dut, oc, otnIndexBase+uint32(i), ethernetIndexBase+uint32(i))
		otnIndexes[p.Name()] = otnIndexBase + uint32(i)
		ethIndexes[p.Name()] = ethernetIndexBase + uint32(i)
	}
}

// awaitPortsState waits for all DUT ports to reach the specified operational state (UP or DOWN).
// It checks each port's operational status using GNMI and waits until the expected state is observed
// or the timeout is reached. After waiting, it sleeps for 3 times the sampling interval.
//
// Parameters:
// - t: *testing.T - The test context.
// - dut: *Device - The device under test.
// - timeout: time.Duration - Maximum time to wait for each port to reach the expected state.
// - samplingInterval: time.Duration - Time interval between status checks.
// - expectedState: oc.Interface_OperStatus - The desired operational state (UP or DOWN).
func awaitPortsState(t *testing.T, dut *ondatra.DUTDevice, timeout, samplingInterval time.Duration, expectedState oc.E_Interface_OperStatus) {
	for _, p := range dut.Ports() {
		gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, expectedState)
	}
	time.Sleep(3 * samplingInterval)
}

// getLineCardFromPort retrieves the line card identifier from a given port on a DUT (Device Under Test).
//
// Parameters:
//   - t: *testing.T - The testing context.
//   - dut: *ondatra.DUTDevice - The device under test.
//   - port: string - The port identifier.
//
// Returns:
//   - string: The line card identifier associated with the specified port.
//
// The function uses a regular expression to extract the port details and then constructs a lookup key
// to find the corresponding line card from the list of components. If the port format is invalid or
// the line card is not found, the function will log an error and terminate the test.
func getLineCardFromPort(t *testing.T, dut *ondatra.DUTDevice, port string) string {
	lc := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD)
	var LC string
	// Using port which is used for shut/unshut
	re := regexp.MustCompile(`\d+/\d+/\d+/\d+`)
	matches := re.FindStringSubmatch(dut.Port(t, port).Name())

	if len(matches) > 0 {
		extractedKey := matches[0]
		subSplitKey := strings.Split(extractedKey, "/")
		if len(subSplitKey) < 3 {
			fmt.Println("Invalid key format after splitting on Optics")
			t.Fatal()
		}
		lookup := strings.Join(subSplitKey[:2], "/") + "/CPU0"
		for _, item := range lc {
			if strings.Contains(item, lookup) {
				LC = item
				break
			}
		}
	}
	return LC
}

func TestZRProcessRestart(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	configureOTN(t, dut)

	awaitPortsState(t, dut, timeout, samplingInterval, oc.Interface_OperStatus_UP)

	// Initiate sample streams
	otnStreams := make(map[string]*samplestream.SampleStream[*oc.TerminalDevice_Channel])
	interfaceStreams := make(map[string]*samplestream.SampleStream[*oc.Interface])
	for portName, otnIndex := range otnIndexes {
		otnStreams[portName] = samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndex).State(), samplingInterval)
		interfaceStreams[portName] = samplestream.New(t, dut, gnmi.OC().Interface(portName).State(), samplingInterval)
		otnStreams[portName].Close()
		interfaceStreams[portName].Close()
	}

	// Verify the leaves
	operstatus := make(map[string]oc.E_Interface_OperStatus)
	for port, stream := range otnStreams {
		operstatus[port] = validateSampleStream(t, interfaceStreams[port].Next(), stream.Next(), port)
	}

	// Do process restart
	err := performance.RestartProcess(t, dut, "invmgr")
	if err != nil {
		t.Fatal(err)
	}

	awaitPortsState(t, dut, timeout, samplingInterval, oc.Interface_OperStatus_UP)

	for port, stream := range otnStreams {
		operstatus[port] = validateSampleStream(t, interfaceStreams[port].Next(), stream.Next(), port)
	}

}

func TestZRShutPort(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	configureOTN(t, dut)

	// Wait for streaming telemetry to report the channels as up.
	awaitPortsState(t, dut, timeout, samplingInterval, oc.Interface_OperStatus_UP)

	// Initiate sample streams
	otnStreams := make(map[string]*samplestream.SampleStream[*oc.TerminalDevice_Channel])
	interfaceStreams := make(map[string]*samplestream.SampleStream[*oc.Interface])
	for portName, otnIndex := range otnIndexes {
		otnStreams[portName] = samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndex).State(), samplingInterval)
		interfaceStreams[portName] = samplestream.New(t, dut, gnmi.OC().Interface(portName).State(), samplingInterval)
		otnStreams[portName].Close()
		interfaceStreams[portName].Close()
	}

	// Verify the leaves
	operstatus := make(map[string]oc.E_Interface_OperStatus)
	for port, stream := range otnStreams {
		operstatus[port] = validateSampleStream(t, interfaceStreams[port].Next(), stream.Next(), port)
	}

	// Disable interface.
	for _, p := range dut.Ports() {
		cfgplugins.ToggleInterface(t, dut, p.Name(), false)
	}

	// Wait for streaming telemetry to report the channels as down.
	awaitPortsState(t, dut, timeout, samplingInterval, oc.Interface_OperStatus_DOWN)

	for port, stream := range otnStreams {
		operstatus[port] = validateSampleStream(t, interfaceStreams[port].Next(), stream.Next(), port)
	}

	// Re-enable transceivers.
	for _, p := range dut.Ports() {
		cfgplugins.ToggleInterface(t, dut, p.Name(), true)
	}

	// Wait for streaming telemetry to report the channels as up.
	awaitPortsState(t, dut, timeout, samplingInterval, oc.Interface_OperStatus_UP)

	for port, stream := range otnStreams {
		operstatus[port] = validateSampleStream(t, interfaceStreams[port].Next(), stream.Next(), port)
	}

}

func TestZRLCReload(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// enable transceivers.
	for _, p := range dut.Ports() {
		cfgplugins.ToggleInterface(t, dut, p.Name(), true)
	}
	configureOTN(t, dut)

	// Wait for streaming telemetry to report the channels as up.
	awaitPortsState(t, dut, timeout, samplingInterval, oc.Interface_OperStatus_UP)

	//Initiate sample streams
	otnStreams := make(map[string]*samplestream.SampleStream[*oc.TerminalDevice_Channel])
	interfaceStreams := make(map[string]*samplestream.SampleStream[*oc.Interface])
	for portName, otnIndex := range otnIndexes {
		otnStreams[portName] = samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndex).State(), samplingInterval)
		interfaceStreams[portName] = samplestream.New(t, dut, gnmi.OC().Interface(portName).State(), samplingInterval)
		otnStreams[portName].Close()
		interfaceStreams[portName].Close()
	}

	// Verify the leaves
	operstatus := make(map[string]oc.E_Interface_OperStatus)
	for port, stream := range otnStreams {
		operstatus[port] = validateSampleStream(t, interfaceStreams[port].Next(), stream.Next(), port)
	}

	LC := getLineCardFromPort(t, dut, "port1")

	t.Logf("Restarting LC %s", LC)
	util.ReloadLinecards(t, []string{LC})
	// Sleeping additional 5 mins
	time.Sleep(5 * time.Minute)

	// enable transceivers.
	for _, p := range dut.Ports() {
		cfgplugins.ToggleInterface(t, dut, p.Name(), true)
	}

	// Wait for streaming telemetry to report the channels as up.
	awaitPortsState(t, dut, timeout, samplingInterval, oc.Interface_OperStatus_UP)

	for port, stream := range otnStreams {
		operstatus[port] = validateSampleStream(t, interfaceStreams[port].Next(), stream.Next(), port)
	}
	t.Logf("All Gnmi leaves received successfully after LC Reload")

}

func TestZRRPFO(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	configureOTN(t, dut)

	// Wait for streaming telemetry to report the channels as up.
	awaitPortsState(t, dut, timeout, samplingInterval, oc.Interface_OperStatus_UP)

	// Initiate sample streams
	otnStreams := make(map[string]*samplestream.SampleStream[*oc.TerminalDevice_Channel])
	interfaceStreams := make(map[string]*samplestream.SampleStream[*oc.Interface])
	for portName, otnIndex := range otnIndexes {
		otnStreams[portName] = samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndex).State(), samplingInterval)
		interfaceStreams[portName] = samplestream.New(t, dut, gnmi.OC().Interface(portName).State(), samplingInterval)
		otnStreams[portName].Close()
		interfaceStreams[portName].Close()
	}

	// Verify the leaves
	operstatus := make(map[string]oc.E_Interface_OperStatus)
	for port, stream := range otnStreams {
		operstatus[port] = validateSampleStream(t, interfaceStreams[port].Next(), stream.Next(), port)
	}

	// Do RPFO
	utils.Dorpfo(context.Background(), t, true)

	// Wait for streaming telemetry to report the channels as up.
	awaitPortsState(t, dut, timeout, samplingInterval, oc.Interface_OperStatus_UP)

	for port, stream := range otnStreams {
		operstatus[port] = validateSampleStream(t, interfaceStreams[port].Next(), stream.Next(), port)
	}
}
