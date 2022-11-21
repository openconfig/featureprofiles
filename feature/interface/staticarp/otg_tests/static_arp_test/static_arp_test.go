// Copyright 2022 Google LLC
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

package te_1_1_static_arp_test

import (
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ygot/ygot"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the test
// topology.  IxNetwork flow requires both source and destination
// networks be configured on the ATE.  It is not possible to send
// packets to the ether.
//
// The testbed consists of ate:port1 -> dut:port1 and
// dut:port2 -> ate:port2.  The first pair is called the "source"
// pair, and the second the "destination" pair.
//
//   - Source: ate:port1 -> dut:port1 subnet 192.0.2.0/30 2001:db8::0/126
//   - Destination: dut:port2 -> ate:port2 subnet 192.0.2.4/30 2001:db8::4/126
//
// Note that the first (.0, .4) and last (.3, .7) IPv4 addresses are
// reserved from the subnet for broadcast, so a /30 leaves exactly 2
// usable addresses.  This does not apply to IPv6 which allows /127
// for point to point links, but we use /126 so the numbering is
// consistent with IPv4.
//
// A traffic flow is configured from ate:port1 as the source interface
// and ate:port2 as the destination interface.  The traffic should
// flow as expected both when using dynamic or static ARP since the
// Ixia interfaces are promiscuous.  However, using custom egress
// filter, we can tell if the static ARP is honored or not.
//
// Synthesized static MAC addresses have the form 02:1a:WW:XX:YY:ZZ
// where WW:XX:YY:ZZ are the four octets of the IPv4 in hex.  The 0x02
// means the MAC address is locally administered.
const (
	plen4 = 30
	plen6 = 126

	poisonedMAC = "12:34:56:78:7a:69" // 0x7a69 = 31337
	noStaticMAC = ""
)

var (
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		MAC:     "02:11:01:00:00:01",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	dutSrc = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		MAC:     "02:1a:c0:00:02:02", // 02:1a+192.0.2.2
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	dutDst = attrs.Attributes{
		Desc:    "DUT to ATE destination",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		MAC:     "02:1a:c0:00:02:05", // 02:1a+192.0.2.5
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	ateDst = attrs.Attributes{
		Name:    "ateDst",
		MAC:     "02:12:01:00:00:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
)

// configInterfaceDUT configures the interface on "me" with static ARP
// of peer.  Note that peermac is used for static ARP, and not
// peer.MAC.
func configInterfaceDUT(i *oc.Interface, me, peer *attrs.Attributes, peermac string) *oc.Interface {
	i.Description = ygot.String(me.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if *deviations.InterfaceEnabled {
		i.Enabled = ygot.Bool(true)
	}

	if me.MAC != "" {
		e := i.GetOrCreateEthernet()
		e.MacAddress = ygot.String(me.MAC)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if *deviations.InterfaceEnabled {
		s4.Enabled = ygot.Bool(true)
	}
	a := s4.GetOrCreateAddress(me.IPv4)
	a.PrefixLength = ygot.Uint8(plen4)

	if peermac != noStaticMAC {
		n4 := s4.GetOrCreateNeighbor(peer.IPv4)
		n4.LinkLayerAddress = ygot.String(peermac)
	}

	s6 := s.GetOrCreateIpv6()
	if *deviations.InterfaceEnabled {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(me.IPv6).PrefixLength = ygot.Uint8(plen6)

	if peermac != noStaticMAC {
		n6 := s6.GetOrCreateNeighbor(peer.IPv6)
		n6.LinkLayerAddress = ygot.String(peermac)
	}

	return i
}

func configureDUT(t *testing.T, peermac string) {
	dut := ondatra.DUT(t, "dut")
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	if peermac == "" {
		gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutSrc, &ateSrc, peermac))
	}
	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutDst, &ateDst, peermac))
}

func configureOTG(t *testing.T) (*ondatra.ATEDevice, gosnappi.Config) {
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	config := otg.NewConfig(t)
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")

	srcPort := config.Ports().Add().SetName(ap1.ID())
	dstPort := config.Ports().Add().SetName(ap2.ID())

	srcDev := config.Devices().Add().SetName(ateSrc.Name)
	srcEth := srcDev.Ethernets().Add().SetName(ateSrc.Name + ".eth").SetPortName(srcPort.Name()).SetMac(ateSrc.MAC)
	srcEth.Ipv4Addresses().Add().SetName(ateSrc.Name + ".IPv4").SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).SetPrefix(int32(ateSrc.IPv4Len))
	srcEth.Ipv6Addresses().Add().SetName(ateSrc.Name + ".IPv6").SetAddress(ateSrc.IPv6).SetGateway(dutSrc.IPv6).SetPrefix(int32(ateSrc.IPv6Len))

	dstDev := config.Devices().Add().SetName(ateDst.Name)
	dstEth := dstDev.Ethernets().Add().SetName(ateDst.Name + ".eth").SetPortName(dstPort.Name()).SetMac(ateDst.MAC)
	dstEth.Ipv4Addresses().Add().SetName(ateDst.Name + ".IPv4").SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).SetPrefix(int32(ateDst.IPv4Len))
	dstEth.Ipv6Addresses().Add().SetName(ateDst.Name + ".IPv6").SetAddress(ateDst.IPv6).SetGateway(dutDst.IPv6).SetPrefix(int32(ateDst.IPv6Len))

	return ate, config
}

