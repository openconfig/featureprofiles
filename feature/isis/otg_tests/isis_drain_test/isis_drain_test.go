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
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	plen4              = 30
	plen6              = 126
	isisInstance       = "DEFAULT"
	areaAddress        = "49.0001"
	sysID              = "1920.0000.2001"
	v4Route            = "203.0.113.0"
	v4RoutePlen        = 24
	v4IP               = "203.0.113.1"
	lag2MAC            = "02:aa:bb:02:00:02"
	lag3MAC            = "02:aa:bb:03:00:02"
	otgLAG2sysID       = "640000000002"
	otgLAG3sysID       = "640000000003"
	maxEcmpPaths uint8 = 16
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
		Name:    "ATEport1",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: plen4,
		IPv6Len: plen6,
		MAC:     "02:aa:bb:01:00:01",
	}

	dutPort2 = attrs.Attributes{
		Desc:    "DUT port 2",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	atePort2 = attrs.Attributes{
		Name:    "ATEport2",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: plen4,
		IPv6Len: plen6,
		MAC:     "02:aa:bb:02:00:01",
	}

	dutPort3 = attrs.Attributes{
		Desc:    "DUT port 3",
		IPv4:    "192.0.2.9",
		IPv6:    "2001:db8::9",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	atePort3 = attrs.Attributes{
		Name:    "ATEport3",
		IPv4:    "192.0.2.10",
		IPv6:    "2001:db8::a",
		IPv4Len: plen4,
		IPv6Len: plen6,
		MAC:     "02:aa:bb:03:00:01",
	}
	agg2ID, agg3ID string
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

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	// configure port 1
	p1 := dut.Port(t, "port1")

	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	configSrcDUT(dut, i1, &dutPort1)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), i1)

	// configure port 2 / trunk 2
	p2 := dut.Port(t, "port2")
	agg2ID = netutil.NextAggregateInterface(t, dut)
	agg2 := &oc.Interface{Name: ygot.String(agg2ID)}
	configDstAggregateDUT(dut, agg2, &dutPort2)
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	configDstMemberDUT(dut, i2, p2, agg2ID)
	t.Logf("Adding port: %s to Aggregate: %s", p2.Name(), agg2ID)
	switch {
	case deviations.AggregateAtomicUpdate(dut):
		b := &gnmi.SetBatch{}
		gnmi.BatchDelete(b, gnmi.OC().Interface(agg2ID).Aggregation().MinLinks().Config())
		gnmi.BatchDelete(b, gnmi.OC().Interface(p2.Name()).Ethernet().AggregateId().Config())
		gnmi.BatchReplace(b, gnmi.OC().Interface(agg2ID).Config(), agg2)
		gnmi.BatchReplace(b, gnmi.OC().Interface(p2.Name()).Config(), i2)
		b.Set(t, dut)
	default:
		gnmi.Replace(t, dut, gnmi.OC().Interface(agg2ID).Config(), agg2)
		gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Config(), i2)
	}

	// configure port 3 / trunk 3
	p3 := dut.Port(t, "port3")
	agg3ID = netutil.NextAggregateInterface(t, dut)
	agg3 := &oc.Interface{Name: ygot.String(agg3ID)}
	configDstAggregateDUT(dut, agg3, &dutPort3)
	i3 := &oc.Interface{Name: ygot.String(p3.Name())}
	configDstMemberDUT(dut, i3, p3, agg3ID)
	t.Logf("Adding port: %s to Aggregate: %s", p3.Name(), agg3ID)

	switch {
	case deviations.AggregateAtomicUpdate(dut):
		b := &gnmi.SetBatch{}
		gnmi.BatchDelete(b, gnmi.OC().Interface(agg3ID).Aggregation().MinLinks().Config())
		gnmi.BatchDelete(b, gnmi.OC().Interface(p3.Name()).Ethernet().AggregateId().Config())
		gnmi.BatchReplace(b, gnmi.OC().Interface(agg3ID).Config(), agg3)
		gnmi.BatchReplace(b, gnmi.OC().Interface(p3.Name()).Config(), i3)
		b.Set(t, dut)
	default:
		gnmi.Replace(t, dut, gnmi.OC().Interface(agg3ID).Config(), agg3)
		gnmi.Replace(t, dut, gnmi.OC().Interface(p3.Name()).Config(), i3)
	}

	// handle deviations for ports and lags
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, agg2ID, deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, agg3ID, deviations.DefaultNetworkInstance(dut), 0)
	}

	if deviations.ExplicitPortSpeed(dut) {
		for _, port := range dut.Ports() {
			fptest.SetPortSpeed(t, port)
		}
	}

	// configure ISIS
	configureISISDUT(t, dut, []string{agg2ID, agg3ID})
}

