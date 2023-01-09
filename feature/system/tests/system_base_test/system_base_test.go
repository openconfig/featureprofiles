/*
 Copyright 2022 Google LLC

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      https://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package system_base_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/rundata"
)

// init rundata is maintained by tools/addrundata.  DO NOT EDIT.
func init() {
	rundata.TestPlanID = "OC-1.1"
	rundata.TestDescription = "System Configuration"
	rundata.TestUUID = "a35c4cc3-805e-4681-973b-2ff06cf889de"
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
