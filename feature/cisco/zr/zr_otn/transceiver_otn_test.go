package transceiver_otn_test

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
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

func findComponentsByTypeNoLogs(t *testing.T, dut *ondatra.DUTDevice, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT) []string {
	components := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())
	var s []string
	for _, c := range components {
		if c.GetType() == nil {
			continue
		}
		switch v := c.GetType().(type) {
		case oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT:
			if v == cType {
				s = append(s, c.GetName())
			}
		}
	}
	return s
}

// validateSampleStream validates the stream data.
func validateSampleStream(t *testing.T, dut *ondatra.DUTDevice, interfaceData *ygnmi.Value[*oc.Interface], terminalDeviceData *ygnmi.Value[*oc.TerminalDevice_Channel], portName string) oc.E_Interface_OperStatus {
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

	if index := terminalDeviceValue.GetAssignment(1).Index; index == nil {
		t.Errorf("Index is empty for port %v", portName)
	} else {
		table.Append([]string{portName, "Index", strconv.FormatUint(uint64(*index), 10)})
	}
	if allocation := terminalDeviceValue.GetAssignment(1).Allocation; allocation == nil {
		t.Errorf("Allocation is empty for port %v", portName)
	} else {
		table.Append([]string{portName, "Allocation", strconv.FormatFloat(*allocation, 'f', 2, 64)})
	}
	if assignmenttype := terminalDeviceValue.GetAssignment(1).AssignmentType; assignmenttype == oc.E_Assignment_AssignmentType(0) {
		t.Errorf("AssignmentType is empty for port %v", portName)
	} else {
		table.Append([]string{portName, "AssignmentType", assignmenttype.String()})
	}
	if description := terminalDeviceValue.GetAssignment(1).Description; description == nil {
		t.Errorf("Description is empty for port %v", portName)
	} else {
		table.Append([]string{portName, "Description", *description})
	}
	if logicalchannel := terminalDeviceValue.GetAssignment(1).LogicalChannel; logicalchannel == nil {
		t.Errorf("LogicalChannel is empty for port %v", portName)
	} else {
		table.Append([]string{portName, "LogicalChannel", strconv.FormatUint(uint64(*logicalchannel), 10)})
	}
	if opticalchannel := terminalDeviceValue.GetAssignment(1).OpticalChannel; opticalchannel == nil {
		t.Errorf("OpticalChannel is empty for port %v", portName)
	} else {
		table.Append([]string{portName, "OpticalChannel", *opticalchannel})
	}
	if adminstate := terminalDeviceValue.AdminState; adminstate == oc.E_TerminalDevice_AdminStateType(0) {
		t.Errorf("Admin state is empty for port %v", portName)
	} else {
		table.Append([]string{portName, "AdminState", adminstate.String()})
	}
	if description := terminalDeviceValue.Description; description == nil {
		t.Errorf("description is empty for port %v", portName)
	} else {
		table.Append([]string{portName, "description", *description})
	}
	if index := terminalDeviceValue.Index; index == nil {
		t.Errorf("Index is empty for port %v", portName)
	} else {
		table.Append([]string{portName, "Index", strconv.FormatUint(uint64(*index), 10)})
	}
	if linkstate := terminalDeviceValue.LinkState; linkstate == oc.E_Channel_LinkState(0) {
		t.Errorf("LinkState is empty for port %v", portName)
	} else {
		table.Append([]string{portName, "LinkState", linkstate.String()})
	}
	if logicalchanneltype := terminalDeviceValue.LogicalChannelType; logicalchanneltype == oc.E_TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE(0) {
		t.Errorf("LogicalChannelType is empty for port %v", portName)
	} else {
		table.Append([]string{portName, "LogicalChannelType", logicalchanneltype.String()})
	}
	if loopbackmode := terminalDeviceValue.LoopbackMode; loopbackmode == oc.E_TerminalDevice_LoopbackModeType(0) {
		t.Errorf("LoopbackMode is empty for port %v", portName)
	} else {
		table.Append([]string{portName, "LoopbackMode", loopbackmode.String()})
	}

	otn := terminalDeviceValue.GetOtn()
	if otn == nil {
		t.Errorf("OTN data is empty for port %v", portName)
		return operStatus
	}
	if b := otn.GetPreFecBer(); b == nil {
		t.Errorf("PreFECBER data is empty for port %v", portName)
	} else {
		table.Append([]string{portName, "PreFECBER_instant", strconv.FormatFloat(b.GetInstant(), 'f', -1, 64)})
		table.Append([]string{portName, "PreFECBER_min", strconv.FormatFloat(b.GetMin(), 'f', -1, 64)})
		table.Append([]string{portName, "PreFECBER_max", strconv.FormatFloat(b.GetMax(), 'f', -1, 64)})
		table.Append([]string{portName, "PreFECBER_avg", strconv.FormatFloat(b.GetAvg(), 'f', -1, 64)})
		validatePMValue(t, portName, "PreFECBER", b.GetInstant(), b.GetMin(), b.GetMax(), b.GetAvg(), minAllowedPreFECBER, maxAllowedPreFECBER, inactivePreFECBER, operStatus)
	}
	if e := otn.GetEsnr(); e == nil {
		t.Errorf("ESNR data is empty for port %v", portName)
	} else {
		table.Append([]string{portName, "ESNR_instant", strconv.FormatFloat(e.GetInstant(), 'f', -1, 64)})
		table.Append([]string{portName, "ESNR_min", strconv.FormatFloat(e.GetMin(), 'f', -1, 64)})
		table.Append([]string{portName, "ESNR_max", strconv.FormatFloat(e.GetMax(), 'f', -1, 64)})
		table.Append([]string{portName, "ESNR_avg", strconv.FormatFloat(e.GetAvg(), 'f', -1, 64)})
		validatePMValue(t, portName, "esnr", e.GetInstant(), e.GetMin(), e.GetMax(), e.GetAvg(), minAllowedESNR, maxAllowedESNR, inactiveESNR, operStatus)
	}
	if q := otn.GetQValue(); q == nil {
		t.Errorf("QValue data is empty for port %v", portName)
	} else {
		table.Append([]string{portName, "QValue_instant", strconv.FormatFloat(q.GetInstant(), 'f', -1, 64)})
		table.Append([]string{portName, "QValue_min", strconv.FormatFloat(q.GetMin(), 'f', -1, 64)})
		table.Append([]string{portName, "QValue_max", strconv.FormatFloat(q.GetMax(), 'f', -1, 64)})
		table.Append([]string{portName, "QValue_avg", strconv.FormatFloat(q.GetAvg(), 'f', -1, 64)})
		validatePMValue(t, portName, "QValue", q.GetInstant(), q.GetMin(), q.GetMax(), q.GetAvg(), minAllowedQValue, maxAllowedQValue, inactiveQValue, operStatus)
	}
	if b := otn.GetPostFecBer(); b == nil {
		t.Errorf("PostFECBER data is empty for port %v", portName)
	} else {
		table.Append([]string{portName, "PostFECBER_instant", strconv.FormatFloat(b.GetInstant(), 'f', -1, 64)})
		table.Append([]string{portName, "PostFECBER_min", strconv.FormatFloat(b.GetMin(), 'f', -1, 64)})
		table.Append([]string{portName, "PostFECBER_max", strconv.FormatFloat(b.GetMax(), 'f', -1, 64)})
		table.Append([]string{portName, "PostFECBER_avg", strconv.FormatFloat(b.GetAvg(), 'f', -1, 64)})
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

func TestZRProcessRestart(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	configureOTN(t, dut)

	// Wait for streaming telemetry to report the channels as up.
	for _, p := range dut.Ports() {
		gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
	}
	time.Sleep(3 * samplingInterval) // Wait an extra sample interval to ensure the device has time to process the change.

	//Initiate sample streams
	otnStreams := make(map[string]*samplestream.SampleStream[*oc.TerminalDevice_Channel])
	interfaceStreams := make(map[string]*samplestream.SampleStream[*oc.Interface])
	for portName, otnIndex := range otnIndexes {
		otnStreams[portName] = samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndex).State(), samplingInterval)
		interfaceStreams[portName] = samplestream.New(t, dut, gnmi.OC().Interface(portName).State(), samplingInterval)
		defer otnStreams[portName].Close()
		defer interfaceStreams[portName].Close()
	}

	//Verify the leaves
	operstatus := make(map[string]oc.E_Interface_OperStatus)
	for port, stream := range otnStreams {
		operstatus[port] = validateSampleStream(t, dut, interfaceStreams[port].Next(), stream.Next(), port)
	}

	// Do process restart
	err := performance.RestartProcess(t, dut, "invmgr")
	if err != nil {
		t.Fatal(err)
	}

	// Wait for streaming telemetry to report the channels as up.
	for _, p := range dut.Ports() {
		gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
	}
	time.Sleep(6 * samplingInterval) // Wait an extra sample interval to ensure the device has time to process the change.

	for port, stream := range otnStreams {
		operstatus[port] = validateSampleStream(t, dut, interfaceStreams[port].Next(), stream.Next(), port)
	}

}

