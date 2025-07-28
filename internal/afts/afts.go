// Package afts provides helper functions to advertise routes to be verified in AFT.
package afts

import (
	"fmt"
	"testing"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	bgpRouteCountIPv4Default  = 2000000
	bgpRouteCountIPv4LowScale = 100000
	bgpRouteCountIPv6Default  = 1000000
	bgpRouteCountIPv6LowScale = 100000
)

// BGPRoute represents a collection of BGP routes to be advertised from the ATE.
type BGPRoute struct {
	// RouterID is the DUT Router ID.
	RouterID string
	// IPV4DevAddr represnts a BGP IPv4 peer.
	IPV4DevAddr gosnappi.DeviceIpv4
	// IPV6DevAddr represents a BGP IPv6 peer.
	IPV6DevAddr gosnappi.DeviceIpv6

	// ASN is a fixed ASN used for all BGP peers.
	ASN uint32

	// Dev is the device on which the BGP routes are advertised.
	Dev gosnappi.Device

	// AdvertiseV4 is the starting address for the IPv4 routes to be advertised.
	AdvertiseV4 string
	// V4Prefix is the prefix on each of the advertised IPv4 routes.
	V4Prefix uint32
	// V4RouteCount is the total number of IPv4 routes to be advertised.
	V4RouteCount uint32

	// AdvertiseV6 is the starting address for the IPv6 routes to be advertised.
	AdvertiseV6 string
	// V6Prefix is the prefix on each of the advertised IPv6 addresses.
	V6Prefix uint32
	// V6RouteCount is the total number of IPv6 routes to be advertised.
	V6RouteCount uint32
}

func validRoute(route *BGPRoute) error {
	if route == nil {
		return fmt.Errorf("route is nil")
	}
	if route.RouterID == "" {
		return fmt.Errorf("bgpV4Addr is nil")
	}
	if route.Dev == nil {
		return fmt.Errorf("device is nil")
	}
	return nil
}

