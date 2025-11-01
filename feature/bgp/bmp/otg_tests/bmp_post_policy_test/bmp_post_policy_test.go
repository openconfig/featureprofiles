package bmp_post_policy_test

import (
	"fmt"
	"net"
	"testing"

	// "time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"

	// "github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	// "github.com/openconfig/ygnmi/ygnmi"
)

const (
	dutAS           = 64520
	ate1AS          = 64531
	ate2AS          = 64532
	plenIPv4        = 30
	plenIPv6        = 126
	routeCount      = 100 // Reduced for test stability.
	policyName      = "bmp-post-policy-test"
	v4RoutePolicy   = "route-policy-v4"
	v4Statement     = "statement-v4"
	rejectV4PfxName = "reject-v4"
	v6RoutePolicy   = "route-policy-v6"
	v6Statement     = "statement-v6"
	rejectV6PfxName = "reject-v6"
	bmpStationPort  = 7039
	statsTimeout    = 60
	bmpServerName   = "bmpStation"
	rejectedV4Net   = "172.16.1.0"
	rejectedV6Net   = "2001:DB8:2::"
	host1IPv4Start  = "198.51.100.0"
	host1IPv6Start  = "2001:db8:100::"
	host2IPv4Start  = "198.51.110.0"
	host2IPv6Start  = "2001:db8:110::"
	hostIPv4PfxLen  = 24
	hostIPv6PfxLen  = 64
	flowCount       = 1
	ecmpMaxPath     = 4
	allowAllPolicy  = "ALLOWAll"
)

var (
	dutPort1 = attrs.Attributes{Desc: "DUT to ATE Port 1", IPv4: "192.0.2.1", IPv6: "2001:db8:2::1", MAC: "02:00:01:02:02:02", IPv4Len: plenIPv4, IPv6Len: plenIPv6}
	atePort1 = attrs.Attributes{Name: "atePort1", IPv4: "192.0.2.2", IPv6: "2001:db8:2::2", MAC: "02:00:01:01:01:01", IPv4Len: plenIPv4, IPv6Len: plenIPv6}
	dutPort2 = attrs.Attributes{Desc: "DUT to ATE Port 2", IPv4: "192.0.3.1", IPv6: "2001:db8:3::1", MAC: "02:00:02:02:02:02", IPv4Len: plenIPv4, IPv6Len: plenIPv6}
	atePort2 = attrs.Attributes{Name: "atePort2", IPv4: "192.0.3.2", IPv6: "2001:db8:3::2", MAC: "02:00:02:01:01:01", IPv4Len: plenIPv4, IPv6Len: plenIPv6}

	advertisedIPv4    ipAddr = ipAddr{address: host1IPv4Start, prefix: hostIPv4PfxLen}
	advertisedIPv6    ipAddr = ipAddr{address: host1IPv6Start, prefix: hostIPv6PfxLen}
	nonAdvertisedIPv4 ipAddr = ipAddr{address: rejectedV4Net, prefix: hostIPv4PfxLen}
	nonAdvertisedIPv6 ipAddr = ipAddr{address: rejectedV6Net, prefix: hostIPv6PfxLen}
)

type ipAddr struct {
	address string
	prefix  uint32
}

func (ip *ipAddr) cidr(t *testing.T) string {
	_, net, err := net.ParseCIDR(fmt.Sprintf("%s/%d", ip.address, ip.prefix))
	if err != nil {
		t.Fatal(err)
	}
	return net.String()
}

