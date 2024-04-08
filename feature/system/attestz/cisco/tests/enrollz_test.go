package enrollz_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"
	"time"

	log "github.com/golang/glog"

	"github.com/openconfig/featureprofiles/internal/fptest"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/ondatra/gnmi/oc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/encoding/prototext"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"

	"github.com/golang/protobuf/proto"
	"github.com/google/gnxi/utils/entity"
	cpb "github.com/openconfig/attestz/proto/common_definitions"
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

func FindComponentSerialnoRoleByType(t *testing.T, dut *ondatra.DUTDevice, cType oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT) ([]string, []string, []string) {
	components := gnmi.GetAll[*oc.Component](t, dut, gnmi.OC().ComponentAny().State())
	var name []string
	var serialNo []string
	var role []string
	for _, c := range components {
		if c.GetType() == nil {
			t.Logf("Component %s type is missing from telemetry", c.GetName())
			continue
		}
		t.Logf("Component %s has type: %v", c.GetName(), c.GetType())
		switch v := c.GetType().(type) {
		case oc.E_PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT:
			if v == cType {
				name = append(name, c.GetName())
				serialNo = append(serialNo, c.GetSerialNo())
				if activeController == c.GetRedundantRole() {
					role = append(role, "active")
				} else {
					role = append(role, "standby")
				}
			}
		default:
			t.Logf("Detected non-hardware component: (%T, %v)", c.GetType(), c.GetType())
		}
	}
	return name, serialNo, role
}

func generateFromCA(targetName string, ca string, caKey string) ([]tls.Certificate, *x509.CertPool) {
	var caEnt *entity.Entity
	var err error
	certPool := x509.NewCertPool()
	caEnt, err = entity.FromFile(ca, caKey)
	if err != nil {
		log.Exitf("Failed to load certificate and key from file: %v", err)
	}
	clientEnt, err := entity.CreateSigned(targetName, nil, caEnt)
	if err != nil {
		log.Exitf("Failed to create a signed entity: %v", err)
	}
	certs := []tls.Certificate{*clientEnt.Certificate}
	certPool.AddCert(caEnt.Certificate.Leaf)

	return certs, certPool
}

type userCredentials struct {
	username string
	password string
}

var (
	usernameKey = "username"
	passwordKey = "password"
)

