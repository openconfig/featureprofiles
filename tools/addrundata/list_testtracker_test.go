package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	mpb "github.com/openconfig/featureprofiles/proto/metadata_go_proto"
	"google.golang.org/protobuf/testing/protocmp"
)

var testsuiteForTT = testsuite{
	"feature/foo/bar/ate_tests/qux_test": &testcase{
		existing: &mpb.Metadata{
			Uuid:        "c857db98-7b2c-433c-b9fb-4511b42edd78",
			PlanId:      "YY-2.1",
			Description: "Qux Functional Test",
		},
	},
	"feature/foo/bar/otg_tests/qux_test": &testcase{
		existing: &mpb.Metadata{
			Uuid:        "c857db98-7b2c-433c-b9fb-4511b42edd78",
			PlanId:      "YY-2.1",
			Description: "Qux Functional Test",
		},
	},
	"feature/foo/baz/quuz_test": &testcase{
		existing: &mpb.Metadata{
			Uuid:        "a5413d74-5b44-49d2-b4e7-84c9751d50be",
			PlanId:      "XX-1.1",
			Description: "Quuz Functional Test",
		},
	},
}

func TestListTestTracker(t *testing.T) {
	var buf bytes.Buffer
	if err := listTestTracker(&buf, "", "", testsuiteForTT); err != nil {
		t.Fatal("Could not write TestTracker:", err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal("Could not unmarshal the generated JSON:", err)
	}
}

func TestListTestTracker_Merge(t *testing.T) {
	want := map[string]any{
		"text": "Base Test Plan",
		"type": "testplan",
	}
	wantdata, err := json.Marshal(want)
	if err != nil {
		t.Fatal("Could not marshal want:", err)
	}

	tempdir := t.TempDir()
	mergejson := filepath.Join(tempdir, "merge.json")
	if err := os.WriteFile(mergejson, wantdata, 0600); err != nil {
		t.Fatal("Could not write mergejson:", err)
	}

	var buf bytes.Buffer
	if err := listTestTracker(&buf, mergejson, "", testsuiteForTT); err != nil {
		t.Fatal("Could not write JSON:", err)
	}

	var merged map[string]any
	if err := json.Unmarshal(buf.Bytes(), &merged); err != nil {
		t.Fatal("Could not unmarshal the generated JSON:", err)
	}

	got := map[string]any{
		"text": merged["text"],
		"type": merged["type"],
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Merged JSON -want,+got:\n%s", diff)
	}
}

var jsopts = []cmp.Option{cmp.AllowUnexported(ttCase{}), protocmp.Transform()}

func TestTTBuildPlan(t *testing.T) {
	got, ok := ttBuildPlan(testsuiteForTT, "")
	if !ok {
		t.Fatal("Could not build ttPlan.")
	}

	want := ttPlan{
		"XX-1": ttSuite{
			"a5413d74-5b44-49d2-b4e7-84c9751d50be": &ttCase{
				metadata: testsuiteForTT["feature/foo/baz/quuz_test"].existing,
				testDirs: map[string]string{"": "feature/foo/baz/quuz_test"},
			},
		},
		"YY-2": ttSuite{
			"c857db98-7b2c-433c-b9fb-4511b42edd78": &ttCase{
				metadata: testsuiteForTT["feature/foo/bar/ate_tests/qux_test"].existing,
				testDirs: map[string]string{
					"ate_tests": "feature/foo/bar/ate_tests/qux_test",
					"otg_tests": "feature/foo/bar/otg_tests/qux_test",
				},
			},
		},
	}

	if diff := cmp.Diff(want, got, jsopts...); diff != "" {
		t.Errorf("ttBuildPlan -want,+got:\n%s", diff)
	}
}

func TestTTBuildPlan_MissingData(t *testing.T) {
	ts := testsuite{"feature/xyzzy/tests/quuz_test": &testcase{}}
	if _, ok := ttBuildPlan(ts, ""); ok {
		t.Errorf("ttBuildPlan ok got %v, want %v", ok, false)
	}
}

func TestTTBuildPlan_DisallowReuseUUID(t *testing.T) {
	ts := testsuite{
		"feature/foo/bar/ate_tests/qux_test": &testcase{
			existing: &mpb.Metadata{
				Uuid:        "c857db98-7b2c-433c-b9fb-4511b42edd78",
				PlanId:      "YY-2.1",
				Description: "Qux Functional Test",
			},
		},
		"feature/foo/baz/quuz_test": &testcase{
			existing: &mpb.Metadata{
				Uuid:        "c857db98-7b2c-433c-b9fb-4511b42edd78",
				PlanId:      "XX-1.1",
				Description: "Quuz Functional Test",
			},
		},
	}
	if _, ok := ttBuildPlan(ts, ""); ok {
		t.Errorf("ttBuildPlan ok got %v, want %v", ok, false)
	}
}

func TestSection(t *testing.T) {
	cases := []struct {
		id   string
		want string
	}{
		{"RT-1.2", "RT-1"},
		{"Foo", "Foo"},
	}
	for _, c := range cases {
		if got := testSection(c.id); got != c.want {
			t.Errorf("testSection(%q) got %q, want %q", c.id, got, c.want)
		}
	}
}

func TestJSONQuote(t *testing.T) {
	cases := []struct {
		s    string
		want string
	}{
		{"apple\nbanana\ncherry\n", `"apple\nbanana\ncherry\n"`},
		{"Tom & Jerry", `"Tom \u0026 Jerry"`},
	}
	for _, c := range cases {
		if got := jsonQuote(c.s); got != c.want {
			t.Errorf("jsonQuote(%q) got %q, want %q", c.s, got, c.want)
		}
	}
}

func TestTTPlan_SortedKeys(t *testing.T) {
	ttp := ttPlan{
		"YY-10": ttSuite{},
		"YY-1a": ttSuite{},
		"YY-2":  ttSuite{},
		"XX-10": ttSuite{},
		"XX-1a": ttSuite{},
		"XX-2":  ttSuite{},
	}
	want := []string{"XX-1a", "XX-2", "XX-10", "YY-1a", "YY-2", "YY-10"}
	got := ttp.sortedKeys()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ttPlan.sortedKeys() -want,+got:\n%s", diff)
	}
}

