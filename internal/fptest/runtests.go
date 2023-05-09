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

package fptest

import (
	"testing"

	log "github.com/golang/glog"
	"github.com/openconfig/featureprofiles/internal/metadata"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
)

// RunTests initializes the appropriate binding and runs the tests.
// It should be called from every featureprofiles tests like this:
//
//	package test
//
//	import "github.com/openconfig/featureprofiles/internal/fptest"
//
//	func TestMain(m *testing.M) {
//	  fptest.RunTests(m)
//	}
func RunTests(m *testing.M) {
	if err := metadata.Init(); err != nil {
		log.Errorf("Unable to initialize test metadata: %v", err)
	}
	ondatra.RunTests(m, binding.New)
}
