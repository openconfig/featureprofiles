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
	"math"
	"testing"

	"github.com/openconfig/featureprofiles/internal/components"
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

// InterfaceConfig configures the interface with the given port.
func InterfaceConfig(t *testing.T, dut *ondatra.DUTDevice, dp *ondatra.Port) {
	t.Helper()
	d := &oc.Root{}
	i := d.GetOrCreateInterface(dp.Name())
	i.Enabled = ygot.Bool(true)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	gnmi.Replace(t, dut, gnmi.OC().Interface(dp.Name()).Config(), i)
	ocComponent := components.OpticalChannelComponentFromPort(t, dut, dp)
	t.Logf("Got opticalChannelComponent from port: %s", ocComponent)
	gnmi.Update(t, dut, gnmi.OC().Component(ocComponent).Name().Config(), ocComponent)
	gnmi.Replace(t, dut, gnmi.OC().Component(ocComponent).OpticalChannel().Config(), &oc.Component_OpticalChannel{
		TargetOutputPower: ygot.Float64(targetOutputPowerdBm),
		Frequency:         ygot.Uint64(targetFrequencyMHz),
	})
}

// ValidateInterfaceConfig validates the output power and frequency for the given port.
func ValidateInterfaceConfig(t *testing.T, dut *ondatra.DUTDevice, dp *ondatra.Port, targetOutputPowerdBm float64, targetFrequencyMHz uint64, targetOutputPowerTolerancedBm float64, targetFrequencyToleranceMHz float64) {
	t.Helper()
	ocComponent := components.OpticalChannelComponentFromPort(t, dut, dp)
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

// ToggleInterface toggles the interface.
func ToggleInterface(t *testing.T, dut *ondatra.DUTDevice, intf string, isEnabled bool) {
	d := &oc.Root{}
	i := d.GetOrCreateInterface(intf)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	i.Enabled = ygot.Bool(isEnabled)
	gnmi.Replace(t, dut, gnmi.OC().Interface(intf).Config(), i)
}

// ConfigOpticalChannel configures the optical channel.
func ConfigOpticalChannel(t *testing.T, dut *ondatra.DUTDevice, och string, frequency uint64, targetOpticalPower float64, operationalMode uint16) {
	gnmi.Update(t, dut, gnmi.OC().Component(och).Name().Config(), och)
	gnmi.Replace(t, dut, gnmi.OC().Component(och).OpticalChannel().Config(), &oc.Component_OpticalChannel{
		OperationalMode:   ygot.Uint16(operationalMode),
		Frequency:         ygot.Uint64(frequency),
		TargetOutputPower: ygot.Float64(targetOpticalPower),
	})
}

// ConfigOTNChannel configures the OTN channel.
func ConfigOTNChannel(t *testing.T, dut *ondatra.DUTDevice, och string, otnIndex, ethIndex uint32) {
	t.Helper()
	if deviations.OTNChannelTribUnsupported(dut) {
		gnmi.Replace(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndex).Config(), &oc.TerminalDevice_Channel{
			Description:        ygot.String("OTN Logical Channel"),
			Index:              ygot.Uint32(otnIndex),
			LogicalChannelType: oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_OTN,
			Assignment: map[uint32]*oc.TerminalDevice_Channel_Assignment{
				0: {
					Index:          ygot.Uint32(1),
					OpticalChannel: ygot.String(och),
					Description:    ygot.String("OTN to Optical Channel"),
					Allocation:     ygot.Float64(400),
					AssignmentType: oc.Assignment_AssignmentType_OPTICAL_CHANNEL,
				},
			},
		})
	} else {
		gnmi.Replace(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndex).Config(), &oc.TerminalDevice_Channel{
			Description:        ygot.String("OTN Logical Channel"),
			Index:              ygot.Uint32(otnIndex),
			LogicalChannelType: oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_OTN,
			TribProtocol:       oc.TransportTypes_TRIBUTARY_PROTOCOL_TYPE_PROT_400GE,
			Assignment: map[uint32]*oc.TerminalDevice_Channel_Assignment{
				0: {
					Index:          ygot.Uint32(0),
					OpticalChannel: ygot.String(och),
					Description:    ygot.String("OTN to Optical Channel"),
					Allocation:     ygot.Float64(400),
					AssignmentType: oc.Assignment_AssignmentType_OPTICAL_CHANNEL,
				},
				1: {
					Index:          ygot.Uint32(1),
					LogicalChannel: ygot.Uint32(ethIndex),
					Description:    ygot.String("OTN to ETH"),
					Allocation:     ygot.Float64(400),
					AssignmentType: oc.Assignment_AssignmentType_LOGICAL_CHANNEL,
				},
			},
		})
	}
}

// ConfigETHChannel configures the ETH channel.
func ConfigETHChannel(t *testing.T, dut *ondatra.DUTDevice, interfaceName, transceiverName string, otnIndex, ethIndex uint32) {
	t.Helper()
	var ingress = &oc.TerminalDevice_Channel_Ingress{}
	if !deviations.EthChannelIngressParametersUnsupported(dut) {
		ingress = &oc.TerminalDevice_Channel_Ingress{
			Interface:   ygot.String(interfaceName),
			Transceiver: ygot.String(transceiverName),
		}
	}
	var assignment = map[uint32]*oc.TerminalDevice_Channel_Assignment{
		0: {
			Index:          ygot.Uint32(0),
			LogicalChannel: ygot.Uint32(otnIndex),
			Description:    ygot.String("ETH to OTN"),
			Allocation:     ygot.Float64(400),
			AssignmentType: oc.Assignment_AssignmentType_LOGICAL_CHANNEL,
		},
	}
	if deviations.EthChannelAssignmentCiscoNumbering(dut) {
		assignment[0].Index = ygot.Uint32(1)
	}
	var channel = &oc.TerminalDevice_Channel{
		Description:        ygot.String("ETH Logical Channel"),
		Index:              ygot.Uint32(ethIndex),
		LogicalChannelType: oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_ETHERNET,
		TribProtocol:       oc.TransportTypes_TRIBUTARY_PROTOCOL_TYPE_PROT_400GE,
		Ingress:            ingress,
		Assignment:         assignment,
	}
	if !deviations.ChannelRateClassParametersUnsupported(dut) {
		channel.RateClass = oc.TransportTypes_TRIBUTARY_RATE_CLASS_TYPE_TRIB_RATE_400G
	}
	gnmi.Replace(t, dut, gnmi.OC().TerminalDevice().Channel(ethIndex).Config(), channel)
}
