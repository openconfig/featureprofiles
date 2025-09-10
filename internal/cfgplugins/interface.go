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
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
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

var (
	opmode   uint16
	once     sync.Once
	lBandPNs = map[string]bool{
		"DP04QSDD-LLH-240": true, // Cisco QSFPDD Acacia 400G ZRP L-Band
		"DP04QSDD-LLH-00A": true, // Cisco QSFPDD Acacia 400G ZRP L-Band
		"DP08SFP8-LRB-240": true, // Cisco OSFP Acacia 800G ZRP L-Band
		"C-OS08LEXNC-GG":   true, // Nokia OSFP 800G ZRP L-Band
		"176-6490-9G1":     true, // Ciena OSFP 800G ZRP L-Band
	}
)

// Temporary code for assigning opmode 1 maintained until opmode is Initialized in all .go file
func init() {
	opmode = 1
}

// OperationalModeList is a type for a list of operational modes in uint16 format.
type OperationalModeList []uint16

// String returns the string representation of the list of operational modes.
func (om *OperationalModeList) String() string {
	var s []string
	for _, v := range *om {
		s = append(s, fmt.Sprintf("%v", v))
	}
	return strings.Join(s, ",")
}

// Set sets the list of operational modes from the string representation.
func (om *OperationalModeList) Set(value string) error {
	for _, s := range strings.Split(value, ",") {
		if v, err := strconv.ParseUint(s, 10, 16); err != nil {
			return err
		} else {
			*om = append(*om, uint16(v))
		}
	}
	return nil
}

// Get returns the list of operational modes.
func (om *OperationalModeList) Get() any {
	return *om
}

// Default returns the default operational mode list.
func (om *OperationalModeList) Default(t *testing.T, dut *ondatra.DUTDevice) OperationalModeList {
	t.Helper()
	p := dut.Ports()[0]
	switch p.PMD() {
	case ondatra.PMD400GBASEZR: // 400G (8x56G)
		switch dut.Vendor() {
		case ondatra.CISCO:
			return OperationalModeList{5003}
		case ondatra.ARISTA, ondatra.JUNIPER:
			return OperationalModeList{1}
		case ondatra.NOKIA:
			return OperationalModeList{1083}
		default:
			t.Fatalf("Unsupported vendor: %v", dut.Vendor())
		}
	case ondatra.PMD400GBASEZRP: // 400G (8x56G)
		switch dut.Vendor() {
		case ondatra.CISCO:
			return OperationalModeList{6004}
		case ondatra.ARISTA, ondatra.JUNIPER, ondatra.NOKIA:
			return OperationalModeList{4}
		default:
			t.Fatalf("Unsupported vendor: %v", dut.Vendor())
		}
	case ondatra.PMD800GBASEZR:
		return OperationalModeList{1, 3} // 800G : 1 (8x112G), 400G : 3 (4x112G)
	case ondatra.PMD800GBASEZRP:
		return OperationalModeList{8, 4} // 800G : 8 (8x112G), 400G : 4 (4x112G)
	default:
		t.Fatalf("Unsupported PMD type: %v", p.PMD())
	}
	return nil
}

// FrequencyList is a type for a list of frequencies in uint64 format.
type FrequencyList []uint64

// String returns the string representation of the list of frequencies.
func (f *FrequencyList) String() string {
	var s []string
	for _, v := range *f {
		s = append(s, fmt.Sprintf("%v", v))
	}
	return strings.Join(s, ",")
}

// Set sets the list of frequencies from the string representation.
func (f *FrequencyList) Set(value string) error {
	for _, s := range strings.Split(value, ",") {
		if v, err := strconv.ParseUint(s, 10, 64); err != nil {
			return err
		} else {
			*f = append(*f, v)
		}
	}
	return nil
}

// Get returns the list of frequencies.
func (f *FrequencyList) Get() any {
	return *f
}

