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
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
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
//   - Source: ate:port1 -> dut:port1 subnet 192.0.2.0/30 2001:db8::0/126
//   - Destination: dut:port{2-9} -> ate:port{2-9}
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
	lagTypeLACP   = oc.IfAggregate_AggregationType_LACP
	lagTypeSTATIC = oc.IfAggregate_AggregationType_STATIC
)

type testCase struct {
	lagType oc.E_IfAggregate_AggregationType

	dut *ondatra.DUTDevice
	ate *ondatra.ATEDevice
	top *ondatra.ATETopology

	dutPorts []*ondatra.Port
	atePorts []*ondatra.Port
	aggID    string
	l3header []ondatra.Header
}

func (*testCase) configSrcDUT(i *oc.Interface, a *attrs.Attributes) {
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

func (tc *testCase) configDstAggregateDUT(i *oc.Interface, a *attrs.Attributes) {
	tc.configSrcDUT(i, a)
	i.Type = ieee8023adLag
	g := i.GetOrCreateAggregation()
	g.LagType = tc.lagType
}

func (tc *testCase) configDstMemberDUT(i *oc.Interface, p *ondatra.Port) {
	i.Description = ygot.String(p.String())
	i.Type = ethernetCsmacd

	if *deviations.InterfaceEnabled {
		i.Enabled = ygot.Bool(true)
	}

	e := i.GetOrCreateEthernet()
	e.AggregateId = ygot.String(tc.aggID)
}

func (tc *testCase) setupAggregateAtomically(t *testing.T) {
	d := &oc.Root{}

	if tc.lagType == lagTypeLACP {
		d.GetOrCreateLacp().GetOrCreateInterface(tc.aggID)
	}

	agg := d.GetOrCreateInterface(tc.aggID)
	agg.GetOrCreateAggregation().LagType = tc.lagType
	agg.Type = ieee8023adLag

	for _, port := range tc.dutPorts[1:] {
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(tc.aggID)
		i.Type = ethernetCsmacd

		if *deviations.InterfaceEnabled {
			i.Enabled = ygot.Bool(true)
		}
	}

	p := gnmi.OC()
	fptest.LogQuery(t, fmt.Sprintf("%s to Update()", tc.dut), p.Config(), d)
	gnmi.Update(t, tc.dut, p.Config(), d)
}

func (tc *testCase) clearAggregate(t *testing.T) {
	// Clear the aggregate minlink.
	gnmi.Delete(t, tc.dut, gnmi.OC().Interface(tc.aggID).Aggregation().MinLinks().Config())

	// Clear the members of the aggregate.
	for _, port := range tc.dutPorts[1:] {
		gnmi.Delete(t, tc.dut, gnmi.OC().Interface(port.Name()).Ethernet().AggregateId().Config())
	}
}

func (tc *testCase) configureDUT(t *testing.T) {
	t.Logf("dut ports = %v", tc.dutPorts)
	if len(tc.dutPorts) < 2 {
		t.Fatalf("Testbed requires at least 2 ports, got %d", len(tc.dutPorts))
	}

	d := gnmi.OC()

	if *deviations.AggregateAtomicUpdate {
		tc.clearAggregate(t)
		tc.setupAggregateAtomically(t)
	}

	lacp := &oc.Lacp_Interface{Name: ygot.String(tc.aggID)}
	if tc.lagType == lagTypeLACP {
		lacp.LacpMode = oc.Lacp_LacpActivityType_ACTIVE
	} else {
		lacp.LacpMode = oc.Lacp_LacpActivityType_UNSET
	}
	lacpPath := d.Lacp().Interface(tc.aggID)
	fptest.LogQuery(t, "LACP", lacpPath.Config(), lacp)
	gnmi.Replace(t, tc.dut, lacpPath.Config(), lacp)

	agg := &oc.Interface{Name: ygot.String(tc.aggID)}
	tc.configDstAggregateDUT(agg, &dutDst)
	aggPath := d.Interface(tc.aggID)
	fptest.LogQuery(t, tc.aggID, aggPath.Config(), agg)
	gnmi.Replace(t, tc.dut, aggPath.Config(), agg)

	srcp := tc.dutPorts[0]
	srci := &oc.Interface{Name: ygot.String(srcp.Name())}
	tc.configSrcDUT(srci, &dutSrc)
	srci.Type = ethernetCsmacd
	srciPath := d.Interface(srcp.Name())
	fptest.LogQuery(t, srcp.String(), srciPath.Config(), srci)
	gnmi.Replace(t, tc.dut, srciPath.Config(), srci)

	for _, port := range tc.dutPorts[1:] {
		i := &oc.Interface{Name: ygot.String(port.Name())}
		i.Type = ethernetCsmacd

		if *deviations.InterfaceEnabled {
			i.Enabled = ygot.Bool(true)
		}
		tc.configDstMemberDUT(i, port)
		iPath := d.Interface(port.Name())
		fptest.LogQuery(t, port.String(), iPath.Config(), i)
		gnmi.Replace(t, tc.dut, iPath.Config(), i)
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
	if p0.PMD() == ondatra.PMD100GBASEFR {
		i0.Ethernet().FEC().WithEnabled(false)
	}
	is100gfr := false
	for _, p := range tc.atePorts[1:] {
		if p.PMD() == ondatra.PMD100GBASEFR {
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

	tc.top.Push(t).StartProtocols(t)
}

const (
	ethernetCsmacd = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	ieee8023adLag  = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	adminUp        = oc.Interface_AdminStatus_UP
	opUp           = oc.Interface_OperStatus_UP
	opDown         = oc.Interface_OperStatus_DOWN
	full           = oc.Ethernet_DuplexMode_FULL
	dynamic        = oc.IfIp_NeighborOrigin_DYNAMIC
)

func (tc *testCase) verifyAggID(t *testing.T, dp *ondatra.Port) {
	dip := gnmi.OC().Interface(dp.Name())
	di := gnmi.Get(t, tc.dut, dip.State())
	if lagID := di.GetEthernet().GetAggregateId(); lagID != tc.aggID {
		t.Errorf("%s LagID got %v, want %v", dp, lagID, tc.aggID)
	}
}

func (tc *testCase) verifyInterfaceDUT(t *testing.T, dp *ondatra.Port) {
	dip := gnmi.OC().Interface(dp.Name())
	di := gnmi.Get(t, tc.dut, dip.State())
	fptest.LogQuery(t, dp.String()+" before Await", dip.State(), di)

	if got := di.GetAdminStatus(); got != adminUp {
		t.Errorf("%s admin-status got %v, want %v", dp, got, adminUp)
	}

	// LAG members may fall behind, so wait for them to be up.
	gnmi.Await(t, tc.dut, dip.OperStatus().State(), time.Minute, opUp)
}

func (tc *testCase) verifyDUT(t *testing.T) {
	// Wait for LAG negotiation and verify LAG type for the aggregate interface.
	gnmi.Await(t, tc.dut, gnmi.OC().Interface(tc.aggID).Type().State(), time.Minute, ieee8023adLag)

	for n, port := range tc.dutPorts {
		if n < 1 {
			// We designate port 0 as the source link, not part of LAG.
			t.Run(fmt.Sprintf("%s [source]", port.ID()), func(t *testing.T) {
				tc.verifyInterfaceDUT(t, port)
			})
			continue
		}
		t.Run(fmt.Sprintf("%s [member]", port.ID()), func(t *testing.T) {
			tc.verifyInterfaceDUT(t, port)
			tc.verifyAggID(t, port)
		})
	}
}

// verifyATE checks the telemetry against the parameters set by
// configureDUT().
func (tc *testCase) verifyATE(t *testing.T) {
	ap := tc.atePorts[0]
	aip := gnmi.OC().Interface(ap.Name())
	fptest.LogQuery(t, ap.String(), aip.State(), gnmi.Get(t, tc.ate, aip.State()))

	// State for the interface.
	if got := gnmi.Get(t, tc.ate, aip.OperStatus().State()); got != opUp {
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
	gnmi.Replace(t, tc.dut, gnmi.OC().Interface(tc.aggID).Aggregation().MinLinks().Config(), minLinks)

	tests := []struct {
		desc      string
		downCount int
		want      oc.E_Interface_OperStatus
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
			want:      oc.Interface_OperStatus_LOWER_LAYER_DOWN,
		},
	}

	for _, tf := range tests {
		t.Run(tf.desc, func(t *testing.T) {
			for _, port := range tc.atePorts[1 : 1+tf.downCount] {
				tc.ate.Actions().NewSetPortState().WithPort(port).WithEnabled(false).Send(t)
				// Linked DUT and ATE ports have the same ID.
				dp := tc.dut.Port(t, port.ID())
				dip := gnmi.OC().Interface(dp.Name())
				t.Logf("Awaiting DUT port down: %v", dp)
				gnmi.Await(t, tc.dut, dip.OperStatus().State(), time.Minute, opDown)
				t.Log("Port is down.")
			}
			if *deviations.InterfaceOperStatus && tf.want == oc.Interface_OperStatus_LOWER_LAYER_DOWN {
				tf.want = opDown
			}
			gnmi.Await(t, tc.dut, gnmi.OC().Interface(tc.aggID).OperStatus().State(), 1*time.Minute, tf.want)
		})
	}

	// Bring the ATE ports back up.
	for _, port := range tc.atePorts {
		tc.ate.Actions().NewSetPortState().WithPort(port).WithEnabled(true).Send(t)
	}
}

func TestNegotiation(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	aggID := netutil.NextBundleInterface(t, dut)

	lagTypes := []oc.E_IfAggregate_AggregationType{lagTypeLACP, lagTypeSTATIC}

	for _, lagType := range lagTypes {
		top := ate.Topology().New()
		tc := &testCase{
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
			t.Run("VerifyDUT", tc.verifyDUT)

			tc.configureATE(t)
			t.Run("VerifyATE", tc.verifyATE)

			t.Run("MinLinks", tc.verifyMinLinks)
		})
	}
}
