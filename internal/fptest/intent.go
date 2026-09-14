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
	"github.com/openconfig/featureprofiles/internal/pathutil"
	ripb "github.com/openconfig/featureprofiles/proto/release_intent_go_proto"
	"github.com/openconfig/ondatra"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

var (
	releaseIntent = flag.String("release_intent", "", "Path to ReleaseIntent textproto file. If specified, tests whose plan ID is not in the release intent are skipped.")
)

// skipIfNotIntended checks the --release_intent flag and skips the current test
// if its plan ID is not included in the release intent.
func skipIfNotIntended() bool {
	planID := metadata.Get().GetPlanId()
	if *releaseIntent != "" {
		intended, err := isTestIntended(*releaseIntent, planID)
		if err != nil {
			log.Exitf("Failed to evaluate --release_intent flag %q: %v", *releaseIntent, err)
		}
		if !intended {
			log.Infof("Skipping test (plan ID %q): not included in release intent %q", planID, *releaseIntent)
			fmt.Printf("=== RUN   TestMain\n    Skipping test (plan ID %q): not included in release intent\n--- SKIP: TestMain (0.00s)\nPASS\n", planID)
			_ = os.Remove(os.Getenv("TEST_PREMATURE_EXIT_FILE"))
			return true
		}
	}
	if planID != "" {
		ondatra.Report().AddSuiteProperty("test.plan_id", planID)
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
