package basetest

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	ipb "github.com/openconfig/featureprofiles/tools/inputcisco"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

const (
	input_file = "isis.yaml"
)

var (
	testInput = ipb.LoadInput(input_file)
	device1   = "dut"
	device2   = "peer"
	ate       = "ate"
	observer  = fptest.NewObserver().AddCsvRecorder("ocreport").
			AddCsvRecorder("ISIS")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
func flapInterface(t *testing.T, dut *ondatra.DUTDevice, interface_name string, flap_duration time.Duration) {

	initialState := dut.Telemetry().Interface(interface_name).Get(t).GetEnabled()
	transientState := !initialState
	setInterfaceState(t, dut, interface_name, transientState)
	time.Sleep(flap_duration * time.Second)
	setInterfaceState(t, dut, interface_name, initialState)
}
func setInterfaceState(t *testing.T, dut *ondatra.DUTDevice, interface_name string, admin_state bool) {

	i := &oc.Interface{
		Enabled: ygot.Bool(admin_state),
		Name:    ygot.String(interface_name),
	}
	update_response := dut.Config().Interface(interface_name).Update(t, i)
	t.Logf("Update response : %v", update_response)
	currEnabledState := dut.Telemetry().Interface(interface_name).Get(t).GetEnabled()
	if currEnabledState != admin_state {
		t.Fatalf("Failed to set interface admin_state to :%v", admin_state)
	} else {
		t.Logf("Interface admin_state set to :%v", admin_state)
	}
}
