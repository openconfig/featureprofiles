// Copyright 2023 Google LLC
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

package vrf_policy_driven_te_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the test
// topology.
//
// ATE port-1 <------> port-1 DUT
// DUT port-2 <------> port-2 ATE
// DUT port-3 <------> port-3 ATE
// DUT port-4 <------> port-4 ATE
// DUT port-5 <------> port-5 ATE
// DUT port-6 <------> port-6 ATE
// DUT port-7 <------> port-7 ATE
// DUT port-8 <------> port-8 ATE

const (
	plenIPv4                = 30
	plenIPv6                = 126
	maskLen24               = "24"
	maskLen32               = "32"
	maskLen126              = "126"
	dscpEncapA1             = 10
	dscpEncapA2             = 18
	dscpEncapB1             = 20
	dscpEncapB2             = 28
	dscpEncapNoMatch        = 30
	ipv4OuterSrc111WithMask = "198.51.100.111/32"
	ipv4OuterSrc222WithMask = "198.51.100.222/32"
	magicIp                 = "192.168.1.1"
	magicMac                = "02:00:00:00:00:01"
	gribiIPv4EntryDefVRF1   = "192.0.2.101"
	gribiIPv4EntryDefVRF2   = "192.0.2.102"
	gribiIPv4EntryDefVRF3   = "192.0.2.103"
	gribiIPv4EntryDefVRF4   = "192.0.2.104"
	gribiIPv4EntryDefVRF5   = "192.0.2.105"
	gribiIPv4EntryVRF1111   = "203.0.113.1"
	gribiIPv4EntryVRF1112   = "203.10.113.2"
	gribiIPv4EntryVRF2221   = "203.0.113.100"
	gribiIPv4EntryVRF2222   = "203.0.113.101"
	gribiIPv4EntryEncapVRF  = "138.0.11.0"
	gribiIPv6EntryEncapVRF  = "2001:db8::138:0:11:0"
	ipv4OuterDst111         = "192.51.100.64"
	ipv4OuterSrc111         = "198.51.100.111"
	ipv4OuterSrc222         = "198.51.100.222"
	ipv4OuterSrc333         = "198.100.200.123"
	prot4                   = 4
	prot41                  = 41
	vrfPolW                 = "vrf_selection_policy_w"
	vrfPolC                 = "vrf_selection_policy_c"
	nhIndex                 = 1
	nhgIndex                = 1
	niDecapTeVrf            = "DECAP_TE_VRF"
	niEncapTeVrfA           = "ENCAP_TE_VRF_A"
	niEncapTeVrfB           = "ENCAP_TE_VRF_B"
	niTeVrf111              = "TE_VRF_111"
	niTeVrf222              = "TE_VRF_222"
	niDefault               = "DEFAULT"
	tolerancePct            = 2
	flowNegTest             = "flowNegTest"
	ipv4InnerDst            = "138.0.11.8"
	ipv6InnerDst            = "2001:db8::138:0:11:8"
	ipv4InnerDstNoEncap     = "20.0.0.1"
	ipv6InnerDstNoEncap     = "2001:db8::20:0:0:1"
	ipv4InnerDst2           = "138.0.11.15"
	ipv6InnerDst2           = "2001:db8::138:0:11:15"
	defaultRoute            = "0.0.0.0/0"
	wantLoss                = true
	routeDelete             = true
	correspondingTTL        = 64
	correspondingHopLimit   = 64
	flow6in4                = "flow6in4"
	flow4in4                = "flow4in4"
	v4Flow                  = true
	dutAreaAddress          = "49.0001"
	dutSysID                = "1920.0000.2001"
	otgSysID1               = "640000000001"
	isisInstance            = "DEFAULT"
	otgIsisPort8LoopV4      = "203.0.113.10"
	otgIsisPort8LoopV6      = "2001:db8::203:0:113:10"
	dutAS                   = 65501
	peerGrpName1            = "BGP-PEER-GROUP1"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "192.0.2.2",
		MAC:     "02:00:01:01:01:01",
		IPv6:    "2001:db8::192:0:2:2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::192:2:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "192.0.2.6",
		MAC:     "02:00:02:01:01:01",
		IPv6:    "2001:db8::192:2:2:2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort3 = attrs.Attributes{
		Desc:    "dutPort3",
		IPv4:    "192.0.2.9",
		IPv6:    "2001:db8::192:3:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		IPv4:    "192.0.2.10",
		MAC:     "02:00:03:01:01:01",
		IPv6:    "2001:db8::192:3:2:2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort4 = attrs.Attributes{
		Desc:    "dutPort4",
		IPv4:    "192.0.2.13",
		IPv6:    "2001:db8::192:4:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort4 = attrs.Attributes{
		Name:    "atePort4",
		IPv4:    "192.0.2.14",
		MAC:     "02:00:04:01:01:01",
		IPv6:    "2001:db8::192:4:2:2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort5 = attrs.Attributes{
		Desc:    "dutPort5",
		IPv4:    "192.0.2.17",
		IPv6:    "2001:db8::192:5:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort5 = attrs.Attributes{
		Name:    "atePort5",
		IPv4:    "192.0.2.18",
		MAC:     "02:00:05:01:01:01",
		IPv6:    "2001:db8::192:5:2:2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort6 = attrs.Attributes{
		Desc:    "dutPort6",
		IPv4:    "192.0.2.21",
		IPv6:    "2001:db8::192:6:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort6 = attrs.Attributes{
		Name:    "atePort6",
		IPv4:    "192.0.2.22",
		MAC:     "02:00:06:01:01:01",
		IPv6:    "2001:db8::192:6:2:2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort7 = attrs.Attributes{
		Desc:    "dutPort7",
		IPv4:    "192.0.2.25",
		IPv6:    "2001:db8::192:7:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort7 = attrs.Attributes{
		Name:    "atePort7",
		IPv4:    "192.0.2.26",
		MAC:     "02:00:07:01:01:01",
		IPv6:    "2001:db8::192:7:2:2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort8 = attrs.Attributes{
		Desc:    "dutPort8",
		IPv4:    "192.0.2.29",
		IPv6:    "2001:db8::192:8:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort8 = attrs.Attributes{
		Name:    "atePort8",
		IPv4:    "192.0.2.30",
		MAC:     "02:00:08:01:01:01",
		IPv6:    "2001:db8::192:8:2:2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutlo0Attrs = attrs.Attributes{
		Desc:    "Loopback ip",
		IPv4:    "203.0.113.11",
		IPv6:    "2001:db8::203:0:113:1",
		IPv4Len: 32,
		IPv6Len: 128,
	}
	loopbackIntfName string
	// TODO : https://github.com/open-traffic-generator/fp-testbed-juniper/issues/42
	// Below code will be uncommented once ixia issue is fixed.
	//tolerance        = 0.2
	bgp4Peer gosnappi.BgpV4Peer
)

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, t testing.TB, c *fluent.GRIBIClient, timeout time.Duration) error {
	t.Helper()
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}

type testArgs struct {
	ctx        context.Context
	client     *fluent.GRIBIClient
	dut        *ondatra.DUTDevice
	ate        *ondatra.ATEDevice
	otgConfig  gosnappi.Config
	top        gosnappi.Config
	electionID gribi.Uint128
	otg        *otg.OTG
}

type policyFwRule struct {
	SeqId           uint32
	family          string
	protocol        oc.UnionUint8
	dscpSet         []uint8
	sourceAddr      string
	decapNi         string
	postDecapNi     string
	decapFallbackNi string
	ni              string
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
	pfR := niPf.GetOrCreateRule(13)
	pfRAction := pfR.GetOrCreateAction()
	pfRAction.NetworkInstance = ygot.String(niDefault)

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

func deleteVrfSelectionPolicy(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Config())
}

func configureVrfSelectionPolicyC(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := &oc.Root{}
	dutPolFwdPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding()

	pfRule1 := &policyFwRule{SeqId: 1, family: "ipv4", protocol: 4, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfA, decapFallbackNi: niTeVrf222}
	pfRule2 := &policyFwRule{SeqId: 2, family: "ipv4", protocol: 41, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfA, decapFallbackNi: niTeVrf222}
	pfRule3 := &policyFwRule{SeqId: 3, family: "ipv4", protocol: 4, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfA, decapFallbackNi: niTeVrf111}
	pfRule4 := &policyFwRule{SeqId: 4, family: "ipv4", protocol: 41, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfA, decapFallbackNi: niTeVrf111}

	pfRule5 := &policyFwRule{SeqId: 5, family: "ipv4", protocol: 4, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfB, decapFallbackNi: niTeVrf222}
	pfRule6 := &policyFwRule{SeqId: 6, family: "ipv4", protocol: 41, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfB, decapFallbackNi: niTeVrf222}
	pfRule7 := &policyFwRule{SeqId: 7, family: "ipv4", protocol: 4, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfB, decapFallbackNi: niTeVrf111}
	pfRule8 := &policyFwRule{SeqId: 8, family: "ipv4", protocol: 41, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfB, decapFallbackNi: niTeVrf111}

	pfRule9 := &policyFwRule{SeqId: 9, family: "ipv4", protocol: 4, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niDefault, decapFallbackNi: niTeVrf222}
	pfRule10 := &policyFwRule{SeqId: 10, family: "ipv4", protocol: 41, sourceAddr: ipv4OuterSrc222WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niDefault, decapFallbackNi: niTeVrf222}
	pfRule11 := &policyFwRule{SeqId: 11, family: "ipv4", protocol: 4, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niDefault, decapFallbackNi: niTeVrf111}
	pfRule12 := &policyFwRule{SeqId: 12, family: "ipv4", protocol: 41, sourceAddr: ipv4OuterSrc111WithMask,
		decapNi: niDecapTeVrf, postDecapNi: niDefault, decapFallbackNi: niTeVrf111}

	pfRule13 := &policyFwRule{SeqId: 13, family: "ipv4", dscpSet: []uint8{dscpEncapA1, dscpEncapA2},
		ni: niEncapTeVrfA}
	pfRule14 := &policyFwRule{SeqId: 14, family: "ipv6", dscpSet: []uint8{dscpEncapA1, dscpEncapA2},
		ni: niEncapTeVrfA}
	pfRule15 := &policyFwRule{SeqId: 15, family: "ipv4", dscpSet: []uint8{dscpEncapA1, dscpEncapA2},
		ni: niEncapTeVrfB}
	pfRule16 := &policyFwRule{SeqId: 16, family: "ipv6", dscpSet: []uint8{dscpEncapA1, dscpEncapA2},
		ni: niEncapTeVrfB}
	pfRule17 := &policyFwRule{SeqId: 17, ni: niDefault}

	pfRuleList := []*policyFwRule{pfRule1, pfRule2, pfRule3, pfRule4, pfRule5, pfRule6,
		pfRule7, pfRule8, pfRule9, pfRule10, pfRule11, pfRule12, pfRule13, pfRule14,
		pfRule15, pfRule16, pfRule17}

	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niP := ni.GetOrCreatePolicyForwarding()
	niPf := niP.GetOrCreatePolicy(vrfPolC)
	niPf.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)

	for _, pfRule := range pfRuleList {
		pfR := niPf.GetOrCreateRule(pfRule.SeqId)

		if pfRule.family == "ipv4" {
			pfRProtoIP := pfR.GetOrCreateIpv4()
			if pfRule.protocol != 0 {
				pfRProtoIP.Protocol = oc.UnionUint8(pfRule.protocol)
			}
			if pfRule.sourceAddr != "" {
				pfRProtoIP.SourceAddress = ygot.String(pfRule.sourceAddr)
			}
			if pfRule.dscpSet != nil {
				pfRProtoIP.DscpSet = pfRule.dscpSet
			}
		} else if pfRule.family == "ipv6" {
			pfRProtoIP := pfR.GetOrCreateIpv6()
			if pfRule.dscpSet != nil {
				pfRProtoIP.DscpSet = pfRule.dscpSet
			}
		}

		pfRAction := pfR.GetOrCreateAction()
		if pfRule.decapNi != "" {
			pfRAction.DecapNetworkInstance = ygot.String(pfRule.decapNi)
		}
		if pfRule.postDecapNi != "" {
			pfRAction.PostDecapNetworkInstance = ygot.String(pfRule.postDecapNi)
		}
		if pfRule.decapFallbackNi != "" {
			pfRAction.DecapFallbackNetworkInstance = ygot.String(pfRule.decapFallbackNi)
		}
		if pfRule.ni != "" {
			pfRAction.NetworkInstance = ygot.String(pfRule.ni)
		}
	}

	p1 := dut.Port(t, "port1")
	interfaceID := p1.Name()
	if deviations.InterfaceRefInterfaceIDFormat(dut) {
		interfaceID = interfaceID + ".0"
	}
	intf := niP.GetOrCreateInterface(interfaceID)
	intf.ApplyVrfSelectionPolicy = ygot.String(vrfPolC)
	intf.GetOrCreateInterfaceRef().Interface = ygot.String(p1.Name())
	intf.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	if deviations.InterfaceRefConfigUnsupported(dut) {
		intf.InterfaceRef = nil
	}
	gnmi.Replace(t, dut, dutPolFwdPath.Config(), niP)
}

