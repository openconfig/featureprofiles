// Package verifiers provides verifiers APIs to verify oper data for different component verifications.
package verifiers

import (
	"context"
	"fmt"
	// "os"
	"testing"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/ondatra"
	textfsm "github.com/openconfig/featureprofiles/exec/utils/textfsm/textfsm"
)

type interfaceVerifier struct{}

// GetIngressTrafficInterfaces gets list of the interfaces which have active ingress unicast traffic,
// this is based on counters incremented while traffic is running. "trafficType" is the type of traffic either "ipv4" or "ipv6", default is interface level unicast packets.
func (v *interfaceVerifier) VerifyInterfaceOperStatus(t testing.T, dut *ondatra.DUTDevice, trafficType string) bool {
	return false
}

// VerifyShowInterfaceCLI returns interface data from the show CLI.
func (v *interfaceVerifier) VerifyShowInterfaceCLI(t *testing.T, ctx context.Context, dut *ondatra.DUTDevice, intfName string) textfsm.ShowInterface{
	cliOutput := config.CMDViaGNMI(ctx, t, dut, fmt.Sprintf("show interface %s", intfName))
	intfTextfsm := textfsm.ShowInterface{}
	if err := intfTextfsm.Parse(cliOutput); err != nil {
		t.Fatalf("%v", err)
	}
	t.Logf("%+v\n", intfTextfsm)
	t.Log("WAIT")
	return intfTextfsm
}
