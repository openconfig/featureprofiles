package zr_cd_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	dp16QAM          = 1
	samplingInterval = 10 * time.Second
	minCDValue       = 0
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
	frequencies        = []uint64{191400000, 196100000}
	targetOutputPowers = []float64{-6, -10}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func interfaceConfig(t *testing.T, dut1 *ondatra.DUTDevice, dp *ondatra.Port, frequency uint64, targetOutputPower float64) {
	d := &oc.Root{}
	i := d.GetOrCreateInterface(dp.Name())
	i.Enabled = ygot.Bool(true)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	gnmi.Replace(t, dut1, gnmi.OC().Interface(dp.Name()).Config(), i)
	OCcomponent := components.OpticalChannelComponentFromPort(t, dut1, dp)
	gnmi.Replace(t, dut1, gnmi.OC().Component(OCcomponent).OpticalChannel().Config(), &oc.Component_OpticalChannel{
		TargetOutputPower: ygot.Float64(targetOutputPower),
		Frequency:         ygot.Uint64(frequency),
	})
}

func verifyCDValue(t *testing.T, dut1 *ondatra.DUTDevice, pStream *samplestream.SampleStream[float64], sensorName string, status portState) float64 {
	CDSample := pStream.Next()
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
		if CDVal < minCDValue && CDVal > maxCDValue {
			t.Fatalf("The variable CD is %v, expected range (%v, %v)", CDVal, minCDValue, maxCDValue)
		}
	default:
		t.Fatalf("Invalid status %v", status)
	}
	t.Logf("Device %v CD %s value : %v", dut1.Name(), sensorName, CDVal)
	return CDVal
}

func verifyAllCDValues(t *testing.T, dut1 *ondatra.DUTDevice, p1StreamInstant, p1StreamMax, p1StreamMin, p1StreamAvg *samplestream.SampleStream[float64], status portState) {
	CDInstant := verifyCDValue(t, dut1, p1StreamInstant, "Instant", status)
	CDMax := verifyCDValue(t, dut1, p1StreamMax, "Max", status)
	CDMin := verifyCDValue(t, dut1, p1StreamMin, "Min", status)
	CDAvg := verifyCDValue(t, dut1, p1StreamAvg, "Avg", status)

	if CDAvg >= CDMin && CDAvg <= CDMax {
		t.Logf("The average is between the maximum and minimum values, Avg:%v Max:%v Min:%v", CDAvg, CDMax, CDMin)
	} else {
		t.Fatalf("The average is NOT between the maximum and minimum values, Avg:%v Max:%v Min:%v", CDAvg, CDMax, CDMin)
	}

	if CDInstant >= CDMin && CDInstant <= CDMax {
		t.Logf("The instant is between the maximum and minimum values, Instant:%v Max:%v Min:%v", CDInstant, CDMax, CDMin)
	} else {
		t.Fatalf("The instant is NOT between the maximum and minimum values, Instant:%v Max:%v Min:%v", CDInstant, CDMax, CDMin)
	}

}

func TestCDValue(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut")
	dp1 := dut1.Port(t, "port1")
	dp2 := dut1.Port(t, "port2")
	fptest.ConfigureDefaultNetworkInstance(t, dut1)

	// Derive transceiver names from ports.
	tr1 := gnmi.Get(t, dut1, gnmi.OC().Interface(dp1.Name()).Transceiver().State())
	tr2 := gnmi.Get(t, dut1, gnmi.OC().Interface(dp2.Name()).Transceiver().State())
	component1 := gnmi.OC().Component(tr1)

	for _, frequency := range frequencies {
		for _, targetOutputPower := range targetOutputPowers {
			interfaceConfig(t, dut1, dp1, frequency, targetOutputPower)
			interfaceConfig(t, dut1, dp2, frequency, targetOutputPower)
			// Wait for channels to be up.
			gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
			gnmi.Await(t, dut1, gnmi.OC().Interface(dp2.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)

			p1StreamInstant := samplestream.New(t, dut1, component1.OpticalChannel().ChromaticDispersion().Instant().State(), samplingInterval)
			p1StreamAvg := samplestream.New(t, dut1, component1.OpticalChannel().ChromaticDispersion().Avg().State(), samplingInterval)
			p1StreamMin := samplestream.New(t, dut1, component1.OpticalChannel().ChromaticDispersion().Min().State(), samplingInterval)
			p1StreamMax := samplestream.New(t, dut1, component1.OpticalChannel().ChromaticDispersion().Max().State(), samplingInterval)

			verifyAllCDValues(t, dut1, p1StreamInstant, p1StreamMax, p1StreamMin, p1StreamAvg, enabled)

			// Disable or shut down the interface on the DUT.
			gnmi.Replace(t, dut1, gnmi.OC().Interface(dp1.Name()).Enabled().Config(), false)
			gnmi.Replace(t, dut1, gnmi.OC().Interface(dp2.Name()).Enabled().Config(), false)
			// Wait for channels to be down.
			gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_DOWN)
			gnmi.Await(t, dut1, gnmi.OC().Interface(dp2.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_DOWN)

			verifyAllCDValues(t, dut1, p1StreamInstant, p1StreamMax, p1StreamMin, p1StreamAvg, disabled)
			time.Sleep(flapInterval)

			// Re-enable interfaces.
			gnmi.Replace(t, dut1, gnmi.OC().Component(tr1).Transceiver().Enabled().Config(), true)
			gnmi.Replace(t, dut1, gnmi.OC().Component(tr2).Transceiver().Enabled().Config(), true)
			// Wait for channels to be up.
			gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
			gnmi.Await(t, dut1, gnmi.OC().Interface(dp2.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)

			verifyAllCDValues(t, dut1, p1StreamInstant, p1StreamMax, p1StreamMin, p1StreamAvg, enabled)

			p1StreamMin.Close()
			p1StreamMax.Close()
			p1StreamAvg.Close()
			p1StreamInstant.Close()
		}
	}
}
