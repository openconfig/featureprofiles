package otgconfighelpers

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/open-traffic-generator/snappi/gosnappi"
)

// BGPGracefulRestartAttrs defines attributes for BGP Graceful Restart.
type BGPGracefulRestartAttrs struct {
	Enabled     bool
	RestartTime uint32
	StaleTime   uint32
}

// BGPTimerAttrs defines attributes for BGP timers.
type BGPTimerAttrs struct {
	HoldTime      uint32
	KeepAliveTime uint32
}

// BGPPeerAttrs defines attributes for a BGP peer.
type BGPPeerAttrs struct {
	Name            string
	PeerAddress     string
	ASNumber        uint32
	ASType          gosnappi.BgpV4PeerAsTypeEnum
	RouterID        string
	GracefulRestart *BGPGracefulRestartAttrs
	Timers          *BGPTimerAttrs
	LearnedV4Pfx    bool
	LearnedV6Pfx    bool
}

// ISIS config required for IBGP usecase
// ISISInterfaceAttrs defines attributes for an ISIS interface.
type ISISInterfaceAttrs struct {
	Name        string
	EthName     string
	NetworkType gosnappi.IsisInterfaceNetworkTypeEnum
	LevelType   gosnappi.IsisInterfaceLevelTypeEnum
	Metric      uint32
}

// ISISAttrs defines attributes for an ISIS routing process.
type ISISAttrs struct {
	Name          string
	SystemID      string
	Hostname      string
	AreaAddresses []string
	Interfaces    []*ISISInterfaceAttrs
}

// BGPPeerOption is a function to set BGPPeerAttrs options.
type BGPPeerOption func(*BGPPeerAttrs)

// DefaultBGPPeerAttrs returns default BGP peer attributes.
func DefaultBGPPeerAttrs() *BGPPeerAttrs {
	return &BGPPeerAttrs{
		ASType: gosnappi.BgpV4PeerAsType.EBGP,
		GracefulRestart: &BGPGracefulRestartAttrs{
			Enabled:     true,
			RestartTime: 120,
			StaleTime:   300,
		},
		Timers: &BGPTimerAttrs{
			HoldTime:      240,
			KeepAliveTime: 80,
		},
	}
}

// WithBGPName sets the BGP peer name.
func WithBGPName(name string) BGPPeerOption {
	return func(attrs *BGPPeerAttrs) {
		attrs.Name = name
	}
}

// WithBGPPeerAddress sets the BGP peer address.
func WithBGPPeerAddress(addr string) BGPPeerOption {
	return func(attrs *BGPPeerAttrs) {
		attrs.PeerAddress = addr
	}
}

// WithBGPASNumber sets the BGP AS number.
func WithBGPASNumber(as uint32) BGPPeerOption {
	return func(attrs *BGPPeerAttrs) {
		attrs.ASNumber = as
	}
}

// WithBGPEBGP sets the BGP session type to EBGP.
func WithBGPEBGP() BGPPeerOption {
	return func(attrs *BGPPeerAttrs) {
		attrs.ASType = gosnappi.BgpV4PeerAsType.EBGP
	}
}

// WithBGPIBGP sets the BGP session type to IBGP.
func WithBGPIBGP() BGPPeerOption {
	return func(attrs *BGPPeerAttrs) {
		attrs.ASType = gosnappi.BgpV4PeerAsType.IBGP
	}
}

// WithBGPRouterID sets the BGP Router ID.
func WithBGPRouterID(routerID string) BGPPeerOption {
	return func(attrs *BGPPeerAttrs) {
		attrs.RouterID = routerID
	}
}

// WithoutBGPGR disables Graceful Restart.
func WithoutBGPGR() BGPPeerOption {
	return func(attrs *BGPPeerAttrs) {
		attrs.GracefulRestart.Enabled = false
	}
}

