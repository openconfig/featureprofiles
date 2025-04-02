// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package bootz implement tests  authz-14.
package bootz

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/bootz/dhcp"
	dhcpLease "github.com/openconfig/bootz/dhcp/plugins/slease"
	"github.com/openconfig/bootz/server/entitymanager/proto/entity"
	"github.com/openconfig/bootz/server/service"
	perf "github.com/openconfig/featureprofiles/feature/cisco/performance"
	"github.com/openconfig/featureprofiles/internal/cisco/ha/utils"
	"github.com/openconfig/featureprofiles/internal/cisco/security/pathz"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/security/authz"
	"github.com/openconfig/featureprofiles/internal/security/gnxi"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/ygot/ygot"
	"github.com/povsister/scp"

	// perf "github.com/openconfig/featureprofiles/feature/cisco/performance"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	bcpb "github.com/openconfig/gnoi/bootconfig"
	spb "github.com/openconfig/gnoi/system"
	authzpb "github.com/openconfig/gnsi/authz"
	certzpb "github.com/openconfig/gnsi/certz"
	credz "github.com/openconfig/gnsi/credentialz"
	pathzpb "github.com/openconfig/gnsi/pathz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding/introspect"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"golang.org/x/crypto/ssh"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	log "github.com/golang/glog"
	ov "github.com/openconfig/bootz/common/ownership_voucher"
	bootzSever "github.com/openconfig/bootz/server"
	bootzem "github.com/openconfig/bootz/server/entitymanager"

	bpb "github.com/openconfig/bootz/proto/bootz"
)

var (
	dhcpIntf        = flag.String("dhcp-intf", "ens224", "Interface that will be used by dhcp server to listen for dhcp requests")
	bootzAddr       = flag.String("bootz_addr", "5.38.4.124:15006", "The ip:port to start the Bootz server. Ip must be specefied and be reachable from the router.")
	imageServerAddr = flag.String("img_serv_addr", "5.38.4.124:15007", "The ip:port to start the Image server. Ip must be specefied and be reachable from the router.")
	imagesDir       = flag.String("img_dir", "/var/www/html/", "Directory where the images will be located.")
	imageVersion    = flag.String("img_ver", "25.3.1.09I", "Version of the image to be loaded using bootz")
	dhcpIP          = flag.String("dhcp_ip", "5.38.9.29/16", "IP address in CIDR format that dhcp server will assign to the dut.")
	dhcpGateway     = flag.String("dhcp_gateway", "5.38.0.1", "Gateway IP that dhcp server will assign to DUT.")
	testInfraID     = flag.String("test_infra_id", "cafyauto", "SPIFFE-ID used by test Infra ID user for authz operation")
)

var (
	controlcardType       = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	chassisType           = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS
	isSetupDone           = false
	chassisSerial         = ""
	controllerCards       = []string{}
	controllerCardSerials = []string{}
	chassisBootzConfig    = &entity.Chassis{}
	hwAddrs               = []string{}
	secArtifacts          = &service.SecurityArtifacts{}
	em                    *bootzem.InMemoryEntityManager
	bServer               *bootzSever.Server
	chassisEntity         = &service.EntityLookup{}
	createdtime           = uint64(time.Now().UnixMicro())
	baseConfig            string
	bootzServerFailed     atomic.Bool
)

type NsrState struct {
	NSRState string `json:"nsr-state"`
}
type TestSetupConfig struct {
	IncludeCredentials bool
	IncludePathz       bool
}
type clientConfig interface {
	createClientConfig() (ssh.ClientConfig, error)
}
type AuthenticationError struct {
	message string
}
type TestSetupOption func(*TestSetupConfig)
type bootzTest struct {
	Name            string
	VendorConfig    string
	OV              []byte
	Image           *bpb.SoftwareImage
	ExpectedFailure bool
}
type targetInfo struct {
	dut     string
	sshIp   string
	sshPort int
	sshUser string
	sshPass string
}
type sshPubkeyParams struct {
	user       string
	privateKey []byte
}
type sshPasswordParams struct {
	user     string
	password string
}

const (
	dhcpTimeout                = 60 * time.Minute // connection to dhcp after factory default
	bootzConnectionTimeout     = 10 * time.Minute // request for bootstrap after dhcp
	bootzStatusTimeout         = 60 * time.Minute // only ov + config
	fullBootzCompletionTimeout = 90 * time.Minute // image + ov + config
	lastBootAttemptTimeDiff    = 10 * time.Minute
	ipv4PrefixLen              = 30
	ateDstNetCIDR              = "203.0.113.0/24"
	ateDstNetStartIp           = "203.0.113.1"
	nhIndex                    = 1
	nhgIndex                   = 42
	flowName                   = "Flow"
	credzFileName              = "credz"
	credzAccName               = "CREDZ"
	myStationMAC               = "00:1A:11:00:00:01"
	profileID                  = "CERTZ_BOOTZ"
	InterfaceName              = "Bundle-Ether1"
	pbrName                    = "PBR"
	NetworkInstanceDefault     = "DEFAULT"
	lc                         = "0/0/CPU0"
	active_rp                  = "0/RP0/CPU0"
	standby_rp                 = "0/RP1/CPU0"
)

var allowAll = `
	"allow_rules": [{
		"name": "allow_all",
		"source": { "principals": ["*"] },
		"request": { "paths": [ "*" ] }
	}]`
var allowAllServicesNoPrincipalFilter = `{
	"name": "authz",` + allowAll + `
}`

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
func (e *AuthenticationError) Error() string {
	return e.message
}
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// WithoutCredentialz sets IncludeCredentials to false
func WithoutCredentialz() TestSetupOption {
	return func(cfg *TestSetupConfig) {
		cfg.IncludeCredentials = false
	}
}

// WithoutPathz sets IncludePathz to false
func WithoutPathz() TestSetupOption {
	return func(cfg *TestSetupConfig) {
		cfg.IncludePathz = false
	}
}
func checkBootzStatus(t *testing.T, expectFailure bool, timeout time.Duration) {
	if bootzServerFailed.Load() {
		t.Fatal("bootz server is down, check the test log for detailed error")
	}
	for _, ccSerial := range controllerCardSerials {
		err := awaitBootzStatus(ccSerial, bpb.ReportStatusRequest_BOOTSTRAP_STATUS_INITIATED, timeout)
		if err != nil {
			t.Errorf("ReportStatusRequest_BOOTSTRAP_STATUS_INITIATED in not reported in %d minutes for controller card %s", timeout, ccSerial)
		} else {
			t.Log("DUT reported ReportStatusRequest_BOOTSTRAP_STATUS_INITIATED to bootz server as expected")
		}
	}
	expectedCCstatus := bpb.ReportStatusRequest_BOOTSTRAP_STATUS_SUCCESS
	if expectFailure {
		expectedCCstatus = bpb.ReportStatusRequest_BOOTSTRAP_STATUS_FAILURE
	}
	for _, ccSerial := range controllerCardSerials {
		err := awaitBootzStatus(ccSerial, expectedCCstatus, timeout)
		if err != nil {
			t.Errorf("Status %s is not reported as expected in %d minutes", expectedCCstatus.String(), timeout)
		} else {
			t.Logf("DUT reported %s to bootz server as expected", expectedCCstatus.String())
		}
	}
}

func dutBootzStatus(t *testing.T, dut *ondatra.DUTDevice, maxRebootTime time.Duration) {
	startReboot := time.Now()
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f minutes since reboot started.", time.Since(startReboot).Minutes())
		time.Sleep(1 * time.Minute)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("Device bootzed successfully with received time: %v", currentTime)
			break
		}

		if time.Since(startReboot) > maxRebootTime {
			t.Logf("Check bootz time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
			break
		}
	}
	t.Logf("Device bootz time: %.2f minutes", time.Since(startReboot).Minutes())
	//TODO: add oc leaves check
}

func testSetup(t *testing.T, dut *ondatra.DUTDevice, opts ...TestSetupOption) {
	// Default configuration
	cfg := &TestSetupConfig{
		IncludeCredentials: true,
		IncludePathz:       true, // Default to true
	}

	// Apply options
	for _, opt := range opts {
		opt(cfg)
	}

	if !isSetupDone {
		baseConfig = getBaseConfig(t, dut)
		comps := components.FindComponentsByType(t, dut, chassisType)
		if len(comps) != 1 {
			t.Fatalf("Could not find the chassis in component list")
		}
		chassisSerial = gnmi.Get(t, dut, gnmi.OC().Component(comps[0]).SerialNo().State())
		t.Logf("Chassis Serial: %s", chassisSerial)

		controllerCards = components.FindComponentsByType(t, dut, controlcardType)
		for _, comp := range controllerCards {
			ccSerial := gnmi.Get(t, dut, gnmi.OC().Component(comp).SerialNo().State())
			controllerCardSerials = append(controllerCardSerials, ccSerial)
		}

		for _, ports := range dut.Ports() {
			// We assume the ports in binding are management interfaces, otherwise the test must fail.
			mac := gnmi.Get(t, dut, gnmi.OC().Interface(ports.Name()).Ethernet().HwMacAddress().State())
			if !gnmi.Get(t, dut, gnmi.OC().Interface(ports.Name()).Management().State()) {
				t.Fatalf("Ports are expected to be management interfaces")
			}
			hwAddrs = append(hwAddrs, mac)
		}
		isSetupDone = true
	}

	loadSecArtifacts(t, "testdata/pdc.cert.pem", "testdata/pdc.key.pem")
	var err error
	em, err = bootzem.New("", secArtifacts)
	if err != nil {
		t.Fatalf("Could not initialize bootz inventory: %v", err)
	}
	prepareBootzConfig(t, dut)
	startBootzSever(t, cfg.IncludeCredentials, cfg.IncludePathz)

	err = startDhcpServer(*dhcpIntf, em, *bootzAddr)
	if err != nil {
		t.Fatalf("Could not start dhcp server on interface %s, err: %v", *dhcpIntf, err)
	}
}

// loadOV load ovs from a specified file
func loadOV(t *testing.T, serialNumber string, pdc *x509.Certificate, verify bool) []byte {
	ovPath := fmt.Sprintf("../cisco/tests/testdata/%s.ov", serialNumber)
	ovByte, err := os.ReadFile(ovPath)
	if err != nil {
		t.Fatalf("Error opening key file %v", err)
	}
	parsedOV, err := ov.Unmarshal(ovByte, nil)
	if err != nil {
		t.Fatalf("unable to verify ownership voucher: %v", err)
	}

	// Verify the serial number for this OV
	t.Logf("Verifying the serial number for OV")
	got := parsedOV.OV.SerialNumber
	want := serialNumber
	if got != want {
		if verify {
			t.Fatalf("Serial number from OV does not match requested Serial Number, want %v, got %v", serialNumber, got)
		}
	}

	// ensure the cert in ov is valid.
	_, err = x509.ParseCertificate(parsedOV.OV.PinnedDomainCert)
	if err != nil {
		t.Fatalf("Unable to parse PDC DER to x509 certificate: %v", err)
	}
	if string(pdc.Raw) != string(parsedOV.OV.PinnedDomainCert) {
		if verify {
			t.Fatalf("The PDC from the ov does not match the expected pdc")
		}
	}
	return ovByte
}

