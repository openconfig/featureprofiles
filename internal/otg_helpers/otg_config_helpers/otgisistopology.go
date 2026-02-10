// otgisistopology.go is a helper for ISIS tests. It supports the below features:
// 1. Configure any number of LAGs that contains any number of ports.
// 2. Supports native and vlan based subinterfaces.
// 3. Configure any number of ISIS routers in the ATE.
package otgconfighelpers

import (
	"strconv"
	"testing"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/ondatra"
)

// AteEmulatedRouterData is the data structure for an emulated router in the ATE.
type AteEmulatedRouterData struct {
	Name                   string
	DUTIPv4                string
	ATEIPv4                string
	LinkIPv4PLen           int
	DUTIPv6                string
	ATEIPv6                string
	LinkIPv6PLen           int
	EthMAC                 string
	ISISAreaAddress        string
	ISISSysID              string
	V4Route                string
	V4RouteCount           int
	V6Route                string
	V6RouteCount           int
	VlanID                 int
	ISISBlocks             []*ISISOTGBlock
	ISISLSPRefreshInterval int
	ISISSPLifetime         int
}

// ATEPortData is the data structure for a port in the ATE.
type ATEPortData struct {
	Name           string
	Mac            string
	OndatraPortIdx int
	OndatraPorts   *ondatra.Port
}

// ATELagData is the data structure for a LAG in the ATE.
type ATELagData struct {
	Name     string
	Mac      string
	Ports    []ATEPortData
	Erouters []*AteEmulatedRouterData
}

// ATEData is the data structure for the ATE data.
type ATEData struct {
	ATE             *ondatra.ATEDevice
	Lags            []*ATELagData
	TrafficFlowsMap map[*AteEmulatedRouterData][]*AteEmulatedRouterData
	TrafficFlows    []gosnappi.Flow
	ConfigureISIS   bool
}

// AppendTrafficFlows appends the traffic flows to the ATE topology.
func (a *ATEData) AppendTrafficFlows(t *testing.T, top gosnappi.Config) {
	for s, ds := range a.TrafficFlowsMap {
		for _, d := range ds {
			a.TrafficFlows = append(a.TrafficFlows, createAllTrafficFlows(t, a.ATE, top, s, d, d.ISISBlocks)...)
		}
	}
}

// Pfx is the data structure for a prefix in the ATE.
type Pfx struct {
	FirstOctet string
	PfxLen     int
	Count      int
}

// ISISOTGBlock is the data structure for an for a block of simulated ISIS routers in the ATE.
type ISISOTGBlock struct {
	Name            string
	Col             int
	Row             int
	ISISIDFirstOct  string
	IPv4Lo0FirstOct string
	IPv6Lo0FirstOct string
	LinkIP4FirstOct int
	V4Pfx           Pfx
	V6Pfx           Pfx
	LinkMultiplier  int
}

func createAllTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, srcEmulatedRouter *AteEmulatedRouterData, dstEmulatedRouter *AteEmulatedRouterData, dstBlocks []*ISISOTGBlock) []gosnappi.Flow {
	t.Helper()
	var tfs []gosnappi.Flow
	for _, b := range dstBlocks {
		// The dst IPs of the flow will be all the link address of the remote block.
		// Below formula calculates the number of links in a block .
		linkCount := ((b.Col-1)*b.Row + (b.Row-1)*b.Col) * b.LinkMultiplier * 2
		tf := createAppendIPV4Flow(t, ate, top, srcEmulatedRouter.Name+"_to_"+b.Name+"_Flow",
			srcEmulatedRouter.ATEIPv4, 1, strconv.Itoa(b.LinkIP4FirstOct)+".0.0.0", linkCount,
			srcEmulatedRouter.Name+".Eth1", dstEmulatedRouter.Name+".Eth1",
			uint32(srcEmulatedRouter.VlanID))
		tfs = append(tfs, tf)
	}

	return tfs
}

