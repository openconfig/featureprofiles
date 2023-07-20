package sztp_base_test

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"crypto/tls"
	"crypto/x509"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	scp "github.com/povsister/scp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

var (
	client_ssh_dir  = fmt.Sprintf("%s/.ssh/", os.Getenv("HOME"))
	remote_dir      = fmt.Sprintf("%s/", os.Getenv("HOME"))
	client_ca_dir   = client_ssh_dir
	hostname, _     = os.Hostname()
	ztp_timeout     = 10 * time.Minute
	sztpServer      = "dev-mgbl-lnx6"
	dhcpServer      = "dev-mgbl-lnx2"
	sztpServerIP    = "5.38.4.124"
	dhcpServerIP    = "5.38.4.249"
	sjcDhcpServer   = "sj21-pxe-01"
	sjcSztpServer   = "sj21-lnx-03"
	sjcSztpServerIP = "1.1.1.103"
	sjcDhcpServerIP = "1.1.7.6"
	usernameServer  = "root"
	passwordServer  = "Bgl11lab@123"
	sjcDhcpPassword = "C1sco123"
	sjcSztpPassword = "roZes@123"
)
var bootVersion string
var (
	sshIP     = flag.String("ssh_ip", "", "External IP address of management interface.")
	sshPort   = flag.String("ssh_port", "", "External Port of management interface")
	sshUser   = flag.String("ssh_user", "", "External username for ssh")
	sshPass   = flag.String("ssh_pass", "", "External password for ssh")
	imagePath = flag.String("image_path", "", "Provide the image incase of upgrade")
	sjcSetup  = flag.Bool("sjc_setup", false, "To run on sjc setup make the flag true")
)

// generates an rsa key pair in client_ssh_dir
func generateKeypair(client_ssh_dir string) error {
	cmd := exec.Command("bash", "-c", fmt.Sprintf("ssh-keygen -t rsa -b 1024 -f %sid_rsa -N '' <<< y", client_ssh_dir))
	err := cmd.Run()
	if err != nil {
		return err
	}
	publicKeyBytes, err := os.ReadFile(fmt.Sprintf("%sid_rsa.pub", client_ssh_dir))
	if err != nil {
		return err
	}
	publicKey := strings.Split(string(publicKeyBytes), " ")[1]
	rawDecodedKey, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil {
		return err
	}
	err = os.WriteFile(fmt.Sprintf("%sid_rsa.bin", client_ssh_dir), []byte(rawDecodedKey), 0600)
	if err != nil {
		return err
	}
	return nil
}

// scp using an existing established SSH connection
func TestPWLess(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	showresp := config.CMDViaGNMI(context.Background(), t, dut, "show version")
	t.Logf(showresp)
	if strings.Contains(showresp, "VXR") {
		t.Logf("Skipping since platfrom is VXR")
		t.Skip()
	}
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
	cli_handle := dut.RawAPIs().CLI(t)
	resp, err := cli_handle.SendCommand(context.Background(), "crypto key import authentication rsa harddisk:/id_rsa.bin")
	t.Logf(resp)
	if err != nil {
		t.Error(err)
	}
	t.Log("Trying to connect to box with public key\n\n")
	ssh_pwless := "ssh -i " + fmt.Sprintf("%sid_rsa ", client_ssh_dir) + fmt.Sprintf("%s@%s -p %s", *sshUser, *sshIP, *sshPort) + " show version"
	t.Log(ssh_pwless)
	outPw, errPw := exec.Command("bash", "-c", ssh_pwless).Output()
	if errPw != nil {
		t.Error(errPw)
	}
	t.Logf("show version from the box\n %v\n", string(outPw))
}
func TestCertAuth(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	showresp := config.CMDViaGNMI(context.Background(), t, dut, "show version")
	t.Logf(showresp)
	if strings.Contains(showresp, "VXR") {
		t.Logf("Skipping since platfrom is VXR")
		t.Skip()
	}
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
	t.Logf("The output is %v\n", string(outCert))
}