// TestMain is the entry point for the test suite.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestBMPPostPolicy(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	t.Log("Start DUT Configuration")
	b := configureDUT(t, dut)
	cfgplugins.ConfigureRoutePolicyAllow(t, dut, b, allowAllPolicy, oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	t.Log("Start ATE Configuration")
	otgConfig := configureATE(t, ate)
	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)
	defer ate.OTG().StopProtocols(t)
	t.Log("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut)

	t.Log("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, ate)
	cfgV4Params := cfgplugins.NonMatchRoutingParams{
		RoutePolicyName: v4RoutePolicy,
		PolicyStatement: v4Statement,
		PrefixSetName:   rejectV4PfxName,
		NonAdvertisedIP: nonAdvertisedIPv4.cidr(t),
	}
	cfgplugins.NonMatchingPrefixRoutePolicy(t, dut, b, cfgV4Params)
	cfgV6Params := cfgplugins.NonMatchRoutingParams{
		RoutePolicyName: v6RoutePolicy,
		PolicyStatement: v6Statement,
		PrefixSetName:   rejectV6PfxName,
		NonAdvertisedIP: nonAdvertisedIPv6.cidr(t),
	}
	cfgplugins.NonMatchingPrefixRoutePolicy(t, dut, b, cfgV6Params)

	t.Log("Verify BMP Session Establishment on DUT")
	verifyBMPTelemetry(t, dut)

	t.Log("Verify BMP session on ATE")
	verifyATEBMP(t, ate)

	t.Log("Verify BMP route monitoring for post-policy")
	verifyBMPPostPolicyRouteMonitoring(t, ate)
}

// configureDUT configures all DUT aspects.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) *gnmi.SetBatch {
	t.Helper()
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	batch := &gnmi.SetBatch{}
	gnmi.BatchReplace(batch, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.BatchReplace(batch, gnmi.OC().Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
	cfgBGP := cfgplugins.BGPConfig{DutAS: dutAS, RouterID: dutPort1.IPv4, ECMPMaxPath: ecmpMaxPath}
	dutBgpConf := cfgplugins.ConfigureDUTBGP(t, dut, batch, cfgBGP)
	configureDUTBGPNeighbors(t, dut, batch, dutBgpConf.Bgp)
	bmpParams := cfgplugins.BMPConfigParams{
		DutAS:        dutAS,
		BGPObj:       dutBgpConf.Bgp, // required for OpenConfig path
		LocalAddr:    dutPort2.IPv4,  // local source address for BMP
		StationPort:  bmpStationPort,
		StationAddr:  atePort2.IPv4,
		StatsTimeOut: statsTimeout,
	}
	cfgplugins.ConfigureBMP(t, dut, batch, bmpParams)
	batch.Set(t, dut)
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	return batch
}

// configureDUTBGPNeighbors appends multiple BGP neighbor configurations to an existing BGP protocol on the DUT. Instead of calling AppendBGPNeighbor repeatedly in the test, this helper iterates over a slice of BGPNeighborConfig and applies each neighbor configuration into the given gnmi.SetBatch.
func configureDUTBGPNeighbors(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, bgp *oc.NetworkInstance_Protocol_Bgp) {
	t.Helper()
	// Add BGP neighbors
	neighbors := []cfgplugins.BGPNeighborConfig{
		{
			AteAS:        ate1AS,
			PortName:     dutPort1.Name,
			NeighborIPv4: atePort1.IPv4,
			NeighborIPv6: atePort1.IPv6,
			IsLag:        false,
		},
		{
			AteAS:        ate2AS,
			PortName:     dutPort2.Name,
			NeighborIPv4: atePort2.IPv4,
			NeighborIPv6: atePort2.IPv6,
			IsLag:        false,
		},
	}
	for _, n := range neighbors {
		cfgplugins.AppendBGPNeighbor(t, dut, batch, bgp, n)
	}
}

// configureATE builds and returns the OTG configuration for the ATE topology.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	ateConfig := gosnappi.NewConfig()

	// Create ATE Ports
	ateP1 := ate.Port(t, "port1")
	ateP2 := ate.Port(t, "port2")

	// First, define OTG ports
	atePrt1 := ateConfig.Ports().Add().SetName(ateP1.ID())
	atePrt2 := ateConfig.Ports().Add().SetName(ateP2.ID())
	// ATE Device 1 (EBGP)
	configureATEDevice(t, ateConfig, atePrt1, atePort1, dutPort1, ate1AS, host1IPv4Start, host1IPv6Start)
	// ATE Device 2 (EBGP)
	configureATEDevice(t, ateConfig, atePrt2, atePort2, dutPort2, ate2AS, host2IPv4Start, host2IPv6Start)
	return ateConfig
}

// configureATEDevice configures the ports along with the associated protocols.
func configureATEDevice(t *testing.T, cfg gosnappi.Config, port gosnappi.Port, atePort, dutPort attrs.Attributes, asn uint32, hostPrefixV4, hostPrefixV6 string) {
	t.Helper()
	var peerTypeV4 gosnappi.BgpV4PeerAsTypeEnum
	var peerTypeV6 gosnappi.BgpV6PeerAsTypeEnum

	dev := cfg.Devices().Add().SetName(atePort.Name)
	eth := dev.Ethernets().Add().SetName(atePort.Name + "Eth").SetMac(atePort.MAC)
	eth.Connection().SetPortName(port.Name())

	ip4 := eth.Ipv4Addresses().Add().SetName(atePort.Name + ".IPv4")
	ip4.SetAddress(atePort.IPv4).SetGateway(dutPort.IPv4).SetPrefix(uint32(atePort.IPv4Len))

	ip6 := eth.Ipv6Addresses().Add().SetName(atePort.Name + ".IPv6")
	ip6.SetAddress(atePort.IPv6).SetGateway(dutPort.IPv6).SetPrefix(uint32(atePort.IPv6Len))

	bgp := dev.Bgp().SetRouterId(atePort.IPv4)
	peerTypeV4 = gosnappi.BgpV4PeerAsType.EBGP
	peerTypeV6 = gosnappi.BgpV6PeerAsType.EBGP

	bgpV4 := bgp.Ipv4Interfaces().Add().SetIpv4Name(ip4.Name())
	v4Peer := bgpV4.Peers().Add().SetName(atePort.Name + ".BGPv4.Peer").SetPeerAddress(dutPort.IPv4).SetAsNumber(asn).SetAsType(peerTypeV4)

	bgpV6 := bgp.Ipv6Interfaces().Add().SetIpv6Name(ip6.Name())
	v6Peer := bgpV6.Peers().Add().SetName(atePort.Name + ".BGPv6.Peer").SetPeerAddress(dutPort.IPv6).SetAsNumber(asn).SetAsType(peerTypeV6)

	// Advertise host routes
	addBGPRoutes(v4Peer.V4Routes().Add(), atePort.Name+".Host.v4", hostPrefixV4, hostIPv4PfxLen, flowCount, ip4.Address())
	addBGPRoutes(v6Peer.V6Routes().Add(), atePort.Name+".Host.v6", hostPrefixV6, hostIPv6PfxLen, flowCount, ip6.Address())
	// // --- BMP Configuration ---
	// TODO: Currently BMP configuration not yet support, uncomment the code once implemented.
	// bmp := dev.Bgp().Bmp().Servers().Add().SetName("BMP_SERVER")
	// bmp.SetPort(bmpStationPort)    // match DUT BMP port
	// bmp.SetAddress(ip4.Address())  // ATE listening address
	// bmp.SetKeepalive(statsTimeout) // optional, in seconds
}

// addBGPRoutes adds BGP route advertisements to an ATE device.
func addBGPRoutes[R any](routes R, name, startAddress string, prefixLen, count uint32, nextHop string) {
	switch r := any(routes).(type) {
	case gosnappi.BgpV4RouteRange:
		r.SetName(name).SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL).SetNextHopIpv4Address(nextHop)
		r.Addresses().Add().SetAddress(startAddress).SetPrefix(prefixLen).SetCount(count)
	case gosnappi.BgpV6RouteRange:
		r.SetName(name).SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL).SetNextHopIpv6Address(nextHop)
		r.Addresses().Add().SetAddress(startAddress).SetPrefix(prefixLen).SetCount(count)
	}
}

func verifyBMPTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	t.Log("Currently, BMP is not implemented; the below code will be uncommented once BMP support is available.")
	// // TODO: Currently, BMP is not implemented; the below code will be uncommented once BMP support is available.
	// bmpStationPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Global().Bmp().Station("BMP_STN1")

	// // --- Watch for BMP session to reach UP state ---
	// statusPath := bmpStationPath.ConnectionStatus().State()
	// _, ok := gnmi.Watch(t, dut, statusPath, 2*time.Minute, func(val *ygnmi.Value[oc.E_BgpTypes_BMPStationConnectionMode]) bool {
	// 	v, present := val.Val()
	// 	return present && v == oc.BgpTypes_BMPStationConnectionMode_ACTIVE
	// }).Await(t)

	// if !ok {
	// 	t.Fatalf("BMP session status is not UP. Got: %v", gnmi.Get(t, dut, statusPath))
	// }

	// // --- Retrieve complete BMP station state ---
	// state := gnmi.Get(t, dut, bmpStationPath.State())

	// // Verify local-address
	// if got := state.GetLocalAddress(); got != dutPort2.IPv4 {
	// 	t.Errorf("BMP Local Address mismatch: got %s, want %s", got, dutPort2.IPv4)
	// }

	// // Verify station address
	// if got := state.GetAddress(); got != atePort2.IPv4 {
	// 	t.Errorf("BMP Station Address mismatch: got %s, want %s", got, atePort2.IPv4)
	// }

	// // Verify station port
	// if got := state.GetPort(); got != bmpStationPort {
	// 	t.Errorf("BMP Station Port mismatch: got %d, want %d", got, bmpStationPort)
	// }

	// // Verify connection-status is UP
	// if got := state.GetConnectionStatus(); got != oc.BgpTypes_BMPStationConnectionStatus_UP {
	// 	t.Errorf("BMP connection status mismatch: got %v, want UP", got)
	// }

	// // Verify policy type (post-policy)
	// if got := state.GetPolicyType(); got != oc.BgpTypes_BMPPolicyType_POST_POLICY {
	// 	t.Errorf("BMP Policy Type mismatch: got %v, want POST_POLICY", got)
	// }

	// // Verify uptime > 0
	// if got := state.GetUptime(); got == 0 {
	// 	t.Errorf("BMP Uptime is 0, expected non-zero uptime")
	// }

	// t.Logf("BMP session telemetry verified successfully: LocalAddr=%s, Station=%s:%d, Status=UP, Policy=POST_POLICY", state.GetLocalAddress(), state.GetAddress(), state.GetPort())
}

