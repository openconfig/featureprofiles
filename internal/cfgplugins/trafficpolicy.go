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
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func NewTrafficPolicy(t *testing.T, dut *ondatra.DUTDevice, policyName string, ipv4InnerDstA string, ipv4InnerDstB string, innerIpv4Prefix int, ipv6InnerDstA string, ipv6InnerDstB string, innerIpv6Prefix int, nextHopGName string, intName string) {
	if deviations.TrafficPolicyToNextHopOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			cliConfig := fmt.Sprintf(`
				traffic-policies
		traffic-policy %s
		match IPv4_DA ipv4
		destination prefix %s/%d
		destination prefix %s/%d
		actions
		count
		redirect next-hop group %s
		match IPV6_DA ipv6
		destination prefix %s/%d 
		destination prefix %s/%d
		actions
		count
		redirect next-hop group %s
		match ipv4-all-default ipv4
		match ipv6-all-default ipv6
		interface %s
		traffic-policy input %s
		`, policyName, ipv4InnerDstB, innerIpv4Prefix, ipv4InnerDstA, innerIpv4Prefix, nextHopGName, ipv6InnerDstA, innerIpv6Prefix, ipv6InnerDstB, innerIpv6Prefix, nextHopGName, intName, policyName)
			helpers.GnmiCLIConfig(t, dut, cliConfig)
		default:
			t.Errorf("Deviation StaticMplsLspUnsupported is not handled for the dut: %v", dut.Vendor())
		}
	} else {
		dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
		policyDutConf := configForwardingPolicy(t, dut, policyName, ipv4InnerDstB)
		applyForwardingPolicy(t, intName, policyName)
		gnmi.Replace(t, dut, dutConfPath.PolicyForwarding().Config(), policyDutConf)
	}
}

func configForwardingPolicy(t *testing.T, dut *ondatra.DUTDevice, policyName string, ipv4InnerDstB string) *oc.NetworkInstance_PolicyForwarding {
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	policyFwding := ni.GetOrCreatePolicyForwarding()

	fwdPolicy := policyFwding.GetOrCreatePolicy(policyName)
	fwdPolicy.SetPolicyId(*fwdPolicy.PolicyId)
	fwdPolicy.SetType(oc.Policy_Type_PBR_POLICY)
	policyRule := fwdPolicy.GetOrCreateRule(1)
	policyRule.Ipv4.SetDestinationAddress(ipv4InnerDstB)
	policyRule.Ipv4.SetHopLimit(1)
	ruleAction := policyRule.GetOrCreateAction()
	t.Log(ruleAction) // "set-ip-ttl" oc not have support...ticket: https://github.com/openconfig/public/pull/1263/files

	return policyFwding
}

// applyForwardingPolicy applies the forwarding policy on the interface.
func applyForwardingPolicy(t *testing.T, ingressPort string, policyName string) {
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
