package confgen

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	oc "github.com/openconfig/ondatra/telemetry"
)

func TestGenerateConfig(t *testing.T) {
	exepectedConf, err := os.ReadFile("testdata/gnmi_interfaces_want.json")
	if err != nil {
		t.Fatalf("Cannot load base config: %v", err)
	}

	want := oc.Device{}
	if err := oc.Unmarshal([]byte(exepectedConf), &want); err != nil {
		t.Fatalf(err.Error())
	}

	generatedConf := GenerateConfig("templates/gnmi_interfaces.jsonnet")
	got := oc.Device{}
	if err := oc.Unmarshal([]byte(generatedConf), &got); err != nil {
		t.Fatalf(err.Error())
	}

	if err := os.WriteFile("output.json", []byte(generatedConf), 0644); err != nil {
		t.Fatalf(err.Error())
	}

	diff := cmp.Diff(want, got)
	if len(diff) > 0 {
		t.Logf("%s", diff)
	}
}
