package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// writeJSON writes the testsuite as JSON, optionally merges with an existing JSON if
// given.  This JSON uses a proprietary schema for test tracker, so it is not recommended
// for general use.  If you want a JSON listing, feel free to file a feature request to
// describe your use case.
func writeJSON(w io.Writer, mergejson string, featuredir string, ts testsuite) error {
	rootdir := filepath.Dir(featuredir)
	jp, ok := jsonBuildPlan(ts, rootdir)
	if !ok {
		return errors.New("inconsistency is detected in rundata")
	}

	o := jp.empty()
	if mergejson != "" {
		data, err := os.ReadFile(mergejson)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, &o); err != nil {
			return err
		}
	}

	jp.merge(o)
	data, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

// jsonBuildPlan builds a hierarchical jsonPlan from a flat testsuite.  The jsonPlan
// reorganizes the testsuite by splitting the test sections into jsonSuite, and collates
// the different test kinds of the same test cases into the same jsonCase.
func jsonBuildPlan(ts testsuite, rootdir string) (jp jsonPlan, ok bool) {
	jp = make(jsonPlan)
	ok = true

	// This contains all the mappings from test UUID to the JSON test case across all test
	// sections, for the purpose of integrity checking.
	jsall := make(jsonSuite)

	for testdir, tc := range ts {
		if !tc.existing.hasData {
			errorf("Missing rundata: %s", testdir)
			ok = false
			continue
		}

		u := tc.existing.testUUID
		jc := jsall[u]
		if jc == nil {
			jc = &jsonCase{}
			jc.parsedData = tc.existing
			jc.testDirs = make(map[string]string)
			jsall[u] = jc
		}

		if !reflect.DeepEqual(tc.existing, jc.parsedData) {
			errorf("Test UUID %s has inconsistent data at %s and %#v", u, testdir, jc.testDirs)
			ok = false
			continue
		}

		kind := testKind(testdir)
		if !isTestKind(kind) {
			kind = ""
		}
		reldir, err := filepath.Rel(rootdir, testdir)
		if err != nil {
			reldir = ""
		}
		jc.testDirs[kind] = reldir

		sec := testSection(jc.testPlanID)
		js := jp[sec]
		if js == nil {
			js = make(jsonSuite)
			jp[sec] = js
		}
		js[u] = jc
	}

	return jp, ok
}

// testSection returns the test section (e.g. RT-1) part of the test plan ID
// (e.g. RT-1.2).
func testSection(testPlanID string) string {
	i := strings.IndexRune(testPlanID, '.')
	if i < 0 {
		i = len(testPlanID)
	}
	return testPlanID[:i]
}

// jsonQuote quotes the string using JSON convention.
func jsonQuote(s string) string {
	data, err := json.Marshal(s)
	if err != nil {
		return strconv.Quote(s)
	}
	return string(data)
}

// jsonPlan maps from the test section (e.g. RT-1, TE-1) to a JSON test suite which
// contains the test cases in that test section.
type jsonPlan map[string]jsonSuite

// empty creates a new JSON object representing an empty testplan.
func (jp jsonPlan) empty() map[string]any {
	const title = "Feature Profiles Test Plan"
	return map[string]any{
		"text": title,
		"type": "testplan",
		"li_attr": map[string]any{
			"rel":          "testplan",
			"title":        title,
			"introduction": jsonQuote("https://github.com/openconfig/featureprofiles"),
		},
	}
}

// sortedKeys returns the keys in jsonPlan sorted in version order.
func (jp jsonPlan) sortedKeys() []string {
	var keys []string
	for k := range jp {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return lessVersion(keys[i], keys[j])
	})
	return keys
}

// merge updates an existing "testplan" JSON object.
func (jp jsonPlan) merge(o map[string]any) {
	todos := make(jsonPlan)
	for k, v := range jp {
		todos[k] = v
	}

	// Update existing children first.
	oldchildren, _ := o["children"].([]any) // Even if !ok, nil is fine.
	var children []any

	for _, child := range oldchildren {
		o, sec, ok := childSuite(child)
		if !ok {
			children = append(children, child) // Passthrough mal-formed testsuites.
			continue
		}
		js := jp[sec]
		if js == nil {
			children = append(children, child) // Passthrough JSON-only testsuites.
			continue
		}
		js.merge(o)
		children = append(children, o)
		delete(todos, sec)
	}

	// Update the todos that were missing from the JSON.
	for _, sec := range todos.sortedKeys() {
		js := todos[sec]
		o := js.empty(sec)
		js.merge(o)
		children = append(children, o)
	}

	o["children"] = children
}

var bracketRE = regexp.MustCompile(`\[(.*?)\]`)

// childSuite returns the JSON object and test section key from an existing child of the
// test plan, or nothing if the child is not a well-formed test suite.
func childSuite(child any) (o map[string]any, sec string, ok bool) {
	o, ok = child.(map[string]any)
	if !ok {
		return nil, "", false
	}
	text, ok := o["text"].(string)
	if !ok {
		return nil, "", false
	}
	matches := bracketRE.FindStringSubmatch(text)
	if matches == nil {
		return nil, "", false
	}
	return o, matches[1], true
}

// jsonSuite maps from the test UUID to a JSON test case which aggregates the test
// locations by test kind.
type jsonSuite map[string]*jsonCase