func setDefaultAttrs(attrs map[string]any) map[string]any {
	for k, v := range defaultCaseAttrs {
		attrs[k] = v
	}
	return attrs
}

func TestTTPlan_Merge(t *testing.T) {
	got := map[string]any{
		"text": "Plan To Be Merged",
		"type": "testplan",
		"children": []any{
			// Passthrough mal-formed testsuites.
			42,
			map[string]any{
				"text": "Not Bracketed",
			},
			// Passthrough JSON-only testsuites.
			map[string]any{
				"text": "[ZZ-1] JSON Only",
			},
			// Existing testsuites to be updated where ordering is preserved.
			map[string]any{
				"text": "[YY-1] Needs Update",
			},
			map[string]any{
				"text": "[XX-1] Needs Update",
			},
			// New testsuites will be added here.
		},
	}

	ttp := ttPlan{
		"XX-1": ttSuite{
			"0eac5b62-ab22-449d-9a9a-255b05572641": &ttCase{
				metadata: &mpb.Metadata{
					Uuid:        "0eac5b62-ab22-449d-9a9a-255b05572641",
					PlanId:      "XX-1.1",
					Description: "Foo",
				},
			},
		},
		"XX-2": ttSuite{
			"f842057d-0100-4198-a18d-593b2bf3610e": &ttCase{
				metadata: &mpb.Metadata{
					Uuid:        "f842057d-0100-4198-a18d-593b2bf3610e",
					PlanId:      "XX-2.1",
					Description: "Bar",
				},
			},
		},
		"YY-1": ttSuite{
			"12cd2de3-69af-4aa6-a3d6-a2d5fbdb86c6": &ttCase{
				metadata: &mpb.Metadata{
					Uuid:        "12cd2de3-69af-4aa6-a3d6-a2d5fbdb86c6",
					PlanId:      "YY-1.1",
					Description: "Xyzzy",
				},
			},
		},
	}

	want := map[string]any{
		"text": "Plan To Be Merged",
		"type": "testplan",
		"children": []any{
			// Passthrough mal-formed testsuites.
			42,
			map[string]any{
				"text": "Not Bracketed",
			},
			// Passthrough JSON-only testsuites.
			map[string]any{
				"text": "[ZZ-1] JSON Only",
			},
			// Existing testsuites to be updated where ordering is preserved.
			map[string]any{
				"text": "[YY-1] Needs Update",
				"children": []any{
					map[string]any{
						"text": "YY-1.1: Xyzzy",
						"type": "testcases",
						"li_attr": setDefaultAttrs(map[string]any{
							"rel":         "testcases",
							"title":       "YY-1.1: Xyzzy",
							"uuid":        "12cd2de3-69af-4aa6-a3d6-a2d5fbdb86c6",
							"description": `""`,
						}),
					},
				},
			},
			map[string]any{
				"text": "[XX-1] Needs Update",
				"children": []any{
					map[string]any{
						"text": "XX-1.1: Foo",
						"type": "testcases",
						"li_attr": setDefaultAttrs(map[string]any{
							"rel":         "testcases",
							"title":       "XX-1.1: Foo",
							"uuid":        "0eac5b62-ab22-449d-9a9a-255b05572641",
							"description": `""`,
						}),
					},
				},
			},
			// New testsuites will be added here.
			map[string]any{
				"text": "[XX-2]",
				"type": "testsuites",
				"li_attr": map[string]any{
					"rel":         "testsuites",
					"title":       "[XX-2]",
					"description": `""`,
					"tags":        "",
				},
				"children": []any{
					map[string]any{
						"text": "XX-2.1: Bar",
						"type": "testcases",
						"li_attr": setDefaultAttrs(map[string]any{
							"rel":         "testcases",
							"title":       "XX-2.1: Bar",
							"uuid":        "f842057d-0100-4198-a18d-593b2bf3610e",
							"description": `""`,
						}),
					},
				},
			},
		},
	}

	ttp.merge(got)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ttPlan.merge() -want,+got:\n%s", diff)
	}
}

