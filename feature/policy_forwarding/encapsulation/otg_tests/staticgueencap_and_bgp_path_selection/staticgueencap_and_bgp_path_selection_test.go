package staticgueencapbgppathselection_test

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/iputil"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	otgvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_validation_helpers"
	"github.com/openconfig/featureprofiles/internal/otg_helpers/packetvalidationhelpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Constants for address families, AS numbers, and protocol settings.
const (
	plenIPv4p2p   = 31
	plenIPv6p2p   = 127
	plenIPv4lo    = 32
	plenIPv6lo    = 128
	dutAS         = 65501
	ateIBGPAS     = 65501 // For iBGP with DUT
	ateEBGPAS     = 65502 // For eBGP with DUT
	isisInstance  = "DEFAULT"
	isisSysID1    = "640000000001"
	isisSysID2    = "640000000002"
	isisAreaAddr  = "49.0001"
	dutSysID      = "1920.0000.2001"
	ibgpPeerGroup = "IBGP-PEERS"
	ebgpPeerGroup = "EBGP-PEERS"
	udpEncapPort  = 6080
	ttl           = 64

	// Static and GUE address
	nexthopGroupName1 = "GUE_TE10"
	nexthopGroupName2 = "GUE_TE11"
	guePolicyName     = "GUE-Policy"
	decapPolicy1      = "DECAP_TE10"
	decapPolicy2      = "DECAP_TE11"

	totalPackets = 50000
	trafficPps   = 1000
)

type otgBGPConfigData struct {
	port        string
	otgPortData []*attrs.Attributes
	dutPortData []*attrs.Attributes
	otgDevice   []gosnappi.Device
	bgpCfg      []*bgpNbr
}

type trafficFlow struct {
	flows       otgconfighelpers.Flow
	innerParams otgconfighelpers.Flow
}

var (
	otgBGPConfig = []*otgBGPConfigData{
		{
			port: "port1",
			otgPortData: []*attrs.Attributes{
				{
					Name:    "port1",
					IPv4:    "192.0.2.2",
					IPv6:    "2001:db8:1::2",
					MAC:     "02:01:01:01:01:01",
					IPv4Len: plenIPv4p2p,
					IPv6Len: plenIPv6p2p,
				},
			},
			dutPortData: []*attrs.Attributes{
				{
					Desc:    "DUT to ATE Port 1",
					IPv4:    "192.0.2.3",
					IPv6:    "2001:db8:1::3",
					IPv4Len: plenIPv4p2p,
					IPv6Len: plenIPv6p2p,
				},
			},
			bgpCfg: []*bgpNbr{
				{peerAs: ateIBGPAS, nbrIp: "192.0.2.2", isV4: true, peerGrpName: ibgpPeerGroup + "-v4ate1", srcIp: dutloopback0.IPv4, routeReflector: true},
				{peerAs: ateIBGPAS, nbrIp: "2001:db8:1::2", isV4: false, peerGrpName: ibgpPeerGroup + "-v6ate1", srcIp: dutloopback0.IPv6, routeReflector: true},
			},
		},
		{
			port: "port2",
			otgPortData: []*attrs.Attributes{
				{
					Name:         "R100",
					IPv4:         "192.0.2.4",
					IPv6:         "2001:db8:1::4",
					MAC:          "02:02:02:02:02:01",
					IPv4Len:      plenIPv4p2p,
					IPv6Len:      plenIPv6p2p,
					Subinterface: 100,
				},
				{
					Name:         "R200",
					IPv4:         "192.0.2.6",
					IPv6:         "2001:db8:1::6",
					MAC:          "02:02:02:02:02:02",
					IPv4Len:      plenIPv4p2p,
					IPv6Len:      plenIPv6p2p,
					Subinterface: 200,
				},
				{
					Name:         "R300",
					IPv4:         "192.0.2.8",
					IPv6:         "2001:db8:1::8",
					MAC:          "02:02:02:02:02:03",
					IPv4Len:      plenIPv4p2p,
					IPv6Len:      plenIPv6p2p,
					Subinterface: 300,
				},
			},
			dutPortData: []*attrs.Attributes{
				{
					Desc:         "DUT Port 2 Vlan 100",
					IPv4:         "192.0.2.5",
					IPv6:         "2001:db8:1::5",
					IPv4Len:      plenIPv4p2p,
					IPv6Len:      plenIPv6p2p,
					Subinterface: 100,
				},
				{
					Desc:         "DUT Port 2 Vlan 200",
					IPv4:         "192.0.2.7",
					IPv6:         "2001:db8:1::7",
					IPv4Len:      plenIPv4p2p,
					IPv6Len:      plenIPv6p2p,
					Subinterface: 200,
				},
				{
					Desc:         "DUT Port 2 Vlan 300",
					IPv4:         "192.0.2.9",
					IPv6:         "2001:db8:1::9",
					IPv4Len:      plenIPv4p2p,
					IPv6Len:      plenIPv6p2p,
					Subinterface: 300,
				},
			},
			bgpCfg: []*bgpNbr{
				{peerAs: ateIBGPAS, nbrIp: "192.0.2.4", isV4: true, peerGrpName: ibgpPeerGroup + "-v4ate_IBGP", srcIp: dutloopback0.IPv4},
				{peerAs: ateIBGPAS, nbrIp: "2001:db8:1::4", isV4: false, peerGrpName: ibgpPeerGroup + "-v6ate_IBGP", srcIp: dutloopback0.IPv6},
				{peerAs: ateIBGPAS, nbrIp: "192.0.2.6", isV4: true, peerGrpName: ibgpPeerGroup + "-v4ate2-200", srcIp: dutloopback0.IPv4},
				{peerAs: ateIBGPAS, nbrIp: "2001:db8:1::6", isV4: false, peerGrpName: ibgpPeerGroup + "-v6ate_C_IBGP", srcIp: dutloopback0.IPv6},
				{peerAs: ateIBGPAS, nbrIp: "192.0.2.8", isV4: true, peerGrpName: ibgpPeerGroup + "-v4ate_M_IBGP", srcIp: dutloopback0.IPv4},
				{peerAs: ateIBGPAS, nbrIp: "2001:db8:1::8", isV4: false, peerGrpName: ibgpPeerGroup + "-v6ate_M_IBGP", srcIp: dutloopback0.IPv6},
			},
		},
		{
			port: "port3",
			otgPortData: []*attrs.Attributes{
				{
					Name:    "port3",
					IPv4:    "192.0.2.10",
					IPv6:    "2001:db8:1::10",
					MAC:     "02:03:03:03:03:03",
					IPv4Len: plenIPv4p2p,
					IPv6Len: plenIPv6p2p,
				},
			},
			dutPortData: []*attrs.Attributes{
				{
					Desc:    "DUT to ATE Port 3",
					IPv4:    "192.0.2.11",
					IPv6:    "2001:db8:1::11",
					IPv4Len: plenIPv4p2p,
					IPv6Len: plenIPv6p2p,
				},
			},
			bgpCfg: []*bgpNbr{
				{peerAs: ateEBGPAS, nbrIp: "192.0.2.10", isV4: true, peerGrpName: ebgpPeerGroup, srcIp: dutloopback0.IPv4},
				{peerAs: ateEBGPAS, nbrIp: "2001:db8:1::10", isV4: false, peerGrpName: ebgpPeerGroup, srcIp: dutloopback0.IPv6},
			},
		},
	}

	// DUT loopback 0 ($DUT_lo0)
	dutloopback0 = &attrs.Attributes{
		Desc:    "DUT Loopback 0",
		IPv4:    "203.0.113.10",
		IPv6:    "2001:db8::203:0:113:10",
		IPv4Len: plenIPv4lo,
		IPv6Len: plenIPv6lo,
	}

	// ATE Port1 user prefixes
	ate1UserPrefixesV4     = "198.61.100.1"
	ate1UserPrefixesV6     = "2001:db8:100:1::"
	ate1UserPrefixesCount  = uint32(5)
	ate1UserPrefixesV4List = iputil.GenerateIPs(ate1UserPrefixesV4+"/24", int(ate1UserPrefixesCount))
	ate1UserPrefixesV6List = iputil.GenerateIPv6(ate1UserPrefixesV6+"/64", uint64(ate1UserPrefixesCount))

	// $ATE2_INTERNAL - Prefixes to be advertised by ATE Port2 IBGP/ ATE2_C
	ate2InternalPrefixesV4     = "198.71.100.1"
	ate2InternalPrefixesV6     = "2001:db8:200:1::"
	ate2InternalPrefixCount    = uint32(5)
	ate2InternalPrefixesV4List = iputil.GenerateIPs(ate2InternalPrefixesV4+"/24", int(ate2InternalPrefixCount))
	ate2InternalPrefixesV6List = iputil.GenerateIPv6(ate2InternalPrefixesV6+"/64", uint64(ate2InternalPrefixCount))

	// ATE Port3 or ATE2 Port3 bgp prefixes
	bgpInternalTE11 = &attrs.Attributes{
		Name:    "ate2InternalTE11",
		IPv4:    "198.18.11.0",
		IPv4Len: 30,
	}
	bgpInternalTE10 = &attrs.Attributes{
		Name:    "ate2InternalTE10",
		IPv4:    "198.18.10.0",
		IPv4Len: 30,
	}

	// ATE Port2 C.IBGP ---> DUT connected via Pseudo Protocol Next-Hops
	ate2ppnh1 = &attrs.Attributes{Name: "ate2ppnh1", IPv6: "2001:db8:2::0", IPv6Len: plenIPv6lo}
	ate2ppnh2 = &attrs.Attributes{Name: "ate2ppnh2", IPv6: "2001:db8:3::0", IPv6Len: plenIPv6lo}

	ate2ppnh1Prefix = "2001:db8:2::0/128"
	ate2ppnh2Prefix = "2001:db8:3::0/128"

	loopbackIntfName string

	dscpValue = map[string]uint32{
		"BE1": 0,
		"AF1": 10,
		"AF2": 18,
		"AF3": 26,
		"AF4": 34,
	}

	expectedDscpValue = map[string]uint32{
		"BE1": 0,
		"AF1": 40,
		"AF2": 72,
		"AF3": 104,
		"AF4": 136,
	}

	atePort1RouteV4 = "v4-user-routes"
	atePort1RouteV6 = "v6-user-routes"

	atePort2RoutesV4   = "v4-internal-routes"
	atePort2RoutesV6   = "v6-internal-routes"
	atePort2RoutesTE10 = "v4-TE10-routes"
	atePort2RoutesTE11 = "v4-TE11-routes"

	trafficFlowData []trafficFlow

	port1DstMac string
	port2DstMac string

	FlowIPv4Validation = &otgvalidationhelpers.OTGValidation{
		Flow: &otgvalidationhelpers.FlowParams{TolerancePct: 0.5},
	}

	outerGUEIPLayerIPv4 = &packetvalidationhelpers.IPv4Layer{
		Protocol: 17,
		TTL:      ttl,
		DstIP:    bgpInternalTE11.IPv4,
	}

	outerGUEv6Encap = &packetvalidationhelpers.IPv4Layer{
		SkipProtocolCheck: true,
		TTL:               ttl,
		DstIP:             bgpInternalTE10.IPv4,
	}

	innerGUEIPLayerIPv4 = &packetvalidationhelpers.IPv4Layer{
		Protocol:          udpEncapPort,
		SkipProtocolCheck: true,
		TTL:               ttl - 1,
	}

	udpLayer = &packetvalidationhelpers.UDPLayer{
		DstPort: udpEncapPort,
	}

	validations = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateIPv4Header,
		packetvalidationhelpers.ValidateUDPHeader,
		packetvalidationhelpers.ValidateInnerIPv4Header,
	}

	encapValidation = &packetvalidationhelpers.PacketValidation{
		PortName:         "port3",
		Validations:      validations,
		IPv4Layer:        outerGUEIPLayerIPv4,
		UDPLayer:         udpLayer,
		InnerIPLayerIPv4: innerGUEIPLayerIPv4,
	}

	validationsV6 = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateIPv4Header,
		packetvalidationhelpers.ValidateUDPHeader,
		// packetvalidationhelpers.ValidateIPv6Header,
	}

	encapValidationv6 = &packetvalidationhelpers.PacketValidation{
		PortName:    "port3",
		Validations: validationsV6,
		IPv4Layer:   outerGUEv6Encap,
		UDPLayer:    udpLayer,
	}

	decapValidation = &packetvalidationhelpers.PacketValidation{
		PortName:    "port1",
		CaptureName: "decapCapture",
		Validations: []packetvalidationhelpers.ValidationType{packetvalidationhelpers.ValidateIPv4Header},
		IPv4Layer:   innerGUEIPLayerIPv4,
	}
)

