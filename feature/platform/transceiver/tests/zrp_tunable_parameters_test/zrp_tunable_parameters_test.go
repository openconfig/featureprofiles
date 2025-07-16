package zrp_tunable_parameters_test

import (
	"flag"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	samplingInterval   = 10 * time.Second
	frequencyTolerance = 1800
	timeout            = 10 * time.Minute
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: 30,
	}
	operationalModeFlag = flag.Int("operational_mode", 5, "vendor-specific operational-mode for the channel")
	operationalMode     uint16
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func Test400ZRPlusTunableFrequency(t *testing.T) {
	if operationalModeFlag != nil {
		operationalMode = uint16(*operationalModeFlag)
	} else {
		t.Fatalf("Please specify the vendor-specific operational-mode flag")
	}
	dut := ondatra.DUT(t, "dut")
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
	oc1 := components.OpticalChannelComponentFromPort(t, dut, p1)
	oc2 := components.OpticalChannelComponentFromPort(t, dut, p2)
	streamOC1 := samplestream.New(t, dut, gnmi.OC().Component(oc1).State(), samplingInterval)
	defer streamOC1.Close()
	streamOC2 := samplestream.New(t, dut, gnmi.OC().Component(oc2).State(), samplingInterval)
	defer streamOC2.Close()
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
					if deviations.OperationalModeUnsupported(dut) {
						operationalMode = 0
					}
					cfgplugins.ConfigOpticalChannel(t, dut, oc1, freq, tc.targetOutputPower, operationalMode)
					cfgplugins.ConfigOpticalChannel(t, dut, oc2, freq, tc.targetOutputPower, operationalMode)
					gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
					gnmi.Await(t, dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
					validateOpticsTelemetry(t, []*samplestream.SampleStream[*oc.Component]{streamOC1, streamOC2}, freq, tc.targetOutputPower, oc.Interface_OperStatus_UP)
				})
			}
		})
	}

}
func Test400ZRPlusTunableOutputPower(t *testing.T) {
	if operationalModeFlag != nil {
		operationalMode = uint16(*operationalModeFlag)
	} else {
		t.Fatalf("Please specify the vendor-specific operational-mode flag")
	}
	dut := ondatra.DUT(t, "dut")
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
	oc1 := components.OpticalChannelComponentFromPort(t, dut, p1)
	oc2 := components.OpticalChannelComponentFromPort(t, dut, p2)
	streamOC1 := samplestream.New(t, dut, gnmi.OC().Component(oc1).State(), samplingInterval)
	defer streamOC1.Close()
	streamOC2 := samplestream.New(t, dut, gnmi.OC().Component(oc2).State(), samplingInterval)
	defer streamOC2.Close()
	tests := []struct {
		description            string
		frequency              uint64
		startTargetOutputPower float64
		endTargetOutputPower   float64
		targetOutputPowerStep  float64
	}{
		{
			// Validate adjustable range of transmit output power across -7 to 0 dBm
			// range in steps of 1dB. So the module’s output power will be set to -7,
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
				if deviations.OperationalModeUnsupported(dut) {
					operationalMode = 0
				}

				cfgplugins.ConfigOpticalChannel(t, dut, oc1, tc.frequency, top, operationalMode)
				cfgplugins.ConfigOpticalChannel(t, dut, oc2, tc.frequency, top, operationalMode)
				gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
				gnmi.Await(t, dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
				validateOpticsTelemetry(t, []*samplestream.SampleStream[*oc.Component]{streamOC1, streamOC2}, tc.frequency, top, oc.Interface_OperStatus_UP)
			})
		}
	}
}

