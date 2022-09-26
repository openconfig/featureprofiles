package qos_test

import (
	"fmt"

	"os"
	"sort"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/testt"
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
	jsonConfig, err := os.ReadFile(baseConfigPath)
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
	jsonConfig, err := os.ReadFile(baseConfigPath)
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
	jsonConfig, err := os.ReadFile(baseConfigPath)
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
	jsonConfig, err := os.ReadFile(baseConfigPath)
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
	jsonConfig, err := os.ReadFile(baseConfigPath)
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
	for ke := range bce.Queue {
		keys = append(keys, ke)
	}

	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	fmt.Printf("key is %+v", keys)

	for _, k := range keys {
		fmt.Println("KEY: ", k, "VAL: ", bce.Queue[k])
		dut.Config().Qos().Queue(k).Update(t, bce.Queue[k])
	}
	setup.ResetStruct(bce, []string{"SchedulerPolicy"})
	bcSchedulerPolicy := setup.GetAnyValue(bce.SchedulerPolicy)
	dut.Config().Qos().SchedulerPolicy(*bcSchedulerPolicy.Name).Update(t, bcSchedulerPolicy)
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
	keys := make([]string, 0, len(bce.Queue))
	for ke := range bce.Queue {
		keys = append(keys, ke)
	}

	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	fmt.Printf("key is %+v", keys)

	for _, k := range keys {
		fmt.Println("KEY: ", k, "VAL: ", bce.Queue[k])
		dut.Config().Qos().Queue(k).Update(t, bce.Queue[k])
	}
	setup.ResetStruct(bce, []string{"SchedulerPolicy"})
	bcSchedulerPolicy := setup.GetAnyValue(bce.SchedulerPolicy)
	dut.Config().Qos().SchedulerPolicy(*bcSchedulerPolicy.Name).Update(t, bcSchedulerPolicy)
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
	var err *string
	for attempt := 1; attempt <= 2; attempt++ {
		err = testt.CaptureFatal(t, func(t testing.TB) {
			dut.Config().Qos().Delete(t)
		})
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Errorf(*err)
	}
}
