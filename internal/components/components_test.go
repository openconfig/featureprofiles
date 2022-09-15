// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package components

import (
	"regexp"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFindMatchingStrings(t *testing.T) {
	args := []string{
		"LineCard1",
		"Supervisor1",
		"LineCard2",
		"Supervisor2",
		"Supervisor3",
		"LineCard2",
	}
	r := regexp.MustCompile(`^Supervisor[0-9]$`)
	want := []string{
		"Supervisor1",
		"Supervisor2",
		"Supervisor3",
	}
	got := FindMatchingStrings(args, r)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("FindMatchingStrings(%s) returned unexpected diff (-want +got):\n%s", args, diff)
	}
}