// Default returns the default frequency list.
func (f *FrequencyList) Default(t *testing.T, dut *ondatra.DUTDevice) FrequencyList {
	t.Helper()
	p := dut.Ports()[0]
	switch p.PMD() {
	case ondatra.PMD400GBASEZR, ondatra.PMD800GBASEZR:
		return FrequencyList{196100000}
	case ondatra.PMD400GBASEZRP, ondatra.PMD800GBASEZRP:
		tr, present := gnmi.Lookup(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State()).Val()
		if !present {
			t.Fatalf("Transceiver not found for port %v", p.Name())
		}
		pn, present := gnmi.Lookup(t, dut, gnmi.OC().Component(tr).PartNo().State()).Val()
		switch {
		case present && lBandPNs[pn]:
			return FrequencyList{190000000}
		default:
			return FrequencyList{196100000}
		}
	default:
		t.Fatalf("Unsupported PMD type: %v", p.PMD())
	}
	return nil
}

// TargetOpticalPowerList is a type for a list of target optical powers in float64 format.
type TargetOpticalPowerList []float64

// String returns the string representation of the list of target optical powers.
func (top *TargetOpticalPowerList) String() string {
	var s []string
	for _, v := range *top {
		s = append(s, fmt.Sprintf("%v", v))
	}
	return strings.Join(s, ",")
}

// Set sets the list of target optical powers from the string representation.
func (top *TargetOpticalPowerList) Set(value string) error {
	for _, s := range strings.Split(value, ",") {
		if v, err := strconv.ParseFloat(s, 64); err != nil {
			return err
		} else {
			*top = append(*top, v)
		}
	}
	return nil
}

// Get returns the list of target optical powers.
func (top *TargetOpticalPowerList) Get() any {
	return *top
}

// Default returns the default target optical power list.
func (top *TargetOpticalPowerList) Default(t *testing.T, dut *ondatra.DUTDevice) TargetOpticalPowerList {
	t.Helper()
	p := dut.Ports()[0]
	switch p.PMD() {
	case ondatra.PMD400GBASEZR:
		return TargetOpticalPowerList{-10}
	case ondatra.PMD400GBASEZRP:
		return TargetOpticalPowerList{-7}
	case ondatra.PMD800GBASEZR:
		return TargetOpticalPowerList{-7}
	case ondatra.PMD800GBASEZRP:
		return TargetOpticalPowerList{-4}
	default:
		t.Fatalf("Unsupported PMD type: %v", p.PMD())
	}
	return nil
}

// AssignOTNIndexes assigns the OTN indexes for the given ports.
func AssignOTNIndexes(t *testing.T, dut *ondatra.DUTDevice) map[string]uint32 {
	ports := dut.Ports()
	sort.Slice(ports, func(i, j int) bool {
		return ports[i].Name() < ports[j].Name()
	})
	otnIndexes := make(map[string]uint32)
	for idx, p := range ports {
		switch p.PMD() {
		case ondatra.PMD400GBASEZR, ondatra.PMD400GBASEZRP:
			otnIndexes[p.Name()] = 4000 + uint32(idx)
		case ondatra.PMD800GBASEZR, ondatra.PMD800GBASEZRP:
			otnIndexes[p.Name()] = 8000 + uint32(idx)
		default:
			t.Fatalf("Unsupported PMD type for %v", p.PMD())
		}
	}
	return otnIndexes
}

// AssignETHIndexes assigns the ETH indexes for the given ports.
func AssignETHIndexes(t *testing.T, dut *ondatra.DUTDevice) map[string]uint32 {
	ports := dut.Ports()
	sort.Slice(ports, func(i, j int) bool {
		return ports[i].Name() < ports[j].Name()
	})
	ethIndexes := make(map[string]uint32)
	for idx, p := range ports {
		switch p.PMD() {
		case ondatra.PMD400GBASEZR, ondatra.PMD400GBASEZRP:
			ethIndexes[p.Name()] = 40000 + uint32(idx)
		case ondatra.PMD800GBASEZR, ondatra.PMD800GBASEZRP:
			ethIndexes[p.Name()] = 80000 + uint32(idx)
		default:
			t.Fatalf("Unsupported PMD type for %v", p.PMD())
		}
	}
	return ethIndexes
}

// ConfigParameters contains the configuration parameters for the ports.
type ConfigParameters struct {
	Enabled             bool
	Frequency           uint64
	TargetOpticalPower  float64
	OperationalMode     uint16
	PortSpeed           oc.E_IfEthernet_ETHERNET_SPEED
	FormFactor          oc.E_TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE
	NumPhysicalChannels uint8
	RateClass           oc.E_TransportTypes_TRIBUTARY_RATE_CLASS_TYPE
	TribProtocol        oc.E_TransportTypes_TRIBUTARY_PROTOCOL_TYPE
	Allocation          float64
	HWPortNames         map[string]string
	TransceiverNames    map[string]string
	OpticalChannelNames map[string]string
	OTNIndexes          map[string]uint32
	ETHIndexes          map[string]uint32
}

