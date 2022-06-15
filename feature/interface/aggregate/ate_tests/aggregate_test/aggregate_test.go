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

package rt_5_2_aggregate_test

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the aggregate testbed with the test
// topology.  IxNetwork flow requires both source and destination
// networks be configured on the ATE.  It is not possible to send
// packets to the ether.
//
// The testbed consists of ate:port1 -> dut:port1 and dut:port{2-9} ->
// ate:port{2-9}.  The first pair is called the "source" pair, and the
// second aggregate link the "destination" pair.
//
//   * Source: ate:port1 -> dut:port1 subnet 192.0.2.0/30 2001:db8::0/126
//   * Destination: dut:port{2-9} -> ate:port{2-9}
//     subnet 192.0.2.4/30 2001:db8::4/126
//
// Note that the first (.0, .4) and last (.3, .7) IPv4 addresses are
// reserved from the subnet for broadcast, so a /30 leaves exactly 2
// usable addresses.  This does not apply to IPv6 which allows /127
// for point to point links, but we use /126 so the numbering is
// consistent with IPv4.
//
// A traffic flow is configured from ate:port1 as source and ate:port{2-9}
// as destination.
const (
	plen4 = 30
	plen6 = 126
)

var (
	dutSrc = attrs.Attributes{
		Desc:    "dutsrc",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	ateSrc = attrs.Attributes{
		Name:    "atesrc",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	dutDst = attrs.Attributes{
		Desc:    "dutdst",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	ateDst = attrs.Attributes{
		Name:    "atedst",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
)

const (
	lagTypeLACP   = telemetry.IfAggregate_AggregationType_LACP
	lagTypeSTATIC = telemetry.IfAggregate_AggregationType_STATIC
)

type testCase struct {
	minlinks uint16
	lagType  telemetry.E_IfAggregate_AggregationType

	dut *ondatra.DUTDevice
	ate *ondatra.ATEDevice
	top *ondatra.ATETopology

	dutPorts []*ondatra.Port
	atePorts []*ondatra.Port
	aggID    string
	l3header []ondatra.Header
}

func (*testCase) configSrcDUT(i *telemetry.Interface, a *attrs.Attributes) {
	i.Description = ygot.String(a.Desc)
	if *deviations.InterfaceEnabled {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if *deviations.InterfaceEnabled {
		s4.Enabled = ygot.Bool(true)
	}
	a4 := s4.GetOrCreateAddress(a.IPv4)
	a4.PrefixLength = ygot.Uint8(plen4)

	s6 := s.GetOrCreateIpv6()
	if *deviations.InterfaceEnabled {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(plen6)
}

func (tc *testCase) configDstAggregateDUT(i *telemetry.Interface, a *attrs.Attributes) {
	tc.configSrcDUT(i, a)
	i.Type = ieee8023adLag
	g := i.GetOrCreateAggregation()
	g.LagType = tc.lagType
	g.MinLinks = ygot.Uint16(tc.minlinks)
}

var portSpeed = map[ondatra.Speed]telemetry.E_IfEthernet_ETHERNET_SPEED{
	ondatra.Speed10Gb:  telemetry.IfEthernet_ETHERNET_SPEED_SPEED_10GB,
	ondatra.Speed100Gb: telemetry.IfEthernet_ETHERNET_SPEED_SPEED_100GB,
	ondatra.Speed400Gb: telemetry.IfEthernet_ETHERNET_SPEED_SPEED_400GB,
}

func (tc *testCase) configDstMemberDUT(i *telemetry.Interface, p *ondatra.Port) {
	i.Description = ygot.String(p.String())
	i.Type = ethernetCsmacd

	if *deviations.InterfaceEnabled {
		i.Enabled = ygot.Bool(true)
	}

	e := i.GetOrCreateEthernet()
	e.AggregateId = ygot.String(tc.aggID)
	// Speed of 99 Gbps is a temporary workaround indicating that the
	// port is a 100G-FR breakout port from a 400G-XDR4.  This relies on
	// the binding to configure the breakout groups for us, so leave the
	// speed setting alone for now.  Blocked by b/200683959.
	if p.Speed() != 99 {
		e.AutoNegotiate = ygot.Bool(false)
		e.DuplexMode = telemetry.Ethernet_DuplexMode_FULL
		e.PortSpeed = portSpeed[p.Speed()]
	}
}

func (tc *testCase) setupAggregateAtomically(t *testing.T) {
	d := &telemetry.Device{}

	if tc.lagType == lagTypeLACP {
		d.GetOrCreateLacp().GetOrCreateInterface(tc.aggID)
	}

	agg := d.GetOrCreateInterface(tc.aggID)
	agg.GetOrCreateAggregation().LagType = tc.lagType
	agg.Type = ieee8023adLag

	for _, port := range tc.dutPorts[1:] {
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(tc.aggID)
	}

	p := tc.dut.Config()
	fptest.LogYgot(t, fmt.Sprintf("%s to Update()", tc.dut), p, d)
	p.Update(t, d)
}

func (tc *testCase) clearAggregateMembers(t *testing.T) {
	for _, port := range tc.dutPorts[1:] {
		tc.dut.Config().Interface(port.Name()).Ethernet().AggregateId().Delete(t)
	}
}

func (tc *testCase) configureDUT(t *testing.T) {
	t.Logf("dut ports = %v", tc.dutPorts)
	if len(tc.dutPorts) < 2 {
		t.Fatalf("Testbed requires at least 2 ports, got %d", len(tc.dutPorts))
	}

	d := tc.dut.Config()

	if *deviations.AggregateAtomicUpdate {
		tc.clearAggregateMembers(t)
		tc.setupAggregateAtomically(t)
	}

	if tc.lagType == lagTypeLACP {
		lacp := &telemetry.Lacp_Interface{Name: ygot.String(tc.aggID)}
		lacp.LacpMode = telemetry.Lacp_LacpActivityType_ACTIVE

		lacpPath := d.Lacp().Interface(tc.aggID)
		fptest.LogYgot(t, "LACP", lacpPath, lacp)
		lacpPath.Replace(t, lacp)
	}

	agg := &telemetry.Interface{Name: ygot.String(tc.aggID)}
	tc.configDstAggregateDUT(agg, &dutDst)
	aggPath := d.Interface(tc.aggID)
	fptest.LogYgot(t, tc.aggID, aggPath, agg)
	aggPath.Replace(t, agg)

	srcp := tc.dutPorts[0]
	srci := &telemetry.Interface{Name: ygot.String(srcp.Name())}
	tc.configSrcDUT(srci, &dutSrc)
	srciPath := d.Interface(srcp.Name())
	fptest.LogYgot(t, srcp.String(), srciPath, srci)
	srciPath.Replace(t, srci)

	for _, port := range tc.dutPorts[1:] {
		i := &telemetry.Interface{Name: ygot.String(port.Name())}
		tc.configDstMemberDUT(i, port)
		iPath := d.Interface(port.Name())
		fptest.LogYgot(t, port.String(), iPath, i)
		iPath.Replace(t, i)
	}
}

func (tc *testCase) configureATE(t *testing.T) {
	if len(tc.atePorts) < 2 {
		t.Fatalf("Testbed requires at least 2 ports, got: %v", tc.atePorts)
	}

	p0 := tc.atePorts[0]
	i0 := tc.top.AddInterface(ateSrc.Name).WithPort(p0)
	i0.IPv4().
		WithAddress(ateSrc.IPv4CIDR()).
		WithDefaultGateway(dutSrc.IPv4)
	i0.IPv6().
		WithAddress(ateSrc.IPv6CIDR()).
		WithDefaultGateway(dutSrc.IPv6)

	// Don't use WithLACPEnabled which is for emulated Ixia LACP.
	agg := tc.top.AddInterface(ateDst.Name)
	lag := tc.top.AddLAG("lag").WithPorts(tc.atePorts[1:]...)
	lag.LACP().WithEnabled(tc.lagType == lagTypeLACP)
	agg.WithLAG(lag)

	// Disable FEC for 100G-FR ports because Novus does not support it.
	if p0.Speed() == 99 {
		i0.Ethernet().FEC().WithEnabled(false)
	}
	is100gfr := false
	for _, p := range tc.atePorts[1:] {
		if p.Speed() == 99 {
			is100gfr = true
		}
	}
	if is100gfr {
		agg.Ethernet().FEC().WithEnabled(false)
	}

	agg.IPv4().
		WithAddress(ateDst.IPv4CIDR()).
		WithDefaultGateway(dutDst.IPv4)
	agg.IPv6().
		WithAddress(ateDst.IPv6CIDR()).
		WithDefaultGateway(dutDst.IPv6)

	// Fail early if the topology is bad.
	tc.top.Push(t)

	ok := false
	for n := 3; !ok; n-- {
		if n == 0 {
			t.Fatal("Not retrying ATE StartProtocols anymore.")
		}
		msg := testt.ExpectFatal(t, func(t testing.TB) {
			t.Log("Trying ATE StartProtocols")
			tc.top.Push(t).StartProtocols(t)
			ok = true
			t.Fatal("Success!")
		})
		t.Logf("ATE StartProtocols: %s", msg)
	}
}

const (
	ethernetCsmacd = telemetry.IETFInterfaces_InterfaceType_ethernetCsmacd
	ieee8023adLag  = telemetry.IETFInterfaces_InterfaceType_ieee8023adLag
	adminUp        = telemetry.Interface_AdminStatus_UP
	opUp           = telemetry.Interface_OperStatus_UP
	opDown         = telemetry.Interface_OperStatus_DOWN
	full           = telemetry.Ethernet_DuplexMode_FULL
	dynamic        = telemetry.IfIp_NeighborOrigin_DYNAMIC
)

func (tc *testCase) verifyLagID(t *testing.T, dp *ondatra.Port) {
	dip := tc.dut.Telemetry().Interface(dp.Name())
	di := dip.Get(t)
	if lagID := di.GetEthernet().GetAggregateId(); lagID != tc.aggID {
		t.Errorf("%s LagID got %v, want %v", dp, lagID, tc.aggID)
	}
}
func (tc *testCase) verifyInterfaceDUT(t *testing.T, dp *ondatra.Port) {
	dip := tc.dut.Telemetry().Interface(dp.Name())
	di := dip.Get(t)
	fptest.LogYgot(t, dp.String(), dip, di)
	if got := di.GetAdminStatus(); got != adminUp {
		t.Errorf("%s admin-status got %v, want %v", dp, got, adminUp)
	}
	if got := di.GetOperStatus(); got != opUp {
		t.Errorf("%s oper-status got %v, want %v", dp, got, opUp)
	}
}
func (tc *testCase) verifyDUT(t *testing.T) {
	for n, port := range tc.dutPorts {
		if n < 1 {
			// We designate port 0 as the source link, not part of LAG.
			t.Run("Source Link Verification", func(t *testing.T) {
				tc.verifyInterfaceDUT(t, port)
			})
			continue
		}
		t.Run("Lag ports verification", func(t *testing.T) {
			tc.verifyInterfaceDUT(t, port)
			tc.verifyLagID(t, port)
		})
	}
	// Verify LAG Type for aggregate interface
	tc.dut.Telemetry().Interface(tc.aggID).Type().Await(t, 10*time.Second, ieee8023adLag)
}

// verifyATE checks the telemetry against the parameters set by
// configureDUT().
func (tc *testCase) verifyATE(t *testing.T) {
	ap := tc.atePorts[0]
	aip := tc.ate.Telemetry().Interface(ap.Name())
	fptest.LogYgot(t, ap.String(), aip, aip.Get(t))

	// State for the interface.
	if got := aip.OperStatus().Get(t); got != opUp {
		t.Errorf("%s oper-status got %v, want %v", ap, got, opUp)
	}
}

// sortPorts sorts the ports by the testbed port ID.
func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})
	return ports
}

func (tc *testCase) verifyMinLinks(t *testing.T) {
	totalPorts := len(tc.dutPorts)
	numLagPorts := totalPorts - 1
	minLinks := uint16(numLagPorts - 1)
	tc.dut.Config().Interface(tc.aggID).Aggregation().MinLinks().Replace(t, minLinks)

	tests := []struct {
		desc      string
		downCount int
		want      telemetry.E_Interface_OperStatus
	}{
		{
			desc:      "MinLink + 1",
			downCount: 0,
			want:      opUp,
		},
		{
			desc:      "MinLink",
			downCount: 1,
			want:      opUp,
		},
		{
			desc:      "MinLink - 1",
			downCount: 2,
			want:      telemetry.Interface_OperStatus_LOWER_LAYER_DOWN,
		},
	}

	for _, tf := range tests {
		t.Run(tf.desc, func(t *testing.T) {
			for _, port := range tc.atePorts[1 : 1+tf.downCount] {
				tc.ate.Actions().NewSetPortState().WithPort(port).WithEnabled(false).Send(t)
				// Linked DUT and ATE ports have the same ID.
				dp := tc.dut.Port(t, port.ID())
				dip := tc.dut.Telemetry().Interface(dp.Name())
				t.Logf("Awaiting DUT port down: %v", dp)
				dip.OperStatus().Await(t, time.Minute, opDown)
				t.Log("Port is down.")
			}
			tc.dut.Telemetry().Interface(tc.aggID).OperStatus().Await(t, 1*time.Minute, tf.want)
		})
	}
}

func TestNegotiation(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	lagTypes := []telemetry.E_IfAggregate_AggregationType{lagTypeLACP, lagTypeSTATIC}

	for _, lagType := range lagTypes {
		top := ate.Topology().New()
		aggID, err := fptest.LAGName(dut.Vendor(), 1001)
		if err != nil {
			t.Fatalf("LAGName for vendor %s: %s", dut.Vendor(), err)
		}

		tc := &testCase{
			minlinks: uint16(len(dut.Ports()) / 2),

			dut:     dut,
			ate:     ate,
			top:     top,
			lagType: lagType,

			dutPorts: sortPorts(dut.Ports()),
			atePorts: sortPorts(ate.Ports()),
			aggID:    aggID,
			l3header: []ondatra.Header{ondatra.NewIPv4Header()},
		}
		t.Run(fmt.Sprintf("LagType=%s", lagType), func(t *testing.T) {
			tc.configureDUT(t)
			tc.configureATE(t)
			t.Run("VerifyATE", tc.verifyATE)
			t.Run("VerifyDUT", tc.verifyDUT)
			t.Run("MinLinks", tc.verifyMinLinks)
		})
	}
}
