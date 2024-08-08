package ppc_test

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/schemaless"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	dst                   = "202.1.0.1"
	v4mask                = "32"
	dstCount              = 1
	totalBgpPfx           = 1
	minInnerDstPrefixBgp  = "202.1.0.1"
	totalIsisPrefix       = 1 //set value for scale isis setup ex: 10000
	minInnerDstPrefixIsis = "201.1.0.1"
	ipv4PrefixLen         = 30
	ipv6PrefixLen         = 126
	policyTypeIsis        = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS
	dutAreaAddress        = "47.0001"
	dutSysId              = "0000.0000.0001"
	isisName              = "osisis"
	policyTypeBgp         = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP
	bgpAs                 = 65000
)

// Testcase defines the parameters related to a testcase
type Testcase struct {
	name      string
	flow      *ondatra.Flow
	eventType eventType // events for creating the trigger scenario
}

type eventType interface {
	IsEventType()
}

type eventZeroTtl struct {
	zeroTtlTrafficFlow bool
}

func (eventArgs eventZeroTtl) IsEventType() {}

func (eventArgs eventZeroTtl) modifyTrafficTtl(t *testing.T, args *testArgs) *ondatra.Flow {
	t.Helper()
	args.ate.Traffic().Stop(t) // stop the valid ipv4 flow before creating new flow
	time.Sleep(10 * time.Millisecond)
	return args.createFlow("invalid_flow_with_0_ttl", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true, ttl: true})
}

type eventAclConfig struct {
	aclName string
	config  bool
}

func (eventArgs eventAclConfig) IsEventType() {}

func (eventArgs eventAclConfig) aclConfig(t *testing.T) {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	cliPath, err := schemaless.NewConfig[string]("", "cli")
	if err != nil {
		t.Fatalf("Failed to create CLI ygnmi query: %v", err)
	}
	var aclConfig string
	if eventArgs.config {
		aclConfig = fmt.Sprintf("ipv4 access-list %v 1 deny any", eventArgs.aclName)
	} else {
		aclConfig = fmt.Sprintf("no ipv4 access-list %v 1 deny any", eventArgs.aclName)
	}
	gnmi.Update(t, dut, cliPath, aclConfig)
}

type eventInterfaceConfig struct {
	config bool
	shut   bool
	mtu    int
	port   []*ondatra.Port
}

func (eventArgs eventInterfaceConfig) IsEventType() {}

func (eventArgs eventInterfaceConfig) interfaceConfig(t *testing.T) {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	cliPath, err := schemaless.NewConfig[string]("", "cli")
	if err != nil {
		t.Fatalf("Failed to create CLI ygnmi query: %v", err)
	}
	for _, port := range eventArgs.port {
		if eventArgs.config {
			if eventArgs.shut {
				if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
					gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Enabled().Config(), false)
				}); errMsg != nil {
					gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Enabled().Config(), false)
				}
			}
			if eventArgs.mtu != 0 {
				mtu := fmt.Sprintf("interface bundle-Ether 122 mtu %d", eventArgs.mtu)
				gnmi.Update(t, dut, cliPath, mtu)
			}
		} else {
			// following reload need to try twice
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Enabled().Config(), true)
			}); errMsg != nil {
				gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Enabled().Config(), true)
			}
			if eventArgs.mtu != 0 {
				mtu := fmt.Sprintf("no interface bundle-Ether 122 mtu %d", eventArgs.mtu)
				gnmi.Update(t, dut, cliPath, mtu)
			}
		}
	}
}

type eventStaticRouteToNull struct {
	prefix string
	config bool
}

func (eventArgs eventStaticRouteToNull) IsEventType() {}

func (eventArgs eventStaticRouteToNull) staticRouteToNull(t *testing.T) {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	cliPath, err := schemaless.NewConfig[string]("", "cli")
	if err != nil {
		t.Fatalf("Failed to create CLI ygnmi query: %v", err)
	}
	var staticRoute string
	if eventArgs.config {
		staticRoute = fmt.Sprintf("router static address-family ipv4 unicast %s null 0", eventArgs.prefix)
	} else {
		staticRoute = fmt.Sprintf("no router static address-family ipv4 unicast %s null 0", eventArgs.prefix)
	}
	gnmi.Update(t, dut, cliPath, staticRoute)
}

type eventEnableMplsLdp struct {
	config bool
}

func (eventArgs eventEnableMplsLdp) IsEventType() {}

func (eventArgs eventEnableMplsLdp) enableMplsLdp(t *testing.T) {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	cliPath, err := schemaless.NewConfig[string]("", "cli")
	if err != nil {
		t.Fatalf("Failed to create CLI ygnmi query: %v", err)
	}
	var mplsLdp string
	if eventArgs.config {
		mplsLdp = "mpls ldp interface bundle-Ether 121"
	} else {
		mplsLdp = "no mpls ldp"
	}
	gnmi.Update(t, dut, cliPath, mplsLdp)
}

