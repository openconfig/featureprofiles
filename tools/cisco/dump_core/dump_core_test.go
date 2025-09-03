package basetest

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/testt"
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
		dutID := device.Id
		dutMap := map[string]*ondatra.DUTDevice{}
		dutMap[dutID] = ondatra.DUT(t, dutID)
		// dut := ondatra.DUT(t, dutID)
		t.Run(fmt.Sprintf("dump core files for device %s", dutID), func(t *testing.T) {
			t.Logf("Start dumping core for device: %s", dutID)
			for _, process := range processes {
				t.Logf("Dumping core for device: %s, process name: %s", dutID, process)
				commands := []string{
					fmt.Sprintf("dumpcore running %s location 0/RP0/CPU0\n", process),
					fmt.Sprintf("dir harddisk:%s*core*\n", process),
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
				defer cancel()
				sshClient := dutMap[dutID].RawAPIs().CLI(t)
				for _, cmd := range commands {
					testt.CaptureFatal(t, func(t testing.TB) {
						if result, err := sshClient.RunCommand(ctx, cmd); err == nil {
							t.Logf("%s> %s", dutID, cmd)
							t.Log(result.Output())
						} else {
							t.Logf("%s> %s", dutID, cmd)
							t.Log(err.Error())
						}
						t.Logf("\n")
					})
				}
			}
			t.Logf("Finished dumping core for device: %s", dutID)
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
