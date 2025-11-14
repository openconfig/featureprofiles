package otgconfighelpers

import (
	"fmt"
	"strings"
	"testing"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/ondatra"
)

const (
	ATEAS = 200

	ISISATESystemIDPrefix = "6400.0000.000" // Intentionally one character short to append port-id as suffix.
	ISISArea              = "49"

	StartingISISRouteV4 = "199.0.0.1"
	StartingISISRouteV6 = "2001:db8::203:0:113:1"

	DefaultISISRouteCount = 100

	StartingBGPRouteIPv4 = "200.0.0.0"
	StartingBGPRouteIPv6 = "3001:1::0"

	DefaultBGPRouteCount = 200

	V4PrefixLen = 32
	V6PrefixLen = 128

	// ATE Suffixes
	bgpV4Suffix = ".bgp4.peer"
	bgpV6Suffix = ".bgp6.peer"
	devSuffix   = ".dev"
	ethSuffix   = ".eth"
	intfSuffix  = ".intf"
	ipv4Suffix  = ".ipv4"
	ipv6Suffix  = ".ipv6"
	isisSuffix  = ".isis"
)

// AdvertisedRoutes represents the advertised routes configuration.
type AdvertisedRoutes struct {
	// Starting address of the routes.
	StartingAddress string
	// Prefix length for each prefix advertised.
	PrefixLength uint32
	// Number of routes advertised. If value is 0, default value is used.
	Count uint32
	// ATE AS of the peer, advertised routes are sent from.
	ATEAS uint32
}

