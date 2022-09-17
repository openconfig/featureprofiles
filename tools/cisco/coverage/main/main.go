package main

//Packahe main is the main package for the covergae tool

import (
	"github.com/openconfig/featureprofiles/tools/cisco/coverage"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(coverage.CoverageAnalyzer)
}
