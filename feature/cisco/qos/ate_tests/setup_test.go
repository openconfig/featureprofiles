package qos_test

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
	baseConfigFile           = "base_config_interface.json"
	baseConfigEgressFile     = "base_config_interface_scheduler.json"
	baseConfigFileSche       = "base_config_interface_test.json"
	baseConfigEgressFileSche = "scheduler_base1_test.json"
	baseConfigFileIpv6       = "base_config_interface_ipv6.json"
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
func BaseConfigSche() *oc.Qos {
	sl := strings.Split(setup.FindTestDataPath(), "/")
	sl = sl[:len(sl)-1]
	baseConfigPath := strings.Join(sl, "/") + "/" + baseConfigFileSche
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
func BaseConfigipv6() *oc.Qos {
	sl := strings.Split(setup.FindTestDataPath(), "/")
	sl = sl[:len(sl)-1]
	baseConfigPath := strings.Join(sl, "/") + "/" + baseConfigFileIpv6
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
	println(baseConfigEgress)
	return baseConfigEgress
}
func BaseConfigEgressSche() *oc.Qos {
	sl := strings.Split(setup.FindTestDataPath(), "/")
	sl = sl[:len(sl)-1]
	baseConfigPath := strings.Join(sl, "/") + "/" + baseConfigEgressFileSche
	jsonConfig, err := ioutil.ReadFile(baseConfigPath)
	if err != nil {
		panic(fmt.Sprintf("Cannot load base config: %v", err))
	}

	baseConfigEgress := new(oc.Qos)
	if err := oc.Unmarshal(jsonConfig, baseConfigEgress); err != nil {
		panic(fmt.Sprintf("Cannot unmarshal base config: %v", err))
	}
	println(baseConfigEgress)
	return baseConfigEgress
}

func setupQos(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bc := BaseConfig()
	setup.ResetStruct(bc, []string{"Interface", "Classifier", "ForwardingGroup"})

	dut.Config().Qos().Update(t, bc)
	return bc

}

func setupQosIpv6(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bc := BaseConfigipv6()
	setup.ResetStruct(bc, []string{"Interface", "Classifier", "ForwardingGroup"})

	dut.Config().Qos().Update(t, bc)
	return bc

}

func setupQosSche(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bc := BaseConfigSche()
	setup.ResetStruct(bc, []string{"Interface", "Classifier", "ForwardingGroup"})

	dut.Config().Qos().Update(t, bc)
	return bc

}
func setupQosTele(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bc := BaseConfig()
	setup.ResetStruct(bc, []string{"Interface", "Classifier"})

	return bc
}
func setupQosEgress(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bce := BaseConfigEgress()
	fmt.Printf("%+v\n", bce.Queue)

	keys := make([]string, 0, len(bce.Queue))
	//var keys []string
	for ke := range bce.Queue {
		keys = append(keys, ke)
	}

	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	fmt.Printf("key is %+v", keys)

	for _, k := range keys {
		fmt.Println("KEY: ", k, "VAL: ", bce.Queue[k])
		//val, ok := bce.Queue[k]
		dut.Config().Qos().Queue(k).Update(t, bce.Queue[k])
	}
	//var myQ *oc.Qos_Queue
	setup.ResetStruct(bce, []string{"SchedulerPolicy"})
	//myQ = setup.GetAnyValue(bce.Queue)
	//fmt.Println("myQ",*myQ.Name)
	//dut.Config().Qos().Queue(*bcQueue.Name).Update(t, bce.Queue)
	bcSchedulerPolicy := setup.GetAnyValue(bce.SchedulerPolicy)
	//bcInterface := setup.GetAnyValue(bce.Interface)
	//fmt.Println("*********QUEUE", bce.Queue, "BCEQUEUE", bcQueue)
	dut.Config().Qos().SchedulerPolicy(*bcSchedulerPolicy.Name).Update(t, bcSchedulerPolicy)
	//dut.Config().Qos().Interface(*bcInterface.InterfaceId).Update(t, bcInterface)
	bcee := BaseConfigEgress()
	for inter, value := range bcee.Interface {
		fmt.Printf("key :%+v and val:%+v", inter, *(value.Output.SchedulerPolicy))
		dut.Config().Qos().Interface(inter).Update(t, value)
	}
	return bce

}
func setupQosEgressSche(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bce := BaseConfigEgressSche()
	fmt.Printf("%+v\n", bce.Queue)
	//keys := make(make([]string, 0, len(bce.Queue))
	//for k, v := range bce.Queue {
	//fmt.Printf("key is %+v and value is %+v\n", k, *(v.Name))
	//      dut.Config().Qos().Queue(k).Update(t, v)
	//}
	keys := make([]string, 0, len(bce.Queue))
	//var keys []string
	for ke := range bce.Queue {
		keys = append(keys, ke)
	}

	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	fmt.Printf("key is %+v", keys)

	for _, k := range keys {
		fmt.Println("KEY: ", k, "VAL: ", bce.Queue[k])
		//val, ok := bce.Queue[k]
		dut.Config().Qos().Queue(k).Update(t, bce.Queue[k])
	}
	//var myQ *oc.Qos_Queue
	setup.ResetStruct(bce, []string{"SchedulerPolicy"})
	//myQ = setup.GetAnyValue(bce.Queue)
	//fmt.Println("myQ",*myQ.Name)
	//dut.Config().Qos().Queue(*bcQueue.Name).Update(t, bce.Queue)
	bcSchedulerPolicy := setup.GetAnyValue(bce.SchedulerPolicy)
	//bcInterface := setup.GetAnyValue(bce.Interface)
	//fmt.Println("*********QUEUE", bce.Queue, "BCEQUEUE", bcQueue)
	dut.Config().Qos().SchedulerPolicy(*bcSchedulerPolicy.Name).Update(t, bcSchedulerPolicy)
	//dut.Config().Qos().Interface(*bcInterface.InterfaceId).Update(t, bcInterface)
	bcee := BaseConfigEgress()
	for inter, value := range bcee.Interface {
		fmt.Printf("key :%+v and val:%+v", inter, *(value.Output.SchedulerPolicy))
		dut.Config().Qos().Interface(inter).Update(t, value)
	}
	return bce

}
func setupQosEgressTel(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bce := BaseConfigEgress()
	return bce
}
func teardownQos(t *testing.T, dut *ondatra.DUTDevice) {
        dut.Config().Qos().Delete(t)
}

