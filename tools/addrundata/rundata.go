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
	for sc.Scan() {
		if line := sc.Text(); line != "func init() {" {
			continue
		}
		if err := pd.parseInit(sc); err != nil {
			return err
		}
		break
	}
	if err := sc.Err(); err != nil {
		return err
	}
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

var tmpl = template.Must(template.New("rundata_test.go").Parse(
	`// Code generated by go run tools/addrundata; DO NOT EDIT.
package {{.Package}}

import "github.com/openconfig/featureprofiles/internal/rundata"

func init() {
{{range .Data}}	rundata.{{.Key}} = {{printf "%q\n" .Value}}{{end -}}
}
`))

// write generates a complete rundata_test.go to the writer.
func (pd *parsedData) write(w io.Writer, pkg string) error {
	tmpl.Execute(w, &struct {
		Package string
		Data    []struct{ Key, Value string }
	}{
		Package: pkg,
		Data: []struct{ Key, Value string }{
			{"TestPlanID", pd.testPlanID},
			{"TestDescription", pd.testDescription},
			{"TestUUID", pd.testUUID},
		},
	})
	return nil
}
