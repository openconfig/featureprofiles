// Copyright 2024 Google LLC
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

package ingress_inner_ttl_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygot/ygot"
)

const (
	// VRF and VLAN configuration
	vrfName = "test_vrf"
	vlanID  = 10

	// Tunnel and MPLS configuration
	ipv4TunnelSrc    = "10.100.100.1"
	ipv4TunnelDstA   = "10.100.101.1"
	ipv4TunnelDstB   = "10.100.102.1"
	ipv4RoutePfx1    = "10.100.101.0/24"
	ipv4RoutePfx2    = "10.100.102.0/24"
	mplsLabel        = 100
	ipCount          = 10
	nexthopGroupName = "NHG-1"

	// Policy and TTL configuration
	policyForwardingName = "pf-policy-rewrite-ttl"
	tunnelIPTTL          = 64
	matchedIPTTL         = 64 // TODO: TTL config issue (issue ID: 442948813), once fix the issue replace to 1
	unmatchedIPTTL       = 32
	rewrittenIPTTL       = 1

	// Traffic parameters
	frameSize       = 512
	pps             = 1000
	lossTolerance   = 2
	trafficDuration = 15 * time.Second

	// ATE and DUT LAG configuration
	ateLAG1Name = "lag1"
	ateLAG2Name = "lag2"
	aggID1      = "Port-Channel1"
	aggID2      = "Port-Channel2"
	plenIPv4    = 30
	plenIPv6    = 126
	sleepTime   = 20
)

