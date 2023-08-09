// Package main provides main functions to generate test runner for firex.
package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

// GoTest represents a single go test
type GoTest struct {
	ID              string
	Name            string
	ShortName       string
	Owners          []string
	Priority        int
	Path            string
	Branch          string
	Revision        string
	PrNum           int `yaml:"pr"`
	Testbeds        []string
	TestbedsInclude []string `yaml:"testbeds_include"`
	TestbedsExclude []string `yaml:"testbeds_exclude"`
	Args            []string
	Timeout         int
	Skip            bool
	MustPass        bool
	HasDeviations   bool
	Internal        bool
	Pretests        []GoTest
	Posttests       []GoTest
	Groups          []string
}

// FirexTest represents a single firex test suite
type FirexTest struct {
	Name      string
	Owners    []string
	Branch    string
	Revision  string
	PrNum     int `yaml:"pr"`
	Priority  int
	Timeout   int
	Skip      bool
	Internal  bool
	Testbeds  []string
	Pretests  []GoTest
	Posttests []GoTest
	Tests     []GoTest
	Groups    []string
}

var (
	filesFlag = flag.String(
		"files", "", "comma separated list of test description yaml files.",
	)

	testNamesFlag = flag.String(
		"test_names", "", "comma separated list of tests to include",
	)

	groupNamesFlag = flag.String(
		"group_names", "", "comma separated list of test groups to include",
	)

	exTestNamesFlag = flag.String(
		"exclude_test_names", "", "comma separated list of tests to exclude",
	)

	pluginsFlag = flag.String(
		"extra_plugins", "", "comma separated list of extra firex plugins",
	)

	envFlag = flag.String(
		"env", "", "comma separated list of env variables",
	)

	testbedsFlag = flag.String(
		"testbeds", "", "custom comma seperated list of testbeds",
	)

	outDirFlag = flag.String(
		"out_dir", "", "output directory",
	)

	internalRepoRevFlag = flag.String(
		"internal_repo_rev", "", "internal fp repo rev to use for firex data",
	)

	testRepoRevFlag = flag.String(
		"test_repo_rev", "", "fp repo rev to use for test execution",
	)

	defaultTestRepoRevFlag = flag.String(
		"default_test_repo_rev", "", "fp repo rev to use for test execution by default",
	)

	testNamePrefixFlag = flag.String(
		"test_name_prefix", "", "prefix to pre-append to test name",
	)

	showTestbedsFlag = flag.Bool(
		"show_testbeds", false, "just output the testbeds used",
	)

	mustPassOnlyFlag = flag.Bool(
		"must_pass_only", false, "include only mustpass test",
	)

	ignorePatchedFlag = flag.Bool(
		"ignore_patched", false, "include only tests with no pr, branch, or rev",
	)

	randomizeFlag = flag.Bool(
		"randomize", false, "randomize tests order",
	)

	sortFlag = flag.Bool(
		"sort", true, "sort tests by priority",
	)

	useShortNameFlag = flag.Bool(
		"use_short_names", false, "output short test names",
	)

	ignoreDeviationsFlag = flag.Bool(
		"ignore_deviations", false, "ignore all deviation flags",
	)

	files              []string
	testNames          []string
	groupNames         []string
	excludeTestNames   []string
	extraPlugins       []string
	testbeds           []string
	env                map[string]string
	outDir             string
	testRepoRev        string
	defaultTestRepoRev string
	internalRepoRev    string
	testNamePrefix     string
	showTestbeds       bool
	mustPassOnly       bool
	ignorePatched      bool
	randomize          bool
	sorted             bool
	useShortName       bool
	ignoreDeviations   bool
)

