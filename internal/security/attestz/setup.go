// Copyright 2024 Google LLC
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

package attestz

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"testing"

	cdpb "github.com/openconfig/attestz/proto/common_definitions"
	attestzpb "github.com/openconfig/attestz/proto/tpm_attestz"
	enrollzpb "github.com/openconfig/attestz/proto/tpm_enrollz"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/security/svid"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding/introspect"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

const (
	attestzServerName   = "attestz-server"
	sslProfileId        = "tls-attestz"
	mgmtVrf             = "mgmtVrf"
	attestzServerPort   = 9000
	controlcardType     = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	cdpbActive          = cdpb.ControlCardRole_CONTROL_CARD_ROLE_ACTIVE
	cdpbStandby         = cdpb.ControlCardRole_CONTROL_CARD_ROLE_STANDBY
	primaryController   = oc.Platform_ComponentRedundantRole_PRIMARY
	secondaryController = oc.Platform_ComponentRedundantRole_SECONDARY
)

// TlsConf struct holds mtls configs for attestz grpc connections.
type TlsConf struct {
	Target     string
	Mtls       bool
	CaKeyFile  string
	CaCertFile string
	ClientCert []byte
	ClientKey  []byte
	ServerCert []byte
	ServerKey  []byte
}

// NewAttestzSession creates a grpc client used in attestz tests.
func (tc *TlsConf) NewAttestzSession(t *testing.T) *AttestzSession {
	tlsConf := new(tls.Config)
	if tc.Mtls {
		keyPair, err := tls.X509KeyPair(tc.ClientCert, tc.ClientKey)
		if err != nil {
			t.Fatalf("Error loading client keypair. error: %v", err)
		}
		tlsConf.Certificates = []tls.Certificate{keyPair}
		caCertBytes, err := os.ReadFile(tc.CaCertFile)
		if err != nil {
			t.Fatalf("Error reading trust bundle file. error: %v", err)
		}
		trustBundle := x509.NewCertPool()
		if !trustBundle.AppendCertsFromPEM(caCertBytes) {
			t.Fatal("Error loading ca trust bundle.")
		}
		tlsConf.RootCAs = trustBundle
	} else {
		tlsConf.InsecureSkipVerify = true
	}

	conn, err := grpc.NewClient(
		tc.Target,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConf)),
	)

	if err != nil {
		t.Fatalf("Could not connect gnsi. error: %v", err)
	}
	return &AttestzSession{
		Conn:          conn,
		Peer:          new(peer.Peer),
		EnrollzClient: enrollzpb.NewTpmEnrollzServiceClient(conn),
		AttestzClient: attestzpb.NewTpmAttestzServiceClient(conn),
	}
}

// EnableMtls creates client/server certificates signed by the given ca & pushes server certificate to the dut for a given ssl profile.
func (tc *TlsConf) EnableMtls(t *testing.T, dut *ondatra.DUTDevice, sslProfile string) {
	caKey, caCert, err := svid.LoadKeyPair(tc.CaKeyFile, tc.CaCertFile)
	if err != nil {
		t.Fatalf("Error loading ca key/cert. error: %v", err)
	}
	clientIP, serverIP := getGrpcPeers(t, tc)
	tc.ClientCert, tc.ClientKey, err = GenTlsCert(clientIP, caCert, caKey, caCert.PublicKeyAlgorithm)
	if err != nil {
		t.Fatalf("Error generating client tls certs. error: %s", err)
	}
	tc.ServerCert, tc.ServerKey, err = GenTlsCert(serverIP, caCert, caKey, caCert.PublicKeyAlgorithm)
	if err != nil {
		t.Fatalf("Error generating server tls certs. error: %s", err)
	}

	caCertBytes, err := os.ReadFile(tc.CaCertFile)
	if err != nil {
		t.Fatalf("Error reading ca cert. error: %v", err)
	}
	RotateCerts(t, dut, CertTypeRaw, sslProfile, tc.ServerKey, tc.ServerCert, caCertBytes)
	tc.Mtls = true
}

