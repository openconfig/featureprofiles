package static_route_test

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen   = 30
	ipv4LBPrefixLen = 32
	ipv6PrefixLen   = 126
	DUT1_BASE_IPv4  = "190.0.1.1"
	DUT2_BASE_IPv4  = "190.0.1.2"
	ProtocolBGP     = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP
	ProtocolISIS    = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS
	ProtocolSTATIC  = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC
	AddressFamilyV4 = oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST
)

type RouterBGPAttrib struct {
	networkInstance string
	RouterID        string
	neighbor        string
	AS              uint32
}

var (
	DefaultInstance = *ciscoFlags.DefaultNetworkInstance
	dut1RouterBGP   = RouterBGPAttrib{
		networkInstance: DefaultInstance,
		RouterID:        "1.1.1.1",
		neighbor:        "2.2.2.2",
		AS:              62000,
	}
	dut2RouterBGP = RouterBGPAttrib{
		networkInstance: DefaultInstance,
		RouterID:        "2.2.2.2",
		neighbor:        "1.1.1.1",
		AS:              62000,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "10.0.0.2",
		IPv4Len: ipv4PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "11.0.0.2",
		IPv4Len: ipv4PrefixLen,
	}

	dut1Port1 = attrs.Attributes{
		Desc:    "dut1ate",
		IPv4:    "10.0.0.1",
		IPv4Len: ipv4PrefixLen,
	}

	dut2Port1 = attrs.Attributes{
		Desc:    "dut2ate",
		IPv4:    "11.0.0.1",
		IPv4Len: ipv4PrefixLen,
	}
)

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {

	var baseIPv4 string
	var loopbackIP string
	var routerBGP RouterBGPAttrib
	var DUTSysID string
	DUTAreaAddress := "47.0001"
	// var v4Prefix string
	// var v4NextHop string

	if dut.ID() == "dut1" {
		baseIPv4 = DUT1_BASE_IPv4
		loopbackIP = "1.1.1.1"
		routerBGP = dut1RouterBGP
		DUTSysID = "0000.0000.0001"
		// v4Prefix = fmt.Sprintf("%s/%d", "2.2.2.2", ipv4LBPrefixLen)
		// v4NextHop = DUT2_BASE_IPv4
	} else {
		baseIPv4 = DUT2_BASE_IPv4
		loopbackIP = "2.2.2.2"
		routerBGP = dut2RouterBGP
		DUTSysID = "0000.0000.0002"
		// v4Prefix = fmt.Sprintf("%s/%d", "1.1.1.1", ipv4LBPrefixLen)
		// v4NextHop = DUT1_BASE_IPv4
	}

	isisIntfNameList := configInterface(t, dut, baseIPv4)
	configLoopBackInterface(t, dut, loopbackIP)
	configRP(t, dut)
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	configRouterISIS(t, dut, DUTAreaAddress, DUTSysID, isisIntfNameList)
	configRouterBGP(t, dut, routerBGP)
	// // configLoopBackStaticRoute(t, dut, v4Prefix, v4NextHop)
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {

	topo := ate.Topology().New()
	p1 := ate.Port(t, "port1")
	i1 := topo.AddInterface(atePort1.Name).WithPort(p1)
	i1.IPv4().
		WithAddress(atePort1.IPv4CIDR()).
		WithDefaultGateway(dut1Port1.IPv4)
	p2 := ate.Port(t, "port2")
	i2 := topo.AddInterface(atePort2.Name).WithPort(p2)
	i2.IPv4().
		WithAddress(atePort2.IPv4CIDR()).
		WithDefaultGateway(dut2Port1.IPv4)

	intfs := topo.Interfaces()
	intfs[atePort1.Name].WithIPv4Loopback("3.3.3.3/32")
	intfs[atePort2.Name].WithIPv4Loopback("3.3.3.3/32")
	util.AddAteISISL2(t, topo, atePort1.Name, "45", "ISIS-3", 10, "3.3.3.3/32", 1)
	util.AddAteISISL2(t, topo, atePort2.Name, "46", "ISIS-2", 10, "3.3.3.3/32", 1)
	util.AddAteISISL2(t, topo, atePort1.Name, "47", "ISIS-30", 10, "30.30.30.1/32", 100)
	util.AddAteISISL2(t, topo, atePort2.Name, "48", "ISIS-31", 10, "31.31.31.1/32", 100)
	util.AddAteEBGPPeer(t, topo, atePort1.Name, "1.1.1.1", 64001, "BGP", atePort1.IPv4, "20.20.20.1/32", 100, true)
	util.AddAteEBGPPeer(t, topo, atePort2.Name, "2.2.2.2", 64001, "BGP", atePort2.IPv4, "21.21.21.1/32", 100, true)

	topo.Push(t)
	topo.StartProtocols(t)

	return topo
}

func configureTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, topo *ondatra.ATETopology) {

	srcEndPoint := topo.Interfaces()[atePort1.Name]
	var dstEndPoint ondatra.Endpoint
	dstEndPoint = topo.Interfaces()[atePort2.Name]

	bgp_flow := createTrafficFlow(t, ate, "Flow_BGP", srcEndPoint, dstEndPoint, "20.20.20.1", "21.21.21.1", 100)
	isis_flow := createTrafficFlow(t, ate, "Flow_ISIS", srcEndPoint, dstEndPoint, "30.30.30.1", "31.31.31.1", 100)
	var flows []*ondatra.Flow
	flows = append(flows, bgp_flow, isis_flow)
	// flows = append(flows, bgp_flow)
	validateTrafficFlow(t, ate, flows)
}

func createTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, flowName string,
	srcEndPoint *ondatra.Interface, dstEndPoint ondatra.Endpoint, srcPrefix, dstPrefix string, count uint32) *ondatra.Flow {

	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.SrcAddressRange().
		WithMin(srcPrefix).
		WithCount(count).
		WithStep("0.0.0.1")
	ipv4Header.DstAddressRange().
		WithMin(dstPrefix).
		WithCount(count).
		WithStep("0.0.0.1")
	flow := ate.Traffic().NewFlow(flowName).
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint)
	flow.WithFrameSize(300).
		WithFrameRateFPS(100).
		WithHeaders(ondatra.NewEthernetHeader(), ipv4Header)

	return flow
}

func validateTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, flows []*ondatra.Flow) {

	ate.Traffic().Start(t, flows...)
	time.Sleep(1 * time.Minute)
	// threshold := 0.90
	// stats := gnmi.GetAll(t, ate, gnmi.OC().InterfaceAny().Counters().State())
	// trafficPass := util.CheckTrafficPassViaPortPktCounter(stats, threshold)
	// fmt.Printf("Debug: trafficPass:%v", trafficPass)

	ate.Traffic().Stop(t)
	time.Sleep(1 * time.Minute)
	for _, flow := range flows {
		outpkt := gnmi.Get(t, ate, gnmi.OC().Flow(flow.Name()).Counters().OutPkts().State())
		inpkt := gnmi.Get(t, ate, gnmi.OC().Flow(flow.Name()).Counters().InPkts().State())
		t.Logf("Flow %s Input Packet Count: %v, Ouput Packet count:%v", flow.Name(), inpkt, outpkt)
	}
	time.Sleep(10 * time.Minute)
}

