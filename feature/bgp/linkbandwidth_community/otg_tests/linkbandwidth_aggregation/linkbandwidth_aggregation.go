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
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/ygnmi/ygnmi"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
)

const (
	v4VlanPlen                                 = uint8(24)
	v6VlanPlen                                 = uint8(64)
	dutAS                                      = 65000
	ateAS1                                     = 65003
	ateAS2                                     = 65001
	peerv41GrpName                             = "UPSTREAM-V41"
	peerv61GrpName                             = "UPSTREAM-V61"
	peerv42GrpName                             = "DOWNSTREAM-V42"
	peerv62GrpName                             = "DOWNSTREAM-V62"
	importPolicy                               = "BGP-IN"
	exportPolicy                               = "BGP-OUT"
	importStatements                           = "term1"
	exportStatements                           = "term2"
	testPrefixV4                               = "200.1.0.0/24"
	testPrefixV4Add                            = "200.1.0.0"
	testPrefixV6                               = "2001::200:1:0:0/126"
	testPrefixV6Add                            = "2001::200:1:0:0"
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
			IPv4:    "101.1.0.1",
			IPv4Len: v4VlanPlen,
			IPv6:    "1000::101:1:0:1",
			IPv6Len: v6VlanPlen,
		},
		numSubIntf: 64,
		ip4:        dutPort1IPv4,
		ip6:        dutPort1IPv6,
	}
	dutPort2 = attributes{
		Attributes: &attrs.Attributes{
			Name:    "port2",
			IPv4:    "102.1.0.1",
			IPv4Len: v4VlanPlen,
			IPv6:    "1000::102:1:0:1",
			IPv6Len: v6VlanPlen,
		},
		ip4: func(_ uint8) string {
			return "102.1.0.1"
		},
		ip6: func(_ uint8) string {
			return "1000::102:1:0:1"
		},
		numSubIntf: 1,
	}

	atePort1 = attributes{
		Attributes: &attrs.Attributes{
			Name:    "port1",
			MAC:     "02:00:02:01:01:01",
			IPv4:    "101.1.0.2",
			IPv4Len: v4VlanPlen,
			IPv6:    "1000::101:1:0:2",
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
			IPv4:    "102.1.0.2",
			IPv4Len: v4VlanPlen,
			IPv6:    "1000::102:1:0:2",
			IPv6Len: v6VlanPlen,
		},
		ip4: func(_ uint8) string {
			return "102.1.0.2"
		},
		ip6: func(_ uint8) string {
			return "1000::102:1:0:2"
		},
		numSubIntf: 1,
		gateway: func(_ uint8) string {
			return "102.1.0.1"
		},
		gateway6: func(_ uint8) string {
			return "1000::102:1:0:1"
		},
	}
)

type attributes struct {
	*attrs.Attributes
	numSubIntf uint32
	ip4        func(vlan uint8) string
	ip6        func(vlan uint8) string
	gateway    func(vlan uint8) string
	gateway6   func(vlan uint8) string
}

// dutPort1IPv4 returns ip addresses starting 101.1.%d.1 for every vlanID.
func dutPort1IPv4(vlan uint8) string {
	return fmt.Sprintf("101.1.%d.1", vlan)
}

// dutPort1IPv6 returns ip addresses starting 1000::101.1.%d.1 for every vlanID.
func dutPort1IPv6(vlan uint8) string {
	return fmt.Sprintf("1000::101:1:%d:1", vlan)
}

// atePort1IPv4 returns ip addresses starting 101.1.%d.2, increasing by 1
// for every vlanID.
func atePort1IPv4(vlan uint8) string {
	return fmt.Sprintf("101.1.%d.2", vlan)
}

// atePort1IPv6 returns ip addresses starting 1000::101:1:%d:2 for every vlanID.
func atePort1IPv6(vlan uint8) string {
	return fmt.Sprintf("1000::101:1:%d:2", vlan)
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
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	configureDUT(t, dut)
	configureRoutePolicy(t, dut)

	nbrList := buildNeighborList(atePort1, atePort2)
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	t.Run("configureBGP", func(t *testing.T) {
		dutConf := bgpWithNbr(dutAS, nbrList, dut, importPolicy, exportPolicy)
		gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
		fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))

		atePort1.configureBGPOnATE(t, top, ateAS1)
		atePort2.configureBGPOnATE(t, top, ateAS2)
		advertiseRoutesWithEBGPExtCommunitys(t, top)
		configureLinkBandwidth(t, dut)

		ate.OTG().PushConfig(t, top)
		ate.OTG().StartProtocols(t)
	})

	testCases := []struct {
		name                string
		expectLBwAdvertised uint64
		enablePeers         bool
		applyConfig         func(t *testing.T, dut *ondatra.DUTDevice, enablePeers bool)
		validate            func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, totalLBwAdvertised uint64)
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
			tc.validate(t, dut, ate, top, tc.expectLBwAdvertised)
		})
	}
	ate.OTG().StopProtocols(t)
}

