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

package isis_drain_test

import (
	"fmt"
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

const (
	plen4        = 30
	plen6        = 126
	isisInstance = "DEFAULT"
	areaAddress  = "49.0001"
	sysID        = "1920.0000.2001"
	v4Route      = "203.0.113.0/24"
	v4IP         = "203.0.113.1"
	v4IPmax      = "203.0.113.255"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT port 1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	atePort1 = attrs.Attributes{
		Name:    "ATE port 1",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "DUT port 2",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	atePort2 = attrs.Attributes{
		Name:    "ATE port 2",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	dutPort3 = attrs.Attributes{
		Desc:    "DUT port 3",
		IPv4:    "192.0.2.9",
		IPv6:    "2001:db8::9",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	atePort3 = attrs.Attributes{
		Name:    "ATE port 3",
		IPv4:    "192.0.2.10",
		IPv6:    "2001:db8::a",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configSrcDUT(dut *ondatra.DUTDevice, i *oc.Interface, a *attrs.Attributes) {
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
	a4 := s4.GetOrCreateAddress(a.IPv4)
	a4.PrefixLength = ygot.Uint8(plen4)

	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(plen6)
}

func configDstAggregateDUT(dut *ondatra.DUTDevice, i *oc.Interface, a *attrs.Attributes) {
	i.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	g := i.GetOrCreateAggregation()
	g.LagType = oc.IfAggregate_AggregationType_STATIC

	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	a4 := s4.GetOrCreateAddress(a.IPv4)
	a4.PrefixLength = ygot.Uint8(plen4)

	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(plen6)
}

func configDstMemberDUT(dut *ondatra.DUTDevice, i *oc.Interface, p *ondatra.Port, aggID string) {
	i.Description = ygot.String(p.String())
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	e := i.GetOrCreateEthernet()
	e.AggregateId = ygot.String(aggID)
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) (agg2ID, agg3ID string) {
	// configure port 1
	p1 := dut.Port(t, "port1")
	p1Name := p1.Name()

	i1 := &oc.Interface{Name: ygot.String(p1Name)}
	configSrcDUT(dut, i1, &dutPort1)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1Name).Config(), i1)

	// configure port 2 / trunk 2
	p2 := dut.Port(t, "port2")
	p2Name := p2.Name()

	agg2ID = netutil.NextAggregateInterface(t, dut)
	agg2 := &oc.Interface{Name: ygot.String(agg2ID)}
	configDstAggregateDUT(dut, agg2, &dutPort2)
	gnmi.Replace(t, dut, gnmi.OC().Interface(agg2ID).Config(), agg2)

	i2 := &oc.Interface{Name: ygot.String(p2Name)}
	configDstMemberDUT(dut, i2, p2, agg2ID)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2Name).Config(), i2)

	// configure port 3 / trunk 3
	p3 := dut.Port(t, "port3")
	p3Name := p3.Name()

	agg3ID = netutil.NextAggregateInterface(t, dut)
	agg3 := &oc.Interface{Name: ygot.String(agg3ID)}
	configDstAggregateDUT(dut, agg3, &dutPort3)
	gnmi.Replace(t, dut, gnmi.OC().Interface(agg3ID).Config(), agg3)

	i3 := &oc.Interface{Name: ygot.String(p3Name)}
	configDstMemberDUT(dut, i3, p3, agg3ID)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p3Name).Config(), i3)

	// handle deviations for ports and lags
	if deviations.ExplicitPortSpeed(dut) {
		for _, port := range dut.Ports() {
			fptest.SetPortSpeed(t, port)
		}
	}

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, agg2ID, deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, agg3ID, deviations.DefaultNetworkInstance(dut), 0)
	}

	// configure ISIS
	configureISISDUT(t, dut, []string{agg2ID, agg3ID})
	return agg2ID, agg3ID
}

