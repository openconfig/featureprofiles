// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cfgplugins

import (
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	ethertypeIPv4 = oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4
	ethertypeIPv6 = oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV6
	seqIDBase     = uint32(10)
)

// PbrRule defines a policy-based routing rule configuration
type PbrRule struct {
	Sequence  uint32
	EtherType oc.NetworkInstance_PolicyForwarding_Policy_Rule_L2_Ethertype_Union
	EncapVrf  string
}

// NewPolicyForwardingVRFSelection creates policy-based routing configuration for VRF selection
func NewPolicyForwardingVRFSelection(dut *ondatra.DUTDevice, name string) *oc.NetworkInstance_PolicyForwarding {
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	pf := ni.GetOrCreatePolicyForwarding()
	p := pf.GetOrCreatePolicy(name)
	p.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)

	for _, pRule := range getPbrRules(dut) {
		r := p.GetOrCreateRule(seqIDOffset(dut, pRule.Sequence))

		if deviations.PfRequireMatchDefaultRule(dut) {
			if pRule.EtherType != nil {
				r.GetOrCreateL2().Ethertype = pRule.EtherType
			}
		}

		if pRule.EncapVrf != "" {
			r.GetOrCreateAction().SetNetworkInstance(pRule.EncapVrf)
		}
	}
	return pf
}

// getPbrRules returns policy-based routing rules for VRF selection
func getPbrRules(dut *ondatra.DUTDevice) []PbrRule {
	vrfDefault := deviations.DefaultNetworkInstance(dut)

	if deviations.PfRequireMatchDefaultRule(dut) {
		return []PbrRule{
			{
				Sequence:  17,
				EtherType: ethertypeIPv4,
				EncapVrf:  vrfDefault,
			},
			{
				Sequence:  18,
				EtherType: ethertypeIPv6,
				EncapVrf:  vrfDefault,
			},
		}
	}
	return []PbrRule{
		{
			Sequence: 17,
			EncapVrf: vrfDefault,
		},
	}
}

// seqIDOffset returns sequence ID with base offset to ensure proper ordering
func seqIDOffset(dut *ondatra.DUTDevice, i uint32) uint32 {
	if deviations.PfRequireSequentialOrderPbrRules(dut) {
		return i + seqIDBase
	}
	return i
}
