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
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
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

// DUTSubInterfaceData is the data structure for a subinterface in the DUT.
type DUTSubInterfaceData struct {
	VlanID        int
	IPv4Address   net.IP
	IPv6Address   net.IP
	IPv4PrefixLen int
	IPv6PrefixLen int
}

// LACPParams is the data structure for the LACP parameters used in the DUTLagData.
type LACPParams struct {
	Activity *oc.E_Lacp_LacpActivityType
	Period   *oc.E_Lacp_LacpPeriodType
}

// DUTAggData is the data structure for a LAG in the DUT.
type DUTAggData struct {
	attrs.Attributes
	SubInterfaces   []*DUTSubInterfaceData
	OndatraPortsIdx []int
	OndatraPorts    []*ondatra.Port
	LagName         string
	LacpParams      *LACPParams
	AggType         oc.E_IfAggregate_AggregationType
}

// Attributes is a type for the attributes of a port.
type Attributes struct {
	*attrs.Attributes
	NumSubIntf       uint32
	Index            uint8
	AteISISSysID     string
	V4Route          func(vlan int) string
	V4ISISRouteCount uint32
	V6Route          func(vlan int) string
	V6ISISRouteCount uint32
	Ip4              func(vlan int) (string, string)
	Ip6              func(vlan int) (string, string)
	Gateway          func(vlan int) (string, string)
	Gateway6         func(vlan int) (string, string)
	Ip4Loopback      func(vlan int) (string, string)
	Ip6Loopback      func(vlan int) (string, string)
	LagMAC           string
	EthMAC           string
	Port1MAC         string
	Pg4              string
	Pg6              string
}

// PopulateOndatraPorts populates the OndatraPorts field of the DutLagData from the OndatraPortsIdx
// field.
func (d *DUTAggData) PopulateOndatraPorts(t *testing.T, dut *ondatra.DUTDevice) {
	for _, v := range d.OndatraPortsIdx {
		d.OndatraPorts = append(d.OndatraPorts, dut.Port(t, "port"+strconv.Itoa(v+1)))
	}
}

var (
	opmode   uint16
	once     sync.Once
	lBandPNs = map[string]bool{
		"DP04QSDD-LLH-240": true, // Cisco QSFPDD Acacia 400G ZRP L-Band
		"DP04QSDD-LLH-24B": true, // Cisco QSFPDD Acacia 400G ZRP L-Band
		"DP04QSDD-LLH-00A": true, // Cisco QSFPDD Acacia 400G ZRP L-Band
		"DP08SFP8-LRB-240": true, // Cisco OSFP Acacia 800G ZRP L-Band
		"DP08SFP8-LRB-24B": true, // Cisco OSFP Acacia 800G ZRP L-Band
		"C-OS08LEXNC-GG":   true, // Nokia OSFP 800G ZRP L-Band
		"176-6490-9G1":     true, // Ciena OSFP 800G ZRP L-Band
		"176-6480-9M0":     true, // Ciena OSFP 800G ZRP L-Band
	}
)

// OperationalModeList is a type for a list of operational modes in uint16 format.
type OperationalModeList []uint16

// StaticAggregateConfig defines the parameters for configuring a static LAG.
type StaticAggregateConfig struct {
	AggID    string
	DutLag   attrs.Attributes
	AggPorts []*ondatra.Port
}

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
		case ondatra.ARISTA, ondatra.JUNIPER, ondatra.NOKIA:
			return OperationalModeList{1}
		default:
			t.Fatalf("Unsupported vendor: %v", dut.Vendor())
		}
	case ondatra.PMD400GBASEZRP: // 400G (8x56G)
		switch dut.Vendor() {
		case ondatra.CISCO:
			return OperationalModeList{6004}
		case ondatra.JUNIPER, ondatra.ARISTA, ondatra.NOKIA:
			return OperationalModeList{5}
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
	TempSensorNames     map[string]string
	OpticalChannelNames map[string]string
	OTNIndexes          map[string]uint32
	ETHIndexes          map[string]uint32
}

