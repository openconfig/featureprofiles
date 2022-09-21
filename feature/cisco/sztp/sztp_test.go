package sztp_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	//	"crypto/tls"

	"github.com/openconfig/ondatra"
	scp "github.com/povsister/scp"
	// "google.golang.org/grpc"
	// "google.golang.org/grpc/credentials"
)

var (
	client_ssh_dir = fmt.Sprintf("%s/.ssh/", os.Getenv("HOME"))

	remote_x_dir  = "/harddisk:/"
	client_ca_dir = client_ssh_dir
	hostname, _   = os.Hostname()
	ztp_timeout   = 10 * time.Minute
)

type pxe struct {
	Host     string
	User     string
	Password string
	Port     int
}

var pxe_root = pxe{"172.26.228.26", "cisco", "cisco123", 60462}

// generates an rsa key pair in client_ssh_dir
func generateKeypair(client_ssh_dir string) error {
	fmt.Printf(client_ssh_dir)
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
	fmt.Printf("generating rsa key pair... \n\n")
	err := generateKeypair(client_ssh_dir)
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("Connecting to box\n\n ")
	sshConf := scp.NewSSHConfigFromPassword("cafyauto", "cisco123")
	scpClient, _ := scp.NewClient("173.39.51.67:57778", sshConf, &scp.ClientOption{})
	defer scpClient.Close()
	fmt.Printf("Copying the file to harddisk:\n\n")

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
	fmt.Printf("Trying to connect to box with public key\n\n")
	ssh_pwless := "ssh -i id_rsa cafyauto@173.39.51.67 -p 57778 show version"
	outPw, errPw := exec.Command("bash", "-c", ssh_pwless).Output()
	if errPw != nil {
		t.Error(errPw)
	}
	fmt.Printf("show version from the box\n %s\n", outPw)
}
func TestCertAuth(t *testing.T) {
	fmt.Printf("Cert based authentication\n")
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
	ssh_cert := fmt.Sprintf("ssh -o CertificateFile=%sclient_ca-cert.pub -i %sid_rsa cafyauto@173.39.51.67 -p 57778 show version", client_ca_dir, client_ssh_dir)
	outCert, errCert := exec.Command("bash", "-c", ssh_cert).Output()
	if errCert != nil {
		t.Error(errCert)
	}
	fmt.Printf("The output is %s\n", outCert)
}

func TestPwDisable(t *testing.T) {
	ssh_pwauth := "ssh -o PreferredAuthentications=cisco123 cafyauto@173.39.51.67 -p 57778"
	outPwauth, errPwauth := exec.Command("bash", "-c", ssh_pwauth).Output()
	if errPwauth == nil {
		t.Error(errPwauth)
	}
	fmt.Printf("The output is %s\n", outPwauth)
}
func TestDiskEn(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	cli_handle := dut.RawAPIs().CLI(t)
	resp, err := cli_handle.SendCommand(context.Background(), "show disk-encryption status")
	if err != nil {
		t.Error(err)
	}
	fmt.Println(resp)
	if strings.Contains(resp, "Not Encrypted") {
		resp, err = cli_handle.SendCommand(context.Background(), "disk-encryption activate")
		if err != nil {
			t.Error(err)
		}
		fmt.Println("Waiting for the box to reload")
		time.Sleep(8 * time.Minute)
		fmt.Println("Executing disk-encryption after reload")
		dut1 := ondatra.DUT(t, "dut")
		cli_handle1 := dut1.RawAPIs().CLI(t)
		resp, err = cli_handle1.SendCommand(context.Background(), "show disk-encryption status")
		if err != nil {
			t.Error(err)
		}
		fmt.Println(resp)
		if strings.Contains(resp, "Not Encrypted") {
			t.Error("Disk encryption failed")
		}

	}

}

func TestSZTP(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	cli_handle := dut.RawAPIs().CLI(t)
	ztp_resp, err := cli_handle.SendCommand(context.Background(), "ztp initiate noprompt")
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("%s\n", ztp_resp)

	fmt.Printf("Sleep (%s) - allowing ztp and reload to complete\n\n", ztp_timeout)
	time.Sleep(9 * time.Minute)

	dut_new := ondatra.DUT(t, "dut")
	cli_handle_new := dut_new.RawAPIs().CLI(t)

	ztp_logs, err := cli_handle_new.SendCommand(context.Background(), "show ztp log | utility tail -n 50")
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("%s\n", ztp_logs)
	if strings.Contains(ztp_logs, "ZTP Exited") {
	} else {
		err = fmt.Errorf("ZTP Failed")
		t.Error(err)
	}
}
