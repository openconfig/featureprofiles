// package debug
//
// firex records the text in the error log
package debug

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/testt"
	"github.com/povsister/scp"
	"google.golang.org/protobuf/encoding/prototext"
)

const (
	techDirectory  = "harddisk:/firex_tech"
	scpCopyTimeout = 300 * time.Second
)

var (
	outDirFlag    = flag.String("outDir", "", "Directory where debug files should be copied")
	timestampFlag = flag.String("timestamp", "1", "Test start timestamp")
	coreFilesFlag = flag.Bool("core", false, "Check for core files that get generated")
	outDir        string
	timestamp     string
)

type targetInfo struct {
	dut     string
	sshIP   string
	sshPort string
	sshUser string
	sshPass string
}

type Targets struct {
	targetInfo map[string]targetInfo
}

var (
	showTechSupport = []string{
		"cef", "cef platform", "ofa", "insight", "rib", "fabric",
		"service-layer", "grpc", "spi", "hw-ac", "bundles", "cfgmgr",
		"ctrace", "ethernet interfaces", "fabric link-include", "p4rt",
		"interface", "optics", "pfi", "platform-fwd", "pbr", "rdsfs", "sysdb",
		"telemetry model-driven", "routing isis", "routing bgp", "linux networking",
		"install",
	}
	pipedCmds = []string{
		"show grpc trace all",
		"show telemetry model-driven trace all",
		"show cef global gribi aft internal location all",
		"show version",
		"show platform",
		"show install fixes active",
		"show running-config",
		"show context location all",
		"show logging",
		"show processes blocked location all",
		"show redundancy",
		"show reboot history detail",
	}
)

// NewTargets initialize the Targets
func NewTargets(t *testing.T) *Targets {
	t.Helper()
	// set up all ssh for the targets
	nt := Targets{
		targetInfo: make(map[string]targetInfo),
	}

	err := nt.getSSHInfo(t)
	if err != nil {
		t.FailNow()
	}
	return &nt
}

// TestMain sets up Ondatra tests init
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestCollectDebugFiles collects debug commands if coreFile flag is set to false, else it Skips the test
func TestCollectDebugFiles(t *testing.T) {
	// set up Targets
	targets := NewTargets(t)
	if *outDirFlag == "" {
		t.FailNow()
	} else {
		outDir = *outDirFlag
		timestamp = *timestampFlag
	}

	commands := []string{
		"run rm -rf /" + techDirectory,
		"mkdir " + techDirectory,
		"run find /misc/disk1 -maxdepth 1 -type f -name '*core*' -newermt @" + timestamp + " -exec cp \"{}\" /" + techDirectory + "/  \\\\;",
	}

	if *coreFilesFlag == false {
		for _, t := range showTechSupport {
			commands = append(commands, fmt.Sprintf("show tech-support %s file %s", t, getTechFileName(t)))
		}

		for _, t := range pipedCmds {
			commands = append(commands, fmt.Sprintf("%s | file %s", t, getTechFileName(t)))
		}
	}

	for dutID, targetInfo := range targets.targetInfo {

		ctx := context.Background()
		cli := targets.GetOndatraCLI(t, dutID)

		for _, cmd := range commands {
			fmt.Println(fmt.Sprintf("Running current command: [%s]", cmd))
			testt.CaptureFatal(t, func(t testing.TB) {
				if result, err := cli.SendCommand(ctx, cmd); err == nil {
					t.Logf("> %s", cmd)
					t.Log(result)
				} else {
					t.Logf("> %s", cmd)
					t.Log(err.Error())
				}
				t.Logf("\n")
			})
		}

		copyDebugFiles(t, targetInfo, "CollectDebugFiles")
	}
}

// copyDebugFiles copies files from the runs to an specified directory with a filename
//
// d targetInfo - contains the dut info
// filename - self-explanatory
func copyDebugFiles(t *testing.T, d targetInfo, filename string) {
	t.Helper()

	target := fmt.Sprintf("%s:%s", d.sshIP, d.sshPort)
	t.Logf("Copying debug files from %s (%s)", d.dut, target)

	sshConf := scp.NewSSHConfigFromPassword(d.sshUser, d.sshPass)
	scpClient, err := scp.NewClient(target, sshConf, &scp.ClientOption{})
	if err != nil {
		t.Errorf("Error initializing scp client: %v", err)
		return
	}
	defer scpClient.Close()

	dutOutDir := filepath.Join(outDir, d.dut, filename)
	if err := os.MkdirAll(dutOutDir, os.ModePerm); err != nil {
		t.Errorf("Error creating output directory: %v", err)
		return
	}

	if err := scpClient.CopyDirFromRemote("/"+techDirectory, dutOutDir, &scp.DirTransferOption{
		Timeout: scpCopyTimeout,
	}); err != nil {
		t.Errorf("Error copying debug files: %v", err)
	}
}

// getSSHInfo adds dut ssh info to a slice targetInfo[dut.Id]
//
// return an error if any
func (ti *Targets) getSSHInfo(t *testing.T) error {
	t.Helper()

	bf := flag.Lookup("binding")
	var bindingFile string

	if bf == nil {
		return fmt.Errorf(fmt.Sprintf("binding file not set correctly : [%s]", bf.Value.String()))
	}

	bindingFile = bf.Value.String()

	in, err := os.ReadFile(bindingFile)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("Error reading binding file: [%v]", err))
	}

	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		return fmt.Errorf(fmt.Sprintf("Error unmarshalling binding file: [%v]", err))
	}

	for _, dut := range b.Duts {
		var sshUser, sshPass, sshIP, sshPort string
		var sshTarget []string
		if dut.Ssh != nil {
			sshUser = dut.Ssh.Username
			sshPass = dut.Ssh.Password

			if dut.Ssh.Target != "" {
				sshTarget = strings.Split(dut.Ssh.Target, ":")
				sshIP = sshTarget[0]
				if len(sshTarget) > 1 {
					sshPort = sshTarget[1]
				}
			}
		} else if dut.Options != nil {
			sshUser = dut.Options.Username
			sshPass = dut.Options.Password
		} else if b.Options != nil {
			sshUser = b.Options.Username
			sshPass = b.Options.Password
		} else {
			return fmt.Errorf("Could not find correct dut username or password")
		}

		if dut.Id != "" && sshIP != "" && sshPort != "" && sshUser != "" && sshPass != "" {
			ti.targetInfo[dut.Id] = targetInfo{
				dut:     dut.Id,
				sshIP:   sshIP,
				sshPort: sshPort,
				sshUser: sshUser,
				sshPass: sshPass,
			}
		} else {
			return fmt.Errorf("One or more values are empty  dut.Id, sshIP, sshPort, sshUser, sshPass ")
		}
	}
	return nil
}

// getTechFileName return the techDirecory + / + replacing " " with _
func getTechFileName(tech string) string {
	return techDirectory + "/" + strings.ReplaceAll(tech, " ", "_")
}

// GetOndatraCLI
//
// returns a new streaming CLI client for the DUT.
func (ti *Targets) GetOndatraCLI(t *testing.T, dutID string) binding.CLIClient {
	t.Helper()
	dut := ondatra.DUT(t, dutID)

	return dut.RawAPIs().CLI(t)
}
