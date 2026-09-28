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

	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/ondatra"
)

// DeviationCiscoTableConnectionsStatictoBGPMetricPropagation is used as an alternative to
// /network-instances/network-instance/table-connections/table-connection/config/disable-metric-propagation.
// In OC this path is set to 'false' by default, therefore enabling table-connections to propagate metrics
// from one protocol to another. This deviation implements CLI to perform the equivalent function.
func DeviationCiscoTableConnectionsStatictoBGPMetricPropagation(t *testing.T, dut *ondatra.DUTDevice, isV4 bool, metric int, routePolicyName string) {
	// router bgp 64512
	//
	//	address-family ipv4 unicast
	//	 redistribute static metric 104
	//	!
	//	address-family ipv6 unicast
	//	 redistribute static metric 106
	var aftype string
	if isV4 {
		aftype = "ipv4"
	} else {
		aftype = "ipv6"
	}
	cliConfig := fmt.Sprintf("router bgp 64512\n address-family %v unicast\n redistribute static metric %d\n !\n!\n", aftype, metric)
	if routePolicyName != "" {
		cliConfig = fmt.Sprintf("router bgp 64512\n address-family %v unicast\n redistribute static metric %d route-policy %s\n !\n!\n", aftype, metric, routePolicyName)
	}
	helpers.GnmiCLIConfig(t, dut, cliConfig)
}