// DUT and ATE port attributes
var (
	dutLag1 = attrs.Attributes{Desc: "DUT LAG1 to ATE", IPv4: "192.0.3.1", IPv6: "2001:db8:3::1", MAC: "02:00:03:02:02:02", IPv4Len: plenIPv4, IPv6Len: plenIPv6}
	ateLag1 = attrs.Attributes{Name: "ateLag1", IPv4: "192.0.3.2", IPv6: "2001:db8:3::2", MAC: "02:00:03:01:01:01", IPv4Len: plenIPv4, IPv6Len: plenIPv6}

	dutLag2     = attrs.Attributes{Desc: "DUT LAG2 to ATE", IPv4: "192.0.4.1", IPv6: "2001:db8:4::1", MAC: "02:00:04:02:02:02", IPv4Len: plenIPv4, IPv6Len: plenIPv6}
	ateLag2     = attrs.Attributes{Name: "ateLag2", IPv4: "192.0.4.2", IPv6: "2001:db8:4::2", MAC: "02:00:04:01:01:01", IPv4Len: plenIPv4, IPv6Len: plenIPv6}
	dutLoopback = attrs.Attributes{
		Desc:    "Loopback ip",
		IPv4:    "10.100.100.1",
		IPv6:    "2001:db8::203:0:113:1",
		IPv4Len: 32,
		IPv6Len: 128,
	}
	matchedIPv4SrcNet   = "10.10.50.0"
	unmatchedIPv4SrcNet = "10.10.51.0"
	matchedIPv6SrcNet   = "2001:f:a::"
	unmatchedIPv6SrcNet = "2001:f:b::"
	ipv4DstNet          = "10.10.52.0"
	ipv6DstNet          = "2001:f:c::"
	isDefaultVrf        = true
	capturePorts        = []string{"port3", "port4"}
	routes              = []struct {
		prefix string
		indx   string
		nextIP string
	}{
		{ipv4RoutePfx1, "0", ateLag2.IPv4},
		{ipv4RoutePfx2, "1", ateLag2.IPv4},
		{ipv4RoutePfx1, "2", ateLag2.IPv6},
		{ipv4RoutePfx2, "3", ateLag2.IPv6},
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test case to verify inner packet TTL rewrite via policy forwarding.
func TestIngressInnerPktTTL(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	t.Log("Step 1: Configure DUT basic setup (interfaces, LAGs, VRF)")
	configureDUT(t, dut)

	t.Log("Step 2: Configure ATE basic setup (interfaces, LAGs, IPs)")
	otgConfig := configureATE(t, ate)
	otg := ate.OTG()
	otg.PushConfig(t, otgConfig)
	otg.StartProtocols(t)
	otgutils.WaitForARP(t, otg, otgConfig, "IPv4")
	otgutils.WaitForARP(t, otg, otgConfig, "IPv6")
	t.Run("IPv4 TTL Rewrite Verification", func(t *testing.T) {
		t.Log("Configuring IPv4 traffic flows on ATE")
		flows := ipv4Flows(t, otgConfig)
		otg.PushConfig(t, otgConfig)
		otg.StartProtocols(t)
		otgOperation(t, dut, ate, otg, otgConfig, flows)
	})

	t.Run("IPv6 TTL Rewrite Verification", func(t *testing.T) {
		t.Log("Configuring IPv6 traffic flows on ATE")
		flows := ipv6Flows(t, otgConfig)
		otg.PushConfig(t, otgConfig)
		otg.StartProtocols(t)
		otgOperation(t, dut, ate, otg, otgConfig, flows)
	})
}

// GetDefaultStaticNextHopGroupParams provides default parameters for the generator.
// matching the values in the provided JSON example.
func GetDefaultStaticNextHopGroupParams() cfgplugins.StaticNextHopGroupParams {
	return cfgplugins.StaticNextHopGroupParams{

		StaticNHGName: "MPLS_in_GRE_Encap",
		NHIPAddr1:     "nh_ip_addr_1",
		NHIPAddr2:     "nh_ip_addr_2",
		DynamicValues: []cfgplugins.DynamicStructParams{
			{
				NexthopGrpName: "NHG-1",
				NexthopType:    "mpls-over-gre",
				Ttl:            tunnelIPTTL,
				TunnelSrc:      ipv4TunnelSrc,
				TunnelDst:      ipv4TunnelDstA,
				MplsLabel:      mplsLabel,
				EntryValue:     1,
			},
			{
				NexthopGrpName: "NHG-1",
				NexthopType:    "mpls-over-gre",
				Ttl:            tunnelIPTTL,
				TunnelSrc:      ipv4TunnelSrc,
				TunnelDst:      ipv4TunnelDstB,
				MplsLabel:      mplsLabel,
				EntryValue:     2,
			},
		},
		DynamicVal: true,
	}
}

// GetDefaultOcPolicyForwardingParams provides default parameters for the generator,
// matching the values in the provided JSON example.
func GetDefaultOcPolicyForwardingParams() cfgplugins.OcPolicyForwardingParams {
	return cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: "DEFAULT",
		InterfaceID:         "Agg1.10",
		AppliedPolicyName:   "customer1",
		ChangeCli:           true,
		InterfaceName:       aggID1 + ".10",
	}
}

// configureDUT orchestrates the entire DUT configuration.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	t.Logf("Configuring Hardware Init")
	configureHardwareInit(t, dut)
	nonDefaultNI := cfgplugins.ConfigureNetworkInstance(t, dut, vrfName, !isDefaultVrf)
	// LAG1
	dutAggPorts1 := []*ondatra.Port{dut.Port(t, "port1"), dut.Port(t, "port2")}
	clearLAGInterfaces(t, dut, dutAggPorts1, aggID1)
	aggObj, aggSubint := configureDUTLag(t, dut, dutAggPorts1, aggID1, dutLag1, vlanID)
	// LAG2
	dutAggPorts2 := []*ondatra.Port{dut.Port(t, "port3"), dut.Port(t, "port4")}
	clearLAGInterfaces(t, dut, dutAggPorts2, aggID2)
	configureDUTLag(t, dut, dutAggPorts2, aggID2, dutLag2, 0)
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	cfgplugins.UpdateNetworkInstanceOnDut(t, dut, vrfName, nonDefaultNI)
	assignToNetworkInstance(t, dut, aggID1, vrfName, 10, true)
	updateLagInterfaceDetails(t, dut, aggSubint, dutLag1, aggObj, aggID1)

	configureDUTLoopback(t, dut)
	ocNHGParams := GetDefaultStaticNextHopGroupParams()
	ocPFParams := GetDefaultOcPolicyForwardingParams()
	_, ni, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
	cfgplugins.NextHopGroupConfig(t, dut, "v4", ni, ocNHGParams)
	cfgplugins.PolicyForwardingConfig(t, dut, "v4", pf, ocPFParams)
	cfgplugins.PolicyForwardingConfig(t, dut, "v6", pf, ocPFParams)
	// Configure all routes in one loop.
	for _, r := range routes {
		configureStaticRoute(t, dut, r.prefix, r.indx, r.nextIP)
	}
}