// sortPorts sorts the given slice of ports by the testbed port ID in ascending order.
func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})
	return ports
}

// tgnOptions are optional parameters to a validate traffic function.
type tgnOptions struct {
	drop, mpls, ipv4, ttl bool
	trafficTimer          int
	fps                   uint64
	fpercent              float64
	frameSize             uint32
	event                 eventType
}

// configureATE configures ports on the ATE.
// port 1 is source port
// ports 2-8 are destination ports
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	atePorts := sortPorts(ate.Ports())
	top := ate.Topology().New()

	ateSource := atePorts[0]
	i1 := top.AddInterface(ateSrc.Name).WithPort(ateSource)
	i1.IPv4().
		WithAddress(ateSrc.IPv4CIDR()).
		WithDefaultGateway(dutSrc.IPv4)
	i1.IPv6().
		WithAddress(ateSrc.IPv6CIDR()).
		WithDefaultGateway(dutSrc.IPv6)

	i2 := top.AddInterface(ateDst.Name)
	lag := top.AddLAG("lag").WithPorts(atePorts[1:]...)
	lag.LACP().WithEnabled(true)
	i2.WithLAG(lag)

	// Disable FEC for 100G-FR ports because Novus does not support it.
	if ateSource.PMD() == ondatra.PMD100GBASEFR {
		i1.Ethernet().FEC().WithEnabled(false)
	}
	is100gfr := false
	for _, p := range atePorts[1:] {
		if p.PMD() == ondatra.PMD100GBASEFR {
			is100gfr = true
		}
	}
	if is100gfr {
		i2.Ethernet().FEC().WithEnabled(false)
	}

	i2.IPv4().
		WithAddress(ateDst.IPv4CIDR()).
		WithDefaultGateway(dutDst.IPv4)
	i2.IPv6().
		WithAddress(ateDst.IPv6CIDR()).
		WithDefaultGateway(dutDst.IPv6)
	//top.Update(t)
	top.Push(t).StartProtocols(t)
	return top
}

// configAteIsisL2 configures ISIS on the ATE
func configAteIsisL2(t *testing.T, topo *ondatra.ATETopology, atePort, areaId, networkName string, metric uint32, v4prefix string, count uint32) {
	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	network := intfs[atePort].AddNetwork(networkName)
	network.ISIS().WithIPReachabilityMetric(metric + 1)
	network.IPv4().WithAddress(v4prefix).WithCount(count)
	rNetwork := intfs[atePort].AddNetwork("recursive")
	rNetwork.ISIS().WithIPReachabilityMetric(metric + 1)
	rNetwork.IPv4().WithAddress("100.100.100.100/32")
	intfs[atePort].ISIS().WithAreaID(areaId).WithLevelL2().WithNetworkTypePointToPoint().WithMetric(metric).WithWideMetricEnabled(true)
}

// configAteEbgpPeer configures EBGP on the ATE
func configAteEbgpPeer(t *testing.T, topo *ondatra.ATETopology, atePort, peerAddress string, localAsn uint32, networkName, nextHop, prefix string, count uint32, useLoopback bool) {

	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}

	network := intfs[atePort].AddNetwork(networkName)
	bgpAttribute := network.BGP()
	bgpAttribute.WithActive(true).WithNextHopAddress(nextHop)

	// Add prefixes, Add network instance
	if prefix != "" {
		network.IPv4().WithAddress(prefix).WithCount(count)
	}
	// Create BGP instance
	bgp := intfs[atePort].BGP()
	bgpPeer := bgp.AddPeer().WithPeerAddress(peerAddress).WithLocalASN(localAsn).WithTypeExternal()
	bgpPeer.WithOnLoopback(useLoopback)

	// Update BGP Capabilities
	bgpPeer.Capabilities().WithIPv4UnicastEnabled(true).WithIPv6UnicastEnabled(true).WithGracefulRestart(true)
}

// configAteRoutingProtocols configures routing protocol configurations on the ATE
func configAteRoutingProtocols(t *testing.T, top *ondatra.ATETopology) {
	//advertising 100.100.100.100/32 for bgp resolve over IGP prefix
	intfs := top.Interfaces()
	intfs["ateDst"].WithIPv4Loopback("100.100.100.100/32")
	configAteIsisL2(t, top, "ateDst", "B4", "isis_network", 20, minInnerDstPrefixIsis+"/"+v4mask, totalIsisPrefix)
	configAteEbgpPeer(t, top, "ateDst", dutDst.IPv4, 64001, "bgp_recursive", ateDst.IPv4, minInnerDstPrefixBgp+"/"+v4mask, totalBgpPfx, true)
	top.Push(t).StartProtocols(t)
}