func configureISISDUT(t *testing.T, dut *ondatra.DUTDevice, intfs []string) {
	d := &oc.Root{}
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	prot.Enabled = ygot.Bool(true)
	isis := prot.GetOrCreateIsis()

	globalISIS := isis.GetOrCreateGlobal()
	if !deviations.IsisMplsUnsupported(dut) {
		// Explicit Disable the default igp-ldp-sync enabled global leaf
		isismpls := prot.GetOrCreateIsis().GetOrCreateGlobal().GetOrCreateMpls()
		isismplsldpsync := isismpls.GetOrCreateIgpLdpSync()
		isismplsldpsync.Enabled = ygot.Bool(false)
	}
	if deviations.ISISInstanceEnabledRequired(dut) {
		globalISIS.Instance = ygot.String(isisInstance)
	}
	globalISIS.LevelCapability = oc.Isis_LevelType_LEVEL_2
	globalISIS.Net = []string{fmt.Sprintf("%v.%v.00", areaAddress, sysID)}
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)

	if deviations.GlobalMaxEcmpPathsUnsupported(dut) {
		globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).SetMaxEcmpPaths(maxEcmpPaths)
		globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).SetMaxEcmpPaths(maxEcmpPaths)
	} else {
		globalISIS.SetMaxEcmpPaths(maxEcmpPaths)
	}

	lspBit := globalISIS.GetOrCreateLspBit().GetOrCreateOverloadBit()
	lspBit.SetBit = ygot.Bool(false)

	isisLevel2 := isis.GetOrCreateLevel(2)
	isisLevel2.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC
	if deviations.ISISLevelEnabled(dut) {
		isisLevel2.Enabled = ygot.Bool(true)
	}
	for _, intfName := range intfs {
		intf := intfName
		if deviations.InterfaceRefInterfaceIDFormat(dut) {
			intf = intfName + ".0"
		}
		isisIntf := isis.GetOrCreateInterface(intf)
		if !deviations.IsisMplsUnsupported(dut) {
			// Explicit Disable the default igp-ldp-sync enabled interface level leaf
			isisintfmplsldpsync := isisIntf.GetOrCreateMpls().GetOrCreateIgpLdpSync()
			isisintfmplsldpsync.Enabled = ygot.Bool(false)
		}
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

func configureATE(t *testing.T, ate *otg.OTG) gosnappi.Config {
	cfg := gosnappi.NewConfig()
	p1 := cfg.Ports().Add().SetName("port1")
	p2 := cfg.Ports().Add().SetName("port2")
	p3 := cfg.Ports().Add().SetName("port3")

	// configure port 1 - src
	i1 := cfg.Devices().Add().SetName(atePort1.Name)
	i1Eth := i1.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	i1Eth.Connection().SetPortName(p1.Name())
	i1IPv4 := i1Eth.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4")
	i1IPv4.SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(plen4)
	i1IPv6 := i1Eth.Ipv6Addresses().Add().SetName(atePort1.Name + ".IPv6")
	i1IPv6.SetAddress(atePort1.IPv6).SetGateway(dutPort1.IPv6).SetPrefix(plen6)

	// configure lag2 - dst
	lag2 := cfg.Lags().Add().SetName("lag2")
	lag2.Protocol().Static().SetLagId(2)
	lag2.Ports().Add().SetPortName(p2.Name()).Ethernet().SetMac(lag2MAC).SetName("LAGRx-2")

	lag2Dev := cfg.Devices().Add().SetName(lag2.Name() + ".Dev")
	lag2Eth := lag2Dev.Ethernets().Add().SetName(atePort2.Name + ".Eth").SetMac(atePort2.MAC)
	lag2Eth.Connection().SetLagName(lag2.Name())
	lag2IPv4 := lag2Eth.Ipv4Addresses().Add().SetName(atePort2.Name + ".IPv4")
	lag2IPv4.SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).SetPrefix(plen4)
	lag2IPv6 := lag2Eth.Ipv6Addresses().Add().SetName(atePort2.Name + ".IPv6")
	lag2IPv6.SetAddress(atePort2.IPv6).SetGateway(dutPort2.IPv6).SetPrefix(plen6)

	// configure lag3 - dst
	lag3 := cfg.Lags().Add().SetName("lag3")
	lag3.Protocol().Static().SetLagId(3)
	lag3.Ports().Add().SetPortName(p3.Name()).Ethernet().SetMac(lag3MAC).SetName("LAGRx-3")

	lag3Dev := cfg.Devices().Add().SetName(lag3.Name() + ".Dev")
	lag3Eth := lag3Dev.Ethernets().Add().SetName(atePort3.Name + ".Eth").SetMac(atePort3.MAC)
	lag3Eth.Connection().SetLagName(lag3.Name())
	lag3IPv4 := lag3Eth.Ipv4Addresses().Add().SetName(atePort3.Name + ".IPv4")
	lag3IPv4.SetAddress(atePort3.IPv4).SetGateway(dutPort3.IPv4).SetPrefix(plen4)
	lag3IPv6 := lag3Eth.Ipv6Addresses().Add().SetName(atePort3.Name + ".IPv6")
	lag3IPv6.SetAddress(atePort3.IPv6).SetGateway(dutPort3.IPv6).SetPrefix(plen6)

	// configure ISIS on lags 2 & 3
	lag2Isis := lag2Dev.Isis().SetSystemId(otgLAG2sysID).SetName("lag2-isis")
	lag2Isis.Basic().SetHostname(lag2Isis.Name())
	lag2Isis.Advanced().SetAreaAddresses([]string{strings.Replace(areaAddress, ".", "", -1)})

	lag2IsisInt := lag2Isis.Interfaces().Add().
		SetEthName(lag2Dev.Ethernets().Items()[0].Name()).SetName("lag2IsisInt").
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetMetric(10)
	lag2IsisInt.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)

	lag3Isis := lag3Dev.Isis().SetSystemId(otgLAG3sysID).SetName("lag3-isis")
	lag3Isis.Basic().SetHostname(lag3Isis.Name())
	lag3Isis.Advanced().SetAreaAddresses([]string{strings.Replace(areaAddress, ".", "", -1)})

	lag3IsisInt := lag3Isis.Interfaces().Add().
		SetEthName(lag3Dev.Ethernets().Items()[0].Name()).SetName("lag3IsisInt").
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetMetric(10)
	lag3IsisInt.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)

	// configure emulated network params
	netLag2 := lag2Dev.Isis().V4Routes().Add().SetName("v4-isisNet-lag2").SetLinkMetric(10)
	netLag2.Addresses().Add().SetAddress(v4Route).SetPrefix(v4RoutePlen)

	netLag3 := lag3Dev.Isis().V4Routes().Add().SetName("v4-isisNet-lag3").SetLinkMetric(10)
	netLag3.Addresses().Add().SetAddress(v4Route).SetPrefix(v4RoutePlen)

	t.Log("Pushing config to ATE")
	ate.PushConfig(t, cfg)
	return cfg
}

