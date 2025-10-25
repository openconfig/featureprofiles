package bmp_base_session_test

import (
	"fmt"
	"net"
	"testing"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	dutAS          = 64520
	ate1AS         = 64530
	ate2AS         = 64531
	plenIPv4       = 30
	plenIPv6       = 126
	bmpStationIP   = "10.23.15.58"
	bmpStationPort = 7039
	statsTimeout   = 60
	host1IPv4Start = "192.168.0.0"
	host1IPv6Start = "2001:db8:100::"
	host2IPv4Start = "10.200.0.0"
	host2IPv6Start = "2001:db8:110::"
	hostIPv4PfxLen = 24
	hostIPv6PfxLen = 64
	flowCount      = 1
)

var (
	dutP1 = attrs.Attributes{
		Desc:    "DUT to ATE Port 1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8:2::1",
		MAC:     "02:00:01:02:02:02",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	ateP1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8:2::2",
		MAC:     "02:00:01:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	dutP2 = attrs.Attributes{
		Desc:    "DUT to ATE Port 2",
		IPv4:    "192.0.3.1",
		IPv6:    "2001:db8:3::1",
		MAC:     "02:00:02:02:02:02",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	ateP2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "192.0.3.2",
		IPv6:    "2001:db8:3::2",
		MAC:     "02:00:02:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
)

type ipAddr struct {
	address string
	prefix  uint32
}

type ateConfigParams struct {
	atePort       gosnappi.Port
	atePortAttrs  attrs.Attributes
	dutPortAttrs  attrs.Attributes
	ateAS         uint32
	hostIPv4Start string
	hostIPv6Start string
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

// configureDUT configures all DUT aspects.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) *gnmi.SetBatch {
	t.Helper()
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	bmpConfigParams := cfgplugins.BMPConfigParams{
		DutAS:        dutAS,
		StationPort:  bmpStationPort,
		StationAddr:  bmpStationIP,
		StatsTimeOut: statsTimeout,
	}

	batch := &gnmi.SetBatch{}
	gnmi.BatchReplace(batch, gnmi.OC().Interface(p1.Name()).Config(), dutP1.NewOCInterface(p1.Name(), dut))
	gnmi.BatchReplace(batch, gnmi.OC().Interface(p2.Name()).Config(), dutP2.NewOCInterface(p2.Name(), dut))
	cfgBGP := cfgplugins.BGPConfig{DutAS: dutAS, RouterID: dutP1.IPv4}
	dutBgpConf := cfgplugins.ConfigureDUTBGP(t, dut, batch, cfgBGP)
	configureDUTBGPNeighbors(t, dut, batch, dutBgpConf.Bgp)
	cfgplugins.ConfigureBMP(t, dut, batch, bmpConfigParams)
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
			PortName:     dutP1.Name,
			NeighborIPv4: ateP1.IPv4,
			NeighborIPv6: ateP1.IPv6,
			IsLag:        false,
		},
		{
			AteAS:        ate2AS,
			PortName:     dutP2.Name,
			NeighborIPv4: ateP2.IPv4,
			NeighborIPv6: ateP2.IPv6,
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
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")

	// First, define OTG ports
	atePort1 := ateConfig.Ports().Add().SetName(p1.ID())
	atePort2 := ateConfig.Ports().Add().SetName(p2.ID())

	ateP1ConfigParams := ateConfigParams{
		atePort:       atePort1,
		atePortAttrs:  ateP1,
		dutPortAttrs:  dutP1,
		ateAS:         ate1AS,
		hostIPv4Start: host1IPv4Start,
		hostIPv6Start: host1IPv6Start,
	}

	ateP2ConfigParams := ateConfigParams{
		atePort:       atePort2,
		atePortAttrs:  ateP2,
		dutPortAttrs:  dutP2,
		ateAS:         ate2AS,
		hostIPv4Start: host2IPv4Start,
		hostIPv6Start: host2IPv6Start,
	}

	// ATE Device 1 (EBGP)
	configureATEDevice(t, ateConfig, ateP1ConfigParams)
	// ATE Device 2 (EBGP)
	configureATEDevice(t, ateConfig, ateP2ConfigParams)
	return ateConfig
}

// configureATEDevice configures the ports along with the associated protocols.
func configureATEDevice(t *testing.T, cfg gosnappi.Config, params ateConfigParams) {
	t.Helper()
	var peerTypeV4 gosnappi.BgpV4PeerAsTypeEnum
	var peerTypeV6 gosnappi.BgpV6PeerAsTypeEnum

	dev := cfg.Devices().Add().SetName(params.atePortAttrs.Name)
	eth := dev.Ethernets().Add().SetName(params.atePortAttrs.Name + "Eth").SetMac(params.atePortAttrs.MAC)
	eth.Connection().SetPortName(params.atePort.Name())

	ip4 := eth.Ipv4Addresses().Add().SetName(params.atePortAttrs.Name + ".IPv4")
	ip4.SetAddress(params.atePortAttrs.IPv4).SetGateway(params.dutPortAttrs.IPv4).SetPrefix(uint32(params.atePortAttrs.IPv4Len))

	ip6 := eth.Ipv6Addresses().Add().SetName(params.atePortAttrs.Name + ".IPv6")
	ip6.SetAddress(params.atePortAttrs.IPv6).SetGateway(params.dutPortAttrs.IPv6).SetPrefix(uint32(params.atePortAttrs.IPv6Len))

	bgp := dev.Bgp().SetRouterId(params.atePortAttrs.IPv4)
	peerTypeV4 = gosnappi.BgpV4PeerAsType.EBGP
	peerTypeV6 = gosnappi.BgpV6PeerAsType.EBGP

	bgpV4 := bgp.Ipv4Interfaces().Add().SetIpv4Name(ip4.Name())
	v4Peer := bgpV4.Peers().Add().SetName(params.atePortAttrs.Name + ".BGPv4.Peer").SetPeerAddress(params.dutPortAttrs.IPv4).SetAsNumber(params.ateAS).SetAsType(peerTypeV4)

	bgpV6 := bgp.Ipv6Interfaces().Add().SetIpv6Name(ip6.Name())
	v6Peer := bgpV6.Peers().Add().SetName(params.atePortAttrs.Name + ".BGPv6.Peer").SetPeerAddress(params.dutPortAttrs.IPv6).SetAsNumber(params.ateAS).SetAsType(peerTypeV6)

	// Advertise host routes
	addBGPRoutes(v4Peer.V4Routes().Add(), params.atePortAttrs.Name+".Host.v4", params.hostIPv4Start, hostIPv4PfxLen, flowCount, ip4.Address())
	addBGPRoutes(v6Peer.V6Routes().Add(), params.atePortAttrs.Name+".Host.v6", params.hostIPv6Start, hostIPv6PfxLen, flowCount, ip6.Address())
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

func verifyBMPSessionOnDUT(t *testing.T) {
	t.Helper()
	t.Log("Currently, BMP is not implemented on OTG yet; the below code will be uncommented once BMP support is available.")
	// // TODO: Currently, BMP is not implemented on OTG yet; the below code will be uncommented once BMP support is available.
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
	// 	t.Errorf("bmp Local Address mismatch: got %s, want %s", got, dutPort2.IPv4)
	// }

	// // Verify station address
	// if got := state.GetAddress(); got != atePort2.IPv4 {
	// 	t.Errorf("bmp Station Address mismatch: got %s, want %s", got, atePort2.IPv4)
	// }

	// // Verify station port
	// if got := state.GetPort(); got != bmpStationPort {
	// 	t.Errorf("bmp Station Port mismatch: got %d, want %d", got, bmpStationPort)
	// }

	// // Verify connection-status is UP
	// if got := state.GetConnectionStatus(); got != oc.BgpTypes_BMPStationConnectionStatus_UP {
	// 	t.Errorf("bmp connection status mismatch: got %v, want UP", got)
	// }

	// // Verify policy type (post-policy)
	// if got := state.GetPolicyType(); got != oc.BgpTypes_BMPPolicyType_POST_POLICY {
	// 	t.Errorf("bmp Policy Type mismatch: got %v, want POST_POLICY", got)
	// }

	// // Verify uptime > 0
	// if got := state.GetUptime(); got == 0 {
	// 	t.Errorf("bmp Uptime is 0, expected non-zero uptime")
	// }

	// t.Logf("bmp session telemetry verified successfully: LocalAddr=%s, Station=%s:%d, Status=UP, Policy=POST_POLICY", state.GetLocalAddress(), state.GetAddress(), state.GetPort())
}

func verifyBMPSessionOnATE(t *testing.T) {
	t.Helper()
	t.Log("Currently, BMP is not implemented on OTG yet; the below code will be uncommented once BMP support is available.")
	// TODO: Currently, BMP is not implemented on OTG yet; the below code will be uncommented once BMP support is available.
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

func verifyBMPStatisticsReporting(t *testing.T) {
	t.Helper()
	t.Log("Currently, BMP is not implemented on OTG yet; the below code will be uncommented once BMP support is available.")
}

func verifyBMPPostPolicyRouteMonitoring(t *testing.T) {
	t.Helper()
	t.Log("Currently, BMP is not implemented on OTG yet; the below code will be uncommented once BMP support is available.")
	// TODO: Currently, BMP is not implemented on OTG yet; the below code will be uncommented once BMP support is available.
	// otg := ate.OTG()
	// bmpPeerPath := gnmi.OTG().BgpPeer(atePort2.Name + ".BGPv4.Peer").Bmp().Server(bmpServerName).Peer(dutPort2.IPv4)

	// reportedV4 := gnmi.Get(t, otg, bmpPeerPath.UnicastPrefixesV4().Reported().State())
	// if reportedV4 != routeCount {
	// 	t.Errorf("number of reported IPv4 prefixes at BMP station is incorrect: got %d, want %d", reportedV4, routeCount)
	// } else {
	// 	t.Logf("Successfully received %d IPv4 routes at BMP station as expected.", reportedV4)
	// }

	// reportedV6 := gnmi.Get(t, otg, bmpPeerPath.UnicastPrefixesV6().Reported().State())
	// if reportedV6 != routeCount {
	// 	t.Errorf("number of reported IPv6 prefixes at BMP station is incorrect: got %d, want %d", reportedV6, routeCount)
	// } else {
	// 	t.Logf("Successfully received %d IPv6 routes at BMP station as expected.", reportedV6)
	// }
}

func TestBMPBaseSession(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	t.Log("Start DUT Configuration")
	configureDUT(t, dut)
	t.Log("Start ATE Configuration")
	otgConfig := configureATE(t, ate)
	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)

	t.Log("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut)

	t.Log("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, ate)

	type testCase struct {
		name string
		fn   func(t *testing.T)
	}

	cases := []testCase{
		{
			name: "1.1.1_Verify_BMP_Session_Establishment",
			fn: func(t *testing.T) {
				t.Log("Verify BMP session on DUT")
				verifyBMPSessionOnDUT(t)

				t.Log("Verify BMP session on ATE")
				verifyBMPSessionOnATE(t)
			},
		},
		{
			name: "1.1.2_Verify_Statisitics_Reporting",
			fn: func(t *testing.T) {
				t.Log("Verify BMP session on DUT")
				verifyBMPStatisticsReporting(t)
			},
		},
		{
			name: "1.1.3_Verify_Route_Monitoring_Post_Policy",
			fn: func(t *testing.T) {
				t.Log("Verify Route Monitoring Post Policy on DUT")
				verifyBMPPostPolicyRouteMonitoring(t)
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tc.fn(t)
		})
	}
}
