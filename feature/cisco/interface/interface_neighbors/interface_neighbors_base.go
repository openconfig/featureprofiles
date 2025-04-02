package interface_neighbors_test

import (
	"context"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/openconfig/featureprofiles/internal/attrs"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/interfaces"
	"github.com/openconfig/ondatra/gnmi/oc/platform"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen    = 24
	ipv6PrefixLen    = 119
	vlanID           = 1
	staticIPv4MAC    = "02:12:01:00:02:01"
	staticIPv6MAC    = "03:12:01:00:03:01"
	TOTAL_INTF_COUNT = 4
)
const (
	RAInterval           = 4
	RALifetime           = 9000
	RAOtherConfig        = true
	RASuppress           = true
	RAInterval_Update    = 1800
	RALifetime_Update    = 0
	RAOtherConfig_Update = false
	RASuppress_Update    = false
)
const (
	NDPrefix                                = "300:0:2::1/124"
	NDPrefixPreferredLifetime               = 5000
	NDPrefixValidLifetime                   = 6000
	NDPrefixDisableAutoconfiguration        = true
	NDPrefixEnableOnlink                    = true
	NDPrefix_Update                         = "400:0:2::1/124"
	NDPrefixPreferredLifetime_Update        = 1000
	NDPrefixValidLifetime_Update            = 10000
	NDPrefixDisableAutoconfiguration_Update = false
	NDPrefixEnableOnlink_Update             = false
)
const (
	NDDad        = 5
	NDDad_Update = 10
)
const (
	static_bit = 1
	delete_bit = 2
	update_bit = 4
)
const (
	MAX_PHY_INTF           = 9
	MAX_BUNDLE_INTF        = 9
	MAX_SUB_INTF           = 20
	MAX_LC_COUNT           = 8
	TOTAL_SCALE_INTF_COUNT = 2268
	TOTAL_BUNDLE_INTF      = (MAX_BUNDLE_INTF * 2) * (MAX_LC_COUNT / 2)

	DUT1_LC0_LC1_BASE_PHY_IPV4 = "10.10.1.1"
	DUT2_LC0_LC1_BASE_PHY_IPV4 = "10.10.1.2"
	DUT1_LC2_LC3_BASE_PHY_IPV4 = "11.10.1.1"
	DUT2_LC2_LC3_BASE_PHY_IPV4 = "11.10.1.2"
	DUT1_LC4_LC5_BASE_PHY_IPV4 = "12.10.1.1"
	DUT2_LC4_LC5_BASE_PHY_IPV4 = "12.10.1.2"
	DUT1_LC6_LC7_BASE_PHY_IPV4 = "13.10.1.1"
	DUT2_LC6_LC7_BASE_PHY_IPV4 = "13.10.1.2"

	DUT1_LC0_LC1_BASE_PHY_SUB_IPV4 = "10.11.1.1"
	DUT2_LC0_LC1_BASE_PHY_SUB_IPV4 = "10.11.1.2"
	DUT1_LC2_LC3_BASE_PHY_SUB_IPV4 = "11.11.1.1"
	DUT2_LC2_LC3_BASE_PHY_SUB_IPV4 = "11.11.1.2"
	DUT1_LC4_LC5_BASE_PHY_SUB_IPV4 = "12.11.1.1"
	DUT2_LC4_LC5_BASE_PHY_SUB_IPV4 = "12.11.1.2"
	DUT1_LC6_LC7_BASE_PHY_SUB_IPV4 = "13.11.1.1"
	DUT2_LC6_LC7_BASE_PHY_SUB_IPV4 = "13.11.1.2"

	DUT1_LC0_LC1_BASE_BUNDLE_IPV4 = "20.20.1.1"
	DUT2_LC0_LC1_BASE_BUNDLE_IPV4 = "20.20.1.2"
	DUT1_LC2_LC3_BASE_BUNDLE_IPV4 = "21.20.1.1"
	DUT2_LC2_LC3_BASE_BUNDLE_IPV4 = "21.20.1.2"
	DUT1_LC4_LC5_BASE_BUNDLE_IPV4 = "22.20.1.1"
	DUT2_LC4_LC5_BASE_BUNDLE_IPV4 = "22.20.1.2"
	DUT1_LC6_LC7_BASE_BUNDLE_IPV4 = "23.20.1.1"
	DUT2_LC6_LC7_BASE_BUNDLE_IPV4 = "23.20.1.2"

	DUT1_LC0_LC1_BASE_BUNDLE_SUB_IPV4 = "20.21.1.1"
	DUT2_LC0_LC1_BASE_BUNDLE_SUB_IPV4 = "20.21.1.2"
	DUT1_LC2_LC3_BASE_BUNDLE_SUB_IPV4 = "21.21.1.1"
	DUT2_LC2_LC3_BASE_BUNDLE_SUB_IPV4 = "21.21.1.2"
	DUT1_LC4_LC5_BASE_BUNDLE_SUB_IPV4 = "22.21.1.1"
	DUT2_LC4_LC5_BASE_BUNDLE_SUB_IPV4 = "22.21.1.2"
	DUT1_LC6_LC7_BASE_BUNDLE_SUB_IPV4 = "23.21.1.1"
	DUT2_LC6_LC7_BASE_BUNDLE_SUB_IPV4 = "23.21.1.2"

	DUT1_LC0_LC1_BASE_PHY_IPV6 = "10:10:1::1"
	DUT2_LC0_LC1_BASE_PHY_IPV6 = "10:10:1::2"
	DUT1_LC2_LC3_BASE_PHY_IPV6 = "11:10:1::1"
	DUT2_LC2_LC3_BASE_PHY_IPV6 = "11:10:1::2"
	DUT1_LC4_LC5_BASE_PHY_IPV6 = "12:10:1::1"
	DUT2_LC4_LC5_BASE_PHY_IPV6 = "12:10:1::2"
	DUT1_LC6_LC7_BASE_PHY_IPV6 = "13:10:1::1"
	DUT2_LC6_LC7_BASE_PHY_IPV6 = "13:10:1::2"

	DUT1_LC0_LC1_BASE_PHY_SUB_IPV6 = "10:11:1::1"
	DUT2_LC0_LC1_BASE_PHY_SUB_IPV6 = "10:11:1::2"
	DUT1_LC2_LC3_BASE_PHY_SUB_IPV6 = "11:11:1::1"
	DUT2_LC2_LC3_BASE_PHY_SUB_IPV6 = "11:11:1::2"
	DUT1_LC4_LC5_BASE_PHY_SUB_IPV6 = "12:11:1::1"
	DUT2_LC4_LC5_BASE_PHY_SUB_IPV6 = "12:11:1::2"
	DUT1_LC6_LC7_BASE_PHY_SUB_IPV6 = "13:11:1::1"
	DUT2_LC6_LC7_BASE_PHY_SUB_IPV6 = "13:11:1::2"

	DUT1_LC0_LC1_BASE_BUNDLE_IPV6 = "20:20:1::1"
	DUT2_LC0_LC1_BASE_BUNDLE_IPV6 = "20:20:1::2"
	DUT1_LC2_LC3_BASE_BUNDLE_IPV6 = "21:20:1::1"
	DUT2_LC2_LC3_BASE_BUNDLE_IPV6 = "21:20:1::2"
	DUT1_LC4_LC5_BASE_BUNDLE_IPV6 = "22:20:1::1"
	DUT2_LC4_LC5_BASE_BUNDLE_IPV6 = "22:20:1::2"
	DUT1_LC6_LC7_BASE_BUNDLE_IPV6 = "23:20:1::1"
	DUT2_LC6_LC7_BASE_BUNDLE_IPV6 = "23:20:1::2"

	DUT1_LC0_LC1_BASE_BUNDLE_SUB_IPV6 = "20:21:1::1"
	DUT2_LC0_LC1_BASE_BUNDLE_SUB_IPV6 = "20:21:1::2"
	DUT1_LC2_LC3_BASE_BUNDLE_SUB_IPV6 = "21:21:1::1"
	DUT2_LC2_LC3_BASE_BUNDLE_SUB_IPV6 = "21:21:1::2"
	DUT1_LC4_LC5_BASE_BUNDLE_SUB_IPV6 = "22:21:1::1"
	DUT2_LC4_LC5_BASE_BUNDLE_SUB_IPV6 = "22:21:1::2"
	DUT1_LC6_LC7_BASE_BUNDLE_SUB_IPV6 = "23:21:1::1"
	DUT2_LC6_LC7_BASE_BUNDLE_SUB_IPV6 = "23:21:1::2"
)

type InterfaceIPv4Address struct {
	ipAddress map[string]*oc.Interface_Subinterface_Ipv4_Address
	neighbor  map[string]*oc.Interface_Subinterface_Ipv4_Neighbor
	proxyArp  *oc.Interface_Subinterface_Ipv4_ProxyArp
}

var IntfIPv4Addr map[string]InterfaceIPv4Address

type InterfaceIPv6Address struct {
	ipAddress map[string]*oc.Interface_Subinterface_Ipv6_Address
	neighbor  map[string]*oc.Interface_Subinterface_Ipv6_Neighbor
	dad       uint32
	routerAdv *oc.Interface_Subinterface_Ipv6_RouterAdvertisement
}

var IntfIPv6Addr map[string]InterfaceIPv6Address

type ScaleParam struct {
	vlanID   uint16
	staticIP string
}
type BaseIPAddress struct {
	dutBaseIP []string
}

var mapBaseIPv4Addr = make(map[string][]BaseIPAddress)
var mapBaseIPv6Addr = make(map[string][]BaseIPAddress)

