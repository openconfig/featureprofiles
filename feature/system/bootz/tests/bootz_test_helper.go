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

package bootz

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/netip"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang/glog"
	"github.com/openconfig/bootz/dhcp"
	"github.com/openconfig/bootz/server/service"
	"github.com/openconfig/ondatra"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/prototext"

	dhcpLease "github.com/openconfig/bootz/dhcp/plugins/slease"
	bpb "github.com/openconfig/bootz/proto/bootz"
	bootzem "github.com/openconfig/bootz/server/entitymanager"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	frpb "github.com/openconfig/gnoi/factory_reset"
)

// BootzReqLog stores the bootstarp request/response logs for connected chassis.
type bootzReqLog struct {
	StartTimeStamp int
	EndTimeStamp   int
	BootResponse   *bpb.BootstrapDataResponse
	BootRequest    *bpb.GetBootstrapDataRequest
	Err            error
}

// BootzStatusLog stores the bootstarp status request/response logs for connected chassis.
type bootzStatusLog struct {
	CardStatus      []bpb.ControlCardState_ControlCardStatus
	BootStrapStatus []bpb.ReportStatusRequest_BootstrapStatus
}
type bootzLogs map[service.EntityLookup]*bootzReqLog
type bootzStatus map[string]*bootzStatusLog

var (
	bootzReqLogs    = bootzLogs{}
	bootzStatusLogs = bootzStatus{}
	muRw            sync.RWMutex
)

func bootzInterceptor(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()
	glog.Infof("Bootz Server request: \n%s", prettyPrint(req))
	switch breq := req.(type) {
	case *bpb.GetBootstrapDataRequest:
		bootzLog := &bootzReqLog{
			StartTimeStamp: start.Nanosecond(),
			BootRequest:    breq,
		}
		h, err := handler(ctx, req)
		bootzLog.Err = err
		bres, _ := h.(*bpb.BootstrapDataResponse)
		bootzLog.BootResponse = bres
		bootzLog.EndTimeStamp = time.Now().Nanosecond()
		ch := breq.GetChassisDescriptor()
		muRw.Lock()
		defer muRw.Unlock()
		if ch.GetSerialNumber() != "" {
			bootzReqLogs[service.EntityLookup{SerialNumber: ch.GetSerialNumber(), Manufacturer: ch.GetManufacturer()}] = bootzLog
		}
		ccStatus := breq.GetControlCardState()
		if ccStatus != nil && ccStatus.GetSerialNumber() != "" {
			bootzReqLogs[service.EntityLookup{SerialNumber: ccStatus.GetSerialNumber(), Manufacturer: ch.GetManufacturer()}] = bootzLog
		}
		if err != nil {
			glog.Errorf("Bootz Server: Error in processing the request %s", prettyPrint(err))
		} else {
			glog.Infof("Bootz Server reply: %s", prettyPrint(h))
		}
		return h, err
	case *bpb.ReportStatusRequest:
		muRw.Lock()
		defer muRw.Unlock()
		for _, cc := range breq.GetStates() {
			serial := cc.GetSerialNumber()
			_, ok := bootzStatusLogs[cc.GetSerialNumber()]
			if !ok {
				bootzStatusLogs[serial] = &bootzStatusLog{
					CardStatus:      []bpb.ControlCardState_ControlCardStatus{},
					BootStrapStatus: []bpb.ReportStatusRequest_BootstrapStatus{},
				}
			}
			bootzStatusLogs[serial].BootStrapStatus = append(bootzStatusLogs[serial].BootStrapStatus, breq.GetStatus())
			bootzStatusLogs[serial].CardStatus = append(bootzStatusLogs[serial].CardStatus, cc.GetStatus())
		}
		h, err := handler(ctx, req)
		if err != nil {
			glog.Errorf("Bootz Server: Error in processing the request %s", prettyPrint(err))
		} else {
			glog.Infof("Bootz Server reply \n%s", prettyPrint(h))
		}
		return h, err
	default:
		return handler(ctx, req)
	}
}

func awaitBootzStatus(ccSerial string, expected bpb.ReportStatusRequest_BootstrapStatus, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("status %s is not received for controller card %s", expected.String(), ccSerial)
		default:
			status, ok := bootzStatusLogs[ccSerial]
			if ok {
				if status.BootStrapStatus[len(status.BootStrapStatus)-1] == expected {
					return nil
				}
			}
			time.Sleep(1 * time.Second) // avoid busy looping
		}
	}
}

func awaitBootzConnection(chassis service.EntityLookup, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("chassis %v is not connected to bootz server", chassis)
		default:
			_, ok := bootzReqLogs[chassis]
			if ok {
				return nil
			}
			time.Sleep(1 * time.Second) // avoid busy looping
		}
	}
}

func startDhcpServer(intf string, em *bootzem.InMemoryEntityManager, bootzAddr string) error {
	conf := &dhcp.Config{
		Interface:  intf,
		AddressMap: make(map[string]*dhcp.Entry),
		BootzURL:   fmt.Sprintf("bootz://%s/grpc", bootzAddr),
		// Add DNS if is needed
		DNS: []string{},
	}
	for _, c := range em.GetChassisInventory() {
		if dhcpConf := c.GetDhcpConfig(); dhcpConf != nil {
			conf.AddressMap[dhcpConf.GetHardwareAddress()] = &dhcp.Entry{
				IP: dhcpConf.GetIpAddress(),
				Gw: dhcpConf.GetGateway(),
			}
		}
		for _, cc := range c.GetControllerCards() {
			if dhcpConf := cc.GetDhcpConfig(); dhcpConf != nil {
				conf.AddressMap[dhcpConf.GetHardwareAddress()] = &dhcp.Entry{
					IP: dhcpConf.GetIpAddress(),
					Gw: dhcpConf.GetGateway(),
				}
			}
		}
	}
	return dhcp.Start(conf)
}

