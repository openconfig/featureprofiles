package qos_test

import (
	"sort"
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	//	"github.com/openconfig/testt"
)

func setupQosEgress(t *testing.T, dut *ondatra.DUTDevice, baseConfigFile string) *oc.Qos {
	bc := setup.BaseConfig(baseConfigFile)

	keys := make([]string, 0, len(bc.Queue))
	for ke := range bc.Queue {
		keys = append(keys, ke)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	for _, k := range keys {
		gnmi.Update(t, dut, gnmi.OC().Qos().Queue(k).Config(), bc.Queue[k])
	}
	for bcSchedulerPolicyName, bcSchedulerPolicy := range bc.SchedulerPolicy {
		gnmi.Update(t, dut, gnmi.OC().Qos().SchedulerPolicy(bcSchedulerPolicyName).Config(), bcSchedulerPolicy)
	}
	for bcInterfaceId, bcInterface := range bc.Interface {
		gnmi.Update(t, dut, gnmi.OC().Qos().Interface(bcInterfaceId).Config(), bcInterface)
	}
	return bc
}
func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	gnmi.Delete(t, dut, gnmi.OC().Qos().Config())
}