// configStaticArp configures static arp entries
func configStaticArp(p string, ipv4addr string, macAddr string) *oc.Interface {
	i := &oc.Interface{Name: ygot.String(p)}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	n4 := s4.GetOrCreateNeighbor(ipv4addr)
	n4.LinkLayerAddress = ygot.String(macAddr)
	return i
}

func staticARPWithMagicUniversalIP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")
	p5 := dut.Port(t, "port5")
	p6 := dut.Port(t, "port6")
	p7 := dut.Port(t, "port7")
	portList := []*ondatra.Port{p2, p3, p4, p5, p6, p7}
	for idx, p := range portList {
		s := &oc.NetworkInstance_Protocol_Static{
			Prefix: ygot.String(magicIp + "/32"),
			NextHop: map[string]*oc.NetworkInstance_Protocol_Static_NextHop{
				strconv.Itoa(idx): {
					Index: ygot.String(strconv.Itoa(idx)),
					InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
						Interface: ygot.String(p.Name()),
					},
				},
			},
		}
		sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
		gnmi.Update(t, dut, sp.Static(magicIp+"/32").Config(), s)
		gnmi.Update(t, dut, gnmi.OC().Interface(p.Name()).Config(), configStaticArp(p.Name(), magicIp, magicMac))
	}
}

// configureNetworkInstance configures vrfs DECAP_TE_VRF,ENCAP_TE_VRF_A,ENCAP_TE_VRF_B,
// TE_VRF_222, TE_VRF_111.
func configNonDefaultNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	c := &oc.Root{}
	vrfs := []string{niDecapTeVrf, niEncapTeVrfA, niEncapTeVrfB, niTeVrf222, niTeVrf111}
	for _, vrf := range vrfs {
		ni := c.GetOrCreateNetworkInstance(vrf)
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrf).Config(), ni)
	}
}

// configureDUT configures port1-8 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")
	p5 := dut.Port(t, "port5")
	p6 := dut.Port(t, "port6")
	p7 := dut.Port(t, "port7")
	p8 := dut.Port(t, "port8")
	portList := []*ondatra.Port{p1, p2, p3, p4, p5, p6, p7, p8}
	portNameList := []string{"port1", "port2", "port3", "port4", "port5", "port6", "port7", "port8"}

	for idx, a := range []attrs.Attributes{dutPort1, dutPort2, dutPort3, dutPort4, dutPort5, dutPort6, dutPort7, dutPort8} {
		p := portList[idx]
		intf := a.NewOCInterface(p.Name(), dut)
		if p.PMD() == ondatra.PMD100GBASEFR {
			e := intf.GetOrCreateEthernet()
			e.AutoNegotiate = ygot.Bool(false)
			e.DuplexMode = oc.Ethernet_DuplexMode_FULL
			e.PortSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB
		}
		gnmi.Replace(t, dut, d.Interface(p.Name()).Config(), intf)
	}

	// Configure loopback interface.
	loopbackIntfName = netutil.LoopbackInterface(t, dut, 0)
	lo0 := gnmi.OC().Interface(loopbackIntfName).Subinterface(0)
	ipv4Addrs := gnmi.LookupAll(t, dut, lo0.Ipv4().AddressAny().State())
	ipv6Addrs := gnmi.LookupAll(t, dut, lo0.Ipv6().AddressAny().State())
	if len(ipv4Addrs) == 0 && len(ipv6Addrs) == 0 {
		loop1 := dutlo0Attrs.NewOCInterface(loopbackIntfName, dut)
		loop1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
		gnmi.Update(t, dut, d.Interface(loopbackIntfName).Config(), loop1)
	} else {
		v4, ok := ipv4Addrs[0].Val()
		if ok {
			dutlo0Attrs.IPv4 = v4.GetIp()
		}
		v6, ok := ipv6Addrs[0].Val()
		if ok {
			dutlo0Attrs.IPv6 = v6.GetIp()
		}
		t.Logf("Got DUT IPv4 loopback address: %v", dutlo0Attrs.IPv4)
		t.Logf("Got DUT IPv6 loopback address: %v", dutlo0Attrs.IPv6)
	}

	for _, p := range portList {
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, p.Name(), deviations.DefaultNetworkInstance(dut), 0)
		}
	}
	for _, pName := range portNameList {
		if deviations.ExplicitPortSpeed(dut) {
			fptest.SetPortSpeed(t, dut.Port(t, pName))
		}
	}
	if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		staticARPWithMagicUniversalIP(t, dut)
	}
}

func configureISIS(t *testing.T, dut *ondatra.DUTDevice, intfList []string, dutAreaAddress, dutSysID string) {
	t.Helper()
	d := &oc.Root{}
	dutConfIsisPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	prot.Enabled = ygot.Bool(true)
	isis := prot.GetOrCreateIsis()
	globalISIS := isis.GetOrCreateGlobal()
	globalISIS.LevelCapability = oc.Isis_LevelType_LEVEL_2
	globalISIS.Net = []string{fmt.Sprintf("%v.%v.00", dutAreaAddress, dutSysID)}
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	if deviations.ISISInstanceEnabledRequired(dut) {
		globalISIS.Instance = ygot.String(isisInstance)
	}
	isisLevel2 := isis.GetOrCreateLevel(2)
	isisLevel2.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC
	if deviations.ISISLevelEnabled(dut) {
		isisLevel2.Enabled = ygot.Bool(true)
	}
	for _, intfName := range intfList {
		isisIntf := isis.GetOrCreateInterface(intfName)
		isisIntf.Enabled = ygot.Bool(true)
		isisIntf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
		isisIntfLevel := isisIntf.GetOrCreateLevel(2)
		isisIntfLevel.Enabled = ygot.Bool(true)
		isisIntfLevelAfi := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
		isisIntfLevelAfi.Metric = ygot.Uint32(200)
		isisIntfLevelAfi.Enabled = ygot.Bool(true)
		if deviations.ISISInterfaceAfiUnsupported(dut) {
			isisIntf.Af = nil
		}
		if deviations.MissingIsisInterfaceAfiSafiEnable(dut) {
			isisIntfLevelAfi.Enabled = nil
		}
	}
	gnmi.Replace(t, dut, dutConfIsisPath.Config(), prot)
}

func bgpCreateNbr(localAs uint32, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	dutOcRoot := &oc.Root{}
	ni1 := dutOcRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.RouterId = ygot.String(dutlo0Attrs.IPv4)
	global.As = ygot.Uint32(localAs)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

	pg1 := bgp.GetOrCreatePeerGroup(peerGrpName1)
	pg1.PeerAs = ygot.Uint32(localAs)

	bgpNbr := bgp.GetOrCreateNeighbor(otgIsisPort8LoopV4)
	bgpNbr.PeerGroup = ygot.String(peerGrpName1)
	bgpNbr.PeerAs = ygot.Uint32(localAs)
	bgpNbr.Enabled = ygot.Bool(true)
	bgpNbrT := bgpNbr.GetOrCreateTransport()
	bgpNbrT.LocalAddress = ygot.String(dutlo0Attrs.IPv4)
	af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	af4.Enabled = ygot.Bool(true)
	af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	af6.Enabled = ygot.Bool(true)

	return niProto
}

func verifyISISTelemetry(t *testing.T, dut *ondatra.DUTDevice, dutIntf string) {
	t.Helper()
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Isis()

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		dutIntf = dutIntf + ".0"
	}
	nbrPath := statePath.Interface(dutIntf)
	query := nbrPath.LevelAny().AdjacencyAny().AdjacencyState().State()
	_, ok := gnmi.WatchAll(t, dut, query, time.Minute, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
		state, present := val.Val()
		return present && state == oc.Isis_IsisInterfaceAdjState_UP
	}).Await(t)
	if !ok {
		t.Logf("IS-IS state on %v has no adjacencies", dutIntf)
		t.Fatal("No IS-IS adjacencies reported.")
	}
}

