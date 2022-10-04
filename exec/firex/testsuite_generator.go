// Package main provides main functions to generate test runner for firex.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// GoTest represents a single go test
type GoTest struct {
	Name       string
	Path       string
	Patch      string
	Args       []string
	Timeout    int
	ShouldFail bool
}

// FirexTest represents a single firex test suite
type FirexTest struct {
	Name     string
	Owner    string
	Priority string
	Timeout  int
	Pyvxr    struct {
		Topology string
	}
	Testbed   string
	Binding   string
	Baseconf  string
	Pretests  []GoTest
	Posttests []GoTest
	Tests     []GoTest
}

var (
	testDescFilesFlag = flag.String(
		"test_desc_files", "", "comma separated list of test description yaml files.",
	)

	testNamesFlag = flag.String(
		"test_names", "", "comma separated list of tests to include",
	)

	workspaceFlag = flag.String(
		"workspace", "", "workspace used for firex launch.",
	)

	testDescFiles []string

	testNames []string

	workspace string
)

var (
	firexSuiteTemplate = template.Must(template.New("firexTestSuite").Funcs(template.FuncMap{
		"join": strings.Join,
	}).Parse(`
{{- range $i, $ft := $.TestSuite }}
{{- if gt (len $ft.Tests) 0 }}
{{- .Name }}:
    framework: b4_fp
    owners:
        - {{ $ft.Owner }}
    pip_pkgs:
        - ixnetwork-restpy
        - protobuf
    {{- if eq $ft.Priority "low" }}
    priority: BCT
    {{- else if eq $ft.Priority "high" }}
    priority: UT
    {{- end }}
    {{- if $ft.Pyvxr.Topology }}
    plugins:
        - vxsim.py
    topo_file: {{ $.Workspace }}/{{ $ft.Pyvxr.Topology }}
    {{- end }}
    {{- if gt $ft.Timeout 0 }}
    plugins:
        - change_inactivity_timeout.py
    changed_inactivity_timeout: {{ $ft.Timeout }}
    {{- end }}
    ondatra_testbed_path: {{ $ft.Testbed }}
    {{- if $ft.Binding }}
    ondatra_binding_path: {{ $ft.Binding }}
    {{- end }}
    {{- if $ft.Baseconf }}
    base_conf_path: {{ $ft.Baseconf }}
    {{- end }}
    supported_platforms:
        - "8000"
    fp_pre_tests:
        {{- range $j, $gt := $ft.Pretests}}
        - {{ $gt.Name }}:
            test_path: {{ $gt.Path }}
            {{- if $gt.Args }}
            test_args: {{ join $gt.Args " " }}
            {{- end }}
        {{- end }}
    script_paths:
        {{- range $j, $gt := $ft.Tests}}
        - {{ $gt.Name }}{{ if $gt.Patch }} (Patched){{ end }}:
            test_path: {{ $gt.Path }}
            {{- if $gt.Args }}
            test_args: {{ join $gt.Args " " }}
            {{- end }}
            {{- if $gt.Patch }}
            test_patch: {{ $gt.Patch }}
            {{- end }}
            test_timeout: {{ $gt.Timeout }}
        {{- end }}
    fp_post_tests:
        {{- range $j, $gt := $ft.Posttests}}
        - {{ $gt.Name }}:
            test_path: {{ $gt.Path }}
            {{- if $gt.Args }}
            test_args: {{ join $gt.Args " " }}
            {{- end }}
        {{- end }}
    smart_sanity_exclude: True
{{- end }}
{{ end }}
`))
)

func init() {
	flag.Parse()
	if *testDescFilesFlag == "" {
		log.Fatal("test_desc_files must be set.")
	}
	testDescFiles = strings.Split(*testDescFilesFlag, ",")
	workspace = *workspaceFlag

	if len(*testNamesFlag) > 0 {
		testNames = strings.Split(*testNamesFlag, ",")
	}
}

func main() {
	suite := []FirexTest{}

	for _, f := range testDescFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			log.Fatalf("Error reading test file %v", err)
		}

		t := FirexTest{}
		err = yaml.Unmarshal(data, &t)
		if err != nil {
			log.Fatalf("Error unmarshaling test file: %v", err)
		}
		suite = append(suite, t)
	}

	// remove untargeted tests
	if len(testNames) > 0 {
		targetedTests := map[string]bool{}
		for _, t := range testNames {
			targetedTests[strings.Split(t, " ")[0]] = true
		}

		for i := range suite {
			keptTests := []GoTest{}
			for j := range suite[i].Tests {
				prefix := strings.Split(suite[i].Tests[j].Name, " ")[0]
				if _, found := targetedTests[prefix]; found {
					keptTests = append(keptTests, suite[i].Tests[j])
				}
			}
			suite[i].Tests = keptTests
		}
	}

	// adjust timeouts
	for i := range suite {
		if suite[i].Timeout > 0 {
			for j := range suite[i].Tests {
				if suite[i].Tests[j].Timeout == 0 {
					suite[i].Tests[j].Timeout = suite[i].Timeout
				}
			}
		}
	}

	for i := range suite {
		maxTestTimeout := 0
		for j := range suite[i].Tests {
			if maxTestTimeout < suite[i].Tests[j].Timeout {
				maxTestTimeout = suite[i].Tests[j].Timeout
			}
		}
		suite[i].Timeout = 2 * maxTestTimeout
	}

	var testSuiteCode strings.Builder
	firexSuiteTemplate.Execute(&testSuiteCode, struct {
		TestSuite []FirexTest
		Workspace string
	}{
		TestSuite: suite,
		Workspace: workspace,
	})

	fmt.Printf("%v", testSuiteCode.String())
}
