// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tunnel_interface_based_ipv4_gre_encapsulation_test

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otg "github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ipv4PrefixLen     = 24
	pps               = 100
	FrameSize         = 512
	aclName           = "f1"
	termName          = "t1"
	EncapSrcMatch     = "192.0.2.2"
	EncapDstMatch     = "192.0.2.6"
	count             = "GreFilterCount"
	greTunnelEndpoint = "TunnelEncapIpv4"
	greSrcAddr        = "198.51.100.1"
	greDstAddr        = "203.0.113.1/32"
	dscp              = 8
	CorrespondingTOS  = 32
	GreProtocol       = 47
	Tunnelaction      = "TunnelEncapIpv4"
	plenIPv4          = 30
	tolerance         = 50
	lossTolerance     = 2
	prefix            = "0.0.0.0/0"
	nexthop           = "192.0.2.6"
)

var (
	dutSrc = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.1",
		IPv4Len: plenIPv4,
	}
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: plenIPv4,
	}
	dutDst = attrs.Attributes{
		Desc:    "DUT to ATE destination",
		IPv4:    "192.0.2.5",
		IPv4Len: plenIPv4,
	}
	ateDst = attrs.Attributes{
		Name:    "atedst",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv4Len: plenIPv4,
	}
)

type parameters struct {
	rtIntf1Ipv4Add  string
	rtIntf2Ipv4Add  string
	rtIntf5Ipv4Add  string
	rtIntf6Ipv4Add  string
	rtIntf1MacAdd   string
	rtIntf2MacAdd   string
	rtIntf5MacAdd   string
	rtIntf6MacAdd   string
	r0Intf1Ipv4Add  string
	r0Intf2Ipv4Add  string
	r0Intf3Ipv4Add  string
	r0Intf4Ipv4Add  string
	r0Fti0Ipv4Add   string
	r0Fti1Ipv4Add   string
	r0Fti2Ipv4Add   string
	r0Fti3Ipv4Add   string
	r0Fti4Ipv4Add   string
	r0Fti5Ipv4Add   string
	r0Fti6Ipv4Add   string
	r0Fti7Ipv4Add   string
	r0Lo0Ut0Ipv4Add string
	r0Lo0Ut1Ipv4Add string
	r0Lo0Ut2Ipv4Add string
	r0Lo0Ut3Ipv4Add string
	ipv4Mask        uint8
	ipv4FullMask    uint8
	r1Intf5Ipv4Add  string
	r1Intf6Ipv4Add  string
	r1Intf3Ipv4Add  string
	r1Intf4Ipv4Add  string
	r1Fti0Ipv4Add   string
	r1Fti1Ipv4Add   string
	r1Fti2Ipv4Add   string
	r1Fti3Ipv4Add   string
	r1Fti4Ipv4Add   string
	r1Fti5Ipv4Add   string
	r1Fti6Ipv4Add   string
	r1Fti7Ipv4Add   string
	r1Lo0Ut0Ipv4Add string
	r1Lo0Ut1Ipv4Add string
	r1Lo0Ut2Ipv4Add string
	r1Lo0Ut3Ipv4Add string
	flow1           string
	flow2           string
	flow3           string
	flow4           string
	trafficDuration int64
	trafficRate     int64
}