func createFlow(flowValues *flowArgs) gosnappi.Flow {
	flow := gosnappi.NewFlow().SetName(flowValues.flowName)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{"atePort1.IPv4"})
	flow.TxRx().Device().SetRxNames([]string{"atePort2.IPv4", "atePort3.IPv4", "atePort4.IPv4",
		"atePort5.IPv4", "atePort6.IPv4", "atePort7.IPv4", "atePort8.IPv4"})
	flow.Size().SetFixed(512)
	flow.Rate().SetPps(100)
	flow.Duration().Continuous()
	flow.Packet().Add().Ethernet().Src().SetValue(atePort1.MAC)
	// Outer IP header
	outerIpHdr := flow.Packet().Add().Ipv4()
	outerIpHdr.Src().SetValue(flowValues.outHdrSrcIP)
	outerIpHdr.Dst().SetValue(flowValues.outHdrDstIP)
	if len(flowValues.outHdrDscp) != 0 {
		outerIpHdr.Priority().Dscp().Phb().SetValues(flowValues.outHdrDscp)
	}
	if flowValues.udp {
		UDPHeader := flow.Packet().Add().Udp()
		UDPHeader.DstPort().Increment().SetStart(1).SetCount(50000).SetStep(1)
		UDPHeader.SrcPort().Increment().SetStart(1).SetCount(50000).SetStep(1)
	}

	if flowValues.proto != 0 {
		innerIpHdr := flow.Packet().Add().Ipv4()
		innerIpHdr.Protocol().SetValue(flowValues.proto)
		innerIpHdr.Src().SetValue(flowValues.InnHdrSrcIP)
		innerIpHdr.Dst().SetValue(flowValues.InnHdrDstIP)
	} else {
		if flowValues.isInnHdrV4 {
			innerIpHdr := flow.Packet().Add().Ipv4()
			innerIpHdr.Src().SetValue(flowValues.InnHdrSrcIP)
			innerIpHdr.Dst().SetValue(flowValues.InnHdrDstIP)
			// TODO : https://github.com/open-traffic-generator/fp-testbed-juniper/issues/42
			// Below code will be uncommented once ixia issue is fixed.
			// if len(flowValues.inHdrDscp) != 0 {
			// 	innerIpHdr.Priority().Dscp().Phb().SetValues(flowValues.inHdrDscp)
			// }
		} else {
			innerIpv6Hdr := flow.Packet().Add().Ipv6()
			innerIpv6Hdr.Src().SetValue(flowValues.InnHdrSrcIPv6)
			innerIpv6Hdr.Dst().SetValue(flowValues.InnHdrDstIPv6)
			// TODO : https://github.com/open-traffic-generator/fp-testbed-juniper/issues/42
			// Below code will be uncommented once ixia issue is fixed.
			// if len(flowValues.inHdrDscp) != 0 {
			// 	innerIpv6Hdr.FlowLabel().SetValues(flowValues.inHdrDscp)
			// }
		}
	}
	return flow
}

func verifyBgpTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	t.Logf("Verifying BGP state.")
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	nbrPath := bgpPath.Neighbor(otgIsisPort8LoopV4)
	// Get BGP adjacency state.
	t.Logf("Waiting for BGP neighbor to establish...")
	var status *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]
	status, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, ok := val.Val()
		return ok && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}).Await(t)
	if !ok {
		fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
		t.Fatal("No BGP neighbor formed")
	}
	state, _ := status.Val()
	t.Logf("BGP adjacency for %s: %v", otgIsisPort8LoopV4, state)
	if want := oc.Bgp_Neighbor_SessionState_ESTABLISHED; state != want {
		t.Errorf("BGP peer %s status got %d, want %d", otgIsisPort8LoopV4, state, want)
	}
}

func programAftWithMagicIp(t *testing.T, dut *ondatra.DUTDevice, args *testArgs) {
	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(11).WithMacAddress(magicMac).WithInterfaceRef(dut.Port(t, "port2").Name()).
			WithIPAddress(magicIp),
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(12).WithMacAddress(magicMac).WithInterfaceRef(dut.Port(t, "port3").Name()).
			WithIPAddress(magicIp),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(11).AddNextHop(11, 1).AddNextHop(12, 3),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF1+"/"+maskLen32).WithNextHopGroup(11),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(13).WithMacAddress(magicMac).WithInterfaceRef(dut.Port(t, "port4").Name()).
			WithIPAddress(magicIp),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(12).AddNextHop(13, 2),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF2+"/"+maskLen32).WithNextHopGroup(12),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(14).WithMacAddress(magicMac).WithInterfaceRef(dut.Port(t, "port5").Name()).
			WithIPAddress(magicIp),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(13).AddNextHop(14, 1),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF3+"/"+maskLen32).WithNextHopGroup(13),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(15).WithMacAddress(magicMac).WithInterfaceRef(dut.Port(t, "port6").Name()).
			WithIPAddress(magicIp),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(14).AddNextHop(15, 1),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF4+"/"+maskLen32).WithNextHopGroup(14),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(16).WithMacAddress(magicMac).WithInterfaceRef(dut.Port(t, "port7").Name()).
			WithIPAddress(magicIp),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(15).AddNextHop(16, 1),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF5+"/"+maskLen32).WithNextHopGroup(15),
	)
}

func configGribiBaselineAFT(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, args *testArgs) {
	t.Helper()

	if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		programAftWithMagicIp(t, dut, args)
	} else {
		// Programming AFT entries for prefixes in DEFAULT VRF
		args.client.Modify().AddEntry(t,
			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(11).WithMacAddress(magicMac).WithInterfaceRef(dut.Port(t, "port2").Name()),
			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(12).WithMacAddress(magicMac).WithInterfaceRef(dut.Port(t, "port3").Name()),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(11).AddNextHop(11, 1).AddNextHop(12, 3),
			fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithPrefix(gribiIPv4EntryDefVRF1+"/"+maskLen32).WithNextHopGroup(11),

			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(13).WithMacAddress(magicMac).WithInterfaceRef(dut.Port(t, "port4").Name()),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(12).AddNextHop(13, 2),
			fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithPrefix(gribiIPv4EntryDefVRF2+"/"+maskLen32).WithNextHopGroup(12),

			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(14).WithMacAddress(magicMac).WithInterfaceRef(dut.Port(t, "port5").Name()),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(13).AddNextHop(14, 1),
			fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithPrefix(gribiIPv4EntryDefVRF3+"/"+maskLen32).WithNextHopGroup(13),

			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(15).WithMacAddress(magicMac).WithInterfaceRef(dut.Port(t, "port6").Name()),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(14).AddNextHop(15, 1),
			fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithPrefix(gribiIPv4EntryDefVRF4+"/"+maskLen32).WithNextHopGroup(14),

			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(16).WithMacAddress(magicMac).WithInterfaceRef(dut.Port(t, "port7").Name()),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(15).AddNextHop(16, 1),
			fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithPrefix(gribiIPv4EntryDefVRF5+"/"+maskLen32).WithNextHopGroup(15),
		)
	}
	if err := awaitTimeout(args.ctx, t, args.client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	defaultVRFIPList := []string{gribiIPv4EntryDefVRF1, gribiIPv4EntryDefVRF2, gribiIPv4EntryDefVRF3, gribiIPv4EntryDefVRF4, gribiIPv4EntryDefVRF5}
	for ip := range defaultVRFIPList {
		chk.HasResult(t, args.client.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(defaultVRFIPList[ip]+"/32").
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}

	// Programming AFT entries for backup NHG
	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(1000).WithDecapsulateHeader(fluent.IPinIP).WithEncapsulateHeader(fluent.IPinIP).
			WithIPinIP(ipv4OuterSrc222, gribiIPv4EntryVRF2221).
			WithNextHopNetworkInstance(niTeVrf222),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(1000).AddNextHop(1000, 1),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(1001).WithDecapsulateHeader(fluent.IPinIP).
			WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(dut)),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(1001).AddNextHop(1001, 1),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(1002).WithDecapsulateHeader(fluent.IPinIP).WithEncapsulateHeader(fluent.IPinIP).
			WithIPinIP(ipv4OuterSrc222, gribiIPv4EntryVRF2222).
			WithNextHopNetworkInstance(niTeVrf222),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(1002).AddNextHop(1002, 1),
	)
	if err := awaitTimeout(args.ctx, t, args.client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	// Programming AFT entries for prefixes in TE_VRF_222
	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(3).WithIPAddress(gribiIPv4EntryDefVRF3),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(2).AddNextHop(3, 1).WithBackupNHG(1001),
		fluent.IPv4Entry().WithNetworkInstance(niTeVrf222).
			WithPrefix(gribiIPv4EntryVRF2221+"/"+maskLen32).WithNextHopGroup(2).
			WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(5).WithIPAddress(gribiIPv4EntryDefVRF5),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(4).AddNextHop(5, 1).WithBackupNHG(1001),
		fluent.IPv4Entry().WithNetworkInstance(niTeVrf222).
			WithPrefix(gribiIPv4EntryVRF2222+"/"+maskLen32).WithNextHopGroup(4).
			WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),
	)
	if err := awaitTimeout(args.ctx, t, args.client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	teVRF222IPList := []string{gribiIPv4EntryVRF2221, gribiIPv4EntryVRF2222}
	for ip := range teVRF222IPList {
		chk.HasResult(t, args.client.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(teVRF222IPList[ip]+"/32").
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}

	// Programming AFT entries for prefixes in TE_VRF_111
	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(1).WithIPAddress(gribiIPv4EntryDefVRF1),
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(2).WithIPAddress(gribiIPv4EntryDefVRF2),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(1).AddNextHop(1, 1).AddNextHop(2, 3).WithBackupNHG(1000),
		fluent.IPv4Entry().WithNetworkInstance(niTeVrf111).
			WithPrefix(gribiIPv4EntryVRF1111+"/"+maskLen32).WithNextHopGroup(1).
			WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(4).WithIPAddress(gribiIPv4EntryDefVRF4),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(3).AddNextHop(4, 1).WithBackupNHG(1002),
		fluent.IPv4Entry().WithNetworkInstance(niTeVrf111).
			WithPrefix(gribiIPv4EntryVRF1112+"/"+maskLen32).WithNextHopGroup(3).
			WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),
	)
	if err := awaitTimeout(args.ctx, t, args.client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	teVRF111IPList := []string{gribiIPv4EntryVRF1111, gribiIPv4EntryVRF1112}
	for ip := range teVRF111IPList {
		chk.HasResult(t, args.client.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(teVRF111IPList[ip]+"/32").
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}

	// Programming AFT entries for prefixes in ENCAP_TE_VRF_A
	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(200).WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(dut)),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(200).AddNextHop(200, 1),
	)
	if err := awaitTimeout(args.ctx, t, args.client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(101).WithEncapsulateHeader(fluent.IPinIP).
			WithIPinIP(ipv4OuterSrc111, gribiIPv4EntryVRF1111).
			WithNextHopNetworkInstance(niTeVrf111),
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(102).WithEncapsulateHeader(fluent.IPinIP).
			WithIPinIP(ipv4OuterSrc111, gribiIPv4EntryVRF1112).
			WithNextHopNetworkInstance(niTeVrf111),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(101).AddNextHop(101, 1).AddNextHop(102, 3).WithBackupNHG(200),
		fluent.IPv4Entry().WithNetworkInstance(niEncapTeVrfA).
			WithPrefix(gribiIPv4EntryEncapVRF+"/"+maskLen24).WithNextHopGroup(101).
			WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),
		fluent.IPv6Entry().WithNetworkInstance(niEncapTeVrfA).
			WithPrefix(gribiIPv6EntryEncapVRF+"/"+maskLen126).WithNextHopGroup(101).
			WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),
	)

	if err := awaitTimeout(args.ctx, t, args.client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	chk.HasResult(t, args.client.Results(t),
		fluent.OperationResult().
			WithIPv4Operation(gribiIPv4EntryEncapVRF+"/24").
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	chk.HasResult(t, args.client.Results(t),
		fluent.OperationResult().
			WithIPv6Operation(gribiIPv6EntryEncapVRF+"/126").
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	args.client.Modify().AddEntry(t,
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(102).AddNextHop(101, 3).AddNextHop(102, 1).WithBackupNHG(200),
		fluent.IPv4Entry().WithNetworkInstance(niEncapTeVrfB).
			WithPrefix(gribiIPv4EntryEncapVRF+"/"+maskLen24).WithNextHopGroup(102).
			WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),
		fluent.IPv6Entry().WithNetworkInstance(niEncapTeVrfB).
			WithPrefix(gribiIPv6EntryEncapVRF+"/"+maskLen126).WithNextHopGroup(102).
			WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),
	)

	if err := awaitTimeout(args.ctx, t, args.client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	chk.HasResult(t, args.client.Results(t),
		fluent.OperationResult().
			WithIPv4Operation(gribiIPv4EntryEncapVRF+"/24").
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	chk.HasResult(t, args.client.Results(t),
		fluent.OperationResult().
			WithIPv6Operation(gribiIPv6EntryEncapVRF+"/126").
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	// Install an 0/0 static route in ENCAP_VRF_A and ENCAP_VRF_B pointing to the DEFAULT VRF.
	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(60).WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(dut)),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(60).AddNextHop(60, 1),
		fluent.IPv4Entry().WithNetworkInstance(niEncapTeVrfA).
			WithPrefix("0.0.0.0/0").WithNextHopGroup(60).
			WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),
	)
	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(65).WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(dut)),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(65).AddNextHop(65, 1),
		fluent.IPv6Entry().WithNetworkInstance(niEncapTeVrfA).
			WithPrefix("::/0").WithNextHopGroup(65).
			WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),
	)
	if err := awaitTimeout(args.ctx, t, args.client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}
	chk.HasResult(t, args.client.Results(t),
		fluent.OperationResult().WithIPv4Operation("0.0.0.0/0").WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).AsResult(),
		chk.IgnoreOperationID(),
	)
	chk.HasResult(t, args.client.Results(t),
		fluent.OperationResult().WithIPv6Operation("::/0").WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).AsResult(),
		chk.IgnoreOperationID(),
	)

	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(61).WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(dut)),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(61).AddNextHop(61, 1),
		fluent.IPv4Entry().WithNetworkInstance(niEncapTeVrfB).
			WithPrefix("0.0.0.0/0").WithNextHopGroup(61).
			WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),
	)
	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(66).WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(dut)),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(66).AddNextHop(66, 1),
		fluent.IPv6Entry().WithNetworkInstance(niEncapTeVrfB).
			WithPrefix("::/0").WithNextHopGroup(66).
			WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),
	)

	if err := awaitTimeout(args.ctx, t, args.client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}
	chk.HasResult(t, args.client.Results(t),
		fluent.OperationResult().WithIPv4Operation("0.0.0.0/0").WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).AsResult(),
		chk.IgnoreOperationID(),
	)
	chk.HasResult(t, args.client.Results(t),
		fluent.OperationResult().WithIPv6Operation("::/0").WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).AsResult(),
		chk.IgnoreOperationID(),
	)
}

