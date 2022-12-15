// Package main provides main functions to generate test runner for firex.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// GoTest represents a single go test
type GoTest struct {
	ID        string
	Name      string
	Owner     string
	Priority  int
	Path      string
	Patch     string
	Testbed   string
	Binding   string
	Baseconf  string
	Topology  string
	Args      []string
	Timeout   int
	Skip      bool
	MustPass  bool
	Pretests  []GoTest
	Posttests []GoTest
}

// FirexTest represents a single firex test suite
type FirexTest struct {
	Name      string
	Owner     string
	Priority  int
	Timeout   int
	Skip      bool
	Topology  string
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

	pluginsFlag = flag.String(
		"extra_plugins", "", "comma separated list of extra firex plugins",
	)

	topologyFlag = flag.String(
		"topology", "", "custom pyvxr topology",
	)

	bindingFlag = flag.String(
		"binding", "", "custom ondatra binding",
	)

	testbedFlag = flag.String(
		"testbed", "", "custom ondatra testbed",
	)

	baseconfFlag = flag.String(
		"baseconf", "", "custom ondatra baseconf",
	)

	outDirFlag = flag.String(
		"out_dir", "", "output directory",
	)

	patchedOnlyFlag = flag.Bool(
		"patched_only", false, "include only patched test",
	)

	excludePatchedFlag = flag.Bool(
		"exclude_patched", false, "exclude patched test",
	)

	mustPassOnlyFlag = flag.Bool(
		"must_pass_only", false, "include only mustpass test",
	)

	testDescFiles  []string
	testNames      []string
	extraPlugins   []string
	topology       string
	binding        string
	testbed        string
	baseconf       string
	outDir         string
	patchedOnly    bool
	mustPassOnly   bool
	excludePatched bool
)

var (
	firexSuiteTemplate = template.Must(template.New("firexTestSuite").Funcs(template.FuncMap{
		"join": strings.Join,
		"hasDeviation": func(gt GoTest) bool {
			for _, arg := range gt.Args {
				if strings.HasPrefix(arg, "-deviation") {
					return true
				}
			}
			return false
		},
	}).Parse(`
{{ $.Test.Name }}:
    framework: b4_fp
    owners:
        - {{ $.Test.Owner }}
    {{- if gt (len $.Plugins) 0 }}
    plugins:
    {{- range $k, $pl := $.Plugins }}
        - {{ $pl }}
    {{- end }}
    {{- end }}
	{{- if $.Test.Topology }}
    topo_file: {{ $.Test.Topology }}
    {{- end }}
    {{- if $.Test.Testbed }}
    ondatra_testbed_path: {{ $.Test.Testbed }}
    {{- end }}
    {{- if $.Test.Binding }}
    ondatra_binding_path: {{ $.Test.Binding }}
    {{- else }}
    ondatra_binding_path: ""
    {{- end }}
    {{- if $.Test.Baseconf }}
    base_conf_path: {{ $.Test.Baseconf }}
    {{- else }}
    base_conf_path: ""
    {{- end }}
    supported_platforms:
        - "8000"
    fp_pre_tests:
        {{- range $j, $pt := $.Test.Pretests}}
        - {{ $pt.Name }}:
            test_path: {{ $pt.Path }}
            {{- if $pt.Args }}
            test_args: {{ join $pt.Args " " }}
            {{- end }}
        {{- end }}
    script_paths:
        - ({{ $.Test.ID }}) {{ $.Test.Name }}{{ if $.Test.Patch }} (Patched){{ end }}{{ if hasDeviation $.Test }} (Deviation){{ end }}{{ if $.Test.MustPass }} (MP){{ end }}:
            test_path: {{ $.Test.Path }}
            {{- if $.Test.Args }}
            test_args: {{ join $.Test.Args " " }}
            {{- end }}
            {{- if $.Test.Patch }}
            test_patch: {{ $.Test.Patch }}
            {{- end }}
            test_timeout: {{ $.Test.Timeout }}
    fp_post_tests:
        {{- range $j, $pt := $.Test.Posttests}}
        - {{ $pt.Name }}:
            test_path: {{ $pt.Path }}
            {{- if $pt.Args }}
            test_args: {{ join $pt.Args " " }}
            {{- end }}
        {{- end }}
    smart_sanity_exclude: True
`))
)

