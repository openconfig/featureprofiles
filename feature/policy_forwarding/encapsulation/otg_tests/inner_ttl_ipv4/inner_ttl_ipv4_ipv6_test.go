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

package inner_ttl_ipv4_ipv6_test

import (
	"fmt"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
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
	"github.com/openconfig/featureprofiles/internal/iputil"
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
	IPv4TunnelSrc    = "10.100.100.1"
	IPv4TunnelDstA   = "10.100.101.1"
	IPv4TunnelDstB   = "10.100.102.1"
	IPv4RoutePfx1    = "10.100.101.0/24"
	IPv4RoutePfx2    = "10.100.102.0/24"
	vrfIPv4Prefix    = "0.0.0.0/0"
	vrfIPv6Prefix    = "::/0"
	mplsLabel        = 100
	srcIPCount       = 100
	dstIPcount       = 10
	nexthopGroupName = "NHG-1"

	// Policy and TTL configuration
	policyForwardingName = "pf-policy-rewrite-ttl"
	tunnelIPTTL          = 64
	matchTTL             = 1
	unMatchTTL           = 32
	rewrittenIPTTL       = 1

	// Traffic parameters
	frameSize       = 512
	ratePPS         = 1000
	lossTolerance   = 2
	trafficDuration = 20 * time.Second

	// ATE and DUT LAG configuration
	ateLAG1Name = "lag1"
	ateLAG2Name = "lag2"
	aggID1      = "Port-Channel1"
	aggID2      = "Port-Channel2"
	plenIPv4    = 30
	plenIPv6    = 126
)

