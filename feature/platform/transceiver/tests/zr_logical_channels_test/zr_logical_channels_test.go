package zr_logical_channels_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	targetOutputPower = -9
	frequency         = 193100000
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
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func Test400ZRLogicalChannels(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	fptest.ConfigureDefaultNetworkInstance(t, dut)
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))

	oc1 := opticalChannelFromPort(t, dut, p1)
	oc2 := opticalChannelFromPort(t, dut, p2)

	configureLogicalChannels(t, dut, 40000, 40001, oc1)
	configureLogicalChannels(t, dut, 40002, 40003, oc2)

	ethChan1 := samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(40000).State(), 10*time.Second)
	defer ethChan1.Close()
	validateEthernetChannelTelemetry(t, 40000, 40001, ethChan1)
	cohChan1 := samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(40001).State(), 10*time.Second)
	defer cohChan1.Close()
	validateCoherentChannelTelemetry(t, 40001, oc1, cohChan1)
	ethChan2 := samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(40002).State(), 10*time.Second)
	defer ethChan2.Close()
	validateEthernetChannelTelemetry(t, 40002, 40003, ethChan2)
	cohChan2 := samplestream.New(t, dut, gnmi.OC().TerminalDevice().Channel(40002).State(), 10*time.Second)
	defer cohChan2.Close()
	validateCoherentChannelTelemetry(t, 40003, oc2, cohChan2)
}

func configureLogicalChannels(t *testing.T, dut *ondatra.DUTDevice, ethernetChIdx, coherentChIdx uint32, opticalChannel string) {
	t.Helper()
	b := &gnmi.SetBatch{}

	// Optical Channel and Tunable Parameters
	gnmi.BatchReplace(b, gnmi.OC().Component(opticalChannel).OpticalChannel().Config(), &oc.Component_OpticalChannel{
		TargetOutputPower: ygot.Float64(targetOutputPower),
		Frequency:         ygot.Uint64(frequency),
	})

	// Ethernet Logical Channel
	gnmi.BatchReplace(b, gnmi.OC().TerminalDevice().Channel(ethernetChIdx).Config(), &oc.TerminalDevice_Channel{
		LogicalChannelType: oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_ETHERNET,
		AdminState:         oc.TerminalDevice_AdminStateType_ENABLED,
		Description:        ygot.String("ETH Logical Channel"),
		Index:              ygot.Uint32(ethernetChIdx),
		RateClass:          oc.TransportTypes_TRIBUTARY_RATE_CLASS_TYPE_TRIB_RATE_400G,
		TribProtocol:       oc.TransportTypes_TRIBUTARY_PROTOCOL_TYPE_PROT_400GE,
		Assignment: map[uint32]*oc.TerminalDevice_Channel_Assignment{
			1: {
				Index:          ygot.Uint32(1),
				LogicalChannel: ygot.Uint32(coherentChIdx),
				Description:    ygot.String("ETH to Coherent"),
				Allocation:     ygot.Float64(400),
				AssignmentType: oc.Assignment_AssignmentType_LOGICAL_CHANNEL,
			},
		},
	})

	// Coherent Logical Channel
	gnmi.BatchReplace(b, gnmi.OC().TerminalDevice().Channel(coherentChIdx).Config(), &oc.TerminalDevice_Channel{
		LogicalChannelType: oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_OTN,
		AdminState:         oc.TerminalDevice_AdminStateType_ENABLED,
		Description:        ygot.String("Coherent Logical Channel"),
		Index:              ygot.Uint32(coherentChIdx),
		Assignment: map[uint32]*oc.TerminalDevice_Channel_Assignment{
			1: {
				Index:          ygot.Uint32(1),
				OpticalChannel: ygot.String(opticalChannel),
				Description:    ygot.String("Coherent to Optical"),
				Allocation:     ygot.Float64(400),
				AssignmentType: oc.Assignment_AssignmentType_OPTICAL_CHANNEL,
			},
		},
	})

	b.Set(t, dut)
}

