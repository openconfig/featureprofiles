package cfgplugins

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

// isSubCompOfHardwarePort is a helper function to check if the given component is a subComponent of the given hardwarePort.
func isSubCompOfHardwarePort(t *testing.T, dut *ondatra.DUTDevice, parentHardwarePortName string, comp *oc.Component) bool {
	for {
		if comp.GetName() == parentHardwarePortName {
			return true
		}
		if comp.GetType() == oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_PORT {
			return false
		}
		comp = gnmi.Get(t, dut, gnmi.OC().Component(comp.GetParent()).State())
	}
}

// OpticalChannelComponentFromPort returns the OpticalChannelComponent name for the provided  port.
func OpticalChannelComponentFromPort(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) string {
	t.Helper()
	if deviations.MissingPortToOpticalChannelMapping(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			transceiverName := gnmi.Get(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State())
			return fmt.Sprintf("%s-Optical0", transceiverName)
		default:
			t.Fatal("Manual Optical channel name required when deviation missing_port_to_optical_channel_component_mapping applied.")
		}
	}
	comps := gnmi.LookupAll(t, dut, gnmi.OC().ComponentAny().State())
	hardwarePortCompName := gnmi.Get(t, dut, gnmi.OC().Interface(p.Name()).HardwarePort().State())
	for _, comp := range comps {
		comp, ok := comp.Val()

		if ok && comp.GetType() == oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_OPTICAL_CHANNEL && isSubCompOfHardwarePort(t, dut, hardwarePortCompName, comp) {
			return comp.GetName()
		}
	}
	t.Fatalf("No interface to optical-channel mapping found for interface = %v", p.Name())
	return ""
}

// ConfigureInterface configures the interface with portName and interfaceType.
func ConfigureInterface(t *testing.T, dut *ondatra.DUTDevice, dp *ondatra.Port) {
	d := &oc.Root{}
	i := d.GetOrCreateInterface(dp.Name())
	i.Enabled = ygot.Bool(true)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	gnmi.Replace(t, dut, gnmi.OC().Interface(dp.Name()).Config(), i)
}

// ConfigureTargetOutputPowerAndFrequency configures TargetOutputPower and Frequency for the given transceiver port.
func ConfigureTargetOutputPowerAndFrequency(t *testing.T, dut *ondatra.DUTDevice, dp *ondatra.Port, targetOutputPower float64, frequency uint64) {
	OCcomponent := OpticalChannelComponentFromPort(t, dut, dp)
	gnmi.Replace(t, dut, gnmi.OC().Component(OCcomponent).OpticalChannel().Config(), &oc.Component_OpticalChannel{
		TargetOutputPower: ygot.Float64(targetOutputPower),
		Frequency:         ygot.Uint64(frequency),
	})
}
