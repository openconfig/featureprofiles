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
	OuterSrcFlowCount  int
	OuterDstFlowCount  int
	InnerSrcStart      string
	InnerDstStart      string
	InnerSrcStep       string
	InnerDstStep       string
	OuterIPv6Flowlabel uint32
	InnerIPv6Flowlabel uint32
	InnerSrcFlowCount  int
	InnerDstFlowCount  int
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
	t.Helper()
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

func (o *OTGParam) ConfigureTGENFlows(t *testing.T) *TGENFlow {
	t.Helper()
	otg := ondatra.ATE(t, "ate").OTG()
	topo := gosnappi.NewConfig()
	topo.Flows().Clear()
	// topo.Flows().Append(otgFlows...)
	flow := gosnappi.NewFlow()

	// Example logic for configuring flows
	for _, intf := range o.Params.TgenIntfAttr {
		flow.SetName("Flow_" + intf.Name)
		// Add more flow configuration logic here
	}

	t.Logf("Pushing flow config to OTG...")
	otg.PushConfig(t, topo)

	return &TGENFlow{
		OTG: []gosnappi.Flow{flow},
	}
}

func (tg *TgenHelper) StartTraffic(t *testing.T, useOTG bool, allFlows *TGENFlow, trafficDuration time.Duration) {
	if useOTG {
		// otg := ondatra.ATE(t, "ate").OTG()
		// otg.StartProtocols(t)
	} else {
		ateFlowList := allFlows.ATE
		ate := ondatra.ATE(t, "ate")
		t.Logf("*** Starting traffic ...")
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