// configureDUTLag configures a LAG (Port-Channel) interface on the DUT,
// assigns its member interfaces, and applies IPv4/IPv6 addressing.
// It supports both untagged (default) and VLAN-tagged subinterfaces.
func configureDUTLag(t *testing.T, dut *ondatra.DUTDevice, aggPorts []*ondatra.Port,
	aggID string, dutLag attrs.Attributes, vlanID int) (*oc.Interface, *oc.Interface_Subinterface) {

	t.Helper()
	for _, port := range aggPorts {
		gnmi.Delete(t, dut, gnmi.OC().Interface(port.Name()).Ethernet().Config())
	}
	setupAggregateAtomically(t, dut, aggPorts, aggID)
	var subif *oc.Interface_Subinterface
	agg := &oc.Interface{Name: ygot.String(aggID)}
	if deviations.InterfaceEnabled(dut) {
		agg.Enabled = ygot.Bool(true)
	}
	subif = agg.GetOrCreateSubinterface(uint32(0))
	subif.Index = ygot.Uint32(uint32(0))
	agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	agg.Description = ygot.String(fmt.Sprintf("DUT %s to ATE", aggID))
	iv4 := subif.GetOrCreateIpv4()
	iv6 := subif.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		iv4.Enabled = ygot.Bool(true)
		iv6.Enabled = ygot.Bool(true)
	}
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC
	if vlanID > 0 {
		subif = agg.GetOrCreateSubinterface(uint32(vlanID))
		subif.Index = ygot.Uint32(uint32(vlanID))
		// Dot1Q tagging
		vlanEncap := subif.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged()
		vlanEncap.VlanId = ygot.Uint16(uint16(vlanID))
	} else {
		subif = agg.GetOrCreateSubinterface(uint32(0))
		subif.Index = ygot.Uint32(uint32(0))
	}
	v4 := subif.GetOrCreateIpv4()
	v4.Enabled = ygot.Bool(true)
	addr4 := v4.GetOrCreateAddress(dutLag.IPv4)
	addr4.PrefixLength = ygot.Uint8(uint8(dutLag.IPv4Len))

	v6 := subif.GetOrCreateIpv6()
	v6.Enabled = ygot.Bool(true)
	addr6 := v6.GetOrCreateAddress(dutLag.IPv6)
	addr6.PrefixLength = ygot.Uint8(uint8(dutLag.IPv6Len))

	gnmi.Replace(t, dut, gnmi.OC().Interface(aggID).Config(), agg)
	for _, port := range aggPorts {
		d := &oc.Root{}
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(aggID)
		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
		gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Config(), i)
	}
	return agg, subif
}

// configureHardwareInit sets up the initial hardware configuration on the DUT.
// It pushes hardware initialization configs for:
//  1. VRF Selection Extended feature
//  2. Policy Forwarding feature
func configureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	hardwareVrfCfg := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeatureVrfSelectionExtended)
	hardwarePfCfg := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeaturePolicyForwarding)
	if hardwareVrfCfg == "" || hardwarePfCfg == "" {
		return
	}
	cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwareVrfCfg)
	cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwarePfCfg)
}