var (
	firexSuiteTemplate = template.Must(template.New("firexTestSuite").Funcs(template.FuncMap{
		"join": strings.Join,
	}).Parse(`
{{ if $.UseShortTestNames}}{{ $.TestNamePrefix }}{{ $.Test.ShortName }}{{ else }}({{ $.Test.ID }}) {{ $.Test.Name }}{{ end }}:
    framework: b4
    owners:
    {{- range $k, $ow := $.Test.Owners }}
        - {{ $ow }}
    {{- end }}
    {{- if gt (len $.Plugins) 0 }}
    plugins:
    {{- range $k, $pl := $.Plugins }}
        - {{ $pl }}
    {{- end }}
    {{- end }}
    {{- if gt (len $.Env) 0 }}
    env:
    {{- range $k, $v := $.Env }}
        {{ $k }}: {{ $v }}
    {{- end }}
    {{- end }}
    testbeds:
    {{- range $k, $tb := $.Test.Testbeds }}
        - {{ $tb }}
    {{- end }}
    supported_platforms:
        - "8000"
    {{- if gt (len $.Test.Pretests) 0 }}
    fp_pre_tests:
        {{- range $j, $pt := $.Test.Pretests}}
        - {{ $pt.Name }}:
            test_path: {{ $pt.Path }}
            {{- if $pt.Args }}
            test_args: {{ join $pt.Args " " }}
            {{- end }}
        {{- end }}
    {{- end }}
    script_paths:
        {{- if $.UseShortTestNames}}
        - {{ $.TestNamePrefix }}{{ $.Test.ShortName }}:
        {{- else }}
        - ({{ $.Test.ID }}) {{ $.Test.Name }}{{ if $.Test.Branch }} ({{ if $.Test.Internal }}I-{{ end }}BR#{{ $.Test.Branch }}){{ end }}{{ if $.Test.PrNum }} ({{ if $.Test.Internal }}I-{{ end }}PR#{{ $.Test.PrNum }}){{ end }}{{ if $.Test.HasDeviations }} (Deviation){{ end }}{{ if $.Test.MustPass }} (MP){{ end }}:
        {{- end }}
            test_name: {{ $.Test.ShortName }}
            test_path: {{ $.Test.Path }}
            {{- if $.Test.Args }}
            test_args: {{ join $.Test.Args " " }}
            {{- end }}
            {{- if $.Test.Branch }}
            test_branch: {{ $.Test.Branch }}
            {{- end }}
            {{- if $.Test.Revision }}
            test_revision: {{ $.Test.Revision }}
            {{- end }}
            {{- if $.Test.PrNum }}
            test_pr: {{ $.Test.PrNum }}
            {{- end }}
            internal_test: {{ $.Test.Internal }}
            test_timeout: {{ $.Test.Timeout }}
            {{- if $.InternalRepoRev }}
            internal_fp_repo_rev: {{ $.InternalRepoRev}}
            {{- end }}
            smart_sanity_exclude: True
    {{- if gt (len $.Test.Posttests) 0 }}
    fp_post_tests:
        {{- range $j, $pt := $.Test.Posttests}}
        - {{ $pt.Name }}:
            test_path: {{ $pt.Path }}
            {{- if $pt.Args }}
            test_args: {{ join $pt.Args " " }}
            {{- end }}
        {{- end }}
    {{- end }}
`))
)

