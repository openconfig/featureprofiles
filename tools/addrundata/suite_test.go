package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	mpb "github.com/openconfig/featureprofiles/proto/metadata_go_proto"
)

// prepareSuite is like ts.write() but for testing purpose.  It writes out the testsuite
// relative to featuredir and builds a new testsuite where the testdir keys are prefixed
// by featuredir, and the testcases are rebuilt according to how it would have been read
// by ts.read().  It also writes the README.md file which would otherwise be untouched by
// ts.write().
func prepareSuite(featuredir string, ts testsuite) (testsuite, error) {
	newts := make(testsuite)
	for reldir, tc := range ts {
		testdir := filepath.Join(featuredir, reldir)
		if err := os.MkdirAll(testdir, 0700); err != nil {
			return nil, err
		}
		if err := tc.write(testdir); err != nil {
			return nil, fmt.Errorf("could not write %s: %w", testdir, err)
		}
		readme := fmt.Sprintf("# %s: %s\n", tc.fixed.PlanId, tc.fixed.Description)
		readmeFilename := filepath.Join(testdir, "README.md")
		if err := os.WriteFile(readmeFilename, []byte(readme), 0600); err != nil {
			return nil, fmt.Errorf("could not write %s: %w", readmeFilename, err)
		}
		testFilename := filepath.Join(testdir, "foo_test.go")
		if _, err := os.Create(testFilename); err != nil {
			return nil, fmt.Errorf("could not create %s: %w", testFilename, err)
		}
		newts[testdir] = &testcase{
			markdown: &mpb.Metadata{
				PlanId:      tc.fixed.PlanId,
				Description: tc.fixed.Description,
			},
			existing: &mpb.Metadata{
				Uuid:        tc.fixed.Uuid,
				PlanId:      tc.fixed.PlanId,
				Description: tc.fixed.Description,
			},
		}
	}
	return newts, nil
}

