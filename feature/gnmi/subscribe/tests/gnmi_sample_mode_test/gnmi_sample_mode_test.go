package gnmi_sample_mode_test

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/isissession"
	"github.com/openconfig/featureprofiles/internal/samplestream"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

var (
	dutPort1 = &attrs.Attributes{
		Desc:    "DUT Port 1",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestGNMISampleMode(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	p1 := dut.Port(t, "port1")

	p1Stream := samplestream.New(t, dut, gnmi.OC().Interface(p1.Name()).Description().State(), 10*time.Second)
	defer p1Stream.Close()

	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))

	desc := p1Stream.Next()
	if desc == nil {
		t.Errorf("Interface %q telemetry not received after config", p1.Name())
	} else {
		v, ok := desc.Val()
		t.Logf("Description from stream : %s", v)
		if !ok {
			t.Errorf("Interface %q telemetry empty after config", p1.Name())
		}

		if got, want := v, dutPort1.Desc; got != want {
			t.Errorf("Interface %q telemetry description is %q, want %q", p1.Name(), got, want)
		}
	}

	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Description().Config(), "DUT Port 1 - Updated")

	desc = p1Stream.Next()
	if desc == nil {
		t.Errorf("Interface %q telemetry not received after description update", p1.Name())
	} else {
		v, ok := desc.Val()
		t.Logf("Description from stream : %s", v)
		if !ok {
			t.Errorf("Interface %q telemetry empty after description update", p1.Name())
		}

		if got, want := v, "DUT Port 1 - Updated"; got != want {
			t.Errorf("Interface %q telemetry description is %q, want %q", p1.Name(), got, want)
		}
	}
}

func TestNoInvalidValuesOnInterfaceFlap(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	p1 := dut.Port(t, "port1")

	p1Stream := samplestream.New(t, dut, gnmi.OC().Interface(p1.Name()).Description().State(), 10*time.Second)
	defer p1Stream.Close()

	// Configure Interface.
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	// Wait until interface is UP.
	gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).AdminStatus().State(), 30*time.Second, oc.Interface_AdminStatus_UP)
	time.Sleep(10 * time.Second) // wait 10 seconds for at-least 1 stream value.

	// Flap interface by setting enabled to false
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Enabled().Config(), false)

	// Wait until interface is DOWN.
	gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).AdminStatus().State(), 30*time.Second, oc.Interface_AdminStatus_DOWN)
	time.Sleep(10 * time.Second) // wait 10 seconds for at-least 1 stream value.

	// Re-enable interface
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Enabled().Config(), true)

	// Wait until interface is UP.
	gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).AdminStatus().State(), 30*time.Second, oc.Interface_AdminStatus_UP)
	time.Sleep(10 * time.Second) // wait 10 seconds for at-least 1 stream value.

	// Now validate description stream didn't return any invalid values.
	vals := p1Stream.All()

	for idx, v := range vals {
		if v, ok := v.Val(); !ok {
			t.Errorf("Interface %q telemetry invalid description received: %v", p1.Name(), v)
		} else {
			t.Logf("Description from stream-%d: %s", idx, v)
		}
	}
}

func TestISISProtocol(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Configure Default Network Instance
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	niPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).State()
	niStream := samplestream.New(t, dut, niPath, 10*time.Second)
	defer niStream.Close()

	isissession.MustNew(t).WithISIS().PushDUT(context.Background(), t)

	// Starting ISIS protocol takes some time to converge. So we may not receive ISIS data
	// in the first sample after configuration.
	samples := niStream.Nexts(5)

	updated := false
	for idx, sample := range samples {
		if sample == nil {
			t.Logf("ISIS session %q telemetry not received after configuration", isissession.ISISName)
			continue
		}
		v, ok := sample.Val()
		if !ok {
			t.Logf("ISIS session %q telemetry empty after configuration", isissession.ISISName)
			continue
		}
		fptest.LogQuery(t, "Network Instance Data", niPath, v)
		if v.GetProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isissession.ISISName).GetIsis() != nil {
			t.Logf("ISIS session %q telemetry received in sample: %d", isissession.ISISName, idx)
			updated = true
			break
		}
	}

	if !updated {
		t.Errorf("ISIS session %q telemetry not received in three samples after configuration", isissession.ISISName)
	}
}
