// Command addrundata adds or updates rundata reporting to all tests in the source code,
// based on each of their README.md.
//
// Tests are found under feature/${feature}/${subfeature}/${testkind}/${testname} where
// testkind is one of "ate_tests", "otg_tests", or simply "tests".  If an ATE test is
// present, it should have the same rundata as the OTG test.
//
// The rundata is stored in the rundata_test.go file in the test package.  Other test
// files are left unchanged.
//
// Test plan ID and the description are extracted from the README.md, whereas the UUID is
// randomly assigned.  Existing UUID assignments are honored.  ATE and OTG versions of the
// same test must have the same UUID.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/golang/glog"
)

var (
	fix = flag.Bool("fix", false, "Update the rundata in tests.  If false, only check if the tests have the most recent rundata.")
)

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func featureDir() (string, error) {
	_, path, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("could not detect caller")
	}
	newpath := filepath.Dir(path)
	for newpath != "." && newpath != "/" {
		featurepath := filepath.Join(newpath, "feature")
		if isDir(featurepath) {
			return featurepath, nil
		}
		newpath = filepath.Dir(newpath)
	}
	return "", fmt.Errorf("feature root not found from %s", path)
}

func main() {
	flag.Parse()

	featuredir, err := featureDir()
	if err != nil {
		glog.Exitf("Unable to locate feature root: %v", err)
	}

	ts := testsuite{}
	if ok := ts.read(featuredir); !ok {
		glog.Exitf("Problems found under feature root.  Please make sure test paths follow feature/<feature>/<subfeature>/<testkind>/<testname>/<testname>_test.go and all tests have an accompanying README.md in the test directory.")
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
