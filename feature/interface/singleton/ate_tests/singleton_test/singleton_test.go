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

package singleton_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/confirm"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygot/ygot"

	telemetry "github.com/openconfig/ondatra/telemetry"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// The testbed consists of ate:port1 -> dut:port1 and dut:port2 ->
// ate:port2.  The first pair is called the "source" pair, and the
// second the "destination" pair.  This is for sending traffic flow.
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
// Static MAC addresses on the DUT have the form 02:1a:WW:XX:YY:ZZ
// where WW:XX:YY:ZZ are the four octets of the IPv4 in hex.  The 0x02
// means the MAC address is locally administered.
const (
	plen4 = 30
	plen6 = 126
)

var (
	dutSrc = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		MAC:     "02:1a:c0:00:02:01", // 02:1a+192.0.2.1
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	ateSrc = attrs.Attributes{
		Name:    "src",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
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

// autoMode specifies the type of auto-negotiation behavior testing: forced, auto, and
// auto-negotiation while also specifying duplex and speed.  The last case is permitted by
// IEEE Std 802.3-2012 and OpenConfig.
type autoMode int

const (
	forcedNegotiation autoMode = iota
	autoNegotiation
	autoNegotiationWithDuplexSpeed
)

type testCase struct {
	mtu  uint16 // This is the L3 MTU, i.e. the payload portion of an Ethernet frame.
	auto autoMode

	dut *ondatra.DUTDevice
	ate *ondatra.ATEDevice
	top *ondatra.ATETopology

	// Initialized by configureDUT.
	duti1, duti2 *telemetry.Interface
}

var portSpeed = map[ondatra.Speed]telemetry.E_IfEthernet_ETHERNET_SPEED{
	ondatra.Speed10Gb:  telemetry.IfEthernet_ETHERNET_SPEED_SPEED_10GB,
	ondatra.Speed100Gb: telemetry.IfEthernet_ETHERNET_SPEED_SPEED_100GB,
	ondatra.Speed400Gb: telemetry.IfEthernet_ETHERNET_SPEED_SPEED_400GB,
}

// configInterfaceDUT configures an oc Interface with the desired MTU.
func (tc *testCase) configInterfaceDUT(i *telemetry.Interface, dp *ondatra.Port, a *attrs.Attributes) {
	a.ConfigInterface(i)

	e := i.GetOrCreateEthernet()
	if tc.auto == autoNegotiation || tc.auto == autoNegotiationWithDuplexSpeed {
		e.AutoNegotiate = ygot.Bool(true)
	} else {
		e.AutoNegotiate = ygot.Bool(false)
	}
	if tc.auto == forcedNegotiation || tc.auto == autoNegotiationWithDuplexSpeed {
		if speed, ok := portSpeed[dp.Speed()]; ok {
			e.DuplexMode = telemetry.Ethernet_DuplexMode_FULL
			e.PortSpeed = speed
		}
	}

	if !*deviations.OmitL2MTU {
		i.Mtu = ygot.Uint16(tc.mtu + 14)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	s4.Mtu = ygot.Uint16(tc.mtu)

	s6 := s.GetOrCreateIpv6()
	s6.Mtu = ygot.Uint32(uint32(tc.mtu))
}

func (tc *testCase) configureDUT(t *testing.T) {
	d := tc.dut.Config()

	p1 := tc.dut.Port(t, "port1")
	tc.duti1 = &telemetry.Interface{Name: ygot.String(p1.Name())}
	tc.configInterfaceDUT(tc.duti1, p1, &dutSrc)
	di1 := d.Interface(p1.Name())
	fptest.LogYgot(t, p1.String(), di1, tc.duti1)
	di1.Replace(t, tc.duti1)

	p2 := tc.dut.Port(t, "port2")
	tc.duti2 = &telemetry.Interface{Name: ygot.String(p2.Name())}
	tc.configInterfaceDUT(tc.duti2, p2, &dutDst)
	di2 := d.Interface(p2.Name())
	fptest.LogYgot(t, p2.String(), di2, tc.duti2)
	di2.Replace(t, tc.duti2)
}

func (tc *testCase) configInterfaceATE(ap *ondatra.Port, atea, duta *attrs.Attributes) {
	ateMTU := tc.mtu + 128 // allowance for testing packets > DUT MTU.
	i := atea.AddToATE(tc.top, ap, duta)
	i.Ethernet().WithMTU(ateMTU)
}

func (tc *testCase) configureATE(t *testing.T) {
	ap1 := tc.ate.Port(t, "port1")
	tc.configInterfaceATE(ap1, &ateSrc, &dutSrc)

	ap2 := tc.ate.Port(t, "port2")
	tc.configInterfaceATE(ap2, &ateDst, &dutDst)

	tc.top.Push(t).StartProtocols(t)
}

const (
	ethernetCsmacd = telemetry.IETFInterfaces_InterfaceType_ethernetCsmacd
	adminUp        = telemetry.Interface_AdminStatus_UP
	opUp           = telemetry.Interface_OperStatus_UP
	full           = telemetry.Ethernet_DuplexMode_FULL
	dynamic        = telemetry.IfIp_NeighborOrigin_DYNAMIC
)

func (tc *testCase) verifyInterfaceDUT(
	t *testing.T,
	dp *ondatra.Port,
	wantdi *telemetry.Interface,
	atea *attrs.Attributes,
) {
	dip := tc.dut.Telemetry().Interface(dp.Name())
	di := dip.Get(t)
	fptest.LogYgot(t, dp.String(), dip, di)

	di.PopulateDefaults()
	if tc.mtu == 1500 {
		// MTU default values are still not populated.
		di.GetSubinterface(0).GetIpv4().Mtu = ygot.Uint16(tc.mtu)
		di.GetSubinterface(0).GetIpv6().Mtu = ygot.Uint32(uint32(tc.mtu))
	}
	// According to IEEE Std 802.3-2012 section 22.2.4.1.4, if PHY does not support
	// auto-negotiation, then trying to enable it should be ignored.
	di.GetOrCreateEthernet().AutoNegotiate = wantdi.GetOrCreateEthernet().AutoNegotiate

	confirm.State(t, wantdi, di)

	// State for the interface.
	if got := di.GetAdminStatus(); got != adminUp {
		t.Errorf("%s admin-status got %v, want %v", dp, got, adminUp)
	}
	if got := di.GetOperStatus(); got != opUp {
		t.Errorf("%s oper-status got %v, want %v", dp, got, opUp)
	}

	if speed, ok := portSpeed[dp.Speed()]; ok {
		if tc.auto == forcedNegotiation || tc.auto == autoNegotiationWithDuplexSpeed {
			if got := dip.Ethernet().PortSpeed().Get(t); got != speed {
				t.Errorf("%s port-speed got %v, want %v", dp, got, speed)
			}
		}
		if tc.auto == autoNegotiation || tc.auto == autoNegotiationWithDuplexSpeed {
			if dip.Ethernet().AutoNegotiate().Get(t) {
				// Auto-negotiation is really enabled.
				if got := dip.Ethernet().NegotiatedPortSpeed().Get(t); got != speed {
					t.Errorf("%s negotiated-port-speed got %v, want %v", dp, got, speed)
				}
			}
		}
	}

	disp := dip.Subinterface(0)

	// IPv4 neighbor discovered by ARP.
	dis4np := disp.Ipv4().Neighbor(atea.IPv4)
	if got := dis4np.Origin().Get(t); got != dynamic {
		t.Errorf("%s IPv4 neighbor %s origin got %v, want %v", dp, atea.IPv4, got, dynamic)
	}

	// IPv6 neighbor discovered by ARP.
	dis6np := disp.Ipv6().Neighbor(atea.IPv6)
	if got := dis6np.Origin().Get(t); got != dynamic {
		t.Errorf("%s IPv6 neighbor %s origin got %v, want %v", dp, atea.IPv6, got, dynamic)
	}
}

// verifyDUT checks the telemetry against the parameters set by
// configureDUT().
func (tc *testCase) verifyDUT(t *testing.T) {
	t.Run("Port1", func(t *testing.T) {
		tc.verifyInterfaceDUT(t, tc.dut.Port(t, "port1"), tc.duti1, &ateSrc)
	})
	t.Run("Port2", func(t *testing.T) {
		tc.verifyInterfaceDUT(t, tc.dut.Port(t, "port2"), tc.duti2, &ateDst)
	})
}

func (tc *testCase) verifyInterfaceATE(t *testing.T, ap *ondatra.Port) {
	aip := tc.ate.Telemetry().Interface(ap.Name())
	ai := aip.Get(t)
	fptest.LogYgot(t, ap.String(), aip, ai)

	// State for the interface.
	if got := ai.GetOperStatus(); got != opUp {
		t.Errorf("%s oper-status got %v, want %v", ap, got, opUp)
	}
}

// verifyATE checks the telemetry against the parameters set by
// configureATE().
func (tc *testCase) verifyATE(t *testing.T) {
	t.Run("Port1", func(t *testing.T) {
		tc.verifyInterfaceATE(t, tc.ate.Port(t, "port1"))
	})
	t.Run("Port2", func(t *testing.T) {
		tc.verifyInterfaceATE(t, tc.ate.Port(t, "port2"))
	})
}

type counters struct {
	unicast, multicast, broadcast uint64
}

func inCounters(tic *telemetry.Interface_Counters) *counters {
	return &counters{unicast: tic.GetInUnicastPkts(),
		multicast: tic.GetInMulticastPkts(),
		broadcast: tic.GetInBroadcastPkts()}
}

func outCounters(tic *telemetry.Interface_Counters) *counters {
	return &counters{unicast: tic.GetOutUnicastPkts(),
		multicast: tic.GetOutMulticastPkts(), broadcast: tic.GetOutBroadcastPkts()}
}

func diffCounters(before, after *counters) *counters {
	return &counters{unicast: after.unicast - before.unicast,
		multicast: after.multicast - before.multicast,
		broadcast: after.broadcast - before.broadcast}
}

// testFlow returns whether the traffic flow from ATE port1 to ATE
// port2 has been successfully detected.
func (tc *testCase) testFlow(t *testing.T, packetSize uint16, ipHeader ondatra.Header) bool {
	i1 := tc.top.Interfaces()[ateSrc.Name]
	i2 := tc.top.Interfaces()[ateDst.Name]
	p1 := tc.dut.Port(t, "port1")
	p2 := tc.dut.Port(t, "port2")
	p1Counter := tc.dut.Telemetry().Interface(p1.Name()).Counters()
	p2Counter := tc.dut.Telemetry().Interface(p2.Name()).Counters()

	// Before Traffic Unicast, Multicast, Broadcast Counter
	p1InBefore := inCounters(p1Counter.Get(t))
	p2OutBefore := outCounters(p2Counter.Get(t))

	ethHeader := ondatra.NewEthernetHeader()
	flow := tc.ate.Traffic().NewFlow("flow").
		WithSrcEndpoints(i1).
		WithDstEndpoints(i2).
		WithHeaders(ethHeader, ipHeader)
	flow.WithFrameSize(uint32(packetSize))
	tc.ate.Traffic().Start(t, flow)
	time.Sleep(15 * time.Second)
	tc.ate.Traffic().Stop(t)

	// Counters from ATE interface telemetry may be inaccurate.  Only
	// showing them for diagnostics only.  Use flow telemetry counters
	// for best results.
	{
		ap1 := tc.ate.Port(t, "port1")
		aicp1 := tc.ate.Telemetry().Interface(ap1.Name()).Counters()
		ap2 := tc.ate.Port(t, "port2")
		aicp2 := tc.ate.Telemetry().Interface(ap2.Name()).Counters()
		t.Logf("ap1 out-pkts %d -> ap2 in-pkts %d", aicp1.OutPkts().Get(t), aicp2.InPkts().Get(t))
		t.Logf("ap1 out-octets %d -> ap2 in-octets %d", aicp1.OutOctets().Get(t), aicp2.InOctets().Get(t))
	}

	// After Traffic Unicast, Multicast, Broadcast Counter
	p1InAfter := inCounters(p1Counter.Get(t))
	p2OutAfter := outCounters(p2Counter.Get(t))
	p1InDiff := diffCounters(p1InBefore, p1InAfter)
	p2OutDiff := diffCounters(p2OutBefore, p2OutAfter)

	if p1InDiff.multicast > 100 {
		t.Errorf("Large number of inbound Multicast packets %d, want <= 100)", p1InDiff.multicast)
	}
	if p2OutDiff.multicast > 100 {
		t.Errorf("Large number of outbound Multicast packets %d, want <= 100)", p2OutDiff.multicast)
	}
	if p1InDiff.broadcast > 100 {
		t.Errorf("Large number of inbound Broad packets %d, want <= 100)", p1InDiff.broadcast)
	}
	if p2OutDiff.multicast > 100 {
		t.Errorf("Large number of outbound Broadcast packets %d, want <= 100)", p2OutDiff.broadcast)
	}

	// Flow counters
	fp := tc.ate.Telemetry().Flow(flow.Name())
	fpc := fp.Counters()
	fptest.LogYgot(t, flow.String(), fpc, fpc.Get(t))

	// Pragmatic check on the average in and out packet sizes.  IPv4 may
	// fragment the packet unless DF bit is set.  IPv6 never fragments.
	// Under no circumstances should DUT send packets greater than MTU.

	octets := fpc.InOctets().Get(t) // Flow does not report out-octets.
	outPkts := fpc.OutPkts().Get(t)
	inPkts := fpc.InPkts().Get(t)
	if outPkts == 0 {
		t.Error("Flow did not send any packet")
	} else if avg := octets / outPkts; avg > uint64(tc.mtu) {
		t.Errorf("Flow source packet size average got %d, want <= %d (MTU)", avg, tc.mtu)
	}
	if p1InDiff.unicast < outPkts {
		t.Errorf("DUT received too few source packets: got %d, want >= %d", p1InDiff.unicast, outPkts)
	}

	if inPkts == 0 {
		// The PacketLargerThanMTU cases do not expect to receive packets,
		// so this is not an error.
		t.Log("Flow did not receive any packet")
	} else if avg := octets / inPkts; avg > uint64(tc.mtu) {
		t.Errorf("Flow destination packet size average got %d, want <= %d (MTU)", avg, tc.mtu)
	}
	if inPkts < p2OutDiff.unicast {
		t.Errorf("ATE received too few destination packets: got %d, want >= %d", inPkts, p2OutDiff.unicast)
	}
	t.Logf("flow loss-pct %f", fp.LossPct().Get(t))
	return fp.LossPct().Get(t) < 0.5 // 0.5% loss.
}

func (tc *testCase) testMTU(t *testing.T) {
	tc.configureDUT(t)
	tc.configureATE(t)

	t.Run("VerifyDUT", func(t *testing.T) { tc.verifyDUT(t) })
	t.Run("VerifyATE", func(t *testing.T) { tc.verifyATE(t) })

	for _, c := range []struct {
		ipName     string
		shouldFrag bool
		ipHeader   ondatra.Header
	}{
		{"IPv4", true, ondatra.NewIPv4Header()},
		{"IPv4-DF", false, ondatra.NewIPv4Header().WithDontFragment(true)},
		{"IPv6", false, ondatra.NewIPv6Header()},
	} {
		t.Run(c.ipName, func(t *testing.T) {
			t.Run("PacketLargerThanMTU", func(t *testing.T) {
				if c.shouldFrag {
					t.Skip("Packet fragmentation is not expected at line rate.")
				}
				if got := tc.testFlow(t, tc.mtu+64, c.ipHeader); got {
					t.Errorf("Traffic flow got %v, want false", got)
				}
			})
			t.Run("PacketExactlyMTU", func(t *testing.T) {
				if got := tc.testFlow(t, tc.mtu, c.ipHeader); !got {
					t.Errorf("Traffic flow got %v, want true", got)
				}
			})
			t.Run("PacketSmallerThanMTU", func(t *testing.T) {
				if got := tc.testFlow(t, tc.mtu-64, c.ipHeader); !got {
					t.Errorf("Traffic flow got %v, want true", got)
				}
			})
		})
	}
}

func TestMTUs(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	// These are the L3 MTUs, i.e. the payload portion of an Ethernet frame.
	mtus := []uint16{1500, 5000, 9212}

	for _, mtu := range mtus {
		top := ate.Topology().New()
		tc := &testCase{
			mtu:  mtu,
			auto: forcedNegotiation,

			dut: dut,
			ate: ate,
			top: top,
		}
		t.Run(fmt.Sprintf("MTU=%d", mtu), tc.testMTU)
	}
}

var autoModeName = map[autoMode]string{
	forcedNegotiation:              "Forced",
	autoNegotiation:                "Auto",
	autoNegotiationWithDuplexSpeed: "AutoWithDuplexSpeed",
}

// TestNegotiate validates that port speed is reported correctly and that port telemetry
// atches expected negotiated speeds for forced, auto-negotiation, and auto-negotiation
// while overriding port speed and duplex.
func TestNegotiate(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	for auto, name := range autoModeName {
		t.Run(name, func(t *testing.T) {
			top := ate.Topology().New()
			tc := &testCase{
				mtu:  1500,
				auto: auto,

				dut: dut,
				ate: ate,
				top: top,
			}

			tc.configureDUT(t)
			tc.configureATE(t)

			t.Run("VerifyDUT", func(t *testing.T) { tc.verifyDUT(t) })
			t.Run("VerifyATE", func(t *testing.T) { tc.verifyATE(t) })
			t.Run("Traffic", func(t *testing.T) {
				if got := tc.testFlow(t, tc.mtu, ondatra.NewIPv6Header()); !got {
					t.Errorf("Traffic flow got %v, want true", got)
				}
			})
		})
	}
}
