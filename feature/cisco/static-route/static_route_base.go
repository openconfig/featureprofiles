package static_route_test

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cisco/config"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/networkinstance"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	DUT1_BASE_IPv4                    = "190.0.1.1"
	DUT2_BASE_IPv4                    = "190.0.1.2"
	DUT1_BASE_IPv6                    = "190:0:1::1"
	DUT2_BASE_IPv6                    = "190:0:1::2"
	REDIS_STATIC_ROUTE_BASE_IPv4      = "10.10.10.10"
	LOCAL_STATIC_ROUTE_BASE_IPv4      = "20.20.20.20"
	UNRSLV_STATIC_ROUTE_BASE_IPv4     = "30.30.30.30"
	LOCAL_STATIC_ROUTE_VRF_BASE_IPv4  = "40.40.40.40"
	UNRSLV_STATIC_ROUTE_VRF_BASE_IPv4 = "50.50.50.50"
	REDIS_STATIC_ROUTE_BASE_IPv6      = "10:10:10::10"
	LOCAL_STATIC_ROUTE_BASE_IPv6      = "20:20:20::20"
	UNRSLV_STATIC_ROUTE_BASE_IPv6     = "30:30:30::30"
	LOCAL_STATIC_ROUTE_VRF_BASE_IPv6  = "40:40:40::40"
	UNRSLV_STATIC_ROUTE_VRF_BASE_IPv6 = "50:50:50::50"
	nonDefaultVRF                     = "vrfStatic"
	defaultVRF                        = "DEFAULT"
	ipv4PrefixLen                     = 30
	ipv4LBPrefixLen                   = 32
	ipv6PrefixLen                     = 126
	ipv6LBPrefixLen                   = 128
	ProtocolBGP                       = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP
	ProtocolISIS                      = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS
	ProtocolSTATIC                    = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC
	AddressFamilyV4                   = oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST
	AddressFamilyV6                   = oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST
	ON_CHANGE_TIMEOUT                 = 30 * time.Second
)