func configureLinkBandwidth(t *testing.T, dut *ondatra.DUTDevice) {
	if deviations.AutoLinkBandwidthUnsupported(dut) {
			switch dut.Vendor() {
			case ondatra.JUNIPER:
				cliConfig := fmt.Sprintf(`protocols {
									bgp {
											group %s {
													multipath {
															multiple-as;
													}
													link-bandwidth {
															auto-sense;
													}
											}
											group %s {
													multipath {
															multiple-as;
													}
													link-bandwidth {
															auto-sense;
													}
											}
									}
							}`, peerv41GrpName, peerv61GrpName)
				helpers.GnmiCLIConfig(t, dut, cliConfig)
			default:
				t.Fatalf("Vendor %s, has no CLI configuration for auto link bandwidth", dut.Vendor())
			}
	}
}

func buildNeighborList(atePort1, atePort2 attributes) []*bgpNeighbor {
	nbrList1 := atePort1.buildIPv4v6NbrList(ateAS1, peerv41GrpName, peerv61GrpName, atePort1.numSubIntf)
	nbrList2 := atePort2.buildIPv4v6NbrList(ateAS2, peerv42GrpName, peerv62GrpName, atePort2.numSubIntf)
	nbrList := append(nbrList1, nbrList2...)
	return nbrList
}

