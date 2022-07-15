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

package backup_nh

import (
	"testing"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

const (
	pbrName = "PBR"
)

// configbasePBR, creates class map, policy and configures under source interface
func configbasePBR(t *testing.T, dut *ondatra.DUTDevice, networkInstance, iptype string, index uint32, protocol telemetry.E_PacketMatchTypes_IP_PROTOCOL, dscpset []uint8) {
	port := dut.Port(t, "port1")
	pfpath := dut.Config().NetworkInstance("default").PolicyForwarding()
	//defer cleaning policy-forwarding
	// defer pfpath.Delete(t)

	r := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r.SequenceId = ygot.Uint32(index)
	r.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(networkInstance)}
	if iptype == "ipv4" {
		r.Ipv4 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol: protocol,
		}
		if len(dscpset) > 0 {
			r.Ipv4.DscpSet = dscpset
		}
	} else if iptype == "ipv6" {
		r.Ipv6 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv6{
			Protocol: protocol,
		}
		if len(dscpset) > 0 {
			r.Ipv6.DscpSet = dscpset
		}
	}
	pf := telemetry.NetworkInstance_PolicyForwarding{}
	p := pf.GetOrCreatePolicy(pbrName)
	p.Type = telemetry.Policy_Type_VRF_SELECTION_POLICY
	p.AppendRule(&r)
	dut.Config().NetworkInstance("default").PolicyForwarding().Replace(t, &pf)

	//defer pbr policy deletion
	// defer pfpath.Policy(pbrName).Delete(t)

	//configure PBR on ingress port
	pfpath.Interface(port.Name()).ApplyVrfSelectionPolicy().Replace(t, pbrName)
	//defer deletion of policy from interface
	//defer pfpath.Interface(port.Name()).ApplyVrfSelectionPolicy().Delete(t)
}
