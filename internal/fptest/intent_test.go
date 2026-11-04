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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeIntentFile writes contents to a file named name under a new temporary
// directory and returns its path.
func writeIntentFile(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatalf("Failed to write intent file %q: %v", path, err)
	}
	return path
}

// makeIntentDir creates a directory named name under a new temporary directory
// and returns its path.
func makeIntentDir(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatalf("Failed to create intent directory %q: %v", path, err)
	}
	return path
}

const validIntent = `
release_id: "2026.1"
excluded_tests { test_id: "ACL-1.2" }
excluded_tests { test_id: "gNOI-4.1" }
`

func TestIsTestExcluded(t *testing.T) {
	intentPath := writeIntentFile(t, "release_intent.textproto", validIntent)

	tests := []struct {
		name string
		// intentPath defaults to a valid .textproto intent when empty.
		intentPath string
		planID     string
		want       bool
	}{
		{
			name:   "first_excluded_plan_id_is_skipped",
			planID: "ACL-1.2",
			want:   true,
		},
		{
			name:   "second_excluded_plan_id_is_skipped",
			planID: "gNOI-4.1",
			want:   true,
		},
		{
			name:   "non_excluded_plan_id_runs",
			planID: "TE-3.8",
			want:   false,
		},
		{
			// .txtpb is the canonical textproto extension, so the file name
			// must not decide whether the intent is usable.
			name:       "txtpb_extension_is_accepted",
			intentPath: writeIntentFile(t, "release_intent.txtpb", validIntent),
			planID:     "ACL-1.2",
			want:       true,
		},
		{
			// release_id is validated by the qualification-reporter pipeline,
			// not by the test runner, so filtering still works without it.
			name:       "intent_without_release_id_still_filters",
			intentPath: writeIntentFile(t, "no_release_id.textproto", `excluded_tests { test_id: "ACL-1.2" }`),
			planID:     "ACL-1.2",
			want:       true,
		},
		{
			// An empty excluded_tests list means that no tests are excluded.
			name:       "no_excluded_tests_runs_every_test",
			intentPath: writeIntentFile(t, "no_exclusions.textproto", `release_id: "2026.1"`),
			planID:     "ACL-1.2",
			want:       false,
		},
		{
			name:       "comment_only_file_runs_every_test",
			intentPath: writeIntentFile(t, "comment_only.textproto", "# release intent for 2026.1\n"),
			planID:     "ACL-1.2",
			want:       false,
		},
		{
			name:       "empty_file_runs_every_test",
			intentPath: writeIntentFile(t, "empty.textproto", ""),
			planID:     "ACL-1.2",
			want:       false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := tc.intentPath
			if path == "" {
				path = intentPath
			}
			got, err := isTestExcluded(path, tc.planID)
			if err != nil {
				t.Fatalf("isTestExcluded(%q, %q) returned unexpected error: %v", path, tc.planID, err)
			}
			if got != tc.want {
				t.Errorf("isTestExcluded(%q, %q) = %v, want %v", path, tc.planID, got, tc.want)
			}
		})
	}
}

func TestIsTestExcludedErrors(t *testing.T) {
	validPath := writeIntentFile(t, "release_intent.textproto", validIntent)

	tests := []struct {
		name string
		// intentPath is the --release_intent value under test.
		intentPath string
		planID     string
		// wantErrContains are substrings the error must contain so that the
		// message tells the user what is wrong and how to fix it.
		wantErrContains []string
	}{
		{
			name:            "missing_plan_id_names_the_metadata_to_fix",
			intentPath:      validPath,
			planID:          "",
			wantErrContains: []string{"no plan ID", "metadata.textproto", "plan_id"},
		},
		{
			name:            "nonexistent_file_reports_the_path",
			intentPath:      filepath.Join(t.TempDir(), "nonexistent.textproto"),
			planID:          "ACL-1.2",
			wantErrContains: []string{"cannot read", "nonexistent.textproto"},
		},
		{
			name:            "relative_path_suggests_an_absolute_path",
			intentPath:      "nonexistent.textproto",
			planID:          "ACL-1.2",
			wantErrContains: []string{"absolute path", "test's own directory"},
		},
		{
			name:            "directory_reports_that_a_file_is_required",
			intentPath:      makeIntentDir(t, "intent_dir.textproto"),
			planID:          "ACL-1.2",
			wantErrContains: []string{"cannot read", "intent_dir"},
		},
		{
			name:            "malformed_content_reports_a_parse_failure",
			intentPath:      writeIntentFile(t, "invalid.textproto", "invalid ::: proto ::: content"),
			planID:          "ACL-1.2",
			wantErrContains: []string{"cannot parse", "invalid.textproto"},
		},
		{
			name:            "empty_excluded_test_id_is_rejected",
			intentPath:      writeIntentFile(t, "empty_test_id.textproto", `excluded_tests { test_id: "ACL-1.2" } excluded_tests {}`),
			planID:          "TE-3.8",
			wantErrContains: []string{"empty excluded_tests[1].test_id", "empty_test_id.textproto"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := isTestExcluded(tc.intentPath, tc.planID)
			if err == nil {
				t.Fatalf("isTestExcluded(%q, %q) = %v, want error", tc.intentPath, tc.planID, got)
			}
			for _, want := range tc.wantErrContains {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("isTestExcluded(%q, %q) error = %q, want it to contain %q", tc.intentPath, tc.planID, err, want)
				}
			}
		})
	}
}