// NewInterfaceConfigAll configures all the ports.
func NewInterfaceConfigAll(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, params *ConfigParameters) {
	t.Helper()
	params.HWPortNames = make(map[string]string)
	params.TransceiverNames = make(map[string]string)
	params.OpticalChannelNames = make(map[string]string)
	for _, p := range dut.Ports() {
		if hwPortName, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(p.Name()).HardwarePort().State()).Val(); !ok {
			t.Fatalf("Hardware port not found for %v", p.Name())
		} else {
			params.HWPortNames[p.Name()] = hwPortName
		}
		if transceiverName, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State()).Val(); !ok {
			t.Fatalf("Transceiver not found for %v", p.Name())
		} else {
			params.TransceiverNames[p.Name()] = transceiverName
		}
		params.OpticalChannelNames[p.Name()] = components.OpticalChannelComponentFromPort(t, dut, p)
		params.OTNIndexes = AssignOTNIndexes(t, dut)
		params.ETHIndexes = AssignETHIndexes(t, dut)
		switch p.PMD() {
		case ondatra.PMD400GBASEZR, ondatra.PMD400GBASEZRP:
			params.FormFactor = oc.TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE_QSFP56_DD
			params.PortSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_400GB
			params.NumPhysicalChannels = 8
			params.RateClass = oc.TransportTypes_TRIBUTARY_RATE_CLASS_TYPE_TRIB_RATE_400G
			params.TribProtocol = oc.TransportTypes_TRIBUTARY_PROTOCOL_TYPE_PROT_400GE
			params.Allocation = 400
		case ondatra.PMD800GBASEZR, ondatra.PMD800GBASEZRP:
			params.FormFactor = oc.TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE_OSFP
			switch params.OperationalMode {
			case 1, 2:
				params.PortSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_800GB
				params.NumPhysicalChannels = 8
				params.RateClass = oc.TransportTypes_TRIBUTARY_RATE_CLASS_TYPE_TRIB_RATE_800G
				params.TribProtocol = oc.TransportTypes_TRIBUTARY_PROTOCOL_TYPE_PROT_800GE
				params.Allocation = 800
			case 3, 4:
				params.PortSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_400GB
				params.NumPhysicalChannels = 4
				params.RateClass = oc.TransportTypes_TRIBUTARY_RATE_CLASS_TYPE_TRIB_RATE_400G
				params.TribProtocol = oc.TransportTypes_TRIBUTARY_PROTOCOL_TYPE_PROT_400GE
				params.Allocation = 400
			default:
				t.Fatalf("Unsupported operational mode for %v: %v", p.PMD(), params.OperationalMode)
			}
		default:
			t.Fatalf("Unsupported PMD type for %v", p.PMD())
		}
		updateInterfaceConfig(batch, p, params)
		updateHWPortConfig(batch, p, params)
		updateOpticalChannelConfig(batch, p, params)
		updateOTNChannelConfig(batch, dut, p, params)
		updateETHChannelConfig(batch, dut, p, params)
	}
}

// updateInterfaceConfig updates the interface config.
func updateInterfaceConfig(batch *gnmi.SetBatch, p *ondatra.Port, params *ConfigParameters) {
	i := &oc.Interface{
		Name:    ygot.String(p.Name()),
		Type:    oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		Enabled: ygot.Bool(params.Enabled),
	}
	if p.PMD() == ondatra.PMD800GBASEZR || p.PMD() == ondatra.PMD800GBASEZRP {
		i.Ethernet = &oc.Interface_Ethernet{
			PortSpeed:  params.PortSpeed,
			DuplexMode: oc.Ethernet_DuplexMode_FULL,
		}
	}
	gnmi.BatchReplace(batch, gnmi.OC().Interface(p.Name()).Config(), i)
}

