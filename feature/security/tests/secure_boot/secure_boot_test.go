package secure_boot_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"google.golang.org/protobuf/encoding/prototext"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

var (
	device1             = "dut"
	cliSecureBootStatus = map[ondatra.Vendor]string{
		ondatra.CISCO: "show  platform security integrity log secure-boot status",
	}
)

func TestSecureBootEnable(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	switch dut.Vendor() {
	case ondatra.CISCO:
		resp := cmdViaGNMI(context.Background(), t, dut, cliSecureBootStatus[ondatra.CISCO])
		if !strings.Contains(resp, "Enabled") {
			t.Errorf("Secure Boot is not Enabled")
		}
	default:
		t.Skipf("Secure boot verification not available for vendor %s", dut.Vendor())
	}
}

// cmdViaGNMI push cli command to device using GNMI
func cmdViaGNMI(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, cmd string) string {
	gnmiC := dut.RawAPIs().GNMI().New(t)
	getRequest := &gpb.GetRequest{
		Prefix: &gpb.Path{
			Origin: "cli",
		},
		Path: []*gpb.Path{
			{
				Elem: []*gpb.PathElem{{
					Name: cmd,
				}},
			},
		},
		Encoding: gpb.Encoding_ASCII,
	}
	t.Logf("Get cli (%s) via GNMI: \n %s", cmd, prototext.Format(getRequest))
	if _, deadlineSet := ctx.Deadline(); !deadlineSet {
		tmpCtx, cncl := context.WithTimeout(ctx, time.Second*120)
		ctx = tmpCtx
		defer cncl()
	}
	resp, err := gnmiC.Get(ctx, getRequest)
	if err != nil {
		t.Fatalf("running cmd (%s) via GNMI is failed: %v", cmd, err)
	}
	t.Logf("Get cli via gnmi reply: \n %s", prototext.Format(resp))
	return string(resp.GetNotification()[0].GetUpdate()[0].GetVal().GetAsciiVal())

}
