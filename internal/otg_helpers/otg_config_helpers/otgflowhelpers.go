// Package otgconfighelpers contains helper functions to create OTG flows and protocol configurations.
package otgconfighelpers

import (
	"github.com/open-traffic-generator/snappi/gosnappi"
)

// Iana Ethertype is the IANA Ethertype for MPLS, IPv4 and IPv6.
const (
	IanaMPLSEthertype = 34887
	IanaIPv4Ethertype = 2048
	IanaIPv6Ethertype = 34525
)

/*
Flow is a struct to hold Flow parameters.
TxNames and RxNames should be set to a valid OTG endpoint name.
Creating basic IPv4 Flow.

	  top = gosnappi.NewConfig()
		FlowIPv4 = &Flow{
			TxNames:   []string{"interface1"},
			RxNames:   []string{"interface2"},
			FrameSize: 1500,
			FlowName:  "IPv4Flow",
			EthFlow:   &EthFlowParams{SrcMAC: "00:11:22:33:44:55", DstMAC: "00:11:22:33:44:66"},
			IPv4Flow:  &IPv4FlowParams{IPv4Src: "12.1.1.1", IPv4Dst: "11.1.1.1", IPv4DstCount: 10},
		}
		FlowIPv4.CreateFlow(top)
		FlowIPv4.AddEthHeader()
		FlowIPv4.AddIPv4Header()
*/
type Flow struct {
	TxNames           []string
	RxNames           []string
	TxPort            string
	RxPorts           []string
	FrameSize         uint32
	FlowName          string
	Flowrate          float32
	PpsRate           uint64
	PacketsToSend     uint32
	Continuous        bool
	IsTxRxPort        bool
	SizeWeightProfile *[]SizeWeightPair
	VLANFlow          *VLANFlowParams
	GREFlow           *GREFlowParams
	EthFlow           *EthFlowParams
	IPv4Flow          *IPv4FlowParams
	IPv6Flow          *IPv6FlowParams
	TCPFlow           *TCPFlowParams
	UDPFlow           *UDPFlowParams
	MPLSFlow          *MPLSFlowParams
	CustomFlow        *CustomFlowParams
	flow              gosnappi.Flow
}

// GREFlowParams is a struct to hold Ethernet traffic parameters.
type GREFlowParams struct {
	Protocol uint32
}

// VLANFlowParams is a struct to hold VLAN traffic parameters.
type VLANFlowParams struct {
	VLANId    uint32
	VLANCount uint32
}

// EthFlowParams is a struct to hold Ethernet traffic parameters.
type EthFlowParams struct {
	SrcMAC      string
	DstMAC      string
	SrcMACCount uint32
	DstMACCount uint32
}

// IPv4FlowParams is a struct to hold IPv4 traffic parameters.
type IPv4FlowParams struct {
	IPv4Src          string
	IPv4Dst          string
	IPv4SrcCount     uint32
	IPv4DstCount     uint32
	IPv4SrcStep      string
	IPv4DstStep      string
	TTL              uint32
	TOS              uint32
	DSCP             uint32
	TOSCount         uint32
	DSCPCount        uint32
	RawPriority      uint32
	RawPriorityCount uint32
}

// IPv6FlowParams is a struct to hold IPv6 traffic parameters.
type IPv6FlowParams struct {
	IPv6Src           string
	IPv6Dst           string
	IPv6SrcCount      uint32
	IPv6DstCount      uint32
	IPv6SrcStep       string
	IPv6DstStep       string
	HopLimit          uint32
	TrafficClass      uint32
	TrafficClassCount uint32
	TrafficClassStep  uint32
}

// TCPFlowParams is a struct to hold TCP traffic parameters.
type TCPFlowParams struct {
	TCPSrcPort  uint32
	TCPDstPort  uint32
	TCPSrcCount uint32
	TCPDstCount uint32
}

// UDPFlowParams is a struct to hold UDP traffic parameters.
type UDPFlowParams struct {
	UDPSrcPort  uint32
	UDPDstPort  uint32
	UDPSrcCount uint32
	UDPDstCount uint32
}

// MPLSFlowParams is a struct to hold MPLS traffic parameters.
type MPLSFlowParams struct {
	MPLSLabel      uint32
	MPLSExp        uint32
	MPLSLabelCount uint32
	MPLSExpCount   uint32
	MPLSLabelStep  uint32
}

// CustomFlowParams is a struct to hold custom header parameters.
type CustomFlowParams struct {
	Bytes string
}

// SizeWeightPair represents a custom Size profile for a traffic flow.
type SizeWeightPair struct {
	Size   uint32
	Weight float32
}

