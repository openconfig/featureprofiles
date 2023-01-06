package basetest

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	ipb "github.com/openconfig/featureprofiles/tools/inputcisco"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	inputFile = "testdata/isis.yaml"
)

var (
	testInput = ipb.LoadInput(inputFile)
	device1   = "dut"
	ate       = "ate"
	observer  = fptest.NewObserver("ISIS").AddCsvRecorder("ocreport").
			AddCsvRecorder("ISIS")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
func flapInterface(t *testing.T, dut *ondatra.DUTDevice, interfaceName string, flapDuration time.Duration) {
	initialState := gnmi.Get(t, dut, gnmi.OC().Interface(interfaceName).Enabled().State())
	transientState := !initialState
	setInterfaceState(t, dut, interfaceName, transientState)
	time.Sleep(flapDuration * time.Second)
	setInterfaceState(t, dut, interfaceName, initialState)
}
func setInterfaceState(t *testing.T, dut *ondatra.DUTDevice, interfaceName string, adminState bool) {

	i := &oc.Interface{
		Enabled: ygot.Bool(adminState),
		Name:    ygot.String(interfaceName),
	}
	updateResponse := gnmi.Update(t, dut, gnmi.OC().Interface(interfaceName).Config(), i)
	t.Logf("Update response : %v", updateResponse)
	currEnabledState := gnmi.Get(t, dut, gnmi.OC().Interface(interfaceName).Enabled().State())
	if currEnabledState != adminState {
		t.Fatalf("Failed to set interface adminState to :%v", adminState)
	} else {
		t.Logf("Interface adminState set to :%v", adminState)
	}
}
