package optics_insertion_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/povsister/scp"
	"google.golang.org/protobuf/encoding/prototext"
)

const (
	scpDstOnDut   = "/tmp/optics_files"
	sshCmdTimeout = 30 * time.Second
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

func TestOpticsModuleInsertion(t *testing.T) {
	for _, d := range parseBindingFile(t) {
		transferFiles(t, &d, "optics_files", scpDstOnDut)

		dut := ondatra.DUT(t, d.dut)
		lcs := getLineCards(t, dut)

		cmds := []string{
			"run cp " + scpDstOnDut + "/* /usr/local/etc/",
		}

		for _, lc := range lcs {
			slot := strings.Split(lc, "/")[1]
			cmds = append(cmds, "run scp "+scpDstOnDut+"/* 173.0."+slot+".1:/usr/local/etc/")
		}

		for _, c := range cmds {
			time.Sleep(3 * time.Second)
			if resp, err := sendCLI(t, dut, c); err != nil {
				t.Fatalf("Error running command %v, on dut %v", c, d.dut)
			} else {
				t.Logf("%v", resp)
			}
		}
	}
}

func getLineCards(t *testing.T, dut *ondatra.DUTDevice) []string {
	lcs := components.FindComponentsByType(t, dut,
		oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD)
	t.Logf("Found linecard list: %v", lcs)

	var validCards []string
	for _, lc := range lcs {
		empty, ok := gnmi.Lookup(t, dut, gnmi.OC().Component(lc).Empty().State()).Val()
		if !ok || (ok && !empty) {
			validCards = append(validCards, lc)
		}
	}

	return validCards
}
func transferFiles(t testing.TB, d *targetInfo, srcDir string, dstDir string) {
	target := fmt.Sprintf("%s:%s", d.sshIp, d.sshPort)
	t.Logf("Copying files to %s (%s) over scp", d.dut, target)
	sshConf := scp.NewSSHConfigFromPassword(d.sshUser, d.sshPass)
	scpClient, err := scp.NewClient(target, sshConf, &scp.ClientOption{})
	if err != nil {
		t.Fatalf("Error initializing scp client: %v", err)
	}
	defer scpClient.Close()

	if err := scpClient.CopyDirToRemote(srcDir, dstDir, &scp.DirTransferOption{
		Timeout: time.Minute,
	}); err != nil {
		t.Fatalf("Error transfering files to target %s (%s:%s): %v", d.dut, d.sshIp, d.sshPort, err)
	}
}

func sendCLI(t testing.TB, dut *ondatra.DUTDevice, cmd string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), sshCmdTimeout)
	defer cancel()
	sshClient := dut.RawAPIs().CLI(t)
	res, err := sshClient.RunCommand(ctx, cmd)
	return res.Output(), err
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