func Test400ZRPlusInterfaceFlap(t *testing.T) {
	if operationalModeFlag != nil {
		operationalMode = uint16(*operationalModeFlag)
	} else {
		t.Fatalf("Please specify the vendor-specific operational-mode flag")
	}
	dut := ondatra.DUT(t, "dut")
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
	oc1 := components.OpticalChannelComponentFromPort(t, dut, p1)
	oc2 := components.OpticalChannelComponentFromPort(t, dut, p2)
	streamOC1 := samplestream.New(t, dut, gnmi.OC().Component(oc1).State(), samplingInterval)
	defer streamOC1.Close()
	streamOC2 := samplestream.New(t, dut, gnmi.OC().Component(oc2).State(), samplingInterval)
	defer streamOC2.Close()
	targetPower := float64(-3)
	frequency := uint64(193100000)

	if deviations.OperationalModeUnsupported(dut) {
		operationalMode = 0
	}

	cfgplugins.ConfigOpticalChannel(t, dut, oc1, frequency, targetPower, operationalMode)
	cfgplugins.ConfigOpticalChannel(t, dut, oc2, frequency, targetPower, operationalMode)
	gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
	gnmi.Await(t, dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
	t.Run("Telemetry before flap", func(t *testing.T) {
		validateOpticsTelemetry(t, []*samplestream.SampleStream[*oc.Component]{streamOC1, streamOC2}, frequency, targetPower, oc.Interface_OperStatus_UP)
	})
	// Disable or shut down the interface on the DUT.
	cfgplugins.ToggleInterface(t, dut, p1.Name(), false)
	cfgplugins.ToggleInterface(t, dut, p2.Name(), false)
	gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_DOWN)
	gnmi.Await(t, dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_DOWN)

	// Verify with interfaces in down state both optics are still streaming
	// configured value for frequency.
	// Verify for the TX output power with interface in down state a decimal64
	// value of -40 dB is streamed.
	t.Run("Telemetry during interface disabled", func(t *testing.T) {
		validateOpticsTelemetry(t, []*samplestream.SampleStream[*oc.Component]{streamOC1, streamOC2}, frequency, -40, oc.Interface_OperStatus_DOWN)
	})
	// Re-enable the interfaces on the DUT.
	cfgplugins.ToggleInterface(t, dut, p1.Name(), true)
	cfgplugins.ToggleInterface(t, dut, p2.Name(), true)
	gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
	gnmi.Await(t, dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
	// Verify the ZR optics tune back to the correct frequency and TX output
	// power as per the configuration and related telemetry values are updated
	// to the value in the normal range again.
	t.Run("Telemetry after flap", func(t *testing.T) {
		validateOpticsTelemetry(t, []*samplestream.SampleStream[*oc.Component]{streamOC1, streamOC2}, frequency, targetPower, oc.Interface_OperStatus_UP)
	})
}

func validateOpticsTelemetry(t *testing.T, streams []*samplestream.SampleStream[*oc.Component], frequency uint64, outputPower float64, operStatus oc.E_Interface_OperStatus) {
	dut := ondatra.DUT(t, "dut")
	var ocs []*oc.Component_OpticalChannel
	for _, s := range streams {
		val := s.Next()
		if val == nil {
			t.Fatal("Optical channel streaming telemetry not received")
		}
		v, ok := val.Val()
		if !ok {
			t.Fatal("Optical channel streaming telemetry empty")
		}
		ocs = append(ocs, v.GetOpticalChannel())
	}

	for _, _oc := range ocs {
		opm := _oc.GetOperationalMode()
		inst := _oc.GetCarrierFrequencyOffset().GetInstant()
		avg := _oc.GetCarrierFrequencyOffset().GetAvg()
		min := _oc.GetCarrierFrequencyOffset().GetMin()
		max := _oc.GetCarrierFrequencyOffset().GetMax()
		if got, want := opm, uint16(operationalMode); got != want && !deviations.OperationalModeUnsupported(dut) {
			t.Errorf("Optical-Channel: operational-mode: got %v, want %v", got, want)
		}
		// Laser frequency offset should not be more than +/- 1.8 GHz max from the
		// configured centre frequency.
		if inst < -1*frequencyTolerance || inst > frequencyTolerance {
			t.Errorf("Optical-Channel: carrier-frequency-offset not in tolerable range, got: %v, want: (+/-)%v", inst, frequencyTolerance)
		}
		for _, ele := range []any{inst, min, max, avg} {
			if reflect.TypeOf(ele).Kind() != reflect.Float64 {
				t.Fatalf("Value %v is not type float64", ele)
			}
		}
		if deviations.MissingZROpticalChannelTunableParametersTelemetry(dut) {
			t.Log("Skipping Min/Max/Avg Tunable Parameters Telemetry validation. Deviation MissingZROpticalChannelTunableParametersTelemetry enabled.")
		} else {
			// For reported data check for validity: min <= avg/instant <= max
			if min > math.Round(inst) {
				t.Errorf("Optical-Channel: carrier-frequency-offset min: %v greater than carrier-frequency-offset instant: %v", min, inst)
			}
			if max < math.Round(inst) {
				t.Errorf("Optical-Channel: carrier-frequency-offset max: %v less than carrier-frequency-offset instant: %v", max, inst)
			}
			if min > math.Round(avg) {
				t.Errorf("Optical-Channel: carrier-frequency-offset min: %v greater than carrier-frequency-offset avg: %v", min, avg)
			}
			if max < math.Round(avg) {
				t.Errorf("Optical-Channel: carrier-frequency-offset max: %v less than carrier-frequency-offset avg: %v", max, avg)
			}
		}
		inst = _oc.GetOutputPower().GetInstant()
		avg = _oc.GetOutputPower().GetAvg()
		min = _oc.GetOutputPower().GetMin()
		max = _oc.GetOutputPower().GetMax()
		// When set to a specific target output power, transmit power control
		// absolute accuracy should be within +/- 1 dBm of the target configured
		// output power.
		switch operStatus {
		case oc.Interface_OperStatus_UP:
			if inst < outputPower-1 || inst > outputPower+1 {
				t.Errorf("Optical-Channel: output-power not in tolerable range, got: %v, want: %v", inst, outputPower)
			}
		case oc.Interface_OperStatus_DOWN:
			if inst != -40 {
				t.Errorf("Optical-Channel: output-power not in tolerable range, got: %v, want: %v", inst, -40)
			}
		}
		for _, ele := range []any{inst, min, max, avg} {
			if reflect.TypeOf(ele).Kind() != reflect.Float64 {
				t.Fatalf("Value %v is not type float64", ele)
			}
		}
		if deviations.MissingZROpticalChannelTunableParametersTelemetry(dut) {
			t.Log("Skipping Min/Max/Avg Tunable Parameters Telemetry validation. Deviation MissingZROpticalChannelTunableParametersTelemetry enabled.")
		} else {
			// For reported data check for validity: min <= avg/instant <= max
			if min > math.Round(inst) {
				t.Errorf("Optical-Channel: output-power min: %v greater than output-power instant: %v", min, inst)
			}
			if max < math.Round(inst) {
				t.Errorf("Optical-Channel: output-power max: %v less than output-power instant: %v", max, inst)
			}
			if min > math.Round(avg) {
				t.Errorf("Optical-Channel: output-power min: %v greater than output-power avg: %v", min, avg)
			}
			if max < math.Round(avg) {
				t.Errorf("Optical-Channel: output-power max: %v less than output-power avg: %v", max, avg)
			}
		}
		if got, want := _oc.GetFrequency(), frequency; got != want {
			t.Errorf("Optical-Channel: frequency: %v, want: %v", got, want)
		}
	}
}
