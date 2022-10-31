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
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/telemetry"
	otgtelemetry "github.com/openconfig/ondatra/telemetry/otg"
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
	lagTypeLACP   = telemetry.IfAggregate_AggregationType_LACP
	lagTypeSTATIC = telemetry.IfAggregate_AggregationType_STATIC
)

type testCase struct {
	lagType telemetry.E_IfAggregate_AggregationType

	dut *ondatra.DUTDevice
	ate *ondatra.ATEDevice
	top gosnappi.Config

	dutPorts []*ondatra.Port
	atePorts []*ondatra.Port
	aggID    string
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
}

func (tc *testCase) configDstMemberDUT(i *telemetry.Interface, p *ondatra.Port) {
	i.Description = ygot.String(p.String())
	i.Type = ethernetCsmacd

	if *deviations.InterfaceEnabled {
		i.Enabled = ygot.Bool(true)
	}

	e := i.GetOrCreateEthernet()
	e.AggregateId = ygot.String(tc.aggID)
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
		i.Type = ethernetCsmacd

		if *deviations.InterfaceEnabled {
			i.Enabled = ygot.Bool(true)
		}
	}

	p := tc.dut.Config()
	fptest.LogYgot(t, fmt.Sprintf("%s to Update()", tc.dut), p, d)
	p.Update(t, d)
}

func (tc *testCase) clearAggregate(t *testing.T) {
	// Clear the aggregate minlink.
	tc.dut.Config().Interface(tc.aggID).Aggregation().MinLinks().Delete(t)

	// Clear the members of the aggregate.
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
		tc.clearAggregate(t)
		tc.setupAggregateAtomically(t)
	}

	lacp := &telemetry.Lacp_Interface{Name: ygot.String(tc.aggID)}
	if tc.lagType == lagTypeLACP {
		lacp.LacpMode = telemetry.Lacp_LacpActivityType_ACTIVE
	} else {
		lacp.LacpMode = telemetry.Lacp_LacpActivityType_UNSET
	}
	lacpPath := d.Lacp().Interface(tc.aggID)
	fptest.LogYgot(t, "LACP", lacpPath, lacp)
	lacpPath.Replace(t, lacp)

	// TODO - to remove this sleep later
	time.Sleep(5 * time.Second)

	agg := &telemetry.Interface{Name: ygot.String(tc.aggID)}
	tc.configDstAggregateDUT(agg, &dutDst)
	aggPath := d.Interface(tc.aggID)
	fptest.LogYgot(t, tc.aggID, aggPath, agg)
	aggPath.Replace(t, agg)

	srcp := tc.dutPorts[0]
	srci := &telemetry.Interface{Name: ygot.String(srcp.Name())}
	tc.configSrcDUT(srci, &dutSrc)
	srci.Type = ethernetCsmacd
	srciPath := d.Interface(srcp.Name())
	fptest.LogYgot(t, srcp.String(), srciPath, srci)
	srciPath.Replace(t, srci)

	for _, port := range tc.dutPorts[1:] {
		i := &telemetry.Interface{Name: ygot.String(port.Name())}
		i.Type = ethernetCsmacd

		if *deviations.InterfaceEnabled {
			i.Enabled = ygot.Bool(true)
		}
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
	tc.top.Ports().Add().SetName(p0.ID())
	srcDev := tc.top.Devices().Add().SetName(ateSrc.Name)
	srcEth := srcDev.Ethernets().Add().SetName(ateSrc.Name + ".Eth")
	srcEth.Connection().SetChoice("port_name").SetPortName(p0.ID())
	srcEth.SetPortName(p0.ID()).SetMac(ateSrc.MAC)
	srcEth.Ipv4Addresses().Add().SetName(ateSrc.Name + ".IPv4").SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).SetPrefix(int32(ateSrc.IPv4Len))
	srcEth.Ipv6Addresses().Add().SetName(ateSrc.Name + ".IPv6").SetAddress(ateSrc.IPv6).SetGateway(dutSrc.IPv6).SetPrefix(int32(ateSrc.IPv6Len))

	// Adding the rest of the ports to the configuration and to the LAG
	agg := tc.top.Lags().Add().SetName(ateDst.Name)
	if tc.lagType == lagTypeSTATIC {
		lagId, _ := strconv.Atoi(tc.aggID)
		agg.Protocol().SetChoice("static").Static().SetLagId(int32(lagId))
		for i, p := range tc.atePorts[1:] {
			port := tc.top.Ports().Add().SetName(p.ID())
			newMac, err := incrementMAC(ateDst.MAC, i+1)
			if err != nil {
				t.Fatal(err)
			}
			agg.Ports().Add().SetPortName(port.Name()).Ethernet().SetMac(newMac).SetName("LAGRx-" + strconv.Itoa(i))
		}
	} else {
		agg.Protocol().SetChoice("lacp")
		for i, p := range tc.atePorts[1:] {
			port := tc.top.Ports().Add().SetName(p.ID())
			newMac, err := incrementMAC(ateDst.MAC, i+1)
			if err != nil {
				t.Fatal(err)
			}
			lagPort := agg.Ports().Add().SetPortName(port.Name())
			lagPort.Ethernet().SetMac(newMac).SetName("LAGRx-" + strconv.Itoa(i))
			lagPort.Lacp().SetActorActivity("active").SetActorPortNumber(int32(i) + 1).SetActorPortPriority(1).SetLacpduTimeout(0)
		}
	}

	dstDev := tc.top.Devices().Add().SetName(agg.Name())
	dstEth := dstDev.Ethernets().Add().SetName(ateDst.Name + ".Eth").SetPortName(agg.Name()).SetMac(ateDst.MAC)
	dstEth.Connection().SetChoice("lag_name").SetLagName(agg.Name())
	dstEth.Ipv4Addresses().Add().SetName(ateDst.Name + ".IPv4").SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).SetPrefix(int32(ateDst.IPv4Len))
	dstEth.Ipv6Addresses().Add().SetName(ateDst.Name + ".IPv6").SetAddress(ateDst.IPv6).SetGateway(dutDst.IPv6).SetPrefix(int32(ateDst.IPv6Len))

	// Fail early if the topology is bad.
	tc.ate.OTG().PushConfig(t, tc.top)
	tc.ate.OTG().StartProtocols(t)
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