func TestFtiTunnels(t *testing.T) {

	p := &parameters{
		rtIntf1Ipv4Add:  "198.51.100.2",
		rtIntf2Ipv4Add:  "198.51.100.3",
		rtIntf5Ipv4Add:  "198.51.100.5",
		rtIntf6Ipv4Add:  "198.51.100.6",
		rtIntf1MacAdd:   "00:00:aa:aa:aa:aa",
		rtIntf2MacAdd:   "00:00:bb:bb:bb:bb",
		rtIntf5MacAdd:   "00:00:cc:cc:cc:cc",
		rtIntf6MacAdd:   "00:00:dd:dd:dd:dd",
		r0Intf1Ipv4Add:  "198.51.100.1",
		r0Intf2Ipv4Add:  "198.51.100.4",
		r0Intf3Ipv4Add:  "198.51.100.7",
		r0Intf4Ipv4Add:  "198.51.100.8",
		r0Fti0Ipv4Add:   "198.51.100.9",
		r0Fti1Ipv4Add:   "198.51.100.10",
		r0Fti2Ipv4Add:   "198.51.100.11",
		r0Fti3Ipv4Add:   "198.51.100.12",
		r0Fti4Ipv4Add:   "198.51.100.13",
		r0Fti5Ipv4Add:   "198.51.100.14",
		r0Fti6Ipv4Add:   "198.51.100.15",
		r0Fti7Ipv4Add:   "198.51.100.16",
		r0Lo0Ut0Ipv4Add: "198.51.100.17",
		r0Lo0Ut1Ipv4Add: "198.51.100.18",
		r0Lo0Ut2Ipv4Add: "198.51.100.19",
		r0Lo0Ut3Ipv4Add: "198.51.100.20",
		ipv4Mask:        24,
		ipv4FullMask:    32,
		flow1:           "IPv4-flow1",
		flow2:           "IPv4-flow2",
		trafficDuration: 60,
		trafficRate:     1000,
	}

	dut1 := ondatra.DUT(t, "dut")
	d1p1 := dut1.Port(t, "port1")
	d1p2 := dut1.Port(t, "port2")
	d1p3 := dut1.Port(t, "port3")
	d1p4 := dut1.Port(t, "port4")

	rt := ondatra.ATE(t, "ate")
	rt1 := rt.Port(t, "port1")
	rt2 := rt.Port(t, "port2")
	rt3 := rt.Port(t, "port3")

	t.Run("Configure DUT ", func(t *testing.T) {
		ConfigureTunnelEncapDUT(t, p, dut1, d1p1, d1p2, d1p3, d1p4)
	})

	t.Run("Configure loopback interface on dut1 and dut2 ", func(t *testing.T) {
		// configure addtional loop address by native cli configuration.
		ConfigureLoobackInterfaceWithIPv4address(t, p.r0Lo0Ut1Ipv4Add, dut1)
		ConfigureLoobackInterfaceWithIPv4address(t, p.r0Lo0Ut2Ipv4Add, dut1)
		ConfigureLoobackInterfaceWithIPv4address(t, p.r0Lo0Ut3Ipv4Add, dut1)

	})

	t.Run("Configure 8 tunnel interface on DUT ", func(t *testing.T) {
		// configure tunnel interface on dut1 - IPv4
		ConfigureTunnelInterface(t, "fti0", p.r0Lo0Ut0Ipv4Add, p.r1Lo0Ut0Ipv4Add, dut1)
		ConfigureTunnelInterface(t, "fti1", p.r0Lo0Ut1Ipv4Add, p.r1Lo0Ut1Ipv4Add, dut1)
		ConfigureTunnelInterface(t, "fti2", p.r0Lo0Ut2Ipv4Add, p.r1Lo0Ut2Ipv4Add, dut1)
		ConfigureTunnelInterface(t, "fti3", p.r0Lo0Ut3Ipv4Add, p.r1Lo0Ut3Ipv4Add, dut1)
	})
	// configure tunnel termination on dut1
	t.Run("Configure tunnel termination at underlay interface on dut1 and dut2", func(t *testing.T) {
		ConfigureTunnelTermination(t, d1p3, dut1)
		ConfigureTunnelTermination(t, d1p4, dut1)
	})
	//configure Network Instance for both dut
	t.Run("Configure routing instance on dut1 and dut2", func(t *testing.T) {
		configureNetworkInstance(t, dut1)
	})

	// underylay IPv4 static route to reach tunnel-destination at dut1
	t.Run("Configure underlay IPv4 static routes on dut1", func(t *testing.T) {
		ipv4Destination1 := GetNetworkAddress(t, p.r1Lo0Ut0Ipv4Add, int(p.ipv4Mask))
		ipv4Destination2 := GetNetworkAddress(t, p.r1Lo0Ut1Ipv4Add, int(p.ipv4Mask))
		ipv4Destination3 := GetNetworkAddress(t, p.r1Lo0Ut2Ipv4Add, int(p.ipv4Mask))
		ipv4Destination4 := GetNetworkAddress(t, p.r1Lo0Ut3Ipv4Add, int(p.ipv4Mask))
		// underlay static route Nexthops
		underlayIPv4NextHopDut1 := []string{p.r1Intf3Ipv4Add, p.r1Intf4Ipv4Add}
		for i, nextHop := range underlayIPv4NextHopDut1 {
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut1, ipv4Destination1, nextHop)
			configIPv4StaticRoute(t, dut1, ipv4Destination1, nextHop, strconv.Itoa(i))
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut1, ipv4Destination2, nextHop)
			configIPv4StaticRoute(t, dut1, ipv4Destination2, nextHop, strconv.Itoa(i))
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut1, ipv4Destination3, nextHop)
			configIPv4StaticRoute(t, dut1, ipv4Destination3, nextHop, strconv.Itoa(i))
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut1, ipv4Destination4, nextHop)
			configIPv4StaticRoute(t, dut1, ipv4Destination4, nextHop, strconv.Itoa(i))
		}
	})

	t.Run("Telemetry: Verify all tunnel interfaces oper-state", func(t *testing.T) {
		tunnelIntf := []string{"fti0", "fti1", "fti2", "fti3", "fti4", "fti5", "fti6", "fti7"}
		const want = oc.Interface_OperStatus_UP
		for _, dp := range tunnelIntf {
			if got := gnmi.Get(t, dut1, gnmi.OC().Interface(dp).Subinterface(0).OperStatus().State()); got != want {
				t.Errorf("device %s interface %s oper-status got %v, want %v", dut1, dp, got, want)
			} else {
				t.Logf("device %s interface %s oper-status got %v", dut1, dp, got)
			}
		}

	})

	//Configure Overlay Static routes for IPv4 at dut1
	t.Run("Configure overlay IPv4 static routes on dut1", func(t *testing.T) {
		ipv4Destination1 := GetNetworkAddress(t, p.rtIntf5Ipv4Add, int(p.ipv4Mask))
		ipv4Destination2 := GetNetworkAddress(t, p.rtIntf6Ipv4Add, int(p.ipv4Mask))
		// overlay static route Nexthops
		overlayIPv4NextHopDut1 := []string{p.r1Fti0Ipv4Add, p.r1Fti1Ipv4Add, p.r1Fti2Ipv4Add, p.r1Fti3Ipv4Add, p.r1Fti4Ipv4Add, p.r1Fti5Ipv4Add, p.r1Fti6Ipv4Add, p.r1Fti7Ipv4Add}
		for i, nextHop := range overlayIPv4NextHopDut1 {
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut1, ipv4Destination1, nextHop)
			configIPv4StaticRoute(t, dut1, ipv4Destination1, nextHop, strconv.Itoa(i))
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut1, ipv4Destination2, nextHop)
			configIPv4StaticRoute(t, dut1, ipv4Destination2, nextHop, strconv.Itoa(i))
		}
	})

	// Send the traffic as mentioned in Tunnel-1.3 and Tunnel-1.4 with TP-1.1 and TP-1.2
	otg := rt.OTG()
	var otgConfig gosnappi.Config
	t.Run("Configure ATE", func(t *testing.T) {
		t.Logf("Start ATE Config.")
		otgConfig = configureOTG(t, otg, p)
	})
	_ = otgConfig

	wantLoss := false
	t.Run("Verify load balance and traffic drops with IPv4 flow via 8 tunnel", func(t *testing.T) {
		t.Log("Verify load balance and traffic drops with IPv4 flow via 8 tunnel")
		VerifyUnderlayOverlayLoadbalanceTest(t, p, dut1, rt, rt, d1p1, d1p2, d1p3, d1p4, rt1, rt2, rt2, rt3, 8, wantLoss)
	})
	captureTrafficStats(t, rt, otgConfig)

}

