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

package grpc

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/p4rtutils"
	"github.com/openconfig/featureprofiles/internal/security/gnxi"
	"github.com/povsister/scp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/openconfig/ondatra/gnmi"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygot/ygot"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/featureprofiles/internal/cisco/security/cert"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"google.golang.org/grpc"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	authzpb "github.com/openconfig/gnsi/authz"
	certzpb "github.com/openconfig/gnsi/certz"
	grpb "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	p4v1pb "github.com/p4lang/p4runtime/go/p4/v1"
)

const (
	clientIP   = "1.1.1.1"
	lc         = "0/0/CPU0"
	active_rp  = "0/RP0/CPU0"
	standby_rp = "0/RP1/CPU0"
)

type RPCExecutor func(ctx context.Context, conn *grpc.ClientConn) error
type targetInfo struct {
	dut      string
	sshIp    string
	sshPort  int
	sshUser  string
	sshPass  string
	ports    map[string]string
	grpcPort int
}

// Variable definitions
var (
	portID     = uint32(10)
	deviceID   = uint64(1)
	streamName = "p4rt"
)

// Define Test Args Structure
type testArgs struct {
	desc       string
	lowID      uint64
	highID     uint64
	deviceID   uint64
	handle     *p4rt_client.P4RTClient
	wantStatus codes.Code
	wantFail   bool
}

type NsrState struct {
	NSRState string `json:"nsr-state"`
}

func intPtr(v int) *int { return &v }

type GrpcUnconfigOptions struct {
	ServerName          string
	Port                int
	ListenAddress       string
	VRF                 string
	DeleteServices      []string
	DeletePort          bool
	DeleteServer        bool
	DeleteKeepalive     bool
	DeleteKeepaliveTO   bool
	DeleteListenAddress bool
	DeleteVRF           bool
	DeletelocalConn     bool
	DeleteremoteConn    bool
	DeleteTLSDisable    bool
	SSLProfileID        string
	DeleteSSLProfileID  bool
}

type RPCValidationMatrix struct {
	GNMI_Set         bool
	GNMI_Subscribe   bool
	GNOI_SystemTime  bool
	GNSI_AuthzRotate bool
	GNSI_AuthzGet    bool
	GRIBI_Modify     bool
	GRIBI_Get        bool
	P4RT             bool
}

type TelemetrySummary struct {
	Subscriptions       int
	SubscriptionsActive int
	DestinationGroups   int
	GrpcTLSDestinations int
	DialinCount         int
	DialinActive        int
	DialinSessions      int
	SensorGroups        int
	SensorPathsTotal    int
	SensorPathsActive   int
}

type GrpcServerConfig struct {
	Name                 string   // Name of the gRPC server, e.g., "server1"
	Port                 int      // Listening port, 0 to skip
	Services             []string // List of services (e.g., GNMI, GNSI, GRIBI)
	TLS                  string   // TLS mode: "", "no-tls", "tls-mutual"
	SSLProfileID         string   // SSL profile ID, if using TLS
	VRF                  string   // Optional VRF for the gRPC server
	ListenAddrs          []string // List of listen addresses (e.g., ["10.1.1.1", "::1"])
	Dscp                 *int     // DSCP value for QoS (0-63), -1 to skip
	KeepaliveTime        *int     // Server keepalive time in seconds
	KeepaliveTimeout     *int     // Server keepalive timeout in seconds
	AddressFamily        string   // Optional address-family string (e.g., "ipv4", "ipv6")
	LocalConn            bool     // Enable local-connection (Unix socket)
	RemoteConn           bool     // Enable remote-connection (TCP)
	MetadataAuth         bool     // Enable metadata authentication
	CertAuth             bool     // Enable certificate authentication
	Disable              bool     // Disable the server
	ApplyGroups          []string // List of groups to apply (via apply-group)
	DeleteServer         bool     // If true, adds "no server <name>"
	MaxConcurrentStreams *int     // Max concurrent streams (1–128, default 32). Nil means skip
}

type GrpcConfig struct {
	Servers     []GrpcServerConfig
	DeleteBlock bool // If true, sends "no grpc"
}

// Opts contains customs options for a gNMI request.
type Opts struct {
}

type EMSDServerDetail struct {
	Name             string
	Port             int
	Services         string
	Enabled          bool
	KeepaliveTime    int
	KeepaliveTimeout int
	ListenAddresses  string
	DSCP             int
}

type ConfigLine struct {
	Line     string
	Unconfig bool
}

type SubBlock struct {
	Name  string
	Lines []ConfigLine
}
type dynamicCreds struct {
	username   string
	password   string
	secureOnly bool //controls RequireTransportSecurity
}

func (c dynamicCreds) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"username": c.username,
		"password": c.password,
	}, nil
}

func (c dynamicCreds) RequireTransportSecurity() bool {
	return c.secureOnly
}

type GrpcServerParams struct {
	ServerName             string
	Port                   uint16
	Enable                 bool
	Services               []oc.E_SystemGrpc_GRPC_SERVICE
	ListenAddresses        []oc.System_GrpcServer_ListenAddresses_Union
	CertificateID          string
	TransportSecurity      *bool
	MetadataAuthentication *bool
	NetworkInstance        string
}

type EMSDServerBrief struct {
	Name          string
	Status        string // "En" or "Di"
	ListenAddress string
	Port          string
	TLS           string // "En", "Di", "Mu"
	Services      []string
	VRF           string
}

type RPCStats struct {
	Requests       int
	Responses      int
	ErrorResponses int
}

type EMSDServerStats struct {
	ServerName     string
	RPCStatsByPath map[string]RPCStats // key: "/service/method", e.g. "/gnmi.gNMI/Set"
}

type EMSDServerStatus struct {
	Name     string
	Status   string
	Port     string
	TLS      string
	Services []string
	VRF      string
}

type ConfigBuilder struct {
	Service     string
	Lines       []ConfigLine
	SubBlocks   []SubBlock
	DeleteBlock bool
}

type DeviceOrOpts interface {
	GNMIOpts() *Opts
}

func (cb *ConfigBuilder) Add(line string) {
	if strings.TrimSpace(line) != "" {
		cb.Lines = append(cb.Lines, ConfigLine{Line: line})
	}
}

func (cb *ConfigBuilder) Remove(line string) {
	if strings.TrimSpace(line) != "" {
		cb.Lines = append(cb.Lines, ConfigLine{Line: line, Unconfig: true})
	}
}

func (cb *ConfigBuilder) AddSubBlock(name string, lines ...string) {
	var sub []ConfigLine
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			sub = append(sub, ConfigLine{Line: l})
		}
	}
	cb.SubBlocks = append(cb.SubBlocks, SubBlock{Name: name, Lines: sub})
}

type GrpcCLIConfigs struct {
	// Root level options
	ListenAddresses      []string
	SSLProfileID         string
	CertificateID        string
	AddressFamily        []string // ipv4, ipv6
	AAAAuthLogin         string
	AAAAuthzExec         string
	AAAAccountingQ       *int
	DSCP                 string
	TTL                  *int
	Vrf                  string
	TLSMinVersion        string
	TLSMaxVersion        string
	TLSTrustpoint        string
	KeepaliveTime        *int // Server keepalive time in seconds
	KeepaliveTimeout     *int // Server keepalive timeout in seconds
	MinKeepaliveInterval *int // Minimum keepalive interval in seconds
	MaxConcurrentStreams *int // Max concurrent gRPC streams
	MaxRequestPerUser    *int // Max concurrent requests per user
	MaxRequestTotal      *int // Max concurrent requests in total
	MaxStreams           *int // Max number of streaming gRPCs
	MaxStreamsPerUser    *int // Max number of streaming gRPCs per user
	MemoryLimit          *int // Soft memory limit in MB

	// Service ports
	GNMIServicePort  *int
	GRIBIServicePort *int
	P4RTServicePort  *int

	// Server block options
	ServerName             string
	ServerPort             *int     // Server listening port
	ServerListenAddresses  []string // gRPC server listening addresses
	ServerServices         []string // List of gRPC services
	ServerSSLProfileID     string   // SSL Profile ID for the server
	ServerTLS              *bool    // Enable TLS for the server
	ServerDSCP             string   // DSCP QoS marking
	ServerTTL              *int     // TTL for gRPC packets
	ServerVRF              string   // VRF for the gRPC server
	ServerKeepaliveTime    *int     // Server keepalive time
	ServerKeepaliveTimeout *int     // Server keepalive timeout
	LocalConnection        *bool    // Enable Unix socket connection
	MetadataAuthentication *bool    // Enable metadata authentication
	RemoteConnection       *bool    // Enable remote TCP connection
	ServerAddressFamily    string   // Address family identifier type
}

type grpcConnResult struct {
	Conn *grpc.ClientConn
	Dir  string
	Err  error
}

type SampleStreamResult struct {
	Name    string
	Updates []*gpb.Notification
	Err     error
}

type flagCred struct {
	t *testing.T
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
	if _, deadlineSet := ctx.Deadline(); !deadlineSet {
		tmpCtx, cncl := context.WithTimeout(ctx, time.Second*120)
		ctx = tmpCtx
		defer cncl()
	}
	resp, err := gnmiC.Get(ctx, getRequest)
	if err != nil {
		t.Fatalf("running cmd (%s) via GNMI failed: %v", cmd, err)
	}
	return string(resp.GetNotification()[0].GetUpdate()[0].GetVal().GetAsciiVal())
}

// GetRequestMetadata is needed by credentials.PerRPCCredentials.
func (fc flagCred) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	creds := map[string]string{}
	targets := parseBindingFile(fc.t)
	if len(targets) == 0 {
		return creds, fmt.Errorf("no targets found in binding file")
	}

	// Pick the first DUT credentials
	creds["username"] = targets[0].sshUser
	creds["password"] = targets[0].sshPass
	return creds, nil
}

// RequireTransportSecurity is needed by credentials.PerRPCCredentials.
func (flagCred) RequireTransportSecurity() bool {
	return false
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

func getCertFromBindingFile() (string, string, string, error) {
	bindingFile := flag.Lookup("binding").Value.String()
	in, err := os.ReadFile(bindingFile)
	if err != nil {
		return "", "", "", err
	}
	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		return "", "", "", err
	}
	cert := b.Duts[0].Options.CertFile
	key := b.Duts[0].Options.KeyFile
	ca := b.Duts[0].Options.TrustBundleFile
	return cert, key, ca, nil
}

func parseBindingFile(t *testing.T) []targetInfo {
	t.Helper()
	bindingFile := flag.Lookup("binding").Value.String()

	in, err := os.ReadFile(bindingFile)
	if err != nil {
		t.Fatalf("unable to read binding file: %v", err)
	}

	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		t.Fatalf("unable to parse binding file: %v", err)
	}

	var targets []targetInfo
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

		host, portStr, err := net.SplitHostPort(dut.Ssh.Target)
		if err != nil {
			t.Fatalf("invalid SSH target format for %q: %v", dut.Ssh.Target, err)
		}
		sshPort, err := strconv.Atoi(portStr)
		if err != nil {
			t.Fatalf("invalid port in SSH target %q: %v", dut.Ssh.Target, err)
		}

		// Extract ports
		portMap := make(map[string]string)
		for _, p := range dut.Ports {
			portMap[p.Id] = p.Name
		}

		// Inside your parseBindingFile loop:
		var grpcPort int
		if dut.Gnmi != nil && dut.Gnmi.Target != "" {
			_, portStr, err := net.SplitHostPort(dut.Gnmi.Target)
			if err != nil {
				t.Fatalf("invalid GNMI target %q: %v", dut.Gnmi.Target, err)
			}
			grpcPort, _ = strconv.Atoi(portStr)
		}

		targets = append(targets, targetInfo{
			dut:      dut.Id,
			sshIp:    host,
			sshPort:  sshPort,
			sshUser:  sshUser,
			sshPass:  sshPass,
			ports:    portMap,
			grpcPort: grpcPort,
		})
	}
	return targets
}

// getTargetConfig returns the first target configuration from the binding file
// This helper reduces code duplication by providing a single point to access the testbed config
func getTargetConfig(t *testing.T) targetInfo {
	t.Helper()
	return parseBindingFile(t)[0]
}

func formatHostPort(ip string, port int) string {
	if strings.Contains(ip, ":") {
		// IPv6 — wrap in brackets
		return fmt.Sprintf("[%s]:%d", ip, port)
	}
	// IPv4
	return fmt.Sprintf("%s:%d", ip, port)
}

