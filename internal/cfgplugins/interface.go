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
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	targetOutputPowerdBm          = -10
	targetOutputPowerTolerancedBm = 1
	targetFrequencyMHz            = 193100000
	targetFrequencyToleranceMHz   = 100000

	vlanInterfaceCLITemplate = `interface vlan %d
   ip address %s/%d
   ipv6 address %s/%d
`
	interfaceFallbackCLITemplate = `interface %s
   port-channel lacp fallback individual
   port-channel lacp fallback timeout %d
`
)

// DUTSubInterfaceData is the data structure for a subinterface in the DUT.
type DUTSubInterfaceData struct {
	VlanID                int
	VlanEnable            *bool
	IPv4Address           net.IP
	IPv6Address           net.IP
	IPv4PrefixLen         int
	IPv6PrefixLen         int
	NetworkInstanceParams NetworkInstanceParams
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
	DutPortsIdx     []int
	DutPorts        []*ondatra.Port
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

// PopulateDUTPorts populates the DUTPorts field of the DutLagData from the DUTPortsIdx
// field.
func (d *DUTAggData) PopulateDUTPorts(t *testing.T, dut *ondatra.DUTDevice) {
	for _, v := range d.DutPortsIdx {
		d.DutPorts = append(d.DutPorts, dut.Port(t, "port"+strconv.Itoa(v+1)))
	}
}

var (
	opmode   uint16
	once     sync.Once
	lBandPNs = map[string]bool{
		"DP04QSDD-LLH-240":   true, // Cisco QSFPDD Acacia 400G ZRP L-Band
		"DP04QSDD-LLH-24B":   true, // Cisco QSFPDD Acacia 400G ZRP L-Band
		"DP04QSDD-LLH-00A":   true, // Cisco QSFPDD Acacia 400G ZRP L-Band
		"BASIC-DP04QSDD-ULH": true, // Cisco QSFPDD Acacia 400G ZRP L-Band
		"C-QD04LEYNC-GG":     true, // Nokia QSFPDD 400G ZRP L-Band
		"176-3471-9G1":       true, // Ciena QSFPDD 400G ZRP L-Band
		"DP08SFP8-LRB-240":   true, // Cisco OSFP Acacia 800G ZRP L-Band
		"DP08SFP8-LRB-24B":   true, // Cisco OSFP Acacia 800G ZRP L-Band
		"C-OS08LEXNC-GG":     true, // Nokia OSFP 800G ZRP L-Band
		"176-6490-9G1":       true, // Ciena OSFP 800G ZRP L-Band
		"176-6491-9G0":       true, // Ciena OSFP 800G ZRP L-Band
		"176-6480-9M0":       true, // Ciena OSFP 800G ZRP L-Band
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
			return OperationalModeList{4}
		default:
			t.Fatalf("Unsupported vendor: %v", dut.Vendor())
		}
	case ondatra.PMD800GBASEZR:
		if dut.Vendor() == ondatra.CISCO {
			return OperationalModeList{6001, 6003} // 800G : 6001 (8x112G), 400G : 6003 (4x112G)
		}
		return OperationalModeList{1, 3} // 800G : 1 (8x112G), 400G : 3 (4x112G)
	case ondatra.PMD800GBASEZRP:
		// Ciena 800ZRP pluggable optics only support 800G mode (not backward compatible with 400G).
		isCienaOptics := strings.Contains(lookupOpticsMfgName(t, dut, p), "CIENA")
		if dut.Vendor() == ondatra.CISCO {
			if isCienaOptics {
				return OperationalModeList{6008} // 800G : 6008 (8x112G)
			}
			return OperationalModeList{6008, 6004} // 800G : 6008 (8x112G), 400G : 6004 (4x112G)
		}
		if isCienaOptics {
			return OperationalModeList{8} // 800G : 8 (8x112G)
		}
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
		switch {
		case lBandPNs[lookupOpticsPartNo(t, dut, p)]:
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
	return TargetOpticalPowerList{maxTargetOpticalPower(t, dut, dut.Ports()[0])}
}

// maxTargetOpticalPower returns the max target optical power for the given port.
func maxTargetOpticalPower(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) float64 {
	t.Helper()
	switch p.PMD() {
	case ondatra.PMD400GBASEZR:
		return -10
	case ondatra.PMD400GBASEZRP:
		return -7
	case ondatra.PMD800GBASEZR:
		return -7
	case ondatra.PMD800GBASEZRP:
		// Ciena transceivers have a max target optical power of -5 dBm.
		if strings.Contains(lookupOpticsMfgName(t, dut, p), "CIENA") {
			return -5
		}
		return -4
	default:
		t.Fatalf("Unsupported PMD type: %v", p.PMD())
	}
	return 0
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
	LinePortNames       map[string]string
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
	params.LinePortNames = make(map[string]string)
	for _, p := range dut.Ports() {
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
			case 1, 2, 8, 6001, 6002, 6008:
				params.PortSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_800GB
				params.NumPhysicalChannels = 8
				params.RateClass = oc.TransportTypes_TRIBUTARY_RATE_CLASS_TYPE_TRIB_RATE_800G
				params.TribProtocol = oc.TransportTypes_TRIBUTARY_PROTOCOL_TYPE_PROT_800GE
				params.Allocation = 800
			case 3, 4, 6003, 6004:
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
		params.HWPortNames[p.Name()] = lookupHWPortName(t, dut, p)
		params.TransceiverNames[p.Name()] = lookupTransceiverName(t, dut, p)
		params.OpticalChannelNames[p.Name()] = lookupOpticalChannelName(t, dut, p)
		params.LinePortNames[p.Name()] = lookupLinePortName(t, dut, p)
		params.TempSensorNames[p.Name()] = "TempSensor-" + lookupTransceiverName(t, dut, p)
		params.OTNIndexes = AssignOTNIndexes(t, dut)
		params.ETHIndexes = AssignETHIndexes(t, dut)
		updateInterfaceConfig(batch, dut, p, params)
		updateHWPortConfig(batch, dut, p, params)
		updateTransceiverConfig(batch, dut, p, params)
		updateOpticalChannelConfig(batch, dut, p, params)
		updateOTNChannelConfig(batch, dut, p, params)
		updateETHChannelConfig(batch, dut, p, params)
	}
}

// updateInterfaceConfig updates the interface config.
func updateInterfaceConfig(batch *gnmi.SetBatch, dut *ondatra.DUTDevice, p *ondatra.Port, params *ConfigParameters) {
	switch dut.Vendor() {
	case ondatra.ARISTA: // Arista always sets duplex mode and port speed.
		gnmi.BatchReplace(batch, gnmi.OC().Interface(p.Name()).Config(), &oc.Interface{
			Name:    ygot.String(p.Name()),
			Type:    oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
			Enabled: ygot.Bool(params.Enabled),
			Ethernet: &oc.Interface_Ethernet{
				PortSpeed:  params.PortSpeed,
				DuplexMode: oc.Ethernet_DuplexMode_FULL,
			},
		})
	case ondatra.NOKIA:
		gnmi.BatchReplace(batch, gnmi.OC().Interface(p.Name()).Config(), &oc.Interface{
			Name:    ygot.String(p.Name()),
			Type:    oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
			Enabled: ygot.Bool(params.Enabled),
		})
		// Nokia sets two interfaces for breakout ports.
		if IsBreakoutPort(dut, p, params) {
			breakoutInterfaceName := InterfaceName(dut, p, params)
			gnmi.BatchReplace(batch, gnmi.OC().Interface(breakoutInterfaceName).Config(), &oc.Interface{
				Name:    ygot.String(breakoutInterfaceName),
				Type:    oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
				Enabled: ygot.Bool(params.Enabled),
			})
		}
	case ondatra.CISCO:
		if IsBreakoutPort(dut, p, params) {
			// Cisco name is change for port stranding 800G to 400G and adds /0 to the end.
			gnmi.BatchDelete(batch, gnmi.OC().Interface(p.Name()).Config())
			breakoutInterfaceName := InterfaceName(dut, p, params)
			gnmi.BatchReplace(batch, gnmi.OC().Interface(breakoutInterfaceName).Config(), &oc.Interface{
				Name:    ygot.String(breakoutInterfaceName),
				Type:    oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
				Enabled: ygot.Bool(params.Enabled),
			})
		} else {
			gnmi.BatchReplace(batch, gnmi.OC().Interface(p.Name()).Config(), &oc.Interface{
				Name:    ygot.String(p.Name()),
				Type:    oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
				Enabled: ygot.Bool(params.Enabled),
			})
		}
	case ondatra.JUNIPER:
		gnmi.BatchReplace(batch, gnmi.OC().Interface(p.Name()).Config(), &oc.Interface{
			Name:    ygot.String(p.Name()),
			Type:    oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
			Enabled: ygot.Bool(params.Enabled),
		})
	}
}

// updateHWPortConfig updates the hardware port config.
func updateHWPortConfig(batch *gnmi.SetBatch, dut *ondatra.DUTDevice, p *ondatra.Port, params *ConfigParameters) {
	if !IsBreakoutPort(dut, p, params) {
		return // No hardware port config for non-breakout ports.
	}
	breakoutIndex := uint8(1)
	if dut.Vendor() == ondatra.CISCO || dut.Vendor() == ondatra.JUNIPER {
		breakoutIndex = 0
	}
	if dut.Vendor() == ondatra.JUNIPER {
		gnmi.BatchReplace(batch, gnmi.OC().Component(params.HWPortNames[p.Name()]).Config(), &oc.Component{
			Name: ygot.String(params.HWPortNames[p.Name()]),
			Port: &oc.Component_Port{
				BreakoutMode: &oc.Component_Port_BreakoutMode{
					Group: map[uint8]*oc.Component_Port_BreakoutMode_Group{
						breakoutIndex: {
							Index:         ygot.Uint8(breakoutIndex),
							BreakoutSpeed: params.PortSpeed,
							NumBreakouts:  ygot.Uint8(1),
						},
					},
				},
			},
		})
	} else {
		gnmi.BatchReplace(batch, gnmi.OC().Component(params.HWPortNames[p.Name()]).Config(), &oc.Component{
			Name: ygot.String(params.HWPortNames[p.Name()]),
			Port: &oc.Component_Port{
				BreakoutMode: &oc.Component_Port_BreakoutMode{
					Group: map[uint8]*oc.Component_Port_BreakoutMode_Group{
						breakoutIndex: {
							Index:               ygot.Uint8(breakoutIndex),
							BreakoutSpeed:       params.PortSpeed,
							NumBreakouts:        ygot.Uint8(1),
							NumPhysicalChannels: ygot.Uint8(params.NumPhysicalChannels),
						},
					},
				},
			},
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
func updateOpticalChannelConfig(batch *gnmi.SetBatch, dut *ondatra.DUTDevice, p *ondatra.Port, params *ConfigParameters) {
	och := &oc.Component_OpticalChannel{
		OperationalMode:   ygot.Uint16(params.OperationalMode),
		Frequency:         ygot.Uint64(params.Frequency),
		TargetOutputPower: ygot.Float64(params.TargetOpticalPower),
		LinePort:          ygot.String(params.LinePortNames[p.Name()]),
	}
	if dut.Vendor() == ondatra.NOKIA {
		och.LinePort = nil
	}
	gnmi.BatchReplace(batch, gnmi.OC().Component(params.OpticalChannelNames[p.Name()]).Config(), &oc.Component{
		Name:           ygot.String(params.OpticalChannelNames[p.Name()]),
		OpticalChannel: och,
	})
}

// updateOTNChannelConfig updates the OTN channel config.
func updateOTNChannelConfig(batch *gnmi.SetBatch, dut *ondatra.DUTDevice, p *ondatra.Port, params *ConfigParameters) {
	gnmi.BatchReplace(batch, gnmi.OC().TerminalDevice().Channel(params.OTNIndexes[p.Name()]).Config(), OTNChannelConfig(dut, p, params))
}

// OTNChannelConfig returns the OTN logical channel config for the port.
func OTNChannelConfig(dut *ondatra.DUTDevice, p *ondatra.Port, params *ConfigParameters) *oc.TerminalDevice_Channel {
	var firstAssignmentIndex uint32
	if deviations.OTNChannelAssignmentCiscoNumbering(dut) {
		firstAssignmentIndex = 1
	} else {
		firstAssignmentIndex = 0
	}
	var ch *oc.TerminalDevice_Channel
	switch {
	case deviations.OTNToETHAssignment(dut):
		ch = &oc.TerminalDevice_Channel{
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
		}
	case deviations.OTNChannelTribUnsupported(dut):
		ch = &oc.TerminalDevice_Channel{
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
			},
		}
	default:
		ch = &oc.TerminalDevice_Channel{
			Description:        ygot.String("OTN Logical Channel"),
			Index:              ygot.Uint32(params.OTNIndexes[p.Name()]),
			LogicalChannelType: oc.TransportTypes_LOGICAL_ELEMENT_PROTOCOL_TYPE_PROT_OTN,
			TribProtocol:       params.TribProtocol,
			Assignment: map[uint32]*oc.TerminalDevice_Channel_Assignment{
				firstAssignmentIndex: {
					Index:          ygot.Uint32(firstAssignmentIndex),
					OpticalChannel: ygot.String(params.OpticalChannelNames[p.Name()]),
					Description:    ygot.String("OTN to Optical Channel"),
					Allocation:     ygot.Float64(params.Allocation),
					AssignmentType: oc.Assignment_AssignmentType_OPTICAL_CHANNEL,
				},
			},
		}
	}
	if !deviations.TerminalDeviceChannelAdminStateUnsupported(dut) && !deviations.OTNToETHAssignment(dut) {
		ch.AdminState = oc.TerminalDevice_AdminStateType_ENABLED
	}
	return ch
}

// updateETHChannelConfig updates the ETH channel config.
func updateETHChannelConfig(batch *gnmi.SetBatch, dut *ondatra.DUTDevice, p *ondatra.Port, params *ConfigParameters) {
	gnmi.BatchReplace(batch, gnmi.OC().TerminalDevice().Channel(params.ETHIndexes[p.Name()]).Config(), ETHChannelConfig(dut, p, params))
}

// ETHChannelConfig returns the ETH logical channel config for the port.
func ETHChannelConfig(dut *ondatra.DUTDevice, p *ondatra.Port, params *ConfigParameters) *oc.TerminalDevice_Channel {
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
	}
	if !deviations.TerminalDeviceChannelAdminStateUnsupported(dut) {
		channel.AdminState = oc.TerminalDevice_AdminStateType_ENABLED
	}
	return channel
}

// ToggleInterfaceState toggles the interface with operational mode.
func ToggleInterfaceState(t *testing.T, dut *ondatra.DUTDevice, params *ConfigParameters) {
	batch := &gnmi.SetBatch{}
	for _, p := range dut.Ports() {
		updateInterfaceConfig(batch, dut, p, params)
	}
	batch.Set(t, dut)
}

// lookupHWPortName returns the hardware port name for the given port.
func lookupHWPortName(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) string {
	hwPortName, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(p.Name()).HardwarePort().State()).Val()
	if ok {
		return hwPortName
	}
	switch dut.Vendor() {
	case ondatra.JUNIPER:
		if hwPortName, ok = gnmi.Lookup(t, dut, gnmi.OC().Interface(strings.Replace(p.Name(), ":0", "", -1)).HardwarePort().State()).Val(); !ok {
			t.Fatalf("Hardware port not found for %v", strings.Replace(p.Name(), ":0", "", -1))
		}
		return hwPortName
	default:
		t.Fatalf("Hardware port not found for %v", p.Name())
	}
	return ""
}

// lookupTransceiverName returns the transceiver name for the given port.
func lookupTransceiverName(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) string {
	transceiverName, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State()).Val()
	if ok {
		return transceiverName
	}
	switch dut.Vendor() {
	case ondatra.JUNIPER:
		if transceiverName, ok = gnmi.Lookup(t, dut, gnmi.OC().Interface(strings.Replace(p.Name(), ":0", "", -1)).Transceiver().State()).Val(); !ok {
			t.Fatalf("Transceiver not found for %v", strings.Replace(p.Name(), ":0", "", -1))
		}
		return transceiverName
	default:
		t.Fatalf("Transceiver not found for %v", p.Name())
	}
	return ""
}

// lookupOpticalChannelName returns the optical channel name for the given port.
func lookupOpticalChannelName(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) string {
	transceiverName := lookupTransceiverName(t, dut, p)
	switch dut.Vendor() {
	case ondatra.ARISTA:
		return transceiverName + "-Optical0"
	case ondatra.JUNIPER:
		return transceiverName + ":OCH0"
	default:
		opticalChannelName, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(transceiverName).Transceiver().Channel(0).AssociatedOpticalChannel().State()).Val()
		if !ok {
			t.Fatalf("Optical channel not found for %v", p.Name())
		}
		return opticalChannelName
	}
}

// lookupLinePortName returns the line port name for the given port.
func lookupLinePortName(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) string {
	switch dut.Vendor() {
	case ondatra.ARISTA, ondatra.JUNIPER:
		return lookupHWPortName(t, dut, p)
	case ondatra.CISCO:
		return lookupTransceiverName(t, dut, p)
	case ondatra.NOKIA:
		return ""
	default:
		return ""
	}
}

// lookupOpticsMfgName returns the optics mfg name for the given port.
func lookupOpticsMfgName(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) string {
	transceiverName := lookupTransceiverName(t, dut, p)
	mfgName, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(transceiverName).MfgName().State()).Val()
	if !ok {
		t.Fatalf("mfg-name not found for %v", transceiverName)
	}
	return mfgName
}

