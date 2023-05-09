package smu_install_test

import (
	"context"
	"flag"
	"fmt"
	"io"
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
	smuDestination      = "/harddisk:/smu"
	fileTransferTimeout = 900 * time.Second
	installTimeout      = 1800 * time.Second
	sshCmdTimeout       = 30 * time.Second
	statusCheckDelay    = 60 * time.Second
)

var (
	smuPathsFlag = flag.String("smus", "", "Comma separated list of smu paths")
	smuPaths     []string
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type targetInfo struct {
	dut     string
	sshIp   string
	sshPort string
	sshUser string
	sshPass string
}

func TestSMUInstall(t *testing.T) {
	if *smuPathsFlag == "" {
		t.Fatal("Missing smuPathsFlag arg")
	}
	smuPaths = strings.Split(*smuPathsFlag, ",")

	for _, p := range smuPaths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("SMU {%s} does not exist: %v", p, err)
		}
	}

	for _, d := range parseBindingFile(t) {
		target := fmt.Sprintf("%s:%s", d.sshIp, d.sshPort)
		t.Logf("Copying smus to %s (%s)", d.dut, target)

		dut := ondatra.DUT(t, d.dut)
		if _, err := sendCLI(t, dut, "run rm -rf "+smuDestination); err != nil {
			t.Fatalf("Error sending command: %v", err)
		}
		if _, err := sendCLI(t, dut, "mkdir "+smuDestination); err != nil {
			t.Fatalf("Error sending command: %v", err)
		}

		time.Sleep(5 * time.Second)

		sshConf := scp.NewSSHConfigFromPassword(d.sshUser, d.sshPass)
		scpClient, err := scp.NewClient(target, sshConf, &scp.ClientOption{})
		if err != nil {
			t.Fatalf("Error initializing scp client: %v", err)
		}
		defer scpClient.Close()

		for _, p := range smuPaths {
			file := filepath.Base(p)

			if err := scpClient.CopyFileToRemote(p, smuDestination+"/"+file, &scp.FileTransferOption{
				Timeout: fileTransferTimeout,
			}); err != nil {
				t.Fatalf("Error copying smus to target %s (%s:%s): %v", d.dut, d.sshIp, d.sshPort, err)
			}
		}
		scpClient.Close()

		runInstallCmd(t, dut, "install source "+smuDestination+" noprompt")
		runInstallCmd(t, dut, "install commit")
	}
}

func runInstallCmd(t testing.TB, dut *ondatra.DUTDevice, cmd string) {
	if result, err := sendCLI(t, dut, cmd); err == nil {
		if !strings.Contains(result, "has started") {
			t.Fatalf("Unexpected response:\n%s\n", result)
		}
	} else {
		t.Fatalf("Error running command: %v", err)
	}

	success := false
	for start := time.Now(); time.Since(start) < installTimeout && !success; {
		time.Sleep(statusCheckDelay)

		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			if result, err := sendCLI(t, dut, "sh install request"); err == nil {
				if strings.Contains(result, "In progress") {
					t.Logf("Install operation in progress...")
				} else if strings.Contains(result, "Failure") {
					t.Fatalf("Install operation failed:\n%s\n", result)
				} else if strings.Contains(result, "Success") {
					success = true
				} else {
					t.Fatalf("Unexpected response:\n%s\n", result)
				}
			} else if err == context.DeadlineExceeded || err == io.EOF {
				t.Logf("Device is probably rebooting...")
			} else {
				t.Logf("Error running command: %v", err)
			}
		}); errMsg != nil {
			t.Logf("Device is probably rebooting...")
		}
	}

	if !success {
		t.Fatalf("Install operation timed out")
	}
}

func sendCLI(t testing.TB, dut *ondatra.DUTDevice, cmd string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), sshCmdTimeout)
	defer cancel()
	sshClient := dut.RawAPIs().CLI(t)
	defer sshClient.Close()
	return sshClient.SendCommand(ctx, cmd)
}

func parseBindingFile(t *testing.T) []targetInfo {
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

	targets := []targetInfo{}
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
		sshIp := sshTarget[0]
		sshPort := "22"
		if len(sshTarget) > 1 {
			sshPort = sshTarget[1]
		}

		targets = append(targets, targetInfo{
			dut:     dut.Id,
			sshIp:   sshIp,
			sshPort: sshPort,
			sshUser: sshUser,
			sshPass: sshPass,
		})
	}

	return targets
}
