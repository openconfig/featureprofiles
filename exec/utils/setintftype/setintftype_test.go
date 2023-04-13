package setinftype_test

import (
	"flag"
	"os"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"google.golang.org/protobuf/encoding/prototext"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestSetIntfType(t *testing.T) {
	for _, d := range getDUTs(t) {
		dut := ondatra.DUT(t, d.Id)
		for _, p := range dut.Ports() {
			gnmi.Update(t, dut, gnmi.OC().Interface(p.Name()).Type().Config(),
				oc.IETFInterfaces_InterfaceType_ethernetCsmacd)
		}
	}
}

func getDUTs(t *testing.T) []*bindpb.Device {
	t.Helper()

	bindingFile := flag.Lookup("binding").Value.String()
	in, err := os.ReadFile(bindingFile)
	if err != nil {
		t.Fatalf("unable to read binding file")
	}

	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		t.Fatalf("unable to parse binding file")
	}
	return b.Duts
}
