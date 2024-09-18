package zr_inventory_test

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
	timeout          = 5 * time.Minute
	waitInterval     = 30 * time.Second
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
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
	dut := ondatra.DUT(t, "dut")
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	cfgplugins.InterfaceConfig(t, dut, dp1)
	cfgplugins.InterfaceConfig(t, dut, dp2)

	// Derive transceiver names from ports.
	tr1 := gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).Transceiver().State())
	tr2 := gnmi.Get(t, dut, gnmi.OC().Interface(dp2.Name()).Transceiver().State())

	if (dp1.PMD() != ondatra.PMD400GBASEZR) || (dp2.PMD() != ondatra.PMD400GBASEZR) {
		t.Fatalf("Transceivers types (%v, %v): (%v, %v) are not 400ZR, expected %v", tr1, tr2, dp1.PMD(), dp2.PMD(), ondatra.PMD400GBASEZR)
	}
	component1 := gnmi.OC().Component(tr1)

	// Wait for channels to be up.
	gnmi.Await(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
	gnmi.Await(t, dut, gnmi.OC().Interface(dp2.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)

	var p1StreamsStr []*samplestream.SampleStream[string]
	var p1StreamsUnion []*samplestream.SampleStream[oc.Component_Type_Union]

	// TODO: b/333021032 - Uncomment the description check from the test once the bug is fixed.
	p1StreamsStr = append(p1StreamsStr,
		samplestream.New(t, dut, component1.SerialNo().State(), samplingInterval),
		samplestream.New(t, dut, component1.PartNo().State(), samplingInterval),
		samplestream.New(t, dut, component1.MfgName().State(), samplingInterval),
		samplestream.New(t, dut, component1.MfgDate().State(), samplingInterval),
		samplestream.New(t, dut, component1.HardwareVersion().State(), samplingInterval),
		samplestream.New(t, dut, component1.FirmwareVersion().State(), samplingInterval),
		// samplestream.New(t, dut1, component1.Description().State(), samplingInterval),
	)
	p1StreamsUnion = append(p1StreamsUnion, samplestream.New(t, dut, component1.Type().State(), samplingInterval))

	verifyAllInventoryValues(t, p1StreamsStr, p1StreamsUnion)

	// Disable or shut down the interface on the DUT.
	for _, p := range dut.Ports() {
		cfgplugins.ToggleInterface(t, dut, p.Name(), false)
	}
	// Wait for channels to be down.
	gnmi.Await(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_DOWN)
	gnmi.Await(t, dut, gnmi.OC().Interface(dp2.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_DOWN)

	t.Logf("Interfaces are down: %v, %v", dp1.Name(), dp2.Name())
	verifyAllInventoryValues(t, p1StreamsStr, p1StreamsUnion)

	time.Sleep(waitInterval)
	// Re-enable interfaces.
	for _, p := range dut.Ports() {
		cfgplugins.ToggleInterface(t, dut, p.Name(), true)
	}
	// Wait for channels to be up.
	gnmi.Await(t, dut, gnmi.OC().Interface(dp1.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
	gnmi.Await(t, dut, gnmi.OC().Interface(dp2.Name()).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)

	t.Logf("Interfaces are up: %v, %v", dp1.Name(), dp2.Name())
	verifyAllInventoryValues(t, p1StreamsStr, p1StreamsUnion)
}
