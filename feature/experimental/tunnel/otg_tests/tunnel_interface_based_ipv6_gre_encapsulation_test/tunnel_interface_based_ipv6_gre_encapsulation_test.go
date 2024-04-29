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

package tunnel_interface_based_ipv6_gre_encapsulation_test

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	tunnelSrcIP              = "198.18.2.0"
	tunnelDesIP              = "198.18.32.0"
	tunnelNhIpv6Network      = "2001:db8:1:1:198:18:1:0"
	tunnelSrcIpv6Network     = "2001:db8:1:1:198:18:2:0"
	tunnelDesIpv6Network     = "2001:db8:1:1:198:18:32:0"
	encapInnerDesIpv6Network = "2001:db8:1:1:198:18:100:1"
	decapInnerDesIpv6Network = "2001:db8:1:1:198:18:200:1"
	tunnelPlen6              = 127
	interfacePlen6           = 112
	plen6                    = 128
	tunnelCount              = 32
	tunnelInterface          = "fti0"
	trafficRatePps           = 5000
	trafficDuration          = 120
	tolerance                = 12
)

var (
	dutIntf1 = attrs.Attributes{
		Desc:    "dutsrc",
		MAC:     "00:00:a1:a1:a1:a1",
		IPv6:    "2001:db8:1:1:198:18:0:1",
		IPv6Len: interfacePlen6,
	}

	dutIntf2 = attrs.Attributes{
		Desc:    "dutdst1",
		MAC:     "00:00:b1:b1:b1:b1",
		IPv6:    "2001:db8:1:1:198:18:3:1",
		IPv6Len: interfacePlen6,
	}

	dutIntf3 = attrs.Attributes{
		Desc:    "dutdst2",
		MAC:     "00:00:c1:c1:c1:c1",
		IPv6:    "2001:db8:1:1:198:18:4:1",
		IPv6Len: interfacePlen6,
	}

	otgIntf1 = attrs.Attributes{
		Name:    "otgsrc",
		IPv6:    "2001:db8:1:1:198:18:0:2",
		IPv6Len: interfacePlen6,
		MAC:     "00:00:01:01:01:01",
	}

	otgIntf2 = attrs.Attributes{
		Name:    "otgdst1",
		IPv6:    "2001:db8:1:1:198:18:3:2",
		IPv6Len: interfacePlen6,
		MAC:     "00:00:02:02:02:02",
	}

	otgIntf3 = attrs.Attributes{
		Name:    "otgdst2",
		IPv6:    "2001:db8:1:1:198:18:4:2",
		IPv6Len: interfacePlen6,
		MAC:     "00:00:03:03:03:03",
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestTunnelEncapsulationByGREOverIPv6WithLoadBalance(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dutPort1 := dut.Port(t, "port1")
	dutPort2 := dut.Port(t, "port2")
	dutPort3 := dut.Port(t, "port3")
	ate := ondatra.ATE(t, "ate")
	ateport1 := ate.Port(t, "port1")
	ateport2 := ate.Port(t, "port2")
	ateport3 := ate.Port(t, "port3")
	egressInterfaces := []string{dutPort2.Name(), dutPort3.Name()}
	initialEgressPkts := make([]uint64, tunnelCount)
	initialTunnelInPkts := make([]uint64, tunnelCount)
	initialTunnelOutPkts := make([]uint64, tunnelCount)
	tunnelLoadblanceDiff := tunnelCount * 3
	interfaceLoadblanceDiff := tolerance
	t.Run("Configure dut with 32 tunnel interface with one ingress and 2 egress interface", func(t *testing.T) {
		configureTunnelBaseOnDUT(t, dut, dutPort1, &dutIntf1)
		configureTunnelBaseOnDUT(t, dut, dutPort2, &dutIntf2)
		configureTunnelBaseOnDUT(t, dut, dutPort3, &dutIntf3)
		step := 0
		var overlayIPv6Nh []string
		for unit := 0; unit < tunnelCount; unit++ {
			tunnelSrc := incrementAddress(t, tunnelSrcIpv6Network, unit, "host")
			tunnelDstNetwork := incrementAddress(t, tunnelDesIpv6Network, unit, "network")
			tunnelDst := incrementAddress(t, tunnelDstNetwork, 1, "host")
			tunnelIpv6address := incrementAddress(t, tunnelNhIpv6Network, step, "host")
			t.Logf("unit : %d tunnel ipv6 address: %s/%d tunnel source address: %s tunnel destination: %s", unit, tunnelIpv6address, tunnelPlen6, tunnelSrc, tunnelDst)
			if deviations.TunnelConfigPathUnsupported(dut) {
				configureTunnelInterface(t, tunnelInterface, unit, tunnelSrc, tunnelDst, tunnelIpv6address, tunnelPlen6, dut)
			}
			overlayIPv6Nh = append(overlayIPv6Nh, incrementAddress(t, tunnelIpv6address, 1, "host"))
			step = step + 2
		}
		t.Logf("Configure routing instance on dut")
		configureNetworkInstance(t, dut)
		t.Logf("Configure IPv6 tunnel destination address reachable via ECMP link")
		underlayIpv6Nh := []string{otgIntf2.IPv6, otgIntf3.IPv6}
		for i, nextHop := range underlayIpv6Nh {
			_, ipv6Destination := fetchNetworkAddress(t, tunnelDesIpv6Network, 96)
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut, ipv6Destination, nextHop)
			configStaticRoute(t, dut, ipv6Destination, nextHop, strconv.Itoa(i))
		}

		t.Logf("overlay static route via tunnel for an original IPv6 destination prefix")
		for i, nextHop := range overlayIPv6Nh {
			_, ipv6Destination := fetchNetworkAddress(t, encapInnerDesIpv6Network, interfacePlen6)
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut, ipv6Destination, nextHop)
			configStaticRoute(t, dut, ipv6Destination, nextHop, strconv.Itoa(i))
		}
	})
	t.Run("Configure OTG ports", func(t *testing.T) {
		top := gosnappi.NewConfig()
		t.Logf("Start Port/device configuraturation on OTG")
		configureOtgPorts(top, ateport1, otgIntf1.Name, otgIntf1.MAC, otgIntf1.IPv6, dutIntf1.IPv6, otgIntf1.IPv6Len)
		configureOtgPorts(top, ateport2, otgIntf2.Name, otgIntf2.MAC, otgIntf2.IPv6, dutIntf2.IPv6, otgIntf2.IPv6Len)
		configureOtgPorts(top, ateport3, otgIntf3.Name, otgIntf3.MAC, otgIntf3.IPv6, dutIntf3.IPv6, otgIntf3.IPv6Len)
		ate.OTG().PushConfig(t, top)
		time.Sleep(30 * time.Second)
		t.Logf("Start Traffic flow configuraturation in OTG")
		configureTrafficFlowsToEncasulation(t, top, ateport1, ateport2, ateport3, &otgIntf1, dutIntf1.MAC)
		t.Logf(top.Marshal().ToJson())
		ate.OTG().PushConfig(t, top)
		ate.OTG().StartProtocols(t)
		time.Sleep(30 * time.Second)
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
	})
	t.Run("Fetch all the interface status before start traffic ", func(t *testing.T) {
		initialEgressPkts = fetchEgressInterfacestatsics(t, dut, egressInterfaces)
		if !deviations.TunnelStatePathUnsupported(dut) {
			initialTunnelInPkts, initialTunnelOutPkts = fetchTunnelInterfacestatsics(t, dut, tunnelCount)
		}
	})
	t.Run("Incoming traffic flow should be equally distributed for Encapsulation(ECMP) ", func(t *testing.T) {
		t.Log("Send traffic from OTG Port1 to Port2 and Port3")
		wantLoss := true
		sendTraffic(t, ate)
		flows := []string{"IPv6"}
		for i, flowName := range flows {
			t.Logf("Verify flow %d stats", i)
			verifyTrafficStatistics(t, ate, flowName, wantLoss)
		}

	})
	t.Run("Verify after Encapsulation loadbalance (ECMP) && load balanced to available Tunnel interfaces ", func(t *testing.T) {
		finalEgressPkts := fetchEgressInterfacestatsics(t, dut, egressInterfaces)
		t.Logf("Verify Incoming traffic flow should be equally distributed for Encapsulation(ECMP)")
		verifyEcmpLoadBalance(t, initialEgressPkts, finalEgressPkts, 1, int64(len(egressInterfaces)), 0, true, interfaceLoadblanceDiff)
		if !deviations.TunnelStatePathUnsupported(dut) {
			finalTunnelInPkts, finalTunnelOutPkts := fetchTunnelInterfacestatsics(t, dut, tunnelCount)
			t.Logf("Incoming traffic on DUT-PORT1 should be load balanced to available Tunnel interfaces for encapsulation")
			verifyEcmpLoadBalance(t, initialTunnelOutPkts, finalTunnelOutPkts, 1, int64(tunnelCount), 0, true, tunnelLoadblanceDiff)
			verifyUnusedTunnelStatistic(t, initialTunnelInPkts, finalTunnelInPkts)
		}
	})
}