// load sec artifacts
func loadSecArtifacts(t *testing.T, pdcCertPEM, pdcKeyPEM string) {
	serials := []string{chassisSerial}
	if len(controllerCardSerials) >= 1 { // modular chassis
		serials = controllerCardSerials
	}
	pdcKey, pdcCert := loadKeyPair(t, pdcKeyPEM, pdcCertPEM)
	os.Remove("testdata/tls.key.pem")
	os.Remove("testdata/tls.cert.pem")

	ip := extractIP(*bootzAddr)
	// ip := strings.Split(*bootzAddr, ":")[0]
	anchorCert := generateCert(t, pdcCert, pdcKey, ip, "bootz server")

	sa := &service.SecurityArtifacts{
		PDC:                 pdcCert,
		OwnerCert:           pdcCert,
		OwnerCertPrivateKey: pdcKey,
		TLSKeypair:          anchorCert,
		OV:                  service.OVList{},
		TrustAnchor:         pdcCert,
	}
	for _, serial := range serials {
		sa.OV[serial] = loadOV(t, serial, pdcCert, true)
	}
	secArtifacts = sa
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
		log.Infof("%v", string(publicKeyBytes))
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
	index := strings.LastIndex(target, ":")
	targetIP := target[:index]
	targetPort, _ := strconv.Atoi(target[index+1:])
	return targetIP, targetPort, nil
}
func createUsersOnDevice(t *testing.T, dut *ondatra.DUTDevice, users []*oc.System_Aaa_Authentication_User) {
	ocAuthentication := &oc.System_Aaa_Authentication{}
	for _, user := range users {
		ocAuthentication.AppendUser(user)
	}
	gnmi.Update(t, dut, gnmi.OC().System().Aaa().Authentication().Config(), ocAuthentication)
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

func createAuthErr(message string) error {
	return &AuthenticationError{message: message}
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
func prepareBootzConfig(t *testing.T, dut *ondatra.DUTDevice) {
	caser := cases.Title(language.English)
	chassisEntity = &service.EntityLookup{SerialNumber: chassisSerial, Manufacturer: caser.String(dut.Vendor().String())}

	// Extract SSH username
	bindingFile := flag.Lookup("binding").Value.String()
	in, err := os.ReadFile(bindingFile)
	if err != nil {
		t.Fatalf("unable to read binding file")
	}

	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		t.Fatalf("unable to parse binding file")
	}

	// Define sshUser outside the loop to avoid scope issues
	var sshUser string
	for _, dut := range b.Duts {
		if dut.Ssh.Username != "" {
			sshUser = dut.Ssh.Username
		} else if dut.Options.Username != "" {
			sshUser = dut.Options.Username
		} else {
			sshUser = b.Options.Username
		}
	}

	certPEM, err := os.ReadFile("certificates/server_cert.pem")
	if err != nil {
		fmt.Println("Failed to read cert file", err)
	}
	privKeyPEM, err := os.ReadFile("certificates/server_key.pem")
	if err != nil {
		fmt.Println("Failed to read key file", err)
	}

	var passreq credz.PasswordRequest
	passreq.Accounts = append(passreq.Accounts, &credz.PasswordRequest_Account{Account: "gfallback", Password: &credz.PasswordRequest_Password{Value: &credz.PasswordRequest_Password_Plaintext{Plaintext: "test123"}}, Version: "1.1", CreatedOn: createdtime})
	passwordHash2 := credz.PasswordRequest_CryptoHash{
		HashType:  credz.PasswordRequest_CryptoHash_HASH_TYPE_CRYPT_SHA_2_512,
		HashValue: "$6$NIaYs1Z77yGWDs1.$7sdcm8XY1NkXpJ1kLC/TQFzQZ.3oqrOsB7zE00ukJzYfmHybY6APRFauRd9XuaR6.fSC/q6VwWjDzYIq4Bpg21"}
	passreq.Accounts = append(passreq.Accounts, &credz.PasswordRequest_Account{
		Account: "gfallback2",
		Password: &credz.PasswordRequest_Password{
			Value: &credz.PasswordRequest_Password_CryptoHash{
				CryptoHash: &passwordHash2}},
		Version: "1.1", CreatedOn: createdtime})

	var credentials bpb.Credentials
	var akreq credz.AuthorizedKeysRequest
	credentialsData := createAccountCredentials(credzFileName, credzAccName)
	akreq.Credentials = append(akreq.Credentials, &credentialsData)
	credentials.Credentials = append(credentials.Credentials, &akreq)
	// credentials.Passwords = append(credentials.Passwords, &passreq)

	chassisBootzConfig = &entity.Chassis{
		SerialNumber:  chassisSerial,
		SoftwareImage: nil,
		Manufacturer:  caser.String(dut.Vendor().String()),
		BootMode:      bpb.BootMode_BOOT_MODE_SECURE,
		Config: &entity.Config{
			BootConfig: &entity.BootConfig{
				VendorConfig: []byte(getBaseConfig(t, dut)),
			},
			GnsiConfig: &entity.GNSIConfig{
				AuthzUpload: &authzpb.UploadRequest{
					Version:   "0.0",
					CreatedOn: createdtime,
					Policy:    allowAllServicesNoPrincipalFilter, // TODO: add authz policy here
				},
				PathzUpload: &pathzpb.UploadRequest{
					Version: "1",
					Policy: &pathzpb.AuthorizationPolicy{
						Rules: []*pathzpb.AuthorizationRule{{
							Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
							Principal: &pathzpb.AuthorizationRule_User{User: sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						}},
					},
				},
				CertzUpload: &certzpb.UploadRequest{
					Entities: []*certzpb.Entity{
						{
							Version:   "1.0",
							CreatedOn: 123456789,
							Entity: &certzpb.Entity_CertificateChain{
								CertificateChain: &certzpb.CertificateChain{
									Certificate: &certzpb.Certificate{
										Type:            certzpb.CertificateType_CERTIFICATE_TYPE_X509,
										Encoding:        certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
										CertificateType: &certzpb.Certificate_RawCertificate{RawCertificate: certPEM},
										PrivateKeyType:  &certzpb.Certificate_RawPrivateKey{RawPrivateKey: privKeyPEM},
									},
								},
							},
						},
					},
				},
				Credentials: &credentials,
			},
		},
		DhcpConfig: &entity.DHCPConfig{
			HardwareAddress: chassisSerial,
			IpAddress:       *dhcpIP,
			Gateway:         *dhcpGateway,
		},
	}

	// ✅ Print DHCP Configuration
	fmt.Println("\n **DHCP Configuration**")
	fmt.Printf("  ↳ Hardware Address: %s\n", chassisBootzConfig.DhcpConfig.HardwareAddress)
	fmt.Printf("  ↳ IP Address: %s\n", chassisBootzConfig.DhcpConfig.IpAddress)
	fmt.Printf("  ↳ Gateway: %s\n", chassisBootzConfig.DhcpConfig.Gateway)

	for i, cc := range controllerCardSerials {
		ccConfig := &entity.ControlCard{
			SerialNumber: cc,
			DhcpConfig: &entity.DHCPConfig{
				HardwareAddress: hwAddrs[i],
				IpAddress:       *dhcpIP,
				Gateway:         *dhcpGateway,
			},
		}
		chassisBootzConfig.ControllerCards = append(chassisBootzConfig.ControllerCards, ccConfig)
	}

	// Print Controller Cards Configuration
	fmt.Println("\n **Controller Cards Configuration**")
	for _, cc := range chassisBootzConfig.ControllerCards {
		fmt.Printf("  ↳ Controller Serial: %s\n", cc.SerialNumber)
		fmt.Printf("     ↳ Hardware Address: %s\n", cc.DhcpConfig.HardwareAddress)
		fmt.Printf("     ↳ IP Address: %s\n", cc.DhcpConfig.IpAddress)
		fmt.Printf("     ↳ Gateway: %s\n", cc.DhcpConfig.Gateway)
	}
	em.ReplaceDevice(chassisEntity, chassisBootzConfig)
}
func startBootzSever(t *testing.T, includeCredentials bool, includePathz bool) {
	err := em.ReplaceDevice(chassisEntity, chassisBootzConfig)
	if err != nil {
		t.Fatalf("Could not add chassis config to entitymanager config: %v", err)
	}
	imgaeServOpts := &bootzSever.ImgSrvOpts{
		ImagesLocation: *imagesDir,
		Address:        *imageServerAddr,
		CertFile:       "testdata/tls.cert.pem",
		KeyFile:        "testdata/tls.key.pem",
	}
	interceptorConfig := InterceptorConfig{
		IncludeCredentials: includeCredentials,
		IncludePathz:       includePathz,
	}
	interceptor := &bootzSever.InterceptorOpts{
		BootzInterceptor: bootzInterceptor(interceptorConfig),
	}
	bServer, err = bootzSever.NewServer(*bootzAddr, em, secArtifacts, imgaeServOpts, interceptor)
	if err != nil {
		t.Fatalf("Could not initiate bootz server %v", err)
	}
	bootzServerFailed.Store(false)
	go func() {
		err := bServer.Start()
		if err != nil {
			t.Logf("Unexpected Bootz server error %v, test will be terminated ASAP", err)
			bootzServerFailed.Store(true)
		}
	}()
}
func setupAndRotateCertificates(t *testing.T, dut *ondatra.DUTDevice) []string {
	// Credz setup
	profileID := "CERTZ_BOOTZ"

	// Generate client key pairs
	clientKeyNames, _, _ := generateAllSupportedPvtPublicKeyPairs(credzFileName, false, false)

	// Read certificate and private key
	certPEM, err := os.ReadFile("certificates/server_cert.pem")
	if err != nil {
		t.Fatalf("Failed to read cert file: %s", err)
	}
	privKeyPEM, err := os.ReadFile("certificates/server_key.pem")
	if err != nil {
		t.Fatalf("Failed to read key file: %s", err)
	}

	// Read CA certificates
	certificates, err := readCertificatesFromFile("certificates/ca.cert.pem")
	if err != nil {
		t.Fatalf("Failed to read CA certificates: %s", err)
	}

	// Convert CA certificates into a chain
	var certChainMessage certzpb.CertificateChain
	for i, cert := range certificates {
		certMessage := &certzpb.Certificate{
			Type:            certzpb.CertificateType_CERTIFICATE_TYPE_X509,
			Encoding:        certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
			CertificateType: &certzpb.Certificate_RawCertificate{RawCertificate: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})},
		}
		if i > 0 {
			certChainMessage.Parent = &certzpb.CertificateChain{
				Certificate: certMessage,
				Parent:      certChainMessage.Parent,
			}
		} else {
			certChainMessage = certzpb.CertificateChain{Certificate: certMessage}
		}
	}

	// Prepare RotateCertificateRequest
	certCreatedTime := uint64(time.Now().Add(-10 * time.Second).Unix())
	version := "1.0"
	request := &certzpb.RotateCertificateRequest{
		ForceOverwrite: true,
		SslProfileId:   profileID,
		RotateRequest: &certzpb.RotateCertificateRequest_Certificates{
			Certificates: &certzpb.UploadRequest{
				Entities: []*certzpb.Entity{
					{
						Version:   version,
						CreatedOn: certCreatedTime,
						Entity: &certzpb.Entity_CertificateChain{
							CertificateChain: &certzpb.CertificateChain{
								Certificate: &certzpb.Certificate{
									Type:            certzpb.CertificateType_CERTIFICATE_TYPE_X509,
									Encoding:        certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
									CertificateType: &certzpb.Certificate_RawCertificate{RawCertificate: certPEM},
									PrivateKeyType:  &certzpb.Certificate_RawPrivateKey{RawPrivateKey: privKeyPEM},
								},
							},
						},
					},
					{
						Version:   version,
						CreatedOn: certCreatedTime,
						Entity: &certzpb.Entity_TrustBundle{
							TrustBundle: &certChainMessage,
						},
					},
				},
			},
		},
	}

	// Get gNSI Client
	gnsiC := dut.RawAPIs().GNSI(t)

	// Start certificate rotation
	stream, err := gnsiC.Certz().Rotate(context.Background())
	if err != nil {
		t.Fatalf("Failed to get stream: %s", err)
	}
	if err = stream.Send(request); err != nil {
		t.Fatalf("Failed to send RotateRequest: %s", err)
	}
	response, err := stream.Recv()
	if err != nil {
		t.Fatalf("Failed to receive RotateCertificateResponse: %s", err)
	}
	t.Logf("Rotate successful %v", response)

	// Finalize rotation
	finalizeRequest := &certzpb.RotateCertificateRequest{
		ForceOverwrite: true,
		SslProfileId:   profileID,
		RotateRequest:  &certzpb.RotateCertificateRequest_FinalizeRotation{FinalizeRotation: &certzpb.FinalizeRequest{}},
	}
	if err := stream.Send(finalizeRequest); err != nil {
		t.Fatalf("Failed to send Finalize Rotation: %s", err)
	}
	if _, err = stream.Recv(); err != nil && err != io.EOF {
		t.Fatalf("Failed, finalize Rotation is cancelled: %s", err)
	}
	stream.CloseSend()

	// Apply gNSI CLI config
	configToChange := fmt.Sprintf("grpc gnsi service certz ssl-profile-id %s \n", profileID)
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	// Return generated client keys
	return clientKeyNames
}
func roatateAccountCredentialsForPassword(t *testing.T, stream credz.Credentialz_RotateAccountCredentialsClient, passreq credz.PasswordRequest) {

	log.Infof("Send Rotate Account Credentisls Request ")
	err := stream.Send(&credz.RotateAccountCredentialsRequest{Request: &credz.RotateAccountCredentialsRequest_Password{Password: &passreq}})
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
func finalizeAccountRequest(t *testing.T, stream credz.Credentialz_RotateAccountCredentialsClient) {
	log.Infof("Send Finalize Request")
	err := stream.Send(&credz.RotateAccountCredentialsRequest{Request: &credz.RotateAccountCredentialsRequest_Finalize{}})
	if err != nil {
		t.Fatalf("Stream send finalize failed : " + err.Error())
	}
	if _, err = stream.Recv(); err != nil {
		if err != io.EOF {
			log.Exit("Failed, finalize Rotation is cancelled", err)
		}
	}
	log.Infof("Finalize Request done")
}

// ### bootz-1: Validate minimum necessary bootz configuration
func TestBootz1(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Create directory
	mkdirErr := os.Mkdir(credzFileName, 0755)
	defer os.RemoveAll(credzFileName)
	if mkdirErr != nil {
		t.Fatalf("Error creating directory: %v", mkdirErr)
	}

	// Credz code
	accountName := "CREDZ"

	// Create users on the device
	var users []*oc.System_Aaa_Authentication_User
	users = append(users, &oc.System_Aaa_Authentication_User{
		Username: &accountName,
		Role:     oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN,
	})
	createUsersOnDevice(t, dut, users)

	authenticationType := []string{"ecdsa-sha2-nistp256", "ecdsa-sha2-nistp521", "ssh-ed25519", "rsa-pubkey", "rsa-pubkey"}

	targetIP, targetPort, err := getIpAndPortFromBindingFile()
	if err != nil {
		t.Fatalf("Error in reading target IP and Port from Binding file: %v", err)
	}

	gnsiC := dut.RawAPIs().GNSI(t)
	profileID := "CERTZ_BOOTZ"

	// Check if profile already exists
	profilesList, err := gnsiC.Certz().GetProfileList(context.Background(), &certzpb.GetProfileListRequest{})
	t.Logf("Available ProfilesList: %v", profilesList)
	if err != nil {
		t.Logf("Failed to list profiles got : %v", err)
	}

	profileExists := false
	for _, profile := range profilesList.GetSslProfileIds() {
		if profile == profileID {
			profileExists = true
			break
		}
	}

	// Delete & Add profile only if it does NOT exist
	if !profileExists {
		profiles, err := gnsiC.Certz().AddProfile(context.Background(), &certzpb.AddProfileRequest{SslProfileId: profileID})
		if err != nil {
			t.Fatalf("Unexpected Error in adding profile list: %v", err)
		}
		t.Logf("Profile add successful: %v", profiles)
	} else {
		t.Logf("Profile %s already exists. Skipping AddProfile.", profileID)
	}

	// Call setupAndRotateCertificates after profile check
	clientKeyNames := setupAndRotateCertificates(t, dut)

	testSetup(t, dut)
	defer bServer.Stop()
	defer dhcp.Stop()

	dutPreTestVersion := gnmi.Get(t, dut, gnmi.OC().System().SoftwareVersion().State())
	bootzStarted := false

	bootz1 := []bootzTest{
		{
			Name:            "Bootz-1.1: Missing config",
			VendorConfig:    "",
			ExpectedFailure: true,
		},
		{
			Name:            "Bootz-1.2: Invalid config",
			VendorConfig:    "invalid config",
			ExpectedFailure: true,
		},
		{
			Name:            "Bootz-1.3: Valid config",
			VendorConfig:    baseConfig,
			ExpectedFailure: false,
		},
	}
	t.Run("Running Bootz1 Test to Validate minimum necessary bootz configuration", func(t *testing.T) {
		for _, tt := range bootz1 {
			t.Run(tt.Name, func(t *testing.T) {
				if bootzServerFailed.Load() {
					t.Fatal("bootz server is down, check the test log for detailed error")
				}
				// reset bootz logs
				bootzStatusLogs = bootzStatus{}
				bootzReqLogs = bootzLogs{}
				//ensure no old dhcp log causing an issue
				dhcpLease.CleanLog()

				chassisBootzConfig.GetConfig().BootConfig.VendorConfig = []byte(tt.VendorConfig)
				em.ReplaceDevice(chassisEntity, chassisBootzConfig)
				if !bootzStarted {
					factoryReset(t, dut)
					bootzStarted = true
				}
				dhcpIDs := []string{chassisSerial}
				dhcpIDs = append(dhcpIDs, hwAddrs...)
				err := awaitDHCPCompletion(dhcpIDs, dhcpTimeout)
				if err != nil {
					t.Errorf("DUT connection to DHCP server was not successful in %d minutes", dhcpTimeout)
				} else {
					t.Logf("DUT connection to DHCP server was  successful")
				}
				err = awaitBootzConnection(*chassisEntity, bootzConnectionTimeout)
				if err != nil {
					t.Errorf("DUT connection to bootz server was not successful in %d minutes", bootzConnectionTimeout)
				} else {
					t.Log("DUT is connected to bootz server")
				}
				checkBootzStatus(t, tt.ExpectedFailure, bootzStatusTimeout)
			})
		}
		dutBootzStatus(t, dut, 5*time.Second)
		dutPostTestVersion := gnmi.Get(t, dut, gnmi.OC().System().SoftwareVersion().State())
		if dutPreTestVersion != dutPostTestVersion {
			t.Fatalf("DUT software versions do not match, pretest: %s , posttest: %s ", dutPreTestVersion, dutPostTestVersion)
		}
	})

	// Validate Pathz Behaviour
	gNSIvalidation(t, dut, clientKeyNames, accountName, targetIP, targetPort, authenticationType)
}

// ### bootz-2: Validate Software image in bootz configuration
func TestBootz2(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Create directory
	mkdirErr := os.Mkdir(credzFileName, 0755)
	defer os.RemoveAll(credzFileName)
	if mkdirErr != nil {
		t.Fatalf("Error creating directory: %v", mkdirErr)
	}

	// Credz code
	accountName := "CREDZ"
	authenticationType := []string{"ecdsa-sha2-nistp256", "ecdsa-sha2-nistp521", "ssh-ed25519", "rsa-pubkey", "rsa-pubkey"}

	targetIP, targetPort, err := getIpAndPortFromBindingFile()
	if err != nil {
		t.Fatalf("Error in reading target IP and Port from Binding file: %v", err)
	}

	gnsiC := dut.RawAPIs().GNSI(t)
	profileID := "CERTZ_BOOTZ"

	// Check if profile already exists
	profilesList, err := gnsiC.Certz().GetProfileList(context.Background(), &certzpb.GetProfileListRequest{})
	t.Logf("Available ProfilesList: %v", profilesList)
	if err != nil {
		t.Logf("Failed to list profiles got : %v", err)
	}

	profileExists := false
	for _, profile := range profilesList.GetSslProfileIds() {
		if profile == profileID {
			profileExists = true
			break
		}
	}

	// Delete & Add profile only if it does NOT exist
	if !profileExists {
		profiles, err := gnsiC.Certz().AddProfile(context.Background(), &certzpb.AddProfileRequest{SslProfileId: profileID})
		if err != nil {
			t.Fatalf("Unexpected Error in adding profile list: %v", err)
		}
		t.Logf("Profile add successful: %v", profiles)
	} else {
		t.Logf("Profile %s already exists. Skipping AddProfile.", profileID)
	}

	// Call common setup and rotate function
	clientKeyNames := setupAndRotateCertificates(t, dut)

	// Start the boot config client.
	testSetup(t, dut)
	defer bServer.Stop()
	defer dhcp.Stop()

	dutPreTestVersion := gnmi.Get(t, dut, gnmi.OC().System().SoftwareVersion().State())
	bootzStarted := false

	bootz2 := []bootzTest{
		{
			Name:         "Bootz-2.1 Invalid software image ",
			VendorConfig: baseConfig,
			Image: &bpb.SoftwareImage{
				Name:          "badimage.iso",
				Url:           fmt.Sprintf("https://%s/badimage.iso", *imageServerAddr),
				HashAlgorithm: "sha256",
				OsImageHash:   getImageHash(t, fmt.Sprintf("%s/badimage.iso", *imagesDir)),
				Version:       "99999",
			},
			ExpectedFailure: true,
		},
		{
			Name:         "Bootz-2.2: Software version is different",
			VendorConfig: baseConfig,
			Image: &bpb.SoftwareImage{
				Name:          "goodimage.iso",
				Url:           fmt.Sprintf("https://%s/goodimage.iso", *imageServerAddr),
				HashAlgorithm: "sha256",
				OsImageHash:   getImageHash(t, fmt.Sprintf("%s/goodimage.iso", *imagesDir)),
				Version:       *imageVersion,
			},
			ExpectedFailure: false,
		},
	}
	t.Run("Running Bootz2 Test to Validate Software image in bootz configuration", func(t *testing.T) {
		for _, tt := range bootz2 {
			t.Run(tt.Name, func(t *testing.T) {
				if bootzServerFailed.Load() {
					t.Fatal("bootz server is down, check the test log for detailed error")
				}
				// reset bootz logs
				bootzStatusLogs = bootzStatus{}
				bootzReqLogs = bootzLogs{}
				//ensure no old dhcp log causing an issue
				dhcpLease.CleanLog()

				chassisBootzConfig.GetConfig().BootConfig.VendorConfig = []byte(tt.VendorConfig)
				chassisBootzConfig.SoftwareImage = tt.Image
				em.ReplaceDevice(chassisEntity, chassisBootzConfig)
				if !bootzStarted {
					factoryReset(t, dut)
					bootzStarted = true
				}
				dhcpIDs := []string{chassisSerial}
				dhcpIDs = append(dhcpIDs, hwAddrs...)
				err := awaitDHCPCompletion(dhcpIDs, dhcpTimeout)
				if err != nil {
					t.Errorf("DUT connection to DHCP server was not successful in %d minutes", dhcpTimeout)
				} else {
					t.Logf("DUT connection to DHCP server was  successful")
				}
				err = awaitBootzConnection(*chassisEntity, bootzConnectionTimeout)
				if err != nil {
					t.Errorf("DUT connection to bootz server was not successful in %d minutes", bootzConnectionTimeout)
				} else {
					t.Log("DUT is connected to bootz server")
				}
				checkBootzStatus(t, tt.ExpectedFailure, fullBootzCompletionTimeout)
			})
		}
		dutBootzStatus(t, dut, fullBootzCompletionTimeout)
		dutPostTestVersion := gnmi.Get(t, dut, gnmi.OC().System().SoftwareVersion().State())
		if dutPostTestVersion != *imageVersion {
			t.Fatalf("DUT software versions do not match, pretest: %s , posttest: %s ", dutPreTestVersion, dutPostTestVersion)
		}

	})

	// Validate BootConfig Behaviour after RP Switchover
	testBootConfigValidation(t, dut)

	// Validate Pathz Behaviour
	gNSIvalidation(t, dut, clientKeyNames, accountName, targetIP, targetPort, authenticationType)
}

// ### bootz-3: Validate Ownership Voucher in bootz configuration
func TestBootz3(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Create directory
	mkdirErr := os.Mkdir(credzFileName, 0755)
	// defer os.RemoveAll(credzFileName)
	if mkdirErr != nil {
		t.Fatalf("Error creating directory: %v", mkdirErr)
	}

	// Call common setup and rotate function
	setupAndRotateCertificates(t, dut)

	testSetup(t, dut)
	defer bServer.Stop()
	defer dhcp.Stop()

	bootz3 := []bootzTest{
		// need to discuss and remove
		/*bootzTest{
			// TODO: clarify this since in secure mode we may not support this
			Name:            "Bootz-3.1: No ownership voucher ",
			VendorConfig:    baseConfig,
			ExpectedFailure: false,
			OV: []byte(""),
		},*/
		{
			Name:            "Bootz-3.2 Invalid OV",
			VendorConfig:    baseConfig,
			ExpectedFailure: true,
			OV:              []byte("invalid ov"),
		},
		{
			Name:            "bootz-3.3  Valid OV format but for differnt device",
			VendorConfig:    baseConfig,
			ExpectedFailure: true,
			OV:              loadOV(t, "wrongserial", secArtifacts.PDC, false), // get serail as flasg
		},
		{
			Name:            "bootz-3.4 Valid OV",
			VendorConfig:    baseConfig,
			ExpectedFailure: false,
		},
	}

	dutPreTestVersion := gnmi.Get(t, dut, gnmi.OC().System().SoftwareVersion().State())
	bootzStarted := false

	t.Run("Running Bootz3 Validate Ownership Voucher in bootz configuration", func(t *testing.T) {
		for _, tt := range bootz3 {
			t.Run(tt.Name, func(t *testing.T) {
				if bootzServerFailed.Load() {
					t.Fatal("bootz server is down, check the test log for detailed error")
				}
				// reset bootz logs
				bootzStatusLogs = bootzStatus{}
				bootzReqLogs = bootzLogs{}
				//ensure no old dhcp log causing an issue
				dhcpLease.CleanLog()

				chassisBootzConfig.GetConfig().BootConfig.VendorConfig = []byte(tt.VendorConfig)
				chassisBootzConfig.SoftwareImage = &bpb.SoftwareImage{}
				for k := range secArtifacts.OV {
					secArtifacts.OV[k] = tt.OV
				}
				if len(tt.OV) == 0 { // load the valid ovs
					for _, cc := range controllerCardSerials {
						secArtifacts.OV[cc] = loadOV(t, cc, secArtifacts.PDC, true)
					}
				}
				em.ReplaceDevice(chassisEntity, chassisBootzConfig)
				if !bootzStarted {
					factoryReset(t, dut)
					bootzStarted = true
				}
				dhcpIDs := []string{chassisSerial}
				dhcpIDs = append(dhcpIDs, hwAddrs...)
				err := awaitDHCPCompletion(dhcpIDs, dhcpTimeout)
				if err != nil {
					t.Errorf("DUT connection to DHCP server was not successful in %d minutes", dhcpTimeout)
				} else {
					t.Logf("DUT connection to DHCP server was  successful")
				}
				err = awaitBootzConnection(*chassisEntity, bootzConnectionTimeout)
				if err != nil {
					t.Errorf("DUT connection to bootz server was not successful in %d minutes", bootzConnectionTimeout)
				} else {
					t.Log("DUT is connected to bootz server")
				}
				if !tt.ExpectedFailure { // when OV validation fails, device has no secure way to connect and report the status
					checkBootzStatus(t, tt.ExpectedFailure, bootzStatusTimeout)
				}
			})
		}
		dutBootzStatus(t, dut, 5*time.Second)
		dutPostTestVersion := gnmi.Get(t, dut, gnmi.OC().System().SoftwareVersion().State())
		if dutPreTestVersion != dutPostTestVersion {
			t.Fatalf("DUT Software Versions do not match, pretest: %s , posttest: %s ", dutPreTestVersion, dutPostTestVersion)
		}
	})
}

// ### bootz-4: Validate device properly resets if provided invalid image
func TestBootz4(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	defer os.RemoveAll(credzFileName)

	testSetup(t, dut)
	defer bServer.Stop()
	defer dhcp.Stop()

	bootz4 := []bootzTest{
		{
			Name:         "Bootz-4.1 Invalid OS image provided",
			VendorConfig: baseConfig,
			Image: &bpb.SoftwareImage{
				Name:          "badimage.iso",
				Url:           fmt.Sprintf("https://%s/badimage.iso", *imageServerAddr),
				HashAlgorithm: "SHA256",
				OsImageHash:   getImageHash(t, fmt.Sprintf("%s/badimage.iso", *imagesDir)),
				Version:       "999",
			},
			ExpectedFailure: true,
		}, //gets covered as a part of Bootz2.2
		{
			Name:         "Bootz-4.2 Failed to fetch image from remote URL",
			VendorConfig: baseConfig,
			Image: &bpb.SoftwareImage{
				Name:          "badimage.iso",
				Url:           fmt.Sprintf("https://%s/goodimage.isoinvalidUrl", *imageServerAddr),
				HashAlgorithm: "SHA256",
				OsImageHash:   getImageHash(t, fmt.Sprintf("%s/goodimage.iso", *imagesDir)),
				Version:       "999",
			},
			ExpectedFailure: true,
		},
		{
			Name:         "Bootz-4.3 OS Checksum Doesn't Match",
			VendorConfig: baseConfig,
			Image: &bpb.SoftwareImage{
				Name:          "goodimage.iso",
				Url:           fmt.Sprintf("https://%s/goodimage.iso", *imageServerAddr),
				HashAlgorithm: "SHA256",
				OsImageHash:   "Invalid Hash",
				Version:       "999",
			},
			ExpectedFailure: true,
		},
		{
			Name:            "Bootz-4.4: No OS Provided",
			VendorConfig:    baseConfig,
			Image:           &bpb.SoftwareImage{},
			ExpectedFailure: false,
		},
	}
	dutPreTestVersion := gnmi.Get(t, dut, gnmi.OC().System().SoftwareVersion().State())
	bootzStarted := false

	t.Run("Running Bootz4 Test to Validate Software image in bootz configuration", func(t *testing.T) {
		for _, tt := range bootz4 {
			t.Run(tt.Name, func(t *testing.T) {
				if bootzServerFailed.Load() {
					t.Fatal("bootz server is down, check the test log for detailed error")
				}
				// reset bootz logs
				bootzStatusLogs = bootzStatus{}
				bootzReqLogs = bootzLogs{}
				//ensure no old dhcp log causing an issue
				dhcpLease.CleanLog()

				chassisBootzConfig.GetConfig().BootConfig.VendorConfig = []byte(tt.VendorConfig)
				chassisBootzConfig.SoftwareImage = tt.Image
				em.ReplaceDevice(chassisEntity, chassisBootzConfig)
				if !bootzStarted {
					factoryReset(t, dut)
					bootzStarted = true
				}
				dhcpIDs := []string{chassisSerial}
				dhcpIDs = append(dhcpIDs, hwAddrs...)
				err := awaitDHCPCompletion(dhcpIDs, dhcpTimeout)
				if err != nil {
					t.Errorf("DUT connection to DHCP server was not successful in %d minutes", dhcpTimeout)
				} else {
					t.Logf("DUT connection to DHCP server was  successful")
				}
				err = awaitBootzConnection(*chassisEntity, bootzConnectionTimeout)
				if err != nil {
					t.Errorf("DUT connection to bootz server was not successful in %d minutes", bootzConnectionTimeout)
				} else {
					t.Log("DUT is connected to bootz server")
				}
				checkBootzStatus(t, tt.ExpectedFailure, bootzStatusTimeout)
			})
		}
		dutBootzStatus(t, dut, 5*time.Second)
		dutPostTestVersion := gnmi.Get(t, dut, gnmi.OC().System().SoftwareVersion().State())
		if dutPreTestVersion != dutPostTestVersion {
			t.Fatalf("DUT Software Versions do not match, pretest: %s , posttest: %s ", dutPreTestVersion, dutPostTestVersion)
		}
	})
}

// ### Test Bootconfig Feature
func TestBootConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Delete all policy file
	pathz.DeletePolicyData(t, dut, "*")

	// Perform eMSD process restart after deleting the bootconfig file
	t.Logf("Restarting emsd at %s", time.Now())
	perf.RestartProcess(t, dut, "emsd")
	t.Logf("Restart emsd finished at %s", time.Now())

	t.Run("Initial BootConfig Not Present: Set/Get BootConfig with Inline Vendor Configuration and validate after RP Switchover", func(t *testing.T) {
		dut := ondatra.DUT(t, "dut")

		// Start the boot config client.
		bootconfigClient := start(t)

		// Perform GetBootConfig operation when the initial boot config is not present
		getResp, getErr := bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		// Inline validation of the error
		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// SetBootConfig with inline vendor configuration
		want := &bpb.BootConfig{VendorConfig: []byte("hostname setbootconfig")}

		setReq := &bcpb.SetBootConfigRequest{BootConfig: want}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		// Perform SetBootConfig operation
		setResp, setErr := bootconfigClient.SetBootConfig(context.Background(), setReq)
		if setErr != nil {
			t.Fatalf("SetBootConfig failed: %v", setErr)
		}
		t.Logf("SetBootConfig Response: %v", setResp)

		// Perform GetBootConfig operation after SetBootConfig
		verifyResp, verifyErr := bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "non-bootconfig", true)
		validateVendorConfig(t, dut, "hostname Rotate", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)

		// RP Switchover.
		utils.Dorpfo(context.Background(), t, true)

		// validate NSR-Ready is ready
		redundancy_nsrState(context.Background(), t, true)

		// Start the boot config client.
		bootconfigClient = start(t)

		// Perform GetBootConfig operation
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate GetBootConfig matches expectation after RP Switchover.
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations after RP Switchover.
		validateBootConfigConfiguration(t, dut, "rpso", true)
		validateVendorConfig(t, dut, "hostname Rotate", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)

		// Delete Bootconfig file
		pathz.DeletePolicyData(t, dut, "boot_config_merged.txt")

		// Perform GetBootConfig again to verify persistence after deleting bootconfig file.
		getResp, getErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		// Inline validation of the error
		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// Validate BootConfig behaviour after deleting the bootconfig file.
		validateBootConfigConfiguration(t, dut, "rpso-1", true)
		validateVendorConfig(t, dut, "hostname Rotate", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, true)

		// Perform eMSD process restart after deleting the bootconfig file
		t.Logf("Restarting emsd at %s", time.Now())
		perf.RestartProcess(t, dut, "emsd")
		t.Logf("Restart emsd finished at %s", time.Now())

		// Validate GetBootConfig operation after process restart
		getResp, getErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		// Inline validation of the error
		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// Validate BootConfig behaviour after deleting and restarting emsd.
		validateBootConfigConfiguration(t, dut, "restart-proc", false)
		validateVendorConfig(t, dut, "hostname Rotate", false)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, true)
	})
	t.Run("Initial BootConfig Not Present: Set/Get BootConfig with only valid OC Configuration and validate after process restart", func(t *testing.T) {
		dut := ondatra.DUT(t, "dut")

		// Start the boot config client.
		bootconfigClient := start(t)

		// Define the path to the bootconfig file
		bootconfigPath := "testdata/bootconfig_valid.txt"

		// Perform GetBootConfig operation when the initial boot config is not present
		getResp, getErr := bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		// Inline validation of the error
		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// Define OC configuration
		ocConfig := getBootConfigFromFile(t, bootconfigPath).OcConfig

		// Required OC configuration for set and get bootconfig
		want := &bpb.BootConfig{OcConfig: ocConfig}

		// SetBootConfig with only valid OC configuration
		setReq := &bcpb.SetBootConfigRequest{BootConfig: want}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		// Perform SetBootConfig operation
		setResp, setErr := bootconfigClient.SetBootConfig(context.Background(), setReq)
		if setErr != nil {
			t.Fatalf("SetBootConfig failed: %v", setErr)
		}
		t.Logf("SetBootConfig Response: %v", setResp)

		// Perform GetBootConfig operation after SetBootConfig
		verifyResp, verifyErr := bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "non-bootconfig", true)
		validateVendorConfig(t, dut, "hostname Rotate", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)

		// Perform eMSD process restart
		t.Logf("Restarting emsd at %s", time.Now())
		perf.RestartProcess(t, dut, "emsd")
		t.Logf("Restart emsd finished at %s", time.Now())

		// Perform GetBootConfig operation after process restart
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate GetBootConfig matches expectation after process restart.
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "restart-pro", true)
		validateVendorConfig(t, dut, "hostname Rotate", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)

		// Delete Bootconfig file
		pathz.DeletePolicyData(t, dut, "boot_config_merged.txt")

		// Perform GetBootConfig again to verify persistence after deleting bootconfig file.
		getResp, getErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		// Inline validation of the error
		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "restart-pro-1", true)
		validateVendorConfig(t, dut, "hostname Rotate", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, true)

		// Perform eMSD process restart after deleting the bootconfig file
		t.Logf("Restarting emsd at %s", time.Now())
		perf.RestartProcess(t, dut, "emsd")
		t.Logf("Restart emsd finished at %s", time.Now())

		// Validate GetBootConfig operation after process restart
		getResp, getErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		// Inline validation of the error
		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// Validate BootConfig behaviour after deleting and restarting emsd.
		validateBootConfigConfiguration(t, dut, "restart-pro-2", false)
		validateVendorConfig(t, dut, "hostname Rotate", false)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, true)
	})
	t.Run("Initial BootConfig Not Present: Set/Get BootConfig with File-based Configuration and validate after Router reload", func(t *testing.T) {
		dut := ondatra.DUT(t, "dut")

		// Start the boot config client.
		bootconfigClient := start(t)

		// Define the path to the bootconfig file
		bootconfigPath := "testdata/bootconfig_valid.txt"

		// Perform GetBootConfig operation when the initial boot config is not present
		getResp, getErr := bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		// Inline validation of the error
		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// Required BootConfig from file
		want := getBootConfigFromFile(t, bootconfigPath)

		// SetBootConfig with file-based configuration
		setReq := &bcpb.SetBootConfigRequest{BootConfig: want}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		// Perform SetBootConfig operation
		setResp, setErr := bootconfigClient.SetBootConfig(context.Background(), setReq)
		if setErr != nil {
			t.Fatalf("SetBootConfig failed: %v", setErr)
		}
		t.Logf("SetBootConfig Response: %v", setResp)

		// Perform GetBootConfig operation
		verifyResp, verifyErr := bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "non-bootconfig", true)
		validateVendorConfig(t, dut, "ssh server rate-limit 600", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)

		// Reload router
		perf.ReloadRouter(t, dut)

		// waiting for 30 seconds to yiny-resync
		time.Sleep(30 * time.Second)

		// Start the boot config client.
		bootconfigClient = start(t)

		// Perform GetBootConfig operation after router reload
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation after router reload.
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations after router reload
		validateBootConfigConfiguration(t, dut, "reload", true)
		validateVendorConfig(t, dut, "ssh server rate-limit 600", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)

		// Delete Bootconfig file and verify the behaviour
		pathz.DeletePolicyData(t, dut, "boot_config_merged.txt")

		// Perform GetBootConfig again to verify persistence after deleting bootconfig file.
		getResp, getErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		// Inline validation of the error
		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// Validate BootConfig behaviour after deleting the bootconfig file.
		validateBootConfigConfiguration(t, dut, "reload-1", true)
		validateVendorConfig(t, dut, "ssh server rate-limit 600", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, true)

		// Perform eMSD process restart after deleting the bootconfig file
		t.Logf("Restarting emsd at %s", time.Now())
		perf.RestartProcess(t, dut, "emsd")
		t.Logf("Restart emsd finished at %s", time.Now())

		// Validate GetBootConfig operation after process restart
		getResp, getErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		// Inline validation of the error
		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// Validate BootConfig behaviour after deleting and restarting emsd.
		validateBootConfigConfiguration(t, dut, "restart-proc", false)
		validateVendorConfig(t, dut, "ssh server rate-limit 600", false)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, true)
	})
	t.Run("Initial BootConfig Not Present: Test Set/Get BootConfig with NIL configuration", func(t *testing.T) {
		dut := ondatra.DUT(t, "dut")

		// Start the boot config client.
		bootconfigClient := start(t)

		// Perform GetBootConfig operation when the initial boot config is not present
		getResp, getErr := bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		// Inline validation of the error
		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		setReq := &bcpb.SetBootConfigRequest{BootConfig: nil}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		setResp, err := bootconfigClient.SetBootConfig(context.Background(), setReq)
		if err != nil {
			t.Logf("Expected failure: SetBootConfig returned error: %v", err)
		} else {
			t.Errorf("SetBootConfig should have failed but succeeded with response: %v", setResp)
		}

		getResp, getErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}
		validateBootConfigConfiguration(t, dut, "nil-bootconfig", false)
		validateVendorConfig(t, dut, "hostname Rotate", false)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, true)
	})
	t.Run("Initial BootConfig Not Present: Test Set/Get BootConfig with invalid vendor configuration", func(t *testing.T) {
		dut := ondatra.DUT(t, "dut")

		// Start the boot config client.
		bootconfigClient := start(t)

		// Perform GetBootConfig operation when the initial boot config is not present
		getResp, getErr := bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		// Inline validation of the error
		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		want := &bpb.BootConfig{VendorConfig: []byte("grpc\n gnsi service certz ssl-profile-id CETZ_BOOTZ")}

		setReq := &bcpb.SetBootConfigRequest{BootConfig: want}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		setResp, err := bootconfigClient.SetBootConfig(context.Background(), setReq)
		if err != nil {
			t.Logf("Expected failure: SetBootConfig returned error: %v", err)
		} else {
			t.Errorf("SetBootConfig should have failed but succeeded with response: %v", setResp)
		}

		getResp, getErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}
		validateBootConfigConfiguration(t, dut, "invald-bootconfig", false)
		validateVendorConfig(t, dut, "hostname profile-id", false)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, true)
	})
	t.Run("Initial BootConfig Not Present: Test Set/Get BootConfig with empty configuration and validate behaviour after RPSwitchover", func(t *testing.T) {
		dut := ondatra.DUT(t, "dut")

		// Start the boot config client.
		bootconfigClient := start(t)

		// Perform GetBootConfig operation when the initial boot config is not present
		getResp, getErr := bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		// Inline validation of the error
		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		setReq := &bcpb.SetBootConfigRequest{BootConfig: &bpb.BootConfig{}}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		setResp, err := bootconfigClient.SetBootConfig(context.Background(), setReq)
		if err != nil {
			t.Logf("Expected failure: SetBootConfig returned error: %v", err)
		} else {
			t.Errorf("SetBootConfig should have failed but succeeded with response: %v", setResp)
		}

		getResp, getErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "empty-bootconfig", false)
		validateVendorConfig(t, dut, "hostname Rotate", false)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, true)

		// RP Switchover.
		utils.Dorpfo(context.Background(), t, true)

		// validate NSR-Ready is ready
		redundancy_nsrState(context.Background(), t, true)

		// Start the boot config client after RP Switchover.
		bootconfigClient = start(t)

		// Perform GetBootConfig operation after RP Switchover
		getResp, getErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		// Inline validation of the error
		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// Perform GetBootConfig again to verify persistence after RP Switchover.
		setResp, err = bootconfigClient.SetBootConfig(context.Background(), setReq)
		if err != nil {
			t.Logf("Expected failure: SetBootConfig returned error: %v", err)
		} else {
			t.Errorf("SetBootConfig should have failed but succeeded with response: %v", setResp)
		}

		// Validate getbootconfig after RP Switchover
		getResp, getErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// validate bootconfig configuration behaviour after RP Switchover
		validateBootConfigConfiguration(t, dut, "rpso", false)
		validateVendorConfig(t, dut, "hostname Rotate", false)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, true)
	})
	t.Run("Initial BootConfig Not Present: Test Set/Get BootConfig with invalid configuration file and validate after Router reload", func(t *testing.T) {
		dut := ondatra.DUT(t, "dut")

		// Start the boot config client.
		bootconfigClient := start(t)

		// Define the path to the bootconfig file
		bootconfigPath := "testdata/bootconfig_invalid.txt"

		// Perform GetBootConfig operation when the initial boot config is not present
		getResp, getErr := bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		// Inline validation of the error
		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// SetBootConfig with invalid configuration
		setReq := &bcpb.SetBootConfigRequest{BootConfig: getBootConfigFromFile(t, bootconfigPath)}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		setResp, err := bootconfigClient.SetBootConfig(context.Background(), setReq)
		if err != nil {
			t.Logf("Expected failure: SetBootConfig returned error: %v", err)
		} else {
			t.Errorf("SetBootConfig should have failed but succeeded with response: %v", setResp)
		}

		getResp, getErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "invalid-bootconfig", false)
		validateVendorConfig(t, dut, "hostname Rotate", false)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, true)

		// Reload router
		perf.ReloadRouter(t, dut)

		// waiting for 30 seconds to yiny-resync
		time.Sleep(30 * time.Second)

		// Start the boot config client after Router Reload.
		bootconfigClient = start(t)

		// Perform GetBootConfig operation after router reload
		getResp, getErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		// Inline validation of the error
		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// Perform GetBootConfig again to verify persistence after Router Reload.
		setResp, err = bootconfigClient.SetBootConfig(context.Background(), setReq)
		if err != nil {
			t.Logf("Expected failure: SetBootConfig returned error: %v", err)
		} else {
			t.Errorf("SetBootConfig should have failed but succeeded with response: %v", setResp)
		}

		// Validate getbootconfig after Router Reload
		getResp, getErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// Perform common validations after Router Reload
		validateBootConfigConfiguration(t, dut, "reload", false)
		validateVendorConfig(t, dut, "hostname Rotate", false)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, true)
	})
	t.Run("Initial BootConfig Not Present: Test Set/Get BootConfig with invalid OC configs and validate after eMSD process restart", func(t *testing.T) {
		dut := ondatra.DUT(t, "dut")

		// Start the boot config client.
		bootconfigClient := start(t)

		// Perform GetBootConfig operation when the initial boot config is not present
		getResp, getErr := bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		// Inline validation of the error
		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// SetBootConfig with invalid OC configuration
		setReq := &bcpb.SetBootConfigRequest{BootConfig: &bpb.BootConfig{OcConfig: []byte("invlid oc config")}}

		// Perform SetBootConfig operation
		setResp, err := bootconfigClient.SetBootConfig(context.Background(), setReq)
		if err != nil {
			t.Logf("Expected failure: SetBootConfig returned error: %v", err)
		} else {
			t.Errorf("SetBootConfig should have failed but succeeded with response: %v", setResp)
		}

		getResp, getErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "invalid-oc", false)
		validateVendorConfig(t, dut, "hostname Rotate", false)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, true)

		// Perform eMSD process restart
		t.Logf("Restarting emsd at %s", time.Now())
		perf.RestartProcess(t, dut, "emsd")
		t.Logf("Restart emsd finished at %s", time.Now())

		// Perform GetBootConfig operation after eMSD Process Restart.
		getResp, getErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		// Inline validation of the error
		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// Perform GetBootConfig again to verify persistence after eMSD Process Restart.
		setResp, err = bootconfigClient.SetBootConfig(context.Background(), setReq)
		if err != nil {
			t.Logf("Expected failure: SetBootConfig returned error: %v", err)
		} else {
			t.Errorf("SetBootConfig should have failed but succeeded with response: %v", setResp)
		}

		// Validate getbootconfig after eMSD process restart
		getResp, getErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// Perform common validations after eMSD Process Restart
		validateBootConfigConfiguration(t, dut, "rpso", false)
		validateVendorConfig(t, dut, "hostname Rotate", false)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, true)
	})
	t.Run("Initial BootConfig Present: Set/Get BootConfig with Inline Vendor Configuration and validate after RP Switchover", func(t *testing.T) {
		dut := ondatra.DUT(t, "dut")

		// Start the boot config client.
		bootconfigClient := start(t)

		// Initial SetBootConfig with inline vendor configuration
		want := &bpb.BootConfig{VendorConfig: []byte("hostname setbootconfig")}

		setReq := &bcpb.SetBootConfigRequest{BootConfig: want}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		// Perform SetBootConfig operation
		setResp, setErr := bootconfigClient.SetBootConfig(context.Background(), setReq)
		if setErr != nil {
			t.Fatalf("SetBootConfig failed: %v", setErr)
		}
		t.Logf("SetBootConfig Response: %v", setResp)

		// Perform GetBootConfig operation after SetBootConfig
		verifyResp, verifyErr := bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform SetBootConfig operation again with different inline vendor configuration
		want = &bpb.BootConfig{VendorConfig: []byte("ssh server rate-limit 6000")}

		setReq = &bcpb.SetBootConfigRequest{BootConfig: want}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		// Perform SetBootConfig operation
		setResp, setErr = bootconfigClient.SetBootConfig(context.Background(), setReq)
		if setErr != nil {
			t.Fatalf("SetBootConfig failed: %v", setErr)
		}
		t.Logf("SetBootConfig Response: %v", setResp)

		// Perform GetBootConfig operation after SetBootConfig
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "non-bootconfig", false)
		validateVendorConfig(t, dut, "ssh server rate-limit 1000", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)

		// RP Switchover.
		utils.Dorpfo(context.Background(), t, true)

		// validate NSR-Ready is ready
		redundancy_nsrState(context.Background(), t, true)

		// Start the boot config client.
		bootconfigClient = start(t)

		// Perform GetBootConfig operation after SetBootConfig
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "rpso", false)
		validateVendorConfig(t, dut, "ssh server rate-limit 1000", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)

		// Perform SetBootConfig with inline vendor configuration after RP Switchover
		want = &bpb.BootConfig{VendorConfig: []byte("hostname setbootconfig")}

		setReq = &bcpb.SetBootConfigRequest{BootConfig: want}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		// Perform SetBootConfig operation after RP Switchover
		setResp, setErr = bootconfigClient.SetBootConfig(context.Background(), setReq)
		if setErr != nil {
			t.Fatalf("SetBootConfig failed: %v", setErr)
		}
		t.Logf("SetBootConfig Response: %v", setResp)

		// Perform GetBootConfig operation after SetBootConfig after RP Switchover
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations after RP Switchover
		validateBootConfigConfiguration(t, dut, "rpso-1", true)
		validateVendorConfig(t, dut, "ssh server rate-limit 1000", false)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)
	})
	t.Run("Initial BootConfig Present: Set/Get BootConfig with OC Configuration and validate after process restart", func(t *testing.T) {
		dut := ondatra.DUT(t, "dut")

		// Start the boot config client.
		bootconfigClient := start(t)

		// Perform GetBootConfig operation after SetBootConfig
		verifyResp, verifyErr := bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyResp == nil {
			t.Fatalf("GetBootConfig failed: %v", verifyResp)
		}
		t.Logf("GetBootConfig ErrorResponse: %v", verifyErr)

		// Perform SetBootConfig operation again with different inline vendor configuration
		want := &bpb.BootConfig{VendorConfig: []byte("ssh server rate-limit 6000")}

		setReq := &bcpb.SetBootConfigRequest{BootConfig: want}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		// Perform SetBootConfig operation
		setResp, setErr := bootconfigClient.SetBootConfig(context.Background(), setReq)
		if setErr != nil {
			t.Fatalf("SetBootConfig failed: %v", setErr)
		}
		t.Logf("SetBootConfig Response: %v", setResp)

		// Perform GetBootConfig operation after SetBootConfig
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "non-bootconfig", false)
		validateVendorConfig(t, dut, "ssh server rate-limit 1000", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)

		// Perform eMSD process restart
		t.Logf("Restarting emsd at %s", time.Now())
		perf.RestartProcess(t, dut, "emsd")
		t.Logf("Restart emsd finished at %s", time.Now())

		// Perform GetBootConfig operation after SetBootConfig
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "non-bootconfig", false)
		validateVendorConfig(t, dut, "ssh server rate-limit 1000", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)

		// Define the path to the bootconfig file
		bootconfigPath := "testdata/bootconfig_valid.txt"

		// Define OC configuration
		ocConfig := getBootConfigFromFile(t, bootconfigPath).OcConfig

		// Required OC configuration for set and get bootconfig
		want = &bpb.BootConfig{OcConfig: ocConfig}

		// SetBootConfig with only valid OC configuration
		setReq = &bcpb.SetBootConfigRequest{BootConfig: want}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		// Perform SetBootConfig operation
		setResp, setErr = bootconfigClient.SetBootConfig(context.Background(), setReq)
		if setErr != nil {
			t.Fatalf("SetBootConfig failed: %v", setErr)
		}
		t.Logf("SetBootConfig Response: %v", setResp)

		// Perform GetBootConfig operation after SetBootConfig
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "non-bootconfig", true)
		validateVendorConfig(t, dut, "hostname Rotate", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)

		// Perform eMSD process restart
		t.Logf("Restarting emsd at %s", time.Now())
		perf.RestartProcess(t, dut, "emsd")
		t.Logf("Restart emsd finished at %s", time.Now())

		// SetBootConfig with only valid OC configuration
		setReq = &bcpb.SetBootConfigRequest{BootConfig: want}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		// Perform SetBootConfig operation
		setResp, setErr = bootconfigClient.SetBootConfig(context.Background(), setReq)
		if setErr != nil {
			t.Fatalf("SetBootConfig failed: %v", setErr)
		}
		t.Logf("SetBootConfig Response: %v", setResp)

		// Perform GetBootConfig operation after SetBootConfig
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "non-bootconfig", true)
		validateVendorConfig(t, dut, "hostname Rotate", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)

		// Perform SetBootConfig operation again with different inline vendor configuration
		want = &bpb.BootConfig{VendorConfig: []byte("ssh server rate-limit 6000")}

		setReq = &bcpb.SetBootConfigRequest{BootConfig: want}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		// Perform SetBootConfig operation
		setResp, setErr = bootconfigClient.SetBootConfig(context.Background(), setReq)
		if setErr != nil {
			t.Fatalf("SetBootConfig failed: %v", setErr)
		}
		t.Logf("SetBootConfig Response: %v", setResp)

		// Perform GetBootConfig operation after SetBootConfig
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "non-bootconfig", false)
		validateVendorConfig(t, dut, "ssh server rate-limit 1000", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)
	})
	t.Run("Initial BootConfig Present: Set/Get BootConfig with File-based Configuration and validate after Router reload", func(t *testing.T) {
		dut := ondatra.DUT(t, "dut")

		// Start the boot config client.
		bootconfigClient := start(t)

		// Perform GetBootConfig operation after SetBootConfig
		verifyResp, verifyErr := bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyResp == nil {
			t.Fatalf("GetBootConfig failed: %v", verifyResp)
		}
		t.Logf("GetBootConfig ErrorResponse: %v", verifyErr)

		// Define the path to the bootconfig file
		bootconfigPath := "testdata/bootconfig_valid.txt"

		// Required BootConfig from file
		want := getBootConfigFromFile(t, bootconfigPath)

		// SetBootConfig with file-based configuration
		setReq := &bcpb.SetBootConfigRequest{BootConfig: want}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		// Perform SetBootConfig operation
		setResp, setErr := bootconfigClient.SetBootConfig(context.Background(), setReq)
		if setErr != nil {
			t.Fatalf("SetBootConfig failed: %v", setErr)
		}
		t.Logf("SetBootConfig Response: %v", setResp)

		// Perform GetBootConfig operation
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "non-bootconfig", true)
		validateVendorConfig(t, dut, "ssh server rate-limit 600", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)

		// Reload router
		perf.ReloadRouter(t, dut)

		// waiting for 30 seconds to yiny-resync
		time.Sleep(30 * time.Second)

		// Start the boot config client.
		bootconfigClient = start(t)

		// Define OC configuration
		ocConfig := getBootConfigFromFile(t, bootconfigPath).OcConfig

		// Required OC configuration for set and get bootconfig
		want = &bpb.BootConfig{OcConfig: ocConfig}

		// SetBootConfig with only valid OC configuration
		setReq = &bcpb.SetBootConfigRequest{BootConfig: want}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		// Perform SetBootConfig operation
		setResp, setErr = bootconfigClient.SetBootConfig(context.Background(), setReq)
		if setErr != nil {
			t.Fatalf("SetBootConfig failed: %v", setErr)
		}
		t.Logf("SetBootConfig Response: %v", setResp)

		// Perform GetBootConfig operation after SetBootConfig
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "reload", true)
		validateVendorConfig(t, dut, "ssh server rate-limit 600", false)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)

		// Delete Bootconfig file and verify the behaviour
		pathz.DeletePolicyData(t, dut, "boot_config_merged.txt")

		// Perform GetBootConfig again to verify persistence after deleting bootconfig file.
		getResp, getErr := bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		// Inline validation of the error
		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// Validate BootConfig behaviour after deleting the bootconfig file.
		validateBootConfigConfiguration(t, dut, "delete-bootconfigfile", true)
		validateVendorConfig(t, dut, "ssh server rate-limit 600", false)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, true)

		// Perform eMSD process restart after deleting the bootconfig file
		t.Logf("Restarting emsd at %s", time.Now())
		perf.RestartProcess(t, dut, "emsd")
		t.Logf("Restart emsd finished at %s", time.Now())

		// Validate GetBootConfig operation after process restart
		getResp, getErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		// Inline validation of the error
		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// Validate BootConfig behaviour after deleting and restarting emsd.
		validateBootConfigConfiguration(t, dut, "non-bootconfig", false)
		validateVendorConfig(t, dut, "ssh server rate-limit 600", false)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, true)
	})
	t.Run("Initial BootConfig Present: Test Set/Get BootConfig with NIL configuration", func(t *testing.T) {
		dut := ondatra.DUT(t, "dut")

		// Start the boot config client.
		bootconfigClient := start(t)

		// Define the path to the bootconfig file
		bootconfigPath := "testdata/bootconfig_valid.txt"

		// Define OC configuration
		ocConfig := getBootConfigFromFile(t, bootconfigPath).OcConfig

		// Required OC configuration for set and get bootconfig
		want := &bpb.BootConfig{OcConfig: ocConfig}

		// SetBootConfig with only valid OC configuration
		setReq := &bcpb.SetBootConfigRequest{BootConfig: want}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		// Perform SetBootConfig operation
		setResp, setErr := bootconfigClient.SetBootConfig(context.Background(), setReq)
		if setErr != nil {
			t.Fatalf("SetBootConfig failed: %v", setErr)
		}
		t.Logf("SetBootConfig Response: %v", setResp)

		// Perform GetBootConfig operation after SetBootConfig
		verifyResp, verifyErr := bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "non-bootconfig", true)
		validateVendorConfig(t, dut, "hostname Rotate", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)

		// SetBootConfig with BootConfig as nil
		setReq = &bcpb.SetBootConfigRequest{BootConfig: nil}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		setResp, err := bootconfigClient.SetBootConfig(context.Background(), setReq)
		if err != nil {
			t.Logf("Expected failure: SetBootConfig returned error: %v", err)
		} else {
			t.Errorf("SetBootConfig should have failed but succeeded with response: %v", setResp)
		}

		// Perform GetBootConfig operation after SetBootConfig
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		validateBootConfigConfiguration(t, dut, "nil-bootconfig", true)
		validateVendorConfig(t, dut, "hostname Rotate", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)
	})
	t.Run("Initial BootConfig Present: Test Set/Get BootConfig with invalid vendor configuration", func(t *testing.T) {
		dut := ondatra.DUT(t, "dut")

		// Start the boot config client.
		bootconfigClient := start(t)

		// Perform GetBootConfig operation when the initial boot config is not present
		getResp, getErr := bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		// Inline validation of the error
		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		want := &bpb.BootConfig{VendorConfig: []byte("grpc\n gnsi service certz ssl-profile-id CETZ_BOOTZ")}

		setReq := &bcpb.SetBootConfigRequest{BootConfig: want}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		setResp, err := bootconfigClient.SetBootConfig(context.Background(), setReq)
		if err != nil {
			t.Logf("Expected failure: SetBootConfig returned error: %v", err)
		} else {
			t.Errorf("SetBootConfig should have failed but succeeded with response: %v", setResp)
		}

		getResp, getErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}
		validateBootConfigConfiguration(t, dut, "invald-bootconfig", false)
		validateVendorConfig(t, dut, "hostname profile-id", false)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, true)
	})
	t.Run("Initial BootConfig Present: Test Set/Get BootConfig with empty & invalid config file configuration after RPSwitchover", func(t *testing.T) {
		dut := ondatra.DUT(t, "dut")

		// Start the boot config client.
		bootconfigClient := start(t)

		// Perform GetBootConfig operation after SetBootConfig
		verifyResp, verifyErr := bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyResp == nil {
			t.Fatalf("GetBootConfig failed: %v", verifyResp)
		}
		t.Logf("GetBootConfig ErrorResponse: %v", verifyErr)

		// Perform SetBootConfig operation with different inline vendor configuration
		want := &bpb.BootConfig{VendorConfig: []byte("ssh server rate-limit 6000")}

		setReq := &bcpb.SetBootConfigRequest{BootConfig: want}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		// Perform SetBootConfig operation
		setResp, setErr := bootconfigClient.SetBootConfig(context.Background(), setReq)
		if setErr != nil {
			t.Fatalf("SetBootConfig failed: %v", setErr)
		}
		t.Logf("SetBootConfig Response: %v", setResp)

		// Perform GetBootConfig operation after SetBootConfig
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "non-bootconfig", false)
		validateVendorConfig(t, dut, "ssh server rate-limit 1000", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)

		// SetBootConfig with empty configuration
		setReq = &bcpb.SetBootConfigRequest{BootConfig: &bpb.BootConfig{}}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		setResp, err := bootconfigClient.SetBootConfig(context.Background(), setReq)
		if err != nil {
			t.Logf("Expected failure: SetBootConfig returned error: %v", err)
		} else {
			t.Errorf("SetBootConfig should have failed but succeeded with response: %v", setResp)
		}

		// Perform GetBootConfig operation after SetBootConfig
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "empty-bootconfig", false)
		validateVendorConfig(t, dut, "ssh server rate-limit 1000", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)

		// RP Switchover.
		utils.Dorpfo(context.Background(), t, true)

		// validate NSR-Ready is ready
		redundancy_nsrState(context.Background(), t, true)

		// Start the boot config client.
		bootconfigClient = start(t)

		// Perform GetBootConfig operation after rpswitchover
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation after rpswitchover
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations after rpswitchover
		validateBootConfigConfiguration(t, dut, "rpso-bootconfig", false)
		validateVendorConfig(t, dut, "ssh server rate-limit 1000", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)

		// SetBootConfig with BootConfig as nil
		setReq = &bcpb.SetBootConfigRequest{BootConfig: nil}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		setResp, err = bootconfigClient.SetBootConfig(context.Background(), setReq)
		if err != nil {
			t.Logf("Expected failure: SetBootConfig returned error: %v", err)
		} else {
			t.Errorf("SetBootConfig should have failed but succeeded with response: %v", setResp)
		}

		// Perform GetBootConfig operation after SetBootConfig
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		validateBootConfigConfiguration(t, dut, "nil-bootconfig", false)
		validateVendorConfig(t, dut, "ssh server rate-limit 1000", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)

		// Define the path to the bootconfig file
		bootconfigPath := "testdata/bootconfig_invalid.txt"

		// SetBootConfig with invalid configuration file
		setReq = &bcpb.SetBootConfigRequest{BootConfig: getBootConfigFromFile(t, bootconfigPath)}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		setResp, err = bootconfigClient.SetBootConfig(context.Background(), setReq)
		if err != nil {
			t.Logf("Expected failure: SetBootConfig returned error: %v", err)
		} else {
			t.Errorf("SetBootConfig should have failed but succeeded with response: %v", setResp)
		}

		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "invalid-bootconfig", false)
		validateVendorConfig(t, dut, "ssh server rate-limit 1000", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)
	})
	t.Run("Initial BootConfig Present: Test Set/Get BootConfig with invalid file-based, nil & OC config after Router reload & process restart", func(t *testing.T) {
		dut := ondatra.DUT(t, "dut")

		// Start the boot config client.
		bootconfigClient := start(t)

		// Perform GetBootConfig operation after SetBootConfig
		verifyResp, verifyErr := bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyResp == nil {
			t.Fatalf("GetBootConfig failed: %v", verifyResp)
		}
		t.Logf("GetBootConfig ErrorResponse: %v", verifyErr)

		// Define the path to the bootconfig file
		bootconfigPath := "testdata/bootconfig_valid.txt"

		// Required BootConfig from file
		want := getBootConfigFromFile(t, bootconfigPath)

		// SetBootConfig with file-based configuration
		setReq := &bcpb.SetBootConfigRequest{BootConfig: want}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		// Perform SetBootConfig operation
		setResp, setErr := bootconfigClient.SetBootConfig(context.Background(), setReq)
		if setErr != nil {
			t.Fatalf("SetBootConfig failed: %v", setErr)
		}
		t.Logf("SetBootConfig Response: %v", setResp)

		// Perform GetBootConfig operation
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "non-bootconfig", true)
		validateVendorConfig(t, dut, "ssh server rate-limit 600", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)

		// Define the path to the bootconfig file
		bootconfigPath = "testdata/bootconfig_invalid.txt"

		// SetBootConfig with invalid configuration file
		setReq = &bcpb.SetBootConfigRequest{BootConfig: getBootConfigFromFile(t, bootconfigPath)}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		setResp, err := bootconfigClient.SetBootConfig(context.Background(), setReq)
		if err != nil {
			t.Logf("Expected failure: SetBootConfig returned error: %v", err)
		} else {
			t.Errorf("SetBootConfig should have failed but succeeded with response: %v", setResp)
		}

		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "non-bootconfig", true)
		validateVendorConfig(t, dut, "ssh server rate-limit 600", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)

		// Reload router
		perf.ReloadRouter(t, dut)

		// waiting for 30 seconds to yiny-resync
		time.Sleep(30 * time.Second)

		// Start the boot config client.
		bootconfigClient = start(t)

		// SetBootConfig with BootConfig as nil after router reload
		setReq = &bcpb.SetBootConfigRequest{BootConfig: nil}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		setResp, err = bootconfigClient.SetBootConfig(context.Background(), setReq)
		if err != nil {
			t.Logf("Expected failure: SetBootConfig returned error: %v", err)
		} else {
			t.Errorf("SetBootConfig should have failed but succeeded with response: %v", setResp)
		}

		// Perform GetBootConfig operation after SetBootConfig after router reload
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation after router reload
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "non-bootconfig", true)
		validateVendorConfig(t, dut, "ssh server rate-limit 600", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)

		// Define the path to the bootconfig file after router reload
		bootconfigPath = "testdata/bootconfig_invalid.txt"

		// SetBootConfig with invalid configuration file a
		setReq = &bcpb.SetBootConfigRequest{BootConfig: getBootConfigFromFile(t, bootconfigPath)}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		setResp, err = bootconfigClient.SetBootConfig(context.Background(), setReq)
		if err != nil {
			t.Logf("Expected failure: SetBootConfig returned error: %v", err)
		} else {
			t.Errorf("SetBootConfig should have failed but succeeded with response: %v", setResp)
		}

		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation after router reload
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations after router reload
		validateBootConfigConfiguration(t, dut, "non-bootconfig", true)
		validateVendorConfig(t, dut, "ssh server rate-limit 600", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)

		// Perform eMSD process restart
		t.Logf("Restarting emsd at %s", time.Now())
		perf.RestartProcess(t, dut, "emsd")
		t.Logf("Restart emsd finished at %s", time.Now())

		// GetBootConfig after process restart
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation after process restart
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations after process restart
		validateBootConfigConfiguration(t, dut, "non-bootconfig", true)
		validateVendorConfig(t, dut, "ssh server rate-limit 600", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)

		// SetBootConfig with invalid OC configuration
		setReq = &bcpb.SetBootConfigRequest{BootConfig: &bpb.BootConfig{OcConfig: []byte("invlid oc config")}}

		// Perform SetBootConfig operation
		setResp, err = bootconfigClient.SetBootConfig(context.Background(), setReq)
		if err != nil {
			t.Logf("Expected failure: SetBootConfig returned error: %v", err)
		} else {
			t.Errorf("SetBootConfig should have failed but succeeded with response: %v", setResp)
		}

		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation after process restart
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations after process restart
		validateBootConfigConfiguration(t, dut, "non-bootconfig", true)
		validateVendorConfig(t, dut, "ssh server rate-limit 600", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, false)

		// Delete Bootconfig file and verify the behaviour
		pathz.DeletePolicyData(t, dut, "boot_config_merged.txt")

		// Perform GetBootConfig again to verify persistence after deleting bootconfig file.
		getResp, getErr := bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		// Inline validation of the error
		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// Perform common validations after deleting the bootconfig file.
		validateBootConfigConfiguration(t, dut, "non-bootconfig", true)
		validateVendorConfig(t, dut, "ssh server rate-limit 600", true)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, true)

		// Perform eMSD process restart after deleting the bootconfig file
		t.Logf("Restarting emsd at %s", time.Now())
		perf.RestartProcess(t, dut, "emsd")
		t.Logf("Restart emsd finished at %s", time.Now())

		// Validate GetBootConfig operation after process restart
		getResp, getErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		// Inline validation of the error
		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// Validate BootConfig behaviour after deleting bootconfig and restarting emsd.
		validateBootConfigConfiguration(t, dut, "non-bootconfig", false)
		validateVendorConfig(t, dut, "ssh server rate-limit 600", false)
		validateNonBootConfig(t, dut, false)
		checkBootConfigFiles(t, dut, true)
	})
}

