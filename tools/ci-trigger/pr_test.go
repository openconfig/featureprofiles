// Copyright 2023 Google LLC
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

package main

import (
	"encoding/base64"
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	opb "github.com/openconfig/ondatra/proto"
)

func TestModifiedFunctionalTests(t *testing.T) {
	cases := []struct {
		desc            string
		functionalTests []string
		modifiedFiles   []string
		want            []string
	}{
		{
			desc:            "path test",
			functionalTests: []string{"/a/b"},
			modifiedFiles:   []string{"/a/b/c.txt"},
			want:            []string{"/a/b"},
		},
		{
			desc:            "nothing shared",
			functionalTests: []string{"/a", "/b", "/c"},
			modifiedFiles:   []string{"/d", "/e"},
			want:            nil,
		},
		{
			desc:            "everything shared",
			functionalTests: []string{"/a", "/b", "/c"},
			modifiedFiles:   []string{"/a", "/b/b", "/c/c/c"},
			want:            []string{"/a", "/b", "/c"},
		},
		{
			desc:            "some shared",
			functionalTests: []string{"/a", "/b", "/c"},
			modifiedFiles:   []string{"/d/d", "/b/b", "/e/e"},
			want:            []string{"/b"},
		},
		{
			desc:            "different size inputs",
			functionalTests: []string{"/a"},
			modifiedFiles:   []string{"/a/a", "/b/b", "/c/c"},
			want:            []string{"/a"},
		},
		{
			desc:            "empty inputs",
			functionalTests: nil,
			modifiedFiles:   nil,
			want:            nil,
		},
		{
			desc: "nil inputs",
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			got := modifiedFunctionalTests(tc.functionalTests, tc.modifiedFiles)
			if diff := cmp.Diff(tc.want, got, cmpopts.SortSlices(func(a, b string) bool { return a < b })); diff != "" {
				t.Errorf("modifiedFunctionalTests -want,+got\n:%s", diff)
			}
		})
	}
}