func clientCredentials(targetName string, ca string, caKey string, username string, password string) []grpc.DialOption {

	opts := []grpc.DialOption{}
	tlsConfig := &tls.Config{}

	certificates, certPool := generateFromCA(targetName, ca, caKey)
	tlsConfig.ServerName = targetName
	tlsConfig.Certificates = certificates
	tlsConfig.RootCAs = certPool
	opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	var authorizedUser userCredentials
	authorizedUser.username = username
	authorizedUser.password = password
	opts = append(opts, grpc.WithPerRPCCredentials(&authorizedUser))
	return opts
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

func grpcConn(targetName string, target string, caCert string, username string, password string) (grpc.ClientConnInterface, error) {
	opts := clientCredentials(targetName, caCert,
		"/ws/anidamod-bgl/gNSI_B4/featureprofiles/internal/cisco/security/cert/keys/CA/ca.key.pem", username, password)

	conn, err := grpc.Dial(target, opts...)
	objType := reflect.TypeOf(conn)
	log.Infof("%s", objType)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func getIakCert(conn grpc.ClientConnInterface, inputType string) (*enrollzpb.GetIakCertResponse, error) {

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

	// log.V(1).Info("GetIakCertRequest:\n", proto.MarshalTextString(request))
	client := enrollzpb.NewTpmEnrollzServiceClient(conn)
	resp, err := client.GetIakCert(context.Background(), request)
	if err != nil {
		return nil, err
	}
	log.Infof(proto.MarshalTextString(resp))
	return resp, nil
}

func rotateOIAKCert(conn grpc.ClientConnInterface, inputType string, oIAKCert string) (*enrollzpb.RotateOIakCertResponse, error) {

	request := &enrollzpb.RotateOIakCertRequest{
		ControlCardSelection: &cpb.ControlCardSelection{
			ControlCardId: nil,
		},
		OiakCert: oIAKCert,
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

	// log.V(1).Info("GetIakCertRequest:\n", proto.MarshalTextString(request))
	client := enrollzpb.NewTpmEnrollzServiceClient(conn)
	resp, err := client.RotateOIakCert(context.Background(), request)
	if err != nil {
		return nil, err
	}
	log.Infof(proto.MarshalTextString(resp))
	log.V(1).Info("Roate OIAK cert success:\n", proto.MarshalTextString(resp))
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

func extractPubKeyFromCert(cert string) (any, error) {
	certFile := []byte(strings.Replace(string(cert), "\\n", "\n", -1))
	log.Infof("Extracted Pubkey from IAK Cert:\n%s", string(certFile))

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
		return nil, fmt.Errorf("failed to create CSR: %v", err)
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
		return nil, fmt.Errorf("failed to parse CSR: %v", err)
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
		return nil, fmt.Errorf("failed to create certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	return certPEM, nil
}

func getListOfCardTypes(t *testing.T, dut *ondatra.DUTDevice) [][]string {
	var cardTypes [][]string
	cardNames, serialNo, roles := FindComponentSerialnoRoleByType(t, dut, controlcardType)
	t.Logf("Found card names list: %v", cardNames)
	t.Logf("Found card serialNo list: %v", serialNo)
	t.Logf("Found card role list: %v", roles)
	cardTypes = append(cardTypes, cardNames)
	cardTypes = append(cardTypes, serialNo)
	cardTypes = append(cardTypes, roles)
	return cardTypes
}

func generateOiakCert(iakCert string, dirName string, expiredCert bool) (string, error) {
	pubKey, err := extractPubKeyFromCert(iakCert)
	if err != nil {
		log.Error("Failed to read the Pubkey from Iak Certificate")
		return "", err
	}
	caCertFile := path.Join(dirName, "cacert.rsa.pem")
	caKeyFile := path.Join(dirName, "cakey.rsa.pem")
	_, err = os.Stat(caCertFile)
	if err == nil {
		err := os.Remove(caCertFile)
		if err != nil {
			log.Error("Error removing file:", err.Error())
			return "", err
		}
	}
	_, err = os.Stat(caKeyFile)
	if err == nil {
		err := os.Remove(caKeyFile)
		if err != nil {
			log.Error("Error removing file:", err.Error())
			return "", err
		}
	}
	caKey, caCert, err := cert.GenRootCA("ROOTCA", x509.RSA, 100, dirName)
	if err != nil {
		log.Error("Generation of root ca using rsa is failed: ", err.Error())
		return "", err
	}
	pvtKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		log.Error("Error in generating Private key: ", err.Error())
		return "", err
	}
	csrPEM, err := generateCSR(pvtKey, "Google")
	if err != nil {
		log.Error("Error in generating CSR: ", err.Error())
		return "", err
	}
	signedCertPEM, err := signCertificate(csrPEM, caCert, caKey.(*rsa.PrivateKey), pubKey.(*rsa.PublicKey), expiredCert)

	if err != nil {
		log.Error("Error signing certificate: ", err.Error())
		return "", err
	}
	return string(signedCertPEM), nil
}

func generateOiakCertWithInValidPubKey(dirName string, expiredCert bool) (string, error) {
	caCertFile := path.Join(dirName, "cacert.rsa.pem")
	caKeyFile := path.Join(dirName, "cakey.rsa.pem")
	_, err := os.Stat(caCertFile)
	if err == nil {
		err := os.Remove(caCertFile)
		if err != nil {
			log.Error("Error removing file:", err.Error())
			return "", err
		}
	}
	_, err = os.Stat(caKeyFile)
	if err == nil {
		err := os.Remove(caKeyFile)
		if err != nil {
			log.Error("Error removing file:", err.Error())
			return "", err
		}
	}
	caKey, caCert, err := cert.GenRootCA("ROOTCA", x509.RSA, 100, dirName)
	if err != nil {
		log.Error("Generation of root ca using rsa is failed: ", err.Error())
		return "", err
	}
	pvtKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		log.Error("Error in generating Private key: ", err.Error())
		return "", err
	}
	csrPEM, err := generateCSR(pvtKey, "Google")
	if err != nil {
		log.Error("Error in generating CSR: ", err.Error())
		return "", err
	}
	signedCertPEM, err := signCertificate(csrPEM, caCert, caKey.(*rsa.PrivateKey), nil, expiredCert)

	if err != nil {
		log.Error("Error signing certificate: ", err.Error())
		return "", err
	}
	return string(signedCertPEM), nil
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
	conn, err := grpcConn(targetIP, target, options.TrustBundleFile, options.Username, options.Password)
	if err != nil {
		t.Fatalf("Error in establishing Grpc Clinet connection: %v", err)
	}
	for _, cardType := range cardTypes {
		for _, cType := range cardType {
			t.Run(fmt.Sprintf("Get IAK Certificate with %s", cType), func(t *testing.T) {
				_, err := getIakCert(conn, cType)
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

	conn, err := grpcConn(targetIP, target, options.TrustBundleFile, options.Username, options.Password)
	if err != nil {
		t.Fatalf("Error in establishing Grpc Clinet connection: %v", err)
	}
	for _, cardType := range cardTypes {
		for _, cType := range cardType {
			t.Run(fmt.Sprintf("Roatate OIAK Certificate with %s", cType), func(t *testing.T) {
				var signedCertPEM string
				var rotateResp *enrollzpb.RotateOIakCertResponse

				resp, err := getIakCert(conn, cType)
				if err != nil {
					t.Fatalf("Error in getIak Certificate with card type-%s:%v", cType, err)
				}
				signedCertPEM, err = generateOiakCert(resp.IakCert, dirName, false)
				if err != nil {
					t.Fatalf("Error in generating OIAK Certificate: %v", err)
				}
				rotateResp, err = rotateOIAKCert(conn, cType, signedCertPEM)
				if err != nil {
					t.Fatalf("Error in rotate OIAK Certificate with card type-%s: %v", cType, err)
				}
				log.Infof("%s", rotateResp)
			})
		}
	}

	for _, cardType := range cardTypes {
		for _, cType := range cardType {
			t.Run(fmt.Sprintf("Roatate Expired OIAK Certificate with %s", cType), func(t *testing.T) {
				var signedCertPEM string
				var rotateResp *enrollzpb.RotateOIakCertResponse

				resp, err := getIakCert(conn, cType)
				if err != nil {
					t.Fatalf("Error in getIak Certificate with card type-%s:%v", cType, err)
				}
				signedCertPEM, err = generateOiakCert(resp.IakCert, dirName, true)
				if err != nil {
					t.Fatalf("Error in generating OIAK Certificate: %v", err)
				}
				rotateResp, err = rotateOIAKCert(conn, cType, signedCertPEM)
				if err != nil {
					if strings.Contains(err.Error(), "AttestZ client oIAK certificate not valid yet") {
						log.Infof("Certificate not valid yet is an expected error")
					} else {
						t.Fatalf("Error in rotate OIAK Certificate with card type-%s: %v", cType, err)
					}
				}
				log.Infof("%s", rotateResp)
			})
		}
	}
	for _, cardType := range cardTypes {
		for _, cType := range cardType {
			t.Run(fmt.Sprintf("Roatate OIAK Certificate with Invalid PubKey %s", cType), func(t *testing.T) {
				var signedCertPEM string
				var rotateResp *enrollzpb.RotateOIakCertResponse
				signedCertPEM, err = generateOiakCertWithInValidPubKey(dirName, false)
				if err != nil {
					t.Fatalf("Error in generating OIAK Certificate: %v", err)
				}
				rotateResp, err = rotateOIAKCert(conn, cType, signedCertPEM)
				if err != nil {
					if strings.Contains(err.Error(), "AttestZ client mismatch oIAK pubkey") {
						log.Infof("AttestZ client mismatch OIAK pubkey is an expected error")
					} else {
						t.Fatalf("Error in rotate OIAK Certificate with card type-%s: %v", cType, err)
					}
				}
				log.Infof("%s", rotateResp)
			})
		}
	}
}
