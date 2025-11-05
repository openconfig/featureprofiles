package bmp_pre_policy_test

import (
	"fmt"
	"testing"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

const (
	dutAS             = 64520
	ateAS             = 64530
	bmpStationIP      = "10.23.15.58"
	bmpStationPort    = 7039
	bmpStationName    = "BMP_STATION_1"
	statisticsTimeout = 60
	ecmpMaxPath       = 4
	port1             = "port1"
	port2             = "port2"
	v4Routes          = "bgp-v4-routes"
	v6Routes          = "bgp-v6-routes"
	v4RouteCount      = 15000000 / 10000
	v6RouteCount      = 5000000 / 10000
	ipv4Prefix        = "172.16.0.0"
	ipv4PrefixLen     = 16
	ipv6Prefix        = "2001:DB8::"
	ipv6PrefixLen     = 32
	v4RoutePolicy     = "route-policy-v4"
	v4Statement       = "statement-v4"
	rejectV4PfxName   = "reject-v4"
	v6RoutePolicy     = "route-policy-v6"
	v6Statement       = "statement-v6"
	rejectV6PfxName   = "reject-v6"
	rejectedV4Net     = "172.16.0.0"
	rejectedV6Net     = "2001:DB8::"
	hostIPv4PfxLen    = "16..32"
	hostIPv6PfxLen    = "32..128"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT Port 1",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
		IPv6:    "2001:db8::1",
		IPv6Len: 126,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT Port 2",
		IPv4:    "192.0.2.5",
		IPv4Len: 30,
		IPv6:    "2001:db8::5",
		IPv6Len: 126,
	}
	atePort1 = attrs.Attributes{
		Desc:    "ATE Port 1",
		Name:    port1,
		IPv4:    "192.0.2.2",
		IPv4Len: 30,
		MAC:     "00:01:12:00:00:01",
		IPv6:    "2001:db8::2",
		IPv6Len: 126,
	}
	atePort2 = attrs.Attributes{
		Desc:    "ATE Port 2",
		Name:    port2,
		IPv4:    "192.0.2.6",
		IPv4Len: 30,
		MAC:     "00:01:12:00:00:02",
		IPv6:    "2001:db8::6",
		IPv6Len: 126,
	}

	defaultNIName string
)

type testCase struct {
	name string
	run  func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice)
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configureDUT configures all DUT aspects.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p1 := dut.Port(t, port1)
	p2 := dut.Port(t, port2)

	batch := &gnmi.SetBatch{}
	defaultNIName = deviations.DefaultNetworkInstance(dut)

	t.Logf("Configuring Network Instance %s", defaultNIName)
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	t.Logf("Configuring Interfaces")
	gnmi.BatchReplace(batch, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.BatchReplace(batch, gnmi.OC().Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))

	t.Logf("Configuring BGP")
	cfgBGP := cfgplugins.BGPConfig{
		DutAS:       dutAS,
		RouterID:    dutPort1.IPv4,
		ECMPMaxPath: ecmpMaxPath,
	}
	dutBgpConf := cfgplugins.ConfigureDUTBGP(t, dut, batch, cfgBGP)
	neighbor := cfgplugins.BGPNeighborConfig{
		AteAS:        ateAS,
		PortName:     dutPort1.Name,
		NeighborIPv4: atePort1.IPv4,
		NeighborIPv6: atePort1.IPv6,
	}
	cfgplugins.AppendBGPNeighbor(t, dut, batch, dutBgpConf.Bgp, neighbor)

	t.Logf("Configuring BMP")
	bmpConfig := cfgplugins.BMPConfigParams{
		DutAS:          dutAS,
		StationPort:    bmpStationPort,
		StationAddr:    bmpStationIP,
		ConnectionMode: cfgplugins.BMPConnectionTypeActive,
		StationName:    bmpStationName,
		StatsTimeOut:   statisticsTimeout,
		PolicyType:     cfgplugins.BMPPrePolicyType,
	}
	cfgplugins.ConfigureBMP(t, dut, batch, bmpConfig)

	cfgV4Params := cfgplugins.NonMatchRoutingParams{
		RoutePolicyName: v4RoutePolicy,
		PolicyStatement: v4Statement,
		PrefixSetName:   rejectV4PfxName,
		NonAdvertisedIP: fmt.Sprintf("%s/%d", rejectedV4Net, ipv4PrefixLen),
		MaskLenExact:    hostIPv4PfxLen,
		IPType:          cfgplugins.IPv4,
	}
	cfgplugins.NonMatchingPrefixRoutePolicy(t, dut, batch, cfgV4Params)
	cfgV6Params := cfgplugins.NonMatchRoutingParams{
		RoutePolicyName: v6RoutePolicy,
		PolicyStatement: v6Statement,
		PrefixSetName:   rejectV6PfxName,
		NonAdvertisedIP: fmt.Sprintf("%s/%d", rejectedV6Net, ipv6PrefixLen),
		MaskLenExact:    hostIPv6PfxLen,
		IPType:          cfgplugins.IPv6,
	}
	cfgplugins.NonMatchingPrefixRoutePolicy(t, dut, batch, cfgV6Params)
	batch.Set(t, dut)
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	top := gosnappi.NewConfig()
	ap1 := ate.Port(t, port1)
	ap2 := ate.Port(t, port2)

	d1 := atePort1.AddToOTG(top, ap1, &dutPort1)
	atePort2.AddToOTG(top, ap2, &dutPort2)

	ip1 := d1.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	ip1v6 := d1.Ethernets().Items()[0].Ipv6Addresses().Items()[0]

	bgp := d1.Bgp().SetRouterId(atePort1.IPv4)
	bgpPeer := bgp.Ipv4Interfaces().Add().SetIpv4Name(ip1.Name()).Peers().Add().SetName(fmt.Sprintf("%s.v4.BGP.peer", d1.Name()))
	bgpPeer.SetPeerAddress(dutPort2.IPv4).SetAsNumber(uint32(ateAS)).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)

	bgpNetv4 := bgpPeer.V4Routes().Add().SetName(v4Routes)
	bgpNetv4.SetNextHopIpv4Address(ip1.Address())
	bgpNetv4.Addresses().Add().SetAddress(ipv4Prefix).SetPrefix(ipv4PrefixLen).SetStep(1).SetCount(v4RouteCount)

	bgpPeerv6 := bgp.Ipv6Interfaces().Add().SetIpv6Name(ip1v6.Name()).Peers().Add().SetName(fmt.Sprintf("%s.v6.BGP.peer", d1.Name()))
	bgpPeerv6.SetPeerAddress(dutPort2.IPv6).SetAsNumber(uint32(ateAS)).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

	bgpNetv6 := bgpPeerv6.V6Routes().Add().SetName(v6Routes)
	bgpNetv6.SetNextHopIpv6Address(ip1v6.Address())
	bgpNetv6.Addresses().Add().SetAddress(ipv6Prefix).SetPrefix(ipv6PrefixLen).SetStep(1).SetCount(v6RouteCount)

	return top
}