// DUT and ATE port attributes
var (
	dutLag1             = attrs.Attributes{Desc: "DUT LAG1 to ATE", IPv4: "192.0.3.1", IPv6: "2001:db8:3::1", MAC: "02:00:03:02:02:02", IPv4Len: plenIPv4, IPv6Len: plenIPv6}
	ateLag1             = attrs.Attributes{Name: "ateLag1", IPv4: "192.0.3.2", IPv6: "2001:db8:3::2", MAC: "02:00:03:01:01:01", IPv4Len: plenIPv4, IPv6Len: plenIPv6}
	dutLag2             = attrs.Attributes{Desc: "DUT LAG2 to ATE", IPv4: "192.0.4.1", IPv6: "2001:db8:4::1", MAC: "02:00:04:02:02:02", IPv4Len: plenIPv4, IPv6Len: plenIPv6}
	ateLag2             = attrs.Attributes{Name: "ateLag2", IPv4: "192.0.4.2", IPv6: "2001:db8:4::2", MAC: "02:00:04:01:01:01", IPv4Len: plenIPv4, IPv6Len: plenIPv6}
	dutLoopback         = attrs.Attributes{Desc: "Loopback ip", IPv4: "10.100.100.1", IPv6: "2001:db8::203:0:113:1", IPv4Len: 32, IPv6Len: 128}
	matchedIPv4SrcNet   = "10.10.50.0"
	unmatchedIPv4SrcNet = "10.10.51.0"
	matchedIPv6SrcNet   = "2001:f:a:1::1"
	unmatchedIPv6SrcNet = "2001:f:b:1::1"
	v4DstNet            = "10.10.52.0"
	v6DstNet            = "2001:f:c:1::1"
	v6Step              = "1::1"
	isDefaultVRF        = true
	capturePorts        = []string{"port3", "port4"}
	routes              = []struct {
		prefix string
		indx   string
		nextIP string
	}{
		{IPv4RoutePfx1, "0", ateLag2.IPv4},
		{IPv4RoutePfx2, "1", ateLag2.IPv4},
		{IPv4RoutePfx1, "2", ateLag2.IPv6},
		{IPv4RoutePfx2, "3", ateLag2.IPv6},
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test case to verify inner packet TTL rewrite via policy forwarding.
func TestIngressInnerPktTTL(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	batch := configureDUT(t, dut)
	otgConfig := configureATE(t, ate)
	otg := ate.OTG()
	configPush(t, otg, otgConfig)
	otgutils.WaitForARP(t, otg, otgConfig, "IPv4")
	otgutils.WaitForARP(t, otg, otgConfig, "IPv6")
	verifyPortsUp(t, dut.Device)
	tests := []struct {
		name          string
		family        string
		matchSrcNet   string
		unMatchSrcNet string
		dstNet        string
		protoStr      string
		vrfIPPrefix   string
		matchTTL      int
		unMatchTTL    int
	}{
		{
			name:          "IPv4 TTL Rewrite Verification",
			family:        "ipv4",
			matchSrcNet:   matchedIPv4SrcNet,
			unMatchSrcNet: unmatchedIPv4SrcNet,
			dstNet:        v4DstNet,
			matchTTL:      matchTTL,
			unMatchTTL:    unMatchTTL,
			vrfIPPrefix:   vrfIPv4Prefix,
			protoStr:      "ip",
		},
		{
			name:          "IPv6 TTL Rewrite Verification",
			family:        "ipv6",
			matchSrcNet:   matchedIPv6SrcNet,
			unMatchSrcNet: unmatchedIPv6SrcNet,
			dstNet:        v6DstNet,
			matchTTL:      matchTTL,
			unMatchTTL:    unMatchTTL,
			vrfIPPrefix:   vrfIPv6Prefix,
			protoStr:      "ipv6",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ocNHGParams := defaultStaticNextHopGroupParams()
			ocPFParams := defaultOcPolicyForwardingParams()
			_, ni, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
			cfgplugins.NextHopGroupConfig(t, dut, "v4", ni, ocNHGParams)

			// --- Matched case ---
			ocPFParams.MatchTTL = tc.matchTTL
			ocPFParams.IPType = tc.family
			cfgplugins.NewPolicyForwardingMatchAndSetTTL(t, dut, pf, cfgplugins.OcPolicyForwardingParams{PolicyName: policyForwardingName, RemovePolicy: true, IPType: tc.family})
			cfgplugins.NewPolicyForwardingMatchAndSetTTL(t, dut, pf, ocPFParams)

			matchFlow := createFlow(t, otgConfig, fmt.Sprintf("%s%s", "matched-", tc.family), tc.matchSrcNet, tc.dstNet, tc.family, uint32(tc.matchTTL))
			configPush(t, otg, otgConfig)
			otgOperation(t, dut, ate, otg, otgConfig, matchFlow, tc.family, rewrittenIPTTL)

			// --- Unmatched case ---
			cfgplugins.NewPolicyForwardingMatchAndSetTTL(t, dut, pf, cfgplugins.OcPolicyForwardingParams{PolicyName: policyForwardingName, RemovePolicy: true, IPType: tc.family})
			configureVRFStaticRoute(t, dut, batch, vrfName, tc.vrfIPPrefix, nexthopGroupName, tc.protoStr)
			unMatchFlow := createFlow(t, otgConfig, fmt.Sprintf("%s%s", "unMatched-", tc.family), tc.unMatchSrcNet, tc.dstNet, tc.family, uint32(tc.unMatchTTL))
			configPush(t, otg, otgConfig)
			otgOperation(t, dut, ate, otg, otgConfig, unMatchFlow, tc.family, tc.unMatchTTL-1)
		})
	}
}

// configPush method to push OTG configuration and start the protocols.
func configPush(t *testing.T, otg *otg.OTG, otgConfig gosnappi.Config) {
	t.Helper()
	otg.PushConfig(t, otgConfig)
	otg.StartProtocols(t)
}

// defaultStaticNextHopGroupParams provides default parameters for the generator. matching the values in the provided JSON example.
func defaultStaticNextHopGroupParams() cfgplugins.StaticNextHopGroupParams {
	return cfgplugins.StaticNextHopGroupParams{
		StaticNHGName: "MPLS_in_GRE_Encap",
		NHIPAddr1:     "nh_ip_addr_1",
		NHIPAddr2:     "nh_ip_addr_2",
		DynamicValues: []cfgplugins.DynamicStructParams{
			{
				NexthopGrpName: nexthopGroupName,
				NexthopType:    "mpls-over-gre",
				TTL:            tunnelIPTTL,
				TunnelSrc:      IPv4TunnelSrc,
				TunnelDst:      IPv4TunnelDstA,
				MplsLabel:      mplsLabel,
				EntryValue:     1,
			},
			{
				NexthopGrpName: nexthopGroupName,
				NexthopType:    "mpls-over-gre",
				TTL:            tunnelIPTTL,
				TunnelSrc:      IPv4TunnelSrc,
				TunnelDst:      IPv4TunnelDstB,
				MplsLabel:      mplsLabel,
				EntryValue:     2,
			},
		},
		DynamicVal: true,
	}
}

// defaultOcPolicyForwardingParams provides default parameters for the generator, matching the values in the provided JSON example.
func defaultOcPolicyForwardingParams() cfgplugins.OcPolicyForwardingParams {
	return cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: "DEFAULT",
		InterfaceID:         "Agg1.10",
		AppliedPolicyName:   "customer1",
		InterfaceName:       aggID1 + ".10",
		MatchTTL:            matchTTL,
		ActionSetTTL:        rewrittenIPTTL,
		PolicyName:          policyForwardingName,
		ActionNHGName:       nexthopGroupName,
	}
}

