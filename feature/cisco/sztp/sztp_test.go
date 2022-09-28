package sztp_test

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"crypto/tls"
	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/ondatra"
	scp "github.com/povsister/scp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	client_ssh_dir = fmt.Sprintf("%s/.ssh/", os.Getenv("HOME"))
	remote_dir     = fmt.Sprintf("%s/", os.Getenv("HOME"))
	client_ca_dir  = client_ssh_dir
	hostname, _    = os.Hostname()
	ztp_timeout    = 10 * time.Minute
)
var (
	sshIP   = flag.String("ssh_ip", "", "External IP address of management interface.")
	sshPort = flag.String("ssh_port", "", "External Port of management interface")
	sshUser = flag.String("ssh_user", "", "External username for ssh")
	sshPass = flag.String("ssh_pass", "", "External password for ssh")
)


// generates an rsa key pair in client_ssh_dir
func generateKeypair(client_ssh_dir string) error {
	cmd := exec.Command("bash", "-c", fmt.Sprintf("ssh-keygen -t rsa -b 1024 -f %sid_rsa -N '' <<< y", client_ssh_dir))
	err := cmd.Run()
	if err != nil {
		return err
	}
	publicKeyBytes, err := ioutil.ReadFile(fmt.Sprintf("%sid_rsa.pub", client_ssh_dir))
	if err != nil {
		return err
	}
	publicKey := strings.Split(string(publicKeyBytes), " ")[1]
	rawDecodedKey, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(fmt.Sprintf("%sid_rsa.bin", client_ssh_dir), []byte(rawDecodedKey), 0600)
	if err != nil {
		return err
	}
	return nil
}

// scp using an existing established SSH connection
func TestPWLess(t *testing.T) {
	if *sshIP == "" {
		t.Fatal("--ssh_ip flag must be set.")
	}

	t.Log("generating rsa key pair... \n\n")
	err := generateKeypair(client_ssh_dir)
	if err != nil {
		t.Error(err)
	}
	t.Log("Connecting to box\n\n ")
	sshConf := scp.NewSSHConfigFromPassword(*sshUser, *sshPass)
	scpClient, _ := scp.NewClient(fmt.Sprintf("%s:%s", *sshIP, *sshPort), sshConf, &scp.ClientOption{})
	defer scpClient.Close()
	t.Log("Copying the file to harddisk:\n\n")

	err = scpClient.CopyFileToRemote(fmt.Sprintf("%sid_rsa.bin", client_ssh_dir), "/harddisk:/id_rsa.bin", &scp.FileTransferOption{})
	if err != nil {
		t.Error(err)
	}
	dut := ondatra.DUT(t, "dut")
	cli_handle := dut.RawAPIs().CLI(t)
	resp, err := cli_handle.SendCommand(context.Background(), "crypto key import authentication rsa harddisk:/id_rsa.bin")
	t.Logf(resp)
	if err != nil {
		t.Error(err)
	}
	t.Log("Trying to connect to box with public key\n\n")
	ssh_pwless := "ssh -i id_rsa " + fmt.Sprintf("%s@%s -p %s", *sshUser, *sshIP, *sshPort) + " show version"
	t.Log(ssh_pwless)
	outPw, errPw := exec.Command("bash", "-c", ssh_pwless).Output()
	if errPw != nil {
		t.Error(errPw)
	}
	t.Logf("show version from the box\n %v\n", outPw)
}
func TestCertAuth(t *testing.T) {
	if *sshIP == "" {
		t.Fatal("--ssh_ip flag must be set.")
	}

	t.Log("Cert based authentication\n")
	ca_server := fmt.Sprintf("ssh-keygen -t rsa -b 1024 -f %sclient_ca -N '' <<< y", client_ca_dir)
	errSer := exec.Command("bash", "-c", ca_server).Run()
	if errSer != nil {
		t.Error(errSer)
	}
	ca_client := fmt.Sprintf("ssh-keygen -s %sclient_ca -I '%s' -V '+1d' %sid_rsa.pub", client_ca_dir, hostname, client_ssh_dir)
	errCli := exec.Command("bash", "-c", ca_client).Run()
	if errCli != nil {
		t.Error(errCli)
	}
	ssh_cert := fmt.Sprintf("ssh -o CertificateFile=%sclient_ca-cert.pub -i %sid_rsa %s@%s -p %s show version", client_ca_dir, client_ssh_dir, *sshUser, *sshIP, *sshPort)
	outCert, errCert := exec.Command("bash", "-c", ssh_cert).Output()
	if errCert != nil {
		t.Error(errCert)
	}
	t.Logf("The output is %v\n", outCert)
}

func TestPwDisable(t *testing.T) {
	if *sshIP == "" {
		t.Fatal("--ssh_ip flag must be set.")
	}

	ssh_pwauth := "ssh -o PreferredAuthentications=cisco123 " + fmt.Sprintf("%s@%s -p %s", *sshUser, *sshIP, *sshPort)
	outPwauth, errPwauth := exec.Command("bash", "-c", ssh_pwauth).Output()
	if errPwauth == nil {
		t.Error(errPwauth)
	}
	t.Logf("The output is %v\n", outPwauth)
}

