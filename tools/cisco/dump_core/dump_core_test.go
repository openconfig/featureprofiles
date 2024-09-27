package basetest

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/ondatra"
	"google.golang.org/protobuf/encoding/prototext"
)

var (
	processList = flag.String("processlist", "", "Comma-separated list of process")
)

var (
	// default process to dump core
	processes = []string{
		"bundlemgr_checker", "ifmgr", "netio", "pkt_trace_agent",
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestCoreFileDecode(t *testing.T) {
	if *processList != "" {
		processes = strings.Split(*processList, ",")
	}

	b, err := processBindingFile(t)
	if err != nil {
		t.Fatalf("Failed to process binding file: %v", err)
	}

	for _, device := range b.Duts {
		dutName := device.Name
		dut := ondatra.DUT(t, dutName)
		t.Run(fmt.Sprintf("dump core files for device %s", dutName), func(t *testing.T) {
			t.Logf("Start dumping core for device: %s", dutName)
			for _, process := range processes {
				process := process // Capture the current value of process
				t.Logf("Dumping core for device: %s, process name: %s", dutName, process)
				t.Logf("%s>dumpcore running %s location 0/RP0/CPU0\n", dutName, process)
				dut.CLI().RunResult(t, fmt.Sprintf("dumpcore running %s location 0/RP0/CPU0\n", process))
			}
			t.Logf("Finished dumping core for device: %s", dutName)
		})
	}
}

func processBindingFile(t *testing.T) (*bindpb.Binding, error) {
	t.Helper()
	t.Log("Starting processing binding file")

	bf := flag.Lookup("binding")
	if bf == nil {
		return nil, fmt.Errorf("binding file flag not set correctly")
	}

	bindingFile := bf.Value.String()
	if bindingFile == "" {
		return nil, fmt.Errorf("binding file path is empty")
	}

	in, err := os.ReadFile(bindingFile)
	if err != nil {
		return nil, fmt.Errorf("error reading binding file: %v", err)
	}

	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		return nil, fmt.Errorf("error processing binding file: %v", err)
	}

	t.Log("Processing binding file successful")
	return b, nil
}
