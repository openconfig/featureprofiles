// Package intf provides verifiers APIs to verify oper data for node interface.
package verifiers

import (
	"github.com/openconfig/ondatra"
	"testing"
)

type Interfaces struct{}

// GetIngressTrafficInterfaces gets list of the interfaces which have active ingress unicast traffic,
// this is based on counters incremented while traffic is running. "trafficType" is the type of traffic either "ipv4" or "ipv6", default is interface level unicast packets.
func (v *Interfaces) VerifyInterfaceOperStatus(t *testing.T, dut *ondatra.DUTDevice, trafficType string) bool {
	return false
}
