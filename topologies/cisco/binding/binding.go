package binding

import (
	"flag"
	"log"
	"os"

	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"google.golang.org/protobuf/encoding/prototext"
)

func GetBinding() *bindpb.Binding {
	bindingFile := flag.Lookup("binding").Value.String()

	in, err := os.ReadFile(bindingFile)
	if err != nil {
		log.Fatalf("unable to read binding file")
	}

	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		log.Fatalf("unable to parse binding file")
	}

	return b
}