// Test BootZ with username alone in bootconfig.txt without password.
func TestBootZ_Credz_User(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnsiC := dut.RawAPIs().GNSI(t)
	accountNames := []string{"gfallback", "gfallback2"}
	passwords := []string{"test123", "cisco123"}
	gnmipasswords := []string{"gnmi123", "gnmi321"}
	newPasswords := []string{"rotate123", "rotate321"}

	// Create directory
	if err := os.Mkdir(credzFileName, 0755); err != nil {
		t.Fatalf("Error creating directory: %v", err)
	}
	defer os.RemoveAll(credzFileName)

	// Create users on the device
	var users []*oc.System_Aaa_Authentication_User
	for _, account := range accountNames {
		users = append(users, &oc.System_Aaa_Authentication_User{
			Username: &account,
			Role:     oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN,
		})
	}
	createUsersOnDevice(t, dut, users)

	// Call common setup and rotate function
	setupAndRotateCertificates(t, dut)

	testSetup(t, dut, WithoutPathz())
	defer bServer.Stop()
	defer dhcp.Stop()

	dutPreTestVersion := gnmi.Get(t, dut, gnmi.OC().System().SoftwareVersion().State())

	// Define BootConfig Test
	bootconfig := bootzTest{
		Name:            "Bootz-1: Valid config",
		VendorConfig:    baseConfig,
		ExpectedFailure: false,
	}

	t.Run(bootconfig.Name, func(t *testing.T) {
		if bootzServerFailed.Load() {
			t.Fatal("bootz server is down, check the test log for detailed error")
		}
		// Reset bootz logs
		bootzStatusLogs = bootzStatus{}
		bootzReqLogs = bootzLogs{}

		// Ensure no old DHCP log is causing an issue
		dhcpLease.CleanLog()

		chassisBootzConfig.GetConfig().BootConfig.VendorConfig = []byte(bootconfig.VendorConfig)
		em.ReplaceDevice(chassisEntity, chassisBootzConfig)

		ztpInitiateMgmtDhcp4(t, dut)

		// Run checkBootzStatus and capture test failure state
		checkBootzStatus(t, bootconfig.ExpectedFailure, bootzStatusTimeout)
	})
	dutBootzStatus(t, dut, 5*time.Second)
	dutPostTestVersion := gnmi.Get(t, dut, gnmi.OC().System().SoftwareVersion().State())

	if dutPreTestVersion != dutPostTestVersion {
		t.Fatalf("DUT software versions do not match, pretest: %s, posttest: %s", dutPreTestVersion, dutPostTestVersion)
	}
	t.Run("Verify Password Based Authentication", func(t *testing.T) {
		// Get target IP and Port
		targetIP, targetPort, err := getIpAndPortFromBindingFile()
		if err != nil {
			t.Fatalf("Error reading target IP and Port: %v", err)
		}

		// Verify SSH authentication with Credz Passwords after BootZ
		for i, account := range accountNames {
			fmt.Printf("Verifying SSH for User: %s, Password: %s\n", account, passwords[i])
			if err := createSSHClientAndVerify(targetIP, targetPort, sshPasswordParams{user: account, password: passwords[i]}, "password", nil); err != nil {
				t.Fatalf("Error in SSH connection with username %s", account)
			}
		}

		// Create new users and password on the device through gNMI SET
		var users []*oc.System_Aaa_Authentication_User
		for i, account := range accountNames {
			users = append(users, &oc.System_Aaa_Authentication_User{
				Username: &account,
				Role:     oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN,
				Password: &gnmipasswords[i],
			})
		}
		createUsersOnDevice(t, dut, users)

		// Verify SSH authentication with Credz Passwords after gNMI SET
		for i, account := range accountNames {
			fmt.Printf("Verifying SSH for User: %s, Password: %s\n", account, passwords[i])
			if err := createSSHClientAndVerify(targetIP, targetPort, sshPasswordParams{user: account, password: passwords[i]}, "password", nil); err != nil {
				t.Fatalf("Error in SSH connection with username %s", account)
			}
		}

		// Verify SSH authentication with gNMI Passwords (Negative Test)
		for i, account := range accountNames {
			fmt.Printf("Verifying SSH (Expected Failure) for User: %s, Password: %s\n", account, gnmipasswords[i])
			err := createSSHClientAndVerify(targetIP, targetPort, sshPasswordParams{user: account, password: gnmipasswords[i]}, "password", nil)
			if err == nil {
				t.Fatalf("Unexpected success in SSH connection with username %s using gNMI password. Expected failure.", account)
			} else {
				fmt.Printf("Expected failure occurred for user %s using gNMI password.\n", account)
			}
		}

		// Rotate account credentials
		passreq := credz.PasswordRequest{}
		for i, account := range accountNames {
			passreq.Accounts = append(passreq.Accounts, &credz.PasswordRequest_Account{
				Account:   account,
				Password:  &credz.PasswordRequest_Password{Value: &credz.PasswordRequest_Password_Plaintext{Plaintext: newPasswords[i]}},
				Version:   "1.8",
				CreatedOn: 123,
			})
		}
		stream, err := gnsiC.Credentialz().RotateAccountCredentials(context.Background())
		if err != nil {
			t.Fatalf("Error rotating credentials: %v", err)
		}
		defer stream.CloseSend()

		roatateAccountCredentialsForPassword(t, stream, passreq)
		finalizeAccountRequest(t, stream)

		// Verify SSH authentication with new Credz Rotate passwords
		for i, account := range accountNames {
			fmt.Printf("Verifying SSH for User: %s, Password: %s\n", account, newPasswords[i])
			if err := createSSHClientAndVerify(targetIP, targetPort, sshPasswordParams{user: account, password: newPasswords[i]}, "password", nil); err != nil {
				t.Fatalf("Error in SSH connection with username %s", account)
			}
		}

		// Verify SSH authentication with gNMI Passwords (Negative Test)
		for i, account := range accountNames {
			fmt.Printf("Verifying SSH (Expected Failure) for User: %s, Password: %s\n", account, gnmipasswords[i])
			err := createSSHClientAndVerify(targetIP, targetPort, sshPasswordParams{user: account, password: gnmipasswords[i]}, "password", nil)
			if err == nil {
				t.Fatalf("Unexpected success in SSH connection with username %s using gNMI password. Expected failure.", account)
			} else {
				fmt.Printf("Expected failure occurred for user %s using gNMI password.\n", account)
			}
		}

		// Verify SSH authentication with BootZ Passwords (Negative Test)
		for i, account := range accountNames {
			fmt.Printf("Verifying SSH (Expected Failure) for User: %s, Password: %s\n", account, passwords[i])
			err := createSSHClientAndVerify(targetIP, targetPort, sshPasswordParams{user: account, password: passwords[i]}, "password", nil)
			if err == nil {
				t.Fatalf("Unexpected success in SSH connection with username %s using gNMI password. Expected failure.", account)
			} else {
				fmt.Printf("Expected failure occurred for user %s using gNMI password.\n", account)
			}
		}
	})
}