// updateHWPortConfig updates the hardware port config.
func updateHWPortConfig(batch *gnmi.SetBatch, p *ondatra.Port, params *ConfigParameters) {
	if p.PMD() == ondatra.PMD400GBASEZR || p.PMD() == ondatra.PMD400GBASEZRP {
		return // No HwPort config for 400GZR/400GZR Plus.
	}
	gnmi.BatchReplace(batch, gnmi.OC().Component(params.HWPortNames[p.Name()]).Config(), &oc.Component{
		Name: ygot.String(params.HWPortNames[p.Name()]),
		Port: &oc.Component_Port{
			BreakoutMode: &oc.Component_Port_BreakoutMode{
				Group: map[uint8]*oc.Component_Port_BreakoutMode_Group{
					1: {
						Index:               ygot.Uint8(1),
						BreakoutSpeed:       params.PortSpeed,
						NumBreakouts:        ygot.Uint8(1),
						NumPhysicalChannels: ygot.Uint8(params.NumPhysicalChannels),
					},
				},
			},
		},
	})
}

// updateOpticalChannelConfig updates the optical channel config.
func updateOpticalChannelConfig(batch *gnmi.SetBatch, p *ondatra.Port, params *ConfigParameters) {

	gnmi.BatchReplace(batch, gnmi.OC().Component(params.OpticalChannelNames[p.Name()]).Config(), &oc.Component{
		Name: ygot.String(params.OpticalChannelNames[p.Name()]),
		OpticalChannel: &oc.Component_OpticalChannel{
			OperationalMode:   ygot.Uint16(params.OperationalMode),
			Frequency:         ygot.Uint64(params.Frequency),
			TargetOutputPower: ygot.Float64(params.TargetOpticalPower),
		},
	})
}

// updateOTNChannelConfig updates the OTN channel config.
func updateOTNChannelConfig(batch *gnmi.SetBatch, dut *ondatra.DUTDevice, p *ondatra.Port, params *ConfigParameters) {
	var firstAssignmentIndex uint32
	if deviations.OTNChannelAssignmentCiscoNumbering(dut) {
		firstAssignmentIndex = 1
	} else {
		firstAssignmentIndex = 0
	}
	if deviations.OTNToETHAssignment(dut) {
		gnmi.BatchReplace(batch, gnmi.OC().TerminalDevice().Channel(params.OTNIndexes[p.Name()]).Config(), &oc.TerminalDevice_Channel{
			Description:        ygot.String("OTN Logical Channel"),
			Index:              ygot.Uint32(params.OTNIndexes[p.Name()]),
			LogicalChannelType: oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_OTN,
			Assignment: map[uint32]*oc.TerminalDevice_Channel_Assignment{
				firstAssignmentIndex: {
					Index:          ygot.Uint32(firstAssignmentIndex),
					OpticalChannel: ygot.String(params.OpticalChannelNames[p.Name()]),
					Description:    ygot.String("OTN to Optical Channel"),
					Allocation:     ygot.Float64(params.Allocation),
					AssignmentType: oc.Assignment_AssignmentType_OPTICAL_CHANNEL,
				},
				firstAssignmentIndex + 1: {
					Index:          ygot.Uint32(firstAssignmentIndex + 1),
					LogicalChannel: ygot.Uint32(params.ETHIndexes[p.Name()]),
					Description:    ygot.String("OTN to ETH"),
					Allocation:     ygot.Float64(params.Allocation),
					AssignmentType: oc.Assignment_AssignmentType_LOGICAL_CHANNEL,
				},
			},
		})
	} else {
		gnmi.BatchReplace(batch, gnmi.OC().TerminalDevice().Channel(params.OTNIndexes[p.Name()]).Config(), &oc.TerminalDevice_Channel{
			Description:        ygot.String("OTN Logical Channel"),
			Index:              ygot.Uint32(params.OTNIndexes[p.Name()]),
			LogicalChannelType: oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_OTN,
			TribProtocol:       params.TribProtocol,
			AdminState:         oc.TerminalDevice_AdminStateType_ENABLED,
			Assignment: map[uint32]*oc.TerminalDevice_Channel_Assignment{
				firstAssignmentIndex: {
					Index:          ygot.Uint32(firstAssignmentIndex),
					OpticalChannel: ygot.String(params.OpticalChannelNames[p.Name()]),
					Description:    ygot.String("OTN to Optical Channel"),
					Allocation:     ygot.Float64(params.Allocation),
					AssignmentType: oc.Assignment_AssignmentType_OPTICAL_CHANNEL,
				},
			},
		})
	}
}