// ConfigureAdvertisedEBGPRoutes advertises the given routes over eBGP from the ATE on the given interfaces.
func ConfigureAdvertisedEBGPRoutes(t *testing.T, route *BGPRoute) error {
	if err := validRoute(route); err != nil {
		return err
	}
	bgp := route.Dev.Bgp().SetRouterId(route.RouterID)

	bgp4Peer := bgp.Ipv4Interfaces().Add().SetIpv4Name(route.IPV4DevAddr.Name()).Peers().Add().SetName(route.Dev.Name() + ".BGP4.peer")
	bgp4Peer.SetPeerAddress(route.IPV4DevAddr.Gateway()).SetAsNumber(route.ASN).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	routes := bgp4Peer.V4Routes().Add().SetName(bgp4Peer.Name() + ".v4route")
	routes.SetNextHopIpv4Address(route.IPV4DevAddr.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	routes.Addresses().Add().
		SetAddress(route.AdvertiseV4).
		SetPrefix(route.V4Prefix).
		SetCount(route.V4RouteCount)

	bgp6Peer := bgp.Ipv6Interfaces().Add().SetIpv6Name(route.IPV6DevAddr.Name()).Peers().Add().SetName(route.Dev.Name() + ".BGP6.peer")
	bgp6Peer.SetPeerAddress(route.IPV6DevAddr.Gateway()).SetAsNumber(route.ASN).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	routesV6 := bgp6Peer.V6Routes().Add().SetName(bgp6Peer.Name() + ".v6route")
	routesV6.SetNextHopIpv6Address(route.IPV6DevAddr.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	routesV6.Addresses().Add().
		SetAddress(route.AdvertiseV6).
		SetPrefix(route.V6Prefix).
		SetCount(route.V6RouteCount)
	return nil
}

// BGPNeighbor represents a BGP neighbor.
type BGPNeighbor struct {
	IP      string
	Version oc.E_BgpTypes_AFI_SAFI_TYPE
}

// EBGPParams represents the parameters for creating an eBGP neighbor.
type EBGPParams struct {
	// DUT is the DUT to configure the eBGP neighbors with.
	DUT *ondatra.DUTDevice
	// DUTASN is the ASN of the DUT.
	DUTASN uint32
	// NeighborASN is the ASN of ALL the provided neighbors.
	NeighborASN uint32

	// RouterID is the BGP router ID of the DUT.
	RouterID string
	// PeerGrpNameV4 is the peer group name for IPv4.
	PeerGrpNameV4 string
	// PeerGrpNameV6 is the peer group name for IPv6.
	PeerGrpNameV6 string
	// Neighbors is the list of BGP neighbors to configure.
	Neighbors []*BGPNeighbor

	// ApplyPolicyName is the name of the policy to apply to the created neighbor(s).
	ApplyPolicyName string
}

func verifyEBGPParams(t *testing.T, params *EBGPParams) error {
	t.Helper()
	if params == nil {
		return fmt.Errorf("params is nil")
	}
	if params.DUT == nil {
		return fmt.Errorf("DUT is nil")
	}
	if params.DUTASN == 0 {
		return fmt.Errorf("DUT ASN is 0")
	}
	if params.RouterID == "" {
		return fmt.Errorf("Router ID is empty")
	}
	if params.PeerGrpNameV4 == "" {
		return fmt.Errorf("Peer Group Name V4 is empty")
	}
	if params.PeerGrpNameV6 == "" {
		return fmt.Errorf("Peer Group Name V6 is empty")
	}
	if len(params.Neighbors) == 0 {
		return fmt.Errorf("no neighbors specified")
	}
	for _, nbr := range params.Neighbors {
		if nbr.IP == "" {
			return fmt.Errorf("neighbor IP is empty")
		}
		isV4 := nbr.Version == oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST
		isV6 := nbr.Version == oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST
		if !isV4 && !isV6 {
			return fmt.Errorf("neighbor version is not supported")
		}
	}
	return nil
}

// CreateEBGPNeighbor creates an eBGP neighbor from the provided parameters. The number of max paths
// is determined by the number of neighbors.
func CreateEBGPNeighbor(t *testing.T, params *EBGPParams) *oc.NetworkInstance_Protocol {
	t.Helper()
	if err := verifyEBGPParams(t, params); err != nil {
		t.Fatalf("Failed to verify params: %v", err)
	}
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(params.DUT))
	protocol := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := protocol.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.SetAs(params.DUTASN)
	global.SetRouterId(params.RouterID)

	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	peerGroupV4 := bgp.GetOrCreatePeerGroup(params.PeerGrpNameV4)
	peerGroupV4.SetPeerAs(params.NeighborASN)
	afiSAFI := global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	afiSAFI.SetEnabled(true)
	afiSAFI.GetOrCreateUseMultiplePaths().GetOrCreateEbgp().SetMaximumPaths(uint32(len(params.Neighbors)))
	peerGroupV4AfiSafi := peerGroupV4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	peerGroupV4AfiSafi.SetEnabled(true)
	peerGroupV4AfiSafi.GetOrCreateUseMultiplePaths().SetEnabled(true)

	peerGroupV6 := bgp.GetOrCreatePeerGroup(params.PeerGrpNameV6)
	peerGroupV6.SetPeerAs(params.NeighborASN)
	asisafi6 := global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	asisafi6.SetEnabled(true)
	asisafi6.GetOrCreateUseMultiplePaths().GetOrCreateEbgp().SetMaximumPaths(uint32(len(params.Neighbors)))
	peerGroupV6AfiSafi := peerGroupV6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	peerGroupV6AfiSafi.SetEnabled(true)
	peerGroupV6AfiSafi.GetOrCreateUseMultiplePaths().SetEnabled(true)

	for _, nbr := range params.Neighbors {
		neighbor := bgp.GetOrCreateNeighbor(nbr.IP)
		neighbor.SetPeerAs(params.NeighborASN)
		neighbor.SetEnabled(true)
		switch ver := nbr.Version; ver {
		case oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST:
			neighbor.SetPeerGroup(params.PeerGrpNameV4)
			neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).SetEnabled(true)
			neighbourAFV4 := peerGroupV4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			neighbourAFV4.SetEnabled(true)
			applyPolicy := neighbourAFV4.GetOrCreateApplyPolicy()
			applyPolicy.ImportPolicy = []string{params.ApplyPolicyName}
			applyPolicy.ExportPolicy = []string{params.ApplyPolicyName}
		case oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST:
			neighbor.SetPeerGroup(params.PeerGrpNameV6)
			neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).SetEnabled(true)
			neighbourAFV6 := peerGroupV6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			neighbourAFV6.SetEnabled(true)
			applyPolicy := neighbourAFV6.GetOrCreateApplyPolicy()
			applyPolicy.ImportPolicy = []string{params.ApplyPolicyName}
			applyPolicy.ExportPolicy = []string{params.ApplyPolicyName}
		}
	}
	return protocol
}