func TestChildSuite(t *testing.T) {
	cases := map[string]struct {
		child   any
		wantsec string
		wantok  bool
	}{
		"NotMap": {
			child:  42,
			wantok: false,
		},
		"NoText": {
			child:  map[string]any{},
			wantok: false,
		},
		"NoMatch": {
			child:  map[string]any{"text": "xyzzy"},
			wantok: false,
		},
		"Match": {
			child:   map[string]any{"text": "[XX-1] Xyzzy"},
			wantsec: "XX-1",
			wantok:  true,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, gotsec, gotok := childSuite(c.child)
			if gotsec != c.wantsec {
				t.Errorf("Result of sec got %q, want %q", gotsec, c.wantsec)
			}
			if gotok != c.wantok {
				t.Errorf("Result of ok got %v, want %v", gotok, c.wantok)
			}
		})
	}
}

func TestTTSuite_SortedKeys(t *testing.T) {
	tts := ttSuite{
		"864550d6-e843-4301-846a-a1998a23bb5a": &ttCase{
			metadata: &mpb.Metadata{PlanId: "XX-1.10"},
		},
		"e9345234-fc59-44f3-9d21-00b57137fb40": &ttCase{
			metadata: &mpb.Metadata{PlanId: "XX-1.1a"},
		},
		"bc261bca-d50f-42db-80f9-7c955c4e3889": &ttCase{
			metadata: &mpb.Metadata{PlanId: "XX-1.2"},
		},
	}
	want := []string{
		"e9345234-fc59-44f3-9d21-00b57137fb40", // XX-1.1a
		"bc261bca-d50f-42db-80f9-7c955c4e3889", // XX-1.2
		"864550d6-e843-4301-846a-a1998a23bb5a", // XX-1.10
	}
	got := tts.sortedKeys()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ttSuite.sortedKeys() -want,+got:\n%s", diff)
	}
}