type isisConfig struct {
	port  string
	level oc.E_Isis_LevelType
}

type bgpNbr struct {
	peerGrpName    string
	nbrIp          string
	srcIp          string
	peerAs         uint32
	isV4           bool
	routeReflector bool
}

type flowGroupData struct {
	Flows []gosnappi.Flow
}

var flowGroups = make(map[string]flowGroupData)

// configureDUT configures interfaces, BGP, IS-IS, and static tunnel routes on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice, port *ondatra.Port, portAttr *attrs.Attributes) {
	d := gnmi.OC()
	gnmi.Update(t, dut, d.Interface(port.Name()).Config(), configInterfaceDUT(t, port, new(oc.Root), portAttr, dut))

	// Configure Network instance type on DUT
	t.Log("Configure/update Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, port.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, port)
	}
}

func configureStaticRoute(t *testing.T, dut *ondatra.DUTDevice) {
	b := &gnmi.SetBatch{}

	// Configuring Static Route: PNH-IPv6 --> IPv4 GUE tunnel.
	sV4 := &cfgplugins.StaticRouteCfg{
		NetworkInstance:  deviations.DefaultNetworkInstance(dut),
		Prefix:           ate2ppnh1Prefix,
		NexthopGroup:     true,
		NexthopGroupName: nexthopGroupName2,
		T:                t,
		TrafficType:      oc.Aft_EncapsulationHeaderType_UDPV4,
		PolicyName:       guePolicyName,
		Rule:             "rule1",
	}

	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
		t.Fatalf("Failed to configure IPv6 static route: %v", err)
	}
	b.Set(t, dut)

	sV4 = &cfgplugins.StaticRouteCfg{
		NetworkInstance:  deviations.DefaultNetworkInstance(dut),
		Prefix:           ate2ppnh2Prefix,
		NexthopGroup:     true,
		NexthopGroupName: nexthopGroupName1,
		T:                t,
		TrafficType:      oc.Aft_EncapsulationHeaderType_UDPV6,
		PolicyName:       guePolicyName,
		Rule:             "rule2",
	}

	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
		t.Fatalf("Failed to configure IPv6 static route: %v", err)
	}
	b.Set(t, dut)
}

// Configures the given DUT interface.
func configInterfaceDUT(t *testing.T, p *ondatra.Port, d *oc.Root, a *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {
	t.Helper()

	i := d.GetOrCreateInterface(p.Name())
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	// Always create subif 0
	subif := i.GetOrCreateSubinterface(0)
	subif.Index = ygot.Uint32(0)
	iv4 := subif.GetOrCreateIpv4()
	iv6 := subif.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		iv4.Enabled = ygot.Bool(true)
		iv6.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(a.Subinterface)

	if a.Subinterface != 0 {
		if deviations.DeprecatedVlanID(dut) {
			s.GetOrCreateVlan().VlanId = oc.UnionUint16(a.Subinterface)
		} else {
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(uint16(a.Subinterface))
		}
	}
	s4 := s.GetOrCreateIpv4()
	a4 := s4.GetOrCreateAddress(a.IPv4)
	a4.PrefixLength = ygot.Uint8(uint8(a.IPv4Len))
	if deviations.InterfaceEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	s6 := s.GetOrCreateIpv6()
	a6 := s6.GetOrCreateAddress(a.IPv6)
	a6.PrefixLength = ygot.Uint8(uint8(a.IPv6Len))
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}

	return i
}

func configureLoopback(t *testing.T, dut *ondatra.DUTDevice) {
	// Configure interface loopback
	loopbackIntfName = netutil.LoopbackInterface(t, dut, 0)
	lo0 := gnmi.OC().Interface(loopbackIntfName).Subinterface(0)
	ipv4Addrs := gnmi.LookupAll(t, dut, lo0.Ipv4().AddressAny().State())
	ipv6Addrs := gnmi.LookupAll(t, dut, lo0.Ipv6().AddressAny().State())
	if len(ipv4Addrs) == 0 && len(ipv6Addrs) == 0 {
		loop1 := dutloopback0.NewOCInterface(loopbackIntfName, dut)
		loop1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
		gnmi.Update(t, dut, gnmi.OC().Interface(loopbackIntfName).Config(), loop1)
	} else {
		v4, ok := ipv4Addrs[0].Val()
		if ok {
			dutloopback0.IPv4 = v4.GetIp()
		}
		v6, ok := ipv6Addrs[0].Val()
		if ok {
			dutloopback0.IPv6 = v6.GetIp()
		}
		t.Logf("Got DUT IPv4 loopback address: %v", dutloopback0.IPv4)
		t.Logf("Got DUT IPv6 loopback address: %v", dutloopback0.IPv6)
	}
}

func configureISIS(t *testing.T, dut *ondatra.DUTDevice) {
	isisConf := []*isisConfig{
		{port: dut.Port(t, otgBGPConfig[0].port).Name(), level: oc.Isis_LevelType_LEVEL_2},
		{port: dut.Port(t, otgBGPConfig[1].port).Name(), level: oc.Isis_LevelType_LEVEL_2},
	}

	// Configure IS-IS protocol on port1 and port2
	root := &oc.Root{}
	dutConfIsisPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	ni := root.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	isisProtocol := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)

	isisProtocol.SetEnabled(true)
	isis := isisProtocol.GetOrCreateIsis()

	globalISIS := isis.GetOrCreateGlobal()
	if deviations.ISISInstanceEnabledRequired(dut) {
		globalISIS.SetInstance(isisInstance)
	}

	// Configure Global ISIS settings
	globalISIS.SetNet([]string{fmt.Sprintf("%s.%s.00", isisAreaAddr, dutSysID)})
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).SetEnabled(true)
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).SetEnabled(true)
	level := isis.GetOrCreateLevel(2)
	level.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC
	// Configure ISIS enabled flag at level
	if deviations.ISISLevelEnabled(dut) {
		level.SetEnabled(true)
	}

	for _, isisPort := range isisConf {
		intf := isis.GetOrCreateInterface(isisPort.port)
		intf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
		intf.SetEnabled(true)
		if deviations.ISISInterfaceLevel1DisableRequired(dut) {
			intf.GetOrCreateLevel(1).SetEnabled(false)
		} else {
			intf.GetOrCreateLevel(2).SetEnabled(true)
		}
		globalISIS.LevelCapability = isisPort.level
		intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).SetEnabled(true)
		intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).SetEnabled(true)
		if deviations.ISISInterfaceAfiUnsupported(dut) {
			intf.Af = nil
		}
	}

	// Push ISIS configuration to DUT
	gnmi.Replace(t, dut, dutConfIsisPath.Config(), isisProtocol)

}

func bgpCreateNbr(t *testing.T, localAs uint32, dut *ondatra.DUTDevice, nbrs []*bgpNbr) {
	localAddressLeaf := ""
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	dutOcRoot := &oc.Root{}
	ni1 := dutOcRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.SetRouterId(dutloopback0.IPv4)
	global.SetAs(localAs)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).SetEnabled(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).SetEnabled(true)

	for _, nbr := range nbrs {
		pg1 := bgp.GetOrCreatePeerGroup(nbr.peerGrpName)
		pg1.SetPeerAs(nbr.peerAs)

		bgpNbr := bgp.GetOrCreateNeighbor(nbr.nbrIp)
		bgpNbr.SetPeerGroup(nbr.peerGrpName)
		bgpNbr.SetPeerAs(nbr.peerAs)
		bgpNbr.SetEnabled(true)
		bgpNbrT := bgpNbr.GetOrCreateTransport()

		localAddressLeaf = nbr.srcIp

		if dut.Vendor() == ondatra.CISCO {
			localAddressLeaf = dutloopback0.Name
		}
		bgpNbrT.SetLocalAddress(localAddressLeaf)
		if nbr.routeReflector {
			routeReflector := bgpNbr.GetOrCreateRouteReflector()
			routeReflector.SetRouteReflectorClient(true)
		}
		af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		af4.SetEnabled(true)
		af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		af6.SetEnabled(true)

	}

	gnmi.Update(t, dut, dutConfPath.Config(), niProto)
}

