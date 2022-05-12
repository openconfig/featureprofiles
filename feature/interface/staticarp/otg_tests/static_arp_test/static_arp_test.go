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
	"log"
	"sort"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/helpers"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"

	"github.com/openconfig/ondatra"
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
//   * Source: ate:port1 -> dut:port1 subnet 192.0.2.0/30 2001:db8::0/126
//   * Destination: dut:port2 -> ate:port2 subnet 192.0.2.4/30 2001:db8::4/126
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
	trafficDuration   = 10 * time.Second
	trafficPacketRate = 200
	plen4             = 30
	plen6             = 126

	poisonedMAC = "12:34:56:78:7a:69" // 0x7a69 = 31337
	noStaticMAC = ""
)

var (
	ateSrc = attrs.Attributes{
		Name:    "port1",
		MAC:     "00:11:01:00:00:01",
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
		Name:    "port2",
		MAC:     "00:12:01:00:00:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
)

// configInterfaceDUT configures the interface on "me" with static ARP
// of peer.  Note that peermac is used for static ARP, and not
// peer.MAC.
func configInterfaceDUT(i *telemetry.Interface, me, peer *attrs.Attributes, peermac string) *telemetry.Interface {
	i.Description = ygot.String(me.Desc)
	i.Type = telemetry.IETFInterfaces_InterfaceType_ethernetCsmacd
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

func configureDUT(t *testing.T, peermac string) (interfaceOrder bool) {
	interfaceOrder = true
	dut := ondatra.DUT(t, "dut")
	d := dut.Config()
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	i1 := &telemetry.Interface{Name: ygot.String(p1.Name())}
	i2 := &telemetry.Interface{Name: ygot.String(p2.Name())}
	portList := []string{*i1.Name, *i2.Name}
	if sort.StringsAreSorted(portList) {
		if peermac == "" {
			d.Interface(p1.Name()).Replace(t,
				configInterfaceDUT(i1, &dutSrc, &ateSrc, peermac))
		}
		d.Interface(p2.Name()).Replace(t,
			configInterfaceDUT(i2, &dutDst, &ateDst, peermac))
	} else {
		interfaceOrder = false
		d.Interface(p1.Name()).Replace(t,
			configInterfaceDUT(i1, &dutDst, &ateDst, peermac))
		if peermac == "" {
			d.Interface(p2.Name()).Replace(t,
				configInterfaceDUT(i2, &dutSrc, &ateSrc, peermac))
		}
	}
	return interfaceOrder
}

func configureOTG(t *testing.T, interfaceOrder bool) (*ondatra.ATEDevice, gosnappi.Config) {
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG(t)
	config := otg.NewConfig()
	var srcPort gosnappi.Port
	var dstPort gosnappi.Port
	if interfaceOrder {
		srcPort = config.Ports().Add().SetName(ateSrc.Name)
		dstPort = config.Ports().Add().SetName(ateDst.Name)
	} else {
		srcPort = config.Ports().Add().SetName(ateDst.Name)
		dstPort = config.Ports().Add().SetName(ateSrc.Name)
	}

	srcDev := config.Devices().Add().SetName(ateSrc.Name)
	srcEth := srcDev.Ethernets().Add().
		SetName(ateSrc.Name + ".eth").
		SetPortName(srcPort.Name()).
		SetMac(ateSrc.MAC)
	srcEth.Ipv4Addresses().Add().
		SetName(ateSrc.Name + ".ipv4").
		SetAddress(ateSrc.IPv4).
		SetGateway(dutSrc.IPv4).
		SetPrefix(int32(ateSrc.IPv4Len))
	srcEth.Ipv6Addresses().Add().
		SetName(ateSrc.Name + ".ipv6").
		SetAddress(ateSrc.IPv6).
		SetGateway(dutSrc.IPv6).
		SetPrefix(int32(ateSrc.IPv6Len))

	dstDev := config.Devices().Add().SetName(ateDst.Name)
	dstEth := dstDev.Ethernets().Add().
		SetName(ateDst.Name + ".eth").
		SetPortName(dstPort.Name()).
		SetMac(ateDst.MAC)
	dstEth.Ipv4Addresses().Add().
		SetName(ateDst.Name + ".ipv4").
		SetAddress(ateDst.IPv4).
		SetGateway(dutDst.IPv4).
		SetPrefix(int32(ateDst.IPv4Len))
	dstEth.Ipv6Addresses().Add().
		SetName(ateDst.Name + ".ipv6").
		SetAddress(ateDst.IPv6).
		SetGateway(dutDst.IPv6).
		SetPrefix(int32(ateDst.IPv6Len))

	return ate, config
}

func checkArpEntry(t *testing.T, ipType string, poisoned bool) {
	dut := ondatra.DUT(t, "dut")
	var expectedMac string
	if poisoned {
		expectedMac = poisonedMAC
	} else {
		expectedMac = ateDst.MAC
	}
	switch ipType {
	case "IPv4":
		macAddress := dut.Telemetry().Interface("Ethernet2").Subinterface(0).Ipv4().Neighbor(ateDst.IPv4).Get(t).LinkLayerAddress
		if *macAddress != expectedMac {
			t.Errorf("ARP entry for %v is %v and expected was %v", ateDst.IPv4, *macAddress, expectedMac)
		} else {
			t.Logf("ARP entry for %v is %v", ateDst.IPv4, *macAddress)
		}
	case "IPv6":
		macAddress := dut.Telemetry().Interface("Ethernet2").Subinterface(0).Ipv6().Neighbor(ateDst.IPv6).Get(t).LinkLayerAddress
		if *macAddress != expectedMac {
			t.Errorf("ARP entry for %v is %v and expected was %v", ateDst.IPv6, *macAddress, expectedMac)
		} else {
			t.Logf("ARP entry for %v is %v", ateDst.IPv6, *macAddress)
		}
	}
}

func GetInterfaceMacs(t *testing.T, dev *ondatra.Device) map[string]string {
	t.Helper()
	dutMacDetails := make(map[string]string)
	for _, p := range dev.Ports() {
		eth := dev.Telemetry().Interface(p.Name()).Ethernet().Get(t)
		t.Logf("Mac address of Interface %s in DUT: %s", p.Name(), eth.GetMacAddress())
		dutMacDetails[p.Name()] = eth.GetMacAddress()
	}
	return dutMacDetails
}

func checkOTGArpEntry(t *testing.T, c gosnappi.Config, ipType string, poisoned bool) {
	ate := ondatra.ATE(t, "ate")
	dut := ondatra.DUT(t, "dut")
	dutInterfaceMac := GetInterfaceMacs(t, dut.Device)
	t.Logf("Mac Addresses of DUT: %v", dutInterfaceMac)
	expectedMacEntries := []string{}
	if poisoned == true {
		expectedMacEntries = append(expectedMacEntries, dutInterfaceMac["Ethernet1"])
	} else {
		for _, macValue := range dutInterfaceMac {
			expectedMacEntries = append(expectedMacEntries, macValue)
		}
	}

	err := helpers.WaitFor(
		t,
		func() (bool, error) { return helpers.ArpEntriesOk(t, ate, ipType, expectedMacEntries) }, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func testFlow(
	t *testing.T,
	want string,
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
	otg := ate.OTG(t)
	i1 := ateSrc.Name
	i2 := ateDst.Name
	config.Flows().Clear().Items()
	switch ipType {
	case "IPv4":
		flowipv4 := config.Flows().Add().SetName("FlowIpv4")
		flowipv4.Metrics().SetEnable(true)
		flowipv4.TxRx().Device().
			SetTxNames([]string{i1 + ".ipv4"}).
			SetRxNames([]string{i2 + ".ipv4"})
		flowipv4.Size().SetFixed(512)
		flowipv4.Rate().SetPps(trafficPacketRate)
		flowipv4.Duration().SetChoice("continuous")
		e1 := flowipv4.Packet().Add().Ethernet()
		e1.Src().SetValue(ateSrc.MAC)
		v4 := flowipv4.Packet().Add().Ipv4()
		v4.Src().SetValue(ateSrc.IPv4)
		v4.Dst().SetValue(ateDst.IPv4)
		otg.PushConfig(t, ate, config)
		checkOTGArpEntry(t, config, "IPv4", poisoned)
	case "IPv6":
		flowipv6 := config.Flows().Add().SetName("FlowIpv6")
		flowipv6.Metrics().SetEnable(true)
		flowipv6.TxRx().Device().
			SetTxNames([]string{i1 + ".ipv6"}).
			SetRxNames([]string{i2 + ".ipv6"})
		flowipv6.Size().SetFixed(512)
		flowipv6.Rate().SetPps(trafficPacketRate)
		flowipv6.Duration().SetChoice("continuous")
		e1 := flowipv6.Packet().Add().Ethernet()
		e1.Src().SetValue(ateSrc.MAC)
		v4 := flowipv6.Packet().Add().Ipv6()
		v4.Src().SetValue(ateSrc.IPv6)
		v4.Dst().SetValue(ateDst.IPv6)
		otg.PushConfig(t, ate, config)
		checkOTGArpEntry(t, config, "IPv6", poisoned)
	}

	// Starting the traffic
	otg.StartTraffic(t)
	err := helpers.WatchFlowMetrics(t, ate, config, &helpers.WaitForOpts{Interval: 1 * time.Second, Timeout: trafficDuration})
	if err != nil {
		log.Println(err)
	}
	t.Logf("Stop traffic")
	otg.StopTraffic(t)

	// Get the flow statistics
	fMetrics, err := helpers.GetFlowMetrics(t, ate, config)
	if err != nil {
		t.Fatal("Error while getting the flow metrics")
	}

	helpers.PrintMetricsTable(&helpers.MetricsTableOpts{
		ClearPrevious: false,
		FlowMetrics:   fMetrics,
	})

	for _, f := range fMetrics.Items() {
		lossPct := (f.FramesTx() - f.FramesRx()) * 100 / f.FramesTx()
		if lossPct > 0 {
			t.Errorf("LossPct for flow %s got %d, want 0", f.Name(), lossPct)
		}
	}

}

func TestStaticARP(t *testing.T) {
	// First configure the DUT with dynamic ARP.

	interfaceOrder := configureDUT(t, noStaticMAC)
	// var ate *ondatra.ATEDevice
	ate, config := configureOTG(t, interfaceOrder)

	// Default MAC addresses on Ixia are assigned incrementally as:
	//   - 00:11:01:00:00:01
	//   - 00:12:01:00:00:01
	// etc.
	//
	// The last 15-bits therefore resolve to "1".
	t.Run("NotPoisoned", func(t *testing.T) {
		t.Run("IPv4", func(t *testing.T) {
			testFlow(t, "1" /* want */, ate, config, "IPv4", false)
			checkArpEntry(t, "IPv4", false)
		})
		t.Run("IPv6", func(t *testing.T) {
			testFlow(t, "1" /* want */, ate, config, "IPv6", false)
			checkArpEntry(t, "IPv6", false)
		})
	})

	// Reconfigure the DUT with static MAC.
	configureDUT(t, poisonedMAC)

	// Poisoned MAC address ends with 7a:69, so 0x7a69 = 31337.
	t.Run("Poisoned", func(t *testing.T) {
		t.Run("IPv4", func(t *testing.T) {
			testFlow(t, "31337" /* want */, ate, config, "IPv4", true)
			checkArpEntry(t, "IPv4", true)
		})
		t.Run("IPv6", func(t *testing.T) {
			testFlow(t, "31337" /* want */, ate, config, "IPv6", true)
			checkArpEntry(t, "IPv6", true)
		})
	})
}

func TestUnsetDut(t *testing.T) {
	t.Logf("Start Unsetting DUT Config")
	dut := ondatra.DUT(t, "dut")
	dut.Config().New().WithAristaFile("unset_" + dut.Name() + ".txt").Push(t)
}
