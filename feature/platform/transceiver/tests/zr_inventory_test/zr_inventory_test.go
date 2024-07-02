package zr_inventory_test

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
	timeout          = 5 * time.Minute
	waitInterval     = 30 * time.Second
)

const (
	frequency         = 193100000
	targetOutputPower = -10
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configInterface(t *testing.T, dut1 *ondatra.DUTDevice, dp *ondatra.Port, frequency uint64, targetOutputPower float64) {
	d := &oc.Root{}
	i := d.GetOrCreateInterface(dp.Name())
	i.Enabled = ygot.Bool(true)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	gnmi.Replace(t, dut1, gnmi.OC().Interface(dp.Name()).Config(), i)
	c := components.OpticalChannelComponentFromPort(t, dut1, dp)
	gnmi.Replace(t, dut1, gnmi.OC().Component(c).OpticalChannel().Config(), &oc.Component_OpticalChannel{
		TargetOutputPower: ygot.Float64(targetOutputPower),
		Frequency:         ygot.Uint64(frequency),
	})
}

func verifyAllInventoryValues(t *testing.T, pStreamsStr []*samplestream.SampleStream[string], pStreamsUnion []*samplestream.SampleStream[oc.Component_Type_Union]) {
	for _, stream := range pStreamsStr {
		inventoryStr := stream.Next()
		if inventoryStr == nil {
			t.Fatalf("Inventory telemetry %v was not streamed in the most recent subscription interval", stream)
		}
		inventoryVal, ok := inventoryStr.Val()
		if !ok {
			t.Fatalf("Inventory telemetry %q is not present or valid, expected <string>", inventoryStr)
		} else {
			t.Logf("Inventory telemetry %q is valid: %q", inventoryStr, inventoryVal)
		}
	}

	for _, stream := range pStreamsUnion {
		inventoryUnion := stream.Next()
		if inventoryUnion == nil {
			t.Fatalf("Inventory telemetry %v was not streamed in the most recent subscription interval", stream)
		}
		inventoryVal, ok := inventoryUnion.Val()
		if !ok {
			t.Fatalf("Inventory telemetry %q is not present or valid, expected <union>", inventoryUnion)
		} else {
			t.Logf("Inventory telemetry %q is valid: %q", inventoryUnion, inventoryVal)
		}

	}
}

func TestInventory(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut")
	dp1 := dut1.Port(t, "port1")
	dp2 := dut1.Port(t, "port2")
	fptest.ConfigureDefaultNetworkInstance(t, dut1)

	// Derive transceiver names from ports.
	tr1 := gnmi.Get(t, dut1, gnmi.OC().Interface(dp1.Name()).Transceiver().State())
	tr2 := gnmi.Get(t, dut1, gnmi.OC().Interface(dp2.Name()).Transceiver().State())

	if (dp1.PMD() != ondatra.PMD400GBASEZR) || (dp2.PMD() != ondatra.PMD400GBASEZR) {
		t.Fatalf("Transceivers types (%v, %v): (%v, %v) are not 400ZR, expected %v", tr1, tr2, dp1.PMD(), dp2.PMD(), ondatra.PMD400GBASEZR)
	}
	component1 := gnmi.OC().Component(tr1)

	configInterface(t, dut1, dp1, frequency, targetOutputPower)
	configInterface(t, dut1, dp2, frequency, targetOutputPower)
	// Wait for channels to be up.
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp2.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)

	var p1StreamsStr []*samplestream.SampleStream[string]
	var p1StreamsUnion []*samplestream.SampleStream[oc.Component_Type_Union]
	p1StreamsStr = append(p1StreamsStr,
		samplestream.New(t, dut1, component1.SerialNo().State(), samplingInterval),
		samplestream.New(t, dut1, component1.PartNo().State(), samplingInterval),
		samplestream.New(t, dut1, component1.MfgName().State(), samplingInterval),
		samplestream.New(t, dut1, component1.MfgDate().State(), samplingInterval),
		samplestream.New(t, dut1, component1.HardwareVersion().State(), samplingInterval),
		samplestream.New(t, dut1, component1.FirmwareVersion().State(), samplingInterval),
		samplestream.New(t, dut1, component1.Description().State(), samplingInterval),
	)
	p1StreamsUnion = append(p1StreamsUnion, samplestream.New(t, dut1, component1.Type().State(), samplingInterval))

	verifyAllInventoryValues(t, p1StreamsStr, p1StreamsUnion)

	// Disable or shut down the interface on the DUT.
	gnmi.Replace(t, dut1, gnmi.OC().Interface(dp1.Name()).Enabled().Config(), false)
	gnmi.Replace(t, dut1, gnmi.OC().Interface(dp2.Name()).Enabled().Config(), false)
	// Wait for channels to be down.
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_DOWN)
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp2.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_DOWN)

	t.Logf("Interfaces are down: %v, %v", dp1.Name(), dp2.Name())
	verifyAllInventoryValues(t, p1StreamsStr, p1StreamsUnion)

	time.Sleep(waitInterval)
	// Re-enable interfaces.
	gnmi.Replace(t, dut1, gnmi.OC().Interface(dp1.Name()).Enabled().Config(), true)
	gnmi.Replace(t, dut1, gnmi.OC().Interface(dp2.Name()).Enabled().Config(), true)
	// Wait for channels to be up.
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
	gnmi.Await(t, dut1, gnmi.OC().Interface(dp2.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)

	t.Logf("Interfaces are up: %v, %v", dp1.Name(), dp2.Name())
	verifyAllInventoryValues(t, p1StreamsStr, p1StreamsUnion)
}