// CreateFlow defines Tx and Rx end points for traffic flow.
func (f *Flow) CreateFlow(top gosnappi.Config) {
	f.flow = top.Flows().Add().SetName(f.FlowName)
	f.flow.Metrics().SetEnable(true)
	if f.IsTxRxPort {
		f.flow.TxRx().Port().
			SetTxName(f.TxPort).
			SetRxNames(f.RxPorts)
	} else {
		f.flow.TxRx().Device().
			SetTxNames(f.TxNames).
			SetRxNames(f.RxNames)
	}
	if f.FrameSize != 0 {
		f.flow.Size().SetFixed(f.FrameSize)
	}
	if f.Flowrate != 0 {
		f.flow.Rate().SetPercentage(f.Flowrate)
	}
	if f.PpsRate != 0 {
		f.flow.Rate().SetPps(f.PpsRate)
	}
	if f.PacketsToSend != 0 {
		f.flow.Duration().FixedPackets().SetPackets(f.PacketsToSend)
	} else if f.Continuous {
		f.flow.Duration().Continuous()
	}
	if f.SizeWeightProfile != nil {
		for _, sizeWeight := range *f.SizeWeightProfile {
			f.flow.Size().WeightPairs().Custom().Add().SetSize(sizeWeight.Size).SetWeight(sizeWeight.Weight)
		}
	}
}

// AddEthHeader adds an Ethernet header to the flow.
func (f *Flow) AddEthHeader() {
	eth := f.flow.Packet().Add().Ethernet()
	if f.EthFlow == nil {
		return
	}
	if f.EthFlow.SrcMAC != "" {
		if f.EthFlow.SrcMACCount != 0 {
			eth.Src().Increment().SetStart(f.EthFlow.SrcMAC).SetCount(f.EthFlow.SrcMACCount)
		} else {
			eth.Src().SetValue(f.EthFlow.SrcMAC)
		}
	}
	if f.EthFlow.DstMAC != "" {
		if f.EthFlow.DstMACCount != 0 {
			eth.Dst().Increment().SetStart(f.EthFlow.DstMAC).SetCount(f.EthFlow.DstMACCount)
		} else {
			eth.Dst().SetValue(f.EthFlow.DstMAC)
		}
	}
}

// AddGREHeader adds a GRE header to the flow.
func (f *Flow) AddGREHeader() {
	greHdr := f.flow.Packet().Add().Gre()
	switch f.GREFlow.Protocol {
	case IanaMPLSEthertype:
		greHdr.Protocol().SetValue(IanaMPLSEthertype)
	case IanaIPv4Ethertype:
		greHdr.Protocol().SetValue(IanaIPv4Ethertype)
	case IanaIPv6Ethertype:
		greHdr.Protocol().SetValue(IanaIPv6Ethertype)
	default:
		greHdr.Protocol().SetValue(IanaIPv4Ethertype)
	}
}

// AddVLANHeader adds a VLAN header to the flow.
func (f *Flow) AddVLANHeader() {
	if f.VLANFlow.VLANCount != 0 {
		f.flow.Packet().Add().Vlan().Id().Increment().SetStart(f.VLANFlow.VLANId).SetCount(f.VLANFlow.VLANCount)
	} else {
		f.flow.Packet().Add().Vlan().Id().SetValue(f.VLANFlow.VLANId)
	}
}

// AddMPLSHeader adds an MPLS header to the flow.
func (f *Flow) AddMPLSHeader() {
	mplsHdr := f.flow.Packet().Add().Mpls()

	switch {
	case f.MPLSFlow.MPLSLabelCount != 0 && f.MPLSFlow.MPLSLabelStep != 0:
		mplsHdr.Label().Increment().
			SetStart(f.MPLSFlow.MPLSLabel).
			SetCount(f.MPLSFlow.MPLSLabelCount).
			SetStep(f.MPLSFlow.MPLSLabelStep)
	case f.MPLSFlow.MPLSLabelCount != 0:
		mplsHdr.Label().Increment().
			SetStart(f.MPLSFlow.MPLSLabel).
			SetCount(f.MPLSFlow.MPLSLabelCount)
	default:
		mplsHdr.Label().SetValue(f.MPLSFlow.MPLSLabel)
	}

	switch {
	case f.MPLSFlow.MPLSExpCount != 0:
		mplsHdr.TrafficClass().Increment().
			SetStart(f.MPLSFlow.MPLSExp).
			SetCount(f.MPLSFlow.MPLSExpCount)
	default:
		mplsHdr.TrafficClass().SetValue(f.MPLSFlow.MPLSExp)
	}
}

