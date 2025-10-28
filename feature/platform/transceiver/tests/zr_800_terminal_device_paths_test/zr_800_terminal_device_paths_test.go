package zr_800_terminal_device_paths_test

import (
	"flag"
	"testing"

	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/telemetry/transceiver"
)

var (
	frequencyList          cfgplugins.FrequencyList
	targetOpticalPowerList cfgplugins.TargetOpticalPowerList
	operationalModeList    cfgplugins.OperationalModeList
)

func init() {
	flag.Var(&operationalModeList, "operational_mode", "operational-mode for the channel.")
	flag.Var(&frequencyList, "frequency", "frequency for the channel.")
	flag.Var(&targetOpticalPowerList, "target_optical_power", "target-optical-power for the channel.")
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestTerminalDevicePaths(t *testing.T) {
	transceiver.TerminalDevicePathsTest(t, &transceiver.TunableParamters{
		OperationalModeList:    operationalModeList,
		FrequencyList:          frequencyList,
		TargetOpticalPowerList: targetOpticalPowerList,
	})
}