func DialInsecureGRPC(ctx context.Context, t *testing.T, addr string, port int, username, password string, overrideOpts ...grpc.DialOption) *grpc.ClientConn {
	// gRPC options without TLS
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithPerRPCCredentials(dynamicCreds{
			username:   username,
			password:   password,
			secureOnly: false, // allow insecure
		}),
	}
	opts = append(opts, overrideOpts...)

	fullAddr := formatHostPort(addr, port)
	conn, err := grpc.NewClient(fullAddr, opts...)
	if err != nil {
		t.Fatalf("Failed to dial insecure gRPC at %s: %v", fullAddr, err)
	}
	return conn
}

func DialSecureGRPC(ctx context.Context, t *testing.T, addr string, port int, username, password string, overrideOpts ...grpc.DialOption) *grpc.ClientConn {
	cert, key, ca, err := getCertFromBindingFile()
	if err != nil {
		t.Fatalf("Failed to get cert/key from binding file: %v", err)
	}

	// Load client certificate and key
	certificate, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		t.Fatalf("Failed to load client cert and key: %v", err)
	}

	// Load CA certs
	caCerts, err := readCertificatesFromFile(ca)
	if err != nil {
		t.Fatalf("Failed to load CA certs: %v", err)
	}

	rootCAs := x509.NewCertPool()
	for _, c := range caCerts {
		rootCAs.AddCert(c)
	}

	// TLS config with CA trust
	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{certificate},
		RootCAs:      rootCAs,
	}

	// gRPC credentials
	creds := credentials.NewTLS(tlsConf)

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithPerRPCCredentials(dynamicCreds{
			username:   username,
			password:   password,
			secureOnly: true, // enforce TLS
		}),
	}
	opts = append(opts, overrideOpts...)

	fullAddr := formatHostPort(addr, port)
	conn, err := grpc.NewClient(fullAddr, opts...)
	if err != nil {
		t.Fatalf("Failed to dial secure gRPC at %s: %v", fullAddr, err)
	}
	return conn
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

func createProfileRotateCertz(t *testing.T) (clientCertPath, clientKeyPath, caCertPath, dir string) {
	dut := ondatra.DUT(t, "dut")
	dir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatalf("creating temp dir, err: %s", err)
	}

	config := getTargetConfig(t)
	sshIP := config.sshIp

	// Get all available listen-addresses
	listenAddrs := GetGrpcListenAddrs(t, dut)

	options, err := getOptionsFromBindingFile()
	if err != nil {
		t.Fatalf("Error reading Options from Binding file: %v", err)
	}

	gnsiC, err := dut.RawAPIs().BindingDUT().DialGNSI(context.Background())
	if err != nil {
		t.Fatalf("Failed to dial GNSI: %v", err)
	}
	profileID := "rotatecertzrsa"

	profilesList, err := gnsiC.Certz().GetProfileList(context.Background(), &certzpb.GetProfileListRequest{})
	if err != nil {
		t.Logf("Failed to list profiles: %v", err)
	}

	profileExists := false
	for _, profile := range profilesList.GetSslProfileIds() {
		if profile == profileID {
			profileExists = true
			break
		}
	}
	if !profileExists {
		if _, err := gnsiC.Certz().AddProfile(context.Background(), &certzpb.AddProfileRequest{SslProfileId: profileID}); err != nil {
			t.Fatalf("Unexpected Error in adding profile list: %v", err)
		}
	}

	// Generate root CA
	_, _, err = cert.GenRootCA("ROOTCA", x509.RSA, 100, dir)
	if err != nil {
		t.Fatalf("Root CA generation failed: %v", err)
	}

	caCertPath = filepath.Join(dir, "cacert.rsa.pem")
	caKey, caCertLoaded, err := cert.LoadKeyPair(filepath.Join(dir, "cakey.rsa.pem"), caCertPath)
	if err != nil {
		t.Fatalf("Loading root CA failed: %v", err)
	}

	var ipList []net.IP
	ipList = append(ipList, net.ParseIP(sshIP))
	for _, addr := range listenAddrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			t.Fatalf("Invalid IP address found in listenAddrs: %s", addr)
		}
		ipList = append(ipList, ip)
	}

	// Server cert
	// certTemp, err := cert.PopulateCertTemplate("server", []string{"Server.cisco.com"}, []net.IP{net.ParseIP(serverIP)}, []net.IP{net.IPv4(listenAddrs)}, "test", 100)
	certTemp, err := cert.PopulateCertTemplate("server", []string{"Server.cisco.com"}, ipList, "test", 100)
	certTemp.NotBefore = time.Now().Add(-5 * time.Minute)
	if err != nil {
		t.Fatalf("Server cert template failed: %v", err)
	}

	tlscert, err := cert.GenerateCert(certTemp, caCertLoaded, caKey, x509.RSA)
	if err != nil {
		t.Fatalf("Server cert generation failed: %v", err)
	}
	serverKeyPath := filepath.Join(dir, "server_key.pem")
	serverCertPath := filepath.Join(dir, "server_cert.pem")
	if err := cert.SaveTLSCertInPems(tlscert, serverKeyPath, serverCertPath, x509.RSA); err != nil {
		t.Fatalf("Saving server certs failed: %v", err)
	}

	// Client cert
	certTemp1, err := cert.PopulateCertTemplate(options.Username, []string{"client.cisco.com"}, nil, "test", 100)
	certTemp1.NotBefore = time.Now().Add(-5 * time.Minute)
	if err != nil {
		t.Fatalf("Client cert template failed: %v", err)
	}

	clientCert, err := cert.GenerateCert(certTemp1, caCertLoaded, caKey, x509.RSA)
	if err != nil {
		t.Fatalf("Client cert generation failed: %v", err)
	}
	clientKeyPath = filepath.Join(dir, "client_key.pem")
	clientCertPath = filepath.Join(dir, "client_cert.pem")
	if err = cert.SaveTLSCertInPems(clientCert, clientKeyPath, clientCertPath, x509.RSA); err != nil {
		t.Fatalf("Saving client certs failed: %v", err)
	}

	// Rotate certs via gNSI
	certPEM, _ := os.ReadFile(serverCertPath)
	privKeyPEM, _ := os.ReadFile(serverKeyPath)
	certs, err := readCertificatesFromFile(filepath.Join(dir, "cacert.rsa.pem"))
	if err != nil {
		t.Fatalf("Reading CA bundle failed: %v", err)
	}
	var certChain certzpb.CertificateChain
	for i, cert := range certs {
		certMsg := &certzpb.Certificate{
			Type:            certzpb.CertificateType_CERTIFICATE_TYPE_X509,
			Encoding:        certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
			CertificateType: &certzpb.Certificate_RawCertificate{RawCertificate: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})},
		}
		if i > 0 {
			certChain.Parent = &certzpb.CertificateChain{Certificate: certMsg, Parent: certChain.Parent}
		} else {
			certChain = certzpb.CertificateChain{Certificate: certMsg}
		}
	}

	stream, err := gnsiC.Certz().Rotate(context.Background())
	if err != nil {
		t.Fatalf("Failed to initiate Rotate: %v", err)
	}
	rotateReq := &certzpb.RotateCertificateRequest{
		ForceOverwrite: true,
		SslProfileId:   profileID,
		RotateRequest: &certzpb.RotateCertificateRequest_Certificates{
			Certificates: &certzpb.UploadRequest{
				Entities: []*certzpb.Entity{
					{
						Version:   "1.0",
						CreatedOn: uint64(time.Now().Unix()),
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
						Version:   "1.0",
						CreatedOn: uint64(time.Now().Unix()),
						Entity: &certzpb.Entity_TrustBundle{
							TrustBundle: &certChain,
						},
					},
				},
			},
		},
	}
	if err := stream.Send(rotateReq); err != nil {
		t.Fatalf("Rotate send failed: %v", err)
	}
	if _, err := stream.Recv(); err != nil && err != io.EOF {
		t.Fatalf("Rotate receive failed: %v", err)
	}
	if err := stream.Send(&certzpb.RotateCertificateRequest{
		SslProfileId:  profileID,
		RotateRequest: &certzpb.RotateCertificateRequest_FinalizeRotation{FinalizeRotation: &certzpb.FinalizeRequest{}},
	}); err != nil {
		t.Fatalf("Finalize send failed: %v", err)
	}
	stream.CloseSend()

	// ADD DELAY: Wait for certificates to be fully written and become valid
	t.Logf("Waiting 30 seconds for certificates to stabilize...")
	time.Sleep(30 * time.Second)

	return clientCertPath, clientKeyPath, caCertPath, dir
}

func DialSelfSignedGrpc(ctx context.Context, t *testing.T, addr string, port int, clientCertPath, clientKeyPath, caCertPath string) (*grpc.ClientConn, error) {
	clientTLS, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
	if err != nil {
		t.Fatalf("Client cert load failed: %v", err)
	}

	caCertBytes, err := os.ReadFile(caCertPath)
	if err != nil {
		t.Fatalf("CA cert read failed: %v", err)
	}
	rootCAs := x509.NewCertPool()
	ok := rootCAs.AppendCertsFromPEM(caCertBytes)
	if !ok {
		t.Fatalf("Failed to append CA cert")
	}

	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{clientTLS},
		RootCAs:      rootCAs,
	}

	creds := credentials.NewTLS(tlsConf)

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithPerRPCCredentials(flagCred{t: t}),
	}

	// opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(tlsConf))}

	fullAddr := formatHostPort(addr, port)
	conn, err := grpc.NewClient(fullAddr, opts...)
	if err != nil {
		t.Fatalf("Dial to %s failed: %v", fullAddr, err)
	}
	return conn, nil
}

func buildGrpcConfigBuilder(cfg GrpcConfig) ConfigBuilder {
	builder := ConfigBuilder{Service: "grpc"}

	if cfg.DeleteBlock {
		builder.DeleteBlock = true
		return builder
	}

	for _, s := range cfg.Servers {
		serverBlock := fmt.Sprintf("server %s", s.Name)

		if s.DeleteServer {
			builder.Remove(serverBlock)
			continue
		}

		var lines []string

		if s.Disable {
			lines = append(lines, "disable")
		}
		if s.Port > 0 {
			lines = append(lines, fmt.Sprintf("port %d", s.Port))
		}
		for _, svc := range s.Services {
			lines = append(lines, fmt.Sprintf("services %s", svc))
		}
		if s.TLS != "" {
			lines = append(lines, fmt.Sprintf("tls %s", s.TLS))
		}
		if s.SSLProfileID != "" {
			lines = append(lines, fmt.Sprintf("ssl-profile-id %s", s.SSLProfileID))
		}
		if s.CertAuth {
			lines = append(lines, "certificate-authentication")
		}
		if s.MetadataAuth {
			lines = append(lines, "metadata-authentication")
		}
		if s.VRF != "" {
			lines = append(lines, fmt.Sprintf("vrf %s", s.VRF))
		}
		for _, addr := range s.ListenAddrs {
			lines = append(lines, fmt.Sprintf("listen-addresses %s", addr))
		}
		if s.AddressFamily != "" {
			lines = append(lines, fmt.Sprintf("address-family %s", s.AddressFamily))
		}
		if s.Dscp != nil {
			lines = append(lines, fmt.Sprintf("dscp %d", *s.Dscp))
		}
		if s.KeepaliveTime != nil {
			lines = append(lines, fmt.Sprintf("keepalive time %d", *s.KeepaliveTime))
		}
		if s.KeepaliveTimeout != nil {
			lines = append(lines, fmt.Sprintf("keepalive timeout %d", *s.KeepaliveTimeout))
		}
		if s.LocalConn {
			lines = append(lines, "local-connection")
		}
		if s.RemoteConn {
			lines = append(lines, "remote-connection disable")
		}
		if s.MaxConcurrentStreams != nil {
			lines = append(lines, fmt.Sprintf("max-concurrent-streams %d", *s.MaxConcurrentStreams))
		}
		for _, group := range s.ApplyGroups {
			lines = append(lines, fmt.Sprintf("apply-group %s", group))
		}

		builder.AddSubBlock(serverBlock, lines...)
	}

	return builder
}

