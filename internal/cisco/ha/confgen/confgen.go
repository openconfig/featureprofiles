// Package confgen provide api for generating config at a very high scale using OC and JSONNET
package confgen

import (
	"encoding/json"
	"log"

	"github.com/google/go-jsonnet"
)

// Bundle stores a bundle information.
type Bundle struct {
	ID                int
	Interfaces        []string
	SubInterfaceRange []int
}

// GenerateConfig generate configs based for a given bundles
func GenerateConfig(bundles []Bundle, templatePath string) string {
	vm := jsonnet.MakeVM()

	if bundleJSON, err := json.Marshal(bundles); err == nil {
		vm.ExtCode("bundles", string(bundleJSON))
	} else {
		log.Fatal(err)
	}

	genConfig, err := vm.EvaluateFile(templatePath)
	if err != nil {
		log.Fatal(err)
	}
	return genConfig
}