// lookupOpticsPartNo returns the optics part number for the given port.
func lookupOpticsPartNo(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) string {
	transceiverName := lookupTransceiverName(t, dut, p)
	partNo, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(transceiverName).PartNo().State()).Val()
	if !ok {
		t.Fatalf("part-no not found for %v", transceiverName)
	}
	return partNo
}

// IsBreakoutPort returns true if the port is a breakout port, otherwise returns false.
func IsBreakoutPort(dut *ondatra.DUTDevice, p *ondatra.Port, params *ConfigParameters) bool {
	switch p.PMD() {
	case ondatra.PMD400GBASEZR, ondatra.PMD400GBASEZRP:
		return false
	case ondatra.PMD800GBASEZR, ondatra.PMD800GBASEZRP:
		switch dut.Vendor() {
		case ondatra.ARISTA, ondatra.JUNIPER:
			return true
		default:
			return params.PortSpeed == oc.IfEthernet_ETHERNET_SPEED_SPEED_400GB
		}
	default:
		return false
	}
}

// InterfaceName returns the interface name for the given port.
func InterfaceName(dut *ondatra.DUTDevice, p *ondatra.Port, params *ConfigParameters) string {
	if !IsBreakoutPort(dut, p, params) {
		return p.Name()
	}
	switch dut.Vendor() {
	case ondatra.ARISTA, ondatra.JUNIPER:
		return p.Name()
	case ondatra.NOKIA:
		return p.Name() + "/1"
	case ondatra.CISCO:
		return strings.Replace(p.Name(), "EightHundredGigE", "FourHundredGigE", -1) + "/0"
	default:
		return p.Name()
	}
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
	var ch *oc.TerminalDevice_Channel
	if deviations.OTNChannelTribUnsupported(dut) {
		ch = &oc.TerminalDevice_Channel{
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
		}
	} else {
		ch = &oc.TerminalDevice_Channel{
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
			},
		}
	}
	if deviations.OTNToETHAssignment(dut) {
		var firstAssignmentIndex uint32
		if deviations.OTNChannelAssignmentCiscoNumbering(dut) {
			firstAssignmentIndex = 1
		}
		ch.Assignment[firstAssignmentIndex+1] = &oc.TerminalDevice_Channel_Assignment{
			Index:          ygot.Uint32(firstAssignmentIndex + 1),
			LogicalChannel: ygot.Uint32(ethIndex),
			Description:    ygot.String("OTN to ETH"),
			Allocation:     ygot.Float64(400),
			AssignmentType: oc.Assignment_AssignmentType_LOGICAL_CHANNEL,
		}
	}
	if !deviations.TerminalDeviceChannelAdminStateUnsupported(dut) {
		ch.AdminState = oc.TerminalDevice_AdminStateType_ENABLED
	}
	gnmi.Replace(t, dut, gnmi.OC().TerminalDevice().Channel(otnIndex).Config(), ch)
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
	if !deviations.TerminalDeviceChannelAdminStateUnsupported(dut) {
		channel.AdminState = oc.TerminalDevice_AdminStateType_ENABLED
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
	vlanFlag := true
	sub := i.GetOrCreateSubinterface(uint32(s.VlanID))
	sub.Enabled = ygot.Bool(true)

	if s.VlanEnable != nil {
		vlanFlag = *s.VlanEnable
	}

	if s.VlanID != 0 && vlanFlag {
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
		if deviations.IPv4MissingEnabled(dut) {
			sub.GetOrCreateIpv4().SetEnabled(true)
		}

		if deviations.RequireRoutedSubinterface0(dut) {
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
	if deviations.IPv4MissingEnabled(dut) {
		agg.GetSubinterface(0).GetOrCreateIpv4().SetEnabled(true)
		agg.GetSubinterface(0).GetOrCreateIpv6().SetEnabled(true)
	}

	if deviations.RequireRoutedSubinterface0(dut) {
		agg.GetSubinterface(0).GetOrCreateIpv4().SetEnabled(true)
		agg.GetSubinterface(0).GetOrCreateIpv6().SetEnabled(true)
	}

	agg.GetOrCreateAggregation().LagType = l.AggType

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
	gnmi.BatchReplace(b, gnmi.OC().Interface(aggID).Config(), agg)
	gnmi.BatchDelete(b, gnmi.OC().Interface(aggID).Aggregation().MinLinks().Config())

	l.PopulateOndatraPorts(t, dut)
	if len(l.OndatraPorts) != 0 {
		for _, op := range l.OndatraPorts {
			AddPortToAggregate(t, dut, aggID, l.OndatraPorts, b, op)
		}
	} else {
		l.PopulateDUTPorts(t, dut)
		for _, op := range l.DutPorts {
			AddPortToAggregate(t, dut, aggID, l.DutPorts, b, op)
		}
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
			if (i.NetworkInstanceParams != NetworkInstanceParams{}) {
				AssignInterfaceToNetworkInstance(t, b, dut, aggID, &i.NetworkInstanceParams, uint32(i.VlanID), true)
			}
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
		if deviations.IPv4MissingEnabled(dut) {
			s4.Enabled = ygot.Bool(true)
		}
		s4a := s4.GetOrCreateAddress(ipv4Addr)
		s4a.PrefixLength = ygot.Uint8(ipv4Prefix)
	}

	// IPv6 Configuration
	if ipv6Addr != "" {
		s6 := s.GetOrCreateIpv6()
		if deviations.IPv4MissingEnabled(dut) {
			s6.Enabled = ygot.Bool(true)
		}
		s6a := s6.GetOrCreateAddress(ipv6Addr)
		s6a.PrefixLength = ygot.Uint8(ipv6Prefix)
	}
}

type AccessVlanParams struct {
	Intf   *oc.Interface
	VlanID uint16
}

// ConfigureAccessVlan sets the interface to ACCESS mode with given VLAN ID.
func ConfigureAccessVlan(p AccessVlanParams) {
	i := p.Intf

	// Remove L3 config (force L2 mode)
	i.Subinterface = nil

	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

	eth := i.GetOrCreateEthernet()

	// Configure switched VLAN
	swVlan := eth.GetOrCreateSwitchedVlan()
	swVlan.SetInterfaceMode(oc.Vlan_VlanModeType_ACCESS)
	swVlan.SetAccessVlan(p.VlanID)
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
		t.Log("Configuring VLAN using OpenConfig global VLAN model")

		vi := &oc.NetworkInstance_Vlan{
			VlanId: ygot.Uint16(uint16(cfg.VlanID)),
			Name:   ygot.String(fmt.Sprintf("VLAN_%d", cfg.VlanID)),
		}

		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Vlan(uint16(cfg.VlanID)).Config(), vi)
	}
}

