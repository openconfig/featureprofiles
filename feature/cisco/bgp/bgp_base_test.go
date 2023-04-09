package basetest

import (
	"testing"
	"fmt"

	"github.com/openconfig/featureprofiles/internal/fptest"
	ipb "github.com/openconfig/featureprofiles/tools/inputcisco"
	"github.com/openconfig/featureprofiles/tools/cisco/yang_coverage"
	"github.com/openconfig/ondatra/eventlis"
)

const (
	inputFile = "testdata/bgp.yaml"
)

var (
	testInput = ipb.LoadInput(inputFile)
	device1   = "dut"
	ate1      = "ate"
	observer  = fptest.NewObserver("BGP").AddCsvRecorder("ocreport").
			AddCsvRecorder("BGP")
	event = eventlis.EventListener{}
)

func TestMain(m *testing.M) {
	fmt.Println("start of main")
	ws := "/nobackup/sanshety/ws/iosxr"
	models := []string{
		fmt.Sprintf("%s/manageability/yang/pyang/modules/openconfig-network-instance.yang", ws),
		fmt.Sprintf("%s/manageability/yang/pyang/modules/cisco-xr-openconfig-network-instance-deviations.yang", ws),
	}
	prefixPaths := []string{"/network-instances/network-instance/protocols/protocol/bgp",
	"/network-instances/network-instance/inter-instance-policies"}

	err := yang_coverage.CreateInstance("oc-sanity", models, prefixPaths, ws, event)
	fptest.RunTests(m)
	fmt.Println("end of main ", err)
}

