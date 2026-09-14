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
	"os"
	"path/filepath"
	"regexp"
	"testing"

	ripb "github.com/openconfig/featureprofiles/proto/release_intent_go_proto"
	"google.golang.org/protobuf/proto"
)

func TestIsTestIntended(t *testing.T) {
	tmpDir := t.TempDir()

	textprotoPath := filepath.Join(tmpDir, "release_intent.textproto")
	if err := os.WriteFile(textprotoPath, []byte(`
release_id: "2026.1"
intended_test_ids: "ACL-1.2"
intended_test_ids: "gNOI-4.1"
`), 0644); err != nil {
		t.Fatalf("Failed to write temporary textproto intent file: %v", err)
	}

	binaryProtoPath := filepath.Join(tmpDir, "release_intent.pb")
	relIntent := &ripb.ReleaseIntent{
		ReleaseId:       "2026.1",
		IntendedTestIds: []string{"TE-1.21", "TE-3.3"},
	}
	binaryData, err := proto.Marshal(relIntent)
	if err != nil {
		t.Fatalf("Failed to marshal binary ReleaseIntent: %v", err)
	}
	if err := os.WriteFile(binaryProtoPath, binaryData, 0644); err != nil {
		t.Fatalf("Failed to write temporary binary intent file: %v", err)
	}

	invalidProtoPath := filepath.Join(tmpDir, "invalid.textproto")
	if err := os.WriteFile(invalidProtoPath, []byte("invalid ::: proto ::: content"), 0644); err != nil {
		t.Fatalf("Failed to write invalid intent file: %v", err)
	}

	tests := []struct {
		name       string
		intentPath string
		planID     string
		want       bool
		wantErr    bool
	}{
		{
			name:       "empty intent allows all tests",
			intentPath: "",
			planID:     "ACL-1.2",
			want:       true,
			wantErr:    false,
		},
		{
			name:       "test plan ID in textproto intent",
			intentPath: textprotoPath,
			planID:     "ACL-1.2",
			want:       true,
			wantErr:    false,
		},
		{
			name:       "second test plan ID in textproto intent",
			intentPath: textprotoPath,
			planID:     "gNOI-4.1",
			want:       true,
			wantErr:    false,
		},
		{
			name:       "test plan ID not in textproto intent",
			intentPath: textprotoPath,
			planID:     "TE-3.8",
			want:       false,
			wantErr:    false,
		},
		{
			name:       "test plan ID in binary proto intent",
			intentPath: binaryProtoPath,
			planID:     "TE-1.21",
			want:       true,
			wantErr:    false,
		},
		{
			name:       "empty plan ID with intent set is not intended",
			intentPath: textprotoPath,
			planID:     "",
			want:       false,
			wantErr:    false,
		},
		{
			name:       "nonexistent intent file returns error",
			intentPath: filepath.Join(tmpDir, "nonexistent.textproto"),
			planID:     "ACL-1.2",
			want:       false,
			wantErr:    true,
		},
		{
			name:       "invalid intent file content returns error",
			intentPath: invalidProtoPath,
			planID:     "ACL-1.2",
			want:       false,
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := isTestIntended(tc.intentPath, tc.planID)
			if (err != nil) != tc.wantErr {
				t.Fatalf("isTestIntended(%q, %q) error = %v, wantErr %v", tc.intentPath, tc.planID, err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("isTestIntended(%q, %q) = %v, want %v", tc.intentPath, tc.planID, got, tc.want)
			}
		})
	}
}

// harnessRunLine and harnessResultLine mirror the patterns a Go test output
// parser uses to recognize the start and the result of a test case.  A skipped
// test is only recorded if skipOutput keeps matching them.
var (
	harnessRunLine    = regexp.MustCompile(`(?m)^=== RUN   (\S+)$`)
	harnessResultLine = regexp.MustCompile(`(?m)^--- (PASS|FAIL|SKIP): (\S+) \(([0-9.]+)(?: seconds|s)\)$`)
)

func TestSkipOutput(t *testing.T) {
	got := skipOutput("ACL-1.2")

	want := "=== RUN   TestMain\n" +
		"    Skipping test (plan ID \"ACL-1.2\"): not included in execution intent\n" +
		"--- SKIP: TestMain (0.00s)\n" +
		"PASS\n"
	if got != want {
		t.Errorf("skipOutput(\"ACL-1.2\") = %q, want %q", got, want)
	}

	// Pin the contract itself, not just the text, so that a reworded message
	// still fails loudly if it stops being parseable as a skipped test.
	run := harnessRunLine.FindStringSubmatch(got)
	if run == nil {
		t.Fatalf("skipOutput(\"ACL-1.2\") = %q, want a line matching %v", got, harnessRunLine)
	}
	result := harnessResultLine.FindStringSubmatch(got)
	if result == nil {
		t.Fatalf("skipOutput(\"ACL-1.2\") = %q, want a line matching %v", got, harnessResultLine)
	}
	if result[1] != "SKIP" {
		t.Errorf("skipOutput(\"ACL-1.2\") reported result %q, want %q", result[1], "SKIP")
	}
	if result[2] != run[1] {
		t.Errorf("skipOutput(\"ACL-1.2\") reported result for test %q, want %q to match the test that was started", result[2], run[1])
	}
}

func TestClearPrematureExit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.exited_prematurely")
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatalf("Failed to write premature exit file: %v", err)
	}
	t.Setenv("TEST_PREMATURE_EXIT_FILE", path)

	clearPrematureExit()

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("After clearPrematureExit(), os.Stat(%q) error = %v, want %v", path, err, os.ErrNotExist)
	}
}