// SVIParams holds the addressing and naming details for the SVI.
type SVIParams struct {
	IntfName string
	IPv4     string
	IPv4Len  uint8
	IPv6     string
	IPv6Len  uint8
	// Vlan, if non-nil, is set as routed-vlan/config/vlan to bind the SVI to its VLAN, either by
	// VLAN ID (oc.UnionUint16) or by VLAN name (oc.UnionString). It is needed on platforms where
	// the routed VLAN interface name does not imply the VLAN, e.g. Junos "irb.<unit>", which Junos
	// maps to "set vlans <vlan-name> l3-interface irb.<unit>".
	Vlan oc.Interface_RoutedVlan_Vlan_Union
}

// ConfigureSVI configures an L3 VLAN interface with IPv4 and IPv6 addresses.
func ConfigureSVI(t *testing.T, dut *ondatra.DUTDevice, params SVIParams) {
	t.Helper()
	t.Logf("Configuring SVI: %s", params.IntfName)

	// Initialize the Interface object with the L3 VLAN type
	svi := &oc.Interface{
		Name: ygot.String(params.IntfName),
		Type: oc.IETFInterfaces_InterfaceType_l3ipvlan,
	}

	// Handle vendor-specific interface enabled deviation
	if deviations.InterfaceEnabled(dut) {
		svi.Enabled = ygot.Bool(true)
	}

	// Navigate to the RoutedVlan container (subinterface-like layer for SVIs)
	rv := svi.GetOrCreateRoutedVlan()

	// Bind the SVI to its VLAN (routed-vlan/config/vlan) when requested.
	if params.Vlan != nil {
		rv.Vlan = params.Vlan
	}

	// IPv4 Configuration
	v4 := rv.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		v4.Enabled = ygot.Bool(true)
	}
	v4Addr := v4.GetOrCreateAddress(params.IPv4)
	v4Addr.PrefixLength = ygot.Uint8(params.IPv4Len)

	// IPv6 Configuration
	v6 := rv.GetOrCreateIpv6()
	v6.Enabled = ygot.Bool(true)
	v6Addr := v6.GetOrCreateAddress(params.IPv6)
	v6Addr.PrefixLength = ygot.Uint8(params.IPv6Len)

	// Apply the configuration via gNMI Replace
	gnmi.Replace(t, dut, gnmi.OC().Interface(params.IntfName).Config(), svi)
}

// AddressFamilyParams defines parameters for IPv4/v6 interfaces.
type AddressFamilyParams struct {
	InterfaceNames []string
}

