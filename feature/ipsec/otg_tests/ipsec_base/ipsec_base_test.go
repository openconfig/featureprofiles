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
//

package ipsec_base_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	packetvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/packetvalidationhelpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
)

const (
	vlanID = 10

	// Subinterface index for base/management subinterface.
	baseSubinterfaceIndex = 0

	// ATE OTG topology names.
	ate1LagName = "Lag1"
	ate2LagName = "Lag2"
	ate1DevName = "d1"
	ate2DevName = "d2"

	// VRF names.
	ateVRF    = "ATE_VRF"
	tunnelVRF = "TUNNEL_VRF"

	// Interface names.
	tunnelIfName    = "Tunnel1"
	tunnel2IfName   = "Tunnel2"
	loopbackIfName  = "Loopback0"
	loopback2IfName = "Loopback1"

	// IKE FQDN identities.
	dut1FQDN = "dut1.test.local"
	dut2FQDN = "dut2.test.local"

	// Distinct FQDN identities for Tunnel2 to avoid IKE peer merging on Arista.
	dut1FQDN2 = "dut1-t2.test.local"
	dut2FQDN2 = "dut2-t2.test.local"

	// Loopback IPv6 addresses used as IPSec tunnel endpoints (RFC 3849).
	dut1LoopbackIPv6  = "2001:db8:3::1"
	dut2LoopbackIPv6  = "2001:db8:4::1"
	dut1Loopback2IPv6 = "2001:db8:5::1"
	dut2Loopback2IPv6 = "2001:db8:6::1"
	loopbackPrefixLen = 128

	// Tunnel interface addresses in CIDR notation (RFC 5737 / RFC 3849).
	dut1TunnelIPv4CIDR  = "192.0.2.5/30"
	dut2TunnelIPv4CIDR  = "192.0.2.6/30"
	dut1TunnelIPv6CIDR  = "2001:db8:100:1::1/64"
	dut2TunnelIPv6CIDR  = "2001:db8:100:1::2/64"
	dut1Tunnel2IPv4CIDR = "198.51.100.1/30"
	dut2Tunnel2IPv4CIDR = "198.51.100.2/30"
	dut1Tunnel2IPv6CIDR = "2001:db8:100:2::1/64"
	dut2Tunnel2IPv6CIDR = "2001:db8:100:2::2/64"

	// Tunnel next-hop addresses used in static routes (no prefix length).
	dut1TunnelIPv4NH = "192.0.2.5"
	dut2TunnelIPv4NH = "192.0.2.6"
	dut1TunnelIPv6NH = "2001:db8:100:1::1"
	dut2TunnelIPv6NH = "2001:db8:100:1::2"

	// Second tunnel next-hop addresses used for ECMP across both tunnels.
	dut1Tunnel2IPv4NH = "198.51.100.1"
	dut2Tunnel2IPv4NH = "198.51.100.2"
	dut1Tunnel2IPv6NH = "2001:db8:100:2::1"
	dut2Tunnel2IPv6NH = "2001:db8:100:2::2"

	// Static route destination prefixes.
	ate1IPv4Prefix   = "192.0.2.0/30"
	ate2IPv4Prefix   = "203.0.113.0/30"
	ate1IPv6Prefix   = "2001:db8:1::0/126"
	ate2IPv6Prefix   = "2001:db8:2::0/126"
	dut1LoopbackPfx  = "2001:db8:3::1/128"
	dut2LoopbackPfx  = "2001:db8:4::1/128"
	dut1Loopback2Pfx = "2001:db8:5::1/128"
	dut2Loopback2Pfx = "2001:db8:6::1/128"

	// OTG MACsec peer name.
	macsecPeerName = "Peer A"

	// OTG flow names.
	flowIPv4Fwd = "Flow-IPv4-Fwd"
	flowIPv4Bwd = "Flow-IPv4-Bwd"
	flowIPv6Fwd = "Flow-IPv6-Fwd"
	flowIPv6Bwd = "Flow-IPv6-Bwd"

	// Traffic generation parameters.
	trafficPPS  = 100
	trafficPkts = 1000

	// Timeout durations.
	lagUpTimeout         = 2 * time.Minute
	trafficWaitTime      = 30 * time.Second
	verifyTrafficTimeout = 2 * time.Minute
	flowTrafficDuration  = 10 * time.Second
)

type SizeWeightPair struct {
	Size   uint32
	Weight float32
}

var (
	// MKA keys.
	cak         = "1234abcd1234abcd1234abcd1234abcd"
	ckn         = "12345678123456781234567812345678"
	fallbackCak = "1234abcd1234abcd1234abcd1234abce"
	fallbackCkn = "12345678123456781234567812345679"

	// DUT port groupings; custPorts are customer-facing (MACsec edge).
	custPorts = []string{"port5"}
	// corePorts are DUT-to-DUT core transport ports, grouped per LAG.
	corePorts = [][]string{
		{"port1", "port2"},
		{"port3", "port4"},
	}
	// ateCustPorts are the ATE OTG customer ports.
	ateCustPorts = []string{"port1", "port2"}

	// ATE LAG configurations (RFC 5737 test networks).
	ate1LagConfig = attrs.Attributes{
		Desc:    "ATE LAG1 configuration",
		IPv4:    "192.0.2.2",
		IPv4Len: 30,
		IPv6:    "2001:db8:1::2",
		IPv6Len: 126,
		MAC:     "00:00:11:01:01:01",
		MTU:     1500,
	}
	ate2LagConfig = attrs.Attributes{
		Desc:    "ATE LAG2 configuration",
		IPv4:    "203.0.113.2",
		IPv4Len: 30,
		IPv6:    "2001:db8:2::2",
		IPv6Len: 126,
		MAC:     "00:00:12:02:02:02",
		MTU:     1500,
	}

	// DUT1 customer-facing interface (VLAN 10 with MACsec).
	dut1CustIntf = attrs.Attributes{
		Desc:    "DUT1 customer interface configuration",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
		IPv6:    "2001:db8:1::1",
		IPv6Len: 126,
		MAC:     "00:00:11:01:01:03",
		MTU:     9216,
		ID:      10,
	}
	// DUT2 customer-facing interface (no VLAN).
	dut2CustIntf = attrs.Attributes{
		Desc:    "DUT2 customer interface configuration",
		IPv4:    "203.0.113.1",
		IPv4Len: 30,
		IPv6:    "2001:db8:2::1",
		IPv6Len: 126,
		MAC:     "00:00:12:02:02:03",
		MTU:     9216,
		ID:      0,
	}

	// DUT core interface configurations (IPv6-only for DUT-to-DUT links).
	dut1CoreIntf1 = attrs.Attributes{
		Desc:    "DUT1 core interface 1 configuration",
		IPv6:    "2001:db8:200:1::1",
		IPv6Len: 126,
		MAC:     "02:00:10:01:01:01",
		MTU:     9216,
	}
	dut1CoreIntf2 = attrs.Attributes{
		Desc:    "DUT1 core interface 2 configuration",
		IPv6:    "2001:db8:200:2::1",
		IPv6Len: 126,
		MAC:     "02:00:10:02:01:01",
		MTU:     9216,
	}
	dut2CoreIntf1 = attrs.Attributes{
		Desc:    "DUT2 core interface 1 configuration",
		IPv6:    "2001:db8:200:1::2",
		IPv6Len: 126,
		MAC:     "02:00:20:01:01:01",
		MTU:     9216,
	}
	dut2CoreIntf2 = attrs.Attributes{
		Desc:    "DUT2 core interface 2 configuration",
		IPv6:    "2001:db8:200:2::2",
		IPv6Len: 126,
		MAC:     "02:00:20:02:01:01",
		MTU:     9216,
	}

	// ATE LAG port MAC addresses.
	ate1LagPortMac = "00:16:01:00:00:01"
	ate2LagPortMac = "00:17:01:00:00:01"

	sizeWeightProfile = []SizeWeightPair{
		{Size: 64, Weight: 20},
		{Size: 128, Weight: 10},
		{Size: 256, Weight: 10},
		{Size: 512, Weight: 10},
		{Size: 1024, Weight: 10},
		{Size: 1500, Weight: 10},
		{Size: 4500, Weight: 10},
		{Size: 9088, Weight: 10},
	}
)

