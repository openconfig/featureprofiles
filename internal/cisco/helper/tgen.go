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
	TrafficFlowParam []*TrafficFlowAttr
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
	topo.Push(t)
	time.Sleep(10 * time.Second) // Sleep for 10 seconds to allow ATE to apply the config
	topo.StartProtocols(t)

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
	var flows []*ondatra.Flow
	for _, trafficFlowAttr := range atep.Params.TrafficFlowParam {
		flow := ate.Traffic().NewFlow(trafficFlowAttr.FlowName)

		// Configure source port
		p1 := ate.Port(t, trafficFlowAttr.TgenSrcPort.Name)
		srcPort := topo.AddInterface(trafficFlowAttr.TgenSrcPort.Name).WithPort(p1)

		// Configure destination ports
		dstEndPoints := []ondatra.Endpoint{}
		for _, v := range trafficFlowAttr.TgenDstPorts {
			p := ate.Port(t, v)
			d := topo.AddInterface(v).WithPort(p)
			dstEndPoints = append(dstEndPoints, d)
		}

		// Ethernet header
		ethHeader := ondatra.NewEthernetHeader().WithSrcAddress(trafficFlowAttr.TgenSrcPort.MAC).WithDstAddress(trafficFlowAttr.DstMacAddress)

		// Outer IP header configuration
		var outerIPv4Header *ondatra.IPv4Header
		var outerIPv6Header *ondatra.IPv6Header
		if trafficFlowAttr.OuterProtocolType == "IPv4" {
			outerIPv4Header = ondatra.NewIPv4Header()
			outerIPv4Header.SrcAddressRange().WithStep(trafficFlowAttr.OuterSrcStep).WithMin(trafficFlowAttr.OuterSrcStart).WithCount(uint32(trafficFlowAttr.OuterSrcFlowCount))
			outerIPv4Header.DstAddressRange().WithStep(trafficFlowAttr.OuterDstStep).WithMin(trafficFlowAttr.OuterDstStart).WithCount(uint32(trafficFlowAttr.OuterDstFlowCount))
			outerIPv4Header.WithDSCP(trafficFlowAttr.OuterDSCP).WithTTL(trafficFlowAttr.OuterTTL).WithECN(trafficFlowAttr.OuterECN)
		} else if trafficFlowAttr.OuterProtocolType == "IPv6" {
			outerIPv6Header = ondatra.NewIPv6Header()
			outerIPv6Header.SrcAddressRange().WithStep(trafficFlowAttr.OuterSrcStep).WithMin(trafficFlowAttr.OuterSrcStart).WithCount(uint32(trafficFlowAttr.OuterSrcFlowCount))
			outerIPv6Header.DstAddressRange().WithStep(trafficFlowAttr.OuterDstStep).WithMin(trafficFlowAttr.OuterDstStart).WithCount(uint32(trafficFlowAttr.OuterDstFlowCount))
			outerIPv6Header.WithDSCP(trafficFlowAttr.OuterDSCP).WithHopLimit(trafficFlowAttr.OuterTTL).WithECN(trafficFlowAttr.OuterECN).FlowLabelRange().WithMin(trafficFlowAttr.OuterIPv6Flowlabel).WithRandom()
		}

		// Inner IP header configuration
		var innerIPv4Header *ondatra.IPv4Header
		var innerIPv6Header *ondatra.IPv6Header
		if trafficFlowAttr.InnerProtocolType == "IPv4" {
			innerIPv4Header = ondatra.NewIPv4Header()
			innerIPv4Header.SrcAddressRange().WithStep(trafficFlowAttr.InnerSrcStep).WithMin(trafficFlowAttr.InnerSrcStart).WithCount(uint32(trafficFlowAttr.InnerSrcFlowCount))
			innerIPv4Header.DstAddressRange().WithStep(trafficFlowAttr.InnerDstStep).WithMin(trafficFlowAttr.InnerDstStart).WithCount(uint32(trafficFlowAttr.InnerDstFlowCount))
			innerIPv4Header.WithDSCP(trafficFlowAttr.InnerDSCP).WithTTL(trafficFlowAttr.InnerTTL).WithECN(trafficFlowAttr.InnerECN)
		} else if trafficFlowAttr.InnerProtocolType == "IPv6" {
			innerIPv6Header = ondatra.NewIPv6Header()
			innerIPv6Header.SrcAddressRange().WithStep(trafficFlowAttr.InnerSrcStep).WithMin(trafficFlowAttr.InnerSrcStart).WithCount(uint32(trafficFlowAttr.InnerSrcFlowCount))
			innerIPv6Header.DstAddressRange().WithStep(trafficFlowAttr.InnerDstStep).WithMin(trafficFlowAttr.InnerDstStart).WithCount(uint32(trafficFlowAttr.InnerDstFlowCount))
			innerIPv6Header.WithDSCP(trafficFlowAttr.InnerDSCP).WithHopLimit(trafficFlowAttr.InnerTTL).WithECN(trafficFlowAttr.InnerECN).FlowLabelRange().WithMin(trafficFlowAttr.InnerIPv6Flowlabel).WithRandom()
		}

		// Layer4 header configuration
		var l4TCPHeader *ondatra.TCPHeader
		var l4UDPHeader *ondatra.UDPHeader
		// Randomize L4 ports
		if trafficFlowAttr.L4PortRandom {
			if trafficFlowAttr.L4TCP {
				// TCP header
				l4TCPHeader = ondatra.NewTCPHeader()
				l4TCPHeader.SrcPortRange().WithRandom().WithCount(trafficFlowAttr.L4FlowCount)
				l4TCPHeader.DstPortRange().WithRandom().WithCount(trafficFlowAttr.L4FlowCount)
			} else {
				// UDP header
				l4UDPHeader = ondatra.NewUDPHeader()
				l4UDPHeader.SrcPortRange().WithRandom().WithCount(trafficFlowAttr.L4FlowCount)
				l4UDPHeader.DstPortRange().WithRandom().WithCount(trafficFlowAttr.L4FlowCount)
			}

		} else { // Use specified L4 port range
			if trafficFlowAttr.L4TCP {
				// TCP header
				l4TCPHeader = ondatra.NewTCPHeader()
				l4TCPHeader.SrcPortRange().WithMin(trafficFlowAttr.L4SrcPortStart).WithStep(trafficFlowAttr.L4FlowStep).WithCount(trafficFlowAttr.L4FlowCount)
				l4TCPHeader.DstPortRange().WithMin(trafficFlowAttr.L4DstPortStart).WithStep(trafficFlowAttr.L4FlowStep).WithCount(trafficFlowAttr.L4FlowCount)
			} else {
				// UDP header
				l4UDPHeader = ondatra.NewUDPHeader()
				l4UDPHeader.SrcPortRange().WithMin(trafficFlowAttr.L4SrcPortStart).WithStep(trafficFlowAttr.L4FlowStep).WithCount(trafficFlowAttr.L4FlowCount)
				l4UDPHeader.DstPortRange().WithMin(trafficFlowAttr.L4DstPortStart).WithStep(trafficFlowAttr.L4FlowStep).WithCount(trafficFlowAttr.L4FlowCount)
			}
		}

		// Combine headers based on encapsulation type
		var l4Header ondatra.Header
		if trafficFlowAttr.L4TCP {
			l4Header = l4TCPHeader
		} else {
			l4Header = l4UDPHeader
		}

		if trafficFlowAttr.OuterProtocolType == "IPv4" && trafficFlowAttr.InnerProtocolType == "IPv4" {
			flow.WithSrcEndpoints(srcPort).
				WithHeaders(ethHeader, outerIPv4Header, innerIPv4Header, l4Header).
				WithDstEndpoints(dstEndPoints...).
				WithFrameRateFPS(trafficFlowAttr.TrafficPPS).WithFrameSize(trafficFlowAttr.PacketSize)
		} else if trafficFlowAttr.OuterProtocolType == "IPv4" && trafficFlowAttr.InnerProtocolType == "IPv6" {
			flow.WithSrcEndpoints(srcPort).
				WithHeaders(ethHeader, outerIPv4Header, innerIPv6Header, l4Header).
				WithDstEndpoints(dstEndPoints...).
				WithFrameRateFPS(trafficFlowAttr.TrafficPPS).WithFrameSize(trafficFlowAttr.PacketSize)
		} else if trafficFlowAttr.OuterProtocolType == "IPv6" && trafficFlowAttr.InnerProtocolType == "IPv4" {
			flow.WithSrcEndpoints(srcPort).
				WithHeaders(ethHeader, outerIPv6Header, innerIPv4Header, l4Header).
				WithDstEndpoints(dstEndPoints...).
				WithFrameRateFPS(trafficFlowAttr.TrafficPPS).WithFrameSize(trafficFlowAttr.PacketSize)
		} else if trafficFlowAttr.OuterProtocolType == "IPv6" && trafficFlowAttr.InnerProtocolType == "IPv6" {
			flow.WithSrcEndpoints(srcPort).
				WithHeaders(ethHeader, outerIPv6Header, innerIPv6Header, l4Header).
				WithDstEndpoints(dstEndPoints...).
				WithFrameRateFPS(trafficFlowAttr.TrafficPPS).WithFrameSize(trafficFlowAttr.PacketSize)
		} else if trafficFlowAttr.OuterProtocolType == "IPv4" {
			flow.WithSrcEndpoints(srcPort).
				WithHeaders(ethHeader, outerIPv4Header, l4Header).
				WithDstEndpoints(dstEndPoints...).
				WithFrameRateFPS(trafficFlowAttr.TrafficPPS).WithFrameSize(trafficFlowAttr.PacketSize)
		} else if trafficFlowAttr.OuterProtocolType == "IPv6" {
			flow.WithSrcEndpoints(srcPort).
				WithHeaders(ethHeader, outerIPv6Header, l4Header).
				WithDstEndpoints(dstEndPoints...).
				WithFrameRateFPS(trafficFlowAttr.TrafficPPS).WithFrameSize(trafficFlowAttr.PacketSize)
		} else {
			t.Log("No valid protocol type specified")

		}
		flows = append(flows, flow)
	}
	return &TGENFlow{
		ATE: flows,
	}
}

