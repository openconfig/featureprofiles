package enrollz_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/ondatra/gnmi/oc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/encoding/prototext"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"

	cpb "github.com/openconfig/attestz/proto/common_definitions"
	attestzpb "github.com/openconfig/attestz/proto/tpm_attestz"
	enrollzpb "github.com/openconfig/attestz/proto/tpm_enrollz"
	cert "github.com/openconfig/featureprofiles/internal/cisco/security/cert"
	"google.golang.org/grpc"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	controlcardType   = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	activeController  = oc.Platform_ComponentRedundantRole_PRIMARY
	standbyController = oc.Platform_ComponentRedundantRole_SECONDARY
)

type userCredentials struct {
	username string
	password string
}

var (
	usernameKey = "username"
	passwordKey = "password"
)

func getListOfCardTypes(t *testing.T, dut *ondatra.DUTDevice) [][]string {
	components := gnmi.GetAll[*oc.Component](t, dut, gnmi.OC().ComponentAny().State())
	var controllCards [][]string
	for _, c := range components {
		var card []string
		if c.GetType() == nil {
			t.Logf("Component %s type is missing from telemetry", c.GetName())
			continue
		}
		t.Logf("Component %s has type: %v", c.GetName(), c.GetType())
		switch v := c.GetType().(type) {
		case oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT:
			if v == controlcardType {
				card = append(card, c.GetName())
				card = append(card, c.GetSerialNo())
				if activeController == c.GetRedundantRole() {
					card = append(card, "active")
				} else {
					card = append(card, "standby")
				}
			}
		default:
			t.Logf("Detected non-hardware component: (%T, %v)", c.GetType(), c.GetType())
		}
		controllCards = append(controllCards, card)
	}
	return controllCards
}

func loadFromFile(certPath string, keyPath string, caCertPath string) ([]tls.Certificate, *x509.CertPool, error) {
	certPool := x509.NewCertPool()
	certificate, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, nil, err
	}
	certificate.Leaf, err = x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		return nil, nil, err
	}
	caFile, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, nil, err
	}
	block, _ := pem.Decode(caFile)
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, err
	}
	certs := []tls.Certificate{certificate}
	certPool.AddCert(caCert)
	return certs, certPool, nil
}

func clientCredentials(targetName string, ca string, username string, password string, clientCert string, clientKey string) ([]grpc.DialOption, error) {

	opts := []grpc.DialOption{}
	tlsConfig := &tls.Config{}

	certificates, certPool, err := loadFromFile(clientCert, clientKey, ca)
	if err != nil {
		return nil, err
	}

	tlsConfig.ServerName = targetName
	tlsConfig.Certificates = certificates
	tlsConfig.RootCAs = certPool
	opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	var authorizedUser userCredentials
	authorizedUser.username = username
	authorizedUser.password = password
	opts = append(opts, grpc.WithPerRPCCredentials(&authorizedUser))
	return opts, nil
}

func (a *userCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		usernameKey: a.username,
		passwordKey: a.password,
	}, nil
}

func (a *userCredentials) RequireTransportSecurity() bool {
	return true
}