func ConfigureLoobackInterfaceWithIPv4address(t *testing.T, address string, dut *ondatra.DUTDevice) {

	// IPv4 address on lo0 interface
	t.Logf("Push the IPv4 address to lo0 interface :\n%s", dut.Vendor())
	switch dut.Vendor() {
	case ondatra.JUNIPER:
		config := ConfigureAdditionalIPv4AddressonLoopback(address)
		t.Logf("Push the CLI config:\n%s", config)

		gnmiClient := dut.RawAPIs().GNMI().Default(t)
		gpbSetRequest, err := buildCliConfigRequest(config)
		if err != nil {
			t.Fatalf("Cannot build a gNMI SetRequest: %v", err)
		}

		t.Log("gnmiClient Set CLI config")
		if _, err = gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
			t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
		}
	default:
		t.Errorf("Invalid IPv4 Loop back address configuration")
	}

}

func ConfigureTunnelInterface(t *testing.T, intf string, tunnelSrc string, tunnelDst string, dut *ondatra.DUTDevice) {

	// IPv4 tunnel source and destination configuration
	t.Logf("Push the IPv4 tunnel endpoint config:\n%s", dut.Vendor())
	switch dut.Vendor() {
	case ondatra.JUNIPER:
		config := ConfigureTunnelEndPoints(intf, tunnelSrc, tunnelDst)
		t.Logf("Push the CLI config:\n%s", config)
		gnmiClient := dut.RawAPIs().GNMI().Default(t)
		gpbSetRequest, err := buildCliConfigRequest(config)
		if err != nil {
			t.Fatalf("Cannot build a gNMI SetRequest: %v", err)
		}

		t.Log("gnmiClient Set CLI config")
		if _, err = gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
			t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
		}
	default:
		t.Errorf("Invalid Tunnel endpoint configuration")
	}
}

// Configure network instance
func configureNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	gnmi.Replace(t, dut, dutConfPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
}

