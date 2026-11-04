// Copyright 2025 Google LLC
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
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/ondatra"
)

type GNPSIParams struct {
	Port       int
	SSLProfile string
}

func ConfigureGNPSI(t *testing.T, dut *ondatra.DUTDevice, params *GNPSIParams) {
	if deviations.GnpsiOcUnsupported(dut) {
		configureGNPSIFromCLI(t, dut, params)
	} else {
		configureGNPSIFromOC(t, dut)
	}
}

func configureGNPSIFromOC(t *testing.T, dut *ondatra.DUTDevice) {
	// TODO : Implement GNPSI OC configuration when supported in OC (ref: https://github.com/openconfig/public/pull/1385)
	t.Fatalf("gNPSI OC configuration is not supported: %s - %s", dut.Version(), dut.Model())
}

func configureGNPSIFromCLI(t *testing.T, dut *ondatra.DUTDevice, params *GNPSIParams) {
	t.Log("Configuring gNPSI from CLI")
	t.Logf("Using gNPSI port: %d", params.Port)
	switch dut.Vendor() {
	case ondatra.ARISTA:
		cli := fmt.Sprintf(`
        management api gnpsi
        transport grpc test
        ssl profile %s
        port %d
        source sFlow
        no disabled
        `, params.SSLProfile, params.Port)
		helpers.GnmiCLIConfig(t, dut, cli)
	case ondatra.JUNIPER:
		// The gNPSI service relies on sFlow configuration which should already be configured
		t.Log("For Juniper devices, gNPSI is implicitely configured with sFlow. No additional CLI configuration is required.")
	
	default:
		t.Errorf("gNPSI configuration not implemented for vendor %s", dut.Vendor())
	}
}
