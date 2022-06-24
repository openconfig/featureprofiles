package qos_test

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	baseConfigFile          = "base_config_interface.json"
	testNameInput  []string = []string{
		"pmap9_new",
	}
	testTypeInput []oc.E_Input_Classifier_Type = []oc.E_Input_Classifier_Type{
		oc.E_Input_Classifier_Type(1),
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

func setupQos(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bc := BaseConfig()
	setup.ResetStruct(bc, []string{"Interface", "Classifier"})
	bcClassifier := setup.GetAnyValue(bc.Classifier)
	bcInterface := setup.GetAnyValue(bc.Interface)
	dut.Config().Qos().Classifier(*bcClassifier.Name).Update(t, bcClassifier)
	dut.Config().Qos().Interface(*bcInterface.InterfaceId).Update(t, bcInterface)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