// IsIPv4InterfaceARPresolved validates that the IPv4 interface is resolved based on the interface configured.
func IsIPv4InterfaceARPresolved(t *testing.T, ate *ondatra.ATEDevice, cfg AddressFamilyParams) error {
	for _, intf := range cfg.InterfaceNames {
		_, ok := gnmi.WatchAll(t, ate.OTG(), gnmi.OTG().Interface(intf+".Eth").Ipv4NeighborAny().LinkLayerAddress().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
			return val.IsPresent()
		}).Await(t)
		if !ok {
			return fmt.Errorf("IPv4 %s gateway not resolved", intf)
		}
	}
	return nil
}

// IsIPv6InterfaceARPresolved validates that the IPv6 interface is resolved based on the interface configured.
func IsIPv6InterfaceARPresolved(t *testing.T, ate *ondatra.ATEDevice, cfg AddressFamilyParams) error {
	for _, intf := range cfg.InterfaceNames {
		_, ok := gnmi.WatchAll(t, ate.OTG(), gnmi.OTG().Interface(intf+".Eth").Ipv6NeighborAny().LinkLayerAddress().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
			return val.IsPresent()
		}).Await(t)
		if !ok {
			return fmt.Errorf("IPv6 %s gateway not resolved", intf)
		}
	}
	return nil
}

func CreateVlanFromOC(t *testing.T, dut *ondatra.DUTDevice, vlanBatch *gnmi.SetBatch, networkInstance string, vlan DUTSubInterfaceData) {
	vlanName := fmt.Sprintf("vlan%d", vlan.VlanID)
	root := &oc.Root{}
	vlanObj := root.GetOrCreateNetworkInstance(networkInstance).GetOrCreateVlan(uint16(vlan.VlanID))
	vlanObj.Name = ygot.String(vlanName)
	vlanObj.VlanId = ygot.Uint16(uint16(vlan.VlanID))
	gnmi.BatchReplace(vlanBatch, gnmi.OC().NetworkInstance(networkInstance).Vlan(uint16(vlan.VlanID)).Config(), vlanObj)
}

func ConfigureVlanInterfaceFromCLI(t *testing.T, dut *ondatra.DUTDevice, vlan DUTSubInterfaceData) {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.ARISTA:
		t.Cleanup(func() {
			helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("no interface vlan %d\n", vlan.VlanID))
		})
		cli := fmt.Sprintf(vlanInterfaceCLITemplate, vlan.VlanID, vlan.IPv4Address, vlan.IPv4PrefixLen, vlan.IPv6Address, vlan.IPv6PrefixLen)
		helpers.GnmiCLIConfig(t, dut, cli)
	case ondatra.NOKIA:
		p1 := dut.Port(t, "port1").Name()
		p2 := dut.Port(t, "port2").Name()
		irbIntf := fmt.Sprintf("irb0.%d", vlan.VlanID)
		macVrf := fmt.Sprintf("vlan%d", vlan.VlanID)
		cleanupCLI := fmt.Sprintf(`
delete / network-instance %[10]s interface %[9]s
delete / network-instance %[8]s
delete / interface irb0 subinterface %[3]d
delete / interface %[1]s subinterface 0
delete / interface %[1]s vlan-tagging
delete / interface %[2]s subinterface 0
delete / interface %[2]s vlan-tagging
`, p1, p2, vlan.VlanID, vlan.IPv4Address, vlan.IPv4PrefixLen, vlan.IPv6Address, vlan.IPv6PrefixLen, macVrf, irbIntf, deviations.DefaultNetworkInstance(dut))
		t.Cleanup(func() {
			helpers.GnmiCLIConfig(t, dut, cleanupCLI)
		})
		cli := fmt.Sprintf(`
set / interface %[1]s vlan-tagging false
set / interface %[1]s subinterface 0 type bridged
set / interface %[1]s subinterface 0 admin-state enable
set / interface %[2]s vlan-tagging false
set / interface %[2]s subinterface 0 type bridged
set / interface %[2]s subinterface 0 admin-state enable
set / interface irb0 admin-state enable
set / interface irb0 subinterface %[3]d admin-state enable
set / interface irb0 subinterface %[3]d ipv4 admin-state enable
set / interface irb0 subinterface %[3]d ipv4 address %[4]s/%[5]d
set / interface irb0 subinterface %[3]d ipv6 admin-state enable
set / interface irb0 subinterface %[3]d ipv6 address %[6]s/%[7]d
set / network-instance %[8]s type mac-vrf
set / network-instance %[8]s admin-state enable
set / network-instance %[8]s interface %[1]s.0
set / network-instance %[8]s interface %[2]s.0
set / network-instance %[8]s interface %[9]s
set / network-instance %[10]s interface %[9]s
`, p1, p2, vlan.VlanID, vlan.IPv4Address, vlan.IPv4PrefixLen, vlan.IPv6Address, vlan.IPv6PrefixLen, macVrf, irbIntf, deviations.DefaultNetworkInstance(dut))
		helpers.GnmiCLIConfig(t, dut, cli)
	case ondatra.CISCO:
		// IOS XR has no switchports or VLAN interfaces: bridge DUT Ports 1 and 2 in an L2VPN
		// bridge-domain routed through BVI<vlan-id>, which carries the VLAN interface addresses.
		p1 := dut.Port(t, "port1").Name()
		p2 := dut.Port(t, "port2").Name()
		cleanupCLI := fmt.Sprintf(`
l2vpn
 bridge group BG_1
  no bridge-domain BD_%[3]d
 !
!
no interface BVI%[3]d
interface %[1]s
 no l2transport
!
interface %[2]s
 no l2transport
!
`, p1, p2, vlan.VlanID)
		t.Cleanup(func() {
			helpers.GnmiCLIConfig(t, dut, cleanupCLI)
		})
		cli := fmt.Sprintf(`
interface %[1]s
 l2transport
!
interface %[2]s
 l2transport
!
interface BVI%[3]d
 ipv4 address %[4]s/%[5]d
 ipv6 address %[6]s/%[7]d
!
l2vpn
 bridge group BG_1
  bridge-domain BD_%[3]d
   interface %[1]s
   !
   interface %[2]s
   !
   routed interface BVI%[3]d
   !
  !
 !
!
`, p1, p2, vlan.VlanID, vlan.IPv4Address, vlan.IPv4PrefixLen, vlan.IPv6Address, vlan.IPv6PrefixLen)
		helpers.GnmiCLIConfig(t, dut, cli)
	default:
		t.Fatalf("VLAN interface CLI deviation not implemented for vendor: %s", dut.Vendor())
	}
}

func ConfigureVlanInterfaceFromOC(t *testing.T, dut *ondatra.DUTDevice, vlanBatch *gnmi.SetBatch, vlan DUTSubInterfaceData) {
	root := &oc.Root{}
	vlanName := fmt.Sprintf("vlan%d", vlan.VlanID)
	vlanIntf := root.GetOrCreateInterface(vlanName)
	vlanIntf.Type = oc.IETFInterfaces_InterfaceType_l3ipvlan
	vlanIntf.Enabled = ygot.Bool(true)
	vlanIPv4 := vlanIntf.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	vlanIPv4.Enabled = ygot.Bool(true)
	vlanIPv4.GetOrCreateAddress(vlan.IPv4Address.String()).PrefixLength = ygot.Uint8(uint8(vlan.IPv4PrefixLen))
	vlanIPv6 := vlanIntf.GetOrCreateSubinterface(0).GetOrCreateIpv6()
	vlanIPv6.Enabled = ygot.Bool(true)
	vlanIPv6.GetOrCreateAddress(vlan.IPv6Address.String()).PrefixLength = ygot.Uint8(uint8(vlan.IPv6PrefixLen))
	gnmi.BatchReplace(vlanBatch, gnmi.OC().Interface(vlanName).Config(), vlanIntf)
}

func ConfigureLACPFallbackCLI(t *testing.T, dut *ondatra.DUTDevice, lagIntfName string, timeoutSecs uint16) {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.ARISTA:
		cli := fmt.Sprintf(interfaceFallbackCLITemplate, lagIntfName, timeoutSecs)
		helpers.GnmiCLIConfig(t, dut, cli)
	default:
		t.Fatalf("configureLACPFallbackCLI: unsupported vendor %s", dut.Vendor())
	}
}

// LoopbackConfig contains parameters for configuring a loopback interface.
type LoopbackConfig struct {
	Name      string
	IP        string
	PrefixLen uint8
	IsIPv6    bool
	Batch     *gnmi.SetBatch
}

