package credz_test

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	log "github.com/golang/glog"

	"github.com/openconfig/featureprofiles/internal/fptest"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	credz "github.com/openconfig/gnsi/credentialz"
	"github.com/openconfig/ondatra/gnmi/oc"
	"google.golang.org/protobuf/encoding/prototext"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"golang.org/x/crypto/ssh"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type AuthenticationError struct {
	message string
}

type User struct {
	Name     string
	SpiffeID string
}

func (e *AuthenticationError) Error() string {
	return e.message
}

func createAuthErr(message string) error {
	return &AuthenticationError{message: message}
}

func createSSHClient(host string, port int, user string, privateKeyPath *string, password *string) error {
	authentication_type := map[string]string{
		"ECDSA_256":       "ecdsa-sha2-nistp256",
		"ECDSA_521":       "ecdsa-sha2-nistp521",
		"ED25519_ED25519": "ssh-ed25519",
		"RSA_2048":        "rsa-pubkey",
		"RSA_4096":        "rsa-pubkey",
		"password":        "password",
	}
	var config ssh.ClientConfig
	var auth_type string

	if password != nil {
		config = ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{
				ssh.Password(*password),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
		auth_type = "password"

	} else {
		privateKeyBytes, err := ioutil.ReadFile(*privateKeyPath)
		if err != nil {
			log.Exit("Error reading private key file: ", err)
		}
		privateKey, err := ssh.ParsePrivateKey(privateKeyBytes)
		if err != nil {
			log.Exit("Error parsing private key: ", err)
		}

		config = ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(privateKey),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
		_, auth_type = filepath.Split(*privateKeyPath)

	}
	address := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", address, &config)
	if err != nil {
		log.Error("Error in establishing SSH connection: ", err)
		return err
	}
	defer client.Close()

	log.Infof("Successfully connected to DUT")

	session, err := client.NewSession()
	if err != nil {
		log.Exit("Error creating session: ", err)
	}
	defer session.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf
	command := fmt.Sprintf("show ssh | inc %s", user)
	err = session.Run(command)
	if err != nil {
		log.Exit("Error running command :", err)
	}
	output := stdoutBuf.String()

	if strings.Contains(output, authentication_type[auth_type]) {
		log.Infof("Authentication type of user(%s) is %s as expected", user, authentication_type[auth_type])
	} else {
		err_msg := fmt.Sprintf("Authentication type of user(%s) is not %s", user, authentication_type[auth_type])
		log.Error(err_msg)
		err := createAuthErr(err_msg)
		if err != nil {
			return err
		}
	}

	log.Infof("Command output for %s", output)
	log.Infof("======================================")
	return nil
}

func genereate_pvt_public_key_pairs(file_path string, encryption_type string, bytes_for_encrypt string) error {

	var cmd *exec.Cmd
	if bytes_for_encrypt == "" {
		cmd = exec.Command("ssh-keygen", "-t", encryption_type, "-f", file_path, "-N", "")
	} else {
		cmd = exec.Command("ssh-keygen", "-t", encryption_type, "-b", bytes_for_encrypt, "-f", file_path, "-N", "")
	}
	err := cmd.Run()
	if err != nil {
		return err
	}
	log.Infof("Generated public/private %v keypair and saved as %v", encryption_type, file_path)
	return nil
}

func create_account_credentials(file_path string, account_name string) (credz.AccountCredentials, []string) {
	credentials_data := credz.AccountCredentials{
		Account:   account_name,
		Version:   "1.1",
		CreatedOn: 123}

	var optKey credz.Option
	var optKey2 credz.Option
	var opt_array []*credz.Option
	optKey.Key = &credz.Option_Id{Id: 16}
	optKey2.Key = &credz.Option_Id{Id: 3}
	optKey2.Value = fmt.Sprintf("show ssh | inc %s", account_name)
	opt_array = append(opt_array, &optKey)
	opt_array = append(opt_array, &optKey2)
	var pvt_keys_names []string

	for i := 1; i <= 5; i++ {
		keyTypeName := credz.KeyType_name[int32(i)]
		temp_list := strings.Split(string(keyTypeName), "_")
		bytes_for_keygen := temp_list[len(temp_list)-1]
		encryption := temp_list[2]
		file_name := fmt.Sprintf("%s/%s_%s", file_path, encryption, bytes_for_keygen)
		_, err := strconv.Atoi(bytes_for_keygen)
		pvt_keys_names = append(pvt_keys_names, file_name)
		if err == nil {
			genereate_pvt_public_key_pairs(file_name, encryption, bytes_for_keygen)
		} else {
			genereate_pvt_public_key_pairs(file_name, encryption, "")
		}
		publicKeyBytes, _ := os.ReadFile(fmt.Sprintf("%s.pub", file_name))
		pub_key := strings.Split(string(publicKeyBytes), " ")[1]

		keys_data := credz.AccountCredentials_AuthorizedKey{
			AuthorizedKey: []byte(pub_key),
			KeyType:       credz.KeyType(i),
			Options:       opt_array,
			Description:   fmt.Sprintf("%s pub key", temp_list[2])}
		credentials_data.AuthorizedKeys = append(credentials_data.AuthorizedKeys, &keys_data)
	}
	log.Infof("AccountCredentials:", credentials_data)

	return credentials_data, pvt_keys_names
}

func roatateAccountCredentialsRequest(stream credz.Credentialz_RotateAccountCredentialsClient, akreq credz.AuthorizedKeysRequest) {

	log.Infof("Send Rotate Account Credentisls Request ")
	err := stream.Send(&credz.RotateAccountCredentialsRequest{Request: &credz.RotateAccountCredentialsRequest_Credential{Credential: &akreq}})
	if err != nil {
		log.Exit("Credz:  Stream send returned error: " + err.Error())
	}
	gotRes, err := stream.Recv()
	if err != nil {
		log.Exit("Credz:  Stream receive returned error: " + err.Error())
	}
	aares := gotRes.GetCredential()
	if aares == nil {
		log.Infof("Authorized keys response is nil")
	}
	log.Infof("Rotate Account Credentials Request done")
}

func finalizeAccountRequest(stream credz.Credentialz_RotateAccountCredentialsClient) {
	log.Infof("Send Finalize Request")
	err := stream.Send(&credz.RotateAccountCredentialsRequest{Request: &credz.RotateAccountCredentialsRequest_Finalize{}})
	if err != nil {
		log.Exit("Stream send finalize failed : " + err.Error())
	}
	log.Infof("Finalize Request done")
}

func createUsersOnDevice(t *testing.T, dut *ondatra.DUTDevice, users []*oc.System_Aaa_Authentication_User) {
	ocAuthentication := &oc.System_Aaa_Authentication{}
	for _, user := range users {
		ocAuthentication.AppendUser(user)
	}
	gnmi.Update(t, dut, gnmi.OC().System().Aaa().Authentication().Config(), ocAuthentication)
}

func finalizeHostRequest(stream credz.Credentialz_RotateHostParametersClient) {
	log.Infof("Credz: Host Parameters Finalize")
	err := stream.Send(&credz.RotateHostParametersRequest{Request: &credz.RotateHostParametersRequest_Finalize{}})
	if err != nil {
		fmt.Println("Credz:  Stream send finalize failed : " + err.Error())
	}
}

func TestPubKeyAuthentication(t *testing.T) {
	file_path := "credz"
	account_name := "CREDZ"

	bindingFile := flag.Lookup("binding").Value.String()
	in, err := os.ReadFile(bindingFile)
	if err != nil {
		t.Fatalf("unable to read binding file")
	}

	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		t.Fatalf("unable to parse binding file")
	}
	target := b.Duts[0].Ssh.Target
	target_ip := strings.Split(target, ":")[0]
	target_port, _ := strconv.Atoi(strings.Split(target, ":")[1])

	var akreq credz.AuthorizedKeysRequest
	dut := ondatra.DUT(t, "dut")

	var users []*oc.System_Aaa_Authentication_User
	users = append(users, &oc.System_Aaa_Authentication_User{Username: &account_name,
		Role: oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN,
	})

	createUsersOnDevice(t, dut, users)

	gnsiC := dut.RawAPIs().GNSI(t)

	mkdir_err := os.Mkdir(file_path, 0755)
	defer os.RemoveAll(file_path)
	if mkdir_err != nil {
		t.Fatalf("Error creating directory: %v", mkdir_err)
		return
	}
	credentials_data, pvt_key_names := create_account_credentials(file_path, account_name)
	akreq.Credentials = append(akreq.Credentials, &credentials_data)

	stream, err := gnsiC.Credentialz().RotateAccountCredentials(context.Background())
	if err != nil {
		t.Fatalf("failed to get stream: %v", err)
	}

	roatateAccountCredentialsRequest(stream, akreq)

	finalizeAccountRequest(stream)

	var err_count int
	err_count = 0
	for _, pvt_file := range pvt_key_names {
		err = createSSHClient(target_ip, target_port, credentials_data.Account, &pvt_file, nil)
		if err != nil {
			err_count = err_count + 1
		}
	}
	if err_count != 0 {
		log.Exit("Error in establishing SSH connection")
	}

	var akreq_for_neg credz.AuthorizedKeysRequest
	credentials := credz.AccountCredentials{
		Account:   account_name,
		Version:   "1.1",
		CreatedOn: 123}
	akreq_for_neg.Credentials = append(akreq_for_neg.Credentials, &credentials)

	log.Infof("Removing all authorized keys from user")
	roatateAccountCredentialsRequest(stream, akreq_for_neg)

	finalizeAccountRequest(stream)
	err_count = 0
	for _, pvt_file := range pvt_key_names {
		err = createSSHClient(target_ip, target_port, credentials_data.Account, &pvt_file, nil)
		if err != nil {
			err_count = err_count + 1
		}
	}
	if err_count == 0 {
		log.Exit("Establishing SSH connection which is not expected")
	} else {
		log.Infof("Error occurred while attempting to establish an SSH connection as expected after removing authorized keys from the user.")
	}

}