func configureISISDUT(t *testing.T, dut *ondatra.DUTDevice, intfs []string) {
	d := &oc.Root{}
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	prot.Enabled = ygot.Bool(true)
	isis := prot.GetOrCreateIsis()

	globalISIS := isis.GetOrCreateGlobal()
	if deviations.ISISInstanceEnabledRequired(dut) {
		globalISIS.Instance = ygot.String(isisInstance)
	}
	globalISIS.LevelCapability = oc.Isis_LevelType_LEVEL_2
	globalISIS.Net = []string{fmt.Sprintf("%v.%v.00", areaAddress, sysID)}
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalISIS.SetMaxEcmpPaths(2)
	lspBit := globalISIS.GetOrCreateLspBit().GetOrCreateOverloadBit()
	lspBit.SetBit = ygot.Bool(false)

	isisLevel2 := isis.GetOrCreateLevel(2)
	isisLevel2.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC

	for _, intfName := range intfs {
		isisIntf := isis.GetOrCreateInterface(intfName)
		isisIntf.GetOrCreateInterfaceRef().Interface = ygot.String(intfName)
		isisIntf.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
		if deviations.InterfaceRefConfigUnsupported(dut) {
			isisIntf.InterfaceRef = nil
		}
		isisIntf.Enabled = ygot.Bool(true)
		isisIntf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
		isisIntf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
		isisIntf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
		if deviations.ISISInterfaceAfiUnsupported(dut) {
			isisIntf.Af = nil
		}

		isisIntfLevel := isisIntf.GetOrCreateLevel(2)
		isisIntfLevel.Enabled = ygot.Bool(true)

		isisIntfLevelAfiv4 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
		isisIntfLevelAfiv4.Metric = ygot.Uint32(10)
		isisIntfLevelAfiv4.Enabled = ygot.Bool(true)
		isisIntfLevelAfiv6 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)
		isisIntfLevelAfiv6.Metric = ygot.Uint32(10)
		isisIntfLevelAfiv6.Enabled = ygot.Bool(true)
		if deviations.MissingIsisInterfaceAfiSafiEnable(dut) {
			isisIntfLevelAfiv4.Enabled = nil
			isisIntfLevelAfiv6.Enabled = nil
		}
	}
	gnmi.Update(t, dut, gnmi.OC().Config(), d)
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	top := ate.Topology().New()
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	p3 := ate.Port(t, "port3")

	i1 := top.AddInterface(atePort1.Name).WithPort(p1)
	i1.IPv4().WithAddress(atePort1.IPv4CIDR()).WithDefaultGateway(dutPort1.IPv4)
	i1.IPv6().WithAddress(atePort1.IPv6CIDR()).WithDefaultGateway(dutPort1.IPv6)

	agg2 := top.AddInterface(atePort2.Name)
	lag2 := top.AddLAG("lag2").WithPorts(p2)
	lag2.LACP().WithEnabled(false)
	agg2.WithLAG(lag2)
	agg2.IPv4().WithAddress(atePort2.IPv4CIDR()).WithDefaultGateway(dutPort2.IPv4)
	agg2.IPv6().WithAddress(atePort2.IPv6CIDR()).WithDefaultGateway(dutPort2.IPv6)

	agg3 := top.AddInterface(atePort3.Name)
	lag3 := top.AddLAG("lag3").WithPorts(p3)
	lag3.LACP().WithEnabled(false)
	agg3.WithLAG(lag3)
	agg3.IPv4().WithAddress(atePort3.IPv4CIDR()).WithDefaultGateway(dutPort3.IPv4)
	agg3.IPv6().WithAddress(atePort3.IPv6CIDR()).WithDefaultGateway(dutPort3.IPv6)

	isisAgg2 := agg2.ISIS()
	isisAgg2.WithAreaID(areaAddress).WithNetworkTypePointToPoint().WithLevelL2().WithWideMetricEnabled(true).WithMetric(10)

	isisAgg3 := agg3.ISIS()
	isisAgg3.WithAreaID(areaAddress).WithNetworkTypePointToPoint().WithLevelL2().WithWideMetricEnabled(true).WithMetric(10)

	netGrp2 := agg2.AddNetwork("isisNet")
	netGrp2.IPv4().WithAddress(v4Route).WithCount(1)
	netGrp2.ISIS().WithActive(true)

	netGrp3 := agg3.AddNetwork("isisNet")
	netGrp3.IPv4().WithAddress(v4Route).WithCount(1)
	netGrp3.ISIS().WithActive(true)

	t.Log("Pushing config to ATE and starting protocols...")
	top.Push(t).StartProtocols(t)
	return top
}

