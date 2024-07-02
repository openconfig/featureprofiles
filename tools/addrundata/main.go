// Command addrundata adds or updates rundata reporting to all tests in the source code,
// based on each of their README.md.
//
// Tests are found under feature/${feature}/${subfeature}/${testkind}/${testname} where
// testkind is one of "ate_tests", "otg_tests", or simply "tests".  If an ATE test is
// present, it should have the same rundata as the OTG test.
//
// The rundata is stored in the metadata.textproto file in the test package.  Other test
// files are left unchanged.
//
// Test plan ID and the description are extracted from the README.md, whereas the UUID is
// randomly assigned.  Existing UUID assignments are honored.  ATE and OTG versions of the
// same test must have the same UUID.
package main

import (
	"fmt"
	"os"

	"flag"

	"github.com/golang/glog"
	"github.com/openconfig/featureprofiles/tools/internal/fpciutil"
)

var (
	dir       = flag.String("dir", "", "Directory to search for tests; if not specified, uses the ancestor 'feature' directory.")
	fix       = flag.Bool("fix", false, "Update the rundata in tests.  If false, only check if the tests have the most recent rundata.")
	list      = flag.String("list", "", "List the tests in one of the following formats: csv, json")
	mergejson = flag.String("mergejson", "", "Merge the JSON listing from this JSON file.")
)

func main() {
	flag.Parse()

	featuredir := *dir
	if featuredir == "" {
		var err error
		featuredir, err = fpciutil.FeatureDir()
		if err != nil {
			glog.Exitf("Unable to locate feature root: %v", err)
		}
	}

	ts := testsuite{}
	if ok := ts.read(featuredir); !ok {
		glog.Exitf("Problems found under feature root.  Please make sure test paths follow feature/<feature>/<subfeature>/<testkind>/<testname>/<testname>_test.go and all tests have an accompanying README.md in the test directory.")
	}

	switch *list {
	case "":
		// Not listing, so it's either check-only or fix.  See below.
	case "csv":
		if err := listCSV(os.Stdout, featuredir, ts); err != nil {
			glog.Exitf("Error writing CSV: %v", err)
		}
		return
	case "json":
		if err := listJSON(os.Stdout, featuredir, ts); err != nil {
			glog.Exitf("Error writing JSON: %v", err)
		}
		return
	case "testtracker":
		if err := listTestTracker(os.Stdout, *mergejson, featuredir, ts); err != nil {
			glog.Exitf("Error writing TestTracker: %v", err)
		}
		return
	default:
		glog.Exitf("Unknown listing format: %s", *list)
	}

	if !*fix {
		if ok := ts.check(featuredir); !ok {
			glog.Exitf("Rundata check found problems.  Please run: go run ./tools/addrundata --fix")
		}
		return
	}

	if ok := ts.check(featuredir); !ok {
		glog.Errorf("Rundata check found problems.  Will try to apply fixes.")
	}
	if ok := ts.fix(); !ok {
		glog.Exitf("Failed to fix rundata.")
	}

	switch err := ts.write(featuredir); err {
	case errNoop:
		fmt.Fprintln(os.Stderr, "Everything already up to date.")
	case nil:
		fmt.Fprintln(os.Stderr, "Rundata are successfully updated.")
	default:
		glog.Exitf("Failed to update rundata: %v", err)
	}
}
