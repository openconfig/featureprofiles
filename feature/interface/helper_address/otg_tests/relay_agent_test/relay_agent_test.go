// Copyright 2025 Google LLC
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

package relay_agent_test

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/iputil"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ipv4PrefixLen           = 30
	ipv6PrefixLen           = 126
	dhcpLeaseStartAddress   = "192.0.2.34"
	dhcpLeaseGateway        = "192.0.2.33"
	dhcpLeasePrefixLen      = 27
	dhcpV6LeaseStartAddress = "2001:db8:a:2::2"
	dhcpV6LeasePrefixLen    = 64
	vlanID                  = 10
	lagTypeLACP             = oc.IfAggregate_AggregationType_LACP
	ieee8023adLag           = oc.IETFInterfaces_InterfaceType_ieee8023adLag
)

var (
	dutP1 = attrs.Attributes{
		Name:    "dutP1",
		Desc:    "dhcp-relay-port",
		IPv4:    "192.0.2.33",
		IPv6:    "2001:db8:a:2::1",
		IPv4Len: dhcpLeasePrefixLen,
		IPv6Len: dhcpV6LeasePrefixLen,
		MTU:     1500,
		ID:      1,
	}

	ateP1 = attrs.Attributes{
		Name: "ateP1",
		Desc: "dhcp-client",
		MAC:  "02:11:01:00:00:01",
	}

	dutP3 = attrs.Attributes{
		Name:    "dutP3",
		Desc:    "dutate-2",
		IPv4:    "192.0.2.253",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	ateP3 = attrs.Attributes{
		Name:    "ateP3",
		MAC:     "02:00:02:01:01:02",
		IPv4:    "192.0.2.254",
		IPv6:    "2001:db8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
)

type testCase struct {
	name string
	dut  *ondatra.DUTDevice
	ate  *ondatra.ATEDevice
	conf gosnappi.Config

	dutPorts []*ondatra.Port
	atePorts []*ondatra.Port
	aggID    string
	isLag    bool
	lagType  oc.E_IfAggregate_AggregationType
}

// sortPorts sorts the ports by the testbed port ID.
func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})
	return ports
}

// configureEth configures the ethernet device on the ATE with either a direct port or a LAG based on the test case parameters.
func (tc *testCase) configureEth(t *testing.T) gosnappi.DeviceEthernet {
	t.Helper()

	if tc.isLag {
		lagIface := tc.conf.Lags().Add().SetName(ateP1.Name + ".Lag")
		lagIface.Protocol().Lacp().SetActorKey(1).SetActorSystemPriority(1).SetActorSystemId(ateP1.MAC)

		for i, atePort := range tc.atePorts[0:2] {
			port := tc.conf.Ports().Add().SetName(atePort.ID())
			mac, err := iputil.IncrementMAC(ateP1.MAC, i+1)
			if err != nil {
				t.Fatal(err)
			}
			lagPort := lagIface.Ports().Add().SetPortName(port.Name())
			lagPort.Ethernet().SetMac(mac).SetName("LAGRx-" + strconv.Itoa(i))
			lagPort.Lacp().SetActorActivity("active").SetActorPortNumber(uint32(i) + 1).SetActorPortPriority(1).SetLacpduTimeout(0)
		}

		dev := tc.conf.Devices().Add().SetName(ateP1.Name)
		eth := dev.Ethernets().
			Add().
			SetName(ateP1.Name + ".eth").
			SetMac(ateP1.MAC).
			SetMtu(1500)
		eth.Connection().SetLagName(lagIface.Name())
		return eth
	} else {
		port := tc.conf.Ports().Add().SetName(tc.atePorts[0].ID())
		dev := tc.conf.Devices().Add().SetName(ateP1.Name)
		eth := dev.Ethernets().
			Add().
			SetName(ateP1.Name + ".eth").
			SetMac(ateP1.MAC).
			SetMtu(1500)
		eth.Connection().SetPortName(port.Name())
		return eth
	}
}

