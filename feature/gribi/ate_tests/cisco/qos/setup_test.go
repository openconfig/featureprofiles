package cisco_gribi_test

import (
	"fmt"
	"io/ioutil"
	"sort"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	baseConfigFile               = "base_config_interface.json"
	baseConfigEgressFile         = "scheduler_base1.json"
	testSetDscpInput     []uint8 = []uint8{
		63,
	}
	testSetMplsTcInput []uint8 = []uint8{
		7,
	}
)

func BaseConfig() *oc.Qos {
	sl := strings.Split(setup.FindTestDataPath(), "/")
	sl = sl[:len(sl)-1]
	baseConfigPath := strings.Join(sl, "/") + "/" + baseConfigFile
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

func BaseConfigEgress() *oc.Qos {
	sl := strings.Split(setup.FindTestDataPath(), "/")
	sl = sl[:len(sl)-1]
	baseConfigPath := strings.Join(sl, "/") + "/" + baseConfigEgressFile
	jsonConfig, err := ioutil.ReadFile(baseConfigPath)
	if err != nil {
		panic(fmt.Sprintf("Cannot load base config: %v", err))
	}

	baseConfigEgress := new(oc.Qos)
	if err := oc.Unmarshal(jsonConfig, baseConfigEgress); err != nil {
		panic(fmt.Sprintf("Cannot unmarshal base config: %v", err))
	}
	return baseConfigEgress
	println(baseConfigEgress)
}

func setupQos(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bc := BaseConfig()
	setup.ResetStruct(bc, []string{"Interface", "Classifier"})
	bcClassifier := setup.GetAnyValue(bc.Classifier)
	bcInterface := setup.GetAnyValue(bc.Interface)
	dut.Config().Qos().Classifier(*bcClassifier.Name).Update(t, bcClassifier)
	dut.Config().Qos().Interface(*bcInterface.InterfaceId).Update(t, bcInterface)
	return bc

}

func setupQosEgress(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bce := BaseConfigEgress()
	fmt.Printf("%+v\n", bce.Queue)
	var keys []string
	for ke, _ := range bce.Queue {
		keys = append(keys, ke)
	}

	sort.Sort(sort.Reverse(sort.StringSlice(keys)))

	for _, k := range keys {
		fmt.Println("KEY: ", k, "VAL: ", bce.Queue[k])
		//val, ok := bce.Queue[k]
		dut.Config().Qos().Queue(k).Update(t, bce.Queue[k])
	}
	setup.ResetStruct(bce, []string{"SchedulerPolicy", "Interface"})

	bcSchedulerPolicy := setup.GetAnyValue(bce.SchedulerPolicy)
	//bcInterface := setup.GetAnyValue(bce.Interface)
	dut.Config().Qos().SchedulerPolicy(*bcSchedulerPolicy.Name).Update(t, bcSchedulerPolicy)
	//dut.Config().Qos().Interface(*bcInterface.InterfaceId).Update(t, bcInterface)
	for inter, value := range bce.Interface {
		dut.Config().Qos().Interface(inter).Update(t, value)

	}

	return bce

}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