func configureOTG() {
	// Enable ISIS and BGP Protocols on port 1.
	port1Data := otgBGPConfig[0]
	iDut1Dev := port1Data.otgDevice[0]

	isisDut := iDut1Dev.Isis().SetName("ISIS1").SetSystemId(isisSysID1)
	isisDut.Basic().SetIpv4TeRouterId(port1Data.otgPortData[0].IPv4).SetHostname(isisDut.Name()).SetLearnedLspFilter(true)
	isisDut.Interfaces().Add().SetEthName(iDut1Dev.Ethernets().Items()[0].Name()).
		SetName("devIsisInt1").
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT)

	iDutBgp := iDut1Dev.Bgp().SetRouterId(port1Data.otgPortData[0].IPv4)
	iDutBgp4Peer := iDutBgp.Ipv4Interfaces().Add().SetIpv4Name(iDut1Dev.Ethernets().Items()[0].Ipv4Addresses().Items()[0].Name()).
		Peers().Add().SetName(port1Data.otgPortData[0].Name + ".BGP4.peer")
	iDutBgp4Peer.SetPeerAddress(dutloopback0.IPv4).SetAsNumber(ateIBGPAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	iDutBgp4Peer.Capability().SetIpv4Unicast(true).SetIpv6Unicast(true)
	iDutBgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	// Advertise user prefixes on port 1
	v4routes := iDutBgp4Peer.V4Routes().Add().SetName(atePort1RouteV4)
	v4routes.Addresses().Add().SetAddress(ate1UserPrefixesV4).SetStep(1).SetPrefix(24).SetCount(ate1UserPrefixesCount)

	iDutBgp6Peer := iDutBgp.Ipv6Interfaces().Add().SetIpv6Name(iDut1Dev.Ethernets().Items()[0].Ipv6Addresses().Items()[0].Name()).
		Peers().Add().SetName(port1Data.otgPortData[0].Name + ".BGP6.peer")
	iDutBgp6Peer.SetPeerAddress(dutloopback0.IPv6).SetAsNumber(ateIBGPAS).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
	iDutBgp6Peer.Capability().SetIpv4Unicast(true).SetIpv6Unicast(true)
	iDutBgp6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
	// Advertise user prefixes v6 on port 1
	v6routes := iDutBgp6Peer.V6Routes().Add().SetName(atePort1RouteV6)
	v6routes.Addresses().Add().SetAddress(ate1UserPrefixesV6).SetStep(1).SetPrefix(64).SetCount(ate1UserPrefixesCount)

	// Configure OTG Port2
	port2Data := otgBGPConfig[1]

	// Enable ISIS and BGP Protocols on port 2 VLAN 100
	iDut2Dev := port2Data.otgDevice[0]

	isis2Dut := iDut2Dev.Isis().SetName("ISIS2").SetSystemId(isisSysID2)
	isis2Dut.Basic().SetIpv4TeRouterId(port2Data.otgPortData[0].IPv4).SetHostname(isis2Dut.Name()).SetLearnedLspFilter(true)
	isis2Dut.Interfaces().Add().SetEthName(iDut2Dev.Ethernets().Items()[0].Name()).
		SetName("devIsisInt2").
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT)

	// Configure IBGP Peer on port2 VLAN100
	iDut2Bgp := iDut2Dev.Bgp().SetRouterId(port2Data.otgPortData[0].IPv4)
	iDut2Bgp4Peer := iDut2Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut2Dev.Ethernets().Items()[0].Ipv4Addresses().Items()[0].Name()).Peers().Add().SetName(port2Data.otgPortData[0].Name + ".BGP4.peer")
	iDut2Bgp4Peer.SetPeerAddress(dutloopback0.IPv4).SetAsNumber(ateIBGPAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	iDut2Bgp4Peer.Capability().SetIpv4Unicast(true).SetIpv6Unicast(true)
	iDut2Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	// Advertise prefixes from IBGP Peer
	iDut2Bgpv4routes := iDut2Bgp4Peer.V4Routes().Add().SetName(atePort2RoutesV4)
	iDut2Bgpv4routes.Addresses().Add().SetAddress(ate2InternalPrefixesV4).SetStep(1).SetPrefix(24).SetCount(ate2InternalPrefixCount)

	iDut2BgpTe10Routes := iDut2Bgp4Peer.V4Routes().Add().SetName(atePort2RoutesTE10)
	iDut2BgpTe10Routes.Addresses().Add().SetAddress(bgpInternalTE10.IPv4).SetPrefix(uint32(bgpInternalTE10.IPv4Len)).SetCount(1)

	iDut2BgpTe11Routes := iDut2Bgp4Peer.V4Routes().Add().SetName(atePort2RoutesTE11)
	iDut2BgpTe11Routes.Addresses().Add().SetAddress(bgpInternalTE11.IPv4).SetPrefix(uint32(bgpInternalTE11.IPv4Len)).SetCount(1)

	// iDut2Bgpv6 := iDut2Dev.Bgp().SetRouterId(port2Data.otgPortData[0].IPv6)
	iDut2Bgp6Peer := iDut2Bgp.Ipv6Interfaces().Add().SetIpv6Name(iDut2Dev.Ethernets().Items()[0].Ipv6Addresses().Items()[0].Name()).Peers().Add().SetName(port2Data.otgPortData[0].Name + ".BGP6.peer")
	iDut2Bgp6Peer.SetPeerAddress(dutloopback0.IPv6).SetAsNumber(ateIBGPAS).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
	iDut2Bgp6Peer.Capability().SetIpv4Unicast(true).SetIpv6Unicast(true)
	iDut2Bgp6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	iDut2Bgpv6routes := iDut2Bgp6Peer.V6Routes().Add().SetName(atePort2RoutesV6)
	iDut2Bgpv6routes.Addresses().Add().SetAddress(ate2InternalPrefixesV6).SetStep(1).SetPrefix(64).SetCount(ate1UserPrefixesCount)

	// Configure IBGP_C on port 2 VLAN 200
	iDut2Dev200 := port2Data.otgDevice[1]
	iDut2Bgp200 := iDut2Dev200.Bgp().SetRouterId(port2Data.otgPortData[1].IPv4)

	ate2CBgpv4Peer := iDut2Bgp200.Ipv4Interfaces().Add().SetIpv4Name(iDut2Dev200.Ethernets().Items()[0].Ipv4Addresses().Items()[0].Name()).Peers().Add().SetName(port2Data.otgPortData[1].Name + ".CBGP4.peer")
	ate2CBgpv4Peer.SetPeerAddress(dutloopback0.IPv4).SetAsNumber(ateIBGPAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	ate2CBgpv4Peer.Capability().SetIpv4Unicast(true).SetIpv6Unicast(true)
	ate2CBgpv4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	ate2CBgpv6Peer := iDut2Bgp200.Ipv6Interfaces().Add().SetIpv6Name(iDut2Dev200.Ethernets().Items()[0].Ipv6Addresses().Items()[0].Name()).Peers().Add().SetName(port2Data.otgPortData[1].Name + ".CBGP6.peer")
	ate2CBgpv6Peer.SetPeerAddress(dutloopback0.IPv6).SetAsNumber(ateIBGPAS).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
	ate2CBgpv6Peer.Capability().SetIpv4Unicast(true).SetIpv6Unicast(true)
	ate2CBgpv6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	// Configure IBGP_M on port 2 VLAN 300
	iDut2Dev300 := port2Data.otgDevice[2]
	iDut2Bgp300 := iDut2Dev300.Bgp().SetRouterId(port2Data.otgPortData[2].IPv4)

	ate2MBgpv4Peer := iDut2Bgp300.Ipv4Interfaces().Add().SetIpv4Name(iDut2Dev300.Ethernets().Items()[0].Ipv4Addresses().Items()[0].Name()).Peers().Add().SetName(port2Data.otgPortData[2].Name + ".MBGP4.peer")
	ate2MBgpv4Peer.SetPeerAddress(dutloopback0.IPv4).SetAsNumber(ateIBGPAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	ate2MBgpv4Peer.Capability().SetIpv4Unicast(true).SetIpv6Unicast(true)
	ate2MBgpv4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	ate2MBgpv6Peer := iDut2Bgp300.Ipv6Interfaces().Add().SetIpv6Name(iDut2Dev300.Ethernets().Items()[0].Ipv6Addresses().Items()[0].Name()).Peers().Add().SetName(port2Data.otgPortData[2].Name + ".MBGP6.peer")
	ate2MBgpv6Peer.SetPeerAddress(dutloopback0.IPv6).SetAsNumber(ateIBGPAS).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
	ate2MBgpv6Peer.Capability().SetIpv4Unicast(true).SetIpv6Unicast(true)
	ate2MBgpv6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	// Configure OTG Port3
	port3Data := otgBGPConfig[2]
	iDut3Dev := port3Data.otgDevice[0]

	ate3Bgp := iDut3Dev.Bgp().SetRouterId(port3Data.otgPortData[0].IPv4)

	ate3Bgpv4Peer := ate3Bgp.Ipv4Interfaces().Add().SetIpv4Name(port3Data.otgPortData[0].Name + ".IPv4").Peers().Add().SetName("ate3.bgp4.peer")
	ate3Bgpv4Peer.SetPeerAddress(dutloopback0.IPv4).SetAsNumber(ateEBGPAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP).LearnedInformationFilter().SetUnicastIpv4Prefix(true)

	ate3Bgpv6Peer := ate3Bgp.Ipv6Interfaces().Add().SetIpv6Name(port3Data.otgPortData[0].Name + ".IPv6").Peers().Add().SetName("ate3.bgp6.peer")
	ate3Bgpv6Peer.SetPeerAddress(dutloopback0.IPv6).SetAsNumber(ateEBGPAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP).LearnedInformationFilter().SetUnicastIpv4Prefix(true)

	ebgpRoutes := ate3Bgpv4Peer.V4Routes().Add().SetName("ebgp4-te10-routes")
	ebgpRoutes.Addresses().Add().SetAddress(bgpInternalTE10.IPv4).SetPrefix(uint32(32))

	ebgpRoutes11 := ate3Bgpv4Peer.V4Routes().Add().SetName("ebgp4-te11-routes")
	ebgpRoutes11.Addresses().Add().SetAddress(bgpInternalTE11.IPv4).SetPrefix(uint32(32))
}

func advertiseRoutesWithiBGP(prefixes []string, nexthopIp string, ipv4 bool, peerName string) {
	port2Data := otgBGPConfig[1]
	iDut2Dev := port2Data.otgDevice[1]

	if ipv4 {
		bgpPeer := iDut2Dev.Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0]
		for _, addr := range prefixes {
			v4routes2a := bgpPeer.V4Routes().Add().SetName(peerName)
			v4routes2a.Addresses().Add().SetAddress(addr).SetPrefix(24).SetCount(1)
			v4routes2a.SetNextHopIpv6Address(nexthopIp).SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV6).SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL).AddPath().SetPathId(1)
			v4routes2a.Advanced().SetIncludeLocalPreference(true).SetLocalPreference(200)
		}
	} else {
		bgpPeer := iDut2Dev.Bgp().Ipv6Interfaces().Items()[0].Peers().Items()[0]
		for _, addr := range prefixes {
			v6routes2a := bgpPeer.V6Routes().Add().SetName(peerName)
			v6routes2a.SetNextHopIpv6Address(nexthopIp).SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL).AddPath().SetPathId(1)
			v6routes2a.Advanced().SetIncludeLocalPreference(true).SetLocalPreference(200)
			v6routes2a.Addresses().Add().SetAddress(addr).SetPrefix(64).SetCount(1)
		}
	}
}