func changeMetric(t *testing.T, dut *ondatra.DUTDevice, intf string, metric uint32) {
	t.Logf("Changing metric to %v on interface %v", metric, intf)
	d := &oc.Root{}
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	isis := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).GetOrCreateIsis()
	if deviations.InterfaceRefInterfaceIDFormat(dut) {
		intf += ".0"
	}
	isisIntfLevel := isis.GetOrCreateInterface(intf).GetOrCreateLevel(2)
	isisIntfLevelAfiv4 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
	isisIntfLevelAfiv4.Metric = ygot.Uint32(metric)
	isisIntfLevelAfiv6 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)
	isisIntfLevelAfiv6.Metric = ygot.Uint32(metric)
	if deviations.ISISRequireSameL1MetricWithL2Metric(dut) {
		l1 := isis.GetOrCreateInterface(intf).GetOrCreateLevel(1)
		l1V4 := l1.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
		l1V4.Metric = ygot.Uint32(metric)
		l1V6 := l1.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)
		l1V6.Metric = ygot.Uint32(metric)
	}
	gnmi.Update(t, dut, gnmi.OC().Config(), d)
}

func createFlow(t *testing.T, ateTopo gosnappi.Config, name string, dstPorts ...string) gosnappi.Flow {
	t.Helper()
	flow := ateTopo.Flows().Add()
	flow.SetName(name)
	flow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames(dstPorts)
	flow.Size().SetFixed(300)
	flow.Metrics().SetEnable(true)

	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(atePort1.MAC)

	ip := flow.Packet().Add().Ipv4()
	ip.Src().SetValue(atePort1.IPv4)
	ip.Dst().Increment().SetStart(v4IP).SetCount(50)

	tcp := flow.Packet().Add().Tcp()
	tcp.DstPort().Increment().SetStart(12345).SetCount(200)
	return flow
}

