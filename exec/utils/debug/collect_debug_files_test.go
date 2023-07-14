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

var (
	showTechSupport = []string{
		"cef", "cef platform", "ofa", "insight", "rib", "fabric",
		"service-layer", "grpc", "spi", "hw-ac", "bundles", "cfgmgr",
		"ctrace", "ethernet interfaces", "fabric link-include", "p4rt",
		"interface", "optics", "pfi", "platform-fwd", "rdsfs", "sysdb",
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

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestCollectDebugFiles(t *testing.T) {
	if *outDirFlag == "" {
		t.Fatal("Missing outDir arg")
	} else {
		outDir = *outDirFlag
		timestamp = *timestampFlag
	}

	commands := []string{
		"run rm -rf /" + techDirectory,
		"mkdir " + techDirectory,
		"run find /misc/disk1 -maxdepth 1 -type f -name '*core*' -newermt @" + timestamp + " -exec cp \"{}\" /" + techDirectory + "/  \\\\;",
	}

	for _, t := range showTechSupport {
		commands = append(commands, fmt.Sprintf("show tech-support %s file %s", t, getTechFileName(t)))
	}

	for _, t := range pipedCmds {
		commands = append(commands, fmt.Sprintf("%s | file %s", t, getTechFileName(t)))
	}

	targets := getSSHInfo(t)

	for dutID, targetInfo := range targets {
		t.Logf("Collecting debug files on %s", dutID)

		ctx := context.Background()
		dut := ondatra.DUT(t, dutID)
		sshClient := dut.RawAPIs().CLI(t)
		defer sshClient.Close()

		for _, cmd := range commands {
			testt.CaptureFatal(t, func(t testing.TB) {
				if result, err := sshClient.SendCommand(ctx, cmd); err == nil {
					t.Logf("> %s", cmd)
					t.Log(result)
				} else {
					t.Logf("> %s", cmd)
					t.Log(err.Error())
				}
				t.Logf("\n")
			})
		}

		copyDebugFiles(t, targetInfo)
	}
}

func copyDebugFiles(t *testing.T, d targetInfo) {
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

	dutOutDir := filepath.Join(outDir, d.dut)
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

func getSSHInfo(t *testing.T) map[string]targetInfo {
	t.Helper()

	bindingFile := flag.Lookup("binding").Value.String()
	in, err := os.ReadFile(bindingFile)
	if err != nil {
		t.Fatalf("Unable to read binding file")
	}

	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		t.Fatalf("Unable to parse binding file")
	}

	targets := map[string]targetInfo{}
	for _, dut := range b.Duts {

		sshUser := dut.Ssh.Username
		if sshUser == "" {
			sshUser = dut.Options.Username
		}
		if sshUser == "" {
			sshUser = b.Options.Username
		}

		sshPass := dut.Ssh.Password
		if sshPass == "" {
			sshPass = dut.Options.Password
		}
		if sshPass == "" {
			sshPass = b.Options.Password
		}

		sshTarget := strings.Split(dut.Ssh.Target, ":")
		sshIP := sshTarget[0]
		sshPort := "22"
		if len(sshTarget) > 1 {
			sshPort = sshTarget[1]
		}

		targets[dut.Id] = targetInfo{
			dut:     dut.Id,
			sshIP:   sshIP,
			sshPort: sshPort,
			sshUser: sshUser,
			sshPass: sshPass,
		}
	}
	return targets
}

func getTechFileName(tech string) string {
	return techDirectory + "/" + strings.ReplaceAll(tech, " ", "_")
}
