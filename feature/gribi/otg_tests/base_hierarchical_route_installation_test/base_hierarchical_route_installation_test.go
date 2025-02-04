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

package base_hierarchical_route_installation_test

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the test
// topology.
//
// The testbed consists of ate:port1 -> dut:port1
// and dut:port2 -> ate:port2.
//
//   - ate:port1 -> dut:port1 subnet 192.0.2.0/30
//   - ate:port2 -> dut:port2 subnet 192.0.2.4/30
const (
	ipv4PrefixLen           = 30
	ateDstIP                = "198.51.100.1"
	ateDstNetCIDR           = ateDstIP + "/32"
	ateIndirectNH           = "203.0.113.1"
	ateIndirectNHCIDR       = ateIndirectNH + "/32"
	nhIndex                 = 1
	nhgIndex                = 42
	nhIndex2                = 2
	nhgIndex2               = 52
	nonDefaultVRF           = "VRF-1"
	nhMAC                   = "00:1A:11:00:0A:BC"
	macFilter               = "0xABC" // Hex equalent last 12 bits
	policyName              = "redirect-to-VRF1"
	niDecapTeVrf            = "DECAP_TE_VRF"
	niEncapTeVrfA           = "ENCAP_TE_VRF_A"
	niEncapTeVrfB           = "ENCAP_TE_VRF_B"
	niEncapTeVrfC           = "ENCAP_TE_VRF_C"
	niEncapTeVrfD           = "ENCAP_TE_VRF_D"
	vrfPolW                 = "vrf_selection_policy_w"
	niDefault               = "DEFAULT"
	dscpEncapA1             = 10
	dscpEncapA2             = 18
	dscpEncapB1             = 20
	dscpEncapB2             = 28
	dscpEncapNoMatch        = 30
	ipv4OuterSrc111WithMask = "198.51.100.111/32"
	ipv4OuterSrc222WithMask = "198.51.100.222/32"
	niTeVrf111              = "TE_VRF_111"
	niTeVrf222              = "TE_VRF_222"
	decapFlowSrc            = "198.51.100.111"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "port1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "port2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort2DummyIP = attrs.Attributes{
		Desc:       "dutPort2",
		IPv4Sec:    "192.0.2.21",
		IPv4LenSec: 30,
	}

	atePort2DummyIP = attrs.Attributes{
		Desc:    "atePort2",
		IPv4:    "192.0.2.22",
		IPv4Len: 32,
	}

	atePorts = map[string]attrs.Attributes{
		"port1": atePort1,
		"port2": atePort2,
	}
	dutPorts = map[string]attrs.Attributes{
		"port1": dutPort1,
		"port2": dutPort2,
	}
)

// configInterfaceDUT configures the interface with the Addrs.
func configInterfaceDUT(i *oc.Interface, a *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {
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
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(ipv4PrefixLen)

	return i
}

// configureNetworkInstance creates nonDefaultVRF
func configureNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(nonDefaultVRF)
	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(nonDefaultVRF).Config(), ni)

	fptest.ConfigureDefaultNetworkInstance(t, dut)

	// configure PBF in DEFAULT vrf
	defNIPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	gnmi.Replace(t, dut, defNIPath.PolicyForwarding().Config(), configurePBF(dut))
}

// configureNetworkInstance configures vrfs DECAP_TE_VRF,ENCAP_TE_VRF_A,ENCAP_TE_VRF_B,
// ENCAP_TE_VRF_C, ENCAP_TE_VRF_D, TE_VRF_111, TE_VRF_222
func configNonDefaultNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	c := &oc.Root{}
	vrfs := []string{niDecapTeVrf, niEncapTeVrfA, niEncapTeVrfB, niEncapTeVrfC, niEncapTeVrfD, niTeVrf111, niTeVrf222}
	for _, vrf := range vrfs {
		ni := c.GetOrCreateNetworkInstance(vrf)
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrf).Config(), ni)
	}
}

type policyFwRule struct {
	SeqId           uint32
	protocol        oc.UnionUint8
	dscpSet         []uint8
	sourceAddr      string
	decapNi         string
	postDecapNi     string
	decapFallbackNi string
}