func configureGribiRoute(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, args *testArgs, prefWithMask string) {
	t.Helper()
	// Using gRIBI, install an  IPv4Entry for the prefix 192.51.100.1/24 that points to a
	// NextHopGroup that contains a single NextHop that specifies decapsulating the IPv4
	// header and specifies the DEFAULT network instance.This IPv4Entry should be installed
	// into the DECAP_TE_VRF.

	args.client.Modify().AddEntry(t,
		fluent.IPv4Entry().WithNetworkInstance(niDecapTeVrf).
			WithPrefix(prefWithMask).WithNextHopGroup(1001).
			WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),
	)
	if err := awaitTimeout(args.ctx, t, args.client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	chk.HasResult(t, args.client.Results(t),
		fluent.OperationResult().WithIPv4Operation(prefWithMask).WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).AsResult(),
		chk.IgnoreOperationID(),
	)
}

func configureGribiMixedPrefEntries(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, args *testArgs, prefList []string) {
	t.Helper()

	for _, pref := range prefList {
		args.client.Modify().AddEntry(t,
			fluent.IPv4Entry().WithNetworkInstance(niDecapTeVrf).
				WithPrefix(pref).WithNextHopGroup(1001).
				WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),
		)
	}

	if err := awaitTimeout(args.ctx, t, args.client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	for _, pref := range prefList {
		chk.HasResult(t, args.client.Results(t),
			fluent.OperationResult().WithIPv4Operation(pref).WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).AsResult(),
			chk.IgnoreOperationID(),
		)
	}
}