func GetNetworkAddress(t *testing.T, address string, mask int) string {

	Addr := net.ParseIP(address)
	var network net.IP
	_ = network

	// This mask corresponds to a /24 subnet for IPv4.
	ipv4Mask := net.CIDRMask(mask, 32)
	//t.Logf("%s in %T\n",ipv4Mask,ipv4Mask)
	network = Addr.Mask(ipv4Mask)
	net := fmt.Sprintf("%s/%d", network, mask)
	t.Logf("network address : %s", net)
	return net

}

func configIPv4StaticRoute(t *testing.T, dut *ondatra.DUTDevice, prefix string, nexthop string, index string) {
	ni := oc.NetworkInstance{Name: ygot.String(deviations.DefaultNetworkInstance(dut))}
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	sr := static.GetOrCreateStatic(prefix)
	nh := sr.GetOrCreateNextHop(index)
	nh.NextHop = oc.UnionString(nexthop)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)

}

func configureOTG(t *testing.T, otg *otg.OTG, p *parameters) gosnappi.Config {

	//  NewConfig creates a new OTG config.
	config := otg.NewConfig(t)
	// Add ports to config.
	port1 := config.Ports().Add().SetName("port1")
	port2 := config.Ports().Add().SetName("port2")
	port3 := config.Ports().Add().SetName("port5")
	port4 := config.Ports().Add().SetName("port6")

	//port1
	iDut1Dev := config.Devices().Add().SetName("port1")
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName("port1" + ".Eth").SetMac(p.rtIntf1MacAdd)
	iDut1Eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName("port1" + ".IPv4")
	iDut1Ipv4.SetAddress(p.rtIntf1Ipv4Add).SetGateway(p.r0Intf1Ipv4Add).SetPrefix(int32(p.ipv4Mask))

	//port2
	iDut2Dev := config.Devices().Add().SetName("port2")
	iDut2Eth := iDut2Dev.Ethernets().Add().SetName("port2" + ".Eth").SetMac(p.rtIntf2MacAdd)
	iDut2Eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(port2.Name())
	iDut2Ipv4 := iDut2Eth.Ipv4Addresses().Add().SetName("port2" + ".IPv4")
	iDut2Ipv4.SetAddress(p.rtIntf2Ipv4Add).SetGateway(p.r0Intf2Ipv4Add).SetPrefix(int32(p.ipv4Mask))

	//port5
	iDut3Dev := config.Devices().Add().SetName("port5")
	iDut3Eth := iDut3Dev.Ethernets().Add().SetName("port5" + ".Eth").SetMac(p.rtIntf5MacAdd)
	iDut3Eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(port3.Name())
	iDut3Ipv4 := iDut3Eth.Ipv4Addresses().Add().SetName("port5" + ".IPv4")
	iDut3Ipv4.SetAddress(p.rtIntf5Ipv4Add).SetGateway(p.r1Intf5Ipv4Add).SetPrefix(int32(p.ipv4Mask))

	//port6
	iDut4Dev := config.Devices().Add().SetName("port6")
	iDut4Eth := iDut4Dev.Ethernets().Add().SetName("port6" + ".Eth").SetMac(p.rtIntf6MacAdd)
	iDut4Eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(port4.Name())
	iDut4Ipv4 := iDut4Eth.Ipv4Addresses().Add().SetName("port6" + ".IPv4")
	iDut4Ipv4.SetAddress(p.rtIntf6Ipv4Add).SetGateway(p.r1Intf6Ipv4Add).SetPrefix(int32(p.ipv4Mask))

	t.Logf("Start Ote Traffic config")
	t.Logf("configure IPv4 flow from %s to %s ", port1.Name(), port3.Name())
	// Set config flow
	flow1ipv4 := config.Flows().Add().SetName(p.flow1)
	flow1ipv4.Metrics().SetEnable(true)
	// Set source and reciving ports.
	flow1ipv4.TxRx().Device().
		SetTxNames([]string{iDut1Ipv4.Name()}).
		SetRxNames([]string{iDut3Ipv4.Name()})
	// Flow settings.
	flow1ipv4.Size().SetFixed(512)
	flow1ipv4.Rate().SetPps(p.trafficRate)
	flow1ipv4.Duration().SetChoice("continuous")
	// Ethernet header
	f1e1 := flow1ipv4.Packet().Add().Ethernet()
	f1e1.Src().SetValue(iDut1Eth.Mac())
	// IP header
	f1v4 := flow1ipv4.Packet().Add().Ipv4()
	// V4 source
	f1v4.Src().Increment().SetStart(iDut1Ipv4.Address()).SetCount(200)
	// V4 destination
	f1v4.Dst().SetValue(iDut3Ipv4.Address())

	t.Logf("configure IPv4 flow from %s to %s ", port2.Name(), port4.Name())
	// Set config flow
	flow2ipv4 := config.Flows().Add().SetName(p.flow2)
	flow2ipv4.Metrics().SetEnable(true)
	// Set source and reciving ports.
	flow2ipv4.TxRx().Device().
		SetTxNames([]string{iDut2Ipv4.Name()}).
		SetRxNames([]string{iDut4Ipv4.Name()})
	// Flow settings.
	flow2ipv4.Size().SetFixed(512)
	flow2ipv4.Rate().SetPps(p.trafficRate)
	flow2ipv4.Duration().SetChoice("continuous")
	// Ethernet header
	f2e1 := flow2ipv4.Packet().Add().Ethernet()
	f2e1.Src().SetValue(iDut2Eth.Mac())
	// IP header
	f2v4 := flow2ipv4.Packet().Add().Ipv4()
	// V4 source
	f2v4.Src().Increment().SetStart(iDut2Ipv4.Address()).SetCount(200)
	// V4 destination
	f2v4.Dst().SetValue(iDut4Ipv4.Address())

	//t.Logf(config.ToJson())
	t.Logf("Pushing Traffic config to ATE and starting protocols...")
	otg.PushConfig(t, config)
	time.Sleep(30 * time.Second)
	otg.StartProtocols(t)
	time.Sleep(30 * time.Second)
	otgutils.WaitForARP(t, otg, config, "IPv4")
	return config
}

