package zr_logical_channels_test

import (
	"flag"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	targetOutputPower = -9
	frequency         = 193100000
	samplingInterval  = 10 * time.Second
	timeout           = 10 * time.Minute
	otnIndex1         = uint32(4001)
	otnIndex2         = uint32(4002)
	ethernetIndex1    = uint32(40001)
	ethernetIndex2    = uint32(40002)
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: 30,
	}
	operationalModeFlag = flag.Int("operational_mode", 1, "vendor-specific operational-mode for the channel")
	operationalMode     uint16
)

type testcase struct {
	desc string
	got  any
	want any
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func Test400ZRLogicalChannels(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	if operationalModeFlag != nil {
		operationalMode = uint16(*operationalModeFlag)
	} else {
		t.Fatalf("Please specify the vendor-specific operational-mode flag")
	}
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	fptest.ConfigureDefaultNetworkInstance(t, dut)

	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))

	oc1 := components.OpticalChannelComponentFromPort(t, dut, p1)
	oc2 := components.OpticalChannelComponentFromPort(t, dut, p2)
	tr1 := gnmi.Get(t, dut, gnmi.OC().Interface(p1.Name()).Transceiver().State())
	tr2 := gnmi.Get(t, dut, gnmi.OC().Interface(p2.Name()).Transceiver().State())

	cfgplugins.ConfigOpticalChannel(t, dut, oc1, frequency, targetOutputPower, operationalMode)
	cfgplugins.ConfigOTNChannel(t, dut, oc1, otnIndex1, ethernetIndex1)
	cfgplugins.ConfigETHChannel(t, dut, p1.Name(), tr1, otnIndex1, ethernetIndex1)
	cfgplugins.ConfigOpticalChannel(t, dut, oc2, frequency, targetOutputPower, operationalMode)
	cfgplugins.ConfigOTNChannel(t, dut, oc2, otnIndex2, ethernetIndex2)
	cfgplugins.ConfigETHChannel(t, dut, p2.Name(), tr2, otnIndex2, ethernetIndex2)

	ethChan1 := samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(ethernetIndex1).State(), samplingInterval)
	defer ethChan1.Close()
	ethChan2 := samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(ethernetIndex2).State(), samplingInterval)
	defer ethChan2.Close()
	otnChan1 := samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndex1).State(), samplingInterval)
	defer otnChan1.Close()
	otnChan2 := samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndex2).State(), samplingInterval)
	defer otnChan2.Close()

	gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
	gnmi.Await(t, dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)

	validateEthernetChannelTelemetry(t, dut, otnIndex1, ethernetIndex1, ethChan1)
	validateEthernetChannelTelemetry(t, dut, otnIndex2, ethernetIndex2, ethChan2)
	validateOTNChannelTelemetry(t, dut, otnIndex1, ethernetIndex1, oc1, otnChan1)
	validateOTNChannelTelemetry(t, dut, otnIndex2, ethernetIndex2, oc2, otnChan2)
}

func validateEthernetChannelTelemetry(t *testing.T, dut *ondatra.DUTDevice, otnChIdx, ethernetChIdx uint32, stream *samplestream.SampleStream[*oc.TerminalDevice_Channel]) {
	val := stream.Next() // value received in the gnmi subscription within 10 seconds
	if val == nil {
		t.Fatalf("Ethernet Channel telemetry stream not received in last 10 seconds")
	}
	ec, ok := val.Val()
	if !ok {
		t.Fatalf("Ethernet Channel telemetry stream empty in last 10 seconds")
	}
	tcs := []testcase{
		{
			desc: "Index",
			got:  ec.GetIndex(),
			want: ethernetChIdx,
		},
		{
			desc: "Description",
			got:  ec.GetDescription(),
			want: "ETH Logical Channel",
		},
		{
			desc: "Logical Channel Type",
			got:  ec.GetLogicalChannelType().String(),
			want: oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_ETHERNET.String(),
		},
		{
			desc: "Trib Protocol",
			got:  ec.GetTribProtocol().String(),
			want: oc.TransportTypes_TRIBUTARY_PROTOCOL_TYPE_PROT_400GE.String(),
		},
	}
	var assignmentIndexTestcases []testcase

	if deviations.EthChannelAssignmentCiscoNumbering(dut) {
		assignmentIndexTestcases = []testcase{
			{
				desc: "Assignment: Index",
				got:  ec.GetAssignment(1).GetIndex(),
				want: uint32(1)},
			{
				desc: "Assignment: Logical Channel",
				got:  ec.GetAssignment(1).GetLogicalChannel(),
				want: otnChIdx,
			},
			{
				desc: "Assignment: Description",
				got:  ec.GetAssignment(1).GetDescription(),
				want: "ETH to OTN",
			},
			{
				desc: "Assignment: Allocation",
				got:  ec.GetAssignment(1).GetAllocation(),
				want: float64(400),
			},
			{
				desc: "Assignment: Type",
				got:  ec.GetAssignment(1).GetAssignmentType().String(),
				want: oc.Assignment_AssignmentType_LOGICAL_CHANNEL.String(),
			}}
	} else {
		assignmentIndexTestcases = []testcase{
			{
				desc: "Assignment: Index",
				got:  ec.GetAssignment(0).GetIndex(),
				want: uint32(0),
			},
			{
				desc: "Assignment: Logical Channel",
				got:  ec.GetAssignment(0).GetLogicalChannel(),
				want: otnChIdx,
			},
			{
				desc: "Assignment: Description",
				got:  ec.GetAssignment(0).GetDescription(),
				want: "ETH to OTN",
			},
			{
				desc: "Assignment: Allocation",
				got:  ec.GetAssignment(0).GetAllocation(),
				want: float64(400),
			},
			{
				desc: "Assignment: Type",
				got:  ec.GetAssignment(0).GetAssignmentType().String(),
				want: oc.Assignment_AssignmentType_LOGICAL_CHANNEL.String(),
			}}
	}
	tcs = append(tcs, assignmentIndexTestcases...)
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			if diff := cmp.Diff(tc.got, tc.want); diff != "" {
				t.Errorf("Ethernet Logical Channel: %s, diff (-got +want):\n%s", tc.desc, diff)
			}
		})
	}
}

