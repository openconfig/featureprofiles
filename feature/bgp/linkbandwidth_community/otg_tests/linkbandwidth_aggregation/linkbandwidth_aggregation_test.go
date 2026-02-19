// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package linkbandwidthaggregation implements RT-7.6.
package linkbandwidthaggregation

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	v4VlanPlen                                 = uint8(16)
	v6VlanPlen                                 = uint8(64)
	dutAS                                      = 65000
	ateAS1                                     = 65003
	ateAS2                                     = 65001
	ateAS1V4                                   = 65003
	ateAS2V4                                   = 65001
	ateAS1V6                                   = 65103
	ateAS2V6                                   = 65101
	dutP1IPv4                                  = "101.1.1.1"
	dutP1IPv6                                  = "1000::101:1:1:1"
	ateP1IPv4                                  = "101.1.2.1"
	ateP1IPv6                                  = "1000::101:1:2:1"
	ateP2IPv4                                  = "102.1.2.2"
	ateP2IPv6                                  = "1000::102:1:2:2"
	peerv41GrpName                             = "UPSTREAM-V41"
	peerv61GrpName                             = "UPSTREAM-V61"
	peerv42GrpName                             = "DOWNSTREAM-V42"
	peerv62GrpName                             = "DOWNSTREAM-V62"
	importPolicy                               = "BGP-IN"
	exportPolicy                               = "BGP-OUT"
	importStatements                           = "term1"
	exportStatements                           = "term2"
	testPrefixV4Add                            = "200.1.0.1"
	testPrefixV6Add                            = "2001::200:1:0:1"
	numberOfPrefixes                           = 1
	mbps10                                     = "00004b189680"
	mbps20                                     = "00004b989680"
	mbps40                                     = "00004c189680"
	mbps80                                     = "00004c989680"
	extComSubType                              = "4"
	comBdWthRE                                 = `Communities: bandwidth:0:(\d+)`
	totalCumulativeLBwAdvertised               = 1600000000
	after32PeerDisabledCumulativeLBwAdvertised = 1280000000
	disablePeers                               = 32
	enablePeers                                = 32
	accept                                     = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
)

var (
	dutPort1 = attributes{
		Attributes: &attrs.Attributes{
			Name:    "port1",
			IPv4:    "101.1.1.1",
			IPv4Len: v4VlanPlen,
			IPv6:    "1000::101:1:1:1",
			IPv6Len: v6VlanPlen,
		},
		numSubIntf: 64,
		ip4:        dutPort1IPv4,
		ip6:        dutPort1IPv6,
	}
	dutPort2 = attributes{
		Attributes: &attrs.Attributes{
			Name:    "port2",
			IPv4:    "102.1.1.1",
			IPv4Len: v4VlanPlen,
			IPv6:    "1000::102:1:1:1",
			IPv6Len: v6VlanPlen,
		},
		ip4: func(_ uint8) (string, string) {
			return "102.1.1.1", ""
		},
		ip6: func(_ uint8) (string, string) {
			return "1000::102:1:1:1", ""
		},
		numSubIntf: 1,
	}

	atePort1 = attributes{
		Attributes: &attrs.Attributes{
			Name:    "port1",
			MAC:     "02:00:02:01:01:01",
			IPv4:    "101.1.2.1",
			IPv4Len: v4VlanPlen,
			IPv6:    "1000::101:1:2:1",
			IPv6Len: v6VlanPlen,
		},
		numSubIntf: 64,
		ip4:        atePort1IPv4,
		gateway:    dutPort1IPv4,
		ip6:        atePort1IPv6,
		gateway6:   dutPort1IPv6,
	}
	atePort2 = attributes{
		Attributes: &attrs.Attributes{
			Name:    "port2",
			MAC:     "02:00:04:01:01:01",
			IPv4:    "102.1.2.1",
			IPv4Len: v4VlanPlen,
			IPv6:    "1000::102:1:2:1",
			IPv6Len: v6VlanPlen,
		},
		ip4: func(_ uint8) (string, string) {
			return "102.1.2.1", ""
		},
		ip6: func(_ uint8) (string, string) {
			return "1000::102:1:2:1", ""
		},
		numSubIntf: 1,
		gateway: func(_ uint8) (string, string) {
			return "102.1.1.1", ""
		},
		gateway6: func(_ uint8) (string, string) {
			return "1000::102:1:1:1", ""
		},
	}
)