// WithBGPTimers sets the BGP timers.
func WithBGPTimers(holdTime, keepAliveTime uint32) BGPPeerOption {
	return func(attrs *BGPPeerAttrs) {
		attrs.Timers.HoldTime = holdTime
		attrs.Timers.KeepAliveTime = keepAliveTime
	}
}

// WithBGPLearnedV4Pfx enables learning IPv4 unicast prefixes.
func WithBGPLearnedV4Pfx(enable bool) BGPPeerOption {
	return func(attrs *BGPPeerAttrs) {
		attrs.LearnedV4Pfx = enable
	}
}

// WithBGPLearnedV6Pfx enables learning IPv6 unicast prefixes.
func WithBGPLearnedV6Pfx(enable bool) BGPPeerOption {
	return func(attrs *BGPPeerAttrs) {
		attrs.LearnedV6Pfx = enable
	}
}

// AddBGPV4Peer configures a BGPv4 peer on the device.
func AddBGPV4Peer(dev gosnappi.Device, intfName string, opts ...BGPPeerOption) gosnappi.BgpV4Peer {
	attrs := DefaultBGPPeerAttrs()
	for _, opt := range opts {
		opt(attrs)
	}

	bgp := dev.Bgp()
	if bgp == nil {
		bgp = dev.Bgp()
	}
	if attrs.RouterID != "" {
		bgp.SetRouterId(attrs.RouterID)
	}

	var bgp4Intf gosnappi.BgpV4Interface
	for _, item := range bgp.Ipv4Interfaces().Items() {
		if item.Ipv4Name() == intfName {
			bgp4Intf = item
			break
		}
	}
	if bgp4Intf == nil {
		bgp4Intf = bgp.Ipv4Interfaces().Add().SetIpv4Name(intfName)
	}

	bgp4Peer := bgp4Intf.Peers().Add()

	peerName := attrs.Name
	if peerName == "" {
		peerName = fmt.Sprintf("%s.%s.BGP4.peer%d", dev.Name(), intfName, len(bgp4Intf.Peers().Items()))
	}
	bgp4Peer.SetName(peerName)

	if attrs.PeerAddress != "" {
		bgp4Peer.SetPeerAddress(attrs.PeerAddress)
	}
	bgp4Peer.SetAsNumber(attrs.ASNumber)
	bgp4Peer.SetAsType(attrs.ASType)

	if attrs.GracefulRestart != nil {
		bgp4Peer.GracefulRestart().SetEnableGr(attrs.GracefulRestart.Enabled)
		if attrs.GracefulRestart.Enabled {
			bgp4Peer.GracefulRestart().SetRestartTime(attrs.GracefulRestart.RestartTime)
			bgp4Peer.GracefulRestart().SetStaleTime(attrs.GracefulRestart.StaleTime)
		}
	}

	if attrs.Timers != nil {
		bgp4Peer.Advanced().SetHoldTimeInterval(attrs.Timers.HoldTime)
		bgp4Peer.Advanced().SetKeepAliveInterval(attrs.Timers.KeepAliveTime)
	}

	bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(attrs.LearnedV4Pfx)

	return bgp4Peer
}