// RouteCount returns the expected route count for the given dut and IP family.
func RouteCount(dut *ondatra.DUTDevice, isV4 bool) uint32 {
	if deviations.LowScaleAft(dut) {
		if isV4 {
			return bgpRouteCountIPv4LowScale
		}
		return bgpRouteCountIPv6LowScale
	}
	if isV4 {
		return bgpRouteCountIPv4Default
	}
	return bgpRouteCountIPv6Default
}

// ISISParams represents the parameters for creating and advertising ISIS neighbors for a provided
// device.
type ISISParams struct {
	Dev gosnappi.Device

	V4Addr, EthName, PortName, SystemID string

	V4AdvertisedAddr   string
	V4AdvertisedPrefix uint32
	V4AdvertisedCount  uint32

	V6AdvertisedAddr   string
	V6AdvertisedPrefix uint32
	V6AdvertisedCount  uint32
}

func validISISParams(params *ISISParams) error {
	if params == nil {
		return fmt.Errorf("params is nil")
	}
	if params.Dev == nil {
		return fmt.Errorf("device is nil")
	}
	if params.V4Addr == "" {
		return fmt.Errorf("V4Addr is empty")
	}
	if params.EthName == "" {
		return fmt.Errorf("EthName is empty")
	}
	if params.PortName == "" {
		return fmt.Errorf("PortName is empty")
	}
	if params.SystemID == "" {
		return fmt.Errorf("SystemID is empty")
	}
	if params.V4AdvertisedAddr == "" {
		return fmt.Errorf("V4AdvertisedAddr is empty")
	}
	if params.V6AdvertisedAddr == "" {
		return fmt.Errorf("V6AdvertisedAddr is empty")
	}
	return nil
}

// ConfigureAdvertisedISISRoutes configures the ATE to advertise ISIS routes.
func ConfigureAdvertisedISISRoutes(t *testing.T, params *ISISParams) {
	t.Helper()

	isis := params.Dev.Isis().SetName(params.Dev.Name() + ".isis").
		SetSystemId(params.SystemID)
	isis.Basic().
		SetIpv4TeRouterId(params.V4Addr).
		SetHostname(fmt.Sprintf("ixia-c-%s", params.PortName))
	isis.Advanced().
		SetAreaAddresses([]string{"49"})
	isisInt := isis.Interfaces().Add().SetName(isis.Name() + ".intf").
		SetEthName(params.EthName).
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetMetric(10)
	isisInt.TrafficEngineering().Add().PriorityBandwidths()
	isisInt.Advanced().
		SetAutoAdjustMtu(true).
		SetAutoAdjustArea(true).
		SetAutoAdjustSupportedProtocols(true)

	isisV4Routes := isis.V4Routes().Add().SetName(isis.Name() + ".v4")
	isisV4Routes.Addresses().Add().
		SetAddress(params.V4AdvertisedAddr).
		SetPrefix(params.V4AdvertisedPrefix).
		SetCount(params.V4AdvertisedCount)
	isisV6Routes := isis.V6Routes().Add().SetName(isis.Name() + ".v6")
	isisV6Routes.Addresses().Add().
		SetAddress(params.V6AdvertisedAddr).
		SetPrefix(params.V6AdvertisedPrefix).
		SetCount(params.V6AdvertisedCount)
}
