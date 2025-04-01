/ Copyright 2022 Google LLC
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

package breakout_subscription_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"sort"
	"strconv"
	"strings"
	"testing"
	"text/tabwriter"
	"time"

	"google3/third_party/golang/cmp/cmp"
	"google3/third_party/golang/cmp/cmpopts/cmpopts"
	"google3/third_party/golang/ygot/ygot/ygot"
	"google3/third_party/open_traffic_generator/gosnappi/gosnappi"
	"google3/third_party/openconfig/featureprofiles/internal/attrs/attrs"
	"google3/third_party/openconfig/featureprofiles/internal/deviations/deviations"
	"google3/third_party/openconfig/featureprofiles/internal/fptest/fptest"
	"google3/third_party/openconfig/featureprofiles/internal/otgutils/otgutils"
	"google3/third_party/openconfig/ondatra/ondatra"
	"google3/third_party/openconfig/ygnmi/ygnmi/ygnmi"

	"google3/third_party/openconfig/ondatra/gnmi/gnmi"
	"google3/third_party/openconfig/ondatra/gnmi/oc/oc"
	otgtelemetry "google3/third_party/openconfig/ondatra/gnmi/otg/otg"
	"google3/third_party/openconfig/ondatra/netutil/netutil"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the aggregate testbed with the test
// topology.  IxNetwork flow requires both source and destination
// networks be configured on the ATE.  It is not possible to send
// packets to the ether.
//
// The testbed consists of ate:port{1-2} -> dut:port{1-2} and dut:port3 ->
// ate:port3.  The first pair is called the "source" aggregatepair, and the
// second  link the "destination" pair.
//
//   * Source: ate:port{1-2} -> dut:port{1-2} subnet 192.0.2.0/30 2001:db8::0/126
//   * Destination: dut:port3 -> ate:port3
//     subnet 192.0.2.4/30 2001:db8::4/126
//
// Note that the first (.0, .4) and last (.3, .7) IPv4 addresses are
// reserved from the subnet for broadcast, so a /30 leaves exactly 2
// usable addresses.  This does not apply to IPv6 which allows /127
// for point to point links, but we use /126 so the numbering is
// consistent with IPv4.
//
const (
	plen4          = 30
	plen6          = 126
	opUp           = oc.Interface_OperStatus_UP
	ethernetCsmacd = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	ieee8023adLag  = oc.IETFInterfaces_InterfaceType_ieee8023adLag
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
		MAC:     "02:11:01:00:00:01",
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
		MAC:     "02:12:01:00:00:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
)

const (
	lagTypeLACP = oc.IfAggregate_AggregationType_LACP
)

