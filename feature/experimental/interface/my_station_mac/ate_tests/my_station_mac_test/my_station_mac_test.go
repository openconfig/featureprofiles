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

package my_station_mac_test

import (
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

//
// The testbed consists of ate:port1 -> dut:port1 and
// dut:port2 -> ate:port2.  The first pair is called the "source"
// pair, and the second the "destination" pair.
//
//   - Source: ate:port1 -> dut:port1 subnet 192.0.2.0/30 2001:db8::0/126
//   - Destination: dut:port2 -> ate:port2 subnet 192.0.2.4/30 2001:db8::4/126
//

const (
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126
	myStationMAC  = "00:1A:11:00:00:01"
)

var (
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutSrc = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutDst = attrs.Attributes{
		Desc:    "DUT to ATE destination",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	ateDst = attrs.Attributes{
		Name:    "ateDst",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
)

// configInterfaceDUT configures the DUT interfaces.
func configInterfaceDUT(i *oc.Interface, a *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(ipv4PrefixLen)

	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(ipv6PrefixLen)

	return i
}

// configureDUT configures port1 and port2 on the DUT.
func configureDUT(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutSrc, dut))

	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutDst, dut))

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

// configureATE configures port1 and port2 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
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

	return top
}

// testTraffic generates and verifies traffic flow with destination MAC as MyStationMAC.
func testTraffic(
	t *testing.T,
	pktLossPct float32,
	ate *ondatra.ATEDevice,
	top *ondatra.ATETopology,
	headers ...ondatra.Header,
) {
	i1 := top.Interfaces()[ateSrc.Name]
	i2 := top.Interfaces()[ateDst.Name]

	flow := ate.Traffic().NewFlow("Flow").
		WithSrcEndpoints(i1).
		WithDstEndpoints(i2).
		WithHeaders(headers...)

	ate.Traffic().Start(t, flow)
	time.Sleep(10 * time.Second)
	ate.Traffic().Stop(t)

	flowPath := gnmi.OC().Flow(flow.Name())

	if got := gnmi.Get(t, ate, flowPath.LossPct().State()); got != pktLossPct {
		t.Errorf("Packet loss percentage for flow %s: got %g, want %g", flow.Name(), got, pktLossPct)
	}
}

// TestMyStationMAC verifies that MyStationMAC installed on the DUT is honored and used for routing.
func TestMyStationMAC(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Logf("Configure DUT")
	configureDUT(t)

	t.Logf("Configure ATE")
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	top.Push(t).StartProtocols(t)

	t.Logf("Configure MyStationMAC")
	gnmi.Replace(t, dut, gnmi.OC().System().MacAddress().RoutingMac().Config(), myStationMAC)

	if !deviations.MacAddressMissing(dut) {
		t.Logf("Verify configured MyStationMAC through telemetry")
		if got := gnmi.Get(t, dut, gnmi.OC().System().MacAddress().RoutingMac().State()); strings.ToUpper(got) != myStationMAC {
			t.Errorf("MyStationMAC got %v, want %v", got, myStationMAC)
		}
	}

	t.Logf("Verify traffic flow")

	ethHeader := ondatra.NewEthernetHeader()
	ethHeader.WithDstAddress(myStationMAC)

	ipv4Header := ondatra.NewIPv4Header()
	ipv6Header := ondatra.NewIPv6Header()

	t.Run("With MyStationMAC", func(t *testing.T) {
		t.Run("IPv4", func(t *testing.T) {
			testTraffic(t, 0 /* pkt loss percent */, ate, top, ethHeader, ipv4Header)
		})
		t.Run("IPv6", func(t *testing.T) {
			testTraffic(t, 0 /* pkt loss percent */, ate, top, ethHeader, ipv6Header)
		})
	})

	t.Logf("Remove MyStationMAC configuraiton")
	gnmi.Delete(t, dut, gnmi.OC().System().MacAddress().RoutingMac().Config())

	t.Run("Without MyStationMAC", func(t *testing.T) {
		t.Run("IPv4", func(t *testing.T) {
			testTraffic(t, 100 /* pkt loss percent */, ate, top, ethHeader, ipv4Header)
		})
		t.Run("IPv6", func(t *testing.T) {
			testTraffic(t, 100 /* pkt loss percent */, ate, top, ethHeader, ipv6Header)
		})
	})

}