// Packet capture validation definitions, modelled on
// feature/policy_forwarding/otg_tests/mpls_gre_ipv4_encap_test/mpls_gre_ipv4_encap_test.go.
var (
	// MacsecValidations confirms traffic is MACsec-encrypted.
	MacsecValidations = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateMacsecHeader,
	}
	// MacsecPacketValidation validates MACsec-encrypted packets on port1.
	MacsecPacketValidation = &packetvalidationhelpers.PacketValidation{
		PortName:    "port1",
		CaptureName: "macsec-capture",
		MacsecLayer: &packetvalidationhelpers.MacsecLayer{EtherType: 0x88E5},
		Validations: MacsecValidations,
	}

	// DSCPValidations confirms IPv4 DSCP preservation on egress.
	DSCPValidations = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateIPv4Header,
	}
	// DSCPPacketValidation validates the IPv4 DSCP (TOS) value on port2.
	DSCPPacketValidation = &packetvalidationhelpers.PacketValidation{
		PortName:    "port2",
		CaptureName: "dscp-capture",
		IPv4Layer: &packetvalidationhelpers.IPv4Layer{
			SkipProtocolCheck: true,
			DstIP:             ate2LagConfig.IPv4,
			TTL:               62},
		Validations: DSCPValidations,
	}

	// FlowLabelValidations confirms IPv6 flow-label preservation on egress.
	FlowLabelValidations = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateIPv6Header,
	}
	// FlowLabelPacketValidation validates the IPv6 flow label on port2.
	FlowLabelPacketValidation = &packetvalidationhelpers.PacketValidation{
		PortName:    "port2",
		CaptureName: "flowlabel-capture",
		IPv6Layer: &packetvalidationhelpers.IPv6Layer{
			DstIP:    ate2LagConfig.IPv6,
			HopLimit: 62,
		},
		Flags:       &packetvalidationhelpers.ValidationFlags{ValidateFlowLabel: true},
		Validations: FlowLabelValidations,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configDUTInterface configures the DUT interface with the given attributes and applies necessary deviations.
func configDUTInterface(t *testing.T, i *oc.Interface, a *attrs.Attributes, dut *ondatra.DUTDevice) {
	t.Helper()

	i.Description = ygot.String(a.Desc)
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	// Keep subinterface 0 with MTU enabled; required by some devices.
	s0 := i.GetOrCreateSubinterface(baseSubinterfaceIndex)
	ipv4 := s0.GetOrCreateIpv4()
	ipv6 := s0.GetOrCreateIpv6()
	ipv4.Mtu = ygot.Uint16(a.MTU)
	ipv6.Mtu = ygot.Uint32(uint32(a.MTU))
	if deviations.InterfaceEnabled(dut) {
		ipv4.Enabled = ygot.Bool(true)
		ipv6.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(a.ID)
	s4 := s.GetOrCreateIpv4()
	s6 := s.GetOrCreateIpv6()
	s4.Mtu = ygot.Uint16(a.MTU)
	s6.Mtu = ygot.Uint32(uint32(a.MTU))
	if deviations.InterfaceEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
		s6.Enabled = ygot.Bool(true)
	}
	if a.ID != 0 {
		s.GetOrCreateVlan().
			GetOrCreateMatch().
			GetOrCreateSingleTagged().
			SetVlanId(uint16(vlanID))
	}
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

// aggregateSubinterfaceName returns the interface name to use for CLI-based VRF assignment.
func aggregateSubinterfaceName(lagName string, subinterfaceID uint32) string {
	if subinterfaceID == 0 {
		return lagName
	}
	return fmt.Sprintf("%s.%d", lagName, subinterfaceID)
}

// assignAggregateToVRF assigns an aggregate/subinterface to the requested VRF using OC.
// This keeps VRF assignment separate from interface modelling and avoids helper
// failures when vendor-specific network-instance inputs are incomplete.
func assignAggregateToVRF(t *testing.T, dut *ondatra.DUTDevice, lagName string, subinterfaceID uint32, vrfName string) {
	t.Helper()
	if vrfName == "" {
		return
	}

	intfName := aggregateSubinterfaceName(lagName, subinterfaceID)
	d := gnmi.OC()
	ni := d.NetworkInstance(vrfName).Interface(intfName)
	gnmi.Update(t, dut, ni.Config(), &oc.NetworkInstance_Interface{
		Id:           ygot.String(intfName),
		Interface:    ygot.String(lagName),
		Subinterface: ygot.Uint32(subinterfaceID),
	})
}

// configureLAGInterface sets up the LAG aggregate interface, LACP, member ports, subinterfaces, and optional VRF assignment.
func configureLAGInterface(t *testing.T, dut *ondatra.DUTDevice, lagName string, ports []*ondatra.Port, a *attrs.Attributes, vrfName string) {
	t.Helper()
	d := gnmi.OC()

	// Configure aggregate first, then LACP, then members.
	lacp := &oc.Lacp_Interface{Name: ygot.String(lagName)}
	lacp.LacpMode = oc.Lacp_LacpActivityType_ACTIVE

	agg := &oc.Interface{Name: ygot.String(lagName)}
	// Set high-level fields only; add subinterfaces/IPs after aggregate exists.
	agg.Description = ygot.String(a.Desc)
	if deviations.InterfaceEnabled(dut) {
		agg.Enabled = ygot.Bool(true)
	}
	// Set lag-type so member ports can reference this aggregate.
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_LACP
	agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag

	// Transaction 1: Create aggregate, LACP, and member ports.
	if deviations.AggregateAtomicUpdate(dut) {
		batch := &gnmi.SetBatch{}
		gnmi.BatchUpdate(batch, d.Interface(lagName).Config(), agg)
		gnmi.BatchUpdate(batch, d.Lacp().Interface(lagName).Config(), lacp)
		for _, p := range ports {
			i := &oc.Interface{Name: ygot.String(p.Name())}
			i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
			// i.Mtu = ygot.Uint16(a.MTU)
			if deviations.InterfaceEnabled(dut) {
				i.Enabled = ygot.Bool(true)
			}
			e := i.GetOrCreateEthernet()
			e.AggregateId = ygot.String(lagName)
			gnmi.BatchUpdate(batch, d.Interface(p.Name()).Config(), i)
		}
		batch.Set(t, dut)
	} else {
		gnmi.Update(t, dut, d.Interface(lagName).Config(), agg)
		gnmi.Update(t, dut, d.Lacp().Interface(lagName).Config(), lacp)
		for _, p := range ports {
			i := &oc.Interface{Name: ygot.String(p.Name())}
			i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
			// i.Mtu = ygot.Uint16(a.MTU)
			if deviations.InterfaceEnabled(dut) {
				i.Enabled = ygot.Bool(true)
			}
			e := i.GetOrCreateEthernet()
			e.AggregateId = ygot.String(lagName)
			gnmi.Update(t, dut, d.Interface(p.Name()).Config(), i)
		}
	}

	// Assign VRF before IP addresses; use a.ID for correct subinterface assignment.
	assignAggregateToVRF(t, dut, lagName, a.ID, vrfName)

	if deviations.AggregateAtomicUpdate(dut) {
		post := &gnmi.SetBatch{}
		full := &oc.Interface{Name: ygot.String(lagName)}
		full.GetOrCreateAggregation().LagType = agg.GetOrCreateAggregation().GetLagType()
		full.Type = agg.Type
		// Populate subinterface(s) and addresses.
		configDUTInterface(t, full, a, dut)
		gnmi.BatchUpdate(post, d.Interface(lagName).Config(), full)
		post.Set(t, dut)
	} else {
		full := &oc.Interface{Name: ygot.String(lagName)}
		full.GetOrCreateAggregation().LagType = agg.GetOrCreateAggregation().GetLagType()
		full.Type = agg.Type
		configDUTInterface(t, full, a, dut)
		gnmi.Update(t, dut, d.Interface(lagName).Config(), full)
	}
}

// createVRFs creates VRFs via OC or CLI (Arista requires atomic CLI for routing-enable).
func createVRFs(t *testing.T, dut *ondatra.DUTDevice, vrfNames []string) {
	t.Helper()
	if len(vrfNames) == 0 {
		return
	}
	if deviations.IpRoutingInVrfOcUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			// Arista requires atomic CLI for vrf instance + ip routing + ipv6 unicast-routing.
			var b strings.Builder
			for _, vrfName := range vrfNames {
				if vrfName == "" {
					continue
				}
				fmt.Fprintf(&b, "vrf instance %s\n!\nip routing vrf %s\n!\nipv6 unicast-routing vrf %s\n!\n",
					vrfName, vrfName, vrfName)
			}
			if cli := b.String(); cli != "" {
				t.Logf("Creating VRF(s) via CLI on Arista (OC cannot express routing-enable): %v", vrfNames)
				helpers.GnmiCLIConfig(t, dut, cli)
			}
		}
	} else {
		// Create VRF instance using OpenConfig for other vendors.
		d := gnmi.OC()
		for _, vrfName := range vrfNames {
			if vrfName == "" {
				continue
			}
			ni := &oc.NetworkInstance{Name: ygot.String(vrfName)}
			ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
			gnmi.Update(t, dut, d.NetworkInstance(vrfName).Config(), ni)
		}
		t.Logf("Applied OC VRF creation for %d VRF(s): %v", len(vrfNames), vrfNames)
	}
}

// staticRoute represents a single static route entry.
type staticRoute struct {
	Prefix    string // destination prefix in CIDR notation
	NextHop   string // next-hop IP address
	VRF       string // source VRF (empty for the default VRF)
	EgressVRF string // egress VRF for cross-VRF leaking (empty if not used)
}

