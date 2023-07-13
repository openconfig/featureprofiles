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

// Package telemetry_port_speed_test implements tests that cover port-speed related
// telemetry variables.
package telemetry_port_speed_test

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
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test cases:
//   - Validate that the port speed reported in telemetry is the expected port speed.
//   - Turn port down at ATE, validate that operational status of the port is reported as down.
//   - Connect N ports between ATE and DUT configured as part of a LACP bundle.
//     Validate that the effective speed of the LAG is reported as N*port speed.
//   - Disable each port at ATE and determine that the effective speed is reduced by the expected amount.
//   - Turn ports sequentially up at the ATE, and determine that the effective speed is increased as expected.
//
// Topology:
//
//	dut:port1 <--> ate:port1
//	dut:portN <--> ate:portN
const (
	plen4 = 30
	plen6 = 126
)

var (
	dutIPs = attrs.Attributes{
		Name:    "dutip",
		Desc:    "LAG To ATE",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	ateIPs = attrs.Attributes{
		Name:    "ateip",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
)

const (
	ethernetCsmacd = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	ieee8023adLag  = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	lagTypeLACP    = oc.IfAggregate_AggregationType_LACP
	lagTypeSTATIC  = oc.IfAggregate_AggregationType_STATIC
	minLink        = 1
)

type testCase struct {
	minlinks uint16
	lagType  oc.E_IfAggregate_AggregationType

	dut *ondatra.DUTDevice
	ate *ondatra.ATEDevice
	top *ondatra.ATETopology

	dutPorts []*ondatra.Port
	atePorts []*ondatra.Port
	aggID    string
}

func (tc *testCase) configDUT(i *oc.Interface, a *attrs.Attributes) {
	i.Description = ygot.String(a.Desc)
	if deviations.InterfaceEnabled(tc.dut) {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(tc.dut) && !deviations.IPv4MissingEnabled(tc.dut) {
		s4.Enabled = ygot.Bool(true)
	}
	s4.GetOrCreateAddress(a.IPv4).PrefixLength = ygot.Uint8((plen4))

	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(tc.dut) {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(plen6)
}

func (tc *testCase) configAggregateDUT(i *oc.Interface, a *attrs.Attributes) {
	tc.configDUT(i, a)
	i.Type = ieee8023adLag
	g := i.GetOrCreateAggregation()
	g.LagType = tc.lagType
	g.MinLinks = ygot.Uint16(tc.minlinks)
}

var portSpeed = map[ondatra.Speed]oc.E_IfEthernet_ETHERNET_SPEED{
	ondatra.Speed10Gb:  oc.IfEthernet_ETHERNET_SPEED_SPEED_10GB,
	ondatra.Speed100Gb: oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB,
	ondatra.Speed400Gb: oc.IfEthernet_ETHERNET_SPEED_SPEED_400GB,
}

func (tc *testCase) configMemberDUT(i *oc.Interface, p *ondatra.Port) {
	i.Description = ygot.String(p.String())
	i.Type = ethernetCsmacd
	if deviations.InterfaceEnabled(tc.dut) {
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

	for _, port := range tc.dutPorts {
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(tc.aggID)
		i.Type = ethernetCsmacd
	}

	fptest.LogQuery(t, fmt.Sprintf("%s to Update()", tc.dut), gnmi.OC().Config(), d)
	gnmi.Update(t, tc.dut, gnmi.OC().Config(), d)
}

func (tc *testCase) clearAggregateMembers(t *testing.T) {
	for _, port := range tc.dutPorts {
		gnmi.Delete(t, tc.dut, gnmi.OC().Interface(port.Name()).Ethernet().AggregateId().Config())
	}
}

// sortPorts sorts the ports by the testbed port ID.
func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})
	return ports
}

func (tc *testCase) configureDUT(t *testing.T) {
	t.Logf("dut ports = %v", tc.dutPorts)
	if len(tc.dutPorts) < 2 {
		t.Fatalf("Testbed requires at least 2 ports, got %d", len(tc.dutPorts))
	}

	d := gnmi.OC()

	if deviations.AggregateAtomicUpdate(tc.dut) {
		tc.clearAggregateMembers(t)
		tc.setupAggregateAtomically(t)
	}

	for _, port := range tc.dutPorts {
		iName := port.Name()
		i := &oc.Interface{Name: ygot.String(iName)}
		tc.configMemberDUT(i, port)
		iPath := d.Interface(iName)
		fptest.LogQuery(t, port.String(), iPath.Config(), i)
		gnmi.Replace(t, tc.dut, iPath.Config(), i)
		if deviations.ExplicitPortSpeed(tc.dut) {
			fptest.SetPortSpeed(t, port)
		}
	}

	if tc.lagType == lagTypeLACP {
		lacp := &oc.Lacp_Interface{Name: ygot.String(tc.aggID)}
		lacp.LacpMode = oc.Lacp_LacpActivityType_ACTIVE

		lacpPath := d.Lacp().Interface(tc.aggID)
		fptest.LogQuery(t, "LACP", lacpPath.Config(), lacp)
		gnmi.Replace(t, tc.dut, lacpPath.Config(), lacp)
		t.Cleanup(func() {
			gnmi.Delete(t, tc.dut, lacpPath.Config())
		})
	}

	agg := &oc.Interface{Name: ygot.String(tc.aggID)}
	tc.configAggregateDUT(agg, &dutIPs)
	aggPath := d.Interface(tc.aggID)
	fptest.LogQuery(t, tc.aggID, aggPath.Config(), agg)
	gnmi.Replace(t, tc.dut, aggPath.Config(), agg)
	if deviations.ExplicitInterfaceInDefaultVRF(tc.dut) {
		fptest.AssignToNetworkInstance(t, tc.dut, tc.aggID, deviations.DefaultNetworkInstance(tc.dut), 0)
	}
	t.Cleanup(func() {
		gnmi.Delete(t, tc.dut, gnmi.OC().Interface(tc.aggID).Aggregation().MinLinks().Config())
		for _, port := range tc.dutPorts {
			iName := port.Name()
			iPath := d.Interface(iName)
			gnmi.Replace(t, tc.dut, iPath.Config(), &oc.Interface{Name: ygot.String(iName), Type: ethernetCsmacd})
		}
		if deviations.AggregateAtomicUpdate(tc.dut) {
			resetBatch := &gnmi.SetBatch{}
			if deviations.ExplicitInterfaceInDefaultVRF(tc.dut) {
				gnmi.BatchDelete(resetBatch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(tc.dut)).Interface(tc.aggID+".0").Config())
			}
			gnmi.BatchDelete(resetBatch, aggPath.Config())
			gnmi.BatchDelete(resetBatch, d.Lacp().Interface(tc.aggID).Config())
			resetBatch.Set(t, tc.dut)
		}
		gnmi.Delete(t, tc.dut, aggPath.Config())
	})
}

func (tc *testCase) configureATE(t *testing.T) {
	if len(tc.atePorts) < 2 {
		t.Fatalf("Testbed requires at least 2 ports, got: %v", tc.atePorts)
	}

	// Don't use WithLACPEnabled which is for emulated Ixia LACP.
	agg := tc.top.AddInterface(ateIPs.Name)
	lag := tc.top.AddLAG("lag").WithPorts(tc.atePorts...)
	lag.LACP().WithEnabled(tc.lagType == lagTypeLACP)
	agg.WithLAG(lag)

	// Disable FEC for 100G-FR ports because Novus does not support it.
	is100gfr := false
	for _, p := range tc.atePorts {
		if p.PMD() == ondatra.PMD100GBASEFR {
			is100gfr = true
		}
	}
	if is100gfr {
		agg.Ethernet().FEC().WithEnabled(false)
	}

	agg.IPv4().
		WithAddress(ateIPs.IPv4CIDR()).
		WithDefaultGateway(dutIPs.IPv4)
	agg.IPv6().
		WithAddress(ateIPs.IPv6CIDR()).
		WithDefaultGateway(dutIPs.IPv6)

	tc.top.Push(t).StartProtocols(t)
}

func (tc *testCase) verifyDUT(t *testing.T, numPort int) {
	dutPort := tc.dut.Port(t, "port1")
	want := int(dutPort.Speed()) * numPort * 1000
	val, _ := gnmi.Watch(t, tc.dut, gnmi.OC().Interface(tc.aggID).Aggregation().LagSpeed().State(), 60*time.Second, func(val *ygnmi.Value[uint32]) bool { return val.IsPresent() }).Await(t)
	if got, _ := val.Val(); int(got) != want {
		t.Errorf("Get(DUT port status): got %v, want %v", got, want)
	}
}

func TestGNMIPortSpeed(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dutPort := dut.Port(t, "port1")
	if got, want := gnmi.Get(t, dut, gnmi.OC().Interface(dutPort.Name()).Ethernet().PortSpeed().State()), portSpeed[dutPort.Speed()]; got != want {
		t.Errorf("Get(DUT port1 status): got %v, want %v", got, want)
	}
}

func TestGNMIPortDown(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	dutPort := dut.Port(t, "port1")
	atePort := ate.Port(t, "port1")
	top := ate.Topology().New()
	intf := top.AddInterface(ateIPs.Name).WithPort(atePort)
	intf.IPv4().
		WithAddress(ateIPs.IPv4CIDR()).
		WithDefaultGateway(dutIPs.IPv4)
	intf.IPv6().
		WithAddress(ateIPs.IPv6CIDR()).
		WithDefaultGateway(dutIPs.IPv6)
	top.Push(t)
	ate.Actions().NewSetPortState().WithPort(atePort).WithEnabled(false).Send(t)
	dutPortStatus := gnmi.Get(t, dut, gnmi.OC().Interface(dutPort.Name()).OperStatus().State())

	if want := oc.Interface_OperStatus_DOWN; dutPortStatus != want {
		t.Errorf("Get(DUT port1 status): got %v, want %v", dutPortStatus, want)
	}
	ate.Actions().NewSetPortState().WithPort(atePort).WithEnabled(true).Send(t)
}

func TestGNMICombinedLACPSpeed(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	for _, lagType := range []oc.E_IfAggregate_AggregationType{lagTypeLACP, lagTypeSTATIC} {
		t.Run(lagType.String(), func(t *testing.T) {
			top := ate.Topology().New()
			tc := &testCase{
				minlinks: minLink,
				lagType:  lagType,

				dut: dut,
				ate: ate,
				top: top,

				dutPorts: sortPorts(dut.Ports()),
				atePorts: sortPorts(ate.Ports()),
				aggID:    netutil.NextAggregateInterface(t, dut),
			}
			tc.configureDUT(t)
			tc.configureATE(t)
			tc.verifyDUT(t, len(tc.dutPorts))
		})
	}
}

func TestGNMIReducedLACPSpeed(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	totalPort := len(ate.Ports())

	for _, lagType := range []oc.E_IfAggregate_AggregationType{lagTypeLACP, lagTypeSTATIC} {
		t.Run(lagType.String(), func(t *testing.T) {
			top := ate.Topology().New()
			tc := &testCase{
				minlinks: minLink,
				lagType:  lagType,
				dut:      dut,
				ate:      ate,
				top:      top,

				dutPorts: sortPorts(dut.Ports()),
				atePorts: sortPorts(ate.Ports()),
				aggID:    netutil.NextAggregateInterface(t, dut),
			}
			tc.configureDUT(t)
			tc.configureATE(t)
			for _, port := range tc.atePorts {
				totalPort--
				if totalPort < 1 {
					break
				}
				ate.Actions().NewSetPortState().WithPort(port).WithEnabled(false).Send(t)
				time.Sleep(10 * time.Second)
				tc.verifyDUT(t, totalPort)
			}
			for _, port := range tc.atePorts {
				totalPort++
				if totalPort > len(tc.atePorts)-1 {
					break
				}
				ate.Actions().NewSetPortState().WithPort(port).WithEnabled(true).Send(t)
				time.Sleep(10 * time.Second)
				tc.verifyDUT(t, totalPort+1)
			}
		})
	}
}
