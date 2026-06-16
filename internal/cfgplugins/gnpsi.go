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
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

type GNPSIParams struct {
	ServerName string
	Port       uint16
	SSLProfile string
	NIName     string
}

func ConfigureGNPSI(t *testing.T, dut *ondatra.DUTDevice, params *GNPSIParams) {
	t.Logf("Using gNPSI port: %d", params.Port)
	if deviations.GnpsiOcUnsupported(dut) {
		configureGNPSIFromCLI(t, dut, params)
	} else {
		configureGNPSIFromOC(t, dut, params)
	}
}

func configureGNPSIFromOC(t *testing.T, dut *ondatra.DUTDevice, params *GNPSIParams) {
	t.Log("Configuring gNPSI from OC")
	gnmiServerPath := gnmi.OC().System().GrpcServer(params.ServerName)
	gnmiServer := &oc.System_GrpcServer{
		Name:            ygot.String(params.ServerName),
		Port:            ygot.Uint16(params.Port),
		Enable:          ygot.Bool(true),
		NetworkInstance: ygot.String(params.NIName),
	}
	gnmiServer.Services = []oc.E_SystemGrpc_GRPC_SERVICE{oc.SystemGrpc_GRPC_SERVICE_GNPSI}
	gnmi.Replace(t, dut, gnmiServerPath.Config(), gnmiServer)
}

func configureGNPSIFromCLI(t *testing.T, dut *ondatra.DUTDevice, params *GNPSIParams) {
	t.Log("Configuring gNPSI from CLI")
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
	default:
		t.Errorf("gNPSI configuration not implemented for vendor %s", dut.Vendor())
	}
}