func pushGrpcCLIConfig(t *testing.T, gnmiC gpb.GNMIClient, builder ConfigBuilder, expectFailure bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gnmiPath := &gpb.Path{Origin: "cli"}

	if builder.DeleteBlock {
		uncfg := fmt.Sprintf("no %s\n!\n", builder.Service)
		val := &gpb.TypedValue{
			Value: &gpb.TypedValue_AsciiVal{AsciiVal: uncfg},
		}
		_, err := gnmiC.Set(ctx, &gpb.SetRequest{
			Update: []*gpb.Update{{Path: gnmiPath, Val: val}},
		})
		if expectFailure {
			if err == nil {
				t.Fatalf("Failed: Expected failure during deletion, but config applied successfully")
			} else {
				t.Logf("Passed: Expected failure occurred: %v", err)
			}
		} else if err != nil {
			t.Fatalf("Failed to delete grpc block: %v", err)
		}
		return
	}

	var cfg strings.Builder

	cfg.WriteString(fmt.Sprintf("%s\n", builder.Service))
	for _, l := range builder.Lines {
		if l.Unconfig {
			cfg.WriteString(fmt.Sprintf(" no %s\n", l.Line))
		} else {
			cfg.WriteString(fmt.Sprintf(" %s\n", l.Line))
		}
	}
	for _, sb := range builder.SubBlocks {
		cfg.WriteString(fmt.Sprintf(" %s\n", sb.Name))
		for _, l := range sb.Lines {
			if l.Unconfig {
				cfg.WriteString(fmt.Sprintf("  no %s\n", l.Line))
			} else {
				cfg.WriteString(fmt.Sprintf("  %s\n", l.Line))
			}
		}
		cfg.WriteString(" !\n")
	}

	cfgStr := cfg.String()
	trimmed := strings.TrimSpace(cfgStr)
	if trimmed == builder.Service || trimmed == "" {
		return
	}

	val := &gpb.TypedValue{
		Value: &gpb.TypedValue_AsciiVal{AsciiVal: cfgStr},
	}
	updateReq := &gpb.SetRequest{Update: []*gpb.Update{{Path: gnmiPath, Val: val}}}
	resp, err := gnmiC.Set(ctx, updateReq)

	if expectFailure {
		if err == nil {
			t.Fatalf("Expected failure, but config applied successfully. Response: %v", resp)
		} else {
			t.Logf("Expected failure occurred: %v", err)
		}
	} else if err != nil {
		t.Fatalf("Failed: Configs apply unsuccessfull got error response %v", err)
	} else {
	}
}

func BuildGrpcServerConfig(params GrpcServerParams) *oc.System {
	var listenAddrs []oc.System_GrpcServer_ListenAddresses_Union

	server := &oc.System_GrpcServer{
		Name:     ygot.String(params.ServerName),
		Enable:   ygot.Bool(params.Enable),
		Services: params.Services,
	}

	if params.Port != 0 {
		server.Port = ygot.Uint16(params.Port)
	}

	if params.ListenAddresses != nil {
		listenAddrs = append(listenAddrs, params.ListenAddresses...)
		server.ListenAddresses = listenAddrs
	}

	if params.CertificateID != "" {
		server.CertificateId = ygot.String(params.CertificateID)
	}
	if params.TransportSecurity != nil {
		server.TransportSecurity = ygot.Bool(*params.TransportSecurity)
	}
	if params.MetadataAuthentication != nil {
		server.MetadataAuthentication = ygot.Bool(*params.MetadataAuthentication)
	}
	if params.NetworkInstance != "" {
		server.NetworkInstance = ygot.String(params.NetworkInstance)
	}

	return &oc.System{
		GrpcServer: map[string]*oc.System_GrpcServer{
			params.ServerName: server,
		},
	}
}

// Helper function to generate correct "no" CLI
func GenerateRemoveCLI(cli string) string {
	cli = strings.TrimSpace(cli)

	// Special handling for services that require base-level removal
	baseRemovals := map[string]string{
		"gnmi":  "no gnmi",
		"gribi": "no gribi",
		"p4rt":  "no p4rt",
	}

	for key, removal := range baseRemovals {
		if strings.HasPrefix(cli, key+" ") || cli == key {
			return removal
		}
	}

	// Default: prepend "no" to the CLI
	return fmt.Sprintf("no %s", cli)
}
func dialConcurrentGRPC(
	ctx context.Context,
	t *testing.T,
	conns []struct {
		Name string
		Port int
	},
	dialFn func(ctx context.Context, t *testing.T, ip string, port int, service string) (*grpc.ClientConn, string, error),
	sshIP string,
	service string,
) map[string]grpcConnResult {
	var wg sync.WaitGroup
	resultMap := make(map[string]grpcConnResult)
	mu := sync.Mutex{}

	for _, c := range conns {
		wg.Add(1)
		go func(name string, port int) {
			defer wg.Done()
			conn, dir, err := dialFn(ctx, t, sshIP, port, service)
			mu.Lock()
			resultMap[name] = grpcConnResult{Conn: conn, Dir: dir, Err: err}
			mu.Unlock()
		}(c.Name, c.Port)
	}
	wg.Wait()
	return resultMap
}
func PushCliConfigViaGNMI(ctx context.Context, t testing.TB, dut *ondatra.DUTDevice, cfg string) (*gpb.SetResponse, error) {
	gnmiC := dut.RawAPIs().GNMI(t)

	textReplaceReq := &gpb.Update{
		Path: &gpb.Path{Origin: "cli"},
		Val: &gpb.TypedValue{
			Value: &gpb.TypedValue_AsciiVal{
				AsciiVal: cfg,
			},
		},
	}
	setRequest := &gpb.SetRequest{
		Update: []*gpb.Update{textReplaceReq},
	}

	if _, deadlineSet := ctx.Deadline(); !deadlineSet {
		tmpCtx, cncl := context.WithTimeout(ctx, time.Second*120)
		ctx = tmpCtx
		defer cncl()
	}

	resp, err := gnmiC.Set(ctx, setRequest)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func Config_Unconfig_Vrf(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, vrfName string, action string) {
	t.Helper()

	baseCtx := ctx
	timeoutCtx, cancel := context.WithTimeout(baseCtx, 60*time.Second)
	defer cancel()

	// Step 1: Fetch management interfaces via GNMI/CLI
	cmd := "show running-config | include interface"
	var output string

	for i := 0; i < 3; i++ {
		output = config.CMDViaGNMI(timeoutCtx, t, dut, cmd)

		if output != "" && !strings.Contains(output, "context deadline exceeded") {
			break
		}
		time.Sleep(3 * time.Second)
	}

	if output == "" {
		t.Fatalf("Failed to retrieve interface list after retries")
	}

	// Parse interface names
	lines := strings.Split(output, "\n")
	var mgmtIntfs []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "interface MgmtEth") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				mgmtIntfs = append(mgmtIntfs, fields[1])
			}
		}
	}

	if len(mgmtIntfs) == 0 {
		t.Fatalf("No management interfaces found in device config")
	}

	// Step 2: Get virtual IPs
	ipv4 := GetVirtualIP(baseCtx, t, dut, "ipv4")
	ipv6 := GetVirtualIP(baseCtx, t, dut, "ipv6")

	var cfg strings.Builder

	switch action {
	case "configure":
		cfg.WriteString(fmt.Sprintf("vrf %s\n!\n", vrfName))

		if ipv4 != "" {
			cfg.WriteString(fmt.Sprintf("ipv4 virtual address vrf %s %s\n", vrfName, ipv4))
		}
		if ipv6 != "" {
			cfg.WriteString(fmt.Sprintf("ipv6 virtual address vrf %s %s\n", vrfName, ipv6))
		}
		cfg.WriteString("!\n")

		cfg.WriteString(fmt.Sprintf("grpc\n vrf %s\n!\n", vrfName))

		for _, intf := range mgmtIntfs {
			cfg.WriteString(fmt.Sprintf("interface %s\n vrf %s\n!\n", intf, vrfName))
		}

		cfg.WriteString(fmt.Sprintf("ssh server vrf %s\n!\n", vrfName))

	case "unconfigure":
		cfg.WriteString(fmt.Sprintf("no ssh server vrf %s\n", vrfName))

		for _, intf := range mgmtIntfs {
			cfg.WriteString(fmt.Sprintf("interface %s\n no vrf %s\n!\n", intf, vrfName))
		}

		cfg.WriteString(fmt.Sprintf("grpc\n no vrf %s\n!\n", vrfName))

		if ipv4 != "" {
			cfg.WriteString(fmt.Sprintf("no ipv4 virtual address vrf %s %s\n", vrfName, ipv4))
		}
		if ipv6 != "" {
			cfg.WriteString(fmt.Sprintf("no ipv6 virtual address vrf %s %s\n", vrfName, ipv6))
		}

		cfg.WriteString(fmt.Sprintf("no vrf %s\n!\n", vrfName))

	default:
		t.Fatalf("Invalid action '%s'. Expected 'configure' or 'unconfigure'", action)
	}

	// Step 3: Push config
	resp, err := PushCliConfigViaGNMI(timeoutCtx, t, dut, cfg.String())
	if err != nil {
		t.Logf("[INFO] Config push generated error: %v", err)
	} else {
		t.Logf("[INFO] Response: %+v", resp)
	}

	// Give device time to settle
	time.Sleep(10 * time.Second)
}

func GetVirtualIP(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, af string) string {
	t.Helper()

	cmd := fmt.Sprintf("show running-config | include %s virtual address", af)

	// Increase timeout to 60s for slow CLI-origin GNMI calls
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var output string

	// Retry 3 times in case of transient GNMI queue issues
	for i := 0; i < 3; i++ {
		output = config.CMDViaGNMI(ctx, t, dut, cmd)

		// If output is valid or not an error, break
		if !strings.Contains(output, "context deadline exceeded") && output != "" {
			break
		}

		// Wait before retrying
		time.Sleep(3 * time.Second)
	}

	var re *regexp.Regexp
	if af == "ipv4" {
		re = regexp.MustCompile(`ipv4 virtual address(?: vrf \S+)? (\d+\.\d+\.\d+\.\d+/\d+)`)
	} else {
		re = regexp.MustCompile(`ipv6 virtual address(?: vrf \S+)? ([\da-fA-F:]+/\d+)`)
	}

	match := re.FindStringSubmatch(output)
	if match == nil {
		return ""
	}
	return match[1]
}

func gNMIUpdate[T any](t testing.TB, dut *ondatra.DUTDevice, q ygnmi.ConfigQuery[T], val T) (*ygnmi.Result, error) {
	t.Helper()

	c, err := newClient(t, dut, "Update")
	if err != nil {
		t.Fatalf("[ERROR] Failed to create gNMI client: %v", err)
		return nil, err
	}

	res, err := ygnmi.Update(context.Background(), c, q, val)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func newClient(t testing.TB, dut *ondatra.DUTDevice, method string) (*ygnmi.Client, error) {
	t.Helper()

	gnmiClient := dut.RawAPIs().GNMI(t)
	if gnmiClient == nil {
		return nil, fmt.Errorf("raw GNMI client is nil for DUT %s", dut.Name())
	}

	client, err := ygnmi.NewClient(gnmiClient, ygnmi.WithTarget(dut.Name()))
	if err != nil {
		return nil, fmt.Errorf("%s(t) on %s: %v", method, dut.Name(), err)
	}
	return client, nil
}
func CreateUsersOnDevice(t *testing.T, dut *ondatra.DUTDevice, userCount int) ([]string, string) {
	ocAuthentication := &oc.System_Aaa_Authentication{}
	var usernames []string
	commonPassword := "cisco123"

	for i := 1; i <= userCount; i++ {
		username := fmt.Sprintf("testuser%d", i)
		user := &oc.System_Aaa_Authentication_User{
			Username: ygot.String(username),
			Role:     oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN,
			Password: ygot.String(commonPassword),
		}
		ocAuthentication.AppendUser(user)
		usernames = append(usernames, username)
	}

	gnmi.Update(t, dut, gnmi.OC().System().Aaa().Authentication().Config(), ocAuthentication)
	return usernames, commonPassword
}

func DeleteUsersFromDevice(t *testing.T, dut *ondatra.DUTDevice, usernames []string) {
	for _, username := range usernames {
		userPath := gnmi.OC().System().Aaa().Authentication().User(username).Config()
		gnmi.Delete(t, dut, userPath)
	}
}

// ValidateGrpcServerField validates the specified gRPC server field (currently supports "port" and "name").
// If expectMatch is true, it checks if actual == expected.
// If expectMatch is false, it checks that actual != expected.
func ValidateGrpcServerField(t *testing.T, dut *ondatra.DUTDevice, serverName string, field string, expected interface{}, wantMatch bool) {
	t.Helper()

	switch field {
	case "port":
		expectedPort, ok := expected.(uint16)
		if !ok {
			t.Fatalf("Failed: Expected value for port must be of type uint16, got %T", expected)
		}
		port := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer(serverName).Port().State())

		if (port == expectedPort) != wantMatch {
			if wantMatch {
				t.Errorf("Failed: Expected port to match: got %v, want %v", port, expectedPort)
			} else {
				t.Errorf("Failed: Expected port to NOT match %v, but it did", expectedPort)
			}
		} else {
		}

	case "name":
		expectedName, ok := expected.(string)
		if !ok {
			t.Fatalf("Expected value for name must be of type string, got %T", expected)
		}
		name := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer(serverName).Name().State())

		if (name == expectedName) != wantMatch {
			if wantMatch {
				t.Errorf("Failed: Expected name to match: got %q, want %q", name, expectedName)
			} else {
				t.Errorf("Failed: Expected name to NOT match %q, but it did", expectedName)
			}
		} else {
		}

	case "service":
		expectedServices, ok := expected.([]string)
		if !ok {
			t.Fatalf("Expected value for service must be of type []string, got %T", expected)
		}
		services := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer(serverName).Services().State())

		serviceMap := map[string]bool{}
		for _, s := range services {
			serviceMap[s.String()] = true
		}

		matchCount := 0
		for _, es := range expectedServices {
			if serviceMap[es] {
				matchCount++
			}
		}

		allMatched := matchCount == len(expectedServices)

		if allMatched != wantMatch {
			if wantMatch {
				t.Errorf("Failed: Expected all services to match %v, but got %v", expectedServices, services)
			} else {
				t.Errorf("Failed: Expected services to NOT match %v, but all were present", expectedServices)
			}
		} else {
		}

	default:
		t.Fatalf("Unsupported field: %q. Supported fields: \"port\", \"name\", \"service\"", field)
	}
}