type attributes struct {
	*attrs.Attributes
	numSubIntf uint32
	ip4        func(vlan uint8) (string, string)
	ip6        func(vlan uint8) (string, string)
	gateway    func(vlan uint8) (string, string)
	gateway6   func(vlan uint8) (string, string)
}

// dutPort1IPv4 returns ipv4 addresses for every vlanID.
func dutPort1IPv4(vlan uint8) (string, string) {
	ip, err := cfgplugins.IncrementIP(dutP1IPv4, int(vlan))
	return ip, err
}

// dutPort1IPv6 returns ip6 addresses for every vlanID.
func dutPort1IPv6(vlan uint8) (string, string) {
	ip, err := cfgplugins.IncrementIP(dutP1IPv6, int(vlan))
	return ip, err
}

// atePort1IPv4 returns ip4 addresses for every vlanID.
func atePort1IPv4(vlan uint8) (string, string) {
	ip, err := cfgplugins.IncrementIP(ateP1IPv4, int(vlan))
	return ip, err
}

// atePort1IPv6 returns ip6 addresses for every vlanID.
func atePort1IPv6(vlan uint8) (string, string) {
	ip, err := cfgplugins.IncrementIP(ateP1IPv6, int(vlan))
	return ip, err
}

// cidr takes as input the IPv4 address and the mask and returns the IP string in
// CIDR notation.
func cidr(ipv4 string, ones int) string {
	return ipv4 + "/" + strconv.Itoa(ones)
}

// TestMain sets up the test environment.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestLinkBandwidthExtendedCommunityCumulative tests the link bandwidth extended community cumulative feature.
func TestLinkBandwidthExtendedCommunityCumulative(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	top := gosnappi.NewConfig()
	for _, atePort := range []attributes{atePort1, atePort2} {
		atePort.configureATE(t, top, ate)
	}
	time.Sleep(30 * time.Second)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	configureDUT(t, dut)
	configureRoutePolicy(t, dut)

	var eBgpConfigList []*cfgplugins.EBgpConfigScale

	eBgpConfig1 := &cfgplugins.EBgpConfigScale{
		AteASV4:       ateAS1V4,
		AteASV6:       ateAS1V6,
		AtePortIPV4:   atePort1.IPv4,
		AtePortIPV6:   atePort1.IPv6,
		PeerV4GrpName: peerv41GrpName,
		PeerV6GrpName: peerv61GrpName,
		NumOfPeers:    atePort1.numSubIntf,
		PortName:      atePort1.Name,
	}

	eBgpConfig2 := &cfgplugins.EBgpConfigScale{
		AteASV4:       ateAS2V4,
		AteASV6:       ateAS2V6,
		AtePortIPV4:   atePort2.IPv4,
		AtePortIPV6:   atePort2.IPv6,
		PeerV4GrpName: peerv42GrpName,
		PeerV6GrpName: peerv62GrpName,
		NumOfPeers:    atePort2.numSubIntf,
		PortName:      atePort2.Name,
	}

	eBgpConfigList = append(eBgpConfigList, eBgpConfig1)
	eBgpConfigList = append(eBgpConfigList, eBgpConfig2)

	t.Run("configureBGP", func(t *testing.T) {
		top, dutConf := cfgplugins.ConfigureEBgpPeersScale(t, dut, ate, top, eBgpConfigList)
		dutConf = configureBgpPolicy(t, dutConf, dut)
		dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
		gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
		fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))
		advertiseRoutesWithEBGPExtCommunitys(t, top)
		ate.OTG().PushConfig(t, top)
		ate.OTG().StartProtocols(t)
	})

	testCases := []struct {
		name                string
		expectLBwAdvertised uint64
		enablePeers         bool
		applyConfig         func(t *testing.T, dut *ondatra.DUTDevice, enablePeers bool)
		validate            func(t *testing.T, dut *ondatra.DUTDevice, totalLBwAdvertised uint64)
	}{
		{
			name:                "RT-7.6.1: Verify LBW cumulative to eBGP peer",
			applyConfig:         nil,
			expectLBwAdvertised: totalCumulativeLBwAdvertised,
			validate:            validateCumulativeLBwCommunityAdvertisedByDUT,
		},
		{
			name:                "RT-7.6.2 eBGP: Verify LBW changes: Disable 32 peers advertising 10Mpbs bandwidth community.",
			enablePeers:         false,
			applyConfig:         disableEnableBgpWithNbr,
			expectLBwAdvertised: after32PeerDisabledCumulativeLBwAdvertised,
			validate:            validateCumulativeLBwCommunityAdvertisedByDUT,
		},
		{
			name:                "RT-7.6.2 eBGP: Verify LBW changes: Re-Enable 32 peers advertising 10Mpbs bandwidth community.",
			enablePeers:         true,
			applyConfig:         disableEnableBgpWithNbr,
			expectLBwAdvertised: totalCumulativeLBwAdvertised,
			validate:            validateCumulativeLBwCommunityAdvertisedByDUT,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.applyConfig != nil {
				tc.applyConfig(t, dut, tc.enablePeers)
			}
			time.Sleep(30 * time.Second)
			if !deviations.BgpExtendedCommunityIndexUnsupported(dut) {
				if !deviations.BGPRibOcPathUnsupported(dut) {
					tc.validate(t, dut, tc.expectLBwAdvertised)
				}
			}
		})
	}
	ate.OTG().StopProtocols(t)
}

