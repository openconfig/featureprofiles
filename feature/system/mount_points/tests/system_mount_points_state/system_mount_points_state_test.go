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

package system_mount_points_state

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/functional_translators/registrar"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

func getOptsForFunctionalTranslator(t *testing.T, dut *ondatra.DUTDevice, functionalTranslatorName string) []ygnmi.Option {
	if functionalTranslatorName == "" {
		return nil
	}
	ft, ok := registrar.FunctionalTranslatorRegistry[functionalTranslatorName]
	if !ok {
		t.Fatalf("Functional translator %s is not registered", functionalTranslatorName)
	}
	return []ygnmi.Option{ygnmi.WithFT(ft)}
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestSystemMountPointState(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	if deviations.MountPointStatePathsUnsupported(dut) {
		t.Skipf("Mount point state paths are unsupported and there is no functional translator")
	}
	opts := getOptsForFunctionalTranslator(t, dut, deviations.SystemMountPointStateFt(dut))
	mountPointNames := gnmi.GetAll(t, dut.GNMIOpts().WithYGNMIOpts(opts...), gnmi.OC().System().MountPointAny().Name().State())

	if len(mountPointNames) == 0 {
		t.Errorf("No mount points found when one or more are expected!")
	}

	for _, mountPointName := range mountPointNames {
		t.Run(mountPointName, func(t *testing.T) {
			if mountPointName == "" {
				t.Errorf("Mount point missing name")
			}
			size := gnmi.Get(t, dut.GNMIOpts().WithYGNMIOpts(opts...), gnmi.OC().System().MountPoint(mountPointName).Size().State())
			utilized := gnmi.Get(t, dut.GNMIOpts().WithYGNMIOpts(opts...), gnmi.OC().System().MountPoint(mountPointName).Utilized().State())
			if utilized > size {
				t.Errorf("Mount point %q has utilization %d greater than size %d", mountPointName, utilized, size)
			}
			available := gnmi.Get(t, dut.GNMIOpts().WithYGNMIOpts(opts...), gnmi.OC().System().MountPoint(mountPointName).Available().State())
			if available > size {
				t.Errorf("Mount point %q has available space %d greater than size %d", mountPointName, available, size)
			}
		})
	}
}
