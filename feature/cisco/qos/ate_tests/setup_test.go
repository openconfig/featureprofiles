package qos_test

import (
	"fmt"

	"os"
	"sort"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
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

	gnmi.Update(t, dut, gnmi.OC().Qos().Config(), bc)
	return bc
}

func setupQosIpv6(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bc := BaseConfigipv6()
	setup.ResetStruct(bc, []string{"Interface", "Classifier", "ForwardingGroup"})

	gnmi.Update(t, dut, gnmi.OC().Qos().Config(), bc)
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
		gnmi.Update(t, dut, gnmi.OC().Qos().Queue(k).Config(), bce.Queue[k])
	}
	setup.ResetStruct(bce, []string{"SchedulerPolicy"})
	bcSchedulerPolicy := setup.GetAnyValue(bce.SchedulerPolicy)
	gnmi.Update(t, dut, gnmi.OC().Qos().SchedulerPolicy(*bcSchedulerPolicy.Name).Config(), bcSchedulerPolicy)
	bcee := BaseConfigEgress()
	for inter, value := range bcee.Interface {
		fmt.Printf("key :%+v and val:%+v", inter, *(value.Output.SchedulerPolicy))
		gnmi.Update(t, dut, gnmi.OC().Qos().Interface(inter).Config(), value)
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
			gnmi.Delete(t, dut, gnmi.OC().Qos().Config())
		})
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Errorf(*err)
	}
}