// Test BootZ with username and password in bootconfig.txt.
func TestBootz_Credz_User_Passwd(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnsiC := dut.RawAPIs().GNSI(t)
	accountNames := []string{"gfallback", "gfallback2"}
	bootpasswords := []string{"rotate123", "rotate321"}
	passwords := []string{"test123", "cisco123"}
	gnmipasswords := []string{"gnmi123", "gnmi321"}
	newPasswords := []string{"credz123", "credz321"}

	// Create directory
	if err := os.Mkdir(credzFileName, 0755); err != nil {
		t.Fatalf("Error creating directory: %v", err)
	}
	defer os.RemoveAll(credzFileName)

	// Call common setup and rotate function
	setupAndRotateCertificates(t, dut)

	testSetup(t, dut, WithoutPathz())
	defer bServer.Stop()
	defer dhcp.Stop()

	dutPreTestVersion := gnmi.Get(t, dut, gnmi.OC().System().SoftwareVersion().State())

	// Define BootConfig Test
	bootconfig := bootzTest{
		Name:            "Bootz-1: Valid config",
		VendorConfig:    baseConfig,
		ExpectedFailure: false,
	}

	t.Run(bootconfig.Name, func(t *testing.T) {
		if bootzServerFailed.Load() {
			t.Fatal("bootz server is down, check the test log for detailed error")
		}
		// Reset bootz logs
		bootzStatusLogs = bootzStatus{}
		bootzReqLogs = bootzLogs{}

		// Ensure no old DHCP log is causing an issue
		dhcpLease.CleanLog()

		chassisBootzConfig.GetConfig().BootConfig.VendorConfig = []byte(bootconfig.VendorConfig)
		em.ReplaceDevice(chassisEntity, chassisBootzConfig)

		ztpInitiateMgmtDhcp4(t, dut)

		// Run checkBootzStatus and capture test failure state
		checkBootzStatus(t, bootconfig.ExpectedFailure, bootzStatusTimeout)
	})
	dutBootzStatus(t, dut, 5*time.Second)
	dutPostTestVersion := gnmi.Get(t, dut, gnmi.OC().System().SoftwareVersion().State())

	if dutPreTestVersion != dutPostTestVersion {
		t.Fatalf("DUT software versions do not match, pretest: %s, posttest: %s", dutPreTestVersion, dutPostTestVersion)
	}

	t.Run("Verify Password Based Authentication", func(t *testing.T) {
		// Get target IP and Port
		targetIP, targetPort, err := getIpAndPortFromBindingFile()
		if err != nil {
			t.Fatalf("Error reading target IP and Port: %v", err)
		}

		// Verify SSH authentication with Credz Passwords after BootZ
		for i, account := range accountNames {
			fmt.Printf("Verifying SSH for User: %s, Password: %s\n", account, passwords[i])
			if err := createSSHClientAndVerify(targetIP, targetPort, sshPasswordParams{user: account, password: passwords[i]}, "password", nil); err != nil {
				t.Fatalf("Error in SSH connection with username %s", account)
			}
		}

		// Create new users and password on the device through gNMI SET
		var users []*oc.System_Aaa_Authentication_User
		for i, account := range accountNames {
			users = append(users, &oc.System_Aaa_Authentication_User{
				Username: &account,
				Role:     oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN,
				Password: &gnmipasswords[i],
			})
		}
		createUsersOnDevice(t, dut, users)

		// Verify SSH authentication with Credz Passwords after gNMI SET
		for i, account := range accountNames {
			fmt.Printf("Verifying SSH for User: %s, Password: %s\n", account, passwords[i])
			if err := createSSHClientAndVerify(targetIP, targetPort, sshPasswordParams{user: account, password: passwords[i]}, "password", nil); err != nil {
				t.Fatalf("Error in SSH connection with username %s", account)
			}
		}

		// Verify SSH authentication with gNMI Passwords (Negative Test)
		for i, account := range accountNames {
			fmt.Printf("Verifying SSH (Expected Failure) for User: %s, Password: %s\n", account, gnmipasswords[i])
			err := createSSHClientAndVerify(targetIP, targetPort, sshPasswordParams{user: account, password: gnmipasswords[i]}, "password", nil)
			if err == nil {
				t.Fatalf("Unexpected success in SSH connection with username %s using gNMI password. Expected failure.", account)
			} else {
				fmt.Printf("Expected failure occurred for user %s using gNMI password.\n", account)
			}
		}

		// Verify SSH authentication with Boot_config.txt Passwords (Negative Test)
		for i, account := range accountNames {
			fmt.Printf("Verifying SSH (Expected Failure) for User: %s, Password: %s\n", account, bootpasswords[i])
			err := createSSHClientAndVerify(targetIP, targetPort, sshPasswordParams{user: account, password: bootpasswords[i]}, "password", nil)
			if err == nil {
				t.Fatalf("Unexpected success in SSH connection with username %s using gNMI password. Expected failure.", account)
			} else {
				fmt.Printf("Expected failure occurred for user %s using gNMI password.\n", account)
			}
		}

		// Rotate account credentials
		passreq := credz.PasswordRequest{}
		for i, account := range accountNames {
			passreq.Accounts = append(passreq.Accounts, &credz.PasswordRequest_Account{
				Account:   account,
				Password:  &credz.PasswordRequest_Password{Value: &credz.PasswordRequest_Password_Plaintext{Plaintext: newPasswords[i]}},
				Version:   "1.8",
				CreatedOn: 123,
			})
		}
		stream, err := gnsiC.Credentialz().RotateAccountCredentials(context.Background())
		if err != nil {
			t.Fatalf("Error rotating credentials: %v", err)
		}
		defer stream.CloseSend()

		roatateAccountCredentialsForPassword(t, stream, passreq)
		finalizeAccountRequest(t, stream)

		// Verify SSH authentication with new passwords
		for i, account := range accountNames {
			fmt.Printf("Verifying SSH for User: %s, Password: %s\n", account, newPasswords[i])
			if err := createSSHClientAndVerify(targetIP, targetPort, sshPasswordParams{user: account, password: newPasswords[i]}, "password", nil); err != nil {
				t.Fatalf("Error in SSH connection with username %s", account)
			}
		}

		// Verify SSH authentication with gNMI Passwords (Negative Test)
		for i, account := range accountNames {
			fmt.Printf("Verifying SSH (Expected Failure) for User: %s, Password: %s\n", account, gnmipasswords[i])
			err := createSSHClientAndVerify(targetIP, targetPort, sshPasswordParams{user: account, password: gnmipasswords[i]}, "password", nil)
			if err == nil {
				t.Fatalf("Unexpected success in SSH connection with username %s using gNMI password. Expected failure.", account)
			} else {
				fmt.Printf("Expected failure occurred for user %s using gNMI password.\n", account)
			}
		}

		// Verify SSH authentication with BootZ Passwords (Negative Test)
		for i, account := range accountNames {
			fmt.Printf("Verifying SSH (Expected Failure) for User: %s, Password: %s\n", account, passwords[i])
			err := createSSHClientAndVerify(targetIP, targetPort, sshPasswordParams{user: account, password: passwords[i]}, "password", nil)
			if err == nil {
				t.Fatalf("Unexpected success in SSH connection with username %s using gNMI password. Expected failure.", account)
			} else {
				fmt.Printf("Expected failure occurred for user %s using gNMI password.\n", account)
			}
		}
	})
}