// configureStaticRoutes configures static routes via OpenConfig or CLI based on device capabilities.
// Routes are grouped by VRF and batched where possible to minimize gNMI roundtrips.
func configureStaticRoutes(t *testing.T, dut *ondatra.DUTDevice, routes []staticRoute) {
	t.Helper()

	if deviations.StaticRouteInVrfOcUnsupported(dut) {
		// Configure all routes via CLI on devices that don't support OC static routes.
		if dut.Vendor() != ondatra.ARISTA {
			t.Fatalf("Static route CLI configuration not implemented for vendor %v; implement vendor-specific handling or use OpenConfig", dut.Vendor())
		}
		// Group CLI routes by VRF and batch commands per VRF
		cliRoutesByVRF := make(map[string][]staticRoute)
		for _, r := range routes {
			vrfName := r.VRF
			if vrfName == "" {
				vrfName = deviations.DefaultNetworkInstance(dut)
			}
			cliRoutesByVRF[vrfName] = append(cliRoutesByVRF[vrfName], r)
		}

		for vrfName, vrfRoutes := range cliRoutesByVRF {
			var cliCommands []string
			for _, r := range vrfRoutes {
				ipType := "ip"
				for _, ch := range r.Prefix {
					if ch == ':' {
						ipType = "ipv6"
						break
					}
				}
				var cli string
				switch {
				case r.EgressVRF != "" && r.VRF != "":
					cli = fmt.Sprintf("%s route vrf %s %s egress-vrf %s %s",
						ipType, r.VRF, r.Prefix, r.EgressVRF, r.NextHop)
				case r.EgressVRF == "" && r.VRF == "":
					cli = fmt.Sprintf("%s route %s %s", ipType, r.Prefix, r.NextHop)
				default:
					cli = fmt.Sprintf("%s route %s egress-vrf %s %s",
						ipType, r.Prefix, r.EgressVRF, r.NextHop)
				}
				cliCommands = append(cliCommands, cli)
			}
			if len(cliCommands) > 0 {
				helpers.GnmiCLIConfig(t, dut, strings.Join(cliCommands, "\n"))
				t.Logf("Configured %d CLI static routes in VRF %s", len(cliCommands), vrfName)
			}
		}
	} else {
		// Configure all routes via OpenConfig on devices that support it.
		type ocNextHop struct {
			IP        string
			EgressVRF string
		}

		// Group OC routes by VRF and prefix for batching
		ocRoutesByVRF := make(map[string]map[string][]ocNextHop) // [vrfName][prefix][]{IP, EgressVRF}
		for _, r := range routes {
			vrfName := r.VRF
			if vrfName == "" {
				vrfName = deviations.DefaultNetworkInstance(dut)
			}
			if ocRoutesByVRF[vrfName] == nil {
				ocRoutesByVRF[vrfName] = make(map[string][]ocNextHop)
			}
			ocRoutesByVRF[vrfName][r.Prefix] = append(ocRoutesByVRF[vrfName][r.Prefix], ocNextHop{
				IP:        r.NextHop,
				EgressVRF: r.EgressVRF,
			})
		}

		// Configure OC routes: batch by VRF, use incremented indices for each next-hop
		for vrfName, prefixMap := range ocRoutesByVRF {
			proto := &oc.NetworkInstance_Protocol{
				Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
				Name:       ygot.String(deviations.StaticProtocolName(dut)),
			}
			for prefix, nextHops := range prefixMap {
				sr := proto.GetOrCreateStatic(prefix)
				sr.Prefix = ygot.String(prefix)
				// Use incremented index for each next-hop to support ECMP
				for i, nhInfo := range nextHops {
					indexStr := strconv.Itoa(i)
					nh := sr.GetOrCreateNextHop(indexStr)
					nh.Index = ygot.String(indexStr)
					nh.NextHop = oc.UnionString(nhInfo.IP)
					if nhInfo.EgressVRF != "" {
						nh.NextNetworkInstance = ygot.String(nhInfo.EgressVRF)
					}
				}
			}
			sp := gnmi.OC().NetworkInstance(vrfName).Protocol(
				oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
			gnmi.Update(t, dut, sp.Config(), proto)
			t.Logf("Configured %d OC static routes in VRF %s", len(prefixMap), vrfName)
		}
	}
}
func configureDUT(t *testing.T, dut *ondatra.DUTDevice,
	portGroups [][]*ondatra.Port,
	portAttrs []attrs.Attributes,
	vrfName string) {

	t.Helper()

	if len(portGroups) != len(portAttrs) {
		t.Fatalf("mismatched portGroups and portAttrs lengths")
	}

	// VRF should already exist; just configure interfaces.

	for i := range portGroups {
		// Generate unique aggregate ID per DUT per LAG.
		lag := netutil.NextAggregateInterface(t, dut)
		configureLAGInterface(t, dut, lag, portGroups[i], &portAttrs[i], vrfName)
	}
}
func configureLoopback(t *testing.T, dut *ondatra.DUTDevice, lbName, ip string, prefixLen uint8, isIPv6 bool) {
	t.Helper()

	i := &oc.Interface{}
	i.Name = ygot.String(lbName)
	i.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback

	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	s0 := i.GetOrCreateSubinterface(baseSubinterfaceIndex)

	if isIPv6 {
		ipv6 := s0.GetOrCreateIpv6()
		if deviations.InterfaceEnabled(dut) {
			ipv6.Enabled = ygot.Bool(true)
		}
		addr := ipv6.GetOrCreateAddress(ip)
		addr.PrefixLength = ygot.Uint8(prefixLen)
	} else {
		ipv4 := s0.GetOrCreateIpv4()
		if deviations.InterfaceEnabled(dut) {
			ipv4.Enabled = ygot.Bool(true)
		}
		addr := ipv4.GetOrCreateAddress(ip)
		addr.PrefixLength = ygot.Uint8(prefixLen)
	}

	gnmi.Replace(t, dut,
		gnmi.OC().
			Interface(lbName).
			Config(),
		i)
}

func configureATE(t *testing.T) gosnappi.Config {
	t.Helper()

	top := gosnappi.NewConfig()

	// Port mapping: ATE port1/port2 <-> DUT1/DUT2 port5 (MACSec on DUT1).
	p1 := top.Ports().Add().SetName(ateCustPorts[0])
	p2 := top.Ports().Add().SetName(ateCustPorts[1])

	// add lags
	l1 := top.Lags().Add().SetName(ate1LagName)

	lagPort1 := l1.Ports().Add().SetPortName(p1.Name())
	lagPort1.Lacp().
		SetActorActivity("active").
		SetActorPortNumber(1)
	lagPort1.Ethernet().
		SetName("lag1Eth").
		SetMac(ate1LagPortMac).
		SetMtu(uint32(ate1LagConfig.MTU))
	l1.Protocol().Lacp().
		SetActorSystemId("00:00:00:00:00:01").
		SetActorSystemPriority(0).
		SetActorKey(1)

	// MACsec Configuration
	macsec1 := lagPort1.Macsec()
	secy1 := macsec1.SecureEntity().SetName(macsecPeerName)
	secy1Encapsulation := secy1.DataPlane().Encapsulation()
	secy1Encapsulation.CryptoEngine().EncryptDecrypt().HardwareAcceleration().InlineCrypto()
	// MKA Configuration
	mka := secy1.KeyGenerationProtocol().Mka().SetName("PeerA-Mka")
	mka.Basic().KeySource().Psk()

	mka.Basic().SetKeyDerivationFunction(gosnappi.MkaBasicKeyDerivationFunctionEnum("aes_cmac_128"))
	mka.Basic().SetSendIcvIndicatiorInMkpdu(false)
	mka.Basic().SetMkaVersion(2)
	scs := mka.Basic().SupportedCipherSuites()
	scs.SetGcmAes256(false)
	scs.SetGcmAesXpn256(false)

	onePsk := mka.Basic().KeySource().Psks().Add()
	onePsk.SetCakValue(cak)
	onePsk.SetCakName(ckn)
	secureChannel := mka.Tx().SecureChannels().Add()
	secureChannel.SetName("SecureChannel1").
		SetSystemId(lagPort1.Ethernet().Mac())

	// add devices
	d1 := top.Devices().Add().SetName(ate1DevName)
	// add protocol stacks for device d1
	d1Eth1 := d1.Ethernets().
		Add().
		SetName("d1Eth").
		SetMac(ate1LagConfig.MAC)
	d1Eth1.Connection().SetLagName(l1.Name())

	d1Eth1.Vlans().Add().SetName("d1EthVlan").SetId(vlanID)

	d1ipv4 := d1Eth1.Ipv4Addresses().
		Add().
		SetName("p1d1ipv4").
		SetAddress(ate1LagConfig.IPv4).
		SetGateway(dut1CustIntf.IPv4).
		SetPrefix(uint32(ate1LagConfig.IPv4Len))

	d1ipv6 := d1Eth1.Ipv6Addresses().
		Add().
		SetName("p1d1ipv6").
		SetAddress(ate1LagConfig.IPv6).
		SetGateway(dut1CustIntf.IPv6).
		SetPrefix(uint32(ate1LagConfig.IPv6Len))

	l2 := top.Lags().Add().SetName(ate2LagName)
	lagPort2 := l2.Ports().Add().SetPortName(p2.Name())
	lagPort2.Lacp().
		SetActorActivity("active").
		SetActorPortNumber(1)
	lagPort2.Ethernet().
		SetName("lag2Eth").
		SetMac(ate2LagPortMac).
		SetMtu(uint32(ate2LagConfig.MTU))
	l2.Protocol().Lacp().
		SetActorSystemId("00:00:00:00:00:02").
		SetActorSystemPriority(0).
		SetActorKey(1)

	d2 := top.Devices().Add().SetName(ate2DevName)
	d2Eth1 := d2.Ethernets().
		Add().
		SetName("d2Eth").
		SetMac(ate2LagConfig.MAC)
	d2Eth1.Connection().SetLagName(l2.Name())

	d2ipv4 := d2Eth1.Ipv4Addresses().
		Add().
		SetName("p2d2ipv4").
		SetAddress(ate2LagConfig.IPv4).
		SetGateway(dut2CustIntf.IPv4).
		SetPrefix(uint32(ate2LagConfig.IPv4Len))

	d2ipv6 := d2Eth1.Ipv6Addresses().
		Add().
		SetName("p2d2ipv6").
		SetAddress(ate2LagConfig.IPv6).
		SetGateway(dut2CustIntf.IPv6).
		SetPrefix(uint32(ate2LagConfig.IPv6Len))

	if len(top.Flows().Items()) > 0 {
		top.Flows().Clear()
	}
	flow := top.Flows().Add().SetName(flowIPv4Fwd)
	flow.TxRx().Device().SetTxNames([]string{d1ipv4.Name()}).SetRxNames([]string{d2ipv4.Name()})

	for _, sizeWeight := range sizeWeightProfile {
		flow.Size().WeightPairs().Custom().Add().SetSize(sizeWeight.Size).SetWeight(sizeWeight.Weight)
	}

	flow.Rate().SetPps(trafficPPS)
	flow.Duration().Continuous()
	flow.Metrics().SetEnable(true)

	e1 := flow.Packet().Add().Ethernet()
	e1.Src().SetValue(ate1LagConfig.MAC)

	flow.Packet().Add().Macsec()

	vlan := flow.Packet().Add().Vlan()
	vlan.Id().SetValue(vlanID)

	v4 := flow.Packet().Add().Ipv4()
	// Increment the source address to generate flow entropy so that ECMP hashing
	// spreads the customer traffic across the DUT-to-DUT member links.
	v4.Src().Increment().SetStart(ate1LagConfig.IPv4).SetStep("0.0.0.1").SetCount(1000)
	v4.Dst().SetValue(ate2LagConfig.IPv4)

	flowBwd := top.Flows().Add().SetName(flowIPv4Bwd)
	flowBwd.TxRx().Device().SetTxNames([]string{d2ipv4.Name()}).SetRxNames([]string{d1ipv4.Name()})

	for _, sizeWeight := range sizeWeightProfile {
		flowBwd.Size().WeightPairs().Custom().Add().SetSize(sizeWeight.Size).SetWeight(sizeWeight.Weight)
	}

	flowBwd.Rate().SetPps(trafficPPS)
	flowBwd.Duration().Continuous()
	flowBwd.Metrics().SetEnable(true)

	e2 := flowBwd.Packet().Add().Ethernet()
	e2.Src().SetValue(ate2LagConfig.MAC)

	v4Bwd := flowBwd.Packet().Add().Ipv4()
	v4Bwd.Src().SetValue(ate2LagConfig.IPv4)
	v4Bwd.Dst().SetValue(ate1LagConfig.IPv4)

	// IPv6 Flow from port1 to port2.
	flowV6 := top.Flows().Add().SetName(flowIPv6Fwd)
	flowV6.TxRx().Device().SetTxNames([]string{d1ipv6.Name()}).SetRxNames([]string{d2ipv6.Name()})

	for _, sizeWeight := range sizeWeightProfile {
		flowV6.Size().WeightPairs().Custom().Add().SetSize(sizeWeight.Size).SetWeight(sizeWeight.Weight)
	}
	flowV6.Rate().SetPps(trafficPPS)
	flowV6.Duration().Continuous()
	flowV6.Metrics().SetEnable(true)

	e3 := flowV6.Packet().Add().Ethernet()
	e3.Src().SetValue(ate1LagConfig.MAC)

	flowV6.Packet().Add().Macsec()

	vlan3 := flowV6.Packet().Add().Vlan()
	vlan3.Id().SetValue(vlanID)

	v6 := flowV6.Packet().Add().Ipv6()
	// Increment the source address to generate flow entropy so that ECMP hashing
	// spreads the customer traffic across the DUT-to-DUT member links.
	v6.Src().Increment().SetStart(ate1LagConfig.IPv6).SetStep("::1").SetCount(1000)
	v6.Dst().SetValue(ate2LagConfig.IPv6)

	// IPv6 Flow from port2 to port1.
	flowV6Bwd := top.Flows().Add().SetName(flowIPv6Bwd)
	flowV6Bwd.TxRx().Device().SetTxNames([]string{d2ipv6.Name()}).SetRxNames([]string{d1ipv6.Name()})

	for _, sizeWeight := range sizeWeightProfile {
		flowV6Bwd.Size().WeightPairs().Custom().Add().SetSize(sizeWeight.Size).SetWeight(sizeWeight.Weight)
	}

	flowV6Bwd.Rate().SetPps(trafficPPS)
	flowV6Bwd.Duration().Continuous()
	flowV6Bwd.Metrics().SetEnable(true)

	e4 := flowV6Bwd.Packet().Add().Ethernet()
	e4.Src().SetValue(ate2LagConfig.MAC)

	v6Bwd := flowV6Bwd.Packet().Add().Ipv6()
	v6Bwd.Src().SetValue(ate2LagConfig.IPv6)
	v6Bwd.Dst().SetValue(ate1LagConfig.IPv6)

	return top
}

func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, cfg gosnappi.Config, flowName string, testResults bool) error {
	t.Helper()

	flowPath := gnmi.OTG().Flow(flowName).State()
	watchTimeout := verifyTrafficTimeout

	watchFn := func(val *ygnmi.Value[*otgtelemetry.Flow]) bool {
		metric, ok := val.Val()
		if !ok || metric == nil {
			return false
		}

		framesTx := metric.GetCounters().GetOutPkts()
		framesRx := metric.GetCounters().GetInPkts()
		if framesTx == 0 {
			return false
		}

		if testResults {
			// Expect frames to be received.
			return framesRx == framesTx
		}

		// Expect no frames to be received.
		return framesRx == 0
	}

	watch := gnmi.Watch(t, ate.OTG(), flowPath, watchTimeout, watchFn)

	last, ok := watch.Await(t)
	if !ok {
		recvMetric := gnmi.Get(t, ate.OTG(), flowPath)
		framesTx := recvMetric.GetCounters().GetOutPkts()
		framesRx := recvMetric.GetCounters().GetInPkts()

		// If the final snapshot already matches expectations, treat as pass.
		if testResults {
			if framesTx > 0 && framesRx == framesTx {
				t.Logf("%s: traffic verification passed: FramesTx: %d, FramesRx: %d", flowName, framesTx, framesRx)
				return nil
			}
		} else {
			if framesTx > 0 && framesRx == 0 {
				t.Logf("%s: traffic verification passed: FramesTx: %d, FramesRx: %d", flowName, framesTx, framesRx)
				return nil
			}
		}

		var errMsg string
		if testResults {
			errMsg = fmt.Sprintf("%s: traffic verification did not pass: FramesTx: %d, FramesRx: %d, want FramesRx == FramesTx and FramesTx > 0", flowName, framesTx, framesRx)
		} else {
			errMsg = fmt.Sprintf("%s: traffic verification did not pass: FramesTx: %d, FramesRx: %d, want FramesRx == 0 and FramesTx > 0", flowName, framesTx, framesRx)
		}
		otgutils.LogFlowMetrics(t, ate.OTG(), cfg)
		return fmt.Errorf("%s", errMsg)
	}

	recvMetric, present := last.Val()
	if !present || recvMetric == nil {
		recvMetric = gnmi.Get(t, ate.OTG(), flowPath)
	}
	framesTx := recvMetric.GetCounters().GetOutPkts()
	framesRx := recvMetric.GetCounters().GetInPkts()
	otgutils.LogFlowMetrics(t, ate.OTG(), cfg)
	t.Logf("%s: traffic verification passed: FramesTx: %d, FramesRx: %d", flowName, framesTx, framesRx)
	return nil
}

