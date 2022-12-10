// Copyright 2022 Google LLC
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

package cli_origin_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test cases:
//  - Connect ATE port-1 to DUT port-1.
//  - Push interface disabling configuration using SetRequest specifying:
//    - origin: “cli” (openconfig is default origin) - containing modeled configuration:.
//       interface <DUT port-1>
//         shutdown
//  - Validate that DUT port-1 admin status is shown as down through telemetry.
//  - Validate that DUT port-1 oper status is shown as down through telemetry.
//  - Push interface enabling configuration using SetRequest specifying:
//    - origin: “cli” (openconfig, default origin) - containing modelled configuration.
//       interface <DUT port-1>
//         no shutdown
//  - Validate that DUT port-1 admin status is shown as up through telemetry.
//  - Validate that DUT port-1 oper status is shown as up through telemetry.
//
//
// Topology:
//   dut:port1 <--> ate:port1
//
// Test notes:
//  - This test is intended to cover only the case of pushing some configuration.
//    Since it is unknown what CLI configuration would be required in the emergency case
//    that is covered by this requirement.
//
//  Sample gnmic command to send CLI config and get CLI command output using gmic:
//   - gnmic -a ipaddr:10162 -u username -p password --skip-verify \
//       --encoding ASCII set --update-path "cli:" \
//       --update-value "logging host 192.0.2.18 514"
//
//   - gnmic tool info:
//     - https://github.com/karimra/gnmic/blob/main/README.md
//

func TestOriginCliConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp1 := dut.Port(t, "port1")

	cases := []struct {
		desc                string
		intfOper            bool
		expectedAdminStatus oc.E_Interface_AdminStatus
		expectedOperStatus  oc.E_Interface_OperStatus
	}{
		{
			desc:                "Set interface admin status to down",
			intfOper:            false,
			expectedAdminStatus: oc.Interface_AdminStatus_DOWN,
			expectedOperStatus:  oc.Interface_OperStatus_DOWN,
		},
		{
			desc:                "Set interface admin status to up",
			intfOper:            true,
			expectedAdminStatus: oc.Interface_AdminStatus_UP,
			expectedOperStatus:  oc.Interface_OperStatus_UP,
		},
	}

	gnmiClient := dut.RawAPIs().GNMI().Default(t)
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			var config string

			switch dut.Vendor() {
			case ondatra.JUNIPER:
				config = juniperCLI(dp1.Name(), tc.intfOper)
			case ondatra.CISCO:
				config = ciscoCLI(dp1.Name(), tc.intfOper)
			case ondatra.ARISTA:
				config = aristaCLI(dp1.Name(), tc.intfOper)
			}

			t.Logf("Push the CLI config:\n%s", config)

			gpbSetRequest, err := buildCliConfigRequest(config)
			if err != nil {
				t.Fatalf("Cannot build a gNMI SetRequest: %v", err)
			}

			t.Log("gnmiClient Set CLI config")
			if _, err = gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
				t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
			}

			gnmi.Await(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), 2*time.Minute, tc.expectedOperStatus)
			gnmi.Await(t, dut, gnmi.OC().Interface(dp1.Name()).AdminStatus().State(), 2*time.Minute, tc.expectedAdminStatus)
			operStatus := gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State())
			if want := tc.expectedOperStatus; operStatus != want {
				t.Errorf("Get(DUT port1 oper status): got %v, want %v", operStatus, want)
			}
			adminStatus := gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).AdminStatus().State())
			if want := tc.expectedAdminStatus; adminStatus != want {
				t.Errorf("Get(DUT port1 admin status): got %v, want %v", adminStatus, want)
			}
		})
	}
}

func juniperCLI(intf string, enabled bool) string {
	op := "disable"
	if enabled {
		op = "enable"
	}
	return fmt.Sprintf(`
  interfaces {
	%s {
	  %s;
	}
  }
  `, intf, op)
}

func ciscoCLI(intf string, enabled bool) string {
	op := "shutdown"
	if enabled {
		op = "no shutdown"
	}
	return fmt.Sprintf(`
  interface %s
    %s
  `, intf, op)
}

func aristaCLI(intf string, enabled bool) string {
	op := "shutdown"
	if enabled {
		op = "no shutdown"
	}
	return fmt.Sprintf(`
  interface %s
    %s
  `, intf, op)

}

func buildCliConfigRequest(config string) (*gpb.SetRequest, error) {
	// Build config with Origin set to cli and Ascii encoded config.
	gpbSetRequest := &gpb.SetRequest{
		Update: []*gpb.Update{{
			Path: &gpb.Path{
				Origin: "cli",
				Elem:   []*gpb.PathElem{},
			},
			Val: &gpb.TypedValue{
				Value: &gpb.TypedValue_AsciiVal{
					AsciiVal: config,
				},
			},
		}},
	}
	return gpbSetRequest, nil
}
