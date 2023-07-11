// Copyright 2022 Google LLC

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     https://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// validate_paths inspects paths in the Feature Profiles and fails if any are not standard
// OpenConfig paths.
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"flag"

	log "github.com/golang/glog"
	fppb "github.com/openconfig/featureprofiles/proto/feature_go_proto"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/openconfig/ygot/util"
	"github.com/protocolbuffers/txtpbfmt/parser"
	"google.golang.org/protobuf/encoding/prototext"
)

var (
	featuresRootFlag = flag.String("feature_root", "", "root directory of the feature profiles")
	yangRootsFlag    = flag.String(
		"yang_roots", "", "comma separated list of directories containing .yang files.",
	)
	yangSkipsFlag = flag.String(
		"yang_skip_roots", "", "sub-directories of the .yang roots which should be ignored.",
	)
	featureFilesFlag = flag.String("feature_files", "", "optional file containing list of feature.textprotos to validate instead of checking all files. If not specified, then all files will be checked. Note that all files will still be checked and annotated, but only these files will cause failure.")
)

var (
	featuresRoot string
	yangPaths    []string
	skipYANGDirs = map[string]bool{}
)

type pathType int

const (
	unset pathType = iota
	configuration
	telemetry
)

// addKnownPaths records information about all paths in and under a `yang.Entity`.
func addKnownPaths(ps map[string]pathType, e *yang.Entry) {
	if e.IsLeaf() || e.IsLeafList() {
		pt := unset
		switch util.IsConfig(e) {
		case true:
			pt = configuration
		case false:
			pt = telemetry
		}
		ps[fmt.Sprintf("%q", util.SchemaTreePathNoModule(e))] = pt
		return
	}
	for _, ce := range util.Children(e) {
		addKnownPaths(ps, ce)
	}
}