func configureOTG(t *testing.T, otg *otg.OTG, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	config := gosnappi.NewConfig()

	port1 := config.Ports().Add().SetName("port1")
	port2 := config.Ports().Add().SetName("port2")
	port3 := config.Ports().Add().SetName("port3")
	port4 := config.Ports().Add().SetName("port4")
	port5 := config.Ports().Add().SetName("port5")
	port6 := config.Ports().Add().SetName("port6")
	port7 := config.Ports().Add().SetName("port7")
	port8 := config.Ports().Add().SetName("port8")

	pmd100GFRPorts := []string{}
	for _, p := range config.Ports().Items() {
		port := ate.Port(t, p.Name())
		if port.PMD() == ondatra.PMD100GBASEFR {
			pmd100GFRPorts = append(pmd100GFRPorts, port.ID())
		}
	}
	// Disable FEC for 100G-FR ports because Novus does not support it.
	if len(pmd100GFRPorts) > 0 {
		l1Settings := config.Layer1().Add().SetName("L1").SetPortNames(pmd100GFRPorts)
		l1Settings.SetAutoNegotiate(true).SetIeeeMediaDefaults(false).SetSpeed("speed_100_gbps")
		autoNegotiate := l1Settings.AutoNegotiation()
		autoNegotiate.SetRsFec(false)
	}

	iDut1Dev := config.Devices().Add().SetName(atePort1.Name)
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	iDut1Eth.Connection().SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4")
	iDut1Ipv4.SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))
	iDut1Ipv6 := iDut1Eth.Ipv6Addresses().Add().SetName(atePort1.Name + ".IPv6")
	iDut1Ipv6.SetAddress(atePort1.IPv6).SetGateway(dutPort1.IPv6).SetPrefix(uint32(atePort1.IPv6Len))

	iDut2Dev := config.Devices().Add().SetName(atePort2.Name)
	iDut2Eth := iDut2Dev.Ethernets().Add().SetName(atePort2.Name + ".Eth").SetMac(atePort2.MAC)
	iDut2Eth.Connection().SetPortName(port2.Name())
	iDut2Ipv4 := iDut2Eth.Ipv4Addresses().Add().SetName(atePort2.Name + ".IPv4")
	iDut2Ipv4.SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).SetPrefix(uint32(atePort2.IPv4Len))
	iDut2Ipv6 := iDut2Eth.Ipv6Addresses().Add().SetName(atePort2.Name + ".IPv6")
	iDut2Ipv6.SetAddress(atePort2.IPv6).SetGateway(dutPort2.IPv6).SetPrefix(uint32(atePort2.IPv6Len))

	iDut3Dev := config.Devices().Add().SetName(atePort3.Name)
	iDut3Eth := iDut3Dev.Ethernets().Add().SetName(atePort3.Name + ".Eth").SetMac(atePort3.MAC)
	iDut3Eth.Connection().SetPortName(port3.Name())
	iDut3Ipv4 := iDut3Eth.Ipv4Addresses().Add().SetName(atePort3.Name + ".IPv4")
	iDut3Ipv4.SetAddress(atePort3.IPv4).SetGateway(dutPort3.IPv4).SetPrefix(uint32(atePort3.IPv4Len))
	iDut3Ipv6 := iDut3Eth.Ipv6Addresses().Add().SetName(atePort3.Name + ".IPv6")
	iDut3Ipv6.SetAddress(atePort3.IPv6).SetGateway(dutPort3.IPv6).SetPrefix(uint32(atePort3.IPv6Len))

	iDut4Dev := config.Devices().Add().SetName(atePort4.Name)
	iDut4Eth := iDut4Dev.Ethernets().Add().SetName(atePort4.Name + ".Eth").SetMac(atePort4.MAC)
	iDut4Eth.Connection().SetPortName(port4.Name())
	iDut4Ipv4 := iDut4Eth.Ipv4Addresses().Add().SetName(atePort4.Name + ".IPv4")
	iDut4Ipv4.SetAddress(atePort4.IPv4).SetGateway(dutPort4.IPv4).SetPrefix(uint32(atePort4.IPv4Len))
	iDut4Ipv6 := iDut4Eth.Ipv6Addresses().Add().SetName(atePort4.Name + ".IPv6")
	iDut4Ipv6.SetAddress(atePort4.IPv6).SetGateway(dutPort4.IPv6).SetPrefix(uint32(atePort4.IPv6Len))

	iDut5Dev := config.Devices().Add().SetName(atePort5.Name)
	iDut5Eth := iDut5Dev.Ethernets().Add().SetName(atePort5.Name + ".Eth").SetMac(atePort5.MAC)
	iDut5Eth.Connection().SetPortName(port5.Name())
	iDut5Ipv4 := iDut5Eth.Ipv4Addresses().Add().SetName(atePort5.Name + ".IPv4")
	iDut5Ipv4.SetAddress(atePort5.IPv4).SetGateway(dutPort5.IPv4).SetPrefix(uint32(atePort5.IPv4Len))
	iDut5Ipv6 := iDut5Eth.Ipv6Addresses().Add().SetName(atePort5.Name + ".IPv6")
	iDut5Ipv6.SetAddress(atePort5.IPv6).SetGateway(dutPort5.IPv6).SetPrefix(uint32(atePort5.IPv6Len))

	iDut6Dev := config.Devices().Add().SetName(atePort6.Name)
	iDut6Eth := iDut6Dev.Ethernets().Add().SetName(atePort6.Name + ".Eth").SetMac(atePort6.MAC)
	iDut6Eth.Connection().SetPortName(port6.Name())
	iDut6Ipv4 := iDut6Eth.Ipv4Addresses().Add().SetName(atePort6.Name + ".IPv4")
	iDut6Ipv4.SetAddress(atePort6.IPv4).SetGateway(dutPort6.IPv4).SetPrefix(uint32(atePort6.IPv4Len))
	iDut6Ipv6 := iDut6Eth.Ipv6Addresses().Add().SetName(atePort6.Name + ".IPv6")
	iDut6Ipv6.SetAddress(atePort6.IPv6).SetGateway(dutPort6.IPv6).SetPrefix(uint32(atePort6.IPv6Len))

	iDut7Dev := config.Devices().Add().SetName(atePort7.Name)
	iDut7Eth := iDut7Dev.Ethernets().Add().SetName(atePort7.Name + ".Eth").SetMac(atePort7.MAC)
	iDut7Eth.Connection().SetPortName(port7.Name())
	iDut7Ipv4 := iDut7Eth.Ipv4Addresses().Add().SetName(atePort7.Name + ".IPv4")
	iDut7Ipv4.SetAddress(atePort7.IPv4).SetGateway(dutPort7.IPv4).SetPrefix(uint32(atePort7.IPv4Len))
	iDut7Ipv6 := iDut7Eth.Ipv6Addresses().Add().SetName(atePort7.Name + ".IPv6")
	iDut7Ipv6.SetAddress(atePort7.IPv6).SetGateway(dutPort7.IPv6).SetPrefix(uint32(atePort7.IPv6Len))

	iDut8Dev := config.Devices().Add().SetName(atePort8.Name)
	iDut8Eth := iDut8Dev.Ethernets().Add().SetName(atePort8.Name + ".Eth").SetMac(atePort8.MAC)
	iDut8Eth.Connection().SetPortName(port8.Name())
	iDut8Ipv4 := iDut8Eth.Ipv4Addresses().Add().SetName(atePort8.Name + ".IPv4")
	iDut8Ipv4.SetAddress(atePort8.IPv4).SetGateway(dutPort8.IPv4).SetPrefix(uint32(atePort8.IPv4Len))
	iDut8Ipv6 := iDut8Eth.Ipv6Addresses().Add().SetName(atePort8.Name + ".IPv6")
	iDut8Ipv6.SetAddress(atePort8.IPv6).SetGateway(dutPort8.IPv6).SetPrefix(uint32(atePort8.IPv6Len))
	// Configure Loopback on port8.
	iDut8LoopV4 := iDut8Dev.Ipv4Loopbacks().Add().SetName("Port8LoopV4").SetEthName(iDut8Eth.Name())
	iDut8LoopV4.SetAddress(otgIsisPort8LoopV4)
	iDut8LoopV6 := iDut8Dev.Ipv6Loopbacks().Add().SetName("Port8LoopV6").SetEthName(iDut8Eth.Name())
	iDut8LoopV6.SetAddress(otgIsisPort8LoopV6)

	// Enable ISIS and BGP Protocols on port 8.
	isisDut := iDut8Dev.Isis().SetName("ISIS1").SetSystemId(otgSysID1)
	isisDut.Basic().SetIpv4TeRouterId(atePort8.IPv4).SetHostname(isisDut.Name()).SetLearnedLspFilter(true)
	isisDut.Interfaces().Add().SetEthName(iDut8Dev.Ethernets().Items()[0].Name()).
		SetName("devIsisInt1").
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT)

	// Advertise OTG Port8 loopback address via ISIS.
	isisPort2V4 := iDut8Dev.Isis().V4Routes().Add().SetName("ISISPort8V4").SetLinkMetric(10)
	isisPort2V4.Addresses().Add().SetAddress(otgIsisPort8LoopV4).SetPrefix(32)
	isisPort2V6 := iDut8Dev.Isis().V6Routes().Add().SetName("ISISPort8V6").SetLinkMetric(10)
	isisPort2V6.Addresses().Add().SetAddress(otgIsisPort8LoopV6).SetPrefix(uint32(128))

	iDutBgp := iDut8Dev.Bgp().SetRouterId(otgIsisPort8LoopV4)
	iDutBgp4Peer := iDutBgp.Ipv4Interfaces().Add().SetIpv4Name(iDut8LoopV4.Name()).Peers().Add().SetName(atePort8.Name + ".BGP4.peer")
	iDutBgp4Peer.SetPeerAddress(dutlo0Attrs.IPv4).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	iDutBgp4Peer.Capability().SetIpv4Unicast(true).SetIpv6Unicast(true)
	iDutBgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	bgp4Peer = iDutBgp4Peer

	bgpNeti1Bgp4PeerRoutes := iDutBgp4Peer.V4Routes().Add().SetName(atePort8.Name + ".BGP4.Route")
	bgpNeti1Bgp4PeerRoutes.SetNextHopIpv4Address(otgIsisPort8LoopV4).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL).
		Advanced().SetLocalPreference(100).SetIncludeLocalPreference(true)
	bgpNeti1Bgp4PeerRoutes.Addresses().Add().SetAddress(ipv4InnerDst).SetPrefix(32).
		SetCount(1).SetStep(1)
	bgpNeti1Bgp4PeerRoutes.Addresses().Add().SetAddress(ipv4InnerDstNoEncap).SetPrefix(32).
		SetCount(1).SetStep(1)

	bgpNeti1Bgp6PeerRoutes := iDutBgp4Peer.V6Routes().Add().SetName(atePort8.Name + ".BGP6.Route")
	bgpNeti1Bgp6PeerRoutes.SetNextHopIpv6Address(otgIsisPort8LoopV6).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL).
		Advanced().SetLocalPreference(100).SetIncludeLocalPreference(true)
	bgpNeti1Bgp6PeerRoutes.Addresses().Add().SetAddress(ipv6InnerDst).SetPrefix(128).
		SetCount(1).SetStep(1)
	bgpNeti1Bgp6PeerRoutes.Addresses().Add().SetAddress(ipv6InnerDstNoEncap).SetPrefix(128).
		SetCount(1).SetStep(1)

	t.Logf("Pushing config to ATE and starting protocols...")
	otg.PushConfig(t, config)
	time.Sleep(30 * time.Second)
	otg.StartProtocols(t)
	time.Sleep(30 * time.Second)
	return config
}

func updateBgpRoutes(t *testing.T, args *testArgs, deleteRoute bool) {
	t.Helper()
	config := args.otgConfig
	if deleteRoute {
		bgp4Peer.V4Routes().Clear()
	} else {
		bgpNeti1Bgp4PeerRoutes := bgp4Peer.V4Routes().Add().SetName(atePort8.Name + ".BGP4.Route")
		bgpNeti1Bgp4PeerRoutes.SetNextHopIpv4Address(otgIsisPort8LoopV4).
			SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
			SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL).
			Advanced().SetLocalPreference(100).SetIncludeLocalPreference(true)
		bgpNeti1Bgp4PeerRoutes.Addresses().Add().SetAddress(ipv4InnerDst).SetPrefix(32).
			SetCount(1).SetStep(1)
		bgpNeti1Bgp4PeerRoutes.Addresses().Add().SetAddress(ipv4InnerDstNoEncap).SetPrefix(32).
			SetCount(1).SetStep(1)
	}
	args.otg.PushConfig(t, config)
	time.Sleep(30 * time.Second)
	args.otg.StartProtocols(t)
	time.Sleep(30 * time.Second)
}

func sendTraffic(t *testing.T, args *testArgs, capturePortList []string, flowList []gosnappi.Flow) {
	t.Helper()

	args.otgConfig.Flows().Clear()
	for _, flow := range flowList {
		args.otgConfig.Flows().Append(flow)
	}

	args.otgConfig.Captures().Clear()
	args.otgConfig.Captures().Add().SetName("packetCapture").
		SetPortNames(capturePortList).
		SetFormat(gosnappi.CaptureFormat.PCAP)

	args.otg.PushConfig(t, args.otgConfig)
	time.Sleep(30 * time.Second)
	args.otg.StartProtocols(t)
	time.Sleep(30 * time.Second)

	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	args.otg.SetControlState(t, cs)

	t.Logf("Starting traffic")
	args.otg.StartTraffic(t)
	time.Sleep(15 * time.Second)
	t.Logf("Stop traffic")
	args.otg.StopTraffic(t)

	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	args.otg.SetControlState(t, cs)
}

func verifyTraffic(t *testing.T, args *testArgs, flowList []string, wantLoss bool) {
	t.Helper()
	for _, flowName := range flowList {
		t.Logf("Verifying flow metrics for the flow %s\n", flowName)
		recvMetric := gnmi.Get(t, args.otg, gnmi.OTG().Flow(flowName).State())
		txPackets := recvMetric.GetCounters().GetOutPkts()
		rxPackets := recvMetric.GetCounters().GetInPkts()

		lostPackets := txPackets - rxPackets
		var lossPct uint64
		if txPackets != 0 {
			lossPct = lostPackets * 100 / txPackets
		} else {
			t.Errorf("Traffic stats are not correct %v", recvMetric)
		}
		if wantLoss {
			if lossPct < 100-tolerancePct {
				t.Errorf("Traffic is expected to fail %s\n got %v, want 100%% failure", flowName, lossPct)
			} else {
				t.Logf("Traffic Loss Test Passed!")
			}
		} else {
			if lossPct > tolerancePct {
				t.Errorf("Traffic Loss Pct for Flow: %s\n got %v, want 0", flowName, lossPct)
			} else {
				t.Logf("Traffic Test Passed!")
			}
		}
	}
}

type packetValidation struct {
	portName        string
	outDstIP        []string
	inHdrIP         string
	validateDecap   bool
	validateTTL     bool
	validateNoDecap bool
	validateEncap   bool
}

func captureAndValidatePackets(t *testing.T, args *testArgs, packetVal *packetValidation) {
	bytes := args.otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(packetVal.portName))
	f, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Fatalf("ERROR: Could not create temporary pcap file: %v\n", err)
	}
	if _, err := f.Write(bytes); err != nil {
		t.Fatalf("ERROR: Could not write bytes to pcap file: %v\n", err)
	}
	f.Close()
	handle, err := pcap.OpenOffline(f.Name())
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	if packetVal.validateTTL {
		validateTrafficTTL(t, packetSource)
	}
	if packetVal.validateDecap {
		validateTrafficDecap(t, packetSource)
	}
	if packetVal.validateNoDecap {
		validateTrafficNonDecap(t, packetSource, packetVal.outDstIP[0], packetVal.inHdrIP)
	}
	if packetVal.validateEncap {
		validateTrafficEncap(t, packetSource, packetVal.outDstIP, packetVal.inHdrIP)
	}
	args.otgConfig.Captures().Clear()
	args.otg.PushConfig(t, args.otgConfig)
	time.Sleep(30 * time.Second)
}

