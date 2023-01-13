package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/google/uuid"
)

// testcase carries parsed rundata from different sources to be fixed and checked.
type testcase struct {
	pkg      string     // Package used in the test code.
	markdown parsedData // From the README.md.
	existing parsedData // From existing source code.
	fixed    parsedData // Fixed rundata to write back, populated by fix().
}

// read reads the markdown and existing rundata from the test directory.
func (tc *testcase) read(testdir string) error {
	if err := readFile(filepath.Join(testdir, "README.md"), tc.markdown.fromMarkdown); err != nil {
		return fmt.Errorf("could not parse README.md: %w", err)
	}
	testpaths, err := filepath.Glob(filepath.Join(testdir, "*_test.go"))
	if err != nil {
		return fmt.Errorf("could not glob: %w", err)
	}
	for _, testpath := range testpaths {
		if err := readFile(testpath, tc.readPackage); err != nil {
			return fmt.Errorf("could not detect test package: %w", err)
		}
		if tc.pkg != "" {
			break
		}
	}
	if err := readFile(filepath.Join(testdir, "rundata_test.go"), tc.existing.fromCode); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("could not parse rundata_test.go: %w", err)
	}
	return nil
}

// readFile opens a filename for reading and calls the specified reader function.
func readFile(filename string, fn func(io.Reader) error) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return fn(bytes.NewReader(data))
}

func (tc *testcase) readPackage(r io.Reader) error {
	const pkg = "package"
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, pkg) {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[0] == pkg {
			tc.pkg = parts[1]
			break
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return nil
}

// check verifies that existing rundata are valid.  The returned errors indicate issues
// that need fixing.
//
// It does not check the fixed rundata because that should already be valid.
func (tc *testcase) check(testdir string) []error {
	var errs []error

	if tc.markdown.hasData {
		if tc.existing.testPlanID != tc.markdown.testPlanID {
			errs = append(errs, fmt.Errorf(
				"rundata test plan ID needs update: was %q, will be %q",
				tc.existing.testPlanID, tc.markdown.testPlanID))
		}

		if tc.existing.testDescription != tc.markdown.testDescription {
			errs = append(errs, fmt.Errorf(
				"rundata test description needs update: was %q, will be %q",
				tc.existing.testDescription, tc.markdown.testDescription))
		}
	} else {
		errs = append(errs, errors.New("markdown rundata is missing"))
	}

	if testUUID := tc.existing.testUUID; testUUID == "" {
		errs = append(errs, errors.New("missing UUID from rundata"))
	} else if u, err := uuid.Parse(testUUID); err != nil {
		errs = append(errs, fmt.Errorf(
			"cannot parse UUID from rundata: %s: %w", testUUID, err))
	} else if u.Variant() != uuid.RFC4122 || u.Version() != 4 {
		errs = append(errs, fmt.Errorf(
			"bad UUID from rundata: %s: got variant %s version %d; want variant RFC4122 version 4",
			testUUID, u.Variant(), u.Version()))
	}

	return errs
}

// fix populates the fixed rundata from markdown or existing rundata.
func (tc *testcase) fix() error {
	if !tc.markdown.hasData {
		return errors.New("markdown rundata is missing")
	}

	tc.fixed.testPlanID = tc.markdown.testPlanID
	tc.fixed.testDescription = tc.markdown.testDescription
	tc.fixed.hasData = true

	u, err := uuid.Parse(tc.existing.testUUID)
	if err == nil && u.Variant() == uuid.RFC4122 && u.Version() == 4 {
		// Existing UUID is valid, but make sure it is normalized.
		tc.fixed.testUUID = u.String()
		return nil
	}

	// Generate a new UUID.  Consistency between ATE and OTG tests is not handled here.  It
	// will be done by testsuite's fix() function below.
	u, err = uuid.NewRandom()
	if err != nil {
		return err
	}
	tc.fixed.testUUID = u.String()
	return nil
}

var errNoop = errors.New("already up to date")

// write commits the fixed rundata to the filesystem.
func (tc *testcase) write(testdir string) error {
	if tc.pkg == "" {
		return errors.New("missing test package name")
	}
	if !tc.fixed.hasData {
		return errors.New("test case was not fixed")
	}
	if reflect.DeepEqual(tc.existing, tc.fixed) {
		return errNoop
	}

	w := &strings.Builder{}
	if err := tc.fixed.write(w, tc.pkg); err != nil {
		return fmt.Errorf("could not generate the rundata: %w", err)
	}

	out, err := os.CreateTemp(testdir, "rundata_test.go.*")
	if err != nil {
		return fmt.Errorf("could not create: %w", err)
	}
	defer out.Close()
	if _, err := out.WriteString(w.String()); err != nil {
		return fmt.Errorf("could not write: %w", err)
	}

	source := filepath.Join(testdir, "rundata_test.go")
	return os.Rename(out.Name(), source)
}