func configureInterfaces(otgConfig gosnappi.Config, portObj gosnappi.Port, portAttr *attrs.Attributes, dutAttr *attrs.Attributes) gosnappi.Device {
	iDutDev := otgConfig.Devices().Add().SetName(portAttr.Name)
	iDutEth := iDutDev.Ethernets().Add().SetName(portAttr.Name + ".Eth").SetMac(portAttr.MAC)
	iDutEth.Connection().SetPortName(portObj.Name())

	if portAttr.Subinterface != 0 {
		iDutEth.Vlans().Add().SetName(portAttr.Name + ".Eth" + ".VLAN").SetId(portAttr.Subinterface)
	}

	iDutIpv4 := iDutEth.Ipv4Addresses().Add().SetName(portAttr.Name + ".IPv4")
	iDutIpv4.SetAddress(portAttr.IPv4).SetGateway(dutAttr.IPv4).SetPrefix(uint32(portAttr.IPv4Len))
	iDutIpv6 := iDutEth.Ipv6Addresses().Add().SetName(portAttr.Name + ".IPv6")
	iDutIpv6.SetAddress(portAttr.IPv6).SetGateway(dutAttr.IPv6).SetPrefix(uint32(portAttr.IPv6Len))

	return iDutDev
}

func configureTrafficFlows(t *testing.T, otgConfig gosnappi.Config, trafficFlowData []trafficFlow) {
	flowSetNum := regexp.MustCompile(`^flowSet(\d+)`)

	for _, trafficFlow := range trafficFlowData {
		flow := createflow(otgConfig, &trafficFlow.flows, false, &trafficFlow.innerParams)
		matches := flowSetNum.FindStringSubmatch(trafficFlow.flows.FlowName)
		if len(matches) == 0 {
			t.Fatalf("flow name %s does not match expected pattern", trafficFlow.flows.FlowName)
		}
		flowSet := matches[0]
		fg := flowGroups[flowSet]

		fg.Flows = append(fg.Flows, flow)
		flowGroups[flowSet] = fg
	}
}

func createflow(top gosnappi.Config, params *otgconfighelpers.Flow, clearFlows bool, paramsInner *otgconfighelpers.Flow) gosnappi.Flow {
	if clearFlows {
		top.Flows().Clear()
	}

	params.CreateFlow(top)

	params.AddEthHeader()

	if params.VLANFlow != nil {
		params.AddVLANHeader()
	}

	if params.IPv4Flow != nil {
		params.AddIPv4Header()
	}

	if params.IPv6Flow != nil {
		params.AddIPv6Header()
	}

	if params.UDPFlow != nil {
		params.AddUDPHeader()
	}

	if paramsInner != nil {
		if paramsInner.IPv4Flow != nil {
			params.IPv4Flow = paramsInner.IPv4Flow
			params.AddIPv4Header()
		}

		if paramsInner.IPv6Flow != nil {
			params.IPv6Flow = paramsInner.IPv6Flow
			params.AddIPv6Header()
		}
	}

	return params.GetFlow()
}

func withdrawBGPRoutes(t *testing.T, routeNames []string) {
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Protocol().Route().SetNames(routeNames).SetState(gosnappi.StateProtocolRouteState.WITHDRAW)
	otg.SetControlState(t, cs)

}

func verifyTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, flowName string) {
	FlowIPv4Validation.Flow.Name = flowName
	if err := FlowIPv4Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("validation on flows failed (): %q", err)
	}
}

func validatePrefixes(t *testing.T, dut *ondatra.DUTDevice, neighborIP string, isV4 bool, PfxRcd, PfxSent uint32) {
	t.Helper()
	var afiSafi oc.E_BgpTypes_AFI_SAFI_TYPE

	t.Logf("Validate prefixes for %s. Expecting prefix received %v", neighborIP, PfxRcd)
	switch isV4 {
	case true:
		afiSafi = oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST
	case false:
		afiSafi = oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST
	}

	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	query := bgpPath.Neighbor(neighborIP).AfiSafi(afiSafi).Prefixes().Received().State()
	_, ok := gnmi.Watch(t, dut, query, 30*time.Second, func(val *ygnmi.Value[uint32]) bool {
		if v, ok := val.Val(); ok {
			if v != uint32(PfxRcd) {
				t.Errorf("received prefixes - got: %v, want: %v", v, PfxRcd)
			}
		}
		return true
	}).Await(t)
	if !ok {
		t.Errorf("no received prefixes found")
	}

	sentQuery := bgpPath.Neighbor(neighborIP).AfiSafi(afiSafi).Prefixes().Sent().State()
	_, ok = gnmi.Watch(t, dut, sentQuery, 30*time.Second, func(val *ygnmi.Value[uint32]) bool {
		if v, ok := val.Val(); ok {
			if v != uint32(PfxSent) {
				t.Errorf("sent prefixes - got: %v, want: %v", v, PfxRcd)
			}
		}
		return true
	}).Await(t)
	if !ok {
		t.Errorf("no sent prefixes found")
	}

}

func validateOutCounters(t *testing.T, dut *ondatra.DUTDevice, otg *otg.OTG) {
	var totalTxFromATE uint64

	flows := otg.FetchConfig(t).Flows().Items()
	for _, flow := range flows {
		txPkts := gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State())

		totalTxFromATE += txPkts
	}

	dutOutCounters := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, otgBGPConfig[2].port).Name()).Counters().State()).GetOutUnicastPkts()

	expectedTotalTraffic := uint64(totalPackets * len(flows))
	if totalTxFromATE > 0 {
		if float64(dutOutCounters) < float64(totalTxFromATE)*0.98 {
			t.Errorf("dut counters is significantly less than ATE Tx (%d). Recieved: %d, Expected approx %d", totalTxFromATE, dutOutCounters, expectedTotalTraffic)
		}
	} else if expectedTotalTraffic > 0 {
		t.Errorf("no traffic was reported as transmitted by ATE flows, but %d total packets were expected", expectedTotalTraffic)
	}
}

// configureGueTunnel configures a GUE tunnel with optional ToS and TTL.
func configureGueEncap(t *testing.T, dut *ondatra.DUTDevice) {

	_, ni, _ := cfgplugins.SetupPolicyForwardingInfraOC(deviations.DefaultNetworkInstance(dut))

	v4NexthopUDPParams := cfgplugins.NexthopGroupUDPParams{
		TrafficType:     oc.Aft_EncapsulationHeaderType_UDPV4,
		NexthopGrpName:  nexthopGroupName1,
		Index:           "0",
		SrcIp:           loopbackIntfName,
		DstIp:           []string{bgpInternalTE10.IPv4},
		TTL:             64,
		DstUdpPort:      udpEncapPort,
		NetworkInstance: ni,
		DeleteTtl:       false,
	}
	// Create nexthop group for v4
	cfgplugins.NextHopGroupConfigForIpOverUdp(t, dut, v4NexthopUDPParams)

	v4NexthopUDPParams2 := cfgplugins.NexthopGroupUDPParams{
		TrafficType:     oc.Aft_EncapsulationHeaderType_UDPV4,
		NexthopGrpName:  nexthopGroupName2,
		Index:           "1",
		SrcIp:           loopbackIntfName,
		DstIp:           []string{bgpInternalTE11.IPv4},
		TTL:             64,
		DstUdpPort:      udpEncapPort,
		NetworkInstance: ni,
		DeleteTtl:       false,
	}
	// Create nexthop group for v4
	cfgplugins.NextHopGroupConfigForIpOverUdp(t, dut, v4NexthopUDPParams2)

	v6NexthopUDPParams := cfgplugins.NexthopGroupUDPParams{
		TrafficType:     oc.Aft_EncapsulationHeaderType_UDPV6,
		DstUdpPort:      udpEncapPort,
		NetworkInstance: ni,
	}
	// Create nexthop group for v4
	cfgplugins.NextHopGroupConfigForIpOverUdp(t, dut, v6NexthopUDPParams)

	// Apply traffic policy on interface
	if deviations.NextHopGroupOCUnsupported(dut) {
		interfacePolicyParams := cfgplugins.OcPolicyForwardingParams{
			InterfaceID:       dut.Port(t, "port1").Name(),
			AppliedPolicyName: guePolicyName,
		}
		cfgplugins.InterfacePolicyForwardingApply(t, dut, dut.Port(t, "port1").Name(), guePolicyName, ni, interfacePolicyParams)
	}
}

func sendTrafficCapture(t *testing.T, ate *ondatra.ATEDevice, otgConfig gosnappi.Config, flowNames []string) {
	cs := gosnappi.NewControlState()
	if flowNames[0] == "all" {
		ate.OTG().StartTraffic(t)
	} else {
		cs.Traffic().FlowTransmit().
			SetState(gosnappi.StateTrafficFlowTransmitState.START).
			SetFlowNames(flowNames)
		ate.OTG().SetControlState(t, cs)
	}
	cs = packetvalidationhelpers.StartCapture(t, ate)
	time.Sleep(60 * time.Second)
	ate.OTG().StopTraffic(t)
	time.Sleep(60 * time.Second)
	packetvalidationhelpers.StopCapture(t, ate, cs)
}

func validatePacket(t *testing.T, ate *ondatra.ATEDevice, validationPacket *packetvalidationhelpers.PacketValidation) error {
	err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, validationPacket)
	return err
}