func configInterface(t *testing.T, dut *ondatra.DUTDevice, baseIPv4 string) []string {

	var isisIntfNameList []string
	intfNameList := getInterfaceNameList(t, dut)

	for i := 0; i < 6; i++ {
		var portAttrib = &attrs.Attributes{}
		portName := "port" + strconv.Itoa(i+1)
		port := intfNameList[i]
		intf := &oc.Interface{Name: &port}
		path := gnmi.OC().Interface(port)
		portAttrib.Desc = dut.ID() + portName
		portAttrib.IPv4 = baseIPv4
		baseIPv4 = getNewIP(baseIPv4)
		portAttrib.IPv4Len = ipv4PrefixLen
		if i >= 4 {
			portAttrib.Subinterface = 1
		} else {
			portAttrib.Subinterface = 0
			isisIntfNameList = append(isisIntfNameList, intfNameList[i])
		}
		gnmi.Replace(t, dut, path.Config(), configInterfaceIPv4DUT(intf, portAttrib))
	}

	p1 := dut.Port(t, "port7").Name()
	p2 := dut.Port(t, "port8").Name()
	p3 := dut.Port(t, "port9").Name()
	p4 := dut.Port(t, "port10").Name()

	i1 := &oc.Interface{Name: ygot.String("Bundle-Ether100")}
	i2 := &oc.Interface{Name: ygot.String("Bundle-Ether101")}

	pathb1m1 := gnmi.OC().Interface(p1)
	pathb1m2 := gnmi.OC().Interface(p2)
	pathb2m1 := gnmi.OC().Interface(p3)
	pathb2m2 := gnmi.OC().Interface(p4)

	var bundlePortAttrib = &attrs.Attributes{}

	bundlePortAttrib.Desc = dut.ID() + "Bundle-Ether100"
	bundlePortAttrib.IPv4 = baseIPv4
	baseIPv4 = getNewIP(baseIPv4)
	bundlePortAttrib.IPv4Len = ipv4PrefixLen
	bundlePortAttrib.Subinterface = 0

	pathb1 := gnmi.OC().Interface("Bundle-Ether100")
	gnmi.Replace(t, dut, pathb1.Config(), configInterfaceIPv4DUT(i1, bundlePortAttrib))
	BE100 := generateBundleMemberInterfaceConfig(p1, "Bundle-Ether100")
	gnmi.Replace(t, dut, pathb1m1.Config(), BE100)
	BE100 = generateBundleMemberInterfaceConfig(p2, "Bundle-Ether100")
	gnmi.Replace(t, dut, pathb1m2.Config(), BE100)
	isisIntfNameList = append(isisIntfNameList, "Bundle-Ether100")

	bundlePortAttrib.Desc = dut.ID() + "Bundle-Ether101"
	bundlePortAttrib.IPv4 = baseIPv4
	baseIPv4 = getNewIP(baseIPv4)
	bundlePortAttrib.IPv4Len = ipv4PrefixLen
	bundlePortAttrib.Subinterface = 1

	pathb2 := gnmi.OC().Interface("Bundle-Ether101")
	gnmi.Replace(t, dut, pathb2.Config(), configInterfaceIPv4DUT(i2, bundlePortAttrib))
	BE101 := generateBundleMemberInterfaceConfig(p3, "Bundle-Ether101")
	gnmi.Replace(t, dut, pathb2m1.Config(), BE101)
	BE101 = generateBundleMemberInterfaceConfig(p4, "Bundle-Ether101")
	gnmi.Replace(t, dut, pathb2m2.Config(), BE101)

	var portAttrib = &attrs.Attributes{}
	if dut.ID() == "dut1" {
		portAttrib = &dut1Port1
	} else {
		portAttrib = &dut2Port1
	}
	portName := "port11"
	port := dut.Port(t, portName).Name()
	intf := &oc.Interface{Name: &port}
	path := gnmi.OC().Interface(port)

	gnmi.Replace(t, dut, path.Config(), configInterfaceIPv4DUT(intf, portAttrib))
	isisIntfNameList = append(isisIntfNameList, port)

	op := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().SubinterfaceAny().Name().State())
	for _, val := range op {
		isisIntfNameList = append(isisIntfNameList, val)
	}

	return isisIntfNameList
}

func configInterfaceIPv4DUT(i *oc.Interface, a *attrs.Attributes) *oc.Interface {

	i.Description = ygot.String(a.Desc)
	s := &oc.Interface_Subinterface{}

	if i.GetName()[:6] == "Bundle" {
		i.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
		g := i.GetOrCreateAggregation()
		g.LagType = oc.IfAggregate_AggregationType_STATIC

	} else {
		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	}
	if a.Subinterface > 0 {
		s = i.GetOrCreateSubinterface(a.Subinterface)
		if a.Subinterface > 0 {
			s.GetOrCreateVlan().GetOrCreateMatch().
				GetOrCreateSingleTagged().SetVlanId(1)
		}
	} else {
		s = i.GetOrCreateSubinterface(0)
	}
	s4 := s.GetOrCreateIpv4()
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(a.IPv4Len)

	return i
}
func configLoopBackInterface(t *testing.T, dut *ondatra.DUTDevice, loopbackIP string) {

	var portAttrib = &attrs.Attributes{}

	lb := netutil.LoopbackInterface(t, dut, 0)
	portAttrib.IPv4 = loopbackIP
	portAttrib.IPv4Len = ipv4LBPrefixLen
	portAttrib.Subinterface = 0
	lo1 := portAttrib.NewOCInterface(lb, dut)
	lo1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback

	gnmi.Update(t, dut, gnmi.OC().Interface(lb).Config(), lo1)
}

func getInterfaceNameList(t *testing.T, dut *ondatra.DUTDevice) []string {

	var intfNameList []string

	ports := dut.Ports()
	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})
	sort.SliceStable(ports, func(i, j int) bool {
		return len(ports[i].ID()) < len(ports[j].ID())
	})
	for i := 0; i < len(ports); i++ {
		intfNameList = append(intfNameList, ports[i].Name())
	}

	return intfNameList
}

