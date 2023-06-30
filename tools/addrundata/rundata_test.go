package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	mpb "github.com/openconfig/featureprofiles/proto/metadata_go_proto"
)

var pdopt = protocmp.Transform()

func TestParseMarkdown(t *testing.T) {
	want := &mpb.Metadata{
		PlanId:      "XX-1.1",
		Description: "Foo Functional Test",
	}

	cases := []struct {
		name, heading string
	}{
		{name: "standard", heading: "# XX-1.1: Foo Functional Test"},
		{name: "excess spaces", heading: "#  XX-1.1 : Foo Functional Test  "},
		{name: "no space", heading: "#XX-1.1:Foo Functional Test"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			const tmpl = `%s

## Summary

## Procedure
`
			data := fmt.Sprintf(tmpl, c.heading)
			got, err := parseMarkdown(bytes.NewReader([]byte(data)))
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(want, got, pdopt); diff != "" {
				t.Errorf("parseMarkdown -want,+got:\n%s", diff)
			}
		})
	}
}

func TestWriteProto(t *testing.T) {
	want := &mpb.Metadata{
		Uuid:        "123e4567-e89b-42d3-8456-426614174000",
		PlanId:      "XX-1.1",
		Description: "Foo Functional Test",
	}
	buf := &bytes.Buffer{}
	if err := writeProto(buf, want); err != nil {
		t.Fatalf("Cannot write: %v", err)
	}
	gotText := buf.String()

	const wantHeader = `# proto-file: github.com/openconfig/featureprofiles/proto/metadata.proto
# proto-message: Metadata

`
	if !strings.HasPrefix(gotText, wantHeader) {
		t.Errorf("writeProto got %q, want header %q", gotText, wantHeader)
	}
	if wantNewLines, gotNewLines := 6, strings.Count(gotText, "\n"); wantNewLines != gotNewLines {
		t.Errorf("writeProto got %q with %v new lines, want %v new lines", gotText, gotNewLines, wantNewLines)
	}

	got, err := parseProto(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Cannot read back: %v", err)
	}
	if diff := cmp.Diff(want, got, pdopt); diff != "" {
		t.Errorf("writeProto -want,+got:\n%s", diff)
	}
}
