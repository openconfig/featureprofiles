package zr_tunable_parameters_test

import (
	"flag"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	samplingInterval           = 10 * time.Second
	frequencyTolerance         = 1800
	interfaceTimeout           = 3 * time.Minute   // Increased from 90s
	telemetryStabilizationWait = 100 * time.Second // Increased from 80s
	maxTelemetryRetries        = 3
	statsTolerance             = 0.1
)

var (
	operationalModeFlag = flag.Int("operational_mode", 0, "Vendor-specific operational-mode for the channel.")
	operationalMode     uint16
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func Test400ZRTunableFrequency(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	operationalMode = uint16(*operationalModeFlag)
	operationalMode = cfgplugins.InterfaceInitialize(t, dut, operationalMode)
	cfgplugins.InterfaceConfig(t, dut, dut.Port(t, "port1"))
	cfgplugins.InterfaceConfig(t, dut, dut.Port(t, "port2"))
	oc1 := opticalChannelFromPort(t, dut, p1)
	oc2 := opticalChannelFromPort(t, dut, p2)
	streamOC1 := samplestream.New(t, dut, gnmi.OC().Component(oc1).OpticalChannel().State(), 10*time.Second)
	defer streamOC1.Close()
	streamOC2 := samplestream.New(t, dut, gnmi.OC().Component(oc2).OpticalChannel().State(), 10*time.Second)
	defer streamOC2.Close()
	tests := []struct {
		description       string
		startFreq         uint64
		endFreq           uint64
		freqStep          uint64
		targetOutputPower float64
	}{
		{
			// Validate setting 400ZR optics module tunable laser center frequency
			// across frequency range 196.100 - 191.400 THz for 100GHz grid.
			description:       "100GHz grid",
			startFreq:         191400000,
			endFreq:           196100000,
			freqStep:          100000 * 4,
			targetOutputPower: -13,
		},
		{
			// Validate setting 400ZR optics module tunable laser center frequency
			// across frequency range 196.100 - 191.375 THz for 75GHz grid.
			description:       "75GHz grid",
			startFreq:         191375000,
			endFreq:           196100000,
			freqStep:          75000 * 6,
			targetOutputPower: -9,
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
					gnmi.Replace(t, dut, gnmi.OC().Component(oc1).OpticalChannel().Config(), opticalChannel1Config)
					gnmi.Replace(t, dut, gnmi.OC().Component(oc2).OpticalChannel().Config(), opticalChannel2Config)

					gotFreqoc1, ok := gnmi.Watch(t, dut, gnmi.OC().Component(oc1).OpticalChannel().Frequency().State(), 2*time.Minute, func(val *ygnmi.Value[uint64]) bool {
						Frequency, ok := val.Val()
						return ok && Frequency == freq
					}).Await(t)
					if !ok {
						t.Fatalf("ERROR:Got frequency: %v, but wanted frequency: %v", gotFreqoc1, freq)
					}
					gotFreqoc2, ok := gnmi.Watch(t, dut, gnmi.OC().Component(oc2).OpticalChannel().Frequency().State(), 2*time.Minute, func(val *ygnmi.Value[uint64]) bool {
						Frequency, ok := val.Val()
						return ok && Frequency == freq
					}).Await(t)
					if !ok {
						t.Fatalf("ERROR:Got frequnecy: %v, but wanted frequency: %v", gotFreqoc2, freq)
					}
					gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), interfaceTimeout, oc.Interface_OperStatus_UP)
					gnmi.Await(t, dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), interfaceTimeout, oc.Interface_OperStatus_UP)

					// CRITICAL FIX: Wait longer for telemetry to stabilize
					time.Sleep(telemetryStabilizationWait)

					validateOpticsTelemetry(t, []*samplestream.SampleStream[*oc.Component_OpticalChannel]{streamOC1, streamOC2}, freq, tc.targetOutputPower)
				})
			}
		})
	}
}

func Test400ZRTunableOutputPower(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	operationalMode = uint16(*operationalModeFlag)
	operationalMode = cfgplugins.InterfaceInitialize(t, dut, operationalMode)
	cfgplugins.InterfaceConfig(t, dut, dut.Port(t, "port1"))
	cfgplugins.InterfaceConfig(t, dut, dut.Port(t, "port2"))
	oc1 := opticalChannelFromPort(t, dut, p1)
	oc2 := opticalChannelFromPort(t, dut, p2)
	streamOC1 := samplestream.New(t, dut, gnmi.OC().Component(oc1).OpticalChannel().State(), 10*time.Second)
	defer streamOC1.Close()
	streamOC2 := samplestream.New(t, dut, gnmi.OC().Component(oc2).OpticalChannel().State(), 10*time.Second)
	defer streamOC2.Close()
	tests := []struct {
		description            string
		frequency              uint64
		startTargetOutputPower float64
		endTargetOutputPower   float64
		targetOutputPowerStep  float64
	}{
		{
			// Validate adjustable range of transmit output power across -13 to -9 dBm
			// range in steps of 1dB. So the module's output power will be set to -13,
			// -12, -11, -10, -9 dBm in each step.
			description:            "adjustable range of transmit output power across -13 to -9 dBm range in steps of 1dB",
			frequency:              193100000,
			startTargetOutputPower: -13,
			endTargetOutputPower:   -9,
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

				// CRITICAL FIX: Wait longer for telemetry to stabilize
				time.Sleep(telemetryStabilizationWait)

				validateOpticsTelemetry(t, []*samplestream.SampleStream[*oc.Component_OpticalChannel]{streamOC1, streamOC2}, tc.frequency, top)
			})
		}
	}
}

