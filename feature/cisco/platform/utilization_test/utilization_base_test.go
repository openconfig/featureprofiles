package utilization_test

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygot/ygot"
)

const (
	dutAS                   = 64500
	ateAS                   = 64501
	bgpRoutePolicyName      = "BGP-ROUTE-POLICY-ALLOW"
	minInnerDstPrefixBgp    = "202.1.0.1"
	v4mask                  = "32"
	totalBgpPfx             = 1
	bgpAs                   = 65000
	policyTypeBgp           = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP
	bgpAdvertisedRouteStart = "202.1.0.5"
)

var (
	portId        = uint32(10)
	ipv4PrefixLen = uint8(30)
	ipv6PrefixLen = uint8(126)
)

type PBROptions struct {
	// BackupNHG specifies the backup next-hop-group to be used when all next-hops are unavailable.
	SrcIP string
}

type Percentage float64

type TimeTicks64 uint64

type utilization struct {
	used                         uint64
	free                         uint64
	maxLimit                     uint64
	highWaterMark                uint64
	lastHighWaterMark            TimeTicks64
	oorRedThresholdPercentage    Percentage
	oorYellowThresholdPercentage Percentage
	resourceOrrState             string
	lastResourceOrrChange        TimeTicks64
}

var (
	dutSrc = attrs.Attributes{
		Desc:    "dutSrc",
		IPv4:    "100.121.1.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:1:1",
		IPv6Len: ipv6PrefixLen,
	}
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		IPv4:    "100.121.1.2",
		IPv4Len: ipv4PrefixLen,
		MAC:     "02:00:01:01:01:01",
		IPv6:    "2000::100:121:1:2",
		IPv6Len: ipv6PrefixLen,
	}
	dutDst = attrs.Attributes{
		Desc:    "dutDst",
		IPv4:    "100.122.1.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:122:1:1",
		IPv6Len: ipv6PrefixLen,
	}
	ateDst = attrs.Attributes{
		Name:    "ateDst",
		IPv4:    "100.122.1.2",
		IPv4Len: ipv4PrefixLen,
		MAC:     "02:00:02:01:01:01",
		IPv6:    "2000::100:122:1:2",
		IPv6Len: ipv6PrefixLen,
	}
)

func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})
	return ports
}

func configInterfaceDUT(i *oc.Interface, a *attrs.Attributes) *oc.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(ipv4PrefixLen)

	s6 := s.GetOrCreateIpv6()
	s6a := s6.GetOrCreateAddress(a.IPv6)
	s6a.PrefixLength = ygot.Uint8(ipv6PrefixLen)

	return i
}

// configureDUT configures port1-port8 on DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dutPorts := sortPorts(dut.Ports())
	d := gnmi.OC()

	// incoming interface is Bundle-Ether121 with only 1 member (port1)
	incoming := &oc.Interface{Name: ygot.String("Bundle-Ether121")}
	gnmi.Replace(t, dut, d.Interface(*incoming.Name).Config(), configInterfaceDUT(incoming, &dutSrc))
	srcPort := dutPorts[0]
	dutSource := generateBundleMemberInterfaceConfig(t, srcPort.Name(), *incoming.Name)
	gnmi.Replace(t, dut, gnmi.OC().Interface(srcPort.Name()).Config(), dutSource)

	outgoing := &oc.Interface{Name: ygot.String("Bundle-Ether122")}
	outgoingData := configInterfaceDUT(outgoing, &dutDst)
	g := outgoingData.GetOrCreateAggregation()
	g.LagType = oc.IfAggregate_AggregationType_LACP
	gnmi.Replace(t, dut, d.Interface(*outgoing.Name).Config(), configInterfaceDUT(outgoing, &dutDst))
	for _, port := range dutPorts[1:] {
		dutDest := generateBundleMemberInterfaceConfig(t, port.Name(), *outgoing.Name)
		gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Config(), dutDest)
	}
}

func generateBundleMemberInterfaceConfig(t *testing.T, name, bundleID string) *oc.Interface {
	t.Helper()
	i := &oc.Interface{Name: ygot.String(name)}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	e := i.GetOrCreateEthernet()
	e.AutoNegotiate = ygot.Bool(false)
	e.AggregateId = ygot.String(bundleID)
	return i
}

