package helpers

import (
	"github.com/openconfig/ondatra/knebind"
)

var kneBindConfig *knebind.Config

func init() {
	var err error
	kneBindConfig, err = knebind.ParseConfigFile("/home/opetre/featureprofiles/topologies/kne/testbed.kne.yml")
	if err != nil {
		panic(err)
	}
}
