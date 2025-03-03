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

package lacp_interval_test

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
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

// The testbed consists of dut1:port{1-2} -> dut2:port{1-2}
//
//   - Source: dut1:port{1-2} -> dut2:port{1-2} subnet 192.0.2.0/30 2001:db8::0/126

// Note that the first (.0) and last (.3) IPv4 addresses are
// reserved from the subnet for broadcast, so a /30 leaves exactly 2
// usable addresses.  This does not apply to IPv6 which allows /127
// for point to point links, but we use /126 so the numbering is
// consistent with IPv4.

const (
	plen4          = 30
	plen6          = 126
	opUp           = oc.Interface_OperStatus_UP
	adminUp        = oc.Interface_AdminStatus_UP
	ethernetCsmacd = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	ieee8023adLag  = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	lagTypeLACP    = oc.IfAggregate_AggregationType_LACP
)

var (
	dut1Src = attrs.Attributes{
		Desc:    "dutsrc",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	dut2Src = attrs.Attributes{
		Name:    "atesrc",
		MAC:     "02:11:01:00:00:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
)

type testCase struct {
	lagType      oc.E_IfAggregate_AggregationType
	dut1         *ondatra.DUTDevice
	dut2         *ondatra.DUTDevice
	top          gosnappi.Config
	desc         string
	lacpInterval oc.E_Lacp_LacpPeriodType

	// dutPorts is the set of ports the DUT -- the first (i.e., dutPorts[0])
	// is not configured in the aggregate interface.
	dut1Ports []*ondatra.Port
	// atePorts is the set of ports on the ATE -- the first, as with the DUT
	// is not configured in the aggregate interface.
	// is not configured in the aggregate interface.
	dut2Ports []*ondatra.Port
	aggID     string
}

func (tc *testCase) clearAggregate(t *testing.T) {
	// Clear the aggregate minlink.
	gnmi.Delete(t, tc.dut1, gnmi.OC().Interface(tc.aggID).Aggregation().MinLinks().Config())
	gnmi.Delete(t, tc.dut2, gnmi.OC().Interface(tc.aggID).Aggregation().MinLinks().Config())

	// Clear the members of the aggregate.
	for _, port := range tc.dut1Ports {
		gnmi.Delete(t, tc.dut1, gnmi.OC().Interface(port.Name()).Ethernet().AggregateId().Config())
	}
	for _, port := range tc.dut2Ports {
		gnmi.Delete(t, tc.dut2, gnmi.OC().Interface(port.Name()).Ethernet().AggregateId().Config())
	}
}

func (tc *testCase) setupAggregateAtomically(t *testing.T) {
	d := &oc.Root{}

	if tc.lagType == lagTypeLACP {
		d.GetOrCreateLacp().GetOrCreateInterface(tc.aggID)
	}

	agg := d.GetOrCreateInterface(tc.aggID)
	agg.GetOrCreateAggregation().LagType = tc.lagType
	agg.Type = ieee8023adLag

	for _, port := range tc.dut1Ports {
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(tc.aggID)
		i.Type = ethernetCsmacd

		if deviations.InterfaceEnabled(tc.dut1) {
			i.Enabled = ygot.Bool(true)
		}
	}
	for _, port := range tc.dut2Ports {
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(tc.aggID)
		i.Type = ethernetCsmacd

		if deviations.InterfaceEnabled(tc.dut2) {
			i.Enabled = ygot.Bool(true)
		}
	}

	p := gnmi.OC()
	fptest.LogQuery(t, fmt.Sprintf("%s to Update()", tc.dut1), p.Config(), d)
	fptest.LogQuery(t, fmt.Sprintf("%s to Update()", tc.dut2), p.Config(), d)
	gnmi.Update(t, tc.dut1, p.Config(), d)
	gnmi.Update(t, tc.dut2, p.Config(), d)
}

// sortPorts sorts the ports by the testbed port ID.
func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})
	return ports
}

func (tc *testCase) verifyAggID1(t *testing.T, dp *ondatra.Port) {
	dip := gnmi.OC().Interface(dp.Name())
	di1 := gnmi.Get(t, tc.dut1, dip.State())
	if lagID1 := di1.GetEthernet().GetAggregateId(); lagID1 != tc.aggID {
		t.Errorf("%s LagID got %v, want %v", dp, lagID1, tc.aggID)
	}
}

