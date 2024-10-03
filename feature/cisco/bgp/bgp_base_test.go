package bgp_test

import (
	"fmt"
	"testing"
	"time"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/fptest"
	ipb "github.com/openconfig/featureprofiles/tools/inputcisco"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	inputFile                      = "testdata/bgp.yaml"
	telemetryTimeout time.Duration = 10 * time.Second
	configApplyTime  time.Duration = 5 * time.Second // FIXME: Workaround
	configDeleteTime time.Duration = 5 * time.Second // FIXME: Workaround
	dutName          string        = "dut"
)

var (
	testInput = ipb.LoadInput(inputFile)
	device1   = "dut"
	ate1      = "ate"
	observer  = fptest.NewObserver("BGP").AddCsvRecorder("ocreport").
			AddCsvRecorder("BGP")
)

func fixBgpLeafRefConstraints(t *testing.T, dut *ondatra.DUTDevice, bgpInstance string) {
	t.Helper()
	// update name inside Config container to satisfy leafref constraint on the list key
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Name().Config(), *ciscoFlags.DefaultNetworkInstance)
	// update identifier and name inside /protocols/protocol/ path to satisfy leafref constraint on the list key
	protoPath := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance)
	batchSet := &gnmi.SetBatch{}
	gnmi.BatchUpdate(batchSet, protoPath.Identifier().Config(), oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP)
	gnmi.BatchUpdate(batchSet, protoPath.Name().Config(), bgpInstance)
	batchSet.Set(t, dut)
}

func cleanup(t *testing.T, dut *ondatra.DUTDevice, bgpInst string) {
	gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInst).Bgp().Config())
	time.Sleep(configDeleteTime)
}

// NOTE: Using separate BGP instances due to XR errors when back-to-back
// delete and re-add hits failure on BGP backend cleanup.
// FIXME: May need to be triaged in XR BGP implementation or XR config backend.
func getNextBgpInstance(instanceName string, asNumber uint32) (string, uint32) {
	var index uint32 = 1
	i := index
	index++
	return fmt.Sprintf("%s_%d", instanceName, i), i + asNumber
}