func validateCumulativeLBwCommunityAdvertisedByDUT(t *testing.T, dut *ondatra.DUTDevice, expectCumulativeLBwAdvertised uint64) {
	dni := deviations.DefaultNetworkInstance(dut)
	bgpRIBPath := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Rib()
	locRib := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Rib_AfiSafi_Ipv4Unicast_LocRib](t, dut, bgpRIBPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().LocRib().State())
	t.Logf("RIB: %v", locRib)
	for route, prefix := range locRib.Route {
		if prefix.GetPrefix() != testPrefixV4Add {
			continue
		}
		t.Logf("%v", expectCumulativeLBwAdvertised)
		t.Logf("Found Route(prefix %s, origin: %v, pathid: %d) => %s", route.Prefix, route.Origin, route.PathId, prefix.GetPrefix())
		if prefix.ExtCommunityIndex == nil {
			t.Fatalf("No V4 community index found")
		}
		extCommunity := bgpRIBPath.ExtCommunity(prefix.GetExtCommunityIndex()).ExtCommunity().State()
		if extCommunity == nil {
			t.Fatalf("No V4 community found at given index: %v", prefix.GetExtCommunityIndex())
		}
	}

	bgpRIBPathV6 := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Rib()
	locRibv6 := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Rib_AfiSafi_Ipv6Unicast_LocRib](t, dut, bgpRIBPathV6.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Ipv6Unicast().LocRib().State())
	t.Logf("RIB: %v", locRibv6)
	for route, prefix := range locRibv6.Route {
		if prefix.GetPrefix() != testPrefixV6Add {
			continue
		}
		t.Logf("%v", expectCumulativeLBwAdvertised)
		t.Logf("Found Route(prefix %s, origin: %v, pathid: %d) => %s", route.Prefix, route.Origin, route.PathId, prefix.GetPrefix())
		if prefix.ExtCommunityIndex == nil {
			t.Fatalf("No V6 community index found")
		}
		extCommunity := bgpRIBPathV6.ExtCommunity(prefix.GetExtCommunityIndex()).ExtCommunity().State()
		if extCommunity == nil {
			t.Fatalf("No V6 community found at given index: %v", prefix.GetExtCommunityIndex())
		}
	}
}

