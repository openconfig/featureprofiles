package main

import (
	"flag"
	"fmt"
	"os"

	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
)

var (
	bindingFileFlag = flag.String("binding", "", "Path to binding file")
	outputFileFlag  = flag.String("out", "", "Path to output json file")
)

func main() {
	flag.Parse()

	if *bindingFileFlag == "" {
		die("Missing binding arg")
	}

	in, err := os.ReadFile(*bindingFileFlag)
	if err != nil {
		die("Unable to read binding file: %v", err)
	}

	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		die("Unable to parse binding file: %v", err)
	}

	opts := protojson.MarshalOptions{
		Multiline: true,
		Indent:    "  ",
	}

	j, err := opts.Marshal(b)
	if err != nil {
		die("Unable to marshal binding: %v", err)
	}

	if *outputFileFlag == "" {
		fmt.Println(string(j))
	} else {
		os.WriteFile(*outputFileFlag, j, 0644)
	}
}

func die(format string, a ...any) {
	fmt.Printf(format+"\n", a...)
	os.Exit(1)
}
