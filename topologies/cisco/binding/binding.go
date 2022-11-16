// Internal binding utils
package binding

import (
	"flag"
	"os"
	"testing"

	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"google.golang.org/protobuf/encoding/prototext"
)

// GetBinding returns the parsed binding struct
func GetBinding(t *testing.T) *bindpb.Binding {
	bindingFile := flag.Lookup("binding").Value.String()

	in, err := os.ReadFile(bindingFile)
	if err != nil {
		t.Fatalf("unable to read binding file")
	}

	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		t.Fatalf("unable to parse binding file")
	}

	return b
}