func configureLinkBandwidth(t *testing.T, dut *ondatra.DUTDevice) {
	if deviations.AutoLinkBandwidthUnsupported(dut) {
		switch dut.Vendor() {
		default:
			t.Fatalf("Vendor %s, has no CLI configuration for auto link bandwidth", dut.Vendor())
		}
	} else {
		t.Fatalf("Vendor %s, has no OC support for auto link bandwidth", dut.Vendor())
	}
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	for _, dutPort := range []attributes{dutPort1, dutPort2} {
		dutPort.configInterfaceDUT(t, dut)
		dutPort.assignSubifsToDefaultNetworkInstance(t, dut)
	}
}

func (a *attributes) configInterfaceDUT(t *testing.T, d *ondatra.DUTDevice) {
	t.Helper()
	p := d.Port(t, a.Name)
	i := &oc.Interface{Name: ygot.String(p.Name())}

	if a.numSubIntf > 1 {
		i.Description = ygot.String(a.Desc)
		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		if deviations.InterfaceEnabled(d) {
			i.Enabled = ygot.Bool(true)
		}
	} else {
		i = a.NewOCInterface(p.Name(), d)
	}

	if deviations.ExplicitPortSpeed(d) {
		i.GetOrCreateEthernet().PortSpeed = fptest.GetIfSpeed(t, p)
	}

	if deviations.RequireRoutedSubinterface0(d) && a.numSubIntf == 1 {
		s4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
		s4.Enabled = ygot.Bool(true)
		s6 := i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
		s6.Enabled = ygot.Bool(true)
	}

	a.configSubinterfaceDUT(t, i, d)
	intfPath := gnmi.OC().Interface(p.Name())
	gnmi.Update(t, d, intfPath.Config(), i)
	fptest.LogQuery(t, "DUT", intfPath.Config(), gnmi.Get(t, d, intfPath.Config()))
}

func (a *attributes) configSubinterfaceDUT(t *testing.T, intf *oc.Interface, dut *ondatra.DUTDevice) {
	t.Helper()

	if a.numSubIntf == 1 {
		return
	}

	for i := uint32(1); i <= a.numSubIntf; i++ {
		s := intf.GetOrCreateSubinterface(i)
		if deviations.InterfaceEnabled(dut) {
			s.Enabled = ygot.Bool(true)
		}
		if deviations.DeprecatedVlanID(dut) {
			s.GetOrCreateVlan().VlanId = oc.UnionUint16(i)
		} else {
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(uint16(i))
		}

		s4 := s.GetOrCreateIpv4()
		if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
			s4.Enabled = ygot.Bool(true)
		}

		if a.Name == "port1" {
			ip, eMsg := a.ip4(uint8(i))
			if eMsg != "" {
				t.Fatalf("Failed to generate IPv4 address with error '%s'", eMsg)
			}
			s4a := s4.GetOrCreateAddress(ip)
			s4a.PrefixLength = ygot.Uint8(v4VlanPlen)
			t.Logf("Adding DUT Subinterface with ID: %d, Vlan ID: %d and IPv4 address: %s", i, i, ip)

			ip6, eMsg := a.ip6(uint8(i))
			if eMsg != "" {
				t.Fatalf("Failed to generate IPv6 address with error '%s'", eMsg)
			}
			s6 := s.GetOrCreateIpv6()
			if deviations.InterfaceEnabled(dut) {
				s6.Enabled = ygot.Bool(true)
			}
			s6a := s6.GetOrCreateAddress(ip6)
			s6a.PrefixLength = ygot.Uint8(v6VlanPlen)
			t.Logf("Adding DUT Subinterface with ID: %d, Vlan ID: %d and IPv6 address: %s", i, i, ip6)
		}
	}
}

func (a *attributes) assignSubifsToDefaultNetworkInstance(t *testing.T, d *ondatra.DUTDevice) {
	p := d.Port(t, a.Name)
	if deviations.ExplicitInterfaceInDefaultVRF(d) {
		if a.numSubIntf == 1 {
			fptest.AssignToNetworkInstance(t, d, p.Name(), deviations.DefaultNetworkInstance(d), 0)
		} else {
			for i := uint32(1); i <= a.numSubIntf; i++ {
				fptest.AssignToNetworkInstance(t, d, p.Name(), deviations.DefaultNetworkInstance(d), i)
			}
		}
	}
}