func fetchEgressInterfacestatsics(t *testing.T, dut *ondatra.DUTDevice, interfaceSlice []string) []uint64 {
	egressStats := make([]uint64, len(interfaceSlice))
	for i, intf := range interfaceSlice {
		egressStats[i] = gnmi.Get(t, dut, gnmi.OC().Interface(intf).Counters().OutPkts().State())
	}
	t.Log("Egress interface Out pkts stats:", egressStats)
	return egressStats
}

func fetchTunnelInterfacestatsics(t *testing.T, dut *ondatra.DUTDevice, count int) ([]uint64, []uint64) {
	tunnelOutStats := make([]uint64, count)
	tunnelInStats := make([]uint64, count)
	for i := 0; i < count; i++ {
		tunnelOutStats[i] = gnmi.Get(t, dut, gnmi.OC().Interface(tunnelInterface).Subinterface(uint32(i)).Counters().OutPkts().State())
		tunnelInStats[i] = gnmi.Get(t, dut, gnmi.OC().Interface(tunnelInterface).Subinterface(uint32(i)).Counters().InPkts().State())
	}
	t.Log("Tunnel In pkts stats:", tunnelInStats)
	t.Log("Tunnel Out pkts stats:", tunnelOutStats)
	return tunnelInStats, tunnelOutStats
}

