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

// Package vrfpolicy contains functions to build specific vrf policies
package vrfpolicy

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
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

type ipv4 struct {
	protocol   oc.UnionUint8
	dscpSet    []uint8
	sourceAddr string
}

type action struct {
	decapNI         string
	postDecapNI     string
	decapFallbackNI string
}

type policyFwRule struct {
	seqID  uint32
	ipv4   ipv4
	action action
}

// configureNetworkInstance configures vrfs DECAP_TE_VRF, ENCAP_TE_VRF_A, ENCAP_TE_VRF_B,
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

// BuildVRFSelectionPolicyW vrf selection policy rule
// Reference: https://github.com/openconfig/featureprofiles/blob/main/feature/gribi/vrf_policy_driven_te/README.md?plain=1#L252
func BuildVRFSelectionPolicyW(t *testing.T, dut *ondatra.DUTDevice, niName string) *oc.NetworkInstance_PolicyForwarding {
	d := &oc.Root{}
	configNonDefaultNetworkInstance(t, dut)

	pfRule1 := &policyFwRule{
		seqID:  1,
		ipv4:   ipv4{protocol: 4, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc222WithMask},
		action: action{decapNI: niDecapTeVrf, postDecapNI: niEncapTeVrfA, decapFallbackNI: niTeVrf222},
	}
	pfRule2 := &policyFwRule{
		seqID:  2,
		ipv4:   ipv4{protocol: 41, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc222WithMask},
		action: action{decapNI: niDecapTeVrf, postDecapNI: niEncapTeVrfA, decapFallbackNI: niTeVrf222},
	}
	pfRule3 := &policyFwRule{
		seqID:  3,
		ipv4:   ipv4{protocol: 4, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc111WithMask},
		action: action{decapNI: niDecapTeVrf, postDecapNI: niEncapTeVrfA, decapFallbackNI: niTeVrf111},
	}
	pfRule4 := &policyFwRule{
		seqID:  4,
		ipv4:   ipv4{protocol: 41, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc111WithMask},
		action: action{decapNI: niDecapTeVrf, postDecapNI: niEncapTeVrfA, decapFallbackNI: niTeVrf111},
	}

	pfRule5 := &policyFwRule{
		seqID:  5,
		ipv4:   ipv4{protocol: 4, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc222WithMask},
		action: action{decapNI: niDecapTeVrf, postDecapNI: niEncapTeVrfB, decapFallbackNI: niTeVrf222},
	}
	pfRule6 := &policyFwRule{
		seqID:  6,
		ipv4:   ipv4{protocol: 41, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc222WithMask},
		action: action{decapNI: niDecapTeVrf, postDecapNI: niEncapTeVrfB, decapFallbackNI: niTeVrf222},
	}
	pfRule7 := &policyFwRule{
		seqID:  7,
		ipv4:   ipv4{protocol: 4, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc111WithMask},
		action: action{decapNI: niDecapTeVrf, postDecapNI: niEncapTeVrfB, decapFallbackNI: niTeVrf111},
	}
	pfRule8 := &policyFwRule{
		seqID:  8,
		ipv4:   ipv4{protocol: 41, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc111WithMask},
		action: action{decapNI: niDecapTeVrf, postDecapNI: niEncapTeVrfB, decapFallbackNI: niTeVrf111},
	}

	pfRule9 := &policyFwRule{
		seqID:  9,
		ipv4:   ipv4{protocol: 4, sourceAddr: ipv4OuterSrc222WithMask},
		action: action{decapNI: niDecapTeVrf, postDecapNI: niDefault, decapFallbackNI: niTeVrf222},
	}
	pfRule10 := &policyFwRule{
		seqID:  10,
		ipv4:   ipv4{protocol: 41, sourceAddr: ipv4OuterSrc222WithMask},
		action: action{decapNI: niDecapTeVrf, postDecapNI: niDefault, decapFallbackNI: niTeVrf222},
	}
	pfRule11 := &policyFwRule{
		seqID:  11,
		ipv4:   ipv4{protocol: 4, sourceAddr: ipv4OuterSrc111WithMask},
		action: action{decapNI: niDecapTeVrf, postDecapNI: niDefault, decapFallbackNI: niTeVrf111},
	}
	pfRule12 := &policyFwRule{
		seqID:  12,
		ipv4:   ipv4{protocol: 41, sourceAddr: ipv4OuterSrc111WithMask},
		action: action{decapNI: niDecapTeVrf, postDecapNI: niDefault, decapFallbackNI: niTeVrf111},
	}

	pfRuleList := []*policyFwRule{
		pfRule1, pfRule2, pfRule3, pfRule4, pfRule5, pfRule6,
		pfRule7, pfRule8, pfRule9, pfRule10, pfRule11, pfRule12,
	}

	if deviations.PfRequireSequentialOrderPbrRules(dut) {
		pfRule10.seqID = 910
		pfRule11.seqID = 911
		pfRule12.seqID = 912
	}

	ni := d.GetOrCreateNetworkInstance(niName)
	niP := ni.GetOrCreatePolicyForwarding()
	niPf := niP.GetOrCreatePolicy(vrfPolW)
	niPf.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)

	for _, pfRule := range pfRuleList {
		pfR := niPf.GetOrCreateRule(pfRule.seqID)
		pfRProtoIPv4 := pfR.GetOrCreateIpv4()
		pfRProtoIPv4.Protocol = oc.UnionUint8(pfRule.ipv4.protocol)
		if pfRule.ipv4.dscpSet != nil {
			pfRProtoIPv4.DscpSet = pfRule.ipv4.dscpSet
		}
		pfRProtoIPv4.SourceAddress = ygot.String(pfRule.ipv4.sourceAddr)
		pfRAction := pfR.GetOrCreateAction()
		pfRAction.DecapNetworkInstance = ygot.String(pfRule.action.decapNI)
		pfRAction.PostDecapNetworkInstance = ygot.String(pfRule.action.postDecapNI)
		pfRAction.DecapFallbackNetworkInstance = ygot.String(pfRule.action.decapFallbackNI)
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

	return niP
}