func grpcConn(targetName string, target string, caCert string, username string, password string, cCert string, cKey string) (grpc.ClientConnInterface, error) {
	opts, err := clientCredentials(targetName, caCert, username, password, cCert, cKey)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.Dial(target, opts...)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func getIakCert(t *testing.T, conn grpc.ClientConnInterface, inputType string) (*enrollzpb.GetIakCertResponse, error) {

	request := &enrollzpb.GetIakCertRequest{
		ControlCardSelection: &cpb.ControlCardSelection{
			ControlCardId: nil,
		},
	}

	switch {
	case inputType == "active":
		request.ControlCardSelection.ControlCardId = &cpb.ControlCardSelection_Role{
			Role: cpb.ControlCardRole_CONTROL_CARD_ROLE_ACTIVE,
		}
	case inputType == "standby":
		request.ControlCardSelection.ControlCardId = &cpb.ControlCardSelection_Role{
			Role: cpb.ControlCardRole_CONTROL_CARD_ROLE_STANDBY,
		}
	case strings.Contains(inputType, "/"):
		request.ControlCardSelection.ControlCardId = &cpb.ControlCardSelection_Slot{
			Slot: inputType,
		}
	default:
		request.ControlCardSelection.ControlCardId = &cpb.ControlCardSelection_Serial{
			Serial: inputType,
		}
	}

	client := enrollzpb.NewTpmEnrollzServiceClient(conn)
	resp, err := client.GetIakCert(context.Background(), request)
	if err != nil {
		return nil, err
	}
	t.Logf("%v", resp)
	return resp, nil
}

func rotateOIAKCert(t *testing.T, conn grpc.ClientConnInterface, inputType string, oIAKCert string, oIDEVIDCert string) (*enrollzpb.RotateOIakCertResponse, error) {

	request := &enrollzpb.RotateOIakCertRequest{
		ControlCardSelection: &cpb.ControlCardSelection{
			ControlCardId: nil,
		},
		OiakCert:    oIAKCert,
		OidevidCert: oIDEVIDCert,
	}

	switch {
	case inputType == "active":
		request.ControlCardSelection.ControlCardId = &cpb.ControlCardSelection_Role{
			Role: cpb.ControlCardRole_CONTROL_CARD_ROLE_ACTIVE,
		}
	case inputType == "standby":
		request.ControlCardSelection.ControlCardId = &cpb.ControlCardSelection_Role{
			Role: cpb.ControlCardRole_CONTROL_CARD_ROLE_STANDBY,
		}
	case strings.Contains(inputType, "/"):
		request.ControlCardSelection.ControlCardId = &cpb.ControlCardSelection_Slot{
			Slot: inputType,
		}
	default:
		request.ControlCardSelection.ControlCardId = &cpb.ControlCardSelection_Serial{
			Serial: inputType,
		}
	}

	client := enrollzpb.NewTpmEnrollzServiceClient(conn)
	resp, err := client.RotateOIakCert(context.Background(), request)
	if err != nil {
		return nil, err
	}
	t.Logf("%v", resp)
	return resp, nil
}

func getTargetFromBindingFile() (string, error) {
	bindingFile := flag.Lookup("binding").Value.String()
	in, err := os.ReadFile(bindingFile)
	if err != nil {
		return "", err
	}
	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		return "", err
	}
	target := b.Duts[0].Gnsi.Target
	return target, nil
}

func getOptionsFromBindingFile() (*bindpb.Options, error) {

	bindingFile := flag.Lookup("binding").Value.String()
	in, err := os.ReadFile(bindingFile)
	if err != nil {
		return nil, err
	}
	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		return nil, err
	}
	options := b.Duts[0].Options
	return options, nil
}

func extractPubKeyFromCert(t *testing.T, cert string) (any, error) {
	certFile := []byte(strings.Replace(string(cert), "\\n", "\n", -1))
	t.Logf("Extracted Pubkey from Cert:\n%s", string(certFile))

	block, _ := pem.Decode(certFile)
	if block == nil {
		return nil, errors.New("Failed to parse certificate PEM")
	}
	parsedCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	return parsedCert.PublicKey, nil

}

func generateCSR(privateKey *rsa.PrivateKey, commonName string) ([]byte, error) {
	csrTemplate := x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: commonName,
		},
	}

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &csrTemplate, privateKey)
	if err != nil {
		return nil, err
	}

	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})
	return csrPEM, nil
}

