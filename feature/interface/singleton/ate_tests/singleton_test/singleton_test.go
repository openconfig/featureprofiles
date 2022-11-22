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

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/confirm"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
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

	dut           *ondatra.DUTDevice
	ate           *ondatra.ATEDevice
	top           *ondatra.ATETopology
	breakoutPorts map[string][]string
	// Initialized by configureDUT.
	duti1, duti2 *oc.Interface
}

var portSpeed = map[ondatra.Speed]oc.E_IfEthernet_ETHERNET_SPEED{
	ondatra.Speed10Gb:  oc.IfEthernet_ETHERNET_SPEED_SPEED_10GB,
	ondatra.Speed100Gb: oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB,
	ondatra.Speed400Gb: oc.IfEthernet_ETHERNET_SPEED_SPEED_400GB,
}

// configInterfaceDUT configures an oc Interface with the desired MTU.
func (tc *testCase) configInterfaceDUT(i *oc.Interface, dp *ondatra.Port, a *attrs.Attributes) {
	a.ConfigOCInterface(i)

	e := i.GetOrCreateEthernet()
	if tc.auto == autoNegotiation || tc.auto == autoNegotiationWithDuplexSpeed {
		e.AutoNegotiate = ygot.Bool(true)
	} else {
		e.AutoNegotiate = ygot.Bool(false)
	}
	if tc.auto == forcedNegotiation || tc.auto == autoNegotiationWithDuplexSpeed {
		if speed, ok := portSpeed[dp.Speed()]; ok {
			e.DuplexMode = oc.Ethernet_DuplexMode_FULL
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

func (tc *testCase) configureDUTBreakout(t *testing.T) *telemetry.Component_Port_BreakoutMode_Group {
	t.Helper()
	d := tc.dut.Config()
	tc.breakoutPorts = make(map[string][]string)

	for _, dp := range tc.dut.Ports() {
		// TODO(liulk): figure out a better way to detect breakout port.
		if dp.PMD() != ondatra.PMD100GBASEFR {
			continue
		}
		parent := tc.dut.Telemetry().Interface(dp.Name()).HardwarePort().Get(t)
		tc.breakoutPorts[parent] = append(tc.breakoutPorts[parent], dp.Name())
	}
	var group *telemetry.Component_Port_BreakoutMode_Group
	for physical := range tc.breakoutPorts {
		bmode := &telemetry.Component_Port_BreakoutMode{}
		bmp := d.Component(physical).Port().BreakoutMode()
		group = bmode.GetOrCreateGroup(0)
		// TODO(liulk): use one of the logical port.Speed().
		group.BreakoutSpeed = telemetry.IfEthernet_ETHERNET_SPEED_SPEED_100GB
		group.NumBreakouts = ygot.Uint8(4)
		bmp.Replace(t, bmode)
	}
	return group

}

func (tc *testCase) configureDUT(t *testing.T) {
	d := gnmi.OC()

	p1 := tc.dut.Port(t, "port1")
	tc.duti1 = &oc.Interface{Name: ygot.String(p1.Name())}
	tc.configInterfaceDUT(tc.duti1, p1, &dutSrc)
	di1 := d.Interface(p1.Name())
	fptest.LogQuery(t, p1.String(), di1.Config(), tc.duti1)
	gnmi.Replace(t, tc.dut, di1.Config(), tc.duti1)

	p2 := tc.dut.Port(t, "port2")
	tc.duti2 = &oc.Interface{Name: ygot.String(p2.Name())}
	tc.configInterfaceDUT(tc.duti2, p2, &dutDst)
	di2 := d.Interface(p2.Name())
	fptest.LogQuery(t, p2.String(), di2.Config(), tc.duti2)
	gnmi.Replace(t, tc.dut, di2.Config(), tc.duti2)
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
	ethernetCsmacd = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	adminUp        = oc.Interface_AdminStatus_UP
	opUp           = oc.Interface_OperStatus_UP
	full           = oc.Ethernet_DuplexMode_FULL
	dynamic        = oc.IfIp_NeighborOrigin_DYNAMIC
)

func (tc *testCase) verifyInterfaceDUT(
	t *testing.T,
	dp *ondatra.Port,
	wantdi *oc.Interface,
	atea *attrs.Attributes,
) {
	dip := gnmi.OC().Interface(dp.Name())
	di := gnmi.Get(t, tc.dut, dip.State())
	fptest.LogQuery(t, dp.String(), dip.State(), di)

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
			if got := gnmi.Get(t, tc.dut, dip.Ethernet().PortSpeed().State()); got != speed {
				t.Errorf("%s port-speed got %v, want %v", dp, got, speed)
			}
		}
		if tc.auto == autoNegotiation || tc.auto == autoNegotiationWithDuplexSpeed {
			if gnmi.Get(t, tc.dut, dip.Ethernet().AutoNegotiate().State()) {
				// Auto-negotiation is really enabled.
				if got := gnmi.Get(t, tc.dut, dip.Ethernet().NegotiatedPortSpeed().State()); got != speed {
					t.Errorf("%s negotiated-port-speed got %v, want %v", dp, got, speed)
				}
			}
		}
	}

	disp := dip.Subinterface(0)

	// IPv4 neighbor discovered by ARP.
	dis4np := disp.Ipv4().Neighbor(atea.IPv4)
	if got := gnmi.Get(t, tc.dut, dis4np.Origin().State()); got != dynamic {
		t.Errorf("%s IPv4 neighbor %s origin got %v, want %v", dp, atea.IPv4, got, dynamic)
	}

	// IPv6 neighbor discovered by ARP.
	dis6np := disp.Ipv6().Neighbor(atea.IPv6)
	if got := gnmi.Get(t, tc.dut, dis6np.Origin().State()); got != dynamic {
		t.Errorf("%s IPv6 neighbor %s origin got %v, want %v", dp, atea.IPv6, got, dynamic)
	}
}

// verifyDUT checks the telemetry against the parameters set by
// configureDUT().
func (tc *testCase) verifyDUT(t *testing.T, breakoutGroup *telemetry.Component_Port_BreakoutMode_Group) {
	t.Run("Port1", func(t *testing.T) {
		tc.verifyInterfaceDUT(t, tc.dut.Port(t, "port1"), tc.duti1, &ateSrc)
	})
	t.Run("Port2", func(t *testing.T) {
		tc.verifyInterfaceDUT(t, tc.dut.Port(t, "port2"), tc.duti2, &ateDst)
	})
	t.Run("Breakout", func(t *testing.T) {
		for physical := range tc.breakoutPorts {
			if physical == "" {
				continue // Not a breakout.
			}
			const want = 4
			got := breakoutGroup.GetNumBreakouts()
			if !cmp.Equal(got, want) {
				t.Errorf("number of brekaoutports  = %v, want = %v", got, want)
			}
		}
	})
}

func (tc *testCase) verifyInterfaceATE(t *testing.T, ap *ondatra.Port) {
	aip := gnmi.OC().Interface(ap.Name())
	ai := gnmi.Get(t, tc.ate, aip.State())
	fptest.LogQuery(t, ap.String(), aip.State(), ai)

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

func inCounters(tic *oc.Interface_Counters) *counters {
	return &counters{unicast: tic.GetInUnicastPkts(),
		multicast: tic.GetInMulticastPkts(),
		broadcast: tic.GetInBroadcastPkts()}
}

func outCounters(tic *oc.Interface_Counters) *counters {
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
	p1Counter := gnmi.OC().Interface(p1.Name()).Counters()
	p2Counter := gnmi.OC().Interface(p2.Name()).Counters()

	// Before Traffic Unicast, Multicast, Broadcast Counter
	p1InBefore := inCounters(gnmi.Get(t, tc.dut, p1Counter.State()))
	p2OutBefore := outCounters(gnmi.Get(t, tc.dut, p2Counter.State()))

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
		aicp1 := gnmi.OC().Interface(ap1.Name()).Counters()
		ap2 := tc.ate.Port(t, "port2")
		aicp2 := gnmi.OC().Interface(ap2.Name()).Counters()
		t.Logf("ap1 out-pkts %d -> ap2 in-pkts %d", gnmi.Get(t, tc.ate, aicp1.OutPkts().State()), gnmi.Get(t, tc.ate, aicp2.InPkts().State()))
		t.Logf("ap1 out-octets %d -> ap2 in-octets %d", gnmi.Get(t, tc.ate, aicp1.OutOctets().State()), gnmi.Get(t, tc.ate, aicp2.InOctets().State()))
	}

	// After Traffic Unicast, Multicast, Broadcast Counter
	p1InAfter := inCounters(gnmi.Get(t, tc.dut, p1Counter.State()))
	p2OutAfter := outCounters(gnmi.Get(t, tc.dut, p2Counter.State()))
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
	fp := gnmi.OC().Flow(flow.Name())
	fpc := fp.Counters()
	fptest.LogQuery(t, flow.String(), fpc.State(), gnmi.Get(t, tc.ate, fpc.State()))

	// Pragmatic check on the average in and out packet sizes.  IPv4 may
	// fragment the packet unless DF bit is set.  IPv6 never fragments.
	// Under no circumstances should DUT send packets greater than MTU.

	octets := gnmi.Get(t, tc.ate, fpc.InOctets().State()) // Flow does not report out-octets.
	outPkts := gnmi.Get(t, tc.ate, fpc.OutPkts().State())
	inPkts := gnmi.Get(t, tc.ate, fpc.InPkts().State())
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
	t.Logf("flow loss-pct %f", gnmi.Get(t, tc.ate, fp.LossPct().State()))
	return gnmi.Get(t, tc.ate, fp.LossPct().State()) < 0.5 // 0.5% loss.
}

func (tc *testCase) testMTU(t *testing.T) {
	tc.configureDUT(t)
	tc.configureATE(t)
	breakoutGroup := tc.configureDUTBreakout(t)

	t.Run("VerifyDUT", func(t *testing.T) { tc.verifyDUT(t, breakoutGroup) })
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
			breakoutGroup := tc.configureDUTBreakout(t)
			tc.configureDUT(t)
			tc.configureATE(t)

			t.Run("VerifyDUT", func(t *testing.T) { tc.verifyDUT(t, breakoutGroup) })
			t.Run("VerifyATE", func(t *testing.T) { tc.verifyATE(t) })
			t.Run("Traffic", func(t *testing.T) {
				if got := tc.testFlow(t, tc.mtu, ondatra.NewIPv6Header()); !got {
					t.Errorf("Traffic flow got %v, want true", got)
				}
			})
		})
	}
}