func init() {
	flag.Parse()
	if *filesFlag == "" {
		log.Fatal("-files must be set.")
	}
	files = strings.Split(*filesFlag, ",")

	if len(*testNamesFlag) > 0 {
		testNames = strings.Split(*testNamesFlag, ",")
	}

	if len(*groupNamesFlag) > 0 {
		groupNames = strings.Split(*groupNamesFlag, ",")
	}

	if len(*exTestNamesFlag) > 0 {
		excludeTestNames = strings.Split(*exTestNamesFlag, ",")
	}

	if len(*pluginsFlag) > 0 {
		extraPlugins = strings.Split(*pluginsFlag, ",")
	}

	if len(*envFlag) > 0 {
		env = make(map[string]string)
		for _, e := range strings.Split(*envFlag, ",") {
			keyValPair := strings.Split(e, "=")
			env[strings.TrimSpace(keyValPair[0])] = strings.TrimSpace(keyValPair[1])
		}
	}

	if len(*testbedsFlag) > 0 {
		testbeds = strings.Split(*testbedsFlag, ",")
	}

	if len(*outDirFlag) > 0 {
		outDir = *outDirFlag
	}

	if len(*internalRepoRevFlag) > 0 {
		internalRepoRev = *internalRepoRevFlag
	}

	if len(*testNamePrefixFlag) > 0 {
		testNamePrefix = *testNamePrefixFlag
	}

	if len(*testRepoRevFlag) > 0 {
		testRepoRev = *testRepoRevFlag
	}

	if len(*defaultTestRepoRevFlag) > 0 {
		defaultTestRepoRev = *defaultTestRepoRevFlag
	}

	showTestbeds = *showTestbedsFlag
	mustPassOnly = *mustPassOnlyFlag
	ignorePatched = *ignorePatchedFlag
	randomize = *randomizeFlag
	sorted = *sortFlag
	useShortName = *useShortNameFlag
	ignoreDeviations = *ignoreDeviationsFlag
}