func (a *attributes) configureATE(t *testing.T, top gosnappi.Config, ate *ondatra.ATEDevice) {
	t.Helper()
	p := ate.Port(t, a.Name)

	top.Ports().Add().SetName(p.ID())
	if a.numSubIntf == 1 {
		gateway, eMsg := a.gateway(1)
		if eMsg != "" {
			t.Fatalf("Failed to generate gateway address with error '%s'", eMsg)
		}
		gateway6, eMsg := a.gateway6(1)
		if eMsg != "" {
			t.Fatalf("Failed to generate gateway6 address with error '%s'", eMsg)
		}
		dev := top.Devices().Add().SetName(a.Name)
		eth := dev.Ethernets().Add().SetName(a.Name + ".Eth").SetMac(a.MAC)
		eth.Connection().SetPortName(p.ID())
		ipObj4 := eth.Ipv4Addresses().Add().SetName(dev.Name() + ".IPv4")
		ipObj4.SetAddress(ateP2IPv4).SetGateway(gateway).SetPrefix(uint32(a.IPv4Len))
		t.Logf("Adding ATE Ipv4 address: %s with gateway: %s", cidr(a.IPv4, int(a.IPv4Len)), gateway)
		ipObj6 := eth.Ipv6Addresses().Add().SetName(dev.Name() + ".IPv6")
		ipObj6.SetAddress(ateP2IPv6).SetGateway(gateway6).SetPrefix(uint32(a.IPv6Len))
		t.Logf("Adding ATE Ipv6 address: %s with gateway: %s", cidr(a.IPv6, int(a.IPv6Len)), gateway)
		return
	}

	for i := uint32(1); i <= a.numSubIntf; i++ {
		name := fmt.Sprintf(`%sdst%d`, a.Name, i)

		gateway, eMsg := a.gateway(uint8(i))
		if eMsg != "" {
			t.Fatalf("Failed to generate gateway address with error '%s'", eMsg)
		}
		gateway6, eMsg := a.gateway6(uint8(i))
		if eMsg != "" {
			t.Fatalf("Failed to generate gateway6 address with error '%s'", eMsg)
		}

		mac, err := incrementMAC(a.MAC, int(i)+1)
		if err != nil {
			t.Fatalf("Failed to generate mac address with error %s", err)
		}

		dev := top.Devices().Add().SetName(name + ".Dev")
		eth := dev.Ethernets().Add().SetName(name + ".Eth").SetMac(mac)
		eth.Connection().SetPortName(p.ID())
		eth.Vlans().Add().SetName(name).SetId(uint32(i))
		if a.Name == "port1" {
			ip, eMsg := a.ip4(uint8(i))
			if eMsg != "" {
				t.Fatalf("Failed to generate IPv4 address with error '%s'", eMsg)
			}
			eth.Ipv4Addresses().Add().SetName(name + ".IPv4").SetAddress(ip).SetGateway(gateway).SetPrefix(uint32(v4VlanPlen))
			t.Logf("Adding ATE Ipv4 address: %s with gateway: %s and VlanID: %d", cidr(ip, int(v4VlanPlen)), gateway, i)
			ip6, eMsg := a.ip6(uint8(i))
			if eMsg != "" {
				t.Fatalf("Failed to generate IPv6 address with error '%s'", eMsg)
			}
			eth.Ipv6Addresses().Add().SetName(name + ".IPv6").SetAddress(ip6).SetGateway(gateway6).SetPrefix(uint32(v6VlanPlen))
			t.Logf("Adding ATE Ipv6 address: %s with gateway: %s and VlanID: %d", cidr(ip6, int(v6VlanPlen)), gateway6, i)
		}
	}
}