func validateCumulativeLBwCommunityAdvertisedByDUT(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, expectCumulativeLBwAdvertised uint64) {
	_, ok := gnmi.WatchAll(t,
		ate.OTG(),
		gnmi.OTG().BgpPeer(atePort2.Name+".BGP4.peer").UnicastIpv4PrefixAny().State(),
		time.Minute,
		func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv4Prefix]) bool {
			_, present := v.Val()
			return present
		}).Await(t)
	if ok {
		bgpPrefixes := gnmi.GetAll(t, ate.OTG(), gnmi.OTG().BgpPeer(atePort2.Name+".BGP4.peer").UnicastIpv4PrefixAny().State())
		for _, bgpPrefix := range bgpPrefixes {
			if bgpPrefix.GetAddress() == testPrefixV4Add {
				if deviations.AdvertisedCumulativeLBwOCUnsupported(dut) {
					if ondatra.DUT(t, "dut").Vendor() == ondatra.JUNIPER {
						cmd := fmt.Sprintf("show route advertising-protocol bgp %s detail", atePort2.IPv4)
						runningConfig, err := dut.RawAPIs().CLI(t).RunCommand(context.Background(), cmd)
						if err != nil {
							t.Fatalf("IPV4: '%s' failed: %v", cmd, err)
						}
						re := regexp.MustCompile(comBdWthRE)
						match := re.FindStringSubmatch(runningConfig.Output())
						if len(match) < 2 {
							t.Fatalf("IPV4: cumulative bandwidth community not found in the output")
						}
						bandwidth, err := strconv.Atoi(match[1])
						if err != nil {
							t.Fatalf("IPV4: failed to convert bandwidth value to integer: %v", err)
						}
						if uint64(bandwidth) != expectCumulativeLBwAdvertised {
							t.Logf("IPV4: Advertised cumulative bandwidth cmd output: %s", runningConfig.Output())
							t.Fatalf("IPV4: Advertised cumulative bandwidth community value does not match expected "+
								"value: got %v, want %v", bandwidth, expectCumulativeLBwAdvertised)
						} else {
							t.Logf("IPV4: Advertised cumulative bandwidth cmd output: %s", runningConfig.Output())
							t.Logf("IPV4: Advertised cumulative bandwidth community value matches expected value: %v", bandwidth)
						}
					}
				}
			}
		}
	} else {
		t.Fatalf("IPV4: did not receive any prefixes from peer on ATE port2")
	}

	_, okay := gnmi.WatchAll(t,
		ate.OTG(),
		gnmi.OTG().BgpPeer(atePort2.Name+".BGP6.peer").UnicastIpv6PrefixAny().State(),
		time.Minute,
		func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv6Prefix]) bool {
			_, present := v.Val()
			return present
		}).Await(t)
	if okay {
		bgpPrefixes := gnmi.GetAll(t, ate.OTG(), gnmi.OTG().BgpPeer(atePort2.Name+".BGP6.peer").UnicastIpv6PrefixAny().State())
		for _, bgpPrefix := range bgpPrefixes {
			if bgpPrefix.GetAddress() == testPrefixV6Add {
				if deviations.AdvertisedCumulativeLBwOCUnsupported(dut) {
					if ondatra.DUT(t, "dut").Vendor() == ondatra.JUNIPER {
						cmd := fmt.Sprintf("show route advertising-protocol bgp %s detail", atePort2.IPv6)
						runningConfig, err := dut.RawAPIs().CLI(t).RunCommand(context.Background(), cmd)
						if err != nil {
							t.Fatalf("IPV6: '%s' failed: %v", cmd, err)
						}
						re := regexp.MustCompile(comBdWthRE)
						match := re.FindStringSubmatch(runningConfig.Output())
						if len(match) < 2 {
							t.Fatalf("IPV6: cumulative bandwidth community not found in the output")
						}
						bandwidth, err := strconv.Atoi(match[1])
						if err != nil {
							t.Fatalf("IPV6: failed to convert bandwidth value to integer: %v", err)
						}
						if uint64(bandwidth) != expectCumulativeLBwAdvertised {
							t.Logf("IPV6: Advertised cumulative bandwidth cmd output: %s", runningConfig.Output())
							t.Fatalf("IPV6: Advertised cumulative bandwidth community value does not match expected "+
								"value: got %v, want %v", bandwidth, expectCumulativeLBwAdvertised)
						} else {
							t.Logf("IPV6: Advertised cumulative bandwidth cmd output: %s", runningConfig.Output())
							t.Logf("IPV6: Advertised cumulative bandwidth community value matches expected value: %v", bandwidth)
						}
					}
				}
			}
		}
	} else {
		t.Fatalf("IPV6: did not receive any prefixes from peer on ATE port2")
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
			ip := a.ip4(uint8(i))
			s4a := s4.GetOrCreateAddress(ip)
			s4a.PrefixLength = ygot.Uint8(v4VlanPlen)
			t.Logf("Adding DUT Subinterface with ID: %d, Vlan ID: %d and IPv4 address: %s", i, i, ip)

			ip6 := a.ip6(uint8(i))
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
		gateway := a.gateway(1)
		gateway6 := a.gateway6(1)
		dev := top.Devices().Add().SetName(a.Name)
		eth := dev.Ethernets().Add().SetName(a.Name + ".Eth").SetMac(a.MAC)
		eth.Connection().SetPortName(p.ID())
		ipObj4 := eth.Ipv4Addresses().Add().SetName(dev.Name() + ".IPv4")
		ipObj4.SetAddress(a.IPv4).SetGateway(gateway).SetPrefix(uint32(a.IPv4Len))
		t.Logf("Adding ATE Ipv4 address: %s with gateway: %s", cidr(a.IPv4, int(a.IPv4Len)), gateway)
		ipObj6 := eth.Ipv6Addresses().Add().SetName(dev.Name() + ".IPv6")
		ipObj6.SetAddress(a.IPv6).SetGateway(gateway6).SetPrefix(uint32(a.IPv6Len))
		t.Logf("Adding ATE Ipv6 address: %s with gateway: %s", cidr(a.IPv6, int(a.IPv6Len)), gateway)
		return
	}

	for i := uint32(1); i <= a.numSubIntf; i++ {
		name := fmt.Sprintf(`%sdst%d`, a.Name, i)

		gateway := a.gateway(uint8(i))
		gateway6 := a.gateway6(uint8(i))
		mac, err := incrementMAC(a.MAC, int(i)+1)
		if err != nil {
			t.Fatalf("Failed to generate mac address with error %s", err)
		}

		dev := top.Devices().Add().SetName(name + ".Dev")
		eth := dev.Ethernets().Add().SetName(name + ".Eth").SetMac(mac)
		eth.Connection().SetPortName(p.ID())
		eth.Vlans().Add().SetName(name).SetId(uint32(i))
		if a.Name == "port1" {
			ip := a.ip4(uint8(i))
			eth.Ipv4Addresses().Add().SetName(name + ".IPv4").SetAddress(ip).SetGateway(gateway).SetPrefix(uint32(v4VlanPlen))
			t.Logf("Adding ATE Ipv4 address: %s with gateway: %s and VlanID: %d", cidr(ip, int(v4VlanPlen)), gateway, i)
			ip6 := a.ip6(uint8(i))
			eth.Ipv6Addresses().Add().SetName(name + ".IPv6").SetAddress(ip6).SetGateway(gateway6).SetPrefix(uint32(v6VlanPlen))
			t.Logf("Adding ATE Ipv6 address: %s with gateway: %s and VlanID: %d", cidr(ip6, int(v6VlanPlen)), gateway6, i)
		}
	}
}