var (
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "6.0.1.2",
		IPv6:    "6:0:1::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "7.0.1.2",
		IPv6:    "7:0:1::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dut1Port1 = attrs.Attributes{
		Desc:    "dut1ate",
		IPv4:    "6.0.1.1",
		IPv6:    "6:0:1::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dut2Port1 = attrs.Attributes{
		Desc:    "dut2ate",
		IPv4:    "7.0.1.1",
		IPv6:    "7:0:1::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
)

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {

	var connectedPort string
	var unresolvedPort string
	var redisV4Prefix string
	var redisV6Prefix string
	var localV4Prefix string
	var localV6Prefix string
	var unrslvV4Prefix string
	var unrslvV6Prefix string
	var loopbackIPv4 string
	var loopbackIPv6 string
	var baseIPv4 string
	var baseIPv6 string
	var DUTSysID string
	var portName string
	count := 5
	DUTAreaAddress := "47.0001"

	if dut.ID() == "dut1" {
		DUTSysID = "0000.0000.0001"
		// portName = "port11"
		portName = "dut1_ate_port1"
		connectedPort = dut.Port(t, portName).Name()
		baseIPv4 = DUT1_BASE_IPv4
		loopbackIPv4 = "1.1.1.1"
		redisV4Prefix = fmt.Sprintf("%s/%d", REDIS_STATIC_ROUTE_BASE_IPv4, ipv4LBPrefixLen)
		redisV6Prefix = fmt.Sprintf("%s/%d", REDIS_STATIC_ROUTE_BASE_IPv6, ipv6LBPrefixLen)
		baseIPv6 = DUT1_BASE_IPv6
		loopbackIPv6 = "1:1:1::1"
	} else {
		DUTSysID = "0000.0000.0002"
		portName := "dut2_ate_port1"
		connectedPort = dut.Port(t, portName).Name()
		unresolvedPort = "HundredGigE0/0/0/3"
		baseIPv4 = DUT2_BASE_IPv4
		loopbackIPv4 = "2.2.2.2"
		localV4Prefix = fmt.Sprintf("%s/%d", LOCAL_STATIC_ROUTE_BASE_IPv4, ipv4LBPrefixLen)
		localV6Prefix = fmt.Sprintf("%s/%d", LOCAL_STATIC_ROUTE_BASE_IPv6, ipv6LBPrefixLen)
		unrslvV4Prefix = fmt.Sprintf("%s/%d", UNRSLV_STATIC_ROUTE_BASE_IPv4, ipv4LBPrefixLen)
		unrslvV6Prefix = fmt.Sprintf("%s/%d", UNRSLV_STATIC_ROUTE_BASE_IPv6, ipv6LBPrefixLen)
		baseIPv6 = DUT2_BASE_IPv6
		loopbackIPv6 = "2:2:2::2"
	}

	isisIntfNameList := configInterface(t, dut, baseIPv4, baseIPv6)
	configLoopBackInterface(t, dut, loopbackIPv4, loopbackIPv6)
	configRP(t, dut)
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	configRouterISIS(t, dut, DUTAreaAddress, DUTSysID, isisIntfNameList)
	configRouterBGP(t, dut)
	var ipv4 bool
	if dut.ID() == "dut1" {
		ipv4 = true
		configBulkStaticRoute(t, dut, redisV4Prefix, connectedPort, count, ipv4, defaultVRF)
		ipv4 = false
		configBulkStaticRoute(t, dut, redisV6Prefix, connectedPort, count, ipv4, defaultVRF)
	} else {
		ipv4 = true
		configBulkStaticRoute(t, dut, localV4Prefix, connectedPort, count, ipv4, defaultVRF)
		configBulkStaticRoute(t, dut, unrslvV4Prefix, unresolvedPort, count, ipv4, defaultVRF)
		ipv4 = false
		configBulkStaticRoute(t, dut, localV6Prefix, connectedPort, count, ipv4, defaultVRF)
		configBulkStaticRoute(t, dut, unrslvV6Prefix, unresolvedPort, count, ipv4, defaultVRF)
	}
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
	util.AddAteISISL2(t, topo, atePort1.Name, "47", "ISIS-30", 10, "30.30.30.1/32", 5)
	util.AddAteISISL2(t, topo, atePort2.Name, "48", "ISIS-31", 10, "31.31.31.1/32", 5)
	util.AddAteEBGPPeer(t, topo, atePort1.Name, "1.1.1.1", 64001, "BGP", atePort1.IPv4, "20.20.20.1/32", 5, true)
	util.AddAteEBGPPeer(t, topo, atePort2.Name, "2.2.2.2", 64001, "BGP", atePort2.IPv4, "21.21.21.1/32", 5, true)

	topo.Push(t)
	topo.StartProtocols(t)

	return topo
}

func configureTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, topo *ondatra.ATETopology) {

	srcEndPoint := topo.Interfaces()[atePort1.Name]
	var dstEndPoint ondatra.Endpoint
	dstEndPoint = topo.Interfaces()[atePort2.Name]

	bgp_flow := createTrafficFlow(t, ate, "Flow_BGP", srcEndPoint, dstEndPoint, "20.20.20.1", "21.21.21.1", 5)
	isis_flow := createTrafficFlow(t, ate, "Flow_ISIS", srcEndPoint, dstEndPoint, "30.30.30.1", "31.31.31.1", 5)
	var flows []*ondatra.Flow
	flows = append(flows, bgp_flow, isis_flow)
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

	ate.Traffic().Stop(t)
	time.Sleep(1 * time.Minute)
	for _, flow := range flows {
		outpkt := gnmi.Get(t, ate, gnmi.OC().Flow(flow.Name()).Counters().OutPkts().State())
		inpkt := gnmi.Get(t, ate, gnmi.OC().Flow(flow.Name()).Counters().InPkts().State())
		t.Logf("Flow %s Input Packet Count: %v, Ouput Packet count:%v", flow.Name(), inpkt, outpkt)
	}
	time.Sleep(10 * time.Minute)
}