func validateTrafficTTL(t *testing.T, packetSource *gopacket.PacketSource) {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	var packetCheckCount uint32 = 0
	for packet := range packetSource.Packets() {
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		if ipLayer != nil && packetCheckCount <= 3 {
			packetCheckCount++
			ipPacket, _ := ipLayer.(*layers.IPv4)
			if !deviations.TTLCopyUnsupported(dut) {
				if ipPacket.TTL != correspondingTTL {
					t.Errorf("IP TTL value is altered to: %d", ipPacket.TTL)
				}
			}
			innerPacket := gopacket.NewPacket(ipPacket.Payload, ipPacket.NextLayerType(), gopacket.Default)
			ipInnerLayer := innerPacket.Layer(layers.LayerTypeIPv4)
			ipv6InnerLayer := innerPacket.Layer(layers.LayerTypeIPv6)
			if ipInnerLayer != nil {
				t.Errorf("Packets are not decapped, Inner IP header is not removed.")
			}
			if ipv6InnerLayer != nil {
				t.Errorf("Packets are not decapped, Inner IPv6 header is not removed.")
			}
		}
	}
}

func validateTrafficDecap(t *testing.T, packetSource *gopacket.PacketSource) {
	t.Helper()
	for packet := range packetSource.Packets() {
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		if ipLayer == nil {
			continue
		}
		ipPacket, _ := ipLayer.(*layers.IPv4)
		innerPacket := gopacket.NewPacket(ipPacket.Payload, ipPacket.NextLayerType(), gopacket.Default)
		ipInnerLayer := innerPacket.Layer(layers.LayerTypeIPv4)
		ipv6InnerLayer := innerPacket.Layer(layers.LayerTypeIPv6)
		if ipInnerLayer != nil {
			t.Errorf("Packets are not decapped, Inner IP header is not removed.")
		}
		if ipv6InnerLayer != nil {
			t.Errorf("Packets are not decapped, Inner IPv6 header is not removed.")
		}
	}
}

func validateTrafficNonDecap(t *testing.T, packetSource *gopacket.PacketSource, outDstIP, inHdrIP string) {
	t.Helper()
	t.Log("Validate traffic non decap routes")
	var packetCheckCount uint32 = 1
	for packet := range packetSource.Packets() {
		if packetCheckCount >= 5 {
			break
		}
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		if ipLayer == nil {
			continue
		}
		ipPacket, _ := ipLayer.(*layers.IPv4)
		innerPacket := gopacket.NewPacket(ipPacket.Payload, ipPacket.NextLayerType(), gopacket.Default)
		ipInnerLayer := innerPacket.Layer(layers.LayerTypeIPv4)
		if ipInnerLayer != nil {
			if ipPacket.DstIP.String() != outDstIP {
				t.Errorf("Negatice test for Decap failed. Traffic sent to route which does not match the decap route are decaped")
			}
			ipInnerPacket, _ := ipInnerLayer.(*layers.IPv4)
			if ipInnerPacket.DstIP.String() != inHdrIP {
				t.Errorf("Negatice test for Decap failed. Traffic sent to route which does not match the decap route are decaped")
			}
			t.Logf("Traffic for non decap routes passed.")
			break
		}
	}
}

func validateTrafficEncap(t *testing.T, packetSource *gopacket.PacketSource, outDstIP []string, innerIP string) {
	t.Helper()
	t.Log("Validate traffic non decap routes")
	var packetCheckCount uint32 = 1
	for packet := range packetSource.Packets() {
		if packetCheckCount >= 5 {
			break
		}
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		if ipLayer == nil {
			continue
		}
		ipPacket, _ := ipLayer.(*layers.IPv4)
		innerPacket := gopacket.NewPacket(ipPacket.Payload, ipPacket.NextLayerType(), gopacket.Default)
		ipInnerLayer := innerPacket.Layer(layers.LayerTypeIPv4)
		if ipInnerLayer != nil {
			if len(outDstIP) == 2 {
				if ipPacket.DstIP.String() != outDstIP[0] || ipPacket.DstIP.String() != outDstIP[1] {
					t.Errorf("Packets are not encapsulated as expected")
				}
			} else {
				if ipPacket.DstIP.String() != outDstIP[0] {
					t.Errorf("Packets are not encapsulated as expected")
				}
			}
			t.Logf("Traffic for encap routes passed.")
			break
		}
	}
}

// TODO : https://github.com/open-traffic-generator/fp-testbed-juniper/issues/42
// Below code will be uncommented once ixia issue is fixed.

// normalize normalizes the input values so that the output values sum
// to 1.0 but reflect the proportions of the input.  For example,
// input [1, 2, 3, 4] is normalized to [0.1, 0.2, 0.3, 0.4].
/* func normalize(xs []uint64) (ys []float64, sum uint64) {
	for _, x := range xs {
		sum += x
	}
	ys = make([]float64, len(xs))
	for i, x := range xs {
		ys[i] = float64(x) / float64(sum)
	}
	return ys, sum
} */

// TODO : https://github.com/open-traffic-generator/fp-testbed-juniper/issues/42
// Below code will be uncommented once ixia issue is fixed.

/* func validateTrafficDistribution(t *testing.T, ate *ondatra.ATEDevice, wantWeights []float64) {
	inFramesAllPorts := gnmi.GetAll(t, ate.OTG(), gnmi.OTG().PortAny().Counters().InFrames().State())
	// Skip source port, Port1.
	gotWeights, _ := normalize(inFramesAllPorts[1:])

	t.Log("got ratio:", gotWeights)
	t.Log("want ratio:", wantWeights)
	if diff := cmp.Diff(wantWeights, gotWeights, cmpopts.EquateApprox(0, tolerance)); diff != "" {
		t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
	}
} */

type flowArgs struct {
	flowName                     string
	outHdrSrcIP, outHdrDstIP     string
	InnHdrSrcIP, InnHdrDstIP     string
	InnHdrSrcIPv6, InnHdrDstIPv6 string
	udp, isInnHdrV4              bool
	outHdrDscp                   []uint32
	// TODO : https://github.com/open-traffic-generator/fp-testbed-juniper/issues/42
	// Below code will be uncommented once ixia issue is fixed.
	// inHdrDscp []uint32
	proto uint32
}

// testGribiDecapMatchSrcProtoNoMatchDSCP is to validate subtest test1.
// Test-1 match on source and protocol no match on DSCP; flow VRF_DECAP hit -> DEFAULT
func testGribiDecapMatchSrcProtoNoMatchDSCP(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, args *testArgs) {
	t.Helper()
	cases := []struct {
		desc           string
		prefixWithMask string
	}{{
		desc:           "Mask Length 24",
		prefixWithMask: "192.51.100.0/24",
	}, {
		desc:           "Mask Length 32",
		prefixWithMask: "192.51.100.64/32",
	}, {
		desc:           "Mask Length 28",
		prefixWithMask: "192.51.100.64/28",
	}, {
		desc:           "Mask Length 22",
		prefixWithMask: "192.51.100.0/22",
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Log("Flush existing gRIBI routes before test.")
			if err := gribi.FlushAll(args.client); err != nil {
				t.Fatal(err)
			}

			// Configure GRIBi baseline AFTs.
			configGribiBaselineAFT(ctx, t, dut, args)

			t.Run("Program gRIBi route", func(t *testing.T) {
				configureGribiRoute(ctx, t, dut, args, tc.prefixWithMask)
			})
			// Send both 6in4 and 4in4 packets. Verify that the packets have their outer
			// v4 header stripped and are forwarded according to the route in the DEFAULT
			// VRF that matches the inner IP address.
			portList := []string{"port8"}
			//dstPorts := []attrs.Attributes{atePort2, atePort3, atePort4, atePort5, atePort6, atePort7, atePort8}
			t.Run("Create ip-in-ip and ipv6-in-ip flows, send traffic and verify decap functionality",
				func(t *testing.T) {

					flow1 := createFlow(&flowArgs{flowName: flow4in4,
						outHdrSrcIP: ipv4OuterSrc111, outHdrDstIP: ipv4OuterDst111, outHdrDscp: []uint32{dscpEncapNoMatch},
						InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: ipv4InnerDst, isInnHdrV4: true})

					flow2 := createFlow(&flowArgs{flowName: flow6in4,
						outHdrSrcIP: ipv4OuterSrc111, outHdrDstIP: ipv4OuterDst111, InnHdrSrcIPv6: atePort1.IPv6,
						InnHdrDstIPv6: ipv6InnerDst, isInnHdrV4: false, outHdrDscp: []uint32{dscpEncapNoMatch}})

					sendTraffic(t, args, portList, []gosnappi.Flow{flow1, flow2})
					verifyTraffic(t, args, []string{flow4in4, flow6in4}, !wantLoss)
					captureAndValidatePackets(t, args, &packetValidation{portName: portList[0],
						outDstIP: []string{ipv4OuterDst111}, inHdrIP: ipv4InnerDst, validateTTL: true, validateDecap: true})
				})

			// Test with packets with a destination address that does not match
			// the decap route, and verify that such packets are not decapped.
			portList = []string{"port4"}
			t.Run("Send traffic to non decap route and verify the behavior",
				func(t *testing.T) {
					flow3 := createFlow(&flowArgs{flowName: flowNegTest,
						outHdrSrcIP: ipv4OuterSrc111, outHdrDstIP: gribiIPv4EntryVRF1111, isInnHdrV4: true,
						InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: ipv4InnerDst})
					sendTraffic(t, args, portList, []gosnappi.Flow{flow3})
					verifyTraffic(t, args, []string{flowNegTest}, !wantLoss)
					captureAndValidatePackets(t, args, &packetValidation{portName: portList[0],
						outDstIP: []string{gribiIPv4EntryVRF1111}, inHdrIP: ipv4InnerDst, validateNoDecap: true})
				})
		})
	}
}