// AddIPv4Header adds an IPv4 header to the flow.
func (f *Flow) AddIPv4Header() {
	ipv4Hdr := f.flow.Packet().Add().Ipv4()
	if f.IPv4Flow.IPv4SrcCount != 0 {
		inc := ipv4Hdr.Src().Increment().SetStart(f.IPv4Flow.IPv4Src).SetCount(f.IPv4Flow.IPv4SrcCount)
		if f.IPv4Flow.IPv4SrcStep != "" {
			inc.SetStep(f.IPv4Flow.IPv4SrcStep)
		}
	} else {
		ipv4Hdr.Src().SetValue(f.IPv4Flow.IPv4Src)
	}
	if f.IPv4Flow.IPv4DstCount != 0 {
		inc := ipv4Hdr.Dst().Increment().SetStart(f.IPv4Flow.IPv4Dst).SetCount(f.IPv4Flow.IPv4DstCount)
		if f.IPv4Flow.IPv4DstStep != "" {
			inc.SetStep(f.IPv4Flow.IPv4DstStep)
		}
	} else {
		ipv4Hdr.Dst().SetValue(f.IPv4Flow.IPv4Dst)
	}
	if f.IPv4Flow.TTL != 0 {
		ipv4Hdr.TimeToLive().SetValue(f.IPv4Flow.TTL)
	}
	if f.IPv4Flow.DSCP != 0 || f.IPv4Flow.DSCPCount != 0 {
		if f.IPv4Flow.DSCPCount != 0 {
			ipv4Hdr.Priority().Dscp().Phb().Increment().SetStart(f.IPv4Flow.DSCP).SetCount(f.IPv4Flow.DSCPCount)
		} else {
			ipv4Hdr.Priority().Dscp().Phb().SetValue(f.IPv4Flow.DSCP)
		}
	}
	if f.IPv4Flow.TOS != 0 || f.IPv4Flow.TOSCount != 0 {
		if f.IPv4Flow.TOSCount != 0 {
			ipv4Hdr.Priority().Tos().Precedence().Increment().SetStart(f.IPv4Flow.TOS).SetCount(f.IPv4Flow.TOSCount)
		} else {
			ipv4Hdr.Priority().Tos().Precedence().SetValue(f.IPv4Flow.TOS)
		}
	}
	if f.IPv4Flow.RawPriority != 0 || f.IPv4Flow.RawPriorityCount != 0 {
		if f.IPv4Flow.RawPriorityCount != 0 {
			ipv4Hdr.Priority().Raw().Increment().SetStart(f.IPv4Flow.RawPriority).SetCount(f.IPv4Flow.RawPriorityCount)
		} else {
			ipv4Hdr.Priority().Raw().SetValue(f.IPv4Flow.RawPriority)
		}
	}
}

// AddIPv6Header adds an IPv6 header to the flow.
func (f *Flow) AddIPv6Header() {
	ipv6Hdr := f.flow.Packet().Add().Ipv6()
	if f.IPv6Flow.IPv6SrcCount != 0 {
		inc := ipv6Hdr.Src().Increment().SetStart(f.IPv6Flow.IPv6Src).SetCount(f.IPv6Flow.IPv6SrcCount)
		if f.IPv6Flow.IPv6SrcStep != "" {
			inc.SetStep(f.IPv6Flow.IPv6SrcStep)
		}
	} else {
		ipv6Hdr.Src().SetValue(f.IPv6Flow.IPv6Src)
	}
	if f.IPv6Flow.IPv6DstCount != 0 {
		inc := ipv6Hdr.Dst().Increment().SetStart(f.IPv6Flow.IPv6Dst).SetCount(f.IPv6Flow.IPv6DstCount)
		if f.IPv6Flow.IPv6DstStep != "" {
			inc.SetStep(f.IPv6Flow.IPv6DstStep)
		}
	} else {
		ipv6Hdr.Dst().SetValue(f.IPv6Flow.IPv6Dst)
	}
	if f.IPv6Flow.HopLimit != 0 {
		ipv6Hdr.HopLimit().SetValue(f.IPv6Flow.HopLimit)
	}
	if f.IPv6Flow.TrafficClass != 0 || f.IPv6Flow.TrafficClassCount != 0 {
		if f.IPv6Flow.TrafficClassCount != 0 {
			tc := ipv6Hdr.TrafficClass().Increment().SetStart(f.IPv6Flow.TrafficClass).SetCount(f.IPv6Flow.TrafficClassCount)
			if f.IPv6Flow.TrafficClassStep != 0 {
				tc.SetStep(f.IPv6Flow.TrafficClassStep)
			}
		} else {
			ipv6Hdr.TrafficClass().SetValue(f.IPv6Flow.TrafficClass)
		}
	}
}