func signCertificate(csr []byte, caCert *x509.Certificate, caPrivateKey *rsa.PrivateKey, forcePublicKey *rsa.PublicKey, expiredCert bool) ([]byte, error) {
	csrBlock, _ := pem.Decode(csr)
	if csrBlock == nil || csrBlock.Type != "CERTIFICATE REQUEST" {
		return nil, fmt.Errorf("failed to decode PEM block containing CSR")
	}
	csrParsed, err := x509.ParseCertificateRequest(csrBlock.Bytes)
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(123456),
		Subject:      csrParsed.Subject,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(0, 0, 1),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	if expiredCert == true {
		template = x509.Certificate{
			SerialNumber: big.NewInt(123456),
			Subject:      csrParsed.Subject,
			NotBefore:    time.Now().AddDate(0, 0, -2),
			NotAfter:     time.Now().AddDate(0, 0, -1),
			KeyUsage:     x509.KeyUsageDigitalSignature,
		}
	}
	var publicKey interface{}
	if forcePublicKey != nil {
		publicKey = forcePublicKey
	} else {
		publicKey = csrParsed.PublicKey
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, caCert, publicKey, caPrivateKey)
	if err != nil {
		return nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	return certPEM, nil
}

func generateOiakCert(t *testing.T, resp *enrollzpb.GetIakCertResponse, dirName string, expiredCert bool) (string, string, error) {
	iakPubKey, err := extractPubKeyFromCert(t, resp.IakCert)
	if err != nil {
		return "", "", err
	}
	idevidPubKey, err := extractPubKeyFromCert(t, resp.IdevidCert)
	if err != nil {
		return "", "", err
	}
	caCertFile := path.Join(dirName, "cacert.rsa.pem")
	caKeyFile := path.Join(dirName, "cakey.rsa.pem")
	_, err = os.Stat(caCertFile)
	if err == nil {
		err := os.Remove(caCertFile)
		if err != nil {
			return "", "", err
		}
	}
	_, err = os.Stat(caKeyFile)
	if err == nil {
		err := os.Remove(caKeyFile)
		if err != nil {
			return "", "", err
		}
	}
	caKey, caCert, err := cert.GenRootCA("ROOTCA", x509.RSA, 100, dirName)
	if err != nil {
		return "", "", err
	}
	pvtKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return "", "", err
	}
	csrPEM, err := generateCSR(pvtKey, "Google")
	if err != nil {
		return "", "", err
	}
	signedCertiakPEM, err := signCertificate(csrPEM, caCert, caKey.(*rsa.PrivateKey), iakPubKey.(*rsa.PublicKey), expiredCert)
	if err != nil {
		return "", "", err
	}
	signedCertidevidPEM, err := signCertificate(csrPEM, caCert, caKey.(*rsa.PrivateKey), idevidPubKey.(*rsa.PublicKey), expiredCert)

	if err != nil {
		return "", "", err
	}
	return string(signedCertiakPEM), string(signedCertidevidPEM), nil
}

func generateOiakCertWithInValidPubKey(dirName string, expiredCert bool) (string, error) {
	caCertFile := path.Join(dirName, "cacert.rsa.pem")
	caKeyFile := path.Join(dirName, "cakey.rsa.pem")
	_, err := os.Stat(caCertFile)
	if err == nil {
		err := os.Remove(caCertFile)
		if err != nil {
			return "", err
		}
	}
	_, err = os.Stat(caKeyFile)
	if err == nil {
		err := os.Remove(caKeyFile)
		if err != nil {
			return "", err
		}
	}
	caKey, caCert, err := cert.GenRootCA("ROOTCA", x509.RSA, 100, dirName)
	if err != nil {
		return "", err
	}
	pvtKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return "", err
	}
	csrPEM, err := generateCSR(pvtKey, "Google")
	if err != nil {
		return "", err
	}
	signedCertPEM, err := signCertificate(csrPEM, caCert, caKey.(*rsa.PrivateKey), nil, expiredCert)

	if err != nil {
		return "", err
	}
	return string(signedCertPEM), nil
}