func configInterface(t *testing.T, dut *ondatra.DUTDevice, baseIPv4 string, baseIPv6 string) []string {

	var isisIntfNameList []string
	intfNameList := getInterfaceNameList(t, dut)

	for i := 0; i < 5; i++ {
		var portAttrib = &attrs.Attributes{}
		portName := "port" + strconv.Itoa(i+1)
		port := intfNameList[i]
		intf := &oc.Interface{Name: &port}
		path := gnmi.OC().Interface(port)
		portAttrib.Desc = dut.ID() + portName
		portAttrib.IPv4 = baseIPv4
		baseIPv4 = getNewIPv4(baseIPv4)
		portAttrib.IPv4Len = ipv4PrefixLen
		portAttrib.IPv6 = baseIPv6
		baseIPv6 = getNewIPv6(baseIPv6)
		portAttrib.IPv6Len = ipv6PrefixLen

		if i >= 4 {
			portAttrib.Subinterface = 1
		} else {
			portAttrib.Subinterface = 0
			isisIntfNameList = append(isisIntfNameList, intfNameList[i])
		}
		gnmi.Replace(t, dut, path.Config(), configInterfaceDUT(intf, portAttrib))
	}

	// p1 := dut.Port(t, "port7").Name()
	// p2 := dut.Port(t, "port8").Name()
	// p3 := dut.Port(t, "port9").Name()
	// p4 := dut.Port(t, "port10").Name()

	p1 := dut.Port(t, "port6").Name()
	p2 := dut.Port(t, "port7").Name()
	p3 := dut.Port(t, "port8").Name()
	p4 := dut.Port(t, "port9").Name()

	i1 := &oc.Interface{Name: ygot.String("Bundle-Ether100")}
	i2 := &oc.Interface{Name: ygot.String("Bundle-Ether101")}

	pathb1m1 := gnmi.OC().Interface(p1)
	pathb1m2 := gnmi.OC().Interface(p2)
	pathb2m1 := gnmi.OC().Interface(p3)
	pathb2m2 := gnmi.OC().Interface(p4)

	var bundlePortAttrib = &attrs.Attributes{}

	bundlePortAttrib.Desc = dut.ID() + "Bundle-Ether100"
	bundlePortAttrib.IPv4 = baseIPv4
	baseIPv4 = getNewIPv4(baseIPv4)
	bundlePortAttrib.IPv4Len = ipv4PrefixLen
	bundlePortAttrib.Subinterface = 0
	bundlePortAttrib.IPv6 = baseIPv6
	baseIPv6 = getNewIPv6(baseIPv6)
	bundlePortAttrib.IPv6Len = ipv6PrefixLen
	bundlePortAttrib.Subinterface = 0

	pathb1 := gnmi.OC().Interface("Bundle-Ether100")
	gnmi.Replace(t, dut, pathb1.Config(), configInterfaceDUT(i1, bundlePortAttrib))
	BE100 := generateBundleMemberInterfaceConfig(p1, "Bundle-Ether100")
	gnmi.Replace(t, dut, pathb1m1.Config(), BE100)
	BE100 = generateBundleMemberInterfaceConfig(p2, "Bundle-Ether100")
	gnmi.Replace(t, dut, pathb1m2.Config(), BE100)
	isisIntfNameList = append(isisIntfNameList, "Bundle-Ether100")

	bundlePortAttrib.Desc = dut.ID() + "Bundle-Ether101"
	bundlePortAttrib.IPv4 = baseIPv4
	baseIPv4 = getNewIPv4(baseIPv4)
	bundlePortAttrib.IPv4Len = ipv4PrefixLen
	bundlePortAttrib.Subinterface = 1
	bundlePortAttrib.IPv6 = baseIPv6
	baseIPv6 = getNewIPv6(baseIPv6)
	bundlePortAttrib.IPv6Len = ipv6PrefixLen
	bundlePortAttrib.Subinterface = 1

	pathb2 := gnmi.OC().Interface("Bundle-Ether101")
	gnmi.Replace(t, dut, pathb2.Config(), configInterfaceDUT(i2, bundlePortAttrib))
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
	// portName := "port11"
	var portName string
	if dut.ID() == "dut1" {
		portName = "dut1_ate_port1"
	} else {
		portName = "dut2_ate_port1"
	}
	port := dut.Port(t, portName).Name()
	intf := &oc.Interface{Name: &port}
	path := gnmi.OC().Interface(port)

	gnmi.Replace(t, dut, path.Config(), configInterfaceDUT(intf, portAttrib))
	isisIntfNameList = append(isisIntfNameList, port)

	op := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().SubinterfaceAny().Name().State())
	for _, val := range op {
		isisIntfNameList = append(isisIntfNameList, val)
	}

	return isisIntfNameList
}