// configureDUT orchestrates the entire DUT configuration.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) *gnmi.SetBatch {
	t.Helper()
	t.Logf("Configuring Hardware Init")
	batch := &gnmi.SetBatch{}
	configureHardwareInit(t, dut)
	nonDefaultNI := cfgplugins.ConfigureNetworkInstance(t, dut, vrfName, !isDefaultVRF)
	// LAG1
	dutAggPorts1 := []*ondatra.Port{dut.Port(t, "port1"), dut.Port(t, "port2")}
	clearLAGInterfaces(t, dut, dutAggPorts1, aggID1)
	aggObj, aggSubint := configureDUTLag(t, dut, batch, dutAggPorts1, aggID1, dutLag1, vlanID)
	// LAG2
	dutAggPorts2 := []*ondatra.Port{dut.Port(t, "port3"), dut.Port(t, "port4")}
	clearLAGInterfaces(t, dut, dutAggPorts2, aggID2)
	configureDUTLag(t, dut, batch, dutAggPorts2, aggID2, dutLag2, 0)
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	cfgplugins.UpdateNetworkInstanceOnDut(t, dut, vrfName, nonDefaultNI)
	assignToNetworkInstance(t, dut, batch, aggID1, vrfName, 10, true)
	updateLagInterfaceDetails(t, dut, batch, aggSubint, dutLag1, aggObj, aggID1)

	configureDUTLoopback(t, dut, batch)
	// Configure all routes in one loop.
	for _, r := range routes {
		configureStaticRoute(t, dut, batch, r.prefix, r.indx, r.nextIP)
	}
	batch.Set(t, dut)

	return batch
}

// configureDUTLag configures a LAG (Port-Channel) interface on the DUT, assigns its member interfaces, and applies IPv4/IPv6 addressing. It supports both untagged (default) and VLAN-tagged subinterfaces.
func configureDUTLag(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, aggPorts []*ondatra.Port, aggID string, dutLag attrs.Attributes, vlanID int) (*oc.Interface, *oc.Interface_Subinterface) {
	t.Helper()
	var subif *oc.Interface_Subinterface
	agg := &oc.Interface{Name: ygot.String(aggID)}
	if deviations.InterfaceEnabled(dut) {
		agg.Enabled = ygot.Bool(true)
	}
	agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	agg.Description = ygot.String(fmt.Sprintf("DUT %s to ATE", aggID))
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC

	// Always create subif 0
	subif = agg.GetOrCreateSubinterface(0)
	subif.Index = ygot.Uint32(0)
	iv4 := subif.GetOrCreateIpv4()
	iv6 := subif.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		iv4.Enabled = ygot.Bool(true)
		iv6.Enabled = ygot.Bool(true)
	}

	// If VLAN > 0, create that subif (with tagging + IPs)
	if vlanID > 0 {
		subif = agg.GetOrCreateSubinterface(uint32(vlanID))
		subif.Index = ygot.Uint32(uint32(vlanID))
		vlanEncap := subif.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged()
		vlanEncap.VlanId = ygot.Uint16(uint16(vlanID))
	} else {
		subif = agg.GetOrCreateSubinterface(0)
		subif.Index = ygot.Uint32(0)
	}

	// IP configuration on chosen subif
	v4 := subif.GetOrCreateIpv4()
	v4.Enabled = ygot.Bool(true)
	addr4 := v4.GetOrCreateAddress(dutLag.IPv4)
	addr4.PrefixLength = ygot.Uint8(uint8(dutLag.IPv4Len))

	v6 := subif.GetOrCreateIpv6()
	v6.Enabled = ygot.Bool(true)
	addr6 := v6.GetOrCreateAddress(dutLag.IPv6)
	addr6.PrefixLength = ygot.Uint8(uint8(dutLag.IPv6Len))

	// Add LAG to batch
	gnmi.BatchReplace(batch, gnmi.OC().Interface(aggID).Config(), agg)

	// Add member ports to batch
	for _, port := range aggPorts {
		d := &oc.Root{}
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(aggID)
		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
		gnmi.BatchReplace(batch, gnmi.OC().Interface(port.Name()).Config(), i)
	}
	return agg, subif
}

