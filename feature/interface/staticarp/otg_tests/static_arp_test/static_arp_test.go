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
	"strings"
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

	poisonedMAC = "12:34:56:78:7a:69" // 0xa69 = 2665
	noStaticMAC = ""
)

var (
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		MAC:     "02:11:01:00:01:01",
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
		MAC:     "02:12:01:00:02:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
)

// configInterfaceDUT configures the interface on "me" with static ARP
// of peer.  Note that peermac is used for static ARP, and not
// peer.MAC.
func configInterfaceDUT(t *testing.T, p *ondatra.Port, me, peer *attrs.Attributes, peermac string, dut *ondatra.DUTDevice) *oc.Interface {
	i := &oc.Interface{Name: ygot.String(p.Name())}
	i.Description = ygot.String(me.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	if deviations.ExplicitPortSpeed(dut) {
		e := i.GetOrCreateEthernet()
		e.PortSpeed = fptest.GetIfSpeed(t, p)
	}

	if me.MAC != "" {
		e := i.GetOrCreateEthernet()
		e.MacAddress = ygot.String(me.MAC)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	a := s4.GetOrCreateAddress(me.IPv4)
	a.PrefixLength = ygot.Uint8(plen4)

	if peermac != noStaticMAC {
		n4 := s4.GetOrCreateNeighbor(peer.IPv4)
		n4.LinkLayerAddress = ygot.String(peermac)
	}

	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
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
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(t, p1, &dutSrc, &ateSrc, peermac, dut))
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}

	p2 := dut.Port(t, "port2")
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(t, p2, &dutDst, &ateDst, peermac, dut))
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

func configureATE(t *testing.T) gosnappi.Config {
	ate := ondatra.ATE(t, "ate")
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")

	config := gosnappi.NewConfig()
	ateSrc.AddToOTG(config, ap1, &dutSrc)
	ateDst.AddToOTG(config, ap2, &dutDst)

	ate.OTG().PushConfig(t, config)
	return config
}

// Extract the hex equivalent last 12 bits
func getMacFilter(mac string) string {
	newMac := strings.Replace(mac, ":", "", -1)
	return "0x" + newMac[9:]
}

func testFlow(
	t *testing.T,
	ate *ondatra.ATEDevice,
	config gosnappi.Config,
	ipType string,
	dstMac string,
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
	config.Flows().Clear().Items()
	var flow gosnappi.Flow
	switch ipType {
	case "IPv4":
		flow = config.Flows().Add().SetName("FlowIPv4")
		flow.Metrics().SetEnable(true)
		flow.TxRx().Port().
			SetTxName("port1").
			SetRxNames([]string{"port2"})
		e1 := flow.Packet().Add().Ethernet()
		e1.Src().SetValue(ateSrc.MAC)
		e1.Dst().SetValue(dstMac)
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(ateSrc.IPv4)
		v4.Dst().SetValue(ateDst.IPv4)
	case "IPv6":
		flow = config.Flows().Add().SetName("FlowIPv6")
		flow.Metrics().SetEnable(true)
		flow.TxRx().Port().
			SetTxName("port1").
			SetRxNames([]string{"port2"})
		e1 := flow.Packet().Add().Ethernet()
		e1.Src().SetValue(ateSrc.MAC)
		e1.Dst().SetValue(dstMac)
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(ateSrc.IPv6)
		v6.Dst().SetValue(ateDst.IPv6)
	}
	flow.Duration().SetChoice("fixed_packets")
	flow.Duration().FixedPackets().SetPackets(1000)
	flow.Size().SetFixed(100)
	eth := flow.EgressPacket().Add().Ethernet()
	ethTag := eth.Dst().MetricTags().Add()
	ethTag.SetName("EgressTrackingFlow").SetOffset(36).SetLength(12)
	otg.PushConfig(t, config)
	otg.StartProtocols(t)

	// Starting the traffic
	otg.StartTraffic(t)
	time.Sleep(15 * time.Second)
	t.Logf("Stop traffic")
	otg.StopTraffic(t)

	// Get the flow statistics
	otgutils.LogFlowMetrics(t, otg, config)
	for _, f := range config.Flows().Items() {
		txPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(f.Name()).Counters().OutPkts().State())
		rxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(f.Name()).Counters().InPkts().State())
		lossPct := float32(txPkts-rxPkts) * 100 / float32(txPkts)
		if txPkts == 0 {
			t.Fatalf("TxPkts == 0, want > 0.")
		}
		if lossPct > 0 {
			t.Errorf("LossPct for flow %s detected - got %v expected 0", f.Name(), lossPct)
		} else {
			var macFilter string
			if poisoned {
				macFilter = getMacFilter(poisonedMAC)
			} else {
				macFilter = getMacFilter(ateDst.MAC)
			}
			// Check the egress packets
			etPath := gnmi.OTG().Flow(f.Name()).TaggedMetricAny()
			ets := gnmi.GetAll(t, ate.OTG(), etPath.State())
			if got := len(ets); got != 1 {
				t.Errorf("EgressTracking got %d items, want %d", got, 1)
			}
			etTagspath := gnmi.OTG().Flow(f.Name()).TaggedMetricAny().TagsAny()
			etTags := gnmi.GetAll(t, ate.OTG(), etTagspath.State())
			if got := etTags[0].GetTagValue().GetValueAsHex(); !strings.EqualFold(got, macFilter) {
				t.Errorf("EgressTracking filter got %q, want %q", got, macFilter)
			}
			if got := ets[0].GetCounters().GetInPkts(); got != rxPkts {
				t.Errorf("EgressTracking counter in-pkts got %d, want %d", got, rxPkts)
			} else {
				t.Logf("Received %d packets with %s as the last 12 bits in the dst MAC", got, macFilter)
			}
		}
	}
}

func TestStaticARP(t *testing.T) {
	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	config := configureATE(t)

	// Configure the DUT with dynamic ARP.
	configureDUT(t, noStaticMAC)

	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), config, "IPv4")
	dstMac := gnmi.Get(t, ate.OTG(), gnmi.OTG().Interface(ateSrc.Name+".Eth").Ipv4Neighbor(dutSrc.IPv4).LinkLayerAddress().State())

	t.Run("NotPoisoned", func(t *testing.T) {
		t.Run("IPv4", func(t *testing.T) {
			testFlow(t, ate, config, "IPv4", dstMac, false)
		})
		t.Run("IPv6", func(t *testing.T) {
			testFlow(t, ate, config, "IPv6", dstMac, false)
		})
	})

	// Reconfigure the DUT with static MAC.
	configureDUT(t, poisonedMAC)

	t.Run("Poisoned", func(t *testing.T) {
		t.Run("IPv4", func(t *testing.T) {
			testFlow(t, ate, config, "IPv4", dstMac, true)
		})
		t.Run("IPv6", func(t *testing.T) {
			testFlow(t, ate, config, "IPv6", dstMac, true)
		})
	})
}
