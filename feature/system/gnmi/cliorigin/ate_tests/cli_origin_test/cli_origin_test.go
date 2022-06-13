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
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/util"

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
//   - gnmic -a ipaddr:10162 -u username -p password --skip-verify get \
//       --path "cli:/show version"
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

	var intfConfig string = `
interface %s
  %s
`
	cases := []struct {
		desc                string
		intfOper            string
		expectedAdminStatus telemetry.E_Interface_AdminStatus
		expectedOperStatus  telemetry.E_Interface_OperStatus
	}{
		{
			desc:                "shutdown interface",
			intfOper:            "shutdown",
			expectedAdminStatus: telemetry.Interface_AdminStatus_DOWN,
			expectedOperStatus:  telemetry.Interface_OperStatus_DOWN,
		},
		{
			desc:                "no shutdown interface",
			intfOper:            "no shutdown",
			expectedAdminStatus: telemetry.Interface_AdminStatus_UP,
			expectedOperStatus:  telemetry.Interface_OperStatus_UP,
		},
	}

	gnmiClient := dut.RawAPIs().GNMI().Default(t)
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			config := fmt.Sprintf(intfConfig, dp1.Name(), tc.intfOper)
			t.Logf("Push the CLI config:\n%s", config)

			gpbSetRequest, err := buildCliConfigRequest(config)
			if err != nil {
				t.Fatalf("Cannot build a gNMI SetRequest: %v", err)
			}

			t.Log("gnmiClient Set CLI config")
			if _, err = gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
				t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
			}

			dut.Telemetry().Interface(dp1.Name()).OperStatus().Await(t, 2*time.Minute, tc.expectedOperStatus)
			dut.Telemetry().Interface(dp1.Name()).AdminStatus().Await(t, 2*time.Minute, tc.expectedAdminStatus)
			operStatus := dut.Telemetry().Interface(dp1.Name()).OperStatus().Get(t)
			if want := tc.expectedOperStatus; operStatus != want {
				t.Errorf("Get(DUT port1 oper status): got %v, want %v", operStatus, want)
			}
			adminStatus := dut.Telemetry().Interface(dp1.Name()).AdminStatus().Get(t)
			if want := tc.expectedAdminStatus; adminStatus != want {
				t.Errorf("Get(DUT port1 admin status): got %v, want %v", adminStatus, want)
			}
		})
	}
}

func TestOriginCliShowCmd(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	gnmiClient := dut.RawAPIs().GNMI().Default(t)
	cases := []struct {
		desc   string
		cliCmd string
	}{
		{
			desc:   "Get software version",
			cliCmd: "show version",
		},
		{
			desc:   "Get lldp neighbors list",
			cliCmd: "show lldp neighbors",
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {

			t.Log("gnmiClient Get CLI show version output")
			getResponse, err := gnmiClient.Get(context.Background(), &gpb.GetRequest{
				Path: []*gpb.Path{{
					Origin: "cli",
					Elem:   []*gpb.PathElem{{Name: tc.cliCmd}},
				}},
				Type:     gpb.GetRequest_OPERATIONAL,
				Encoding: gpb.Encoding_JSON_IETF,
			})
			if err != nil {
				t.Fatalf("Cannot fetch %s output from the DUT with error: %v", tc.cliCmd, err)
			}
			t.Logf("getResponse: %v", getResponse)

			if output, err := extractCliOutput(getResponse); err != nil {
				t.Errorf("extractCliOutput(getResponse) failed with error: %v", err)
			} else {
				t.Logf("Display the CLI command %v output in key/value pair:", tc.cliCmd)
				for k, v := range output {
					t.Logf("%v => %v", k, v)
				}
			}
		})
	}
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

func extractCliOutput(getResponse *gpb.GetResponse) (map[string]interface{}, error) {
	if got := len(getResponse.GetNotification()); got != 1 {
		return nil, fmt.Errorf("number of notifications got %d, want 1", got)
	}

	n := getResponse.GetNotification()[0]
	u := n.Update[0]
	path, err := util.JoinPaths(n.GetPrefix(), u.GetPath())
	if err != nil {
		return nil, err
	}
	if len(path.GetElem()) != 1 {
		return nil, fmt.Errorf("update path, got: %v, want 1", len(path.GetElem()))
	}
	var val map[string]interface{}
	if err = json.Unmarshal(u.GetVal().GetJsonIetfVal(), &val); err != nil {
		return nil, fmt.Errorf("cannot unmarshal the json content, err: %v", err)
	}
	if len(val) == 0 {
		return nil, fmt.Errorf("update val, got: %v, want non-empty map", val)
	}
	return val, nil
}
