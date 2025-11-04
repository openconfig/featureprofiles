// Package verifiers offers APIs to verify operational data for components.
package verifiers

import (
	"context"
	"fmt"
	// "os"
	"testing"

	textfsm "github.com/openconfig/featureprofiles/exec/utils/textfsm/textfsm"
	// "github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/ondatra"
)

type fibVerifier struct{}

// VerifyShowCEFVRFDetail returns the CEF VRF detail for a given prefix and VRF using the "show cef <vrf> <prefix>" CLI.
func (v *fibVerifier) VerifyShowCEFVRFDetail(t *testing.T, ctx context.Context, dut *ondatra.DUTDevice, prefix, vrf string, want ...*textfsm.ShowCefVrfDetail) (textfsm.ShowCefVrfDetail, bool) {
	matches := true
	// cliOutput := config.TextWithSSH(ctx, t, dut, fmt.Sprintf("show cef vrf %s %s detail", vrf, prefix))
	cliOutput := dut.CLI().Run(t, fmt.Sprintf("show cef vrf %s %s detail", vrf, prefix))
	cefVrfTextfsm := textfsm.ShowCefVrfDetail{}
	if err := cefVrfTextfsm.Parse(cliOutput); err != nil {
		t.Fatalf("%v", err)
	}
	t.Logf("%+v\n", cefVrfTextfsm)
	for _, w := range want {
		if err := util.CompareStructRequiredFields(w, cefVrfTextfsm); err != nil {
			matches = false
		}
	}
	return cefVrfTextfsm, matches
}