// SetupBaseline setup a test ssl profile & grpc server to be used in attestz tests.
func SetupBaseline(t *testing.T, dut *ondatra.DUTDevice) (string, *oc.System_GrpcServer) {
	AddProfile(t, dut, sslProfileId)
	RotateCerts(t, dut, CertTypeIdevid, sslProfileId, nil, nil, nil)
	gs := createTestGrpcServer(t, dut)
	// Prepare target for the newly created gRPC Server
	dialTarget := introspect.DUTDialer(t, dut, introspect.GNSI).DialTarget
	resolvedTarget, err := net.ResolveTCPAddr("tcp", dialTarget)
	if err != nil {
		t.Fatalf("Failed resolving gnsi target %s", dialTarget)
	}
	resolvedTarget.Port = attestzServerPort
	t.Logf("Target for new gNSI service: %s", resolvedTarget.String())
	return resolvedTarget.String(), gs
}

func getGrpcPeers(t *testing.T, tc *TlsConf) (string, string) {
	var as *AttestzSession
	as = tc.NewAttestzSession(t)
	defer as.Conn.Close()
	// Make a test grpc call to get peer endpoints from the connection.
	_, err := as.EnrollzClient.GetIakCert(context.Background(), &enrollzpb.GetIakCertRequest{ControlCardSelection: ParseRoleSelection(cdpbActive)}, grpc.Peer(as.Peer))
	if err != nil {
		t.Fatalf("Error getting peer endpoints. error: %s", err)
	}
	localAddr := as.Peer.LocalAddr.(*net.TCPAddr)
	remoteAddr := as.Peer.Addr.(*net.TCPAddr)
	t.Logf("Got Local Address: %v, Remote Address: %v", localAddr, remoteAddr)
	return localAddr.IP.String(), remoteAddr.IP.String()
}

func createTestGrpcServer(t *testing.T, dut *ondatra.DUTDevice) *oc.System_GrpcServer {
	t.Logf("Setting test grpc-server")
	root := &oc.Root{}
	s := root.GetOrCreateSystem()
	gs := s.GetOrCreateGrpcServer(attestzServerName)
	gs.SetEnable(true)
	gs.SetPort(uint16(attestzServerPort))
	gs.SetCertificateId(sslProfileId)
	gs.SetServices([]oc.E_SystemGrpc_GRPC_SERVICE{oc.SystemGrpc_GRPC_SERVICE_GNMI, oc.SystemGrpc_GRPC_SERVICE_GNSI})
	gs.SetMetadataAuthentication(false)
	gs.SetNetworkInstance(mgmtVrf)
	gnmi.Update(t, dut, gnmi.OC().System().Config(), s)
	return gnmi.Get[*oc.System_GrpcServer](t, dut, gnmi.OC().System().GrpcServer(attestzServerName).State())
}

// Ensure that we can call both controllers.
func findControllers(t *testing.T, dut *ondatra.DUTDevice, controllers []string) (string, string) {
	var primary, secondary string
	for _, controller := range controllers {
		role := gnmi.Get[oc.E_Platform_ComponentRedundantRole](t, dut, gnmi.OC().Component(controller).RedundantRole().State())
		t.Logf("Component(controller).RedundantRole().Get(t): %v, Role: %v", controller, role)
		if role == secondaryController {
			secondary = controller
		} else if role == primaryController {
			primary = controller
		} else {
			t.Fatalf("Expected controller %s to be active or standby, got %v", controller, role)
		}
	}
	if secondary == "" || primary == "" {
		t.Fatalf("Expected non-empty primary and secondary Controller, got primary: %v, secondary: %v", primary, secondary)
	}
	t.Logf("Detected primary: %v, secondary: %v", primary, secondary)

	return primary, secondary
}

// FindControlCards finds active & standby control card on the dut.
func FindControlCards(t *testing.T, dut *ondatra.DUTDevice) (*ControlCard, *ControlCard) {
	activeCard = &ControlCard{Role: cdpbActive}
	standbyCard = &ControlCard{Role: cdpbStandby}
	controllers := components.FindComponentsByType(t, dut, controlcardType)
	t.Logf("Found controller list: %v", controllers)
	if len(controllers) != 2 {
		t.Skipf("Dual controllers required on %v: got %v, want 2", dut.Model(), len(controllers))
	}
	activeCard.Name, standbyCard.Name = findControllers(t, dut, controllers)
	return activeCard, standbyCard
}
