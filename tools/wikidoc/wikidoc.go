// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// wikidoc inspects all feature profiles for test plans and compiles into a
// single location
package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"flag"

	log "github.com/golang/glog"
	"google.golang.org/protobuf/encoding/prototext"

	mpb "github.com/openconfig/featureprofiles/proto/metadata_go_proto"
)

// testDoc stores test plan metadata.
type testDoc struct {
	// Test directory name, e.g. "example_test"
	Name string
	// Full test title, e.g. "XX-01: Example Test"
	Title string
	// Path is the file location of the test documentation, typically named README.md.
	Path string
}

// path relative from outputRoot containing all test plan documents.
const wikiPath = "/testplans/"

var (
	featureRoot = flag.String("feature_root", "", "root directory of the feature profiles")
	outputRoot  = flag.String("output_root", "", "root directory to output testplan docs")
	sidebarTmpl = flag.String("sidebar_tmpl", "tools/wikidoc/sidebar.tmpl", "path to sidebar template")
)

func main() {
	flag.Parse()
	if *featureRoot == "" {
		log.Fatal("feature_root must be set.")
	}
	if *outputRoot == "" {
		log.Fatal("output_root must be set.")
	}

	docs, err := fetchTestDocs(*featureRoot)
	if err != nil {
		log.Fatal(err)
	}
	sortTests(docs)

	err = writeTestDocs(docs, *outputRoot)
	if err != nil {
		log.Fatal(err)
	}

	err = writeSidebar(docs, *sidebarTmpl, *outputRoot)
	if err != nil {
		log.Fatal(err)
	}
}

// writeSidebar creates a sidebar document formatted from tmplFile in root.
func writeSidebar(docs []testDoc, tmplFile string, rootPath string) error {
	f, err := os.Create(rootPath + "/_Sidebar.md")
	if err != nil {
		return err
	}
	defer f.Close()

	sidebar, err := os.ReadFile(tmplFile)
	if err != nil {
		return err
	}

	t, err := template.New("sidebar").Parse(string(sidebar))
	if err != nil {
		return err
	}
	t.Execute(f, docs)

	return nil
}

// writeTestDocs outputs test docs into rootPath.
func writeTestDocs(docs []testDoc, rootPath string) error {
	err := os.MkdirAll(rootPath+wikiPath, os.ModePerm)
	if err != nil {
		return err
	}

	for _, doc := range docs {
		f, err := os.ReadFile(doc.Path)
		if err != nil {
			return err
		}

		err = os.WriteFile(rootPath+wikiPath+doc.Name+".md", f, 0644)
		if err != nil {
			return err
		}
	}

	return nil
}

// fetchTestDocs gathers all valid test plan documents in rootPath
func fetchTestDocs(rootPath string) ([]testDoc, error) {
	docMap := make(map[string]testDoc)

	err := filepath.WalkDir(rootPath,
		func(path string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if filepath.Base(path) != "metadata.textproto" {
				return nil
			}

			bytes, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			md := new(mpb.Metadata)
			if err := prototext.Unmarshal(bytes, md); err != nil {
				return err
			}
			docMap[md.GetUuid()] = testDoc{
				Name:  filepath.Base(filepath.Dir(path)),
				Path:  filepath.Dir(path) + "/README.md",
				Title: md.GetPlanId() + ": " + md.GetDescription(),
			}

			return nil
		})

	docs := make([]testDoc, 0, len(docMap))
	for _, v := range docMap {
		docs = append(docs, v)
	}
	return docs, err
}

func sortTests(docs []testDoc) {
	re := regexp.MustCompile("[0-9]+|[a-z]+")
	sort.Slice(docs, func(i, j int) bool {
		in := re.FindAllString(strings.ToLower(docs[i].Title), -1)
		jn := re.FindAllString(strings.ToLower(docs[j].Title), -1)

		minLen := len(in)
		if len(in) > len(jn) {
			minLen = len(jn)
		}

		for k := 0; k < minLen; k++ {
			if in[k] == jn[k] {
				continue
			}

			iv, errI := strconv.Atoi(in[k])
			jv, errJ := strconv.Atoi(jn[k])

			if errI == nil && errJ == nil {
				return iv < jv
			}

			return strings.Compare(in[k], jn[k]) < 0
		}

		return len(in) < len(jn)
	})
}
