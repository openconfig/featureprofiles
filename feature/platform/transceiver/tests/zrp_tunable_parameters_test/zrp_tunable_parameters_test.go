package zrp_tunable_parameters_test

import (
	"flag"
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
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

func Test400ZRPlusTunableFrequency(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	operationalMode = cfgplugins.InterfaceInitialize(t, dut, operationalMode)
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
					opticalChannel1Config := &oc.Component_OpticalChannel{
						TargetOutputPower: ygot.Float64(tc.targetOutputPower),
						Frequency:         ygot.Uint64(freq),
						OperationalMode:   ygot.Uint16(operationalMode)}
					opticalChannel2Config := &oc.Component_OpticalChannel{
						TargetOutputPower: ygot.Float64(tc.targetOutputPower),
						Frequency:         ygot.Uint64(freq),
						OperationalMode:   ygot.Uint16(operationalMode)}

					if deviations.OperationalModeUnsupported(dut) {
						opticalChannel1Config.OperationalMode = nil
						opticalChannel2Config.OperationalMode = nil
					}

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

	operationalMode = cfgplugins.InterfaceInitialize(t, dut, operationalMode)
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
				opticalChannel1Config := &oc.Component_OpticalChannel{
					TargetOutputPower: ygot.Float64(top),
					Frequency:         ygot.Uint64(tc.frequency),
					OperationalMode:   ygot.Uint16(operationalMode),
				}
				opticalChannel2Config := &oc.Component_OpticalChannel{
					TargetOutputPower: ygot.Float64(top),
					Frequency:         ygot.Uint64(tc.frequency),
					OperationalMode:   ygot.Uint16(operationalMode),
				}
				if deviations.OperationalModeUnsupported(dut) {
					opticalChannel1Config.OperationalMode = nil
					opticalChannel2Config.OperationalMode = nil
				}
				gnmi.Replace(t, dut, gnmi.OC().Component(oc1).OpticalChannel().Config(), opticalChannel1Config)
				gnmi.Replace(t, dut, gnmi.OC().Component(oc2).OpticalChannel().Config(), opticalChannel2Config)

				gotOPoc1, ok := gnmi.Watch(t, dut, gnmi.OC().Component(oc1).OpticalChannel().TargetOutputPower().State(), 2*time.Minute, func(val *ygnmi.Value[float64]) bool {
					outPower, ok := val.Val()
					return ok && outPower == top
				}).Await(t)
				if !ok {
					t.Fatalf("ERROR:Got output power: %v, but wanted output power: %v", gotOPoc1, top)
				}
				gotOPoc2, ok := gnmi.Watch(t, dut, gnmi.OC().Component(oc2).OpticalChannel().TargetOutputPower().State(), 2*time.Minute, func(val *ygnmi.Value[float64]) bool {
					outPower, ok := val.Val()
					return ok && outPower == top
				}).Await(t)
				if !ok {
					t.Fatalf("ERROR:Got output power: %v, but wanted output power: %v", gotOPoc2, top)
				}
				gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), interfaceTimeout, oc.Interface_OperStatus_UP)
				gnmi.Await(t, dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), interfaceTimeout, oc.Interface_OperStatus_UP)

				transceiver.TunableParamsTest(t, &transceiver.TunableParams{
					OpMode:      operationalMode,
					Freq:        tc.frequency,
					OutputPower: top,
				})
			})
		}
	}
}