func testBootConfigValidation(t *testing.T, dut *ondatra.DUTDevice) {
	t.Run("BootConfig Validation Post BootZ Success", func(t *testing.T) {
		// Start the boot config client.
		bootconfigClient := start(t)

		// Perform GetBootConfig operation after SetBootConfig
		verifyResp, verifyErr := bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig ErrorResponse: %v", verifyErr)
		t.Logf("GetBootConfig Response: %v", verifyResp)
		if verifyResp == nil {
			t.Fatalf("GetBootConfig failed: %v", verifyResp)
		}
		t.Logf("GetBootConfig ErrorResponse: %v", verifyErr)

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "test-bootconfig", true)
		validateVendorConfig(t, dut, "ssh server rate-limit 700", true)
		checkBootConfigFiles(t, dut, false)
		validateNonBootConfig(t, dut, true)

		// Define the path to the bootconfig file
		bootconfigPath := "testdata/bootconfig_valid.txt"

		// Required BootConfig from file
		want := getBootConfigFromFile(t, bootconfigPath)

		// SetBootConfig with file-based configuration
		setReq := &bcpb.SetBootConfigRequest{BootConfig: want}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		// Perform SetBootConfig operation
		setResp, setErr := bootconfigClient.SetBootConfig(context.Background(), setReq)
		if setErr != nil {
			t.Fatalf("SetBootConfig failed: %v", setErr)
		}
		t.Logf("SetBootConfig Response: %v", setResp)

		// Perform GetBootConfig operation
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "non-bootconfig", true)
		validateVendorConfig(t, dut, "ssh server rate-limit 700", true)
		checkBootConfigFiles(t, dut, false)
		validateNonBootConfig(t, dut, true)

		// Define the path to the bootconfig file
		bootconfigPath = "testdata/bootconfig_invalid.txt"

		// SetBootConfig with invalid configuration file
		setReq = &bcpb.SetBootConfigRequest{BootConfig: getBootConfigFromFile(t, bootconfigPath)}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		setResp, err := bootconfigClient.SetBootConfig(context.Background(), setReq)
		if err != nil {
			t.Logf("Expected failure: SetBootConfig returned error: %v", err)
		} else {
			t.Errorf("SetBootConfig should have failed but succeeded with response: %v", setResp)
		}

		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "non-bootconfig", true)
		validateVendorConfig(t, dut, "ssh server rate-limit 700", true)
		checkBootConfigFiles(t, dut, false)
		validateNonBootConfig(t, dut, true)

		// Reload router
		perf.ReloadRouter(t, dut)

		// Start the boot config client.
		bootconfigClient = start(t)

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "non-bootconfig", true)
		validateVendorConfig(t, dut, "ssh server rate-limit 700", true)
		checkBootConfigFiles(t, dut, false)
		validateNonBootConfig(t, dut, true)

		// SetBootConfig with BootConfig as nil after router reload
		setReq = &bcpb.SetBootConfigRequest{BootConfig: nil}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		setResp, err = bootconfigClient.SetBootConfig(context.Background(), setReq)
		if err != nil {
			t.Logf("Expected failure: SetBootConfig returned error: %v", err)
		} else {
			t.Errorf("SetBootConfig should have failed but succeeded with response: %v", setResp)
		}

		// Perform GetBootConfig operation after SetBootConfig after router reload
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation after router reload
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "non-bootconfig", true)
		validateVendorConfig(t, dut, "ssh server rate-limit 700", true)
		checkBootConfigFiles(t, dut, false)
		validateNonBootConfig(t, dut, true)

		// SetBootConfig with invalid OC configuration after router reload
		setReq = &bcpb.SetBootConfigRequest{BootConfig: &bpb.BootConfig{OcConfig: []byte("invlid oc config")}}

		// Perform SetBootConfig operation after router reload
		setResp, err = bootconfigClient.SetBootConfig(context.Background(), setReq)
		if err != nil {
			t.Logf("Expected failure: SetBootConfig returned error: %v", err)
		} else {
			t.Errorf("SetBootConfig should have failed but succeeded with response: %v", setResp)
		}

		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation after router reload
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations after router reload
		validateBootConfigConfiguration(t, dut, "non-bootconfig", true)
		validateVendorConfig(t, dut, "ssh server rate-limit 700", true)
		checkBootConfigFiles(t, dut, false)
		validateNonBootConfig(t, dut, true)

		// Perform eMSD process restart
		t.Logf("Restarting emsd at %s", time.Now())
		perf.RestartProcess(t, dut, "emsd")
		t.Logf("Restart emsd finished at %s", time.Now())

		// GetBootConfig after process restart
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation after process restart
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations after process restart
		validateBootConfigConfiguration(t, dut, "non-bootconfig", true)
		validateVendorConfig(t, dut, "ssh server rate-limit 700", true)
		checkBootConfigFiles(t, dut, false)
		validateNonBootConfig(t, dut, true)

		// Perform SetBootConfig operation again with different inline vendor configuration
		want = &bpb.BootConfig{VendorConfig: []byte("ssh server rate-limit 6000")}

		setReq = &bcpb.SetBootConfigRequest{BootConfig: want}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		// Perform SetBootConfig operation
		setResp, setErr = bootconfigClient.SetBootConfig(context.Background(), setReq)
		if setErr != nil {
			t.Fatalf("SetBootConfig failed: %v", setErr)
		}
		t.Logf("SetBootConfig Response: %v", setResp)

		// Perform GetBootConfig operation after SetBootConfig
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "non-bootconfig", false)
		validateVendorConfig(t, dut, "ssh server rate-limit 1000", true)
		checkBootConfigFiles(t, dut, false)
		validateNonBootConfig(t, dut, true)

		// Perform eMSD process restart
		t.Logf("Restarting emsd at %s", time.Now())
		perf.RestartProcess(t, dut, "emsd")
		t.Logf("Restart emsd finished at %s", time.Now())

		// Perform GetBootConfig operation after SetBootConfig
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "restart", false)
		validateVendorConfig(t, dut, "ssh server rate-limit 1000", true)
		checkBootConfigFiles(t, dut, false)
		validateNonBootConfig(t, dut, true)

		// Define the path to the bootconfig file
		bootconfigPath = "testdata/bootconfig_valid.txt"
		// Define OC configuration
		ocConfig := getBootConfigFromFile(t, bootconfigPath).OcConfig

		// Required OC configuration for set and get bootconfig
		want = &bpb.BootConfig{OcConfig: ocConfig}

		// SetBootConfig with only valid OC configuration
		setReq = &bcpb.SetBootConfigRequest{BootConfig: want}
		t.Logf("Sending SetBootConfig request: %v", setReq)

		// Perform SetBootConfig operation
		setResp, setErr = bootconfigClient.SetBootConfig(context.Background(), setReq)
		if setErr != nil {
			t.Fatalf("SetBootConfig failed: %v", setErr)
		}
		t.Logf("SetBootConfig Response: %v", setResp)

		// Perform GetBootConfig operation after SetBootConfig
		verifyResp, verifyErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		if verifyErr != nil {
			t.Fatalf("GetBootConfig failed: %v", verifyErr)
		}
		t.Logf("GetBootConfig Response: %v", verifyResp)

		// Validate BootConfig matches expectation
		if !proto.Equal(verifyResp.GetBootConfig(), want) {
			t.Fatalf("BootConfig mismatch: got %v, want %v", verifyResp.GetBootConfig(), want)
		}
		t.Logf("BootConfig matched successfully: %v", verifyResp.GetBootConfig())

		// Perform common validations
		validateBootConfigConfiguration(t, dut, "non-bootconfig", true)
		validateVendorConfig(t, dut, "hostname Rotate", true)
		checkBootConfigFiles(t, dut, false)
		validateNonBootConfig(t, dut, true)

		// Delete Bootconfig file and verify the behaviour
		pathz.DeletePolicyData(t, dut, "boot_config_merged.txt")

		// Perform GetBootConfig again to verify persistence after deleting bootconfig file.
		getResp, getErr := bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		// Inline validation of the error
		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// Perform common validations after deleting the bootconfig file.
		validateBootConfigConfiguration(t, dut, "non-bootconfig", true)
		validateVendorConfig(t, dut, "hostname Rotate", true)
		checkBootConfigFiles(t, dut, true)
		validateNonBootConfig(t, dut, true)

		// Perform eMSD process restart after deleting the bootconfig file
		t.Logf("Restarting emsd at %s", time.Now())
		perf.RestartProcess(t, dut, "emsd")
		t.Logf("Restart emsd finished at %s", time.Now())

		// Validate GetBootConfig operation after process restart
		getResp, getErr = bootconfigClient.GetBootConfig(context.Background(), &bcpb.GetBootConfigRequest{})
		t.Logf("GetBootConfig Response: %v", getResp)

		// Inline validation of the error
		if status.Code(getErr) == codes.NotFound && strings.Contains(getErr.Error(), "boot config not found") {
			t.Logf("Expected error received: %v", getErr)
		} else {
			t.Errorf("Unexpected error: got %v, want rpc error: code = %v desc = %s", getErr, codes.NotFound, "boot config not found")
		}

		// Validate BootConfig behaviour after deleting bootconfig and restarting emsd.
		validateBootConfigConfiguration(t, dut, "non-bootconfig", false)
		validateVendorConfig(t, dut, "ssh server rate-limit 600", false)
		checkBootConfigFiles(t, dut, true)
		validateNonBootConfig(t, dut, true)
	})
}