var dut1BaseIPv4Addr = [4]BaseIPAddress{
	{
		dutBaseIP: []string{DUT1_LC0_LC1_BASE_PHY_IPV4, DUT1_LC0_LC1_BASE_PHY_SUB_IPV4, DUT1_LC0_LC1_BASE_BUNDLE_IPV4, DUT1_LC0_LC1_BASE_BUNDLE_SUB_IPV4},
	},
	{
		dutBaseIP: []string{DUT1_LC2_LC3_BASE_PHY_IPV4, DUT1_LC2_LC3_BASE_PHY_SUB_IPV4, DUT1_LC2_LC3_BASE_BUNDLE_IPV4, DUT1_LC2_LC3_BASE_BUNDLE_SUB_IPV4},
	},
	{
		dutBaseIP: []string{DUT1_LC4_LC5_BASE_PHY_IPV4, DUT1_LC4_LC5_BASE_PHY_SUB_IPV4, DUT1_LC4_LC5_BASE_BUNDLE_IPV4, DUT1_LC4_LC5_BASE_BUNDLE_SUB_IPV4},
	},
	{
		dutBaseIP: []string{DUT1_LC6_LC7_BASE_PHY_IPV4, DUT1_LC6_LC7_BASE_PHY_SUB_IPV4, DUT1_LC6_LC7_BASE_BUNDLE_IPV4, DUT1_LC6_LC7_BASE_BUNDLE_SUB_IPV4},
	},
}
var dut2BaseIPv4Addr = [4]BaseIPAddress{
	{
		dutBaseIP: []string{DUT2_LC0_LC1_BASE_PHY_IPV4, DUT2_LC0_LC1_BASE_PHY_SUB_IPV4, DUT2_LC0_LC1_BASE_BUNDLE_IPV4, DUT2_LC0_LC1_BASE_BUNDLE_SUB_IPV4},
	},
	{
		dutBaseIP: []string{DUT2_LC2_LC3_BASE_PHY_IPV4, DUT2_LC2_LC3_BASE_PHY_SUB_IPV4, DUT2_LC2_LC3_BASE_BUNDLE_IPV4, DUT2_LC2_LC3_BASE_BUNDLE_SUB_IPV4},
	},
	{
		dutBaseIP: []string{DUT2_LC4_LC5_BASE_PHY_IPV4, DUT2_LC4_LC5_BASE_PHY_SUB_IPV4, DUT2_LC4_LC5_BASE_BUNDLE_IPV4, DUT2_LC4_LC5_BASE_BUNDLE_SUB_IPV4},
	},
	{
		dutBaseIP: []string{DUT2_LC6_LC7_BASE_PHY_IPV4, DUT2_LC6_LC7_BASE_PHY_SUB_IPV4, DUT2_LC6_LC7_BASE_BUNDLE_IPV4, DUT2_LC6_LC7_BASE_BUNDLE_SUB_IPV4},
	},
}
var dut1BaseIPv6Addr = [4]BaseIPAddress{
	{
		dutBaseIP: []string{DUT1_LC0_LC1_BASE_PHY_IPV6, DUT1_LC0_LC1_BASE_PHY_SUB_IPV6, DUT1_LC0_LC1_BASE_BUNDLE_IPV6, DUT1_LC0_LC1_BASE_BUNDLE_SUB_IPV6},
	},
	{
		dutBaseIP: []string{DUT1_LC2_LC3_BASE_PHY_IPV6, DUT1_LC2_LC3_BASE_PHY_SUB_IPV6, DUT1_LC2_LC3_BASE_BUNDLE_IPV6, DUT1_LC2_LC3_BASE_BUNDLE_SUB_IPV6},
	},
	{
		dutBaseIP: []string{DUT1_LC4_LC5_BASE_PHY_IPV6, DUT1_LC4_LC5_BASE_PHY_SUB_IPV6, DUT1_LC4_LC5_BASE_BUNDLE_IPV6, DUT1_LC4_LC5_BASE_BUNDLE_SUB_IPV6},
	},
	{
		dutBaseIP: []string{DUT1_LC6_LC7_BASE_PHY_IPV6, DUT1_LC6_LC7_BASE_PHY_SUB_IPV6, DUT1_LC6_LC7_BASE_BUNDLE_IPV6, DUT1_LC6_LC7_BASE_BUNDLE_SUB_IPV6},
	},
}
var dut2BaseIPv6Addr = [4]BaseIPAddress{
	{
		dutBaseIP: []string{DUT2_LC0_LC1_BASE_PHY_IPV6, DUT2_LC0_LC1_BASE_PHY_SUB_IPV6, DUT2_LC0_LC1_BASE_BUNDLE_IPV6, DUT2_LC0_LC1_BASE_BUNDLE_SUB_IPV6},
	},
	{
		dutBaseIP: []string{DUT2_LC2_LC3_BASE_PHY_IPV6, DUT2_LC2_LC3_BASE_PHY_SUB_IPV6, DUT2_LC2_LC3_BASE_BUNDLE_IPV6, DUT2_LC2_LC3_BASE_BUNDLE_SUB_IPV6},
	},
	{
		dutBaseIP: []string{DUT2_LC4_LC5_BASE_PHY_IPV6, DUT2_LC4_LC5_BASE_PHY_SUB_IPV6, DUT2_LC4_LC5_BASE_BUNDLE_IPV6, DUT2_LC4_LC5_BASE_BUNDLE_SUB_IPV6},
	},
	{
		dutBaseIP: []string{DUT2_LC6_LC7_BASE_PHY_IPV6, DUT2_LC6_LC7_BASE_PHY_SUB_IPV6, DUT2_LC6_LC7_BASE_BUNDLE_IPV6, DUT2_LC6_LC7_BASE_BUNDLE_SUB_IPV6},
	},
}

type InterfaceLCList struct {
	interfaceLC1 []string
	interfaceLC2 []string
}

type BundleMemberPorts struct {
	BundleName  string
	MemberPorts [2]string
}

var mapBundleMemberPorts = make(map[string][]BundleMemberPorts)

type InterfaceAttributes struct {
	intfName string
	attrib   *attrs.Attributes
}

var dut1IntfAttrib [4]InterfaceAttributes
var dut2IntfAttrib [4]InterfaceAttributes

var (
	RAIntervalDefault = uint32(0)
	RALifetimeDefault = uint32(0)
)
var (
	staticIPv4 = ""
	staticIPv6 = ""
)
var (
	dut1Port1 = attrs.Attributes{
		Desc:         "dut1Port1",
		IPv4:         "192.0.2.1",
		IPv6:         "192:0:2::1",
		Subinterface: 0,
		IPv4Len:      ipv4PrefixLen,
		IPv6Len:      ipv6PrefixLen,
	}
	dut1Port2 = attrs.Attributes{
		Desc:         "dut1Port2",
		IPv4:         "192.0.3.1",
		IPv6:         "192:0:3::1",
		Subinterface: 1,
		IPv4Len:      ipv4PrefixLen,
		IPv6Len:      ipv6PrefixLen,
	}
	dut1Port3 = attrs.Attributes{
		Desc:         "dut1Port3",
		IPv4:         "192.0.4.1",
		IPv6:         "192:0:4::1",
		Subinterface: 0,
		IPv4Len:      ipv4PrefixLen,
		IPv6Len:      ipv6PrefixLen,
	}
	dut1Port4 = attrs.Attributes{
		Desc:         "dut1Port4",
		IPv4:         "192.0.5.1",
		IPv6:         "192:0:5::1",
		Subinterface: 1,
		IPv4Len:      ipv4PrefixLen,
		IPv6Len:      ipv6PrefixLen,
	}
	dut2Port1 = attrs.Attributes{
		Desc:         "dut2Port1",
		IPv4:         "192.0.2.2",
		IPv6:         "192:0:2::2",
		Subinterface: 0,
		IPv4Len:      ipv4PrefixLen,
		IPv6Len:      ipv6PrefixLen,
	}
	dut2Port2 = attrs.Attributes{
		Desc:         "dut2Port2",
		IPv4:         "192.0.3.2",
		IPv6:         "192:0:3::2",
		Subinterface: 1,
		IPv4Len:      ipv4PrefixLen,
		IPv6Len:      ipv6PrefixLen,
	}
	dut2Port3 = attrs.Attributes{
		Desc:         "dut2Port3",
		IPv4:         "192.0.4.2",
		IPv6:         "192:0:4::2",
		Subinterface: 0,
		IPv4Len:      ipv4PrefixLen,
		IPv6Len:      ipv6PrefixLen,
	}
	dut2Port4 = attrs.Attributes{
		Desc:         "dut2Port4",
		IPv4:         "192.0.5.2",
		IPv6:         "192:0:5::2",
		Subinterface: 1,
		IPv4Len:      ipv4PrefixLen,
		IPv6Len:      ipv6PrefixLen,
	}
)

