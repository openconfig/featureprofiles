package zrp_tunable_parameters_test

import (
	"flag"
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/telemetry/transceiver"
	"github.com/openconfig/ondatra"
)

var (
	operationalModeFlag = flag.Int("operational_mode", 0, "vendor-specific operational-mode for the channel")
	operationalMode     uint16
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func initializeTunableParamsTest(t *testing.T) {
	t.Helper()
	operationalMode = uint16(*operationalModeFlag)
	operationalMode = cfgplugins.InterfaceInitialize(t, ondatra.DUT(t, "dut"), operationalMode)
}

func Test400ZRPlusTunableFrequency(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	initializeTunableParamsTest(t)
	cfgplugins.InterfaceConfig(t, dut, dut.Port(t, "port1"))
	cfgplugins.InterfaceConfig(t, dut, dut.Port(t, "port2"))

	tests := []struct {
		description       string
		startFreq         uint64
		endFreq           uint64
		freqStep          uint64
		targetOutputPower float64
	}{
		{
			// Validate setting 400ZR++ optics module tunable laser center frequency
			// across frequency range 196.100 - 191.400 THz for 100GHz grid.
			description:       "100GHz grid",
			startFreq:         191400000,
			endFreq:           196100000,
			freqStep:          100000 * 4,
			targetOutputPower: -3,
		},
		{
			// Validate setting 400ZR++ optics module tunable laser center frequency
			// across frequency range 196.100 - 191.375 THz for 75GHz grid.
			description:       "75GHz grid",
			startFreq:         191375000,
			endFreq:           196100000,
			freqStep:          75000 * 6,
			targetOutputPower: -3,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			for freq := tc.startFreq; freq <= tc.endFreq; freq += tc.freqStep {
				t.Run(fmt.Sprintf("Freq: %v", freq), func(t *testing.T) {
					transceiver.TunableParamsTest(t, &transceiver.TunableParams{
						OpMode:      operationalMode,
						Freq:        freq,
						OutputPower: tc.targetOutputPower,
					})
				})
			}
		})
	}
}

func Test400ZRPlusTunableOutputPower(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	initializeTunableParamsTest(t)
	cfgplugins.InterfaceConfig(t, dut, dut.Port(t, "port1"))
	cfgplugins.InterfaceConfig(t, dut, dut.Port(t, "port2"))
	tests := []struct {
		description            string
		frequency              uint64
		startTargetOutputPower float64
		endTargetOutputPower   float64
		targetOutputPowerStep  float64
	}{
		{
			// Validate adjustable range of transmit output power across -7 to 0 dBm
			// range in steps of 1dB. So the module's output power will be set to -7,
			// -6,-5, -4, -3, -2, -1, 0 dBm in each step.
			description:            "adjustable range of transmit output power across -7 to 0 dBm range in steps of 1dB",
			frequency:              193100000,
			startTargetOutputPower: -7,
			endTargetOutputPower:   0,
			targetOutputPowerStep:  1,
		},
	}
	for _, tc := range tests {
		for top := tc.startTargetOutputPower; top <= tc.endTargetOutputPower; top += tc.targetOutputPowerStep {
			t.Run(fmt.Sprintf("Target Power: %v", top), func(t *testing.T) {
				transceiver.TunableParamsTest(t, &transceiver.TunableParams{
					OpMode:      operationalMode,
					Freq:        tc.frequency,
					OutputPower: top,
				})
			})
		}
	}
}

func Test400ZRPlusInterfaceFlap(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	initializeTunableParamsTest(t)
	cfgplugins.InterfaceConfig(t, dut, dut.Port(t, "port1"))
	cfgplugins.InterfaceConfig(t, dut, dut.Port(t, "port2"))

	transceiver.TunableParamsInterfaceFlapTest(t, &transceiver.TunableParams{
		OpMode:      operationalMode,
		Freq:        193100000,
		OutputPower: -3,
	})
}
