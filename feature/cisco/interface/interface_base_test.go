package basetest

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	ipb "github.com/openconfig/featureprofiles/tools/inputcisco"
	"github.com/openconfig/featureprofiles/tools/inputcisco/feature"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
)

const (
	inputFile = "testdata/interface.yaml"
)

var (
	testInput = ipb.LoadInput(inputFile)
	device1   = "dut"
	observer  = fptest.NewObserver("Interface").AddCsvRecorder("ocreport").
			AddCsvRecorder("Interface")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
func sliceEqual(a, b []string) bool {
	sort.Strings(a)
	sort.Strings(b)
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func verifyForwardingViable(t *testing.T, dut *ondatra.DUTDevice, interfaceName string) {
	t.Logf("Set forwarding viable on interface %v ", interfaceName)

	rawData := fmt.Sprintf(` {
						"Cisco-IOS-XR-ifmgr-cfg:interface-configurations": {
						 "interface-configuration": [
						  {
						   "active": "act",
						   "interface-name": "%v",
						   "Cisco-IOS-XR-drivers-media-eth-cfg:ethernet": {
							"forwarding-unviable": [
							 null
							]
						   }
						  }
						 ]
						}
					}`, interfaceName)
	fjson, err := os.CreateTemp("", "fv.json")
	if err != nil {
		t.Error("Error occcured while creating the file ", err)

	}
	defer os.Remove(fjson.Name())
	_, err = fjson.Write([]byte(rawData))
	if err != nil {
		t.Error("There is error when applying the config: ", err)
	}
	feature.ConfigJSON(dut, t, fjson.Name())

	t.Logf("Get and verify forwarding-unviable config %v", interfaceName)
	getRequest := &gnmipb.GetRequest{
		Path: []*gnmipb.Path{
			{Origin: "openconfig", Elem: []*gnmipb.PathElem{
				{Name: "Cisco-IOS-XR-ifmgr-cfg:interface-configurations"},
				{Name: "interface-configuration", Key: map[string]string{"active": "act", "interface-name": interfaceName}},
			}},
		},
		Type:     gnmipb.GetRequest_CONFIG,
		Encoding: gnmipb.Encoding_JSON_IETF,
	}
	res, err := dut.RawAPIs().GNMI().Default(t).Get(context.Background(), getRequest)
	if err != nil {
		t.Fatal("There is error when getting configuration: ", err)

	}
	t.Log(res)
	if strings.Contains(res.String(), "forwarding-unviable") {
		t.Logf("Configured forwarding-unviable on interface %v ", interfaceName)
	} else {
		t.Errorf("Forwarding-unviable not configured on the interface %v ", interfaceName)
	}

}
