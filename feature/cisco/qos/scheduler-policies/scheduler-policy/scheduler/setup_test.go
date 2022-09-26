package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
	//	"github.com/openconfig/testt"
)

var (
	// baseConfigFile           = "scheduler_base.json"
	// baseconfigFile1          = "Scheduler_base1.json"
	testNameInput []string = []string{
		"tc2", "tc3", "tc4", "tc5", "tc6", "tc7",
	}
	testNameInputReverse []string = []string{
		"tc7", "tc6", "tc5", "tc4", "tc3", "tc2",
	}
	testNameInput1 []string = []string{
		"tc6", "tc5", "tc4", "tc3", "tc2",
	}
	testNameInterface []interfaceScheduler
)

type interfaceScheduler struct {
	interfaceId string
	policyName  string
}

func setupQos(t *testing.T, dut *ondatra.DUTDevice, baseConfigFile string) *oc.Qos {
	bc := setup.BaseConfig(baseConfigFile)
	dut.Config().Qos().Update(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}

func init() {
	testNameInterface = []interfaceScheduler{
		{
			interfaceId: "FourHundredGigE0/0/0/0",
			policyName:  "eg_policy1111",
		},
		{
			interfaceId: "FourHundredGigE0/0/0/1",
			policyName:  "eg_policy2222",
		},
	}
}
