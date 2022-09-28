package confgen

import (
	"os"
	"testing"

	oc "github.com/openconfig/ondatra/telemetry"
)

var bundles = []Bundle{
	{
		Id:                121,
		Interfaces:        []string{"HundredGigE0/0/0/1"},
		SubInterfaceRange: []int{100, 196},
	},
	{
		Id:                122,
		Interfaces:        []string{"HundredGigE0/0/0/3"},
		SubInterfaceRange: []int{100, 196},
	},
	{
		Id:                123,
		Interfaces:        []string{"HundredGigE0/0/0/5"},
		SubInterfaceRange: []int{100, 196},
	},
	{
		Id:                124,
		Interfaces:        []string{"HundredGigE0/3/0/0"},
		SubInterfaceRange: []int{100, 196},
	},
	{
		Id:                125,
		Interfaces:        []string{"HundredGigE0/3/0/2"},
		SubInterfaceRange: []int{100, 196},
	},
	{
		Id:                126,
		Interfaces:        []string{"HundredGigE0/3/0/3"},
		SubInterfaceRange: []int{100, 196},
	},
	{
		Id:                127,
		Interfaces:        []string{"HundredGigE0/3/0/3"},
		SubInterfaceRange: []int{100, 196},
	},
	{
		Id: 128,
	},
}

func TestGenerateConfig(t *testing.T) {
	generatedConf := GenerateConfig(bundles,"templates/gnmi.jsonnet")
	if err := os.WriteFile("output.json", []byte(generatedConf), 0644); err != nil {
		t.Fatalf(err.Error())
	}

	if err := oc.Unmarshal([]byte(generatedConf), &oc.Device{}); err != nil {
		t.Fatalf(err.Error())
	}
}