func ValidateGNMIGetConfig(t *testing.T, gnmiC gpb.GNMIClient, path []*gpb.Path, field string, expected interface{}, expectFailure bool) {
	t.Helper()

	req := &gpb.GetRequest{
		Path:     path,
		Type:     gpb.GetRequest_CONFIG,
		Encoding: gpb.Encoding_JSON_IETF,
	}

	_, err := protojson.Marshal(req)
	if err != nil {
	}

	resp, err := gnmiC.Get(context.Background(), req)
	if err != nil {
		if expectFailure {
			t.Logf("Expected failure occurred during gNMI Get: %v", err)
			return
		}
		t.Fatalf("gNMI Get failed: %v", err)
	}

	if len(resp.GetNotification()) == 0 {
		if expectFailure {
			t.Log("Expected failure: No notifications in response")
			return
		}
		t.Fatalf("No notifications in gNMI response")
	}

	updates := resp.GetNotification()[0].GetUpdate()
	if len(updates) == 0 {
		if expectFailure {
			t.Log("Expected failure: No updates in gNMI notification")
			return
		}
		t.Fatalf("No updates in gNMI notification")
	}

	val := updates[0].GetVal()
	if val == nil {
		if expectFailure {
			t.Log("Expected failure: Val in update is nil")
			return
		}
		t.Fatalf("gNMI Get Val is nil")
	}

	jsonData := val.GetJsonIetfVal()
	if len(jsonData) == 0 {
		if expectFailure {
			t.Log("Expected failure: json_ietf_val is empty")
			return
		}
		t.Fatalf("json_ietf_val is empty")
	}

	var data interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		t.Fatalf("Failed to unmarshal json_ietf_val: %v", err)
	}

	var got interface{}
	switch val := data.(type) {
	case map[string]interface{}:
		var ok bool
		got, ok = val[field]
		if !ok {
			t.Fatalf("Field %q not found in JSON object", field)
		}
	case string, float64, bool, nil:
		got = val
	default:
		t.Fatalf("Unhandled JSON type: %T", val)
	}

	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Validation failed for field %q: expected %v, got %v", field, expected, got)
	} else {
	}
}

func ParseShowEMSDCoreServer(output, serverName string) (EMSDServerStatus, error) {
	lines := strings.Split(output, "\n")
	var result EMSDServerStatus
	var found bool

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "-") {
			continue
		}

		if !found && (strings.HasPrefix(line, serverName) || strings.Contains(line, serverName)) {
			fields := strings.Split(line, "|")
			if len(fields) < 7 {
				return EMSDServerStatus{}, fmt.Errorf("unexpected field count in line: %s", line)
			}

			result = EMSDServerStatus{
				Name:   strings.TrimSpace(fields[0]),
				Status: strings.TrimSpace(fields[1]),
				Port:   strings.TrimSpace(fields[3]),
				TLS:    strings.TrimSpace(fields[4]),
				VRF:    strings.TrimSpace(fields[6]),
			}
			service := strings.TrimSpace(fields[5])
			if service != "" {
				result.Services = append(result.Services, strings.Fields(service)...)
			}
			found = true
			continue
		}

		if found {
			fields := strings.Split(line, "|")
			if len(fields) >= 6 {
				service := strings.TrimSpace(fields[5])
				if service != "" {
					result.Services = append(result.Services, strings.Fields(service)...)
				}
			} else {
				break
			}
		}
	}

	if !found {
		return EMSDServerStatus{}, fmt.Errorf("server %s not found", serverName)
	}
	return result, nil
}

func BuildExpectedEMSDServerStatus(name, status string, port int, tls string, services []string, vrf string) EMSDServerStatus {
	return EMSDServerStatus{
		Name:     name,
		Status:   status,
		Port:     fmt.Sprintf("%d", port),
		TLS:      tls,
		Services: services,
		VRF:      vrf,
	}
}

// Utility to extract integer from "Key : Value" line
func extractInt(line string) int {
	parts := strings.Split(line, ":")
	if len(parts) < 2 {
		return 0
	}
	v := strings.TrimSpace(parts[1])
	n, _ := strconv.Atoi(v)
	return n
}

// Parses CLI output from EMSD stats into structured form
func ParseEMSDServerStats(output, server string) (EMSDServerStats, error) {
	result := EMSDServerStats{
		ServerName:     server,
		RPCStatsByPath: make(map[string]RPCStats),
	}

	var currentRPC string
	var current RPCStats
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "/") {
			if currentRPC != "" {
				result.RPCStatsByPath[currentRPC] = current
			}
			currentRPC = line
			current = RPCStats{}
		} else if strings.HasPrefix(line, "Requests") {
			current.Requests = extractInt(line)
		} else if strings.HasPrefix(line, "Responses") {
			current.Responses = extractInt(line)
		} else if strings.HasPrefix(line, "Error Responses") {
			current.ErrorResponses = extractInt(line)
		}
	}
	if currentRPC != "" {
		result.RPCStatsByPath[currentRPC] = current
	}

	return result, scanner.Err()
}

// Compares actual vs expected per-RPC statistics
func ValidateRPCStats(t *testing.T, label string, actual, expected RPCStats) {
	hasError := false

	if actual.Requests != expected.Requests {
		t.Errorf("[%s] Requests: got %d, want %d", label, actual.Requests, expected.Requests)
		hasError = true
	}
	if actual.Responses != expected.Responses {
		t.Errorf("[%s] Responses: got %d, want %d", label, actual.Responses, expected.Responses)
		hasError = true
	}
	if actual.ErrorResponses != expected.ErrorResponses {
		t.Errorf("[%s] ErrorResponses: got %d, want %d", label, actual.ErrorResponses, expected.ErrorResponses)
		hasError = true
	}
	if hasError {
		t.Logf("[%s] Expected: %+v | Actual: %+v", label, expected, actual)
	} else {
		t.Logf("[%s] RPCStats match: %+v", label, actual)
	}
}

// func TestgRPCServicesForDefaultPort(t testing.TB, dut *ondatra.DUTDevice, rpc *gnxi.RPC, expectErr bool) {
// 	t.Helper()
// 	ctx := context.Background()

// 	fmt.Printf("Invoking RPC: %s", rpc.Path)
// 	err := rpc.Exec(ctx, dut, nil)

// 	if err != nil {
// 		if expectErr {
// 			t.Logf("Expected error occurred for RPC %s: %v", rpc.Path, err)
// 			return
// 		}
// 		t.Errorf("Unexpected error occurred for RPC %s: %v", rpc.Path, err)
// 		return
// 	}

// 	if expectErr {
// 		t.Errorf("Expected error for RPC %s but it succeeded", rpc.Path)
// 		return
// 	}
// 	fmt.Printf("RPC %s executed successfully\n", rpc.Path)
// }

func TestgRPCServicesForDefaultPort(t testing.TB, dut *ondatra.DUTDevice, rpc *gnxi.RPC, expectErr bool) error {
	t.Helper()
	ctx := context.Background()

	start := time.Now()
	fmt.Printf("Invoking RPC: %s", rpc.Path)

	err := rpc.Exec(ctx, dut, nil)
	end := time.Now()

	duration := end.Sub(start)

	if err != nil {
		if expectErr {
			t.Logf("[EXPECTED FAIL] %s failed as expected at %v (duration: %v)\nError: %v", rpc.Path, end.Format(time.RFC3339), duration, err)
			return nil
		}
		return fmt.Errorf("unexpected error occurred for RPC %s: %v", rpc.Path, err)
	}

	if expectErr {
		return fmt.Errorf("expected error but RPC %s succeeded", rpc.Path)
	}

	fmt.Printf("RPC %s executed successfully\n", rpc.Path)
	return nil
}

func VerifygRPCServicesForDefaultPortParallel(t testing.TB, matrix RPCValidationMatrix) {
	dut := ondatra.DUT(t, "dut")

	type rpcTest struct {
		name     string
		fn       func() error
		wantFail bool
	}

	rpcs := []rpcTest{
		{"GNMI_Set", func() error { return TestgRPCServicesForDefaultPort(t, dut, gnxi.RPCs.GnmiSet, false) }, !matrix.GNMI_Set},
		{"GNMI_Subscribe", func() error { return TestgRPCServicesForDefaultPort(t, dut, gnxi.RPCs.GnmiSubscribe, false) }, !matrix.GNMI_Subscribe},
		{"GNOI_SystemTime", func() error { return TestgRPCServicesForDefaultPort(t, dut, gnxi.RPCs.GnoiSystemTime, false) }, !matrix.GNOI_SystemTime},
		{"GNSI_AuthzRotate", func() error { return TestgRPCServicesForDefaultPort(t, dut, gnxi.RPCs.GnsiAuthzRotate, false) }, !matrix.GNSI_AuthzRotate},
		{"GNSI_AuthzGet", func() error { return TestgRPCServicesForDefaultPort(t, dut, gnxi.RPCs.GnsiAuthzGet, false) }, !matrix.GNSI_AuthzGet},
		{"GRIBI_Modify", func() error { return TestgRPCServicesForDefaultPort(t, dut, gnxi.RPCs.GribiModify, false) }, !matrix.GRIBI_Modify},
		{"GRIBI_Get", func() error { return TestgRPCServicesForDefaultPort(t, dut, gnxi.RPCs.GribiGet, false) }, !matrix.GRIBI_Get},
	}

	var wg sync.WaitGroup
	errMu := sync.Mutex{}
	var failed bool

	for _, r := range rpcs {
		wg.Add(1)
		go func(r rpcTest) {
			defer wg.Done()

			start := time.Now()
			err := r.fn()
			end := time.Now()

			switch {
			case err != nil && r.wantFail:
				t.Logf("[EXPECTED FAIL] %s RPC blocked (as expected) at %v (duration=%v)",
					r.name, end.Format(time.RFC3339), end.Sub(start))

			case err != nil && !r.wantFail:
				t.Errorf("[FAIL] %s RPC failed unexpectedly: %v", r.name, err)
				errMu.Lock()
				failed = true
				errMu.Unlock()

			case err == nil && r.wantFail:
				t.Errorf("[FAIL] %s RPC succeeded unexpectedly", r.name)
				errMu.Lock()
				failed = true
				errMu.Unlock()

			default:
				t.Logf("[SUCCESS] %s RPC completed", r.name)
			}
		}(r)
	}

	wg.Wait()

	if failed {
		// t.FailNow() // marks test as FAILED immediately after all goroutines finish
	}
}

func VerifygRPCServicesForDefaultPort(t testing.TB, matrix RPCValidationMatrix) {
	dut := ondatra.DUT(t, "dut")
	TestgRPCServicesForDefaultPort(t, dut, gnxi.RPCs.GnmiSet, !matrix.GNMI_Set)
	TestgRPCServicesForDefaultPort(t, dut, gnxi.RPCs.GnmiSubscribe, !matrix.GNMI_Subscribe)
	TestgRPCServicesForDefaultPort(t, dut, gnxi.RPCs.GnoiSystemTime, !matrix.GNOI_SystemTime)
	TestgRPCServicesForDefaultPort(t, dut, gnxi.RPCs.GnsiAuthzRotate, !matrix.GNSI_AuthzRotate)
	TestgRPCServicesForDefaultPort(t, dut, gnxi.RPCs.GnsiAuthzGet, !matrix.GNSI_AuthzGet)
	TestgRPCServicesForDefaultPort(t, dut, gnxi.RPCs.GribiModify, !matrix.GRIBI_Modify)
	TestgRPCServicesForDefaultPort(t, dut, gnxi.RPCs.GribiGet, !matrix.GRIBI_Get)
	VerifyP4RTArbitration(t, dut, !matrix.P4RT)
}

