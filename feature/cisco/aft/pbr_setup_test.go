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

package aft_test

import (
	"testing"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	pbrName = "PBR"
)

// configbasePBR, creates class map, policy and configures under source interface
func configbasePBR(t *testing.T, dut *ondatra.DUTDevice, networkInstance, iptype string, index uint32, protocol oc.E_PacketMatchTypes_IP_PROTOCOL, dscpset []uint8) {
	pfpath := gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding()

	r := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r.SequenceId = ygot.Uint32(index)
	r.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(networkInstance)}
	if iptype == "ipv4" {
		r.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol: protocol,
		}
		if len(dscpset) > 0 {
			r.Ipv4.DscpSet = dscpset
		}
	} else if iptype == "ipv6" {
		r.Ipv6 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv6{
			Protocol: protocol,
		}
		if len(dscpset) > 0 {
			r.Ipv6.DscpSet = dscpset
		}
	}
	pf := oc.NetworkInstance_PolicyForwarding{}
	p := pf.GetOrCreatePolicy(pbrName)
	p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
	p.AppendRule(&r)
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding().Config(), &pf)

	//configure PBR on ingress port
	gnmi.Replace(t, dut, pfpath.Interface("Bundle-Ether120").ApplyVrfSelectionPolicy().Config(), pbrName)
}

// unconfigbasePBR, creates class map, policy and configures under source interface
func unconfigbasePBR(t *testing.T, dut *ondatra.DUTDevice) {
	pfpath := gnmi.OC().NetworkInstance(*ciscoFlags.PbrInstance).PolicyForwarding()
	gnmi.Delete(t, dut, pfpath.Interface("Bundle-Ether120").ApplyVrfSelectionPolicy().Config())
	gnmi.Delete(t, dut, pfpath.Policy(pbrName).Config())
	gnmi.Delete(t, dut, pfpath.Config())
}
