// Copyright 2022 Nokia, Google LLC
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
//
// This code is a Contribution to OpenConfig Feature Profiles project ("Work")
// made under the Google Software Grant and Corporate Contributor License
// Agreement ("CLA") and governed by the Apache License 2.0. No other rights
// or licenses in or to any of Nokia's intellectual property are granted for
// any other purpose. This code is provided on an "as is" basis without
// any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package fptest

import (
	"testing"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

var portSpeed = map[ondatra.Speed]oc.E_IfEthernet_ETHERNET_SPEED{
	ondatra.Speed1Gb:   oc.IfEthernet_ETHERNET_SPEED_SPEED_1GB,
	ondatra.Speed5Gb:   oc.IfEthernet_ETHERNET_SPEED_SPEED_5GB,
	ondatra.Speed10Gb:  oc.IfEthernet_ETHERNET_SPEED_SPEED_10GB,
	ondatra.Speed100Gb: oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB,
	ondatra.Speed400Gb: oc.IfEthernet_ETHERNET_SPEED_SPEED_400GB,
}

// SetPortSpeed sets the DUT config for the interface port-speed according
// to ondatra.Prot.Speed()
func SetPortSpeed(t *testing.T, p *ondatra.Port) {
	speed, ok := portSpeed[p.Speed()]
	if !ok {
		// Port speed is unspecified or unrecognized. Explicit config not performed
		return
	}
	t.Logf("Configuring %v port-speed to %v", p.Name(), speed)
	gnmi.Update(t, p.Device(), gnmi.OC().Interface(p.Name()).Ethernet().PortSpeed().Config(), speed)
}