func lagUpCheck(t *testing.T, lagName string, wantMembersUp uint64) func(*ygnmi.Value[*otgtelemetry.Lag]) bool {
	return func(val *ygnmi.Value[*otgtelemetry.Lag]) bool {
		lag, ok := val.Val()
		if !ok || lag == nil {
			return false
		}

		oper := lag.GetOperStatus()
		membersUp := lag.GetCounters().GetMemberPortsUp()

		if oper == otgtelemetry.Lag_OperStatus_UP && membersUp == wantMembersUp {
			t.Logf("OTG LAG %s is UP with %d member(s) up", lagName, membersUp)
			return true
		}

		t.Logf("Waiting OTG LAG %s: oper-status=%v member-ports-up=%d (want oper-status=UP, member-ports-up=%d)",
			lagName, oper, membersUp, wantMembersUp)

		return false
	}
}

func waitForOTGLAGUP(t *testing.T, ate *ondatra.ATEDevice, lagName string, wantMembersUp uint64, timeout time.Duration) {
	t.Helper()

	otg := ate.OTG()

	t.Logf("Waiting for OTG LAG %s to be UP with %d member(s)", lagName, wantMembersUp)

	watchFn := lagUpCheck(t, lagName, wantMembersUp)
	watch := gnmi.Watch(
		t,
		otg,
		gnmi.OTG().Lag(lagName).State(),
		timeout,
		watchFn,
	)

	if _, ok := watch.Await(t); !ok {
		finalOper := gnmi.Get(t, otg, gnmi.OTG().Lag(lagName).OperStatus().State())
		finalMembers := gnmi.Get(t, otg, gnmi.OTG().Lag(lagName).Counters().MemberPortsUp().State())

		t.Fatalf("OTG LAG %s did not become ready within %v: final oper-status=%v member-ports-up=%d (want oper-status=UP, member-ports-up=%d)",
			lagName, timeout, finalOper, finalMembers, wantMembersUp)
	}
}

func macsecUpCheck(t *testing.T, ifName string) func(*ygnmi.Value[otgtelemetry.E_Interface_SessionState]) bool {
	return func(val *ygnmi.Value[otgtelemetry.E_Interface_SessionState]) bool {
		state, ok := val.Val()
		if !ok {
			t.Logf("Waiting MACsec session on %s: no value yet", ifName)
			return false
		}
		if state != otgtelemetry.Interface_SessionState_UP {
			t.Logf("Waiting MACsec session on %s: current state=%v, want UP", ifName, state)
			return false
		}
		return true
	}
}

func waitForOTGMACSecUp(t *testing.T, ate *ondatra.ATEDevice, ifName string, timeout time.Duration) {
	t.Helper()

	otg := ate.OTG()

	t.Logf("Waiting for OTG MACsec session on %s to be UP", ifName)

	watchFn := macsecUpCheck(t, ifName)
	watch := gnmi.Watch(
		t,
		otg,
		gnmi.OTG().Macsec().Interface(ifName).SessionState().State(),
		timeout,
		watchFn,
	)

	if _, ok := watch.Await(t); !ok {
		finalState := gnmi.Get(t, otg, gnmi.OTG().Macsec().Interface(ifName).SessionState().State())
		t.Fatalf("MACsec session on %s did not come UP within %v, final state=%v",
			ifName, timeout, finalState)
	}
}

func configureBaseSingleTunnel(t *testing.T, dut1, dut2 *ondatra.DUTDevice) {
	t.Helper()

	batch1 := cfgplugins.ConfigureIPSecTunnel(t, dut1, cfgplugins.IPSecTunnelCfg{
		TunnelName:  tunnelIfName,
		Description: "IPsec Tunnel Pair 1 to DUT2",
		LocalFQDN:   dut1FQDN,
		RemoteFQDN:  dut2FQDN,
		TunnelIPv4:  dut1TunnelIPv4CIDR,
		TunnelIPv6:  dut1TunnelIPv6CIDR,
		TunnelSrc:   dut1LoopbackIPv6,
		TunnelDst:   dut2LoopbackIPv6,
		TunnelVRF:   tunnelVRF,
	})
	batch1.Set(t, dut1)

	batch2 := cfgplugins.ConfigureIPSecTunnel(t, dut2, cfgplugins.IPSecTunnelCfg{
		TunnelName:  tunnelIfName,
		Description: "IPsec Tunnel Pair 1 to DUT1",
		LocalFQDN:   dut2FQDN,
		RemoteFQDN:  dut1FQDN,
		TunnelIPv4:  dut2TunnelIPv4CIDR,
		TunnelIPv6:  dut2TunnelIPv6CIDR,
		TunnelSrc:   dut2LoopbackIPv6,
		TunnelDst:   dut1LoopbackIPv6,
		TunnelVRF:   tunnelVRF,
	})
	batch2.Set(t, dut2)
}

func configureDualTunnels(t *testing.T, dut1, dut2 *ondatra.DUTDevice) {
	t.Helper()

	configureBaseSingleTunnel(t, dut1, dut2)
	configureLoopback(t, dut1, loopback2IfName, dut1Loopback2IPv6, loopbackPrefixLen, true)
	configureLoopback(t, dut2, loopback2IfName, dut2Loopback2IPv6, loopbackPrefixLen, true)

	batch3 := cfgplugins.ConfigureIPSecTunnel(t, dut1, cfgplugins.IPSecTunnelCfg{
		TunnelName:  tunnel2IfName,
		Description: "IPsec Tunnel Pair 2 to DUT2",
		LocalFQDN:   dut1FQDN2,
		RemoteFQDN:  dut2FQDN2,
		TunnelIPv4:  dut1Tunnel2IPv4CIDR,
		TunnelIPv6:  dut1Tunnel2IPv6CIDR,
		TunnelSrc:   dut1Loopback2IPv6,
		TunnelDst:   dut2Loopback2IPv6,
		TunnelVRF:   tunnelVRF,
		IKEPolicy:   "IKE_POLICY_2",
		SAPolicy:    "SA_POLICY_2",
		Profile:     "IPSEC_PROFILE_2",
	})
	batch3.Set(t, dut1)

	batch4 := cfgplugins.ConfigureIPSecTunnel(t, dut2, cfgplugins.IPSecTunnelCfg{
		TunnelName:  tunnel2IfName,
		Description: "IPsec Tunnel Pair 2 to DUT1",
		LocalFQDN:   dut2FQDN2,
		RemoteFQDN:  dut1FQDN2,
		TunnelIPv4:  dut2Tunnel2IPv4CIDR,
		TunnelIPv6:  dut2Tunnel2IPv6CIDR,
		TunnelSrc:   dut2Loopback2IPv6,
		TunnelDst:   dut1Loopback2IPv6,
		TunnelVRF:   tunnelVRF,
		IKEPolicy:   "IKE_POLICY_2",
		SAPolicy:    "SA_POLICY_2",
		Profile:     "IPSEC_PROFILE_2",
	})
	batch4.Set(t, dut2)

	configureStaticRoutes(t, dut1, []staticRoute{{Prefix: dut2Loopback2Pfx, NextHop: dut2CoreIntf2.IPv6}, {Prefix: dut2Loopback2Pfx, NextHop: dut2CoreIntf1.IPv6}})
	configureStaticRoutes(t, dut2, []staticRoute{{Prefix: dut1Loopback2Pfx, NextHop: dut1CoreIntf2.IPv6}, {Prefix: dut1Loopback2Pfx, NextHop: dut1CoreIntf1.IPv6}})

	// Add second equal-cost route for ECMP load-balancing; provides failover on tunnel down.
	configureStaticRoutes(t, dut1, []staticRoute{
		{Prefix: ate2IPv4Prefix, NextHop: dut2Tunnel2IPv4NH, VRF: ateVRF, EgressVRF: tunnelVRF},
		{Prefix: ate2IPv6Prefix, NextHop: dut2Tunnel2IPv6NH, VRF: ateVRF, EgressVRF: tunnelVRF},
	})
	configureStaticRoutes(t, dut2, []staticRoute{
		{Prefix: ate1IPv4Prefix, NextHop: dut1Tunnel2IPv4NH, VRF: ateVRF, EgressVRF: tunnelVRF},
		{Prefix: ate1IPv6Prefix, NextHop: dut1Tunnel2IPv6NH, VRF: ateVRF, EgressVRF: tunnelVRF},
	})
}

