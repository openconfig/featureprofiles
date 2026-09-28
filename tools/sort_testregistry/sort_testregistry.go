// Binary sort_registry sorts the test registry lexically such that it is easier
// for humans to add to the file and find the next available ID. It can be run
// by running:
//
//	go run tools/sort_testregistry/sort_testregistry.go
//
// prior to submitting.
package main

import (
	"bytes"
	"flag"
	"os"
	"sort"

	log "github.com/golang/glog"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	tpb "github.com/openconfig/featureprofiles/proto/testregistry_go_proto"
)

var (
	file = flag.String("file", "testregistry.textproto", "input file to read registry from")
)

func main() {
	flag.Parse()
	r := &tpb.TestRegistry{}

	f, err := os.ReadFile(*file)
	if err != nil {
		log.Exitf("cannot read input, err: %v", err)
	}
	if err := prototext.Unmarshal(f, r); err != nil {
		log.Exitf("invalid registry input, err: %v", err)
	}

	ids := []string{}
	tests := map[string][]*tpb.Test{}
	for _, t := range r.GetTest() {
		if _, ok := tests[t.GetId()]; !ok {
			tests[t.GetId()] = []*tpb.Test{}
			ids = append(ids, t.GetId())
		}

		// Deduplicate identical entries.
		var skip bool
		for _, existing := range tests[t.GetId()] {
			if proto.Equal(existing, t) {
				log.Infof("skipping duplicate for %s", t.GetId())
				skip = true
			}
		}
		if skip {
			continue
		}

		tests[t.GetId()] = append(tests[t.GetId()], t)

	}

	n := &tpb.TestRegistry{
		Name: r.GetName(),
		Test: []*tpb.Test{},
	}

	sort.Strings(ids)
	for _, i := range ids {
		for _, tc := range tests[i] {
			log.Infof("appending %s to %s", tc.GetId(), i)
			n.Test = append(n.Test, tc)
		}
	}
	mo := &prototext.MarshalOptions{
		Multiline: true,
		Indent:    "  ",
	}

	s, err := mo.Marshal(n)
	if err != nil {
		log.Exitf("cannot marshal proceessed proto, err: %v", err)
	}

	b := &bytes.Buffer{}
	b.WriteString("# proto-file: /proto/testregistry.proto\n")
	b.WriteString("# proto-message: TestRegistry\n\n")
	b.Write(s)

	if err := os.WriteFile(*file, b.Bytes(), 0644); err != nil {
		log.Exitf("cannot write out processed file, err: %v", err)
	}
}