func checkDUTEntry(t *testing.T, ipType string, poisoned bool) {
	dut := ondatra.DUT(t, "dut")
	var expectedMac string
	if poisoned {
		expectedMac = poisonedMAC
	} else {
		expectedMac = ateDst.MAC
	}
	switch ipType {
	case "IPv4":
		macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port2").Name()).Subinterface(0).Ipv4().Neighbor(ateDst.IPv4).State()).LinkLayerAddress
		if *macAddress != expectedMac {
			t.Errorf("ARP entry for %v is %v and expected was %v", ateDst.IPv4, *macAddress, expectedMac)
		} else {
			t.Logf("ARP entry for %v is %v", ateDst.IPv4, *macAddress)
		}
	case "IPv6":
		macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port2").Name()).Subinterface(0).Ipv6().Neighbor(ateDst.IPv6).State()).LinkLayerAddress
		if *macAddress != expectedMac {
			t.Errorf("Neighbor entry for %v is %v and expected was %v", ateDst.IPv6, *macAddress, expectedMac)
		} else {
			t.Logf("Neighbor entry for %v is %v", ateDst.IPv6, *macAddress)
		}
	}
}

func waitOTGARPEntry(t *testing.T, ipType string) {
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()

	switch ipType {
	case "IPv4":
		gnmi.WatchAll(t, otg, gnmi.OTG().Interface(ateSrc.Name+".eth").Ipv4NeighborAny().LinkLayerAddress().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
			return val.IsPresent()
		}).Await(t)
	case "IPv6":
		gnmi.WatchAll(t, otg, gnmi.OTG().Interface(ateSrc.Name+".eth").Ipv6NeighborAny().LinkLayerAddress().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
			return val.IsPresent()
		}).Await(t)
	}
}