// ConfigureLoopback configures a loopback interface with IPv4 or IPv6 addressing.
func ConfigureLoopback(t *testing.T, dut *ondatra.DUTDevice, cfg LoopbackConfig) {
	t.Helper()

	i := &oc.Interface{}
	i.Name = ygot.String(cfg.Name)
	i.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback

	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	s0 := i.GetOrCreateSubinterface(0)

	if cfg.IsIPv6 {
		ipv6 := s0.GetOrCreateIpv6()
		if deviations.InterfaceEnabled(dut) {
			ipv6.Enabled = ygot.Bool(true)
		}
		addr := ipv6.GetOrCreateAddress(cfg.IP)
		addr.PrefixLength = ygot.Uint8(cfg.PrefixLen)
	} else {
		ipv4 := s0.GetOrCreateIpv4()
		if deviations.InterfaceEnabled(dut) {
			ipv4.Enabled = ygot.Bool(true)
		}
		addr := ipv4.GetOrCreateAddress(cfg.IP)
		addr.PrefixLength = ygot.Uint8(cfg.PrefixLen)
	}

	if cfg.Batch != nil {
		gnmi.BatchUpdate(cfg.Batch, gnmi.OC().Interface(cfg.Name).Config(), i)
	} else {
		gnmi.Replace(t, dut, gnmi.OC().Interface(cfg.Name).Config(), i)
	}
}

// IPv6AddressParams holds parameters for configuring and validating an IPv6 address on a DUT interface.
type IPv6AddressParams struct {
	PortName         string
	SubinterfaceIdx  uint32
	Description      string
	IPv6             string
	PrefixLen        uint8
	AddrType         oc.E_IfIp_Ipv6AddressType
	ReplaceInterface bool
}

// IPv6LinkLocalPrefixLen returns the effective IPv6 link-local prefix length for the DUT,
// accounting for the LinkLocalMaskLen deviation.
func IPv6LinkLocalPrefixLen(dut *ondatra.DUTDevice, defaultLen uint8) uint8 {
	if deviations.LinkLocalMaskLen(dut) {
		return 128
	}
	return defaultLen
}

// ConfigureInterfaceIPv6Address appends gNMI configuration operations to sb for configuring
// an IPv6 address (link-local or global unicast) on a DUT interface/subinterface.
func ConfigureInterfaceIPv6Address(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, cfg IPv6AddressParams) *gnmi.SetBatch {
	t.Helper()
	p := dut.Port(t, cfg.PortName)
	prefixLen := cfg.PrefixLen
	if cfg.AddrType == oc.IfIp_Ipv6AddressType_LINK_LOCAL_UNICAST {
		prefixLen = IPv6LinkLocalPrefixLen(dut, cfg.PrefixLen)
	}

	if cfg.ReplaceInterface {
		attr := attrs.Attributes{
			Desc:         cfg.Description,
			Subinterface: cfg.SubinterfaceIdx,
			IPv6:         cfg.IPv6,
			IPv6Len:      prefixLen,
		}
		intf := attr.NewOCInterface(p.Name(), dut)
		subInt := intf.GetOrCreateSubinterface(cfg.SubinterfaceIdx)
		if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
			subInt.GetOrCreateIpv4().Enabled = ygot.Bool(true)
		}
		v6 := subInt.GetOrCreateIpv6()
		v6.Enabled = ygot.Bool(true)
		addr := v6.GetOrCreateAddress(cfg.IPv6)
		addr.SetPrefixLength(prefixLen)
		if cfg.AddrType == oc.IfIp_Ipv6AddressType_LINK_LOCAL_UNICAST {
			addr.SetType(cfg.AddrType)
		}
		if deviations.ExplicitPortSpeed(dut) {
			eth := intf.GetOrCreateEthernet()
			eth.PortSpeed = fptest.GetIfSpeed(t, p)
		}
		gnmi.BatchReplace(sb, gnmi.OC().Interface(p.Name()).Config(), intf)
		return sb
	}

	addr := &oc.Interface_Subinterface_Ipv6_Address{
		Ip:           ygot.String(cfg.IPv6),
		PrefixLength: ygot.Uint8(prefixLen),
	}
	if cfg.AddrType == oc.IfIp_Ipv6AddressType_LINK_LOCAL_UNICAST {
		addr.Type = cfg.AddrType
	}
	gnmi.BatchUpdate(sb, gnmi.OC().Interface(p.Name()).Subinterface(cfg.SubinterfaceIdx).Ipv6().Address(cfg.IPv6).Config(), addr)
	return sb
}

// VerifyInterfaceIPv6Address validates the config and state paths for an IPv6 address and subinterface enabled status using gnmi.Watch and gnmi.Get.
func VerifyInterfaceIPv6Address(t *testing.T, dut *ondatra.DUTDevice, portName string, subIntfIdx uint32, ip string, wantPrefixLen uint8, wantType oc.E_IfIp_Ipv6AddressType, timeout time.Duration) {
	t.Helper()
	if wantType == oc.IfIp_Ipv6AddressType_LINK_LOCAL_UNICAST {
		wantPrefixLen = IPv6LinkLocalPrefixLen(dut, wantPrefixLen)
	}
	v6Path := gnmi.OC().Interface(portName).Subinterface(subIntfIdx).Ipv6()
	addrPath := v6Path.Address(ip)

	if _, ok := gnmi.Watch(t, dut, addrPath.State(), timeout, func(val *ygnmi.Value[*oc.Interface_Subinterface_Ipv6_Address]) bool {
		state, present := val.Val()
		typeMatch := wantType == oc.IfIp_Ipv6AddressType_UNSET || state.GetType() == wantType
		return present && state != nil && state.GetIp() == ip && state.GetPrefixLength() == wantPrefixLen && typeMatch
	}).Await(t); !ok {
		state := gnmi.Get(t, dut, addrPath.State())
		if got, want := state.GetIp(), ip; got != want {
			t.Errorf("IP address state mismatch for port: %s, got: %s, want: %s", portName, got, want)
		}
		if wantType != oc.IfIp_Ipv6AddressType_UNSET {
			if got, want := state.GetType(), wantType; got != want {
				t.Errorf("IP address type state mismatch for port: %s, got: %v, want: %v", portName, got, want)
			}
		}
		if got, want := state.GetPrefixLength(), wantPrefixLen; got != want {
			t.Errorf("IP address prefix length state mismatch for port: %s, got: %d, want: %d", portName, got, want)
		}
	}

	conf := gnmi.Get(t, dut, addrPath.Config())
	if got, want := conf.GetIp(), ip; got != want {
		t.Errorf("IP address config-path mismatch for port: %s, got: %s, want: %s", portName, got, want)
	}
	if wantType == oc.IfIp_Ipv6AddressType_LINK_LOCAL_UNICAST {
		if got, want := conf.GetType(), wantType; got != want {
			t.Errorf("IP address type config-path mismatch for port: %s, got: %v, want: %v", portName, got, want)
		}
	}
	if got, want := conf.GetPrefixLength(), wantPrefixLen; got != want {
		t.Errorf("IP address prefix length config-path mismatch for port: %s, got: %d, want: %d", portName, got, want)
	}

	if got, want := gnmi.Get(t, dut, v6Path.Enabled().Config()), true; got != want {
		t.Errorf("IPv6 subinterface status config-path mismatch for port: %s, got: %v, want: %v", portName, got, want)
	}
	if got, want := gnmi.Get(t, dut, v6Path.Enabled().State()), true; got != want {
		t.Errorf("IPv6 subinterface status state mismatch for port: %s, got: %v, want: %v", portName, got, want)
	}
}

// L3AggregateParams holds parameters for configuring an L3 aggregate (LAG) interface and its member ports.
type L3AggregateParams struct {
	AggID            string
	LagType          oc.E_IfAggregate_AggregationType
	MinLinks         uint16
	LacpMode         oc.E_Lacp_LacpActivityType
	LacpInterval     oc.E_Lacp_LacpPeriodType
	MemberPorts      []*ondatra.Port
	ForwardingViable *bool
	Attributes       *attrs.Attributes
}

// SingleTaggedSubinterfaceParams holds parameters for configuring a single-tagged 802.1Q VLAN L3 subinterface.
type SingleTaggedSubinterfaceParams struct {
	InterfaceName string
	SubintfIndex  uint32
	VlanID        uint16
	IPv4Addr      string
	IPv4PrefixLen uint8
	IPv6Addr      string
	IPv6PrefixLen uint8
}

// ClearAggregateMembers removes the aggregate-id and forwarding-viable configuration from the specified member ports.
func ClearAggregateMembers(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, memberPorts []*ondatra.Port) *gnmi.SetBatch {
	t.Helper()
	if len(memberPorts) == 0 {
		t.Fatalf("ClearAggregateMembers: memberPorts slice cannot be empty")
	}
	if sb != nil {
		for _, port := range memberPorts {
			gnmi.BatchDelete(sb, gnmi.OC().Interface(port.Name()).Ethernet().AggregateId().Config())
			gnmi.BatchDelete(sb, gnmi.OC().Interface(port.Name()).ForwardingViable().Config())
		}
		return sb
	}
	localBatch := &gnmi.SetBatch{}
	for _, port := range memberPorts {
		gnmi.BatchDelete(localBatch, gnmi.OC().Interface(port.Name()).Ethernet().AggregateId().Config())
		gnmi.BatchDelete(localBatch, gnmi.OC().Interface(port.Name()).ForwardingViable().Config())
	}
	localBatch.Set(t, dut)
	return nil
}

