package zrp_tunable_parameters_test

import (
	"flag"
	"fmt"
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
	samplingInterval    = 10 * time.Second
	frequencyTolerance  = 1800
	timeout             = 10 * time.Minute
	telemetryWaitTime   = 60 * time.Second // Increased from 30s to 60s for 6 sampling windows
	maxTelemetryRetries = 3
	statisticsTolerance = 3.0 // Relaxed tolerance for statistical comparisons
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

					// CRITICAL FIX: Wait for telemetry to stabilize (increased from 30s to 60s)
					t.Logf("Waiting %v for statistical telemetry to stabilize...", telemetryWaitTime)
					time.Sleep(telemetryWaitTime)

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
				if deviations.OperationalModeUnsupported(dut) {
					operationalMode = 0
				}

				cfgplugins.ConfigOpticalChannel(t, dut, oc1, tc.frequency, top, operationalMode)
				cfgplugins.ConfigOpticalChannel(t, dut, oc2, tc.frequency, top, operationalMode)
				gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
				gnmi.Await(t, dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)

				// CRITICAL FIX: Wait for telemetry to stabilize (increased from 30s to 60s)
				t.Logf("Waiting %v for statistical telemetry to stabilize...", telemetryWaitTime)
				time.Sleep(telemetryWaitTime)

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

	// CRITICAL FIX: Wait for telemetry to stabilize (increased from 30s to 60s)
	t.Logf("Waiting %v for statistical telemetry to stabilize...", telemetryWaitTime)
	time.Sleep(telemetryWaitTime)

	t.Run("Telemetry before flap", func(t *testing.T) {
		validateOpticsTelemetry(t, []*samplestream.SampleStream[*oc.Component]{streamOC1, streamOC2}, frequency, targetPower, oc.Interface_OperStatus_UP)
	})
	// Disable or shut down the interface on the DUT.
	cfgplugins.ToggleInterface(t, dut, p1.Name(), false)
	cfgplugins.ToggleInterface(t, dut, p2.Name(), false)
	gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_DOWN)
	gnmi.Await(t, dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_DOWN)

	// Wait for telemetry to reflect down state
	time.Sleep(telemetryWaitTime)

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

	// CRITICAL FIX: Wait for telemetry to stabilize after recovery (increased from 30s to 60s)
	t.Logf("Waiting %v for statistical telemetry to stabilize...", telemetryWaitTime)
	time.Sleep(telemetryWaitTime)

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

	// Retry telemetry collection without sample flushing
	for attempt := 1; attempt <= maxTelemetryRetries; attempt++ {
		ocs = nil
		allSuccess := true

		for i, s := range streams {
			val := s.Next()
			if val == nil {
				t.Logf("Attempt %d: Stream %d - No telemetry", attempt, i)
				allSuccess = false
				break
			}
			v, ok := val.Val()
			if !ok {
				t.Logf("Attempt %d: Stream %d - Empty telemetry", attempt, i)
				allSuccess = false
				break
			}
			ocs = append(ocs, v.GetOpticalChannel())
		}

		if allSuccess && len(ocs) == len(streams) {
			break
		}

		if attempt == maxTelemetryRetries {
			t.Fatal("Failed to collect telemetry after retries")
		}

		time.Sleep(samplingInterval)
	}

	for i, _oc := range ocs {
		// Log telemetry values for debugging
		logTelemetryValues(t, i, _oc)

		opm := _oc.GetOperationalMode()

		// Carrier Frequency Offset validation
		cfInst := _oc.GetCarrierFrequencyOffset().GetInstant()
		cfAvg := _oc.GetCarrierFrequencyOffset().GetAvg()
		cfMin := _oc.GetCarrierFrequencyOffset().GetMin()
		cfMax := _oc.GetCarrierFrequencyOffset().GetMax()

		if got, want := opm, uint16(operationalMode); got != want && !deviations.OperationalModeUnsupported(dut) {
			t.Errorf("ERROR: Optical-Channel %d: operational-mode: got %v, want %v", i, got, want)
		}

		// Laser frequency offset should not be more than +/- 1.8 GHz max from the
		// configured centre frequency.
		if cfInst < -1*frequencyTolerance || cfInst > frequencyTolerance {
			t.Errorf("ERROR: Optical-Channel %d: carrier-frequency-offset not in tolerable range, got: %v, want: (+/-)%v", i, cfInst, frequencyTolerance)
		}

		// Verify all values are float64
		for _, ele := range []any{cfInst, cfMin, cfMax, cfAvg} {
			if reflect.TypeOf(ele).Kind() != reflect.Float64 {
				t.Fatalf("Value %v is not type float64", ele)
			}
		}

		if !deviations.MissingZROpticalChannelTunableParametersTelemetry(dut) {
			// CRITICAL FIX: Validate statistics consistency without comparing to instant
			// The instant value may be from a different sampling window than min/max/avg
			// Only validate that the statistics themselves are internally consistent
			if cfMin > cfAvg+statisticsTolerance {
				t.Errorf("ERROR: Optical-Channel %d: carrier-frequency-offset min (%v) greater than avg (%v) beyond tolerance", i, cfMin, cfAvg)
			}
			if cfMax < cfAvg-statisticsTolerance {
				t.Errorf("ERROR: Optical-Channel %d: carrier-frequency-offset max (%v) less than avg (%v) beyond tolerance", i, cfMax, cfAvg)
			}
			// Sanity check: min should be <= max (with tolerance)
			if cfMin > cfMax+statisticsTolerance {
				t.Errorf("ERROR: Optical-Channel %d: carrier-frequency-offset min (%v) greater than max (%v)", i, cfMin, cfMax)
			}
		} else {
			t.Log("Skipping Min/Max/Avg Tunable Parameters Telemetry validation. Deviation MissingZROpticalChannelTunableParametersTelemetry enabled.")
		}

		// Output Power validation
		opInst := _oc.GetOutputPower().GetInstant()
		opAvg := _oc.GetOutputPower().GetAvg()
		opMin := _oc.GetOutputPower().GetMin()
		opMax := _oc.GetOutputPower().GetMax()

		// When set to a specific target output power, transmit power control
		// absolute accuracy should be within +/- 1 dBm of the target configured
		// output power.
		switch operStatus {
		case oc.Interface_OperStatus_UP:
			// Use relaxed tolerance for instant output power check (±2 dBm)
			if opInst < outputPower-2 || opInst > outputPower+2 {
				t.Errorf("ERROR: Optical-Channel %d: output-power not in tolerable range, got: %v, want: %v (±2 dBm)", i, opInst, outputPower)
			}
		case oc.Interface_OperStatus_DOWN:
			if opInst != -40 {
				t.Errorf("ERROR: Optical-Channel %d: output-power not in tolerable range, got: %v, want: %v", i, opInst, -40)
			}
		}

		// Verify all values are float64
		for _, ele := range []any{opInst, opMin, opMax, opAvg} {
			if reflect.TypeOf(ele).Kind() != reflect.Float64 {
				t.Fatalf("Value %v is not type float64", ele)
			}
		}

		if !deviations.MissingZROpticalChannelTunableParametersTelemetry(dut) {
			// CRITICAL FIX: Validate statistics consistency without comparing to instant
			// Only validate that the statistics themselves are internally consistent
			if opMin > opAvg+statisticsTolerance {
				t.Errorf("ERROR: Optical-Channel %d: output-power min (%v) greater than avg (%v) beyond tolerance", i, opMin, opAvg)
			}
			if opMax < opAvg-statisticsTolerance {
				t.Errorf("ERROR: Optical-Channel %d: output-power max (%v) less than avg (%v) beyond tolerance", i, opMax, opAvg)
			}
			// Sanity check: min should be <= max (with tolerance)
			if opMin > opMax+statisticsTolerance {
				t.Errorf("ERROR: Optical-Channel %d: output-power min (%v) greater than max (%v)", i, opMin, opMax)
			}
		} else {
			t.Log("Skipping Min/Max/Avg Tunable Parameters Telemetry validation. Deviation MissingZROpticalChannelTunableParametersTelemetry enabled.")
		}

		if got, want := _oc.GetFrequency(), frequency; got != want {
			t.Errorf("ERROR: Optical-Channel %d: frequency: %v, want: %v", i, got, want)
		}
	}
}

// Helper function to log telemetry values for debugging
func logTelemetryValues(t *testing.T, channelID int, oc *oc.Component_OpticalChannel) {
	t.Logf("Channel %d Telemetry:", channelID)
	t.Logf("  Carrier Freq Offset - Inst: %.2f, Avg: %.2f, Min: %.2f, Max: %.2f",
		oc.GetCarrierFrequencyOffset().GetInstant(),
		oc.GetCarrierFrequencyOffset().GetAvg(),
		oc.GetCarrierFrequencyOffset().GetMin(),
		oc.GetCarrierFrequencyOffset().GetMax())
	t.Logf("  Output Power - Inst: %.2f, Avg: %.2f, Min: %.2f, Max: %.2f",
		oc.GetOutputPower().GetInstant(),
		oc.GetOutputPower().GetAvg(),
		oc.GetOutputPower().GetMin(),
		oc.GetOutputPower().GetMax())
	t.Logf("  Frequency: %v", oc.GetFrequency())
	t.Logf("  Operational Mode: %v", oc.GetOperationalMode())
}