func advertiseRoutesWithEBGPExtCommunitys(t *testing.T, top gosnappi.Config) {
	devices := top.Devices().Items()
	prefixesV4 := createPrefixesV4(t, numberOfPrefixes)
	for idx, device := range devices {
		if strings.Contains(device.Name(), "port1") {
			bgp4Peer := device.Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0]
			netv4 := bgp4Peer.V4Routes().Add().SetName(fmt.Sprintf("v4-bgpNet-%s", device.Name()))
			for _, prefix := range prefixesV4 {
				if idx <= 31 {
					netv4.ExtendedCommunities().Add().Custom().SetCommunitySubtype(extComSubType).SetValue(mbps10)
				} else if idx <= 47 {
					netv4.ExtendedCommunities().Add().Custom().SetCommunitySubtype(extComSubType).SetValue(mbps20)
				} else if idx <= 55 {
					netv4.ExtendedCommunities().Add().Custom().SetCommunitySubtype(extComSubType).SetValue(mbps40)
				} else if idx <= 63 {
					netv4.ExtendedCommunities().Add().Custom().SetCommunitySubtype(extComSubType).SetValue(mbps80)
				}
				netv4.Addresses().Add().SetAddress(prefix.Addr().String()).SetPrefix(uint32(prefix.Bits()))
				netv4.AddPath().SetPathId(uint32(1))
			}
		}
	}

	prefixesV6 := createPrefixesV6(t, numberOfPrefixes)
	for idx, device := range devices {
		if strings.Contains(device.Name(), "port1") {
			bgp6Peer := device.Bgp().Ipv6Interfaces().Items()[0].Peers().Items()[0]
			netv6 := bgp6Peer.V6Routes().Add().SetName(fmt.Sprintf("v6-bgpNet-%s", device.Name()))
			for _, prefix := range prefixesV6 {
				if idx <= 31 {
					netv6.ExtendedCommunities().Add().Custom().SetCommunitySubtype(extComSubType).SetValue(mbps10)
				} else if idx <= 47 {
					netv6.ExtendedCommunities().Add().Custom().SetCommunitySubtype(extComSubType).SetValue(mbps20)
				} else if idx <= 55 {
					netv6.ExtendedCommunities().Add().Custom().SetCommunitySubtype(extComSubType).SetValue(mbps40)
				} else if idx <= 63 {
					netv6.ExtendedCommunities().Add().Custom().SetCommunitySubtype(extComSubType).SetValue(mbps80)
				}
				netv6.Addresses().Add().SetAddress(prefix.Addr().String()).SetPrefix(uint32(prefix.Bits()))
				netv6.AddPath().SetPathId(uint32(1))
			}
		}
	}
}

func createPrefixesV4(t *testing.T, numberOfPrefixes uint32) []netip.Prefix {
	t.Helper()

	var ips []netip.Prefix
	v4VlanPlenStr := strconv.FormatUint(uint64(32), 10)
	testPrefixV4 := testPrefixV4Add + "/" + v4VlanPlenStr
	for i := uint32(0); i < numberOfPrefixes; i++ {
		ips = append(ips, netip.MustParsePrefix(fmt.Sprint(testPrefixV4)))
	}

	return ips
}

func createPrefixesV6(t *testing.T, numberOfPrefixes uint32) []netip.Prefix {
	t.Helper()
	var ips []netip.Prefix

	v6VlanPlenStr := strconv.FormatUint(uint64(128), 10)
	testPrefixV6 := testPrefixV6Add + "/" + v6VlanPlenStr
	for i := uint32(0); i < numberOfPrefixes; i++ {
		ip := netip.MustParsePrefix(fmt.Sprint(testPrefixV6))
		ips = append(ips, ip)
	}

	return ips
}

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