func configureDUT(t *testing.T, dut *ondatra.DUTDevice, IPv4 bool) {

	port1 := dut.Port(t, "port1").Name()
	port2 := dut.Port(t, "port2").Name()
	port3 := dut.Port(t, "port3").Name()
	port4 := dut.Port(t, "port4").Name()
	port5 := dut.Port(t, "port5").Name()
	port6 := dut.Port(t, "port6").Name()
	path1 := gnmi.OC().Interface(port1)
	path2 := gnmi.OC().Interface(port2)
	pathb1 := gnmi.OC().Interface("Bundle-Ether100")
	pathb1m1 := gnmi.OC().Interface(port3)
	pathb1m2 := gnmi.OC().Interface(port4)
	pathb2 := gnmi.OC().Interface("Bundle-Ether101")
	pathb2m1 := gnmi.OC().Interface(port5)
	pathb2m2 := gnmi.OC().Interface(port6)

	i1 := &oc.Interface{Name: &port1}
	i2 := &oc.Interface{Name: &port2}
	i3 := &oc.Interface{Name: ygot.String("Bundle-Ether100")}
	i4 := &oc.Interface{Name: ygot.String("Bundle-Ether101")}

	batchConfig := &gnmi.SetBatch{}

	if IPv4 == true {
		if dut.ID() == "dut1" {
			gnmi.BatchReplace(batchConfig, path1.Config(), configInterfaceIPv4DUT(i1, &dut1Port1))
			gnmi.BatchReplace(batchConfig, path2.Config(), configInterfaceIPv4DUT(i2, &dut1Port2))
			gnmi.BatchReplace(batchConfig, pathb1.Config(), configInterfaceIPv4DUT(i3, &dut1Port3))
			BE100 := generateBundleMemberInterfaceConfig(port3, "Bundle-Ether100")
			gnmi.BatchReplace(batchConfig, pathb1m1.Config(), BE100)
			BE100 = generateBundleMemberInterfaceConfig(port4, "Bundle-Ether100")
			gnmi.BatchReplace(batchConfig, pathb1m2.Config(), BE100)
			gnmi.BatchReplace(batchConfig, pathb2.Config(), configInterfaceIPv4DUT(i4, &dut1Port4))
			BE101 := generateBundleMemberInterfaceConfig(port5, "Bundle-Ether101")
			gnmi.BatchReplace(batchConfig, pathb2m1.Config(), BE101)
			BE101 = generateBundleMemberInterfaceConfig(port6, "Bundle-Ether101")
			gnmi.BatchReplace(batchConfig, pathb2m2.Config(), BE101)
		} else {
			gnmi.BatchReplace(batchConfig, path1.Config(), configInterfaceIPv4DUT(i1, &dut2Port1))
			gnmi.BatchReplace(batchConfig, path2.Config(), configInterfaceIPv4DUT(i2, &dut2Port2))
			gnmi.BatchReplace(batchConfig, pathb1.Config(), configInterfaceIPv4DUT(i3, &dut2Port3))
			BE100 := generateBundleMemberInterfaceConfig(port3, "Bundle-Ether100")
			gnmi.BatchReplace(batchConfig, pathb1m1.Config(), BE100)
			BE100 = generateBundleMemberInterfaceConfig(port4, "Bundle-Ether100")
			gnmi.BatchReplace(batchConfig, pathb1m2.Config(), BE100)
			gnmi.BatchReplace(batchConfig, pathb2.Config(), configInterfaceIPv4DUT(i4, &dut2Port4))
			BE101 := generateBundleMemberInterfaceConfig(port5, "Bundle-Ether101")
			gnmi.BatchReplace(batchConfig, pathb2m1.Config(), BE101)
			BE101 = generateBundleMemberInterfaceConfig(port6, "Bundle-Ether101")
			gnmi.BatchReplace(batchConfig, pathb2m2.Config(), BE101)
		}
	} else {
		if dut.ID() == "dut1" {
			gnmi.BatchReplace(batchConfig, path1.Config(), configInterfaceIPv6DUT(i1, &dut1Port1))
			gnmi.BatchReplace(batchConfig, path2.Config(), configInterfaceIPv6DUT(i2, &dut1Port2))
			gnmi.BatchReplace(batchConfig, pathb1.Config(), configInterfaceIPv6DUT(i3, &dut1Port3))
			BE100 := generateBundleMemberInterfaceConfig(port3, "Bundle-Ether100")
			gnmi.BatchReplace(batchConfig, pathb1m1.Config(), BE100)
			BE100 = generateBundleMemberInterfaceConfig(port4, "Bundle-Ether100")
			gnmi.BatchReplace(batchConfig, pathb1m2.Config(), BE100)
			gnmi.BatchReplace(batchConfig, pathb2.Config(), configInterfaceIPv6DUT(i4, &dut1Port4))
			BE101 := generateBundleMemberInterfaceConfig(port5, "Bundle-Ether101")
			gnmi.BatchReplace(batchConfig, pathb2m1.Config(), BE101)
			BE101 = generateBundleMemberInterfaceConfig(port6, "Bundle-Ether101")
			gnmi.BatchReplace(batchConfig, pathb2m2.Config(), BE101)
		} else {
			gnmi.BatchReplace(batchConfig, path1.Config(), configInterfaceIPv6DUT(i1, &dut2Port1))
			gnmi.BatchReplace(batchConfig, path2.Config(), configInterfaceIPv6DUT(i2, &dut2Port2))
			gnmi.BatchReplace(batchConfig, pathb1.Config(), configInterfaceIPv6DUT(i3, &dut2Port3))
			BE100 := generateBundleMemberInterfaceConfig(port3, "Bundle-Ether100")
			gnmi.BatchReplace(batchConfig, pathb1m1.Config(), BE100)
			BE100 = generateBundleMemberInterfaceConfig(port4, "Bundle-Ether100")
			gnmi.BatchReplace(batchConfig, pathb1m2.Config(), BE100)
			gnmi.BatchReplace(batchConfig, pathb2.Config(), configInterfaceIPv6DUT(i4, &dut2Port4))
			BE101 := generateBundleMemberInterfaceConfig(port5, "Bundle-Ether101")
			gnmi.BatchReplace(batchConfig, pathb2m1.Config(), BE101)
			BE101 = generateBundleMemberInterfaceConfig(port6, "Bundle-Ether101")
			gnmi.BatchReplace(batchConfig, pathb2m2.Config(), BE101)
		}
	}
	batchConfig.Set(t, dut)
}

func configInterfaceIPv4DUT(i *oc.Interface, a *attrs.Attributes, scaleParam ...ScaleParam) *oc.Interface {

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
		if len(scaleParam) > 0 && scaleParam[0].vlanID > 0 {
			s.GetOrCreateVlan().GetOrCreateMatch().
				GetOrCreateSingleTagged().SetVlanId(scaleParam[0].vlanID)
		} else {
			s.GetOrCreateVlan().GetOrCreateMatch().
				GetOrCreateSingleTagged().SetVlanId(vlanID)
		}
	} else {
		s = i.GetOrCreateSubinterface(0)
	}
	s4 := s.GetOrCreateIpv4()
	if i.GetEthernet() != nil {
		if len(scaleParam) > 0 && scaleParam[0].staticIP != "" {
			s4.GetOrCreateNeighbor(scaleParam[0].staticIP).
				SetLinkLayerAddress(staticIPv4MAC)
		} else {
			s4.GetOrCreateNeighbor(staticIPv4).
				SetLinkLayerAddress(staticIPv4MAC)
		}
		return i

	}
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(a.IPv4Len)

	return i
}

func configInterfaceIPv6DUT(i *oc.Interface, a *attrs.Attributes, scaleParam ...ScaleParam) *oc.Interface {

	i.Description = ygot.String(a.Desc)
	s := &oc.Interface_Subinterface{}

	if i.GetName()[:6] == "Bundle" {
		i.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	} else {
		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	}
	if a.Subinterface > 0 {
		s = i.GetOrCreateSubinterface(a.Subinterface)
		if len(scaleParam) > 0 && scaleParam[0].vlanID > 0 {
			s.GetOrCreateVlan().GetOrCreateMatch().
				GetOrCreateSingleTagged().SetVlanId(scaleParam[0].vlanID)
		} else {
			s.GetOrCreateVlan().GetOrCreateMatch().
				GetOrCreateSingleTagged().SetVlanId(vlanID)
		}
	} else {
		s = i.GetOrCreateSubinterface(0)
	}
	s6 := s.GetOrCreateIpv6()
	if i.GetEthernet() != nil {
		if len(scaleParam) > 0 && scaleParam[0].staticIP != "" {
			s6.GetOrCreateNeighbor(scaleParam[0].staticIP).
				SetLinkLayerAddress(staticIPv4MAC)
		} else {
			s6.GetOrCreateNeighbor(staticIPv6).
				SetLinkLayerAddress(staticIPv6MAC)
		}
		return i
	}
	s6a := s6.GetOrCreateAddress(a.IPv6)
	s6a.PrefixLength = ygot.Uint8(a.IPv6Len)

	return i
}

func createIntfAttrib(t *testing.T, dut1 *ondatra.DUTDevice, dut2 *ondatra.DUTDevice) {

	dut1IntfAttrib[0].intfName = dut1.Port(t, "port1").Name()
	dut1IntfAttrib[0].attrib = &dut1Port1
	dut1IntfAttrib[1].intfName = dut1.Port(t, "port2").Name()
	dut1IntfAttrib[1].attrib = &dut1Port2
	dut1IntfAttrib[2].intfName = "Bundle-Ether100"
	dut1IntfAttrib[2].attrib = &dut1Port3
	dut1IntfAttrib[3].intfName = "Bundle-Ether101"
	dut1IntfAttrib[3].attrib = &dut1Port4

	dut2IntfAttrib[0].intfName = dut2.Port(t, "port1").Name()
	dut2IntfAttrib[0].attrib = &dut2Port1
	dut2IntfAttrib[1].intfName = dut2.Port(t, "port2").Name()
	dut2IntfAttrib[1].attrib = &dut2Port2
	dut2IntfAttrib[2].intfName = "Bundle-Ether100"
	dut2IntfAttrib[2].attrib = &dut2Port3
	dut2IntfAttrib[3].intfName = "Bundle-Ether101"
	dut2IntfAttrib[3].attrib = &dut2Port4
}

func configureProxyARP(t *testing.T, dut *ondatra.DUTDevice) {

	batchConfig := &gnmi.SetBatch{}
	var dutIntfAttrib [4]InterfaceAttributes

	if dut.ID() == "dut1" {
		dutIntfAttrib = dut1IntfAttrib
	} else {
		dutIntfAttrib = dut2IntfAttrib
	}
	for i := 0; i < TOTAL_INTF_COUNT; i++ {
		portName := dutIntfAttrib[i].intfName
		attrib := dutIntfAttrib[i].attrib
		idx := attrib.Subinterface

		path := gnmi.OC().Interface(portName)
		obj := &oc.Interface{}
		obj.Name = ygot.String(portName)
		obj.GetOrCreateSubinterface(idx).GetOrCreateIpv4().
			GetOrCreateProxyArp().SetMode(oc.ProxyArp_Mode_ALL)

		gnmi.BatchReplace(batchConfig, path.Config(), configInterfaceIPv4DUT(obj, attrib))
	}
	batchConfig.Set(t, dut)
}