// configureHardwareInit sets up the initial hardware configuration on the DUT. It pushes hardware initialization configs for VRF Selection Extended feature and Policy Forwarding feature.
func configureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	features := []cfgplugins.FeatureType{
		cfgplugins.FeatureVrfSelectionExtended,
		cfgplugins.FeaturePolicyForwarding,
		cfgplugins.FeatureTTLPolicyForwarding,
	}
	for _, feature := range features {
		hardwareInitCfg := cfgplugins.NewDUTHardwareInit(t, dut, feature)
		if hardwareInitCfg != "" {
			cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwareInitCfg)
		}
	}
}

// updateLagInterfaceDetails updates the IPv4/IPv6 addressing details on a LAG subinterface after a VRF has been configured.
func updateLagInterfaceDetails(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, subif *oc.Interface_Subinterface, dutLag attrs.Attributes, agg *oc.Interface, aggID string) {
	t.Helper()
	v4 := subif.GetOrCreateIpv4()
	v4.Enabled = ygot.Bool(true)
	addr4 := v4.GetOrCreateAddress(dutLag.IPv4)
	addr4.PrefixLength = ygot.Uint8(uint8(dutLag.IPv4Len))
	v6 := subif.GetOrCreateIpv6()
	v6.Enabled = ygot.Bool(true)
	addr6 := v6.GetOrCreateAddress(dutLag.IPv6)
	addr6.PrefixLength = ygot.Uint8(uint8(dutLag.IPv6Len))
	gnmi.BatchUpdate(batch, gnmi.OC().Interface(aggID).Config(), agg)
}