func configureTrafficFlows(t *testing.T, dut *ondatra.DUTDevice, otg *otg.OTG, flows []gosnappi.Flow) {
	if len(flows) == 0 {
		return
	}
	t.Logf("Configuring traffic flows")

	top := otg.GetConfig(t)
	top.Flows().Clear()
	for _, flow := range flows {
		top.Flows().Append(flow)
	}
	t.Log("Pushing config to ATE")
	otg.PushConfig(t, top)
	t.Logf("Starting protocols and awaiting for ARP & IS-IS adjacencies")
	otg.StartProtocols(t)
	time.Sleep(30 * time.Second)
	otgutils.WaitForARP(t, otg, top, "IPv4")
	otgutils.WaitForARP(t, otg, top, "IPv6")
	awaitAdjacency(t, dut, agg2ID)
	awaitAdjacency(t, dut, agg3ID)
}

func validateTrafficFlows(t *testing.T, dut *ondatra.DUTDevice, otg *otg.OTG, good []gosnappi.Flow, bad []gosnappi.Flow, nhCount int) {

	configureTrafficFlows(t, dut, otg, append(good, bad...))
	aftCheck(t, dut, nhCount)

	top := otg.GetConfig(t)
	otg.StartTraffic(t)
	time.Sleep(15 * time.Second)
	otg.StopTraffic(t)
	time.Sleep(10 * time.Second)

	otgutils.LogFlowMetrics(t, otg, top)

	for _, flow := range good {
		outPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State()))
		inPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).Counters().InPkts().State()))
		if outPkts == 0 {
			t.Fatalf("OutPkts for flow %s is 0, want > 0", flow)
		}
		if got := ((outPkts - inPkts) * 100) / outPkts; got > 0 {
			t.Errorf("LossPct for flow %s: got %v, want 0", flow.Name(), got)
		}
	}

	for _, flow := range bad {
		outPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State()))
		inPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).Counters().InPkts().State()))
		if outPkts == 0 {
			t.Fatalf("OutPkts for flow %s is 0, want > 0", flow)
		}
		if got := ((outPkts - inPkts) * 100) / outPkts; got < 100 {
			t.Errorf("LossPct for flow %s: got %v, want 100", flow.Name(), got)
		}
	}
}

func awaitAdjacency(t *testing.T, dut *ondatra.DUTDevice, intfName string) {
	isisPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Isis()
	if deviations.InterfaceRefInterfaceIDFormat(dut) {
		intfName += ".0"
	}
	intf := isisPath.Interface(intfName)

	query := intf.LevelAny().AdjacencyAny().AdjacencyState().State()
	_, ok := gnmi.WatchAll(t, dut, query, 2*time.Minute, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
		v, ok := val.Val()
		return v == oc.Isis_IsisInterfaceAdjState_UP && ok
	}).Await(t)

	if !ok {
		t.Fatalf("IS-IS adjacency was not formed on interface %v", intfName)
	}
}

func aftCheck(t *testing.T, dut *ondatra.DUTDevice, nhCount int) {
	aftsPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts()

	_, ok := gnmi.Watch(t, dut, aftsPath.Ipv4Entry(v4Route+"/24").State(), time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		ipv4Entry, present := val.Val()
		if !present {
			return false
		}
		hopGroup := gnmi.Get(t, dut, aftsPath.NextHopGroup(ipv4Entry.GetNextHopGroup()).State())
		got := len(hopGroup.NextHop)
		want := nhCount
		t.Logf("Aft check for %s: Got %d nexthop,want %d", ipv4Entry.GetPrefix(), got, want)
		return got == want

	}).Await(t)

	if !ok {
		t.Errorf("Aft check failed for %s", v4Route+"/24")
	}
}
func TestDrain(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()

	configureDUT(t, dut)
	ateTopo := configureATE(t, otg)

	ecmpFlows := createFlow(t, ateTopo, "ecmp-flow", atePort2.Name+".IPv4", atePort3.Name+".IPv4")
	lag2Flow := createFlow(t, ateTopo, "trunk2-flow", atePort2.Name+".IPv4")
	lag3Flow := createFlow(t, ateTopo, "trunk3-flow", atePort3.Name+".IPv4")

	t.Logf("Validating baseline traffic flow")
	validateTrafficFlows(t, dut, otg, []gosnappi.Flow{ecmpFlows}, nil, 2)

	// Change trunk-2 metric to 1000 and validate the traffic flows
	changeMetric(t, dut, agg2ID, 1000)
	t.Logf("Validating traffic flows after increasing the metric")
	validateTrafficFlows(t, dut, otg, []gosnappi.Flow{lag3Flow}, []gosnappi.Flow{lag2Flow}, 1)

	// Restore trunk-2 metric
	changeMetric(t, dut, agg2ID, 10)
	t.Logf("Validating traffic flows after restoring the metric")
	validateTrafficFlows(t, dut, otg, []gosnappi.Flow{ecmpFlows}, nil, 2)
}