func TestPwDisable(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	showresp := config.CMDViaGNMI(context.Background(), t, dut, "show version")
	t.Logf(showresp)
	if strings.Contains(showresp, "VXR") {
		t.Logf("Skipping since platfrom is VXR")
		t.Skip()
	}
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
	dut := ondatra.DUT(t, "dut")
	showresp := config.CMDViaGNMI(context.Background(), t, dut, "show version")
	t.Logf(showresp)
	if strings.Contains(showresp, "VXR") {
		t.Logf("Skipping since platfrom is VXR")
		t.Skip()
	}
	if *sshIP == "" {
		t.Fatal("--ssh_ip flag must be set.")
	}
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
	dut := ondatra.DUT(t, "dut")
	showresp := config.CMDViaGNMI(context.Background(), t, dut, "show version")
	t.Logf(showresp)
	if strings.Contains(showresp, "VXR") {
		t.Logf("Skipping since platfrom is VXR")
		t.Skip()
	}
	if *sshIP == "" {
		t.Fatal("--ssh_ip flag must be set.")
	}
	//t.Log("configuring grpc tls to generate the certificates/key")
	cli_handle := dut.RawAPIs().CLI(t)
	rmGrpc, errRmGrpc := cli_handle.SendCommand(context.Background(), "run rm -rf /misc/config/grpc/")
	t.Logf(rmGrpc)
	if errRmGrpc != nil {
		t.Error(errRmGrpc)
	}
	config.TextWithSSH(context.Background(), t, dut, "configure \n no grpc \n commit \n", 10*time.Second)
	config.TextWithSSH(context.Background(), t, dut, "configure \n grpc port 57777 \n commit \n", 10*time.Second)
	sshConf := scp.NewSSHConfigFromPassword(*sshUser, *sshPass)
	scpClient, _ := scp.NewClient(fmt.Sprintf("%s:%s", *sshIP, *sshPort), sshConf, &scp.ClientOption{})
	defer scpClient.Close()
	errGrpcPem := scpClient.CopyFileFromRemote("/misc/config/grpc/ems.pem", remote_dir, &scp.FileTransferOption{})
	if errGrpcPem != nil {
		t.Error(errGrpcPem)
	}
	fmt.Printf("%vems.pem", remote_dir)
	rootPEM, err := os.ReadFile(fmt.Sprintf("%sems.pem", remote_dir))
	if err != nil {
		t.Error(err)
	}
	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM([]byte(rootPEM))
	if !ok {
		t.Error(ok)
	}
	configGrpc := &tls.Config{
		RootCAs:            roots,
		InsecureSkipVerify: true,
	}
	t.Log("Connecting to grpc")
	ctx := metadata.AppendToOutgoingContext(context.Background(), "username", *sshUser, "password", *sshPass)
	tlsCredential := credentials.NewTLS(configGrpc)
	conn, err := grpc.DialContext(ctx,
		fmt.Sprintf("%s:%s", *sshIP, "7001"),
		grpc.WithTransportCredentials(tlsCredential),
	)
	if err != nil {
		t.Error(err)
	}
	gnmi, err := gpb.NewGNMIClient(conn), nil
	if err != nil {
		t.Error(err)
	}
	gNMI_out, err := gnmi.Get(ctx, &gpb.GetRequest{
		Path: []*gpb.Path{{
			Elem: []*gpb.PathElem{
				{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
		},
		Type:     gpb.GetRequest_CONFIG,
		Encoding: gpb.Encoding_JSON_IETF,
	})
	t.Logf("Gnmi Response using TLS:\n%v", gNMI_out)
	if err != nil {
		t.Error(err)

	}

	defer conn.Close()

	defer config.TextWithSSH(context.Background(), t, dut, "configure \n grpc no-tls\n commit \n", 10*time.Second)

}

func TestSZTP(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Connect to dhcp server")
	showresp := config.CMDViaGNMI(context.Background(), t, dut, "show version")
	t.Logf(showresp)
	if strings.Contains(showresp, "VXR") {
		t.Logf("Skipping since platfrom is VXR")
		t.Skip()
	}
	remove_known_hosts(t)
	dhcpServerConn := connect_remote_server(t, usernameServer, passwordServer, dhcpServer)
	sztpServerConn := connect_remote_server(t, usernameServer, passwordServer, sztpServer)
	defer dhcpServerConn.Close()
	defer sztpServerConn.Close()

	t.Logf("Take a backup of the existing dhcpd.conf")
	create_backup_dhcpd_file(t, dhcpServerConn)

	t.Logf("Create dhcp entry and start dhcp service")
	create_dhcpd_setup(t, dhcpServerConn, sztpServerIP, dut, false)

	t.Logf("Revert the dhcpd.conf back ")
	defer restore_backup_dhcpd_file(t, dhcpServerConn, backupFile)
	t.Run("ZTP initiate dhcp4 management noprompt", func(t *testing.T) {
		t.Logf("Version Check ")
		version_check(t, dut, sztpServerConn, dhcpServerConn, false, false)

		t.Log("Start sztp server ")
		start_sztp_server(t, sztpServerConn)
		defer stop_sztp_server(t, sztpServerConn)

		t.Log("Initiate ztp and check for ztp logs")
		ztp_initiate(t, dut)
	})
	t.Run("Image upgrade/downgrade ", func(t *testing.T) {
		if *imagePath != "" {
			t.Logf("Version Check ")
			version_check(t, dut, sztpServerConn, dhcpServerConn, true, false)

			t.Log("Start sztp server ")
			start_sztp_server(t, sztpServerConn)
			defer stop_sztp_server(t, sztpServerConn)

			t.Log("Initiate ztp and check for ztp logs")
			ztp_initiate(t, dut)
		}
	})
}

func TestBootz(t *testing.T) {
	remove_known_hosts(t)
	dut := ondatra.DUT(t, "dut")
	dhcpPassword := passwordServer
	sztpPassword := passwordServer
	if *sjcSetup == true {
		dhcpServer = sjcDhcpServer
		sztpServer = sjcSztpServer
		dhcpPassword = sjcDhcpPassword
		sztpPassword = sjcSztpPassword
	}
	// t.Logf("Connect to dhcp server")
	dhcpServerConn := connect_remote_server(t, usernameServer, dhcpPassword, dhcpServer)
	t.Logf("Connect to bootz server")
	bootzServerConn := connect_remote_server(t, usernameServer, sztpPassword, sztpServer)

	defer dhcpServerConn.Close()
	defer bootzServerConn.Close()

	// t.Logf("Take a backup of the existing dhcpd.conf")
	// create_backup_dhcpd_file(t, dhcpServerConn)

	// t.Logf("Create dhcp entry and start dhcp service")
	// create_dhcpd_setup(t, dhcpServerConn, dhcpServerIP, dut, true)

	// t.Logf("Revert the dhcpd.conf back ")
	// defer restore_backup_dhcpd_file(t, dhcpServerConn, backupFile)

	t.Run("ZTP initiate dhcp4 management noprompt without disk-encryption", func(t *testing.T) {
		t.Log("Deactivate Encryption\n")
		encrypt, err := dut.RawAPIs().CLI(t).SendCommand(context.Background(), "disk-encryption deactivate location all")
		t.Logf("Sleep for 5 mins after disk-encryption deactivate")
		time.Sleep(3 * time.Minute)
		if err != nil {
			t.Fatalf("Failed to send command %v on the device, Error : %v ", "disk-encryption activate location all", err)

		}
		t.Logf("Device encryption deactivate: %v", encrypt)
		deviceBootStatus(t, dut)
		encrypt, err = dut.RawAPIs().CLI(t).SendCommand(context.Background(), "show disk-encryption status")
		if err != nil {
			t.Fatalf("Failed to send command %v on the router, Error : %v ", "show disk-encryption status", err)

		}
		t.Logf("Show device encryption status: %v", encrypt)
		t.Logf("Version Check ")
		version_check(t, dut, bootzServerConn, dhcpServerConn, false, true)

		start_bootz_server(t, bootzServerConn, dut)
		defer stop_bootz_server(t, bootzServerConn)
		t.Logf("Version %v ", bootVersion)
		ztp_initiate(t, dut)
		dutNew := ondatra.DUT(t, "dut")
		verify_bootz(t, dutNew)
	})
	t.Run("ZTP initiate dhcp4 management noprompt with disk-encryption", func(t *testing.T) {
		t.Log("Activate Encryption\n")
		encrypt, err := dut.RawAPIs().CLI(t).SendCommand(context.Background(), "disk-encryption activate location all")
		t.Logf("Sleep for 5 mins after disk-encryption activate")
		time.Sleep(5 * time.Minute)
		if err != nil {
			t.Fatalf("Failed to send command %v on the device, Error : %v ", "disk-encryption activate location all", err)

		}
		t.Logf("Device encryption activate: %v", encrypt)
		deviceBootStatus(t, dut)
		encrypt, err = dut.RawAPIs().CLI(t).SendCommand(context.Background(), "show disk-encryption status")
		if err != nil {
			t.Fatalf("Failed to send command %v on the router, Error : %v ", "show disk-encryption status", err)

		}
		t.Logf("Show device encryption status: %v", encrypt)
		t.Logf("Version Check ")
		version_check(t, dut, bootzServerConn, dhcpServerConn, false, true)
		start_bootz_server(t, bootzServerConn, dut)
		defer stop_bootz_server(t, bootzServerConn)
		t.Logf("Version %v ", bootVersion)
		ztp_initiate(t, dut)
		remove_known_hosts(t)
		dutNew := ondatra.DUT(t, "dut")
		verify_bootz(t, dutNew)
	})
	t.Run("ZTP initiate dhcp4 management noprompt with RP Switchover", func(t *testing.T) {
		if *sjcSetup == true {
			t.Logf("Version Check ")
			rpSwitchOver(t, dut)
			version_check(t, dut, bootzServerConn, dhcpServerConn, false, true)

			start_bootz_server(t, bootzServerConn, dut)
			defer stop_bootz_server(t, bootzServerConn)
			t.Logf("Version %v ", bootVersion)
			ztp_initiate(t, dut)
			remove_known_hosts(t)
			dutNew := ondatra.DUT(t, "dut")

			verify_bootz(t, dutNew)
		}
	})
	t.Run("ZTP initiate dhcp4 management noprompt with image upgrade", func(t *testing.T) {
		t.Logf("Version Check ")
		version_check(t, dut, bootzServerConn, dhcpServerConn, true, true)

		start_bootz_server(t, bootzServerConn, dut)
		defer stop_bootz_server(t, bootzServerConn)
		t.Logf("Version %v ", bootVersion)
		ztp_initiate(t, dut)
		remove_known_hosts(t)
		dutNew := ondatra.DUT(t, "dut")

		verify_bootz(t, dutNew)
		bootVersionAfterUpgrade := versionOnBox(t, dutNew)
		t.Logf("Version on the box %v ", bootVersionAfterUpgrade)
		if bootVersionAfterUpgrade != imageUpgradeVersionRequested {
			t.Errorf("Image Upgrade Unsuccessfull")
		}
	})

	/*
		TODO:
		1. Missing configuration : pass an empty vendor config, /root/bootz/sahubbal/bootz_server_exec/configuration/server/vendorconfig/base.cfg
		2. Invalid configuration : pass an invalid config, eg: miss ! at the bigining of the base.cfg
		3. Invalid software image : pass on iso of a different platform
		4. No ownership voucher	: pass an empty voucher, vouchers are stored here- /root/bootz/sahubbal/bootz_server_exec/certificates/vouchers/
		5. Invalid OV : pass a junk ov or a pass an ov with different serial number
		6. no OS provided : pass an empty json here - /root/bootz/sahubbal/bootz_server_exec/configuration/image/image_cfg_input.json
		7. failed to fetch image from remote URL : pass on a differnt http ip so that fecthing fails
		8. OS checksum doesn't match : pass on wrong hash value in image_cfg_input.json

	*/
}