// updateLagInterfaceDetails updates the IPv4/IPv6 addressing details on a LAG subinterface
// after a VRF has been configured.
func updateLagInterfaceDetails(t *testing.T, dut *ondatra.DUTDevice, subif *oc.Interface_Subinterface, dutLag attrs.Attributes, agg *oc.Interface, aggID string) {
	t.Helper()
	v4 := subif.GetOrCreateIpv4()
	v4.Enabled = ygot.Bool(true)
	addr4 := v4.GetOrCreateAddress(dutLag.IPv4)
	addr4.PrefixLength = ygot.Uint8(uint8(dutLag.IPv4Len))
	v6 := subif.GetOrCreateIpv6()
	v6.Enabled = ygot.Bool(true)
	addr6 := v6.GetOrCreateAddress(dutLag.IPv6)
	addr6.PrefixLength = ygot.Uint8(uint8(dutLag.IPv6Len))
	gnmi.Update(t, dut, gnmi.OC().Interface(aggID).Config(), agg)
}

func setupAggregateAtomically(t *testing.T, dut *ondatra.DUTDevice, aggPorts []*ondatra.Port, aggID string) {
	t.Helper()
	d := &oc.Root{}
	agg := d.GetOrCreateInterface(aggID)
	agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC

	for _, port := range aggPorts {
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(aggID)
		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
	}
	gnmi.Update(t, dut, gnmi.OC().Config(), d)
}

// assignToNetworkInstance attaches a physical or logical subinterface of a DUT
// to a given network instance (VRF).
func assignToNetworkInstance(t testing.TB, d *ondatra.DUTDevice, i string, ni string, si uint32, subInt bool) {
	t.Helper()
	if ni == "" {
		t.Fatalf("Network instance not provided for interface assignment")
	}
	netInst := &oc.NetworkInstance{Name: ygot.String(ni)}
	intf := &oc.Interface{Name: ygot.String(i)}
	netInstIntf, err := netInst.NewInterface(intf.GetName())
	if err != nil {
		t.Errorf("Error fetching NewInterface for %s", intf.GetName())
	}
	netInstIntf.Interface = ygot.String(intf.GetName())
	netInstIntf.Subinterface = ygot.Uint32(si)
	if subInt {
		netInstIntf.Id = ygot.String(fmt.Sprintf("%s.%d", intf.GetName(), si))
	} else {
		netInstIntf.Id = ygot.String(intf.GetName())
	}
	if intf.GetOrCreateSubinterface(si) != nil {
		gnmi.Update(t, d, gnmi.OC().NetworkInstance(ni).Config(), netInst)
	}
}

// clearLAGInterfaces deletes member port configs and the aggregate interface config.
func clearLAGInterfaces(t *testing.T, dut *ondatra.DUTDevice, aggPorts []*ondatra.Port, aggID string) {
	t.Helper()
	d := gnmi.OC()
	for _, port := range aggPorts {
		gnmi.Delete(t, dut, d.Interface(port.Name()).Config())
	}
	gnmi.Delete(t, dut, d.Interface(aggID).Config())
}

// configureStaticRoute installs a static IPv4 route on the DUT.
func configureStaticRoute(t *testing.T, dut *ondatra.DUTDevice, ipRoutePfx, indx, nxtIp string) {
	t.Helper()
	b := &gnmi.SetBatch{}
	sV4 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          ipRoutePfx,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			indx: oc.UnionString(nxtIp),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
		t.Fatalf("Failed to configure IPv4 static route: %v", err)
	}
	b.Set(t, dut)
}