func TestFunctionalTestPaths(t *testing.T) {
	fs := fstest.MapFS{
		"README.md":                        {},
		"foo/metadata.textproto":           {},
		"feature/a/a/README.md":            {},
		"feature/a/b/metadata.textproto":   {},
		"feature/c/d/e/metadata.textproto": {},
	}
	want := []string{"feature/a/b", "feature/c/d/e"}
	got, err := functionalTestPaths(fs)
	if err != nil {
		t.Errorf("functionalTestPaths: got err %v, want nil", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("functionalTestPaths -want,+got\n:%s", diff)
	}
}

func TestPopulateTestDetail(t *testing.T) {
	in := pullRequest{
		ID:      100,
		HeadSHA: "1a2b3",
		localFS: fstest.MapFS{
			"feature/a/a/metadata.textproto":                                {Data: []byte("uuid: \"uuid-A\"\nplan_id: \"plan_id-A\"\ndescription: \"description-A\"\nunknown_field: true\n")},
			"feature/bgp/addpath/otg_tests/example_test/metadata.textproto": {Data: []byte("uuid: \"uuid-B\"\nplan_id: \"plan_id-B\"\ndescription: \"description-B\"\nunknown_field: true\n")},
			"feature/bgp/addpath/ate_tests/example_test/metadata.textproto": {Data: []byte("uuid: \"uuid-C\"\nplan_id: \"plan_id-C\"\ndescription: \"description-C\"\nunknown_field: true\n")},
		},
	}
	modifiedTests := []string{"feature/bgp/addpath/otg_tests/example_test", "feature/bgp/addpath/ate_tests/example_test"}
	want := pullRequest{
		ID:      100,
		HeadSHA: "1a2b3",
		Virtual: []device{
			{
				Type: deviceType{
					Vendor:        opb.Device_ARISTA,
					HardwareModel: "cEOS",
				},
				Tests: []functionalTest{
					{
						Name:        "plan_id-B",
						Description: "description-B",
						Path:        "feature/bgp/addpath/otg_tests/example_test",
						DocURL:      "https://github.com/" + githubProjectOwner + "/" + githubProjectRepo + "/blob/1a2b3/feature/bgp/addpath/otg_tests/example_test/README.md",
						TestURL:     "https://github.com/" + githubProjectOwner + "/" + githubProjectRepo + "/blob/1a2b3/feature/bgp/addpath/otg_tests/example_test",
						BadgePath:   gcpBucketPrefix + "/100/1a2b3/" + base64.RawURLEncoding.EncodeToString([]byte("feature/bgp/addpath/otg_tests/example_test")) + ".ARISTA_cEOS.svg",
						BadgeURL:    "https://storage.googleapis.com/" + gcpBucket + "/" + gcpBucketPrefix + "/100/1a2b3/" + base64.RawURLEncoding.EncodeToString([]byte("feature/bgp/addpath/otg_tests/example_test")) + ".ARISTA_cEOS.svg",
						Status:      "pending authorization",
					},
				},
			},
			{
				Type: deviceType{
					Vendor:        opb.Device_CISCO,
					HardwareModel: "8000E",
				},
				Tests: []functionalTest{
					{
						Name:        "plan_id-B",
						Description: "description-B",
						Path:        "feature/bgp/addpath/otg_tests/example_test",
						DocURL:      "https://github.com/" + githubProjectOwner + "/" + githubProjectRepo + "/blob/1a2b3/feature/bgp/addpath/otg_tests/example_test/README.md",
						TestURL:     "https://github.com/" + githubProjectOwner + "/" + githubProjectRepo + "/blob/1a2b3/feature/bgp/addpath/otg_tests/example_test",
						BadgePath:   gcpBucketPrefix + "/100/1a2b3/" + base64.RawURLEncoding.EncodeToString([]byte("feature/bgp/addpath/otg_tests/example_test")) + ".CISCO_8000E.svg",
						BadgeURL:    "https://storage.googleapis.com/" + gcpBucket + "/" + gcpBucketPrefix + "/100/1a2b3/" + base64.RawURLEncoding.EncodeToString([]byte("feature/bgp/addpath/otg_tests/example_test")) + ".CISCO_8000E.svg",
						Status:      "pending authorization",
					},
				},
			},
			{
				Type: deviceType{
					Vendor:        opb.Device_CISCO,
					HardwareModel: "XRd",
				},
				Tests: []functionalTest{
					{
						Name:        "plan_id-B",
						Description: "description-B",
						Path:        "feature/bgp/addpath/otg_tests/example_test",
						DocURL:      "https://github.com/" + githubProjectOwner + "/" + githubProjectRepo + "/blob/1a2b3/feature/bgp/addpath/otg_tests/example_test/README.md",
						TestURL:     "https://github.com/" + githubProjectOwner + "/" + githubProjectRepo + "/blob/1a2b3/feature/bgp/addpath/otg_tests/example_test",
						BadgePath:   gcpBucketPrefix + "/100/1a2b3/" + base64.RawURLEncoding.EncodeToString([]byte("feature/bgp/addpath/otg_tests/example_test")) + ".CISCO_XRd.svg",
						BadgeURL:    "https://storage.googleapis.com/" + gcpBucket + "/" + gcpBucketPrefix + "/100/1a2b3/" + base64.RawURLEncoding.EncodeToString([]byte("feature/bgp/addpath/otg_tests/example_test")) + ".CISCO_XRd.svg",
						Status:      "pending authorization",
					},
				},
			},
			{
				Type: deviceType{
					Vendor:        opb.Device_JUNIPER,
					HardwareModel: "cPTX",
				},
				Tests: []functionalTest{
					{
						Name:        "plan_id-B",
						Description: "description-B",
						Path:        "feature/bgp/addpath/otg_tests/example_test",
						DocURL:      "https://github.com/" + githubProjectOwner + "/" + githubProjectRepo + "/blob/1a2b3/feature/bgp/addpath/otg_tests/example_test/README.md",
						TestURL:     "https://github.com/" + githubProjectOwner + "/" + githubProjectRepo + "/blob/1a2b3/feature/bgp/addpath/otg_tests/example_test",
						BadgePath:   gcpBucketPrefix + "/100/1a2b3/" + base64.RawURLEncoding.EncodeToString([]byte("feature/bgp/addpath/otg_tests/example_test")) + ".JUNIPER_cPTX.svg",
						BadgeURL:    "https://storage.googleapis.com/" + gcpBucket + "/" + gcpBucketPrefix + "/100/1a2b3/" + base64.RawURLEncoding.EncodeToString([]byte("feature/bgp/addpath/otg_tests/example_test")) + ".JUNIPER_cPTX.svg",
						Status:      "pending authorization",
					},
				},
			},
			{
				Type: deviceType{
					Vendor:        opb.Device_NOKIA,
					HardwareModel: "SR Linux",
				},
				Tests: []functionalTest{
					{
						Name:        "plan_id-B",
						Description: "description-B",
						Path:        "feature/bgp/addpath/otg_tests/example_test",
						DocURL:      "https://github.com/" + githubProjectOwner + "/" + githubProjectRepo + "/blob/1a2b3/feature/bgp/addpath/otg_tests/example_test/README.md",
						TestURL:     "https://github.com/" + githubProjectOwner + "/" + githubProjectRepo + "/blob/1a2b3/feature/bgp/addpath/otg_tests/example_test",
						BadgePath:   gcpBucketPrefix + "/100/1a2b3/" + base64.RawURLEncoding.EncodeToString([]byte("feature/bgp/addpath/otg_tests/example_test")) + ".NOKIA_SR_Linux.svg",
						BadgeURL:    "https://storage.googleapis.com/" + gcpBucket + "/" + gcpBucketPrefix + "/100/1a2b3/" + base64.RawURLEncoding.EncodeToString([]byte("feature/bgp/addpath/otg_tests/example_test")) + ".NOKIA_SR_Linux.svg",
						Status:      "pending authorization",
					},
				},
			},
		},
	}

	err := in.populateTestDetail(modifiedTests)
	if err != nil {
		t.Fatalf("populateModifiedTests(%v): %v", modifiedTests, err)
	}
	if diff := cmp.Diff(want, in, cmpopts.IgnoreUnexported(pullRequest{})); diff != "" {
		t.Errorf("populateModifiedTests(%v): -want,+got:\n%s", modifiedTests, diff)
	}
}
