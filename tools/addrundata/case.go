package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	mpb "github.com/openconfig/featureprofiles/proto/metadata_go_proto"
	"google.golang.org/protobuf/proto"
)

// testcase carries parsed rundata from different sources to be fixed and checked.
type testcase struct {
	pkg      string        // Package used in the test code.
	markdown *mpb.Metadata // From the README.md.
	existing *mpb.Metadata // From existing source code.
	fixed    *mpb.Metadata // Fixed rundata to write back, populated by fix().
}

// read reads the markdown and existing rundata from the test directory.
func (tc *testcase) read(testdir string) error {
	if err := readFile(filepath.Join(testdir, "README.md"), tc.readMarkdown); err != nil {
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
	if err := readFile(filepath.Join(testdir, "rundata_test.go"), tc.readCode); err != nil && !os.IsNotExist(err) {
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

func (tc *testcase) readMarkdown(r io.Reader) error {
	pd, err := parseMarkdown(r)
	if err != nil {
		return err
	}
	tc.markdown = pd
	return nil
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
	if tc.pkg == "" {
		return errors.New("missing test package name")
	}
	return nil
}

func (tc *testcase) readCode(r io.Reader) error {
	pd, err := parseCode(r)
	if err != nil {
		return err
	}
	tc.existing = pd
	return nil
}

// check verifies that existing rundata are valid.  The returned errors indicate issues
// that need fixing.
//
// It does not check the fixed rundata because that should already be valid.
func (tc *testcase) check() []error {
	var errs []error

	if tc.existing == nil {
		errs = append(errs, errors.New("existing rundata is missing"))
	}
	if tc.markdown == nil {
		errs = append(errs, errors.New("existing markdown is missing"))
	}

	if tc.markdown != nil && tc.existing != nil {
		if tc.existing.PlanId != tc.markdown.PlanId {
			errs = append(errs, fmt.Errorf(
				"rundata test plan ID needs update: was %q, will be %q",
				tc.existing.PlanId, tc.markdown.PlanId))
		}

		if tc.existing.Description != tc.markdown.Description {
			errs = append(errs, fmt.Errorf(
				"rundata test description needs update: was %q, will be %q",
				tc.existing.Description, tc.markdown.Description))
		}
	}

	if tc.existing != nil {
		if testUUID := tc.existing.Uuid; testUUID == "" {
			errs = append(errs, errors.New("missing UUID from rundata"))
		} else if u, err := uuid.Parse(testUUID); err != nil {
			errs = append(errs, fmt.Errorf(
				"cannot parse UUID from rundata: %s: %w", testUUID, err))
		} else if u.Variant() != uuid.RFC4122 || u.Version() != 4 {
			errs = append(errs, fmt.Errorf(
				"bad UUID from rundata: %s: got variant %s version %d; want variant RFC4122 version 4",
				testUUID, u.Variant(), u.Version()))
		}
	}

	return errs
}

// fix populates the fixed rundata from markdown or existing rundata.
func (tc *testcase) fix() error {
	if tc.markdown == nil {
		return errors.New("markdown rundata is missing")
	}

	tc.fixed = &mpb.Metadata{
		PlanId:      tc.markdown.PlanId,
		Description: tc.markdown.Description,
	}

	if tc.existing != nil {
		u, err := uuid.Parse(tc.existing.Uuid)
		if err == nil && u.Variant() == uuid.RFC4122 && u.Version() == 4 {
			// Existing UUID is valid, but make sure it is normalized.
			tc.fixed.Uuid = u.String()
			return nil
		}
	}

	// Generate a new UUID.  Consistency between ATE and OTG tests is not handled here.  It
	// will be done by testsuite's fix() function below.
	u, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	tc.fixed.Uuid = u.String()
	return nil
}

var errNoop = errors.New("already up to date")

// write commits the fixed rundata to the filesystem.
func (tc *testcase) write(testdir string) error {
	if tc.pkg == "" {
		return errors.New("missing test package name")
	}
	if tc.fixed == nil {
		return errors.New("test case was not fixed")
	}
	if proto.Equal(tc.existing, tc.fixed) {
		return errNoop
	}

	w := &strings.Builder{}
	if err := writeCode(w, tc.fixed, tc.pkg); err != nil {
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