func (tc *testCase) verifyAggID2(t *testing.T, dp *ondatra.Port) {
	dip := gnmi.OC().Interface(dp.Name())
	di2 := gnmi.Get(t, tc.dut2, dip.State())
	if lagID2 := di2.GetEthernet().GetAggregateId(); lagID2 != tc.aggID {
		t.Errorf("%s LagID got %v, want %v", dp, lagID2, tc.aggID)
	}
}

func (tc *testCase) configDstMemberDUT(i *oc.Interface, p *ondatra.Port) {
	i.Description = ygot.String(p.String())
	i.Type = ethernetCsmacd

	if deviations.InterfaceEnabled(tc.dut1) {
		i.Enabled = ygot.Bool(true)
	}

	e := i.GetOrCreateEthernet()
	e.AggregateId = ygot.String(tc.aggID)
}

func (tc *testCase) configDstAggregateDUT(i *oc.Interface, a *attrs.Attributes) {
	tc.configSrcDUT(i, a)
	i.Type = ieee8023adLag
	g := i.GetOrCreateAggregation()
	g.LagType = tc.lagType
}

func (tc *testCase) configSrcDUT(i *oc.Interface, a *attrs.Attributes) {
	i.Description = ygot.String(a.Desc)
	if deviations.InterfaceEnabled(tc.dut1) {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(tc.dut1) && !deviations.IPv4MissingEnabled(tc.dut1) {
		s4.Enabled = ygot.Bool(true)
	}
	a4 := s4.GetOrCreateAddress(a.IPv4)
	a4.PrefixLength = ygot.Uint8(plen4)

	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(tc.dut1) {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(plen6)
}

func (tc *testCase) configureDUT(t *testing.T) {
	t.Logf("dut1 ports = %v, dut2 ports = %v", tc.dut1Ports, tc.dut2Ports)
	if len(tc.dut1Ports) < 2 || len(tc.dut2Ports) < 2 {
		t.Fatalf("Testbed requires at least 2 ports, got %d and %d", len(tc.dut1Ports), len(tc.dut2Ports))
	}

	d := gnmi.OC()

	if deviations.AggregateAtomicUpdate(tc.dut1) {
		tc.clearAggregate(t)
		tc.setupAggregateAtomically(t)
	}

	lacp := &oc.Lacp_Interface{Name: ygot.String(tc.aggID)}
	lacp.LacpMode = oc.Lacp_LacpActivityType_ACTIVE
	lacp.Interval = tc.lacpInterval
	lacpPath := d.Lacp().Interface(tc.aggID)
	fptest.LogQuery(t, "LACP", lacpPath.Config(), lacp)
	gnmi.Replace(t, tc.dut1, lacpPath.Config(), lacp)
	gnmi.Replace(t, tc.dut2, lacpPath.Config(), lacp)

	// TODO - to remove this sleep later
	time.Sleep(5 * time.Second)

	agg := &oc.Interface{Name: ygot.String(tc.aggID)}
	aggPath := d.Interface(tc.aggID)
	tc.configDstAggregateDUT(agg, &dut1Src)
	tc.configDstAggregateDUT(agg, &dut2Src)
	fptest.LogQuery(t, tc.aggID, aggPath.Config(), agg)
	gnmi.Replace(t, tc.dut1, aggPath.Config(), agg)
	gnmi.Replace(t, tc.dut2, aggPath.Config(), agg)

	if deviations.ExplicitInterfaceInDefaultVRF(tc.dut1) {
		fptest.AssignToNetworkInstance(t, tc.dut1, tc.aggID, deviations.DefaultNetworkInstance(tc.dut1), 0)
		fptest.AssignToNetworkInstance(t, tc.dut1, dut1Src.Name, deviations.DefaultNetworkInstance(tc.dut1), 0)
	}
	for _, port := range tc.dut1Ports {
		i := &oc.Interface{Name: ygot.String(port.Name())}
		i.Type = ethernetCsmacd

		if deviations.InterfaceEnabled(tc.dut1) {
			i.Enabled = ygot.Bool(true)
		}
		tc.configDstMemberDUT(i, port)
		iPath := d.Interface(port.Name())
		fptest.LogQuery(t, port.String(), iPath.Config(), i)
		gnmi.Replace(t, tc.dut1, iPath.Config(), i)
	}
	if deviations.ExplicitPortSpeed(tc.dut1) {
		for _, port := range tc.dut1Ports {
			fptest.SetPortSpeed(t, port)
		}
	}
	for _, port := range tc.dut2Ports {
		i := &oc.Interface{Name: ygot.String(port.Name())}
		i.Type = ethernetCsmacd

		if deviations.InterfaceEnabled(tc.dut2) {
			i.Enabled = ygot.Bool(true)
		}
		tc.configDstMemberDUT(i, port)
		iPath := d.Interface(port.Name())
		fptest.LogQuery(t, port.String(), iPath.Config(), i)
		gnmi.Replace(t, tc.dut2, iPath.Config(), i)
	}
	if deviations.ExplicitPortSpeed(tc.dut2) {
		for _, port := range tc.dut2Ports {
			fptest.SetPortSpeed(t, port)
		}
	}
}

func (tc *testCase) verifyDUT(t *testing.T) {
	// Wait for LAG negotiation and verify LAG type for the aggregate interface.
	gnmi.Await(t, tc.dut1, gnmi.OC().Interface(tc.aggID).Type().State(), time.Minute, ieee8023adLag)

	for _, port := range tc.dut1Ports {
		t.Run(fmt.Sprintf("%s [member]", port.ID()), func(t *testing.T) {
			tc.verifyInterfaceDUT1(t, port)
			tc.verifyAggID1(t, port)
		})
	}
	for _, port := range tc.dut2Ports {
		t.Run(fmt.Sprintf("%s [member]", port.ID()), func(t *testing.T) {
			tc.verifyInterfaceDUT2(t, port)
			tc.verifyAggID2(t, port)
		})
	}
	LACPIntervals := gnmi.OC().Lacp().Interface(tc.aggID).Interval()
	LACPIntervalDUT1 := gnmi.Get(t, tc.dut1, LACPIntervals.State())
	LACPIntervalDUT2 := gnmi.Get(t, tc.dut2, LACPIntervals.State())
	if LACPIntervalDUT1 != LACPIntervalDUT2 {
		t.Errorf("LACP Interval is not same on both the DUTs, DUT1: %v, DUT2: %v", LACPIntervalDUT1, LACPIntervalDUT2)
	}
	if LACPIntervalDUT1 != tc.lacpInterval {
		t.Errorf("LACP Interval is not same as configured, got: %v, want: %v", LACPIntervalDUT1, tc.lacpInterval)
	}
	t.Logf("LACP Interval is same as configured, got: %v, want: %v", LACPIntervalDUT1, tc.lacpInterval)
}

func (tc *testCase) verifyInterfaceDUT1(t *testing.T, dp *ondatra.Port) {
	dip := gnmi.OC().Interface(dp.Name())
	di1 := gnmi.Get(t, tc.dut1, dip.State())
	fptest.LogQuery(t, dp.String()+" before Await", dip.State(), di1)

	if got := di1.GetAdminStatus(); got != adminUp {
		t.Errorf("%s admin-status got %v, want %v", dp, got, adminUp)
	}

	// LAG members may fall behind, so wait for them to be up.
	gnmi.Await(t, tc.dut1, dip.OperStatus().State(), time.Minute, opUp)
}

func (tc *testCase) verifyInterfaceDUT2(t *testing.T, dp *ondatra.Port) {
	dip := gnmi.OC().Interface(dp.Name())
	di2 := gnmi.Get(t, tc.dut2, dip.State())
	fptest.LogQuery(t, dp.String()+" before Await", dip.State(), di2)

	if got := di2.GetAdminStatus(); got != adminUp {
		t.Errorf("%s admin-status got %v, want %v", dp, got, adminUp)
	}

	// LAG members may fall behind, so wait for them to be up.
	gnmi.Await(t, tc.dut2, dip.OperStatus().State(), time.Minute, opUp)
}

func TestLacpTimers(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	aggID := netutil.NextAggregateInterface(t, dut1)

	lacpIntervals := []oc.E_Lacp_LacpPeriodType{oc.Lacp_LacpPeriodType_FAST, oc.Lacp_LacpPeriodType_SLOW}

	for _, lacpInterval := range lacpIntervals {
		tc := &testCase{
			dut1:         dut1,
			dut2:         dut2,
			lagType:      lagTypeLACP,
			lacpInterval: lacpInterval,
			dut1Ports:    sortPorts(dut1.Ports()),
			dut2Ports:    sortPorts(dut2.Ports()),
			aggID:        aggID,
		}
		t.Run(fmt.Sprintf("LACPInterval=%s", lacpInterval), func(t *testing.T) {
			tc.configureDUT(t)
			tc.verifyDUT(t)

		})
	}
}