// ConfigureL3AggregateInterface configures an L3 aggregate (LAG) interface, LACP parameters,
// and member ports using a gNMI SetBatch.
func ConfigureL3AggregateInterface(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, params L3AggregateParams) *gnmi.SetBatch {
	t.Helper()
	if len(params.MemberPorts) == 0 {
		t.Fatalf("ConfigureL3AggregateInterface: MemberPorts slice cannot be empty")
	}
	if sb == nil {
		sb = &gnmi.SetBatch{}
	}

	if deviations.AggregateAtomicUpdate(dut) {
		ClearAggregateMembers(t, dut, nil, params.MemberPorts)
		d := &oc.Root{}
		if params.LagType == oc.IfAggregate_AggregationType_LACP {
			lacpIntf := d.GetOrCreateLacp().GetOrCreateInterface(params.AggID)
			lacpIntf.LacpMode = params.LacpMode
			if params.LacpInterval != oc.Lacp_LacpPeriodType_UNSET {
				lacpIntf.Interval = params.LacpInterval
			}
		}
		agg := d.GetOrCreateInterface(params.AggID)
		agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
		agg.Enabled = ygot.Bool(true)
		agg.GetOrCreateAggregation().LagType = params.LagType
		if params.MinLinks > 0 {
			agg.GetOrCreateAggregation().MinLinks = ygot.Uint16(params.MinLinks)
		}
		for _, port := range params.MemberPorts {
			i := d.GetOrCreateInterface(port.Name())
			i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
			i.Enabled = ygot.Bool(true)
			if params.ForwardingViable != nil {
				i.ForwardingViable = ygot.Bool(*params.ForwardingViable)
			}
			i.GetOrCreateEthernet().AggregateId = ygot.String(params.AggID)
		}
		gnmi.Update(t, dut, gnmi.OC().Config(), d)
	}

	if params.LagType == oc.IfAggregate_AggregationType_LACP {
		lacp := &oc.Lacp_Interface{
			Name:     ygot.String(params.AggID),
			LacpMode: params.LacpMode,
		}
		if params.LacpInterval != oc.Lacp_LacpPeriodType_UNSET {
			lacp.Interval = params.LacpInterval
		}
		gnmi.BatchReplace(sb, gnmi.OC().Lacp().Interface(params.AggID).Config(), lacp)
	}

	agg := &oc.Interface{Name: ygot.String(params.AggID)}
	if params.Attributes != nil {
		params.Attributes.ConfigOCInterface(agg, dut)
	}
	agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	agg.Enabled = ygot.Bool(true)
	aggCfg := agg.GetOrCreateAggregation()
	aggCfg.LagType = params.LagType
	if params.MinLinks > 0 {
		aggCfg.MinLinks = ygot.Uint16(params.MinLinks)
	}
	if params.Attributes != nil {
		gnmi.BatchReplace(sb, gnmi.OC().Interface(params.AggID).Config(), agg)
	} else {
		gnmi.BatchUpdate(sb, gnmi.OC().Interface(params.AggID).Config(), agg)
	}

	for _, port := range params.MemberPorts {
		i := &oc.Interface{
			Name:        ygot.String(port.Name()),
			Description: ygot.String(port.String()),
			Type:        oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
			Enabled:     ygot.Bool(true),
		}
		if params.ForwardingViable != nil {
			i.ForwardingViable = ygot.Bool(*params.ForwardingViable)
		}
		i.GetOrCreateEthernet().AggregateId = ygot.String(params.AggID)
		if params.Attributes != nil {
			gnmi.BatchReplace(sb, gnmi.OC().Interface(port.Name()).Config(), i)
		} else {
			gnmi.BatchUpdate(sb, gnmi.OC().Interface(port.Name()).Config(), i)
		}
	}

	return sb
}

// ConfigureSingleTaggedSubinterface configures a single-tagged 802.1Q VLAN subinterface with dual-stack
// IPv4 and IPv6 addresses and adds it to the provided gNMI SetBatch.
func ConfigureSingleTaggedSubinterface(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, params SingleTaggedSubinterfaceParams) *gnmi.SetBatch {
	t.Helper()
	if sb == nil {
		sb = &gnmi.SetBatch{}
	}

	sub := &oc.Interface_Subinterface{
		Index: ygot.Uint32(params.SubintfIndex),
	}
	if deviations.InterfaceEnabled(dut) {
		sub.Enabled = ygot.Bool(true)
	}
	if params.VlanID > 0 {
		if deviations.DeprecatedVlanID(dut) {
			sub.GetOrCreateVlan().VlanId = oc.UnionUint16(params.VlanID)
		} else {
			sub.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().SetVlanId(params.VlanID)
		}
	}

	if params.IPv4Addr != "" {
		v4 := sub.GetOrCreateIpv4()
		if !deviations.IPv4MissingEnabled(dut) {
			v4.Enabled = ygot.Bool(true)
		}
		a4 := v4.GetOrCreateAddress(params.IPv4Addr)
		a4.Ip = ygot.String(params.IPv4Addr)
		a4.PrefixLength = ygot.Uint8(params.IPv4PrefixLen)
	}

	if params.IPv6Addr != "" {
		v6 := sub.GetOrCreateIpv6()
		v6.Enabled = ygot.Bool(true)
		a6 := v6.GetOrCreateAddress(params.IPv6Addr)
		a6.Ip = ygot.String(params.IPv6Addr)
		a6.PrefixLength = ygot.Uint8(params.IPv6PrefixLen)
	}

	subPath := gnmi.OC().Interface(params.InterfaceName).Subinterface(params.SubintfIndex)
	gnmi.BatchUpdate(sb, subPath.Config(), sub)
	return sb
}

// SetInterfaceAdminEnabled sets /interfaces/interface[name=<intfName>]/config/enabled via gNMI Replace.
func SetInterfaceAdminEnabled(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, intfName string, enabled bool) *gnmi.SetBatch {
	t.Helper()
	path := gnmi.OC().Interface(intfName).Enabled().Config()
	if sb != nil {
		gnmi.BatchReplace(sb, path, enabled)
		return sb
	}
	gnmi.Replace(t, dut, path, enabled)
	return nil
}

// SetMemberForwardingViable sets /interfaces/interface[name=<portName>]/config/forwarding-viable
// while preserving /interfaces/interface[name=<portName>]/ethernet/config/aggregate-id.
func SetMemberForwardingViable(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, portName, aggID string, viable bool) *gnmi.SetBatch {
	t.Helper()
	fvPath := gnmi.OC().Interface(portName).ForwardingViable().Config()
	aggIDPath := gnmi.OC().Interface(portName).Ethernet().AggregateId().Config()
	if sb != nil {
		gnmi.BatchReplace(sb, fvPath, viable)
		if aggID != "" {
			gnmi.BatchReplace(sb, aggIDPath, aggID)
		}
		return sb
	}
	localBatch := &gnmi.SetBatch{}
	gnmi.BatchReplace(localBatch, fvPath, viable)
	if aggID != "" {
		gnmi.BatchReplace(localBatch, aggIDPath, aggID)
	}
	localBatch.Set(t, dut)
	return nil
}

// CompareInterfaceNames compares two interface names in natural numeric order (e.g. Ethernet2 < Ethernet10).
func CompareInterfaceNames(a, b string) bool {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		isDigitA := a[i] >= '0' && a[i] <= '9'
		isDigitB := b[j] >= '0' && b[j] <= '9'
		if isDigitA && isDigitB {
			numA := 0
			for i < len(a) && a[i] >= '0' && a[i] <= '9' {
				numA = numA*10 + int(a[i]-'0')
				i++
			}
			numB := 0
			for j < len(b) && b[j] >= '0' && b[j] <= '9' {
				numB = numB*10 + int(b[j]-'0')
				j++
			}
			if numA != numB {
				return numA < numB
			}
			continue
		}
		if a[i] != b[j] {
			return a[i] < b[j]
		}
		i++
		j++
	}
	return len(a) < len(b)
}

// SortPortIDsByDUTName sorts a slice of port IDs in natural numeric order of their corresponding DUT interface names
// and returns both the sorted port IDs and the resolved *ondatra.Port slice.
func SortPortIDsByDUTName(t *testing.T, dut *ondatra.DUTDevice, portIDs []string) ([]string, []*ondatra.Port) {
	t.Helper()
	sortedIDs := append([]string(nil), portIDs...)
	sort.Slice(sortedIDs, func(i, j int) bool {
		return CompareInterfaceNames(dut.Port(t, sortedIDs[i]).Name(), dut.Port(t, sortedIDs[j]).Name())
	})
	ports := make([]*ondatra.Port, 0, len(sortedIDs))
	for _, id := range sortedIDs {
		ports = append(ports, dut.Port(t, id))
	}
	return sortedIDs, ports
}

// watchAwaitWithRetry executes a gnmi.Watch query on dut and awaits the predicate until timeout.
func watchAwaitWithRetry[T any](t *testing.T, dut *ondatra.DUTDevice, q ygnmi.SingletonQuery[T], timeout time.Duration, pred func(*ygnmi.Value[T]) bool) (*ygnmi.Value[T], bool) {
	t.Helper()
	return gnmi.Watch(t, dut, q, timeout, pred).Await(t)
}