func configureNDRouterAdvDUT(t *testing.T, dut *ondatra.DUTDevice) {

	batchConfig := &gnmi.SetBatch{}
	var dutIntfAttrib [4]InterfaceAttributes

	if dut.ID() == "dut1" {
		dutIntfAttrib = dut1IntfAttrib
	} else {
		dutIntfAttrib = dut2IntfAttrib
	}
	for i := 0; i < TOTAL_INTF_COUNT; i++ {
		portName := dutIntfAttrib[i].intfName
		attrib := dutIntfAttrib[i].attrib
		idx := attrib.Subinterface
		path := gnmi.OC().Interface(portName)

		obj := &oc.Interface{}
		obj.Name = ygot.String(portName)
		ra := obj.GetOrCreateSubinterface(idx).GetOrCreateIpv6().
			GetOrCreateRouterAdvertisement()
		if RAIntervalDefault == 0 {
			getPath := gnmi.OC().Interface(portName).Subinterface(idx).Ipv6().RouterAdvertisement().State()
			op := gnmi.Get(t, dut, getPath)
			RALifetimeDefault = op.GetLifetime()
			RAIntervalDefault = op.GetInterval()
		}
		ra.SetInterval(RAInterval)
		ra.SetLifetime(RALifetime)
		ra.SetOtherConfig(RAOtherConfig)
		ra.SetSuppress(RASuppress)

		gnmi.BatchUpdate(batchConfig, path.Config(), configInterfaceIPv6DUT(obj, attrib))
	}
	batchConfig.Set(t, dut)
}

func configureNDPrefix(t *testing.T, dut *ondatra.DUTDevice) {

	batchConfig := &gnmi.SetBatch{}
	var dutIntfAttrib [4]InterfaceAttributes

	if dut.ID() == "dut1" {
		dutIntfAttrib = dut1IntfAttrib
	} else {
		dutIntfAttrib = dut2IntfAttrib
	}
	for i := 0; i < TOTAL_INTF_COUNT; i++ {
		portName := dutIntfAttrib[i].intfName
		attrib := dutIntfAttrib[i].attrib
		idx := attrib.Subinterface
		path := gnmi.OC().Interface(portName)

		obj := &oc.Interface{}
		obj.Name = ygot.String(portName)
		ndp := obj.GetOrCreateSubinterface(idx).GetOrCreateIpv6().
			GetOrCreateRouterAdvertisement().GetOrCreatePrefix(NDPrefix)
		ndp.SetPreferredLifetime(NDPrefixPreferredLifetime)
		ndp.SetValidLifetime(NDPrefixValidLifetime)
		ndp.SetDisableAutoconfiguration(NDPrefixDisableAutoconfiguration)
		ndp.SetEnableOnlink(NDPrefixEnableOnlink)

		gnmi.BatchUpdate(batchConfig, path.Config(), configInterfaceIPv6DUT(obj, attrib))
	}
	batchConfig.Set(t, dut)
}

func configureNDDad(t *testing.T, dut *ondatra.DUTDevice) {

	batchConfig := &gnmi.SetBatch{}
	var dutIntfAttrib [4]InterfaceAttributes

	if dut.ID() == "dut1" {
		dutIntfAttrib = dut1IntfAttrib
	} else {
		dutIntfAttrib = dut2IntfAttrib
	}
	for i := 0; i < TOTAL_INTF_COUNT; i++ {
		portName := dutIntfAttrib[i].intfName
		attrib := dutIntfAttrib[i].attrib
		idx := attrib.Subinterface
		path := gnmi.OC().Interface(portName)
		obj := &oc.Interface{}
		obj.Name = ygot.String(portName)

		obj.GetOrCreateSubinterface(idx).GetOrCreateIpv6().
			SetDupAddrDetectTransmits(NDDad)

		gnmi.BatchUpdate(batchConfig, path.Config(), configInterfaceIPv6DUT(obj, attrib))
	}
	batchConfig.Set(t, dut)
}

func configureScaleDUT(t *testing.T, dut *ondatra.DUTDevice, baseIPAddr []BaseIPAddress, IPv4 bool, wg *sync.WaitGroup) {

	defer wg.Done()

	var FourHundredGigELC0List []string
	var FourHundredGigELC1List []string
	var FourHundredGigELC2List []string
	var FourHundredGigELC3List []string
	var FourHundredGigELC4List []string
	var FourHundredGigELC5List []string
	var FourHundredGigELC6List []string
	var FourHundredGigELC7List []string
	var HundredGigELC0List []string
	var HundredGigELC1List []string
	var HundredGigELC2List []string
	var HundredGigELC3List []string
	var HundredGigELC4List []string
	var HundredGigELC5List []string
	var HundredGigELC6List []string
	var HundredGigELC7List []string

	// Interfaces := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State())
	// batchConfig := &gnmi.SetBatch{}

	// for _, intf := range Interfaces {
	// 	if findBreakoutPort(intf.GetName()) == true {
	// 		fmt.Printf("Debug: Breakout Interface Name :%s\n", intf.GetName())
	// 		cp, comp, bmp, bmode := doBreakoutPort(t, dut, intf.GetName())
	// 		gnmi.BatchReplace(batchConfig, cp.Config(), comp)
	// 		gnmi.BatchReplace(batchConfig, bmp.Config(), bmode)
	// 	}
	// }
	// batchConfig.Set(t, dut)

	Interfaces := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State())

	for _, intf := range Interfaces {
		if len(intf.GetName()) >= 18 && intf.GetName()[:18] == "FourHundredGigE0/0" {
			FourHundredGigELC0List = append(FourHundredGigELC0List, intf.GetName())
		} else if len(intf.GetName()) >= 18 && intf.GetName()[:18] == "FourHundredGigE0/1" {
			FourHundredGigELC1List = append(FourHundredGigELC1List, intf.GetName())
		} else if len(intf.GetName()) >= 18 && intf.GetName()[:18] == "FourHundredGigE0/2" {
			FourHundredGigELC2List = append(FourHundredGigELC2List, intf.GetName())
		} else if len(intf.GetName()) >= 18 && intf.GetName()[:18] == "FourHundredGigE0/3" {
			FourHundredGigELC3List = append(FourHundredGigELC3List, intf.GetName())
		} else if len(intf.GetName()) >= 18 && intf.GetName()[:18] == "FourHundredGigE0/4" {
			FourHundredGigELC4List = append(FourHundredGigELC4List, intf.GetName())
		} else if len(intf.GetName()) >= 18 && intf.GetName()[:18] == "FourHundredGigE0/5" {
			FourHundredGigELC5List = append(FourHundredGigELC5List, intf.GetName())
		} else if len(intf.GetName()) >= 18 && intf.GetName()[:18] == "FourHundredGigE0/6" {
			FourHundredGigELC6List = append(FourHundredGigELC6List, intf.GetName())
		} else if len(intf.GetName()) >= 18 && intf.GetName()[:18] == "FourHundredGigE0/7" {
			FourHundredGigELC7List = append(FourHundredGigELC7List, intf.GetName())
		} else if len(intf.GetName()) >= 14 && intf.GetName()[:14] == "HundredGigE0/0" {
			HundredGigELC0List = append(HundredGigELC0List, intf.GetName())
		} else if len(intf.GetName()) >= 14 && intf.GetName()[:14] == "HundredGigE0/1" {
			HundredGigELC1List = append(HundredGigELC1List, intf.GetName())
		} else if len(intf.GetName()) >= 14 && intf.GetName()[:14] == "HundredGigE0/2" {
			HundredGigELC2List = append(HundredGigELC2List, intf.GetName())
		} else if len(intf.GetName()) >= 14 && intf.GetName()[:14] == "HundredGigE0/3" {
			HundredGigELC3List = append(HundredGigELC3List, intf.GetName())
		} else if len(intf.GetName()) >= 14 && intf.GetName()[:14] == "HundredGigE0/4" {
			HundredGigELC4List = append(HundredGigELC4List, intf.GetName())
		} else if len(intf.GetName()) >= 14 && intf.GetName()[:14] == "HundredGigE0/5" {
			HundredGigELC5List = append(HundredGigELC5List, intf.GetName())
		} else if len(intf.GetName()) >= 14 && intf.GetName()[:14] == "HundredGigE0/6" {
			HundredGigELC6List = append(HundredGigELC6List, intf.GetName())
		} else if len(intf.GetName()) >= 14 && intf.GetName()[:14] == "HundredGigE0/7" {
			HundredGigELC7List = append(HundredGigELC7List, intf.GetName())
		}
	}
	sort.SliceStable(FourHundredGigELC0List, func(i, j int) bool {
		return len(FourHundredGigELC0List[i]) < len(FourHundredGigELC0List[j])
	})
	sort.SliceStable(FourHundredGigELC1List, func(i, j int) bool {
		return len(FourHundredGigELC1List[i]) < len(FourHundredGigELC1List[j])
	})
	sort.SliceStable(FourHundredGigELC2List, func(i, j int) bool {
		return len(FourHundredGigELC2List[i]) < len(FourHundredGigELC2List[j])
	})
	sort.SliceStable(FourHundredGigELC3List, func(i, j int) bool {
		return len(FourHundredGigELC3List[i]) < len(FourHundredGigELC3List[j])
	})
	sort.SliceStable(FourHundredGigELC4List, func(i, j int) bool {
		return len(FourHundredGigELC4List[i]) < len(FourHundredGigELC4List[j])
	})
	sort.SliceStable(FourHundredGigELC5List, func(i, j int) bool {
		return len(FourHundredGigELC5List[i]) < len(FourHundredGigELC5List[j])
	})
	sort.SliceStable(FourHundredGigELC6List, func(i, j int) bool {
		return len(FourHundredGigELC6List[i]) < len(FourHundredGigELC6List[j])
	})
	sort.SliceStable(FourHundredGigELC7List, func(i, j int) bool {
		return len(FourHundredGigELC7List[i]) < len(FourHundredGigELC7List[j])
	})

	var FourHundredGigEList []InterfaceLCList
	// var HundredGigEList []InterfaceLCList

	FourHundredGigEList = []InterfaceLCList{{FourHundredGigELC0List, FourHundredGigELC1List}, {FourHundredGigELC2List, FourHundredGigELC3List},
		{FourHundredGigELC4List, FourHundredGigELC5List}, {FourHundredGigELC6List, FourHundredGigELC7List}}
	// HundredGigEList = []InterfaceLCList{{HundredGigELC0List, HundredGigELC1List}, {HundredGigELC2List, HundredGigELC3List},
	// 	{HundredGigELC4List, HundredGigELC5List}, {HundredGigELC6List, HundredGigELC7List}}

	if IPv4 == true {
		for i := 0; i < MAX_LC_COUNT/2; i++ {
			configInterfaceIPv4PhyScale(t, FourHundredGigEList[i], baseIPAddr[i], dut)
			configInterfaceIPv4PhySubScale(t, FourHundredGigEList[i], baseIPAddr[i], dut)
			configInterfaceIPv4BundleScale(t, FourHundredGigEList[i], i, baseIPAddr[i], dut)
			configInterfaceIPv4BundleSubScale(t, FourHundredGigEList[i], i, baseIPAddr[i], dut)
		}
	} else {
		for i := 0; i < MAX_LC_COUNT/2; i++ {
			// configInterfaceIPv6PhyScale(t, FourHundredGigEList[i], baseIPAddr[i], dut)
			// configInterfaceIPv6PhySubScale(t, FourHundredGigEList[i], baseIPAddr[i], dut)
			configInterfaceIPv6BundleScale(t, FourHundredGigEList[i], i, baseIPAddr[i], dut)
			// configInterfaceIPv6BundleSubScale(t, FourHundredGigEList[i], i, baseIPAddr[i], dut)
		}
	}
}

