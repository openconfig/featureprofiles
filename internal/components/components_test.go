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
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/ondatra/gnmi/oc"
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

func TestWaitUntil(t *testing.T) {
	tests := []struct {
		desc    string
		offset  time.Duration
		wantMin time.Duration
		wantMax time.Duration
	}{
		{desc: "deadline passed", offset: -time.Minute, wantMin: minPerComponentWait, wantMax: minPerComponentWait},
		{desc: "deadline sooner than minimum", offset: 10 * time.Second, wantMin: minPerComponentWait, wantMax: minPerComponentWait},
		{desc: "deadline later than minimum", offset: 10 * time.Minute, wantMin: 9 * time.Minute, wantMax: 10 * time.Minute},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got := waitUntil(time.Now().Add(tc.offset))
			if got < tc.wantMin || got > tc.wantMax {
				t.Errorf("waitUntil(now%+v) = %v, want between %v and %v", tc.offset, got, tc.wantMin, tc.wantMax)
			}
		})
	}
}

func TestHardwarePortCandidates(t *testing.T) {
	tests := []struct {
		name string
		want []string
	}{
		{name: "Ethernet3/1", want: []string{"Ethernet3/1"}},
		{name: "et-1/0/2:2", want: []string{"et-1/0/2:2", "et-1/0/2"}},
		{name: ":2", want: []string{":2"}},
	}
	for _, tc := range tests {
		if diff := cmp.Diff(tc.want, hardwarePortCandidates(tc.name)); diff != "" {
			t.Errorf("hardwarePortCandidates(%q) returned unexpected diff (-want +got):\n%s", tc.name, diff)
		}
	}
}

func TestLinecardAncestor(t *testing.T) {
	const (
		linecard = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD
		port     = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_PORT
		chip     = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_INTEGRATED_CIRCUIT
		chassis  = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS
	)
	tests := []struct {
		desc    string
		comp    string
		types   map[string]oc.Component_Type_Union
		parents map[string]string
		want    string
		wantErr bool
	}{{
		desc:    "parent chain",
		comp:    "Port3/1",
		types:   map[string]oc.Component_Type_Union{"Port3/1": port, "Chip3": chip, "Linecard3": linecard, "Chassis": chassis},
		parents: map[string]string{"Port3/1": "Chip3", "Chip3": "Linecard3", "Linecard3": "Chassis"},
		want:    "Linecard3",
	}, {
		desc:    "colon prefix fallback when parent is missing",
		comp:    "FPC0:PIC0:PORT1",
		types:   map[string]oc.Component_Type_Union{"FPC0:PIC0:PORT1": port, "FPC0:PIC0:NPU0": chip, "FPC0:PIC0": chip, "FPC0": linecard},
		parents: map[string]string{"FPC0:PIC0:PORT1": "FPC0:PIC0:NPU0", "FPC0:PIC0": "FPC0"},
		want:    "FPC0",
	}, {
		desc:    "no linecard",
		comp:    "Port1",
		types:   map[string]oc.Component_Type_Union{"Port1": port, "Chassis": chassis},
		parents: map[string]string{"Port1": "Chassis"},
		wantErr: true,
	}, {
		desc:    "loop",
		comp:    "A",
		types:   map[string]oc.Component_Type_Union{"A": port, "B": chip},
		parents: map[string]string{"A": "B", "B": "A"},
		wantErr: true,
	}}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got, err := linecardAncestor(tc.comp, tc.types, tc.parents)
			if (err != nil) != tc.wantErr {
				t.Fatalf("linecardAncestor(%q) error = %v, wantErr %v", tc.comp, err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("linecardAncestor(%q) = %q, want %q", tc.comp, got, tc.want)
			}
		})
	}
}