func configRoutePolicy(t *testing.T, dut *ondatra.DUTDevice) {
	dev := &oc.Root{}
	inst := dev.GetOrCreateRoutingPolicy()
	pdef := inst.GetOrCreatePolicyDefinition("ALLOW")
	stmt1, _ := pdef.AppendNewStatement("1")
	stmt1.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	dutNode := gnmi.OC().RoutingPolicy()
	dutConf := dev.GetOrCreateRoutingPolicy()
	gnmi.Update(t, dut, dutNode.Config(), dutConf)
}

func configBgp(t *testing.T, dut *ondatra.DUTDevice, neighbor string) {
	dev := &oc.Root{}
	inst := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	prot := inst.GetOrCreateProtocol(policyTypeBgp, *ciscoFlags.DefaultNetworkInstance)
	bgp := prot.GetOrCreateBgp()
	glob := bgp.GetOrCreateGlobal()
	glob.As = ygot.Uint32(bgpAs)
	glob.RouterId = ygot.String("1.1.1.1")
	glob.GetOrCreateGracefulRestart().Enabled = ygot.Bool(true)
	glob.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)

	pg := bgp.GetOrCreatePeerGroup("BGP-PEER-GROUP")
	pg.PeerAs = ygot.Uint32(64001)
	pg.LocalAs = ygot.Uint32(63001)
	pg.PeerGroupName = ygot.String("BGP-PEER-GROUP")

	peer := bgp.GetOrCreateNeighbor(neighbor)
	peer.PeerGroup = ygot.String("BGP-PEER-GROUP")
	peer.GetOrCreateEbgpMultihop().Enabled = ygot.Bool(true)
	peer.GetOrCreateEbgpMultihop().MultihopTtl = ygot.Uint8(255)
	peer.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	peer.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy().ImportPolicy = []string{"ALLOW"}
	peer.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy().ExportPolicy = []string{"ALLOW"}
}

func configureAcceptRoutePolicy(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pd := rp.GetOrCreatePolicyDefinition(bgpRoutePolicyName)
	st, err := pd.AppendNewStatement("id-1")
	if err != nil {
		t.Fatal(err)
	}
	st.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

func configureOTG(t *testing.T, otg *otg.OTG) (gosnappi.BgpV4Peer, gosnappi.DeviceIpv4, gosnappi.Config) {
	t.Helper()
	config := gosnappi.NewConfig()
	port1 := config.Ports().Add().SetName("port1")
	port2 := config.Ports().Add().SetName("port2")

	iDut1Dev := config.Devices().Add().SetName(ateSrc.Name)
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(ateSrc.Name + ".Eth").SetMac(ateSrc.MAC)
	iDut1Eth.Connection().SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName(ateSrc.Name + ".IPv4")
	iDut1Ipv4.SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).SetPrefix(uint32(ateSrc.IPv4Len))
	iDut1Ipv6 := iDut1Eth.Ipv6Addresses().Add().SetName(ateSrc.Name + ".IPv6")
	iDut1Ipv6.SetAddress(ateSrc.IPv6).SetGateway(dutSrc.IPv6).SetPrefix(uint32(ateSrc.IPv6Len))

	iDut2Dev := config.Devices().Add().SetName(ateDst.Name)
	iDut2Eth := iDut2Dev.Ethernets().Add().SetName(ateDst.Name + ".Eth").SetMac(ateDst.MAC)
	iDut2Eth.Connection().SetPortName(port2.Name())
	iDut2Ipv4 := iDut2Eth.Ipv4Addresses().Add().SetName(ateDst.Name + ".IPv4")
	iDut2Ipv4.SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).SetPrefix(uint32(ateDst.IPv4Len))

	iDut1Bgp := iDut1Dev.Bgp().SetRouterId(iDut1Ipv4.Address())
	iDut1Bgp4Peer := iDut1Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut1Ipv4.Name()).Peers().Add().SetName(ateSrc.Name + ".BGP4.peer")
	iDut1Bgp4Peer.SetPeerAddress(iDut1Ipv4.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	iDut1Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv4Prefix(true)

	t.Logf("Pushing config to ATE and starting protocols...")
	otg.PushConfig(t, config)
	time.Sleep(30 * time.Second)
	otg.StartProtocols(t)
	time.Sleep(30 * time.Second)

	return iDut1Bgp4Peer, iDut1Ipv4, config
}