func configureRoutePolicy(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()

	pd1 := rp.GetOrCreatePolicyDefinition(importPolicy)
	st1, err1 := pd1.AppendNewStatement(importStatements)
	if err1 != nil {
		t.Fatal(err1)
	}
	st1.GetOrCreateConditions().GetOrCreateBgpConditions().SetRouteType(oc.BgpConditions_RouteType_EXTERNAL)
	st1.GetOrCreateActions().PolicyResult = accept

	pd2 := rp.GetOrCreatePolicyDefinition(exportPolicy)
	st2, err2 := pd2.AppendNewStatement(exportStatements)
	if err2 != nil {
		t.Fatal(err2)
	}
	st2.GetOrCreateConditions().GetOrCreateBgpConditions().SetRouteType(oc.BgpConditions_RouteType_EXTERNAL)
	st2.GetOrCreateActions().PolicyResult = accept
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

func disableEnableBgpWithNbr(t *testing.T, dut *ondatra.DUTDevice, enablePeers bool) {
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	cfg := &cfgplugins.EBgpConfigScale{
		AteASV4:       ateAS1V4,
		AteASV6:       ateAS1V6,
		AtePortIPV4:   atePort1.IPv4,
		AtePortIPV6:   atePort1.IPv6,
		PeerV4GrpName: peerv41GrpName,
		PeerV6GrpName: peerv61GrpName,
		NumOfPeers:    uint32(disablePeers),
		PortName:      atePort1.Name,
	}

	nbrs := cfgplugins.BuildIPv4v6NbrScale(t, cfg)

	for _, nbr := range nbrs {
		bgpNbr := bgp.GetOrCreateNeighbor(nbr.Neighborip)
		bgpNbr.PeerAs = ygot.Uint32(nbr.As)
		if enablePeers {
			bgpNbr.Enabled = ygot.Bool(true)
		} else {
			bgpNbr.Enabled = ygot.Bool(false)
		}
		bgpNbr.PeerGroup = ygot.String(nbr.Pg)

		if deviations.AutoLinkBandwidthUnsupported(dut) {
			configureLinkBandwidth(t, dut)
		} else {
			bgpNbr.GetOrCreateAutoLinkBandwidth().GetOrCreateImport().SetEnabled(true)
			bgpNbr.GetOrCreateAutoLinkBandwidth().GetOrCreateImport().SetTransitive(true)
			bgpNbr.GetOrCreateUseMultiplePaths().SetEnabled(true)
			bgpNbr.GetOrCreateUseMultiplePaths().GetOrCreateEbgp().SetAllowMultipleAs(true)
		}

		bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).SetSendCommunityType([]oc.E_Bgp_CommunityType{oc.Bgp_CommunityType_STANDARD, oc.Bgp_CommunityType_EXTENDED})
		bgpNbr.GetOrCreateApplyPolicy().DefaultExportPolicy = oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE
		bgpNbr.GetOrCreateApplyPolicy().DefaultImportPolicy = oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE
	}

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	gnmi.Update(t, dut, dutConfPath.Config(), niProto)
}

func configureBgpPolicy(t *testing.T, dutConf *oc.NetworkInstance_Protocol,
	dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	bgp := dutConf.GetOrCreateBgp()
	g := bgp.GetOrCreateGlobal()
	g.As = ygot.Uint32(dutAS)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	g.RouterId = ygot.String(dutPort1.IPv4)
	pgv41 := bgp.GetOrCreatePeerGroup(peerv41GrpName)
	pgv61 := bgp.GetOrCreatePeerGroup(peerv61GrpName)
	pgv42 := bgp.GetOrCreatePeerGroup(peerv42GrpName)
	pgv62 := bgp.GetOrCreatePeerGroup(peerv62GrpName)
	pgsPort1 := []*oc.NetworkInstance_Protocol_Bgp_PeerGroup{pgv41, pgv61}
	pgsPort2 := []*oc.NetworkInstance_Protocol_Bgp_PeerGroup{pgv42, pgv62}
	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		for _, pg := range pgsPort1 {
			rpl := pg.GetOrCreateApplyPolicy()
			rpl.SetImportPolicy([]string{importPolicy})
		}
		for _, pg := range pgsPort2 {
			rpl := pg.GetOrCreateApplyPolicy()
			rpl.SetExportPolicy([]string{exportPolicy})
		}
	} else {
		// port1
		pgv41.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy().SetImportPolicy([]string{importPolicy})
		pgv61.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).GetOrCreateApplyPolicy().SetImportPolicy([]string{importPolicy})
		// port2
		pgv42.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy().SetExportPolicy([]string{exportPolicy})
		pgv62.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).GetOrCreateApplyPolicy().SetExportPolicy([]string{exportPolicy})
	}
	return dutConf
}

// #TODO: Add Test for RT-7.6.3: Verify LBW cumulative to iBGP peer (currently unsupported on Juniper b/434631360)