func configureVrfSelectionPolicyW(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := &oc.Root{}
	dutPolFwdPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding()

	pfRule1 := &policyFwRule{SeqId: 1, protocol: 4, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfA, decapFallbackNi: niTeVrf222}
	pfRule2 := &policyFwRule{SeqId: 2, protocol: 41, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfA, decapFallbackNi: niTeVrf222}
	pfRule3 := &policyFwRule{SeqId: 3, protocol: 4, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfA, decapFallbackNi: niTeVrf111}
	pfRule4 := &policyFwRule{SeqId: 4, protocol: 41, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfA, decapFallbackNi: niTeVrf111}

	pfRule5 := &policyFwRule{SeqId: 5, protocol: 4, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfB, decapFallbackNi: niTeVrf222}
	pfRule6 := &policyFwRule{SeqId: 6, protocol: 41, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfB, decapFallbackNi: niTeVrf222}
	pfRule7 := &policyFwRule{SeqId: 7, protocol: 4, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfB, decapFallbackNi: niTeVrf111}
	pfRule8 := &policyFwRule{SeqId: 8, protocol: 41, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfB, decapFallbackNi: niTeVrf111}

	pfRule9 := &policyFwRule{SeqId: 9, protocol: 4, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niDefault, decapFallbackNi: niTeVrf222}
	pfRule10 := &policyFwRule{SeqId: 10, protocol: 41, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niDefault, decapFallbackNi: niTeVrf222}
	pfRule11 := &policyFwRule{SeqId: 11, protocol: 4, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niDefault, decapFallbackNi: niTeVrf111}
	pfRule12 := &policyFwRule{SeqId: 12, protocol: 41, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niDefault, decapFallbackNi: niTeVrf111}

	if deviations.PfRequireSequentialOrderPbrRules(dut) {
		pfRule10.SeqId = 910
		pfRule11.SeqId = 911
		pfRule12.SeqId = 912
	}

	pfRuleList := []*policyFwRule{pfRule1, pfRule2, pfRule3, pfRule4, pfRule5, pfRule6,
		pfRule7, pfRule8, pfRule9, pfRule10, pfRule11, pfRule12}

	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niP := ni.GetOrCreatePolicyForwarding()
	niPf := niP.GetOrCreatePolicy(vrfPolW)
	niPf.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)

	for _, pfRule := range pfRuleList {
		pfR := niPf.GetOrCreateRule(pfRule.SeqId)
		pfRProtoIPv4 := pfR.GetOrCreateIpv4()
		pfRProtoIPv4.Protocol = oc.UnionUint8(pfRule.protocol)
		if pfRule.dscpSet != nil {
			pfRProtoIPv4.DscpSet = pfRule.dscpSet
		}
		pfRProtoIPv4.SourceAddress = ygot.String(pfRule.sourceAddr)
		pfRAction := pfR.GetOrCreateAction()
		pfRAction.DecapNetworkInstance = ygot.String(pfRule.decapNi)
		pfRAction.PostDecapNetworkInstance = ygot.String(pfRule.postDecapNi)
		pfRAction.DecapFallbackNetworkInstance = ygot.String(pfRule.decapFallbackNi)
	}

	if deviations.PfRequireMatchDefaultRule(dut) {
		pfR13 := niPf.GetOrCreateRule(913)
		pfR13.GetOrCreateL2().SetEthertype(oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4)
		pfRAction := pfR13.GetOrCreateAction()
		pfRAction.NetworkInstance = ygot.String(niDefault)
		pfR14 := niPf.GetOrCreateRule(914)
		pfR14.GetOrCreateL2().SetEthertype(oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV6)
		pfRAction = pfR14.GetOrCreateAction()
		pfRAction.NetworkInstance = ygot.String(niDefault)
	} else {
		pfR := niPf.GetOrCreateRule(13)
		pfRAction := pfR.GetOrCreateAction()
		pfRAction.NetworkInstance = ygot.String(niDefault)
	}

	p1 := dut.Port(t, "port1")
	interfaceID := p1.Name()
	if deviations.InterfaceRefInterfaceIDFormat(dut) {
		interfaceID = interfaceID + ".0"
	}

	intf := niP.GetOrCreateInterface(interfaceID)
	intf.ApplyVrfSelectionPolicy = ygot.String(vrfPolW)
	intf.GetOrCreateInterfaceRef().Interface = ygot.String(p1.Name())
	intf.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)

	if deviations.InterfaceRefConfigUnsupported(dut) {
		intf.InterfaceRef = nil
	}

	gnmi.Replace(t, dut, dutPolFwdPath.Config(), niP)
}

