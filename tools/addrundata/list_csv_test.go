package main

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	mpb "github.com/openconfig/featureprofiles/proto/metadata_go_proto"
)

func TestListCSV(t *testing.T) {
	ts := testsuite{
		"feature/foo/bar/ate_tests/qux_test": &testcase{
			markdown: &mpb.Metadata{
				PlanId:      "YY-2.1",
				Description: "Qux Functional Test",
			},
		},
		"feature/foo/bar/otg_tests/qux_test": &testcase{
			markdown: &mpb.Metadata{
				PlanId:      "YY-2.1",
				Description: "Qux Functional Test",
			},
		},
		"feature/foo/baz/quuz_test": &testcase{
			markdown: &mpb.Metadata{
				PlanId:      "XX-1.1",
				Description: "Quuz Functional Test",
			},
		},
	}

	const want = `Feature,ID,Desc,Test Path
foo/baz,XX-1.1,Quuz Functional Test,feature/foo/baz/quuz_test
foo/bar,YY-2.1,Qux Functional Test,feature/foo/bar/ate_tests/qux_test
foo/bar,YY-2.1,Qux Functional Test,feature/foo/bar/otg_tests/qux_test
`

	var buf strings.Builder
	if err := listCSV(&buf, "", ts); err != nil {
		t.Fatal("Could not write CSV:", err)
	}
	got := buf.String()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("listCSV -want,+got:\n%s", diff)
	}
}

func TestFeatureFromTestDir(t *testing.T) {
	cases := []struct {
		testdir string
		want    string
	}{
		{"feature/foo/bar/ate_tests/qux_test", "feature/foo/bar"},
		{"feature/foo/bar/tests/qux_test", "feature/foo/bar"},
		{"feature/foo/baz/quuz_test", "feature/foo/baz"},
	}
	for _, c := range cases {
		got := featureFromTestDir(c.testdir)
		if got != c.want {
			t.Errorf("featureFromTestDir(%q) got %q, want %q", c.testdir, got, c.want)
		}
	}
}

func TestSortedByTestPlanID(t *testing.T) {
	ts := testsuite{
		"feature/foo/bar/ate_tests/qux_test": &testcase{
			markdown: &mpb.Metadata{PlanId: "YY-2.1"},
		},
		"feature/foo/bar/otg_tests/qux_test": &testcase{
			markdown: &mpb.Metadata{PlanId: "YY-2.1"},
		},
		"feature/foo/baz/quuz_test": &testcase{
			markdown: &mpb.Metadata{PlanId: "XX-1.1"},
		},
	}

	want := []string{
		"feature/foo/baz/quuz_test",          // XX-1.1
		"feature/foo/bar/ate_tests/qux_test", // YY-2.1, ate_tests
		"feature/foo/bar/otg_tests/qux_test", // YY-2.1, otg_tests
	}

	got := sortedByTestPlanID(ts)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("sortedByTestPlanID -want,+got:\n%s", diff)
	}
}