func validateAFTCounters(t *testing.T, dut *ondatra.DUTDevice, isV4 bool, routeIp string) {
	t.Logf("Validate AFT parameters for %s", routeIp)
	if isV4 {
		ipv4Path := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().Ipv4Entry(routeIp)
		if _, ok := gnmi.Watch(t, dut, ipv4Path.State(), time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
			ipv4Entry, present := val.Val()
			return present && ipv4Entry.GetPrefix() == routeIp
		}).Await(t); ok {
			t.Error("ipv4-entry/state/prefix got but should not be present")
		}
	} else {
		ipv6Path := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().Ipv6Entry(routeIp)
		if _, ok := gnmi.Watch(t, dut, ipv6Path.State(), time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool {
			ipv6Entry, present := val.Val()
			return present && ipv6Entry.GetPrefix() == routeIp
		}).Await(t); ok {
			t.Error("ipv6-entry/state/prefix got but should not be present")
		}
	}
}

func testTrafficMigration(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config, flowNames map[string][]string, dscpVal string) {
	t.Log("Validate IPv4 GUE encapsulation and decapsulation")
	sendTrafficCapture(t, ate, otgConfig, flowNames["v4"])
	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
	otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)

	for _, flow := range flowNames["v4"] {
		verifyTrafficFlow(t, ate, flow)
	}

	gueLayer := *outerGUEIPLayerIPv4
	gueLayer.Tos = uint8(expectedDscpValue[dscpVal])

	gueInnerLayer := *innerGUEIPLayerIPv4
	gueInnerLayer.Tos = uint8(expectedDscpValue[dscpVal])

	encapValidation.IPv4Layer = &gueLayer
	encapValidation.InnerIPLayerIPv4 = &gueInnerLayer

	if err := validatePacket(t, ate, encapValidation); err != nil {
		t.Errorf("capture and validatePackets failed (): %q", err)
	}

	t.Log("Validate GUE Decapsulation")
	decapInner := *innerGUEIPLayerIPv4
	decapInner.SkipProtocolCheck = true
	decapInner.Tos = uint8(expectedDscpValue[dscpVal])
	decapValidation.IPv4Layer = &decapInner

	if err := validatePacket(t, ate, decapValidation); err != nil {
		t.Errorf("capture and validatePackets failed (): %q", err)
	}

	t.Log("Validate IPv6 GUE encapsulation")
	sendTrafficCapture(t, ate, otgConfig, flowNames["v6"])
	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)

	for _, flow := range flowNames["v6"] {
		verifyTrafficFlow(t, ate, flow)
	}

	encapValidationv6.IPv4Layer.Tos = uint8(dscpValue[dscpVal])
	if err := validatePacket(t, ate, encapValidationv6); err != nil {
		t.Errorf("capture and validatePackets failed (): %q", err)
	}

	// Validate the counters received on ATE and DUT are same
	validateOutCounters(t, dut, ate.OTG())
}

