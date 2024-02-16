package certgen_test

import (
	"context"
	"crypto/x509"
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

	caKeyFileName  = "ca.key.pem"
	caCertFileName = "ca.cert.pem"
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

func getCACertPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if strings.Contains(cwd, "/featureprofiles/") {
		rootSrc := strings.Split(cwd, "featureprofiles")[0]
		return rootSrc + "featureprofiles/internal/cisco/security/cert/keys/CA/", nil
	}
	return "", fmt.Errorf("ca_cert_path need to be passed as arg")
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

		t.Logf("IP: %v", d.gnmiIp)
		t.Logf("User: %v", d.gnmiUser)

		certTemp, err := certUtil.PopulateCertTemplate(d.gnmiUser, []string{d.gnmiUser}, []net.IP{net.ParseIP(d.gnmiIp)}, d.gnmiUser, 100)
		if err != nil {
			panic(fmt.Sprintf("Could not generate template: %v", err))
		}
		dir, err := getCACertPath()
		if err != nil {
			t.Fatalf(fmt.Sprintf("Could not find a path for ca key/cert: %v", err))
		}
		caKey, caCert, err := certUtil.LoadKeyPair(path.Join(dir, caKeyFileName), path.Join(dir, caCertFileName))
		if err != nil {
			panic(fmt.Sprintf("Could not load ca key/cert: %v", err))
		}
		tlsCert, err := certUtil.GenerateCert(certTemp, caCert, caKey, x509.RSA)
		if err != nil {
			panic(fmt.Sprintf("Could not generate ca cert/key: %v", err))
		}
		err = certUtil.SaveTLSCertInPems(tlsCert, path.Join(dutOutDir, "ems.key.pem"), path.Join(dutOutDir, "ems.cert.pem"), x509.RSA)
		if err != nil {
			panic(fmt.Sprintf("Could not save cleint cert/key in pem files: %v", err))
		}

		copyFile(t, path.Join(dir, caCertFileName), path.Join(dutOutDir, caCertFileName))
		transferKeys(t, &d, dutOutDir)

		copyKeysCmds := []string{
			"run cp " + path.Join(keysDestination, "ems.cert.pem") + " /misc/config/grpc/ems.pem",
			"run cp " + path.Join(keysDestination, "ems.key.pem") + " /misc/config/grpc/ems.key",
			"run cp " + path.Join(keysDestination, caCertFileName) + " /misc/config/grpc/ca.cert",
		}

		dut := ondatra.DUT(t, d.dut)
		for _, c := range copyKeysCmds {
			time.Sleep(3 * time.Second)
			if resp, err := sendCLI(t, dut, c); err != nil {
				t.Fatalf("Error running command %v, on dut %v", c, d.dut)
			} else {
				t.Logf("%v", resp)
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