// configureDUT configures port1 and port2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	for p, dp := range dutPorts {
		p1 := dut.Port(t, p)
		i1 := &oc.Interface{Name: ygot.String(p1.Name())}
		gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dp, dut))
		if deviations.ExplicitPortSpeed(dut) {
			fptest.SetPortSpeed(t, p1)
		}
		if deviations.ExplicitIPv6EnableForGRIBI(dut) {
			gnmi.Update(t, dut, d.Interface(p1.Name()).Subinterface(0).Ipv6().Enabled().Config(), bool(true))
		}
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		}
	}

	if deviations.GRIBIMACOverrideWithStaticARP(dut) {
		staticARPWithSecondaryIP(t, dut)
	}

	configureNetworkInstance(t, dut)

	// apply PBF to src interface.
	dp1 := dut.Port(t, "port1")
	applyForwardingPolicy(t, dp1.Name())
	if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		staticARPWithMagicUniversalIP(t, dut)
	}
}

// configurePBF returns a fully configured network-instance PF struct.
func configurePBF(dut *ondatra.DUTDevice) *oc.NetworkInstance_PolicyForwarding {
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	pf := ni.GetOrCreatePolicyForwarding()
	vrfPolicy := pf.GetOrCreatePolicy(policyName)
	vrfPolicy.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)
	vrfPolicy.GetOrCreateRule(1).GetOrCreateIpv4().SourceAddress = ygot.String(atePort1.IPv4 + "/32")
	vrfPolicy.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String(nonDefaultVRF)
	return pf
}

// applyForwardingPolicy applies the forwarding policy on the interface.
func applyForwardingPolicy(t *testing.T, ingressPort string) {
	t.Logf("Applying forwarding policy on interface %v ... ", ingressPort)
	d := &oc.Root{}
	dut := ondatra.DUT(t, "dut")
	interfaceID := ingressPort
	if deviations.InterfaceRefInterfaceIDFormat(dut) {
		interfaceID = ingressPort + ".0"
	}
	pfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Interface(interfaceID)
	pfCfg := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreatePolicyForwarding().GetOrCreateInterface(interfaceID)
	pfCfg.ApplyVrfSelectionPolicy = ygot.String(policyName)
	pfCfg.GetOrCreateInterfaceRef().Interface = ygot.String(ingressPort)
	pfCfg.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	if deviations.InterfaceRefConfigUnsupported(dut) {
		pfCfg.InterfaceRef = nil
	}
	gnmi.Replace(t, dut, pfPath.Config(), pfCfg)
}

// configureATE configures port1 and port2 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	top := gosnappi.NewConfig()
	for p, ap := range atePorts {
		dp := dutPorts[p]
		top.Ports().Add().SetName(ap.Name)
		i1 := top.Devices().Add().SetName(ap.Name)
		eth1 := i1.Ethernets().Add().SetName(ap.Name + ".Eth").SetMac(ap.MAC)
		eth1.Connection().SetPortName(i1.Name())
		eth1.Ipv4Addresses().Add().SetName(i1.Name() + ".IPv4").
			SetAddress(ap.IPv4).SetGateway(dp.IPv4).
			SetPrefix(uint32(ap.IPv4Len))
	}
	return top
}

// createFlow returns a flow name from atePort1 to the dstPfx.
func createFlow(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, name string) gosnappi.Flow {
	flow := gosnappi.NewFlow().SetName(name)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{atePort2.Name + ".IPv4"})
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(atePort1.MAC)
	v4 := flow.Packet().Add().Ipv4()
	v4.Src().SetValue(atePort1.IPv4)
	v4.Dst().Increment().SetStart(ateDstIP).SetCount(1)

	if name == "transitFlow" {
		v4.Src().SetValue(decapFlowSrc)
		v4.Priority().Dscp().Phb().SetValues([]uint32{dscpEncapA1})
		innerIpHdr := flow.Packet().Add().Ipv4()
		innerIpHdr.Src().SetValue(atePort1.IPv4)
		innerIpHdr.Dst().SetValue(atePort2.IPv4)
	}
	eth := flow.EgressPacket().Add().Ethernet()
	ethTag := eth.Dst().MetricTags().Add()
	ethTag.SetName("EgressTrackingFlow").SetOffset(36).SetLength(12)
	return flow
}

