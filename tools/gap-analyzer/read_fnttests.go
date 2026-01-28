// tools/run_fnt_workflow/read_fnttests.go
package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
)

var (
	failScenario = flag.Bool("fail_scenario", false, "If true, intentionally fail the script.")
)

func main() {
	flag.Parse()

	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("Running FNT Gap Analysis Test Script...")

	if *failScenario {
		fmt.Println("--- Intentionally Failing Scenario ---")
		log.Fatalf("Intentionally failing for testing: --fail_scenario=true was set.")
	} else {
		fmt.Println("--- Successful Scenario ---")
		fmt.Println("Successful run of workflow")
	}
	fmt.Println(strings.Repeat("=", 50))
}
