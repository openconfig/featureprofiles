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
	baseConfigFile           = "base_config_classifier_term.json"
	testSetDscpInput []uint8 = []uint8{
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

func setupQos(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bc := BaseConfig()
	setup.ResetStruct(bc, []string{"Classifier"})
	dut.Config().Qos().Update(t, bc)
	// dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
