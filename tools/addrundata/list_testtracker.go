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

// listTestTracker writes the testsuite as a TestTracker test plan, which is formatted in
// JSON.  It optionally merges with an existing JSON if given.  The JSON uses a
// proprietary schema for test tracker.  See listJSON for a simpler schema.
func listTestTracker(w io.Writer, mergejson string, featuredir string, ts testsuite) error {
	rootdir := filepath.Dir(featuredir)
	ttp, ok := ttBuildPlan(ts, rootdir)
	if !ok {
		return errors.New("inconsistency is detected in rundata")
	}

	o := ttp.empty()
	if mergejson != "" {
		data, err := os.ReadFile(mergejson)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, &o); err != nil {
			return err
		}
	}

	ttp.merge(o)
	data, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

// ttBuildPlan builds a hierarchical ttPlan from a flat testsuite.  The ttPlan reorganizes
// the testsuite by splitting the test sections into ttSuite, and collates the different
// test kinds of the same test cases into the same ttCase.
func ttBuildPlan(ts testsuite, rootdir string) (ttp ttPlan, ok bool) {
	ttp = make(ttPlan)
	ok = true

	// This contains all the mappings from test UUID to the test cases across all test
	// sections, for the purpose of integrity checking.
	ttsall := make(ttSuite)

	for testdir, tc := range ts {
		if !tc.existing.hasData {
			errorf("Missing rundata: %s", testdir)
			ok = false
			continue
		}

		u := tc.existing.testUUID
		ttc := ttsall[u]
		if ttc == nil {
			ttc = &ttCase{}
			ttc.parsedData = tc.existing
			ttc.testDirs = make(map[string]string)
			ttsall[u] = ttc
		}

		if !reflect.DeepEqual(tc.existing, ttc.parsedData) {
			errorf("Test UUID %s has inconsistent data at %s and %#v", u, testdir, ttc.testDirs)
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
		ttc.testDirs[kind] = reldir

		sec := testSection(ttc.testPlanID)
		tts := ttp[sec]
		if tts == nil {
			tts = make(ttSuite)
			ttp[sec] = tts
		}
		tts[u] = ttc
	}

	return ttp, ok
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

// ttPlan maps from the test section (e.g. RT-1, TE-1) to a test suite which contains the
// test cases in that test section.
type ttPlan map[string]ttSuite

// empty creates a new JSON object representing an empty testplan.
func (ttp ttPlan) empty() map[string]any {
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

// sortedKeys returns the keys in ttPlan sorted in version order.
func (ttp ttPlan) sortedKeys() []string {
	var keys []string
	for k := range ttp {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return lessVersion(keys[i], keys[j])
	})
	return keys
}

// merge updates an existing "testplan" JSON object.
func (ttp ttPlan) merge(o map[string]any) {
	todos := make(ttPlan)
	for k, v := range ttp {
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
		tts := ttp[sec]
		if tts == nil {
			children = append(children, child) // Passthrough JSON-only testsuites.
			continue
		}
		tts.merge(o)
		children = append(children, o)
		delete(todos, sec)
	}

	// Update the todos that were missing from the JSON.
	for _, sec := range todos.sortedKeys() {
		tts := todos[sec]
		o := tts.empty(sec)
		tts.merge(o)
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

// ttSuite maps from the test UUID to a test case which aggregates the test locations by
// test kind.
type ttSuite map[string]*ttCase

// empty creates a new JSON object representing an empty testsuite.
func (tts ttSuite) empty(sec string) map[string]any {
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

// sortedKeys returns the UUID keys in ttSuite where the corresponding test plan IDs are
// sorted in version order.
func (tts ttSuite) sortedKeys() []string {
	var keys []string
	for k := range tts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return lessVersion(tts[keys[i]].testPlanID, tts[keys[j]].testPlanID)
	})
	return keys
}

// merge updates an existing "testsuites" JSON object.
func (tts ttSuite) merge(o map[string]any) {
	todos := make(ttSuite)
	bytp := make(ttSuite) // Lookup by test plan ID.
	for u, ttc := range tts {
		todos[u] = ttc
		bytp[ttc.testPlanID] = ttc
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
			if ttc := bytp[key.testPlanID]; ttc != nil {
				ttc.merge(key.o)
				children = append(children, key.o)
				// Use ttc.testUUID because key.testUUID from the JSON may be out of date.
				delete(todos, ttc.testUUID)
				continue
			}
		}

		if ttc := tts[key.testUUID]; ttc != nil {
			ttc.merge(key.o)
			children = append(children, key.o)
			delete(todos, key.testUUID)
			continue
		}

		children = append(children, child) // Passthrough JSON-only testcase.
	}

	// Update the todos that were missing from the JSON.
	for _, u := range todos.sortedKeys() {
		ttc := todos[u]
		o := make(map[string]any)
		ttc.merge(o)
		children = append(children, o)
	}

	o["children"] = children
}

// ttCaseKey represents the test UUID and test plan ID that could be extracted from an
// existing test case child of a test suite.
type ttCaseKey struct {
	o          map[string]any
	testUUID   string
	testPlanID string
}

// childCase returns the ttCaseKey from an existing child of the test suite, or
// nothing if the child is not a well-formed test case.
func childCase(child any) (key ttCaseKey, ok bool) {
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

// ttCase contains the test rundata and the test locations (if the test has multiple
// variants).
type ttCase struct {
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

// ttDesc builds a description with featureprofiles github links for each test kind.
func ttDesc(testDirs map[string]string) string {
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
func (ttc *ttCase) merge(o map[string]any) {
	title := fmt.Sprintf("%s: %s", ttc.testPlanID, ttc.testDescription)

	o["type"] = "testcases"
	o["text"] = title

	attrs, ok := o["li_attr"].(map[string]any)
	if !ok {
		attrs = map[string]any{}
		o["li_attr"] = attrs
	}

	attrs["rel"] = "testcases"
	attrs["title"] = title
	attrs["uuid"] = ttc.testUUID
	attrs["description"] = jsonQuote(ttDesc(ttc.testDirs))

	// Unused but required.
	for k, v := range defaultCaseAttrs {
		if _, ok := attrs[k]; !ok {
			attrs[k] = v
		}
	}
}
