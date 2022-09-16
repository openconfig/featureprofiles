// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package components provides functions to enumerate components from the device.
package components

import (
	"regexp"
	"testing"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
)

// FindComponentsByType finds the list of components based on hardware type.
func FindComponentsByType(t *testing.T, dut *ondatra.DUTDevice, cType telemetry.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT) []string {
	components := dut.Telemetry().ComponentAny().Name().Get(t)
	var s []string
	for _, c := range components {
		lookupType := dut.Telemetry().Component(c).Type().Lookup(t)
		if !lookupType.IsPresent() {
			t.Logf("Component %s type is missing from telemetry", c)
			continue
		}
		componentType := lookupType.Val(t)
		t.Logf("Component %s has type: %v", c, componentType)
		switch v := componentType.(type) {
		case telemetry.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT:
			if v == cType {
				s = append(s, c)
			}
		default:
			t.Logf("Detected non-hardware component: (%T, %v)", componentType, componentType)
		}
	}
	return s
}

// FindMatchingStrings filters out the components list based on regex pattern.
func FindMatchingStrings(components []string, r *regexp.Regexp) []string {
	var s []string
	for _, c := range components {
		if r.MatchString(c) {
			s = append(s, c)
		}
	}
	return s
}