// updateETHChannelConfig updates the ETH channel config.
func updateETHChannelConfig(batch *gnmi.SetBatch, dut *ondatra.DUTDevice, p *ondatra.Port, params *ConfigParameters) {
	var assignmentIndex uint32
	if deviations.EthChannelAssignmentCiscoNumbering(dut) {
		assignmentIndex = 1
	} else {
		assignmentIndex = 0
	}
	var ingress *oc.TerminalDevice_Channel_Ingress
	if !deviations.EthChannelIngressParametersUnsupported(dut) {
		ingress = &oc.TerminalDevice_Channel_Ingress{
			Interface:   ygot.String(p.Name()),
			Transceiver: ygot.String(params.TransceiverNames[p.Name()]),
		}
	}
	assignment := map[uint32]*oc.TerminalDevice_Channel_Assignment{
		assignmentIndex: {
			Index:          ygot.Uint32(assignmentIndex),
			LogicalChannel: ygot.Uint32(params.OTNIndexes[p.Name()]),
			Description:    ygot.String("ETH to OTN"),
			Allocation:     ygot.Float64(params.Allocation),
			AssignmentType: oc.Assignment_AssignmentType_LOGICAL_CHANNEL,
		},
	}
	if deviations.EthChannelAssignmentCiscoNumbering(dut) {
		assignment[0].Index = ygot.Uint32(1)
	}
	channel := &oc.TerminalDevice_Channel{
		Description:        ygot.String("ETH Logical Channel"),
		Index:              ygot.Uint32(params.ETHIndexes[p.Name()]),
		LogicalChannelType: oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_ETHERNET,
		Ingress:            ingress,
		Assignment:         assignment,
	}
	if !deviations.ChannelRateClassParametersUnsupported(dut) {
		channel.RateClass = params.RateClass
	}
	if !deviations.OTNChannelTribUnsupported(dut) {
		channel.TribProtocol = params.TribProtocol
		channel.AdminState = oc.TerminalDevice_AdminStateType_ENABLED
	}
	gnmi.BatchReplace(batch, gnmi.OC().TerminalDevice().Channel(params.ETHIndexes[p.Name()]).Config(), channel)
}

// ToggleInterfaceState toggles the interface with operational mode.
func ToggleInterfaceState(t *testing.T, p *ondatra.Port, params *ConfigParameters) {
	i := &oc.Interface{
		Name:    ygot.String(p.Name()),
		Type:    oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		Enabled: ygot.Bool(params.Enabled),
	}
	if p.PMD() == ondatra.PMD800GBASEZR || p.PMD() == ondatra.PMD800GBASEZRP {
		i.Ethernet = &oc.Interface_Ethernet{
			PortSpeed:  params.PortSpeed,
			DuplexMode: oc.Ethernet_DuplexMode_FULL,
		}
	}
	gnmi.Replace(t, p.Device(), gnmi.OC().Interface(p.Name()).Config(), i)
}

// InterfaceInitialize assigns OpMode with value received through operationalMode flag.
func InterfaceInitialize(t *testing.T, dut *ondatra.DUTDevice, initialOperationalMode uint16) uint16 {
	once.Do(func() {
		t.Helper()
		if initialOperationalMode == 0 { // '0' signals to use vendor-specific default
			switch dut.Vendor() {
			case ondatra.CISCO:
				opmode = 5003
				t.Logf("cfgplugins.Initialize: Cisco DUT, setting opmode to default: %d", opmode)
			case ondatra.ARISTA:
				opmode = 1
				t.Logf("cfgplugins.Initialize: Arista DUT, setting opmode to default: %d", opmode)
			case ondatra.JUNIPER:
				opmode = 1
				t.Logf("cfgplugins.Initialize: Juniper DUT, setting opmode to default: %d", opmode)
			case ondatra.NOKIA:
				opmode = 1083
				t.Logf("cfgplugins.Initialize: Nokia DUT, setting opmode to default: %d", opmode)
			default:
				opmode = 1
				t.Logf("cfgplugins.Initialize: Using global default opmode: %d", opmode)
			}
		} else {
			opmode = initialOperationalMode
			t.Logf("cfgplugins.Initialize: Using provided initialOperationalMode: %d", opmode)
		}
		t.Logf("cfgplugins.Initialize: Initialization complete. Final opmode set to: %d", opmode)
	})
	return InterfaceGetOpMode()
}