// dhcpClientConfig configures the DHCP client (ATE port 1) on the given ethernet device.
func (tc *testCase) dhcpClientConfig(t *testing.T) {
	t.Helper()

	clientEth := tc.configureEth(t)

	// Add VLAN
	clientVlan := clientEth.Vlans().Add().SetName(ateP1.Name + ".vlan")
	clientVlan.SetId(uint32(vlanID))

	// Configure DHCPv4 client
	dhcpV4Client := clientEth.Dhcpv4Interfaces().Add().
		SetName(ateP1.Name + ".dhcp-client")
	dhcpV4Client.FirstServer()
	dhcpV4Client.ParametersRequestList().
		SetSubnetMask(true).
		SetRouter(true).
		SetRenewalTimer(true)

	// Configure DHCPv6 client
	dhcpV6Client := clientEth.Dhcpv6Interfaces().Add().
		SetName(ateP1.Name + ".dhcp-v6-client")
	dhcpV6Client.IaType().Iana()
	dhcpV6Client.DuidType().Llt()

}

// dhcpServerConfig configures the DHCP server (ATE port 3) on the given config.
func (tc *testCase) dhcpServerConfig(t *testing.T) {
	t.Helper()

	// Add DHCP server port and device
	serverPort := tc.conf.Ports().Add().SetName(tc.atePorts[2].ID())
	serverDevice := tc.conf.Devices().Add().SetName(ateP3.Name)

	// Configure server ethernet
	serverEth := serverDevice.Ethernets().
		Add().
		SetName(ateP3.Name + ".eth").
		SetMac(ateP3.MAC).
		SetMtu(1500)
	serverEth.Connection().SetPortName(serverPort.Name())

	// Configure server IPv4 address
	serverIPv4 := serverEth.Ipv4Addresses().
		Add().
		SetName(ateP3.Name + ".ip").
		SetAddress(ateP3.IPv4).
		SetGateway(dutP3.IPv4).
		SetPrefix(ipv4PrefixLen)

	// Configure DHCPv4 server and address pool
	dhcpV4Server := serverDevice.DhcpServer().Ipv4Interfaces().Add().
		SetName(serverIPv4.Name() + ".dhcp-server")
	dhcpV4Server.SetIpv4Name(serverIPv4.Name()).
		AddressPools().
		Add().
		SetName(dhcpV4Server.Name() + ".server-pool").
		SetLeaseTime(3600).
		SetStartAddress(dhcpLeaseStartAddress).
		SetStep(1).
		SetCount(uint32(1)).
		SetPrefixLength(27).
		Options().
		SetRouterAddress(dutP1.IPv4).
		SetEchoRelayWithTlv82(true)

	// Configure server IPv6 address
	serverIPv6 := serverEth.Ipv6Addresses().
		Add().
		SetName(ateP3.Name + ".ipv6").
		SetAddress(ateP3.IPv6).
		SetGateway(dutP3.IPv6).
		SetPrefix(ipv6PrefixLen)

	// Configure DHCPv6 server and lease pool
	dhcpV6Server := serverDevice.DhcpServer().Ipv6Interfaces().Add().
		SetName(serverIPv6.Name() + ".dhcpv6-server")

	dhcpV6Lease := dhcpV6Server.SetIpv6Name(serverIPv6.Name()).
		Leases().Add()
	dhcpV6Lease.SetLeaseTime(3600)

	dhcpV6IaType := dhcpV6Lease.IaType().Iana()
	dhcpV6IaType.
		SetPrefixLen(64).
		SetStartAddress(dhcpV6LeaseStartAddress).
		SetStep(1).
		SetSize(uint32(1))

}

// configureATE configures ATE interfaces
func (tc *testCase) configureATE(t *testing.T) {
	t.Helper()

	tc.dhcpClientConfig(t)
	tc.dhcpServerConfig(t)

}

