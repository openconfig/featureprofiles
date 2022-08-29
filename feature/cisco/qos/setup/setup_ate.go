package setup

import (
	"fmt"

	"os"

	"strings"
)

var (
	oCPackagest = []string{"system", "acl", "networkinstance", "lacp", "local-routes", "lldp", "network-instance", "components", "qos", "interface"} // order is important
)

// FindTestDataPath This function finds tha path of the file
func FindTestDataPath() string {
	path, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("Error: %v", err))
	}
	for _, ocPkg := range oCPackagest {
		if strings.Contains(path, ocPkg) {
			return strings.Split(path, ocPkg)[0] + "/" + ocPkg + "/testdata/base_config.json"
		}
	}
	return "testdata/base_config.json"
}