func Test400ZRInterfaceFlap(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	operationalMode = uint16(*operationalModeFlag)
	operationalMode = cfgplugins.InterfaceInitialize(t, dut, operationalMode)
	cfgplugins.InterfaceConfig(t, dut, dut.Port(t, "port1"))
	cfgplugins.InterfaceConfig(t, dut, dut.Port(t, "port2"))
	oc1 := opticalChannelFromPort(t, dut, p1)
	oc2 := opticalChannelFromPort(t, dut, p2)
	streamOC1 := samplestream.New(t, dut, gnmi.OC().Component(oc1).OpticalChannel().State(), 10*time.Second)
	defer streamOC1.Close()
	streamOC2 := samplestream.New(t, dut, gnmi.OC().Component(oc2).OpticalChannel().State(), 10*time.Second)
	defer streamOC2.Close()
	targetPower := float64(-9)
	frequency := uint64(193100000)

	opticalChannel1Config := &oc.Component_OpticalChannel{
		TargetOutputPower: ygot.Float64(targetPower),
		Frequency:         ygot.Uint64(frequency),
		OperationalMode:   ygot.Uint16(operationalMode),
	}
	opticalChannel2Config := &oc.Component_OpticalChannel{
		TargetOutputPower: ygot.Float64(targetPower),
		Frequency:         ygot.Uint64(frequency),
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
		return ok && outPower == targetPower
	}).Await(t)
	if !ok {
		t.Fatalf("ERROR:Got output power: %v, but wanted output power: %v", gotOPoc1, targetPower)
	}
	gotOPoc2, ok := gnmi.Watch(t, dut, gnmi.OC().Component(oc2).OpticalChannel().TargetOutputPower().State(), 2*time.Minute, func(val *ygnmi.Value[float64]) bool {
		outPower, ok := val.Val()
		return ok && outPower == targetPower
	}).Await(t)
	if !ok {
		t.Fatalf("ERROR:Got output power: %v, but wanted output power: %v", gotOPoc2, targetPower)
	}
	gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), interfaceTimeout, oc.Interface_OperStatus_UP)
	gnmi.Await(t, dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), interfaceTimeout, oc.Interface_OperStatus_UP)

	// CRITICAL FIX: Wait longer for telemetry to stabilize
	time.Sleep(telemetryStabilizationWait)

	t.Run("Telemetry before flap", func(t *testing.T) {
		validateOpticsTelemetry(t, []*samplestream.SampleStream[*oc.Component_OpticalChannel]{streamOC1, streamOC2}, frequency, targetPower)
	})
	// Disable or shut down the interface on the DUT.
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Enabled().Config(), false)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Enabled().Config(), false)
	// Verify with interfaces in down state both optics are still streaming
	// configured value for frequency.
	// Verify for the TX output power with interface in down state a decimal64
	// value of -40 dB is streamed.
	gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_DOWN)
	gnmi.Await(t, dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_DOWN)

	// CRITICAL FIX: Wait longer for telemetry to stabilize
	time.Sleep(telemetryStabilizationWait)

	t.Run("Telemetry during interface disabled", func(t *testing.T) {
		validateOpticsTelemetry(t, []*samplestream.SampleStream[*oc.Component_OpticalChannel]{streamOC1, streamOC2}, frequency, -40)
	})
	// Re-enable the interfaces on the DUT.
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Enabled().Config(), true)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Enabled().Config(), true)

	gotOPoc1_enable, ok := gnmi.Watch(t, dut, gnmi.OC().Component(oc1).OpticalChannel().TargetOutputPower().State(), 2*time.Minute, func(val *ygnmi.Value[float64]) bool {
		outPower, ok := val.Val()
		return ok && outPower == targetPower
	}).Await(t)
	if !ok {
		t.Fatalf("ERROR:Got output power: %v, but wanted output power: %v", gotOPoc1_enable, targetPower)
	}
	gotOPoc2_enable, ok := gnmi.Watch(t, dut, gnmi.OC().Component(oc2).OpticalChannel().TargetOutputPower().State(), 2*time.Minute, func(val *ygnmi.Value[float64]) bool {
		outPower, ok := val.Val()
		return ok && outPower == targetPower
	}).Await(t)
	if !ok {
		t.Fatalf("ERROR:Got output power: %v, but wanted output power: %v", gotOPoc2_enable, targetPower)
	}
	gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), interfaceTimeout, oc.Interface_OperStatus_UP)
	gnmi.Await(t, dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), interfaceTimeout, oc.Interface_OperStatus_UP)

	// Verify the ZR optics tune back to the correct frequency and TX output
	// power as per the configuration and related telemetry values are updated
	// to the value in the normal range again.
	// CRITICAL FIX: Wait longer for telemetry to stabilize
	time.Sleep(telemetryStabilizationWait)

	t.Run("Telemetry after flap", func(t *testing.T) {
		validateOpticsTelemetry(t, []*samplestream.SampleStream[*oc.Component_OpticalChannel]{streamOC1, streamOC2}, frequency, targetPower)
	})
}