func (otgp *OTGParam) ConfigureTGENFlows(t *testing.T) *TGENFlow {
	topo := gosnappi.NewConfig()
	var flows []gosnappi.Flow
	for _, trafficFlowAttr := range otgp.Params.TrafficFlowParam {
		flow := topo.Flows().Add().SetName(trafficFlowAttr.FlowName)
		flow.Metrics().SetEnable(true)

		flow.TxRx().Port().SetTxName(trafficFlowAttr.TgenSrcPort.Name).SetRxNames(trafficFlowAttr.TgenDstPorts)
		// Ethernet header
		ethHeader := flow.Packet().Add().Ethernet()
		ethHeader.Src().SetValue(trafficFlowAttr.TgenSrcPort.MAC)
		ethHeader.Dst().SetValue(trafficFlowAttr.DstMacAddress)

		// Outer IP header configuration
		var outerIPv4Header gosnappi.FlowIpv4
		var outerIPv6Header gosnappi.FlowIpv6
		if trafficFlowAttr.OuterProtocolType == "IPv4" {
			outerIPv4Header = flow.Packet().Add().Ipv4()
			outerIPv4Header.Src().Increment().SetStart(trafficFlowAttr.OuterSrcStart).SetStep(trafficFlowAttr.OuterSrcStep).SetCount(trafficFlowAttr.OuterSrcFlowCount)
			outerIPv4Header.Dst().Increment().SetStart(trafficFlowAttr.OuterDstStart).SetStep(trafficFlowAttr.OuterDstStep).SetCount(trafficFlowAttr.OuterDstFlowCount)
			outerIPv4Header.TimeToLive().SetValue(uint32(trafficFlowAttr.OuterTTL))
			outerIPv4Header.Priority().Dscp().Phb().SetValue(uint32(trafficFlowAttr.OuterDSCP))
			outerIPv4Header.Priority().Dscp().Ecn().SetValue(uint32(trafficFlowAttr.OuterECN))
		} else if trafficFlowAttr.OuterProtocolType == "IPv6" {
			outerIPv6Header = flow.Packet().Add().Ipv6()
			outerIPv6Header.Src().Increment().SetStart(trafficFlowAttr.OuterSrcStart).SetStep(trafficFlowAttr.OuterSrcStep).SetCount(trafficFlowAttr.OuterSrcFlowCount)
			outerIPv6Header.Dst().Increment().SetStart(trafficFlowAttr.OuterDstStart).SetStep(trafficFlowAttr.OuterDstStep).SetCount(trafficFlowAttr.OuterDstFlowCount)
			outerIPv6Header.HopLimit().SetValue(uint32(trafficFlowAttr.OuterTTL))
			outerIPv6Header.TrafficClass().SetValue(uint32(trafficFlowAttr.OuterDSCP << 2))
			outerIPv6Header.FlowLabel().Increment().SetStart(trafficFlowAttr.OuterIPv6Flowlabel).SetStep(1).SetCount(trafficFlowAttr.OuterSrcFlowCount)
		}

		// Inner IP header configuration
		var innerIPv4Header gosnappi.FlowIpv4
		var innerIPv6Header gosnappi.FlowIpv6
		if trafficFlowAttr.InnerProtocolType == "IPv4" {
			innerIPv4Header = flow.Packet().Add().Ipv4()
			innerIPv4Header.Src().Increment().SetStart(trafficFlowAttr.InnerSrcStart).SetStep(trafficFlowAttr.InnerSrcStep).SetCount(trafficFlowAttr.InnerSrcFlowCount)
			innerIPv4Header.Dst().Increment().SetStart(trafficFlowAttr.InnerDstStart).SetStep(trafficFlowAttr.InnerDstStep).SetCount(trafficFlowAttr.InnerDstFlowCount)
			innerIPv4Header.TimeToLive().SetValue(uint32(trafficFlowAttr.InnerTTL))
			innerIPv4Header.Priority().Dscp().Phb().SetValue(uint32(trafficFlowAttr.InnerDSCP))
			innerIPv4Header.Priority().Dscp().Ecn().SetValue(uint32(trafficFlowAttr.InnerECN))
		} else if trafficFlowAttr.InnerProtocolType == "IPv6" {
			innerIPv6Header = flow.Packet().Add().Ipv6()
			innerIPv6Header.Src().Increment().SetStart(trafficFlowAttr.InnerSrcStart).SetStep(trafficFlowAttr.InnerSrcStep).SetCount(trafficFlowAttr.InnerSrcFlowCount)
			innerIPv6Header.Dst().Increment().SetStart(trafficFlowAttr.InnerDstStart).SetStep(trafficFlowAttr.InnerDstStep).SetCount(trafficFlowAttr.InnerDstFlowCount)
			innerIPv6Header.HopLimit().SetValue(uint32(trafficFlowAttr.InnerTTL))
			innerIPv6Header.TrafficClass().SetValue(uint32(trafficFlowAttr.InnerDSCP << 2))
			innerIPv6Header.FlowLabel().Increment().SetStart(trafficFlowAttr.InnerIPv6Flowlabel).SetStep(1).SetCount(trafficFlowAttr.InnerSrcFlowCount)
		}

		// Layer4 header configuration
		var l4TCPHeader gosnappi.FlowTcp
		var l4UDPHeader gosnappi.FlowUdp
		// Randomize L4 ports
		if trafficFlowAttr.L4PortRandom {
			if trafficFlowAttr.L4TCP {
				// TCP header
				l4TCPHeader = flow.Packet().Add().Tcp()
				l4TCPHeader.SrcPort().Random().SetCount(trafficFlowAttr.L4FlowCount)
				l4TCPHeader.SrcPort().Random().SetCount(trafficFlowAttr.L4FlowCount)
			} else {
				// UDP header
				l4UDPHeader = flow.Packet().Add().Udp()
				l4UDPHeader.SrcPort().Random().SetCount(trafficFlowAttr.L4FlowCount)
				l4UDPHeader.SrcPort().Random().SetCount(trafficFlowAttr.L4FlowCount)
			}

		} else { // Use specified L4 port range
			if trafficFlowAttr.L4TCP {
				// TCP header
				l4TCPHeader = flow.Packet().Add().Tcp()
				l4TCPHeader.SrcPort().Increment().SetStart(trafficFlowAttr.L4SrcPortStart).SetStep(trafficFlowAttr.L4FlowStep).SetCount(trafficFlowAttr.L4FlowCount)
				l4TCPHeader.SrcPort().Increment().SetStart(trafficFlowAttr.L4DstPortStart).SetStep(trafficFlowAttr.L4FlowStep).SetCount(trafficFlowAttr.L4FlowCount)
			} else {
				// UDP header
				l4UDPHeader = flow.Packet().Add().Udp()
				l4UDPHeader.SrcPort().Increment().SetStart(trafficFlowAttr.L4SrcPortStart).SetStep(trafficFlowAttr.L4FlowStep).SetCount(trafficFlowAttr.L4FlowCount)
				l4UDPHeader.SrcPort().Increment().SetStart(trafficFlowAttr.L4DstPortStart).SetStep(trafficFlowAttr.L4FlowStep).SetCount(trafficFlowAttr.L4FlowCount)
			}
		}
	}
	return &TGENFlow{
		OTG: flows,
	}
}

func (tg *TgenHelper) StartTraffic(t *testing.T, useOTG bool, allFlows *TGENFlow, trafficDuration time.Duration, topo *TGENTopology, dontReapplyTraffic bool) {
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
		if dontReapplyTraffic {
			ate.Traffic().Start(t)
			t.Log("PAUSE")
		} else {
			ate.Traffic().Start(t, ateFlowList...)
			t.Log("PAUSE")
		}
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
