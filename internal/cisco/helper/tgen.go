package helper

import (
	gosnappi "github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/ondatra"
	"testing"
	"time"
)

type TgenHelper struct{}

// TGENConfig is the interface to configure TGEN interfaces
type TGENConfig interface {
	ConfigureTgenInterface(t *testing.T) *TGENTopology
	ConfigureTGENFlows(t *testing.T) *TGENFlow
}

// TGENTopology holds either ATE or OTG topology object
type TGENTopology struct {
	ATE *ondatra.ATETopology
	OTG gosnappi.Config
}

type TGENFlow struct {
	ATE []*ondatra.Flow
	OTG []gosnappi.Flow
}

// TrafficFlowAttr holds the Tgen traffic flow parameters.
type TrafficFlowAttr struct {
	FlowName           string
	SrCMacAddress      string
	DstMacAddress      string
	OuterProtocolType  string // "IPv4" or "IPv6"
	InnerProtocolType  string // "IPv4" or "IPv6"
	OuterSrcStart      string
	OuterDstStart      string
	OuterSrcStep       string
	OuterDstStep       string
	OuterSrcFlowCount  uint32
	OuterDstFlowCount  uint32
	InnerSrcStart      string
	InnerDstStart      string
	InnerSrcStep       string
	InnerDstStep       string
	OuterIPv6Flowlabel uint32
	InnerIPv6Flowlabel uint32
	InnerSrcFlowCount  uint32
	InnerDstFlowCount  uint32
	InnerDSCP          uint8
	OuterDSCP          uint8
	OuterTTL           uint8
	InnerTTL           uint8
	OuterECN           uint8
	InnerECN           uint8
	ProtocolType       string
	L4TCP              bool // if true, use TCP, else use UDP
	L4SrcPortStart     uint32
	L4DstPortStart     uint32
	L4PortRandom       bool // if true, use randomized L4 ports, else use port range
	L4FlowStep         uint32
	L4FlowCount        uint32
	TrafficPPS         uint64
	PacketSize         uint32
	TgenSrcPort        attrs.Attributes
	TgenDstPorts       []string
}

// TgenConfigParam holds the configuration input for both ATE and OTG
type TgenConfigParam struct {
	DutIntfAttr      []attrs.Attributes
	TgenIntfAttr     []attrs.Attributes
	TgenPortList     []*ondatra.Port
	TrafficFlowParam *TrafficFlowAttr
}

type ATEParam struct {
	Params *TgenConfigParam
}

type OTGParam struct {
	Params *TgenConfigParam
}

func (atep *ATEParam) ConfigureTgenInterface(t *testing.T) *TGENTopology {
	ate := ondatra.ATE(t, "ate")
	topo := ate.Topology().New()

	for i, intf := range atep.Params.TgenIntfAttr {
		intf.AddToATE(topo, atep.Params.TgenPortList[i], &atep.Params.DutIntfAttr[i])
	}

	t.Logf("Pushing config to ATE and starting protocols...")
	topo.Push(t).StartProtocols(t)

	return &TGENTopology{
		ATE: topo,
	}
}

func (otgp *OTGParam) ConfigureTgenInterface(t *testing.T) *TGENTopology {
	otg := ondatra.ATE(t, "ate").OTG()
	topo := gosnappi.NewConfig()

	for i, intf := range otgp.Params.TgenIntfAttr {
		intf.AddToOTG(topo, otgp.Params.TgenPortList[i], &otgp.Params.DutIntfAttr[i])
	}

	t.Logf("Pushing config to OTG and starting protocols...")
	otg.PushConfig(t, topo)
	otg.StartProtocols(t)

	return &TGENTopology{
		OTG: topo,
	}
}