func configInterfaceDUT(i *oc.Interface, a *attrs.Attributes) *oc.Interface {

	i.Description = ygot.String(a.Desc)
	s := &oc.Interface_Subinterface{
		Enabled: ygot.Bool(true),
	}

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
	s6 := s.GetOrCreateIpv6()
	s6a := s6.GetOrCreateAddress(a.IPv6)
	s6a.PrefixLength = ygot.Uint8(a.IPv6Len)

	return i
}
func configLoopBackInterface(t *testing.T, dut *ondatra.DUTDevice, loopbackIPv4 string, loopbackIPv6 string) {

	var portAttrib = &attrs.Attributes{}

	lb := netutil.LoopbackInterface(t, dut, 0)
	portAttrib.IPv4 = loopbackIPv4
	portAttrib.IPv4Len = ipv4LBPrefixLen
	portAttrib.IPv6 = loopbackIPv6
	portAttrib.IPv6Len = ipv6LBPrefixLen
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

func configBulkStaticRoute(t *testing.T, dut *ondatra.DUTDevice,
	prefix, nextHop string, count int, ipv4 bool, vrf string) {

	ni := oc.NetworkInstance{Name: ygot.String(vrf)}
	static := ni.GetOrCreateProtocol(ProtocolSTATIC, *ciscoFlags.DefaultNetworkInstance)
	sr := static.GetOrCreateStatic(prefix)
	nh := sr.GetOrCreateNextHop("0")
	nh.GetOrCreateInterfaceRef().Interface = ygot.String(nextHop)

	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(vrf).
		Protocol(ProtocolSTATIC, *ciscoFlags.DefaultNetworkInstance).Config(), static)

	for i := 0; i < count; i++ {
		tempV4Prefix := prefix
		if ipv4 == true {
			prefix = getNewStaticIPv4(prefix[:11]) + "/32"
			nextHop = tempV4Prefix[:11]
		} else {
			prefix = getNewStaticIPv6(prefix[:12]) + "/128"
			nextHop = tempV4Prefix[:12]
		}
		st := ni.GetOrCreateProtocol(ProtocolSTATIC, *ciscoFlags.DefaultNetworkInstance)
		sr := st.GetOrCreateStatic(prefix)
		nh := sr.GetOrCreateNextHop(strconv.Itoa(i + 1))
		nh.NextHop = oc.UnionString(nextHop)
		nh.Recurse = ygot.Bool(true)

		gnmi.Update(t, dut, gnmi.OC().NetworkInstance(vrf).
			Protocol(ProtocolSTATIC, *ciscoFlags.DefaultNetworkInstance).Config(), st)
	}
}

func configStaticRoute(t *testing.T, dut *ondatra.DUTDevice, noRecurse, recurse bool,
	interfaceName, prefix, nextHop, vrf string) (*oc.NetworkInstance_Protocol,
	*networkinstance.NetworkInstance_ProtocolPath) {

	ni := oc.NetworkInstance{Name: ygot.String(vrf)}
	path := gnmi.OC().NetworkInstance(vrf).
		Protocol(ProtocolSTATIC, *ciscoFlags.DefaultNetworkInstance)
	static := ni.GetOrCreateProtocol(ProtocolSTATIC, *ciscoFlags.DefaultNetworkInstance)
	sr := static.GetOrCreateStatic(prefix)
	nh := sr.GetOrCreateNextHop("0")
	nh.SetNextHop(oc.UnionString(nextHop))

	if noRecurse == false {
		nh.SetRecurse(recurse)
	}
	if interfaceName != "" {
		nh.GetOrCreateInterfaceRef().Interface = ygot.String(interfaceName)
	}

	return static, path
}

func configStaticRouteWithAttributes(t *testing.T, dut *ondatra.DUTDevice, recurse bool,
	interfaceName, prefix, nextHop string, metric, tag, distance uint32) (*oc.NetworkInstance_Protocol,
	*networkinstance.NetworkInstance_ProtocolPath) {

	ni := oc.NetworkInstance{Name: ygot.String(*ciscoFlags.DefaultNetworkInstance)}
	path := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).
		Protocol(ProtocolSTATIC, *ciscoFlags.DefaultNetworkInstance)
	static := ni.GetOrCreateProtocol(ProtocolSTATIC, *ciscoFlags.DefaultNetworkInstance)

	sr := static.GetOrCreateStatic(prefix)
	sr.SetSetTag(oc.UnionUint32((tag)))

	nh := sr.GetOrCreateNextHop("0")
	nh.SetNextHop(oc.UnionString(nextHop))
	nh.SetRecurse(recurse)
	nh.SetMetric(metric)
	nh.SetPreference(distance)
	if interfaceName != "" {
		nh.GetOrCreateInterfaceRef().Interface = ygot.String(interfaceName)
	}
	return static, path
}

