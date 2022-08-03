package main

import (
	"github.com/openconfig/featureprofiles/tools/cisco/coverage"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(coverage.CoverageAnalyzer)
}
