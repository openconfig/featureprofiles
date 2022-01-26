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
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	log "github.com/golang/glog"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/openconfig/ygot/util"
	"github.com/protocolbuffers/txtpbfmt/parser"
)

var (
	featuresRootFlag = flag.String("feature_root", "", "root directory of the feature profiles")
	yangRootsFlag    = flag.String(
		"yang_roots", "", "comma separated list of directories containing .yang files.",
	)
	yangSkipsFlag = flag.String(
		"yang_skip_roots", "", "sub-directories of the .yang roots which should be ignored.",
	)
)

var (
	featuresRoot string
	yangPaths    []string
	skipYANGDirs = map[string]bool{}
)

func init() {
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
}

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

	yang.AddPath(dirs...)

	ms := yang.NewModules()

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
	oc     string
	line   int32
	detail string
}

type file struct {
	name  string
	lines []line
}

// unknownPaths parses all `path:` lines in the input `files`, reporting any paths which are not in
// the `knownOC` set.
func unknownPaths(knownOC map[string]pathType, files []string) ([]file, error) {
	unknown := []file{}

	for _, f := range files {
		bs, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, err
		}

		// Using the parser.Parse rather than an Unmarshal so I can get line numbers.
		lines := []line{}
		ast, err := parser.Parse(bs)
		if err != nil {
			return nil, err
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
								lines = append(lines, line{
									oc: v.Value, line: c.Start.Line, detail: detail})
							}
						}
					}
				}
			}
		}

		if len(lines) == 0 {
			continue
		}
		unknown = append(unknown, file{name: f, lines: lines})
	}
	return unknown, nil
}

// featureFiles lists the file paths containing features data.
func featureFiles() ([]string, error) {
	files := []string{}
	err := filepath.WalkDir(featuresRoot,
		func(path string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if e.IsDir() {
				return nil
			}
			if strings.HasSuffix(e.Name(), ".textproto") {
				files = append(files, path)
			}
			return nil
		})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// Check that every OC path used in the Feature Profiles is defined in the public OpenConfig yang.
func main() {
	ms, err := modules()
	if err != nil {
		log.Fatal(err)
	}
	knownPaths := map[string]pathType{}
	for _, m := range ms {
		addKnownPaths(knownPaths, yang.ToEntry(m))
	}

	files, err := featureFiles()
	if err != nil {
		log.Fatal(err)
	}

	unknown, err := unknownPaths(knownPaths, files)
	if err != nil {
		log.Fatal(err)
	}

	if len(unknown) == 0 {
		return
	}
	msg := []string{"Feature paths inconsistent with YANG schema:"}
	for _, f := range unknown {
		msg = append(msg, "  file: "+f.name)
		for _, l := range f.lines {
			msg = append(msg, fmt.Sprintf("    line %d: %s %s", l.line, l.detail, l.oc))
		}
	}
	log.Error(strings.Join(msg, "\n"))
	os.Exit(1)
}
