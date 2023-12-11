// Package main provide utility to generate certificate for user and routers.
package main

import (
	"crypto/x509"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"

	certUtil "github.com/openconfig/featureprofiles/internal/cisco/security/cert"
)

var (
	CACert         *x509.Certificate
	caPath         = flag.String("ca_cert_path", "", "path to a directory for ca cert and key located")
	clientKeysPath = flag.String("client_keys_path", "", "path to a directory where clinets keys will be saved")
	caKeyFileName  = "ca.key.pem"
	caCertFileName = "ca.cert.pem"
)

func getCACertPath() (string, error) {
	if *caPath != "" {
		return *caPath, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if strings.Contains(cwd, "/featureprofiles/") {
		rootSrc := strings.Split(cwd, "featureprofiles")[0]
		return rootSrc + "featureprofiles/internal/cisco/security/cert/keys/CA/", nil
	}
	return "", fmt.Errorf("ca_cert_path need to be passed as arg")
}

func getClientsKeysPath() (string, error) {
	if *clientKeysPath != "" {
		return *clientKeysPath, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if strings.Contains(cwd, "/featureprofiles/") {
		rootSrc := strings.Split(cwd, "featureprofiles")[0]
		return rootSrc + "featureprofiles/internal/cisco/security/cert/keys/clients/", nil
	}
	return "", fmt.Errorf("ca_cert_path need to be passed as arg")
}

func main() {
	// add your router or proxy address to the array of ip
	certTemp, err := certUtil.PopulateCertTemplate("test", []string{"test"}, []net.IP{net.IPv4(173, 39, 51, 67)}, "testspiffie", 100)
	if err != nil {
		panic(fmt.Sprintf("Could not generate template: %v", err))
	}
	dir := ""
	if *caPath == "" {
		dir, err = getCACertPath()
		if err != nil {
			panic(fmt.Sprintf("Could not find a path for ca key/cert: %v", err))
		}
	}
	caKey, caCert, err := certUtil.LoadKeyPair(dir+"/"+caKeyFileName, dir+"/"+caCertFileName)
	if err != nil {
		panic(fmt.Sprintf("Could not load ca key/cert: %v", err))
	}
	tlsCert, err := certUtil.GenerateCert(certTemp, caCert, caKey, x509.RSA)
	if err != nil {
		panic(fmt.Sprintf("Could not generate ca cert/key: %v", err))
	}
	clientKeyDir, _ := getClientsKeysPath()
	err = certUtil.SaveTLSCertInPems(tlsCert, clientKeyDir+"test.key.pem", clientKeyDir+"test.cert.pem", x509.RSA)
	if err != nil {
		panic(fmt.Sprintf("Could not save cleint cert/key in pem files: %v", err))
	}
	// Do not remove below as these are infor for b4 testbed emsd cert ips
	/*certUtil.GenCERT("ems", 500, []net.IP{net.IPv4(173, 39, 51, 67), net.IPv4(10, 85, 84, 159), net.IPv4(10, 85, 84, 38)}, "", "")
	// add your users if you use any users other than the followings
	certUtil.GenCERT("cisco", 100, []net.IP{}, "cisco", "")
	certUtil.GenCERT("cafyauto", 100, []net.IP{}, "cafyauto", "")*/
}