type testCase struct {
	desc    string
	dut     *ondatra.DUTDevice
	ate     *ondatra.ATEDevice
	top     gosnappi.Config
	lagType oc.E_IfAggregate_AggregationType

	dutPorts []*ondatra.Port
	atePorts []*ondatra.Port
	aggID    string
	l3header string
}
func (tc *testCase) configureATE(t *testing.T) {
	if len(tc.atePorts) < 2 {
		t.Fatalf("Testbed requires at least 2 ports, got: %v", tc.atePorts)
	}

	p0 := tc.atePorts[2]
	tc.top.Ports().Add().SetName(p0.ID())
	d0 := tc.top.Devices().Add().SetName(ateDst.Name)
	srcEth := d0.Ethernets().Add().SetName(ateDst.Name + ".Eth").SetMac(ateDst.MAC)
	srcEth.Connection().SetPortName(p0.ID())
	srcEth.Ipv4Addresses().Add().SetName(ateDst.Name + ".IPv4").SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).SetPrefix(uint32(ateDst.IPv4Len))
	srcEth.Ipv6Addresses().Add().SetName(ateDst.Name + ".IPv6").SetAddress(ateDst.IPv6).SetGateway(dutDst.IPv6).SetPrefix(uint32(ateDst.IPv6Len))

	agg := tc.top.Lags().Add().SetName("LAG")
	for i, p := range tc.atePorts[0:1] {
		port := tc.top.Ports().Add().SetName(p.ID())
		lagPort := agg.Ports().Add()
		newMac, err := incrementMAC(ateSrc.MAC, i+1)
		if err != nil {
			t.Fatal(err)
		}
		lagPort.SetPortName(port.Name()).
			Ethernet().SetMac(newMac).
			SetName("LAGRx-" + strconv.Itoa(i))
		lagPort.Lacp().SetActorPortNumber(uint32(i + 1)).SetActorPortPriority(1).SetActorActivity("active")
	}
	agg.Protocol().Lacp().SetActorKey(1).SetActorSystemPriority(1).SetActorSystemId("01:01:01:01:01:01")

	// Disable FEC for 100G-FR ports because Novus does not support it.
	p100gbasefr := []string{}
	for _, p := range tc.atePorts {
		if p.PMD() == ondatra.PMD100GBASEFR {
			p100gbasefr = append(p100gbasefr, p.ID())
		}
	}

	if len(p100gbasefr) > 0 {
		l1Settings := tc.top.Layer1().Add().SetName("L1").SetPortNames(p100gbasefr)
		l1Settings.SetAutoNegotiate(true).SetIeeeMediaDefaults(false).SetSpeed("speed_100_gbps")
		autoNegotiate := l1Settings.AutoNegotiation()
		autoNegotiate.SetRsFec(false)
	}

	dstDev := tc.top.Devices().Add().SetName(agg.Name() + ".dev")
	dstEth := dstDev.Ethernets().Add().SetName(ateSrc.Name + ".Eth").SetMac(ateSrc.MAC)
	dstEth.Connection().SetLagName(agg.Name())
	dstEth.Ipv4Addresses().Add().SetName(ateSrc.Name + ".IPv4").SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).SetPrefix(uint32(ateSrc.IPv4Len))
	dstEth.Ipv6Addresses().Add().SetName(ateSrc.Name + ".IPv6").SetAddress(ateSrc.IPv6).SetGateway(dutSrc.IPv6).SetPrefix(uint32(ateSrc.IPv6Len))

	tc.ate.OTG().PushConfig(t, tc.top)
	tc.ate.OTG().StartProtocols(t)
}