func (a *attributes) buildIPv4v6NbrList(asn uint32, v4pg, v6pg string, numSubIntf uint32) []*bgpNeighbor {
	var nbrList []*bgpNeighbor
	for i := uint32(1); i <= numSubIntf; i++ {
		asn6 := asn + 65
		if a.ip4 != nil {
			ip := a.ip4(uint8(i))
			bgpNbr := &bgpNeighbor{
				as:         asn,
				neighborip: ip,
				isV4:       true,
				pg:         v4pg,
			}
			nbrList = append(nbrList, bgpNbr)
		}
		if a.ip6 != nil {
			ip := a.ip6(uint8(i))
			bgpNbr := &bgpNeighbor{
				as:         asn6,
				neighborip: ip,
				isV4:       false,
				pg:         v6pg,
			}
			nbrList = append(nbrList, bgpNbr)
		}
		asn = asn + 1
		asn6 = asn6 + 1
	}
	return nbrList
}

func (a *attributes) configureBGPOnATE(t *testing.T, top gosnappi.Config, asn uint32) {
	t.Helper()

	devices := top.Devices().Items()
	devMap := make(map[string]gosnappi.Device)
	for _, dev := range devices {
		devMap[dev.Name()] = dev
	}

	for i := uint32(1); i <= a.numSubIntf; i++ {
		asn6 := asn + 65
		di := a.Name
		if a.Name == "port1" {
			di = fmt.Sprintf("%sdst%d.Dev", a.Name, i)
		}
		device := devMap[di]
		if a.ip4 != nil {
			bgp := device.Bgp().SetRouterId(a.ip4(uint8(i)))
			ipv4 := device.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
			bgp4Peer := bgp.Ipv4Interfaces().Add().SetIpv4Name(ipv4.Name()).Peers().Add().SetName(device.Name() + ".BGP4.peer")
			bgp4Peer.SetPeerAddress(ipv4.Gateway())
			bgp4Peer.SetAsNumber(asn)
			bgp4Peer.SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
			bgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
			bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true)
		}
		if a.ip6 != nil {
			bgp := device.Bgp().SetRouterId(a.IPv4)
			ipv6 := device.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
			bgp6Peer := bgp.Ipv6Interfaces().Add().SetIpv6Name(ipv6.Name()).Peers().Add().SetName(device.Name() + ".BGP6.peer")
			bgp6Peer.SetPeerAddress(ipv6.Gateway())
			bgp6Peer.SetAsNumber(asn6)
			bgp6Peer.SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
			bgp6Peer.Capability().SetIpv6UnicastAddPath(true)
			bgp6Peer.LearnedInformationFilter().SetUnicastIpv6Prefix(true)
		}
		asn = asn + 1
		asn6 = asn6 + 1
	}
}

