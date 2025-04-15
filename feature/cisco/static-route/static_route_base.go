package static_route_test

import (
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
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
	var v4Prefix string
	var v4NextHop string

	if dut.ID() == "dut1" {
		baseIPv4 = DUT1_BASE_IPv4
		loopbackIP = "1.1.1.1"
		routerBGP = dut1RouterBGP
		v4Prefix = fmt.Sprintf("%s/%d", "2.2.2.2", ipv4LBPrefixLen)
		v4NextHop = DUT2_BASE_IPv4
	} else {
		baseIPv4 = DUT2_BASE_IPv4
		loopbackIP = "2.2.2.2"
		routerBGP = dut2RouterBGP
		v4Prefix = fmt.Sprintf("%s/%d", "1.1.1.1", ipv4LBPrefixLen)
		v4NextHop = DUT1_BASE_IPv4
	}
	configInterface(t, dut, baseIPv4)
	configLoopBackInterface(t, dut, loopbackIP)
	configRouterBGP(t, dut, routerBGP)
	configLoopBackStaticRoute(t, dut, v4Prefix, v4NextHop)
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {

	topo := ate.Topology().New()
	t.Logf("Debug: Intf:%v", topo.Interfaces())

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

	topo.Push(t)
	time.Sleep(5 * time.Second)
	topo.StartProtocols(t)

	return topo
}

func configInterface(t *testing.T, dut *ondatra.DUTDevice, baseIPv4 string) {

	for i := 0; i < 4; i++ {
		var portAttrib = &attrs.Attributes{}
		portName := "port" + strconv.Itoa(i+1)
		port := dut.Port(t, portName).Name()
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
		}
		gnmi.Replace(t, dut, path.Config(), configInterfaceIPv4DUT(intf, portAttrib))
	}

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
func configLoopBackStaticRoute(t *testing.T, dut *ondatra.DUTDevice, v4Prefix, Basev4NextHop string) {

	ni := oc.NetworkInstance{Name: ygot.String(DefaultInstance)}
	static := ni.GetOrCreateProtocol(ProtocolSTATIC, DefaultInstance)
	sr := static.GetOrCreateStatic(v4Prefix)
	v4NextHop := Basev4NextHop

	for i := 0; i < 4; i++ {
		nh := sr.GetOrCreateNextHop(strconv.Itoa(i))
		nh.NextHop = oc.UnionString(v4NextHop)
		v4NextHop = getNewIP(v4NextHop)

		gnmi.Update(t, dut, gnmi.OC().NetworkInstance(DefaultInstance).
			Protocol(ProtocolSTATIC, DefaultInstance).Config(), static)
	}

}
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

func configRouterBGP(t *testing.T, dut *ondatra.DUTDevice, routerBGP RouterBGPAttrib) {

	root := &oc.Root{}
	dni := root.GetOrCreateNetworkInstance(routerBGP.networkInstance)

	bgpP := dni.GetOrCreateProtocol(ProtocolBGP, "BGP")
	bgpP.SetEnabled(true)
	bgp := bgpP.GetOrCreateBgp()

	g := bgp.GetOrCreateGlobal()
	g.SetAs(routerBGP.AS)
	g.SetRouterId(routerBGP.RouterID)
	g.GetOrCreateAfiSafi(AddressFamilyV4).Enabled = ygot.Bool(true)

	pgv4 := bgp.GetOrCreatePeerGroup("BGP-PEER")
	pgv4.PeerAs = ygot.Uint32(routerBGP.AS)
	pgv4.PeerGroupName = ygot.String("BGP-PEER")

	nV4 := bgp.GetOrCreateNeighbor(routerBGP.neighbor)
	nV4.SetPeerAs(routerBGP.AS)
	nV4.GetOrCreateAfiSafi(AddressFamilyV4).Enabled = ygot.Bool(true)
	nV4.SetEnabled(true)
	nV4.GetOrCreateTransport().SetLocalAddress("Loopback0")
	nV4.SetPeerGroup("BGP-PEER")

	for val, p := range dni.Protocol {
		t.Logf("Debug:Before config: protocol:%v val %v \n", val, p.GetEnabled())
	}

	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(routerBGP.networkInstance).Config(), dni)
	dniState := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(routerBGP.networkInstance).State())

	for val, p := range dniState.Protocol {
		t.Logf("Debug:After config: protocol:%v val %v \n", val, p.GetEnabled())
	}
	// t.Logf("Debug: dniState:%v \n ", dniState)

}

func getNewIP(ip string) string {

	newIP := net.ParseIP(ip)
	newIP = newIP.To4()
	newIP[0] += 1

	return newIP.String()
}