func rotateHostParametersRequest(stream credz.Credentialz_RotateHostParametersClient, aareq credz.AllowedAuthenticationRequest) {
	log.Info("Credentialz: Allowed Authentication request")

	err := stream.Send(&credz.RotateHostParametersRequest{Request: &credz.RotateHostParametersRequest_AuthenticationAllowed{AuthenticationAllowed: &aareq}})
	if err != nil {
		log.Exit("Credz:  Stream send returned error: " + err.Error())
	}

	gotRes, err := stream.Recv()
	if err != nil {
		log.Exit("Credz:  Stream receive returned error: " + err.Error())
	}
	aares := gotRes.GetAuthenticationAllowed()
	if aares == nil {
		log.Exit("Credentialz response is nil")
	}
}

func TestSshPasswordLoginDisallowed(t *testing.T) {
	file_path := "credz"
	account_name := "CREDZ"
	account_password := "credz123"

	var users []*oc.System_Aaa_Authentication_User
	users = append(users, &oc.System_Aaa_Authentication_User{
		Username: &account_name,
		Password: &account_password,
		Role:     oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN,
	})

	bindingFile := flag.Lookup("binding").Value.String()
	in, err := os.ReadFile(bindingFile)
	if err != nil {
		t.Fatalf("unable to read binding file")
	}

	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		t.Fatalf("unable to parse binding file")
	}
	target := b.Duts[0].Ssh.Target
	target_ip := strings.Split(target, ":")[0]
	target_port, _ := strconv.Atoi(strings.Split(target, ":")[1])

	dut := ondatra.DUT(t, "dut")
	createUsersOnDevice(t, dut, users)

	gnsiC := dut.RawAPIs().GNSI(t)

	mkdir_err := os.Mkdir(file_path, 0755)
	defer os.RemoveAll(file_path)
	if mkdir_err != nil {
		t.Fatalf("Error creating directory: %v", mkdir_err)
		return
	}

	var akreq credz.AuthorizedKeysRequest

	credentials_data, pvt_key_names := create_account_credentials(file_path, account_name)
	akreq.Credentials = append(akreq.Credentials, &credentials_data)

	stream, err := gnsiC.Credentialz().RotateAccountCredentials(context.Background())
	if err != nil {
		t.Fatalf("failed to get stream: %v", err)
	}
	roatateAccountCredentialsRequest(stream, akreq)
	finalizeAccountRequest(stream)

	err = createSSHClient(target_ip, target_port, credentials_data.Account, nil, &account_password)
	if err != nil {
		log.Exit("Error in establishing SSH connection with password")
	}

	var err_count int
	err_count = 0
	for _, pvt_file := range pvt_key_names {
		err = createSSHClient(target_ip, target_port, credentials_data.Account, &pvt_file, nil)
		if err != nil {
			err_count = err_count + 1
		}
	}
	if err_count != 0 {
		log.Exit("Error in establishing SSH connection with pubkey authentication")
	}

	var auth_type []credz.AuthenticationType
	auth_type = append(auth_type, credz.AuthenticationType_AUTHENTICATION_TYPE_PASSWORD)

	aareq := credz.AllowedAuthenticationRequest{
		AuthenticationTypes: auth_type,
	}

	stream.CloseSend()
	host_param_stream, err := gnsiC.Credentialz().RotateHostParameters(context.Background())
	if err != nil {
		t.Fatalf("failed to get stream: %v", err)
	}
	log.Infof("Aollowed Authentication request type Password")
	rotateHostParametersRequest(host_param_stream, aareq)
	finalizeHostRequest(host_param_stream)
	time.Sleep(2 * time.Second)
	err = createSSHClient(target_ip, target_port, credentials_data.Account, nil, &account_password)
	if err != nil {
		log.Exit("Error in establishing SSH connection with password")
	}

	err_count = 0
	for _, pvt_file := range pvt_key_names {
		err = createSSHClient(target_ip, target_port, credentials_data.Account, &pvt_file, nil)
		if err != nil {
			err_count = err_count + 1
			log.Error("Error in establishing SSH connection which is expected")

		}
	}
	if err_count == 0 {
		log.Exit("Error in establishing SSH connection with pubkey authentication")
	}

	var auth_type_pubkey []credz.AuthenticationType
	auth_type_pubkey = append(auth_type_pubkey, credz.AuthenticationType_AUTHENTICATION_TYPE_PUBKEY)

	aareq = credz.AllowedAuthenticationRequest{
		AuthenticationTypes: auth_type_pubkey,
	}
	log.Infof("Aollowed Authentication request type PubKey")
	rotateHostParametersRequest(host_param_stream, aareq)
	finalizeHostRequest(host_param_stream)
	time.Sleep(2 * time.Second)

	err = createSSHClient(target_ip, target_port, credentials_data.Account, nil, &account_password)
	if err == nil {
		log.Exit("Establishing SSH connection with password which is not expected")
	} else {
		log.Infof("Error in establishing connection with password which is expected")
	}

	err_count = 0
	for _, pvt_file := range pvt_key_names {
		err = createSSHClient(target_ip, target_port, credentials_data.Account, &pvt_file, nil)
		if err != nil {
			err_count = err_count + 1
		}
	}
	if err_count != 0 {
		log.Exit("Error in establishing SSH connection with pubkey authentication")
	}

	var auth_types []credz.AuthenticationType
	auth_types = append(auth_types, credz.AuthenticationType_AUTHENTICATION_TYPE_PASSWORD)
	auth_types = append(auth_types, credz.AuthenticationType_AUTHENTICATION_TYPE_PUBKEY)
	auth_types = append(auth_types, credz.AuthenticationType_AUTHENTICATION_TYPE_KBDINTERACTIVE)

	aareq = credz.AllowedAuthenticationRequest{
		AuthenticationTypes: auth_types,
	}
	log.Infof("Aollowed Authentication request type Password, PubKey and KBDInteractive")
	rotateHostParametersRequest(host_param_stream, aareq)
	finalizeHostRequest(host_param_stream)
}