// assignToNetworkInstance attaches a physical or logical subinterface of a DUT to a given network instance (VRF).
func assignToNetworkInstance(t testing.TB, d *ondatra.DUTDevice, batch *gnmi.SetBatch, i string, ni string, si uint32, subInt bool) {
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
		gnmi.BatchUpdate(batch, gnmi.OC().NetworkInstance(ni).Config(), netInst)
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
func configureStaticRoute(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, ipRoutePfx, indx, nxtIP string) {
	t.Helper()
	staticRoute := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          ipRoutePfx,
		NextHops:        map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{indx: oc.UnionString(nxtIP)},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(batch, staticRoute, dut); err != nil {
		t.Fatalf("Failed to configure static routes: %v", err)
	}
}

// configureVRFStaticRoute installs a static IPv4 route on the DUT.
func configureVRFStaticRoute(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, vrfStr, ipRoutePfx, nxtHGName, protoStr string) {
	t.Helper()
	staticRoute := &cfgplugins.StaticVRFRouteCfg{
		NetworkInstance: vrfStr,
		Prefix:          ipRoutePfx,
		NextHopGroup:    nxtHGName,
		ProtocolStr:     protoStr,
	}
	if _, err := cfgplugins.NewStaticVRFRoute(t, batch, staticRoute, dut); err != nil {
		t.Fatalf("Failed to configure VRF static routes: %v", err)
	}
}

// configureDUTLoopback sets up or retrieves the loopback interface on the DUT.
func configureDUTLoopback(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch) {
	t.Helper()
	d := gnmi.OC()
	loopbackIntfName := netutil.LoopbackInterface(t, dut, 0)
	lo0 := gnmi.OC().Interface(loopbackIntfName).Subinterface(0)

	ipv4Addrs := gnmi.LookupAll(t, dut, lo0.Ipv4().AddressAny().State())
	ipv6Addrs := gnmi.LookupAll(t, dut, lo0.Ipv6().AddressAny().State())
	var subif *oc.Interface_Subinterface
	loop1 := &oc.Interface{Name: ygot.String(loopbackIntfName), Type: oc.IETFInterfaces_InterfaceType_softwareLoopback}
	if len(ipv4Addrs) == 0 && len(ipv6Addrs) == 0 {
		loop1.Enabled = ygot.Bool(true)
		// Subif 0 (required for loopback)
		subif = loop1.GetOrCreateSubinterface(0)
		subif.GetOrCreateIpv4().Enabled = ygot.Bool(true)
		subif.GetOrCreateIpv6().Enabled = ygot.Bool(true)
		// Push loopback config
		gnmi.BatchUpdate(batch, d.Interface(loopbackIntfName).Config(), loop1)
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
	gnmi.BatchUpdate(batch, d.Interface(loopbackIntfName).Config(), loop1)
	// Save into dutLoopback struct
	dutLoopback.IPv4 = v4addr.GetIp()
	dutLoopback.IPv6 = v6addr.GetIp()
}

// configureATE configures the ATE ports, LAGs, and IP interfaces.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	ateConfig := gosnappi.NewConfig()

	// ATE LAG1
	ateAggPorts1 := []*ondatra.Port{ate.Port(t, "port1"), ate.Port(t, "port2")}
	configureLAGDevice(t, ateConfig, ateLAG1Name, 1, ateLag1, dutLag1, ateAggPorts1, 10)

	// ATE LAG2
	ateAggPorts2 := []*ondatra.Port{ate.Port(t, "port3"), ate.Port(t, "port4")}
	configureLAGDevice(t, ateConfig, ateLAG2Name, 2, ateLag2, dutLag2, ateAggPorts2, 0)
	return ateConfig
}

// configureLAGDevice sets up a Link Aggregation Group (LAG) on the ATE.
func configureLAGDevice(t *testing.T, ateConfig gosnappi.Config, lagName string, lagID uint32, lagAttrs attrs.Attributes, dutAttrs attrs.Attributes, atePorts []*ondatra.Port, vlanID int) {
	t.Helper()
	lag := ateConfig.Lags().Add().SetName(lagName)
	lag.Protocol().Static().SetLagId(lagID)

	for i, p := range atePorts {
		port := ateConfig.Ports().Add().SetName(p.ID())
		mac, err := iputil.IncrementMAC(lagAttrs.MAC, i+1)
		if err != nil {
			t.Fatal(err)
		}
		lag.Ports().Add().SetPortName(port.Name()).Ethernet().SetMac(mac).SetName("LAGMember" + strconv.Itoa(i+1))
	}

	dev := ateConfig.Devices().Add().SetName(lagAttrs.Name + ".Dev")
	eth := dev.Ethernets().Add().SetName(lagAttrs.Name + ".Eth").SetMac(lagAttrs.MAC)
	eth.Connection().SetLagName(lagName)
	// If VLAN ID specified, add VLAN tagging
	if vlanID > 0 {
		vlanObj := eth.Vlans().Add()
		vlanObj.SetName(fmt.Sprintf("%s_vlan%d", lagAttrs.Name, vlanID))
		vlanObj.SetId(uint32(vlanID))
	}

	// IPv4
	ipv4 := eth.Ipv4Addresses().Add().SetName(lagAttrs.Name + ".IPV4")
	ipv4.SetAddress(lagAttrs.IPv4).SetGateway(dutAttrs.IPv4).SetPrefix(uint32(lagAttrs.IPv4Len))

	// IPv6
	ipv6 := eth.Ipv6Addresses().Add().SetName(lagAttrs.Name + ".IPV6")
	ipv6.SetAddress(lagAttrs.IPv6).SetGateway(dutAttrs.IPv6).SetPrefix(uint32(lagAttrs.IPv6Len))
}

// createFlow configures IPv4/IPv6 (match/unmatch) traffic flows on the ATE.
func createFlow(t *testing.T, config gosnappi.Config, name, srcIP, dstIP, protoType string, ttl uint32) string {
	t.Helper()
	config.Flows().Clear()
	flow := config.Flows().Add().SetName(name)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{fmt.Sprintf("%s%s%s", ateLag1.Name, ".", strings.ToUpper(protoType))}).SetRxNames([]string{fmt.Sprintf("%s%s%s", ateLag2.Name, ".", strings.ToUpper(protoType))})
	flow.Rate().SetPps(ratePPS)
	flow.Size().SetFixed(frameSize)
	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(ateLag1.MAC)
	eth.Dst().Auto()
	flow.Packet().Add().Vlan().Id().SetValue(vlanID)
	switch strings.ToLower(protoType) {
	case "ipv4":
		ip := flow.Packet().Add().Ipv4()
		ip.Src().Increment().SetStart(srcIP).SetCount(srcIPCount)
		ip.Dst().Increment().SetStart(dstIP).SetCount(dstIPcount)
		ip.TimeToLive().SetValue(ttl)

	case "ipv6":
		ip := flow.Packet().Add().Ipv6()
		ip.Src().Increment().SetStart(srcIP).SetCount(srcIPCount).SetStep(v6Step)
		ip.Dst().Increment().SetStart(dstIP).SetCount(dstIPcount).SetStep(v6Step)
		ip.HopLimit().SetValue(ttl)

	default:
		t.Fatalf("unsupported protocol: %s", protoType)
	}

	return flow.Name()
}