func TestgRPCServiceForMultiServer(t testing.TB, name string, conn *grpc.ClientConn, fn RPCExecutor, expectErr bool) {
	t.Helper()
	ctx := context.Background()

	err := fn(ctx, conn)

	if err != nil {
		if expectErr {
			t.Logf("Expected error occurred for RPC %s: %v", name, err)
			return
		}
		t.Errorf("Unexpected error occurred for RPC %s: %v", name, err)
		return
	}

	if expectErr {
		t.Errorf("[FAIL] Expected error for RPC %s but it succeeded", name)
		return
	}

}

func VerifygRPCServicesForMultiServer(t testing.TB, conn *grpc.ClientConn, matrix RPCValidationMatrix) {
	rpcMap := map[string]struct {
		enabled bool
		fn      RPCExecutor
	}{
		"GNMI_Set":         {matrix.GNMI_Set, GnmiSetWithConn},
		"GNMI_Subscribe":   {matrix.GNMI_Subscribe, GnmiSubscribeWithConn},
		"GNOI_SystemTime":  {matrix.GNOI_SystemTime, GnoiSystemTimeWithConn},
		"GNSI_AuthzRotate": {matrix.GNSI_AuthzRotate, GnsiAuthzRotateWithConn},
		"GNSI_AuthzGet":    {matrix.GNSI_AuthzGet, GnsiAuthzGetWithConn},
		"GRIBI_Modify":     {matrix.GRIBI_Modify, GribiModifyWithConn},
		"GRIBI_Get":        {matrix.GRIBI_Get, GribiGetWithConn},
		"P4RT":             {matrix.P4RT, VerifyP4RTArbitrationWithConn},
	}

	for name, r := range rpcMap {
		expectErr := !r.enabled // if not enabled, we expect error
		TestgRPCServiceForMultiServer(t, name, conn, r.fn, expectErr)
	}
}

func streamP4RTArb(args *testArgs) (codes.Code, bool, error) {
	streamParameter := p4rt_client.P4RTStreamParameters{
		Name:        streamName,
		DeviceId:    args.deviceID,
		ElectionIdH: args.highID,
		ElectionIdL: args.lowID,
	}
	if args.handle == nil {
		return codes.OK, false, errors.New("missing client")
	}
	args.handle.StreamChannelCreate(&streamParameter)
	if err := args.handle.StreamChannelSendMsg(&streamName, &p4v1pb.StreamMessageRequest{
		Update: &p4v1pb.StreamMessageRequest_Arbitration{
			Arbitration: &p4v1pb.MasterArbitrationUpdate{
				DeviceId: streamParameter.DeviceId,
				ElectionId: &p4v1pb.Uint128{
					High: streamParameter.ElectionIdH,
					Low:  streamParameter.ElectionIdL,
				},
			},
		},
	}); err != nil {
		return codes.OK, false, fmt.Errorf("errors seen when sending ClientArbitration message: %v", err)
	}
	time.Sleep(1 * time.Second)
	return arbitrationResponseStatus(args)
}

// arbitrationResponseStatus returns the status code received and
// whether the stream was terminated.
// Error returned is nil if a valid arbitration response is received or
// the stream is terminated.
func arbitrationResponseStatus(args *testArgs) (codes.Code, bool, error) {
	handle := args.handle
	fmt.Printf("Checking handle response status %v\n", handle)
	if handle == nil {
		return codes.OK, false, errors.New("missing client")
	}
	// Grab Arb Response to look at status code
	_, arbResp, arbErr := handle.StreamChannelGetArbitrationResp(&streamName, 1)
	if err := p4rtutils.StreamTermErr(args.handle.StreamTermErr); err != nil {
		return status.Code(err), true, nil
	}
	if arbErr != nil {
		return codes.OK, false, fmt.Errorf("errors seen in ClientArbitration response: %v", arbErr)
	}

	if arbResp == nil {
		return codes.OK, false, errors.New("missing ClientArbitration response")
	}
	if arbResp.Arb == nil {
		return codes.OK, false, errors.New("missing MasterArbitrationUpdate response")
	}
	if arbResp.Arb.GetStatus() == nil {
		return codes.OK, false, errors.New("missing MasterArbitrationUpdate Status in response")
	}
	return codes.Code(arbResp.Arb.GetStatus().GetCode()), false, nil
}

func clientConnection(t testing.TB, dut *ondatra.DUTDevice) *p4rt_client.P4RTClient {
	ctx := context.Background()
	clientHandle := p4rt_client.NewP4RTClient(&p4rt_client.P4RTClientParameters{})

	client, err := dut.RawAPIs().BindingDUT().DialP4RT(ctx)
	if err != nil {
		t.Fatalf("Failed to dial P4RT: %v", err)
	}

	if err := clientHandle.P4rtClientSet(client); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}
	return clientHandle
}

func p4rtClientConn(conn *grpc.ClientConn) *p4rt_client.P4RTClient {
	clientHandle := p4rt_client.NewP4RTClient(&p4rt_client.P4RTClientParameters{})
	clientHandle.P4rtClientSet(p4v1pb.NewP4RuntimeClient(conn))
	return clientHandle
}

func Configurep4RTService(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	nodes := p4rtutils.P4RTNodesByPort(t, dut)

	p4rtNode, ok := nodes["port1"]
	if !ok {
		t.Fatal("Couldn't find P4RT Node for port: port1")
	}

	// Set up component
	c := oc.Component{
		Name: ygot.String(p4rtNode),
		IntegratedCircuit: &oc.Component_IntegratedCircuit{
			NodeId: ygot.Uint64(deviceID),
		},
	}
	gnmi.Replace(t, dut, gnmi.OC().Component(p4rtNode).Config(), &c)

	// Set up interface
	portName := dut.Port(t, "port1").Name()
	currIntf := &oc.Interface{
		Name: ygot.String(portName),
		Type: oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		Id:   &portID,
	}
	gnmi.Replace(t, dut, gnmi.OC().Interface(portName).Config(), currIntf)
}

func Unconfigurep4RTService(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	nodes := p4rtutils.P4RTNodesByPort(t, dut)

	p4rtNode, ok := nodes["port1"]
	if !ok {
		t.Fatal("Couldn't find P4RT Node for port: port1")
	}

	// Delete component config
	gnmi.Delete(t, dut, gnmi.OC().Component(p4rtNode).Config())

	time.Sleep(5 * time.Second)

	// Delete interface config
	portName := dut.Port(t, "port1").Name()
	gnmi.Delete(t, dut, gnmi.OC().Interface(portName).Config())

	time.Sleep(5 * time.Second)
}

func VerifyP4RTArbitration(t testing.TB, dut *ondatra.DUTDevice, negative bool) {
	testCases := []testArgs{
		{
			desc:       "Primary",
			lowID:      100,
			highID:     0,
			handle:     clientConnection(t.(*testing.T), dut),
			deviceID:   deviceID,
			wantFail:   false,
			wantStatus: codes.OK,
		},
		{
			desc:       "Secondary",
			lowID:      90,
			highID:     0,
			handle:     clientConnection(t.(*testing.T), dut),
			deviceID:   deviceID,
			wantFail:   false,
			wantStatus: codes.NotFound,
		},
		{
			desc:       "Primary Reconnect",
			lowID:      100,
			highID:     0,
			handle:     clientConnection(t.(*testing.T), dut),
			deviceID:   deviceID,
			wantFail:   false,
			wantStatus: codes.OK,
		},
	}

	for _, tc := range testCases {
		resp, terminated, err := streamP4RTArb(&tc)

		if negative {
			if resp != codes.Unimplemented && resp != codes.NotFound {
				t.Errorf("[%s] Expected gRPC code Unimplemented or NotFound but got: %v", tc.desc, resp)
			}
			if !terminated {
				t.Errorf("[%s] Expected stream to be terminated on failure", tc.desc)
			}
		} else {
			if err != nil {
				t.Errorf("[%s] Unexpected error in stream: %v", tc.desc, err)
			}
			if terminated {
				t.Errorf("[%s] Unexpected stream termination, got %v", tc.desc, resp)
			}
			if resp != tc.wantStatus {
				t.Errorf("[%s] Expected status %v, got %v", tc.desc, tc.wantStatus, resp)
			}
		}

		if tc.handle != nil {
			tc.handle.StreamChannelDestroy(&streamName)
			tc.handle.ServerDisconnect()
		} else {
			t.Errorf("P4RT client handle is nil for test case %q", tc.desc)
		}
	}
}

func VerifyP4RTArbitrationWithConn(ctx context.Context, conn *grpc.ClientConn) error {
	handle := p4rtClientConn(conn)
	testCases := []testArgs{
		{
			desc:       "Primary",
			lowID:      100,
			highID:     0,
			handle:     handle,
			deviceID:   deviceID,
			wantFail:   false,
			wantStatus: codes.OK,
		},
		{
			desc:       "Secondary",
			lowID:      90,
			highID:     0,
			handle:     handle,
			deviceID:   deviceID,
			wantFail:   false,
			wantStatus: codes.NotFound,
		},
		{
			desc:       "Primary Reconnect",
			lowID:      100,
			highID:     0,
			handle:     handle,
			deviceID:   deviceID,
			wantFail:   false,
			wantStatus: codes.OK,
		},
	}

	for _, tc := range testCases {
		resp, terminated, err := streamP4RTArb(&tc)
		if err != nil {
			return fmt.Errorf("[%s] Unexpected error: %v", tc.desc, err)
		}
		if terminated {
			return fmt.Errorf("[%s] Unexpected stream termination, resp: %v", tc.desc, resp)
		}
		if resp != tc.wantStatus {
			return fmt.Errorf("[%s] Expected %v, got %v", tc.desc, tc.wantStatus, resp)
		}
	}
	if handle != nil {
		handle.StreamChannelDestroy(&streamName)
		handle.ServerDisconnect()
	}
	return nil
}

func CMDViaGNMIWithConn(t *testing.T, conn *grpc.ClientConn, cliCmd string, expectErr bool) string {
	t.Helper()

	ctx := context.Background()
	gnmiClient := gpb.NewGNMIClient(conn)

	getRequest := &gpb.GetRequest{
		Prefix: &gpb.Path{
			Origin: "cli",
		},
		Path: []*gpb.Path{
			{
				Elem: []*gpb.PathElem{{Name: cliCmd}},
			},
		},
		Encoding: gpb.Encoding_ASCII,
	}

	if _, deadlineSet := ctx.Deadline(); !deadlineSet {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
		defer cancel()
	}

	resp, err := gnmiClient.Get(ctx, getRequest)

	if expectErr {
		if err == nil {
			t.Fatalf("Expected error for CLI command %q, but got none", cliCmd)
		}
		t.Logf("Expected error occurred for CLI command %q: %v", cliCmd, err)
		return ""
	}

	if err != nil {
		t.Fatalf("GNMI Get failed for CLI command %q: %v", cliCmd, err)
	}

	output := string(resp.GetNotification()[0].GetUpdate()[0].GetVal().GetAsciiVal())
	return output
}

func ParseEMSDServerBrief_CLI(output, serverName string) (EMSDServerBrief, error) {
	var result EMSDServerBrief
	result.Name = serverName

	lines := strings.Split(output, "\n")
	found := false
	var services []string

	for i, line := range lines {
		if strings.Contains(line, serverName) {
			found = true
			fields := strings.Split(line, "|")
			if len(fields) < 7 {
				return result, fmt.Errorf("unexpected format in main row: %q", line)
			}
			result.Status = strings.TrimSpace(fields[1])
			result.ListenAddress = strings.TrimSpace(fields[2])
			result.Port = strings.TrimSpace(fields[3])
			result.TLS = strings.TrimSpace(fields[4])

			s := strings.TrimSpace(fields[5])
			if s != "" {
				services = append(services, s)
			}
			result.VRF = strings.TrimSpace(fields[6])

			// Check following lines for additional services
			for j := i + 1; j < len(lines); j++ {
				nextLine := lines[j]
				if strings.Contains(nextLine, "+----+") || strings.TrimSpace(nextLine) == "" {
					break
				}
				svcFields := strings.Split(nextLine, "|")
				if len(svcFields) >= 6 {
					s := strings.TrimSpace(svcFields[5])
					if s != "" {
						services = append(services, s)
					}
				}
			}
			break
		}
	}

	if !found {
		return result, fmt.Errorf("server '%s' not found in brief output", serverName)
	}
	result.Services = services
	return result, nil
}

