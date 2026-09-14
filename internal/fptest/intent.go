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
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	log "github.com/golang/glog"
	"github.com/openconfig/featureprofiles/internal/metadata"
	"github.com/openconfig/featureprofiles/internal/pathutil"
	ripb "github.com/openconfig/featureprofiles/proto/release_intent_go_proto"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

var (
	intent = flag.String("intent", "", "Path to ReleaseIntent textproto file. If specified, tests whose plan ID is not in the intent are skipped.")
)

// skipOutput returns the Go test output reporting that the test was skipped.
//
// RunTests returns without ever calling m.Run when a test is not in the
// release intent, so the testing package never reports a result for it.  These
// lines reproduce what the testing package would have printed, so that a
// harness parsing the test output records a skipped result instead of an empty
// test run.  This is the Go test output protocol; TestSkipOutput pins it.
func skipOutput(planID string) string {
	return fmt.Sprintf("=== RUN   TestMain\n    Skipping test (plan ID %q): not included in execution intent\n--- SKIP: TestMain (0.00s)\nPASS\n", planID)
}

// clearPrematureExit removes the file a test harness uses to detect a test
// binary that exited before running its tests.  RunTests returns early by
// design when a test is not in the release intent, which would otherwise be
// reported as a premature exit rather than as the skip or error it is.
func clearPrematureExit() {
	f := os.Getenv("TEST_PREMATURE_EXIT_FILE")
	if f == "" {
		return
	}
	if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Warningf("Failed to remove premature exit file %q: %v", f, err)
	}
}

// shouldSkipForIntent reports whether the current test should be skipped
// because its plan ID is not included in the release intent given by --intent.
//
// Flags must already be parsed; every RunTests entry point guarantees this.
func shouldSkipForIntent() bool {
	if *intent == "" {
		return false
	}
	planID := metadata.Get().GetPlanId()
	intended, err := isTestIntended(*intent, planID)
	if err != nil {
		// Clear the premature exit marker first, so the harness reports this
		// error rather than the abrupt exit that log.Exitf is about to cause.
		clearPrematureExit()
		log.Exitf("Failed to evaluate intent flag %q: %v", *intent, err)
	}
	if !intended {
		log.Infof("Skipping test (plan ID %q): not included in execution intent %q", planID, *intent)
		fmt.Print(skipOutput(planID))
		clearPrematureExit()
		return true
	}
	return false
}

// isTestIntended reports whether the test with the given planID is intended to run
// according to the ReleaseIntent file specified by intentPath.
// If intentPath is empty, all tests are considered intended.
// If planID is empty, the test is considered unintended.
func isTestIntended(intentPath, planID string) (bool, error) {
	if intentPath == "" {
		return true, nil
	}
	if planID == "" {
		return false, nil
	}
	data, err := os.ReadFile(intentPath)
	if err != nil && !filepath.IsAbs(intentPath) {
		// Retry relative to the repository root, keeping the original error so
		// that a failure names the path the caller actually passed.
		if root, rootErr := pathutil.RootPath(); rootErr == nil {
			if rootData, rootReadErr := os.ReadFile(filepath.Join(root, intentPath)); rootReadErr == nil {
				data, err = rootData, nil
			}
		}
	}
	if err != nil {
		return false, fmt.Errorf("failed to read release intent file %q: %w", intentPath, err)
	}
	var relIntent ripb.ReleaseIntent
	if err := prototext.Unmarshal(data, &relIntent); err != nil {
		if pErr := proto.Unmarshal(data, &relIntent); pErr != nil {
			return false, fmt.Errorf("failed to parse release intent file %q: %w", intentPath, err)
		}
	}
	return slices.Contains(relIntent.GetIntendedTestIds(), planID), nil
}