// AddBGPV6Peer configures a BGPv6 peer on the device.
func AddBGPV6Peer(dev gosnappi.Device, intfName string, opts ...BGPPeerOption) gosnappi.BgpV6Peer {
	attrs := DefaultBGPPeerAttrs()
	for _, opt := range opts {
		opt(attrs)
	}

	bgp := dev.Bgp()
	if bgp == nil {
		bgp = dev.Bgp()
	}
	if attrs.RouterID != "" {
		bgp.SetRouterId(attrs.RouterID)
	}

	var bgp6Intf gosnappi.BgpV6Interface
	for _, item := range bgp.Ipv6Interfaces().Items() {
		if item.Ipv6Name() == intfName {
			bgp6Intf = item
			break
		}
	}
	if bgp6Intf == nil {
		bgp6Intf = bgp.Ipv6Interfaces().Add().SetIpv6Name(intfName)
	}

	bgp6Peer := bgp6Intf.Peers().Add()

	peerName := attrs.Name
	if peerName == "" {
		peerName = fmt.Sprintf("%s.%s.BGP6.peer%d", dev.Name(), intfName, len(bgp6Intf.Peers().Items()))
	}
	bgp6Peer.SetName(peerName)

	if attrs.PeerAddress != "" {
		bgp6Peer.SetPeerAddress(attrs.PeerAddress)
	}
	bgp6Peer.SetAsNumber(attrs.ASNumber)
	if attrs.ASType == gosnappi.BgpV4PeerAsType.EBGP {
		bgp6Peer.SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	} else {
		bgp6Peer.SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
	}

	if attrs.GracefulRestart != nil {
		bgp6Peer.GracefulRestart().SetEnableGr(attrs.GracefulRestart.Enabled)
		if attrs.GracefulRestart.Enabled {
			bgp6Peer.GracefulRestart().SetRestartTime(attrs.GracefulRestart.RestartTime)
			bgp6Peer.GracefulRestart().SetStaleTime(attrs.GracefulRestart.StaleTime)
		}
	}

	if attrs.Timers != nil {
		bgp6Peer.Advanced().SetHoldTimeInterval(attrs.Timers.HoldTime)
		bgp6Peer.Advanced().SetKeepAliveInterval(attrs.Timers.KeepAliveTime)
	}

	bgp6Peer.LearnedInformationFilter().SetUnicastIpv6Prefix(attrs.LearnedV6Pfx)

	return bgp6Peer
}

// BGPRouteAttrs defines attributes for BGP route ranges.
type BGPRouteAttrs struct {
	NextHop             string
	Communities         []gosnappi.BgpCommunity
	ExtendedCommunities []gosnappi.BgpExtendedCommunity
	AsPath              gosnappi.BgpAsPath
	LocalPref           *uint32
	Origin              gosnappi.BgpAttributesOriginEnum
	Med                 *uint32
	NextHopIpv4         string
	NextHopIpv6         string
	NextHopMode         string // MANUAL or LOCAL_IP
	AddressCount        uint32
}

// BGPRouteOption is a function to set BGPRouteAttrs options.
type BGPRouteOption func(*BGPRouteAttrs)

// DefaultBGPRouteAttrs returns default BGP route attributes.
func DefaultBGPRouteAttrs() *BGPRouteAttrs {
	return &BGPRouteAttrs{
		AddressCount: 1,
		NextHopMode:  "MANUAL",
	}
}

// WithBGPRouteNextHopIPv4 sets the IPv4 next hop.
func WithBGPRouteNextHopIPv4(nextHop string) BGPRouteOption {
	return func(attrs *BGPRouteAttrs) {
		attrs.NextHopIpv4 = nextHop
	}
}

// WithBGPRouteNextHopMode sets the next hop mode.
func WithBGPRouteNextHopMode(nextHopMode string) BGPRouteOption {
	return func(attrs *BGPRouteAttrs) {
		attrs.NextHopMode = nextHopMode
	}
}

// WithBGPRouteNextHopIPv6 sets the IPv6 next hop.
func WithBGPRouteNextHopIPv6(nextHop string) BGPRouteOption {
	return func(attrs *BGPRouteAttrs) {
		attrs.NextHopIpv6 = nextHop
	}
}

// WithBGPRouteCommunities adds communities.
func WithBGPRouteCommunities(communities []gosnappi.BgpCommunity) BGPRouteOption {
	return func(attrs *BGPRouteAttrs) {
		attrs.Communities = communities
	}
}

// WithBGPRouteExtendedCommunities adds extended communities.
func WithBGPRouteExtendedCommunities(extCommunities []gosnappi.BgpExtendedCommunity) BGPRouteOption {
	return func(attrs *BGPRouteAttrs) {
		attrs.ExtendedCommunities = extCommunities
	}
}