func TestTTSuite_Merge(t *testing.T) {
	got := map[string]any{
		"text": "Suite To Be Merged",
		"type": "testsuites",
		"children": []any{
			// Passthrough mal-formed testcases.
			42,
			map[string]any{
				"text": "XX-1.10: Missing li_attr",
			},
			map[string]any{
				"text":    "XX-1.11: Missing UUID",
				"li_attr": map[string]any{},
			},
			map[string]any{
				"text": "XX-1.12: Invalid UUID",
				"li_attr": map[string]any{
					"uuid": "Invalid UUID",
				},
			},
			// Passthrough JSON-only testcases.
			map[string]any{
				"text": "XX-1.20: JSON Only",
				"type": "testcases",
				"li_attr": map[string]any{
					"title": "XX-1.20: JSON Only",
				},
			},
			// Existing testcases to be updated where ordering is preserved.
			map[string]any{
				"text": "XX-1.3: Needs Update",
				"li_attr": map[string]any{
					"title": "XX-1.3: Needs Update",
					"uuid":  "f6fbad49-ede3-4d47-b58b-8a8005e4b598", // To be updated.
				},
			},
			map[string]any{
				"text": "No Title", // To become XX-1.2
				"li_attr": map[string]any{
					"uuid": "755ae14f-7d1a-465a-8cfb-f1674ea68763",
				},
			},
			map[string]any{
				"text": "Title Has No ID", // To become XX-1.1
				"li_attr": map[string]any{
					"title": "Title Has No ID",
					"uuid":  "d2d462b4-db36-4159-9152-744dc6168ba8",
				},
			},
			// New testcases will be added here.
		},
	}

	tts := ttSuite{
		"d2d462b4-db36-4159-9152-744dc6168ba8": &ttCase{
			metadata: &mpb.Metadata{
				Uuid:        "d2d462b4-db36-4159-9152-744dc6168ba8",
				PlanId:      "XX-1.1",
				Description: "Apple",
			},
		},
		"755ae14f-7d1a-465a-8cfb-f1674ea68763": &ttCase{
			metadata: &mpb.Metadata{
				Uuid:        "755ae14f-7d1a-465a-8cfb-f1674ea68763",
				PlanId:      "XX-1.2",
				Description: "Banana",
			},
		},
		"c2cb54c0-2acc-4fd8-8c2a-7f9ccb9ea192": &ttCase{
			metadata: &mpb.Metadata{
				Uuid:        "c2cb54c0-2acc-4fd8-8c2a-7f9ccb9ea192",
				PlanId:      "XX-1.3",
				Description: "Cherry",
			},
		},
		"f7372990-dfb2-4a8f-acfb-c7a31b29522c": &ttCase{
			metadata: &mpb.Metadata{
				Uuid:        "f7372990-dfb2-4a8f-acfb-c7a31b29522c",
				PlanId:      "XX-1.4",
				Description: "Durian",
			},
		},
	}

	want := map[string]any{
		"text": "Suite To Be Merged",
		"type": "testsuites",
		"children": []any{
			// Passthrough mal-formed testcases.
			42,
			map[string]any{
				"text": "XX-1.10: Missing li_attr",
			},
			map[string]any{
				"text":    "XX-1.11: Missing UUID",
				"li_attr": map[string]any{},
			},
			map[string]any{
				"text": "XX-1.12: Invalid UUID",
				"li_attr": map[string]any{
					"uuid": "Invalid UUID",
				},
			},
			// Passthrough JSON-only testcases.
			map[string]any{
				"text": "XX-1.20: JSON Only",
				"type": "testcases",
				"li_attr": map[string]any{
					"title": "XX-1.20: JSON Only",
				},
			},
			// Existing testcases to be updated where ordering is preserved.
			map[string]any{
				"type": "testcases",
				"text": "XX-1.3: Cherry", // Updated.
				"li_attr": setDefaultAttrs(map[string]any{
					"rel":         "testcases",
					"title":       "XX-1.3: Cherry",                       // Updated.
					"uuid":        "c2cb54c0-2acc-4fd8-8c2a-7f9ccb9ea192", // Updated.
					"description": `""`,
				}),
			},
			map[string]any{
				"type": "testcases",
				"text": "XX-1.2: Banana", // Updated.
				"li_attr": setDefaultAttrs(map[string]any{
					"rel":         "testcases",
					"title":       "XX-1.2: Banana",
					"uuid":        "755ae14f-7d1a-465a-8cfb-f1674ea68763",
					"description": `""`,
				}),
			},
			map[string]any{
				"type": "testcases",
				"text": "XX-1.1: Apple", // Updated.
				"li_attr": setDefaultAttrs(map[string]any{
					"rel":         "testcases",
					"title":       "XX-1.1: Apple",
					"uuid":        "d2d462b4-db36-4159-9152-744dc6168ba8",
					"description": `""`,
				}),
			},
			// New testcases will be added here.
			map[string]any{
				"type": "testcases",
				"text": "XX-1.4: Durian",
				"li_attr": setDefaultAttrs(map[string]any{
					"rel":         "testcases",
					"title":       "XX-1.4: Durian",
					"uuid":        "f7372990-dfb2-4a8f-acfb-c7a31b29522c",
					"description": `""`,
				}),
			},
		},
	}

	tts.merge(got)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ttSuite.merge() -want,+got:\n%s", diff)
	}
}