// empty creates a new JSON object representing an empty testsuite.
func (js jsonSuite) empty(sec string) map[string]any {
	title := fmt.Sprintf("[%s]", sec)
	return map[string]any{
		"text": title,
		"type": "testsuites",
		"li_attr": map[string]any{
			"rel":         "testsuites",
			"title":       title,
			"description": jsonQuote(""),
			"tags":        "",
		},
	}
}

// sortedKeys returns the UUID keys in jsonSuite where the corresponding test plan IDs are
// sorted in version order.
func (js jsonSuite) sortedKeys() []string {
	var keys []string
	for k := range js {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return lessVersion(js[keys[i]].testPlanID, js[keys[j]].testPlanID)
	})
	return keys
}

// merge updates an existing "testsuites" JSON object.
func (js jsonSuite) merge(o map[string]any) {
	todos := make(jsonSuite)
	bytp := make(jsonSuite) // Lookup by test plan ID.
	for u, jc := range js {
		todos[u] = jc
		bytp[jc.testPlanID] = jc
	}

	// Update existing children first.
	oldchildren, _ := o["children"].([]any) // Even if !ok, nil is fine.
	var children []any

	for _, child := range oldchildren {
		key, ok := childCase(child)
		if !ok {
			children = append(children, child) // Passthrough mal-formed testcase.
			continue
		}

		if key.testPlanID != "" {
			if jc := bytp[key.testPlanID]; jc != nil {
				jc.merge(key.o)
				children = append(children, key.o)
				// Use jc.testUUID because key.testUUID from the JSON may be out of date.
				delete(todos, jc.testUUID)
				continue
			}
		}

		if jc := js[key.testUUID]; jc != nil {
			jc.merge(key.o)
			children = append(children, key.o)
			delete(todos, key.testUUID)
			continue
		}

		children = append(children, child) // Passthrough JSON-only testcase.
	}

	// Update the todos that were missing from the JSON.
	for _, u := range todos.sortedKeys() {
		jc := todos[u]
		o := make(map[string]any)
		jc.merge(o)
		children = append(children, o)
	}

	o["children"] = children
}

// jsonCaseKey represents the test UUID and test plan ID that could be extracted from an
// existing JSON test case child of a test suite.
type jsonCaseKey struct {
	o          map[string]any
	testUUID   string
	testPlanID string
}

// childCase returns the jsonCaseKey from an existing child of the test suite, or
// nothing if the child is not a well-formed test case.
func childCase(child any) (key jsonCaseKey, ok bool) {
	key.o, ok = child.(map[string]any)
	if !ok {
		return key, false
	}

	attrs, ok := key.o["li_attr"].(map[string]any)
	if !ok {
		return key, false
	}

	// Populate key.testUUID.
	key.testUUID, ok = attrs["uuid"].(string)
	if !ok {
		return key, false
	}
	u, err := uuid.Parse(key.testUUID)
	if err != nil || u.Variant() != uuid.RFC4122 || u.Version() != 4 {
		return key, false
	}
	key.testUUID = u.String() // Normalize.

	// Populate key.testPlanID which is optional.
	title, ok := attrs["title"].(string)
	if !ok {
		return key, true // Optional.
	}
	if i := strings.IndexRune(title, ':'); i >= 0 {
		key.testPlanID = strings.TrimSpace(title[:i])
	}
	return key, true
}

// jsonCase contains the test rundata and the test locations (if the test has multiple
// variants).
type jsonCase struct {
	parsedData
	testDirs map[string]string // Test locations by test kind.
}

// buildinSortedKeys returns the keys of a map[string]string sorted using the builtin
// comparison.
func builtinSortedKeys(m map[string]string) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	return keys
}

// jsonDesc builds a description with featureprofiles github links for each test kind.
func jsonDesc(testDirs map[string]string) string {
	if len(testDirs) == 0 {
		return ""
	}

	const repoTreeMain = "https://github.com/openconfig/featureprofiles/tree/main"

	var desc strings.Builder
	fmt.Fprintln(&desc, "See code location:")

	kinds := builtinSortedKeys(testDirs)
	for _, kind := range kinds {
		kindstr := testKinds[kind]
		if kindstr == "" {
			kindstr = "Test"
		}
		fmt.Fprintf(&desc, "  - %s: %s/%s\n", kindstr, repoTreeMain, testDirs[kind])
	}

	return desc.String()
}

var defaultCaseAttrs = map[string]any{
	"script":        []string{},
	"requirement":   "",
	"script_name":   "",
	"script_type":   "NA",
	"script_status": "Needs Evaluation",
	"tags":          "",
	"priority":      0,
	"duration":      0,
	"goal":          "",
	"topology":      nil,
}

// merge updates an existing "testcases" JSON object.
func (jc *jsonCase) merge(o map[string]any) {
	title := fmt.Sprintf("%s: %s", jc.testPlanID, jc.testDescription)

	o["type"] = "testcases"
	o["text"] = title

	attrs, ok := o["li_attr"].(map[string]any)
	if !ok {
		attrs = map[string]any{}
		o["li_attr"] = attrs
	}

	attrs["rel"] = "testcases"
	attrs["title"] = title
	attrs["uuid"] = jc.testUUID
	attrs["description"] = jsonQuote(jsonDesc(jc.testDirs))

	// Unused but required.
	for k, v := range defaultCaseAttrs {
		if _, ok := attrs[k]; !ok {
			attrs[k] = v
		}
	}
}