func changeMetric(t *testing.T, dut *ondatra.DUTDevice, intf string, metric uint32) {
	t.Logf("Changing metric to %v on interface %v", metric, intf)
	d := &oc.Root{}
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	isis := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).GetOrCreateIsis()
	isisIntfLevel := isis.GetOrCreateInterface(intf).GetOrCreateLevel(2)
	isisIntfLevelAfiv4 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
	isisIntfLevelAfiv4.Metric = ygot.Uint32(metric)
	isisIntfLevelAfiv6 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)
	isisIntfLevelAfiv6.Metric = ygot.Uint32(metric)
	gnmi.Update(t, dut, gnmi.OC().Config(), d)
}

// createFlow returns an ONDATRA traffic flow
func createFlow(t *testing.T, ate *ondatra.ATEDevice, ateTopo *ondatra.ATETopology, name string, dstPorts ...string) []*ondatra.Flow {
	srcIntf := ateTopo.Interfaces()[atePort1.Name]
	var dsts []ondatra.Endpoint
	for _, dst := range dstPorts {
		dsts = append(dsts, ateTopo.Interfaces()[dst])
	}

	t.Log("Configuring v4 traffic flow ")
	v4Header := ondatra.NewIPv4Header()
	tcpHeader := ondatra.NewTCPHeader()
	tcpHeader.DstPortRange().WithCount(200)

	v4Header.DstAddressRange().WithMin(v4IP).WithMax(v4IPmax).WithCount(50)
	v4Flow := ate.Traffic().NewFlow(name).WithDstEndpoints(dsts...).
		WithSrcEndpoints(srcIntf).WithHeaders(ondatra.NewEthernetHeader(), v4Header, tcpHeader).
		WithFrameSize(300).WithIngressTrackingByPorts(true)

	return []*ondatra.Flow{v4Flow}
}

func validateTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, good []*ondatra.Flow, bad []*ondatra.Flow) {
	if len(good) == 0 && len(bad) == 0 {
		return
	}
	t.Logf("Validating traffic flows")
	flows := append(good, bad...)
	ate.Traffic().Start(t, flows...)
	time.Sleep(15 * time.Second)
	ate.Traffic().Stop(t)

	for _, flow := range good {
		if got := gnmi.Get(t, ate, gnmi.OC().Flow(flow.Name()).LossPct().State()); got > 0 {
			t.Fatalf("LossPct for flow %s: got %g, want 0", flow.Name(), got)
		}
	}

	for _, flow := range bad {
		if got := gnmi.Get(t, ate, gnmi.OC().Flow(flow.Name()).LossPct().State()); got < 100 {
			t.Fatalf("LossPct for flow %s: got %g, want 100", flow.Name(), got)
		}
	}
}

func TestDrain(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	agg2ID, _ := configureDUT(t, dut)
	ateTopo := configureATE(t, ate)

	ecmpFlows := createFlow(t, ate, ateTopo, "ecmp-flow", atePort2.Name, atePort3.Name)
	lag2Flow := createFlow(t, ate, ateTopo, "trunk2-flow", atePort2.Name)
	lag3Flow := createFlow(t, ate, ateTopo, "trunk3-flow", atePort3.Name)

	t.Logf("Validating baseline traffic flow")
	validateTrafficFlows(t, ate, ecmpFlows, nil)

	// Change trunk-2 metric to 1000 and validate the traffic flows
	changeMetric(t, dut, agg2ID, 1000)
	t.Logf("Validating traffic flows after increasing the metric")
	validateTrafficFlows(t, ate, lag3Flow, lag2Flow)

	// Restore trunk-2 metric
	changeMetric(t, dut, agg2ID, 10)
	t.Logf("Validating traffic flows after restoring the metric")
	validateTrafficFlows(t, ate, ecmpFlows, nil)
}