// ConfigureVRFSelectionPolicyW configures vrf selection policy W on default NI and applies to DUT port1
func ConfigureVRFSelectionPolicyW(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	port1 := dut.Port(t, "port1")
	interfaceID := port1.Name()
	if deviations.InterfaceRefInterfaceIDFormat(dut) {
		interfaceID = interfaceID + ".0"
	}

	niForwarding := BuildVRFSelectionPolicyW(t, dut, deviations.DefaultNetworkInstance(dut))
	dutForwardingPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding()
	gnmi.Replace(t, dut, dutForwardingPath.Config(), niForwarding)

	interface1 := niForwarding.GetOrCreateInterface(interfaceID)
	interface1.ApplyVrfSelectionPolicy = ygot.String(vrfPolW)
	interface1.GetOrCreateInterfaceRef().Interface = ygot.String(port1.Name())
	interface1.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	if deviations.InterfaceRefConfigUnsupported(dut) {
		interface1.InterfaceRef = nil
	}
	gnmi.Replace(t, dut, dutForwardingPath.Interface(interfaceID).Config(), interface1)
}

// DeletePolicyForwarding deletes policy configured under given interface.
func DeletePolicyForwarding(t *testing.T, dut *ondatra.DUTDevice, portID string) {
	t.Helper()
	p1 := dut.Port(t, portID)
	ingressPort := p1.Name()
	interfaceID := ingressPort
	if deviations.InterfaceRefInterfaceIDFormat(dut) {
		interfaceID = ingressPort + ".0"
	}
	pfpath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Interface(interfaceID)
	gnmi.Delete(t, dut, pfpath.Config())
}