// Creates ATE Traffic Flow using TrafficFlowAttr struct.
func (atep *ATEParam) ConfigureTGENFlows(t *testing.T) *TGENFlow {
	ate := ondatra.ATE(t, "ate")
	topo := ate.Topology().New()
	flow := ate.Traffic().NewFlow(atep.Params.TrafficFlowParam.FlowName)

	// Configure source port
	p1 := ate.Port(t, atep.Params.TrafficFlowParam.TgenSrcPort.Name)
	srcPort := topo.AddInterface(atep.Params.TrafficFlowParam.TgenSrcPort.Name).WithPort(p1)

	// Configure destination ports
	dstEndPoints := []ondatra.Endpoint{}
	for _, v := range atep.Params.TrafficFlowParam.TgenDstPorts {
		p := ate.Port(t, v)
		d := topo.AddInterface(v).WithPort(p)
		dstEndPoints = append(dstEndPoints, d)
	}

	// Ethernet header
	ethHeader := ondatra.NewEthernetHeader().WithSrcAddress(atep.Params.TrafficFlowParam.TgenSrcPort.MAC).WithDstAddress(atep.Params.TrafficFlowParam.DstMacAddress)

	// Outer IP header configuration
	var outerIPv4Header *ondatra.IPv4Header
	var outerIPv6Header *ondatra.IPv6Header
	if atep.Params.TrafficFlowParam.OuterProtocolType == "IPv4" {
		outerIPv4Header = ondatra.NewIPv4Header()
		outerIPv4Header.SrcAddressRange().WithStep(atep.Params.TrafficFlowParam.OuterSrcStep).WithMin(atep.Params.TrafficFlowParam.OuterSrcStart).WithCount(uint32(atep.Params.TrafficFlowParam.OuterSrcFlowCount))
		outerIPv4Header.DstAddressRange().WithStep(atep.Params.TrafficFlowParam.OuterDstStep).WithMin(atep.Params.TrafficFlowParam.OuterDstStart).WithCount(uint32(atep.Params.TrafficFlowParam.OuterDstFlowCount))
		outerIPv4Header.WithDSCP(atep.Params.TrafficFlowParam.OuterDSCP).WithTTL(atep.Params.TrafficFlowParam.OuterTTL).WithECN(atep.Params.TrafficFlowParam.OuterECN)
	} else if atep.Params.TrafficFlowParam.OuterProtocolType == "IPv6" {
		outerIPv6Header = ondatra.NewIPv6Header()
		outerIPv6Header.SrcAddressRange().WithStep(atep.Params.TrafficFlowParam.OuterSrcStep).WithMin(atep.Params.TrafficFlowParam.OuterSrcStart).WithCount(uint32(atep.Params.TrafficFlowParam.OuterSrcFlowCount))
		outerIPv6Header.DstAddressRange().WithStep(atep.Params.TrafficFlowParam.OuterDstStep).WithMin(atep.Params.TrafficFlowParam.OuterDstStart).WithCount(uint32(atep.Params.TrafficFlowParam.OuterDstFlowCount))
		outerIPv6Header.WithDSCP(atep.Params.TrafficFlowParam.OuterDSCP).WithHopLimit(atep.Params.TrafficFlowParam.OuterTTL).WithECN(atep.Params.TrafficFlowParam.OuterECN).FlowLabelRange().WithMin(atep.Params.TrafficFlowParam.OuterIPv6Flowlabel).WithRandom()
	}

	// Inner IP header configuration
	var innerIPv4Header *ondatra.IPv4Header
	var innerIPv6Header *ondatra.IPv6Header
	if atep.Params.TrafficFlowParam.InnerProtocolType == "IPv4" {
		innerIPv4Header = ondatra.NewIPv4Header()
		innerIPv4Header.SrcAddressRange().WithStep(atep.Params.TrafficFlowParam.InnerSrcStep).WithMin(atep.Params.TrafficFlowParam.InnerSrcStart).WithCount(uint32(atep.Params.TrafficFlowParam.InnerSrcFlowCount))
		innerIPv4Header.DstAddressRange().WithStep(atep.Params.TrafficFlowParam.InnerDstStep).WithMin(atep.Params.TrafficFlowParam.InnerDstStart).WithCount(uint32(atep.Params.TrafficFlowParam.InnerDstFlowCount))
		innerIPv4Header.WithDSCP(atep.Params.TrafficFlowParam.InnerDSCP).WithTTL(atep.Params.TrafficFlowParam.InnerTTL).WithECN(atep.Params.TrafficFlowParam.InnerECN)
	} else if atep.Params.TrafficFlowParam.InnerProtocolType == "IPv6" {
		innerIPv6Header = ondatra.NewIPv6Header()
		innerIPv6Header.SrcAddressRange().WithStep(atep.Params.TrafficFlowParam.InnerSrcStep).WithMin(atep.Params.TrafficFlowParam.InnerSrcStart).WithCount(uint32(atep.Params.TrafficFlowParam.InnerSrcFlowCount))
		innerIPv6Header.DstAddressRange().WithStep(atep.Params.TrafficFlowParam.InnerDstStep).WithMin(atep.Params.TrafficFlowParam.InnerDstStart).WithCount(uint32(atep.Params.TrafficFlowParam.InnerDstFlowCount))
		innerIPv6Header.WithDSCP(atep.Params.TrafficFlowParam.InnerDSCP).WithHopLimit(atep.Params.TrafficFlowParam.InnerTTL).WithECN(atep.Params.TrafficFlowParam.InnerECN).FlowLabelRange().WithMin(atep.Params.TrafficFlowParam.InnerIPv6Flowlabel).WithRandom()
	}

	// Layer4 header configuration
	var l4TCPHeader *ondatra.TCPHeader
	var l4UDPHeader *ondatra.UDPHeader
	// Randomize L4 ports
	if atep.Params.TrafficFlowParam.L4PortRandom {
		if atep.Params.TrafficFlowParam.L4TCP {
			// TCP header
			l4TCPHeader = ondatra.NewTCPHeader()
			l4TCPHeader.SrcPortRange().WithRandom().WithCount(atep.Params.TrafficFlowParam.L4FlowCount)
			l4TCPHeader.DstPortRange().WithRandom().WithCount(atep.Params.TrafficFlowParam.L4FlowCount)
		} else {
			// UDP header
			l4UDPHeader = ondatra.NewUDPHeader()
			l4UDPHeader.SrcPortRange().WithRandom().WithCount(atep.Params.TrafficFlowParam.L4FlowCount)
			l4UDPHeader.DstPortRange().WithRandom().WithCount(atep.Params.TrafficFlowParam.L4FlowCount)
		}

	} else { // Use specified L4 port range
		if atep.Params.TrafficFlowParam.L4TCP {
			// TCP header
			l4TCPHeader = ondatra.NewTCPHeader()
			l4TCPHeader.SrcPortRange().WithMin(atep.Params.TrafficFlowParam.L4SrcPortStart).WithStep(atep.Params.TrafficFlowParam.L4FlowStep).WithCount(atep.Params.TrafficFlowParam.L4FlowCount)
			l4TCPHeader.DstPortRange().WithMin(atep.Params.TrafficFlowParam.L4DstPortStart).WithStep(atep.Params.TrafficFlowParam.L4FlowStep).WithCount(atep.Params.TrafficFlowParam.L4FlowCount)
		} else {
			// UDP header
			l4UDPHeader = ondatra.NewUDPHeader()
			l4UDPHeader.SrcPortRange().WithMin(atep.Params.TrafficFlowParam.L4SrcPortStart).WithStep(atep.Params.TrafficFlowParam.L4FlowStep).WithCount(atep.Params.TrafficFlowParam.L4FlowCount)
			l4UDPHeader.DstPortRange().WithMin(atep.Params.TrafficFlowParam.L4DstPortStart).WithStep(atep.Params.TrafficFlowParam.L4FlowStep).WithCount(atep.Params.TrafficFlowParam.L4FlowCount)
		}
	}

	// Combine headers based on encapsulation type
	var l4Header ondatra.Header
	if atep.Params.TrafficFlowParam.L4TCP {
		l4Header = l4TCPHeader
	} else {
		l4Header = l4UDPHeader
	}

	if atep.Params.TrafficFlowParam.OuterProtocolType == "IPv4" && atep.Params.TrafficFlowParam.InnerProtocolType == "IPv4" {
		flow.WithSrcEndpoints(srcPort).
			WithHeaders(ethHeader, outerIPv4Header, innerIPv4Header, l4Header).
			WithDstEndpoints(dstEndPoints...).
			WithFrameRateFPS(atep.Params.TrafficFlowParam.TrafficPPS).WithFrameSize(atep.Params.TrafficFlowParam.PacketSize)
	} else if atep.Params.TrafficFlowParam.OuterProtocolType == "IPv4" && atep.Params.TrafficFlowParam.InnerProtocolType == "IPv6" {
		flow.WithSrcEndpoints(srcPort).
			WithHeaders(ethHeader, outerIPv4Header, innerIPv6Header, l4Header).
			WithDstEndpoints(dstEndPoints...).
			WithFrameRateFPS(atep.Params.TrafficFlowParam.TrafficPPS).WithFrameSize(atep.Params.TrafficFlowParam.PacketSize)
	} else if atep.Params.TrafficFlowParam.OuterProtocolType == "IPv6" && atep.Params.TrafficFlowParam.InnerProtocolType == "IPv4" {
		flow.WithSrcEndpoints(srcPort).
			WithHeaders(ethHeader, outerIPv6Header, innerIPv4Header, l4Header).
			WithDstEndpoints(dstEndPoints...).
			WithFrameRateFPS(atep.Params.TrafficFlowParam.TrafficPPS).WithFrameSize(atep.Params.TrafficFlowParam.PacketSize)
	} else if atep.Params.TrafficFlowParam.OuterProtocolType == "IPv6" && atep.Params.TrafficFlowParam.InnerProtocolType == "IPv6" {
		flow.WithSrcEndpoints(srcPort).
			WithHeaders(ethHeader, outerIPv6Header, innerIPv6Header, l4Header).
			WithDstEndpoints(dstEndPoints...).
			WithFrameRateFPS(atep.Params.TrafficFlowParam.TrafficPPS).WithFrameSize(atep.Params.TrafficFlowParam.PacketSize)
	} else if atep.Params.TrafficFlowParam.OuterProtocolType == "IPv4" {
		flow.WithSrcEndpoints(srcPort).
			WithHeaders(ethHeader, outerIPv4Header, l4Header).
			WithDstEndpoints(dstEndPoints...).
			WithFrameRateFPS(atep.Params.TrafficFlowParam.TrafficPPS).WithFrameSize(atep.Params.TrafficFlowParam.PacketSize)
	} else if atep.Params.TrafficFlowParam.OuterProtocolType == "IPv6" {
		flow.WithSrcEndpoints(srcPort).
			WithHeaders(ethHeader, outerIPv6Header, l4Header).
			WithDstEndpoints(dstEndPoints...).
			WithFrameRateFPS(atep.Params.TrafficFlowParam.TrafficPPS).WithFrameSize(atep.Params.TrafficFlowParam.PacketSize)
	} else {
		t.Log("No valid protocol type specified")

	}

	return &TGENFlow{
		ATE: []*ondatra.Flow{flow},
	}
}