func (tc *testCase) clearAggregateMembers(t *testing.T) {
	for n, port := range tc.dutPorts {
		if n < 1 {
			// We designate port 0 as the source link, not part of LAG.
			continue
		}
		gnmi.Delete(t, tc.dut, gnmi.OC().Interface(port.Name()).Ethernet().AggregateId().Config())
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

	for n, port := range tc.dutPorts {
		if n < 1 {
			// We designate port 0 as the source link, not part of LAG.
			continue
		}
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(tc.aggID)
		i.Type = ethernetCsmacd

		if deviations.InterfaceEnabled(tc.dut) {
			i.Enabled = ygot.Bool(true)
		}
	}

	p := gnmi.OC()
	fptest.LogQuery(t, fmt.Sprintf("%s to Update()", tc.dut), p.Config(), d)
	gnmi.Update(t, tc.dut, p.Config(), d)
}

func (tc *testCase) configSrcAggregateDUT(i *oc.Interface, a *attrs.Attributes) {
	tc.configSrcDUT(i, a)
	i.Type = ieee8023adLag
	g := i.GetOrCreateAggregation()
	g.LagType = tc.lagType
}

func (tc *testCase) configSrcMemberDUT(i *oc.Interface, p *ondatra.Port) {
	i.Description = ygot.String(p.String())
	i.Type = ethernetCsmacd
	if deviations.InterfaceEnabled(tc.dut) {
		i.Enabled = ygot.Bool(true)
	}

	e := i.GetOrCreateEthernet()
	e.AggregateId = ygot.String(tc.aggID)
}

func (tc *testCase) configDstDUT(i *oc.Interface, a *attrs.Attributes) {
	i.Description = ygot.String(a.Desc)
	if deviations.InterfaceEnabled(tc.dut) {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(tc.dut) && !deviations.IPv4MissingEnabled(tc.dut) {
		s4.Enabled = ygot.Bool(true)
	}
	a4 := s4.GetOrCreateAddress(a.IPv4)
	a4.PrefixLength = ygot.Uint8(plen4)

	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(tc.dut) {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(plen6)
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

	if tc.lagType == lagTypeLACP {
		lacp := &oc.Lacp_Interface{Name: ygot.String(tc.aggID)}
		lacp.LacpMode = oc.Lacp_LacpActivityType_ACTIVE

		lacpPath := d.Lacp().Interface(tc.aggID)
		fptest.LogQuery(t, "LACP", lacpPath.Config(), lacp)
		gnmi.Replace(t, tc.dut, lacpPath.Config(), lacp)
	}

	time.Sleep(5 * time.Second)

	agg := &oc.Interface{Name: ygot.String(tc.aggID)}
	tc.configSrcAggregateDUT(agg, &dutDst)
	aggPath := d.Interface(tc.aggID)
	fptest.LogQuery(t, tc.aggID, aggPath.Config(), agg)
	gnmi.Replace(t, tc.dut, aggPath.Config(), agg)

	dstp := tc.dutPorts[]
	dsti := &oc.Interface{Name: ygot.String(srcp.Name())}
	tc.configDstDUT(dsti, &dutDst)
	dsti.Type = ethernetCsmacd
	dstiPath := d.Interface(dstp.Name())
	fptest.LogQuery(t, dstp.String(), dstiPath.Config(), dsti)
	gnmi.Replace(t, tc.dut, dstiPath.Config(), dsti)
	if deviations.ExplicitInterfaceInDefaultVRF(tc.dut) {
		fptest.AssignToNetworkInstance(t, tc.dut, dstp.Name(), deviations.DefaultNetworkInstance(tc.dut), 0)
		fptest.AssignToNetworkInstance(t, tc.dut, tc.aggID, deviations.DefaultNetworkInstance(tc.dut), 0)
	}
	for _, port := range tc.dutPorts[0:1] {
		i := &oc.Interface{Name: ygot.String(port.Name())}
		tc.configSrcMemberDUT(i, port)
		iPath := d.Interface(port.Name())
		fptest.LogQuery(t, port.String(), iPath.Config(), i)
		gnmi.Replace(t, tc.dut, iPath.Config(), i)
	}
	if deviations.ExplicitPortSpeed(tc.dut) {
		for _, port := range tc.dutPorts {
			fptest.SetPortSpeed(t, port)
		}
	}
}
func subscribeOnChangeInterfaceName(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) *gnmi.Watcher[string] {
	t.Helper()

	interfaceNamePath := gnmi.OC().Interface(p.Name()).Name().State()
	t.Logf("TRY: subscribe ON_CHANGE to %s", interfaceNamePath)

	watchName := gnmi.Watch(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
		interfaceNamePath,
		time.Minute,
		func(val *ygnmi.Value[string]) bool {
			iname, present := val.Val()
			return present && iname == p.Name()
		})

	return watchName
}
// subscribeOnChangeInterfaceID sends gnmi subscribe ON_CHANGE to various paths

func subscribeOnChange(t *testing.T, dut *ondatra.DUTDevice) *gnmi.Watcher[uint32] {
	t.Helper()

	p1 := dut.Port(t, "port1")

	interfaceIDPath := gnmi.OC().Interface(p1.Name()).Id().State()
	hardwarePortPath := gnmi.OC().Interface(p1.Name()).HardwarePort().State()
	adminStatusPath := gnmi.OC().Interface(p1.Name()).AdminStatus().State()
	operStatusPath := gnmi.OC().Interface(p1.Name()).OperStatus().State()
	forwardingViablePath := gnmi.OC().Interface(p1.Name()).ForwardingViable().State()
	ethernetMacAddressPath := gnmi.OC().Interface(p1.Name()).Ethernet().MacAddress().State()
	ethernetPortSpeedPath := gnmi.OC().Interface(p1.Name()).Ethernet().PortSpeed().State()

	watchID := gnmi.Watch(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
		interfaceIDPath,
		time.Minute,
		// Stop the gnmi.Watch() if value is invalid.
		func(val *ygnmi.Value[uint32]) bool {
			id, present := val.Val()
			return !present || id != dutPort1.ID
		})
	watchHardwarePort := gnmi.Watch(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
		hardwarePortPath,
		time.Minute,
		// Stop the gnmi.Watch() if value is invalid.
		func(val *ygnmi.Value[uint32]) bool {
			id, present := val.Val()
			return !present || id != dutPort1.ID
		})
	watchAdminStatus := gnmi.Watch(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
		adminStatusPath,
		time.Minute,
		// Stop the gnmi.Watch() if value is invalid.
		func(val *ygnmi.Value[oc.E_Interface_AdminStatus]) bool {
			status, present := val.Val()
			return !present || status != oc.Interface_AdminStatus_UP
		})
	watchOperStatus := gnmi.Watch(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
		operStatusPath,
		time.Minute,
		// Stop the gnmi.Watch() if value is invalid.
		func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
			status, present := val.Val()
			return !present || status != opUp
		})
	watchForwardingViable := gnmi.Watch(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
		forwardingViablePath,
		time.Minute,
		// Stop the gnmi.Watch() if value is invalid.
		func(val *ygnmi.Value[bool]) bool {
			viable, present := val.Val()
			return !present || viable!= true
		})
	watchEthernetMacAddress := gnmi.Watch(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
		ethernetMacAddressPath,
		time.Minute,
		// Stop the gnmi.Watch() if value is invalid.
		func(val *ygnmi.Value[string]) bool {
			mac, present := val.Val()
			return !present || mac != dutPort1.MAC
		})
	watchEthernetPortSpeed := gnmi.Watch(t,
		dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE)),
		ethernetPortSpeedPath,
		time.Minute,
		// Stop the gnmi.Watch() if value is invalid.
		func(val *ygnmi.Value[oc.E_IfEthernet_ETHERNET_SPEED]) bool {
			speed, present := val.Val()
			return !present || speed != oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB
		})

	return watchID, watchHardwarePort, watchAdminStatus, watchOperStatus, watchForwardingViable, watchEthernetMacAddress, watchEthernetPortSpeed
}