func yangFiles(root string) ([]string, error) {
	ps := map[string]bool{}
	err := filepath.WalkDir(root, func(p string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if info == nil {
			return nil
		}
		if info.IsDir() {
			if skipYANGDirs[p] {
				fmt.Println("Skipping definitions in", p)
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".yang") {
			ps[p] = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	res := make([]string, 0, len(ps))
	for p := range ps {
		res = append(res, p)
	}
	return res, nil
}

func modules() (map[string]*yang.Module, error) {
	var files, dirs []string
	for _, p := range yangPaths {
		ds, err := yang.PathsWithModules(p)
		if err != nil {
			return nil, err
		}
		dirs = append(dirs, ds...)

		fs, err := yangFiles(p)
		if err != nil {
			return nil, err
		}
		files = append(files, fs...)
	}

	ms := yang.NewModules()

	ms.AddPath(dirs...)

	for _, p := range files {
		p = path.Base(p)
		if err := ms.Read(p); err != nil {
			return nil, fmt.Errorf("ms.Read(%s): %v", p, err)
		}
	}

	if errs := ms.Process(); len(errs) != 0 {
		log.Error("ms.Process errors:")
		for _, e := range errs {
			log.Error(" ", e)
		}
		return nil, errors.New("yang module process error")
	}
	return ms.Modules, nil
}

type line struct {
	line   int32
	column int32
	oc     string
	detail string
}

type file struct {
	name  string
	lines []line
	// Errors which are not correlated with a line.
	errors []string
}

func (f file) githubAnnotations() string {
	var b strings.Builder
	for _, errLine := range f.errors {
		b.WriteString(fmt.Sprintf("::%s file=%s::%s\n", "error", f.name, errLine))
	}

	for _, line := range f.lines {
		// https://docs.github.com/en/actions/using-workflows/workflow-commands-for-github-actions#setting-an-error-message
		b.WriteString(fmt.Sprintf("::%s file=%s,line=%d,col=%d::%s %s\n", "error", f.name, line.line, line.column, line.detail, line.oc))
	}

	return b.String()
}

func constructValidProfiles(files []string) (map[string]bool, map[string]*file) {
	tmp := fppb.FeatureProfile{}
	validProfiles := make(map[string]bool)
	reports := make(map[string]*file)

	for _, f := range files {
		report := reports[f]
		if report == nil {
			report = &file{name: f}
			reports[f] = report
		}
		bs, err := os.ReadFile(f)
		if err != nil {
			// Just accumulate the file error since we can't do anything else.
			report.errors = append(report.errors, err.Error())
			continue
		}

		// Unmarshal will report syntax errors (although generally without line numbers).
		if err := prototext.Unmarshal(bs, &tmp); err != nil {
			report.errors = append(report.errors, err.Error())
		}

		// Validate feature profile ID name by checking path.
		targetFeatureProfileName := getFeatureProfileNameFromPath(f)
		featureProfileIDName := tmp.GetId().GetName()
		if targetFeatureProfileName != featureProfileIDName {
			report.errors = append(report.errors, featureProfileIDName+" is inconsistent with path, want "+targetFeatureProfileName)
		} else {
			validProfiles[featureProfileIDName] = true
		}
	}
	return validProfiles, reports
}

// checkFiles parses all `path:` lines in the input `files`, reporting any syntax errors and paths
// which are not in the `knownOC` set.
func checkFiles(knownOC map[string]pathType, files []string, validProfiles map[string]bool, reports map[string]*file) error {
	tmp := fppb.FeatureProfile{}

	log.Infof("%d files to validate", len(files))

	for _, f := range files {
		log.Infof("Validating file: %v", f)
		report := reports[f]
		if report == nil {
			report = &file{name: f}
			reports[f] = report
		}

		bs, err := os.ReadFile(f)
		if err != nil {
			// Just accumulate the file error since we can't do anything else.
			report.errors = append(report.errors, err.Error())
			continue
		}

		// Unmarshal will report syntax errors (although generally without line numbers).
		if err := prototext.Unmarshal(bs, &tmp); err != nil {
			report.errors = append(report.errors, err.Error())
		}

		// Use parser.Parse so I can get line numbers for OC paths we don't recognize.
		ast, err := parser.Parse(bs)
		if err != nil {
			return err
		}
		for _, a := range ast {
			switch a.Name {
			case "config_path", "telemetry_path":
				for _, c := range a.Children {
					if c.Name == "path" {
						for _, v := range c.Values {
							var detail string
							switch knownOC[v.Value] {
							case configuration:
								if a.Name != "config_path" {
									detail = fmt.Sprintf("erroneously labeled %s", a.Name)
								}
							case telemetry:
								if a.Name != "telemetry_path" {
									detail = fmt.Sprintf("erroneously labeled %s", a.Name)
								}
							case unset:
								detail = "missing from YANG"
							}
							if detail != "" {
								report.lines = append(report.lines, line{
									line:   c.Start.Line,
									column: c.Start.Column,
									oc:     v.Value,
									detail: detail,
								})
							}
						}
					}
				}
			case "feature_profile_dependency":
				for _, c := range a.Children {
					if c.Name == "name" {
						for _, v := range c.Values {
							profileName := v.Value[1 : len(v.Value)-1] // Trim quotes
							if !validProfiles[profileName] {
								report.lines = append(report.lines, line{
									line:   c.Start.Line,
									column: c.Start.Column,
									detail: "cannot find feature profile dependency " + profileName,
								})
							}
						}
					}
				}
			}
		}
	}
	return nil
}

// cleanReports removes any empty reports.
func cleanReports(reports map[string]*file) {
	for key, report := range reports {
		if reflect.DeepEqual(report, &file{name: report.name}) {
			delete(reports, key)
		}
	}
}

// getFeatureProfileNameFromPath gets feature profile id.name from path.
func getFeatureProfileNameFromPath(file string) string {
	featureProfileFilePath := strings.ReplaceAll(strings.TrimPrefix(file, featuresRoot), "/", " ")
	featureProfileFilePathArray := strings.Fields(featureProfileFilePath)
	featureProfileFilePathArray = featureProfileFilePathArray[0 : len(featureProfileFilePathArray)-1]
	return strings.Join(featureProfileFilePathArray, "_")
}

// featureFiles returns the feature files to check and all existing feature files.
func featureFiles() (map[string]struct{}, []string, error) {
	var allFiles []string
	filesToCheck := map[string]struct{}{}
	err := filepath.WalkDir(featuresRoot,
		func(path string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if e.IsDir() {
				return nil
			}
			if e.Name() == "feature.textproto" {
				allFiles = append(allFiles, path)
				filesToCheck[path] = struct{}{}
			}
			return nil
		})
	if err != nil {
		return nil, nil, err
	}
	sort.Strings(allFiles)

	if *featureFilesFlag != "" {
		filesToCheck = map[string]struct{}{}
		readBytes, err := os.ReadFile(*featureFilesFlag)
		if err != nil {
			log.Fatalf("cannot read feature_files flag: %v", err)
		}
		for _, line := range strings.Split(string(readBytes), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			path, err := filepath.Abs(line)
			if err != nil {
				return nil, nil, err
			}
			filesToCheck[path] = struct{}{}
		}
	}

	return filesToCheck, allFiles, nil
}

// Check that every OC path used in the Feature Profiles is defined in the public OpenConfig yang.
func main() {
	flag.Parse()
	if *featuresRootFlag == "" {
		log.Fatal("feature_root must be set.")
	}
	if *yangRootsFlag == "" {
		log.Fatal("yang_roots must be set.")
	}
	featuresRoot = *featuresRootFlag
	yangPaths = strings.Split(*yangRootsFlag, ",")
	for _, s := range strings.Split(*yangSkipsFlag, ",") {
		skipYANGDirs[s] = true
	}

	ms, err := modules()
	if err != nil {
		log.Fatal(err)
	}
	knownPaths := map[string]pathType{}
	for _, m := range ms {
		addKnownPaths(knownPaths, yang.ToEntry(m))
	}

	filesToCheck, allFiles, err := featureFiles()
	if err != nil {
		log.Fatal(err)
	}

	validProfiles, reports := constructValidProfiles(allFiles)
	if err := checkFiles(knownPaths, allFiles, validProfiles, reports); err != nil {
		log.Fatal(err)
	}

	cleanReports(reports)

	log.Infof("%d files must pass: %v", len(filesToCheck), filesToCheck)
	if len(reports) == 0 {
		return
	}

	msg := []string{"Feature paths inconsistent with YANG schema:"}
	failed := false
	for _, f := range reports {
		fmt.Print(f.githubAnnotations())
		if _, ok := filesToCheck[f.name]; ok {
			failed = true
		}
		msg = append(msg, "  file: "+f.name)
		if len(f.errors) != 0 {
			msg = append(msg, "  toplevel errors:")
			for _, e := range f.errors {
				msg = append(msg, "   "+e)
			}
		}
		for _, l := range f.lines {
			msg = append(msg, fmt.Sprintf("    line %d: %s %s", l.line, l.detail, l.oc))
		}
	}
	log.Info(strings.Join(msg, "\n"))
	if failed {
		os.Exit(1)
	}
}