func findBreakoutPort(intf string) bool {

	breakoutPortList := [8]string{"0/28", "0/29", "0/30", "0/31", "0/32", "0/33", "0/34", "0/35"}

	for _, bIntf := range breakoutPortList {
		if strings.HasSuffix(intf, bIntf) == true {
			return true
		}
	}
	return false
}
func doBreakoutPort(t *testing.T, dut *ondatra.DUTDevice, intf string) (*platform.ComponentPath,
	*oc.Component, *platform.Component_Port_BreakoutModePath, *oc.Component_Port_BreakoutMode) {

	intfHW := gnmi.Get(t, dut, gnmi.OC().Interface(intf).HardwarePort().State())
	bmode := &oc.Component_Port_BreakoutMode{}
	bmp := gnmi.OC().Component(intfHW).Port().BreakoutMode()
	cp := gnmi.OC().Component(intfHW)
	group := bmode.GetOrCreateGroup(0)
	group.BreakoutSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB
	group.NumBreakouts = ygot.Uint8(4)
	comp := &oc.Component{Name: ygot.String(intfHW)}

	return cp, comp, bmp, bmode
}

func configInterfaceIPv4PhyScale(t *testing.T, FourHundredGigEList InterfaceLCList, baseIP BaseIPAddress,
	dut *ondatra.DUTDevice) {

	DUT_LC1_LC2_BASE_IP_COPY := baseIP.dutBaseIP[0]
	batchConfig := &gnmi.SetBatch{}

	for j := 0; j < (MAX_PHY_INTF); j++ {
		intfLC1 := FourHundredGigEList.interfaceLC1
		intfLC2 := FourHundredGigEList.interfaceLC2
		// fmt.Printf("Debug: DUT:%v InterfaceLC1: %s  InterfaceLC2: %s\n", dut.ID(), intfLC1[j], intfLC2[j])
		pathLC1 := gnmi.OC().Interface(intfLC1[j])
		pathLC2 := gnmi.OC().Interface(intfLC2[j])
		intfNameLC1 := &oc.Interface{Name: ygot.String(intfLC1[j])}
		intfNameLC2 := &oc.Interface{Name: ygot.String(intfLC2[j])}
		var dutScalePort = &attrs.Attributes{}

		dutScalePort.Desc = dut.ID() + "Port" + strconv.Itoa(j)
		dutScalePort.IPv4Len = ipv4PrefixLen
		dutScalePort.IPv4 = DUT_LC1_LC2_BASE_IP_COPY
		dutScalePort.Subinterface = 0

		DUT_LC1_LC2_BASE_IP_COPY = getScaleNewIPv4(DUT_LC1_LC2_BASE_IP_COPY)
		objIPv4 := configInterfaceIPv4DUT(intfNameLC1, dutScalePort)
		gnmi.BatchReplace(batchConfig, pathLC1.Config(), objIPv4)
		pathLC1, objStaticARP := configInterfaceStaticARPScale(objIPv4, dutScalePort)
		gnmi.BatchReplace(batchConfig, pathLC1.Config(), objStaticARP)
		pathLC1, objProxyARP := configInterfaceProxyARPScale(objIPv4, dutScalePort.Subinterface)
		gnmi.BatchReplace(batchConfig, pathLC1.Config(), objProxyARP)

		dutScalePort.IPv4 = DUT_LC1_LC2_BASE_IP_COPY
		dutScalePort.Subinterface = 0

		DUT_LC1_LC2_BASE_IP_COPY = getScaleNewIPv4(DUT_LC1_LC2_BASE_IP_COPY)
		objIPv4 = configInterfaceIPv4DUT(intfNameLC2, dutScalePort)
		gnmi.BatchReplace(batchConfig, pathLC2.Config(), objIPv4)
		pathLC2, objStaticARP = configInterfaceStaticARPScale(objIPv4, dutScalePort)
		gnmi.BatchReplace(batchConfig, pathLC2.Config(), objStaticARP)
		pathLC1, objProxyARP = configInterfaceProxyARPScale(objIPv4, dutScalePort.Subinterface)
		gnmi.BatchReplace(batchConfig, pathLC2.Config(), objProxyARP)
	}
	batchConfig.Set(t, dut)
}

func configInterfaceIPv4PhySubScale(t *testing.T, FourHundredGigEList InterfaceLCList, baseIP BaseIPAddress,
	dut *ondatra.DUTDevice) {

	DUT_LC1_LC2_BASE_IP_COPY := baseIP.dutBaseIP[1]
	intfLC1 := FourHundredGigEList.interfaceLC1
	intfLC2 := FourHundredGigEList.interfaceLC2
	batchConfig := &gnmi.SetBatch{}
	var scaleParam ScaleParam

	for j := MAX_PHY_INTF; j < (MAX_PHY_INTF * 2); j++ {

		pathLC1 := gnmi.OC().Interface(intfLC1[j])
		pathLC2 := gnmi.OC().Interface(intfLC2[j])

		gnmi.BatchDelete(batchConfig, pathLC1.Config())
		gnmi.BatchDelete(batchConfig, pathLC2.Config())
		for k := 0; k < MAX_SUB_INTF; k++ {
			// fmt.Printf("Debug: DUT:%v InterfaceSubLC1: %s  InterfaceSubLC2: %s\n", dut.ID(), intfLC1[j], intfLC2[j])
			intfNameLC1 := &oc.Interface{Name: ygot.String(intfLC1[j])}
			intfNameLC2 := &oc.Interface{Name: ygot.String(intfLC2[j])}
			var dutScalePort = &attrs.Attributes{}

			dutScalePort.Desc = dut.ID() + "Port" + strconv.Itoa(j)
			dutScalePort.IPv4Len = ipv4PrefixLen
			dutScalePort.IPv4 = DUT_LC1_LC2_BASE_IP_COPY
			dutScalePort.Subinterface = uint32(1 + k)
			scaleParam.vlanID = uint16(vlanID + k)

			DUT_LC1_LC2_BASE_IP_COPY = getScaleNewIPv4(DUT_LC1_LC2_BASE_IP_COPY)
			objIPv4 := configInterfaceIPv4DUT(intfNameLC1, dutScalePort, scaleParam)
			gnmi.BatchUpdate(batchConfig, pathLC1.Config(), objIPv4)
			pathLC1, objStaticARP := configInterfaceStaticARPScale(objIPv4, dutScalePort, scaleParam.vlanID)
			gnmi.BatchUpdate(batchConfig, pathLC1.Config(), objStaticARP)
			pathLC1, objProxyARP := configInterfaceProxyARPScale(objIPv4, dutScalePort.Subinterface)
			gnmi.BatchUpdate(batchConfig, pathLC1.Config(), objProxyARP)

			dutScalePort.IPv4 = DUT_LC1_LC2_BASE_IP_COPY
			dutScalePort.Subinterface = uint32(1 + k)
			scaleParam.vlanID = uint16(vlanID + k)

			DUT_LC1_LC2_BASE_IP_COPY = getScaleNewIPv4(DUT_LC1_LC2_BASE_IP_COPY)
			objIPv4 = configInterfaceIPv4DUT(intfNameLC2, dutScalePort, scaleParam)
			gnmi.BatchUpdate(batchConfig, pathLC2.Config(), objIPv4)
			pathLC2, objStaticARP = configInterfaceStaticARPScale(objIPv4, dutScalePort, scaleParam.vlanID)
			gnmi.BatchUpdate(batchConfig, pathLC2.Config(), objStaticARP)
			pathLC2, objProxyARP := configInterfaceProxyARPScale(objIPv4, dutScalePort.Subinterface)
			gnmi.BatchUpdate(batchConfig, pathLC2.Config(), objProxyARP)
		}
	}
	batchConfig.Set(t, dut)
}

func configInterfaceIPv4BundleScale(t *testing.T, HundredGigEList InterfaceLCList, i int,
	baseIP BaseIPAddress, dut *ondatra.DUTDevice) {

	DUT_LC1_LC2_BASE_IP_COPY := baseIP.dutBaseIP[2]
	intfLC1 := HundredGigEList.interfaceLC1
	intfLC2 := HundredGigEList.interfaceLC2
	mpIdx := MAX_PHY_INTF * 2
	bundleIntf := "Bundle-Ether2" + strconv.Itoa(i)
	batchConfig := &gnmi.SetBatch{}

	for j := 0; j < (MAX_BUNDLE_INTF); j++ {
		BI := bundleIntf + strconv.Itoa(j)
		// fmt.Printf("Debug: DUT: %v Bundle :%s\n", dut.ID(), BI)
		// fmt.Printf("Debug: Memb1 :%s Memb2: %s\n", intfLC1[mpIdx+j], intfLC2[mpIdx+j])
		pathb := gnmi.OC().Interface(BI)
		pathm1 := gnmi.OC().Interface(intfLC1[mpIdx+j])
		pathm2 := gnmi.OC().Interface(intfLC2[mpIdx+j])
		intf := &oc.Interface{Name: ygot.String(BI)}
		var dutScalePort = &attrs.Attributes{}

		bundleArray := BundleMemberPorts{
			BundleName:  BI,
			MemberPorts: [2]string{intfLC1[mpIdx+j], intfLC2[mpIdx+j]},
		}
		mapBundleMemberPorts[dut.ID()] = append(mapBundleMemberPorts[dut.ID()], bundleArray)

		dutScalePort.Desc = dut.ID() + "Port" + strconv.Itoa(j)
		dutScalePort.IPv4Len = ipv4PrefixLen
		dutScalePort.IPv4 = DUT_LC1_LC2_BASE_IP_COPY
		dutScalePort.Subinterface = 0

		DUT_LC1_LC2_BASE_IP_COPY = getScaleNewIPv4(DUT_LC1_LC2_BASE_IP_COPY)
		objIPv4 := configInterfaceIPv4DUT(intf, dutScalePort)
		gnmi.BatchReplace(batchConfig, pathb.Config(), objIPv4)
		BE1 := generateBundleMemberInterfaceConfig(intfLC1[mpIdx+j], BI)
		gnmi.BatchReplace(batchConfig, pathm1.Config(), BE1)
		BE2 := generateBundleMemberInterfaceConfig(intfLC2[mpIdx+j], BI)
		gnmi.BatchReplace(batchConfig, pathm2.Config(), BE2)
		pathb, objStaticARP := configInterfaceStaticARPScale(objIPv4, dutScalePort)
		gnmi.BatchReplace(batchConfig, pathb.Config(), objStaticARP)
		pathb, objProxyARP := configInterfaceProxyARPScale(objIPv4, dutScalePort.Subinterface)
		gnmi.BatchReplace(batchConfig, pathb.Config(), objProxyARP)
	}
	batchConfig.Set(t, dut)
}