// NewInterfaceConfigAll configures all the ports.
func NewInterfaceConfigAll(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, params *ConfigParameters) {
	t.Helper()
	params.HWPortNames = make(map[string]string)
	params.TransceiverNames = make(map[string]string)
	params.TempSensorNames = make(map[string]string)
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
		params.TempSensorNames[p.Name()] = "TempSensor-" + params.TransceiverNames[p.Name()]
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
			case 1, 2, 8:
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
		updateInterfaceConfig(batch, dut, p, params)
		updateHWPortConfig(batch, dut, p, params)
		updateTransceiverConfig(batch, dut, p, params)
		updateOpticalChannelConfig(batch, p, params)
		updateOTNChannelConfig(batch, dut, p, params)
		updateETHChannelConfig(batch, dut, p, params)
	}
}

// updateInterfaceConfig updates the interface config.
func updateInterfaceConfig(batch *gnmi.SetBatch, dut *ondatra.DUTDevice, p *ondatra.Port, params *ConfigParameters) {
	i := &oc.Interface{
		Name:    ygot.String(p.Name()),
		Type:    oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		Enabled: ygot.Bool(params.Enabled),
	}
	switch {
	case deviations.PortSpeedDuplexModeUnsupportedForInterfaceConfig(dut):
		// No port speed and duplex mode config for devices that do not support it.
	case p.PMD() == ondatra.PMD400GBASEZR || p.PMD() == ondatra.PMD400GBASEZRP:
		// No port speed and duplex mode config for 400GZR/400GZR Plus as it is not supported.
	default:
		i.Ethernet = &oc.Interface_Ethernet{
			PortSpeed:  params.PortSpeed,
			DuplexMode: oc.Ethernet_DuplexMode_FULL,
		}
	}
	gnmi.BatchReplace(batch, gnmi.OC().Interface(p.Name()).Config(), i)
}

