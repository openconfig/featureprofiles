package qos_test

import (
	"fmt"
	"testing"
	"io/ioutil"
	"strings"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	baseConfigFile = "scheduler_base.json"
	baseconfigFile1 = "Scheduler_base1.json"
	testNameInput []string = []string{
		"tc2","tc3","tc4","tc5","tc6","tc7",
	}
	inputDscp = 4
	testNameInputReverse []string = []string{
		"tc7","tc6","tc5","tc4","tc3","tc2",
	}
	testNameInput1 []string = []string{
		"tc6","tc5","tc4","tc3","tc2",
	}
	testNamescheduler []string = []string{
		"eg_policy1111","tc5","tc4","tc3","tc2",
	}
	testNameInterface  []interfaceScheduler 
	testPrioritychange = oc.E_Scheduler_Priority(1)
	testPriorityInput []oc.E_Scheduler_Priority = []oc.E_Scheduler_Priority{
		oc.E_Scheduler_Priority(1), //STRICT
	}
	testSequenceInput []uint32 = []uint32{
		2311126647,
	}
	testTypeInput []oc.E_QosTypes_QOS_SCHEDULER_TYPE = []oc.E_QosTypes_QOS_SCHEDULER_TYPE{
		oc.E_QosTypes_QOS_SCHEDULER_TYPE(2), //TWO_RATE_THREE_COLOR
	}
)

type Params struct {
	filename string
  }
  
type interfaceScheduler struct {
	interfaceId string
	policyName string
  }


func BaseConfig(p  Params) *oc.Qos {
	sl := strings.Split(setup.FindTestDataPath(), "/")
	sl = sl[:len(sl)-1]
	var baseConfigPath string
	if p.filename == "" {
		baseConfigPath = strings.Join(sl, "/") + "/" + baseConfigFile
	} else {
		baseConfigPath = strings.Join(sl, "/") + "/" + p.filename
	}
	jsonConfig, err := ioutil.ReadFile(baseConfigPath)
	if err != nil {
		panic(fmt.Sprintf("Cannot load base config: %v", err))
	}

	baseConfig := new(oc.Qos)
	if err := oc.Unmarshal(jsonConfig, baseConfig); err != nil {
		panic(fmt.Sprintf("Cannot unmarshal base config: %v", err))
	}
	return baseConfig
}

func setupInitQos1 (t *testing.T, dut *ondatra.DUTDevice ,p Params) *oc.Qos {
	var bc *oc.Qos
	if p.filename == "" {
		bc = BaseConfig(Params{filename : "scheduler_base1.json"})
	}else {
		bc = BaseConfig(Params{filename : p.filename})
	}	
	dut.Config().Qos().Update(t, bc)
	return bc
}

func setupInitQos (t *testing.T , dut *ondatra.DUTDevice) *oc.Qos {
	bc := BaseConfig(Params{filename : "scheduler_base1.json"})
	dut.Config().Qos().Update(t, bc)
	return bc
}

func setupQos(t *testing.T, dut *ondatra.DUTDevice, file string) *oc.Qos {
	bc := setup.BaseConfig()
	setup.ResetStruct(bc, []string{"SchedulerPolicy"})
	//dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}

func init() {
	testNameInterface = []interfaceScheduler{
		interfaceScheduler{
			interfaceId: "FourHundredGigE0/0/0/0",
			policyName: "eg_policy1111",
		},
		interfaceScheduler{
			interfaceId: "FourHundredGigE0/0/0/1",
			policyName: "eg_policy2222",
		},
	}
}