// watchAllAwaitWithRetry executes a gnmi.WatchAll query on dut and awaits the predicate until timeout.
func watchAllAwaitWithRetry[T any](t *testing.T, dut *ondatra.DUTDevice, q ygnmi.WildcardQuery[T], timeout time.Duration, pred func(*ygnmi.Value[T]) bool) (*ygnmi.Value[T], bool) {
	t.Helper()
	return gnmi.WatchAll(t, dut, q, timeout, pred).Await(t)
}

// lookupWithRetry executes a gnmi.Lookup query on dut.
func lookupWithRetry[T any](t *testing.T, dut *ondatra.DUTDevice, q ygnmi.SingletonQuery[T]) *ygnmi.Value[T] {
	t.Helper()
	return gnmi.Lookup(t, dut, q)
}

// AwaitInterfaceAdminAndOperStatus verifies via gNMI Watch/Await that the specified interface
// reaches the expected enabled, admin-status, and oper-status telemetry states.
func AwaitInterfaceAdminAndOperStatus(t *testing.T, dut *ondatra.DUTDevice, intfName string, wantEnabled bool, wantAdmin oc.E_Interface_AdminStatus, wantOper oc.E_Interface_OperStatus, timeout time.Duration) {
	t.Helper()
	intfPath := gnmi.OC().Interface(intfName)
	if _, ok := watchAwaitWithRetry(t, dut, intfPath.Enabled().State(), timeout, func(val *ygnmi.Value[bool]) bool {
		got, present := val.Val()
		return present && got == wantEnabled
	}); !ok {
		t.Errorf("Interface %s enabled state did not reach %v within %v (got %v)", intfName, wantEnabled, timeout, lookupWithRetry(t, dut, intfPath.Enabled().State()))
	}
	if _, ok := watchAwaitWithRetry(t, dut, intfPath.AdminStatus().State(), timeout, func(val *ygnmi.Value[oc.E_Interface_AdminStatus]) bool {
		got, present := val.Val()
		return present && got == wantAdmin
	}); !ok {
		t.Errorf("Interface %s admin-status did not reach %v within %v (got %v)", intfName, wantAdmin, timeout, lookupWithRetry(t, dut, intfPath.AdminStatus().State()))
	}
	if _, ok := watchAwaitWithRetry(t, dut, intfPath.OperStatus().State(), timeout, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
		got, present := val.Val()
		return present && got == wantOper
	}); !ok {
		t.Errorf("Interface %s oper-status did not reach %v within %v (got %v)", intfName, wantOper, timeout, lookupWithRetry(t, dut, intfPath.OperStatus().State()))
	}
}

// AwaitAggregateLowerLayerDown verifies via gNMI Watch/Await that when all member links are down
// while the aggregate interface remains enabled, its admin-status remains UP and its oper-status
// reaches LOWER_LAYER_DOWN or DOWN.
func AwaitAggregateLowerLayerDown(t *testing.T, dut *ondatra.DUTDevice, aggID string, timeout time.Duration) {
	t.Helper()
	aggIntfPath := gnmi.OC().Interface(aggID)
	if _, ok := watchAwaitWithRetry(t, dut, aggIntfPath.AdminStatus().State(), timeout, func(val *ygnmi.Value[oc.E_Interface_AdminStatus]) bool {
		got, present := val.Val()
		return present && got == oc.Interface_AdminStatus_UP
	}); !ok {
		t.Errorf("Aggregate interface %s admin-status did not remain UP within %v (got %v)", aggID, timeout, lookupWithRetry(t, dut, aggIntfPath.AdminStatus().State()))
	}
	if _, ok := watchAwaitWithRetry(t, dut, aggIntfPath.OperStatus().State(), timeout, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
		got, present := val.Val()
		return present && (got == oc.Interface_OperStatus_LOWER_LAYER_DOWN || got == oc.Interface_OperStatus_DOWN)
	}); !ok {
		t.Errorf("Aggregate interface %s oper-status did not reach LOWER_LAYER_DOWN or DOWN within %v (got %v)", aggID, timeout, lookupWithRetry(t, dut, aggIntfPath.OperStatus().State()))
	}
}

// AwaitAggregateState verifies via gNMI Watch/Await that the aggregate interface reports the expected lag-type.
func AwaitAggregateState(t *testing.T, dut *ondatra.DUTDevice, aggID string, wantLagType oc.E_IfAggregate_AggregationType, timeout time.Duration) {
	t.Helper()
	aggPath := gnmi.OC().Interface(aggID).Aggregation()
	if _, ok := watchAwaitWithRetry(t, dut, aggPath.LagType().State(), timeout, func(val *ygnmi.Value[oc.E_IfAggregate_AggregationType]) bool {
		got, present := val.Val()
		return present && got == wantLagType
	}); !ok {
		t.Errorf("Interface %s aggregation lag-type did not reach %v within %v (got %v)", aggID, wantLagType, timeout, lookupWithRetry(t, dut, aggPath.LagType().State()))
	}
}

// AwaitMemberForwardingViableAndAggregateID verifies via gNMI Watch/Await that a LAG member port
// reports the expected forwarding-viable state and ethernet aggregate-id.
func AwaitMemberForwardingViableAndAggregateID(t *testing.T, dut *ondatra.DUTDevice, portName, wantAggID string, wantViable bool, timeout time.Duration) {
	t.Helper()
	intfPath := gnmi.OC().Interface(portName)
	if _, ok := watchAwaitWithRetry(t, dut, intfPath.ForwardingViable().State(), timeout, func(val *ygnmi.Value[bool]) bool {
		got, present := val.Val()
		if !present {
			return deviations.MissingValueForDefaults(dut) && wantViable
		}
		return got == wantViable
	}); !ok {
		t.Errorf("Interface %s forwarding-viable did not reach %v within %v (got %v)", portName, wantViable, timeout, lookupWithRetry(t, dut, intfPath.ForwardingViable().State()))
	}
	if _, ok := watchAwaitWithRetry(t, dut, intfPath.Ethernet().AggregateId().State(), timeout, func(val *ygnmi.Value[string]) bool {
		got, present := val.Val()
		return present && got == wantAggID
	}); !ok {
		t.Errorf("Interface %s ethernet aggregate-id did not reach %q within %v (got %v)", portName, wantAggID, timeout, lookupWithRetry(t, dut, intfPath.Ethernet().AggregateId().State()))
	}
}

// AwaitLACPMemberState verifies via gNMI Watch/Await that all specified LAG member ports report
// ACTIVE activity, aggregatable=true, collecting=true, distributing=true, and synchronization=IN_SYNC.
func AwaitLACPMemberState(t *testing.T, dut *ondatra.DUTDevice, aggID string, memberPorts []*ondatra.Port, timeout time.Duration) {
	t.Helper()
	if len(memberPorts) == 0 {
		t.Fatalf("AwaitLACPMemberState: memberPorts slice cannot be empty")
	}
	lacpIntfPath := gnmi.OC().Lacp().Interface(aggID)
	batch := gnmi.OCBatch()
	for _, p := range memberPorts {
		mPath := lacpIntfPath.Member(p.Name())
		batch.AddPaths(
			mPath.Activity(),
			mPath.Aggregatable(),
			mPath.Collecting(),
			mPath.Distributing(),
			mPath.Synchronization(),
		)
	}
	if _, ok := watchAwaitWithRetry(t, dut, batch.State(), timeout, func(val *ygnmi.Value[*oc.Root]) bool {
		root, present := val.Val()
		if !present || root == nil {
			return false
		}
		lacpIntf := root.GetLacp().GetInterface(aggID)
		if lacpIntf == nil {
			return false
		}
		for _, p := range memberPorts {
			m := lacpIntf.GetMember(p.Name())
			if m == nil {
				return false
			}
			if m.GetActivity() != oc.Lacp_LacpActivityType_ACTIVE ||
				!m.GetAggregatable() ||
				!m.GetCollecting() ||
				!m.GetDistributing() ||
				m.GetSynchronization() != oc.Lacp_LacpSynchronizationType_IN_SYNC {
				return false
			}
		}
		return true
	}); !ok {
		t.Errorf("LACP members %v on %s did not all reach (ACTIVE, aggregatable, collecting, distributing, IN_SYNC) within %v", memberPorts, aggID, timeout)
	}
}