// define different flows for traffic
func TestBreakoutSubscription(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	aggID := netutil.NextAggregateInterface(t, dut)

	tests := []testCase{
		{
			desc:     "PLT-1.2.1 Check response after a triggered interface state change",

		},
		{
			desc:     "PLT-1.2.2 Check response after a triggered interface flap",

		},
		{
			desc:     "PLT-1.2.3 Check response after a triggered LC reboot",

		},
		{
			desc:     "PLT-1.2.4 Check response after a triggered reboot",

		},

	}
	tc := &testCase{
		dut:     dut,
		ate:     ate,
		lagType: lagTypeLACP,
		top:     gosnappi.NewConfig(),

		dutPorts: sortPorts(dut.Ports()),
		atePorts: sortPorts(ate.Ports()),
		aggID:    aggID,
	}
	tc.configureATE(t)
	tc.configureDUT(t)
	t.Run("verifyDUT", tc.verifyDUT)

	for _, tf := range tests {
		t.Run(tf.desc, func(t *testing.T) {
				// Verify subscribe ON_CHANGE is supported using a commonly supported OC path.
			 watchName, ok := subscribeOnChangeInterfaceName(t, dut, dut.Ports()[0]).Await(t)
			if !ok {
				t.Fatalf("/interfaces/interface[name=%q]/state/name got:%v want:%q", dut.Ports()[0].Name(), watchName, dut.Ports()[0].Name())
			}
		
			// Subscribe ON_CHANGE to '/interfaces/interface/state/id'.
			watchPaths[] := subscribeOnChangeInterfaceID(t, dut)
			

		})
	}
}
