// Copyright 2026 Google LLC
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

package cfgplugins

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/ondatra"
)

// DisableHardwareNexthopProxy disables hardware nexthop proxying for devices that require it
// to ensure FIB-ACK works correctly. This is typically needed for Arista devices.
// See: https://partnerissuetracker.corp.google.com/issues/422275961
func DisableHardwareNexthopProxy(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	if !deviations.DisableHardwareNexthopProxy(dut) {
		return
	}

	switch dut.Vendor() {
	case ondatra.ARISTA:
		const aristaDisableNHGProxyCLI = "ip hardware fib next-hop proxy disabled"
		helpers.GnmiCLIConfig(t, dut, aristaDisableNHGProxyCLI)
	default:
		t.Fatalf("Deviation DisableHardwareNexthopProxy is not handled for the dut: %v", dut.Vendor())
	}
}