// verifyTraffic checks that the flows sent and received the expected number of packets.
func verifyTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, c gosnappi.Config, flowName string) {
	t.Helper()
	otg := ate.OTG()
	otgutils.LogPortMetrics(t, otg, c)
	otgutils.LogLAGMetrics(t, otg, c)
	otgutils.LogFlowMetrics(t, otg, c)

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

// otgOperation orchestrates the entire OTG workflow for traffic validation.
// It validates ATE packet counters, enables capture on specified ports, pushes configuration, starts protocols, ensures DUT ports are up, starts capture, runs traffic for a fixed duration, stops traffic, stops capture, verifies traffic flow, and validates captured packets.
func otgOperation(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig *otg.OTG, config gosnappi.Config, flow, protoType string, ttl int) {
	t.Helper()
	enableCapture(t, config, capturePorts)
	configPush(t, otgConfig, config)
	validateATEPkts(t, otgConfig, flow)

	cs := startCapture(t, otgConfig)
	otgConfig.StartTraffic(t)
	time.Sleep(trafficDuration)
	otgConfig.StopTraffic(t)

	stopCapture(t, otgConfig, cs)
	verifyTrafficFlow(t, ate, config, flow)
	if protoType == "ipv4" {
		// TODO: We cannot add an error return here because the GenerateIPs method is already merged into main. To fix this, we’ll need to raise a new PR and update it.
		IPList := iputil.GenerateIPs(v4DstNet, dstIPcount)
		captureAndValidatePackets(t, ate, IPList, protoType, ttl)
	} else {
		IPList, err := iputil.GenerateIPv6s(net.ParseIP(v6DstNet), dstIPcount)
		if err != nil {
			t.Errorf("%s", err)
		}
		captureAndValidatePackets(t, ate, IPList, protoType, ttl)
	}
}

// validateATEPkts validates packet counts for the given traffic flows on the ATE.
func validateATEPkts(t *testing.T, otgConfig *otg.OTG, flowName string) {
	t.Helper()
	otgConfig.StartTraffic(t)
	time.Sleep(trafficDuration)
	otgConfig.StopTraffic(t)
	egressAtePkts := gnmi.Get(t, otgConfig, gnmi.OTG().Flow(flowName).Counters().InPkts().State())
	ingressAtePkts := gnmi.Get(t, otgConfig, gnmi.OTG().Flow(flowName).Counters().OutPkts().State())
	if ingressAtePkts == 0 || egressAtePkts == 0 {
		t.Errorf("Got the unexpected packet count ingressAtePkts: %d, egressAtePkts: %d", ingressAtePkts, egressAtePkts)
	}
	if ingressAtePkts >= egressAtePkts {
		t.Logf("Interface counters matched: InUnicastPkts : %d OutUnicastPkts : %d", ingressAtePkts, egressAtePkts)
	} else {
		t.Errorf("Error: Interface counters didn't match.")
	}
}

// captureAndValidatePackets captures packets on all configured capture ports and validates them against expected values (destination IPs, TTL/HopLimit, MPLS label).
func captureAndValidatePackets(t *testing.T, ate *ondatra.ATEDevice, dstIPs []string, protocolType string, ttl int) {
	t.Helper()
	for _, p := range capturePorts {
		packetCaptureGRE := processCapture(t, ate.OTG(), p)
		handle, err := pcap.OpenOffline(packetCaptureGRE)
		if err != nil {
			t.Fatal(err)
		}
		defer handle.Close()
		packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
		validatePackets(t, packetSource, dstIPs, ttl, mplsLabel, protocolType)
	}
}

// validatePackets iterates through captured packets and validates protocol headers.
func validatePackets(t *testing.T, packetSource *gopacket.PacketSource, expectedIPs []string, expectedTTL, expectedLabel int, protocolType string) {
	t.Helper()
	var matched bool
	var lastErr string
	for packet := range packetSource.Packets() {
		var innerIPv4 *layers.IPv4
		var innerIPv6 *layers.IPv6
		ipv4Count, ipv6Count := 0, 0

		for _, l := range packet.Layers() {
			switch l.LayerType() {
			case layers.LayerTypeIPv4:
				ipv4Count++
				// second IPv4 = inner
				if ipv4Count == 2 {
					innerIPv4 = l.(*layers.IPv4)
				}
			case layers.LayerTypeIPv6:
				ipv6Count++
				// Using IPv4 for the GRE tunnel since IPv6 is not supported, so checking the first IPv6 (inner) occurrence instead.
				if ipv6Count == 1 {
					innerIPv6 = l.(*layers.IPv6)
				}
			}
		}

		switch protocolType {
		case "ipv4":
			if innerIPv4 == nil {
				lastErr = "missing inner IPv4 layer"
				continue
			}
			gotTTL := int(innerIPv4.TTL)
			gotDstIP := innerIPv4.DstIP.String()

			if slices.Contains(expectedIPs, gotDstIP) && gotTTL == expectedTTL {
				matched = true
			} else {
				lastErr = fmt.Sprintf("INNER IPv4 mismatch: GotIP=%s TTL=%d, ExpectedIPs=%v ExpectedTTL=%d", gotDstIP, gotTTL, expectedIPs, expectedTTL)
			}

		case "ipv6":
			if innerIPv6 == nil {
				lastErr = "missing inner IPv6 layer"
				continue
			}
			gotHopLimit := int(innerIPv6.HopLimit)
			gotDstIP := innerIPv6.DstIP.String()

			if slices.Contains(expectedIPs, gotDstIP) && gotHopLimit == expectedTTL {
				matched = true
			} else {
				lastErr = fmt.Sprintf("INNER IPv6 mismatch: GotIP=%s HopLimit=%d, ExpectedIPs=%v ExpectedTTL=%d", gotDstIP, gotHopLimit, expectedIPs, expectedTTL)
			}

		default:
			t.Fatalf("Unsupported protocol type: %s. Must be 'ipv4' or 'ipv6'", protocolType)
		}
		// Validate GRE layer
		if packet.Layer(layers.LayerTypeGRE) == nil {
			lastErr = "missing GRE header"
			continue
		}

		// Validate MPLS layer
		mplsLayer := packet.Layer(layers.LayerTypeMPLS)
		if mplsLayer == nil {
			lastErr = "missing MPLS header"
			continue
		}
		mplsPacket := mplsLayer.(*layers.MPLS)
		if mplsPacket.Label != uint32(expectedLabel) {
			lastErr = fmt.Sprintf("MPLS label mismatch: Got=%d, Expected=%d", mplsPacket.Label, expectedLabel)
			continue
		}
		matched = true
	}

	// Final result
	if matched {
		t.Logf("Packet validation succeeded for protocol=%s", protocolType)
	} else {
		t.Errorf("packet validation failed for protocol=%s: %s", protocolType, lastErr)
	}
}

// enableCapture configures packet capture on the provided OTG configuration for the specified port names. The captures are stored in PCAP format.
func enableCapture(t *testing.T, config gosnappi.Config, portNames []string) {
	t.Helper()
	config.Captures().Clear()
	for _, portName := range portNames {
		t.Logf("Configuring packet capture for OTG port: %s", portName)
		fcap := config.Captures().Add()
		fcap.SetName(portName)
		fcap.SetPortNames([]string{portName})
		fcap.SetFormat(gosnappi.CaptureFormat.PCAP)
	}
}

// startCapture initiates packet capture on all OTG ports defined in the config and returns the ControlState used to manage the capture session.
func startCapture(t *testing.T, otg *otg.OTG) gosnappi.ControlState {
	t.Helper()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	otg.SetControlState(t, cs)

	return cs
}

// stopCapture stops packet capture on all OTG ports using the provided ControlState from startCapture.
func stopCapture(t *testing.T, otg *otg.OTG, cs gosnappi.ControlState) {
	t.Helper()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	otg.SetControlState(t, cs)
}

// processCapture fetches captured traffic bytes from the specified OTG port, writes them to a temporary PCAP file, and returns the file path for analysis.
func processCapture(t *testing.T, otg *otg.OTG, port string) string {
	t.Helper()
	bytes := otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(port))
	time.Sleep(trafficDuration)
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