// validateTraffic will return loss percentage of traffic
func ValidateTraffic(t *testing.T, ate *ondatra.ATEDevice, flow gosnappi.Flow, flowFilter string) float32 {
	top := ate.OTG().FetchConfig(t)
	top.Flows().Clear()
	top.Flows().Append(flow)
	ate.OTG().PushConfig(t, top)

	ate.OTG().StartProtocols(t)
	ate.OTG().StartTraffic(t)
	time.Sleep(15 * time.Second)
	ate.OTG().StopTraffic(t)
	time.Sleep(45 * time.Second)

	otgutils.LogFlowMetrics(t, ate.OTG(), top)

	txPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State())
	rxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).Counters().InPkts().State())
	lossPct := float32(txPkts-rxPkts) * 100 / float32(txPkts)
	if int(lossPct) == 0 && flowFilter != "" {
		etPath := gnmi.OTG().Flow(flow.Name()).TaggedMetricAny()
		ets := gnmi.GetAll(t, ate.OTG(), etPath.State())
		if got := len(ets); got != 1 {
			t.Errorf("EgressTracking got %d items, want %d", got, 1)
		}
		etTagspath := gnmi.OTG().Flow(flow.Name()).TaggedMetricAny().TagsAny()
		etTags := gnmi.GetAll(t, ate.OTG(), etTagspath.State())
		if got := etTags[0].GetTagValue().GetValueAsHex(); !strings.EqualFold(got, macFilter) {
			t.Errorf("EgressTracking filter got %q, want %q", got, macFilter)
		}
		if got := ets[0].GetCounters().GetInPkts(); got != rxPkts {
			t.Errorf("EgressTracking counter in-pkts got %d, want %d", got, rxPkts)
		}
	}
	return float32(lossPct)
}

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx    context.Context
	client *gribi.Client
	dut    *ondatra.DUTDevice
	ate    *ondatra.ATEDevice
	top    gosnappi.Config
}

