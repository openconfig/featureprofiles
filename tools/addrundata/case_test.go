package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	mpb "github.com/openconfig/featureprofiles/proto/metadata_go_proto"
	"google.golang.org/protobuf/testing/protocmp"
)

const (
	markdownText = `# XX-1.1: Description from markdown

## Summary

## Procedure
`
	metadataText = `# proto/metadata.proto
uuid: "cb772d39-4f2d-41d2-b286-bca33101d575"
plan_id: "XY-1.1"
description: "Description from proto"
`
)

var tcopts = []cmp.Option{cmp.AllowUnexported(testcase{}), protocmp.Transform()}

func TestCase_Read(t *testing.T) {
	tests := []struct {
		desc         string
		markdownText string
		metadataText string
		want         testcase
		wantErr      string
	}{{
		desc:    "empty",
		wantErr: "no such file",
	}, {
		desc:         "bad markdown",
		markdownText: "~!@#$%^&*()_+",
		wantErr:      "parse markdown",
	}, {
		desc:         "good markdown",
		markdownText: markdownText,
		want: testcase{
			markdown: &mpb.Metadata{
				PlanId:      "XX-1.1",
				Description: "Description from markdown",
			},
		},
	}, {
		desc:         "bad metadata",
		markdownText: markdownText,
		metadataText: "~!@#$%^&*()_+",
		wantErr:      "parse metadata",
	}, {
		desc:         "good metadata",
		markdownText: markdownText,
		metadataText: metadataText,
		want: testcase{
			markdown: &mpb.Metadata{
				PlanId:      "XX-1.1",
				Description: "Description from markdown",
			},
			existing: &mpb.Metadata{
				Uuid:        "cb772d39-4f2d-41d2-b286-bca33101d575",
				PlanId:      "XY-1.1",
				Description: "Description from proto",
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			testdir := t.TempDir()
			for fname, fdata := range map[string]string{
				"README.md":          test.markdownText,
				"metadata.textproto": test.metadataText,
			} {
				if fdata != "" {
					if err := os.WriteFile(filepath.Join(testdir, fname), []byte(fdata), 0600); err != nil {
						t.Fatalf("Could not write %s: %v", fname, err)
					}
				}
			}

			var got testcase
			err := got.read(testdir)
			if (err == nil) != (test.wantErr == "") || (err != nil && !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("testcase.read got error %v, want error containing %q:", err, test.wantErr)
			}
			if err != nil {
				return
			}
			if diff := cmp.Diff(test.want, got, tcopts...); diff != "" {
				t.Errorf("testcase.read -want,+got:\n%s", diff)
			}
		})
	}
}

func TestCase_Check(t *testing.T) {
	cases := []struct {
		name string
		tc   testcase
		want int
	}{{
		name: "good",
		tc: testcase{
			markdown: &mpb.Metadata{
				PlanId:      "XX-1.1",
				Description: "Foo Functional Test",
			},
			existing: &mpb.Metadata{
				Uuid:        "123e4567-e89b-42d3-8456-426614174000",
				PlanId:      "XX-1.1",
				Description: "Foo Functional Test",
				Testbed:     mpb.Metadata_TESTBED_DUT_ATE_4LINKS,
			},
		},
		want: 0,
	}, {
		name: "allbad",
		tc: testcase{
			markdown: &mpb.Metadata{
				PlanId:      "XX-1.1",
				Description: "Description from Markdown",
			},
			existing: &mpb.Metadata{
				Uuid:        "123e4567-e89b-12d3-a456-426614174000",
				PlanId:      "YY-1.1",
				Description: "Description from Test",
			},
		},
		want: 4,
	}, {
		name: "noexisting",
		tc: testcase{
			markdown: &mpb.Metadata{
				PlanId:      "XX-1.1",
				Description: "Foo Functional Test",
			},
		},
		want: 1,
	}, {
		name: "nodata",
		tc:   testcase{},
		want: 2,
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			errs := c.tc.check()
			t.Logf("Errors from check: %#q", errs)
			if got := len(errs); got != c.want {
				t.Errorf("Number of errors from check got %d, want %d.", got, c.want)
			}
		})
	}
}

