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

package system_mount_points_state_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestSystemMountPointState(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	opts := fptest.GetOptsForFunctionalTranslator(t, deviations.SystemMountPointStateFt(dut))
	mountPointNames := gnmi.GetAll(t, dut.GNMIOpts().WithYGNMIOpts(opts...), gnmi.OC().System().MountPointAny().Name().State())

	if len(mountPointNames) == 0 {
		t.Fatalf("No mount points found when one or more are expected!")
	}

	for _, mountPointName := range mountPointNames {
		t.Run(mountPointName, func(t *testing.T) {
			if mountPointName == "" {
				t.Errorf("Mount point missing name")
			}
			size := gnmi.Get(t, dut.GNMIOpts().WithYGNMIOpts(opts...), gnmi.OC().System().MountPoint(mountPointName).Size().State())
			// NOTE: We need not check size < 0 since no value of type uint64 is less than 0
			utilized := gnmi.Get(t, dut.GNMIOpts().WithYGNMIOpts(opts...), gnmi.OC().System().MountPoint(mountPointName).Utilized().State())
			if utilized > size {
				t.Errorf("Mount point %q has utilization %d greater than size %d", mountPointName, utilized, size)
			}
			available := gnmi.Get(t, dut.GNMIOpts().WithYGNMIOpts(opts...), gnmi.OC().System().MountPoint(mountPointName).Available().State())
			if available > size {
				t.Errorf("Mount point %q has available space %d greater than size %d", mountPointName, available, size)
			}
			if utilized+available > size {
				t.Errorf("Mount point %q has size %d less than the total of utilized %d and available space %d", mountPointName, size, utilized, available)
			}
		})
	}
}