func init() {
	flag.Parse()
	if *testDescFilesFlag == "" {
		log.Fatal("test_desc_files must be set.")
	}
	testDescFiles = strings.Split(*testDescFilesFlag, ",")

	if len(*testNamesFlag) > 0 {
		testNames = strings.Split(*testNamesFlag, ",")
	}

	if len(*pluginsFlag) > 0 {
		extraPlugins = strings.Split(*pluginsFlag, ",")
	}

	if len(*topologyFlag) > 0 {
		topology = *topologyFlag
	}

	if len(*bindingFlag) > 0 {
		binding = *bindingFlag
	}

	if len(*testbedFlag) > 0 {
		testbed = *testbedFlag
	}

	if len(*baseconfFlag) > 0 {
		baseconf = *baseconfFlag
	}

	if len(*outDirFlag) > 0 {
		outDir = *outDirFlag
	}

	mustPassOnly = *mustPassOnlyFlag
	patchedOnly = *patchedOnlyFlag
	excludePatched = *excludePatchedFlag
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

	// Targeted mode: remove untargeted tests
	if len(testNames) > 0 {
		targetedTests := map[string]bool{}
		res := []*regexp.Regexp{}
		for _, t := range testNames {
			if strings.HasPrefix(t, "r/") {
				res = append(res, regexp.MustCompile(t[2:]))
			} else {
				targetedTests[strings.Split(t, " ")[0]] = true
			}
		}

		for i := range suite {
			keptTests := []GoTest{}
			for j := range suite[i].Tests {
				prefix := strings.Split(suite[i].Tests[j].Name, " ")[0]
				if _, found := targetedTests[prefix]; found {
					keptTests = append(keptTests, suite[i].Tests[j])
				} else {
					for _, re := range res {
						if re.MatchString(suite[i].Tests[j].Name) {
							keptTests = append(keptTests, suite[i].Tests[j])
							break
						}
					}
				}
			}
			suite[i].Tests = keptTests
		}
	} else {
		// Normal mode: remove skipped tests
		for i := range suite {
			keptTests := []GoTest{}
			for j := range suite[i].Tests {
				if !suite[i].Tests[j].Skip &&
					(!mustPassOnly || suite[i].Tests[j].MustPass) &&
					(!patchedOnly || suite[i].Tests[j].Patch != "") &&
					(!excludePatched || suite[i].Tests[j].Patch == "") {
					keptTests = append(keptTests, suite[i].Tests[j])
				}
			}
			suite[i].Tests = keptTests
		}

		kepSuite := []FirexTest{}
		for i := range suite {
			if !suite[i].Skip && len(suite[i].Tests) > 0 {
				kepSuite = append(kepSuite, suite[i])
			}
		}
		suite = kepSuite
	}

	// adjust timeouts, priorities, & owners
	for i := range suite {
		if suite[i].Priority == 0 {
			suite[i].Priority = 100000000
		}

		for j := range suite[i].Tests {
			if suite[i].Tests[j].Priority == 0 {
				suite[i].Tests[j].Priority = 100000000
			}

			if suite[i].Timeout > 0 && suite[i].Tests[j].Timeout == 0 {
				suite[i].Tests[j].Timeout = suite[i].Timeout
			}

			if len(suite[i].Owner) > 0 && len(suite[i].Tests[j].Owner) == 0 {
				suite[i].Tests[j].Owner = suite[i].Owner
			}

			if len(suite[i].Baseconf) > 0 && len(suite[i].Tests[j].Baseconf) == 0 {
				suite[i].Tests[j].Baseconf = suite[i].Baseconf
			}

			if len(suite[i].Testbed) > 0 && len(suite[i].Tests[j].Testbed) == 0 {
				suite[i].Tests[j].Testbed = suite[i].Testbed
			}

			if len(suite[i].Binding) > 0 && len(suite[i].Tests[j].Binding) == 0 {
				suite[i].Tests[j].Binding = suite[i].Binding
			}

			if len(suite[i].Topology) > 0 && len(suite[i].Tests[j].Topology) == 0 {
				suite[i].Tests[j].Topology = suite[i].Topology
			}

			if len(suite[i].Tests[j].Pretests) == 0 {
				suite[i].Tests[j].Pretests = append(suite[i].Tests[j].Pretests, suite[i].Pretests...)
			}

			if len(suite[i].Tests[j].Posttests) == 0 {
				suite[i].Tests[j].Posttests = append(suite[i].Tests[j].Posttests, suite[i].Posttests...)
			}
		}
	}

	// sort by priority
	for _, suite := range suite {
		sort.Slice(suite.Tests, func(i, j int) bool {
			return suite.Tests[i].Priority < suite.Tests[j].Priority
		})
	}

	sort.Slice(suite, func(i, j int) bool {
		return suite[i].Priority < suite[j].Priority
	})

	// Assign ids to tests
	numTestCases := 1
	for i := range suite {
		numTestCases += len(suite[i].Tests)
	}

	id := 1
	widthNeeded := len(fmt.Sprint(numTestCases))
	for i := range suite {
		for j := range suite[i].Tests {
			suite[i].Tests[j].ID = fmt.Sprintf("%0"+fmt.Sprint(widthNeeded)+"d", id)
			id = id + 1
		}
	}

	var testSuiteCode strings.Builder

	for i := range suite {
		for j := range suite[i].Tests {
			if len(topology) > 0 {
				suite[i].Tests[j].Binding = ""
				suite[i].Tests[j].Testbed = ""
				suite[i].Tests[j].Baseconf = ""
				suite[i].Tests[j].Topology = topology
			}
			if len(testbed) > 0 {
				suite[i].Tests[j].Testbed = testbed
			}
			if len(baseconf) > 0 {
				suite[i].Tests[j].Baseconf = baseconf
			}
			if len(binding) > 0 {
				suite[i].Tests[j].Topology = ""
				suite[i].Tests[j].Binding = binding
			}

			if len(outDir) > 0 {
				testSuiteCode.Reset()
			}

			firexSuiteTemplate.Execute(&testSuiteCode, struct {
				Test    GoTest
				Plugins []string
			}{
				Test:    suite[i].Tests[j],
				Plugins: extraPlugins,
			})

			if len(outDir) > 0 {
				suiteFile := filepath.Join(outDir,
					strings.Split(strings.TrimSpace(suite[i].Tests[j].Name), " ")[0]+".yaml")
				os.WriteFile(suiteFile, []byte(testSuiteCode.String()), 0644)
			}
		}
	}

	if len(outDir) == 0 {
		fmt.Printf("%v", testSuiteCode.String())
	}
}
