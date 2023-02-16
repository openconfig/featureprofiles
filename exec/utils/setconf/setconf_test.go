package setconf_test

import (
	"context"
	"flag"
	"os"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
)

var (
	confFlag   = flag.String("conf", "", "CLI configuration file")
	updateFlag = flag.Bool("update", false, "Perform Update instead of Replace")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestSetConf(t *testing.T) {
	if *confFlag == "" {
		t.Fatal("Missing conf arg")
	}

	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	b, err := os.ReadFile(*confFlag)
	if err != nil {
		t.Fatalf("Error reading cli config file: %v", err)
	}

	conf := string(b)
	updateRequest := &gnmi.Update{
		Path: &gnmi.Path{
			Origin: "cli",
		},
		Val: &gnmi.TypedValue{
			Value: &gnmi.TypedValue_AsciiVal{
				AsciiVal: conf,
			},
		},
	}

	setRequest := &gnmi.SetRequest{}
	if *updateFlag {
		setRequest.Update = []*gnmi.Update{updateRequest}
	} else {
		setRequest.Replace = []*gnmi.Update{updateRequest}
	}

	gnmiClient := dut.RawAPIs().GNMI().New(t)
	if _, err := gnmiClient.Set(ctx, setRequest); err != nil {
		t.Fatalf("gNMI set request failed: %v", err)
	}
}