// WithBGPRouteASPath sets the AS Path.
func WithBGPRouteASPath(asPath gosnappi.BgpAsPath) BGPRouteOption {
	return func(attrs *BGPRouteAttrs) {
		attrs.AsPath = asPath
	}
}

// WithBGPRouteAddressCount sets the number of addresses in the range.
func WithBGPRouteAddressCount(count uint32) BGPRouteOption {
	return func(attrs *BGPRouteAttrs) {
		attrs.AddressCount = count
	}
}

// AddBGPV4Routes configures BGPv4 routes to be advertised.
func AddBGPV4Routes(peer gosnappi.BgpV4Peer, name string, prefixes []string, opts ...BGPRouteOption) gosnappi.BgpV4RouteRange {
	attrs := DefaultBGPRouteAttrs()
	for _, opt := range opts {
		opt(attrs)
	}

	routes := peer.V4Routes().Add().SetName(name)
	for _, p := range prefixes {
		parts := strings.Split(p, "/")
		addr := parts[0]
		prefixLen := 32
		if len(parts) > 1 {
			prefixLen, _ = strconv.Atoi(parts[1])
		}
		routes.Addresses().Add().SetAddress(addr).SetPrefix(uint32(prefixLen)).SetCount(attrs.AddressCount)
	}

	if attrs.NextHopIpv4 != "" {
		routes.SetNextHopIpv4Address(attrs.NextHopIpv4).
			SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4)
		if attrs.NextHopMode == "MANUAL" {
			routes.SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
		} else {
			routes.SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.LOCAL_IP)
		}
	} else {
		routes.SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.LOCAL_IP)
	}

	for _, c := range attrs.Communities {
		routes.Communities().Add().SetType(c.Type()).SetAsNumber(c.AsNumber()).SetAsCustom(c.AsCustom())
	}

	for _, c := range attrs.ExtendedCommunities {
		extComm := routes.ExtendedCommunities().Add().NonTransitive2OctetAsType().LinkBandwidthSubtype()
		extComm.SetGlobal2ByteAs(c.NonTransitive2OctetAsType().LinkBandwidthSubtype().Global2ByteAs())
		extComm.SetBandwidth(c.NonTransitive2OctetAsType().LinkBandwidthSubtype().Bandwidth())
	}

	if attrs.AsPath != nil {
		routes.SetAsPath(attrs.AsPath)
	}
	// TODO: will add more usecases for other attributes like LocalPref, Origin, MED, etc.

	return routes
}

// AddBGPV6Routes configures BGPv6 routes to be advertised.
func AddBGPV6Routes(peer gosnappi.BgpV6Peer, name string, prefixes []string, opts ...BGPRouteOption) gosnappi.BgpV6RouteRange {
	attrs := DefaultBGPRouteAttrs()
	for _, opt := range opts {
		opt(attrs)
	}
	routes := peer.V6Routes().Add().SetName(name)
	for _, p := range prefixes {
		parts := strings.Split(p, "/")
		addr := parts[0]
		prefixLen := 128
		if len(parts) > 1 {
			prefixLen, _ = strconv.Atoi(parts[1])
		}
		routes.Addresses().Add().SetAddress(addr).SetPrefix(uint32(prefixLen)).SetCount(attrs.AddressCount)
	}

	if attrs.NextHopIpv6 != "" {
		routes.SetNextHopIpv6Address(attrs.NextHopIpv6).
			SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6)
		if attrs.NextHopMode == "MANUAL" {
			routes.SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
		} else {
			routes.SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.LOCAL_IP)
		}
	} else {
		routes.SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.LOCAL_IP)
	}

	for _, c := range attrs.Communities {
		routes.Communities().Add().SetType(c.Type()).SetAsNumber(c.AsNumber()).SetAsCustom(c.AsCustom())
	}

	for _, c := range attrs.ExtendedCommunities {
		extComm := routes.ExtendedCommunities().Add().NonTransitive2OctetAsType().LinkBandwidthSubtype()
		extComm.SetGlobal2ByteAs(c.NonTransitive2OctetAsType().LinkBandwidthSubtype().Global2ByteAs())
		extComm.SetBandwidth(c.NonTransitive2OctetAsType().LinkBandwidthSubtype().Bandwidth())
	}

	if attrs.AsPath != nil {
		routes.SetAsPath(attrs.AsPath)
	}

	// TODO: will add more usecases for other attributes like LocalPref, Origin, MED, etc.

	return routes
}

