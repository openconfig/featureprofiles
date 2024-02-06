package credz_test

import (
	"bytes"
	"context"
	"flag"
	"fmt"
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

type clientConfig interface {
	createClientConfig() (ssh.ClientConfig, error)
}

type sshPasswordParams struct {
	user     string
	password string
}

func (params sshPasswordParams) createClientConfig() (ssh.ClientConfig, error) {
	config := ssh.ClientConfig{
		User: params.user,
		Auth: []ssh.AuthMethod{
			ssh.Password(params.password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	return config, nil
}

type sshPubkeyParams struct {
	user       string
	privateKey []byte
}

func (params sshPubkeyParams) createClientConfig() (ssh.ClientConfig, error) {
	privateKey, err := ssh.ParsePrivateKey(params.privateKey)
	if err != nil {
		return ssh.ClientConfig{}, err
	}
	config := ssh.ClientConfig{
		User: params.user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(privateKey),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	return config, nil
}

type sshClientParams struct {
	user         string
	clientPvtKey []byte
	clientCert   []byte
}

func (params sshClientParams) createClientConfig() (ssh.ClientConfig, error) {
	signer, err := ssh.ParsePrivateKey(params.clientPvtKey)
	if err != nil {
		return ssh.ClientConfig{}, err
	}
	cert, _, _, _, err := ssh.ParseAuthorizedKey(params.clientCert)
	if err != nil {
		return ssh.ClientConfig{}, err
	}
	certSigner, err := ssh.NewCertSigner(cert.(*ssh.Certificate), signer)
	if err != nil {
		return ssh.ClientConfig{}, err
	}
	config := ssh.ClientConfig{
		User: params.user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(certSigner, signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	return config, nil
}

type sshHostParams struct {
	user              string
	clientPvtKey      []byte
	clientCert        []byte
	hostCert          []byte
	hostKeyAlgorithms []string
}

func (params sshHostParams) createClientConfig() (ssh.ClientConfig, error) {
	signer, err := ssh.ParsePrivateKey(params.clientPvtKey)
	if err != nil {
		return ssh.ClientConfig{}, err
	}
	cert, _, _, _, err := ssh.ParseAuthorizedKey(params.clientCert)
	if err != nil {
		return ssh.ClientConfig{}, err
	}
	certSigner, err := ssh.NewCertSigner(cert.(*ssh.Certificate), signer)
	if err != nil {
		return ssh.ClientConfig{}, err
	}
	key, _, _, _, err := ssh.ParseAuthorizedKey(params.hostCert)
	if err != nil {
		return ssh.ClientConfig{}, err
	}
	config := ssh.ClientConfig{
		User: params.user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(certSigner, signer),
		},
		HostKeyAlgorithms: params.hostKeyAlgorithms,
		HostKeyCallback:   ssh.FixedHostKey(key),
	}
	return config, nil
}

func createSSHClientAndVerify(hostIP string, port int, cC clientConfig, authType string, hostCert *string) error {
	config, err := cC.createClientConfig()
	if err != nil {
		return err
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", hostIP, port), &config)
	if err != nil {
		log.Error(err.Error())
		return err
	}
	defer client.Close()
	log.Infof("Successfully connected to DUT")

	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf
	command := fmt.Sprintf("show ssh | inc %s", config.User)
	err = session.Run(command)
	if err != nil {
		return err
	}
	output := stdoutBuf.String()
	log.Infof(output)
	if strings.Contains(output, authType) {
		log.Infof("Authentication type of user(%s) is %s as expected", config.User, authType)
	} else {
		errMsg := fmt.Sprintf("Authentication type of user(%s) is not %s", config.User, authType)
		err := createAuthErr(errMsg)
		if err != nil {
			return err
		}
	}
	if hostCert != nil {
		if strings.Contains(output, *hostCert) {
			log.Infof("Pubkey of user(%s) is %s as expected", config.User, *hostCert)
		} else {
			errMsg := fmt.Sprintf("Pubkey type of user(%s) is not %s", config.User, *hostCert)
			err := createAuthErr(errMsg)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func genereatePvtPublicKeyPairs(filePath string, encryptionType string, bytesForEncrypt *string) error {

	var cmd *exec.Cmd
	if bytesForEncrypt != nil {
		cmd = exec.Command("ssh-keygen", "-t", encryptionType, "-b", *bytesForEncrypt, "-f", filePath, "-N", "")
	} else {
		cmd = exec.Command("ssh-keygen", "-t", encryptionType, "-f", filePath, "-N", "")
	}
	err := cmd.Run()
	if err != nil {
		return err
	}
	log.Infof("Generated public/private %v keypair and saved as %v", encryptionType, filePath)
	return nil
}

func generateClientCertificates(caPvtKeyName string, authUsrName string, clientPubKeyName string) error {
	clientPubKeyName = clientPubKeyName + ".pub"
	cmd := exec.Command("ssh-keygen", "-s", caPvtKeyName, "-I", "CISCO", "-V", "+1d", "-n", authUsrName, clientPubKeyName)
	err := cmd.Run()
	if err != nil {
		return err
	}
	clientCert := strings.Replace(clientPubKeyName, ".pub", "-cert.pub", -1)
	log.Infof("Generated client cerificate and saved as %v", clientCert)
	openCmd := exec.Command("ssh-keygen", "-Lf", clientCert)
	output, err := openCmd.CombinedOutput()
	if err != nil {
		return err
	}
	log.Infof("Output of %v: %v", clientCert, string(output))
	return nil
}

func generateHostCertificates(caPvtKeyName string, hostPubkeyName string) error {
	hostPubkeyName = hostPubkeyName + ".pub"
	cmd := exec.Command("ssh-keygen", "-s", caPvtKeyName, "-I", "CISCO", "-V", "+1d", "-h", hostPubkeyName)
	err := cmd.Run()
	if err != nil {
		return err
	}
	hostCerrt := strings.Replace(hostPubkeyName, ".pub", "-cert.pub", -1)
	log.Infof("Generated client cerificate and saved as %v", hostCerrt)
	openCmd := exec.Command("ssh-keygen", "-Lf", hostCerrt)
	output, err := openCmd.CombinedOutput()
	if err != nil {
		return err
	}
	log.Infof("Output of %v: %v", hostCerrt, string(output))
	return nil
}

func generateAllSupportedPvtPublicKeyPairs(filePath string, ca bool, host bool) ([]string, []string, []string) {
	var clientPvtKeyNames []string
	var caPvtKeyNames []string
	var hostPvtKeyNames []string
	for i := 1; i <= 5; i++ {
		keyTypeName := credz.KeyType_name[int32(i)]
		tempList := strings.Split(string(keyTypeName), "_")
		bytesForKeygen := tempList[len(tempList)-1]
		encryption := tempList[2]
		fileName := fmt.Sprintf("%s/%s_%s", filePath, encryption, bytesForKeygen)
		caFileName := fmt.Sprintf("%s/ca_%s_%s", filePath, encryption, bytesForKeygen)
		hostFileName := fmt.Sprintf("%s/host_%s_%s", filePath, encryption, bytesForKeygen)
		_, err := strconv.Atoi(bytesForKeygen)
		clientPvtKeyNames = append(clientPvtKeyNames, fileName)
		caPvtKeyNames = append(caPvtKeyNames, caFileName)
		hostPvtKeyNames = append(hostPvtKeyNames, hostFileName)
		if err == nil {
			genereatePvtPublicKeyPairs(fileName, encryption, &bytesForKeygen)
			if ca == true {
				genereatePvtPublicKeyPairs(caFileName, encryption, &bytesForKeygen)
			}
			if host == true {
				genereatePvtPublicKeyPairs(hostFileName, encryption, &bytesForKeygen)
			}
		} else {
			genereatePvtPublicKeyPairs(fileName, encryption, nil)
			if ca == true {
				genereatePvtPublicKeyPairs(caFileName, encryption, nil)
			}
			if host == true {
				genereatePvtPublicKeyPairs(hostFileName, encryption, nil)
			}
		}
	}
	return clientPvtKeyNames, caPvtKeyNames, hostPvtKeyNames
}

func createAccountPoliciesAndPrincipals(accountName string, filePath string) (credz.UserPolicy, credz.CaPublicKeyRequest) {
	userPolicydata := credz.UserPolicy{
		Account:   accountName,
		Version:   "1.1",
		CreatedOn: 123}

	caPubkeyReq := credz.CaPublicKeyRequest{
		Version:   "1.1",
		CreatedOn: 123}
	var usrpAuthPrincipals credz.UserPolicy_SshAuthorizedPrincipals

	var optKey credz.Option
	var optKey2 credz.Option
	var opt_array []*credz.Option
	optKey.Key = &credz.Option_Id{Id: 16}
	optKey2.Key = &credz.Option_Id{Id: 3}
	optKey2.Value = fmt.Sprintf("show ssh | inc %s ; show ssh session detail", accountName)
	opt_array = append(opt_array, &optKey)
	opt_array = append(opt_array, &optKey2)

	for i := 1; i <= 4; i++ {
		keyTypeName := credz.KeyType_name[int32(i)]
		tempList := strings.Split(string(keyTypeName), "_")
		bytesForKeygen := tempList[len(tempList)-1]
		encryption := tempList[2]
		caPubKeyName := fmt.Sprintf("%s/ca_%s_%s.pub", filePath, encryption, bytesForKeygen)
		authPrincipals := credz.UserPolicy_SshAuthorizedPrincipal{
			Options:        opt_array,
			AuthorizedUser: encryption + "_" + bytesForKeygen,
		}
		usrpAuthPrincipals.AuthorizedPrincipals = append(usrpAuthPrincipals.AuthorizedPrincipals, &authPrincipals)
		publicKeyBytes, _ := os.ReadFile(caPubKeyName)
		pubkeyData := credz.PublicKey{
			PublicKey:   publicKeyBytes,
			KeyType:     credz.KeyType(i),
			Description: keyTypeName}
		caPubkeyReq.SshCaPublicKeys = append(caPubkeyReq.SshCaPublicKeys, &pubkeyData)
	}
	userPolicydata.AuthorizedPrincipals = &usrpAuthPrincipals

	log.Infof("User Policy:", userPolicydata)
	log.Infof("CA Public key Request:", caPubkeyReq)

	return userPolicydata, caPubkeyReq
}

func createAccountCredentials(filePath string, accountName string) credz.AccountCredentials {
	credentialsData := credz.AccountCredentials{
		Account:   accountName,
		Version:   "1.1",
		CreatedOn: 123}

	var optKey credz.Option
	var optKey2 credz.Option
	var optArray []*credz.Option
	optKey.Key = &credz.Option_Id{Id: 16}
	optKey2.Key = &credz.Option_Id{Id: 3}
	optKey2.Value = fmt.Sprintf("show ssh | inc %s", accountName)
	optArray = append(optArray, &optKey)
	optArray = append(optArray, &optKey2)

	for i := 1; i <= 5; i++ {
		keyTypeName := credz.KeyType_name[int32(i)]
		tempList := strings.Split(string(keyTypeName), "_")
		bytesForKeygen := tempList[len(tempList)-1]
		encryption := tempList[2]
		fileName := fmt.Sprintf("%s/%s_%s", filePath, encryption, bytesForKeygen)
		publicKeyBytes, _ := os.ReadFile(fmt.Sprintf("%s.pub", fileName))
		pubKey := strings.Split(string(publicKeyBytes), " ")[1]

		keysData := credz.AccountCredentials_AuthorizedKey{
			AuthorizedKey: []byte(pubKey),
			KeyType:       credz.KeyType(i),
			Options:       optArray,
			Description:   fmt.Sprintf("%s pub key", tempList[2])}
		credentialsData.AuthorizedKeys = append(credentialsData.AuthorizedKeys, &keysData)
	}
	log.Infof("AccountCredentials:", credentialsData)
	return credentialsData
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

func roatateAccountCredentialsRequestUser(stream credz.Credentialz_RotateAccountCredentialsClient, aureq credz.AuthorizedUsersRequest) {

	log.Infof("Send Rotate Account Credentisls Request ")
	err := stream.Send(&credz.RotateAccountCredentialsRequest{Request: &credz.RotateAccountCredentialsRequest_User{User: &aureq}})
	if err != nil {
		log.Exit("Credz:  Stream send returned error: " + err.Error())
	}
	gotRes, err := stream.Recv()
	if err != nil {
		log.Exit("Credz:  Stream receive returned error: " + err.Error())
	}
	aares := gotRes.GetUser()
	if aares == nil {
		log.Infof("Authorized users response is nil")
	}
	log.Infof("Rotate Account User Request done")
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

func rotateHostParametersRequestForSshCAPubKey(stream credz.Credentialz_RotateHostParametersClient, caPubkeyReq credz.CaPublicKeyRequest) {
	log.Info("Credentialz: SSH CA public key request")

	err := stream.Send(&credz.RotateHostParametersRequest{Request: &credz.RotateHostParametersRequest_SshCaPublicKey{SshCaPublicKey: &caPubkeyReq}})
	if err != nil {
		log.Exit("Credz:  Stream send returned error: " + err.Error())
	}

	gotRes, err := stream.Recv()
	if err != nil {
		log.Exit("Credz:  Stream receive returned error: " + err.Error())
	}
	aares := gotRes.GetSshCaPublicKey()
	if aares == nil {
		log.Exit("CA public keys response is nil")
	}

	fmt.Println("CA public keys  Request done")
}

func rotateHostParametersRequestForServerKeys(stream credz.Credentialz_RotateHostParametersClient, skreq credz.ServerKeysRequest) {
	log.Info("Credentialz: Server Keys request")

	err := stream.Send(&credz.RotateHostParametersRequest{Request: &credz.RotateHostParametersRequest_ServerKeys{ServerKeys: &skreq}})
	if err != nil {
		log.Exit("Credz:  Stream send returned error: " + err.Error())
		return
	}

	gotRes, err := stream.Recv()
	if err != nil {
		log.Exit("Credz:  Stream receive returned error: " + err.Error())
	}
	aares := gotRes.GetServerKeys()
	if aares == nil {
		log.Exit("Server keys response is nil")
	}

	fmt.Println("Server public keys Request done")
}

func getIpAndPortFromBindingFile() (string, int, error) {
	bindingFile := flag.Lookup("binding").Value.String()
	in, err := os.ReadFile(bindingFile)
	if err != nil {
		return "", 0, err
	}
	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		return "", 0, err
	}
	target := b.Duts[0].Ssh.Target
	targetIP := strings.Split(target, ":")[0]
	targetPort, _ := strconv.Atoi(strings.Split(target, ":")[1])
	return targetIP, targetPort, nil
}

func TestCredentialz_2(t *testing.T) {
	filePath := "credz"
	accountName := "CREDZ"
	accountPassword := "credz123"
	authenticationType := []string{"ecdsa-sha2-nistp256", "ecdsa-sha2-nistp521", "ssh-ed25519", "rsa-pubkey", "rsa-pubkey"}

	var users []*oc.System_Aaa_Authentication_User
	users = append(users, &oc.System_Aaa_Authentication_User{
		Username: &accountName,
		Password: &accountPassword,
		Role:     oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN,
	})

	tartgetIP, tartgetPort, err := getIpAndPortFromBindingFile()
	if err != nil {
		t.Fatalf("Error in reading target IP and Port from Binding file: %v", err)
		return
	}

	dut := ondatra.DUT(t, "dut")
	createUsersOnDevice(t, dut, users)

	gnsiC := dut.RawAPIs().GNSI(t)

	mkdirErr := os.Mkdir(filePath, 0755)
	defer os.RemoveAll(filePath)
	if mkdirErr != nil {
		t.Fatalf("Error creating directory: %v", mkdirErr)
		return
	}
	clientKeyNames, _, _ := generateAllSupportedPvtPublicKeyPairs(filePath, false, false)

	hostParamStream, err := gnsiC.Credentialz().RotateHostParameters(context.Background())
	defer hostParamStream.CloseSend()
	if err != nil {
		t.Fatalf("failed to get stream: %v", err)
	}

	var authTypes []credz.AuthenticationType
	authTypes = append(authTypes, credz.AuthenticationType_AUTHENTICATION_TYPE_PASSWORD)
	authTypes = append(authTypes, credz.AuthenticationType_AUTHENTICATION_TYPE_PUBKEY)
	authTypes = append(authTypes, credz.AuthenticationType_AUTHENTICATION_TYPE_KBDINTERACTIVE)

	aareq := credz.AllowedAuthenticationRequest{
		AuthenticationTypes: authTypes,
	}
	log.Infof("Aollowed Authentication request type Password, PubKey and KBDInteractive")
	rotateHostParametersRequest(hostParamStream, aareq)
	finalizeHostRequest(hostParamStream)
	hostParamStream.CloseSend()
	time.Sleep(2 * time.Second)

	var akreq credz.AuthorizedKeysRequest

	credentialsData := createAccountCredentials(filePath, accountName)
	akreq.Credentials = append(akreq.Credentials, &credentialsData)

	stream, err := gnsiC.Credentialz().RotateAccountCredentials(context.Background())
	defer stream.CloseSend()
	if err != nil {
		t.Fatalf("failed to get stream: %v", err)
	}
	roatateAccountCredentialsRequest(stream, akreq)
	finalizeAccountRequest(stream)
	stream.CloseSend()
	time.Sleep(2 * time.Second)

	sshPasswordParams := sshPasswordParams{
		user:     accountName,
		password: accountPassword,
	}

	hostParamStream, err = gnsiC.Credentialz().RotateHostParameters(context.Background())
	defer hostParamStream.CloseSend()

	t.Run("Set Password as authentication type and check ssh with password and pubkey", func(t *testing.T) {
		var authTypePwd []credz.AuthenticationType
		authTypePwd = append(authTypePwd, credz.AuthenticationType_AUTHENTICATION_TYPE_PASSWORD)

		aareq = credz.AllowedAuthenticationRequest{
			AuthenticationTypes: authTypePwd,
		}
		log.Infof("Aollowed Authentication request type Password")

		if err != nil {
			t.Fatalf("failed to get stream: %v", err)
		}
		rotateHostParametersRequest(hostParamStream, aareq)
		finalizeHostRequest(hostParamStream)
		time.Sleep(2 * time.Second)
		err = createSSHClientAndVerify(tartgetIP, tartgetPort, sshPasswordParams, "password", nil)
		if err != nil {
			log.Exit("Error in establishing SSH connection with password: ", err.Error())
		}

		errCount := 0
		for i := 0; i < len(clientKeyNames); i++ {
			pvtKeyBytes, err := os.ReadFile(clientKeyNames[i])
			if err != nil {
				log.Exit("Error in parsing certificate file: ", err)
			}
			sshPubKeyParams := sshPubkeyParams{
				user:       accountName,
				privateKey: pvtKeyBytes,
			}
			err = createSSHClientAndVerify(tartgetIP, tartgetPort, sshPubKeyParams, authenticationType[i], nil)
			if err != nil {
				errCount = errCount + 1
			}
		}
		if errCount == 0 {
			log.Exit("Establishing SSH connection with Public key which is not expected")
		} else {
			log.Infof("Error in establishing SSH connection with Public key authentication which is expected")
		}

	})

	t.Run("set Pubkey as authentication type and check ssh with password and pubkey", func(t *testing.T) {
		var authTypePubkey []credz.AuthenticationType
		authTypePubkey = append(authTypePubkey, credz.AuthenticationType_AUTHENTICATION_TYPE_PUBKEY)

		aareq = credz.AllowedAuthenticationRequest{
			AuthenticationTypes: authTypePubkey}

		log.Infof("Aollowed Authentication request type PubKey")
		rotateHostParametersRequest(hostParamStream, aareq)
		finalizeHostRequest(hostParamStream)
		time.Sleep(2 * time.Second)

		err = createSSHClientAndVerify(tartgetIP, tartgetPort, sshPasswordParams, "password", nil)
		if err == nil {
			log.Exit("Establishing SSH connection with password which is not expected")
		} else {
			log.Infof("Error in establishing connection with password which is expected")
		}
		errCount := 0
		for i := 0; i < len(clientKeyNames); i++ {
			pvtKeyBytes, err := os.ReadFile(clientKeyNames[i])
			if err != nil {
				log.Exit("Error in parsing certificate file: ", err)
			}
			sshPubKeyParams := sshPubkeyParams{
				user:       accountName,
				privateKey: pvtKeyBytes,
			}
			err = createSSHClientAndVerify(tartgetIP, tartgetPort, sshPubKeyParams, authenticationType[i], nil)
			if err != nil {
				errCount = errCount + 1
			}
		}
		if errCount != 0 {
			log.Exit("Error in establishing SSH connection with pubkey authentication")
		} else {
			log.Infof("Establishing SSH connection with Pubkey authentication")
		}
	})

	aareq = credz.AllowedAuthenticationRequest{
		AuthenticationTypes: authTypes,
	}
	log.Infof("Aollowed Authentication request type Password, PubKey and KBDInteractive")
	rotateHostParametersRequest(hostParamStream, aareq)
	finalizeHostRequest(hostParamStream)
	hostParamStream.CloseSend()
}

func TestCredentialz_3(t *testing.T) {
	filePath := "credz"
	accountName := "CERT"
	authenticationType := []string{"ecdsa-nistp256-cert", "ecdsa-nistp521-cert", "ed25519-cert", "rsa-cert"}
	hostKeyAlgorithms := []string{"ecdsa-sha2-nistp256-cert-v01@openssh.com",
		"ecdsa-sha2-nistp521-cert-v01@openssh.com", "ssh-ed25519-cert-v01@openssh.com", "ssh-rsa-cert-v01@openssh.com"}

	tartgetIP, tartgetPort, err := getIpAndPortFromBindingFile()
	if err != nil {
		t.Fatalf("Error in reading target IP and Port from Binding file: %v", err)
		return
	}

	var users []*oc.System_Aaa_Authentication_User
	users = append(users, &oc.System_Aaa_Authentication_User{
		Username: &accountName,
		Role:     oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN,
	})

	dut := ondatra.DUT(t, "dut")
	createUsersOnDevice(t, dut, users)

	gnsiC := dut.RawAPIs().GNSI(t)

	mkdirErr := os.Mkdir(filePath, 0755)
	defer os.RemoveAll(filePath)
	if mkdirErr != nil {
		t.Fatalf("Error creating directory: %v", mkdirErr)
		return
	}
	clientKeyNames, caKeyNames, hostKeyNames := generateAllSupportedPvtPublicKeyPairs(filePath, true, true)
	clientKeyNames = clientKeyNames[:len(clientKeyNames)-1]
	caKeyNames = caKeyNames[:len(caKeyNames)-1]
	hostKeyNames = hostKeyNames[:len(hostKeyNames)-1]
	var authUsrReq credz.AuthorizedUsersRequest

	policyReq, caPubkeyReq := createAccountPoliciesAndPrincipals(accountName, filePath)
	authUsrReq.Policies = append(authUsrReq.Policies, &policyReq)

	stream, err := gnsiC.Credentialz().RotateAccountCredentials(context.Background())
	if err != nil {
		t.Fatalf("failed to get stream: %v", err)
	}
	roatateAccountCredentialsRequestUser(stream, authUsrReq)
	finalizeAccountRequest(stream)
	stream.CloseSend()
	time.Sleep(2 * time.Second)

	hostParamStream, err := gnsiC.Credentialz().RotateHostParameters(context.Background())
	defer hostParamStream.CloseSend()
	if err != nil {
		t.Fatalf("	failed to get stream: %v", err)
	}

	rotateHostParametersRequestForSshCAPubKey(hostParamStream, caPubkeyReq)
	finalizeHostRequest(hostParamStream)

	for i := 0; i < len(clientKeyNames); i++ {
		_, authUsr := filepath.Split(clientKeyNames[i])
		generateClientCertificates(caKeyNames[i], authUsr, clientKeyNames[i])
		generateHostCertificates(caKeyNames[i], hostKeyNames[i])
	}

	var skreq credz.ServerKeysRequest
	var authArtifacts []*credz.ServerKeysRequest_AuthenticationArtifacts

	skreq = credz.ServerKeysRequest{
		AuthArtifacts: []*credz.ServerKeysRequest_AuthenticationArtifacts{},
		Version:       "1.1",
		CreatedOn:     123,
	}

	for i := 0; i < len(clientKeyNames); i++ {
		hostCertFile := hostKeyNames[i] + "-cert.pub"

		hostPvtKeyBytes, err := os.ReadFile(hostKeyNames[i])
		if err != nil {
			log.Exit("Error in reading host private key file: ", err.Error())
		}
		hostcertBytes, err := os.ReadFile(hostCertFile)
		if err != nil {
			log.Exit("Error in reading host certificate file: ", err.Error())
		}
		auArtifacts := &credz.ServerKeysRequest_AuthenticationArtifacts{PrivateKey: hostPvtKeyBytes, Certificate: hostcertBytes}
		authArtifacts = append(authArtifacts, auArtifacts)
	}
	skreq.AuthArtifacts = authArtifacts

	rotateHostParametersRequestForServerKeys(hostParamStream, skreq)
	finalizeHostRequest(hostParamStream)
	time.Sleep(2 * time.Second)

	t.Run("Client Based Authentication", func(t *testing.T) {
		errCount := 0
		for i := 0; i < len(clientKeyNames); i++ {
			clientCertPath := clientKeyNames[i] + "-cert.pub"
			// _, user := filepath.Split(clientKeyNames[i])

			clientPvtKey, err := os.ReadFile(clientKeyNames[i])
			if err != nil {
				log.Exit("Error in reading Client Private key: ", err.Error())
			}
			clientCert, err := os.ReadFile(clientCertPath)
			if err != nil {
				log.Exit("Error in reading : ", err.Error())
			}
			sshParams := sshClientParams{
				user:         accountName,
				clientPvtKey: clientPvtKey,
				clientCert:   clientCert,
			}
			err = createSSHClientAndVerify(tartgetIP, tartgetPort, sshParams, authenticationType[i], nil)
			if err != nil {
				errCount = errCount + 1
				log.Error("Error in establishing ssh connection: ", err.Error())
			}
		}
		if errCount == 0 {
			log.Infof("Verified all types of encryption algorithms for Host based authentication successfully")
		} else {
			log.Exit("Host based authentication failed")
		}

	})

	t.Run("Host Based Authentication", func(t *testing.T) {
		errCount := 0
		for i := 0; i < len(clientKeyNames); i++ {
			clientCertPath := clientKeyNames[i] + "-cert.pub"
			hostCertPath := hostKeyNames[i] + "-cert.pub"
			// _, user := filepath.Split(clientKeyNames[i])

			clientPvtKey, err := os.ReadFile(clientKeyNames[i])
			if err != nil {
				log.Exit("Error in reading Client Private key: ", err.Error())
			}
			clientCert, err := os.ReadFile(clientCertPath)
			if err != nil {
				log.Exit("Error in reading : ", err.Error())
			}
			hostCert, err := os.ReadFile(hostCertPath)
			if err != nil {
				log.Exit("Error in reading host certificate: ", err.Error())
			}
			sshParams := sshHostParams{
				user:              accountName,
				clientPvtKey:      clientPvtKey,
				clientCert:        clientCert,
				hostCert:          hostCert,
				hostKeyAlgorithms: []string{hostKeyAlgorithms[i]},
			}
			hostVerifier := strings.Split(hostKeyAlgorithms[i], "@")[0]
			err = createSSHClientAndVerify(tartgetIP, tartgetPort, sshParams, authenticationType[i], &hostVerifier)
			if err != nil {
				errCount = errCount + 1
				log.Error("Error in establishing ssh connection: ", err.Error())
			}
		}
		if errCount == 0 {
			log.Infof("Verified all types of encryption algorithms for Host based authentication successfully")
		} else {
			log.Exit("Host based authentication failed")
		}

	})

}

func TestCredentialz_4(t *testing.T) {
	filePath := "credz"
	accountName := "CREDZ"
	authenticationType := []string{"ecdsa-sha2-nistp256", "ecdsa-sha2-nistp521", "ssh-ed25519", "rsa-pubkey", "rsa-pubkey"}

	tartgetIP, tartgetPort, err := getIpAndPortFromBindingFile()
	if err != nil {
		t.Fatalf("Error in reading target IP and Port from Binding file: %v", err)
		return
	}

	var akreq credz.AuthorizedKeysRequest
	dut := ondatra.DUT(t, "dut")

	var users []*oc.System_Aaa_Authentication_User
	users = append(users, &oc.System_Aaa_Authentication_User{Username: &accountName,
		Role: oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN,
	})

	createUsersOnDevice(t, dut, users)

	gnsiC := dut.RawAPIs().GNSI(t)

	mkdirErr := os.Mkdir(filePath, 0755)
	defer os.RemoveAll(filePath)
	if mkdirErr != nil {
		t.Fatalf("Error creating directory: %v", mkdirErr)
		return
	}

	clientKeyNames, _, _ := generateAllSupportedPvtPublicKeyPairs(filePath, false, false)

	credentialsData := createAccountCredentials(filePath, accountName)
	akreq.Credentials = append(akreq.Credentials, &credentialsData)

	stream, err := gnsiC.Credentialz().RotateAccountCredentials(context.Background())
	defer stream.CloseSend()
	if err != nil {
		t.Fatalf("failed to get stream: %v", err)
	}

	roatateAccountCredentialsRequest(stream, akreq)

	finalizeAccountRequest(stream)

	t.Run("PublicKey Based Authentication", func(t *testing.T) {
		errCount := 0
		for i := 0; i < len(clientKeyNames); i++ {
			pvtKeyBytes, err := os.ReadFile(clientKeyNames[i])
			if err != nil {
				log.Exit("Error in parsing certificate file: ", err)
			}
			sshParams := sshPubkeyParams{
				user:       accountName,
				privateKey: pvtKeyBytes,
			}
			err = createSSHClientAndVerify(tartgetIP, tartgetPort, sshParams, authenticationType[i], nil)
			if err != nil {
				errCount = errCount + 1
			}
		}
		if errCount != 0 {
			log.Exit("Error in establishing SSH connection with Public Key based")
		}
	})

	var akreqForNeg credz.AuthorizedKeysRequest
	credentials := credz.AccountCredentials{
		Account:   accountName,
		Version:   "1.1",
		CreatedOn: 123}
	akreqForNeg.Credentials = append(akreqForNeg.Credentials, &credentials)

	log.Infof("Removing all authorized keys from user")
	roatateAccountCredentialsRequest(stream, akreqForNeg)

	finalizeAccountRequest(stream)
	stream.CloseSend()
	t.Run("Remove all authorized keys from user and check ssh", func(t *testing.T) {
		errCount := 0
		for i := 0; i < len(clientKeyNames); i++ {
			pvtKeyBytes, err := os.ReadFile(clientKeyNames[i])
			if err != nil {
				log.Exit("Error in parsing certificate file: ", err)
			}
			sshParams := sshPubkeyParams{
				user:       accountName,
				privateKey: pvtKeyBytes,
			}
			err = createSSHClientAndVerify(tartgetIP, tartgetPort, sshParams, authenticationType[i], nil)
			if err != nil {
				errCount = errCount + 1
			}
		}
		if errCount == 0 {
			log.Exit("Establishing SSH connection which is not expected after removing authorized keys")
		} else {
			log.Infof("Error occurred while attempting to establish SSH connection as expected after removing authorized keys from the user.")
		}
	})
}