func gNSIvalidation(t *testing.T, dut *ondatra.DUTDevice, clientKeyNames []string, accountName string, targetIP string, targetPort int, authenticationType []string) {
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
			err = createSSHClientAndVerify(targetIP, targetPort, sshParams, authenticationType[i], nil)
			if err != nil {
				errCount = errCount + 1
			}
		}
		if errCount != 0 {
			t.Fatalf("Error in establishing SSH connection with Public Key based")
		}
	})

	t.Run("IMBootZ Pathz: Test Default Pathz Policy Behaviour", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			gnsiC, err := dut.RawAPIs().BindingDUT().DialGNSI(context.Background())
			if err != nil {
				t.Fatalf("Could not connect gnsi %v", err)
			}

			// Define probe request
			probeReq := &pathzpb.ProbeRequest{
				Mode:           pathzpb.Mode_MODE_WRITE,
				User:           d.sshUser,
				Path:           &gpb.Path{Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			// Define expected response
			want := &pathzpb.ProbeResponse{
				Version: "1",
				Action:  pathzpb.Action_ACTION_PERMIT,
			}

			// Perform Probe request
			t.Logf("Probe Request : %v", probeReq)
			got, err := gnsiC.Pathz().Probe(context.Background(), probeReq)
			t.Logf("Probe Response : %v", got)

			if err != nil {
				t.Fatalf("Probe() unexpected error: %v", err)
			}

			// Check for differences between expected and actual responses
			if d := cmp.Diff(want, got, protocmp.Transform()); d != "" {
				t.Fatalf("Probe() unexpected diff: %s", d)
			}

			get_res := &pathzpb.GetResponse{
				Version: "1",
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_PERMIT,
					}},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, err := gnsiC.Pathz().Get(context.Background(), getReq_Sand)
			t.Logf("Sandbox Response : %v", sand_res)
			t.Logf("Sandbox Error : %v", err)

			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := gnsiC.Pathz().Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}

			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "1", false)

			// Verify the pathz policy statistics.
			expectedStats := map[string]int{
				"ProbeRequests": 1,
				"GetRequests":   2,
				"GetErrors":     1,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

			// Perform eMSD process restart
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform Probe request
			t.Logf("Probe Request : %v", probeReq)
			got, err = gnsiC.Pathz().Probe(context.Background(), probeReq)
			t.Logf("Probe Response : %v", got)

			if err != nil {
				t.Fatalf("Probe() unexpected error: %v", err)
			}

			// Check for differences between expected and actual responses
			if d := cmp.Diff(want, got, protocmp.Transform()); d != "" {
				t.Fatalf("Probe() unexpected diff: %s", d)
			}

			// Perform GET operations for sandbox policy instance after process restart
			sand_res_after_process_restart, err := gnsiC.Pathz().Get(context.Background(), getReq_Sand)
			t.Logf("Sandbox Response : %v", sand_res_after_process_restart)
			t.Logf("Sandbox Error : %v", err)

			if d := cmp.Diff(get_res, sand_res_after_process_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Perform GET operations for active policy instance after process restart.
			actv_res_after_process_restart, err := gnsiC.Pathz().Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_process_restart, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "1", false)

			// Verify the pathz policy statistics.
			expectedStats = map[string]int{
				"ProbeRequests": 1,
				"GetRequests":   2,
				"GetErrors":     1,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

			// Reload router
			perf.ReloadRouter(t, dut)

			// Perform Probe request
			gnsiC, err = dut.RawAPIs().BindingDUT().DialGNSI(context.Background())
			if err != nil {
				t.Fatalf("Could not connect gnsi %v", err)
			}

			t.Logf("Probe Request : %v", probeReq)
			got, err = gnsiC.Pathz().Probe(context.Background(), probeReq)
			t.Logf("Probe Response : %v", got)

			if err != nil {
				t.Fatalf("Probe() unexpected error: %v", err)
			}

			// Check for differences between expected and actual responses
			if d := cmp.Diff(want, got, protocmp.Transform()); d != "" {
				t.Fatalf("Probe() unexpected diff: %s", d)
			}

			// Perform GET operations for sandbox policy instance after router reload
			sand_res_after_reload, err := gnsiC.Pathz().Get(context.Background(), getReq_Sand)
			t.Logf("Sandbox Response : %v", sand_res_after_reload)
			t.Logf("Sandbox Error : %v", err)

			if d := cmp.Diff(get_res, sand_res_after_reload, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Perform GET operations for active policy instance after router reload
			actv_res_after_router_reload, err := gnsiC.Pathz().Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_router_reload, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "1", false)

			// Verify the pathz policy statistics.
			expectedStats = map[string]int{
				"ProbeRequests": 1,
				"GetRequests":   2,
				"GetErrors":     1,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)
		}
	})
	t.Run("IMBootZ Pathz: Test Rotate New Pathz Policy & gNMI Operation Behaviour", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {

			// Get default hostname from the device
			state := gnmi.OC().System().Hostname()
			val := gnmi.Get(t, dut, state.State())
			t.Logf("gNMI Get Response: %v", val)

			gnsiC, err := dut.RawAPIs().BindingDUT().DialGNSI(context.Background())
			if err != nil {
				t.Fatalf("Could not connect gnsi %v", err)
			}

			// Define probe request
			probeReq_sand := &pathzpb.ProbeRequest{
				Mode:           pathzpb.Mode_MODE_WRITE,
				User:           d.sshUser,
				Path:           &gpb.Path{Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			// Define expected response
			want_sandbox := &pathzpb.ProbeResponse{
				Version: "10",
				Action:  pathzpb.Action_ACTION_DENY,
			}

			// Define probe request
			probeReq_acvt := &pathzpb.ProbeRequest{
				Mode:           pathzpb.Mode_MODE_WRITE,
				User:           d.sshUser,
				Path:           &gpb.Path{Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			// Define expected response
			want_acvt := &pathzpb.ProbeResponse{
				Version: "1",
				Action:  pathzpb.Action_ACTION_PERMIT,
			}

			sand_get_res := &pathzpb.GetResponse{
				Version:   "10",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_DENY,
					}},
				},
			}

			acvt_get_res := &pathzpb.GetResponse{
				Version: "1",
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_PERMIT,
					}},
				},
			}

			req_1 := &pathzpb.RotateRequest{
				RotateRequest: &pathzpb.RotateRequest_UploadRequest{
					UploadRequest: &pathzpb.UploadRequest{
						Version:   "10",
						CreatedOn: createdtime,
						Policy: &pathzpb.AuthorizationPolicy{
							Rules: []*pathzpb.AuthorizationRule{{
								Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
								Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
								Mode:      pathzpb.Mode_MODE_WRITE,
								Action:    pathzpb.Action_ACTION_DENY,
							}},
						},
					},
				},
			}

			rot_1, _ := gnsiC.Pathz().Rotate(context.Background())
			t.Logf("Request Sent: %s", req_1)
			if err := rot_1.Send(req_1); err != nil {
				t.Logf("Rec Err %v", err)
				t.Fatal(err)
			}

			received, err := rot_1.Recv()
			t.Logf("Received Request: %s", received)
			t.Logf("Rec Err %v", err)

			time.Sleep(3 * time.Second)

			// Perform Probe request
			t.Logf("Probe Request : %v", probeReq_sand)
			got, err := gnsiC.Pathz().Probe(context.Background(), probeReq_sand)
			t.Logf("Probe Response : %v", got)

			if err != nil {
				t.Fatalf("Probe() unexpected error: %v", err)
			}

			// Check for differences between expected and actual responses
			if d := cmp.Diff(want_sandbox, got, protocmp.Transform()); d != "" {
				t.Fatalf("Probe() unexpected diff: %s", d)
			}

			// Perform Probe request
			t.Logf("Probe Request : %v", probeReq_acvt)
			got, err = gnsiC.Pathz().Probe(context.Background(), probeReq_acvt)
			t.Logf("Probe Response : %v", got)

			if err != nil {
				t.Fatalf("Probe() unexpected error: %v", err)
			}

			// Check for differences between expected and actual responses
			if d := cmp.Diff(want_acvt, got, protocmp.Transform()); d != "" {
				t.Fatalf("Probe() unexpected diff: %s", d)
			}

			// Verify the pathz policy statistics.
			expectedStats := map[string]int{
				"RotationsInProgressCount": 0,
				"PolicyRotations":          1,
				"PolicyRotationErrors":     0,
				"PolicyUploadRequests":     1,
				"ProbeRequests":            3,
				"GetRequests":              2,
				"GetErrors":                1,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

			// Perform Rotate request 2
			req_2 := &pathzpb.RotateRequest{
				RotateRequest: &pathzpb.RotateRequest_UploadRequest{
					UploadRequest: &pathzpb.UploadRequest{
						Version:   "2",
						CreatedOn: createdtime,
						Policy: &pathzpb.AuthorizationPolicy{
							Rules: []*pathzpb.AuthorizationRule{{
								Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "lldp"}, {Name: "config"}, {Name: "enabled"}}},
								Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
								Mode:      pathzpb.Mode_MODE_WRITE,
								Action:    pathzpb.Action_ACTION_PERMIT,
							}},
						},
					},
				},
			}

			rot_2, _ := gnsiC.Pathz().Rotate(context.Background())
			t.Logf("Request Sent: %s", req_2)
			if err := rot_2.Send(req_2); err != nil {
				t.Logf("Rec Err %v", err)
				t.Fatal(err)
			}

			received, err = rot_2.Recv()
			t.Logf("Received Request: %s", received)
			t.Logf("Rec Err %v", err)

			// Perform Probe request
			t.Logf("Probe Request : %v", probeReq_sand)
			got, err = gnsiC.Pathz().Probe(context.Background(), probeReq_sand)
			t.Logf("Probe Response : %v", got)

			if err != nil {
				t.Fatalf("Probe() unexpected error: %v", err)
			}

			// Check for differences between expected and actual responses
			if d := cmp.Diff(want_sandbox, got, protocmp.Transform()); d != "" {
				t.Fatalf("Probe() unexpected diff: %s", d)
			}

			// Perform Probe request
			t.Logf("Probe Request : %v", probeReq_acvt)
			got, err = gnsiC.Pathz().Probe(context.Background(), probeReq_acvt)
			t.Logf("Probe Response : %v", got)

			if err != nil {
				t.Fatalf("Probe() unexpected error: %v", err)
			}

			// Check for differences between expected and actual responses
			if d := cmp.Diff(want_acvt, got, protocmp.Transform()); d != "" {
				t.Fatalf("Probe() unexpected diff: %s", d)
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, err := gnsiC.Pathz().Get(context.Background(), getReq_Sand)
			t.Logf("Sandbox Response : %v", sand_res)
			t.Logf("Sandbox Error : %v", err)

			if d := cmp.Diff(sand_get_res, sand_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := gnsiC.Pathz().Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz Active Get request is failed on device %s", dut.Name())
			}

			if d := cmp.Diff(acvt_get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform gNMI SET operations
			validategNMISETOperations(t, dut, false, val)

			// Perform GET operation and validate the result.
			portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// // Perform gNMI SET operations with for undefined pathz policy xpath.
			path := gnmi.OC().Lldp().Enabled()
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", false, true, 0, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Verify the pathz policy statistics.
			expectedStats = map[string]int{
				"RotationsInProgressCount": 1,
				"PolicyRotations":          2,
				"PolicyRotationErrors":     0,
				"PolicyUploadRequests":     1,
				"ProbeRequests":            5,
				"GetRequests":              4,
				"GetErrors":                1,
				"GnmiPathLeaves":           2,
				"GnmiSetPathDeny":          1,
				"GnmiSetPathPermit":        3,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

			// Finalize
			req := &pathzpb.RotateRequest{
				RotateRequest: &pathzpb.RotateRequest_FinalizeRotation{},
			}
			// Perform Rotate request
			t.Logf("Request Sent: %s", req)
			if err := rot_1.Send(req); err != nil {
				t.Logf("Rec Err %v", err)
				t.Fatal(err)
			}

			time.Sleep(3 * time.Second)

			// Define probe request
			probeReq_acvt = &pathzpb.ProbeRequest{
				Mode:           pathzpb.Mode_MODE_WRITE,
				User:           d.sshUser,
				Path:           &gpb.Path{Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			// Define expected response
			want_acvt = &pathzpb.ProbeResponse{
				Version: "10",
				Action:  pathzpb.Action_ACTION_DENY,
			}

			get_res := &pathzpb.GetResponse{
				Version:   "10",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_DENY,
					}},
				},
			}

			time.Sleep(5 * time.Second)

			// Perform Probe request for sandbox policy instance
			t.Logf("Probe Request : %v", probeReq_sand)
			got, err = gnsiC.Pathz().Probe(context.Background(), probeReq_sand)
			t.Logf("Probe Response : %v", got)
			t.Logf("Probe Error : %v", err)

			// Check for differences between expected and actual responses
			if d := cmp.Diff(want_sandbox, got, protocmp.Transform()); d == "" {
				t.Fatalf("Probe() unexpected diff: %s", d)
			}

			// Perform Probe request for active policy instance
			t.Logf("Probe Request : %v", probeReq_acvt)
			got, err = gnsiC.Pathz().Probe(context.Background(), probeReq_acvt)
			t.Logf("Probe Response : %v", got)

			if err != nil {
				t.Fatalf("Probe() unexpected error: %v", err)
			}

			// Check for differences between expected and actual responses
			if d := cmp.Diff(want_acvt, got, protocmp.Transform()); d != "" {
				t.Fatalf("Probe() unexpected diff: %s", d)
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand = &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ = gnsiC.Pathz().Get(context.Background(), getReq_Sand)
			t.Logf("Sanbox Response: %s", sand_res)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv = &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err = gnsiC.Pathz().Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}

			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform gNMI SET operations after Pathz rotate finalize
			validategNMISETOperations(t, dut, true, val)

			// Perform GET operation and validate the result after Pathz rotate finalize
			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Perform gNMI SET operations with for undefined pathz policy xpath after Pathz rotate finalize
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "10", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 2, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, true, 3, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Verify the pathz policy statistics.
			expectedStats = map[string]int{
				"RotationsInProgressCount": 1,
				"PolicyRotations":          2,
				"PolicyUploadRequests":     1,
				"PolicyFinalize":           1,
				"ProbeRequests":            7,
				"GetRequests":              6,
				"GetErrors":                2,
				"GnmiPathLeaves":           2,
				"GnmiSetPathDeny":          5,
				"GnmiSetPathPermit":        3,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

			// Perform eMSD process restart
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform Probe request for sandbox policy instance after process restart
			t.Logf("Probe Request : %v", probeReq_sand)
			got, err = gnsiC.Pathz().Probe(context.Background(), probeReq_sand)
			t.Logf("Probe Response : %v", got)
			t.Logf("Probe Error : %v", err)

			// Check for differences between expected and actual responses after process restart
			if d := cmp.Diff(want_sandbox, got, protocmp.Transform()); d == "" {
				t.Fatalf("Probe() unexpected diff: %s", d)
			}

			// Perform Probe request for active policy instance after process restart
			t.Logf("Probe Request : %v", probeReq_acvt)
			got, err = gnsiC.Pathz().Probe(context.Background(), probeReq_acvt)
			t.Logf("Probe Response : %v", got)

			if err != nil {
				t.Fatalf("Probe() unexpected error: %v", err)
			}

			// Check for differences between expected and actual responses after process restart
			if d := cmp.Diff(want_acvt, got, protocmp.Transform()); d != "" {
				t.Fatalf("Probe() unexpected diff: %s", d)
			}

			// Perform GET operations for sandbox policy instance after process restart
			getReq_Sand = &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ = gnsiC.Pathz().Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("ProcessRestart: Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform GET operations for active policy instance after process restart
			getReq_Actv = &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err = gnsiC.Pathz().Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("ProcessRestart: Pathz Get request is failed on device %s", dut.Name())
			}

			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("ProcessRestart:Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform gNMI SET operations after process restart
			validategNMISETOperations(t, dut, true, val)

			// Perform GET operation and validate the result after process restart
			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Perform gNMI SET operations with for undefined pathz policy xpath after process restart
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info after process restart
			pathz.VerifyPolicyInfo(t, dut, createdtime, "10", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Verify the pathz policy statistics after process restart.
			expectedStats = map[string]int{
				"RotationsInProgressCount": 0,
				"PolicyRotations":          0,
				"PolicyUploadRequests":     0,
				"PolicyFinalize":           0,
				"ProbeRequests":            2,
				"GetRequests":              2,
				"GetErrors":                1,
				"GnmiPathLeaves":           2,
				"GnmiSetPathDeny":          4,
				"GnmiSetPathPermit":        0,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

			// Reload router
			perf.ReloadRouter(t, dut)

			// Perform Probe request after router reload
			gnsiC, err = dut.RawAPIs().BindingDUT().DialGNSI(context.Background())
			if err != nil {
				t.Fatalf("Could not connect gnsi %v", err)
			}

			// Perform Probe request for sandbox policy instance after router reload
			t.Logf("Probe Request : %v", probeReq_sand)
			got, err = gnsiC.Pathz().Probe(context.Background(), probeReq_sand)
			t.Logf("Probe Response : %v", got)
			t.Logf("Probe Error : %v", err)

			// Check for differences between expected and actual responses after router reload
			if d := cmp.Diff(want_sandbox, got, protocmp.Transform()); d == "" {
				t.Fatalf("Probe() unexpected diff: %s", d)
			}

			// Perform Probe request for active policy instance after router reload
			t.Logf("Probe Request : %v", probeReq_acvt)
			got, err = gnsiC.Pathz().Probe(context.Background(), probeReq_acvt)
			t.Logf("Probe Response : %v", got)

			if err != nil {
				t.Fatalf("Probe() unexpected error: %v", err)
			}

			// Check for differences between expected and actual responses after router reload
			if d := cmp.Diff(want_acvt, got, protocmp.Transform()); d != "" {
				t.Fatalf("Probe() unexpected diff: %s", d)
			}

			// Perform GET operations for sandbox policy instance after router reload
			getReq_Sand = &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ = gnsiC.Pathz().Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("RouterReload:Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform GET operations for active policy instance after router reload
			getReq_Actv = &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err = gnsiC.Pathz().Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("RouterReload: Pathz Get request is failed on device %s", dut.Name())
			}

			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("RouterReload: Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform gNMI SET operations after router reload
			validategNMISETOperations(t, dut, true, val)

			// Perform GET operation and validate the result after router reload
			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Perform gNMI SET operations with for undefined pathz policy xpath after router reload
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info after router reload
			pathz.VerifyPolicyInfo(t, dut, createdtime, "10", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Verify the pathz policy statistics after router reload.
			expectedStats = map[string]int{
				"RotationsInProgressCount": 0,
				"PolicyRotations":          0,
				"PolicyUploadRequests":     0,
				"PolicyFinalize":           0,
				"ProbeRequests":            2,
				"GetRequests":              2,
				"GetErrors":                1,
				"GnmiPathLeaves":           2,
				"GnmiSetPathDeny":          4,
				"GnmiSetPathPermit":        0,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)
		}
	})
	t.Run("IMBootZ Pathz: Test Corrupt Pathz Policy File", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {

			// Get default hostname from the device
			state := gnmi.OC().System().Hostname()
			val := gnmi.Get(t, dut, state.State())
			t.Logf("gNMI Get Response: %v", val)

			gnsiC, err := dut.RawAPIs().BindingDUT().DialGNSI(context.Background())
			if err != nil {
				t.Fatalf("Could not connect gnsi %v", err)
			}

			pathzRulesPath := "testdata/invalid_policy.txt"
			copyPathzRules := "/mnt/rdsfs/ems/gnsi"

			// Perform Rotate request
			req_1 := &pathzpb.RotateRequest{
				RotateRequest: &pathzpb.RotateRequest_UploadRequest{
					UploadRequest: &pathzpb.UploadRequest{
						Version:   "1",
						CreatedOn: createdtime,
						Policy: &pathzpb.AuthorizationPolicy{
							Rules: []*pathzpb.AuthorizationRule{{
								Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
								Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
								Mode:      pathzpb.Mode_MODE_WRITE,
								Action:    pathzpb.Action_ACTION_DENY,
							}},
						},
					},
				},
			}

			rot, errmsg := gnsiC.Pathz().Rotate(context.Background())
			t.Logf("Request Sent: %s", req_1)
			t.Logf("Request Sent: %s", errmsg)
			if err := rot.Send(req_1); err != nil {
				t.Logf("Rec Err %v", err)
				t.Fatal(err)
			}

			time.Sleep(5 * time.Second)

			// Finalize
			req := &pathzpb.RotateRequest{
				RotateRequest: &pathzpb.RotateRequest_FinalizeRotation{},
			}

			t.Logf("Request Sent: %s", req)
			if err := rot.Send(req); err != nil {
				t.Logf("Rec Err %v", err)
				t.Fatal(err)
			}

			time.Sleep(5 * time.Second)

			// Perform Rotate request 2
			req_2 := &pathzpb.RotateRequest{
				RotateRequest: &pathzpb.RotateRequest_UploadRequest{
					UploadRequest: &pathzpb.UploadRequest{
						Version:   "2",
						CreatedOn: createdtime,
						Policy: &pathzpb.AuthorizationPolicy{
							Rules: []*pathzpb.AuthorizationRule{{
								Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "lldp"}, {Name: "config"}, {Name: "enabled"}}},
								Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
								Mode:      pathzpb.Mode_MODE_WRITE,
								Action:    pathzpb.Action_ACTION_PERMIT,
							}},
						},
					},
				},
			}

			rot, _ = gnsiC.Pathz().Rotate(context.Background())
			t.Logf("Request Sent: %s", req_2)
			if err := rot.Send(req_2); err != nil {
				t.Logf("Rec Err %v", err)
				t.Fatal(err)
			}

			time.Sleep(5 * time.Second)

			// Finalize
			req = &pathzpb.RotateRequest{
				RotateRequest: &pathzpb.RotateRequest_FinalizeRotation{},
			}

			t.Logf("Request Sent: %s", req)
			if err := rot.Send(req); err != nil {
				t.Logf("Rec Err %v", err)
				t.Fatal(err)
			}

			time.Sleep(5 * time.Second)

			get_res := &pathzpb.GetResponse{
				Version:   "2",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "lldp"}, {Name: "config"}, {Name: "enabled"}}},
						Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_PERMIT,
					}},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, err := gnsiC.Pathz().Get(context.Background(), getReq_Sand)
			t.Logf("Sandbox Response : %v", sand_res)
			t.Logf("Sandbox Error : %v", err)

			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := gnsiC.Pathz().Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz Active Get request is failed on device %s", dut.Name())
			}

			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform gNMI SET operations
			validategNMISETOperations(t, dut, true, val)

			// Perform GET operation and validate the result.
			portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Perform gNMI SET operations with pathz policy.
			path := gnmi.OC().Lldp().Enabled()
			gnmi.Update(t, dut, path.Config(), true)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "2", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 4, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/lldp/config/enabled", false, true, 0, 1)
			pathz.VerifyReadPolicyCounters(t, dut, "/lldp/config/enabled", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Verify the pathz policy statistics after router reload.
			expectedStats := map[string]int{
				"RotationsInProgressCount": 0,
				"PolicyRotations":          2,
				"PolicyUploadRequests":     2,
				"PolicyFinalize":           2,
				"ProbeRequests":            2,
				"GetRequests":              4,
				"GetErrors":                2,
				"GnmiPathLeaves":           3,
				"GnmiSetPathDeny":          7,
				"GnmiSetPathPermit":        1,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

			// Perform eMSD process restart
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Rotate Request 1 after process restart
			rot_1, _ := gnsiC.Pathz().Rotate(context.Background())
			t.Logf("Request Sent: %s", req_1)
			if err := rot_1.Send(req_1); err != nil {
				t.Logf("Rec Err %v", err)
				t.Fatal(err)
			}

			// Finalize Rotation after Process restart
			t.Logf("Request Sent: %s", req)
			if err := rot_1.Send(req); err != nil {
				t.Logf("Rec Err %v", err)
				t.Fatal(err)
			}

			time.Sleep(5 * time.Second)

			// Rotate Request 2 after process restart
			rot_2, _ := gnsiC.Pathz().Rotate(context.Background())
			t.Logf("Request Sent: %s", req_2)
			if err := rot_2.Send(req_2); err != nil {
				t.Logf("Rec Err %v", err)
				t.Fatal(err)
			}

			// Finalize Rotation after Process restart
			t.Logf("Request Sent: %s", req)
			if err := rot_2.Send(req); err != nil {
				t.Logf("Rec Err %v", err)
				t.Fatal(err)
			}

			time.Sleep(5 * time.Second)

			// Perform GET operations for sandbox policy instance after process restart
			sand_res, err = gnsiC.Pathz().Get(context.Background(), getReq_Sand)
			t.Logf("Sandbox Response : %v", sand_res)
			t.Logf("Sandbox Error : %v", err)

			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform GET operations for active policy instance after process restart
			actv_res, err = gnsiC.Pathz().Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz Active Get request is failed on device %s", dut.Name())
			}

			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform gNMI SET operations after process restart
			validategNMISETOperations(t, dut, true, val)

			// Perform GET operation and validate the result after process restart
			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Perform gNMI SET operations with for pathz policy after process restart.
			gnmi.Update(t, dut, path.Config(), true)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "2", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/lldp/config/enabled", false, true, 0, 1)
			pathz.VerifyReadPolicyCounters(t, dut, "/lldp/config/enabled", false, false, 0, 0)

			// Verify the pathz policy statistics after router reload.
			expectedStats = map[string]int{
				"RotationsInProgressCount": 0,
				"PolicyRotations":          2,
				"PolicyUploadRequests":     2,
				"PolicyFinalize":           2,
				"ProbeRequests":            0,
				"GetRequests":              2,
				"GetErrors":                1,
				"GnmiPathLeaves":           2,
				"GnmiSetPathDeny":          3,
				"GnmiSetPathPermit":        1,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

			// Reload router
			perf.ReloadRouter(t, dut)

			gnsiC, err = dut.RawAPIs().BindingDUT().DialGNSI(context.Background())
			if err != nil {
				t.Fatalf("Could not connect gnsi %v", err)
			}

			// Rotate request 1 after router reload
			rot, _ = gnsiC.Pathz().Rotate(context.Background())
			t.Logf("Request Sent: %s", req_1)
			if err := rot.Send(req_1); err != nil {
				t.Logf("Rec Err %v", err)
				t.Fatal(err)
			}

			// Finalize Rotation after router reload
			t.Logf("Request Sent: %s", req)
			if err := rot.Send(req); err != nil {
				t.Logf("Rec Err %v", err)
				t.Fatal(err)
			}

			time.Sleep(5 * time.Second)

			// Rotate request 2 after router reload
			rot, _ = gnsiC.Pathz().Rotate(context.Background())
			t.Logf("Request Sent: %s", req_2)
			if err := rot.Send(req_2); err != nil {
				t.Logf("Rec Err %v", err)
				t.Fatal(err)
			}

			// Finalize Rotation after router reload
			t.Logf("Request Sent: %s", req)
			if err := rot.Send(req); err != nil {
				t.Logf("Rec Err %v", err)
				t.Fatal(err)
			}

			time.Sleep(5 * time.Second)

			// Perform GET operations for sandbox policy instance after router reload
			sand_res, err = gnsiC.Pathz().Get(context.Background(), getReq_Sand)
			t.Logf("Sandbox Response : %v", sand_res)
			t.Logf("Sandbox Error : %v", err)

			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform GET operations for active policy instance after router reload
			actv_res, err = gnsiC.Pathz().Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz Active Get request is failed on device %s", dut.Name())
			}

			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform gNMI SET operations after router reload
			validategNMISETOperations(t, dut, true, val)

			// Perform GET operation and validate the result after router reload
			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Perform gNMI SET operations with for undefined pathz policy xpath after router reload.
			gnmi.Update(t, dut, path.Config(), true)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "2", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/lldp/config/enabled", false, true, 0, 1)
			pathz.VerifyReadPolicyCounters(t, dut, "/lldp/config/enabled", false, false, 0, 0)

			// Verify the pathz policy statistics after router reload.
			expectedStats = map[string]int{
				"RotationsInProgressCount": 0,
				"PolicyRotations":          2,
				"PolicyUploadRequests":     2,
				"PolicyFinalize":           2,
				"ProbeRequests":            0,
				"GetRequests":              2,
				"GetErrors":                1,
				"GnmiPathLeaves":           2,
				"GnmiSetPathDeny":          3,
				"GnmiSetPathPermit":        1,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

			// SCP Client
			target := fmt.Sprintf("%s:%v", d.sshIp, d.sshPort)
			t.Logf("Copying Pathz rules file to %s (%s) over scp", d.dut, target)
			sshConf := scp.NewSSHConfigFromPassword(d.sshUser, d.sshPass)
			scpClient, err := scp.NewClient(target, sshConf, &scp.ClientOption{})
			if err != nil {
				t.Fatalf("Error initializing scp client: %v", err)
			}
			defer scpClient.Close()

			time.Sleep(10 * time.Second)

			// Copy invalid policy file to DUT
			resp := scpClient.CopyFileToRemote(pathzRulesPath, copyPathzRules, &scp.FileTransferOption{})
			t.Logf("copying file got %v", resp)
			if resp == nil || strings.Contains(resp.Error(), "Function not implemented") {
				t.Logf("SCP successful: File copied successfully")
			} else {
				t.Fatalf("SCP attempt failed: %s", resp.Error())
			}

			time.Sleep(10 * time.Second)

			// Move the invalid_policy.txt to pathz_policy.txt
			cliHandle := dut.RawAPIs().CLI(t)
			_, err = cliHandle.RunCommand(context.Background(), "run mv /mnt/rdsfs/ems/gnsi/invalid_policy.txt /mnt/rdsfs/ems/gnsi/pathz_policy.txt")
			time.Sleep(30 * time.Second)
			if err != nil {
				t.Error(err)
			}

			// guarantee a few timestamps before emsd restart occurs
			time.Sleep(10 * time.Second)

			// Perform GET operations for sandbox policy instance
			sand_res, err = gnsiC.Pathz().Get(context.Background(), getReq_Sand)
			t.Logf("Sandbox Response : %v", sand_res)
			t.Logf("Sandbox Error : %v", err)

			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform GET operations for active policy instance
			actv_res, err = gnsiC.Pathz().Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz Active Get request is failed on device %s", dut.Name())
			}

			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Invalid Pathz Rule: Perform gNMI SET operations
			validategNMISETOperations(t, dut, true, val)

			// Invalid Pathz Rule: Perform GET operation and validate the result.
			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Invalid Pathz Rule: Perform gNMI SET operations with for undefined pathz policy xpath.
			gnmi.Update(t, dut, path.Config(), true)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "2", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 6, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/lldp/config/enabled", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/lldp/config/enabled", false, false, 0, 0)

			// Verify the pathz policy statistics after router reload.
			expectedStats = map[string]int{
				"RotationsInProgressCount": 0,
				"PolicyRotations":          2,
				"PolicyUploadRequests":     2,
				"PolicyFinalize":           2,
				"ProbeRequests":            0,
				"GetRequests":              4,
				"GetErrors":                2,
				"GnmiPathLeaves":           2,
				"GnmiSetPathDeny":          6,
				"GnmiSetPathPermit":        2,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

			// Perform eMSD process restart
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			get_res = &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_DENY,
					}},
				},
			}

			// Invalid Pathz Rule: Perform GET operations for sandbox policy instance after process restart
			sand_res, err = gnsiC.Pathz().Get(context.Background(), getReq_Sand)
			t.Logf("Sandbox Response : %v", sand_res)
			t.Logf("Sandbox Error : %v", err)

			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Invalid Pathz Rule: Perform GET operations for active policy instance after process restart
			actv_res, err = gnsiC.Pathz().Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after corrupting pathz file: %s", d)
			}

			// Invalid Pathz Rule: Perform gNMI SET operations
			validategNMISETOperations(t, dut, true, val)

			// Invalid Pathz Rule: Perform GET operation and validate the result after process restart
			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Invalid Pathz Rule: Perform gNMI SET operations with lldp pathz policy xpath after process restart.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Verify the pathz policy statistics after router reload.
			expectedStats = map[string]int{
				"RotationsInProgressCount": 0,
				"PolicyRotations":          0,
				"PolicyUploadRequests":     0,
				"PolicyFinalize":           0,
				"ProbeRequests":            0,
				"GetRequests":              2,
				"GetErrors":                1,
				"GnmiPathLeaves":           2,
				"GnmiSetPathDeny":          4,
				"GnmiSetPathPermit":        0,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.txt")

			// Invalid Pathz Rule: Perform GET operations after deleting pathz file
			sand_res, err = gnsiC.Pathz().Get(context.Background(), getReq_Sand)
			t.Logf("Sandbox Response : %v", sand_res)
			t.Logf("Sandbox Error : %v", err)

			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Invalid Pathz Rule: Perform GET operations after deleting pathz file
			actv_res, err = gnsiC.Pathz().Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after corrupting pathz file: %s", d)
			}

			// Invalid Pathz Rule: Perform gNMI SET operations after deleting pathz file
			validategNMISETOperations(t, dut, true, val)

			// Invalid Pathz Rule: Perform GET operation after deleting pathz file
			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Invalid Pathz Rule: Perform gNMI SET operations with lldp pathz policy xpath after deleting pathz file.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 2, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 6, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Verify the pathz policy statistics after router reload.
			expectedStats = map[string]int{
				"RotationsInProgressCount": 0,
				"PolicyRotations":          0,
				"PolicyUploadRequests":     0,
				"PolicyFinalize":           0,
				"ProbeRequests":            0,
				"GetRequests":              4,
				"GetErrors":                2,
				"GnmiPathLeaves":           2,
				"GnmiSetPathDeny":          8,
				"GnmiSetPathPermit":        0,
				"PolicyUnmarshallErrors":   1,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

			// Reload router
			perf.ReloadRouter(t, dut)

			gnsiC, err = dut.RawAPIs().BindingDUT().DialGNSI(context.Background())
			if err != nil {
				t.Fatalf("Could not connect gnsi %v", err)
			}

			// Invalid Pathz Rule: Perform GET operations after router reload
			sand_res, err = gnsiC.Pathz().Get(context.Background(), getReq_Sand)
			t.Logf("Sandbox Response : %v", sand_res)
			t.Logf("Sandbox Error : %v", err)

			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Invalid Pathz Rule: Perform GET operations for active policy instance after router reload
			actv_res, _ = gnsiC.Pathz().Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after deleting backup pathz file: %s", d)
			}

			// Perform gNMI SET operations after router reload
			validategNMISETOperations(t, dut, true, val)

			// Perform GET operation and validate the result after router reload
			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Invalid Pathz Rule: Perform gNMI SET operations with lldp pathz policy xpath after router reload.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			timestamp := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").GnmiPathzPolicyCreatedOn().State())
			t.Logf("Got the expected Policy timestamp: %v", timestamp)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, timestamp, "Cisco-Deny-All-Bad-File-Encoding", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 4, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

			// Verify the pathz policy statistics after router reload.
			expectedStats = map[string]int{
				"RotationsInProgressCount": 0,
				"PolicyRotations":          0,
				"PolicyUploadRequests":     0,
				"PolicyFinalize":           0,
				"ProbeRequests":            0,
				"GetRequests":              2,
				"GetErrors":                1,
				"GnmiPathLeaves":           1,
				"GnmiSetPathDeny":          4,
				"GnmiSetPathPermit":        0,
				"PolicyUnmarshallErrors":   1,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.bak")

			// Invalid Pathz Rule: Perform GET operations after router reload
			sand_res, err = gnsiC.Pathz().Get(context.Background(), getReq_Sand)
			t.Logf("Sandbox Response : %v", sand_res)
			t.Logf("Sandbox Error : %v", err)

			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Invalid Pathz Rule: Perform GET operations for active policy instance after router reload
			actv_res, _ = gnsiC.Pathz().Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after deleting backup pathz file: %s", d)
			}

			// Perform gNMI SET operations after router reload
			validategNMISETOperations(t, dut, true, val)

			// Perform GET operation and validate the result after router reload
			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Invalid Pathz Rule: Perform gNMI SET operations with lldp pathz policy xpath after router reload.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			timestamp = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").GnmiPathzPolicyCreatedOn().State())
			t.Logf("Got the expected Policy timestamp: %v", timestamp)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, timestamp, "Cisco-Deny-All-Bad-File-Encoding", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 8, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

			// Verify the pathz policy statistics after router reload.
			expectedStats = map[string]int{
				"RotationsInProgressCount": 0,
				"PolicyRotations":          0,
				"PolicyUploadRequests":     0,
				"PolicyFinalize":           0,
				"ProbeRequests":            0,
				"GetRequests":              4,
				"GetErrors":                2,
				"GnmiPathLeaves":           1,
				"GnmiSetPathDeny":          8,
				"GnmiSetPathPermit":        0,
				"PolicyUnmarshallErrors":   1,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

			// Perform eMSD process restart
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Invalid Pathz Rule: Perform GET operations after deleting pathz file
			sand_res, err = gnsiC.Pathz().Get(context.Background(), getReq_Sand)
			t.Logf("Sandbox Response : %v", sand_res)
			t.Logf("Sandbox Error : %v", err)

			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform GET operations for active policy instance after deleting backup pathz policy.
			actv_res, _ = gnsiC.Pathz().Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			t.Logf("Active Error : %v", err)
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after deleting backup pathz file: %s", d)
			}

			// Perform gNMI SET operations after router reload
			validategNMISETOperations(t, dut, false, val)

			// Perform GET operation and validate the result after router reload
			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Perform gNMI SET operations with for llpd pathz policy xpath after router reload.
			gnmi.Update(t, dut, path.Config(), true)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)

			// Verify the pathz policy statistics after router reload.
			expectedStats = map[string]int{
				"RotationsInProgressCount": 0,
				"PolicyRotations":          0,
				"PolicyUploadRequests":     0,
				"PolicyFinalize":           0,
				"ProbeRequests":            0,
				"GetRequests":              2,
				"GetErrors":                2,
				"GnmiPathLeaves":           0,
				"GnmiSetPathDeny":          0,
				"GnmiSetPathPermit":        0,
				"PolicyUnmarshallErrors":   0,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)
		}
	})

	t.Run("Authz-1, Test Default Behaviour", func(t *testing.T) {
		gnsiC, err := dut.RawAPIs().BindingDUT().DialGNSI(context.Background())
		if err != nil {
			t.Fatalf("Could not connect gnsi %v", err)
		}

		expectedRes := authzpb.ProbeResponse_ACTION_PERMIT

		// Authz get request
		_, IMpolicy := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s is %s", dut.Name(), IMpolicy.PrettyPrint(t))

		// Authz probe request
		rpc := gnxi.RPCs.AllRPC
		resp, err := gnsiC.Authz().Probe(context.Background(), &authzpb.ProbeRequest{User: *testInfraID, Rpc: rpc.Path})
		t.Logf("Probe Response : %v", resp)
		if err != nil {
			t.Fatalf("Prob Request %s failed on dut %s", prettyPrint(&authzpb.ProbeRequest{User: *testInfraID, Rpc: rpc.Path}), dut.Name())
		}

		if resp.GetAction() != expectedRes {
			t.Fatalf("Prob response is not expected for user %s and path %s on dut %s, want %v, got %v", *testInfraID, rpc.Path, dut.Name(), expectedRes, resp.GetAction())
		}

		// Perform eMSD process restart
		t.Logf("Restarting emsd at %s", time.Now())
		perf.RestartProcess(t, dut, "emsd")
		t.Logf("Restart emsd finished at %s", time.Now())

		// Authz get request after process restart
		_, IMpolicy = authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s is %s", dut.Name(), IMpolicy.PrettyPrint(t))

		// Authz probe request after process restart
		resp, err = gnsiC.Authz().Probe(context.Background(), &authzpb.ProbeRequest{User: *testInfraID, Rpc: rpc.Path})
		t.Logf("Probe Response : %v", resp)
		t.Logf("Probe Error : %v", err)
		if err != nil {
			t.Fatalf("Prob Request %s failed on dut %s", prettyPrint(&authzpb.ProbeRequest{User: *testInfraID, Rpc: rpc.Path}), dut.Name())
		}

		if resp.GetAction() != expectedRes {
			t.Fatalf("Prob response is not expected for user %s and path %s on dut %s, want %v, got %v", *testInfraID, rpc.Path, dut.Name(), expectedRes, resp.GetAction())
		}

		// Reload router
		perf.ReloadRouter(t, dut)

		gnsiC, err = dut.RawAPIs().BindingDUT().DialGNSI(context.Background())
		if err != nil {
			t.Fatalf("Could not connect gnsi %v", err)
		}

		// Authz get request after router reload
		_, IMpolicy = authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s is %s", dut.Name(), IMpolicy.PrettyPrint(t))

		// Authz probe request after router reload
		resp, err = gnsiC.Authz().Probe(context.Background(), &authzpb.ProbeRequest{User: *testInfraID, Rpc: rpc.Path})
		t.Logf("Probe Response : %v", resp)
		if err != nil {
			t.Fatalf("Prob Request %s failed on dut %s", prettyPrint(&authzpb.ProbeRequest{User: *testInfraID, Rpc: rpc.Path}), dut.Name())
		}

		if resp.GetAction() != expectedRes {
			t.Fatalf("Prob response is not expected for user %s and path %s on dut %s, want %v, got %v", *testInfraID, rpc.Path, dut.Name(), expectedRes, resp.GetAction())
		}
	})
	t.Run("Authz-2, Test only one rotation request at a time", func(t *testing.T) {
		expectedRes := authzpb.ProbeResponse_ACTION_DENY
		policyMap := authz.LoadPolicyFromJSONFile(t, "testdata/policy.json")

		gnsiC, err := dut.RawAPIs().BindingDUT().DialGNSI(context.Background())
		if err != nil {
			t.Fatalf("Could not connect gnsi %v", err)
		}

		time.Sleep(10 * time.Second)

		// Authz get request
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint(t))
		// defer policyBefore.Rotate(t, dut, uint64(time.Now().Unix()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

		// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
		newpolicy, ok := policyMap["policy-everyone-can-gnmi-not-gribi"]
		if !ok {
			t.Fatal("Policy policy-everyone-can-gnmi-not-gribi is not loaded from policy json file")
		}
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.AllRPC})
		jsonPolicy, err := newpolicy.Marshal()
		if err != nil {
			t.Fatalf("Could not marshal the policy %s", string(jsonPolicy))
		}

		// Authz Rotate Request 1
		rotateStream, err := gnsiC.Authz().Rotate(context.Background())
		if err != nil {
			t.Fatalf("Could not start a rotate stream %v", err)
		}

		time.Sleep(5 * time.Second)

		// defer rotateStream.CloseSend()
		autzRotateReq_1 := &authzpb.RotateAuthzRequest_UploadRequest{
			UploadRequest: &authzpb.UploadRequest{
				Version:   fmt.Sprintf("v0.%v", (time.Now().UnixNano())),
				CreatedOn: uint64(time.Now().Unix()),
				Policy:    string(jsonPolicy),
			},
		}

		t.Logf("Sending Authz.Rotate request on device (client 1): \n %v", autzRotateReq_1)
		err = rotateStream.Send(&authzpb.RotateAuthzRequest{RotateRequest: autzRotateReq_1})
		t.Log("Sent Rotate Error : ", err)
		if err == nil {
			t.Logf("Authz.Rotate upload (client 1) was successful, receiving response ...")
		}

		time.Sleep(5 * time.Second)
		_, err = rotateStream.Recv()
		t.Log("Received Rotate Error : ", err)
		if err != nil {
			t.Fatalf("Error while receiving rotate request reply (client 1) %v", err)
		}

		// Close the Stream
		// rotateStream.CloseSend()

		// Authz Rotate Request 2 - Before Finalizing the Request 1
		newpolicy, ok = policyMap["policy-everyone-can-gribi-not-gnmi"]
		if !ok {
			t.Fatal("Policy policy-everyone-can-gribi-not-gnmi is not loaded from policy json file")
		}
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.AllRPC})
		jsonPolicy, err = newpolicy.Marshal()
		if err != nil {
			t.Fatalf("Could not marshal the policy %s", string(jsonPolicy))
		}

		rotateStream, err = gnsiC.Authz().Rotate(context.Background())
		if err != nil {
			t.Fatalf("Could not start a rotate stream %v", err)
		}

		autzRotateReq_2 := &authzpb.RotateAuthzRequest_UploadRequest{
			UploadRequest: &authzpb.UploadRequest{
				Version:   fmt.Sprintf("v0.%v", (time.Now().UnixNano())),
				CreatedOn: uint64(time.Now().Unix()),
				Policy:    string(jsonPolicy),
			},
		}

		t.Logf("Sending Authz.Rotate request on device (client 2): \n %v", autzRotateReq_2)
		err = rotateStream.Send(&authzpb.RotateAuthzRequest{RotateRequest: autzRotateReq_2})
		t.Logf("Sent Rotate Error : %v", err)
		if err == nil {
			t.Logf("Authz.Rotate upload was successful (client 2), receiving response ...")
		}
		_, err = rotateStream.Recv()
		t.Logf("Received Rotate Error : %v", err)
		if err == nil {
			t.Fatalf("Second Rotate request (client 2) should be Rejected - Error while receiving rotate request reply %v", err)
		}

		time.Sleep(5 * time.Second)

		// Authz probe request
		rpc := gnxi.RPCs.GribiGet
		resp, err := gnsiC.Authz().Probe(context.Background(), &authzpb.ProbeRequest{User: *testInfraID, Rpc: rpc.Path})
		t.Logf("Probe Response : %v", resp)
		t.Logf("Probe Error : %v", err)
		if err != nil {
			t.Fatalf("Prob Request %s failed on dut %s", prettyPrint(&authzpb.ProbeRequest{User: *testInfraID, Rpc: rpc.Path}), dut.Name())
		}

		if resp.GetAction() != expectedRes {
			t.Fatalf("Prob response is not expected for user %s and path %s on dut %s, want %v, got %v", *testInfraID, rpc.Path, dut.Name(), expectedRes, resp.GetAction())
		}

		// Perform eMSD process restart
		t.Logf("Restarting emsd at %s", time.Now())
		perf.RestartProcess(t, dut, "emsd")
		t.Logf("Restart emsd finished at %s", time.Now())

		rotateStream, err = gnsiC.Authz().Rotate(context.Background())
		if err != nil {
			t.Fatalf("Could not start a rotate stream %v", err)
		}

		// Authz rotate request after process restart
		t.Logf("Sending Authz.Rotate request on device (client 1): \n %v", autzRotateReq_1)
		err = rotateStream.Send(&authzpb.RotateAuthzRequest{RotateRequest: autzRotateReq_1})
		t.Log("Sent Rotate Error : ", err)
		if err == nil {
			t.Logf("Authz.Rotate upload (client 1) was successful, receiving response ...")
		}
		_, err = rotateStream.Recv()
		t.Log("Received Rotate Error : ", err)
		if err != nil {
			t.Fatalf("Error while receiving rotate request reply (client 1) %v", err)
		}

		// Authz Rotate Request 2 - Before Finalizing the Request 1 after process restart
		rotateStream, err = gnsiC.Authz().Rotate(context.Background())
		if err != nil {
			t.Fatalf("Could not start a rotate stream %v", err)
		}

		t.Logf("Sending Authz.Rotate request on device (client 2): \n %v", autzRotateReq_2)
		err = rotateStream.Send(&authzpb.RotateAuthzRequest{RotateRequest: autzRotateReq_2})
		t.Logf("Sent Rotate Error : %v", err)
		if err == nil {
			t.Logf("Authz.Rotate upload was successful (client 2), receiving response ...")
		}
		_, err = rotateStream.Recv()
		t.Logf("Received Rotate Error : %v", err)
		if err == nil {
			t.Fatalf("Second Rotate request (client 2) should be Rejected - Error while receiving rotate request reply %v", err)
		}

		// Authz probe request after process restart
		resp, err = gnsiC.Authz().Probe(context.Background(), &authzpb.ProbeRequest{User: *testInfraID, Rpc: rpc.Path})
		t.Logf("Probe Response : %v", resp)
		if err != nil {
			t.Fatalf("Prob Request %s failed on dut %s", prettyPrint(&authzpb.ProbeRequest{User: *testInfraID, Rpc: rpc.Path}), dut.Name())
		}

		if resp.GetAction() != expectedRes {
			t.Fatalf("Prob response is not expected for user %s and path %s on dut %s, want %v, got %v", *testInfraID, rpc.Path, dut.Name(), expectedRes, resp.GetAction())
		}

		// Reload router
		perf.ReloadRouter(t, dut)

		gnsiC, err = dut.RawAPIs().BindingDUT().DialGNSI(context.Background())
		if err != nil {
			t.Fatalf("Could not connect gnsi %v", err)
		}

		expectedRes = authzpb.ProbeResponse_ACTION_PERMIT

		// Authz probe request after router reload
		resp, err = gnsiC.Authz().Probe(context.Background(), &authzpb.ProbeRequest{User: *testInfraID, Rpc: rpc.Path})
		t.Logf("Probe Response : %v", resp)
		if err != nil {
			t.Fatalf("Prob Request %s failed on dut %s", prettyPrint(&authzpb.ProbeRequest{User: *testInfraID, Rpc: rpc.Path}), dut.Name())
		}

		if resp.GetAction() != expectedRes {
			t.Fatalf("Prob response is not expected for user %s and path %s on dut %s, want %v, got %v", *testInfraID, rpc.Path, dut.Name(), expectedRes, resp.GetAction())
		}
	})
	t.Run("Authz-3, Test Authz Rules Behavior", func(t *testing.T) {

		policyMap := authz.LoadPolicyFromJSONFile(t, "testdata/policy.json")

		// Get default hostname from the device
		state := gnmi.OC().System().Hostname()
		val := gnmi.Get(t, dut, state.State())
		t.Logf("gNMI Get Response: %v", val)

		gnsiC, err := dut.RawAPIs().BindingDUT().DialGNSI(context.Background())
		if err != nil {
			t.Fatalf("Could not connect gnsi %v", err)
		}

		// Authz get request
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint(t))

		// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
		newpolicy, ok := policyMap["policy-everyone-can-gnmi-not-gnoi-time"]
		if !ok {
			t.Fatal("policy-everyone-can-gnmi-not-gnoi-time is not loaded from policy json file")
		}
		// newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.AllRPC})
		jsonPolicy, err := newpolicy.Marshal()
		if err != nil {
			t.Fatalf("Could not marshal the policy %s", string(jsonPolicy))
		}
		// Authz Rotate Request 1
		rotateStream, err := gnsiC.Authz().Rotate(context.Background())
		if err != nil {
			t.Fatalf("Could not start rotate stream %v", err)
		}

		time.Sleep(5 * time.Second)

		// defer rotateStream.CloseSend()
		autzRotateReq_1 := &authzpb.RotateAuthzRequest_UploadRequest{
			UploadRequest: &authzpb.UploadRequest{
				Version:   fmt.Sprintf("v0.%v", (time.Now().UnixNano())),
				CreatedOn: uint64(time.Now().Unix()),
				Policy:    string(jsonPolicy),
			},
		}
		t.Logf("Sending Authz.Rotate request on device (client 1): \n %v", autzRotateReq_1)
		err = rotateStream.Send(&authzpb.RotateAuthzRequest{RotateRequest: autzRotateReq_1})
		if err == nil {
			t.Logf("Authz.Rotate upload (client 1) was successful, receiving response ...")
		}

		_, err = rotateStream.Recv()
		if err != nil {
			t.Fatalf("Error while receiving rotate request reply (client 1) %v", err)
		}

		finalizeRotateReq := &authzpb.RotateAuthzRequest_FinalizeRotation{FinalizeRotation: &authzpb.FinalizeRequest{}}
		t.Logf("Sending Authz.Rotate FinalizeRotation request: \n%v", finalizeRotateReq)
		err = rotateStream.Send(&authzpb.RotateAuthzRequest{RotateRequest: finalizeRotateReq})
		if err != nil {
			t.Fatalf("Error while finalizing rotate request  %v", err)
		}
		t.Logf("Authz.Rotate FinalizeRotation is successful (First Client)")

		// Perform gNMI SET operations
		// isPermissionDeniedError(t, dut, "gNMI-SET-Denied")
		validategNMISETOperations(t, dut, true, val)

		// Perform GET operation
		portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
		if portNum == uint16(0) || portNum > uint16(0) {
			t.Logf("Got the expected port number")
		} else {
			t.Fatalf("Unexpected value for port number: %v", portNum)
		}

		// Validate gNOI operations
		validategNOIOperations(t, dut, true)

		// Perform eMSD process restart
		t.Logf("Restarting emsd at %s", time.Now())
		perf.RestartProcess(t, dut, "emsd")
		t.Logf("Restart emsd finished at %s", time.Now())

		// Perform gNMI SET operations after router reload
		validategNMISETOperations(t, dut, true, val)

		// Perform GET operation and validate the result after router reload
		if portNum == uint16(0) || portNum > uint16(0) {
			t.Logf("Got the expected port number")
		} else {
			t.Fatalf("Unexpected value for port number: %v", portNum)
		}

		// Validate gNOI operations
		validategNOIOperations(t, dut, true)

		// Authz Rotate Request 2
		newpolicy, ok = policyMap["policy-everyone-can-gnoi-time-not-gnmi"]
		if !ok {
			t.Fatal("policy-everyone-can-gnoi-time-not-gnmi is not loaded from policy json file")
		}
		// newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.AllRPC})
		jsonPolicy, err = newpolicy.Marshal()
		if err != nil {
			t.Fatalf("Could not marshal the policy %s", string(jsonPolicy))
		}

		rotateStream, err = gnsiC.Authz().Rotate(context.Background())
		if err != nil {
			t.Fatalf("Could not start rotate stream %v", err)
		}

		// defer rotateStream.CloseSend()
		autzRotateReq_2 := &authzpb.RotateAuthzRequest_UploadRequest{
			UploadRequest: &authzpb.UploadRequest{
				Version:   fmt.Sprintf("v0.%v", (time.Now().UnixNano())),
				CreatedOn: uint64(time.Now().Unix()),
				Policy:    string(jsonPolicy),
			},
		}

		t.Logf("Sending Authz.Rotate request on device (client 2): \n %v", autzRotateReq_2)
		err = rotateStream.Send(&authzpb.RotateAuthzRequest{RotateRequest: autzRotateReq_2})
		if err == nil {
			t.Logf("Authz.Rotate upload was successful (client 2), receiving response ...")
		}

		time.Sleep(5 * time.Second)

		_, err = rotateStream.Recv()
		t.Logf("Received Rotate Error : %v", err)
		if err != nil {
			t.Fatalf("Second Rotate request Rejected - Error while receiving rotate request reply %v", err)
		}

		time.Sleep(5 * time.Second)

		finalizeRotateReq = &authzpb.RotateAuthzRequest_FinalizeRotation{FinalizeRotation: &authzpb.FinalizeRequest{}}
		t.Logf("Sending Authz.Rotate FinalizeRotation request: \n%v", finalizeRotateReq)

		err = rotateStream.Send(&authzpb.RotateAuthzRequest{RotateRequest: finalizeRotateReq})
		if err != nil {
			t.Fatalf("Error while finalizing rotate request  %v", err)
		}
		t.Logf("Authz.Rotate FinalizeRotation is successful (Second Client)")

		// Perform gNMI SET operations after router reload
		isPermissionDeniedError(t, dut, "gNMI-SET-Denied")
		// validategNMISETOperations(t, dut, true, val)

		// Perform GET operation and validate the result
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			t.Logf("Validate gNMI Get RPC")
			gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
		}); errMsg != nil {
			t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
		} else {
			t.Errorf("This gNMI GET should have failed ")
		}

		// Validate gNOI operations
		validategNOIOperations(t, dut, false)

		// Delete Pathz policy file and verify the behaviour
		pathz.DeletePolicyData(t, dut, "authz_policy.txt")

		// Reload router
		perf.ReloadRouter(t, dut)

		// Perform gNMI SET operations after router reload
		validategNMISETOperations(t, dut, true, val)

		// Perform GET operation and validate the result after router reload
		if portNum == uint16(0) || portNum > uint16(0) {
			t.Logf("Got the expected port number")
		} else {
			t.Fatalf("Unexpected value for port number: %v", portNum)
		}

		// Validate gNOI operations
		validategNOIOperations(t, dut, true)

		// Delete Pathz policy file and verify the behaviour
		pathz.DeletePolicyData(t, dut, "authz_policy.bak")

		// Perform eMSD process restart
		t.Logf("Restarting emsd at %s", time.Now())
		perf.RestartProcess(t, dut, "emsd")
		t.Logf("Restart emsd finished at %s", time.Now())

		// Perform gNMI SET operations after router reload
		validategNMISETOperations(t, dut, true, val)

		// Perform GET operation and validate the result after router reload
		if portNum == uint16(0) || portNum > uint16(0) {
			t.Logf("Got the expected port number")
		} else {
			t.Fatalf("Unexpected value for port number: %v", portNum)
		}

		// Validate gNOI operations
		validategNOIOperations(t, dut, true)

		// Delete Pathz policy file and verify the behaviour
		pathz.DeletePolicyData(t, dut, "authz_policy.txt")

		// Perform eMSD process restart
		t.Logf("Restarting emsd at %s", time.Now())
		perf.RestartProcess(t, dut, "emsd")
		t.Logf("Restart emsd finished at %s", time.Now())

		// Perform gNMI SET operations after router reload
		validategNMISETOperations(t, dut, false, val)

		// Perform GET operation and validate the result after router reload
		if portNum == uint16(0) || portNum > uint16(0) {
			t.Logf("Got the expected port number")
		} else {
			t.Fatalf("Unexpected value for port number: %v", portNum)
		}

		// Validate gNOI operations
		validategNOIOperations(t, dut, false)
	})
}