func (otgp *OTGParam) ConfigureTGENFlows(t *testing.T) *TGENFlow {
	topo := gosnappi.NewConfig()
	flow := topo.Flows().Add().SetName(otgp.Params.TrafficFlowParam.FlowName)
	flow.Metrics().SetEnable(true)

	flow.TxRx().Port().SetTxName(otgp.Params.TrafficFlowParam.TgenSrcPort.Name).SetRxNames(otgp.Params.TrafficFlowParam.TgenDstPorts)
	// Ethernet header
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(otgp.Params.TrafficFlowParam.TgenSrcPort.MAC)
	ethHeader.Dst().SetValue(otgp.Params.TrafficFlowParam.DstMacAddress)

	// Outer IP header configuration
	var outerIPv4Header gosnappi.FlowIpv4
	var outerIPv6Header gosnappi.FlowIpv6
	if otgp.Params.TrafficFlowParam.OuterProtocolType == "IPv4" {
		outerIPv4Header = flow.Packet().Add().Ipv4()
		outerIPv4Header.Src().Increment().SetStart(otgp.Params.TrafficFlowParam.OuterSrcStart).SetStep(otgp.Params.TrafficFlowParam.OuterSrcStep).SetCount(otgp.Params.TrafficFlowParam.OuterSrcFlowCount)
		outerIPv4Header.Dst().Increment().SetStart(otgp.Params.TrafficFlowParam.OuterDstStart).SetStep(otgp.Params.TrafficFlowParam.OuterDstStep).SetCount(otgp.Params.TrafficFlowParam.OuterDstFlowCount)
		outerIPv4Header.TimeToLive().SetValue(uint32(otgp.Params.TrafficFlowParam.OuterTTL))
		outerIPv4Header.Priority().Dscp().Phb().SetValue(uint32(otgp.Params.TrafficFlowParam.OuterDSCP))
		outerIPv4Header.Priority().Dscp().Ecn().SetValue(uint32(otgp.Params.TrafficFlowParam.OuterECN))
	} else if otgp.Params.TrafficFlowParam.OuterProtocolType == "IPv6" {
		outerIPv6Header = flow.Packet().Add().Ipv6()
		outerIPv6Header.Src().Increment().SetStart(otgp.Params.TrafficFlowParam.OuterSrcStart).SetStep(otgp.Params.TrafficFlowParam.OuterSrcStep).SetCount(otgp.Params.TrafficFlowParam.OuterSrcFlowCount)
		outerIPv6Header.Dst().Increment().SetStart(otgp.Params.TrafficFlowParam.OuterDstStart).SetStep(otgp.Params.TrafficFlowParam.OuterDstStep).SetCount(otgp.Params.TrafficFlowParam.OuterDstFlowCount)
		outerIPv6Header.HopLimit().SetValue(uint32(otgp.Params.TrafficFlowParam.OuterTTL))
		outerIPv6Header.TrafficClass().SetValue(uint32(otgp.Params.TrafficFlowParam.OuterDSCP << 2))
		outerIPv6Header.FlowLabel().Increment().SetStart(otgp.Params.TrafficFlowParam.OuterIPv6Flowlabel).SetStep(1).SetCount(otgp.Params.TrafficFlowParam.OuterSrcFlowCount)
	}

	// Inner IP header configuration
	var innerIPv4Header gosnappi.FlowIpv4
	var innerIPv6Header gosnappi.FlowIpv6
	if otgp.Params.TrafficFlowParam.InnerProtocolType == "IPv4" {
		innerIPv4Header = flow.Packet().Add().Ipv4()
		innerIPv4Header.Src().Increment().SetStart(otgp.Params.TrafficFlowParam.InnerSrcStart).SetStep(otgp.Params.TrafficFlowParam.InnerSrcStep).SetCount(otgp.Params.TrafficFlowParam.InnerSrcFlowCount)
		innerIPv4Header.Dst().Increment().SetStart(otgp.Params.TrafficFlowParam.InnerDstStart).SetStep(otgp.Params.TrafficFlowParam.InnerDstStep).SetCount(otgp.Params.TrafficFlowParam.InnerDstFlowCount)
		innerIPv4Header.TimeToLive().SetValue(uint32(otgp.Params.TrafficFlowParam.InnerTTL))
		innerIPv4Header.Priority().Dscp().Phb().SetValue(uint32(otgp.Params.TrafficFlowParam.InnerDSCP))
		innerIPv4Header.Priority().Dscp().Ecn().SetValue(uint32(otgp.Params.TrafficFlowParam.InnerECN))
	} else if otgp.Params.TrafficFlowParam.InnerProtocolType == "IPv6" {
		innerIPv6Header = flow.Packet().Add().Ipv6()
		innerIPv6Header.Src().Increment().SetStart(otgp.Params.TrafficFlowParam.InnerSrcStart).SetStep(otgp.Params.TrafficFlowParam.InnerSrcStep).SetCount(otgp.Params.TrafficFlowParam.InnerSrcFlowCount)
		innerIPv6Header.Dst().Increment().SetStart(otgp.Params.TrafficFlowParam.InnerDstStart).SetStep(otgp.Params.TrafficFlowParam.InnerDstStep).SetCount(otgp.Params.TrafficFlowParam.InnerDstFlowCount)
		innerIPv6Header.HopLimit().SetValue(uint32(otgp.Params.TrafficFlowParam.InnerTTL))
		innerIPv6Header.TrafficClass().SetValue(uint32(otgp.Params.TrafficFlowParam.InnerDSCP << 2))
		innerIPv6Header.FlowLabel().Increment().SetStart(otgp.Params.TrafficFlowParam.InnerIPv6Flowlabel).SetStep(1).SetCount(otgp.Params.TrafficFlowParam.InnerSrcFlowCount)
	}

	// Layer4 header configuration
	var l4TCPHeader gosnappi.FlowTcp
	var l4UDPHeader gosnappi.FlowUdp
	// Randomize L4 ports
	if otgp.Params.TrafficFlowParam.L4PortRandom {
		if otgp.Params.TrafficFlowParam.L4TCP {
			// TCP header
			l4TCPHeader = flow.Packet().Add().Tcp()
			l4TCPHeader.SrcPort().Random().SetCount(otgp.Params.TrafficFlowParam.L4FlowCount)
			l4TCPHeader.SrcPort().Random().SetCount(otgp.Params.TrafficFlowParam.L4FlowCount)
		} else {
			// UDP header
			l4UDPHeader = flow.Packet().Add().Udp()
			l4UDPHeader.SrcPort().Random().SetCount(otgp.Params.TrafficFlowParam.L4FlowCount)
			l4UDPHeader.SrcPort().Random().SetCount(otgp.Params.TrafficFlowParam.L4FlowCount)
		}

	} else { // Use specified L4 port range
		if otgp.Params.TrafficFlowParam.L4TCP {
			// TCP header
			l4TCPHeader = flow.Packet().Add().Tcp()
			l4TCPHeader.SrcPort().Increment().SetStart(otgp.Params.TrafficFlowParam.L4SrcPortStart).SetStep(otgp.Params.TrafficFlowParam.L4FlowStep).SetCount(otgp.Params.TrafficFlowParam.L4FlowCount)
			l4TCPHeader.SrcPort().Increment().SetStart(otgp.Params.TrafficFlowParam.L4DstPortStart).SetStep(otgp.Params.TrafficFlowParam.L4FlowStep).SetCount(otgp.Params.TrafficFlowParam.L4FlowCount)
		} else {
			// UDP header
			l4UDPHeader = flow.Packet().Add().Udp()
			l4UDPHeader.SrcPort().Increment().SetStart(otgp.Params.TrafficFlowParam.L4SrcPortStart).SetStep(otgp.Params.TrafficFlowParam.L4FlowStep).SetCount(otgp.Params.TrafficFlowParam.L4FlowCount)
			l4UDPHeader.SrcPort().Increment().SetStart(otgp.Params.TrafficFlowParam.L4DstPortStart).SetStep(otgp.Params.TrafficFlowParam.L4FlowStep).SetCount(otgp.Params.TrafficFlowParam.L4FlowCount)
		}
	}

	return &TGENFlow{
		OTG: []gosnappi.Flow{flow},
	}
}
func (tg *TgenHelper) StartTraffic(t *testing.T, useOTG bool, allFlows *TGENFlow, trafficDuration time.Duration, topo *TGENTopology) {
	if useOTG {
		otg := ondatra.ATE(t, "ate").OTG()
		otgTopo := topo.OTG
		otgTopo.Flows().Clear().Items()
		otgTopo.Flows().Append(allFlows.OTG...)
		otg.PushConfig(t, otgTopo)
		otg.StopProtocols(t)

	} else {
		ateFlowList := allFlows.ATE
		ate := ondatra.ATE(t, "ate")
		t.Logf("*** Starting traffic ...")
		// ateTopo := topo.ATE
		// ateTopo.Update(t)
		ate.Traffic().Start(t, ateFlowList...)
		time.Sleep(trafficDuration)
		t.Logf("*** Stop traffic ...")
		ate.Traffic().Stop(t)
	}
}

// ConfigureTGEN selects the Tgen API ATE vs OTG ased on useOTG flag
func (h *TgenHelper) ConfigureTGEN(useOTG bool, param *TgenConfigParam) TGENConfig {
	if useOTG {
		return &OTGParam{Params: param}
	}
	return &ATEParam{Params: param}
}