func TestZRShutPort(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	configureOTN(t, dut)

	// Wait for streaming telemetry to report the channels as up.
	for _, p := range dut.Ports() {
		gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
	}
	time.Sleep(3 * samplingInterval) // Wait an extra sample interval to ensure the device has time to process the change.

	//Initiate sample streams
	otnStreams := make(map[string]*samplestream.SampleStream[*oc.TerminalDevice_Channel])
	interfaceStreams := make(map[string]*samplestream.SampleStream[*oc.Interface])
	for portName, otnIndex := range otnIndexes {
		otnStreams[portName] = samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndex).State(), samplingInterval)
		interfaceStreams[portName] = samplestream.New(t, dut, gnmi.OC().Interface(portName).State(), samplingInterval)
		defer otnStreams[portName].Close()
		defer interfaceStreams[portName].Close()
	}

	//Verify the leaves
	operstatus := make(map[string]oc.E_Interface_OperStatus)
	for port, stream := range otnStreams {
		operstatus[port] = validateSampleStream(t, dut, interfaceStreams[port].Next(), stream.Next(), port)
	}

	// Disable interface.
	for _, p := range dut.Ports() {
		cfgplugins.ToggleInterface(t, dut, p.Name(), false)
	}

	// Wait for streaming telemetry to report the channels as down.
	for _, p := range dut.Ports() {
		gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_DOWN)
	}
	time.Sleep(3 * samplingInterval) // Wait an extra sample interval to ensure the device has time to process the change.

	for port, stream := range otnStreams {
		operstatus[port] = validateSampleStream(t, dut, interfaceStreams[port].Next(), stream.Next(), port)
	}

	// Re-enable transceivers.
	for _, p := range dut.Ports() {
		cfgplugins.ToggleInterface(t, dut, p.Name(), true)
	}

	// Wait for streaming telemetry to report the channels as up.
	for _, p := range dut.Ports() {
		gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
	}
	time.Sleep(3 * samplingInterval) // Wait an extra sample interval to ensure the device has time to process the change.

	for port, stream := range otnStreams {
		operstatus[port] = validateSampleStream(t, dut, interfaceStreams[port].Next(), stream.Next(), port)
	}

}