// configureQoSClassification configures QoS policy to classify IPSec and IKE traffic.
func configureQoSClassification(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	helpers.GnmiCLIConfig(t, dut, `class-map CM-IPSEC-AF3
   match protocol ipsec
!
class-map CM-IKE-AF4
   match protocol ike
!
policy-map PM-IPSEC-CONGESTION
   class CM-IPSEC-AF3
      set traffic-class 3
   class CM-IKE-AF4
      set traffic-class 4
   class class-default
      set traffic-class 1
!`)
}

func verifyTunnelOperStatus(t *testing.T, dut *ondatra.DUTDevice, tunnelName string, want oc.E_Interface_OperStatus, timeout time.Duration) {
	t.Helper()

	path := gnmi.OC().Interface(tunnelName).OperStatus().State()
	_, ok := gnmi.Watch(t, dut, path, timeout, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
		status, present := val.Val()
		return present && status == want
	}).Await(t)
	if !ok {
		got := gnmi.Get(t, dut, path)
		t.Fatalf("tunnel %s oper-status got %v, want %v", tunnelName, got, want)
	}
}

func readMemberOutPkts(t *testing.T, dut *ondatra.DUTDevice, memberPorts []*ondatra.Port) map[string]uint64 {
	t.Helper()
	vals := make(map[string]uint64)
	for _, p := range memberPorts {
		vals[p.Name()] = gnmi.Get(t, dut, gnmi.OC().Interface(p.Name()).Counters().OutPkts().State())
	}
	return vals
}

func verifyDUTDUTLoadBalance(t *testing.T, dut *ondatra.DUTDevice, memberPorts []*ondatra.Port, baseline map[string]uint64, tolerance float64, wantSingleLink bool) error {
	t.Helper()

	after := readMemberOutPkts(t, dut, memberPorts)
	delta := make(map[string]uint64)
	var total uint64
	active := 0

	for _, p := range memberPorts {
		name := p.Name()
		if after[name] > baseline[name] {
			delta[name] = after[name] - baseline[name]
		}
		total += delta[name]
		if delta[name] > 0 {
			active++
		}
	}

	if total == 0 {
		var errs []error
		errs = append(errs, fmt.Errorf("no packets observed on DUT-to-DUT member links"))
		return fmt.Errorf("verification failed: %v", errs)
	}

	var errs []error

	if wantSingleLink {
		if active != 1 {
			errs = append(errs, fmt.Errorf("single-link expectation failed: active members got %d, want 1", active))
		}
	} else {
		if active != len(memberPorts) {
			errs = append(errs, fmt.Errorf("balanced load expectation failed: active members got %d, want %d", active, len(memberPorts)))
		}

		evenShare := 1.0 / float64(len(memberPorts))
		for _, p := range memberPorts {
			name := p.Name()
			share := float64(delta[name]) / float64(total)
			if math.Abs(share-evenShare) > tolerance {
				errs = append(errs, fmt.Errorf("member %s share got %.3f, want %.3f +/- %.3f", name, share, evenShare, tolerance))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("load balance verification failed: %v", errs)
	}
	return nil
}

func verifyTunnelOperStatusStaysUp(t *testing.T, dut *ondatra.DUTDevice, tunnelName string, window time.Duration) {
	t.Helper()

	path := gnmi.OC().Interface(tunnelName).OperStatus().State()
	watch := gnmi.Watch(t, dut, path, window, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
		status, present := val.Val()
		return present && status != oc.Interface_OperStatus_UP
	})
	if _, downSeen := watch.Await(t); downSeen {
		got := gnmi.Get(t, dut, path)
		t.Fatalf("tunnel %s oper-status changed during watch window, final got %v, want %v", tunnelName, got, oc.Interface_OperStatus_UP)
	}
	got := gnmi.Get(t, dut, path)
	if got != oc.Interface_OperStatus_UP {
		t.Fatalf("tunnel %s oper-status got %v, want %v", tunnelName, got, oc.Interface_OperStatus_UP)
	}
}

func runTrafficWindow(t *testing.T, otg *otg.OTG, d time.Duration) {
	t.Helper()
	otg.StartTraffic(t)
	time.Sleep(d)
	otg.StopTraffic(t)
}

// readChildSPIs reads child SPI values via raw gNMI Get with wildcarded list keys.
func readChildSPIs(t *testing.T, dut *ondatra.DUTDevice) []uint64 {
	t.Helper()
	gnmiClient := dut.RawAPIs().GNMI(t)
	resp, err := gnmiClient.Get(context.Background(), &gpb.GetRequest{
		Path: []*gpb.Path{{
			Elem: []*gpb.PathElem{
				{Name: "network-instances"},
				{Name: "network-instance", Key: map[string]string{"name": deviations.DefaultNetworkInstance(dut)}},
				{Name: "security"},
				{Name: "ipsec"},
				{Name: "ipv6"},
				{Name: "connections"},
				{Name: "connection"}, // no keys = wildcard over all connections
				{Name: "child-security-associations"},
				{Name: "child-security-association"}, // no keys = wildcard over all SAs
				{Name: "state"},
				{Name: "spi"},
			},
		}},
		Type:     gpb.GetRequest_STATE,
		Encoding: gpb.Encoding_JSON_IETF,
	})
	if err != nil {
		t.Logf("readChildSPIs: gNMI Get error: %v", err)
		return nil
	}
	seen := make(map[uint64]bool)
	var spis []uint64
	for _, notif := range resp.GetNotification() {
		for _, update := range notif.GetUpdate() {
			// Extract SPI from path key of child-security-association.
			for _, elem := range update.GetPath().GetElem() {
				if elem.GetName() == "child-security-association" {
					if spiStr, ok := elem.GetKey()["spi"]; ok {
						if spi, err := strconv.ParseUint(spiStr, 10, 64); err == nil && spi > 0 && !seen[spi] {
							seen[spi] = true
							spis = append(spis, spi)
						}
					}
				}
			}
			// Fallback: parse JSON value.
			if jsonVal := update.GetVal().GetJsonIetfVal(); len(jsonVal) > 0 && len(spis) == 0 {
				var fval float64
				if err := json.Unmarshal(jsonVal, &fval); err == nil && fval > 0 {
					spi := uint64(fval)
					if !seen[spi] {
						seen[spi] = true
						spis = append(spis, spi)
					}
				}
			}
		}
	}
	return spis
}

func verifyDSCPPreservation(t *testing.T, ate *ondatra.ATEDevice, dscp uint32) error {
	t.Helper()

	otg := ate.OTG()
	top := configureATE(t)
	top.Flows().Clear()
	DSCPPacketValidation.CaptureName = fmt.Sprintf("capture-dscp-%d", dscp)
	DSCPPacketValidation.IPv4Layer.Tos = uint8(dscp << 2)
	packetvalidationhelpers.ConfigurePacketCapture(t, top, DSCPPacketValidation)

	flow := top.Flows().Add().SetName(fmt.Sprintf("Flow-IPv4-DSCP-%d", dscp))
	flow.TxRx().Device().SetTxNames([]string{"p1d1ipv4"}).SetRxNames([]string{"p2d2ipv4"})
	for _, sizeWeight := range sizeWeightProfile {
		flow.Size().WeightPairs().Custom().Add().SetSize(sizeWeight.Size).SetWeight(sizeWeight.Weight)
	}
	flow.Rate().SetPps(trafficPPS)
	flow.Duration().FixedPackets().SetPackets(trafficPkts)
	flow.Metrics().SetEnable(true)

	e := flow.Packet().Add().Ethernet()
	e.Src().SetValue(ate1LagConfig.MAC)
	flow.Packet().Add().Macsec()
	flow.Packet().Add().Vlan().Id().SetValue(vlanID)
	v4 := flow.Packet().Add().Ipv4()
	v4.Src().SetValue(ate1LagConfig.IPv4)
	v4.Dst().SetValue(ate2LagConfig.IPv4)
	v4.Priority().Dscp().Phb().SetValue(dscp)

	otg.PushConfig(t, top)
	otg.StartProtocols(t)
	otgutils.WaitForARP(t, otg, top, "IPv4")

	cs := packetvalidationhelpers.StartCapture(t, ate)
	runTrafficWindow(t, otg, flowTrafficDuration)
	packetvalidationhelpers.StopCapture(t, ate, cs)

	if err := verifyTraffic(t, ate, top, flow.Name(), true); err != nil {
		packetvalidationhelpers.ClearCapture(t, top, ate)
		return fmt.Errorf("traffic verification failed: %v", err)
	}
	if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, DSCPPacketValidation); err != nil {
		packetvalidationhelpers.ClearCapture(t, top, ate)
		return fmt.Errorf("capture and validate packets failed dscp=%d: %v", dscp, err)
	}
	packetvalidationhelpers.ClearCapture(t, top, ate)
	return nil
}

