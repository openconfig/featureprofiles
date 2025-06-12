// Package loadbalancing provides verifiers APIs to verify oper data for loadbalancing verifications.
package verifiers

import (
	// "time"

	"github.com/openconfig/featureprofiles/internal/cisco/helper"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"text/tabwriter"
	// "github.com/openconfig/ygnmi/ygnmi"
	// "github.com/openconfig/ondatra/gnmi/oc"
	"testing"
)

type LoadbalancingVerifier struct{}

// VerifyTrafficDistributionPerWeight verifies the outgoing traffic distribution for a given map of [Interface]:Weight,
// this is based on counters incremented while traffic is running. "trafficType" is the type of traffic either "ipv4" or "ipv6", default is interface level unicast packets.
func (v *LoadbalancingVerifier) VerifyEgressTrafficDistributionPerWeight(t *testing.T, dut *ondatra.DUTDevice, OutIFWeight map[string]uint64, tolerance float32) map[string]float32 {
	trafficDistribution := make(map[string]float32)
	for intf, weight := range OutIFWeight {
		intfCounter := helper.Interface.GetPerInterfaceCounters(t, dut, intf)
		intfCounter.GetOutUnicastPkts()
	}

	t.Log("Print a table Outgoing interfaces, corresponding weight, OutPacket count & normalized distribution")
	return trafficDistribution
}