func main() {
	suite := []FirexTest{}

	for _, f := range files {
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
		targetedTests := []string{}
		res := map[string]*regexp.Regexp{}
		for _, t := range testNames {
			targetedTests = append(targetedTests, strings.Split(t, " ")[0])
			if strings.HasPrefix(t, "r/") {
				res[t] = regexp.MustCompile(t[2:])
			}
		}

		keptTests := map[string][]GoTest{}
		testCount := 1
		for _, t := range targetedTests {
			for i := range suite {
				if _, ok := keptTests[suite[i].Name]; !ok {
					keptTests[suite[i].Name] = []GoTest{}
				}
				for j := range suite[i].Tests {
					if strings.HasPrefix(t, "r/") {
						if res[t].MatchString(suite[i].Tests[j].Name) {
							suite[i].Tests[j].Priority = testCount
							testCount = testCount + 1
							keptTests[suite[i].Name] = append(keptTests[suite[i].Name], suite[i].Tests[j])
						}
					} else if t == strings.Split(suite[i].Tests[j].Name, " ")[0] {
						suite[i].Tests[j].Priority = testCount
						testCount = testCount + 1
						keptTests[suite[i].Name] = append(keptTests[suite[i].Name], suite[i].Tests[j])
					}
				}
			}
		}

		for i := range suite {
			suite[i].Tests = keptTests[suite[i].Name]
		}
	} else if len(groupNames) > 0 {
		keptTests := map[string][]GoTest{}
		for _, tg := range groupNames {
			for i := range suite {
				if _, ok := keptTests[suite[i].Name]; !ok {
					keptTests[suite[i].Name] = []GoTest{}
				}
				for j := range suite[i].Tests {
					for _, g := range suite[i].Tests[j].Groups {
						if tg == g {
							keptTests[suite[i].Name] = append(keptTests[suite[i].Name], suite[i].Tests[j])
						}
					}
				}
			}
		}

		for i := range suite {
			suite[i].Tests = keptTests[suite[i].Name]
		}
	} else {
		// Normal mode: remove skipped tests
		for i := range suite {
			keptTests := []GoTest{}
			for j := range suite[i].Tests {
				if !suite[i].Tests[j].Skip &&
					(!mustPassOnly || suite[i].Tests[j].MustPass) &&
					(!ignorePatched || !isPatched(suite[i].Tests[j])) {
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

	if len(excludeTestNames) > 0 {
		excludedTests := map[string]bool{}
		for _, t := range excludeTestNames {
			if strings.HasPrefix(t, "r/") {
				re := regexp.MustCompile(t[2:])
				for i := range suite {
					for j := range suite[i].Tests {
						if re.MatchString(suite[i].Tests[j].Name) {
							excludedTests[strings.Split(suite[i].Tests[j].Name, " ")[0]] = true
						}
					}
				}
			} else {
				excludedTests[strings.Split(t, " ")[0]] = true
			}
		}

		for i := range suite {
			keptTests := []GoTest{}
			for j := range suite[i].Tests {
				prefix := strings.Split(suite[i].Tests[j].Name, " ")[0]
				if _, found := excludedTests[prefix]; !found {
					keptTests = append(keptTests, suite[i].Tests[j])
				}
			}
			suite[i].Tests = keptTests
		}
	}

	// adjust timeouts, priorities, & owners
	for i := range suite {
		if suite[i].Priority == 0 {
			suite[i].Priority = 100000000
		}

		for j := range suite[i].Tests {
			owners := make(map[string]bool)
			for _, ow := range suite[i].Owners {
				owners[ow] = true
			}
			for _, ow := range suite[i].Tests[j].Owners {
				owners[ow] = true
			}
			suite[i].Tests[j].Owners = []string{}
			for ow := range owners {
				suite[i].Tests[j].Owners = append(suite[i].Tests[j].Owners, ow)
			}

			if suite[i].Tests[j].Priority == 0 {
				suite[i].Tests[j].Priority = 100000000
			}

			if suite[i].Timeout > 0 && suite[i].Tests[j].Timeout == 0 {
				suite[i].Tests[j].Timeout = suite[i].Timeout
			}

			if suite[i].Internal {
				suite[i].Tests[j].Internal = true
			}

			if len(suite[i].Branch) > 0 && len(suite[i].Tests[j].Branch) == 0 {
				suite[i].Tests[j].Branch = suite[i].Branch
			}

			if len(suite[i].Revision) > 0 && len(suite[i].Tests[j].Revision) == 0 {
				suite[i].Tests[j].Revision = suite[i].Revision
			}

			if suite[i].PrNum != 0 && suite[i].Tests[j].PrNum == 0 {
				suite[i].Tests[j].PrNum = suite[i].PrNum
			}

			if len(suite[i].Testbeds) > 0 && len(suite[i].Tests[j].Testbeds) == 0 {
				suite[i].Tests[j].Testbeds = suite[i].Testbeds
			}

			if len(suite[i].Groups) > 0 && len(suite[i].Tests[j].Groups) == 0 {
				suite[i].Tests[j].Groups = suite[i].Groups
			}

			if len(suite[i].Tests[j].Pretests) == 0 {
				suite[i].Tests[j].Pretests = append(suite[i].Tests[j].Pretests, suite[i].Pretests...)
			}

			if len(suite[i].Tests[j].Posttests) == 0 {
				suite[i].Tests[j].Posttests = append(suite[i].Tests[j].Posttests, suite[i].Posttests...)
			}
		}
	}

	if randomize {
		rand.Seed(time.Now().UnixNano())
	}

	// sort by priority
	for _, suite := range suite {
		if randomize {
			rand.Shuffle(len(suite.Tests), func(i, j int) { suite.Tests[i], suite.Tests[j] = suite.Tests[j], suite.Tests[i] })
		} else if sorted {
			sort.Slice(suite.Tests, func(i, j int) bool {
				return suite.Tests[i].Priority < suite.Tests[j].Priority
			})
		}
	}

	if randomize {
		rand.Shuffle(len(suite), func(i, j int) { suite[i], suite[j] = suite[j], suite[i] })
	} else if sorted {
		sort.Slice(suite, func(i, j int) bool {
			return suite[i].Priority < suite[j].Priority
		})
	}

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

	if ignoreDeviations {
		for i := range suite {
			for j := range suite[i].Tests {
				keptsArgs := []string{}
				for k := range suite[i].Tests[j].Args {
					if !strings.HasPrefix(suite[i].Tests[j].Args[k], "-deviation") {
						keptsArgs = append(keptsArgs, suite[i].Tests[j].Args[k])
					}
				}
				suite[i].Tests[j].Args = keptsArgs
			}
		}
	}

	var testSuiteCode strings.Builder
	tbFound := map[string]bool{}

	for i := range suite {
		for j := range suite[i].Tests {
			suite[i].Tests[j].ShortName = strings.Split(suite[i].Tests[j].Name, " ")[0]
			if len(testRepoRev) > 0 {
				suite[i].Tests[j].Internal = false
				suite[i].Tests[j].PrNum = 0
				suite[i].Tests[j].Branch = ""
				suite[i].Tests[j].Revision = ""

				parts := strings.Split(testRepoRev, "#")
				switch parts[0] {
				case "I-PR":
					suite[i].Tests[j].Internal = true
					if pr, err := strconv.Atoi(parts[1]); err == nil {
						suite[i].Tests[j].PrNum = pr
					} else {
						log.Fatalf("%v is not a valid integer pr number", parts[1])
					}
				case "PR":
					if pr, err := strconv.Atoi(parts[1]); err == nil {
						suite[i].Tests[j].PrNum = pr
					} else {
						log.Fatalf("%v is not a valid integer pr number", parts[1])
					}
				case "I-BR":
					suite[i].Tests[j].Branch = parts[1]
					suite[i].Tests[j].Internal = true
				case "BR":
					suite[i].Tests[j].Branch = parts[1]
				case "I-REV":
					suite[i].Tests[j].Revision = parts[1]
					suite[i].Tests[j].Internal = true
				case "REV":
					suite[i].Tests[j].Revision = parts[1]
				default:
					suite[i].Tests[j].Revision = testRepoRev
					suite[i].Tests[j].Internal = true
				}
			}

			if len(defaultTestRepoRev) > 0 {
				if !isPatched(suite[i].Tests[j]) {
					suite[i].Tests[j].Revision = defaultTestRepoRev
					suite[i].Tests[j].Internal = true
				}
			}

			if len(testbeds) > 0 {
				suite[i].Tests[j].Testbeds = testbeds
			}

			if len(outDir) > 0 {
				testSuiteCode.Reset()
			}

			if len(suite[i].Tests[j].TestbedsExclude) > 0 {
				testbedsKeep := []string{}
				for _, tb := range suite[i].Tests[j].Testbeds {
					found := false
					for _, tbi := range suite[i].Tests[j].TestbedsExclude {
						if tbi == tb {
							found = true
							break
						}
					}

					if !found {
						testbedsKeep = append(testbedsKeep, tb)
					}
				}
				suite[i].Tests[j].Testbeds = testbedsKeep
			}

			suite[i].Tests[j].Testbeds =
				append(suite[i].Tests[j].Testbeds, suite[i].Tests[j].TestbedsInclude...)

			for _, tb := range suite[i].Tests[j].Testbeds {
				tbFound[tb] = true
			}

			firexSuiteTemplate.Execute(&testSuiteCode, struct {
				Test              GoTest
				UseShortTestNames bool
				TestNamePrefix    string
				Plugins           []string
				Env               map[string]string
				InternalRepoRev   string
			}{
				Test:              suite[i].Tests[j],
				UseShortTestNames: useShortName,
				TestNamePrefix:    testNamePrefix,
				Plugins:           extraPlugins,
				Env:               env,
				InternalRepoRev:   internalRepoRev,
			})

			if len(outDir) > 0 {
				suiteFile := filepath.Join(outDir,
					strings.Split(strings.TrimSpace(suite[i].Tests[j].Name), " ")[0]+".yaml")
				os.WriteFile(suiteFile, []byte(testSuiteCode.String()), 0644)
			}
		}
	}

	if !showTestbeds && len(outDir) == 0 {
		fmt.Printf("%v", testSuiteCode.String())
	} else if showTestbeds {
		output := ""
		for k := range tbFound {
			output += k + ","
		}
		output = strings.Trim(output, ",")
		fmt.Println(output)
	}
}

func isPatched(test GoTest) bool {
	return test.Branch != "" ||
		test.Revision != "" ||
		test.PrNum != 0
}