// createFlow returns a flow from atePort1 to the dstPfx, expected to arrive at ATE dst interface
func (args *testArgs) createFlow(name string, dstEndPoint []ondatra.Endpoint, opts ...*tgnOptions) *ondatra.Flow {
	srcEndPoint := args.top.Interfaces()[ateSrc.Name]
	var flow *ondatra.Flow
	var header []ondatra.Header

	for _, opt := range opts {
		if opt.mpls {
			hdrMpls := ondatra.NewMPLSHeader()
			header = []ondatra.Header{ondatra.NewEthernetHeader(), hdrMpls}
		}
		if opt.ipv4 {
			var hdrIpv4 *ondatra.IPv4Header
			// explicity set ttl 0 if zero
			if opt.ttl {
				hdrIpv4 = ondatra.NewIPv4Header().WithTTL(0)
			} else {
				hdrIpv4 = ondatra.NewIPv4Header()
			}
			hdrIpv4.WithSrcAddress(dutSrc.IPv4).DstAddressRange().WithMin(dst).WithCount(dstCount).WithStep("0.0.0.1")
			header = []ondatra.Header{ondatra.NewEthernetHeader(), hdrIpv4}
		}
	}
	flow = args.ate.Traffic().NewFlow(name).
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint...).
		WithHeaders(header...)

	if opts[0].fps != 0 {
		flow.WithFrameRateFPS(opts[0].fps)
	} else {
		flow.WithFrameRateFPS(1000)
	}

	if opts[0].fpercent != 0 {
		flow.WithFrameRatePct(opts[0].fpercent)
	} else {
		flow.WithFrameRatePct(100)
	}

	if opts[0].frameSize != 0 {
		flow.WithFrameSize(opts[0].frameSize)
	} else {
		flow.WithFrameSize(300)
	}
	return flow
}

// validateTrafficFlows validates traffic loss on tgn side and DUT incoming and outgoing counters
func (args *testArgs) validateTrafficFlows(t *testing.T, flow *ondatra.Flow, opts ...*tgnOptions) uint64 {
	t.Helper()
	args.ate.Traffic().Start(t, flow)
	// run traffic for 30 seconds, before introducing fault
	time.Sleep(time.Duration(30) * time.Second)

	// Set configs if needed for the trigger scenario
	for _, op := range opts {
		if eventAction, ok := op.event.(*eventInterfaceConfig); ok {
			eventAction.interfaceConfig(t)
		} else if eventAction, ok := op.event.(*eventStaticRouteToNull); ok {
			eventAction.staticRouteToNull(t)
		} else if eventAction, ok := op.event.(*eventEnableMplsLdp); ok {
			eventAction.enableMplsLdp(t)
		} else if eventAction, ok := op.event.(*eventAclConfig); ok {
			eventAction.aclConfig(t)
		} else if eventAction, ok := op.event.(*eventZeroTtl); ok {
			flow = eventAction.modifyTrafficTtl(t, args)
			args.ate.Traffic().Start(t, flow)
		}
	}

	time.Sleep(60 * time.Second) // sleep if any trigger config was performed
	args.ate.Traffic().Stop(t)

	// remove the trigger configs before further check
	for _, op := range opts {
		if _, ok := op.event.(*eventInterfaceConfig); ok {
			eventAction := eventInterfaceConfig{config: false, mtu: 1514, port: sortPorts(args.dut.Ports())[1:]}
			eventAction.interfaceConfig(t)
		} else if _, ok := op.event.(*eventStaticRouteToNull); ok {
			eventAction := eventStaticRouteToNull{prefix: "202.1.0.1/32", config: false}
			eventAction.staticRouteToNull(t)
		} else if _, ok := op.event.(*eventEnableMplsLdp); ok {
			eventAction := eventEnableMplsLdp{config: false}
			eventAction.enableMplsLdp(t)
		} else if _, ok := op.event.(*eventAclConfig); ok {
			eventAction := eventAclConfig{config: false}
			eventAction.aclConfig(t)
		}
		// no cleanup action required for 0 TTL trigger as it is based on ATE flow
	}

	for _, op := range opts {
		if op.drop {
			out := gnmi.Get(t, args.ate, gnmi.OC().Flow(flow.Name()).Counters().OutPkts().State())
			t.Logf("OutPkts = %d", out)
			in := gnmi.Get(t, args.ate, gnmi.OC().Flow(flow.Name()).Counters().InPkts().State())
			t.Logf("InPkts = %d", in)
			return out - in
		}
	}
	return 0
}