func dialConn(t *testing.T, dut *ondatra.DUTDevice, svc introspect.Service, wantPort uint32) *grpc.ClientConn {
	t.Helper()
	if svc == introspect.GNOI || svc == introspect.GNSI {
		// Renaming service name due to gnoi and gnsi always residing on same port as gnmi.
		svc = introspect.GNMI
	}
	dialer := introspect.DUTDialer(t, dut, introspect.GNMI)
	if dialer.DevicePort != int(wantPort) {
		t.Fatalf("DUT is not listening on correct port for %q: got %d, want %d", svc, dialer.DevicePort, wantPort)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := dialer.Dial(ctx)
	if err != nil {
		t.Fatalf("grpc.Dial failed to: %q", dialer.DialTarget)
	}
	return conn
}

func start(t *testing.T) bcpb.BootConfigClient {
	dut := ondatra.DUT(t, "dut")
	conn := dialConn(t, dut, introspect.GNOI, 9339)
	c := bcpb.NewBootConfigClient(conn)

	return c
}

// This will read bootconfig file used for testcases
func getBootConfigFromFile(t *testing.T, bcPath string) (bc *bpb.BootConfig) {
	// Read the boot_config file
	data, err := os.ReadFile(bcPath)
	if err != nil {
		t.Fatal(err)
	}

	// Unmarshal the boot_config file
	bc = &bpb.BootConfig{}
	err = prototext.Unmarshal(data, bc)
	if err != nil {
		t.Fatal(err)
	}

	return bc
}

// This will validate bootconfig vendor config
func validateVendorConfig(t *testing.T, dut *ondatra.DUTDevice, cfg string, enableNegativeTests bool) {
	t.Helper()

	// Derive the unconfig command dynamically
	uncfg := "no " + strings.Join(strings.Fields(cfg)[:len(strings.Fields(cfg))-1], " ")

	// Define gNMI path and value for operations
	gnmiPath := &gpb.Path{Origin: "cli"}
	deleteValue := &gpb.TypedValue{
		Value: &gpb.TypedValue_AsciiVal{
			AsciiVal: uncfg,
		},
	}
	setValue := &gpb.TypedValue{
		Value: &gpb.TypedValue_AsciiVal{
			AsciiVal: cfg,
		},
	}

	// Create a gNMI client
	gnmiC := dut.RawAPIs().GNMI(t)

	// Context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Perform Delete operation
	t.Logf("Sending Delete gNMI SetRequest: %v", uncfg)
	deleteReq := &gpb.SetRequest{
		Update: []*gpb.Update{{Path: gnmiPath, Val: deleteValue}},
	}
	if resp, err := gnmiC.Set(ctx, deleteReq); err != nil {
		t.Fatalf("gNMI Delete operation failed: %v", err)
	} else {
		t.Logf("Delete Response: %v", resp)
	}

	// Validate Delete operation
	command := "show running-config " + strings.Join(strings.Fields(cfg)[:len(strings.Fields(cfg))-1], " ")
	ActualConfig := CMDViaGNMI(ctx, t, dut, command)

	if enableNegativeTests {
		// Ensure configuration is still present (Negative Test)
		if !strings.Contains(ActualConfig, cfg) && !strings.Contains(ActualConfig, "ios") && !strings.Contains(ActualConfig, "No such configuration item(s)") {
			t.Logf("Validation Successful: Config not deleted: %s", ActualConfig)
		} else {
			t.Errorf("Validation Unsuccessful: Config deleted unexpectedly: got %q", ActualConfig)
		}
	} else {
		// Ensure configuration is deleted (Positive Test)
		if strings.Contains(ActualConfig, cfg) || strings.Contains(ActualConfig, "No such configuration item(s)") || strings.Contains(ActualConfig, "ios") {
			t.Logf("Validation Successful: Config deleted successfully: %s", ActualConfig)
		} else {
			t.Errorf("Validation Unsuccessful: Config not deleted: got %q", ActualConfig)
		}
	}

	// Perform Update operation
	t.Logf("Sending Update gNMI SetRequest: %v", cfg)
	updateReq := &gpb.SetRequest{
		Update: []*gpb.Update{{Path: gnmiPath, Val: setValue}},
	}
	if resp, err := gnmiC.Set(ctx, updateReq); err != nil {
		t.Fatalf("gNMI Update operation failed: %v", err)
	} else {
		t.Logf("Update Response: %v", resp)
	}

	// Validate Update operation
	ActualConfig = CMDViaGNMI(ctx, t, dut, command)

	if enableNegativeTests {
		// Ensure configuration is not applied
		if strings.Contains(ActualConfig, cfg) {
			t.Errorf("Validation Unsuccessful: Config applied unexpectedly: got %q", ActualConfig)
		} else {
			t.Logf("Validation Successful: Config not applied as expected: %s", ActualConfig)
		}
	} else {
		// Ensure configuration is applied
		if !strings.Contains(ActualConfig, cfg) {
			t.Errorf("Validation Unsuccessful: Config not applied: got %q, expected %q", ActualConfig, cfg)
		} else {
			t.Logf("Validation Successful: Config applied successfully: %s", ActualConfig)
		}
	}
}

// This will validate bootconfig non-vendor config
func validateNonBootConfig(t *testing.T, dut *ondatra.DUTDevice, isNegativeTestCase bool) {
	t.Helper()

	// Step 1: Configure Policy Forwarding
	t.Logf("Starting Policy Forwarding Configuration...")

	r1 := &oc.NetworkInstance_PolicyForwarding_Policy_Rule{
		SequenceId: ygot.Uint32(1),
		Ipv4:       &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{DscpSet: []uint8{16}},
		Action:     &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String("TE")},
	}

	p := &oc.NetworkInstance_PolicyForwarding_Policy{
		PolicyId: ygot.String(pbrName),
		Type:     oc.Policy_Type_VRF_SELECTION_POLICY,
		Rule:     map[uint32]*oc.NetworkInstance_PolicyForwarding_Policy_Rule{1: r1},
	}

	request := &oc.NetworkInstance{
		Name: ygot.String("DEFAULT"),
		PolicyForwarding: &oc.NetworkInstance_PolicyForwarding{
			Policy: map[string]*oc.NetworkInstance_PolicyForwarding_Policy{pbrName: p},
		},
	}

	t.Logf("Applying Policy Forwarding Configuration...")

	if isNegativeTestCase {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").Config(), request)
		}); errMsg != nil {
			// Log the expected failure with the captured error message
			t.Logf("Expected failure for Policy Forwarding, and got testt.CaptureFatal error: %s", *errMsg)
		} else {
			// If no error is captured, it should have failed, so log this as an error
			t.Errorf("Expected failure for Policy Forwarding but it succeeded")
		}
	} else {
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").Config(), request)
		// Validate Policy Forwarding State
		t.Logf("Validating Policy Forwarding State...")
		state := gnmi.OC().NetworkInstance(NetworkInstanceDefault).PolicyForwarding().Policy(pbrName)
		stateGot := gnmi.Get(t, dut, state.State())
		if stateGot.Type != oc.Policy_Type_VRF_SELECTION_POLICY {
			t.Errorf("Policy type mismatch: got %v, want %v", stateGot.Type, oc.Policy_Type_VRF_SELECTION_POLICY)
		} else {
			t.Logf("Policy type validated successfully: %v", stateGot.Type)
		}
	}

	// Step 2: Configure LLDP
	t.Logf("Configuring LLDP...")

	if isNegativeTestCase {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Update(t, dut, gnmi.OC().Lldp().HelloTimer().Config(), 5)
		}); errMsg != nil {
			// Log the expected failure with the captured error message
			t.Logf("Expected failure for LLDP, and got testt.CaptureFatal error: %s", *errMsg)
		} else {
			// If no error is captured, it should have failed, so log this as an error
			t.Errorf("Expected failure for LLDP but it succeeded")
		}
	} else {

		gnmi.Update(t, dut, gnmi.OC().Lldp().HelloTimer().Config(), 5)

		time.Sleep(5 * time.Second)

		// Validate LLDP State
		t.Logf("Validating LLDP State...")
		lldpTimer := gnmi.Get(t, dut, gnmi.OC().Lldp().HelloTimer().State())
		expectedTimer := uint64(5)

		if lldpTimer != expectedTimer {
			t.Errorf("Validation Unsuccessful: LLDP Hello Timer mismatch, got %v, want %v", lldpTimer, expectedTimer)
		} else {
			t.Logf("Validation Successful: LLDP Hello Timer is set to %v", lldpTimer)
		}
	}

	// Define the BGP configuration and unconfiguration commands
	const bgpCfg = "router bgp 5000"
	const bgpUncfg = "no router bgp 5000"

	// Define gNMI path and values for BGP operations
	gnmiPath := &gpb.Path{Origin: "cli"}
	bgpVal := &gpb.TypedValue{
		Value: &gpb.TypedValue_AsciiVal{
			AsciiVal: bgpCfg,
		},
	}
	bgpDeleteVal := &gpb.TypedValue{
		Value: &gpb.TypedValue_AsciiVal{
			AsciiVal: bgpUncfg,
		},
	}

	gnmiC := dut.RawAPIs().GNMI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Step 3: Configure BGP (Always Expects Success)
	t.Logf("Configuring BGP...")
	updateReq := &gpb.SetRequest{
		Update: []*gpb.Update{{Path: gnmiPath, Val: bgpVal}},
	}
	resp, err := gnmiC.Set(ctx, updateReq)
	if err != nil {
		t.Fatalf("gNMI Update operation for BGP failed: %v", err)
	}
	t.Logf("BGP Update Response: %v", resp)

	// Validate BGP Configuration
	t.Logf("Validating BGP Configuration...")
	bgpConfig := CMDViaGNMI(context.Background(), t, dut, "show running-config router bgp")
	if !strings.Contains(bgpConfig, bgpCfg) {
		t.Errorf("BGP Config mismatch: got %q, expected %q", bgpConfig, bgpCfg)
	} else {
		t.Logf("BGP Config validated successfully: %s", bgpConfig)
	}

	// Step 4: Cleanup - Delete Policy Forwarding, LLDP, and BGP Configurations
	t.Logf("Starting Cleanup Process...")

	// Deleting Policy Forwarding Configuration
	t.Logf("Deleting Policy Forwarding Configuration...")
	if isNegativeTestCase {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Config())
		}); errMsg != nil {
			// Log the expected failure with the captured error message
			t.Logf("Expected failure for Policy Forwarding Deletion, and got testt.CaptureFatal error: %s", *errMsg)
		} else {
			// If no error is captured, it should have failed, so log this as an error
			t.Errorf("Expected failure for Policy Forwarding Deletion but it succeeded")
		}
	} else {
		gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").PolicyForwarding().Policy(pbrName).Config())
	}

	// Deleting LLDP Configuration
	t.Logf("Disabling LLDP...")
	if isNegativeTestCase {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Delete(t, dut, gnmi.OC().Lldp().HelloTimer().Config())
		}); errMsg != nil {
			// Log the expected failure with the captured error message
			t.Logf("Expected failure for LLDP Deletion, and got testt.CaptureFatal error: %s", *errMsg)
		} else {
			// If no error is captured, it should have failed, so log this as an error
			t.Errorf("Expected failure for LLDP Deletion but it succeeded")
		}
	} else {
		expectedTimer := uint64(5)
		gnmi.Delete(t, dut, gnmi.OC().Lldp().HelloTimer().Config())

		// Validate LLDP Deletion
		lldpTimer := gnmi.Get(t, dut, gnmi.OC().Lldp().HelloTimer().State())
		if lldpTimer == expectedTimer {
			t.Errorf("Validation Unsuccessful: LLDP Hello Timer didnt delete, got %v, want %v", lldpTimer, expectedTimer)
		} else {
			t.Logf("Validation Successful: LLDP Hello Timer is set to %v", lldpTimer)
		}
	}

	// Remove BGP
	t.Logf("Removing BGP Configuration...")
	deleteReq := &gpb.SetRequest{
		Update: []*gpb.Update{{Path: gnmiPath, Val: bgpDeleteVal}},
	}
	resp, err = gnmiC.Set(ctx, deleteReq)
	if err != nil {
		t.Fatalf("gNMI Replace operation for BGP failed: %v", err)
	}
	t.Logf("BGP Replace Response: %v", resp)

	// Validate BGP Deletion
	bgpConfig = CMDViaGNMI(context.Background(), t, dut, "show running-config router bgp")
	if strings.Contains(bgpConfig, bgpCfg) {
		t.Errorf("BGP Config still exists after deletion: %q", bgpConfig)
	} else {
		t.Logf("BGP Config deletion validated successfully")
	}
}

