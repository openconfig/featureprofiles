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
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/ocpath"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygnmi/ygnmi"
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

// GetSubcomponentPath creates a gNMI path based on the componnent name.
func GetSubcomponentPath(name string) *tpb.Path {
	if *deviations.GNOISubcomponentPath {
		return &tpb.Path{
			Elem: []*tpb.PathElem{{Name: name}},
		}
	}
	return &tpb.Path{
		Origin: "openconfig",
		Elem: []*tpb.PathElem{
			{Name: "components"},
			{Name: "component", Key: map[string]string{"name": name}},
		},
	}
}

// Y provides the ygnmi based components helper.  A ygnmi.Client is tied to a specific
// DUT.
type Y struct {
	*ygnmi.Client
}

// New creates a new ygnmi based helper from a *ondatra.DUTDevice.
func New(t testing.TB, dut *ondatra.DUTDevice) Y {
	gnmic := dut.RawAPIs().GNMI().Default(t)
	yc, err := ygnmi.NewClient(gnmic)
	if err != nil {
		t.Fatalf("Could not create ygnmi.Client: %v", err)
	}
	return Y{yc}
}

// FindByType finds the list of components based on component type.
func (y Y) FindByType(ctx context.Context, want oc.Component_Type_Union) ([]string, error) {
	var names []string

	anyTypePath := ocpath.Root().ComponentAny().Type()
	values, err := ygnmi.LookupAll(ctx, y.Client, anyTypePath.State())
	if err != nil {
		return nil, err
	}

	for _, value := range values {
		if got, ok := value.Val(); ok {
			if got != want {
				continue
			}
			name := value.Path.GetElem()[1].GetKey()["name"]
			names = append(names, name)
		}
	}

	if len(names) < 1 {
		return nil, fmt.Errorf("none of the %d components match %v", len(values), want)
	}
	return names, nil
}