func configInterfaceIPv4BundleSubScale(t *testing.T, HundredGigEList InterfaceLCList, i int,
	baseIP BaseIPAddress, dut *ondatra.DUTDevice) {

	DUT_LC1_LC2_BASE_IP_COPY := baseIP.dutBaseIP[3]
	intfLC1 := HundredGigEList.interfaceLC1
	intfLC2 := HundredGigEList.interfaceLC2
	bundleIntf := "Bundle-Ether3" + strconv.Itoa(i)
	mpIdx := (MAX_PHY_INTF * 2) + MAX_BUNDLE_INTF
	batchConfig := &gnmi.SetBatch{}
	var scaleParam ScaleParam

	for j := 0; j < (MAX_BUNDLE_INTF); j++ {
		BI := bundleIntf + strconv.Itoa(j)
		pathb := gnmi.OC().Interface(BI)
		pathm1 := gnmi.OC().Interface(intfLC1[mpIdx+j])
		pathm2 := gnmi.OC().Interface(intfLC2[mpIdx+j])

		bundleArray := BundleMemberPorts{
			BundleName:  BI,
			MemberPorts: [2]string{intfLC1[mpIdx+j], intfLC2[mpIdx+j]},
		}
		mapBundleMemberPorts[dut.ID()] = append(mapBundleMemberPorts[dut.ID()], bundleArray)

		gnmi.BatchDelete(batchConfig, pathb.Config())
		gnmi.BatchDelete(batchConfig, pathm1.Config())
		gnmi.BatchDelete(batchConfig, pathm2.Config())
		for k := 0; k < MAX_SUB_INTF; k++ {
			// fmt.Printf("Debug: DUT: %v Bundle :%s\n", dut.ID(), BI)
			// fmt.Printf("Debug: Memb1 :%s Memb2: %s\n", intfLC1[mpIdx+j], intfLC2[mpIdx+j])
			intf := &oc.Interface{Name: ygot.String(BI)}
			var dutScalePort = &attrs.Attributes{}

			dutScalePort.Desc = dut.ID() + "Port" + strconv.Itoa(j)
			dutScalePort.IPv4Len = ipv4PrefixLen
			dutScalePort.IPv4 = DUT_LC1_LC2_BASE_IP_COPY
			dutScalePort.Subinterface = uint32(1 + k)
			scaleParam.vlanID = uint16(vlanID + k)

			DUT_LC1_LC2_BASE_IP_COPY = getScaleNewIPv4(DUT_LC1_LC2_BASE_IP_COPY)
			objIPv4 := configInterfaceIPv4DUT(intf, dutScalePort, scaleParam)
			gnmi.BatchUpdate(batchConfig, pathb.Config(), objIPv4)
			BE1 := generateBundleMemberInterfaceConfig(intfLC1[mpIdx+j], BI)
			gnmi.BatchUpdate(batchConfig, pathm1.Config(), BE1)
			BE2 := generateBundleMemberInterfaceConfig(intfLC2[mpIdx+j], BI)
			gnmi.BatchUpdate(batchConfig, pathm2.Config(), BE2)
			pathb, objStaticARP := configInterfaceStaticARPScale(objIPv4, dutScalePort, scaleParam.vlanID)
			gnmi.BatchUpdate(batchConfig, pathb.Config(), objStaticARP)
			pathb, objProxyARP := configInterfaceProxyARPScale(objIPv4, dutScalePort.Subinterface)
			gnmi.BatchUpdate(batchConfig, pathb.Config(), objProxyARP)
		}
	}
	batchConfig.Set(t, dut)
}

func configInterfaceProxyARPScale(obj *oc.Interface, subIntf uint32) (*interfaces.InterfacePath,
	*oc.Interface) {

	path := gnmi.OC().Interface(obj.GetName())

	obj.GetOrCreateSubinterface(subIntf).GetOrCreateIpv4().
		GetOrCreateProxyArp().SetMode(oc.ProxyArp_Mode_ALL)

	return path, obj
}

func configInterfaceStaticARPScale(obj *oc.Interface, a *attrs.Attributes,
	vlanID ...uint16) (*interfaces.InterfacePath, *oc.Interface) {

	var scaleParam ScaleParam
	path := gnmi.OC().Interface(obj.GetName())

	obj.GetOrCreateEthernet()
	scaleParam.staticIP = getNewStaticIPv4(a.IPv4)
	if len(vlanID) > 0 {
		scaleParam.vlanID = vlanID[0]
	}
	objs := configInterfaceIPv4DUT(obj, a, scaleParam)

	return path, objs
}

func configInterfaceIPv6PhyScale(t *testing.T, FourHundredGigEList InterfaceLCList, baseIP BaseIPAddress,
	dut *ondatra.DUTDevice) {

	DUT_LC1_LC2_BASE_IP_COPY := baseIP.dutBaseIP[0]
	intfLC1 := FourHundredGigEList.interfaceLC1
	intfLC2 := FourHundredGigEList.interfaceLC2
	batchConfig := &gnmi.SetBatch{}

	for j := 0; j < (MAX_PHY_INTF); j++ {
		// fmt.Printf("Debug: DUT:%v InterfaceLC1: %s InterfaceLC2:%s\n", dut.ID(), intfLC1[j], intfLC2[j])
		pathLC1 := gnmi.OC().Interface(intfLC1[j])
		pathLC2 := gnmi.OC().Interface(intfLC2[j])
		intfNameLC1 := &oc.Interface{Name: ygot.String(intfLC1[j])}
		intfNameLC2 := &oc.Interface{Name: ygot.String(intfLC2[j])}
		var dutScalePort = &attrs.Attributes{}

		dutScalePort.Desc = dut.ID() + "Port" + strconv.Itoa(j)
		dutScalePort.IPv6Len = ipv6PrefixLen
		dutScalePort.IPv6 = DUT_LC1_LC2_BASE_IP_COPY
		dutScalePort.Subinterface = 0

		DUT_LC1_LC2_BASE_IP_COPY = getScaleNewIPv6(DUT_LC1_LC2_BASE_IP_COPY)
		objIPv6 := configInterfaceIPv6DUT(intfNameLC1, dutScalePort)
		gnmi.BatchReplace(batchConfig, pathLC1.Config(), objIPv6)
		pathLC1, objstaticND := configInterfaceNDStaticScale(objIPv6, dutScalePort)
		gnmi.BatchReplace(batchConfig, pathLC1.Config(), objstaticND)
		pathLC1, objRadv := configInterfaceNDRouterAdvScale(objIPv6, dutScalePort.Subinterface)
		gnmi.BatchReplace(batchConfig, pathLC1.Config(), objRadv)
		pathLC1, objPfx := configInterfaceNDPrefixScale(objIPv6, dutScalePort.Subinterface)
		gnmi.BatchReplace(batchConfig, pathLC1.Config(), objPfx)
		pathLC1, objDad := configInterfaceNDDadScale(objIPv6, dutScalePort.Subinterface)
		gnmi.BatchReplace(batchConfig, pathLC1.Config(), objDad)

		dutScalePort.IPv6 = DUT_LC1_LC2_BASE_IP_COPY
		dutScalePort.Subinterface = 0

		DUT_LC1_LC2_BASE_IP_COPY = getScaleNewIPv6(DUT_LC1_LC2_BASE_IP_COPY)
		objIPv6 = configInterfaceIPv6DUT(intfNameLC2, dutScalePort)
		gnmi.BatchReplace(batchConfig, pathLC2.Config(), objIPv6)
		pathLC2, objstaticND = configInterfaceNDStaticScale(objIPv6, dutScalePort)
		gnmi.BatchReplace(batchConfig, pathLC2.Config(), objstaticND)
		pathLC2, objRadv = configInterfaceNDRouterAdvScale(objIPv6, dutScalePort.Subinterface)
		gnmi.BatchReplace(batchConfig, pathLC2.Config(), objRadv)
		pathLC2, objPfx = configInterfaceNDPrefixScale(objIPv6, dutScalePort.Subinterface)
		gnmi.BatchReplace(batchConfig, pathLC2.Config(), objPfx)
		pathLC2, objDad = configInterfaceNDDadScale(objIPv6, dutScalePort.Subinterface)
		gnmi.BatchReplace(batchConfig, pathLC2.Config(), objDad)
	}
	batchConfig.Set(t, dut)
}

