package helpers

import (
	"github.com/openconfig/ondatra/knebind"
)

var kneBindConfig *knebind.Config

func init() {
	var err error
	kneBindConfig, err = knebind.ParseConfigFile("../resources/global/knebind-config.yaml")
	if err != nil {
		panic(err)
	}
}