// configDUTInterface configures the DUT interface with the given attributes and applies necessary deviations.
func configDUTInterface(i *oc.Interface, a *attrs.Attributes, dut *ondatra.DUTDevice) {
	i.Description = ygot.String(a.Desc)
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	s1 := i.GetOrCreateSubinterface(0)
	b4 := s1.GetOrCreateIpv4()
	b6 := s1.GetOrCreateIpv6()
	b4.Mtu = ygot.Uint16(a.MTU)
	b6.Mtu = ygot.Uint32(uint32(a.MTU))
	if deviations.InterfaceEnabled(dut) {
		b4.Enabled = ygot.Bool(true)
	}
	s := i.GetOrCreateSubinterface(a.ID)
	s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().SetVlanId(uint16(vlanID))
	configureInterfaceAddress(dut, s, a)
}

// configureInterfaceAddress configures the IP addresses on the given subinterface based on the provided attributes.
func configureInterfaceAddress(dut *ondatra.DUTDevice, s *oc.Interface_Subinterface, a *attrs.Attributes) {
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	if a.IPv4 != "" {
		a4 := s4.GetOrCreateAddress(a.IPv4)
		a4.PrefixLength = ygot.Uint8(a.IPv4Len)
	}
	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	if a.IPv6 != "" {
		s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(a.IPv6Len)
	}

	if a.IPv6Sec != "" {
		s62 := s.GetOrCreateIpv6()
		if deviations.InterfaceEnabled(dut) {
			s62.Enabled = ygot.Bool(true)
		}
		s62.GetOrCreateAddress(a.IPv6Sec).PrefixLength = ygot.Uint8(a.IPv6Len)
	}
}

// configureLAGInterface sets up the LAG aggregate interface with LACP and applies DHCP relay CLI config.
func (tc *testCase) configureLAGInterface(t *testing.T, attrs *attrs.Attributes) {
	t.Helper()
	d := gnmi.OC()
	var dutAggPorts []*ondatra.Port
	for _, port := range tc.dutPorts[0:2] {
		dutAggPorts = append(dutAggPorts, tc.dut.Port(t, port.ID()))
	}
	if deviations.AggregateAtomicUpdate(tc.dut) {
		cfgplugins.DeleteAggregate(t, tc.dut, tc.aggID, dutAggPorts)
		cfgplugins.SetupAggregateAtomically(t, tc.dut, tc.aggID, dutAggPorts)
	}

	lacp := &oc.Lacp_Interface{Name: ygot.String(tc.aggID)}
	lacp.LacpMode = oc.Lacp_LacpActivityType_ACTIVE
	lacpPath := d.Lacp().Interface(tc.aggID)
	gnmi.Replace(t, tc.dut, lacpPath.Config(), lacp)

	agg := &oc.Interface{Name: ygot.String(tc.aggID)}
	configDUTInterface(agg, attrs, tc.dut)
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_LACP
	agg.Type = ieee8023adLag
	aggPath := d.Interface(tc.aggID)
	gnmi.Replace(t, tc.dut, aggPath.Config(), agg)

	for _, port := range dutAggPorts {
		holdTimeConfig := &oc.Interface_HoldTime{
			Up:   ygot.Uint32(3000),
			Down: ygot.Uint32(150),
		}
		intfPath := gnmi.OC().Interface(port.Name())
		gnmi.Update(t, tc.dut, intfPath.HoldTime().Config(), holdTimeConfig)
	}

	b := new(gnmi.SetBatch)
	dhcpRelayConfigParams := cfgplugins.DHCPRelayConfigParams{
		Interface:         fmt.Sprintf("%s.%d", *agg.Name, attrs.ID),
		IPv4HelperAddress: ateP3.IPv4,
		IPv6HelperAddress: ateP3.IPv6,
	}
	cfgplugins.DhcpRelayConfig(t, tc.dut, b, dhcpRelayConfigParams)
}

