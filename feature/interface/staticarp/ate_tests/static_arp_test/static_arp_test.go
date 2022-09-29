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
	"fmt"
	"testing"
	"time"

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
		Name:    "dst",
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

func configureDUT(t *testing.T, peermac string) {
	dut := ondatra.DUT(t, "dut")
	d := dut.Config()

	p1 := dut.Port(t, "port1")
	i1 := &telemetry.Interface{Name: ygot.String(p1.Name())}
	d.Interface(p1.Name()).Replace(t,
		configInterfaceDUT(i1, &dutSrc, &ateSrc, peermac))

	p2 := dut.Port(t, "port2")
	i2 := &telemetry.Interface{Name: ygot.String(p2.Name())}
	d.Interface(p2.Name()).Replace(t,
		configInterfaceDUT(i2, &dutDst, &ateDst, peermac))
}

func configureATE(t *testing.T) (*ondatra.ATEDevice, *ondatra.ATETopology) {
	ate := ondatra.ATE(t, "ate")
	top := ate.Topology().New()

	p1 := ate.Port(t, "port1")
	i1 := top.AddInterface(ateSrc.Name).WithPort(p1)
	i1.IPv4().
		WithAddress(ateSrc.IPv4CIDR()).
		WithDefaultGateway(dutSrc.IPv4)
	i1.IPv6().
		WithAddress(ateSrc.IPv6CIDR()).
		WithDefaultGateway(dutSrc.IPv6)

	p2 := ate.Port(t, "port2")
	i2 := top.AddInterface(ateDst.Name).WithPort(p2)
	i2.IPv4().
		WithAddress(ateDst.IPv4CIDR()).
		WithDefaultGateway(dutDst.IPv4)
	i2.IPv6().
		WithAddress(ateDst.IPv6CIDR()).
		WithDefaultGateway(dutDst.IPv6)

	return ate, top
}

func testFlow(
	t *testing.T,
	want string,
	ate *ondatra.ATEDevice,
	top *ondatra.ATETopology,
	headers ...ondatra.Header,
) {
	i1 := top.Interfaces()[ateSrc.Name]
	i2 := top.Interfaces()[ateDst.Name]

	// Egress tracking inspects packets from DUT and key the flow
	// counters by custom bit offset and width.  Width is limited to
	// 15-bits.
	//
	// Ethernet header:
	//   - Destination MAC (6 octets)
	//   - Source MAC (6 octets)
	//   - Optional 802.1q VLAN tag (4 octets)
	//   - Frame size (2 octets)
	flow := ate.Traffic().NewFlow("Flow").
		WithSrcEndpoints(i1).
		WithDstEndpoints(i2).
		WithHeaders(headers...)
	flow.EgressTracking().WithOffset(33).WithWidth(15)

	ate.Traffic().Start(t, flow)
	time.Sleep(15 * time.Second)
	ate.Traffic().Stop(t)

	flowPath := ate.Telemetry().Flow(flow.Name())

	if got := flowPath.LossPct().Get(t); got > 0 {
		t.Errorf("LossPct for flow %s got %g, want 0", flow.Name(), got)
	}

	etPath := flowPath.EgressTrackingAny()
	ets := etPath.Get(t)
	for i, et := range ets {
		fptest.LogYgot(t, fmt.Sprintf("ATE flow EgressTracking[%d]", i), etPath, et)
	}

	if got := len(ets); got != 1 {
		t.Errorf("EgressTracking got %d items, want 1", got)
		return
	}

	if got := ets[0].GetFilter(); got != want {
		t.Errorf("EgressTracking filter got %q, want %q", got, want)
	}

	if got := ets[0].GetCounters().GetInPkts(); got < 1000 {
		t.Errorf("EgressTracking counter in-pkts got %d, want >= 1000", got)
	}
}

func TestStaticARP(t *testing.T) {
	// First configure the DUT with dynamic ARP.
	configureDUT(t, noStaticMAC)
	ate, top := configureATE(t)
	top.Push(t).StartProtocols(t)

	ethHeader := ondatra.NewEthernetHeader()
	ipv4Header := ondatra.NewIPv4Header()
	ipv6Header := ondatra.NewIPv6Header()

	// Default MAC addresses on Ixia are assigned incrementally as:
	//   - 00:11:01:00:00:01
	//   - 00:12:01:00:00:01
	// etc.
	//
	// The last 15-bits therefore resolve to "1".
	t.Run("NotPoisoned", func(t *testing.T) {
		t.Run("IPv4", func(t *testing.T) {
			testFlow(t, "1" /* want */, ate, top, ethHeader, ipv4Header)
		})
		t.Run("IPv6", func(t *testing.T) {
			testFlow(t, "1" /* want */, ate, top, ethHeader, ipv6Header)
		})
	})

	// Reconfigure the DUT with static MAC.
	configureDUT(t, poisonedMAC)

	// Poisoned MAC address ends with 7a:69, so 0x7a69 = 31337.
	t.Run("Poisoned", func(t *testing.T) {
		t.Run("IPv4", func(t *testing.T) {
			testFlow(t, "31337" /* want */, ate, top, ethHeader, ipv4Header)
		})
		t.Run("IPv6", func(t *testing.T) {
			testFlow(t, "31337" /* want */, ate, top, ethHeader, ipv6Header)
		})
	})
}