func TestDiskEn(t *testing.T) {
	if *sshIP == "" {
		t.Fatal("--ssh_ip flag must be set.")
	}

	dut := ondatra.DUT(t, "dut")
	cli_handle := dut.RawAPIs().CLI(t)
	resp, err := cli_handle.SendCommand(context.Background(), "show disk-encryption status")
	if err != nil {
		t.Error(err)
	}
	t.Log(resp)
	if strings.Contains(resp, "Not Encrypted") {
		resp, err = cli_handle.SendCommand(context.Background(), "disk-encryption activate")
		if err != nil {
			t.Error(err)
		}
		t.Logf(resp)
		t.Log("Waiting for the box to reload")
		time.Sleep(8 * time.Minute)
		t.Log("Executing disk-encryption after reload")
		dut1 := ondatra.DUT(t, "dut")
		cli_handle1 := dut1.RawAPIs().CLI(t)
		resp, err = cli_handle1.SendCommand(context.Background(), "show disk-encryption status")
		if err != nil {
			t.Error(err)
		}
		t.Log(resp)
		if strings.Contains(resp, "Not Encrypted") {
			t.Error("Disk encryption failed")
		}

	}

}

func TestTLS(t *testing.T) {
	if *sshIP == "" {
		t.Fatal("--ssh_ip flag must be set.")
	}
	dut := ondatra.DUT(t, "dut")
	cli_handle := dut.RawAPIs().CLI(t)
	t.Log("configuring grpc tls to generate the certificates/key")
	config.TextWithSSH(context.Background(), t, dut, "configure \n no grpc no-tls\n commit \n", 10*time.Second)
	rmGrpc, errRmGrpc := cli_handle.SendCommand(context.Background(), "run cp /misc/config/grpc/ems.pem /harddisk:")
	t.Logf(rmGrpc)
	if errRmGrpc != nil {
		t.Error(errRmGrpc)
	}
	rmGrpc, errRmGrpc = cli_handle.SendCommand(context.Background(), "run cp /misc/config/grpc/ems.key /harddisk:")
	t.Logf(rmGrpc)
	if errRmGrpc != nil {
		t.Error(errRmGrpc)
	}
	sshConf := scp.NewSSHConfigFromPassword(*sshUser, *sshPass)
	scpClient, _ := scp.NewClient(fmt.Sprintf("%s:%s", *sshIP, *sshPort), sshConf, &scp.ClientOption{})
	defer scpClient.Close()
	errGrpcPem := scpClient.CopyFileFromRemote("/harddisk:/ems.pem", remote_dir, &scp.FileTransferOption{})
	if errGrpcPem != nil {
		t.Error(errGrpcPem)
	}
	errGrpcKey := scpClient.CopyFileFromRemote("/harddisk:/ems.key", remote_dir, &scp.FileTransferOption{})
	if errGrpcKey != nil {
		t.Error(errGrpcKey)
	}
	serverCert, err := tls.LoadX509KeyPair(fmt.Sprintf("%s/ems.pem", remote_dir), fmt.Sprintf("%s/ems.key", remote_dir))
	if err != nil {
		t.Error(err)
	}
	configGrpc := &tls.Config{
		Certificates:       []tls.Certificate{serverCert},
		ClientAuth:         tls.NoClientCert,
		InsecureSkipVerify: true,
	}

	tlsCredential := credentials.NewTLS(configGrpc)
	conn, err := grpc.Dial(
		fmt.Sprintf("%s:%s", *sshIP, *sshPort),
		grpc.WithTransportCredentials(tlsCredential),
	)
	if err != nil {
		t.Error(err)
	}
	defer conn.Close()
	config.TextWithSSH(context.Background(), t, dut, "configure \n grpc no-tls\n commit \n", 10*time.Second)

}

func TestSZTP(t *testing.T) {
	if *sshIP == "" {
		t.Fatal("--ssh_ip flag must be set.")
	}

	dut := ondatra.DUT(t, "dut")
	cli_handle := dut.RawAPIs().CLI(t)
	ztp_resp, err := cli_handle.SendCommand(context.Background(), "ztp initiate noprompt")
	if err != nil {
		t.Error(err)
	}
	t.Logf("%v\n", ztp_resp)

	t.Logf("Sleep (%v) - allowing ztp and reload to complete\n\n", ztp_timeout)
	time.Sleep(9 * time.Minute)
	dut_new := ondatra.DUT(t, "dut")
	cli_handle_new := dut_new.RawAPIs().CLI(t)

	ztp_logs, err := cli_handle_new.SendCommand(context.Background(), "show ztp log | utility tail -n 60")
	if err != nil {
		t.Error(err)
	}
	t.Logf("%v\n", ztp_logs)
	if strings.Contains(ztp_logs, "ZTP completed successfully") {
	} else {
		err = fmt.Errorf("ZTP Failed")
		t.Error(err)
	}
}
