package transceiver

import (
	"testing"

	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/ondatra/gnmi/oc"
)

func TestValidateOpticalChannelComponent_PolarizationTelemetry_UP(t *testing.T) {
	portName := "port1"
	params := &cfgplugins.ConfigParameters{
		OpticalChannelNames: map[string]string{portName: "OpticalChannel1"},
		TransceiverNames:    map[string]string{portName: "Transceiver1"},
		LinePortNames:       map[string]string{portName: "Port1"},
		OperationalMode:     1,
		Frequency:           196100000,
		TargetOpticalPower:  -7.0,
	}

	comp := &oc.Component{
		Name:   ygot.String("OpticalChannel1"),
		Parent: ygot.String("Transceiver1"),
	}
	och := comp.GetOrCreateOpticalChannel()
	och.OperationalMode = ygot.Uint16(1)
	och.Frequency = ygot.Uint64(196100000)
	och.TargetOutputPower = ygot.Float64(-7.0)
	och.LinePort = ygot.String("Port1")
	och.LaserBiasCurrent = &oc.Component_OpticalChannel_LaserBiasCurrent{
		Instant: ygot.Float64(50.0),
	}
	och.InputPower = &oc.Component_OpticalChannel_InputPower{
		Instant: ygot.Float64(-7.0),
		Avg:     ygot.Float64(-7.0),
		Min:     ygot.Float64(-8.0),
		Max:     ygot.Float64(-6.0),
	}
	och.OutputPower = &oc.Component_OpticalChannel_OutputPower{
		Instant: ygot.Float64(-7.0),
		Avg:     ygot.Float64(-7.0),
		Min:     ygot.Float64(-8.0),
		Max:     ygot.Float64(-6.0),
	}
	och.ChromaticDispersion = &oc.Component_OpticalChannel_ChromaticDispersion{
		Instant: ygot.Float64(500.0),
		Avg:     ygot.Float64(500.0),
		Min:     ygot.Float64(450.0),
		Max:     ygot.Float64(550.0),
	}
	och.CarrierFrequencyOffset = &oc.Component_OpticalChannel_CarrierFrequencyOffset{
		Instant: ygot.Float64(10.0),
		Avg:     ygot.Float64(10.0),
		Min:     ygot.Float64(5.0),
		Max:     ygot.Float64(15.0),
	}

	// 12 Polarization Telemetry Leaves: PMD, Second-Order PMD, SOP-ROC
	och.PolarizationModeDispersion = &oc.Component_OpticalChannel_PolarizationModeDispersion{
		Instant: ygot.Float64(15.0),
		Avg:     ygot.Float64(14.8),
		Min:     ygot.Float64(10.0),
		Max:     ygot.Float64(20.0),
	}
	och.SecondOrderPolarizationModeDispersion = &oc.Component_OpticalChannel_SecondOrderPolarizationModeDispersion{
		Instant: ygot.Float64(120.0),
		Avg:     ygot.Float64(118.5),
		Min:     ygot.Float64(100.0),
		Max:     ygot.Float64(150.0),
	}
	och.SopRoc = &oc.Component_OpticalChannel_SopRoc{
		Instant: ygot.Float64(250.0),
		Avg:     ygot.Float64(245.0),
		Min:     ygot.Float64(100.0),
		Max:     ygot.Float64(400.0),
	}

	validateOpticalChannelComponent(t, portName, comp, params, oc.Interface_OperStatus_UP)
}

func TestValidateOpticalChannelComponent_PolarizationTelemetry_DOWN(t *testing.T) {
	portName := "port1"
	params := &cfgplugins.ConfigParameters{
		OpticalChannelNames: map[string]string{portName: "OpticalChannel1"},
		TransceiverNames:    map[string]string{portName: "Transceiver1"},
		LinePortNames:       map[string]string{portName: "Port1"},
		OperationalMode:     1,
		Frequency:           196100000,
		TargetOpticalPower:  -7.0,
	}

	comp := &oc.Component{
		Name:   ygot.String("OpticalChannel1"),
		Parent: ygot.String("Transceiver1"),
	}
	och := comp.GetOrCreateOpticalChannel()
	och.OperationalMode = ygot.Uint16(1)
	och.TargetOutputPower = ygot.Float64(-7.0)
	och.LinePort = ygot.String("Port1")
	och.InputPower = &oc.Component_OpticalChannel_InputPower{
		Instant: ygot.Float64(-30.0),
		Avg:     ygot.Float64(-30.0),
		Min:     ygot.Float64(-35.0),
		Max:     ygot.Float64(-30.0),
	}
	och.OutputPower = &oc.Component_OpticalChannel_OutputPower{
		Instant: ygot.Float64(-30.0),
		Avg:     ygot.Float64(-30.0),
		Min:     ygot.Float64(-35.0),
		Max:     ygot.Float64(-30.0),
	}
	och.ChromaticDispersion = &oc.Component_OpticalChannel_ChromaticDispersion{
		Instant: ygot.Float64(0.0),
		Avg:     ygot.Float64(0.0),
		Min:     ygot.Float64(0.0),
		Max:     ygot.Float64(0.0),
	}

	// Down status threshold tests
	och.PolarizationModeDispersion = &oc.Component_OpticalChannel_PolarizationModeDispersion{
		Instant: ygot.Float64(0.0),
		Avg:     ygot.Float64(0.0),
		Min:     ygot.Float64(0.0),
		Max:     ygot.Float64(0.0),
	}
	och.SecondOrderPolarizationModeDispersion = &oc.Component_OpticalChannel_SecondOrderPolarizationModeDispersion{
		Instant: ygot.Float64(0.0),
		Avg:     ygot.Float64(0.0),
		Min:     ygot.Float64(0.0),
		Max:     ygot.Float64(0.0),
	}
	och.SopRoc = &oc.Component_OpticalChannel_SopRoc{
		Instant: ygot.Float64(0.0),
		Avg:     ygot.Float64(0.0),
		Min:     ygot.Float64(0.0),
		Max:     ygot.Float64(0.0),
	}

	validateOpticalChannelComponent(t, portName, comp, params, oc.Interface_OperStatus_DOWN)
}

func TestIsOpticalChannelStreamReady(t *testing.T) {
	params := &cfgplugins.ConfigParameters{
		OperationalMode:    1,
		TargetOpticalPower: -7.0,
	}

	comp := &oc.Component{
		Name: ygot.String("OpticalChannel1"),
	}
	och := comp.GetOrCreateOpticalChannel()
	och.OperationalMode = ygot.Uint16(1)
	och.InputPower = &oc.Component_OpticalChannel_InputPower{
		Instant: ygot.Float64(-7.0),
		Avg:     ygot.Float64(-7.0),
		Min:     ygot.Float64(-8.0),
		Max:     ygot.Float64(-6.0),
	}
	och.OutputPower = &oc.Component_OpticalChannel_OutputPower{
		Instant: ygot.Float64(-7.0),
		Avg:     ygot.Float64(-7.0),
		Min:     ygot.Float64(-8.0),
		Max:     ygot.Float64(-6.0),
	}

	if !isOpticalChannelStreamReady(comp, oc.Interface_OperStatus_UP, params) {
		t.Errorf("isOpticalChannelStreamReady got false, want true for valid UP parameters")
	}

	// Wrong operational mode
	och.OperationalMode = ygot.Uint16(2)
	if isOpticalChannelStreamReady(comp, oc.Interface_OperStatus_UP, params) {
		t.Errorf("isOpticalChannelStreamReady got true, want false for mismatched operational mode")
	}
}
