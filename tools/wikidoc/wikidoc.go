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
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"flag"

	log "github.com/golang/glog"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/text"
)

// testDoc stores test plan metadata.
type testDoc struct {
	Name  string
	Title string
	Path  string
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
	docs := []testDoc{}

	err := filepath.WalkDir(rootPath,
		func(path string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !validDoc(path) {
				return nil
			}

			title, err := docTitle(path)
			if err != nil {
				return err
			}

			doc := testDoc{
				Name:  filepath.Base(filepath.Dir(path)),
				Title: title,
				Path:  path,
			}
			docs = append(docs, doc)

			return nil
		})

	return docs, err
}

// validDoc checks if a given file path is eligible to contain a testplan doc.
func validDoc(path string) bool {
	if filepath.Base(path) != "README.md" {
		return false
	}

	validPaths := []string{"/ate_tests/", "/tests/"}
	for _, validPath := range validPaths {
		if strings.Contains(path, validPath) {
			return true
		}
	}

	return false
}

// docTitle fetches the first header string from a markdown file
func docTitle(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	markdown := goldmark.New()
	doc := markdown.Parser().Parse(text.NewReader(b))
	if doc.ChildCount() == 0 {
		return "", errors.New("no children")
	}

	return string(doc.FirstChild().Text(b)), nil
}
