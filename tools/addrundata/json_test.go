package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

var testsuiteForJSON = testsuite{
	"feature/foo/bar/ate_tests/qux_test": &testcase{
		existing: parsedData{
			testPlanID:      "YY-2.1",
			testDescription: "Qux Functional Test",
			testUUID:        "c857db98-7b2c-433c-b9fb-4511b42edd78",
			hasData:         true,
		},
	},
	"feature/foo/bar/otg_tests/qux_test": &testcase{
		existing: parsedData{
			testPlanID:      "YY-2.1",
			testDescription: "Qux Functional Test",
			testUUID:        "c857db98-7b2c-433c-b9fb-4511b42edd78",
			hasData:         true,
		},
	},
	"feature/foo/baz/quuz_test": &testcase{
		existing: parsedData{
			testPlanID:      "XX-1.1",
			testDescription: "Quuz Functional Test",
			testUUID:        "a5413d74-5b44-49d2-b4e7-84c9751d50be",
			hasData:         true,
		},
	},
}

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := writeJSON(&buf, "", "", testsuiteForJSON); err != nil {
		t.Fatal("Could not write JSON:", err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal("Could not unmarshal the generated JSON:", err)
	}
}

func TestWriteJSON_Merge(t *testing.T) {
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
	if err := writeJSON(&buf, mergejson, "", testsuiteForJSON); err != nil {
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

var jsopt = cmp.AllowUnexported(jsonCase{}, parsedData{})

func TestJSONBuildPlan(t *testing.T) {
	got, ok := jsonBuildPlan(testsuiteForJSON, "")
	if !ok {
		t.Fatal("Could not build jsonPlan.")
	}

	want := jsonPlan{
		"XX-1": jsonSuite{
			"a5413d74-5b44-49d2-b4e7-84c9751d50be": &jsonCase{
				parsedData: testsuiteForJSON["feature/foo/baz/quuz_test"].existing,
				testDirs:   map[string]string{"": "feature/foo/baz/quuz_test"},
			},
		},
		"YY-2": jsonSuite{
			"c857db98-7b2c-433c-b9fb-4511b42edd78": &jsonCase{
				parsedData: testsuiteForJSON["feature/foo/bar/ate_tests/qux_test"].existing,
				testDirs: map[string]string{
					"ate_tests": "feature/foo/bar/ate_tests/qux_test",
					"otg_tests": "feature/foo/bar/otg_tests/qux_test",
				},
			},
		},
	}

	if diff := cmp.Diff(want, got, jsopt); diff != "" {
		t.Errorf("jsonBuildPlan -want,+got:\n%s", diff)
	}
}

func TestJSONBuildPlan_MissingData(t *testing.T) {
	ts := testsuite{"feature/xyzzy/tests/quuz_test": &testcase{}}
	if _, ok := jsonBuildPlan(ts, ""); ok {
		t.Errorf("jsonBuildPlan ok got %v, want %v", ok, false)
	}
}

func TestJSONBuildPlan_DisallowReuseUUID(t *testing.T) {
	ts := testsuite{
		"feature/foo/bar/ate_tests/qux_test": &testcase{
			existing: parsedData{
				testPlanID:      "YY-2.1",
				testDescription: "Qux Functional Test",
				testUUID:        "c857db98-7b2c-433c-b9fb-4511b42edd78",
				hasData:         true,
			},
		},
		"feature/foo/baz/quuz_test": &testcase{
			existing: parsedData{
				testPlanID:      "XX-1.1",
				testDescription: "Quuz Functional Test",
				testUUID:        "c857db98-7b2c-433c-b9fb-4511b42edd78",
				hasData:         true,
			},
		},
	}
	if _, ok := jsonBuildPlan(ts, ""); ok {
		t.Errorf("jsonBuildPlan ok got %v, want %v", ok, false)
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

func TestJSONPlan_SortedKeys(t *testing.T) {
	jp := jsonPlan{
		"YY-10": jsonSuite{},
		"YY-1a": jsonSuite{},
		"YY-2":  jsonSuite{},
		"XX-10": jsonSuite{},
		"XX-1a": jsonSuite{},
		"XX-2":  jsonSuite{},
	}
	want := []string{"XX-1a", "XX-2", "XX-10", "YY-1a", "YY-2", "YY-10"}
	got := jp.sortedKeys()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("jsonPlan.sortedKeys() -want,+got:\n%s", diff)
	}
}

func setDefaultAttrs(attrs map[string]any) map[string]any {
	for k, v := range defaultCaseAttrs {
		attrs[k] = v
	}
	return attrs
}

func TestJSONPlan_Merge(t *testing.T) {
	got := map[string]any{
		"text": "JSON Plan To Be Merged",
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

	jp := jsonPlan{
		"XX-1": jsonSuite{
			"0eac5b62-ab22-449d-9a9a-255b05572641": &jsonCase{
				parsedData: parsedData{
					testPlanID:      "XX-1.1",
					testDescription: "Foo",
					testUUID:        "0eac5b62-ab22-449d-9a9a-255b05572641",
				},
			},
		},
		"XX-2": jsonSuite{
			"f842057d-0100-4198-a18d-593b2bf3610e": &jsonCase{
				parsedData: parsedData{
					testPlanID:      "XX-2.1",
					testDescription: "Bar",
					testUUID:        "f842057d-0100-4198-a18d-593b2bf3610e",
				},
			},
		},
		"YY-1": jsonSuite{
			"12cd2de3-69af-4aa6-a3d6-a2d5fbdb86c6": &jsonCase{
				parsedData: parsedData{
					testPlanID:      "YY-1.1",
					testDescription: "Xyzzy",
					testUUID:        "12cd2de3-69af-4aa6-a3d6-a2d5fbdb86c6",
				},
			},
		},
	}

	want := map[string]any{
		"text": "JSON Plan To Be Merged",
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

	jp.merge(got)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("jsonPlan.merge() -want,+got:\n%s", diff)
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

func TestJSONSuite_SortedKeys(t *testing.T) {
	js := jsonSuite{
		"864550d6-e843-4301-846a-a1998a23bb5a": &jsonCase{
			parsedData: parsedData{testPlanID: "XX-1.10"},
		},
		"e9345234-fc59-44f3-9d21-00b57137fb40": &jsonCase{
			parsedData: parsedData{testPlanID: "XX-1.1a"},
		},
		"bc261bca-d50f-42db-80f9-7c955c4e3889": &jsonCase{
			parsedData: parsedData{testPlanID: "XX-1.2"},
		},
	}
	want := []string{
		"e9345234-fc59-44f3-9d21-00b57137fb40", // XX-1.1a
		"bc261bca-d50f-42db-80f9-7c955c4e3889", // XX-1.2
		"864550d6-e843-4301-846a-a1998a23bb5a", // XX-1.10
	}
	got := js.sortedKeys()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("jsonSuite.sortedKeys() -want,+got:\n%s", diff)
	}
}

func TestJSONSuite_Merge(t *testing.T) {
	got := map[string]any{
		"text": "JSON Suite To Be Merged",
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

	js := jsonSuite{
		"d2d462b4-db36-4159-9152-744dc6168ba8": &jsonCase{
			parsedData: parsedData{
				testPlanID:      "XX-1.1",
				testDescription: "Apple",
				testUUID:        "d2d462b4-db36-4159-9152-744dc6168ba8",
			},
		},
		"755ae14f-7d1a-465a-8cfb-f1674ea68763": &jsonCase{
			parsedData: parsedData{
				testPlanID:      "XX-1.2",
				testDescription: "Banana",
				testUUID:        "755ae14f-7d1a-465a-8cfb-f1674ea68763",
			},
		},
		"c2cb54c0-2acc-4fd8-8c2a-7f9ccb9ea192": &jsonCase{
			parsedData: parsedData{
				testPlanID:      "XX-1.3",
				testDescription: "Cherry",
				testUUID:        "c2cb54c0-2acc-4fd8-8c2a-7f9ccb9ea192",
			},
		},
		"f7372990-dfb2-4a8f-acfb-c7a31b29522c": &jsonCase{
			parsedData: parsedData{
				testPlanID:      "XX-1.4",
				testDescription: "Durian",
				testUUID:        "f7372990-dfb2-4a8f-acfb-c7a31b29522c",
			},
		},
	}

	want := map[string]any{
		"text": "JSON Suite To Be Merged",
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

	js.merge(got)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("jsonSuite.merge() -want,+got:\n%s", diff)
	}
}

var jkopt = cmp.AllowUnexported(jsonCaseKey{})

func TestChildCase(t *testing.T) {
	cases := map[string]struct {
		child  any
		wantok bool
		want   jsonCaseKey
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
			want: jsonCaseKey{
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
			want: jsonCaseKey{
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
			want: jsonCaseKey{
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
			want: jsonCaseKey{
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
			want: jsonCaseKey{
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
				t.Errorf("childCase jsonCaseKey -want,+got:\n%s", diff)
			}
		})
	}
}

func TestJSONDesc(t *testing.T) {
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
	got := jsonDesc(testDirs)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("jsonDesc -want,+got:\n%s", diff)
	}
}

func TestJSONCase_Merge(t *testing.T) {
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

	jc := &jsonCase{
		parsedData: parsedData{
			testPlanID:      "XX-1.1",
			testDescription: "Quuz Functional Test",
			testUUID:        "a5413d74-5b44-49d2-b4e7-84c9751d50be",
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
	jc.merge(got)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("jsonCase.merge() -want,+got:\n%s", diff)
	}
}
