package qos_test

import (
	"fmt"
	"sort"
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	testInterfaceIdInput []string = []string{
		"FourHundredGigE0/0/0/1",
		"Bundle-Ether120",
	}
)

// Setting up everything in a single config doesn't work due to the sch pol queue ordering issue
func setupQos(t *testing.T, dut *ondatra.DUTDevice, baseConfigFile string) *oc.Qos {
	bc := setup.BaseConfig(baseConfigFile)
	setup.ResetStruct(bc, []string{"Interface", "Classifier", "SchedulerPolicy", "ForwardingGroup", "Queue"})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func setupQosIngress(t *testing.T, dut *ondatra.DUTDevice, baseConfigFile string) *oc.Qos {
	bc := setup.BaseConfig(baseConfigFile)
	setup.ResetStruct(bc, []string{"Interface", "Classifier"})
	dut.Config().Qos().Update(t, bc)
	return bc
}

func setupQosEgress(t *testing.T, dut *ondatra.DUTDevice, baseConfigFile string) *oc.Qos {
	bce := setup.BaseConfig(baseConfigFile)
	fmt.Printf("%+v\n", bce.Queue)
	//keys := make(make([]string, 0, len(bce.Queue))
	//for k, v := range bce.Queue {
	//fmt.Printf("key is %+v and value is %+v\n", k, *(v.Name))
	//      dut.Config().Qos().Queue(k).Update(t, v)
	//}
	keys := make([]string, 0, len(bce.Queue))
	//var keys []string
	for ke, _ := range bce.Queue {
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
	bcee := setup.BaseConfig(baseConfigFile)
	for inter, value := range bcee.Interface {
		fmt.Printf("key :%+v and val:%+v", inter, *(value.Output.SchedulerPolicy))
		dut.Config().Qos().Interface(inter).Update(t, value)
	}
	return bce

}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
