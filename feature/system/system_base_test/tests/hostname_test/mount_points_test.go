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

package mount_points_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/functional-translators/ftconsts"
	"github.com/openconfig/functional-translators/registrar"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ygnmi/ygnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestMountPoints(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ft, ok := registrar.FunctionalTranslatorRegistry[ftconsts.CiscoXRMountTranslator]
	if !ok {
		t.Fatalf("Functional translator %s is not registered", ftconsts.CiscoXRMountTranslator)
	}
	var opts []ygnmi.Option
	opts = append(opts, ygnmi.WithFT(ft))
	mountPoints := gnmi.LookupAll(t, dut.GNMIOpts().WithYGNMIOpts(opts...), gnmi.OC().System().MountPointAny().Name().State())
	for _, mp := range mountPoints {
		name, ok := mp.Val()
		if !ok {
			t.Fatal("Failed to get mount point name")
		}
		t.Logf("Mount Point: %s", name)

		if size, ok := gnmi.Lookup(t, dut.GNMIOpts().WithYGNMIOpts(opts...), gnmi.OC().System().MountPoint(name).Size().State()).Val(); !ok {
			t.Fatalf("Failed to get size for mount point %s", name)
		} else {
			t.Logf("Mount Point %s: Size: %v", name, size)
		}
		if used, ok := gnmi.Lookup(t, dut.GNMIOpts().WithYGNMIOpts(opts...), gnmi.OC().System().MountPoint(name).Utilized().State()).Val(); !ok {
			t.Fatalf("Failed to get utilized for mount point %s", name)
		} else {
			t.Logf("Mount Point %s: Utilized: %v", name, used)
		}
		if available, ok := gnmi.Lookup(t, dut.GNMIOpts().WithYGNMIOpts(opts...), gnmi.OC().System().MountPoint(name).Available().State()).Val(); !ok {
			t.Fatalf("Failed to get available for mount point %s", name)
		} else {
			t.Logf("Mount Point %s: Available: %v", name, available)
		}
	}
}