func attestPcrs(conn grpc.ClientConnInterface, inputType string, nonce string, pcrIndices string) (*attestzpb.AttestResponse, error) {
	nonceSlice, err := hex.DecodeString(nonce)
	if err != nil {
		return nil, err
	}

	request := &attestzpb.AttestRequest{
		ControlCardSelection: &cpb.ControlCardSelection{
			ControlCardId: nil,
		},
		Nonce:      []byte(nonceSlice),
		HashAlgo:   0,
		PcrIndices: nil,
	}
	request.HashAlgo = attestzpb.Tpm20HashAlgo_TPM20HASH_ALGO_SHA256

	strSlice := strings.Split(pcrIndices, ",")

	for _, str := range strSlice {
		num, err := strconv.Atoi(str)
		if err != nil {
			return nil, err
		}
		request.PcrIndices = append(request.PcrIndices, int32(num))
	}

	switch {
	case inputType == "active":
		request.ControlCardSelection.ControlCardId = &cpb.ControlCardSelection_Role{
			Role: cpb.ControlCardRole_CONTROL_CARD_ROLE_ACTIVE,
		}
	case inputType == "standby":
		request.ControlCardSelection.ControlCardId = &cpb.ControlCardSelection_Role{
			Role: cpb.ControlCardRole_CONTROL_CARD_ROLE_STANDBY,
		}
	case strings.Contains(inputType, "/"):
		request.ControlCardSelection.ControlCardId = &cpb.ControlCardSelection_Slot{
			Slot: inputType,
		}
	default:
		request.ControlCardSelection.ControlCardId = &cpb.ControlCardSelection_Serial{
			Serial: inputType,
		}
	}

	client := attestzpb.NewTpmAttestzServiceClient(conn)
	resp, err := client.Attest(context.Background(), request)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func verifySignature(t *testing.T, resp *attestzpb.AttestResponse) error {
	pubKey, err := extractPubKeyFromCert(t, resp.OiakCert)
	if err != nil {
		t.Logf("Failed to read the Pubkey from OIAK Certificate")
		return err
	}
	// quoteSignData, err := hex.DecodeString(string(resp.QuoteSignature))
	// if err != nil {
	// 	t.Logf("Error decoding hex data: %v", err)
	// 	return err
	// }
	// quoteData, err := hex.DecodeString(string(resp.Quoted))
	// if err != nil {
	// 	t.Logf("Error decoding hex data: %v", err)
	// 	return err
	// }

	quoteSignData, err := base64.StdEncoding.DecodeString(string(resp.QuoteSignature))
	if err != nil {
		return err
	}
	quoteData, err := base64.StdEncoding.DecodeString(string(resp.Quoted))
	if err != nil {
		return err
	}
	hash := sha256.Sum256(quoteData)

	err = rsa.VerifyPKCS1v15(pubKey.(*rsa.PublicKey), crypto.SHA256, hash[:], quoteSignData)
	if err != nil {
		t.Logf("signature verification failed")
		return err
	}

	t.Logf("Signature verification successful")
	return nil
}

func CMDViaGNMI(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, cmd string) string {
	gnmiC := dut.RawAPIs().GNMI(t)
	getRequest := &gpb.GetRequest{
		Prefix: &gpb.Path{
			Origin: "cli",
		},
		Path: []*gpb.Path{
			{
				Elem: []*gpb.PathElem{{
					Name: cmd,
				}},
			},
		},
		Encoding: gpb.Encoding_ASCII,
	}
	t.Logf("get cli (%s) via GNMI: \n %s", cmd, prototext.Format(getRequest))
	if _, deadlineSet := ctx.Deadline(); !deadlineSet {
		tmpCtx, cncl := context.WithTimeout(ctx, time.Second*120)
		ctx = tmpCtx
		defer cncl()
	}
	resp, err := gnmiC.Get(ctx, getRequest)
	if err != nil {
		t.Fatalf("running cmd (%s) via GNMI failed: %v", cmd, err)
	}
	t.Logf("Get cli via gnmi reply: \n %s", prototext.Format(resp))
	return string(resp.GetNotification()[0].GetUpdate()[0].GetVal().GetAsciiVal())
}

func verifyPCRValues(t *testing.T, dut *ondatra.DUTDevice, pcrIndices string, pcrValFromAttestZ map[int32][]byte, controllerCardLoc string) error {
	pcrValFromCMD := CMDViaGNMI(context.Background(), t, dut, fmt.Sprintf("show  platform security attest pcr %s location %s", pcrIndices, controllerCardLoc))

	pattern := regexp.MustCompile(`\d+\s+(.*?)=`)
	pcrValList := pattern.FindAllString(pcrValFromCMD, -1)
	errCount := 0
	for i := 0; i < len(pcrValFromAttestZ); i++ {
		if strings.Contains(pcrValList[i], string(pcrValFromAttestZ[int32(i)])) {
			t.Logf("PCR value %d matched Successfully", i)
		} else {
			t.Logf("PCR value from cmd: %d: %s", i, pcrValList[i])
			t.Logf("PCR value from AttestZ Request: %d: %s", i, string(pcrValFromAttestZ[int32(i)]))
			t.Logf("PCR value %d mismatch", i)
			errCount = errCount + 1
		}
	}
	if errCount != 0 {
		return fmt.Errorf("Error in PCR value Verification")
	}
	return nil
}

func verifyControlCardId(t *testing.T, dut *ondatra.DUTDevice, controlCardIds *cpb.ControlCardVendorId, controllCard []string) error {

	platformData := CMDViaGNMI(context.Background(), t, dut, "show platform")
	pidPattern := regexp.MustCompile(`(.*)\((Active)`)
	pid := pidPattern.FindString(platformData)
	chassisData := CMDViaGNMI(context.Background(), t, dut, "show inventory chassis")

	snPattern := regexp.MustCompile(`SN:\s+(.*)\s+`)
	serialNo := snPattern.FindString(chassisData)

	errCount := 0
	if strings.Contains(pid, controlCardIds.ChassisPartNumber) {
		t.Logf("Chassid Part number verified successfully")
	} else {
		t.Logf("Chassis Part number from CMD: %s", pid)
		t.Logf("Chassis Part number from attestZ request: %s", controlCardIds.ChassisPartNumber)
		t.Logf("Chassid Part number mismatch")
		errCount = errCount + 1
	}
	if strings.Contains(serialNo, controlCardIds.ChassisSerialNumber) {
		t.Logf("Chassis Serial Number verified successfully")
	} else {
		t.Logf("Chassis Serial Number from CMD: %s", serialNo)
		t.Logf("Chassis Serial Number from attestZ request: %s", controlCardIds.ChassisSerialNumber)
		t.Logf("Chassis Serial number mismatch")
		errCount = errCount + 1
	}
	if strings.Contains("Cisco Systems, Inc.", controlCardIds.ChassisManufacturer) {
		t.Logf("Chassis Manufacturer verified successfully")
	} else {
		t.Logf("Expected Chassis Manufacturer: Cisco Systems, Inc.")
		t.Logf("Chassis Manufacturer from attestZ request: %s", controlCardIds.ChassisManufacturer)
		t.Logf("Chassis Manufacturer mismatch")
		errCount = errCount + 1
	}
	if strings.Contains(controllCard[0], controlCardIds.ControlCardSlot) {
		t.Logf("Controll Card Name verified successfully")
	} else {
		t.Logf("Controll Card Name from components list: %s", controllCard[0])
		t.Logf("Controll Card Name from attestZ request: %s", controlCardIds.ControlCardSlot)
		t.Logf("Controll Card Name mismatch")
		errCount = errCount + 1
	}
	if strings.Contains(controllCard[1], controlCardIds.ControlCardSerial) {
		t.Logf("Controll Card serial no verified successfully")
	} else {
		t.Logf("Controll Card serial no from components list: %s", controllCard[1])
		t.Logf("Controll Card serial no from attestZ request: %s", controlCardIds.ControlCardSerial)
		t.Logf("Controll Card serial no mismatch")
		errCount = errCount + 1
	}
	var role string
	if cpb.ControlCardRole_CONTROL_CARD_ROLE_ACTIVE == controlCardIds.ControlCardRole {
		role = "active"
	} else {
		role = "standby"
	}
	if strings.Contains(controllCard[2], role) {
		t.Logf("Controll Card role verified successfully")
	} else {
		t.Logf("Controll Card role from components list: %s", controllCard[2])
		t.Logf("Controll Card role from attestZ request: %s", controlCardIds.ControlCardRole)
		t.Logf("Controll Card role mismatch")
		errCount = errCount + 1
	}
	if errCount != 0 {
		return fmt.Errorf("Error in Controll Card ID value Verification")
	}
	return nil
}

func TestGetIAKCert(t *testing.T) {
	target, err := getTargetFromBindingFile()
	if err != nil {
		t.Fatalf("Error in reading target IP and Port from Binding file: %v", err)
	}
	targetIP := strings.Split(target, ":")[0]
	options, err := getOptionsFromBindingFile()
	if err != nil {
		t.Fatalf("Error in reading Options from binding file: %v", err)
	}
	dut := ondatra.DUT(t, "dut")
	cardTypes := getListOfCardTypes(t, dut)
	conn, err := grpcConn(targetIP, target, options.TrustBundleFile, options.Username, options.Password, options.CertFile, options.KeyFile)
	if err != nil {
		t.Fatalf("Error in establishing Grpc Clinet connection: %v", err)
	}
	for _, cardType := range cardTypes {
		for _, cType := range cardType {
			t.Run(fmt.Sprintf("Get IAK Certificate with %s", cType), func(t *testing.T) {
				_, err := getIakCert(t, conn, cType)
				if err != nil {
					t.Fatalf("Error in getIak Certificate with card type-%s: %v", cType, err)
				}
			})
		}
	}
}

func TestRotateOIak(t *testing.T) {
	dirName := "testdata"
	os.Mkdir(dirName, 0755)
	defer os.RemoveAll(dirName)

	target, err := getTargetFromBindingFile()
	if err != nil {
		t.Fatalf("Error in reading target IP and Port from Binding file: %v", err)
	}
	targetIP := strings.Split(target, ":")[0]

	options, err := getOptionsFromBindingFile()
	if err != nil {
		t.Fatalf("Error in reading Options from binding file: %v", err)
	}

	dut := ondatra.DUT(t, "dut")
	cardTypes := getListOfCardTypes(t, dut)

	conn, err := grpcConn(targetIP, target, options.TrustBundleFile, options.Username, options.Password, options.CertFile, options.KeyFile)
	if err != nil {
		t.Fatalf("Error in establishing Grpc Clinet connection: %v", err)
	}
	for _, cardType := range cardTypes {
		for _, cType := range cardType {
			t.Run(fmt.Sprintf("Roatate OIAK Certificate with %s", cType), func(t *testing.T) {
				var signedCertiakPEM string
				var signedCertidevidPEM string
				var rotateResp *enrollzpb.RotateOIakCertResponse

				resp, err := getIakCert(t, conn, cType)
				if err != nil {
					t.Fatalf("Error in getIak Certificate with card type-%s:%v", cType, err)
				}
				signedCertiakPEM, signedCertidevidPEM, err = generateOiakCert(t, resp, dirName, false)
				if err != nil {
					t.Fatalf("Error in generating OIAK Certificate: %v", err)
				}
				rotateResp, err = rotateOIAKCert(t, conn, cType, signedCertiakPEM, signedCertidevidPEM)
				if err != nil {
					t.Fatalf("Error in rotate OIAK Certificate with card type-%s: %v", cType, err)
				}
				t.Logf("%s", rotateResp)
			})
		}
	}

	for _, cardType := range cardTypes {
		for _, cType := range cardType {
			t.Run(fmt.Sprintf("Roatate Expired OIAK Certificate with %s", cType), func(t *testing.T) {
				var signedCertiakPEM string
				var signedCertidevidPEM string
				var rotateResp *enrollzpb.RotateOIakCertResponse

				resp, err := getIakCert(t, conn, cType)
				if err != nil {
					t.Fatalf("Error in getIak Certificate with card type-%s:%v", cType, err)
				}
				signedCertiakPEM, signedCertidevidPEM, err = generateOiakCert(t, resp, dirName, true)
				if err != nil {
					t.Fatalf("Error in generating OIAK Certificate: %v", err)
				}
				rotateResp, err = rotateOIAKCert(t, conn, cType, signedCertiakPEM, signedCertidevidPEM)
				if err != nil {
					if strings.Contains(err.Error(), "AttestZ client oIAK certificate not valid yet") {
						t.Logf("Certificate not valid yet is an expected error")
					} else {
						t.Fatalf("Error in rotate OIAK Certificate with card type-%s: %v", cType, err)
					}
				}
				t.Logf("%s", rotateResp)
			})
		}
	}
	for _, cardType := range cardTypes {
		for _, cType := range cardType {
			t.Run(fmt.Sprintf("Roatate OIAK and ODEVID Certificate with Invalid PubKey %s", cType), func(t *testing.T) {
				var signedCertPEM string
				var rotateResp *enrollzpb.RotateOIakCertResponse
				signedCertPEM, err = generateOiakCertWithInValidPubKey(dirName, false)
				if err != nil {
					t.Fatalf("Error in generating OIAK Certificate: %v", err)
				}
				rotateResp, err = rotateOIAKCert(t, conn, cType, signedCertPEM, signedCertPEM)
				if err != nil {
					if strings.Contains(err.Error(), "AttestZ client mismatch oIAK pubkey") {
						t.Logf("AttestZ client mismatch OIAK pubkey is an expected error")
					} else {
						t.Fatalf("Error in rotate OIAK Certificate with card type-%s: %v", cType, err)
					}
				}
				t.Logf("%s", rotateResp)
			})
		}
	}
}

func TestGetPCRIndices(t *testing.T) {

	pcrIndices := "0,1,2,3,4,5,6,7,8,9"
	target, err := getTargetFromBindingFile()
	if err != nil {
		t.Fatalf("Error in reading target IP and Port from Binding file: %v", err)
	}
	targetIP := strings.Split(target, ":")[0]
	options, err := getOptionsFromBindingFile()
	if err != nil {
		t.Fatalf("Error in reading Options from binding file: %v", err)
	}
	dut := ondatra.DUT(t, "dut")
	cardTypes := getListOfCardTypes(t, dut)
	conn, err := grpcConn(targetIP, target, options.TrustBundleFile, options.Username, options.Password, options.CertFile, options.KeyFile)
	if err != nil {
		t.Fatalf("Error in establishing Grpc Clinet connection: %v", err)
	}
	for _, cardType := range cardTypes {
		for _, cType := range cardType {
			t.Run(fmt.Sprintf("Get IAK Certificate with %s", cType), func(t *testing.T) {
				resp, err := attestPcrs(conn, cType, "1234", pcrIndices)
				if err != nil {
					t.Fatalf("Error in getIak Certificate with card type-%s: %v", cType, err)
				}
				t.Logf("%v", resp)

				err = verifyControlCardId(t, dut, resp.ControlCardId, cardType)
				if err != nil {
					t.Fatalf(err.Error())
				} else {
					t.Logf("Controll Card ID values Verification Successfull")
				}

				err = verifySignature(t, resp)
				if err != nil {
					t.Fatalf("Error in verifying signature %v", err)
				}

				err = verifyPCRValues(t, dut, pcrIndices, resp.PcrValues, cardType[0])
				if err != nil {
					t.Fatalf(err.Error())
				} else {
					t.Logf("PCR values Verification Successfull")
				}
			})
		}
	}
}