func advertiseRoutesWithEBGPExtCommunitys(t *testing.T, top gosnappi.Config) {
	devices := top.Devices().Items()
	prefixesV4 := createPrefixesV4(t)
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

	prefixesV6 := createPrefixesV6(t)
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

func createPrefixesV4(t *testing.T) []netip.Prefix {
	t.Helper()

	var ips []netip.Prefix
	// Create /24
	for i := 0; i < 1; i++ {
		ips = append(ips, netip.MustParsePrefix(fmt.Sprint(testPrefixV4)))
	}

	return ips
}

func createPrefixesV6(t *testing.T) []netip.Prefix {
	t.Helper()
	var ips []netip.Prefix

	// Create /126
	for i := 0; i < 1; i++ {
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

type bgpNeighbor struct {
	as         uint32
	neighborip string
	isV4       bool
	pg         string
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

	var cliConfig string
	if deviations.AggregateBandwidthPolicyActionUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.JUNIPER:
			cliConfig = fmt.Sprintf(`policy-options {
							policy-statement %s {
									term %s {
											then {
													aggregate-bandwidth {
															transitive;
													}
											}
									}
							}
						}`, exportPolicy, exportStatements)
			helpers.GnmiCLIConfig(t, dut, cliConfig)
		default:
			t.Fatalf("Vendor %s, has no CLI configuration for aggregate-bandwidth policy action", dut.Vendor())
		}
	}

	st2.GetOrCreateActions().PolicyResult = accept
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

func bgpWithNbr(as uint32, nbrs []*bgpNeighbor, dut *ondatra.DUTDevice, impPolicy, expPolicy string) *oc.NetworkInstance_Protocol {
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	g := bgp.GetOrCreateGlobal()
	g.As = ygot.Uint32(as)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	g.RouterId = ygot.String(dutPort1.IPv4)

	pgv41 := bgp.GetOrCreatePeerGroup(peerv41GrpName)
	pgv41.PeerAs = ygot.Uint32(ateAS1)

	pgv61 := bgp.GetOrCreatePeerGroup(peerv61GrpName)
	pgv61.PeerAs = ygot.Uint32(ateAS1)

	pgv42 := bgp.GetOrCreatePeerGroup(peerv42GrpName)
	pgv42.PeerAs = ygot.Uint32(ateAS2)

	pgv62 := bgp.GetOrCreatePeerGroup(peerv62GrpName)
	pgv62.PeerAs = ygot.Uint32(ateAS2)

	pgsPort1 := []*oc.NetworkInstance_Protocol_Bgp_PeerGroup{pgv41, pgv61}
	pgsPort2 := []*oc.NetworkInstance_Protocol_Bgp_PeerGroup{pgv42, pgv62}
	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		for _, pg := range pgsPort1 {
			rpl := pg.GetOrCreateApplyPolicy()
			rpl.SetImportPolicy([]string{impPolicy})
		}
		for _, pg := range pgsPort2 {
			rpl := pg.GetOrCreateApplyPolicy()
			rpl.SetExportPolicy([]string{expPolicy})
		}
	} else {
		// V4
		pgaf1 := pgv41.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		pgaf1.Enabled = ygot.Bool(true)
		rpl1 := pgaf1.GetOrCreateApplyPolicy()
		rpl1.SetImportPolicy([]string{impPolicy})
		pgaf2 := pgv42.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		pgaf2.Enabled = ygot.Bool(true)
		rpl2 := pgaf2.GetOrCreateApplyPolicy()
		rpl2.SetExportPolicy([]string{expPolicy})
		// V6
		pgaf3 := pgv61.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		pgaf3.Enabled = ygot.Bool(true)
		rpl3 := pgaf3.GetOrCreateApplyPolicy()
		rpl3.SetImportPolicy([]string{impPolicy})
		pgaf4 := pgv62.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		pgaf4.Enabled = ygot.Bool(true)
		rpl4 := pgaf4.GetOrCreateApplyPolicy()
		rpl4.SetExportPolicy([]string{expPolicy})
	}

	for _, nbr := range nbrs {
		bgpNbr := bgp.GetOrCreateNeighbor(nbr.neighborip)
		bgpNbr.GetOrCreateTimers().HoldTime = ygot.Uint16(180)
		bgpNbr.GetOrCreateTimers().KeepaliveInterval = ygot.Uint16(60)
		bgpNbr.GetOrCreateTimers().SetMinimumAdvertisementInterval(10)
		bgpNbr.PeerAs = ygot.Uint32(nbr.as)
		bgpNbr.Enabled = ygot.Bool(true)
		bgpNbr.PeerGroup = ygot.String(nbr.pg)
		if nbr.isV4 {
			af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(true)
			af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(false)
		} else {
			af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(true)
			af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(false)
		}
	}
	return niProto
}

func disableEnableBgpWithNbr(t *testing.T, dut *ondatra.DUTDevice, enablePeers bool) {
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	nbrs := atePort1.buildIPv4v6NbrList(ateAS1, peerv41GrpName, peerv61GrpName, disablePeers)

	for _, nbr := range nbrs {
		bgpNbr := bgp.GetOrCreateNeighbor(nbr.neighborip)
		bgpNbr.GetOrCreateTimers().HoldTime = ygot.Uint16(180)
		bgpNbr.GetOrCreateTimers().KeepaliveInterval = ygot.Uint16(60)
		bgpNbr.GetOrCreateTimers().SetMinimumAdvertisementInterval(10)
		bgpNbr.PeerAs = ygot.Uint32(nbr.as)
		if enablePeers {
			bgpNbr.Enabled = ygot.Bool(true)
		} else {
			bgpNbr.Enabled = ygot.Bool(false)
		}
		bgpNbr.PeerGroup = ygot.String(nbr.pg)
		if nbr.isV4 {
			af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(true)
			af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(false)
		} else {
			af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(true)
			af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(false)
		}
	}

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	gnmi.Update(t, dut, dutConfPath.Config(), niProto)
}

// #TODO: Add Test for RT-7.6.3: Verify LBW cumulative to iBGP peer (currently unsupported on Juniper b/434631360)