func verifyTrafficStatistics(t *testing.T, ate *ondatra.ATEDevice, flowName string, wantLoss bool) {
	otg := ate.OTG()
	t.Logf("Traffic Loss Test Validation for flow %s\n", flowName)
	recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(flowName).State())
	txPackets := recvMetric.GetCounters().GetOutPkts()
	t.Logf("Flow: %s transmitted packets: %d !", flowName, txPackets)
	rxPackets := recvMetric.GetCounters().GetInPkts()
	t.Logf("Flow: %s received packets: %d !", flowName, rxPackets)
	lostPackets := txPackets - rxPackets
	t.Logf("Flow: %s lost packets: %d !", flowName, lostPackets)
	lossPct := lostPackets * 100 / txPackets
	t.Logf("Flow: %s packet loss percent : %d !", flowName, lossPct)
	if wantLoss {
		if lossPct > uint64(tolerance) {
			t.Errorf("Traffic Loss for Flow: %s but got %v, want 0 Failed.", flowName, lossPct)
		} else {
			t.Logf("No Traffic Loss Test Passed!!")
		}
	} else {
		if lossPct < 100-uint64(tolerance) {
			t.Errorf("Traffic is expected to fail but flow :%s  got %v, want 100%% Failed.", flowName, lossPct)
		} else {
			t.Logf("Traffic Loss Test Passed!!")
		}
	}
}

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice) {
	otg := ate.OTG()
	t.Logf("Starting traffic")
	otg.StartTraffic(t)
	time.Sleep(time.Duration(trafficDuration) * time.Second)
	t.Logf("Stop traffic")
	otg.StopTraffic(t)
}