func ValidateEMSDServerBrief_SSH(t *testing.T, dut *ondatra.DUTDevice, serverName string, expected EMSDServerBrief, negative bool) {
	fmt.Printf("Validating EMSD Server Brief for server: %s\n", serverName)
	ctx := context.Background()
	cli := dut.RawAPIs().CLI(t)

	output, _ := cli.RunCommand(ctx, "show emsd server "+serverName+" brief")
	out := output.Output()

	if strings.Contains(out, fmt.Sprintf("Server '%s' not found", serverName)) {
		if negative {
			t.Errorf("[FAIL] Expected server '%s' to exist, but it was not found", serverName)
		} else {
			t.Logf("[PASS] Expected Server '%s' correctly not found message", serverName)
		}
		return
	}

	if !negative {
		t.Errorf("[FAIL] Expected server '%s' to be absent, but it exists", serverName)
		return
	}

	actual, err := ParseEMSDServerBrief_CLI(out, serverName)
	if err != nil {
		t.Fatalf("Failed to parse EMSD brief output: %v", err)
	}

	if actual.Status == expected.Status {
		t.Logf("[PASS] Status: %q", actual.Status)
	} else {
		t.Errorf("[FAIL] Status: got %q, want %q", actual.Status, expected.Status)
	}

	if actual.TLS == expected.TLS {
		t.Logf("[PASS] TLS: %q", actual.TLS)
	} else {
		t.Errorf("[FAIL] TLS: got %q, want %q", actual.TLS, expected.TLS)
	}

	if actual.Port == expected.Port {
		t.Logf("[PASS] Port: %q", actual.Port)
	} else {
		t.Errorf("[FAIL] Port: got %q, want %q", actual.Port, expected.Port)
	}

	if actual.VRF == expected.VRF {
		t.Logf("[PASS] VRF: %q", actual.VRF)
	} else {
		t.Errorf("[FAIL] VRF: got %q, want %q", actual.VRF, expected.VRF)
	}

	if equalStringSets(actual.Services, expected.Services) {
		t.Logf("[PASS] Services: %v", actual.Services)
	} else {
		t.Errorf("[FAIL] Services: got %v, want %v", actual.Services, expected.Services)
	}

	if actual.ListenAddress == expected.ListenAddress {
		t.Logf("[PASS] ListenAddr: %q", actual.ListenAddress)
	} else {
		t.Errorf("[FAIL] ListenAddr: got %q, want %q", actual.ListenAddress, expected.ListenAddress)
	}
}

func equalStringSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[string]int)
	for _, s := range a {
		set[s]++
	}
	for _, s := range b {
		if set[s] == 0 {
			return false
		}
		set[s]--
		if set[s] < 0 {
			return false
		}
	}
	return true
}

func ValidateEMSDServerStats_SSH(t *testing.T, dut *ondatra.DUTDevice, serverName string, expected EMSDServerStats, negative bool) {
	ctx := context.Background()
	cli := dut.RawAPIs().CLI(t)

	println("serverName: ", serverName)
	output, err := cli.RunCommand(ctx, "show emsd server "+serverName+" statistics")
	if err != nil {
		t.Logf("Error running command: %v", err)
	}

	out := output.Output()

	if strings.Contains(out, fmt.Sprintf("No statistics found for server '%s'", serverName)) {
		if negative {
			t.Logf("[PASS] Expected no statistics for server '%s'", serverName)
		} else {
			t.Errorf("[FAIL] Unexpected absence of statistics for server '%s'", serverName)
		}
		return
	}

	if negative {
		t.Errorf("[FAIL] Expected no statistics for server '%s', but statistics exist", serverName)
		return
	}

	actual, err := ParseEMSDServerStats(out, serverName)
	if err != nil {
		t.Fatalf("Failed to parse EMSD server statistics output: %v", err)
	}

	for rpcPath, expStats := range expected.RPCStatsByPath {
		actStats, ok := actual.RPCStatsByPath[rpcPath]
		if !ok {
			t.Errorf("[FAIL] Missing RPC stats for: %s", rpcPath)
			continue
		}
		ValidateRPCStats(t, rpcPath, actStats, expStats)
	}
}

func RestartAndValidateEMSD(t *testing.T, dut *ondatra.DUTDevice) {
	ctx := context.Background()
	cli := dut.RawAPIs().CLI(t)

	// Step 1: Get initial respawn count
	initialRespawn, err := getEMSDRespawnCount(t, dut)
	if err != nil {
		t.Fatalf("Failed to get initial EMSD respawn count: %v", err)
	}

	// Step 2: Restart EMSD
	t.Log("Restarting EMSD process...")
	_, err = cli.RunCommand(ctx, "process restart emsd")
	if err != nil {
		t.Fatalf("Failed to restart EMSD process: %v", err)
	}

	// Step 3: Wait for EMSD to restart
	time.Sleep(30 * time.Second)

	// Step 4: Get new respawn count
	newRespawn, err := getEMSDRespawnCount(t, dut)
	if err != nil {
		t.Fatalf("Failed to get new EMSD respawn count: %v", err)
	}

	// Step 5: Validate
	if newRespawn <= initialRespawn {
		t.Errorf("Respawn count did not increase: initial=%d, new=%d", initialRespawn, newRespawn)
	} else {
		t.Logf("EMSD successfully restarted, respawn count incremented.")
	}
}

func getEMSDRespawnCount(t testing.TB, dut *ondatra.DUTDevice) (int, error) {
	ctx := context.Background()
	cli := dut.RawAPIs().CLI(t)

	output, err := cli.RunCommand(ctx, "show processes emsd")
	if err != nil {
		return 0, fmt.Errorf("failed to run show command: %v", err)
	}
	resp := output.Output()

	re := regexp.MustCompile(`Respawn count:\s+(\d+)`)
	match := re.FindStringSubmatch(resp)
	if len(match) != 2 {
		return 0, fmt.Errorf("could not parse respawn count from output:\n%s", resp)
	}

	count, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, fmt.Errorf("failed to convert respawn count to int: %v", err)
	}
	return count, nil
}

func GnmiSetWithConn(ctx context.Context, conn *grpc.ClientConn) error {
	println("conn: ", conn)
	gnmiC := gpb.NewGNMIClient(conn)

	ygnmiC, err := ygnmi.NewClient(gnmiC)
	if err != nil {
		return err
	}
	yopts := []ygnmi.Option{
		ygnmi.WithUseGet(),
		ygnmi.WithEncoding(gpb.Encoding_JSON_IETF),
	}
	_, err = ygnmi.Replace[string](ctx, ygnmiC, gnmi.OC().System().Hostname().Config(), "test", yopts...)
	return err
}

func GnmiGetWithConn(ctx context.Context, conn *grpc.ClientConn) error {
	gnmiC := gpb.NewGNMIClient(conn)

	ygnmiC, err := ygnmi.NewClient(gnmiC)
	if err != nil {
		return err
	}
	yopts := []ygnmi.Option{
		ygnmi.WithUseGet(),
		ygnmi.WithEncoding(gpb.Encoding_JSON_IETF),
	}
	_, err = ygnmi.Get[string](ctx, ygnmiC, gnmi.OC().System().Hostname().Config(), yopts...)
	if err != nil && strings.Contains(err.Error(), "value not present") {
		return nil // tolerate missing value
	}
	return err
}

func GnmiSubscribeWithConn(ctx context.Context, conn *grpc.ClientConn) error {
	gnmiC := gpb.NewGNMIClient(conn)

	ygnmiC, err := ygnmi.NewClient(gnmiC)
	if err != nil {
		return err
	}
	_, err = ygnmi.Get[string](ctx, ygnmiC, gnmi.OC().System().Hostname().State())
	return err
}

func GnoiSystemTimeWithConn(ctx context.Context, conn *grpc.ClientConn) error {
	sysClient := spb.NewSystemClient(conn)
	_, err := sysClient.Time(ctx, &spb.TimeRequest{})
	return err
}

func GnoiSystemPingWithConn(ctx context.Context, conn *grpc.ClientConn) error {
	sysClient := spb.NewSystemClient(conn)
	stream, err := sysClient.Ping(ctx, &spb.PingRequest{Destination: "192.0.2.1"})
	if err != nil {
		return err
	}
	_, err = stream.Recv()
	return err
}

func GribiModifyWithConn(ctx context.Context, conn *grpc.ClientConn) error {
	gribiClient := grpb.NewGRIBIClient(conn)
	modStream, err := gribiClient.Modify(ctx)
	if err != nil {
		return err
	}
	err = modStream.Send(&grpb.ModifyRequest{
		Params: &grpb.SessionParameters{
			Redundancy:  grpb.SessionParameters_SINGLE_PRIMARY,
			Persistence: grpb.SessionParameters_PRESERVE,
		},
	})
	if err != nil {
		return err
	}
	_, err = modStream.Recv()
	return err
}

func GribiGetWithConn(ctx context.Context, conn *grpc.ClientConn) error {
	gribiClient := grpb.NewGRIBIClient(conn)
	getReq := &grpb.GetRequest{
		NetworkInstance: &grpb.GetRequest_All{},
		Aft:             grpb.AFTType_ALL,
	}
	stream, err := gribiClient.Get(ctx, getReq)
	if err != nil {
		return err
	}
	_, err = stream.Recv()
	if err == io.EOF {
		return nil
	}
	return err
}

func GnsiAuthzRotateWithConn(ctx context.Context, conn *grpc.ClientConn) error {
	authzClient := authzpb.NewAuthzClient(conn)
	stream, err := authzClient.Rotate(ctx)
	if err != nil {
		return err
	}
	err = stream.Send(&authzpb.RotateAuthzRequest{
		RotateRequest: &authzpb.RotateAuthzRequest_UploadRequest{
			UploadRequest: &authzpb.UploadRequest{
				Version:   "0.0",
				CreatedOn: uint64(time.Now().UnixNano()),
				Policy:    "",
			},
		},
	})
	if err != nil {
		return err
	}
	_, err = stream.Recv()
	if strings.Contains(err.Error(), "invalid policy") || status.Code(err) == codes.InvalidArgument {
		return nil // treat expected failure as success
	}
	return err
}

func GnsiAuthzGetWithConn(ctx context.Context, conn *grpc.ClientConn) error {
	authzClient := authzpb.NewAuthzClient(conn)
	_, err := authzClient.Get(ctx, &authzpb.GetRequest{})
	return err
}

func ParseTelemetrySummary(output string) (*TelemetrySummary, error) {
	summary := &TelemetrySummary{}
	var err error

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(line, "Subscriptions"):
			_, err = fmt.Sscanf(line, "Subscriptions Total: %d Active: %d Paused: %d",
				&summary.Subscriptions, &summary.SubscriptionsActive, new(int))
		case strings.HasPrefix(line, "Destination Groups"):
			_, err = fmt.Sscanf(line, "Destination Groups Total: %d", &summary.DestinationGroups)
		case strings.HasPrefix(line, "Destinations"):
			_, err = fmt.Sscanf(line, "Destinations grpc-tls: %d grpc-nontls: %d tcp: %d udp: %d",
				&summary.GrpcTLSDestinations, new(int), new(int), new(int))
		case strings.HasPrefix(line, "dialin:"):
			_, err = fmt.Sscanf(line, "dialin: %d Active: %d Sessions: %d Connecting: %d",
				&summary.DialinCount, &summary.DialinActive, &summary.DialinSessions, new(int))
		case strings.HasPrefix(line, "Sensor Groups"):
			_, err = fmt.Sscanf(line, "Sensor Groups Total: %d", &summary.SensorGroups)
		case strings.HasPrefix(line, "Sensor Paths"):
			_, err = fmt.Sscanf(line, "Sensor Paths Total: %d Active: %d Not Resolved: %d",
				&summary.SensorPathsTotal, &summary.SensorPathsActive, new(int))
		}

		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("parsing failed at line: %q, err: %v", line, err)
		}
	}
	return summary, nil
}