// func configLoopBackStaticRoute(t *testing.T, dut *ondatra.DUTDevice, v4Prefix, Basev4NextHop string) {

// 	ni := oc.NetworkInstance{Name: ygot.String(DefaultInstance)}
// 	static := ni.GetOrCreateProtocol(ProtocolSTATIC, DefaultInstance)
// 	sr := static.GetOrCreateStatic(v4Prefix)
// 	v4NextHop := Basev4NextHop

// 	for i := 0; i < 4; i++ {
// 		nh := sr.GetOrCreateNextHop(strconv.Itoa(i))
// 		nh.NextHop = oc.UnionString(v4NextHop)
// 		v4NextHop = getNewIP(v4NextHop)

// 		gnmi.Update(t, dut, gnmi.OC().NetworkInstance(DefaultInstance).
// 			Protocol(ProtocolSTATIC, DefaultInstance).Config(), static)
// 	}

// 	if dut.ID() == "dut1" {
// 		sr := static.GetOrCreateStatic("3.3.3.3/32")
// 		nh := sr.GetOrCreateNextHop("0")
// 		nh.NextHop = oc.UnionString("10.0.0.2")

// 		gnmi.Update(t, dut, gnmi.OC().NetworkInstance(DefaultInstance).
// 			Protocol(ProtocolSTATIC, DefaultInstance).Config(), static)
// 	}

