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

package ipsec_scale_test

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/iputil"
	packetvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/packetvalidationhelpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
)

const (
	vlanID = 10

	// ATE OTG topology names.
	ate1LagName = "Lag1"
	ate2LagName = "Lag2"
	ate1DevName = "d1"
	ate2DevName = "d2"

	// VRF names.
	ateVRF    = "ATE_VRF"
	tunnelVRF = "TUNNEL_VRF"

	// Interface names.
	tunnelIfName   = "Tunnel1"
	loopbackIfName = "Loopback0"

	// Loopback IPv6 addresses used as IPSec tunnel endpoints (RFC 3849).
	dut1LoopbackIPv6  = "2001:db8:3::1"
	dut2LoopbackIPv6  = "2001:db8:4::1"
	loopbackPrefixLen = 128

	// Static route destination prefixes.
	ate1IPv4Prefix = "192.0.2.0/30"
	ate2IPv4Prefix = "203.0.113.0/30"
	ate1IPv6Prefix = "2001:db8:1::0/126"
	ate2IPv6Prefix = "2001:db8:2::0/126"

	// OTG MACsec peer name.
	macsecPeerName = "Peer A"

	// OTG flow names.
	flowIPv4Fwd = "Flow-IPv4-Fwd"
	flowIPv4Bwd = "Flow-IPv4-Bwd"
	flowIPv6Fwd = "Flow-IPv6-Fwd"
	flowIPv6Bwd = "Flow-IPv6-Bwd"

	// Traffic generation parameters.
	trafficPPS = 100

	// numTunnels is the number of parallel IPSec tunnels between the DUT pair for
	// the scale test (IPSEC-1.2.x), all sharing TUNNEL_VRF with ECMP'd traffic.
	numTunnels = 256

	// Timeout durations.
	lagUpTimeout    = 2 * time.Minute
	trafficWaitTime = 30 * time.Second

	// tunnelUpTimeout allows extra time for the many IPSec tunnels to finish IKE
	// negotiation and come UP at scale.
	tunnelUpTimeout = 10 * time.Minute
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

	// DUT port groupings (reference: mpls_gre_ipv4_encap_test.go).
	// custPorts are the DUT customer/ATE-facing ports (MACsec edge).
	custPorts = []string{"port5"}
	// corePorts are the DUT-to-DUT core transport ports, grouped per LAG.
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

	// DUT customer-facing (ATE-facing) interface configurations (RFC 5737 test networks).
	// DUT1: VLAN 10 with MACsec
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
	// DUT2: No VLAN
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

	// DUT core interface configurations (IPv6-only for DUT-to-DUT links per RFC 5737 test networks)
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
	// MacsecValidations lists the validations performed on the customer-facing
	// capture to confirm traffic is MACsec-encrypted.
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

	// Keep subinterface 0 present with MTU enabled. Some devices require this
	// base subinterface to exist even when traffic is configured on a tagged subinterface.
	s0 := i.GetOrCreateSubinterface(0)
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

// assignAggregateToVRF assigns an aggregate/subinterface to the requested VRF using OC,
// keeping VRF assignment separate from interface modelling.
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

	// Configure aggregate interface first (some devices validate that the
	// interface exists before accepting LACP config), then LACP, then members.
	lacp := &oc.Lacp_Interface{Name: ygot.String(lagName)}
	lacp.LacpMode = oc.Lacp_LacpActivityType_ACTIVE

	agg := &oc.Interface{Name: ygot.String(lagName)}
	// Only set high-level interface fields here; avoid creating subinterfaces
	// or assigning IPs until the aggregate and members exist.
	agg.Description = ygot.String(a.Desc)
	if deviations.InterfaceEnabled(dut) {
		agg.Enabled = ygot.Bool(true)
	}
	// Ensure lag-type is present so member ports can reference this aggregate.
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_LACP
	agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag

	// First transaction: create the aggregate (without subinterfaces/IPs),
	// create the LACP entry and configure member ports.
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

	// Assign VRF before programming IP addresses (moving into a VRF afterwards may
	// clear the address). Use a.ID so tagged ATE-facing subinterfaces map correctly.
	assignAggregateToVRF(t, dut, lagName, a.ID, vrfName)

	if deviations.AggregateAtomicUpdate(dut) {
		post := &gnmi.SetBatch{}
		full := &oc.Interface{Name: ygot.String(lagName)}
		full.GetOrCreateAggregation().LagType = agg.GetOrCreateAggregation().GetLagType()
		full.Type = agg.Type
		// Use helper to populate subinterface(s) and addresses.
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

// createVRFs creates VRFs via OC, or via a single atomic CLI transaction on Arista
// where the routing-enable commands have no OC equivalent.
func createVRFs(t *testing.T, dut *ondatra.DUTDevice, vrfNames []string) {
	t.Helper()
	if len(vrfNames) == 0 {
		return
	}
	if deviations.IpRoutingInVrfOcUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			// Arista needs vrf instance + ip routing + ipv6 unicast-routing in one
			// atomic CLI block; OC cannot express the routing-enable commands.
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
		// All other vendors: create VRF instance using OpenConfig.
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

// configureStaticRoutes configures static routes via OpenConfig for named-VRF routes
// without egress-vrf, and via CLI for egress-vrf or default-VRF routes (Arista).
func configureStaticRoutes(t *testing.T, dut *ondatra.DUTDevice, routes []staticRoute) {
	t.Helper()

	// Group named-VRF routes without egress-vrf for OC configuration.
	ocRoutesByVRF := make(map[string][]staticRoute)
	for _, r := range routes {
		if r.EgressVRF == "" && r.VRF != "" {
			ocRoutesByVRF[r.VRF] = append(ocRoutesByVRF[r.VRF], r)
		}
	}

	// Configure named-VRF plain next-hop routes via OpenConfig.
	for vrfName, vrfRoutes := range ocRoutesByVRF {
		proto := &oc.NetworkInstance_Protocol{
			Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
			Name:       ygot.String(deviations.StaticProtocolName(dut)),
		}
		for _, r := range vrfRoutes {
			sr := proto.GetOrCreateStatic(r.Prefix)
			sr.Prefix = ygot.String(r.Prefix)
			nh := sr.GetOrCreateNextHop("0")
			nh.Index = ygot.String("0")
			nh.NextHop = oc.UnionString(r.NextHop)
		}
		sp := gnmi.OC().NetworkInstance(vrfName).Protocol(
			oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
		gnmi.Update(t, dut, sp.Config(), proto)
	}

	if deviations.StaticRouteInVrfOcUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			// Configure egress-vrf/default-VRF routes via CLI, batched into a single
			// gNMI CLI Set to avoid one Set per route (expensive at scale).
			var cliLines []string
			for _, r := range routes {
				if r.EgressVRF == "" && r.VRF != "" {
					continue // already handled via OC above
				}
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
					// Default VRF, plain next-hop — no vrf qualifier.
					cli = fmt.Sprintf("%s route %s %s", ipType, r.Prefix, r.NextHop)
				default:
					// EgressVRF set but VRF is empty (edge case: egress from default VRF).
					cli = fmt.Sprintf("%s route %s egress-vrf %s %s",
						ipType, r.Prefix, r.EgressVRF, r.NextHop)
				}
				cliLines = append(cliLines, cli)
			}
			if len(cliLines) > 0 {
				helpers.GnmiCLIConfig(t, dut, strings.Join(cliLines, "\n"))
			}
		}
	} else {
		// Configure via OC: routes with egress-vrf or in the default VRF.
		for _, r := range routes {
			if r.EgressVRF == "" && r.VRF != "" {
				continue // already handled via OC above
			}
			vrfName := r.VRF
			if vrfName == "" {
				vrfName = deviations.DefaultNetworkInstance(dut)
			}
			proto := &oc.NetworkInstance_Protocol{
				Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
				Name:       ygot.String(deviations.StaticProtocolName(dut)),
			}
			sr := proto.GetOrCreateStatic(r.Prefix)
			sr.Prefix = ygot.String(r.Prefix)
			nh := sr.GetOrCreateNextHop("0")
			nh.Index = ygot.String("0")
			nh.NextHop = oc.UnionString(r.NextHop)
			if r.EgressVRF != "" {
				nh.NextNetworkInstance = ygot.String(r.EgressVRF)
			}
			sp := gnmi.OC().NetworkInstance(vrfName).Protocol(
				oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
			gnmi.Update(t, dut, sp.Config(), proto)
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

	// VRF should already be created by createVRF() before calling this
	// Just configure the interfaces without VRF creation

	for i := range portGroups {
		// Generate a unique aggregate ID per DUT per LAG.
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

	s0 := i.GetOrCreateSubinterface(0)

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

	// Port mapping expected from testbed:
	// - ATE port1 <-> DUT1 port5 (with MACSec on DUT side)
	// - ATE port2 <-> DUT2 port5
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

	// -- Macsec Config
	macsec1 := lagPort1.Macsec()
	secy1 := macsec1.SecureEntity().SetName(macsecPeerName)
	secy1Encapsulation := secy1.DataPlane().Encapsulation()
	secy1Encapsulation.CryptoEngine().EncryptDecrypt().HardwareAcceleration().InlineCrypto()
	// -- MKA Config
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
	watchTimeout := 2 * time.Minute

	watch := gnmi.Watch(t, ate.OTG(), flowPath, watchTimeout, func(val *ygnmi.Value[*otgtelemetry.Flow]) bool {
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
	})

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

func waitForOTGLAGUP(t *testing.T, ate *ondatra.ATEDevice, lagName string, wantMembersUp uint64, timeout time.Duration) {
	t.Helper()

	otg := ate.OTG()

	t.Logf("Waiting for OTG LAG %s to be UP with %d member(s)", lagName, wantMembersUp)

	watch := gnmi.Watch(
		t,
		otg,
		gnmi.OTG().Lag(lagName).State(),
		timeout,
		func(val *ygnmi.Value[*otgtelemetry.Lag]) bool {
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
		},
	)

	if _, ok := watch.Await(t); !ok {
		finalOper := gnmi.Get(t, otg, gnmi.OTG().Lag(lagName).OperStatus().State())
		finalMembers := gnmi.Get(t, otg, gnmi.OTG().Lag(lagName).Counters().MemberPortsUp().State())

		t.Fatalf("OTG LAG %s did not become ready within %v: final oper-status=%v member-ports-up=%d (want oper-status=UP, member-ports-up=%d)",
			lagName, timeout, finalOper, finalMembers, wantMembersUp)
	}
}

func waitForOTGMACSecUp(t *testing.T, ate *ondatra.ATEDevice, ifName string, timeout time.Duration) {
	t.Helper()

	otg := ate.OTG()

	t.Logf("Waiting for OTG MACsec session on %s to be UP", ifName)

	watch := gnmi.Watch(
		t,
		otg,
		gnmi.OTG().Macsec().Interface(ifName).SessionState().State(),
		timeout,
		func(val *ygnmi.Value[otgtelemetry.E_Interface_SessionState]) bool {
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
		},
	)

	if _, ok := watch.Await(t); !ok {
		finalState := gnmi.Get(t, otg, gnmi.OTG().Macsec().Interface(ifName).SessionState().State())
		t.Fatalf("MACsec session on %s did not come UP within %v, final state=%v",
			ifName, timeout, finalState)
	}
}

// configureScaledTunnels configures numTunnels parallel IPSec tunnels between dut1
// and dut2 in TUNNEL_VRF and returns the per-tunnel next-hops used to ECMP traffic.
func configureScaledTunnels(t *testing.T, dut1, dut2 *ondatra.DUTDevice, numTunnels int) (dut1TunnelV4NHs, dut2TunnelV4NHs, dut1TunnelV6NHs, dut2TunnelV6NHs []string) {
	t.Helper()

	// Tunnel overlay addressing: IPv4 /30s from 172.16.0.0/22; IPv6 /64s and
	// loopback /128s derived per tunnel index below.
	dut1TunnelV4s, err := iputil.GenerateIPsWithStep("172.16.0.1", numTunnels, "0.0.0.4")
	if err != nil {
		t.Fatalf("failed to generate DUT1 tunnel IPv4 addresses: %v", err)
	}
	dut2TunnelV4s, err := iputil.GenerateIPsWithStep("172.16.0.2", numTunnels, "0.0.0.4")
	if err != nil {
		t.Fatalf("failed to generate DUT2 tunnel IPv4 addresses: %v", err)
	}

	// Accumulate per-tunnel CLI blocks and reachability routes and push them once
	// per DUT after the loop; one Set per tunnel is prohibitively slow at scale.
	var dut1Tunnels, dut2Tunnels []string
	var dut1ReachRoutes, dut2ReachRoutes []staticRoute

	for i := 0; i < numTunnels; i++ {
		n := i + 1
		lbName := fmt.Sprintf("Loopback%d", i)
		tunName := fmt.Sprintf("Tunnel%d", n)
		dut1LbV6 := fmt.Sprintf("2001:db8:31:%x::1", i)
		dut2LbV6 := fmt.Sprintf("2001:db8:32:%x::1", i)
		dut1TunV6 := fmt.Sprintf("2001:db8:100:%x::1", i)
		dut2TunV6 := fmt.Sprintf("2001:db8:100:%x::2", i)
		ikePolicy := fmt.Sprintf("IKE_POLICY_%d", n)
		saPolicy := fmt.Sprintf("SA_POLICY_%d", n)
		profile := fmt.Sprintf("IPSEC_PROFILE_%d", n)

		// Per-tunnel loopback endpoints.
		configureLoopback(t, dut1, lbName, dut1LbV6, loopbackPrefixLen, true)
		configureLoopback(t, dut2, lbName, dut2LbV6, loopbackPrefixLen, true)

		dut1Cfg := cfgplugins.IPSecTunnelCfg{
			TunnelName:  tunName,
			Description: fmt.Sprintf("IPsec Tunnel Pair %d to DUT2", n),
			LocalFQDN:   fmt.Sprintf("dut1-t%d.test.local", n),
			RemoteFQDN:  fmt.Sprintf("dut2-t%d.test.local", n),
			TunnelIPv4:  fmt.Sprintf("%s/30", dut1TunnelV4s[i]),
			TunnelIPv6:  fmt.Sprintf("%s/64", dut1TunV6),
			TunnelSrc:   dut1LbV6,
			TunnelDst:   dut2LbV6,
			TunnelVRF:   tunnelVRF,
			IKEPolicy:   ikePolicy,
			SAPolicy:    saPolicy,
			Profile:     profile,
		}
		dut2Cfg := cfgplugins.IPSecTunnelCfg{
			TunnelName:  tunName,
			Description: fmt.Sprintf("IPsec Tunnel Pair %d to DUT1", n),
			LocalFQDN:   fmt.Sprintf("dut2-t%d.test.local", n),
			RemoteFQDN:  fmt.Sprintf("dut1-t%d.test.local", n),
			TunnelIPv4:  fmt.Sprintf("%s/30", dut2TunnelV4s[i]),
			TunnelIPv6:  fmt.Sprintf("%s/64", dut2TunV6),
			TunnelSrc:   dut2LbV6,
			TunnelDst:   dut1LbV6,
			TunnelVRF:   tunnelVRF,
			IKEPolicy:   ikePolicy,
			SAPolicy:    saPolicy,
			Profile:     profile,
		}

		// On vendors without an OpenConfig IPSec model (e.g. Arista), batch the
		// CLI blocks; otherwise configure each tunnel via OpenConfig immediately.
		if deviations.IpsecOcUnsupported(dut1) {
			dut1Tunnels = append(dut1Tunnels, cfgplugins.BuildIPSecTunnel(dut1Cfg))
		} else {
			batch1 := cfgplugins.ConfigureIPSecTunnel(t, dut1, dut1Cfg)
			batch1.Set(t, dut1)
		}
		if deviations.IpsecOcUnsupported(dut2) {
			dut2Tunnels = append(dut2Tunnels, cfgplugins.BuildIPSecTunnel(dut2Cfg))
		} else {
			batch2 := cfgplugins.ConfigureIPSecTunnel(t, dut2, dut2Cfg)
			batch2.Set(t, dut2)
		}

		// Reachability to the far-end loopback endpoint over both DUT-DUT core LAGs.
		dut1ReachRoutes = append(dut1ReachRoutes,
			staticRoute{Prefix: fmt.Sprintf("%s/128", dut2LbV6), NextHop: dut2CoreIntf2.IPv6},
			staticRoute{Prefix: fmt.Sprintf("%s/128", dut2LbV6), NextHop: dut2CoreIntf1.IPv6},
		)
		dut2ReachRoutes = append(dut2ReachRoutes,
			staticRoute{Prefix: fmt.Sprintf("%s/128", dut1LbV6), NextHop: dut1CoreIntf2.IPv6},
			staticRoute{Prefix: fmt.Sprintf("%s/128", dut1LbV6), NextHop: dut1CoreIntf1.IPv6},
		)

		dut1TunnelV4NHs = append(dut1TunnelV4NHs, dut1TunnelV4s[i])
		dut2TunnelV4NHs = append(dut2TunnelV4NHs, dut2TunnelV4s[i])
		dut1TunnelV6NHs = append(dut1TunnelV6NHs, dut1TunV6)
		dut2TunnelV6NHs = append(dut2TunnelV6NHs, dut2TunV6)
	}

	// Push all accumulated tunnel CLI in a single gNMI CLI Set per DUT.
	if len(dut1Tunnels) > 0 {
		helpers.GnmiCLIConfig(t, dut1, strings.Join(dut1Tunnels, "\n"))
	}
	if len(dut2Tunnels) > 0 {
		helpers.GnmiCLIConfig(t, dut2, strings.Join(dut2Tunnels, "\n"))
	}

	// Program all far-end loopback reachability routes in a single batched call
	// per DUT (configureStaticRoutes coalesces them into one gNMI Set).
	configureStaticRoutes(t, dut1, dut1ReachRoutes)
	configureStaticRoutes(t, dut2, dut2ReachRoutes)

	return dut1TunnelV4NHs, dut2TunnelV4NHs, dut1TunnelV6NHs, dut2TunnelV6NHs
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
		return fmt.Errorf("no packets observed on DUT-to-DUT member links")
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

// TestIPSecScaleWithMACSecOverAggregatedLinks implements IPSEC-1.2: it brings up the
// max number of parallel IPSec tunnels over MACsec and validates IPv4/IPv6 connectivity.
func TestIPSecScaleWithMACSecOverAggregatedLinks(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()

	// Step: Configure DUT customer-facing interfaces, VLANs, VRFs, MACSec, and DUT-DUT transport aggregates.
	// Create two core LAGs (each with 2 member ports) and apply to both DUTs.
	// Use per-DUT aggregate IDs from netutil to ensure device-valid agg names.
	// ATE still uses logical names ate1LagName/ate2LagName.

	// Build DUT port objects from the custPorts/corePorts groupings.
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
	// port5 on each DUT connects to ATE; on DUT1 it carries MACsec.
	dut1CustPort := dut1.Port(t, custPorts[0])
	dut2CustPort := dut2.Port(t, custPorts[0])

	// DUT-specific attributes: core LAGs on each DUT use the core interface attributes.
	dut1PortAttrs := []attrs.Attributes{dut1CoreIntf1, dut1CoreIntf2}
	dut2PortAttrs := []attrs.Attributes{dut2CoreIntf1, dut2CoreIntf2}

	// Create all VRFs upfront before configuring interfaces.
	createVRFs(t, dut1, []string{ateVRF, tunnelVRF})
	createVRFs(t, dut2, []string{ateVRF, tunnelVRF})

	// Configure DUTs: generate one aggregate per port group inside configureDUT.
	// The core port groups are the LAGs in TUNNEL_VRF for DUT-to-DUT communication.
	configureDUT(t, dut1, dut1CorePortGroups, dut1PortAttrs, "")
	configureDUT(t, dut1, [][]*ondatra.Port{{dut1CustPort}}, []attrs.Attributes{dut1CustIntf}, ateVRF)

	configureDUT(t, dut2, dut2CorePortGroups, dut2PortAttrs, "")
	configureDUT(t, dut2, [][]*ondatra.Port{{dut2CustPort}}, []attrs.Attributes{dut2CustIntf}, ateVRF)

	// Configure loopback interfaces used as IPSec tunnel endpoints.
	configureLoopback(t, dut1, loopbackIfName, dut1LoopbackIPv6, loopbackPrefixLen, true)
	configureLoopback(t, dut2, loopbackIfName, dut2LoopbackIPv6, loopbackPrefixLen, true)

	// Configure loopback interfaces used as IPSec tunnel endpoints.
	// All per-tunnel loopback endpoints are configured by configureScaledTunnels below.

	batchMACsec := cfgplugins.ConfigureMACsec(t, dut1, cfgplugins.MACsecCfg{
		IntfName:    dut1CustPort.Name(),
		ProfileName: "macSecProfile",
		CKN:         ckn,
		CAK:         cak,
		FallbackCKN: fallbackCkn,
		FallbackCAK: fallbackCak,
	})
	batchMACsec.Set(t, dut1)

	// Configure the maximum number of parallel IPSec tunnels between the two
	// DUTs. Each tunnel uses a dedicated loopback pair as its endpoints and an
	// independent IKE/SA/profile, all sharing TUNNEL_VRF. The returned per-tunnel
	// next-hop addresses are used to ECMP customer traffic across every tunnel.
	dut1TunnelV4NHs, dut2TunnelV4NHs, dut1TunnelV6NHs, dut2TunnelV6NHs := configureScaledTunnels(t, dut1, dut2, numTunnels)

	// DUT1 routing: customer return path towards ATE1, plus ECMP of customer
	// traffic destined to ATE2 across every tunnel's DUT2-side next-hop.
	dut1Routes := []staticRoute{
		{Prefix: ate1IPv4Prefix, NextHop: ate1LagConfig.IPv4, VRF: tunnelVRF, EgressVRF: ateVRF},
		{Prefix: ate1IPv6Prefix, NextHop: ate1LagConfig.IPv6, VRF: tunnelVRF, EgressVRF: ateVRF},
	}
	for _, nh := range dut2TunnelV4NHs {
		dut1Routes = append(dut1Routes, staticRoute{Prefix: ate2IPv4Prefix, NextHop: nh, VRF: ateVRF, EgressVRF: tunnelVRF})
	}
	for _, nh := range dut2TunnelV6NHs {
		dut1Routes = append(dut1Routes, staticRoute{Prefix: ate2IPv6Prefix, NextHop: nh, VRF: ateVRF, EgressVRF: tunnelVRF})
	}
	configureStaticRoutes(t, dut1, dut1Routes)

	// DUT2 routing: customer return path towards ATE2, plus ECMP of customer
	// traffic destined to ATE1 across every tunnel's DUT1-side next-hop.
	dut2Routes := []staticRoute{
		{Prefix: ate2IPv4Prefix, NextHop: ate2LagConfig.IPv4, VRF: tunnelVRF, EgressVRF: ateVRF},
		{Prefix: ate2IPv6Prefix, NextHop: ate2LagConfig.IPv6, VRF: tunnelVRF, EgressVRF: ateVRF},
	}
	for _, nh := range dut1TunnelV4NHs {
		dut2Routes = append(dut2Routes, staticRoute{Prefix: ate1IPv4Prefix, NextHop: nh, VRF: ateVRF, EgressVRF: tunnelVRF})
	}
	for _, nh := range dut1TunnelV6NHs {
		dut2Routes = append(dut2Routes, staticRoute{Prefix: ate1IPv6Prefix, NextHop: nh, VRF: ateVRF, EgressVRF: tunnelVRF})
	}
	configureStaticRoutes(t, dut2, dut2Routes)
	configureStaticRoutes(t, dut1, dut1Routes)

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

	// Start the customer-port capture before traffic; it is consumed once by
	// IPSEC-1.2.1 to confirm MACsec encryption.
	cs := packetvalidationhelpers.StartCapture(t, ate)
	captureValidated := false

	// runTrafficAndVerify runs a traffic window and verifies the named flows are
	// forwarded with no loss; used by each IPSEC-1.2.x subtest.
	runTrafficAndVerify := func(t *testing.T, flowNames ...string) {
		verifyTunnelOperStatus(t, dut1, tunnelIfName, oc.Interface_OperStatus_UP, tunnelUpTimeout)
		verifyTunnelOperStatus(t, dut2, tunnelIfName, oc.Interface_OperStatus_UP, tunnelUpTimeout)

		otg.StartTraffic(t)
		// Wait for traffic to flow and stabilize.
		time.Sleep(trafficWaitTime)
		otg.StopTraffic(t)

		// Consume the MACsec capture on the first traffic window and confirm the
		// customer traffic egressing towards the ATE is MACsec-encrypted.
		if !captureValidated {
			packetvalidationhelpers.StopCapture(t, ate, cs)
			if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, MacsecPacketValidation); err != nil {
				t.Errorf("CaptureAndValidatePackets() MACsec: %v", err)
			}
			captureValidated = true
		}

		for _, flowName := range flowNames {
			if err := verifyTraffic(t, ate, top, flowName, true); err != nil {
				t.Errorf("traffic verification failed: %v", err)
			}
		}
	}

	// The subtests map 1:1 to the IPSEC-1.2.x README sections. With a single
	// attachment, the device-max cases (1.2.3/1.2.4) reuse the same tunnel set.
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			// Step: Verify IPv4 connectivity over the max number of tunnels for a
			// single customer attachment.
			name: "IPSEC-1.2.1: Verify IPv4 Connectivity over a Max # of Tunnels for Single Attachment",
			fn: func(t *testing.T) {
				pre := readMemberOutPkts(t, dut1, dut1CorePorts)

				runTrafficAndVerify(t, flowIPv4Fwd, flowIPv4Bwd)

				if err := verifyDUTDUTLoadBalance(t, dut1, dut1CorePorts, pre, 0.25, false); err != nil {
					t.Errorf("load balance verification failed: %v", err)
				}
			},
		},
		{
			// Step: Verify IPv6 connectivity over the max number of tunnels for a
			// single customer attachment.
			name: "IPSEC-1.2.2: Verify IPv6 Connectivity over a Max # of Tunnels for Single Attachment",
			fn: func(t *testing.T) {

				pre := readMemberOutPkts(t, dut1, dut1CorePorts)

				runTrafficAndVerify(t, flowIPv6Fwd, flowIPv6Bwd)

				if err := verifyDUTDUTLoadBalance(t, dut1, dut1CorePorts, pre, 0.25, false); err != nil {
					t.Errorf("load balance verification failed: %v", err)
				}
			},
		},
		{
			// Step: Verify IPv4 connectivity over the device programmed with its max
			// number of tunnels.
			name: "IPSEC-1.2.3: Verify IPv4 Connectivity over Device with Max # of Tunnels",
			fn: func(t *testing.T) {
				t.Skip("IPSec on OTG is not supported, skipping this test")
			},
		},
		{
			// Step: Verify IPv6 connectivity over the device programmed with its max
			// number of tunnels.
			name: "IPSEC-1.2.4: Verify IPv6 Connectivity over Device with Max # of Tunnels",
			fn: func(t *testing.T) {
				t.Skip("IPSec on OTG is not supported, skipping this test")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, tc.fn)
	}
}