func VerifyUnderlayOverlayLoadbalanceTest(t *testing.T, p *parameters, dut1 *ondatra.DUTDevice, dut2 *ondatra.ATEDevice, rt *ondatra.ATEDevice, d1p1 *ondatra.Port, d1p2 *ondatra.Port, d1p3 *ondatra.Port, d1p4 *ondatra.Port, d2p1 *ondatra.Port, d2p2 *ondatra.Port, d2p3 *ondatra.Port, d2p4 *ondatra.Port, FtiIntfCount int64, wantLoss bool) {

	// dut1 interface statistics
	initialInfStats := map[string]uint64{}
	initialInfStats["dut1InputIntf1InPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface(d1p1.Name()).Counters().InUnicastPkts().State())
	initialInfStats["dut1InputIntf2InPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface(d1p2.Name()).Counters().InUnicastPkts().State())
	initialInfStats["dut1OutputIntf3OutPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface(d1p3.Name()).Counters().OutUnicastPkts().State())
	initialInfStats["dut1OutputIntf4OutPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface(d1p4.Name()).Counters().OutUnicastPkts().State())

	t.Logf("Initial ingress interface: %v input pkts stats: %d at dut1\n", d1p1, initialInfStats["dut1InputIntf1InPkts"])
	t.Logf("Initial ingress interface: %v input pkts stats: %d at dut1\n", d1p2, initialInfStats["dut1InputIntf2InPkts"])
	t.Logf("Initial egress interface: %v output pkts stats: %d at dut1\n", d1p3, initialInfStats["dut1OutputIntf3OutPkts"])
	t.Logf("Initial egress interface: %v output pkts stats: %d at dut1\n", d1p4, initialInfStats["dut1OutputIntf4OutPkts"])
	//dut2 interface statistics
	initialInfStats["dut2InputIntf1InPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface(d2p1.Name()).Counters().InUnicastPkts().State())
	initialInfStats["dut2InputIntf2InPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface(d2p2.Name()).Counters().InUnicastPkts().State())
	initialInfStats["dut2OutputIntf3OutPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface(d2p3.Name()).Counters().OutUnicastPkts().State())
	initialInfStats["dut2OutputIntf4IutPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface(d2p4.Name()).Counters().OutUnicastPkts().State())

	t.Logf("Initial ingress interface: %v input pkts stats: %d at dut2\n", d2p1, initialInfStats["dut2InputIntf1InPkts"])
	t.Logf("Initial ingress interface: %v input pkts stats: %d at dut2\n", d2p2, initialInfStats["dut2InputIntf2InPkts"])
	t.Logf("Initial egress interface: %v output pkts stats: %d at dut2\n", d2p3, initialInfStats["dut2OutputIntf3OutPkts"])
	t.Logf("Initial egress interface: %v output pkts stats: %d at dut2\n", d2p4, initialInfStats["dut2OutputIntf4IutPkts"])

	// Verify GRE Traffic loss at ATE
	//rt := ate.OTG()
	wantDrops := false
	t.Log("Send and validate traffic from ATE Port1 and Port2")
	SendTraffic(t, rt, p)

	flows := []string{p.flow1, p.flow2, p.flow3, p.flow4}
	for i, flowName := range flows {
		t.Logf("Verify flow %d stats", i)
		VerifyTraffic(t, rt, flowName, wantDrops)
	}

	// Incoming traffic flow should be equally distributed for Encapsulation(ECMP)
	// dut1 interface statistics
	finalInfStats := map[string]uint64{}
	finalInfStats["dut1InputIntf1InPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface(d1p1.Name()).Counters().InUnicastPkts().State())
	finalInfStats["dut1InputIntf2InPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface(d1p2.Name()).Counters().InUnicastPkts().State())
	finalInfStats["dut1OutputIntf3OutPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface(d1p3.Name()).Counters().OutUnicastPkts().State())
	finalInfStats["dut1OutputIntf4OutPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface(d1p4.Name()).Counters().OutUnicastPkts().State())

	t.Logf("After Traffic Test ingress interface: %v input pkts stats: %d at dut1\n", d1p1, finalInfStats["dut1InputIntf1InPkts"])
	t.Logf("After Traffic Test ingress interface: %v input pkts stats: %d at dut1\n", d1p2, finalInfStats["dut1InputIntf2InPkts"])
	t.Logf("After Traffic Test egress interface: %v output pkts stats: %d at dut1\n", d1p3, finalInfStats["dut1OutputIntf3OutPkts"])
	t.Logf("After Traffic Test egress interface: %v output pkts stats: %d at dut1\n", d1p4, finalInfStats["dut1OutputIntf4OutPkts"])
	//dut2 interface statistics
	finalInfStats["dut2InputIntf1InPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface(d2p1.Name()).Counters().InUnicastPkts().State())
	finalInfStats["dut2InputIntf2InPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface(d2p2.Name()).Counters().InUnicastPkts().State())
	finalInfStats["dut2OutputIntf3OutPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface(d2p3.Name()).Counters().OutUnicastPkts().State())
	finalInfStats["dut2OutputIntf4IutPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface(d2p4.Name()).Counters().OutUnicastPkts().State())

	t.Logf("After Traffic Test ingress interface: %v input pkts stats: %d at dut2\n", d2p1, finalInfStats["dut2InputIntf1InPkts"])
	t.Logf("After Traffic Test ingress interface: %v input pkts stats: %d at dut2\n", d2p2, finalInfStats["dut2InputIntf2InPkts"])
	t.Logf("After Traffic Test egress interface: %v output pkts stats: %d at dut2\n", d2p3, finalInfStats["dut2OutputIntf3OutPkts"])
	t.Logf("After Traffic Test egress interface: %v output pkts stats: %d at dut2\n", d2p4, finalInfStats["dut2OutputIntf4IutPkts"])

	// Incoming traffic flow should be equally distributed for Encapsulation(ECMP)
	t.Logf("Verify Underlay loadbalancing 2 fti tunnel interface - Incoming traffic flow should be equally distributed for Encapsulation(ECMP) ")
	for key := range finalInfStats {
		VerifyLoadbalance(t, 4, p.trafficRate, p.trafficDuration, 2, int64(initialInfStats[key]), int64(finalInfStats[key]))
	}
}

func SendTraffic(t *testing.T, ate *ondatra.ATEDevice, p *parameters) {
	otg := ate.OTG()
	t.Logf("Starting traffic")
	otg.StartTraffic(t)
	time.Sleep(time.Duration(p.trafficDuration) * time.Second)
	t.Logf("Stop traffic")
	otg.StopTraffic(t)
}

func VerifyLoadbalance(t *testing.T, flowCount int64, rate int64, duration int64, sharingIntfCont int64, initialStats int64, finalStats int64) {

	tolerance := 5
	// colculate correct stats on interface
	stats := finalStats - initialStats
	expectedTotalPkts := (flowCount * rate * duration)
	expectedPerLinkPkts := expectedTotalPkts / sharingIntfCont
	t.Logf("Total packets %d flow through the %d links", expectedTotalPkts, sharingIntfCont)
	t.Logf("Expected per link packets %d ", expectedPerLinkPkts)
	min := expectedPerLinkPkts - (expectedPerLinkPkts * int64(tolerance) / 100)
	max := expectedPerLinkPkts + (expectedPerLinkPkts * int64(tolerance) / 100)

	if min < stats && stats < max {
		t.Logf("Traffic  %d is in expected range: %d - %d", stats, min, max)
		t.Logf("Traffic Load balance Test Passed!")
	} else {
		t.Errorf("Traffic is expected in range %d - %d but got %d. Load balance Test Failed\n", min, max, stats)

	}

}

// verifyTraffic confirms that every traffic flow has the expected amount of loss (0% or 100%
// depending on wantLoss, +- 5%).
func VerifyTraffic(t *testing.T, ate *ondatra.ATEDevice, flowName string, wantLoss bool) {
	otg := ate.OTG()
	tolerancePct := 5
	t.Logf("Verifying flow metrics for flow %s\n", flowName)
	recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(flowName).State())
	txPackets := recvMetric.GetCounters().GetOutPkts()
	t.Logf("Flow: %s transmitted packets: %d !", flowName, txPackets)
	rxPackets := recvMetric.GetCounters().GetInPkts()
	t.Logf("Flow: %s received packets: %d !", flowName, rxPackets)
	lostPackets := txPackets - rxPackets
	t.Logf("Flow: %s lost packets: %d !", flowName, lostPackets)
	lossPct := lostPackets * 100 / txPackets
	t.Logf("Flow: %s packet loss percent : %d !", flowName, lossPct)
	t.Logf("Traffic Loss Test Validation")
	if wantLoss {
		if lossPct < 100-uint64(tolerancePct) {
			t.Errorf("Traffic is expected to fail %s\n got %v, want 100%% failure", flowName, lossPct)
		} else {
			t.Logf("Traffic Loss Test Passed!")
		}
	} else {
		if lossPct > uint64(tolerancePct) {
			t.Errorf("Traffic Loss Pct for Flow: %s\n got %v, want 0", flowName, lossPct)
		} else {
			t.Logf("Traffic No Loss Test Passed!")
		}
	}
}

func ConfigureAdditionalIPv4AddressonLoopback(address string) string {

	return fmt.Sprintf(`
	interfaces {

    lo0 {
        unit 0 {
            family inet {
                address %s;
            }
        }
    }
}`, address)

}

func ConfigureTunnelEndPoints(intf string, tunnelSrc string, tunnelDest string) string {

	return fmt.Sprintf(`
	interfaces {
	%s {
		unit 0 {
			tunnel {
				encapsulation gre {
					source {
						address %s;
					}
					destination {
						address %s;
					}
				}
			}
		}
	}
	}`, intf, tunnelSrc, tunnelDest)

}

func buildCliConfigRequest(config string) (*gpb.SetRequest, error) {
	// Build config with Origin set to cli and Ascii encoded config.
	gpbSetRequest := &gpb.SetRequest{
		Update: []*gpb.Update{{
			Path: &gpb.Path{
				Origin: "cli",
				Elem:   []*gpb.PathElem{},
			},
			Val: &gpb.TypedValue{
				Value: &gpb.TypedValue_AsciiVal{
					AsciiVal: config,
				},
			},
		}},
	}
	return gpbSetRequest, nil
}

// captureTrafficStats Captures traffic statistics and verifies for the loss
func captureTrafficStats(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config) {
	otg := ate.OTG()
	ap := ate.Port(t, "port1")
	t.Log("get sent packets from port1 Traffic statistics")
	aic1 := gnmi.OTG().Port(ap.ID()).Counters()
	sentPkts := gnmi.Get(t, otg, aic1.OutFrames().State())
	fptest.LogQuery(t, "ate:port1 counters", aic1.State(), gnmi.Get(t, otg, aic1.State()))
	op := ate.Port(t, "port2")
	aic2 := gnmi.OTG().Port(op.ID()).Counters()
	t.Log("get recieved packets from port2 Traffic statistics")
	rxPkts := gnmi.Get(t, otg, aic2.InFrames().State())
	fptest.LogQuery(t, "ate:port2 counters", aic2.State(), gnmi.Get(t, otg, aic2.State()))
	var lostPkts uint64
	t.Log("Verify Traffic statistics")
	if rxPkts > sentPkts {
		lostPkts = rxPkts - sentPkts
	} else {
		lostPkts = sentPkts - rxPkts
	}
	t.Logf("Packets: %d sent, %d received, %d lost", sentPkts, rxPkts, lostPkts)
	if lostPkts > tolerance {
		t.Errorf("Lost Packets are more than tolerance: %d", lostPkts)
	} else {
		t.Log("Traffic Test Passed!")
	}
	bytes := otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(config.Ports().Items()[1].Name()))
	f, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Fatalf("ERROR: Could not create temporary pcap file: %v\n", err)
	}
	if _, err := f.Write(bytes); err != nil {
		t.Fatalf("ERROR: Could not write bytes to pcap file: %v\n", err)
	}
	f.Close()
	ValidatePackets(t, f.Name())
}

func ValidatePackets(t *testing.T, filename string) {
	handle, err := pcap.OpenOffline(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		if ipLayer == nil {
			t.Errorf("IpLayer is null: %d", ipLayer)
		}
		ipPacket, _ := ipLayer.(*layers.IPv4)
		if ipPacket.Protocol != GreProtocol {
			t.Errorf("Packet is not encapslated properly. Encapsulated protocol is: %d", ipPacket.Protocol)
		}
	}
}

func ConfigureTunnelEncapDUT(t *testing.T, p *parameters, dut *ondatra.DUTDevice, dp1 *ondatra.Port, dp2 *ondatra.Port, dp3 *ondatra.Port, dp4 *ondatra.Port) {

	dutIntfs := []struct {
		desc     string
		intfName string
		ipAddr   string
		ipv4mask uint8
	}{
		{
			desc:     "R0_ATE1",
			intfName: dp1.Name(),
			ipAddr:   p.r0Intf1Ipv4Add,
			ipv4mask: p.ipv4Mask,
		}, {
			desc:     "R0_ATE2",
			intfName: dp2.Name(),
			ipAddr:   p.r0Intf2Ipv4Add,
			ipv4mask: p.ipv4Mask,
		}, {
			desc:     "R0_R1_1",
			intfName: dp3.Name(),
			ipAddr:   p.r0Intf3Ipv4Add,
			ipv4mask: p.ipv4Mask,
		},
		{
			desc:     "R0_R1_2",
			intfName: dp4.Name(),
			ipAddr:   p.r0Intf4Ipv4Add,
			ipv4mask: p.ipv4Mask,
		},
		{
			desc:     "tunnel0",
			intfName: "lo0",
			ipAddr:   p.r0Lo0Ut0Ipv4Add,
			ipv4mask: p.ipv4FullMask,
		},

		{
			desc:     "tunnel-1",
			intfName: "fti0",
			ipAddr:   p.r0Fti0Ipv4Add,
			ipv4mask: p.ipv4Mask,
		},

		{
			desc:     "tunnel-2",
			intfName: "fti1",
			ipAddr:   p.r0Fti1Ipv4Add,
			ipv4mask: p.ipv4Mask,
		},

		{
			desc:     "tunnel-3",
			intfName: "fti2",
			ipAddr:   p.r0Fti2Ipv4Add,
			ipv4mask: p.ipv4Mask,
		},
		{
			desc:     "tunnel-4",
			intfName: "fti3",
			ipAddr:   p.r0Fti3Ipv4Add,
			ipv4mask: p.ipv4Mask,
		},

		{
			desc:     "tunnel-5",
			intfName: "fti4",
			ipAddr:   p.r0Fti4Ipv4Add,
			ipv4mask: p.ipv4Mask,
		},

		{
			desc:     "tunnel-6",
			intfName: "fti5",
			ipAddr:   p.r0Fti5Ipv4Add,
			ipv4mask: p.ipv4Mask,
		},
		{
			desc:     "tunnel-7",
			intfName: "fti6",
			ipAddr:   p.r0Fti6Ipv4Add,
			ipv4mask: p.ipv4Mask,
		},
		{
			desc:     "tunnel-8",
			intfName: "fti7",
			ipAddr:   p.r0Fti7Ipv4Add,
			ipv4mask: p.ipv4Mask,
		},
	}

	// Configure the interfaces.
	for _, intf := range dutIntfs {
		t.Logf("Configure DUT interface %s with attributes %v", intf.intfName, intf)
		i := &oc.Interface{
			Name:        ygot.String(intf.intfName),
			Description: ygot.String(intf.desc),
			Type:        oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
			Enabled:     ygot.Bool(true),
		}
		// configure ipv4 address
		i.GetOrCreateEthernet()
		i4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
		a := i4.GetOrCreateAddress(intf.ipAddr)
		a.PrefixLength = ygot.Uint8(intf.ipv4mask)
		gnmi.Replace(t, dut, gnmi.OC().Interface(intf.intfName).Config(), i)

	}
}

func ConfigureTunnelTerminationOption(interf string) string {

	return fmt.Sprintf(`
	interfaces {

    %s {
        unit 0 {
            family inet {
                  tunnel-termination;
            }
            family inet6 {
                tunnel-termination;
            }
        }
    }
}`, interf)

}

func ConfigureTunnelTermination(t *testing.T, intf *ondatra.Port, dut *ondatra.DUTDevice) {

	// IPv4 tunnel termination on underlay port
	t.Logf("IPv4 tunnel termination on underlay port :\n%s", dut.Vendor())
	switch dut.Vendor() {
	case ondatra.JUNIPER:
		config := ConfigureTunnelTerminationOption(intf.Name())
		t.Logf("Push the CLI config:\n%s", config)
		gnmiClient := dut.RawAPIs().GNMI().Default(t)
		gpbSetRequest, err := buildCliConfigRequest(config)
		if err != nil {
			t.Fatalf("Cannot build a gNMI SetRequest: %v", err)
		}

		t.Log("gnmiClient Set CLI config")
		if _, err = gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
			t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
		}
	default:
		t.Errorf("Invalid Tunnel termination configuration")
	}
}