func configureOtgPorts(top gosnappi.Config, port *ondatra.Port, name string, mac string, ipv6Address string, ipv6Gateway string, ipv6Mask uint8) {
	top.Ports().Add().SetName(port.ID())
	//port1
	iDutDev := top.Devices().Add().SetName(name)
	iDutEth := iDutDev.Ethernets().Add().SetName(name + ".Eth").SetMac(mac)
	iDutEth.Connection().SetPortName(port.ID())
	iDutIpv6 := iDutEth.Ipv6Addresses().Add().SetName(name + ".IPv6")
	iDutIpv6.SetAddress(ipv6Address).SetGateway(ipv6Gateway).SetPrefix(uint32(ipv6Mask))
}

func configureTrafficFlowsToEncasulation(t *testing.T, top gosnappi.Config, port1 *ondatra.Port, port2 *ondatra.Port, port3 *ondatra.Port, peer *attrs.Attributes, destMac string) {
	t.Logf("configure IPv6 flow from %s ", port1.Name())
	// Set config flow
	flow1ipv6 := top.Flows().Add().SetName("IPv6")
	flow1ipv6.Metrics().SetEnable(true)
	//TxRx ports
	flow1ipv6.TxRx().Port().SetTxName(port1.ID()).SetRxNames([]string{port2.ID(), port3.ID()})
	// Flow settings.
	flow1ipv6.Size().SetFixed(512)
	flow1ipv6.Rate().SetPps(trafficRatePps)
	flow1ipv6.Duration().Continuous()
	// Ethernet header
	f1e2 := flow1ipv6.Packet().Add().Ethernet()
	f1e2.Src().SetValue(peer.MAC)
	f1e2.Dst().SetValue(destMac)
	// IPv6 header
	f1v6 := flow1ipv6.Packet().Add().Ipv6()
	// V6 NextHeader
	f1v6.NextHeader().SetValue(6)
	// V6 source
	f1v6.Src().Increment().SetStart(peer.IPv6).SetCount(200)
	// V6 destination
	f1v6.Dst().Increment().SetStart(encapInnerDesIpv6Network).SetCount(200)
	// Add L4 protocol
	flow1ipv6.Packet().Add().Tcp()
	// Increment Source port
	flow1ipv6.Packet().Add().Tcp().SrcPort().Increment().SetStart(9000).SetCount(28000)
	// Increment destination port
	flow1ipv6.Packet().Add().Tcp().DstPort().Increment().SetStart(37001).SetCount(28000)
}

func fetchNetworkAddress(t *testing.T, address string, mask int) (string, string) {
	addr := net.ParseIP(address)
	var network net.IP
	// This mask corresponds to a /128 subnet for IPv6.
	ipv6Mask := net.CIDRMask(mask, 128)
	network = addr.Mask(ipv6Mask)
	networkWithMask := network.String() + "/" + strconv.Itoa(mask)
	networkAlone := network.String()
	//t.Logf("Network address : %s", networkWithMask)
	return networkAlone, networkWithMask
}

func incrementAddress(t *testing.T, address string, i int, part string) string {
	addr := net.ParseIP(address)
	var oct int
	switch part {
	case "network":
		oct = 13
	case "host":
		oct = 15
	default:
		t.Errorf("Invalid value")
	}
	for j := 0; j < i; j++ {
		addr[oct]++
	}
	return addr.String()
}

func configureTunnelBaseOnDUT(t *testing.T, dut *ondatra.DUTDevice, dp *ondatra.Port, a *attrs.Attributes) {
	dutIntfs := []struct {
		desc     string
		intfName string
		ipv6Addr string
		ipv6mask uint8
		mac      string
	}{
		{
			desc:     a.Desc,
			intfName: dp.Name(),
			ipv6Addr: a.IPv6,
			ipv6mask: a.IPv6Len,
			mac:      a.MAC,
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

		// configure mac address
		e := i.GetOrCreateEthernet()
		e.MacAddress = ygot.String(intf.mac)

		// configure ipv6 address
		i6 := i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
		b := i6.GetOrCreateAddress(intf.ipv6Addr)
		b.PrefixLength = ygot.Uint8(intf.ipv6mask)
		gnmi.Replace(t, dut, gnmi.OC().Interface(intf.intfName).Config(), i)
	}
}

func configureTunnelInterface(t *testing.T, intf string, unit int, tunnelSrc string, tunnelDst string, tunnelIpv6address string, Ipv6Mask int, dut *ondatra.DUTDevice) {
	t.Logf("Push the IPv6 tunnel endpoint config:\n%s", dut.Vendor())
	var config string
	switch dut.Vendor() {
	case ondatra.JUNIPER:
		config = configureTunnelEndPoints(intf, unit, tunnelSrc, tunnelDst, tunnelIpv6address, Ipv6Mask)
		t.Logf("Push the CLI config:\n%s", config)

	default:
		t.Errorf("Invalid Tunnel endpoint configuration")
	}
	gnmiClient := dut.RawAPIs().GNMI(t)
	gpbSetRequest := buildCliConfigRequest(config)

	t.Log("gnmiClient Set CLI config")
	if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
		t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
	}
}

