// Copyright 2026 Google LLC
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
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	log "github.com/golang/glog"
	"github.com/openconfig/featureprofiles/internal/metadata"
	"github.com/openconfig/ondatra"
	ripb "github.com/openconfig/qualification-reporter/proto"
	"google.golang.org/protobuf/encoding/prototext"
)

var releaseIntent = flag.String("release_intent", "", "Path to the ReleaseIntent textproto file listing the test plan IDs to qualify. If set, tests whose plan ID is not listed in intended_test_ids are skipped. Prefer an absolute path: a relative path is resolved against the test's own directory.")

// skipIfNotIntended reports whether the current test should be skipped because
// its plan ID is not part of the release intent given by --release_intent.
// It exits the test binary if --release_intent is set but cannot be evaluated,
// so that an unusable intent file fails the run loudly instead of silently
// skipping every test.
//
// As a side effect, it publishes the plan ID as a suite property when the test
// does run. This must happen before ondatra.RunTests so that the JSON-Lines
// ledger records the plan ID rather than falling back to the binary name.
func skipIfNotIntended() bool {
	// RunTests calls this even when initMetadata failed, and initMetadata
	// returns before its own flag.Parse when metadata is missing. Parse here so
	// that a bad metadata file cannot silently disable the intent filter.
	if !flag.Parsed() {
		flag.Parse()
	}
	var planID string
	if md := metadata.Get(); md != nil {
		planID = md.GetPlanId()
	}
	if *releaseIntent != "" {
		intended, err := isTestIntended(*releaseIntent, planID)
		if err != nil {
			log.Exitf("Invalid --release_intent: %v", err)
		}
		if !intended {
			log.Infof("Skipping test (plan ID %q): not listed in intended_test_ids of release intent %q", planID, *releaseIntent)
			// Emit go test formatted output so that test runners parsing the
			// binary's stdout record the test as skipped rather than as having
			// produced no results at all.
			fmt.Printf("=== RUN   TestMain\n    Skipping test (plan ID %q): not included in release intent\n--- SKIP: TestMain (0.00s)\nPASS\n", planID)
			return true
		}
	}
	if planID != "" {
		ondatra.Report().AddSuiteProperty("test.plan_id", planID)
	}
	return false
}

// isTestIntended reports whether the test with the given planID is listed in
// the ReleaseIntent textproto at intentPath. It returns an error describing
// what to fix if planID is unknown or intentPath is not a usable ReleaseIntent.
func isTestIntended(intentPath, planID string) (bool, error) {
	if planID == "" {
		return false, fmt.Errorf("cannot match the test against release intent %q because the test has no plan ID; check that the test's metadata.textproto exists and sets plan_id", intentPath)
	}
	relIntent, err := readReleaseIntent(intentPath)
	if err != nil {
		return false, err
	}
	return slices.Contains(relIntent.GetIntendedTestIds(), planID), nil
}

// readReleaseIntent reads and parses the ReleaseIntent textproto at intentPath.
func readReleaseIntent(intentPath string) (*ripb.ReleaseIntent, error) {
	data, err := os.ReadFile(intentPath)
	if err != nil {
		if !filepath.IsAbs(intentPath) {
			// Each test binary runs in its own package directory, so a relative
			// path is resolved against that directory and is rarely what the
			// caller meant.
			return nil, fmt.Errorf("cannot read release intent file %q: %w; pass an absolute path because a relative path is resolved against the test's own directory", intentPath, err)
		}
		return nil, fmt.Errorf("cannot read release intent file %q: %w", intentPath, err)
	}
	relIntent := &ripb.ReleaseIntent{}
	if err := prototext.Unmarshal(data, relIntent); err != nil {
		return nil, fmt.Errorf("cannot parse release intent file %q as a ReleaseIntent textproto: %w", intentPath, err)
	}
	// An intent that parses but lists no tests would skip every test and report
	// a green run, so reject it here. Other fields are left to the
	// qualification-reporter pipeline, which owns intent policy.
	if len(relIntent.GetIntendedTestIds()) == 0 {
		return nil, fmt.Errorf("release intent file %q lists no intended_test_ids, which would skip every test; list the plan IDs to qualify", intentPath)
	}
	return relIntent, nil
}