func TestBMPPrePolicy(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	t.Log("Configuring DUT")
	configureDUT(t, dut)

	t.Log("Configuring ATE")
	config := configureATE(t, ate)
	otg := ate.OTG()
	otg.PushConfig(t, config)
	otg.StartProtocols(t)
	defer ate.OTG().StopProtocols(t)

	otgutils.WaitForARP(t, ate.OTG(), config, cfgplugins.IPv4)
	otgutils.WaitForARP(t, ate.OTG(), config, cfgplugins.IPv6)

	t.Log("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut)

	t.Log("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, ate)

	testCases := []testCase{
		{
			name: "Verify BMP session establishment",
			run: func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
				verifyBMPTelemetry(t, dut)
				verifyATEBMP(t, ate)
			},
		},
		{
			name: "Verify route monitoring with pre-policy and exclude-noneligible",
			run:  verifyBMPPrePolicyRouteMonitoring,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.run == nil {
				t.Fatalf("Nothing to run for testcase %s", tc.name)
			}
			tc.run(t, dut, ate)
		})
	}
}

func verifyBMPTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	t.Errorf("Currently, BMP is not implemented on %s %s; the below code will be uncommented once BMP support is available.", dut.Vendor(), dut.Model())
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

	// // Verify policy type (pre-policy)
	// if got := state.GetPolicyType(); got != oc.BgpTypes_BMPPolicyType_PRE_POLICY {
	// 	t.Errorf("BMP Policy Type mismatch: got %v, want PRE_POLICY", got)
	// }

	// // Verify uptime > 0
	// if got := state.GetUptime(); got == 0 {
	// 	t.Errorf("BMP Uptime is 0, expected non-zero uptime")
	// }

	// t.Logf("BMP session telemetry verified successfully: LocalAddr=%s, Station=%s:%d, Status=UP, Policy=PRE_POLICY", state.GetLocalAddress(), state.GetAddress(), state.GetPort())
}

func verifyATEBMP(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	t.Errorf("Currently, BMP is not implemented on %s %s; the below code will be uncommented once BMP support is available.", ate.Vendor(), ate.Model())
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

func verifyBMPPrePolicyRouteMonitoring(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	t.Helper()
	t.Errorf("Currently, BMP is not implemented on %s %s and %s %s; the below code will be uncommented once BMP support is available.", ate.Vendor(), ate.Model(), dut.Vendor(), dut.Model())
	// TODO: Currently, BMP is not implemented; the below code will be uncommented once BMP support is available.
	// otg := ate.OTG()
	// bmpPeerPath := gnmi.OTG().BgpPeer(atePort2.Name + ".BGPv4.Peer").Bmp().Server(bmpServerName).Peer(dutPort2.IPv4)

	// reportedV4 := gnmi.Get(t, otg, bmpPeerPath.UnicastPrefixesV4().Reported().State())
	// if reportedV4 != v4RouteCount {
	// 	t.Errorf("Number of reported IPv4 prefixes at BMP station is incorrect: got %d, want %d", reportedV4, v4RouteCount)
	// } else {
	// 	t.Logf("Successfully received %d IPv4 routes at BMP station as expected.", reportedV4)
	// }

	// reportedV6 := gnmi.Get(t, otg, bmpPeerPath.UnicastPrefixesV6().Reported().State())
	// if reportedV6 != v6RouteCount {
	// 	t.Errorf("Number of reported IPv6 prefixes at BMP station is incorrect: got %d, want %d", reportedV6, v6RouteCount)
	// } else {
	// 	t.Logf("Successfully received %d IPv6 routes at BMP station as expected.", reportedV6)
	// }
}