// configureDUTLoopback sets up or retrieves the loopback interface on the DUT.
func configureDUTLoopback(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()
	loopbackIntfName := netutil.LoopbackInterface(t, dut, 0)
	lo0 := gnmi.OC().Interface(loopbackIntfName).Subinterface(0)

	ipv4Addrs := gnmi.LookupAll(t, dut, lo0.Ipv4().AddressAny().State())
	ipv6Addrs := gnmi.LookupAll(t, dut, lo0.Ipv6().AddressAny().State())
	var subif *oc.Interface_Subinterface
	loop1 := &oc.Interface{
		Name: ygot.String(loopbackIntfName),
		Type: oc.IETFInterfaces_InterfaceType_softwareLoopback,
	}
	if len(ipv4Addrs) == 0 && len(ipv6Addrs) == 0 {
		loop1.Enabled = ygot.Bool(true)

		// Subif 0 (required for loopback)
		subif = loop1.GetOrCreateSubinterface(0)
		subif.GetOrCreateIpv4().Enabled = ygot.Bool(true)
		subif.GetOrCreateIpv6().Enabled = ygot.Bool(true)
		// Push loopback config
		gnmi.Update(t, dut, d.Interface(loopbackIntfName).Config(), loop1)
	} else {
		if v4, ok := ipv4Addrs[0].Val(); ok {
			dutLoopback.IPv4 = v4.GetIp()
		}
		if v6, ok := ipv6Addrs[0].Val(); ok {
			dutLoopback.IPv6 = v6.GetIp()
		}
		t.Logf("Got DUT IPv4 loopback address: %v", dutLoopback.IPv4)
		t.Logf("Got DUT IPv6 loopback address: %v", dutLoopback.IPv6)
	}

	v4addr := subif.GetOrCreateIpv4().GetOrCreateAddress(dutLoopback.IPv4)
	v4addr.PrefixLength = ygot.Uint8(dutLoopback.IPv4Len)
	v6addr := subif.GetOrCreateIpv6().GetOrCreateAddress(dutLoopback.IPv6)
	v6addr.PrefixLength = ygot.Uint8(dutLoopback.IPv6Len)
	gnmi.Update(t, dut, d.Interface(loopbackIntfName).Config(), loop1)
	// Save into dutLoopback struct
	dutLoopback.IPv4 = v4addr.GetIp()
	dutLoopback.IPv6 = v6addr.GetIp()
}

// configureATE configures the ATE ports, LAGs, and IP interfaces.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	ateConfig := gosnappi.NewConfig()

	// ATE LAG1
	ateAggPorts1 := []*ondatra.Port{
		ate.Port(t, "port1"),
		ate.Port(t, "port2"),
	}
	configureLAGDevice(t, ateConfig, ateLAG1Name, 1, ateLag1, dutLag1, ateAggPorts1, 10)

	// ATE LAG2
	ateAggPorts2 := []*ondatra.Port{
		ate.Port(t, "port3"),
		ate.Port(t, "port4"),
	}
	configureLAGDevice(t, ateConfig, ateLAG2Name, 2, ateLag2, dutLag2, ateAggPorts2, 0)
	return ateConfig
}

// configureLAGDevice sets up a Link Aggregation Group (LAG) on the ATE.
func configureLAGDevice(t *testing.T, ateConfig gosnappi.Config, lagName string, lagID uint32,
	lagAttrs attrs.Attributes, dutAttrs attrs.Attributes, atePorts []*ondatra.Port, vlanID int) {

	t.Helper()
	lag := ateConfig.Lags().Add().SetName(lagName)
	lag.Protocol().Static().SetLagId(lagID)

	for i, p := range atePorts {
		port := ateConfig.Ports().Add().SetName(p.ID())
		mac, err := incrementMAC(lagAttrs.MAC, i+1)
		if err != nil {
			t.Fatal(err)
		}
		lag.Ports().Add().
			SetPortName(port.Name()).
			Ethernet().
			SetMac(mac).
			SetName("LAGMember" + strconv.Itoa(i+1))
	}

	dev := ateConfig.Devices().Add().SetName(lagAttrs.Name + ".Dev")
	eth := dev.Ethernets().Add().
		SetName(lagAttrs.Name + ".Eth").
		SetMac(lagAttrs.MAC)
	eth.Connection().SetLagName(lagName)
	// If VLAN ID specified, add VLAN tagging
	if vlanID > 0 {
		vlanObj := eth.Vlans().Add()
		vlanObj.SetName(fmt.Sprintf("%s_vlan%d", lagAttrs.Name, vlanID))
		vlanObj.SetId(uint32(vlanID))
	}

	// IPv4
	ipv4 := eth.Ipv4Addresses().Add().SetName(lagAttrs.Name + ".IPv4")
	ipv4.SetAddress(lagAttrs.IPv4).
		SetGateway(dutAttrs.IPv4).
		SetPrefix(uint32(lagAttrs.IPv4Len))

	// IPv6
	ipv6 := eth.Ipv6Addresses().Add().SetName(lagAttrs.Name + ".IPv6")
	ipv6.SetAddress(lagAttrs.IPv6).
		SetGateway(dutAttrs.IPv6).
		SetPrefix(uint32(lagAttrs.IPv6Len))
}

