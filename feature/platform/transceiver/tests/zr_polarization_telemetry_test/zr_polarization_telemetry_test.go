// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package zr_polarization_telemetry_test

import (
	"flag"
	"testing"

	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/telemetry/transceiver/polarization"
)

var (
	frequencyList          cfgplugins.FrequencyList
	targetOpticalPowerList cfgplugins.TargetOpticalPowerList
	operationalModeList    cfgplugins.OperationalModeList
)

func init() {
	flag.Var(&operationalModeList, "operational_mode", "operational-mode for the channel.")
	flag.Var(&frequencyList, "frequency", "frequency for the channel.")
	flag.Var(&targetOpticalPowerList, "target_optical_power", "target-optical-power for the channel.")
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestPolarizationTelemetry(t *testing.T) {
	polarization.PolarizationTelemetryTest(t, &polarization.TunableParameters{
		OperationalModeList:    operationalModeList,
		FrequencyList:          frequencyList,
		TargetOpticalPowerList: targetOpticalPowerList,
	})
}