func verifyFlowLabelPreservation(t *testing.T, ate *ondatra.ATEDevice, flowLabel uint32) error {
	t.Helper()

	otg := ate.OTG()
	top := configureATE(t)
	top.Flows().Clear()
	FlowLabelPacketValidation.CaptureName = fmt.Sprintf("capture-flowlabel-%d", flowLabel)
	FlowLabelPacketValidation.IPv6Layer.FlowLabel = flowLabel
	packetvalidationhelpers.ConfigurePacketCapture(t, top, FlowLabelPacketValidation)

	flow := top.Flows().Add().SetName(fmt.Sprintf("Flow-IPv6-FlowLabel-%d", flowLabel))
	flow.TxRx().Device().SetTxNames([]string{"p1d1ipv6"}).SetRxNames([]string{"p2d2ipv6"})
	for _, sizeWeight := range sizeWeightProfile {
		flow.Size().WeightPairs().Custom().Add().SetSize(sizeWeight.Size).SetWeight(sizeWeight.Weight)
	}
	flow.Rate().SetPps(trafficPPS)
	flow.Duration().FixedPackets().SetPackets(trafficPkts)
	flow.Metrics().SetEnable(true)

	e := flow.Packet().Add().Ethernet()
	e.Src().SetValue(ate1LagConfig.MAC)
	flow.Packet().Add().Macsec()
	flow.Packet().Add().Vlan().Id().SetValue(vlanID)
	v6 := flow.Packet().Add().Ipv6()
	v6.Src().SetValue(ate1LagConfig.IPv6)
	v6.Dst().SetValue(ate2LagConfig.IPv6)
	v6.FlowLabel().SetValue(flowLabel)

	otg.PushConfig(t, top)
	otg.StartProtocols(t)
	otgutils.WaitForARP(t, otg, top, "IPv6")

	cs := packetvalidationhelpers.StartCapture(t, ate)
	runTrafficWindow(t, otg, flowTrafficDuration)
	packetvalidationhelpers.StopCapture(t, ate, cs)

	if err := verifyTraffic(t, ate, top, flow.Name(), true); err != nil {
		packetvalidationhelpers.ClearCapture(t, top, ate)
		return fmt.Errorf("traffic verification failed: %v", err)
	}
	if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, FlowLabelPacketValidation); err != nil {
		packetvalidationhelpers.ClearCapture(t, top, ate)
		return fmt.Errorf("capture and validate packets failed flowLabel=%d: %v", flowLabel, err)
	}
	packetvalidationhelpers.ClearCapture(t, top, ate)
	return nil
}