// CreateBGPCommunity creates a BGP community.
func CreateBGPCommunity(asNum, custom uint32) gosnappi.BgpCommunity {
	comm := gosnappi.NewBgpCommunity()
	comm.SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
	comm.SetAsNumber(asNum)
	comm.SetAsCustom(custom)
	return comm
}

// CreateBGPLinkBandwidthExtCommunity creates a Non-Transitive 2-Octet AS Type Link Bandwidth Extended Community.
// globalAS: The Global Administrator AS number.
// bandwidth: The link bandwidth in bytes per second.
func CreateBGPLinkBandwidthExtCommunity(globalAS uint32, bandwidth float32) gosnappi.BgpExtendedCommunity {
	extComm := gosnappi.NewBgpExtendedCommunity()
	lbw := extComm.NonTransitive2OctetAsType().LinkBandwidthSubtype()
	lbw.SetGlobal2ByteAs(globalAS)
	lbw.SetBandwidth(bandwidth)
	return extComm
}

// CreateBGPASPath creates a BGP AS Path.
func CreateBGPASPath(asNumbers []uint32, segmentType gosnappi.BgpAsPathSegmentTypeEnum, asSetMode gosnappi.BgpAsPathAsSetModeEnum) gosnappi.BgpAsPath {
	asPath := gosnappi.NewBgpAsPath()
	asPath.SetAsSetMode(asSetMode)
	segment := asPath.Segments().Add()
	segment.SetType(segmentType)
	segment.SetAsNumbers(asNumbers)
	return asPath
}

// ConfigureISIS configures the ISIS protocol on a device.
func ConfigureISIS(t *testing.T, dev gosnappi.Device, attrs *ISISAttrs) gosnappi.DeviceIsisRouter {
	t.Helper()
	isis := dev.Isis().SetName(attrs.Name).SetSystemId(attrs.SystemID)
	isis.Basic().SetHostname(attrs.Hostname).SetLearnedLspFilter(true)
	isis.Advanced().SetAreaAddresses(attrs.AreaAddresses)

	for _, intfAttrs := range attrs.Interfaces {
		isisInt := isis.Interfaces().Add().
			SetName(intfAttrs.Name).
			SetEthName(intfAttrs.EthName).
			SetNetworkType(intfAttrs.NetworkType).
			SetLevelType(intfAttrs.LevelType).
			SetMetric(intfAttrs.Metric)
		isisInt.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)
	}
	return isis
}

// AddISISRoutesV4 adds IPv4 routes to ISIS.
func AddISISRoutesV4(isis gosnappi.DeviceIsisRouter, name string, linkMetric uint32, address string, prefix uint32, count uint32) {
	route := isis.V4Routes().Add().SetName(name).SetLinkMetric(linkMetric)
	route.Addresses().Add().SetAddress(address).SetPrefix(prefix).SetCount(count)
}

// AddISISRoutesV6 adds IPv6 routes to ISIS.
func AddISISRoutesV6(isis gosnappi.DeviceIsisRouter, name string, linkMetric uint32, address string, prefix uint32, count uint32) {
	route := isis.V6Routes().Add().SetName(name).SetLinkMetric(linkMetric)
	route.Addresses().Add().SetAddress(address).SetPrefix(prefix).SetCount(count)
}