// This will list boot_config file is available in misc/config/ems/gnsi location
func checkBootConfigFiles(t *testing.T, dut *ondatra.DUTDevice, No_file bool) {
	cliHandle := dut.RawAPIs().CLI(t)
	fileName := "boot_config_merged.txt"
	incorrectFileName := "boot_config.txt"

	// Step 1: List files in the directory
	listFilesCmd := `run ls -lrt /misc/config/ems/gnsi/`
	t.Logf("Executing command to list files: %q", listFilesCmd)
	listFilesOutput, err := cliHandle.RunCommand(context.Background(), listFilesCmd)
	if err != nil {
		t.Fatalf("Failed to list files in the directory. Command: %q, Error: %v", listFilesCmd, err)
	}
	t.Logf("File details in gnsi directory:\n%s", listFilesOutput.Output())

	// Step 3: Validate file presence or absence based on the scenario
	containsFile := strings.Contains(listFilesOutput.Output(), fileName)
	containsBackupFile := strings.Contains(listFilesOutput.Output(), incorrectFileName)

	if No_file {
		if containsFile || containsBackupFile {
			t.Errorf("Validation Unsuccessfull: Files %q or %q should not be present, but they are found in the gnsi directory", fileName, incorrectFileName)
		} else {
			t.Logf("Validation Successfull: Files %q and %q are not present in the gnsi directory", fileName, incorrectFileName)
		}
	} else {
		if containsFile && containsBackupFile {
			t.Errorf("Failed: Wrong %q is present in the gnsi directory, which is not allowed", incorrectFileName)
		} else if !containsFile && containsBackupFile {
			t.Errorf("Failed: Wrong file %q is present without the main file %q", incorrectFileName, fileName)
		} else {
			t.Logf("File %q is present , Validation successfull", fileName)
		}
	}
}

// This will validate bootconfig configurations.
func validateBootConfigConfiguration(t *testing.T, dut *ondatra.DUTDevice, hostname string, negative bool) {
	config := gnmi.OC().System().Hostname()
	state := gnmi.OC().System().Hostname()

	if negative {
		expectedValue := gnmi.Get(t, dut, state.State())
		t.Logf("gNMI Get Response (Expected Value): %v", expectedValue)

		// Step 1: Apply configuration using gNMI Update
		t.Logf("Applying configuration using gNMI Update %s:", hostname)
		gnmi.Update(t, dut, config.Config(), hostname)

		// Fetch and validate state after Update
		t.Logf("Fetching and validating state after gNMI Update")
		stateGot := gnmi.Get(t, dut, state.State())
		t.Logf("gNMI Subscribe Response: %v", stateGot)
		time.Sleep(5 * time.Second)

		if stateGot == expectedValue {
			t.Logf("Validation Successful: Hostname Unchanged after Update: %v", stateGot)
		} else {
			t.Errorf("Hostname changed incorrectly after Update: got %v, expected %s", stateGot, expectedValue)

		}

		// Step 2: Delete configuration using gNMI
		t.Logf("Deleting configuration using gNMI")
		gnmi.Delete(t, dut, config.Config())

		// Fetch and validate state after Delete
		t.Logf("Fetching and validating state after gNMI Delete")
		stateGot = gnmi.Get(t, dut, state.State())
		t.Logf("gNMI Subscribe Response: %v", stateGot)
		time.Sleep(5 * time.Second)

		if stateGot != "ios" {
			t.Logf("Validation Successfull: Hostname not deleted after gNMI Delete")
		} else {
			t.Errorf("Hostname deleted after gNMI Delete: got %v, expected %s", stateGot, expectedValue)
		}

		// Step 3: Replace configuration using gNMI
		t.Logf("Replacing configuration using gNMI Replace %s:", hostname)
		gnmi.Replace(t, dut, config.Config(), hostname)

		// Fetch and validate state after Replace
		t.Logf("Fetching and validating state after gNMI Replace")
		stateGot = gnmi.Get(t, dut, state.State())
		t.Logf("gNMI Subscribe Response: %v", stateGot)
		time.Sleep(5 * time.Second)

		if stateGot == expectedValue {
			t.Logf("Validation Successfull: Hostname Unchanged after Replace: %v", stateGot)
		} else {
			t.Errorf("Hostname changed incorrectly after Replace : got %v, expected %s", stateGot, expectedValue)
		}
	} else {
		// Step 1: Apply configuration using gNMI Update
		t.Logf("Applying configuration using gNMI Update")
		gnmi.Update(t, dut, config.Config(), hostname)

		// Fetch and validate state after Update
		t.Logf("Fetching and validating state after gNMI Update")
		stateGot := gnmi.Get(t, dut, state.State())
		t.Logf("gNMI Subscribe Response: %v", stateGot)
		time.Sleep(5 * time.Second)

		if stateGot == hostname {
			t.Logf("Got expected hostname after gNMI Update: %s", stateGot)
		} else {
			t.Errorf("Unexpected hostname after gNMI Update: got %v, expected %v", stateGot, hostname)
		}

		// Step 2: Delete configuration using gNMI
		t.Logf("Deleting configuration using gNMI")
		gnmi.Delete(t, dut, config.Config())

		// Fetch and validate state after Delete
		t.Logf("Fetching and validating state after gNMI Delete")
		stateGot = gnmi.Get(t, dut, state.State())
		t.Logf("gNMI Subscribe Response: %v", stateGot)
		time.Sleep(5 * time.Second)

		if stateGot == "ios" {
			t.Logf("Hostname successfully deleted after gNMI Delete operation")
		} else {
			t.Errorf("Unexpected hostname after gNMI Delete: got %v, expected empty state", stateGot)
		}

		// Step 3: Replace configuration using gNMI
		t.Logf("Replacing configuration using gNMI Replace")
		gnmi.Replace(t, dut, config.Config(), hostname)

		// Fetch and validate state after Replace
		t.Logf("Fetching and validating state after gNMI Replace")
		stateGot = gnmi.Get(t, dut, state.State())
		t.Logf("gNMI Subscribe Response: %v", stateGot)
		time.Sleep(5 * time.Second)

		if stateGot == hostname {
			t.Logf("Got expected hostname after gNMI Replace: %s", stateGot)
		} else {
			t.Errorf("Unexpected hostname after gNMI Replace: got %v, expected %v", stateGot, hostname)
		}
	}
}

// This will give redudancy state.
func redundancy_nsrState(ctx context.Context, t *testing.T, gribi_reconnect bool) {
	t.Helper()
	dut := ondatra.DUT(t, "dut")

	var responseRawObj NsrState
	nsrreq := &gpb.GetRequest{
		Path: []*gpb.Path{
			{
				Origin: "Cisco-IOS-XR-infra-rmf-oper", Elem: []*gpb.PathElem{
					{Name: "redundancy"},
					{Name: "summary"},
					{Name: "red-pair"},
				},
			},
		},
		Type:     gpb.GetRequest_STATE,
		Encoding: gpb.Encoding_JSON_IETF,
	}

	// Set timeout and polling interval
	timeout := 10 * time.Minute
	pollInterval := 60 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		nsrState, err := dut.RawAPIs().GNMI(t).Get(context.Background(), nsrreq)
		if err != nil {
			t.Logf("Error fetching NSR state: %v", err)
			time.Sleep(pollInterval)
			continue
		}

		t.Logf("Received NSR State: %v", nsrState)

		// Extract JSON data
		if len(nsrState.GetNotification()) == 0 || len(nsrState.GetNotification()[0].GetUpdate()) == 0 {
			t.Logf("No valid NSR state updates received, retrying...")
			time.Sleep(pollInterval)
			continue
		}

		jsonIetfData := nsrState.GetNotification()[0].GetUpdate()[0].GetVal().GetJsonIetfVal()
		if jsonIetfData == nil {
			t.Fatalf("Received nil JSON IETF data, possible model change")
		}

		err = json.Unmarshal(jsonIetfData, &responseRawObj)
		if err != nil {
			t.Fatalf("Failed to parse NSR state JSON: %v\nReceived JSON: %s", err, string(jsonIetfData))
		}

		t.Logf("Parsed NSR State: %v", responseRawObj.NSRState)

		// Check if NSR state is "Ready"
		if responseRawObj.NSRState == "Ready" {
			t.Logf("NSR state is now Ready")
			time.Sleep(20 * time.Second)
			return
		}

		// Wait before retrying
		time.Sleep(pollInterval)
	}

	t.Fatalf("Timed out waiting for NSR state to become 'Ready'")
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
		sshPort, _ := strconv.Atoi(strings.Split(dut.Ssh.Target, ":")[1])
		// sshPort := "22"
		// if len(sshTarget) > 1 {
		// 	sshPort = sshTarget[1]
		// }

		targets = append(targets, targetInfo{
			dut:     dut.Id,
			sshIp:   sshIp,
			sshPort: sshPort,
			sshUser: sshUser,
			sshPass: sshPass,
		})
		t.Logf("Probe Response : %v", targets)
	}
	return targets
}

func validategNOIOperations(t *testing.T, dut *ondatra.DUTDevice, enableNegativeTests bool) {

	gnoiC, err := dut.RawAPIs().BindingDUT().DialGNOI(context.Background())
	if err != nil {
		t.Fatalf("Could not connect gnoi %v", err)
	}

	if enableNegativeTests {
		// Invalid Pathz Rule: Perform gNMI SET operations : UPDATE after process restart
		got, err := gnoiC.System().Time(context.Background(), &spb.TimeRequest{})
		t.Logf("gNOI Response : %v", got)
		t.Logf("gNOI Error : %v", err)
		if err != nil {
			t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", err)
		} else {
			t.Fatalf("This gNOI Operation should have failed ")
		}
	} else {
		response, err := gnoiC.System().Time(context.Background(), &spb.TimeRequest{})
		t.Logf("Time Response : %v", response)
		if err != nil {
			t.Fatalf("Time Request failed on dut %s", dut.Name())
		}
	}
}

func validategNMISETOperations(t *testing.T, dut *ondatra.DUTDevice, enableNegativeTests bool, val string) {
	config := gnmi.OC().System().Hostname()
	state := gnmi.OC().System().Hostname()

	if enableNegativeTests {
		// Invalid Pathz Rule: Perform gNMI SET operations : UPDATE after process restart
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			got := gnmi.Update(t, dut, config.Config(), "SF_B4")
			t.Logf("gNMI Update : %v", got)
		}); errMsg != nil {
			t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
		} else {
			t.Errorf("This gNMI Update should have failed ")
		}

		// Invalid Pathz Rule: Verify the hostname after gNMI SET operation after process restart
		// Fetch and log the state using gNMI
		stateGot := gnmi.Get(t, dut, state.State())
		t.Logf("gNMI Subscribe Response: %v", stateGot)
		time.Sleep(5 * time.Second)

		// Check the fetched state and compare it with the expected value (val)
		if stateGot != val {
			t.Errorf("Telemetry hostname: got %v, expected %s", stateGot, val)
		} else {
			t.Logf("Telemetry hostname: got expected hostname %v", stateGot)
		}

		// Invalid Pathz Rule: Perform gNMI SET operations : DELETE after process restart
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			got := gnmi.Delete(t, dut, config.Config())
			t.Logf("gNMI Delete : %v", got)
		}); errMsg != nil {
			t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
		} else {
			t.Errorf("This gNMI Update should have failed ")
		}

		// Invalid Pathz Rule: Verify the hostname after gNMI SET operation after process restart
		stateGot = gnmi.Get(t, dut, state.State())
		t.Logf("gNMI Subscribe Response: %v", stateGot)
		time.Sleep(5 * time.Second)

		// Check the fetched state and compare it with the expected value (val)
		if stateGot != val {
			t.Errorf("Telemetry hostname: got %v, expected %s", stateGot, val)
		} else {
			t.Logf("Telemetry hostname: got expected hostname %v", stateGot)
		}

		// Invalid Pathz Rule: Perform gNMI SET operations : REPLACE after process restart
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			got := gnmi.Replace(t, dut, config.Config(), "MTB_SF")
			t.Logf("gNMI Update : %v", got)
		}); errMsg != nil {
			t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
		} else {
			t.Errorf("This gNMI Update should have failed ")
		}

		// Invalid Pathz Rule: Verify the hostname after gNMI SET operation after process restart
		stateGot = gnmi.Get(t, dut, state.State())
		t.Logf("gNMI Subscribe Response: %v", stateGot)
		time.Sleep(5 * time.Second)

		// Check the fetched state and compare it with the expected value (val)
		if stateGot != val {
			t.Errorf("Telemetry hostname: got %v, expected %s", stateGot, val)
		} else {
			t.Logf("Telemetry hostname: got expected hostname %v", stateGot)
		}
	} else {

		gnmi.Update(t, dut, config.Config(), "SF_B4")

		// Fetch and log the state using gNMI
		stateGot := gnmi.Get(t, dut, state.State())
		t.Logf("gNMI Subscribe Response: %v", stateGot)
		time.Sleep(5 * time.Second)

		// Check the fetched state and compare it with the expected value (val)
		if stateGot != val {
			t.Errorf("Telemetry hostname: got %v, expected %s", stateGot, val)
		} else {
			t.Logf("Telemetry hostname: got expected hostname %v", stateGot)
		}

		gnmi.Delete(t, dut, config.Config())

		// Fetch and log the state using gNMI
		stateGot = gnmi.Get(t, dut, state.State())
		t.Logf("gNMI Subscribe Response: %v", stateGot)
		time.Sleep(5 * time.Second)

		// Check the fetched state and compare it with the expected value (val)
		if stateGot != val {
			t.Errorf("Telemetry hostname: got %v, expected %s", stateGot, val)
		} else {
			t.Logf("Telemetry hostname: got expected hostname %v", stateGot)
		}

		gnmi.Replace(t, dut, config.Config(), "MTB_SF")

		// Fetch and log the state using gNMI
		stateGot = gnmi.Get(t, dut, state.State())
		t.Logf("gNMI Subscribe Response: %v", stateGot)
		time.Sleep(5 * time.Second)

		// Check the fetched state and compare it with the expected value (val)
		if stateGot != val {
			t.Errorf("Telemetry hostname: got %v, expected %s", stateGot, val)
		} else {
			t.Logf("Telemetry hostname: got expected hostname %v", stateGot)
		}
	}
}

func isPermissionDeniedError(t *testing.T, dut *ondatra.DUTDevice, oper string) {
	config := gnmi.OC().System().Hostname()
	operations := []struct {
		name      string
		operation func(t testing.TB)
	}{
		{"Update", func(t testing.TB) { gnmi.Update(t, dut, config.Config(), "SF2") }},
		{"Delete", func(t testing.TB) { gnmi.Delete(t, dut, config.Config()) }},
		{"Replace", func(t testing.TB) { gnmi.Replace(t, dut, config.Config(), "MTB_SF2") }},
	}

	for _, op := range operations {
		t.Run(op.name+oper, func(t *testing.T) {
			if errMsg := testt.CaptureFatal(t, op.operation); errMsg != nil {
				t.Logf("Expected failure for %s and got testt.CaptureFatal errMsg: %s", op.name, *errMsg)
			} else {
				t.Errorf("This gNMI operation (%s) should have failed", op.name)
			}
		})
	}
}