// verifyTelemetry verifies the telemetry for route recursilvely
func verifyTelemetry(t *testing.T, args *testArgs, nhtype string, vrfName string) {

	// Verify that the entry for 198.51.100.1/32 (a) is installed through AFT Telemetry. a->c or a->b are the expected results.
	ipv4Entry := gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(vrfName).Afts().Ipv4Entry(ateDstNetCIDR).State())
	if got, want := ipv4Entry.GetPrefix(), ateDstNetCIDR; got != want {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/prefix = %v, want %v", got, want)
	}
	if got, want := ipv4Entry.GetOriginProtocol(), oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_GRIBI; got != want {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/origin-protocol = %v, want %v", got, want)
	}
	if got, want := ipv4Entry.GetNextHopGroupNetworkInstance(), deviations.DefaultNetworkInstance(args.dut); got != want {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/next-hop-group-network-instance = %v, want %v", got, want)
	}
	nhgIndexInst := ipv4Entry.GetNextHopGroup()
	if nhgIndexInst == 0 {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/next-hop-group is not present")
	}
	nhg := gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Afts().NextHopGroup(nhgIndexInst).State())
	if got, want := nhg.GetProgrammedId(), uint64(nhgIndex); got != want {
		t.Errorf("TestRecursiveIPv4Entry: next-hop-group/state/programmed-id = %v, want %v", got, want)
	}

	for nhIndexInst, nhgNH := range nhg.NextHop {
		if got, want := nhgNH.GetIndex(), uint64(nhIndexInst); got != want {
			t.Errorf("next-hop index is incorrect: got %v, want %v", got, want)
		}
		nh := gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Afts().NextHop(nhIndexInst).State())
		// for devices that return  the nexthop with resolving it recursively. For a->b->c the device returns c
		if got := nh.GetIpAddress(); got != ateIndirectNH {
			if nhtype == "MAC" {
				if gotMac := nh.GetMacAddress(); !strings.EqualFold(gotMac, nhMAC) {
					t.Errorf("next-hop MAC is incorrect:  gotMac %v, wantMac %v", gotMac, nhMAC)
				}
			} else {
				if got := nh.GetIpAddress(); got != atePort2.IPv4 {
					t.Errorf("next-hop is incorrect: got %v, want %v ", got, atePort2.IPv4)
				}
			}
		}
	}

	// Verify that the entry for 203.0.113.1/32 (b) is installed through AFT Telemetry.
	ipv4Entry = gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Afts().Ipv4Entry(ateIndirectNHCIDR).State())
	if got, want := ipv4Entry.GetPrefix(), ateIndirectNHCIDR; got != want {
		t.Errorf("TestRecursiveIPv4Entry = %v: ipv4-entry/state/prefix, want %v", got, want)
	}
	if got, want := ipv4Entry.GetOriginProtocol(), oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_GRIBI; got != want {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/origin-protocol = %v, want %v", got, want)
	}
	if got, want := ipv4Entry.GetNextHopGroupNetworkInstance(), deviations.DefaultNetworkInstance(args.dut); got != want {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/next-hop-group-network-instance = %v, want %v", got, want)
	}
	nhgIndexInst = ipv4Entry.GetNextHopGroup()
	if nhgIndexInst == 0 {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/next-hop-group is not present")
	}
	nhg = gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Afts().NextHopGroup(nhgIndexInst).State())
	if got, want := nhg.GetProgrammedId(), uint64(nhgIndex2); got != want {
		t.Errorf("TestRecursiveIPv4Entry: next-hop-group/state/programmed-id = %v, want %v", got, want)
	}

	for nhIndexInst, nhgNH := range nhg.NextHop {
		if got, want := nhgNH.GetIndex(), uint64(nhIndexInst); got != want {
			t.Errorf("next-hop index is incorrect: got %v, want %v", got, want)
		}
		nh := gnmi.Get(t, args.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Afts().NextHop(nhIndexInst).State())
		if nhtype == "MAC" {
			if deviations.GRIBIMACOverrideWithStaticARP(args.dut) || deviations.GRIBIMACOverrideStaticARPStaticRoute(args.dut) {
				continue
			}
			if got, want := nh.GetMacAddress(), nhMAC; !strings.EqualFold(got, want) {
				t.Errorf("next-hop MAC is incorrect: got %v, want %v", got, want)
			}
		} else {
			if got, want := nh.GetIpAddress(), atePort2.IPv4; got != want {
				t.Errorf("next-hop address is incorrect: got %v, want %v", got, want)
			}
		}
	}
}

