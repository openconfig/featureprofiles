package zr_cd_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	dp16QAM          = 1
	samplingInterval = 10 * time.Second
	minCDValue       = -200
	maxCDValue       = 2400
	inActiveCDValue  = 0.0
	timeout          = 10 * time.Minute
	flapInterval     = 30 * time.Second
)

type portState int

const (
	disabled portState = iota
	enabled
)

var (
	frequencies        = []uint64{191400000, 196100000} // 400ZR OIF wavelength range
	targetOutputPowers = []float64{-13, -9}             // 400ZR OIF Tx power range
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func verifyCDValue(t *testing.T, dut1 *ondatra.DUTDevice, pStream *samplestream.SampleStream[float64], sensorName string, status portState) float64 {
	CDSampleNexts := pStream.Nexts(2)
	CDSample := CDSampleNexts[1]
	t.Logf("CDSampleNexts %v", CDSampleNexts)
	if CDSample == nil {
		t.Fatalf("CD telemetry %s was not streamed in the most recent subscription interval", sensorName)
	}
	CDVal, ok := CDSample.Val()
	if !ok {
		t.Fatalf("CD %q telemetry is not present", CDSample)
	}
	// Check CD return value of correct type
	switch {
	case status == disabled:
		if CDVal != inActiveCDValue {
			t.Fatalf("The inactive CD is %v, expected %v", CDVal, inActiveCDValue)
		}
	case status == enabled:
		if CDVal < minCDValue || CDVal > maxCDValue {
			t.Fatalf("The variable CD is %v, expected range (%v, %v)", CDVal, minCDValue, maxCDValue)
		}
	default:
		t.Fatalf("Invalid status %v", status)
	}
	// Get current time
	now := time.Now()
	// Format the time string
	formattedTime := now.Format("2006-01-02 15:04:05")
	t.Logf("%s Device %v CD %s value at status %v: %v", formattedTime, dut1.Name(), sensorName, status, CDVal)

	return CDVal
}

// TODO: Avg and Instant value checks are not available. Need to align their sample streaming windows.
func verifyAllCDValues(t *testing.T, dut1 *ondatra.DUTDevice, p1StreamInstant, p1StreamMax, p1StreamMin, p1StreamAvg *samplestream.SampleStream[float64], status portState) {
	verifyCDValue(t, dut1, p1StreamInstant, "Instant", status)
	verifyCDValue(t, dut1, p1StreamMax, "Max", status)
	verifyCDValue(t, dut1, p1StreamMin, "Min", status)
	verifyCDValue(t, dut1, p1StreamAvg, "Avg", status)

	// if CDAvg >= CDMin && CDAvg <= CDMax {
	// 	t.Logf("The average is between the maximum and minimum values, Avg:%v Max:%v Min:%v", CDAvg, CDMax, CDMin)
	// } else {
	// 	t.Fatalf("The average is NOT between the maximum and minimum values, Avg:%v Max:%v Min:%v", CDAvg, CDMax, CDMin)
	// }

	// if CDInstant >= CDMin && CDInstant <= CDMax {
	// 	t.Logf("The instant is between the maximum and minimum values, Instant:%v Max:%v Min:%v", CDInstant, CDMax, CDMin)
	// } else {
	// 	t.Fatalf("The instant is NOT between the maximum and minimum values, Instant:%v Max:%v Min:%v", CDInstant, CDMax, CDMin)
	// }
}

func TestCDValue(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	cfgplugins.InterfaceConfig(t, dut, dp1)
	cfgplugins.InterfaceConfig(t, dut, dp2)

	tr1 := gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).Transceiver().State())
	tr2 := gnmi.Get(t, dut, gnmi.OC().Interface(dp2.Name()).Transceiver().State())
	och1 := gnmi.Get(t, dut, gnmi.OC().Component(tr1).Transceiver().Channel(0).AssociatedOpticalChannel().State())
	och2 := gnmi.Get(t, dut, gnmi.OC().Component(tr2).Transceiver().Channel(0).AssociatedOpticalChannel().State())
	component1 := gnmi.OC().Component(och1)

	for _, frequency := range frequencies {
		for _, targetOutputPower := range targetOutputPowers {
			cfgplugins.ConfigOpticalChannel(t, dut, och1, frequency, targetOutputPower, dp16QAM)
			cfgplugins.ConfigOpticalChannel(t, dut, och2, frequency, targetOutputPower, dp16QAM)

			// Wait for channels to be up.
			gnmi.Await(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
			gnmi.Await(t, dut, gnmi.OC().Interface(dp2.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)

			p1StreamInstant := samplestream.New(t, dut, component1.OpticalChannel().ChromaticDispersion().Instant().State(), samplingInterval)
			p1StreamMin := samplestream.New(t, dut, component1.OpticalChannel().ChromaticDispersion().Min().State(), samplingInterval)
			p1StreamMax := samplestream.New(t, dut, component1.OpticalChannel().ChromaticDispersion().Max().State(), samplingInterval)
			p1StreamAvg := samplestream.New(t, dut, component1.OpticalChannel().ChromaticDispersion().Avg().State(), samplingInterval)

			defer p1StreamInstant.Close()
			defer p1StreamMin.Close()
			defer p1StreamMax.Close()
			defer p1StreamAvg.Close()

			verifyAllCDValues(t, dut, p1StreamInstant, p1StreamMax, p1StreamMin, p1StreamAvg, enabled)

			// Disable interface.
			for _, p := range dut.Ports() {
				cfgplugins.ToggleInterface(t, dut, p.Name(), false)
			}
			// Wait for channels to be down.
			gnmi.Await(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_DOWN)
			gnmi.Await(t, dut, gnmi.OC().Interface(dp2.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_DOWN)
			t.Logf("Interfaces are down: %v, %v", dp1.Name(), dp2.Name())
			verifyAllCDValues(t, dut, p1StreamInstant, p1StreamMax, p1StreamMin, p1StreamAvg, enabled)

			time.Sleep(flapInterval)

			// Enable interface.
			for _, p := range dut.Ports() {
				cfgplugins.ToggleInterface(t, dut, p.Name(), true)
			}
			// Wait for channels to be up.
			gnmi.Await(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
			gnmi.Await(t, dut, gnmi.OC().Interface(dp2.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
			t.Logf("Interfaces are up: %v, %v", dp1.Name(), dp2.Name())
			verifyAllCDValues(t, dut, p1StreamInstant, p1StreamMax, p1StreamMin, p1StreamAvg, enabled)

		}
	}
}
