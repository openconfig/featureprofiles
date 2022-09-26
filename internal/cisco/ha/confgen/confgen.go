package confgen

import (
	"log"

	"github.com/google/go-jsonnet"
)

func GenerateConfig(file string) string {
	vm := jsonnet.MakeVM()
	genConfig, err := vm.EvaluateFile(file)
	if err != nil {
		log.Fatal(err)
	}
	return genConfig
}
