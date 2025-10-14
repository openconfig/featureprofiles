package setconf_test

import (
	"context"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding/introspect"
	"github.com/openconfig/ondatra/gnmi"

	bootzproto "github.com/openconfig/bootz/proto/bootz"
	bcpb "github.com/openconfig/gnoi/bootconfig"
)

var (
	confFlag      = flag.String("conf", "", "CLI configuration file")
	bootConfig    = flag.Bool("boot", false, "Set boot configuration")
	dutIdFlag     = flag.String("dut", "dut", "DUT id (default: dut)")
	updateFlag    = flag.Bool("update", false, "Perform Update instead of Replace (default: false)")
	ignoreErrFlag = flag.Bool("ignore_set_err", false, "Ignore set request errors (default: false)")
)

func newBootConfigClient(t *testing.T, dut *ondatra.DUTDevice) bcpb.BootConfigClient {
	dialer := introspect.DUTDialer(t, dut, introspect.GNMI)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := dialer.Dial(ctx)
	if err != nil {
		t.Fatalf("grpc.Dial failed to: %q", dialer.DialTarget)
	}
	c := bcpb.NewBootConfigClient(conn)
	return c
}

func gnoiSetBootConfig(t *testing.T, dut *ondatra.DUTDevice, config string) {
	t.Helper()
	c := newBootConfigClient(t, dut)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	req := &bcpb.SetBootConfigRequest{
		BootConfig: &bootzproto.BootConfig{
			VendorConfig: []byte(config),
		},
	}

	if _, err := c.SetBootConfig(ctx, req); err != nil {
		t.Fatalf("Failed to set boot config: %v", err)
	} else {
		t.Logf("Boot config successfully set")
	}
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestSetConf(t *testing.T) {
	if *confFlag == "" {
		t.Fatal("Missing conf arg")
	}

	ctx := context.Background()
	dut := ondatra.DUT(t, *dutIdFlag)

	b, err := os.ReadFile(*confFlag)
	if err != nil {
		t.Fatalf("Error reading cli config file: %v", err)
	}

	conf := string(b)
	t.Logf("%v", conf)

	if *bootConfig {
		gnoiSetBootConfig(t, dut, conf)
		return
	}

	updateRequest := &gnmipb.Update{
		Path: &gnmipb.Path{
			Origin: "cli",
		},
		Val: &gnmipb.TypedValue{
			Value: &gnmipb.TypedValue_AsciiVal{
				AsciiVal: conf,
			},
		},
	}

	setRequest := &gnmipb.SetRequest{}
	if *updateFlag {
		setRequest.Update = []*gnmipb.Update{updateRequest}
	} else {
		setRequest.Replace = []*gnmipb.Update{updateRequest}
	}

	gnmiClient := dut.RawAPIs().GNMI(t)
	if _, err := gnmiClient.Set(ctx, setRequest); err != nil {
		if *ignoreErrFlag {
			t.Logf("gNMI set request failed: %v", err)
		} else {
			t.Fatalf("gNMI set request failed: %v", err)
		}

	}
}

func TestClearBootConf(t *testing.T) {
	dut := ondatra.DUT(t, *dutIdFlag)
	hostname := gnmi.Get(t, dut, gnmi.OC().System().Hostname().State())
	conf := `hostname ` + hostname
	gnoiSetBootConfig(t, dut, conf)
}