type PBROptions struct {
	// BackupNHG specifies the backup next-hop-group to be used when all next-hops are unavailable.
	SrcIP string
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
		IPv6:    "2000::100:122:1:2",
		IPv6Len: ipv6PrefixLen,
	}
)

// configInterfaceDUT configures the interfaces with corresponding addresses
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

func configIsis(t *testing.T, dut *ondatra.DUTDevice, intfNames []string) {
	dev := &oc.Root{}
	inst := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	prot := inst.GetOrCreateProtocol(policyTypeIsis, isisName)
	isis := prot.GetOrCreateIsis()
	glob := isis.GetOrCreateGlobal()
	glob.Net = []string{fmt.Sprintf("%v.%v.00", dutAreaAddress, dutSysId)}
	glob.LevelCapability = 2
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)

	for _, intfName := range intfNames {
		intf := isis.GetOrCreateInterface(intfName)
		intf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
		intf.Enabled = ygot.Bool(true)
		intf.HelloPadding = 1
		intf.Passive = ygot.Bool(false)
		intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
		intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	}
	level := isis.GetOrCreateLevel(2)
	level.MetricStyle = 2

	dutNode := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(policyTypeIsis, isisName)
	dutConf := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance).GetOrCreateProtocol(policyTypeIsis, isisName)
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

	dutNode := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(policyTypeBgp, *ciscoFlags.DefaultNetworkInstance)
	dutConf := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance).GetOrCreateProtocol(policyTypeBgp, *ciscoFlags.DefaultNetworkInstance)
	gnmi.Update(t, dut, dutNode.Config(), dutConf)
}

func configVRF(t *testing.T, dut *ondatra.DUTDevice, vrfs []string) {
	for _, vrfName := range vrfs {
		vrf := &oc.NetworkInstance{
			Name: ygot.String(vrfName),
			Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
		}
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrfName).Config(), vrf)
	}
}

// configBasePBR creates class map, policy and configures them under source interface
func configBasePBR(t *testing.T, dut *ondatra.DUTDevice, networkInstance, ipType string, index uint32, pbrName string, protocol oc.E_PacketMatchTypes_IP_PROTOCOL, dscpSet []uint8, opts ...*PBROptions) {
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	r := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r.SequenceId = ygot.Uint32(index)
	r.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(networkInstance)}
	if ipType == "ipv4" {
		if len(opts) != 0 {
			for _, opt := range opts {
				if opt.SrcIP != "" {
					r.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
						SourceAddress: &opt.SrcIP,
						Protocol:      protocol,
					}
				}
			}
		} else {
			r.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
				Protocol: protocol,
			}
		}
		if len(dscpSet) > 0 {
			r.Ipv4.DscpSet = dscpSet
		}
	} else if ipType == "ipv6" {
		r.Ipv6 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv6{
			Protocol: protocol,
		}
		if len(dscpSet) > 0 {
			r.Ipv6.DscpSet = dscpSet
		}
	}
	pf := oc.NetworkInstance_PolicyForwarding{}
	p := pf.GetOrCreatePolicy(pbrName)
	p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
	err := p.AppendRule(&r)
	if err != nil {
		t.Error(err)
	}

	intf := pf.GetOrCreateInterface("Bundle-Ether121.0")
	intf.GetOrCreateInterfaceRef().Interface = ygot.String("Bundle-Ether121")
	intf.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	intf.ApplyVrfSelectionPolicy = ygot.String(pbrName)
	intf.InterfaceId = ygot.String("Bundle-Ether121.0")
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).PolicyForwarding().Config(), &pf)
}

// TODO - support levels and sub-modes for FEAT-22487
// getData retrieves data from a DUT using GNMI.
// It performs a subscription to the specified path using a wildcard query.
func getData(t *testing.T, path string, query ygnmi.WildcardQuery[uint64]) (uint64, error) {
	t.Helper()
	dut := ondatra.DUT(t, "dut")

	watchOpts := dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_SAMPLE),
		ygnmi.WithSampleInterval(10*time.Second))
	data, pred := gnmi.WatchAll(t, watchOpts, query, 45*time.Second, func(val *ygnmi.Value[uint64]) bool {
		_, present := val.Val()
		stringPath, err := ygot.PathToString(val.Path)
		if err != nil {
			t.Logf("error converting path to string: %v", err)
			return false
		}
		if stringPath == path {
			return present
		}
		return !present
	},
	).Await(t)
	if pred == false {
		return 0, fmt.Errorf("watch failed for path %s. Predicate returned is %v", path, pred)
	}

	counter, ok := data.Val()
	if ok {
		return counter, nil
	} else {
		return 0, fmt.Errorf("failed to collect data for path %s", path)
	}
}