func verifyATEBMP(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	t.Log("Currently, BMP is not implemented; the below code will be uncommented once BMP support is available.")
	// TODO: Currently, BMP is not implemented; the below code will be uncommented once BMP support is available.
	// otg := ate.OTG()

	// bmpPeerPath := gnmi.OTG().BgpPeer(atePort2.Name + ".BGPv4.Peer").Bmp().Server(bmpServerName).Peer(dutPort2.IPv4)
	// _, ok := gnmi.Watch(t, otg, bmpPeerPath.SessionState().State(), 2*time.Minute, func(val *ygnmi.Value[otgtelemetry.E_BmpPeer_SessionState]) bool {
	// 	state, ok := val.Val()
	// 	return ok && state == otgtelemetry.BmpPeer_SessionState_UP
	// }).Await(t)
	// if !ok {
	// 	fptest.LogQuery(t, "ATE BMP session state", bmpPeerPath.State(), gnmi.Get(t, otg, bmpPeerPath.State()))
	// 	t.Fatalf("ATE did not see BMP session as UP")
	// }
	// t.Log("ATE reports BMP session is UP")
}

func verifyBMPPostPolicyRouteMonitoring(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	t.Log("Currently, BMP is not implemented; the below code will be uncommented once BMP support is available.")
	// TODO: Currently, BMP is not implemented; the below code will be uncommented once BMP support is available.
	// otg := ate.OTG()
	// bmpPeerPath := gnmi.OTG().BgpPeer(atePort2.Name + ".BGPv4.Peer").Bmp().Server(bmpServerName).Peer(dutPort2.IPv4)

	// reportedV4 := gnmi.Get(t, otg, bmpPeerPath.UnicastPrefixesV4().Reported().State())
	// if reportedV4 != routeCount {
	// 	t.Errorf("Number of reported IPv4 prefixes at BMP station is incorrect: got %d, want %d", reportedV4, routeCount)
	// } else {
	// 	t.Logf("Successfully received %d IPv4 routes at BMP station as expected.", reportedV4)
	// }

	// reportedV6 := gnmi.Get(t, otg, bmpPeerPath.UnicastPrefixesV6().Reported().State())
	// if reportedV6 != routeCount {
	// 	t.Errorf("Number of reported IPv6 prefixes at BMP station is incorrect: got %d, want %d", reportedV6, routeCount)
	// } else {
	// 	t.Logf("Successfully received %d IPv6 routes at BMP station as expected.", reportedV6)
	// }
}