func createAppendIPV4Flow(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, flowName string, srcIP string, srcCount int, dstIP string, dstCount int, srcEmulatedRouter, dstEmulatedRouter string, srcVlanID uint32) gosnappi.Flow {
	t.Helper()
	appendTop := gosnappi.NewConfigAppend()
	flow := appendTop.ConfigAppendList().Add().Flows().Add()
	flow.SetName(flowName)
	flow.TxRx().Device().SetTxNames([]string{srcEmulatedRouter + ".IPv4"}).SetRxNames([]string{dstEmulatedRouter + ".IPv4"})
	flow.Size().SetFixed(1400)
	flow.Metrics().SetEnable(true)
	flow.Rate().SetPercentage(0.1)
	flow.Packet().Add().Ethernet()
	if srcVlanID != 0 {
		flow.Packet().Add().Vlan().SetId(gosnappi.NewPatternFlowVlanId().SetValue(srcVlanID))
	}
	ipv4SrcPattern := gosnappi.NewPatternFlowIpv4SrcCounter()
	ipv4SrcPattern.SetStart(srcIP).SetCount(uint32(srcCount))

	ipv4DstPattern := gosnappi.NewPatternFlowIpv4DstCounter()
	ipv4DstPattern.SetStart(dstIP).SetCount(uint32(dstCount))

	ipHeader := flow.Packet().Add().Ipv4()
	ipHeader.Src().SetIncrement(ipv4SrcPattern)
	ipHeader.Dst().SetIncrement(ipv4DstPattern)
	t.Logf("Appending flow: %s", flowName)
	ate.OTG().AppendConfig(t, appendTop)

	return flow
}

func createISISBlock(t *testing.T, top gosnappi.Config, block *ISISOTGBlock, emulatedRouterIdx int) gosnappi.Config {
	t.Helper()

	gridSt := NewGridIsisData(top)
	gridSt.SetRow(block.Row).SetCol(block.Col).
		SetSystemIDFirstOctet(block.ISISIDFirstOct).
		SetLinkIP4FirstOctet(block.LinkIP4FirstOct).
		SetLinkMultiplier(block.LinkMultiplier).
		SetBlockName(block.Name)

	if block.V4Pfx.FirstOctet != "" {
		grigRouteV4 := gridSt.V4RouteInfo()
		grigRouteV4.SetAddressFirstOctet(block.V4Pfx.FirstOctet).
			SetPrefix(block.V4Pfx.PfxLen).
			SetCount(block.V4Pfx.Count)
	}

	if block.V6Pfx.FirstOctet != "" {
		gridRouteV6 := gridSt.V6RouteInfo()
		gridRouteV6.SetAddressFirstOctet(block.V6Pfx.FirstOctet).
			SetPrefix(block.V6Pfx.PfxLen).
			SetCount(block.V6Pfx.Count)
	}

	gridTopo, err := gridSt.GenerateTopology()
	if err != nil {
		t.Fatalf("failed to generate isis topology for otg: %v", err)
	}
	if err := gridTopo.Connect(top.Devices().Items()[emulatedRouterIdx], 0, 1, gridSt.NextLinkIP4ToUse()); err != nil {
		t.Fatalf("failed to connect isis topology to the emultaed router in the otg: %v", err)
	}

	return top
}