func (tc *testCase) verifyAggID(t *testing.T, dp *ondatra.Port) {
	dip := tc.dut.Telemetry().Interface(dp.Name())
	di := dip.Get(t)
	if lagID := di.GetEthernet().GetAggregateId(); lagID != tc.aggID {
		t.Errorf("%s LagID got %v, want %v", dp, lagID, tc.aggID)
	}
}

func (tc *testCase) verifyInterfaceDUT(t *testing.T, dp *ondatra.Port) {
	dip := tc.dut.Telemetry().Interface(dp.Name())
	di := dip.Get(t)
	fptest.LogYgot(t, dp.String()+" before Await", dip, di)

	if got := di.GetAdminStatus(); got != adminUp {
		t.Errorf("%s admin-status got %v, want %v", dp, got, adminUp)
	}

	// LAG members may fall behind, so wait for them to be up.
	dip.OperStatus().Await(t, time.Minute, opUp)
}

func (tc *testCase) verifyDUT(t *testing.T) {
	// Wait for LAG negotiation and verify LAG type for the aggregate interface.
	tc.dut.Telemetry().Interface(tc.aggID).Type().Await(t, time.Minute, ieee8023adLag)

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
	// State for the interface.
	time.Sleep(3 * time.Second)
	otgutils.LogLAGMetrics(t, tc.ate.OTG(), tc.top)

	if tc.lagType == telemetry.IfAggregate_AggregationType_LACP {
		otgutils.LogLACPMetrics(t, tc.ate.OTG(), tc.top)
	}
	portMetrics := tc.ate.OTG().Telemetry().Port(ap.ID()).Get(t)
	if portMetrics.GetLink() != otgtelemetry.Port_Link_UP {
		t.Errorf("%s oper-status got %v, want %v", ap.ID(), portMetrics.GetLink(), otgtelemetry.Port_Link_UP)
	}
	t.Logf("Checking if LAG is up on OTG")
	tc.ate.OTG().Telemetry().Lag(ateDst.Name).OperStatus().Watch(t, time.Minute, func(val *otgtelemetry.QualifiedE_Lag_OperStatus) bool {
		return val.Val(t).String() == "UP"
	}).Await(t)

}

