// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package parent_component_validation_test

import (
	"regexp"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func checkParentComponent(t *testing.T, dut *ondatra.DUTDevice, entity string) string {
	parent := gnmi.Lookup(t, dut, gnmi.OC().Component(entity).Parent().State())
	val, present := parent.Val()

	if !present {
		t.Errorf("Parent component NOT found for entify: %s", entity)
	}
	gotV := gnmi.Lookup(t, dut, gnmi.OC().Component(val).Name().State())
	got, present := gotV.Val()

	if present {
		t.Logf("Found parent component %s for entity %s", got, entity)
	}
	return got
}

// TestInterfaceParentComponent tests that the parent component of any given interface is a Switch Chip.
func TestInterfaceParentComponent(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	parentComponentRegex := regexp.MustCompile("^(SwitchChip(?:[0-9]+|[0-9]/[0-9])?|NPU[0-9]|[0-9]/(?:[0-9]|RP[0-9])/CPU[0-9]-NPU[0-9]|FPC[0-9]+:PIC[0-9]:NPU[0-9]+)$")
	cases := []struct {
		desc string
		port string
	}{
		{
			desc: "Port1",
			port: "port1",
		},
		{
			desc: "Port2",
			port: "port2",
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			dp := dut.Port(t, tc.port)
			hardwarePortName := gnmi.Lookup(t, dut, gnmi.OC().Interface(dp.Name()).HardwarePort().State())
			hVal, present := hardwarePortName.Val()
			if !present {
				t.Errorf("Hardware port NOT found for interface: %s", dp.Name())
			}
			parent := checkParentComponent(t, dut, hVal)
			t.Logf("Interface %s parent is %s", dp.Name(), parent)
			if !parentComponentRegex.MatchString(parent) {
				t.Errorf("Interface %s parent %q did not match pattern %s", dp.Name(), parent, parentComponentRegex.String())
			}
		})
	}
}