// AdvertiseBGPRoutes configures BGP advertised routes on the dev over the ipv4 and ipv6 interfaces.
func AdvertiseBGPRoutes(dev gosnappi.Device, ipv4 gosnappi.DeviceIpv4, ipv6 gosnappi.DeviceIpv6, v4Advertised, v6Advertised *AdvertisedRoutes) {
	bgp := dev.Bgp().SetRouterId(ipv4.Address())
	bgp4Peer := bgp.Ipv4Interfaces().Add().SetIpv4Name(ipv4.Name()).Peers().Add().SetName(dev.Name() + bgpV4Suffix)
	bgp4Peer.SetPeerAddress(ipv4.Gateway()).SetAsNumber(v4Advertised.ATEAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	bgp6Peer := bgp.Ipv6Interfaces().Add().SetIpv6Name(ipv6.Name()).Peers().Add().SetName(dev.Name() + bgpV6Suffix)
	bgp6Peer.SetPeerAddress(ipv6.Gateway()).SetAsNumber(v6Advertised.ATEAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

	routes := bgp4Peer.V4Routes().Add().SetName(bgp4Peer.Name() + ipv4Suffix)
	routes.SetNextHopIpv4Address(ipv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	routes.Addresses().Add().
		SetAddress(v4Advertised.StartingAddress).
		SetPrefix(v4Advertised.PrefixLength).
		SetCount(v4Advertised.Count)

	routesV6 := bgp6Peer.V6Routes().Add().SetName(bgp6Peer.Name() + ipv6Suffix)
	routesV6.SetNextHopIpv6Address(ipv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	routesV6.Addresses().Add().
		SetAddress(v6Advertised.StartingAddress).
		SetPrefix(v6Advertised.PrefixLength).
		SetCount(v6Advertised.Count)
}

// AdvertiseISISRoutes configures ISIS advertised routes on the dev over the ipv4 and ipv6 interfaces.
func AdvertiseISISRoutes(dev gosnappi.Device, ipv4 gosnappi.DeviceIpv4, ipv6 gosnappi.DeviceIpv6, v4Advertised, v6Advertised *AdvertisedRoutes) {
	isis := dev.Isis().SetName(dev.Name() + isisSuffix)
	v4Route := isis.V4Routes().Add().SetName(isis.Name() + ipv4Suffix)
	v4Route.Addresses().Add().SetAddress(v4Advertised.StartingAddress).
		SetPrefix(v4Advertised.PrefixLength).
		SetCount(v4Advertised.Count)
	v6Route := isis.V6Routes().Add().SetName(isis.Name() + ipv6Suffix)
	v6Route.Addresses().Add().SetAddress(v6Advertised.StartingAddress).
		SetPrefix(v6Advertised.PrefixLength).
		SetCount(v6Advertised.Count)
}

// ATEAdvertiseRoutes represents the advertised routes configuration.
type ATEAdvertiseRoutes struct {
	// ATE device
	ATE *ondatra.ATEDevice
	// ATE attributes for ports.
	ATEAttrs []attrs.Attributes
	// DUT attributes for ports.
	DUTAttrs []attrs.Attributes
	// ISIS Routes to be advertised
	ISISV4Routes, ISISV6Routes *AdvertisedRoutes
	// BGP Routes to be advertised
	BGPV4Routes, BGPV6Routes *AdvertisedRoutes
}

// missingRoutesDefault sets default values for the advertised routes, if any are nil.
func (ar *ATEAdvertiseRoutes) missingRoutesDefault() {
	if ar.ISISV4Routes == nil {
		ar.ISISV4Routes = &AdvertisedRoutes{
			StartingAddress: StartingISISRouteV4,
			PrefixLength:    V4PrefixLen,
			Count:           DefaultISISRouteCount,
			ATEAS:           ATEAS,
		}
	}
	if ar.ISISV6Routes == nil {
		ar.ISISV6Routes = &AdvertisedRoutes{
			StartingAddress: StartingISISRouteV6,
			PrefixLength:    V6PrefixLen,
			Count:           DefaultISISRouteCount,
			ATEAS:           ATEAS,
		}
	}
	if ar.BGPV4Routes == nil {
		ar.BGPV4Routes = &AdvertisedRoutes{
			StartingAddress: StartingBGPRouteIPv4,
			PrefixLength:    V4PrefixLen,
			Count:           DefaultBGPRouteCount,
			ATEAS:           ATEAS,
		}
	}
	if ar.BGPV6Routes == nil {
		ar.BGPV6Routes = &AdvertisedRoutes{
			StartingAddress: StartingBGPRouteIPv6,
			PrefixLength:    V6PrefixLen,
			Count:           DefaultBGPRouteCount,
			ATEAS:           ATEAS,
		}
	}
}

// ConfigureATEWithISISAndBGPRoutes builds an OTG config with Ethernet, IPv4/IPv6,
// ISIS, and advertised BGP routes for the provided ports and returns it.
// - ISIS routes are advertised only from the first port in ports slice to produce a single next-hop.
func ConfigureATEWithISISAndBGPRoutes(t *testing.T, ateRoutes *ATEAdvertiseRoutes) gosnappi.Config {
	t.Helper()
	config := gosnappi.NewConfig()

	ate := ateRoutes.ATE
	ateRoutes.missingRoutesDefault()

	for i := range ateRoutes.ATEAttrs {
		ateAttr := ateRoutes.ATEAttrs[i]
		dutAttr := ateRoutes.DUTAttrs[i]

		atePort := ate.Port(t, ateAttr.Name)
		port := config.Ports().Add().SetName(atePort.ID())
		dev := config.Devices().Add().SetName(ateAttr.Name + devSuffix)

		eth := dev.Ethernets().Add().SetName(dev.Name() + ethSuffix).
			SetMac(ateAttr.MAC).
			SetMtu(uint32(ateAttr.MTU))
		eth.Connection().SetPortName(port.Name())

		ipv4 := eth.Ipv4Addresses().Add().SetName(eth.Name() + ipv4Suffix).
			SetAddress(ateAttr.IPv4).
			SetGateway(dutAttr.IPv4).
			SetPrefix(uint32(ateAttr.IPv4Len))

		ipv6 := eth.Ipv6Addresses().Add().SetName(eth.Name() + ipv6Suffix).
			SetAddress(ateAttr.IPv6).
			SetGateway(dutAttr.IPv6).
			SetPrefix(uint32(ateAttr.IPv6Len))

		isis := dev.Isis().SetName(dev.Name() + isisSuffix).
			SetSystemId(fmt.Sprintf("%s%d", strings.ReplaceAll(ISISATESystemIDPrefix, ".", ""), i+1))
		isis.Basic().
			SetIpv4TeRouterId(ipv4.Address()).
			SetHostname(fmt.Sprintf("ixia-c-port%d", i+1))
		isis.Advanced().SetAreaAddresses([]string{ISISArea})
		isis.Advanced().SetEnableHelloPadding(false)
		isisInt := isis.Interfaces().Add().SetName(isis.Name() + intfSuffix).
			SetEthName(eth.Name()).
			SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
			SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
			SetMetric(10)
		isisInt.TrafficEngineering().Add().PriorityBandwidths()
		isisInt.Advanced().
			SetAutoAdjustMtu(true).
			SetAutoAdjustArea(true).
			SetAutoAdjustSupportedProtocols(true)

		// Advertise ISIS routes only from first port to produce a single next-hop
		if i == 0 {
			if ateRoutes.ISISV4Routes.Count == 0 {
				ateRoutes.ISISV4Routes.Count = DefaultISISRouteCount
			}
			if ateRoutes.ISISV6Routes.Count == 0 {
				ateRoutes.ISISV6Routes.Count = DefaultISISRouteCount
			}
			AdvertiseISISRoutes(dev, ipv4, ipv6, ateRoutes.ISISV4Routes, ateRoutes.ISISV6Routes)
		}

		// Advertise BGP routes
		if ateRoutes.BGPV4Routes.Count == 0 {
			ateRoutes.BGPV4Routes.Count = DefaultBGPRouteCount
		}
		if ateRoutes.BGPV6Routes.Count == 0 {
			ateRoutes.BGPV6Routes.Count = DefaultBGPRouteCount
		}
		AdvertiseBGPRoutes(dev, ipv4, ipv6, ateRoutes.BGPV4Routes, ateRoutes.BGPV6Routes)
	}

	return config
}