func configStaticRouteNoRecurseWithAttributes(t *testing.T, dut *ondatra.DUTDevice,
	prefix, nextHop string, metric, tag, distance uint32) (*oc.NetworkInstance_Protocol,
	*networkinstance.NetworkInstance_ProtocolPath) {

	ni := oc.NetworkInstance{Name: ygot.String(*ciscoFlags.DefaultNetworkInstance)}
	path := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).
		Protocol(ProtocolSTATIC, *ciscoFlags.DefaultNetworkInstance)
	static := ni.GetOrCreateProtocol(ProtocolSTATIC, *ciscoFlags.DefaultNetworkInstance)

	sr := static.GetOrCreateStatic(prefix)
	sr.SetSetTag(oc.UnionUint32((tag)))

	nh := sr.GetOrCreateNextHop("0")
	nh.SetNextHop(oc.UnionString(nextHop))
	nh.SetMetric(metric)
	nh.SetPreference(distance)

	return static, path
}

func configStaticRouteBFD(t *testing.T, dut *ondatra.DUTDevice, recurse bool,
	interfaceName, prefix, nextHop string) (*oc.NetworkInstance_Protocol,
	*networkinstance.NetworkInstance_ProtocolPath) {

	ni := oc.NetworkInstance{Name: ygot.String(*ciscoFlags.DefaultNetworkInstance)}
	path := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).
		Protocol(ProtocolSTATIC, *ciscoFlags.DefaultNetworkInstance)
	static := ni.GetOrCreateProtocol(ProtocolSTATIC, *ciscoFlags.DefaultNetworkInstance)

	sr := static.GetOrCreateStatic(prefix)
	nh := sr.GetOrCreateNextHop("0")
	nh.GetOrCreateEnableBfd().SetEnabled(true)
	nh.SetNextHop(oc.UnionString(nextHop))
	nh.SetRecurse(recurse)
	if interfaceName != "" {
		nh.GetOrCreateInterfaceRef().Interface = ygot.String(interfaceName)
	}

	return static, path
}
func deleteStaticRoute(t *testing.T, dut *ondatra.DUTDevice, ipAf string) {

	path := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *ciscoFlags.DefaultNetworkInstance).
		StaticAny()
	delPath := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).
		Protocol(ProtocolSTATIC, *ciscoFlags.DefaultNetworkInstance)

	op := gnmi.GetAll(t, dut, path.State())

	batchConfig := &gnmi.SetBatch{}
	for _, val := range op {
		if val.GetPrefix()[:4] == "100." && ipAf == "ipv4" {
			gnmi.BatchDelete(batchConfig, delPath.Static(val.GetPrefix()).Config())
		} else if val.GetPrefix()[:4] == "100:" && ipAf == "ipv6" {
			gnmi.BatchDelete(batchConfig, delPath.Static(val.GetPrefix()).Config())
		}
	}
	batchConfig.Set(t, dut)
}

