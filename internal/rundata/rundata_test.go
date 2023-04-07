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

package rundata

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/openconfig/ondatra/binding"
)

func TestTopology(t *testing.T) {
	cases := []struct {
		name string
		resv *binding.Reservation
		want string
	}{{
		name: "empty",
		resv: &binding.Reservation{},
		want: "",
	}, {
		name: "atedut_12",
		resv: &binding.Reservation{
			ATEs: map[string]binding.ATE{
				"ate": &binding.AbstractATE{
					Dims: &binding.Dims{
						Ports: map[string]*binding.Port{
							"port1":  nil,
							"port2":  nil,
							"port3":  nil,
							"port4":  nil,
							"port5":  nil,
							"port6":  nil,
							"port7":  nil,
							"port8":  nil,
							"port9":  nil,
							"port10": nil,
							"port11": nil,
							"port12": nil,
						},
					},
				},
			},
			DUTs: map[string]binding.DUT{
				"dut": &binding.AbstractDUT{
					Dims: &binding.Dims{
						Ports: map[string]*binding.Port{
							"port1":  nil,
							"port2":  nil,
							"port3":  nil,
							"port4":  nil,
							"port5":  nil,
							"port6":  nil,
							"port7":  nil,
							"port8":  nil,
							"port9":  nil,
							"port10": nil,
							"port11": nil,
							"port12": nil,
						},
					},
				},
			},
		},
		want: "ate:12,dut:12",
	}, {
		name: "dutdut",
		resv: &binding.Reservation{
			DUTs: map[string]binding.DUT{
				"dut1": &binding.AbstractDUT{
					Dims: &binding.Dims{
						Ports: map[string]*binding.Port{
							"port1": nil,
							"port2": nil,
							"port3": nil,
							"port4": nil,
						},
					},
				},
				"dut2": &binding.AbstractDUT{
					Dims: &binding.Dims{
						Ports: map[string]*binding.Port{
							"port1": nil,
							"port2": nil,
							"port3": nil,
							"port4": nil,
						},
					},
				},
			},
		},
		want: "dut1:4,dut2:4",
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := topology(c.resv)
			if got != c.want {
				t.Errorf("topology got %q, want %q", got, c.want)
			}
		})
	}
}

func TestProperties(t *testing.T) {
	const (
		wantUUID        = "TestProperties123"
		wantPlanID      = "TestProperties"
		wantDescription = "TestProperties unit test"
	)

	metadataText := fmt.Sprintf(`
uuid: "%s"
plan_id: "%s"
description: "%s"
`, wantUUID, wantPlanID, wantDescription)

	// Change to a temp directory before writing the metadata proto
	// to avoid modifying the test directory.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.Chdir(wd)
	}()
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("metadata.textproto", []byte(metadataText), 0644); err != nil {
		t.Fatal(err)
	}

	*knownIssueURL = "https://example.com"

	got := Properties(context.Background(), &binding.Reservation{})
	t.Log(got)

	for wantk, wantv := range map[string]string{
		"test.uuid":        wantUUID,
		"test.plan_id":     wantPlanID,
		"test.description": wantDescription,
		"known_issue_url":  *knownIssueURL,
	} {
		if gotv := got[wantk]; gotv != wantv {
			t.Errorf("Property %s got %q, want %q", wantk, gotv, wantv)
		}
	}

	for _, wantk := range []string{
		"test.path",
		"topology",
	} {
		if _, ok := got[wantk]; !ok {
			t.Errorf("Missing key from Properties: %s", wantk)
		}
	}
}

func TestTiming(t *testing.T) {
	got := Timing(context.Background())
	t.Log(got)

	for _, k := range []string{
		"time.begin",
		"time.end",
	} {
		if _, ok := got[k]; !ok {
			t.Errorf("Missing key from Timing: %s", k)
		}
	}
}
