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
Creting basic IPv4 Flow.

	  top = gosnappi.NewConfig()
		FlowIPv4 = &Flow{
			TxNames:   []string{"interface1"},
			RxNames:   []string{"interface2"},
			FrameSize: 1500,
			FlowName:  "IPv4Flow",
			EthFlow:   &EthFlowParams{SrcMAC: "00:11:22:33:44:55", DstMAC: "00:11:22:33:44:66"},
			IPv4Flow:  &IPv4FlowParams{IPv4Src: "192.0.2.1", IPv4Dst: "192.0.2.129", IPv4DstCount: 10},
		}
		FlowIPv4.CreateFlow(top)
		FlowIPv4.AddEthHeader()
		FlowIPv4.AddIPv4Header()
*/
type Flow struct {
	TxNames       []string
	RxNames       []string
	FrameSize     uint32
	FlowName      string
	Flowrate      float32
	PpsRate       uint64
	PacketsToSend uint32
	VLANFlow      *VLANFlowParams
	GREFlow       *GREFlowParams
	EthFlow       *EthFlowParams
	IPv4Flow      *IPv4FlowParams
	IPv6Flow      *IPv6FlowParams
	TCPFlow       *TCPFlowParams
	UDPFlow       *UDPFlowParams
	MPLSFlow      *MPLSFlowParams
	flow          gosnappi.Flow
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
	HopLimit          uint32
	TrafficClass      uint32
	TrafficClassCount uint32
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
}

// CreateFlow defines Tx and Rx end points for traffic flow.
func (f *Flow) CreateFlow(top gosnappi.Config) {
	f.flow = top.Flows().Add().SetName(f.FlowName)
	f.flow.Metrics().SetEnable(true)
	f.flow.TxRx().Device().
		SetTxNames(f.TxNames).
		SetRxNames(f.RxNames)

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
	}
}

// AddEthHeader adds an Ethernet header to the flow.
func (f *Flow) AddEthHeader() {
	eth := f.flow.Packet().Add().Ethernet()
	if f.EthFlow.SrcMACCount != 0 {
		eth.Src().Increment().SetStart(f.EthFlow.SrcMAC).SetCount(f.EthFlow.SrcMACCount)
	} else {
		eth.Src().SetValue(f.EthFlow.SrcMAC)
	}
	if f.EthFlow.DstMACCount != 0 {
		eth.Dst().Increment().SetStart(f.EthFlow.DstMAC).SetCount(f.EthFlow.DstMACCount)
	} else {
		eth.Dst().SetValue(f.EthFlow.DstMAC)
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

// AddVlanHeader adds a VLAN header to the flow.
func (f *Flow) AddVlanHeader() {
	if f.VLANFlow.VLANCount != 0 {
		f.flow.Packet().Add().Vlan().Id().Increment().SetStart(f.VLANFlow.VLANId).SetCount(f.VLANFlow.VLANCount)
	} else {
		f.flow.Packet().Add().Vlan().Id().SetValue(f.VLANFlow.VLANId)
	}
}

// AddMPLSHeader adds an MPLS header to the flow.
func (f *Flow) AddMPLSHeader() {
	mplsHdr := f.flow.Packet().Add().Mpls()
	if f.MPLSFlow.MPLSLabelCount != 0 {
		mplsHdr.Label().Increment().SetStart(f.MPLSFlow.MPLSLabel).SetCount(f.MPLSFlow.MPLSLabelCount)
	} else {
		mplsHdr.Label().SetValue(f.MPLSFlow.MPLSLabel)
	}
	if f.MPLSFlow.MPLSExpCount != 0 {
		mplsHdr.TrafficClass().Increment().SetStart(f.MPLSFlow.MPLSExp).SetCount(f.MPLSFlow.MPLSExpCount)
	} else {
		mplsHdr.TrafficClass().SetValue(f.MPLSFlow.MPLSExp)
	}
}

// AddIPv4Header adds an IPv4 header to the flow.
func (f *Flow) AddIPv4Header() {
	ipv4Hdr := f.flow.Packet().Add().Ipv4()
	if f.IPv4Flow.IPv4SrcCount != 0 {
		ipv4Hdr.Src().Increment().SetStart(f.IPv4Flow.IPv4Src).SetCount(f.IPv4Flow.IPv4SrcCount)
	} else {
		ipv4Hdr.Src().SetValue(f.IPv4Flow.IPv4Src)
	}
	if f.IPv4Flow.IPv4DstCount != 0 {
		ipv4Hdr.Dst().Increment().SetStart(f.IPv4Flow.IPv4Dst).SetCount(f.IPv4Flow.IPv4DstCount)
	} else {
		ipv4Hdr.Dst().SetValue(f.IPv4Flow.IPv4Dst)
	}
	if f.IPv4Flow.TTL != 0 {
		ipv4Hdr.TimeToLive().SetValue(f.IPv4Flow.TTL)
	}
	if f.IPv4Flow.DSCP != 0 {
		if f.IPv4Flow.DSCPCount != 0 {
			ipv4Hdr.Priority().Dscp().Phb().Increment().SetStart(f.IPv4Flow.DSCP).SetCount(f.IPv4Flow.DSCPCount)
		} else {
			ipv4Hdr.Priority().Dscp().Phb().SetValue(f.IPv4Flow.DSCP)
		}
	}
	if f.IPv4Flow.TOS != 0 {
		if f.IPv4Flow.TOSCount != 0 {
			ipv4Hdr.Priority().Tos().Precedence().Increment().SetStart(f.IPv4Flow.TOS).SetCount(f.IPv4Flow.TOSCount)
		} else {
			ipv4Hdr.Priority().Tos().Precedence().SetValue(f.IPv4Flow.TOS)
		}
	}
	if f.IPv4Flow.RawPriority != 0 {
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
		ipv6Hdr.Src().Increment().SetStart(f.IPv6Flow.IPv6Src).SetCount(f.IPv6Flow.IPv6SrcCount)
	} else {
		ipv6Hdr.Src().SetValue(f.IPv6Flow.IPv6Src)
	}
	if f.IPv6Flow.IPv6DstCount != 0 {
		ipv6Hdr.Dst().Increment().SetStart(f.IPv6Flow.IPv6Dst).SetCount(f.IPv6Flow.IPv6DstCount)
	} else {
		ipv6Hdr.Dst().SetValue(f.IPv6Flow.IPv6Dst)
	}
	if f.IPv6Flow.HopLimit != 0 {
		ipv6Hdr.HopLimit().SetValue(f.IPv6Flow.HopLimit)
	}
	if f.IPv6Flow.TrafficClassCount != 0 {
		ipv6Hdr.TrafficClass().Increment().SetStart(f.IPv6Flow.TrafficClass).SetCount(f.IPv6Flow.TrafficClassCount)
	} else {
		ipv6Hdr.TrafficClass().SetValue(f.IPv6Flow.TrafficClass)
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
