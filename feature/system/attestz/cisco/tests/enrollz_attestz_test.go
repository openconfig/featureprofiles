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
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/ondatra/gnmi/oc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/encoding/prototext"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	certzpb "github.com/openconfig/gnsi/certz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/gnmi"

	cpb "github.com/openconfig/attestz/proto/common_definitions"
	attestzpb "github.com/openconfig/attestz/proto/tpm_attestz"
	enrollzpb "github.com/openconfig/attestz/proto/tpm_enrollz"
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
				controllCards = append(controllCards, card)
			}

		default:
			t.Logf("Detected non-hardware component: (%T, %v)", c.GetType(), c.GetType())
		}

	}
	t.Logf("ControllCards list: %v", controllCards)
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

	conn, err := grpc.NewClient(target, opts...)
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

func rotateOIAKCert(t *testing.T, conn grpc.ClientConnInterface, inputType string, oIAKCert string, oIDEVIDCert string, sslProfileID string) (*enrollzpb.RotateOIakCertResponse, error) {

	request := &enrollzpb.RotateOIakCertRequest{
		ControlCardSelection: &cpb.ControlCardSelection{
			ControlCardId: nil,
		},
		OiakCert:     oIAKCert,
		OidevidCert:  oIDEVIDCert,
		SslProfileId: sslProfileID,
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
	if options.CertFile == "" && options.KeyFile == "" {
		options = b.Duts[0].Gnsi
		if options.CertFile == "" && options.KeyFile == "" {
			options = b.Options
		}
	}
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

func signCertificate(ip string, csr []byte, caCert *x509.Certificate, caPrivateKey *rsa.PrivateKey, forcePublicKey *rsa.PublicKey, expiredCert bool) ([]byte, error) {
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
		IPAddresses:  []net.IP{net.ParseIP(ip)},
	}
	if expiredCert == true {
		template = x509.Certificate{
			SerialNumber: big.NewInt(123456),
			Subject:      csrParsed.Subject,
			NotBefore:    time.Now().AddDate(0, 0, -2),
			NotAfter:     time.Now().AddDate(0, 0, -1),
			KeyUsage:     x509.KeyUsageDigitalSignature,
			IPAddresses:  []net.IP{net.ParseIP(ip)},
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

func generateOiakCert(t *testing.T, ip string, caCertFile string, resp *enrollzpb.GetIakCertResponse, dirName string, expiredCert bool) (string, string, error) {
	caPath, _ := filepath.Split(caCertFile)
	caKeyFile := filepath.Join(caPath, "ca.key.pem")
	iakPubKey, err := extractPubKeyFromCert(t, resp.IakCert)
	if err != nil {
		return "", "", err
	}
	idevidPubKey, err := extractPubKeyFromCert(t, resp.IdevidCert)
	if err != nil {
		return "", "", err
	}
	caCertPem, err := os.ReadFile(caCertFile)
	if err != nil {
		return "", "", err
	}
	caCertpemBlock, _ := pem.Decode(caCertPem)
	if caCertpemBlock == nil {
		return "", "", errors.New("Failed to decode PEM block containing the certificate")
	}

	caCert, err := x509.ParseCertificate(caCertpemBlock.Bytes)
	if err != nil {
		return "", "", err
	}

	caKeyPem, err := os.ReadFile(caKeyFile)
	if err != nil {
		return "", "", err
	}

	caKeypemBlock, _ := pem.Decode(caKeyPem)
	if caKeypemBlock == nil {
		return "", "", errors.New("Failed to decode PEM block containing the certificate")
	}
	caKey, err := x509.ParsePKCS1PrivateKey(caKeypemBlock.Bytes)
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

	signedCertiakPEM, err := signCertificate(ip, csrPEM, caCert, caKey, iakPubKey.(*rsa.PublicKey), expiredCert)
	if err != nil {
		return "", "", err
	}
	signedCertidevidPEM, err := signCertificate(ip, csrPEM, caCert, caKey, idevidPubKey.(*rsa.PublicKey), expiredCert)

	if err != nil {
		return "", "", err
	}
	return string(signedCertiakPEM), string(signedCertidevidPEM), nil
}

func generateOiakCertWithInValidPubKey(ip string, caCertFile string, dirName string, expiredCert bool) (string, error) {
	caPath, _ := filepath.Split(caCertFile)
	caKeyFile := filepath.Join(caPath, "ca.key.pem")
	caCertPem, err := os.ReadFile(caCertFile)
	if err != nil {
		return "", err
	}
	caCertpemBlock, _ := pem.Decode(caCertPem)
	if caCertpemBlock == nil {
		return "", errors.New("Failed to decode PEM block containing the certificate")
	}

	caCert, err := x509.ParseCertificate(caCertpemBlock.Bytes)
	if err != nil {
		return "", err
	}

	caKeyPem, err := os.ReadFile(caKeyFile)
	if err != nil {
		return "", err
	}

	caKeypemBlock, _ := pem.Decode(caKeyPem)
	if caKeypemBlock == nil {
		return "", errors.New("Failed to decode PEM block containing the certificate")
	}
	caKey, err := x509.ParsePKCS1PrivateKey(caKeypemBlock.Bytes)
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
	signedCertPEM, err := signCertificate(ip, csrPEM, caCert, caKey, nil, expiredCert)

	if err != nil {
		return "", err
	}
	return string(signedCertPEM), nil
}

func attestPcrs(t *testing.T, conn grpc.ClientConnInterface, inputType string, nonce string, pcrIndices string) (*attestzpb.AttestResponse, error) {
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
	t.Logf("%v", request)
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

func concatenatePreviousHexValAndDoSHA256(digestVal string, pcrVal string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(digestVal)
	if err != nil {
		return "", err
	}
	hexStr := hex.EncodeToString(decoded)
	var byteValue []byte
	if pcrVal != "" {
		byteValue, err = hex.DecodeString(pcrVal + hexStr)
	} else {
		byteValue, err = hex.DecodeString(strings.Repeat("0", len(hexStr)) + hexStr)
	}
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(byteValue)
	sha256Hash := hash[:]
	sha256Hex := hex.EncodeToString(sha256Hash)
	return sha256Hex, nil
}

func verifyPCRValues(t *testing.T, dut *ondatra.DUTDevice, pcrValFromAttestZ map[int32][]byte, controllerCardLoc string) error {
	pcrValFromCMD := CMDViaGNMI(context.Background(), t, dut, fmt.Sprintf("show  platform security integrity log boot location %s", controllerCardLoc))
	pattern := `Event Number:\s+\d[\s\S]+?Event Data:`
	re := regexp.MustCompile(pattern)
	matches := re.FindAllString(pcrValFromCMD, -1)
	pcr := make(map[string]string)
	pcrRe := regexp.MustCompile(`PCR Index:\s+(\d+)`)
	digestRe := regexp.MustCompile(`SHA256\nEvent Digest:\s+(.+)`)
	for _, event := range matches[1:] {
		pcrIndex := pcrRe.FindStringSubmatch(event)[1]
		digestVal := digestRe.FindStringSubmatch(event)[1]
		if _, ok := pcr[pcrIndex]; ok {
			pcrVal, err := concatenatePreviousHexValAndDoSHA256(digestVal, pcr[pcrIndex])
			if err != nil {
				return err
			}
			pcr[pcrIndex] = pcrVal
		} else {
			pcrVal, err := concatenatePreviousHexValAndDoSHA256(digestVal, "")
			if err != nil {
				return err
			}
			pcr[pcrIndex] = pcrVal
		}
	}
	errCount := 0
	for i := 0; i < len(pcr); i++ {
		bytes, err := hex.DecodeString(pcr[strconv.Itoa(i)])
		if err != nil {
			return err
		}
		base64String := base64.StdEncoding.EncodeToString(bytes)
		if base64String == string(pcrValFromAttestZ[int32(i)]) {
			t.Logf("PCR value %d matched Successfully", i)
		} else {
			t.Logf("PCR value from cmd: %d: %s", i, pcr[strconv.Itoa(i)])
			t.Logf("PCR value from AttestZ Request: %d: %s", i, string(pcrValFromAttestZ[int32(i)]))
			t.Logf("PCR value %d mismatch", i)
			errCount = errCount + 1
		}
	}
	if errCount != 0 {
		return fmt.Errorf("PCR value Mismatch")
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

func readCertificatesFromFile(filename string) ([]*x509.Certificate, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var certificates []*x509.Certificate
	block, rest := pem.Decode(data)
	for block != nil {
		if block.Type == "CERTIFICATE" {
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, err
			}
			certificates = append(certificates, cert)
		}
		if len(rest) == 0 {
			break
		}
		block, rest = pem.Decode(rest)
	}

	return certificates, nil
}

func GetSudiRootCA(t *testing.T) (string, error) {
	dut := ondatra.DUT(t, "dut")
	ciscoHASudi := CMDViaGNMI(context.Background(), t, dut, "show platform security attest certificate CiscoHASUDI")
	t.Logf("%v", ciscoHASudi)
	pattern := `-----BEGIN CERTIFICATE-----[\s\S]+?-----END CERTIFICATE-----`
	re := regexp.MustCompile(pattern)
	matches := re.FindAllString(ciscoHASudi, -1)[:2]
	if len(matches) == 0 {
		return "", errors.New("Failed to Parse 'show platform security attest certificate CiscoHASUDI' cmd output")
	}
	certs := strings.Join(matches[:2], "\n")
	t.Log(certs)
	return certs, nil
}

func gnsiNewConnWithIdevID(t *testing.T, target string, options *bindpb.Options) (*grpc.ClientConn, error) {
	targetIP := strings.Split(target, ":")[0]
	certificate, err := tls.LoadX509KeyPair(options.CertFile, options.KeyFile)
	if err != nil {
		t.Fatalf("Failed to load client certificate and key: %v", err)
	}

	caCert, err := GetSudiRootCA(t)
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM([]byte(caCert))
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{certificate},
		RootCAs:            caCertPool,
		MaxVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true, // To Skip the SAN verification for SUDI Cert
		ServerName:         targetIP,
	}
	tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		opts := x509.VerifyOptions{
			Roots: caCertPool,
		}
		for _, rawCert := range rawCerts {
			cert, err := x509.ParseCertificate(rawCert)
			if err != nil {
				return fmt.Errorf("failed to parse certificate: %v", err)
			}
			if _, err := cert.Verify(opts); err != nil {
				return fmt.Errorf("certificate verification failed: %v", err)
			}
		}
		return nil
	}

	opts := []grpc.DialOption{}
	opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	var authorizedUser userCredentials
	authorizedUser.username = "cafyauto"
	authorizedUser.password = "cisco123"
	opts = append(opts, grpc.WithPerRPCCredentials(&authorizedUser))

	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return nil, fmt.Errorf("Failed to dial server: %v", err)
	}

	return conn, nil
}

func createCertZProfileAndRotateTrustBundle(t *testing.T, gnsiC binding.GNSIClients, profileId string,
	trustBundleFile string, certSource certzpb.Certificate_CertSource) error {

	_, _ = gnsiC.Certz().DeleteProfile(context.Background(), &certzpb.DeleteProfileRequest{SslProfileId: profileId})
	profiles, err := gnsiC.Certz().AddProfile(context.Background(), &certzpb.AddProfileRequest{SslProfileId: profileId})
	if err != nil {
		return err
	}
	t.Logf("Profile add successful %v", profiles)

	//ROTATE with TRUSTBUNDLE
	certificates, err := readCertificatesFromFile(trustBundleFile)
	if err != nil {
		return err
	}
	var certChainMessage certzpb.CertificateChain
	var x509toPEM = func(cert *x509.Certificate) []byte {
		return pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		})
	}
	for i, cert := range certificates {
		certMessage := &certzpb.Certificate{
			Type:            certzpb.CertificateType_CERTIFICATE_TYPE_X509,
			Encoding:        certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
			CertificateType: &certzpb.Certificate_RawCertificate{RawCertificate: x509toPEM(cert)},
		}
		if i > 0 {
			certChainMessage.Parent = &certzpb.CertificateChain{
				Certificate: certMessage,
				Parent:      certChainMessage.Parent,
			}
		} else {
			certChainMessage = certzpb.CertificateChain{
				Certificate: certMessage,
			}
		}
	}

	stream, err := gnsiC.Certz().Rotate(context.Background())
	if err != nil {
		t.Fatalf("failed to get stream:%v", err)
	}
	var response *certzpb.RotateCertificateResponse

	request := &certzpb.RotateCertificateRequest{
		ForceOverwrite: true,
		SslProfileId:   profileId,
		RotateRequest: &certzpb.RotateCertificateRequest_Certificates{
			Certificates: &certzpb.UploadRequest{
				Entities: []*certzpb.Entity{
					{
						Version:   "1.0",
						CreatedOn: 123456789,
						Entity: &certzpb.Entity_CertificateChain{
							CertificateChain: &certzpb.CertificateChain{
								Certificate: &certzpb.Certificate{
									Type:            certzpb.CertificateType_CERTIFICATE_TYPE_X509,
									Encoding:        certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
									CertificateType: &certzpb.Certificate_CertSource_{CertSource: certSource},
									PrivateKeyType:  &certzpb.Certificate_KeySource_{KeySource: certzpb.Certificate_KEY_SOURCE_IDEVID_TPM},
								},
							},
						},
					},
					{
						Version:   "1.0",
						CreatedOn: 123456789,
						Entity: &certzpb.Entity_TrustBundle{
							TrustBundle: &certChainMessage,
						},
					},
				},
			},
		},
	}
	//log.V(1).Info("RotateCertificateRequest:\n", prettyPrint(request))
	t.Logf("RotateCertificateRequest:%v", request)
	if err = stream.Send(request); err != nil {
		t.Fatalf("failed to send RotateRequest:%v", err)
	}
	if response, err = stream.Recv(); err != nil {
		t.Fatalf("failed to receive RotateCertificateResponse:%v", err)
	}
	t.Logf("RotateCertificateResponse:%v", response)
	t.Logf("Rotate successful %v", request)

	//FINALIZE ROTATE REQUEST
	request = &certzpb.RotateCertificateRequest{
		ForceOverwrite: true,
		SslProfileId:   profileId,
		RotateRequest:  &certzpb.RotateCertificateRequest_FinalizeRotation{FinalizeRotation: &certzpb.FinalizeRequest{}},
	}
	t.Logf("RotateCertificateRequest:%v", request)
	if err := stream.Send(request); err != nil {
		t.Fatalf("failed to send RotateRequest:%v", err)
	}
	if _, err = stream.Recv(); err != nil {
		if err != io.EOF {
			t.Fatalf("Failed, finalize Rotation is cancelled: %v", err)
		}
	}
	t.Logf("RotateCertificateFinalize: Success, stream has ended")
	stream.CloseSend()
	return nil
}

func TestGetIAKCert(t *testing.T) {
	target, err := getTargetFromBindingFile()
	if err != nil {
		t.Fatalf("Error in reading target IP and Port from Binding file: %v", err)
	}
	targetIP := strings.Split(target, ":")[0]
	options, err := getOptionsFromBindingFile()
	t.Logf("options: %v", options)
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

func TestEnrollZFlow(t *testing.T) {
	dirName := "testdata"
	profileId := "CERTZ_ENROLLZ"
	os.Mkdir(dirName, 0755)
	defer os.RemoveAll(dirName)

	target, err := getTargetFromBindingFile()
	targetIP := strings.Split(target, ":")[0]
	if err != nil {
		t.Fatalf("Error in reading target IP and Port from Binding file: %v", err)
	}

	options, err := getOptionsFromBindingFile()
	if err != nil {
		t.Fatalf("Error in reading Options from binding file: %v", err)
	}

	dut := ondatra.DUT(t, "dut")
	gnsiC := dut.RawAPIs().GNSI(t)

	// Adding New SSL Profile and Rotate CertSource as IDEVID
	err = createCertZProfileAndRotateTrustBundle(t, gnsiC, profileId, options.TrustBundleFile,
		certzpb.Certificate_CERT_SOURCE_IDEVID)
	if err != nil {
		t.Fatalf("Error in Create and Rotate CertZ Profile: %v", err)
	}

	//CONFIG SSL Profile ID under GRPC
	t.Log("Config new gNSI CLI")
	configToChange := fmt.Sprintf("grpc gnsi service certz ssl-profile-id %s \n", profileId)
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	// Create a new connection with IDEVID
	gnsiCNew, err := gnsiNewConnWithIdevID(t, target, options)
	if err != nil {
		t.Fatalf("Error in establishing connection with IDEVID: %v", err)
	}
	defer gnsiCNew.Close()
	client := certzpb.NewCertzClient(gnsiCNew)

	//Get-profile with rotated CERTS
	profilelist, err := client.GetProfileList(context.Background(), &certzpb.GetProfileListRequest{})
	if err != nil {
		t.Fatalf("Unexpected Error in getting profile list: %v", err)
	}
	t.Logf("Profile list get was successful, %v", profilelist)

	//Rotate OIAK and OIDEVID
	cardTypes := getListOfCardTypes(t, dut)
	for _, cardType := range cardTypes {
		for _, cType := range cardType {
			t.Run(fmt.Sprintf("Rotate OIAK Certificate with %s", cType), func(t *testing.T) {
				var signedCertiakPEM string
				var signedCertidevidPEM string
				var rotateResp *enrollzpb.RotateOIakCertResponse

				resp, err := getIakCert(t, gnsiCNew, cType)
				if err != nil {
					t.Fatalf("Error in getIak Certificate with card type-%s:%v", cType, err)
				}
				signedCertiakPEM, signedCertidevidPEM, err = generateOiakCert(t, targetIP, options.TrustBundleFile, resp, dirName, false)
				if err != nil {
					t.Fatalf("Error in generating OIAK Certificate: %v", err)
				}
				rotateResp, err = rotateOIAKCert(t, gnsiCNew, cType, signedCertiakPEM, signedCertidevidPEM, profileId)
				if err != nil {
					t.Fatalf("Error in rotate OIAK Certificate with card type-%s: %v", cType, err)
				}
				t.Logf("%s", rotateResp)
			})
		}
	}

	//Close the IDEVID Connection and Create a connection again and check
	gnsiCNew.Close()
	gnsiCNew, err = gnsiNewConnWithIdevID(t, target, options)
	if err != nil {
		t.Fatalf("Error in establishing connection with IDEVID: %v", err)
	}
	client = certzpb.NewCertzClient(gnsiCNew)

	_, err = client.GetProfileList(context.Background(), &certzpb.GetProfileListRequest{})
	if err != nil {
		t.Logf("Error in getting profile list which is expected: %v", err)
	}

	//Create a connection with OIDEVID (with Ondatra CA)
	conn, err := grpcConn(targetIP, target, options.TrustBundleFile, options.Username, options.Password, options.CertFile, options.KeyFile)
	if err != nil {
		t.Fatalf("Error in establishing Grpc Clinet connection: %v", err)
	}
	client = certzpb.NewCertzClient(conn)

	nonce := "1234"
	pcrIndices := "0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23"
	t.Logf("%v", cardTypes)
	for _, cardType := range cardTypes {
		for _, cType := range cardType {
			t.Run(fmt.Sprintf("Get PCR Indices with %s", cType), func(t *testing.T) {
				resp, err := attestPcrs(t, conn, cType, nonce, pcrIndices)
				if err != nil {
					t.Fatalf("Error in get PCR Indices with card type-%s: %v", cType, err)
				}
				t.Logf("%v", resp)

				err = verifyControlCardId(t, dut, resp.ControlCardId, cardType)
				if err != nil {
					t.Fatal(err.Error())
				} else {
					t.Logf("Controll Card ID values Verification Successfull")
				}

				err = verifySignature(t, resp)
				if err != nil {
					t.Fatalf("Error in verifying signature %v", err)
				}

				err = verifyPCRValues(t, dut, resp.PcrValues, cardType[0])
				if err != nil {
					t.Fatal(err.Error())
				} else {
					t.Logf("PCR values Verification Successfull")
				}
			})
		}
	}

	//UnCONFIG SSL Profile ID under GRPC
	t.Log("UnConfig new gNSI CLI")
	configToremove := fmt.Sprintf("no grpc gnsi service certz ssl-profile-id %s \n", profileId)
	ctx = context.Background()
	util.GNMIWithText(ctx, t, dut, configToremove)

	//Delete rotated profile-id
	delprofile, err := client.DeleteProfile(context.Background(), &certzpb.DeleteProfileRequest{SslProfileId: profileId})
	if err != nil {
		t.Fatalf("Unexpected Error in deleting profile list: %v", err)
	}
	t.Logf("Delete Profile was successful, %v", delprofile)

}

func TestRotateOIak(t *testing.T) {
	dirName := "testdata"
	profileId := "CERTZ_ENROLLZ"
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
	gnsiC := dut.RawAPIs().GNSI(t)

	err = createCertZProfileAndRotateTrustBundle(t, gnsiC, profileId, options.TrustBundleFile,
		certzpb.Certificate_CERT_SOURCE_OIDEVID)
	if err != nil {
		t.Fatalf("Error in Create and Rotate CertZ Profile: %v", err)
	}

	t.Log("Config new gNSI CLI")
	configToChange := fmt.Sprintf("grpc gnsi service certz ssl-profile-id %s \n", profileId)
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	conn, err := grpcConn(targetIP, target, options.TrustBundleFile, options.Username, options.Password, options.CertFile, options.KeyFile)
	if err != nil {
		t.Fatalf("Error in establishing Grpc Clinet connection: %v", err)
	}
	cardTypes := getListOfCardTypes(t, dut)
	for _, cardType := range cardTypes {
		for _, cType := range cardType {
			t.Run(fmt.Sprintf("Rotate OIAK Certificate with %s", cType), func(t *testing.T) {
				var signedCertiakPEM string
				var signedCertidevidPEM string
				var rotateResp *enrollzpb.RotateOIakCertResponse

				resp, err := getIakCert(t, conn, cType)
				if err != nil {
					t.Fatalf("Error in getIak Certificate with card type-%s:%v", cType, err)
				}
				signedCertiakPEM, signedCertidevidPEM, err = generateOiakCert(t, targetIP, options.TrustBundleFile, resp, dirName, false)
				if err != nil {
					t.Fatalf("Error in generating OIAK Certificate: %v", err)
				}
				rotateResp, err = rotateOIAKCert(t, conn, cType, signedCertiakPEM, signedCertidevidPEM, profileId)
				if err != nil {
					t.Fatalf("Error in rotate OIAK Certificate with card type-%s: %v", cType, err)
				}
				t.Logf("%s", rotateResp)
			})
		}
	}

	for _, cardType := range cardTypes {
		for _, cType := range cardType {
			t.Run(fmt.Sprintf("Rotate Expired OIAK Certificate with %s", cType), func(t *testing.T) {
				var signedCertiakPEM string
				var signedCertidevidPEM string
				var rotateResp *enrollzpb.RotateOIakCertResponse

				resp, err := getIakCert(t, conn, cType)
				if err != nil {
					t.Fatalf("Error in getIak Certificate with card type-%s:%v", cType, err)
				}
				signedCertiakPEM, signedCertidevidPEM, err = generateOiakCert(t, targetIP, options.TrustBundleFile, resp, dirName, true)
				if err != nil {
					t.Fatalf("Error in generating OIAK Certificate: %v", err)
				}
				rotateResp, err = rotateOIAKCert(t, conn, cType, signedCertiakPEM, signedCertidevidPEM, profileId)
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
			t.Run(fmt.Sprintf("Rotate OIAK and ODEVID Certificate with Invalid PubKey %s", cType), func(t *testing.T) {
				var signedCertPEM string
				var rotateResp *enrollzpb.RotateOIakCertResponse
				signedCertPEM, err = generateOiakCertWithInValidPubKey(targetIP, options.TrustBundleFile, dirName, false)
				if err != nil {
					t.Fatalf("Error in generating OIAK Certificate: %v", err)
				}
				rotateResp, err = rotateOIAKCert(t, conn, cType, signedCertPEM, signedCertPEM, profileId)
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

	//UnCONFIG SSL Profile ID under GRPC
	t.Log("UnConfig new gNSI CLI")
	configToremove := fmt.Sprintf("no grpc gnsi service certz ssl-profile-id %s \n", profileId)
	ctx = context.Background()
	util.GNMIWithText(ctx, t, dut, configToremove)

	//Delete rotated profile-id
	delprofile, err := gnsiC.Certz().DeleteProfile(context.Background(), &certzpb.DeleteProfileRequest{SslProfileId: profileId})
	if err != nil {
		t.Fatalf("Unexpected Error in deleting profile list: %v", err)
	}
	t.Logf("Delete Profile was successful, %v", delprofile)
}

func TestGetPCRIndices(t *testing.T) {

	pcrIndices := "0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23"
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
	nonce := "1234"
	t.Logf("%v", cardTypes)
	for _, cardType := range cardTypes {
		for _, cType := range cardType {
			t.Run(fmt.Sprintf("Get PCR Indices with %s", cType), func(t *testing.T) {
				resp, err := attestPcrs(t, conn, cType, nonce, pcrIndices)
				if err != nil {
					t.Fatalf("Error in get PCR Indices with card type-%s: %v", cType, err)
				}
				t.Logf("%v", resp)

				err = verifyControlCardId(t, dut, resp.ControlCardId, cardType)
				if err != nil {
					t.Fatal(err.Error())
				} else {
					t.Logf("Controll Card ID values Verification Successfull")
				}

				err = verifySignature(t, resp)
				if err != nil {
					t.Fatalf("Error in verifying signature %v", err)
				}

				err = verifyPCRValues(t, dut, resp.PcrValues, cardType[0])
				if err != nil {
					t.Fatal(err.Error())
				} else {
					t.Logf("PCR values Verification Successfull")
				}
			})
		}
	}
	pcrIndices = "0,1,2,3,4,0,1,2,3,4"
	for _, cardType := range cardTypes {
		for _, cType := range cardType {
			t.Run(fmt.Sprintf("Test Invalid PCR Indices with  %s", cType), func(t *testing.T) {
				resp, err := attestPcrs(t, conn, cType, nonce, pcrIndices)
				if err != nil {
					t.Logf("Error in Get PCR Indices with card type-%s which is expected: %v", cType, err)
				} else {
					t.Fatalf("Getting PCR Indices with card type-%s which is not expected: %v", cType, resp)
				}
			})
		}
	}
}