func awaitDHCPCompletion(hwAddr []string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("DHCP connection was not successfull")
		default:
			for _, addr := range hwAddr {
				r := dhcpLease.AssignedIP(addr)
				if r != "" {
					return nil
				}
			}
			time.Sleep(1 * time.Second) // avoid busy looping
		}
	}
}

// generateCert generate an RSA key/certificate based on given ca key/certificate and cert template
func generateCert(t *testing.T, signingCert *x509.Certificate, signingKey any, ip, cn string) *tls.Certificate {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		t.Fatalf("Could not generate certificate : %v", err)
	}
	certSpec := &x509.Certificate{
		SerialNumber: big.NewInt(int64(time.Now().Year())),
		Subject: pkix.Name{
			CommonName:   cn,
			Organization: []string{"OpenconfigFeatureProfiles"},
			Country:      []string{"US"},
		},
		IPAddresses: []net.IP{net.IP(addr.AsSlice())},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(0, 0, 365),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
	}

	privKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		t.Fatalf("Generation of RSA keys is failed %v", err)
	}

	pubKey := privKey.Public()
	certBytes, err := x509.CreateCertificate(rand.Reader, certSpec, signingCert, pubKey, signingKey)
	if err != nil {
		t.Fatalf("Creation of certificate is failed %v", err)
	}
	x509Cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		t.Fatalf("Parsing of certificate is failed %v", err)
	}
	tlsCert := tls.Certificate{
		Certificate: [][]byte{certBytes},
		PrivateKey:  privKey,
		Leaf:        x509Cert,
	}

	// pem encode
	caCertPEM := new(bytes.Buffer)
	pem.Encode(caCertPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	if err := os.WriteFile("testdata/tls.cert.pem", caCertPEM.Bytes(), 0444); err != nil {
		t.Fatalf("Saving TLS certificate is failed %v", err)
	}

	caPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	})
	if err := os.WriteFile("testdata/tls.key.pem", caPrivKeyPEM.Bytes(), 0400); err != nil {
		t.Fatalf("Saving Private Key is failed %v", err)
	}
	return &tlsCert
}

func prettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

// loadKeyPair loads a pair of RSA private key and certificate from pem files
func loadKeyPair(t *testing.T, keyPath, certPath string) (*rsa.PrivateKey, *x509.Certificate) {
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("Error opening key file %v", err)
	}
	caKeyPem, _ := pem.Decode(keyPEM)
	caPrivateKey, err := x509.ParsePKCS1PrivateKey(caKeyPem.Bytes)
	if err != nil {
		t.Fatalf("Error in parsing private key  %v", err)
	}
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("Error in opening cert file  %v", err)
	}
	caCertPem, _ := pem.Decode(certPEM)
	if caCertPem == nil {
		t.Fatalf("Error in loading ca cert %v", err)
	}
	caCert, err := x509.ParseCertificate(caCertPem.Bytes)
	if err != nil {
		t.Fatalf("Error in parsing ca cert %v", err)
	}
	return caPrivateKey, caCert
}

// CMDViaGNMI push cli command to dut using GNMI,
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

func getBaseConfig(t *testing.T, dut *ondatra.DUTDevice) string {
	switch dut.Vendor() {
	case ondatra.CISCO:
		runningConfig := CMDViaGNMI(context.Background(), t, dut, "show running-config")
		_, after, _ := strings.Cut(runningConfig, "!")
		if after == "" {
			t.Logf("Running config is not expected %s", runningConfig)
		}
		return after
	default:
		t.Fatalf("The vendor config of vendor %s is missing", dut.Vendor().String())
	}
	return ""
}

func getImageHash(t *testing.T, imgPath string) string {
	f, err := os.Open(imgPath)
	if err != nil {
		t.Fatalf("Could not open the file %v", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatalf("Could not calculate sha256 %v", err)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func factoryReset(t *testing.T, dut *ondatra.DUTDevice) {
	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(context.Background())
	if err != nil {
		t.Fatalf("Could not start the GNOI client, %v", err)
	}
	facRe, err := gnoiClient.FactoryReset().Start(context.Background(), &frpb.StartRequest{FactoryOs: false, ZeroFill: false})
	if err != nil {
		t.Fatalf("Failed to initiate Factory Reset on the device, Error : %v ", err)
	}
	t.Logf("Get Response after Factory Reset: %v\n", facRe.GetResponse())
	switch v := facRe.GetResponse().(type) {
	case *frpb.StartResponse_ResetError:
		actErr := facRe.GetResetError()
		t.Fatalf("Error during Factory Reset %v: %v", actErr.GetOther(), actErr.GetDetail())
	case *frpb.StartResponse_ResetSuccess:
		t.Logf("Factory rest as sucessful")
	default:
		t.Fatalf("Expected resetSuccess following Start: got %v", v)
	}
}