func TestStaticGue(t *testing.T) {
	var deviceObj gosnappi.Device

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otgConfig := gosnappi.NewConfig()

	for _, cfg := range otgBGPConfig {
		dutPort := dut.Port(t, cfg.port)
		portObj := otgConfig.Ports().Add().SetName(cfg.port)

		// Configure ATE & DUT interfaces
		for index, ap := range cfg.otgPortData {
			configureDUT(t, dut, dutPort, cfg.dutPortData[index])
			deviceObj = configureInterfaces(otgConfig, portObj, ap, cfg.dutPortData[index])
			cfg.otgDevice = append(cfg.otgDevice, deviceObj)
		}

		// Configure BGP Peers on DUT
		bgpCreateNbr(t, dutAS, dut, cfg.bgpCfg)
	}

	configureLoopback(t, dut)
	configureStaticRoute(t, dut)
	configureISIS(t, dut)
	configureGueEncap(t, dut)

	// Configure gue decap config
	ocPFParams := cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: "DEFAULT",
		AppliedPolicyName:   decapPolicy1,
		TunnelIP:            fmt.Sprintf("%s/32", bgpInternalTE10.IPv4),
		GUEPort:             udpEncapPort,
		IPType:              "ip",
		Dynamic:             true,
	}
	_, _, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
	cfgplugins.DecapGroupConfigGue(t, dut, pf, ocPFParams)

	ocPFParams = cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: "DEFAULT",
		AppliedPolicyName:   decapPolicy2,
		TunnelIP:            fmt.Sprintf("%s/32", bgpInternalTE11.IPv4),
		GUEPort:             udpEncapPort,
		IPType:              "ip",
		Dynamic:             true,
	}
	cfgplugins.DecapGroupConfigGue(t, dut, pf, ocPFParams)

	port1DstMac = gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())
	port2DstMac = gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port2").Name()).Ethernet().MacAddress().State())

	// configure interfaces on OTG
	configureOTG()

	packetvalidationhelpers.ConfigurePacketCapture(t, otgConfig, encapValidation)
	packetvalidationhelpers.ConfigurePacketCapture(t, otgConfig, decapValidation)

	trafficFlowData = []trafficFlow{
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[0].port,
				RxPorts:       []string{otgBGPConfig[2].port, otgBGPConfig[1].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet1-v4-1",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[0].otgPortData[0].MAC, DstMAC: port1DstMac},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: ate1UserPrefixesV4List[0], IPv4Dst: ate2InternalPrefixesV4List[0], DSCP: dscpValue["BE1"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[0].port,
				RxPorts:       []string{otgBGPConfig[2].port, otgBGPConfig[1].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet1-v6-1",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[0].otgPortData[0].MAC, DstMAC: port1DstMac},
				IPv6Flow:      &otgconfighelpers.IPv6FlowParams{IPv6Src: ate1UserPrefixesV6List[0], IPv6Dst: ate2InternalPrefixesV6List[0], TrafficClass: dscpValue["BE1"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[0].port,
				RxPorts:       []string{otgBGPConfig[2].port, otgBGPConfig[1].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet1-v4-2",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[0].otgPortData[0].MAC, DstMAC: port1DstMac},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: ate1UserPrefixesV4List[1], IPv4Dst: ate2InternalPrefixesV4List[1], DSCP: dscpValue["AF1"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[0].port,
				RxPorts:       []string{otgBGPConfig[2].port, otgBGPConfig[1].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet1-v6-2",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[0].otgPortData[0].MAC, DstMAC: port1DstMac},
				IPv6Flow:      &otgconfighelpers.IPv6FlowParams{IPv6Src: ate1UserPrefixesV6List[1], IPv6Dst: ate2InternalPrefixesV6List[1], TrafficClass: dscpValue["AF1"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[0].port,
				RxPorts:       []string{otgBGPConfig[2].port, otgBGPConfig[1].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet1-v4-3",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[0].otgPortData[0].MAC, DstMAC: port1DstMac},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: ate1UserPrefixesV4List[2], IPv4Dst: ate2InternalPrefixesV4List[2], DSCP: dscpValue["AF2"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[0].port,
				RxPorts:       []string{otgBGPConfig[2].port, otgBGPConfig[1].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet1-v6-3",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[0].otgPortData[0].MAC, DstMAC: port1DstMac},
				IPv6Flow:      &otgconfighelpers.IPv6FlowParams{IPv6Src: ate1UserPrefixesV6List[2], IPv6Dst: ate2InternalPrefixesV6List[2], TrafficClass: dscpValue["AF2"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[0].port,
				RxPorts:       []string{otgBGPConfig[2].port, otgBGPConfig[1].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet2-v4-1",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[0].otgPortData[0].MAC, DstMAC: port1DstMac},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: ate1UserPrefixesV4List[3], IPv4Dst: ate2InternalPrefixesV4List[3], DSCP: dscpValue["AF3"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[0].port,
				RxPorts:       []string{otgBGPConfig[2].port, otgBGPConfig[1].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet2-v6-1",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[0].otgPortData[0].MAC, DstMAC: port1DstMac},
				IPv6Flow:      &otgconfighelpers.IPv6FlowParams{IPv6Src: ate1UserPrefixesV6List[3], IPv6Dst: ate2InternalPrefixesV6List[3], TrafficClass: dscpValue["AF3"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[0].port,
				RxPorts:       []string{otgBGPConfig[2].port, otgBGPConfig[1].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet2-v4-2",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[0].otgPortData[0].MAC, DstMAC: port1DstMac},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: ate1UserPrefixesV4List[4], IPv4Dst: ate2InternalPrefixesV4List[4], DSCP: dscpValue["AF4"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[0].port,
				RxPorts:       []string{otgBGPConfig[2].port, otgBGPConfig[1].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet2-v6-2",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[0].otgPortData[0].MAC, DstMAC: port1DstMac},
				IPv6Flow:      &otgconfighelpers.IPv6FlowParams{IPv6Src: ate1UserPrefixesV6List[4], IPv6Dst: ate2InternalPrefixesV6List[4], TrafficClass: dscpValue["AF4"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[2].port,
				RxPorts:       []string{otgBGPConfig[0].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet3-v4-1",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[2].otgPortData[0].MAC, DstMAC: port1DstMac},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: dutloopback0.IPv4, IPv4Dst: bgpInternalTE11.IPv4, TTL: ttl},
				UDPFlow:       &otgconfighelpers.UDPFlowParams{UDPDstPort: udpEncapPort},
			},
			innerParams: otgconfighelpers.Flow{
				IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: ate2InternalPrefixesV4List[0], IPv4Dst: ate1UserPrefixesV4List[0], DSCP: dscpValue["BE1"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[2].port,
				RxPorts:       []string{otgBGPConfig[0].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet3-v6-1",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[2].otgPortData[0].MAC, DstMAC: port1DstMac},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: dutloopback0.IPv4, IPv4Dst: bgpInternalTE11.IPv4, TTL: ttl},
				UDPFlow:       &otgconfighelpers.UDPFlowParams{UDPDstPort: udpEncapPort},
			},
			innerParams: otgconfighelpers.Flow{
				IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: ate2InternalPrefixesV6List[0], IPv6Dst: ate1UserPrefixesV6List[0], TrafficClass: dscpValue["BE1"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[2].port,
				RxPorts:       []string{otgBGPConfig[0].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet3-v4-2",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[2].otgPortData[0].MAC, DstMAC: port1DstMac},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: dutloopback0.IPv4, IPv4Dst: bgpInternalTE11.IPv4, TTL: ttl},
				UDPFlow:       &otgconfighelpers.UDPFlowParams{UDPDstPort: udpEncapPort},
			},
			innerParams: otgconfighelpers.Flow{
				IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: ate2InternalPrefixesV4List[1], IPv4Dst: ate1UserPrefixesV4List[1], DSCP: dscpValue["AF1"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[2].port,
				RxPorts:       []string{otgBGPConfig[0].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet3-v6-2",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[2].otgPortData[0].MAC, DstMAC: port1DstMac},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: dutloopback0.IPv4, IPv4Dst: bgpInternalTE11.IPv4, TTL: ttl},
				UDPFlow:       &otgconfighelpers.UDPFlowParams{UDPDstPort: udpEncapPort},
			},
			innerParams: otgconfighelpers.Flow{
				IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: ate2InternalPrefixesV6List[1], IPv6Dst: ate1UserPrefixesV6List[1], TrafficClass: dscpValue["AF1"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[2].port,
				RxPorts:       []string{otgBGPConfig[0].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet3-v4-3",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[2].otgPortData[0].MAC, DstMAC: port1DstMac},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: dutloopback0.IPv4, IPv4Dst: bgpInternalTE11.IPv4, TTL: ttl},
				UDPFlow:       &otgconfighelpers.UDPFlowParams{UDPDstPort: udpEncapPort},
			},
			innerParams: otgconfighelpers.Flow{
				IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: ate2InternalPrefixesV4List[2], IPv4Dst: ate1UserPrefixesV4List[2], DSCP: dscpValue["AF2"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[2].port,
				RxPorts:       []string{otgBGPConfig[0].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet3-v6-3",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[2].otgPortData[0].MAC, DstMAC: port1DstMac},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: dutloopback0.IPv4, IPv4Dst: bgpInternalTE11.IPv4, TTL: ttl},
				UDPFlow:       &otgconfighelpers.UDPFlowParams{UDPDstPort: udpEncapPort},
			},
			innerParams: otgconfighelpers.Flow{
				IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: ate2InternalPrefixesV6List[2], IPv6Dst: ate1UserPrefixesV6List[2], TrafficClass: dscpValue["AF2"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[2].port,
				RxPorts:       []string{otgBGPConfig[0].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet4-v4-1",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[2].otgPortData[0].MAC, DstMAC: port1DstMac},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: dutloopback0.IPv4, IPv4Dst: bgpInternalTE10.IPv4, TTL: ttl},
				UDPFlow:       &otgconfighelpers.UDPFlowParams{UDPDstPort: udpEncapPort},
			},
			innerParams: otgconfighelpers.Flow{
				IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: ate2InternalPrefixesV4List[3], IPv4Dst: ate1UserPrefixesV4List[3], DSCP: dscpValue["AF3"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[2].port,
				RxPorts:       []string{otgBGPConfig[0].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet4-v6-1",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[2].otgPortData[0].MAC, DstMAC: port1DstMac},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: dutloopback0.IPv4, IPv4Dst: bgpInternalTE10.IPv4, TTL: ttl},
				UDPFlow:       &otgconfighelpers.UDPFlowParams{UDPDstPort: udpEncapPort},
			},
			innerParams: otgconfighelpers.Flow{
				IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: ate2InternalPrefixesV6List[3], IPv6Dst: ate1UserPrefixesV6List[3], TrafficClass: dscpValue["AF3"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[2].port,
				RxPorts:       []string{otgBGPConfig[0].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet4-v4-2",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[2].otgPortData[0].MAC, DstMAC: port1DstMac},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: dutloopback0.IPv4, IPv4Dst: bgpInternalTE10.IPv4, TTL: ttl},
				UDPFlow:       &otgconfighelpers.UDPFlowParams{UDPDstPort: udpEncapPort},
			},
			innerParams: otgconfighelpers.Flow{
				IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: ate2InternalPrefixesV4List[4], IPv4Dst: ate1UserPrefixesV4List[4], DSCP: dscpValue["AF4"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[2].port,
				RxPorts:       []string{otgBGPConfig[0].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet4-v6-2",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[2].otgPortData[0].MAC, DstMAC: port1DstMac},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: dutloopback0.IPv4, IPv4Dst: bgpInternalTE10.IPv4, TTL: ttl},
				UDPFlow:       &otgconfighelpers.UDPFlowParams{UDPDstPort: udpEncapPort},
			},
			innerParams: otgconfighelpers.Flow{
				IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: ate2InternalPrefixesV6List[4], IPv6Dst: ate1UserPrefixesV6List[4], TrafficClass: dscpValue["AF4"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[1].port,
				RxPorts:       []string{otgBGPConfig[0].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet5-v4-1",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[1].otgPortData[1].MAC, DstMAC: port2DstMac},
				VLANFlow:      &otgconfighelpers.VLANFlowParams{VLANId: otgBGPConfig[1].otgPortData[1].Subinterface},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: dutloopback0.IPv4, IPv4Dst: bgpInternalTE11.IPv4, TTL: ttl},
				UDPFlow:       &otgconfighelpers.UDPFlowParams{UDPDstPort: udpEncapPort},
			},
			innerParams: otgconfighelpers.Flow{
				IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: ate2InternalPrefixesV4List[0], IPv4Dst: ate1UserPrefixesV4List[0], DSCP: dscpValue["BE1"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[1].port,
				RxPorts:       []string{otgBGPConfig[0].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet5-v6-1",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[1].otgPortData[1].MAC, DstMAC: port2DstMac},
				VLANFlow:      &otgconfighelpers.VLANFlowParams{VLANId: otgBGPConfig[1].otgPortData[1].Subinterface},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: dutloopback0.IPv4, IPv4Dst: bgpInternalTE11.IPv4, TTL: ttl},
				UDPFlow:       &otgconfighelpers.UDPFlowParams{UDPDstPort: udpEncapPort},
			},
			innerParams: otgconfighelpers.Flow{
				IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: ate2InternalPrefixesV6List[0], IPv6Dst: ate1UserPrefixesV6List[0], TrafficClass: dscpValue["BE1"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[1].port,
				RxPorts:       []string{otgBGPConfig[0].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet5-v4-2",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[1].otgPortData[1].MAC, DstMAC: port2DstMac},
				VLANFlow:      &otgconfighelpers.VLANFlowParams{VLANId: otgBGPConfig[1].otgPortData[1].Subinterface},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: dutloopback0.IPv4, IPv4Dst: bgpInternalTE11.IPv4, TTL: ttl},
				UDPFlow:       &otgconfighelpers.UDPFlowParams{UDPDstPort: udpEncapPort},
			},
			innerParams: otgconfighelpers.Flow{
				IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: ate2InternalPrefixesV4List[1], IPv4Dst: ate1UserPrefixesV4List[1], DSCP: dscpValue["AF1"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[1].port,
				RxPorts:       []string{otgBGPConfig[0].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet5-v6-2",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[1].otgPortData[1].MAC, DstMAC: port2DstMac},
				VLANFlow:      &otgconfighelpers.VLANFlowParams{VLANId: otgBGPConfig[1].otgPortData[1].Subinterface},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: dutloopback0.IPv4, IPv4Dst: bgpInternalTE11.IPv4, TTL: ttl},
				UDPFlow:       &otgconfighelpers.UDPFlowParams{UDPDstPort: udpEncapPort},
			},
			innerParams: otgconfighelpers.Flow{
				IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: ate2InternalPrefixesV6List[1], IPv6Dst: ate1UserPrefixesV6List[1], TrafficClass: dscpValue["AF1"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[1].port,
				RxPorts:       []string{otgBGPConfig[0].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet5-v4-3",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[1].otgPortData[1].MAC, DstMAC: port2DstMac},
				VLANFlow:      &otgconfighelpers.VLANFlowParams{VLANId: otgBGPConfig[1].otgPortData[1].Subinterface},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: dutloopback0.IPv4, IPv4Dst: bgpInternalTE11.IPv4, TTL: ttl},
				UDPFlow:       &otgconfighelpers.UDPFlowParams{UDPDstPort: udpEncapPort},
			},
			innerParams: otgconfighelpers.Flow{
				IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: ate2InternalPrefixesV4List[2], IPv4Dst: ate1UserPrefixesV4List[2], DSCP: dscpValue["AF2"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[1].port,
				RxPorts:       []string{otgBGPConfig[0].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet5-v6-3",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[1].otgPortData[1].MAC, DstMAC: port2DstMac},
				VLANFlow:      &otgconfighelpers.VLANFlowParams{VLANId: otgBGPConfig[1].otgPortData[1].Subinterface},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: dutloopback0.IPv4, IPv4Dst: bgpInternalTE11.IPv4, TTL: ttl},
				UDPFlow:       &otgconfighelpers.UDPFlowParams{UDPDstPort: udpEncapPort},
			},
			innerParams: otgconfighelpers.Flow{
				IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: ate2InternalPrefixesV6List[2], IPv6Dst: ate1UserPrefixesV6List[2], TrafficClass: dscpValue["AF2"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[1].port,
				RxPorts:       []string{otgBGPConfig[0].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet5-v4-4",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[1].otgPortData[1].MAC, DstMAC: port2DstMac},
				VLANFlow:      &otgconfighelpers.VLANFlowParams{VLANId: otgBGPConfig[1].otgPortData[1].Subinterface},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: dutloopback0.IPv4, IPv4Dst: bgpInternalTE11.IPv4, TTL: ttl},
				UDPFlow:       &otgconfighelpers.UDPFlowParams{UDPDstPort: udpEncapPort},
			},
			innerParams: otgconfighelpers.Flow{
				IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: ate2InternalPrefixesV4List[3], IPv4Dst: ate1UserPrefixesV4List[3], DSCP: dscpValue["AF3"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[1].port,
				RxPorts:       []string{otgBGPConfig[0].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet5-v6-4",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[1].otgPortData[1].MAC, DstMAC: port2DstMac},
				VLANFlow:      &otgconfighelpers.VLANFlowParams{VLANId: otgBGPConfig[1].otgPortData[1].Subinterface},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: dutloopback0.IPv4, IPv4Dst: bgpInternalTE11.IPv4, TTL: ttl},
				UDPFlow:       &otgconfighelpers.UDPFlowParams{UDPDstPort: udpEncapPort},
			},
			innerParams: otgconfighelpers.Flow{
				IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: ate2InternalPrefixesV6List[3], IPv6Dst: ate1UserPrefixesV6List[3], TrafficClass: dscpValue["AF3"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[1].port,
				RxPorts:       []string{otgBGPConfig[0].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet5-v4-5",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[1].otgPortData[1].MAC, DstMAC: port2DstMac},
				VLANFlow:      &otgconfighelpers.VLANFlowParams{VLANId: otgBGPConfig[1].otgPortData[1].Subinterface},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: dutloopback0.IPv4, IPv4Dst: bgpInternalTE11.IPv4, TTL: ttl},
				UDPFlow:       &otgconfighelpers.UDPFlowParams{UDPDstPort: udpEncapPort},
			},
			innerParams: otgconfighelpers.Flow{
				IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: ate2InternalPrefixesV4List[4], IPv4Dst: ate1UserPrefixesV4List[4], DSCP: dscpValue["AF4"]},
			},
		},
		{
			flows: otgconfighelpers.Flow{
				TxPort:        otgBGPConfig[1].port,
				RxPorts:       []string{otgBGPConfig[0].port},
				IsTxRxPort:    true,
				PacketsToSend: totalPackets,
				PpsRate:       trafficPps,
				FlowName:      "flowSet5-v6-5",
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: otgBGPConfig[1].otgPortData[1].MAC, DstMAC: port2DstMac},
				VLANFlow:      &otgconfighelpers.VLANFlowParams{VLANId: otgBGPConfig[1].otgPortData[1].Subinterface},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: dutloopback0.IPv4, IPv4Dst: bgpInternalTE11.IPv4, TTL: ttl},
				UDPFlow:       &otgconfighelpers.UDPFlowParams{UDPDstPort: udpEncapPort},
			},
			innerParams: otgconfighelpers.Flow{
				IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: ate2InternalPrefixesV6List[4], IPv6Dst: ate1UserPrefixesV6List[4], TrafficClass: dscpValue["AF4"]},
			},
		},
	}

	configureTrafficFlows(t, otgConfig, trafficFlowData)

	type testCase struct {
		Name        string
		Description string
		testFunc    func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config)
	}

	testCases := []testCase{
		{
			Name:        "Testcase: Validate the basic config",
			Description: "Validate traffic with basic config",
			testFunc:    testBaselineTraffic,
		},
		{
			Name:        "Testcase: Verify BE1 Traffic Migrated from being routed over the DUT_Port2",
			Description: "Verify BE1 Traffic Migrated from being routed over the DUT_Port2",
			testFunc:    testBE1TrafficMigration,
		},
		{
			Name:        "Testcase: Verify AF1 Traffic Migrated from being routed over the DUT_Port2",
			Description: "Verify AF1 Traffic Migrated from being routed over the DUT_Port2",
			testFunc:    testAF1TrafficMigration,
		},
		{
			Name:        "Testcase: Verify AF2 Traffic Migrated from being routed over the DUT_Port2",
			Description: "Verify AF2 Traffic Migrated from being routed over the DUT_Port2",
			testFunc:    testAF2TrafficMigration,
		},
		{
			Name:        "Testcase: Verify AF3 Traffic Migrated from being routed over the DUT_Port2",
			Description: "Verify AF3 Traffic Migrated from being routed over the DUT_Port2",
			testFunc:    testAF3TrafficMigration,
		},
		{
			Name:        "Testcase: Verify AF4 Traffic Migrated from being routed over the DUT_Port2",
			Description: "Verify AF4 Traffic Migrated from being routed over the DUT_Port2",
			testFunc:    testAF4TrafficMigration,
		},
		{
			Name:        "Testcase: DUT as a GUE Decap Node",
			Description: "Verify DUT as a GUE Decap Node",
			testFunc:    testDUTDecapNode,
		},
		{
			Name:        "Testcase: Negative Scenario - EBGP Route for remote tunnel endpoints Removed",
			Description: "Verify EBGP Route for remote tunnel endpoints Removed",
			testFunc:    testTunnelEndpointRemoved,
		},
		{
			Name:        "Testcase: Negative Scenario - IBGP Route for Remote Tunnel Endpoints Removed",
			Description: "Verify IBGP Route for Remote Tunnel Endpoints Removed",
			testFunc:    testIbgpTunnelEndpointRemoved,
		},
		{
			Name:        "Testcase: Establish IBGP Peering over EBGP",
			Description: "Verify Establish IBGP Peering over EBGP",
			testFunc:    testEstablishIBGPoverEBGP,
		},
	}

	// Run the test cases.
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Logf("Description: %s", tc.Description)
			tc.testFunc(t, dut, ate, otgConfig)
		})
	}

}

func testBaselineTraffic(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {
	flowsets := []string{"flowSet1", "flowSet2", "flowSet5"}

	otgConfig.Flows().Clear()

	for _, flowset := range flowsets {
		otgConfig.Flows().Append(flowGroups[flowset].Flows...)
	}

	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)

	t.Logf("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, ate)

	t.Logf("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut)

	// Validating no prefixes are exchanged over the IBGP peering between $ATE2_C.IBGP.v6 and $DUT_lo0.v6
	validatePrefixes(t, dut, otgBGPConfig[1].otgPortData[2].IPv6, false, 0, 0)

	sendTrafficCapture(t, ate, otgConfig, []string{"all"})

	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
	otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)

	for _, flows := range flowsets {
		for _, flow := range flowGroups[flows].Flows {
			verifyTrafficFlow(t, ate, flow.Name())
		}
	}
}

func testBE1TrafficMigration(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {
	advertiseRoutesWithiBGP([]string{ate2InternalPrefixesV4List[0]}, ate2ppnh1.IPv6, true, fmt.Sprintf("%s-CBGP-1", atePort2RoutesV4))
	advertiseRoutesWithiBGP([]string{ate2InternalPrefixesV6List[0]}, ate2ppnh1.IPv6, false, fmt.Sprintf("%s-CBGP-1", atePort2RoutesV6))

	flowsets := []string{"flowSet1", "flowSet5"}

	flowNames := map[string][]string{
		"v4": {},
		"v6": {},
	}

	otgConfig.Flows().Clear()

	for _, flow := range flowsets {
		otgConfig.Flows().Append(flowGroups[flow].Flows[0:2]...)

		flowNames["v4"] = append(flowNames["v4"], flowGroups[flow].Flows[0].Name())
		flowNames["v6"] = append(flowNames["v6"], flowGroups[flow].Flows[1].Name())
	}

	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)

	t.Logf("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, ate)

	t.Logf("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut)

	time.Sleep(10 * time.Second)
	// Validating routes to prefixes learnt from $ATE2_C.IBGP.v6/128
	validatePrefixes(t, dut, otgBGPConfig[1].otgPortData[1].IPv6, false, 1, 5)

	testTrafficMigration(t, dut, ate, otgConfig, flowNames, "BE1")

}

func testAF1TrafficMigration(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {
	advertiseRoutesWithiBGP([]string{ate2InternalPrefixesV4List[1]}, ate2ppnh1.IPv6, true, fmt.Sprintf("%s-CBGP-2", atePort2RoutesV4))
	advertiseRoutesWithiBGP([]string{ate2InternalPrefixesV6List[1]}, ate2ppnh1.IPv6, false, fmt.Sprintf("%s-CBGP-2", atePort2RoutesV6))
	flowsets := []string{"flowSet1", "flowSet5"}
	flowNames := map[string][]string{
		"v4": {},
		"v6": {},
	}

	otgConfig.Flows().Clear()

	for _, flow := range flowsets {
		otgConfig.Flows().Append(flowGroups[flow].Flows[2:4]...)

		flowNames["v4"] = append(flowNames["v4"], flowGroups[flow].Flows[2].Name())
		flowNames["v6"] = append(flowNames["v6"], flowGroups[flow].Flows[3].Name())
	}

	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)

	t.Logf("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, ate)

	t.Logf("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut)

	time.Sleep(10 * time.Second)
	// Validating routes to prefixes learnt from $ATE2_C.IBGP.v6/128
	validatePrefixes(t, dut, otgBGPConfig[1].otgPortData[1].IPv6, false, 2, 5)

	t.Log("Validate IPv4 GUE encapsulation and decapsulation")
	sendTrafficCapture(t, ate, otgConfig, flowNames["v4"])
	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)

	for _, flow := range flowNames["v4"] {
		verifyTrafficFlow(t, ate, flow)
	}

	testTrafficMigration(t, dut, ate, otgConfig, flowNames, "AF1")
}

func testAF2TrafficMigration(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {
	advertiseRoutesWithiBGP([]string{ate2InternalPrefixesV4List[2]}, ate2ppnh1.IPv6, true, fmt.Sprintf("%s-CBGP-3", atePort2RoutesV4))
	advertiseRoutesWithiBGP([]string{ate2InternalPrefixesV6List[2]}, ate2ppnh1.IPv6, false, fmt.Sprintf("%s-CBGP-3", atePort2RoutesV6))

	flowNames := map[string][]string{
		"v4": {},
		"v6": {},
	}

	otgConfig.Flows().Clear()

	otgConfig.Flows().Append(flowGroups["flowSet1"].Flows[4:]...)
	flowNames["v4"] = append(flowNames["v4"], flowGroups["flowSet1"].Flows[4].Name())
	flowNames["v6"] = append(flowNames["v6"], flowGroups["flowSet1"].Flows[5].Name())

	otgConfig.Flows().Append(flowGroups["flowSet5"].Flows[4:6]...)
	flowNames["v4"] = append(flowNames["v4"], flowGroups["flowSet5"].Flows[4].Name())
	flowNames["v6"] = append(flowNames["v6"], flowGroups["flowSet5"].Flows[5].Name())

	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)

	t.Logf("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, ate)

	t.Logf("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut)
	time.Sleep(10 * time.Second)
	// Validating routes to prefixes learnt from $ATE2_C.IBGP.v6/128
	validatePrefixes(t, dut, otgBGPConfig[1].otgPortData[1].IPv6, false, 3, 5)

	t.Log("Validate IPv4 GUE encapsulation and decapsulation")
	sendTrafficCapture(t, ate, otgConfig, flowNames["v4"])
	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)

	for _, flow := range flowNames["v4"] {
		verifyTrafficFlow(t, ate, flow)
	}

	testTrafficMigration(t, dut, ate, otgConfig, flowNames, "AF2")
}

func testAF3TrafficMigration(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {
	advertiseRoutesWithiBGP([]string{ate2InternalPrefixesV4List[3]}, ate2ppnh2.IPv6, true, fmt.Sprintf("%s-CBGP-4", atePort2RoutesV4))
	advertiseRoutesWithiBGP([]string{ate2InternalPrefixesV6List[3]}, ate2ppnh2.IPv6, false, fmt.Sprintf("%s-CBGP-4", atePort2RoutesV6))

	flowNames := map[string][]string{
		"v4": {},
		"v6": {},
	}

	otgConfig.Flows().Clear()

	otgConfig.Flows().Append(flowGroups["flowSet2"].Flows[0:2]...)
	flowNames["v4"] = append(flowNames["v4"], flowGroups["flowSet2"].Flows[0].Name())
	flowNames["v6"] = append(flowNames["v6"], flowGroups["flowSet2"].Flows[1].Name())

	otgConfig.Flows().Append(flowGroups["flowSet5"].Flows[6:8]...)
	flowNames["v4"] = append(flowNames["v4"], flowGroups["flowSet5"].Flows[6].Name())
	flowNames["v6"] = append(flowNames["v6"], flowGroups["flowSet5"].Flows[7].Name())

	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)

	t.Logf("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, ate)

	t.Logf("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut)

	time.Sleep(10 * time.Second)
	// Validating routes to prefixes learnt from $ATE2_C.IBGP.v6/128
	validatePrefixes(t, dut, otgBGPConfig[1].otgPortData[1].IPv6, false, 4, 5)

	t.Log("Validate IPv4 GUE encapsulation and decapsulation")
	sendTrafficCapture(t, ate, otgConfig, flowNames["v4"])
	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)

	for _, flow := range flowNames["v4"] {
		verifyTrafficFlow(t, ate, flow)
	}

	testTrafficMigration(t, dut, ate, otgConfig, flowNames, "AF3")
}

func testAF4TrafficMigration(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {
	advertiseRoutesWithiBGP([]string{ate2InternalPrefixesV4List[4]}, ate2ppnh2.IPv6, true, fmt.Sprintf("%s-CBGP-5", atePort2RoutesV4))
	advertiseRoutesWithiBGP([]string{ate2InternalPrefixesV6List[4]}, ate2ppnh2.IPv6, false, fmt.Sprintf("%s-CBGP-5", atePort2RoutesV6))

	flowNames := map[string][]string{
		"v4": {},
		"v6": {},
	}

	otgConfig.Flows().Clear()

	otgConfig.Flows().Append(flowGroups["flowSet2"].Flows[2:]...)
	flowNames["v4"] = append(flowNames["v4"], flowGroups["flowSet2"].Flows[2].Name())
	flowNames["v6"] = append(flowNames["v6"], flowGroups["flowSet2"].Flows[3].Name())

	otgConfig.Flows().Append(flowGroups["flowSet5"].Flows[8:10]...)
	flowNames["v4"] = append(flowNames["v4"], flowGroups["flowSet5"].Flows[8].Name())
	flowNames["v6"] = append(flowNames["v6"], flowGroups["flowSet5"].Flows[9].Name())

	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)

	t.Logf("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, ate)

	t.Logf("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut)

	time.Sleep(10 * time.Second)
	// Validating routes to prefixes learnt from $ATE2_C.IBGP.v6/128
	validatePrefixes(t, dut, otgBGPConfig[1].otgPortData[1].IPv6, false, 5, 5)

	t.Log("Validate IPv4 GUE encapsulation and decapsulation")
	sendTrafficCapture(t, ate, otgConfig, flowNames["v4"])
	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)

	for _, flow := range flowNames["v4"] {
		verifyTrafficFlow(t, ate, flow)
	}
	testTrafficMigration(t, dut, ate, otgConfig, flowNames, "AF4")
}

func testDUTDecapNode(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {
	// Active flows for Flow-Set #1 through Flow-Set #4.
	flowsets := []string{"flowSet1", "flowSet2", "flowSet3", "flowSet4"}

	otgConfig.Flows().Clear()

	for _, flowset := range flowsets {
		otgConfig.Flows().Append(flowGroups[flowset].Flows...)
	}

	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)

	t.Logf("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, ate)

	t.Logf("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut)

	sendTrafficCapture(t, ate, otgConfig, []string{"all"})

	// Verify Traffic
	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
	otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)

	for _, flows := range flowsets {
		for _, flow := range flowGroups[flows].Flows {
			verifyTrafficFlow(t, ate, flow.Name())
		}
	}
}

func testTunnelEndpointRemoved(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {
	flowsets := []string{"flowSet1", "flowSet2", "flowSet3", "flowSet4"}

	otgConfig.Flows().Clear()

	for _, flowset := range flowsets {
		otgConfig.Flows().Append(flowGroups[flowset].Flows...)
	}

	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)

	withdrawBGPRoutes(t, []string{"ebgp4-te10-routes", "ebgp4-te11-routes"})
	time.Sleep(20 * time.Second)

	t.Logf("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, ate)

	t.Logf("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut)

	time.Sleep(10 * time.Second)
	validatePrefixes(t, dut, otgBGPConfig[2].otgPortData[0].IPv4, true, 0, 12)

	sendTrafficCapture(t, ate, otgConfig, []string{"all"})

	// Verify Traffic
	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
	otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)

	for _, flows := range flowsets {
		for _, flow := range flowGroups[flows].Flows {
			verifyTrafficFlow(t, ate, flow.Name())
		}
	}
}

func testIbgpTunnelEndpointRemoved(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {
	_, ni, _ := cfgplugins.SetupPolicyForwardingInfraOC(deviations.DefaultNetworkInstance(dut))

	if deviations.NextHopGroupOCUnsupported(dut) {
		interfacePolicyParams := cfgplugins.OcPolicyForwardingParams{
			InterfaceID:       dut.Port(t, "port1").Name(),
			AppliedPolicyName: guePolicyName,
			RemovePolicyName:  true,
		}
		cfgplugins.InterfacePolicyForwardingApply(t, dut, dut.Port(t, "port1").Name(), guePolicyName, ni, interfacePolicyParams)
	}

	t.Log("Stop advertising tunnel endpoints on ATE Port2")
	withdrawBGPRoutes(t, []string{atePort2RoutesTE10, atePort2RoutesTE11})
	time.Sleep(10 * time.Second)

	b := &gnmi.SetBatch{}

	// Configuring Static Route: PNH-IPv6 --> IPv4 GUE tunnel.
	sV4 := &cfgplugins.StaticRouteCfg{
		NetworkInstance:   deviations.DefaultNetworkInstance(dut),
		Prefix:            ate2ppnh1Prefix,
		NexthopGroup:      true,
		NexthopGroupName:  nexthopGroupName2,
		T:                 t,
		RemoveStaticRoute: true,
	}

	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
		t.Fatalf("Failed to configure IPv6 static route: %v", err)
	}
	b.Set(t, dut)

	sV4 = &cfgplugins.StaticRouteCfg{
		NetworkInstance:   deviations.DefaultNetworkInstance(dut),
		Prefix:            ate2ppnh2Prefix,
		NexthopGroup:      true,
		NexthopGroupName:  nexthopGroupName1,
		T:                 t,
		RemoveStaticRoute: true,
	}

	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
		t.Fatalf("Failed to configure IPv6 static route: %v", err)
	}
	b.Set(t, dut)

	validateAFTCounters(t, dut, false, ate2ppnh1Prefix)
	validateAFTCounters(t, dut, false, ate2ppnh2Prefix)

	validatePrefixes(t, dut, otgBGPConfig[1].otgPortData[0].IPv6, false, 5, 5)

	sendTrafficCapture(t, ate, otgConfig, []string{"all"})

	// Verify Traffic
	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
	otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)

	for _, flows := range []string{"flowSet1", "flowSet2", "flowSet3", "flowSet4"} {
		for _, flow := range flowGroups[flows].Flows {
			verifyTrafficFlow(t, ate, flow.Name())
		}
	}

}

func testEstablishIBGPoverEBGP(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {
	_, ni, _ := cfgplugins.SetupPolicyForwardingInfraOC(deviations.DefaultNetworkInstance(dut))

	if deviations.NextHopGroupOCUnsupported(dut) {
		interfacePolicyParams := cfgplugins.OcPolicyForwardingParams{
			InterfaceID:       dut.Port(t, "port1").Name(),
			AppliedPolicyName: guePolicyName,
		}
		cfgplugins.InterfacePolicyForwardingApply(t, dut, dut.Port(t, "port1").Name(), guePolicyName, ni, interfacePolicyParams)
	}

	configureStaticRoute(t, dut)

	// Active flows for Flow-Set #1 through Flow-Set #4.
	port3Data := otgBGPConfig[2]
	iDut3Dev := port3Data.otgDevice[0]

	bgpPeer := iDut3Dev.Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0]
	v4routes := bgpPeer.V4Routes().Add().SetName("ATE2_C_IBGP_via_EBGP")
	v4routes.Addresses().Add().SetAddress(ate2InternalPrefixesV4).SetPrefix(24).SetCount(5)

	bgpPeerv6 := iDut3Dev.Bgp().Ipv6Interfaces().Items()[0].Peers().Items()[0]
	v6routes := bgpPeerv6.V6Routes().Add().SetName("ATE2_C_IBGP_via_EBGPv6")
	v6routes.Addresses().Add().SetAddress(ate2InternalPrefixesV6).SetPrefix(64).SetCount(5)
	ate.OTG().PushConfig(t, otgConfig)

	time.Sleep(20 * time.Second)
	d := &oc.Root{}
	i := d.GetOrCreateInterface(dut.Port(t, "port2").Name())
	i.SetEnabled(false)
	gnmi.Replace(t, dut, gnmi.OC().Interface(dut.Port(t, "port2").Name()).Config(), i)

	ate.OTG().StartProtocols(t)

	// Validating one flow to be encapsulated when sent from Port1 -> Port3
	sendTrafficCapture(t, ate, otgConfig, []string{flowGroups["flowSet1"].Flows[0].Name()})

	gueLayer := *outerGUEIPLayerIPv4
	gueLayer.Tos = uint8(expectedDscpValue["BE1"])
	encapValidation.IPv4Layer = &gueLayer

	innerGueLayer := *innerGUEIPLayerIPv4
	innerGueLayer.Tos = uint8(expectedDscpValue["BE1"])
	encapValidation.InnerIPLayerIPv4 = &innerGueLayer
	if err := validatePacket(t, ate, encapValidation); err != nil {
		t.Errorf("capture and validatePackets failed (): %q", err)
	}

	// Validting no traffic loss for other flows
	sendTrafficCapture(t, ate, otgConfig, []string{"all"})

	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
	otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)

	for _, flows := range []string{"flowSet1", "flowSet2", "flowSet3", "flowSet4"} {
		for _, flow := range flowGroups[flows].Flows {
			verifyTrafficFlow(t, ate, flow.Name())
		}
	}
}