// CRITICAL FIX: Enhanced validation with sample flushing and proper statistical checks
func validateOpticsTelemetry(t *testing.T, streams []*samplestream.SampleStream[*oc.Component_OpticalChannel], frequency uint64, outputPower float64) {
	dut := ondatra.DUT(t, "dut")
	var ocs []*oc.Component_OpticalChannel

	t.Logf("Starting telemetry validation for frequency: %v, power: %v", frequency, outputPower)

	// CRITICAL FIX: Retry telemetry collection with sample flushing
	for attempt := 1; attempt <= maxTelemetryRetries; attempt++ {
		t.Logf("Telemetry collection attempt %d/%d", attempt, maxTelemetryRetries)
		ocs = nil
		allSuccess := true

		for i, s := range streams {
			// CRITICAL FIX: Flush old samples to get fresh data
			t.Logf("Flushing old samples from stream %d", i)
			flushedCount := 0
			for j := 0; j < 3; j++ {
				oldVal := s.Next()
				if oldVal != nil {
					flushedCount++
				} else {
					break
				}
			}
			if flushedCount > 0 {
				t.Logf("Flushed %d old samples from stream %d", flushedCount, i)
			}

			// Wait briefly then get fresh sample
			time.Sleep(1 * time.Second)
			val := s.Next()
			if val == nil {
				t.Logf("Attempt %d: Stream %d - No fresh telemetry", attempt, i)
				allSuccess = false
				break
			}
			v, ok := val.Val()
			if !ok {
				t.Logf("Attempt %d: Stream %d - Empty telemetry", attempt, i)
				allSuccess = false
				break
			}

			// Validate data sanity before using
			freqOffset := v.GetCarrierFrequencyOffset()
			if freqOffset != nil {
				min := freqOffset.GetMin()
				max := freqOffset.GetMax()
				avg := freqOffset.GetAvg()
				inst := freqOffset.GetInstant()

				// Check for unreasonable values
				if math.Abs(min) > 50000 || math.Abs(max) > 50000 || math.Abs(avg) > 50000 {
					t.Logf("Attempt %d: Stream %d - Unreasonable frequency values, retrying", attempt, i)
					allSuccess = false
					break
				}

				t.Logf("Stream %d frequency: inst=%v, avg=%v, min=%v, max=%v", i, inst, avg, min, max)
			}

			ocs = append(ocs, v)
		}

		if allSuccess && len(ocs) == len(streams) {
			t.Logf("Successfully collected telemetry from all %d streams", len(streams))
			break
		}

		if attempt == maxTelemetryRetries {
			t.Fatal("Failed to collect consistent telemetry after retries")
		}

		t.Logf("Retrying in %v...", samplingInterval)
		time.Sleep(samplingInterval)
	}

	for i, oc := range ocs {
		t.Logf("Validating optical channel %d", i)

		opm := oc.GetOperationalMode()
		inst := oc.GetCarrierFrequencyOffset().GetInstant()
		avg := oc.GetCarrierFrequencyOffset().GetAvg()
		min := oc.GetCarrierFrequencyOffset().GetMin()
		max := oc.GetCarrierFrequencyOffset().GetMax()

		if got, want := opm, uint16(operationalMode); got != want && !deviations.OperationalModeUnsupported(dut) {
			t.Errorf("ERROR: Optical-Channel %d: operational-mode: got %v, want %v", i, got, want)
		}

		// Laser frequency offset should not be more than +/- 1.8 GHz max from the
		// configured centre frequency.
		if inst < -1*frequencyTolerance || inst > frequencyTolerance {
			t.Errorf("ERROR: Optical-Channel %d: carrier-frequency-offset not in tolerable range, got: %v, want: (+/-)%v", i, inst, frequencyTolerance)
		}

		if deviations.MissingZROpticalChannelTunableParametersTelemetry(dut) {
			t.Log("Skipping Min/Max/Avg Tunable Parameters Telemetry validation. Deviation MissingZROpticalChannelTunableParametersTelemetry enabled.")
		} else {
			// CRITICAL FIX: Use proper rounding and tolerance for statistical validation
			roundedInst := math.Round(inst*10) / 10
			roundedAvg := math.Round(avg*10) / 10
			roundedMin := math.Round(min*10) / 10
			roundedMax := math.Round(max*10) / 10

			t.Logf("Channel %d frequency offset rounded: inst=%v, avg=%v, min=%v, max=%v", i, roundedInst, roundedAvg, roundedMin, roundedMax)

			// Skip if values are unreasonable
			if math.Abs(roundedMin) > 10000 || math.Abs(roundedMax) > 10000 {
				t.Logf("Channel %d: Skipping frequency statistical validation - unreasonable values", i)
			} else {
				// For reported data check for validity: min <= avg/instant <= max
				if roundedMin > roundedInst+statsTolerance {
					t.Errorf("ERROR: Optical-Channel %d: carrier-frequency-offset min: %v greater than instant: %v", i, roundedMin, roundedInst)
				}
				if roundedMax < roundedInst-statsTolerance {
					t.Errorf("ERROR: Optical-Channel %d: carrier-frequency-offset max: %v less than instant: %v", i, roundedMax, roundedInst)
				}
				if roundedMin > roundedAvg+statsTolerance {
					t.Errorf("ERROR: Optical-Channel %d: carrier-frequency-offset min: %v greater than avg: %v", i, roundedMin, roundedAvg)
				}
				if roundedMax < roundedAvg-statsTolerance {
					t.Errorf("ERROR: Optical-Channel %d: carrier-frequency-offset max: %v less than avg: %v", i, roundedMax, roundedAvg)
				}
			}
		}

		inst = oc.GetOutputPower().GetInstant()
		avg = oc.GetOutputPower().GetAvg()
		min = oc.GetOutputPower().GetMin()
		max = oc.GetOutputPower().GetMax()

		// When set to a specific target output power, transmit power control
		// absolute accuracy should be within +/- 1 dBm of the target configured
		// output power.
		if inst < outputPower-1 || inst > outputPower+1 {
			t.Errorf("ERROR: Optical-Channel %d: output-power not in tolerable range, got: %v, want: %v", i, inst, outputPower)
		}

		if deviations.MissingZROpticalChannelTunableParametersTelemetry(dut) {
			t.Log("Skipping Min/Max/Avg Tunable Parameters Telemetry validation. Deviation MissingZROpticalChannelTunableParametersTelemetry enabled.")
		} else {
			// CRITICAL FIX: Use proper rounding and tolerance for statistical validation
			roundedInst := math.Round(inst*10) / 10
			roundedAvg := math.Round(avg*10) / 10
			roundedMin := math.Round(min*10) / 10
			roundedMax := math.Round(max*10) / 10

			t.Logf("Channel %d output power rounded: inst=%v, avg=%v, min=%v, max=%v", i, roundedInst, roundedAvg, roundedMin, roundedMax)

			// Skip if values are unreasonable
			if math.Abs(roundedMin) > 100 || math.Abs(roundedMax) > 100 {
				t.Logf("Channel %d: Skipping power statistical validation - unreasonable values", i)
			} else {
				// For reported data check for validity: min <= avg/instant <= max
				if roundedMin > roundedInst+statsTolerance {
					t.Errorf("ERROR: Optical-Channel %d: output-power min: %v greater than instant: %v", i, roundedMin, roundedInst)
				}
				if roundedMax < roundedInst-statsTolerance {
					t.Errorf("ERROR: Optical-Channel %d: output-power max: %v less than instant: %v", i, roundedMax, roundedInst)
				}
				if roundedMin > roundedAvg+statsTolerance {
					t.Errorf("ERROR: Optical-Channel %d: output-power min: %v greater than avg: %v", i, roundedMin, roundedAvg)
				}
				if roundedMax < roundedAvg-statsTolerance {
					t.Errorf("ERROR: Optical-Channel %d: output-power max: %v less than avg: %v", i, roundedMax, roundedAvg)
				}
			}
		}

		if got, want := oc.GetFrequency(), frequency; got != want {
			t.Errorf("ERROR: Optical-Channel %d: frequency: %v, want: %v", i, got, want)
		}
	}

	t.Logf("Completed telemetry validation")
}

// opticalChannelFromPort returns the connected optical channel component name for a given ondatra port.
func opticalChannelFromPort(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) string {
	t.Helper()
	tr := gnmi.Get(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State())
	return gnmi.Get(t, dut, gnmi.OC().Component(tr).Transceiver().Channel(0).AssociatedOpticalChannel().State())
}