func TestCase_Fix(t *testing.T) {
	tc := testcase{
		markdown: &mpb.Metadata{
			PlanId:      "XX-1.1",
			Description: "Foo Functional Test",
		},
	}
	if err := tc.fix(); err != nil {
		t.Fatal(err)
	}
	got := tc.fixed
	want := &mpb.Metadata{
		Uuid:        got.Uuid,
		PlanId:      tc.markdown.PlanId,
		Description: tc.markdown.Description,
		Testbed:     mpb.Metadata_TESTBED_DUT_ATE_2LINKS,
	}
	if diff := cmp.Diff(want, got, tcopts...); diff != "" {
		t.Errorf("fixed -want,+got:\n%s", diff)
	}
}

func TestCase_FixUUID(t *testing.T) {
	tc := testcase{
		markdown: &mpb.Metadata{
			PlanId:      "XX-1.1",
			Description: "Foo Functional Test",
		},
		existing: &mpb.Metadata{
			Testbed: mpb.Metadata_TESTBED_DUT,
			Uuid:    "urn:uuid:123e4567-e89b-42d3-8456-426614174000",
		},
	}
	if err := tc.fix(); err != nil {
		t.Fatal(err)
	}
	got := tc.fixed
	want := &mpb.Metadata{
		Uuid:        "123e4567-e89b-42d3-8456-426614174000",
		PlanId:      tc.markdown.PlanId,
		Description: tc.markdown.Description,
		Testbed:     mpb.Metadata_TESTBED_DUT,
	}
	if diff := cmp.Diff(want, got, tcopts...); diff != "" {
		t.Errorf("fixed -want,+got:\n%s", diff)
	}
}

func TestCase_FixWithPlatformExceptions(t *testing.T) {
	tc := testcase{
		markdown: &mpb.Metadata{
			PlanId:      "XX-1.1",
			Description: "Foo Functional Test",
		},
		existing: &mpb.Metadata{
			Testbed: mpb.Metadata_TESTBED_DUT,
			PlatformExceptions: []*mpb.Metadata_PlatformExceptions{
				{
					Platform: &mpb.Metadata_Platform{
						HardwareModelRegex: "8808",
					},
					Deviations: &mpb.Metadata_Deviations{
						Ipv4MissingEnabled: true,
					},
				},
			},
			Tags: []mpb.Metadata_Tags{mpb.Metadata_TAGS_AGGREGATION},
		},
	}
	if err := tc.fix(); err != nil {
		t.Fatal(err)
	}
	got := tc.fixed
	want := &mpb.Metadata{
		Uuid:        got.Uuid,
		PlanId:      tc.markdown.PlanId,
		Description: tc.markdown.Description,
		Testbed:     mpb.Metadata_TESTBED_DUT,
		PlatformExceptions: []*mpb.Metadata_PlatformExceptions{
			{
				Platform: &mpb.Metadata_Platform{
					HardwareModelRegex: "8808",
				},
				Deviations: &mpb.Metadata_Deviations{
					Ipv4MissingEnabled: true,
				},
			},
		},
		Tags: []mpb.Metadata_Tags{mpb.Metadata_TAGS_AGGREGATION},
	}
	if diff := cmp.Diff(want, got, tcopts...); diff != "" {
		t.Errorf("fixed -want,+got:\n%s", diff)
	}
}

func TestCase_Write(t *testing.T) {
	var want, got testcase

	// Prepare a testdir with just README.md
	testdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(testdir, "README.md"), []byte(markdownText), 0600); err != nil {
		t.Fatal(err)
	}

	// Read, fix, and write.
	if err := want.read(testdir); err != nil {
		t.Fatal(err)
	}
	if err := want.fix(); err != nil {
		t.Fatal(err)
	}
	if err := want.write(testdir); err != nil {
		t.Fatal(err)
	}

	// Read it back to ensure we got the same data.
	if err := got.read(testdir); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want.fixed, got.existing, tcopts...); diff != "" {
		t.Errorf("Write then read output differs -want,+got:\n%s", diff)
	}
}