func validateOTNChannelTelemetry(t *testing.T, dut *ondatra.DUTDevice, otnChIdx uint32, ethChIdx uint32, opticalChannel string, stream *samplestream.SampleStream[*oc.TerminalDevice_Channel]) {
	val := stream.Next() // value received in the gnmi subscription within 10 seconds
	if val == nil {
		t.Fatalf("OTN Channel telemetry stream not received in last 10 seconds")
	}
	cc, ok := val.Val()
	if !ok {
		t.Fatalf("OTN Channel telemetry stream empty in last 10 seconds")
	}
	tcs := []testcase{
		{
			desc: "Description",
			got:  cc.GetDescription(),
			want: "OTN Logical Channel",
		},
		{
			desc: "Index",
			got:  cc.GetIndex(),
			want: otnChIdx,
		},
		{
			desc: "Logical Channel Type",
			got:  cc.GetLogicalChannelType().String(),
			want: oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_OTN.String(),
		},
	}
	var opticalChannelAssignmentIndexTestcases []testcase

	if deviations.OTNChannelAssignmentCiscoNumbering(dut) {
		ciscoOpticalChannelFormat := strings.ReplaceAll(opticalChannel, "/", "_") // Ex: OpticalChannel0_0_0_18
		opticalChannelAssignmentIndexTestcases = []testcase{
			{
				desc: "Assignment: Index",
				got:  cc.GetAssignment(1).GetIndex(),
				want: uint32(1),
			},
			{
				desc: "Optical Channel Assignment: Optical Channel",
				got:  cc.GetAssignment(1).GetOpticalChannel(),
				want: ciscoOpticalChannelFormat,
			},
			{
				desc: "Optical Channel Assignment: Description",
				got:  cc.GetAssignment(1).GetDescription(),
				want: "OTN to Optical Channel",
			},
			{
				desc: "Optical Channel Assignment: Allocation",
				got:  cc.GetAssignment(1).GetAllocation(),
				want: float64(400),
			},
			{
				desc: "Optical Channel Assignment: Type",
				got:  cc.GetAssignment(1).GetAssignmentType().String(),
				want: oc.Assignment_AssignmentType_OPTICAL_CHANNEL.String(),
			},
		}
	} else {
		opticalChannelAssignmentIndexTestcases = []testcase{
			{
				desc: "Assignment: Index",
				got:  cc.GetAssignment(0).GetIndex(),
				want: uint32(0)},
			{
				desc: "Optical Channel Assignment: Optical Channel",
				got:  cc.GetAssignment(0).GetOpticalChannel(),
				want: opticalChannel,
			},
			{
				desc: "Optical Channel Assignment: Description",
				got:  cc.GetAssignment(0).GetDescription(),
				want: "OTN to Optical Channel",
			},
			{
				desc: "Optical Channel Assignment: Allocation",
				got:  cc.GetAssignment(0).GetAllocation(),
				want: float64(400),
			},
			{
				desc: "Optical Channel Assignment: Type",
				got:  cc.GetAssignment(0).GetAssignmentType().String(),
				want: oc.Assignment_AssignmentType_OPTICAL_CHANNEL.String(),
			},
		}
	}
	tcs = append(tcs, opticalChannelAssignmentIndexTestcases...)

	if !deviations.OTNChannelTribUnsupported(dut) {
		logicalChannelAssignmentTestcases := []testcase{
			{
				desc: "Ethernet Assignment: Index",
				got:  cc.GetAssignment(1).GetIndex(),
				want: uint32(1),
			},
			{
				desc: "Ethernet Assignment: Logical Channel",
				got:  cc.GetAssignment(1).GetLogicalChannel(),
				want: ethChIdx,
			},
			{
				desc: "Ethernet Assignment: Description",
				got:  cc.GetAssignment(1).GetDescription(),
				want: "OTN to ETH",
			},
			{
				desc: "Ethernet Assignment: Allocation",
				got:  cc.GetAssignment(1).GetAllocation(),
				want: float64(400),
			},
			{
				desc: "Ethernet Assignment: Type",
				got:  cc.GetAssignment(1).GetAssignmentType().String(),
				want: oc.Assignment_AssignmentType_LOGICAL_CHANNEL.String(),
			},
		}
		tcs = append(tcs, logicalChannelAssignmentTestcases...)
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			if diff := cmp.Diff(tc.got, tc.want); diff != "" {
				t.Errorf("OTN Logical Channel: %s, diff (-got +want):\n%s", tc.desc, diff)
			}
		})
	}
}
