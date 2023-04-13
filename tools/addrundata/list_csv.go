package main

import (
	"encoding/csv"
	"io"
	"path/filepath"
	"sort"
	"strings"
)

// listCSV writes the testsuite as CSV, with the columns "Feature", "ID", "Desc", and
// "Test Path".
func listCSV(w io.Writer, featuredir string, ts testsuite) error {
	rootdir := filepath.Dir(featuredir)

	cw := csv.NewWriter(w)
	heading := []string{"Feature", "ID", "Desc", "Test Path"}
	if err := cw.Write(heading); err != nil {
		return err
	}

	keys := sortedByTestPlanID(ts)

	for _, k := range keys {
		reldir, err := filepath.Rel(rootdir, k)
		if err != nil {
			reldir = k
		}
		feature := featureFromTestDir(reldir)
		feature = strings.TrimPrefix(feature, "feature/")
		pd := ts[k].markdown
		row := []string{feature, pd.testPlanID, pd.testDescription, reldir}
		if err := cw.Write(row); err != nil {
			break
		}
	}

	cw.Flush()
	return cw.Error()
}

// featureFromTestDir extracts the feature path from a test directory.
func featureFromTestDir(testdir string) string {
	kinddir := filepath.Dir(testdir)
	kind := filepath.Base(kinddir)
	if !isTestKind(kind) {
		return kinddir
	}
	return filepath.Dir(kinddir)
}

// sortedByTestPlanID returns the testsuite keys sorted by the test plan ID of the test
// cases.
func sortedByTestPlanID(ts testsuite) []string {
	keys := []string{}
	for k := range ts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		idi := ts[keys[i]].markdown.testPlanID
		idj := ts[keys[j]].markdown.testPlanID
		if idi == idj {
			return keys[i] < keys[j]
		}
		return lessVersion(idi, idj)
	})
	return keys
}