// AddTCPHeader adds a TCP header to the flow.
func (f *Flow) AddTCPHeader() {
	tcpHdr := f.flow.Packet().Add().Tcp()
	if f.TCPFlow.TCPSrcCount != 0 {
		tcpHdr.SrcPort().Increment().SetStart(f.TCPFlow.TCPSrcPort).SetCount(f.TCPFlow.TCPSrcCount)
	} else {
		tcpHdr.SrcPort().SetValue(f.TCPFlow.TCPSrcPort)
	}
	if f.TCPFlow.TCPDstCount != 0 {
		tcpHdr.DstPort().Increment().SetStart(f.TCPFlow.TCPDstPort).SetCount(f.TCPFlow.TCPDstCount)
	} else {
		tcpHdr.DstPort().SetValue(f.TCPFlow.TCPDstPort)
	}
}

// AddUDPHeader adds a UDP header to the flow.
func (f *Flow) AddUDPHeader() {
	udpHdr := f.flow.Packet().Add().Udp()
	if f.UDPFlow.UDPSrcCount != 0 {
		udpHdr.SrcPort().Increment().SetStart(f.UDPFlow.UDPSrcPort).SetCount(f.UDPFlow.UDPSrcCount)
	} else {
		udpHdr.SrcPort().SetValue(f.UDPFlow.UDPSrcPort)
	}
	if f.UDPFlow.UDPDstCount != 0 {
		udpHdr.DstPort().Increment().SetStart(f.UDPFlow.UDPDstPort).SetCount(f.UDPFlow.UDPDstCount)
	} else {
		udpHdr.DstPort().SetValue(f.UDPFlow.UDPDstPort)
	}
}

// AddCustomHeader adds an custom header to the flow.
func (f *Flow) AddCustomHeader() {
	customHeader := f.flow.Packet().Add().Custom()
	customHeader.SetBytes(f.CustomFlow.Bytes)
}

// GetFlow will return the flow object
func (f *Flow) GetFlow() gosnappi.Flow {
	return f.flow
}

// ContinuousUDPFlowParams holds parameters for creating a continuous IPv4 or IPv6 UDP flow
// (optionally single-tagged with an 802.1Q VLAN ID) with varied 5-tuple UDP ports for LAG hash distribution.
type ContinuousUDPFlowParams struct {
	FlowName     string
	TxEndpoint   string
	RxEndpoint   string
	SrcMAC       string
	DstMAC       string
	VlanID       uint32
	IsIPv6       bool
	SrcIP        string
	DstIP        string
	IPStep       string
	IPCount      uint32
	IncrementSrc bool
	IncrementDst bool
	FrameSize    uint32
	PpsRate      uint64
}

// AddContinuousUDPFlow adds a continuous IPv4 or IPv6 UDP flow (with optional 802.1Q VLAN header) to config.
func AddContinuousUDPFlow(config gosnappi.Config, params ContinuousUDPFlowParams) {
	f := &Flow{
		FlowName:   params.FlowName,
		TxNames:    []string{params.TxEndpoint},
		RxNames:    []string{params.RxEndpoint},
		FrameSize:  params.FrameSize,
		PpsRate:    params.PpsRate,
		Continuous: true,
		EthFlow: &EthFlowParams{
			SrcMAC: params.SrcMAC,
			DstMAC: params.DstMAC,
		},
		UDPFlow: &UDPFlowParams{
			UDPSrcPort:  10000,
			UDPSrcCount: 64,
			UDPDstPort:  20000,
			UDPDstCount: 64,
		},
	}
	f.CreateFlow(config)
	f.AddEthHeader()
	if params.VlanID > 0 {
		f.VLANFlow = &VLANFlowParams{VLANId: params.VlanID}
		f.AddVLANHeader()
	}
	count := params.IPCount
	if count == 0 {
		count = 1
	}
	if params.IsIPv6 {
		v6Params := &IPv6FlowParams{
			IPv6Src: params.SrcIP,
			IPv6Dst: params.DstIP,
		}
		if params.IncrementSrc {
			v6Params.IPv6SrcCount = count
			v6Params.IPv6SrcStep = params.IPStep
		}
		if params.IncrementDst {
			v6Params.IPv6DstCount = count
			v6Params.IPv6DstStep = params.IPStep
		}
		f.IPv6Flow = v6Params
		f.AddIPv6Header()
	} else {
		v4Params := &IPv4FlowParams{
			IPv4Src: params.SrcIP,
			IPv4Dst: params.DstIP,
		}
		if params.IncrementSrc {
			v4Params.IPv4SrcCount = count
			v4Params.IPv4SrcStep = params.IPStep
		}
		if params.IncrementDst {
			v4Params.IPv4DstCount = count
			v4Params.IPv4DstStep = params.IPStep
		}
		f.IPv4Flow = v4Params
		f.AddIPv4Header()
	}
	f.AddUDPHeader()
}