// }
func configStaticRoute(t *testing.T, dut *ondatra.DUTDevice, v4Prefix, v4NextHop, v6Prefix, v6NextHop string, delete bool) {
	t.Logf("*** Configuring static route in DEFAULT network-instance ...")
	ni := oc.NetworkInstance{Name: ygot.String(DefaultInstance)}
	static := ni.GetOrCreateProtocol(ProtocolSTATIC, "STATIC")
	if v4Prefix != "" {
		sr := static.GetOrCreateStatic(v4Prefix)
		nh := sr.GetOrCreateNextHop("0")
		nh.NextHop = oc.UnionString(v4NextHop)
		if delete {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(DefaultInstance).
				Protocol(ProtocolSTATIC, "STATIC").Static(v4Prefix).Config())

		} else {
			gnmi.Update(t, dut, gnmi.OC().NetworkInstance(DefaultInstance).
				Protocol(ProtocolSTATIC, "STATIC").Config(), static)
		}
	}
	if v6Prefix != "" {
		sr := static.GetOrCreateStatic(v6Prefix)
		nh := sr.GetOrCreateNextHop("0")
		nh.NextHop = oc.UnionString(v6NextHop)
		if delete {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(DefaultInstance).
				Protocol(ProtocolSTATIC, "STATIC").Static(v6Prefix).Config())
		} else {
			gnmi.Update(t, dut, gnmi.OC().NetworkInstance(DefaultInstance).
				Protocol(ProtocolSTATIC, "STATIC").Config(), static)
		}
	}
}
func configRouterISIS(t *testing.T, dut *ondatra.DUTDevice, DUTAreaAddress, DUTSysID string, ifaceNameList []string) {

	dev := &oc.Root{}
	inst := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	prot := inst.GetOrCreateProtocol(ProtocolISIS, "ISIS")
	isis := prot.GetOrCreateIsis()
	glob := isis.GetOrCreateGlobal()
	glob.Net = []string{fmt.Sprintf("%v.%v.00", DUTAreaAddress, DUTSysID)}
	glob.LevelCapability = 2
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	// glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	// glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	for i := 0; i < len(ifaceNameList); i++ {
		intf := isis.GetOrCreateInterface(ifaceNameList[i])
		intf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
		intf.Enabled = ygot.Bool(true)
		intf.HelloPadding = 1
		intf.Passive = ygot.Bool(false)
		intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
		// intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	}
	intfLB := isis.GetOrCreateInterface("Loopback0")
	intfLB.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
	intfLB.Enabled = ygot.Bool(true)
	intfLB.HelloPadding = 1
	intfLB.Passive = ygot.Bool(false)
	intfLB.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)

	level := isis.GetOrCreateLevel(2)
	level.MetricStyle = 2

	dutNode := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(ProtocolISIS, "ISIS")
	dutConf := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance).GetOrCreateProtocol(ProtocolISIS, "ISIS")
	gnmi.Replace(t, dut, dutNode.Config(), dutConf)
}
func configRouterBGP(t *testing.T, dut *ondatra.DUTDevice, routerBGP RouterBGPAttrib) {

	dev := &oc.Root{}
	inst := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	prot := inst.GetOrCreateProtocol(ProtocolBGP, "BGP")
	bgp := prot.GetOrCreateBgp()
	glob := bgp.GetOrCreateGlobal()
	glob.As = ygot.Uint32(65000)
	if dut.ID() == "dut1" {
		glob.RouterId = ygot.String("1.1.1.1")
	} else {
		glob.RouterId = ygot.String("2.2.2.2")
	}
	glob.GetOrCreateGracefulRestart().Enabled = ygot.Bool(true)
	glob.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)

	pgATE := bgp.GetOrCreatePeerGroup("BGP-ATE-GROUP")
	pgATE.PeerAs = ygot.Uint32(64001)
	pgATE.LocalAs = ygot.Uint32(63001)
	pgATE.PeerGroupName = ygot.String("BGP-ATE-GROUP")

	peerATE := bgp.GetOrCreateNeighbor("3.3.3.3")
	peerATE.PeerGroup = ygot.String("BGP-ATE-GROUP")
	peerATE.GetOrCreateTransport().SetLocalAddress("Loopback0")
	peerATE.GetOrCreateEbgpMultihop().Enabled = ygot.Bool(true)
	peerATE.GetOrCreateEbgpMultihop().MultihopTtl = ygot.Uint8(255)
	peerATE.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	peerATE.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy().ImportPolicy = []string{"ALLOW"}
	peerATE.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy().ExportPolicy = []string{"ALLOW"}

	if dut.ID() == "dut1" {
		pg := bgp.GetOrCreatePeerGroup("BGP-PEER-GROUP")
		pg.PeerAs = ygot.Uint32(63001)
		pg.LocalAs = ygot.Uint32(63001)
		pg.PeerGroupName = ygot.String("BGP-PEER-GROUP")

		peer := bgp.GetOrCreateNeighbor("2.2.2.2")
		peer.PeerGroup = ygot.String("BGP-PEER-GROUP")
		peer.GetOrCreateTransport().SetLocalAddress("Loopback0")
		peer.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
		peer.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy().ImportPolicy = []string{"ALLOW"}
		peer.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy().ExportPolicy = []string{"ALLOW"}
	} else {
		pg := bgp.GetOrCreatePeerGroup("BGP-PEER-GROUP")
		pg.PeerAs = ygot.Uint32(63001)
		pg.LocalAs = ygot.Uint32(63001)
		pg.PeerGroupName = ygot.String("BGP-PEER-GROUP")

		peer := bgp.GetOrCreateNeighbor("1.1.1.1")
		peer.PeerGroup = ygot.String("BGP-PEER-GROUP")
		peer.GetOrCreateTransport().SetLocalAddress("Loopback0")
		peer.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
		peer.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy().ImportPolicy = []string{"ALLOW"}
		peer.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy().ExportPolicy = []string{"ALLOW"}
	}

	dutNode := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(ProtocolBGP, "BGP")
	dutConf := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance).GetOrCreateProtocol(ProtocolBGP, "BGP")
	gnmi.Replace(t, dut, dutNode.Config(), dutConf)
}

func configRP(t *testing.T, dut *ondatra.DUTDevice) {
	dev := &oc.Root{}
	inst := dev.GetOrCreateRoutingPolicy()
	pdef := inst.GetOrCreatePolicyDefinition("ALLOW")
	stmt1, _ := pdef.AppendNewStatement("1")
	stmt1.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	dutNode := gnmi.OC().RoutingPolicy()
	dutConf := dev.GetOrCreateRoutingPolicy()
	gnmi.Update(t, dut, dutNode.Config(), dutConf)
}
func getNewIP(ip string) string {

	newIP := net.ParseIP(ip)
	newIP = newIP.To4()
	newIP[0] += 1

	return newIP.String()
}

func generateBundleMemberInterfaceConfig(name, bundleID string) *oc.Interface {

	i := &oc.Interface{Name: ygot.String(name)}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	e := i.GetOrCreateEthernet()
	e.AutoNegotiate = ygot.Bool(false)
	e.AggregateId = ygot.String(bundleID)

	return i
}