func ValidateEMSDServerDetail_SSH(t *testing.T, dut *ondatra.DUTDevice, serverName string, expected EMSDServerDetail, negative bool) {
	ctx := context.Background()
	cli := dut.RawAPIs().CLI(t)

	output, err := cli.RunCommand(ctx, "show emsd server "+serverName+" detail")
	if err != nil {
		t.Fatalf("Failed to execute CLI command: %v", err)
	}
	out := output.Output()

	if strings.Contains(out, fmt.Sprintf("No EMSD server found with name '%s'", serverName)) || !strings.Contains(out, "Server name") {
		if negative {
			t.Logf("[PASS] Expected no EMSD server detail for '%s'", serverName)
		} else {
			t.Errorf("[FAIL] Expected server detail for '%s', but not found", serverName)
		}
		return
	}

	if negative {
		t.Errorf("[FAIL] Expected no EMSD server detail for '%s', but server exists", serverName)
		return
	}

	actual := parseEMSDServerDetail(out)

	if actual.Name != expected.Name {
		t.Errorf("[FAIL] Server name: got %q, want %q", actual.Name, expected.Name)
	} else {
		t.Logf("[PASS] Server name: %q", actual.Name)
	}

	if actual.Port != expected.Port {
		t.Errorf("[FAIL] Port: got %d, want %d", actual.Port, expected.Port)
	} else {
		t.Logf("[PASS] Port: %d", actual.Port)
	}

	if actual.Services != expected.Services {
		t.Errorf("[FAIL] Services: got %q, want %q", actual.Services, expected.Services)
	} else {
		t.Logf("[PASS] Services: %q", actual.Services)
	}

	if actual.Enabled != expected.Enabled {
		t.Errorf("[FAIL] Enabled: got %t, want %t", actual.Enabled, expected.Enabled)
	} else {
		t.Logf("[PASS] Enabled: %t", actual.Enabled)
	}

	if actual.KeepaliveTime != expected.KeepaliveTime {
		t.Errorf("[FAIL] KeepaliveTime: got %d, want %d", actual.KeepaliveTime, expected.KeepaliveTime)
	} else {
		t.Logf("[PASS] KeepaliveTime: %d", actual.KeepaliveTime)
	}

	if actual.KeepaliveTimeout != expected.KeepaliveTimeout {
		t.Errorf("[FAIL] KeepaliveTimeout: got %d, want %d", actual.KeepaliveTimeout, expected.KeepaliveTimeout)
	} else {
		t.Logf("[PASS] KeepaliveTimeout: %d", actual.KeepaliveTimeout)
	}

	if actual.ListenAddresses != expected.ListenAddresses {
		t.Errorf("[FAIL] ListenAddresses: got %q, want %q", actual.ListenAddresses, expected.ListenAddresses)
	} else {
		t.Logf("[PASS] ListenAddresses: %q", actual.ListenAddresses)
	}

	if actual.DSCP != expected.DSCP {
		t.Errorf("[FAIL] DSCP: got %d, want %d", actual.DSCP, expected.DSCP)
	} else {
		t.Logf("[PASS] DSCP: %d", actual.DSCP)
	}
}

func parseEMSDServerDetail(out string) EMSDServerDetail {
	getVal := func(key string) string {
		re := regexp.MustCompile(fmt.Sprintf(`(?m)^%s\s*:\s*(.*)$`, regexp.QuoteMeta(key)))
		matches := re.FindStringSubmatch(out)
		if len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
		return ""
	}

	enabledStr := getVal("Enabled")
	enabled := strings.EqualFold(enabledStr, "Yes")

	port, _ := strconv.Atoi(getVal("Port"))
	kaTime, _ := strconv.Atoi(getVal("Keepalive time"))
	kaTimeout, _ := strconv.Atoi(getVal("Keepalive timeout"))
	dscp, _ := strconv.Atoi(getVal("DSCP"))

	return EMSDServerDetail{
		Name:             getVal("Server name"),
		Port:             port,
		Services:         getVal("Services"),
		Enabled:          enabled,
		KeepaliveTime:    kaTime,
		KeepaliveTimeout: kaTimeout,
		ListenAddresses:  getVal("Listen addresses"),
		DSCP:             dscp,
	}
}

func ValidateTelemetrySummary_SSH(t *testing.T, dut *ondatra.DUTDevice, expected *TelemetrySummary) {
	ctx := context.Background()
	cli := dut.RawAPIs().CLI(t)
	output, _ := cli.RunCommand(ctx, "show telemetry model-driven summary")
	summaryOut := output.Output()

	actual, err := ParseTelemetrySummary(summaryOut)
	if err != nil {
		t.Fatalf("Failed to parse telemetry summary output: %v", err)
	}

	compareInt := func(field string, got, want int) {
		if got == want {
			t.Logf("[PASS] %s: %d", field, got)
		} else {
			t.Errorf("[FAIL] %s: got %d, want %d", field, got, want)
		}
	}

	compareInt("Subscriptions", actual.Subscriptions, expected.Subscriptions)
	compareInt("SubscriptionsActive", actual.SubscriptionsActive, expected.SubscriptionsActive)
	compareInt("DestinationGroups", actual.DestinationGroups, expected.DestinationGroups)
	compareInt("GrpcTLSDestinations", actual.GrpcTLSDestinations, expected.GrpcTLSDestinations)
	compareInt("DialinCount", actual.DialinCount, expected.DialinCount)
	compareInt("DialinActive", actual.DialinActive, expected.DialinActive)
	compareInt("DialinSessions", actual.DialinSessions, expected.DialinSessions)
	compareInt("SensorGroups", actual.SensorGroups, expected.SensorGroups)
	compareInt("SensorPathsTotal", actual.SensorPathsTotal, expected.SensorPathsTotal)
	compareInt("SensorPathsActive", actual.SensorPathsActive, expected.SensorPathsActive)
}

func startGNMIStream(ctx context.Context, name string, conn *grpc.ClientConn, timeout time.Duration) error {
	client := gpb.NewGNMIClient(conn)

	req := &gpb.SubscribeRequest{
		Request: &gpb.SubscribeRequest_Subscribe{
			Subscribe: &gpb.SubscriptionList{
				Mode: gpb.SubscriptionList_STREAM,
				Subscription: []*gpb.Subscription{
					{
						Path:           &gpb.Path{Elem: []*gpb.PathElem{{Name: "system"}, {Name: "state"}, {Name: "hostname"}}},
						Mode:           gpb.SubscriptionMode_SAMPLE,
						SampleInterval: 30e9,
					},
				},
				UpdatesOnly: false,
			},
		},
	}

	// 1. Dial check
	if conn == nil {
		return fmt.Errorf("[%s] connection is nil", name)
	}
	log.Printf("[%s] connection established", name)

	subCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	sub, err := client.Subscribe(subCtx)
	if err != nil {
		return fmt.Errorf("[%s] Subscribe() failed: %w", name, err)
	}

	// 2. Send check
	if err := sub.Send(req); err != nil {
		return fmt.Errorf("[%s] Send() failed: %w", name, err)
	}
	log.Printf("[%s] GNMI stream started successfully", name)

	time.Sleep(20 * time.Second) // wait before receiving

	for {
		resp, err := sub.Recv()
		if err != nil {
			s, ok := status.FromError(err)
			if ok {
				switch s.Code() {
				case codes.Canceled:
					// Silent termination for cancelled streams - no debug output
					return nil
				case codes.Unavailable:
					fmt.Printf("[DEBUG] %s: Stream unavailable - connection closed (expected termination)\n", name)
					return nil
				case codes.DeadlineExceeded:
					fmt.Printf("[DEBUG] %s: Stream deadline exceeded (expected termination)\n", name)
					return nil
				case codes.ResourceExhausted:
					fmt.Printf("[DEBUG] %s: Recv() failed, error: %v\n", name, err)
					fmt.Printf("[DEBUG] %s: gRPC error code: %v, message: %s\n", name, s.Code(), s.Message())
					fmt.Printf("[DEBUG] %s: Resource exhausted - likely hit stream limit (expected for overflow clients)\n", name)
					return fmt.Errorf("resource exhausted for %s (stream limit reached): %w", name, err)
				case codes.PermissionDenied:
					fmt.Printf("[DEBUG] %s: Recv() failed, error: %v\n", name, err)
					fmt.Printf("[DEBUG] %s: gRPC error code: %v, message: %s\n", name, s.Code(), s.Message())
					fmt.Printf("[DEBUG] %s: Permission denied - auth/authorization issue\n", name)
					return fmt.Errorf("permission denied for %s: %w", name, err)
				}
			}

			// Unexpected error
			fmt.Printf("[DEBUG] %s: Recv() failed, error: %v\n", name, err)
			fmt.Printf("[DEBUG] %s: Unexpected error during Recv(): %v\n", name, err)
			return fmt.Errorf("Recv() failed for %s: %w", name, err)
		}

		// Check for sync_response and highlight it
		if resp.GetSyncResponse() {
			fmt.Printf("[UPDATE] Received update for %s: sync_response:true\n", name)
		} else {
			// Log other updates with reduced verbosity
			fmt.Printf("[UPDATE] Received update for %s: %v\n", name, resp)
		}

		// Continue listening for more updates
	}
}

func ApplyBlockingACL(ctx context.Context, t testing.TB, dut *ondatra.DUTDevice, intf string, port int) error {
	t.Helper()

	// Step 1: Define ACLs (but don’t apply yet)
	aclCfg := fmt.Sprintf(`
	ipv4 access-list BLOCK-GRPC
	10 deny tcp any any eq %d
	20 permit ipv4 any any
	!
	ipv6 access-list BLOCK-GRPC
	10 deny tcp any any eq %d
	20 permit ipv6 any any
	!
	`, port, port)

	// Step 2: Apply ACLs to interface
	ifCfg := fmt.Sprintf(`
	interface %s
	ipv4 access-group BLOCK-GRPC in
	ipv6 access-group BLOCK-GRPC in
	!
	`, intf)

	// Push ACL definition first
	if _, err := PushCliConfigViaGNMI(ctx, t, dut, aclCfg); err != nil {
		t.Fatalf("Failed to define ACL on DUT: %v", err)
		return err
	}

	// Push interface attachment
	if _, err := PushCliConfigViaGNMI(ctx, t, dut, ifCfg); err != nil {
		t.Fatalf("Failed to apply ACL to interface %s: %v", intf, err)
		return err
	}

	return nil
}

func RemoveBlockingACLs(ctx context.Context, t testing.TB, dut *ondatra.DUTDevice, interfaces []string) error {
	// Step 1: Remove ACL from each provided interface
	var intfCfgBuilder strings.Builder
	for _, intf := range interfaces {
		intfCfgBuilder.WriteString(fmt.Sprintf(`
	interface %s
	no ipv4 access-group BLOCK-GRPC ingress
	no ipv6 access-group BLOCK-GRPC ingress
	!
	`, intf))
		intfCfg := intfCfgBuilder.String()
		if intfCfg != "" {
			fmt.Printf("STEP 1: Removing ACL from interfaces...\n")
			if _, err := PushCliConfigViaGNMI(ctx, t, dut, intfCfg); err != nil {
				return fmt.Errorf("failed to detach ACLs from interfaces: %v", err)
			}
		}
	}

	// Remove ACL definitions
	aclCfg := `
	no ipv4 access-list BLOCK-GRPC
	no ipv6 access-list BLOCK-GRPC
	!
	`
	fmt.Printf("STEP 2: Removing ACL definitions...\n")
	if _, err := PushCliConfigViaGNMI(ctx, t, dut, aclCfg); err != nil {
		return fmt.Errorf("failed to remove ACL definitions: %v", err)
	}
	return nil
}

func BuildGrpcUnconfigBuilder(opts GrpcUnconfigOptions) ConfigBuilder {
	if opts.DeleteServer {
		return ConfigBuilder{
			Service: "grpc",
			SubBlocks: []SubBlock{
				{
					Name: fmt.Sprintf("no server %s", opts.ServerName),
				},
			},
		}
	}

	subBlock := SubBlock{
		Name: fmt.Sprintf("server %s", opts.ServerName),
	}

	if opts.Port > 0 {
		subBlock.Lines = append(subBlock.Lines, ConfigLine{
			Line:     fmt.Sprintf("port %d", opts.Port),
			Unconfig: true,
		})
	}

	for _, svc := range opts.DeleteServices {
		subBlock.Lines = append(subBlock.Lines, ConfigLine{
			Line:     fmt.Sprintf("services %s", svc),
			Unconfig: true,
		})
	}

	if opts.DeleteKeepalive {
		subBlock.Lines = append(subBlock.Lines, ConfigLine{
			Line:     "keepalive time",
			Unconfig: true,
		})
	}

	if opts.DeleteKeepaliveTO {
		subBlock.Lines = append(subBlock.Lines, ConfigLine{
			Line:     "keepalive timeout",
			Unconfig: true,
		})
	}

	if opts.DeleteListenAddress && opts.ListenAddress != "" {
		subBlock.Lines = append(subBlock.Lines, ConfigLine{
			Line:     fmt.Sprintf("listen-address %s", opts.ListenAddress),
			Unconfig: true,
		})
	}

	if opts.DeleteVRF && opts.VRF != "" {
		subBlock.Lines = append(subBlock.Lines, ConfigLine{
			Line:     fmt.Sprintf("vrf %s", opts.VRF),
			Unconfig: true,
		})
	}

	if opts.DeletelocalConn {
		subBlock.Lines = append(subBlock.Lines, ConfigLine{
			Line:     "local-connection",
			Unconfig: true,
		})
	}

	if opts.DeleteremoteConn {
		subBlock.Lines = append(subBlock.Lines, ConfigLine{
			Line:     "remote-connection disable",
			Unconfig: true,
		})
	}

	if opts.DeleteTLSDisable {
		subBlock.Lines = append(subBlock.Lines, ConfigLine{
			Line:     "tls disable",
			Unconfig: true,
		})
	}

	if opts.DeleteSSLProfileID && opts.SSLProfileID != "" {
		subBlock.Lines = append(subBlock.Lines, ConfigLine{
			Line:     fmt.Sprintf("ssl-profile-id %s", opts.SSLProfileID),
			Unconfig: true,
		})
	}

	return ConfigBuilder{
		Service:   "grpc",
		SubBlocks: []SubBlock{subBlock},
	}
}

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