func configRouterISIS(t *testing.T, dut *ondatra.DUTDevice, DUTAreaAddress,
	DUTSysID string, ifaceNameList []string) {

	dev := &oc.Root{}
	inst := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	prot := inst.GetOrCreateProtocol(ProtocolISIS, "ISIS")
	isis := prot.GetOrCreateIsis()

	glob := isis.GetOrCreateGlobal()
	glob.Net = []string{fmt.Sprintf("%v.%v.00", DUTAreaAddress, DUTSysID)}
	glob.LevelCapability = 2
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)

	for i := 0; i < len(ifaceNameList); i++ {
		intf := isis.GetOrCreateInterface(ifaceNameList[i])
		intf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
		intf.Enabled = ygot.Bool(true)
		intf.HelloPadding = 1
		intf.Passive = ygot.Bool(false)
		intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
		intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)

	}
	intfLB := isis.GetOrCreateInterface("Loopback0")
	intfLB.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
	intfLB.Enabled = ygot.Bool(true)
	intfLB.HelloPadding = 1
	intfLB.Passive = ygot.Bool(false)
	intfLB.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	intfLB.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)

	level := isis.GetOrCreateLevel(2)
	level.MetricStyle = 2

	dutNode := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(ProtocolISIS, "ISIS")
	dutConf := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance).GetOrCreateProtocol(ProtocolISIS, "ISIS")
	gnmi.Replace(t, dut, dutNode.Config(), dutConf)
}
func configRouterBGP(t *testing.T, dut *ondatra.DUTDevice) {

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
	glob.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

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
	peerATE.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	peerATE.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).GetOrCreateApplyPolicy().ImportPolicy = []string{"ALLOW"}
	peerATE.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).GetOrCreateApplyPolicy().ExportPolicy = []string{"ALLOW"}

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
		peer.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
		peer.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).GetOrCreateApplyPolicy().ImportPolicy = []string{"ALLOW"}
		peer.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).GetOrCreateApplyPolicy().ExportPolicy = []string{"ALLOW"}

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
		peer.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
		peer.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).GetOrCreateApplyPolicy().ImportPolicy = []string{"ALLOW"}
		peer.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).GetOrCreateApplyPolicy().ExportPolicy = []string{"ALLOW"}
	}

	dutNode := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(ProtocolBGP, "BGP")
	dutConf := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance).GetOrCreateProtocol(ProtocolBGP, "BGP")
	gnmi.Replace(t, dut, dutNode.Config(), dutConf)

	if dut.ID() == "dut1" {
		cliConfig := "router bgp 65000 instance BGP\n address-family ipv4 unicast\n  redistribute static route-policy ALLOW\n !\n!\n"
		config.TextWithGNMI(context.Background(), t, dut, cliConfig)
		cliConfig = "router bgp 65000 instance BGP\n address-family ipv6 unicast\n  redistribute static route-policy ALLOW\n !\n!\n"
		config.TextWithGNMI(context.Background(), t, dut, cliConfig)
	}
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

func configVRF(t *testing.T, dut *ondatra.DUTDevice) {

	dev := &oc.Root{}
	inst := dev.GetOrCreateNetworkInstance(nonDefaultVRF)
	inst.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	inst.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "DEFAULT")
	// vrfIntf := inst.GetOrCreateInterface(dut.Port(t, "port12").Name())
	// vrfIntf.SetInterface(dut.Port(t, "port12").Name())
	vrfIntf := inst.GetOrCreateInterface(dut.Port(t, "dut2_ate_port2").Name())
	vrfIntf.SetInterface(dut.Port(t, "dut2_ate_port2").Name())

	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(nonDefaultVRF).Config(), inst)

}

func configVRFInterface(t *testing.T, dut *ondatra.DUTDevice) {

	var portAttrib = &attrs.Attributes{}
	// portName := dut.Port(t, "port11").Name()

	// ipv4 := gnmi.GetAll(t, dut, gnmi.OC().Interface(portName).Subinterface(0).Ipv4().AddressAny().State())
	// ipv6_test := gnmi.Get(t, dut, gnmi.OC().Interface(portName).Subinterface(0).Ipv6().Address("6:0:1::1").State())
	// fmt.Printf("Debug:ipv6_test:%v\n", *ipv6_test.Ip)
	// ipv6 := gnmi.GetAll(t, dut, gnmi.OC().Interface(portName).Subinterface(0).Ipv6().AddressAny().State())

	// vrfPortName := dut.Port(t, "port12").Name()
	vrfPortName := dut.Port(t, "dut2_ate_port2").Name()
	vrfIntf := &oc.Interface{Name: &vrfPortName}
	path := gnmi.OC().Interface(vrfPortName)
	// portAttrib.Desc = dut.ID() + vrfPortName
	// portAttrib.IPv4 = ipv4[0].GetIp()
	// portAttrib.IPv4Len = ipv4[0].GetPrefixLength()
	// portAttrib.IPv6 = ipv6[0].GetIp()
	// portAttrib.IPv6Len = ipv6[0].GetPrefixLength()

	portAttrib.Desc = dut.ID() + vrfPortName
	portAttrib.IPv4 = dut1Port1.IPv4
	portAttrib.IPv4Len = dut1Port1.IPv4Len
	portAttrib.IPv6 = dut1Port1.IPv6
	portAttrib.IPv6Len = dut1Port1.IPv6Len

	i := configInterfaceDUT(vrfIntf, portAttrib)

	gnmi.Replace(t, dut, path.Config(), i)
}