// testGribiDecapMatchSrcProtoDSCP is to validate subtest 2.
// Test-2, match on source, protocol and DSCP, VRF_DECAP hit -> VRF_ENCAP_A miss -> DEFAULT
func testGribiDecapMatchSrcProtoDSCP(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, args *testArgs) {
	t.Helper()
	cases := []struct {
		desc           string
		prefixWithMask string
	}{{
		desc:           "Mask Length 24",
		prefixWithMask: "192.51.100.0/24",
	}, {
		desc:           "Mask Length 32",
		prefixWithMask: "192.51.100.64/32",
	}, {
		desc:           "Mask Length 28",
		prefixWithMask: "192.51.100.64/28",
	}, {
		desc:           "Mask Length 22",
		prefixWithMask: "192.51.100.0/22",
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Log("Flush existing gRIBI routes before test.")
			if err := gribi.FlushAll(args.client); err != nil {
				t.Fatal(err)
			}

			// Configure GRIBi baseline AFTs.
			configGribiBaselineAFT(ctx, t, dut, args)

			t.Run("Program gRIBi route", func(t *testing.T) {
				configureGribiRoute(ctx, t, dut, args, tc.prefixWithMask)
			})
			// Send both 6in4 and 4in4 packets. Verify that the packets have their outer
			// v4 header stripped and are forwarded according to the route in the DEFAULT
			// VRF that matches the inner IP address.
			portList := []string{"port8"}
			//dstPorts := []attrs.Attributes{atePort2, atePort3, atePort4, atePort5, atePort6, atePort7, atePort8}
			t.Run("Create ip-in-ip and ipv6-in-ip flows, send traffic and verify decap functionality",
				func(t *testing.T) {

					flow1 := createFlow(&flowArgs{flowName: flow4in4,
						outHdrSrcIP: ipv4OuterSrc111, outHdrDstIP: ipv4OuterDst111, outHdrDscp: []uint32{dscpEncapA1},
						InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: ipv4InnerDstNoEncap, isInnHdrV4: true})

					flow2 := createFlow(&flowArgs{flowName: flow6in4,
						outHdrSrcIP: ipv4OuterSrc111, outHdrDstIP: ipv4OuterDst111, InnHdrSrcIPv6: atePort1.IPv6,
						InnHdrDstIPv6: ipv6InnerDstNoEncap, isInnHdrV4: false, outHdrDscp: []uint32{dscpEncapA1}})

					sendTraffic(t, args, portList, []gosnappi.Flow{flow1, flow2})
					verifyTraffic(t, args, []string{flow4in4, flow6in4}, !wantLoss)
					captureAndValidatePackets(t, args, &packetValidation{portName: portList[0],
						outDstIP: []string{ipv4OuterDst111}, inHdrIP: ipv4InnerDstNoEncap, validateTTL: true, validateDecap: true})
				})
		})
	}
}

// testGribiDecapMixedLenPref is to validate subtest 3.
// Test-3, Mixed Prefix Decap gRIBI Entries.
func testGribiDecapMixedLenPref(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, args *testArgs) {
	t.Helper()
	var testPref1 string = "192.51.128.0/22"
	var testPref2 string = "192.55.200.3/32"

	var traffiDstIP1 string = "192.55.200.3"
	var traffiDstIP2 string = "192.51.128.5"

	t.Log("Flush existing gRIBI routes before test.")
	if err := gribi.FlushAll(args.client); err != nil {
		t.Fatal(err)
	}

	// Configure GRIBi baseline AFTs.
	configGribiBaselineAFT(ctx, t, dut, args)

	t.Run("Program gRIBi route", func(t *testing.T) {
		configureGribiMixedPrefEntries(ctx, t, dut, args, []string{testPref1, testPref2})
	})
	// Send both 6in4 and 4in4 packets. Verify that the packets have their outer
	// v4 header stripped and are forwarded according to the route in the DEFAULT
	// VRF that matches the inner IP address.
	portList := []string{"port8"}

	flow1 := createFlow(&flowArgs{flowName: "flow1",
		outHdrSrcIP: ipv4OuterSrc111, outHdrDstIP: traffiDstIP1, InnHdrSrcIPv6: atePort1.IPv6,
		InnHdrDstIPv6: ipv6InnerDst, isInnHdrV4: false, outHdrDscp: []uint32{dscpEncapNoMatch}})

	flow2 := createFlow(&flowArgs{flowName: "flow2",
		outHdrSrcIP: ipv4OuterSrc111, outHdrDstIP: traffiDstIP2, InnHdrSrcIP: atePort1.IPv4,
		InnHdrDstIP: ipv4InnerDst, isInnHdrV4: true, outHdrDscp: []uint32{dscpEncapNoMatch}})

	sendTraffic(t, args, portList, []gosnappi.Flow{flow1, flow2})
	verifyTraffic(t, args, []string{"flow1", "flow2"}, !wantLoss)
	captureAndValidatePackets(t, args, &packetValidation{portName: portList[0],
		outDstIP: []string{traffiDstIP1}, inHdrIP: ipv4InnerDst, validateTTL: false, validateDecap: true})

	// Test with packets with a destination address that does not match
	// the decap route, and verify that such packets are not decapped.
	portList = []string{"port4"}
	t.Run("Send traffic to non decap route and verify the behavior", func(t *testing.T) {
		flow4 := createFlow(&flowArgs{flowName: flowNegTest,
			outHdrSrcIP: ipv4OuterSrc111, outHdrDstIP: gribiIPv4EntryVRF1111, isInnHdrV4: true,
			InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: ipv4InnerDst})
		sendTraffic(t, args, portList, []gosnappi.Flow{flow4})
		verifyTraffic(t, args, []string{flowNegTest}, !wantLoss)
		captureAndValidatePackets(t, args, &packetValidation{portName: portList[0],
			outDstIP: []string{gribiIPv4EntryVRF1111}, inHdrIP: ipv4InnerDst, validateNoDecap: true})
	})
}

// testTunnelTrafficNoDecap is validate Test-4: Tunneled traffic with no decap:
// Ensures that tunneled traffic is correctly forwarded when there is no match in the DECAP_VRF.
// The intent of this test is to ensure that the VRF selection policy correctly sends these
// packets to either TE_VRF_111 or TE_VRF_222.
func testTunnelTrafficNoDecap(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, args *testArgs) {
	t.Helper()

	t.Log("Flush existing gRIBI routes before test.")
	if err := gribi.FlushAll(args.client); err != nil {
		t.Fatal(err)
	}

	// Configure GRIBi baseline AFTs.
	configGribiBaselineAFT(ctx, t, dut, args)

	// TODO : https://github.com/open-traffic-generator/fp-testbed-juniper/issues/42
	// Below code will be uncommented once ixia issue is fixed.
	/*
		portList := []string{"port2"}

		flow1 := createFlow(&flowArgs{flowName: "flow1",
			outHdrSrcIP: ipv4OuterSrc111, outHdrDstIP: gribiIPv4EntryVRF1111, outHdrDscp: []uint32{dscpEncapNoMatch, dscpEncapA1},
			InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: ipv4InnerDst, isInnHdrV4: true, udp: true, inHdrDscp: []uint32{dscpEncapA1}})

		flow2 := createFlow(&flowArgs{flowName: "flow2",
			outHdrSrcIP: ipv4OuterSrc111, outHdrDstIP: gribiIPv4EntryVRF1111, InnHdrSrcIPv6: atePort1.IPv6, inHdrDscp: []uint32{dscpEncapA1},
			InnHdrDstIPv6: ipv6InnerDst, isInnHdrV4: false, udp: true, outHdrDscp: []uint32{dscpEncapNoMatch, dscpEncapA1}})

		sendTraffic(t, args, portList, []gosnappi.Flow{flow1, flow2})
		verifyTraffic(t, args, []string{"flow1", "flow2"}, !wantLoss)
		captureAndValidatePackets(t, args, &packetValidation{portName: portList[0],
			outDstIP: []string{gribiIPv4EntryVRF1111}, inHdrIP: ipv4InnerDst, validateEncap: true})

		wantWeights := []float64{
			0.0625, // 6.25  Port2
			0.1875, // 18.75 Port3
			0.75,   // 75.0  Port4
			0,      // 0 Port5
			0,      // 0 Port6
			0,      // 0 Port7
			0,      // 0 Port8
		}
		validateTrafficDistribution(t, args.ate, wantWeights)

		flow3 := createFlow(&flowArgs{flowName: "flow3",
			outHdrSrcIP: ipv4OuterSrc222, outHdrDstIP: gribiIPv4EntryVRF2221, outHdrDscp: []uint32{dscpEncapNoMatch, dscpEncapB1},
			InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: ipv4InnerDst, isInnHdrV4: true, udp: true})

		flow4 := createFlow(&flowArgs{flowName: "flow4",
			outHdrSrcIP: ipv4OuterSrc222, outHdrDstIP: gribiIPv4EntryVRF2221, InnHdrSrcIPv6: atePort1.IPv6,
			InnHdrDstIPv6: ipv6InnerDst, isInnHdrV4: false, udp: true, outHdrDscp: []uint32{dscpEncapNoMatch, dscpEncapB1}})

		sendTraffic(t, args, portList, []gosnappi.Flow{flow3, flow4})
		verifyTraffic(t, args, []string{"flow3", "flow4"}, !wantLoss)
		captureAndValidatePackets(t, args, &packetValidation{portName: portList[0],
			outDstIP: []string{gribiIPv4EntryVRF2221}, inHdrIP: ipv4InnerDst, validateEncap: true})

		// Verify received pkts on DUT port-5.
		outTrafficCounters := gnmi.OTG().Port("port1").State()
		outPkts := gnmi.Get(t, args.ate.OTG(), outTrafficCounters).GetCounters().GetOutFrames()

		inTrafficCounters := gnmi.OTG().Port("port5").State()
		inPkts := gnmi.Get(t, args.ate.OTG(), inTrafficCounters).GetCounters().GetInFrames()

		if (outPkts - inPkts) < tolerancePct {
			t.Error("Traffic did not egressed through DUT port5")
		}
	*/
}

