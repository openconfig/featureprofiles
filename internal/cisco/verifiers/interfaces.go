// Package verifiers provides verifiers APIs to verify oper data for different component verifications.
package verifiers

import (
	"context"
	"fmt"
	// "os"
	"testing"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/ondatra"
)

type interfaceVerifier struct{}

// GetIngressTrafficInterfaces gets list of the interfaces which have active ingress unicast traffic,
// this is based on counters incremented while traffic is running. "trafficType" is the type of traffic either "ipv4" or "ipv6", default is interface level unicast packets.
func (v *interfaceVerifier) VerifyInterfaceOperStatus(t testing.T, dut *ondatra.DUTDevice, trafficType string) bool {
	return false
}

// VerifyShowInterfaceCLI returns interface data from the show CLI.
func (v *interfaceVerifier) VerifyShowInterfaceCLI(t *testing.T, ctx context.Context, dut *ondatra.DUTDevice, intfName string) []map[string]any {
	// cwd, err := os.Getwd()
	// if err != nil {
	// 	t.Fatalf("error")
	// }
	interfaceTextfsmPath := "/Users/arvbaska/hash_lb_suite2/featureprofiles/internal/cisco/verifiers/textfsm_templates/interface/show_interface.textfsm"
	cliOutput := config.CMDViaGNMI(ctx, t, dut, fmt.Sprintf("show interface %s", intfName))
	results, err := ProcessTextFSM(interfaceTextfsmPath, cliOutput)
	if err != nil {
		t.Fatalf("error")
	}
	return results
}
