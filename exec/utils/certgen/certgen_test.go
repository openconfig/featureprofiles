package certgen_test

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/ondatra"
	"github.com/povsister/scp"
	"google.golang.org/protobuf/encoding/prototext"

	certUtil "github.com/openconfig/featureprofiles/internal/cisco/security/cert"
)

const (
	keysDestination = "/harddisk:/keys"
	sshCmdTimeout   = 30 * time.Second
)

var (
	outDirFlag = flag.String("outDir", "", "Output directory for generated certificates")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type targetInfo struct {
	dut      string
	gnmiIp   string
	gnmiUser string
	sshIp    string
	sshPort  string
	sshUser  string
	sshPass  string
}

func TestCertGen(t *testing.T) {
	outDir := *outDirFlag
	if outDir == "" {
		t.Fatalf("Missing -outDir flag")
	}

	for _, d := range parseBindingFile(t) {
		dutOutDir := path.Join(outDir, d.dut)
		if err := os.MkdirAll(dutOutDir, os.ModePerm); err != nil {
			t.Fatalf("Error creating output directory: %v", err)
		}

		caCertFile, err := certUtil.GetCACertFile()
		if err != nil {
			t.Fatalf("Unable to get CA cert path: %v", err)
		}

		certUtil.GenCERT("ems", 500, []net.IP{net.ParseIP(d.gnmiIp)}, "", dutOutDir)
		certUtil.GenCERT(d.gnmiUser, 100, []net.IP{}, d.gnmiUser, dutOutDir)
		copyFile(t, caCertFile, path.Join(dutOutDir, "ca.cert"))
		transferKeys(t, &d, dutOutDir)

		copyKeysCmds := []string{
			"run cp " + path.Join(keysDestination, "ems.cert.pem") + " /misc/config/grpc/ems.pem",
			"run cp " + path.Join(keysDestination, "ems.key.pem") + " /misc/config/grpc/ems.key",
			"run cp " + path.Join(keysDestination, "ca.cert") + " /misc/config/grpc/ca.cert",
		}

		dut := ondatra.DUT(t, d.dut)
		for _, c := range copyKeysCmds {
			time.Sleep(3 * time.Second)
			if _, err := sendCLI(t, dut, c); err != nil {
				t.Fatalf("Error running command %v, on dut %v", c, d.dut)
			}
		}
	}
}

func transferKeys(t testing.TB, d *targetInfo, keysDir string) {
	target := fmt.Sprintf("%s:%s", d.sshIp, d.sshPort)
	t.Logf("Copying keys to %s (%s) over scp", d.dut, target)
	sshConf := scp.NewSSHConfigFromPassword(d.sshUser, d.sshPass)
	scpClient, err := scp.NewClient(target, sshConf, &scp.ClientOption{})
	if err != nil {
		t.Fatalf("Error initializing scp client: %v", err)
	}
	defer scpClient.Close()

	if err := scpClient.CopyDirToRemote(keysDir, keysDestination, &scp.DirTransferOption{
		Timeout: time.Minute,
	}); err != nil {
		t.Fatalf("Error transfering keys to target %s (%s:%s): %v", d.dut, d.sshIp, d.sshPort, err)
	}
}

func copyFile(t testing.TB, src string, dst string) {
	// Read all content of src to data, may cause OOM for a large file.
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("Error reading file: %v", err)
	}
	err = os.WriteFile(dst, data, 0644)
	if err != nil {
		t.Fatalf("Error writing file: %v", err)
	}
}

func sendCLI(t testing.TB, dut *ondatra.DUTDevice, cmd string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), sshCmdTimeout)
	defer cancel()
	sshClient := dut.RawAPIs().CLI(t)
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

		gnmiUser := dut.Gnmi.Username
		if gnmiUser == "" {
			gnmiUser = dut.Options.Username
		}
		if gnmiUser == "" {
			gnmiUser = b.Options.Username
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

		gnmiTarget := strings.Split(dut.Gnmi.Target, ":")
		gnmiIp := gnmiTarget[0]

		targets = append(targets, targetInfo{
			dut:      dut.Id,
			sshIp:    sshIp,
			sshPort:  sshPort,
			sshUser:  sshUser,
			sshPass:  sshPass,
			gnmiIp:   gnmiIp,
			gnmiUser: gnmiUser,
		})
	}

	return targets
}
