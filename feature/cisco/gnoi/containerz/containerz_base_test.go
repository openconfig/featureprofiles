package containerz_test

import (
	"context"
	"flag"
	"fmt"

	"os"
	"strings"
	"testing"
	"time"

	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/povsister/scp"

	"github.com/openconfig/ondatra"
	"google.golang.org/protobuf/encoding/prototext"
)

const (
	fileCopyTimeout = 1800 * time.Second
)

func parseBindingFile(t *testing.T) []dutInfo {
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

	targets := []dutInfo{}
	for _, dut := range b.Duts {

		gnoiUser := dut.Gnoi.Username
		if gnoiUser == "" {
			gnoiUser = dut.Options.Username
		}
		if gnoiUser == "" {
			gnoiUser = b.Options.Username
		}

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

		gnoiTarget := strings.Split(dut.Gnoi.Target, ":")
		gnoiIp := gnoiTarget[0]
		gnoiPort := gnoiTarget[1]

		targets = append(targets, dutInfo{
			dut:      dut.Id,
			sshIp:    sshIp,
			sshPort:  sshPort,
			sshUser:  sshUser,
			sshPass:  sshPass,
			gnoiIp:   gnoiIp,
			gnoiPort: gnoiPort,
			gnoiUser: gnoiUser,
		})
	}

	return targets
}

func copyImageSCP(t testing.TB, d dutInfo, imagePath string, fileDestination string) {
	target := fmt.Sprintf("%s:%s", d.sshIp, d.sshPort)
	t.Logf("Copying image to %s (%s) over scp", d.dut, target)
	sshConf := scp.NewSSHConfigFromPassword(d.sshUser, d.sshPass)
	scpClient, err := scp.NewClient(target, sshConf, &scp.ClientOption{})
	if err != nil {
		t.Fatalf("Error initializing scp client: %v", err)
	}
	defer scpClient.Close()

	ticker := time.NewTicker(1 * time.Minute)
	tickerQuit := make(chan bool)

	go func() {
		for {
			select {
			case <-ticker.C:
				t.Logf("Copying image...")
			case <-tickerQuit:
				return
			}
		}
	}()

	defer func() {
		ticker.Stop()
		tickerQuit <- true
	}()

	if err := scpClient.CopyFileToRemote(imagePath, fileDestination, &scp.FileTransferOption{
		Timeout: fileCopyTimeout,
	}); err != nil {
		t.Fatalf("Error copying image to target %s (%s:%s): %v", d.dut, d.sshIp, d.sshPort, err)
	}
}

func sendCLI(t testing.TB, dut *ondatra.DUTDevice, cmd string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sshClient := dut.RawAPIs().CLI(t)
	t.Logf("Running: %s", cmd)
	out, err := sshClient.RunCommand(ctx, cmd)
	return out.Output(), err
}
