// Package setup includes funtions to load oc model from a json file
package setup

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/openconfig/ondatra/gnmi/oc"
)

var (
	jsonConfig []uint8
	oCPackages = []string{"system", "sampling", "acl",
		"networkinstance", "lacp", "local-routes",
		"lldp", "network-instance", "components", "qos", "interface"} // order is important
)

func findTestDataPath() string {
	path, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("Error: %v", err))
	}
	for _, ocPkg := range oCPackages {
		if strings.Contains(path, ocPkg) {
			return strings.Split(path, ocPkg)[0] + "/" + ocPkg + "/testdata/base_config.json"
		}
	}
	return "testdata/base_config.json"
}

// BaseConfig returns the base configuration used for test setup
func BaseConfig() *oc.Sampling {
	baseConfig := new(oc.Sampling)
	oc.Unmarshal(jsonConfig, baseConfig)
	return baseConfig
}

func init() {
	var err error
	jsonConfig, err = os.ReadFile(findTestDataPath())
	if err != nil {
		panic(fmt.Sprintf("Cannot load base config: %v", err))
	}

	var baseConfig oc.Sampling
	if err := oc.Unmarshal(jsonConfig, &baseConfig); err != nil {
		panic(fmt.Sprintf("Cannot unmarshal base config: %v", err))
	}
}

// SkipSubscribe returns true when the test cases do not need to do subscribe for the leafs
func SkipSubscribe() bool {
	return false
}

// SkipGet returns true when the test cases do not need to do subscribe for the leafs
func SkipGet() bool {
	return false
}

// GetAnyValue return the first entry from a map
func GetAnyValue[M ~map[K]V, K comparable, V any](m M) V {
	var r V
	for _, v := range m {
		r = v
		break
	}
	return r
}

// ResetStruct removes all non-primitive child from the struct except the ones passed as excepts
func ResetStruct[T any](s *T, except []string) {
	fields := reflect.TypeOf(*s)
	values := reflect.ValueOf(s).Elem()

OUTER:
	for i := 0; i < fields.NumField(); i++ {
		f := fields.Field(i)
		for _, e := range except {
			if f.Name == e {
				continue OUTER
			}
		}

		if f.Type.Kind() == reflect.Map || (f.Type.Kind() == reflect.Pointer && f.Type.Elem().Kind() == reflect.Struct) {
			el := values.Field(i)
			if el.IsValid() && !el.IsNil() && !el.IsZero() && el.CanSet() {
				el.Set(reflect.Zero(f.Type))
			}
		}
	}
}
