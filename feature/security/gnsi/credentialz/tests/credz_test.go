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
	"github.com/openconfig/gnoi/system"
	credz "github.com/openconfig/gnsi/credentialz"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"google.golang.org/protobuf/encoding/prototext"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"golang.org/x/crypto/ssh"

	"github.com/openconfig/featureprofiles/internal/args"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/testt"

	spb "github.com/openconfig/gnoi/system"
)

const (
	maxSwitchoverTime = 900
	controlcardType   = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	activeController  = oc.Platform_ComponentRedundantRole_PRIMARY
	standbyController = oc.Platform_ComponentRedundantRole_SECONDARY
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

var (
	serverIP = flag.String("serverip", "", "Server IP address")
)

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

func createSSHClientWithCmd(hostIP string, port int, hostCert string, pvtKey string, accountName string) (string, error) {
	cmd := exec.Command("ssh", "-tt", "-o", "StrictHostKeyChecking=no", "-o", fmt.Sprintf("CertificateFile=%s", hostCert),
		"-i", pvtKey, fmt.Sprintf("%s@%s", accountName, hostIP), "-p", fmt.Sprintf("%d", port), fmt.Sprintf("show ssh | inc %s", accountName))

	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error starting SSH command: %s\n", err)
		return "", err
	}
	return string(out), nil
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

func generateKeyPairsUsingHiba(filePath string, hibaPath string, bytesForEncrypt string, userName *string) error {
	var cmd *exec.Cmd
	if userName != nil {
		cmd = exec.Command(hibaPath, "-c", "-d", filePath, "-b", bytesForEncrypt, "-u", "-I", *userName, "--", "-N", "")
	} else {
		cmd = exec.Command(hibaPath, "-c", "-d", filePath, "-b", bytesForEncrypt, "--", "-N", "")
	}
	err := cmd.Run()
	if err != nil {
		return err
	}
	log.Infof("Generated public/private rsa keypair and saved as %v", filePath)
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

func generateHostCertificates(caPvtKeyName string, hostPubkeyName string, validity string) error {
	hostPubkeyName = hostPubkeyName + ".pub"
	cmd := exec.Command("ssh-keygen", "-s", caPvtKeyName, "-I", "CISCO", "-V", validity, "-h", hostPubkeyName)
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

func roatateAccountCredentialsRequest(t *testing.T, stream credz.Credentialz_RotateAccountCredentialsClient, akreq credz.AuthorizedKeysRequest) {

	log.Infof("Send Rotate Account Credentisls Request ")
	err := stream.Send(&credz.RotateAccountCredentialsRequest{Request: &credz.RotateAccountCredentialsRequest_Credential{Credential: &akreq}})
	if err != nil {
		t.Fatalf("Credz:  Stream send returned error: " + err.Error())
	}
	gotRes, err := stream.Recv()
	if err != nil {
		t.Fatalf("Credz:  Stream receive returned error: " + err.Error())
	}
	aares := gotRes.GetCredential()
	if aares == nil {
		log.Infof("Authorized keys response is nil")
	}
	log.Infof("Rotate Account Credentials Request done")
}

func roatateAccountCredentialsRequestUser(t *testing.T, stream credz.Credentialz_RotateAccountCredentialsClient, aureq credz.AuthorizedUsersRequest) {

	log.Infof("Send Rotate Account Credentisls Request ")
	err := stream.Send(&credz.RotateAccountCredentialsRequest{Request: &credz.RotateAccountCredentialsRequest_User{User: &aureq}})
	if err != nil {
		t.Fatalf("Credz:  Stream send returned error: " + err.Error())
	}
	gotRes, err := stream.Recv()
	if err != nil {
		t.Fatalf("Credz:  Stream receive returned error: " + err.Error())
	}
	aares := gotRes.GetUser()
	if aares == nil {
		log.Infof("Authorized users response is nil")
	}
	log.Infof("Rotate Account User Request done")
}

func finalizeAccountRequest(t *testing.T, stream credz.Credentialz_RotateAccountCredentialsClient) {
	log.Infof("Send Finalize Request")
	err := stream.Send(&credz.RotateAccountCredentialsRequest{Request: &credz.RotateAccountCredentialsRequest_Finalize{}})
	if err != nil {
		t.Fatalf("Stream send finalize failed : " + err.Error())
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

func finalizeHostRequest(t *testing.T, stream credz.Credentialz_RotateHostParametersClient) {
	log.Infof("Credz: Host Parameters Finalize")
	err := stream.Send(&credz.RotateHostParametersRequest{Request: &credz.RotateHostParametersRequest_Finalize{}})
	if err != nil {
		fmt.Println("Credz:  Stream send finalize failed : " + err.Error())
	}
}

func rotateHostParametersRequest(t *testing.T, stream credz.Credentialz_RotateHostParametersClient, aareq credz.AllowedAuthenticationRequest) {
	log.Info("Credentialz: Allowed Authentication request")

	err := stream.Send(&credz.RotateHostParametersRequest{Request: &credz.RotateHostParametersRequest_AuthenticationAllowed{AuthenticationAllowed: &aareq}})
	if err != nil {
		t.Fatalf("Credz:  Stream send returned error: " + err.Error())
	}

	gotRes, err := stream.Recv()
	if err != nil {
		t.Fatalf("Credz:  Stream receive returned error: " + err.Error())
	}
	aares := gotRes.GetAuthenticationAllowed()
	if aares == nil {
		t.Fatalf("Credentialz response is nil")
	}
}

func rotateHostParametersRequestForSshCAPubKey(t *testing.T, stream credz.Credentialz_RotateHostParametersClient, caPubkeyReq credz.CaPublicKeyRequest) {
	log.Info("Credentialz: SSH CA public key request")

	err := stream.Send(&credz.RotateHostParametersRequest{Request: &credz.RotateHostParametersRequest_SshCaPublicKey{SshCaPublicKey: &caPubkeyReq}})
	if err != nil {
		t.Fatalf("Credz:  Stream send returned error: " + err.Error())
	}

	gotRes, err := stream.Recv()
	if err != nil {
		t.Fatalf("Credz:  Stream receive returned error: " + err.Error())
	}
	aares := gotRes.GetSshCaPublicKey()
	if aares == nil {
		t.Fatalf("CA public keys response is nil")
	}

	fmt.Println("CA public keys  Request done")
}

func rotateHostParametersRequestForServerKeys(t *testing.T, stream credz.Credentialz_RotateHostParametersClient, skreq credz.ServerKeysRequest) {
	log.Info("Credentialz: Server Keys request")

	err := stream.Send(&credz.RotateHostParametersRequest{Request: &credz.RotateHostParametersRequest_ServerKeys{ServerKeys: &skreq}})
	if err != nil {
		t.Fatalf("Credz:  Stream send returned error: " + err.Error())
		return
	}

	gotRes, err := stream.Recv()
	if err != nil {
		t.Fatalf("Credz:  Stream receive returned error: " + err.Error())
	}
	aares := gotRes.GetServerKeys()
	if aares == nil {
		t.Fatalf("Server keys response is nil")
	}

	fmt.Println("Server public keys Request done")
}

func rotateHostParametersRequestForprincipalCheck(t *testing.T, stream credz.Credentialz_RotateHostParametersClient, pcreq credz.AuthorizedPrincipalCheckRequest) {
	log.Info("Credentialz: Authorized Principal check request")

	err := stream.Send(&credz.RotateHostParametersRequest{Request: &credz.RotateHostParametersRequest_AuthorizedPrincipalCheck{AuthorizedPrincipalCheck: &pcreq}})
	if err != nil {
		t.Fatalf("Credz:  Stream send returned error: " + err.Error())
	}

	gotRes, err := stream.Recv()
	if err != nil {
		t.Fatalf("Credz:  Stream receive returned error: " + err.Error())
	}
	aares := gotRes.GetAuthorizedPrincipalCheck()
	if aares == nil {
		t.Fatalf("Authorized pricipal check response is nil")
	}

	fmt.Println("Authorized principal check Request done")
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

func createFileAndWriteData(filePath string, fileName string, data string) error {
	err := os.MkdirAll(filePath, os.ModePerm)
	if err != nil {
		return err
	}
	hostPubKeyFilePath := fmt.Sprintf("%s/%s", filePath, fileName)
	hostPubKeyFile, err := os.Create(hostPubKeyFilePath)
	if err != nil {
		return err
	}
	defer hostPubKeyFile.Close()
	_, err = hostPubKeyFile.WriteString(data)
	if err != nil {
		return err
	}
	return nil
}

func createAnIdentityFileWithServerName(hibaPath string, filePath string, idKeyPairs []string) error {
	var cmd *exec.Cmd
	arguments := append([]string{"-i", "-f", filePath}, idKeyPairs...)
	cmd = exec.Command(hibaPath, arguments...)

	err := cmd.Run()
	if err != nil {
		return err
	}
	log.Infof("Generated identity file with server name and save as %v", filePath)
	return nil
}

func createGrantFileWithClientName(hibaPath string, filePath string, idKeyPairs []string) error {
	var cmd *exec.Cmd
	arguments := append([]string{"-f", filePath}, idKeyPairs...)
	cmd = exec.Command(hibaPath, arguments...)

	err := cmd.Run()
	if err != nil {
		return err
	}
	log.Infof("Generated grants file with client name and save as %v", filePath)
	return nil
}

func generateHostCertByAttachingIdentities(hibaPath string, filePath string, hostName string, identityFileName, validity string) error {
	cmd := exec.Command(hibaPath, "-s", "-d", filePath, "-h", "-I", hostName, "-H", identityFileName, "--", "-P", "", "-V", validity)

	err := cmd.Run()
	if err != nil {
		return err
	}
	hostCert := fmt.Sprintf("%s/hosts/%s-cert.pub", filePath, hostName)
	output, err := exec.Command("ssh-keygen", "-Lf", hostCert).CombinedOutput()
	if err != nil {
		return err
	}
	log.Infof("Generated host certificate by attaching identities file")
	log.Infof(string(output))
	return nil
}

func makeUserEligleForGrants(hibaPath string, filePath string, usrName string, clientName string) error {
	cmd := exec.Command(hibaPath, "-d", filePath, "-p", "-I", usrName, "-H", clientName)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	log.Infof(string(out))
	log.Infof("%s user is now eligible for grants", usrName)
	return nil
}

func generateClientCertByAttachingGrants(hibaPath string, filePath string, usrName string, grantFileName, validity string) error {
	cmd := exec.Command(hibaPath, "-s", "-d", filePath, "-u", "-I", usrName, "-H", grantFileName, "--", "-P", "", "-V", validity)

	out, err := cmd.CombinedOutput()
	log.Infof(string(out))
	if err != nil {
		return err
	}
	clientCert := fmt.Sprintf("%s/users/%s-cert.pub", filePath, usrName)
	output, err := exec.Command("ssh-keygen", "-Lf", clientCert).CombinedOutput()
	if err != nil {
		return err
	}
	log.Infof("Generated client certificate by attaching grants file")
	log.Infof(string(output))
	return nil
}

func RestartProcess(t *testing.T, dut *ondatra.DUTDevice, processName string) error {
	resp, err := dut.RawAPIs().GNOI(t).System().KillProcess(context.Background(), &system.KillProcessRequest{
		Name:    processName,
		Restart: true,
		Signal:  system.KillProcessRequest_SIGNAL_TERM,
	})
	if err != nil {
		return err
	}
	if resp == nil {
		t.Error("")
	}
	time.Sleep(30 * time.Second)
	return nil
}

func rpSwitchOver(t *testing.T, dut *ondatra.DUTDevice) {

	controllerCards := components.FindComponentsByType(t, dut, controlcardType)
	t.Logf("Found controller card list: %v", controllerCards)

	if *args.NumControllerCards >= 0 && len(controllerCards) != *args.NumControllerCards {
		t.Errorf("Incorrect number of controller cards: got %v, want exactly %v (specified by flag)", len(controllerCards), *args.NumControllerCards)
	}

	if got, want := len(controllerCards), 2; got < want {
		t.Skipf("Not enough controller cards for the test on %v: got %v, want at least %v", dut.Model(), got, want)
	}

	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyRP(t, dut, controllerCards)
	t.Logf("Detected rpStandby: %v, rpActive: %v", rpStandbyBeforeSwitch, rpActiveBeforeSwitch)

	switchoverReady := gnmi.OC().Component(rpActiveBeforeSwitch).SwitchoverReady()
	gnmi.Await(t, dut, switchoverReady.State(), 30*time.Minute, true)
	t.Logf("SwitchoverReady().Get(t): %v", gnmi.Get(t, dut, switchoverReady.State()))
	if got, want := gnmi.Get(t, dut, switchoverReady.State()), true; got != want {
		t.Errorf("switchoverReady.Get(t): got %v, want %v", got, want)
	}

	gnoiClient := dut.RawAPIs().GNOI(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	switchoverRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(rpStandbyBeforeSwitch, useNameOnly),
	}
	t.Logf("switchoverRequest: %v", switchoverRequest)
	switchoverResponse, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
	if err != nil {
		t.Fatalf("Failed to perform control processor switchover with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v, err: %v", switchoverResponse, err)

	want := rpStandbyBeforeSwitch
	got := ""
	if deviations.GNOISubcomponentPath(dut) {
		got = switchoverResponse.GetControlProcessor().GetElem()[0].GetName()
	} else {
		got = switchoverResponse.GetControlProcessor().GetElem()[1].GetKey()["name"]
	}
	if got != want {
		t.Fatalf("switchoverResponse.GetControlProcessor().GetElem()[0].GetName(): got %v, want %v", got, want)
	}
	if got, want := switchoverResponse.GetVersion(), ""; got == want {
		t.Errorf("switchoverResponse.GetVersion(): got %v, want non-empty version", got)
	}
	if got := switchoverResponse.GetUptime(); got == 0 {
		t.Errorf("switchoverResponse.GetUptime(): got %v, want > 0", got)
	}

	startSwitchover := time.Now()
	t.Logf("Wait for new active RP to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f seconds since switchover started.", time.Since(startSwitchover).Seconds())
		time.Sleep(30 * time.Second)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("RP switchover has completed successfully with received time: %v", currentTime)
			break
		}
		if got, want := uint64(time.Since(startSwitchover).Seconds()), uint64(maxSwitchoverTime); got >= want {
			t.Fatalf("time.Since(startSwitchover): got %v, want < %v", got, want)
		}
	}
	t.Logf("RP switchover time: %.2f seconds", time.Since(startSwitchover).Seconds())

	rpStandbyAfterSwitch, rpActiveAfterSwitch := components.FindStandbyRP(t, dut, controllerCards)
	t.Logf("Found standbyRP after switchover: %v, activeRP: %v", rpStandbyAfterSwitch, rpActiveAfterSwitch)

	if got, want := rpActiveAfterSwitch, rpStandbyBeforeSwitch; got != want {
		t.Errorf("Get rpActiveAfterSwitch: got %v, want %v", got, want)
	}
	if got, want := rpStandbyAfterSwitch, rpActiveBeforeSwitch; got != want {
		t.Errorf("Get rpStandbyAfterSwitch: got %v, want %v", got, want)
	}

	t.Log("Validate OC Switchover time/reason.")
	activeRP := gnmi.OC().Component(rpActiveAfterSwitch)

	swTime, swTimePresent := gnmi.Watch(t, dut, activeRP.LastSwitchoverTime().State(), 1*time.Minute, func(val *ygnmi.Value[uint64]) bool { return val.IsPresent() }).Await(t)
	if !swTimePresent {
		t.Errorf("activeRP.LastSwitchoverTime().Watch(t).IsPresent(): got %v, want %v", false, true)
	} else {
		st, _ := swTime.Val()
		t.Logf("Found activeRP.LastSwitchoverTime(): %v", st)
	}

	if got, want := gnmi.Lookup(t, dut, activeRP.LastSwitchoverReason().State()).IsPresent(), true; got != want {
		t.Errorf("activeRP.LastSwitchoverReason().Lookup(t).IsPresent(): got %v, want %v", got, want)
	} else {
		lastSwitchoverReason := gnmi.Get(t, dut, activeRP.LastSwitchoverReason().State())
		t.Logf("Found lastSwitchoverReason.GetDetails(): %v", lastSwitchoverReason.GetDetails())
		t.Logf("Found lastSwitchoverReason.GetTrigger().String(): %v", lastSwitchoverReason.GetTrigger().String())
	}
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
	}

	dut := ondatra.DUT(t, "dut")
	createUsersOnDevice(t, dut, users)

	gnsiC := dut.RawAPIs().GNSI(t)

	mkdirErr := os.Mkdir(filePath, 0755)
	defer os.RemoveAll(filePath)
	if mkdirErr != nil {
		t.Fatalf("Error creating directory: %v", mkdirErr)
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
	rotateHostParametersRequest(t, hostParamStream, aareq)
	finalizeHostRequest(t, hostParamStream)
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
	roatateAccountCredentialsRequest(t, stream, akreq)
	finalizeAccountRequest(t, stream)
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
		rotateHostParametersRequest(t, hostParamStream, aareq)
		finalizeHostRequest(t, hostParamStream)
		time.Sleep(2 * time.Second)
		err = createSSHClientAndVerify(tartgetIP, tartgetPort, sshPasswordParams, "password", nil)
		if err != nil {
			t.Fatalf("Error in establishing SSH connection with password: %v", err.Error())
		}

		errCount := 0
		for i := 0; i < len(clientKeyNames); i++ {
			pvtKeyBytes, err := os.ReadFile(clientKeyNames[i])
			if err != nil {
				t.Fatalf("Error in parsing certificate file: %v", err)
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
			t.Fatalf("Establishing SSH connection with Public key which is not expected")
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
		rotateHostParametersRequest(t, hostParamStream, aareq)
		finalizeHostRequest(t, hostParamStream)
		time.Sleep(2 * time.Second)

		err = createSSHClientAndVerify(tartgetIP, tartgetPort, sshPasswordParams, "password", nil)
		if err == nil {
			t.Fatalf("Establishing SSH connection with password which is not expected")
		} else {
			log.Infof("Error in establishing connection with password which is expected")
		}
		errCount := 0
		for i := 0; i < len(clientKeyNames); i++ {
			pvtKeyBytes, err := os.ReadFile(clientKeyNames[i])
			if err != nil {
				t.Fatalf("Error in parsing certificate file: %v", err)
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
			t.Fatalf("Error in establishing SSH connection with pubkey authentication")
		} else {
			log.Infof("Establishing SSH connection with Pubkey authentication")
		}
	})

	aareq = credz.AllowedAuthenticationRequest{
		AuthenticationTypes: authTypes,
	}
	log.Infof("Aollowed Authentication request type Password, PubKey and KBDInteractive")
	rotateHostParametersRequest(t, hostParamStream, aareq)
	finalizeHostRequest(t, hostParamStream)
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
	roatateAccountCredentialsRequestUser(t, stream, authUsrReq)
	finalizeAccountRequest(t, stream)
	stream.CloseSend()
	time.Sleep(2 * time.Second)

	hostParamStream, err := gnsiC.Credentialz().RotateHostParameters(context.Background())
	defer hostParamStream.CloseSend()
	if err != nil {
		t.Fatalf("	failed to get stream: %v", err)
	}

	rotateHostParametersRequestForSshCAPubKey(t, hostParamStream, caPubkeyReq)
	finalizeHostRequest(t, hostParamStream)

	for i := 0; i < len(clientKeyNames); i++ {
		_, authUsr := filepath.Split(clientKeyNames[i])
		generateClientCertificates(caKeyNames[i], authUsr, clientKeyNames[i])
		generateHostCertificates(caKeyNames[i], hostKeyNames[i], "+1d")
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
			t.Fatalf("Error in reading host private key file: %v", err.Error())
		}
		hostcertBytes, err := os.ReadFile(hostCertFile)
		if err != nil {
			t.Fatalf("Error in reading host certificate file: %v", err.Error())
		}
		auArtifacts := &credz.ServerKeysRequest_AuthenticationArtifacts{PrivateKey: hostPvtKeyBytes, Certificate: hostcertBytes}
		authArtifacts = append(authArtifacts, auArtifacts)
	}
	skreq.AuthArtifacts = authArtifacts

	rotateHostParametersRequestForServerKeys(t, hostParamStream, skreq)
	finalizeHostRequest(t, hostParamStream)
	time.Sleep(2 * time.Second)

	t.Run("Client Based Authentication", func(t *testing.T) {
		errCount := 0
		for i := 0; i < len(clientKeyNames); i++ {
			clientCertPath := clientKeyNames[i] + "-cert.pub"

			clientPvtKey, err := os.ReadFile(clientKeyNames[i])
			if err != nil {
				t.Fatalf("Error in reading Client Private key: %v", err.Error())
			}
			clientCert, err := os.ReadFile(clientCertPath)
			if err != nil {
				t.Fatalf("Error in reading : %v", err.Error())
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
			t.Fatalf("Host based authentication failed")
		}

	})

	t.Run("Host Based Authentication", func(t *testing.T) {
		errCount := 0
		for i := 0; i < len(clientKeyNames); i++ {
			clientCertPath := clientKeyNames[i] + "-cert.pub"
			hostCertPath := hostKeyNames[i] + "-cert.pub"

			clientPvtKey, err := os.ReadFile(clientKeyNames[i])
			if err != nil {
				t.Fatalf("Error in reading Client Private key: %v", err.Error())
			}
			clientCert, err := os.ReadFile(clientCertPath)
			if err != nil {
				t.Fatalf("Error in reading : %v", err.Error())
			}
			hostCert, err := os.ReadFile(hostCertPath)
			if err != nil {
				t.Fatalf("Error in reading host certificate: %v", err.Error())
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
			t.Fatalf("Host based authentication failed")
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
	}

	clientKeyNames, _, _ := generateAllSupportedPvtPublicKeyPairs(filePath, false, false)

	credentialsData := createAccountCredentials(filePath, accountName)
	akreq.Credentials = append(akreq.Credentials, &credentialsData)

	stream, err := gnsiC.Credentialz().RotateAccountCredentials(context.Background())
	defer stream.CloseSend()
	if err != nil {
		t.Fatalf("failed to get stream: %v", err)
	}

	roatateAccountCredentialsRequest(t, stream, akreq)

	finalizeAccountRequest(t, stream)

	t.Run("PublicKey Based Authentication", func(t *testing.T) {
		errCount := 0
		for i := 0; i < len(clientKeyNames); i++ {
			pvtKeyBytes, err := os.ReadFile(clientKeyNames[i])
			if err != nil {
				t.Fatalf("Error in parsing certificate file: %v", err)
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
			t.Fatalf("Error in establishing SSH connection with Public Key based")
		}
	})

	var akreqForNeg credz.AuthorizedKeysRequest
	credentials := credz.AccountCredentials{
		Account:   accountName,
		Version:   "1.1",
		CreatedOn: 123}
	akreqForNeg.Credentials = append(akreqForNeg.Credentials, &credentials)

	log.Infof("Removing all authorized keys from user")
	roatateAccountCredentialsRequest(t, stream, akreqForNeg)

	finalizeAccountRequest(t, stream)
	stream.CloseSend()
	t.Run("Remove all authorized keys from user and check ssh", func(t *testing.T) {
		errCount := 0
		for i := 0; i < len(clientKeyNames); i++ {
			pvtKeyBytes, err := os.ReadFile(clientKeyNames[i])
			if err != nil {
				t.Fatalf("Error in parsing certificate file: %v", err)
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
			t.Fatalf("Establishing SSH connection which is not expected after removing authorized keys")
		} else {
			log.Infof("Error occurred while attempting to establish SSH connection as expected after removing authorized keys from the user.")
		}
	})
}

func TestCredentialz_5(t *testing.T) {
	filePath := "hiba"
	accountName := "CERT"
	hostName := "host"
	usersFilePath := fmt.Sprintf("%s/users", filePath)
	hostFilesPath := fmt.Sprintf("%s/hosts", filePath)
	hibaCaPath := "/ws/anidamod-bgl/gNSI_B4/HIBA/hiba/hiba-ca.sh"
	hibaGenPath := "/ws/anidamod-bgl/gNSI_B4/HIBA/hiba/hiba-gen"
	identityFilePath := fmt.Sprintf("%s/policy/identities", filePath)
	identityFileName := "server"
	identityFile := fmt.Sprintf("%s/%s", identityFilePath, identityFileName)
	grantsFilePath := fmt.Sprintf("%s/policy/grants", filePath)
	grantsFileName := "client"
	grantsFile := fmt.Sprintf("%s/%s", grantsFilePath, grantsFileName)

	idKeyPairs := []string{"domain", "net.google.com", "feature", "credz"}

	tartgetIP, tartgetPort, err := getIpAndPortFromBindingFile()
	if err != nil {
		t.Fatalf("Error in reading target IP and Port from Binding file: %v", err)
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
	}

	err = generateKeyPairsUsingHiba(filePath, hibaCaPath, "2048", nil)
	if err != nil {
		t.Fatalf("Error in generating CA key pairs using HIBA: %v", err.Error())
	}

	err = os.MkdirAll(usersFilePath, os.ModePerm)
	if err != nil {
		t.Fatalf("Error creating identity file directory: %v", err.Error())
	}
	err = generateKeyPairsUsingHiba(filePath, hibaCaPath, "2048", &accountName)
	if err != nil {
		t.Fatalf("Error in generating client key pairs using HIBA: %v", err.Error())
	}

	res, err := gnsiC.Credentialz().GetPublicKeys(context.Background(), &credz.GetPublicKeysRequest{})
	if err != nil {
		t.Fatalf("Failed to get public keys = %v", err)
	}
	var hostPubKey string
	for _, pubkey := range res.PublicKeys {
		if pubkey.KeyType == credz.KeyType_KEY_TYPE_RSA_2048 {
			hostPubKey = string(pubkey.PublicKey)
		}
	}

	err = createFileAndWriteData(hostFilesPath, fmt.Sprintf("%s.pub", hostName), hostPubKey)
	if err != nil {
		t.Fatalf("Error in creating host pubkey file: %v", err.Error())
	}

	err = os.MkdirAll(identityFilePath, os.ModePerm)
	if err != nil {
		t.Fatalf("Error creating identity file directory: %v", err.Error())
	}

	err = createAnIdentityFileWithServerName(hibaGenPath, identityFile, idKeyPairs)
	if err != nil {
		t.Fatalf("Error creating identity file with server name: %v", err.Error())
	}
	err = generateHostCertByAttachingIdentities(hibaCaPath, filePath, hostName, identityFileName, "+10d")
	if err != nil {
		t.Fatalf("Error generating host certificate file by attaching identities: %v", err.Error())
	}
	hostCert, err := os.ReadFile(fmt.Sprintf("%s/%s-cert.pub", hostFilesPath, hostName))
	if err != nil {
		t.Fatalf("Error in reading host certificate file: %v", err.Error())
	}

	skreq := credz.ServerKeysRequest{
		AuthArtifacts: []*credz.ServerKeysRequest_AuthenticationArtifacts{
			{Certificate: hostCert}},
		Version:   "1.1",
		CreatedOn: 123,
	}

	hostParamStream, err := gnsiC.Credentialz().RotateHostParameters(context.Background())
	defer hostParamStream.CloseSend()
	if err != nil {
		t.Fatalf("	failed to get stream: %v", err)
	}
	rotateHostParametersRequestForServerKeys(t, hostParamStream, skreq)
	finalizeHostRequest(t, hostParamStream)
	time.Sleep(2 * time.Second)

	caPubkey, err := os.ReadFile(fmt.Sprintf("%s/ca.pub", filePath))
	if err != nil {
		t.Fatalf("Error in reading ca Pubkey file: %v", err.Error())
	}
	caPubkeyReq := credz.CaPublicKeyRequest{
		SshCaPublicKeys: []*credz.PublicKey{{PublicKey: caPubkey, KeyType: credz.KeyType_KEY_TYPE_RSA_2048}},
		Version:         "1.1",
		CreatedOn:       123}

	rotateHostParametersRequestForSshCAPubKey(t, hostParamStream, caPubkeyReq)
	finalizeHostRequest(t, hostParamStream)

	err = createGrantFileWithClientName(hibaGenPath, grantsFile, idKeyPairs)
	if err != nil {
		t.Fatalf("Error creating grants file with client name: %v", err.Error())
	}

	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	err = makeUserEligleForGrants(hibaCaPath, fmt.Sprintf("%s/%s", currentDir, filePath), accountName, grantsFileName)
	if err != nil {
		t.Fatalf("Error in giving grants to client: %v", err.Error())
	}

	err = generateClientCertByAttachingGrants(hibaCaPath, filePath, accountName, grantsFileName, "+10d")
	if err != nil {
		t.Fatalf("Error generating client certificate file by attaching grants: %v", err.Error())
	}

	pcreq := credz.AuthorizedPrincipalCheckRequest{
		Tool: credz.AuthorizedPrincipalCheckRequest_Tool(1),
	}
	rotateHostParametersRequestForprincipalCheck(t, hostParamStream, pcreq)
	finalizeHostRequest(t, hostParamStream)
	time.Sleep(2 * time.Second)

	out, err := createSSHClientWithCmd(tartgetIP, tartgetPort, fmt.Sprintf("%s/%s-cert.pub", usersFilePath, accountName),
		fmt.Sprintf("%s/%s", usersFilePath, accountName), accountName)
	if err != nil {
		t.Fatalf("Error in establishing ssh connection: %v", err.Error())
	}
	log.Infof(out)
	if strings.Contains(out, "rsa-cert") {
		log.Infof("Authentication type of user(%s) is rsa-cert as expected", accountName)
	} else {
		t.Fatalf(fmt.Sprintf("Authentication type of user(%s) is not rsa-cert", accountName))
	}
	pcreq = credz.AuthorizedPrincipalCheckRequest{
		Tool: credz.AuthorizedPrincipalCheckRequest_Tool(0),
	}
	rotateHostParametersRequestForprincipalCheck(t, hostParamStream, pcreq)
	finalizeHostRequest(t, hostParamStream)
}

func TestPubkeyWithProcessRestartAndRPFO(t *testing.T) {
	filePath := "credz"
	accountName := "CREDZ"
	authenticationType := []string{"ecdsa-sha2-nistp256", "ecdsa-sha2-nistp521", "ssh-ed25519", "rsa-pubkey", "rsa-pubkey"}

	tartgetIP, tartgetPort, err := getIpAndPortFromBindingFile()
	if err != nil {
		t.Fatalf("Error in reading target IP and Port from Binding file: %v", err)
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
	}

	clientKeyNames, _, _ := generateAllSupportedPvtPublicKeyPairs(filePath, false, false)

	credentialsData := createAccountCredentials(filePath, accountName)
	akreq.Credentials = append(akreq.Credentials, &credentialsData)

	stream, err := gnsiC.Credentialz().RotateAccountCredentials(context.Background())
	defer stream.CloseSend()
	if err != nil {
		t.Fatalf("failed to get stream: %v", err)
	}

	roatateAccountCredentialsRequest(t, stream, akreq)

	finalizeAccountRequest(t, stream)
	stream.CloseSend()

	sshVerification := func() {
		errCount := 0
		for i := 0; i < len(clientKeyNames); i++ {
			pvtKeyBytes, err := os.ReadFile(clientKeyNames[i])
			if err != nil {
				t.Fatalf("Error in parsing certificate file: %v", err)
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
			t.Fatalf("Error in establishing SSH connection with Public Key based")
		}
	}

	RestartProcess(t, dut, "emsd")
	sshVerification()
	RestartProcess(t, dut, "ssh_conf_proxy")
	sshVerification()
	RestartProcess(t, dut, "cepki")
	sshVerification()
	rpSwitchOver(t, dut)
	sshVerification()

}

func TestExpiredHostCert(t *testing.T) {
	filePath := "credz"
	accountName := "CERT"
	authenticationType := []string{"ecdsa-nistp256-cert", "ecdsa-nistp521-cert", "ed25519-cert", "rsa-cert"}
	hostKeyAlgorithms := []string{"ecdsa-sha2-nistp256-cert-v01@openssh.com",
		"ecdsa-sha2-nistp521-cert-v01@openssh.com", "ssh-ed25519-cert-v01@openssh.com", "ssh-rsa-cert-v01@openssh.com"}

	tartgetIP, tartgetPort, err := getIpAndPortFromBindingFile()
	if err != nil {
		t.Fatalf("Error in reading target IP and Port from Binding file: %v", err)
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
	roatateAccountCredentialsRequestUser(t, stream, authUsrReq)
	finalizeAccountRequest(t, stream)
	stream.CloseSend()
	time.Sleep(2 * time.Second)

	hostParamStream, err := gnsiC.Credentialz().RotateHostParameters(context.Background())
	defer hostParamStream.CloseSend()
	if err != nil {
		t.Fatalf("	failed to get stream: %v", err)
	}

	rotateHostParametersRequestForSshCAPubKey(t, hostParamStream, caPubkeyReq)
	finalizeHostRequest(t, hostParamStream)

	for i := 0; i < len(clientKeyNames); i++ {
		_, authUsr := filepath.Split(clientKeyNames[i])
		generateClientCertificates(caKeyNames[i], authUsr, clientKeyNames[i])
		generateHostCertificates(caKeyNames[i], hostKeyNames[i], "20100101123000:20110101123000")
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
			t.Fatalf("Error in reading host private key file: %v", err.Error())
		}
		hostcertBytes, err := os.ReadFile(hostCertFile)
		if err != nil {
			t.Fatalf("Error in reading host certificate file: %v", err.Error())
		}
		auArtifacts := &credz.ServerKeysRequest_AuthenticationArtifacts{PrivateKey: hostPvtKeyBytes, Certificate: hostcertBytes}
		authArtifacts = append(authArtifacts, auArtifacts)
	}
	skreq.AuthArtifacts = authArtifacts

	rotateHostParametersRequestForServerKeys(t, hostParamStream, skreq)
	finalizeHostRequest(t, hostParamStream)
	hostParamStream.CloseSend()
	time.Sleep(2 * time.Second)

	errCount := 0
	for i := 0; i < len(clientKeyNames); i++ {
		clientCertPath := clientKeyNames[i] + "-cert.pub"
		hostCertPath := hostKeyNames[i] + "-cert.pub"

		clientPvtKey, err := os.ReadFile(clientKeyNames[i])
		if err != nil {
			t.Fatalf("Error in reading Client Private key: %v", err.Error())
		}
		clientCert, err := os.ReadFile(clientCertPath)
		if err != nil {
			t.Fatalf("Error in reading : %v", err.Error())
		}
		hostCert, err := os.ReadFile(hostCertPath)
		if err != nil {
			t.Fatalf("Error in reading host certificate: %v", err.Error())
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
		t.Fatalf("Verified all types of encryption algorithms for Host based authentication successfully, which is not expected")
	} else {
		log.Infof("Host based authentication failed, which is expected")
	}
}

// func TestClientCertWithProcessRestartAndRPFO(t *testing.T) {
// 	filePath := "credz"
// 	accountName := "CERT"
// 	authenticationType := []string{"ecdsa-nistp256-cert", "ecdsa-nistp521-cert", "ed25519-cert", "rsa-cert"}

// 	tartgetIP, tartgetPort, err := getIpAndPortFromBindingFile()
// 	if err != nil {
// 		t.Fatalf("Error in reading target IP and Port from Binding file: %v", err)
// 	}

// 	var users []*oc.System_Aaa_Authentication_User
// 	users = append(users, &oc.System_Aaa_Authentication_User{
// 		Username: &accountName,
// 		Role:     oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN,
// 	})

// 	dut := ondatra.DUT(t, "dut")
// 	createUsersOnDevice(t, dut, users)

// 	gnsiC := dut.RawAPIs().GNSI(t)

// 	mkdirErr := os.Mkdir(filePath, 0755)
// 	defer os.RemoveAll(filePath)
// 	if mkdirErr != nil {
// 		t.Fatalf("Error creating directory: %v", mkdirErr)
// 	}
// 	clientKeyNames, caKeyNames, hostKeyNames := generateAllSupportedPvtPublicKeyPairs(filePath, true, true)
// 	clientKeyNames = clientKeyNames[:len(clientKeyNames)-1]
// 	caKeyNames = caKeyNames[:len(caKeyNames)-1]
// 	hostKeyNames = hostKeyNames[:len(hostKeyNames)-1]
// 	var authUsrReq credz.AuthorizedUsersRequest

// 	policyReq, caPubkeyReq := createAccountPoliciesAndPrincipals(accountName, filePath)
// 	authUsrReq.Policies = append(authUsrReq.Policies, &policyReq)

// 	stream, err := gnsiC.Credentialz().RotateAccountCredentials(context.Background())
// 	if err != nil {
// 		t.Fatalf("failed to get stream: %v", err)
// 	}
// 	roatateAccountCredentialsRequestUser(t, stream, authUsrReq)
// 	finalizeAccountRequest(t, stream)
// 	stream.CloseSend()
// 	time.Sleep(2 * time.Second)

// 	hostParamStream, err := gnsiC.Credentialz().RotateHostParameters(context.Background())
// 	defer hostParamStream.CloseSend()
// 	if err != nil {
// 		t.Fatalf("	failed to get stream: %v", err)
// 	}

// 	rotateHostParametersRequestForSshCAPubKey(t, hostParamStream, caPubkeyReq)
// 	finalizeHostRequest(t, hostParamStream)

// 	for i := 0; i < len(clientKeyNames); i++ {
// 		_, authUsr := filepath.Split(clientKeyNames[i])
// 		generateClientCertificates(caKeyNames[i], authUsr, clientKeyNames[i])
// 		generateHostCertificates(caKeyNames[i], hostKeyNames[i], "+1d")
// 	}

// 	var skreq credz.ServerKeysRequest
// 	var authArtifacts []*credz.ServerKeysRequest_AuthenticationArtifacts

// 	skreq = credz.ServerKeysRequest{
// 		AuthArtifacts: []*credz.ServerKeysRequest_AuthenticationArtifacts{},
// 		Version:       "1.1",
// 		CreatedOn:     123,
// 	}

// 	for i := 0; i < len(clientKeyNames); i++ {
// 		hostCertFile := hostKeyNames[i] + "-cert.pub"

// 		hostPvtKeyBytes, err := os.ReadFile(hostKeyNames[i])
// 		if err != nil {
// 			t.Fatalf("Error in reading host private key file: %v", err.Error())
// 		}
// 		hostcertBytes, err := os.ReadFile(hostCertFile)
// 		if err != nil {
// 			t.Fatalf("Error in reading host certificate file: %v", err.Error())
// 		}
// 		auArtifacts := &credz.ServerKeysRequest_AuthenticationArtifacts{PrivateKey: hostPvtKeyBytes, Certificate: hostcertBytes}
// 		authArtifacts = append(authArtifacts, auArtifacts)
// 	}
// 	skreq.AuthArtifacts = authArtifacts

// 	rotateHostParametersRequestForServerKeys(t, hostParamStream, skreq)
// 	finalizeHostRequest(t, hostParamStream)
// 	time.Sleep(2 * time.Second)

// 	sshVerification := func() {
// 		errCount := 0
// 		for i := 0; i < len(clientKeyNames); i++ {
// 			clientCertPath := clientKeyNames[i] + "-cert.pub"

// 			clientPvtKey, err := os.ReadFile(clientKeyNames[i])
// 			if err != nil {
// 				t.Fatalf("Error in reading Client Private key: %v", err.Error())
// 			}
// 			clientCert, err := os.ReadFile(clientCertPath)
// 			if err != nil {
// 				t.Fatalf("Error in reading : %v", err.Error())
// 			}
// 			sshParams := sshClientParams{
// 				user:         accountName,
// 				clientPvtKey: clientPvtKey,
// 				clientCert:   clientCert,
// 			}
// 			err = createSSHClientAndVerify(tartgetIP, tartgetPort, sshParams, authenticationType[i], nil)
// 			if err != nil {
// 				errCount = errCount + 1
// 				log.Error("Error in establishing ssh connection: ", err.Error())
// 			}
// 		}
// 		if errCount == 0 {
// 			log.Infof("Verified all types of encryption algorithms for Host based authentication successfully")
// 		} else {
// 			t.Fatalf("Host based authentication failed")
// 		}
// 	}
// 	RestartProcess(t, dut, "emsd")
// 	sshVerification()
// 	RestartProcess(t, dut, "ssh_conf_proxy")
// 	sshVerification()
// 	RestartProcess(t, dut, "cepki")
// 	sshVerification()
// 	rpSwitchOver(t, dut)
// 	sshVerification()

// }
