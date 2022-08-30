package qos_test

import (
	"sort"
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
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
		dut.Config().Qos().Queue(k).Update(t, bc.Queue[k])
	}
	for bcSchedulerPolicyName, bcSchedulerPolicy := range bc.SchedulerPolicy {
		dut.Config().Qos().SchedulerPolicy(bcSchedulerPolicyName).Update(t, bcSchedulerPolicy)
	}
	for bcInterfaceId, bcInterface := range bc.Interface {
		dut.Config().Qos().Interface(bcInterfaceId).Update(t, bcInterface)
	}
	return bc
}
func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
        dut.Config().Qos().Delete(t)
}

