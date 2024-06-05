// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cfgplugins

import (
	"fmt"
	"math"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	targetOutputPowerdBm          = -10
	targetOutputPowerTolerancedBm = 1
	targetFrequencyMHz            = 193100000
	targetFrequencyToleranceMHz   = 100000
)

// opticalChannelComponentFromPort returns the name of the optical channel component for the given
// port.
func opticalChannelComponentFromPort(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) string {
	t.Helper()
	if deviations.MissingPortToOpticalChannelMapping(dut) {
		transceiverName := gnmi.Get(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State())
		return fmt.Sprintf("%s-Optical0", transceiverName)
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

// InterfaceConfig configures the interface with the given port.
func InterfaceConfig(t *testing.T, dut *ondatra.DUTDevice, dp *ondatra.Port) {
	t.Helper()
	d := &oc.Root{}
	i := d.GetOrCreateInterface(dp.Name())
	i.Enabled = ygot.Bool(true)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	gnmi.Replace(t, dut, gnmi.OC().Interface(dp.Name()).Config(), i)
	ocComponent := opticalChannelComponentFromPort(t, dut, dp)
	t.Logf("Got opticalChannelComponent from port: %s", ocComponent)
	gnmi.Replace(t, dut, gnmi.OC().Component(ocComponent).OpticalChannel().Config(), &oc.Component_OpticalChannel{
		TargetOutputPower: ygot.Float64(targetOutputPowerdBm),
		Frequency:         ygot.Uint64(targetFrequencyMHz),
	})
}

// ValidateInterfaceConfig validates the output power and frequency for the given port.
func ValidateInterfaceConfig(t *testing.T, dut *ondatra.DUTDevice, dp *ondatra.Port) {
	t.Helper()
	ocComponent := opticalChannelComponentFromPort(t, dut, dp)
	t.Logf("Got opticalChannelComponent from port: %s", ocComponent)

	outputPower := gnmi.Get(t, dut, gnmi.OC().Component(ocComponent).OpticalChannel().TargetOutputPower().State())
	if math.Abs(float64(outputPower)-float64(targetOutputPowerdBm)) > targetOutputPowerTolerancedBm {
		t.Fatalf("Output power is not within expected tolerance, got: %v want: %v tolerance: %v", outputPower, targetOutputPowerdBm, targetOutputPowerTolerancedBm)
	}

	frequency := gnmi.Get(t, dut, gnmi.OC().Component(ocComponent).OpticalChannel().Frequency().State())
	if math.Abs(float64(frequency)-float64(targetFrequencyMHz)) > targetFrequencyToleranceMHz {
		t.Fatalf("Frequency is not within expected tolerance, got: %v want: %v tolerance: %v", frequency, targetFrequencyMHz, targetFrequencyToleranceMHz)
	}
}