// addRoute configures and starts BGP IPv4 routes on a traffic generator.
func addRoute(t *testing.T, otg *otg.OTG, bgpPeer gosnappi.BgpV4Peer, otgPort1 gosnappi.DeviceIpv4, otgConfig gosnappi.Config) {
	t.Helper()
	peerRoutes := bgpPeer.V4Routes().Add().SetName(ateSrc.Name + ".BGP4.Route")
	peerRoutes.SetNextHopIpv4Address(otgPort1.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	peerRoutes.Addresses().Add().
		SetAddress(bgpAdvertisedRouteStart).
		SetPrefix(uint32(ipv4PrefixLen)).
		SetCount(550000).SetStep(2)
	peerRoutes.Advanced().SetIncludeLocalPreference(false)

	otg.PushConfig(t, otgConfig)
	time.Sleep(30 * time.Second)
	otg.StartProtocols(t)
	time.Sleep(time.Minute)

}

func clearBGPRoutes(t *testing.T, otg *otg.OTG, bgpPeer gosnappi.BgpV6Peer, otgConfig gosnappi.Config) {
	bgpPeer.V6Routes().Clear()
	otg.PushConfig(t, otgConfig)
	time.Sleep(30 * time.Second)
	otg.StopProtocols(t)
	time.Sleep(time.Minute)
}

// configureBGPWithIncrementalNetworks configures a BGP peer with a series of incrementally defined network prefixes.
func configureBGPWithIncrementalNetworks(t *testing.T, otg *otg.OTG, otgConfig gosnappi.Config, bgpPeer gosnappi.BgpV4Peer, startAddress string, numNetworks int, step int) {
	t.Helper() // Marks this function as a helper to improve test logging accuracy.

	// Iterate through the number of networks to be added.
	for i := 0; i < numNetworks; i++ {
		// Calculate the current network address by incrementing the start address.
		address := fmt.Sprintf("%s%d", startAddress, i*step)

		// Add a new BGP route with the calculated network address.
		peerRoutes := bgpPeer.V4Routes().Add().SetName(fmt.Sprintf("BGP.Route.%d", i))
		peerRoutes.SetNextHopIpv4Address(address).
			SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
			SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
		peerRoutes.Addresses().Add().
			SetAddress(address).
			SetPrefix(uint32(ipv4PrefixLen)).
			SetCount(1).SetStep(uint32(step))
		peerRoutes.Advanced().SetIncludeLocalPreference(false)

		fmt.Printf("Configured BGP route for network: %s/%d\n", address, uint32(ipv4PrefixLen))
	}

	// Push the configuration to the traffic generator to apply the settings.
	otg.PushConfig(t, otgConfig)
	time.Sleep(30 * time.Second)
	otg.StartProtocols(t)
	time.Sleep(time.Minute) // Allow additional time for the protocols to stabilize.
}

type NextHopGroupConfig struct {
	IpAddress []string
}

// Define a type that implements the union interface
type NextHopUnionString struct {
	oc.RoutingPolicy_PolicyDefinition_Statement_Actions_BgpActions_SetNextHop_Union
	String string
}

func createNextHopUnion(ipAddress string) oc.RoutingPolicy_PolicyDefinition_Statement_Actions_BgpActions_SetNextHop_Union {
	return &NextHopUnionString{
		String: ipAddress,
	}
}
func configureNextHopGroup(t *testing.T, dut *ondatra.DUTDevice, nhgConfig []string) {
	t.Helper()

	root := &oc.Root{}
	routingPolicy := root.GetOrCreateRoutingPolicy()
	policyDefinition := routingPolicy.GetOrCreatePolicyDefinition("NGH1")

	statementName := "set-nexthop"
	statement, err := policyDefinition.AppendNewStatement(statementName)
	if err != nil {
		t.Fatalf("Failed to append statement: %v", err)
	}

	// Access the BGP actions and set the next hop
	bgpActions := statement.GetOrCreateActions().GetOrCreateBgpActions()

	var nhgconf NextHopGroupConfig
	for _, nh := range nhgconf.IpAddress {
		nextHopUnion := createNextHopUnion(nh)
		fmt.Printf("Setting next hop: %v\n", nextHopUnion)
		bgpActions.SetNextHop = nextHopUnion
	}

	// Push the configuration to the device
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), routingPolicy)
}

func componentUtilizations(t *testing.T, dut *ondatra.DUTDevice, comps []string) map[string]*utilization {
	t.Helper()

	utzs := map[string]*utilization{}
	for _, c := range comps {
		comp := gnmi.Get(t, dut, gnmi.OC().Component(c).State())
		res := comp.GetIntegratedCircuit().GetUtilization().GetResource("service_lp_attributes_table_0")
		utzs[c] = &utilization{
			used: res.GetUsed(),
			free: res.GetFree(),
		}
	}
	return utzs
}
