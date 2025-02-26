// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package twofiftysix_ucmp_test

import (
	"testing"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	vrfDecap             = "DECAP_TE_VRF"
	vrfRepair            = "REPAIR"
	vrfRepaired          = "REPAIRED"
	vrfEncapA            = "ENCAP_TE_VRF_A"
	vrfDecapPostRepaired = "DECAP"
	dscpEncapA1          = 10
	ipv4OuterSrc111      = "198.50.100.111"
	ipv4FlowIP           = "138.0.11.8"
	ipv4EntryPrefixLen   = 24
	ipv6EntryPrefix      = "2015:aa8::"
	ipv6EntryPrefixLen   = 32
)

// configbasePBR, creates class map, policy and configures under source interface
func configbasePBR(t *testing.T, dut *ondatra.DUTDevice, networkInstance, iptype string, index uint32, protocol oc.E_PacketMatchTypes_IP_PROTOCOL, dscpset []uint8, pbrName string, intfName string, val bool) {
	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	r := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r.SequenceId = ygot.Uint32(index)
	if !val {
		r.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(networkInstance)}
	} else {
		decapVrfSet := []string{vrfDecap, vrfEncapA, "REPAIR"}
		r.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{DecapNetworkInstance: ygot.String(decapVrfSet[0]), PostDecapNetworkInstance: ygot.String(decapVrfSet[1]), DecapFallbackNetworkInstance: ygot.String(decapVrfSet[2])}

	}

	if iptype == "ipv4" {
		r.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol: protocol,
		}
		if len(dscpset) > 0 {
			r.Ipv4.DscpSet = dscpset
		}
	} else if iptype == "ipv6" {
		r4 := r.GetOrCreateIpv4()
		r4.Protocol = oc.UnionUint8(41)
	}
	pf := oc.NetworkInstance_PolicyForwarding{}
	p := pf.GetOrCreatePolicy(pbrName)
	p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
	p.AppendRule(&r)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).PolicyForwarding().Config(), &pf)

	//configure PBR on ingress port
	d := &oc.Root{}
	pfpath := d.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance).GetOrCreatePolicyForwarding().GetOrCreateInterface(intfName + ".0")
	pfpath.ApplyVrfSelectionPolicy = ygot.String(pbrName)
	pfpath.GetOrCreateInterfaceRef().Interface = ygot.String(intfName)
	pfpath.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	intfConfPath := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).PolicyForwarding().Interface(intfName + ".0")
	gnmi.Replace(t, dut, intfConfPath.Config(), pfpath)
}

// unconfigbasePBR, creates class map, policy and configures under source interface
func unconfigbasePBR(t *testing.T, dut *ondatra.DUTDevice, pbrName string, intfName string) {
	t.Helper()

	pfpath := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).PolicyForwarding()
	pfintfPath := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).PolicyForwarding().Interface(intfName + ".0")
	gnmi.Delete(t, dut, pfintfPath.Config())
	gnmi.Delete(t, dut, pfpath.Policy(pbrName).Config())
}