// testTunnelTrafficMatchDefaultTerm is to validate subtest 5.
// Test-5: match on "default term", send to default VRF
// Tests support for TE disabled IPinIP IPv4 (IP protocol 4) cluster traffic arriving on WAN
// facing ports. Specifically, this test verifies the tunnel traffic identification using
// ipv4_outer_src_111 and ipv4_outer_src_222 in the VRF selection policy.
func testTunnelTrafficMatchDefaultTerm(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, args *testArgs) {
	t.Helper()

	t.Log("Flush existing gRIBI routes before test.")
	if err := gribi.FlushAll(args.client); err != nil {
		t.Fatal(err)
	}

	// Configure GRIBi baseline AFTs.
	configGribiBaselineAFT(ctx, t, dut, args)

	portList := []string{"port8"}

	flow1 := createFlow(&flowArgs{flowName: "flow1",
		outHdrSrcIP: ipv4OuterSrc333, outHdrDstIP: ipv4InnerDst,
		InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: ipv4InnerDst2, isInnHdrV4: true})

	flow2 := createFlow(&flowArgs{flowName: "flow2",
		outHdrSrcIP: ipv4OuterSrc333, outHdrDstIP: ipv4InnerDst, InnHdrSrcIPv6: atePort1.IPv6,
		InnHdrDstIPv6: ipv6InnerDst2, isInnHdrV4: false})

	sendTraffic(t, args, portList, []gosnappi.Flow{flow1, flow2})
	verifyTraffic(t, args, []string{"flow1", "flow2"}, !wantLoss)

	// Verify received pkts on DUT port-8 per the route in the DEFAULT VRF.
	outTrafficCounters := gnmi.OTG().Port("port1").State()
	outPkts := gnmi.Get(t, args.ate.OTG(), outTrafficCounters).GetCounters().GetOutFrames()

	inTrafficCounters := gnmi.OTG().Port("port8").State()
	inPkts := gnmi.Get(t, args.ate.OTG(), inTrafficCounters).GetCounters().GetInFrames()

	if (outPkts - inPkts) < tolerancePct {
		t.Error("Traffic did not egressed through Default VRF")
	}

	flow3 := createFlow(&flowArgs{flowName: "flow3",
		outHdrSrcIP: ipv4OuterSrc111, outHdrDstIP: ipv4InnerDst, proto: 17, udp: true,
		InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: ipv4InnerDst2, isInnHdrV4: true})

	flow4 := createFlow(&flowArgs{flowName: "flow4",
		outHdrSrcIP: ipv4OuterSrc222, outHdrDstIP: ipv4InnerDst, proto: 17, udp: true,
		InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: ipv4InnerDst2, isInnHdrV4: true})

	sendTraffic(t, args, portList, []gosnappi.Flow{flow3, flow4})
	verifyTraffic(t, args, []string{"flow3", "flow4"}, !wantLoss)

	// Verify received pkts on DUT port-8 per the route in the DEFAULT VRF.
	outTrafficCounters = gnmi.OTG().Port("port1").State()
	outPkts = gnmi.Get(t, args.ate.OTG(), outTrafficCounters).GetCounters().GetOutFrames()

	inTrafficCounters = gnmi.OTG().Port("port8").State()
	inPkts = gnmi.Get(t, args.ate.OTG(), inTrafficCounters).GetCounters().GetInFrames()

	if (outPkts - inPkts) < tolerancePct {
		t.Error("Traffic did not egressed through Default VRF")
	}

	// Remove the matching route (e.g. stop the BGP routes) in
	// the DEFAULT VRF and verify that the traffic are dropped.
	updateBgpRoutes(t, args, routeDelete)
	sendTraffic(t, args, portList, []gosnappi.Flow{flow3, flow4})
	verifyTraffic(t, args, []string{"flow3", "flow4"}, wantLoss)

	// Add deleted bgp routes.
	updateBgpRoutes(t, args, !routeDelete)
}

// testTunnelTrafficDecapEncap is to validate subtest Test-6 decap then encap.
func testTunnelTrafficDecapEncap(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, args *testArgs) {
	t.Helper()

	t.Log("Flush existing gRIBI routes before test.")
	if err := gribi.FlushAll(args.client); err != nil {
		t.Fatal(err)
	}

	// Configure GRIBi baseline AFTs.
	configGribiBaselineAFT(ctx, t, dut, args)

	t.Run("Program gRIBi decap route", func(t *testing.T) {
		configureGribiRoute(ctx, t, dut, args, ipv4OuterDst111+"/32")
	})

	// TODO : https://github.com/open-traffic-generator/fp-testbed-juniper/issues/42
	// Below code will be uncommented once ixia issue is fixed.
	/*
		portList := []string{"port2"}

		flow1 := createFlow(&flowArgs{flowName: "flow1",
			outHdrSrcIP: ipv4OuterSrc222, outHdrDstIP: ipv4OuterDst111, outHdrDscp: []uint32{dscpEncapA1},
			InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: ipv4InnerDst, isInnHdrV4: true, udp: true, inHdrDscp: []uint32{dscpEncapA1}})

		flow2 := createFlow(&flowArgs{flowName: "flow2",
			outHdrSrcIP: ipv4OuterSrc111, outHdrDstIP: ipv4OuterDst111, outHdrDscp: []uint32{dscpEncapA1},
			InnHdrSrcIPv6: atePort1.IPv6, InnHdrDstIPv6: ipv6InnerDst, isInnHdrV4: false, udp: true, inHdrDscp: []uint32{dscpEncapA1}})

		sendTraffic(t, args, portList, []gosnappi.Flow{flow1, flow2})
		verifyTraffic(t, args, []string{"flow1", "flow2"}, !wantLoss)

		captureAndValidatePackets(t, args, &packetValidation{portName: portList[0],
			outDstIP: []string{gribiIPv4EntryVRF1111, gribiIPv4EntryVRF1112}, inHdrIP: ipv4InnerDst, validateEncap: true})

		wantWeights := []float64{
			0.0156, // 1.56 Port2
			0.0468, // 4.68 Port3
			0.1875, // 18.75 Port4
			0,      // 0 Port5
			0.75,   // 75.0 Port6
			0,      // 0 Port7
			0,      // 0 Port8
		}
		validateTrafficDistribution(t, args.ate, wantWeights)

		flow3 := createFlow(&flowArgs{flowName: "flow3",
			outHdrSrcIP: ipv4OuterSrc111, outHdrDstIP: ipv4OuterDst111, outHdrDscp: []uint32{dscpEncapB1},
			InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: ipv4InnerDst, isInnHdrV4: true, udp: true, inHdrDscp: []uint32{dscpEncapB1}})

		flow4 := createFlow(&flowArgs{flowName: "flow4",
			outHdrSrcIP: ipv4OuterSrc222, outHdrDstIP: ipv4OuterDst111, outHdrDscp: []uint32{dscpEncapB1},
			InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: ipv4InnerDst, isInnHdrV4: true, udp: true, inHdrDscp: []uint32{dscpEncapB1}})

		sendTraffic(t, args, portList, []gosnappi.Flow{flow3, flow4})
		verifyTraffic(t, args, []string{"flow3", "flow4"}, !wantLoss)

		captureAndValidatePackets(t, args, &packetValidation{portName: args.otgConfig.Ports().Items()[1].Name(),
			outDstIP: []string{gribiIPv4EntryVRF1111, gribiIPv4EntryVRF1112}, inHdrIP: ipv4InnerDst, validateEncap: true})

		wantWeights = []float64{
			0.0468, // 4.68 Port2
			0.1406, // 14.06 Port3
			0.5625, // 56.25 Port4
			0,      // 0 Port5
			0.25,   // 25 Port6
			0,      // 0 Port7
			0,      // 0 Port8
		}
		validateTrafficDistribution(t, args.ate, wantWeights)
	*/
}

// TestMatchSourceAndProtoNoMatchDSCP is to test support for decap/encap for gRIBI routes.
// Test VRF selection logic involving different decapsulation and encapsulation lookup scenarios
// via gRIBI.
func TestGribiDecap(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")
	gribic := dut.RawAPIs().GRIBI(t)
	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()

	// Baseline config
	t.Run("Configure Default Network Instance", func(t *testing.T) {
		fptest.ConfigureDefaultNetworkInstance(t, dut)
	})

	t.Run("Configure Non-Default Network Instances", func(t *testing.T) {
		configNonDefaultNetworkInstance(t, dut)
	})

	t.Run("Configure interfaces on DUT", func(t *testing.T) {
		configureDUT(t, dut)
	})

	t.Run("Apply vrf selectioin policy W to DUT port-1", func(t *testing.T) {
		configureVrfSelectionPolicyW(t, dut)
	})

	t.Log("Install BGP route resolved by ISIS.")
	t.Log("Configure ISIS on DUT")
	configureISIS(t, dut, []string{dut.Port(t, "port8").Name(), loopbackIntfName}, dutAreaAddress, dutSysID)

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	gnmi.Delete(t, dut, dutConfPath.Config())
	dutConf := bgpCreateNbr(dutAS, dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
	fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))

	otg := ate.OTG()
	var otgConfig gosnappi.Config
	t.Run("Configure OTG", func(t *testing.T) {
		otgConfig = configureOTG(t, otg, ate)
	})

	verifyISISTelemetry(t, dut, dut.Port(t, "port8").Name())
	verifyBgpTelemetry(t, dut)

	// Connect gRIBI client to DUT referred to as gRIBI - using PRESERVE persistence and
	// SINGLE_PRIMARY mode, with FIB ACK requested. Specify gRIBI as the leader.
	client := fluent.NewClient()
	client.Connection().WithStub(gribic).WithPersistence().WithInitialElectionID(1, 0).
		WithFIBACK().WithRedundancyMode(fluent.ElectedPrimaryClient)
	client.Start(ctx, t)
	defer client.Stop(t)

	defer func() {
		if err := gribi.FlushAll(client); err != nil {
			t.Error(err)
		}
	}()

	client.StartSending(ctx, t)
	if err := awaitTimeout(ctx, t, client, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation for clientA: %v", err)
	}
	eID := gribi.BecomeLeader(t, client)

	args := &testArgs{
		ctx:        ctx,
		client:     client,
		dut:        dut,
		ate:        ate,
		otgConfig:  otgConfig,
		top:        top,
		electionID: eID,
		otg:        otg,
	}

	t.Run("Test-1: Match on source and protocol, no match on DSCP; flow VRF_DECAP hit -> DEFAULT", func(t *testing.T) {
		testGribiDecapMatchSrcProtoNoMatchDSCP(ctx, t, dut, args)
	})
	t.Run("Test-2: match on source, protocol and DSCP, VRF_DECAP hit -> VRF_ENCAP_A miss -> DEFAULT", func(t *testing.T) {
		testGribiDecapMatchSrcProtoDSCP(ctx, t, dut, args)
	})

	t.Run("Test-3: Mixed Prefix Decap gRIBI Entries", func(t *testing.T) {
		if deviations.GribiDecapMixedPlenUnsupported(dut) {
			t.Skip("Gribi route programming with mixed prefix length is not supported.")
		}
		testGribiDecapMixedLenPref(ctx, t, dut, args)
	})

	t.Log("Delete vrf selection policy W and Apply vrf selectioin policy C.")
	deleteVrfSelectionPolicy(t, dut)
	configureVrfSelectionPolicyC(t, dut)

	t.Run("Test-4: Tunneled traffic with no decap", func(t *testing.T) {
		testTunnelTrafficNoDecap(ctx, t, dut, args)
	})

	t.Log("Delete vrf selection policy C and Apply vrf selectioin policy W.")
	deleteVrfSelectionPolicy(t, dut)
	configureVrfSelectionPolicyW(t, dut)

	t.Run("Test-5: Match on default term and send to default VRF", func(t *testing.T) {
		testTunnelTrafficMatchDefaultTerm(ctx, t, dut, args)
	})

	t.Run("Test-6: Decap then encap", func(t *testing.T) {
		testTunnelTrafficDecapEncap(ctx, t, dut, args)
	})
}