// ipv4Flows configures IPv4 traffic flows on the ATE.
//
// This function creates two traffic flows in the provided gosnappi Config:
//  1. "MatchedIPv4" flow – uses a defined source network, destination network,
//     and TTL expected to match the DUT policy.
//  2. "UnmatchedIPv4" flow – uses a different source network and TTL expected
//     not to match the DUT policy.
func ipv4Flows(t *testing.T, config gosnappi.Config) []string {
	t.Helper()
	config.Flows().Clear()

	createIPv4Flow := func(name, srcNet string, ttl uint32) string {
		flow := config.Flows().Add().SetName(name)
		flow.Metrics().SetEnable(true)
		flow.TxRx().Device().
			SetTxNames([]string{ateLag1.Name + ".IPv4"}).
			SetRxNames([]string{ateLag2.Name + ".IPv4"})
		flow.Rate().SetPps(pps)
		flow.Size().SetFixed(frameSize)

		flow.Packet().Add().Ethernet()
		flow.Packet().Add().Vlan().Id().SetValue(vlanID)

		ip := flow.Packet().Add().Ipv4()
		ip.Src().Increment().SetStart(srcNet).SetCount(ipCount)
		ip.Dst().Increment().SetStart(ipv4DstNet).SetCount(ipCount)
		ip.TimeToLive().SetValue(ttl)

		return flow.Name()
	}

	var flowList []string
	flowList = append(flowList,
		createIPv4Flow("MatchedIPv4", matchedIPv4SrcNet, matchedIPTTL),
		createIPv4Flow("UnmatchedIPv4", unmatchedIPv4SrcNet, unmatchedIPTTL),
	)

	return flowList
}

// ipv6Flows configures IPv6 traffic flows on the ATE.
// This function creates two IPv6 traffic flows in the gosnappi Config:
//  1. "MatchedIPv6" flow – configured with a specific source network and Hop Limit
//     expected to match DUT policy.
//  2. "UnmatchedIPv6" flow – configured with a different Hop Limit or parameters
//     expected not to match DUT policy.
func ipv6Flows(t *testing.T, c gosnappi.Config) []string {
	t.Helper()
	c.Flows().Clear()

	createIPv6Flow := func(name, srcNet, dstNet string, ttl uint32) string {
		flow := c.Flows().Add().SetName(name)
		flow.Metrics().SetEnable(true)
		flow.TxRx().Device().
			SetTxNames([]string{ateLag1.Name + ".IPv6"}).
			SetRxNames([]string{ateLag2.Name + ".IPv6"})
		flow.Rate().SetPps(pps)
		flow.Size().SetFixed(frameSize)
		flow.Packet().Add().Ethernet()
		flow.Packet().Add().Vlan().Id().SetValue(vlanID)
		ip := flow.Packet().Add().Ipv6()
		ip.Src().Increment().SetStart(srcNet).SetCount(ipCount)
		ip.Dst().Increment().SetStart(dstNet).SetCount(ipCount)
		ip.HopLimit().SetValue(ttl)

		return flow.Name()
	}

	var flowList []string
	flowList = append(flowList,
		createIPv6Flow("MatchedIPv6", matchedIPv6SrcNet, ipv6DstNet, matchedIPTTL),
		createIPv6Flow("UnmatchedIPv6", unmatchedIPv6SrcNet, ipv6DstNet, unmatchedIPTTL),
	)

	return flowList
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

// verifyTraffic checks that the flows sent and received the expected number of packets.
func verifyTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, c gosnappi.Config, flows []string) {
	t.Helper()
	otg := ate.OTG()
	otgutils.LogPortMetrics(t, otg, c)
	otgutils.LogLAGMetrics(t, otg, c)
	otgutils.LogFlowMetrics(t, otg, c)

	for _, flowName := range flows {
		flowPath := gnmi.OTG().Flow(flowName)
		txPkts := gnmi.Get(t, otg, flowPath.Counters().OutPkts().State())
		rxPkts := gnmi.Get(t, otg, flowPath.Counters().InPkts().State())

		if txPkts == 0 {
			t.Errorf("Flow %s did not transmit any packets", flowName)
		}

		lostPkts := txPkts - rxPkts
		if got := (lostPkts * 100 / txPkts); got >= lossTolerance {
			t.Errorf("Flow %s saw unexpected packet loss: sent %d, got %d", flowName, txPkts, rxPkts)
		} else {
			t.Logf("Flow %s traffic verification passed: sent %d, got %d", flowName, txPkts, rxPkts)
		}
	}
}