func TestZRLCReload(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	lc := findComponentsByTypeNoLogs(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD)

	// enable transceivers.
	for _, p := range dut.Ports() {
		cfgplugins.ToggleInterface(t, dut, p.Name(), true)
	}
	configureOTN(t, dut)

	// Wait for streaming telemetry to report the channels as up.
	for _, p := range dut.Ports() {
		gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
	}
	time.Sleep(3 * samplingInterval) // Wait an extra sample interval to ensure the device has time to process the change.

	//Initiate sample streams
	otnStreams := make(map[string]*samplestream.SampleStream[*oc.TerminalDevice_Channel])
	interfaceStreams := make(map[string]*samplestream.SampleStream[*oc.Interface])
	for portName, otnIndex := range otnIndexes {
		otnStreams[portName] = samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndex).State(), samplingInterval)
		interfaceStreams[portName] = samplestream.New(t, dut, gnmi.OC().Interface(portName).State(), samplingInterval)
		defer otnStreams[portName].Close()
		defer interfaceStreams[portName].Close()
	}

	//Verify the leaves
	operstatus := make(map[string]oc.E_Interface_OperStatus)
	for port, stream := range otnStreams {
		operstatus[port] = validateSampleStream(t, dut, interfaceStreams[port].Next(), stream.Next(), port)
	}

	var LC string
	//Using port1 for shut/unshut
	re := regexp.MustCompile(`\d+/\d+/\d+/\d+`)
	matches := re.FindStringSubmatch(dut.Port(t, "port1").Name())

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

	t.Logf("Restarting LC %s", LC)
	util.ReloadLinecards(t, []string{LC})
	//Sleeping additional 5 mins
	time.Sleep(5 * time.Minute)

	// enable transceivers.
	for _, p := range dut.Ports() {
		cfgplugins.ToggleInterface(t, dut, p.Name(), true)
	}

	// Wait for streaming telemetry to report the channels as up.
	for _, p := range dut.Ports() {
		gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
	}
	time.Sleep(6 * samplingInterval) // Wait an extra sample interval to ensure the device has time to process the change.

	for port, stream := range otnStreams {
		operstatus[port] = validateSampleStream(t, dut, interfaceStreams[port].Next(), stream.Next(), port)
	}
	t.Logf("All Gnmi leaves received successfully after LC Reload")

}

func TestZRRPFO(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	configureOTN(t, dut)

	// Wait for streaming telemetry to report the channels as up.
	for _, p := range dut.Ports() {
		gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
	}
	time.Sleep(3 * samplingInterval) // Wait an extra sample interval to ensure the device has time to process the change.

	//Initiate sample streams
	otnStreams := make(map[string]*samplestream.SampleStream[*oc.TerminalDevice_Channel])
	interfaceStreams := make(map[string]*samplestream.SampleStream[*oc.Interface])
	for portName, otnIndex := range otnIndexes {
		otnStreams[portName] = samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndex).State(), samplingInterval)
		interfaceStreams[portName] = samplestream.New(t, dut, gnmi.OC().Interface(portName).State(), samplingInterval)
		defer otnStreams[portName].Close()
		defer interfaceStreams[portName].Close()
	}

	//Verify the leaves
	operstatus := make(map[string]oc.E_Interface_OperStatus)
	for port, stream := range otnStreams {
		operstatus[port] = validateSampleStream(t, dut, interfaceStreams[port].Next(), stream.Next(), port)
	}

	// Do RPFO
	utils.Dorpfo(context.Background(), t, true)

	// Wait for streaming telemetry to report the channels as up.
	for _, p := range dut.Ports() {
		gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
	}
	time.Sleep(6 * samplingInterval) // Wait an extra sample interval to ensure the device has time to process the change.

	for port, stream := range otnStreams {
		operstatus[port] = validateSampleStream(t, dut, interfaceStreams[port].Next(), stream.Next(), port)
	}
}
