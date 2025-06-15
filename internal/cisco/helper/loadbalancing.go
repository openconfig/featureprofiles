// Package loadbalancing provides verifiers APIs to verify oper data for loadbalancing verifications.
package helper

import (
	"time"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"

	// "github.com/openconfig/ondatra/gnmi/oc"
	"testing"
)

type LoadbalancingHelper struct{}

// GetIngressTrafficInterfaces gets list of the interfaces which have active ingress unicast traffic,
// this is based on counters incremented while traffic is running. "trafficType" is the type of traffic either "ipv4" or "ipv6", default is interface level unicast packets.
func (v *LoadbalancingHelper) GetIngressTrafficInterfaces(t *testing.T, dut *ondatra.DUTDevice, trafficType string) map[string]uint64 {
	intfMap := make(map[string]uint64)
	//Get initial counters for all interfaces
	beforeCounters := Interface.GetAllInterfaceInUnicast(t, dut, trafficType)
	time.Sleep(30 * time.Second) //sleep for 30 seconds for interface statsd cache to update
	//Get counters for all interfaces after interface statsd cache update
	afterCounters := Interface.GetAllInterfaceInUnicast(t, dut, trafficType)
	// Subtract values for each key
	differenceCounters := make(map[string]uint64)
	for key, beforeValue := range beforeCounters {
		if afterValue, exists := afterCounters[key]; exists {
			// Perform subtraction and store in the new map
			differenceCounters[key] = afterValue - beforeValue
		}
	}
	for key, value := range differenceCounters {
		if value != 0 {
			intfMap[key] = value
		}
	}
	t.Log("intfMap", intfMap)
	return intfMap

}

// GetPrefixOutGoingInterfaces retrieves all outgoing next-hop interfaces with their weights for a given prefix.
func (v *LoadbalancingHelper) GetPrefixOutGoingInterfaces(t *testing.T, dut *ondatra.DUTDevice, prefix, vrf string) map[string]uint32 {
	// Validate input
	NHWeightMap := make(map[string]uint32)
	aftIPv4Path := gnmi.OC().NetworkInstance(vrf).Afts().Ipv4Entry(prefix).State()
	aftGet := gnmi.Get(t, dut, aftIPv4Path)
	t.Log("aftIPv4Path", aftGet)

	return NHWeightMap
}

// normalize normalizes the input values so that the output values sum
// to 1.0 but reflect the proportions of the input.  For example,
// input [1, 2, 3, 4] is normalized to [0.1, 0.2, 0.3, 0.4].
func (v *LoadbalancingHelper) Normalize(xs []uint64) (ys []float64, sum uint64) {
	for _, x := range xs {
		sum += x
	}
	ys = make([]float64, len(xs))
	for i, x := range xs {
		ys[i] = float64(x) / float64(sum)
	}
	return ys, sum
}