func otgOperation(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig *otg.OTG, config gosnappi.Config, flows []string) {
	t.Helper()
	validateATEPkts(t, otgConfig, flows)
	enableCapture(t, config, capturePorts)
	otgConfig.PushConfig(t, config)
	otgConfig.StartProtocols(t)

	verifyPortsUp(t, dut.Device)
	cs := startCapture(t, otgConfig)
	otgConfig.StartTraffic(t)
	time.Sleep(sleepTime * time.Second)
	otgConfig.StopTraffic(t)

	stopCapture(t, otgConfig, cs)
	verifyTrafficFlow(t, ate, config, flows)
	captureAndValidatePackets(t, ate, []string{ipv4TunnelDstA, ipv4TunnelDstB}, "ipv4")
}

// validateATEPkts validates packet counts for the given traffic flows on the ATE.
func validateATEPkts(t *testing.T, otgConfig *otg.OTG, flows []string) {
	t.Helper()
	for _, fName := range flows {
		otgConfig.StartTraffic(t)
		time.Sleep(sleepTime * time.Second)
		otgConfig.StopTraffic(t)
		egressAtePkts := gnmi.Get(t, otgConfig, gnmi.OTG().Flow(fName).Counters().InPkts().State())
		ingressAtePkts := gnmi.Get(t, otgConfig, gnmi.OTG().Flow(fName).Counters().OutPkts().State())
		if ingressAtePkts == 0 || egressAtePkts == 0 {
			t.Errorf("Got the unexpected packet count ingressAtePkts: %d, egressAtePkts: %d", ingressAtePkts, egressAtePkts)
		}
		if ingressAtePkts >= egressAtePkts {
			t.Logf("Interface counters matched: InUnicastPkts : %d OutUnicastPkts : %d", ingressAtePkts, egressAtePkts)
		} else {
			t.Errorf("Error: Interface counters didn't match.")
		}
	}
}

// captureAndValidatePackets captures packets on all configured capture ports
// and validates them against expected values (destination IPs, TTL/HopLimit, MPLS label).
func captureAndValidatePackets(t *testing.T, ate *ondatra.ATEDevice, dstIPs []string, protocolType string) {
	t.Helper()
	for _, p := range capturePorts {
		packetCaptureGRE := processCapture(t, ate.OTG(), p)
		handle, err := pcap.OpenOffline(packetCaptureGRE)
		if err != nil {
			t.Fatal(err)
		}
		defer handle.Close()
		packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
		validatePackets(t, packetSource, dstIPs, tunnelIPTTL, mplsLabel, protocolType)
	}
}

