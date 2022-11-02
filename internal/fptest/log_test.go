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

	"github.com/openconfig/ygot/ygot"
	ci "github.com/openconfig/ondatra/config/interfaces"
	"github.com/openconfig/ondatra"
	ti "github.com/openconfig/ondatra/telemetry/interfaces"
)

func TestSanitizeFilename(t *testing.T) {
	kept := "ABC+,-.:;=^|~xyz()<>[]{}123"
	underscored := " /_"
	dropped := "!@#$%&*"
	arg := kept + underscored + dropped
	want := kept + "___"

	if got := sanitizeFilename(arg); got != want {
		t.Errorf("sanitizeFilename(%q) got %q, want %q", arg, got, want)
	}
}

func TestWriteOutput(t *testing.T) {
	if err := WriteOutput("TestWriteOutput", ".json", "{}"); err != nil {
		t.Errorf("writeOutput got error: %v", err)
	}
}

func TestIsConfig(t *testing.T) {
	cases := []struct {
		path ygot.PathStruct
		want bool
	}{
		{&ondatra.Config{}, true},
		{&ci.InterfacePath{}, true},
		{&ti.InterfacePath{}, false},
	}

	for i, c := range cases {
		if got := isConfig(c.path); got != c.want {
			t.Errorf("case[%d] got %v, want %v", i, got, c.want)
		}
	}
}
