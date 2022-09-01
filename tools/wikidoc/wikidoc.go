package main

import (
	"errors"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	log "github.com/golang/glog"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/text"
)

// testDoc describes a test plan document
type testDoc struct {
	Name  string
	Title string
	Path  string
}

// path relative from outputRoot containing all test plans
const wikiPath = "/testplans/"

var (
	featureRoot = flag.String("feature_root", "", "root directory of the feature profiles")
	outputRoot  = flag.String("output_root", "", "root directory to output test docs")
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

	docs, err := fetchTestDocs()
	if err != nil {
		log.Fatal(err)
	}

	err = writeTestDocs(docs)
	if err != nil {
		log.Fatal(err)
	}

	err = writeSidebar(docs)
	if err != nil {
		log.Fatal(err)
	}
}

// writeSidebar creates a sidebar document formatted from sidebarTmpl in outputRoot
func writeSidebar(docs []testDoc) error {
	f, err := os.Create(*outputRoot + "/_Sidebar.md")
	if err != nil {
		return err
	}
	defer f.Close()

	sidebar, err := os.ReadFile(*sidebarTmpl)
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

// writeTestDocs outputs test docs into outputRoot
func writeTestDocs(docs []testDoc) error {
	err := os.MkdirAll(*outputRoot+wikiPath, os.ModePerm)
	if err != nil {
		return err
	}
	for _, doc := range docs {
		f, err := os.ReadFile(doc.Path)
		if err != nil {
			return err
		}
		err = os.WriteFile(*outputRoot+wikiPath+doc.Name+".md", f, 0644)
		if err != nil {
			return err
		}
	}

	return nil
}

// fetchTestDocs finds all valid test documents in featureRoot
func fetchTestDocs() ([]testDoc, error) {
	docs := []testDoc{}

	err := filepath.WalkDir(*featureRoot,
		func(path string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !validPath(path) {
				return nil
			}

			testTitle, err := docTitle(path)
			if err != nil {
				return err
			}

			doc := testDoc{
				Name:  filepath.Base(filepath.Dir(path)),
				Title: testTitle,
				Path:  path,
			}
			docs = append(docs, doc)

			return nil
		})

	return docs, err
}

// validPath checks if a given file path is eligible to contain a test doc.
func validPath(path string) bool {
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
