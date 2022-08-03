package main

import (
	"github.com/openconfig/featureprofiles/tools/coverage"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(coverage.CoverageAnalyzer)
}