// validatePackets iterates through captured packets and validates protocol headers.
func validatePackets(t *testing.T, packetSource *gopacket.PacketSource, expectedIPs []string, expectedTTL, expectedLabel int, protocolType string) {
	t.Helper()
outer:
	for packet := range packetSource.Packets() {
		var match bool

		switch protocolType {
		case "ipv4":
			ipLayer := packet.Layer(layers.LayerTypeIPv4)
			if ipLayer == nil {
				continue
			}
			ipPacket := ipLayer.(*layers.IPv4)
			gotTTL := ipPacket.TTL
			gotDstIP := ipPacket.DstIP.String()

			if slices.Contains(expectedIPs, gotDstIP) && int(gotTTL) == expectedTTL {
				t.Logf("Matched IPv4 packet: DstIP = %s, TTL = %d", gotDstIP, gotTTL)
				match = true
			} else {
				t.Errorf("Failed to match IPv4 IP/TTL, GotIP = %s, ExpectedIPs = %v, GotTTL = %d, ExpectedTTL = %d",
					gotDstIP, expectedIPs, gotTTL, expectedTTL)
			}

		case "ipv6":
			ipLayer := packet.Layer(layers.LayerTypeIPv6)
			if ipLayer == nil {
				continue
			}
			ipPacket := ipLayer.(*layers.IPv6)
			gotHopLimit := ipPacket.HopLimit
			gotDstIP := ipPacket.DstIP.String()

			if slices.Contains(expectedIPs, gotDstIP) && int(gotHopLimit) == expectedTTL {
				t.Logf("Matched IPv6 packet: DstIP = %s, HopLimit = %d", gotDstIP, gotHopLimit)
				match = true
			} else {
				t.Errorf("Failed to match IPv6 IP/HopLimit, GotIP = %s, ExpectedIPs = %v, GotHopLimit = %d, ExpectedHopLimit = %d",
					gotDstIP, expectedIPs, gotHopLimit, expectedTTL)
			}

		default:
			t.Fatalf("Unsupported protocol type: %s. Must be 'ipv4' or 'ipv6'", protocolType)
		}

		// validate GRE
		greLayer := packet.Layer(layers.LayerTypeGRE)
		if greLayer == nil {
			t.Fatalf("GRE header missing")
		} else {
			t.Log("Found GRE header")
		}

		// validate MPLS
		mplsLayer := packet.Layer(layers.LayerTypeMPLS)
		if mplsLayer == nil {
			t.Fatalf("MPLS header missing")
		} else {
			mplsPacket := mplsLayer.(*layers.MPLS)
			if mplsPacket.Label == uint32(expectedLabel) {
				t.Logf("Matched MPLS Label, Got: %d, Expected: %d", mplsPacket.Label, expectedLabel)
			} else {
				t.Fatalf("MPLS Label mismatch, Got: %d, Expected: %d", mplsPacket.Label, expectedLabel)
			}
		}

		if match {
			break outer // break only after all validations succeed
		}
	}
}

func enableCapture(t *testing.T, config gosnappi.Config, portNames []string) {
	t.Helper()
	config.Captures().Clear()
	for _, portName := range portNames {
		t.Logf("Configuring packet capture for OTG port: %s", portName)
		cap := config.Captures().Add()
		cap.SetName(portName)
		cap.SetPortNames([]string{portName})
		cap.SetFormat(gosnappi.CaptureFormat.PCAP)
	}
}

func startCapture(t *testing.T, otg *otg.OTG) gosnappi.ControlState {
	t.Helper()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	otg.SetControlState(t, cs)

	return cs
}

func stopCapture(t *testing.T, otg *otg.OTG, cs gosnappi.ControlState) {
	t.Helper()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	otg.SetControlState(t, cs)
}

func processCapture(t *testing.T, otg *otg.OTG, port string) string {
	t.Helper()
	bytes := otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(port))
	time.Sleep(sleepTime * time.Second)
	capture, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Errorf("ERROR: Could not create temporary pcap file: %v\n", err)
	}
	if _, err := capture.Write(bytes); err != nil {
		t.Errorf("ERROR: Could not write bytes to pcap file: %v\n", err)
	}
	defer capture.Close()

	return capture.Name()
}

// Verify ports status
func verifyPortsUp(t *testing.T, dev *ondatra.Device) {
	t.Helper()
	t.Log("Verifying port status")
	for _, p := range dev.Ports() {
		status := gnmi.Get(t, dev, gnmi.OC().Interface(p.Name()).OperStatus().State())
		if want := oc.Interface_OperStatus_UP; status != want {
			t.Errorf("%s Status: got %v, want %v", p, status, want)
		}
	}
}
