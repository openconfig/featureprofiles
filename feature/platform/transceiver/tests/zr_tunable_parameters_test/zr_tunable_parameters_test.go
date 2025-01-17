package zr_tunable_parameters_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	samplingInterval   = 10 * time.Second
	frequencyTolerance = 1800
	dp16QAM            = 1
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
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
func Test400ZRTunableFrequency(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
	oc1 := opticalChannelFromPort(t, dut, p1)
	oc2 := opticalChannelFromPort(t, dut, p2)
	streamOC1 := samplestream.New(t, dut, gnmi.OC().Component(oc1).State(), 10*time.Second)
	defer streamOC1.Close()
	streamOC2 := samplestream.New(t, dut, gnmi.OC().Component(oc2).State(), 10*time.Second)
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
					gnmi.Replace(t, dut, gnmi.OC().Component(oc1).OpticalChannel().Config(), &oc.Component_OpticalChannel{
						TargetOutputPower: ygot.Float64(tc.targetOutputPower),
						Frequency:         ygot.Uint64(freq),
						OperationalMode:   ygot.Uint16(dp16QAM),
					})
					gnmi.Replace(t, dut, gnmi.OC().Component(oc2).OpticalChannel().Config(), &oc.Component_OpticalChannel{
						TargetOutputPower: ygot.Float64(tc.targetOutputPower),
						Frequency:         ygot.Uint64(freq),
						OperationalMode:   ygot.Uint16(dp16QAM),
					})
					gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)
					gnmi.Await(t, dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)
					validateOpticsTelemetry(t, []*samplestream.SampleStream[*oc.Component]{streamOC1, streamOC2}, freq, tc.targetOutputPower)
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
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
	oc1 := opticalChannelFromPort(t, dut, p1)
	oc2 := opticalChannelFromPort(t, dut, p2)
	streamOC1 := samplestream.New(t, dut, gnmi.OC().Component(oc1).State(), 10*time.Second)
	defer streamOC1.Close()
	streamOC2 := samplestream.New(t, dut, gnmi.OC().Component(oc2).State(), 10*time.Second)
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
			// range in steps of 1dB. So the module’s output power will be set to -13,
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
				gnmi.Replace(t, dut, gnmi.OC().Component(oc1).OpticalChannel().Config(), &oc.Component_OpticalChannel{
					TargetOutputPower: ygot.Float64(top),
					Frequency:         ygot.Uint64(tc.frequency),
					OperationalMode:   ygot.Uint16(dp16QAM),
				})
				gnmi.Replace(t, dut, gnmi.OC().Component(oc2).OpticalChannel().Config(), &oc.Component_OpticalChannel{
					TargetOutputPower: ygot.Float64(top),
					Frequency:         ygot.Uint64(tc.frequency),
					OperationalMode:   ygot.Uint16(dp16QAM),
				})
				gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)
				gnmi.Await(t, dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)
				validateOpticsTelemetry(t, []*samplestream.SampleStream[*oc.Component]{streamOC1, streamOC2}, tc.frequency, top)
			})
		}
	}
}
func Test400ZRInterfaceFlap(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
	oc1 := opticalChannelFromPort(t, dut, p1)
	oc2 := opticalChannelFromPort(t, dut, p2)
	streamOC1 := samplestream.New(t, dut, gnmi.OC().Component(oc1).State(), 10*time.Second)
	defer streamOC1.Close()
	streamOC2 := samplestream.New(t, dut, gnmi.OC().Component(oc2).State(), 10*time.Second)
	defer streamOC2.Close()
	targetPower := float64(-9)
	frequency := uint64(193100000)
	gnmi.Replace(t, dut, gnmi.OC().Component(oc1).OpticalChannel().Config(), &oc.Component_OpticalChannel{
		TargetOutputPower: ygot.Float64(targetPower),
		Frequency:         ygot.Uint64(frequency),
		OperationalMode:   ygot.Uint16(dp16QAM),
	})
	gnmi.Replace(t, dut, gnmi.OC().Component(oc2).OpticalChannel().Config(), &oc.Component_OpticalChannel{
		TargetOutputPower: ygot.Float64(targetPower),
		Frequency:         ygot.Uint64(frequency),
		OperationalMode:   ygot.Uint16(dp16QAM),
	})
	gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)
	gnmi.Await(t, dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)
	t.Run("Telemetry before flap", func(t *testing.T) {
		validateOpticsTelemetry(t, []*samplestream.SampleStream[*oc.Component]{streamOC1, streamOC2}, frequency, targetPower)
	})
	// Disable or shut down the interface on the DUT.
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Enabled().Config(), false)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Enabled().Config(), false)
	// Verify with interfaces in down state both optics are still streaming
	// configured value for frequency.
	// Verify for the TX output power with interface in down state a decimal64
	// value of -40 dB is streamed.
	t.Run("Telemetry during interface disabled", func(t *testing.T) {
		validateOpticsTelemetry(t, []*samplestream.SampleStream[*oc.Component]{streamOC1, streamOC2}, frequency, -40)
	})
	// Re-enable the interfaces on the DUT.
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Enabled().Config(), true)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Enabled().Config(), true)
	gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)
	gnmi.Await(t, dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)
	// Verify the ZR optics tune back to the correct frequency and TX output
	// power as per the configuration and related telemetry values are updated
	// to the value in the normal range again.
	t.Run("Telemetry after flap", func(t *testing.T) {
		validateOpticsTelemetry(t, []*samplestream.SampleStream[*oc.Component]{streamOC1, streamOC2}, frequency, targetPower)
	})
}
func validateOpticsTelemetry(t *testing.T, streams []*samplestream.SampleStream[*oc.Component], frequency uint64, outputPower float64) {
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

	for _, oc := range ocs {
		opm := oc.GetOperationalMode()
		inst := oc.GetCarrierFrequencyOffset().GetInstant()
		avg := oc.GetCarrierFrequencyOffset().GetAvg()
		min := oc.GetCarrierFrequencyOffset().GetMin()
		max := oc.GetCarrierFrequencyOffset().GetMax()
		if got, want := opm, uint16(dp16QAM); got != want {
			t.Errorf("Optical-Channel: operational-mode: got %v, want %v", got, want)
		}
		// Laser frequency offset should not be more than +/- 1.8 GHz max from the
		// configured centre frequency.
		if inst < -1*frequencyTolerance || inst > frequencyTolerance {
			t.Errorf("Optical-Channel: carrier-frequency-offset not in tolerable range, got: %v, want: (+/-)%v", inst, frequencyTolerance)
		}
		if deviations.MissingZROpticalChannelTunableParametersTelemetry(dut) {
			t.Log("Skipping Min/Max/Avg Tunable Parameters Telemetry validation. Deviation MissingZROpticalChannelTunableParametersTelemetry enabled.")
		} else {
			// For reported data check for validity: min <= avg/instant <= max
			if min > inst {
				t.Errorf("Optical-Channel: carrier-frequency-offset min: %v greater than carrier-frequency-offset instant: %v", min, inst)
			}
			if max < inst {
				t.Errorf("Optical-Channel: carrier-frequency-offset max: %v less than carrier-frequency-offset instant: %v", max, inst)
			}
			if min > avg {
				t.Errorf("Optical-Channel: carrier-frequency-offset min: %v greater than carrier-frequency-offset avg: %v", min, avg)
			}
			if max < avg {
				t.Errorf("Optical-Channel: carrier-frequency-offset max: %v less than carrier-frequency-offset avg: %v", max, avg)
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
			t.Errorf("Optical-Channel: output-power not in tolerable range, got: %v, want: %v", inst, outputPower)
		}
		if deviations.MissingZROpticalChannelTunableParametersTelemetry(dut) {
			t.Log("Skipping Min/Max/Avg Tunable Parameters Telemetry validation. Deviation MissingZROpticalChannelTunableParametersTelemetry enabled.")
		} else {
			// For reported data check for validity: min <= avg/instant <= max
			if min > inst {
				t.Errorf("Optical-Channel: output-power min: %v greater than output-power instant: %v", min, inst)
			}
			if max < inst {
				t.Errorf("Optical-Channel: output-power max: %v less than output-power instant: %v", max, inst)
			}
			if min > avg {
				t.Errorf("Optical-Channel: output-power min: %v greater than output-power avg: %v", min, avg)
			}
			if max < avg {
				t.Errorf("Optical-Channel: output-power max: %v less than output-power avg: %v", max, avg)
			}
		}
		if got, want := oc.GetFrequency(), frequency; got != want {
			t.Errorf("Optical-Channel: frequency: %v, want: %v", got, want)
		}
	}
}

// opticalChannelFromPort returns the connected optical channel component name for a given ondatra port.
func opticalChannelFromPort(t *testing.T, dut *ondatra.DUTDevice, p *ondatra.Port) string {
	t.Helper()
	tr := gnmi.Get(t, dut, gnmi.OC().Interface(p.Name()).Transceiver().State())
	return gnmi.Get(t, dut, gnmi.OC().Component(tr).Transceiver().Channel(0).AssociatedOpticalChannel().State())
}