// testRecursiveIPv4Entry verifies recursive IPv4 Entry for 198.51.100.1/32 (a) -> 203.0.113.1/32 (b) -> 192.0.2.6 (c).
// The IPv4 Entry is verified through AFT Telemetry and Traffic.
func testRecursiveIPv4EntrywithIPNexthop(t *testing.T, args *testArgs) {

	t.Logf("Adding IP %v with NHG %d NH %d with IP %v as NH via gRIBI", ateIndirectNH, nhgIndex2, nhIndex2, atePort2.IPv4)
	nh, op1 := gribi.NHEntry(nhIndex2, atePort2.IPv4, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	nhg, op2 := gribi.NHGEntry(nhgIndex2, map[uint64]uint64{nhIndex2: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddEntries(t, []fluent.GRIBIEntry{nh, nhg}, []*client.OpResult{op1, op2})
	args.client.AddIPv4(t, ateIndirectNHCIDR, nhgIndex2, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Logf("Adding IP %v with NHG %d NH %d  with indirect IP %v via gRIBI", ateDstNetCIDR, nhgIndex, nhIndex, ateIndirectNHCIDR)
	nh, op1 = gribi.NHEntry(nhIndex, ateIndirectNH, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	nhg, op2 = gribi.NHGEntry(nhgIndex, map[uint64]uint64{nhIndex: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddEntries(t, []fluent.GRIBIEntry{nh, nhg}, []*client.OpResult{op1, op2})
	args.client.AddIPv4(t, ateDstNetCIDR, nhgIndex, nonDefaultVRF, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	baseFlow := createFlow(t, args.ate, args.top, "BaseFlow")

	time.Sleep(30 * time.Second)

	t.Run("ValidateTelemtry", func(t *testing.T) {
		t.Log("Validate Telemetry to verify IPV4 entry is resolved through IP next-hop")
		verifyTelemetry(t, args, "IP", nonDefaultVRF)
	})

	t.Run("ValidateTraffic", func(t *testing.T) {
		t.Log("Validate Traffic is recieved on atePort2 with IP next-hop")
		if got, want := ValidateTraffic(t, args.ate, baseFlow, ""), 0; int(got) != want {
			t.Errorf("Loss: got %v, want %v", got, want)
		}
	})

	t.Logf("Deleting NH entry and verifing there is no traffic")
	args.client.DeleteIPv4(t, ateIndirectNHCIDR, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	ipv4Path := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Afts().Ipv4Entry(ateIndirectNHCIDR)
	if gnmi.Lookup(t, args.dut, ipv4Path.State()).IsPresent() {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/prefix: Found route %s that should not exist", ateIndirectNHCIDR)
	}
	t.Run("ValidateNoTrafficAfterNHDelete", func(t *testing.T) {
		t.Log("Validate No traffic Traffic is recieved on atePort2 after NH delete")
		if got, want := ValidateTraffic(t, args.ate, baseFlow, ""), 100; int(got) != want {
			t.Errorf("Loss: got %v, want %v", got, want)
		}
	})
}

// testRecursiveIPv4EntrywithMACNexthop verifies recursive IPv4 Entry for 198.51.100.1/32 (a) -> 203.0.113.1/32 (b) -> Port1 + MAC
// The IPv4 Entry is verified through AFT Telemetry and Traffic.
func testRecursiveIPv4EntrywithMACNexthop(t *testing.T, args *testArgs) {

	p := args.dut.Port(t, "port2")
	t.Logf("Adding IP %v with NHG %d NH %d with interface %v and MAC %v as NH via gRIBI", ateIndirectNH, nhgIndex2, nhIndex2, p.Name(), nhMAC)
	var nh fluent.GRIBIEntry
	var op1 *client.OpResult
	switch {
	case deviations.GRIBIMACOverrideStaticARPStaticRoute(args.dut):
		nh, op1 = gribi.NHEntry(nhIndex2, "MACwithInterface", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Interface: p.Name(), Mac: nhMAC, Dest: atePort2DummyIP.IPv4})
	case deviations.GRIBIMACOverrideWithStaticARP(args.dut):
		nh, op1 = gribi.NHEntry(nhIndex2, "MACwithIp", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Mac: nhMAC, Dest: atePort2DummyIP.IPv4})
	default:
		nh, op1 = gribi.NHEntry(nhIndex2, "MACwithInterface", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Interface: p.Name(), Mac: nhMAC})
	}
	nhg, op2 := gribi.NHGEntry(nhgIndex2, map[uint64]uint64{nhIndex2: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddEntries(t, []fluent.GRIBIEntry{nh, nhg}, []*client.OpResult{op1, op2})
	args.client.AddIPv4(t, ateIndirectNHCIDR, nhgIndex2, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Logf("Adding IP %v with NHG %d NH %d  with indirect IP %v via gRIBI", ateDstNetCIDR, nhgIndex, nhIndex, ateIndirectNHCIDR)
	nh, op1 = gribi.NHEntry(nhIndex, ateIndirectNH, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	nhg, op2 = gribi.NHGEntry(nhgIndex, map[uint64]uint64{nhIndex: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddEntries(t, []fluent.GRIBIEntry{nh, nhg}, []*client.OpResult{op1, op2})
	args.client.AddIPv4(t, ateDstNetCIDR, nhgIndex, nonDefaultVRF, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	baseFlow := createFlow(t, args.ate, args.top, "BaseFlow")
	time.Sleep(30 * time.Second)
	t.Run("ValidateTelemtry", func(t *testing.T) {
		t.Log("Validate Telemetry to verify IPV4 entry is resolved through MAC next-hop")
		verifyTelemetry(t, args, "MAC", nonDefaultVRF)
	})
	t.Run("ValidateTraffic", func(t *testing.T) {
		t.Log("Validate Traffic is recieved on atePort2 with dst MAC as gRIBI NH MAC")
		if got, want := ValidateTraffic(t, args.ate, baseFlow, macFilter), 0; int(got) != want {
			t.Errorf("Loss: got %v, want %v", got, want)
		}
	})

	t.Logf("Deleting NH entry and verifing there is no traffic")
	args.client.DeleteIPv4(t, ateIndirectNHCIDR, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	ipv4Path := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Afts().Ipv4Entry(ateIndirectNHCIDR)
	if gnmi.Lookup(t, args.dut, ipv4Path.State()).IsPresent() {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/prefix: Found route %s that should not exist", ateIndirectNHCIDR)
	}
	t.Run("ValidateNoTrafficAfterNHDelete", func(t *testing.T) {
		t.Log("Validate No traffic Traffic is recieved on atePort2 after NH delete")
		if got, want := ValidateTraffic(t, args.ate, baseFlow, macFilter), 100; int(got) != want {
			t.Errorf("Loss: got %v, want %v", got, want)
		}
	})
}

// testRecursiveIPv4EntrywithVrfPolW verifies recursive IPv4 Entry for
// 198.51.100.1/32 (a) with vrf selection w
func testRecursiveIPv4EntrywithVrfPolW(t *testing.T, args *testArgs) {

	if deviations.SkipPbfWithDecapEncapVrf(args.dut) {

		t.Skip("Skipping Test as it is not supported")
	}
	t.Log("Delete existing vrf selection policy and Apply vrf selectioin policy W")
	configNonDefaultNetworkInstance(t, args.dut)
	configureVrfSelectionPolicyW(t, args.dut)

	t.Logf("Adding IP %v with NHG %d NH %d with IP %v as NH via gRIBI", ateIndirectNH, nhgIndex2, nhIndex2, atePort2.IPv4)
	nh, op1 := gribi.NHEntry(nhIndex2, atePort2.IPv4, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	nhg, op2 := gribi.NHGEntry(nhgIndex2, map[uint64]uint64{nhIndex2: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddEntries(t, []fluent.GRIBIEntry{nh, nhg}, []*client.OpResult{op1, op2})
	args.client.AddIPv4(t, ateIndirectNHCIDR, nhgIndex2, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Logf("Adding IP %v with NHG %d NH %d  with indirect IP %v via gRIBI", ateDstNetCIDR, nhgIndex, nhIndex, ateIndirectNHCIDR)
	nh, op1 = gribi.NHEntry(nhIndex, ateIndirectNH, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	nhg, op2 = gribi.NHGEntry(nhgIndex, map[uint64]uint64{nhIndex: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddEntries(t, []fluent.GRIBIEntry{nh, nhg}, []*client.OpResult{op1, op2})
	args.client.AddIPv4(t, ateDstNetCIDR, nhgIndex, niTeVrf111, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	baseFlow := createFlow(t, args.ate, args.top, "transitFlow")

	time.Sleep(30 * time.Second)

	t.Run("ValidateTelemtry", func(t *testing.T) {
		t.Log("Validate Telemetry to verify IPV4 entry is resolved through IP next-hop")
		verifyTelemetry(t, args, "IP", niTeVrf111)
	})

	t.Run("ValidateTraffic", func(t *testing.T) {
		t.Log("Validate Traffic is recieved on atePort2 with IP next-hop")
		if got, want := ValidateTraffic(t, args.ate, baseFlow, ""), 0; int(got) != want {
			t.Errorf("Loss: got %v, want %v", got, want)
		}
	})

	t.Logf("Deleting NH entry and verifing there is no traffic")
	args.client.DeleteIPv4(t, ateIndirectNHCIDR, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	ipv4Path := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Afts().Ipv4Entry(ateIndirectNHCIDR)
	if gnmi.Lookup(t, args.dut, ipv4Path.State()).IsPresent() {
		t.Errorf("TestRecursiveIPv4Entry: ipv4-entry/state/prefix: Found route %s that should not exist", ateIndirectNHCIDR)
	}
	t.Run("ValidateNoTrafficAfterNHDelete", func(t *testing.T) {
		t.Log("Validate No traffic Traffic is recieved on atePort2 after NH delete")
		if got, want := ValidateTraffic(t, args.ate, baseFlow, ""), 100; int(got) != want {
			t.Errorf("Loss: got %v, want %v", got, want)
		}
	})
}

func staticARPWithMagicUniversalIP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p2 := dut.Port(t, "port2")
	s2 := &oc.NetworkInstance_Protocol_Static{
		Prefix: ygot.String(atePort2DummyIP.IPv4CIDR()),
		NextHop: map[string]*oc.NetworkInstance_Protocol_Static_NextHop{
			strconv.Itoa(nhIndex2): {
				Index: ygot.String(strconv.Itoa(nhIndex2)),
				InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
					Interface: ygot.String(p2.Name()),
				},
			},
		},
	}
	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	static, ok := gnmi.LookupConfig(t, dut, sp.Config()).Val()
	if !ok || static == nil {
		static = &oc.NetworkInstance_Protocol{
			Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
			Name:       ygot.String(deviations.StaticProtocolName(dut)),
			Static: map[string]*oc.NetworkInstance_Protocol_Static{
				atePort2DummyIP.IPv4CIDR(): s2,
			},
		}
		gnmi.Replace(t, dut, sp.Config(), static)
	} else {
		gnmi.Replace(t, dut, sp.Static(atePort2DummyIP.IPv4CIDR()).Config(), s2)
	}
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), configStaticArp(p2, atePort2DummyIP.IPv4, nhMAC))
}

func staticARPWithSecondaryIP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p2 := dut.Port(t, "port2")
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2DummyIP.NewOCInterface(p2.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), configStaticArp(p2, atePort2DummyIP.IPv4, nhMAC))
}

func configStaticArp(p *ondatra.Port, ipv4addr string, macAddr string) *oc.Interface {
	i := &oc.Interface{Name: ygot.String(p.Name())}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	n4 := s4.GetOrCreateNeighbor(ipv4addr)
	n4.LinkLayerAddress = ygot.String(macAddr)
	return i
}

func TestRecursiveIPv4Entries(t *testing.T) {

	ctx := context.Background()

	// Configure DUT
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	defer func() {
		if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
			sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
			gnmi.Delete(t, dut, sp.Static(atePort2DummyIP.IPv4CIDR()).Config())
		}
	}()

	// Configure ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	tests := []struct {
		name string
		desc string
		fn   func(t *testing.T, args *testArgs)
	}{
		{
			name: "testRecursiveIPv4EntrywithIPNexthop",
			desc: "Program IPV4 entry recursively to IP next-hop and verify with Telemetry and Traffic.",
			fn:   testRecursiveIPv4EntrywithIPNexthop,
		},
		{
			name: "testRecursiveIPv4EntrywithMACNexthop",
			desc: "Program IPV4 entry recursively to MAC next-hop and verify with Telemetry and Traffic",
			fn:   testRecursiveIPv4EntrywithMACNexthop,
		},
		{
			name: "testRecursiveIPv4EntrywithVRFSelectionPolW",
			desc: "Program IPV4 entry with VRF Selection Policy W and verify with Telemetry and Traffic.",
			fn:   testRecursiveIPv4EntrywithVrfPolW,
		},
	}

	// Each case will run with its own gRIBI fluent client.
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Name: %s", tc.name)
			t.Logf("Description: %s", tc.desc)

			// Configure the gRIBI client
			client := gribi.Client{
				DUT:         dut,
				FIBACK:      true,
				Persistence: true,
			}

			defer client.Close(t)
			if err := client.Start(t); err != nil {
				t.Fatalf("gRIBI Connection can not be established")
			}
			client.BecomeLeader(t)

			defer client.FlushAll(t)

			args := &testArgs{
				ctx:    ctx,
				dut:    dut,
				ate:    ate,
				top:    top,
				client: &client,
			}
			tc.fn(t, args)
		})
	}
}