func configureTunnelEndPoints(intf string, unit int, tunnelSrc string, tunnelDest string, tunnelIpv6address string, Ipv6Mask int) string {
	return fmt.Sprintf(`
	interfaces {
	%s {
		unit %d {
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
			family inet6 {
				address %s/%d;
			}
		}
	}
	}`, intf, unit, tunnelSrc, tunnelDest, tunnelIpv6address, Ipv6Mask)
}

func configStaticRoute(t *testing.T, dut *ondatra.DUTDevice, prefix string, nexthop string, index string) {
	ni := oc.NetworkInstance{Name: ygot.String(deviations.DefaultNetworkInstance(dut))}
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	sr := static.GetOrCreateStatic(prefix)
	nh := sr.GetOrCreateNextHop(index)
	nh.NextHop = oc.UnionString(nexthop)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
}

func buildCliConfigRequest(config string) *gpb.SetRequest {
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
	return gpbSetRequest
}

func configureNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {
	t.Logf("Configure routing instance on dut")
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	gnmi.Replace(t, dut, dutConfPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
}

func verifyEcmpLoadBalance(t *testing.T, inital []uint64, final []uint64, flowCount int64, sharingIntfCont int64, firstintf int, wantLoss bool, lbTolerance int) {
	expectedTotalPkts := (flowCount * trafficRatePps * trafficDuration)
	expectedPerLinkPkts := expectedTotalPkts / sharingIntfCont
	t.Logf("Total packets %d flow through the %d links", expectedTotalPkts, sharingIntfCont)
	t.Logf("Expected per link packets %d ", expectedPerLinkPkts)
	min := expectedPerLinkPkts - (expectedPerLinkPkts * int64(lbTolerance) / 100)
	max := expectedPerLinkPkts + (expectedPerLinkPkts * int64(lbTolerance) / 100)

	for i := firstintf; i < len(inital); i++ {
		stats := final[i] - inital[i]
		t.Logf("Initial packets %d Final Packets %d ", inital[i], final[i])
		if wantLoss {
			if min < int64(stats) && int64(stats) < max {
				t.Logf("Traffic  %d is in expected range: %d - %d", stats, min, max)
				t.Logf("Traffic Load balance Test Passed!!")
			} else {
				t.Errorf("Traffic is expected in range %d - %d but got %d. Load balance Test Failed\n", min, max, stats)

			}
		} else {
			if min > int64(stats) || int64(stats) > max {
				t.Logf("Traffic  %d is not in expected range: %d - %d", stats, min, max)
				t.Logf("Tunnel interfaces was down, Traffic not used this interface as expected Passed!!")
			} else {
				t.Errorf("Traffic is not expected in range %d - %d but got %d. Negative Load balance Test Failed\n", min, max, stats)
			}
		}
	}
}

func verifyUnusedTunnelStatistic(t *testing.T, inital []uint64, final []uint64) {
	for i := 0; i < len(inital); i++ {
		value := final[i] - inital[i]
		if int(value) > tolerance {
			t.Logf("Traffic initial stats %d && final stats %d ", inital[i], final[i])
			t.Errorf("Tunnel interface used and got %d stats additionally which is not expected FAILED!!\n", value)
		}
	}
}