// setDutInterfaceWithState sets the admin state to a member of the lag
func (tc *testCase) setDutInterfaceWithState(t testing.TB, p *ondatra.Port, state bool) {
	dc := tc.dut.Config()
	i := &telemetry.Interface{}
	i.Enabled = ygot.Bool(state)
	dc.Interface(p.Name()).Update(t, i)
}

// sortPorts sorts the ports by the testbed port ID.
func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})
	return ports
}

// incrementMAC increments the MAC by i. Returns error if the mac cannot be parsed or overflows the mac address space
func incrementMAC(mac string, i int) (string, error) {
	macAddr, err := net.ParseMAC(mac)
	if err != nil {
		return "", err
	}
	convMac := binary.BigEndian.Uint64(append([]byte{0, 0}, macAddr...))
	convMac = convMac + uint64(i)
	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, convMac)
	if err != nil {
		return "", err
	}
	newMac := net.HardwareAddr(buf.Bytes()[2:8])
	return newMac.String(), nil
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
				if tc.lagType == telemetry.IfAggregate_AggregationType_LACP {

					// Linked DUT and ATE ports have the same ID.
					dp := tc.dut.Port(t, port.ID())
					t.Logf("Taking otg port %s down in the LAG", port.ID())
					tc.ate.OTG().DisableLACPMembers(t, []string{port.ID()})
					time.Sleep(3 * time.Second)
					otgutils.LogLACPMetrics(t, tc.ate.OTG(), tc.top)
					otgutils.LogLAGMetrics(t, tc.ate.OTG(), tc.top)

					t.Logf("Awaiting LAG DUT port: %v to stop collecting", dp)
					tc.dut.Telemetry().Lacp().InterfaceAny().Member(dp.Name()).Collecting().Watch(
						t, time.Minute, func(val *telemetry.QualifiedBool) bool {
							return !val.Val(t)
						}).Await(t)
					t.Logf("Awaiting LAG DUT port: %v to stop distributing", dp)
					tc.dut.Telemetry().Lacp().InterfaceAny().Member(dp.Name()).Distributing().Watch(
						t, time.Minute, func(val *telemetry.QualifiedBool) bool {
							return !val.Val(t)
						}).Await(t)

				}
				if tc.lagType == telemetry.IfAggregate_AggregationType_STATIC {
					// Setting admin state down on the DUT interface. Setting the otg interface down has no effect in kne
					dp := tc.dut.Port(t, port.ID())
					tc.setDutInterfaceWithState(t, dp, false)
				}
			}
			tc.dut.Telemetry().Interface(tc.aggID).OperStatus().Await(t, 1*time.Minute, tf.want)
		})
	}
}

func TestNegotiation(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	aggID := netutil.NextBundleInterface(t, dut)

	lagTypes := []telemetry.E_IfAggregate_AggregationType{lagTypeLACP, lagTypeSTATIC}

	for _, lagType := range lagTypes {
		top := ate.OTG().NewConfig(t)

		tc := &testCase{
			dut:     dut,
			ate:     ate,
			top:     top,
			lagType: lagType,

			dutPorts: sortPorts(dut.Ports()),
			atePorts: sortPorts(ate.Ports()),
			aggID:    aggID,
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