func TestSuite_Read(t *testing.T) {
	featuredir := t.TempDir()
	want, err := prepareSuite(featuredir, testsuite{
		"foo/bar/ate_tests/qux_test": &testcase{
			fixed: &mpb.Metadata{
				Uuid:        "c857db98-7b2c-433c-b9fb-4511b42edd78",
				PlanId:      "XX-2.1",
				Description: "Qux Functional Test",
			},
		},
		"foo/bar/otg_tests/qux_test": &testcase{
			fixed: &mpb.Metadata{
				Uuid:        "c857db98-7b2c-433c-b9fb-4511b42edd78",
				PlanId:      "XX-2.1",
				Description: "Qux Functional Test",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	got := make(testsuite)
	if !got.read(featuredir) {
		t.Fatalf("Could not read: %s", featuredir)
	}

	if diff := cmp.Diff(want, got, tcopts...); diff != "" {
		t.Errorf("testsuite.read -want,+got:\n%s", diff)
	}
}

func TestSuite_Read_BadPath(t *testing.T) {
	featuredir := t.TempDir()
	_, err := prepareSuite(featuredir, testsuite{
		"foo/bar/qux_test": &testcase{
			fixed: &mpb.Metadata{
				Uuid:        "c857db98-7b2c-433c-b9fb-4511b42edd78",
				PlanId:      "XX-2.1",
				Description: "Qux Functional Test",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	got := make(testsuite)
	if ok := got.read(featuredir); ok {
		t.Fatalf("got.read ok got %v, want %v", ok, false)
	}
}

func TestSuite_Check(t *testing.T) {
	quxMarkdownOnly := &testcase{
		markdown: &mpb.Metadata{
			PlanId:      "XX-2.1",
			Description: "Qux Functional Test",
		},
	}
	qux := &testcase{
		markdown: &mpb.Metadata{
			PlanId:      "XX-2.1",
			Description: "Qux Functional Test",
		},
		existing: &mpb.Metadata{
			Uuid:        "c857db98-7b2c-433c-b9fb-4511b42edd78",
			PlanId:      "XX-2.1",
			Description: "Qux Functional Test",
			Testbed:     mpb.Metadata_TESTBED_DUT_ATE_4LINKS,
		},
	}
	quuz := &testcase{
		markdown: &mpb.Metadata{
			PlanId:      "XX-2.2",
			Description: "Quuz Functional Test",
		},
		existing: &mpb.Metadata{
			Uuid:        "a5413d74-5b44-49d2-b4e7-84c9751d50be",
			PlanId:      "XX-2.2",
			Description: "Quuz Functional Test",
			Testbed:     mpb.Metadata_TESTBED_DUT_DUT_4LINKS,
		},
	}
	quuzDupPlanID := &testcase{
		markdown: &mpb.Metadata{
			PlanId:      "XX-2.1", // from qux.
			Description: "Quuz Functional Test",
		},
		existing: &mpb.Metadata{
			Uuid:        "a5413d74-5b44-49d2-b4e7-84c9751d50be",
			PlanId:      "XX-2.1", // from qux.
			Description: "Quuz Functional Test",
		},
	}
	quuzDupUUID := &testcase{
		markdown: &mpb.Metadata{
			PlanId:      "XX-2.2",
			Description: "Qux Functional Test",
		},
		existing: &mpb.Metadata{
			Uuid:        "c857db98-7b2c-433c-b9fb-4511b42edd78",
			PlanId:      "XX-2.2",
			Description: "Qux Functional Test",
		},
	}

	wants := []struct {
		name string
		ts   testsuite
		ok   bool
	}{{
		name: "NeedsUpdate",
		ts: testsuite{
			"foo/bar/tests/qux_test": quxMarkdownOnly,
		},
		ok: false,
	}, {
		name: "Updated",
		ts: testsuite{
			"foo/bar/tests/qux_test":  qux,
			"foo/bar/tests/quuz_test": quuz,
		},
		ok: true,
	}, {
		name: "DuplicateTestPlanID",
		ts: testsuite{
			"foo/bar/tests/qux_test":  qux,
			"foo/bar/tests/quuz_test": quuzDupPlanID,
		},
		ok: false,
	}, {
		name: "DuplicateUUID",
		ts: testsuite{
			"foo/bar/tests/qux_test":  qux,
			"foo/bar/tests/quuz_test": quuzDupUUID,
		},
		ok: false,
	}, {
		name: "SameATEOTG",
		ts: testsuite{
			"foo/bar/ate_tests/qux_test": qux,
			"foo/bar/otg_tests/qux_test": qux,
		},
		ok: true,
	}, {
		name: "DifferentATEOTG",
		ts: testsuite{
			"foo/bar/ate_tests/qux_test": qux,
			"foo/bar/otg_tests/qux_test": quuz,
		},
		ok: false,
	}}

	for _, want := range wants {
		t.Run(want.name, func(t *testing.T) {
			gotok := want.ts.check("")
			if gotok != want.ok {
				t.Errorf("Check got ok %v, want %v", gotok, want.ok)
			}
		})
	}
}

func TestSuite_Fix(t *testing.T) {
	quxMarkdownOnly := &testcase{
		markdown: &mpb.Metadata{
			PlanId:      "XX-2.1",
			Description: "Qux Functional Test",
		},
	}

	// Each testcase needs their own copy because fix modifies the testcase in place.
	copyCase := func(tc testcase) *testcase {
		return &tc
	}

	ts := testsuite{
		"foo/bar/ate_tests/qux_test": copyCase(*quxMarkdownOnly),
		"foo/bar/otg_tests/qux_test": copyCase(*quxMarkdownOnly),
	}

	if !ts.fix() {
		t.Error("testsuite.fix failed")
	}

	ateFixed := ts["foo/bar/ate_tests/qux_test"].fixed
	otgFixed := ts["foo/bar/otg_tests/qux_test"].fixed

	if diff := cmp.Diff(ateFixed, otgFixed, tcopts...); diff != "" {
		t.Errorf("After fix, ATE and OTG rundata differ (-ate,+otg):\n%s", diff)
	}
}

func checkMarkdowns(t testing.TB, featuredir string, ts testsuite, markdowns map[string]*mpb.Metadata) {
	t.Helper()

	for reldir, wantpd := range markdowns {
		testdir := filepath.Join(featuredir, reldir)
		tc, ok := ts[testdir]
		if !ok {
			t.Errorf("Not read: %s", reldir)
			continue
		}
		if diff := cmp.Diff(wantpd, tc.markdown, pdopt); diff != "" {
			t.Errorf("Markdown differs -want,+got:\n%s", diff)
		}
	}
}

func TestSuite_ReadFixWriteReadCheck(t *testing.T) {
	markdowns := map[string]*mpb.Metadata{
		"foo/bar/ate_tests/qux_test": {
			PlanId:      "XX-2.1",
			Description: "Qux Functional Test",
		},
		"foo/bar/otg_tests/qux_test": {
			PlanId:      "XX-2.1",
			Description: "Qux Functional Test",
		},
		"foo/bar/tests/quuz_test": {
			PlanId:      "XX-2.2",
			Description: "Quuz Functional Test",
		},
	}

	// Populate the featuredir hierarchy with the README.md files and a dummy test file.
	featuredir := t.TempDir()
	for reldir, md := range markdowns {
		testdir := filepath.Join(featuredir, reldir)
		if err := os.MkdirAll(testdir, 0700); err != nil {
			t.Fatalf("Cannot create directories: %s", testdir)
		}

		readme := fmt.Sprintf("# %s: %s\n", md.PlanId, md.Description)
		readmeFilename := filepath.Join(testdir, "README.md")
		if err := os.WriteFile(readmeFilename, []byte(readme), 0600); err != nil {
			t.Fatalf("Could not write %s: %v", readmeFilename, err)
		}

		pkg := filepath.Base(reldir)
		testmain := fmt.Sprintf(`package %s

import testing

func TestMain(m *testing.M) {
  os.Exit(m.Run())
}
`, pkg)
		testmainFilename := fmt.Sprintf("%s/%s_test.go", testdir, pkg)
		if err := os.WriteFile(testmainFilename, []byte(testmain), 0600); err != nil {
			t.Fatalf("Could not write %s: %v", testmainFilename, err)
		}
	}

	ts := make(testsuite)
	if !ts.read(featuredir) {
		t.Fatalf("Could not read: %s", featuredir)
	}

	// Not doing ts.check() yet because it will flag that rundata need update, which is true
	// because we've only populated the README.md, not the rundata in test code.

	// Check that the markdowns are read correctly.
	checkMarkdowns(t, featuredir, ts, markdowns)

	// Fix the rundata and write it back.
	if !ts.fix() {
		t.Fatal("Could not fix testsuite.")
	}
	if err := ts.write(featuredir); err != nil {
		t.Fatal("Could not write testsuite:", err)
	}

	// Read the fixed rundata and make sure check now succeeds.
	newts := make(testsuite)
	if !newts.read(featuredir) {
		t.Fatalf("Could not read: %s", featuredir)
	}
	checkMarkdowns(t, featuredir, ts, markdowns)
	if !newts.check(featuredir) {
		t.Errorf("Check failed after fixing and writing back.")
	}
}