func validateEthernetChannelTelemetry(t *testing.T, ethernetChIdx, coherentChIdx uint32, stream *samplestream.SampleStream[*oc.TerminalDevice_Channel]) {
	val := stream.Next(t) // value received in the gnmi subscription within 10 seconds
	if val == nil {
		t.Fatalf("Ethernet Channel telemetry stream not received in last 10 seconds")
	}
	ec, ok := val.Val()
	if !ok {
		t.Fatalf("Ethernet Channel telemetry stream empty in last 10 seconds")
	}
	tcs := []struct {
		desc string
		got  any
		want any
	}{
		{
			desc: "Index",
			got:  ec.GetIndex(),
			want: ethernetChIdx,
		},
		{
			desc: "Admin State",
			got:  ec.GetAdminState().String(),
			want: oc.TerminalDevice_AdminStateType_ENABLED.String(),
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
			desc: "Rate Class",
			got:  ec.GetRateClass().String(),
			want: oc.TransportTypes_TRIBUTARY_RATE_CLASS_TYPE_TRIB_RATE_400G.String(),
		},
		{
			desc: "Trib Protocol",
			got:  ec.GetTribProtocol().String(),
			want: oc.TransportTypes_TRIBUTARY_PROTOCOL_TYPE_PROT_400GE.String(),
		},
		{
			desc: "Assignment: Index",
			got:  ec.GetAssignment(1).GetIndex(),
			want: uint32(1),
		},
		{
			desc: "Assignment: Logical Channel",
			got:  ec.GetAssignment(1).GetLogicalChannel(),
			want: coherentChIdx,
		},
		{
			desc: "Assignment: Description",
			got:  ec.GetAssignment(1).GetDescription(),
			want: "ETH to Coherent",
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
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			if diff := cmp.Diff(tc.got, tc.want); diff != "" {
				t.Errorf("Ethernet Logical Channel: %s, diff (-got +want):\n%s", tc.desc, diff)
			}
		})
	}
}

func validateCoherentChannelTelemetry(t *testing.T, coherentChIdx uint32, opticalChannel string, stream *samplestream.SampleStream[*oc.TerminalDevice_Channel]) {
	val := stream.Next(t) // value received in the gnmi subscription within 10 seconds
	if val == nil {
		t.Fatalf("Coherent Channel telemetry stream not received in last 10 seconds")
	}
	cc, ok := val.Val()
	if !ok {
		t.Fatalf("Coherent Channel telemetry stream empty in last 10 seconds")
	}
	tcs := []struct {
		desc string
		got  any
		want any
	}{
		{
			desc: "Admin State",
			got:  cc.GetAdminState().String(),
			want: oc.TerminalDevice_AdminStateType_ENABLED.String(),
		},
		{
			desc: "Description",
			got:  cc.GetDescription(),
			want: "Coherent Logical Channel",
		},
		{
			desc: "Index",
			got:  cc.GetIndex(),
			want: coherentChIdx,
		},
		{
			desc: "Logical Channel Type",
			got:  cc.GetLogicalChannelType().String(),
			want: oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_OTN.String(),
		},
		{
			desc: "Assignment: Index",
			got:  cc.GetAssignment(1).GetIndex(),
			want: uint32(1),
		},
		{
			desc: "Assignment: Optical Channel",
			got:  cc.GetAssignment(1).GetOpticalChannel(),
			want: opticalChannel,
		},
		{
			desc: "Assignment: Description",
			got:  cc.GetAssignment(1).GetDescription(),
			want: "Coherent to Optical",
		},
		{
			desc: "Assignment: Allocation",
			got:  cc.GetAssignment(1).GetAllocation(),
			want: float64(400),
		},
		{
			desc: "Assignment: Type",
			got:  cc.GetAssignment(1).GetAssignmentType().String(),
			want: oc.Assignment_AssignmentType_OPTICAL_CHANNEL.String(),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			if diff := cmp.Diff(tc.got, tc.want); diff != "" {
				t.Errorf("Coherent Logical Channel: %s, diff (-got +want):\n%s", tc.desc, diff)
			}
		})
	}
}

// opticalChannelFromPort returns the connected optical channel component name for a given ondatra port.
func opticalChannelFromPort(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) string {
	t.Helper()

	if deviations.MissingPortToOpticalChannelMapping(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			return fmt.Sprintf("%s-Optical0", p.Name())
		default:
			t.Fatal("Manual Optical channel name required when deviation missing_port_to_optical_channel_component_mapping applied.")
		}
	}
	compName := gnmi.Get(t, dut, gnmi.OC().Interface(p.Name()).HardwarePort().State())

	for {
		comp, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(compName).State()).Val()
		if !ok {
			t.Fatalf("Recursive optical channel lookup failed for port: %s, component %s not found.", p.Name(), compName)
		}
		if comp.GetType() == oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_OPTICAL_CHANNEL {
			return compName
		}

		if comp.GetParent() == "" {
			t.Fatalf("Recursive optical channel lookup failed for port: %s, parent of component %s not found.", p.Name(), compName)
		}

		compName = comp.GetParent()
	}
}