func TestIPSecWithMACSecOverAggregatedLinks(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()

	// Configure DUT interfaces, VLANs, VRFs, MACSec, and DUT-DUT transport aggregates.

	// Build DUT port objects from custPorts/corePorts groupings.
	dut1CorePortGroups := make([][]*ondatra.Port, len(corePorts))
	dut2CorePortGroups := make([][]*ondatra.Port, len(corePorts))
	var dut1CorePorts []*ondatra.Port
	for i, group := range corePorts {
		for _, name := range group {
			dut1CorePortGroups[i] = append(dut1CorePortGroups[i], dut1.Port(t, name))
			dut2CorePortGroups[i] = append(dut2CorePortGroups[i], dut2.Port(t, name))
			dut1CorePorts = append(dut1CorePorts, dut1.Port(t, name))
		}
	}
	// Port5 on each DUT connects to ATE; DUT1 carries MACsec.
	dut1CustPort := dut1.Port(t, custPorts[0])
	dut2CustPort := dut2.Port(t, custPorts[0])

	// DUT-specific attributes for core LAGs.
	dut1PortAttrs := []attrs.Attributes{dut1CoreIntf1, dut1CoreIntf2}
	dut2PortAttrs := []attrs.Attributes{dut2CoreIntf1, dut2CoreIntf2}

	// Create all VRFs before configuring interfaces.
	createVRFs(t, dut1, []string{ateVRF, tunnelVRF})
	createVRFs(t, dut2, []string{ateVRF, tunnelVRF})

	// Configure DUTs: generate aggregates for core LAGs in TUNNEL_VRF.
	configureDUT(t, dut1, dut1CorePortGroups, dut1PortAttrs, "")
	configureDUT(t, dut1, [][]*ondatra.Port{{dut1CustPort}}, []attrs.Attributes{dut1CustIntf}, ateVRF)

	configureDUT(t, dut2, dut2CorePortGroups, dut2PortAttrs, "")
	configureDUT(t, dut2, [][]*ondatra.Port{{dut2CustPort}}, []attrs.Attributes{dut2CustIntf}, ateVRF)

	// Configure loopback interfaces for IPSec tunnel endpoints.
	configureLoopback(t, dut1, loopbackIfName, dut1LoopbackIPv6, loopbackPrefixLen, true)
	configureLoopback(t, dut2, loopbackIfName, dut2LoopbackIPv6, loopbackPrefixLen, true)

	batchMACsec := cfgplugins.ConfigureMACsec(t, dut1, cfgplugins.MACsecCfg{
		IntfName:    dut1CustPort.Name(),
		ProfileName: "macSecProfile",
		CKN:         ckn,
		CAK:         cak,
		FallbackCKN: fallbackCkn,
		FallbackCAK: fallbackCak,
	})
	batchMACsec.Set(t, dut1)

	batch5 := cfgplugins.ConfigureIPSecTunnel(t, dut1, cfgplugins.IPSecTunnelCfg{
		TunnelName:  tunnelIfName,
		Description: "IPsec Tunnel Pair 1 to DUT2",
		LocalFQDN:   dut1FQDN,
		RemoteFQDN:  dut2FQDN,
		TunnelIPv4:  dut1TunnelIPv4CIDR,
		TunnelIPv6:  dut1TunnelIPv6CIDR,
		TunnelSrc:   dut1LoopbackIPv6,
		TunnelDst:   dut2LoopbackIPv6,
		TunnelVRF:   tunnelVRF,
	})
	batch5.Set(t, dut1)

	batch6 := cfgplugins.ConfigureIPSecTunnel(t, dut2, cfgplugins.IPSecTunnelCfg{
		TunnelName:  tunnelIfName,
		Description: "IPsec Tunnel Pair 1 to DUT1",
		LocalFQDN:   dut2FQDN,
		RemoteFQDN:  dut1FQDN,
		TunnelIPv4:  dut2TunnelIPv4CIDR,
		TunnelIPv6:  dut2TunnelIPv6CIDR,
		TunnelSrc:   dut2LoopbackIPv6,
		TunnelDst:   dut1LoopbackIPv6,
		TunnelVRF:   tunnelVRF,
	})
	batch6.Set(t, dut2)

	configureStaticRoutes(t, dut1, []staticRoute{
		{Prefix: ate1IPv4Prefix, NextHop: ate1LagConfig.IPv4, VRF: tunnelVRF, EgressVRF: ateVRF},
		{Prefix: ate2IPv4Prefix, NextHop: dut2TunnelIPv4NH, VRF: ateVRF, EgressVRF: tunnelVRF},
		{Prefix: ate1IPv6Prefix, NextHop: ate1LagConfig.IPv6, VRF: tunnelVRF, EgressVRF: ateVRF},
		{Prefix: ate2IPv6Prefix, NextHop: dut2TunnelIPv6NH, VRF: ateVRF, EgressVRF: tunnelVRF},
		{Prefix: dut2LoopbackPfx, NextHop: dut2CoreIntf2.IPv6},
		{Prefix: dut2LoopbackPfx, NextHop: dut2CoreIntf1.IPv6},
	})

	configureStaticRoutes(t, dut2, []staticRoute{
		{Prefix: ate2IPv4Prefix, NextHop: ate2LagConfig.IPv4, VRF: tunnelVRF, EgressVRF: ateVRF},
		{Prefix: ate1IPv4Prefix, NextHop: dut1TunnelIPv4NH, VRF: ateVRF, EgressVRF: tunnelVRF},
		{Prefix: ate2IPv6Prefix, NextHop: ate2LagConfig.IPv6, VRF: tunnelVRF, EgressVRF: ateVRF},
		{Prefix: ate1IPv6Prefix, NextHop: dut1TunnelIPv6NH, VRF: ateVRF, EgressVRF: tunnelVRF},
		{Prefix: dut1LoopbackPfx, NextHop: dut1CoreIntf2.IPv6},
		{Prefix: dut1LoopbackPfx, NextHop: dut1CoreIntf1.IPv6},
	})

	// Step: Configure ATE topology and flows.
	top := configureATE(t)
	// Enable capture should be part of setconfig
	packetvalidationhelpers.ConfigurePacketCapture(t, top, MacsecPacketValidation)
	otg.PushConfig(t, top)
	otg.StartProtocols(t)

	waitForOTGMACSecUp(t, ate, macsecPeerName, lagUpTimeout)
	waitForOTGLAGUP(t, ate, ate1LagName, 1, lagUpTimeout)
	waitForOTGLAGUP(t, ate, ate2LagName, 1, lagUpTimeout)

	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

	// Start capture before running traffic.
	cs := packetvalidationhelpers.StartCapture(t, ate)

	// The subtests below map 1:1 to the IPSEC-1.1.x sections in the README.
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			// Verify base operational readiness.
			name: "IPSEC-1.1.1: Verify Base IPv4 & IPv6 traffic forwarding with ipsec",
			fn: func(t *testing.T) {
				verifyTunnelOperStatus(t, dut1, tunnelIfName, oc.Interface_OperStatus_UP, lagUpTimeout)
				verifyTunnelOperStatus(t, dut2, tunnelIfName, oc.Interface_OperStatus_UP, lagUpTimeout)

				otg.StartTraffic(t)

				time.Sleep(trafficWaitTime)

				otg.StopTraffic(t)

				// Stop capture after traffic.
				packetvalidationhelpers.StopCapture(t, ate, cs)

				if err := verifyTraffic(t, ate, top, flowIPv4Fwd, true); err != nil {
					t.Errorf("traffic verification failed: %v", err)
				}
				if err := verifyTraffic(t, ate, top, flowIPv4Bwd, true); err != nil {
					t.Errorf("traffic verification failed: %v", err)
				}
				if err := verifyTraffic(t, ate, top, flowIPv6Fwd, true); err != nil {
					t.Errorf("traffic verification failed: %v", err)
				}
				if err := verifyTraffic(t, ate, top, flowIPv6Bwd, true); err != nil {
					t.Errorf("traffic verification failed: %v", err)
				}

				// Validate customer traffic is MACsec-encrypted.
				if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, MacsecPacketValidation); err != nil {
					t.Errorf("captureAndValidatePackets() MACsec: %v", err)
				}

				for _, dscp := range []uint32{10, 20, 30} {
					if err := verifyDSCPPreservation(t, ate, dscp); err != nil {
						t.Errorf("dscp preservation verification failed dscp=%d: %v", dscp, err)
					}
				}

				for _, flowLabel := range []uint32{10, 1000} {
					if err := verifyFlowLabelPreservation(t, ate, flowLabel); err != nil {
						t.Errorf("flow label preservation verification failed flowLabel=%d: %v", flowLabel, err)
					}
				}

				top = configureATE(t)

				// Restore original ATE topology after per-flow-label captures.
				otg.PushConfig(t, top)
				otg.StartProtocols(t)
				otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
				otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
			},
		},
		{
			name: "IPSEC-1.1.2: Verify Line-Rate IPv4 Connectivity over a Single Tunnel",
			fn: func(t *testing.T) {
				pre := readMemberOutPkts(t, dut1, dut1CorePorts)
				runTrafficWindow(t, otg, 15*time.Second)
				if err := verifyTraffic(t, ate, top, flowIPv4Fwd, true); err != nil {
					t.Errorf("traffic verification failed: %v", err)
				}
				if err := verifyDUTDUTLoadBalance(t, dut1, dut1CorePorts, pre, 0.25, false); err != nil {
					t.Errorf("load balance verification failed: %v", err)
				}
			},
		},
		{
			name: "IPSEC-1.1.3: Verify Line-Rate IPv6 Connectivity over a Single Tunnel",
			fn: func(t *testing.T) {
				pre := readMemberOutPkts(t, dut1, dut1CorePorts)
				runTrafficWindow(t, otg, 15*time.Second)
				if err := verifyTraffic(t, ate, top, flowIPv6Fwd, true); err != nil {
					t.Errorf("traffic verification failed: %v", err)
				}
				if err := verifyDUTDUTLoadBalance(t, dut1, dut1CorePorts, pre, 0.25, false); err != nil {
					t.Errorf("load balance verification failed: %v", err)
				}
			},
		},
		{
			name: "IPSEC-1.1.4: Verify Hitless SA Renegotation (New Key Generation)",
			fn: func(t *testing.T) {
				verifyTunnelOperStatus(t, dut1, tunnelIfName, oc.Interface_OperStatus_UP, lagUpTimeout)

				batchSA1 := cfgplugins.SetShortSALifetime(t, dut1, cfgplugins.IPSecTunnelCfg{SAPolicy: "SA_POLICY_1"}, 10)
				batchSA1.Set(t, dut1)
				batchSA2 := cfgplugins.SetShortSALifetime(t, dut2, cfgplugins.IPSecTunnelCfg{SAPolicy: "SA_POLICY_1"}, 10)
				batchSA2.Set(t, dut2)
				spisInitial := readChildSPIs(t, dut1)
				if len(spisInitial) == 0 {
					t.Fatal("no child SPI values found before SA renegotiation; cannot verify key rotation")
				}
				t.Logf("Initial SPIs: %v", spisInitial)

				otg.StartTraffic(t)

				// Poll every 2 minutes for up to 10 minutes until the SPI changes.
				const (
					pollInterval = 2 * time.Minute
					maxWait      = 10 * time.Minute
				)
				initialSet := make(map[uint64]bool)
				for _, s := range spisInitial {
					initialSet[s] = true
				}
				var spisFinal []uint64
				spiChanged := false
				start := time.Now()
				deadline := start.Add(maxWait)
				for time.Now().Before(deadline) {
					time.Sleep(pollInterval)
					spisFinal = readChildSPIs(t, dut1)
					t.Logf("Polled SPIs at %v elapsed: %v", time.Since(start).Round(time.Second), spisFinal)
					for _, s := range spisFinal {
						if !initialSet[s] {
							spiChanged = true
							break
						}
					}
					if spiChanged {
						t.Logf("SA Regenotiation Done: SPI changed after %v", time.Since(start).Round(time.Second))
						break
					}
				}

				otg.StopTraffic(t)

				// Verify SPI values have changed after SA renegotiation.
				if len(spisFinal) == 0 {
					t.Fatal("no child SPI values found after SA renegotiation")
				}
				t.Logf("Final SPIs: %v", spisFinal)
				if !spiChanged {
					t.Errorf("spi did not change after SA renegotiation within %v: initial=%v, final=%v", maxWait, spisInitial, spisFinal)
				}

				if err := verifyTraffic(t, ate, top, flowIPv4Fwd, true); err != nil {
					t.Errorf("traffic verification failed: %v", err)
				}
				if err := verifyTraffic(t, ate, top, flowIPv4Bwd, true); err != nil {
					t.Errorf("traffic verification failed: %v", err)
				}
			},
		},
		{
			name: "IPSEC-1.1.5: Verify Hitless IKE Renegotation",
			fn: func(t *testing.T) {
				verifyTunnelOperStatus(t, dut1, tunnelIfName, oc.Interface_OperStatus_UP, lagUpTimeout)
				batchIKE1 := cfgplugins.SetShortIKELifetime(t, dut1, cfgplugins.IPSecTunnelCfg{IKEPolicy: "IKE_POLICY_1"}, 10)
				batchIKE1.Set(t, dut1)
				batchIKE2 := cfgplugins.SetShortIKELifetime(t, dut2, cfgplugins.IPSecTunnelCfg{IKEPolicy: "IKE_POLICY_1"}, 10)
				batchIKE2.Set(t, dut2)
				// Read initial SPI values before renegotiation.
				spisInitial := readChildSPIs(t, dut1)
				if len(spisInitial) == 0 {
					t.Fatal("no child SPI values found before IKE renegotiation; cannot verify key rotation")
				}
				t.Logf("Initial SPIs: %v", spisInitial)

				otg.StartTraffic(t)

				// Poll every 2 minutes for up to 10 minutes until the SPI changes.
				const (
					pollInterval = 2 * time.Minute
					maxWait      = 10 * time.Minute
				)
				initialSet := make(map[uint64]bool)
				for _, s := range spisInitial {
					initialSet[s] = true
				}
				var spisFinal []uint64
				spiChanged := false
				start := time.Now()
				deadline := start.Add(maxWait)
				for time.Now().Before(deadline) {
					time.Sleep(pollInterval)
					spisFinal = readChildSPIs(t, dut1)
					t.Logf("Polled SPIs at %v elapsed: %v", time.Since(start).Round(time.Second), spisFinal)
					for _, s := range spisFinal {
						if !initialSet[s] {
							spiChanged = true
							break
						}
					}
					if spiChanged {
						t.Logf("IKE Renegotiation Done: SPI changed after %v", time.Since(start).Round(time.Second))
						break
					}
				}

				otg.StopTraffic(t)

				// Verify SPI values have changed after SA renegotiation.
				if len(spisFinal) == 0 {
					t.Fatal("no child SPI values found after IKE renegotiation")
				}
				t.Logf("Final SPIs: %v", spisFinal)
				if !spiChanged {
					t.Errorf("SPI did not change after IKE renegotiation within %v: initial=%v, final=%v", maxWait, spisInitial, spisFinal)
				}

				verifyTraffic(t, ate, top, flowIPv4Fwd, true)
				verifyTraffic(t, ate, top, flowIPv4Bwd, true)
			},
		},
		{
			name: "IPSEC-1.1.6: Verify DPD / dead-peer detection",
			fn: func(t *testing.T) {
				switch dut1.Vendor() {
				case ondatra.ARISTA:
					// Arista does not support ACLs on loopback interfaces, so skip this subtest.
					t.Skip("Skipping DPD test for Arista DUT1; ACLs on loopback not supported")
				}
				configureDualTunnels(t, dut1, dut2)
				batchDPD1 := cfgplugins.ConfigureDPD(t, dut1, cfgplugins.IPSecTunnelCfg{TunnelName: tunnelIfName}, 2, 10)
				batchDPD1.Set(t, dut1)
				batchDPD2 := cfgplugins.ConfigureDPD(t, dut2, cfgplugins.IPSecTunnelCfg{TunnelName: tunnelIfName}, 2, 10)
				batchDPD2.Set(t, dut2)
				t.Cleanup(func() {
					batchDel1 := cfgplugins.DeleteTunnelInterface(t, dut1, tunnel2IfName)
					batchDel1.Set(t, dut1)
					batchDel2 := cfgplugins.DeleteTunnelInterface(t, dut2, tunnel2IfName)
					batchDel2.Set(t, dut2)
					configureBaseSingleTunnel(t, dut1, dut2)
				})
				verifyTunnelOperStatus(t, dut1, tunnelIfName, oc.Interface_OperStatus_UP, lagUpTimeout)
				verifyTunnelOperStatus(t, dut1, tunnel2IfName, oc.Interface_OperStatus_UP, lagUpTimeout)

				pre := readMemberOutPkts(t, dut1, dut1CorePorts)
				runTrafficWindow(t, otg, 20*time.Second)
				if err := verifyTraffic(t, ate, top, flowIPv4Fwd, true); err != nil {
					t.Errorf("traffic verification failed: %v", err)
				}
				if err := verifyDUTDUTLoadBalance(t, dut1, dut1CorePorts, pre, 0.30, false); err != nil {
					t.Errorf("load balance verification failed: %v", err)
				}

				runTrafficWindow(t, otg, 20*time.Second)
				verifyTraffic(t, ate, top, flowIPv4Fwd, true)
			},
		},
		{
			name: "IPSEC-1.1.7: Invalid Tunnel - Mismatch on Key",
			fn: func(t *testing.T) {
				configureDualTunnels(t, dut1, dut2)
				t.Cleanup(func() {
					// Remove the second tunnel configured by configureDualTunnels and
					// restore the base single-tunnel setup for subsequent subtests.
					batchDelTun1 := cfgplugins.DeleteTunnelInterface(t, dut1, tunnel2IfName)
					batchDelTun1.Set(t, dut1)
					batchDelTun2 := cfgplugins.DeleteTunnelInterface(t, dut2, tunnel2IfName)
					batchDelTun2.Set(t, dut2)
					configureBaseSingleTunnel(t, dut1, dut2)
				})

				// Pre-check: both tunnels are UP, traffic passes, and ECMP is balanced
				// across the physical member links.
				verifyTunnelOperStatus(t, dut1, tunnelIfName, oc.Interface_OperStatus_UP, lagUpTimeout)
				verifyTunnelOperStatus(t, dut1, tunnel2IfName, oc.Interface_OperStatus_UP, lagUpTimeout)

				preBoth := readMemberOutPkts(t, dut1, dut1CorePorts)
				runTrafficWindow(t, otg, 20*time.Second)
				if err := verifyTraffic(t, ate, top, flowIPv4Fwd, true); err != nil {
					t.Errorf("traffic verification failed: %v", err)
				}
				if err := verifyDUTDUTLoadBalance(t, dut1, dut1CorePorts, preBoth, 0.30, false); err != nil {
					t.Errorf("load balance verification failed: %v", err)
				}

				// Introduce a key mismatch on Tunnel2 only; Tunnel1 stays UP.
				batchKeyMismatch := cfgplugins.SetMismatchedKey(t, dut2, cfgplugins.IPSecTunnelCfg{TunnelName: tunnel2IfName, Profile: "IPSEC_PROFILE_2"})
				batchKeyMismatch.Set(t, dut2)
				verifyTunnelOperStatus(t, dut1, tunnel2IfName, oc.Interface_OperStatus_DOWN, lagUpTimeout)

				// Post-check: traffic still passes and ECMP remains balanced across the
				// physical member links via the single healthy tunnel.
				preSingle := readMemberOutPkts(t, dut1, dut1CorePorts)
				runTrafficWindow(t, otg, 20*time.Second)
				verifyTraffic(t, ate, top, flowIPv4Fwd, true)
				verifyDUTDUTLoadBalance(t, dut1, dut1CorePorts, preSingle, 0.30, false)
			},
		},
		{
			name: "IPSEC-1.1.8: Invalid Tunnel - Mismatch on Cipher Algorithm",
			fn: func(t *testing.T) {
				configureDualTunnels(t, dut1, dut2)
				t.Cleanup(func() {
					// Remove the second tunnel configured by configureDualTunnels and
					// restore the base single-tunnel setup for subsequent subtests.
					batchDelTun1 := cfgplugins.DeleteTunnelInterface(t, dut1, tunnel2IfName)
					batchDelTun1.Set(t, dut1)
					batchDelTun2 := cfgplugins.DeleteTunnelInterface(t, dut2, tunnel2IfName)
					batchDelTun2.Set(t, dut2)
					configureBaseSingleTunnel(t, dut1, dut2)
				})

				// Pre-check: both tunnels are UP, traffic passes, and ECMP is balanced
				// across the physical member links.
				verifyTunnelOperStatus(t, dut1, tunnelIfName, oc.Interface_OperStatus_UP, lagUpTimeout)
				verifyTunnelOperStatus(t, dut1, tunnel2IfName, oc.Interface_OperStatus_UP, lagUpTimeout)

				preBoth := readMemberOutPkts(t, dut1, dut1CorePorts)
				runTrafficWindow(t, otg, 20*time.Second)
				verifyTraffic(t, ate, top, flowIPv4Fwd, true)
				verifyDUTDUTLoadBalance(t, dut1, dut1CorePorts, preBoth, 0.30, false)

				// Introduce cipher mismatch on Tunnel2; Tunnel1 stays UP.
				batchCipherMismatch := cfgplugins.SetMismatchedCipher(t, dut2, cfgplugins.IPSecTunnelCfg{TunnelName: tunnel2IfName, SAPolicy: "SA_POLICY_2"})
				batchCipherMismatch.Set(t, dut2)
				verifyTunnelOperStatus(t, dut1, tunnel2IfName, oc.Interface_OperStatus_DOWN, lagUpTimeout)

				// Post-check: traffic still passes and ECMP remains balanced across the
				// physical member links via the single healthy tunnel.
				preSingle := readMemberOutPkts(t, dut1, dut1CorePorts)
				runTrafficWindow(t, otg, 20*time.Second)
				verifyTraffic(t, ate, top, flowIPv4Fwd, true)
				verifyDUTDUTLoadBalance(t, dut1, dut1CorePorts, preSingle, 0.30, false)
			},
		},
		{
			name: "IPSEC-1.1.9: Verify IPSec shared-key key rotation",
			fn: func(t *testing.T) {
				runTrafficWindow(t, otg, 10*time.Second)
				if err := verifyTraffic(t, ate, top, flowIPv4Fwd, true); err != nil {
					t.Errorf("traffic verification failed: %v", err)
				}

				cfgplugins.RotateSharedKey(t, dut1, cfgplugins.IPSecTunnelCfg{TunnelName: tunnelIfName}, "052B0A1A3F51")
				cfgplugins.RotateSharedKey(t, dut2, cfgplugins.IPSecTunnelCfg{TunnelName: tunnelIfName}, "052B0A1A3F51")
				t.Cleanup(func() {
					configureBaseSingleTunnel(t, dut1, dut2)
				})

				verifyTunnelOperStatus(t, dut1, tunnelIfName, oc.Interface_OperStatus_UP, lagUpTimeout)
				runTrafficWindow(t, otg, 10*time.Second)
				if err := verifyTraffic(t, ate, top, flowIPv4Fwd, true); err != nil {
					t.Errorf("traffic verification failed: %v", err)
				}
			},
		},
		{
			name: "IPSEC-1.1.10: Verify Flow-Label Hash-Disablement",
			fn: func(t *testing.T) {
				switch dut1.Vendor() {
				case ondatra.ARISTA:
					// Arista does not support flow-label hash disablement, so skip this subtest.
					t.Skip("Skipping Flow-Label Hash-Disablement test for Arista DUT; feature not supported")
				}
				cfgplugins.DisableFlowLabelHash(t, dut1, cfgplugins.IPSecTunnelCfg{TunnelName: tunnelIfName})
				t.Cleanup(func() {
					configureBaseSingleTunnel(t, dut1, dut2)
				})

				pre := readMemberOutPkts(t, dut1, dut1CorePorts)
				runTrafficWindow(t, otg, 20*time.Second)
				verifyTraffic(t, ate, top, flowIPv6Fwd, true)
				verifyDUTDUTLoadBalance(t, dut1, dut1CorePorts, pre, 0.35, true)
			},
		},
		{
			name: "IPSEC-1.1.11: Verify Tunnel Re-Pathing upon Failure",
			fn: func(t *testing.T) {
				preAll := readMemberOutPkts(t, dut1, dut1CorePorts)
				runTrafficWindow(t, otg, 15*time.Second)
				verifyTraffic(t, ate, top, flowIPv4Fwd, true)
				verifyDUTDUTLoadBalance(t, dut1, dut1CorePorts, preAll, 0.30, false)

				downCfg := &oc.Interface{Name: ygot.String(dut1CorePorts[0].Name())}
				downCfg.Enabled = ygot.Bool(false)
				gnmi.Update(t, dut1, gnmi.OC().Interface(dut1CorePorts[0].Name()).Config(), downCfg)
				t.Cleanup(func() {
					upCfg := &oc.Interface{Name: ygot.String(dut1CorePorts[0].Name())}
					upCfg.Enabled = ygot.Bool(true)
					gnmi.Update(t, dut1, gnmi.OC().Interface(dut1CorePorts[0].Name()).Config(), upCfg)
				})

				preRemain := readMemberOutPkts(t, dut1, dut1CorePorts[1:])
				runTrafficWindow(t, otg, 20*time.Second)
				verifyTraffic(t, ate, top, flowIPv4Fwd, true)
				verifyDUTDUTLoadBalance(t, dut1, dut1CorePorts[1:], preRemain, 0.35, false)
				verifyTunnelOperStatus(t, dut1, tunnelIfName, oc.Interface_OperStatus_UP, lagUpTimeout)
			},
		},
		{
			name: "IPSEC-1.1.12: Verify QoS: Control Plane survives with Dataplane Congestion",
			fn: func(t *testing.T) {
				switch dut1.Vendor() {
				case ondatra.ARISTA:
					t.Skip("Skipping QoS test for Arista DUT; feature not supported")
				}
				const renewalWindow = 75 * time.Second

				for _, p := range dut1CorePorts[1:] {
					dis := &oc.Interface{Name: ygot.String(p.Name())}
					dis.Enabled = ygot.Bool(false)
					gnmi.Update(t, dut1, gnmi.OC().Interface(p.Name()).Config(), dis)
				}
				t.Cleanup(func() {
					for _, p := range dut1CorePorts[1:] {
						en := &oc.Interface{Name: ygot.String(p.Name())}
						en.Enabled = ygot.Bool(true)
						gnmi.Update(t, dut1, gnmi.OC().Interface(p.Name()).Config(), en)
					}
					configureBaseSingleTunnel(t, dut1, dut2)
				})

				configureQoSClassification(t, dut1)
				batchSA1 := cfgplugins.SetShortSALifetime(t, dut1, cfgplugins.IPSecTunnelCfg{SAPolicy: "SA_POLICY_1"}, 60)
				batchSA1.Set(t, dut1)
				batchSA2 := cfgplugins.SetShortSALifetime(t, dut2, cfgplugins.IPSecTunnelCfg{SAPolicy: "SA_POLICY_1"}, 60)
				batchSA2.Set(t, dut2)
				batchIKE1 := cfgplugins.SetShortIKELifetime(t, dut1, cfgplugins.IPSecTunnelCfg{IKEPolicy: "IKE_POLICY_1"}, 60)
				batchIKE1.Set(t, dut1)
				batchIKE2 := cfgplugins.SetShortIKELifetime(t, dut2, cfgplugins.IPSecTunnelCfg{IKEPolicy: "IKE_POLICY_1"}, 60)
				batchIKE2.Set(t, dut2)
				batchDPD1 := cfgplugins.ConfigureDPD(t, dut1, cfgplugins.IPSecTunnelCfg{TunnelName: tunnelIfName}, 2, 10)
				batchDPD1.Set(t, dut1)
				batchDPD2 := cfgplugins.ConfigureDPD(t, dut2, cfgplugins.IPSecTunnelCfg{TunnelName: tunnelIfName}, 2, 10)
				batchDPD2.Set(t, dut2)
				// At least one SA/IKE renewal expected within 75s window; verify tunnel stays UP.
				t.Log("short SA/IKE lifetimes applied; validating tunnel stays UP through renewal window")

				verifyTunnelOperStatus(t, dut1, tunnelIfName, oc.Interface_OperStatus_UP, lagUpTimeout)
				verifyTunnelOperStatus(t, dut2, tunnelIfName, oc.Interface_OperStatus_UP, lagUpTimeout)
				otg.StartTraffic(t)
				verifyTunnelOperStatusStaysUp(t, dut1, tunnelIfName, renewalWindow)
				verifyTunnelOperStatusStaysUp(t, dut2, tunnelIfName, renewalWindow)
				otg.StopTraffic(t)
				verifyTunnelOperStatus(t, dut1, tunnelIfName, oc.Interface_OperStatus_UP, lagUpTimeout)
				verifyTunnelOperStatus(t, dut2, tunnelIfName, oc.Interface_OperStatus_UP, lagUpTimeout)
				verifyTraffic(t, ate, top, flowIPv4Fwd, true)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, tc.fn)
	}
}