// updateHWPortConfig updates the hardware port config.
func updateHWPortConfig(batch *gnmi.SetBatch, dut *ondatra.DUTDevice, p *ondatra.Port, params *ConfigParameters) {
	switch {
	case deviations.BreakoutModeUnsupportedForEightHundredGb(dut) && params.PortSpeed == oc.IfEthernet_ETHERNET_SPEED_SPEED_800GB:
		return
	case p.PMD() == ondatra.PMD400GBASEZR || p.PMD() == ondatra.PMD400GBASEZRP:
		// No breakout mode config for 400GZR/400GZR Plus as it is not supported.
		return
	default:
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
	if deviations.ExplicitBreakoutInterfaceConfig(dut) {
		gnmi.BatchReplace(batch, gnmi.OC().Interface(p.Name()+"/1").Config(), &oc.Interface{
			Name:    ygot.String(p.Name() + "/1"),
			Type:    oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
			Enabled: ygot.Bool(params.Enabled),
		})
	}
}

// updateTransceiverConfig updates the transceiver config.
func updateTransceiverConfig(batch *gnmi.SetBatch, dut *ondatra.DUTDevice, p *ondatra.Port, params *ConfigParameters) {
	if !deviations.ExplicitDcoConfig(dut) {
		return // No transceiver config for devices that do not require explicit DCO config.
	}
	gnmi.BatchReplace(batch, gnmi.OC().Component(params.TransceiverNames[p.Name()]).Config(), &oc.Component{
		Name: ygot.String(params.TransceiverNames[p.Name()]),
		Transceiver: &oc.Component_Transceiver{
			ModuleFunctionalType: oc.TransportTypes_TRANSCEIVER_MODULE_FUNCTIONAL_TYPE_TYPE_DIGITAL_COHERENT_OPTIC,
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
func ToggleInterfaceState(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port, params *ConfigParameters) {
	i := &oc.Interface{
		Name:    ygot.String(p.Name()),
		Type:    oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		Enabled: ygot.Bool(params.Enabled),
	}
	switch {
	case deviations.PortSpeedDuplexModeUnsupportedForInterfaceConfig(dut):
		// No port speed and duplex mode config for devices that do not support it.
	case p.PMD() == ondatra.PMD400GBASEZR || p.PMD() == ondatra.PMD400GBASEZRP:
		// No port speed and duplex mode config for 400GZR/400GZR Plus as it is not supported.
	default:
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
			var oml OperationalModeList
			defaultOpModes := oml.Default(t, dut)
			if len(defaultOpModes) == 0 {
				t.Fatalf("No default operational mode found for vendor %v and PMD %v", dut.Vendor(), dut.Ports()[0].PMD())
			}
			opmode = defaultOpModes[0]
			t.Logf("cfgplugins.Initialize: Setting opmode to default: %d", opmode)
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

// OpticalChannelOpt is an option for ConfigOpticalChannel.
type OpticalChannelOpt func(*oc.Component_OpticalChannel)

// WithLinePort sets the line-port for the optical channel if supported by the DUT.
func WithLinePort(dut *ondatra.DUTDevice, och string) OpticalChannelOpt {
	return func(oc *oc.Component_OpticalChannel) {
		if !deviations.LinePortUnsupported(dut) {
			linePort := strings.ReplaceAll(och, "OpticalChannel", "Optics")
			oc.LinePort = ygot.String(linePort)
		}
	}
}

// ConfigOpticalChannel configures the optical channel.
func ConfigOpticalChannel(t *testing.T, dut *ondatra.DUTDevice, och string, frequency uint64, targetOpticalPower float64, operationalMode uint16, opts ...OpticalChannelOpt) {
	opticalChannel := &oc.Component_OpticalChannel{
		OperationalMode:   ygot.Uint16(operationalMode),
		Frequency:         ygot.Uint64(frequency),
		TargetOutputPower: ygot.Float64(targetOpticalPower),
	}
	for _, opt := range opts {
		opt(opticalChannel)
	}
	if opticalChannel.GetLinePort() != "" {
		t.Logf("LinePort was configured for optical channel %s: %s", och, opticalChannel.GetLinePort())
	}
	gnmi.Replace(t, dut, gnmi.OC().Component(och).Config(), &oc.Component{
		Name:           ygot.String(och),
		OpticalChannel: opticalChannel,
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

// SetupStaticAggregateAtomically sets up the static aggregate interface atomically.
func SetupStaticAggregateAtomically(t *testing.T, dut *ondatra.DUTDevice, aggrBatch *gnmi.SetBatch, cfg StaticAggregateConfig) *oc.Interface {
	t.Helper()
	// Create LAG
	agg := cfg.DutLag.NewOCInterface(cfg.AggID, dut)
	agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC
	gnmi.BatchReplace(aggrBatch, gnmi.OC().Interface(cfg.AggID).Config(), agg)

	// Create all member ports
	for _, port := range cfg.AggPorts {
		d := &oc.Root{}
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(cfg.AggID)
		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
		gnmi.BatchReplace(aggrBatch, gnmi.OC().Interface(port.Name()).Config(), i)
	}
	return agg
}

// AddPortToAggregate adds an Ondatra port as a member to the aggregate interface.
func AddPortToAggregate(t *testing.T, dut *ondatra.DUTDevice, aggID string, dutAggPorts []*ondatra.Port, b *gnmi.SetBatch, op *ondatra.Port) {
	gnmi.BatchDelete(b, gnmi.OC().Interface(op.Name()).Ethernet().AggregateId().Config())

	d := &oc.Root{}
	i := d.GetOrCreateInterface(op.Name())
	i.Description = ygot.String("LAG - Member - " + op.Name())
	e := i.GetOrCreateEthernet()
	e.AggregateId = ygot.String(aggID)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	if op.PMD() == ondatra.PMD100GBASEFR && deviations.ExplicitPortSpeed(dut) {
		e.AutoNegotiate = ygot.Bool(false)
		e.DuplexMode = oc.Ethernet_DuplexMode_FULL
		e.PortSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB
	}
	gnmi.BatchReplace(b, gnmi.OC().Interface(op.Name()).Config(), i)
}

// AddSubInterface adds a subinterface to an interface.
func AddSubInterface(t *testing.T, dut *ondatra.DUTDevice, b *gnmi.SetBatch, i *oc.Interface, s *DUTSubInterfaceData) {
	sub := i.GetOrCreateSubinterface(uint32(s.VlanID))
	sub.Enabled = ygot.Bool(true)
	if s.VlanID != 0 {
		if deviations.DeprecatedVlanID(dut) {
			sub.GetOrCreateVlan().VlanId = oc.UnionUint16(int(s.VlanID))
		} else {
			sub.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(uint16(s.VlanID))
		}
	}
	if s.IPv4Address == nil && s.IPv6Address == nil {
		t.Fatalf("No IPv4 or IPv6 address found for  %s or a subinterface under this lag", i.GetName())
	}
	if s.IPv4Address != nil {
		sub.GetOrCreateIpv4().GetOrCreateAddress(s.IPv4Address.String()).PrefixLength = ygot.Uint8(uint8(s.IPv4PrefixLen))
		if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
			sub.GetOrCreateIpv4().SetEnabled(true)
		}
	}
	if s.IPv6Address != nil {
		sub.GetOrCreateIpv6().GetOrCreateAddress(s.IPv6Address.String()).PrefixLength = ygot.Uint8(uint8(s.IPv6PrefixLen))
		if deviations.IPv4MissingEnabled(dut) {
			sub.GetOrCreateIpv6().SetEnabled(true)
		}
	}
	gnmi.BatchReplace(b, gnmi.OC().Interface(i.GetName()).Subinterface(uint32(s.VlanID)).Config(), sub)
}

// NewAggregateInterface creates the below configuration for the aggregate interface:
// 1. Create a new aggregate interface
// 2. LACP configuration
// 3. Adds member Ports configuration to an aggregate interface
// 4. Subinterface configuration including thier IP address and VLAN ID
// Note that you will still need to push the batch config to the DUT in your code.
func NewAggregateInterface(t *testing.T, dut *ondatra.DUTDevice, b *gnmi.SetBatch, l *DUTAggData) *oc.Interface {
	aggID := l.LagName
	agg := l.NewOCInterface(aggID, dut)
	agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	if !deviations.IPv4MissingEnabled(dut) && len(l.SubInterfaces) == 0 {
		agg.GetSubinterface(0).GetOrCreateIpv4().SetEnabled(true)
		agg.GetSubinterface(0).GetOrCreateIpv6().SetEnabled(true)
	}
	agg.GetOrCreateAggregation().LagType = l.AggType
	gnmi.BatchReplace(b, gnmi.OC().Interface(aggID).Config(), agg)

	// Set LACP mode to ACTIVE for the LAG interface
	if l.LacpParams != nil {
		if l.LacpParams.Activity == nil || l.LacpParams.Period == nil {
			t.Fatalf("LACP activity or period is not set for LAG %s", aggID)
		}
		lacp := &oc.Lacp_Interface{Name: ygot.String(aggID)}
		lacp.LacpMode = *l.LacpParams.Activity
		lacp.Interval = *l.LacpParams.Period
		lacpPath := gnmi.OC().Lacp().Interface(aggID)
		gnmi.BatchReplace(b, lacpPath.Config(), lacp)
	}
	gnmi.BatchDelete(b, gnmi.OC().Interface(aggID).Aggregation().MinLinks().Config())

	l.PopulateOndatraPorts(t, dut)
	for _, op := range l.OndatraPorts {
		AddPortToAggregate(t, dut, aggID, l.OndatraPorts, b, op)
	}

	if l.Attributes.IPv4 == "" && l.Attributes.IPv6 == "" {
		if !deviations.InterfaceEnabled(dut) {
			// TODO : Need to investigate if this a real diviation or not as it is not clear openconfig
			agg.DeleteSubinterface(0)
			gnmi.BatchReplace(b, gnmi.OC().Interface(aggID).Config(), agg)
		}
		for _, i := range l.SubInterfaces {
			if i.VlanID == 0 {
				t.Fatalf("No VLAN ID found for a subinterface under lag %s", aggID)
			}
			AddSubInterface(t, dut, b, agg, i)
		}
	}
	return agg
}

// NewSubInterfaces creates the below configuration for the subinterfaces:
func NewSubInterfaces(t *testing.T, dut *ondatra.DUTDevice, dutPorts []Attributes) {
	t.Helper()
	for _, dutPort := range dutPorts {
		dutPort.configInterfaceDUT(t, dut)
		dutPort.assignSubifsToDefaultNetworkInstance(t, dut)
	}
}

// configInterfaceDUT configures the DUT with interface and subinterfaces.
func (a *Attributes) configInterfaceDUT(t *testing.T, d *ondatra.DUTDevice) {
	t.Helper()
	p := d.Port(t, a.Name)
	portName := p.Name()

	if a.NumSubIntf > 1 && d.Vendor() == ondatra.ARISTA && d.Model() == "ceos" {
		cliConfig := fmt.Sprintf("interface %s\n no switchport \n", portName)
		helpers.GnmiCLIConfig(t, d, cliConfig)
		t.Logf("Applied Arista cEOS specific config for %s: %s", portName, cliConfig)
	}

	var i *oc.Interface
	if a.NumSubIntf > 1 {
		i = &oc.Interface{
			Name:        ygot.String(portName),
			Description: ygot.String(a.Desc),
			Type:        oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
			Enabled:     ygot.Bool(true),
		}
	} else {
		i = a.NewOCInterface(portName, d)
		i.Enabled = ygot.Bool(true)
	}

	ApplyEthernetConfig(t, i, p, d)

	if a.NumSubIntf == 1 {
		ip4, eMsg := a.Ip4(1)
		if eMsg != "" {
			t.Fatalf("Error in fetching IPV4 address for port %s: %s", a.Name, eMsg)
		}
		ip6, eMsg := a.Ip6(1)
		if eMsg != "" {
			t.Fatalf("Error in fetching IPV6 address for port %s: %s", a.Name, eMsg)
		}
		if deviations.RequireRoutedSubinterface0(d) {
			EnsureRoutedSubinterface0(i, d, ip4, ip6, a.IPv4Len, a.IPv6Len)
		}
	} else {
		// Configure subinterfaces 1..n for multi-subinterface cases
		for idx := 1; idx <= int(a.NumSubIntf); idx++ {
			subIntfIndex := uint32(a.Index*10) + uint32(idx)
			s := i.GetOrCreateSubinterface(subIntfIndex)
			a.configureSubinterface(t, s, d, idx)
		}
	}

	t.Logf("Configuring interface %s on DUT %s", portName, d.ID())
	intfPath := gnmi.OC().Interface(portName)
	gnmi.Replace(t, d, intfPath.Config(), i)
}

// configureSubinterface is a helper to configure a single subinterface object.
func (a *Attributes) configureSubinterface(t *testing.T, s *oc.Interface_Subinterface, dut *ondatra.DUTDevice, subIndex int) {
	t.Helper()

	if deviations.InterfaceEnabled(dut) {
		s.Enabled = ygot.Bool(true)
	}

	vlanID := uint16(int(a.Index*10) + subIndex)
	ConfigureVLAN(s, dut, vlanID)

	ipv4Addr, eMsg := a.Ip4(subIndex)
	if eMsg != "" {
		t.Fatalf("Error in fetching IPV4 address for port %s: %s", a.Name, eMsg)
	}
	ipv6Addr, eMsg := a.Ip6(subIndex)
	if eMsg != "" {
		t.Fatalf("Error in fetching IPV6 address for port %s: %s", a.Name, eMsg)
	}
	ConfigureSubinterfaceIPs(s, dut, ipv4Addr, a.IPv4Len, ipv6Addr, a.IPv6Len)
}

// ApplyEthernetConfig configures Ethernet-specific settings like port speed and duplex.
func ApplyEthernetConfig(t *testing.T, i *oc.Interface, p *ondatra.Port, d *ondatra.DUTDevice) {
	t.Helper()
	if p.PMD() == ondatra.PMD100GBASEFR && deviations.ExplicitPortSpeed(d) {
		eth := i.GetOrCreateEthernet()
		speed := fptest.GetIfSpeed(t, p)
		eth.PortSpeed = speed
		eth.AutoNegotiate = ygot.Bool(false)
		eth.DuplexMode = oc.Ethernet_DuplexMode_FULL
		t.Logf("Applied Ethernet config for %s: Speed %v, AutoNegotiate False, Duplex Full", i.GetName(), speed)
	}
}

// EnsureRoutedSubinterface0 creates and enables IPv4/IPv6 on subinterface 0 if required by deviations.
func EnsureRoutedSubinterface0(i *oc.Interface, d *ondatra.DUTDevice, ipv4Addr string, ipv6Addr string, ipv4Prefix uint8, ipv6Prefix uint8) {
	s4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	s4.Enabled = ygot.Bool(true)
	s6 := i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
	s6.Enabled = ygot.Bool(true)
}

// ConfigureVLAN configures VLAN settings for a subinterface.
func ConfigureVLAN(s *oc.Interface_Subinterface, dut *ondatra.DUTDevice, vlanID uint16) {
	if deviations.DeprecatedVlanID(dut) {
		id := vlanID
		if id > 256 {
			id++
		}
		s.GetOrCreateVlan().VlanId = oc.UnionUint16(id)
	} else {
		s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(vlanID)
	}
}

// ConfigureSubinterfaceIPs configures IPv4 and IPv6 addresses for a subinterface.
func ConfigureSubinterfaceIPs(s *oc.Interface_Subinterface, dut *ondatra.DUTDevice, ipv4Addr string, ipv4Prefix uint8, ipv6Addr string, ipv6Prefix uint8) {
	// IPv4 Configuration
	if ipv4Addr != "" {
		s4 := s.GetOrCreateIpv4()
		if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
			s4.Enabled = ygot.Bool(true)
		}
		s4a := s4.GetOrCreateAddress(ipv4Addr)
		s4a.PrefixLength = ygot.Uint8(ipv4Prefix)
	}

	// IPv6 Configuration
	if ipv6Addr != "" {
		s6 := s.GetOrCreateIpv6()
		if deviations.InterfaceEnabled(dut) {
			s6.Enabled = ygot.Bool(true)
		}
		s6a := s6.GetOrCreateAddress(ipv6Addr)
		s6a.PrefixLength = ygot.Uint8(ipv6Prefix)
	}
}

// assignSubifsToDefaultNetworkInstance assigns the subinterfaces to the default network instance.
func (a *Attributes) assignSubifsToDefaultNetworkInstance(t *testing.T, d *ondatra.DUTDevice) {
	if !deviations.ExplicitInterfaceInDefaultVRF(d) {
		return
	}
	t.Helper()

	p := d.Port(t, a.Name)
	instanceName := deviations.DefaultNetworkInstance(d)
	portName := p.Name()

	assignFunc := fptest.AssignToNetworkInstance

	if a.NumSubIntf == 1 {
		assignFunc(t, d, portName, instanceName, 0)
	} else {
		for i := uint32(1); i <= a.NumSubIntf; i++ {
			subIntfIndex := uint32(a.Index*10) + i
			assignFunc(t, d, portName, instanceName, subIntfIndex)
		}
	}
}

// AddInterfaceMTUOps adds gNMI operations for interface MTU to the batch.
func AddInterfaceMTUOps(b *gnmi.SetBatch, dut *ondatra.DUTDevice, intfName string, mtu uint16, isDelete bool) {
	intf := gnmi.OC().Interface(intfName)

	if deviations.OmitL2MTU(dut) {
		ipv4MtuPath := intf.Subinterface(0).Ipv4().Mtu()
		ipv6MtuPath := intf.Subinterface(0).Ipv6().Mtu()
		if isDelete {
			gnmi.BatchDelete(b, ipv4MtuPath.Config())
			gnmi.BatchDelete(b, ipv6MtuPath.Config())
		} else {
			gnmi.BatchReplace(b, ipv4MtuPath.Config(), mtu)
			gnmi.BatchReplace(b, ipv6MtuPath.Config(), uint32(mtu))
		}
	} else {
		mtuPath := intf.Mtu()
		if isDelete {
			gnmi.BatchDelete(b, mtuPath.Config())
		} else {
			gnmi.BatchReplace(b, mtuPath.Config(), mtu)
		}
	}
}

// StaticARPEntry defines per-port static ARP mapping.
type StaticARPEntry struct {
	PortName string // DUT port name (e.g., "port2")
	MagicIP  string // Per-port IP (e.g., "192.0.2.1")
	MagicMAC string // Per-port MAC (e.g., "00:1A:2B:3C:4D:5E")
}

// StaticARPConfig holds all per-port static ARP entries.
type StaticARPConfig struct {
	Entries []StaticARPEntry
}

// StaticARPWithMagicUniversalIP configures static ARP and static routes per-port.
func StaticARPWithMagicUniversalIP(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, cfg StaticARPConfig) *gnmi.SetBatch {
	t.Helper()

	// Group entries by MagicIP so each prefix can have multiple next-hops.
	entriesByIP := make(map[string][]StaticARPEntry)
	for _, entry := range cfg.Entries {
		entriesByIP[entry.MagicIP] = append(entriesByIP[entry.MagicIP], entry)
	}

	for magicIP, entries := range entriesByIP {
		// 1. Build all next-hops for this MagicIP.
		nextHops := make(map[string]*oc.NetworkInstance_Protocol_Static_NextHop)
		for i, entry := range entries {
			port := dut.Port(t, entry.PortName)
			nextHops[strconv.Itoa(i)] = &oc.NetworkInstance_Protocol_Static_NextHop{
				Index: ygot.String(strconv.Itoa(i)),
				InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
					Interface: ygot.String(port.Name()),
				},
			}
		}

		// 2. Define the static route with all built next-hops.
		s := &oc.NetworkInstance_Protocol_Static{
			Prefix:  ygot.String(magicIP + "/32"),
			NextHop: nextHops,
		}

		// Add static route config to batch.
		sp := gnmi.OC().
			NetworkInstance(deviations.DefaultNetworkInstance(dut)).
			Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
		gnmi.BatchUpdate(sb, sp.Static(magicIP+"/32").Config(), s)

		// 3. Add static ARP entries separately for each port.
		for _, entry := range entries {
			port := dut.Port(t, entry.PortName)
			gnmi.BatchUpdate(sb,
				gnmi.OC().Interface(port.Name()).Config(),
				configStaticArp(port.Name(), entry.MagicIP, entry.MagicMAC),
			)

			t.Logf("Configuring ARP: port=%s, ip=%s, mac=%s",
				entry.PortName, entry.MagicIP, entry.MagicMAC)
		}

		t.Logf("Configuring static route for MagicIP %s with %d next-hops", magicIP, len(entries))
	}

	return sb
}

// SecondaryIPEntry defines per-port dummy IP + ARP mapping for secondary IP config.
type SecondaryIPEntry struct {
	PortName      string           // DUT port name (e.g., "port2")
	PortDummyAttr attrs.Attributes //  DUT dummy IP attributes
	DummyIP       string           // OTG Dummy IPv4 address (e.g., "192.0.2.10")
	MagicMAC      string           // MAC to use for static ARP (e.g., "00:1A:2B:3C:4D:FF")

}

// SecondaryIPConfig holds all per-port secondary IP configurations.
type SecondaryIPConfig struct {
	Entries []SecondaryIPEntry
}

// StaticARPWithSecondaryIP configures secondary IPs and static ARP for gRIBI compatibility
func StaticARPWithSecondaryIP(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, cfg SecondaryIPConfig) *gnmi.SetBatch {

	t.Helper()

	for _, entry := range cfg.Entries {
		port := dut.Port(t, entry.PortName)

		// Configure secondary IP on the DUT port.
		gnmi.BatchUpdate(sb, gnmi.OC().Interface(port.Name()).Config(), entry.PortDummyAttr.NewOCInterface(port.Name(), dut))

		// Configure static ARP entry.
		gnmi.BatchUpdate(sb,
			gnmi.OC().Interface(port.Name()).Config(),
			configStaticArp(port.Name(), entry.DummyIP, entry.MagicMAC),
		)

		t.Logf("Configuring secondary IP + static ARP: port=%s, dummyIP=%s, mac=%s",
			entry.PortName, entry.DummyIP, entry.MagicMAC)
	}
	return sb
}

// configStaticArp configures static ARP entries for gRIBI next hop resolution
func configStaticArp(p string, ipv4addr string, macAddr string) *oc.Interface {
	i := &oc.Interface{Name: ygot.String(p)}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	n4 := s4.GetOrCreateNeighbor(ipv4addr)
	n4.LinkLayerAddress = ygot.String(macAddr)
	return i
}

// URPFConfigParams holds all parameters required to configure Unicast Reverse Path Forwarding (uRPF) on a DUT interface. It includes the interface name and its IPv4/IPv6 subinterface objects.
type URPFConfigParams struct {
	InterfaceName string
	IPv4Obj       *oc.Interface_Subinterface_Ipv4
	IPv6Obj       *oc.Interface_Subinterface_Ipv6
}

// ConfigureURPFonDutInt configures URPF on the interface.
func ConfigureURPFonDutInt(t *testing.T, dut *ondatra.DUTDevice, cfg URPFConfigParams) {
	t.Helper()
	if deviations.URPFConfigOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			urpfCliConfig := fmt.Sprintf(`
			interface %s
			ip verify unicast source reachable-via any
			ipv6 verify unicast source reachable-via any
			`, cfg.InterfaceName)
			helpers.GnmiCLIConfig(t, dut, urpfCliConfig)
		default:
			t.Fatalf("Unsupported vendor: %v", dut.Vendor())
		}
	} else {
		cfg.IPv4Obj.GetOrCreateUrpf()
		cfg.IPv4Obj.Urpf.Enabled = ygot.Bool(true)
		cfg.IPv4Obj.Urpf.Mode = oc.IfIp_UrpfMode_STRICT
		cfg.IPv6Obj.GetOrCreateUrpf()
		cfg.IPv6Obj.Urpf.Enabled = ygot.Bool(true)
		cfg.IPv6Obj.Urpf.Mode = oc.IfIp_UrpfMode_STRICT
	}
}

// EnableInterfaceAndSubinterfaces enables the parent interface and v4 and v6 subinterfaces.
func EnableInterfaceAndSubinterfaces(t *testing.T, dut *ondatra.DUTDevice, b *gnmi.SetBatch, portAttribs attrs.Attributes) {
	t.Helper()
	port := dut.Port(t, portAttribs.Name)
	intPath := gnmi.OC().Interface(port.Name()).Config()
	intf := &oc.Interface{
		Name:    ygot.String(port.Name()),
		Type:    oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		Enabled: ygot.Bool(true),
	}
	if deviations.InterfaceEnabled(dut) {
		intf.GetOrCreateSubinterface(portAttribs.Subinterface).GetOrCreateIpv4().SetEnabled(true)
		intf.GetOrCreateSubinterface(portAttribs.Subinterface).GetOrCreateIpv6().SetEnabled(true)
	}
	gnmi.BatchUpdate(b, intPath, intf)
}

// VlanParams defines the parameters for configuring a VLAN.
type VlanParams struct {
	VlanID uint16
}

// ConfigureVlan configures the Vlan and remove the spanning-tree with ID.
func ConfigureVlan(t *testing.T, dut *ondatra.DUTDevice, cfg VlanParams) {
	t.Helper()
	if !deviations.DeprecatedVlanID(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			cliConfig := fmt.Sprintf(`vlan %[1]d
			no spanning-tree vlan-id %[1]d`, cfg.VlanID)
			helpers.GnmiCLIConfig(t, dut, cliConfig)
		default:
			t.Logf("Unsupported vendor %s for native command support for deviation 'Vlan ID'", dut.Vendor())
		}
	} else {
		t.Log("Currently do not have support to configure VLAN and spanning-tree through OC, need to uncomment once implemented")
	}
}