var jkopt = cmp.AllowUnexported(ttCaseKey{})

func TestChildCase(t *testing.T) {
	cases := map[string]struct {
		child  any
		wantok bool
		want   ttCaseKey
	}{
		"NotMap": {
			child:  42,
			wantok: false,
		},
		"NoAttrs": {
			child:  map[string]any{},
			wantok: false,
		},
		"NoUUID": {
			child: map[string]any{
				"li_attr": map[string]any{},
			},
			wantok: false,
		},
		"BadUUID_NotString": {
			child: map[string]any{
				"li_attr": map[string]any{
					"uuid": 42,
				},
			},
			wantok: false,
		},
		"BadUUID_BadString": {
			child: map[string]any{
				"li_attr": map[string]any{
					"uuid": "xyzzy",
				},
			},
			wantok: false,
		},
		"NormalizeUUID": {
			child: map[string]any{
				"li_attr": map[string]any{
					"uuid": "{173e8a50-6040-4788-bfd2-ee62b2cf95c8}",
				},
			},
			wantok: true,
			want: ttCaseKey{
				testUUID: "173e8a50-6040-4788-bfd2-ee62b2cf95c8",
			},
		},
		"NoTitle": {
			child: map[string]any{
				"li_attr": map[string]any{
					"uuid": "173e8a50-6040-4788-bfd2-ee62b2cf95c8",
				},
			},
			wantok: true,
			want: ttCaseKey{
				testUUID: "173e8a50-6040-4788-bfd2-ee62b2cf95c8",
			},
		},
		"BadTitle_NotString": {
			child: map[string]any{
				"li_attr": map[string]any{
					"uuid":  "173e8a50-6040-4788-bfd2-ee62b2cf95c8",
					"title": 42,
				},
			},
			wantok: true,
			want: ttCaseKey{
				testUUID: "173e8a50-6040-4788-bfd2-ee62b2cf95c8",
			},
		},
		"BadTitle_BadString": {
			child: map[string]any{
				"li_attr": map[string]any{
					"uuid":  "173e8a50-6040-4788-bfd2-ee62b2cf95c8",
					"title": "xyzzy",
				},
			},
			wantok: true,
			want: ttCaseKey{
				testUUID: "173e8a50-6040-4788-bfd2-ee62b2cf95c8",
			},
		},
		"All": {
			child: map[string]any{
				"li_attr": map[string]any{
					"uuid":  "173e8a50-6040-4788-bfd2-ee62b2cf95c8",
					"title": "XX-1.1: Foo Functional Test",
				},
			},
			wantok: true,
			want: ttCaseKey{
				testPlanID: "XX-1.1",
				testUUID:   "173e8a50-6040-4788-bfd2-ee62b2cf95c8",
			},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, gotok := childCase(c.child)
			if gotok != c.wantok {
				t.Errorf("Result of ok got %v, want %v", gotok, c.wantok)
			}
			if !gotok {
				return
			}
			c.want.o = c.child.(map[string]any)
			if diff := cmp.Diff(c.want, got, jkopt); diff != "" {
				t.Errorf("childCase ttCaseKey -want,+got:\n%s", diff)
			}
		})
	}
}

