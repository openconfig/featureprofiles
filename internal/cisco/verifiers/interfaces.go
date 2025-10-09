// Package verifiers offers APIs to verify operational data for components.
package verifiers

import (
	"context"
	"fmt"
	"testing"

	textfsm "github.com/openconfig/featureprofiles/exec/utils/textfsm/textfsm"
	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/ondatra"
)

type interfaceVerifier struct{}

// VerifyShowInterfaceCLI returns interface data from the "show interface" CLI. 'want' is optional parameter,
// if its input it compares the want data with the corresponding fields in the CLI got output.
func (v *interfaceVerifier) VerifyShowInterfaceCLI(t *testing.T, ctx context.Context, dut *ondatra.DUTDevice, intfName string, want ...*textfsm.ShowInterface) (textfsm.ShowInterface, bool) {
	matches := true
	cliOutput := config.CMDViaGNMI(ctx, t, dut, fmt.Sprintf("show interface %s", intfName))
	intfTextfsm := textfsm.ShowInterface{}
	if err := intfTextfsm.Parse(cliOutput); err != nil {
		t.Fatalf("%v", err)
	}
	t.Logf("%+v\n", intfTextfsm)
	for _, w := range want {
		if err := util.CompareStructRequiredFields(w, intfTextfsm); err != nil {
			matches = false
		}
	}
	return intfTextfsm, matches
}

// GetInterfaceOutPPS returns the Output packets per second (PPS) for a given interface using Show interface CLI.
func (v *interfaceVerifier) GetInterfaceOutPPS(t *testing.T, dut *ondatra.DUTDevice, interfaceName string) uint64 {
	ctx := context.Background()
	var pps uint64
	showIntf, _ := Interfaceverifier().VerifyShowInterfaceCLI(t, ctx, dut, interfaceName)
	for _, rate := range showIntf.GetAllThirtySecOutputPps() {
		pps = uint64(util.StringToInt(t, rate))
	}
	return pps
}
