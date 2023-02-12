package upgrade_test

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
	"github.com/openconfig/ondatra/raw"
	"github.com/povsister/scp"
	"google.golang.org/protobuf/encoding/prototext"
)

const (
	imageDestination = "/harddisk:/8000-x64.iso"
	installCmd       = "install replace " + imageDestination + " noprompt commit"
	installStatusCmd = "sh install request"
	sshCmdTimeout    = 30 * time.Second
	statusCheckDelay = 30 * time.Second
	rebootDelay      = 60 * time.Second
	rebootRetries    = 3
)

var (
	imagePathFlag = flag.String("imagePath", "", "Full path to image iso")
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

func TestSoftwareUpgrade(t *testing.T) {
	if *imagePathFlag == "" {
		t.Fatal("Missing imagePath arg")
	}

	imagePath := *imagePathFlag
	if _, err := os.Stat(imagePath); err != nil {
		t.Fatalf("Image {%s} does not exist: %v", imagePath, err)
	}

	for _, d := range parseBindingFile(t) {
		target := fmt.Sprintf("%s:%s", d.sshIp, d.sshPort)
		t.Logf("Copying image to %s (%s)", d.dut, target)
		sshConf := scp.NewSSHConfigFromPassword(d.sshUser, d.sshPass)
		scpClient, err := scp.NewClient(target, sshConf, &scp.ClientOption{})
		if err != nil {
			t.Fatalf("Error initializing scp client: %v", err)
		}
		defer scpClient.Close()

		if err := scpClient.CopyFileToRemote(imagePath, "/harddisk:/8000-x64.iso", &scp.FileTransferOption{}); err != nil {
			t.Fatalf("Error copying image to target %s (%s:%s): %v", d.dut, d.sshIp, d.sshPort, err)
		}

		dut := ondatra.DUT(t, d.dut)
		sshClient := dut.RawAPIs().CLI(t)
		defer sshClient.Close()

		if result, err := sendCLI(t, sshClient, installCmd); err == nil {
			if !strings.Contains(result, "has started") {
				t.Fatalf("Unexpected response:\n%s\n", result)
			}
		} else {
			t.Fatalf("Error running command: %v", err)
		}

		retries := 0
		for {
			if result, err := sendCLI(t, sshClient, installStatusCmd); err == nil {
				if strings.Contains(result, "In progress") {
					t.Logf("Install operation in progress. Sleeping for %v...", statusCheckDelay)
					time.Sleep(statusCheckDelay)
					continue
				} else if strings.Contains(result, "Failure") {
					t.Fatalf("Upgrade failed:\n%s\n", result)
				} else if strings.Contains(result, "Success") {
					break
				} else {
					t.Fatalf("Unexpected response:\n%s\n", result)
				}
			} else if err == context.DeadlineExceeded {
				if retries < rebootRetries {
					t.Logf("Device probably rebooting. Sleeping for %v...", rebootDelay)
					retries += 1
					time.Sleep(rebootDelay)
				} else {
					t.Fatalf("Gave up waiting for device to come up")
				}
			} else {
				t.Logf("Error running command: %v", err)
			}
		}
	}
}

func sendCLI(t *testing.T, sshClient raw.StreamClient, cmd string) (string, error) {
	t.Helper()
	t.Logf("Sending command: %s", cmd)
	ctx, cancel := context.WithTimeout(context.Background(), sshCmdTimeout)
	defer cancel()
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