func TestTTDesc(t *testing.T) {
	testDirs := map[string]string{
		"":              "feature/experimental/foo/empty_tests/foo_test",
		"ate_tests":     "feature/experimental/foo/ate_tests/foo_test",
		"kne_tests":     "feature/experimental/foo/kne_tests/foo_test",
		"otg_tests":     "feature/experimental/foo/otg_tests/foo_test",
		"tests":         "feature/experimental/foo/tests/foo_test",
		"unknown_tests": "feature/experimental/foo/unknown_tests/foo_test",
	}
	want := `See code location:
  - Test: https://github.com/openconfig/featureprofiles/tree/main/feature/experimental/foo/empty_tests/foo_test
  - ATE Test: https://github.com/openconfig/featureprofiles/tree/main/feature/experimental/foo/ate_tests/foo_test
  - KNE Test: https://github.com/openconfig/featureprofiles/tree/main/feature/experimental/foo/kne_tests/foo_test
  - OTG Test: https://github.com/openconfig/featureprofiles/tree/main/feature/experimental/foo/otg_tests/foo_test
  - Test: https://github.com/openconfig/featureprofiles/tree/main/feature/experimental/foo/tests/foo_test
  - Test: https://github.com/openconfig/featureprofiles/tree/main/feature/experimental/foo/unknown_tests/foo_test
`
	got := ttDesc(testDirs)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ttDesc -want,+got:\n%s", diff)
	}
}

func TestTTCase_Merge(t *testing.T) {
	wantAttrs := setDefaultAttrs(map[string]any{
		"author":      "liulk",
		"rel":         "testcases",
		"title":       "XX-1.1: Quuz Functional Test",
		"uuid":        "a5413d74-5b44-49d2-b4e7-84c9751d50be",
		"description": `""`,
	})
	wantAttrs["script_status"] = "Preserved"
	wantAttrs["duration"] = 42

	want := map[string]any{
		"type":    "testcases",
		"text":    "XX-1.1: Quuz Functional Test",
		"li_attr": wantAttrs,
		"pk":      1234567,
	}

	ttc := &ttCase{
		metadata: &mpb.Metadata{
			Uuid:        "a5413d74-5b44-49d2-b4e7-84c9751d50be",
			PlanId:      "XX-1.1",
			Description: "Quuz Functional Test",
		},
	}

	got := map[string]any{
		"li_attr": map[string]any{
			"author":        "liulk",
			"script_status": "Preserved",
			"duration":      42,
		},
		"pk": 1234567,
	}
	ttc.merge(got)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ttCase.merge() -want,+got:\n%s", diff)
	}
}