// ConfigureATE creates and pushes the base configuration for the interfaces and ISIS on the ATE.
// If the "ConfigureISIS" field in the AteData is false , ISIS will not be configured on the ATE.
func ConfigureATE(t *testing.T, ate *ondatra.ATEDevice, ateData *ATEData) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()

	var pmd100GFRPorts []string
	for i, l := range ateData.Lags {
		// Create LAG interface
		agg := top.Lags().Add().SetName(l.Name)
		agg.Protocol().Lacp().SetActorKey(1).SetActorSystemPriority(1).SetActorSystemId(l.Mac)
		for _, p := range l.Ports {
			op := ate.Port(t, "port"+strconv.Itoa(p.OndatraPortIdx+1))
			top.Ports().Add().SetName(op.ID())
			if op.PMD() == ondatra.PMD100GBASEFR {
				pmd100GFRPorts = append(pmd100GFRPorts, op.ID())
			}
			// Create Physical port member of the LAG
			aggMemberPort := agg.Ports().Add().SetPortName(op.ID())
			aggMemberPort.Lacp().SetActorActivity("active").SetActorPortNumber(uint32(1)).SetActorPortPriority(1).SetLacpduTimeout(0)
			aggMemberPort.Ethernet().SetMac(p.Mac).SetName(l.Name + "." + strconv.Itoa(i+1))
		}

		for _, er := range l.Erouters {
			// Create Emulated router
			t.Logf("Creating emulated router in the topology: %s", er.Name)
			emulatedRouter := top.Devices().Add().SetName(er.Name)
			emulatedRouterEthInt := emulatedRouter.Ethernets().Add().SetName(er.Name + ".Eth").SetMac(er.EthMAC)
			emulatedRouterEthInt.Connection().SetLagName(agg.Name())
			emulatedRouterEthInt.Ipv4Addresses().Add().SetName(er.Name + ".Eth1" + ".IPv4").SetAddress(er.ATEIPv4).SetGateway(er.DUTIPv4).SetPrefix(uint32(er.LinkIPv4PLen))
			emulatedRouterEthInt.Ipv6Addresses().Add().SetName(er.Name + ".Eth1" + ".IPv6").SetAddress(er.ATEIPv6).SetGateway(er.DUTIPv6).SetPrefix(uint32(er.LinkIPv6PLen))
			if er.VlanID != 0 {
				emulatedRouterEthInt.Vlans().Add().SetName(er.Name + ".Eth1" + ".VLAN").SetId(uint32(er.VlanID))
			}

			if ateData.ConfigureISIS {
				configureOTGISIS(t, emulatedRouter, er)
				emulatedRouterIdx := len(top.Devices().Items()) - 1
				for _, b := range er.ISISBlocks {
					t.Logf("Creating ISIS block %s and connecting it to the emulated router %s in the topology", b.Name, er.Name)
					top = createISISBlock(t, top, b, emulatedRouterIdx)
				}
			}
		}
	}

	// Disable FEC for 100G-FR ports because Novus does not support it.
	if len(pmd100GFRPorts) > 0 {
		l1Settings := top.Layer1().Add().SetName("L1").SetPortNames(pmd100GFRPorts)
		l1Settings.SetAutoNegotiate(true).SetIeeeMediaDefaults(false).SetSpeed("speed_100_gbps")
		autoNegotiate := l1Settings.AutoNegotiation()
		autoNegotiate.SetRsFec(false)
	}

	return top
}

func configureOTGISIS(t *testing.T, dev gosnappi.Device, eRouter *AteEmulatedRouterData) {
	t.Helper()
	isis := dev.Isis().SetSystemId(eRouter.ISISSysID).SetName(eRouter.Name + "_IXIA_Emulated")
	isis.Basic().SetHostname(isis.Name()).SetLearnedLspFilter(true)
	isis.Advanced().SetAreaAddresses([]string{eRouter.ISISAreaAddress})
	isis.Advanced().SetEnableHelloPadding(false)
	isis.Basic().SetEnableWideMetric(true)
	if eRouter.ISISLSPRefreshInterval != 0 {
		isis.Advanced().SetLspRefreshRate(uint32(eRouter.ISISLSPRefreshInterval))
	}
	if eRouter.ISISSPLifetime != 0 {
		isis.Advanced().SetLspLifetime(uint32(eRouter.ISISSPLifetime))
	}
	isisInt := isis.Interfaces().Add().
		SetEthName(dev.Ethernets().Items()[0].Name()).SetName(eRouter.Name + ".ISISInt").
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).SetMetric(10)

	isisInt.Advanced().SetEnable3WayHandshake(true)
	isisInt.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)
}