func testFlow(
	t *testing.T,
	ate *ondatra.ATEDevice,
	config gosnappi.Config,
	ipType string,
	poisoned bool,
) {

	// Egress tracking inspects packets from DUT and key the flow
	// counters by custom bit offset and width.  Width is limited to
	// 15-bits.
	//
	// Ethernet header:
	//   - Destination MAC (6 octets)
	//   - Source MAC (6 octets)
	//   - Optional 802.1q VLAN tag (4 octets)
	//   - Frame size (2 octets)
	// Configure the flow
	otg := ate.OTG()
	i1 := ateSrc.Name
	i2 := ateDst.Name
	config.Flows().Clear().Items()
	switch ipType {
	case "IPv4":
		flowipv4 := config.Flows().Add().SetName("FlowIPv4")
		flowipv4.Metrics().SetEnable(true)
		flowipv4.TxRx().Device().
			SetTxNames([]string{i1 + ".IPv4"}).
			SetRxNames([]string{i2 + ".IPv4"})
		flowipv4.Duration().SetChoice("fixed_packets")
		flowipv4.Duration().FixedPackets().SetPackets(1000)
		flowipv4.Size().SetFixed(100)
		e1 := flowipv4.Packet().Add().Ethernet()
		e1.Src().SetValue(ateSrc.MAC)
		v4 := flowipv4.Packet().Add().Ipv4()
		v4.Src().SetValue(ateSrc.IPv4)
		v4.Dst().SetValue(ateDst.IPv4)
		otg.PushConfig(t, config)
		otg.StartProtocols(t)
		waitOTGARPEntry(t, "IPv4")
	case "IPv6":
		flowipv6 := config.Flows().Add().SetName("FlowIPv6")
		flowipv6.Metrics().SetEnable(true)
		flowipv6.TxRx().Device().
			SetTxNames([]string{i1 + ".IPv6"}).
			SetRxNames([]string{i2 + ".IPv6"})
		flowipv6.Duration().SetChoice("fixed_packets")
		flowipv6.Duration().FixedPackets().SetPackets(1000)
		flowipv6.Size().SetFixed(100)
		e1 := flowipv6.Packet().Add().Ethernet()
		e1.Src().SetValue(ateSrc.MAC)
		v4 := flowipv6.Packet().Add().Ipv6()
		v4.Src().SetValue(ateSrc.IPv6)
		v4.Dst().SetValue(ateDst.IPv6)
		otg.PushConfig(t, config)
		otg.StartProtocols(t)
		waitOTGARPEntry(t, "IPv6")
	}

	// Starting the traffic
	otg.StartTraffic(t)
	time.Sleep(15 * time.Second)
	t.Logf("Stop traffic")
	otg.StopTraffic(t)

	// Get the flow statistics
	otgutils.LogFlowMetrics(t, otg, config)
	for _, f := range config.Flows().Items() {
		recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(f.Name()).State())
		if recvMetric.GetCounters().GetInPkts() != recvMetric.GetCounters().GetOutPkts() || recvMetric.GetCounters().GetInPkts() != 1000 {
			t.Errorf("LossPct for flow %s detected, expected 0", f.Name())
		}
	}

}

func TestStaticARP(t *testing.T) {
	// First configure the DUT with dynamic ARP.

	configureDUT(t, noStaticMAC)
	// var ate *ondatra.ATEDevice
	ate, config := configureOTG(t)

	// Default MAC addresses on Ixia are assigned incrementally as:
	//   - 02:11:01:00:00:01
	//   - 02:12:01:00:00:01
	// etc.
	//
	t.Run("NotPoisoned", func(t *testing.T) {
		t.Run("IPv4", func(t *testing.T) {
			testFlow(t, ate, config, "IPv4", false)
			checkDUTEntry(t, "IPv4", false)
		})
		t.Run("IPv6", func(t *testing.T) {
			testFlow(t, ate, config, "IPv6", false)
			checkDUTEntry(t, "IPv6", false)
		})
	})

	// Reconfigure the DUT with static MAC.
	configureDUT(t, poisonedMAC)

	t.Run("Poisoned", func(t *testing.T) {
		t.Run("IPv4", func(t *testing.T) {
			testFlow(t, ate, config, "IPv4", true)
			checkDUTEntry(t, "IPv4", true)
		})
		t.Run("IPv6", func(t *testing.T) {
			testFlow(t, ate, config, "IPv6", true)
			checkDUTEntry(t, "IPv6", true)
		})
	})
}