func configInterfaceIPv6PhySubScale(t *testing.T, FourHundredGigEList InterfaceLCList, baseIP BaseIPAddress,
	dut *ondatra.DUTDevice) {

	DUT_LC1_LC2_BASE_IP_COPY := baseIP.dutBaseIP[1]
	intfLC1 := FourHundredGigEList.interfaceLC1
	intfLC2 := FourHundredGigEList.interfaceLC2
	batchConfig := &gnmi.SetBatch{}
	var scaleParam ScaleParam

	for j := MAX_PHY_INTF; j < (MAX_PHY_INTF * 2); j++ {
		pathLC1 := gnmi.OC().Interface(intfLC1[j])
		pathLC2 := gnmi.OC().Interface(intfLC2[j])

		gnmi.BatchDelete(batchConfig, pathLC1.Config())
		gnmi.BatchDelete(batchConfig, pathLC2.Config())
		for k := 0; k < MAX_SUB_INTF; k++ {
			// fmt.Printf("Debug: DUT:%v InterfaceSub: %s InterfaceSub:%s\n", dut.ID(), intfLC1[j], intfLC2[j])
			intfNameLC1 := &oc.Interface{Name: ygot.String(intfLC1[j])}
			intfNameLC2 := &oc.Interface{Name: ygot.String(intfLC2[j])}
			var dutScalePort = &attrs.Attributes{}

			dutScalePort.Desc = dut.ID() + "Port" + strconv.Itoa(j)
			dutScalePort.IPv6Len = ipv6PrefixLen
			dutScalePort.IPv6 = DUT_LC1_LC2_BASE_IP_COPY
			dutScalePort.Subinterface = uint32(1 + k)
			scaleParam.vlanID = uint16(vlanID + k)

			DUT_LC1_LC2_BASE_IP_COPY = getScaleNewIPv6(DUT_LC1_LC2_BASE_IP_COPY)
			objIPv6 := configInterfaceIPv6DUT(intfNameLC1, dutScalePort, scaleParam)
			pathLC1, objStaticND := configInterfaceNDStaticScale(objIPv6, dutScalePort, scaleParam.vlanID)
			gnmi.BatchUpdate(batchConfig, pathLC1.Config(), objStaticND)
			pathLC1, objRadv := configInterfaceNDRouterAdvScale(objIPv6, dutScalePort.Subinterface)
			gnmi.BatchUpdate(batchConfig, pathLC1.Config(), objRadv)
			pathLC1, objPfx := configInterfaceNDPrefixScale(objIPv6, dutScalePort.Subinterface)
			gnmi.BatchUpdate(batchConfig, pathLC1.Config(), objPfx)
			pathLC1, objDad := configInterfaceNDDadScale(objIPv6, dutScalePort.Subinterface)
			gnmi.BatchUpdate(batchConfig, pathLC1.Config(), objDad)

			dutScalePort.IPv6 = DUT_LC1_LC2_BASE_IP_COPY
			dutScalePort.Subinterface = uint32(1 + k)
			scaleParam.vlanID = uint16(vlanID + k)

			DUT_LC1_LC2_BASE_IP_COPY = getScaleNewIPv6(DUT_LC1_LC2_BASE_IP_COPY)
			objIPv6 = configInterfaceIPv6DUT(intfNameLC2, dutScalePort, scaleParam)
			pathLC2, objStaticND = configInterfaceNDStaticScale(objIPv6, dutScalePort, scaleParam.vlanID)
			gnmi.BatchUpdate(batchConfig, pathLC2.Config(), objStaticND)
			pathLC2, objRadv = configInterfaceNDRouterAdvScale(objIPv6, dutScalePort.Subinterface)
			gnmi.BatchUpdate(batchConfig, pathLC2.Config(), objRadv)
			pathLC2, objPfx = configInterfaceNDPrefixScale(objIPv6, dutScalePort.Subinterface)
			gnmi.BatchUpdate(batchConfig, pathLC2.Config(), objPfx)
			pathLC2, objDad = configInterfaceNDDadScale(objIPv6, dutScalePort.Subinterface)
			gnmi.BatchUpdate(batchConfig, pathLC2.Config(), objDad)
		}
	}
	batchConfig.Set(t, dut)
}

func configInterfaceIPv6BundleScale(t *testing.T, HundredGigEList InterfaceLCList, i int,
	baseIP BaseIPAddress, dut *ondatra.DUTDevice) {

	DUT_LC1_LC2_BASE_IP_COPY := baseIP.dutBaseIP[2]
	intfLC1 := HundredGigEList.interfaceLC1
	intfLC2 := HundredGigEList.interfaceLC2
	bundleIntf := "Bundle-Ether2" + strconv.Itoa(i)
	mpIdx := MAX_PHY_INTF * 2
	batchConfig := &gnmi.SetBatch{}

	for j := 0; j < (MAX_BUNDLE_INTF); j++ {
		BI := bundleIntf + strconv.Itoa(j)
		// fmt.Printf("Debug: DUT: %v Bundle :%s\n", dut.ID(), BI)
		// fmt.Printf("Debug: Memb1 :%s Memb2: %s\n", intfLC1[mpIdx+j], intfLC2[mpIdx+j])
		pathb := gnmi.OC().Interface(BI)
		pathm1 := gnmi.OC().Interface(intfLC1[mpIdx+j])
		pathm2 := gnmi.OC().Interface(intfLC2[mpIdx+j])
		intf := &oc.Interface{Name: ygot.String(BI)}
		var dutScalePort = &attrs.Attributes{}

		bundleArray := BundleMemberPorts{
			BundleName:  BI,
			MemberPorts: [2]string{intfLC1[mpIdx+j], intfLC2[mpIdx+j]},
		}
		mapBundleMemberPorts[dut.ID()] = append(mapBundleMemberPorts[dut.ID()], bundleArray)

		gnmi.BatchDelete(batchConfig, pathb.Config())
		gnmi.BatchDelete(batchConfig, pathm1.Config())
		gnmi.BatchDelete(batchConfig, pathm2.Config())

		dutScalePort.Desc = dut.ID() + "Port" + strconv.Itoa(j)
		dutScalePort.IPv6Len = ipv6PrefixLen
		dutScalePort.IPv6 = DUT_LC1_LC2_BASE_IP_COPY
		dutScalePort.Subinterface = 0

		DUT_LC1_LC2_BASE_IP_COPY = getScaleNewIPv6(DUT_LC1_LC2_BASE_IP_COPY)
		objIPv6 := configInterfaceIPv6DUT(intf, dutScalePort)
		gnmi.BatchReplace(batchConfig, pathb.Config(), objIPv6)
		BE1 := generateBundleMemberInterfaceConfig(intfLC1[mpIdx+j], BI)
		gnmi.BatchReplace(batchConfig, pathm1.Config(), BE1)
		BE2 := generateBundleMemberInterfaceConfig(intfLC2[mpIdx+j], BI)
		gnmi.BatchReplace(batchConfig, pathm2.Config(), BE2)
		pathb, objStaticND := configInterfaceNDStaticScale(objIPv6, dutScalePort)
		gnmi.BatchReplace(batchConfig, pathb.Config(), objStaticND)
		pathb, objRadv := configInterfaceNDRouterAdvScale(objIPv6, dutScalePort.Subinterface)
		gnmi.BatchReplace(batchConfig, pathb.Config(), objRadv)
		pathb, objPfx := configInterfaceNDPrefixScale(objIPv6, dutScalePort.Subinterface)
		gnmi.BatchReplace(batchConfig, pathb.Config(), objPfx)
		pathb, objDad := configInterfaceNDDadScale(objIPv6, dutScalePort.Subinterface)
		gnmi.BatchReplace(batchConfig, pathb.Config(), objDad)
	}
	batchConfig.Set(t, dut)
}

func configInterfaceIPv6BundleSubScale(t *testing.T, HundredGigEList InterfaceLCList, i int,
	baseIP BaseIPAddress, dut *ondatra.DUTDevice) {

	DUT_LC1_LC2_BASE_IP_COPY := baseIP.dutBaseIP[3]
	intfLC1 := HundredGigEList.interfaceLC1
	intfLC2 := HundredGigEList.interfaceLC2
	bundleIntf := "Bundle-Ether3" + strconv.Itoa(i)
	mpIdx := (MAX_PHY_INTF * 2) + MAX_BUNDLE_INTF
	batchConfig := &gnmi.SetBatch{}
	var scaleParam ScaleParam

	for j := 0; j < (MAX_BUNDLE_INTF); j++ {
		BI := bundleIntf + strconv.Itoa(j)
		pathb := gnmi.OC().Interface(BI)
		pathm1 := gnmi.OC().Interface(intfLC1[mpIdx+j])
		pathm2 := gnmi.OC().Interface(intfLC2[mpIdx+j])

		gnmi.BatchDelete(batchConfig, pathb.Config())
		gnmi.BatchDelete(batchConfig, pathm1.Config())
		gnmi.BatchDelete(batchConfig, pathm2.Config())

		bundleArray := BundleMemberPorts{
			BundleName:  BI,
			MemberPorts: [2]string{intfLC1[mpIdx+j], intfLC2[mpIdx+j]},
		}
		mapBundleMemberPorts[dut.ID()] = append(mapBundleMemberPorts[dut.ID()], bundleArray)

		for k := 0; k < MAX_SUB_INTF; k++ {
			// fmt.Printf("Debug: DUT: %v Bundle :%s\n", dut.ID(), BI)
			// fmt.Printf("Debug: Memb1 :%s Memb2: %s\n", intfLC1[mpIdx+j], intfLC2[mpIdx+j])
			intf := &oc.Interface{Name: ygot.String(BI)}
			var dutScalePort = &attrs.Attributes{}

			dutScalePort.Desc = dut.ID() + "Port" + strconv.Itoa(j)
			dutScalePort.IPv6Len = ipv6PrefixLen
			dutScalePort.IPv6 = DUT_LC1_LC2_BASE_IP_COPY
			dutScalePort.Subinterface = uint32(1 + k)
			scaleParam.vlanID = uint16(vlanID + k)

			DUT_LC1_LC2_BASE_IP_COPY = getScaleNewIPv6(DUT_LC1_LC2_BASE_IP_COPY)
			objIPv6 := configInterfaceIPv6DUT(intf, dutScalePort, scaleParam)
			gnmi.BatchUpdate(batchConfig, pathb.Config(), objIPv6)
			BE1 := generateBundleMemberInterfaceConfig(intfLC1[mpIdx+j], BI)
			gnmi.BatchUpdate(batchConfig, pathm1.Config(), BE1)
			BE2 := generateBundleMemberInterfaceConfig(intfLC2[mpIdx+j], BI)
			gnmi.BatchUpdate(batchConfig, pathm2.Config(), BE2)
			pathb, objStaticND := configInterfaceNDStaticScale(objIPv6, dutScalePort, scaleParam.vlanID)
			gnmi.BatchUpdate(batchConfig, pathb.Config(), objStaticND)
			pathb, objRadv := configInterfaceNDRouterAdvScale(objIPv6, dutScalePort.Subinterface)
			gnmi.BatchUpdate(batchConfig, pathb.Config(), objRadv)
			pathb, objPfx := configInterfaceNDPrefixScale(objIPv6, dutScalePort.Subinterface)
			gnmi.BatchUpdate(batchConfig, pathb.Config(), objPfx)
			pathb, objDad := configInterfaceNDDadScale(objIPv6, dutScalePort.Subinterface)
			gnmi.BatchUpdate(batchConfig, pathb.Config(), objDad)
		}
	}
	batchConfig.Set(t, dut)
}

