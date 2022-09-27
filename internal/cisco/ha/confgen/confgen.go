package confgen

import (
	"encoding/json"
	"log"

	"github.com/google/go-jsonnet"
)

type bundle struct {
	Id                int
	Interfaces        []string
	SubInterfaceRange []int
}

func GenerateConfig(bundles []bundle) string {
	vm := jsonnet.MakeVM()

	if bundleJson, err := json.Marshal(bundles); err == nil {
		vm.ExtCode("bundles", string(bundleJson))
	} else {
		log.Fatal(err)
	}

	genConfig, err := vm.EvaluateFile("templates/gnmi.jsonnet")
	if err != nil {
		log.Fatal(err)
	}
	return genConfig
}
