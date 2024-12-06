package main

import (
	"flag"
	"fmt"
	"os"

	testbedpb "github.com/openconfig/ondatra/proto"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
)

var (
	testbed        = flag.String("testbed", "", "Path to testbed file")
	outputFileFlag = flag.String("out", "", "Path to output json file")
)

func main() {
	flag.Parse()

	if *testbed == "" {
		die("Missing testbed arg")
	}

	in, err := os.ReadFile(*testbed)
	if err != nil {
		die("Unable to read testbed file: %v", err)
	}

	b := &testbedpb.Testbed{}
	if err := prototext.Unmarshal(in, b); err != nil {
		die("Unable to parse testbed file: %v", err)
	}

	opts := protojson.MarshalOptions{
		Multiline: true,
		Indent:    "  ",
	}

	j, err := opts.Marshal(b)
	if err != nil {
		die("Unable to marshal testbed: %v", err)
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