func configInterfaceNDStaticScale(obj *oc.Interface, a *attrs.Attributes,
	vlanID ...uint16) (*interfaces.InterfacePath, *oc.Interface) {

	var scaleParam ScaleParam
	path := gnmi.OC().Interface(obj.GetName())

	obj.GetOrCreateEthernet()
	scaleParam.staticIP = getNewStaticIPv6(a.IPv6)
	if len(vlanID) > 0 {
		scaleParam.vlanID = vlanID[0]
	}
	objs := configInterfaceIPv6DUT(obj, a, scaleParam)

	return path, objs
}

func configInterfaceNDRouterAdvScale(obj *oc.Interface, subIntf uint32) (*interfaces.InterfacePath,
	*oc.Interface) {

	path := gnmi.OC().Interface(obj.GetName())
	ra := obj.GetOrCreateSubinterface(subIntf).GetOrCreateIpv6().
		GetOrCreateRouterAdvertisement()
	ra.SetInterval(RAInterval)
	ra.SetLifetime(RALifetime)
	ra.SetOtherConfig(RAOtherConfig)
	ra.SetSuppress(RASuppress)

	return path, obj
}

func configInterfaceNDPrefixScale(obj *oc.Interface, subIntf uint32) (*interfaces.InterfacePath,
	*oc.Interface) {

	path := gnmi.OC().Interface(obj.GetName())
	ndp := obj.GetOrCreateSubinterface(subIntf).GetOrCreateIpv6().
		GetOrCreateRouterAdvertisement().GetOrCreatePrefix(NDPrefix)
	ndp.SetPreferredLifetime(NDPrefixPreferredLifetime)
	ndp.SetValidLifetime(NDPrefixValidLifetime)
	ndp.SetDisableAutoconfiguration(NDPrefixDisableAutoconfiguration)
	ndp.SetEnableOnlink(NDPrefixEnableOnlink)

	return path, obj
}

func configInterfaceNDDadScale(obj *oc.Interface, subIntf uint32) (*interfaces.InterfacePath,
	*oc.Interface) {

	path := gnmi.OC().Interface(obj.GetName())
	obj.GetOrCreateSubinterface(subIntf).GetOrCreateIpv6().
		SetDupAddrDetectTransmits(NDDad)

	return path, obj
}

func generateBundleMemberInterfaceConfig(name, bundleID string) *oc.Interface {

	i := &oc.Interface{Name: ygot.String(name)}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	e := i.GetOrCreateEthernet()
	e.AutoNegotiate = ygot.Bool(false)
	e.AggregateId = ygot.String(bundleID)

	return i
}

func getNewIPv4(ip string) string {

	newIP := net.ParseIP(ip)
	newIP = newIP.To4()
	newIP[3] += 2

	return newIP.String()
}

func getScaleNewIPv4(ip string) string {

	newIP := net.ParseIP(ip)
	newIP = newIP.To4()

	if newIP[2]+1 > 254 {
		newIP[1] += 1
		newIP[2] = 1
	} else {
		newIP[2] += 1
	}

	return newIP.String()
}

func getNewStaticIPv4(ip string) string {

	newIP := net.ParseIP(ip)
	newIP = newIP.To4()
	newIP[3] += 10

	return newIP.String()
}

func getNewIPv6(ip string) string {

	newIP := net.ParseIP(ip)
	newIP = newIP.To16()
	newIP[15] += 2

	return newIP.String()
}

func getScaleNewIPv6(ip string) string {

	newIP := net.ParseIP(ip)
	newIP = newIP.To16()

	if newIP[5]+1 > 254 {
		newIP[4] += 1
		newIP[5] = 1
	} else {
		newIP[5] += 1
	}

	return newIP.String()
}
func getNewStaticIPv6(ip string) string {

	newIP := net.ParseIP(ip)
	newIP = newIP.To16()
	newIP[15] += 10

	return newIP.String()
}
func getNeighbor(ip string, ipv4 bool) string {

	if ipv4 == true {
		newIP := net.ParseIP(ip)
		newIP = newIP.To4()
		newIP[3] += 1

		return newIP.String()
	} else {
		newIP := net.ParseIP(ip)
		newIP = newIP.To16()
		newIP[15] += 1

		return newIP.String()
	}
}

func getStaticNeighbor(ip string, ipv4 bool) string {

	if ipv4 == true {
		newIP := net.ParseIP(ip)
		newIP = newIP.To4()
		newIP[3] += 10

		return newIP.String()
	} else {
		newIP := net.ParseIP(ip)
		newIP = newIP.To16()
		newIP[15] += 10

		return newIP.String()

	}
}

func pingNeighbors(t *testing.T, dut1 *ondatra.DUTDevice, dut2 *ondatra.DUTDevice, IPv4 bool) {

	pingRequest := &spb.PingRequest{}
	gnoiClient, err := dut1.RawAPIs().BindingDUT().DialGNOI(context.Background())
	ports := [4]string{dut2.Port(t, "port1").Name(), dut2.Port(t, "port2").Name(),
		"Bundle-Ether100", "Bundle-Ether101"}

	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	for i := 0; i < len(ports); i++ {

		if IPv4 == true {
			pingRequest.Destination = dut2IntfAttrib[i].attrib.IPv4
		} else {
			pingRequest.Destination = dut2IntfAttrib[i].attrib.IPv6
		}
		pingClient, err := gnoiClient.System().Ping(context.Background(), pingRequest)
		if err != nil {
			t.Errorf("Failed to query gnoi endpoint: %v", err)
		}
		responses, err := fetchResponses(pingClient)
		if err != nil {
			if IPv4 == true {
				t.Logf("Failed to handle gnoi ping client stream: %v\n", err.Error())
			} else {
				t.Logf("Failed to handle gnoi ping client stream: %v for Destination %s", err, dut2IntfAttrib[i].attrib.IPv6)
			}
		}
		summary := responses[len(responses)-1]
		if summary.Received == 0 {
			if IPv4 == true {
				t.Logf("No response to ping from Destination %s\n", dut2IntfAttrib[i].attrib.IPv4)
			} else {
				t.Logf("No response to ping from Destination %s\n", dut2IntfAttrib[i].attrib.IPv6)
			}
		}
	}
}

func pingScaleNeighbors(t *testing.T, dut1 *ondatra.DUTDevice, dut2 *ondatra.DUTDevice, IPv4 bool) {

	var InterfacesIPv4 []*oc.Interface_Subinterface_Ipv4
	var InterfacesIPv6 []*oc.Interface_Subinterface_Ipv6
	var destIP []string
	var wg sync.WaitGroup

	if IPv4 == true {
		InterfacesIPv4 = gnmi.GetAll(t, dut1, gnmi.OC().InterfaceAny().SubinterfaceAny().Ipv4().State())
		for _, intf := range InterfacesIPv4 {
			for ip := range intf.Address {
				if ip[:2] == "10" || ip[:2] == "11" || ip[:2] == "12" || ip[:2] == "13" ||
					ip[:2] == "20" || ip[:2] == "21" || ip[:2] == "22" || ip[:2] == "23" {
					destIP = append(destIP, ip)
				}
			}
		}
	} else {
		InterfacesIPv6 = gnmi.GetAll(t, dut1, gnmi.OC().InterfaceAny().SubinterfaceAny().Ipv6().State())
		for _, intf := range InterfacesIPv6 {
			for ip := range intf.Address {
				if ip[:2] == "10" || ip[:2] == "11" || ip[:2] == "12" || ip[:2] == "13" ||
					ip[:2] == "20" || ip[:2] == "21" || ip[:2] == "22" || ip[:2] == "23" {
					destIP = append(destIP, ip)
				}
			}
		}
	}
	gnoiClient, err := dut2.RawAPIs().BindingDUT().DialGNOI(context.Background())
	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	const maxConcurrentGoroutines = 10
	sem := make(chan struct{}, maxConcurrentGoroutines)
	for i := 0; i < len(destIP); i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(dest string) {
			defer wg.Done()
			defer func() { <-sem }()
			pingRequest := &spb.PingRequest{Count: 2}
			pingRequest.Destination = dest
			// fmt.Printf("Debug: Pinging %s\n", dest)

			pingClient, err := gnoiClient.System().Ping(context.Background(), pingRequest)

			if err != nil {
				t.Errorf("Failed to query gnoi endpoint: %v", err)
			}
			responses, err := fetchResponses(pingClient)
			if err != nil {
				t.Logf("Failed to handle gnoi ping client stream: %v for Destination %s", err, dest)
			}
			if len(responses) > 1 {
				summary := responses[len(responses)-1]
				if summary.Received == 0 {
					t.Logf("No response to ping from Destination %s\n", dest)
				}
			}
		}(destIP[i])
	}
	wg.Wait()
}

func fetchResponses(c spb.System_PingClient) ([]*spb.PingResponse, error) {

	pingResp := []*spb.PingResponse{}

	for {
		resp, err := c.Recv()
		switch {
		case err == io.EOF:
			return pingResp, nil
		case err != nil:
			return nil, err
		default:
			pingResp = append(pingResp, resp)
		}
	}
}