func getNewIPv4(ip string) string {

	newIP := net.ParseIP(ip)
	newIP = newIP.To4()
	newIP[0] += 1

	return newIP.String()
}

func getScaleNewIPv4(ip string) string {

	newIP := net.ParseIP(ip)
	newIP = newIP.To4()
	newIP[3] += 1

	return newIP.String()
}

func getNewStaticIPv4(ip string) string {

	newIP := net.ParseIP(ip)
	newIP = newIP.To4()
	newIP[0] += 1
	newIP[1] += 1
	newIP[2] += 1
	newIP[3] += 1

	return newIP.String()
}

func getNewIPv6(ip string) string {

	newIP := net.ParseIP(ip)
	newIP = newIP.To16()
	newIP[1] += 1

	return newIP.String()
}

func getScaleNewIPv6(ip string) string {

	newIP := net.ParseIP(ip)
	newIP = newIP.To16()
	newIP[15] += 1

	return newIP.String()
}

func getNewStaticIPv6(ip string) string {

	newIP := net.ParseIP(ip)
	newIP = newIP.To16()
	newIP[1] += 1
	newIP[3] += 1
	newIP[5] += 1
	newIP[15] += 1

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

func gnmiOptsForOnChange(t *testing.T, dut *ondatra.DUTDevice) *gnmi.Opts {
	return dut.GNMIOpts().WithYGNMIOpts(
		ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE))
}

func gnmiOptsForSample(t *testing.T, dut *ondatra.DUTDevice, interval time.Duration) *gnmi.Opts {
	return dut.GNMIOpts().WithYGNMIOpts(
		ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_SAMPLE),
		ygnmi.WithSampleInterval(interval),
	)
}

func showRouteCLI(t *testing.T, dut *ondatra.DUTDevice, cliHandle binding.CLIClient,
	ipAf, prefix string, static ...string) (binding.CommandResult, error) {

	if len(static) > 0 {
		cli := fmt.Sprintf("show route %s unicast %s\n", ipAf, static[0])
		ctx, _ := context.WithTimeout(context.Background(), time.Second*5)

		return cliHandle.RunCommand(ctx, cli)
	} else {
		cli := fmt.Sprintf("show route %s unicast %s\n", ipAf, prefix)
		ctx, _ := context.WithTimeout(context.Background(), time.Second*5)

		return cliHandle.RunCommand(ctx, cli)
	}
}

func showRouteVRFCLI(t *testing.T, dut *ondatra.DUTDevice, cliHandle binding.CLIClient,
	vrf, ipAf, prefix string, static ...string) (binding.CommandResult, error) {

	if len(static) > 0 {
		cli := fmt.Sprintf("show route vrf %s %s unicast %s\n", vrf, ipAf, static[0])
		ctx, _ := context.WithTimeout(context.Background(), time.Second*5)

		return cliHandle.RunCommand(ctx, cli)
	} else {
		cli := fmt.Sprintf("show route vrf %s %s unicast %s\n", vrf, ipAf, prefix)
		ctx, _ := context.WithTimeout(context.Background(), time.Second*5)

		return cliHandle.RunCommand(ctx, cli)
	}
}

func extractPrefixes(input, ipAf string) []string {

	var matches []string

	if ipAf == "ipv4" {
		regex := regexp.MustCompile(`\b\d{1,3}(\.\d{1,3}){3}/\d{1,2}\b`)
		matches = regex.FindAllString(input, -1)
	} else {
		regex := regexp.MustCompile(`(?m)^S\s+([0-9a-fA-F:]+(::)?[0-9a-fA-F]*)/\d{1,3}`)
		tempMatches := regex.FindAllStringSubmatch(input, -1)
		for _, match := range tempMatches {
			matches = append(matches, match[0][5:])
		}
	}

	return matches
}
