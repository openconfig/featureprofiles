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
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode"

	"flag"

	"github.com/openconfig/featureprofiles/internal/check"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

var (
	// outputsDir defaults to the path of the undeclared test outputs
	// directory; see Bazel Test Encyclopedia.
	// https://docs.bazel.build/versions/main/test-encyclopedia.html
	outputsDir = flag.String("outputs_dir",
		os.Getenv("TEST_UNDECLARED_OUTPUTS_DIR"),
		"specifies the directory where test results will be written")
)

// sanitizeFilename keeps letters, digits, and safe punctuations, but removes
// unsafe punctuations and other characters.
func sanitizeFilename(filename string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		switch r {
		case '+', ',', '-', '.', ':', ';', '=', '^', '|', '~':
			return r
		case '(', ')', '<', '>', '[', ']', '{', '}':
			return r
		case ' ', '/', '_':
			return '_'
		default:
			return -1 // drop
		}
	}, filename)
}

// WriteOutput writes content to a file in --outputs_dir, after sanitizing
// the filename and making it unique.  Returns the sanitized filename
// relative to --outputs_dir.
func WriteOutput(filename, suffix string, content string) (string, error) {
	if *outputsDir == "" {
		log.Printf("Test output %q is discarded without -outputs_dir.  Please specify -outputs_dir to keep it.", filename)
		return "", nil
	}
	template := fmt.Sprintf(
		"%s.%s%s%s",
		sanitizeFilename(filename),
		time.Now().Format("03:04:05"), // order by time to help discovery.
		".*",                          // randomize for os.CreateTemp()
		suffix)
	f, err := os.CreateTemp(*outputsDir, template)
	if err != nil {
		return "", err
	}
	defer f.Close()
	_, err = f.Write([]byte(content))
	log.Printf("Test output written: %s", f.Name())

	rel, rerr := filepath.Rel(*outputsDir, f.Name())
	if rerr != nil {
		rel = f.Name()
	}
	return rel, err
}

// LoggableQuery is a subset of the ygnmi.AnyQuery type used for logging
type LoggableQuery interface {
	PathStruct() ygnmi.PathStruct
	IsState() bool
}

var _ LoggableQuery = ygnmi.AnyQuery[string](nil)

// LogQuery logs a ygot GoStruct at path as either config or telemetry,
// depending on the query.  It also writes a copy to a *.json file in
// the directory specified by the -outputs_dir flag.
func LogQuery(t testing.TB, what string, query LoggableQuery, obj ygot.GoStruct) {
	t.Helper()
	logQuery(t, what, query, obj, true)
}

// WriteQuery is like LogQuery but only writes to test outputs dir so it
// does not pollute the test log.
func WriteQuery(t testing.TB, what string, query LoggableQuery, obj ygot.GoStruct) {
	t.Helper()
	logQuery(t, what, query, obj, false)
}

func logQuery(t testing.TB, what string, query LoggableQuery, obj ygot.GoStruct, shouldLog bool) {
	t.Helper()
	pathText := check.FormatPath(query.PathStruct())
	config := !query.IsState()

	var title string
	if config {
		title = "Config"
	} else {
		title = "Telemetry"
	}

	header := fmt.Sprintf("%s for %s at %s", title, what, pathText)
	text, err := ygot.EmitJSON(obj, &ygot.EmitJSONConfig{
		Format: ygot.RFC7951,
		RFC7951Config: &ygot.RFC7951JSONConfig{
			AppendModuleName: true,
			PreferShadowPath: config,
		},
		Indent:         "  ",
		SkipValidation: true,
	})
	if err != nil {
		t.Errorf("%s render error: %v", header, err)
	}
	if shouldLog {
		t.Logf("%s:\n%s", header, text)
	}
	filename, err := WriteOutput(t.Name()+" "+header, ".json", text)
	if err != nil {
		t.Logf("Could not write test output: %v", err)
	}
	if filename != "" {
		ondatra.Report().AddTestProperty(t, "test_output0", filename)
	}
}