// configureEthernetInterface sets up a standard ethernet interface with VLAN subinterface and applies DHCP relay CLI config.
func (tc *testCase) configureEthernetInterface(t *testing.T, attrs *attrs.Attributes) {
	t.Helper()
	iface := &oc.Interface{}
	iface.Name = ygot.String(tc.dutPorts[0].Name())
	iface.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(tc.dut) {
		iface.Enabled = ygot.Bool(true)
	}

	subIface0 := iface.GetOrCreateSubinterface(0)
	if deviations.InterfaceEnabled(tc.dut) {
		subIface0.Enabled = ygot.Bool(true)
	}

	subIface0IPv4 := subIface0.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(tc.dut) && !deviations.IPv4MissingEnabled(tc.dut) {
		subIface0IPv4.Enabled = ygot.Bool(true)
	}

	// Configure subinterface 1 with VLAN and IP
	subIface1 := iface.GetOrCreateSubinterface(attrs.ID)
	if deviations.InterfaceEnabled(tc.dut) {
		subIface1.Enabled = ygot.Bool(true)
	}
	if deviations.DeprecatedVlanID(tc.dut) {
		subIface1.GetOrCreateVlan().VlanId = oc.UnionUint16(vlanID)
	} else {
		subIface1.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(uint16(vlanID))
	}

	subIface1IPv4 := subIface1.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(tc.dut) && !deviations.IPv4MissingEnabled(tc.dut) {
		subIface1IPv4.Enabled = ygot.Bool(true)
	}
	subIface1IPv4Addr := subIface1IPv4.GetOrCreateAddress(attrs.IPv4)
	subIface1IPv4Addr.PrefixLength = ygot.Uint8(attrs.IPv4Len)

	subIface1IPv6 := subIface1.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(tc.dut) {
		subIface1IPv6.Enabled = ygot.Bool(true)
	}
	subIface1IPv6Addr := subIface1IPv6.GetOrCreateAddress(attrs.IPv6)
	subIface1IPv6Addr.PrefixLength = ygot.Uint8(attrs.IPv6Len)

	ifacePath := gnmi.OC().Interface(tc.dutPorts[0].Name())
	gnmi.Update(t, tc.dut, ifacePath.Config(), iface)

	b := new(gnmi.SetBatch)
	dhcpRelayConfigParams := cfgplugins.DHCPRelayConfigParams{
		Interface:         fmt.Sprintf("%s.%d", tc.dutPorts[0].Name(), attrs.ID),
		IPv4HelperAddress: ateP3.IPv4,
		IPv6HelperAddress: ateP3.IPv6,
	}
	cfgplugins.DhcpRelayConfig(t, tc.dut, b, dhcpRelayConfigParams)
}

// configureDHCPRelay dispatches to the appropriate interface setup based on isLag.
func (tc *testCase) configureDHCPRelay(t *testing.T, attrs *attrs.Attributes) {
	t.Helper()
	if tc.isLag {
		tc.configureLAGInterface(t, attrs)
	} else {
		tc.configureEthernetInterface(t, attrs)
	}
}