// AwaitInterfaceCountersIncrement watches /interfaces/interface/state/counters/in-unicast-pkts,
// out-unicast-pkts, in-octets, and out-octets on the specified interface until packet and octet
// counters increment by at least minPktDelta.
func AwaitInterfaceCountersIncrement(t *testing.T, dut *ondatra.DUTDevice, intfName string, minPktDelta uint64, timeout time.Duration) {
	t.Helper()
	cntPath := gnmi.OC().Interface(intfName).Counters()
	batch := gnmi.OCBatch()
	batch.AddPaths(
		cntPath.InUnicastPkts(),
		cntPath.OutUnicastPkts(),
		cntPath.InOctets(),
		cntPath.OutOctets(),
	)

	isMemberPort := lookupWithRetry(t, dut, gnmi.OC().Interface(intfName).Ethernet().AggregateId().State()).IsPresent()
	inRequiredDelta := minPktDelta
	if isMemberPort {
		// Hardware ATE LAGs may hash reverse device-bound traffic from a single source MAC onto one member link,
		// while DUT egress load-balancing distributes OutUnicastPkts across all active member links.
		inRequiredDelta = 0
	}

	var baseIn, baseOut, baseInOct, baseOutOct uint64
	var baseInSet, baseOutSet, baseInOctSet, baseOutOctSet bool
	var inPktOK, outPktOK, inOctOK, outOctOK bool

	// Some platforms (such as Junos EVO) only emit counter values on initial gNMI subscription sync
	// (~8-12s per Watch) rather than streaming frequent updates over a single long-lived TARGET_DEFINED subscription.
	// Phase 1 captures the baseline counters on initial sync and immediately terminates the first Watch;
	// Phase 2 then watches for counter increments above that baseline in bounded 30s Watch windows.
	deadline := time.Now().Add(timeout)
	const watchStep = 30 * time.Second
	for time.Now().Before(deadline) && !(baseInSet && baseOutSet && baseInOctSet && baseOutOctSet) {
		rem := time.Until(deadline)
		if rem <= 0 {
			break
		}
		stepTimeout := watchStep
		if rem < stepTimeout {
			stepTimeout = rem
		}
		watchAwaitWithRetry(t, dut, batch.State(), stepTimeout, func(val *ygnmi.Value[*oc.Root]) bool {
			root, present := val.Val()
			if !present || root == nil {
				return false
			}
			intf := root.GetInterface(intfName)
			if intf == nil || intf.Counters == nil {
				return false
			}
			c := intf.Counters
			if c.InUnicastPkts != nil && !baseInSet {
				baseIn = *c.InUnicastPkts
				baseInSet = true
				if inRequiredDelta == 0 {
					inPktOK = true
				}
			}
			if c.OutUnicastPkts != nil && !baseOutSet {
				baseOut = *c.OutUnicastPkts
				baseOutSet = true
				if minPktDelta == 0 {
					outPktOK = true
				}
			}
			if c.InOctets != nil && !baseInOctSet {
				baseInOct = *c.InOctets
				baseInOctSet = true
				if inRequiredDelta == 0 {
					inOctOK = true
				}
			}
			if c.OutOctets != nil && !baseOutOctSet {
				baseOutOct = *c.OutOctets
				baseOutOctSet = true
				if minPktDelta == 0 {
					outOctOK = true
				}
			}
			return baseInSet && baseOutSet && baseInOctSet && baseOutOctSet
		})
	}

	if inPktOK && outPktOK && inOctOK && outOctOK {
		return
	}

	time.Sleep(2 * time.Second)

	for time.Now().Before(deadline) {
		rem := time.Until(deadline)
		if rem <= 0 {
			break
		}
		stepTimeout := watchStep
		if rem < stepTimeout {
			stepTimeout = rem
		}
		if _, ok := watchAwaitWithRetry(t, dut, batch.State(), stepTimeout, func(val *ygnmi.Value[*oc.Root]) bool {
			root, present := val.Val()
			if !present || root == nil {
				return false
			}
			intf := root.GetInterface(intfName)
			if intf == nil || intf.Counters == nil {
				return false
			}
			c := intf.Counters
			if c.InUnicastPkts != nil {
				v := *c.InUnicastPkts
				if !baseInSet {
					baseIn = v
					baseInSet = true
					if inRequiredDelta == 0 {
						inPktOK = true
					}
				} else if v >= baseIn+inRequiredDelta {
					inPktOK = true
				}
			}
			if c.OutUnicastPkts != nil {
				v := *c.OutUnicastPkts
				if !baseOutSet {
					baseOut = v
					baseOutSet = true
					if minPktDelta == 0 {
						outPktOK = true
					}
				} else if v >= baseOut+minPktDelta {
					outPktOK = true
				}
			}
			if c.InOctets != nil {
				v := *c.InOctets
				if !baseInOctSet {
					baseInOct = v
					baseInOctSet = true
					if inRequiredDelta == 0 {
						inOctOK = true
					}
				} else if v > baseInOct {
					inOctOK = true
				}
			}
			if c.OutOctets != nil {
				v := *c.OutOctets
				if !baseOutOctSet {
					baseOutOct = v
					baseOutOctSet = true
					if minPktDelta == 0 {
						outOctOK = true
					}
				} else if v > baseOutOct {
					outOctOK = true
				}
			}
			return inPktOK && outPktOK && inOctOK && outOctOK
		}); ok {
			return
		}
	}

	t.Errorf("Interface %s counters did not increment within %v (inPktOK=%v outPktOK=%v inOctOK=%v outOctOK=%v)", intfName, timeout, inPktOK, outPktOK, inOctOK, outOctOK)
}

// AwaitDifferentialMemberEgressTraffic verifies via gNMI Watch that when drainedPort has forwarding-viable=false,
// its out-unicast-pkts rate drops to 0 while activePorts carry 100% of egress routed traffic.
func AwaitDifferentialMemberEgressTraffic(t *testing.T, dut *ondatra.DUTDevice, drainedPort string, activePorts []*ondatra.Port, minActiveDelta uint64, timeout time.Duration) {
	t.Helper()
	if len(activePorts) == 0 {
		t.Fatalf("AwaitDifferentialMemberEgressTraffic: activePorts slice cannot be empty")
	}
	batch := gnmi.OCBatch()
	batch.AddPaths(gnmi.OC().Interface(drainedPort).Counters().OutUnicastPkts())
	for _, ap := range activePorts {
		batch.AddPaths(gnmi.OC().Interface(ap.Name()).Counters().OutUnicastPkts())
	}

	var baseSet bool
	var baseDrained uint64
	baseActive := make(map[string]uint64)

	deadline := time.Now().Add(timeout)
	const watchStep = 30 * time.Second
	var verified bool
	for time.Now().Before(deadline) && !verified {
		rem := time.Until(deadline)
		if rem <= 0 {
			break
		}
		stepTimeout := watchStep
		if rem < stepTimeout {
			stepTimeout = rem
		}
		if !baseSet {
			watchAwaitWithRetry(t, dut, batch.State(), stepTimeout, func(val *ygnmi.Value[*oc.Root]) bool {
				root, present := val.Val()
				if !present || root == nil {
					return false
				}
				drainedIntf := root.GetInterface(drainedPort)
				if drainedIntf == nil || drainedIntf.Counters == nil || drainedIntf.Counters.OutUnicastPkts == nil {
					return false
				}
				curActive := make(map[string]uint64)
				for _, ap := range activePorts {
					ai := root.GetInterface(ap.Name())
					if ai == nil || ai.Counters == nil || ai.Counters.OutUnicastPkts == nil {
						return false
					}
					curActive[ap.Name()] = ai.GetCounters().GetOutUnicastPkts()
				}
				baseDrained = drainedIntf.GetCounters().GetOutUnicastPkts()
				for k, v := range curActive {
					baseActive[k] = v
				}
				baseSet = true
				return true
			})
			if !baseSet {
				continue
			}
			time.Sleep(2 * time.Second)
			continue
		}

		watchAwaitWithRetry(t, dut, batch.State(), stepTimeout, func(val *ygnmi.Value[*oc.Root]) bool {
			root, present := val.Val()
			if !present || root == nil {
				return false
			}
			drainedIntf := root.GetInterface(drainedPort)
			if drainedIntf == nil || drainedIntf.Counters == nil || drainedIntf.Counters.OutUnicastPkts == nil {
				return false
			}
			curDrained := drainedIntf.GetCounters().GetOutUnicastPkts()

			curActive := make(map[string]uint64)
			for _, ap := range activePorts {
				ai := root.GetInterface(ap.Name())
				if ai == nil || ai.Counters == nil || ai.Counters.OutUnicastPkts == nil {
					return false
				}
				curActive[ap.Name()] = ai.GetCounters().GetOutUnicastPkts()
			}

			// Wait until every active member port has transmitted at least minActiveDelta packets.
			for _, ap := range activePorts {
				if curActive[ap.Name()] < baseActive[ap.Name()]+minActiveDelta {
					return false
				}
			}

			// Check if the drained member port transmitted 0 unicast packets over this active window.
			if curDrained == baseDrained {
				verified = true
				return true
			}

			// If the drained port still had in-flight packets draining at the start of the window,
			// reset baseline and return true so a fresh Watch window starts after a brief pause.
			baseDrained = curDrained
			for k, v := range curActive {
				baseActive[k] = v
			}
			return true
		})
		if !verified {
			time.Sleep(2 * time.Second)
		}
	}

	if !verified {
		t.Errorf("Drained member port %s continued transmitting out-unicast-pkts or active ports %v did not increment within %v", drainedPort, activePorts, timeout)
	}
}
