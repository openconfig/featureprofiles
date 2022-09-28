package confgen

import (
	"encoding/json"
	"log"

	"github.com/google/go-jsonnet"
)

type Bundle struct {
	Id                int
	Interfaces        []string
	SubInterfaceRange []int
}

func GenerateConfig(bundles []Bundle, templatePath string) string {
	vm := jsonnet.MakeVM()

	if bundleJson, err := json.Marshal(bundles); err == nil {
		vm.ExtCode("bundles", string(bundleJson))
	} else {
		log.Fatal(err)
	}

	genConfig, err := vm.EvaluateFile(templatePath)
	if err != nil {
		log.Fatal(err)
	}
	return genConfig
}
