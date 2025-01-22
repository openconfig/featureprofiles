package zrp_cd_test

import (
	"flag"
	"reflect"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	samplingInterval = 10 * time.Second
	minCDValue       = -200
	maxCDValue       = 2400
	inActiveCDValue  = 0.0
	timeout          = 10 * time.Minute
	flapInterval     = 30 * time.Second
)

var (
	frequencies         = []uint64{191400000, 196100000}
	targetOutputPowers  = []float64{-7, 0}
	operationalModeFlag = flag.Int("operational_mode", 5, "vendor-specific operational-mode for the channel")
	operationalMode     uint16
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func verifyCDValue(t *testing.T, dut1 *ondatra.DUTDevice, pStream *samplestream.SampleStream[float64], sensorName string, operStatus oc.E_Interface_OperStatus) float64 {
	cdSampleNexts := pStream.Nexts(2)
	cdSample := cdSampleNexts[1]
	t.Logf("CDSampleNexts %v", cdSampleNexts)
	if cdSample == nil {
		t.Fatalf("CD telemetry %s was not streamed in the most recent subscription interval", sensorName)
	}
	cdVal, ok := cdSample.Val()
	if !ok {
		t.Fatalf("CD %q telemetry is not present", cdSample)
	}
	if reflect.TypeOf(cdVal).Kind() != reflect.Float64 {
		t.Fatalf("CD value is not type float64")
	}
	// Check CD return value of correct type
	switch operStatus {
	case oc.Interface_OperStatus_DOWN:
		if cdVal != inActiveCDValue {
			t.Fatalf("The inactive CD is %v, expected %v", cdVal, inActiveCDValue)
		}
	case oc.Interface_OperStatus_UP:
		if cdVal < minCDValue || cdVal > maxCDValue {
			t.Fatalf("The variable CD is %v, expected range (%v, %v)", cdVal, minCDValue, maxCDValue)
		}
	default:
		t.Fatalf("Invalid status %v", operStatus)
	}
	// Get current time
	now := time.Now()
	// Format the time string
	formattedTime := now.Format("2006-01-02 15:04:05")
	t.Logf("%s Device %v CD %s value at status %v: %v", formattedTime, dut1.Name(), sensorName, operStatus, cdVal)

	return cdVal
}

// TODO: Avg and Instant value checks are not available. Need to align their sample streaming windows.
func verifyAllCDValues(t *testing.T, dut1 *ondatra.DUTDevice, p1StreamInstant, p1StreamMax, p1StreamMin, p1StreamAvg *samplestream.SampleStream[float64], operStatus oc.E_Interface_OperStatus) {
	tests := []struct {
		desc       string
		stream     *samplestream.SampleStream[float64]
		streamType string
		operStatus oc.E_Interface_OperStatus
	}{
		{
			desc:       "Instant",
			stream:     p1StreamInstant,
			streamType: "Instant",
			operStatus: operStatus,
		},
		{
			desc:       "Max",
			stream:     p1StreamMax,
			streamType: "Max",
			operStatus: operStatus,
		},
		{
			desc:       "Min",
			stream:     p1StreamMin,
			streamType: "Min",
			operStatus: operStatus,
		},
		{
			desc:       "Avg",
			stream:     p1StreamAvg,
			streamType: "Avg",
			operStatus: operStatus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			verifyCDValue(t, dut1, tt.stream, tt.streamType, tt.operStatus)
		})
	}
}

func TestCDValue(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	if operationalModeFlag != nil {
		operationalMode = uint16(*operationalModeFlag)
	} else {
		t.Fatalf("Please specify the vendor-specific operational-mode flag")
	}
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")

	och1 := components.OpticalChannelComponentFromPort(t, dut, dp1)
	och2 := components.OpticalChannelComponentFromPort(t, dut, dp2)

	component1 := gnmi.OC().Component(och1)
	for _, frequency := range frequencies {
		for _, targetOutputPower := range targetOutputPowers {
			cfgplugins.ConfigOpticalChannel(t, dut, och1, frequency, targetOutputPower, operationalMode)
			cfgplugins.ConfigOpticalChannel(t, dut, och2, frequency, targetOutputPower, operationalMode)

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

			verifyAllCDValues(t, dut, p1StreamInstant, p1StreamMax, p1StreamMin, p1StreamAvg, oc.Interface_OperStatus_UP)

			// Disable interface.
			for _, p := range dut.Ports() {
				cfgplugins.ToggleInterface(t, dut, p.Name(), false)
			}
			// Wait for channels to be down.
			gnmi.Await(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_DOWN)
			gnmi.Await(t, dut, gnmi.OC().Interface(dp2.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_DOWN)
			t.Logf("Interfaces are down: %v, %v", dp1.Name(), dp2.Name())
			verifyAllCDValues(t, dut, p1StreamInstant, p1StreamMax, p1StreamMin, p1StreamAvg, oc.Interface_OperStatus_DOWN)

			time.Sleep(flapInterval)

			// Enable interface.
			for _, p := range dut.Ports() {
				cfgplugins.ToggleInterface(t, dut, p.Name(), true)
			}
			// Wait for channels to be up.
			gnmi.Await(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
			gnmi.Await(t, dut, gnmi.OC().Interface(dp2.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
			t.Logf("Interfaces are up: %v, %v", dp1.Name(), dp2.Name())
			verifyAllCDValues(t, dut, p1StreamInstant, p1StreamMax, p1StreamMin, p1StreamAvg, oc.Interface_OperStatus_UP)

		}
	}
}