// InterfaceGetOpMode returns the opmode value after the Initialize function has been called
func InterfaceGetOpMode() uint16 {
	return opmode
}

// InterfaceConfig configures the interface with the given port.
func InterfaceConfig(t *testing.T, dut *ondatra.DUTDevice, dp *ondatra.Port) {
	t.Helper()
	d := &oc.Root{}
	i := d.GetOrCreateInterface(dp.Name())
	i.Enabled = ygot.Bool(true)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	gnmi.Replace(t, dut, gnmi.OC().Interface(dp.Name()).Config(), i)
	if deviations.ExplicitDcoConfig(dut) {
		transceiverName := gnmi.Get(t, dut, gnmi.OC().Interface(dp.Name()).Transceiver().State())
		gnmi.Replace(t, dut, gnmi.OC().Component(transceiverName).Config(), &oc.Component{
			Name: ygot.String(transceiverName),
			Transceiver: &oc.Component_Transceiver{
				ModuleFunctionalType: oc.TransportTypes_TRANSCEIVER_MODULE_FUNCTIONAL_TYPE_TYPE_DIGITAL_COHERENT_OPTIC,
			},
		})
	}
	oc := components.OpticalChannelComponentFromPort(t, dut, dp)
	ConfigOpticalChannel(t, dut, oc, targetFrequencyMHz, targetOutputPowerdBm, opmode)
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
	gnmi.Replace(t, dut, gnmi.OC().Component(och).Config(), &oc.Component{
		Name: ygot.String(och),
		OpticalChannel: &oc.Component_OpticalChannel{
			OperationalMode:   ygot.Uint16(operationalMode),
			Frequency:         ygot.Uint64(frequency),
			TargetOutputPower: ygot.Float64(targetOpticalPower),
		},
	})
}

// ConfigOTNChannel configures the OTN channel.
func ConfigOTNChannel(t *testing.T, dut *ondatra.DUTDevice, och string, otnIndex, ethIndex uint32) {
	t.Helper()
	t.Logf(" otnIndex:%v, ethIndex: %v", otnIndex, ethIndex)
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
			AdminState:         oc.TerminalDevice_AdminStateType_ENABLED,
			Assignment: map[uint32]*oc.TerminalDevice_Channel_Assignment{
				0: {
					Index:          ygot.Uint32(0),
					OpticalChannel: ygot.String(och),
					Description:    ygot.String("OTN to Optical Channel"),
					Allocation:     ygot.Float64(400),
					AssignmentType: oc.Assignment_AssignmentType_OPTICAL_CHANNEL,
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
		AdminState:         oc.TerminalDevice_AdminStateType_ENABLED,
	}
	if !deviations.ChannelRateClassParametersUnsupported(dut) {
		channel.RateClass = oc.TransportTypes_TRIBUTARY_RATE_CLASS_TYPE_TRIB_RATE_400G
	}
	gnmi.Replace(t, dut, gnmi.OC().TerminalDevice().Channel(ethIndex).Config(), channel)
}

// SetupAggregateAtomically sets up the aggregate interface atomically.
func SetupAggregateAtomically(t *testing.T, dut *ondatra.DUTDevice, aggID string, dutAggPorts []*ondatra.Port) {
	d := &oc.Root{}

	d.GetOrCreateLacp().GetOrCreateInterface(aggID)

	agg := d.GetOrCreateInterface(aggID)
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_LACP
	agg.Type = ieee8023adLag

	for _, port := range dutAggPorts {
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(aggID)
		i.Type = ethernetCsmacd

		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
	}

	p := gnmi.OC()
	fptest.LogQuery(t, fmt.Sprintf("%s to Update()", dut), p.Config(), d)
	gnmi.Update(t, dut, p.Config(), d)
}

// DeleteAggregate deletes the aggregate interface.
func DeleteAggregate(t *testing.T, dut *ondatra.DUTDevice, aggID string, dutAggPorts []*ondatra.Port) {
	// Clear the aggregate minlink.
	gnmi.Delete(t, dut, gnmi.OC().Interface(aggID).Aggregation().MinLinks().Config())

	// Clear the members of the aggregate.
	for _, port := range dutAggPorts {
		gnmi.Delete(t, dut, gnmi.OC().Interface(port.Name()).Ethernet().AggregateId().Config())
	}
}
