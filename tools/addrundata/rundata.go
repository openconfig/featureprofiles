package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	mpb "github.com/openconfig/featureprofiles/proto/metadata_go_proto"
	"google.golang.org/protobuf/encoding/prototext"
)

type parsedData struct {
	testPlanID      string
	testDescription string
	testUUID        string
	hasData         bool
}

// markdownRE matches the heading line: `# XX-1.1: Foo Functional Test`
var markdownRE = regexp.MustCompile(`#(.*?):(.*)`)

// fromMarkdown reads parsedData from README.md
func (pd *parsedData) fromMarkdown(r io.Reader) error {
	sc := bufio.NewScanner(r)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return err
		}
		return errors.New("missing markdown heading")
	}
	line := sc.Text()
	m := markdownRE.FindStringSubmatch(line)
	if len(m) < 3 {
		return fmt.Errorf("cannot parse markdown: %s", line)
	}
	pd.testPlanID = strings.TrimSpace(m[1])
	pd.testDescription = strings.TrimSpace(m[2])
	pd.hasData = true
	return nil
}

// fromCode reads parsedData from a source code.
func (pd *parsedData) fromCode(r io.Reader) error {
	sc := bufio.NewScanner(r)
	var foundInit bool
	for sc.Scan() {
		if line := sc.Text(); line != "func init() {" {
			continue
		}
		foundInit = true
		if err := pd.parseInit(sc); err != nil {
			return err
		}
		break
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if !foundInit {
		return errors.New("missing func init()")
	}
	return nil
}

// fromCode reads parsedData from a textproto.
func (pd *parsedData) fromProto(r io.Reader) error {
	bytes, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	md := new(mpb.Metadata)
	if err := prototext.Unmarshal(bytes, md); err != nil {
		return err
	}
	if uuid := md.GetUuid(); uuid != "" {
		pd.testUUID = uuid
	}
	if planID := md.GetPlanId(); planID != "" {
		pd.testPlanID = planID
	}
	if desc := md.GetDescription(); desc != "" {
		pd.testDescription = desc
	}
	pd.hasData = true
	return nil
}

// rundataRE matches a line like this: `  rundata.TestUUID = "..."`
var rundataRE = regexp.MustCompile(`\s+rundata\.(\w+) = (".*")`)

// parseInit parses the rundata from the body of func init().
func (pd *parsedData) parseInit(sc *bufio.Scanner) error {
	for sc.Scan() {
		line := sc.Text()
		if line == "}" {
			pd.hasData = true
			return nil
		}
		m := rundataRE.FindStringSubmatch(line)
		if len(m) < 3 {
			continue
		}
		k := m[1]
		v, err := strconv.Unquote(m[2])
		if err != nil {
			return fmt.Errorf("cannot parse rundata line: %s: %w", line, err)
		}
		switch k {
		case "TestPlanID":
			pd.testPlanID = v
		case "TestDescription":
			pd.testDescription = v
		case "TestUUID":
			pd.testUUID = v
		}
	}
	return errors.New("func init() was not terminated")
}

var tmpl = template.Must(template.New("metadata.textproto").Parse(
	`# proto-file: proto/metadata.proto
# proto-message: Metadata

uuid: "{{.UUID}}"
plan_id: "{{.PlanID}}"
description: "{{.Description}}"
`))

// write generates a complete metadata.textproto to the writer.
func (pd *parsedData) write(w io.Writer) error {
	return tmpl.Execute(w, struct {
		UUID, PlanID, Description string
	}{
		UUID:        pd.testUUID,
		PlanID:      pd.testPlanID,
		Description: pd.testDescription,
	})
}