// configureDUTPort2 configures the second DUT port with the given attributes for connectivity to the DHCP server.
func (tc *testCase) configureDUTPort2(t *testing.T, attrs *attrs.Attributes) {
	t.Helper()
	d := gnmi.OC()
	i := attrs.NewOCInterface(tc.dutPorts[2].Name(), tc.dut)
	i.Description = ygot.String(attrs.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(tc.dut) {
		i.Enabled = ygot.Bool(true)
	}

	i.GetOrCreateEthernet()
	i4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	if deviations.InterfaceEnabled(tc.dut) {
		i4.Enabled = ygot.Bool(true)
	}
	a := i4.GetOrCreateAddress(attrs.IPv4)
	a.PrefixLength = ygot.Uint8(attrs.IPv4Len)

	i6 := i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
	if deviations.InterfaceEnabled(tc.dut) {
		i6.Enabled = ygot.Bool(true)
	}

	a6 := i6.GetOrCreateAddress(attrs.IPv6)
	a6.PrefixLength = ygot.Uint8(attrs.IPv6Len)

	gnmi.Replace(t, tc.dut, d.Interface(tc.dutPorts[2].Name()).Config(), i)
}

// configureDUT configures the DUT with the necessary interface and DHCP relay settings for the test.
func (tc *testCase) configureDUT(t *testing.T) {
	t.Helper()
	t.Log("Configure DUT")
	tc.configureDHCPRelay(t, &dutP1)
	tc.configureDUTPort2(t, &dutP3)
}

// deleteInterfaceConfig removes the DHCP relay configuration from the DUT interface after the test is complete.
func (tc *testCase) deleteInterfaceConfig(t *testing.T) {
	t.Helper()
	t.Log("Delete Interface Config in DUT")
	if tc.isLag {
		var dutAggPorts []*ondatra.Port
		for _, port := range tc.dutPorts[0:2] {
			dutAggPorts = append(dutAggPorts, tc.dut.Port(t, port.ID()))
		}
		if deviations.AggregateAtomicUpdate(tc.dut) {
			cfgplugins.DeleteAggregate(t, tc.dut, tc.aggID, dutAggPorts)
		}
	} else {
		gnmi.Delete(t, tc.dut, gnmi.OC().Interface(tc.dutPorts[0].Name()).Config())
	}
}

// verifyDHCPv4Address verifies that the DHCPv4 client received the expected lease address.
func (tc *testCase) verifyDHCPv4Address(t *testing.T) {
	t.Helper()
	t.Log(tc.name + ": Verifying DHCPv4 address lease")
	_, ok := gnmi.WatchAll(t, tc.ate.OTG(), gnmi.OTG().Dhcpv4ClientAny().Interface().Address().State(), time.Minute, func(v *ygnmi.Value[string]) bool {
		dhcpV4ClientAddress, present := v.Val()
		if !present {
			return false
		}
		return dhcpV4ClientAddress == dhcpLeaseStartAddress
	}).Await(t)
	if !ok {
		t.Fatalf("Did not receive expected DHCPv4 address lease %s", dhcpLeaseStartAddress)
	} else {
		t.Logf("Received expected DHCPv4 address lease: %s", dhcpLeaseStartAddress)
	}
}

// verifyDHCPv4Gateway verifies that the DHCPv4 client received the expected gateway address.
func (tc *testCase) verifyDHCPv4Gateway(t *testing.T) {
	t.Helper()
	t.Log(tc.name + ": Verifying DHCPv4 gateway address")
	_, ok := gnmi.WatchAll(t, tc.ate.OTG(), gnmi.OTG().Dhcpv4ClientAny().Interface().GatewayAddress().State(), time.Minute, func(v *ygnmi.Value[string]) bool {
		dhcpV4ClientGateway, present := v.Val()
		if !present {
			return false
		}
		return dhcpV4ClientGateway == dhcpLeaseGateway
	}).Await(t)
	if !ok {
		t.Fatalf("Did not receive expected DHCPv4 gateway address %s", dhcpLeaseGateway)
	} else {
		t.Logf("Received expected DHCPv4 gateway address: %s", dhcpLeaseGateway)
	}
}

// verifyDHCPv6Address verifies that the DHCPv6 client received the expected lease address.
func (tc *testCase) verifyDHCPv6Address(t *testing.T) {
	t.Helper()
	t.Log(tc.name + ": Verifying DHCPv6 address lease")
	_, ok := gnmi.WatchAll(t, tc.ate.OTG(), gnmi.OTG().Dhcpv6ClientAny().Interface().IaAddressAny().State(), time.Minute, func(v *ygnmi.Value[*otgtelemetry.Dhcpv6Client_Interface_IaAddress]) bool {
		dhcpV6ClientAddress, present := v.Val()
		if !present {
			return false
		}
		return dhcpV6ClientAddress.GetAddress() == dhcpV6LeaseStartAddress
	}).Await(t)
	if !ok {
		t.Fatalf("Did not receive expected DHCPv6 address lease %s", dhcpV6LeaseStartAddress)
	} else {
		t.Logf("Received expected DHCPv6 address lease: %s", dhcpV6LeaseStartAddress)
	}
}

// getDUTLinkLocalAddress retrieves the link-local address from the DUT interface to verify against the DHCPv6 gateway received by the client.
func (tc *testCase) getDUTLinkLocalAddress(t *testing.T) string {
	t.Helper()

	intfName := tc.dutPorts[0].Name()

	if tc.isLag {
		intfName = tc.aggID
	}

	addrs := gnmi.GetAll(t, tc.dut, gnmi.OC().Interface(intfName).Subinterface(dutP1.ID).Ipv6().AddressAny().State())

	for _, addr := range addrs {
		ip := addr.GetIp()
		if strings.HasPrefix(ip, "fe80:") {
			return ip
		}
	}

	t.Fatalf("No link-local address found on interface %s subinterface %d", intfName, dutP1.ID)
	return ""
}

// verifyDHCPv6Gateway verifies that the DHCPv6 client received the expected gateway address.
func (tc *testCase) verifyDHCPv6Gateway(t *testing.T) {
	t.Helper()
	t.Log(tc.name + ": Verifying DHCPv6 gateway address")
	dutLinkLocal := tc.getDUTLinkLocalAddress(t)

	_, ok := gnmi.WatchAll(t, tc.ate.OTG(), gnmi.OTG().Dhcpv6ClientAny().Interface().IaAddressAny().State(), time.Minute, func(v *ygnmi.Value[*otgtelemetry.Dhcpv6Client_Interface_IaAddress]) bool {
		dhcpV6ClientGateway, present := v.Val()
		if !present {
			return false
		}
		return dhcpV6ClientGateway.GetGateway() == dutLinkLocal
	}).Await(t)
	if !ok {
		t.Fatalf("Did not receive expected DHCPv6 gateway address %s", dutLinkLocal)
	} else {
		t.Logf("Received expected DHCPv6 gateway address: %s", dutLinkLocal)
	}
}

// verifyDHCPDiscovery orchestrates all DHCP verification checks.
func (tc *testCase) verifyDHCPDiscovery(t *testing.T) {
	t.Helper()
	tc.verifyDHCPv4Address(t)
	tc.verifyDHCPv4Gateway(t)
	tc.verifyDHCPv6Address(t)
	tc.verifyDHCPv6Gateway(t)
}

// run executes the full test lifecycle for a single test case.
func (tc *testCase) run(t *testing.T) {
	t.Helper()
	tc.configureDUT(t)
	tc.configureATE(t)
	defer tc.deleteInterfaceConfig(t)
	tc.ate.OTG().PushConfig(t, tc.conf)
	tc.ate.OTG().StartProtocols(t)
	tc.verifyDHCPDiscovery(t)
}

func TestDHCPRelayIndividualPort(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	aggID := netutil.NextAggregateInterface(t, dut)
	tcs := []*testCase{
		{
			name:     "dhcp request on an individual port",
			dut:      dut,
			ate:      ate,
			conf:     gosnappi.NewConfig(),
			dutPorts: sortPorts(dut.Ports()),
			atePorts: sortPorts(ate.Ports()),
			isLag:    false,
		},
		{
			name:     "dhcp request on an lag port",
			dut:      dut,
			ate:      ate,
			conf:     gosnappi.NewConfig(),
			dutPorts: sortPorts(dut.Ports()),
			atePorts: sortPorts(ate.Ports()),
			isLag:    true,
			lagType:  lagTypeLACP,
			aggID:    aggID,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, tc.run)
	}
}