func GetGrpcListenAddrs(t *testing.T, dut *ondatra.DUTDevice) []string {
	// Step 1: Detect active/standby RP
	var supervisors []string
	active_state := gnmi.OC().Component(active_rp).Name().State()
	active := gnmi.Get(t, dut, active_state)
	standby_state := gnmi.OC().Component(standby_rp).Name().State()
	standby := gnmi.Get(t, dut, standby_state)
	supervisors = append(supervisors, active, standby)

	_, rpActive := components.FindStandbyControllerCard(t, dut, supervisors)

	// Step 2: Discover management interfaces
	output := CMDViaGNMI(context.Background(), t, dut, "show running-config | include interface")
	if output == "" {
		t.Fatalf("No CLI output received")
	}

	var mgmtRP0, mgmtRP1 string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "interface MgmtEth") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if strings.Contains(fields[1], "RP0") {
					mgmtRP0 = fields[1]
				} else if strings.Contains(fields[1], "RP1") {
					mgmtRP1 = fields[1]
				}
			}
		}
	}
	if mgmtRP0 == "" && mgmtRP1 == "" {
		t.Fatalf("No management interfaces found in CLI output")
	}

	// Step 3: Select mgmt interface for active/standby RP
	var activeIntf, standbyIntf string
	if strings.Contains(rpActive, "RP0") {
		activeIntf = mgmtRP0
		standbyIntf = mgmtRP1
	} else if strings.Contains(rpActive, "RP1") {
		activeIntf = mgmtRP1
		standbyIntf = mgmtRP0
	} else {
		t.Fatalf("Unknown active RP format: %v", rpActive)
	}

	// Step 4: Extract listen addresses
	var listenAddrs []string
	seen := make(map[string]bool)

	addIfValid := func(raw string) {
		ip := strings.TrimSpace(raw)
		if ip == "" || seen[ip] {
			return
		}
		if strings.Count(ip, ".") == 3 || strings.Count(ip, ":") >= 2 {
			listenAddrs = append(listenAddrs, ip)
			seen[ip] = true
		}
	}

	parseInterfaceIPs := func(intf string) {
		if intf == "" {
			return
		}
		ifConfig := CMDViaGNMI(context.Background(), t, dut, "show running-config interface "+intf)
		for _, line := range strings.Split(ifConfig, "\n") {
			fields := strings.Fields(line)
			if len(fields) < 3 {
				continue
			}
			if fields[0] == "ipv4" && fields[1] == "address" {
				addIfValid(fields[2])
			}
			if fields[0] == "ipv6" && fields[1] == "address" && !strings.Contains(line, "virtual") {
				addIfValid(strings.Split(fields[2], "/")[0])
			}
		}
	}

	// Always collect from active RP
	parseInterfaceIPs(activeIntf)

	// Optionally also include standby
	parseInterfaceIPs(standbyIntf)

	if len(listenAddrs) == 0 {
		t.Fatal("No valid listen addresses found")
	}

	return listenAddrs
}

func BuildExpectedRPCMatrix(services []string) RPCValidationMatrix {
	var matrix RPCValidationMatrix

	for _, service := range services {
		switch service {
		case "GNMI":
			matrix.GNMI_Set = true
			matrix.GNMI_Subscribe = true
		case "GNOI":
			matrix.GNOI_SystemTime = true
		case "GNSI":
			matrix.GNSI_AuthzRotate = true
			matrix.GNSI_AuthzGet = true
		case "GRIBI":
			matrix.GRIBI_Modify = true
			matrix.GRIBI_Get = true
		case "P4RT":
			matrix.P4RT = true
		}
	}

	return matrix
}

func copyAndChmodBinary(t *testing.T, scpClient *scp.Client, bin string) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	cliHandle := dut.RawAPIs().CLI(t)
	remotePath := "/harddisk:/" + filepath.Base(bin)

	// Step 1: Copy binary to remote
	copyResp := scpClient.CopyFileToRemote(bin, "/harddisk:/", &scp.FileTransferOption{})
	if copyResp != nil {
		if strings.Contains(copyResp.Error(), "Function not implemented") {
			t.Logf("[WARN] SCP not supported, skipping copy for %s", bin)
		} else {
			t.Fatalf("[FAIL] SCP attempt failed for %s: %s", bin, copyResp.Error())
		}
	} else {
	}

	// Step 2: Verify file existence before chmod
	checkCmd := fmt.Sprintf("run ls -l %s", remotePath)
	checkResp, err := cliHandle.RunCommand(ctx, checkCmd)
	if err != nil || !strings.Contains(checkResp.Output(), filepath.Base(bin)) {
		t.Fatalf("[FAIL] Binary %s not found at remote path %s, output: %s", bin, remotePath, checkResp.Output())
	}

	// Step 3: Apply chmod
	chmodCmd := fmt.Sprintf("run chmod +x %s", remotePath)
	chmodResp, err := cliHandle.RunCommand(ctx, chmodCmd)
	if err != nil {
		t.Fatalf("[FAIL] Failed to chmod %s: %v", bin, err)
	}
	if strings.Contains(chmodResp.Output(), "No such file") {
		t.Fatalf("[FAIL] chmod failed, file not found: %s", chmodResp.Output())
	}
}

func validateGnoicOutput(t *testing.T, output string, expectSuccess bool) {
	targetNameRe := regexp.MustCompile(`\|\s*(unix://[^\s]+)\s*\|`)
	timestampRe := regexp.MustCompile(`\|\s*(\d{19})\s*\|`)

	if expectSuccess {
		// Positive case: we expect both target and timestamp to be present
		if m := targetNameRe.FindStringSubmatch(output); m != nil {
			t.Logf("Target Name: %s", m[1])
		} else {
			t.Fatalf("[FAIL] Target Name not found in output (expected success):\n%s", output)
		}

		if m := timestampRe.FindStringSubmatch(output); m != nil {
			t.Logf("Timestamp: %s", m[1])
		} else {
			t.Fatalf("[FAIL] Timestamp not found in output (expected success):\n%s", output)
		}
	} else {
		// Negative case: we expect *no* valid target or timestamp
		if targetNameRe.MatchString(output) || timestampRe.MatchString(output) {
			t.Fatalf("[FAIL] Unexpected success values found in negative test output:\n%s", output)
		} else {
			t.Logf("[PASS] Negative test passed, no target/timestamp found:\n%s", output)
		}
	}
}

func extractTimestampFromGenericLine(line string) (time.Time, error) {
	re := regexp.MustCompile(`\w+\s+\d+\s+\d+:\d+:\d+\.\d+`) // Match "Oct 27 08:53:44.198"
	match := re.FindString(line)
	if match == "" {
		return time.Time{}, fmt.Errorf("no timestamp found in line: %v", line)
	}

	// Prepend current year (since device logs usually omit it)
	year := time.Now().Year()
	tsString := fmt.Sprintf("%d %s", year, strings.TrimSpace(match))

	// Parse with correct layout (no timezone)
	t, err := time.ParseInLocation("2006 Jan 2 15:04:05.000", tsString, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse timestamp: %v (input: %v)", err, tsString)
	}
	return t, nil
}

func getMgmtUpTime(ctx context.Context, t *testing.T) time.Time {
	dut := ondatra.DUT(t, "dut")
	cli := dut.RawAPIs().CLI(t)
	show, _ := cli.RunCommand(ctx, `show logging | i "MgmtEth0/RP0/CPU0/0, changed state to Up"`)
	lines := strings.Split(show.Output(), "\n")

	for _, l := range lines {
		if strings.Contains(l, "MgmtEth0/RP0/CPU0/0, changed state to Up") {
			ts, err := extractTimestampFromGenericLine(l)
			if err == nil {
				t.Logf("[Mgmt UP] %v", ts)
				return ts
			}
		}
	}
	return time.Time{}
}

func getGrpcStartTime(ctx context.Context, t *testing.T) time.Time {
	dut := ondatra.DUT(t, "dut")
	cli := dut.RawAPIs().CLI(t)
	show, _ := cli.RunCommand(ctx, `show emsd trace all | i "Start serving on TCP socket"`)
	lines := strings.Split(show.Output(), "\n")

	for _, l := range lines {
		if strings.Contains(l, "Start serving on TCP socket") {
			ts, err := extractTimestampFromGenericLine(l)
			if err == nil {
				t.Logf("[gRPC Server Start] %v", ts)
				return ts
			}
		}
	}
	return time.Time{}
}

// --- Helper to extract "Startup config done" time from cfgmgr trace ---
func getCfgmgrStartupTime(ctx context.Context, t *testing.T) time.Time {
	dut := ondatra.DUT(t, "dut")
	cli := dut.RawAPIs().CLI(t)

	show, _ := cli.RunCommand(ctx, `show cfgmgr trace | i "Startup config done"`)
	lines := strings.Split(show.Output(), "\n")

	for _, l := range lines {
		if strings.Contains(l, "Startup config done") {
			ts, err := extractTimestampFromGenericLine(l)
			if err == nil {
				t.Logf("[TRACE] Cfgmgr Startup Done timestamp: %v", ts)
				return ts
			}
			t.Logf("[WARN] Failed to parse timestamp from cfgmgr line: %v, err: %v", l, err)
		}
	}
	t.Log("[WARN] No 'Startup config done' entry found in cfgmgr trace output")
	return time.Time{}
}
func getTraceEventTime(ctx context.Context, t *testing.T, cmd, keyword string) time.Time {
	dut := ondatra.DUT(t, "dut")
	cli := dut.RawAPIs().CLI(t)

	show, err := cli.RunCommand(ctx, cmd)
	if err != nil {
		t.Logf("[WARN] Failed to run command '%s': %v", cmd, err)
		return time.Time{}
	}

	lines := strings.Split(show.Output(), "\n")
	var foundLine string
	var eventTime time.Time

	for _, l := range lines {
		if strings.Contains(l, keyword) {
			ts, err := extractTimestampFromGenericLine(l)
			if err == nil {
				eventTime = ts
				foundLine = l
				// Keep updating eventTime if multiple matches exist — we’ll get the latest one
			} else {
				t.Logf("[WARN] Failed to parse timestamp from line: %v, err: %v", l, err)
			}
		}
	}

	if eventTime.IsZero() {
		t.Logf("[WARN] No timestamp found for keyword '%s' in output of '%s'", keyword, cmd)
	} else {
		t.Logf("[TRACE] Found '%s' at %v (line: %v)", keyword, eventTime, foundLine)
	}
	return eventTime
}

func ValidateGrpcServersInParallel(ctx context.Context, t *testing.T,
	sshIP, user, pass string, servers []GrpcServerConfig) {

	var wg sync.WaitGroup

	for _, srv := range servers {
		srv := srv
		wg.Add(1)

		go func() {
			defer wg.Done()
			t.Logf("[START] Validating server: %s (%d)", srv.Name, srv.Port)

			// secure grpc dial
			conn := DialSecureGRPC(ctx, t, sshIP, srv.Port, user, pass)
			if conn == nil {
				t.Errorf("[FAIL] %s: connection failed", srv.Name)
				return
			}
			defer conn.Close()

			// run services
			for _, svc := range srv.Services {
				switch svc {
				case "GNMI":
					t.Logf("[%s] Running GNMI Set + Subscribe", srv.Name)
					GnmiSetWithConn(ctx, conn)
					GnmiSubscribeWithConn(ctx, conn)

				case "GNOI":
					t.Logf("[%s] Running GNOI System", srv.Name)
					GnoiSystemTimeWithConn(ctx, conn)

				case "GNSI":
					t.Logf("[%s] Running GNSI Authz", srv.Name)
					GnsiAuthzRotateWithConn(ctx, conn)
					GnsiAuthzGetWithConn(ctx, conn)

				case "GRIBI":
					t.Logf("[%s] Running GRIBI Modify + Get", srv.Name)
					GribiModifyWithConn(ctx, conn)
					GribiGetWithConn(ctx, conn)

				case "P4RT":
					t.Logf("[%s] Running P4RT", srv.Name)
					VerifyP4RTArbitrationWithConn(ctx, conn)
				}
			}
			t.Logf("[PASS] %s validation done", srv.Name)
		}()
	}
	wg.Wait()
}
